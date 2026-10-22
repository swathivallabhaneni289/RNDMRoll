package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// UserRepo is a pgx-backed implementation of user.Repository.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo builds a UserRepo backed by pool. It returns the concrete type
// so one value can be handed out as a user.Repository and, where a handler
// needs the password change, as a user.PasswordChanger.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

var (
	_ user.Repository      = (*UserRepo)(nil)
	_ user.PasswordChanger = (*UserRepo)(nil)
)

// userColumns is the one column list every query selects or returns, in the
// order scanUser reads it. It selects whether a birthday is on file, never
// the date itself, so no code path can return or log a birthday.
const userColumns = `id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, (birthday is not null) as has_birthday, created_at, updated_at, account_visibility, (disabled_at is not null) as disabled`

// scanUser scans a single row shaped like userColumns. The caller decides how to
// interpret a returned error (pgx.ErrNoRows means different things to
// different callers: ErrNotFound for a lookup, a no-op for an update that
// matched nothing).
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var via *string
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Username, &u.Bio, &u.AvatarURL,
		&u.EmailVerified, &via, &u.HasBirthday, &u.CreatedAt, &u.UpdatedAt, &u.AccountVisibility, &u.Disabled,
	)
	if err != nil {
		return nil, err
	}
	if via != nil {
		vs := user.VerificationSource(*via)
		u.EmailVerifiedVia = &vs
	}
	return &u, nil
}

// mapUniqueViolation maps a Postgres unique-violation (SQLSTATE 23505) on
// one of the users table's two unique indexes to its typed domain error.
// This insert-time mapping is the authoritative uniqueness decision (see
// RESEARCH.md Pitfall 4), and it returns nil for any other kind of error,
// including no error at all.
func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "users_email_lower_idx":
		return user.ErrEmailTaken
	case "users_username_lower_idx":
		return user.ErrUsernameTaken
	default:
		return err
	}
}

// mapCheckViolation maps a Postgres check violation (SQLSTATE 23514) to a
// *user.FieldError naming the request field, by constraint name. It returns
// nil for any other error. The error text is fixed: database text never
// reaches a client.
func mapCheckViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "users_username_charset":
		return &user.FieldError{Field: "username"}
	case "users_birthday_min":
		return &user.FieldError{Field: "birthday"}
	case "users_bio_check":
		return &user.FieldError{Field: "bio"}
	case "users_account_visibility_check":
		return &user.FieldError{Field: "account_visibility"}
	default:
		return err
	}
}

// mapWriteError applies both violation mappings to an insert or update error.
func mapWriteError(err error) error {
	if mapped := mapUniqueViolation(err); mapped != nil {
		return mapped
	}
	if mapped := mapCheckViolation(err); mapped != nil {
		return mapped
	}
	return err
}

func (r *UserRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *user.VerificationSource) (*user.User, error) {
	var viaStr *string
	if via != nil {
		s := string(*via)
		viaStr = &s
	}
	const q = `
		insert into users (email, password_hash, email_verified, email_verified_via)
		values ($1::text, $2::text, $3, $4::text)
		returning ` + userColumns + `
	`
	row := r.pool.QueryRow(ctx, q, email, passwordHash, verified, viaStr)
	u, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return u, nil
}

