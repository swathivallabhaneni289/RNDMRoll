// Package httpapi (this file): the four password-path authentication
// endpoints -- signup, login, refresh, logout -- and their route
// registration. Sign-up is one request: it takes every field of the
// "Make it yours." page and answers with a signed-in session, with no code
// and no email check. This is ACCT-01's core HTTP surface.
package httpapi

import (
	"context"
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
	// accountLimit counts log in tries per address and account found, however
	// the account was named (Login says why). The admin page's log in counts
	// against it too (NewServer hands it over).
	accountLimit *middleware.Limiter
	// adminEmails are the ADMIN_EMAILS addresses (lower case): sign-up
	// answers each as taken (SetAdminEmails).
	adminEmails map[string]bool
	// clock is the time every limit of these routes counts by; nil is the
	// real clock. A test sets it before sending anything.
	clock func() time.Time
	// compare is auth.ComparePassword when nil; a test counts the
	// comparisons through it.
	compare func(hash, plain string) error
	// floor is emailTakenFloor; sleep waits for d (time.Sleep). A test sets
	// sleep to see the waits.
	floor time.Duration
	sleep func(d time.Duration)
}

// emailTakenFloor is how long after it came in sign-up answers 409
// email_taken at the soonest: the reset's floor (resetAnswerFloor). An
// ADMIN_EMAILS address is answered without the insert a taken email runs,
// so without the floor it would answer sooner and tell a stranger that the
// address is on the list rather than signed up.
const emailTakenFloor = resetAnswerFloor

// loginAccountTries is how many log in tries one address may make at one
// account a minute, by its email and its username together.
const loginAccountTries = 5

// NewAuthHandler constructs an AuthHandler. accessTTL is the lifetime of
// each minted access token; the refresh token's lifetime lives on refresh
// itself (see auth.NewRefreshService). There is no mailer: sign-up sends no
// email.
func NewAuthHandler(users user.Repository, refresh *auth.RefreshService, jwtSecret []byte, accessTTL time.Duration) *AuthHandler {
	h := &AuthHandler{
		users:     users,
		refresh:   refresh,
		jwtSecret: jwtSecret,
		accessTTL: accessTTL,
		floor:     emailTakenFloor,
		sleep:     time.Sleep,
	}
	h.accountLimit = middleware.NewLimiter(loginAccountTries, time.Minute, h.now)
	return h
}

// now is the limits' clock: clock, or the real one.
func (h *AuthHandler) now() time.Time {
	if h.clock != nil {
		return h.clock()
	}
	return time.Now()
}

// SetAdminEmails names the ADMIN_EMAILS addresses. Sign-up answers each one
// exactly as it answers an email that is taken (409 email_taken), also when
// no account has it: an address on the list can only ever belong to the
// account that had it when the server started (FindAdminAccounts), never to
// a stranger who signs up with it first. Called once, before serving.
func (h *AuthHandler) SetAdminEmails(emails []string) {
	h.adminEmails = map[string]bool{}
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			h.adminEmails[e] = true
		}
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
		Clock:    h.now,
	}), middleware.RateLimit(middleware.LimitConfig{
		Requests: 3,
		Window:   time.Minute,
		KeyFunc:  signupLimitKey,
		Clock:    h.now,
	}), h.Signup)
	// Log in is limited twice as well: 20 a minute per address in total, so one
	// address cannot try many different usernames (a username is shown to
	// friends, unlike an email), then 5 a minute per address and login. A
	// third limit, 5 a minute per address and account, runs in Login itself,
	// once it knows the account, and answers as a wrong password does.
	rg.POST("/auth/login", middleware.RateLimit(middleware.LimitConfig{
		Requests: 20,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
		Clock:    h.now,
	}), middleware.RateLimit(middleware.LimitConfig{
		Requests: 5,
		Window:   time.Minute,
		KeyFunc:  loginLimitKey,
		Clock:    h.now,
	}), h.Login)
	// Neither refresh nor logout sits behind RequireAuth: refresh exists
	// precisely because the access token has already expired, and logout
	// must work from that same state. Each is limited per address instead,
	// 60 a minute (refreshAndLogoutPerMinute): generous, since a phone
	// refreshes about every 15 minutes, but a bound on the database work one
	// address can ask for with tokens it made up or kept. The app keeps its
	// token on a 429 and tries again later.
	rg.POST("/auth/refresh", middleware.RateLimit(middleware.LimitConfig{
		Requests: refreshAndLogoutPerMinute,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
		Clock:    h.now,
	}), h.Refresh)
	rg.POST("/auth/logout", middleware.RateLimit(middleware.LimitConfig{
		Requests: refreshAndLogoutPerMinute,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyByIP,
		Clock:    h.now,
	}), h.Logout)
}

