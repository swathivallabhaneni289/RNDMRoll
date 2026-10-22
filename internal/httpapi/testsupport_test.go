// Package httpapi_test-support: this file provides the harness every handler
// test builds on -- a bare router constructor plus in-memory fakes for the
// user and refresh-token repository interfaces. Because every handler mounts
// itself on a fresh engine inside its own test, no handler test depends on
// cmd/api/main.go existing.
package httpapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// TestDeps bundles the fakes a handler test wires into the handler
// constructors it builds on top of newTestRouter. Each field is the
// interface type a real handler depends on, not the concrete fake, so a
// test can swap in a different double without changing the handler.
type TestDeps struct {
	Users         user.Repository
	RefreshTokens user.RefreshTokenRepository
}

// frozenClock is a limit clock that never moves: a test's limits never
// refill, so they never depend on how fast the machine runs the requests
// (bcrypt under the race detector can take seconds).
func frozenClock() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }

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
	// birthdays holds the stored date per user. user.User deliberately has
	// no date field, so a test reads it here to prove what was stored.
	birthdays map[uuid.UUID]string
	// tokens, when set, lets remove revoke refresh tokens the way the
	// database does when an account row goes (cascade), and lets
	// ChangePassword replace an account's sessions.
	tokens *fakeRefreshRepo
	// changePasswordErr, when set, is what ChangePassword returns before it
	// changes anything: a test uses it to stand in for a failed transaction.
	changePasswordErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*user.User), birthdays: make(map[uuid.UUID]string)}
}

// count returns how many accounts are stored.
func (f *fakeUserRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.users)
}

// storedBirthday returns the birthday on file for id, "" when none.
func (f *fakeUserRepo) storedBirthday(id uuid.UUID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.birthdays[id]
}

// seedComplete stores a finished account that has a birthday on file.
func (f *fakeUserRepo) seedComplete(t *testing.T, email, name, username, birthday string) *user.User {
	t.Helper()
	pw := "x"
	u, err := f.CreateComplete(context.Background(), user.NewAccount{
		Email: email, PasswordHash: pw, Birthday: birthday, Name: name, Username: username,
	})
	if err != nil {
		t.Fatalf("seed complete account: %v", err)
	}
	return u
}

func (f *fakeUserRepo) CreateComplete(ctx context.Context, in user.NewAccount) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Email, in.Email) {
			return nil, user.ErrEmailTaken
		}
		if u.Username != nil && strings.EqualFold(*u.Username, in.Username) {
			return nil, user.ErrUsernameTaken
		}
	}
	now := time.Now()
	hash, name, username := in.PasswordHash, in.Name, in.Username
	// Like the real repository: an empty starting value is stored as public,
	// and a value the check constraint would refuse is a field error.
	visibility := in.AccountVisibility
	if visibility == "" {
		visibility = user.VisibilityPublic
	}
	if !user.ValidVisibility(visibility) {
		return nil, &user.FieldError{Field: "account_visibility"}
	}
	u := &user.User{
		ID:                uuid.New(),
		Email:             in.Email,
		PasswordHash:      &hash,
		Name:              &name,
		Username:          &username,
		Bio:               in.Bio,
		HasBirthday:       true,
		CreatedAt:         now,
		UpdatedAt:         now,
		AccountVisibility: visibility,
	}
	f.users[u.ID] = u
	f.birthdays[u.ID] = in.Birthday
	stored := *u
	return &stored, nil
}

// remove deletes an account the way the database does when its row goes:
// the account is gone and its refresh tokens stop working.
func (f *fakeUserRepo) remove(t *testing.T, id uuid.UUID) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[id]; !ok {
		t.Fatalf("remove: no account %s", id)
	}
	delete(f.users, id)
	delete(f.birthdays, id)
	if f.tokens != nil {
		_ = f.tokens.RevokeAllForUser(context.Background(), id)
	}
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
		ID:                uuid.New(),
		Email:             email,
		PasswordHash:      passwordHash,
		EmailVerified:     verified,
		EmailVerifiedVia:  via,
		CreatedAt:         now,
		UpdatedAt:         now,
		AccountVisibility: user.VisibilityPublic, // the column default
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

