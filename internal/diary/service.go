package diary

import (
	"context"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// maxStreakWeeks bounds the goal weeks read for the streak.
const maxStreakWeeks = 520

// Service builds the diary page and the Profile summary from a Reader.
type Service struct {
	r       Reader
	clk     clock.Clock
	friends FriendCounter
}

// NewService builds a Service. A nil FriendCounter counts 0.
func NewService(r Reader, clk clock.Clock, f FriendCounter) *Service {
	if f == nil {
		f = ZeroFriends{}
	}
	return &Service{r: r, clk: clk, friends: f}
}

// PageParams are the already-validated query values of the diary.
type PageParams struct {
	CategoryID *string
	Cursor     string
	Limit      int
}

// Page returns one page. The cursor is decoded before any read.
func (s *Service) Page(ctx context.Context, userID string, p PageParams) (Page, error) {
	before, err := DecodeCursor(p.Cursor)
	if err != nil {
		return Page{}, err
	}
	limit := p.Limit
	if limit < 1 || limit > MaxLimit {
		return Page{}, ErrBadLimit
	}
	rows, err := s.r.Rows(ctx, userID, p.CategoryID, before, limit+1)
	if err != nil {
		return Page{}, err
	}
	out := Page{}
	if len(rows) > limit {
		rows = rows[:limit]
		out.NextCursor = EncodeCursor(rows[limit-1].LocalDate)
	}
	out.Rows = rows
	if out.Chips, err = s.r.Chips(ctx, userID); err != nil {
		return Page{}, err
	}
	return out, nil
}

// Summary returns the Profile numbers. A missing or bad zone gives a streak
// of 0, never an error.
func (s *Service) Summary(ctx context.Context, userID string) (Summary, error) {
	var out Summary
	var err error
	if out.Spins, err = s.r.SpinDays(ctx, userID); err != nil {
		return Summary{}, err
	}
	raw, err := s.r.Tastes(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	out.Tastes = make([]Taste, 0, len(raw))
	for _, t := range raw {
		out.Tastes = append(out.Tastes, Taste{Category: t.Chip, Tenths: AverageTenths(t.SumStars, t.Count), Count: t.Count})
	}
	if out.Recent, err = s.r.Recent(ctx, userID, RecentCount); err != nil {
		return Summary{}, err
	}
	if out.Friends, err = s.friends.Count(ctx, userID); err != nil {
		return Summary{}, err
	}
	zone, err := s.r.TimeZone(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	if zone != "" {
		if loc, zerr := spin.ParseZone(zone); zerr == nil {
			weeks, err := s.r.GoalWeekStarts(ctx, userID, maxStreakWeeks)
			if err != nil {
				return Summary{}, err
			}
			cur := spin.WeekStart(spin.LocalDate(s.clk.NowFor(userID), loc))
			out.WeekStreak = WeekStreak(cur.String(), weeks)
		}
	}
	return out, nil
}
