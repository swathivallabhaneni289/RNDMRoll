// Package admin holds what the admin web page reads and changes: every open
// report with whose it is, one person's reports, the disabled accounts, what
// was removed and the second looks asked about it, the decision log, and the
// actions (dismiss, remove the post or comment, bring it back or keep it
// removed, disable or enable the account). The page itself is
// internal/httpapi/admin.go.
package admin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is a person, report, post or comment that is not there. For an
// action on a post or comment it is also one the action cannot reach: Remove
// finds it removed past RemovalGrace, and Bring it back finds it past the
// grace, or not removed (unless ErrAlreadyDone).
var ErrNotFound = errors.New("admin: not found")

// ErrAlreadyDone is a Remove or a Bring it back that finds its own work done,
// as the second click of a double click does: the post or comment is removed
// already inside RemovalGrace, or back already by a Bring it back (the newest
// decision about it). Nothing changes and nothing is logged.
var ErrAlreadyDone = errors.New("admin: already done")

// ErrPostRemoved and ErrParentRemoved are a Bring it back of a comment that
// would still be gone for everyone: its post, or the comment it answers, is
// removed too. Nothing changes, and nothing is logged: that one must come
// back first. With both removed, it is ErrPostRemoved.
var (
	ErrPostRemoved   = errors.New("admin: its post is removed")
	ErrParentRemoved = errors.New("admin: the comment it answers is removed")
)

// ErrNoRequest is a Keep it removed with no open request for a second look,
// or for one that is not the request the page showed: a page drawn before a
// Bring it back answered it, so it is back, or removed again since and
// maybe asked about again. Nothing changes, and nothing is logged.
var ErrNoRequest = errors.New("admin: no second look asked")

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

// RemovalGrace is how long a post or comment an admin removed is kept
// (developer, 2026-11-17): hidden from everyone, its owner too, and back with
// Bring it back until then. After it the purge deletes it and its photos for
// good. The line is on the database's clock.
const RemovalGrace = 30 * 24 * time.Hour

// The decision log (migration 0020's moderation_events): what an admin
// decided about a post, a comment or an account. Item is KindPost,
// KindComment or ItemAccount. LogHidden is the one row no admin writes: the
// report that hides a post or comment logs it, so its owner hears of it
// (developer, 2026-11-18). LogKept is a Keep it removed, the answer to a
// second look that leaves it removed (developer, 2026-11-19).
const (
	ItemAccount = "account"

	LogRemoved    = "removed"
	LogRestored   = "restored"
	LogShownAgain = "shown_again"
	LogDisabled   = "disabled"
	LogEnabled    = "enabled"
	LogHidden     = "hidden"
	LogKept       = "kept"
)

// The answers to a second look (migration 0021's second_looks.decision):
// none yet, Bring it back, or Keep it removed.
const (
	LookOpen     = ""
	LookRestored = "restored"
	LookKept     = "kept"
)

// Ref is the short reference of a decision: the first 8 hex characters of
// its id in the log. The Removed tab, the person's history and the owner's
// bell item about a removal all show the removal's, so the admin can match a
// second look, or an email about one, to its removal.
func Ref(event uuid.UUID) string { return event.String()[:8] }

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

// PostsLimit bounds the posts a person's page shows: the newest this many.
const PostsLimit = 60

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
	// RemovedBefore is how many of their posts and comments an admin removed
	// and did not bring back, read from the decision log: the purge and a
	// reporter's deleted account take nothing from it.
	RemovedBefore int
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
	// Hidden says the post or comment is hidden while under review now
	// (social.ReviewThreshold open reports from accounts that are not gone),
	// worked out in SQL by the same rule as every read of the app. Never on
	// a closed report, a report about a person, or a post or comment that is
	// gone or that a removal keeps from everyone.
	Hidden bool
}

// OpenReport is an open report with the person it is about: the one
// reported, or whose post or comment it is.
type OpenReport struct {
	Owner Person
	Report
}

// Post is one of a person's posts as their page shows it, as it is now.
type Post struct {
	ID    uuid.UUID
	Title string
	// Category is the wheel category its spin landed on, LocalDate its day
	// (YYYY-MM-DD), Note its text ("" for none) and PhotoKeys every photo's
	// key, in order.
	Category  string
	LocalDate string
	Note      string
	PhotoKeys []string
	// Removed says an admin removed it inside RemovalGrace; Hidden says it
	// is hidden while under review now (never with Removed); Reported says
	// a report on it is open.
	Removed  bool
	Hidden   bool
	Reported bool
}

