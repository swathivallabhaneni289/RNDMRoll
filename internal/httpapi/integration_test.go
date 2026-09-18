// Package httpapi (this file): end-to-end coverage of both Phase 1
// requirements (ACCT-01, ACCT-03) against a real Postgres database, driving
// the actual Server built by NewServer over HTTP rather than calling a
// handler method directly. Every helper and test in this file is prefixed
// with "integration" or "Integration" to avoid colliding with the many
// package-level test helpers wave 4 plans already declared in this same
// package (doJSONRequest and decodeBody are the two exceptions -- they
// already exist in auth_test.go and are reused here as-is).
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// integrationTestDatabaseURL is set from TEST_DATABASE_URL in TestMain.
// Empty means no test database is configured; every integration test below
// skips via requireIntegrationPool, matching plan 01-03's
// internal/store/postgres/postgres_test.go skip pattern so `go test ./...`
// stays green on a machine without a test database.
var integrationTestDatabaseURL string

// integrationJWTSecret signs and verifies tokens for every test in this
// file. Fixed rather than randomized per test so TestUnverifiedCannotReach
// Profile can mint its own token directly with auth.IssueAccessToken and
// have the server accept it.
var integrationJWTSecret = []byte("integration-test-secret-at-least-32-bytes")

func TestMain(m *testing.M) {
	integrationTestDatabaseURL = os.Getenv("TEST_DATABASE_URL")
	if integrationTestDatabaseURL != "" {
		if err := integrationRunMigrations(integrationTestDatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "httpapi: migration setup failed: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// integrationRunMigrations shells out to the migrate CLI to bring
// dbURL to the latest schema version before any integration test runs,
// mirroring internal/store/postgres/postgres_test.go's TestMain.
func integrationRunMigrations(dbURL string) error {
	if _, err := exec.LookPath("migrate"); err != nil {
		return fmt.Errorf("migrate CLI not found on PATH: %w", err)
	}
	cmd := exec.Command("migrate", "-path", filepath.Join(integrationRepoRoot(), "migrations"), "-database", dbURL, "up")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("migrate up failed: %w: %s", err, string(out))
	}
	return nil
}

// integrationRepoRoot resolves the repository root from this test file's
// own location, independent of the working directory `go test` happens to
// run from.
func integrationRepoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("httpapi: could not determine test file path")
	}
	// this file lives at <repoRoot>/internal/httpapi/integration_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// requireIntegrationPool skips the calling test when TEST_DATABASE_URL is
// unset, otherwise returns a fresh pool and truncates every account table
// on cleanup so tests do not leak state into one another.
func requireIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if integrationTestDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping httpapi integration test")
	}
	pool, err := postgres.NewPool(context.Background(), integrationTestDatabaseURL)
	if err != nil {
		t.Fatalf("httpapi integration: postgres.NewPool: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "truncate users cascade"); err != nil {
			t.Logf("httpapi integration: cleanup truncate failed: %v", err)
		}
		pool.Close()
	})
	return pool
}

// buildIntegrationServer wires the real Server -- real pgx repositories,
// real internal/mail.Service -- over pool. The mail service's Sender is
// internal/mail.NewLogSender pointed at an in-memory buffer instead of
// os.Stdout, so a test can extract the verification link without a real
// mail provider (MAIL_DRIVER=log is what this environment actually
// exercises; see plan 01-01's checkpoint). OAuth and avatar-upload
// dependencies are package-local fakes (fakeAppleVerifier,
// fakeGoogleVerifier, fakeAvatarStore -- declared in oauth_test.go and
// profile_test.go, same package) since the password-path onboarding flow
// this suite drives never reaches Apple, Google, or S3, and this
// environment holds non-functional placeholder credentials for all three.
func buildIntegrationServer(t *testing.T, pool *pgxpool.Pool) (*Server, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	users := postgres.NewUserRepo(pool)
	refreshTokens := postgres.NewRefreshTokenRepo(pool)
	verifications := postgres.NewEmailVerificationRepo(pool)

	refreshSvc := auth.NewRefreshService(refreshTokens, 30*24*time.Hour)

	mailLog := &bytes.Buffer{}
	mailSvc := mail.NewService(verifications, mail.NewLogSender(mailLog), "http://localhost:8080", 24*time.Hour)

	authHandler := NewAuthHandler(users, refreshSvc, mailSvc, integrationJWTSecret, 15*time.Minute)
	verifyHandler := NewVerifyEmailHandler(users, verifications, mailSvc, refreshSvc, integrationJWTSecret, 15*time.Minute, "rndmroll")
	oauthHandler := NewOAuthHandler(users, &fakeAppleVerifier{}, &fakeGoogleVerifier{}, refreshSvc, integrationJWTSecret, 15*time.Minute)
	profileHandler := NewProfileHandler(users, &fakeAvatarStore{})
	usernameHandler := NewUsernameHandler(users)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := NewServer(Deps{
		Auth:        authHandler,
		VerifyEmail: verifyHandler,
		OAuth:       oauthHandler,
		Profile:     profileHandler,
		Username:    usernameHandler,
		Users:       users,
		JWTSecret:   integrationJWTSecret,
		Logger:      logger,
	})

	return server, mailLog
}

