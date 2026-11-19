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

// The admin page's answers to a second look (developer, 2026-11-19), on the
// database: the Removed tab lists a line with an open request first, with
// the owner's words, their photos and Keep it removed; Keep it removed
// closes the request and logs kept for the owner's bell; Bring it back
// closes it as restored, also after a Keep; and the person page's History
// shows each request between its removal and its answer.

// removeNow removes p's post or comment straight through the store, as
// Remove does, on p's own clock, and returns the removal's id.
func (w *adminModWorld) removeNow(kind, id string, p feedPerson) string {
	w.t.Helper()
	if err := postgres.NewAdminRepo(w.pool).RemoveItem(context.Background(), kind, uuid.MustParse(id), uuid.MustParse(p.id), "spam",
		time.Now().UTC(), w.clk.NowFor(p.id)); err != nil {
		w.t.Fatalf("remove the %s: %v", kind, err)
	}
	return w.lastLogged(p).ID
}

// askLook sends p's second look about removal from the app, with words and
// the photo keys, and wants it made.
func (w *adminModWorld) askLook(p feedPerson, removal, words string, keys ...string) {
	w.t.Helper()
	if keys == nil {
		keys = []string{}
	}
	if got := w.call(p, http.MethodPost, "/v1/removals/"+removal+"/second-look", map[string]any{"text": words, "photo_keys": keys},
		http.StatusCreated); got["second_look"] != "asked" {
		w.t.Fatalf("ask about %s: %v", removal, got)
	}
}

// keepFormOf is the Keep it removed form the Removed tab draws for target.
func keepFormOf(csrf, kind, target string, owner feedPerson) string {
	return `<form method="post" action="/admin/removed/keep"><input type="hidden" name="csrf" value="` + csrf +
		`"><input type="hidden" name="kind" value="` + kind + `"><input type="hidden" name="id" value="` + target +
		`"><input type="hidden" name="person" value="` + owner.id +
		`"><input type="hidden" name="from" value="removed"><button>Keep it removed</button></form>`
}

// keep sends the Keep it removed form that the Removed tab draws for the
// post or comment target, and wants the tab to draw exactly that form.
func (w *adminModWorld) keep(kind, target string, owner feedPerson) string {
	w.t.Helper()
	tab := w.page("/admin/removed")
	if !strings.Contains(tab, keepFormOf(csrfOf(w.t, tab), kind, target, owner)) {
		w.t.Fatalf("the Removed tab has no Keep it removed for %s %s: %s", kind, target, tab)
	}
	return w.keepAnyway(kind, target, owner)
}

// keepAnyway sends Keep it removed for target, whether the tab offers it or
// not.
func (w *adminModWorld) keepAnyway(kind, target string, owner feedPerson) string {
	w.t.Helper()
	return adminDo(w.t, w.feedWorld, http.MethodPost, "/admin/removed/keep", w.cookie,
		url.Values{"csrf": {csrfOf(w.t, w.page("/admin/removed"))}, "kind": {kind}, "id": {target}, "person": {owner.id}, "from": {"removed"}},
		"").Header().Get("Location")
}

// lookState is the answer stored on the second look about removal: "open"
// while there is none, "none" with no request, and a note when its time
// does not match it.
func (w *adminModWorld) lookState(removal string) string {
	w.t.Helper()
	return w.one(`select coalesce((select coalesce(decision, 'open') ||
		case when (decision is null) = (decided_at is null) then '' else ' (time wrong)' end
		from second_looks where removal_id = $1), 'none')`, removal)
}

// lineOrder is where each mark first shows on page, -1 for none.
func lineOrder(page string, marks ...string) []int {
	out := make([]int, len(marks))
	for i, m := range marks {
		out[i] = strings.Index(page, m)
	}
	return out
}

// proofLink is a second look's photo as the page draws it: signed, opening
// full size.
func proofLink(key string) string {
	return `<a href="` + fakePhotoLink(key) + `" target="_blank" rel="noopener"><img class="thumb" src="` + fakePhotoLink(key) +
		`" alt="Second look photo" loading="lazy" decoding="async"></a>`
}

