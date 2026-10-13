package spin

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var testWheel = []WheelEntry{
	{ID: "id-meal", Name: "Meal", Weight: 1},
	{ID: "id-movie", Name: "Movie", Weight: 1},
	{ID: "id-song", Name: "Song", Weight: 1},
	{ID: "id-book", Name: "Book", Weight: 1},
	{ID: "id-place", Name: "Place", Weight: 1},
	{ID: "id-game", Name: "Game", Weight: 1},
}

// at returns the instant a wall clock in loc reads the given date and time.
func at(t testing.TB, loc *time.Location, date string, h, m, s int) time.Time {
	t.Helper()
	d := mustDate(t, date)
	got, ok := InstantOfLocal(time.Date(d.Year, d.Month, d.Day, h, m, s, 0, time.UTC), loc)
	if !ok {
		t.Fatalf("%s %02d:%02d:%02d does not exist in %v", date, h, m, s, loc)
	}
	return got
}

// spinAt builds a stored spin made at `when` in the zone zoneName, on the
// local date of `when` in that zone.
func spinAt(t testing.TB, zoneName string, when time.Time, kind SpinKind, seq int) *Spin {
	t.Helper()
	loc := mustLoc(t, zoneName)
	d := LocalDate(when, loc)
	w := WindowFor(d, loc)
	return &Spin{
		ID:             "spin-" + d.String() + "-" + string(kind),
		LocalDate:      d,
		TZName:         zoneName,
		Kind:           kind,
		Seq:            seq,
		CategoryID:     testWheel[2].ID,
		CategoryName:   testWheel[2].Name,
		Wheel:          testWheel,
		WinnerIndex:    2,
		SpunAt:         when,
		WindowOpensAt:  w.OpensAt,
		WindowClosesAt: w.ClosesAt,
		Result:         ResultOpen,
	}
}

const ny = "America/New_York"

func baseInput(t testing.TB, zone string, now time.Time) TodayInput {
	t.Helper()
	return TodayInput{
		Now:    now,
		Loc:    mustLoc(t, zone),
		TZName: zone,
		Wheel:  testWheel,
		Goal:   DefaultWeeklyGoal,
	}
}

func TestBuildToday_Waiting(t *testing.T) {
	loc := mustLoc(t, ny)
	now := at(t, loc, "2026-10-07", 16, 0, 0)
	got := BuildToday(baseInput(t, ny, now))

	if got.State != StateWaiting {
		t.Fatalf("state = %s, want waiting", got.State)
	}
	if got.LocalDate.String() != "2026-10-07" || got.TimeZone != ny {
		t.Errorf("date %v zone %q", got.LocalDate, got.TimeZone)
	}
	if got.Spin != nil {
		t.Errorf("spin = %+v, want nil", got.Spin)
	}
	if len(got.Wheel) != 6 || got.Wheel[0].Name != "Meal" {
		t.Errorf("wheel = %+v, want the live wheel", got.Wheel)
	}
	wantOpen := at(t, loc, "2026-10-07", 20, 0, 0)
	if !time.Time(got.NextOpenAt).Equal(wantOpen) || !time.Time(got.Window.OpensAt).Equal(wantOpen) {
		t.Errorf("next_open_at %v / opens_at %v, want %v", time.Time(got.NextOpenAt), time.Time(got.Window.OpensAt), wantOpen)
	}
	if got.NextOpenWeekday != "Wednesday" {
		t.Errorf("next_open_weekday = %q, want Wednesday", got.NextOpenWeekday)
	}
	if got.Bonus.Held || got.Bonus.CanUse {
		t.Errorf("bonus = %+v", got.Bonus)
	}
}

func TestBuildToday_OpenAtEightAndUntilMidnight(t *testing.T) {
	loc := mustLoc(t, ny)
	for _, c := range []struct {
		name string
		now  time.Time
		want TodayState
	}{
		{"one second before eight", at(t, loc, "2026-10-07", 19, 59, 59), StateWaiting},
		{"exactly eight", at(t, loc, "2026-10-07", 20, 0, 0), StateOpen},
		{"eight thirty", at(t, loc, "2026-10-07", 20, 30, 0), StateOpen},
		{"23:59:59.999", at(t, loc, "2026-10-07", 23, 59, 59).Add(999 * time.Millisecond), StateOpen},
		{"midnight is the new date and waiting", at(t, loc, "2026-10-08", 0, 0, 0), StateWaiting},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := BuildToday(baseInput(t, ny, c.now))
			if got.State != c.want {
				t.Fatalf("state = %s, want %s", got.State, c.want)
			}
		})
	}
	// While open, the next opening is tomorrow's.
	got := BuildToday(baseInput(t, ny, at(t, loc, "2026-10-07", 20, 30, 0)))
	if want := at(t, loc, "2026-10-08", 20, 0, 0); !time.Time(got.NextOpenAt).Equal(want) {
		t.Errorf("next_open_at while open = %v, want %v", time.Time(got.NextOpenAt), want)
	}
	if got.NextOpenWeekday != "Thursday" {
		t.Errorf("next_open_weekday while open = %q, want Thursday", got.NextOpenWeekday)
	}
}

