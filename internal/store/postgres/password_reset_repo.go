package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PasswordResetRepo stores Forgot password codes (migration 0014).
type PasswordResetRepo struct {
	pool *pgxpool.Pool
}

// NewPasswordResetRepo builds a PasswordResetRepo backed by pool.
func NewPasswordResetRepo(pool *pgxpool.Pool) *PasswordResetRepo {
	return &PasswordResetRepo{pool: pool}
}

// Issue stores a new code for the person, in one statement, unless the one
// they have must stand: a used-up code (maxTries tries) stays until it
// expires, and one made after freshSince is not replaced. A new code keeps
// the tries of an unexpired one, so asking again never buys more guesses;
// only after an expiry does a code start from zero. stored reports whether
// the new code was kept (and so is to be emailed).
func (r *PasswordResetRepo) Issue(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at, freshSince time.Time, maxTries int) (stored bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		insert into password_resets (user_id, code_hash, expires_at, attempts, created_at) values ($1, $2, $3, 0, $4)
		on conflict (user_id) do update set code_hash = excluded.code_hash, expires_at = excluded.expires_at,
			attempts = case when password_resets.expires_at > excluded.created_at then password_resets.attempts else 0 end,
			created_at = excluded.created_at
		where password_resets.expires_at <= excluded.created_at
			or (password_resets.attempts < $5 and password_resets.created_at <= $6)`,
		userID, codeHash, expiresAt, at, maxTries, freshSince)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Try counts one try at the person's code and returns its hash, in one
// statement, so tries sent at once still stop at maxTries. ok is false, and
// nothing is counted, when there is no code, it has expired or it is used up.
// The right code's try is counted too; RefundTry gives it back if the
// password then does not change.
func (r *PasswordResetRepo) Try(ctx context.Context, userID uuid.UUID, at time.Time, maxTries int) (codeHash []byte, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
		update password_resets set attempts = attempts + 1
		where user_id = $1 and attempts < $2 and expires_at > $3
		returning code_hash`, userID, maxTries, at).Scan(&codeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return codeHash, err == nil, err
}

// RefundTry gives back one try at the person's code, if it is still codeHash
// and has a try to give: the right code's try, when the password then did not
// change. It only ever lowers the count of the code it names, and never below
// zero, so it cannot give back a wrong guess at another code; a code spent or
// replaced in the meantime is left as it is, and a spent one never comes back.
func (r *PasswordResetRepo) RefundTry(ctx context.Context, userID uuid.UUID, codeHash []byte) error {
	_, err := r.pool.Exec(ctx, `
		update password_resets set attempts = attempts - 1
		where user_id = $1 and code_hash = $2 and attempts > 0`, userID, codeHash)
	return err
}

// SpendAndChangePassword deletes the person's code if it is still codeHash
// and, in the same transaction, changes their password as
// UserRepo.ChangePassword does: both commit or neither does, so a change that
// fails or is given up part way leaves the code as it was. Only one request
// can spend a code: the delete holds the code's row until the end, so a second
// request with the same code waits for the first and then finds nothing to
// spend (spent is false, and nothing changed), unless the first rolled back.
// An account that is gone is user.ErrNotFound.
func (r *PasswordResetRepo) SpendAndChangePassword(ctx context.Context, userID uuid.UUID, codeHash []byte,
	passwordHash string, newTokenHash []byte, expiresAt time.Time, userAgent *string) (spent bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	tag, err := tx.Exec(ctx, `delete from password_resets where user_id = $1 and code_hash = $2`, userID, codeHash)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if err := changePassword(ctx, tx, userID, passwordHash, newTokenHash, expiresAt, userAgent); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
