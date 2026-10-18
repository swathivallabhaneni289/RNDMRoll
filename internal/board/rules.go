package board

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// CleanComment trims the text and turns \r\n into \n. It must be 1 to 140
// characters with no control character except a line break. Reasons on
// field text: required, too_long, invalid.
func CleanComment(s string) (string, *FieldProblem) {
	return cleanComment(s, false)
}

// CleanCommentWithPhoto is CleanComment for a comment that carries a photo:
// the text may then be empty (like Instagram), never longer than 140.
func CleanCommentWithPhoto(s string) (string, *FieldProblem) {
	return cleanComment(s, true)
}

func cleanComment(s string, photo bool) (string, *FieldProblem) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" && photo {
		return "", nil
	}
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

// Thread orders the comments the viewer may see into one flat list: the
// top-level comments in the order given (oldest first), each followed
// directly by its replies in the order given. A reply whose top-level
// comment is not in rows (its writer has a block with the viewer) is left
// out, so a reply is never drawn under the wrong comment.
func Thread(rows []CommentRow) []CommentRow {
	tops := make(map[uuid.UUID]bool, len(rows))
	replies := map[uuid.UUID][]CommentRow{}
	for _, c := range rows {
		if c.ParentID == nil {
			tops[c.ID] = true
		} else {
			replies[*c.ParentID] = append(replies[*c.ParentID], c)
		}
	}
	out := make([]CommentRow, 0, len(rows))
	for _, c := range rows {
		if c.ParentID != nil || !tops[c.ID] {
			continue
		}
		out = append(out, c)
		out = append(out, replies[c.ID]...)
	}
	return out
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
