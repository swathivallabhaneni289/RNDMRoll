package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestKeyByIPAndField_UsesTheFirstKeyThatHoldsText(t *testing.T) {
	key := KeyByIPAndField("login", "email")
	cases := []struct {
		name string
		body string
		want string
	}{
		{"first key", `{"login":"Some_Name"}`, "192.0.2.1|some_name"},
		{"older key", `{"email":"A@B.com"}`, "192.0.2.1|a@b.com"},
		{"first key wins", `{"login":"one","email":"two"}`, "192.0.2.1|one"},
		{"empty first key falls through", `{"login":"","email":"two"}`, "192.0.2.1|two"},
		{"neither key", `{"password":"x"}`, "192.0.2.1|"},
		{"not json", `nope`, "192.0.2.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			if got := key(c); got != tc.want {
				t.Fatalf("key for %s: got %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}
