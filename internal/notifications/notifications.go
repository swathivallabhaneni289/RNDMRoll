// Package notifications builds the bell's list from data that already exists
// (comments, replies, follows, requests, the spin window and the person's
// spins, goal weeks, and the decision log's rows about the person's own posts
// and comments: the admin's decisions, and the hides by reports). No
// notification is stored and no job runs; the only state is one seen time per
// person. The notification settings filter the list and the unread flag,
// except for the decision log's rows, which always show.
// Reactions on posts make no items (developer, 2026-10-18: reactions were
// removed from the app; the stored "reactions" switch no longer changes the
// list).
package notifications

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// Kinds.
const (
	KindComment        = "comment"
	KindReply          = "reply"
	KindFollow         = "follow"
	KindFollowRequest  = "follow_request"
	KindFollowAccepted = "follow_accepted"
	KindSpinOpen       = "spin_open"
	KindBonusSpin      = "bonus_spin"

	// The admin's decisions about the viewer's own post or comment
	// (developer, 2026-11-17): "We removed your post for hate speech." (the
	// reason the admin gave, 2026-11-18; "We removed your post. It broke our
	// rules." with Something else or none) and "Sorry, we got that wrong.
	// Your post is back.", and the same for a comment. Back is a Bring it
	// back, or a Dismiss that showed a hidden one again. The app words the
	// reason: the server sends its key.
	KindPostRemoved    = "post_removed"
	KindCommentRemoved = "comment_removed"
	KindPostBack       = "post_back"
	KindCommentBack    = "comment_back"

	// Reports hid the viewer's own post or comment until an admin looks
	// (developer, 2026-11-18): "Your post is hidden while we take a look.",
	// and the same for a comment.
	KindPostHidden    = "post_hidden"
	KindCommentHidden = "comment_hidden"

	// The admin looked again at the viewer's removed post or comment, as
	// they asked, and kept it removed (developer, 2026-11-19): "We looked
	// again. Your post stays removed.", and the same for a comment. Like a
	// removal, it opens nothing.
	KindPostKept    = "post_kept"
	KindCommentKept = "comment_kept"
)

// The second look on a removal (SecondLook): none asked, asked and not yet
// answered, Keep it removed, or back. Back is also a removal that a Bring it
// back answered with nothing asked: there is nothing left to ask about.
// Closed is one nobody asked about that can no longer be asked about: its
// post or comment is deleted, or its 30 days are over. Only None can ask.
const (
	SecondLookNone     = ""
	SecondLookAsked    = "asked"
	SecondLookKept     = "kept"
	SecondLookRestored = "restored"
	SecondLookClosed   = "closed"
)

// Sections.
const (
	SectionToday   = "today"
	SectionWeek    = "week"
	SectionEarlier = "earlier"
)

// Limits.
const (
	WindowDays = 30
	MaxItems   = 50
	TextRunes  = 60
)

// Raw is one item as the repository reads it.
type Raw struct {
	Kind         string
	ID           string
	At           time.Time
	ActorID      uuid.UUID
	EntryID      *uuid.UUID
	CategoryName *string
	Text         *string
	RequestID    *uuid.UUID
	// PhotoKey is the post's first photo, for the small picture at the right.
	PhotoKey *string
}

// Bonus is a week whose goal paid a bonus spin.
type Bonus struct {
	WeekStart string
	Goal      int
	At        time.Time
}

