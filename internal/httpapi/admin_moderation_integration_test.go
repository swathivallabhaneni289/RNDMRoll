package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// The admin page's moderation on the database (migration 0020). Remove takes
// a post or comment down for everyone, its owner too, and deletes nothing;
// Bring it back undoes it inside the 30 days, on the database's clock;
// Dismiss shows a hidden post or comment again and logs it once; every
// decision, Disable and Enable included, goes in the log on the owner's own
// clock, and the person page's History, its Removed before figure and the
// Removed tab read it; and the report that hides something is the one the
// admin email hears as urgent.

// adminModWorld is an admin world over a pool the test also reads, with the
// photo store and the report notifier the server uses, and the admin's
// session.
type adminModWorld struct {
	*feedWorld
	pool   *pgxpool.Pool
	photos *fakeEntryPhotoStore
	heard  *heardReports
	cookie string
}

func newAdminModWorld(t *testing.T) *adminModWorld {
	t.Helper()
	return newAdminModWorldWith(t, "")
}

// newAdminModWorldWith is newAdminModWorld whose bell gives contact as the
// address to ask for a second look ("" for none, as with CONTACT_EMAIL
// unset).
func newAdminModWorldWith(t *testing.T, contact string) *adminModWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &adminModWorld{feedWorld: &feedWorld{t: t, clk: clk}, pool: pool, photos: &fakeEntryPhotoStore{}, heard: &heardReports{}}
	adminFirst(w.feedWorld, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"}, entryPhotos: w.photos,
		contactEmail: contact,
		wire: func(s *social.Service, b *board.Service, _ *Deps) {
			s.SetReportNotifier(w.heard)
			b.SetReportNotifier(w.heard)
		}}, "ad_boss")
	if w.cookie = adminLogin(t, w.feedWorld, "ad_boss@example.com", adminTestPassword); w.cookie == "" {
		t.Fatal("the admin could not log in")
	}
	return w
}

// page is the admin page at path.
func (w *adminModWorld) page(path string) string {
	w.t.Helper()
	rec := adminDo(w.t, w.feedWorld, http.MethodGet, path, w.cookie, nil, "")
	if rec.Code != http.StatusOK {
		w.t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// cardForm is the Remove or Dismiss form ("remove" or "dismiss") of the one
// card on owner's page that holds mark, as the page draws it: Remove with
// the report id it carries, Dismiss with every id on the card.
func (w *adminModWorld) cardForm(action, kind string, owner feedPerson, mark string) url.Values {
	w.t.Helper()
	page := w.page("/admin/people/" + owner.id)
	card := adminReportOf(w.t, page, mark)
	form := url.Values{"csrf": {csrfOf(w.t, page)}, "kind": {kind}, "person": {owner.id}}
	if action == "dismiss" {
		for _, m := range dismissIDs.FindAllStringSubmatch(card[strings.Index(card, `action="/admin/reports/dismiss"`):], -1) {
			form.Add("id", m[1])
		}
	} else {
		form.Set("id", adminReportID(w.t, card))
	}
	return form
}

// send posts form to /admin/reports/<action> and returns where it went.
func (w *adminModWorld) send(action string, form url.Values) string {
	w.t.Helper()
	return adminDo(w.t, w.feedWorld, http.MethodPost, "/admin/reports/"+action, w.cookie, form, "").Header().Get("Location")
}

// act sends a card's form as the page draws it.
func (w *adminModWorld) act(action, kind string, owner feedPerson, mark string) string {
	w.t.Helper()
	return w.send(action, w.cardForm(action, kind, owner, mark))
}

// restore sends the Bring it back form that the Removed tab draws for the
// post or comment target, and wants the tab to draw exactly that form.
func (w *adminModWorld) restore(kind, target string, owner feedPerson) string {
	w.t.Helper()
	tab := w.page("/admin/removed")
	form := `<form method="post" action="/admin/removed/restore"><input type="hidden" name="csrf" value="` + csrfOf(w.t, tab) +
		`"><input type="hidden" name="kind" value="` + kind + `"><input type="hidden" name="id" value="` + target +
		`"><input type="hidden" name="person" value="` + owner.id +
		`"><input type="hidden" name="from" value="removed"><button class="strong">Bring it back</button></form>`
	if !strings.Contains(tab, form) {
		w.t.Fatalf("the Removed tab has no Bring it back for %s %s: %s", kind, target, tab)
	}
	return w.restoreAnyway(kind, target, owner)
}

// restoreAnyway sends Bring it back for target, whether the tab offers it
// or not.
func (w *adminModWorld) restoreAnyway(kind, target string, owner feedPerson) string {
	w.t.Helper()
	return adminDo(w.t, w.feedWorld, http.MethodPost, "/admin/removed/restore", w.cookie,
		url.Values{"csrf": {csrfOf(w.t, w.page("/admin/removed"))}, "kind": {kind}, "id": {target}, "person": {owner.id}, "from": {"removed"}},
		"").Header().Get("Location")
}

// sees reports whether p can open the post page of entry.
func (w *adminModWorld) sees(p feedPerson, entry string) bool {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodGet, "/v1/posts/"+entry, p.token, nil)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		w.t.Fatalf("the post as %s: %d %s", p.name, rec.Code, rec.Body.String())
	}
	return rec.Code == http.StatusOK
}

