// Package httpapi: password_reset.go serves Forgot password (developer,
// 2026-10-21): POST /auth/password/forgot {email} emails a 6-digit code,
// POST /auth/password/verify {email, code} checks the code before the app
// asks for a new password (developer, 2026-11-13), and POST
// /auth/password/reset {email, code, new_password} checks it again, sets the
// new password and signs the person in. No link is needed, so it works before
// the real web address exists.
//
//   - forgot always answers 200 {} at once, whether or not the email has an
//     account: the lookup and the email run after the answer, so neither the
//     answer nor its timing tells a stranger who is signed up.
//   - verify checks the code exactly as reset does (checkCode serves both),
//     with the same answers after the same work, and answers 200 {} for the
//     right code. It spends nothing and changes nothing: the right code costs
//     no try there and still sets the password at reset until it expires, is
//     replaced or is used, and only reset's change cancels a deletion. A code
//     reset has spent is gone, so verify never says "used": it says
//     "wrong_code".
//   - Only an HMAC of the code is stored. A code lives 30 minutes and allows
//     5 tries, which a new code never resets: a new request replaces the code
//     but keeps its tries, a used-up code gets no new one until it expires,
//     and a code under a minute old is not replaced (nothing is emailed).
//   - A wrong, expired or used-up code, an email with no account and a
//     disabled account all get the same answer, at verify as at reset: 400
//     field "code", reason "wrong_code", after the same work (an email with no
//     account runs the try statement too, for no one). The other answers
//     (verify's 200 among them) need the right code (or the one before it),
//     so they tell a stranger nothing, and none costs a try: reason
//     "replaced" for the code the newest one replaced, also when the
//     replacing happened while the password was being set, and reason "used"
//     for the right code that another request used first.
//   - Every answer of verify and reset comes no sooner than resetAnswerFloor
//     after the request came in, so whether it wrote anything (a counted try)
//     or not (an email with no account) cannot be told by its timing.
//   - Only a wrong code costs a try, at verify exactly as at reset (one count
//     for both), counted in the same statement that checks the code, so wrong
//     guesses sent at once still stop at 5. The right code never costs one,
//     at either: reset spends it in the same transaction that changes the
//     password, so the right code sent twice at once changes it once, and a
//     change that fails or is given up part way leaves the code as it was,
//     even on the last try.
//   - The three routes share their limits (Register): 10 a minute per address
//     and 5 a minute per address and email, one count for all three, so
//     checking a code at verify buys no requests that forgot and reset would
//     not allow.
//   - The new password follows the sign-up rules, and bcrypt's 72 bytes,
//     checked before the code is; setting it ends every other session (as
//     Change password does) and the answer is a fresh session.
//   - A disabled account gets no code and cannot reset, also when an admin
//     disables it while the reset runs (the change refuses it in its own
//     transaction, and a disable deletes the code).
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
	// resetAnswerFloor is how long after it came in a verify or a reset is
	// answered at the soonest, whatever the answer. It is longer than the
	// slowest normal reset (bcrypt, the try statement and the change
	// together), and verify does less, so a try that wrote (a wrong code for
	// an account) and one that did not (an email with no account) take the
	// same time.
	resetAnswerFloor = 300 * time.Millisecond
)

// ResetStore keeps the Forgot password codes (postgres.PasswordResetRepo).
type ResetStore interface {
	Issue(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt, at, freshSince time.Time, maxTries int) (bool, error)
	// Try checks codeHash against the person's live code in one statement
	// and counts a try only when it is wrong. right: it is the code. replaced:
	// it is the code the live one replaced (no try counted). Both false: wrong,
	// or no live code with tries left (a uuid.Nil userID never has one).
	Try(ctx context.Context, userID uuid.UUID, codeHash []byte, at time.Time, maxTries int) (right, replaced bool, err error)
	// SpendAndChangePassword spends the code if it is still codeHash and, in
	// the same transaction, changes the password as
	// user.PasswordChanger.ChangePassword does: both happen or neither does.
	// With nothing changed, it answers user.ErrResetCodeReplaced (a newer
	// code replaced this one meanwhile), user.ErrResetCodeUsed (another
	// request spent it first), user.ErrAccountDisabled or user.ErrNotFound.
	// restored is true when the change also cancelled a deletion the owner
	// had asked for (inside its grace period).
	SpendAndChangePassword(ctx context.Context, userID uuid.UUID, codeHash []byte, passwordHash string,
		sessionID uuid.UUID, newTokenHash []byte, expiresAt time.Time, userAgent *string) (restored bool, err error)
}

