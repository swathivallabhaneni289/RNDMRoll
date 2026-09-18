// Package httpapi (this file): the four password-path authentication
// endpoints -- signup, login, refresh, logout -- and their route
// registration. This is ACCT-01's core HTTP surface.
package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// Mailer is the narrow consumer contract AuthHandler depends on to send a
// verification email. Declared here at the point of use, rather than
// importing internal/mail (which plan 01-09 builds in a sibling worktree in
// this same wave), is what lets these two plans execute concurrently.
//
// Deviation from PLAN.md (Rule 3 - blocking): the plan's own text specifies
// this interface as SendVerificationEmail(ctx, *user.User) error. That
// shape is incompatible with the mailer interface plan 01-06 already
// committed in internal/httpapi/testsupport_test.go
// (SendVerificationEmail(ctx, toEmail, token string) error), whose
// TestDeps.Mailer field and fakeMailer are typed against it. The plan's own
// task instruction requires writing this plan's tests against that shared
// harness without modifying testsupport_test.go (owned by 01-06, and
// off-limits here since 01-09 is editing it concurrently in this wave per
// the orchestrator's file-overlap analysis). Matching the already-shipped
// signature is the only way to satisfy both constraints; 01-06's own
// SUMMARY.md already anticipated this reconciliation ("Plan 01-09 should
// reconcile its concrete Mailer interface with testsupport_test.go's local
// placeholder"). Because the signature takes toEmail/token rather than a
// *user.User, AuthHandler now needs its own EmailVerificationRepository
// dependency (not listed in the plan's struct field list) to generate and
// persist the token before calling the mailer.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, toEmail, token string) error
}

const (
	// verificationTokenTTL is how long a freshly issued email verification
	// token stays valid, per RESEARCH.md Common Pitfalls #5.
	verificationTokenTTL = 24 * time.Hour
	// verificationTokenBytes is the crypto/rand entropy per verification
	// token, matching internal/auth/refresh.go's refresh-token treatment.
	verificationTokenBytes = 32
)

// dummyPasswordHash is a real bcrypt hash compared against on a
// missing-account login attempt, so that path performs the same bcrypt
// work a real wrong-password attempt would -- RESEARCH.md's Security
// Domain requires unknown-account and wrong-password to be indistinguishable,
// including by timing, not just by response body.
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := auth.HashPassword("dummy-password-for-constant-time-comparison")
	if err != nil {
		panic("httpapi: failed to precompute dummy password hash: " + err.Error())
	}
	return hash
}

// AuthHandler implements the signup, login, refresh, and logout endpoints.
type AuthHandler struct {
	users         user.Repository
	refresh       *auth.RefreshService
	verifications user.EmailVerificationRepository
	mailer        Mailer
	jwtSecret     []byte
	accessTTL     time.Duration
}

// NewAuthHandler constructs an AuthHandler. accessTTL is the lifetime of
// each minted access token; the refresh token's lifetime lives on refresh
// itself (see auth.NewRefreshService).
func NewAuthHandler(users user.Repository, refresh *auth.RefreshService, verifications user.EmailVerificationRepository, mailer Mailer, jwtSecret []byte, accessTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		users:         users,
		refresh:       refresh,
		verifications: verifications,
		mailer:        mailer,
		jwtSecret:     jwtSecret,
		accessTTL:     accessTTL,
	}
}

// Register mounts the auth routes on rg. This is the only place route
// paths for this handler appear, so no central router file is needed.
func (h *AuthHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/auth/signup", middleware.RateLimit(middleware.LimitConfig{
		Requests: 3,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
	}), h.Signup)
	rg.POST("/auth/login", middleware.RateLimit(middleware.LimitConfig{
		Requests: 5,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIPAndField("email"),
	}), h.Login)
	// Neither refresh nor logout sits behind RequireAuth: refresh exists
	// precisely because the access token has already expired, and logout
	// must work from that same state.
	rg.POST("/auth/refresh", h.Refresh)
	rg.POST("/auth/logout", h.Logout)
}

// SignupRequest is the signup request body. max=72 is bcrypt's hard byte
// ceiling (RESEARCH.md Pitfall 1), enforced here so HashPassword never
// sees an over-long input.
type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// LoginRequest is the login request body.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// Signup creates a new unverified account and requests a verification
// email. A mailer failure is logged, not surfaced as an error response --
// the account exists and the user can resend, so failing the whole signup
// over a transient mail-provider outage would be worse.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	// ShouldBindBodyWith (not ShouldBindJSON) so the request body survives
	// being read here even on routes whose rate limiter already consumed it
	// via KeyByIPAndField -- signup doesn't need that today, but Login does,
	// and using the same read path in both keeps that constraint from
	// silently breaking if a future edit swaps signup's key function.
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooLong) {
			// binding's max=72 counts runes, bcrypt's ceiling counts bytes;
			// a multi-byte-rune password can pass validation and still hit
			// this. Map it to the same validation_failed code rather than
			// falling through to a 500 on ordinary user input.
			RespondValidationError(c, err)
			return
		}
		log.Printf("httpapi: signup hash password failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}

	ctx := c.Request.Context()
	u, err := h.users.Create(ctx, req.Email, &hash, false, nil)
	if err != nil {
		RespondError(c, err)
		return
	}

	if err := h.sendVerificationEmail(ctx, u); err != nil {
		log.Printf("httpapi: signup verification email failed for user %s: %v", u.ID, err)
	}

	Respond(c, http.StatusCreated, gin.H{"user_id": u.ID})
}

