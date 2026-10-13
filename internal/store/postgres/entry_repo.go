package postgres

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// This file holds the entry and photo methods of the spin transaction. The
// layer knows storage keys only (folder and file name): it never builds a
// link. Every statement carries the person's id, except PhotoKeysInUse, which
// answers across all people because a file belongs to one photo row only.

// parseID turns an id the service passes into a typed uuid. The service sends
// canonical ids only, so a failure here is a bug and comes back as a plain
// error.
func parseID(what, s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("spin: %s id is not a uuid: %w", what, err)
	}
	return id, nil
}

// entrySelect is the one read of an entry. The category id and the name come
// from the entry's own spin (the entry stores neither), and late is the late
// flag of the date's posted-day row, false when the date has none.
const entrySelect = `
	select e.id::text, e.spin_id::text, e.local_date::text, s.category_id::text, s.category_name,
	       e.title, e.stars, e.reaction, e.comment, coalesce(p.late, false), e.created_at, e.updated_at
	from entries e
	join spins s on s.id = e.spin_id and s.user_id = e.user_id
	left join posted_days p on p.user_id = e.user_id and p.local_date = e.local_date
	where e.user_id = $1 and `

// entryRow reads one entry without its photos (Photos is an empty list). The
// where text may use $2 and up. It returns nil when no row matches.
func (t *pgTx) entryRow(where string, args ...any) (*spin.Entry, error) {
	var e spin.Entry
	var dateStr string
	err := t.tx.QueryRow(t.ctx, entrySelect+where, append([]any{t.uid}, args...)...).
		Scan(&e.ID, &e.SpinID, &dateStr, &e.CategoryID, &e.CategoryName, &e.Title, &e.Stars,
			&e.Reaction, &e.Comment, &e.Late, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if e.LocalDate, err = spin.ParseDate(dateStr); err != nil {
		return nil, err
	}
	e.CreatedAt, e.UpdatedAt = e.CreatedAt.UTC(), e.UpdatedAt.UTC()
	e.Photos = []spin.EntryPhoto{}
	return &e, nil
}

// entryQuery reads one complete entry: the row and its photos in position
// order. It returns nil when no row matches.
func (t *pgTx) entryQuery(where string, args ...any) (*spin.Entry, error) {
	e, err := t.entryRow(where, args...)
	if err != nil || e == nil {
		return nil, err
	}
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return nil, err
	}
	if e.Photos, err = t.photosOf(id); err != nil {
		return nil, err
	}
	return e, nil
}

