package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// The bell tells the owner what the admin decided about their post or
// comment (spec section 6): "We removed your post." with the address to ask
// for a second look and the removal's ref, and "Your post is back." after
// Bring it back or a Dismiss that showed it again. These are system items:
// no actor, no switch turns them off, and they light the dot. A removal
// never gives a way back to what was removed.

// bellContact is the address the test server gives as CONTACT_EMAIL.
const bellContact = "second.look@example.com"

// decisionKinds are the bell's kinds for the admin's decisions.
var decisionKinds = []string{"post_removed", "comment_removed", "post_back", "comment_back"}

// decisionIDs lists the ids of the decision items in p's bell, in order.
func (w *adminModWorld) decisionIDs(p feedPerson) []string {
	w.t.Helper()
	out := []string{}
	for _, v := range w.notes(p)["items"].([]any) {
		if m := v.(map[string]any); slices.Contains(decisionKinds, m["kind"].(string)) {
			out = append(out, m["id"].(string))
		}
	}
	return out
}

// bellItem is the item with id in p's bell, and the whole answer's text.
func (w *adminModWorld) bellItem(p feedPerson, id string) (map[string]any, string) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodGet, "/v1/notifications", p.token, nil)
	if rec.Code != http.StatusOK {
		w.t.Fatalf("the bell of %s: %d %s", p.name, rec.Code, rec.Body.String())
	}
	list := w.notes(p)
	for _, v := range list["items"].([]any) {
		if m := v.(map[string]any); m["id"] == id {
			return m, rec.Body.String()
		}
	}
	w.t.Fatalf("no item %s in the bell of %s: %v", id, p.name, list["items"])
	return nil, ""
}

// wantItem fails the test unless item has each of the fields in want, and
// has none of the keys in absent.
func wantItem(t *testing.T, what string, item, want map[string]any, absent ...string) {
	t.Helper()
	for k, v := range want {
		if got, ok := item[k]; !ok || got != v {
			t.Fatalf("%s: %s is %v, want %v (item %v)", what, k, got, v, item)
		}
	}
	for _, k := range absent {
		if _, ok := item[k]; ok {
			t.Fatalf("%s: carries %s, want none (item %v)", what, k, item)
		}
	}
}