// An open second look puts its line first on the Removed tab, ahead of one
// removed later, with the Second look asked tag, the owner's words, escaped,
// and their photos, signed, in its own What cell, and Keep it removed
// beside Bring it back; the tab's count stays the same. Keep it removed
// keeps it removed, closes the request as kept, logs kept on the owner's
// clock with the title, and the bell says so; the line stays, with no tag
// and no Keep, back in its place. A double click changes nothing more.
// Bring it back on a line with an open request closes it as restored, and
// after a Keep turns kept into restored. A Keep from a page drawn before
// that says the second look was answered, and logs nothing.
func TestAdminSecondLookIntegration_AnOpenRequestComesFirstAndKeepAnswersIt(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("sk_bob", "sk_dana", "sk_wren")
	b, d, cw := ps[0], ps[1], ps[2]
	const day = "2026-10-08"
	post := w.post(b, day)
	dPost := w.post(d, day)
	comment := w.reply(cw, w.post(cw, day), "Rude words", "")["id"].(string)
	bLine, dLine, cwLine := "<strong>@sk_bob</strong>", "<strong>@sk_dana</strong>", "<strong>@sk_wren</strong>"

	// Bob's post goes first, then Dana's, then Wren's comment.
	postRemoval := w.removeNow("post", post, b)
	w.removeNow("post", dPost, d)
	commentRemoval := w.removeNow("comment", comment, cw)
	if got := lineOrder(w.page("/admin/removed"), cwLine, dLine, bLine); !slices.IsSorted(got) || got[0] < 0 {
		t.Fatalf("with no second look, want the most recently removed first: %v", got)
	}

	k1, k2 := lookKey(b, "00000000000000b1"), lookKey(b, "00000000000000b2")
	w.askLook(b, postRemoval, "<b>Mine</b>, not spam.\nAsk anyone.", k1, k2)
	w.askLook(cw, commentRemoval, "A joke")
	tab := w.page("/admin/removed")
	if got := lineOrder(tab, cwLine, bLine, dLine); !slices.IsSorted(got) || got[0] < 0 {
		t.Fatalf("want both lines with a second look first, the most recently removed first, then Dana's: %v: %s", got, tab)
	}
	if !strings.Contains(tab, `class="on" aria-current="page">Removed<span class="n">3</span>`) {
		t.Fatalf("the tab's count, want 3 as before: %s", tab)
	}
	bobs := adminReportOf(t, tab, bLine)
	what := bobs[strings.Index(bobs, `<div class="body">`):strings.Index(bobs, `<div class="owner">`)]
	if !strings.Contains(what, `<div class="look"><p class="flag"><span class="tag">Second look asked</span></p>`+
		`<p class="text quote">&lt;b&gt;Mine&lt;/b&gt;, not spam.`+"\n"+`Ask anyone.</p><div class="strip">`+proofLink(k1)+proofLink(k2)+`</div></div></div>`) {
		t.Fatalf("Bob's What cell, want the tag, his words and his photos under the post: %s", what)
	}
	if !strings.Contains(bobs, keepFormOf(csrfOf(t, tab), "post", post, b)) ||
		!strings.Contains(adminReportOf(t, tab, cwLine), `<p class="text quote">A joke</p></div></div>`) {
		t.Fatalf("want Keep it removed on Bob's line and Wren's words on hers: %s", tab)
	}
	if danas := adminReportOf(t, tab, dLine); strings.Contains(danas, "Second look") || strings.Contains(danas, "/admin/removed/keep") {
		t.Fatalf("Dana asked for nothing, yet her line: %s", danas)
	}

	// Keep it removed.
	if got := w.keep("post", post, b); got != "/admin/removed?n=kept" {
		t.Fatalf("keep it removed: %q", got)
	}
	tab = w.page("/admin/removed?n=kept")
	if !strings.Contains(tab, `<div class="note">Kept removed. They&#39;ll see it in the app.</div>`) {
		t.Fatalf("the notice: %s", tab)
	}
	if got := w.lookState(postRemoval); got != "kept" {
		t.Fatalf("the second look after Keep: %s", got)
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post, "kept post " + post}) {
		t.Fatalf("the log: %v", got)
	}
	kept := w.lastLogged(b)
	if kept.Entry != post || kept.Comment != "" || kept.Snapshot != "Dune" || !kept.At.Equal(w.clk.NowFor(b.id)) {
		t.Fatalf("Keep logged: %+v, want the post and its title on Bob's clock", kept)
	}
	item, _ := w.bellItem(b, "post_kept:"+kept.ID)
	wantItem(t, "Keep it removed in the bell", item, map[string]any{"kind": "post_kept", "text": "Dune", "entry_id": nil},
		"ref", "removal", "second_look")
	if item, _ := w.bellItem(b, "post_removed:"+postRemoval); item["second_look"] != "kept" {
		t.Fatalf("the removal in the bell once kept: %v", item)
	}
	if w.sees(b, post) {
		t.Fatal("Bob sees the post he was told stays removed")
	}
	bobs = adminReportOf(t, tab, bLine)
	if strings.Contains(bobs, "Second look") || strings.Contains(bobs, "/admin/removed/keep") || strings.Contains(bobs, "Mine") ||
		!strings.Contains(bobs, `action="/admin/removed/restore"`) {
		t.Fatalf("Bob's line once kept, want it plain, Bring it back still there: %s", bobs)
	}
	if got := lineOrder(tab, cwLine, dLine, bLine); !slices.IsSorted(got) || got[0] < 0 {
		t.Fatalf("once kept, want Bob's line back in its place, after Dana's: %v", got)
	}

	// A double click: what the first said, and nothing more.
	if got := w.keepAnyway("post", post, b); got != "/admin/removed?n=kept" {
		t.Fatalf("Keep again: %q", got)
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 2 || w.lookState(postRemoval) != "kept" {
		t.Fatalf("Keep again changed something: %v, %s", got, w.lookState(postRemoval))
	}

	// Bring it back on Wren's open request answers it as restored.
	if got := w.restore("comment", comment, cw); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the comment back: %q", got)
	}
	if got := w.lookState(commentRemoval); got != "restored" {
		t.Fatalf("Wren's second look after Bring it back: %s", got)
	}
	if item, _ := w.bellItem(cw, "comment_removed:"+commentRemoval); item["second_look"] != "restored" {
		t.Fatalf("Wren's removal in the bell once back: %v", item)
	}

	// Bring it back after Keep: kept becomes restored.
	if got := w.restore("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the post back after Keep: %q", got)
	}
	if got := w.lookState(postRemoval); got != "restored" || !w.sees(b, post) {
		t.Fatalf("Bob's second look after Bring it back: %s (he sees the post: %v)", got, w.sees(b, post))
	}
	if item, _ := w.bellItem(b, "post_removed:"+postRemoval); item["second_look"] != "restored" {
		t.Fatalf("Bob's removal in the bell once back: %v", item)
	}

	// Keep from a page drawn before Bring it back: answered, nothing logged.
	if got := w.keepAnyway("post", post, b); got != "/admin/removed?n=answered" {
		t.Fatalf("Keep once it is back: %q", got)
	}
	if tab := w.page("/admin/removed?n=answered"); !strings.Contains(tab, `<div class="note">That second look was already answered.</div>`) {
		t.Fatalf("the notice: %s", tab)
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post, "kept post " + post, "restored post " + post}) ||
		w.lookState(postRemoval) != "restored" {
		t.Fatalf("Keep once it is back changed something: %v, %s", got, w.lookState(postRemoval))
	}
}

