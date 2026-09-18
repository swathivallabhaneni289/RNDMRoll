package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// fakeVerifyMailer is a package-local double for mail.Mailer
// (SendVerificationEmail(ctx, *user.User) error), distinct from
// testsupport_test.go's `mailer` placeholder interface
// (SendVerificationEmail(ctx, toEmail, token string) error). The placeholder
// cannot express internal/mail.Service's real contract -- Service owns
// token issuance and needs the user's ID, not a pre-made token -- so this
// plan declares its own fake here rather than reshaping the shared
// placeholder out from under sibling wave 4 worktrees. See 01-09-SUMMARY.md.
type fakeVerifyMailer struct {
	mu   sync.Mutex
	sent []*user.User
}

func (f *fakeVerifyMailer) SendVerificationEmail(ctx context.Context, u *user.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, u)
	return nil
}

var _ mail.Mailer = (*fakeVerifyMailer)(nil)

func newVerifyEmailTestHandler(t *testing.T) (*VerifyEmailHandler, *fakeUserRepo, *fakeVerificationRepo, *fakeVerifyMailer) {
	t.Helper()
	users := newFakeUserRepo()
	verifications := newFakeVerificationRepo()
	mailer := &fakeVerifyMailer{}
	refresh := auth.NewRefreshService(newFakeRefreshRepo(), 30*24*time.Hour)
	h := NewVerifyEmailHandler(users, verifications, mailer, refresh, []byte("test-secret-at-least-32-bytes!!"), 15*time.Minute, "rndmroll")
	return h, users, verifications, mailer
}

func newVerifyEmailTestRouter(t *testing.T, h *VerifyEmailHandler) *gin.Engine {
	t.Helper()
	router := newTestRouter(t, TestDeps{})
	rg := router.Group("/v1/auth")
	h.Register(rg)
	return router
}

// issueTestVerificationToken mirrors internal/mail.Service's own hashing so
// most handler tests can seed a token without depending on that package.
// TestVerifyEmail_IntegratesWithMailServiceIssuedToken separately exercises
// the real Service to catch any drift between the two.
func issueTestVerificationToken(t *testing.T, repo *fakeVerificationRepo, userID uuid.UUID, ttl time.Duration) string {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)
	if err := repo.Insert(context.Background(), userID, sum[:], time.Now().Add(ttl)); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	return raw
}