// integrationVerificationTokenPattern extracts a token value from a
// captured log-mailer message, whose verification link always has the
// shape ".../verify-email/callback?token=<base64url-token>" (see
// internal/mail/mailer.go's SendVerificationEmail).
var integrationVerificationTokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// integrationExtractVerificationToken returns the most recently issued
// verification token in mailLog. Taking the last match (not the only
// match) is deliberate: mailLog accumulates across every signup a test
// performs against the same server, so a test onboarding a second account
// after a first still extracts the right token.
func integrationExtractVerificationToken(t *testing.T, mailLog *bytes.Buffer) string {
	t.Helper()
	matches := integrationVerificationTokenPattern.FindAllStringSubmatch(mailLog.String(), -1)
	if len(matches) == 0 {
		t.Fatalf("httpapi integration: no verification token found in captured mail log: %s", mailLog.String())
	}
	return matches[len(matches)-1][1]
}

// integrationDoAuthedRequest mirrors auth_test.go's doJSONRequest but also
// sets the Authorization header, since that helper has no way to attach a
// bearer token.
func integrationDoAuthedRequest(t *testing.T, router *gin.Engine, method, path, accessToken string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// integrationSession is the access/refresh pair integrationOnboardAccount
// returns once an account has completed the password-path onboarding flow.
type integrationSession struct {
	AccessToken  string
	RefreshToken string
}

// integrationOnboardAccount drives signup, verify-email, and a single
// PATCH /v1/me setting both name and username through HTTP, leaving the
// account fully onboarded (onboarding_complete == true). Used by every
// test below except TestFullOnboardingFlow itself, which walks the same
// sequence as separate, individually asserted steps because that
// step-by-step shape is the thing it exists to prove.
func integrationOnboardAccount(t *testing.T, engine *gin.Engine, mailLog *bytes.Buffer, email, password, name, username string) integrationSession {
	t.Helper()

	signupRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/signup", map[string]any{
		"email":    email,
		"password": password,
	})
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup(%s): expected 201, got %d: %s", email, signupRec.Code, signupRec.Body.String())
	}

	token := integrationExtractVerificationToken(t, mailLog)
	verifyRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/verify-email", map[string]any{"token": token})
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify-email(%s): expected 200, got %d: %s", email, verifyRec.Code, verifyRec.Body.String())
	}
	verifyBody := decodeBody(t, verifyRec)
	accessToken, _ := verifyBody["access_token"].(string)
	refreshToken, _ := verifyBody["refresh_token"].(string)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("verify-email(%s): expected access_token and refresh_token, got: %v", email, verifyBody)
	}

	patchRec := integrationDoAuthedRequest(t, engine, http.MethodPatch, "/v1/me", accessToken, map[string]any{
		"name":     name,
		"username": username,
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch profile(%s): expected 200, got %d: %s", email, patchRec.Code, patchRec.Body.String())
	}

	return integrationSession{AccessToken: accessToken, RefreshToken: refreshToken}
}

