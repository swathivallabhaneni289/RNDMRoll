package spin

import (
	"context"
)

// EntryOutcome is the answer to PostEntry: whether the entry already existed
// (a retry, or a second phone), the entry as stored and Today as of the same
// instant.
type EntryOutcome struct {
	Already bool
	Entry   Entry
	Today   Today
}

// EditOutcome is the answer to EditEntry: the entry after the edit and the
// storage keys of the photos the edit removed or replaced. The caller removes
// those files after the commit.
type EditOutcome struct {
	Entry   Entry
	Dropped []string
}

// DeleteOutcome is the answer to DeleteEntry: the storage keys of every photo
// the entry had, and Today as of the same instant.
type DeleteOutcome struct {
	Dropped []string
	Today   Today
}

// SkipOutcome is the answer to SkipDay: whether the day was already skipped
// (a retry) and Today as of the same instant.
type SkipOutcome struct {
	Already bool
	Today   Today
}

// PostEntry logs the entry for the local date forDate. The phone sends the
// date and the number of the spin it saw (spinSeq); the server decides
// everything else: which spin the entry belongs to (the date's latest, a bonus
// counts), whether it is late and whether the day is still open.
//
// The input is cleaned and the extension of every photo key is checked before
// the lock (a *FieldProblem). Inside the person's lock, with the zone and the
// clock read once, the checks run in this order:
//
//  1. the date already has an entry whose photo keys, in order, are the same
//     list as the request's: that entry is returned with Already true (so a
//     retry never fails against its own entry);
//  2. no spin on the date (including a date that has not come) is ErrNoSpin;
//  3. a settled day (a posted-day row of any source, or a latest spin that is
//     not open) is ErrDaySettled;
//  4. spinSeq is not the latest spin's number: ErrSpinChanged;
//  5. now is at or after the date's late_until: ErrClosed;
//  6. a key that already belongs to any photo row: a *FieldProblem for
//     photos, invalid.
//
// Then, in the same transaction: the entry row (a lost race returns the
// existing row: the same ordered keys mean Already, any other list means
// ErrDaySettled), its photo rows at positions 0 to n-1 in request order, the
// posted day (markPostedIn, source post, which counts the week and may grant
// the bonus) and the spin's result set to posted with settled_at = now. Any
// failure undoes all of it.
func (s *Service) PostEntry(ctx context.Context, userID string, forDate Date, spinSeq int, in EntryInput) (EntryOutcome, error) {
	if forDate.IsZero() {
		return EntryOutcome{}, &InputError{Field: "for_date", Reason: "invalid"}
	}
	clean, err := CleanEntryInput(in)
	if err != nil {
		return EntryOutcome{}, err
	}
	photos, err := storedPhotosOf(clean.Photos)
	if err != nil {
		return EntryOutcome{}, err
	}
	var out EntryOutcome
	err = s.run(ctx, userID, true, func(c *call) error {
		o, err := s.postEntryIn(c, forDate, spinSeq, clean, photos)
		if err != nil {
			return err
		}
		out = o
		return nil
	})
	if err != nil {
		return EntryOutcome{}, err
	}
	return out, nil
}

// storedPhotosOf turns the cleaned new photos into the rows to store: the
// content type comes from the key's extension (an unknown extension is a
// photos problem) and the position is the place in the list.
func storedPhotosOf(list []NewPhoto) ([]NewStoredPhoto, error) {
	out := make([]NewStoredPhoto, len(list))
	for i, p := range list {
		contentType, ok := ContentTypeOfKey(p.Key)
		if !ok {
			return nil, problem(fieldPhotos, ProblemInvalid)
		}
		out[i] = NewStoredPhoto{
			Position:    i,
			Key:         p.Key,
			ContentType: contentType,
			Source:      p.Source,
			SizeBytes:   copyInt(p.SizeBytes),
		}
	}
	return out, nil
}

// hasKeys reports whether the photos' keys, in position order, are exactly
// keys.
func hasKeys(photos []EntryPhoto, keys []string) bool {
	if len(photos) != len(keys) {
		return false
	}
	for i, p := range photos {
		if p.Key != keys[i] {
			return false
		}
	}
	return true
}

