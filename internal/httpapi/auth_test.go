package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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
// (3/min per address and email, 5/min on login) reject later subtests.
func newAuthTestHandler(t *testing.T) (*gin.Engine, *AuthHandler, TestDeps) {
	t.Helper()
	deps := TestDeps{
		Users:         newFakeUserRepo(),
		RefreshTokens: newFakeRefreshRepo(),
	}
	router := newTestRouter(t, deps)
	refreshSvc := auth.NewRefreshService(deps.RefreshTokens, 30*24*time.Hour)
	handler := NewAuthHandler(deps.Users, refreshSvc, []byte("test-secret"), 15*time.Minute)
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

// --- Signup ---

// validBirthday is a fixed, clearly adult date used by signup bodies.
const validBirthday = "1990-06-15"

// signupBody returns a valid sign-up body with the given fields replaced. A
// nil value removes the key.
func signupBody(mods map[string]any) map[string]any {
	body := map[string]any{
		"email":    "new-user@example.com",
		"password": "correct-horse-battery",
		"birthday": validBirthday,
		"name":     "New User",
		"username": "new_user",
	}
	for k, v := range mods {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	return body
}

// birthdayYearsAgo returns the YYYY-MM-DD date exactly n years before the
// current UTC date. On 29 February in a year where n years ago has no such
// day, it uses 28 February of that year, so the person is still at least n
// (never a day short, which AddDate's roll to 1 March would make them).
func birthdayYearsAgo(n int) string {
	now := time.Now().UTC()
	d := time.Date(now.Year()-n, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if d.Month() != now.Month() {
		d = time.Date(now.Year()-n, now.Month(), 28, 0, 0, 0, 0, time.UTC)
	}
	return d.Format("2006-01-02")
}

func assertFieldError(t *testing.T, rec *httptest.ResponseRecorder, field string) {
	t.Helper()
	assertValidationFailed(t, rec)
	body := decodeBody(t, rec)
	if body["field"] != field {
		t.Fatalf("expected field=%q, got %v (body %s)", field, body["field"], rec.Body.String())
	}
}

func TestSignup_ValidBody_Returns200WithASessionAndStoresEverything(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"bio": "hello there"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	for _, field := range []string{"access_token", "refresh_token", "expires_in", "user"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected %s in response, got %v", field, body)
		}
	}
	userBody, _ := body["user"].(map[string]any)
	if userBody["email"] != "new-user@example.com" || userBody["name"] != "New User" || userBody["username"] != "new_user" || userBody["bio"] != "hello there" {
		t.Errorf("unexpected user body: %v", userBody)
	}
	if userBody["email_verified"] != false {
		t.Errorf("email_verified = %v, want false (no email check)", userBody["email_verified"])
	}
	if userBody["onboarding_complete"] != true {
		t.Errorf("onboarding_complete = %v, want true", userBody["onboarding_complete"])
	}

	stored, err := deps.Users.GetByEmailCI(context.Background(), "new-user@example.com")
	if err != nil {
		t.Fatalf("expected user to be stored: %v", err)
	}
	if stored.PasswordHash == nil || *stored.PasswordHash == "" || *stored.PasswordHash == "correct-horse-battery" {
		t.Fatal("expected a bcrypt hash, not the plaintext password")
	}
	if stored.EmailVerified {
		t.Fatal("expected email_verified to be false after signup")
	}
	if !stored.HasBirthday {
		t.Fatal("expected a birthday on file")
	}
	if got := deps.Users.(*fakeUserRepo).storedBirthday(stored.ID); got != validBirthday {
		t.Fatalf("stored birthday = %q, want %q", got, validBirthday)
	}
}

func TestSignup_ResponseHasTheSameKeysAsLogin(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	signup := decodeBody(t, doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(nil)))
	login := decodeBody(t, doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "new-user@example.com",
		Password: "correct-horse-battery",
	}))

	keys := func(m map[string]any) string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	if keys(signup) != keys(login) {
		t.Fatalf("signup keys %q differ from login keys %q", keys(signup), keys(login))
	}
	su, _ := signup["user"].(map[string]any)
	lu, _ := login["user"].(map[string]any)
	if keys(su) != keys(lu) {
		t.Fatalf("signup user keys %q differ from login user keys %q", keys(su), keys(lu))
	}
}

