package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ReportPost implements board.Repository. A second report by the same
// person replaces the reason and opens it again.
func (r *BoardRepo) ReportPost(ctx context.Context, reporter, entryID, owner uuid.UUID, title, reason string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into post_reports (reporter_id, entry_id, owner_id, title, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, entry_id) do update
		set reason = excluded.reason, created_at = excluded.created_at, resolved_at = null, resolution = null`,
		reporter, entryID, owner, title, reason, at)
	return err
}

// ReportComment implements board.Repository.
func (r *BoardRepo) ReportComment(ctx context.Context, reporter, commentID, author uuid.UUID, body, reason string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into comment_reports (reporter_id, comment_id, owner_id, body, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, comment_id) do update
		set reason = excluded.reason, created_at = excluded.created_at, resolved_at = null, resolution = null`,
		reporter, commentID, author, body, reason, at)
	return err
}
