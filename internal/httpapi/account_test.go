package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// Change password (Phase 2.1, plan 01): POST /v1/me/password through the full
// server over the in-memory fakes, so RequireAuth, RequireUser, the body cap
// and the limiters are all in play. The Postgres side is in
// internal/store/postgres, and one end-to-end test against Postgres is in
// account_integration_test.go.

// syncBuffer is a bytes.Buffer that is safe to write from the server and read
// from a test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// accountRig is the full server with the change-password route, plus the
// fakes behind it and a buffer that holds everything the server logged
// through both the slog logger handed to NewServer and the standard log
// package (the handlers' unexpected-error lines use the latter).
type accountRig struct {
	*fullTestRig
	fake *fakeUserRepo
	logs *syncBuffer
}

func newAccountRig(t *testing.T) *accountRig {
	t.Helper()
	logs := &syncBuffer{}
	previous := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	fake := newFakeUserRepo()
	rig := newFullTestRigWith(t, fake, rigOptions{
		account: true,
		logger:  slog.New(slog.NewTextHandler(logs, nil)),
	})
	return &accountRig{fullTestRig: rig, fake: fake, logs: logs}
}

// accountSession is a person signed up through the rig.
type accountSession struct {
	id       uuid.UUID
	email    string
	password string
	access   string
	refresh  string
}