// sendVerificationEmail generates a single-use, hashed, expiring
// verification token, persists it, and hands the raw token to the mailer.
func (h *AuthHandler) sendVerificationEmail(ctx context.Context, u *user.User) error {
	raw, hash, err := generateVerificationToken()
	if err != nil {
		return err
	}
	if err := h.verifications.Insert(ctx, u.ID, hash, time.Now().Add(verificationTokenTTL)); err != nil {
		return err
	}
	return h.mailer.SendVerificationEmail(ctx, u.Email, raw)
}

func generateVerificationToken() (raw string, hash []byte, err error) {
	buf := make([]byte, verificationTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)
	return raw, sum[:], nil
}

// Login authenticates by email and password. Unknown-account and
// wrong-password attempts are indistinguishable: both run a bcrypt
// comparison (against a fixed dummy hash for the unknown-account case) and
// both return the identical user.ErrInvalidCredentials body, per
// RESEARCH.md's Security Domain enumeration-resistance requirement.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	ctx := c.Request.Context()
	u, err := h.users.GetByEmailCI(ctx, req.Email)
	if err != nil {
		_ = auth.ComparePassword(dummyPasswordHash, req.Password)
		RespondError(c, user.ErrInvalidCredentials)
		return
	}

	storedHash := ""
	if u.PasswordHash != nil {
		storedHash = *u.PasswordHash
	}
	if err := auth.ComparePassword(storedHash, req.Password); err != nil {
		RespondError(c, user.ErrInvalidCredentials)
		return
	}

	if !u.EmailVerified {
		RespondError(c, user.ErrEmailNotVerified)
		return
	}

	h.issueSession(c, u)
}

// issueSession mints an access+refresh token pair for u and writes the
// AuthResult response body (lib/api/types.ts's AuthResult shape).
func (h *AuthHandler) issueSession(c *gin.Context, u *user.User) {
	ctx := c.Request.Context()
	accessToken, err := auth.IssueAccessToken(u.ID, h.jwtSecret, h.accessTTL)
	if err != nil {
		log.Printf("httpapi: issue access token failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}
	userAgent := c.Request.UserAgent()
	refreshToken, err := h.refresh.Issue(ctx, u.ID, &userAgent)
	if err != nil {
		log.Printf("httpapi: issue refresh token failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
		"user":          apiUser(u),
	})
}

// apiUser maps the domain user to lib/api/types.ts's ApiUser shape.
func apiUser(u *user.User) gin.H {
	return gin.H{
		"id":                  u.ID,
		"email":               u.Email,
		"name":                u.Name,
		"username":            u.Username,
		"bio":                 u.Bio,
		"avatar_url":          u.AvatarURL,
		"email_verified":      u.EmailVerified,
		"onboarding_complete": u.OnboardingComplete(),
	}
}

// RefreshRequest is the refresh request body.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest is the logout request body.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh exchanges a presented refresh token for a new access+refresh
// pair, rotating the refresh token in the same call. The response always
// carries the rotated refresh token -- the client persists both values on
// every refresh, and omitting the new one would leave the device holding a
// token the server has already revoked, surfacing as a spurious forced
// logout on the client's next launch. user.ErrTokenExpired and
// user.ErrTokenInvalid propagate unchanged through RespondError, since the
// mobile client branches on exactly those two codes.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	ctx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	userID, newRefreshToken, err := h.refresh.Redeem(ctx, req.RefreshToken, &userAgent)
	if err != nil {
		RespondError(c, err)
		return
	}

	accessToken, err := auth.IssueAccessToken(userID, h.jwtSecret, h.accessTTL)
	if err != nil {
		log.Printf("httpapi: refresh issue access token failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
	})
}

// Logout revokes the presented refresh token and responds 204. A token
// that is already missing, malformed, or revoked still returns 204: the
// client calls logout on a best-effort basis and must not be trapped in a
// signed-in shell by a failure here (RefreshService.Revoke already
// tolerates this miss).
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	if err := h.refresh.Revoke(c.Request.Context(), req.RefreshToken); err != nil {
		RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
