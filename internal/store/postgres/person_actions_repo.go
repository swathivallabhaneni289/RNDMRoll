package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// Mute implements social.Repository.
func (r *SocialRepo) Mute(ctx context.Context, muter, muted uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into mutes (muter_id, muted_id, created_at) values ($1, $2, $3)
		on conflict (muter_id, muted_id) do nothing`, muter, muted, at)
	return err
}

// Unmute implements social.Repository.
func (r *SocialRepo) Unmute(ctx context.Context, muter, muted uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from mutes where muter_id = $1 and muted_id = $2`, muter, muted)
	return err
}

// IsMuted implements social.Repository.
func (r *SocialRepo) IsMuted(ctx context.Context, muter, muted uuid.UUID) (bool, error) {
	var yes bool
	err := r.pool.QueryRow(ctx, `select exists (select 1 from mutes where muter_id = $1 and muted_id = $2)`, muter, muted).Scan(&yes)
	return yes, err
}

// Report implements social.Repository.
func (r *SocialRepo) Report(ctx context.Context, reporter, reported uuid.UUID, reason string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into user_reports (reporter_id, reported_id, reason, created_at) values ($1, $2, $3, $4)
		on conflict (reporter_id, reported_id) do update set reason = excluded.reason, created_at = excluded.created_at`,
		reporter, reported, reason, at)
	return err
}

// Bio implements social.Repository.
func (r *SocialRepo) Bio(ctx context.Context, id uuid.UUID) (string, error) {
	var bio string
	err := r.pool.QueryRow(ctx, `select coalesce(bio, '') from users where id = $1`, id).Scan(&bio)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", social.ErrNotFound
	}
	return bio, err
}