// one reads one line of text straight from the database.
func (w *adminModWorld) one(q string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), q, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return s
}

// exec runs one statement straight on the database and wants it to change
// exactly one row.
func (w *adminModWorld) exec(q string, args ...any) {
	w.t.Helper()
	tag, err := w.pool.Exec(context.Background(), q, args...)
	if err != nil || tag.RowsAffected() != 1 {
		w.t.Fatalf("%s: %v, %d rows changed, want one", q, err, tag.RowsAffected())
	}
}

// loggedRow is one row of the log as the bell will read it.
type loggedRow struct {
	ID, Entry, Comment, Snapshot string
	Reasons                      []string
	At                           time.Time
}

// lastLogged is the newest row of the log about owner.
func (w *adminModWorld) lastLogged(owner feedPerson) loggedRow {
	w.t.Helper()
	var r loggedRow
	if err := w.pool.QueryRow(context.Background(), `select id::text, coalesce(entry_id::text, ''), coalesce(comment_id::text, ''),
		snapshot, reasons, at from moderation_events where owner_id = $1 order by created_at desc, id limit 1`, owner.id).Scan(
		&r.ID, &r.Entry, &r.Comment, &r.Snapshot, &r.Reasons, &r.At); err != nil {
		w.t.Fatal(err)
	}
	return r
}

// people signs up one finished account per username.
func (w *adminModWorld) people(usernames ...string) []feedPerson {
	w.t.Helper()
	out := make([]feedPerson, len(usernames))
	for i, u := range usernames {
		out[i] = w.person(u, "Person "+u)
	}
	return out
}

