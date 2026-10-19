package spintest

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// QueueRand is a random source for tests. Values queued with Queue come out
// first (each taken modulo n); after that it is a seeded generator. It is
// safe for use from many goroutines.
type QueueRand struct {
	mu    sync.Mutex
	r     *rand.Rand
	queue []int
}

// NewQueueRand returns a QueueRand seeded with seed.
func NewQueueRand(seed uint64) *QueueRand {
	return &QueueRand{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// Queue makes the next calls to IntN return these values.
func (q *QueueRand) Queue(values ...int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = append(q.queue, values...)
}

// IntN implements spin.Rand.
func (q *QueueRand) IntN(n int) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queue) > 0 {
		v := q.queue[0]
		q.queue = q.queue[1:]
		return v % n
	}
	return q.r.IntN(n)
}

// Env is what one run of the suite needs from a repository. Each hook
// reaches storage directly, so the suite can see and set things the service
// does not expose.
type Env struct {
	Service *spin.Service
	// Clock is the clock the Service was built with.
	Clock *clock.Fixed
	// Rand is the random source the Service was built with.
	Rand *QueueRand

	// NewUser returns the id of a new account the repository accepts.
	NewUser func(t *testing.T) string
	// State reads the person's state row.
	State func(t *testing.T, userID string) spin.UserState
	// Counts returns how many spin rows and category rows (archived ones
	// included) the person has.
	Counts func(t *testing.T, userID string) (spins, categories int)
	// SetSpinResult sets one spin's stored result, as the entry phase will.
	SetSpinResult func(t *testing.T, userID, spinID string, result spin.SpinResult)
	// ForceZone stores a zone name that the service never checked.
	ForceZone func(t *testing.T, userID, name string)
	// FillArchived adds archived categories directly: `unreferenced` with no
	// spin, and `referenced` each with a daily spin on its own past date.
	FillArchived func(t *testing.T, userID string, unreferenced, referenced int)
}

const ny = "America/New_York"

var ctx = context.Background()

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%s): %v", name, err)
	}
	return loc
}

func date(t *testing.T, s string) spin.Date {
	t.Helper()
	d, err := spin.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%s): %v", s, err)
	}
	return d
}

// localAt returns the instant the wall clock in zoneName reads the date and
// time given.
func localAt(t *testing.T, zoneName, dateStr string, h, m, s int) time.Time {
	t.Helper()
	d := date(t, dateStr)
	got, ok := spin.InstantOfLocal(time.Date(d.Year, d.Month, d.Day, h, m, s, 0, time.UTC), zone(t, zoneName))
	if !ok {
		t.Fatalf("%s %02d:%02d:%02d does not exist in %s", dateStr, h, m, s, zoneName)
	}
	return got
}

// setClock moves the suite's clock to a local time.
func (e *Env) setClock(t *testing.T, zoneName, dateStr string, h, m, s int) {
	t.Helper()
	e.Clock.Set(localAt(t, zoneName, dateStr, h, m, s))
}

// ready makes a new person who has reported zoneName.
func (e *Env) ready(t *testing.T, zoneName string) string {
	t.Helper()
	u := e.NewUser(t)
	if _, err := e.Service.SetTimeZone(ctx, u, zoneName); err != nil {
		t.Fatalf("SetTimeZone(%s): %v", zoneName, err)
	}
	return u
}

// spunBefore makes a daily spin for u at 8:30 PM New York time on dateStr, so
// the person is no longer a first spinner. Pick a date at least two days
// before the test's own, so that spin leaves no late window behind.
func (e *Env) spunBefore(t *testing.T, u, dateStr string) {
	t.Helper()
	e.setClock(t, ny, dateStr, 20, 30, 0)
	e.spin(t, u)
}

func (e *Env) wheel(t *testing.T, u string) spin.WheelView {
	t.Helper()
	w, err := e.Service.Wheel(ctx, u)
	if err != nil {
		t.Fatalf("Wheel: %v", err)
	}
	return w
}

func (e *Env) today(t *testing.T, u string) spin.Today {
	t.Helper()
	got, err := e.Service.Today(ctx, u)
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	return got
}

func (e *Env) spin(t *testing.T, u string) spin.SpinOutcome {
	t.Helper()
	out, err := e.Service.Spin(ctx, u, nil)
	if err != nil {
		t.Fatalf("Spin: %v", err)
	}
	return out
}

func (e *Env) bonus(t *testing.T, u string, afterSeq int, forDate string) spin.SpinOutcome {
	t.Helper()
	out, err := e.Service.Bonus(ctx, u, afterSeq, date(t, forDate))
	if err != nil {
		t.Fatalf("Bonus(after %d, %s): %v", afterSeq, forDate, err)
	}
	return out
}

// postWeekDays pretends Monday to Thursday of the week that holds thursday
// are posted, which earns the bonus spin. The clock must already read that
// Thursday or later. The tests then spin on the Friday, because a day that is
// posted is settled and refuses a bonus.
func (e *Env) postWeekDays(t *testing.T, u string, thursday string) spin.TestDaysResult {
	t.Helper()
	th := date(t, thursday)
	var dates []spin.Date
	for i := 3; i >= 0; i-- {
		dates = append(dates, th.AddDays(-i))
	}
	res, err := e.Service.MarkTestDays(ctx, u, dates)
	if err != nil {
		t.Fatalf("MarkTestDays: %v", err)
	}
	return res
}

func wantErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func wantWheelReason(t *testing.T, err error, reason string, index int) {
	t.Helper()
	var we *spin.WheelError
	if !errors.As(err, &we) {
		t.Fatalf("error = %v, want a *WheelError (%s)", err, reason)
	}
	if we.Reason != reason || we.Index != index {
		t.Fatalf("wheel error %q at %d, want %q at %d", we.Reason, we.Index, reason, index)
	}
}

func wantHeld(t *testing.T, e *Env, u string, held int) {
	t.Helper()
	if got := e.State(t, u).BonusHeld; got != held {
		t.Fatalf("bonus_held = %d, want %d", got, held)
	}
}

func wantSpinRows(t *testing.T, e *Env, u string, n int) {
	t.Helper()
	if got, _ := e.Counts(t, u); got != n {
		t.Fatalf("spin rows = %d, want %d", got, n)
	}
}

// startOfWheel is the value to queue so the default wheel's picker lands on
// the entry at index: the sum of the weights before it, which is where that
// entry's share of the draw starts. The weights come from spin.DefaultWheel(),
// so this follows the starting wheel.
func startOfWheel(index int) int {
	sum := 0
	for _, c := range spin.DefaultWheel()[:index] {
		sum += c.Weight
	}
	return sum
}