// Keep it removed changes nothing and logs nothing without an open second
// look: none asked (the tab draws no Keep then), a form naming someone else
// as the owner, a stale Keep after the item was brought back and removed
// again (the new removal has no request), or a removal past the 30 days,
// which leaves the tab while its request stays open for the purge. A
// comment whose post was removed too can be kept removed, though Bring it
// back asks for its post first and leaves its request open.
func TestAdminSecondLookIntegration_KeepChangesNothingWithoutAnOpenRequest(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("sn_bob", "sn_dana", "sn_wren")
	b, d, cw := ps[0], ps[1], ps[2]
	w.followAccepted(cw, d)
	const day = "2026-10-08"
	post := w.post(b, day)
	first := w.removeNow("post", post, b)

	if tab := w.page("/admin/removed"); strings.Contains(tab, "/admin/removed/keep") || strings.Contains(tab, "Second look") {
		t.Fatalf("nothing asked, yet the tab offers Keep: %s", tab)
	}
	if got := w.keepAnyway("post", post, b); got != "/admin/removed?n=answered" {
		t.Fatalf("Keep with nothing asked: %q", got)
	}
	w.askLook(b, first, "Please look again")
	if got := w.keepAnyway("post", post, d); got != "/admin/removed?n=gone" || w.lookState(first) != "open" {
		t.Fatalf("Keep naming Dana as the owner: %q, %s", got, w.lookState(first))
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post}) {
		t.Fatalf("a refused Keep logged: %v", got)
	}

	// Brought back, then removed again: the new removal has no request, so
	// a Keep from a page drawn before is answered.
	if got := w.restore("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("bring it back: %q", got)
	}
	again := w.removeNow("post", post, b)
	if got := w.keepAnyway("post", post, b); got != "/admin/removed?n=answered" || w.lookState(first) != "restored" ||
		w.lookState(again) != "none" {
		t.Fatalf("Keep on the new removal: %q, the first request %s, the new one %s", got, w.lookState(first), w.lookState(again))
	}

	// Past the 30 days: off the tab, Keep is gone, the request stays open.
	w.askLook(b, again, "And again")
	if tab := w.page("/admin/removed"); !strings.Contains(tab, keepFormOf(csrfOf(t, tab), "post", post, b)) {
		t.Fatalf("the new request's line, want Keep it removed: %s", tab)
	}
	w.exec(`update entries set removed_at = now() - interval '30 days' where id = $1`, post)
	if tab := w.page("/admin/removed"); !strings.Contains(tab, "Nothing removed in the last 30 days.") {
		t.Fatalf("past the 30 days the tab still lists it: %s", tab)
	}
	if got := w.keepAnyway("post", post, b); got != "/admin/removed?n=gone" || w.lookState(again) != "open" {
		t.Fatalf("Keep past the 30 days: %q, %s", got, w.lookState(again))
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post, "restored post " + post, "removed post " + post}) {
		t.Fatalf("a refused Keep logged: %v", got)
	}

	// Wren's comment, then Dana's post under it: Wren still asks, and the
	// admin can keep it removed, though not bring it back first.
	dPost := w.post(d, day)
	w.post(cw, day)
	comment := w.reply(cw, dPost, "Rude words", "")["id"].(string)
	commentRemoval := w.removeNow("comment", comment, cw)
	w.removeNow("post", dPost, d)
	w.askLook(cw, commentRemoval, "A joke")
	if got := w.restore("comment", comment, cw); got != "/admin/removed?n=postfirst" || w.lookState(commentRemoval) != "open" {
		t.Fatalf("bring the comment back under its removed post: %q, %s", got, w.lookState(commentRemoval))
	}
	if got := w.keep("comment", comment, cw); got != "/admin/removed?n=kept" || w.lookState(commentRemoval) != "kept" {
		t.Fatalf("keep the comment under its removed post: %q, %s", got, w.lookState(commentRemoval))
	}
	if got := loggedActions(t, w.pool, cw.id); !slices.Equal(got, []string{"removed comment " + comment, "kept comment " + comment}) {
		t.Fatalf("Wren's log: %v", got)
	}
	if got := w.lastLogged(cw); got.Entry != dPost || got.Comment != comment || got.Snapshot != "Rude words" {
		t.Fatalf("the comment kept removed, logged: %+v", got)
	}
}

