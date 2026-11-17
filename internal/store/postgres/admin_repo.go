package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// AdminRepo is admin.Repository over the pool. Unlike every other read, it
// sees disabled accounts and ignores blocks: it is the admin's view.
type AdminRepo struct {
	pool *pgxpool.Pool
}

// NewAdminRepo builds an AdminRepo backed by pool.
func NewAdminRepo(pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{pool: pool}
}

var _ admin.Repository = (*AdminRepo)(nil)

// adminPersonColumns are a person's columns as the admin page reads them (users
// as u), in scanAdminPerson's order. The email is never among them.
const adminPersonColumns = `u.id, coalesce(u.username, ''), coalesce(u.name, ''), coalesce(u.bio, ''), coalesce(u.avatar_key, ''),
	u.created_at, u.disabled_at, u.deletion_requested_at`

// scanAdminPerson reads adminPersonColumns into p, ahead of rest.
func scanAdminPerson(row pgx.Row, p *admin.Person, rest ...any) error {
	return row.Scan(append([]any{&p.ID, &p.Username, &p.Name, &p.Bio, &p.PhotoKey, &p.CreatedAt, &p.DisabledAt,
		&p.DeletionRequestedAt}, rest...)...)
}

// allReports is every report in the three tables as the page reads it, with
// owner: the person reported, or whose post or comment it is. A post report
// carries the post as it is now (its title, its day, its text and every
// photo, in order), so the admin sees what was reported in full; a post that
// is gone has none of them. Callers filter it by owner or by resolved_at.
const allReports = `
	select 'person' as kind, ur.reported_id as owner, ur.reporter_id as id, coalesce(rp.username, '') as reporter,
	       ur.reason, ur.created_at, ur.resolved_at, coalesce(ur.resolution, '') as resolution,
	       null::uuid as entry_id, '' as title, '' as title_now, '' as local_date, '' as note,
	       array[]::text[] as photo_keys, null::uuid as comment_id, '' as body, '' as photo_key
	from user_reports ur join users rp on rp.id = ur.reporter_id
	union all
	select 'post', pr.owner_id, pr.id, coalesce(rp.username, ''), pr.reason, pr.created_at, pr.resolved_at,
	       coalesce(pr.resolution, ''), pr.entry_id, pr.title, coalesce(e.title, ''),
	       coalesce(e.local_date::text, ''), coalesce(e.comment, ''),
	       coalesce((select array_agg(ph.storage_key order by ph.position) from entry_photos ph
	                 where ph.entry_id = pr.entry_id), array[]::text[]),
	       null::uuid, '', ''
	from post_reports pr join users rp on rp.id = pr.reporter_id
	left join entries e on e.id = pr.entry_id
	union all
	select 'comment', cr.owner_id, cr.id, coalesce(rp.username, ''), cr.reason, cr.created_at, cr.resolved_at,
	       coalesce(cr.resolution, ''), null::uuid, '', '', '', '', array[]::text[],
	       cr.comment_id, cr.body,
	       coalesce((select pc.photo_key from post_comments pc where pc.id = cr.comment_id), '')
	from comment_reports cr join users rp on rp.id = cr.reporter_id`

// reportColumns are allReports' columns (as r) in scanReport's order.
const reportColumns = `r.kind, r.id, r.reporter, r.reason, r.created_at, r.resolved_at, r.resolution,
	r.entry_id, r.title, r.title_now, r.local_date, r.note, r.photo_keys, r.comment_id, r.body, r.photo_key`

// reportFields are rep's fields in reportColumns' order.
func reportFields(rep *admin.Report) []any {
	return []any{&rep.Kind, &rep.ID, &rep.Reporter, &rep.Reason, &rep.CreatedAt, &rep.ResolvedAt,
		&rep.Resolution, &rep.EntryID, &rep.Title, &rep.TitleNow, &rep.LocalDate, &rep.Note, &rep.PhotoKeys,
		&rep.CommentID, &rep.Body, &rep.PhotoKey}
}