func (f *fakeUserRepo) GetByUsernameCI(ctx context.Context, username string) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Username != nil && strings.EqualFold(*u.Username, username) {
			stored := *u
			return &stored, nil
		}
	}
	return nil, user.ErrNotFound
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

func (f *fakeUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	// The database check constraint, as a field error, before anything is
	// applied: one UPDATE changes everything or nothing.
	if p.AccountVisibility != nil && !user.ValidVisibility(*p.AccountVisibility) {
		return nil, &user.FieldError{Field: "account_visibility"}
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
	if p.AccountVisibility != nil {
		u.AccountVisibility = *p.AccountVisibility
	}
	u.UpdatedAt = time.Now()
	stored := *u
	return &stored, nil
}

// ChangePassword mirrors the Postgres repository's one transaction: the hash,
// every session and the one new session change together or not at all, and
// a disabled account changes nothing. It needs the tokens field (the rigs set
// it) and fails loudly without it, so a rig that forgot to wire it cannot pass
// by accident.
func (f *fakeUserRepo) ChangePassword(ctx context.Context, userID uuid.UUID, passwordHash string, sessionID uuid.UUID, newTokenHash []byte, expiresAt time.Time, userAgent *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.changePasswordErr != nil {
		return f.changePasswordErr
	}
	if f.tokens == nil {
		return errors.New("fakeUserRepo.ChangePassword: the tokens field is not wired")
	}
	u, ok := f.users[userID]
	if !ok {
		return user.ErrNotFound
	}
	if u.Disabled {
		return user.ErrAccountDisabled
	}
	if err := f.tokens.RevokeAllForUser(ctx, userID); err != nil {
		return err
	}
	f.tokens.insertWithID(sessionID, userID, newTokenHash, expiresAt)
	hash := passwordHash
	u.PasswordHash = &hash
	u.UpdatedAt = time.Now()
	return nil
}

// setDisabled disables the account id, as an admin's disable does.
func (f *fakeUserRepo) setDisabled(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		u.Disabled = true
	}
}

// storedPasswordHash returns the password hash on file for id, "" when none.
func (f *fakeUserRepo) storedPasswordHash(id uuid.UUID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok && u.PasswordHash != nil {
		return *u.PasswordHash
	}
	return ""
}

var (
	_ user.Repository      = (*fakeUserRepo)(nil)
	_ user.PasswordChanger = (*fakeUserRepo)(nil)
)

// --- fakeRefreshRepo: in-memory user.RefreshTokenRepository ---

type fakeRefreshRow struct {
	id        uuid.UUID
	userID    uuid.UUID
	tokenHash string
	expiresAt time.Time
	revokedAt *time.Time
	rotatedTo *uuid.UUID
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

// insertWithID stores a token under an id the caller chose, as a password
// change stores the new session's.
func (f *fakeRefreshRepo) insertWithID(id, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[id] = &fakeRefreshRow{id: id, userID: userID, tokenHash: string(tokenHash), expiresAt: expiresAt}
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

// Rotate, like the Postgres repository's, refuses an old token that is no
// longer live and names the new one in the old one's rotatedTo.
func (f *fakeRefreshRepo) Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.rows[oldID]
	if !ok || old.revokedAt != nil {
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	newID := uuid.New()
	f.rows[newID] = &fakeRefreshRow{id: newID, userID: old.userID, tokenHash: string(newHash), expiresAt: expiresAt}
	now := time.Now()
	old.revokedAt = &now
	old.rotatedTo = &newID
	return newID, nil
}

// RevokeByHash, like the Postgres repository's, ends the session: it follows
// rotatedTo to the end of the chain and revokes that token when it is live.
func (f *fakeRefreshRepo) RevokeByHash(ctx context.Context, tokenHash []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.tokenHash != string(tokenHash) {
			continue
		}
		for row.rotatedTo != nil {
			row = f.rows[*row.rotatedTo]
		}
		if row.revokedAt != nil {
			return user.ErrTokenInvalid
		}
		now := time.Now()
		row.revokedAt = &now
		return nil
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

// activeCount returns how many refresh tokens userID holds that are not
// revoked.
func (f *fakeRefreshRepo) activeCount(userID uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, row := range f.rows {
		if row.userID == userID && row.revokedAt == nil {
			n++
		}
	}
	return n
}

var _ user.RefreshTokenRepository = (*fakeRefreshRepo)(nil)
