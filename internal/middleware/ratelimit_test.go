package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

// keyTestBody stands in for a handler's request type, binding rule and all.
type keyTestBody struct {
	Login    string `json:"login"`
	Password string `json:"password" binding:"required"`
}

func TestKeyByIPAndBody_ReadsTheNameAsTheHandlerDecodesIt(t *testing.T) {
	key := KeyByIPAndBody(func(b *keyTestBody) string { return b.Login })
	cases := []struct {
		name string
		body string
		want string
	}{
		{"the name", `{"login":"Some_Name","password":"x"}`, "192.0.2.1|some_name"},
		// Trimmed and lowercased as the handlers compare it, so padding is not
		// a new bucket.
		{"padded", `{"login":"\tSome_Name\n","password":"x"}`, "192.0.2.1|some_name"},
		// Keys match in any case and the last of two wins, as in the handler.
		{"key in capitals", `{"LOGIN":"victim","password":"x"}`, "192.0.2.1|victim"},
		{"decoy, then capitals", `{"login":"decoy","Login":"victim","password":"x"}`, "192.0.2.1|victim"},
		{"capitals, then decoy", `{"Login":"victim","login":"decoy","password":"x"}`, "192.0.2.1|decoy"},
		{"the same key twice", `{"login":"decoy","password":"x","login":"victim"}`, "192.0.2.1|victim"},
		{"empty, then capitals", `{"login":"","lOgIn":"victim","password":"x"}`, "192.0.2.1|victim"},
		// What follows the body is ignored, as the handler's decoder ignores it.
		{"a second body after", `{"login":"victim","password":"x"} {"login":"decoy"}`, "192.0.2.1|victim"},
		// The binding rules are the handler's: a body it will refuse still
		// counts against its name.
		{"no password", `{"login":"Some_Name"}`, "192.0.2.1|some_name"},
		{"no name", `{"password":"x"}`, "192.0.2.1|"},
		{"not json", `nope`, "192.0.2.1"},
		{"not text", `{"login":5,"password":"x"}`, "192.0.2.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			if got := key(c); got != tc.want {
				t.Fatalf("key for %s: got %q, want %q", tc.body, got, tc.want)
			}
			// The handler still binds the body afterward and, when it accepts
			// it, reads the same name.
			var bound keyTestBody
			if err := c.ShouldBindBodyWith(&bound, binding.JSON); err == nil {
				if got := "192.0.2.1|" + strings.ToLower(strings.TrimSpace(bound.Login)); got != tc.want {
					t.Fatalf("the handler read %q, the key %q", got, tc.want)
				}
			}
		})
	}
}