// refreshAndLogoutPerMinute is how many refreshes, and separately how many
// log outs, one address may send a minute.
const refreshAndLogoutPerMinute = 60

// The per-email and per-login limits read the body into the handler's own
// request type, so each counts the very name its handler goes on to use.
var (
	signupLimitKey = middleware.KeyByIPAndBody(func(r *SignupRequest) string { return r.Email })
	loginLimitKey  = middleware.KeyByIPAndBody(func(r *LoginRequest) string { return r.typed() })
)

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

// LoginRequest is the login request body. Login is what the person typed on the
// Log in page: an email address (it has an @) or a username. Email is the older
// name for the same field and is still accepted; Login wins when both are sent.
// The password rule is only required,max=72: a short password is simply a wrong
// password (401), not a validation error, so login never reveals the sign-up
// password rule.
type LoginRequest struct {
	Login    string `json:"login,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password" binding:"required,max=72"`
}

// typed is what the person typed on the Log in page: Login, or the older Email
// when Login is empty. Log in and its limit both read it here, so the limit
// always counts the account Log in checks.
func (r *LoginRequest) typed() string {
	if r.Login != "" {
		return r.Login
	}
	return r.Email
}

// passwordReason checks a new password: required, 8 to 72 characters, with at
// least one capital letter, one number and one symbol. It returns "" when the
// password is fine. Sign-up and change password follow this one rule; log in
// never checks it, so older, simpler passwords still work.
func passwordReason(p string) string {
	n := utf8.RuneCountInString(p)
	switch {
	case p == "":
		return reasonRequired
	case n < minPasswordLength:
		return reasonTooShort
	case n > maxPasswordLength:
		return reasonTooLong
	}
	var upper, digit, symbol bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbol = true
		}
	}
	if !upper || !digit || !symbol {
		return reasonTooWeak
	}
	return ""
}

// validEmail accepts a plain address of at most 254 characters: it must parse
// as a single bare address (no display name, no spaces) and have a dot in the
// domain. No kind of space, control character or invisible format character
// is let through anywhere in it (mail.ParseAddress lets a no-break space or a
// zero-width space through, which would make "victim@example.com" and a
// look-alike two accounts). Sign-up also asks for plain ASCII (asciiEmail).
func validEmail(s string) (reason string) {
	if s == "" {
		return reasonRequired
	}
	if len(s) > maxEmailLength {
		return reasonTooLong
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return reasonInvalid
		}
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

// asciiEmail reports whether an address is plain printable ASCII. Sign-up
// keeps only such addresses, so the lowercasing Go does for the limits and
// the lower() Postgres does for the unique index and every lookup always
// agree, and no address can pass for another with a look-alike letter. Log
// in, Forgot password and reset need no such rule: no account has any other
// address, and the only letters either lowercase turns into ASCII (the Kelvin
// sign and the capital I with a dot) come out the same in both.
func asciiEmail(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] >= 0x7f {
			return false
		}
	}
	return true
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
// verified and no mail is sent. The account starts public, or private when the
// person is under 18 that day (account_visibility in the body). An
// ADMIN_EMAILS address is answered as taken (SetAdminEmails), and every 409
// email_taken comes no sooner than emailTakenFloor after the request came in.
// signupWorkDeadline bounds the account insert, which runs to its end even if the client hangs up.
const signupWorkDeadline = 15 * time.Second

