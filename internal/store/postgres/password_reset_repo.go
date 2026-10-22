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

// PasswordResetRepo stores Forgot password codes (migrations 0014 and 0016).
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
// only after an expiry does a code start from zero. The unexpired code a new
// one replaces is kept as the previous code, so Try can say it was replaced;
// an expired one is not. A disabled account gets no code: the statement
// inserts or replaces one only for an account it reads as not disabled. The
// read takes no lock (users comes after password_resets in the order of
// session_locks.go), and a disable deletes the code in its own transaction.
// stored reports whether the new code was kept (and so is to be emailed).
func (r *PasswordResetRepo) Issue(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at, freshSince time.Time, maxTries int) (stored bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		insert into password_resets (user_id, code_hash, expires_at, attempts, created_at)
		select id, $2::bytea, $3::timestamptz, 0, $4::timestamptz from users where id = $1 and disabled_at is null
		on conflict (user_id) do update set code_hash = excluded.code_hash, expires_at = excluded.expires_at,
			attempts = case when password_resets.expires_at > excluded.created_at then password_resets.attempts else 0 end,
			previous_code_hash = case when password_resets.expires_at > excluded.created_at then password_resets.code_hash end,
			created_at = excluded.created_at
		where password_resets.expires_at <= excluded.created_at
			or (password_resets.attempts < $5 and password_resets.created_at <= $6)`,
		userID, codeHash, expiresAt, at, maxTries, freshSince)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Try checks codeHash against the person's code, in one statement, and counts
// a try only when it is wrong: the right code never costs a try, so a change
// that then fails or is given up loses nothing, and wrong guesses sent at
// once still stop at maxTries (each waits for the one before and sees its
// count). right is true for the person's code; replaced is true, with no try
// counted either, for the code that one replaced (only someone who got the
// earlier email can know it). Both are false, and nothing is counted, when
// there is no code, it has expired or it is used up: then even the right or
// the previous code is just wrong. A uuid.Nil userID matches no code, so an
// email with no account runs the same statement.
func (r *PasswordResetRepo) Try(ctx context.Context, userID uuid.UUID, codeHash []byte, at time.Time, maxTries int) (right, replaced bool, err error) {
	err = r.pool.QueryRow(ctx, `
		update password_resets
		set attempts = attempts + case when code_hash = $2 or previous_code_hash = $2 then 0 else 1 end
		where user_id = $1 and attempts < $3 and expires_at > $4
		returning code_hash = $2, coalesce(previous_code_hash = $2, false)`,
		userID, codeHash, maxTries, at).Scan(&right, &replaced)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return right, replaced && !right, nil
}

// SpendAndChangePassword deletes the person's code if it is still codeHash
// and, in the same transaction, changes their password as
// UserRepo.ChangePassword does (the new session's refresh token is stored
// under sessionID): both commit or neither does, so a change that fails or is
// given up part way leaves the code as it was. Only one request can spend a
// code: the delete holds the code's row until the end, so a second request
// with the same code waits for the first and then finds nothing to spend,
// unless the first rolled back. The previous code goes with the row.
//
// It returns nil once the code is spent and the password changed. When there
// was no code to spend, nothing changes and the error says why (whyNotSpent):
// user.ErrNotFound or user.ErrAccountDisabled for an account that is gone or
// disabled, user.ErrResetCodeReplaced when a newer code replaced this one
// meanwhile, and user.ErrResetCodeUsed when another request spent it. A
// disabled account is also refused when the code is still there
// (changePassword), and keeps its code.
func (r *PasswordResetRepo) SpendAndChangePassword(ctx context.Context, userID uuid.UUID, codeHash []byte,
	passwordHash string, sessionID uuid.UUID, newTokenHash []byte, expiresAt time.Time, userAgent *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	tag, err := tx.Exec(ctx, `delete from password_resets where user_id = $1 and code_hash = $2`, userID, codeHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return whyNotSpent(ctx, tx, userID, codeHash)
	}
	if err := changePassword(ctx, tx, userID, passwordHash, sessionID, newTokenHash, expiresAt, userAgent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// whyNotSpent says why the code codeHash, right when it was tried a moment
// ago, was no longer there to spend. The account comes first, so one that is
// gone or disabled (a disable deletes the code) gets the answer any wrong
// code gets. Then: the code is the one the newest code replaced, or else
// another request spent it (and a new code may have been asked for since).
func whyNotSpent(ctx context.Context, tx pgx.Tx, userID uuid.UUID, codeHash []byte) error {
	var disabled bool
	err := tx.QueryRow(ctx, `select disabled_at is not null from users where id = $1`, userID).Scan(&disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.ErrNotFound
	}
	if err != nil {
		return err
	}
	if disabled {
		return user.ErrAccountDisabled
	}
	var replaced bool
	err = tx.QueryRow(ctx, `select coalesce(previous_code_hash = $2, false) from password_resets where user_id = $1`,
		userID, codeHash).Scan(&replaced)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if replaced {
		return user.ErrResetCodeReplaced
	}
	return user.ErrResetCodeUsed
}
