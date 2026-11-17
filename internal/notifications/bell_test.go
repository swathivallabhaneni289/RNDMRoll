package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// fakeSocial is the social storage List reads: the default switches and no
// people. Any other method is never called here.
type fakeSocial struct{ social.Repository }

func (fakeSocial) Settings(context.Context, uuid.UUID) (social.Settings, error) {
	return social.DefaultSettings, nil
}

func (fakeSocial) People(context.Context, []uuid.UUID) (map[uuid.UUID]social.Person, error) {
	return map[uuid.UUID]social.Person{}, nil
}

// fakeNotes has no comments, follows or bonus weeks; it keeps one seen time.
type fakeNotes struct {
	seen     time.Time
	haveSeen bool
}

func (*fakeNotes) Comments(context.Context, uuid.UUID, time.Time, int) ([]Raw, error) {
	return nil, nil
}

func (*fakeNotes) Replies(context.Context, uuid.UUID, time.Time, int) ([]Raw, error) {
	return nil, nil
}

func (*fakeNotes) Follows(context.Context, uuid.UUID, time.Time, int) ([]Raw, error) {
	return nil, nil
}

func (*fakeNotes) Requests(context.Context, uuid.UUID, int) ([]Raw, error) {
	return nil, nil
}

func (*fakeNotes) Accepted(context.Context, uuid.UUID, time.Time, int) ([]Raw, error) {
	return nil, nil
}

func (*fakeNotes) Bonuses(context.Context, uuid.UUID, time.Time) ([]Bonus, error) {
	return nil, nil
}

func (*fakeNotes) Decisions(context.Context, uuid.UUID, time.Time, int) ([]Decision, error) {
	return nil, nil
}

func (f *fakeNotes) Seen(context.Context, uuid.UUID) (time.Time, bool, error) {
	return f.seen, f.haveSeen, nil
}

func (f *fakeNotes) MarkSeen(_ context.Context, _ uuid.UUID, at time.Time) error {
	f.seen, f.haveSeen = at, true
	return nil
}

// bellWorld is one person's bell over the real spin service on the
// in-memory repository, with one Fixed clock.
type bellWorld struct {
	t     *testing.T
	clk   *clock.Fixed
	spins *spin.Service
	bell  *Service
	id    uuid.UUID
}

func newBellWorld(t *testing.T) *bellWorld {
	t.Helper()
	clk := clock.NewFixed(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	spins := spin.NewService(spintest.NewMemRepo(), clk, spintest.NewQueueRand(20261116))
	bell := NewService(&fakeNotes{}, social.NewService(fakeSocial{}, clk), spins)
	return &bellWorld{t: t, clk: clk, spins: spins, bell: bell, id: uuid.New()}
}

// at moves the clock to a wall time in zone.
func (w *bellWorld) at(zone string, year int, month time.Month, day, h, m int) {
	w.t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		w.t.Fatal(err)
	}
	w.clk.Set(time.Date(year, month, day, h, m, 0, 0, loc))
}

func (w *bellWorld) zone(name string) {
	w.t.Helper()
	if _, err := w.spins.SetTimeZone(context.Background(), w.id.String(), name); err != nil {
		w.t.Fatal(err)
	}
}

func (w *bellWorld) spin() {
	w.t.Helper()
	if _, err := w.spins.Spin(context.Background(), w.id.String(), nil); err != nil {
		w.t.Fatalf("Spin: %v", err)
	}
}

func (w *bellWorld) seen() {
	w.t.Helper()
	if err := w.bell.MarkSeen(context.Background(), w.id); err != nil {
		w.t.Fatal(err)
	}
}

// spinOpen returns the list's "Your spin is open." item, or nil, and the
// dot as the summary gives it.
func (w *bellWorld) spinOpen() (*Item, bool) {
	w.t.Helper()
	l, err := w.bell.List(context.Background(), w.id)
	if err != nil {
		w.t.Fatalf("List: %v", err)
	}
	dot, err := w.bell.Unread(context.Background(), w.id)
	if err != nil || dot != l.Unread {
		w.t.Fatalf("Unread = %v, %v; the list says %v", dot, err, l.Unread)
	}
	for i := range l.Items {
		if l.Items[i].Kind == KindSpinOpen {
			return &l.Items[i], dot
		}
	}
	return nil, dot
}

