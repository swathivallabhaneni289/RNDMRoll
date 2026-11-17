package httpapi

// Spin, posts and the Board over the real server and the test database: who
// may see a post while a follow is not accepted, a day that stays open after
// your own post is deleted or archived, the Late flag on the cards, the whole
// post in photo order, the locked card's names, the per-person limits of the
// Board routes, the Board's page size and refusals, and the Spins figure.
// Every name in this file starts with spinposts or TestSpinposts.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// spinpostsDays lists the dates of a Board page's days, in order.
func spinpostsDays(t *testing.T, b map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, d := range b["days"].([]any) {
		out = append(out, dig(t, d, "local_date").(string))
	}
	return out
}

// spinpostsDayCards maps each day of a Board page to its cards' entry ids,
// in order. A day listed twice fails the test.
func spinpostsDayCards(t *testing.T, b map[string]any) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, d := range b["days"].([]any) {
		date := dig(t, d, "local_date").(string)
		if _, twice := out[date]; twice {
			t.Fatalf("day %s listed twice: %v", date, b)
		}
		ids := []string{}
		for _, c := range dig(t, d, "cards").([]any) {
			ids = append(ids, c.(map[string]any)["entry_id"].(string))
		}
		out[date] = ids
	}
	return out
}

// spinpostsCard is the card of entry id on a Board page, or nil.
func spinpostsCard(t *testing.T, b map[string]any, id string) map[string]any {
	t.Helper()
	for _, d := range b["days"].([]any) {
		for _, c := range dig(t, d, "cards").([]any) {
			if card := c.(map[string]any); card["entry_id"] == id {
				return card
			}
		}
	}
	return nil
}

// spinpostsAuthors lists the author id of every card of a Board page.
func spinpostsAuthors(t *testing.T, b map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, d := range b["days"].([]any) {
		for _, c := range dig(t, d, "cards").([]any) {
			out = append(out, dig(t, c, "author", "id").(string))
		}
	}
	return out
}

// spinpostsLocked reads the locked card's count and names.
func spinpostsLocked(t *testing.T, b map[string]any) (int, string) {
	t.Helper()
	if b["state"] != "locked" || b["locked"] == nil {
		t.Fatalf("not locked: %v", b)
	}
	names := []string{}
	for _, n := range dig(t, b, "locked", "names").([]any) {
		names = append(names, n.(string))
	}
	return int(dig(t, b, "locked", "posted_count").(float64)), strings.Join(names, ",")
}

// spinpostsWantPostHidden fails unless viewer's GET /v1/posts/{id} is the
// same 404, body and all, as a post that does not exist.
func spinpostsWantPostHidden(w *feedWorld, viewer feedPerson, id, label string) {
	w.t.Helper()
	got := integrationDo(w.t, w.engine, http.MethodGet, "/v1/posts/"+id, viewer.token, nil)
	miss := integrationDo(w.t, w.engine, http.MethodGet, "/v1/posts/"+uuid.NewString(), viewer.token, nil)
	if got.Code != http.StatusNotFound || miss.Code != http.StatusNotFound || got.Body.String() != miss.Body.String() {
		w.t.Fatalf("%s: post %s answered %d %s to %s, a missing post %d %s", label, id, got.Code, got.Body.String(), viewer.name, miss.Code, miss.Body.String())
	}
}

// spinpostsSpinAt makes p's daily spin for date at h:m (the shared clock
// moves there) and returns its number.
func spinpostsSpinAt(w *feedWorld, p feedPerson, date string, h, m int) int {
	w.t.Helper()
	w.at(date, h, m)
	spun := w.ok(p, http.MethodPost, "/v1/spin", map[string]any{"for_date": date})
	return int(dig(w.t, spun, "today", "spin", "seq").(float64))
}

// spinpostsTicket asks the upload ticket route for one jpeg key as p.
func spinpostsTicket(w *feedWorld, p feedPerson) string {
	w.t.Helper()
	ticket := w.ok(p, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000})
	return ticket["photo_key"].(string)
}

// spinpostsPostSpun posts p's entry for date and spin seq at the shared
// clock's time, with the photos keys (one new ticket when none), and returns
// the entry of the answer.
func spinpostsPostSpun(w *feedWorld, p feedPerson, date string, seq int, keys ...string) map[string]any {
	w.t.Helper()
	if len(keys) == 0 {
		keys = []string{spinpostsTicket(w, p)}
	}
	created := w.call(p, http.MethodPost, "/v1/entries", entryRequest(date, seq, keys...), http.StatusCreated)
	return entryObject(w.t, created["entry"])
}