func (h *AuthHandler) Signup(c *gin.Context) {
	start := h.now()
	var req SignupRequest
	// ShouldBindBodyWith (not ShouldBindJSON) so the request body survives
	// being read here even though the rate limiter already consumed it via
	// signupLimitKey.
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	if reason := validEmail(req.Email); reason != "" {
		RespondFieldError(c, "email", reason)
		return
	}
	if !asciiEmail(req.Email) {
		RespondFieldError(c, "email", reasonInvalid)
		return
	}
	if reason := passwordReason(req.Password); reason != "" {
		RespondFieldError(c, "password", reason)
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
	age := user.AgeOn(birth, now)
	if age < user.MinimumAge {
		RespondUnderMinimumAge(c)
		return
	}

	if err := user.ValidateUsername(req.Username); err != nil {
		RespondFieldError(c, "username", reasonInvalid)
		return
	}

	hash, ok := hashSignupPassword(c, "password", req.Password)
	if !ok {
		return
	}

	// An ADMIN_EMAILS address is taken, whether or not its account exists:
	// the same answer, no sooner than an email in use (emailTakenFloor).
	if h.adminEmails[strings.ToLower(req.Email)] {
		holdUntil(h.now, h.sleep, start, h.floor)
		RespondError(c, user.ErrEmailTaken)
		return
	}

	// A new account starts public, except that one under 18 on the day it is
	// made starts private (user.DefaultVisibility is the whole rule). The
	// person can switch it either way in Settings.
	// Not the request's context: a client that half-closes its connection cancels it, which
	// would turn a taken email into a 500 while a reserved admin email still answers 409.
	createCtx, cancelCreate := context.WithTimeout(context.WithoutCancel(c.Request.Context()), signupWorkDeadline)
	defer cancelCreate()
	u, err := h.users.CreateComplete(createCtx, user.NewAccount{
		Email:             req.Email,
		PasswordHash:      hash,
		Birthday:          birth.Format("2006-01-02"),
		Name:              name,
		Username:          req.Username,
		Bio:               bio,
		AccountVisibility: user.DefaultVisibility(age),
	})
	if err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			respondUsernameTaken(c, h.users, req.Username)
			return
		}
		if errors.Is(err, user.ErrEmailTaken) {
			holdUntil(h.now, h.sleep, start, h.floor)
		}
		RespondError(c, err)
		return
	}

	h.issueSession(c, u, http.StatusOK)
}

// hashSignupPassword hashes the sign-up password. On failure it has
// already answered and returns false. The checks before it count characters;
// bcrypt's ceiling counts bytes, so a multi-byte password can pass them and
// still be refused here, naming the field it came from.
func hashSignupPassword(c *gin.Context, field, plain string) (string, bool) {
	hash, err := auth.HashPassword(plain)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooLong) {
			RespondFieldError(c, field, reasonTooLong)
			return "", false
		}
		log.Printf("httpapi: signup hash password failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return "", false
	}
	return hash, true
}