func TestSignup_TrimsNameAndStoresEmptyBioAsNone(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"name": "  Padded Name  ", "bio": "   "}))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	stored, err := deps.Users.GetByEmailCI(context.Background(), "new-user@example.com")
	if err != nil {
		t.Fatalf("expected user to be stored: %v", err)
	}
	if stored.Name == nil || *stored.Name != "Padded Name" {
		t.Errorf("name = %v, want trimmed", stored.Name)
	}
	if stored.Bio != nil {
		t.Errorf("bio = %q, want none", *stored.Bio)
	}
}

func TestSignup_BadFieldsReturn400NamingTheFieldAndStoreNothing(t *testing.T) {
	cases := []struct {
		name  string
		mods  map[string]any
		field string
	}{
		{"email missing", map[string]any{"email": nil}, "email"},
		{"email malformed", map[string]any{"email": "not-an-email"}, "email"},
		{"email with display name", map[string]any{"email": "Bob <bob@example.com>"}, "email"},
		{"email over 254", map[string]any{"email": strings.Repeat("a", 250) + "@example.com"}, "email"},
		{"password missing", map[string]any{"password": nil}, "password"},
		{"password 7 characters", map[string]any{"password": "short12"}, "password"},
		{"password 73 characters", map[string]any{"password": strings.Repeat("a", 73)}, "password"},
		{"password 80 bytes in 40 runes", map[string]any{"password": strings.Repeat("\u00e9", 40)}, "password"},
		{"birthday missing", map[string]any{"birthday": nil}, "birthday"},
		{"birthday wrong format", map[string]any{"birthday": "06/15/1990"}, "birthday"},
		{"birthday nonexistent", map[string]any{"birthday": "2001-02-29"}, "birthday"},
		{"birthday before 1900", map[string]any{"birthday": "1899-12-31"}, "birthday"},
		{"birthday in the future", map[string]any{"birthday": "2999-01-01"}, "birthday"},
		{"name missing", map[string]any{"name": nil}, "name"},
		{"name blank", map[string]any{"name": "   "}, "name"},
		{"name over 50", map[string]any{"name": strings.Repeat("n", 51)}, "name"},
		{"name with NUL", map[string]any{"name": "a\u0000b"}, "name"},
		{"name with control character", map[string]any{"name": "a\u0007b"}, "name"},
		{"bio over 160", map[string]any{"bio": strings.Repeat("b", 161)}, "bio"},
		{"bio with NUL", map[string]any{"bio": "a\u0000b"}, "bio"},
		{"username missing", map[string]any{"username": nil}, "username"},
		{"username uppercase", map[string]any{"username": "BadName"}, "username"},
		{"username too short", map[string]any{"username": "ab"}, "username"},
		{"username too long", map[string]any{"username": strings.Repeat("u", 21)}, "username"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, deps := newAuthTestHandler(t)
			rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(tc.mods))
			assertFieldError(t, rec, tc.field)
			if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
				t.Fatalf("a rejected signup stored %d account(s)", n)
			}
		})
	}
}

