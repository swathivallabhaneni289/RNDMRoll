// Package httpapi (this file): end-to-end coverage of both Phase 1
// requirements (ACCT-01, ACCT-03) against a real Postgres database, driving
// the actual Server built by NewServer over HTTP rather than calling a
// handler method directly. Every helper and test in this file is prefixed
// with "integration" or "Integration" to avoid colliding with the many
// package-level test helpers other files in this package already declare
// (doJSONRequest and decodeBody are the two exceptions -- they already
// exist in auth_test.go and are reused here as-is).
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
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// integrationTestDatabaseURL is set from TEST_DATABASE_URL in TestMain.
// Empty means no test database is configured; every integration test below
// skips via requireIntegrationPool, so `go test ./...` stays green on a
// machine without a test database. A configured URL must name a database
// ending in _test: TestMain stops the process before any migration or
// truncate otherwise.
var integrationTestDatabaseURL string

// integrationJWTSecret signs and verifies tokens for every test in this
// file. Fixed rather than randomized per test so a test can mint its own
// token directly with auth.IssueAccessToken and have the server accept it.
var integrationJWTSecret = []byte("integration-test-secret-at-least-32-bytes")

func TestMain(m *testing.M) {
	integrationTestDatabaseURL = os.Getenv("TEST_DATABASE_URL")
	if integrationTestDatabaseURL != "" {
		// Refuse a non-test database before anything migrates or truncates.
		if err := postgres.CheckTestDatabaseURL(integrationTestDatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "httpapi: %v\n", err)
			os.Exit(1)
		}
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
//
// NOTE on running this package's tests alongside internal/store/postgres's:
// both packages truncate the same `users` table against the same
// TEST_DATABASE_URL. `go test ./...` never sets TEST_DATABASE_URL itself,
// so a plain `go test ./...` run is unaffected either way (every test here
// and in internal/store/postgres skips). But `TEST_DATABASE_URL=... go
// test ./...` (exporting it for the whole module) can run this package's
// and internal/store/postgres's test binaries concurrently under go test's
// default package-level parallelism, and a truncate from one package's
// cleanup can then wipe rows a concurrently-running test in the other
// still needs. `make test-all-integration` runs the full suite with
// TEST_DATABASE_URL set and `-p 1` to serialize package execution and
// avoid exactly that race; prefer it over a bare `TEST_DATABASE_URL=...
// go test ./...` invocation.
func requireIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if integrationTestDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping httpapi integration test")
	}
	pool, err := postgres.NewPool(context.Background(), integrationTestDatabaseURL)
	if err != nil {
		t.Fatalf("httpapi integration: postgres.NewPool: %v", err)
	}
	// Truncate on entry too, not just on cleanup: guards against residue
	// left behind by a prior run that crashed or was interrupted before its
	// own t.Cleanup ran, which would otherwise surface as a spurious
	// user.ErrEmailTaken on this run's first signup.
	if _, err := pool.Exec(context.Background(), "truncate users cascade"); err != nil {
		pool.Close()
		t.Fatalf("httpapi integration: entry truncate failed: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "truncate users cascade"); err != nil {
			t.Logf("httpapi integration: cleanup truncate failed: %v", err)
		}
		pool.Close()
	})
	return pool
}

// buildIntegrationServer wires the real Server -- real pgx repositories --
// over pool. OAuth and avatar-upload dependencies are package-local fakes
// (fakeAppleVerifier, fakeGoogleVerifier, fakeAvatarStore -- declared in
// oauth_test.go and profile_test.go, same package) since this environment
// holds non-functional placeholder credentials for all three. The Google
// fake is returned so a test can arm an identity.
func buildIntegrationServer(t *testing.T, pool *pgxpool.Pool) (*Server, *fakeGoogleVerifier) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	users := postgres.NewUserRepo(pool)
	refreshTokens := postgres.NewRefreshTokenRepo(pool)

	refreshSvc := auth.NewRefreshService(refreshTokens, 30*24*time.Hour)
	google := &fakeGoogleVerifier{}

	authHandler := NewAuthHandler(users, refreshSvc, integrationJWTSecret, 15*time.Minute)
	oauthHandler := NewOAuthHandler(users, &fakeAppleVerifier{}, google, refreshSvc, integrationJWTSecret, 15*time.Minute)
	profileHandler := NewProfileHandler(users, &fakeAvatarStore{}, testAvatarBaseURL)
	usernameHandler := NewUsernameHandler(users)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := NewServer(Deps{
		Auth:      authHandler,
		OAuth:     oauthHandler,
		Profile:   profileHandler,
		Username:  usernameHandler,
		Users:     users,
		JWTSecret: integrationJWTSecret,
		Logger:    logger,
	})

	return server, google
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