// spinpostsRequestFrom is the id of from's pending follow request in to's
// incoming list.
func spinpostsRequestFrom(w *feedWorld, to, from feedPerson) string {
	w.t.Helper()
	for _, r := range w.lists(to)["incoming"].([]any) {
		if dig(w.t, r, "person", "id") == from.id {
			return dig(w.t, r, "request_id").(string)
		}
	}
	w.t.Fatalf("no request from %s to %s", from.name, to.name)
	return ""
}

// G01. Every follow is a request. While A's request to B is pending, after B
// declines it, after A unfollows and after B removes A as a follower, nothing
// of B reaches A: not B's cards, not B in the locked card's count or names,
// not B's post page. The accepted state between them is the control that the
// same reads do show B. A person whose only follow is pending has no follows.
func TestSpinpostsBoard_AFollowNotAcceptedShowsNothingOfTheAuthor(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g1_alice", "Alice Ames")
	b := w.person("g1_bob", "Bob Brown")
	c := w.person("g1_cara", "Cara Cole")
	d := w.person("g1_dan", "Dan Dorn")
	// A follows C, so A's Board is locked or open, never no_follows.
	w.followAccepted(a, c)
	if rel := w.follow(a, b); rel != "requested" {
		t.Fatalf("a's follow of b: %s", rel)
	}

	bPosts := []string{}
	// evening: B and C post at 8:30 and 8:31 PM, A's Board is read at 8:40
	// (A has not posted: locked), then A posts at 8:41.
	evening := func(date string) (aEntry, bEntry, cEntry string, locked map[string]any) {
		t.Helper()
		bEntry = w.postAt(b, date, 20, 30)
		cEntry = w.postAt(c, date, 20, 31)
		bPosts = append(bPosts, bEntry)
		w.at(date, 20, 40)
		locked = w.board(a, "")
		aEntry = w.postAt(a, date, 20, 41)
		return aEntry, bEntry, cEntry, locked
	}
	// hidden: the locked card counted and named C only, and once A posted,
	// the day shows A and C, no card on any day is B's, and every post of B is
	// the same 404 as a missing post.
	hidden := func(label, date, aEntry, cEntry string, locked map[string]any) {
		t.Helper()
		if n, names := spinpostsLocked(t, locked); n != 1 || names != "Cara" {
			t.Fatalf("%s: the locked card says %d posted, names %q; want 1, Cara", label, n, names)
		}
		got := w.board(a, "?limit=10")
		if got["state"] != "unlocked" {
			t.Fatalf("%s: after A posts: %v", label, got)
		}
		if ids := spinpostsDayCards(t, got)[date]; strings.Join(ids, ",") != aEntry+","+cEntry {
			t.Fatalf("%s: the day's cards %v, want A's %s and C's %s", label, ids, aEntry, cEntry)
		}
		if has(spinpostsAuthors(t, got), b.id) {
			t.Fatalf("%s: a card of B is on A's Board: %v", label, got)
		}
		for _, id := range bPosts {
			spinpostsWantPostHidden(w, a, id, label)
		}
	}

	// Pending.
	const d1, d2, d3, d4, d5 = "2026-10-08", "2026-10-09", "2026-10-10", "2026-10-11", "2026-10-12"
	aEntry, _, cEntry, locked := evening(d1)
	hidden("a pending request", d1, aEntry, cEntry, locked)

	// D's only follow is a request to B: no follows, even on a posted day.
	if rel := w.follow(d, b); rel != "requested" {
		t.Fatalf("d's follow of b: %s", rel)
	}
	w.postAt(d, d1, 20, 42)
	if got := w.board(d, ""); got["state"] != "no_follows" || len(got["days"].([]any)) != 0 || got["locked"] != nil {
		t.Fatalf("a person whose only follow is pending: %v", got)
	}
	spinpostsWantPostHidden(w, d, bPosts[0], "d's pending request")

	// Declined.
	w.at(d2, 19, 0)
	w.ok(b, http.MethodPost, "/v1/follows/requests/"+spinpostsRequestFrom(w, b, a)+"/decline", map[string]any{})
	if got := w.lookup(a, "g1_bob", http.StatusOK); got["relation"] != "none" {
		t.Fatalf("after the decline: %v", got)
	}
	aEntry, _, cEntry, locked = evening(d2)
	hidden("a declined request", d2, aEntry, cEntry, locked)

	// Requested again and accepted: the control. The same reads show B.
	w.at(d3, 19, 0)
	w.followAccepted(a, b)
	aEntry, bEntry, cEntry, locked := evening(d3)
	if n, names := spinpostsLocked(t, locked); n != 2 || names != "Bob,Cara" {
		t.Fatalf("accepted: the locked card says %d posted, names %q; want 2, Bob,Cara", n, names)
	}
	got := w.board(a, "?limit=10")
	if ids := spinpostsDayCards(t, got)[d3]; strings.Join(ids, ",") != aEntry+","+cEntry+","+bEntry {
		t.Fatalf("accepted: the day's cards %v", ids)
	}
	for _, id := range bPosts {
		if post := w.ok(a, http.MethodGet, "/v1/posts/"+id, nil); dig(t, post, "card", "author", "id") != b.id {
			t.Fatalf("accepted: post %s: %v", id, post)
		}
	}

	// Unfollowed.
	w.at(d4, 19, 0)
	w.ok(a, http.MethodPost, "/v1/follows/"+b.id+"/unfollow", map[string]any{})
	aEntry, _, cEntry, locked = evening(d4)
	hidden("an unfollow", d4, aEntry, cEntry, locked)

	// Accepted again (B's cards come back), then B removes A as a follower.
	w.at(d5, 19, 0)
	w.followAccepted(a, b)
	if !has(spinpostsAuthors(t, w.board(a, "?limit=10")), b.id) {
		t.Fatal("accepted again: B's cards are not back")
	}
	w.ok(b, http.MethodPost, "/v1/followers/"+a.id+"/remove", map[string]any{})
	aEntry, _, cEntry, locked = evening(d5)
	hidden("a removed follower", d5, aEntry, cEntry, locked)
}

