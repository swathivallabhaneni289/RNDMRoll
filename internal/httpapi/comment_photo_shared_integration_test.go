package httpapi

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// A photo file belongs to one place. The comment route already refuses a
// post's file, and a post refuses a comment's file too, on an edit and on a
// new post, with the answer any other file in use gets. Found live: a post
// that took a comment's file lost its photo when the comment was deleted.
func TestPostIntegration_APostRefusesACommentsPhotoFile(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: photos}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}

	a := w.person("ks_alice", "Alice Ames")
	b := w.person("ks_bob", "Bob Brown")
	w.followAccepted(b, a)
	const day = "2026-10-08"
	aEntry := w.post(a, day)
	bEntry := w.post(b, day)
	key := photos.newKey(uuid.MustParse(b.id), "jpg")
	commentID := w.call(b, http.MethodPost, "/v1/posts/"+aEntry+"/comments",
		map[string]any{"text": "shared file", "photo_key": key}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)

	refused := func(label string, got map[string]any) {
		t.Helper()
		if got["error"] != "validation_failed" || got["field"] != "photos" {
			t.Fatalf("%s: %v, want validation_failed on photos", label, got)
		}
	}
	// An edit that adds the comment's file to B's own post.
	entry := entryObject(t, w.ok(b, http.MethodGet, "/v1/entries/"+bEntry, nil)["entry"])
	links := photoLinks(t, entry)
	refused("the edit", w.call(b, http.MethodPatch, "/v1/entries/"+bEntry,
		map[string]any{"photos": append(keepPhotos(photoIDs(t, entry)...), newPhotoItem(key))}, http.StatusBadRequest))
	// A new post the next day with the comment's file.
	const next = "2026-10-09"
	w.at(next, 20, 30)
	spun := w.ok(b, http.MethodPost, "/v1/spin", map[string]any{"for_date": next})
	seq := int(dig(t, spun, "today", "spin", "seq").(float64))
	refused("the new post", w.call(b, http.MethodPost, "/v1/entries", entryRequest(next, seq, key), http.StatusBadRequest))

	// The post keeps its own photo only, and deleting the comment removes
	// the comment's file.
	after := entryObject(t, w.ok(b, http.MethodGet, "/v1/entries/"+bEntry, nil)["entry"])
	if got := photoLinks(t, after); !slices.Equal(got, links) {
		t.Fatalf("B's post photos after the refusals: %v, want %v", got, links)
	}
	before := len(photos.deletes())
	w.ok(b, http.MethodPost, "/v1/comments/"+commentID+"/delete", map[string]any{})
	if got := photos.deletes()[before:]; !slices.Equal(got, []string{key}) {
		t.Fatalf("removed files = %v, want the comment's %s", got, key)
	}
}

// A file that a comment and a post both name stays while the post shows it.
// Both routes refuse such a file, but two hand-made requests at the same
// moment could still leave one. Deleting the comment, by its writer or by an
// admin's remove, removes the files of the comment and its replies but not
// that one.
func TestPostIntegration_DeletingACommentKeepsAFileAPostStillShows(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: photos}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}

	a := w.person("kd_alice", "Alice Ames")
	b := w.person("kd_bob", "Bob Brown")
	w.followAccepted(b, a)
	const day = "2026-10-08"
	aEntry := w.post(a, day)
	bEntry := w.post(b, day)
	path := "/v1/posts/" + aEntry + "/comments"
	comment := func(p feedPerson, body map[string]any) string {
		t.Helper()
		return w.call(p, http.MethodPost, path, body, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	}
	bPhoto := postPhotoKeys(t, pool, bEntry)[0]

	// B's comment names the file of B's own post, straight in the database,
	// and A replies under it with a photo of their own.
	shared := comment(b, map[string]any{"text": "My own post's file"})
	setCommentPhoto(t, pool, shared, bPhoto)
	aReplyPhoto := photos.newKey(uuid.MustParse(a.id), "png")
	comment(a, map[string]any{"text": "Nice", "photo_key": aReplyPhoto, "parent_id": shared})

	before := len(photos.deletes())
	w.ok(b, http.MethodPost, "/v1/comments/"+shared+"/delete", map[string]any{})
	if got := photos.deletes()[before:]; !slices.Equal(got, []string{aReplyPhoto}) {
		t.Fatalf("removed files = %v, want only the reply's %s", got, aReplyPhoto)
	}
	if got := postPhotoKeys(t, pool, bEntry); !slices.Equal(got, []string{bPhoto}) {
		t.Fatalf("B's post photos: %v, want [%s]", got, bPhoto)
	}
	entry := entryObject(t, w.ok(b, http.MethodGet, "/v1/entries/"+bEntry, nil)["entry"])
	if got := photoLinks(t, entry); !slices.Equal(got, photos.linksOf(bPhoto)) {
		t.Fatalf("B's post links: %v, want %v", got, photos.linksOf(bPhoto))
	}

	// An admin's remove goes through the same delete.
	again := comment(b, map[string]any{"text": "Again"})
	setCommentPhoto(t, pool, again, bPhoto)
	keys, err := postgres.NewAdminRepo(pool).DeleteComment(context.Background(), uuid.MustParse(again))
	if err != nil || len(keys) != 0 {
		t.Fatalf("the admin's remove: keys %v (err %v), want none", keys, err)
	}
	if got := postPhotoKeys(t, pool, bEntry); !slices.Equal(got, []string{bPhoto}) {
		t.Fatalf("B's post photos after the admin's remove: %v, want [%s]", got, bPhoto)
	}
}
