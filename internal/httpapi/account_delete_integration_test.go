package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// deleteTestPassword is the password of every account these tests make.
const deleteTestPassword = "Correct-horse-battery-staple1"

// An ask is moved back in time with SQL, by an exact number of hours. The
// grace period is 2592000 seconds, 720 hours. An interval written in days
// follows the session's time zone across a clock change (Europe's ended on
// 2026-10-25, America's ends on 2026-11-01), so "30 days" could come out an
// hour off and put an account on the wrong side of the line.
const (
	deletionPastGrace   = "720 hours 1 minute"
	deletionInsideGrace = "719 hours"
)

// Counts the checks below read for one account.
const (
	deleteLiveSessionsQ = `select count(*) from refresh_tokens where user_id = $1 and revoked_at is null`
	deletePushTokensQ   = `select count(*) from push_tokens where user_id = $1`
	deleteResetCodesQ   = `select count(*) from password_resets where user_id = $1`
)

// deleteWorld is a feed world (one Fixed clock, everyone in New York) whose
// server also mounts the phone's push token route and the three Forgot
// password routes (their codes go to box, with no answer floor). It keeps
// the pool: some checks only the database can answer, and only SQL can move
// an ask back in time.
type deleteWorld struct {
	*pushWorld
	box   *captureSender
	reset *PasswordResetHandler
}

func newDeleteWorld(t *testing.T) *deleteWorld {
	t.Helper()
	if user.DeletionGracePeriod != 720*time.Hour {
		t.Fatalf("the grace period is %v; deletionPastGrace and deletionInsideGrace are written for 720 hours", user.DeletionGracePeriod)
	}
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &deleteWorld{pushWorld: &pushWorld{pool: pool, names: map[string]string{}}, box: &captureSender{}}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk,
		wire: func(_ *social.Service, _ *board.Service, deps *Deps) {
			deps.PushTokens = NewPushTokenHandler(postgres.NewPushRepo(pool))
			w.reset = NewPasswordResetHandler(postgres.NewUserRepo(pool),
				auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), time.Hour),
				postgres.NewPasswordResetRepo(pool), w.box, integrationJWTSecret, 15*time.Minute)
			w.reset.floor = 0
			deps.PasswordReset = w.reset
		}}).Engine()
	// The code emails go out after the answer: they finish before the
	// truncate (cleanups run last first).
	t.Cleanup(w.reset.Wait)
	w.feedWorld = &feedWorld{t: t, engine: engine, clk: clk}
	return w
}

// account signs username up (its email is username@example.com, its password
// deleteTestPassword), sets its zone to New York, and returns it with the
// session sign-up gave.
func (w *deleteWorld) account(username, name string) (feedPerson, integrationSession) {
	w.t.Helper()
	s := integrationOnboardAccount(w.t, w.engine, username+"@example.com", deleteTestPassword, name, username)
	p := feedPerson{name: name, token: s.AccessToken}
	p.id = w.ok(p, http.MethodGet, "/v1/me", nil)["id"].(string)
	w.ok(p, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": feedZone})
	return p, s
}

// askDeletion sends POST /v1/me/delete with token and body.
func (w *deleteWorld) askDeletion(token string, body any) *httptest.ResponseRecorder {
	w.t.Helper()
	return integrationDo(w.t, w.engine, http.MethodPost, "/v1/me/delete", token, body)
}

// askToDelete asks with the right password and returns the answer, which must
// be 200 with exactly delete_after (RFC 3339) and delete_on_label.
func (w *deleteWorld) askToDelete(token string) (deleteAfter time.Time, label string) {
	w.t.Helper()
	rec := w.askDeletion(token, map[string]any{"password": deleteTestPassword})
	if rec.Code != http.StatusOK {
		w.t.Fatalf("delete account with the right password: %d %s, want 200", rec.Code, rec.Body.String())
	}
	body := decodeBody(w.t, rec)
	raw, _ := body["delete_after"].(string)
	label, _ = body["delete_on_label"].(string)
	deleteAfter, err := time.Parse(time.RFC3339, raw)
	if err != nil || label == "" || len(body) != 2 {
		w.t.Fatalf("delete account answered %s, want exactly delete_after (RFC 3339) and delete_on_label (parse error: %v)", rec.Body.String(), err)
	}
	return deleteAfter, label
}

// requestedAt is the account's users.deletion_requested_at, nil when it is
// not set.
func (w *deleteWorld) requestedAt(id string) *time.Time {
	w.t.Helper()
	var at *time.Time
	if err := w.pool.QueryRow(context.Background(), `select deletion_requested_at from users where id = $1`, uuid.MustParse(id)).Scan(&at); err != nil {
		w.t.Fatalf("read deletion_requested_at of %s: %v", id, err)
	}
	return at
}

// backdateAsk moves the account's ask back so that it was made ago before
// now, on the database's clock (deletionPastGrace or deletionInsideGrace).
// Only SQL on the test database can do it: every rule reads now().
func (w *deleteWorld) backdateAsk(id, ago string) {
	w.t.Helper()
	tag, err := w.pool.Exec(context.Background(), `
		update users set deletion_requested_at = now() - $2::interval
		where id = $1 and deletion_requested_at is not null`, uuid.MustParse(id), ago)
	if err != nil || tag.RowsAffected() != 1 {
		w.t.Fatalf("move the ask of %s back by %s: %d rows, err %v", id, ago, tag.RowsAffected(), err)
	}
}

// countFor runs a count whose one parameter is the account id.
func (w *deleteWorld) countFor(q, id string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), q, uuid.MustParse(id)).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// passwordHashOf is the account's stored password hash.
func (w *deleteWorld) passwordHashOf(id string) string {
	w.t.Helper()
	var hash string
	if err := w.pool.QueryRow(context.Background(), `select password_hash from users where id = $1`, uuid.MustParse(id)).Scan(&hash); err != nil {
		w.t.Fatalf("read the password hash of %s: %v", id, err)
	}
	return hash
}

// logIn sends the app's Log in with login (an email or a username).
func (w *deleteWorld) logIn(login, password string) *httptest.ResponseRecorder {
	w.t.Helper()
	return integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": login, "password": password})
}

