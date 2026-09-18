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
// password-signup flow's own verification email, or implicitly by an
// already-verifying OAuth provider.
type VerificationSource string

const (
	VerifiedViaPasswordFlow VerificationSource = "password_flow"
	VerifiedViaApple        VerificationSource = "apple"
	VerifiedViaGoogle       VerificationSource = "google"
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
	AppleSubject     *string
	GoogleSubject    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// OnboardingComplete reports whether the user has finished the onboarding
// flow: their email is verified and both a name and a username are set.
// This is the sole authority for the D-05 step-6 "land in the app"
// condition — the API exposes it and the mobile root layout routes on it.
func (u *User) OnboardingComplete() bool {
	if !u.EmailVerified {
		return false
	}
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
	ErrNotFound                    = errors.New("user: not found")
	ErrEmailTaken                  = errors.New("user: email already registered")
	ErrUsernameTaken               = errors.New("user: username already taken")
	ErrInvalidCredentials          = errors.New("user: invalid credentials")
	ErrEmailNotVerified            = errors.New("user: email not verified")
	ErrTokenInvalid                = errors.New("user: token invalid")
	ErrTokenExpired                = errors.New("user: token expired")
	ErrTokenConsumed               = errors.New("user: token already consumed")
	ErrSubjectLinkedToOtherAccount = errors.New("user: provider subject already linked to another account")
)