// PasswordResetHandler serves the three Forgot password routes.
type PasswordResetHandler struct {
	users     user.Repository
	refresh   *auth.RefreshService
	store     ResetStore
	sender    mail.Sender
	secret    []byte
	accessTTL time.Duration
	// now is the handler's clock: the codes' times, the limits and the
	// answer floor all read it (a test moves it).
	now func() time.Time
	// floor is resetAnswerFloor; sleep waits for d (time.Sleep). A test sets
	// both (most to no floor at all).
	floor time.Duration
	sleep func(d time.Duration)
	wg    sync.WaitGroup
	// photoLink signs the profile photo's link in the session's user
	// (Deps.PhotoLink; NewServer sets it). Nil sends no photo.
	photoLink func(string) string
}

// NewPasswordResetHandler builds the handler. sender delivers the code email;
// store keeps the codes and changes the password with the code it spends.
func NewPasswordResetHandler(users user.Repository, refresh *auth.RefreshService, store ResetStore,
	sender mail.Sender, secret []byte, accessTTL time.Duration) *PasswordResetHandler {
	return &PasswordResetHandler{users: users, refresh: refresh, store: store, sender: sender,
		secret: secret, accessTTL: accessTTL, now: time.Now, floor: resetAnswerFloor, sleep: time.Sleep}
}

// holdUntil waits, through sleep, until floor has passed on now's clock since
// start. It waits on a timer alone, never on the request's context: a client
// that half-closes its connection ends that context but still reads the
// answer, so a wait that ended with it would hand that client the very timing
// the floor hides.
func holdUntil(now func() time.Time, sleep func(time.Duration), start time.Time, floor time.Duration) {
	if wait := floor - now().Sub(start); wait > 0 {
		sleep(wait)
	}
}

// clock is now as a limit's clock, read when each request is counted, so a
// test can set now after Register.
func (h *PasswordResetHandler) clock() time.Time { return h.now() }

// Wait waits for every code email started so far (tests and shutdown).
func (h *PasswordResetHandler) Wait() { h.wg.Wait() }

// Register mounts the routes on the public /v1 group. Like sign-up they are
// limited twice, and the three routes share both limits (the same two
// limiters, so one count): 10 a minute per address in total (forgot starts
// work after its answer, so one address must not start it for email after
// email), then 5 a minute per address and email. Every answer of verify and
// reset, the limits' included, waits for the answer floor (answerFloor).
func (h *PasswordResetHandler) Register(rg *gin.RouterGroup) {
	perAddress := middleware.RateLimit(middleware.LimitConfig{
		Requests: 10, Window: time.Minute, KeyFunc: middleware.KeyByIP, Clock: h.clock,
	})
	limit := middleware.RateLimit(middleware.LimitConfig{
		Requests: 5, Window: time.Minute, KeyFunc: resetLimitKey, Clock: h.clock,
	})
	rg.POST("/auth/password/forgot", perAddress, limit, h.forgot)
	rg.POST("/auth/password/verify", h.answerFloor, perAddress, limit, h.verify)
	rg.POST("/auth/password/reset", h.answerFloor, perAddress, limit, h.reset)
}

// answerFloor holds the answer of the route it leads until floor after the
// request came in (resetAnswerFloor says why). It holds the writer, not the
// handler, so whatever answers (a limit, the handler, a 500) waits before its
// first byte, and no answer can be missed. The wait never ends early, also
// when the request's context ends (holdUntil says why).
func (h *PasswordResetHandler) answerFloor(c *gin.Context) {
	start := h.now()
	w := &heldWriter{ResponseWriter: c.Writer, release: func() {
		holdUntil(h.now, h.sleep, start, h.floor)
	}}
	c.Writer = w
	c.Next()
	// An answer with no body is written only after the chain returns.
	w.hold()
}

// heldWriter waits once, through release, before the first byte of the
// answer goes out.
type heldWriter struct {
	gin.ResponseWriter
	once    sync.Once
	release func()
}

