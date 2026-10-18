package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

func TestPostIntegration_APhotoInAComment(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: photos}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}

	a := w.person("cp_alice", "Alice Ames")
	b := w.person("cp_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	path := "/v1/posts/" + bEntry + "/comments"

	mine := photos.newKey(uuid.MustParse(a.id), "jpg")
	theirs := photos.newKey(uuid.MustParse(b.id), "jpg")

	// No text and no photo, someone else's key, a bad key: refused.
	w.call(a, http.MethodPost, path, map[string]any{"text": "  "}, http.StatusBadRequest)
	if got := w.call(a, http.MethodPost, path, map[string]any{"photo_key": theirs}, http.StatusBadRequest); got["field"] != "photo" {
		t.Fatalf("someone else's key: %v", got)
	}
	w.call(a, http.MethodPost, path, map[string]any{"photo_key": "entries/../x.jpg"}, http.StatusBadRequest)

	// A photo alone is a comment; its link comes back and on the post page.
	got := w.call(a, http.MethodPost, path, map[string]any{"photo_key": mine}, http.StatusCreated)["comment"].(map[string]any)
	if got["text"] != "" || got["photo_url"] == "" {
		t.Fatalf("photo comment: %v", got)
	}
	photoComment := got["id"].(string)
	// The same file cannot be used twice.
	w.call(a, http.MethodPost, path, map[string]any{"text": "again", "photo_key": mine}, http.StatusBadRequest)
	// A reply with a photo and words.
	second := photos.newKey(uuid.MustParse(b.id), "png")
	reply := w.call(b, http.MethodPost, path, map[string]any{"text": "Nice", "photo_key": second, "parent_id": photoComment}, http.StatusCreated)["comment"].(map[string]any)
	if reply["photo_url"] == "" || reply["text"] != "Nice" {
		t.Fatalf("reply with a photo: %v", reply)
	}
	page := w.ok(b, http.MethodGet, "/v1/posts/"+bEntry, nil)
	if c := page["comments"].([]any)[0].(map[string]any); c["photo_url"] == "" {
		t.Fatalf("post page: %v", c)
	}

	// The bell gives the photo-only comment no text (the app says "sent a photo.").
	for _, item := range w.notes(b)["items"].([]any) {
		it := item.(map[string]any)
		if it["kind"] == "comment" && it["text"] != nil {
			t.Fatalf("photo-only comment carries text: %v", it)
		}
	}

	// Deleting the comment deletes its photo and its reply's photo.
	w.ok(a, http.MethodPost, "/v1/comments/"+photoComment+"/delete", map[string]any{})
	gone := map[string]bool{}
	for _, k := range photos.deletes() {
		gone[k] = true
	}
	if !gone[mine] || !gone[second] {
		t.Fatalf("files not removed: %v", photos.deletes())
	}
}
