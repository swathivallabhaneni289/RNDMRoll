package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/push"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// pushRecorder is a push.Sender that keeps every message.
type pushRecorder struct {
	mu   sync.Mutex
	msgs []push.Message
}

func (r *pushRecorder) Send(_ context.Context, msgs []push.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msgs...)
}

// pushWorld is a feedWorld whose services push to a recorder.
type pushWorld struct {
	*feedWorld
	pool     *pgxpool.Pool
	rec      *pushRecorder
	notifier *push.Notifier
	names    map[string]string // id -> first name, for readable failures
}

func newPushWorld(t *testing.T) *pushWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	pw := &pushWorld{pool: pool, rec: &pushRecorder{}, names: map[string]string{}}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk,
		wire: func(socialSvc *social.Service, boardSvc *board.Service, deps *Deps) {
			repo := postgres.NewPushRepo(pool)
			pw.notifier = push.NewNotifier(repo, socialSvc, pw.rec, slog.New(slog.NewTextHandler(io.Discard, nil)))
			boardSvc.SetCommentNotifier(pw.notifier)
			socialSvc.SetFollowNotifier(pw.notifier)
			deps.PushTokens = NewPushTokenHandler(repo)
		}}).Engine()
	pw.feedWorld = &feedWorld{t: t, engine: engine, clk: clk}
	return pw
}

func (w *pushWorld) member(username, name string) feedPerson {
	p := w.person(username, name)
	w.names[p.id] = username
	return p
}

// take waits for the background sends and answers "to: body" lines since
// the last take, sorted.
func (w *pushWorld) take() []string {
	w.notifier.Wait()
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	out := []string{}
	for _, m := range w.rec.msgs {
		if m.Title != "RNDMRoll" {
			w.t.Errorf("title %q", m.Title)
		}
		out = append(out, w.names[m.UserID.String()]+": "+m.Body)
	}
	w.rec.msgs = nil
	sort.Strings(out)
	return out
}

func (w *pushWorld) expect(what string, want ...string) {
	w.t.Helper()
	got := w.take()
	sort.Strings(want)
	if !sameList(got, want) {
		w.t.Fatalf("%s: pushes %q, want %q", what, got, want)
	}
}

// lastData is the data of the only message waiting.
func (w *pushWorld) lastData() map[string]string {
	w.t.Helper()
	w.notifier.Wait()
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	if len(w.rec.msgs) != 1 {
		w.t.Fatalf("want one message, got %d", len(w.rec.msgs))
	}
	return w.rec.msgs[0].Data
}