func (s *Service) postEntryIn(c *call, forDate Date, spinSeq int, in EntryInput, photos []NewStoredPhoto) (EntryOutcome, error) {
	tx := c.tx
	keys := make([]string, len(photos))
	for i, p := range photos {
		keys[i] = p.Key
	}

	existing, err := tx.EntryOfDate(forDate)
	if err != nil {
		return EntryOutcome{}, err
	}
	if existing != nil && hasKeys(existing.Photos, keys) {
		return s.entryAnswer(c, *existing, true)
	}

	latest, err := tx.LatestSpinOfDate(forDate)
	if err != nil {
		return EntryOutcome{}, err
	}
	if latest == nil {
		return EntryOutcome{}, ErrNoSpin
	}
	posted, err := tx.PostedDay(forDate)
	if err != nil {
		return EntryOutcome{}, err
	}
	if daySettled(posted, latest) {
		return EntryOutcome{}, ErrDaySettled
	}
	if spinSeq != latest.Seq {
		return EntryOutcome{}, ErrSpinChanged
	}
	if !c.now.Before(WindowFor(forDate, c.loc).LateUntil) {
		return EntryOutcome{}, ErrClosed
	}

	used, err := tx.PhotoKeysInUse(keys)
	if err != nil {
		return EntryOutcome{}, err
	}
	for _, k := range keys {
		if used[k] {
			return EntryOutcome{}, problem(fieldPhotos, ProblemInvalid)
		}
	}

	entry, inserted, err := tx.InsertEntry(NewEntry{
		SpinID:    latest.ID,
		LocalDate: forDate,
		Title:     in.Title,
		Stars:     in.Stars,
		Reaction:  in.Reaction,
		Comment:   in.Comment,
		At:        c.now,
	})
	if err != nil {
		return EntryOutcome{}, err
	}
	if !inserted {
		// Another writer's entry for this date or spin won. The same ordered
		// keys are a retry; any other list means the day is taken.
		if hasKeys(entry.Photos, keys) {
			return s.entryAnswer(c, entry, true)
		}
		return EntryOutcome{}, ErrDaySettled
	}
	if err := tx.InsertPhotos(entry.ID, photos, c.now); err != nil {
		return EntryOutcome{}, err
	}
	if _, err := markPostedIn(tx, c.now, c.loc, c.userID, forDate, SourcePost); err != nil {
		return EntryOutcome{}, err
	}
	if err := tx.SettleSpin(latest.ID, ResultPosted, c.now); err != nil {
		return EntryOutcome{}, err
	}

	// Read the entry back so Late, the category and the photos are exactly
	// what a later read gives.
	stored, err := tx.EntryByID(entry.ID)
	if err != nil {
		return EntryOutcome{}, err
	}
	return s.entryAnswer(c, *stored, false)
}

func (s *Service) entryAnswer(c *call, e Entry, already bool) (EntryOutcome, error) {
	t, err := s.todayIn(c)
	if err != nil {
		return EntryOutcome{}, err
	}
	return EntryOutcome{Already: already, Entry: e, Today: t}, nil
}

