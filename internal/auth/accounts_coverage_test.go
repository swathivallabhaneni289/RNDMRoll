package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// accountsExpiry reads the stored expiry of the token with id.
func accountsExpiry(t *testing.T, repo *fakeRefreshRepo, id uuid.UUID) time.Time {
	t.Helper()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	row, ok := repo.rows[id]
	if !ok {
		t.Fatalf("no stored token %s", id)
	}
	return row.expiresAt
}

// accountsWithin fails unless got is ttl after a moment between from and to.
func accountsWithin(t *testing.T, label string, got, from, to time.Time, ttl time.Duration) {
	t.Helper()
	if got.Before(from.Add(ttl)) || got.After(to.Add(ttl)) {
		t.Fatalf("%s: expires %v, want %v after a moment between %v and %v", label, got, ttl, from, to)
	}
}

// SES-01: the refresh token lasts 30 days from its last use. A log in's token
// gets 30 days, and each refresh gives the new token a fresh 30 days from now,
// however little the old one had left, so someone who opens the app now and
// then stays logged in.
func TestAccounts_EachRefreshGivesTheNewTokenAFresh30Days(t *testing.T) {
	const ttl = 30 * 24 * time.Hour
	ctx := context.Background()
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, ttl)
	userID := uuid.New()

	before := time.Now()
	raw, id, err := svc.Issue(ctx, userID, nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	accountsWithin(t, "the issued token", accountsExpiry(t, repo, id), before, time.Now(), ttl)

	for round := 1; round <= 2; round++ {
		// As if 29 days and 23 hours had passed: an hour left.
		repo.mu.Lock()
		repo.rows[id].expiresAt = time.Now().Add(time.Hour)
		repo.mu.Unlock()
		old := accountsExpiry(t, repo, id)

		before = time.Now()
		gotUser, next, nextID, err := svc.Redeem(ctx, raw, nil)
		if err != nil {
			t.Fatalf("round %d: Redeem: %v", round, err)
		}
		if gotUser != userID || nextID == id || next == raw {
			t.Fatalf("round %d: Redeem gave user %s, id %s, a new token %t", round, gotUser, nextID, next != raw)
		}
		fresh := accountsExpiry(t, repo, nextID)
		accountsWithin(t, "the refreshed token", fresh, before, time.Now(), ttl)
		if !fresh.After(old) {
			t.Fatalf("round %d: the new token ends %v, no later than the old one's %v", round, fresh, old)
		}
		if repo.live(id) {
			t.Fatalf("round %d: the old token is still live after the refresh", round)
		}
		raw, id = next, nextID
	}
}

// accountsSigned signs claims for userID, expiring in 15 minutes, with method
// and secret.
func accountsSigned(t *testing.T, method jwt.SigningMethod, userID uuid.UUID, secret []byte) string {
	t.Helper()
	now := time.Now()
	claims := accessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
	}, SessionID: uuid.NewString()}
	signed, err := jwt.NewWithClaims(method, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign with %s: %v", method.Alg(), err)
	}
	return signed
}

// Section 10 Security: the algorithm is pinned. A token signed with the right
// secret but another HMAC algorithm is refused, so only HS256 is ever read.
func TestAccounts_ParseRefusesOtherHMACAlgorithms(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID := uuid.New()

	if subject, _, err := ParseSessionAccessToken(accountsSigned(t, jwt.SigningMethodHS256, userID, secret), secret); err != nil || subject != userID {
		t.Fatalf("an HS256 token: subject %s, error %v; want %s and no error", subject, err, userID)
	}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		token := accountsSigned(t, method, userID, secret)
		if _, _, err := ParseSessionAccessToken(token, secret); !errors.Is(err, user.ErrTokenInvalid) {
			t.Fatalf("an %s token with the right secret: error %v, want ErrTokenInvalid", method.Alg(), err)
		}
		if _, err := ParseAccessToken(token, secret); !errors.Is(err, user.ErrTokenInvalid) {
			t.Fatalf("ParseAccessToken took an %s token: %v", method.Alg(), err)
		}
	}
}