// askForgot sends Forgot password for email and waits for any code email it
// started.
func (w *deleteWorld) askForgot(email string) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/password/forgot", "", map[string]any{"email": email})
	w.reset.Wait()
	if rec.Code != http.StatusOK {
		w.t.Fatalf("forgot password for %s: %d %s, want 200", email, rec.Code, rec.Body.String())
	}
}

// verifyCode sends a Forgot password code to verify.
func (w *deleteWorld) verifyCode(email, code string) *httptest.ResponseRecorder {
	w.t.Helper()
	return integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/password/verify", "",
		map[string]any{"email": email, "code": code})
}

// resetPassword sends a Forgot password reset.
func (w *deleteWorld) resetPassword(email, code, newPassword string) *httptest.ResponseRecorder {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/password/reset", "",
		map[string]any{"email": email, "code": code, "new_password": newPassword})
	w.reset.Wait()
	return rec
}

// liveSessionBySQL stores, straight into the database, a live refresh token
// for the account and a push token tied to it, as a sign-in that raced an ask
// might leave behind, and returns the raw refresh token, hashed the way
// auth.RefreshService hashes one.
func (w *deleteWorld) liveSessionBySQL(id, pushToken string) string {
	w.t.Helper()
	ctx := context.Background()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		w.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	var session uuid.UUID
	if err := w.pool.QueryRow(ctx, `
		insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, now() + interval '1 hour')
		returning id`, uuid.MustParse(id), sum[:]).Scan(&session); err != nil {
		w.t.Fatalf("store a live session for %s: %v", id, err)
	}
	if _, err := w.pool.Exec(ctx, `
		insert into push_tokens (user_id, token, platform, refresh_token_id) values ($1, $2, 'ios', $3)`,
		uuid.MustParse(id), pushToken, session); err != nil {
		w.t.Fatalf("store a push token for %s: %v", id, err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// wantAccountDisabled checks that rec is RequireUser's refusal of an account
// that is disabled or being deleted: 401 and only {"error":"account_disabled"}.
func wantAccountDisabled(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := decodeBody(t, rec)
	if rec.Code != http.StatusUnauthorized || body["error"] != string(CodeAccountDisabled) || len(body) != 1 {
		t.Fatalf("%s: %d %s, want 401 {\"error\":\"account_disabled\"}", what, rec.Code, rec.Body.String())
	}
}

// wantSameAsMiss checks that got is, byte for byte, the answer a miss got.
func wantSameAsMiss(t *testing.T, what string, got, miss *httptest.ResponseRecorder) {
	t.Helper()
	if miss.Code != http.StatusNotFound {
		t.Fatalf("%s: the miss itself answered %d %s, want 404", what, miss.Code, miss.Body.String())
	}
	if got.Code != miss.Code || got.Body.String() != miss.Body.String() {
		t.Fatalf("%s: %d %s, want what a miss gets: %d %s", what, got.Code, got.Body.String(), miss.Code, miss.Body.String())
	}
}

// sortedCopy returns list sorted, leaving list as it was.
func sortedCopy(list ...string) []string {
	out := append([]string{}, list...)
	sort.Strings(out)
	return out
}

// Asking, with the right password, marks the account to be deleted after the
// grace period and, in the same moment, ends everything that could keep it
// signed in or reach it: both phones' refresh tokens are revoked, their push
// tokens deleted, and a Forgot password code waiting in the inbox deleted (so
// it cannot be used afterwards either). The answer says when, worked out by
// the database from the moment it stored, and writes that day in the
// person's own zone. The access tokens of both phones are refused at once,
// and so is asking again. Nothing is deleted yet: the account, its post, its
// email and its username stay until the purge.
func TestDeleteAccountIntegration_AsksAndEndsEverySession(t *testing.T) {
	w := newDeleteWorld(t)
	ctx := context.Background()
	b, phoneA := w.account("da_bob", "Bob Brown")
	const email = "da_bob@example.com"
	w.post(b, "2026-10-08")

	// A second phone signs in, and each phone saves its push token.
	phoneB := w.session("/v1/auth/login", "", map[string]any{"login": email, "password": deleteTestPassword})
	const tokA, tokB = "ExponentPushToken[deleteA]", "ExponentPushToken[deleteB]"
	w.saveToken(phoneA.AccessToken, tokA)
	w.saveToken(phoneB.AccessToken, tokB)
	w.wantPushedTo("two phones, before the ask", b.id, tokA, tokB)
	// A Forgot password code is waiting.
	w.askForgot(email)
	code := w.box.lastCode(t, email)
	if n := w.countFor(deleteResetCodesQ, b.id); n != 1 {
		t.Fatalf("before the ask: %d Forgot password codes stored, want 1", n)
	}
	if n := w.countFor(deleteLiveSessionsQ, b.id); n != 2 {
		t.Fatalf("before the ask: %d live sessions, want 2", n)
	}

	deleteAfter, label := w.askToDelete(phoneA.AccessToken)

	// The ask is stored, on the database's clock.
	asked := w.requestedAt(b.id)
	if asked == nil {
		t.Fatal("deletion_requested_at is not set after the ask")
	}
	var age float64
	if err := w.pool.QueryRow(ctx, `select extract(epoch from now() - deletion_requested_at)::float8 from users where id = $1`,
		uuid.MustParse(b.id)).Scan(&age); err != nil || age < 0 || age > 60 {
		t.Fatalf("the ask was stored %v seconds before the database's now (err %v), want within a minute", age, err)
	}
	// delete_after is the stored moment plus the grace period, to the
	// second: RFC 3339 drops the fraction.
	want := asked.Add(user.DeletionGracePeriod)
	if gap := want.Sub(deleteAfter); gap < 0 || gap >= time.Second {
		t.Fatalf("delete_after %s, want %s (the stored ask %s plus %v), to the second",
			deleteAfter.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano), asked.Format(time.RFC3339Nano), user.DeletionGracePeriod)
	}
	// The label is that day in New York, the zone the account stored. The
	// database writes the same day for the same instant there.
	ny, err := time.LoadLocation(feedZone)
	if err != nil {
		t.Fatal(err)
	}
	if wantLabel := deleteAfter.In(ny).Format("Monday, January 2"); label != wantLabel {
		t.Fatalf("delete_on_label %q, want %q (delete_after %s in New York)", label, wantLabel, deleteAfter.Format(time.RFC3339))
	}
	var dbLabel string
	if err := w.pool.QueryRow(ctx, `
		select to_char((deletion_requested_at + make_interval(secs => $2)) at time zone $3, 'FMDay, FMMonth FMDD')
		from users where id = $1`, uuid.MustParse(b.id), user.DeletionGracePeriod.Seconds(), feedZone).Scan(&dbLabel); err != nil {
		t.Fatal(err)
	}
	if label != dbLabel {
		t.Fatalf("delete_on_label %q, but the database writes the day %q", label, dbLabel)
	}

	// Every session ended in the same transaction: no live refresh token, no
	// push token, no Forgot password code.
	if n := w.countFor(deleteLiveSessionsQ, b.id); n != 0 {
		t.Fatalf("after the ask: %d live sessions, want 0", n)
	}
	if n := w.countFor(deletePushTokensQ, b.id); n != 0 {
		t.Fatalf("after the ask: %d push tokens kept, want 0", n)
	}
	w.wantPushedTo("after the ask", b.id)
	if n := w.countFor(deleteResetCodesQ, b.id); n != 0 {
		t.Fatalf("after the ask: %d Forgot password codes kept, want 0", n)
	}
	// The code that was waiting can no longer reset the password, which
	// would also undo the ask.
	rec := w.resetPassword(email, code, "Brand-new-password-2!")
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["reason"] != reasonWrongCode {
		t.Fatalf("a reset with the code sent before the ask: %d %s, want 400 wrong_code", rec.Code, rec.Body.String())
	}

	// Both phones' access tokens are refused at once, and so are their
	// refresh tokens. Those were revoked with the ask, so a refresh answers
	// what an ended session gets, 401 token_invalid, before the account is
	// even read (the 403 account_disabled needs a live session; the
	// GoneOnEverySurface test makes one).
	wantAccountDisabled(t, "the phone that asked, on /me", integrationDo(t, w.engine, http.MethodGet, "/v1/me", phoneA.AccessToken, nil))
	wantAccountDisabled(t, "the other phone, on /me", integrationDo(t, w.engine, http.MethodGet, "/v1/me", phoneB.AccessToken, nil))
	w.refreshRefused("the refresh token of the phone that asked", phoneA.RefreshToken)
	w.refreshRefused("the refresh token of the other phone", phoneB.RefreshToken)
	// Asking again with the old token is RequireUser's refusal, and the
	// stored moment does not move.
	wantAccountDisabled(t, "asking again", w.askDeletion(phoneA.AccessToken, map[string]any{"password": deleteTestPassword}))
	if at := w.requestedAt(b.id); at == nil || !at.Equal(*asked) {
		t.Fatalf("asking again moved the ask from %v to %v", asked, at)
	}

	// Nothing is deleted yet. The email and the username stay taken, so a
	// log in inside the grace period finds the account as it was.
	if n := w.countFor(`select count(*) from entries where user_id = $1`, b.id); n != 1 {
		t.Fatalf("after the ask: %d posts kept, want the 1 there was", n)
	}
	taken := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email": email, "password": deleteTestPassword, "birthday": validBirthday, "name": "Someone Else", "username": "da_someone",
	})
	if taken.Code != http.StatusConflict || decodeBody(t, taken)["error"] != "email_taken" {
		t.Fatalf("sign-up with the email of an account being deleted: %d %s, want 409 email_taken", taken.Code, taken.Body.String())
	}
	avail := integrationDo(t, w.engine, http.MethodGet, "/v1/usernames/available?username=da_bob", "", nil)
	if avail.Code != http.StatusOK || decodeBody(t, avail)["available"] != false {
		t.Fatalf("the username of an account being deleted: %d %s, want available false", avail.Code, avail.Body.String())
	}
}

