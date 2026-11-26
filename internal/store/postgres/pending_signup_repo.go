package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// PendingSignupRepo stores the sign-ups waiting for their email code
// (migration 0023) and makes each account in the transaction that spends its
// code. It is also the purge's sweep of the sign-ups never finished.
type PendingSignupRepo struct {
	pool *pgxpool.Pool
}

// NewPendingSignupRepo builds a PendingSignupRepo backed by pool.
func NewPendingSignupRepo(pool *pgxpool.Pool) *PendingSignupRepo {
	return &PendingSignupRepo{pool: pool}
}

var _ purge.SignupStore = (*PendingSignupRepo)(nil)

// Start stores the sign-up p, with the token tokenHash and its first code
// codeHash (live until expiresAt), started at at, in one statement, unless
// its email already has maxStarts sign-ups started after countSince. The
// count is a plain read inside the insert and takes no lock, so sign-ups of
// one email started at the very same moment can each miss the others and
// overshoot the cap by a few; the limits in front of the route (3 a minute
// per address and email, 10 per address) keep that small. Every sign-up is
// its own row, so a start never touches another person's sign-up of the
// same email. stored reports whether the sign-up was kept.
func (r *PendingSignupRepo) Start(ctx context.Context, p user.PendingSignup, tokenHash, codeHash []byte,
	expiresAt, at, countSince time.Time, maxStarts int) (stored bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		insert into pending_signups (id, token_hash, email, password_hash, name, username, bio, birthday,
			code_hash, expires_at, code_created_at, codes_sent, attempts, created_at)
		select $1, $2::bytea, $3::text, $4::text, $5::text, $6::text, $7::text, $8::date,
			$9::bytea, $10::timestamptz, $11::timestamptz, 1, 0, $11::timestamptz
		where (select count(*) from pending_signups
			where lower(email) = lower($3::text) and created_at > $12::timestamptz) < $13`,
		p.ID, tokenHash, p.Email, p.PasswordHash, p.Name, p.Username, p.Bio, p.Birthday,
		codeHash, expiresAt, at, countSince, maxStarts)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Find returns the sign-up of the token tokenHash while its code is live at
// at, and user.ErrSignupExpired when there is none: never started, its code
// expired, or its account was made. A sign-up whose tries are used up is
// still found: its codes are wrong, not expired (Try).
func (r *PendingSignupRepo) Find(ctx context.Context, tokenHash []byte, at time.Time) (*user.PendingSignup, error) {
	var p user.PendingSignup
	var birthday time.Time
	err := r.pool.QueryRow(ctx, `
		select id, email, password_hash, name, username, bio, birthday from pending_signups
		where token_hash = $1 and expires_at > $2`, tokenHash, at).
		Scan(&p.ID, &p.Email, &p.PasswordHash, &p.Name, &p.Username, &p.Bio, &birthday)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, user.ErrSignupExpired
	}
	if err != nil {
		return nil, err
	}
	p.Birthday = birthday.Format("2006-01-02")
	return &p, nil
}

// Try checks codeHash against the live code of the sign-up id, in one
// statement, and counts a try only when it is wrong, as a reset's Try does:
// the right code never costs one, so making the account can fail or be
// given up and lose nothing, and wrong guesses sent at once still stop at
// maxTries (each waits for the one before and sees its count). right is
// true for the live code; replaced is true, with no try counted either, for
// the code it replaced (only someone who got the earlier email can know it).
// Both are false, and nothing is counted, once the tries are used up: then
// even the right code is just wrong. A code that has expired, and a sign-up
// that is gone (its account was made), answer user.ErrSignupExpired.
func (r *PendingSignupRepo) Try(ctx context.Context, id uuid.UUID, codeHash []byte, at time.Time, maxTries int) (right, replaced bool, err error) {
	err = r.pool.QueryRow(ctx, `
		update pending_signups
		set attempts = attempts + case when code_hash = $2 or previous_code_hash = $2 then 0 else 1 end
		where id = $1 and attempts < $3 and expires_at > $4
		returning code_hash = $2, coalesce(previous_code_hash = $2, false)`,
		id, codeHash, maxTries, at).Scan(&right, &replaced)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, r.whyNoTry(ctx, id, at)
	}
	if err != nil {
		return false, false, err
	}
	return right, replaced && !right, nil
}

// whyNoTry says why Try matched nothing a moment ago: a sign-up whose code is
// still live has used up its tries (nil: the code is simply wrong); one whose
// code expired, or that is gone, is user.ErrSignupExpired.
func (r *PendingSignupRepo) whyNoTry(ctx context.Context, id uuid.UUID, at time.Time) error {
	var live bool
	err := r.pool.QueryRow(ctx, `select expires_at > $2 from pending_signups where id = $1`, id, at).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !live) {
		return user.ErrSignupExpired
	}
	return err
}

// Reissue replaces the live code of the sign-up id with codeHash (live until
// expiresAt), made at at, in one statement, unless the one it has must stand
// or none may follow it. The tries are kept, so asking again never buys more
// guesses, and the code replaced is kept as the previous code, so Try can
// say it was replaced. stored reports whether the new code was kept (and so
// is to be emailed). When it was not, a plain read says why, in the order
// the answers take: a sign-up that is gone or whose code expired, one whose
// tries are used up, and one that has had maxCodes codes are
// user.ErrSignupExpired; one whose code was made after freshSince keeps that
// code with no error (stored false: nothing is emailed, and the code it has
// still finishes it). The last code is not replaced within a minute even on
// the last of maxCodes, so a second tap never hears "expired" while the code
// it just got is live.
func (r *PendingSignupRepo) Reissue(ctx context.Context, id uuid.UUID, codeHash []byte,
	expiresAt, at, freshSince time.Time, maxTries, maxCodes int) (stored bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		update pending_signups
		set previous_code_hash = code_hash, code_hash = $2, expires_at = $3, code_created_at = $4, codes_sent = codes_sent + 1
		where id = $1 and expires_at > $4 and attempts < $6 and code_created_at <= $5 and codes_sent < $7`,
		id, codeHash, expiresAt, at, freshSince, maxTries, maxCodes)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var live, usedUp, fresh, codesLeft bool
	err = r.pool.QueryRow(ctx, `
		select expires_at > $2, attempts >= $3, code_created_at > $4, codes_sent < $5
		from pending_signups where id = $1`, id, at, maxTries, freshSince, maxCodes).
		Scan(&live, &usedUp, &fresh, &codesLeft)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, user.ErrSignupExpired
	}
	if err != nil {
		return false, err
	}
	if !live || usedUp || (!fresh && !codesLeft) {
		return false, user.ErrSignupExpired
	}
	return false, nil
}

