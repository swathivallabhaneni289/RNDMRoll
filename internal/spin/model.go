package spin

import (
	"time"
)

// SpinKind is "daily" for the day's own spin and "bonus" for the one extra
// spin a held bonus buys.
type SpinKind string

const (
	KindDaily SpinKind = "daily"
	KindBonus SpinKind = "bonus"
)

// SpinResult is what became of a spin's day. Phase 2 only ever stores
// "open"; the entry phase sets "posted" and "skipped" on the day's latest
// spin.
type SpinResult string

const (
	ResultOpen    SpinResult = "open"
	ResultPosted  SpinResult = "posted"
	ResultSkipped SpinResult = "skipped"
)

// PostedSource says where a posted day came from: "post" is a real entry
// (the entry phase), "test" is a pretend day made by the test tools. A
// "test" row hides a real post for that date.
type PostedSource string

const (
	SourcePost PostedSource = "post"
	SourceTest PostedSource = "test"
)

// TodayState is the state of the Today screen. The server decides it.
type TodayState string

const (
	// StateWaiting: before 8:00 PM, or the day was missed, or a daily spin
	// under 12 hours old keeps the wheel shut.
	StateWaiting TodayState = "waiting"
	// StateOpen: 8:00 PM or later, no spin yet today.
	StateOpen TodayState = "open"
	// StateResult: a spin was made today.
	StateResult TodayState = "result"
	// StateLate: after midnight, before the next opening, and yesterday's
	// spin is still unsettled.
	StateLate TodayState = "late"
)

// WheelEntry is one wedge of a wheel: a category id, the name as written and
// the weight. A spin stores its whole wheel as a list of these.
type WheelEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Category is one row of a person's wheel.
type Category struct {
	ID         string
	Name       string
	Weight     int
	Position   int
	ArchivedAt *time.Time
}

// Spin is a stored spin. Wheel is the wheel used, in order, and WinnerIndex
// points into it. CategoryName is the name as it was written at the time, so
// a later rename changes nothing here.
type Spin struct {
	ID             string
	LocalDate      Date
	TZName         string
	Kind           SpinKind
	Seq            int
	CategoryID     string
	CategoryName   string
	Wheel          []WheelEntry
	WinnerIndex    int
	SpunAt         time.Time
	WindowOpensAt  time.Time
	WindowClosesAt time.Time
	Result         SpinResult
	CreatedAt      time.Time
}

// UserState is a person's one state row. It is the lock for everything the
// person does.
type UserState struct {
	// SeededAt, SeededLocalDate and SeededTZ say when the row was first made
	// (from the service clock) so the entry phase can treat the first partial
	// week differently for a streak.
	SeededAt        time.Time
	SeededLocalDate Date
	SeededTZ        string
	WeeklyGoal      int
	// BonusHeld is 0 or 1: at most one bonus spin is held.
	BonusHeld    int
	WheelVersion int
}

// PostedDay is a date counted as posted.
type PostedDay struct {
	Date   Date
	Source PostedSource
	Late   bool
}

// GoalWeek records that a week reached its goal. A week that has a row
// never pays a second bonus, even after the held one is spent.
type GoalWeek struct {
	WeekStart    Date
	Goal         int
	MetAt        time.Time
	BonusGranted bool
}

// --- the shape sent to the phone ---

// Instant is a moment written as RFC 3339 in UTC with millisecond
// precision, for example "2026-10-07T20:30:00.000Z".
type Instant time.Time

// MarshalJSON writes the instant as an RFC 3339 UTC string.
func (i Instant) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(i).UTC().Format("2006-01-02T15:04:05.000Z") + `"`), nil
}

// WindowView is the current local date's spin window.
type WindowView struct {
	OpensAt  Instant `json:"opens_at"`
	ClosesAt Instant `json:"closes_at"`
}

