package spin

import "time"

// The time labels of the Board, the post page and the notification list. The
// server writes every one; the phone formats no time. Pure: no clock.

// ClockOf writes t in loc as the time of day, 12-hour, no leading zero
// ("8:41 PM", "12:05 AM").
func ClockOf(t time.Time, loc *time.Location) string {
	return clockLabel(t, loc)
}

// CivilDaysBetween returns the whole days from one date to another by plain
// calendar arithmetic (to minus from; negative when from is later).
func CivilDaysBetween(from, to Date) int {
	return to.DaysSince(from)
}

// PostedLocalTime is a card's time of day in the AUTHOR's own zone: the zone
// the entry's spin was made in. A zone name that does not load falls back to
// the viewer's zone and never fails the page.
func PostedLocalTime(at time.Time, zoneName string, fallback *time.Location) string {
	loc := fallback
	if l, err := ParseZone(zoneName); err == nil {
		loc = l
	}
	return ClockOf(at, loc)
}

// ViewerLabel is the one rule for a comment's and a notification's label, in
// the VIEWER's zone: the time of day for an instant on the viewer's current
// local day (an instant after it counts as today), the short weekday ("Mon")
// for the six local days before, the month and day ("Oct 3") before that.
func ViewerLabel(at, now time.Time, loc *time.Location) string {
	n := DaysBefore(at, now, loc)
	switch {
	case n <= 0:
		return ClockOf(at, loc)
	case n <= 6:
		return at.In(loc).Format("Mon")
	default:
		return at.In(loc).Format("Jan 2")
	}
}

// DaysBefore is how many local calendar days at lies before now in loc (0 for
// the same day, negative for a later day). The labels and the notification
// sections share it, so a label and a section never disagree.
func DaysBefore(at, now time.Time, loc *time.Location) int {
	return CivilDaysBetween(LocalDate(at, loc), LocalDate(now, loc))
}