// Decision is one row of the decision log (migration 0020's
// moderation_events) about the viewer's own post or comment, as the
// repository reads it: the facts, which decisionItem turns into an item by
// the rules of its kind.
type Decision struct {
	// ID is the log row's. The item's id and the removal's ref come from it.
	ID uuid.UUID
	// Item is admin.KindPost or admin.KindComment; Action is admin.LogRemoved,
	// admin.LogRestored, admin.LogShownAgain, admin.LogHidden or
	// admin.LogKept.
	Item, Action string
	// At is the owner's own clock when it was decided, or hidden.
	At time.Time
	// Snapshot is the title or the comment text when it was decided.
	Snapshot string
	// Reason is the reason the admin gave at a removal, a report reason's
	// key: "" on every other row, and on a removal with none.
	Reason string
	// SecondLook is the second look on a removal (SecondLookNone and the
	// others), on the database's clock: "" on every other row.
	SecondLook string
	// EntryID is the post (for a comment, its post) while it is still in the
	// database: nil once it is deleted.
	EntryID *uuid.UUID
	// Visible is whether the viewer can see that post now (social.CanView).
	Visible bool
	// PhotoKey is that post's first photo, read only while it is Visible.
	PhotoKey *string
}

// Item is one row of the list as the viewer sees it.
type Item struct {
	ID           string
	Kind         string
	At           time.Time
	AtLabel      string
	Section      string
	Unread       bool
	Actor        *social.Person
	EntryID      *uuid.UUID
	CategoryName *string
	Text         *string
	RequestID    *uuid.UUID
	PhotoKey     *string
	// Ref, Reason, Removal and SecondLook are on a removal only: its short
	// reference (admin.Ref), the reason the admin gave, as a report reason's
	// key ("" for none), the removal's full id, which the app asks for a
	// second look with, and the second look on it (SecondLookNone and the
	// others).
	Ref        string
	Reason     string
	Removal    *uuid.UUID
	SecondLook string
}

// List is the list and its unread flag.
type List struct {
	Unread bool
	Items  []Item
}

// Repository reads the sources. Every read is keyed to the viewer, leaves
// out the viewer's own actions and anyone with a block either way, and
// returns at most limit rows newest first.
type Repository interface {
	// Comments on the viewer's entries, replies included, except a reply to
	// the viewer's own comment (that one is a Replies item only).
	Comments(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]Raw, error)
	// Replies to the viewer's comments on posts the viewer can still see.
	Replies(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]Raw, error)
	// Follows: people who follow the viewer, made at once or by a request
	// the viewer accepted (dated by the request).
	Follows(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]Raw, error)
	// Requests: pending requests to the viewer, whatever their age.
	Requests(ctx context.Context, owner uuid.UUID, limit int) ([]Raw, error)
	// Accepted: the viewer's requests that the other person accepted.
	Accepted(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]Raw, error)
	Bonuses(ctx context.Context, viewer uuid.UUID, since time.Time) ([]Bonus, error)
	// DecisionsAndSeen reads the two things every list needs whatever the
	// switches, together: the admin's removals and returns of the viewer's
	// own posts and comments, dated by the viewer's own clock (decisions
	// about the account are not among them), the newest limit and every
	// removal that can still be asked about (SecondLookNone), and the
	// viewer's seen time (haveSeen is false while there is none).
	DecisionsAndSeen(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) (decisions []Decision, seen time.Time, haveSeen bool, err error)
	// MarkSeen moves the seen time forward only.
	MarkSeen(ctx context.Context, viewer uuid.UUID, at time.Time) error
}

// Clock is the viewer's now and zone, and whether and when the wheel opened
// for them tonight (spin.Service).
type Clock interface {
	ViewerClock(ctx context.Context, userID string) (time.Time, *time.Location, error)
	SpinOpenedTonight(ctx context.Context, userID string) (at time.Time, ok bool, err error)
}

// Service builds the list.
type Service struct {
	repo   Repository
	social *social.Service
	clk    Clock
}

// NewService builds a Service.
func NewService(repo Repository, soc *social.Service, clk Clock) *Service {
	return &Service{repo: repo, social: soc, clk: clk}
}

// SectionOf is an item's section in the viewer's zone, by the same day count
// as its label: today (or later), the six days before, earlier.
func SectionOf(at, now time.Time, loc *time.Location) string {
	n := spin.DaysBefore(at, now, loc)
	switch {
	case n <= 0:
		return SectionToday
	case n <= 6:
		return SectionWeek
	default:
		return SectionEarlier
	}
}

