package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// Asking for a second look in the app (developer, 2026-11-19), on the
// database: POST /v1/removals/:id/second-look.

// heardLooks counts the admin emails the second looks asked for.
type heardLooks struct {
	mu sync.Mutex
	n  int
}

func (h *heardLooks) SecondLookAsked() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.n++
}

func (h *heardLooks) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.n
}

// lookWorld is a feed world whose second looks are heard, over a pool the
// test also uses.
type lookWorld struct {
	*feedWorld
	pool  *pgxpool.Pool
	heard *heardLooks
}

func newLookWorld(t *testing.T) *lookWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &lookWorld{pool: pool, heard: &heardLooks{}}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk,
		wire: func(_ *social.Service, _ *board.Service, d *Deps) { d.SecondLooks.svc.SetNotifier(w.heard) }}).Engine()
	w.feedWorld = &feedWorld{t: t, engine: engine, clk: clk}
	return w
}

// remove removes p's post or comment with the admin's Remove and returns
// the removal's id.
func (w *lookWorld) remove(kind, id string, p feedPerson) string {
	w.t.Helper()
	ctx := context.Background()
	now := time.Now()
	if err := postgres.NewAdminRepo(w.pool).RemoveItem(ctx, kind, uuid.MustParse(id), uuid.MustParse(p.id), "spam", now, now); err != nil {
		w.t.Fatalf("remove: %v", err)
	}
	var removal string
	if err := w.pool.QueryRow(ctx, `select id::text from moderation_events where item_id = $1 and action = 'removed'
		order by created_at desc limit 1`, id).Scan(&removal); err != nil {
		w.t.Fatal(err)
	}
	return removal
}

// ask sends p's request about removal and wants status; it returns the body.
func (w *lookWorld) ask(p feedPerson, removal string, body any, status int) map[string]any {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodPost, "/v1/removals/"+removal+"/second-look", p.token, body)
	if rec.Code != status {
		w.t.Fatalf("ask as %s: %d %s, want %d", p.name, rec.Code, rec.Body.String(), status)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		w.t.Fatalf("ask: %v: %s", err, rec.Body.String())
	}
	return out
}

func lookKey(p feedPerson, hex string) string { return "entries/" + p.id + "/" + hex + ".jpg" }