// integrationSession is the access/refresh pair a one-request sign-up
// returns.
type integrationSession struct {
	AccessToken  string
	RefreshToken string
}

// integrationRemote returns a distinct socket address per call so the
// per-address rate limits never interfere with a test that makes many calls.
var integrationRemoteCounter struct {
	sync.Mutex
	n int
}

func integrationRemote() string {
	integrationRemoteCounter.Lock()
	defer integrationRemoteCounter.Unlock()
	integrationRemoteCounter.n++
	return fmt.Sprintf("192.0.2.%d:%d", integrationRemoteCounter.n%250+1, 30000+integrationRemoteCounter.n)
}

// integrationDo sends one request through the real server from a fresh
// socket address, with an optional bearer token, and runs the "no birth key
// in any response" check on whatever comes back.
func integrationDo(t *testing.T, engine *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = integrationRemote()
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assertNoBirthKey(t, method+" "+path, rec.Body.Bytes())
	return rec
}

// integrationOnboardAccount signs an account up in ONE request, leaving it
// complete and signed in (the response carries the session).
func integrationOnboardAccount(t *testing.T, engine *gin.Engine, email, password, name, username string) integrationSession {
	t.Helper()

	rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email":    email,
		"password": password,
		"birthday": validBirthday,
		"name":     name,
		"username": username,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("signup(%s): expected 200, got %d: %s", email, rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	accessToken, _ := body["access_token"].(string)
	refreshToken, _ := body["refresh_token"].(string)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("signup(%s): expected access_token and refresh_token, got: %v", email, body)
	}
	return integrationSession{AccessToken: accessToken, RefreshToken: refreshToken}
}

// integrationUserCount returns how many accounts the database holds.
func integrationUserCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