func (r *accountRig) signUp(t *testing.T, email, username, password string) accountSession {
	t.Helper()
	rec := serveFrom(t, r.srv, integrationRemote(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
		"email": email, "username": username, "password": password,
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("signup %s: expected 200, got %d: %s", email, rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	idText, _ := userOf(t, body)["id"].(string)
	id, err := uuid.Parse(idText)
	if err != nil {
		t.Fatalf("signup %s: bad id %q: %v", email, idText, err)
	}
	access, _ := body["access_token"].(string)
	refresh, _ := body["refresh_token"].(string)
	return accountSession{id: id, email: email, password: password, access: access, refresh: refresh}
}

func (r *accountRig) logIn(t *testing.T, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	return serveFrom(t, r.srv, integrationRemote(), http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email": email, "password": password,
	})
}

// postPassword sends POST /v1/me/password from the given socket address.
func (r *accountRig) postPassword(t *testing.T, remote, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return serveFrom(t, r.srv, remote, http.MethodPost, "/v1/me/password", token, body)
}

// changePassword sends the change from a fresh address, so only the limits a
// test is about can fire.
func (r *accountRig) changePassword(t *testing.T, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return r.postPassword(t, integrationRemote(), token, body)
}

func (r *accountRig) refreshWith(t *testing.T, refreshToken string) *httptest.ResponseRecorder {
	t.Helper()
	return serveFrom(t, r.srv, integrationRemote(), http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refreshToken})
}

func (r *accountRig) meWith(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return serveFrom(t, r.srv, integrationRemote(), http.MethodGet, "/v1/me", token, nil)
}

// assertNothingChanged checks that a refused change left the stored hash and
// the account's sessions exactly as they were.
func (r *accountRig) assertNothingChanged(t *testing.T, s accountSession, hashBefore string, activeBefore int) {
	t.Helper()
	if got := r.fake.storedPasswordHash(s.id); got != hashBefore {
		t.Errorf("the stored password hash changed after a refused change")
	}
	if got := r.tokens.activeCount(s.id); got != activeBefore {
		t.Errorf("active refresh tokens = %d, want %d (a refused change must not touch sessions)", got, activeBefore)
	}
}

// keysOf returns the sorted keys of m joined with commas.
func keysOf(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestChangePassword_Succeeds(t *testing.T) {
	rig := newAccountRig(t)
	const oldPassword, newPassword = "Old-password-1234!", "New-password-5678!"
	s := rig.signUp(t, "change@example.com", "change_one", oldPassword)

	rec := rig.changePassword(t, s.access, map[string]any{"current_password": oldPassword, "new_password": newPassword})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	for _, key := range []string{"access_token", "refresh_token", "expires_in", "user"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("expected %s in the response, got %v", key, body)
		}
	}
	if body["expires_in"] != float64(900) {
		t.Errorf("expires_in = %v, want 900", body["expires_in"])
	}
	newAccess, _ := body["access_token"].(string)
	newRefresh, _ := body["refresh_token"].(string)
	if newAccess == "" || newRefresh == "" || newRefresh == s.refresh {
		t.Fatalf("expected new tokens, got access %q refresh %q", newAccess, newRefresh)
	}
	got := userOf(t, body)
	if got["id"] != s.id.String() || got["email"] != s.email || got["account_visibility"] != "public" {
		t.Errorf("unexpected user in the body: %v", got)
	}

	// The same body shape as log in.
	login := decodeBody(t, rig.logIn(t, s.email, newPassword))
	if keysOf(body) != keysOf(login) || keysOf(got) != keysOf(userOf(t, login)) {
		t.Errorf("response keys %q / %q differ from log in's %q / %q", keysOf(body), keysOf(got), keysOf(login), keysOf(userOf(t, login)))
	}

	// The new access token works.
	if me := rig.meWith(t, newAccess); me.Code != http.StatusOK {
		t.Fatalf("GET /me with the new access token: %d %s", me.Code, me.Body.String())
	}
	// The new password logs in, the old one no longer does.
	if rec := rig.logIn(t, s.email, newPassword); rec.Code != http.StatusOK {
		t.Fatalf("log in with the new password: %d %s", rec.Code, rec.Body.String())
	}
	assertInvalidCredentials(t, rig.logIn(t, s.email, oldPassword))

	// The stored value is a bcrypt hash of the new password, not the text.
	stored := rig.fake.storedPasswordHash(s.id)
	if stored == newPassword || stored == oldPassword {
		t.Fatal("the stored value is a plain password")
	}
	if auth.ComparePassword(stored, newPassword) != nil {
		t.Error("the stored hash does not match the new password")
	}
	if auth.ComparePassword(stored, oldPassword) == nil {
		t.Error("the stored hash still matches the old password")
	}

	// The new refresh token is a live session.
	if rec := rig.refreshWith(t, newRefresh); rec.Code != http.StatusOK {
		t.Fatalf("refresh with the new token: %d %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_WrongCurrentIs400FieldCurrentPassword(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "wrong@example.com", "wrong_one", "Right-password-1!")
	hashBefore := rig.fake.storedPasswordHash(s.id)
	activeBefore := rig.tokens.activeCount(s.id)

	rec := rig.changePassword(t, s.access, map[string]any{"current_password": "not-the-password", "new_password": "Another-password-2!"})

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("a wrong current password must never be a 401 (it signs the phone out): %s", rec.Body.String())
	}
	assertFieldError(t, rec, "current_password")
	if got := decodeBody(t, rec)["message"]; got != "That password is not right." {
		t.Errorf("message = %v", got)
	}
	rig.assertNothingChanged(t, s, hashBefore, activeBefore)
	// The session and the old password both still work.
	if rec := rig.refreshWith(t, s.refresh); rec.Code != http.StatusOK {
		t.Errorf("the old refresh token should still redeem: %d %s", rec.Code, rec.Body.String())
	}
	if rec := rig.logIn(t, s.email, "Right-password-1!"); rec.Code != http.StatusOK {
		t.Errorf("the old password should still log in: %d %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_NewPasswordTooShortTooLongAndBytes(t *testing.T) {
	const current = "Current-password-1!"
	cases := []struct {
		name    string
		current string
		next    string
		field   string
		message string
	}{
		{"7 characters", current, "abcdefg", "new_password", "This value is too short."},
		{"73 characters", current, strings.Repeat("a", 73), "new_password", "This value is too long."},
		// 40 two-byte characters pass the character count and are refused when
		// they are hashed: bcrypt counts 72 bytes.
		{"40 two-byte characters, 80 bytes", current, strings.Repeat("É", 38) + "1!", "new_password", "This value is too long."},
		// The new password's rule runs before the current one is compared.
		{"too short with a wrong current password", "not-the-password", "abcdefg", "new_password", "This value is too short."},
		{"empty", current, "", "new_password", "This field is required."},
		// Long enough but missing a capital, a number or a symbol.
		{"no capital", current, "lowercase-pass-1!", "new_password", "Use a capital letter, a number and a symbol."},
		{"no number", current, "Lowercase-pass-!", "new_password", "Use a capital letter, a number and a symbol."},
		{"no symbol", current, "Lowercasepass1", "new_password", "Use a capital letter, a number and a symbol."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newAccountRig(t)
			s := rig.signUp(t, "rule@example.com", "rule_one", current)
			hashBefore := rig.fake.storedPasswordHash(s.id)
			activeBefore := rig.tokens.activeCount(s.id)

			rec := rig.changePassword(t, s.access, map[string]any{"current_password": tc.current, "new_password": tc.next})

			assertFieldError(t, rec, tc.field)
			if got := decodeBody(t, rec)["message"]; got != tc.message {
				t.Errorf("message = %v, want %q", got, tc.message)
			}
			if tc.next != "" && strings.Contains(rec.Body.String(), tc.next) {
				t.Error("the response echoes the new password")
			}
			rig.assertNothingChanged(t, s, hashBefore, activeBefore)
		})
	}
}

func TestChangePassword_SameAsCurrentIs400FieldNewPassword(t *testing.T) {
	rig := newAccountRig(t)
	const current = "Current-password-1!"
	s := rig.signUp(t, "same@example.com", "same_one", current)
	hashBefore := rig.fake.storedPasswordHash(s.id)
	activeBefore := rig.tokens.activeCount(s.id)

	rec := rig.changePassword(t, s.access, map[string]any{"current_password": current, "new_password": current})

	assertFieldError(t, rec, "new_password")
	if got := decodeBody(t, rec)["message"]; got != "Choose a different password." {
		t.Errorf("message = %v", got)
	}
	rig.assertNothingChanged(t, s, hashBefore, activeBefore)

	// A wrong current password is judged first, so an unchanged-looking pair
	// of wrong values says nothing about the real one.
	wrong := rig.changePassword(t, s.access, map[string]any{"current_password": "Typed-wrong-1!", "new_password": "Typed-wrong-1!"})
	assertFieldError(t, wrong, "current_password")
}

func TestChangePassword_OtherSessionsAreRevokedAndThisOneKept(t *testing.T) {
	rig := newAccountRig(t)
	const oldPassword, newPassword = "Old-password-1234!", "New-password-5678!"
	a := rig.signUp(t, "sessions@example.com", "sessions_one", oldPassword)

	// Two more phones log in.
	var others []string
	for i := 0; i < 2; i++ {
		rec := rig.logIn(t, a.email, oldPassword)
		if rec.Code != http.StatusOK {
			t.Fatalf("log in %d: %d %s", i, rec.Code, rec.Body.String())
		}
		token, _ := decodeBody(t, rec)["refresh_token"].(string)
		others = append(others, token)
	}
	if got := rig.tokens.activeCount(a.id); got != 3 {
		t.Fatalf("expected 3 live sessions before the change, got %d", got)
	}

	rec := rig.changePassword(t, a.access, map[string]any{"current_password": oldPassword, "new_password": newPassword})
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	newRefresh, _ := decodeBody(t, rec)["refresh_token"].(string)

	// Every other phone is signed out at its next refresh.
	for i, token := range others {
		r := rig.refreshWith(t, token)
		if r.Code != http.StatusUnauthorized || decodeBody(t, r)["error"] != "token_invalid" {
			t.Errorf("other phone %d: expected 401 token_invalid, got %d %s", i, r.Code, r.Body.String())
		}
	}
	// This phone's old refresh token is replaced by the new one.
	if r := rig.refreshWith(t, a.refresh); r.Code != http.StatusUnauthorized || decodeBody(t, r)["error"] != "token_invalid" {
		t.Errorf("the old refresh token of this phone: expected 401 token_invalid, got %d %s", r.Code, r.Body.String())
	}
	if got := rig.tokens.activeCount(a.id); got != 1 {
		t.Errorf("live sessions after the change = %d, want exactly 1", got)
	}
	if r := rig.refreshWith(t, newRefresh); r.Code != http.StatusOK {
		t.Errorf("the response's refresh token should redeem: %d %s", r.Code, r.Body.String())
	}
}

func TestChangePassword_IsRateLimitedPerCallerAndPerAddress(t *testing.T) {
	rig := newAccountRig(t)
	empty := map[string]any{} // a cheap 400 that the limiters still count

	// Per caller: 5 a minute. Each call comes from its own address, so only
	// the caller's limit can stop the 6th.
	x := rig.signUp(t, "limit-x@example.com", "limit_x", "Password-for-x-1!")
	for i := 1; i <= 5; i++ {
		rec := rig.changePassword(t, x.access, empty)
		assertFieldError(t, rec, "current_password")
	}
	rec := rig.changePassword(t, x.access, empty)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the 6th call by one caller: expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["error"]; got != "rate_limited" {
		t.Fatalf("the 6th call: error = %v, want rate_limited", got)
	}
	// Another caller is not limited.
	y := rig.signUp(t, "limit-y@example.com", "limit_y", "Password-for-y-1!")
	assertFieldError(t, rig.changePassword(t, y.access, empty), "current_password")

	// Per address: 20 a minute. Four callers use their 5 calls each from one
	// address; a fifth, fresh caller is stopped by the address alone. Everyone
	// signs up first, so the 20 calls run back to back.
	callers := make([]accountSession, 5)
	for i := range callers {
		n := string(rune('1' + i))
		callers[i] = rig.signUp(t, "limit-p"+n+"@example.com", "limit_p"+n, "Password-for-p-1!")
	}
	const address = "198.51.100.77:4000"
	for i, c := range callers[:4] {
		for call := 1; call <= 5; call++ {
			rec := rig.postPassword(t, address, c.access, empty)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("caller %d call %d from the shared address: expected 400, got %d: %s", i+1, call, rec.Code, rec.Body.String())
			}
		}
	}
	fifth := callers[4]
	rec = rig.postPassword(t, address, fifth.access, empty)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the 21st call from one address: expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["error"]; got != "rate_limited" {
		t.Fatalf("the 21st call: error = %v, want rate_limited", got)
	}
	// The same caller from another address is not limited: it was the address,
	// not the caller, that ran out.
	assertFieldError(t, rig.postPassword(t, "198.51.100.78:4000", fifth.access, empty), "current_password")
}

func TestChangePassword_NoTokenIs401AndDeletedAccountIs401(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "gone@example.com", "gone_one", "Current-password-1!")
	body := map[string]any{"current_password": "Current-password-1!", "new_password": "Another-password-2!"}

	for name, token := range map[string]string{"no token": "", "not a token": "not-a-token"} {
		rec := rig.changePassword(t, token, body)
		if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["error"] != "token_invalid" {
			t.Errorf("%s: expected 401 token_invalid, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}

	// A valid token for an account that has since gone.
	rig.fake.remove(t, s.id)
	rec := rig.changePassword(t, s.access, body)
	if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["error"] != "token_invalid" {
		t.Fatalf("deleted account: expected 401 token_invalid, got %d: %s", rec.Code, rec.Body.String())
	}
	if rig.fake.count() != 0 {
		t.Fatal("the account came back")
	}
}

func TestChangePassword_NoPasswordInAnyBodyOrLog(t *testing.T) {
	rig := newAccountRig(t)
	const current, next, wrong = "Current-Pw-Zq81-unique", "Next-Pw-Xr47-unique", "Wrong-Pw-Kj29-unique"
	secrets := []string{current, next, wrong}

	var bodies []string
	note := func(rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
		bodies = append(bodies, rec.Body.String())
		return rec
	}

	// One caller makes the refusals (5 calls, the most one caller may make a
	// minute); another makes the malformed one, the failed transaction and
	// the success.
	a := rig.signUp(t, "nolog-a@example.com", "nolog_a", current)
	note(rig.changePassword(t, a.access, map[string]any{"current_password": wrong, "new_password": next}))
	note(rig.changePassword(t, a.access, map[string]any{"current_password": current, "new_password": "short1"}))
	note(rig.changePassword(t, a.access, map[string]any{"current_password": current, "new_password": strings.Repeat("É", 38) + "1!"}))
	note(rig.changePassword(t, a.access, map[string]any{"current_password": current, "new_password": current}))
	note(rig.changePassword(t, a.access, map[string]any{"current_password": current}))

	b := rig.signUp(t, "nolog-b@example.com", "nolog_b", current)
	note(serveFrom(t, rig.srv, integrationRemote(), http.MethodPost, "/v1/me/password", b.access, nil))
	rig.fake.changePasswordErr = errors.New("storage unavailable")
	failed := note(rig.changePassword(t, b.access, map[string]any{"current_password": current, "new_password": next}))
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("a failed transaction: expected 500, got %d: %s", failed.Code, failed.Body.String())
	}
	rig.fake.changePasswordErr = nil
	ok := note(rig.changePassword(t, b.access, map[string]any{"current_password": current, "new_password": next}))
	if ok.Code != http.StatusOK {
		t.Fatalf("success: expected 200, got %d: %s", ok.Code, ok.Body.String())
	}

	all := strings.Join(bodies, "\n")
	logged := rig.logs.String()
	for _, secret := range secrets {
		if strings.Contains(all, secret) {
			t.Errorf("a response body contains %q", secret)
		}
		if strings.Contains(logged, secret) {
			t.Errorf("the server's log contains %q", secret)
		}
	}
	// The scan is not vacuous: both log paths did receive lines.
	if !strings.Contains(logged, "path=/v1/me/password") {
		t.Error("the request logger wrote nothing about the route")
	}
	if !strings.Contains(logged, "unmapped error") {
		t.Error("the unexpected-error line of the failed transaction was not captured")
	}
}

func TestChangePassword_MissingFieldsAreRequiredErrors(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "missing@example.com", "missing_one", "Current-password-1!")

	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"empty object", map[string]any{}, "current_password"},
		{"no new password", map[string]any{"current_password": "Current-password-1!"}, "new_password"},
		{"no current password", map[string]any{"new_password": "Another-password-2!"}, "current_password"},
		{"both empty", map[string]any{"current_password": "", "new_password": ""}, "current_password"},
		{"new password null", map[string]any{"current_password": "Current-password-1!", "new_password": nil}, "new_password"},
	}
	for _, tc := range cases {
		rec := rig.changePassword(t, s.access, tc.body)
		if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["field"] != tc.field {
			t.Errorf("%s: expected 400 field %q, got %d: %s", tc.name, tc.field, rec.Code, rec.Body.String())
			continue
		}
		if got := decodeBody(t, rec)["message"]; got != "This field is required." {
			t.Errorf("%s: message = %v", tc.name, got)
		}
	}

	// A wrong type, an unreadable body and no body at all are the one generic
	// refusal, which names no field.
	other := rig.signUp(t, "missing2@example.com", "missing_two", "Current-password-1!")
	for name, body := range map[string]any{
		"a number":      map[string]any{"current_password": 5, "new_password": "Another-password-2!"},
		"a list":        []string{"Current-password-1!"},
		"nothing sent":  nil,
		"not an object": "just text",
	} {
		rec := rig.changePassword(t, other.access, body)
		assertValidationFailed(t, rec)
		if _, named := decodeBody(t, rec)["field"]; named {
			t.Errorf("%s: a generic refusal should name no field: %s", name, rec.Body.String())
		}
	}
}

