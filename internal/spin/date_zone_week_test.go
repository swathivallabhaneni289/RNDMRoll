package spin

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- Date ---

func TestParseDate_AcceptsOnlyRealStrictDates(t *testing.T) {
	good := map[string]Date{
		"2026-10-07": {2026, time.October, 7},
		"2028-02-29": {2028, time.February, 29},
		"1900-01-01": {1900, time.January, 1},
		"2200-12-31": {2200, time.December, 31},
	}
	for in, want := range good {
		got, err := ParseDate(in)
		if err != nil || got != want {
			t.Errorf("ParseDate(%q) = %v, %v; want %v", in, got, err, want)
		}
		if got.String() != in {
			t.Errorf("String() = %q, want %q", got.String(), in)
		}
	}
	bad := []string{
		"", "2026-1-5", "2026-10-7", "26-10-07", "2026/10/07", "2026-10-07 ", " 2026-10-07",
		"2026-02-29", "2026-02-30", "2026-13-01", "2026-00-10", "2026-10-00", "2026-10-32",
		"1899-12-31", "2201-01-01", "0000-01-01", "+026-10-07", "2026-10-07T00:00:00Z", "abcd-ef-gh",
	}
	for _, in := range bad {
		if got, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) = %v, want an error", in, got)
		}
	}
}

func TestDate_ArithmeticCompareAndJSON(t *testing.T) {
	d := mustDate(t, "2026-02-27")
	if got := d.AddDays(2).String(); got != "2026-03-01" {
		t.Errorf("AddDays(2) = %s, want 2026-03-01 (not a leap year)", got)
	}
	leap := mustDate(t, "2028-02-27")
	if got := leap.AddDays(2).String(); got != "2028-02-29" {
		t.Errorf("leap AddDays(2) = %s, want 2028-02-29", got)
	}
	if got := mustDate(t, "2026-01-01").AddDays(-1).String(); got != "2025-12-31" {
		t.Errorf("AddDays(-1) = %s", got)
	}
	a, b := mustDate(t, "2026-10-07"), mustDate(t, "2026-10-08")
	if !a.Before(b) || !b.After(a) || a.Compare(a) != 0 || b.DaysSince(a) != 1 || a.DaysSince(b) != -1 {
		t.Error("Before, After, Compare or DaysSince disagree")
	}
	if mustDate(t, "2026-10-07").Weekday() != time.Wednesday {
		t.Errorf("2026-10-07 weekday = %v, want Wednesday", mustDate(t, "2026-10-07").Weekday())
	}
	raw, err := json.Marshal(struct {
		D Date `json:"d"`
	}{a})
	if err != nil || string(raw) != `{"d":"2026-10-07"}` {
		t.Errorf("json = %s, %v", raw, err)
	}
	if !(Date{}).IsZero() || a.IsZero() {
		t.Error("IsZero")
	}
}

// --- zones ---

func TestParseZone_AcceptsRealNamesAndRefusesTheRest(t *testing.T) {
	for _, name := range []string{
		"America/New_York", "Europe/London", "Asia/Kolkata", "Pacific/Apia", "UTC",
		"America/Argentina/Buenos_Aires", "Etc/GMT+5", "Etc/GMT-14", "America/Port-au-Prince",
	} {
		loc, err := ParseZone(name)
		if err != nil || loc == nil {
			t.Errorf("ParseZone(%q) = %v, %v; want a zone", name, loc, err)
		}
	}
	bad := []string{
		"",                         // empty
		"america/new_york",         // lower case: loads on a case-blind host, refused anyway
		"America/new_york",         // second part lower case
		"Local",                    // this server's own zone
		"../etc/passwd",            // path
		"../x",                     // path
		"America/../Asia/Kolkata",  // path
		"America//New_York",        // empty part
		"/America/New_York",        // leading slash
		"America/New_York/",        // trailing slash
		"America/New York",         // space
		"America/New_York\n",       // control
		"America/New_York\x00",     // NUL
		"America/Not_A_Real_Place", // well formed, not in the database
		"Europe/Londön",            // non-ASCII
		strings.Repeat("A", 65),    // too long
		"EST5EDT; drop table",      // junk
		"Etc/GMT%2B5",              // percent escape
	}
	for _, name := range bad {
		if loc, err := ParseZone(name); err == nil {
			t.Errorf("ParseZone(%q) = %v, want an error", name, loc)
		} else if !errors.Is(err, ErrZoneInvalid) {
			t.Errorf("ParseZone(%q) error = %v, want ErrZoneInvalid", name, err)
		}
	}
	// 64 characters is allowed by length (this one is not a real zone, so it
	// is refused by the database, not by the length rule).
	long := "A" + strings.Repeat("b", 63)
	if _, err := ParseZone(long); !errors.Is(err, ErrZoneInvalid) {
		t.Errorf("64 character unknown name: %v", err)
	}
}