// CreateComplete inserts every column of a finished account in a single
// statement, so a failure of any kind (taken email, taken username, a check
// violation) stores nothing. The account is unverified: sign-up has no email
// check. An empty AccountVisibility is stored as 'public'.
func (r *UserRepo) CreateComplete(ctx context.Context, in user.NewAccount) (*user.User, error) {
	visibility := in.AccountVisibility
	if visibility == "" {
		visibility = user.VisibilityPublic
	}
	const q = `
		insert into users (email, password_hash, name, username, bio, birthday, email_verified, email_verified_via, account_visibility)
		values ($1::text, $2::text, $3::text, $4::text, $5::text, $6::date, false, null, $7::text)
		returning ` + userColumns + `
	`
	row := r.pool.QueryRow(ctx, q, in.Email, in.PasswordHash, in.Name, in.Username, in.Bio, in.Birthday, visibility)
	u, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	const q = `select ` + userColumns + ` from users where id = $1`
	u, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByEmailCI(ctx context.Context, email string) (*user.User, error) {
	const q = `select ` + userColumns + ` from users where lower(email) = lower($1::text)`
	u, err := scanUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByUsernameCI(ctx context.Context, username string) (*user.User, error) {
	const q = `select ` + userColumns + ` from users where lower(username) = lower($1::text)`
	u, err := scanUser(r.pool.QueryRow(ctx, q, username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	const q = `select exists(select 1 from users where lower(username) = lower($1::text))`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, username).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *UserRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, via user.VerificationSource) error {
	const q = `update users set email_verified = true, email_verified_via = $1::text where id = $2`
	tag, err := r.pool.Exec(ctx, q, string(via), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}
	return nil
}

// UpdateProfile applies a partial update via coalesce($n::text, column) so
// even the partial-update path stays a single fixed query string, so no
// dynamically built SET list ever varies with input. The birthday is not part
// of it: it is set once, at sign-up, and never changes.
func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	const q = `
		update users
		set name = coalesce($1::text, name),
		    username = coalesce($2::text, username),
		    bio = coalesce($3::text, bio),
		    avatar_url = coalesce($4::text, avatar_url),
		    account_visibility = coalesce($5::text, account_visibility)
		where id = $6
		returning ` + userColumns + `
	`
	row := r.pool.QueryRow(ctx, q, p.Name, p.Username, p.Bio, p.AvatarURL, p.AccountVisibility, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, mapWriteError(err)
	}
	return u, nil
}

// ChangePassword replaces the account's password hash and its sessions in ONE
// transaction: the new hash is stored, every active refresh token is revoked,
// and one new token is inserted, so the caller's own session continues and
// every other phone is signed out at its next refresh. Any error rolls the
// whole change back (the old hash and the old tokens stay as they were).
// An account that is gone is user.ErrNotFound and changes nothing.
//
// The revoke runs twice, on purpose. A refresh on another phone that is in
// flight at this moment (it has made its new token and holds the lock on its
// old one) makes the first revoke wait; the new token it commits was not
// visible to that statement, so it would survive the change. The second
// revoke starts after that wait and sees it. A refresh that has not yet
// locked its old token finds it revoked and rolls back by itself.
func (r *UserRepo) ChangePassword(ctx context.Context, userID uuid.UUID, passwordHash string, newTokenHash []byte, expiresAt time.Time, userAgent *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if err := changePassword(ctx, tx, userID, passwordHash, newTokenHash, expiresAt, userAgent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// changePassword is ChangePassword's writes, as steps of the caller's
// transaction tx (ChangePassword says why the revoke runs twice). Forgot
// password runs them in the transaction that spends its code
// (PasswordResetRepo.SpendAndChangePassword), so both changes follow one set
// of rules.
func changePassword(ctx context.Context, tx pgx.Tx, userID uuid.UUID, passwordHash string, newTokenHash []byte, expiresAt time.Time, userAgent *string) error {
	// The users_set_updated_at trigger stamps the row.
	const setHash = `update users set password_hash = $2 where id = $1`
	tag, err := tx.Exec(ctx, setHash, userID, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}

	const revokeAll = `update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null`
	if _, err := tx.Exec(ctx, revokeAll, userID); err != nil {
		return err
	}
	// Every phone the account was signed in on stops getting its pushes; the
	// phone that stays signed in registers its token again on its next start.
	if _, err := tx.Exec(ctx, `delete from push_tokens where user_id = $1`, userID); err != nil {
		return err
	}

	const insertToken = `insert into refresh_tokens (user_id, token_hash, expires_at, user_agent) values ($1, $2, $3, $4::text)`
	if _, err := tx.Exec(ctx, insertToken, userID, newTokenHash, expiresAt, userAgent); err != nil {
		return err
	}

	const revokeOthers = `update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null and token_hash <> $2`
	_, err = tx.Exec(ctx, revokeOthers, userID, newTokenHash)
	return err
}