// Finish makes the account in, the one the sign-up id was started for, in
// the transaction that spends the sign-up's code, so the account is made
// only if that code is still codeHash, and every sign-up of the email goes
// with it: one request wins, so the right code sent twice at once makes one
// account, and the other request finds no sign-up. Every sign-up of the
// email is held first, in id order, before the account's users row is
// written (session_locks.go), and only those rows are deleted: two sign-ups
// of one email finished at the same moment then wait for each other in turn,
// never in a circle, and the one that waited finds its own row gone. A
// sign-up of the email started after the hold is left alone; it can never
// be finished, since the email is taken, and the sweep takes it.
//
// With nothing changed it answers user.ErrSignupExpired (the sign-up is
// gone: another request made the account, or the sweep took it),
// user.ErrSignupCodeReplaced (a newer code replaced this one meanwhile), or
// the insert's own refusal (insertAccount): user.ErrEmailTaken,
// user.ErrUsernameTaken (someone took it since the start, since a sign-up
// holds neither) or a *user.FieldError.
func (r *PendingSignupRepo) Finish(ctx context.Context, id uuid.UUID, codeHash []byte, in user.NewAccount) (*user.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// A row a new code changed while this waited for it is read as it is
	// now: the locking read returns the newest version.
	rows, err := tx.Query(ctx, `
		select id, code_hash = $2 from pending_signups
		where lower(email) = lower($1::text) order by id for update`, in.Email, codeHash)
	if err != nil {
		return nil, err
	}
	var held []uuid.UUID
	found, ours := false, false
	for rows.Next() {
		var rowID uuid.UUID
		var same bool
		if err := rows.Scan(&rowID, &same); err != nil {
			rows.Close()
			return nil, err
		}
		held = append(held, rowID)
		if rowID == id {
			found, ours = true, same
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch {
	case !found:
		return nil, user.ErrSignupExpired
	case !ours:
		return nil, user.ErrSignupCodeReplaced
	}
	if _, err := tx.Exec(ctx, `delete from pending_signups where id = any($1::uuid[])`, held); err != nil {
		return nil, err
	}
	u, err := insertAccount(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

// signupKeep is user.SignupCapWindow as an interval, for the sweep.
var signupKeep = fmt.Sprintf("interval '%d seconds'", int64(user.SignupCapWindow/time.Second))

// SweepSignups implements purge.SignupStore: it deletes, in one statement,
// every sign-up whose code has expired and that started more than
// user.SignupCapWindow ago, on the database's clock. A younger one stays even
// once its code has expired, since the cap of sign-ups per email counts it.
// Both times were written on the server's clock, which may differ a little
// from the database's; a code lives 30 minutes, and a sign-up must be an
// hour old, so a difference of seconds changes nothing. The rows are taken
// in id order, as Finish takes them, so the two never wait on each other in
// a circle.
func (r *PendingSignupRepo) SweepSignups(ctx context.Context) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		delete from pending_signups where id in (
			select id from pending_signups
			where expires_at <= now() and created_at <= now() - `+signupKeep+`
			order by id for update)`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