// GetEntry returns one of the person's entries. An id that is not a canonical
// UUID, that has no entry, or whose entry belongs to someone else is
// ErrEntryNotFound.
func (s *Service) GetEntry(ctx context.Context, userID, id string) (Entry, error) {
	cid, ok := canonicalUUID(id)
	if !ok {
		return Entry{}, ErrEntryNotFound
	}
	var out Entry
	err := s.run(ctx, userID, false, func(c *call) error {
		e, err := c.tx.EntryByID(cid)
		if err != nil {
			return err
		}
		out = *e
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	return out, nil
}

// EditEntry changes title, stars, reaction, comment and photos of one of the
// person's entries, with no time limit. It never changes the date, the
// category, the late flag, the posted day or the spin's result.
//
// The patch is cleaned (a *FieldProblem) and the extension of every new key is
// checked before the lock. Inside the lock the entry is read (ErrEntryNotFound
// for an unknown, foreign or deleted one) and, when the patch carries photos,
// every id must be one of this entry's own photos (the id is checked in Go
// and never reaches storage unchecked), every new key must be unused, and the
// ids and keys must not repeat; any failure is a photos problem, invalid, and
// nothing is written.
//
// Only a real change writes: when the merged values and the final photo list
// equal what is stored (an empty patch, or one that repeats the stored
// values), nothing is written and updated_at stays. Any change bumps
// updated_at to now, a photo-only edit included. Dropped is the keys of the
// photos the edit removed or replaced, in their old position order.
func (s *Service) EditEntry(ctx context.Context, userID, id string, patch EntryPatch) (EditOutcome, error) {
	cid, ok := canonicalUUID(id)
	if !ok {
		return EditOutcome{}, ErrEntryNotFound
	}
	clean, err := CleanEntryPatch(patch)
	if err != nil {
		return EditOutcome{}, err
	}
	var contentTypes []string
	if clean.Photos != nil {
		for _, ref := range *clean.Photos {
			if ref.ID != "" {
				continue
			}
			contentType, ok := ContentTypeOfKey(ref.Key)
			if !ok {
				return EditOutcome{}, problem(fieldPhotos, ProblemInvalid)
			}
			contentTypes = append(contentTypes, contentType)
		}
	}

	var out EditOutcome
	err = s.run(ctx, userID, false, func(c *call) error {
		o, err := s.editEntryIn(c, cid, clean, contentTypes)
		if err != nil {
			return err
		}
		out = o
		return nil
	})
	if err != nil {
		return EditOutcome{}, err
	}
	return out, nil
}

func (s *Service) editEntryIn(c *call, id string, patch EntryPatch, contentTypes []string) (EditOutcome, error) {
	tx := c.tx
	entry, err := tx.EntryByID(id)
	if err != nil {
		return EditOutcome{}, err
	}

	// The scalar values the entry should end with.
	update := EntryUpdate{Title: entry.Title, Stars: entry.Stars, Reaction: entry.Reaction, Comment: entry.Comment}
	if patch.Title != nil {
		update.Title = *patch.Title
	}
	if patch.Stars != nil {
		update.Stars = *patch.Stars
	}
	if patch.Reaction != nil {
		update.Reaction = nilIfEmpty(*patch.Reaction)
	}
	if patch.Comment != nil {
		update.Comment = nilIfEmpty(*patch.Comment)
	}
	scalarsChanged := update.Title != entry.Title || update.Stars != entry.Stars ||
		!sameText(update.Reaction, entry.Reaction) || !sameText(update.Comment, entry.Comment)

	// The photo list the entry should end with. Every check is finished
	// before anything is written.
	var slots []PhotoSlot
	photosChanged := false
	if patch.Photos != nil {
		own := make(map[string]EntryPhoto, len(entry.Photos))
		for _, p := range entry.Photos {
			own[p.ID] = p
		}
		refs := *patch.Photos
		slots = make([]PhotoSlot, len(refs))
		seen := make(map[string]bool, len(refs))
		var newKeys []string
		next := 0
		for i, ref := range refs {
			if ref.ID != "" {
				pid, ok := canonicalUUID(ref.ID)
				if !ok {
					return EditOutcome{}, problem(fieldPhotos, ProblemInvalid)
				}
				kept, found := own[pid]
				if !found || seen[pid] {
					return EditOutcome{}, problem(fieldPhotos, ProblemInvalid)
				}
				seen[pid] = true
				slots[i] = PhotoSlot{ID: pid, Key: kept.Key, ContentType: kept.ContentType, Source: kept.Source, SizeBytes: copyInt(kept.SizeBytes)}
				continue
			}
			slots[i] = PhotoSlot{Key: ref.Key, ContentType: contentTypes[next], Source: ref.Source, SizeBytes: copyInt(ref.SizeBytes)}
			newKeys = append(newKeys, ref.Key)
			next++
		}
		used, err := tx.PhotoKeysInUse(newKeys)
		if err != nil {
			return EditOutcome{}, err
		}
		for _, k := range newKeys {
			if used[k] {
				return EditOutcome{}, problem(fieldPhotos, ProblemInvalid)
			}
		}
		photosChanged = len(slots) != len(entry.Photos)
		for i := 0; !photosChanged && i < len(slots); i++ {
			photosChanged = slots[i].ID == "" || slots[i].ID != entry.Photos[i].ID
		}
	}

	if !scalarsChanged && !photosChanged {
		return EditOutcome{Entry: *entry}, nil
	}

	if err := tx.UpdateEntry(id, update, c.now); err != nil {
		return EditOutcome{}, err
	}
	var dropped []string
	if photosChanged {
		dropped, err = tx.ReplacePhotos(id, slots, c.now)
		if err != nil {
			return EditOutcome{}, err
		}
	}
	stored, err := tx.EntryByID(id)
	if err != nil {
		return EditOutcome{}, err
	}
	return EditOutcome{Entry: *stored, Dropped: dropped}, nil
}

// nilIfEmpty turns the empty string of a cleared field into nil.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sameText reports whether two optional texts hold the same value.
func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// DeleteEntry removes one of the person's entries and its photo rows and
// returns every one of its photo keys, so the caller can remove the files, and
// Today. It removes nothing else: the posted day and the spin's result stay,
// so the day cannot be posted again and a deleted post still counts. An id that
// is not a canonical UUID, that has no entry, or that is someone else's is
// ErrEntryNotFound, and so is a second delete.
func (s *Service) DeleteEntry(ctx context.Context, userID, id string) (DeleteOutcome, error) {
	cid, ok := canonicalUUID(id)
	if !ok {
		return DeleteOutcome{}, ErrEntryNotFound
	}
	var out DeleteOutcome
	err := s.run(ctx, userID, true, func(c *call) error {
		dropped, err := c.tx.DeleteEntry(cid)
		if err != nil {
			return err
		}
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out = DeleteOutcome{Dropped: dropped, Today: t}
		return nil
	})
	if err != nil {
		return DeleteOutcome{}, err
	}
	return out, nil
}

// SkipDay skips the local date forDate: the date's latest spin becomes
// skipped with settled_at = now. It writes no posted day, touches no goal week
// and grants nothing. The order of checks, inside the lock:
//
//  1. no spin on the date is ErrNoSpin;
//  2. the latest spin is already skipped and the date has no posted day:
//     Already true, nothing is written;
//  3. any other settled day is ErrDaySettled;
//  4. spinSeq is not the latest spin's number: ErrSpinChanged;
//  5. now is at or after the date's late_until: ErrClosed.
func (s *Service) SkipDay(ctx context.Context, userID string, forDate Date, spinSeq int) (SkipOutcome, error) {
	if forDate.IsZero() {
		return SkipOutcome{}, &InputError{Field: "for_date", Reason: "invalid"}
	}
	var out SkipOutcome
	err := s.run(ctx, userID, true, func(c *call) error {
		tx := c.tx
		// The stored result is read as it is (dayFacts shows posted when a
		// posted-day row exists, which would hide a skip with no such row).
		latest, err := tx.LatestSpinOfDate(forDate)
		if err != nil {
			return err
		}
		if latest == nil {
			return ErrNoSpin
		}
		posted, err := tx.PostedDay(forDate)
		if err != nil {
			return err
		}
		already := latest.Result == ResultSkipped && posted == nil
		if !already {
			switch {
			case daySettled(posted, latest):
				return ErrDaySettled
			case spinSeq != latest.Seq:
				return ErrSpinChanged
			case !c.now.Before(WindowFor(forDate, c.loc).LateUntil):
				return ErrClosed
			}
			if err := tx.SettleSpin(latest.ID, ResultSkipped, c.now); err != nil {
				return err
			}
		}
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out = SkipOutcome{Already: already, Today: t}
		return nil
	})
	if err != nil {
		return SkipOutcome{}, err
	}
	return out, nil
}
