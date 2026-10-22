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
	// AccountVisibility is VisibilityPublic or VisibilityPrivate; the caller
	// validates it with ValidVisibility before it gets here.
	AccountVisibility *string
}

// NewAccount is everything a one-request sign-up stores. Birthday is a
// validated "YYYY-MM-DD" date. Bio is optional (nil stores NULL).
// AccountVisibility is the starting value (see DefaultVisibility); an empty
// string is stored as VisibilityPublic, so no caller can store an empty value.
type NewAccount struct {
	Email             string
	PasswordHash      string
	Birthday          string
	Name              string
	Username          string
	Bio               *string
	AccountVisibility string
}

// Repository is the storage contract for account and profile data. Handler
// plans code against this interface; no handler plan needs to write SQL.
type Repository interface {
	Create(ctx context.Context, email string, passwordHash *string, verified bool, via *VerificationSource) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmailCI(ctx context.Context, email string) (*User, error)
	// GetByUsernameCI finds an account by its username, ignoring case, so the
	// Log in page can take a username as well as an email. It returns
	// ErrNotFound when no account holds that username.
	GetByUsernameCI(ctx context.Context, username string) (*User, error)
	UsernameTaken(ctx context.Context, username string) (bool, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID, via VerificationSource) error
	UpdateProfile(ctx context.Context, id uuid.UUID, p ProfilePatch) (*User, error)
	// CreateComplete inserts a finished, unverified account in ONE statement
	// (every column at once), so any failure stores nothing. Unique
	// violations map to ErrEmailTaken and ErrUsernameTaken; a check
	// violation maps to *FieldError.
	CreateComplete(ctx context.Context, in NewAccount) (*User, error)
}

// PasswordChanger replaces an account's password and its sessions in ONE
// transaction. It is deliberately not part of Repository: only the Postgres
// repository and the httpapi test fake implement it, so no other Repository
// double has to grow a method.
type PasswordChanger interface {
	// ChangePassword stores passwordHash for userID, revokes every active
	// refresh token the account holds, and inserts one new token (id
	// sessionID, the session the caller's new access token names, with
	// newTokenHash, expiresAt, userAgent), all in one transaction: on any
	// error nothing at all has changed. It returns ErrNotFound when the
	// account is gone and ErrAccountDisabled when an admin disabled it.
	ChangePassword(ctx context.Context, userID uuid.UUID, passwordHash string, sessionID uuid.UUID, newTokenHash []byte, expiresAt time.Time, userAgent *string) error
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
	// crash mid-rotation cannot leave two simultaneously valid tokens. The old
	// row's rotated_to names the new one. It returns ErrTokenInvalid when
	// oldID is no longer live.
	Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error)
	// RevokeByHash ends the session tokenHash belongs to: it revokes the
	// token when it is live, and when it was rotated, the live token at the
	// end of its rotated_to chain (refresh after refresh), so an old token
	// still ends the session that grew from it. The walk along the chain is
	// capped: a token more refreshes old than the cap ends nothing. It
	// returns ErrTokenInvalid when nothing live was left to revoke within
	// the cap (an unknown token, one revoked by a log out or a password
	// change, or one too many refreshes old).
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
