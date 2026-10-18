package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// gridIDs are the entry ids of a person page's grid, in order; nil when the
// page carries no grid.
func gridIDs(t *testing.T, page map[string]any) []string {
	t.Helper()
	if page["posts"] == nil {
		return nil
	}
	ids := []string{}
	for _, item := range page["posts"].([]any) {
		ids = append(ids, item.(map[string]any)["entry_id"].(string))
	}
	return ids
}

func TestPersonIntegration_HeadForAnyoneGridForAFollowerOnSharedDays(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pp_alice", "Alice Ames")
	b := w.person("pp_bob", "Bob Brown")

	b1 := w.post(b, "2026-10-06")
	b2 := w.post(b, "2026-10-07")
	b3 := w.post(b, "2026-10-08")
	w.post(a, "2026-10-06")
	w.skip(a, "2026-10-07")
	w.post(a, "2026-10-08")

	// A stranger sees the head only.
	page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	if page["relation"] != "none" || page["follows_you"] != false || page["posts"] != nil || page["next_cursor"] != nil {
		t.Fatalf("stranger: %v", page)
	}
	if dig(t, page, "person", "username") != "pp_bob" || dig(t, page, "stats", "spins") != float64(3) ||
		dig(t, page, "stats", "followers") != float64(0) {
		t.Fatalf("head: %v", page)
	}

	// A pending request is still the head only.
	w.follow(a, b)
	page = w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	if page["relation"] != "requested" || page["posts"] != nil {
		t.Fatalf("requested: %v", page)
	}
	if got := w.ok(b, http.MethodGet, "/v1/people/"+a.id, nil); got["follows_you"] != false {
		t.Fatalf("a pending request is not a follow: %v", got)
	}
	w.ok(a, http.MethodPost, "/v1/follows/"+b.id+"/unfollow", map[string]any{})

	// An accepted follower sees only the days they posted too, newest first.
	w.followAccepted(a, b)
	page = w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	if ids := gridIDs(t, page); len(ids) != 2 || ids[0] != b3 || ids[1] != b1 {
		t.Fatalf("grid: %v (b2 %s is a day a skipped)", ids, b2)
	}
	if dig(t, page, "stats", "followers") != float64(1) {
		t.Fatalf("followers: %v", page)
	}
	first := page["posts"].([]any)[0].(map[string]any)
	if first["photo_url"] == "" || first["photo_count"] != float64(1) || first["title"] == "" {
		t.Fatalf("tile: %v", first)
	}

	// Pages: one at a time, the cursor is the last tile.
	page = w.ok(a, http.MethodGet, "/v1/people/"+b.id+"?limit=1", nil)
	if ids := gridIDs(t, page); len(ids) != 1 || ids[0] != b3 || page["next_cursor"] != b3 {
		t.Fatalf("page 1: %v", page)
	}
	page = w.ok(a, http.MethodGet, "/v1/people/"+b.id+"?limit=1&cursor="+b3, nil)
	if ids := gridIDs(t, page); len(ids) != 1 || ids[0] != b1 || page["next_cursor"] != nil {
		t.Fatalf("page 2: %v", page)
	}
	w.call(a, http.MethodGet, "/v1/people/"+b.id+"?limit=0", nil, http.StatusBadRequest)
	w.call(a, http.MethodGet, "/v1/people/"+b.id+"?cursor=nope", nil, http.StatusBadRequest)

	// An archived post leaves the grid.
	w.ok(b, http.MethodPost, "/v1/entries/"+b3+"/archive", map[string]any{})
	if ids := gridIDs(t, w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)); len(ids) != 1 || ids[0] != b1 {
		t.Fatalf("archived: %v", ids)
	}

	// Yourself, an unknown id and a block either way are all 404.
	w.call(a, http.MethodGet, "/v1/people/"+a.id, nil, http.StatusNotFound)
	w.call(a, http.MethodGet, "/v1/people/00000000-0000-0000-0000-000000000001", nil, http.StatusNotFound)
	w.call(a, http.MethodGet, "/v1/people/not-an-id", nil, http.StatusNotFound)
	w.ok(b, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	w.call(a, http.MethodGet, "/v1/people/"+b.id, nil, http.StatusNotFound)
	w.call(b, http.MethodGet, "/v1/people/"+a.id, nil, http.StatusNotFound)

	// The fixed routes beside it still answer.
	w.ok(a, http.MethodGet, "/v1/people/search?q=pp", nil)
}

func TestPersonIntegration_MuteTakesPostsOffTheBoardOnly(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pm_alice", "Alice Ames")
	b := w.person("pm_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)

	if ids := cardIDs(t, w.board(a, "")); !has(ids, bEntry) {
		t.Fatalf("before mute: %v", ids)
	}
	if got := w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/mute", map[string]any{}); got["muted"] != true {
		t.Fatalf("mute: %v", got)
	}
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/mute", map[string]any{})
	if ids := cardIDs(t, w.board(a, "")); has(ids, bEntry) {
		t.Fatalf("muted still on the Board: %v", ids)
	}
	// The person page and the post itself still open; the page says muted.
	page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	if page["muted"] != true || len(gridIDs(t, page)) != 1 {
		t.Fatalf("person page while muted: %v", page)
	}
	w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)
	// B is never told: B's view of A says nothing.
	if got := w.ok(b, http.MethodGet, "/v1/people/"+a.id, nil); got["muted"] != false {
		t.Fatalf("b sees a mute: %v", got)
	}

	if got := w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/unmute", map[string]any{}); got["muted"] != false {
		t.Fatalf("unmute: %v", got)
	}
	if ids := cardIDs(t, w.board(a, "")); !has(ids, bEntry) {
		t.Fatalf("after unmute: %v", ids)
	}
	w.call(a, http.MethodPost, "/v1/people/"+a.id+"/mute", map[string]any{}, http.StatusNotFound)
}

func TestPersonIntegration_Report(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pr_alice", "Alice Ames")
	b := w.person("pr_bob", "Bob Brown")

	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "spam"})
	// A second report replaces the reason.
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	w.call(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "nope"}, http.StatusBadRequest)
	w.call(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{}, http.StatusBadRequest)
	w.call(a, http.MethodPost, "/v1/people/"+a.id+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.ok(b, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	w.call(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
}

func TestPersonIntegration_BioForAnyoneWhoCanOpenThePage(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pb_alice", "Alice Ames")
	b := w.person("pb_bob", "Bob Brown")
	w.ok(b, http.MethodPatch, "/v1/me", map[string]any{"bio": "Spins on Sundays."})

	// The page is read directly: the feed guard forbids "bio", and this is
	// the one route that sends it, to a stranger too, like Instagram.
	rec := integrationDo(t, w.engine, http.MethodGet, "/v1/people/"+b.id, a.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("page: %d %s", rec.Code, rec.Body.String())
	}
	page := decodeBody(t, rec)
	if page["bio"] != "Spins on Sundays." || page["posts"] != nil {
		t.Fatalf("stranger sees the bio and no grid: %v", page)
	}
	for _, bad := range []string{`"email"`, `"birthday"`, `"password_hash"`, `"avatar_url"`, `"storage_key"`} {
		if strings.Contains(rec.Body.String(), bad) {
			t.Fatalf("page carries %s: %s", bad, rec.Body.String())
		}
	}
	// No bio: the key is left out.
	if got := w.ok(b, http.MethodGet, "/v1/people/"+a.id, nil); got["bio"] != nil {
		t.Fatalf("empty bio: %v", got)
	}
}
