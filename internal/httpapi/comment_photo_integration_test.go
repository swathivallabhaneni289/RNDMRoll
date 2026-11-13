package httpapi

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
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

// commentPushes is a board.CommentNotifier that counts the comments it hears
// about. AddComment calls it before it answers, so the count is ready then.
type commentPushes struct {
	mu sync.Mutex
	n  int
}

func (c *commentPushes) CommentAdded(uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *commentPushes) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// The phone gives up on an answer after 10 seconds and keeps the photo, so a
// retry sends the same photo key. The same photo with the same words on the
// same post and thread is the comment already saved: the same answer, no
// second comment and no second push. Any other use of the file stays refused.
func TestPostIntegration_APhotoCommentSentAgainIsTheSameComment(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	pushes := &commentPushes{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: photos,
		wire: func(_ *social.Service, boardSvc *board.Service, _ *Deps) { boardSvc.SetCommentNotifier(pushes) }}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}

	a := w.person("cr_alice", "Alice Ames")
	b := w.person("cr_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	aEntry := w.post(a, day)
	path := "/v1/posts/" + bEntry + "/comments"
	key := photos.newKey(uuid.MustParse(a.id), "jpg")

	first := w.call(a, http.MethodPost, path, map[string]any{"text": "Look", "photo_key": key}, http.StatusCreated)["comment"].(map[string]any)
	// Sent again 15 minutes on, with the words spaced differently: the
	// answer is the saved comment, at the time it was saved.
	w.at(day, 20, 45)
	again := w.call(a, http.MethodPost, path, map[string]any{"text": " Look ", "photo_key": key}, http.StatusCreated)["comment"].(map[string]any)
	if again["id"] != first["id"] || again["at"] != first["at"] || again["text"] != "Look" || again["photo_url"] == "" {
		t.Fatalf("sent again: %v, first %v", again, first)
	}
	if n := pushes.count(); n != 1 {
		t.Fatalf("pushed %d times, want once", n)
	}
	comments := w.ok(b, http.MethodGet, "/v1/posts/"+bEntry, nil)["comments"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["id"] != first["id"] {
		t.Fatalf("post page: %v", comments)
	}

	// Other words, a reply, or another post with the same file: refused.
	for _, send := range []struct {
		path string
		body map[string]any
	}{
		{path, map[string]any{"text": "Look again", "photo_key": key}},
		{path, map[string]any{"text": "Look", "photo_key": key, "parent_id": first["id"]}},
		{"/v1/posts/" + aEntry + "/comments", map[string]any{"text": "Look", "photo_key": key}},
	} {
		if got := w.call(a, http.MethodPost, send.path, send.body, http.StatusBadRequest); got["field"] != "photo" {
			t.Fatalf("%s %v: %v", send.path, send.body, got)
		}
	}
	if n := pushes.count(); n != 1 {
		t.Fatalf("pushed %d times after the refusals, want once", n)
	}

	// Two sends at the same moment: one writes, the other gets that comment.
	raced := map[string]any{"text": "Twice", "photo_key": photos.newKey(uuid.MustParse(a.id), "jpg")}
	reqs := []*http.Request{
		integrationRequest(t, http.MethodPost, path, a.token, raced),
		integrationRequest(t, http.MethodPost, path, a.token, raced),
	}
	ids := map[string]bool{}
	for i, rec := range integrationAtOnce(w.engine, reqs) {
		if rec.Code != http.StatusCreated {
			t.Fatalf("raced send %d: %d %s", i, rec.Code, rec.Body.String())
		}
		ids[dig(t, decodeBody(t, rec), "comment", "id").(string)] = true
	}
	if len(ids) != 1 || pushes.count() != 2 {
		t.Fatalf("raced sends: comments %v, pushes %d, want one new comment pushed once", ids, pushes.count())
	}
	if comments := w.ok(b, http.MethodGet, "/v1/posts/"+bEntry, nil)["comments"].([]any); len(comments) != 2 {
		t.Fatalf("post page after the raced sends: %v", comments)
	}
}