func postVerifyEmailJSON(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestVerifyEmail_FreshTokenReturns200WithSession(t *testing.T) {
	h, users, verifications, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	u, err := users.Create(context.Background(), "person@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token := issueTestVerificationToken(t, verifications, u.ID, time.Hour)

	w := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": token})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID            string `json:"id"`
			EmailVerified bool   `json:"email_verified"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected non-empty access_token and refresh_token")
	}
	if !resp.User.EmailVerified {
		t.Fatal("expected user.email_verified to be true")
	}
	if resp.User.ID != u.ID.String() {
		t.Fatalf("user.id = %q, want %q", resp.User.ID, u.ID.String())
	}

	updated, err := users.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if !updated.EmailVerified {
		t.Fatal("expected stored user to be marked verified")
	}
	if updated.EmailVerifiedVia == nil || *updated.EmailVerifiedVia != user.VerifiedViaPasswordFlow {
		t.Fatalf("expected EmailVerifiedVia = password_flow, got %v", updated.EmailVerifiedVia)
	}
}

func TestVerifyEmail_SameTokenTwiceReturns409TokenConsumed(t *testing.T) {
	h, users, verifications, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	u, err := users.Create(context.Background(), "person2@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token := issueTestVerificationToken(t, verifications, u.ID, time.Hour)

	first := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": token})
	if first.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d", first.Code)
	}

	second := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": token})
	if second.Code != http.StatusConflict {
		t.Fatalf("second call: expected 409, got %d: %s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "token_consumed") {
		t.Fatalf("expected token_consumed body, got: %s", second.Body.String())
	}
}

func TestVerifyEmail_ExpiredTokenReturns401TokenExpired(t *testing.T) {
	h, users, verifications, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	u, err := users.Create(context.Background(), "person3@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token := issueTestVerificationToken(t, verifications, u.ID, -time.Hour)

	w := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": token})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "token_expired") {
		t.Fatalf("expected token_expired body, got: %s", w.Body.String())
	}
}

func TestVerifyEmail_NeverIssuedTokenReturns401TokenInvalid(t *testing.T) {
	h, _, _, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	w := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": "never-issued-token"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "token_invalid") {
		t.Fatalf("expected token_invalid body, got: %s", w.Body.String())
	}
}

func TestVerifyEmailResend_RegisteredAndUnregisteredByteIdentical(t *testing.T) {
	h, users, _, mailer := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	if _, err := users.Create(context.Background(), "registered@example.com", nil, false, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	registered := postVerifyEmailJSON(t, router, "/v1/auth/verify-email/resend", map[string]string{"email": "registered@example.com"})
	unregistered := postVerifyEmailJSON(t, router, "/v1/auth/verify-email/resend", map[string]string{"email": "nobody@example.com"})

	if registered.Code != http.StatusAccepted {
		t.Fatalf("registered: expected 202, got %d: %s", registered.Code, registered.Body.String())
	}
	if unregistered.Code != http.StatusAccepted {
		t.Fatalf("unregistered: expected 202, got %d: %s", unregistered.Code, unregistered.Body.String())
	}
	if registered.Body.String() != unregistered.Body.String() {
		t.Fatalf("responses differ: %q vs %q", registered.Body.String(), unregistered.Body.String())
	}

	mailer.mu.Lock()
	sentCount := len(mailer.sent)
	mailer.mu.Unlock()
	if sentCount != 1 {
		t.Fatalf("expected exactly 1 send (for the registered address), got %d", sentCount)
	}
}

func TestVerifyEmailCallback_EscapesTokenAndRedirectsToDeepLink(t *testing.T) {
	h, _, _, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	maliciousToken := `"><script>alert(1)</script>`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify-email/callback?token="+url.QueryEscape(maliciousToken), nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("callback page must not contain the raw unescaped token")
	}
	if !strings.Contains(body, "rndmroll://verify-email?token=") {
		t.Fatalf("callback page missing deep link, got: %s", body)
	}
}

// captureSender is a minimal mail.Sender double used only to bridge into a
// real mail.Service for the cross-package integration test below.
type captureSender struct {
	lastText string
}

func (s *captureSender) Send(ctx context.Context, to, subject, html, text string) error {
	s.lastText = text
	return nil
}

var _ mail.Sender = (*captureSender)(nil)

// TestVerifyEmail_IntegratesWithMailServiceIssuedToken exercises the real
// internal/mail.Service (not the hand-rolled issueTestVerificationToken
// helper) end to end through Verify, so a mismatch between how Service
// encodes/hashes a token and how Verify decodes/hashes it would fail this
// test even if every other test above still passes.
func TestVerifyEmail_IntegratesWithMailServiceIssuedToken(t *testing.T) {
	h, users, verifications, _ := newVerifyEmailTestHandler(t)
	router := newVerifyEmailTestRouter(t, h)

	u, err := users.Create(context.Background(), "integration@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	sender := &captureSender{}
	svc := mail.NewService(verifications, sender, "https://rndmroll.app", time.Hour)
	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("SendVerificationEmail failed: %v", err)
	}

	idx := strings.Index(sender.lastText, "token=")
	if idx == -1 {
		t.Fatalf("no token in message: %s", sender.lastText)
	}
	token := sender.lastText[idx+len("token="):]
	if end := strings.IndexAny(token, " \n\t"); end != -1 {
		token = token[:end]
	}

	w := postVerifyEmailJSON(t, router, "/v1/auth/verify-email", map[string]string{"token": token})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a real Service-issued token, got %d: %s", w.Code, w.Body.String())
	}
}
