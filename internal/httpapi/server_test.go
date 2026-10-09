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

// testAvatarBaseURL is the storage public base URL the test servers use; it
// matches the PublicURL prefix fakeAvatarStore hands out.
const testAvatarBaseURL = "https://cdn.example.test"

// panicIfTouchedUserRepo embeds a nil user.Repository: any method call
// reaches the nil interface and panics. Used to prove a route never
// touches the database, rather than merely asserting a status code that
// could pass for the wrong reason.
type panicIfTouchedUserRepo struct {
	user.Repository
}

// fullTestRig is a Server wired exactly like cmd/api/main.go, using
// in-memory fakes for every dependency so no test here reaches a real
// database or network.
type fullTestRig struct {
	srv     *Server
	users   user.Repository
	refresh *auth.RefreshService
}

func newFullTestRig(t *testing.T, usersRepo user.Repository) *fullTestRig {
	t.Helper()

	tokens := newFakeRefreshRepo()
	if fake, ok := usersRepo.(*fakeUserRepo); ok {
		fake.tokens = tokens
	}
	refreshSvc := auth.NewRefreshService(tokens, 30*24*time.Hour)

	authHandler := NewAuthHandler(usersRepo, refreshSvc, testServerJWTSecret, 15*time.Minute)
	profileHandler := NewProfileHandler(usersRepo, &fakeAvatarStore{}, testAvatarBaseURL)
	usernameHandler := NewUsernameHandler(usersRepo)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &fullTestRig{
		srv: NewServer(Deps{
			Auth:      authHandler,
			Profile:   profileHandler,
			Username:  usernameHandler,
			Users:     usersRepo,
			JWTSecret: testServerJWTSecret,
			Logger:    logger,
		}),
		users:   usersRepo,
		refresh: refreshSvc,
	}
}

func newFullTestServer(t *testing.T, usersRepo user.Repository) *Server {
	t.Helper()
	return newFullTestRig(t, usersRepo).srv
}

// serveFrom sends one request through srv as if it came from remote (a
// socket address such as "198.51.100.7:4000"), with an optional bearer
// token and JSON body. Using a distinct remote per request keeps the
// per-address rate limits out of the way of tests that make many calls.
func serveFrom(t *testing.T, srv *Server, remote, method, path, token string, body any) *httptest.ResponseRecorder {
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
	req.RemoteAddr = remote
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Engine().ServeHTTP(rec, req)
	return rec
}

