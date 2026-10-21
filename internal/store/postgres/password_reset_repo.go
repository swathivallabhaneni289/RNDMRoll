package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PasswordReset is one outstanding Forgot password code.
type PasswordReset struct {
	CodeHash  []byte
	ExpiresAt time.Time
	Attempts  int
}

// PasswordResetRepo stores Forgot password codes (migration 0014).
type PasswordResetRepo struct {
	pool *pgxpool.Pool
}

// NewPasswordResetRepo builds a PasswordResetRepo backed by pool.
func NewPasswordResetRepo(pool *pgxpool.Pool) *PasswordResetRepo {
	return &PasswordResetRepo{pool: pool}
}

// Put replaces the person's code.
func (r *PasswordResetRepo) Put(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into password_resets (user_id, code_hash, expires_at, attempts, created_at) values ($1, $2, $3, 0, $4)
		on conflict (user_id) do update set code_hash = excluded.code_hash, expires_at = excluded.expires_at,
			attempts = 0, created_at = excluded.created_at`, userID, codeHash, expiresAt, at)
	return err
}

// Get reads the person's code; ok false when there is none.
func (r *PasswordResetRepo) Get(ctx context.Context, userID uuid.UUID) (PasswordReset, bool, error) {
	var p PasswordReset
	err := r.pool.QueryRow(ctx, `select code_hash, expires_at, attempts from password_resets where user_id = $1`, userID).
		Scan(&p.CodeHash, &p.ExpiresAt, &p.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return PasswordReset{}, false, nil
	}
	return p, err == nil, err
}

// CountTry adds one wrong try.
func (r *PasswordResetRepo) CountTry(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `update password_resets set attempts = attempts + 1 where user_id = $1`, userID)
	return err
}

// Delete removes the person's code (used, or replaced by the new password).
func (r *PasswordResetRepo) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from password_resets where user_id = $1`, userID)
	return err
}
