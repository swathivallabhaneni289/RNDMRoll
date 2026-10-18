// Package notifications builds the bell's list from data that already exists
// (comments, replies, follows, requests, the spin window, goal weeks). No
// notification is stored and no job runs; the only state is one seen time
// per person. The notification settings filter the list and the unread flag.
// Reactions on posts make no items (developer, 2026-10-18: reactions were
// removed from the app; the stored "reactions" switch no longer changes the
// list).
package notifications

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
	Seen(ctx context.Context, viewer uuid.UUID) (time.Time, bool, error)
	// MarkSeen moves the seen time forward only.
	MarkSeen(ctx context.Context, viewer uuid.UUID, at time.Time) error
}

// Clock is the viewer's now and zone (spin.Service).
type Clock interface {
	ViewerClock(ctx context.Context, userID string) (time.Time, *time.Location, error)
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
		today := spin.LocalDate(now, loc)
		win := spin.WindowFor(today, loc)
		if !now.Before(win.OpensAt) && now.Before(win.ClosesAt) {
			items = append(items, Item{ID: "spin_open:" + today.String(), Kind: KindSpinOpen, At: win.OpensAt.UTC()})
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

	slices.SortFunc(items, func(a, b Item) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	if len(items) > MaxItems {
		items = items[:MaxItems]
	}

	seen, haveSeen, err := s.repo.Seen(ctx, viewer)
	if err != nil {
		return List{}, err
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

// Unread is the bell's dot: the same answer as the list's flag.
func (s *Service) Unread(ctx context.Context, viewer uuid.UUID) (bool, error) {
	l, err := s.List(ctx, viewer)
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