// Login authenticates by email or username, plus password. A login with an @
// is an email address; anything else is a username (an email can never be a
// username, so the two cannot be confused). Either way it is the account's one
// password that is checked. Unknown-account and wrong-password
// attempts are indistinguishable: both run a bcrypt comparison (against a fixed
// dummy hash for the unknown-account case) and both return the identical
// user.ErrInvalidCredentials body, per RESEARCH.md's Security Domain
// enumeration-resistance requirement. That includes a username nobody holds and
// text that could never be a username: neither is a format error.
//
// The per-login limit counts the name as typed, so an account's email and its
// username are two of its buckets. Once the account is found, one more limit
// counts the tries at the account itself from this address, whichever name
// they used: 5 a minute (the admin page's log in shares it). Past it, the
// answer is a wrong password's, after the same comparison, the right password
// included: never a 429, which would tell that the name just typed belongs
// to the account the earlier tries named (five tries by a username, then one
// by an email, would meet a 429 only when the email is that account's). A
// name that finds no account is counted by the per-login limit only, so no
// number of unknown names makes a 429 for another name either.
//
// A lookup that fails for any reason but "no such account" is a server error
// (500), not a wrong password: only an account that is not there takes the
// dummy comparison.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	login := req.typed()
	// An empty or email-shaped login is checked the way sign-up checks an
	// address, so a typo in an address gets a field error the screen can name.
	byEmail := login == "" || strings.Contains(login, "@")
	if byEmail {
		if reason := validEmail(login); reason != "" {
			RespondFieldError(c, "login", reason)
			return
		}
	}

	ctx := c.Request.Context()
	var (
		u   *user.User
		err error
	)
	switch {
	case byEmail:
		u, err = h.users.GetByEmailCI(ctx, login)
	case user.ValidateUsername(strings.ToLower(login)) == nil:
		u, err = h.users.GetByUsernameCI(ctx, login)
	default:
		err = user.ErrNotFound
	}
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		RespondError(c, err)
		return
	}
	// No such account: no account to count, and the dummy comparison.
	allowed := true
	if err == nil {
		allowed = h.accountLimit.Allow(middleware.KeyByIP(c) + "|" + u.ID.String())
	} else {
		u = nil
	}
	compare := h.compare
	if compare == nil {
		compare = auth.ComparePassword
	}
	if !passwordMatches(compare, u, req.Password, allowed) {
		RespondError(c, user.ErrInvalidCredentials)
		return
	}
	// Said only after the right password, so it never tells a stranger
	// that an account exists.
	if u.Disabled {
		Respond(c, http.StatusForbidden, gin.H{"error": string(CodeAccountDisabled)})
		return
	}

	// No verified check: sign-up sends no email, so an unverified account
	// logs in like any other.
	h.issueSession(c, u, http.StatusOK)
}

// passwordMatches compares password with u's stored hash through compare
// (auth.ComparePassword), the work a wrong password costs, and reports
// whether it is right and allowed is true. With no account (u nil) or one
// without a password it compares against the dummy hash instead, and is false
// whatever was typed: the dummy is never used as if it were the account's
// own. Past the account's limit (allowed false) it still compares, then is
// false, so the answer and its timing are a wrong password's.
func passwordMatches(compare func(hash, plain string) error, u *user.User, password string, allowed bool) bool {
	if u == nil || u.PasswordHash == nil || *u.PasswordHash == "" {
		_ = compare(dummyPasswordHash, password)
		return false
	}
	return compare(*u.PasswordHash, password) == nil && allowed
}

// issueSession mints an access+refresh token pair for u and writes the
// AuthResult response body (lib/api/types.ts's AuthResult shape). The refresh
// token comes first: the access token names it as its session.
func (h *AuthHandler) issueSession(c *gin.Context, u *user.User, status int) {
	ctx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	refreshToken, session, err := h.refresh.Issue(ctx, u.ID, &userAgent)
	if err != nil {
		log.Printf("httpapi: issue refresh token failed: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}
	accessToken, err := auth.IssueSessionAccessToken(u.ID, session, h.jwtSecret, h.accessTTL)
	if err != nil {
		log.Printf("httpapi: issue access token failed: %v", err)
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
		"account_visibility":  u.AccountVisibility,
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
// mobile client branches on exactly those two codes. A token that was
// already rotated ends its whole session (RefreshService.Redeem), so a lost
// answer or a stolen copy never leaves a live session behind.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	ctx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	userID, newRefreshToken, session, err := h.refresh.Redeem(ctx, req.RefreshToken, &userAgent)
	if err != nil {
		RespondError(c, err)
		return
	}
	if u, err := h.users.GetByID(ctx, userID); err == nil && u.Disabled {
		Respond(c, http.StatusForbidden, gin.H{"error": string(CodeAccountDisabled)})
		return
	}

	accessToken, err := auth.IssueSessionAccessToken(userID, session, h.jwtSecret, h.accessTTL)
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

// Logout ends the session of the presented refresh token and responds 204:
// the token itself, or, when it was rotated since the phone stored it (a
// refresh answer that never arrived), the token the session holds now. A
// token that is already missing, malformed, or revoked still returns 204: the
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
