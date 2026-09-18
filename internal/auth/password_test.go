package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

func TestPassword_HashDoesNotEqualPlaintext(t *testing.T) {
	plain := "correct horse battery staple"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == plain {
		t.Fatal("hash must not equal the plaintext input")
	}
}

func TestPassword_HashThenCompareAccepts(t *testing.T) {
	plain := "correct horse battery staple"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := ComparePassword(hash, plain); err != nil {
		t.Fatalf("ComparePassword rejected the correct password: %v", err)
	}
}

func TestPassword_CompareRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	err = ComparePassword(hash, "wrong password")
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestPassword_CompareTreatsMissingHashSameAsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	wrongPasswordErr := ComparePassword(hash, "wrong password")
	// An empty/malformed stored hash is the social-only account case: a
	// login attempt against it must be indistinguishable from a wrong
	// password, never a distinct "no password set" error.
	noPasswordErr := ComparePassword("", "any password")
	if !errors.Is(wrongPasswordErr, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got %v", wrongPasswordErr)
	}
	if !errors.Is(noPasswordErr, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for missing hash, got %v", noPasswordErr)
	}
}

func TestPassword_HashRejectsPasswordLongerThan72Bytes(t *testing.T) {
	tooLong := strings.Repeat("a", 73)
	_, err := HashPassword(tooLong)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestPassword_HashAccepts72ByteBoundary(t *testing.T) {
	exact := strings.Repeat("a", 72)
	if _, err := HashPassword(exact); err != nil {
		t.Fatalf("72-byte password should be accepted, got error: %v", err)
	}
}
