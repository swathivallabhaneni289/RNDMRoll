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

// EmailVerificationRepo is a pgx-backed implementation of
// user.EmailVerificationRepository.
type EmailVerificationRepo struct {
	pool *pgxpool.Pool
}

// NewEmailVerificationRepo builds an EmailVerificationRepo backed by pool.
func NewEmailVerificationRepo(pool *pgxpool.Pool) user.EmailVerificationRepository {
	return &EmailVerificationRepo{pool: pool}
}

func (r *EmailVerificationRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	const q = `insert into email_verification_tokens (user_id, token_hash, expires_at) values ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, q, userID, tokenHash, expiresAt)
	return err
}

// ConsumeByHash atomically sets consumed_at and returns the owning user ID,
// or ErrTokenInvalid / ErrTokenConsumed / ErrTokenExpired. Expiry is
// compared against the passed-in now rather than SQL now() so callers (and
// tests) control the clock deterministically.
func (r *EmailVerificationRepo) ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var (
		userID     uuid.UUID
		expiresAt  time.Time
		consumedAt *time.Time
	)
	const selectQ = `select user_id, expires_at, consumed_at from email_verification_tokens where token_hash = $1 for update`
	err = tx.QueryRow(ctx, selectQ, tokenHash).Scan(&userID, &expiresAt, &consumedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, user.ErrTokenInvalid
		}
		return uuid.UUID{}, err
	}
	if consumedAt != nil {
		return uuid.UUID{}, user.ErrTokenConsumed
	}
	if now.After(expiresAt) {
		return uuid.UUID{}, user.ErrTokenExpired
	}

	const updateQ = `update email_verification_tokens set consumed_at = $1 where token_hash = $2`
	if _, err := tx.Exec(ctx, updateQ, now, tokenHash); err != nil {
		return uuid.UUID{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.UUID{}, err
	}
	return userID, nil
}

func (r *EmailVerificationRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `delete from email_verification_tokens where user_id = $1`
	_, err := r.pool.Exec(ctx, q, userID)
	return err
}