// The day the answer names is written in the person's own stored zone, and
// in UTC for a person who has none. Kiritimati (UTC+14) and Pago Pago
// (UTC-11) are 25 hours apart, so one instant never has the same date in
// both: a label that ignored the stored zone could not pass both checks.
func TestDeleteAccountIntegration_TheDayIsWrittenInTheStoredZone(t *testing.T) {
	w := newDeleteWorld(t)
	load := func(name string) *time.Location {
		t.Helper()
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		return loc
	}
	for _, tc := range []struct{ username, zone, other string }{
		{"dz_kiri", "Pacific/Kiritimati", "Pacific/Pago_Pago"},
		{"dz_pago", "Pacific/Pago_Pago", "Pacific/Kiritimati"},
		{"dz_nozone", "", ""},
	} {
		s := integrationOnboardAccount(t, w.engine, tc.username+"@example.com", deleteTestPassword, "Zone Person", tc.username)
		loc := time.UTC
		if tc.zone != "" {
			if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/time-zone", s.AccessToken, map[string]any{"time_zone": tc.zone}); rec.Code != http.StatusOK {
				t.Fatalf("%s: set the zone %s: %d %s", tc.username, tc.zone, rec.Code, rec.Body.String())
			}
			loc = load(tc.zone)
		}
		deleteAfter, label := w.askToDelete(s.AccessToken)
		if want := deleteAfter.In(loc).Format("Monday, January 2"); label != want {
			t.Fatalf("%s (zone %q): delete_on_label %q, want %q (delete_after %s)", tc.username, tc.zone, label, want, deleteAfter.Format(time.RFC3339))
		}
		if tc.other != "" {
			if wrong := deleteAfter.In(load(tc.other)).Format("Monday, January 2"); label == wrong {
				t.Fatalf("%s (zone %q): delete_on_label %q is the day in %s", tc.username, tc.zone, label, tc.other)
			}
		}
	}
}

