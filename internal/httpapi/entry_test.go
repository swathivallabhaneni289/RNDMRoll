package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// --- the fake photo store ---

// fakeEntryPhotoPublicBase is the link prefix the fake store builds links
// with, the same host the avatar fake and the test servers use.
const fakeEntryPhotoPublicBase = "https://cdn.example.test/"

// fakeEntryExtensions is the real store's type map: exact content types only.
var fakeEntryExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// fakeEntryTicket is what the fake store remembers about one ticket it made.
type fakeEntryTicket struct {
	UserID        uuid.UUID
	ContentType   string
	ContentLength int64
	Key           string
}

// fakeEntryPhotoStore is an in-memory storage.EntryPhotoStore. It follows the
// real store's rules (the three image types and the length bound, with the same
// sentinel errors) and hands out keys shaped exactly like real ones,
// entries/<user id>/<16 lowercase hex>.<ext>, a new random one for every
// ticket. PublicURL is fakeEntryPhotoPublicBase plus the key. Every
// DeleteEntryPhoto call is recorded in order, and failDeletesWith makes them
// return an error. It is safe for concurrent use.
type fakeEntryPhotoStore struct {
	mu        sync.Mutex
	issued    []fakeEntryTicket
	deleted   []string
	deleteErr error
}

var _ storage.EntryPhotoStore = (*fakeEntryPhotoStore)(nil)

// newKey makes a key shaped like a real one for userID without recording a
// ticket, so a test can have as many keys as it likes without using up the
// route's 30 tickets a minute. ext is jpg, png or webp.
func (f *fakeEntryPhotoStore) newKey(userID uuid.UUID, ext string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic("fakeEntryPhotoStore: " + err.Error())
	}
	return "entries/" + userID.String() + "/" + hex.EncodeToString(buf) + "." + ext
}

func (f *fakeEntryPhotoStore) PresignEntryPhotoUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*storage.EntryPhotoTicket, error) {
	ext, ok := fakeEntryExtensions[contentType]
	if !ok {
		return nil, storage.ErrEntryPhotoType
	}
	if contentLength <= 0 || contentLength > storage.MaxEntryPhotoBytes {
		return nil, storage.ErrEntryPhotoTooLarge
	}
	key := f.newKey(userID, ext)
	f.mu.Lock()
	f.issued = append(f.issued, fakeEntryTicket{UserID: userID, ContentType: contentType, ContentLength: contentLength, Key: key})
	f.mu.Unlock()
	return &storage.EntryPhotoTicket{
		UploadURL: "https://fake-bucket.example.test/signed-put/" + key + "?sig=abc",
		Key:       key,
		ExpiresIn: 300,
	}, nil
}

func (f *fakeEntryPhotoStore) PublicURL(key string) string {
	return fakeEntryPhotoPublicBase + key
}

func (f *fakeEntryPhotoStore) DeleteEntryPhoto(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, key)
	return f.deleteErr
}

// deletes returns every key DeleteEntryPhoto was called with, in call order.
func (f *fakeEntryPhotoStore) deletes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.deleted)
}

// tickets returns every ticket the store made, in order.
func (f *fakeEntryPhotoStore) tickets() []fakeEntryTicket {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.issued)
}

// failDeletesWith makes every later DeleteEntryPhoto call return err (nil
// stops failing). The call is still recorded.
func (f *fakeEntryPhotoStore) failDeletesWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteErr = err
}

// --- helpers on the spin rig ---

// photoKey asks for an upload ticket as p (a 1 MiB jpeg) through the route and
// returns the key. The key is in p's own folder and new for every call. A
// person gets 30 tickets a minute: use fakeEntryPhotoStore.newKey for more.
func (r *spinRig) photoKey(p spinPerson) string {
	r.t.Helper()
	rec := r.call(http.MethodPost, "/v1/entry-photos/upload-url", p.token, map[string]any{
		"content_type":   "image/jpeg",
		"content_length": 1 << 20,
	})
	if rec.Code != http.StatusOK {
		r.t.Fatalf("POST /v1/entry-photos/upload-url: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := decodeBody(r.t, rec)["photo_key"].(string)
	if key == "" {
		r.t.Fatalf("the ticket has no photo_key: %s", rec.Body.String())
	}
	return key
}

// entryRequest is a valid POST /v1/entries body for forDate and spinSeq: the
// title "Dune", 7 stars (3.5), the reaction "fire", a comment, and one photo
// per key, the first from the camera and the rest from the library. A test
// changes the map for the case it needs.
func entryRequest(forDate string, spinSeq int, keys ...string) map[string]any {
	photos := make([]any, len(keys))
	for i, k := range keys {
		source := "library"
		if i == 0 {
			source = "camera"
		}
		photos[i] = map[string]any{"photo_key": k, "photo_source": source}
	}
	return map[string]any{
		"for_date": forDate,
		"spin_seq": spinSeq,
		"title":    "Dune",
		"stars":    7,
		"reaction": "fire",
		"comment":  "Slow and worth it.",
		"photos":   photos,
	}
}

// postEntry posts a valid entry as p (see entryRequest) and returns the
// answer as it came.
func (r *spinRig) postEntry(p spinPerson, forDate string, spinSeq int, keys ...string) *httptest.ResponseRecorder {
	r.t.Helper()
	return r.call(http.MethodPost, "/v1/entries", p.token, entryRequest(forDate, spinSeq, keys...))
}

// spinEvening sets p's zone to New York, moves the Fixed clock to 8:30 PM on
// date and spins for p. It returns the spin as Today shows it. The clock is
// the whole rig's, so two people share the evening.
func (r *spinRig) spinEvening(p spinPerson, date string) map[string]any {
	r.t.Helper()
	const ny = "America/New_York"
	r.setZone(p, ny)
	r.clockAt(ny, date, 20, 30, 0)
	rec := r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{"for_date": date})
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		r.t.Fatalf("POST /v1/spin: %d %s", rec.Code, rec.Body.String())
	}
	spun, _ := dig(r.t, decodeBody(r.t, rec), "today", "spin").(map[string]any)
	if spun == nil {
		r.t.Fatalf("no spin after spinning: %s", rec.Body.String())
	}
	return spun
}

// entrySortedKeys returns the keys of a decoded JSON object, sorted.
func entrySortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// entryObject reads a decoded JSON value as an object, or fails the test.
func entryObject(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not a JSON object: %T %v", v, v)
	}
	return m
}

// --- smoke tests: the whole path through the real routes ---

// entryKeysWant is the exact member list of an Entry in every answer.
var entryKeysWant = []string{
	"archived", "category", "comment", "comments_off", "created_at", "id", "late", "local_date", "photo_source",
	"photo_url", "photos", "reaction", "stars", "title", "updated_at",
}

