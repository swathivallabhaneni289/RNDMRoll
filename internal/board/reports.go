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
// as the post page). Your own post, one you cannot see and one you already
// reported (it is gone for you) are ErrNotFound; an unknown reason is
// social.ErrBadReason. The owner is never told who reported it, and hears of
// reports only from the one that hides the post (the bell). The report
// notifier hears only a report that was kept.
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
	hid, err := s.repo.ReportPost(ctx, viewer, row.ID, row.Owner.ID, row.Title, reason,
		s.clk.NowFor(viewer.String()).UTC(), s.clk.NowFor(row.Owner.ID.String()).UTC())
	if err != nil {
		return err
	}
	s.reported(reason, hid)
	return nil
}

// ReportComment keeps the viewer's report of a comment that the post page
// lists for them (visibleComment), on a post they can see. Your own comment
// is ErrNotFound, and so is any comment the page leaves out: a writer with a
// block either way, one the post's author hid (unless you are that author),
// one you already reported, one under review or removed, and a reply whose
// top-level comment is left out for any of these reasons.
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
	if _, err := s.visibleComment(ctx, viewer, c.EntryID, c.ID); err != nil {
		return err
	}
	hid, err := s.repo.ReportComment(ctx, viewer, c.ID, c.Author.ID, c.Body, reason,
		s.clk.NowFor(viewer.String()).UTC(), s.clk.NowFor(c.Author.ID.String()).UTC())
	if err != nil {
		return err
	}
	s.reported(reason, hid)
	return nil
}

// reported tells the report notifier, when there is one, about a report
// kept: as a hiding one when it made its post or comment hidden while under
// review, which the admin email sends at once.
func (s *Service) reported(reason string, hid bool) {
	switch {
	case s.reports == nil:
	case hid:
		s.reports.ReportedHiding(reason)
	default:
		s.reports.Reported(reason)
	}
}
