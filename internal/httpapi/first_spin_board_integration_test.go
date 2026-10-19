package httpapi

import (
	"net/http"
	"testing"
)

// postAt spins and posts p's first spin for date at h:m local (before 8 PM,
// so only a first spin may do it) and returns the entry id.
func (w *feedWorld) postAt(p feedPerson, date string, h, m int) string {
	w.t.Helper()
	w.at(date, h, m)
	spun := w.ok(p, http.MethodPost, "/v1/spin", map[string]any{"for_date": date})
	seq := int(dig(w.t, spun, "today", "spin", "seq").(float64))
	ticket := w.ok(p, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000})
	created := w.call(p, http.MethodPost, "/v1/entries", entryRequest(date, seq, ticket["photo_key"].(string)), http.StatusCreated)
	return entryObject(w.t, created["entry"])["id"].(string)
}

func TestBoardIntegration_AFirstSpinPostedInTheMorningOpensTodaysBoard(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fb_alice", "Alice Ames")
	b := w.person("fb_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"

	// B, also new, makes a first spin at 9 AM and posts it.
	bEntry := w.postAt(b, day, 9, 0)

	// A has never spun: the Board is already on today (never "last night"
	// for a new person), locked until A posts, offering the first spin.
	w.at(day, 10, 0)
	if got := w.board(a, ""); got["state"] != "locked" || got["day_label"] != "today" ||
		dig(t, got, "locked", "action") != "spin" {
		t.Fatalf("before A posts: %v", got)
	}

	// A's first spin, posted at 10:30 AM: the Board is on today at once,
	// with A's post and B's.
	aEntry := w.postAt(a, day, 10, 30)
	got := w.board(a, "")
	if got["state"] == "locked" || got["day_label"] != "today" {
		t.Fatalf("after A posts: %v", got)
	}
	if ids := cardIDs(t, got); !has(ids, aEntry) || !has(ids, bEntry) {
		t.Fatalf("cards: %v", ids)
	}
	w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)

	// Someone who has spun before is still on last night before 8 PM.
	c := w.person("fb_cara", "Cara Cole")
	w.followAccepted(c, b)
	w.post(c, "2026-10-06")
	w.at(day, 11, 0)
	if got := w.board(c, ""); got["day_label"] != "last_night" {
		t.Fatalf("a person with an earlier spin: %v", got)
	}
}
