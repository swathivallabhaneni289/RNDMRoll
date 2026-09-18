package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// fakeRefreshRow is an in-memory stand-in for a refresh_tokens row.
type fakeRefreshRow struct {
	id        uuid.UUID
	userID    uuid.UUID
	tokenHash string
	expiresAt time.Time
	revokedAt *time.Time
}

// fakeRefreshRepo implements user.RefreshTokenRepository entirely in
// memory, so this suite runs with no TEST_DATABASE_URL. Plan 01-03's
// Postgres suite already covers the real implementation's transactional
// behavior; this fake only needs to reproduce its documented contract.
type fakeRefreshRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*fakeRefreshRow
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{rows: make(map[uuid.UUID]*fakeRefreshRow)}
}

func (f *fakeRefreshRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.New()
	f.rows[id] = &fakeRefreshRow{id: id, userID: userID, tokenHash: string(tokenHash), expiresAt: expiresAt}
	return id, nil
}

func (f *fakeRefreshRepo) GetActiveByHash(ctx context.Context, tokenHash []byte) (*user.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.tokenHash != string(tokenHash) {
			continue
		}
		if row.revokedAt != nil {
			return nil, user.ErrTokenInvalid
		}
		if time.Now().After(row.expiresAt) {
			return nil, user.ErrTokenExpired
		}
		return &user.RefreshToken{ID: row.id, UserID: row.userID, ExpiresAt: row.expiresAt, RevokedAt: row.revokedAt}, nil
	}
	return nil, user.ErrTokenInvalid
}

func (f *fakeRefreshRepo) Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.rows[oldID]
	if !ok {
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	newID := uuid.New()
	f.rows[newID] = &fakeRefreshRow{id: newID, userID: old.userID, tokenHash: string(newHash), expiresAt: expiresAt}
	now := time.Now()
	old.revokedAt = &now
	return newID, nil
}

func (f *fakeRefreshRepo) RevokeByHash(ctx context.Context, tokenHash []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.tokenHash == string(tokenHash) && row.revokedAt == nil {
			now := time.Now()
			row.revokedAt = &now
			return nil
		}
	}
	return user.ErrTokenInvalid
}

func (f *fakeRefreshRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, row := range f.rows {
		if row.userID == userID && row.revokedAt == nil {
			row.revokedAt = &now
		}
	}
	return nil
}

var _ user.RefreshTokenRepository = (*fakeRefreshRepo)(nil)

func TestRefresh_IssueDoesNotPersistTheRawToken(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)
	userID := uuid.New()

	raw, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	for _, row := range repo.rows {
		if row.tokenHash == raw {
			t.Fatal("stored value must be a digest, never the raw token")
		}
	}
}

func TestRefresh_RedeemAcceptsFreshTokenOnce(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)
	userID := uuid.New()

	raw, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	gotUserID, newToken, err := svc.Redeem(context.Background(), raw, nil)
	if err != nil {
		t.Fatalf("Redeem rejected a freshly issued token: %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("expected user %s, got %s", userID, gotUserID)
	}
	if newToken == raw {
		t.Fatal("rotation must return a different token than the one presented")
	}
}

func TestRefresh_RedeemOfSameTokenTwiceFails(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)
	userID := uuid.New()

	raw, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, _, err := svc.Redeem(context.Background(), raw, nil); err != nil {
		t.Fatalf("first Redeem should succeed: %v", err)
	}
	if _, _, err := svc.Redeem(context.Background(), raw, nil); err == nil {
		t.Fatal("second Redeem of the same token must fail: rotation should have revoked it")
	}
}

func TestRefresh_RedeemOfExpiredTokenReturnsExpired(t *testing.T) {
	repo := newFakeRefreshRepo()
	// A negative TTL means the token is already expired the instant it is issued.
	svc := NewRefreshService(repo, -1*time.Minute)
	userID := uuid.New()

	raw, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	_, _, err = svc.Redeem(context.Background(), raw, nil)
	if !errors.Is(err, user.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestRefresh_RedeemOfRevokedTokenReturnsInvalid(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)
	userID := uuid.New()

	raw, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if err := svc.Revoke(context.Background(), raw); err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}

	_, _, err = svc.Redeem(context.Background(), raw, nil)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestRefresh_IssueProducesDistinctTokens(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)
	userID := uuid.New()

	first, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	second, err := svc.Issue(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if first == second {
		t.Fatal("two tokens issued back to back must be distinct")
	}
}

func TestRefresh_RevokeOfAlreadyInvalidTokenStillSucceeds(t *testing.T) {
	repo := newFakeRefreshRepo()
	svc := NewRefreshService(repo, 30*24*time.Hour)

	if err := svc.Revoke(context.Background(), "not-a-real-token"); err != nil {
		t.Fatalf("Revoke of an unknown/malformed token must tolerate the miss, got: %v", err)
	}
}
