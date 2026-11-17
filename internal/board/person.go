package board

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// DefaultPersonPosts and MaxPersonPosts bound one page of a person's grid.
const (
	DefaultPersonPosts = 21
	MaxPersonPosts     = 60
)

// ErrBadCursor: a person page cursor this server did not make.
var ErrBadCursor = errors.New("board: invalid cursor")

// personCursorPrefix sets a person page cursor apart from the diary's
// ("v1:"), so neither list takes the other's.
const personCursorPrefix = "person1:"

// EncodePersonCursor is the next_cursor of a person's grid: the unpadded
// base64url of personCursorPrefix and the date of the page's last tile. The
// date alone places a tile, since a person has at most one entry a day
// (entries_one_per_day), and it still places it when that entry is deleted
// before the next page is read.
func EncodePersonCursor(date string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(personCursorPrefix + date))
}

// DecodePersonCursor returns the date inside a cursor. Anything
// EncodePersonCursor did not make, an empty cursor included, is
// ErrBadCursor.
func DecodePersonCursor(s string) (string, error) {
	if s == "" || len(s) > 64 {
		return "", ErrBadCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", ErrBadCursor
	}
	date, ok := strings.CutPrefix(string(raw), personCursorPrefix)
	if !ok || len(date) != 10 {
		return "", ErrBadCursor
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", ErrBadCursor
	}
	return date, nil
}

// PersonPost is one tile of another person's grid.
type PersonPost struct {
	EntryID      uuid.UUID
	LocalDate    string
	CategoryName string
	Title        string
	PhotoURL     string
	PhotoCount   int
}

// PersonPosts is one page of owner's grid as the viewer sees it, newest
// first: the same rule as the Board (an accepted follow, no block either
// way, not archived, only days the viewer posted too, and nothing removed,
// under review or reported by the viewer). before is the
// date of the last tile of the page before (DecodePersonCursor). next, the
// date of this page's last tile, is set when more may follow.
func (s *Service) PersonPosts(ctx context.Context, viewer, owner uuid.UUID, before *string, limit int) ([]PersonPost, *string, error) {
	rows, err := s.repo.PersonEntries(ctx, viewer, owner, before, limit+1)
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1].LocalDate
		next = &last
	}
	// Every listed row passed the rule in SQL.
	f := social.Facts{Follows: true, ViewerPostedDay: true}
	out := make([]PersonPost, 0, len(rows))
	for _, r := range rows {
		p := PersonPost{
			EntryID:      r.ID,
			LocalDate:    r.LocalDate,
			CategoryName: r.CategoryName,
			Title:        r.Title,
			PhotoCount:   len(r.Photos),
		}
		if len(r.Photos) > 0 {
			p.PhotoURL = s.link(f, r.Photos[0].Key)
		}
		out = append(out, p)
	}
	return out, next, nil
}

// PersonPostCount is the Posts figure of a person's page: their posts,
// archived ones left out. Anyone who can open the page sees it. It also
// leaves out what moderation keeps from the viewer (removed, under review,
// reported by them), as the grid does. It still counts posts on days the
// viewer did not post, which the grid does not show.
func (s *Service) PersonPostCount(ctx context.Context, viewer, owner uuid.UUID) (int, error) {
	return s.repo.PostCount(ctx, viewer, owner)
}

// SetCommentHidden hides or shows a comment on the viewer's own post. Only
// the post's owner may, and never on their own comment (they delete it
// instead); anything else is ErrNotFound, the same as a missing comment.
func (s *Service) SetCommentHidden(ctx context.Context, viewer, commentID uuid.UUID, hidden bool) error {
	c, err := s.repo.CommentByID(ctx, commentID)
	if err != nil {
		return err
	}
	if c.EntryOwner != viewer || c.Author.ID == viewer {
		return ErrNotFound
	}
	var at *time.Time
	if hidden {
		now := s.clk.NowFor(viewer.String()).UTC()
		at = &now
	}
	return s.repo.SetCommentHidden(ctx, commentID, at)
}