// remoteN returns a distinct documentation-range socket address for n.
func remoteN(n int) string {
	return fmt.Sprintf("203.0.113.%d:%d", n%250+1, 40000+n)
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

	w := serveFrom(t, srv, remoteN(1), http.MethodPost, "/v1/auth/signup", "", signupBody(nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected signup to succeed without a token, got %d: %s", w.Code, w.Body.String())
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

func TestServer_UnverifiedTokenReachesMe(t *testing.T) {
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

	w := serveFrom(t, srv, remoteN(3), http.MethodGet, "/v1/me", token, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for an unverified account, got %d: %s", w.Code, w.Body.String())
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

	w := serveFrom(t, srv, remoteN(4), http.MethodGet, "/v1/me", token, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a verified account, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_SignedUpAccountCanRefreshAndUseMe(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())

	signup := serveFrom(t, srv, remoteN(5), http.MethodPost, "/v1/auth/signup", "", signupBody(nil))
	if signup.Code != http.StatusOK {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body.String())
	}
	refreshToken, _ := decodeBody(t, signup)["refresh_token"].(string)

	refresh := serveFrom(t, srv, remoteN(6), http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refreshToken})
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh of an unverified account: %d %s", refresh.Code, refresh.Body.String())
	}
	access, _ := decodeBody(t, refresh)["access_token"].(string)

	me := serveFrom(t, srv, remoteN(7), http.MethodGet, "/v1/me", access, nil)
	if me.Code != http.StatusOK {
		t.Fatalf("/me: %d %s", me.Code, me.Body.String())
	}
	body := decodeBody(t, me)
	if body["onboarding_complete"] != true || body["email_verified"] != false {
		t.Fatalf("expected complete and unverified, got %v", body)
	}
}

func TestServer_DeletedUsersTokenGets401OnMeAndAvatarUpload(t *testing.T) {
	users := newFakeUserRepo()
	u, err := users.Create(context.Background(), "gone@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	srv := newFullTestServer(t, users)
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	users.remove(t, u.ID)

	paths := []struct{ method, path string }{
		{http.MethodGet, "/v1/me"},
		{http.MethodPatch, "/v1/me"},
		{http.MethodPost, "/v1/me/avatar/upload-url"},
	}
	for i, p := range paths {
		var body any
		if p.method != http.MethodGet {
			body = map[string]any{"bio": "x", "content_type": "image/png", "content_length": 100}
		}
		w := serveFrom(t, srv, remoteN(10+i), p.method, p.path, token, body)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d: %s", p.method, p.path, w.Code, w.Body.String())
		}
		if got := decodeBody(t, w)["error"]; got != "token_invalid" {
			t.Fatalf("%s %s: expected token_invalid, got %v", p.method, p.path, got)
		}
	}
}

// --- request size cap ---

func TestServer_OversizeBodiesAre413OnSignupLoginAndPatchMe(t *testing.T) {
	users := newFakeUserRepo()
	u := users.seedComplete(t, "big@example.com", "Big Body", "big_body", validBirthday)
	srv := newFullTestServer(t, users)
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	big := `{"email":"a@example.com","password":"` + strings.Repeat("a", 20<<10) + `"}`
	targets := []struct {
		method, path, token string
	}{
		{http.MethodPost, "/v1/auth/signup", ""},
		{http.MethodPost, "/v1/auth/login", ""},
		{http.MethodPatch, "/v1/me", token},
	}
	for i, tg := range targets {
		for _, chunked := range []bool{false, true} {
			name := fmt.Sprintf("%s %s chunked=%v", tg.method, tg.path, chunked)
			t.Run(name, func(t *testing.T) {
				var body io.Reader = strings.NewReader(big)
				if chunked {
					// An unknown length: the declared-length check cannot fire,
					// so the reader's own cap must.
					body = io.NopCloser(strings.NewReader(big))
				}
				req := httptest.NewRequest(tg.method, tg.path, body)
				if chunked {
					req.ContentLength = -1
				}
				req.RemoteAddr = remoteN(20 + i)
				req.Header.Set("Content-Type", "application/json")
				if tg.token != "" {
					req.Header.Set("Authorization", "Bearer "+tg.token)
				}
				rec := httptest.NewRecorder()
				srv.Engine().ServeHTTP(rec, req)

				if rec.Code != http.StatusRequestEntityTooLarge {
					t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
				}
				if got := decodeBody(t, rec)["error"]; got != "payload_too_large" {
					t.Fatalf("expected payload_too_large, got %v", got)
				}
			})
		}
	}
}

func TestServer_ABodyUnderTheCapIsNotRefusedAsTooLarge(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	// Roughly 10 KB: well over any real sign-up, under the 16 KB cap.
	body := signupBody(map[string]any{"bio": strings.Repeat("b", 10<<10)})

	w := serveFrom(t, srv, remoteN(30), http.MethodPost, "/v1/auth/signup", "", body)

	assertFieldError(t, w, "bio")
}

// --- per-address limits ---

func TestServer_SpoofedForwardedForStillHitsTheLimitOfTheRealAddress(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	realAddr := "198.51.100.9:5555"

	// The sign-up limit is 10 a minute per address. Each request claims a
	// different client address in X-Forwarded-For and X-Real-IP.
	for i := 0; i < 10; i++ {
		// A different email each time keeps the 3-per-address-and-email limit
		// out of the way; the missing password makes each one a cheap 400.
		payload := fmt.Sprintf(`{"email":"spoof%d@example.com"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", strings.NewReader(payload))
		req.RemoteAddr = realAddr
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.9.8.%d", i+1))
		req.Header.Set("X-Real-IP", fmt.Sprintf("10.7.6.%d", i+1))
		rec := httptest.NewRecorder()
		srv.Engine().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: expected 400, got %d: %s", i+1, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", strings.NewReader(`{"email":"spoof99@example.com"}`))
	req.RemoteAddr = realAddr
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.9.8.200")
	rec := httptest.NewRecorder()
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for the 11th request from one real address, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["error"]; got != "rate_limited" {
		t.Fatalf("expected rate_limited, got %v", got)
	}

	// A different real address is not limited.
	other := serveFrom(t, srv, "198.51.100.10:5555", http.MethodPost, "/v1/auth/signup", "", map[string]any{"email": "spoof100@example.com"})
	if other.Code != http.StatusBadRequest {
		t.Fatalf("a different real address should not be limited, got %d", other.Code)
	}
}

func TestServer_LoginIsLimitedToFivePerMinutePerAddressAndLogin(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	// The missing password makes each attempt a cheap 400; the limits count it
	// all the same.
	for i := 0; i < 5; i++ {
		rec := serveFrom(t, srv, "198.51.100.40:1", http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "Some_Name"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: expected 400, got %d", i+1, rec.Code)
		}
	}
	// The same login in other capitals, and under the older email key, share
	// the bucket.
	for _, body := range []map[string]any{{"login": "some_name"}, {"email": "SOME_NAME"}} {
		rec := serveFrom(t, srv, "198.51.100.40:1", http.MethodPost, "/v1/auth/login", "", body)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on the 6th try for %v, got %d", body, rec.Code)
		}
	}
	// Same address, another login: only the per-address limit applies.
	rec := serveFrom(t, srv, "198.51.100.40:1", http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "other_name"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for another login, got %d", rec.Code)
	}
}

func TestServer_LoginIsLimitedToTwentyPerMinutePerAddress(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	for i := 0; i < 20; i++ {
		rec := serveFrom(t, srv, "198.51.100.41:1", http.MethodPost, "/v1/auth/login", "", map[string]any{"login": fmt.Sprintf("name_%d", i)})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: expected 400, got %d", i+1, rec.Code)
		}
	}
	rec := serveFrom(t, srv, "198.51.100.41:1", http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "name_99"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 21st login from one address, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["error"]; got != "rate_limited" {
		t.Fatalf("expected rate_limited, got %v", got)
	}
	// A different address is not limited.
	other := serveFrom(t, srv, "198.51.100.42:1", http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "name_99"})
	if other.Code != http.StatusBadRequest {
		t.Fatalf("a different address should not be limited, got %d", other.Code)
	}
}

func TestServer_SignupIsLimitedToThreePerMinutePerAddressAndEmail(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	for i := 0; i < 3; i++ {
		rec := serveFrom(t, srv, "198.51.100.20:1", http.MethodPost, "/v1/auth/signup", "", map[string]any{"email": "same@example.com"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: expected 400, got %d", i+1, rec.Code)
		}
	}
	rec := serveFrom(t, srv, "198.51.100.20:1", http.MethodPost, "/v1/auth/signup", "", map[string]any{"email": "same@example.com"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 4th try for one address and email, got %d", rec.Code)
	}
	// Same address, other email: only the 10-per-address limit applies.
	rec = serveFrom(t, srv, "198.51.100.20:1", http.MethodPost, "/v1/auth/signup", "", map[string]any{"email": "different@example.com"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for another email, got %d", rec.Code)
	}
}

// --- public username checks ---

func TestServer_UsernameChecksNeedNoToken(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())

	suggest := serveFrom(t, srv, remoteN(40), http.MethodGet, "/v1/usernames/suggest?name=Ada+Lovelace", "", nil)
	if suggest.Code != http.StatusOK {
		t.Fatalf("suggest without a token: %d %s", suggest.Code, suggest.Body.String())
	}
	available := serveFrom(t, srv, remoteN(41), http.MethodGet, "/v1/usernames/available?username=ada_lovelace", "", nil)
	if available.Code != http.StatusOK {
		t.Fatalf("available without a token: %d %s", available.Code, available.Body.String())
	}
	if decodeBody(t, available)["available"] != true {
		t.Fatalf("expected available=true, got %s", available.Body.String())
	}
}

func TestServer_UsernameChecksRejectOverLongInputBeforeAnyQuery(t *testing.T) {
	// A repo that panics on any call proves no query ran.
	srv := newFullTestServer(t, panicIfTouchedUserRepo{})

	long := strings.Repeat("a", 101)
	suggest := serveFrom(t, srv, remoteN(42), http.MethodGet, "/v1/usernames/suggest?name="+long, "", nil)
	assertFieldError(t, suggest, "name")
	if strings.Contains(suggest.Body.String(), long) {
		t.Fatal("suggest error echoes the input")
	}

	available := serveFrom(t, srv, remoteN(43), http.MethodGet, "/v1/usernames/available?username="+strings.Repeat("a", 21), "", nil)
	assertFieldError(t, available, "username")

	bad := serveFrom(t, srv, remoteN(44), http.MethodGet, "/v1/usernames/available?username=Bad%20Name", "", nil)
	assertFieldError(t, bad, "username")
	if strings.Contains(bad.Body.String(), "Bad") {
		t.Fatal("available error echoes the input")
	}
}

func TestServer_UsernameChecksAreLimitedTo90PerMinutePerAddress(t *testing.T) {
	srv := newFullTestServer(t, newFakeUserRepo())
	for i := 0; i < 90; i++ {
		rec := serveFrom(t, srv, "198.51.100.30:1", http.MethodGet, "/v1/usernames/available?username=free_name", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	rec := serveFrom(t, srv, "198.51.100.30:1", http.MethodGet, "/v1/usernames/available?username=free_name", "", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 91st request, got %d", rec.Code)
	}
}

// --- the birthday never leaves the server ---

// assertNoBirthKey fails if any object key anywhere in the JSON body
// contains "birth" (any case).
func assertNoBirthKey(t *testing.T, label string, body []byte) {
	t.Helper()
	if len(bytes.TrimSpace(body)) == 0 {
		return
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%s: response is not JSON: %v (%s)", label, err, string(body))
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if strings.Contains(strings.ToLower(k), "birth") {
					t.Errorf("%s: response key %q at %q contains \"birth\": %s", label, k, path, string(body))
				}
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range x {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		}
	}
	walk("$", v)
}

func TestServer_NoResponseBodyCarriesABirthdayKey(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	n := 100
	next := func() string { n++; return remoteN(n) }
	check := func(label string, rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
		t.Helper()
		assertNoBirthKey(t, label, rec.Body.Bytes())
		return rec
	}

	// Sign-up that sets a birthday, then everything that can echo a user.
	signup := check("signup", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/signup", "", signupBody(nil)))
	if signup.Code != http.StatusOK {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body.String())
	}
	sBody := decodeBody(t, signup)
	access, _ := sBody["access_token"].(string)
	refreshToken, _ := sBody["refresh_token"].(string)

	check("login", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/login", "", map[string]any{"email": "new-user@example.com", "password": "correct-horse-battery"}))
	check("login wrong", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/login", "", map[string]any{"email": "new-user@example.com", "password": "nope"}))
	check("me", serveFrom(t, rig.srv, next(), http.MethodGet, "/v1/me", access, nil))
	check("patch bio", serveFrom(t, rig.srv, next(), http.MethodPatch, "/v1/me", access, map[string]any{"bio": "hi"}))
	check("patch birthday on a complete account", serveFrom(t, rig.srv, next(), http.MethodPatch, "/v1/me", access, map[string]any{"birthday": validBirthday}))
	check("refresh", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refreshToken}))
	check("avatar upload", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/me/avatar/upload-url", access, map[string]any{"content_type": "image/png", "content_length": 100}))

	// Errors after a request that carried a birthday.
	dupUser := check("409 username_taken", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"email": "second@example.com"})))
	if dupUser.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", dupUser.Code)
	}
	dupEmail := check("409 email_taken", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"username": "another_one"})))
	if dupEmail.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", dupEmail.Code)
	}
	check("400 birthday", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"birthday": "2001-02-29"})))
	check("403 under 13", serveFrom(t, rig.srv, next(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{"birthday": birthdayYearsAgo(12)})))

	// Username endpoints and a 413.
	check("suggest", serveFrom(t, rig.srv, next(), http.MethodGet, "/v1/usernames/suggest?name=Ada", "", nil))
	check("available taken", serveFrom(t, rig.srv, next(), http.MethodGet, "/v1/usernames/available?username=new_user", "", nil))
	big := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", strings.NewReader(strings.Repeat("x", 20<<10)))
	big.RemoteAddr = next()
	bigRec := httptest.NewRecorder()
	rig.srv.Engine().ServeHTTP(bigRec, big)
	check("413", bigRec)

	// And the stored account really has one, so the checks above are not
	// passing because nothing was ever saved.
	stored, err := rig.users.GetByEmailCI(context.Background(), "new-user@example.com")
	if err != nil || !stored.HasBirthday {
		t.Fatalf("expected a stored birthday, err = %v", err)
	}
}