// The owner asks once, with their words trimmed and up to 3 of their own
// fresh photos, kept on their own clock; the admins hear of it once, never
// of the retry. Someone else, a bad id and no session get nothing, each
// rule answers in its own short way, and a proof's file is refused to a
// comment.
func TestSecondLookIntegration_TheOwnerAsksOnce(t *testing.T) {
	w := newLookWorld(t)
	ann, bo, cy := w.person("sl_ann", "Ann"), w.person("sl_bo", "Bo"), w.person("sl_cy", "Cy")
	post := w.post(ann, "2026-10-08")
	removal := w.remove(admin.KindPost, post, ann)
	k1, k2 := lookKey(ann, "00000000000000a1"), lookKey(ann, "00000000000000a2")
	// The rules are tried by Cy on a removal of their own: the limit is 10
	// sends a minute each, and the test's minute stands still.
	cyRemoval := w.remove(admin.KindPost, w.post(cy, "2026-10-08"), cy)
	c1, c2 := lookKey(cy, "00000000000000c1"), lookKey(cy, "00000000000000c2")

	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/removals/"+removal+"/second-look", "", map[string]any{"text": "Why"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", rec.Code)
	}
	for _, id := range []string{"not-a-uuid", removal[:35], uuid.NewString()} {
		if got := w.ask(ann, id, map[string]any{"text": "Why"}, http.StatusNotFound); got["error"] != "not_found" {
			t.Fatalf("removal %q: %v", id, got)
		}
	}
	if got := w.ask(bo, removal, map[string]any{"text": "Why"}, http.StatusNotFound); got["error"] != "not_found" {
		t.Fatalf("someone else's removal: %v", got)
	}
	for name, tc := range map[string]struct {
		body          map[string]any
		field, reason string
	}{
		"no words":         {map[string]any{"photo_keys": []string{c1}}, "text", "required"},
		"only spaces":      {map[string]any{"text": "  \n "}, "text", "required"},
		"501 characters":   {map[string]any{"text": strings.Repeat("x", 501)}, "text", "too_long"},
		"words as number":  {map[string]any{"text": 7}, "text", "invalid"},
		"4 photos":         {map[string]any{"text": "Why", "photo_keys": []string{c1, c2, lookKey(cy, "00000000000000a3"), lookKey(cy, "00000000000000a4")}}, "photo_keys", "invalid"},
		"someone's photo":  {map[string]any{"text": "Why", "photo_keys": []string{lookKey(bo, "00000000000000b1")}}, "photo_keys", "invalid"},
		"one photo twice":  {map[string]any{"text": "Why", "photo_keys": []string{c1, c1}}, "photo_keys", "invalid"},
		"photos as a word": {map[string]any{"text": "Why", "photo_keys": "k"}, "photo_keys", "invalid"},
	} {
		if got := w.ask(cy, cyRemoval, tc.body, http.StatusBadRequest); got["field"] != tc.field || got["reason"] != tc.reason {
			t.Fatalf("%s: %v, want %s %s", name, got, tc.field, tc.reason)
		}
	}
	if w.heard.count() != 0 {
		t.Fatalf("a refusal was emailed")
	}

	if got := w.ask(ann, removal, map[string]any{"text": "  It was a joke.  ", "photo_keys": []string{k1, k2}}, http.StatusCreated); len(got) != 1 || got["second_look"] != "asked" {
		t.Fatalf("ask: %v", got)
	}
	var body string
	var keys []string
	var at time.Time
	if err := w.pool.QueryRow(context.Background(), `select body, photo_keys, at from second_looks where removal_id = $1`, removal).Scan(&body, &keys, &at); err != nil {
		t.Fatal(err)
	}
	if body != "It was a joke." || strings.Join(keys, ",") != k1+","+k2 || !at.Equal(w.clk.NowFor(ann.id)) {
		t.Fatalf("stored %q %v at %v, want the words trimmed, both photos and the owner's clock %v", body, keys, at, w.clk.NowFor(ann.id))
	}
	if w.heard.count() != 1 {
		t.Fatalf("%d admin emails, want 1", w.heard.count())
	}
	if got := w.ask(ann, removal, map[string]any{"text": "It was a joke.", "photo_keys": []string{k1, k2}}, http.StatusOK); got["second_look"] != "asked" {
		t.Fatalf("the phone's retry: %v", got)
	}
	if got := w.ask(ann, removal, map[string]any{"text": "Please look again"}, http.StatusConflict); got["error"] != "second_look_asked" {
		t.Fatalf("a second request: %v", got)
	}
	if w.heard.count() != 1 {
		t.Fatalf("%d admin emails after the retry and the second request, want still 1", w.heard.count())
	}

	// The proof's file is refused to a comment, and to another request.
	other := w.post(ann, "2026-10-09")
	if got := w.call(ann, http.MethodPost, "/v1/posts/"+other+"/comments", map[string]any{"photo_key": k1}, http.StatusBadRequest); got["field"] != "photo" {
		t.Fatalf("a comment with a proof's file: %v", got)
	}
	otherRemoval := w.remove(admin.KindPost, other, ann)
	if got := w.ask(ann, otherRemoval, map[string]any{"text": "Why", "photo_keys": []string{k2}}, http.StatusBadRequest); got["field"] != "photo_keys" {
		t.Fatalf("another request with the first one's file: %v", got)
	}

	// Brought back: answered. Past the 30 days: too late.
	if err := postgres.NewAdminRepo(w.pool).RestoreItem(context.Background(), admin.KindPost, uuid.MustParse(other), uuid.MustParse(ann.id), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := w.ask(ann, otherRemoval, map[string]any{"text": "Why"}, http.StatusConflict); got["error"] != "second_look_back" {
		t.Fatalf("after Bring it back: %v", got)
	}
	third := w.post(bo, "2026-10-08")
	thirdRemoval := w.remove(admin.KindPost, third, bo)
	if _, err := w.pool.Exec(context.Background(), `with e as (update entries set removed_at = now() - interval '30 days' where id = $1)
		update moderation_events set created_at = now() - interval '30 days' where id = $2`, third, thirdRemoval); err != nil {
		t.Fatal(err)
	}
	if got := w.ask(bo, thirdRemoval, map[string]any{"text": "Why"}, http.StatusConflict); got["error"] != "second_look_too_late" {
		t.Fatalf("past the 30 days: %v", got)
	}
	if w.heard.count() != 1 {
		t.Fatalf("%d admin emails, want still 1", w.heard.count())
	}
}
