package spin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// factsWorld is a spin service over the in-memory repository with one Fixed
// clock, for reading BoardFacts the way the Board does.
type factsWorld struct {
	t   *testing.T
	svc *spin.Service
	clk *clock.Fixed
}

func newFactsWorld(t *testing.T) *factsWorld {
	t.Helper()
	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	return &factsWorld{t: t, svc: spin.NewService(spintest.NewMemRepo(), clk, spintest.NewQueueRand(20261008)), clk: clk}
}

// at moves the clock to a wall time in zone.
func (w *factsWorld) at(zone, date string, h, m int) {
	w.t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		w.t.Fatal(err)
	}
	d, err := spin.ParseDate(date)
	if err != nil {
		w.t.Fatal(err)
	}
	w.clk.Set(time.Date(d.Year, d.Month, d.Day, h, m, 0, 0, loc))
}

// person makes a new account that reported zone.
func (w *factsWorld) person(zone string) string {
	w.t.Helper()
	u := uuid.NewString()
	if _, err := w.svc.SetTimeZone(context.Background(), u, zone); err != nil {
		w.t.Fatal(err)
	}
	return u
}

func (w *factsWorld) spin(u string) spin.SpinOutcome {
	w.t.Helper()
	out, err := w.svc.Spin(context.Background(), u, nil)
	if err != nil {
		w.t.Fatalf("Spin: %v", err)
	}
	return out
}

func (w *factsWorld) facts(u string) spin.BoardFacts {
	w.t.Helper()
	bf, err := w.svc.BoardFacts(context.Background(), u)
	if err != nil {
		w.t.Fatalf("BoardFacts: %v", err)
	}
	return bf
}

// wantDay checks the day in play, the late flag and the action.
func wantDay(t *testing.T, label string, bf spin.BoardFacts, day string, late bool, action string) {
	t.Helper()
	if bf.DayInPlay.String() != day || bf.LateWindow != late || bf.Action != action {
		t.Fatalf("%s: day %s late %v action %s, want %s %v %s", label, bf.DayInPlay, bf.LateWindow, bf.Action, day, late, action)
	}
}

func TestBoardFacts_AFirstSpinPutsTheBoardOnTodayPostedOrNot(t *testing.T) {
	w := newFactsWorld(t)
	const ny = "America/New_York"
	const day = "2026-10-08"
	w.at(ny, day, 10, 0)
	u := w.person(ny)

	// Never spun: today, offering the first spin.
	wantDay(t, "never spun", w.facts(u), day, false, spin.ActionSpin)

	// The first spin, not posted: still today (never last night's posts,
	// which no post of this person's could open), offering to log it on time.
	out := w.spin(u)
	w.at(ny, day, 10, 5)
	bf := w.facts(u)
	wantDay(t, "an unposted first spin", bf, day, false, spin.ActionLog)
	if bf.Posted || bf.Skipped || bf.PostLate || bf.CategoryName == nil || *bf.CategoryName != out.Today.Spin.Category.Name {
		t.Fatalf("an unposted first spin: %+v", bf)
	}

	// Skipped: today, skipped, nothing to do until the next spin.
	if _, err := w.svc.SkipDay(context.Background(), u, out.Today.Spin.LocalDate, out.Today.Spin.Seq); err != nil {
		t.Fatal(err)
	}
	bf = w.facts(u)
	wantDay(t, "a skipped first spin", bf, day, false, spin.ActionWait)
	if bf.Posted || !bf.Skipped {
		t.Fatalf("a skipped first spin: %+v", bf)
	}

	// The next morning nothing is on that date yet: last night, skipped.
	w.at(ny, "2026-10-09", 9, 0)
	bf = w.facts(u)
	wantDay(t, "the next morning", bf, day, true, spin.ActionWait)
	if !bf.Skipped {
		t.Fatalf("the next morning: %+v", bf)
	}
}

func TestBoardFacts_SomeoneWhoSpunBeforeIsOnLastNightUntil8PM(t *testing.T) {
	w := newFactsWorld(t)
	const ny = "America/New_York"
	w.at(ny, "2026-10-06", 20, 30)
	u := w.person(ny)
	w.spin(u)

	w.at(ny, "2026-10-08", 10, 0)
	wantDay(t, "10 AM", w.facts(u), "2026-10-07", true, spin.ActionWait)
	w.at(ny, "2026-10-08", 20, 0)
	wantDay(t, "8 PM", w.facts(u), "2026-10-08", false, spin.ActionSpin)
}

func TestBoardFacts_ASpinMadeBeforeAMoveWestKeepsTheBoardOnItsDate(t *testing.T) {
	w := newFactsWorld(t)
	const ny, honolulu = "America/New_York", "Pacific/Honolulu"
	w.at(ny, "2026-10-06", 20, 30)
	u := w.person(ny)
	w.spin(u)
	w.at(ny, "2026-10-08", 20, 30)
	out := w.spin(u)

	// In Honolulu it is 2:30 PM on the same date: the spin is today's, as on
	// the Spin page, so the Board stays on it and offers to log it.
	if _, err := w.svc.SetTimeZone(context.Background(), u, honolulu); err != nil {
		t.Fatal(err)
	}
	if today, err := w.svc.Today(context.Background(), u); err != nil || today.State != spin.StateResult || today.Spin.ID != out.Today.Spin.ID {
		t.Fatalf("today after the move: %+v %v", today, err)
	}
	wantDay(t, "after the move", w.facts(u), "2026-10-08", false, spin.ActionLog)
}