func TestBuildToday_ResultShowsTheLatestSpinAndItsOwnWheel(t *testing.T) {
	loc := mustLoc(t, ny)
	now := at(t, loc, "2026-10-07", 20, 45, 0)
	in := baseInput(t, ny, now)
	in.Spin = spinAt(t, ny, at(t, loc, "2026-10-07", 20, 2, 0), KindDaily, 0)
	spunAt := in.Spin.SpunAt
	in.LastDailySpunAt = &spunAt
	in.Wheel = []WheelEntry{{ID: "x", Name: "Changed", Weight: 1}, {ID: "y", Name: "Wheel", Weight: 1}}
	in.BonusHeld = true

	got := BuildToday(in)
	if got.State != StateResult {
		t.Fatalf("state = %s, want result", got.State)
	}
	if got.Spin == nil || got.Spin.Category.Name != "Song" || got.Spin.Kind != KindDaily || got.Spin.Seq != 0 {
		t.Fatalf("spin = %+v", got.Spin)
	}
	if len(got.Wheel) != 6 || got.Wheel[0].Name != "Meal" {
		t.Errorf("wheel = %+v, want the spin's own wheel, not the changed live one", got.Wheel)
	}
	if !got.Bonus.Held || !got.Bonus.CanUse {
		t.Errorf("bonus = %+v, want held and usable", got.Bonus)
	}
	// The next opening is tomorrow's, which is 22 hours away, so the 12 hour
	// gap does not move it.
	if want := at(t, loc, "2026-10-08", 20, 0, 0); !time.Time(got.NextOpenAt).Equal(want) {
		t.Errorf("next_open_at = %v, want %v", time.Time(got.NextOpenAt), want)
	}
	if got.NextOpenWeekday != "Thursday" {
		t.Errorf("next_open_weekday = %q, want Thursday", got.NextOpenWeekday)
	}
}

func TestBuildToday_BonusCanUseRules(t *testing.T) {
	loc := mustLoc(t, ny)
	now := at(t, loc, "2026-10-07", 21, 0, 0)
	mk := func() TodayInput {
		in := baseInput(t, ny, now)
		in.Spin = spinAt(t, ny, at(t, loc, "2026-10-07", 20, 5, 0), KindDaily, 0)
		in.BonusHeld = true
		return in
	}

	in := mk()
	if got := BuildToday(in); !got.Bonus.CanUse {
		t.Error("held, result, not settled: want can_use")
	}

	in = mk()
	in.BonusHeld = false
	if got := BuildToday(in); got.Bonus.CanUse || got.Bonus.Held {
		t.Errorf("none held: %+v", got.Bonus)
	}

	in = mk()
	in.Settled = true
	if got := BuildToday(in); got.Bonus.CanUse {
		t.Error("settled day offers a bonus")
	}

	// A day that already used its bonus spin offers no second: the database
	// allows one bonus spin per date.
	in = mk()
	in.Spin = spinAt(t, ny, at(t, loc, "2026-10-07", 20, 30, 0), KindBonus, 1)
	if got := BuildToday(in); got.Bonus.CanUse {
		t.Error("a day with a bonus spin offers another")
	}

	// Not in the open, waiting or late states.
	in = baseInput(t, ny, at(t, loc, "2026-10-07", 20, 5, 0))
	in.BonusHeld = true
	if got := BuildToday(in); got.State != StateOpen || got.Bonus.CanUse {
		t.Errorf("open: state %s bonus %+v", got.State, got.Bonus)
	}
}

func TestBuildToday_LateAfterMidnightUntilSettledOrTheNextOpening(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	yesterday := spinAt(t, ny, spun, KindDaily, 0)

	mk := func(now time.Time) TodayInput {
		in := baseInput(t, ny, now)
		in.YesterdaySpin = yesterday
		in.LastDailySpunAt = &spun
		return in
	}

	// 00:30 the next day: late, and the screen shows last night's spin on its
	// own wheel.
	got := BuildToday(mk(at(t, loc, "2026-10-08", 0, 30, 0)))
	if got.State != StateLate {
		t.Fatalf("state = %s, want late", got.State)
	}
	if got.Spin == nil || got.Spin.LocalDate.String() != "2026-10-07" || got.Spin.Category.Name != "Song" {
		t.Fatalf("late spin = %+v, want last night's", got.Spin)
	}
	if got.Spin.SpunLocalTime != "8:30 PM" {
		t.Errorf("spun_local_time = %q, want 8:30 PM", got.Spin.SpunLocalTime)
	}
	if got.LocalDate.String() != "2026-10-08" {
		t.Errorf("local_date = %v, want the new date", got.LocalDate)
	}
	if got.Bonus.CanUse {
		t.Error("a bonus offered in the late state")
	}
	// Late has no countdown but still names the next opening: 8 PM today.
	if want := at(t, loc, "2026-10-08", 20, 0, 0); !time.Time(got.NextOpenAt).Equal(want) {
		t.Errorf("next_open_at = %v, want %v", time.Time(got.NextOpenAt), want)
	}

	// 19:59:59: still late. 20:00:00 with more than 12 hours since the spin:
	// open.
	if got := BuildToday(mk(at(t, loc, "2026-10-08", 19, 59, 59))); got.State != StateLate {
		t.Errorf("19:59:59 state = %s, want late", got.State)
	}
	if got := BuildToday(mk(at(t, loc, "2026-10-08", 20, 0, 0))); got.State != StateOpen {
		t.Errorf("20:00:00 state = %s, want open", got.State)
	}

	// Yesterday settled (posted): waiting, not late.
	in := mk(at(t, loc, "2026-10-08", 0, 30, 0))
	posted := *yesterday
	posted.Result = ResultPosted
	in.YesterdaySpin = &posted
	if got := BuildToday(in); got.State != StateWaiting || got.Spin != nil {
		t.Errorf("settled yesterday: state %s spin %v, want waiting and none", got.State, got.Spin)
	}
	skipped := *yesterday
	skipped.Result = ResultSkipped
	in.YesterdaySpin = &skipped
	if got := BuildToday(in); got.State != StateWaiting {
		t.Errorf("skipped yesterday: state %s, want waiting", got.State)
	}
}

