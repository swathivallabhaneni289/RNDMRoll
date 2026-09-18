// Package httpapi: username.go implements the advisory username-suggestion
// and availability endpoints. Both sit behind authentication and are rate
// limited by subject to blunt use as an account-enumeration oracle
// (RESEARCH.md Security Domain). Neither is authoritative -- the save-time
// unique-constraint conflict handled in profile.go's patchMe is what
// actually decides a username, per PATTERNS.md's race-condition note.
package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// UsernameHandler serves GET /usernames/suggest and GET /usernames/available.
type UsernameHandler struct {
	users user.Repository
}

// NewUsernameHandler constructs a UsernameHandler.
func NewUsernameHandler(users user.Repository) *UsernameHandler {
	return &UsernameHandler{users: users}
}

// Register mounts this handler's routes on rg, rate limited to 20 requests
// per minute keyed by the authenticated subject. The caller is responsible
// for attaching RequireAuth -- and, in production, RequireVerified -- to rg
// before calling Register; see plan 01-13.
func (h *UsernameHandler) Register(rg *gin.RouterGroup) {
	limited := rg.Group("/usernames")
	limited.Use(middleware.RateLimit(middleware.LimitConfig{
		Requests: 20,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyBySubject,
	}))
	limited.GET("/suggest", h.suggest)
	limited.GET("/available", h.available)
}

// suggest responds with {username, alternates}: a free suggestion derived
// from the supplied display name, plus three further free alternates.
func (h *UsernameHandler) suggest(c *gin.Context) {
	name := c.Query("name")

	suggestion, err := user.SuggestUsername(c.Request.Context(), h.users, name)
	if err != nil {
		RespondError(c, err)
		return
	}

	base := user.NormalizeUsername(name)
	alternates, err := user.SuggestAlternates(c.Request.Context(), h.users, base, 3)
	if err != nil {
		alternates = []string{}
	}

	Respond(c, http.StatusOK, gin.H{
		"username":   suggestion,
		"alternates": alternates,
	})
}

// available responds with {available, alternates}: whether the exact
// requested username is free, with alternates populated only when it is
// taken.
func (h *UsernameHandler) available(c *gin.Context) {
	candidate := c.Query("username")

	if err := user.ValidateUsername(candidate); err != nil {
		RespondValidationError(c, err)
		return
	}

	taken, err := h.users.UsernameTaken(c.Request.Context(), candidate)
	if err != nil {
		RespondError(c, err)
		return
	}

	if !taken {
		Respond(c, http.StatusOK, gin.H{"available": true, "alternates": []string{}})
		return
	}

	alternates, err := user.SuggestAlternates(c.Request.Context(), h.users, candidate, 3)
	if err != nil {
		alternates = []string{}
	}
	Respond(c, http.StatusOK, gin.H{"available": false, "alternates": alternates})
}
