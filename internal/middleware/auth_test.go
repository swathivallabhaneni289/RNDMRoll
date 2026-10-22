package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuth_MissingHeaderReturns401AndHandlerNeverRuns(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	handlerRan := false

	router := gin.New()
	router.GET("/protected", RequireAuth(secret), func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if handlerRan {
		t.Fatal("handler must not run when the Authorization header is missing")
	}
}

func TestAuth_ValidBearerTokenReachesHandlerWithSubject(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID := uuid.New()
	token, err := auth.IssueAccessToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	var gotSubject uuid.UUID
	router := gin.New()
	router.GET("/protected", RequireAuth(secret), func(c *gin.Context) {
		subject, err := SubjectFromContext(c)
		if err != nil {
			t.Errorf("SubjectFromContext returned error: %v", err)
		}
		gotSubject = subject
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotSubject != userID {
		t.Fatalf("expected subject %s, got %s", userID, gotSubject)
	}
}

func TestAuth_TokenSignedByAnotherSecretReturns401(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	otherSecret := []byte("a-totally-different-secret-here")
	token, err := auth.IssueAccessToken(uuid.New(), otherSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	router := gin.New()
	router.GET("/protected", RequireAuth(secret), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_SubjectFromContextErrorsWhenMiddlewareDidNotRun(t *testing.T) {
	router := gin.New()
	router.GET("/unprotected", func(c *gin.Context) {
		if _, err := SubjectFromContext(c); err == nil {
			t.Error("expected an error when RequireAuth did not run")
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unprotected", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// RequireAuth keeps the session a token names, and a token from before
// sessions were named still gets through, with none.
func TestAuth_SessionFromContext(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID, sessionID := uuid.New(), uuid.New()
	withSession, err := auth.IssueSessionAccessToken(userID, sessionID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueSessionAccessToken: %v", err)
	}
	without, err := auth.IssueAccessToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	type seen struct {
		subject uuid.UUID
		session uuid.UUID
		ok      bool
	}
	var got seen
	router := gin.New()
	router.GET("/protected", RequireAuth(secret), func(c *gin.Context) {
		got.subject, _ = SubjectFromContext(c)
		got.session, got.ok = SessionFromContext(c)
		c.Status(http.StatusOK)
	})
	call := func(token string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)
		return w.Code
	}

	if code := call(withSession); code != http.StatusOK || got != (seen{userID, sessionID, true}) {
		t.Fatalf("a session token: %d, %+v", code, got)
	}
	got = seen{}
	if code := call(without); code != http.StatusOK || got != (seen{userID, uuid.Nil, false}) {
		t.Fatalf("a token without a session: %d, %+v", code, got)
	}

	// Without RequireAuth there is no session either.
	plain := gin.New()
	plain.GET("/open", func(c *gin.Context) {
		if _, ok := SessionFromContext(c); ok {
			t.Error("a session without RequireAuth")
		}
		c.Status(http.StatusOK)
	})
	plain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/open", nil))
}
