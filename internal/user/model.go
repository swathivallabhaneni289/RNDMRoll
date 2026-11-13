// Package user defines the domain model, typed errors, and repository
// contracts for accounts. Handler plans code against these interfaces
// without needing to read any storage implementation.
package user

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// VerificationSource records how a user's email was verified: through the
// old password-signup verification email (no longer sent; kept so existing
// rows and the check constraint stay valid).
type VerificationSource string

const (
	VerifiedViaPasswordFlow VerificationSource = "password_flow"
)

// User is the domain representation of an account row. Nullable columns are
// pointers so "not set yet" (nil) is distinguishable from "set to empty",
// which the onboarding router depends on to decide what step comes next.
// AvatarKey is the storage key of the profile photo (avatars/{id}/{16
// hex}.{ext}, migration 0019), nil for none: it is never sent, the HTTP layer
// signs a link from it.
type User struct {
	ID               uuid.UUID
	Email            string
	PasswordHash     *string
	Name             *string
	Username         *string
	Bio              *string
	AvatarKey        *string
	EmailVerified    bool
	EmailVerifiedVia *VerificationSource
	// HasBirthday reports whether a birthday is on file. The date itself is
	// deliberately not part of this struct: no query selects it, so no code
	// path can return or log it.
	HasBirthday bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// AccountVisibility is VisibilityPublic or VisibilityPrivate. It is stored
	// and shown only; the Friends view and Global (Phases 5 and 6) read the
	// author's value.
	AccountVisibility string
	// Disabled: an admin disabled the account (migration 0012). Login,
	// refresh and every signed-in request refuse it with account_disabled.
	Disabled bool
	// DeletionRequested: the person asked for the account to be deleted
	// (migration 0018). Like a disabled account it is gone for everyone
	// else, and refresh and every signed-in request refuse it with
	// account_disabled. Log in or a Forgot password reset inside
	// DeletionGracePeriod cancels the ask.
	DeletionRequested bool
	// DeletionDue: that grace period is over, on the database's clock when
	// the row was read. The account is as good as gone: the purge may delete
	// it at any moment, and log in and Forgot password answer exactly as for
	// an email with no account.
	DeletionDue bool
}

// OnboardingComplete reports whether the account has a name and a username.
// Email verification is not part of it: sign-up is one request with no email
// check, so an unverified account is complete once those two are set.
// This is the sole authority for the "land in the app" condition: the API
// exposes it and the mobile root layout routes on it.
func (u *User) OnboardingComplete() bool {
	if u.Name == nil || *u.Name == "" {
		return false
	}
	if u.Username == nil || *u.Username == "" {
		return false
	}
	return true
}

// Sentinel domain errors. ErrInvalidCredentials is deliberately the only
// error for a failed login: returning a distinct "no such user" error would
// let the login endpoint be used to enumerate registered emails.
var (
	ErrNotFound           = errors.New("user: not found")
	ErrEmailTaken         = errors.New("user: email already registered")
	ErrUsernameTaken      = errors.New("user: username already taken")
	ErrInvalidCredentials = errors.New("user: invalid credentials")
	ErrEmailNotVerified   = errors.New("user: email not verified")
	ErrTokenInvalid       = errors.New("user: token invalid")
	ErrTokenExpired       = errors.New("user: token expired")
	ErrTokenConsumed      = errors.New("user: token already consumed")
	// ErrAccountDisabled: an admin disabled the account, so its password is
	// not changed (Change password and Forgot password both refuse it). An
	// account whose owner asked for it to be deleted is refused the same way
	// by Change password and by asking again; Forgot password refuses it
	// only once the grace period is over (inside it, a reset keeps the
	// account).
	ErrAccountDisabled = errors.New("user: account disabled")
	// ErrResetCodeReplaced: the Forgot password code was right, but a newer
	// code replaced it before the password could change. ErrResetCodeUsed:
	// the code was right, but another request used it first. Only someone who
	// got the code can meet either.
	ErrResetCodeReplaced = errors.New("user: reset code replaced")
	ErrResetCodeUsed     = errors.New("user: reset code already used")
)

// FieldError reports that a stored value broke a database check constraint,
// naming the request field it came from. Its text is fixed and never carries
// database text or the offending value.
type FieldError struct {
	Field string
}

func (e *FieldError) Error() string { return "user: invalid " + e.Field }