// G02. A day stays posted after the viewer's own post is deleted or archived
// (POST-05, POST-06, the Posted day rule): the Board stays open for it and
// friends' posts still open. The viewer's own archived card is off their own
// Board, though they can open it. A posted date with no card left is not
// listed. A day the viewer skipped never shows a friend's post.
func TestSpinpostsBoard_YourOwnPostGoneStillOpensTheDay(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g2_alice", "Alice Ames")
	b := w.person("g2_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	const d5, d6, d7, d8 = "2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"

	// Oct 5: A skips and B posts.
	w.skip(a, d5)
	b5 := w.postAt(b, d5, 20, 31)
	w.at(d5, 20, 40)
	if got := w.board(a, ""); got["state"] != "skipped" || len(got["days"].([]any)) != 0 {
		t.Fatalf("the skipped evening: %v", got)
	}
	spinpostsWantPostHidden(w, a, b5, "a skipped day")

	// Oct 6: both post, then A archives its own post.
	a6 := w.postAt(a, d6, 20, 30)
	b6 := w.postAt(b, d6, 20, 31)
	w.at(d6, 20, 40)
	w.ok(a, http.MethodPost, "/v1/entries/"+a6+"/archive", map[string]any{})
	w.at(d6, 21, 0)
	got := w.board(a, "?limit=10")
	if got["state"] != "unlocked" || strings.Join(spinpostsDays(t, got), ",") != d6 {
		t.Fatalf("after A archives: %v", got)
	}
	if ids := spinpostsDayCards(t, got)[d6]; strings.Join(ids, ",") != b6 {
		t.Fatalf("after A archives, Oct 6 shows %v, want only B's %s (A's own archived card off)", ids, b6)
	}
	w.ok(a, http.MethodGet, "/v1/posts/"+b6, nil)
	if own := w.ok(a, http.MethodGet, "/v1/posts/"+a6, nil); dig(t, own, "card", "mine") != true {
		t.Fatalf("A opens its own archived post: %v", own)
	}

	// Oct 7: A posts and deletes it, then B posts at 8:45 PM.
	a7 := w.postAt(a, d7, 20, 30)
	w.at(d7, 20, 35)
	w.ok(a, http.MethodPost, "/v1/entries/"+a7+"/delete", map[string]any{})
	b7 := w.postAt(b, d7, 20, 45)
	w.at(d7, 21, 0)
	got = w.board(a, "?limit=10")
	if got["state"] != "unlocked" || strings.Join(spinpostsDays(t, got), ",") != d7+","+d6 {
		t.Fatalf("after A deletes: %v", got)
	}
	if cards := spinpostsDayCards(t, got); strings.Join(cards[d7], ",") != b7 || strings.Join(cards[d6], ",") != b6 {
		t.Fatalf("after A deletes: %v", cards)
	}
	w.ok(a, http.MethodGet, "/v1/posts/"+b7, nil)
	spinpostsWantPostHidden(w, a, a7, "A's deleted post")

	// Oct 8: A posts and deletes it, and nobody else posts. Oct 8 is posted
	// with no card: not listed, never an empty day.
	a8 := w.postAt(a, d8, 20, 30)
	w.at(d8, 20, 35)
	w.ok(a, http.MethodPost, "/v1/entries/"+a8+"/delete", map[string]any{})
	w.at(d8, 21, 0)
	got = w.board(a, "?limit=10")
	if got["state"] != "unlocked" || got["day_label"] != "today" || strings.Join(spinpostsDays(t, got), ",") != d7+","+d6 {
		t.Fatalf("a posted day with no card left: %v", got)
	}
	for date, ids := range spinpostsDayCards(t, got) {
		if len(ids) == 0 {
			t.Fatalf("day %s is listed with no cards: %v", date, got)
		}
	}

	// The skipped day never opened, whatever came after.
	spinpostsWantPostHidden(w, a, b5, "a skipped day, later")
}

// G03. SPIN-06: late posts show Late on the Board. A card's late is its
// author's posted day's flag: true for a post made after noon the next day,
// false for an on-time one, also one made after midnight and before noon.
func TestSpinpostsBoard_LatePostsSayLateOnTheCardAndThePost(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g3_alice", "Alice Ames")
	b := w.person("g3_bob", "Bob Brown")
	c := w.person("g3_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(a, c)
	w.followAccepted(b, a)
	const day, next = "2026-10-08", "2026-10-09"

	aEntry := w.postAt(a, day, 20, 30)
	bSeq := spinpostsSpinAt(w, b, day, 20, 30)
	cSeq := spinpostsSpinAt(w, c, day, 20, 35)

	// C posts at 12:30 AM: after midnight, before noon, on time.
	w.at(next, 0, 30)
	cEntry := spinpostsPostSpun(w, c, day, cSeq)
	// B posts at 12:30 PM the next day: late.
	w.at(next, 12, 30)
	bEntry := spinpostsPostSpun(w, b, day, bSeq)
	if cEntry["late"] != false || bEntry["late"] != true {
		t.Fatalf("the entry answers: c late %v, b late %v", cEntry["late"], bEntry["late"])
	}

	w.at(next, 13, 0)
	got := w.board(a, "")
	if got["state"] != "unlocked" || got["day_label"] != "last_night" {
		t.Fatalf("a's Board the next afternoon: %v", got)
	}
	for _, want := range []struct {
		id   string
		late bool
	}{{bEntry["id"].(string), true}, {aEntry, false}, {cEntry["id"].(string), false}} {
		card := spinpostsCard(t, got, want.id)
		if card == nil || card["late"] != want.late {
			t.Fatalf("a's card of %s: %v, want late %v", want.id, card, want.late)
		}
	}
	if post := w.ok(a, http.MethodGet, "/v1/posts/"+bEntry["id"].(string), nil); dig(t, post, "card", "late") != true {
		t.Fatalf("B's late post page: %v", post)
	}
	if post := w.ok(a, http.MethodGet, "/v1/posts/"+cEntry["id"].(string), nil); dig(t, post, "card", "late") != false {
		t.Fatalf("C's on-time post page: %v", post)
	}

	// From the late author's side: B's own card is late, A's is not.
	got = w.board(b, "")
	if own := spinpostsCard(t, got, bEntry["id"].(string)); own == nil || own["late"] != true || own["mine"] != true {
		t.Fatalf("b's own card: %v", own)
	}
	if card := spinpostsCard(t, got, aEntry); card == nil || card["late"] != false {
		t.Fatalf("b's card of A: %v", card)
	}
}

// G04. BRD-05: a follower's post page shows the whole post: every photo in
// position order, the first one as the cover on the post and the Board, the
// note, the emoji, the stars and the posted time. After the author puts the
// photos in a new order, the post and the Board follow it.
func TestSpinpostsPost_AFollowerSeesTheWholePostInPhotoOrder(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g4_alice", "Alice Ames")
	b := w.person("g4_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"

	seq := spinpostsSpinAt(w, b, day, 20, 30)
	k1, k2, k3 := spinpostsTicket(w, b), spinpostsTicket(w, b), spinpostsTicket(w, b)
	bEntry := spinpostsPostSpun(w, b, day, seq, k1, k2, k3)["id"].(string)
	w.postAt(a, day, 20, 40)

	photosOf := func(post map[string]any) (urls []string, positions []string, ids []string) {
		t.Helper()
		for i, p := range post["photos"].([]any) {
			ph := p.(map[string]any)
			urls = append(urls, ph["url"].(string))
			positions = append(positions, fmt.Sprint(ph["position"]))
			ids = append(ids, ph["id"].(string))
			if ph["position"] != float64(i) {
				t.Fatalf("photo %d has position %v: %v", i, ph["position"], post["photos"])
			}
		}
		return urls, positions, ids
	}

	post := w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)
	urls, positions, ids := photosOf(post)
	if strings.Join(urls, " ") != fakePhotoLink(k1)+" "+fakePhotoLink(k2)+" "+fakePhotoLink(k3) || strings.Join(positions, ",") != "0,1,2" {
		t.Fatalf("photos: %v %v, want k1 k2 k3 at 0 1 2", urls, positions)
	}
	card := post["card"].(map[string]any)
	if card["photo_url"] != fakePhotoLink(k1) || post["note"] != "Slow and worth it." || card["reaction"] != "fire" ||
		card["stars"] != float64(7) || card["posted_local_time"] != "8:30 PM" || card["title"] != "Dune" {
		t.Fatalf("the post: %v", post)
	}
	if bc := spinpostsCard(t, w.board(a, ""), bEntry); bc == nil || bc["photo_url"] != fakePhotoLink(k1) {
		t.Fatalf("the Board's cover: %v, want %s", bc, fakePhotoLink(k1))
	}

	// B keeps the third photo first, then the first, and drops the second.
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"photos": []any{
		map[string]any{"id": ids[2]}, map[string]any{"id": ids[0]},
	}})
	post = w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)
	urls, positions, _ = photosOf(post)
	if strings.Join(urls, " ") != fakePhotoLink(k3)+" "+fakePhotoLink(k1) || strings.Join(positions, ",") != "0,1" {
		t.Fatalf("photos after the new order: %v %v, want k3 k1 at 0 1", urls, positions)
	}
	if dig(t, post, "card", "photo_url") != fakePhotoLink(k3) {
		t.Fatalf("the post's cover after the new order: %v", post["card"])
	}
	if bc := spinpostsCard(t, w.board(a, ""), bEntry); bc == nil || bc["photo_url"] != fakePhotoLink(k3) {
		t.Fatalf("the Board's cover after the new order: %v, want %s", bc, fakePhotoLink(k3))
	}
}

