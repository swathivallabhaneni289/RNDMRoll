package httpapi

import (
	"context"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// The page of someone you blocked stays, empty, with relation "blocked"
// (developer, 2026-10-18, like Instagram); they still get 404 for yours, and
// Unblock brings back a plain page with no follow between you.
func TestPersonIntegration_PageOfSomeoneYouBlocked(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pb_alice", "Alice Ames")
	b := w.person("pb_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.post(b, "2026-10-08")
	w.post(a, "2026-10-08")

	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": b.id})
	page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	stats := page["stats"].(map[string]any)
	if page["relation"] != "blocked" || dig(t, page, "person", "id") != b.id || page["posts"] != nil || page["bio"] != nil ||
		stats["posts"] != float64(0) || stats["followers"] != float64(0) || page["follows_you"] != false {
		t.Fatalf("blocked page: %v", page)
	}
	w.call(b, http.MethodGet, "/v1/people/"+a.id, nil, http.StatusNotFound)

	w.call(a, http.MethodDelete, "/v1/me/blocks/"+b.id, nil, http.StatusNoContent)
	if page := w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil); page["relation"] != "none" {
		t.Fatalf("after unblock: %v", page)
	}
}

// Blocked accounts leaves out an account that is gone for everyone (PRD,
// Disabled account and Deleted account): one an admin disabled, and one
// whose owner asked for it to be deleted. The block stays, so the account is
// listed again when it comes back.
func TestBlockIntegration_BlockedAccountsLeavesOutGoneAccounts(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, engine: buildIntegrationServerWith(t, pool, integrationOptions{clock: clk}).Engine(), clk: clk}
	a := w.person("bg_alice", "Alice Ames")
	b := w.person("bg_bob", "Bob Brown")
	c := w.person("bg_cara", "Cara Cole")
	d := w.person("bg_dan", "Dan Dorn")
	for _, p := range []feedPerson{b, c, d} {
		w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": p.id})
	}
	listed := func() []string {
		t.Helper()
		ids := []string{}
		for _, item := range w.ok(a, http.MethodGet, "/v1/me/blocks", nil)["items"].([]any) {
			ids = append(ids, dig(t, item, "user", "id").(string))
		}
		sort.Strings(ids)
		return ids
	}
	setDisabled := func(p feedPerson, on bool) {
		t.Helper()
		q := `update users set disabled_at = null where id = $1`
		if on {
			q = `update users set disabled_at = now() where id = $1`
		}
		if _, err := pool.Exec(context.Background(), q, uuid.MustParse(p.id)); err != nil {
			t.Fatalf("disable %s (%v): %v", p.name, on, err)
		}
	}
	if got, want := listed(), sortedCopy(b.id, c.id, d.id); !sameList(got, want) {
		t.Fatalf("before: %v, want %v", got, want)
	}

	// An admin disables B; C asks for the account to be deleted (the password
	// feedWorld.person gives every account).
	setDisabled(b, true)
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/me/delete", c.token, map[string]any{"password": "Correct-horse-battery-staple1"}); rec.Code != http.StatusOK {
		t.Fatalf("C asks to delete: %d %s", rec.Code, rec.Body.String())
	}
	if got := listed(); len(got) != 1 || got[0] != d.id {
		t.Fatalf("gone accounts listed: %v, want only %s", got, d.id)
	}

	// B comes back, still blocked.
	setDisabled(b, false)
	if got, want := listed(), sortedCopy(b.id, d.id); !sameList(got, want) {
		t.Fatalf("after B came back: %v, want %v", got, want)
	}
}
