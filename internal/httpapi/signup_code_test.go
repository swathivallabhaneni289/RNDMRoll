package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// The two "expired" answers reach the app exactly as the contract says:
// verify's names the field "code", resend's "signup_token", and both carry
// reason "expired" (a reason RespondFieldError did not know would go out as
// "invalid").
func TestSignupCode_ExpiredIsAnsweredAsExpired(t *testing.T) {
	if _, ok := fieldReasonText[reasonExpired]; !ok {
		t.Fatal("reason expired has no text, so RespondFieldError would send it as invalid")
	}
	router, _, _ := newAuthTestHandler(t)
	verify := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup/verify", map[string]any{"signup_token": "no-such-token", "code": "123456"})
	resend := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup/resend", map[string]any{"signup_token": "no-such-token"})
	for name, tc := range map[string]struct {
		rec  *httptest.ResponseRecorder
		want string
	}{
		"verify": {verify, `{"error":"validation_failed","field":"code","message":"This sign-up has expired. Start again to get a new code.","reason":"expired"}`},
		"resend": {resend, `{"error":"validation_failed","field":"signup_token","message":"This sign-up has expired. Start again to get a new code.","reason":"expired"}`},
	} {
		if tc.rec.Code != http.StatusBadRequest || strings.TrimSpace(tc.rec.Body.String()) != tc.want {
			t.Fatalf("%s: %d %s, want 400 %s", name, tc.rec.Code, tc.rec.Body.String(), tc.want)
		}
		if body := decodeBody(t, tc.rec); body["reason"] != "expired" {
			t.Fatalf("%s: reason %v, want expired", name, body["reason"])
		}
	}
}

// heldSignupSender holds every email until released, and keeps the
// context's error at the moment each one goes out.
type heldSignupSender struct {
	release chan struct{}
	mu      sync.Mutex
	errs    []error
}

func (s *heldSignupSender) Send(ctx context.Context, _, _, _, _ string) error {
	<-s.release
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, ctx.Err())
	return nil
}

