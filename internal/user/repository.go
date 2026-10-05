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
	// ClaimUnverifiedEmail is for a provider that has just proved ownership
	// of the account's address. Only while the account is still unverified,
	// it marks the email verified via the provider and discards the password
	// credential, since whoever pre-registered the address with a password
	// never proved they own it. It reports whether it changed a row; false
	// means the account was already verified (or does not exist) and nothing
	// was touched.
	ClaimUnverifiedEmail(ctx context.Context, id uuid.UUID, via VerificationSource) (bool, error)
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
