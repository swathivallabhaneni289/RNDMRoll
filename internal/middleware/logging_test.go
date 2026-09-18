package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newLoggingTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestRequestLogger_LogsMethodPathStatusAndDuration(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(newLoggingTestLogger(&buf)))
	router.GET("/widgets/123", func(c *gin.Context) {
		c.Status(http.StatusTeapot)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/widgets/123", nil))

	logLine := buf.String()
	for _, want := range []string{"GET", "/widgets/123", "418", "duration_ms"} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("expected log line to contain %q, got: %s", want, logLine)
		}
	}
}

func TestRequestLogger_NeverLogsAuthorizationHeaderOrPasswordFieldValue(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(newLoggingTestLogger(&buf)))
	router.POST("/v1/auth/login", func(c *gin.Context) {
		// Deliberately drain the body, mirroring a real handler, to prove
		// the logger itself never inspects it -- not just that nobody
		// happened to read it in this test.
		var body map[string]any
		_ = c.ShouldBindJSON(&body)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"swathi@example.com","password":"hunter2-super-secret"}`))
	req.Header.Set("Authorization", "Bearer super-secret-bearer-token-value")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	logLine := buf.String()
	if strings.Contains(logLine, "super-secret-bearer-token-value") {
		t.Fatalf("log line leaks the Authorization header value: %s", logLine)
	}
	if strings.Contains(logLine, "hunter2-super-secret") {
		t.Fatalf("log line leaks the password field value: %s", logLine)
	}
}

func TestRequestLogger_RedactsSensitiveQueryParamValues(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(newLoggingTestLogger(&buf)))
	router.GET("/v1/auth/verify-email/callback", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify-email/callback?token=extremely-sensitive-token-value", nil)
	router.ServeHTTP(w, req)

	logLine := buf.String()
	if strings.Contains(logLine, "extremely-sensitive-token-value") {
		t.Fatalf("log line leaks a sensitive query parameter value: %s", logLine)
	}
}

func TestRedactSensitiveQuery_KeepsNonSensitiveParamsAndRedactsSensitiveOnes(t *testing.T) {
	got := redactSensitiveQuery("name=Swathi&token=abc123&code=xyz789")
	if strings.Contains(got, "abc123") || strings.Contains(got, "xyz789") {
		t.Fatalf("expected sensitive query values to be redacted, got: %s", got)
	}
	if !strings.Contains(got, "Swathi") {
		t.Fatalf("expected non-sensitive query values to survive redaction, got: %s", got)
	}
}

func TestRequestLogger_LogsClientIP(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(newLoggingTestLogger(&buf)))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	router.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "203.0.113.7") {
		t.Fatalf("expected log line to contain the client IP, got: %s", buf.String())
	}
}
