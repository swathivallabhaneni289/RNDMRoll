package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
