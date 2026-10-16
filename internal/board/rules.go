package board

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// CleanComment trims the text and turns \r\n into \n. It must be 1 to 140
// characters with no control character except a line break. Reasons on
// field text: required, too_long, invalid.
func CleanComment(s string) (string, *FieldProblem) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", &FieldProblem{Field: "text", Reason: "required"}
	}
	if utf8.RuneCountInString(s) > MaxCommentRunes {
		return "", &FieldProblem{Field: "text", Reason: "too_long"}
	}
	for _, r := range s {
		if r == '\n' {
			continue
		}
		if unicode.IsControl(r) || r == utf8.RuneError {
			return "", &FieldProblem{Field: "text", Reason: "invalid"}
		}
	}
	return s, nil
}

// Decide is the Board's state, checked in order: following nobody, posted
// the day in play, skipped it, else locked.
func Decide(followsAnyone, posted, skipped bool) string {
	switch {
	case !followsAnyone:
		return StateNoFollows
	case posted:
		return StateUnlocked
	case skipped:
		return StateSkipped
	default:
		return StateLocked
	}
}

// DayLabel names a listed date against the day in play: today, yesterday
// (the calendar date before it), else earlier.
func DayLabel(date, dayInPlay spin.Date) string {
	switch spin.CivilDaysBetween(date, dayInPlay) {
	case 0:
		return LabelToday
	case 1:
		return LabelYesterday
	default:
		return LabelEarlier
	}
}

// FirstName is the first word of a name, at most 20 characters.
func FirstName(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	r := []rune(fields[0])
	if len(r) > 20 {
		r = r[:20]
	}
	return string(r)
}
