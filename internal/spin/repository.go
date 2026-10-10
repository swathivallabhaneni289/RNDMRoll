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
	// cannot pin the connection pool. fn's error rolls everything back.
	InUserTx(ctx context.Context, userID string, seed Seed, fn func(Tx) error) error
}

// Tx is one person's transaction. The state row is already locked when fn is
// called, and every method is scoped to that person.
//
// Inserts say whether they wrote a row. A second writer that lost a race
// gets the existing row back with inserted false in the same transaction (a
// unique violation would abort a Postgres transaction).
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
	// counts), or nil.
	LatestSpinOfDate(d Date) (*Spin, error)
	// LastDailySpunAt returns when the person's latest daily spin was made,
	// on any date, or nil.
	LastDailySpunAt() (*time.Time, error)
	// InsertSpin stores a spin, ignoring s.ID, s.Result and s.CreatedAt.
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

	// ClearTestData deletes this person's spins, test posted days and goal
	// weeks and sets bonus_held to 0. The wheel, the zone and the seeded_*
	// columns stay.
	ClearTestData() error
}
