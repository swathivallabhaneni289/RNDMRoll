package spin

import "time"

const (
	// WeekStartDay is the day a counted week begins on. A week is Monday to
	// Sunday in the person's own zone. This constant and DefaultWeeklyGoal
	// are the only places those two numbers are written.
	WeekStartDay = time.Monday

	// DefaultWeeklyGoal is the number of posted days in a week that earns a
	// bonus spin. The database column has no default; the starting value is
	// written here.
	DefaultWeeklyGoal = 4

	daysInWeek = 7
)

// WeekDay is one day of a week as the Today screen shows it.
type WeekDay struct {
	Date    Date
	Letter  string
	Weekday string
}

// WeekStart returns the first day of the week that holds d. It works on
// calendar dates only, so no zone or clock change can move it.
func WeekStart(d Date) Date {
	back := (int(d.Weekday()) - int(WeekStartDay) + daysInWeek) % daysInWeek
	return d.AddDays(-back)
}

// WeekDays returns the seven days of the week that begins on start, first
// day first, each with its one-letter name and its weekday name. The names
// come from the one helper that writes a weekday for the screen.
func WeekDays(start Date) []WeekDay {
	days := make([]WeekDay, daysInWeek)
	for i := range days {
		d := start.AddDays(i)
		name := weekdayName(d.utc(), time.UTC)
		days[i] = WeekDay{Date: d, Letter: name[:1], Weekday: name}
	}
	return days
}