func TestBuildToday_LateIsGuardedByTheEndOfTheLateWindow(t *testing.T) {
	// Yesterday's spin was never posted. It is now 8:30 PM the next day, so
	// the late window ended at 8:00 PM. A daily spin made 5 hours ago in
	// another zone keeps the wheel shut, and the screen must say waiting, not
	// late: posting yesterday is no longer possible.
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 23, 55, 0)
	now := at(t, loc, "2026-10-08", 20, 30, 0)
	recent := now.Add(-5 * time.Hour)

	in := baseInput(t, ny, now)
	in.YesterdaySpin = spinAt(t, ny, spun, KindDaily, 0)
	in.LastDailySpunAt = &recent

	got := BuildToday(in)
	if got.State != StateWaiting {
		t.Fatalf("state = %s, want waiting (gap shut, late window over)", got.State)
	}
	if got.Spin != nil {
		t.Errorf("spin = %+v, want none", got.Spin)
	}
	// The next opening honours the gap: the first 8 PM at or after
	// recent + 12h (that is 03:30 tomorrow), which is tomorrow's 8 PM.
	if want := at(t, loc, "2026-10-09", 20, 0, 0); !time.Time(got.NextOpenAt).Equal(want) {
		t.Errorf("next_open_at = %v, want %v", time.Time(got.NextOpenAt), want)
	}
}

func TestBuildToday_ASpinUnderTwelveHoursOldKeepsOpenShut(t *testing.T) {
	tokyo := mustLoc(t, "Asia/Tokyo")
	la := mustLoc(t, "America/Los_Angeles")

	// A daily spin made at 20:05 in Los Angeles is 12:05 the next day in
	// Tokyo. Tokyo's own 20:00 comes 8 hours after it: shut.
	spun := at(t, la, "2026-10-07", 20, 5, 0)
	in := baseInput(t, "Asia/Tokyo", at(t, tokyo, "2026-10-08", 20, 30, 0))
	in.LastDailySpunAt = &spun
	if got := BuildToday(in); got.State != StateWaiting {
		t.Fatalf("state = %s, 8h25m after the spin; want waiting", got.State)
	}

	// Past 8 PM local, exactly 12 hours after the last spin is open and one
	// nanosecond short is not.
	late := baseInput(t, "Asia/Tokyo", at(t, tokyo, "2026-10-09", 21, 0, 0))
	exact := late.Now.Add(-MinSpinGap)
	late.LastDailySpunAt = &exact
	if got := BuildToday(late); got.State != StateOpen {
		t.Errorf("exactly 12h after the last spin: state = %s, want open", got.State)
	}
	short := late.Now.Add(-MinSpinGap).Add(time.Nanosecond)
	late.LastDailySpunAt = &short
	if got := BuildToday(late); got.State != StateWaiting {
		t.Errorf("one nanosecond under 12h: state = %s, want waiting", got.State)
	}
	// And the screen then names an opening at or after the end of the gap.
	got := BuildToday(late)
	if time.Time(got.NextOpenAt).Before(short.Add(MinSpinGap)) {
		t.Errorf("next_open_at %v is inside the gap", time.Time(got.NextOpenAt))
	}
}

func TestBuildToday_NextOpenHonoursTheGapAfterAZoneChange(t *testing.T) {
	// Kiritimati (UTC+14) to Pago Pago (UTC-11): the local date goes back a
	// day, and a spin made at 20:05 Kiritimati time is 19:05 the day before
	// in Pago Pago, one hour before its 8 PM.
	kir := mustLoc(t, "Pacific/Kiritimati")
	pago := mustLoc(t, "Pacific/Pago_Pago")
	spun := at(t, kir, "2026-10-07", 20, 5, 0)

	in := baseInput(t, "Pacific/Pago_Pago", spun.Add(time.Minute))
	in.LastDailySpunAt = &spun
	// In Pago Pago it is 19:06 on the 6th: waiting, and the next opening is
	// 20:00 on the 6th, which is under 12 hours after the spin. It must not
	// be offered.
	got := BuildToday(in)
	if got.State != StateWaiting {
		t.Fatalf("state = %s, want waiting", got.State)
	}
	earliest := spun.Add(MinSpinGap)
	if time.Time(got.NextOpenAt).Before(earliest) {
		t.Fatalf("next_open_at %v is inside the gap (before %v)", time.Time(got.NextOpenAt), earliest)
	}
	if l := time.Time(got.NextOpenAt).In(pago); l.Hour() != OpenHour || l.Minute() != 0 {
		t.Errorf("next_open_at reads %v, want 20:00", l)
	}
	// It is the first opening at or after the gap, not a later one.
	prior := PrevDate(LocalDate(time.Time(got.NextOpenAt), pago), pago)
	if prev := OpensAt(prior, pago); !prev.Before(earliest) {
		t.Errorf("an earlier opening %v is also outside the gap", prev)
	}
}