// G05. BRD-02 and BRD-01: the locked card counts the people you follow who
// posted and names up to four of them by first name, in posting order. A
// muted person is in neither.
func TestSpinpostsBoard_TheLockedCardNamesFourInPostingOrderButNotTheMuted(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g5_alice", "Alice Ames")
	// Posting order is not the order of the names, so a list sorted by name
	// (or by anything but the time of posting) is caught.
	order := []feedPerson{
		w.person("g5_fay", "Fay Fox"),
		w.person("g5_dan", "Dan Dorn"),
		w.person("g5_bob", "Bob Brown"),
		w.person("g5_eve", "Eve East"),
		w.person("g5_cara", "Cara Cole"),
	}
	const day = "2026-10-08"
	for i, p := range order {
		w.followAccepted(a, p)
		w.postAt(p, day, 20, 30+i)
	}
	w.at(day, 20, 40)
	locked := func(label string, wantCount int, wantNames string) {
		t.Helper()
		if n, names := spinpostsLocked(t, w.board(a, "")); n != wantCount || names != wantNames {
			t.Fatalf("%s: %d posted, names %q; want %d, %q", label, n, names, wantCount, wantNames)
		}
	}
	locked("five posted", 5, "Fay,Dan,Bob,Eve")

	dan := order[1]
	w.ok(a, http.MethodPost, "/v1/people/"+dan.id+"/mute", map[string]any{})
	locked("Dan muted", 4, "Fay,Bob,Eve,Cara")
	w.ok(a, http.MethodPost, "/v1/people/"+dan.id+"/unmute", map[string]any{})
	locked("Dan unmuted", 5, "Fay,Dan,Bob,Eve")
}

