package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// TestDiaryIntegration_HistoryOverPostgres walks a person through nine days
// (posted, skipped, deleted, archived) and reads the diary and the summary
// back through the real server and database.
func TestDiaryIntegration_HistoryOverPostgres(t *testing.T) {
	pool := requireIntegrationPool(t)
	const email = "int-diary@example.com"
	store := &fakeEntryPhotoStore{}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{
		clock: clock.NewPerUser(), toolEmails: []string{email}, entryPhotos: store,
	}).Engine()
	me := integrationOnboardAccount(t, engine, email, "Correct-horse-battery-staple1", "Diary Person", "diary_person")
	other := integrationOnboardAccount(t, engine, "int-diary-other@example.com", "Correct-horse-battery-staple1", "Diary Other", "diary_other")

	do := func(tok, method, path string, body any, want int) (map[string]any, string) {
		t.Helper()
		rec := integrationDo(t, engine, method, path, tok, body)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body.String(), want)
		}
		return decodeBody(t, rec), rec.Body.String()
	}
	mine := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		m, _ := do(me.AccessToken, method, path, body, want)
		return m
	}

	// Brand new account: zeros and empty lists.
	do(me.AccessToken, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": "America/New_York"}, http.StatusOK)
	_, raw := do(me.AccessToken, http.MethodGet, "/v1/me/summary", nil, http.StatusOK)
	if raw != `{"followers":0,"recent":[],"spins":0,"tastes":[],"week_streak":0}` {
		t.Fatalf("empty summary: %s", raw)
	}
	_, raw = do(me.AccessToken, http.MethodGet, "/v1/diary", nil, http.StatusOK)
	if raw != `{"categories":[],"items":[],"next_cursor":null}` {
		t.Fatalf("empty diary: %s", raw)
	}

	// day spins on `date`, then posts (or skips, or posts and then deletes or archives).
	var firstCategory string
	day := func(date, action string) (id string) {
		t.Helper()
		mine(http.MethodPost, "/v1/test/clock", map[string]any{"local": date + "T20:30:00"}, http.StatusOK)
		spun := mine(http.MethodPost, "/v1/spin", map[string]any{"for_date": date}, http.StatusCreated)
		seq := int(dig(t, spun, "today", "spin", "seq").(float64))
		if action == "skip" {
			mine(http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": date, "spin_seq": seq}, http.StatusOK)
			return ""
		}
		ticket := mine(http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000}, http.StatusOK)
		created := mine(http.MethodPost, "/v1/entries", entryRequest(date, seq, ticket["photo_key"].(string)), http.StatusCreated)
		entry := entryObject(t, created["entry"])
		id = entry["id"].(string)
		if firstCategory == "" && action == "post" {
			firstCategory = dig(t, entry, "category", "id").(string)
		}
		switch action {
		case "delete":
			mine(http.MethodPost, "/v1/entries/"+id+"/delete", map[string]any{}, http.StatusOK)
		case "archive":
			mine(http.MethodPost, "/v1/entries/"+id+"/archive", map[string]any{}, http.StatusOK)
		}
		return id
	}
	day("2026-10-04", "delete") // a Sunday, deleted: no row
	day("2026-10-05", "post")
	day("2026-10-06", "post")
	day("2026-10-07", "skip")
	day("2026-10-08", "post")
	day("2026-10-09", "post")
	day("2026-10-10", "archive") // archived: still a spun day, no row

	// The summary: seven spun days, one week met (Oct 5 to 11 has four posts
	// plus the archived day), three newest visible entries.
	sum := mine(http.MethodGet, "/v1/me/summary", nil, http.StatusOK)
	if sum["spins"] != float64(7) || sum["followers"] != float64(0) || sum["week_streak"] != float64(1) {
		t.Fatalf("summary numbers: %v", sum)
	}
	recent := sum["recent"].([]any)
	if len(recent) != 3 {
		t.Fatalf("recent: %v", recent)
	}
	for i, want := range []string{"2026-10-09", "2026-10-08", "2026-10-06"} {
		if got := dig(t, recent[i], "local_date"); got != want {
			t.Fatalf("recent[%d] = %v, want %s", i, got, want)
		}
		if url, _ := dig(t, recent[i], "photo_url").(string); !strings.HasPrefix(url, store.PublicURL("")) || url == "" {
			t.Fatalf("recent[%d] photo_url %q", i, url)
		}
	}
	total := 0
	for _, tv := range sum["tastes"].([]any) {
		total += int(dig(t, tv, "count").(float64))
		if _, ok := dig(t, tv, "average").(float64); !ok {
			t.Fatalf("taste average: %v", tv)
		}
	}
	if total != 4 {
		t.Fatalf("taste counts add to %d, want 4 (archived and deleted left out)", total)
	}

	// The diary: four entries and one skipped day, newest first.
	page := mine(http.MethodGet, "/v1/diary", nil, http.StatusOK)
	items := page["items"].([]any)
	wantDates := []string{"2026-10-09", "2026-10-08", "2026-10-07", "2026-10-06", "2026-10-05"}
	if len(items) != len(wantDates) || page["next_cursor"] != nil {
		t.Fatalf("diary: %v", page)
	}
	for i, d := range wantDates {
		it := items[i].(map[string]any)
		if i == 2 {
			if it["kind"] != "skipped" || it["local_date"] != d || dig(t, it, "category", "name") == "" {
				t.Fatalf("skipped row: %v", it)
			}
			continue
		}
		if it["kind"] != "entry" || dig(t, it, "entry", "local_date") != d {
			t.Fatalf("row %d: %v", i, it)
		}
	}

	// Paging with limit 2 covers the same rows without overlap.
	var seen []string
	cursor := ""
	for pages := 0; pages < 5; pages++ {
		q := "/v1/diary?limit=2"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		p := mine(http.MethodGet, q, nil, http.StatusOK)
		for _, it := range p["items"].([]any) {
			m := it.(map[string]any)
			d, _ := m["local_date"].(string)
			if d == "" {
				d = dig(t, m, "entry", "local_date").(string)
			}
			seen = append(seen, d)
		}
		next, _ := p["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if strings.Join(seen, ",") != strings.Join(wantDates, ",") {
		t.Fatalf("paged dates %v, want %v", seen, wantDates)
	}

	// The category filter leaves skipped rows out and keeps only that category.
	filtered := mine(http.MethodGet, "/v1/diary?category_id="+firstCategory, nil, http.StatusOK)
	for _, it := range filtered["items"].([]any) {
		if dig(t, it, "kind") != "entry" || dig(t, it, "entry", "category", "id") != firstCategory {
			t.Fatalf("filtered row: %v", it)
		}
	}
	if len(filtered["items"].([]any)) == 0 || len(filtered["categories"].([]any)) == 0 {
		t.Fatalf("filtered: %v", filtered)
	}

	// Another person sees nothing of it, even with this person's category id.
	otherPage, raw := do(other.AccessToken, http.MethodGet, "/v1/diary?category_id="+firstCategory, nil, http.StatusOK)
	if len(otherPage["items"].([]any)) != 0 {
		t.Fatalf("another person's diary: %s", raw)
	}
	if _, raw = do(other.AccessToken, http.MethodGet, "/v1/me/summary", nil, http.StatusOK); !strings.Contains(raw, `"spins":0`) {
		t.Fatalf("another person's summary: %s", raw)
	}

	// Bad query values name their field.
	for q, field := range map[string]string{
		"category_id=abc": "category_id", "cursor=abc": "cursor", "limit=0": "limit", "limit=51": "limit", "limit=x": "limit",
	} {
		body, _ := do(me.AccessToken, http.MethodGet, "/v1/diary?"+q, nil, http.StatusBadRequest)
		if body["field"] != field {
			t.Fatalf("%s: %v", q, body)
		}
	}
	if rec := integrationDo(t, engine, http.MethodGet, "/v1/diary", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}

	// No key, birthday or email in either body.
	for _, p := range []string{"/v1/diary", "/v1/me/summary"} {
		_, raw := do(me.AccessToken, http.MethodGet, p, nil, http.StatusOK)
		for _, bad := range []string{"photo_key", "birthday", "email", "password_hash", "account_visibility"} {
			if strings.Contains(raw, bad) {
				t.Fatalf("%s carries %q: %s", p, bad, raw)
			}
		}
	}
}
