// Package admin holds what the admin web page reads and changes: open
// reports grouped by the reported person, one person's reports, and the
// actions (dismiss, remove the post or comment, disable or enable the
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

// Summary is one reported person on the first page: how many open reports
// name them, the reasons, and the newest.
type Summary struct {
	PersonID uuid.UUID
	Username string
	Name     string
	Disabled bool
	Open     int
	Reasons  []string
	LatestAt time.Time
}

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

// Repository is the admin page's storage.
type Repository interface {
	Summaries(ctx context.Context) ([]Summary, error)
	DisabledPeople(ctx context.Context) ([]Summary, error)
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
	// photo keys.
	DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error)
}
