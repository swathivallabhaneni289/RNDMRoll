// Package httpapi: signup_code.go finishes a sign-up with the 6-digit code
// it emails (developer, 2026-11-24: "lets add it", after asking for sign-up
// to work like any other website, with not too many checks; Instagram emails
// a 6-digit code during sign-up). The account is made only once the code is
// typed into the app:
//
//   - POST /auth/signup (AuthHandler.Signup) checks every field as before,
//     then stores the sign-up, waiting for its code (start), and answers
//     200 {"signup_token", "email"} with no session. The code is emailed
//     after the answer.
//   - POST /auth/signup/verify {signup_token, code} makes the account with
//     the right code, verified, and answers exactly what log in answers.
//   - POST /auth/signup/resend {signup_token} emails a new code.
//
// The code rules are Forgot password's (password_reset.go), kept per
// sign-up rather than per account:
//
//   - The token is 32 random bytes the app holds; only its SHA-256 is
//     stored. Only an HMAC of the code is stored, bound to the sign-up and
//     its email with the server's secret, so a code finishes only its own
//     sign-up: a stranger who starts a sign-up with someone else's email
//     gets no account from the code that person types into their own app.
//     Every start is its own row, so no start disturbs another person's
//     sign-up.
//   - A code lives 30 minutes. A sign-up allows 5 wrong tries, kept across
//     its codes (a new code never resets them), and at most 5 codes, the
//     first and 4 more. Only a wrong code costs a try, counted in the same
//     statement that checks it, so wrong guesses sent at once still stop at
//     5. A code under a minute old is not replaced (nothing is emailed), and
//     the code a new one replaced is answered "replaced" at no try.
//   - The right code spends the sign-up in the transaction that makes the
//     account, and every sign-up of the email goes with it, so the right
//     code sent twice at once makes one account.
//   - A sign-up holds neither the email nor the username for anyone: the
//     account's own unique indexes decide when the code makes it.
//   - One email may start at most 5 sign-ups an hour (each emails a code);
//     past that the start answers 429. Sign-up's own two limits stand in
//     front of that, and verify and resend share two more (Register).
//   - No answer floor: the token is a 256-bit secret, so how long an answer
//     takes tells a stranger nothing.
//
// Log in, the accounts made before 2026-11-24 (never checked) and every
// other route stay as they were.
package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// The code rules, the reset's (password_reset.go) kept per sign-up.
const (
	signupCodeTTL     = 30 * time.Minute
	signupMaxAttempts = 5
	// signupMaxCodes is the most codes one sign-up gets: the first and 4
	// more. After that the app starts again.
	signupMaxCodes = 5
	// signupResendAfter: a younger code is not replaced, so a second tap
	// cannot spoil the email on its way or fill the inbox.
	signupResendAfter = time.Minute
	// signupStartsPerEmail is the most sign-ups one email may start in
	// user.SignupCapWindow (an hour): each emails a code, so the cap bounds
	// what a stranger can send to someone else's inbox.
	signupStartsPerEmail = 5
	// signupTokenBytes is the token's entropy: 256 bits, a refresh token's.
	signupTokenBytes = 32
)

// SignupStore keeps the sign-ups waiting for their code
// (postgres.PendingSignupRepo).
type SignupStore interface {
	// Start stores the sign-up p with its token and first code, in one
	// statement, unless its email has had maxStarts sign-ups started after
	// countSince. stored: it was kept.
	Start(ctx context.Context, p user.PendingSignup, tokenHash, codeHash []byte, expiresAt, at, countSince time.Time, maxStarts int) (stored bool, err error)
	// Find returns the sign-up of the token tokenHash while its code is live
	// at at; user.ErrSignupExpired when there is none.
	Find(ctx context.Context, tokenHash []byte, at time.Time) (*user.PendingSignup, error)
	// Try checks codeHash against the sign-up's live code in one statement
	// and counts a try only when it is wrong. right: it is the code.
	// replaced: it is the code the live one replaced (no try counted). Both
	// false: wrong, or the tries are used up. user.ErrSignupExpired: the code
	// expired, or the sign-up is gone.
	Try(ctx context.Context, id uuid.UUID, codeHash []byte, at time.Time, maxTries int) (right, replaced bool, err error)
	// Reissue replaces the sign-up's live code with codeHash, keeping its
	// tries. stored: kept, and so to be emailed. false with no error: the live
	// code was made after freshSince and stands. user.ErrSignupExpired: no new
	// code can be had (gone, expired, tries used up, or maxCodes codes made).
	Reissue(ctx context.Context, id uuid.UUID, codeHash []byte, expiresAt, at, freshSince time.Time, maxTries, maxCodes int) (stored bool, err error)
	// Finish makes the account in, in the transaction that spends the
	// sign-up if its code is still codeHash and deletes every sign-up of its
	// email. With nothing changed: user.ErrSignupExpired (another request
	// made it first), user.ErrSignupCodeReplaced, or the insert's refusal
	// (user.ErrEmailTaken, user.ErrUsernameTaken, a *user.FieldError).
	Finish(ctx context.Context, id uuid.UUID, codeHash []byte, in user.NewAccount) (*user.User, error)
}

