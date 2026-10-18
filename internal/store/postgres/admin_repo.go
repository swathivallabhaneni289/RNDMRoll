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

// openReports is every open report as (person, reason, created_at).
const openReports = `
	select reported_id as person, reason, created_at from user_reports where resolved_at is null
	union all
	select owner_id, reason, created_at from post_reports where resolved_at is null
	union all
	select owner_id, reason, created_at from comment_reports where resolved_at is null`

func scanSummaries(rows pgx.Rows) ([]admin.Summary, error) {
	defer rows.Close()
	out := []admin.Summary{}
	for rows.Next() {
		var s admin.Summary
		if err := rows.Scan(&s.PersonID, &s.Username, &s.Name, &s.Disabled, &s.Open, &s.Reasons, &s.LatestAt); err != nil {
			return nil, err
		}
		s.LatestAt = s.LatestAt.UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}

// Summaries implements admin.Repository: people with open reports, the most
// reported first, then the newest.
func (r *AdminRepo) Summaries(ctx context.Context) ([]admin.Summary, error) {
	rows, err := r.pool.Query(ctx, `
		select u.id, coalesce(u.username, ''), coalesce(u.name, ''), u.disabled_at is not null,
		       count(*)::int, array_agg(distinct o.reason order by o.reason), max(o.created_at)
		from (`+openReports+`) o join users u on u.id = o.person
		group by u.id
		order by count(*) desc, max(o.created_at) desc`)
	if err != nil {
		return nil, err
	}
	return scanSummaries(rows)
}

// DisabledPeople implements admin.Repository: disabled accounts, newest first.
func (r *AdminRepo) DisabledPeople(ctx context.Context) ([]admin.Summary, error) {
	rows, err := r.pool.Query(ctx, `
		select u.id, coalesce(u.username, ''), coalesce(u.name, ''), true, 0, array[]::text[], u.disabled_at
		from users u where u.disabled_at is not null
		order by u.disabled_at desc`)
	if err != nil {
		return nil, err
	}
	return scanSummaries(rows)
}

// Person implements admin.Repository.
func (r *AdminRepo) Person(ctx context.Context, id uuid.UUID) (admin.Person, error) {
	var p admin.Person
	err := r.pool.QueryRow(ctx, `
		select id, coalesce(username, ''), coalesce(name, ''), email, coalesce(bio, ''), created_at, disabled_at
		from users where id = $1`, id).Scan(&p.ID, &p.Username, &p.Name, &p.Email, &p.Bio, &p.CreatedAt, &p.DisabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.Person{}, admin.ErrNotFound
	}
	return p, err
}

// Reports implements admin.Repository: every report about the person, open
// ones first, newest first within each.
func (r *AdminRepo) Reports(ctx context.Context, person uuid.UUID) ([]admin.Report, error) {
	rows, err := r.pool.Query(ctx, `
		select * from (
			select 'person' as kind, ur.reporter_id as id, coalesce(rp.username, '') as reporter, ur.reason,
			       ur.created_at, ur.resolved_at, coalesce(ur.resolution, '') as resolution,
			       null::uuid as entry_id, '' as title, '' as photo_key, null::uuid as comment_id, '' as body
			from user_reports ur join users rp on rp.id = ur.reporter_id
			where ur.reported_id = $1
			union all
			select 'post', pr.id, coalesce(rp.username, ''), pr.reason, pr.created_at, pr.resolved_at,
			       coalesce(pr.resolution, ''), pr.entry_id, pr.title,
			       coalesce((select ph.storage_key from entry_photos ph where ph.entry_id = pr.entry_id order by ph.position limit 1), ''),
			       null::uuid, ''
			from post_reports pr join users rp on rp.id = pr.reporter_id
			where pr.owner_id = $1
			union all
			select 'comment', cr.id, coalesce(rp.username, ''), cr.reason, cr.created_at, cr.resolved_at,
			       coalesce(cr.resolution, ''), null::uuid, '',
			       coalesce((select pc.photo_key from post_comments pc where pc.id = cr.comment_id), ''),
			       cr.comment_id, cr.body
			from comment_reports cr join users rp on rp.id = cr.reporter_id
			where cr.owner_id = $1
		) all_reports
		order by (resolved_at is null) desc, created_at desc
		limit 200`, person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Report{}
	for rows.Next() {
		var rep admin.Report
		if err := rows.Scan(&rep.Kind, &rep.ID, &rep.Reporter, &rep.Reason, &rep.CreatedAt, &rep.ResolvedAt,
			&rep.Resolution, &rep.EntryID, &rep.Title, &rep.PhotoKey, &rep.CommentID, &rep.Body); err != nil {
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

// ResolveTarget implements admin.Repository.
func (r *AdminRepo) ResolveTarget(ctx context.Context, kind string, target uuid.UUID, resolution string, at time.Time) error {
	q := `update post_reports set resolved_at = $3, resolution = $2 where entry_id = $1 and resolved_at is null`
	if kind == admin.KindComment {
		q = `update comment_reports set resolved_at = $3, resolution = $2 where comment_id = $1 and resolved_at is null`
	}
	_, err := r.pool.Exec(ctx, q, target, resolution, at)
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

// SetDisabled implements admin.Repository.
func (r *AdminRepo) SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time) error {
	tag, err := r.pool.Exec(ctx, `update users set disabled_at = $2 where id = $1`, person, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return admin.ErrNotFound
	}
	return nil
}

// DeleteComment implements admin.Repository (its replies go with it).
func (r *AdminRepo) DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error) {
	keys, err := deleteCommentRows(ctx, r.pool, id)
	if errors.Is(err, board.ErrNotFound) {
		return nil, nil
	}
	return keys, err
}
