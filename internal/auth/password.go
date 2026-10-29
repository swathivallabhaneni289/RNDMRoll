// Package auth implements the security primitives every auth endpoint
// depends on: password hashing, access-token issue/parse, and refresh-token
// issue/rotation. Handler plans build on these directly rather than
// re-deriving any of them.
package auth

import (
	"errors"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// MaxPasswordBytes is bcrypt's hard algorithmic ceiling, measured in bytes
// rather than characters -- a multi-byte UTF-8 password hits it sooner than
// its visible length suggests. RESEARCH.md Pitfall 1: the correct response
// is a clear validation error, not a pre-hash workaround such as hashing
// with SHA-256 before bcrypt. Forgot password checks it before a code try is
// counted.
const MaxPasswordBytes = 72

// ErrPasswordTooLong is returned by HashPassword when plain exceeds
// bcrypt's 72-byte ceiling.
var ErrPasswordTooLong = errors.New("auth: password exceeds 72 bytes")

// The length of a new password, in characters. HashPassword then holds
// bcrypt's ceiling, which counts bytes.
const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

// PasswordReason checks a new password: required, 8 to 72 characters, with at
// least one capital letter, one number and one symbol. It returns "" when the
// password is fine, else the reason the API answers with: "required",
// "too_short", "too_long" or "too_weak". Sign-up, change password, Forgot
// password and `make password` (cmd/set-password) follow this one rule; log
// in never checks it, so older, simpler passwords still work.
func PasswordReason(p string) string {
	n := utf8.RuneCountInString(p)
	switch {
	case p == "":
		return "required"
	case n < minPasswordLength:
		return "too_short"
	case n > maxPasswordLength:
		return "too_long"
	}
	var upper, digit, symbol bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbol = true
		}
	}
	if !upper || !digit || !symbol {
		return "too_weak"
	}
	return ""
}

// HashPassword returns a bcrypt hash of plain at bcrypt.DefaultCost. It
// rejects input over 72 bytes before ever calling bcrypt, since bcrypt
// itself would otherwise silently ignore bytes past that ceiling.
func HashPassword(plain string) (string, error) {
	if len(plain) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword reports whether plain matches hash. Any mismatch --
// wrong password, or an empty/malformed stored hash (an account with no
// password set) -- maps to the single
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
