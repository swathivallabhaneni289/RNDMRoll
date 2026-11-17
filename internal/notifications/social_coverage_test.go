package notifications

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// socialPeople is the social storage List reads: the switches it is given
// and a person for every id it knows. Any other method is never called.
type socialPeople struct {
	social.Repository
	settings social.Settings
	known    map[uuid.UUID]social.Person
}

func (s *socialPeople) Settings(context.Context, uuid.UUID) (social.Settings, error) {
	return s.settings, nil
}

func (s *socialPeople) People(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]social.Person, error) {
	out := map[uuid.UUID]social.Person{}
	for _, id := range ids {
		if p, ok := s.known[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

// socialRead is one read of the repository: which, and the since and limit
// it was given (a zero since for the reads that take none).
type socialRead struct {
	name  string
	since time.Time
	limit int
}

// socialNotes answers each read with the rows it is given and records the
// reads.
type socialNotes struct {
	comments, replies, follows, requests, accepted []Raw
	bonuses                                        []Bonus
	decisions                                      []Decision
	seen                                           time.Time
	haveSeen                                       bool
	reads                                          []socialRead
}

func (n *socialNotes) read(name string, since time.Time, limit int) {
	n.reads = append(n.reads, socialRead{name: name, since: since, limit: limit})
}

func (n *socialNotes) Comments(_ context.Context, _ uuid.UUID, since time.Time, limit int) ([]Raw, error) {
	n.read("comments", since, limit)
	return n.comments, nil
}

func (n *socialNotes) Replies(_ context.Context, _ uuid.UUID, since time.Time, limit int) ([]Raw, error) {
	n.read("replies", since, limit)
	return n.replies, nil
}

func (n *socialNotes) Follows(_ context.Context, _ uuid.UUID, since time.Time, limit int) ([]Raw, error) {
	n.read("follows", since, limit)
	return n.follows, nil
}

func (n *socialNotes) Requests(_ context.Context, _ uuid.UUID, limit int) ([]Raw, error) {
	n.read("requests", time.Time{}, limit)
	return n.requests, nil
}

func (n *socialNotes) Accepted(_ context.Context, _ uuid.UUID, since time.Time, limit int) ([]Raw, error) {
	n.read("accepted", since, limit)
	return n.accepted, nil
}

func (n *socialNotes) Bonuses(_ context.Context, _ uuid.UUID, since time.Time) ([]Bonus, error) {
	n.read("bonuses", since, 0)
	return n.bonuses, nil
}

func (n *socialNotes) DecisionsAndSeen(_ context.Context, _ uuid.UUID, since time.Time, limit int) ([]Decision, time.Time, bool, error) {
	n.read("decisions", since, limit)
	return n.decisions, n.seen, n.haveSeen, nil
}

func (n *socialNotes) MarkSeen(context.Context, uuid.UUID, time.Time) error { return nil }

// socialClock is the viewer's now and zone; the wheel did not open tonight.
type socialClock struct {
	now time.Time
	loc *time.Location
}

func (c socialClock) ViewerClock(context.Context, string) (time.Time, *time.Location, error) {
	return c.now, c.loc, nil
}

func (c socialClock) SpinOpenedTonight(context.Context, string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

// socialBell is a bell over the fakes at 9 PM on Monday Nov 16 in New York,
// with every switch on and one known actor.
func socialBell(t *testing.T) (*Service, *socialNotes, *socialPeople, time.Time, uuid.UUID) {
	t.Helper()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 21, 0, 0, 0, ny)
	actor := uuid.New()
	people := &socialPeople{settings: social.DefaultSettings,
		known: map[uuid.UUID]social.Person{actor: {ID: actor, Name: "Ann Actor", Username: "ann_actor"}}}
	notes := &socialNotes{}
	clk := clock.NewFixed(now)
	return NewService(notes, social.NewService(people, clk), socialClock{now: now, loc: ny}), notes, people, now, actor
}

// NOT-01 how it shows: at most 50 items, newest first, from the last 30
// days. Every read but the pending requests is given since = now minus 30
// days, and each is given the limit of 50; with more than 50 rows in all,
// the oldest are left out.
func TestSocialCoverage_TheBellKeepsTheNewest50FromTheLast30Days(t *testing.T) {
	bell, notes, _, now, actor := socialBell(t)
	// 61 items an hour apart: hour 7 is the bonus, every third hour a
	// follow, the rest comments.
	want := []string{}
	for h := 1; h <= 61; h++ {
		at := now.Add(-time.Duration(h) * time.Hour).UTC()
		entry := uuid.New()
		category, text := "Movie", "hi"
		switch {
		case h == 7:
			notes.bonuses = append(notes.bonuses, Bonus{WeekStart: "2026-11-16", Goal: 4, At: at})
			want = append(want, "bonus_spin:2026-11-16")
			continue
		case h%3 == 0 && len(notes.follows) < 20:
			id := fmt.Sprintf("follow:%02d", h)
			notes.follows = append(notes.follows, Raw{Kind: KindFollow, ID: id, At: at, ActorID: actor})
			want = append(want, id)
		default:
			id := fmt.Sprintf("comment:%02d", h)
			notes.comments = append(notes.comments, Raw{Kind: KindComment, ID: id, At: at, ActorID: actor,
				EntryID: &entry, CategoryName: &category, Text: &text})
			want = append(want, id)
		}
	}
	if len(notes.follows) != 20 || len(notes.comments) != 40 || len(notes.bonuses) != 1 {
		t.Fatalf("the rows: %d follows, %d comments, %d bonuses", len(notes.follows), len(notes.comments), len(notes.bonuses))
	}
	// The repository answers newest first per read; shuffle the comments to
	// show List sorts them itself.
	slices.Reverse(notes.comments)

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	ids := map[string]bool{}
	for i, it := range list.Items {
		got = append(got, it.ID)
		if ids[it.ID] {
			t.Fatalf("item %s twice", it.ID)
		}
		ids[it.ID] = true
		if i > 0 && it.At.After(list.Items[i-1].At) {
			t.Fatalf("not newest first at %d: %v after %v", i, it.At, list.Items[i-1].At)
		}
		if !it.Unread || it.AtLabel == "" || it.Section == "" {
			t.Fatalf("item %s: %+v", it.ID, it)
		}
	}
	if !slices.Equal(got, want[:MaxItems]) || MaxItems != 50 || !list.Unread {
		t.Fatalf("the list: %d items %v, want the 50 newest %v", len(got), got, want[:50])
	}

	since := now.Add(-30 * 24 * time.Hour)
	if WindowDays != 30 {
		t.Fatalf("WindowDays is %d, want 30", WindowDays)
	}
	for _, r := range notes.reads {
		switch r.name {
		case "requests":
			if !r.since.IsZero() || r.limit != 50 {
				t.Fatalf("the requests read: %+v, want no since and a limit of 50", r)
			}
		case "bonuses":
			if !r.since.Equal(since) {
				t.Fatalf("the bonuses read: since %v, want %v", r.since, since)
			}
		default:
			if !r.since.Equal(since) || r.limit != 50 {
				t.Fatalf("the %s read: since %v limit %d, want since %v and 50", r.name, r.since, r.limit, since)
			}
		}
	}
	// Six reads, and the admin's decisions about the viewer's own posts
	// and comments, which take the same window and limit.
	if len(notes.reads) != 7 {
		t.Fatalf("the reads: %+v, want all seven", notes.reads)
	}
}

// NOT-01: "You hit your goal this week. One bonus spin added." The item's id
// is bonus_spin:<week start>, its text the goal, no actor, at the instant the
// goal was met, in its place among the others. Read only while the
// Milestones switch is on; seen before it, it is not unread.
func TestSocialCoverage_TheBonusItemIsPlacedByTheTimeTheGoalWasMet(t *testing.T) {
	bell, notes, people, now, actor := socialBell(t)
	met := now.Add(-2 * time.Hour).UTC()
	notes.bonuses = []Bonus{{WeekStart: "2026-11-16", Goal: 5, At: met}}
	entry := uuid.New()
	category, text := "Meal", "yum"
	notes.comments = []Raw{
		{Kind: KindComment, ID: "comment:new", At: now.Add(-time.Hour).UTC(), ActorID: actor, EntryID: &entry, CategoryName: &category, Text: &text},
		{Kind: KindComment, ID: "comment:old", At: now.Add(-3 * time.Hour).UTC(), ActorID: actor, EntryID: &entry, CategoryName: &category, Text: &text},
	}
	notes.seen, notes.haveSeen = now.Add(-90*time.Minute).UTC(), true

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("the list: %+v", list.Items)
	}
	b := list.Items[1]
	if list.Items[0].ID != "comment:new" || list.Items[2].ID != "comment:old" || b.ID != "bonus_spin:2026-11-16" ||
		b.Kind != KindBonusSpin || b.Text == nil || *b.Text != "5" || b.Actor != nil || !b.At.Equal(met) ||
		b.AtLabel != "7:00 PM" || b.Section != SectionToday || b.Unread || !list.Unread {
		t.Fatalf("the bonus item and its place: %+v", list.Items)
	}

	// Milestones off: the bonuses are not even read.
	people.settings.Milestones = false
	notes.reads = nil
	list, err = bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range notes.reads {
		if r.name == "bonuses" {
			t.Fatal("the bonuses were read with Milestones off")
		}
	}
	for _, it := range list.Items {
		if it.Kind == KindBonusSpin {
			t.Fatalf("a bonus item with Milestones off: %+v", list.Items)
		}
	}
}
