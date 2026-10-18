package httpapi

import (
	"net/http"
	"testing"
)

// commentIDs are the ids of a post page's comments as p reads them, and the
// comment count of its card.
func (w *feedWorld) commentIDs(p feedPerson, entryID string) ([]string, float64) {
	w.t.Helper()
	got := w.ok(p, http.MethodGet, "/v1/posts/"+entryID, nil)
	ids := []string{}
	list, _ := got["comments"].([]any)
	for _, c := range list {
		ids = append(ids, c.(map[string]any)["id"].(string))
	}
	return ids, dig(w.t, got, "card", "comment_count").(float64)
}

func TestPostIntegration_TheOwnerHidesAComment(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("hc_alice", "Alice Ames")
	b := w.person("hc_bob", "Bob Brown")
	c := w.person("hc_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)
	path := "/v1/posts/" + bEntry

	rude := w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": "Rude"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	reply := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Hey", "parent_id": rude}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	kind := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Lovely"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	own := w.call(b, http.MethodPost, path+"/comments", map[string]any{"text": "Thanks"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)

	// Only the owner may hide, and not their own comment.
	w.call(c, http.MethodPost, "/v1/comments/"+rude+"/hide", map[string]any{}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/comments/"+rude+"/hide", map[string]any{}, http.StatusNotFound)
	w.call(b, http.MethodPost, "/v1/comments/"+own+"/hide", map[string]any{}, http.StatusNotFound)
	if got := w.ok(b, http.MethodPost, "/v1/comments/"+rude+"/hide", map[string]any{}); got["hidden"] != true {
		t.Fatalf("hide: %v", got)
	}

	// C no longer sees it or its reply, and the count follows.
	if ids, n := w.commentIDs(c, bEntry); len(ids) != 2 || !has(ids, kind) || !has(ids, own) || n != 2 {
		t.Fatalf("c after hide: %v %v", ids, n)
	}
	// A, its writer, still sees it and is not told.
	got := w.ok(a, http.MethodGet, path, nil)
	for _, item := range got["comments"].([]any) {
		cm := item.(map[string]any)
		if cm["id"] == rude && (cm["hidden"] != false || cm["can_hide"] != false) {
			t.Fatalf("writer is told: %v", cm)
		}
	}
	if ids, _ := w.commentIDs(a, bEntry); !has(ids, rude) {
		t.Fatalf("writer lost the comment: %v", ids)
	}
	// B, the owner, sees it marked hidden, with the reply under it.
	got = w.ok(b, http.MethodGet, path, nil)
	seen := map[string]map[string]any{}
	for _, item := range got["comments"].([]any) {
		cm := item.(map[string]any)
		seen[cm["id"].(string)] = cm
	}
	if seen[rude]["hidden"] != true || seen[rude]["can_hide"] != true || seen[reply] == nil || seen[own]["can_hide"] != false {
		t.Fatalf("owner view: %v", got["comments"])
	}

	// Unhide brings it back for everyone.
	w.ok(b, http.MethodPost, "/v1/comments/"+rude+"/unhide", map[string]any{})
	if ids, n := w.commentIDs(c, bEntry); len(ids) != 4 || n != 4 {
		t.Fatalf("after unhide: %v %v", ids, n)
	}
}