func TestEntryFullDayThroughTheRoutes(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("entry-a@example.com", "entry_a")
	const day = "2026-10-07"
	spun := r.spinEvening(a, day)
	seq := int(spun["seq"].(float64))
	store := r.rig.entryPhotos

	// No entry yet: Today shows the result and a null entry.
	before := r.today(a)
	if before["state"] != "result" || before["entry"] != nil {
		t.Fatalf("before posting: state %v entry %v", before["state"], before["entry"])
	}

	// A ticket, then the post.
	key := r.photoKey(a)
	if !regexp.MustCompile(`^entries/` + a.id.String() + `/[0-9a-f]{16}\.jpg$`).MatchString(key) {
		t.Fatalf("the ticket key is not in the caller's folder: %q", key)
	}
	rec := r.postEntry(a, day, seq, key)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/entries: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "photo_key") || strings.Contains(rec.Body.String(), "storage_key") {
		t.Fatalf("a storage key is named in the answer: %s", rec.Body.String())
	}
	created := decodeBody(t, rec)
	if created["already"] != false {
		t.Fatalf("already = %v, want false", created["already"])
	}
	link := store.PublicURL(key)
	entry := entryObject(t, created["entry"])
	if got := entrySortedKeys(entry); !slices.Equal(got, entryKeysWant) {
		t.Fatalf("entry members = %v, want %v", got, entryKeysWant)
	}
	if entry["title"] != "Dune" || entry["stars"] != float64(7) || entry["reaction"] != "fire" || entry["comment"] != "Slow and worth it." {
		t.Fatalf("entry fields: %v", entry)
	}
	if entry["local_date"] != day || entry["late"] != false || entry["photo_url"] != link || entry["photo_source"] != "camera" {
		t.Fatalf("entry date, late, cover: %v", entry)
	}
	if _, err := uuid.Parse(entry["id"].(string)); err != nil {
		t.Fatalf("entry id %v is not a UUID", entry["id"])
	}
	if dig(t, entry, "category", "name") != dig(t, spun, "category", "name") || dig(t, entry, "category", "id") != dig(t, spun, "category", "id") {
		t.Fatalf("entry category %v is not the spin's %v", entry["category"], spun["category"])
	}
	photos := entry["photos"].([]any)
	if len(photos) != 1 {
		t.Fatalf("photos = %v, want one", photos)
	}
	first := entryObject(t, photos[0])
	if got := entrySortedKeys(first); !slices.Equal(got, []string{"id", "position", "url"}) {
		t.Fatalf("photo members = %v", got)
	}
	if first["url"] != link || first["position"] != float64(0) {
		t.Fatalf("photo = %v, want url %s at position 0", first, link)
	}

	// The post answer's Today, and a fresh GET /v1/today (which goes through
	// the spin handler, so it proves NewServer gave it the photo links).
	for label, today := range map[string]map[string]any{
		"the create answer": entryObject(t, created["today"]),
		"GET /v1/today":     r.today(a),
	} {
		if today["state"] != "posted" {
			t.Fatalf("%s: state = %v, want posted", label, today["state"])
		}
		sum := entryObject(t, today["entry"])
		if sum["photo_url"] != link || sum["photo_count"] != float64(1) || sum["title"] != "Dune" ||
			sum["stars"] != float64(7) || sum["reaction"] != "fire" || sum["late"] != false || sum["id"] != entry["id"] {
			t.Fatalf("%s: entry summary = %v, want the cover link %s", label, sum, link)
		}
		if _, leaked := sum["cover_key"]; leaked || strings.Contains(strings.Join(entrySortedKeys(sum), ","), "key") {
			t.Fatalf("%s: the summary names a key: %v", label, sum)
		}
		if dig(t, today, "week", "posted") != float64(1) {
			t.Fatalf("%s: week.posted = %v, want 1", label, dig(t, today, "week", "posted"))
		}
	}

	// The same post again: 200 and already, the same entry.
	rec = r.postEntry(a, day, seq, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("the same post again: %d %s", rec.Code, rec.Body.String())
	}
	again := decodeBody(t, rec)
	if again["already"] != true || dig(t, again, "entry", "id") != entry["id"] {
		t.Fatalf("the same post again: %v", again)
	}

	// GET the entry: the same Entry as the create answer.
	id := entry["id"].(string)
	rec = r.call(http.MethodGet, "/v1/entries/"+id, a.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/entries/%s: %d %s", id, rec.Code, rec.Body.String())
	}
	got := entryObject(t, decodeBody(t, rec)["entry"])
	if got["id"] != id || got["photo_url"] != link || got["title"] != "Dune" || got["created_at"] != entry["created_at"] {
		t.Fatalf("GET entry: %v, want the created one %v", got, entry)
	}

	// The next posting is refused: the day is settled.
	other := r.photoKey(a)
	wantCode(t, r.postEntry(a, day, seq, other), http.StatusConflict, "day_settled")

	// Another person can not read, edit or delete it. (Delete, like post and
	// skip, needs the caller's zone, so B reports one first.)
	b := r.person("entry-b@example.com", "entry_b")
	r.setZone(b, "America/New_York")
	wantCode(t, r.call(http.MethodGet, "/v1/entries/"+id, b.token, nil), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPatch, "/v1/entries/"+id, b.token, map[string]any{"title": "Mine now"}), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPost, "/v1/entries/"+id+"/delete", b.token, map[string]any{}), http.StatusNotFound, "not_found")
	if now := entryObject(t, decodeBody(t, r.call(http.MethodGet, "/v1/entries/"+id, a.token, nil))["entry"]); now["title"] != "Dune" {
		t.Fatalf("another person changed the entry: %v", now)
	}

	// Delete: Today stays posted with a null entry, and the file goes.
	rec = r.call(http.MethodPost, "/v1/entries/"+id+"/delete", a.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	deleted := entryObject(t, decodeBody(t, rec)["today"])
	if deleted["state"] != "posted" || deleted["entry"] != nil {
		t.Fatalf("after the delete: state %v entry %v", deleted["state"], deleted["entry"])
	}
	if got := store.deletes(); !slices.Equal(got, []string{key}) {
		t.Fatalf("removed files = %v, want [%s]", got, key)
	}
	wantCode(t, r.call(http.MethodGet, "/v1/entries/"+id, a.token, nil), http.StatusNotFound, "not_found")
	wantCode(t, r.postEntry(a, day, seq, r.photoKey(a)), http.StatusConflict, "day_settled")
}