func TestBuildToday_StampTimeIsInTheSpinsOwnZone(t *testing.T) {
	cases := []struct {
		name string
		zone string
		date string
		h, m int
		want string
	}{
		{"evening", ny, "2026-10-07", 20, 2, "8:02 PM"},
		{"just after midnight", ny, "2026-10-08", 0, 5, "12:05 AM"},
		{"noon", ny, "2026-10-07", 12, 0, "12:00 PM"},
		{"single digit hour has no leading zero", ny, "2026-10-07", 9, 7, "9:07 AM"},
		{"Kolkata half hour zone", "Asia/Kolkata", "2026-10-07", 20, 2, "8:02 PM"},
		{"Kathmandu 45 minute zone", "Asia/Kathmandu", "2026-10-07", 20, 2, "8:02 PM"},
		{"Lord Howe", "Australia/Lord_Howe", "2026-10-07", 23, 59, "11:59 PM"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := mustLoc(t, c.zone)
			when := at(t, loc, c.date, c.h, c.m, 0)
			in := baseInput(t, c.zone, when.Add(10*time.Minute))
			in.Spin = spinAt(t, c.zone, when, KindDaily, 0)
			got := BuildToday(in)
			if got.Spin == nil || got.Spin.SpunLocalTime != c.want {
				t.Fatalf("spun_local_time = %q, want %q", got.Spin.SpunLocalTime, c.want)
			}
		})
	}

	t.Run("stays in the zone the spin was made in after the stored zone changed", func(t *testing.T) {
		tokyo := mustLoc(t, "Asia/Tokyo")
		when := at(t, tokyo, "2026-10-07", 20, 2, 0)
		in := baseInput(t, "America/Los_Angeles", when.Add(time.Minute))
		in.Spin = spinAt(t, "Asia/Tokyo", when, KindDaily, 0)
		in.Spin.LocalDate = LocalDate(in.Now, in.Loc)
		got := BuildToday(in)
		if got.Spin.SpunLocalTime != "8:02 PM" {
			t.Fatalf("spun_local_time = %q, want 8:02 PM (Tokyo time), not Los Angeles time", got.Spin.SpunLocalTime)
		}
	})

	t.Run("falls back to the current zone when the spin's zone cannot load", func(t *testing.T) {
		loc := mustLoc(t, ny)
		when := at(t, loc, "2026-10-07", 20, 2, 0)
		in := baseInput(t, ny, when.Add(time.Minute))
		in.Spin = spinAt(t, ny, when, KindDaily, 0)
		in.Spin.TZName = "Not/AZone"
		got := BuildToday(in)
		if got.Spin.SpunLocalTime != "8:02 PM" {
			t.Fatalf("spun_local_time = %q, want 8:02 PM in the current zone", got.Spin.SpunLocalTime)
		}
	})
}

func TestBuildToday_NextOpenWeekdayIsTheLocalWeekdayNotTheUTCOne(t *testing.T) {
	for _, zone := range []string{ny, "America/Los_Angeles"} {
		t.Run(zone, func(t *testing.T) {
			loc := mustLoc(t, zone)
			// Friday 2026-10-09 morning: the next opening is Friday 20:00
			// local, which is already Saturday in UTC.
			now := at(t, loc, "2026-10-09", 9, 0, 0)
			got := BuildToday(baseInput(t, zone, now))
			if got.NextOpenWeekday != "Friday" {
				t.Fatalf("next_open_weekday = %q, want Friday", got.NextOpenWeekday)
			}
			if utcDay := time.Time(got.NextOpenAt).UTC().Weekday(); utcDay != time.Saturday {
				t.Fatalf("test setup: the opening is %v in UTC, want Saturday", utcDay)
			}
		})
	}

	// After the spin on Friday evening the next opening is Saturday 20:00
	// local: Sunday in UTC.
	loc := mustLoc(t, ny)
	when := at(t, loc, "2026-10-09", 20, 30, 0)
	in := baseInput(t, ny, when.Add(time.Minute))
	in.Spin = spinAt(t, ny, when, KindDaily, 0)
	in.LastDailySpunAt = &when
	got := BuildToday(in)
	if got.NextOpenWeekday != "Saturday" {
		t.Errorf("after a Friday spin next_open_weekday = %q, want Saturday", got.NextOpenWeekday)
	}
	if time.Time(got.NextOpenAt).UTC().Weekday() != time.Sunday {
		t.Errorf("setup: opening is %v in UTC, want Sunday", time.Time(got.NextOpenAt).UTC().Weekday())
	}
}

