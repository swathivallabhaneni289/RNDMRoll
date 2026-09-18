// Package middleware implements Gin request middleware shared by every
// handler plan: bearer-token authentication and per-key rate limiting.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// contextKey is a package-private type so authSubjectKey cannot collide
// with a context key set by another package.
type contextKey int

const authSubjectKey contextKey = iota

// ErrNoSubjectInContext is returned by SubjectFromContext when RequireAuth
// did not run for this request.
var ErrNoSubjectInContext = errors.New("middleware: no authenticated subject in context")

// RequireAuth returns Gin middleware that requires a valid HS256 bearer
// access token signed with secret. On success it stores the token's
// subject in the request context for SubjectFromContext to retrieve. On
// failure it aborts with 401 and a body of {"error":"token_expired"} for a
// specifically expired token or {"error":"token_invalid"} otherwise --
// codes the mobile client branches on to trigger its refresh flow.
func RequireAuth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_invalid"})
			return
		}
		tokenString := strings.TrimPrefix(header, prefix)

		subject, err := auth.ParseAccessToken(tokenString, secret)
		if err != nil {
			code := "token_invalid"
			if errors.Is(err, user.ErrTokenExpired) {
				code = "token_expired"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": code})
			return
		}

		ctx := context.WithValue(c.Request.Context(), authSubjectKey, subject)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// SubjectFromContext returns the authenticated user ID RequireAuth stored
// for this request. It returns ErrNoSubjectInContext rather than a zero
// UUID when the key is absent: PATTERNS.md's access-control rule requires
// GET/PATCH /me to derive the target user exclusively from the token
// subject, and a silent zero UUID would turn a missing-middleware wiring
// bug into a data-exposure bug.
func SubjectFromContext(c *gin.Context) (uuid.UUID, error) {
	value := c.Request.Context().Value(authSubjectKey)
	subject, ok := value.(uuid.UUID)
	if !ok {
		return uuid.UUID{}, ErrNoSubjectInContext
	}
	return subject, nil
}