func TestEntryPatchThroughTheRoute(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("entry-patch@example.com", "entry_patch")
	const day = "2026-10-07"
	spun := r.spinEvening(a, day)
	store := r.rig.entryPhotos

	first, second := store.newKey(a.id, "jpg"), store.newKey(a.id, "png")
	rec := r.postEntry(a, day, int(spun["seq"].(float64)), first, second)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	entry := entryObject(t, decodeBody(t, rec)["entry"])
	id := entry["id"].(string)
	photos := entry["photos"].([]any)
	if len(photos) != 2 || dig(t, photos[1], "url") != store.PublicURL(second) {
		t.Fatalf("created photos: %v", photos)
	}
	secondID := dig(t, photos[1], "id").(string)

	// Change a field, drop the first photo, keep the second and add a new
	// one. Only the dropped file is removed.
	third := store.newKey(a.id, "webp")
	rec = r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{
		"title":   "Dune: Part Two",
		"stars":   9,
		"comment": "",
		"photos": []any{
			map[string]any{"id": secondID},
			map[string]any{"photo_key": third, "photo_source": "library"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	patched := entryObject(t, decodeBody(t, rec)["entry"])
	if patched["title"] != "Dune: Part Two" || patched["stars"] != float64(9) || patched["comment"] != nil || patched["reaction"] != "fire" {
		t.Fatalf("patched fields: %v", patched)
	}
	list := patched["photos"].([]any)
	if len(list) != 2 || dig(t, list[0], "id") != secondID || dig(t, list[0], "url") != store.PublicURL(second) ||
		dig(t, list[1], "url") != store.PublicURL(third) || dig(t, list[1], "position") != float64(1) {
		t.Fatalf("patched photos: %v", list)
	}
	if patched["photo_url"] != store.PublicURL(second) {
		t.Fatalf("photo_url = %v, want the new first photo's link", patched["photo_url"])
	}
	if got := store.deletes(); !slices.Equal(got, []string{first}) {
		t.Fatalf("removed files = %v, want only [%s]", got, first)
	}
	sum := entryObject(t, r.today(a)["entry"])
	if sum["photo_url"] != store.PublicURL(second) || sum["photo_count"] != float64(2) || sum["title"] != "Dune: Part Two" {
		t.Fatalf("Today after the edit: %v", sum)
	}

	// An empty body changes nothing and removes nothing.
	rec = r.raw(http.MethodPatch, "/v1/entries/"+id, a.token, "", false)
	if rec.Code != http.StatusOK || entryObject(t, decodeBody(t, rec)["entry"])["updated_at"] != patched["updated_at"] {
		t.Fatalf("empty PATCH: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.deletes(); len(got) != 1 {
		t.Fatalf("an empty PATCH removed files: %v", got)
	}

	// A store that fails to remove a file does not fail the request.
	store.failDeletesWith(errors.New("storage is down"))
	rec = r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{
		"photos": []any{map[string]any{"id": secondID}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH while the store fails: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.deletes(); !slices.Equal(got, []string{first, third}) {
		t.Fatalf("removed files = %v, want [%s %s]", got, first, third)
	}
	if n := len(entryObject(t, decodeBody(t, rec)["entry"])["photos"].([]any)); n != 1 {
		t.Fatalf("photos after the last edit: %d, want 1", n)
	}

	// The photo list is refused by name and with its fixed text.
	for _, c := range []struct {
		name   string
		photos any
		text   string
	}{
		{"empty list", []any{}, "Add at least one photo."},
		{"another folder", []any{map[string]any{"photo_key": store.newKey(uuid.New(), "jpg"), "photo_source": "camera"}}, "One of the photos can't be used."},
		{"both id and key", []any{map[string]any{"id": secondID, "photo_key": store.newKey(a.id, "jpg"), "photo_source": "camera"}}, "One of the photos can't be used."},
		{"neither", []any{map[string]any{"photo_source": "camera"}}, "One of the photos can't be used."},
		{"not an object", []any{"x"}, "One of the photos can't be used."},
		{"not a list", map[string]any{"id": secondID}, "One of the photos can't be used."},
		{"an id that is not a photo of the entry", []any{map[string]any{"id": uuid.NewString()}}, "One of the photos can't be used."},
	} {
		rec := r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{"photos": c.photos})
		body := wantCode(t, rec, http.StatusBadRequest, "validation_failed")
		if body["field"] != "photos" || body["message"] != c.text {
			t.Fatalf("%s: %v, want field photos and %q", c.name, body, c.text)
		}
	}
}

func TestEntryTicketRouteIssuesAKeyInTheCallersFolder(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("entry-ticket@example.com", "entry_ticket")
	store := r.rig.entryPhotos

	keys := map[string]bool{}
	for contentType, ext := range map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"} {
		rec := r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": contentType, "content_length": 2345678})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", contentType, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		key, _ := body["photo_key"].(string)
		if !storage.EntryPhotoKeyOwnedBy(key, a.id) || !strings.HasSuffix(key, "."+ext) {
			t.Fatalf("%s: key %q is not a %s key in the caller's folder", contentType, key, ext)
		}
		if upload, _ := body["upload_url"].(string); upload == "" || body["expires_in"] != float64(300) {
			t.Fatalf("%s: ticket = %v", contentType, body)
		}
		if keys[key] {
			t.Fatalf("a key was handed out twice: %s", key)
		}
		keys[key] = true
	}
	if got := store.tickets(); len(got) != 3 || got[0].UserID != a.id || got[0].ContentLength != 2345678 {
		t.Fatalf("tickets the store saw: %v", got)
	}

	// The refusals name their field.
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/gif", "content_length": 100}), "content_type")
	// A member of the wrong JSON type is named too.
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": 5, "content_length": 100}), "content_type")
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/jpeg", "content_length": "100"}), "content_length")
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_length": 100}), "content_type")
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/jpeg", "content_length": 0}), "content_length")
	wantField(t, r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/jpeg"}), "content_length")
	over := r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/jpeg", "content_length": storage.MaxEntryPhotoBytes + 1})
	wantField(t, over, "content_length")
	if got := decodeBody(t, over)["message"]; got != "This value is too long." {
		t.Fatalf("over the limit: message %v", got)
	}
	atLimit := r.call(http.MethodPost, "/v1/entry-photos/upload-url", a.token, map[string]any{"content_type": "image/jpeg", "content_length": storage.MaxEntryPhotoBytes})
	if atLimit.Code != http.StatusOK {
		t.Fatalf("exactly 50 MiB: %d %s", atLimit.Code, atLimit.Body.String())
	}
}

func TestSkipThroughTheRoute(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("skip-a@example.com", "skip_a")
	b := r.person("skip-b@example.com", "skip_b")
	const day = "2026-10-07"

	// B never spun: there is nothing of B's to skip, whatever A did.
	spun := r.spinEvening(a, day)
	seq := int(spun["seq"].(float64))
	r.setZone(b, "America/New_York")
	wantCode(t, r.call(http.MethodPost, "/v1/spin/skip", b.token, map[string]any{"for_date": day, "spin_seq": seq}), http.StatusConflict, "no_spin")

	// The fields are named.
	wantField(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"spin_seq": seq}), "for_date")
	wantField(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": "tomorrow", "spin_seq": seq}), "for_date")
	wantField(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": day}), "spin_seq")
	wantField(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": day, "spin_seq": -1}), "spin_seq")
	wantField(t, r.raw(http.MethodPost, "/v1/spin/skip", a.token, "", false), "for_date")

	// A wrong spin number, a date with no spin.
	wantCode(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": day, "spin_seq": seq + 1}), http.StatusConflict, "spin_changed")
	wantCode(t, r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": "2026-10-06", "spin_seq": 0}), http.StatusConflict, "no_spin")

	// The skip: 200 either way, the second one is "already".
	rec := r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": day, "spin_seq": seq})
	if rec.Code != http.StatusOK {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	today := entryObject(t, body["today"])
	if body["already"] != false || today["state"] != "skipped" || today["entry"] != nil {
		t.Fatalf("skip answer: %v", body)
	}
	week := entryObject(t, today["week"])
	days := week["days"].([]any)
	if dig(t, days[int(week["today_index"].(float64))], "skipped") != true || week["posted"] != float64(0) {
		t.Fatalf("the week after the skip: %v", week)
	}
	rec = r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": day, "spin_seq": seq})
	if again := decodeBody(t, rec); rec.Code != http.StatusOK || again["already"] != true || dig(t, again, "today", "state") != "skipped" {
		t.Fatalf("skip again: %d %v", rec.Code, again)
	}

	// A post after the skip is refused, and the day stays skipped.
	wantCode(t, r.postEntry(a, day, seq, r.photoKey(a)), http.StatusConflict, "day_settled")
	if got := r.today(a)["state"]; got != "skipped" {
		t.Fatalf("state after the refused post = %v", got)
	}

	// B's own day is not A's: B spins, skips, and A is untouched.
	r.spinEvening(b, day)
	rec = r.call(http.MethodPost, "/v1/spin/skip", b.token, map[string]any{"for_date": day, "spin_seq": 0})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["already"] != false {
		t.Fatalf("B's skip: %d %s", rec.Code, rec.Body.String())
	}
	if got := r.today(a)["state"]; got != "skipped" {
		t.Fatalf("A's state = %v", got)
	}
}

// TestEntryIntegration_PostReadAndSkipOverPostgres runs the same wiring on
// the real repository and the integration builder, which hands the entry
// handler a fake photo store the test holds.
func TestEntryIntegration_PostReadAndSkipOverPostgres(t *testing.T) {
	pool := requireIntegrationPool(t)
	const ny, day = "America/New_York", "2026-10-07"

	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	store := &fakeEntryPhotoStore{}
	server := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: store})
	engine := server.Engine()

	a := integrationOnboardAccount(t, engine, "int-entry-a@example.com", "Correct-horse-battery-staple1", "Entry A", "int_entry_a")
	b := integrationOnboardAccount(t, engine, "int-entry-b@example.com", "Correct-horse-battery-staple1", "Entry B", "int_entry_b")
	do := func(method, path, token string, body any) *httptest.ResponseRecorder {
		t.Helper()
		return integrationDo(t, engine, method, path, token, body)
	}
	for _, s := range []integrationSession{a, b} {
		if rec := do(http.MethodPost, "/v1/time-zone", s.AccessToken, map[string]any{"time_zone": ny}); rec.Code != http.StatusOK {
			t.Fatalf("time zone: %d %s", rec.Code, rec.Body.String())
		}
	}
	clk.Set(localInstant(t, ny, day, 20, 30, 0))
	for _, s := range []integrationSession{a, b} {
		if rec := do(http.MethodPost, "/v1/spin", s.AccessToken, map[string]any{"for_date": day}); rec.Code != http.StatusCreated {
			t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
		}
	}

	// A ticket, a post with two photos, and the reads.
	var keys []string
	for i := 0; i < 2; i++ {
		rec := do(http.MethodPost, "/v1/entry-photos/upload-url", a.AccessToken, map[string]any{"content_type": "image/jpeg", "content_length": 5000})
		if rec.Code != http.StatusOK {
			t.Fatalf("ticket: %d %s", rec.Code, rec.Body.String())
		}
		keys = append(keys, decodeBody(t, rec)["photo_key"].(string))
	}
	rec := do(http.MethodPost, "/v1/entries", a.AccessToken, entryRequest(day, 0, keys...))
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	created := decodeBody(t, rec)
	entry := entryObject(t, created["entry"])
	id := entry["id"].(string)
	if entry["photo_url"] != store.PublicURL(keys[0]) || len(entry["photos"].([]any)) != 2 {
		t.Fatalf("created entry: %v", entry)
	}
	today := entryObject(t, decodeBody(t, do(http.MethodGet, "/v1/today", a.AccessToken, nil)))
	if sum := entryObject(t, today["entry"]); today["state"] != "posted" || sum["photo_url"] != store.PublicURL(keys[0]) || sum["photo_count"] != float64(2) {
		t.Fatalf("today: %v", today)
	}
	rec = do(http.MethodGet, "/v1/entries/"+id, a.AccessToken, nil)
	if rec.Code != http.StatusOK || dig(t, decodeBody(t, rec), "entry", "id") != id {
		t.Fatalf("GET entry: %d %s", rec.Code, rec.Body.String())
	}
	wantCode(t, do(http.MethodGet, "/v1/entries/"+id, b.AccessToken, nil), http.StatusNotFound, "not_found")
	// B can not use A's key either.
	wantField(t, do(http.MethodPost, "/v1/entries", b.AccessToken, entryRequest(day, 0, keys[0])), "photos")

	// B skips; A deletes, and the files go.
	rec = do(http.MethodPost, "/v1/spin/skip", b.AccessToken, map[string]any{"for_date": day, "spin_seq": 0})
	if rec.Code != http.StatusOK || dig(t, decodeBody(t, rec), "today", "state") != "skipped" {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPost, "/v1/entries/"+id+"/delete", a.AccessToken, map[string]any{})
	if rec.Code != http.StatusOK || dig(t, decodeBody(t, rec), "today", "entry") != nil {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.deletes(); !slices.Equal(got, keys) {
		t.Fatalf("removed files = %v, want %v", got, keys)
	}
}

// --- helpers for the contract tests (this file and entry_flow_test.go) ---

// entryPersonCounter numbers the people anyone makes, so no two tests (and no
// two cases of one table) share an account, a route limit or a spin.
var entryPersonCounter atomic.Int64

// eAcute is the letter e with an acute accent (two bytes, one character). It is
// made from its code point so no test file holds non-ASCII text, and the
// rune-count limits of a title and a comment are tested with characters that
// are wider than one byte.
var eAcute = string(rune(0xE9))

// anyone makes a finished account of its own, with no zone and no spin. Route
// limits are per person and count refused requests too, so a table of
// refusals gives every case its own person.
func (r *spinRig) anyone() spinPerson {
	r.t.Helper()
	n := entryPersonCounter.Add(1)
	return r.person(fmt.Sprintf("entry-anyone-%d@example.com", n), fmt.Sprintf("entry_anyone_%d", n))
}

// dayOf makes a person of their own and spins for them at 8:30 PM on date (the
// shared Fixed clock is left there). It returns the person and the number of
// the spin they saw.
func (r *spinRig) dayOf(date string) (spinPerson, int) {
	r.t.Helper()
	p := r.anyone()
	return p, int(r.spinEvening(p, date)["seq"].(float64))
}

// earnBonus gives p a held bonus spin with four pretend posted days, Monday to
// Thursday of the week of 2026-10-09. The shared clock is left at noon on that
// Thursday. p must be on the rig's test tools list.
func (r *spinRig) earnBonus(p spinPerson) {
	r.t.Helper()
	const ny = "America/New_York"
	r.setZone(p, ny)
	r.clockAt(ny, "2026-10-08", 12, 0, 0)
	rec := r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{
		"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"},
	})
	if rec.Code != http.StatusOK || decodeBody(r.t, rec)["bonus_held"] != true {
		r.t.Fatalf("POST /v1/test/posted-days: %d %s, want a held bonus", rec.Code, rec.Body.String())
	}
}

// created posts a valid entry (see entryRequest) as p and returns the whole
// answer. The call fails unless the answer is 201.
func (r *spinRig) created(p spinPerson, date string, seq int, keys ...string) map[string]any {
	r.t.Helper()
	rec := r.postEntry(p, date, seq, keys...)
	if rec.Code != http.StatusCreated {
		r.t.Fatalf("POST /v1/entries: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(r.t, rec)
	wantEntryShape(r.t, entryObject(r.t, body["entry"]))
	return body
}

// entryOf reads GET /v1/entries/{id} as p and returns the entry.
func (r *spinRig) entryOf(p spinPerson, id string) map[string]any {
	r.t.Helper()
	rec := r.call(http.MethodGet, "/v1/entries/"+id, p.token, nil)
	if rec.Code != http.StatusOK {
		r.t.Fatalf("GET /v1/entries/%s: %d %s", id, rec.Code, rec.Body.String())
	}
	entry := entryObject(r.t, decodeBody(r.t, rec)["entry"])
	wantEntryShape(r.t, entry)
	return entry
}

// patched sends PATCH /v1/entries/{id} as p and returns the entry of the 200
// answer. The call fails on any other answer.
func (r *spinRig) patched(p spinPerson, id string, body any) map[string]any {
	r.t.Helper()
	rec := r.call(http.MethodPatch, "/v1/entries/"+id, p.token, body)
	if rec.Code != http.StatusOK {
		r.t.Fatalf("PATCH /v1/entries/%s: %d %s", id, rec.Code, rec.Body.String())
	}
	entry := entryObject(r.t, decodeBody(r.t, rec)["entry"])
	wantEntryShape(r.t, entry)
	return entry
}

// wantEntryShape fails unless an Entry has exactly the members of the contract
// (so no storage key under any name) and every photo exactly its id, url and
// position, one to ten of them.
func wantEntryShape(t *testing.T, entry map[string]any) {
	t.Helper()
	if got := entrySortedKeys(entry); !slices.Equal(got, entryKeysWant) {
		t.Fatalf("entry members = %v, want %v", got, entryKeysWant)
	}
	photos, _ := entry["photos"].([]any)
	if len(photos) < 1 || len(photos) > 10 {
		t.Fatalf("an entry has %d photos, want 1 to 10: %v", len(photos), entry)
	}
	for _, item := range photos {
		if got := entrySortedKeys(entryObject(t, item)); !slices.Equal(got, []string{"id", "position", "url"}) {
			t.Fatalf("photo members = %v", got)
		}
	}
}

// photoIDs lists the ids of an entry's photos in the order they are listed.
func photoIDs(t *testing.T, entry map[string]any) []string {
	t.Helper()
	list, _ := entry["photos"].([]any)
	ids := make([]string, len(list))
	for i, item := range list {
		ids[i], _ = dig(t, item, "id").(string)
	}
	return ids
}

// photoLinks lists the url of every photo of an entry in the order listed.
func photoLinks(t *testing.T, entry map[string]any) []string {
	t.Helper()
	list, _ := entry["photos"].([]any)
	links := make([]string, len(list))
	for i, item := range list {
		links[i], _ = dig(t, item, "url").(string)
	}
	return links
}

// linksOf is the list of links the fake store gives to keys, in order.
func (f *fakeEntryPhotoStore) linksOf(keys ...string) []string {
	links := make([]string, len(keys))
	for i, k := range keys {
		links[i] = f.PublicURL(k)
	}
	return links
}

// keysOfLinks turns the links of an entry back into the keys behind them.
func keysOfLinks(links []string) []string {
	keys := make([]string, len(links))
	for i, l := range links {
		keys[i] = strings.TrimPrefix(l, fakeEntryPhotoPublicBase)
	}
	return keys
}

// keepPhotos names kept photos by id, for the photos list of a PATCH.
func keepPhotos(ids ...string) []any {
	list := make([]any, len(ids))
	for i, id := range ids {
		list[i] = map[string]any{"id": id}
	}
	return list
}

// newPhotoItem names a new upload (from the library) for a photos list.
func newPhotoItem(key string) map[string]any {
	return map[string]any{"photo_key": key, "photo_source": "library"}
}

// instantOf reads an RFC 3339 member of a response.
func instantOf(t *testing.T, v any) time.Time {
	t.Helper()
	s, _ := v.(string)
	got, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("%v is not an instant: %v", v, err)
	}
	return got
}

// weekDay returns the item of week.days in Today for date.
func weekDay(t *testing.T, today map[string]any, date string) map[string]any {
	t.Helper()
	days, _ := dig(t, today, "week", "days").([]any)
	for _, d := range days {
		if m, ok := d.(map[string]any); ok && m["date"] == date {
			return m
		}
	}
	t.Fatalf("no week day %s in %v", date, days)
	return nil
}

// problemText is the fixed message of a field refusal: the texts for the
// photo list are its own, every other field uses the shared ones.
func problemText(field, reason string) string {
	if field == "photos" {
		switch reason {
		case "required":
			return "Add at least one photo."
		case "too_many":
			return "You can add up to 10 photos."
		}
		return "One of the photos can't be used."
	}
	switch reason {
	case "required":
		return "This field is required."
	case "too_long":
		return "This value is too long."
	}
	return "This value is not valid."
}

// wantProblem expects the 400 answer for field with the fixed text of reason.
func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, field, reason string) {
	t.Helper()
	body := wantCode(t, rec, http.StatusBadRequest, "validation_failed")
	if body["field"] != field || body["message"] != problemText(field, reason) {
		t.Fatalf("answer = %v, want field %q with the text of %q (%q)", body, field, reason, problemText(field, reason))
	}
}

// entrySummary is what the entry member of Today must say.
type entrySummary struct {
	title    string
	stars    float64
	reaction any // a reaction name, or nil for none
	count    float64
	cover    string // the cover photo's link
}

// wantEntrySummary expects today to be posted with exactly the summary want:
// the seven members, the cover as a link and never a bare key.
func wantEntrySummary(t *testing.T, label string, today map[string]any, want entrySummary) {
	t.Helper()
	if today["state"] != "posted" {
		t.Fatalf("%s: state = %v, want posted", label, today["state"])
	}
	sum, ok := today["entry"].(map[string]any)
	if !ok {
		t.Fatalf("%s: entry = %v, want the summary", label, today["entry"])
	}
	if got := entrySortedKeys(sum); !slices.Equal(got, []string{"archived", "comments_off", "id", "late", "photo_count", "photo_url", "reaction", "stars", "title"}) {
		t.Fatalf("%s: summary members = %v", label, got)
	}
	if sum["title"] != want.title || sum["stars"] != want.stars || sum["reaction"] != want.reaction ||
		sum["photo_count"] != want.count || sum["photo_url"] != want.cover {
		t.Fatalf("%s: summary = %v, want %+v", label, sum, want)
	}
	if !strings.HasPrefix(want.cover, fakeEntryPhotoPublicBase+"entries/") {
		t.Fatalf("%s: the expected cover %q is not a link", label, want.cover)
	}
}

// --- every route: no token, deleted user, unfinished account, size cap, wrong type ---

// entryRoute is one route of the entry contract with a body that is fine for
// it and a body with a member of the wrong JSON type. field is the name the
// answer to that body carries ("" when the body is not an object, so none can
// be named). A route with an {id} gets a canonical UUID that names nothing:
// the id check passes and the body is what is under test.
type entryRoute struct {
	name, method, path string
	body, wrongType    any
	field              string
}

func entryRoutes() []entryRoute {
	id := uuid.NewString()
	return []entryRoute{
		{"ticket", http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 1000}, map[string]any{"content_type": 5}, "content_type"},
		{"create", http.MethodPost, "/v1/entries", map[string]any{"for_date": "2026-10-07", "spin_seq": 0}, map[string]any{"for_date": 5}, "for_date"},
		{"get", http.MethodGet, "/v1/entries/" + id, nil, nil, ""},
		{"patch", http.MethodPatch, "/v1/entries/" + id, map[string]any{"title": "x"}, map[string]any{"title": 5}, "title"},
		{"delete", http.MethodPost, "/v1/entries/" + id + "/delete", map[string]any{}, []any{1}, ""},
		{"skip", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": "2026-10-07", "spin_seq": 0}, map[string]any{"for_date": 5}, "for_date"},
	}
}

func TestEntryRoutesRefuseNoTokenDeletedAndUnfinishedAccounts(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	gone := r.person("entry-gone@example.com", "entry_gone")
	r.users.remove(t, gone.id)
	unfinished, err := r.users.Create(context.Background(), "entry-unfinished@example.com", nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	unfinishedToken, err := auth.IssueAccessToken(unfinished.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range entryRoutes() {
		t.Run(route.name, func(t *testing.T) {
			wantCode(t, r.call(route.method, route.path, "", route.body), http.StatusUnauthorized, "token_invalid")
			wantCode(t, r.call(route.method, route.path, gone.token, route.body), http.StatusUnauthorized, "token_invalid")
			wantCode(t, r.call(route.method, route.path, unfinishedToken, route.body), http.StatusForbidden, "profile_incomplete")
		})
	}
}

func TestEntryRoutesRefuseOversizeMalformedAndWrongTypedBodies(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("entry-sizes@example.com", "entry_sizes")
	big := `{"title":"` + strings.Repeat("a", 20<<10) + `"}`

	for _, route := range entryRoutes() {
		if route.method == http.MethodGet {
			continue // a GET has no body to size or type
		}
		t.Run(route.name, func(t *testing.T) {
			// Over 16 KB, declared and chunked.
			for _, chunked := range []bool{false, true} {
				wantCode(t, r.raw(route.method, route.path, p.token, big, chunked), http.StatusRequestEntityTooLarge, "payload_too_large")
			}
			// A member of the wrong type is named; a body that is not an object
			// is the generic answer.
			body := wantCode(t, r.call(route.method, route.path, p.token, route.wrongType), http.StatusBadRequest, "validation_failed")
			if route.field == "" && body["field"] != nil {
				t.Fatalf("a body that is not an object named a field: %v", body)
			}
			if route.field != "" && (body["field"] != route.field || body["message"] != problemText(route.field, "invalid")) {
				t.Fatalf("wrong type: %v, want field %s", body, route.field)
			}
			// Malformed JSON.
			wantCode(t, r.raw(route.method, route.path, p.token, `{"oops":`, false), http.StatusBadRequest, "validation_failed")
		})
	}
}

func TestEntryUnknownAndMalformedIdsAreNotFound(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	// The person has a zone, so even the delete route (which builds Today)
	// reaches its id check.
	p, _ := r.dayOf("2026-10-07")
	for name, id := range map[string]string{
		"unknown":               uuid.NewString(),
		"not a uuid":            "not-a-uuid",
		"a number":              "12345",
		"36 characters, not id": strings.Repeat("z", 36),
	} {
		t.Run(name, func(t *testing.T) {
			path := "/v1/entries/" + id
			wantCode(t, r.call(http.MethodGet, path, p.token, nil), http.StatusNotFound, "not_found")
			wantCode(t, r.call(http.MethodPatch, path, p.token, map[string]any{"title": "x"}), http.StatusNotFound, "not_found")
			wantCode(t, r.call(http.MethodPost, path+"/delete", p.token, map[string]any{}), http.StatusNotFound, "not_found")
		})
	}
}

func TestEntryAnotherPersonCanNeverReadEditDeleteOrUseAnEntry(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	store := r.rig.entryPhotos
	a, seqA := r.dayOf(day)
	b, seqB := r.dayOf(day) // B has a zone and a spin too, so delete reaches its id check
	aKey := store.newKey(a.id, "jpg")
	id := entryObject(t, r.created(a, day, seqA, aKey)["entry"])["id"].(string)
	before := r.entryOf(a, id)

	// B reads, edits and deletes A's entry by its id: 404 each time.
	path := "/v1/entries/" + id
	wantCode(t, r.call(http.MethodGet, path, b.token, nil), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPatch, path, b.token, map[string]any{"title": "Mine now", "stars": 1}), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPatch, path, b.token, map[string]any{"photos": []any{newPhotoItem(store.newKey(b.id, "jpg"))}}), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPost, path+"/delete", b.token, map[string]any{}), http.StatusNotFound, "not_found")

	// A's entry is exactly as it was, still posted, and no file was removed.
	if after := r.entryOf(a, id); !reflect.DeepEqual(after, before) {
		t.Fatalf("A's entry changed:\n before %v\n after  %v", before, after)
	}
	today := r.today(a)
	if today["state"] != "posted" || dig(t, today, "entry", "id") != id {
		t.Fatalf("A's today after B's attempts: %v", today)
	}
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("files were removed: %v", got)
	}

	// B can never post with A's photo key, whether A's entry holds it or not.
	wantProblem(t, r.postEntry(b, day, seqB, aKey), "photos", "invalid")
	bID := entryObject(t, r.created(b, day, seqB, store.newKey(b.id, "jpg"))["entry"])["id"].(string)
	// ... nor add it to an entry of their own.
	rec := r.call(http.MethodPatch, "/v1/entries/"+bID, b.token, map[string]any{"photos": []any{newPhotoItem(aKey)}})
	wantProblem(t, rec, "photos", "invalid")
}

// --- the photo ticket ---

func TestEntryTicketAcceptsTheThreeImageTypesInTheCallersFolder(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("entry-types@example.com", "entry_types")
	store := r.rig.entryPhotos

	seen := map[string]bool{}
	for _, c := range []struct{ contentType, ext string }{
		{"image/jpeg", "jpg"}, {"image/png", "png"}, {"image/webp", "webp"}, {"image/jpeg", "jpg"},
	} {
		rec := r.call(http.MethodPost, "/v1/entry-photos/upload-url", p.token, map[string]any{"content_type": c.contentType, "content_length": 4096})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.contentType, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if got := entrySortedKeys(body); !slices.Equal(got, []string{"expires_in", "photo_key", "upload_url"}) {
			t.Fatalf("%s: ticket members = %v", c.contentType, got)
		}
		key, _ := body["photo_key"].(string)
		folder := "entries/" + p.id.String() + "/"
		if !strings.HasPrefix(key, folder) || !strings.HasSuffix(key, "."+c.ext) || !storage.EntryPhotoKeyOwnedBy(key, p.id) {
			t.Fatalf("%s: key %q is not a .%s file in %s", c.contentType, key, c.ext, folder)
		}
		if upload, _ := body["upload_url"].(string); upload == "" || body["expires_in"] != float64(300) {
			t.Fatalf("%s: ticket = %v", c.contentType, body)
		}
		// Two tickets, two keys, even for the same type.
		if seen[key] {
			t.Fatalf("a key was handed out twice: %s", key)
		}
		seen[key] = true
	}
	if got := store.tickets(); len(got) != 4 || got[1].ContentType != "image/png" || got[1].ContentLength != 4096 || got[1].UserID != p.id {
		t.Fatalf("what the store was asked: %v", got)
	}
}

func TestEntryTicketRefusesOtherTypesAndBadLengths(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("entry-lengths@example.com", "entry_lengths")
	const mib50 = 50 << 20
	ask := func(contentType string, length int64) *httptest.ResponseRecorder {
		return r.call(http.MethodPost, "/v1/entry-photos/upload-url", p.token, map[string]any{"content_type": contentType, "content_length": length})
	}

	for _, c := range []struct {
		name, contentType string
		length            int64
		field, reason     string
	}{
		{"gif", "image/gif", 1000, "content_type", "invalid"},
		{"heic", "image/heic", 1000, "content_type", "invalid"},
		{"a type with parameters", "image/jpeg; charset=x", 1000, "content_type", "invalid"},
		{"zero length", "image/jpeg", 0, "content_length", "required"},
		{"negative length", "image/jpeg", -1, "content_length", "required"},
		{"50 MiB plus one", "image/jpeg", mib50 + 1, "content_length", "too_long"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantProblem(t, ask(c.contentType, c.length), c.field, c.reason)
		})
	}
	// One byte and exactly 50 MiB are fine.
	for _, length := range []int64{1, mib50} {
		if rec := ask("image/webp", length); rec.Code != http.StatusOK {
			t.Fatalf("length %d: %d %s", length, rec.Code, rec.Body.String())
		}
	}
	// A refusal never reaches the store as a ticket.
	if got := r.rig.entryPhotos.tickets(); len(got) != 2 {
		t.Fatalf("tickets made = %d, want 2", len(got))
	}
}

// --- limits ---

func TestEntryRoutesAreLimitedPerPersonAMinute(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	id := uuid.NewString()
	for _, c := range []struct {
		name, method, path string
		body               any
		limit              int
	}{
		{"ticket", http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 1000}, 30},
		{"create", http.MethodPost, "/v1/entries", map[string]any{}, 10},
		{"get", http.MethodGet, "/v1/entries/" + id, nil, 60},
		{"patch", http.MethodPatch, "/v1/entries/" + id, map[string]any{}, 20},
		{"delete", http.MethodPost, "/v1/entries/" + id + "/delete", map[string]any{}, 20},
		{"skip", http.MethodPost, "/v1/spin/skip", map[string]any{}, 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := r.anyone()
			// The limiter counts requests, answered or refused: refused ones are
			// cheap and use up the same allowance.
			for i := 1; i <= c.limit; i++ {
				if rec := r.call(c.method, c.path, p.token, c.body); rec.Code == http.StatusTooManyRequests {
					t.Fatalf("request %d of %d was limited: %s", i, c.limit, rec.Body.String())
				}
			}
			wantCode(t, r.call(c.method, c.path, p.token, c.body), http.StatusTooManyRequests, "rate_limited")
			// The limit is per person.
			if rec := r.call(c.method, c.path, r.anyone().token, c.body); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("another person was limited: %s", rec.Body.String())
			}
		})
	}
}