// G06, the Board half. GET /v1/board, GET /v1/posts/:id, GET /v1/diary and
// GET /v1/me/summary each answer 429 rate_limited after 60 calls a minute,
// per person. The limit clock stands still for the test.
func TestSpinpostsBoardRoutes_AreLimitedPerPersonAMinute(t *testing.T) {
	w := newFeedWorld(t)
	for i, c := range []struct {
		name, path string
		limit      int
	}{
		{"board", "/v1/board", 60},
		{"post", "/v1/posts/" + uuid.NewString(), 60},
		{"diary", "/v1/diary", 60},
		{"summary", "/v1/me/summary", 60},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := w.person(fmt.Sprintf("g6_one_%d", i), "Limit One")
			for n := 1; n <= c.limit; n++ {
				if rec := integrationDo(t, w.engine, http.MethodGet, c.path, p.token, nil); rec.Code == http.StatusTooManyRequests {
					t.Fatalf("request %d of %d was limited: %s", n, c.limit, rec.Body.String())
				}
			}
			rec := integrationDo(t, w.engine, http.MethodGet, c.path, p.token, nil)
			if rec.Code != http.StatusTooManyRequests || decodeBody(t, rec)["error"] != "rate_limited" {
				t.Fatalf("request %d: %d %s, want 429 rate_limited", c.limit+1, rec.Code, rec.Body.String())
			}
			other := w.person(fmt.Sprintf("g6_two_%d", i), "Limit Two")
			if rec := integrationDo(t, w.engine, http.MethodGet, c.path, other.token, nil); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("another person was limited: %s", rec.Body.String())
			}
		})
	}
}