// TestFullOnboardingFlow walks the whole sign-up through HTTP: public
// username checks with no token, ONE sign-up request that returns a signed-in
// session, then GET /v1/me reporting onboarding_complete. It exercises every
// handler and both cross-cutting middlewares over the real Server.
func TestFullOnboardingFlow(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	suggestRec := integrationDo(t, engine, http.MethodGet, "/v1/usernames/suggest?name=Onboarding+Flow+Tester", "", nil)
	if suggestRec.Code != http.StatusOK {
		t.Fatalf("suggest username without a token: expected 200, got %d: %s", suggestRec.Code, suggestRec.Body.String())
	}
	username, _ := decodeBody(t, suggestRec)["username"].(string)
	if username == "" {
		t.Fatalf("suggest username: expected a non-empty suggestion, got: %s", suggestRec.Body.String())
	}
	availableRec := integrationDo(t, engine, http.MethodGet, "/v1/usernames/available?username="+username, "", nil)
	if availableRec.Code != http.StatusOK || decodeBody(t, availableRec)["available"] != true {
		t.Fatalf("available: %d %s", availableRec.Code, availableRec.Body.String())
	}

	signupRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email":    "onboarding-flow@example.com",
		"password": "correct-horse-battery-staple",
		"birthday": validBirthday,
		"name":     "Onboarding Flow Tester",
		"username": username,
		"bio":      "one request",
	})
	if signupRec.Code != http.StatusOK {
		t.Fatalf("signup: expected 200, got %d: %s", signupRec.Code, signupRec.Body.String())
	}
	signupBody := decodeBody(t, signupRec)
	accessToken, _ := signupBody["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("signup: expected access_token, got: %v", signupBody)
	}
	signupUser, _ := signupBody["user"].(map[string]any)
	if signupUser["onboarding_complete"] != true || signupUser["email_verified"] != false {
		t.Fatalf("signup user: expected complete and unverified, got %v", signupUser)
	}

	meRec := integrationDo(t, engine, http.MethodGet, "/v1/me", accessToken, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("get me: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
	meBody := decodeBody(t, meRec)
	if complete, _ := meBody["onboarding_complete"].(bool); !complete {
		t.Fatalf("expected onboarding_complete=true straight after sign-up, got: %v", meBody)
	}
	if meBody["username"] != username || meBody["name"] != "Onboarding Flow Tester" || meBody["bio"] != "one request" {
		t.Fatalf("expected /v1/me to reflect the sign-up, got: %v", meBody)
	}

	// The birthday really is stored, and only the server knows it.
	var stored string
	if err := pool.QueryRow(context.Background(), `select birthday::text from users where email = $1`, "onboarding-flow@example.com").Scan(&stored); err != nil || stored != validBirthday {
		t.Fatalf("stored birthday = %q, err = %v, want %q", stored, err, validBirthday)
	}
}

// TestSessionSurvivesRelaunch covers ACCT-01's distinguishing clause: a
// mobile client discards its access token on every cold start and holds
// only the refresh token, so the app must be able to mint a fresh access
// token from just that and immediately use it.
func TestSessionSurvivesRelaunch(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, "relaunch@example.com", "correct-horse-battery-staple", "Relaunch Tester", "relaunch_user")

	// Discard session.AccessToken here, on purpose: only the refresh token
	// crosses a real cold start.
	refreshRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}
	newAccessToken, _ := decodeBody(t, refreshRec)["access_token"].(string)
	if newAccessToken == "" {
		t.Fatalf("refresh: expected a new access_token, got: %s", refreshRec.Body.String())
	}

	meRec := integrationDo(t, engine, http.MethodGet, "/v1/me", newAccessToken, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("get me with the relaunch-issued access token: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
	if decodeBody(t, meRec)["onboarding_complete"] != true {
		t.Fatalf("expected a complete account after relaunch, got %s", meRec.Body.String())
	}
}

// TestUnverifiedAccountCanLogInRefreshAndUseTheApp proves the gate is a valid
// token for an account that exists, nothing more: an account whose email was
// never verified logs in, refreshes (Redeem), and reaches /me complete.
func TestUnverifiedAccountCanLogInRefreshAndUseTheApp(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	integrationOnboardAccount(t, engine, "unverified-profile@example.com", "correct-horse-battery-staple", "Unverified Person", "unverified_person")
	var verified bool
	if err := pool.QueryRow(context.Background(), `select email_verified from users where email = $1`, "unverified-profile@example.com").Scan(&verified); err != nil || verified {
		t.Fatalf("expected an unverified account, verified = %v, err = %v", verified, err)
	}

	loginRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email": "unverified-profile@example.com", "password": "correct-horse-battery-staple",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login of an unverified account: expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
	refreshToken, _ := decodeBody(t, loginRec)["refresh_token"].(string)

	refreshRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refreshToken})
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}
	access, _ := decodeBody(t, refreshRec)["access_token"].(string)

	meRec := integrationDo(t, engine, http.MethodGet, "/v1/me", access, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("/me for an unverified account: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
	meBody := decodeBody(t, meRec)
	if meBody["onboarding_complete"] != true || meBody["email_verified"] != false {
		t.Fatalf("expected complete and unverified, got %v", meBody)
	}

	wrong := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email": "unverified-profile@example.com", "password": "abcde",
	})
	if wrong.Code != http.StatusUnauthorized || decodeBody(t, wrong)["error"] != "invalid_credentials" {
		t.Fatalf("5-character wrong password: expected 401 invalid_credentials, got %d: %s", wrong.Code, wrong.Body.String())
	}
}

