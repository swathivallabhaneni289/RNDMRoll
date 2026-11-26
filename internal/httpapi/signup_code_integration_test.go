package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// signupCodeSubject is the subject a code email has: the reset's format, so
// a live check reads the code from it.
const signupCodeSubject = "Your RNDMRoll code: "

// signupWorld is the whole server over the real database, with sign-up's
// code step on a clock the test moves.
type signupWorld struct {
	t      *testing.T
	pool   *pgxpool.Pool
	engine *gin.Engine
	h      *SignupCodeHandler
	box    *captureSender
	now    time.Time
}

func newSignupWorld(t *testing.T) *signupWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	w := &signupWorld{t: t, pool: pool, box: &captureSender{}, now: time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)}
	w.engine = buildIntegrationServerWith(t, pool, integrationOptions{signupMail: w.box}).Engine()
	w.h = signupBoxOf(t, w.engine).h
	w.h.now = func() time.Time { return w.now }
	return w
}

// startAnswer starts a sign-up with signupBody changed by mods, from a fresh
// address, and waits for the email it started.
func (w *signupWorld) startAnswer(mods map[string]any) (int, map[string]any) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/signup", "", signupBody(mods))
	w.h.Wait()
	return rec.Code, decodeBody(w.t, rec)
}

// start starts a sign-up that must be stored and answers its token.
func (w *signupWorld) start(mods map[string]any) string {
	w.t.Helper()
	code, body := w.startAnswer(mods)
	token, _ := body["signup_token"].(string)
	if code != http.StatusOK || token == "" {
		w.t.Fatalf("start %v: %d %v, want 200 with a token", mods, code, body)
	}
	return token
}

// code is the code emailed last to email.
func (w *signupWorld) code(email string) string {
	w.t.Helper()
	w.h.Wait()
	return w.box.lastCode(w.t, email)
}

// verify sends code for token and checks the status.
func (w *signupWorld) verify(token, code string, want int) map[string]any {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/signup/verify", "", map[string]any{"signup_token": token, "code": code})
	if rec.Code != want {
		w.t.Fatalf("verify: %d %s, want %d", rec.Code, rec.Body.String(), want)
	}
	return decodeBody(w.t, rec)
}

// refused sends code for token and checks the answer is 400 field "code"
// with reason.
func (w *signupWorld) refused(token, code, reason string) {
	w.t.Helper()
	got := w.verify(token, code, http.StatusBadRequest)
	if got["error"] != string(CodeValidationFailed) || got["field"] != "code" || got["reason"] != reason || got["message"] != fieldReasonText[reason] {
		w.t.Fatalf("verify: %v, want field code, reason %s", got, reason)
	}
}

// resend asks for a new code for token, waits for any email it started and
// checks the status.
func (w *signupWorld) resend(token string, want int) map[string]any {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/signup/resend", "", map[string]any{"signup_token": token})
	w.h.Wait()
	if rec.Code != want {
		w.t.Fatalf("resend: %d %s, want %d", rec.Code, rec.Body.String(), want)
	}
	return decodeBody(w.t, rec)
}

// resendRefused asks for a new code for token and checks the answer is 400
// field "signup_token", reason "expired".
func (w *signupWorld) resendRefused(token string) {
	w.t.Helper()
	got := w.resend(token, http.StatusBadRequest)
	if got["error"] != string(CodeValidationFailed) || got["field"] != "signup_token" || got["reason"] != reasonExpired {
		w.t.Fatalf("resend: %v, want field signup_token, reason expired", got)
	}
}

// tokenHash is the token's hash as the row keeps it.
func (w *signupWorld) tokenHash(token string) []byte {
	w.t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		w.t.Fatalf("the token does not decode: %v", err)
	}
	sum := sha256.Sum256(raw)
	return sum[:]
}

// tries is how many tries the sign-up of token has used, and whether it
// still waits.
func (w *signupWorld) tries(token string) (int, bool) {
	w.t.Helper()
	var n int
	err := w.pool.QueryRow(context.Background(), `select attempts from pending_signups where token_hash = $1`, w.tokenHash(token)).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		w.t.Fatal(err)
	}
	return n, true
}