// G11. The Board's page is 5 days by default and up to 10 (section 10,
// Performance). A post id that is not a UUID is 404 not_found on the post,
// its reaction and its comments. An account with no zone gets the time_zone
// field error from the Board, never a 500.
func TestSpinpostsBoard_PageSizeAndRefusals(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("g11_alice", "Alice Ames")
	b := w.person("g11_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	dates := []string{"2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"}
	for _, d := range dates {
		w.postAt(a, d, 20, 30)
		w.postAt(b, d, 20, 31)
	}
	w.at("2026-10-08", 21, 0)

	got := w.board(a, "")
	if days := spinpostsDays(t, got); strings.Join(days, ",") != "2026-10-08,2026-10-07,2026-10-06,2026-10-05,2026-10-04" || got["next_before"] != "2026-10-04" {
		t.Fatalf("the default page: %v next_before %v", days, got["next_before"])
	}
	got = w.board(a, "?limit=10")
	if days := spinpostsDays(t, got); len(days) != 7 || got["next_before"] != nil {
		t.Fatalf("limit=10: %v next_before %v", days, got["next_before"])
	}
	for _, d := range spinpostsDayCards(t, got) {
		if len(d) != 2 {
			t.Fatalf("every day has both posts: %v", got)
		}
	}

	for _, r := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/v1/posts/not-a-uuid", nil},
		{http.MethodPost, "/v1/posts/not-a-uuid/reaction", map[string]any{"reaction": "fire"}},
		{http.MethodPost, "/v1/posts/not-a-uuid/comments", map[string]any{"text": "Hi"}},
	} {
		rec := integrationDo(t, w.engine, r.method, r.path, a.token, r.body)
		if rec.Code != http.StatusNotFound || rec.Body.String() != `{"error":"not_found"}` {
			t.Fatalf("%s %s: %d %s, want 404 not_found", r.method, r.path, rec.Code, rec.Body.String())
		}
	}

	s := integrationOnboardAccount(t, w.engine, "g11_zoe@example.com", "Correct-horse-battery-staple1", "Zoe Zone", "g11_zoe")
	nozone := feedPerson{name: "Zoe Zone", token: s.AccessToken}
	if got := w.call(nozone, http.MethodGet, "/v1/board", nil, http.StatusBadRequest); got["error"] != "validation_failed" || got["field"] != "time_zone" {
		t.Fatalf("the Board with no zone: %v", got)
	}
}