func (w *heldWriter) hold() { w.once.Do(w.release) }

func (w *heldWriter) WriteHeaderNow() {
	w.hold()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *heldWriter) Write(b []byte) (int, error) {
	w.hold()
	return w.ResponseWriter.Write(b)
}

func (w *heldWriter) WriteString(s string) (int, error) {
	w.hold()
	return w.ResponseWriter.WriteString(s)
}

func (w *heldWriter) Flush() {
	w.hold()
	w.ResponseWriter.Flush()
}

// resetLimitKey keys the per-email limit the three routes share by the email
// the handlers use. A verify or reset body names the account in the same email
// field as a forgot body, and no other field of either matches that key, so
// reading any of the three bodies as a ForgotPasswordRequest gives the email
// its handler reads.
var resetLimitKey = middleware.KeyByIPAndBody(func(r *ForgotPasswordRequest) string { return r.Email })

// ForgotPasswordRequest is the POST /auth/password/forgot body.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// VerifyCodeRequest is the POST /auth/password/verify body.
type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
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

// resetCodeShape reports whether code is what newResetCode makes: exactly
// resetCodeDigits ASCII digits. Six bytes alone are not enough: letters, or
// the digits of another script (two or three bytes each), are not a code.
func resetCodeShape(code string) bool {
	if len(code) != resetCodeDigits {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
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
	// Past the grace period of a deletion its owner asked for, the account
	// is answered as no account: no code (Issue checks the same rule).
	if u.Disabled || u.DeletionDue || u.PasswordHash == nil || *u.PasswordHash == "" {
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

// verify tells the app whether the code is the right one before it asks for
// the new password (developer, 2026-11-13). It checks the code exactly as
// reset does, in the same order and with the same answers (checkCode), and a
// wrong code costs a try here as there. The right code is answered 200 {}
// and is neither spent nor counted, so it still works at reset, where the app
// sends it again with the new password.
func (h *PasswordResetHandler) verify(c *gin.Context) {
	var req VerifyCodeRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	// A code that could never be one (not six ASCII digits) is told so and
	// costs no try: it is never checked against the store.
	code := strings.TrimSpace(req.Code)
	if !resetCodeShape(code) {
		RespondFieldError(c, "code", reasonInvalid)
		return
	}
	if _, _, ok := h.checkCode(c, req.Email, code); !ok {
		return
	}
	// Nothing was spent and nothing changed: the same code still sets the
	// password at reset until it expires, is replaced or is used.
	Respond(c, http.StatusOK, gin.H{})
}

func (h *PasswordResetHandler) reset(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	// A code that could never be one (not six ASCII digits) is told so and
	// costs no try: it is never checked against the store.
	code := strings.TrimSpace(req.Code)
	if !resetCodeShape(code) {
		RespondFieldError(c, "code", reasonInvalid)
		return
	}
	if reason := auth.PasswordReason(req.NewPassword); reason != "" {
		RespondFieldError(c, "new_password", reason)
		return
	}
	// bcrypt's ceiling counts bytes, so 72 characters can still be too many.
	// Said before the code is checked, so it never costs a try.
	if len(req.NewPassword) > auth.MaxPasswordBytes {
		RespondFieldError(c, "new_password", reasonTooLong)
		return
	}

	// Reset checks the code itself, as verify does: it never takes a pass at
	// verify on trust, and the code may have expired, been replaced or been
	// used since.
	u, codeHash, ok := h.checkCode(c, req.Email, code)
	if !ok {
		return
	}

	// The change itself runs on the request's context: a person who gives up part way keeps the
	// code, and only someone with the right code gets this far.
	accessToken, refreshToken, restored, err := h.setPassword(c.Request.Context(), u.ID, codeHash, req.NewPassword, c.Request.UserAgent())
	switch {
	case err == nil:
	case errors.Is(err, user.ErrResetCodeReplaced):
		// A newer code replaced the right one while the password was being
		// set. Nothing changed and no try was counted.
		RespondFieldError(c, "code", reasonReplaced)
		return
	case errors.Is(err, user.ErrResetCodeUsed):
		// Another request used the right code first. No try was counted.
		RespondFieldError(c, "code", reasonUsed)
		return
	case errors.Is(err, user.ErrAccountDisabled), errors.Is(err, user.ErrNotFound):
		// Disabled or gone while this ran: what any wrong code gets.
		RespondFieldError(c, "code", reasonWrongCode)
		return
	case errors.Is(err, auth.ErrPasswordTooLong):
		RespondFieldError(c, "new_password", reasonTooLong)
		return
	default:
		// The code was right but the password did not change: the server or
		// the database failed, or the app gave up. The right code cost no try
		// and was not spent, so the app can simply ask again, even on the
		// last try.
		RespondError(c, err)
		return
	}
	body := gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTTL.Seconds()),
		"user":          apiUser(u, h.photoLink),
	}
	// Inside the grace period of a deletion its owner asked for, the reset
	// kept the account, as log in does, and says so once.
	if restored {
		body["restored"] = true
	}
	Respond(c, http.StatusOK, body)
}

// checkCode checks code, already of the right shape (resetCodeShape), against
// the live code of the account of email. Verify and reset both check it here,
// so they answer alike after the same work. ok is false once it has answered:
// 400 field "code" with reason "replaced" for the code the newest one
// replaced, or "wrong_code" for any other code that is not the right one, or
// the error of a lookup or try that failed. With ok it has answered nothing:
// the code is right and was neither spent nor counted, u is its account, and
// codeHash is the hash reset spends it by.
func (h *PasswordResetHandler) checkCode(c *gin.Context, email, code string) (u *user.User, codeHash []byte, ok bool) {
	// The lookup and the try do not use the request's context: a client that half-closes its
	// connection cancels it, which would end them early and change which answer comes back.
	ctx, cancelWork := context.WithTimeout(context.WithoutCancel(c.Request.Context()), resetWorkDeadline)
	defer cancelWork()
	// An email with no account, with a disabled one, or with one past the
	// grace period of a deletion its owner asked for, runs the same
	// statement as any other, for no one (uuid.Nil has no code), so its
	// answer, the wrong-code answer, comes after the same work.
	userID := uuid.Nil
	u, err := h.users.GetByEmailCI(ctx, strings.ToLower(strings.TrimSpace(email)))
	switch {
	case err == nil && !u.Disabled && !u.DeletionDue:
		userID = u.ID
	case err != nil && !errors.Is(err, user.ErrNotFound):
		// The lookup failed: not a wrong code, which the app would count.
		RespondError(c, err)
		return nil, nil, false
	}
	codeHash = h.codeHash(userID, code)
	right, replaced, err := h.store.Try(ctx, userID, codeHash, h.now(), resetMaxAttempts)
	switch {
	case err != nil:
		RespondError(c, err)
	case replaced:
		RespondFieldError(c, "code", reasonReplaced)
	case !right:
		RespondFieldError(c, "code", reasonWrongCode)
	default:
		// Only an account's own code is right (uuid.Nil has none), so u is
		// that account.
		return u, codeHash, true
	}
	return nil, nil, false
}

// setPassword sets the new password with the right code and makes the new
// session. The code is spent in the same transaction that changes the
// password, by one request only: the right code sent twice at once changes the
// password once (the other request gets user.ErrResetCodeUsed), no change can
// leave the code usable, and a change that fails or is given up part way
// leaves the code as it was. restored is true when the change also cancelled
// a deletion the owner had asked for.
func (h *PasswordResetHandler) setPassword(ctx context.Context, userID uuid.UUID, codeHash []byte,
	newPassword, userAgent string) (accessToken, refreshToken string, restored bool, err error) {
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", "", false, err
	}
	// The new session's id is chosen here, so its access token can be made
	// before anything is stored and still name it.
	session := uuid.New()
	accessToken, err = auth.IssueSessionAccessToken(userID, session, h.secret, h.accessTTL)
	if err != nil {
		return "", "", false, err
	}
	refreshToken, err = h.refresh.IssueInTransaction(ctx, func(tokenHash []byte, expiresAt time.Time) error {
		var spendErr error
		restored, spendErr = h.store.SpendAndChangePassword(ctx, userID, codeHash, newHash, session, tokenHash, expiresAt, &userAgent)
		return spendErr
	})
	if err != nil {
		return "", "", false, err
	}
	return accessToken, refreshToken, restored, nil
}
