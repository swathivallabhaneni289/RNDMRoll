// Package diary holds the read-only rules of the diary and the Profile
// numbers: the paging cursor, the limit, the taste average and the week
// streak, and a service over a Reader. It never writes and never builds a
// link: the Reader returns the entry record (with its storage keys) and the
// HTTP layer makes the links.
package diary

import "github.com/swathivallabhaneni289/RNDMRoll/internal/spin"

// Chip is a category the person has at least one entry in. Name is the
// category's current name (its last name when Removed).
type Chip struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Removed bool   `json:"removed"`
}

// RowKind says what a diary row is.
type RowKind string

const (
	KindEntry   RowKind = "entry"
	KindSkipped RowKind = "skipped"
)

// Row is one diary row. For an entry row Entry is set; for a skipped row
// Entry is nil and Category is the skipped spin's category as written.
type Row struct {
	Kind      RowKind
	Entry     *spin.Entry
	LocalDate string
	Category  spin.CategoryRef
}

// Page is one page of the diary.
type Page struct {
	Rows       []Row
	NextCursor string
	Chips      []Chip
}

// RawTaste is the sum and the count of the stored stars of one category.
type RawTaste struct {
	Chip     Chip
	SumStars int
	Count    int
}

// Taste is one category's average, in tenths (46 is 4.6), and its count.
type Taste struct {
	Category Chip
	Tenths   int
	Count    int
}

// Summary is the Profile page's numbers.
type Summary struct {
	Spins      int
	WeekStreak int
	Followers  int
	Following  int
	Tastes     []Taste
	Recent     []spin.Entry
}
