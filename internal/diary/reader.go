package diary

import (
	"context"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// RecentCount is how many entries the summary lists.
const RecentCount = 3

// Reader is the one read seam. Every method is scoped to one person and none
// writes or locks. Archived entries are never returned by Rows, Chips,
// Tastes or Recent; SpinDays and GoalWeekStarts still count their days.
type Reader interface {
	// Rows returns up to limit rows, newest first, with local_date strictly
	// before `before` when it is not empty. Skipped days are included only when
	// categoryID is nil.
	Rows(ctx context.Context, userID string, categoryID *string, before string, limit int) ([]Row, error)
	Chips(ctx context.Context, userID string) ([]Chip, error)
	SpinDays(ctx context.Context, userID string) (int, error)
	// GoalWeekStarts returns week_start dates (YYYY-MM-DD), newest first.
	GoalWeekStarts(ctx context.Context, userID string, max int) ([]string, error)
	Tastes(ctx context.Context, userID string) ([]RawTaste, error)
	Recent(ctx context.Context, userID string, n int) ([]spin.Entry, error)
	// TimeZone is the stored zone name, or "" when none.
	TimeZone(ctx context.Context, userID string) (string, error)
}

// FollowerCounter counts accepted followers and accepted follows the person
// has (social.Service).
type FollowerCounter interface {
	CountFollowers(ctx context.Context, userID string) (int, error)
	CountFollowing(ctx context.Context, userID string) (int, error)
}

// ZeroFollowers always counts 0 (a server built without follows).
type ZeroFollowers struct{}

// CountFollowers implements FollowerCounter.
func (ZeroFollowers) CountFollowers(context.Context, string) (int, error) { return 0, nil }

// CountFollowing implements FollowerCounter.
func (ZeroFollowers) CountFollowing(context.Context, string) (int, error) { return 0, nil }