// wantTries checks the sign-up of token still waits with n tries used.
func (w *signupWorld) wantTries(what, token string, n int) {
	w.t.Helper()
	if got, ok := w.tries(token); !ok || got != n {
		w.t.Fatalf("%s: the sign-up has %d tries (waiting: %v), want %d", what, got, ok, n)
	}
}

// waiting is how many sign-ups of email wait, in any case.
func (w *signupWorld) waiting(email string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), `select count(*) from pending_signups where lower(email) = lower($1)`, email).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// The whole sign-up: the start answers a token and the email with no
// session, stores no account and keeps no token or code in the clear; the
// code is emailed, with the reset's subject; the right code makes the
// account, verified, and answers exactly what log in answers; and the
// sign-up is gone after.
func TestSignupCodeIntegration(t *testing.T) {
	w := newSignupWorld(t)
	const typed, email, password = "New-User@Example.com", "new-user@example.com", "Correct-horse-battery1"

	status, started := w.startAnswer(map[string]any{"email": typed})
	if status != http.StatusOK || keysOf(started) != "email,signup_token" || started["email"] != typed {
		t.Fatalf("start: %d %v, want 200 with only signup_token and the email as typed", status, started)
	}
	token, _ := started["signup_token"].(string)
	if raw, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(raw) != signupTokenBytes {
		t.Fatalf("the token %q is not %d bytes of base64url: %v", token, signupTokenBytes, err)
	}
	if n := integrationUserCount(t, w.pool); n != 0 {
		t.Fatalf("the start made %d account(s)", n)
	}
	if n := w.waiting(email); n != 1 {
		t.Fatalf("%d sign-ups wait, want 1", n)
	}

	// One email, to the address as typed, with the code in its subject and
	// its body, in plain words.
	if n := w.box.count(); n != 1 {
		t.Fatalf("%d emails, want 1", n)
	}
	code := w.code(typed)
	w.box.mu.Lock()
	sent := w.box.sent[0]
	w.box.mu.Unlock()
	if sent.to != typed || sent.subject != signupCodeSubject+code || len(code) != 6 {
		t.Fatalf("the code email: to %q, subject %q, code %q", sent.to, sent.subject, code)
	}
	for _, want := range []string{"Use this code in the app to finish signing up:", code,
		"It expires in 30 minutes. If you did not sign up for RNDMRoll, you can ignore this message."} {
		if !strings.Contains(sent.text, want) {
			t.Fatalf("the code email %q does not say %q", sent.text, want)
		}
	}
	if strings.ContainsAny(sent.subject+sent.text, "—–") {
		t.Fatalf("the code email has a dash: %q", sent.text)
	}

	// The row keeps hashes only: the token's SHA-256, and a code HMAC that is
	// neither the code nor its plain hash.
	var codeHash []byte
	if err := w.pool.QueryRow(context.Background(), `select code_hash from pending_signups where token_hash = $1`, w.tokenHash(token)).Scan(&codeHash); err != nil {
		t.Fatalf("no row under the token's hash: %v", err)
	}
	if plain := sha256.Sum256([]byte(code)); string(codeHash) == code || string(codeHash) == string(plain[:]) {
		t.Fatal("the row keeps the code, or its plain hash")
	}

	// The right code: log in's answer, the account verified.
	got := w.verify(token, code, http.StatusOK)
	login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": password})
	if login.Code != http.StatusOK {
		t.Fatalf("log in with the sign-up's password: %d %s", login.Code, login.Body.String())
	}
	loggedIn := decodeBody(t, login)
	if keysOf(got) != keysOf(loggedIn) || keysOf(userOf(t, got)) != keysOf(userOf(t, loggedIn)) {
		t.Fatalf("verify answered %q (user %q), log in %q (user %q)",
			keysOf(got), keysOf(userOf(t, got)), keysOf(loggedIn), keysOf(userOf(t, loggedIn)))
	}
	if got["expires_in"] != float64(900) || userOf(t, got)["email"] != typed || userOf(t, got)["email_verified"] != true {
		t.Fatalf("verify: %v", got)
	}
	var verified bool
	var via string
	if err := w.pool.QueryRow(context.Background(), `select email_verified, email_verified_via from users where lower(email) = $1`, email).Scan(&verified, &via); err != nil || !verified || via != "password_flow" {
		t.Fatalf("the account: verified %v via %q (err %v), want true via password_flow", verified, via, err)
	}
	if access, _ := got["access_token"].(string); integrationDo(t, w.engine, http.MethodGet, "/v1/me", access, nil).Code != http.StatusOK {
		t.Fatal("the verify's session does not reach /me")
	}

	// Done: the sign-up is gone, so its code and its token are expired.
	if n := w.waiting(email); n != 0 {
		t.Fatalf("%d sign-ups still wait", n)
	}
	w.refused(token, code, reasonExpired)
	w.resendRefused(token)
	if n := integrationUserCount(t, w.pool); n != 1 {
		t.Fatalf("%d accounts, want 1", n)
	}
}

