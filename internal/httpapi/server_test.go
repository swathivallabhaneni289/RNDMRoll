package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// testServerJWTSecret is fixed (not randomized per test) so a test can issue
// its own access token with auth.IssueAccessToken and expect the server
// built by newFullTestServer to accept it.
var testServerJWTSecret = []byte("test-secret-at-least-32-bytes!!")

// panicIfTouchedUserRepo embeds a nil user.Repository: any method call
// reaches the nil interface and panics. Used to prove a route never
// touches the database, rather than merely asserting a status code that
// could pass for the wrong reason.
type panicIfTouchedUserRepo struct {
	user.Repository
}

// newFullTestServer builds a Server wired exactly like cmd/api/main.go,
// using in-memory fakes for every dependency so no test here reaches a
// real database or network.
func newFullTestServer(t *testing.T, usersRepo user.Repository) *Server {
	t.Helper()

	refreshSvc := auth.NewRefreshService(newFakeRefreshRepo(), 30*24*time.Hour)
	verifications := newFakeVerificationRepo()
	mailer := newFakeMailer()

	authHandler := NewAuthHandler(usersRepo, refreshSvc, mailer, testServerJWTSecret, 15*time.Minute)
	verifyHandler := NewVerifyEmailHandler(usersRepo, verifications, mailer, refreshSvc, testServerJWTSecret, 15*time.Minute, "rndmroll")
	oauthHandler := NewOAuthHandler(usersRepo, &fakeAppleVerifier{}, &fakeGoogleVerifier{}, refreshSvc, testServerJWTSecret, 15*time.Minute)
	profileHandler := NewProfileHandler(usersRepo, &fakeAvatarStore{})
	usernameHandler := NewUsernameHandler(usersRepo)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewServer(Deps{
		Auth:        authHandler,
		VerifyEmail: verifyHandler,
		OAuth:       oauthHandler,
		Profile:     profileHandler,
		Username:    usernameHandler,
		Users:       usersRepo,
		JWTSecret:   testServerJWTSecret,
		Logger:      logger,
	})
}

func TestServer_HealthzReturns200WithoutTouchingAnyDependency(t *testing.T) {
	srv := newFullTestServer(t, panicIfTouchedUserRepo{})

	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /healthz, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_SignupIsReachableWithoutAuthentication(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())

	body := strings.NewReader(`{"email":"new-signup@example.com","password":"correct-horse-battery"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected signup to succeed without a token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_VerifyEmailIsMountedUnderAuthPrefix(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())

	body := strings.NewReader(`{"token":"not-a-real-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/verify-email", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)

	// A 404 here would mean the route isn't mounted at all; any other code
	// (401 token_invalid for the bogus token, in practice) proves the route
	// exists at the documented /v1/auth/verify-email path.
	if w.Code == http.StatusNotFound {
		t.Fatalf("expected /v1/auth/verify-email to be mounted, got 404")
	}
}

func TestServer_MeRequiresAuthentication(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())

	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for /v1/me with no token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_MeRejectsUnverifiedAccountWith403(t *testing.T) {
	users := newFakeUserRepo()
	u, err := users.Create(context.Background(), "unverified@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create unverified user: %v", err)
	}
	srv := newFullTestServer(t, users)

	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 email_not_verified for an unverified account, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_MeAllowsVerifiedAccount(t *testing.T) {
	users := newFakeUserRepo()
	via := user.VerifiedViaPasswordFlow
	u, err := users.Create(context.Background(), "verified@example.com", nil, true, &via)
	if err != nil {
		t.Fatalf("create verified user: %v", err)
	}
	srv := newFullTestServer(t, users)

	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a verified account, got %d: %s", w.Code, w.Body.String())
	}
}
