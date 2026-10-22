// Package httpapi: account.go implements the account routes of the signed-in
// person that are not profile fields. There is one: POST /me/password, change
// password. Like the profile routes it finds the caller only from the token's
// subject; no request field names an account.
package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// ChangePasswordRequest is the POST /me/password body. It carries no binding
// tags on purpose: changePassword checks each field by hand, in a fixed order,
// so every refusal names its field.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// AccountHandler serves POST /me/password.
type AccountHandler struct {
	users     user.Repository
	passwords user.PasswordChanger
	refresh   *auth.RefreshService
	jwtSecret []byte
	accessTTL time.Duration
	// clock is the time the limits count by; nil is the real clock. A test
	// sets it before sending anything.
	clock func() time.Time
}

// now is the limits' clock: clock, or the real one.
func (h *AccountHandler) now() time.Time {
	if h.clock != nil {
		return h.clock()
	}
	return time.Now()
}

// NewAccountHandler constructs an AccountHandler. passwords is the one
// transaction that stores the new hash and replaces the account's sessions
// (the Postgres user repository); refresh makes the new session's token;
// jwtSecret and accessTTL mint the new access token, exactly as log in does.
func NewAccountHandler(users user.Repository, passwords user.PasswordChanger, refresh *auth.RefreshService, jwtSecret []byte, accessTTL time.Duration) *AccountHandler {
	return &AccountHandler{
		users:     users,
		passwords: passwords,
		refresh:   refresh,
		jwtSecret: jwtSecret,
		accessTTL: accessTTL,
	}
}

// Register mounts POST /me/password on rg. The caller attaches RequireAuth and
// RequireUser to rg first (NewServer applies both to the signed-in group).
//
// The route is limited twice, so a stolen access token cannot be used to
// guess the current password: 5 a minute per caller, then 20 a minute per
// address. The caller's limit comes first, so one person's retries do not use
// up the address's share for everyone else behind it. Both answer 429
// {"error":"rate_limited"}.
func (h *AccountHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/me/password", middleware.RateLimit(middleware.LimitConfig{
		Requests: 5,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyBySubject,
		Clock:    h.now,
	}), middleware.RateLimit(middleware.LimitConfig{
		Requests: 20,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
		Clock:    h.now,
	}), h.changePassword)
}

// changePassword replaces the caller's password. Order: the caller; the body
// (400 validation_failed, or 413 over the size cap); current_password present;
// new_password by the one password rule; the current password compared with
// bcrypt (an account with no stored hash compares against the dummy hash and
// is refused all the same); the new password refused when it equals the
// current one; the new password hashed (bcrypt counts 72 BYTES, so a long
// multi-byte one is a field error on new_password); then ONE transaction that
// stores the hash, revokes every refresh token the account holds and inserts
// one new one; then the response. An account an admin disabled while this
// ran is refused inside that transaction, as RequireUser refuses it (401
// account_disabled), and keeps its password.
//
// A wrong current password is 400 validation_failed field current_password,
// never a 401: a 401 on a signed-in call means "your session is gone" and the
// phone signs the person out. The answer is the same for every way of being
// wrong. Success answers with the same body as log in, so this phone keeps
// its session by saving the new tokens; every other phone's refresh token is
// gone at once, and its short access token dies within the access token's
// lifetime.
//
// No password is ever put in a response or written anywhere: this handler
// reports only unexpected errors, through RespondError, which never include
// the request.
func (h *AccountHandler) changePassword(c *gin.Context) {
	u, ok := callerAccount(c, h.users)
	if !ok {
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if req.CurrentPassword == "" {
		RespondFieldError(c, "current_password", reasonRequired)
		return
	}
	if reason := passwordReason(req.NewPassword); reason != "" {
		RespondFieldError(c, "new_password", reason)
		return
	}

	// Compare against the stored hash. An account with none (or an empty one)
	// compares against the dummy hash so the work is the same, and is refused
	// whatever was typed: the dummy is never used as if it were the account's
	// own password.
	hasHash := u.PasswordHash != nil && *u.PasswordHash != ""
	stored := dummyPasswordHash
	if hasHash {
		stored = *u.PasswordHash
	}
	if err := auth.ComparePassword(stored, req.CurrentPassword); err != nil || !hasHash {
		RespondFieldError(c, "current_password", reasonIncorrect)
		return
	}
	if req.NewPassword == req.CurrentPassword {
		RespondFieldError(c, "new_password", reasonUnchanged)
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooLong) {
			RespondFieldError(c, "new_password", reasonTooLong)
			return
		}
		RespondError(c, err)
		return
	}

	// The access token is made before anything is stored: it cannot fail
	// after the sessions are gone and leave the caller with nothing. It names
	// the new session, whose id is chosen here for that.
	session := uuid.New()
	accessToken, err := auth.IssueSessionAccessToken(u.ID, session, h.jwtSecret, h.accessTTL)
	if err != nil {
		RespondError(c, err)
		return
	}

	ctx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	refreshToken, err := h.refresh.IssueInTransaction(ctx, func(tokenHash []byte, expiresAt time.Time) error {
		return h.passwords.ChangePassword(ctx, u.ID, newHash, session, tokenHash, expiresAt, &userAgent)
	})
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			// The account went away between the token check and the change.
			RespondError(c, user.ErrTokenInvalid)
			return
		}
		if errors.Is(err, user.ErrAccountDisabled) {
			Respond(c, http.StatusUnauthorized, gin.H{"error": string(CodeAccountDisabled)})
			return
		}
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
		"user":          apiUser(u),
	})
}