// TestLoginWithAUsernameReachesTheSameAccountAsTheEmail proves the Log in
// page's "email or username" box against real Postgres: the username (in any
// case) and the email open the same account with the account's one password,
// and a wrong password or an unknown username is the same 401.
func TestLoginWithAUsernameReachesTheSameAccountAsTheEmail(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	const password = "correct-horse-battery-staple"
	integrationOnboardAccount(t, engine, "username-login@example.com", password, "Username Login", "username_login")

	for name, login := range map[string]string{
		"username":             "username_login",
		"username in capitals": "USERNAME_Login",
		"email":                "username-login@example.com",
	} {
		rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{
			"login": login, "password": password,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("login by %s: expected 200, got %d: %s", name, rec.Code, rec.Body.String())
		}
		got, _ := decodeBody(t, rec)["user"].(map[string]any)
		if got["email"] != "username-login@example.com" || got["username"] != "username_login" {
			t.Fatalf("login by %s: expected the account just made, got %v", name, got)
		}
	}

	wrong := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{
		"login": "username_login", "password": "abcde",
	})
	unknown := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{
		"login": "nobody_here", "password": "abcde",
	})
	for name, rec := range map[string]*httptest.ResponseRecorder{"wrong password": wrong, "unknown username": unknown} {
		if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["error"] != "invalid_credentials" {
			t.Fatalf("%s: expected 401 invalid_credentials, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("expected byte-identical bodies, got %q vs %q", wrong.Body.String(), unknown.Body.String())
	}
}

// TestCrossAccountIsolation creates two accounts and asserts account B holds
// no way to read or modify account A's record, constructing the attempt the
// way a real caller would: sending A's ID in the PATCH /v1/me body while
// authenticated as B.
func TestCrossAccountIsolation(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	sessionA := integrationOnboardAccount(t, engine, "account-a@example.com", "correct-horse-battery-staple", "Account A", "account_a_user")
	sessionB := integrationOnboardAccount(t, engine, "account-b@example.com", "correct-horse-battery-staple", "Account B", "account_b_user")

	meABefore := decodeBody(t, integrationDo(t, engine, http.MethodGet, "/v1/me", sessionA.AccessToken, nil))
	accountAID := meABefore["id"]

	// Authenticated as B, submit A's ID alongside a real mutating field.
	// UpdateProfileRequest has no id/user_id field, so the id is silently
	// ignored by JSON binding -- if the target user were derived from
	// anything other than the bearer token's subject, this bio would land
	// on A's row instead of B's.
	patchRec := integrationDo(t, engine, http.MethodPatch, "/v1/me", sessionB.AccessToken, map[string]any{
		"id":  accountAID,
		"bio": "this change must land on B, never on A",
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch as B with A's id in the body: expected 200, got %d: %s", patchRec.Code, patchRec.Body.String())
	}

	meAAfter := decodeBody(t, integrationDo(t, engine, http.MethodGet, "/v1/me", sessionA.AccessToken, nil))
	if meAAfter["bio"] != nil {
		t.Fatalf("account A's bio was modified by a request authenticated as B: %v", meAAfter["bio"])
	}
	if meAAfter["id"] != accountAID {
		t.Fatalf("account A's id changed across requests: before=%v after=%v", accountAID, meAAfter["id"])
	}

	meB := decodeBody(t, integrationDo(t, engine, http.MethodGet, "/v1/me", sessionB.AccessToken, nil))
	if meB["bio"] != "this change must land on B, never on A" {
		t.Fatalf("expected B's own bio to be updated, got: %v", meB["bio"])
	}
	if meB["id"] == accountAID {
		t.Fatalf("B's id in the response equals A's id -- the wrong row was returned or updated")
	}

	// B may not save A's avatar folder or an external image either.
	idA, _ := accountAID.(string)
	stolen := integrationDo(t, engine, http.MethodPatch, "/v1/me", sessionB.AccessToken, map[string]any{
		"avatar_url": testAvatarBaseURL + "/avatars/" + idA + "/x.png",
	})
	if stolen.Code != http.StatusBadRequest || decodeBody(t, stolen)["field"] != "avatar_url" {
		t.Fatalf("A's avatar folder: expected 400 avatar_url, got %d: %s", stolen.Code, stolen.Body.String())
	}
	external := integrationDo(t, engine, http.MethodPatch, "/v1/me", sessionB.AccessToken, map[string]any{
		"avatar_url": "https://evil.example.org/x.png",
	})
	if external.Code != http.StatusBadRequest || decodeBody(t, external)["field"] != "avatar_url" {
		t.Fatalf("external avatar: expected 400 avatar_url, got %d: %s", external.Code, external.Body.String())
	}
}

// TestRefreshTokenSingleUse refreshes once, then replays the original
// refresh token and asserts it is now rejected -- RefreshService.Redeem's
// rotate-on-use guarantee (plan 01-06), proven here over HTTP rather than
// only at the service layer.
func TestRefreshTokenSingleUse(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, "single-use-refresh@example.com", "correct-horse-battery-staple", "Single Use", "single_use_user")

	firstRefreshRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if firstRefreshRec.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d: %s", firstRefreshRec.Code, firstRefreshRec.Body.String())
	}

	replayRec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{
		"refresh_token": session.RefreshToken,
	})
	if replayRec.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh token: expected 401, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
	if got := decodeBody(t, replayRec)["error"]; got != "token_invalid" {
		t.Fatalf("expected error=token_invalid on replay, got %v", got)
	}
}