// The start answers before its code email goes out, and the email does not
// ride on the request's context: the real server ends that once it has
// answered.
func TestSignupStart_AnswersBeforeTheEmail(t *testing.T) {
	router, h, _ := newAuthTestHandler(t)
	sender := &heldSignupSender{release: make(chan struct{})}
	h.signups.emails.sender = sender
	var once sync.Once
	release := func() { once.Do(func() { close(sender.release) }) }
	t.Cleanup(release) // runs before the handler's wait

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", strings.NewReader(
		`{"email":"held@example.com","password":"Correct-horse-battery1","birthday":"1990-06-15","name":"Held","username":"held_one"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(rec, req)
	}()
	// The email is held until release, so any wait at all would never end;
	// the window only has to fit the start's bcrypt, slow under -race.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("the start waited on the email")
	}
	cancel()
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "signup_token") {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	release()
	h.signups.Wait()
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.errs) != 1 || sender.errs[0] != nil {
		t.Fatalf("emails sent with context errors %v, want one email with none", sender.errs)
	}
}

// The code email: the reset's subject with the code in it, the code again in
// the body, and plain words with no dashes.
func TestSignupMessage_TheResetsSubjectAndPlainWords(t *testing.T) {
	subject, html, text := signupMessage("042137")
	if subject != "Your RNDMRoll code: 042137" {
		t.Fatalf("subject %q", subject)
	}
	wantText := "Use this code in the app to finish signing up:\n\n042137\n\n" +
		"It expires in 30 minutes. If you did not sign up for RNDMRoll, you can ignore this message."
	if text != wantText {
		t.Fatalf("text %q, want %q", text, wantText)
	}
	if !strings.Contains(html, "<strong>042137</strong>") {
		t.Fatalf("html %q has no code", html)
	}
	if strings.ContainsAny(subject+html+text, "—–") {
		t.Fatal("the email has a dash")
	}
}

// A code is bound to its sign-up and the sign-up's email: the same six
// digits hash to something else for another sign-up, another email or
// another secret, and the same for the email in other capitals.
func TestSignupCode_CodesAreBoundToTheSignUpAndItsEmail(t *testing.T) {
	h := NewSignupCodeHandler(newFakeUserRepo(), nil, nil, []byte("secret-one"))
	other := NewSignupCodeHandler(newFakeUserRepo(), nil, nil, []byte("secret-two"))
	a, b := uuid.New(), uuid.New()
	base := string(h.codeHash(a, "person@example.com", "123456"))
	for name, got := range map[string][]byte{
		"another sign-up": h.codeHash(b, "person@example.com", "123456"),
		"another email":   h.codeHash(a, "someone@example.com", "123456"),
		"another code":    h.codeHash(a, "person@example.com", "123457"),
		"another secret":  other.codeHash(a, "person@example.com", "123456"),
	} {
		if string(got) == base {
			t.Fatalf("%s hashes the same", name)
		}
	}
	if string(h.codeHash(a, "Person@Example.COM", "123456")) != base {
		t.Fatal("the email in other capitals hashes differently")
	}
}

// Verify and resend share two limits, one count for both: 10 a minute per
// address, then 5 a minute per address and token. Another address, and
// another token from the same address, still get through until their own
// limit.
func TestSignupCode_VerifyAndResendShareTheirLimits(t *testing.T) {
	router, _, _ := newAuthTestHandler(t)
	send := func(remote, path, token string) int {
		t.Helper()
		body := fmt.Sprintf(`{"signup_token":%q,"code":"123456"}`, token)
		return postRawJSON(t, router, remote, path, body).Code
	}
	paths := []string{"/v1/auth/signup/verify", "/v1/auth/signup/resend"}
	// One token: 5 tries across the two routes, then 429.
	for i := range 5 {
		if code := send("198.51.100.60:1", paths[i%2], "one-token"); code != http.StatusBadRequest {
			t.Fatalf("try %d with one token: %d, want the expired answer", i+1, code)
		}
	}
	for _, path := range paths {
		if code := send("198.51.100.60:1", path, "one-token"); code != http.StatusTooManyRequests {
			t.Fatalf("a sixth try with one token at %s: %d, want 429", path, code)
		}
	}
	if code := send("198.51.100.61:1", paths[0], "one-token"); code != http.StatusBadRequest {
		t.Fatalf("the token from another address: %d", code)
	}
	// One address: 10 requests across the routes and tokens, then 429.
	for i := range 10 {
		if code := send("198.51.100.62:1", paths[i%2], fmt.Sprintf("token-%d", i)); code != http.StatusBadRequest {
			t.Fatalf("request %d from one address: %d", i+1, code)
		}
	}
	if code := send("198.51.100.62:1", paths[1], "token-99"); code != http.StatusTooManyRequests {
		t.Fatalf("an 11th request from one address: %d, want 429", code)
	}
}

// With no sign-up store wired, a sign-up that passed every check is a
// server error and makes nothing: never an account without its code.
func TestSignup_WithNoSignUpStoreIsAServerError(t *testing.T) {
	router, h, deps := newAuthTestHandler(t)
	h.signups = nil
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(nil))
	if rec.Code != http.StatusInternalServerError || strings.TrimSpace(rec.Body.String()) != `{"error":"server_error"}` {
		t.Fatalf("no store: %d %s, want 500 server_error", rec.Code, rec.Body.String())
	}
	if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
		t.Fatalf("%d accounts made", n)
	}
}

// signupFailingUsername is a user.Repository whose username check fails.
type signupFailingUsername struct{ *fakeUserRepo }

func (signupFailingUsername) UsernameTaken(context.Context, string) (bool, error) {
	return false, errors.New("httpapi test: the database is down")
}

// The start's lookups that fail are a server error, never "taken" or a
// stored sign-up.
func TestSignupStart_ALookupThatFailsIsAServerError(t *testing.T) {
	for name, users := range map[string]user.Repository{
		"the email lookup":   failingLookups{newFakeUserRepo()},
		"the username check": signupFailingUsername{newFakeUserRepo()},
	} {
		t.Run(name, func(t *testing.T) {
			router := newTestRouter(t, TestDeps{})
			h := NewAuthHandler(users, auth.NewRefreshService(newFakeRefreshRepo(), time.Hour), []byte("test-secret"), 15*time.Minute)
			h.clock = frozenClock
			store := newFakeSignupStore(users)
			h.signups = NewSignupCodeHandler(users, store, &captureSender{}, []byte("test-secret"))
			h.Register(router.Group("/v1"))
			rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(nil))
			if rec.Code != http.StatusInternalServerError || decodeBody(t, rec)["error"] != string(CodeServerError) {
				t.Fatalf("%d %s, want 500 server_error", rec.Code, rec.Body.String())
			}
			if n := store.count(); n != 0 {
				t.Fatalf("%d sign-ups stored", n)
			}
		})
	}
}

// signupFailingFinish is a SignupStore whose account insert fails, as the
// database would.
type signupFailingFinish struct{ *fakeSignupStore }

func (signupFailingFinish) Finish(context.Context, uuid.UUID, []byte, user.NewAccount) (*user.User, error) {
	return nil, errors.New("httpapi test: the database is down")
}

// The right code whose account cannot be made is a server error, not a
// wrong code: nothing was made, no try was counted, and the same code works
// once the database is back.
func TestSignupVerify_AFailedInsertKeepsTheCode(t *testing.T) {
	router, h, deps := newAuthTestHandler(t)
	start := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup", signupBody(nil))
	token, _ := decodeBody(t, start)["signup_token"].(string)
	code := signupCode(t, router, "new-user@example.com")
	store := h.signups.store.(*fakeSignupStore)

	h.signups.store = signupFailingFinish{store}
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup/verify", map[string]any{"signup_token": token, "code": code})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a failed insert: %d %s, want 500", rec.Code, rec.Body.String())
	}
	h.signups.store = store
	if n := deps.Users.(*fakeUserRepo).count(); n != 0 {
		t.Fatalf("%d accounts after the failed insert", n)
	}
	rec = doJSONRequest(t, router, http.MethodPost, "/v1/auth/signup/verify", map[string]any{"signup_token": token, "code": code})
	if rec.Code != http.StatusOK {
		t.Fatalf("the same code once the database is back: %d %s", rec.Code, rec.Body.String())
	}
}