// G12. Section 9: the Spins figure counts days spun, and a bonus spin does
// not add one. Pretend posted days are not spins either.
func TestSpinpostsSpinsFigure_ABonusSpinAddsNone(t *testing.T) {
	pool := requireIntegrationPool(t)
	const email = "g12_sam@example.com"
	engine := buildIntegrationServerWith(t, pool, integrationOptions{
		clock: clock.NewPerUser(), toolEmails: []string{email},
	}).Engine()
	sam := integrationOnboardAccount(t, engine, email, "Correct-horse-battery-staple1", "Sam Spinner", "g12_sam")
	fan := integrationOnboardAccount(t, engine, "g12_fan@example.com", "Correct-horse-battery-staple1", "Fan Follower", "g12_fan")

	do := func(tok, method, path string, body any, want int) map[string]any {
		t.Helper()
		rec := integrationDo(t, engine, method, path, tok, body)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body.String(), want)
		}
		return decodeBody(t, rec)
	}
	samID := do(sam.AccessToken, http.MethodGet, "/v1/me", nil, http.StatusOK)["id"].(string)
	for _, tok := range []string{sam.AccessToken, fan.AccessToken} {
		do(tok, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": "America/New_York"}, http.StatusOK)
	}

	// Four pretend posted days, Monday to Thursday: a bonus is held.
	do(sam.AccessToken, http.MethodPost, "/v1/test/clock", map[string]any{"local": "2026-10-08T12:00:00"}, http.StatusOK)
	if got := do(sam.AccessToken, http.MethodPost, "/v1/test/posted-days", map[string]any{
		"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"},
	}, http.StatusOK); got["bonus_held"] != true {
		t.Fatalf("posted days: %v", got)
	}

	// Friday 8:30 PM: the daily spin, the bonus spin, then the post.
	do(sam.AccessToken, http.MethodPost, "/v1/test/clock", map[string]any{"local": "2026-10-09T20:30:00"}, http.StatusOK)
	do(sam.AccessToken, http.MethodPost, "/v1/spin", map[string]any{"for_date": "2026-10-09"}, http.StatusCreated)
	bonus := do(sam.AccessToken, http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": "2026-10-09"}, http.StatusCreated)
	seq := int(dig(t, bonus, "today", "spin", "seq").(float64))
	if seq != 1 {
		t.Fatalf("the bonus spin's number: %d, want 1", seq)
	}
	ticket := do(sam.AccessToken, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000}, http.StatusOK)
	do(sam.AccessToken, http.MethodPost, "/v1/entries", entryRequest("2026-10-09", seq, ticket["photo_key"].(string)), http.StatusCreated)

	if got := do(sam.AccessToken, http.MethodGet, "/v1/me/summary", nil, http.StatusOK); got["spins"] != float64(1) {
		t.Fatalf("your own Spins figure after a bonus spin: %v, want 1", got["spins"])
	}

	// An accepted follower reads the same figure on the person page.
	if got := do(fan.AccessToken, http.MethodPost, "/v1/follows", map[string]any{"user_id": samID}, http.StatusOK); got["relation"] != "requested" {
		t.Fatalf("follow: %v", got)
	}
	incoming := do(sam.AccessToken, http.MethodGet, "/v1/follows", nil, http.StatusOK)["incoming"].([]any)
	if len(incoming) != 1 {
		t.Fatalf("incoming: %v", incoming)
	}
	do(sam.AccessToken, http.MethodPost, "/v1/follows/requests/"+dig(t, incoming[0], "request_id").(string)+"/accept", map[string]any{}, http.StatusOK)
	page := do(fan.AccessToken, http.MethodGet, "/v1/people/"+samID, nil, http.StatusOK)
	if page["relation"] != "following" || dig(t, page, "stats", "spins") != float64(1) {
		t.Fatalf("the person page's Spins figure after a bonus spin: %v, want 1", page)
	}
}
