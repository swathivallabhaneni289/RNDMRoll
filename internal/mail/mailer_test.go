package mail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// --- fakeMailVerificationRepo: local in-memory user.EmailVerificationRepository ---
//
// A package-local fake, distinct from internal/httpapi's fakeVerificationRepo,
// so Service's behavior (order of DeleteForUser/Insert calls, exact hash
// stored) can be asserted in isolation from any HTTP handler.

type verificationInsertCall struct {
	userID    uuid.UUID
	tokenHash []byte
	expiresAt time.Time
}

type fakeMailVerificationRepo struct {
	mu               sync.Mutex
	calls            []string
	inserted         []verificationInsertCall
	deletedForUserID []uuid.UUID
}

func newFakeMailVerificationRepo() *fakeMailVerificationRepo {
	return &fakeMailVerificationRepo{}
}

func (f *fakeMailVerificationRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "insert")
	hashCopy := append([]byte(nil), tokenHash...)
	f.inserted = append(f.inserted, verificationInsertCall{userID: userID, tokenHash: hashCopy, expiresAt: expiresAt})
	return nil
}

func (f *fakeMailVerificationRepo) ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	return uuid.UUID{}, user.ErrTokenInvalid
}

func (f *fakeMailVerificationRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "delete")
	f.deletedForUserID = append(f.deletedForUserID, userID)
	return nil
}

var _ user.EmailVerificationRepository = (*fakeMailVerificationRepo)(nil)

// --- fakeSender: captures the last composed message ---

type fakeSender struct {
	mu          sync.Mutex
	lastTo      string
	lastSubject string
	lastHTML    string
	lastText    string
	sendCount   int
}

func (f *fakeSender) Send(ctx context.Context, to, subject, html, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTo = to
	f.lastSubject = subject
	f.lastHTML = html
	f.lastText = text
	f.sendCount++
	return nil
}

var _ Sender = (*fakeSender)(nil)

func extractToken(t *testing.T, text string) string {
	t.Helper()
	idx := strings.Index(text, "token=")
	if idx == -1 {
		t.Fatalf("no token= query param found in message body: %s", text)
	}
	rest := text[idx+len("token="):]
	end := strings.IndexAny(rest, " \n\t")
	if end != -1 {
		rest = rest[:end]
	}
	return rest
}

func TestService_SendVerificationEmail_StoresHashedTokenMatchingLinkPlaintext(t *testing.T) {
	repo := newFakeMailVerificationRepo()
	sender := &fakeSender{}
	svc := NewService(repo, sender, "https://rndmroll.app", time.Hour)
	u := &user.User{ID: uuid.New(), Email: "person@example.com"}

	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("SendVerificationEmail returned error: %v", err)
	}

	if len(repo.inserted) != 1 {
		t.Fatalf("expected 1 insert call, got %d", len(repo.inserted))
	}
	insert := repo.inserted[0]
	if insert.userID != u.ID {
		t.Fatalf("insert userID = %s, want %s", insert.userID, u.ID)
	}
	if sender.lastTo != u.Email {
		t.Fatalf("sender.to = %q, want %q", sender.lastTo, u.Email)
	}

	token := extractToken(t, sender.lastText)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("link token did not base64url-decode: %v", err)
	}
	sum := sha256.Sum256(raw)
	if !bytes.Equal(sum[:], insert.tokenHash) {
		t.Fatal("stored hash does not equal sha256 of the plaintext token carried in the link")
	}
}

