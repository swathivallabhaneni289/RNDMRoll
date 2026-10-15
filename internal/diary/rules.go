package diary

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Paging limits.
const (
	DefaultLimit = 20
	MaxLimit     = 50
)

var (
	// ErrBadCursor means the cursor is not one this server made.
	ErrBadCursor = errors.New("diary: invalid cursor")
	// ErrBadLimit means the limit is not an integer from 1 to 50.
	ErrBadLimit = errors.New("diary: invalid limit")
)

const cursorPrefix = "v1:"

// EncodeCursor is the unpadded base64url of "v1:" plus the date.
func EncodeCursor(date string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursorPrefix + date))
}

// DecodeCursor returns the date inside a cursor. An empty cursor is the
// first page and gives "" and no error.
func DecodeCursor(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > 64 {
		return "", ErrBadCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", ErrBadCursor
	}
	text := string(raw)
	if !strings.HasPrefix(text, cursorPrefix) {
		return "", ErrBadCursor
	}
	date := strings.TrimPrefix(text, cursorPrefix)
	if len(date) != 10 {
		return "", ErrBadCursor
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", ErrBadCursor
	}
	return date, nil
}

// ParseLimit gives DefaultLimit for "" and ErrBadLimit for anything that is
// not plain digits from 1 to 50.
func ParseLimit(s string) (int, error) {
	if s == "" {
		return DefaultLimit, nil
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrBadLimit
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > MaxLimit {
		return 0, ErrBadLimit
	}
	return n, nil
}

// AverageTenths is the mean of the stored stars divided by 2, in tenths,
// half rounding up: (10*sum + count) / (2*count). A count of 0 gives 0.
func AverageTenths(sum, count int) int {
	if count <= 0 {
		return 0
	}
	return (10*sum + count) / (2 * count)
}

// WeekStreak counts consecutive weeks (seven days apart) present in
// weekStarts, ending with the current week when it is there, else with the
// week before it. Unparseable strings are ignored.
func WeekStreak(currentWeekStart string, weekStarts []string) int {
	set := make(map[string]bool, len(weekStarts))
	for _, s := range weekStarts {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			set[t.Format("2006-01-02")] = true
		}
	}
	cur, err := time.Parse("2006-01-02", currentWeekStart)
	if err != nil {
		return 0
	}
	if !set[cur.Format("2006-01-02")] {
		cur = cur.AddDate(0, 0, -7)
	}
	n := 0
	for set[cur.Format("2006-01-02")] {
		n++
		cur = cur.AddDate(0, 0, -7)
	}
	return n
}
