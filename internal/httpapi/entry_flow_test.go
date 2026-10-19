package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The tests in this file walk whole days through the real routes over the
// in-memory repository and a Fixed clock (New York). The helpers they use
// (anyone, dayOf, earnBonus, created, entryOf, patched, wantProblem,
// wantEntrySummary and so on) are in entry_test.go.

// --- a full day ---

func TestEntryFullDayWithAFixedClock(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("entry-fullday@example.com", "entry_fullday")
	store := r.rig.entryPhotos
	const ny, day = "America/New_York", "2026-10-07"
	r.setZone(a, ny)
	// A spin two evenings ago, so the first spin rule does not open the
	// wheel at 4 PM.
	r.spinEvening(a, "2026-10-05")

	// 4 PM: waiting, and the entry member is there and null.
	r.clockAt(ny, day, 16, 0, 0)
	if got := r.today(a); got["state"] != "waiting" || dig(t, got, "entry") != nil {
		t.Fatalf("4 PM: %v", got)
	}
	// Exactly 8 PM: open.
	r.clockAt(ny, day, 20, 0, 0)
	if got := r.today(a)["state"]; got != "open" {
		t.Fatalf("8:00:00 PM: state = %v, want open", got)
	}
	// The spin is 201 and Today shows the result with no entry.
	r.clockAt(ny, day, 20, 30, 0)
	rec := r.call(http.MethodPost, "/v1/spin", a.token, map[string]any{"for_date": day})
	if rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
	}
	seq := int(dig(t, decodeBody(t, rec), "today", "spin", "seq").(float64))
	if got := r.today(a); got["state"] != "result" || dig(t, got, "entry") != nil || dig(t, got, "week", "posted") != float64(0) {
		t.Fatalf("after the spin: %v", got)
	}

	// The post: 201, not already, Today posted with the summary, the week
	// count up by one.
	keys := []string{store.newKey(a.id, "jpg"), store.newKey(a.id, "png"), store.newKey(a.id, "webp")}
	rec = r.postEntry(a, day, seq, keys...)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	created := decodeBody(t, rec)
	if got := entrySortedKeys(created); !slices.Equal(got, []string{"already", "entry", "today"}) {
		t.Fatalf("create answer members = %v", got)
	}
	entry := entryObject(t, created["entry"])
	id := entry["id"].(string)
	if created["already"] != false || entry["late"] != false || entry["local_date"] != day || entry["photo_source"] != "camera" {
		t.Fatalf("created: %v", created)
	}
	today := entryObject(t, created["today"])
	if today["state"] != "posted" || dig(t, today, "entry", "id") != id || dig(t, today, "week", "posted") != float64(1) {
		t.Fatalf("today after the post: %v", today)
	}
	if today["spin"] == nil || dig(t, today, "bonus", "can_use") != false {
		t.Fatalf("a posted day shows its spin and offers no bonus: %v", today)
	}

	// The same post again: 200, already, the same entry, Today still posted
	// and the count still one.
	rec = r.postEntry(a, day, seq, keys...)
	again := decodeBody(t, rec)
	if rec.Code != http.StatusOK || again["already"] != true || dig(t, again, "entry", "id") != id ||
		dig(t, again, "today", "state") != "posted" || dig(t, again, "today", "week", "posted") != float64(1) {
		t.Fatalf("the same post again: %d %s", rec.Code, rec.Body.String())
	}

	// GET gives the entry the create answer gave.
	if got := r.entryOf(a, id); !reflect.DeepEqual(got, entry) {
		t.Fatalf("GET differs from the create answer:\n create %v\n get    %v", entry, got)
	}

	// PATCH at 9 PM: the fields change and updated_at moves; created_at, the
	// date and the late flag do not.
	r.clockAt(ny, day, 21, 0, 0)
	patched := r.patched(a, id, map[string]any{"title": "Dune: Part Two", "stars": 9, "reaction": "wow"})
	if patched["title"] != "Dune: Part Two" || patched["stars"] != float64(9) || patched["reaction"] != "wow" || patched["comment"] != "Slow and worth it." {
		t.Fatalf("patched fields: %v", patched)
	}
	if !instantOf(t, patched["updated_at"]).After(instantOf(t, entry["updated_at"])) {
		t.Fatalf("updated_at did not move: %v then %v", entry["updated_at"], patched["updated_at"])
	}
	for _, name := range []string{"created_at", "local_date", "late", "category", "id"} {
		if !reflect.DeepEqual(patched[name], entry[name]) {
			t.Fatalf("an edit changed %s: %v then %v", name, entry[name], patched[name])
		}
	}
	// An empty string clears the reaction and the comment.
	r.clockAt(ny, day, 21, 5, 0)
	cleared := r.patched(a, id, map[string]any{"reaction": "", "comment": ""})
	if cleared["reaction"] != nil || cleared["comment"] != nil || cleared["title"] != "Dune: Part Two" || cleared["stars"] != float64(9) {
		t.Fatalf("cleared: %v", cleared)
	}

	// Add one photo: the three stay in place and the new one is last. No file
	// is removed.
	ids := photoIDs(t, cleared)
	fourth, fifth := store.newKey(a.id, "jpg"), store.newKey(a.id, "png")
	r.clockAt(ny, day, 21, 10, 0)
	added := r.patched(a, id, map[string]any{"photos": append(keepPhotos(ids...), newPhotoItem(fourth))})
	if got := photoLinks(t, added); !slices.Equal(got, store.linksOf(append(slices.Clone(keys), fourth)...)) {
		t.Fatalf("after adding: %v", got)
	}
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("adding a photo removed files: %v", got)
	}

	// Reorder only: the same ids reversed. The order follows, the cover
	// follows, no file is removed.
	r.clockAt(ny, day, 21, 15, 0)
	reversed := photoIDs(t, added)
	slices.Reverse(reversed)
	reordered := r.patched(a, id, map[string]any{"photos": keepPhotos(reversed...)})
	if got := photoLinks(t, reordered); !slices.Equal(got, store.linksOf(fourth, keys[2], keys[1], keys[0])) {
		t.Fatalf("after reordering: %v", got)
	}
	if got := photoIDs(t, reordered); !slices.Equal(got, reversed) || reordered["photo_url"] != store.PublicURL(fourth) || reordered["photo_source"] != "library" {
		t.Fatalf("a reorder must keep the ids and move the cover (and its source): %v", reordered)
	}
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("a reorder removed files: %v", got)
	}

	// Add one, remove one and reorder in one edit: only the removed file goes.
	// reversed is [fourth, keys[2], keys[1], keys[0]]; keys[1] is dropped.
	r.clockAt(ny, day, 21, 20, 0)
	mixed := r.patched(a, id, map[string]any{"photos": []any{
		newPhotoItem(fifth),
		map[string]any{"id": reversed[3]},
		map[string]any{"id": reversed[0]},
		map[string]any{"id": reversed[1]},
	}})
	if got := photoLinks(t, mixed); !slices.Equal(got, store.linksOf(fifth, keys[0], fourth, keys[2])) {
		t.Fatalf("after the mixed edit: %v", got)
	}
	if got := store.deletes(); !slices.Equal(got, []string{keys[1]}) {
		t.Fatalf("removed files = %v, want only [%s]", got, keys[1])
	}

	// A store that fails does not fail the edit.
	store.failDeletesWith(errors.New("storage is down"))
	r.clockAt(ny, day, 21, 25, 0)
	last := r.patched(a, id, map[string]any{"photos": keepPhotos(photoIDs(t, mixed)[1:]...)})
	store.failDeletesWith(nil)
	if got := photoLinks(t, last); !slices.Equal(got, store.linksOf(keys[0], fourth, keys[2])) || last["photo_source"] != "camera" {
		t.Fatalf("after the edit with a failing store: %v", last)
	}
	if got := store.deletes(); !slices.Equal(got, []string{keys[1], fifth}) {
		t.Fatalf("removed files = %v, want [%s %s]", got, keys[1], fifth)
	}

	// An empty body changes nothing, even with the clock moved on (a null is
	// the same as absent).
	r.clockAt(ny, day, 21, 30, 0)
	for _, body := range []map[string]any{{}, {"photos": nil, "title": nil}} {
		if got := r.patched(a, id, body); !reflect.DeepEqual(got, last) {
			t.Fatalf("PATCH %v changed the entry:\n before %v\n after  %v", body, last, got)
		}
	}
	// An edit that would leave no photo is refused and changes nothing.
	rec = r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{"photos": []any{}})
	wantProblem(t, rec, "photos", "required")
	if got := r.entryOf(a, id); !reflect.DeepEqual(got, last) {
		t.Fatalf("a refused edit changed the entry: %v", got)
	}
	if got := len(store.deletes()); got != 2 {
		t.Fatalf("an empty or refused edit removed files: %d removals", got)
	}

	// Delete: {today} only, posted with a null entry, the count not dropped,
	// and a delete for every key the entry had.
	r.clockAt(ny, day, 22, 0, 0)
	wantKeys := keysOfLinks(photoLinks(t, last))
	removedBefore := len(store.deletes())
	rec = r.call(http.MethodPost, "/v1/entries/"+id+"/delete", a.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	deleted := decodeBody(t, rec)
	if got := entrySortedKeys(deleted); !slices.Equal(got, []string{"today"}) {
		t.Fatalf("delete answer members = %v", got)
	}
	today = entryObject(t, deleted["today"])
	if today["state"] != "posted" || dig(t, today, "entry") != nil || dig(t, today, "week", "posted") != float64(1) {
		t.Fatalf("today after the delete: %v", today)
	}
	removed := slices.Clone(store.deletes()[removedBefore:])
	sort.Strings(removed)
	sort.Strings(wantKeys)
	if !slices.Equal(removed, wantKeys) {
		t.Fatalf("removed files = %v, want every key of the entry %v", removed, wantKeys)
	}

	// Gone: a second delete, a read and an edit are 404; the day stays taken.
	wantCode(t, r.call(http.MethodPost, "/v1/entries/"+id+"/delete", a.token, map[string]any{}), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodGet, "/v1/entries/"+id, a.token, nil), http.StatusNotFound, "not_found")
	wantCode(t, r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{"title": "x"}), http.StatusNotFound, "not_found")
	wantCode(t, r.postEntry(a, day, seq, store.newKey(a.id, "jpg")), http.StatusConflict, "day_settled")
	if got := r.today(a); got["state"] != "posted" || dig(t, got, "entry") != nil || dig(t, got, "week", "posted") != float64(1) {
		t.Fatalf("today at the end: %v", got)
	}
}

