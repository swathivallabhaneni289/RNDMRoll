package httpapi

import (
	"net/http"
	"strings"
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

func TestBoardIntegration_AFirstSpinNotPostedIsOnToday(t *testing.T) {
	w := newFeedWorld(t)
	b := w.person("fu_bob", "Bob Brown")
	// B posts the evening before: last night's post.
	bEntry := w.post(b, "2026-10-07")
	a := w.person("fu_alice", "Alice Ames")
	d := w.person("fu_dana", "Dana Dean")
	w.followAccepted(a, b)
	w.followAccepted(d, b)
	const day = "2026-10-08"

	// A, new, makes a first spin at 3 PM and does not post. The spin is
	// today's, so the Board is on today, where nobody A follows has posted:
	// B's post from last night is not offered, since no post of A's could
	// open it.
	w.at(day, 15, 0)
	w.ok(a, http.MethodPost, "/v1/spin", map[string]any{"for_date": day})
	got := w.board(a, "")
	if got["state"] != "locked" || got["day_label"] != "today" || dig(t, got, "locked", "posted_count") != float64(0) ||
		len(dig(t, got, "locked", "names").([]any)) != 0 || dig(t, got, "locked", "action") != "log" ||
		dig(t, got, "locked", "post_late") != false {
		t.Fatalf("an unposted first spin: %v", got)
	}

	// A skips it: the Board says so, for today.
	w.ok(a, http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": day, "spin_seq": 0})
	if got := w.board(a, ""); got["state"] != "skipped" || got["day_label"] != "today" || got["locked"] != nil {
		t.Fatalf("a skipped first spin: %v", got)
	}

	// D, new, posts a first spin: today, with D's own card only.
	dEntry := w.postAt(d, day, 15, 30)
	got = w.board(d, "")
	if got["state"] != "unlocked" || got["day_label"] != "today" || strings.Join(cardIDs(t, got), ",") != dEntry {
		t.Fatalf("a posted first spin: %v", got)
	}
	w.call(d, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
}
