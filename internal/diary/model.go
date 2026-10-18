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
// Social is set on an entry row with comments.
type Row struct {
	Kind      RowKind
	Entry     *spin.Entry
	LocalDate string
	Category  spin.CategoryRef
	Social    *PostSocial
}

// LatestCount is how many of a post's newest comments PostSocial.Latest holds
// (developer, 2026-10-18: like Instagram, "View all N comments" and a short
// preview, not every comment).
const LatestCount = 1

// PostSocial is the comments on one of the person's own posts, for the posts
// list (developer, 2026-10-17: comments shown under each post, like
// Instagram). It counts what the post page shows its owner, without the
// comments the owner hid: no one with a block either way. Reactions on posts
// were removed (developer, 2026-10-18).
type PostSocial struct {
	// Comments counts the comments and replies; Latest is up to LatestCount
	// of the newest, oldest first (the order they are read in).
	Comments int
	Latest   []LatestComment
}

// LatestComment is one of the newest comments on a post and who wrote it.
// PhotoURL is the writer's profile photo link, nil when they have none.
type LatestComment struct {
	AuthorID string
	Name     string
	Username string
	PhotoURL *string
	Text     string
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