// TestFullOnboardingFlow walks the entire D-05 password path through HTTP:
// signup, pull the verification token out of the captured mail body,
// verify-email and keep the returned session, set a name, fetch a
// suggested username, set that username, then confirm GET /v1/me reports
// onboarding_complete. This is the closest automated proxy for ACCT-01 and
// ACCT-03 together: it exercises every handler and both cross-cutting
// middlewares (Recovery, RequestLogger) in sequence, over the real Server.
func TestFullOnboardingFlow(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, mailLog := buildIntegrationServer(t, pool)
	engine := server.Engine()

	const email = "onboarding-flow@example.com"

	signupRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/signup", map[string]any{
		"email":    email,
		"password": "correct-horse-battery-staple",
	})
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %s", signupRec.Code, signupRec.Body.String())
	}

	token := integrationExtractVerificationToken(t, mailLog)
	verifyRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/verify-email", map[string]any{"token": token})
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify-email: expected 200, got %d: %s", verifyRec.Code, verifyRec.Body.String())
	}
	verifyBody := decodeBody(t, verifyRec)
	accessToken, _ := verifyBody["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("verify-email: expected access_token, got: %v", verifyBody)
	}

	nameRec := integrationDoAuthedRequest(t, engine, http.MethodPatch, "/v1/me", accessToken, map[string]any{
		"name": "Onboarding Flow Tester",
	})
	if nameRec.Code != http.StatusOK {
		t.Fatalf("patch name: expected 200, got %d: %s", nameRec.Code, nameRec.Body.String())
	}

	suggestRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/usernames/suggest?name=Onboarding+Flow+Tester", accessToken, nil)
	if suggestRec.Code != http.StatusOK {
		t.Fatalf("suggest username: expected 200, got %d: %s", suggestRec.Code, suggestRec.Body.String())
	}
	suggestBody := decodeBody(t, suggestRec)
	username, _ := suggestBody["username"].(string)
	if username == "" {
		t.Fatalf("suggest username: expected a non-empty suggestion, got: %v", suggestBody)
	}

	usernameRec := integrationDoAuthedRequest(t, engine, http.MethodPatch, "/v1/me", accessToken, map[string]any{
		"username": username,
	})
	if usernameRec.Code != http.StatusOK {
		t.Fatalf("patch username: expected 200, got %d: %s", usernameRec.Code, usernameRec.Body.String())
	}

	meRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", accessToken, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("get me: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
	meBody := decodeBody(t, meRec)
	if onboardingComplete, _ := meBody["onboarding_complete"].(bool); !onboardingComplete {
		t.Fatalf("expected onboarding_complete=true after signup+verify+name+username, got: %v", meBody)
	}
	if meBody["username"] != username {
		t.Fatalf("expected /v1/me to reflect the username just set (%q), got %v", username, meBody["username"])
	}
}

// TestSessionSurvivesRelaunch covers ACCT-01's distinguishing clause: a
// mobile client discards its access token on every cold start and holds
// only the refresh token, so the app must be able to mint a fresh access
// token from just that and immediately use it.
func TestSessionSurvivesRelaunch(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, mailLog := buildIntegrationServer(t, pool)
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, mailLog, "relaunch@example.com", "correct-horse-battery-staple", "Relaunch Tester", "relaunch_user")

	// Discard session.AccessToken here, on purpose: only the refresh token
	// crosses a real cold start.
	refreshRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}
	refreshBody := decodeBody(t, refreshRec)
	newAccessToken, _ := refreshBody["access_token"].(string)
	if newAccessToken == "" {
		t.Fatalf("refresh: expected a new access_token, got: %v", refreshBody)
	}

	meRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", newAccessToken, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("get me with the relaunch-issued access token: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
}