// Remove takes a post down for everyone, its owner too, and a comment down
// for everyone, its writer and the post's owner too. Nothing is deleted:
// the rows, the photos and their files stay. Every report open on it is
// closed as removed and nothing else, and the removal is logged once, on
// the owner's own clock, with the title or the text and the reasons of the
// reports it closed, the most serious first. The same Remove again is gone
// and logs nothing more. The Removed tab lists both, the newest first, with
// the days left, the ref of the removal and the photos signed.
func TestAdminModerationIntegration_RemoveIsForEveryoneAndDeletesNothing(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("am_bob", "am_vera", "am_wren", "am_rhea", "am_remy")
	b, v, cw, r1, r2 := ps[0], ps[1], ps[2], ps[3], ps[4]
	for _, p := range []feedPerson{v, cw, r1, r2} {
		w.followAccepted(p, b)
	}
	for _, p := range []feedPerson{b, cw, r1, r2} {
		w.followAccepted(p, v)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	vPost := w.post(v, day)
	for _, p := range []feedPerson{cw, r1, r2} {
		w.post(p, day)
	}
	comment := w.reply(cw, vPost, "Rude words", "")["id"].(string)
	w.ok(r1, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	w.ok(r2, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	w.ok(r2, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "harassment"})
	photos := postPhotoKeys(t, w.pool, post)
	ownersNow := w.clk.NowFor(b.id)

	form := w.cardForm("remove", "post", b, "Hate speech · Spam")
	if got := w.send("remove", form); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove the post: %q", got)
	}
	if page := w.page("/admin/people/" + b.id + "?n=removed"); !strings.Contains(page, "Removed. It&#39;s on the Removed tab for 30 days.") {
		t.Fatalf("the notice: %s", page)
	}
	for _, p := range []feedPerson{b, v} {
		if w.sees(p, post) {
			t.Fatalf("%s still sees the removed post", p.name)
		}
	}
	w.call(b, http.MethodGet, "/v1/entries/"+post, nil, http.StatusNotFound)
	if has(cardIDs(t, w.board(b, "")), post) {
		t.Fatal("the owner's Board still has the removed post")
	}
	if got := postPhotoKeys(t, w.pool, post); !slices.Equal(got, photos) || len(w.photos.deletes()) != 0 {
		t.Fatalf("Remove deleted something: photos %v (want %v), files %v", got, photos, w.photos.deletes())
	}
	if got := w.one(`select string_agg(resolution, ' ') from post_reports where entry_id = $1`, post); got != "removed removed" {
		t.Fatalf("the post's reports: %s, want both closed as removed", got)
	}
	if got := w.one(`select coalesce(resolution, 'open') from comment_reports where comment_id = $1`, comment); got != "open" {
		t.Fatalf("the comment's report: %s, want it left open", got)
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post}) {
		t.Fatalf("the log: %v", got)
	}
	removal := w.lastLogged(b)
	if removal.Entry != post || removal.Comment != "" || removal.Snapshot != "Dune" ||
		!slices.Equal(removal.Reasons, []string{"hate", "spam"}) || !removal.At.Equal(ownersNow) {
		t.Fatalf("the removal logged: %+v, want the post, its title, hate then spam, at %v", removal, ownersNow)
	}

	// The same Remove again: gone, nothing more logged.
	if got := w.send("remove", form); got != "/admin/people/"+b.id+"?n=gone" {
		t.Fatalf("Remove again: %q", got)
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 1 {
		t.Fatalf("Remove again logged more: %v", got)
	}

	tab := w.page("/admin/removed")
	card := adminReportOf(t, tab, "<h3>Dune</h3>")
	for _, want := range []string{
		`Hate speech · Spam</p><p class="kind">Post</p>`,
		` · 30 days left · Ref: ` + removal.ID[:8] + `</p>`,
		`<a href="` + fakePhotoLink(photos[0]) + `" target="_blank" rel="noopener"><img class="thumb" src="` + fakePhotoLink(photos[0]) + `"`,
		`<strong>@am_bob</strong>`,
		`<a class="button push" href="/admin/people/` + b.id + `">Open account</a>`,
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("the Removed tab's card lacks %q: %s", want, card)
		}
	}
	if !strings.Contains(tab, `class="on" aria-current="page">Removed<span class="n">1</span>`) || strings.Contains(tab, `http-equiv="refresh"`) {
		t.Fatalf("the Removed tab, want its count and no refresh: %s", tab)
	}

	// The comment: gone for its writer and for the post's owner.
	if got := w.act("remove", "comment", cw, "Bullying or harassment"); got != "/admin/people/"+cw.id+"?n=removed" {
		t.Fatalf("remove the comment: %q", got)
	}
	for _, p := range []feedPerson{cw, v} {
		if ids, n := w.commentIDs(p, vPost); len(ids) != 0 || n != 0 {
			t.Fatalf("%s still sees the removed comment: %v (%v)", p.name, ids, n)
		}
	}
	if got := loggedActions(t, w.pool, cw.id); !slices.Equal(got, []string{"removed comment " + comment}) {
		t.Fatalf("the writer's log: %v", got)
	}
	if got := w.lastLogged(cw); got.Entry != vPost || got.Comment != comment || got.Snapshot != "Rude words" ||
		!slices.Equal(got.Reasons, []string{"harassment"}) || !got.At.Equal(ownersNow) {
		t.Fatalf("the comment's removal logged: %+v", got)
	}
	tab = w.page("/admin/removed")
	if at := strings.Index(tab, `<p class="text quote">Rude words</p>`); at < 0 || at > strings.Index(tab, "<h3>Dune</h3>") ||
		!strings.Contains(tab, `Removed<span class="n">2</span>`) {
		t.Fatalf("the Removed tab, want the comment first and both counted: %s", tab)
	}
	for _, path := range []string{"/admin", "/admin/disabled", "/admin/people/" + b.id} {
		if page := w.page(path); !strings.Contains(page, `<a href="/admin/removed">Removed<span class="n">2</span></a>`) {
			t.Fatalf("%s, want the Removed tab's count of 2: %s", path, page)
		}
	}
	if len(w.photos.deletes()) != 0 {
		t.Fatalf("a removal deleted files: %v", w.photos.deletes())
	}
}

