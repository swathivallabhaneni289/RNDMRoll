package spin

import "time"

const (
	// OpenHour is the local hour (24-hour clock) the day's spin opens. It is
	// a product constant: 8:00 PM in the person's own zone.
	OpenHour = 20

	// MinSpinGap is the least time between two daily spins, so a zone change
	// cannot buy a second spin inside 12 hours.
	MinSpinGap = 12 * time.Hour
)

// Window is one local date's spin window in one zone.
//
//   - OpensAt is 8:00 PM on the date.
//   - ClosesAt is the first instant whose local date is later than the date
//     (local midnight, or the first instant after it when midnight is
//     skipped).
//   - LateUntil is the next opening after ClosesAt. A post made from ClosesAt
//     up to LateUntil is late but still counts.
type Window struct {
	OpensAt   time.Time
	ClosesAt  time.Time
	LateUntil time.Time
}

// LocalDate returns the calendar date t shows in loc.
func LocalDate(t time.Time, loc *time.Location) Date {
	return DateOf(t.In(loc))
}

// firstAtOrAfter returns the first instant at which the wall clock in loc
// reads civil or later. civil is a date and time of day carried as a UTC
// time.Time (a label, not an instant).
//
// time.Date does not promise which instant it gives when a local time is
// skipped (a spring-forward gap) or happens twice (a fall-back overlap), and
// it can answer with the wrong date. This walks zone periods instead, so the
// answer is exact:
//
//   - a time that happens twice gives its first occurrence;
//   - a time that is skipped gives the first instant after the gap.
//
// Within one zone period the offset is constant, so the reading reaches civil
// at civil minus the offset. If that falls before the period's start, the
// target was jumped over and the period's start is the answer. If it falls
// at or after the period's end, the target was not reached in this period
// and the walk moves to the next one.
func firstAtOrAfter(civil time.Time, loc *time.Location) time.Time {
	// No zone is more than 16 hours from UTC, so 30 hours before civil is
	// surely an instant whose reading is still earlier than civil.
	t := civil.Add(-30 * time.Hour).In(loc)
	for i := 0; i < 16; i++ {
		start, end := t.ZoneBounds()
		_, offset := t.Zone()
		candidate := civil.Add(-time.Duration(offset) * time.Second)
		if !start.IsZero() && candidate.Before(start) {
			candidate = start
		}
		if end.IsZero() || candidate.Before(end) {
			return candidate.In(loc)
		}
		t = end
	}
	// Not reachable with a real zone (a day holds a handful of zone
	// periods at most). Fall back to the standard library's own answer.
	return time.Date(civil.Year(), civil.Month(), civil.Day(), civil.Hour(), civil.Minute(), civil.Second(), 0, loc)
}

// DayStart returns the first instant whose local date in loc is d. If d does
// not exist in loc (Samoa skipped 2011-12-30 entirely) it returns the first
// instant of the next date that does.
func DayStart(d Date, loc *time.Location) time.Time {
	return firstAtOrAfter(d.utc(), loc)
}

// OpensAt returns the instant the spin opens on date d in loc: 8:00 PM.
func OpensAt(d Date, loc *time.Location) time.Time {
	return firstAtOrAfter(d.utc().Add(OpenHour*time.Hour), loc)
}

// WindowFor returns the spin window of date d in loc.
func WindowFor(d Date, loc *time.Location) Window {
	opens := OpensAt(d, loc)
	closes := DayStart(d.AddDays(1), loc)
	// The close is where the next local date begins, which is not always
	// d plus one (after a skipped day it is the date after that), so the
	// late window is measured to that date's own opening.
	lateUntil := OpensAt(LocalDate(closes, loc), loc)
	return Window{OpensAt: opens, ClosesAt: closes, LateUntil: lateUntil}
}

// PrevDate returns the local date of the instant just before d begins in
// loc. It is "yesterday" even where the day before d was cut short or
// skipped.
func PrevDate(d Date, loc *time.Location) Date {
	return LocalDate(DayStart(d, loc).Add(-time.Nanosecond), loc)
}

// NextOpen returns the first opening strictly after t in loc: today's 8:00 PM
// if it is still ahead, else the opening of the date that begins when this
// one closes. It never adds 24 hours or a day to an instant.
func NextOpen(t time.Time, loc *time.Location) time.Time {
	d := LocalDate(t, loc)
	if opens := OpensAt(d, loc); opens.After(t) {
		return opens
	}
	closes := DayStart(d.AddDays(1), loc)
	return OpensAt(LocalDate(closes, loc), loc)
}

// InstantOfLocal returns the instant at which the wall clock in loc reads
// civil exactly (civil is a date and time of day carried as a UTC time.Time).
// A reading that happens twice gives its first instant. ok is false when the
// reading never happens (it falls in a spring-forward gap).
func InstantOfLocal(civil time.Time, loc *time.Location) (instant time.Time, ok bool) {
	instant = firstAtOrAfter(civil, loc)
	l := instant.In(loc)
	same := l.Year() == civil.Year() && l.Month() == civil.Month() && l.Day() == civil.Day() &&
		l.Hour() == civil.Hour() && l.Minute() == civil.Minute() && l.Second() == civil.Second()
	return instant, same
}