func TestPushIntegration_TokenRoutes(t *testing.T) {
	w := newPushWorld(t)
	a := w.member("pt_alice", "Alice Ames")
	b := w.member("pt_bob", "Bob Brown")
	const tok = "ExponentPushToken[abcDEF123]"
	owner := func() string {
		var id string
		if err := w.pool.QueryRow(context.Background(), `select user_id::text from push_tokens where token = $1`, tok).Scan(&id); err != nil {
			return ""
		}
		return id
	}

	w.call(a, http.MethodPost, "/v1/me/push-tokens", map[string]any{"token": "nope", "platform": "ios"}, http.StatusBadRequest)
	w.call(a, http.MethodPost, "/v1/me/push-tokens", map[string]any{"token": tok, "platform": "web"}, http.StatusBadRequest)
	w.call(a, http.MethodPost, "/v1/me/push-tokens", map[string]any{"token": tok, "platform": "ios"}, http.StatusNoContent)
	w.call(a, http.MethodPost, "/v1/me/push-tokens", map[string]any{"token": tok, "platform": "ios"}, http.StatusNoContent)
	if owner() != a.id {
		t.Fatalf("token owner %q, want alice", owner())
	}
	// The same phone signed in as Bob: the token moves.
	w.call(b, http.MethodPost, "/v1/me/push-tokens", map[string]any{"token": tok, "platform": "ios"}, http.StatusNoContent)
	if owner() != b.id {
		t.Fatalf("token owner %q, want bob", owner())
	}
	// Alice cannot delete Bob's token; Bob can.
	w.call(a, http.MethodDelete, "/v1/me/push-tokens", map[string]any{"token": tok}, http.StatusNoContent)
	if owner() != b.id {
		t.Fatal("alice deleted bob's token")
	}
	w.call(b, http.MethodDelete, "/v1/me/push-tokens", map[string]any{"token": tok}, http.StatusNoContent)
	if owner() != "" {
		t.Fatal("token still kept after delete")
	}
	rec := integrationDo(t, w.engine, http.MethodPost, "/v1/me/push-tokens", "", map[string]any{"token": tok, "platform": "ios"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", rec.Code)
	}
}

func TestPushIntegration_FourEventsSettingsBlocksAndSelf(t *testing.T) {
	w := newPushWorld(t)
	alice := w.member("pe_alice", "Alice Ames")
	bob := w.member("pe_bob", "Bob Brown")
	cara := w.member("pe_cara", "Cara Cole")

	// A follow request, then a repeat: one push.
	if rel := w.follow(bob, alice); rel != "requested" {
		t.Fatalf("relation %s", rel)
	}
	if d := w.lastData(); d["kind"] != "follow_request" || d["actor_id"] != bob.id {
		t.Fatalf("request data %v", d)
	}
	w.expect("request", "pe_alice: Bob wants to follow you.")
	w.follow(bob, alice)
	w.expect("repeat request")
	// Accepted.
	var reqID string
	for _, r := range w.lists(alice)["incoming"].([]any) {
		reqID = r.(map[string]any)["request_id"].(string)
	}
	w.ok(alice, http.MethodPost, "/v1/follows/requests/"+reqID+"/accept", map[string]any{})
	if d := w.lastData(); d["kind"] != "follow_accepted" || d["actor_id"] != alice.id {
		t.Fatalf("accepted data %v", d)
	}
	w.expect("accepted", "pe_bob: Alice accepted your follow request.")

	w.followAccepted(cara, alice)
	w.expect("cara follows", "pe_alice: Cara wants to follow you.", "pe_cara: Alice accepted your follow request.")

	const day = "2026-10-08"
	entry := w.post(alice, day)
	w.post(bob, day)
	w.post(cara, day)
	w.expect("posts")

	// A comment on Alice's post.
	bobsComment := w.reply(bob, entry, "Love it", "")["id"].(string)
	if d := w.lastData(); d["kind"] != "comment" || d["entry_id"] != entry || d["own"] != "true" {
		t.Fatalf("comment data %v", d)
	}
	w.expect("comment", `pe_alice: Bob commented: "Love it"`)

	// Cara replies to Bob: Bob gets the reply, Alice the comment.
	w.reply(cara, entry, "Same", bobsComment)
	w.notifier.Wait()
	w.rec.mu.Lock()
	for _, m := range w.rec.msgs {
		if m.Data["kind"] == "reply" && m.Data["own"] != "false" {
			t.Errorf("reply on alice's post is not bob's own: %v", m.Data)
		}
	}
	w.rec.mu.Unlock()
	w.expect("reply", `pe_bob: Cara replied to your comment: "Same"`, `pe_alice: Cara commented: "Same"`)

	// Alice replies to Bob: Bob only, never Alice herself.
	w.reply(alice, entry, "Thanks", bobsComment)
	w.expect("owner reply", `pe_bob: Alice replied to your comment: "Thanks"`)

	// Alice comments on her own post, then Bob replies to it: a reply only.
	alicesComment := w.reply(alice, entry, "Mine", "")["id"].(string)
	w.expect("own comment")
	w.reply(bob, entry, "Yours", alicesComment)
	if d := w.lastData(); d["kind"] != "reply" || d["own"] != "true" {
		t.Fatalf("reply on own post data %v", d)
	}
	w.expect("reply to owner", `pe_alice: Bob replied to your comment: "Yours"`)

	// The comments switch off: no comment or reply pushes for Alice.
	w.ok(alice, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": false})
	w.reply(cara, entry, "Quiet", "")
	w.reply(cara, entry, "Quiet too", alicesComment)
	w.expect("switch off")
	w.ok(alice, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": true})

	// Alice blocks Bob: Cara's reply under Bob's comment reaches no one
	// (Alice has a block with its writer; Bob no longer sees the post).
	w.ok(alice, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": bob.id})
	w.reply(cara, entry, "Still here", bobsComment)
	w.expect("block")
}

// Follow requests sent at once notify once: the push goes only from the
// request that made the row, decided under the pair lock.
func TestPushIntegration_FollowsAtOnceNotifyOnce(t *testing.T) {
	w := newPushWorld(t)
	alice := w.member("pc_alice", "Alice Ames")
	bob := w.member("pc_bob", "Bob Brown")

	// Open the connections first, so the requests really run side by side.
	var conns []*pgxpool.Conn
	for range 8 {
		conn, err := w.pool.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, conn)
	}
	for _, conn := range conns {
		conn.Release()
	}
	reqs := make([]*http.Request, 8)
	for i := range reqs {
		reqs[i] = integrationRequest(t, http.MethodPost, "/v1/follows", bob.token, map[string]any{"user_id": alice.id})
	}
	for i, rec := range integrationAtOnce(w.engine, reqs) {
		if rec.Code != http.StatusOK {
			t.Fatalf("follow %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	w.expect("follows at once", "pc_alice: Bob wants to follow you.")
}

// pushedTo is what a push to the account id would go to: the tokens of its
// live sessions.
func (w *pushWorld) pushedTo(id string) []string {
	w.t.Helper()
	tokens, err := postgres.NewPushRepo(w.pool).Tokens(context.Background(), uuid.MustParse(id))
	if err != nil {
		w.t.Fatalf("Tokens: %v", err)
	}
	sort.Strings(tokens)
	return tokens
}

func (w *pushWorld) wantPushedTo(what, id string, want ...string) {
	w.t.Helper()
	sort.Strings(want)
	if got := w.pushedTo(id); !sameList(got, want) {
		w.t.Fatalf("%s: pushes go to %q, want %q", what, got, want)
	}
}

// saveToken sends the phone's token with accessToken.
func (w *pushWorld) saveToken(accessToken, token string) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/me/push-tokens", accessToken, map[string]any{"token": token, "platform": "ios"})
	if rec.Code != http.StatusNoContent {
		w.t.Fatalf("save the push token: %d %s", rec.Code, rec.Body.String())
	}
}

// session posts to an auth route that answers a session and returns it.
func (w *pushWorld) session(path string, token string, body map[string]any) integrationSession {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, path, token, body)
	if rec.Code != http.StatusOK {
		w.t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
	}
	got := decodeBody(w.t, rec)
	return integrationSession{AccessToken: got["access_token"].(string), RefreshToken: got["refresh_token"].(string)}
}

// signUp makes an account and returns its id and session.
func (w *pushWorld) signUp(username string) (string, integrationSession) {
	w.t.Helper()
	s := integrationOnboardAccount(w.t, w.engine, username+"@example.com", "Correct-horse-battery-staple1", "Push Person", username)
	rec := integrationDo(w.t, w.engine, http.MethodGet, "/v1/me", s.AccessToken, nil)
	if rec.Code != http.StatusOK {
		w.t.Fatalf("me: %d", rec.Code)
	}
	return decodeBody(w.t, rec)["id"].(string), s
}

// A phone's token is pushed to only while the session that saved it lasts: a
// refresh keeps it, also when the token is saved again with the access token
// from before the refresh; a log out ends it though the app's DELETE never
// came, and the leaving session cannot bring it back; signing in again does.
func TestPushIntegration_TokenFollowsTheSession(t *testing.T) {
	w := newPushWorld(t)
	id, first := w.signUp("ps_alice")
	const tok = "ExponentPushToken[sessionAlice]"

	w.saveToken(first.AccessToken, tok)
	w.wantPushedTo("saved", id, tok)

	refreshed := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": first.RefreshToken})
	w.wantPushedTo("after a refresh", id, tok)
	w.saveToken(first.AccessToken, tok) // the access token from before the refresh
	w.wantPushedTo("saved with the access token from before the refresh", id, tok)
	w.saveToken(refreshed.AccessToken, tok)
	w.wantPushedTo("saved after the refresh", id, tok)

	// Log out, and the DELETE never reaches the server.
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/logout", "", map[string]any{"refresh_token": refreshed.RefreshToken}); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	w.wantPushedTo("after log out", id)
	// The leaving session's access token still works for a few minutes, but
	// saving the token with it does not bring the pushes back.
	w.saveToken(refreshed.AccessToken, tok)
	w.wantPushedTo("saved by the session that logged out", id)

	again := w.session("/v1/auth/login", "", map[string]any{"login": "ps_alice", "password": "Correct-horse-battery-staple1"})
	w.saveToken(again.AccessToken, tok)
	w.wantPushedTo("signed in again", id, tok)
}

// An access token from before sessions were named (no sid) still works for
// everything; the token it saves is kept but pushed to only once the app saves
// it again with a newer access token.
func TestPushIntegration_AccessTokenWithoutASession(t *testing.T) {
	w := newPushWorld(t)
	id, s := w.signUp("ps_old")
	const tok = "ExponentPushToken[sessionOld]"
	old, err := auth.IssueAccessToken(uuid.MustParse(id), integrationJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if rec := integrationDo(t, w.engine, http.MethodGet, "/v1/me", old, nil); rec.Code != http.StatusOK {
		t.Fatalf("an old access token on /me: %d", rec.Code)
	}
	w.saveToken(old, tok)
	var kept int
	if err := w.pool.QueryRow(context.Background(), `select count(*) from push_tokens where token = $1 and refresh_token_id is null`, tok).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("the token saved with an old access token: %d rows with no session (err %v)", kept, err)
	}
	w.wantPushedTo("saved with an old access token", id)
	w.saveToken(s.AccessToken, tok)
	w.wantPushedTo("saved again with a newer access token", id, tok)
}

// Change password ends every session's pushes, the other phone's too, even
// though that phone's access token can still save its token for a few
// minutes. The phone that changed it gets its pushes back by saving its token
// with its new session. A reset does the same.
func TestPushIntegration_PasswordChangeAndResetEndPushes(t *testing.T) {
	w := newPushWorld(t)
	id, phoneA := w.signUp("ps_change")
	phoneB := w.session("/v1/auth/login", "", map[string]any{"login": "ps_change", "password": "Correct-horse-battery-staple1"})
	const tokA, tokB = "ExponentPushToken[changeA]", "ExponentPushToken[changeB]"
	w.saveToken(phoneA.AccessToken, tokA)
	w.saveToken(phoneB.AccessToken, tokB)
	w.wantPushedTo("two phones", id, tokA, tokB)

	changed := w.session("/v1/me/password", phoneA.AccessToken,
		map[string]any{"current_password": "Correct-horse-battery-staple1", "new_password": "Brand-new-password-2!"})
	w.wantPushedTo("after the change", id)
	w.saveToken(phoneB.AccessToken, tokB)
	w.wantPushedTo("the other phone saves its token again", id)
	w.saveToken(changed.AccessToken, tokA)
	w.wantPushedTo("the phone that changed it saves its token", id, tokA)

	// Forgot password on phone B: every session ends again, and B's new one
	// gets its pushes once it saves its token.
	box := &captureSender{}
	reset := NewPasswordResetHandler(postgres.NewUserRepo(w.pool), auth.NewRefreshService(postgres.NewRefreshTokenRepo(w.pool), time.Hour),
		postgres.NewPasswordResetRepo(w.pool), box, integrationJWTSecret, 15*time.Minute)
	reset.floor = 0
	t.Cleanup(reset.Wait)
	routes := gin.New()
	reset.Register(routes.Group("/v1"))
	integrationDo(t, routes, http.MethodPost, "/v1/auth/password/forgot", "", map[string]any{"email": "ps_change@example.com"})
	reset.Wait()
	rec := integrationDo(t, routes, http.MethodPost, "/v1/auth/password/reset", "",
		map[string]any{"email": "ps_change@example.com", "code": box.lastCode(t, "ps_change@example.com"), "new_password": "Third-password-3!"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	w.wantPushedTo("after the reset", id)
	w.saveToken(decodeBody(t, rec)["access_token"].(string), tokB)
	w.wantPushedTo("the phone that reset it saves its token", id, tokB)
}

// An admin's disable ends the pushes, and enabling the account again does
// not bring them back: its sessions are over.
func TestPushIntegration_AdminDisableEndsPushes(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &pushWorld{feedWorld: &feedWorld{t: t, clk: clk}, pool: pool}
	adminFirst(w.feedWorld, pool, integrationOptions{clock: clk, adminEmails: []string{"pd_boss@example.com"},
		wire: func(_ *social.Service, _ *board.Service, deps *Deps) {
			deps.PushTokens = NewPushTokenHandler(postgres.NewPushRepo(pool))
		}}, "pd_boss")
	id, s := w.signUp("pd_bob")
	const tok = "ExponentPushToken[disabledBob]"
	w.saveToken(s.AccessToken, tok)
	w.wantPushedTo("saved", id, tok)

	cookie := adminLogin(t, w.feedWorld, "pd_boss@example.com", adminTestPassword)
	page := adminDo(t, w.feedWorld, http.MethodGet, "/admin/people/"+id, cookie, nil, "").Body.String()
	csrf := csrfOf(t, page)
	rec := adminDo(t, w.feedWorld, http.MethodPost, "/admin/people/"+id+"/disable", cookie, url.Values{"csrf": {csrf}}, "")
	if !strings.Contains(rec.Header().Get("Location"), "n=disabled") {
		t.Fatalf("disable: %s", rec.Header().Get("Location"))
	}
	w.wantPushedTo("disabled", id)
	rec = adminDo(t, w.feedWorld, http.MethodPost, "/admin/people/"+id+"/enable", cookie, url.Values{"csrf": {csrf}}, "")
	if !strings.Contains(rec.Header().Get("Location"), "n=enabled") {
		t.Fatalf("enable: %s", rec.Header().Get("Location"))
	}
	w.wantPushedTo("enabled again", id)
}

// refreshRefused checks that refreshing with refreshToken answers 401
// token_invalid, as the app's sign-out expects.
func (w *pushWorld) refreshRefused(what, refreshToken string) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refreshToken})
	if rec.Code != http.StatusUnauthorized || decodeBody(w.t, rec)["error"] != string(CodeTokenInvalid) {
		w.t.Fatalf("%s: %d %s, want 401 token_invalid", what, rec.Code, rec.Body.String())
	}
}

// logOut sends the app's log out with refreshToken: always 204.
func (w *pushWorld) logOut(refreshToken string) {
	w.t.Helper()
	if rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/auth/logout", "", map[string]any{"refresh_token": refreshToken}); rec.Code != http.StatusNoContent {
		w.t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
}

// A refresh whose answer never reaches the phone (the server rotated the
// token and moved the push token on; the phone still holds the old token)
// ends the session when the phone uses the old token again: it is refused,
// so the app signs out, and the token the server made, with the pushes tied
// to it, ends too. The app's log out with the old token then answers 204.
func TestPushIntegration_ALostRefreshAnswerEndsTheSession(t *testing.T) {
	w := newPushWorld(t)
	id, s := w.signUp("ps_lost")
	const tok = "ExponentPushToken[lostAnswer]"
	w.saveToken(s.AccessToken, tok)

	lost := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": s.RefreshToken})
	w.wantPushedTo("after the refresh whose answer was lost", id, tok)
	w.refreshRefused("the old token again", s.RefreshToken)
	w.wantPushedTo("after the old token came back", id)
	w.refreshRefused("the token the lost answer carried", lost.RefreshToken)
	w.logOut(s.RefreshToken)
	w.wantPushedTo("after the log out", id)
}

// Log out with the token from before a refresh whose answer never arrived
// ends the session the refresh carried on, and its pushes, at once.
func TestPushIntegration_LogOutWithAStaleTokenEndsTheSession(t *testing.T) {
	w := newPushWorld(t)
	id, s := w.signUp("ps_stale")
	const tok = "ExponentPushToken[staleLogout]"
	w.saveToken(s.AccessToken, tok)
	lost := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": s.RefreshToken})
	again := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": lost.RefreshToken})
	w.wantPushedTo("two refreshes on", id, tok)

	w.logOut(s.RefreshToken)
	w.wantPushedTo("after a log out with the oldest token", id)
	w.refreshRefused("the session's newest token", again.RefreshToken)
}

// A stolen refresh token: whoever uses it second, the phone or the thief,
// ends the session for both, and the pushes tied to it.
func TestPushIntegration_AStolenTokenEndsTheSessionForBoth(t *testing.T) {
	w := newPushWorld(t)
	id, s := w.signUp("ps_theft")
	const tok = "ExponentPushToken[stolen]"

	// The thief refreshes first; the phone's own refresh then ends it all.
	w.saveToken(s.AccessToken, tok)
	thief := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": s.RefreshToken})
	w.refreshRefused("the phone, after the thief", s.RefreshToken)
	w.refreshRefused("the thief's next refresh", thief.RefreshToken)
	w.wantPushedTo("thief first", id)

	// The phone refreshes first; the thief's copy then ends it all.
	again := w.session("/v1/auth/login", "", map[string]any{"login": "ps_theft", "password": "Correct-horse-battery-staple1"})
	w.saveToken(again.AccessToken, tok)
	phone := w.session("/v1/auth/refresh", "", map[string]any{"refresh_token": again.RefreshToken})
	w.wantPushedTo("the phone refreshed", id, tok)
	w.refreshRefused("the thief, after the phone", again.RefreshToken)
	w.refreshRefused("the phone's next refresh", phone.RefreshToken)
	w.wantPushedTo("phone first", id)
}

// A DELETE of the push token that arrives after the person logged out and
// signed in again on the phone (the new session saved the token again) leaves
// the token with the new session, and so does a late save from the old one.
// The new session's own DELETE deletes it.
func TestPushIntegration_ALateDeleteLeavesANewerSignInsToken(t *testing.T) {
	w := newPushWorld(t)
	id, first := w.signUp("ps_late")
	const tok = "ExponentPushToken[lateDelete]"
	w.saveToken(first.AccessToken, tok)

	w.logOut(first.RefreshToken)
	again := w.session("/v1/auth/login", "", map[string]any{"login": "ps_late", "password": "Correct-horse-battery-staple1"})
	w.saveToken(again.AccessToken, tok)

	remove := func(accessToken string) {
		t.Helper()
		if rec := integrationDo(t, w.engine, http.MethodDelete, "/v1/me/push-tokens", accessToken, map[string]any{"token": tok}); rec.Code != http.StatusNoContent {
			t.Fatalf("delete the push token: %d %s", rec.Code, rec.Body.String())
		}
	}
	remove(first.AccessToken) // the old session's access token still works for minutes
	w.wantPushedTo("after the late delete", id, tok)
	w.saveToken(first.AccessToken, tok)
	w.wantPushedTo("after a late save from the old session", id, tok)
	remove(again.AccessToken)
	w.wantPushedTo("after the new session's delete", id)
}