// A wrong or missing password is a field error on the password box, never a
// 401 (which would sign the phone out), and changes nothing: no ask, every
// session live, the app still signed in. The route allows 5 tries a minute
// per person, counting the refused ones: the sixth is refused before the
// password is even read, the right password included, and still changes
// nothing. Another person asks in the same minute without trouble.
func TestDeleteAccountIntegration_WrongOrMissingPassword(t *testing.T) {
	w := newDeleteWorld(t)
	b, _ := w.account("dw_bob", "Bob Brown")
	nothingChanged := func(after string) {
		t.Helper()
		if at := w.requestedAt(b.id); at != nil {
			t.Fatalf("after %s: deletion_requested_at is %v, want unset", after, at)
		}
		if n := w.countFor(deleteLiveSessionsQ, b.id); n != 1 {
			t.Fatalf("after %s: %d live sessions, want sign-up's 1", after, n)
		}
		w.ok(b, http.MethodGet, "/v1/me", nil)
	}
	wantField := func(what string, rec *httptest.ResponseRecorder, reason string) {
		t.Helper()
		body := decodeBody(t, rec)
		if rec.Code != http.StatusBadRequest || body["error"] != "validation_failed" || body["field"] != "password" || body["reason"] != reason {
			t.Fatalf("%s: %d %s, want 400 validation_failed, field password, reason %s", what, rec.Code, rec.Body.String(), reason)
		}
	}

	wantField("a wrong password", w.askDeletion(b.token, map[string]any{"password": "Wrong-password-1!"}), "incorrect")
	nothingChanged("a wrong password")
	wantField("no password", w.askDeletion(b.token, map[string]any{}), "required")
	wantField("an empty password", w.askDeletion(b.token, map[string]any{"password": ""}), "required")
	nothingChanged("no password")
	for i := 4; i <= 5; i++ {
		wantField(fmt.Sprintf("try %d, a wrong password", i), w.askDeletion(b.token, map[string]any{"password": "Wrong-password-1!"}), "incorrect")
	}

	// The sixth try in the minute, with the right password.
	rec := w.askDeletion(b.token, map[string]any{"password": deleteTestPassword})
	if body := decodeBody(t, rec); rec.Code != http.StatusTooManyRequests || body["error"] != "rate_limited" {
		t.Fatalf("the sixth try in a minute: %d %s, want 429 rate_limited", rec.Code, rec.Body.String())
	}
	nothingChanged("the sixth try")

	// The limit is per person.
	c, _ := w.account("dw_cara", "Cara Cole")
	w.askToDelete(c.token)
	if w.requestedAt(c.id) == nil {
		t.Fatal("another person's ask in the same minute was not stored")
	}
}