// SignupCodeHandler stores the sign-ups POST /auth/signup starts
// (AuthHandler.Signup hands each one to start) and serves the two routes
// that finish them.
type SignupCodeHandler struct {
	users user.Repository
	store SignupStore
	// emails sends the code emails, after the answers.
	emails emailer
	secret []byte
	// now is the handler's clock: the codes' times, the hourly cap and the
	// limits all read it (a test moves it). The age a new account starts
	// with is judged on the real day, as sign-up's own checks are.
	now func() time.Time
	// auth answers a finished sign-up with log in's session (issueSession).
	// NewServer sets it.
	auth *AuthHandler
}

// NewSignupCodeHandler builds the handler. store keeps the sign-ups and
// makes each account with the code it spends; sender delivers the code
// emails; secret (the server's, as the reset's codes use) binds each code to
// its sign-up.
func NewSignupCodeHandler(users user.Repository, store SignupStore, sender mail.Sender, secret []byte) *SignupCodeHandler {
	return &SignupCodeHandler{users: users, store: store, emails: emailer{sender: sender}, secret: secret, now: time.Now}
}

// clock is now as a limit's clock, read when each request is counted, so a
// test can set now after Register.
func (h *SignupCodeHandler) clock() time.Time { return h.now() }

// Wait waits for every code email started so far (tests and shutdown).
func (h *SignupCodeHandler) Wait() { h.emails.Wait() }

// Register mounts verify and resend on the public /v1 group. They share two
// limits (the same two limiters, so one count for both): 10 a minute per
// address in total, then 5 a minute per address and token. A sign-up's 5
// wrong tries end it whatever the limits allow; they bound the database
// work one address can ask for with tokens it made up or kept.
func (h *SignupCodeHandler) Register(rg *gin.RouterGroup) {
	perAddress := middleware.RateLimit(middleware.LimitConfig{
		Requests: 10, Window: time.Minute, KeyFunc: middleware.KeyByIP, Clock: h.clock,
	})
	perToken := middleware.RateLimit(middleware.LimitConfig{
		Requests: 5, Window: time.Minute, KeyFunc: signupTokenLimitKey, Clock: h.clock,
	})
	rg.POST("/auth/signup/verify", perAddress, perToken, h.verify)
	rg.POST("/auth/signup/resend", perAddress, perToken, h.resend)
}

// signupTokenLimitKey keys the per-token limit verify and resend share by
// the token the handlers use. A verify body names the sign-up in the same
// signup_token field as a resend body, and no other field of either matches
// that key, so reading either body as a ResendSignupCodeRequest gives the
// token its handler reads.
var signupTokenLimitKey = middleware.KeyByIPAndBody(func(r *ResendSignupCodeRequest) string { return r.SignupToken })

// VerifySignupRequest is the POST /auth/signup/verify body.
type VerifySignupRequest struct {
	SignupToken string `json:"signup_token"`
	Code        string `json:"code"`
}

// ResendSignupCodeRequest is the POST /auth/signup/resend body.
type ResendSignupCodeRequest struct {
	SignupToken string `json:"signup_token"`
}