// Only a wrong code costs a try, and 5 end the sign-up: then even the right
// code is wrong, and no new code is sent.
func TestSignupCodeIntegration_FiveWrongTriesEndTheSignUp(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	code := w.code("new-user@example.com")
	for i := range signupMaxAttempts {
		w.refused(token, otherCode(code), reasonWrongCode)
		w.wantTries("a wrong code", token, i+1)
	}
	w.refused(token, code, reasonWrongCode)
	w.wantTries("the right code once used up", token, signupMaxAttempts)
	w.now = w.now.Add(2 * time.Minute)
	w.resendRefused(token)
	if n := w.box.count(); n != 1 {
		t.Fatalf("a used-up sign-up got a new code: %d emails", n)
	}
	if n := integrationUserCount(t, w.pool); n != 0 {
		t.Fatalf("%d accounts, want none", n)
	}
}

// A new code keeps the tries: 3 wrong on the first code and 2 on the second
// use the sign-up up.
func TestSignupCodeIntegration_ANewCodeKeepsTheTries(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	first := w.code("new-user@example.com")
	for range 3 {
		w.refused(token, otherCode(first), reasonWrongCode)
	}
	w.now = w.now.Add(2 * time.Minute)
	if got := w.resend(token, http.StatusOK); len(got) != 0 {
		t.Fatalf("resend answered %v, want {}", got)
	}
	if n := w.box.count(); n != 2 {
		t.Fatalf("%d emails, want 2", n)
	}
	second := w.code("new-user@example.com")
	w.wantTries("after the new code", token, 3)
	for range 2 {
		w.refused(token, otherCode(second), reasonWrongCode)
	}
	w.refused(token, second, reasonWrongCode)
	w.wantTries("used up", token, signupMaxAttempts)
}

// The code a newer one replaced is told apart and costs no try; only the code
// just replaced is, and the newest one works.
func TestSignupCodeIntegration_TheReplacedCodeIsToldAndCostsNoTry(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	first := w.code("new-user@example.com")
	w.refused(token, otherCode(first), reasonWrongCode) // 1 try
	w.now = w.now.Add(2 * time.Minute)
	w.resend(token, http.StatusOK)
	second := w.code("new-user@example.com")
	distinctCodes(t, first, second)
	for range signupMaxAttempts + 1 {
		w.refused(token, first, reasonReplaced)
	}
	w.wantTries("after the replaced code 6 times", token, 1)

	w.now = w.now.Add(2 * time.Minute)
	w.resend(token, http.StatusOK)
	third := w.code("new-user@example.com")
	distinctCodes(t, first, second, third)
	w.refused(token, first, reasonWrongCode)
	w.refused(token, second, reasonReplaced)
	w.wantTries("after a code two back", token, 2)
	w.verify(token, third, http.StatusOK)
}

