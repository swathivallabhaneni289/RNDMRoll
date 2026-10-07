package postgres

import (
	"context"
	"errors"
	"fmt"

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

// NewUserRepo builds a UserRepo backed by pool.
func NewUserRepo(pool *pgxpool.Pool) user.Repository {
	return &UserRepo{pool: pool}
}

// userColumns is the one column list every query selects or returns, in the
// order scanUser reads it. It selects whether a birthday is on file, never
// the date itself, so no code path can return or log a birthday.
const userColumns = `id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, (birthday is not null) as has_birthday, created_at, updated_at`

// scanUser scans a single row shaped like userColumns. The caller decides how to
// interpret a returned error (pgx.ErrNoRows means different things to
// different callers: ErrNotFound for a lookup, a no-op for an update that
// matched nothing).
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var via *string
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Username, &u.Bio, &u.AvatarURL,
		&u.EmailVerified, &via, &u.AppleSubject, &u.GoogleSubject, &u.HasBirthday, &u.CreatedAt, &u.UpdatedAt,
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
// one of the users table's four unique indexes to its typed domain error.
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
	case "users_apple_subject_idx", "users_google_subject_idx":
		return user.ErrSubjectLinkedToOtherAccount
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
// check.
func (r *UserRepo) CreateComplete(ctx context.Context, in user.NewAccount) (*user.User, error) {
	const q = `
		insert into users (email, password_hash, name, username, bio, birthday, email_verified, email_verified_via)
		values ($1::text, $2::text, $3::text, $4::text, $5::text, $6::date, false, null)
		returning ` + userColumns + `
	`
	row := r.pool.QueryRow(ctx, q, in.Email, in.PasswordHash, in.Name, in.Username, in.Bio, in.Birthday)
	u, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return u, nil
}

// Delete removes an account that has no birthday on file. The guard means a
// real, age-checked account can never be deleted through this path; refresh
// tokens and verification tokens go with it (on delete cascade).
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	const q = `delete from users where id = $1 and birthday is null`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
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

func (r *UserRepo) GetByProviderSubject(ctx context.Context, provider user.VerificationSource, subject string) (*user.User, error) {
	var q string
	switch provider {
	case user.VerifiedViaApple:
		q = `select ` + userColumns + ` from users where apple_subject = $1::text`
	case user.VerifiedViaGoogle:
		q = `select ` + userColumns + ` from users where google_subject = $1::text`
	default:
		return nil, fmt.Errorf("postgres: unsupported provider %q", provider)
	}
	u, err := scanUser(r.pool.QueryRow(ctx, q, subject))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider user.VerificationSource, subject string) error {
	var q string
	switch provider {
	case user.VerifiedViaApple:
		q = `update users set apple_subject = $1::text where id = $2`
	case user.VerifiedViaGoogle:
		q = `update users set google_subject = $1::text where id = $2`
	default:
		return fmt.Errorf("postgres: unsupported provider %q", provider)
	}
	tag, err := r.pool.Exec(ctx, q, subject, id)
	if err != nil {
		if mapped := mapUniqueViolation(err); mapped != nil {
			return mapped
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}
	return nil
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

// ClaimAndRevoke verifies the email, drops the password and revokes every
// refresh token in one transaction. The update is guarded by
// email_verified = false, so a verified account is never touched and its
// sessions survive. password_hash is nullable (social-only accounts already
// store NULL), and auth.ComparePassword rejects an empty hash, so NULL is
// the unusable credential and no placeholder hash is needed.
func (r *UserRepo) ClaimAndRevoke(ctx context.Context, id uuid.UUID, via user.VerificationSource) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	const claimQ = `update users set email_verified = true, email_verified_via = $1::text, password_hash = null where id = $2 and email_verified = false`
	tag, err := tx.Exec(ctx, claimQ, string(via), id)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	const revokeQ = `update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null`
	if _, err := tx.Exec(ctx, revokeQ, id); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateProfile applies a partial update via coalesce($n::text, column) so
// even the partial-update path stays a single fixed query string, so no
// dynamically built SET list ever varies with input. The birthday is
// write-once: coalesce(birthday, $6::date) keeps a value already on file, so
// two racing patches cannot overwrite it.
func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	const q = `
		update users
		set name = coalesce($1::text, name),
		    username = coalesce($2::text, username),
		    bio = coalesce($3::text, bio),
		    avatar_url = coalesce($4::text, avatar_url),
		    birthday = coalesce(birthday, $6::date)
		where id = $5
		returning ` + userColumns + `
	`
	row := r.pool.QueryRow(ctx, q, p.Name, p.Username, p.Bio, p.AvatarURL, id, p.Birthday)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, mapWriteError(err)
	}
	return u, nil
}