// --- create: every field names itself ---

// createRefusal is one way a create can be wrong: mutate changes a valid body
// (own makes a new key in the caller's folder) and the answer must be 400 for
// field with the fixed text of reason.
type createRefusal struct {
	name          string
	mutate        func(body map[string]any, own func() string)
	field, reason string
}

func setMember(name string, v any) func(map[string]any, func() string) {
	return func(b map[string]any, _ func() string) { b[name] = v }
}

func dropMember(name string) func(map[string]any, func() string) {
	return func(b map[string]any, _ func() string) { delete(b, name) }
}

// photosOf sets the photos list to one item per key own makes, or to the items
// given.
func photosOf(items func(own func() string) []any) func(map[string]any, func() string) {
	return func(b map[string]any, own func() string) { b["photos"] = items(own) }
}

// wantCreateRefusals posts, for every case, a valid create changed by the case
// as a person of its own (a refusal is checked before any spin is looked at).
func (r *spinRig) wantCreateRefusals(t *testing.T, cases []createRefusal) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := r.anyone()
			own := func() string { return r.rig.entryPhotos.newKey(p.id, "jpg") }
			body := entryRequest("2026-10-07", 0, own())
			c.mutate(body, own)
			wantProblem(t, r.call(http.MethodPost, "/v1/entries", p.token, body), c.field, c.reason)
		})
	}
}