// Asking again within a minute sends nothing and keeps the code, answered as
// a code sent; after the minute a new code goes out.
func TestSignupCodeIntegration_AResendWithinAMinuteSendsNothing(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	first := w.code("new-user@example.com")
	w.now = w.now.Add(30 * time.Second)
	if got := w.resend(token, http.StatusOK); len(got) != 0 {
		t.Fatalf("resend within a minute answered %v, want {}", got)
	}
	if n := w.box.count(); n != 1 {
		t.Fatalf("a resend within a minute emailed: %d emails", n)
	}
	w.now = w.now.Add(31 * time.Second)
	w.resend(token, http.StatusOK)
	if n := w.box.count(); n != 2 {
		t.Fatalf("a resend after a minute: %d emails, want 2", n)
	}
	second := w.code("new-user@example.com")
	distinctCodes(t, first, second)
	w.refused(token, first, reasonReplaced)
	w.verify(token, second, http.StatusOK)
}

// A sign-up gets 5 codes, the first and 4 more. A sixth is refused, though a
// tap within a minute of the fifth is answered as a code sent (it is live),
// and the fifth still finishes the sign-up.
func TestSignupCodeIntegration_ASixthCodeIsRefused(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	for i := 1; i < signupMaxCodes; i++ {
		w.now = w.now.Add(2 * time.Minute)
		w.resend(token, http.StatusOK)
	}
	if n := w.box.count(); n != signupMaxCodes {
		t.Fatalf("%d emails, want %d", n, signupMaxCodes)
	}
	fifth := w.code("new-user@example.com")
	w.now = w.now.Add(10 * time.Second)
	w.resend(token, http.StatusOK)
	w.now = w.now.Add(2 * time.Minute)
	w.resendRefused(token)
	if n := w.box.count(); n != signupMaxCodes {
		t.Fatalf("a sixth code was emailed: %d emails", n)
	}
	w.verify(token, fifth, http.StatusOK)
}

