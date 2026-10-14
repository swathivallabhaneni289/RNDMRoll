package httpapi

import (
	"net/http"
	"testing"
)

// archiveCall sends POST /v1/entries/{id}/{verb} as p and returns the answer.
func (r *spinRig) archiveCall(p spinPerson, id, verb string) map[string]any {
	r.t.Helper()
	rec := r.call(http.MethodPost, "/v1/entries/"+id+"/"+verb, p.token, map[string]any{})
	if rec.Code != http.StatusOK {
		r.t.Fatalf("POST /v1/entries/%s/%s: %d %s", id, verb, rec.Code, rec.Body.String())
	}
	body := decodeBody(r.t, rec)
	if got := entrySortedKeys(body); len(got) != 2 || got[0] != "entry" || got[1] != "today" {
		r.t.Fatalf("%s answer members = %v", verb, got)
	}
	wantEntryShape(r.t, entryObject(r.t, body["entry"]))
	return body
}

func (r *spinRig) archiveList(p spinPerson) []any {
	r.t.Helper()
	rec := r.call(http.MethodGet, "/v1/me/archive", p.token, nil)
	if rec.Code != http.StatusOK {
		r.t.Fatalf("GET /v1/me/archive: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(r.t, rec)
	if got := entrySortedKeys(body); len(got) != 1 || got[0] != "entries" {
		r.t.Fatalf("archive members = %v", got)
	}
	list, _ := body["entries"].([]any)
	for _, item := range list {
		wantEntryShape(r.t, entryObject(r.t, item))
	}
	return list
}

func TestEntryArchiveRoundTripKeepsTheDayCounted(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	a, seq := r.dayOf(day)
	made := r.created(a, day, seq, r.photoKey(a))
	entry := entryObject(t, made["entry"])
	id := entry["id"].(string)
	if entry["archived"] != false || entry["comments_off"] != false {
		t.Fatalf("a new entry: %v", entry)
	}
	before := r.today(a)
	if dig(t, before, "week", "posted") != float64(1) {
		t.Fatalf("before: %v", before)
	}

	// Archive: archived true, Today still posted with the entry present.
	got := r.archiveCall(a, id, "archive")
	if dig(t, got, "entry", "archived") != true || dig(t, got, "entry", "id") != id {
		t.Fatalf("archive: %v", got)
	}
	if dig(t, got, "today", "state") != "posted" || dig(t, got, "today", "entry", "id") != id ||
		dig(t, got, "today", "entry", "archived") != true || dig(t, got, "today", "week", "posted") != float64(1) {
		t.Fatalf("today after archive: %v", got["today"])
	}
	// Again: idempotent.
	again := r.archiveCall(a, id, "archive")
	if dig(t, again, "entry", "archived") != true || dig(t, again, "today", "state") != "posted" {
		t.Fatalf("second archive: %v", again)
	}
	// GET /v1/today agrees, and GET of the entry says archived.
	if g := r.today(a); g["state"] != "posted" || dig(t, g, "entry", "archived") != true {
		t.Fatalf("GET today: %v", g)
	}
	if e := r.entryOf(a, id); e["archived"] != true {
		t.Fatalf("GET entry: %v", e)
	}
	if list := r.archiveList(a); len(list) != 1 {
		t.Fatalf("archive list: %v", list)
	}
	// The archive does not free the day: a second entry is refused.
	rec := r.postEntry(a, day, seq, r.photoKey(a))
	if rec.Code == http.StatusCreated || rec.Code == http.StatusOK {
		t.Fatalf("a second entry on an archived day: %d %s", rec.Code, rec.Body.String())
	}

	// Unarchive: archived false, nothing else created, the day still one.
	back := r.archiveCall(a, id, "unarchive")
	if dig(t, back, "entry", "archived") != false || dig(t, back, "today", "entry", "archived") != false ||
		dig(t, back, "today", "week", "posted") != float64(1) || dig(t, back, "today", "state") != "posted" {
		t.Fatalf("unarchive: %v", back)
	}
	if again := r.archiveCall(a, id, "unarchive"); dig(t, again, "entry", "archived") != false {
		t.Fatalf("second unarchive: %v", again)
	}
	if list := r.archiveList(a); len(list) != 0 {
		t.Fatalf("archive list after unarchive: %v", list)
	}
	after := r.entryOf(a, id)
	if after["id"] != id || len(photoIDs(t, after)) != 1 {
		t.Fatalf("after unarchive: %v", after)
	}
}

func TestEntryArchiveIsOwnerOnly(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	a, seq := r.dayOf(day)
	id := entryObject(t, r.created(a, day, seq, r.photoKey(a))["entry"])["id"].(string)
	other := r.anyone()
	r.setZone(other, "America/New_York")
	for _, verb := range []string{"archive", "unarchive"} {
		rec := r.call(http.MethodPost, "/v1/entries/"+id+"/"+verb, other.token, map[string]any{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s by another person: %d %s", verb, rec.Code, rec.Body.String())
		}
	}
	rec := r.call(http.MethodPost, "/v1/entries/not-a-uuid/archive", a.token, map[string]any{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("archive with a bad id: %d", rec.Code)
	}
	if e := r.entryOf(a, id); e["archived"] != false {
		t.Fatalf("the owner's entry changed: %v", e)
	}
	r.archiveCall(a, id, "archive")
	if list := r.archiveList(other); len(list) != 0 {
		t.Fatalf("another person's archive: %v", list)
	}
}

func TestEntryCommentsOffThroughPatchPersists(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	a, seq := r.dayOf(day)
	id := entryObject(t, r.created(a, day, seq, r.photoKey(a))["entry"])["id"].(string)

	if e := r.patched(a, id, map[string]any{"comments_off": true}); e["comments_off"] != true {
		t.Fatalf("patch on: %v", e)
	}
	if e := r.entryOf(a, id); e["comments_off"] != true {
		t.Fatalf("GET after on: %v", e)
	}
	// An edit that does not name it leaves it alone, and so does archiving.
	if e := r.patched(a, id, map[string]any{"title": "Dune: Part Two"}); e["comments_off"] != true {
		t.Fatalf("other edit changed it: %v", e)
	}
	got := r.archiveCall(a, id, "archive")
	if dig(t, got, "entry", "comments_off") != true || dig(t, got, "today", "entry", "comments_off") != true {
		t.Fatalf("archive: %v", got)
	}
	if e := r.patched(a, id, map[string]any{"comments_off": false}); e["comments_off"] != false || e["archived"] != true {
		t.Fatalf("patch off: %v", e)
	}
	rec := r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{"comments_off": "yes"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("comments_off as text: %d %s", rec.Code, rec.Body.String())
	}
}

func TestEntryArchiveListIsNewestDateFirst(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.anyone()
	ids := map[string]string{}
	for _, day := range []string{"2026-10-05", "2026-10-06", "2026-10-07"} {
		seq := int(r.spinEvening(a, day)["seq"].(float64))
		ids[day] = entryObject(t, r.created(a, day, seq, r.photoKey(a))["entry"])["id"].(string)
	}
	// Archive the middle one, then the oldest; the newest stays out.
	r.archiveCall(a, ids["2026-10-06"], "archive")
	r.archiveCall(a, ids["2026-10-05"], "archive")
	list := r.archiveList(a)
	if len(list) != 2 {
		t.Fatalf("archive list: %v", list)
	}
	first, second := entryObject(t, list[0]), entryObject(t, list[1])
	if first["id"] != ids["2026-10-06"] || second["id"] != ids["2026-10-05"] {
		t.Fatalf("order: %v then %v", first["local_date"], second["local_date"])
	}
}
