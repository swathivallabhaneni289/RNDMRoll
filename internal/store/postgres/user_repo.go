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

// scanUser scans a single row shaped like the fixed column list every query
// below selects/returns: id, email, password_hash, name, username, bio,
// avatar_url, email_verified, email_verified_via, apple_subject,
// google_subject, created_at, updated_at. The caller decides how to
// interpret a returned error (pgx.ErrNoRows means different things to
// different callers: ErrNotFound for a lookup, a no-op for an update that
// matched nothing).
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var via *string
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Username, &u.Bio, &u.AvatarURL,
		&u.EmailVerified, &via, &u.AppleSubject, &u.GoogleSubject, &u.CreatedAt, &u.UpdatedAt,
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
// RESEARCH.md Pitfall 4) — it returns nil for any other kind of error,
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

func (r *UserRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *user.VerificationSource) (*user.User, error) {
	var viaStr *string
	if via != nil {
		s := string(*via)
		viaStr = &s
	}
	const q = `
		insert into users (email, password_hash, email_verified, email_verified_via)
		values ($1::text, $2::text, $3, $4::text)
		returning id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, email, passwordHash, verified, viaStr)
	u, err := scanUser(row)
	if err != nil {
		if mapped := mapUniqueViolation(err); mapped != nil {
			return nil, mapped
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	const q = `select id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at from users where id = $1`
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
	const q = `select id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at from users where lower(email) = lower($1::text)`
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
		q = `select id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at from users where apple_subject = $1::text`
	case user.VerifiedViaGoogle:
		q = `select id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at from users where google_subject = $1::text`
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

// UpdateProfile applies a partial update via coalesce($n::text, column) so
// even the partial-update path stays a single fixed query string — no
// dynamically built SET list ever varies with input.
func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	const q = `
		update users
		set name = coalesce($1::text, name),
		    username = coalesce($2::text, username),
		    bio = coalesce($3::text, bio),
		    avatar_url = coalesce($4::text, avatar_url)
		where id = $5
		returning id, email, password_hash, name, username, bio, avatar_url, email_verified, email_verified_via, apple_subject, google_subject, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, p.Name, p.Username, p.Bio, p.AvatarURL, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		if mapped := mapUniqueViolation(err); mapped != nil {
			return nil, mapped
		}
		return nil, err
	}
	return u, nil
}