// ShowsPosts reports whether a person's page shows their posts: while a
// report about the account itself is open, or while the account is disabled,
// as Disable closes those reports and Enable is decided on what they posted
// (developer, 2026-11-18). A report about one post or comment shows that one
// only, on its card.
func ShowsPosts(p Person, reports []Report) bool {
	if p.DisabledAt != nil {
		return true
	}
	for _, r := range reports {
		if r.Kind == KindPerson && r.ResolvedAt == nil {
			return true
		}
	}
	return false
}

// RemovedItem is a post or comment an admin removed inside RemovalGrace, as
// the Removed tab shows it: as it is, with whose it is and why it went.
type RemovedItem struct {
	// Kind is KindPost or KindComment, and ID the post's or the comment's.
	Kind  string
	ID    uuid.UUID
	Owner Person
	// A post: its title, its day (YYYY-MM-DD), its text ("" for none) and
	// every photo's key, in order.
	Title     string
	LocalDate string
	Note      string
	PhotoKeys []string
	// A comment: its text and its photo's key ("" for none).
	Body     string
	PhotoKey string
	// RemovedAt is when it was removed and DaysLeft the whole days (rounded
	// up) until RemovalGrace ends, both on the database's clock. For a
	// comment, an earlier removal of its post or of the comment it answers
	// ends it sooner, as that one's purge takes it too.
	RemovedAt time.Time
	DaysLeft  int
	// Reasons, Reason and Ref come from its removal in the decision log: the
	// reasons of the reports Remove closed (the most serious first), the
	// reason the admin gave (a report reason's key, "" for none) and the
	// removal's Ref. All are empty when the log has no removal for it.
	Reasons []string
	Reason  string
	Ref     string
	// SecondLook is the owner's open request for a second look at that
	// removal, nil while none is open (developer, 2026-11-19): the tab lists
	// these first, with their words and photos and Keep it removed.
	SecondLook *SecondLook
}

// SecondLook is an owner's request for a second look at a removal of their
// post or comment (migration 0021's second_looks), as the admin page shows
// it.
type SecondLook struct {
	ID uuid.UUID
	// Item is KindPost or KindComment, ItemID the post's or the comment's id
	// (kept after the purge), and RemovalID the removal's row in the
	// decision log, whose Ref the page shows.
	Item      string
	ItemID    uuid.UUID
	RemovalID uuid.UUID
	// Body is their words, trimmed. PhotoKeys are their photos' keys, in
	// order: the purge deletes the files and empties it 30 days after the
	// removal, whatever the answer, and the words stay.
	Body      string
	PhotoKeys []string
	// At is when they asked, on their own clock, as the log's At; CreatedAt
	// is when it was stored, on the database's.
	At        time.Time
	CreatedAt time.Time
	// Decision is LookOpen until an admin answers, then LookRestored or
	// LookKept, and DecidedAt when. One nobody answered stays LookOpen after
	// the purge deletes its post or comment.
	Decision  string
	DecidedAt *time.Time
}

// Event is one decision in the log, about one of the person's posts or
// comments, or their account.
type Event struct {
	ID uuid.UUID
	// Item is KindPost, KindComment or ItemAccount, and Action one of the
	// Log names.
	Item   string
	Action string
	// ItemID is the post's, the comment's or the account's id, kept after
	// the purge.
	ItemID uuid.UUID
	// Snapshot is the title or the comment's text when it was decided, and
	// Reasons the reasons of the reports a removal closed. Reason is the
	// reason the admin gave at a removal, a report reason's key: "" on every
	// other row, and on a removal with none.
	Snapshot string
	Reasons  []string
	Reason   string
	// At is when, on the owner's own clock, and CreatedAt when it was
	// logged, on the database's.
	At        time.Time
	CreatedAt time.Time
}