func TestEntryCreateRefusesABadDateOrSpinNumber(t *testing.T) {
	newSpinRig(t, rigOptions{}).wantCreateRefusals(t, []createRefusal{
		{"for_date is not a date", setMember("for_date", "07/10/2026"), "for_date", "invalid"},
		{"for_date is not a real day", setMember("for_date", "2026-02-30"), "for_date", "invalid"},
		{"for_date is missing", dropMember("for_date"), "for_date", "required"},
		{"spin_seq is missing", dropMember("spin_seq"), "spin_seq", "required"},
		{"spin_seq is negative", setMember("spin_seq", -1), "spin_seq", "invalid"},
		{"spin_seq is text", setMember("spin_seq", "0"), "spin_seq", "invalid"},
	})
}

func TestEntryCreateRefusesABadTitleOrStars(t *testing.T) {
	newSpinRig(t, rigOptions{}).wantCreateRefusals(t, []createRefusal{
		{"title is missing", dropMember("title"), "title", "required"},
		{"title is blank", setMember("title", "   "), "title", "required"},
		{"title is 81 characters", setMember("title", strings.Repeat(eAcute, 81)), "title", "too_long"},
		{"title holds a control character", setMember("title", "Du\u0007ne"), "title", "invalid"},
		{"title is a number", setMember("title", 5), "title", "invalid"},
		{"stars is missing", dropMember("stars"), "stars", "required"},
		{"stars is 0", setMember("stars", 0), "stars", "invalid"},
		{"stars is 11", setMember("stars", 11), "stars", "invalid"},
		{"stars is a fraction", setMember("stars", 3.5), "stars", "invalid"},
		{"stars is text", setMember("stars", "7"), "stars", "invalid"},
	})
}

