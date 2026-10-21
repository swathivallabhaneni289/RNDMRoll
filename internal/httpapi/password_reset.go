// Package httpapi: password_reset.go serves Forgot password (developer,
// 2026-10-21): POST /auth/password/forgot {email} emails a 6-digit code, and
// POST /auth/password/reset {email, code, new_password} sets the new password
// and signs the person in. No link is needed, so it works before the real web
// address exists.
//
//   - forgot always answers 200 {} at once, whether or not the email has an
//     account: the lookup and the email run after the answer, so neither the
//     answer nor its timing tells a stranger who is signed up.
//   - Only an HMAC of the code is stored. A code lives 30 minutes and allows
//     5 tries, which a new code never resets: a new request replaces the code
//     but keeps its tries, a used-up code gets no new one until it expires,
//     and a code under a minute old is not replaced (nothing is emailed).
//   - A wrong code, an unknown email and a used-up code all answer the same
//     400 field "code". A try is counted in the same step that reads the
//     code, and the right code is spent before the password changes, so
//     tries sent at once neither pass the limit nor change it twice.
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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

const (
	resetCodeTTL     = 30 * time.Minute
	resetMaxAttempts = 5
	resetCodeDigits  = 6
	// resetResendAfter: a younger code is not replaced, so a second tap
	// cannot spoil the email on its way or fill the inbox.
	resetResendAfter = time.Minute
	// resetWorkDeadline bounds the lookup, the store and the email that
	// forgot runs after its answer.
	resetWorkDeadline = 15 * time.Second
)

// ResetStore keeps the Forgot password codes (postgres.PasswordResetRepo).
type ResetStore interface {
	Issue(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at, freshSince time.Time, maxTries int) (bool, error)
	Try(ctx context.Context, userID uuid.UUID, at time.Time, maxTries int) ([]byte, bool, error)
	Spend(ctx context.Context, userID uuid.UUID, codeHash []byte) (bool, error)
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
	wg        sync.WaitGroup
}

// NewPasswordResetHandler builds the handler. sender delivers the code email.
func NewPasswordResetHandler(users user.Repository, passwords user.PasswordChanger, refresh *auth.RefreshService,
	store ResetStore, sender mail.Sender, secret []byte, accessTTL time.Duration) *PasswordResetHandler {
	return &PasswordResetHandler{users: users, passwords: passwords, refresh: refresh, store: store, sender: sender,
		secret: secret, accessTTL: accessTTL, now: time.Now}
}

// Wait waits for every code email started so far (tests and shutdown).
func (h *PasswordResetHandler) Wait() { h.wg.Wait() }

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
	// The same answer, at once, whatever happens next: the lookup and the
	// email run after it, so nobody learns who has an account, and a slow
	// email never makes the app give up on a code that was sent.
	Respond(c, http.StatusOK, gin.H{})
	h.background(email)
}

// background runs sendCode for email on its own deadline, never the
// request's.
func (h *PasswordResetHandler) background(email string) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("httpapi: reset code panic: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), resetWorkDeadline)
		defer cancel()
		h.sendCode(ctx, email)
	}()
}

// sendCode emails a new code to the account of email, if it has one that
// may reset and the store keeps the new code.
func (h *PasswordResetHandler) sendCode(ctx context.Context, email string) {
	u, err := h.users.GetByEmailCI(ctx, email)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			log.Printf("httpapi: reset lookup: %v", err)
		}
		return
	}
	if u.Disabled || u.PasswordHash == nil || *u.PasswordHash == "" {
		return
	}
	code, err := newResetCode()
	if err != nil {
		log.Printf("httpapi: reset code: %v", err)
		return
	}
	now := h.now().UTC()
	stored, err := h.store.Issue(ctx, u.ID, h.codeHash(u.ID, code), now.Add(resetCodeTTL), now,
		now.Add(-resetResendAfter), resetMaxAttempts)
	if err != nil {
		log.Printf("httpapi: reset store: %v", err)
		return
	}
	if !stored {
		return
	}
	subject, html, text := resetMessage(code)
	if err := h.sender.Send(ctx, u.Email, subject, html, text); err != nil {
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
	stored, ok, err := h.store.Try(ctx, u.ID, h.now(), resetMaxAttempts)
	if err != nil {
		RespondError(c, err)
		return
	}
	codeHash := h.codeHash(u.ID, code)
	if !ok || !hmac.Equal(stored, codeHash) {
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
	// Spent before the password changes, by one request only: the right code
	// sent twice at once changes the password once, and no change can leave
	// the code usable.
	spent, err := h.store.Spend(ctx, u.ID, codeHash)
	if err != nil {
		RespondError(c, err)
		return
	}
	if !spent {
		wrong()
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
	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
		"user":          apiUser(u),
	})
}