// Bring it back on the Removed tab shows a removed post or comment again for
// its owner and everyone, never for its reporter, logs it, and takes it off
// the tab and off Removed before. Removed again, it stays on the tab up to
// the 30 days on the database's clock; at them it is off the tab and Bring
// it back is refused, logging nothing.
func TestAdminModerationIntegration_BringItBackWorksFor30Days(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("ab_bob", "ab_vera", "ab_wren", "ab_rhea", "ab_remy", "ab_rory")
	b, v, cw, r1, r2, r3 := ps[0], ps[1], ps[2], ps[3], ps[4], ps[5]
	for _, p := range []feedPerson{v, cw, r1, r2, r3} {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	for _, p := range []feedPerson{v, cw, r1, r2, r3} {
		w.post(p, day)
	}
	comment := w.reply(cw, post, "Rude words", "")["id"].(string)
	w.ok(r1, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	w.ok(r3, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "harassment"})
	ownersNow := w.clk.NowFor(b.id)

	if got := w.act("remove", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove: %q", got)
	}
	if page := w.page("/admin/people/" + b.id); strings.Count(page, "Removed before: 1") != 1 {
		t.Fatalf("the profile card, want Removed before: 1 once: %s", page)
	}
	// Bring it back naming someone else as its owner changes nothing.
	if got := w.restoreAnyway("post", post, v); got != "/admin/removed?n=gone" || w.sees(b, post) {
		t.Fatalf("Bring it back for the wrong owner: %q", got)
	}
	if got := w.restore("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("bring it back: %q", got)
	}
	if tab := w.page("/admin/removed?n=restored"); !strings.Contains(tab, "Brought back. They were told.") ||
		!strings.Contains(tab, "Nothing removed in the last 30 days.") || !strings.Contains(tab, `Removed<span class="n">0</span>`) {
		t.Fatalf("the Removed tab after Bring it back: %s", tab)
	}
	for p, want := range map[feedPerson]bool{b: true, v: true, cw: true, r1: false} {
		if w.sees(p, post) != want {
			t.Fatalf("after Bring it back %s sees the post: %v, want %v", p.name, !want, want)
		}
	}
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"removed post " + post, "restored post " + post}) {
		t.Fatalf("the log: %v", got)
	}
	if got := w.lastLogged(b); got.Snapshot != "Dune" || got.Entry != post || got.Comment != "" || len(got.Reasons) != 0 || !got.At.Equal(ownersNow) {
		t.Fatalf("Bring it back logged: %+v", got)
	}
	if page := w.page("/admin/people/" + b.id); strings.Contains(page, "Removed before") {
		t.Fatalf("Removed before after Bring it back: %s", page)
	}

	// A comment comes back the same way, for everyone but its reporter.
	if got := w.act("remove", "comment", cw, "Bullying or harassment"); got != "/admin/people/"+cw.id+"?n=removed" {
		t.Fatalf("remove the comment: %q", got)
	}
	if ids, _ := w.commentIDs(cw, post); len(ids) != 0 {
		t.Fatalf("the writer still sees the removed comment: %v", ids)
	}
	if got := w.restore("comment", comment, cw); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the comment back: %q", got)
	}
	for _, p := range []feedPerson{cw, v, b} {
		if ids, _ := w.commentIDs(p, post); !slices.Equal(ids, []string{comment}) {
			t.Fatalf("%s after the comment came back: %v", p.name, ids)
		}
	}
	if ids, _ := w.commentIDs(r3, post); len(ids) != 0 {
		t.Fatalf("the comment's reporter sees it again: %v", ids)
	}
	if got := loggedActions(t, w.pool, cw.id); !slices.Equal(got, []string{"removed comment " + comment, "restored comment " + comment}) {
		t.Fatalf("the writer's log: %v", got)
	}
	if got := w.lastLogged(cw); got.Entry != post || got.Comment != comment || got.Snapshot != "Rude words" {
		t.Fatalf("the comment brought back, logged: %+v", got)
	}

	// Removed again on a new report, then moved to an hour short of the 30
	// days, then to the 30 days.
	w.ok(r2, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	if got := w.act("remove", "post", b, "Hate speech"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove again: %q", got)
	}
	if page := w.page("/admin/people/" + b.id); strings.Count(page, "Removed before: 1") != 1 {
		t.Fatalf("removed, brought back and removed: want Removed before: 1 once: %s", page)
	}
	w.exec(`update entries set removed_at = now() - interval '30 days' + interval '1 hour' where id = $1`, post)
	again := w.lastLogged(b)
	if tab := w.page("/admin/removed"); !strings.Contains(tab, "Hate speech</p><p class=\"kind\">Post</p>") ||
		!strings.Contains(tab, " · 1 day left · Ref: "+again.ID[:8]+"</p>") || strings.Contains(tab, "Spam") {
		t.Fatalf("an hour short of the 30 days, want 1 day left and the newest removal's ref and reasons: %s", tab)
	}
	w.exec(`update entries set removed_at = now() - interval '30 days' where id = $1`, post)
	if tab := w.page("/admin/removed"); !strings.Contains(tab, "Nothing removed in the last 30 days.") {
		t.Fatalf("at the 30 days the tab still lists it: %s", tab)
	}
	if got := w.restoreAnyway("post", post, b); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back at the 30 days: %q", got)
	}
	if tab := w.page("/admin/removed?n=gone"); !strings.Contains(tab, "That post or comment is already gone.") {
		t.Fatalf("the notice: %s", tab)
	}
	if w.sees(b, post) || w.one(`select (removed_at is not null)::text from entries where id = $1`, post) != "true" {
		t.Fatal("a refused Bring it back brought it back")
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 3 {
		t.Fatalf("a refused Bring it back logged: %v", got)
	}
}

