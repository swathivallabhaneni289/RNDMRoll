package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/codec/json"
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

// KeyByIPAndBody keys the limiter by the client IP combined with the name a
// JSON body is for (an email or a login), trimmed and lowercased as the
// handlers compare it, so throttling applies per account as well as per
// source and padding or capitals never make a fresh bucket. pick reads the
// name from the body decoded into T, the handler's own request type, exactly
// as the handler decodes it: encoding/json matches keys in any case and lets
// the last of two win, so reading the body any other way could count one name
// while the handler uses another. The binding rules are left to the handler,
// so a body it will refuse still counts against the name it carries. The body
// is read with ShouldBindBodyWith, which caches it so the handler can still
// bind it afterward. A body that does not decode is keyed by the IP alone.
func KeyByIPAndBody[T any](pick func(*T) string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		var body T
		if err := c.ShouldBindBodyWith(&body, decodeOnly{}); err != nil {
			return c.ClientIP()
		}
		return c.ClientIP() + "|" + strings.ToLower(strings.TrimSpace(pick(&body)))
	}
}

// decodeOnly decodes a JSON body as binding.JSON does (gin's JSON codec and
// its two decoder switches) without the binding rules, which only the
// handler applies.
type decodeOnly struct{}

func (decodeOnly) Name() string { return "json" }

func (d decodeOnly) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return errors.New("invalid request")
	}
	return d.decode(req.Body, obj)
}

func (d decodeOnly) BindBody(body []byte, obj any) error {
	return d.decode(bytes.NewReader(body), obj)
}

func (decodeOnly) decode(r io.Reader, obj any) error {
	decoder := json.API.NewDecoder(r)
	if binding.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if binding.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	return decoder.Decode(obj)
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
