package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// captureSender keeps every email instead of sending it.
type captureSender struct {
	mu   sync.Mutex
	sent []struct{ to, subject, text string }
}

func (s *captureSender) Send(_ context.Context, to, subject, _ string, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, struct{ to, subject, text string }{to, subject, text})
	return nil
}

func (s *captureSender) lastCode(t *testing.T, to string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.sent) - 1; i >= 0; i-- {
		if s.sent[i].to == to {
			return regexp.MustCompile(`\b\d{6}\b`).FindString(s.sent[i].text)
		}
	}
	t.Fatalf("no email to %s", to)
	return ""
}

func TestPasswordResetIntegration(t *testing.T) {
	pool := requireIntegrationPool(t)
	engine := buildIntegrationServerWith(t, pool, integrationOptions{}).Engine()
	const email, old, fresh = "reset-me@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	sess := integrationOnboardAccount(t, engine, email, old, "Reset Me", "reset_me")

	users := postgres.NewUserRepo(pool)
	box := &captureSender{}
	h := NewPasswordResetHandler(users, users, auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), time.Hour),
		postgres.NewPasswordResetRepo(pool), box, integrationJWTSecret, 15*time.Minute)
	resetEngine := gin.New()
	h.Register(resetEngine.Group("/v1"))
	call := func(path string, body any, want int) map[string]any {
		t.Helper()
		rec := integrationDo(t, resetEngine, http.MethodPost, path, "", body)
		if rec.Code != want {
			t.Fatalf("%s: %d %s, want %d", path, rec.Code, rec.Body.String(), want)
		}
		return decodeBody(t, rec)
	}

	// An unknown email gets the same answer and no email.
	call("/v1/auth/password/forgot", map[string]any{"email": "nobody@example.com"}, http.StatusOK)
	if len(box.sent) != 0 {
		t.Fatalf("emailed a stranger: %v", box.sent)
	}
	call("/v1/auth/password/forgot", map[string]any{"email": "  RESET-me@Example.com "}, http.StatusOK)
	code := box.lastCode(t, email)
	if code == "" || !strings.Contains(box.sent[0].subject, code) {
		t.Fatalf("code email: %+v", box.sent)
	}

	// A weak password, a wrong code, an unknown email: refused.
	if got := call("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": "weak"}, http.StatusBadRequest); got["field"] != "new_password" {
		t.Fatalf("weak: %v", got)
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if got := call("/v1/auth/password/reset", map[string]any{"email": email, "code": wrong, "new_password": fresh}, http.StatusBadRequest); got["field"] != "code" {
		t.Fatalf("wrong code: %v", got)
	}
	call("/v1/auth/password/reset", map[string]any{"email": "nobody@example.com", "code": code, "new_password": fresh}, http.StatusBadRequest)

	// The right code: a new session, the new password works, the old one and
	// the old session do not, and the code is used up.
	got := call("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": fresh}, http.StatusOK)
	if got["access_token"] == "" || dig(t, got, "user", "username") != "reset_me" {
		t.Fatalf("session: %v", got)
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": fresh}); rec.Code != http.StatusOK {
		t.Fatalf("new password: %d", rec.Code)
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code == http.StatusOK {
		t.Fatal("old password still works")
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": sess.RefreshToken}); rec.Code == http.StatusOK {
		t.Fatal("old session still works")
	}
	call("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": "Another-password-3!"}, http.StatusBadRequest)

	// Five wrong tries use the code up, even for the right code after.
	call("/v1/auth/password/forgot", map[string]any{"email": email}, http.StatusOK)
	code = box.lastCode(t, email)
	if code == wrong {
		wrong = "222222"
	}
	for i := 0; i < 5; i++ {
		call("/v1/auth/password/reset", map[string]any{"email": email, "code": wrong, "new_password": "Another-password-3!"}, http.StatusBadRequest)
	}
	call("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": "Another-password-3!"}, http.StatusBadRequest)
}
