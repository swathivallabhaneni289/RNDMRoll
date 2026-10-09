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
type User struct {
	ID               uuid.UUID
	Email            string
	PasswordHash     *string
	Name             *string
	Username         *string
	Bio              *string
	AvatarURL        *string
	EmailVerified    bool
	EmailVerifiedVia *VerificationSource
	// HasBirthday reports whether a birthday is on file. The date itself is
	// deliberately not part of this struct: no query selects it, so no code
	// path can return or log it.
	HasBirthday bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
)

// FieldError reports that a stored value broke a database check constraint,
// naming the request field it came from. Its text is fixed and never carries
// database text or the offending value.
type FieldError struct {
	Field string
}

func (e *FieldError) Error() string { return "user: invalid " + e.Field }
