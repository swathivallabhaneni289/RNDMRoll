package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// feedWorld is a real server over the test database with one Fixed clock
// shared by every account, all in New York.
type feedWorld struct {
	t      *testing.T
	engine *gin.Engine
	clk    *clock.Fixed
}

const feedZone = "America/New_York"

type feedPerson struct {
	name  string
	token string
	id    string
}

func newFeedWorld(t *testing.T) *feedWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk}).Engine()
	return &feedWorld{t: t, engine: engine, clk: clk}
}

// at moves the shared clock to a New York wall time.
func (w *feedWorld) at(date string, h, m int) {
	w.t.Helper()
	w.clk.Set(localInstant(w.t, feedZone, date, h, m, 0))
}

// person signs up a finished account and reports its zone.
func (w *feedWorld) person(username, name string) feedPerson {
	w.t.Helper()
	s := integrationOnboardAccount(w.t, w.engine, username+"@example.com", "Correct-horse-battery-staple1", name, username)
	p := feedPerson{name: name, token: s.AccessToken}
	me := w.ok(p, http.MethodGet, "/v1/me", nil)
	p.id = me["id"].(string)
	w.ok(p, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": feedZone})
	return p
}

// call sends one request as p and checks the status.
func (w *feedWorld) call(p feedPerson, method, path string, body any, want int) map[string]any {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, method, path, p.token, body)
	if rec.Code != want {
		w.t.Fatalf("%s %s as %s: %d %s, want %d", method, path, p.name, rec.Code, rec.Body.String(), want)
	}
	if rec.Body.Len() == 0 {
		return nil
	}
	if isFeedPath(path) {
		for _, bad := range []string{`"email"`, `"birthday"`, `"bio"`, `"password_hash"`, `"photo_key"`, `"avatar_url"`, `"storage_key"`} {
			if strings.Contains(rec.Body.String(), bad) {
				w.t.Fatalf("%s %s carries %s: %s", method, path, bad, rec.Body.String())
			}
		}
	}
	return decodeBody(w.t, rec)
}

