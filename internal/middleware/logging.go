package middleware

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryKeys are query parameter names whose value must never
// appear in a log line verbatim. This is the one place that rule is
// encoded, rather than left to be remembered at every call site: a token or
// password written to a log file is a durable credential disclosure that
// outlives the request (T-01-SRV-02). The email-verification callback route
// (GET .../verify-email/callback?token=...) is the concrete case this
// guards -- RequestLogger never logs the raw query string, only this
// redacted form.
var sensitiveQueryKeys = map[string]struct{}{
	"token":          {},
	"password":       {},
	"authorization":  {},
	"refresh_token":  {},
	"access_token":   {},
	"identity_token": {},
	"id_token":       {},
	"code":           {},
	// The username endpoints are public, so a name or username typed by
	// someone who is not signed up yet must not reach the logs either.
	"name":     {},
	"username": {},
}

// redactSensitiveQuery parses raw as a URL query string and returns it
// re-encoded with every sensitive key's value replaced by "[REDACTED]".
// Malformed input degrades to an empty string rather than ever risking a
// partially-redacted value reaching the log.
func redactSensitiveQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	for key := range values {
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(key)]; sensitive {
			values.Set(key, "[REDACTED]")
		}
	}
	return values.Encode()
}

// RequestLogger returns Gin middleware that writes one structured line per
// request: method, path, status, duration, and client IP. It never logs a
// request header (Authorization included) or any request body field, so a
// password or token submitted in a request body can never reach a log line
// through this middleware regardless of the route. The only per-request
// input it derives a logged value from at all is the query string, and only
// through redactSensitiveQuery.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactSensitiveQuery(c.Request.URL.RawQuery)

		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if query != "" {
			attrs = append(attrs, "query", query)
		}
		logger.Info("request", attrs...)
	}
}
