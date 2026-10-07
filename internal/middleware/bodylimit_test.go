package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newBodyLimitRouter(max int64, readErr *error) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(max))
	router.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if readErr != nil {
			*readErr = err
		}
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.String(http.StatusOK, "%d", len(body))
	})
	return router
}

func TestBodyLimit_UnderTheCapPasses(t *testing.T) {
	router := newBodyLimitRouter(100, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("a", 100))))
	if w.Code != http.StatusOK || w.Body.String() != "100" {
		t.Fatalf("expected 200 and 100 bytes, got %d %q", w.Code, w.Body.String())
	}
}

func TestBodyLimit_OversizeContentLengthIs413BeforeTheHandlerRuns(t *testing.T) {
	handlerRan := false
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(100))
	router.POST("/echo", func(c *gin.Context) { handlerRan = true })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("a", 101))))

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "payload_too_large") {
		t.Fatalf("expected payload_too_large body, got %s", w.Body.String())
	}
	if handlerRan {
		t.Fatal("handler must not run")
	}
}

func TestBodyLimit_ChunkedOversizeBodyFailsWhenRead(t *testing.T) {
	var readErr error
	router := newBodyLimitRouter(100, &readErr)

	req := httptest.NewRequest(http.MethodPost, "/echo", io.NopCloser(strings.NewReader(strings.Repeat("a", 500))))
	req.ContentLength = -1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if readErr == nil {
		t.Fatal("expected the read to fail past the cap")
	}
	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) {
		t.Fatalf("expected *http.MaxBytesError, got %T: %v", readErr, readErr)
	}
}

func TestBodyLimit_UnderstatedContentLengthStillCapped(t *testing.T) {
	var readErr error
	router := newBodyLimitRouter(100, &readErr)

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("a", 500)))
	req.ContentLength = 10 // a lie
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if readErr == nil {
		t.Fatal("expected the read to fail past the cap even when Content-Length lied")
	}
}