// isFeedPath reports a Phase 5 route, whose bodies describe other people.
func isFeedPath(path string) bool {
	for _, p := range []string{"/v1/people/", "/v1/follows", "/v1/followers/", "/v1/me/blocks", "/v1/me/notification-settings",
		"/v1/board", "/v1/posts/", "/v1/comments/", "/v1/notifications"} {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func (w *feedWorld) ok(p feedPerson, method, path string, body any) map[string]any {
	w.t.Helper()
	want := http.StatusOK
	if method == http.MethodPost && path == "/v1/spin" {
		want = http.StatusCreated
	}
	return w.call(p, method, path, body, want)
}

// post spins and posts p's entry for date at 8:30 PM and returns its id.
func (w *feedWorld) post(p feedPerson, date string) string {
	w.t.Helper()
	w.at(date, 20, 30)
	spun := w.ok(p, http.MethodPost, "/v1/spin", map[string]any{"for_date": date})
	seq := int(dig(w.t, spun, "today", "spin", "seq").(float64))
	ticket := w.ok(p, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000})
	created := w.call(p, http.MethodPost, "/v1/entries", entryRequest(date, seq, ticket["photo_key"].(string)), http.StatusCreated)
	return entryObject(w.t, created["entry"])["id"].(string)
}

// skip spins and skips p's day.
func (w *feedWorld) skip(p feedPerson, date string) {
	w.t.Helper()
	w.at(date, 20, 30)
	spun := w.ok(p, http.MethodPost, "/v1/spin", map[string]any{"for_date": date})
	seq := int(dig(w.t, spun, "today", "spin", "seq").(float64))
	w.ok(p, http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": date, "spin_seq": seq})
}

func (w *feedWorld) follow(p, target feedPerson) string {
	w.t.Helper()
	return w.ok(p, http.MethodPost, "/v1/follows", map[string]any{"user_id": target.id})["relation"].(string)
}

func (w *feedWorld) setPrivate(p feedPerson, private bool) {
	w.t.Helper()
	v := "public"
	if private {
		v = "private"
	}
	w.ok(p, http.MethodPatch, "/v1/me", map[string]any{"account_visibility": v})
}

func (w *feedWorld) lists(p feedPerson) map[string]any {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/follows", nil)
}

// usernames returns the usernames of a list of persons (or of requests).
func usernames(t *testing.T, list any) []string {
	t.Helper()
	out := []string{}
	for _, v := range list.([]any) {
		m := v.(map[string]any)
		if p, ok := m["person"]; ok {
			m = p.(map[string]any)
		}
		out = append(out, m["username"].(string))
	}
	return out
}

func (w *feedWorld) lookup(p feedPerson, username string, want int) map[string]any {
	w.t.Helper()
	return w.call(p, http.MethodGet, "/v1/people/lookup?username="+username, nil, want)
}

func (w *feedWorld) notes(p feedPerson) map[string]any {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/notifications", nil)
}

// kinds lists the notification kinds in order.
func kinds(t *testing.T, list map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, v := range list["items"].([]any) {
		m := v.(map[string]any)
		if m["at_label"] == "" || m["section"] == "" {
			t.Fatalf("item without a label or section: %v", m)
		}
		out = append(out, m["kind"].(string))
	}
	return out
}

func itemOf(t *testing.T, list map[string]any, kind string) map[string]any {
	t.Helper()
	for _, v := range list["items"].([]any) {
		if m := v.(map[string]any); m["kind"] == kind {
			return m
		}
	}
	t.Fatalf("no %s item in %v", kind, list)
	return nil
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (w *feedWorld) followers(p feedPerson) float64 {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/me/summary", nil)["followers"].(float64)
}

func TestFollowIntegration_PublicAccountIsFollowedAtOnce(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fa_alice", "Alice Ames")
	b := w.person("fa_bob", "Bob Brown")

	got := w.lookup(a, "@FA_Bob", http.StatusOK)
	if got["relation"] != "none" || got["follows_you"] != false || got["private"] != false || dig(t, got, "person", "id") != b.id {
		t.Fatalf("lookup before: %v", got)
	}
	if rel := w.follow(a, b); rel != "following" {
		t.Fatalf("follow a public account: %s", rel)
	}
	if rel := w.follow(a, b); rel != "following" {
		t.Fatalf("repeat follow: %s", rel)
	}
	if got := w.lookup(a, "fa_bob", http.StatusOK); got["relation"] != "following" {
		t.Fatalf("lookup after: %v", got)
	}
	if got := w.lookup(b, "fa_alice", http.StatusOK); got["relation"] != "none" || got["follows_you"] != true {
		t.Fatalf("one way only: %v", got)
	}
	if l := w.lists(b); strings.Join(usernames(t, l["followers"]), ",") != "fa_alice" || len(l["following"].([]any)) != 0 {
		t.Fatalf("b's lists: %v", l)
	}
	if l := w.lists(a); strings.Join(usernames(t, l["following"]), ",") != "fa_bob" || len(l["followers"].([]any)) != 0 {
		t.Fatalf("a's lists: %v", l)
	}
	n := w.notes(b)
	item := itemOf(t, n, "follow")
	if dig(t, item, "actor", "id") != a.id || item["unread"] != true || item["request_id"] != nil || n["unread"] != true {
		t.Fatalf("follow item: %v", n)
	}
	if k := kinds(t, w.notes(a)); has(k, "follow") || has(k, "follow_accepted") {
		t.Fatalf("the follower is told nothing: %v", k)
	}
	if w.followers(b) != 1 || w.followers(a) != 0 {
		t.Fatalf("followers: b %v a %v", w.followers(b), w.followers(a))
	}

	// Yourself, a miss, a bad shape and a bad id.
	w.lookup(a, "fa_alice", http.StatusNotFound)
	w.lookup(a, "nobody_here", http.StatusNotFound)
	if got := w.lookup(a, "a!", http.StatusBadRequest); got["field"] != "username" {
		t.Fatalf("bad shape: %v", got)
	}
	w.call(a, http.MethodPost, "/v1/follows", map[string]any{"user_id": a.id}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/follows", map[string]any{"user_id": "00000000-0000-0000-0000-000000000001"}, http.StatusNotFound)
	if got := w.call(a, http.MethodPost, "/v1/follows", map[string]any{"user_id": "x"}, http.StatusBadRequest); got["field"] != "user_id" {
		t.Fatalf("bad user_id: %v", got)
	}
	w.call(a, http.MethodPost, "/v1/follows/not-a-uuid/unfollow", map[string]any{}, http.StatusNotFound)
	if rec := integrationDo(t, w.engine, http.MethodGet, "/v1/follows", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestFollowIntegration_PrivateAccountRequestAcceptAndDecline(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fp_alice", "Alice Ames")
	b := w.person("fp_bob", "Bob Brown")
	c := w.person("fp_cara", "Cara Cole")
	w.setPrivate(b, true)

	if got := w.lookup(a, "fp_bob", http.StatusOK); got["private"] != true {
		t.Fatalf("private flag: %v", got)
	}
	if rel := w.follow(a, b); rel != "requested" {
		t.Fatalf("follow a private account: %s", rel)
	}
	if rel := w.follow(a, b); rel != "requested" {
		t.Fatalf("repeat request: %s", rel)
	}
	if got := w.lookup(a, "fp_bob", http.StatusOK); got["relation"] != "requested" {
		t.Fatalf("lookup requested: %v", got)
	}
	bl := w.lists(b)
	incoming := bl["incoming"].([]any)
	if len(incoming) != 1 || dig(t, incoming[0], "person", "id") != a.id || len(bl["followers"].([]any)) != 0 {
		t.Fatalf("b's incoming: %v", bl)
	}
	reqID := dig(t, incoming[0], "request_id").(string)
	if out := w.lists(a)["outgoing"].([]any); len(out) != 1 || dig(t, out[0], "request_id") != reqID {
		t.Fatalf("a's outgoing: %v", out)
	}
	if w.followers(b) != 0 {
		t.Fatal("a pending request is not a follower")
	}
	item := itemOf(t, w.notes(b), "follow_request")
	if item["request_id"] != reqID || dig(t, item, "actor", "id") != a.id {
		t.Fatalf("request item: %v", item)
	}

	// Only the addressee can accept; a stranger gets the same 404.
	w.call(c, http.MethodPost, "/v1/follows/requests/"+reqID+"/accept", map[string]any{}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/follows/requests/"+reqID+"/accept", map[string]any{}, http.StatusNotFound)
	w.clk.Advance(time.Minute)
	w.ok(b, http.MethodPost, "/v1/follows/requests/"+reqID+"/accept", map[string]any{})
	w.call(b, http.MethodPost, "/v1/follows/requests/"+reqID+"/accept", map[string]any{}, http.StatusNotFound)
	if got := w.lookup(a, "fp_bob", http.StatusOK); got["relation"] != "following" {
		t.Fatalf("after accept: %v", got)
	}
	if k := kinds(t, w.notes(b)); has(k, "follow_request") || has(k, "follow") {
		t.Fatalf("b after accept: %v", k)
	}
	acc := itemOf(t, w.notes(a), "follow_accepted")
	if dig(t, acc, "actor", "id") != b.id {
		t.Fatalf("accepted item: %v", acc)
	}
	if w.followers(b) != 1 {
		t.Fatal("accepted follower counts")
	}

	// Decline is silent.
	if rel := w.follow(c, b); rel != "requested" {
		t.Fatalf("c requests: %s", rel)
	}
	cReq := dig(t, w.lists(b)["incoming"].([]any)[0], "request_id").(string)
	w.ok(b, http.MethodPost, "/v1/follows/requests/"+cReq+"/decline", map[string]any{})
	if got := w.lookup(c, "fp_bob", http.StatusOK); got["relation"] != "none" {
		t.Fatalf("after decline: %v", got)
	}
	if len(w.lists(b)["incoming"].([]any)) != 0 || len(w.lists(c)["outgoing"].([]any)) != 0 {
		t.Fatal("a declined request is gone")
	}
	if k := kinds(t, w.notes(c)); len(k) != 0 && (has(k, "follow_accepted") || has(k, "follow_request")) {
		t.Fatalf("c is told nothing: %v", k)
	}
	w.call(b, http.MethodPost, "/v1/follows/requests/"+cReq+"/decline", map[string]any{}, http.StatusNotFound)
	w.call(b, http.MethodPost, "/v1/follows/requests/not-a-uuid/decline", map[string]any{}, http.StatusNotFound)
}

func TestFollowIntegration_CancelUnfollowAndRemoveFollower(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fc_alice", "Alice Ames")
	b := w.person("fc_bob", "Bob Brown")
	c := w.person("fc_cara", "Cara Cole")
	d := w.person("fc_dan", "Dan Dorn")
	w.setPrivate(b, true)

	// Cancel a request.
	w.follow(a, b)
	if got := w.ok(a, http.MethodPost, "/v1/follows/"+b.id+"/unfollow", map[string]any{}); got["relation"] != "none" {
		t.Fatalf("cancel: %v", got)
	}
	if len(w.lists(b)["incoming"].([]any)) != 0 || len(w.lists(a)["outgoing"].([]any)) != 0 {
		t.Fatal("a cancelled request is gone")
	}
	if k := kinds(t, w.notes(b)); has(k, "follow_request") {
		t.Fatalf("the request item goes with it: %v", k)
	}

	// Unfollow.
	w.follow(a, c)
	w.ok(a, http.MethodPost, "/v1/follows/"+c.id+"/unfollow", nil)
	if got := w.lookup(a, "fc_cara", http.StatusOK); got["relation"] != "none" {
		t.Fatalf("unfollow: %v", got)
	}
	if len(w.lists(c)["followers"].([]any)) != 0 || w.followers(c) != 0 {
		t.Fatal("unfollowed")
	}
	if k := kinds(t, w.notes(c)); has(k, "follow") {
		t.Fatalf("the follow item goes too: %v", k)
	}
	w.ok(a, http.MethodPost, "/v1/follows/"+c.id+"/unfollow", map[string]any{})

	// Remove a follower.
	w.follow(d, a)
	if w.followers(a) != 1 {
		t.Fatal("d follows a")
	}
	w.ok(a, http.MethodPost, "/v1/followers/"+d.id+"/remove", map[string]any{})
	if w.followers(a) != 0 || len(w.lists(d)["following"].([]any)) != 0 {
		t.Fatal("removed follower")
	}
	if got := w.lookup(d, "fc_alice", http.StatusOK); got["relation"] != "none" {
		t.Fatalf("d after removal: %v", got)
	}
	w.ok(a, http.MethodPost, "/v1/followers/"+d.id+"/remove", map[string]any{})
}

func TestFollowIntegration_SwitchToPublicAcceptsPendingSilently(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fs_alice", "Alice Ames")
	b := w.person("fs_bob", "Bob Brown")
	w.setPrivate(b, true)
	w.follow(a, b)
	w.clk.Advance(time.Minute)

	// Switching to private again keeps it pending; public accepts it.
	w.setPrivate(b, true)
	if got := w.lookup(a, "fs_bob", http.StatusOK); got["relation"] != "requested" {
		t.Fatalf("still pending: %v", got)
	}
	w.setPrivate(b, false)
	if got := w.lookup(a, "fs_bob", http.StatusOK); got["relation"] != "following" {
		t.Fatalf("after the switch: %v", got)
	}
	if k := kinds(t, w.notes(a)); has(k, "follow_accepted") {
		t.Fatalf("no item for the requester: %v", k)
	}
	if k := kinds(t, w.notes(b)); has(k, "follow") || has(k, "follow_request") {
		t.Fatalf("no item for the owner: %v", k)
	}
	// Switching back to private keeps the follower.
	w.setPrivate(b, true)
	if w.followers(b) != 1 {
		t.Fatal("private keeps followers")
	}
}

func TestBlockIntegration_RemovesFollowsBothWaysAndHidesLookup(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("fb_alice", "Alice Ames")
	b := w.person("fb_bob", "Bob Brown")
	w.follow(a, b)
	w.follow(b, a)

	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": b.id})
	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": b.id})
	for _, p := range []feedPerson{a, b} {
		l := w.lists(p)
		if len(l["followers"].([]any)) != 0 || len(l["following"].([]any)) != 0 {
			t.Fatalf("%s's lists after the block: %v", p.name, l)
		}
		if w.followers(p) != 0 {
			t.Fatalf("%s still counts a follower", p.name)
		}
	}
	miss := w.lookup(b, "nobody_here", http.StatusNotFound)
	blocked := w.lookup(b, "fb_alice", http.StatusNotFound)
	if miss["error"] != blocked["error"] || len(miss) != len(blocked) {
		t.Fatalf("a block looks like a miss: %v %v", miss, blocked)
	}
	w.lookup(a, "fb_bob", http.StatusNotFound)
	w.call(b, http.MethodPost, "/v1/follows", map[string]any{"user_id": a.id}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/follows", map[string]any{"user_id": b.id}, http.StatusNotFound)
	if k := kinds(t, w.notes(a)); has(k, "follow") {
		t.Fatalf("a blocked person's items are gone: %v", k)
	}

	got := w.ok(a, http.MethodGet, "/v1/me/blocks", nil)
	items := got["items"].([]any)
	if len(items) != 1 || dig(t, items[0], "user", "id") != b.id || dig(t, items[0], "user", "username") != "fb_bob" ||
		dig(t, items[0], "user", "name") != "Bob Brown" || dig(t, items[0], "blocked_at") == "" {
		t.Fatalf("blocks list: %v", got)
	}
	if got := w.ok(b, http.MethodGet, "/v1/me/blocks", nil); len(got["items"].([]any)) != 0 {
		t.Fatalf("the blocked person's list: %v", got)
	}
	w.call(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": "00000000-0000-0000-0000-000000000001"}, http.StatusNotFound)

	// Unblock: 204 with no body; nothing comes back.
	if got := w.call(a, http.MethodDelete, "/v1/me/blocks/"+b.id, nil, http.StatusNoContent); got != nil {
		t.Fatalf("204 body: %v", got)
	}
	w.call(a, http.MethodDelete, "/v1/me/blocks/"+b.id, nil, http.StatusNoContent)
	w.call(a, http.MethodDelete, "/v1/me/blocks/nope", nil, http.StatusNotFound)
	if got := w.lookup(b, "fb_alice", http.StatusOK); got["relation"] != "none" || got["follows_you"] != false {
		t.Fatalf("after unblock: %v", got)
	}
}

// board reads p's Board.
func (w *feedWorld) board(p feedPerson, query string) map[string]any {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/board"+query, nil)
}

// cardIDs lists the entry ids of every card of a Board, day by day.
func cardIDs(t *testing.T, b map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, d := range b["days"].([]any) {
		for _, c := range dig(t, d, "cards").([]any) {
			card := c.(map[string]any)
			if card["posted_local_time"] == "" || card["photo_url"] == "" {
				t.Fatalf("card without a time or a photo: %v", card)
			}
			out = append(out, card["entry_id"].(string))
		}
	}
	return out
}

func TestBoardIntegration_TheGate(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("bg_alice", "Alice Ames")
	b := w.person("bg_bob", "Bob Brown")
	c := w.person("bg_cara", "Cara Cole")
	loner := w.person("bg_lone", "Lone Wolf")
	w.follow(a, b)
	w.follow(c, b)

	const day = "2026-10-08"
	bEntry := w.post(b, day)

	if got := w.board(loner, ""); got["state"] != "no_follows" || len(got["days"].([]any)) != 0 || got["locked"] != nil {
		t.Fatalf("no follows: %v", got)
	}

	// A has not posted: locked, and B's post is 404 (the same body as a miss).
	got := w.board(a, "")
	if got["state"] != "locked" || got["day_label"] != "today" || dig(t, got, "locked", "posted_count") != float64(1) ||
		dig(t, got, "locked", "action") != "spin" || len(got["days"].([]any)) != 0 {
		t.Fatalf("locked: %v", got)
	}
	if names := dig(t, got, "locked", "names").([]any); len(names) != 1 || names[0] != "Bob" {
		t.Fatalf("locked names: %v", names)
	}
	hidden := w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	missing := w.call(a, http.MethodGet, "/v1/posts/00000000-0000-0000-0000-000000000001", nil, http.StatusNotFound)
	if hidden["error"] != missing["error"] || len(hidden) != len(missing) {
		t.Fatalf("hidden and missing differ: %v %v", hidden, missing)
	}
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/reaction", map[string]any{"reaction": "fire"}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"text": "hi"}, http.StatusNotFound)

	// C skips: skipped, and the post stays hidden.
	w.skip(c, day)
	if got := w.board(c, ""); got["state"] != "skipped" || got["locked"] != nil || len(got["days"].([]any)) != 0 {
		t.Fatalf("skipped: %v", got)
	}
	w.call(c, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)

	// After A spins the locked card offers to log the spin.
	w.at(day, 20, 40)
	spun := w.ok(a, http.MethodPost, "/v1/spin", map[string]any{"for_date": day})
	if got := w.board(a, ""); dig(t, got, "locked", "action") != "log" || dig(t, got, "locked", "category") != dig(t, spun, "today", "spin", "category", "name") {
		t.Fatalf("locked after a spin: %v", got)
	}

	// A posts: unlocked with both cards, the post opens.
	ticket := w.ok(a, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000})
	seq := int(dig(t, spun, "today", "spin", "seq").(float64))
	w.clk.Advance(time.Minute)
	created := w.call(a, http.MethodPost, "/v1/entries", entryRequest(day, seq, ticket["photo_key"].(string)), http.StatusCreated)
	aEntry := entryObject(t, created["entry"])["id"].(string)
	got = w.board(a, "")
	if got["state"] != "unlocked" || got["locked"] != nil {
		t.Fatalf("unlocked: %v", got)
	}
	if ids := cardIDs(t, got); strings.Join(ids, ",") != aEntry+","+bEntry {
		t.Fatalf("cards newest first: %v", ids)
	}
	if label := dig(t, got["days"].([]any)[0], "label"); label != "today" {
		t.Fatalf("day label: %v", label)
	}
	post := w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)
	if post["can_react"] != true || post["can_comment"] != true || post["received"] != nil || dig(t, post, "card", "mine") != false ||
		dig(t, post, "card", "posted_local_time") != "8:30 PM" || len(post["photos"].([]any)) != 1 {
		t.Fatalf("post: %v", post)
	}
	photos := post["photos"].([]any)
	ph := photos[0].(map[string]any)
	if len(ph) != 3 || ph["id"] == "" || ph["position"] != float64(0) || ph["url"] != dig(t, post, "card", "photo_url") ||
		!strings.HasPrefix(ph["url"].(string), fakeEntryPhotoPublicBase) {
		t.Fatalf("post photos: %v", photos)
	}
	if own := w.ok(a, http.MethodGet, "/v1/posts/"+aEntry, nil); own["can_react"] != false || own["received"] == nil {
		t.Fatalf("own post: %v", own)
	}
	// B does not follow A: A's post stays hidden from B.
	w.call(b, http.MethodGet, "/v1/posts/"+aEntry, nil, http.StatusNotFound)
	if got := w.board(b, ""); got["state"] != "no_follows" || len(got["days"].([]any)) != 0 {
		t.Fatalf("b follows nobody: %v", got)
	}

	// Archived: the owner still reads it; nobody else sees it.
	w.ok(b, http.MethodPost, "/v1/entries/"+bEntry+"/archive", map[string]any{})
	w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	if ids := cardIDs(t, w.board(a, "")); strings.Join(ids, ",") != aEntry {
		t.Fatalf("archived card listed: %v", ids)
	}
	w.ok(b, http.MethodGet, "/v1/posts/"+bEntry, nil)
	w.ok(b, http.MethodPost, "/v1/entries/"+bEntry+"/unarchive", map[string]any{})
	w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)

	// Earlier days and paging: the next evening A is locked, yesterday stays readable.
	const next = "2026-10-09"
	w.at(next, 21, 0)
	got = w.board(a, "")
	if got["state"] != "locked" || strings.Join(cardIDs(t, got), ",") != aEntry+","+bEntry {
		t.Fatalf("locked with yesterday: %v", got)
	}
	if label := dig(t, got["days"].([]any)[0], "label"); label != "yesterday" {
		t.Fatalf("yesterday label: %v", label)
	}
	// Before 8 PM the day in play is last night.
	w.at(next, 9, 0)
	if got := w.board(a, ""); got["day_label"] != "last_night" || got["state"] != "unlocked" {
		t.Fatalf("morning: %v", got)
	}
	w.post(a, next)
	if got := w.board(a, "?limit=1"); got["next_before"] != next || len(got["days"].([]any)) != 1 {
		t.Fatalf("page 1: %v", got)
	}
	if got := w.board(a, "?limit=1&before="+next); got["next_before"] != nil || dig(t, got["days"].([]any)[0], "local_date") != day {
		t.Fatalf("page 2: %v", got)
	}
	for q, field := range map[string]string{"?limit=0": "limit", "?limit=11": "limit", "?before=10-08": "before"} {
		if got := w.call(a, http.MethodGet, "/v1/board"+q, nil, http.StatusBadRequest); got["field"] != field {
			t.Fatalf("%s: %v", q, got)
		}
	}

	// Blocked: hidden both ways.
	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": b.id})
	w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	for _, id := range cardIDs(t, w.board(a, "")) {
		if id == bEntry {
			t.Fatal("a blocked person's card is listed")
		}
	}
	w.call(b, http.MethodGet, "/v1/posts/"+aEntry, nil, http.StatusNotFound)
}

func TestPostIntegration_ReactionsAndComments(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("pr_alice", "Alice Ames")
	b := w.person("pr_bob", "Bob Brown")
	c := w.person("pr_cara", "Cara Cole")
	w.follow(a, b)
	w.follow(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)
	path := "/v1/posts/" + bEntry

	// Your own post: 409 own_post.
	if got := w.call(b, http.MethodPost, path+"/reaction", map[string]any{"reaction": "fire"}, http.StatusConflict); got["error"] != "own_post" {
		t.Fatalf("own post: %v", got)
	}
	if got := w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": "fire"}); got["my_reaction"] != "fire" {
		t.Fatalf("set: %v", got)
	}
	w.clk.Advance(time.Minute)
	if got := w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": "love"}); got["my_reaction"] != "love" {
		t.Fatalf("change: %v", got)
	}
	if got := w.call(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": "clap"}, http.StatusBadRequest); got["field"] != "reaction" {
		t.Fatalf("bad name: %v", got)
	}
	if got := w.call(a, http.MethodPost, path+"/reaction", map[string]any{}, http.StatusBadRequest); got["field"] != "reaction" {
		t.Fatalf("missing key: %v", got)
	}
	own := w.ok(b, http.MethodGet, path, nil)
	received := own["received"].([]any)
	if len(received) != 1 || dig(t, received[0], "reaction") != "love" || dig(t, received[0], "person", "id") != a.id {
		t.Fatalf("received: %v", own)
	}
	if got := w.ok(a, http.MethodGet, path, nil); dig(t, got, "card", "my_reaction") != "love" {
		t.Fatalf("my reaction on the card: %v", got)
	}

	// Comments: write, read, delete by the writer or the owner only.
	got := w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": "  Looks good  "}, http.StatusCreated)
	first := got["comment"].(map[string]any)
	if first["text"] != "Looks good" || first["can_delete"] != true || first["at_label"] != "8:31 PM" || dig(t, first, "author", "id") != a.id {
		t.Fatalf("comment: %v", got)
	}
	w.clk.Advance(time.Minute)
	second := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Nice"}, http.StatusCreated)["comment"].(map[string]any)
	if got := w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": strings.Repeat("a", 141)}, http.StatusBadRequest); got["field"] != "text" {
		t.Fatalf("too long: %v", got)
	}
	if got := w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": "   "}, http.StatusBadRequest); got["field"] != "text" {
		t.Fatalf("empty: %v", got)
	}
	read := w.ok(b, http.MethodGet, path, nil)
	comments := read["comments"].([]any)
	if len(comments) != 2 || dig(t, comments[0], "id") != first["id"] || dig(t, comments[1], "id") != second["id"] ||
		dig(t, read, "card", "comment_count") != float64(2) || dig(t, comments[0], "can_delete") != true {
		t.Fatalf("comments oldest first: %v", read)
	}
	if got := w.ok(a, http.MethodGet, path, nil); dig(t, got["comments"].([]any)[1], "can_delete") != false {
		t.Fatalf("a cannot delete c's comment: %v", got)
	}
	w.call(a, http.MethodPost, "/v1/comments/"+dig(t, second, "id").(string)+"/delete", map[string]any{}, http.StatusNotFound)
	w.ok(b, http.MethodPost, "/v1/comments/"+dig(t, second, "id").(string)+"/delete", map[string]any{})
	w.call(b, http.MethodPost, "/v1/comments/"+dig(t, second, "id").(string)+"/delete", map[string]any{}, http.StatusNotFound)

	// comments_off: refused with 409, existing comments stay and can be deleted.
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"comments_off": true})
	if got := w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": "More"}, http.StatusConflict); got["error"] != "comments_off" {
		t.Fatalf("comments off: %v", got)
	}
	if got := w.call(b, http.MethodPost, path+"/comments", map[string]any{"text": "Mine"}, http.StatusConflict); got["error"] != "comments_off" {
		t.Fatalf("comments off for the owner too: %v", got)
	}
	off := w.ok(a, http.MethodGet, path, nil)
	if off["can_comment"] != false || off["comments_off"] != true || len(off["comments"].([]any)) != 1 {
		t.Fatalf("post with comments off: %v", off)
	}
	w.ok(a, http.MethodPost, "/v1/comments/"+dig(t, first, "id").(string)+"/delete", map[string]any{})
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"comments_off": false})

	// Clearing a reaction.
	if got := w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": nil}); got["my_reaction"] != nil {
		t.Fatalf("clear: %v", got)
	}
	w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": nil})
	if got := w.ok(b, http.MethodGet, path, nil); len(got["received"].([]any)) != 0 {
		t.Fatalf("cleared: %v", got)
	}

	// Deleting the post takes its reactions and comments with it.
	w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": "wow"})
	w.call(a, http.MethodPost, path+"/comments", map[string]any{"text": "Bye"}, http.StatusCreated)
	w.ok(b, http.MethodPost, "/v1/entries/"+bEntry+"/delete", map[string]any{})
	w.call(a, http.MethodGet, path, nil, http.StatusNotFound)
	if k := kinds(t, w.notes(b)); has(k, "reaction") || has(k, "comment") {
		t.Fatalf("items of a deleted post: %v", k)
	}
}