// --- the late window ---

func TestEntryLateWindowPostAndSkipCountForLastNight(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const ny, day, next = "America/New_York", "2026-10-07", "2026-10-08"
	store := r.rig.entryPhotos
	poster, seqP := r.dayOf(day)
	skipper, seqS := r.dayOf(day)

	// Just after midnight: late, with last night's spin.
	r.clockAt(ny, next, 0, 30, 0)
	var lastNight [2]any
	for i, p := range []spinPerson{poster, skipper} {
		today := r.today(p)
		if today["state"] != "late" || today["local_date"] != next || dig(t, today, "spin", "local_date") != day || dig(t, today, "entry") != nil {
			t.Fatalf("00:30: %v", today)
		}
		lastNight[i] = dig(t, today, "spin", "id")
	}

	// A post for last night: 201, late, and Today is posted with last night's
	// spin and the summary until the next opening.
	key := store.newKey(poster.id, "jpg")
	created := r.created(poster, day, seqP, key)
	entry := entryObject(t, created["entry"])
	if entry["late"] != true || entry["local_date"] != day {
		t.Fatalf("late post: %v", entry)
	}
	for label, today := range map[string]map[string]any{"the create answer": entryObject(t, created["today"]), "GET /v1/today": r.today(poster)} {
		if today["state"] != "posted" || today["local_date"] != next || dig(t, today, "spin", "id") != lastNight[0] || dig(t, today, "bonus", "can_use") != false {
			t.Fatalf("%s: %v", label, today)
		}
		if sum := entryObject(t, today["entry"]); sum["late"] != true || sum["id"] != entry["id"] || sum["photo_url"] != store.PublicURL(key) {
			t.Fatalf("%s: summary %v", label, sum)
		}
	}

	// A skip for last night: 200, skipped, last night's spin, no entry, the
	// flag on last night's week day and the count unchanged.
	rec := r.call(http.MethodPost, "/v1/spin/skip", skipper.token, map[string]any{"for_date": day, "spin_seq": seqS})
	if rec.Code != http.StatusOK {
		t.Fatalf("late skip: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	today := entryObject(t, body["today"])
	if body["already"] != false || today["state"] != "skipped" || today["local_date"] != next ||
		dig(t, today, "spin", "id") != lastNight[1] || dig(t, today, "entry") != nil {
		t.Fatalf("late skip answer: %v", body)
	}
	if weekDay(t, today, day)["skipped"] != true || dig(t, today, "week", "posted") != float64(0) {
		t.Fatalf("the week after a late skip: %v", today["week"])
	}
	if got := r.today(skipper)["state"]; got != "skipped" {
		t.Fatalf("GET /v1/today after a late skip: %v", got)
	}

	// At 8:00 PM the new night opens for both, with no spin and no entry.
	r.clockAt(ny, next, 20, 0, 0)
	for _, p := range []spinPerson{poster, skipper} {
		today := r.today(p)
		if today["state"] != "open" || today["spin"] != nil || dig(t, today, "entry") != nil {
			t.Fatalf("8 PM: %v", today)
		}
	}
}

func TestSkipAndPostCloseWhenTheLateWindowEnds(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const ny, day, next = "America/New_York", "2026-10-07", "2026-10-08"
	store := r.rig.entryPhotos
	postsIn, seqA := r.dayOf(day)
	skipsIn, seqB := r.dayOf(day)
	tooLate, seqC := r.dayOf(day)
	skip := func(p spinPerson, seq int) *httptest.ResponseRecorder {
		return r.call(http.MethodPost, "/v1/spin/skip", p.token, map[string]any{"for_date": day, "spin_seq": seq})
	}

	// One second before 8 PM the next night both are still in time, and late.
	r.clockAt(ny, next, 19, 59, 59)
	created := r.created(postsIn, day, seqA, store.newKey(postsIn.id, "jpg"))
	if dig(t, created, "entry", "late") != true {
		t.Fatalf("a post at 7:59:59 PM: %v", created)
	}
	if rec := skip(skipsIn, seqB); rec.Code != http.StatusOK || dig(t, decodeBody(t, rec), "today", "state") != "skipped" {
		t.Fatalf("a skip at 7:59:59 PM: %d %s", rec.Code, rec.Body.String())
	}

	// At exactly 8 PM the window has ended: day_closed for both, and nothing
	// was written.
	r.clockAt(ny, next, 20, 0, 0)
	wantCode(t, r.postEntry(tooLate, day, seqC, store.newKey(tooLate.id, "jpg")), http.StatusConflict, "day_closed")
	wantCode(t, skip(tooLate, seqC), http.StatusConflict, "day_closed")
	if got := r.today(tooLate); got["state"] != "open" || got["spin"] != nil || dig(t, got, "entry") != nil {
		t.Fatalf("after the refusals: %v", got)
	}
}

// --- skip ---

func TestSkipSettlesTheDayAndRefusesALaterPostOrBonus(t *testing.T) {
	const email = "skip-settles@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{email}})
	a := r.person(email, "skip_settles")
	store := r.rig.entryPhotos
	const day = "2026-10-09"
	r.earnBonus(a)
	seq := int(r.spinEvening(a, day)["seq"].(float64))
	skip := func(date string, s int) *httptest.ResponseRecorder {
		return r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": date, "spin_seq": s})
	}

	// A date with no spin (the pretend Thursday is posted but was never spun)
	// and a date that has not come are no_spin, for a skip and for a post.
	for _, date := range []string{"2026-10-08", "2026-10-10"} {
		wantCode(t, skip(date, 0), http.StatusConflict, "no_spin")
		wantCode(t, r.postEntry(a, date, 0, store.newKey(a.id, "jpg")), http.StatusConflict, "no_spin")
	}
	if got := dig(t, r.today(a), "week", "posted"); got != float64(4) {
		t.Fatalf("pretend days before the skip: %v, want 4", got)
	}

	// The skip: 200, not already, skipped, flagged on its week day, and the
	// posted count is unchanged.
	rec := skip(day, seq)
	if rec.Code != http.StatusOK {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	first := decodeBody(t, rec)
	if got := entrySortedKeys(first); !slices.Equal(got, []string{"already", "today"}) {
		t.Fatalf("skip answer members = %v", got)
	}
	today := entryObject(t, first["today"])
	if first["already"] != false || today["state"] != "skipped" || dig(t, today, "entry") != nil || dig(t, today, "week", "posted") != float64(4) {
		t.Fatalf("skip answer: %v", first)
	}
	if d := weekDay(t, today, day); d["skipped"] != true || d["posted"] != false {
		t.Fatalf("the skipped day: %v", d)
	}
	if d := weekDay(t, today, "2026-10-08"); d["skipped"] != false || d["posted"] != true {
		t.Fatalf("a pretend posted day: %v", d)
	}

	// Again: 200 and already.
	rec = skip(day, seq)
	if second := decodeBody(t, rec); rec.Code != http.StatusOK || second["already"] != true || dig(t, second, "today", "state") != "skipped" {
		t.Fatalf("skip again: %d %s", rec.Code, rec.Body.String())
	}

	// A post and a bonus spin after it are refused. The day stays skipped and
	// the bonus is still held.
	wantCode(t, r.postEntry(a, day, seq, store.newKey(a.id, "jpg")), http.StatusConflict, "day_settled")
	wantCode(t, r.call(http.MethodPost, "/v1/spin/bonus", a.token, map[string]any{"after_seq": seq, "for_date": day}), http.StatusConflict, "day_settled")
	now := r.today(a)
	if now["state"] != "skipped" || dig(t, now, "bonus", "held") != true || dig(t, now, "bonus", "can_use") != false {
		t.Fatalf("today after the refusals: %v", now)
	}
}

func TestSkipWithAWrongSpinNumberWritesNothing(t *testing.T) {
	const email = "skip-seq@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{email}})
	b := r.person(email, "skip_seq")
	store := r.rig.entryPhotos
	const day = "2026-10-09"
	r.earnBonus(b)
	r.spinEvening(b, day)

	// A bonus spin made on another phone: the number is now 1.
	rec := r.call(http.MethodPost, "/v1/spin/bonus", b.token, map[string]any{"after_seq": 0, "for_date": day})
	if rec.Code != http.StatusCreated || dig(t, decodeBody(t, rec), "today", "spin", "seq") != float64(1) {
		t.Fatalf("bonus: %d %s", rec.Code, rec.Body.String())
	}
	bonusCategory := dig(t, decodeBody(t, rec), "today", "spin", "category", "id")

	// This phone still has number 0: a skip and a post are refused.
	key := store.newKey(b.id, "jpg")
	wantCode(t, r.call(http.MethodPost, "/v1/spin/skip", b.token, map[string]any{"for_date": day, "spin_seq": 0}), http.StatusConflict, "spin_changed")
	wantCode(t, r.postEntry(b, day, 0, key), http.StatusConflict, "spin_changed")

	// Nothing was written: the day is still a result, no entry, nothing
	// flagged and the count as before.
	today := r.today(b)
	if today["state"] != "result" || dig(t, today, "entry") != nil || dig(t, today, "week", "posted") != float64(4) ||
		weekDay(t, today, day)["skipped"] != false {
		t.Fatalf("today after the refusals: %v", today)
	}

	// The same key posts with the number the phone has now (the refusal used
	// nothing up), and the entry belongs to the bonus spin.
	created := r.created(b, day, 1, key)
	if dig(t, created, "today", "spin", "seq") != float64(1) || dig(t, created, "entry", "category", "id") != bonusCategory {
		t.Fatalf("the post after a bonus spin: %v", created)
	}
}

// --- Today's summary on every route that answers with Today ---

func TestEntryTodaySummaryOnEveryRouteThatAnswersWithToday(t *testing.T) {
	const email = "entry-summary@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{email}})
	a := r.person(email, "entry_summary")
	store := r.rig.entryPhotos
	const ny, e0Day, skipDay, day = "America/New_York", "2026-10-08", "2026-10-09", "2026-10-10"
	post := func(date string, seq int, keys []string) *httptest.ResponseRecorder {
		return r.postEntry(a, date, seq, keys...)
	}

	// Three pretend days, then a real post on Thursday: the fourth posted day
	// of the week holds a bonus spin. Friday is skipped. Saturday is the day
	// under test.
	r.setZone(a, ny)
	r.clockAt(ny, "2026-10-07", 12, 0, 0)
	rec := r.call(http.MethodPost, "/v1/test/posted-days", a.token, map[string]any{"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("posted-days: %d %s", rec.Code, rec.Body.String())
	}
	seq0 := int(r.spinEvening(a, e0Day)["seq"].(float64))
	e0Keys := []string{store.newKey(a.id, "jpg"), store.newKey(a.id, "webp")}
	e0 := entryObject(t, r.created(a, e0Day, seq0, e0Keys...)["entry"])
	if dig(t, r.today(a), "bonus", "held") != true {
		t.Fatalf("the fourth posted day of the week should hold a bonus: %v", r.today(a)["bonus"])
	}
	skipSeq := int(r.spinEvening(a, skipDay)["seq"].(float64))
	if rec := r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": skipDay, "spin_seq": skipSeq}); rec.Code != http.StatusOK {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	r.spinEvening(a, day)

	// Before the post: a result with the entry member present and null.
	if got := r.today(a); got["state"] != "result" || dig(t, got, "entry") != nil {
		t.Fatalf("before the post: %v", got)
	}
	// The bonus is spent first, so the post belongs to spin number 1.
	rec = r.call(http.MethodPost, "/v1/spin/bonus", a.token, map[string]any{"after_seq": 0, "for_date": day})
	if rec.Code != http.StatusCreated {
		t.Fatalf("bonus: %d %s", rec.Code, rec.Body.String())
	}
	keys := []string{store.newKey(a.id, "jpg"), store.newKey(a.id, "png"), store.newKey(a.id, "webp")}
	created := r.created(a, day, 1, keys...)
	id := entryObject(t, created["entry"])["id"].(string)
	want := entrySummary{title: "Dune", stars: 7, reaction: "fire", count: 3, cover: store.PublicURL(keys[0])}

	todayOf := func(rec *httptest.ResponseRecorder, wantStatus int, already bool) map[string]any {
		t.Helper()
		body := decodeBody(t, rec)
		if rec.Code != wantStatus || body["already"] != already {
			t.Fatalf("%d %s, want %d with already %v", rec.Code, rec.Body.String(), wantStatus, already)
		}
		return entryObject(t, body["today"])
	}
	checkAll := func(step string, want entrySummary) {
		t.Helper()
		wantEntrySummary(t, step+": GET /v1/today", r.today(a), want)
		rec := r.call(http.MethodPost, "/v1/spin", a.token, map[string]any{"for_date": day})
		wantEntrySummary(t, step+": POST /v1/spin", todayOf(rec, http.StatusOK, true), want)
		// The retry of the bonus spin with the number the phone had before it.
		rec = r.call(http.MethodPost, "/v1/spin/bonus", a.token, map[string]any{"after_seq": 0, "for_date": day})
		wantEntrySummary(t, step+": POST /v1/spin/bonus", todayOf(rec, http.StatusOK, true), want)
		// A retry of Friday's skip: already, and Today is the current day's.
		rec = r.call(http.MethodPost, "/v1/spin/skip", a.token, map[string]any{"for_date": skipDay, "spin_seq": skipSeq})
		wantEntrySummary(t, step+": POST /v1/spin/skip", todayOf(rec, http.StatusOK, true), want)
	}

	wantEntrySummary(t, "the create answer", entryObject(t, created["today"]), want)
	checkAll("after the post", want)
	// The same create again.
	wantEntrySummary(t, "POST /v1/entries again", todayOf(post(day, 1, keys), http.StatusOK, true), want)

	// Remove the first photo: the count is 2 and the cover is the old second.
	ids := photoIDs(t, entryObject(t, created["entry"]))
	r.clockAt(ny, day, 21, 0, 0)
	r.patched(a, id, map[string]any{"photos": keepPhotos(ids[1], ids[2])})
	want.count, want.cover = 2, store.PublicURL(keys[1])
	checkAll("after removing the first photo", want)

	// Reorder: the cover follows.
	r.patched(a, id, map[string]any{"photos": keepPhotos(ids[2], ids[1])})
	want.cover = store.PublicURL(keys[2])
	checkAll("after a reorder", want)

	// The other scalar fields follow too, and a cleared reaction is null.
	r.patched(a, id, map[string]any{"title": "Dune II", "stars": 3, "reaction": ""})
	want.title, want.stars, want.reaction = "Dune II", 3, nil
	checkAll("after a title, stars and reaction edit", want)

	// Deleting the earlier day's entry answers with the current day's summary
	// and removes only that entry's files.
	removedBefore := len(store.deletes())
	rec = r.call(http.MethodPost, "/v1/entries/"+e0["id"].(string)+"/delete", a.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete of the earlier entry: %d %s", rec.Code, rec.Body.String())
	}
	wantEntrySummary(t, "POST /v1/entries/{id}/delete of another day's entry", entryObject(t, decodeBody(t, rec)["today"]), want)
	removed := slices.Clone(store.deletes()[removedBefore:])
	sort.Strings(removed)
	if wantFiles := slices.Sorted(slices.Values(e0Keys)); !slices.Equal(removed, wantFiles) {
		t.Fatalf("removed files = %v, want only the earlier entry's %v", removed, wantFiles)
	}
	wantEntrySummary(t, "GET /v1/today after that delete", r.today(a), want)

	// Deleting today's entry: posted, and the entry member is null.
	rec = r.call(http.MethodPost, "/v1/entries/"+id+"/delete", a.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if got := entryObject(t, decodeBody(t, rec)["today"]); got["state"] != "posted" || dig(t, got, "entry") != nil {
		t.Fatalf("the delete answer: %v", got)
	}
	if got := r.today(a); got["state"] != "posted" || dig(t, got, "entry") != nil {
		t.Fatalf("GET /v1/today after the delete: %v", got)
	}
}

func TestEntryTodayHasANullEntryOnASkippedDay(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	b, seq := r.dayOf(day)
	skip := func() map[string]any {
		rec := r.call(http.MethodPost, "/v1/spin/skip", b.token, map[string]any{"for_date": day, "spin_seq": seq})
		if rec.Code != http.StatusOK {
			t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
		}
		return decodeBody(t, rec)
	}
	for _, again := range []bool{false, true} {
		body := skip()
		today := entryObject(t, body["today"])
		if body["already"] != again || today["state"] != "skipped" || dig(t, today, "entry") != nil {
			t.Fatalf("skip (already %v): %v", again, body)
		}
	}
	if got := r.today(b); got["state"] != "skipped" || dig(t, got, "entry") != nil {
		t.Fatalf("GET /v1/today: %v", got)
	}
}

// --- nothing private in any answer ---

// entryForbiddenMembers are the names no Phase 3 answer may carry. The upload
// ticket's own photo_key is the one exception, and only on that answer.
var entryForbiddenMembers = map[string]bool{
	"photo_key": true, "storage_key": true, "cover_key": true, "birthday": true, "email": true, "password_hash": true,
}

// entryLeaks returns what is wrong with an answer body: a member that must not
// be there anywhere in the tree, a storage key sent bare (every file is a link
// that starts with the public base), or the account's email text. ticket marks
// the upload ticket's answer, which may carry photo_key and its upload_url.
func entryLeaks(body []byte, email string, ticket bool) []string {
	var problems []string
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return []string{"not JSON: " + string(body)}
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				name := strings.ToLower(k)
				if entryForbiddenMembers[name] && !(ticket && name == "photo_key") {
					problems = append(problems, "member "+k+" at "+path)
				}
				walk(path+"."+k, child)
			}
		case []any:
			for _, child := range x {
				walk(path+"[]", child)
			}
		case string:
			if !ticket && strings.Contains(x, "entries/") && !strings.HasPrefix(x, fakeEntryPhotoPublicBase) {
				problems = append(problems, "a bare key "+x+" at "+path)
			}
		}
	}
	walk("$", v)
	if email != "" && strings.Contains(strings.ToLower(string(body)), strings.ToLower(email)) {
		problems = append(problems, "the account's email text")
	}
	return problems
}

func TestEntryLeakScanSeesWhatItLooksFor(t *testing.T) {
	const email = "someone@example.com"
	for name, c := range map[string]struct {
		body   string
		ticket bool
		leaks  int
	}{
		"clean":                  {`{"entry":{"photos":[{"url":"https://cdn.example.test/entries/u/0123456789abcdef.jpg"}]}}`, false, 0},
		"a photo_key":            {`{"entry":{"photo_key":"x"}}`, false, 1},
		"a photo_key in a list":  {`{"photos":[{"id":"1","PHOTO_KEY":"x"}]}`, false, 1},
		"a cover_key deep down":  {`{"today":{"entry":{"cover_key":"x"}}}`, false, 1},
		"a storage_key":          {`{"storage_key":"x"}`, false, 1},
		"a birthday":             {`{"user":{"birthday":"1990-06-15"}}`, false, 1},
		"an email member":        {`{"user":{"email":"x"}}`, false, 1},
		"a password_hash":        {`{"password_hash":"x"}`, false, 1},
		"a bare key":             {`{"photo_url":"entries/u/0123456789abcdef.jpg"}`, false, 1},
		"the email text":         {`{"message":"hello someone@example.com"}`, false, 1},
		"the ticket's photo_key": {`{"photo_key":"entries/u/0123456789abcdef.jpg","upload_url":"https://s.example/entries/u/x"}`, true, 0},
		"a ticket with an email": {`{"photo_key":"x","email":"y"}`, true, 1},
	} {
		if got := entryLeaks([]byte(c.body), email, c.ticket); len(got) != c.leaks {
			t.Errorf("%s: %d problems %v, want %d", name, len(got), got, c.leaks)
		}
	}
}

func TestEntryNoAnswerCarriesAKeyEmailBirthdayOrPasswordHash(t *testing.T) {
	const email = "entry-private@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{email}})
	p := r.person(email, "entry_private")
	store := r.rig.entryPhotos
	const day, other = "2026-10-09", "2026-10-10"

	// do sends one request as p and checks the answer for leaks; ticket marks
	// the one kind of answer that carries a photo_key.
	do := func(label, method, path string, body any, wantStatus int, ticket bool) map[string]any {
		t.Helper()
		rec := r.call(method, path, p.token, body)
		if rec.Code != wantStatus {
			t.Fatalf("%s: %d %s, want %d", label, rec.Code, rec.Body.String(), wantStatus)
		}
		for _, problem := range entryLeaks(rec.Body.Bytes(), email, ticket) {
			t.Errorf("%s: %s in %s", label, problem, rec.Body.String())
		}
		return decodeBody(t, rec)
	}
	post := func(label string, want int, body map[string]any) map[string]any {
		t.Helper()
		return do(label, http.MethodPost, "/v1/entries", body, want, false)
	}

	r.earnBonus(p)
	r.spinEvening(p, day)
	var keys []string
	for i := 0; i < 2; i++ {
		answer := do("ticket", http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000}, http.StatusOK, true)
		keys = append(keys, answer["photo_key"].(string))
	}
	do("ticket refused", http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/gif", "content_length": 5000}, http.StatusBadRequest, false)
	do("spin again", http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusOK, false)
	do("bonus", http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": day}, http.StatusCreated, false)

	post("create refused", http.StatusBadRequest, map[string]any{"for_date": day, "spin_seq": 1})
	foreign := entryRequest(day, 1, store.newKey(p.id, "jpg"))
	foreign["photos"] = []any{newPhotoItem(store.newKey(r.anyone().id, "jpg"))}
	post("create with a foreign key", http.StatusBadRequest, foreign)
	created := post("create", http.StatusCreated, entryRequest(day, 1, keys...))
	post("create again", http.StatusOK, entryRequest(day, 1, keys...))
	post("create on a settled day", http.StatusConflict, entryRequest(day, 1, store.newKey(p.id, "jpg")))
	id := entryObject(t, created["entry"])["id"].(string)

	do("GET today", http.MethodGet, "/v1/today", nil, http.StatusOK, false)
	do("spin again", http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusOK, false)
	do("bonus retry", http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": day}, http.StatusOK, false)
	do("GET entry", http.MethodGet, "/v1/entries/"+id, nil, http.StatusOK, false)
	do("GET unknown entry", http.MethodGet, "/v1/entries/"+uuid.NewString(), nil, http.StatusNotFound, false)
	ids := photoIDs(t, entryObject(t, created["entry"]))
	do("PATCH", http.MethodPatch, "/v1/entries/"+id, map[string]any{
		"title":  "Dune II",
		"photos": []any{map[string]any{"id": ids[0]}, newPhotoItem(store.newKey(p.id, "webp"))},
	}, http.StatusOK, false)
	do("PATCH refused", http.MethodPatch, "/v1/entries/"+id, map[string]any{"photos": []any{map[string]any{"id": uuid.NewString()}}}, http.StatusBadRequest, false)
	do("skip refused", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": day, "spin_seq": 1}, http.StatusConflict, false)
	do("skip with no spin", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": "2026-10-11", "spin_seq": 0}, http.StatusConflict, false)
	do("DELETE", http.MethodPost, "/v1/entries/"+id+"/delete", map[string]any{}, http.StatusOK, false)
	do("DELETE again", http.MethodPost, "/v1/entries/"+id+"/delete", map[string]any{}, http.StatusNotFound, false)

	// A skipped day, and the reset.
	r.spinEvening(p, other)
	do("skip", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": other, "spin_seq": 0}, http.StatusOK, false)
	do("skip again", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": other, "spin_seq": 0}, http.StatusOK, false)
	do("GET today, skipped", http.MethodGet, "/v1/today", nil, http.StatusOK, false)
	do("reset", http.MethodPost, "/v1/test/reset", map[string]any{}, http.StatusOK, false)
}

// --- photos and keys of another entry or another person ---

func TestEntryAPhotoIdOrKeyOfAnotherEntryOrPersonIsRefusedAndNothingChanges(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	store := r.rig.entryPhotos
	const day1, day2, day3 = "2026-10-05", "2026-10-06", "2026-10-07"

	// A posts on two days, and B posts on the second.
	a := r.anyone()
	seq1 := int(r.spinEvening(a, day1)["seq"].(float64))
	k1 := []string{store.newKey(a.id, "jpg"), store.newKey(a.id, "png")}
	e1 := entryObject(t, r.created(a, day1, seq1, k1...)["entry"])
	seq2 := int(r.spinEvening(a, day2)["seq"].(float64))
	e2 := entryObject(t, r.created(a, day2, seq2, store.newKey(a.id, "jpg"), store.newKey(a.id, "png"))["entry"])
	b, seqB := r.dayOf(day2)
	eb := entryObject(t, r.created(b, day2, seqB, store.newKey(b.id, "jpg"))["entry"])
	seq3 := int(r.spinEvening(a, day3)["seq"].(float64))

	id1, id2, idB := e1["id"].(string), e2["id"].(string), eb["id"].(string)
	photo := func(entry map[string]any, i int) string { return photoIDs(t, entry)[i] }
	removedBefore := len(store.deletes())

	// An id of A's other entry, an id of B's entry, an id that is nobody's:
	// each is refused as a photo that can't be used.
	for name, list := range map[string][]any{
		"an id of another entry of the same person": append(keepPhotos(photo(e2, 0)), keepPhotos(photo(e1, 0))...),
		"an id of another person's entry":           keepPhotos(photo(eb, 0)),
		"an id that is nobody's":                    keepPhotos(uuid.NewString()),
		"a key already attached to another entry":   append(keepPhotos(photo(e2, 0)), newPhotoItem(k1[0])),
	} {
		t.Run(name, func(t *testing.T) {
			rec := r.call(http.MethodPatch, "/v1/entries/"+id2, a.token, map[string]any{"title": "Changed", "photos": list})
			wantProblem(t, rec, "photos", "invalid")
		})
	}
	// A new day's post with a key that is already attached is refused too.
	wantProblem(t, r.postEntry(a, day3, seq3, k1[0]), "photos", "invalid")

	// Nothing was read into an answer or changed: every entry is as it was,
	// no file was removed, and day 3 is still waiting for its post.
	for _, c := range []struct {
		owner spinPerson
		id    string
		was   map[string]any
	}{{a, id1, e1}, {a, id2, e2}, {b, idB, eb}} {
		if got := r.entryOf(c.owner, c.id); !reflect.DeepEqual(got, c.was) {
			t.Fatalf("entry %s changed:\n was %v\n now %v", c.id, c.was, got)
		}
	}
	if got := len(store.deletes()); got != removedBefore {
		t.Fatalf("a refused edit removed files: %v", store.deletes()[removedBefore:])
	}
	if got := r.today(a); got["state"] != "result" || dig(t, got, "entry") != nil {
		t.Fatalf("day 3 after the refusals: %v", got)
	}
	// The refusals used no key up: a fresh key posts day 3.
	r.created(a, day3, seq3, store.newKey(a.id, "jpg"))
}

// --- the reset route ---

func TestEntryResetClearsEntriesAndPostedDaysOfTheCaller(t *testing.T) {
	const emailA, emailB = "entry-reset-a@example.com", "entry-reset-b@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{emailA, emailB}})
	a := r.person(emailA, "entry_reset_a")
	b := r.person(emailB, "entry_reset_b")
	store := r.rig.entryPhotos
	const day = "2026-10-07"
	seqA := int(r.spinEvening(a, day)["seq"].(float64))
	seqB := int(r.spinEvening(b, day)["seq"].(float64))
	keyA := store.newKey(a.id, "jpg")
	idA := entryObject(t, r.created(a, day, seqA, keyA)["entry"])["id"].(string)
	idB := entryObject(t, r.created(b, day, seqB, store.newKey(b.id, "jpg"))["entry"])["id"].(string)
	if got := r.today(a); got["state"] != "posted" || dig(t, got, "week", "posted") != float64(1) {
		t.Fatalf("before the reset: %v", got)
	}

	rec := r.call(http.MethodPost, "/v1/test/reset", a.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}

	// A has no entry, no posted day and no spin; the photo file stays.
	today := r.today(a)
	if today["state"] != "open" || today["spin"] != nil || dig(t, today, "entry") != nil || dig(t, today, "week", "posted") != float64(0) {
		t.Fatalf("after the reset: %v", today)
	}
	if d := weekDay(t, today, day); d["posted"] != false || d["skipped"] != false {
		t.Fatalf("the reset day: %v", d)
	}
	wantCode(t, r.call(http.MethodGet, "/v1/entries/"+idA, a.token, nil), http.StatusNotFound, "not_found")
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("a reset removed files: %v", got)
	}

	// B is untouched.
	if got := r.today(b); got["state"] != "posted" || dig(t, got, "entry", "id") != idB || dig(t, got, "week", "posted") != float64(1) {
		t.Fatalf("B after A's reset: %v", got)
	}
	r.entryOf(b, idB)

	// A spins and posts the same day again.
	seq := int(r.spinEvening(a, day)["seq"].(float64))
	created := r.created(a, day, seq, store.newKey(a.id, "png"))
	if created["already"] != false || dig(t, created, "entry", "id") == idA || dig(t, created, "today", "week", "posted") != float64(1) {
		t.Fatalf("the post after the reset: %v", created)
	}
}

// --- details of create and edit ---

func TestEntryCreateTakesAnAbsentNullOrEmptyReactionAndComment(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	for name, mutate := range map[string]func(map[string]any){
		"absent": func(b map[string]any) { delete(b, "reaction"); delete(b, "comment") },
		"null":   func(b map[string]any) { b["reaction"] = nil; b["comment"] = nil },
		"empty":  func(b map[string]any) { b["reaction"] = ""; b["comment"] = "   " },
	} {
		t.Run(name, func(t *testing.T) {
			p, seq := r.dayOf(day)
			key := r.rig.entryPhotos.newKey(p.id, "jpg")
			body := entryRequest(day, seq, key)
			body["title"] = "  Dune  " // the title is trimmed
			mutate(body)
			rec := r.call(http.MethodPost, "/v1/entries", p.token, body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			created := decodeBody(t, rec)
			entry := entryObject(t, created["entry"])
			if entry["title"] != "Dune" || entry["reaction"] != nil || entry["comment"] != nil {
				t.Fatalf("entry: %v", entry)
			}
			wantEntrySummary(t, "the create answer", entryObject(t, created["today"]),
				entrySummary{title: "Dune", stars: 7, reaction: nil, count: 1, cover: r.rig.entryPhotos.PublicURL(key)})
		})
	}
}

func TestEntryCategoryIsTheNameAsWrittenOnTheDay(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	p, seq := r.dayOf(day)
	spun := entryObject(t, r.today(p)["spin"])
	catID, catName := dig(t, spun, "category", "id"), dig(t, spun, "category", "name")

	// Rename the category the spin landed on, after the spin.
	wheel := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	var list []any
	for _, c := range wheel["categories"].([]any) {
		m := c.(map[string]any)
		item := map[string]any{"id": m["id"], "name": m["name"], "weight": m["weight"]}
		if m["id"] == catID {
			item["name"] = "Renamed " + catName.(string)
		}
		list = append(list, item)
	}
	rec := r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": wheel["version"], "categories": list})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}

	// The entry keeps the id and the name as written on the day.
	created := r.created(p, day, seq, r.rig.entryPhotos.newKey(p.id, "jpg"))
	id := entryObject(t, created["entry"])["id"].(string)
	for label, entry := range map[string]map[string]any{"the create answer": entryObject(t, created["entry"]), "GET": r.entryOf(p, id)} {
		if dig(t, entry, "category", "id") != catID || dig(t, entry, "category", "name") != catName {
			t.Fatalf("%s: category = %v, want %v %v", label, entry["category"], catID, catName)
		}
	}
}

func TestEntryPatchRefusesABadScalarFieldAndChangesNothing(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	a, seq := r.dayOf(day)
	id := entryObject(t, r.created(a, day, seq, r.rig.entryPhotos.newKey(a.id, "jpg"))["entry"])["id"].(string)
	before := r.entryOf(a, id)

	for _, c := range []struct {
		name          string
		body          map[string]any
		field, reason string
	}{
		{"title is blank", map[string]any{"title": "   "}, "title", "required"},
		{"title is 81 characters", map[string]any{"title": strings.Repeat(eAcute, 81)}, "title", "too_long"},
		{"title holds a control character", map[string]any{"title": "Du\u0007ne"}, "title", "invalid"},
		{"title is a number", map[string]any{"title": 5}, "title", "invalid"},
		{"stars is 0", map[string]any{"stars": 0}, "stars", "invalid"},
		{"stars is 11", map[string]any{"stars": 11}, "stars", "invalid"},
		{"stars is a fraction", map[string]any{"stars": 3.5}, "stars", "invalid"},
		{"reaction clap", map[string]any{"reaction": "clap"}, "reaction", "invalid"},
		{"reaction heart", map[string]any{"reaction": "heart"}, "reaction", "invalid"},
		{"comment is 141 characters", map[string]any{"comment": strings.Repeat(eAcute, 141)}, "comment", "too_long"},
		{"comment holds a control character", map[string]any{"comment": "a\u0007b"}, "comment", "invalid"},
		{"a good field beside a bad one", map[string]any{"title": "Fine", "stars": 11}, "stars", "invalid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantProblem(t, r.call(http.MethodPatch, "/v1/entries/"+id, a.token, c.body), c.field, c.reason)
		})
	}
	if got := r.entryOf(a, id); !reflect.DeepEqual(got, before) {
		t.Fatalf("a refused edit changed the entry:\n before %v\n after  %v", before, got)
	}
}

func TestEntryPatchRefusesABadPhotoListAndChangesNothing(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	const day = "2026-10-07"
	store := r.rig.entryPhotos
	a, seq := r.dayOf(day)
	id := entryObject(t, r.created(a, day, seq, store.newKey(a.id, "jpg"), store.newKey(a.id, "png"))["entry"])["id"].(string)
	before := r.entryOf(a, id)
	ids := photoIDs(t, before)
	folder := "entries/" + a.id.String()

	eleven := make([]any, 11)
	for i := range eleven {
		eleven[i] = newPhotoItem(store.newKey(a.id, "jpg"))
	}
	twice := store.newKey(a.id, "jpg")
	for _, c := range []struct {
		name   string
		photos any
		reason string
	}{
		{"an empty list", []any{}, "required"},
		{"eleven items", eleven, "too_many"},
		{"an id twice", keepPhotos(ids[0], ids[0]), "invalid"},
		{"an id twice in two spellings", keepPhotos(ids[0], strings.ToUpper(ids[0])), "invalid"},
		{"a key twice", []any{newPhotoItem(twice), newPhotoItem(twice)}, "invalid"},
		{"an id beside a key", []any{map[string]any{"id": ids[0], "photo_key": store.newKey(a.id, "jpg"), "photo_source": "camera"}}, "invalid"},
		{"neither an id nor a key", []any{map[string]any{"photo_source": "camera"}}, "invalid"},
		{"an id that is not a uuid", keepPhotos("not-a-uuid"), "invalid"},
		{"a new key with no source", []any{map[string]any{"photo_key": store.newKey(a.id, "jpg")}}, "invalid"},
		{"a new key of another person", []any{newPhotoItem(store.newKey(uuid.New(), "jpg"))}, "invalid"},
		{"a new key that is not an image", []any{newPhotoItem(folder + "/0123456789abcdef.gif")}, "invalid"},
		{"an item that is a string", []any{"x"}, "invalid"},
		{"photos is not a list", "x", "invalid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := r.call(http.MethodPatch, "/v1/entries/"+id, a.token, map[string]any{"photos": c.photos})
			wantProblem(t, rec, "photos", c.reason)
		})
	}
	if got := r.entryOf(a, id); !reflect.DeepEqual(got, before) {
		t.Fatalf("a refused edit changed the entry:\n before %v\n after  %v", before, got)
	}
	if got := store.deletes(); len(got) != 0 {
		t.Fatalf("a refused edit removed files: %v", got)
	}
}
