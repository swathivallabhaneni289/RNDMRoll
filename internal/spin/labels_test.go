package spin

import (
	"testing"
	"time"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("zone %s: %v", name, err)
	}
	return loc
}

func TestDayInPlay(t *testing.T) {
	for _, zone := range []string{"America/New_York", "Europe/London", "Asia/Kolkata", "Asia/Kathmandu", "Australia/Lord_Howe"} {
		loc := mustZone(t, zone)
		for _, date := range []string{"2026-03-08", "2026-03-29", "2026-04-05", "2026-10-04", "2026-10-09", "2026-11-01"} {
			d, _ := ParseDate(date)
			prev := PrevDate(d, loc)
			opens := OpensAt(d, loc)
			check := func(at time.Time, want Date, wantLate bool) {
				t.Helper()
				got, late := DayInPlay(at, loc)
				if got != want || late != wantLate {
					t.Errorf("%s %s: %v %v, want %v %v", zone, at.In(loc), got, late, want, wantLate)
				}
			}
			check(opens.Add(-time.Millisecond), prev, true)
			check(opens, d, false)
			check(WindowFor(d, loc).ClosesAt.Add(-time.Second), d, false)
			check(DayStart(d, loc), prev, true)
		}
	}
	// Over a whole year the day in play changes exactly once per local day.
	loc := mustZone(t, "America/New_York")
	start := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	changes := 0
	last, _ := DayInPlay(start, loc)
	for at := start; at.Before(start.AddDate(1, 0, 0)); at = at.Add(30 * time.Minute) {
		if d, _ := DayInPlay(at, loc); d != last {
			changes++
			last = d
		}
	}
	if changes != 365 {
		t.Fatalf("day in play changed %d times in a year, want 365", changes)
	}
}

func TestViewerLabel(t *testing.T) {
	ny := mustZone(t, "America/New_York")
	now := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC) // Thu Oct 8, 9:00 PM in New York
	at := func(y int, m time.Month, d, h, min, s int) time.Time {
		return time.Date(y, m, d, h, min, s, 0, ny)
	}
	cases := []struct {
		at   time.Time
		want string
	}{
		{at(2026, 10, 8, 20, 0, 0), "8:00 PM"},
		{at(2026, 10, 8, 0, 5, 0), "12:05 AM"},
		{at(2026, 10, 7, 23, 59, 59), "Wed"},
		{at(2026, 10, 5, 12, 0, 0), "Mon"},
		{at(2026, 10, 2, 0, 0, 0), "Fri"},
		{at(2026, 10, 1, 23, 59, 59), "Oct 1"},
		{at(2026, 9, 30, 12, 0, 0), "Sep 30"},
		{at(2025, 12, 28, 12, 0, 0), "Dec 28"},
		{now.Add(2 * time.Hour), "11:00 PM"},
	}
	for _, c := range cases {
		if got := ViewerLabel(c.at, now, ny); got != c.want {
			t.Errorf("%v: %q, want %q", c.at, got, c.want)
		}
	}
	// One instant, two viewers: each reads it in their own zone.
	instant := time.Date(2026, 10, 9, 15, 22, 0, 0, time.UTC)
	later := time.Date(2026, 10, 9, 16, 0, 0, 0, time.UTC)
	if got := ViewerLabel(instant, later, ny); got != "11:22 AM" {
		t.Errorf("New York: %q", got)
	}
	if got := ViewerLabel(instant, later, mustZone(t, "Asia/Kolkata")); got != "8:52 PM" {
		t.Errorf("Kolkata: %q", got)
	}
}

func TestPostedLocalTime(t *testing.T) {
	ny := mustZone(t, "America/New_York")
	at := time.Date(2026, 10, 9, 15, 11, 0, 0, time.UTC)
	if got := PostedLocalTime(at, "Asia/Kolkata", ny); got != "8:41 PM" {
		t.Errorf("author's zone: %q", got)
	}
	for _, bad := range []string{"america/new_york", "Local", "../x", ""} {
		if got := PostedLocalTime(at, bad, ny); got != "11:11 AM" {
			t.Errorf("%q falls back to the viewer: %q", bad, got)
		}
	}
}
