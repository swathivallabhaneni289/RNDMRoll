package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// --- shared test helpers ---

// newAuthTestHandler builds a fresh router, handler, and set of in-memory
// fakes for a single test. Each test gets its own instance -- sharing one
// router across a table-driven test would let the signup/login rate limits
// (3/min and 5/min respectively) reject later subtests.
func newAuthTestHandler(t *testing.T) (*gin.Engine, *AuthHandler, TestDeps) {
	t.Helper()
	deps := TestDeps{
		Users:         newFakeUserRepo(),
		RefreshTokens: newFakeRefreshRepo(),
		Verifications: newFakeVerificationRepo(),
		Mailer:        newFakeMailer(),
	}
	router := newTestRouter(t, deps)
	refreshSvc := auth.NewRefreshService(deps.RefreshTokens, 30*24*time.Hour)
	handler := NewAuthHandler(deps.Users, refreshSvc, deps.Verifications, deps.Mailer, []byte("test-secret"), 15*time.Minute)
	rg := router.Group("/v1")
	handler.Register(rg)
	return router, handler, deps
}

func doJSONRequest(t *testing.T, router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response body %q: %v", rec.Body.String(), err)
	}
	return body
}

func assertValidationFailed(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != "validation_failed" {
		t.Fatalf("expected error=validation_failed, got %v", body["error"])
	}
}

func assertInvalidCredentials(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != "invalid_credentials" {
		t.Fatalf("expected error=invalid_credentials, got %v", body["error"])
	}
}

// --- Task 1: Signup and login handlers ---

func TestSignup_ValidBody_Returns201AndHashesPassword(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "new-user@example.com",
		Password: "correct-horse-battery",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if _, ok := body["user_id"]; !ok {
		t.Fatalf("expected user_id in response, got %v", body)
	}

	stored, err := deps.Users.GetByEmailCI(context.Background(), "new-user@example.com")
	if err != nil {
		t.Fatalf("expected user to be stored: %v", err)
	}
	if stored.PasswordHash == nil || *stored.PasswordHash == "" {
		t.Fatal("expected password hash to be set")
	}
	if *stored.PasswordHash == "correct-horse-battery" {
		t.Fatal("expected stored password hash to differ from the plaintext password")
	}
}

func TestSignup_SetsEmailUnverifiedAndSendsVerificationEmailOnce(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "verify-me@example.com",
		Password: "correct-horse-battery",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := deps.Users.GetByEmailCI(context.Background(), "verify-me@example.com")
	if err != nil {
		t.Fatalf("expected user to be stored: %v", err)
	}
	if stored.EmailVerified {
		t.Fatal("expected email_verified to be false immediately after signup")
	}

	fm, ok := deps.Mailer.(*fakeMailer)
	if !ok {
		t.Fatalf("expected deps.Mailer to be *fakeMailer, got %T", deps.Mailer)
	}
	fm.mu.Lock()
	sentCount := len(fm.sent)
	fm.mu.Unlock()
	if sentCount != 1 {
		t.Fatalf("expected exactly 1 verification email sent, got %d", sentCount)
	}
}

func TestSignup_PasswordUnder8Bytes_Returns400ValidationFailed(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "short-pw@example.com",
		Password: "short1",
	})

	assertValidationFailed(t, rec)
}

func TestSignup_PasswordOver72Bytes_Returns400ValidationFailed(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "long-pw@example.com",
		Password: strings.Repeat("a", 73),
	})

	assertValidationFailed(t, rec)
}

func TestSignup_DuplicateEmailDifferentCase_Returns409EmailTaken(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	first := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "Dup@Example.com",
		Password: "correct-horse-battery",
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first signup to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", SignupRequest{
		Email:    "dup@example.com",
		Password: "another-password",
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", second.Code, second.Body.String())
	}
	body := decodeBody(t, second)
	if body["error"] != "email_taken" {
		t.Fatalf("expected error=email_taken, got %v", body["error"])
	}
}

func TestLogin_CorrectCredentials_Returns200WithTokensAndUser(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	password := "correct-horse-battery"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	via := user.VerifiedViaPasswordFlow
	if _, err := deps.Users.Create(context.Background(), "login-ok@example.com", &hash, true, &via); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "login-ok@example.com",
		Password: password,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	for _, field := range []string{"access_token", "refresh_token", "expires_in", "user"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected %s in response, got %v", field, body)
		}
	}
}

func TestLogin_WrongPassword_Returns401InvalidCredentials(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	via := user.VerifiedViaPasswordFlow
	if _, err := deps.Users.Create(context.Background(), "wrongpw@example.com", &hash, true, &via); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "wrongpw@example.com",
		Password: "incorrect-password",
	})

	assertInvalidCredentials(t, rec)
}

