// Package spin holds the rules of the daily spin: when a person's window
// opens and closes in their own time zone, how the wheel is validated and
// picked from, how a week and its bonus spin are counted, and the one pure
// function that turns all of that into the Today screen's data. The files
// that touch no database are pure and never read the clock; repository.go
// and service.go add storage and the clock on top.
package spin

import (
	"fmt"
	"time"
)

// Date is a calendar date with no time of day and no zone: the "local date"
// of a person's day. It is comparable with ==, sorts by Compare, and always
// writes itself as YYYY-MM-DD. The zero Date is not a real date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// Years outside this range are refused by ParseDate, so a typo cannot reach
// the database as a far-away date.
const (
	minDateYear = 1900
	maxDateYear = 2200
)

// DateOf returns the calendar date t shows in its own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate reads a strict YYYY-MM-DD date. It refuses other shapes, dates
// that do not exist (2026-02-30) and years outside 1900 to 2200.
func ParseDate(s string) (Date, error) {
	if len(s) != 10 {
		return Date{}, fmt.Errorf("spin: date %q is not YYYY-MM-DD", s)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("spin: date %q is not YYYY-MM-DD", s)
	}
	d := DateOf(t)
	if d.Year < minDateYear || d.Year > maxDateYear {
		return Date{}, fmt.Errorf("spin: date %q is out of range", s)
	}
	return d, nil
}

// String writes the date as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool { return d == Date{} }

// utc returns midnight UTC of the date. It is the carrier for civil
// arithmetic: no zone is involved, so no day is ever skipped or repeated.
func (d Date) utc() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// AddDays returns the date n calendar days later (earlier when n is
// negative).
func (d Date) AddDays(n int) Date {
	return DateOf(d.utc().AddDate(0, 0, n))
}

// Weekday returns the day of the week of the date.
func (d Date) Weekday() time.Weekday { return d.utc().Weekday() }

// Compare returns -1, 0 or 1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int {
	switch {
	case d == o:
		return 0
	case d.utc().Before(o.utc()):
		return -1
	default:
		return 1
	}
}

// Before reports whether d is earlier than o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is later than o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// DaysSince returns how many calendar days d is after o (negative when
// earlier).
func (d Date) DaysSince(o Date) int {
	return int(d.utc().Sub(o.utc()) / (24 * time.Hour))
}

// MarshalJSON writes the date as a JSON string, YYYY-MM-DD.
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}
