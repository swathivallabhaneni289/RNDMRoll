package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newRecoveryTestLogger returns a JSON slog.Logger writing into buf, so a
// test can assert on exactly what reached the log without touching stderr.
func newRecoveryTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestRecovery_PanicReturns500WithGenericBody(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(Recovery(newRecoveryTestLogger(&buf)))
	router.GET("/boom", func(c *gin.Context) {
		panic("something exploded")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %s)", err, w.Body.String())
	}
	if body["error"] != "server_error" {
		t.Fatalf("expected error=server_error, got %v", body["error"])
	}
}

func TestRecovery_PanicBodyContainsNoStackTraceOrPanicMessage(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(Recovery(newRecoveryTestLogger(&buf)))
	router.GET("/boom", func(c *gin.Context) {
		panic("super-secret-panic-detail")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	router.ServeHTTP(w, req)

	respBody := w.Body.String()
	if strings.Contains(respBody, "super-secret-panic-detail") {
		t.Fatalf("response body leaks the panic message: %s", respBody)
	}
	if strings.Contains(respBody, "goroutine") || strings.Contains(respBody, ".go:") {
		t.Fatalf("response body looks like it contains a stack trace: %s", respBody)
	}
}

func TestRecovery_ProcessKeepsServingAfterPanic(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(Recovery(newRecoveryTestLogger(&buf)))
	router.GET("/boom", func(c *gin.Context) {
		panic("first request explodes")
	})
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on panicking route, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("expected the process to keep serving after a panic, got %d", w2.Code)
	}
}

func TestRecovery_LogsPanicValueAndStackServerSide(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(Recovery(newRecoveryTestLogger(&buf)))
	router.GET("/boom", func(c *gin.Context) {
		panic("logged-panic-value")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	logOutput := buf.String()
	if !strings.Contains(logOutput, "logged-panic-value") {
		t.Fatalf("expected the panic value to be logged server-side, got: %s", logOutput)
	}
}

func TestRecovery_NoPanicPassesThroughUnaffected(t *testing.T) {
	var buf bytes.Buffer
	router := gin.New()
	router.Use(Recovery(newRecoveryTestLogger(&buf)))
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output for a non-panicking request, got: %s", buf.String())
	}
}
