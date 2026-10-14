package spintest

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// memEntry is one stored entry with its photo rows. Photos are kept in
// ascending position order. Like the rest of memData it is cloned for every
// transaction, so a callback that fails leaves nothing behind.
type memEntry struct {
	id          string
	spinID      string
	date        spin.Date
	title       string
	stars       int
	reaction    *string
	comment     *string
	createdAt   time.Time
	updatedAt   time.Time
	photos      []memPhoto
	archived    bool
	commentsOff bool
}

// memPhoto is one photo row. The storage key is never a link.
type memPhoto struct {
	id          string
	position    int
	key         string
	contentType string
	sizeBytes   *int
	source      string
	createdAt   time.Time
}

func copyText(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func copyNumber(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// cloneEntries copies entries deeply: the optional texts, the photo rows and
// each declared size are new values.
func cloneEntries(src []memEntry) []memEntry {
	if src == nil {
		return nil
	}
	out := make([]memEntry, len(src))
	for i, e := range src {
		e.reaction = copyText(e.reaction)
		e.comment = copyText(e.comment)
		photos := make([]memPhoto, len(e.photos))
		for j, p := range e.photos {
			p.sizeBytes = copyNumber(p.sizeBytes)
			photos[j] = p
		}
		e.photos = photos
		out[i] = e
	}
	return out
}

// checkEntryValues holds the rules of the entries table's check constraints,
// so a service bug shows up in the in-memory run as it would in Postgres.
func checkEntryValues(title string, stars int, reaction, comment *string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(title)); n < 1 || n > spin.MaxTitleRunes {
		return errors.New("spintest: entry title is not 1 to 80 characters")
	}
	if stars < spin.StarsMin || stars > spin.StarsMax {
		return errors.New("spintest: entry stars are not 1 to 10")
	}
	if reaction != nil && !spin.IsReaction(*reaction) {
		return errors.New("spintest: entry reaction is not one of the six names")
	}
	if comment != nil && utf8.RuneCountInString(*comment) > spin.MaxCommentRunes {
		return errors.New("spintest: entry comment is longer than 140 characters")
	}
	return nil
}

// checkPhotoValues holds the rules of the entry_photos table's check
// constraints (other than the position and key uniqueness).
func checkPhotoValues(key, contentType, source string, size *int) error {
	if n := utf8.RuneCountInString(key); n < 1 || n > spin.MaxPhotoKeyBytes {
		return errors.New("spintest: photo key is not 1 to 200 characters")
	}
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return errors.New("spintest: photo content type is not allowed")
	}
	if source != spin.PhotoSourceCamera && source != spin.PhotoSourceLibrary {
		return errors.New("spintest: photo source is not camera or library")
	}
	if size != nil && (*size < 1 || *size > spin.MaxPhotoSizeBytes) {
		return errors.New("spintest: photo size is out of range")
	}
	return nil
}

func (t *memTx) entryIndex(id string) int {
	for i := range t.d.entries {
		if t.d.entries[i].id == id {
			return i
		}
	}
	return -1
}

// keysHeld returns every storage key held by a photo row of this person,
// leaving out the rows of the entry at index skip (-1 leaves out none).
func (t *memTx) keysHeld(skip int) map[string]bool {
	held := make(map[string]bool)
	for i := range t.d.entries {
		if i == skip {
			continue
		}
		for _, p := range t.d.entries[i].photos {
			held[p.key] = true
		}
	}
	return held
}

// entryView builds the Entry a read returns: photos in position order, Late
// from the date's posted-day row (false when none), and the category id and
// written name from the entry's spin. Every value is a copy.
func (t *memTx) entryView(e memEntry) spin.Entry {
	out := spin.Entry{
		ID:          e.id,
		SpinID:      e.spinID,
		LocalDate:   e.date,
		Title:       e.title,
		Stars:       e.stars,
		Reaction:    copyText(e.reaction),
		Comment:     copyText(e.comment),
		CreatedAt:   e.createdAt,
		UpdatedAt:   e.updatedAt,
		Archived:    e.archived,
		CommentsOff: e.commentsOff,
		Photos:      make([]spin.EntryPhoto, len(e.photos)),
	}
	if p, ok := t.d.posted[e.date]; ok {
		out.Late = p.Late
	}
	for _, s := range t.d.spins {
		if s.ID == e.spinID {
			out.CategoryID, out.CategoryName = s.CategoryID, s.CategoryName
			break
		}
	}
	for i, p := range e.photos {
		out.Photos[i] = spin.EntryPhoto{
			ID:          p.id,
			Position:    p.position,
			Key:         p.key,
			ContentType: p.contentType,
			SizeBytes:   copyNumber(p.sizeBytes),
			Source:      p.source,
		}
	}
	return out
}