// photosOf reads the photo rows of one of this person's entries in position
// order. An unknown or foreign entry gives an empty list.
func (t *pgTx) photosOf(entryID uuid.UUID) ([]spin.EntryPhoto, error) {
	rows, err := t.tx.Query(t.ctx, `
		select id::text, position, storage_key, content_type, size_bytes, source
		from entry_photos
		where entry_id = $1 and user_id = $2
		order by position`, entryID, t.uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []spin.EntryPhoto{}
	for rows.Next() {
		var p spin.EntryPhoto
		if err := rows.Scan(&p.ID, &p.Position, &p.Key, &p.ContentType, &p.SizeBytes, &p.Source); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// entryExists is nil when the entry is this person's and spin.ErrEntryNotFound
// otherwise.
func (t *pgTx) entryExists(id uuid.UUID) error {
	var one int
	err := t.tx.QueryRow(t.ctx, `select 1 from entries where id = $1 and user_id = $2`, id, t.uid).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return spin.ErrEntryNotFound
	}
	return err
}

func (t *pgTx) insertPhotoRow(entryID uuid.UUID, position int, key, contentType, source string, size *int, now time.Time) error {
	_, err := t.tx.Exec(t.ctx, `
		insert into entry_photos (entry_id, user_id, position, storage_key, content_type, size_bytes, source, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entryID, t.uid, position, key, contentType, size, source, now.UTC())
	return err
}

func (t *pgTx) EntryOfDate(d spin.Date) (*spin.Entry, error) {
	return t.entryQuery(`e.local_date = $2::date`, d.String())
}

func (t *pgTx) EntryByID(id string) (*spin.Entry, error) {
	eid, err := parseID("entry", id)
	if err != nil {
		return nil, err
	}
	e, err := t.entryQuery(`e.id = $2`, eid)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, spin.ErrEntryNotFound
	}
	return e, nil
}

func (t *pgTx) InsertEntry(n spin.NewEntry) (spin.Entry, bool, error) {
	spinID, err := parseID("spin", n.SpinID)
	if err != nil {
		return spin.Entry{}, false, err
	}
	var id uuid.UUID
	err = t.tx.QueryRow(t.ctx, `
		insert into entries (user_id, spin_id, local_date, title, stars, reaction, comment, created_at, updated_at)
		values ($1, $2, $3::date, $4, $5, $6, $7, $8, $8)
		on conflict do nothing
		returning id`,
		t.uid, spinID, n.LocalDate.String(), n.Title, n.Stars, n.Reaction, n.Comment, n.At.UTC()).Scan(&id)
	if err == nil {
		// A new row has no photos yet; Late comes from the date's posted day.
		e, err := t.entryRow(`e.id = $2`, id)
		if err != nil {
			return spin.Entry{}, false, err
		}
		if e == nil {
			return spin.Entry{}, false, errors.New("spin: the new entry could not be read back")
		}
		return *e, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return spin.Entry{}, false, err
	}
	// Either unique key was taken. Read the winner's row in this transaction (a
	// unique violation would have aborted it): by date first, then by spin.
	existing, err := t.entryQuery(`e.local_date = $2::date`, n.LocalDate.String())
	if err != nil {
		return spin.Entry{}, false, err
	}
	if existing == nil {
		if existing, err = t.entryQuery(`e.spin_id = $2`, spinID); err != nil {
			return spin.Entry{}, false, err
		}
	}
	if existing == nil {
		return spin.Entry{}, false, errors.New("spin: entry insert conflicted with another constraint")
	}
	return *existing, false, nil
}

func (t *pgTx) UpdateEntry(id string, u spin.EntryUpdate, now time.Time) error {
	eid, err := parseID("entry", id)
	if err != nil {
		return err
	}
	tag, err := t.tx.Exec(t.ctx, `
		update entries set title = $3, stars = $4, reaction = $5, comment = $6, updated_at = $7
		where id = $1 and user_id = $2`,
		eid, t.uid, u.Title, u.Stars, u.Reaction, u.Comment, now.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return spin.ErrEntryNotFound
	}
	return nil
}

func (t *pgTx) PhotosOfEntry(entryID string) ([]spin.EntryPhoto, error) {
	eid, err := parseID("entry", entryID)
	if err != nil {
		return nil, err
	}
	return t.photosOf(eid)
}

func (t *pgTx) InsertPhotos(entryID string, photos []spin.NewStoredPhoto, now time.Time) error {
	eid, err := parseID("entry", entryID)
	if err != nil {
		return err
	}
	if err := t.entryExists(eid); err != nil {
		return err
	}
	for _, p := range photos {
		if err := t.insertPhotoRow(eid, p.Position, p.Key, p.ContentType, p.Source, p.SizeBytes, now); err != nil {
			return err
		}
	}
	return nil
}

func (t *pgTx) ReplacePhotos(entryID string, slots []spin.PhotoSlot, now time.Time) ([]string, error) {
	eid, err := parseID("entry", entryID)
	if err != nil {
		return nil, err
	}
	if err := t.entryExists(eid); err != nil {
		return nil, err
	}
	if len(slots) > spin.MaxEntryPhotos {
		return nil, errors.New("spin: more than ten photo slots")
	}
	old, err := t.photosOf(eid)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]bool, len(old))
	for _, p := range old {
		stored[p.ID] = true
	}

	// finalPos maps a kept photo's id to its place in the final list.
	finalPos := make(map[string]int, len(slots))
	for pos, s := range slots {
		if s.ID == "" {
			continue
		}
		pid, err := parseID("photo", s.ID)
		if err != nil {
			return nil, err
		}
		id := pid.String()
		if !stored[id] {
			return nil, errors.New("spin: slot names a photo that is not in the entry")
		}
		if _, twice := finalPos[id]; twice {
			return nil, errors.New("spin: slot names the same photo twice")
		}
		finalPos[id] = pos
	}

	// The rows no slot names go first, so a new slot may reuse their keys.
	var removed []string
	var dropIDs []uuid.UUID
	var moveIDs []uuid.UUID
	var movePos []int32
	for _, p := range old {
		id, err := uuid.Parse(p.ID)
		if err != nil {
			return nil, err
		}
		pos, kept := finalPos[p.ID]
		switch {
		case !kept:
			removed = append(removed, p.Key)
			dropIDs = append(dropIDs, id)
		case pos != p.Position:
			moveIDs = append(moveIDs, id)
			movePos = append(movePos, int32(pos))
		}
	}
	if len(dropIDs) > 0 {
		if _, err := t.tx.Exec(t.ctx, `
			delete from entry_photos
			where entry_id = $1 and user_id = $2 and id = any($3::uuid[])`, eid, t.uid, dropIDs); err != nil {
			return nil, err
		}
	}
	// The unique position is deferred, so kept rows may swap in one step.
	if len(moveIDs) > 0 {
		if _, err := t.tx.Exec(t.ctx, `
			update entry_photos p set position = v.position
			from unnest($3::uuid[], $4::int[]) as v(id, position)
			where p.id = v.id and p.entry_id = $1 and p.user_id = $2`,
			eid, t.uid, moveIDs, movePos); err != nil {
			return nil, err
		}
	}
	for pos, s := range slots {
		if s.ID != "" {
			continue
		}
		if err := t.insertPhotoRow(eid, pos, s.Key, s.ContentType, s.Source, s.SizeBytes, now); err != nil {
			return nil, err
		}
	}
	return removed, nil
}

func (t *pgTx) PhotoKeysInUse(keys []string) (map[string]bool, error) {
	used := make(map[string]bool)
	if len(keys) == 0 {
		return used, nil
	}
	// Across all people: a file belongs to one photo row in the whole table.
	rows, err := t.tx.Query(t.ctx, `select storage_key from entry_photos where storage_key = any($1::text[])`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		used[k] = true
	}
	return used, rows.Err()
}

// photoKeysOf reads the storage keys of one of this person's entries in
// position order.
func (t *pgTx) photoKeysOf(entryID uuid.UUID) ([]string, error) {
	rows, err := t.tx.Query(t.ctx, `
		select storage_key from entry_photos
		where entry_id = $1 and user_id = $2
		order by position`, entryID, t.uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (t *pgTx) DeleteEntry(id string) ([]string, error) {
	eid, err := parseID("entry", id)
	if err != nil {
		return nil, err
	}
	// The keys are read first: the photo rows go with the entry by cascade.
	keys, err := t.photoKeysOf(eid)
	if err != nil {
		return nil, err
	}
	tag, err := t.tx.Exec(t.ctx, `delete from entries where id = $1 and user_id = $2`, eid, t.uid)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, spin.ErrEntryNotFound
	}
	return keys, nil
}

func (t *pgTx) SettleSpin(spinID string, result spin.SpinResult, at time.Time) error {
	if result != spin.ResultPosted && result != spin.ResultSkipped {
		return errors.New("spin: a spin is settled as posted or skipped only")
	}
	sid, err := parseID("spin", spinID)
	if err != nil {
		return err
	}
	tag, err := t.tx.Exec(t.ctx, `
		update spins set result = $2, settled_at = $3
		where id = $1 and user_id = $4 and result = 'open'`,
		sid, string(result), at.UTC(), t.uid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return spin.ErrDaySettled
	}
	return nil
}

func (t *pgTx) SkippedDates(from, to spin.Date) ([]spin.Date, error) {
	rows, err := t.tx.Query(t.ctx, `
		select l.local_date::text
		from (
			select distinct on (local_date) local_date, result
			from spins
			where user_id = $1 and local_date between $2::date and $3::date
			order by local_date, seq desc
		) l
		where l.result = 'skipped'
		  and not exists (
			select 1 from posted_days p where p.user_id = $1 and p.local_date = l.local_date
		  )
		order by l.local_date`, t.uid, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []spin.Date
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		d, err := spin.ParseDate(s)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
