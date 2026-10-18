package board

import (
	"context"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

func validReason(reason string) bool {
	for _, r := range social.ReportReasons {
		if r == reason {
			return true
		}
	}
	return false
}

// ReportPost keeps the viewer's report of a post they can see (the same rule
// as the post page). Your own post, and one you cannot see, are ErrNotFound;
// an unknown reason is social.ErrBadReason. The owner is never told.
func (s *Service) ReportPost(ctx context.Context, viewer, entryID uuid.UUID, reason string) error {
	if !validReason(reason) {
		return social.ErrBadReason
	}
	row, _, err := s.visible(ctx, viewer, entryID)
	if err != nil {
		return err
	}
	if row.Owner.ID == viewer {
		return ErrNotFound
	}
	return s.repo.ReportPost(ctx, viewer, row.ID, row.Owner.ID, row.Title, reason, s.clk.NowFor(viewer.String()).UTC())
}

// ReportComment keeps the viewer's report of a comment they can read on a
// post they can see. Your own comment, a hidden one you may not read, and a
// writer with a block either way are ErrNotFound.
func (s *Service) ReportComment(ctx context.Context, viewer, commentID uuid.UUID, reason string) error {
	if !validReason(reason) {
		return social.ErrBadReason
	}
	c, err := s.repo.CommentByID(ctx, commentID)
	if err != nil {
		return err
	}
	if c.Author.ID == viewer {
		return ErrNotFound
	}
	if _, _, err := s.visible(ctx, viewer, c.EntryID); err != nil {
		return err
	}
	if c.Hidden && c.EntryOwner != viewer {
		return ErrNotFound
	}
	if _, err := s.social.Profile(ctx, viewer, c.Author.ID); err != nil {
		return ErrNotFound
	}
	return s.repo.ReportComment(ctx, viewer, c.ID, c.Author.ID, c.Body, reason, s.clk.NowFor(viewer.String()).UTC())
}