func TestNotificationIntegration_SettingsFilterTheList(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("nf_alice", "Alice Ames")
	b := w.person("nf_bob", "Bob Brown")
	const day = "2026-10-08"
	w.follow(a, b)
	bEntry := w.post(b, day)
	w.post(a, day)
	w.clk.Advance(time.Minute)
	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/reaction", map[string]any{"reaction": "fire"})
	w.clk.Advance(time.Minute)
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"text": "Great pick"}, http.StatusCreated)

	if got := w.ok(b, http.MethodGet, "/v1/me/notification-settings", nil); got["reactions"] != true || got["comments"] != true || got["milestones"] != true {
		t.Fatalf("defaults: %v", got)
	}
	list := w.notes(b)
	k := kinds(t, list)
	if !has(k, "reaction") || !has(k, "comment") || !has(k, "follow") || !has(k, "spin_open") || list["unread"] != true {
		t.Fatalf("all on: %v", list)
	}
	if k[0] != "comment" || k[1] != "reaction" {
		t.Fatalf("newest first: %v", k)
	}
	r := itemOf(t, list, "reaction")
	if r["reaction"] != "fire" || r["entry_id"] != bEntry || r["category_name"] == nil || r["section"] != "today" || r["at_label"] != "8:31 PM" {
		t.Fatalf("reaction item: %v", r)
	}
	if cm := itemOf(t, list, "comment"); cm["text"] != "Great pick" || dig(t, cm, "actor", "username") != "nf_alice" {
		t.Fatalf("comment item: %v", cm)
	}
	if so := itemOf(t, list, "spin_open"); so["actor"] != nil || so["at_label"] != "8:00 PM" {
		t.Fatalf("spin open item: %v", so)
	}

	got := w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"reactions": false})
	if got["reactions"] != false || got["comments"] != true || got["milestones"] != true {
		t.Fatalf("patch: %v", got)
	}
	if k := kinds(t, w.notes(b)); has(k, "reaction") || !has(k, "comment") {
		t.Fatalf("reactions off: %v", k)
	}
	w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": false, "milestones": false})
	if k := kinds(t, w.notes(b)); has(k, "comment") || has(k, "spin_open") || has(k, "reaction") || !has(k, "follow") {
		t.Fatalf("all off, follow items stay: %v", k)
	}
	if got := w.ok(b, http.MethodGet, "/v1/me/notification-settings", nil); got["reactions"] != false || got["comments"] != false || got["milestones"] != false {
		t.Fatalf("stored: %v", got)
	}
	if got := w.call(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": "no"}, http.StatusBadRequest); got["field"] != "comments" {
		t.Fatalf("bad type: %v", got)
	}

	// The dot follows the filtered list and the seen time.
	if got := w.ok(b, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != true {
		t.Fatalf("summary with a follow item: %v", got)
	}
	if got := w.ok(b, http.MethodPost, "/v1/notifications/seen", map[string]any{}); got["unread"] != false {
		t.Fatalf("seen: %v", got)
	}
	if got := w.ok(b, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != false {
		t.Fatalf("summary after seen: %v", got)
	}
	w.clk.Advance(time.Minute)
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"text": "Again"}, http.StatusCreated)
	if got := w.ok(b, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != false {
		t.Fatalf("a filtered comment does not light the dot: %v", got)
	}
	w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": true})
	if got := w.ok(b, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != true {
		t.Fatalf("comments on again: %v", got)
	}
	items := w.notes(b)["items"].([]any)
	if dig(t, items[0], "text") != "Again" || dig(t, items[0], "unread") != true || dig(t, items[1], "unread") != false {
		t.Fatalf("unread only after seen: %v", items)
	}
}