// A code past its 30 minutes, a token no sign-up has and one that is not a
// token at all are each 400 reason "expired", at verify (field "code") and
// at resend (field "signup_token"), with nothing counted. A code that could
// never be one is "invalid" and costs no try either.
func TestSignupCodeIntegration_ExpiredAndUnknownTokensSayExpired(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	code := w.code("new-user@example.com")

	for _, bad := range []string{"12345", "1234567", "abcdef", "12 456", "１２３４５６"} {
		w.refused(token, bad, reasonInvalid)
	}
	w.wantTries("codes of the wrong shape", token, 0)

	for name, made := range map[string]string{"unknown": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "not a token": "%%%", "empty": ""} {
		rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/signup/verify", "", map[string]any{"signup_token": made, "code": code})
		if body := decodeBody(t, rec); rec.Code != http.StatusBadRequest || body["field"] != "code" || body["reason"] != "expired" ||
			body["message"] != "This sign-up has expired. Start again to get a new code." {
			t.Fatalf("verify with a token %s: %d %s", name, rec.Code, rec.Body.String())
		}
		rec = integrationDo(t, w.engine, http.MethodPost, "/v1/auth/signup/resend", "", map[string]any{"signup_token": made})
		if body := decodeBody(t, rec); rec.Code != http.StatusBadRequest || body["field"] != "signup_token" || body["reason"] != "expired" {
			t.Fatalf("resend with a token %s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	w.now = w.now.Add(signupCodeTTL)
	w.refused(token, code, reasonExpired)
	w.resendRefused(token)
	if n := w.box.count(); n != 1 {
		t.Fatalf("an expired sign-up got a new code: %d emails", n)
	}
	w.wantTries("the expired code", token, 0)
	if n := integrationUserCount(t, w.pool); n != 0 {
		t.Fatalf("%d accounts, want none", n)
	}
}

// Every code is bound to its own sign-up. Someone starts a sign-up with
// another person's email (the codes go to that person); the person starts
// their own. Neither code finishes the other's sign-up (each costs a try
// there), the person's own code makes their account, and the other sign-up
// goes with it.
func TestSignupCodeIntegration_ACodeFinishesOnlyItsOwnSignUp(t *testing.T) {
	w := newSignupWorld(t)
	stranger := w.start(map[string]any{"username": "not_theirs", "password": "Strangers-password-1!"})
	strangersCode := w.code("new-user@example.com")
	owner := w.start(nil)
	ownersCode := w.code("new-user@example.com")
	distinctCodes(t, strangersCode, ownersCode)

	w.refused(stranger, ownersCode, reasonWrongCode)
	w.refused(owner, strangersCode, reasonWrongCode)
	w.wantTries("the owner's sign-up", owner, 1)
	got := w.verify(owner, ownersCode, http.StatusOK)
	if userOf(t, got)["username"] != "new_user" {
		t.Fatalf("the owner's code made %v", userOf(t, got))
	}
	w.refused(stranger, strangersCode, reasonExpired)
	if n := integrationUserCount(t, w.pool); n != 1 {
		t.Fatalf("%d accounts, want 1", n)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "new-user@example.com", "password": "Strangers-password-1!"}); rec.Code == http.StatusOK {
		t.Fatal("the stranger's password opens the account")
	}
}

// One email may start 5 sign-ups an hour, in any case: the sixth is the
// usual 429 and emails nothing, while another email is not held back, and
// an hour after the five the email may start again.
func TestSignupCodeIntegration_FiveStartsAnHourPerEmail(t *testing.T) {
	w := newSignupWorld(t)
	for i := range signupStartsPerEmail {
		w.start(map[string]any{"email": "capped@example.com", "username": fmt.Sprintf("capped_%d", i)})
	}
	refused := func(what string) {
		t.Helper()
		for _, email := range []string{"capped@example.com", "CAPPED@Example.com"} {
			status, body := w.startAnswer(map[string]any{"email": email, "username": "capped_more"})
			if status != http.StatusTooManyRequests || fmt.Sprint(body) != "map[error:rate_limited]" {
				t.Fatalf("%s, a start for %s: %d %v, want 429 rate_limited", what, email, status, body)
			}
		}
	}
	refused("a sixth start")
	if n := w.box.count(); n != signupStartsPerEmail {
		t.Fatalf("%d emails, want %d", n, signupStartsPerEmail)
	}
	w.start(map[string]any{"email": "free@example.com", "username": "free_one"})
	w.now = w.now.Add(user.SignupCapWindow - time.Minute)
	refused("a minute short of the hour")
	w.now = w.now.Add(time.Minute)
	w.start(map[string]any{"email": "capped@example.com", "username": "capped_later"})
}

// A sign-up holds no username: someone else may take it before the code is
// typed. The code then makes nothing and is answered 409 username_taken with
// suggestions; the sign-up still waits, its code unspent and no try counted.
// The same for an email an account took meanwhile: 409 email_taken.
func TestSignupCodeIntegration_ANameTakenMeanwhileMakesNothing(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(map[string]any{"username": "wanted_name"})
	code := w.code("new-user@example.com")
	integrationOnboardAccount(t, w.engine, "quicker@example.com", "Correct-horse-battery1", "Quicker", "wanted_name")

	got := w.verify(token, code, http.StatusConflict)
	if suggestions, _ := got["suggestions"].([]any); got["error"] != "username_taken" || len(suggestions) != 3 {
		t.Fatalf("a username taken meanwhile: %v", got)
	}
	w.wantTries("after the taken username", token, 0)
	if n := integrationUserCount(t, w.pool); n != 1 {
		t.Fatalf("%d accounts, want only the quicker one", n)
	}

	other := w.start(map[string]any{"email": "late@example.com", "username": "late_one"})
	lateCode := w.code("late@example.com")
	if _, err := postgres.NewUserRepo(w.pool).CreateComplete(context.Background(), user.NewAccount{
		Email: "LATE@example.com", PasswordHash: "bcrypt-hash", Birthday: validBirthday, Name: "Elsewhere", Username: "elsewhere",
	}); err != nil {
		t.Fatal(err)
	}
	if got := w.verify(other, lateCode, http.StatusConflict); fmt.Sprint(got) != "map[error:email_taken]" {
		t.Fatalf("an email taken meanwhile: %v", got)
	}
	if n := integrationUserCount(t, w.pool); n != 2 {
		t.Fatalf("%d accounts, want 2", n)
	}
}

// Making the account deletes every sign-up of its email, in any case, and
// no other email's.
func TestSignupCodeIntegration_FinishingDeletesEverySignUpOfTheEmail(t *testing.T) {
	w := newSignupWorld(t)
	a := w.start(map[string]any{"email": "many@example.com", "username": "many_a"})
	b := w.start(map[string]any{"email": "Many@Example.com", "username": "many_b"})
	bCode := w.code("Many@Example.com")
	c := w.start(map[string]any{"email": "MANY@example.com", "username": "many_c"})
	w.start(map[string]any{"email": "other@example.com", "username": "other_one"})
	if n := w.waiting("many@example.com"); n != 3 {
		t.Fatalf("%d sign-ups of the email wait, want 3", n)
	}
	w.verify(b, bCode, http.StatusOK)
	if n := w.waiting("many@example.com"); n != 0 {
		t.Fatalf("%d sign-ups of the email still wait", n)
	}
	if n := w.waiting("other@example.com"); n != 1 {
		t.Fatalf("%d sign-ups of another email wait, want 1", n)
	}
	for _, token := range []string{a, c} {
		w.resendRefused(token)
	}
}

// The right code sent several times at once makes one account; every other
// request is told the sign-up is over.
func TestSignupCodeIntegration_TheRightCodeAtOnceMakesOneAccount(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	code := w.code("new-user@example.com")
	reqs := make([]*http.Request, 5)
	for i := range reqs {
		reqs[i] = integrationRequest(t, http.MethodPost, "/v1/auth/signup/verify", "", map[string]any{"signup_token": token, "code": code})
	}
	won := 0
	for i, rec := range integrationAtOnce(w.engine, reqs) {
		if rec.Code == http.StatusOK {
			won++
			continue
		}
		if body := decodeBody(t, rec); rec.Code != http.StatusBadRequest || body["field"] != "code" || body["reason"] != reasonExpired {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if won != 1 {
		t.Fatalf("one code made %d accounts' answers", won)
	}
	if n := integrationUserCount(t, w.pool); n != 1 {
		t.Fatalf("%d accounts, want 1", n)
	}
}

// Two sign-ups of one email, each finished with its own code at the same
// moment: one account, and the other request is told the sign-up is over,
// never a server error (the two never wait on each other in a circle).
func TestSignupCodeIntegration_TwoSignUpsOfOneEmailAtOnceMakeOneAccount(t *testing.T) {
	w := newSignupWorld(t)
	for round := range 5 {
		email := fmt.Sprintf("both-%d@example.com", round)
		var reqs []*http.Request
		for i := range 2 {
			token := w.start(map[string]any{"email": email, "username": fmt.Sprintf("both_%d_%d", round, i)})
			reqs = append(reqs, integrationRequest(t, http.MethodPost, "/v1/auth/signup/verify", "",
				map[string]any{"signup_token": token, "code": w.code(email)}))
		}
		won := 0
		for i, rec := range integrationAtOnce(w.engine, reqs) {
			if rec.Code == http.StatusOK {
				won++
				continue
			}
			if body := decodeBody(t, rec); rec.Code != http.StatusBadRequest || body["reason"] != reasonExpired {
				t.Fatalf("round %d, request %d: %d %s", round, i, rec.Code, rec.Body.String())
			}
		}
		if won != 1 || w.waiting(email) != 0 {
			t.Fatalf("round %d: %d made, %d still wait", round, won, w.waiting(email))
		}
	}
	if n := integrationUserCount(t, w.pool); n != 5 {
		t.Fatalf("%d accounts, want 5", n)
	}
}

// Wrong guesses sent at once are counted up to the limit and no further.
func TestSignupCodeIntegration_WrongGuessesAtOnceStopAtTheLimit(t *testing.T) {
	w := newSignupWorld(t)
	token := w.start(nil)
	code := w.code("new-user@example.com")
	reqs := make([]*http.Request, 20)
	for i := range reqs {
		reqs[i] = integrationRequest(t, http.MethodPost, "/v1/auth/signup/verify", "", map[string]any{"signup_token": token, "code": otherCode(code)})
	}
	for i, rec := range integrationAtOnce(w.engine, reqs) {
		if body := decodeBody(t, rec); rec.Code != http.StatusBadRequest || body["reason"] != reasonWrongCode {
			t.Fatalf("guess %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	w.wantTries("20 wrong guesses at once", token, signupMaxAttempts)
	w.refused(token, code, reasonWrongCode)
}