// Dismiss of a hidden post shows it again for everyone but its reporters,
// logs it once on the owner's clock and says so; the Reports tab put the
// hidden card first, with its tag, even ahead of violence. One that was
// never hidden logs nothing. A card drawn before more reports came in
// follows the same rule: drawn with two, dismissed after a third hid the
// post, it shows again; drawn with one, dismissed after three more raced in
// (nobody can report a hidden post, so straight in the database), it stays
// hidden and logs nothing until the rest are dismissed. A hidden comment
// whose post was removed does not show again.
func TestAdminModerationIntegration_DismissShowsAHiddenItemAgainOnce(t *testing.T) {
	w := newAdminModWorld(t)
	owners := w.people("ad_hana", "ad_noel", "ad_pia", "ad_zed")
	oh, on, op, oz := owners[0], owners[1], owners[2], owners[3]
	others := w.people("ad_vera", "ad_wren", "ad_r1", "ad_r2", "ad_r3", "ad_r4", "ad_r5", "ad_r6")
	v, cw, r := others[0], others[1], others[2:]
	for _, p := range others {
		for _, o := range owners {
			w.followAccepted(p, o)
		}
	}
	const day = "2026-10-08"
	hidden, never, partial, zPost := w.post(oh, day), w.post(on, day), w.post(op, day), w.post(oz, day)
	for _, p := range others {
		w.post(p, day)
	}
	report := func(p feedPerson, what, id, reason string) {
		t.Helper()
		w.ok(p, http.MethodPost, "/v1/"+what+"/"+id+"/report", map[string]any{"reason": reason})
	}
	for _, p := range r[:3] {
		report(p, "posts", hidden, "spam")
	}
	report(r[3], "posts", never, "violence")
	report(r[4], "posts", never, "nudity")
	if w.sees(v, hidden) || !w.sees(v, never) {
		t.Fatal("three reports hide the post, two do not")
	}
	list := w.page("/admin")
	hiddenCard, violent := adminReportOf(t, list, "<strong>@ad_hana</strong>"), adminReportOf(t, list, "<strong>@ad_noel</strong>")
	if !strings.Contains(hiddenCard, `<p class="kind">Post</p></div>`+"\n"+`<p class="flag"><span class="tag">Hidden</span> Only the owner sees it for now.</p>`) ||
		strings.Contains(violent, "Hidden") || strings.Index(list, "<strong>@ad_hana</strong>") > strings.Index(list, "<strong>@ad_noel</strong>") {
		t.Fatalf("the Reports tab, want the hidden spam card first with its tag, ahead of violence: %s", list)
	}
	ownersNow := w.clk.NowFor(oh.id)

	if got := w.act("dismiss", "post", oh, "Spam"); got != "/admin/people/"+oh.id+"?n=shownagain" {
		t.Fatalf("dismiss the hidden post: %q", got)
	}
	if page := w.page("/admin/people/" + oh.id + "?n=shownagain"); !strings.Contains(page, "Dismissed. It shows again, and they were told.") ||
		strings.Contains(page, `<span class="tag">Hidden</span>`) {
		t.Fatalf("after the dismissal, want the notice and no Hidden tag on the closed card: %s", page)
	}
	if !w.sees(v, hidden) || w.sees(r[0], hidden) {
		t.Fatal("after the dismissal the post should show for everyone but its reporters")
	}
	if got := loggedActions(t, w.pool, oh.id); !slices.Equal(got, []string{"shown_again post " + hidden}) {
		t.Fatalf("the log: %v", got)
	}
	if got := w.lastLogged(oh); got.Entry != hidden || got.Comment != "" || got.Snapshot != "Dune" || !got.At.Equal(ownersNow) {
		t.Fatalf("shown again logged: %+v", got)
	}

	if got := w.act("dismiss", "post", on, "Violence or threats · Nudity or sexual content"); got != "/admin/people/"+on.id+"?n=dismissed" {
		t.Fatalf("dismiss a post never hidden: %q", got)
	}
	if got := loggedActions(t, w.pool, on.id); len(got) != 0 {
		t.Fatalf("a post never hidden logged: %v", got)
	}

	// Drawn with two reports; a third hides it; the card's two are dismissed.
	for _, p := range r[:2] {
		report(p, "posts", partial, "harassment")
	}
	drawn := w.cardForm("dismiss", "post", op, "Bullying or harassment")
	report(r[2], "posts", partial, "hate")
	if w.sees(v, partial) {
		t.Fatal("the third report did not hide it")
	}
	if got := w.send("dismiss", drawn); got != "/admin/people/"+op.id+"?n=shownagain" {
		t.Fatalf("dismiss the card drawn with two: %q", got)
	}
	if !w.sees(v, partial) || !slices.Equal(loggedActions(t, w.pool, op.id), []string{"shown_again post " + partial}) {
		t.Fatalf("after the card drawn with two: sees %v, log %v", w.sees(v, partial), loggedActions(t, w.pool, op.id))
	}
	// Drawn with the third alone; three more race in.
	drawn = w.cardForm("dismiss", "post", op, "Hate speech")
	for _, p := range r[3:] {
		w.exec(`insert into post_reports (reporter_id, entry_id, owner_id, title, reason, created_at)
			values ($1, $2, $3, 'Dune', 'violence', now())`, p.id, partial, op.id)
	}
	if got := w.send("dismiss", drawn); got != "/admin/people/"+op.id+"?n=dismissed" {
		t.Fatalf("dismiss the card drawn with one: %q", got)
	}
	if w.sees(v, partial) || len(loggedActions(t, w.pool, op.id)) != 1 {
		t.Fatal("three reports left open: it stays hidden and nothing more is logged")
	}
	if got := w.act("dismiss", "post", op, "Violence or threats"); got != "/admin/people/"+op.id+"?n=shownagain" {
		t.Fatalf("dismiss the rest: %q", got)
	}
	if got := loggedActions(t, w.pool, op.id); !w.sees(v, partial) || !slices.Equal(got, []string{"shown_again post " + partial, "shown_again post " + partial}) {
		t.Fatalf("after the rest: sees %v, log %v", w.sees(v, partial), got)
	}

	// A hidden comment, then its post is removed: dismissing the comment's
	// reports shows nothing again.
	comment := w.reply(cw, zPost, "Rude words", "")["id"].(string)
	for _, p := range r[3:] {
		report(p, "comments", comment, "other")
	}
	if card := adminReportOf(t, w.page("/admin/people/"+cw.id), "Something else"); !strings.Contains(card, `<span class="tag">Hidden</span>`) {
		t.Fatalf("the hidden comment's card, want its tag: %s", card)
	}
	report(r[0], "posts", zPost, "spam")
	if got := w.act("remove", "post", oz, "Spam"); got != "/admin/people/"+oz.id+"?n=removed" {
		t.Fatalf("remove its post: %q", got)
	}
	if card := adminReportOf(t, w.page("/admin/people/"+cw.id), "Something else"); strings.Contains(card, "Hidden") {
		t.Fatalf("a comment its post's removal keeps from everyone is not hidden under review: %s", card)
	}
	if got := w.act("dismiss", "comment", cw, "Something else"); got != "/admin/people/"+cw.id+"?n=dismissed" {
		t.Fatalf("dismiss the comment on a removed post: %q", got)
	}
	if got := loggedActions(t, w.pool, cw.id); len(got) != 0 {
		t.Fatalf("a comment on a removed post logged: %v", got)
	}
}