// Repository is the admin page's storage. The moderation actions each run
// in one transaction, and log what they decided there (Event). at is the
// server's now, which closes reports; loggedAt is the owner's own now, which
// the log keeps for the bell and the history.
type Repository interface {
	// OpenReports is every open report with whose it is, newest first: at
	// most OpenReportsLimit+1 of them, so the page can tell when more are
	// open than it shows.
	OpenReports(ctx context.Context) ([]OpenReport, error)
	// DisabledPeople is every disabled account, the most recently disabled
	// first.
	DisabledPeople(ctx context.Context) ([]Person, error)
	// Person is a person someone reported (about them, a post or a comment,
	// open or closed), a disabled account, or someone the log names; anyone
	// else is ErrNotFound, so the page opens no one else.
	Person(ctx context.Context, id uuid.UUID) (Person, error)
	Reports(ctx context.Context, person uuid.UUID) ([]Report, error)
	// Events is the person's decision log, newest first.
	Events(ctx context.Context, person uuid.UUID) ([]Event, error)
	// SecondLooks is every second look the person asked for, newest first,
	// answered or not.
	SecondLooks(ctx context.Context, person uuid.UUID) ([]SecondLook, error)
	// Posts is the person's posts, newest first: at most PostsLimit+1 of
	// them, so the page can tell when there are more than it shows. A post
	// they archived is left out, as only they see it, and so is one removed
	// past RemovalGrace. The page reads them only when ShowsPosts.
	Posts(ctx context.Context, person uuid.UUID) ([]Post, error)
	// Removed is every post and comment removed inside RemovalGrace: those
	// with an open second look first, then the most recently removed first.
	// RemovedCount is how many there are.
	Removed(ctx context.Context) ([]RemovedItem, error)
	RemovedCount(ctx context.Context) (int, error)
	// DismissItem closes the open reports of kind with these ids that are
	// about person, and says how many it closed. For a post or a comment it
	// holds the item meanwhile, so no report lands in between, and when the
	// item was hidden while under review before and is not after (and is
	// not removed, and for a comment its post and the comment it answers are
	// neither hidden under review nor removed), it logs that it shows
	// again: shownAgain.
	DismissItem(ctx context.Context, kind string, ids []uuid.UUID, person uuid.UUID, at, loggedAt time.Time) (closed int, shownAgain bool, err error)
	// RemoveItem removes person's post or comment (kind KindPost or
	// KindComment): it is gone for everyone, its owner too, from now on the
	// database's clock. Every report still open on it is closed as Removed,
	// and the removal is logged with its title or text, the reasons of
	// those reports and reason. Reason is the report reason the admin
	// picked (developer, 2026-11-18); "" takes the most serious reason of
	// the reports it closes, and stays "" when it closes none. Nothing is
	// deleted: the purge does that after RemovalGrace. ErrAlreadyDone when
	// it is removed already inside the grace (the first reason stays),
	// ErrNotFound when it is not there or removed past it.
	RemoveItem(ctx context.Context, kind string, target, person uuid.UUID, reason string, at, loggedAt time.Time) error
	// RestoreItem brings person's removed post or comment back while it is
	// inside RemovalGrace, and logs it. Its reports stay closed, and a second
	// look asked about its newest removal is closed as LookRestored, also
	// one a Keep closed before. A comment
	// whose post or the comment it answers is still removed stays removed:
	// ErrPostRemoved or ErrParentRemoved. ErrAlreadyDone when a Bring it back
	// brought it back already, ErrNotFound when it is not removed otherwise,
	// or past the grace.
	RestoreItem(ctx context.Context, kind string, target, person uuid.UUID, loggedAt time.Time) error
	// KeepRemoved is Keep it removed, the answer to a second look that
	// leaves person's post or comment removed (developer, 2026-11-19): the
	// request look, the one the page showed, is closed as LookKept while it
	// is open and about the newest removal, and that is logged (LogKept),
	// for the owner's bell. ErrNoRequest when it is not, and when the post
	// or comment is not removed (a Bring it back answered it); ErrAlreadyDone
	// when a Keep closed it already (the second click of a double click);
	// ErrNotFound when it is not there, or is past RemovalGrace.
	KeepRemoved(ctx context.Context, kind string, target, person, look uuid.UUID, loggedAt time.Time) error
	// ResolvePerson closes every open report about the person.
	ResolvePerson(ctx context.Context, person uuid.UUID, resolution string, at time.Time) error
	// ReportTarget is the post or comment an open-or-closed report points
	// at and whose it is; nil target when it is gone.
	ReportTarget(ctx context.Context, kind string, id uuid.UUID) (target *uuid.UUID, owner uuid.UUID, err error)
	// SetDisabled disables the account at at, or enables it again when at
	// is nil. Disabling ends every session of the account in the same
	// transaction (its refresh tokens are revoked), and deletes its Forgot
	// password code and its push tokens. When it changes the account, the
	// decision is logged at loggedAt in that transaction too.
	SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time, loggedAt time.Time) error
}
