package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// VerifyEmailHandler implements D-04's email-verification flow: consuming a
// verification token, resending one, and the browser callback that hands
// control back to the app. It touches no route another wave 4 plan owns.
type VerifyEmailHandler struct {
	users          user.Repository
	verifications  user.EmailVerificationRepository
	mailer         mail.Mailer
	refresh        *auth.RefreshService
	jwtSecret      []byte
	accessTokenTTL time.Duration
	deepLinkScheme string
}

// NewVerifyEmailHandler constructs a VerifyEmailHandler.
func NewVerifyEmailHandler(
	users user.Repository,
	verifications user.EmailVerificationRepository,
	mailer mail.Mailer,
	refresh *auth.RefreshService,
	jwtSecret []byte,
	accessTokenTTL time.Duration,
	deepLinkScheme string,
) *VerifyEmailHandler {
	return &VerifyEmailHandler{
		users:          users,
		verifications:  verifications,
		mailer:         mailer,
		refresh:        refresh,
		jwtSecret:      jwtSecret,
		accessTokenTTL: accessTokenTTL,
		deepLinkScheme: deepLinkScheme,
	}
}

// Register mounts the verify-email routes on rg (the "/v1/auth" group). The
// resend route carries a 30 second rate limit keyed by IP and address,
// matching UI-SPEC's client-side resend cooldown exactly so the visible
// countdown and the server limit never disagree.
func (h *VerifyEmailHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/verify-email", h.Verify)
	rg.POST("/verify-email/resend", middleware.RateLimit(middleware.LimitConfig{
		Requests: 1,
		Window:   30 * time.Second,
		KeyFunc:  middleware.KeyByIPAndField("email"),
	}), h.Resend)
	rg.GET("/verify-email/callback", h.Callback)
}

type verifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// hashVerificationToken decodes a client-presented token and hashes the raw
// bytes the same way internal/mail.Service hashed them at issuance, so a
// lookup by digest finds the same row. A token that fails to decode cannot
// possibly match a stored digest, so it is treated as invalid rather than
// propagated as a decode error, mirroring internal/auth's refresh token
// redemption.
func hashVerificationToken(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, user.ErrTokenInvalid
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

// Verify consumes a verification token exactly once. On success it marks
// the account verified via the password-flow source and mints the user's
// first session: D-05 puts verification at onboarding step 2, before the
// name, username, and photo steps, so those steps run as authenticated
// requests.
func (h *VerifyEmailHandler) Verify(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	hash, err := hashVerificationToken(req.Token)
	if err != nil {
		RespondError(c, err)
		return
	}

	ctx := c.Request.Context()
	userID, err := h.verifications.ConsumeByHash(ctx, hash, time.Now())
	if err != nil {
		RespondError(c, err)
		return
	}

	if err := h.users.MarkEmailVerified(ctx, userID, user.VerifiedViaPasswordFlow); err != nil {
		RespondError(c, err)
		return
	}

	u, err := h.users.GetByID(ctx, userID)
	if err != nil {
		RespondError(c, err)
		return
	}

	accessToken, err := auth.IssueAccessToken(u.ID, h.jwtSecret, h.accessTokenTTL)
	if err != nil {
		RespondError(c, err)
		return
	}

	var userAgent *string
	if ua := c.GetHeader("User-Agent"); ua != "" {
		userAgent = &ua
	}
	refreshToken, err := h.refresh.Issue(ctx, u.ID, userAgent)
	if err != nil {
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTokenTTL.Seconds()),
		"user": gin.H{
			"id":                  u.ID.String(),
			"email":               u.Email,
			"name":                u.Name,
			"username":            u.Username,
			"bio":                 u.Bio,
			"avatar_url":          u.AvatarURL,
			"email_verified":      u.EmailVerified,
			"onboarding_complete": u.OnboardingComplete(),
		},
	})
}

type verifyEmailResendRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// Resend issues a fresh verification email when the address is registered
// and not yet verified. It responds 202 with an identical body in every
// case, including an address with no account or one already verified, so
// the endpoint cannot be used to enumerate registered addresses
// (RESEARCH.md's Security Domain lists this alongside login).
func (h *VerifyEmailHandler) Resend(c *gin.Context) {
	var req verifyEmailResendRequest
	// The rate-limit middleware ahead of this route already reads the
	// request body via ShouldBindBodyWith to compute its key, which caches
	// the raw bytes under gin's body-cache key. Binding through the same
	// ShouldBindBodyWith family here reads that cache instead of an
	// already-drained net/http request body.
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	ctx := c.Request.Context()
	u, err := h.users.GetByEmailCI(ctx, req.Email)
	if err == nil && !u.EmailVerified {
		if sendErr := h.mailer.SendVerificationEmail(ctx, u); sendErr != nil {
			log.Printf("httpapi: resend verification email failed: %v", sendErr)
		}
	}

	Respond(c, http.StatusAccepted, gin.H{"status": "ok"})
}

// Callback handles the browser-opened verification link. Mail clients open
// links in a browser, not the app, so this route returns a minimal HTML
// page that redirects to the app's deep link scheme and also renders a
// plain tappable link as a fallback for browsers that block automatic
// scheme navigation. It performs no verification itself: the token is
// consumed only when the app posts it to Verify, which keeps single-use
// consumption tied to the device that started the signup.
func (h *VerifyEmailHandler) Callback(c *gin.Context) {
	token := c.Query("token")
	escaped := html.EscapeString(token)
	deepLink := fmt.Sprintf("%s://verify-email?token=%s", h.deepLinkScheme, escaped)

	page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Verifying your email</title>
<meta http-equiv="refresh" content="0;url=%s">
</head>
<body>
<p>Redirecting you back to the app.</p>
<p><a href="%s">Tap here if you are not redirected automatically.</a></p>
</body>
</html>`, deepLink, deepLink)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}