// RunServiceSuite runs the behaviour every spin repository must give. Each
// subtest gets a fresh environment from newEnv.
func RunServiceSuite(t *testing.T, newEnv func(t *testing.T) *Env) {
	t.Helper()
	run := func(name string, fn func(t *testing.T, e *Env)) {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			fn(t, e)
		})
	}

	run("first call seeds the six starting categories once", func(t *testing.T, e *Env) {
		u := e.NewUser(t)
		e.setClock(t, ny, "2026-10-07", 16, 0, 0)
		if _, err := e.Service.SetTimeZone(ctx, u, ny); err != nil {
			t.Fatal(err)
		}
		first := e.wheel(t, u)
		want := []struct {
			name   string
			weight int
		}{{"Meal", 1}, {"Movie", 1}, {"Song", 1}, {"Book", 1}, {"Place", 1}, {"Game", 1}}
		if len(first.Categories) != 6 || first.Version != 1 {
			t.Fatalf("wheel = %+v, want 6 categories at version 1", first)
		}
		for i, w := range want {
			c := first.Categories[i]
			if c.Name != w.name || c.Weight != w.weight || c.Position != i {
				t.Errorf("category %d = %+v, want %s %d at position %d", i, c, w.name, w.weight, i)
			}
		}
		second := e.wheel(t, u)
		for i := range first.Categories {
			if second.Categories[i].ID != first.Categories[i].ID {
				t.Errorf("category %d changed id on the second call", i)
			}
		}
		if _, cats := e.Counts(t, u); cats != 6 {
			t.Fatalf("category rows = %d, want 6 (a second call seeds nothing)", cats)
		}
		st := e.State(t, u)
		if !st.SeededAt.Equal(e.Clock.NowFor(u)) || st.SeededLocalDate != date(t, "2026-10-07") || st.SeededTZ != ny {
			t.Errorf("seeded_* = %v %v %q, want the clock, 2026-10-07 and %s", st.SeededAt, st.SeededLocalDate, st.SeededTZ, ny)
		}
		if st.WeeklyGoal != spin.DefaultWeeklyGoal || st.BonusHeld != 0 || st.WheelVersion != 1 {
			t.Errorf("state = %+v", st)
		}
	})

	run("a person with no zone gets ErrZoneMissing and nothing is made", func(t *testing.T, e *Env) {
		u := e.NewUser(t)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		if _, err := e.Service.Today(ctx, u); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("Today with no zone: %v", err)
		}
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("Spin with no zone: %v", err)
		}
		if _, err := e.Service.SetTimeZone(ctx, u, "america/new_york"); !errors.Is(err, spin.ErrZoneInvalid) {
			t.Fatalf("SetTimeZone with a bad name: %v", err)
		}
		// The wheel needs no zone, and seeds anyway.
		if w := e.wheel(t, u); len(w.Categories) != 6 {
			t.Fatalf("wheel without a zone = %+v", w)
		}
		// A zone report afterwards works and Today answers.
		if _, err := e.Service.SetTimeZone(ctx, u, ny); err != nil {
			t.Fatal(err)
		}
		if got := e.today(t, u); got.State != spin.StateOpen {
			t.Fatalf("state = %s, want open at 8:30 PM", got.State)
		}
	})

	run("a spin waits for 8 PM and a second one returns the first", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.spunBefore(t, u, "2026-10-05")
		e.setClock(t, ny, "2026-10-07", 19, 59, 59)
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("Spin at 19:59:59: %v, want ErrNotOpen", err)
		}
		if got := e.today(t, u); got.State != spin.StateWaiting {
			t.Fatalf("state = %s, want waiting", got.State)
		}
		wantSpinRows(t, e, u, 1)

		e.setClock(t, ny, "2026-10-07", 20, 0, 0)
		if got := e.today(t, u); got.State != spin.StateOpen {
			t.Fatalf("state at 20:00:00 = %s, want open", got.State)
		}
		first := e.spin(t, u)
		if first.Already {
			t.Fatal("the first spin says already")
		}
		s := first.Today.Spin
		if first.Today.State != spin.StateResult || s == nil || s.Seq != 0 || s.Kind != spin.KindDaily || s.Result != spin.ResultOpen {
			t.Fatalf("today after the spin = %+v", first.Today)
		}
		if s.Wheel[s.WinnerIndex].Name != s.Category.Name || s.Wheel[s.WinnerIndex].ID != s.Category.ID {
			t.Errorf("winner_index %d does not point at %+v in %+v", s.WinnerIndex, s.Category, s.Wheel)
		}
		if s.SpunLocalTime != "8:00 PM" {
			t.Errorf("spun_local_time = %q, want 8:00 PM", s.SpunLocalTime)
		}
		if first.Today.NextOpenWeekday != "Thursday" {
			t.Errorf("next_open_weekday = %q, want Thursday", first.Today.NextOpenWeekday)
		}

		e.setClock(t, ny, "2026-10-07", 20, 10, 0)
		again := e.spin(t, u)
		if !again.Already || again.Today.Spin.ID != s.ID {
			t.Fatalf("second spin: already %v id %s, want already and %s", again.Already, again.Today.Spin.ID, s.ID)
		}
		wantSpinRows(t, e, u, 2)
	})

	run("a person who has never spun may spin at once, and it is that day's spin", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 10, 0, 0)
		before := e.today(t, u)
		if before.State != spin.StateOpen || !before.FirstSpin || before.Spin != nil {
			t.Fatalf("10 AM before any spin: state %s first_spin %v spin %v, want open, true, none", before.State, before.FirstSpin, before.Spin)
		}
		first := e.spin(t, u)
		s := first.Today.Spin
		if first.Already || first.Today.State != spin.StateResult || s == nil || s.Kind != spin.KindDaily || s.LocalDate != date(t, "2026-10-07") {
			t.Fatalf("the first spin = %+v", first)
		}
		if first.Today.FirstSpin {
			t.Error("first_spin is still true after the first spin")
		}
		if s.SpunLocalTime != "10:00 AM" {
			t.Errorf("spun_local_time = %q, want 10:00 AM", s.SpunLocalTime)
		}
		// Today's opening is passed over: the next one is Thursday 8 PM.
		if got, want := time.Time(first.Today.NextOpenAt), localAt(t, ny, "2026-10-08", 20, 0, 0); !got.Equal(want) {
			t.Errorf("next_open_at = %v, want %v", got, want)
		}

		// 8 PM the same day: no second daily spin.
		e.setClock(t, ny, "2026-10-07", 20, 0, 0)
		evening := e.today(t, u)
		if evening.State != spin.StateResult || evening.FirstSpin || evening.Spin == nil || evening.Spin.ID != s.ID {
			t.Fatalf("8 PM after a first spin: %+v", evening)
		}
		again := e.spin(t, u)
		if !again.Already || again.Today.Spin.ID != s.ID {
			t.Fatalf("a spin at 8 PM: already %v, want the first spin back", again.Already)
		}
		wantSpinRows(t, e, u, 1)

		// The next day is the usual rule: no spin before 8 PM (the unposted
		// first spin is late until then), open at 8 PM.
		e.setClock(t, ny, "2026-10-08", 10, 0, 0)
		if got := e.today(t, u); got.State != spin.StateLate || got.FirstSpin || got.Spin == nil || got.Spin.ID != s.ID {
			t.Fatalf("next day 10 AM: state %s first_spin %v, want late with the first spin, false", got.State, got.FirstSpin)
		}
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("next day 10 AM spin: %v, want ErrNotOpen", err)
		}
		e.setClock(t, ny, "2026-10-08", 20, 0, 0)
		if got := e.today(t, u); got.State != spin.StateOpen || got.FirstSpin {
			t.Fatalf("next day 8 PM: state %s first_spin %v, want open, false", got.State, got.FirstSpin)
		}
		if next := e.spin(t, u); next.Already || next.Today.Spin.LocalDate != date(t, "2026-10-08") {
			t.Fatalf("next day spin = %+v", next)
		}
		wantSpinRows(t, e, u, 2)
	})

	run("an early first spin passes over that evening's opening even when 12 hours have gone by", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 7, 0, 0)
		first := e.spin(t, u)
		if got, want := time.Time(first.Today.NextOpenAt), localAt(t, ny, "2026-10-08", 20, 0, 0); !got.Equal(want) {
			t.Errorf("next_open_at = %v, want %v", got, want)
		}
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		if got := e.today(t, u); got.State != spin.StateResult {
			t.Fatalf("8:30 PM after a 7 AM first spin: %s, want result", got.State)
		}
		if again := e.spin(t, u); !again.Already {
			t.Fatal("a second daily spin was made on the first spin's date")
		}
		wantSpinRows(t, e, u, 1)
	})

	run("a first spin on the night the clocks go back counts for that date", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		// 1:30 AM happens twice on 2026-11-01 in New York; this is the first.
		e.setClock(t, ny, "2026-11-01", 1, 30, 0)
		if got := e.today(t, u); got.State != spin.StateOpen || !got.FirstSpin {
			t.Fatalf("1:30 AM: state %s first_spin %v, want open, true", got.State, got.FirstSpin)
		}
		first := e.spin(t, u)
		if first.Today.Spin.LocalDate != date(t, "2026-11-01") {
			t.Fatalf("spin date = %s, want 2026-11-01", first.Today.Spin.LocalDate)
		}
		if got, want := time.Time(first.Today.NextOpenAt), localAt(t, ny, "2026-11-02", 20, 0, 0); !got.Equal(want) {
			t.Errorf("next_open_at = %v, want %v", got, want)
		}
	})

	run("after 8 PM a person who has never spun is open by the usual rule, not the first spin rule", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 21, 0, 0)
		if got := e.today(t, u); got.State != spin.StateOpen || got.FirstSpin {
			t.Fatalf("9 PM: state %s first_spin %v, want open, false", got.State, got.FirstSpin)
		}
	})

	run("a person who has any spin gets no early spin", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.spunBefore(t, u, "2026-10-01")
		e.setClock(t, ny, "2026-10-07", 10, 0, 0)
		if got := e.today(t, u); got.State != spin.StateWaiting || got.FirstSpin {
			t.Fatalf("10 AM: state %s first_spin %v, want waiting, false", got.State, got.FirstSpin)
		}
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("10 AM spin: %v, want ErrNotOpen", err)
		}
		wantSpinRows(t, e, u, 1)
	})

	run("an old or future for_date with no spin is refused", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		yesterday := date(t, "2026-10-06")
		tomorrow := date(t, "2026-10-08")
		if _, err := e.Service.Spin(ctx, u, &yesterday); !errors.Is(err, spin.ErrClosed) {
			t.Fatalf("Spin for yesterday: %v, want ErrClosed", err)
		}
		if _, err := e.Service.Spin(ctx, u, &tomorrow); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("Spin for tomorrow: %v, want ErrNotOpen", err)
		}
		wantSpinRows(t, e, u, 0)
		today := date(t, "2026-10-07")
		out, err := e.Service.Spin(ctx, u, &today)
		if err != nil || out.Already {
			t.Fatalf("Spin for today: %v already=%v", err, out.Already)
		}
	})

	run("a retry after midnight still gets the spin, and the screen is late", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 23, 59, 58)
		made := e.spin(t, u)
		id := made.Today.Spin.ID

		e.setClock(t, ny, "2026-10-08", 0, 0, 1)
		yesterday := date(t, "2026-10-07")
		retry, err := e.Service.Spin(ctx, u, &yesterday)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		if !retry.Already || retry.Today.Spin == nil || retry.Today.Spin.ID != id {
			t.Fatalf("retry: already %v spin %+v, want already with %s", retry.Already, retry.Today.Spin, id)
		}
		if retry.Today.State != spin.StateLate || retry.Today.LocalDate != date(t, "2026-10-08") {
			t.Fatalf("retry today: state %s date %v, want late on 2026-10-08", retry.Today.State, retry.Today.LocalDate)
		}
		if retry.Today.Spin.SpunLocalTime != "11:59 PM" {
			t.Errorf("spun_local_time = %q, want 11:59 PM", retry.Today.Spin.SpunLocalTime)
		}
		// Spinning for the new date before 8 PM is not allowed.
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("Spin for the new date at 00:00:01: %v, want ErrNotOpen", err)
		}
		wantSpinRows(t, e, u, 1)
	})

	run("late becomes waiting once yesterday is posted", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		e.spin(t, u)
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		if got := e.today(t, u); got.State != spin.StateLate || got.Spin == nil {
			t.Fatalf("state %s spin %v, want late with last night's spin", got.State, got.Spin)
		}
		if _, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-07"), spin.SourcePost); err != nil {
			t.Fatalf("MarkPosted(yesterday): %v", err)
		}
		got := e.today(t, u)
		if got.State != spin.StateWaiting || got.Spin != nil {
			t.Fatalf("after posting: state %s spin %v, want waiting and none", got.State, got.Spin)
		}
	})

	run("a bonus with none held is refused and spends nothing", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		e.spin(t, u)
		_, err := e.Service.Bonus(ctx, u, 0, date(t, "2026-10-07"))
		wantErr(t, err, spin.ErrNoBonus)
		wantSpinRows(t, e, u, 1)
		wantHeld(t, e, u, 0)
	})

	run("four posted days grant one bonus and a fifth none", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0) // Thursday
		for i, d := range []string{"2026-10-05", "2026-10-06", "2026-10-07"} {
			r, err := e.Service.MarkPosted(ctx, u, date(t, d), spin.SourceTest)
			if err != nil {
				t.Fatalf("MarkPosted(%s): %v", d, err)
			}
			if !r.Inserted || r.WeekCount != i+1 || r.GoalMet || r.BonusGranted {
				t.Fatalf("day %s: %+v", d, r)
			}
		}
		wantHeld(t, e, u, 0)
		r, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-08"), spin.SourceTest)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Inserted || r.WeekCount != 4 || !r.GoalMet || !r.BonusGranted {
			t.Fatalf("fourth day: %+v, want a granted bonus", r)
		}
		wantHeld(t, e, u, 1)
		if got := e.today(t, u); !got.Bonus.Held || !got.Week.GoalMet || got.Week.Posted != 4 {
			t.Fatalf("today: bonus %+v week %+v", got.Bonus, got.Week)
		}

		e.setClock(t, ny, "2026-10-09", 12, 0, 0)
		r, err = e.Service.MarkPosted(ctx, u, date(t, "2026-10-09"), spin.SourceTest)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Inserted || r.WeekCount != 5 || r.BonusGranted {
			t.Fatalf("fifth day: %+v, want no second bonus", r)
		}
		wantHeld(t, e, u, 1)

		// The same day again changes nothing.
		r, err = e.Service.MarkPosted(ctx, u, date(t, "2026-10-09"), spin.SourceTest)
		if err != nil || r.Inserted || r.WeekCount != 5 {
			t.Fatalf("repeat: %+v %v", r, err)
		}
	})

	run("a week hit while one is held adds none, and a spent bonus does not pay again", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		// Week A (from 2026-10-05) earns the held bonus.
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		if res := e.postWeekDays(t, u, "2026-10-08"); !res.BonusHeld {
			t.Fatal("week A did not grant")
		}
		// Week B (from 2026-10-12) hits its goal while one is held.
		e.setClock(t, ny, "2026-10-15", 12, 0, 0)
		res := e.postWeekDays(t, u, "2026-10-15")
		if !res.Week.GoalMet || !res.BonusHeld {
			t.Fatalf("week B: %+v", res)
		}
		wantHeld(t, e, u, 1)
		// Spend it on Friday evening.
		e.setClock(t, ny, "2026-10-16", 20, 30, 0)
		e.spin(t, u)
		e.bonus(t, u, 0, "2026-10-16")
		wantHeld(t, e, u, 0)
		// A fifth posted day in week B pays nothing: the week is paid.
		r, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-16"), spin.SourceTest)
		if err != nil || !r.Inserted || r.WeekCount != 5 || r.BonusGranted {
			t.Fatalf("fifth day of week B: %+v %v", r, err)
		}
		wantHeld(t, e, u, 0)
		// Week C pays again.
		e.setClock(t, ny, "2026-10-22", 12, 0, 0)
		if res := e.postWeekDays(t, u, "2026-10-22"); !res.BonusHeld {
			t.Fatal("week C did not grant")
		}
	})

	run("two hundred bonus spins never repeat the current category", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		firstThursday := date(t, "2026-10-08")
		for i := 0; i < 200; i++ {
			thu := firstThursday.AddDays(7 * i).String()
			e.setClock(t, ny, thu, 12, 0, 0)
			if res := e.postWeekDays(t, u, thu); !res.BonusHeld {
				t.Fatalf("week %d: no bonus held", i)
			}
			fri := date(t, thu).AddDays(1).String()
			e.setClock(t, ny, fri, 20, 30, 0)
			daily := e.spin(t, u).Today.Spin
			if daily == nil || daily.Seq != 0 {
				t.Fatalf("week %d: daily spin %+v", i, daily)
			}
			b := e.bonus(t, u, 0, fri)
			s := b.Today.Spin
			if b.Already || s == nil || s.Seq != 1 || s.Kind != spin.KindBonus {
				t.Fatalf("week %d: bonus spin %+v", i, s)
			}
			if s.Category.ID == daily.Category.ID {
				t.Fatalf("week %d: bonus spin repeated category %s", i, s.Category.Name)
			}
			if len(s.Wheel) != len(daily.Wheel) {
				t.Fatalf("week %d: bonus wheel has %d entries, daily %d", i, len(s.Wheel), len(daily.Wheel))
			}
			if s.Wheel[s.WinnerIndex].ID != s.Category.ID {
				t.Fatalf("week %d: winner_index does not point at the category", i)
			}
		}
	})

	run("a retried bonus spends nothing and returns the same spin", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		daily := e.spin(t, u).Today.Spin
		first := e.bonus(t, u, 0, "2026-10-09")
		if first.Already {
			t.Fatal("first bonus says already")
		}
		wantHeld(t, e, u, 0)

		retry := e.bonus(t, u, 0, "2026-10-09")
		if !retry.Already || retry.Today.Spin.ID != first.Today.Spin.ID {
			t.Fatalf("retry: already %v id %s, want already and %s", retry.Already, retry.Today.Spin.ID, first.Today.Spin.ID)
		}
		wantHeld(t, e, u, 0)
		wantSpinRows(t, e, u, 2)

		// A second phone that still shows the daily spin gets the same one.
		if retry.Today.Spin.Category.ID == daily.Category.ID {
			t.Error("the bonus category equals the daily one")
		}
		// afterSeq above the latest is a problem with the request, and a day
		// that already used its bonus offers no second.
		_, err := e.Service.Bonus(ctx, u, 5, date(t, "2026-10-09"))
		var in *spin.InputError
		if !errors.As(err, &in) || in.Field != "after_seq" {
			t.Fatalf("after_seq 5: %v, want an after_seq problem", err)
		}
		_, err = e.Service.Bonus(ctx, u, 1, date(t, "2026-10-09"))
		wantErr(t, err, spin.ErrNoBonus)
	})

	run("a stale for_date never spends a bonus", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		e.spin(t, u)

		e.setClock(t, ny, "2026-10-10", 0, 30, 0)
		_, err := e.Service.Bonus(ctx, u, 0, date(t, "2026-10-09"))
		wantErr(t, err, spin.ErrClosed)
		_, err = e.Service.Bonus(ctx, u, 0, date(t, "2026-10-10"))
		wantErr(t, err, spin.ErrNotOpen) // no daily spin yet today, before 8 PM
		_, err = e.Service.Bonus(ctx, u, 0, date(t, "2026-10-11"))
		wantErr(t, err, spin.ErrNotOpen)
		wantHeld(t, e, u, 1)
		wantSpinRows(t, e, u, 1)

		// After 8 PM with no daily spin yet, a bonus is an after_seq problem.
		e.setClock(t, ny, "2026-10-10", 20, 30, 0)
		_, err = e.Service.Bonus(ctx, u, 0, date(t, "2026-10-10"))
		var in *spin.InputError
		if !errors.As(err, &in) || in.Field != "after_seq" {
			t.Fatalf("bonus with no daily spin after 8 PM: %v", err)
		}
		wantHeld(t, e, u, 1)
	})

	run("a settled day refuses a bonus, but a retry of a spent one returns its spin", func(t *testing.T, e *Env) {
		// A: the day is posted, a bonus is held and unspent.
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		e.spin(t, u)
		if _, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-09"), spin.SourceTest); err != nil {
			t.Fatal(err)
		}
		_, err := e.Service.Bonus(ctx, u, 0, date(t, "2026-10-09"))
		wantErr(t, err, spin.ErrDaySettled)
		wantHeld(t, e, u, 1)
		got := e.today(t, u)
		if got.Spin.Result != spin.ResultPosted || got.Bonus.CanUse || !got.Bonus.Held {
			t.Fatalf("today: spin result %s bonus %+v", got.Spin.Result, got.Bonus)
		}

		// B: a bonus is spent, then the day is posted.
		u2 := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-15", 12, 0, 0)
		e.postWeekDays(t, u2, "2026-10-15")
		e.setClock(t, ny, "2026-10-16", 20, 30, 0)
		e.spin(t, u2)
		spent := e.bonus(t, u2, 0, "2026-10-16")
		if _, err := e.Service.MarkPosted(ctx, u2, date(t, "2026-10-16"), spin.SourceTest); err != nil {
			t.Fatal(err)
		}
		retry := e.bonus(t, u2, 0, "2026-10-16")
		if !retry.Already || retry.Today.Spin.ID != spent.Today.Spin.ID {
			t.Fatalf("retry of a spent bonus after posting: already %v", retry.Already)
		}
		_, err = e.Service.Bonus(ctx, u2, 1, date(t, "2026-10-16"))
		wantErr(t, err, spin.ErrDaySettled)

		// C: the latest spin's stored result settles the day too.
		u3 := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-22", 12, 0, 0)
		e.postWeekDays(t, u3, "2026-10-22")
		e.setClock(t, ny, "2026-10-23", 20, 30, 0)
		daily := e.spin(t, u3).Today.Spin
		e.SetSpinResult(t, u3, daily.ID, spin.ResultSkipped)
		_, err = e.Service.Bonus(ctx, u3, 0, date(t, "2026-10-23"))
		wantErr(t, err, spin.ErrDaySettled)
		wantHeld(t, e, u3, 1)
		if got := e.today(t, u3); got.Spin.Result != spin.ResultSkipped || got.Bonus.CanUse {
			t.Fatalf("today after a stored skip: %+v", got.Spin)
		}
	})

	run("a post is refused once its late window has ended and counts late before that", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		loc := zone(t, ny)
		d := date(t, "2026-10-07")
		w := spin.WindowFor(d, loc)

		// Before the close: not late.
		e.Clock.Set(w.ClosesAt.Add(-time.Nanosecond))
		r, err := e.Service.MarkPosted(ctx, u, d, spin.SourcePost)
		if err != nil || !r.Inserted || r.Late {
			t.Fatalf("one nanosecond before close: %+v %v, want not late", r, err)
		}

		// A second date, exactly at late_until: refused. One nanosecond
		// earlier: accepted and late.
		d2 := date(t, "2026-10-08")
		w2 := spin.WindowFor(d2, loc)
		e.Clock.Set(w2.LateUntil)
		_, err = e.Service.MarkPosted(ctx, u, d2, spin.SourcePost)
		wantErr(t, err, spin.ErrClosed)
		e.Clock.Set(w2.LateUntil.Add(-time.Nanosecond))
		r, err = e.Service.MarkPosted(ctx, u, d2, spin.SourcePost)
		if err != nil || !r.Inserted || !r.Late {
			t.Fatalf("one nanosecond before late_until: %+v %v, want accepted and late", r, err)
		}

		// Exactly at the close is late.
		d3 := date(t, "2026-10-09")
		w3 := spin.WindowFor(d3, loc)
		e.Clock.Set(w3.ClosesAt)
		r, err = e.Service.MarkPosted(ctx, u, d3, spin.SourcePost)
		if err != nil || !r.Late {
			t.Fatalf("at close: %+v %v, want late", r, err)
		}

		// A pretend day skips the late check, and a date that has not
		// begun is refused.
		d4 := date(t, "2026-10-01")
		e.Clock.Set(w3.LateUntil.Add(48 * time.Hour))
		if _, err := e.Service.MarkPosted(ctx, u, d4, spin.SourcePost); !errors.Is(err, spin.ErrClosed) {
			t.Fatalf("old date as a post: %v, want ErrClosed", err)
		}
		if r, err := e.Service.MarkPosted(ctx, u, d4, spin.SourceTest); err != nil || !r.Inserted {
			t.Fatalf("old date as a test day: %+v %v", r, err)
		}
		future := date(t, "2026-12-25")
		if _, err := e.Service.MarkPosted(ctx, u, future, spin.SourceTest); !errors.Is(err, spin.ErrNotOpen) {
			t.Fatalf("future date: %v, want ErrNotOpen", err)
		}
	})

	run("wheel edits: add, rename, reweight, archive, and old spins keep their names", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		start := e.wheel(t, u)
		byName := map[string]spin.Category{}
		for _, c := range start.Categories {
			byName[c.Name] = c
		}

		e.Rand.Queue(startOfWheel(2)) // Song wins
		made := e.spin(t, u).Today.Spin
		if made.Category.Name != "Song" {
			t.Fatalf("setup: winner %s, want Song", made.Category.Name)
		}

		id := func(name string) *string { s := byName[name].ID; return &s }
		saved, err := e.Service.SaveWheel(ctx, u, 1, []spin.WheelInput{
			{ID: id("Meal"), Name: "Meal", Weight: 5},    // reweight
			{ID: id("Song"), Name: "  Tune ", Weight: 1}, // rename (trimmed)
			{ID: id("Place"), Name: "Place", Weight: 1},
			{ID: id("Game"), Name: "Game", Weight: 1},
			{ID: id("Book"), Name: "Book", Weight: 1},
			{Name: "Walk", Weight: 2}, // add; Movie is left out and archived
		})
		if err != nil {
			t.Fatalf("SaveWheel: %v", err)
		}
		if saved.Version != 2 || len(saved.Categories) != 6 {
			t.Fatalf("saved = %+v", saved)
		}
		wantOrder := []string{"Meal", "Tune", "Place", "Game", "Book", "Walk"}
		for i, c := range saved.Categories {
			if c.Name != wantOrder[i] || c.Position != i {
				t.Errorf("category %d = %s at %d, want %s at %d", i, c.Name, c.Position, wantOrder[i], i)
			}
		}
		if saved.Categories[0].Weight != 5 || saved.Categories[1].ID != byName["Song"].ID {
			t.Errorf("reweight or rename lost the id: %+v", saved.Categories[:2])
		}
		if _, cats := e.Counts(t, u); cats != 7 {
			t.Errorf("category rows = %d, want 7 (Movie archived, not deleted)", cats)
		}

		// Today's spin keeps its name and its own wheel.
		got := e.today(t, u)
		if got.Spin.Category.Name != "Song" || got.Spin.ID != made.ID {
			t.Errorf("today's spin category = %q, want Song", got.Spin.Category.Name)
		}
		if len(got.Wheel) != 6 || got.Wheel[1].Name != "Movie" || got.Wheel[2].Name != "Song" || got.Wheel[0].Weight != 1 {
			t.Errorf("today's wheel changed: %+v", got.Wheel)
		}
		if live := e.wheel(t, u); live.Version != 2 {
			t.Errorf("version = %d", live.Version)
		}

		current := e.wheel(t, u)
		listOf := func() []spin.WheelInput {
			out := make([]spin.WheelInput, len(current.Categories))
			for i, c := range current.Categories {
				cid := c.ID
				out[i] = spin.WheelInput{ID: &cid, Name: c.Name, Weight: c.Weight}
			}
			return out
		}
		unchanged := func(label string) {
			t.Helper()
			w := e.wheel(t, u)
			if w.Version != current.Version || len(w.Categories) != len(current.Categories) {
				t.Fatalf("%s changed the wheel: %+v", label, w)
			}
			for i := range w.Categories {
				if w.Categories[i] != current.Categories[i] {
					t.Fatalf("%s changed category %d: %+v", label, i, w.Categories[i])
				}
			}
		}

		// Stale version.
		_, err = e.Service.SaveWheel(ctx, u, 1, listOf())
		wantErr(t, err, spin.ErrStale)
		unchanged("a stale version")

		// An id that does not exist, an archived id, a non-UUID, a repeated id.
		withID := func(index int, replacement string) []spin.WheelInput {
			l := listOf()
			l[index].ID = &replacement
			return l
		}
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(2, uuid.NewString()))
		wantWheelReason(t, err, spin.ReasonUnknownID, 2)
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(1, byName["Movie"].ID))
		wantWheelReason(t, err, spin.ReasonUnknownID, 1)
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(3, "not-a-uuid"))
		wantWheelReason(t, err, spin.ReasonUnknownID, 3)
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(4, "{"+byName["Meal"].ID+"}"))
		wantWheelReason(t, err, spin.ReasonUnknownID, 4)
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(4, current.Categories[0].ID))
		wantWheelReason(t, err, spin.ReasonDuplicateID, 4)
		unchanged("a bad id")

		// Another person's category id.
		other := e.ready(t, ny)
		theirs := e.wheel(t, other).Categories[0].ID
		_, err = e.Service.SaveWheel(ctx, u, 2, withID(0, theirs))
		wantWheelReason(t, err, spin.ReasonUnknownID, 0)
		unchanged("a foreign id")
		if mine := e.wheel(t, other); mine.Version != 1 || len(mine.Categories) != 6 {
			t.Fatalf("the other person's wheel changed: %+v", mine)
		}

		// The rules are checked before anything is written.
		bad := listOf()
		bad[2].Weight = 11
		_, err = e.Service.SaveWheel(ctx, u, 2, bad)
		wantWheelReason(t, err, spin.ReasonWeightRange, 2)
		_, err = e.Service.SaveWheel(ctx, u, 2, listOf()[:1])
		wantWheelReason(t, err, spin.ReasonTooFew, -1)
		unchanged("an invalid wheel")

		// Archiving down to two works; a list that would leave fewer is refused.
		two := listOf()[:2]
		after, err := e.Service.SaveWheel(ctx, u, 2, two)
		if err != nil || len(after.Categories) != 2 || after.Version != 3 {
			t.Fatalf("shrinking to two: %+v %v", after, err)
		}
	})

	run("the 500 row cap frees unused archived rows first and then refuses", func(t *testing.T, e *Env) {
		// At the cap with only unreferenced archived rows: they are freed.
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 16, 0, 0)
		start := e.wheel(t, u)
		e.FillArchived(t, u, 494, 0) // 6 live + 494 = 500
		if _, cats := e.Counts(t, u); cats != 500 {
			t.Fatalf("setup: %d rows, want 500", cats)
		}
		list := make([]spin.WheelInput, 0, 7)
		for _, c := range start.Categories {
			cid := c.ID
			list = append(list, spin.WheelInput{ID: &cid, Name: c.Name, Weight: c.Weight})
		}
		list = append(list, spin.WheelInput{Name: "Walk", Weight: 1})
		saved, err := e.Service.SaveWheel(ctx, u, 1, list)
		if err != nil || len(saved.Categories) != 7 {
			t.Fatalf("save at the cap with freeable rows: %+v %v", saved, err)
		}
		if _, cats := e.Counts(t, u); cats != 7 {
			t.Fatalf("rows after freeing = %d, want 7", cats)
		}

		// At the cap with archived rows that spins refer to: refused.
		u2 := e.ready(t, ny)
		start2 := e.wheel(t, u2)
		e.FillArchived(t, u2, 0, 494)
		list2 := make([]spin.WheelInput, 0, 7)
		for _, c := range start2.Categories {
			cid := c.ID
			list2 = append(list2, spin.WheelInput{ID: &cid, Name: c.Name, Weight: c.Weight})
		}
		list2 = append(list2, spin.WheelInput{Name: "Walk", Weight: 1})
		_, err = e.Service.SaveWheel(ctx, u2, 1, list2)
		wantWheelReason(t, err, spin.ReasonTooManyTotal, -1)
		w := e.wheel(t, u2)
		if w.Version != 1 || len(w.Categories) != 6 {
			t.Fatalf("a refused save changed the wheel: %+v", w)
		}
		if _, cats := e.Counts(t, u2); cats != 500 {
			t.Fatalf("rows after a refused save = %d, want 500", cats)
		}
		// Edits that add nothing still work at the cap.
		list2 = list2[:6]
		list2[0].Weight = 9
		if _, err := e.Service.SaveWheel(ctx, u2, 1, list2); err != nil {
			t.Fatalf("reweight at the cap: %v", err)
		}
	})

	run("a bonus spin draws from the day's snapshot, not the edited wheel", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		start := e.wheel(t, u)
		e.Rand.Queue(startOfWheel(0)) // Meal wins the daily spin
		daily := e.spin(t, u).Today.Spin

		// The wheel is changed after the spin: everything is renamed.
		list := make([]spin.WheelInput, len(start.Categories))
		for i, c := range start.Categories {
			cid := c.ID
			list[i] = spin.WheelInput{ID: &cid, Name: "Changed " + c.Name, Weight: 1}
		}
		if _, err := e.Service.SaveWheel(ctx, u, 1, list); err != nil {
			t.Fatal(err)
		}
		b := e.bonus(t, u, 0, "2026-10-09").Today.Spin
		if b.Category.ID == daily.Category.ID {
			t.Fatal("the bonus repeated the daily category")
		}
		snapshot := map[string]string{}
		for _, entry := range daily.Wheel {
			snapshot[entry.ID] = entry.Name
		}
		if name, ok := snapshot[b.Category.ID]; !ok || name != b.Category.Name {
			t.Fatalf("the bonus category %+v is not an entry of the day's snapshot %v", b.Category, snapshot)
		}
		if len(b.Wheel) != len(daily.Wheel) || b.Wheel[0].Name != "Meal" {
			t.Errorf("the bonus spin's wheel is not the snapshot: %+v", b.Wheel)
		}
	})

	run("a bonus spin skips snapshot categories that were removed for good", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		start := e.wheel(t, u)
		e.Rand.Queue(startOfWheel(0)) // Meal wins the daily spin
		daily := e.spin(t, u).Today.Spin

		// Movie and Book (never spun) are archived by leaving them out.
		keep := map[string]bool{"Meal": true, "Song": true, "Place": true, "Game": true}
		var list []spin.WheelInput
		for _, c := range start.Categories {
			if keep[c.Name] {
				cid := c.ID
				list = append(list, spin.WheelInput{ID: &cid, Name: c.Name, Weight: c.Weight})
			}
		}
		if _, err := e.Service.SaveWheel(ctx, u, 1, list); err != nil {
			t.Fatal(err)
		}
		// Fill the table to its cap, so the next save that adds a category
		// frees the archived rows for good (Movie and Book among them).
		e.FillArchived(t, u, 494, 0) // 4 live + 2 archived + 494 = 500
		w := e.wheel(t, u)
		list = list[:0]
		for _, c := range w.Categories {
			cid := c.ID
			list = append(list, spin.WheelInput{ID: &cid, Name: c.Name, Weight: c.Weight})
		}
		list = append(list, spin.WheelInput{Name: "Walk", Weight: 1})
		if _, err := e.Service.SaveWheel(ctx, u, w.Version, list); err != nil {
			t.Fatalf("save that frees the removed rows: %v", err)
		}
		if _, cats := e.Counts(t, u); cats != 5 {
			t.Fatalf("category rows = %d, want 5 (the archived rows were freed)", cats)
		}

		// Unfiltered, a draw of 0 among the snapshot's entries other than
		// Meal lands on Movie, which no longer exists. Filtered, it is Song.
		e.Rand.Queue(0)
		b := e.bonus(t, u, 0, "2026-10-09").Today.Spin
		if b.Category.Name != "Song" || b.Category.ID == daily.Category.ID {
			t.Fatalf("the bonus landed on %s, want Song", b.Category.Name)
		}
		if len(b.Wheel) != 6 {
			t.Errorf("the bonus spin's wheel has %d entries, want the day's 6", len(b.Wheel))
		}
	})

	run("a zone change cannot buy a second spin inside 12 hours", func(t *testing.T, e *Env) {
		cases := []struct{ name, from, to, date string }{
			{"Kiritimati to Pago Pago", "Pacific/Kiritimati", "Pacific/Pago_Pago", "2026-10-07"},
			{"Pago Pago to Kiritimati", "Pacific/Pago_Pago", "Pacific/Kiritimati", "2026-10-07"},
			{"Tokyo to Los Angeles", "Asia/Tokyo", "America/Los_Angeles", "2026-10-07"},
			{"Los Angeles to Tokyo", "America/Los_Angeles", "Asia/Tokyo", "2026-10-07"},
			{"New York to London", "America/New_York", "Europe/London", "2026-10-07"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				u := e.ready(t, c.from)
				e.setClock(t, c.from, c.date, 20, 5, 0)
				first := e.spin(t, u).Today.Spin
				spunAt := e.Clock.NowFor(u)
				if _, err := e.Service.SetTimeZone(ctx, u, c.to); err != nil {
					t.Fatal(err)
				}
				// Every half hour for the next 12 hours, and at the very end.
				probes := make([]time.Time, 0, 26)
				for m := 0; m < 12*60; m += 30 {
					probes = append(probes, spunAt.Add(time.Duration(m)*time.Minute))
				}
				probes = append(probes, spunAt.Add(12*time.Hour-time.Nanosecond))
				for _, p := range probes {
					e.Clock.Set(p)
					out, err := e.Service.Spin(ctx, u, nil)
					switch {
					case err == nil && out.Already && out.Today.Spin != nil && out.Today.Spin.ID == first.ID:
					case errors.Is(err, spin.ErrNotOpen) || errors.Is(err, spin.ErrClosed):
					default:
						t.Fatalf("at +%v: got %+v err %v, want the first spin back or a refusal", p.Sub(spunAt), out.Today.Spin, err)
					}
					wantSpinRows(t, e, u, 1)
				}
				// Walking forward, a new daily spin does come, and never
				// earlier than 12 hours after the first.
				var second *spin.SpinView
				for m := 12 * 60; m <= 72*60 && second == nil; m += 15 {
					p := spunAt.Add(time.Duration(m) * time.Minute)
					e.Clock.Set(p)
					out, err := e.Service.Spin(ctx, u, nil)
					if err == nil && !out.Already {
						second = out.Today.Spin
					}
				}
				if second == nil {
					t.Fatal("no second daily spin became possible within 72 hours")
				}
				if gap := time.Time(second.SpunAt).Sub(spunAt); gap < 12*time.Hour {
					t.Fatalf("second spin came %v after the first", gap)
				}
			})
		}
	})

	run("a stored zone that no longer loads is ErrZoneMissing until the phone reports again", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		e.ForceZone(t, u, "Nowhere/Land")
		if _, err := e.Service.Today(ctx, u); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("Today: %v, want ErrZoneMissing", err)
		}
		if _, err := e.Service.Spin(ctx, u, nil); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("Spin: %v, want ErrZoneMissing", err)
		}
		if _, err := e.Service.Bonus(ctx, u, 0, date(t, "2026-10-07")); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("Bonus: %v, want ErrZoneMissing", err)
		}
		if _, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-07"), spin.SourceTest); !errors.Is(err, spin.ErrZoneMissing) {
			t.Fatalf("MarkPosted: %v, want ErrZoneMissing", err)
		}
		// The wheel does not need a zone.
		if w := e.wheel(t, u); len(w.Categories) != 6 {
			t.Fatalf("wheel = %+v", w)
		}
		if _, err := e.Service.SetTimeZone(ctx, u, ny); err != nil {
			t.Fatal(err)
		}
		if got := e.today(t, u); got.State != spin.StateOpen {
			t.Fatalf("state = %s after the zone was reported again", got.State)
		}
	})

	run("the test reset clears spins, pretend days and goal weeks and keeps the wheel and zone", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		wantHeld(t, e, u, 1)
		e.setClock(t, ny, "2026-10-08", 20, 30, 0)
		e.spin(t, u)
		before := e.State(t, u)
		wheelBefore := e.wheel(t, u)

		if err := e.Service.ResetTestData(ctx, u); err != nil {
			t.Fatal(err)
		}
		wantHeld(t, e, u, 0)
		wantSpinRows(t, e, u, 0)
		got := e.today(t, u)
		if got.State != spin.StateOpen || got.Spin != nil || got.Week.Posted != 0 {
			t.Fatalf("today after reset: state %s spin %v posted %d", got.State, got.Spin, got.Week.Posted)
		}
		after := e.State(t, u)
		if !after.SeededAt.Equal(before.SeededAt) || after.SeededLocalDate != before.SeededLocalDate || after.SeededTZ != before.SeededTZ {
			t.Errorf("seeded_* changed: %+v -> %+v", before, after)
		}
		wheelAfter := e.wheel(t, u)
		for i := range wheelBefore.Categories {
			if wheelAfter.Categories[i] != wheelBefore.Categories[i] {
				t.Errorf("category %d changed", i)
			}
		}
		// The bonus can be earned again, and a spin is possible again.
		if res := e.postWeekDays(t, u, "2026-10-08"); !res.BonusHeld {
			t.Error("the week did not pay again after a reset")
		}
		if out := e.spin(t, u); out.Already {
			t.Error("the spin after a reset says already")
		}
	})

	run("test days are validated: none in the future, none older than 60 days", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 12, 0, 0)
		cases := map[string][]spin.Date{
			"empty":       nil,
			"future":      {date(t, "2026-10-08")},
			"too old":     {date(t, "2026-08-07")}, // 61 days back
			"eight dates": {date(t, "2026-10-01"), date(t, "2026-10-02"), date(t, "2026-10-03"), date(t, "2026-10-04"), date(t, "2026-10-05"), date(t, "2026-10-06"), date(t, "2026-10-07"), date(t, "2026-09-30")},
		}
		for name, dates := range cases {
			_, err := e.Service.MarkTestDays(ctx, u, dates)
			var in *spin.InputError
			if !errors.As(err, &in) || in.Field != "dates" {
				t.Errorf("%s: %v, want a dates problem", name, err)
			}
		}
		// 60 days back is allowed, and a refused call marks nothing.
		if got := e.today(t, u); got.Week.Posted != 0 {
			t.Fatalf("a refused call marked days: %+v", got.Week)
		}
		if _, err := e.Service.MarkTestDays(ctx, u, []spin.Date{date(t, "2026-08-08")}); err != nil {
			t.Fatalf("60 days back: %v", err)
		}
	})

	// --- concurrency ---

	const racers = 20

	run("twenty simultaneous spins make one spin", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		results := make([]spin.SpinOutcome, racers)
		errs := make([]error, racers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results[i], errs[i] = e.Service.Spin(ctx, u, nil)
			}(i)
		}
		close(start)
		wg.Wait()
		fresh := 0
		var id string
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("racer %d: %v", i, errs[i])
			}
			if !results[i].Already {
				fresh++
			}
			if id == "" {
				id = results[i].Today.Spin.ID
			} else if results[i].Today.Spin.ID != id {
				t.Fatalf("racer %d got spin %s, others %s", i, results[i].Today.Spin.ID, id)
			}
		}
		if fresh != 1 {
			t.Fatalf("%d racers made a new spin, want exactly 1", fresh)
		}
		wantSpinRows(t, e, u, 1)
	})

	run("five simultaneous bonus calls with one after_seq make one bonus", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.setClock(t, ny, "2026-10-09", 20, 30, 0)
		e.spin(t, u)
		forDate := date(t, "2026-10-09")
		const n = 5
		results := make([]spin.SpinOutcome, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results[i], errs[i] = e.Service.Bonus(ctx, u, 0, forDate)
			}(i)
		}
		close(start)
		wg.Wait()
		fresh := 0
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("call %d: %v", i, errs[i])
			}
			if !results[i].Already {
				fresh++
			}
			if results[i].Today.Spin.ID != results[0].Today.Spin.ID {
				t.Fatalf("call %d returned another spin", i)
			}
		}
		if fresh != 1 {
			t.Fatalf("%d calls spent the bonus, want exactly 1", fresh)
		}
		wantHeld(t, e, u, 0)
		wantSpinRows(t, e, u, 2)
	})

	run("four simultaneous posted days give one bonus", func(t *testing.T, e *Env) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		dates := []spin.Date{date(t, "2026-10-05"), date(t, "2026-10-06"), date(t, "2026-10-07"), date(t, "2026-10-08")}
		results := make([]spin.PostedResult, len(dates))
		errs := make([]error, len(dates))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, d := range dates {
			wg.Add(1)
			go func(i int, d spin.Date) {
				defer wg.Done()
				<-start
				results[i], errs[i] = e.Service.MarkPosted(ctx, u, d, spin.SourceTest)
			}(i, d)
		}
		close(start)
		wg.Wait()
		granted := 0
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("call %d: %v", i, errs[i])
			}
			if results[i].BonusGranted {
				granted++
			}
		}
		if granted != 1 {
			t.Fatalf("%d calls granted a bonus, want 1", granted)
		}
		wantHeld(t, e, u, 1)
		if got := e.today(t, u); got.Week.Posted != 4 {
			t.Fatalf("week posted = %d", got.Week.Posted)
		}
	})

	run("ten simultaneous first calls make six categories", func(t *testing.T, e *Env) {
		u := e.NewUser(t)
		const n = 10
		results := make([]spin.WheelView, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results[i], errs[i] = e.Service.Wheel(ctx, u)
			}(i)
		}
		close(start)
		wg.Wait()
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("call %d: %v", i, errs[i])
			}
			if len(results[i].Categories) != 6 || results[i].Categories[0].ID != results[0].Categories[0].ID {
				t.Fatalf("call %d: %+v", i, results[i])
			}
		}
		if _, cats := e.Counts(t, u); cats != 6 {
			t.Fatalf("category rows = %d, want 6", cats)
		}
	})

	run("five simultaneous saves with one version give one winner", func(t *testing.T, e *Env) {
		u := e.NewUser(t)
		start0 := e.wheel(t, u)
		const n = 5
		wins := make([]bool, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				list := make([]spin.WheelInput, 0, 3)
				for j := 0; j < 2; j++ {
					cid := start0.Categories[j].ID
					list = append(list, spin.WheelInput{ID: &cid, Name: start0.Categories[j].Name, Weight: 1 + i})
				}
				list = append(list, spin.WheelInput{Name: fmt.Sprintf("New %d", i), Weight: 1})
				_, errs[i] = e.Service.SaveWheel(ctx, u, 1, list)
				wins[i] = errs[i] == nil
			}(i)
		}
		close(start)
		wg.Wait()
		won := 0
		for i := range wins {
			if wins[i] {
				won++
			} else if !errors.Is(errs[i], spin.ErrStale) {
				t.Fatalf("save %d: %v, want ErrStale", i, errs[i])
			}
		}
		if won != 1 {
			t.Fatalf("%d saves won, want 1", won)
		}
		if w := e.wheel(t, u); w.Version != 2 || len(w.Categories) != 3 {
			t.Fatalf("wheel after the race = %+v", w)
		}
	})
}