func TestLoadStoredZone_ReportsAnUnreadableNameAsMissing(t *testing.T) {
	if _, err := loadStoredZone("America/New_York"); err != nil {
		t.Fatalf("loadStoredZone(good): %v", err)
	}
	for _, name := range []string{"", "Nowhere/Land", "america/new_york", "Local"} {
		if _, err := loadStoredZone(name); !errors.Is(err, ErrZoneMissing) {
			t.Errorf("loadStoredZone(%q) = %v, want ErrZoneMissing", name, err)
		}
	}
}

// --- weeks ---

func TestWeek_StartsOnMondayAndHoldsSevenDays(t *testing.T) {
	cases := []struct{ date, wantStart string }{
		{"2026-10-05", "2026-10-05"}, // a Monday
		{"2026-10-07", "2026-10-05"}, // Wednesday
		{"2026-10-11", "2026-10-05"}, // Sunday
		{"2026-10-12", "2026-10-12"}, // next Monday
		{"2026-03-08", "2026-03-02"}, // Sunday of a DST change
		{"2026-11-01", "2026-10-26"}, // Sunday of a DST change
		{"2026-01-01", "2025-12-29"}, // New Year, a Thursday
		{"2027-01-03", "2026-12-28"}, // the Sunday that closes 2026
		{"2028-02-29", "2028-02-28"}, // leap day, a Tuesday
		{"2024-02-29", "2024-02-26"}, // leap day, a Thursday
		{"2026-03-01", "2026-02-23"}, // the day after a non leap February
	}
	for _, c := range cases {
		got := WeekStart(mustDate(t, c.date))
		if got.String() != c.wantStart {
			t.Errorf("WeekStart(%s) = %s, want %s", c.date, got, c.wantStart)
		}
		if got.Weekday() != WeekStartDay {
			t.Errorf("WeekStart(%s) falls on %v, want %v", c.date, got.Weekday(), WeekStartDay)
		}
	}
}

func TestWeekDays_LettersNamesAndCrossings(t *testing.T) {
	days := WeekDays(mustDate(t, "2026-10-05"))
	if len(days) != 7 {
		t.Fatalf("len = %d, want 7", len(days))
	}
	letters, names := "", []string{}
	for _, d := range days {
		letters += d.Letter
		names = append(names, d.Weekday)
	}
	if letters != "MTWTFSS" {
		t.Errorf("letters = %q, want MTWTFSS", letters)
	}
	if got := strings.Join(names, ","); got != "Monday,Tuesday,Wednesday,Thursday,Friday,Saturday,Sunday" {
		t.Errorf("names = %s", got)
	}

	// A week that holds a leap day, and one that crosses New Year.
	leap := WeekDays(mustDate(t, "2028-02-28"))
	if leap[1].Date.String() != "2028-02-29" || leap[2].Date.String() != "2028-03-01" {
		t.Errorf("leap week dates: %v %v", leap[1].Date, leap[2].Date)
	}
	newYear := WeekDays(mustDate(t, "2026-12-28"))
	if newYear[3].Date.String() != "2026-12-31" || newYear[4].Date.String() != "2027-01-01" {
		t.Errorf("new year week dates: %v %v", newYear[3].Date, newYear[4].Date)
	}
	if DefaultWeeklyGoal != 4 {
		t.Errorf("DefaultWeeklyGoal = %d, want 4", DefaultWeeklyGoal)
	}
}
