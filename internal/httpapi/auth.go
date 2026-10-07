// Package httpapi (this file): the four password-path authentication
// endpoints -- signup, login, refresh, logout -- and their route
// registration. Sign-up is one request: it takes every field of the
// "Make it yours." page and answers with a signed-in session, with no code
// and no email check. This is ACCT-01's core HTTP surface.
package httpapi

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
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
	users     user.Repository
	refresh   *auth.RefreshService
	jwtSecret []byte
	accessTTL time.Duration
}

// NewAuthHandler constructs an AuthHandler. accessTTL is the lifetime of
// each minted access token; the refresh token's lifetime lives on refresh
// itself (see auth.NewRefreshService). There is no mailer: sign-up sends no
// email.
func NewAuthHandler(users user.Repository, refresh *auth.RefreshService, jwtSecret []byte, accessTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		users:     users,
		refresh:   refresh,
		jwtSecret: jwtSecret,
		accessTTL: accessTTL,
	}
}

// Register mounts the auth routes on rg. This is the only place route
// paths for this handler appear, so no central router file is needed.
func (h *AuthHandler) Register(rg *gin.RouterGroup) {
	// Sign-up has no session, so it is limited twice: 10 a minute per
	// address in total, then 3 a minute per address and email. The key is the
	// socket address (NewServer trusts no forwarded header).
	rg.POST("/auth/signup", middleware.RateLimit(middleware.LimitConfig{
		Requests: 10,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
	}), middleware.RateLimit(middleware.LimitConfig{
		Requests: 3,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIPAndField("email"),
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

// Field limits shared by sign-up and PATCH /me.
const (
	maxEmailLength    = 254
	minPasswordLength = 8
	maxPasswordLength = 72
	maxNameLength     = 50
	maxBioLength      = 160
)

// SignupRequest is the signup request body. It carries no binding tags on
// purpose: Signup validates each field by hand, in a fixed order, so every
// rejection names its field (RespondFieldError). The password ceiling of 72
// is bcrypt's hard byte limit (RESEARCH.md Pitfall 1).
type SignupRequest struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Birthday string  `json:"birthday"`
	Name     string  `json:"name"`
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
}

// LoginRequest is the login request body. The password rule is only
// required,max=72: a short password is simply a wrong password (401), not a
// validation error, so login never reveals the sign-up password rule.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,max=72"`
}

// validEmail accepts a plain address of at most 254 characters: it must parse
// as a single bare address (no display name, no spaces) and have a dot in the
// domain.
func validEmail(s string) (reason string) {
	if s == "" {
		return reasonRequired
	}
	if len(s) > maxEmailLength {
		return reasonTooLong
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return reasonInvalid
	}
	at := strings.LastIndex(s, "@")
	if at < 1 || !strings.Contains(s[at+1:], ".") || strings.HasSuffix(s, ".") {
		return reasonInvalid
	}
	return ""
}

// cleanName trims name and checks it is 1 to 50 characters. It returns the
// trimmed value and a failure reason ("" when fine).
func cleanName(name string) (string, string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", reasonRequired
	}
	if utf8.RuneCountInString(trimmed) > maxNameLength {
		return "", reasonTooLong
	}
	if hasControlChar(trimmed, false) {
		return "", reasonInvalid
	}
	return trimmed, ""
}

// hasControlChar reports whether s holds a control character (U+0000 and
// the rest of the Unicode control class). allowLineBreaks lets a tab, line
// feed and carriage return through, for free text like a bio. A NUL in
// particular is never storable in Postgres text, so it is refused here with
// a field error rather than surfacing as a 500.
func hasControlChar(s string, allowLineBreaks bool) bool {
	for _, r := range s {
		if !unicode.IsControl(r) {
			continue
		}
		if allowLineBreaks && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		return true
	}
	return false
}

// Signup creates a finished account in one request and signs it in. Order:
// each field is validated (400 validation_failed plus the field name); then
// the age (under 13 is 403 under_minimum_age and nothing is stored, nothing
// about the person is logged); then the username; then the password is
// hashed; then ONE insert writes every column, so any failure stores
// nothing. A duplicate email is 409 email_taken, a duplicate username 409
// username_taken with suggestions (including a race, which the unique index
// decides). Success answers with the same body as Login. The email is NOT
// verified and no mail is sent.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	// ShouldBindBodyWith (not ShouldBindJSON) so the request body survives
	// being read here even though the rate limiter already consumed it via
	// KeyByIPAndField.
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	if reason := validEmail(req.Email); reason != "" {
		RespondFieldError(c, "email", reason)
		return
	}
	passwordRunes := utf8.RuneCountInString(req.Password)
	switch {
	case req.Password == "":
		RespondFieldError(c, "password", reasonRequired)
		return
	case passwordRunes < minPasswordLength:
		RespondFieldError(c, "password", reasonTooShort)
		return
	case passwordRunes > maxPasswordLength:
		RespondFieldError(c, "password", reasonTooLong)
		return
	}

	now := time.Now()
	if req.Birthday == "" {
		RespondFieldError(c, "birthday", reasonRequired)
		return
	}
	birth, err := user.ValidateBirthdate(req.Birthday, now)
	if err != nil {
		if errors.Is(err, user.ErrBirthdateFuture) {
			RespondFieldError(c, "birthday", reasonFuture)
			return
		}
		RespondFieldError(c, "birthday", reasonInvalid)
		return
	}

	name, reason := cleanName(req.Name)
	if reason != "" {
		RespondFieldError(c, "name", reason)
		return
	}
	var bio *string
	if req.Bio != nil && strings.TrimSpace(*req.Bio) != "" {
		if utf8.RuneCountInString(*req.Bio) > maxBioLength {
			RespondFieldError(c, "bio", reasonTooLong)
			return
		}
		if hasControlChar(*req.Bio, true) {
			RespondFieldError(c, "bio", reasonInvalid)
			return
		}
		bio = req.Bio
	}

	// The age check comes after every field is well formed and before
	// anything is stored or hashed. Nothing about a refusal is logged.
	if user.AgeOn(birth, now) < user.MinimumAge {
		RespondUnderMinimumAge(c)
		return
	}

	if err := user.ValidateUsername(req.Username); err != nil {
		RespondFieldError(c, "username", reasonInvalid)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooLong) {
			// The checks above count characters; bcrypt's ceiling counts
			// bytes, so a multi-byte password can pass them and still land
			// here.
			RespondFieldError(c, "password", reasonTooLong)
			return
		}
		log.Printf("httpapi: signup hash password failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}

	u, err := h.users.CreateComplete(c.Request.Context(), user.NewAccount{
		Email:        req.Email,
		PasswordHash: hash,
		Birthday:     birth.Format("2006-01-02"),
		Name:         name,
		Username:     req.Username,
		Bio:          bio,
	})
	if err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			respondUsernameTaken(c, h.users, req.Username)
			return
		}
		RespondError(c, err)
		return
	}

	h.issueSession(c, u, http.StatusOK)
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

	// No verified check: sign-up sends no email, so an unverified account
	// logs in like any other.
	h.issueSession(c, u, http.StatusOK)
}

// issueSession mints an access+refresh token pair for u and writes the
// AuthResult response body (lib/api/types.ts's AuthResult shape).
func (h *AuthHandler) issueSession(c *gin.Context, u *user.User, status int) {
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

	Respond(c, status, gin.H{
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