// Disable and Enable are logged, each once per change of the account and
// on its own clock, and its page's History lists them, the newest first.
func TestAdminModerationIntegration_DisableAndEnableAreLogged(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("al_bob", "al_rhea")
	b, r := ps[0], ps[1]
	w.ok(r, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	ownersNow := w.clk.NowFor(b.id)
	toggle := func(action, notice string) {
		t.Helper()
		page := w.page("/admin/people/" + b.id)
		rec := adminDo(t, w.feedWorld, http.MethodPost, "/admin/people/"+b.id+"/"+action, w.cookie, url.Values{"csrf": {csrfOf(t, page)}}, "")
		if got := rec.Header().Get("Location"); got != "/admin/people/"+b.id+"?n="+notice {
			t.Fatalf("%s: %q", action, got)
		}
	}
	toggle("disable", "disabled")
	toggle("disable", "disabled")
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"disabled account " + b.id}) {
		t.Fatalf("after two disables: %v", got)
	}
	toggle("enable", "enabled")
	toggle("enable", "enabled")
	if got := loggedActions(t, w.pool, b.id); !slices.Equal(got, []string{"disabled account " + b.id, "enabled account " + b.id}) {
		t.Fatalf("after two enables: %v", got)
	}
	if got := w.lastLogged(b); got.Entry != "" || got.Comment != "" || got.Snapshot != "" || !got.At.Equal(ownersNow) {
		t.Fatalf("enable logged: %+v", got)
	}
	page := w.page("/admin/people/" + b.id)
	when := ownersNow.Local().Format("Jan 2, 2006 3:04 PM")
	enabled := strings.Index(page, `<li><p><strong>Account enabled</strong> <span class="muted when">`+when+`</span></p></li>`)
	disabled := strings.Index(page, `<li><p><strong>Account disabled</strong> <span class="muted when">`+when+`</span></p></li>`)
	if enabled < 0 || disabled < enabled || strings.Index(page, "<h2>History</h2>") > enabled {
		t.Fatalf("the History, want Account enabled then Account disabled: %s", page)
	}
}