// B asks for the account to be deleted, and from that moment B is gone for
// everyone else, as if an admin had disabled the account, everywhere another
// person could meet B: search, the username lookup, B's page and B's post,
// the Board (B's card, and the count and the names on the locked card), the
// comments and reactions on A's post (C's reply under B's comment goes with
// it), the comment counts on the card and in A's own posts list, the bell, a
// follow request B sent, Find your friends, the follower and following
// numbers and lists (A's own, and A's page as C sees it), and pushes: no
// phone of B's is pushed to, not even one with a live session. Every surface
// is first checked with B on it, so a check that passes afterwards does not
// pass because the surface was empty. Where B was, what is left is what a
// miss gets, byte for byte. B's own data is hidden, not deleted.
func TestDeleteAccountIntegration_GoneOnEverySurface(t *testing.T) {
	w := newDeleteWorld(t)
	a, _ := w.account("dg_alice", "Alice Ames")
	b, _ := w.account("dg_bob", "Bob Brown")
	c, _ := w.account("dg_cara", "Cara Cole")

	// A and B follow each other, and so do A and C; B asks to follow C, who
	// has not answered. B's phone saves its push token.
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	w.followAccepted(a, c)
	w.followAccepted(c, a)
	if rel := w.follow(b, c); rel != "requested" {
		t.Fatalf("B asks to follow C: %s, want requested", rel)
	}
	const bobsPhone = "ExponentPushToken[goneBob]"
	w.saveToken(b.token, bobsPhone)

	// The first evening all three post. B and C react to A's post, and the
	// three comment on it a minute apart: A, then C, then B, C replying under
	// B's comment, and last B replying to A's comment.
	const day1, day2 = "2026-10-08", "2026-10-09"
	aPost := w.postAt(a, day1, 20, 30)
	bPost := w.postAt(b, day1, 20, 31)
	w.postAt(c, day1, 20, 32)
	w.ok(b, http.MethodPost, "/v1/posts/"+aPost+"/reaction", map[string]any{"reaction": "fire"})
	w.clk.Advance(time.Minute)
	w.ok(c, http.MethodPost, "/v1/posts/"+aPost+"/reaction", map[string]any{"reaction": "love"})
	aTop := w.reply(a, aPost, "My pick", "")["id"].(string)
	cTop := w.reply(c, aPost, "Nice one", "")["id"].(string)
	bTop := w.reply(b, aPost, "Great pick", "")["id"].(string)
	cUnderB := w.reply(c, aPost, "Agreed", bTop)["id"].(string)
	bReply := w.reply(b, aPost, "Thanks for posting", aTop)["id"].(string)
	// The second evening B and C post and A has not: A's Board is locked on
	// it, and still lists the first evening's cards.
	w.postAt(b, day2, 20, 31)
	w.postAt(c, day2, 20, 32)
	w.at(day2, 21, 0)

	// What a miss gets, to compare the gone person with.
	missLookup := integrationDo(t, w.engine, http.MethodGet, "/v1/people/lookup?username=dg_nobody", a.token, nil)
	missPage := integrationDo(t, w.engine, http.MethodGet, "/v1/people/"+uuid.NewString(), a.token, nil)
	missPost := integrationDo(t, w.engine, http.MethodGet, "/v1/posts/"+uuid.NewString(), a.token, nil)

	lockedBoard := func() (cards []string, posted float64, names []string) {
		t.Helper()
		got := w.board(a, "")
		if got["state"] != "locked" {
			t.Fatalf("A's Board on the second evening: %v, want locked", got)
		}
		for _, n := range dig(t, got, "locked", "names").([]any) {
			names = append(names, n.(string))
		}
		return cardIDs(t, got), dig(t, got, "locked", "posted_count").(float64), names
	}
	postPage := func() (comments []string, count float64, reacted []string) {
		t.Helper()
		got := w.ok(a, http.MethodGet, "/v1/posts/"+aPost, nil)
		for _, v := range got["comments"].([]any) {
			comments = append(comments, v.(map[string]any)["id"].(string))
		}
		for _, v := range got["received"].([]any) {
			reacted = append(reacted, dig(t, v, "person", "id").(string))
		}
		sort.Strings(reacted)
		return comments, dig(t, got, "card", "comment_count").(float64), reacted
	}
	postsList := func() (count float64, latestBy string) {
		t.Helper()
		items := w.ok(a, http.MethodGet, "/v1/diary", nil)["items"].([]any)
		row := items[0].(map[string]any)
		if dig(t, row, "entry", "id") != aPost {
			t.Fatalf("A's posts list starts with %v, want A's post", row)
		}
		s := row["social"].(map[string]any)
		latest := s["latest_comments"].([]any)
		if len(latest) != 1 {
			t.Fatalf("A's posts list: latest comments %v, want the newest one", latest)
		}
		return s["comment_count"].(float64), dig(t, latest[0], "author", "id").(string)
	}
	bell := func(p feedPerson) (kindsByB, ids []string) {
		t.Helper()
		for _, v := range w.notes(p)["items"].([]any) {
			item := v.(map[string]any)
			ids = append(ids, item["id"].(string))
			if actor, ok := item["actor"].(map[string]any); ok && actor["id"] == b.id {
				kindsByB = append(kindsByB, item["kind"].(string))
			}
		}
		sort.Strings(kindsByB)
		return kindsByB, ids
	}
	suggestedTo := func(p feedPerson) []string {
		t.Helper()
		out := []string{}
		for _, s := range suggested(t, w.feedWorld, p) {
			out = append(out, dig(t, s, "person", "username").(string))
		}
		return out
	}
	summaryOf := func(p feedPerson) (followers, following float64) {
		t.Helper()
		got := w.ok(p, http.MethodGet, "/v1/me/summary", nil)
		return got["followers"].(float64), got["following"].(float64)
	}
	pageAsC := func() (followers, following float64) {
		t.Helper()
		got := w.ok(c, http.MethodGet, "/v1/people/"+a.id, nil)
		return dig(t, got, "stats", "followers").(float64), dig(t, got, "stats", "following").(float64)
	}
	peopleAsC := func(path string) []string {
		t.Helper()
		return sortedCopy(usernames(t, w.ok(c, http.MethodGet, path, nil)["people"])...)
	}
	both := []string{"dg_bob", "dg_cara"}

	// Before the ask: B is on every surface.
	if got := w.searchNames(a, "dg_b"); !sameList(got, []string{"dg_bob"}) {
		t.Fatalf("before: A's search for dg_b finds %v, want [dg_bob]", got)
	}
	if got := w.lookup(a, "dg_bob", http.StatusOK); dig(t, got, "person", "id") != b.id || got["relation"] != "following" {
		t.Fatalf("before: A's lookup of dg_bob: %v", got)
	}
	if got := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil); dig(t, got, "person", "id") != b.id {
		t.Fatalf("before: B's page as A: %v", got)
	}
	w.ok(a, http.MethodGet, "/v1/posts/"+bPost, nil)
	if cards, posted, names := lockedBoard(); !has(cards, bPost) || posted != 2 || !sameList(names, []string{"Bob", "Cara"}) {
		t.Fatalf("before: A's Board: cards %v (want B's %s among them), locked posted_count %v (want 2), names %v (want [Bob Cara])", cards, bPost, posted, names)
	}
	if comments, count, reacted := postPage(); !sameList(comments, []string{aTop, bReply, cTop, bTop, cUnderB}) || count != 5 ||
		!sameList(reacted, sortedCopy(b.id, c.id)) {
		t.Fatalf("before: A's post page: comments %v (want %v), comment_count %v (want 5), reactions from %v (want B and C)",
			comments, []string{aTop, bReply, cTop, bTop, cUnderB}, count, reacted)
	}
	if n := w.boardCount(a, aPost); n != 5 {
		t.Fatalf("before: the comment count on A's Board card: %v, want 5", n)
	}
	if count, latestBy := postsList(); count != 5 || latestBy != b.id {
		t.Fatalf("before: A's posts list: comment_count %v (want 5), newest comment by %s (want B, %s)", count, latestBy, b.id)
	}
	if byB, ids := bell(a); !sameList(byB, []string{"comment", "follow", "follow_accepted", "reply"}) || !has(ids, "comment:"+cUnderB) {
		t.Fatalf("before: A's bell: items by B %v (want comment, follow, follow_accepted, reply), ids %v (want comment:%s among them)", byB, ids, cUnderB)
	}
	if byB, _ := bell(c); !sameList(byB, []string{"follow_request"}) {
		t.Fatalf("before: C's bell: items by B %v, want [follow_request]", byB)
	}
	if got := usernames(t, w.lists(c)["incoming"]); !sameList(got, []string{"dg_bob"}) {
		t.Fatalf("before: C's follow requests: %v, want [dg_bob]", got)
	}
	if got := suggestedTo(c); !has(got, "dg_bob") {
		t.Fatalf("before: C's suggestions %v, want dg_bob among them", got)
	}
	if followers, following := summaryOf(a); followers != 2 || following != 2 {
		t.Fatalf("before: A's numbers: %v followers, %v following, want 2 and 2", followers, following)
	}
	if l := w.lists(a); !sameList(sortedCopy(usernames(t, l["followers"])...), both) || !sameList(sortedCopy(usernames(t, l["following"])...), both) {
		t.Fatalf("before: A's lists: %v, want followers and following %v", l, both)
	}
	if followers, following := pageAsC(); followers != 2 || following != 2 {
		t.Fatalf("before: A's page as C: %v followers, %v following, want 2 and 2", followers, following)
	}
	if got := peopleAsC("/v1/people/" + a.id + "/followers"); !sameList(got, both) {
		t.Fatalf("before: A's followers as C: %v, want %v", got, both)
	}
	if got := peopleAsC("/v1/people/" + a.id + "/following"); !sameList(got, both) {
		t.Fatalf("before: A's following as C: %v, want %v", got, both)
	}
	w.wantPushedTo("before: B's phone", b.id, bobsPhone)

	w.askToDelete(b.token)

	// After the ask: B is on none of them.
	if got := w.searchNames(a, "dg_b"); len(got) != 0 {
		t.Fatalf("after: A's search for dg_b finds %v, want nobody", got)
	}
	wantSameAsMiss(t, "after: A's lookup of dg_bob",
		integrationDo(t, w.engine, http.MethodGet, "/v1/people/lookup?username=dg_bob", a.token, nil), missLookup)
	wantSameAsMiss(t, "after: B's page as A", integrationDo(t, w.engine, http.MethodGet, "/v1/people/"+b.id, a.token, nil), missPage)
	wantSameAsMiss(t, "after: B's post as A", integrationDo(t, w.engine, http.MethodGet, "/v1/posts/"+bPost, a.token, nil), missPost)
	if cards, posted, names := lockedBoard(); has(cards, bPost) || posted != 1 || !sameList(names, []string{"Cara"}) {
		t.Fatalf("after: A's Board: cards %v (want no %s), locked posted_count %v (want 1), names %v (want [Cara])", cards, bPost, posted, names)
	}
	if comments, count, reacted := postPage(); !sameList(comments, []string{aTop, cTop}) || count != 2 || !sameList(reacted, []string{c.id}) {
		t.Fatalf("after: A's post page: comments %v (want %v: B's two and C's reply under B's are gone), comment_count %v (want 2), reactions from %v (want C only)",
			comments, []string{aTop, cTop}, count, reacted)
	}
	if n := w.boardCount(a, aPost); n != 2 {
		t.Fatalf("after: the comment count on A's Board card: %v, want 2", n)
	}
	if count, latestBy := postsList(); count != 2 || latestBy != c.id {
		t.Fatalf("after: A's posts list: comment_count %v (want 2), newest comment by %s (want C, %s)", count, latestBy, c.id)
	}
	if byB, ids := bell(a); len(byB) != 0 || has(ids, "comment:"+cUnderB) || !has(ids, "comment:"+cTop) {
		t.Fatalf("after: A's bell: items by B %v (want none), ids %v (want no comment:%s, and comment:%s still there)", byB, ids, cUnderB, cTop)
	}
	if byB, _ := bell(c); len(byB) != 0 {
		t.Fatalf("after: C's bell: items by B %v, want none", byB)
	}
	if got := usernames(t, w.lists(c)["incoming"]); len(got) != 0 {
		t.Fatalf("after: C's follow requests: %v, want none", got)
	}
	if got := suggestedTo(c); has(got, "dg_bob") {
		t.Fatalf("after: C's suggestions %v still have dg_bob", got)
	}
	if followers, following := summaryOf(a); followers != 1 || following != 1 {
		t.Fatalf("after: A's numbers: %v followers, %v following, want 1 and 1", followers, following)
	}
	if l := w.lists(a); !sameList(usernames(t, l["followers"]), []string{"dg_cara"}) || !sameList(usernames(t, l["following"]), []string{"dg_cara"}) {
		t.Fatalf("after: A's lists: %v, want followers and following [dg_cara]", l)
	}
	if followers, following := pageAsC(); followers != 1 || following != 1 {
		t.Fatalf("after: A's page as C: %v followers, %v following, want 1 and 1", followers, following)
	}
	if got := peopleAsC("/v1/people/" + a.id + "/followers"); !sameList(got, []string{"dg_cara"}) {
		t.Fatalf("after: A's followers as C: %v, want [dg_cara]", got)
	}
	if got := peopleAsC("/v1/people/" + a.id + "/following"); !sameList(got, []string{"dg_cara"}) {
		t.Fatalf("after: A's following as C: %v, want [dg_cara]", got)
	}

	// Pushes. The ask deleted B's push token. A phone of B's with a live
	// session, stored straight into the database as a sign-in that raced the
	// ask might leave it, is still not pushed to, while the very same rows
	// for C are.
	if n := w.countFor(deletePushTokensQ, b.id); n != 0 {
		t.Fatalf("after: %d push tokens of B kept, want 0", n)
	}
	w.wantPushedTo("after: B's phone", b.id)
	bLive := w.liveSessionBySQL(b.id, "ExponentPushToken[ghostBob]")
	w.liveSessionBySQL(c.id, "ExponentPushToken[ghostCara]")
	w.wantPushedTo("after: B, with a live session stored after the ask", b.id)
	w.wantPushedTo("C, with the same rows", c.id, "ExponentPushToken[ghostCara]")
	// That live session cannot be refreshed either: 403 account_disabled.
	rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": bLive})
	if body := decodeBody(t, rec); rec.Code != http.StatusForbidden || body["error"] != string(CodeAccountDisabled) {
		t.Fatalf("refresh with a live session of an account being deleted: %d %s, want 403 account_disabled", rec.Code, rec.Body.String())
	}

	// B's data is hidden, not deleted: the purge deletes it once the grace
	// period is over.
	if n := w.countFor(`select count(*) from entries where user_id = $1`, b.id); n != 2 {
		t.Fatalf("after: %d posts of B kept, want 2", n)
	}
	if n := w.countFor(`select count(*) from post_comments where user_id = $1`, b.id); n != 2 {
		t.Fatalf("after: %d comments of B kept, want 2", n)
	}
}

