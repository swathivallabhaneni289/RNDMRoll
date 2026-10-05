package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// fakeVerifiedRepo is a package-local, minimal user.Repository double: only
// GetByID has real behavior, since that is all RequireVerified calls. Every
// other method is a harmless stub so the type still satisfies the full
// interface RequireVerified's parameter requires.
type fakeVerifiedRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]*user.User
}

func newFakeVerifiedRepo() *fakeVerifiedRepo {
	return &fakeVerifiedRepo{users: make(map[uuid.UUID]*user.User)}
}

func (f *fakeVerifiedRepo) put(u *user.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[u.ID] = u
}

func (f *fakeVerifiedRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *user.VerificationSource) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (f *fakeVerifiedRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	stored := *u
	return &stored, nil
}

func (f *fakeVerifiedRepo) GetByEmailCI(ctx context.Context, email string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (f *fakeVerifiedRepo) GetByProviderSubject(ctx context.Context, provider user.VerificationSource, subject string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (f *fakeVerifiedRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider user.VerificationSource, subject string) error {
	return nil
}

func (f *fakeVerifiedRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (f *fakeVerifiedRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, via user.VerificationSource) error {
	return nil
}

func (f *fakeVerifiedRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (f *fakeVerifiedRepo) ClaimUnverifiedEmail(ctx context.Context, id uuid.UUID, via user.VerificationSource) (bool, error) {
	return false, nil
}

var _ user.Repository = (*fakeVerifiedRepo)(nil)

const testJWTSecret = "test-secret-at-least-32-bytes!!"

func TestRequireVerified_VerifiedAccountReachesHandler(t *testing.T) {
	repo := newFakeVerifiedRepo()
	userID := uuid.New()
	repo.put(&user.User{ID: userID, Email: "a@example.com", EmailVerified: true})

	token, err := auth.IssueAccessToken(userID, []byte(testJWTSecret), 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	handlerRan := false
	router := gin.New()
	router.GET("/protected", RequireAuth([]byte(testJWTSecret)), RequireVerified(repo), func(c *gin.Context) {
		handlerRan = true
		u, err := UserFromContext(c)
		if err != nil {
			t.Errorf("UserFromContext returned error: %v", err)
		}
		if u.ID != userID {
			t.Errorf("UserFromContext id = %s, want %s", u.ID, userID)
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !handlerRan {
		t.Fatal("expected handler to run for a verified account")
	}
}

func TestRequireVerified_UnverifiedAccountReturns403AndHandlerNeverRuns(t *testing.T) {
	repo := newFakeVerifiedRepo()
	userID := uuid.New()
	repo.put(&user.User{ID: userID, Email: "b@example.com", EmailVerified: false})

	token, err := auth.IssueAccessToken(userID, []byte(testJWTSecret), 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	handlerRan := false
	router := gin.New()
	router.GET("/protected", RequireAuth([]byte(testJWTSecret)), RequireVerified(repo), func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "email_not_verified") {
		t.Fatalf("expected email_not_verified body, got: %s", w.Body.String())
	}
	if handlerRan {
		t.Fatal("handler must not run for an unverified account")
	}
}

func TestRequireVerified_UnknownSubjectReturns401AndHandlerNeverRuns(t *testing.T) {
	repo := newFakeVerifiedRepo() // empty -- no account for this subject

	token, err := auth.IssueAccessToken(uuid.New(), []byte(testJWTSecret), 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	handlerRan := false
	router := gin.New()
	router.GET("/protected", RequireAuth([]byte(testJWTSecret)), RequireVerified(repo), func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if handlerRan {
		t.Fatal("handler must not run for an unresolvable subject")
	}
}

func TestRequireVerified_MissingAuthReturns401AndHandlerNeverRuns(t *testing.T) {
	repo := newFakeVerifiedRepo()

	handlerRan := false
	router := gin.New()
	router.GET("/protected", RequireAuth([]byte(testJWTSecret)), RequireVerified(repo), func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if handlerRan {
		t.Fatal("handler must not run without an Authorization header")
	}
}

func TestUserFromContext_ReturnsErrorWhenMiddlewareDidNotRun(t *testing.T) {
	router := gin.New()
	router.GET("/unprotected", func(c *gin.Context) {
		if _, err := UserFromContext(c); err == nil {
			t.Error("expected an error when RequireVerified did not run")
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unprotected", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