// Removed before counts the owner's removals that were not brought back:
// removed, brought back and removed again is one; a removed comment is
// another. The Reports tab shows it in a card's owner row (in a person card,
// its body), only when it is more than 0, and the person page once, in the
// profile card. The person page's History lists every decision, the newest
// first, a removal with its ref and reasons.
func TestAdminModerationIntegration_RemovedBeforeCountsWhatWasNotBroughtBack(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("ar_bob", "ar_dana", "ar_r1", "ar_r2", "ar_r3", "ar_r4")
	b, d, r1, r2, r3, r4 := ps[0], ps[1], ps[2], ps[3], ps[4], ps[5]
	for _, p := range []feedPerson{d, r1, r2, r3, r4} {
		w.followAccepted(p, b)
	}
	for _, p := range []feedPerson{b, r1, r2, r3, r4} {
		w.followAccepted(p, d)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	dPost := w.post(d, day)
	for _, p := range []feedPerson{r1, r2, r3, r4} {
		w.post(p, day)
	}
	mean := w.reply(b, dPost, "Mean words", "")["id"].(string)
	other := w.reply(b, dPost, "Other words", "")["id"].(string)

	w.ok(r1, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	w.act("remove", "post", b, "Spam")
	w.ok(r1, http.MethodPost, "/v1/comments/"+mean+"/report", map[string]any{"reason": "hate"})
	w.act("remove", "comment", b, "Hate speech")
	w.restore("post", post, b)
	w.ok(r2, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "nudity"})
	if got := w.act("remove", "post", b, "Nudity or sexual content"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove again: %q", got)
	}

	// Open cards for both: B's account, B's other comment, D's post.
	w.ok(r3, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "impersonation"})
	w.ok(r4, http.MethodPost, "/v1/comments/"+other+"/report", map[string]any{"reason": "other"})
	w.ok(r3, http.MethodPost, "/v1/posts/"+dPost+"/report", map[string]any{"reason": "violence"})
	list := w.page("/admin")
	if card := adminReportOf(t, list, "Something else"); !strings.Contains(card, `<span class="before">Removed before: 2</span></div>`) {
		t.Fatalf("B's comment card, want Removed before: 2 in its owner row: %s", card)
	}
	if card := adminReportOf(t, list, "Pretending to be someone else"); !strings.Contains(card, `<p class="by">Removed before: 2</p>`) {
		t.Fatalf("B's account card: %s", card)
	}
	if card := adminReportOf(t, list, "Violence or threats"); strings.Contains(card, "Removed before") {
		t.Fatalf("D's card, with nothing removed: %s", card)
	}

	page := w.page("/admin/people/" + b.id)
	if strings.Count(page, "Removed before: 2") != 1 || !strings.Contains(page, `Joined <span class="when">`) ||
		strings.Index(page, "Removed before: 2") > strings.Index(page, "</section>") {
		t.Fatalf("B's page, want Removed before: 2 once, in the profile card: %s", page)
	}
	log := page[strings.Index(page, "<h2>History</h2>"):strings.Index(page, "<h2>Open reports</h2>")]
	var labels []string
	for _, part := range strings.Split(log, "<strong>")[1:] {
		labels = append(labels, part[:strings.Index(part, "</strong>")])
	}
	if want := []string{"Post removed", "Post brought back", "Comment removed", "Post removed"}; !slices.Equal(labels, want) {
		t.Fatalf("the History, newest first: %v, want %v", labels, want)
	}
	newest := w.lastLogged(b)
	if !strings.Contains(log, `<p class="by">Ref: `+newest.ID[:8]+` · Nudity or sexual content</p><p class="text quote">Dune</p>`) ||
		!strings.Contains(log, `<p class="text quote">Mean words</p>`) {
		t.Fatalf("the History's removals, want the ref, the reasons and what was removed: %s", log)
	}
}