// The person page's History shows each second look, its words escaped and
// its photos while the purge has not deleted them, with the ref of its
// removal, between that removal and Keep it removed, newest first, though
// the owner's clock stood still through all of them; the report that led to
// it comes after them. Once the photos are deleted the words stay.
func TestAdminSecondLookIntegration_TheHistoryShowsEachRequestAndItsAnswer(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("sh_bob", "sh_dana", "sh_rhea")
	b, d, r1 := ps[0], ps[1], ps[2]
	w.followAccepted(r1, b)
	w.followAccepted(b, d)
	const day = "2026-10-08"
	post := w.post(b, day)
	dPost := w.post(d, day)
	w.post(r1, day)
	w.ok(r1, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	comment := w.reply(b, dPost, "Loud words", "")["id"].(string)

	if got := w.act("remove", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove the post: %q", got)
	}
	postRemoval := w.lastLogged(b).ID
	k1, k2 := lookKey(b, "00000000000000c1"), lookKey(b, "00000000000000c2")
	w.askLook(b, postRemoval, "It was a <i>joke</i>", k1, k2)
	commentRemoval := w.removeNow("comment", comment, b)
	w.askLook(b, commentRemoval, "Fair words")
	if got := w.keep("post", post, b); got != "/admin/removed?n=kept" {
		t.Fatalf("keep the post removed: %q", got)
	}

	log := historyOf(t, w.page("/admin/people/"+b.id))
	var labels []string
	for _, part := range strings.Split(log, "<strong>")[1:] {
		labels = append(labels, part[:strings.Index(part, "</strong>")])
	}
	if want := []string{"Post kept removed", "Asked for a second look at a comment", "Comment removed", "Asked for a second look at a post",
		"Post removed", "Post reported for spam"}; !slices.Equal(labels, want) {
		t.Fatalf("the History, newest first: %v, want %v: %s", labels, want, log)
	}
	asked := `<p class="by">Ref: ` + postRemoval[:8] + `</p><p class="text quote">It was a &lt;i&gt;joke&lt;/i&gt;</p><div class="strip">` +
		proofLink(k1) + proofLink(k2) + `</div></li>`
	if !strings.Contains(log, asked) || !strings.Contains(log, `<p class="by">Ref: `+commentRemoval[:8]+`</p><p class="text quote">Fair words</p></li>`) ||
		!strings.Contains(log, `<strong>Post kept removed</strong> <span class="muted when">`+adminWhen(w.clk.NowFor(b.id))+`</span></p><p class="text quote">Dune</p></li>`) {
		t.Fatalf("the History's second looks and Keep: %s", log)
	}

	// The purge deleted the photos (it empties the keys): the words stay.
	w.exec(`update second_looks set photo_keys = '{}' where removal_id = $1`, postRemoval)
	log = historyOf(t, w.page("/admin/people/"+b.id))
	if strings.Contains(log, "Second look photo") || !strings.Contains(log, `<p class="text quote">It was a &lt;i&gt;joke&lt;/i&gt;</p></li>`) {
		t.Fatalf("the History once the photos are deleted, want the words alone: %s", log)
	}
}