func TestChangePassword_BodyOver16KBIs413(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "big@example.com", "big_one", "Current-password-1!")
	hashBefore := rig.fake.storedPasswordHash(s.id)
	activeBefore := rig.tokens.activeCount(s.id)

	big := `{"current_password":"` + strings.Repeat("a", 20<<10) + `","new_password":"Another-password-2!"}`
	for _, chunked := range []bool{false, true} {
		var body io.Reader = strings.NewReader(big)
		if chunked {
			// An unknown length: the declared-length check cannot fire, so the
			// reader's own cap must.
			body = io.NopCloser(strings.NewReader(big))
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/me/password", body)
		if chunked {
			req.ContentLength = -1
		}
		req.RemoteAddr = integrationRemote()
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.access)
		rec := httptest.NewRecorder()
		rig.srv.Engine().ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge || decodeBody(t, rec)["error"] != "payload_too_large" {
			t.Errorf("chunked=%v: expected 413 payload_too_large, got %d: %s", chunked, rec.Code, rec.Body.String())
		}
	}
	rig.assertNothingChanged(t, s, hashBefore, activeBefore)
}

func TestChangePassword_RepositoryFailureIs500AndChangesNothing(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "fail@example.com", "fail_one", "Current-password-1!")
	hashBefore := rig.fake.storedPasswordHash(s.id)
	activeBefore := rig.tokens.activeCount(s.id)
	rig.fake.changePasswordErr = errors.New("storage unavailable")

	rec := rig.changePassword(t, s.access, map[string]any{"current_password": "Current-password-1!", "new_password": "Another-password-2!"})

	if rec.Code != http.StatusInternalServerError || rec.Body.String() != `{"error":"server_error"}` {
		t.Fatalf("expected the fixed 500 body, got %d: %s", rec.Code, rec.Body.String())
	}
	rig.assertNothingChanged(t, s, hashBefore, activeBefore)
}

