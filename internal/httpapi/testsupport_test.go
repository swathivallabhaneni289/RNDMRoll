// Package httpapi_test-support: this file provides the harness every wave 4
// handler plan builds on -- a bare router constructor plus in-memory fakes
// for the plan 01-03 repository interfaces and the plan 01-09 mailer
// interface. Because every wave 4 handler mounts itself on a fresh engine
// inside its own test, no handler test depends on cmd/api/main.go existing.
package httpapi

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// TestDeps bundles the fakes a wave 4 handler test wires into the handler
// constructors it builds on top of newTestRouter. Each field is the
// interface type a real handler depends on, not the concrete fake, so a
// test can swap in a different double without changing the handler.
//
// Mailer is mail.Mailer (the real interface internal/mail.Service
// implements), not a local placeholder. Plan 01-06 originally declared a
// structurally different placeholder here (SendVerificationEmail(ctx,
// toEmail, token string) error) because internal/mail did not exist yet;
// plan 01-09 built the real package with the *user.User-based signature but
// deliberately left this placeholder untouched to avoid a merge conflict
// with concurrent wave-4 worktrees, and flagged the reconciliation for
// plan 01-13. That reconciliation happens here: fakeMailer below now
// implements the real mail.Mailer interface, so every handler test in this
// package (including auth_test.go's) exercises the same contract
// cmd/api/main.go wires against.
type TestDeps struct {
	Users         user.Repository
	RefreshTokens user.RefreshTokenRepository
	Verifications user.EmailVerificationRepository
	Mailer        mail.Mailer
}

// newTestRouter returns a bare gin.Engine with panic recovery attached and
// no routes registered. Each wave 4 handler test mounts its own handler(s)
// on the returned engine -- this harness never assumes a specific route
// shape, which is what keeps the five wave 4 plans independent of each
// other and of cmd/api/main.go.
func newTestRouter(t *testing.T, deps TestDeps) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	return router
}

// --- fakeUserRepo: in-memory user.Repository ---

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]*user.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*user.User)}
}

func (f *fakeUserRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *user.VerificationSource) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return nil, user.ErrEmailTaken
		}
	}
	now := time.Now()
	u := &user.User{
		ID:               uuid.New(),
		Email:            email,
		PasswordHash:     passwordHash,
		EmailVerified:    verified,
		EmailVerifiedVia: via,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	f.users[u.ID] = u
	stored := *u
	return &stored, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	stored := *u
	return &stored, nil
}

func (f *fakeUserRepo) GetByEmailCI(ctx context.Context, email string) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			stored := *u
			return &stored, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeUserRepo) GetByProviderSubject(ctx context.Context, provider user.VerificationSource, subject string) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if providerSubject(u, provider) == subject && subject != "" {
			stored := *u
			return &stored, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeUserRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider user.VerificationSource, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return user.ErrNotFound
	}
	for otherID, other := range f.users {
		if otherID == id {
			continue
		}
		if providerSubject(other, provider) == subject {
			return user.ErrSubjectLinkedToOtherAccount
		}
	}
	switch provider {
	case user.VerifiedViaApple:
		u.AppleSubject = &subject
	case user.VerifiedViaGoogle:
		u.GoogleSubject = &subject
	}
	u.UpdatedAt = time.Now()
	return nil
}

func providerSubject(u *user.User, provider user.VerificationSource) string {
	switch provider {
	case user.VerifiedViaApple:
		if u.AppleSubject != nil {
			return *u.AppleSubject
		}
	case user.VerifiedViaGoogle:
		if u.GoogleSubject != nil {
			return *u.GoogleSubject
		}
	}
	return ""
}

func (f *fakeUserRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Username != nil && strings.EqualFold(*u.Username, username) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUserRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, via user.VerificationSource) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return user.ErrNotFound
	}
	u.EmailVerified = true
	u.EmailVerifiedVia = &via
	u.UpdatedAt = time.Now()
	return nil
}

func (f *fakeUserRepo) ClaimUnverifiedEmail(ctx context.Context, id uuid.UUID, via user.VerificationSource) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok || u.EmailVerified {
		return false, nil
	}
	u.EmailVerified = true
	u.EmailVerifiedVia = &via
	u.PasswordHash = nil
	u.UpdatedAt = time.Now()
	return true, nil
}

func (f *fakeUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	if p.Username != nil {
		for otherID, other := range f.users {
			if otherID == id {
				continue
			}
			if other.Username != nil && strings.EqualFold(*other.Username, *p.Username) {
				return nil, user.ErrUsernameTaken
			}
		}
		u.Username = p.Username
	}
	if p.Name != nil {
		u.Name = p.Name
	}
	if p.Bio != nil {
		u.Bio = p.Bio
	}
	if p.AvatarURL != nil {
		u.AvatarURL = p.AvatarURL
	}
	u.UpdatedAt = time.Now()
	stored := *u
	return &stored, nil
}

var _ user.Repository = (*fakeUserRepo)(nil)

// --- fakeRefreshRepo: in-memory user.RefreshTokenRepository ---

type fakeRefreshRow struct {
	id        uuid.UUID
	userID    uuid.UUID
	tokenHash string
	expiresAt time.Time
	revokedAt *time.Time
}

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

// --- fakeVerificationRepo: in-memory user.EmailVerificationRepository ---

type fakeVerificationRow struct {
	userID     uuid.UUID
	tokenHash  string
	expiresAt  time.Time
	consumedAt *time.Time
}

type fakeVerificationRepo struct {
	mu   sync.Mutex
	rows []*fakeVerificationRow
}

func newFakeVerificationRepo() *fakeVerificationRepo {
	return &fakeVerificationRepo{}
}

func (f *fakeVerificationRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, &fakeVerificationRow{userID: userID, tokenHash: string(tokenHash), expiresAt: expiresAt})
	return nil
}

func (f *fakeVerificationRepo) ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.tokenHash != string(tokenHash) {
			continue
		}
		if row.consumedAt != nil {
			return uuid.UUID{}, user.ErrTokenConsumed
		}
		if now.After(row.expiresAt) {
			return uuid.UUID{}, user.ErrTokenExpired
		}
		row.consumedAt = &now
		return row.userID, nil
	}
	return uuid.UUID{}, user.ErrTokenInvalid
}

func (f *fakeVerificationRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.rows[:0]
	for _, row := range f.rows {
		if row.userID != userID {
			kept = append(kept, row)
		}
	}
	f.rows = kept
	return nil
}

var _ user.EmailVerificationRepository = (*fakeVerificationRepo)(nil)

// --- fakeMailer: in-memory mail.Mailer ---

type fakeMailer struct {
	mu   sync.Mutex
	sent []*user.User
}

func newFakeMailer() *fakeMailer {
	return &fakeMailer{}
}

func (f *fakeMailer) SendVerificationEmail(ctx context.Context, u *user.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *u
	f.sent = append(f.sent, &stored)
	return nil
}

var _ mail.Mailer = (*fakeMailer)(nil)