// --- a refused sign-up stores nothing ---

func TestIntegrationSignup_UnderThirteenStoresNothing(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)

	rec := integrationDo(t, server.Engine(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"birthday": birthdayYearsAgo(12)}))

	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["error"] != "under_minimum_age" {
		t.Fatalf("expected 403 under_minimum_age, got %d: %s", rec.Code, rec.Body.String())
	}
	if n := integrationUserCount(t, pool); n != 0 {
		t.Fatalf("an under-13 sign-up left %d account(s)", n)
	}
}

func TestIntegrationSignup_EmailTakenStoresNothingNew(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()
	integrationOnboardAccount(t, engine, "taken@example.com", "correct-horse-battery-staple", "First Person", "first_person")

	rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
		"email": "TAKEN@example.com", "username": "second_person",
	}))

	if rec.Code != http.StatusConflict || decodeBody(t, rec)["error"] != "email_taken" {
		t.Fatalf("expected 409 email_taken, got %d: %s", rec.Code, rec.Body.String())
	}
	if n := integrationUserCount(t, pool); n != 1 {
		t.Fatalf("account count = %d, want 1", n)
	}
}

func TestIntegrationSignup_UsernameTakenStoresNothingNew(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()
	integrationOnboardAccount(t, engine, "holder@example.com", "correct-horse-battery-staple", "Holder", "held_name")

	rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
		"email": "wants-it@example.com", "username": "HELD_NAME",
	}))
	// Usernames are lowercase only, so the uppercase spelling is a 400 first.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an uppercase username, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
		"email": "wants-it@example.com", "username": "held_name",
	}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != "username_taken" {
		t.Fatalf("expected username_taken, got %v", body["error"])
	}
	if suggestions, _ := body["suggestions"].([]any); len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %v", body["suggestions"])
	}
	if n := integrationUserCount(t, pool); n != 1 {
		t.Fatalf("account count = %d, want 1 (the refused sign-up must leave no row)", n)
	}
}

// Two sign-ups with different emails race for one username. The unique index
// decides: exactly one wins, the other is 409 username_taken, and the loser
// leaves no row behind.
func TestIntegrationSignup_UsernameRaceLeavesExactlyOneAccount(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	const racers = 6
	codes := make([]int, racers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
				"email":    fmt.Sprintf("racer%d@example.com", i),
				"username": "race_winner",
			}))
			codes[i] = rec.Code
		}(i)
	}
	close(start)
	wg.Wait()

	wins, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}
	if wins != 1 || conflicts != racers-1 {
		t.Fatalf("wins = %d, conflicts = %d, want 1 and %d (%v)", wins, conflicts, racers-1, codes)
	}
	if n := integrationUserCount(t, pool); n != 1 {
		t.Fatalf("account count = %d, want 1", n)
	}
}

// --- the birthday stays private ---

