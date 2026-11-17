package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// postPhotoKeys reads the storage keys of an entry's photos, in position
// order.
func postPhotoKeys(t *testing.T, pool *pgxpool.Pool, entryID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select storage_key from entry_photos where entry_id = $1 order by position`, entryID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil || len(keys) == 0 {
		t.Fatalf("the photos of %s: %v (err %v)", entryID, keys, err)
	}
	return keys
}

// setCommentPhoto puts key on a comment straight in the database, the way a
// hand-made request or a bug might, past every check of the comment route.
func setCommentPhoto(t *testing.T, pool *pgxpool.Pool, commentID, key string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `update post_comments set photo_key = $2 where id = $1`, commentID, key); err != nil {
		t.Fatalf("set the photo of comment %s: %v", commentID, err)
	}
}

// Deleting a post removes the files of its photos and then of the photos in
// its comments and replies, whoever wrote them (the owner's own reply
// included). A comment photo the server would never delete stays: a key in
// someone else's folder, a malformed key, and a file one of its writer's own
// posts still shows.
func TestPostIntegration_DeletingAPostRemovesItsCommentPhotos(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: photos}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}

	a := w.person("pc_alice", "Alice Ames")
	b := w.person("pc_bob", "Bob Brown")
	c := w.person("pc_cat", "Cat Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	cEntry := w.post(c, day)
	path := "/v1/posts/" + bEntry + "/comments"
	comment := func(p feedPerson, body map[string]any) string {
		t.Helper()
		return w.call(p, http.MethodPost, path, body, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	}

	aPhoto := photos.newKey(uuid.MustParse(a.id), "jpg")
	bReplyPhoto := photos.newKey(uuid.MustParse(b.id), "png")
	cPhoto := photos.newKey(uuid.MustParse(c.id), "webp")
	aComment := comment(a, map[string]any{"text": "Look", "photo_key": aPhoto})
	comment(b, map[string]any{"text": "Mine", "photo_key": bReplyPhoto, "parent_id": aComment})
	comment(c, map[string]any{"photo_key": cPhoto})
	comment(a, map[string]any{"text": "No photo here"})

	// Three comments the database holds with photos no request could set.
	foreign := comment(c, map[string]any{"text": "Someone else's file"})
	setCommentPhoto(t, pool, foreign, photos.newKey(uuid.MustParse(a.id), "jpg"))
	malformed := comment(c, map[string]any{"text": "A bad key"})
	setCommentPhoto(t, pool, malformed, "entries/"+c.id+"/../"+a.id+"/0123456789abcdef.jpg")
	cPostPhoto := postPhotoKeys(t, pool, cEntry)[0]
	shared := comment(c, map[string]any{"text": "My own post's file"})
	setCommentPhoto(t, pool, shared, cPostPhoto)

	bPhotos := postPhotoKeys(t, pool, bEntry)
	before := len(photos.deletes())
	w.ok(b, http.MethodPost, "/v1/entries/"+bEntry+"/delete", map[string]any{})

	// The post's own photos first, then the comments' (the shared clock gives
	// every comment the same time, so their order among themselves is open).
	got := photos.deletes()[before:]
	wantComments := []string{aPhoto, bReplyPhoto, cPhoto}
	if len(got) != len(bPhotos)+len(wantComments) || !slices.Equal(got[:len(bPhotos)], bPhotos) {
		t.Fatalf("removed files = %v, want the post's own %v and then its comments' %v", got, bPhotos, wantComments)
	}
	gotComments := slices.Clone(got[len(bPhotos):])
	slices.Sort(gotComments)
	slices.Sort(wantComments)
	if !slices.Equal(gotComments, wantComments) {
		t.Fatalf("removed comment photos = %v, want %v", gotComments, wantComments)
	}
	var left int
	if err := pool.QueryRow(context.Background(), `select count(*) from post_comments where entry_id = $1`, bEntry).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d comments left on the deleted post (err %v)", left, err)
	}
	// C's own post still shows its photo.
	if got := postPhotoKeys(t, pool, cEntry); !slices.Equal(got, []string{cPostPhoto}) {
		t.Fatalf("C's post photos: %v", got)
	}

	// Deleting someone else's post is not found and removes nothing.
	before = len(photos.deletes())
	w.call(b, http.MethodPost, "/v1/entries/"+cEntry+"/delete", map[string]any{}, http.StatusNotFound)
	if got := photos.deletes()[before:]; len(got) != 0 {
		t.Fatalf("a refused delete removed %v", got)
	}
}

// An admin's Remove deletes nothing yet: the post with its photos, and the
// comment under it with its photo, stay for Bring it back, while they are
// gone for everyone, the owner and the comment's writer (who never reported
// it) included. The purge deletes the files once the 30 days are over.
func TestAdminIntegration_RemovingAPostKeepsItsPhotosForThePurge(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	photos := &fakeEntryPhotoStore{}
	w := &feedWorld{t: t, clk: clk}
	adminFirst(w, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"}, entryPhotos: photos}, "ad_boss")
	a := w.person("ad_alice", "Alice Ames")
	b := w.person("ad_bob", "Bob Brown")
	c := w.person("ad_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)
	aPhoto := photos.newKey(uuid.MustParse(a.id), "jpg")
	aComment := w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"photo_key": aPhoto}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	w.ok(c, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
	bPhotos := postPhotoKeys(t, pool, bEntry)

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	var postReport string
	for _, part := range strings.Split(page, `name="kind" value="post"><input type="hidden" name="id" value="`)[1:] {
		postReport = part[:36]
	}
	if postReport == "" {
		t.Fatalf("the post report is not offered for removal: %s", page)
	}
	before := len(photos.deletes())
	rec := adminDo(t, w, http.MethodPost, "/admin/reports/remove", cookie,
		url.Values{"csrf": {csrfOf(t, page)}, "kind": {"post"}, "id": {postReport}, "person": {b.id}}, "")
	if !strings.Contains(rec.Header().Get("Location"), "n=removed") {
		t.Fatalf("remove post: %s", rec.Header().Get("Location"))
	}
	if got := photos.deletes()[before:]; len(got) != 0 {
		t.Fatalf("Remove deleted files %v, want none until the purge", got)
	}
	if got := postPhotoKeys(t, pool, bEntry); !slices.Equal(got, bPhotos) {
		t.Fatalf("the post's photos after Remove: %v, want %v", got, bPhotos)
	}
	var kept string
	if err := pool.QueryRow(context.Background(), `select photo_key from post_comments where id = $1`, aComment).Scan(&kept); err != nil || kept != aPhoto {
		t.Fatalf("the comment under it: photo %q (err %v), want it kept with %s", kept, err, aPhoto)
	}
	for _, p := range []feedPerson{b, a} {
		w.call(p, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	}
	w.call(b, http.MethodGet, "/v1/entries/"+bEntry, nil, http.StatusNotFound)
}

