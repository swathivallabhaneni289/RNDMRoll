package spin

import (
	"context"
	"time"
)

// Board actions: what the locked Board card offers.
const (
	// ActionLog: a spin exists and can still be posted.
	ActionLog = "log"
	// ActionSpin: the window is open and no spin was made yet.
	ActionSpin = "spin"
	// ActionWait: nothing can be posted now.
	ActionWait = "wait"
)

// DayInPlay returns the Board's "today": the date of the most recent 8:00 PM
// local that has passed. From 8:00 PM until the next 8:00 PM it is one date.
// lateWindow is true before 8:00 PM, when the day in play is the evening
// before. It never adds 24 hours to an instant.
func DayInPlay(now time.Time, loc *time.Location) (date Date, lateWindow bool) {
	d := LocalDate(now, loc)
	if !now.Before(OpensAt(d, loc)) {
		return d, false
	}
	return PrevDate(d, loc), true
}

// BoardFacts is what the Board learns about the viewer's own spins, read
// under the viewer's lock with one now.
type BoardFacts struct {
	Now  time.Time
	Zone *time.Location
	// DayInPlay is the Board's "today"; LateWindow is true before 8:00 PM.
	DayInPlay  Date
	LateWindow bool
	// Posted: a posted day (any source) exists for DayInPlay.
	Posted bool
	// Skipped: not Posted and DayInPlay's latest spin was skipped.
	Skipped bool
	// Action is ActionLog, ActionSpin or ActionWait, decided by the same
	// Today rules the Spin page uses. CategoryName is the shown spin's
	// category as written when Action is ActionLog, else nil. PostLate is
	// Today's PostLate when Action is ActionLog (a post made now counts
	// late, from noon of the date after the spin's), else false.
	Action       string
	CategoryName *string
	PostLate     bool
}

// BoardFacts reads the viewer's Board facts. Before 8:00 PM the Board's day
// is the evening before, unless today's own date already has a spin (a first
// spin, posted or not) or a posted day, or the viewer has never spun: then it
// is today. A person with no readable stored zone gets ErrZoneMissing. A
// first call makes the state row, as Today does.
func (s *Service) BoardFacts(ctx context.Context, userID string) (BoardFacts, error) {
	var out BoardFacts
	err := s.run(ctx, userID, true, func(c *call) error {
		today, err := s.todayIn(c)
		if err != nil {
			return err
		}
		day, late := DayInPlay(c.now, c.loc)
		// A first spin (made before 8:00 PM, developer 2026-10-19) is for
		// today's own date: once it is made, posted or not, the Board is on
		// today, as the Spin page's week row is (BuildToday). So the new
		// person sees their post and their friends' at once, is never shown
		// last night's posts that no post of theirs could open, and a skip
		// shows as skipped. Before 8:00 PM only a first spin, or one made
		// further east before a move west, is on today's date. A pretend
		// posted day for today (the test tools) counts too. A person who has
		// never spun is on today as well: their first spin is for today's
		// date, so the Board never says "last night" to them.
		if late {
			own := LocalDate(c.now, c.loc)
			early, err := c.tx.PostedDay(own)
			if err != nil {
				return err
			}
			spun, err := c.tx.LatestSpinOfDate(own)
			if err != nil {
				return err
			}
			anySpin, err := c.tx.HasAnySpin()
			if err != nil {
				return err
			}
			if early != nil || spun != nil || !anySpin {
				day, late = own, false
			}
		}
		posted, err := c.tx.PostedDay(day)
		if err != nil {
			return err
		}
		latest, err := c.tx.LatestSpinOfDate(day)
		if err != nil {
			return err
		}
		out = BoardFacts{
			Now:        c.now,
			Zone:       c.loc,
			DayInPlay:  day,
			LateWindow: late,
			Posted:     posted != nil,
			Action:     ActionWait,
		}
		out.Skipped = !out.Posted && latest != nil && latest.Result == ResultSkipped
		switch today.State {
		case StateOpen:
			out.Action = ActionSpin
		case StateResult, StateLate:
			if today.Spin != nil {
				out.Action = ActionLog
				name := today.Spin.Category.Name
				out.CategoryName = &name
				out.PostLate = today.PostLate
			}
		}
		return nil
	})
	return out, err
}

// ViewerClock returns the viewer's now (from the spin slice's clock) and
// stored zone, without taking a lock. ErrZoneMissing when no readable zone
// is stored.
func (s *Service) ViewerClock(ctx context.Context, userID string) (time.Time, *time.Location, error) {
	loc, _, err := s.ZoneOf(ctx, userID)
	if err != nil {
		return time.Time{}, nil, err
	}
	return s.clock.NowFor(userID), loc, nil
}

// SpinOpenedTonight says whether the wheel opened tonight for the viewer and
// when (openedTonight), for the bell's "Your spin is open.": never on an
// evening whose spin was already made before 8:00 PM, and from the end of
// the 12-hour gap when a zone change holds the wheel shut past 8:00 PM. It
// reads the date's daily spin and the last daily spin under the viewer's
// lock with one now. A person with no readable stored zone gets
// ErrZoneMissing. A first call makes the state row, as Today does.
func (s *Service) SpinOpenedTonight(ctx context.Context, userID string) (at time.Time, ok bool, err error) {
	err = s.run(ctx, userID, true, func(c *call) error {
		daily, err := c.tx.DailySpinOfDate(c.today)
		if err != nil {
			return err
		}
		last, err := c.tx.LastDailySpunAt()
		if err != nil {
			return err
		}
		at, ok = openedTonight(c.now, c.loc, daily, last)
		return nil
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return at, ok, nil
}