func TestLogin_UnknownAccountByteIdenticalToWrongPassword(t *testing.T) {
	routerA, _, depsA := newAuthTestHandler(t)
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	via := user.VerifiedViaPasswordFlow
	if _, err := depsA.Users.Create(context.Background(), "exists@example.com", &hash, true, &via); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	wrongPasswordRec := doJSONRequest(t, routerA, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "exists@example.com",
		Password: "incorrect-password",
	})

	routerB, _, _ := newAuthTestHandler(t)
	unknownAccountRec := doJSONRequest(t, routerB, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "does-not-exist@example.com",
		Password: "incorrect-password",
	})

	if wrongPasswordRec.Code != unknownAccountRec.Code {
		t.Fatalf("expected identical status codes, got %d (wrong password) vs %d (unknown account)", wrongPasswordRec.Code, unknownAccountRec.Code)
	}
	if wrongPasswordRec.Body.String() != unknownAccountRec.Body.String() {
		t.Fatalf("expected byte-identical bodies, got %q (wrong password) vs %q (unknown account)", wrongPasswordRec.Body.String(), unknownAccountRec.Body.String())
	}
}

func TestLogin_UnverifiedAccount_Returns403EmailNotVerified(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := deps.Users.Create(context.Background(), "unverified@example.com", &hash, false, nil); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "unverified@example.com",
		Password: "correct-password",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != "email_not_verified" {
		t.Fatalf("expected error=email_not_verified, got %v", body["error"])
	}
}

// --- Task 2: Refresh and logout handlers ---

// loginAndGetTokens seeds a verified user and performs a real login over
// the router, returning the issued access and refresh tokens.
func loginAndGetTokens(t *testing.T, router *gin.Engine, deps TestDeps, email, password string) (accessToken, refreshToken string) {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	via := user.VerifiedViaPasswordFlow
	if _, err := deps.Users.Create(context.Background(), email, &hash, true, &via); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{Email: email, Password: password})
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	accessToken, _ = body["access_token"].(string)
	refreshToken, _ = body["refresh_token"].(string)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("expected non-empty tokens from login, got %v", body)
	}
	return accessToken, refreshToken
}

func TestRefresh_ValidToken_Returns200WithNewTokenPairDifferentFromPresented(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	_, refreshToken := loginAndGetTokens(t, router, deps, "refresh-me@example.com", "correct-password")

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	for _, field := range []string{"access_token", "refresh_token", "expires_in"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected %s in response, got %v", field, body)
		}
	}
	newRefreshToken, _ := body["refresh_token"].(string)
	if newRefreshToken == refreshToken {
		t.Fatal("expected the rotated refresh token to differ from the presented token")
	}
}

func TestRefresh_SecondUseOfSameToken_Returns401TokenInvalid(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	_, refreshToken := loginAndGetTokens(t, router, deps, "reuse@example.com", "correct-password")

	first := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	if first.Code != http.StatusOK {
		t.Fatalf("expected first refresh to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on second use, got %d: %s", second.Code, second.Body.String())
	}
	body := decodeBody(t, second)
	if body["error"] != "token_invalid" {
		t.Fatalf("expected error=token_invalid, got %v", body["error"])
	}
}

func TestRefresh_ExpiredToken_Returns401TokenExpired(t *testing.T) {
	deps := TestDeps{
		Users:         newFakeUserRepo(),
		RefreshTokens: newFakeRefreshRepo(),
		Verifications: newFakeVerificationRepo(),
		Mailer:        newFakeMailer(),
	}
	router := newTestRouter(t, deps)
	expiredRefreshSvc := auth.NewRefreshService(deps.RefreshTokens, -time.Minute)
	handler := NewAuthHandler(deps.Users, expiredRefreshSvc, deps.Verifications, deps.Mailer, []byte("test-secret"), 15*time.Minute)
	rg := router.Group("/v1")
	handler.Register(rg)

	expiredToken, err := expiredRefreshSvc.Issue(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("issue expired token: %v", err)
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", RefreshRequest{RefreshToken: expiredToken})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != "token_expired" {
		t.Fatalf("expected error=token_expired, got %v", body["error"])
	}
}

func TestRefresh_MissingRefreshToken_Returns400ValidationFailed(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", map[string]string{})

	assertValidationFailed(t, rec)
}

func TestLogout_RevokesToken_SubsequentRefreshReturns401(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	_, refreshToken := loginAndGetTokens(t, router, deps, "logout-me@example.com", "correct-password")

	logoutRec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/logout", LogoutRequest{RefreshToken: refreshToken})
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 refreshing a revoked token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogout_AlreadyInvalidToken_Returns204(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/logout", LogoutRequest{RefreshToken: "not-a-real-token"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogout_MissingRefreshToken_Returns400ValidationFailed(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/logout", map[string]string{})

	assertValidationFailed(t, rec)
}
