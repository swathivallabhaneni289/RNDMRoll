// Package admin holds what the admin web page reads and changes: every open
// report with whose it is, one person's reports, the disabled accounts, and
// the actions (dismiss, remove the post or comment, disable or enable the
// account). The page itself is internal/httpapi/admin.go.
package admin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is a person or report that is not there.
var ErrNotFound = errors.New("admin: not found")

// Report kinds.
const (
	KindPerson  = "person"
	KindPost    = "post"
	KindComment = "comment"
)

// Resolutions.
const (
	Dismissed = "dismissed"
	Removed   = "removed"
	Disabled  = "disabled"
)

// ReasonWords are the report reasons as the admin reads them (the same list
// the app shows), on the page and in the report email.
var ReasonWords = map[string]string{
	"spam":          "Spam",
	"nudity":        "Nudity or sexual content",
	"harassment":    "Bullying or harassment",
	"hate":          "Hate speech",
	"violence":      "Violence or threats",
	"impersonation": "Pretending to be someone else",
	"other":         "Something else",
}

// reasonOrder ranks the reasons, the most serious first: violence and nudity
// lead (planned 2026-10-29), then bullying and hate, then the rest.
var reasonOrder = map[string]int{
	"violence": 0, "nudity": 1, "harassment": 2, "hate": 3,
	"impersonation": 4, "spam": 5, "other": 6,
}

// ReasonRank is where reason comes in that order; an unknown one comes last.
func ReasonRank(reason string) int {
	if n, ok := reasonOrder[reason]; ok {
		return n
	}
	return len(reasonOrder)
}

// Urgent reports whether reason is violence or nudity: the page shows these
// first and the email about them goes out at once.
func Urgent(reason string) bool {
	return ReasonRank(reason) <= 1
}

// OpenReportsLimit bounds the open reports the page shows: the newest this
// many.
const OpenReportsLimit = 500

// Person is the reported or disabled person as the admin sees them. It has
// no email address: the page never shows one.
type Person struct {
	ID       uuid.UUID
	Username string
	Name     string
	Bio      string
	// PhotoKey is the profile photo's storage key, "" for none. The page
	// signs its link.
	PhotoKey   string
	CreatedAt  time.Time
	DisabledAt *time.Time
	// DeletionRequestedAt is when the person asked, in the app, for the
	// account to be deleted (nil when they have not). Such an account is not
	// disabled and is not in the disabled list.
	DeletionRequestedAt *time.Time
}

// Report is one report about a person: about them, one of their posts, or
// one of their comments. ID is the report's id; for a report about the
// person it is the reporter's id (one per reporter and person).
type Report struct {
	Kind       string
	ID         uuid.UUID
	Reporter   string
	Reason     string
	CreatedAt  time.Time
	ResolvedAt *time.Time
	Resolution string
	// A post report: the post (nil once it is gone) and its title then. The
	// rest is the post as it is now, and empty once it is gone: its title,
	// its day (YYYY-MM-DD), its text ("" for none) and every photo's key, in
	// order.
	EntryID   *uuid.UUID
	Title     string
	TitleNow  string
	LocalDate string
	Note      string
	PhotoKeys []string
	// A comment report: the comment (nil once it is gone), its text then,
	// and its photo's key ("" for none).
	CommentID *uuid.UUID
	Body      string
	PhotoKey  string
}

// OpenReport is an open report with the person it is about: the one
// reported, or whose post or comment it is.
type OpenReport struct {
	Owner Person
	Report
}

// Repository is the admin page's storage.
type Repository interface {
	// OpenReports is every open report with whose it is, newest first: at
	// most OpenReportsLimit+1 of them, so the page can tell when more are
	// open than it shows.
	OpenReports(ctx context.Context) ([]OpenReport, error)
	// DisabledPeople is every disabled account, the most recently disabled
	// first.
	DisabledPeople(ctx context.Context) ([]Person, error)
	// Person is a person someone reported (about them, a post or a comment,
	// open or closed) or a disabled account; anyone else is ErrNotFound, so
	// the page opens no one else.
	Person(ctx context.Context, id uuid.UUID) (Person, error)
	Reports(ctx context.Context, person uuid.UUID) ([]Report, error)
	// Resolve closes one open report about person. False when there was none.
	Resolve(ctx context.Context, kind string, id, person uuid.UUID, resolution string, at time.Time) (bool, error)
	// OpenReportIDs is the id of every open report on one post or comment
	// (kind KindPost or KindComment). Remove reads them before it deletes
	// the target: once it is gone its reports no longer point at it.
	OpenReportIDs(ctx context.Context, kind string, target uuid.UUID) ([]uuid.UUID, error)
	// ResolveIDs closes the reports of kind with these ids that are still
	// open.
	ResolveIDs(ctx context.Context, kind string, ids []uuid.UUID, resolution string, at time.Time) error
	// ResolvePerson closes every open report about the person.
	ResolvePerson(ctx context.Context, person uuid.UUID, resolution string, at time.Time) error
	// ReportTarget is the post or comment an open-or-closed report points
	// at and whose it is; nil target when it is gone.
	ReportTarget(ctx context.Context, kind string, id uuid.UUID) (target *uuid.UUID, owner uuid.UUID, err error)
	// SetDisabled disables the account at at, or enables it again when at
	// is nil. Disabling ends every session of the account in the same
	// transaction (its refresh tokens are revoked), and deletes its Forgot
	// password code and its push tokens.
	SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time) error
	// DeleteComment deletes a comment and its replies and returns their
	// photo keys, leaving out a file that a post's photo also names.
	DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error)
}