func TestEntryCreateRefusesABadReactionOrComment(t *testing.T) {
	newSpinRig(t, rigOptions{}).wantCreateRefusals(t, []createRefusal{
		{"reaction heart", setMember("reaction", "heart"), "reaction", "invalid"},
		{"reaction clap", setMember("reaction", "clap"), "reaction", "invalid"},
		{"reaction is a number", setMember("reaction", 5), "reaction", "invalid"},
		{"comment is 141 characters", setMember("comment", strings.Repeat(eAcute, 141)), "comment", "too_long"},
		{"comment holds a control character", setMember("comment", "a\u0007b"), "comment", "invalid"},
		{"comment is a number", setMember("comment", 5), "comment", "invalid"},
	})
}

func TestEntryCreateRefusesABadPhotoList(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	store := r.rig.entryPhotos
	// folderOf is the caller's own folder, taken from a key they could have.
	folderOf := func(own func() string) string {
		k := own()
		return k[:strings.LastIndex(k, "/")]
	}
	item := func(m map[string]any) []any { return []any{m} }

	r.wantCreateRefusals(t, []createRefusal{
		{"no photos member", dropMember("photos"), "photos", "required"},
		{"photos is null", setMember("photos", nil), "photos", "required"},
		{"photos is an empty list", setMember("photos", []any{}), "photos", "required"},
		{"eleven photos", photosOf(func(own func() string) []any {
			list := make([]any, 11)
			for i := range list {
				list[i] = newPhotoItem(own())
			}
			return list
		}), "photos", "too_many"},
		{"a key of another person", photosOf(func(func() string) []any {
			return item(newPhotoItem(store.newKey(uuid.New(), "jpg")))
		}), "photos", "invalid"},
		{"a key that climbs out of the folder", photosOf(func(own func() string) []any {
			return item(newPhotoItem(folderOf(own) + "/../" + uuid.NewString() + "/0123456789abcdef.jpg"))
		}), "photos", "invalid"},
		{"a key with upper case hex", photosOf(func(own func() string) []any {
			return item(newPhotoItem(folderOf(own) + "/0123456789ABCDEF.jpg"))
		}), "photos", "invalid"},
		{"a key with an extension that is not an image", photosOf(func(own func() string) []any {
			return item(newPhotoItem(folderOf(own) + "/0123456789abcdef.gif"))
		}), "photos", "invalid"},
		{"a link instead of a key", photosOf(func(own func() string) []any {
			return item(newPhotoItem(store.PublicURL(own())))
		}), "photos", "invalid"},
		{"the same key twice", photosOf(func(own func() string) []any {
			k := own()
			return []any{newPhotoItem(k), newPhotoItem(k)}
		}), "photos", "invalid"},
		{"an id on a create", photosOf(func(func() string) []any {
			return item(map[string]any{"id": uuid.NewString()})
		}), "photos", "invalid"},
		{"an id beside a key", photosOf(func(own func() string) []any {
			return item(map[string]any{"id": uuid.NewString(), "photo_key": own(), "photo_source": "camera"})
		}), "photos", "invalid"},
		{"an item that is a string", photosOf(func(own func() string) []any { return []any{own()} }), "photos", "invalid"},
		{"an item that is null", photosOf(func(func() string) []any { return []any{nil} }), "photos", "invalid"},
		{"photos is not a list", setMember("photos", "entries/x.jpg"), "photos", "invalid"},
		{"a key that is a number", photosOf(func(func() string) []any {
			return item(map[string]any{"photo_key": 5, "photo_source": "camera"})
		}), "photos", "invalid"},
		{"a source that is not camera or library", photosOf(func(own func() string) []any {
			return item(map[string]any{"photo_key": own(), "photo_source": "gallery"})
		}), "photos", "invalid"},
		{"no source", photosOf(func(own func() string) []any {
			return item(map[string]any{"photo_key": own()})
		}), "photos", "invalid"},
		{"a declared size of zero", photosOf(func(own func() string) []any {
			return item(map[string]any{"photo_key": own(), "photo_source": "camera", "size_bytes": 0})
		}), "photos", "invalid"},
		{"a declared size over 50 MiB", photosOf(func(own func() string) []any {
			return item(map[string]any{"photo_key": own(), "photo_source": "camera", "size_bytes": 52428801})
		}), "photos", "invalid"},
	})
}

