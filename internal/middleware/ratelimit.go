package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/time/rate"
)

// LimitConfig configures a token-bucket rate limit: Requests allowed per
// Window, keyed by KeyFunc.
type LimitConfig struct {
	Requests int
	Window   time.Duration
	KeyFunc  func(*gin.Context) string
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns Gin middleware enforcing cfg over one *rate.Limiter per
// key, held in a mutex-guarded map. Entries idle for ten windows are
// evicted so the map cannot grow without bound under a rotating source of
// keys (e.g. spoofed IPs). On rejection it aborts with 429 and
// {"error":"rate_limited"}.
func RateLimit(cfg LimitConfig) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*limiterEntry)
	evictAfter := cfg.Window * 10
	limit := rate.Limit(float64(cfg.Requests) / cfg.Window.Seconds())

	return func(c *gin.Context) {
		key := cfg.KeyFunc(c)

		mu.Lock()
		now := time.Now()
		entry, ok := entries[key]
		if !ok {
			entry = &limiterEntry{limiter: rate.NewLimiter(limit, cfg.Requests)}
			entries[key] = entry
		}
		entry.lastSeen = now
		allowed := entry.limiter.Allow()

		for k, e := range entries {
			if now.Sub(e.lastSeen) > evictAfter {
				delete(entries, k)
			}
		}
		mu.Unlock()

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
			return
		}
		c.Next()
	}
}

// KeyByIP keys the limiter by the request's client IP.
func KeyByIP(c *gin.Context) string {
	return c.ClientIP()
}

// KeyByIPAndField keys the limiter by the client IP combined with a
// lowercased JSON body field (e.g. "email"), so login throttling applies
// per account as well as per source. It reads the body via
// ShouldBindBodyWith, which caches the raw body so the handler can still
// bind it normally afterward.
func KeyByIPAndField(field string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		var body map[string]any
		if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
			return c.ClientIP()
		}
		value, _ := body[field].(string)
		return c.ClientIP() + "|" + strings.ToLower(value)
	}
}

// KeyBySubject keys the limiter by the authenticated subject, for routes
// mounted behind RequireAuth.
func KeyBySubject(c *gin.Context) string {
	subject, err := SubjectFromContext(c)
	if err != nil {
		return c.ClientIP()
	}
	return subject.String()
}