// TestIntegrationNoResponseCarriesABirthdayKey walks every response (success
// and error, including the 409 username_taken body) after requests that set a
// birthday, against the real repositories.
func TestIntegrationNoResponseCarriesABirthdayKey(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, google := buildIntegrationServer(t, pool)
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, "privacy@example.com", "correct-horse-battery-staple", "Privacy Person", "privacy_person")
	steps := []struct {
		label, method, path, token string
		body                       any
	}{
		{"login", http.MethodPost, "/v1/auth/login", "", map[string]any{"email": "privacy@example.com", "password": "correct-horse-battery-staple"}},
		{"me", http.MethodGet, "/v1/me", session.AccessToken, nil},
		{"patch bio", http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"bio": "hi"}},
		{"patch birthday on a complete account", http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"birthday": validBirthday}},
		{"409 username_taken", http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"email": "dup@example.com", "username": "privacy_person"})},
		{"409 email_taken", http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"email": "privacy@example.com", "username": "other_one"})},
		{"400 birthday", http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"birthday": "2001-02-29", "email": "x1@example.com"})},
		{"403 under 13", http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"birthday": birthdayYearsAgo(11), "email": "x2@example.com"})},
		{"available taken", http.MethodGet, "/v1/usernames/available?username=privacy_person", "", nil},
		{"refresh", http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": session.RefreshToken}},
	}
	for _, st := range steps {
		integrationDo(t, engine, st.method, st.path, st.token, st.body)
	}

	// A social account sets its birthday, then finishes.
	google.identity = &auth.GoogleIdentity{Subject: "g-int-privacy", Email: "social-privacy@example.com", EmailVerified: true}
	oauth := integrationDo(t, engine, http.MethodPost, "/v1/auth/oauth/google", "", map[string]any{"id_token": "t"})
	if oauth.Code != http.StatusOK {
		t.Fatalf("oauth: %d %s", oauth.Code, oauth.Body.String())
	}
	socialToken, _ := decodeBody(t, oauth)["access_token"].(string)
	for _, body := range []map[string]any{
		{"birthday": "1995-05-05"},
		{"birthday": "1996-06-06"},
		{"name": "Social Privacy", "username": "social_privacy"},
	} {
		rec := integrationDo(t, engine, http.MethodPatch, "/v1/me", socialToken, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("social patch %v: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	var stored string
	if err := pool.QueryRow(context.Background(), `select birthday::text from users where email = 'social-privacy@example.com'`).Scan(&stored); err != nil || stored != "1995-05-05" {
		t.Fatalf("social birthday stored = %q (err %v), want the first value 1995-05-05", stored, err)
	}
}

// --- Apple and Google accounts: the birthday and the under-13 delete ---

func integrationSocialSession(t *testing.T, engine *gin.Engine, google *fakeGoogleVerifier, subject, email string) integrationSession {
	t.Helper()
	google.identity = &auth.GoogleIdentity{Subject: subject, Email: email, EmailVerified: true}
	rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/oauth/google", "", map[string]any{"id_token": "t"})
	if rec.Code != http.StatusOK {
		t.Fatalf("oauth: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	access, _ := body["access_token"].(string)
	refresh, _ := body["refresh_token"].(string)
	return integrationSession{AccessToken: access, RefreshToken: refresh}
}

func TestIntegrationSocial_UnderThirteenDeletesTheAccountAndItsSessions(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, google := buildIntegrationServer(t, pool)
	engine := server.Engine()
	session := integrationSocialSession(t, engine, google, "g-kid", "kid@example.com")

	rec := integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{
		"birthday": birthdayYearsAgo(12), "name": "Kid", "username": "kid_user", "bio": "nope",
	})

	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["error"] != "under_minimum_age" {
		t.Fatalf("expected 403 under_minimum_age, got %d: %s", rec.Code, rec.Body.String())
	}
	if n := integrationUserCount(t, pool); n != 0 {
		t.Fatalf("account count = %d, want 0", n)
	}
	var tokens int
	if err := pool.QueryRow(context.Background(), `select count(*) from refresh_tokens`).Scan(&tokens); err != nil || tokens != 0 {
		t.Fatalf("refresh tokens left = %d (err %v), want 0", tokens, err)
	}
	refresh := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": session.RefreshToken})
	if refresh.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after deletion: expected 401, got %d: %s", refresh.Code, refresh.Body.String())
	}
	for _, p := range []struct{ method, path string }{{http.MethodGet, "/v1/me"}, {http.MethodPost, "/v1/me/avatar/upload-url"}} {
		var body any
		if p.method == http.MethodPost {
			body = map[string]any{"content_type": "image/png", "content_length": 100}
		}
		got := integrationDo(t, engine, p.method, p.path, session.AccessToken, body)
		if got.Code != http.StatusUnauthorized || decodeBody(t, got)["error"] != "token_invalid" {
			t.Fatalf("%s %s with a deleted account's token: expected 401 token_invalid, got %d: %s", p.method, p.path, got.Code, got.Body.String())
		}
	}
}

func TestIntegrationSocial_BirthdayIsSetOnceAndGatesFinishing(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, google := buildIntegrationServer(t, pool)
	engine := server.Engine()
	session := integrationSocialSession(t, engine, google, "g-once", "once@example.com")

	// Cannot finish without a birthday.
	rec := integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"name": "Once Person", "username": "once_person"})
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["field"] != "birthday" {
		t.Fatalf("finish without a birthday: expected 400 birthday, got %d: %s", rec.Code, rec.Body.String())
	}

	// Birthday alone: 200 and still incomplete.
	rec = integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"birthday": "1994-04-04"})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["onboarding_complete"] != false {
		t.Fatalf("birthday alone: %d %s", rec.Code, rec.Body.String())
	}

	// A different birthday later is ignored (even an under-13 one).
	for _, again := range []string{"1980-01-01", birthdayYearsAgo(10)} {
		rec = integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"birthday": again})
		if rec.Code != http.StatusOK {
			t.Fatalf("second birthday %q: %d %s", again, rec.Code, rec.Body.String())
		}
	}
	var stored string
	if err := pool.QueryRow(context.Background(), `select birthday::text from users where email = 'once@example.com'`).Scan(&stored); err != nil || stored != "1994-04-04" {
		t.Fatalf("stored birthday = %q (err %v), want 1994-04-04", stored, err)
	}

	// Now it can finish, and a finished account takes no birthday.
	rec = integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"name": "Once Person", "username": "once_person"})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["onboarding_complete"] != true {
		t.Fatalf("finish: %d %s", rec.Code, rec.Body.String())
	}
	rec = integrationDo(t, engine, http.MethodPatch, "/v1/me", session.AccessToken, map[string]any{"birthday": "1990-01-01"})
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["field"] != "birthday" {
		t.Fatalf("birthday on a finished account: expected 400 birthday, got %d: %s", rec.Code, rec.Body.String())
	}
}