// Over Postgres, a new profile photo removes the file of the one it replaced,
// once it is saved: the old key is read from the account row itself.
func TestProfileIntegration_ANewPhotoRemovesTheOldFile(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	avatars := &fakeAvatarStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, avatars: avatars}).Engine()
	w := &feedWorld{t: t, engine: engine, clk: clk}
	a := w.person("pc_alice", "Alice Ames")
	first := "avatars/" + a.id + "/0123456789abcdef.jpg"
	second := "avatars/" + a.id + "/fedcba9876543210.png"

	w.ok(a, http.MethodPatch, "/v1/me", map[string]any{"avatar_key": first})
	w.ok(a, http.MethodPatch, "/v1/me", map[string]any{"avatar_key": first})
	w.ok(a, http.MethodPatch, "/v1/me", map[string]any{"bio": "Still the first photo."})
	if got := avatars.deletes(); len(got) != 0 {
		t.Fatalf("no photo was replaced, yet the store removed %v", got)
	}
	me := w.ok(a, http.MethodPatch, "/v1/me", map[string]any{"avatar_key": second})
	if me["avatar_url"] != fakePhotoLink(second) {
		t.Fatalf("the answer's photo: %v", me["avatar_url"])
	}
	if got, want := avatars.deletes(), []fakeAvatarDelete{{Owner: uuid.MustParse(a.id), Key: first}}; !slices.Equal(got, want) {
		t.Fatalf("removed %v, want %v", got, want)
	}
}
