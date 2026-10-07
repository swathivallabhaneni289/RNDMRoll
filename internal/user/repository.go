package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProfilePatch describes a partial update to a user's profile fields. A nil
// field means "leave unchanged".
type ProfilePatch struct {
	Name      *string
	Username  *string
	Bio       *string
	AvatarURL *string
	// Birthday is a validated "YYYY-MM-DD" date. It is written only while no
	// birthday is on file, so the first write wins and a later one is a
	// no-op at the SQL level.
	Birthday *string
}

// NewAccount is everything a one-request sign-up stores. Birthday is a
// validated "YYYY-MM-DD" date. Bio is optional (nil stores NULL).
type NewAccount struct {
	Email        string
	PasswordHash string
	Birthday     string
	Name         string
	Username     string
	Bio          *string
}

// Repository is the storage contract for account and profile data. Handler
// plans code against this interface; no handler plan needs to write SQL.
type Repository interface {
	Create(ctx context.Context, email string, passwordHash *string, verified bool, via *VerificationSource) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmailCI(ctx context.Context, email string) (*User, error)
	GetByProviderSubject(ctx context.Context, provider VerificationSource, subject string) (*User, error)
	LinkProviderSubject(ctx context.Context, id uuid.UUID, provider VerificationSource, subject string) error
	UsernameTaken(ctx context.Context, username string) (bool, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID, via VerificationSource) error
	UpdateProfile(ctx context.Context, id uuid.UUID, p ProfilePatch) (*User, error)
	// CreateComplete inserts a finished, unverified account in ONE statement
	// (every column at once), so any failure stores nothing. Unique
	// violations map to ErrEmailTaken and ErrUsernameTaken; a check
	// violation maps to *FieldError.
	CreateComplete(ctx context.Context, in NewAccount) (*User, error)
	// Delete removes the account only while it has no birthday on file (the
	// under-13 refusal of a social account); its refresh tokens cascade. It
	// reports whether a row was deleted.
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	// ClaimAndRevoke is for a provider that has just proved ownership of the
	// account's address. In ONE transaction, and only while the account is
	// still unverified, it marks the email verified via the provider,
	// discards the password credential and revokes every refresh token,
	// since whoever pre-registered the address with a password never proved
	// they own it. It reports whether it claimed the account; false means
	// the account was already verified (or does not exist) and nothing was
	// touched.
	ClaimAndRevoke(ctx context.Context, id uuid.UUID, via VerificationSource) (bool, error)
}

// RefreshToken is the domain representation of a refresh_tokens row.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// RefreshTokenRepository is the storage contract for refresh-token issuance,
// lookup, rotation, and revocation.
type RefreshTokenRepository interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error)
	// GetActiveByHash returns ErrTokenExpired for a past expires_at and
	// ErrTokenInvalid for a revoked or missing row.
	GetActiveByHash(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
	// Rotate revokes oldID and inserts a new row in one transaction, so a
	// crash mid-rotation cannot leave two simultaneously valid tokens.
	Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error)
	RevokeByHash(ctx context.Context, tokenHash []byte) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// EmailVerificationRepository is the storage contract for the
// email-verification token flow.
type EmailVerificationRepository interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	// ConsumeByHash atomically sets consumed_at and returns the owning
	// user ID, or ErrTokenConsumed / ErrTokenExpired / ErrTokenInvalid.
	ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error)
	// DeleteForUser is used when a resend supersedes outstanding tokens.
	DeleteForUser(ctx context.Context, userID uuid.UUID) error
}
