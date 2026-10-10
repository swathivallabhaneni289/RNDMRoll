package spin

import (
	"encoding/json"
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
