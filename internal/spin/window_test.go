package spin

import (
	"fmt"
	"testing"
	"time"
)

func mustLoc(t testing.TB, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

func utc(y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, time.UTC)
}

func mustDate(t testing.TB, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

// The expected instants below were worked out by hand from each zone's
// offsets on the day (comments give the local reading), not from the code
// under test.
func TestWindow_KnownZonesAndTransitionDays(t *testing.T) {
	cases := []struct {
		name                string
		zone                string
		date                string
		start, opens, close time.Time
	}{
		// New York: spring forward 2026-03-08 02:00 -> 03:00 (a 23 hour day).
		{"New York spring forward", "America/New_York", "2026-03-08",
			utc(2026, 3, 8, 5, 0), utc(2026, 3, 9, 0, 0), utc(2026, 3, 9, 4, 0)},
		// New York: fall back 2026-11-01 02:00 -> 01:00 (a 25 hour day).
		{"New York fall back", "America/New_York", "2026-11-01",
			utc(2026, 11, 1, 4, 0), utc(2026, 11, 2, 1, 0), utc(2026, 11, 2, 5, 0)},
		// London: 01:00 GMT -> 02:00 BST on 2026-03-29.
		{"London spring forward", "Europe/London", "2026-03-29",
			utc(2026, 3, 29, 0, 0), utc(2026, 3, 29, 19, 0), utc(2026, 3, 29, 23, 0)},
		// London: 02:00 BST -> 01:00 GMT on 2026-10-25.
		{"London fall back", "Europe/London", "2026-10-25",
			utc(2026, 10, 24, 23, 0), utc(2026, 10, 25, 20, 0), utc(2026, 10, 26, 0, 0)},
		{"Kolkata half hour", "Asia/Kolkata", "2026-06-15",
			utc(2026, 6, 14, 18, 30), utc(2026, 6, 15, 14, 30), utc(2026, 6, 15, 18, 30)},
		{"Kathmandu 45 minutes", "Asia/Kathmandu", "2026-06-15",
			utc(2026, 6, 14, 18, 15), utc(2026, 6, 15, 14, 15), utc(2026, 6, 15, 18, 15)},
		// Lord Howe: 30 minute shifts. 2026-04-05 02:00 +11 -> 01:30 +10:30.
		{"Lord Howe falls back half an hour", "Australia/Lord_Howe", "2026-04-05",
			utc(2026, 4, 4, 13, 0), utc(2026, 4, 5, 9, 30), utc(2026, 4, 5, 13, 30)},
		// 2026-10-04 02:00 +10:30 -> 02:30 +11.
		{"Lord Howe springs forward half an hour", "Australia/Lord_Howe", "2026-10-04",
			utc(2026, 10, 3, 13, 30), utc(2026, 10, 4, 9, 0), utc(2026, 10, 4, 13, 0)},
		{"Kiritimati +14", "Pacific/Kiritimati", "2026-06-15",
			utc(2026, 6, 14, 10, 0), utc(2026, 6, 15, 6, 0), utc(2026, 6, 15, 10, 0)},
		// Samoa skipped 2011-12-30 (UTC-10 on the 29th, UTC+14 from the 31st).
		{"Apia the day before the skipped day", "Pacific/Apia", "2011-12-29",
			utc(2011, 12, 29, 10, 0), utc(2011, 12, 30, 6, 0), utc(2011, 12, 30, 10, 0)},
		{"Apia the day after the skipped day", "Pacific/Apia", "2011-12-31",
			utc(2011, 12, 30, 10, 0), utc(2011, 12, 31, 6, 0), utc(2011, 12, 31, 10, 0)},
		// Havana: midnight skipped on 2026-03-08 (00:00 -> 01:00), so the
		// date begins at 01:00 local.
		{"Havana skipped midnight", "America/Havana", "2026-03-08",
			utc(2026, 3, 8, 5, 0), utc(2026, 3, 9, 0, 0), utc(2026, 3, 9, 4, 0)},
		// Havana: 01:00 -> 00:00 on 2026-11-01, so midnight happens twice and
		// the date begins at the first one.
		{"Havana repeated midnight", "America/Havana", "2026-11-01",
			utc(2026, 11, 1, 4, 0), utc(2026, 11, 2, 1, 0), utc(2026, 11, 2, 5, 0)},
		// Cairo: 2026-04-24 00:00 -> 01:00 (midnight skipped).
		{"Cairo skipped midnight", "Africa/Cairo", "2026-04-24",
			utc(2026, 4, 23, 22, 0), utc(2026, 4, 24, 17, 0), utc(2026, 4, 24, 21, 0)},
		// Cairo: 2026-10-29 24:00 -> 23:00, so the last hour of the 29th
		// happens twice and the 29th is 25 hours long.
		{"Cairo repeated last hour", "Africa/Cairo", "2026-10-29",
			utc(2026, 10, 28, 21, 0), utc(2026, 10, 29, 17, 0), utc(2026, 10, 29, 22, 0)},
		// Santiago: 2026-04-04 24:00 -> 23:00 (a 25 hour day).
		{"Santiago repeated last hour", "America/Santiago", "2026-04-04",
			utc(2026, 4, 4, 3, 0), utc(2026, 4, 4, 23, 0), utc(2026, 4, 5, 4, 0)},
		// Santiago: 2026-09-06 00:00 -> 01:00 (midnight skipped).
		{"Santiago skipped midnight", "America/Santiago", "2026-09-06",
			utc(2026, 9, 6, 4, 0), utc(2026, 9, 6, 23, 0), utc(2026, 9, 7, 3, 0)},
		{"Santiago the day before the skipped midnight", "America/Santiago", "2026-09-05",
			utc(2026, 9, 5, 4, 0), utc(2026, 9, 6, 0, 0), utc(2026, 9, 6, 4, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := mustLoc(t, c.zone)
			d := mustDate(t, c.date)
			if got := DayStart(d, loc); !got.Equal(c.start) {
				t.Errorf("DayStart = %v, want %v", got.UTC(), c.start)
			}
			w := WindowFor(d, loc)
			if !w.OpensAt.Equal(c.opens) {
				t.Errorf("OpensAt = %v, want %v", w.OpensAt.UTC(), c.opens)
			}
			if !w.ClosesAt.Equal(c.close) {
				t.Errorf("ClosesAt = %v, want %v", w.ClosesAt.UTC(), c.close)
			}
			if got := LocalDate(w.ClosesAt.Add(-time.Nanosecond), loc); got != d {
				t.Errorf("the instant before ClosesAt is on %v, want %v", got, d)
			}
			if got := LocalDate(w.ClosesAt, loc); !got.After(d) {
				t.Errorf("ClosesAt is on %v, want a date after %v", got, d)
			}
		})
	}
}

func TestWindow_ApiaSkippedDay(t *testing.T) {
	loc := mustLoc(t, "Pacific/Apia")
	d29 := mustDate(t, "2011-12-29")
	d31 := mustDate(t, "2011-12-31")

	w := WindowFor(d29, loc)
	if got, want := LocalDate(w.ClosesAt, loc), d31; got != want {
		t.Fatalf("the 29th closes onto %v, want %v (the 30th never happened)", got, want)
	}
	// Late runs to 8 PM on the 31st, local.
	if got := w.LateUntil.In(loc); got.Day() != 31 || got.Hour() != 20 || got.Minute() != 0 {
		t.Errorf("LateUntil = %v, want 2011-12-31 20:00 local", got)
	}

	// From 21:00 on the 29th the next opening is 20:00 on the 31st.
	now := time.Date(2011, 12, 29, 21, 0, 0, 0, loc)
	next := NextOpen(now, loc)
	want := time.Date(2011, 12, 31, 20, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Errorf("NextOpen from 2011-12-29 21:00 = %v, want %v", next.In(loc), want)
	}

	if got := PrevDate(d31, loc); got != d29 {
		t.Errorf("PrevDate(2011-12-31) = %v, want 2011-12-29", got)
	}
	// A date that never happened has an empty window and no panic.
	d30 := mustDate(t, "2011-12-30")
	empty := WindowFor(d30, loc)
	if empty.ClosesAt.Before(empty.OpensAt) {
		t.Errorf("skipped date: closes %v before opens %v", empty.ClosesAt, empty.OpensAt)
	}
}

func TestWindow_NextOpenNeverAddsADay(t *testing.T) {
	ny := mustLoc(t, "America/New_York")
	// Saturday 2026-03-07 21:00 EST: tomorrow is the 23 hour day, and the
	// next opening is 20:00 EDT, 22 hours and 59 minutes... of clock time,
	// not 24 hours later.
	now := time.Date(2026, 3, 7, 21, 0, 0, 0, ny)
	want := time.Date(2026, 3, 8, 20, 0, 0, 0, ny)
	if got := NextOpen(now, ny); !got.Equal(want) {
		t.Fatalf("NextOpen = %v, want %v", got.In(ny), want)
	}
	if got := want.Sub(now); got != 22*time.Hour {
		t.Fatalf("test setup: %v between the two, want 22h (a skipped hour)", got)
	}

	// Exactly at the opening, the next opening is the following date's.
	at := time.Date(2026, 10, 7, 20, 0, 0, 0, ny)
	if got, want := NextOpen(at, ny), time.Date(2026, 10, 8, 20, 0, 0, 0, ny); !got.Equal(want) {
		t.Fatalf("NextOpen at the opening = %v, want %v", got.In(ny), want)
	}
	// One nanosecond before, it is that opening.
	if got := NextOpen(at.Add(-time.Nanosecond), ny); !got.Equal(at) {
		t.Fatalf("NextOpen just before the opening = %v, want %v", got.In(ny), at)
	}
}

func TestInstantOfLocal_GapsRepeatsAndOrdinaryTimes(t *testing.T) {
	civil := func(s string) time.Time {
		v, err := time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return v
	}
	cases := []struct {
		name   string
		zone   string
		local  string
		wantOK bool
		want   time.Time
	}{
		{"ordinary", "America/New_York", "2026-10-07T19:59:30", true, time.Date(2026, 10, 7, 23, 59, 30, 0, time.UTC)},
		{"New York gap", "America/New_York", "2026-03-08T02:30:00", false, time.Time{}},
		{"New York repeat takes the first", "America/New_York", "2026-11-01T01:30:00", true, time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)},
		{"Havana gap at midnight", "America/Havana", "2026-03-08T00:30:00", false, time.Time{}},
		{"Havana repeated midnight hour", "America/Havana", "2026-11-01T00:30:00", true, time.Date(2026, 11, 1, 4, 30, 0, 0, time.UTC)},
		{"Kathmandu", "Asia/Kathmandu", "2026-06-15T08:00:00", true, time.Date(2026, 6, 15, 2, 15, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := mustLoc(t, c.zone)
			got, ok := InstantOfLocal(civil(c.local), loc)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (instant %v)", ok, c.wantOK, got.UTC())
			}
			if c.wantOK && !got.Equal(c.want) {
				t.Fatalf("instant = %v, want %v", got.UTC(), c.want)
			}
		})
	}
}