// newSignupToken makes the token the app holds until the code is typed: 32
// bytes from crypto/rand, base64url-encoded, and the SHA-256 of those bytes,
// which is all the sign-up keeps (a refresh token's scheme).
func newSignupToken() (token string, hash []byte, err error) {
	buf := make([]byte, signupTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(buf)
	return base64.RawURLEncoding.EncodeToString(buf), sum[:], nil
}

// signupTokenHash hashes a token the app sent, as newSignupToken hashed it.
// ok is false for one that is empty or does not decode: no sign-up has it.
func signupTokenHash(token string) (hash []byte, ok bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}

// codeHash is the HMAC of code with the server's secret, bound to the
// sign-up id and its email (lower case): the same six digits hash to
// something else for every other sign-up, so a code finishes only its own.
func (h *SignupCodeHandler) codeHash(id uuid.UUID, email, code string) []byte {
	m := hmac.New(sha256.New, h.secret)
	m.Write([]byte("rndmroll-signup:" + id.String() + ":" + strings.ToLower(email) + ":" + code))
	return m.Sum(nil)
}

// signupMessage is the sign-up code email. Its subject is the reset's
// (resetMessage), so whatever reads a code from the subject reads either.
// Plain words, no em dashes.
func signupMessage(code string) (subject, html, text string) {
	subject = "Your RNDMRoll code: " + code
	body := "Use this code in the app to finish signing up:"
	footer := "It expires in 30 minutes. If you did not sign up for RNDMRoll, you can ignore this message."
	text = fmt.Sprintf("%s\n\n%s\n\n%s", body, code, footer)
	html = fmt.Sprintf("<p>%s</p><p style=\"font-size:28px;letter-spacing:4px\"><strong>%s</strong></p><p>%s</p>", body, code, footer)
	return subject, html, text
}

// sendCode emails code to address. A failed email is logged and changes
// nothing: the person can ask for a new code.
func (h *SignupCodeHandler) sendCode(ctx context.Context, address, code string) {
	subject, html, text := signupMessage(code)
	if err := h.emails.sender.Send(ctx, address, subject, html, text); err != nil {
		log.Printf("httpapi: sign-up code email: %v", err)
	}
}

// start stores a sign-up that passed every check of POST /auth/signup (in,
// the account it will make, without its visibility) and answers it: 200
// {"signup_token", "email"} with no session, the email as the account will
// store it, and the code is emailed after the answer. Past the hourly cap of
// its email it answers 429 rate_limited and stores nothing. ctx is the
// start's work context, off the request's cancel (Signup says why).
func (h *SignupCodeHandler) start(ctx context.Context, c *gin.Context, in user.NewAccount) {
	token, tokenHash, err := newSignupToken()
	if err != nil {
		RespondError(c, err)
		return
	}
	code, err := newResetCode()
	if err != nil {
		RespondError(c, err)
		return
	}
	p := user.PendingSignup{ID: uuid.New(), NewAccount: in}
	now := h.now().UTC()
	stored, err := h.store.Start(ctx, p, tokenHash, h.codeHash(p.ID, in.Email, code), now.Add(signupCodeTTL), now,
		now.Add(-user.SignupCapWindow), signupStartsPerEmail)
	if err != nil {
		RespondError(c, err)
		return
	}
	if !stored {
		// The email has started its 5 sign-ups this hour, each with a code
		// emailed: the usual answer of a limit.
		middleware.RespondRateLimited(c)
		return
	}
	// The answer never waits for the email: a slow email never makes the app
	// give up on a sign-up that was stored.
	Respond(c, http.StatusOK, gin.H{"signup_token": token, "email": in.Email})
	address := in.Email
	h.emails.background(func(ctx context.Context) { h.sendCode(ctx, address, code) })
}

// find looks up the live sign-up of token. ok is false once it has answered:
// 400 field (code at verify, signup_token at resend) with reason "expired"
// when there is none, a token that does not decode included, or the error
// of a lookup that failed.
func (h *SignupCodeHandler) find(ctx context.Context, c *gin.Context, token, field string) (*user.PendingSignup, bool) {
	tokenHash, ok := signupTokenHash(token)
	if !ok {
		RespondFieldError(c, field, reasonExpired)
		return nil, false
	}
	p, err := h.store.Find(ctx, tokenHash, h.now())
	switch {
	case errors.Is(err, user.ErrSignupExpired):
		RespondFieldError(c, field, reasonExpired)
		return nil, false
	case err != nil:
		RespondError(c, err)
		return nil, false
	}
	return p, true
}

// verify makes the account with the right code and answers exactly what log
// in answers (issueSession). In order: a code that could never be one (not
// six ASCII digits) is 400 field "code", reason "invalid", and costs no try;
// a token no live sign-up has is reason "expired"; then the code is tried
// (Try): the code the newest one replaced is "replaced", at no try, and any
// other wrong code, or any code once the 5 tries are used up, is
// "wrong_code". The right code makes the account in the transaction that
// spends the sign-up (Finish), verified.
func (h *SignupCodeHandler) verify(c *gin.Context) {
	var req VerifySignupRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	code := strings.TrimSpace(req.Code)
	if !resetCodeShape(code) {
		RespondFieldError(c, "code", reasonInvalid)
		return
	}
	// Not the request's context: a client that half-closes its connection
	// cancels it, which would end the try or the account's insert early and
	// change which answer comes back. The session itself is issued on the
	// request's context, as log in's is: an account made for a client that
	// is gone logs in with its password.
	ctx, cancelWork := context.WithTimeout(context.WithoutCancel(c.Request.Context()), signupWorkDeadline)
	defer cancelWork()
	p, ok := h.find(ctx, c, req.SignupToken, "code")
	if !ok {
		return
	}
	codeHash := h.codeHash(p.ID, p.Email, code)
	right, replaced, err := h.store.Try(ctx, p.ID, codeHash, h.now(), signupMaxAttempts)
	switch {
	case errors.Is(err, user.ErrSignupExpired):
		// The code expired, or the account was made, since the lookup.
		RespondFieldError(c, "code", reasonExpired)
		return
	case err != nil:
		RespondError(c, err)
		return
	case replaced:
		RespondFieldError(c, "code", reasonReplaced)
		return
	case !right:
		RespondFieldError(c, "code", reasonWrongCode)
		return
	}

	// A new account starts public, except that one under 18 on the day it is
	// made starts private (user.DefaultVisibility is the whole rule), judged
	// on the real day, as sign-up judges the age. The person can switch it
	// either way in Settings.
	birth, err := time.Parse("2006-01-02", p.Birthday)
	if err != nil {
		RespondError(c, err)
		return
	}
	in := p.NewAccount
	in.AccountVisibility = user.DefaultVisibility(user.AgeOn(birth, time.Now()))
	in.EmailVerified = true
	u, err := h.store.Finish(ctx, p.ID, codeHash, in)
	switch {
	case err == nil:
	case errors.Is(err, user.ErrSignupCodeReplaced):
		// A new code replaced the right one while the account was being
		// made. Nothing changed and no try was counted.
		RespondFieldError(c, "code", reasonReplaced)
		return
	case errors.Is(err, user.ErrSignupExpired):
		// Another request made the account first, with this code or with
		// another sign-up's of the email. No try was counted.
		RespondFieldError(c, "code", reasonExpired)
		return
	case errors.Is(err, user.ErrUsernameTaken):
		// Someone took the username since the start (a sign-up holds none).
		// Nothing was made, and the sign-up still waits.
		respondUsernameTaken(c, h.users, p.Username)
		return
	default:
		// A taken email is 409 email_taken, as at the start; anything else
		// that failed is a server error, and the right code was not spent.
		RespondError(c, err)
		return
	}
	h.auth.issueSession(c, u, http.StatusOK, false)
}

// resend emails a new code for the sign-up: 200 {} once the new code is
// stored (emailed after the answer), and 200 {} as well, with nothing
// emailed, while the live code is under a minute old, as Forgot password
// answers (the code just sent still works). A sign-up that is gone or whose
// code expired, one whose tries are used up and one that has had its 5
// codes are 400 field "signup_token", reason "expired", and the app starts
// again. After the 5th code that answer only means no new code can come:
// the 5th still finishes the sign-up at verify until it expires.
func (h *SignupCodeHandler) resend(c *gin.Context) {
	var req ResendSignupCodeRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	// Not the request's context, as at verify: a half-closed connection must
	// not turn a stored code into a 500.
	ctx, cancelWork := context.WithTimeout(context.WithoutCancel(c.Request.Context()), signupWorkDeadline)
	defer cancelWork()
	p, ok := h.find(ctx, c, req.SignupToken, "signup_token")
	if !ok {
		return
	}
	code, err := newResetCode()
	if err != nil {
		RespondError(c, err)
		return
	}
	now := h.now().UTC()
	stored, err := h.store.Reissue(ctx, p.ID, h.codeHash(p.ID, p.Email, code), now.Add(signupCodeTTL), now,
		now.Add(-signupResendAfter), signupMaxAttempts, signupMaxCodes)
	switch {
	case errors.Is(err, user.ErrSignupExpired):
		RespondFieldError(c, "signup_token", reasonExpired)
		return
	case err != nil:
		RespondError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{})
	if stored {
		address := p.Email
		h.emails.background(func(ctx context.Context) { h.sendCode(ctx, address, code) })
	}
}