// CutText cuts s to max characters (line breaks become spaces) and adds
// three dots when it was longer.
func CutText(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// List builds the viewer's list. The viewer needs a stored zone.
func (s *Service) List(ctx context.Context, viewer uuid.UUID) (List, error) {
	now, loc, err := s.clk.ViewerClock(ctx, viewer.String())
	if err != nil {
		return List{}, err
	}
	settings, err := s.social.Settings(ctx, viewer)
	if err != nil {
		return List{}, err
	}
	since := now.Add(-WindowDays * 24 * time.Hour)

	var raws []Raw
	add := func(list []Raw, err error) error {
		raws = append(raws, list...)
		return err
	}
	if settings.Comments {
		if err := add(s.repo.Comments(ctx, viewer, since, MaxItems)); err != nil {
			return List{}, err
		}
		if err := add(s.repo.Replies(ctx, viewer, since, MaxItems)); err != nil {
			return List{}, err
		}
	}
	if err := add(s.repo.Follows(ctx, viewer, since, MaxItems)); err != nil {
		return List{}, err
	}
	if err := add(s.repo.Requests(ctx, viewer, MaxItems)); err != nil {
		return List{}, err
	}
	if err := add(s.repo.Accepted(ctx, viewer, since, MaxItems)); err != nil {
		return List{}, err
	}

	ids := make([]uuid.UUID, 0, len(raws))
	for _, r := range raws {
		ids = append(ids, r.ActorID)
	}
	people, err := s.social.People(ctx, ids)
	if err != nil {
		return List{}, err
	}

	items := make([]Item, 0, len(raws)+2)
	for _, r := range raws {
		p, ok := people[r.ActorID]
		if !ok {
			continue
		}
		actor := p
		items = append(items, Item{
			ID: r.ID, Kind: r.Kind, At: r.At, Actor: &actor, EntryID: r.EntryID,
			CategoryName: r.CategoryName, Text: r.Text, RequestID: r.RequestID,
			PhotoKey: r.PhotoKey,
		})
	}

	if settings.Milestones {
		// "Your spin is open." only on an evening the wheel really opened, so
		// a first spin made earlier that day (the day's spin) adds no item
		// and no dot. The id is the date of the opening itself, not of this
		// list's now: the two are read apart and can fall either side of
		// midnight.
		at, opened, err := s.clk.SpinOpenedTonight(ctx, viewer.String())
		if err != nil {
			return List{}, err
		}
		if opened {
			items = append(items, Item{ID: "spin_open:" + spin.LocalDate(at, loc).String(), Kind: KindSpinOpen, At: at.UTC()})
		}
		bonuses, err := s.repo.Bonuses(ctx, viewer, since)
		if err != nil {
			return List{}, err
		}
		for _, b := range bonuses {
			text := strconv.Itoa(b.Goal)
			items = append(items, Item{ID: "bonus_spin:" + b.WeekStart, Kind: KindBonusSpin, At: b.At, Text: &text})
		}
	}

	// The admin's decisions about the viewer's own posts and comments, and
	// the hides by reports, have no actor, and no switch turns them off: they
	// are the only word the owner gets that something of theirs was hidden,
	// removed or is back. The seen time comes with them, in the same read.
	decisions, seen, haveSeen, err := s.repo.DecisionsAndSeen(ctx, viewer, since, MaxItems)
	if err != nil {
		return List{}, err
	}
	for _, d := range decisions {
		if it, ok := s.decisionItem(d); ok {
			items = append(items, it)
		}
	}

	slices.SortFunc(items, func(a, b Item) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	if len(items) > MaxItems {
		// A removal its owner can still ask a second look about stays past
		// the 50: its line is the only way to ask, and the Terms promise
		// its 30 days.
		rest := slices.DeleteFunc(items[MaxItems:], func(it Item) bool { return !canAsk(it) })
		items = items[:MaxItems+len(rest)]
	}

	out := List{Items: items}
	for i := range out.Items {
		it := &out.Items[i]
		it.Section = SectionOf(it.At, now, loc)
		it.AtLabel = spin.ViewerLabel(it.At, now, loc)
		it.Unread = !haveSeen || it.At.After(seen)
		if it.Unread {
			out.Unread = true
		}
	}
	return out, nil
}

// canAsk is whether it is a removal its owner can still ask a second look
// about.
func canAsk(it Item) bool {
	return (it.Kind == KindPostRemoved || it.Kind == KindCommentRemoved) && it.SecondLook == SecondLookNone
}

// decisionKind is the item kind of a decision about a post or a comment, ""
// for any other (an account's, say), which makes no item.
func decisionKind(item, action string) string {
	removed := action == admin.LogRemoved
	back := action == admin.LogRestored || action == admin.LogShownAgain
	hidden := action == admin.LogHidden
	kept := action == admin.LogKept
	switch {
	case item == admin.KindPost && removed:
		return KindPostRemoved
	case item == admin.KindComment && removed:
		return KindCommentRemoved
	case item == admin.KindPost && back:
		return KindPostBack
	case item == admin.KindComment && back:
		return KindCommentBack
	case item == admin.KindPost && hidden:
		return KindPostHidden
	case item == admin.KindComment && hidden:
		return KindCommentHidden
	case item == admin.KindPost && kept:
		return KindPostKept
	case item == admin.KindComment && kept:
		return KindCommentKept
	}
	return ""
}

// decisionItem is the item for one decision, with the id kind:<log row>, at
// the decision's own time and with the title or the comment text as it was
// then (whole, on one line). A removal carries no way back to what was
// removed, not even while it is still kept for Bring it back: no post and no
// photo, only its ref, the reason the admin gave, its full id to ask for a
// second look with, and that second look's state. A Keep it removed carries
// none of them, no post and no photo either. A post that is back or
// hidden opens the post, and shows its first photo while the viewer can see
// it; a comment that is back or hidden opens its post only while the viewer
// can see that post, which may be someone else's, and shows no photo.
func (s *Service) decisionItem(d Decision) (Item, bool) {
	kind := decisionKind(d.Item, d.Action)
	if kind == "" {
		return Item{}, false
	}
	it := Item{ID: kind + ":" + d.ID.String(), Kind: kind, At: d.At.UTC()}
	// A comment that is only a photo has no text, as in commentRows.
	if text := strings.ReplaceAll(d.Snapshot, "\n", " "); text != "" {
		it.Text = &text
	}
	switch kind {
	case KindPostRemoved, KindCommentRemoved:
		id := d.ID
		it.Ref = admin.Ref(d.ID)
		it.Reason = d.Reason
		it.Removal = &id
		it.SecondLook = d.SecondLook
	case KindPostBack, KindPostHidden:
		it.EntryID = d.EntryID
		if d.Visible {
			it.PhotoKey = d.PhotoKey
		}
	case KindCommentBack, KindCommentHidden:
		if d.Visible {
			it.EntryID = d.EntryID
		}
	}
	return it, true
}

// Unread is the bell's dot: the same answer as the list's flag. A person
// with no stored zone yet (a new account in the moment before the app sends
// it) has no list to read, so the dot is off rather than an error.
func (s *Service) Unread(ctx context.Context, viewer uuid.UUID) (bool, error) {
	l, err := s.List(ctx, viewer)
	if errors.Is(err, spin.ErrZoneMissing) {
		return false, nil
	}
	return l.Unread, err
}

// MarkSeen marks everything up to now as seen.
func (s *Service) MarkSeen(ctx context.Context, viewer uuid.UUID) error {
	now, _, err := s.clk.ViewerClock(ctx, viewer.String())
	if err != nil {
		return err
	}
	return s.repo.MarkSeen(ctx, viewer, now.UTC())
}