// A finished account with no birthday on file (like the three accounts that
// existed before this plan) edits name, bio and avatar freely and is refused
// only when it sends a birthday.
func TestIntegrationLegacyAccountWithNullBirthdayCanStillEdit(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, _ := buildIntegrationServer(t, pool)
	engine := server.Engine()

	hash, err := auth.HashPassword("legacy-password-1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	var id uuid.UUID
	err = pool.QueryRow(context.Background(), `
		insert into users (email, password_hash, name, username, email_verified, email_verified_via)
		values ('legacy@example.com', $1, 'Legacy Person', 'legacy_person', true, 'password_flow') returning id`, hash).Scan(&id)
	if err != nil {
		t.Fatalf("seed legacy account: %v", err)
	}

	login := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"email": "legacy@example.com", "password": "legacy-password-1"})
	if login.Code != http.StatusOK {
		t.Fatalf("legacy login: %d %s", login.Code, login.Body.String())
	}
	token, _ := decodeBody(t, login)["access_token"].(string)

	own := testAvatarBaseURL + "/avatars/" + id.String() + "/legacy.png"
	rec := integrationDo(t, engine, http.MethodPatch, "/v1/me", token, map[string]any{"name": "Legacy Renamed", "bio": "edited", "avatar_url": own})
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy edit: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["name"] != "Legacy Renamed" || body["bio"] != "edited" || body["avatar_url"] != own || body["onboarding_complete"] != true {
		t.Fatalf("unexpected body %v", body)
	}
	rec = integrationDo(t, engine, http.MethodPatch, "/v1/me", token, map[string]any{"birthday": validBirthday})
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["field"] != "birthday" {
		t.Fatalf("legacy account sending a birthday: expected 400 birthday, got %d: %s", rec.Code, rec.Body.String())
	}
	var hasBirthday bool
	if err := pool.QueryRow(context.Background(), `select birthday is not null from users where id = $1`, id).Scan(&hasBirthday); err != nil || hasBirthday {
		t.Fatalf("legacy account gained a birthday (has = %v, err %v)", hasBirthday, err)
	}
}