func (t *memTx) EntryOfDate(d spin.Date) (*spin.Entry, error) {
	for _, e := range t.d.entries {
		if e.date == d {
			v := t.entryView(e)
			return &v, nil
		}
	}
	return nil, nil
}

func (t *memTx) EntryByID(id string) (*spin.Entry, error) {
	i := t.entryIndex(id)
	if i < 0 {
		return nil, spin.ErrEntryNotFound
	}
	v := t.entryView(t.d.entries[i])
	return &v, nil
}

func (t *memTx) InsertEntry(n spin.NewEntry) (spin.Entry, bool, error) {
	for _, e := range t.d.entries {
		if e.date == n.LocalDate || e.spinID == n.SpinID {
			return t.entryView(e), false, nil
		}
	}
	spinFound := false
	for _, s := range t.d.spins {
		if s.ID == n.SpinID {
			spinFound = true
			break
		}
	}
	if !spinFound {
		return spin.Entry{}, false, errors.New("spintest: entry names a spin that does not exist")
	}
	if err := checkEntryValues(n.Title, n.Stars, n.Reaction, n.Comment); err != nil {
		return spin.Entry{}, false, err
	}
	e := memEntry{
		id:        uuid.NewString(),
		spinID:    n.SpinID,
		date:      n.LocalDate,
		title:     n.Title,
		stars:     n.Stars,
		reaction:  copyText(n.Reaction),
		comment:   copyText(n.Comment),
		createdAt: n.At,
		updatedAt: n.At,
	}
	t.d.entries = append(t.d.entries, e)
	return t.entryView(e), true, nil
}

func (t *memTx) UpdateEntry(id string, u spin.EntryUpdate, now time.Time) error {
	i := t.entryIndex(id)
	if i < 0 {
		return spin.ErrEntryNotFound
	}
	if err := checkEntryValues(u.Title, u.Stars, u.Reaction, u.Comment); err != nil {
		return err
	}
	e := &t.d.entries[i]
	e.title, e.stars = u.Title, u.Stars
	e.reaction, e.comment = copyText(u.Reaction), copyText(u.Comment)
	e.commentsOff = u.CommentsOff
	e.updatedAt = now
	return nil
}

func (t *memTx) SetEntryArchived(id string, archived bool, now time.Time) error {
	i := t.entryIndex(id)
	if i < 0 {
		return spin.ErrEntryNotFound
	}
	t.d.entries[i].archived = archived
	return nil
}

func (t *memTx) ArchivedEntries() ([]spin.Entry, error) {
	out := []spin.Entry{}
	for _, e := range t.d.entries {
		if e.archived {
			out = append(out, t.entryView(e))
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[b].LocalDate.Before(out[a].LocalDate) })
	return out, nil
}

func (t *memTx) PhotosOfEntry(entryID string) ([]spin.EntryPhoto, error) {
	i := t.entryIndex(entryID)
	if i < 0 {
		return nil, nil
	}
	v := t.entryView(t.d.entries[i])
	return v.Photos, nil
}

func (t *memTx) InsertPhotos(entryID string, photos []spin.NewStoredPhoto, now time.Time) error {
	i := t.entryIndex(entryID)
	if i < 0 {
		return spin.ErrEntryNotFound
	}
	held := t.keysHeld(-1)
	positions := make(map[int]bool, len(t.d.entries[i].photos)+len(photos))
	for _, p := range t.d.entries[i].photos {
		positions[p.position] = true
	}
	added := make([]memPhoto, 0, len(photos))
	for _, p := range photos {
		if p.Position < 0 || p.Position >= spin.MaxEntryPhotos {
			return errors.New("spintest: photo position is not 0 to 9")
		}
		if positions[p.Position] {
			return errors.New("spintest: photo position is used twice in the entry")
		}
		if err := checkPhotoValues(p.Key, p.ContentType, p.Source, p.SizeBytes); err != nil {
			return err
		}
		if held[p.Key] {
			return errors.New("spintest: photo key already belongs to a photo row")
		}
		positions[p.Position] = true
		held[p.Key] = true
		added = append(added, memPhoto{
			id:          uuid.NewString(),
			position:    p.Position,
			key:         p.Key,
			contentType: p.ContentType,
			sizeBytes:   copyNumber(p.SizeBytes),
			source:      p.Source,
			createdAt:   now,
		})
	}
	merged := append(append([]memPhoto(nil), t.d.entries[i].photos...), added...)
	sort.SliceStable(merged, func(a, b int) bool { return merged[a].position < merged[b].position })
	t.d.entries[i].photos = merged
	return nil
}

