package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// verifiedContextKey is a package-private type distinct from auth.go's
// contextKey, so the verified user stored under it cannot collide with the
// authenticated subject RequireAuth stores.
type verifiedContextKey int

const verifiedUserKey verifiedContextKey = iota

// ErrNoUserInContext is returned by UserFromContext when RequireVerified did
// not run for this request.
var ErrNoUserInContext = errors.New("middleware: no verified user in context")

// RequireVerified returns Gin middleware that requires the authenticated
// subject (set by a preceding RequireAuth) to resolve to a verified
// account. This is what makes D-04 a system property rather than a UI
// convention: the onboarding UI already declines to advance an unverified
// user, but a gate at the route boundary means a direct API call cannot
// bypass the decision either.
//
// A subject that resolves to no account (for example a token minted for an
// account since deleted) aborts with 401 token_invalid. An account whose
// EmailVerified is false aborts with 403 email_not_verified. On success the
// loaded *user.User is stored in the request context for UserFromContext,
// so downstream handlers reuse the record instead of issuing a second
// lookup for the same row.
//
// Social accounts (Apple/Google) are created with EmailVerified already
// true and a provider verification source, so they pass this gate on their
// first request without ever receiving a verification email, matching the
// branch of RESEARCH.md Open Question 1 this phase adopts.
func RequireVerified(repo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, err := SubjectFromContext(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_invalid"})
			return
		}

		u, err := repo.GetByID(c.Request.Context(), subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_invalid"})
			return
		}

		if !u.EmailVerified {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "email_not_verified"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), verifiedUserKey, u)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// UserFromContext returns the *user.User RequireVerified loaded for this
// request.
func UserFromContext(c *gin.Context) (*user.User, error) {
	value := c.Request.Context().Value(verifiedUserKey)
	u, ok := value.(*user.User)
	if !ok {
		return nil, ErrNoUserInContext
	}
	return u, nil
}
