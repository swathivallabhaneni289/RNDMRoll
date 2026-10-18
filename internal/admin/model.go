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

// Person is the reported person as the admin sees them.
type Person struct {
	ID         uuid.UUID
	Username   string
	Name       string
	Email      string
	Bio        string
	CreatedAt  time.Time
	DisabledAt *time.Time
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
	// A post report: the post (nil once it is gone) and its title then.
	EntryID  *uuid.UUID
	Title    string
	PhotoKey string
	// A comment report: the comment (nil once it is gone) and its text then.
	CommentID *uuid.UUID
	Body      string
}

// Repository is the admin page's storage.
type Repository interface {
	Summaries(ctx context.Context) ([]Summary, error)
	DisabledPeople(ctx context.Context) ([]Summary, error)
	Person(ctx context.Context, id uuid.UUID) (Person, error)
	Reports(ctx context.Context, person uuid.UUID) ([]Report, error)
	// Resolve closes one open report about person. False when there was none.
	Resolve(ctx context.Context, kind string, id, person uuid.UUID, resolution string, at time.Time) (bool, error)
	// ResolveTarget closes every open report on one post or comment.
	ResolveTarget(ctx context.Context, kind string, target uuid.UUID, resolution string, at time.Time) error
	// ResolvePerson closes every open report about the person.
	ResolvePerson(ctx context.Context, person uuid.UUID, resolution string, at time.Time) error
	// ReportTarget is the post or comment an open-or-closed report points
	// at and whose it is; nil target when it is gone.
	ReportTarget(ctx context.Context, kind string, id uuid.UUID) (target *uuid.UUID, owner uuid.UUID, err error)
	SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time) error
	// DeleteComment deletes a comment and its replies and returns their
	// photo keys.
	DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error)
}
