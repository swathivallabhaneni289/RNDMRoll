package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// ReportPost implements board.Repository. A person reports a post once: a
// second report changes nothing and is board.ErrNotFound, so the first
// reason and the title kept with it stay, a closed report is never opened
// again, and of two reports that race only one is kept. A post deleted
// after the visibility read is board.ErrNotFound too.
func (r *BoardRepo) ReportPost(ctx context.Context, reporter, entryID, owner uuid.UUID, title, reason string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		insert into post_reports (reporter_id, entry_id, owner_id, title, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, entry_id) do nothing`,
		reporter, entryID, owner, title, reason, at)
	return reportKept(tag.RowsAffected(), err)
}

// ReportComment implements board.Repository, with ReportPost's rule for a
// comment.
func (r *BoardRepo) ReportComment(ctx context.Context, reporter, commentID, author uuid.UUID, body, reason string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		insert into comment_reports (reporter_id, comment_id, owner_id, body, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, comment_id) do nothing`,
		reporter, commentID, author, body, reason, at)
	return reportKept(tag.RowsAffected(), err)
}

// reportKept is nil when a report row was written, and board.ErrNotFound when
// none was (the reporter already had one) or the post or comment went away
// before the insert (a foreign key failure).
func reportKept(rows int64, err error) error {
	if isForeignKeyViolation(err) {
		return board.ErrNotFound
	}
	if err != nil {
		return err
	}
	if rows == 0 {
		return board.ErrNotFound
	}
	return nil
}