// CategoryRef names the category a spin landed on.
type CategoryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SpinView is a spin as the phone sees it. SpunLocalTime is written by the
// server in the zone the spin was made in ("8:02 PM"); the phone prints it
// as sent.
type SpinView struct {
	ID            string       `json:"id"`
	LocalDate     Date         `json:"local_date"`
	Kind          SpinKind     `json:"kind"`
	Seq           int          `json:"seq"`
	Category      CategoryRef  `json:"category"`
	WinnerIndex   int          `json:"winner_index"`
	Wheel         []WheelEntry `json:"wheel"`
	SpunAt        Instant      `json:"spun_at"`
	SpunLocalTime string       `json:"spun_local_time"`
	Result        SpinResult   `json:"result"`
}

// WeekDayView is one day of the week section.
type WeekDayView struct {
	Date    Date   `json:"date"`
	Letter  string `json:"letter"`
	Weekday string `json:"weekday"`
	Posted  bool   `json:"posted"`
}

// WeekView is the week of the current local date, Monday first. TodayIndex
// is the position of the current local date in Days, so the phone does no
// date arithmetic.
type WeekView struct {
	Start      Date          `json:"start"`
	Goal       int           `json:"goal"`
	Posted     int           `json:"posted"`
	GoalMet    bool          `json:"goal_met"`
	TodayIndex int           `json:"today_index"`
	Days       []WeekDayView `json:"days"`
}

// BonusView says whether a bonus spin is held and whether the person may use
// it now.
type BonusView struct {
	Held   bool `json:"held"`
	CanUse bool `json:"can_use"`
}

// Today is everything the Today screen shows. NextOpenWeekday is written by
// the server in the person's current zone ("Friday").
type Today struct {
	Now             Instant      `json:"now"`
	TimeZone        string       `json:"time_zone"`
	LocalDate       Date         `json:"local_date"`
	State           TodayState   `json:"state"`
	Window          WindowView   `json:"window"`
	NextOpenAt      Instant      `json:"next_open_at"`
	NextOpenWeekday string       `json:"next_open_weekday"`
	Spin            *SpinView    `json:"spin"`
	Wheel           []WheelEntry `json:"wheel"`
	Week            WeekView     `json:"week"`
	Bonus           BonusView    `json:"bonus"`
}

// TodayInput is what BuildToday needs. The service reads all of it under the
// person's lock.
type TodayInput struct {
	// Now is the instant read once for the request; Loc is the person's
	// current zone and TZName its name.
	Now    time.Time
	Loc    *time.Location
	TZName string

	// Spin is the latest spin (a bonus counts) of the current local date,
	// or nil. YesterdaySpin is the latest spin of the date before it
	// (PrevDate), or nil. Their Result is already what the screen shows:
	// "posted" when a posted day exists for that date, else the stored
	// value.
	Spin          *Spin
	YesterdaySpin *Spin

	// Settled is whether the current local date is settled (daySettled).
	Settled bool

	// LastDailySpunAt is when the person's latest daily spin was made, in
	// any zone, or nil.
	LastDailySpunAt *time.Time

	// Wheel is the live wheel, shown when no spin is shown.
	Wheel []WheelEntry

	BonusHeld bool
	Goal      int

	// PostedDates are the posted days that fall in the current week.
	PostedDates []Date
}

// gapAllowsSpin reports whether enough time has passed since the last daily
// spin for another one: at least MinSpinGap.
func gapAllowsSpin(now time.Time, lastDailySpunAt *time.Time) bool {
	return lastDailySpunAt == nil || now.Sub(*lastDailySpunAt) >= MinSpinGap
}

