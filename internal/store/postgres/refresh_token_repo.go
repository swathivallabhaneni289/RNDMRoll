package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// RefreshTokenRepo is a pgx-backed implementation of user.RefreshTokenRepository.
type RefreshTokenRepo struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepo builds a RefreshTokenRepo backed by pool.
func NewRefreshTokenRepo(pool *pgxpool.Pool) user.RefreshTokenRepository {
	return &RefreshTokenRepo{pool: pool}
}

func (r *RefreshTokenRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error) {
	const q = `insert into refresh_tokens (user_id, token_hash, expires_at, user_agent) values ($1, $2, $3, $4::text) returning id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, userID, tokenHash, expiresAt, userAgent).Scan(&id); err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// GetActiveByHash returns ErrTokenExpired for a past expires_at and
// ErrTokenInvalid for a revoked or missing row.
func (r *RefreshTokenRepo) GetActiveByHash(ctx context.Context, tokenHash []byte) (*user.RefreshToken, error) {
	const q = `select id, user_id, expires_at, revoked_at from refresh_tokens where token_hash = $1`
	var rt user.RefreshToken
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(&rt.ID, &rt.UserID, &rt.ExpiresAt, &rt.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrTokenInvalid
		}
		return nil, err
	}
	if rt.RevokedAt != nil {
		return nil, user.ErrTokenInvalid
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, user.ErrTokenExpired
	}
	return &rt, nil
}

// Rotate revokes oldID and inserts a new row in one transaction, so a crash
// mid-rotation cannot leave two simultaneously valid refresh tokens. The
// new row is inserted before the old row is updated: rotated_to is a
// foreign key into this same table, so it must reference an existing row.
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var userID uuid.UUID
	var userAgent *string
	const selectQ = `select user_id, user_agent from refresh_tokens where id = $1`
	if err := tx.QueryRow(ctx, selectQ, oldID).Scan(&userID, &userAgent); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, user.ErrTokenInvalid
		}
		return uuid.UUID{}, err
	}

	var newID uuid.UUID
	const insertQ = `insert into refresh_tokens (user_id, token_hash, expires_at, user_agent) values ($1, $2, $3, $4::text) returning id`
	if err := tx.QueryRow(ctx, insertQ, userID, newHash, expiresAt, userAgent).Scan(&newID); err != nil {
		return uuid.UUID{}, err
	}

	const updateQ = `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2`
	if _, err := tx.Exec(ctx, updateQ, newID, oldID); err != nil {
		return uuid.UUID{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.UUID{}, err
	}
	return newID, nil
}

func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, tokenHash []byte) error {
	const q = `update refresh_tokens set revoked_at = now() where token_hash = $1 and revoked_at is null`
	tag, err := r.pool.Exec(ctx, q, tokenHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return user.ErrTokenInvalid
	}
	return nil
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null`
	_, err := r.pool.Exec(ctx, q, userID)
	return err
}