func TestBuildToday_WeekSection(t *testing.T) {
	loc := mustLoc(t, ny)
	now := at(t, loc, "2026-10-08", 16, 0, 0) // Thursday
	in := baseInput(t, ny, now)
	in.PostedDates = []Date{
		mustDate(t, "2026-10-05"), mustDate(t, "2026-10-06"), mustDate(t, "2026-10-07"),
		mustDate(t, "2026-10-04"), // last week: not counted, not shown
	}
	got := BuildToday(in)
	w := got.Week
	if w.Start.String() != "2026-10-05" || w.Goal != 4 || w.Posted != 3 || w.GoalMet || w.TodayIndex != 3 {
		t.Fatalf("week = %+v", w)
	}
	if len(w.Days) != 7 {
		t.Fatalf("days = %d", len(w.Days))
	}
	letters := ""
	for i, d := range w.Days {
		letters += d.Letter
		wantPosted := i >= 0 && i <= 2
		if d.Posted != wantPosted {
			t.Errorf("day %d (%s) posted = %v", i, d.Date, d.Posted)
		}
	}
	if letters != "MTWTFSS" || w.Days[3].Weekday != "Thursday" {
		t.Errorf("letters %q, day 3 %q", letters, w.Days[3].Weekday)
	}

	in.PostedDates = append(in.PostedDates, mustDate(t, "2026-10-08"))
	if got := BuildToday(in); !got.Week.GoalMet || got.Week.Posted != 4 {
		t.Errorf("four posted days: %+v", got.Week)
	}

	// Sunday is index 6, Monday index 0.
	sun := BuildToday(baseInput(t, ny, at(t, loc, "2026-10-11", 12, 0, 0)))
	mon := BuildToday(baseInput(t, ny, at(t, loc, "2026-10-12", 12, 0, 0)))
	if sun.Week.TodayIndex != 6 || mon.Week.TodayIndex != 0 || mon.Week.Start.String() != "2026-10-12" {
		t.Errorf("sunday index %d, monday index %d start %v", sun.Week.TodayIndex, mon.Week.TodayIndex, mon.Week.Start)
	}
}

func TestBuildToday_WireShape(t *testing.T) {
	loc := mustLoc(t, ny)
	when := at(t, loc, "2026-10-07", 20, 2, 0)
	in := baseInput(t, ny, when.Add(30*time.Second))
	in.Spin = spinAt(t, ny, when, KindDaily, 0)
	raw, err := json.Marshal(BuildToday(in))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"now", "time_zone", "local_date", "state", "window", "next_open_at", "next_open_weekday", "spin", "wheel", "week", "bonus"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in %s", key, raw)
		}
	}
	// Instants are RFC 3339 in UTC; dates are plain dates.
	if got := m["now"]; got != "2026-10-08T00:02:30.000Z" {
		t.Errorf("now = %v", got)
	}
	if got := m["local_date"]; got != "2026-10-07" {
		t.Errorf("local_date = %v", got)
	}
	spin := m["spin"].(map[string]any)
	for _, key := range []string{"id", "local_date", "kind", "seq", "category", "winner_index", "wheel", "spun_at", "spun_local_time", "result"} {
		if _, ok := spin[key]; !ok {
			t.Errorf("spin is missing %q", key)
		}
	}
	if spin["spun_at"] != "2026-10-08T00:02:00.000Z" || spin["spun_local_time"] != "8:02 PM" {
		t.Errorf("spun_at %v spun_local_time %v", spin["spun_at"], spin["spun_local_time"])
	}
	win := m["window"].(map[string]any)
	if win["opens_at"] != "2026-10-08T00:00:00.000Z" || win["closes_at"] != "2026-10-08T04:00:00.000Z" {
		t.Errorf("window = %v", win)
	}
	week := m["week"].(map[string]any)
	for _, key := range []string{"start", "goal", "posted", "goal_met", "today_index", "days"} {
		if _, ok := week[key]; !ok {
			t.Errorf("week is missing %q", key)
		}
	}
	day := week["days"].([]any)[0].(map[string]any)
	for _, key := range []string{"date", "letter", "weekday", "posted"} {
		if _, ok := day[key]; !ok {
			t.Errorf("a week day is missing %q", key)
		}
	}
	bonus := m["bonus"].(map[string]any)
	if _, ok := bonus["held"]; !ok {
		t.Error("bonus has no held")
	}
	if _, ok := bonus["can_use"]; !ok {
		t.Error("bonus has no can_use")
	}

	// With no spin, the spin is null and the wheel is a list.
	in2 := baseInput(t, ny, at(t, loc, "2026-10-07", 12, 0, 0))
	in2.Wheel = nil
	raw2, _ := json.Marshal(BuildToday(in2))
	if !strings.Contains(string(raw2), `"spin":null`) || !strings.Contains(string(raw2), `"wheel":[]`) {
		t.Errorf("no-spin body = %s", raw2)
	}
}

// --- the entry phase: posted, skipped, the entry summary, the skipped flag ---

// settledAs returns a copy of s with the given result and settled instant
// (nil for none).
func settledAs(s *Spin, result SpinResult, settledAt *time.Time) *Spin {
	c := *s
	c.Result = result
	c.SettledAt = settledAt
	return &c
}

func instantPtr(t time.Time) *time.Time { return &t }

// entrySummary is a one photo summary as the service would hand it over.
func entrySummary(title string) *EntryRef {
	return &EntryRef{
		ID:         "entry-" + title,
		Title:      title,
		Stars:      7,
		Reaction:   ptr("fire"),
		CoverKey:   "entries/u1/0000000000000001.jpg",
		PhotoCount: 1,
	}
}