func TestService_SendVerificationEmail_SecondIssueDeletesFirst(t *testing.T) {
	repo := newFakeMailVerificationRepo()
	sender := &fakeSender{}
	svc := NewService(repo, sender, "https://rndmroll.app", time.Hour)
	u := &user.User{ID: uuid.New(), Email: "person@example.com"}

	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("first SendVerificationEmail returned error: %v", err)
	}
	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("second SendVerificationEmail returned error: %v", err)
	}

	want := []string{"delete", "insert", "delete", "insert"}
	if len(repo.calls) != len(want) {
		t.Fatalf("call sequence = %v, want %v", repo.calls, want)
	}
	for i, c := range want {
		if repo.calls[i] != c {
			t.Fatalf("call sequence = %v, want %v", repo.calls, want)
		}
	}
	if len(repo.deletedForUserID) != 2 || repo.deletedForUserID[0] != u.ID || repo.deletedForUserID[1] != u.ID {
		t.Fatalf("DeleteForUser calls = %v, want two calls for %s", repo.deletedForUserID, u.ID)
	}
}

func TestLogSender_WritesRecipientAndLinkWithNoNetworkCall(t *testing.T) {
	var buf bytes.Buffer
	sender := NewLogSender(&buf)

	link := "https://rndmroll.app/v1/auth/verify-email/callback?token=abc123"
	err := sender.Send(context.Background(), "person@example.com", "Verify your email for RNDMRoll", "<p>html</p>", "body "+link+" footer")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "person@example.com") {
		t.Fatalf("log output missing recipient: %s", out)
	}
	if !strings.Contains(out, link) {
		t.Fatalf("log output missing link: %s", out)
	}
}

func TestResendSender_PostsWithBearerAndSurfacesStatusWithoutBody(t *testing.T) {
	var gotAuth, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("provider secret error detail"))
	}))
	defer srv.Close()

	sender := &resendSender{
		apiKey:      "test-key",
		fromAddress: "hello@rndmroll.app",
		fromName:    "RNDMRoll",
		endpoint:    srv.URL,
		client:      &http.Client{Timeout: 10 * time.Second},
	}

	err := sender.Send(context.Background(), "to@example.com", "subject", "<p>hi</p>", "hi")
	if err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
	if strings.Contains(err.Error(), "provider secret error detail") {
		t.Fatalf("error must not leak the provider response body, got: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test-key")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
}

func TestService_SendVerificationEmail_LinkUsesBaseURLAndCallbackPath(t *testing.T) {
	repo := newFakeMailVerificationRepo()
	sender := &fakeSender{}
	svc := NewService(repo, sender, "https://rndmroll.app", time.Hour)
	u := &user.User{ID: uuid.New(), Email: "person@example.com"}

	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("SendVerificationEmail returned error: %v", err)
	}

	wantPrefix := "https://rndmroll.app/v1/auth/verify-email/callback?token="
	if !strings.Contains(sender.lastText, wantPrefix) {
		t.Fatalf("message body missing expected link prefix %q, got: %s", wantPrefix, sender.lastText)
	}

	token := extractToken(t, sender.lastText)
	parsed, err := url.Parse(wantPrefix + token)
	if err != nil || parsed.Query().Get("token") != token {
		t.Fatalf("link did not parse with token as a query parameter: %v", err)
	}
}

func TestService_SendVerificationEmail_MessageCopyMatchesConstraints(t *testing.T) {
	repo := newFakeMailVerificationRepo()
	sender := &fakeSender{}
	svc := NewService(repo, sender, "https://rndmroll.app", 24*time.Hour)
	u := &user.User{ID: uuid.New(), Email: "person@example.com"}

	if err := svc.SendVerificationEmail(context.Background(), u); err != nil {
		t.Fatalf("SendVerificationEmail returned error: %v", err)
	}

	if sender.lastSubject != "Verify your email for RNDMRoll" {
		t.Fatalf("subject = %q", sender.lastSubject)
	}
	if !strings.Contains(sender.lastText, "This link expires in 24 hours.") {
		t.Fatalf("text body missing expiry sentence: %s", sender.lastText)
	}
	if sender.lastHTML == "" {
		t.Fatal("expected a non-empty HTML body")
	}
	emDash := string([]rune{0x2014})
	if strings.Contains(sender.lastSubject+sender.lastHTML+sender.lastText, emDash) {
		t.Fatal("message copy must not contain an em dash")
	}
}