// The admin page shows that a reported person asked for their account to be
// deleted and does not list them with the disabled accounts: they did it
// themselves, and logging back in keeps the account. An admin who asks is no
// admin from that moment: the admin session ends at its next page and the
// admin page's log in refuses the right password, until a log in to the app
// keeps the account.
func TestDeleteAccountIntegration_AdminPage(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	const bossEmail = "dx_boss@example.com"
	boss := adminFirst(w, pool, integrationOptions{clock: clk, adminEmails: []string{bossEmail}}, "dx_boss")
	b := w.person("dx_bob", "Bob Brown")
	d := w.person("dx_dan", "Dan Dorn")
	// Both are reported, so the admin may open them.
	r := w.person("dx_rae", "Rae Reporter")
	for _, p := range []feedPerson{b, d} {
		w.ok(r, http.MethodPost, "/v1/people/"+p.id+"/report", map[string]any{"reason": "spam"})
	}
	cookie := adminLogin(t, w, bossEmail, adminTestPassword)
	if cookie == "" {
		t.Fatal("the admin could not log in")
	}
	ask := func(p feedPerson) {
		t.Helper()
		if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/me/delete", p.token, map[string]any{"password": adminTestPassword}); rec.Code != http.StatusOK {
			t.Fatalf("%s asks for the account to be deleted: %d %s", p.name, rec.Code, rec.Body.String())
		}
	}

	// B asks; the admin disables D.
	ask(b)
	csrf := csrfOf(t, adminDo(t, w, http.MethodGet, "/admin/people/"+d.id, cookie, nil, "").Body.String())
	if rec := adminDo(t, w, http.MethodPost, "/admin/people/"+d.id+"/disable", cookie, url.Values{"csrf": {csrf}}, ""); !strings.Contains(rec.Header().Get("Location"), "n=disabled") {
		t.Fatalf("disable D: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// B's page says the deletion was requested and not that B is disabled;
	// D's page the other way round.
	bPage := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "")
	if body := bPage.Body.String(); bPage.Code != http.StatusOK || !strings.Contains(body, "<strong>Deletion requested</strong>") ||
		strings.Contains(body, "<strong>Disabled</strong>") {
		t.Fatalf("B's admin page: %d %s, want \"Deletion requested\" and not \"Disabled\"", bPage.Code, body)
	}
	if body := adminDo(t, w, http.MethodGet, "/admin/people/"+d.id, cookie, nil, "").Body.String(); !strings.Contains(body, "<strong>Disabled</strong>") ||
		strings.Contains(body, "Deletion requested") {
		t.Fatalf("D's admin page: %s, want \"Disabled\" and not \"Deletion requested\"", body)
	}
	// The list: D under Disabled accounts, B not.
	list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "")
	_, disabled, found := strings.Cut(list.Body.String(), "<h2>Disabled accounts</h2>")
	if list.Code != http.StatusOK || !found || !strings.Contains(disabled, "@dx_dan") || strings.Contains(disabled, "@dx_bob") {
		t.Fatalf("the admin list: %d %s, want @dx_dan under Disabled accounts and no @dx_bob", list.Code, list.Body.String())
	}

	// The admin asks too: the session ends at the next page, and its cookie
	// is cleared.
	ask(boss)
	rec := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n=expired" {
		t.Fatalf("the admin page after the admin asked: %d %q, want 303 to /admin/login?n=expired", rec.Code, rec.Header().Get("Location"))
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookie && c.Value == "" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("the admin cookie was not cleared: %v", rec.Result().Cookies())
	}
	if loc, got := adminLoginFrom(t, w, "198.51.100.91:1", bossEmail, adminTestPassword); got != "" || !strings.Contains(loc, "n=badlogin") {
		t.Fatalf("the admin page's log in for an admin being deleted: %q (cookie given: %v), want n=badlogin and no cookie", loc, got != "")
	}

	// Logging back in to the app keeps the account, and the admin is an
	// admin again.
	login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": bossEmail, "password": adminTestPassword})
	if login.Code != http.StatusOK || decodeBody(t, login)["restored"] != true {
		t.Fatalf("the admin logs back in to the app: %d %s, want 200 with restored true", login.Code, login.Body.String())
	}
	if _, got := adminLoginFrom(t, w, "198.51.100.91:2", bossEmail, adminTestPassword); got == "" {
		t.Fatal("the admin page's log in refused an admin who kept the account")
	}
}