// TestCrossAccountIsolation creates two fully onboarded accounts and
// asserts account B holds no way to read or modify account A's record,
// constructing the attempt the way a real caller would: sending A's ID in
// the PATCH /v1/me body while authenticated as B.
func TestCrossAccountIsolation(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, mailLog := buildIntegrationServer(t, pool)
	engine := server.Engine()

	sessionA := integrationOnboardAccount(t, engine, mailLog, "account-a@example.com", "correct-horse-battery-staple", "Account A", "account_a_user")
	sessionB := integrationOnboardAccount(t, engine, mailLog, "account-b@example.com", "correct-horse-battery-staple", "Account B", "account_b_user")

	meABeforeRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", sessionA.AccessToken, nil)
	meABefore := decodeBody(t, meABeforeRec)
	accountAID := meABefore["id"]

	// Authenticated as B, submit A's ID alongside a real mutating field.
	// UpdateProfileRequest has no id/user_id field, so the id is silently
	// ignored by JSON binding -- if the target user were derived from
	// anything other than the bearer token's subject, this bio would land
	// on A's row instead of B's.
	patchRec := integrationDoAuthedRequest(t, engine, http.MethodPatch, "/v1/me", sessionB.AccessToken, map[string]any{
		"id":  accountAID,
		"bio": "this change must land on B, never on A",
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch as B with A's id in the body: expected 200, got %d: %s", patchRec.Code, patchRec.Body.String())
	}

	meAAfterRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", sessionA.AccessToken, nil)
	meAAfter := decodeBody(t, meAAfterRec)
	if meAAfter["bio"] != nil {
		t.Fatalf("account A's bio was modified by a request authenticated as B: %v", meAAfter["bio"])
	}
	if meAAfter["id"] != accountAID {
		t.Fatalf("account A's id changed across requests: before=%v after=%v", accountAID, meAAfter["id"])
	}

	meBRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", sessionB.AccessToken, nil)
	meB := decodeBody(t, meBRec)
	if meB["bio"] != "this change must land on B, never on A" {
		t.Fatalf("expected B's own bio to be updated, got: %v", meB["bio"])
	}
	if meB["id"] == accountAID {
		t.Fatalf("B's id in the response equals A's id -- the wrong row was returned or updated")
	}
}

// TestUnverifiedCannotReachProfile proves D-04 at the API boundary: an
// unverified account cannot reach GET /v1/me. Signup never returns a
// session and Login for an unverified account returns 403 without one, so
// there is no HTTP-only path to a bearer token for an unverified account --
// this test mints one directly with auth.IssueAccessToken, which is the
// only way to exercise RequireVerified's gate rather than merely proving
// login refuses to hand one out.
func TestUnverifiedCannotReachProfile(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	signupRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/signup", map[string]any{
		"email":    "unverified-profile@example.com",
		"password": "correct-horse-battery-staple",
	})
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %s", signupRec.Code, signupRec.Body.String())
	}
	signupBody := decodeBody(t, signupRec)
	userIDStr, _ := signupBody["user_id"].(string)
	if userIDStr == "" {
		t.Fatalf("signup: expected user_id, got: %v", signupBody)
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		t.Fatalf("parse user_id %q: %v", userIDStr, err)
	}

	token, err := auth.IssueAccessToken(userID, integrationJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	meRec := integrationDoAuthedRequest(t, engine, http.MethodGet, "/v1/me", token, nil)
	if meRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an unverified account, got %d: %s", meRec.Code, meRec.Body.String())
	}
	meBody := decodeBody(t, meRec)
	if meBody["error"] != "email_not_verified" {
		t.Fatalf("expected error=email_not_verified, got %v", meBody["error"])
	}
}

// TestRefreshTokenSingleUse refreshes once, then replays the original
// refresh token and asserts it is now rejected -- RefreshService.Redeem's
// rotate-on-use guarantee (plan 01-06), proven here over HTTP rather than
// only at the service layer.
func TestRefreshTokenSingleUse(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, mailLog := buildIntegrationServer(t, pool)
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, mailLog, "single-use-refresh@example.com", "correct-horse-battery-staple", "Single Use", "single_use_user")

	firstRefreshRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if firstRefreshRec.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d: %s", firstRefreshRec.Code, firstRefreshRec.Body.String())
	}

	replayRec := doJSONRequest(t, engine, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if replayRec.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh token: expected 401, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
	replayBody := decodeBody(t, replayRec)
	if replayBody["error"] != "token_invalid" {
		t.Fatalf("expected error=token_invalid on replay, got %v", replayBody["error"])
	}
}
