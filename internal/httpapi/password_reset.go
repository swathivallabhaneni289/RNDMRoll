// Package httpapi: password_reset.go serves Forgot password (developer,
// 2026-10-21): POST /auth/password/forgot {email} emails a 6-digit code, and
// POST /auth/password/reset {email, code, new_password} sets the new password
// and signs the person in. No link is needed, so it works before the real web
// address exists.
//
//   - forgot always answers 200 {}, whether or not the email has an account,
//     so it never tells a stranger who is signed up.
//   - Only an HMAC of the code is stored. A code lives 30 minutes, allows 5
//     wrong tries, and a new request replaces it.
//   - A wrong code, an unknown email and a used-up code all answer the same
//     400 field "code".
//   - The new password follows the sign-up rules; setting it ends every other
//     session (as Change password does) and the answer is a fresh session.
//   - A disabled account gets no code and cannot reset.
package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

const (
	resetCodeTTL      = 30 * time.Minute
	resetMaxAttempts  = 5
	resetCodeDigits   = 6
	resetSendDeadline = 10 * time.Second
)

// ResetStore keeps the Forgot password codes (postgres.PasswordResetRepo).
type ResetStore interface {
	Put(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at time.Time) error
	Get(ctx context.Context, userID uuid.UUID) (postgres.PasswordReset, bool, error)
	CountTry(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// PasswordResetHandler serves the two Forgot password routes.
type PasswordResetHandler struct {
	users     user.Repository
	passwords user.PasswordChanger
	refresh   *auth.RefreshService
	store     ResetStore
	sender    mail.Sender
	secret    []byte
	accessTTL time.Duration
	now       func() time.Time
}

// NewPasswordResetHandler builds the handler. sender delivers the code email.
func NewPasswordResetHandler(users user.Repository, passwords user.PasswordChanger, refresh *auth.RefreshService,
	store ResetStore, sender mail.Sender, secret []byte, accessTTL time.Duration) *PasswordResetHandler {
	return &PasswordResetHandler{users: users, passwords: passwords, refresh: refresh, store: store, sender: sender,
		secret: secret, accessTTL: accessTTL, now: time.Now}
}

// Register mounts the routes on the public /v1 group.
func (h *PasswordResetHandler) Register(rg *gin.RouterGroup) {
	limit := middleware.RateLimit(middleware.LimitConfig{
		Requests: 5, Window: time.Minute, KeyFunc: middleware.KeyByIPAndField("email"),
	})
	rg.POST("/auth/password/forgot", limit, h.forgot)
	rg.POST("/auth/password/reset", limit, h.reset)
}

// ForgotPasswordRequest is the POST /auth/password/forgot body.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the POST /auth/password/reset body.
type ResetPasswordRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

func (h *PasswordResetHandler) codeHash(userID uuid.UUID, code string) []byte {
	m := hmac.New(sha256.New, h.secret)
	m.Write([]byte("rndmroll-password-reset:" + userID.String() + ":" + code))
	return m.Sum(nil)
}

func newResetCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", resetCodeDigits, n.Int64()), nil
}

// resetMessage is the code email. Plain words, no em dashes.
func resetMessage(code string) (subject, html, text string) {
	subject = "Your RNDMRoll code: " + code
	body := "Use this code in the app to set a new password:"
	footer := "It expires in 30 minutes. If you did not ask for this, you can ignore this message and keep your password."
	text = fmt.Sprintf("%s\n\n%s\n\n%s", body, code, footer)
	html = fmt.Sprintf("<p>%s</p><p style=\"font-size:28px;letter-spacing:4px\"><strong>%s</strong></p><p>%s</p>", body, code, footer)
	return subject, html, text
}

func (h *PasswordResetHandler) forgot(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		RespondFieldError(c, "email", reasonInvalid)
		return
	}
	// The same answer whatever happens next, so nobody learns who has an
	// account.
	defer Respond(c, http.StatusOK, gin.H{})

	ctx := c.Request.Context()
	u, err := h.users.GetByEmailCI(ctx, email)
	if err != nil || u.Disabled || u.PasswordHash == nil || *u.PasswordHash == "" {
		return
	}
	code, err := newResetCode()
	if err != nil {
		log.Printf("httpapi: reset code: %v", err)
		return
	}
	now := h.now().UTC()
	if err := h.store.Put(ctx, u.ID, h.codeHash(u.ID, code), now.Add(resetCodeTTL), now); err != nil {
		log.Printf("httpapi: reset store: %v", err)
		return
	}
	subject, html, text := resetMessage(code)
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), resetSendDeadline)
	defer cancel()
	if err := h.sender.Send(sendCtx, u.Email, subject, html, text); err != nil {
		log.Printf("httpapi: reset email: %v", err)
	}
}

func (h *PasswordResetHandler) reset(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	code := strings.TrimSpace(req.Code)
	if len(code) != resetCodeDigits {
		RespondFieldError(c, "code", reasonInvalid)
		return
	}
	if reason := passwordReason(req.NewPassword); reason != "" {
		RespondFieldError(c, "new_password", reason)
		return
	}

	ctx := c.Request.Context()
	wrong := func() { RespondFieldError(c, "code", reasonIncorrect) }
	u, err := h.users.GetByEmailCI(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || u.Disabled {
		wrong()
		return
	}
	rec, ok, err := h.store.Get(ctx, u.ID)
	if err != nil {
		RespondError(c, err)
		return
	}
	if !ok || !h.now().Before(rec.ExpiresAt) || rec.Attempts >= resetMaxAttempts {
		wrong()
		return
	}
	if !hmac.Equal(rec.CodeHash, h.codeHash(u.ID, code)) {
		if err := h.store.CountTry(ctx, u.ID); err != nil {
			log.Printf("httpapi: reset count try: %v", err)
		}
		wrong()
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
	accessToken, err := auth.IssueAccessToken(u.ID, h.secret, h.accessTTL)
	if err != nil {
		RespondError(c, err)
		return
	}
	userAgent := c.Request.UserAgent()
	refreshToken, err := h.refresh.IssueInTransaction(ctx, func(tokenHash []byte, expiresAt time.Time) error {
		return h.passwords.ChangePassword(ctx, u.ID, newHash, tokenHash, expiresAt, &userAgent)
	})
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			wrong()
			return
		}
		RespondError(c, err)
		return
	}
	if err := h.store.Delete(ctx, u.ID); err != nil {
		log.Printf("httpapi: reset delete: %v", err)
	}
	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
		"user":          apiUser(u),
	})
}
