package httpapi

import (
	"net/http"
	"testing"
)

// The page of someone you blocked stays, empty, with relation "blocked"
// (developer, 2026-10-18, like Instagram); they still get 404 for yours, and
// Unblock brings back a plain page with no follow between you.
func TestPersonIntegration_PageOfSomeoneYouBlocked(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pb_alice", "Alice Ames")
	b := w.person("pb_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.post(b, "2026-10-08")
	w.post(a, "2026-10-08")

	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": b.id})
	page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	stats := page["stats"].(map[string]any)
	if page["relation"] != "blocked" || dig(t, page, "person", "id") != b.id || page["posts"] != nil || page["bio"] != nil ||
		stats["posts"] != float64(0) || stats["followers"] != float64(0) || page["follows_you"] != false {
		t.Fatalf("blocked page: %v", page)
	}
	w.call(b, http.MethodGet, "/v1/people/"+a.id, nil, http.StatusNotFound)

	w.call(a, http.MethodDelete, "/v1/me/blocks/"+b.id, nil, http.StatusNoContent)
	if page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil); page["relation"] != "none" {
		t.Fatalf("after unblock: %v", page)
	}
}
