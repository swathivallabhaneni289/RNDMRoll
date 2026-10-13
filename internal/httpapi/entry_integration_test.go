package httpapi

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// TestEntryIntegration_FullDayThenResetOverPostgres walks one person through
// a day of the entry slice against the real database, through the real server
// with the test tools and a per-account clock: sign up, report the zone, move
// the clock to 8:30 PM, spin, two tickets, a post with two photos, read, edit
// (remove one photo), a skip that is refused after the post, delete (the day
// stays posted) and then the reset, after which the same day can be spun and
// posted again. The rows are read back with SQL so the answers are shown to
// match what the database holds, and the accounts table is shown untouched.
func TestEntryIntegration_FullDayThenResetOverPostgres(t *testing.T) {
	pool := requireIntegrationPool(t)
	ctx := context.Background()
	const email = "int-entry-day@example.com"
	const ny, day = "America/New_York", "2026-10-07"

	store := &fakeEntryPhotoStore{}
	server := buildIntegrationServerWith(t, pool, integrationOptions{
		clock:       clock.NewPerUser(),
		toolEmails:  []string{email},
		entryPhotos: store,
	})
	engine := server.Engine()
	session := integrationOnboardAccount(t, engine, email, "correct-horse-battery-staple", "Entry Day", "entry_day")
	other := integrationOnboardAccount(t, engine, "int-entry-other@example.com", "correct-horse-battery-staple", "Entry Other", "entry_other")
	usersBefore := integrationUserCount(t, pool)
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `select id from users where email = $1`, email).Scan(&userID); err != nil {
		t.Fatalf("find the account: %v", err)
	}

	do := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		rec := integrationDo(t, engine, method, path, session.AccessToken, body)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body.String(), want)
		}
		return decodeBody(t, rec)
	}
	// count is the number of the person's rows in one of the four tables the
	// entry slice writes (each has a user_id column).
	count := func(table string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from `+table+` where user_id = $1`, userID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	wantCounts := func(step string, entries, photos, posted, spins int) {
		t.Helper()
		got := []int{count("entries"), count("entry_photos"), count("posted_days"), count("spins")}
		if want := []int{entries, photos, posted, spins}; !slices.Equal(got, want) {
			t.Fatalf("%s: rows (entries, entry_photos, posted_days, spins) = %v, want %v", step, got, want)
		}
	}
	// spinRow is the result and whether settled_at is set of the person's one
	// spin.
	spinRow := func() (string, bool) {
		t.Helper()
		var result string
		var settled bool
		if err := pool.QueryRow(ctx, `select result, settled_at is not null from spins where user_id = $1`, userID).Scan(&result, &settled); err != nil {
			t.Fatalf("read the spin: %v", err)
		}
		return result, settled
	}
	ticketKey := func() string {
		t.Helper()
		ticket := do(http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000}, http.StatusOK)
		return ticket["photo_key"].(string)
	}

	// The zone, the clock at 8:30 PM, the spin.
	do(http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": ny}, http.StatusOK)
	do(http.MethodPost, "/v1/test/clock", map[string]any{"local": day + "T20:30:00"}, http.StatusOK)
	spun := do(http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusCreated)
	seq := int(dig(t, spun, "today", "spin", "seq").(float64))
	wantCounts("after the spin", 0, 0, 0, 1)
	if result, settled := spinRow(); result != "open" || settled {
		t.Fatalf("the spin after spinning: result %s, settled_at set %v", result, settled)
	}

	// Two tickets and a post with two photos.
	keys := []string{ticketKey(), ticketKey()}
	if keys[0] == keys[1] {
		t.Fatalf("two tickets gave one key: %s", keys[0])
	}
	created := do(http.MethodPost, "/v1/entries", entryRequest(day, seq, keys...), http.StatusCreated)
	entry := entryObject(t, created["entry"])
	id := entry["id"].(string)
	if created["already"] != false || entry["late"] != false || entry["local_date"] != day ||
		!slices.Equal(photoLinks(t, entry), store.linksOf(keys...)) || entry["photo_url"] != store.PublicURL(keys[0]) {
		t.Fatalf("created: %v", created)
	}
	wantCounts("after the post", 1, 2, 1, 1)
	if result, settled := spinRow(); result != "posted" || !settled {
		t.Fatalf("the spin after the post: result %s, settled_at set %v", result, settled)
	}
	var source string
	if err := pool.QueryRow(ctx, `select source from posted_days where user_id = $1`, userID).Scan(&source); err != nil || source != "post" {
		t.Fatalf("the posted day's source = %q (err %v), want post", source, err)
	}
	wantEntrySummary(t, "GET /v1/today after the post", do(http.MethodGet, "/v1/today", nil, http.StatusOK),
		entrySummary{title: "Dune", stars: 7, reaction: "fire", count: 2, cover: store.PublicURL(keys[0])})

	// Read it back: the same entry.
	if got := do(http.MethodGet, "/v1/entries/"+id, nil, http.StatusOK); !reflect.DeepEqual(entryObject(t, got["entry"]), entry) {
		t.Fatalf("GET differs from the create answer:\n create %v\n get    %v", entry, got["entry"])
	}

	// Another account (with a zone, so the delete route reaches its id check)
	// can not read, edit or delete it, and can not post with its photo key.
	byOther := func(method, path string, body any, want int, code string) {
		t.Helper()
		rec := integrationDo(t, engine, method, path, other.AccessToken, body)
		if got := decodeBody(t, rec)["error"]; rec.Code != want || got != code {
			t.Fatalf("other account, %s %s: %d %s, want %d %s", method, path, rec.Code, rec.Body.String(), want, code)
		}
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/time-zone", other.AccessToken, map[string]any{"time_zone": ny}); rec.Code != http.StatusOK {
		t.Fatalf("other account's zone: %d %s", rec.Code, rec.Body.String())
	}
	byOther(http.MethodGet, "/v1/entries/"+id, nil, http.StatusNotFound, "not_found")
	byOther(http.MethodPatch, "/v1/entries/"+id, map[string]any{"title": "Mine now"}, http.StatusNotFound, "not_found")
	byOther(http.MethodPost, "/v1/entries/"+id+"/delete", map[string]any{}, http.StatusNotFound, "not_found")
	byOther(http.MethodPost, "/v1/entries", entryRequest(day, 0, keys[0]), http.StatusBadRequest, "validation_failed")
	wantCounts("after the other account's attempts", 1, 2, 1, 1)
	if got := do(http.MethodGet, "/v1/entries/"+id, nil, http.StatusOK); !reflect.DeepEqual(entryObject(t, got["entry"]), entry) {
		t.Fatalf("the entry changed after another account's attempts: %v", got["entry"])
	}
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("another account's attempts removed files: %v", got)
	}

	// Edit: remove the second photo. Only its file goes.
	ids := photoIDs(t, entry)
	patched := do(http.MethodPatch, "/v1/entries/"+id, map[string]any{"photos": keepPhotos(ids[0])}, http.StatusOK)
	if got := photoLinks(t, entryObject(t, patched["entry"])); !slices.Equal(got, store.linksOf(keys[0])) {
		t.Fatalf("photos after the edit: %v", got)
	}
	if got := store.deletes(); !slices.Equal(got, []string{keys[1]}) {
		t.Fatalf("removed files = %v, want [%s]", got, keys[1])
	}
	wantCounts("after the edit", 1, 1, 1, 1)
	wantEntrySummary(t, "GET /v1/today after the edit", do(http.MethodGet, "/v1/today", nil, http.StatusOK),
		entrySummary{title: "Dune", stars: 7, reaction: "fire", count: 1, cover: store.PublicURL(keys[0])})

	// A skip after the post is refused and changes nothing.
	refused := do(http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": day, "spin_seq": seq}, http.StatusConflict)
	if refused["error"] != "day_settled" {
		t.Fatalf("skip after the post: %v", refused)
	}
	if result, _ := spinRow(); result != "posted" {
		t.Fatalf("the spin after a refused skip: %s", result)
	}

	// Delete: the entry and its photo rows go, the day stays posted.
	deleted := do(http.MethodPost, "/v1/entries/"+id+"/delete", map[string]any{}, http.StatusOK)
	today := entryObject(t, deleted["today"])
	if today["state"] != "posted" || dig(t, today, "entry") != nil || dig(t, today, "week", "posted") != float64(1) {
		t.Fatalf("today after the delete: %v", today)
	}
	if got := store.deletes(); !slices.Equal(got, []string{keys[1], keys[0]}) {
		t.Fatalf("removed files = %v, want [%s %s]", got, keys[1], keys[0])
	}
	wantCounts("after the delete", 0, 0, 1, 1)
	if result, settled := spinRow(); result != "posted" || !settled {
		t.Fatalf("the spin after the delete: result %s, settled_at set %v", result, settled)
	}
	if got := do(http.MethodGet, "/v1/entries/"+id, nil, http.StatusNotFound); got["error"] != "not_found" {
		t.Fatalf("GET after the delete: %v", got)
	}
	if got := do(http.MethodPost, "/v1/entries", entryRequest(day, seq, ticketKey()), http.StatusConflict); got["error"] != "day_settled" {
		t.Fatalf("a post after the delete: %v", got)
	}

	// The reset clears the entries, the posted day and the spin, and the same
	// day works again.
	do(http.MethodPost, "/v1/test/reset", map[string]any{}, http.StatusOK)
	wantCounts("after the reset", 0, 0, 0, 0)
	reset := do(http.MethodGet, "/v1/today", nil, http.StatusOK)
	if reset["state"] != "open" || reset["spin"] != nil || dig(t, reset, "entry") != nil || dig(t, reset, "week", "posted") != float64(0) {
		t.Fatalf("today after the reset: %v", reset)
	}
	do(http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusCreated)
	again := do(http.MethodPost, "/v1/entries", entryRequest(day, 0, ticketKey()), http.StatusCreated)
	if again["already"] != false || dig(t, again, "entry", "id") == id || dig(t, again, "today", "state") != "posted" {
		t.Fatalf("the post after the reset: %v", again)
	}
	wantCounts("after posting again", 1, 1, 1, 1)

	if n := integrationUserCount(t, pool); n != usersBefore {
		t.Fatalf("user count changed from %d to %d", usersBefore, n)
	}
}
