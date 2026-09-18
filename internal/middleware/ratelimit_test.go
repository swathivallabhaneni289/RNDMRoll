package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimit_AllowsNThenRejectsTheNextWithinWindow(t *testing.T) {
	cfg := LimitConfig{
		Requests: 3,
		Window:   time.Second,
		KeyFunc:  KeyByIP,
	}
	router := gin.New()
	router.GET("/ping", RateLimit(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < cfg.Requests; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 past the configured limit, got %d", w.Code)
	}
}

func TestRateLimit_DifferentKeysDoNotShareABucket(t *testing.T) {
	cfg := LimitConfig{
		Requests: 1,
		Window:   time.Second,
		KeyFunc: func(c *gin.Context) string {
			return c.GetHeader("X-Test-Key")
		},
	}
	router := gin.New()
	router.GET("/ping", RateLimit(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req1.Header.Set("X-Test-Key", "key-a")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("key-a: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.Header.Set("X-Test-Key", "key-b")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("key-b: expected 200 (independent bucket), got %d", w2.Code)
	}
}
