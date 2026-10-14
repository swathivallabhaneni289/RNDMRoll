package spin

import (
	"context"
	"time"
)

// Seed is what a person's state row is made from the first time anything
// needs it. At is the service clock's instant, LocalDate and TZName the date
// and zone then (UTC when no zone has been reported yet), Goal the starting
// weekly goal. The repository uses it only when it inserts the row.
type Seed struct {
	At        time.Time
	LocalDate Date
	TZName    string
	Goal      int
}

// WheelRow is one row of a wheel change, in wheel order. ID is empty for a
// new category and the id of a live category otherwise.
type WheelRow struct {
	ID       string
	Name     string
	Weight   int
	Position int
}

// WheelChange is a whole wheel edit: the rows that make up the new live
// wheel, and the ids of live categories it leaves out (they are archived).
type WheelChange struct {
	Rows    []WheelRow
	Archive []string
}

// NewEntry is the entry row InsertEntry stores. SpinID is the id of the
// date's latest spin (the entry belongs to it) and LocalDate is that spin's
// date. The entry copies neither the category id nor its name: both are read
// from the spin. At becomes both created_at and updated_at, so it comes from
// the service clock and never from the database.
type NewEntry struct {
	SpinID    string
	LocalDate Date
	Title     string
	Stars     int
	Reaction  *string
	Comment   *string
	At        time.Time
}

// EntryUpdate is the full set of scalar values UpdateEntry writes: the
// service passes the merged values (the stored ones with the patch applied),
// so a nil Reaction or Comment means the column becomes null.
type EntryUpdate struct {
	Title    string
	Stars    int
	Reaction *string
	Comment  *string
	// CommentsOff is the owner's comments switch, written with the rest.
	CommentsOff bool
}

// NewStoredPhoto is one photo row InsertPhotos writes. Position is its place
// in the entry (0 is the cover). ContentType is already set by the service
// from the key's extension. Source is PhotoSourceCamera or
// PhotoSourceLibrary. SizeBytes is what the phone declared, or nil.
type NewStoredPhoto struct {
	Position    int
	Key         string
	ContentType string
	Source      string
	SizeBytes   *int
}

// PhotoSlot is one place in the final photo list that ReplacePhotos writes.
// The index of the slot in the list is its final position. When ID is set the
// slot keeps that existing photo row of the entry and the other fields are
// ignored; when ID is empty the slot is a new row made from Key, ContentType,
// Source and SizeBytes.
type PhotoSlot struct {
	ID          string
	Key         string
	ContentType string
	Source      string
	SizeBytes   *int
}

// Repository is the storage contract for the spin slice. It is separate from
// user.Repository and touches no users column.
type Repository interface {
	// Zone reads a person's stored time zone name outside any transaction.
	// ok is false when none is stored. The service uses it to decide the
	// seed and to refuse a person with no zone before taking any lock.
	Zone(ctx context.Context, userID string) (name string, ok bool, err error)

	// InUserTx runs fn inside one transaction that holds the person's state
	// row lock. If the row does not exist it is made first, from seed, with
	// the six starting categories (only the caller that inserts the row
	// seeds; a concurrent caller waits and then finds them). The lock waits
	// at most 5 seconds and then fails with ErrBusy, so a stuck holder
	// cannot pin the connection pool. fn's error undoes everything.
	InUserTx(ctx context.Context, userID string, seed Seed, fn func(Tx) error) error
}