// Inside the grace period the right password keeps the account, as on
// Instagram: log in, by the email and (on a second ask) by the username,
// cancels the deletion, says so once with "restored": true, and the account
// is back for everyone. A wrong password restores nothing. The next log in
// is an ordinary one with no restored key at all, and the sessions the ask
// ended stay ended.
func TestDeleteAccountIntegration_LogInInsideGraceRestores(t *testing.T) {
	w := newDeleteWorld(t)
	a, _ := w.account("dl_alice", "Alice Ames")
	b, first := w.account("dl_bob", "Bob Brown")

	for _, by := range []string{"dl_bob@example.com", "dl_bob"} {
		w.askToDelete(b.token)
		w.backdateAsk(b.id, deletionInsideGrace)
		// While it waits, B is gone for A.
		w.lookup(a, "dl_bob", http.StatusNotFound)

		// A wrong password: the usual refusal, and the ask stays.
		assertInvalidCredentials(t, w.logIn(by, "Wrong-password-1!"))
		if w.requestedAt(b.id) == nil {
			t.Fatalf("log in by %s with a wrong password cancelled the ask", by)
		}

		rec := w.logIn(by, deleteTestPassword)
		if rec.Code != http.StatusOK {
			t.Fatalf("log in by %s inside the grace period: %d %s, want 200", by, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if body["restored"] != true || dig(t, body, "user", "id") != b.id {
			t.Fatalf("log in by %s inside the grace period: %s, want B's session with restored true", by, rec.Body.String())
		}
		if at := w.requestedAt(b.id); at != nil {
			t.Fatalf("log in by %s inside the grace period left deletion_requested_at at %v", by, at)
		}
		b.token = body["access_token"].(string)
		w.ok(b, http.MethodGet, "/v1/me", nil)
		if got := w.lookup(a, "dl_bob", http.StatusOK); dig(t, got, "person", "id") != b.id {
			t.Fatalf("A's lookup of dl_bob after the restore: %v", got)
		}

		// The next log in is an ordinary one.
		again := w.logIn(by, deleteTestPassword)
		if again.Code != http.StatusOK {
			t.Fatalf("log in by %s after the restore: %d %s", by, again.Code, again.Body.String())
		}
		if v, present := decodeBody(t, again)["restored"]; present {
			t.Fatalf("log in by %s after the restore carries restored: %v, want no such key", by, v)
		}
	}
	w.refreshRefused("the refresh token from before the first ask", first.RefreshToken)
}

// Inside the grace period a Forgot password reset keeps the account too: the
// code is still sent, verify passes it and cancels nothing, and the reset
// changes the password and cancels the deletion in one transaction and says
// so once with "restored": true. The new password logs in, the old one does
// not, and a later reset is an ordinary one with no restored key.
func TestDeleteAccountIntegration_ResetInsideGraceRestores(t *testing.T) {
	w := newDeleteWorld(t)
	b, _ := w.account("dr_bob", "Bob Brown")
	const email, fresh = "dr_bob@example.com", "Brand-new-password-2!"
	w.askToDelete(b.token)
	w.backdateAsk(b.id, deletionInsideGrace)
	wantAccountDisabled(t, "the old access token while the ask waits", integrationDo(t, w.engine, http.MethodGet, "/v1/me", b.token, nil))

	w.askForgot(email)
	if n, sent := w.countFor(deleteResetCodesQ, b.id), w.box.count(); n != 1 || sent != 1 {
		t.Fatalf("forgot password inside the grace period: %d codes stored and %d emails, want 1 and 1", n, sent)
	}
	// Verify checks the code and cancels nothing: only the reset's change does.
	code := w.box.lastCode(t, email)
	if rec := w.verifyCode(email, code); rec.Code != http.StatusOK {
		t.Fatalf("verify inside the grace period: %d %s, want 200", rec.Code, rec.Body.String())
	}
	if w.requestedAt(b.id) == nil {
		t.Fatal("verify cancelled the deletion")
	}
	rec := w.resetPassword(email, code, fresh)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset inside the grace period: %d %s, want 200", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["restored"] != true || dig(t, body, "user", "id") != b.id {
		t.Fatalf("reset inside the grace period: %s, want B's session with restored true", rec.Body.String())
	}
	if at := w.requestedAt(b.id); at != nil {
		t.Fatalf("the reset left deletion_requested_at at %v", at)
	}
	b.token = body["access_token"].(string)
	w.ok(b, http.MethodGet, "/v1/me", nil)

	// The new password logs in, an ordinary log in; the old one does not.
	login := w.logIn(email, fresh)
	if login.Code != http.StatusOK {
		t.Fatalf("log in with the new password: %d %s", login.Code, login.Body.String())
	}
	if v, present := decodeBody(t, login)["restored"]; present {
		t.Fatalf("log in after the reset carries restored: %v, want no such key", v)
	}
	assertInvalidCredentials(t, w.logIn(email, deleteTestPassword))

	// A later reset is an ordinary one.
	w.askForgot(email)
	again := w.resetPassword(email, w.box.lastCode(t, email), "Third-password-3!")
	if again.Code != http.StatusOK {
		t.Fatalf("a later reset: %d %s", again.Code, again.Body.String())
	}
	if v, present := decodeBody(t, again)["restored"]; present {
		t.Fatalf("a later reset carries restored: %v, want no such key", v)
	}
}

// Past the grace period nothing brings the account back, and nothing tells a
// stranger it is still there: the right password, by the email or the
// username, gets byte for byte what a name with no account gets, Forgot
// password stores and sends no code, a reset gets the wrong-code answer an
// email with no account gets, the old tokens stay refused, and the cancel
// itself refuses. A code sent inside the grace period stops working once it
// is over, at verify as at reset.
func TestDeleteAccountIntegration_NoRestorePastGrace(t *testing.T) {
	w := newDeleteWorld(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(w.pool)
	b, first := w.account("dp_bob", "Bob Brown")
	const email, fresh, nobody = "dp_bob@example.com", "Brand-new-password-2!", "dp_nobody@example.com"
	hash := w.passwordHashOf(b.id)
	w.askToDelete(b.token)
	w.backdateAsk(b.id, deletionPastGrace)
	asked := w.requestedAt(b.id)
	stillAsked := func(after string) {
		t.Helper()
		if at := w.requestedAt(b.id); at == nil || !at.Equal(*asked) {
			t.Fatalf("after %s: deletion_requested_at is %v, want it unchanged at %v", after, at, asked)
		}
	}

	for login, unknown := range map[string]string{email: nobody, "dp_bob": "dp_nobody"} {
		got, miss := w.logIn(login, deleteTestPassword), w.logIn(unknown, deleteTestPassword)
		assertInvalidCredentials(t, got)
		if got.Code != miss.Code || got.Body.String() != miss.Body.String() {
			t.Fatalf("log in by %s past the grace period: %d %s, want what %s gets: %d %s",
				login, got.Code, got.Body.String(), unknown, miss.Code, miss.Body.String())
		}
	}
	stillAsked("log in with the right password")

	w.askForgot(email)
	if n, sent := w.countFor(deleteResetCodesQ, b.id), w.box.count(); n != 0 || sent != 0 {
		t.Fatalf("forgot password past the grace period: %d codes stored and %d emails, want none", n, sent)
	}
	got, miss := w.resetPassword(email, "123456", fresh), w.resetPassword(nobody, "123456", fresh)
	if got.Code != http.StatusBadRequest || decodeBody(t, got)["reason"] != reasonWrongCode || got.Body.String() != miss.Body.String() {
		t.Fatalf("a reset past the grace period: %d %s, want the wrong-code answer an email with no account gets: %d %s",
			got.Code, got.Body.String(), miss.Code, miss.Body.String())
	}
	if w.passwordHashOf(b.id) != hash {
		t.Fatal("a reset past the grace period changed the password")
	}

	wantAccountDisabled(t, "the old access token past the grace period", integrationDo(t, w.engine, http.MethodGet, "/v1/me", b.token, nil))
	w.refreshRefused("the old refresh token past the grace period", first.RefreshToken)
	restored, err := users.CancelDeletion(ctx, uuid.MustParse(b.id))
	if err != nil || restored {
		t.Fatalf("CancelDeletion past the grace period: restored %v, err %v, want false and no error", restored, err)
	}
	stillAsked("CancelDeletion")

	// A code sent inside the grace period stops working once it is over.
	d, _ := w.account("dp_dan", "Dan Dorn")
	const danEmail = "dp_dan@example.com"
	danHash := w.passwordHashOf(d.id)
	w.askToDelete(d.token)
	w.backdateAsk(d.id, deletionInsideGrace)
	w.askForgot(danEmail)
	code := w.box.lastCode(t, danEmail)
	w.backdateAsk(d.id, deletionPastGrace)
	got, miss = w.verifyCode(danEmail, code), w.verifyCode(nobody, code)
	if got.Code != http.StatusBadRequest || decodeBody(t, got)["reason"] != reasonWrongCode || got.Body.String() != miss.Body.String() {
		t.Fatalf("verify with a code from inside the grace period, once it is over: %d %s, want the wrong-code answer: %d %s",
			got.Code, got.Body.String(), miss.Code, miss.Body.String())
	}
	got, miss = w.resetPassword(danEmail, code, fresh), w.resetPassword(nobody, code, fresh)
	if got.Code != http.StatusBadRequest || got.Body.String() != miss.Body.String() {
		t.Fatalf("a reset with a code from inside the grace period, once it is over: %d %s, want the wrong-code answer: %d %s",
			got.Code, got.Body.String(), miss.Code, miss.Body.String())
	}
	if w.passwordHashOf(d.id) != danHash || w.requestedAt(d.id) == nil {
		t.Fatal("a reset with a code from inside the grace period, once it is over, changed the password or cancelled the ask")
	}
}
