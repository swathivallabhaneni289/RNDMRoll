package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// ReportPost implements board.Repository. A person reports a post once: a
// second report changes nothing and is board.ErrNotFound, so the first
// reason and the title kept with it stay, a closed report is never opened
// again, and of two reports that race only one is kept. A post deleted or
// removed after the visibility read is board.ErrNotFound too. hid says this
// report made the post hidden while under review (keepReport).
func (r *BoardRepo) ReportPost(ctx context.Context, reporter, entryID, owner uuid.UUID, title, reason string, at time.Time) (bool, error) {
	return r.keepReport(ctx, entryID,
		`select 1 from entries where id = $1 and removed_at is null for no key update`,
		`insert into post_reports (reporter_id, entry_id, owner_id, title, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, entry_id) do nothing`,
		`select `+postOpenReports("e")+` from entries e where e.id = $1`,
		reporter, entryID, owner, title, reason, at)
}

// ReportComment implements board.Repository, with ReportPost's rule for a
// comment.
func (r *BoardRepo) ReportComment(ctx context.Context, reporter, commentID, author uuid.UUID, body, reason string, at time.Time) (bool, error) {
	return r.keepReport(ctx, commentID,
		`select 1 from post_comments where id = $1 and removed_at is null for no key update`,
		`insert into comment_reports (reporter_id, comment_id, owner_id, body, reason, created_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (reporter_id, comment_id) do nothing`,
		`select `+commentOpenReports("c")+` from post_comments c where c.id = $1`,
		reporter, commentID, author, body, reason, at)
}

// keepReport writes one report of the post or comment target in a
// transaction: hold holds the target (gone or removed is board.ErrNotFound),
// insert writes the report with args, and count counts the open reports on
// it afterwards. The hold makes the reports of one target wait for each
// other, and a dismissal or a removal of it too, so each count is its own:
// the report that brings it to exactly social.ReviewThreshold is the one
// that hid it, and only that one says so (the admin email treats it as
// urgent). The reporter always counts, since a gone account makes no
// requests.
func (r *BoardRepo) keepReport(ctx context.Context, target uuid.UUID, hold, insert, count string, args ...any) (hid bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var held int
	if err := tx.QueryRow(ctx, hold, target).Scan(&held); errors.Is(err, pgx.ErrNoRows) {
		return false, board.ErrNotFound
	} else if err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, insert, args...)
	if err := reportKept(tag.RowsAffected(), err); err != nil {
		return false, err
	}
	var open int
	if err := tx.QueryRow(ctx, count, target).Scan(&open); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return open == social.ReviewThreshold, nil
}

// reportKept is nil when a report row was written, and board.ErrNotFound when
// none was (the reporter already had one) or an account it names went away
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