func TestBuildToday_PostedAndSkippedShowTheDaysSpin(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	now := at(t, loc, "2026-10-07", 21, 15, 0)
	daily := spinAt(t, ny, spun, KindDaily, 0)
	entry := entrySummary("Dune")

	cases := []struct {
		name      string
		spin      *Spin
		entry     *EntryRef
		settled   bool
		wantState TodayState
		wantEntry *EntryRef
		wantUse   bool
	}{
		{"an open spin is a result", settledAs(daily, ResultOpen, nil), nil, false, StateResult, nil, true},
		{"a result shows no summary", settledAs(daily, ResultOpen, nil), entry, false, StateResult, nil, true},
		{"posted", settledAs(daily, ResultPosted, instantPtr(now)), entry, true, StatePosted, entry, false},
		{"posted and the entry was deleted", settledAs(daily, ResultPosted, instantPtr(now)), nil, true, StatePosted, nil, false},
		{"posted by a pretend day with no settled time", settledAs(daily, ResultPosted, nil), nil, true, StatePosted, nil, false},
		{"skipped", settledAs(daily, ResultSkipped, instantPtr(now)), nil, true, StateSkipped, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := baseInput(t, ny, now)
			in.Spin, in.Entry, in.Settled = c.spin, c.entry, c.settled
			in.LastDailySpunAt = &spun
			in.BonusHeld = true
			// Yesterday's summary never leaks into today's.
			in.YesterdayEntry = entrySummary("Yesterday")

			got := BuildToday(in)
			if got.State != c.wantState {
				t.Fatalf("state = %s, want %s", got.State, c.wantState)
			}
			if got.Spin == nil || got.Spin.ID != daily.ID || got.Spin.Result != c.spin.Result {
				t.Fatalf("spin = %+v, want the day's spin with result %s", got.Spin, c.spin.Result)
			}
			if len(got.Wheel) != 6 || got.Wheel[0].Name != "Meal" {
				t.Errorf("wheel = %+v, want the spin's own wheel", got.Wheel)
			}
			if got.Entry != c.wantEntry {
				t.Errorf("entry = %+v, want %+v", got.Entry, c.wantEntry)
			}
			if got.Bonus.CanUse != c.wantUse || !got.Bonus.Held {
				t.Errorf("bonus = %+v, want held and can_use %v", got.Bonus, c.wantUse)
			}
			// The next opening is Phase 2's: tomorrow 8 PM.
			if want := at(t, loc, "2026-10-08", 20, 0, 0); !time.Time(got.NextOpenAt).Equal(want) {
				t.Errorf("next_open_at = %v, want %v", time.Time(got.NextOpenAt), want)
			}
		})
	}
}

func TestBuildToday_BonusThenPostAndBonusThenSkip(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	daily := spinAt(t, ny, spun, KindDaily, 0) // seq 0 stays open for good
	bonus := spinAt(t, ny, at(t, loc, "2026-10-07", 21, 0, 0), KindBonus, 1)
	settledAt := at(t, loc, "2026-10-07", 21, 30, 0)

	for _, c := range []struct {
		name   string
		result SpinResult
		state  TodayState
	}{
		{"post", ResultPosted, StatePosted},
		{"skip", ResultSkipped, StateSkipped},
	} {
		t.Run(c.name, func(t *testing.T) {
			// The service hands over the date's latest spin: the bonus spin.
			latest := settledAs(bonus, c.result, instantPtr(settledAt))

			in := baseInput(t, ny, at(t, loc, "2026-10-07", 21, 45, 0))
			in.Spin = latest
			in.Entry = entrySummary("Dune")
			in.Settled = true
			in.BonusHeld = true
			in.LastDailySpunAt = &spun
			got := BuildToday(in)
			if got.State != c.state {
				t.Fatalf("that evening: state = %s, want %s", got.State, c.state)
			}
			if got.Spin == nil || got.Spin.Kind != KindBonus || got.Spin.Seq != 1 || got.Spin.Result != c.result {
				t.Fatalf("that evening: spin = %+v, want the bonus spin %s", got.Spin, c.result)
			}
			if got.Bonus.CanUse {
				t.Error("a bonus offered on a settled day")
			}

			// The next morning, 00:30. Seq 0 is open, but the latest spin is
			// not, and it was settled before midnight: waiting, never late.
			in = baseInput(t, ny, at(t, loc, "2026-10-08", 0, 30, 0))
			in.YesterdaySpin = latest
			in.YesterdayEntry = entrySummary("Dune")
			in.LastDailySpunAt = &spun
			got = BuildToday(in)
			if got.State != StateWaiting || got.Spin != nil || got.Entry != nil {
				t.Fatalf("next morning: state %s spin %v entry %v, want waiting with none", got.State, got.Spin, got.Entry)
			}
		})
	}

	// Had the daily spin been the one handed over, it would be late. That is
	// why the service hands over the latest spin of the date.
	in := baseInput(t, ny, at(t, loc, "2026-10-08", 0, 30, 0))
	in.YesterdaySpin = daily
	in.LastDailySpunAt = &spun
	if got := BuildToday(in); got.State != StateLate {
		t.Errorf("an open seq 0 handed over alone: state = %s, want late", got.State)
	}
}

