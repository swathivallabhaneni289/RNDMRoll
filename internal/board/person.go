package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// DefaultPersonPosts and MaxPersonPosts bound one page of a person's grid.
const (
	DefaultPersonPosts = 21
	MaxPersonPosts     = 60
)

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
// way, not archived, and only days the viewer posted too). after is the last
// entry of the page before. next is set when more may follow.
func (s *Service) PersonPosts(ctx context.Context, viewer, owner uuid.UUID, after *uuid.UUID, limit int) ([]PersonPost, *uuid.UUID, error) {
	rows, err := s.repo.PersonEntries(ctx, viewer, owner, after, limit+1)
	if err != nil {
		return nil, nil, err
	}
	var next *uuid.UUID
	if len(rows) > limit {
		rows = rows[:limit]
		id := rows[limit-1].ID
		next = &id
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
// archived ones left out. Anyone who can open the page sees it.
func (s *Service) PersonPostCount(ctx context.Context, owner uuid.UUID) (int, error) {
	return s.repo.PostCount(ctx, owner)
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