// BuildToday turns the facts into the Today screen's data. It is pure: no
// database, no clock, no randomness.
//
// The state is the first of these that holds:
//
//  1. a daily spin was made on the local date: result;
//  2. it is 8:00 PM or later and no daily spin was made in the last 12
//     hours: open;
//  3. yesterday's spin is unsettled and the late window has not ended
//     (whatever zone made it): late;
//  4. waiting.
//
// next_open_at is the first opening strictly after now and at or after the
// last daily spin plus 12 hours.
func BuildToday(in TodayInput) Today {
	loc := in.Loc
	today := LocalDate(in.Now, loc)
	win := WindowFor(today, loc)
	prev := PrevDate(today, loc)

	state := StateWaiting
	var shown *Spin
	switch {
	case in.Spin != nil:
		state = StateResult
		shown = in.Spin
	case !in.Now.Before(win.OpensAt) && gapAllowsSpin(in.Now, in.LastDailySpunAt):
		state = StateOpen
	case in.YesterdaySpin != nil && in.YesterdaySpin.Result == ResultOpen &&
		in.Now.Before(WindowFor(prev, loc).LateUntil):
		state = StateLate
		shown = in.YesterdaySpin
	}

	next := NextOpen(in.Now, loc)
	if in.LastDailySpunAt != nil {
		if earliest := in.LastDailySpunAt.Add(MinSpinGap); next.Before(earliest) {
			next = NextOpen(earliest.Add(-time.Nanosecond), loc)
		}
	}

	wheel := in.Wheel
	var spinView *SpinView
	if shown != nil {
		wheel = shown.Wheel
		spinView = viewOfSpin(shown, loc)
	}
	if wheel == nil {
		wheel = []WheelEntry{}
	}

	posted := make(map[Date]bool, len(in.PostedDates))
	for _, d := range in.PostedDates {
		posted[d] = true
	}
	weekStart := WeekStart(today)
	days := WeekDays(weekStart)
	dayViews := make([]WeekDayView, len(days))
	count := 0
	for i, d := range days {
		dayViews[i] = WeekDayView{Date: d.Date, Letter: d.Letter, Weekday: d.Weekday, Posted: posted[d.Date]}
		if posted[d.Date] {
			count++
		}
	}

	return Today{
		Now:             Instant(in.Now),
		TimeZone:        in.TZName,
		LocalDate:       today,
		State:           state,
		Window:          WindowView{OpensAt: Instant(win.OpensAt), ClosesAt: Instant(win.ClosesAt)},
		NextOpenAt:      Instant(next),
		NextOpenWeekday: weekdayName(next, loc),
		Spin:            spinView,
		Wheel:           wheel,
		Week: WeekView{
			Start:      weekStart,
			Goal:       in.Goal,
			Posted:     count,
			GoalMet:    count >= in.Goal,
			TodayIndex: today.DaysSince(weekStart),
			Days:       dayViews,
		},
		Bonus: BonusView{
			Held: in.BonusHeld,
			// One bonus spin per date: the database allows one, so a day
			// that already has its bonus offers no second.
			CanUse: state == StateResult && !in.Settled && in.BonusHeld && shown != nil && shown.Kind == KindDaily,
		},
	}
}

// viewOfSpin builds the phone's view of a spin. The time it was made is
// written in the spin's own zone; when that name no longer loads, the
// person's current zone is used.
func viewOfSpin(s *Spin, current *time.Location) *SpinView {
	own := current
	if l, err := ParseZone(s.TZName); err == nil {
		own = l
	}
	wheel := s.Wheel
	if wheel == nil {
		wheel = []WheelEntry{}
	}
	return &SpinView{
		ID:            s.ID,
		LocalDate:     s.LocalDate,
		Kind:          s.Kind,
		Seq:           s.Seq,
		Category:      CategoryRef{ID: s.CategoryID, Name: s.CategoryName},
		WinnerIndex:   s.WinnerIndex,
		Wheel:         wheel,
		SpunAt:        Instant(s.SpunAt),
		SpunLocalTime: clockLabel(s.SpunAt, own),
		Result:        s.Result,
	}
}

// clockLabel writes t in loc as 12-hour time with no leading zero ("8:02 PM",
// "12:05 AM"). It and weekdayName are the only places a time or a weekday is
// written for the screen.
func clockLabel(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("3:04 PM")
}

// weekdayName writes the weekday t falls on in loc ("Friday").
func weekdayName(t time.Time, loc *time.Location) string {
	return t.In(loc).Weekday().String()
}