func (t *memTx) ReplacePhotos(entryID string, slots []spin.PhotoSlot, now time.Time) ([]string, error) {
	i := t.entryIndex(entryID)
	if i < 0 {
		return nil, spin.ErrEntryNotFound
	}
	if len(slots) > spin.MaxEntryPhotos {
		return nil, errors.New("spintest: more than ten photo slots")
	}
	old := t.d.entries[i].photos
	byID := make(map[string]memPhoto, len(old))
	for _, p := range old {
		byID[p.id] = p
	}

	// Keys that stay taken: every row of the person's other entries and every
	// row this call keeps. A row that goes frees its key for a new slot.
	held := t.keysHeld(i)
	next := make([]memPhoto, len(slots))
	kept := make(map[string]bool, len(slots))
	for pos, s := range slots {
		if s.ID == "" {
			continue
		}
		p, ok := byID[s.ID]
		if !ok || kept[s.ID] {
			return nil, errors.New("spintest: slot names a photo that is not in the entry")
		}
		kept[s.ID] = true
		p.position = pos
		next[pos] = p
		held[p.key] = true
	}
	for pos, s := range slots {
		if s.ID != "" {
			continue
		}
		if err := checkPhotoValues(s.Key, s.ContentType, s.Source, s.SizeBytes); err != nil {
			return nil, err
		}
		if held[s.Key] {
			return nil, errors.New("spintest: photo key already belongs to a photo row")
		}
		held[s.Key] = true
		next[pos] = memPhoto{
			id:          uuid.NewString(),
			position:    pos,
			key:         s.Key,
			contentType: s.ContentType,
			sizeBytes:   copyNumber(s.SizeBytes),
			source:      s.Source,
			createdAt:   now,
		}
	}

	var removed []string
	for _, p := range old {
		if !kept[p.id] {
			removed = append(removed, p.key)
		}
	}
	t.d.entries[i].photos = next
	return removed, nil
}

func (t *memTx) PhotoKeysInUse(keys []string) (map[string]bool, error) {
	held := t.keysHeld(-1)
	out := make(map[string]bool)
	for _, k := range keys {
		if held[k] {
			out[k] = true
		}
	}
	return out, nil
}

func (t *memTx) DeleteEntry(id string) ([]string, error) {
	i := t.entryIndex(id)
	if i < 0 {
		return nil, spin.ErrEntryNotFound
	}
	keys := make([]string, len(t.d.entries[i].photos))
	for j, p := range t.d.entries[i].photos {
		keys[j] = p.key
	}
	kept := make([]memEntry, 0, len(t.d.entries)-1)
	kept = append(kept, t.d.entries[:i]...)
	kept = append(kept, t.d.entries[i+1:]...)
	t.d.entries = kept
	return keys, nil
}

func (t *memTx) SettleSpin(spinID string, result spin.SpinResult, at time.Time) error {
	if result != spin.ResultPosted && result != spin.ResultSkipped {
		return errors.New("spintest: a spin is settled as posted or skipped only")
	}
	for i := range t.d.spins {
		if t.d.spins[i].ID != spinID {
			continue
		}
		if t.d.spins[i].Result != spin.ResultOpen {
			return spin.ErrDaySettled
		}
		stamp := at
		t.d.spins[i].Result = result
		t.d.spins[i].SettledAt = &stamp
		return nil
	}
	return spin.ErrDaySettled
}

func (t *memTx) SkippedDates(from, to spin.Date) ([]spin.Date, error) {
	latest := make(map[spin.Date]spin.Spin)
	for _, s := range t.d.spins {
		if s.LocalDate.Before(from) || s.LocalDate.After(to) {
			continue
		}
		if cur, ok := latest[s.LocalDate]; !ok || s.Seq > cur.Seq {
			latest[s.LocalDate] = s
		}
	}
	var out []spin.Date
	for d, s := range latest {
		if s.Result != spin.ResultSkipped {
			continue
		}
		if _, hasPosted := t.d.posted[d]; hasPosted {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}