// instant is t as an answer writes it.
func instant(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// dot is p's bell dot.
func (w *adminModWorld) dot(p feedPerson) bool {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/notifications/summary", nil)["unread"] == true
}

// A removal reaches the post's owner with both switches off: no actor, the
// title, no post and no photo, the address and the ref the Removed tab
// shows, unread, and the dot lit. While the post is still kept for Bring it
// back nothing in the bell names it or its photos. Nobody else hears of it.
// Bring it back is a new item that opens the post and shows its first
// photo. Removed again, that item keeps its post but loses the photo, and
// the new removal has a ref of its own.
func TestModerationBellIntegration_TheOwnerHearsOfARemovalWithNoWayBack(t *testing.T) {
	w := newAdminModWorldWith(t, bellContact)
	ps := w.people("mb_bob", "mb_rhea", "mb_vera")
	b, r, v := ps[0], ps[1], ps[2]
	w.followAccepted(r, b)
	w.followAccepted(v, b)
	const day = "2026-10-08"
	post := w.post(b, day)
	w.post(r, day)
	w.post(v, day)
	photos := postPhotoKeys(t, w.pool, post)
	w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": false, "milestones": false})
	w.ok(b, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	if w.dot(b) {
		t.Fatal("the dot is lit before anything was decided")
	}

	w.clk.Advance(time.Minute)
	w.ok(r, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	if got := w.act("remove", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove: %q", got)
	}
	removal := w.lastLogged(b)
	item, body := w.bellItem(b, "post_removed:"+removal.ID)
	wantItem(t, "the removal", item, map[string]any{"kind": "post_removed", "actor": nil, "text": "Dune", "entry_id": nil,
		"photo_url": "", "category_name": nil, "request_id": nil, "contact": bellContact, "ref": removal.ID[:8],
		"unread": true, "at": instant(removal.At)})
	for _, s := range append([]string{post}, photos...) {
		if strings.Contains(body, s) {
			t.Fatalf("the bell names %s of the removed post: %s", s, body)
		}
	}
	if !w.dot(b) {
		t.Fatal("the dot is off after a removal")
	}
	if tab := w.page("/admin/removed"); !strings.Contains(tab, " · Ref: "+removal.ID[:8]+"</p>") {
		t.Fatalf("the Removed tab does not show the bell's ref %s: %s", removal.ID[:8], tab)
	}
	for _, p := range []feedPerson{r, v} {
		if got := w.decisionIDs(p); len(got) != 0 {
			t.Fatalf("%s hears of B's removal: %v", p.name, got)
		}
	}
	w.ok(b, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	if w.dot(b) {
		t.Fatal("the dot is lit after the removal was seen")
	}

	w.clk.Advance(time.Minute)
	if got := w.restore("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("bring it back: %q", got)
	}
	back := w.lastLogged(b)
	item, _ = w.bellItem(b, "post_back:"+back.ID)
	wantItem(t, "back", item, map[string]any{"kind": "post_back", "actor": nil, "text": "Dune", "entry_id": post,
		"photo_url": fakePhotoLink(photos[0]), "unread": true, "at": instant(back.At)}, "contact", "ref")
	if item, _ = w.bellItem(b, "post_removed:"+removal.ID); item["unread"] != false || !w.dot(b) {
		t.Fatalf("after Bring it back: the removal %v, the dot %v; want the removal read and the dot lit", item, w.dot(b))
	}
	w.ok(b, http.MethodPost, "/v1/notifications/seen", map[string]any{})

	w.clk.Advance(time.Minute)
	w.ok(v, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	if got := w.act("remove", "post", b, "Hate speech"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove again: %q", got)
	}
	again := w.lastLogged(b)
	if got, want := w.decisionIDs(b), []string{"post_removed:" + again.ID, "post_back:" + back.ID, "post_removed:" + removal.ID}; !slices.Equal(got, want) {
		t.Fatalf("the decisions in the bell: %v, want %v", got, want)
	}
	item, body = w.bellItem(b, "post_removed:"+again.ID)
	if item["ref"] != again.ID[:8] || item["ref"] == removal.ID[:8] || item["entry_id"] != nil || item["photo_url"] != "" ||
		item["unread"] != true || !w.dot(b) {
		t.Fatalf("the second removal: %v, want its own ref %s, no post or photo, and unread", item, again.ID[:8])
	}
	item, _ = w.bellItem(b, "post_back:"+back.ID)
	wantItem(t, "back, once removed again", item, map[string]any{"entry_id": post, "photo_url": "", "unread": false}, "contact", "ref")
	for _, k := range photos {
		if strings.Contains(body, k) {
			t.Fatalf("the bell carries the removed post's photo %s: %s", k, body)
		}
	}
}

// A removed comment reaches its writer, not the post's owner or the
// reporter: its text, no post and no photo, the address and the ref. Back,
// it opens the post (someone else's) only while the writer can see it: once
// its owner archives it, the item opens nothing. It never shows a photo.
func TestModerationBellIntegration_TheWriterHearsOfTheirComment(t *testing.T) {
	w := newAdminModWorldWith(t, bellContact)
	ps := w.people("mc_vera", "mc_wren", "mc_rhea")
	v, cw, r := ps[0], ps[1], ps[2]
	w.followAccepted(cw, v)
	w.followAccepted(r, v)
	const day = "2026-10-08"
	post := w.post(v, day)
	w.post(cw, day)
	w.post(r, day)
	comment := w.reply(cw, post, "Rude words", "")["id"].(string)
	w.ok(cw, http.MethodPost, "/v1/notifications/seen", map[string]any{})

	w.clk.Advance(time.Minute)
	w.ok(r, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "harassment"})
	if got := w.act("remove", "comment", cw, "Bullying or harassment"); got != "/admin/people/"+cw.id+"?n=removed" {
		t.Fatalf("remove the comment: %q", got)
	}
	removal := w.lastLogged(cw)
	item, body := w.bellItem(cw, "comment_removed:"+removal.ID)
	wantItem(t, "the removal", item, map[string]any{"kind": "comment_removed", "actor": nil, "text": "Rude words",
		"entry_id": nil, "photo_url": "", "contact": bellContact, "ref": removal.ID[:8], "unread": true, "at": instant(removal.At)})
	if strings.Contains(body, post) || strings.Contains(body, comment) {
		t.Fatalf("the writer's bell names the removed comment or its post: %s", body)
	}
	if !w.dot(cw) {
		t.Fatal("the writer's dot is off after the removal")
	}
	for _, p := range []feedPerson{v, r} {
		if got := w.decisionIDs(p); len(got) != 0 {
			t.Fatalf("%s hears of the comment's removal: %v", p.name, got)
		}
	}

	w.clk.Advance(time.Minute)
	if got := w.restore("comment", comment, cw); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the comment back: %q", got)
	}
	back := w.lastLogged(cw)
	item, _ = w.bellItem(cw, "comment_back:"+back.ID)
	wantItem(t, "back", item, map[string]any{"kind": "comment_back", "actor": nil, "text": "Rude words", "entry_id": post,
		"photo_url": "", "unread": true, "at": instant(back.At)}, "contact", "ref")

	w.ok(v, http.MethodPost, "/v1/entries/"+post+"/archive", map[string]any{})
	item, _ = w.bellItem(cw, "comment_back:"+back.ID)
	wantItem(t, "back, on a post the writer can no longer see", item, map[string]any{"entry_id": nil, "photo_url": ""}, "contact", "ref")
}

// A Dismiss that shows a hidden post or comment again tells its owner it is
// back, with the post and its photo (for a comment, the post only). Being
// hidden tells nobody, and a Dismiss of something never hidden tells nobody.
func TestModerationBellIntegration_ADismissThatShowsItAgainSaysItIsBack(t *testing.T) {
	w := newAdminModWorldWith(t, bellContact)
	ps := w.people("md_bob", "md_wren", "md_vera", "md_r1", "md_r2", "md_r3")
	b, cw, v, r1, r2, r3 := ps[0], ps[1], ps[2], ps[3], ps[4], ps[5]
	for _, p := range []feedPerson{cw, v, r1, r2, r3} {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	for _, p := range []feedPerson{cw, v, r1, r2, r3} {
		w.post(p, day)
	}
	photos := postPhotoKeys(t, w.pool, post)
	comment := w.reply(cw, post, "Rude words", "")["id"].(string)
	quiet := w.reply(v, post, "Fine words", "")["id"].(string)
	for _, p := range []feedPerson{b, cw, v} {
		w.ok(p, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	}

	// A post a reporter reported is gone for them, so each reports the
	// comments first.
	w.clk.Advance(time.Minute)
	w.ok(r1, http.MethodPost, "/v1/comments/"+quiet+"/report", map[string]any{"reason": "spam"})
	for _, p := range []feedPerson{r1, r2, r3} {
		w.ok(p, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "spam"})
		w.ok(p, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	}
	for _, p := range []feedPerson{b, cw} {
		if got := w.decisionIDs(p); len(got) != 0 || w.dot(p) {
			t.Fatalf("hidden, %s hears of it: %v", p.name, got)
		}
	}

	if got := w.act("dismiss", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=shownagain" {
		t.Fatalf("dismiss the post: %q", got)
	}
	shown := w.lastLogged(b)
	item, _ := w.bellItem(b, "post_back:"+shown.ID)
	wantItem(t, "the post back", item, map[string]any{"kind": "post_back", "actor": nil, "text": "Dune", "entry_id": post,
		"photo_url": fakePhotoLink(photos[0]), "unread": true}, "contact", "ref")

	if got := w.act("dismiss", "comment", cw, "Spam"); got != "/admin/people/"+cw.id+"?n=shownagain" {
		t.Fatalf("dismiss the comment: %q", got)
	}
	shown = w.lastLogged(cw)
	item, _ = w.bellItem(cw, "comment_back:"+shown.ID)
	wantItem(t, "the comment back", item, map[string]any{"kind": "comment_back", "actor": nil, "text": "Rude words",
		"entry_id": post, "photo_url": "", "unread": true}, "contact", "ref")

	if got := w.act("dismiss", "comment", v, "Spam"); got != "/admin/people/"+v.id+"?n=dismissed" {
		t.Fatalf("dismiss the comment never hidden: %q", got)
	}
	if got := w.decisionIDs(v); len(got) != 0 || w.dot(v) {
		t.Fatalf("a Dismiss of a comment never hidden tells its writer: %v", got)
	}
}

// With no CONTACT_EMAIL the removal still shows, with its ref and no
// address at all. A Disable and an Enable of the account are logged and make
// no item.
func TestModerationBellIntegration_NoAddressAndNoAccountItems(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("mn_bob", "mn_rhea")
	b, r := ps[0], ps[1]
	w.followAccepted(r, b)
	const day = "2026-10-08"
	post := w.post(b, day)
	w.post(r, day)
	w.ok(r, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	if got := w.act("remove", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove: %q", got)
	}
	removal := w.lastLogged(b)
	for _, action := range []string{"disable", "enable"} {
		w.clk.Advance(time.Minute)
		page := w.page("/admin/people/" + b.id)
		rec := adminDo(t, w.feedWorld, http.MethodPost, "/admin/people/"+b.id+"/"+action, w.cookie, url.Values{"csrf": {csrfOf(t, page)}}, "")
		if got := rec.Header().Get("Location"); got != "/admin/people/"+b.id+"?n="+action+"d" {
			t.Fatalf("%s: %q", action, got)
		}
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 3 {
		t.Fatalf("the log: %v, want the removal, the disable and the enable", got)
	}
	if got := w.decisionIDs(b); !slices.Equal(got, []string{"post_removed:" + removal.ID}) {
		t.Fatalf("the decisions in the bell: %v, want only the removal", got)
	}
	item, _ := w.bellItem(b, "post_removed:"+removal.ID)
	wantItem(t, "the removal with no address", item, map[string]any{"ref": removal.ID[:8], "entry_id": nil, "photo_url": ""}, "contact")

	// The read itself leaves the account's rows out.
	rows, err := postgres.NewNotificationsRepo(w.pool).Decisions(context.Background(), uuid.MustParse(b.id), time.Time{}, 50)
	if err != nil || len(rows) != 1 || rows[0].Item != "post" || rows[0].Action != "removed" {
		t.Fatalf("the read: %+v (err %v), want only the post's removal", rows, err)
	}

	// Like every item, a decision older than the bell's 30 days is not
	// listed.
	w.exec(`update moderation_events set at = at - interval '31 days' where id = $1`, removal.ID)
	if got := w.decisionIDs(b); len(got) != 0 {
		t.Fatalf("a removal 31 days old is still listed: %v", got)
	}
}