func TestSignup_MalformedJSONGetsFixedMessageNotDecoderText(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)
	for name, raw := range map[string]string{
		"syntax error": `{"email": zq-sentinel}`,
		"wrong type":   `{"birthday": 12345}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, "zq-sentinel") || strings.Contains(body, "SignupRequest") || strings.Contains(body, "invalid character") {
			t.Fatalf("%s: body echoes decoder text: %s", name, body)
		}
	}
}

func TestVerifyEmailRoutesAreGone(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/verify-"+"email", map[string]any{"code": "123456"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignup_ErrorBodiesNeverEchoInputOrDatabaseText(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)
	secret := "sentinel-input-value"
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"username": secret}))
	assertFieldError(t, rec, "username")
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("error body echoes the input: %s", rec.Body.String())
	}
}

func TestSignup_UnderThirteenIs403AndStoresNothing(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"birthday": birthdayYearsAgo(12)}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["error"] != "under_minimum_age" {
		t.Fatalf("expected under_minimum_age, got %v", body["error"])
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
		t.Fatalf("an under-13 signup stored %d account(s)", n)
	}
}

func TestSignup_UnderThirteenIsJudgedBeforeTheUsername(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{
		"birthday": birthdayYearsAgo(12),
		"username": "BAD NAME",
	}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 before the username is looked at, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignup_ExactlyThirteenToday_IsAccepted(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"birthday": birthdayYearsAgo(13)}))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for someone who turns 13 today, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignup_DayBeforeThirteenth_IsRefused(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	// One day short of 13: the day after the date exactly 13 years ago.
	exact, _ := time.Parse("2006-01-02", birthdayYearsAgo(13))
	birthday := exact.AddDate(0, 0, 1).Format("2006-01-02")

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"birthday": birthday}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
		t.Fatalf("stored %d account(s)", n)
	}
}

func TestSignup_TomorrowAsBirthdayStoresNothing(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"birthday": tomorrow}))

	// Tomorrow's date is either "in the future" (400) or, inside the UTC+14
	// grace window, a date that makes the person under 13 (403). Both refuse.
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 400 or 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
		t.Fatalf("stored %d account(s)", n)
	}
}

func TestSignup_DuplicateEmailDifferentCase_Returns409EmailTakenAndStoresNothingNew(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	first := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"email": "Dup@Example.com"}))
	if first.Code != http.StatusOK {
		t.Fatalf("expected first signup to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{
		"email":    "dup@example.com",
		"username": "other_name",
	}))
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", second.Code, second.Body.String())
	}
	body := decodeBody(t, second)
	if body["error"] != "email_taken" {
		t.Fatalf("expected error=email_taken, got %v", body["error"])
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 1 {
		t.Fatalf("account count = %d, want 1", n)
	}
}

func TestSignup_DuplicateUsername_Returns409WithSuggestionsAndStoresNothingNew(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)

	first := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(nil))
	if first.Code != http.StatusOK {
		t.Fatalf("expected first signup to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(map[string]any{"email": "second@example.com"}))
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", second.Code, second.Body.String())
	}
	body := decodeBody(t, second)
	if body["error"] != "username_taken" {
		t.Fatalf("expected error=username_taken, got %v", body["error"])
	}
	suggestions, _ := body["suggestions"].([]any)
	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %v", body["suggestions"])
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 1 {
		t.Fatalf("account count = %d, want 1", n)
	}
	if _, err := deps.Users.GetByEmailCI(context.Background(), "second@example.com"); err == nil {
		t.Fatal("the refused signup left an account behind")
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

func TestLogin_UnverifiedAccount_LogsIn(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for an unverified account, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if _, ok := body["access_token"].(string); !ok {
		t.Fatalf("expected an access_token, got %v", body)
	}
}

func TestLogin_ShortWrongPassword_Returns401InvalidCredentialsNot400(t *testing.T) {
	router, _, deps := newAuthTestHandler(t)
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := deps.Users.Create(context.Background(), "shortwrong@example.com", &hash, true, nil); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "shortwrong@example.com",
		Password: "abcde",
	})

	assertInvalidCredentials(t, rec)
}

func TestLogin_PasswordOver72_Returns400(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/login", LoginRequest{
		Email:    "anyone@example.com",
		Password: strings.Repeat("a", 73),
	})

	assertValidationFailed(t, rec)
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
	}
	router := newTestRouter(t, deps)
	expiredRefreshSvc := auth.NewRefreshService(deps.RefreshTokens, -time.Minute)
	handler := NewAuthHandler(deps.Users, expiredRefreshSvc, []byte("test-secret"), 15*time.Minute)
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