func TestBell_NoSpinOpenOnTheEveningOfAFirstSpinMadeThatDay(t *testing.T) {
	w := newBellWorld(t)
	const ny = "America/New_York"
	// A new person spins at 10 AM on Monday (the first spin rule) and opens
	// the bell at 10:30.
	w.at(ny, 2026, 11, 16, 10, 0)
	w.zone(ny)
	w.spin()
	w.at(ny, 2026, 11, 16, 10, 30)
	w.seen()

	// That spin was Monday's: no wheel opens at 8 PM, so no item and no dot.
	for _, hm := range [][2]int{{19, 59}, {20, 0}, {21, 30}, {23, 59}} {
		w.at(ny, 2026, 11, 16, hm[0], hm[1])
		if it, dot := w.spinOpen(); it != nil || dot {
			t.Fatalf("Monday %02d:%02d: item %+v dot %v, want neither", hm[0], hm[1], it, dot)
		}
	}

	// Tuesday the wheel opens at 8 PM again.
	w.at(ny, 2026, 11, 17, 20, 5)
	it, dot := w.spinOpen()
	if it == nil || !dot || !it.Unread || it.AtLabel != "8:00 PM" || it.ID != "spin_open:2026-11-17" {
		t.Fatalf("Tuesday 20:05: item %+v dot %v", it, dot)
	}
}

func TestBell_SpinOpenStaysAfterTheEveningSpin(t *testing.T) {
	w := newBellWorld(t)
	const ny = "America/New_York"
	w.at(ny, 2026, 11, 15, 20, 30)
	w.zone(ny)
	w.spin()

	w.at(ny, 2026, 11, 16, 20, 0)
	if it, _ := w.spinOpen(); it == nil || it.AtLabel != "8:00 PM" {
		t.Fatalf("Monday 20:00: %+v", it)
	}
	w.at(ny, 2026, 11, 16, 20, 30)
	w.spin()
	w.at(ny, 2026, 11, 16, 20, 35)
	if it, _ := w.spinOpen(); it == nil || it.AtLabel != "8:00 PM" || it.Section != SectionToday {
		t.Fatalf("Monday 20:35, after the spin: %+v", it)
	}
}

func TestBell_AfterAZoneChangeTheItemComesWhenTheGapEnds(t *testing.T) {
	w := newBellWorld(t)
	// A spin at 11:50 PM on Monday in New York is 9:50 AM on Tuesday in
	// Karachi, so Tuesday's wheel there opens only at 9:50 PM.
	w.at("America/New_York", 2026, 11, 16, 23, 50)
	w.zone("America/New_York")
	w.spin()
	w.zone("Asia/Karachi")
	w.at("Asia/Karachi", 2026, 11, 17, 19, 0)
	w.seen()

	for _, hm := range [][2]int{{20, 0}, {21, 49}} {
		w.at("Asia/Karachi", 2026, 11, 17, hm[0], hm[1])
		if it, dot := w.spinOpen(); it != nil || dot {
			t.Fatalf("Tuesday %02d:%02d: item %+v dot %v, want neither", hm[0], hm[1], it, dot)
		}
	}
	w.at("Asia/Karachi", 2026, 11, 17, 21, 50)
	if it, dot := w.spinOpen(); it == nil || !dot || it.AtLabel != "9:50 PM" {
		t.Fatalf("Tuesday 21:50: item %+v dot %v, want the item at 9:50 PM", it, dot)
	}
}

func TestBell_TheDotIsOffBeforeTheZoneIsSent(t *testing.T) {
	w := newBellWorld(t)
	dot, err := w.bell.Unread(context.Background(), w.id)
	if err != nil || dot {
		t.Fatalf("Unread with no zone = %v, %v; want false and no error", dot, err)
	}
	// The list itself still asks for the zone.
	if _, err := w.bell.List(context.Background(), w.id); !errors.Is(err, spin.ErrZoneMissing) {
		t.Fatalf("List with no zone: %v, want ErrZoneMissing", err)
	}
}
