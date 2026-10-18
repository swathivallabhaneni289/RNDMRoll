// Package board is the Board (posts of the people you follow, opened day by
// day by your own posts), the post page, reactions and comments. Every read
// of another person's entry goes through social.CanView.
package board

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

var (
	// ErrNotFound: the post or comment does not exist or the caller may not
	// see it. The two are never told apart.
	ErrNotFound = errors.New("board: not found")
	// ErrOwnPost: a reaction on your own post.
	ErrOwnPost = errors.New("board: own post")
	// ErrCommentsOff: the owner turned comments off.
	ErrCommentsOff = errors.New("board: comments off")
)

// FieldProblem is a request field that failed a rule.
type FieldProblem struct {
	Field  string
	Reason string
}

func (p *FieldProblem) Error() string { return "board: " + p.Field + " " + p.Reason }

// Board states.
const (
	StateNoFollows = "no_follows"
	StateLocked    = "locked"
	StateSkipped   = "skipped"
	StateUnlocked  = "unlocked"
)

// Labels.
const (
	DayLabelToday     = "today"
	DayLabelLastNight = "last_night"

	LabelToday     = "today"
	LabelYesterday = "yesterday"
	LabelEarlier   = "earlier"
)

// Limits.
const (
	MaxCommentRunes = 140
	DefaultDays     = 5
	MaxDays         = 10
	MaxNames        = 4
)

// EntryRow is an entry as the board reads it: the record, its owner and its
// photos' storage keys (never sent; links are built in the service).
type EntryRow struct {
	ID           uuid.UUID
	Owner        social.Person
	LocalDate    string
	CategoryID   string
	CategoryName string
	// TZName is the zone the entry's spin was made in (the author's own).
	TZName      string
	Title       string
	Stars       int
	Reaction    *string
	Note        *string
	Late        bool
	CreatedAt   time.Time
	Archived    bool
	CommentsOff bool
	// Photos are in position order; the first is the cover.
	Photos []PhotoRow
}

// PhotoRow is one photo of an entry.
type PhotoRow struct {
	ID       string
	Key      string
	Position int
}

// CommentRow is a stored comment with its writer and its entry's owner.
type CommentRow struct {
	ID         uuid.UUID
	EntryID    uuid.UUID
	Author     social.Person
	EntryOwner uuid.UUID
	Body       string
	CreatedAt  time.Time
	// ParentID is the top-level comment a reply belongs to; nil for a
	// top-level comment. Replies are one level: a parent is never a reply.
	ParentID *uuid.UUID
	// Hidden: the post's owner hid it (only its writer and the owner read it).
	Hidden bool
	// PhotoKey is the comment's one photo (migration 0013), nil for none.
	PhotoKey *string
}

// Card is one post on the Board.
type Card struct {
	EntryID         uuid.UUID
	Author          social.Person
	Mine            bool
	LocalDate       string
	CategoryID      string
	CategoryName    string
	Title           string
	Stars           int
	Reaction        *string
	PhotoURL        string
	Late            bool
	PostedAt        time.Time
	PostedLocalTime string
	CommentCount    int
	MyReaction      *string
}

// Photo is a photo link of a post.
type Photo struct {
	ID       string
	URL      string
	Position int
}

// Comment is a comment as the viewer sees it.
type Comment struct {
	ID        uuid.UUID
	Author    social.Person
	Text      string
	At        time.Time
	AtLabel   string
	CanDelete bool
	// CanHide: the viewer owns the post and did not write the comment.
	CanHide bool
	// Hidden: the post's owner hid it. Only its writer and the owner get it,
	// and the writer is never told (Hidden is false for them).
	Hidden bool
	// PhotoURL is the link of the comment's photo, "" for none.
	PhotoURL string
	// ParentID is set on a reply: the top-level comment it sits under.
	ParentID *uuid.UUID
}

// Received is one reaction on the viewer's own post.
type Received struct {
	Person   social.Person
	Reaction string
	At       time.Time
}

// Post is the post page.
type Post struct {
	Card        Card
	Note        *string
	Photos      []Photo
	Comments    []Comment
	CanReact    bool
	CanComment  bool
	CommentsOff bool
	// Received is set only on the viewer's own post.
	Received []Received
}

// Locked is the locked card's data.
type Locked struct {
	PostedCount int
	Names       []string
	Category    *string
	Action      string
}

// Day is one day of cards.
type Day struct {
	LocalDate string
	Label     string
	Cards     []Card
}

// Board is one page of the Board.
type Board struct {
	State      string
	DayLabel   string
	Locked     *Locked
	Days       []Day
	NextBefore *string
}