func TestChangePassword_AccountGoneDuringTheChangeIs401TokenInvalid(t *testing.T) {
	rig := newAccountRig(t)
	s := rig.signUp(t, "race@example.com", "race_one", "Current-password-1!")
	// RequireUser found the account; the transaction does not.
	rig.fake.changePasswordErr = user.ErrNotFound

	rec := rig.changePassword(t, s.access, map[string]any{"current_password": "Current-password-1!", "new_password": "Another-password-2!"})

	if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["error"] != "token_invalid" {
		t.Fatalf("expected 401 token_invalid, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_AccountWithNoStoredPasswordIsRefusedEvenForTheDummyText(t *testing.T) {
	rig := newAccountRig(t)
	u, err := rig.fake.Create(context.Background(), "nohash@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	// The text the dummy hash was made from must not unlock anything.
	rec := rig.changePassword(t, token, map[string]any{
		"current_password": "dummy-password-for-constant-time-comparison",
		"new_password":     "Another-password-2!",
	})

	assertFieldError(t, rec, "current_password")
	if got := rig.fake.storedPasswordHash(u.ID); got != "" {
		t.Fatalf("a password hash was stored: %q", got)
	}
	if got := rig.tokens.activeCount(u.ID); got != 0 {
		t.Fatalf("a session was made: %d", got)
	}
}

func TestServer_PasswordRouteIsNotMountedWhenTheAccountHandlerIsNotWired(t *testing.T) {
	users := newFakeUserRepo()
	u := users.seedComplete(t, "plain@example.com", "Plain Person", "plain_person", validBirthday)
	srv := newFullTestServer(t, users)
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	rec := serveFrom(t, srv, remoteN(60), http.MethodPost, "/v1/me/password", token, map[string]any{
		"current_password": "x", "new_password": "Another-password-2!",
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no AccountHandler is wired, got %d: %s", rec.Code, rec.Body.String())
	}
}
