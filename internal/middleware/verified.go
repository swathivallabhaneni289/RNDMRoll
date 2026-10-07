package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// userContextKey is a package-private type distinct from auth.go's
// contextKey, so the user stored under it cannot collide with the
// authenticated subject RequireAuth stores.
type userContextKey int

const loadedUserKey userContextKey = iota

// ErrNoUserInContext is returned by UserFromContext when RequireUser did
// not run for this request.
var ErrNoUserInContext = errors.New("middleware: no user in context")

// RequireUser returns Gin middleware that requires the authenticated
// subject (set by a preceding RequireAuth) to resolve to an account that
// still exists. A valid token for an account since deleted (for example an
// under-13 social account the server removed) aborts with 401 token_invalid.
//
// There is deliberately no email-verified check: sign-up is one page with
// no email check, so an unverified account must be able to use the app. On
// success the loaded *user.User is stored in the request context for
// UserFromContext, so downstream handlers reuse the record instead of
// issuing a second lookup for the same row.
//
// This gate says only "a person with an account". An unfinished Apple or
// Google account passes it, so a route that needs a finished profile must
// also check User.OnboardingComplete().
func RequireUser(repo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, err := SubjectFromContext(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_invalid"})
			return
		}

		u, err := repo.GetByID(c.Request.Context(), subject)
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_invalid"})
				return
			}
			// A database blip must not look like "your account is gone":
			// the app signs the person out on token_invalid.
			log.Printf("middleware: RequireUser lookup failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), loadedUserKey, u)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// UserFromContext returns the *user.User RequireUser loaded for this
// request.
func UserFromContext(c *gin.Context) (*user.User, error) {
	value := c.Request.Context().Value(loadedUserKey)
	u, ok := value.(*user.User)
	if !ok {
		return nil, ErrNoUserInContext
	}
	return u, nil
}