// scanFirst walks forward one minute at a time from `from` and returns the
// first instant for which pred holds. It is a slow, obviously-correct oracle
// for the window code: every zone offset of 2026 and of Samoa in 2011 is a
// whole number of minutes, so no transition is stepped over.
func scanFirst(t *testing.T, from time.Time, pred func(time.Time) bool) time.Time {
	t.Helper()
	for i := 0; i < 60*60; i++ { // up to 60 hours
		at := from.Add(time.Duration(i) * time.Minute)
		if pred(at) {
			return at
		}
	}
	t.Fatalf("no instant found within 60 hours of %v", from)
	return time.Time{}
}

var windowZones = []string{
	"America/New_York", "America/Los_Angeles", "Europe/London", "Asia/Kolkata",
	"Asia/Kathmandu", "Australia/Lord_Howe", "Pacific/Kiritimati", "Pacific/Apia",
	"Pacific/Pago_Pago", "Asia/Tokyo", "America/Havana", "Africa/Cairo",
	"America/Santiago", "Pacific/Auckland", "Europe/Dublin",
}

// checkWindowAgainstOracle verifies one zone and one date against a minute
// by minute scan and against the invariants the plan names.
func checkWindowAgainstOracle(t *testing.T, loc *time.Location, d Date) {
	t.Helper()
	label := fmt.Sprintf("%s %v", loc, d)

	// Oracle: the first instant whose local date is d.
	from := d.utc().Add(-30 * time.Hour)
	wantStart := scanFirst(t, from, func(at time.Time) bool { return LocalDate(at, loc) == d })
	gotStart := DayStart(d, loc)
	if !gotStart.Equal(wantStart) {
		t.Fatalf("%s: DayStart = %v, oracle %v", label, gotStart.In(loc), wantStart.In(loc))
	}

	// Oracle: the first instant after the start whose local date is later.
	wantClose := scanFirst(t, wantStart, func(at time.Time) bool { return LocalDate(at, loc).After(d) })
	// Oracle: the first instant in the date that reads 20:00.
	wantOpen := scanFirst(t, wantStart, func(at time.Time) bool {
		l := at.In(loc)
		return DateOf(l) == d && l.Hour() == OpenHour && l.Minute() == 0
	})

	w := WindowFor(d, loc)
	if !w.OpensAt.Equal(wantOpen) {
		t.Fatalf("%s: OpensAt = %v, oracle %v", label, w.OpensAt.In(loc), wantOpen.In(loc))
	}
	if !w.ClosesAt.Equal(wantClose) {
		t.Fatalf("%s: ClosesAt = %v, oracle %v", label, w.ClosesAt.In(loc), wantClose.In(loc))
	}
	if !w.ClosesAt.After(w.OpensAt) {
		t.Fatalf("%s: ClosesAt %v is not after OpensAt %v", label, w.ClosesAt, w.OpensAt)
	}
	if got := LocalDate(w.ClosesAt, loc); !got.After(d) {
		t.Fatalf("%s: ClosesAt is on %v", label, got)
	}
	if got := LocalDate(w.ClosesAt.Add(-time.Nanosecond), loc); got != d {
		t.Fatalf("%s: the instant before ClosesAt is on %v", label, got)
	}

	// LateUntil is 20:00 on the date ClosesAt falls on.
	closeDate := LocalDate(w.ClosesAt, loc)
	if want := OpensAt(closeDate, loc); !w.LateUntil.Equal(want) {
		t.Fatalf("%s: LateUntil = %v, want %v", label, w.LateUntil.In(loc), want.In(loc))
	}
	if l := w.LateUntil.In(loc); l.Hour() != OpenHour || l.Minute() != 0 || DateOf(l) != closeDate {
		t.Fatalf("%s: LateUntil reads %v, want 20:00 on %v", label, l, closeDate)
	}

	// PrevDate is the date of the instant before the start.
	if prev := PrevDate(d, loc); !prev.Before(d) || LocalDate(gotStart.Add(-time.Nanosecond), loc) != prev {
		t.Fatalf("%s: PrevDate = %v", label, prev)
	}

	// NextOpen is always strictly after the instant it is asked from.
	for _, at := range []time.Time{
		gotStart,
		gotStart.Add(-time.Nanosecond),
		w.OpensAt.Add(-time.Nanosecond),
		w.OpensAt,
		w.ClosesAt.Add(-time.Nanosecond),
		w.ClosesAt,
	} {
		next := NextOpen(at, loc)
		if !next.After(at) {
			t.Fatalf("%s: NextOpen(%v) = %v is not after it", label, at.In(loc), next.In(loc))
		}
		if l := next.In(loc); l.Hour() != OpenHour || l.Minute() != 0 {
			t.Fatalf("%s: NextOpen(%v) = %v does not read 20:00", label, at.In(loc), l)
		}
	}
	if got := NextOpen(gotStart, loc); !got.Equal(w.OpensAt) {
		t.Fatalf("%s: NextOpen at the start of the day = %v, want the day's opening %v", label, got.In(loc), w.OpensAt.In(loc))
	}
	if got := NextOpen(w.OpensAt, loc); !got.Equal(w.LateUntil) {
		t.Fatalf("%s: NextOpen at the opening = %v, want %v", label, got.In(loc), w.LateUntil.In(loc))
	}

	// The first instant of the date is "waiting" (00:00:00 is the new date)
	// and the last millisecond is "open".
	first := BuildToday(TodayInput{Now: gotStart, Loc: loc, TZName: loc.String(), Goal: DefaultWeeklyGoal})
	if first.LocalDate != d || first.State != StateWaiting {
		t.Fatalf("%s: first instant gives date %v state %s, want %v waiting", label, first.LocalDate, first.State, d)
	}
	last := BuildToday(TodayInput{Now: w.ClosesAt.Add(-time.Millisecond), Loc: loc, TZName: loc.String(), Goal: DefaultWeeklyGoal})
	if last.LocalDate != d || last.State != StateOpen {
		t.Fatalf("%s: last millisecond gives date %v state %s, want %v open", label, last.LocalDate, last.State, d)
	}
}