// Tx is one person's transaction. The state row is already locked when fn is
// called, and every method is scoped to that person.
//
// Inserts say whether they wrote a row. A second writer that lost a race
// gets the existing row back with inserted false in the same transaction (a
// unique violation would abort a Postgres transaction).
//
// A Spin read from any method carries SettledAt: the instant its result became
// posted or skipped, or nil while it is open (and for a day settled only by a
// pretend posted day, which sets no spin result).
type Tx interface {
	// State returns the state row as it is now in this transaction.
	State() (UserState, error)

	// Zone returns the stored zone name; ok is false when none is stored.
	Zone() (name string, ok bool, err error)
	// SetZone stores the zone name, replacing any earlier one.
	SetZone(name string) error

	// LiveCategories returns the wheel in order, archived ones left out.
	LiveCategories() ([]Category, error)
	// CategoryCount returns every category row the person has, archived
	// ones included.
	CategoryCount() (int, error)
	// ExistingCategoryIDs returns which of ids still exist as rows (live or
	// archived) for this person.
	ExistingCategoryIDs(ids []string) (map[string]bool, error)
	// ApplyWheel writes a wheel change: updates and inserts the rows, in
	// order, and archives the ids left out, stamped now.
	ApplyWheel(change WheelChange, now time.Time) error
	// DeleteUnusedArchived deletes this person's archived categories that no
	// spin refers to, and returns how many went.
	DeleteUnusedArchived() (int, error)
	// BumpWheelVersion adds one to the wheel version and returns it.
	BumpWheelVersion() (int, error)

	// DailySpinOfDate returns the date's daily spin (seq 0), or nil.
	DailySpinOfDate(d Date) (*Spin, error)
	// LatestSpinOfDate returns the date's spin with the highest seq (a bonus
	// counts), or nil. Result is the stored value (a posted-day row does not
	// change it here).
	LatestSpinOfDate(d Date) (*Spin, error)
	// LastDailySpunAt returns when the person's latest daily spin was made,
	// on any date, or nil.
	LastDailySpunAt() (*time.Time, error)
	// InsertSpin stores a spin, ignoring s.ID, s.Result, s.SettledAt and
	// s.CreatedAt (a new spin is open and not settled).
	InsertSpin(s Spin) (stored Spin, inserted bool, err error)

	// SpendBonus takes the held bonus spin: ErrNoBonus when none is held.
	SpendBonus() error
	// GrantBonus makes a bonus spin held when none is, and reports whether
	// it did.
	GrantBonus() (granted bool, err error)

	// PostedDay returns the date's posted-day row, or nil.
	PostedDay(d Date) (*PostedDay, error)
	// PostedDays returns the posted days from from to to, both included.
	PostedDays(from, to Date) ([]PostedDay, error)
	// InsertPostedDay stores a posted day; inserted is false when the date
	// already had one (whatever its source).
	InsertPostedDay(p PostedDay) (inserted bool, err error)
	// InsertGoalWeek stores a goal week; inserted is false when the week
	// already had one.
	InsertGoalWeek(g GoalWeek) (inserted bool, err error)

	// --- entries and their photos ---
	//
	// Every id the service passes to the methods below (an entry id, a photo
	// slot id, a spin id) is a canonical lower-case 36 character UUID: it was
	// either read from the repository or checked with canonicalUUID first. A
	// repository may parse it with uuid.Parse and treat a failure as a bug.
	// Storage keys are plain text, never links.

	// EntryOfDate returns this person's entry for the local date d, or nil
	// when there is none. The Entry is complete: Photos holds every photo row
	// in ascending position order (position 0, the cover, first); Late is the
	// late flag of the date's posted-day row, whatever its source (false when
	// the date has no posted-day row); CategoryID and CategoryName are the
	// category id and the name as written on the entry's own spin, because the
	// entry stores neither. Another person's rows are never read.
	EntryOfDate(d Date) (*Entry, error)

	// EntryByID returns this person's entry with that id, complete as
	// EntryOfDate describes it. An id with no entry, or whose entry belongs to
	// another person, is ErrEntryNotFound. The caller always passes a
	// canonical lower-case 36 character UUID, so the repository does not need
	// to accept any other spelling.
	EntryByID(id string) (*Entry, error)

	// InsertEntry stores the entry row only (no photo rows: the caller adds
	// them with InsertPhotos). created_at and updated_at are both n.At. It is
	// insert-if-absent over BOTH unique keys, one entry per local date and one
	// per spin, with no conflict target: when this person already has an entry
	// for n.LocalDate or for n.SpinID the row is not written, inserted is
	// false, and the existing entry is returned, read inside the same
	// transaction (a unique violation would abort a Postgres transaction). The
	// returned Entry is complete as EntryOfDate returns it at that moment: for
	// a new row it has no photos yet and Late from the date's posted-day row
	// (false when the day has none yet); for the loser of a race it is the
	// winner's entry with its photos in position order.
	InsertEntry(n NewEntry) (entry Entry, inserted bool, err error)

	// UpdateEntry writes title, stars, reaction, comment and updated_at = now
	// on this person's entry, and nothing else (not the date, the spin, the
	// photos or created_at). A nil Reaction or Comment makes the column null.
	// ErrEntryNotFound when no such entry belongs to this person.
	UpdateEntry(id string, u EntryUpdate, now time.Time) error

	// PhotosOfEntry returns the photo rows of one of this person's entries in
	// ascending position order. An entry with none, or an id that is not this
	// person's entry, gives an empty list and no error.
	PhotosOfEntry(entryID string) ([]EntryPhoto, error)

	// InsertPhotos stores new photo rows for one of this person's entries,
	// each at its Position, with a new id and created_at = now. The entry must
	// exist and be this person's (ErrEntryNotFound otherwise). A key that
	// already belongs to a photo row, a position outside 0 to 9 and a position
	// used twice in the entry are errors (the service checks first, so these
	// are bugs, not sentinels).
	InsertPhotos(entryID string, photos []NewStoredPhoto, now time.Time) error

	// ReplacePhotos makes the entry's photos exactly the list slots, in one
	// step: the index of a slot is its final position (0 to n-1, no gaps). A
	// slot with ID set keeps that photo row (its key, content type, size and
	// source stay as stored; only its position may change). A slot with an
	// empty ID is a new row (new id, created_at = now). Photo rows of the
	// entry that no slot names are deleted first, so a new slot may reuse the
	// key of a row that goes. The kept rows may be swapped freely (the unique
	// position is not checked until the step is done). It returns the storage
	// keys of the deleted rows, in the order of their old positions (empty
	// when none). It does not change the entry row, so updated_at is
	// UpdateEntry's. ErrEntryNotFound when the entry is not this person's. A
	// slot ID that is not a photo of this entry, a list longer than ten and a
	// key already held by another photo row are errors (the service checks
	// first, so these are bugs, not sentinels).
	ReplacePhotos(entryID string, slots []PhotoSlot, now time.Time) (removedKeys []string, err error)

	// PhotoKeysInUse says which of keys already belong to a photo row (a key
	// can belong to one row only). The map holds a key only when it is in use;
	// an empty list gives an empty map. Postgres answers across ALL people
	// (the key is unique in the whole table). MemRepo answers from the calling
	// person's own data only, because it never takes another person's lock.
	PhotoKeysInUse(keys []string) (map[string]bool, error)

	// DeleteEntry removes this person's entry and, with it, its photo rows. It
	// reads the entry's storage keys first and returns ALL of them in position
	// order. ErrEntryNotFound when no such entry belongs to this person. It
	// leaves the posted day, the spin and its result, and the goal weeks as
	// they are.
	DeleteEntry(id string) (removedKeys []string, err error)

	// SetEntryArchived archives (true) or unarchives (false) this person's
	// entry. It is idempotent: archiving an archived entry keeps its first
	// archive time. It changes nothing else (not updated_at, the posted day,
	// the spin or the photos). ErrEntryNotFound when no such entry belongs to
	// this person.
	SetEntryArchived(id string, archived bool, now time.Time) error

	// ArchivedEntries returns this person's archived entries, complete as
	// EntryOfDate describes them, newest local date first.
	ArchivedEntries() ([]Entry, error)

	// SettleSpin sets one of this person's spins to result (ResultPosted or
	// ResultSkipped) with settled_at = at, but only while its stored result is
	// still ResultOpen. Zero rows changed (the spin is not open, is not this
	// person's, or does not exist) is ErrDaySettled.
	SettleSpin(spinID string, result SpinResult, at time.Time) error

	// SkippedDates returns, in ascending order, the dates from from to to
	// (both included) whose LATEST spin (highest seq, a bonus counts) has the
	// stored result skipped and that have no posted-day row of any source.
	SkippedDates(from, to Date) ([]Date, error)

	// ClearTestData deletes this person's entries (their photo rows go with
	// them), then ALL of their posted days of any source, then their spins and
	// goal weeks, and sets bonus_held to 0. The wheel, the zone and the
	// seeded_* columns stay. Photo files in storage are not touched: the
	// repository knows only keys.
	ClearTestData() error
}