func TestEntryCreateAcceptsTenPhotosAndEveryLimitAtItsEdge(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	store := r.rig.entryPhotos
	a, seq := r.dayOf(day)

	exts := []string{"jpg", "png", "webp"}
	keys := make([]string, 10)
	items := make([]any, 10)
	for i := range keys {
		keys[i] = store.newKey(a.id, exts[i%3])
		items[i] = map[string]any{"photo_key": keys[i], "photo_source": "library", "size_bytes": 1000 + i}
	}
	body := entryRequest(day, seq, keys...)
	body["photos"] = items
	body["title"] = strings.Repeat(eAcute, 80)                                       // 80 characters, 160 bytes
	body["stars"] = 10                                                               // the top of the range
	body["comment"] = strings.Repeat(eAcute, 70) + "\n" + strings.Repeat(eAcute, 69) // 140 characters

	rec := r.call(http.MethodPost, "/v1/entries", a.token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ten photos: %d %s", rec.Code, rec.Body.String())
	}
	entry := entryObject(t, decodeBody(t, rec)["entry"])
	if entry["title"] != body["title"] || entry["stars"] != float64(10) || entry["comment"] != body["comment"] {
		t.Fatalf("fields at their limits were changed: %v", entry)
	}
	links := photoLinks(t, entry)
	if !slices.Equal(links, store.linksOf(keys...)) {
		t.Fatalf("photo links = %v, want the store's links for %v, in order", links, keys)
	}
	for i, item := range entry["photos"].([]any) {
		if dig(t, item, "position") != float64(i) || !strings.HasPrefix(links[i], "https://") {
			t.Fatalf("photo %d = %v", i, item)
		}
	}
	if entry["photo_url"] != links[0] || entry["photo_source"] != "library" {
		t.Fatalf("photo_url = %v, want the first photo's link %s", entry["photo_url"], links[0])
	}
	if got := r.entryOf(a, entry["id"].(string)); !reflect.DeepEqual(got, entry) {
		t.Fatalf("GET differs from the create answer:\n create %v\n get    %v", entry, got)
	}

	// An eleventh photo through an edit is refused and changes nothing.
	list := append(keepPhotos(photoIDs(t, entry)...), newPhotoItem(store.newKey(a.id, "jpg")))
	rec = r.call(http.MethodPatch, "/v1/entries/"+entry["id"].(string), a.token, map[string]any{"photos": list})
	wantProblem(t, rec, "photos", "too_many")
	if got := r.entryOf(a, entry["id"].(string)); !reflect.DeepEqual(got, entry) {
		t.Fatalf("a refused edit changed the entry: %v", got)
	}
}
