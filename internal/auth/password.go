// Package auth implements the security primitives every auth endpoint
// depends on: password hashing, access-token issue/parse, and refresh-token
// issue/rotation. Handler plans build on these directly rather than
// re-deriving any of them.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// maxPasswordBytes is bcrypt's hard algorithmic ceiling, measured in bytes
// rather than characters -- a multi-byte UTF-8 password hits it sooner than
// its visible length suggests. RESEARCH.md Pitfall 1: the correct response
// is a clear validation error, not a pre-hash workaround such as hashing
// with SHA-256 before bcrypt.
const maxPasswordBytes = 72

// ErrPasswordTooLong is returned by HashPassword when plain exceeds
// bcrypt's 72-byte ceiling.
var ErrPasswordTooLong = errors.New("auth: password exceeds 72 bytes")

// HashPassword returns a bcrypt hash of plain at bcrypt.DefaultCost. It
// rejects input over 72 bytes before ever calling bcrypt, since bcrypt
// itself would otherwise silently ignore bytes past that ceiling.
func HashPassword(plain string) (string, error) {
	if len(plain) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword reports whether plain matches hash. Any mismatch --
// wrong password, or an empty/malformed stored hash (the social-only
// account case, where no password was ever set) -- maps to the single
// user.ErrInvalidCredentials, so a login attempt against a passwordless
// account is indistinguishable from a wrong password. Returning a distinct
// error for either case would let the login endpoint be used to enumerate
// which accounts have a password set.
func ComparePassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return user.ErrInvalidCredentials
	}
	return nil
}
