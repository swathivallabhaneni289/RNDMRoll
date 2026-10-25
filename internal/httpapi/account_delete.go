// Package httpapi: account_delete.go implements POST /me/delete, delete
// account, like Instagram (developer, 2026-10-25). The signed-in person gives
// their password; the account is hidden from everyone at once and every
// session of it ends; it is deleted for good once user.DeletionGracePeriod
// has passed (internal/purge). Logging back in before then keeps it
// (AuthHandler.Login), and so does a Forgot password reset. Like the other
// account routes it finds the caller only from the token's subject.
package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// DeleteAccountRequest is the POST /me/delete body. It carries no binding
// tags on purpose: deleteAccount checks the one field by hand, so the
// refusal names it.
type DeleteAccountRequest struct {
	Password string `json:"password"`
}

// DeleteAccountHandler serves POST /me/delete.
type DeleteAccountHandler struct {
	users     user.Repository
	deletions user.AccountDeleter
	// clock is the time the limits count by; nil is limitClock (the real
	// clock unless a test froze it). A test sets it before sending anything.
	clock func() time.Time
}

// NewDeleteAccountHandler constructs a DeleteAccountHandler. deletions is the
// one transaction that marks the account and ends its sessions (the Postgres
// user repository). NewServer also hands it to log in, which cancels a
// deletion inside its grace period.
func NewDeleteAccountHandler(users user.Repository, deletions user.AccountDeleter) *DeleteAccountHandler {
	return &DeleteAccountHandler{users: users, deletions: deletions}
}

// now is the limits' clock: clock, or limitClock.
func (h *DeleteAccountHandler) now() time.Time {
	if h.clock != nil {
		return h.clock()
	}
	return limitClock()
}

// Register mounts POST /me/delete on rg. The caller attaches RequireAuth and
// RequireUser to rg first (NewServer applies both to the signed-in group).
//
// The route is limited as POST /me/password is, so a stolen access token
// cannot be used to guess the password: 5 a minute per caller, then 20 a
// minute per address. Both answer 429 {"error":"rate_limited"}.
func (h *DeleteAccountHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/me/delete", middleware.RateLimit(middleware.LimitConfig{
		Requests: 5,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyBySubject,
		Clock:    h.now,
	}), middleware.RateLimit(middleware.LimitConfig{
		Requests: 20,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
		Clock:    h.now,
	}), h.deleteAccount)
}

// deleteAccount asks for the caller's account to be deleted. Order: the
// caller; the body (400 validation_failed, or 413 over the size cap);
// password present (400 field password, reason required); the password
// compared with bcrypt against the stored hash (an account with none
// compares against the dummy hash, so the work is the same, and is refused
// all the same: 400 field password, reason incorrect, never a 401, which
// would sign the phone out); then ONE transaction that marks the account,
// revokes every refresh token, and deletes its push tokens and any Forgot
// password code; then the answer.
//
// The answer is when the account will be deleted for good: delete_after, an
// RFC 3339 instant worked out by the database from the moment it stored, and
// delete_on_label, that day written in the person's stored time zone (UTC
// when they have none), because the app formats no date. The access token
// this request came with stops working at once (RequireUser), and the app
// signs out.
//
// No password is ever put in a response or written anywhere.
func (h *DeleteAccountHandler) deleteAccount(c *gin.Context) {
	u, ok := callerAccount(c, h.users)
	if !ok {
		return
	}

	var req DeleteAccountRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if req.Password == "" {
		RespondFieldError(c, "password", reasonRequired)
		return
	}
	hasHash := u.PasswordHash != nil && *u.PasswordHash != ""
	stored := dummyPasswordHash
	if hasHash {
		stored = *u.PasswordHash
	}
	if err := auth.ComparePassword(stored, req.Password); err != nil || !hasHash {
		RespondFieldError(c, "password", reasonIncorrect)
		return
	}

	asked, err := h.deletions.RequestDeletion(c.Request.Context(), u.ID)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrNotFound):
			// The account went away between the token check and the ask.
			RespondError(c, user.ErrTokenInvalid)
		case errors.Is(err, user.ErrAccountDisabled):
			// Disabled, or already asked for, while this ran: what
			// RequireUser answers such an account.
			Respond(c, http.StatusUnauthorized, gin.H{"error": string(CodeAccountDisabled)})
		default:
			RespondError(c, err)
		}
		return
	}

	loc := time.UTC
	if l, err := spin.ParseZone(asked.Zone); err == nil {
		loc = l
	}
	Respond(c, http.StatusOK, gin.H{
		"delete_after":    asked.DeleteAfter.UTC().Format(time.RFC3339),
		"delete_on_label": spin.LongDateLabel(asked.DeleteAfter, loc),
	})
}
