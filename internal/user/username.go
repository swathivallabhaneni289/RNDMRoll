// Package user: username.go provides normalization, validation, and
// collision-free suggestion for handles. Every function here is advisory --
// PATTERNS.md is explicit that the live check is a UX convenience and the
// authoritative rejection is the unique-index violation at insert time
// (mapped to ErrUsernameTaken by the repository implementation). Nothing in
// this file may be treated as a guarantee that a save will succeed.
package user

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const (
	usernameMinLength = 3
	usernameMaxLength = 20

	// fallbackUsernameBase is returned by NormalizeUsername when the input
	// strips down to nothing usable (e.g. a display name written entirely
	// in a non-Latin script) -- so a suggestion is never a blank field,
	// per RESEARCH.md's Anti-Patterns guidance.
	fallbackUsernameBase = "user"

	// maxSuggestAttempts bounds how many random candidates SuggestUsername
	// tries at a given suffix width before widening it.
	maxSuggestAttempts = 8

	shortSuffixDigits = 3
	longSuffixDigits  = 6
)

var (
	// usernamePattern is exactly the database check constraint
	// (migrations/0001_init.up.sql, users_username_charset) so a validator
	// looser than the constraint can never turn a clear 400 into an opaque
	// 500 at insert time.
	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

	whitespaceRunPattern      = regexp.MustCompile(`\s+`)
	disallowedCharPattern     = regexp.MustCompile(`[^a-z0-9_]`)
	repeatedUnderscorePattern = regexp.MustCompile(`_+`)
)

// NormalizeUsername lowercases displayName, collapses whitespace runs to a
// single underscore, strips every character outside [a-z0-9_], collapses
// repeated underscores, trims leading/trailing underscores, and truncates
// to usernameMaxLength characters. When the result is shorter than
// usernameMinLength it returns the fallback base instead of an empty or
// too-short string.
func NormalizeUsername(displayName string) string {
	s := strings.ToLower(displayName)
	s = whitespaceRunPattern.ReplaceAllString(s, "_")
	s = disallowedCharPattern.ReplaceAllString(s, "")
	s = repeatedUnderscorePattern.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > usernameMaxLength {
		s = s[:usernameMaxLength]
	}
	if len(s) < usernameMinLength {
		return fallbackUsernameBase
	}
	return s
}

// ErrUsernameInvalid is the fixed error ValidateUsername returns. Its text
// never contains the input, because this endpoint can be called with no
// session and the text may be sent back to the caller or logged.
var ErrUsernameInvalid = errors.New("user: username must be 3-20 characters of lowercase letters, digits, or underscores")

// ValidateUsername enforces exactly the pattern the database check
// constraint enforces.
func ValidateUsername(s string) error {
	if !usernamePattern.MatchString(s) {
		return ErrUsernameInvalid
	}
	return nil
}

// SuggestUsername normalizes displayName and returns it as-is when free.
// Otherwise it appends a random three- or four-digit suffix, retrying up to
// maxSuggestAttempts times before widening to a longer random suffix. The
// suffix is drawn from crypto/rand rather than an incrementing counter --
// PATTERNS.md is explicit that a sequential scheme both leaks roughly how
// many similar handles exist and degrades as a namespace fills.
func SuggestUsername(ctx context.Context, repo Repository, displayName string) (string, error) {
	base := NormalizeUsername(displayName)

	taken, err := repo.UsernameTaken(ctx, base)
	if err != nil {
		return "", fmt.Errorf("user: check username availability: %w", err)
	}
	if !taken {
		return base, nil
	}

	if candidate, ok, err := trySuffixedCandidates(ctx, repo, base, maxSuggestAttempts); err != nil {
		return "", err
	} else if ok {
		return candidate, nil
	}

	if candidate, ok, err := trySuffixedCandidatesWithDigits(ctx, repo, base, longSuffixDigits, maxSuggestAttempts); err != nil {
		return "", err
	} else if ok {
		return candidate, nil
	}

	return "", fmt.Errorf("user: no available username found for base %q", base)
}

// SuggestAlternates returns n distinct free candidates derived from base,
// used to populate the taken-state suggestion chips the UI-SPEC username
// state machine describes and the PATCH /me 409 conflict body.
func SuggestAlternates(ctx context.Context, repo Repository, base string, n int) ([]string, error) {
	seen := make(map[string]struct{}, n)
	alternates := make([]string, 0, n)
	maxAttempts := n * maxSuggestAttempts * 2

	for attempt := 0; len(alternates) < n && attempt < maxAttempts; attempt++ {
		digits := shortSuffixDigits + attempt%2
		suffix, err := randomDigitString(digits)
		if err != nil {
			return nil, fmt.Errorf("user: generate suggestion suffix: %w", err)
		}
		candidate := suffixedCandidate(base, suffix)
		if _, dup := seen[candidate]; dup {
			continue
		}
		taken, err := repo.UsernameTaken(ctx, candidate)
		if err != nil {
			return nil, fmt.Errorf("user: check username availability: %w", err)
		}
		if taken {
			continue
		}
		seen[candidate] = struct{}{}
		alternates = append(alternates, candidate)
	}

	if len(alternates) < n {
		return nil, fmt.Errorf("user: could not find %d available alternates for base %q", n, base)
	}
	return alternates, nil
}

// trySuffixedCandidates tries attempts random three- or four-digit suffixes
// (alternating), returning the first that is free.
func trySuffixedCandidates(ctx context.Context, repo Repository, base string, attempts int) (string, bool, error) {
	for i := 0; i < attempts; i++ {
		digits := shortSuffixDigits + i%2
		suffix, err := randomDigitString(digits)
		if err != nil {
			return "", false, fmt.Errorf("user: generate suggestion suffix: %w", err)
		}
		candidate := suffixedCandidate(base, suffix)
		taken, err := repo.UsernameTaken(ctx, candidate)
		if err != nil {
			return "", false, fmt.Errorf("user: check username availability: %w", err)
		}
		if !taken {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// trySuffixedCandidatesWithDigits is trySuffixedCandidates with a fixed
// suffix width, used for the widened retry pass.
func trySuffixedCandidatesWithDigits(ctx context.Context, repo Repository, base string, digits, attempts int) (string, bool, error) {
	for i := 0; i < attempts; i++ {
		suffix, err := randomDigitString(digits)
		if err != nil {
			return "", false, fmt.Errorf("user: generate suggestion suffix: %w", err)
		}
		candidate := suffixedCandidate(base, suffix)
		taken, err := repo.UsernameTaken(ctx, candidate)
		if err != nil {
			return "", false, fmt.Errorf("user: check username availability: %w", err)
		}
		if !taken {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// suffixedCandidate appends suffix to base, truncating base if needed so
// the total never exceeds usernameMaxLength.
func suffixedCandidate(base, suffix string) string {
	maxBaseLen := usernameMaxLength - len(suffix)
	if maxBaseLen < usernameMinLength {
		maxBaseLen = usernameMinLength
	}
	if len(base) > maxBaseLen {
		base = base[:maxBaseLen]
	}
	return base + suffix
}

// randomDigitString returns a crypto/rand-derived decimal string of exactly
// n digits (zero-padded).
func randomDigitString(n int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", n, v), nil
}