// The report that makes a post or comment hidden while under review is the
// one the admin email hears as urgent: the third open one from an account
// that is not gone. The ones before it, and the reports of a person, are
// plain (after it, nobody else can see it to report it). Once its reports
// are dismissed, the third new one hides it again. A disabled reporter's
// report does not count, so the report that hides a comment can come fourth.
func TestAdminModerationIntegration_TheReportThatHidesItIsUrgent(t *testing.T) {
	w := newAdminModWorld(t)
	owners := w.people("au_bob", "au_cy")
	b, c := owners[0], owners[1]
	r := w.people("au_r1", "au_r2", "au_r3", "au_r4", "au_r5", "au_r6", "au_r7")
	for _, p := range r {
		w.followAccepted(p, b)
		w.followAccepted(p, c)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	cPost := w.post(c, day)
	for _, p := range r {
		w.post(p, day)
	}
	report := func(p feedPerson, what, id, reason string) {
		t.Helper()
		w.ok(p, http.MethodPost, "/v1/"+what+"/"+id+"/report", map[string]any{"reason": reason})
	}
	report(r[0], "posts", post, "spam")
	report(r[1], "posts", post, "spam")
	if got := w.heard.hiding(); len(got) != 0 {
		t.Fatalf("two reports hid nothing, yet: %v", got)
	}
	report(r[2], "posts", post, "hate")
	report(r[4], "people", b.id, "impersonation")
	if got := w.heard.hiding(); !slices.Equal(got, []string{"hate"}) {
		t.Fatalf("hiding reports: %v, want the third only", got)
	}
	if got := w.heard.got(); !slices.Equal(got, []string{"spam", "spam", "hate", "impersonation"}) {
		t.Fatalf("every report is still heard once: %v", got)
	}
	if got := w.act("dismiss", "post", b, "Hate speech · Spam"); got != "/admin/people/"+b.id+"?n=shownagain" {
		t.Fatalf("dismiss: %q", got)
	}
	report(r[4], "posts", post, "spam")
	report(r[5], "posts", post, "spam")
	report(r[6], "posts", post, "nudity")
	if got := w.heard.hiding(); !slices.Equal(got, []string{"hate", "nudity"}) {
		t.Fatalf("after the dismissal: %v, want the third new one", got)
	}

	// A comment, one of whose reporters is disabled meanwhile.
	comment := w.reply(r[6], cPost, "Rude words", "")["id"].(string)
	report(r[0], "comments", comment, "harassment")
	report(r[1], "comments", comment, "harassment")
	w.exec(`update users set disabled_at = now() where id = $1`, r[0].id)
	report(r[2], "comments", comment, "harassment")
	if got := w.heard.hiding(); len(got) != 2 {
		t.Fatalf("a disabled reporter counted: %v", got)
	}
	report(r[3], "comments", comment, "violence")
	if got := w.heard.hiding(); !slices.Equal(got, []string{"hate", "nudity", "violence"}) {
		t.Fatalf("the comment's hiding report: %v, want the fourth, with one reporter gone", got)
	}
}