func TestBuildToday_LateWindowRows(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	y := spinAt(t, ny, spun, KindDaily, 0)
	closes := y.WindowClosesAt // the midnight that starts 2026-10-08
	entry := entrySummary("Dune")
	morning := func(h, m, s int) time.Time { return at(t, loc, "2026-10-08", h, m, s) }
	evening := at(t, loc, "2026-10-07", 22, 0, 0)

	cases := []struct {
		name      string
		now       time.Time
		result    SpinResult
		settledAt *time.Time
		entry     *EntryRef // yesterday's summary as the service would hand it over
		gapShut   bool      // a daily spin 5 hours ago, in another zone
		wantState TodayState
		wantSpin  bool
		wantEntry *EntryRef
	}{
		{"an open yesterday is late", morning(0, 30, 0), ResultOpen, nil, nil, false, StateLate, true, nil},
		{"posted at 00:30 is posted with yesterday's spin and entry", morning(0, 45, 0), ResultPosted, instantPtr(morning(0, 30, 0)), entry, false, StatePosted, true, entry},
		{"skipped at 00:30 is skipped", morning(0, 45, 0), ResultSkipped, instantPtr(morning(0, 30, 0)), nil, false, StateSkipped, true, nil},
		{"posted exactly at the close is posted", morning(0, 45, 0), ResultPosted, instantPtr(closes), entry, false, StatePosted, true, entry},
		{"posted one nanosecond before the close is waiting", morning(0, 45, 0), ResultPosted, instantPtr(closes.Add(-time.Nanosecond)), entry, false, StateWaiting, false, nil},
		{"posted at 22:00 is waiting", morning(0, 30, 0), ResultPosted, instantPtr(evening), entry, false, StateWaiting, false, nil},
		{"skipped at 22:00 is waiting", morning(0, 30, 0), ResultSkipped, instantPtr(evening), nil, false, StateWaiting, false, nil},
		{"a pretend posted day has no settled time and is waiting", morning(0, 30, 0), ResultPosted, nil, nil, false, StateWaiting, false, nil},
		{"posted at 00:30 is still posted at 19:59:59", morning(19, 59, 59), ResultPosted, instantPtr(morning(0, 30, 0)), entry, false, StatePosted, true, entry},
		{"at exactly 20:00 the same day gives open", morning(20, 0, 0), ResultPosted, instantPtr(morning(0, 30, 0)), entry, false, StateOpen, false, nil},
		{"after 20:00 with the wheel shut it is waiting", morning(20, 30, 0), ResultPosted, instantPtr(morning(0, 30, 0)), entry, true, StateWaiting, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := baseInput(t, ny, c.now)
			in.YesterdaySpin = settledAs(y, c.result, c.settledAt)
			in.YesterdayEntry = c.entry
			in.BonusHeld = true
			last := spun
			if c.gapShut {
				last = c.now.Add(-5 * time.Hour)
			}
			in.LastDailySpunAt = &last

			got := BuildToday(in)
			if got.State != c.wantState {
				t.Fatalf("state = %s, want %s", got.State, c.wantState)
			}
			if (got.Spin != nil) != c.wantSpin {
				t.Fatalf("spin = %+v, want one: %v", got.Spin, c.wantSpin)
			}
			if got.Spin != nil {
				if got.Spin.ID != y.ID || got.Spin.LocalDate.String() != "2026-10-07" || got.Spin.Result != c.result {
					t.Errorf("spin = %+v, want last night's with result %s", got.Spin, c.result)
				}
			}
			if got.Entry != c.wantEntry {
				t.Errorf("entry = %+v, want %+v", got.Entry, c.wantEntry)
			}
			if got.LocalDate.String() != "2026-10-08" {
				t.Errorf("local_date = %v, want the new date", got.LocalDate)
			}
			if got.Bonus.CanUse {
				t.Error("a bonus offered outside the result state")
			}
		})
	}
}

func TestBuildToday_AnEntryOfTodayAndOneOfYesterdayStaySeparate(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-08", 20, 30, 0)
	now := at(t, loc, "2026-10-08", 21, 0, 0)
	in := baseInput(t, ny, now)
	in.Spin = settledAs(spinAt(t, ny, spun, KindDaily, 0), ResultPosted, instantPtr(now))
	in.Entry = entrySummary("Today")
	in.YesterdayEntry = entrySummary("Yesterday")
	in.LastDailySpunAt = &spun
	got := BuildToday(in)
	if got.Entry == nil || got.Entry.Title != "Today" {
		t.Fatalf("entry = %+v, want today's", got.Entry)
	}
}

func TestBuildToday_EntrySummaryCarriesTheCoverAndTheCount(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	now := at(t, loc, "2026-10-07", 21, 0, 0)
	entry := &Entry{
		ID:       "entry-1",
		Title:    "Dune",
		Stars:    9,
		Reaction: ptr("wow"),
		Late:     false,
		// Listed out of order on purpose: the cover is position 0.
		Photos: []EntryPhoto{
			{ID: "p-2", Position: 2, Key: "entries/u1/2222222222222222.png"},
			{ID: "p-0", Position: 0, Key: "entries/u1/0000000000000000.jpg"},
			{ID: "p-1", Position: 1, Key: "entries/u1/1111111111111111.webp"},
		},
	}

	in := baseInput(t, ny, now)
	in.Spin = settledAs(spinAt(t, ny, spun, KindDaily, 0), ResultPosted, instantPtr(now))
	in.Entry = EntryRefOf(entry)
	got := BuildToday(in)
	if got.State != StatePosted || got.Entry == nil {
		t.Fatalf("state %s entry %v, want posted with a summary", got.State, got.Entry)
	}
	e := got.Entry
	if e.PhotoCount != 3 {
		t.Errorf("PhotoCount = %d, want 3", e.PhotoCount)
	}
	if e.CoverKey != "entries/u1/0000000000000000.jpg" {
		t.Errorf("CoverKey = %q, want the key at position 0", e.CoverKey)
	}
	if e.ID != "entry-1" || e.Title != "Dune" || e.Stars != 9 || e.Late || e.Reaction == nil || *e.Reaction != "wow" {
		t.Errorf("summary = %+v", e)
	}
	if e.PhotoURL != "" {
		t.Errorf("PhotoURL = %q, want empty below the HTTP layer", e.PhotoURL)
	}

	// After a delete the service has no entry to hand over.
	in.Entry = EntryRefOf(nil)
	got = BuildToday(in)
	if got.State != StatePosted || got.Entry != nil {
		t.Errorf("after a delete: state %s entry %v, want posted with none", got.State, got.Entry)
	}
}