// OpenReports implements admin.Repository: every open report with whose it
// is, newest first, at most admin.OpenReportsLimit.
func (r *AdminRepo) OpenReports(ctx context.Context) ([]admin.OpenReport, error) {
	rows, err := r.pool.Query(ctx, `
		select `+adminPersonColumns+`, `+reportColumns+`
		from (`+allReports+`) r join users u on u.id = r.owner
		where r.resolved_at is null
		order by r.created_at desc
		limit $1`, admin.OpenReportsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.OpenReport{}
	for rows.Next() {
		var o admin.OpenReport
		if err := scanAdminPerson(rows, &o.Owner, reportFields(&o.Report)...); err != nil {
			return nil, err
		}
		o.Report.CreatedAt = o.Report.CreatedAt.UTC()
		out = append(out, o)
	}
	return out, rows.Err()
}

// DisabledPeople implements admin.Repository: disabled accounts, the most
// recently disabled first.
func (r *AdminRepo) DisabledPeople(ctx context.Context) ([]admin.Person, error) {
	rows, err := r.pool.Query(ctx, `
		select `+adminPersonColumns+`
		from users u where u.disabled_at is not null
		order by u.disabled_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Person{}
	for rows.Next() {
		var p admin.Person
		if err := scanAdminPerson(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Person implements admin.Repository: a person with a report about them, a
// post or a comment (open or closed), or a disabled account. Anyone else is
// admin.ErrNotFound, like an account that is not there, and the email is not
// read at all. The profile photo is its storage key (avatar_key); the page
// signs the link.
func (r *AdminRepo) Person(ctx context.Context, id uuid.UUID) (admin.Person, error) {
	var p admin.Person
	err := scanAdminPerson(r.pool.QueryRow(ctx, `
		select `+adminPersonColumns+`
		from users u
		where u.id = $1 and (u.disabled_at is not null
		       or exists (select 1 from user_reports where reported_id = u.id)
		       or exists (select 1 from post_reports where owner_id = u.id)
		       or exists (select 1 from comment_reports where owner_id = u.id))`, id), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.Person{}, admin.ErrNotFound
	}
	return p, err
}

// Reports implements admin.Repository: every report about the person, open
// ones first, newest first within each (allReports says what each carries).
func (r *AdminRepo) Reports(ctx context.Context, person uuid.UUID) ([]admin.Report, error) {
	rows, err := r.pool.Query(ctx, `
		select `+reportColumns+`
		from (`+allReports+`) r
		where r.owner = $1
		order by (r.resolved_at is null) desc, r.created_at desc
		limit 200`, person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Report{}
	for rows.Next() {
		var rep admin.Report
		if err := rows.Scan(reportFields(&rep)...); err != nil {
			return nil, err
		}
		rep.CreatedAt = rep.CreatedAt.UTC()
		out = append(out, rep)
	}
	return out, rows.Err()
}

// Resolve implements admin.Repository.
func (r *AdminRepo) Resolve(ctx context.Context, kind string, id, person uuid.UUID, resolution string, at time.Time) (bool, error) {
	var q string
	switch kind {
	case admin.KindPerson:
		q = `update user_reports set resolved_at = $4, resolution = $3
			where reporter_id = $1 and reported_id = $2 and resolved_at is null`
	case admin.KindPost:
		q = `update post_reports set resolved_at = $4, resolution = $3
			where id = $1 and owner_id = $2 and resolved_at is null`
	case admin.KindComment:
		q = `update comment_reports set resolved_at = $4, resolution = $3
			where id = $1 and owner_id = $2 and resolved_at is null`
	default:
		return false, admin.ErrNotFound
	}
	tag, err := r.pool.Exec(ctx, q, id, person, resolution, at)
	return tag.RowsAffected() > 0, err
}

// OpenReportIDs implements admin.Repository.
func (r *AdminRepo) OpenReportIDs(ctx context.Context, kind string, target uuid.UUID) ([]uuid.UUID, error) {
	q := `select id from post_reports where entry_id = $1 and resolved_at is null`
	if kind == admin.KindComment {
		q = `select id from comment_reports where comment_id = $1 and resolved_at is null`
	} else if kind != admin.KindPost {
		return nil, admin.ErrNotFound
	}
	rows, err := r.pool.Query(ctx, q, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ResolveIDs implements admin.Repository. It closes by id, not by target, so
// it still finds the reports of a post or comment that is gone.
func (r *AdminRepo) ResolveIDs(ctx context.Context, kind string, ids []uuid.UUID, resolution string, at time.Time) error {
	q := `update post_reports set resolved_at = $3, resolution = $2 where id = any($1::uuid[]) and resolved_at is null`
	if kind == admin.KindComment {
		q = `update comment_reports set resolved_at = $3, resolution = $2 where id = any($1::uuid[]) and resolved_at is null`
	} else if kind != admin.KindPost {
		return admin.ErrNotFound
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, q, ids, resolution, at)
	return err
}

// ResolvePerson implements admin.Repository.
func (r *AdminRepo) ResolvePerson(ctx context.Context, person uuid.UUID, resolution string, at time.Time) error {
	for _, q := range []string{
		`update user_reports set resolved_at = $3, resolution = $2 where reported_id = $1 and resolved_at is null`,
		`update post_reports set resolved_at = $3, resolution = $2 where owner_id = $1 and resolved_at is null`,
		`update comment_reports set resolved_at = $3, resolution = $2 where owner_id = $1 and resolved_at is null`,
	} {
		if _, err := r.pool.Exec(ctx, q, person, resolution, at); err != nil {
			return err
		}
	}
	return nil
}

// ReportTarget implements admin.Repository.
func (r *AdminRepo) ReportTarget(ctx context.Context, kind string, id uuid.UUID) (*uuid.UUID, uuid.UUID, error) {
	q := `select entry_id, owner_id from post_reports where id = $1`
	if kind == admin.KindComment {
		q = `select comment_id, owner_id from comment_reports where id = $1`
	} else if kind != admin.KindPost {
		return nil, uuid.UUID{}, admin.ErrNotFound
	}
	var target *uuid.UUID
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, q, id).Scan(&target, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.UUID{}, admin.ErrNotFound
	}
	return target, owner, err
}

// SetDisabled implements admin.Repository. Disabling also deletes the
// account's Forgot password code, so none can be used while it is disabled
// (or after it is enabled again), revokes every refresh token, so the account
// is logged out everywhere, and deletes its push tokens, so its phones stop
// getting pushes at once. One transaction, in the order of session_locks.go.
// The revoke runs twice, as in a password change (UserRepo.ChangePassword
// says why): a refresh in flight at this moment cannot leave a live session
// behind.
func (r *AdminRepo) SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if at != nil {
		if _, err := tx.Exec(ctx, `delete from password_resets where user_id = $1`, person); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `update users set disabled_at = $2 where id = $1`, person, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return admin.ErrNotFound
	}
	if at != nil {
		for range 2 {
			if _, err := tx.Exec(ctx, revokeAllQ, person); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, deletePushTokensQ, person); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeleteComment implements admin.Repository (its replies go with it).
func (r *AdminRepo) DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error) {
	keys, err := deleteCommentRows(ctx, r.pool, id)
	if errors.Is(err, board.ErrNotFound) {
		return nil, nil
	}
	return keys, err
}