func TestWindow_EveryDayOf2026InEveryZone(t *testing.T) {
	for _, name := range windowZones {
		t.Run(name, func(t *testing.T) {
			loc := mustLoc(t, name)
			for d := mustDate(t, "2026-01-01"); d.Year == 2026; d = d.AddDays(1) {
				checkWindowAgainstOracle(t, loc, d)
			}
		})
	}
}

func TestWindow_ApiaDecember2011(t *testing.T) {
	loc := mustLoc(t, "Pacific/Apia")
	skipped := mustDate(t, "2011-12-30")
	for d := mustDate(t, "2011-12-01"); d.Month == time.December && d.Year == 2011; d = d.AddDays(1) {
		if d == skipped {
			continue // this date never happened in Samoa
		}
		checkWindowAgainstOracle(t, loc, d)
	}
}

func TestWindow_TheYearsAroundTheBoundary(t *testing.T) {
	// The extend rule takes over after the zone file's last listed
	// transition; a day in 2040 and 2100 must still be right.
	ny := mustLoc(t, "America/New_York")
	for _, ds := range []string{"2040-03-11", "2040-11-04", "2100-03-14"} {
		checkWindowAgainstOracle(t, ny, mustDate(t, ds))
	}
}

func TestWindow_TransitionsFoundInTheZoneDatabase(t *testing.T) {
	// Walk each zone's 2026 transitions as the database lists them and check
	// the window code against what each one does to a date: a transition
	// that skips local midnight starts the date at the first instant after
	// the jump, one that repeats midnight starts it at the first pass.
	for _, name := range []string{"America/Havana", "Africa/Cairo", "America/Santiago", "America/New_York", "Europe/London", "Australia/Lord_Howe"} {
		loc := mustLoc(t, name)
		at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).In(loc)
		seen := 0
		for i := 0; i < 8; i++ {
			_, end := at.ZoneBounds()
			if end.IsZero() || end.Year() > 2026 {
				break
			}
			seen++
			before := end.Add(-time.Nanosecond).In(loc)
			after := end.In(loc)
			// Whatever the jump does, the two sides still bracket exactly one
			// date change or none, and the window code must agree with the
			// scan for the dates around it.
			for _, d := range []Date{DateOf(before), DateOf(after), DateOf(before).AddDays(-1), DateOf(after).AddDays(1)} {
				checkWindowAgainstOracle(t, loc, d)
			}
			at = end
		}
		if seen == 0 {
			t.Errorf("%s: found no 2026 transition in the zone database", name)
		}
	}
}