func TestBuildToday_WeekDaySkippedFlag(t *testing.T) {
	loc := mustLoc(t, ny)
	now := at(t, loc, "2026-10-09", 12, 0, 0) // Friday; the week starts 2026-10-05
	in := baseInput(t, ny, now)
	in.PostedDates = []Date{mustDate(t, "2026-10-05"), mustDate(t, "2026-10-06")}
	in.SkippedDates = []Date{
		mustDate(t, "2026-10-07"),
		mustDate(t, "2026-10-07"), // listed twice: still one flagged day
		mustDate(t, "2026-10-06"), // also posted: not flagged
		mustDate(t, "2026-10-04"), // last week: not in this week's days
		mustDate(t, "2026-10-12"), // next week
	}
	got := BuildToday(in)

	wantPosted := map[string]bool{"2026-10-05": true, "2026-10-06": true}
	wantSkipped := map[string]bool{"2026-10-07": true}
	flagged := 0
	for _, d := range got.Week.Days {
		key := d.Date.String()
		if d.Posted != wantPosted[key] {
			t.Errorf("%s posted = %v", key, d.Posted)
		}
		if d.Skipped != wantSkipped[key] {
			t.Errorf("%s skipped = %v, want %v", key, d.Skipped, wantSkipped[key])
		}
		if d.Skipped {
			flagged++
		}
	}
	if flagged != 1 {
		t.Errorf("%d days flagged skipped, want exactly 1", flagged)
	}
	// Skipped days are not posted days: the count and the goal are Phase 2's.
	if got.Week.Posted != 2 || got.Week.GoalMet {
		t.Errorf("week = %+v, want 2 posted and the goal not met", got.Week)
	}

	// No skipped dates: no flags.
	in.SkippedDates = nil
	for _, d := range BuildToday(in).Week.Days {
		if d.Skipped {
			t.Errorf("%s flagged skipped with no skipped dates", d.Date)
		}
	}
}

func TestBuildToday_WireShapeOfTheEntrySummaryAndTheSkippedFlag(t *testing.T) {
	loc := mustLoc(t, ny)
	spun := at(t, loc, "2026-10-07", 20, 30, 0)
	now := at(t, loc, "2026-10-07", 21, 15, 0)
	posted := settledAs(spinAt(t, ny, spun, KindDaily, 0), ResultPosted, instantPtr(now))

	in := baseInput(t, ny, now)
	in.Spin = posted
	in.Entry = &EntryRef{
		ID:         "e1",
		Title:      "Dune",
		Stars:      7,
		CoverKey:   "entries/u1/secretcoverkey.jpg",
		PhotoURL:   "https://cdn.example.test/cover.jpg",
		PhotoCount: 3,
	}
	in.SkippedDates = []Date{mustDate(t, "2026-10-06")}
	raw, err := json.Marshal(BuildToday(in))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["state"] != "posted" {
		t.Errorf("state = %v, want posted", m["state"])
	}
	entry, ok := m["entry"].(map[string]any)
	if !ok {
		t.Fatalf("entry = %v in %s", m["entry"], raw)
	}
	keys := make([]string, 0, len(entry))
	for k := range entry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := []string{"id", "late", "photo_count", "photo_url", "reaction", "stars", "title"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("entry keys = %v, want %v", keys, want)
	}
	if entry["id"] != "e1" || entry["late"] != false || entry["title"] != "Dune" || entry["stars"] != float64(7) ||
		entry["photo_count"] != float64(3) || entry["photo_url"] != "https://cdn.example.test/cover.jpg" {
		t.Errorf("entry = %v", entry)
	}
	if v, present := entry["reaction"]; !present || v != nil {
		t.Errorf("reaction = %v (present %v), want null", v, present)
	}
	if strings.Contains(string(raw), "secretcoverkey") {
		t.Errorf("the cover key leaked into %s", raw)
	}

	// Every week day carries the skipped flag; the flagged one is Tuesday.
	days := m["week"].(map[string]any)["days"].([]any)
	for i, d := range days {
		day := d.(map[string]any)
		v, present := day["skipped"]
		if !present {
			t.Fatalf("week day %d has no skipped key", i)
		}
		if want := i == 1; v != want {
			t.Errorf("week day %d skipped = %v, want %v", i, v, want)
		}
	}

	// No entry: the key is there and null, in a result and in a skipped day.
	for _, c := range []struct {
		name string
		spin *Spin
		want string
	}{
		{"result", settledAs(posted, ResultOpen, nil), "result"},
		{"skipped", settledAs(posted, ResultSkipped, instantPtr(now)), "skipped"},
	} {
		in := baseInput(t, ny, now)
		in.Spin = c.spin
		raw, err := json.Marshal(BuildToday(in))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"entry":null`) || !strings.Contains(string(raw), `"state":"`+c.want+`"`) {
			t.Errorf("%s: body = %s", c.name, raw)
		}
	}
}
