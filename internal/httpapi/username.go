// Package httpapi: username.go implements the advisory username-suggestion
// and availability endpoints. Both need NO token (the sign-up page calls
// them before an account exists) and are rate limited per IP to blunt use as
// an account-enumeration oracle (RESEARCH.md Security Domain). Neither is
// authoritative -- the save-time unique-constraint conflict handled in
// profile.go's patchMe and in sign-up is what actually decides a username,
// per PATTERNS.md's race-condition note.
package httpapi

import (
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// Input ceilings, checked before any query so a huge value never reaches the
// database. A username is at most 20 characters; a display name is capped at
// 100 here (looser than a saved name's 50, since this only seeds a
// suggestion).
const (
	maxSuggestNameLength = 100
	maxUsernameLength    = 20
)

// UsernameHandler serves GET /usernames/suggest and GET /usernames/available.
type UsernameHandler struct {
	users user.Repository
}

// NewUsernameHandler constructs a UsernameHandler.
func NewUsernameHandler(users user.Repository) *UsernameHandler {
	return &UsernameHandler{users: users}
}

// Register mounts this handler's routes on rg, rate limited to 90 requests
// per minute keyed by client IP. The routes are public: rg carries no
// authentication middleware.
func (h *UsernameHandler) Register(rg *gin.RouterGroup) {
	limited := rg.Group("/usernames")
	limited.Use(middleware.RateLimit(middleware.LimitConfig{
		Requests: 90,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
	}))
	limited.GET("/suggest", h.suggest)
	limited.GET("/available", h.available)
}

// suggest responds with {username, alternates}: a free suggestion derived
// from the supplied display name, plus three further free alternates.
func (h *UsernameHandler) suggest(c *gin.Context) {
	name := c.Query("name")
	if utf8.RuneCountInString(name) > maxSuggestNameLength {
		RespondFieldError(c, "name", reasonTooLong)
		return
	}

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
	if utf8.RuneCountInString(candidate) > maxUsernameLength {
		RespondFieldError(c, "username", reasonTooLong)
		return
	}

	if err := user.ValidateUsername(candidate); err != nil {
		RespondFieldError(c, "username", reasonInvalid)
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
