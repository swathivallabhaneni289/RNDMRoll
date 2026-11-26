package httpapi

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
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
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &adminModWorld{feedWorld: &feedWorld{t: t, clk: clk}, pool: pool, photos: &fakeEntryPhotoStore{}, heard: &heardReports{}}
	adminFirst(w.feedWorld, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"}, entryPhotos: w.photos,
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
// the report id it carries and the reason its Reason list has selected,
// Dismiss with every id on the card (and on a person's card every time).
func (w *adminModWorld) cardForm(action, kind string, owner feedPerson, mark string) url.Values {
	w.t.Helper()
	page := w.page("/admin/people/" + owner.id)
	card := adminReportOf(w.t, page, mark)
	form := url.Values{"csrf": {csrfOf(w.t, page)}, "kind": {kind}, "person": {owner.id}}
	if action == "dismiss" {
		dismissForm(card, form)
	} else {
		form.Set("id", adminReportID(w.t, card))
		form.Set("reason", adminRemoveReason(w.t, card))
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
	// Reason is the reason the admin gave at a removal, "" for none.
	Reason string
	At     time.Time
}

// lastLogged is the newest row of the log about owner.
func (w *adminModWorld) lastLogged(owner feedPerson) loggedRow {
	w.t.Helper()
	var r loggedRow
	if err := w.pool.QueryRow(context.Background(), `select id::text, coalesce(entry_id::text, ''), coalesce(comment_id::text, ''),
		snapshot, reasons, reason, at from moderation_events where owner_id = $1 order by created_at desc, id limit 1`, owner.id).Scan(
		&r.ID, &r.Entry, &r.Comment, &r.Snapshot, &r.Reasons, &r.Reason, &r.At); err != nil {
		w.t.Fatal(err)
	}
	return r
}

// historyOf is the History on a person page: its heading to the end of
// its list.
func historyOf(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, "<h2>History</h2>")
	if start < 0 {
		t.Fatalf("no History: %s", page)
	}
	end := strings.Index(page[start:], "</ol>")
	if end < 0 {
		t.Fatalf("the History has no end: %s", page)
	}
	return page[start : start+end+len("</ol>")]
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
// the owner's own clock, with the title or the text, the reasons of the
// reports it closed, the most serious first, and the reason the admin
// picked (the Reason list starts on the most serious). The same Remove
// again (a double click) says what the first said, and logs and tells
// nothing more. The Removed tab lists both, the newest first, with the
// reason, the days left, the ref of the removal and the photos signed.
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
		!slices.Equal(removal.Reasons, []string{"hate", "spam"}) || removal.Reason != "hate" || !removal.At.Equal(ownersNow) {
		t.Fatalf("the removal logged: %+v, want the post, its title, hate then spam, the reason hate, at %v", removal, ownersNow)
	}

	// The same Remove again: removed, nothing more logged or told.
	if got := w.send("remove", form); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("Remove again: %q", got)
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 1 {
		t.Fatalf("Remove again logged more: %v", got)
	}
	if got := w.decisionIDs(b); !slices.Equal(got, []string{"post_removed:" + removal.ID}) {
		t.Fatalf("Remove again reached the owner's bell: %v", got)
	}

	tab := w.page("/admin/removed")
	card := adminReportOf(t, tab, "<h3>Dune</h3>")
	for _, want := range []string{
		`<div class="type"><p class="kind">Post</p></div>`,
		`<div class="about"><p class="reasons">Hate speech</p>`,
		`<p class="by">30 days left · Ref: ` + removal.ID[:8] + `</p>`,
		`<a href="` + fakePhotoLink(photos[0]) + `" target="_blank" rel="noopener"><img class="thumb" src="` + fakePhotoLink(photos[0]) + `"`,
		`<strong>@am_bob</strong>`,
		`<a class="button push" href="/admin/people/` + b.id + `?from=removed">Open profile</a>`,
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
		!slices.Equal(got.Reasons, []string{"harassment"}) || got.Reason != "harassment" || !got.At.Equal(ownersNow) {
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

// reasonListOf is Remove's Reason list as the page draws it, with selected
// picked: the seven report reasons, the most serious first.
func reasonListOf(selected string) string {
	var b strings.Builder
	b.WriteString(`<label class="reason">Reason <select name="reason">`)
	for _, r := range [][2]string{{"violence", "Violence or threats"}, {"nudity", "Nudity or sexual content"},
		{"harassment", "Bullying or harassment"}, {"hate", "Hate speech"}, {"impersonation", "Pretending to be someone else"},
		{"spam", "Spam"}, {"other", "Something else"}} {
		b.WriteString(`<option value="` + r[0] + `"`)
		if r[0] == selected {
			b.WriteString(" selected")
		}
		b.WriteString(">" + r[1] + "</option>")
	}
	b.WriteString(`</select></label>`)
	return b.String()
}

// Every Remove form, on the Reports tab and on the person page, for a post
// and for a comment, has the Reason list (developer, 2026-11-18): the seven
// report reasons, the most serious first, with exactly one selected, the
// most serious reason the card was reported for. A post reported for spam
// and violence starts on violence, a comment reported for hate and bullying
// on bullying, and a post reported only for Something else on it. A card
// about an account has no Remove, and so no list.
func TestAdminModerationIntegration_RemoveListsTheReasonsMostSeriousFirst(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("rl_bob", "rl_cleo", "rl_vera", "rl_r1", "rl_r2")
	b, c, v, r1, r2 := ps[0], ps[1], ps[2], ps[3], ps[4]
	for _, p := range []feedPerson{v, r1, r2} {
		w.followAccepted(p, b)
		w.followAccepted(p, c)
	}
	const day = "2026-10-08"
	post, cPost := w.post(b, day), w.post(c, day)
	for _, p := range []feedPerson{v, r1, r2} {
		w.post(p, day)
	}
	comment := w.reply(v, cPost, "Rude words", "")["id"].(string)
	report := func(p feedPerson, what, id, reason string) {
		t.Helper()
		w.ok(p, http.MethodPost, "/v1/"+what+"/"+id+"/report", map[string]any{"reason": reason})
	}
	report(r1, "posts", post, "spam")
	report(r2, "posts", post, "violence")
	// A post a reporter reported is gone for them, with its comments, so the
	// comment is reported first.
	report(r1, "comments", comment, "hate")
	report(r2, "comments", comment, "harassment")
	report(r1, "posts", cPost, "other")
	report(r2, "people", b.id, "impersonation")

	list := w.page("/admin")
	for _, want := range []struct {
		owner                  feedPerson
		mark, selected, button string
	}{
		{b, "Violence or threats · Spam", "violence", "Remove post"},
		{v, "Bullying or harassment · Hate speech", "harassment", "Remove comment"},
		{c, `<p class="reasons">Something else</p>`, "other", "Remove post"},
	} {
		for _, page := range []string{list, w.page("/admin/people/" + want.owner.id)} {
			card := adminReportOf(t, page, want.mark)
			if !strings.Contains(card, reasonListOf(want.selected)+`<button class="strong">`+want.button+`</button></form>`) ||
				strings.Count(card, "<select") != 1 || strings.Count(card, " selected>") != 1 {
				t.Fatalf("the card holding %q, want one Reason list, on %s, beside %s: %s", want.mark, want.selected, want.button, card)
			}
		}
	}
	if account := adminReportOf(t, list, "Pretending to be someone else"); strings.Contains(account, "<select") {
		t.Fatalf("the account's card has a Reason list: %s", account)
	}
	if n := strings.Count(list, `<label class="reason">`); n != 3 {
		t.Fatalf("%d Reason lists on the Reports tab, want one for each Remove: %s", n, list)
	}
}

// The admin can give another reason than the reporters did, and the one
// given is what the owner hears and what the page keeps (developer,
// 2026-11-18): a post reported for spam and removed for hate speech is
// "hate" in its owner's bell, the log keeps hate and still the reports'
// spam, and the History and the Removed tab say Hate speech. A second click
// with another reason changes nothing, its reason included. A reason that is
// not a report reason's key goes back to the list, and removes, closes and
// logs nothing. A form with no reason takes the most serious reason of the
// reports it closes, and a removal that closes none keeps no reason, so the
// bell sends none.
func TestAdminModerationIntegration_RemoveKeepsTheReasonTheAdminGave(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("rg_bob", "rg_vera", "rg_wren", "rg_r1", "rg_r2")
	b, v, cw, r1, r2 := ps[0], ps[1], ps[2], ps[3], ps[4]
	for _, p := range []feedPerson{cw, r1, r2} {
		w.followAccepted(p, b)
		w.followAccepted(p, v)
	}
	const day = "2026-10-08"
	post, vPost, cwPost := w.post(b, day), w.post(v, day), w.post(cw, day)
	for _, p := range []feedPerson{r1, r2} {
		w.post(p, day)
	}
	comment := w.reply(cw, vPost, "Rude words", "")["id"].(string)
	w.ok(r1, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "spam"})
	w.ok(r2, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "nudity"})
	w.ok(r1, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})

	// The list starts on the reporters' spam. Any other word is refused.
	form := w.cardForm("remove", "post", b, `<p class="reasons">Spam</p>`)
	if got := form.Get("reason"); got != "spam" {
		t.Fatalf("the Reason list starts on %q, want spam", got)
	}
	for _, bad := range []string{"rude", "Hate speech", "HATE", " hate"} {
		refused := maps.Clone(form)
		refused.Set("reason", bad)
		if got := w.send("remove", refused); got != "/admin" {
			t.Fatalf("the reason %q: %q, want the list", bad, got)
		}
	}
	if !w.sees(cw, post) || len(loggedActions(t, w.pool, b.id)) != 0 ||
		w.one(`select coalesce(resolution, 'open') from post_reports where entry_id = $1`, post) != "open" {
		t.Fatal("a reason that is not a report reason's key removed, logged or closed something")
	}

	// Removed for hate speech.
	form.Set("reason", "hate")
	if got := w.send("remove", form); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove for hate speech: %q", got)
	}
	removal := w.lastLogged(b)
	if removal.Reason != "hate" || !slices.Equal(removal.Reasons, []string{"spam"}) {
		t.Fatalf("the removal logged: %+v, want the reason hate, and spam from the reports", removal)
	}
	item, _ := w.bellItem(b, "post_removed:"+removal.ID)
	wantItem(t, "the removal", item, map[string]any{"kind": "post_removed", "reason": "hate", "ref": removal.ID[:8], "removal": removal.ID})
	page := w.page("/admin/people/" + b.id)
	history := historyOf(t, page)
	if !strings.Contains(history, `<p class="by">Ref: `+removal.ID[:8]+` · Hate speech</p><p class="text quote">Dune</p>`) ||
		strings.Contains(history, "Spam") {
		t.Fatalf("the History, want the removal with Hate speech alone: %s", history)
	}
	if card := adminReportOf(t, w.page("/admin/removed"), "<h3>Dune</h3>"); !strings.Contains(card, `<div class="about"><p class="reasons">Hate speech</p>`) ||
		strings.Contains(card, "Spam") {
		t.Fatalf("the Removed tab's card, want Hate speech alone: %s", card)
	}

	// A second click with another reason says removed, and the first reason
	// stays.
	form.Set("reason", "violence")
	if got := w.send("remove", form); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove again: %q", got)
	}
	if got, logged := w.lastLogged(b), loggedActions(t, w.pool, b.id); len(logged) != 1 || got.ID != removal.ID || got.Reason != "hate" {
		t.Fatalf("after a second click the log is %v, its newest %+v, want the first removal and its hate", logged, got)
	}
	if item, _ := w.bellItem(b, "post_removed:"+removal.ID); item["reason"] != "hate" {
		t.Fatalf("after a second click the bell's removal: %v, want hate", item)
	}

	// No reason: the most serious of the reports it closes.
	bare := w.cardForm("remove", "comment", cw, "Nudity or sexual content · Spam")
	if got := bare.Get("reason"); got != "nudity" {
		t.Fatalf("the comment's Reason list starts on %q, want nudity", got)
	}
	bare.Del("reason")
	if got := w.send("remove", bare); got != "/admin/people/"+cw.id+"?n=removed" {
		t.Fatalf("remove with no reason: %q", got)
	}
	gone := w.lastLogged(cw)
	if gone.Reason != "nudity" || !slices.Equal(gone.Reasons, []string{"nudity", "spam"}) {
		t.Fatalf("the comment removed with no reason: %+v, want nudity, the most serious", gone)
	}
	if item, _ := w.bellItem(cw, "comment_removed:"+gone.ID); item["reason"] != "nudity" {
		t.Fatalf("the writer's bell: %v, want nudity", item)
	}

	// A removal that closes no report, with no reason (straight through the
	// store), keeps none, and the bell sends none.
	if err := postgres.NewAdminRepo(w.pool).RemoveItem(context.Background(), "post", uuid.MustParse(cwPost), uuid.MustParse(cw.id), "",
		time.Now().UTC(), w.clk.NowFor(cw.id)); err != nil {
		t.Fatalf("remove a post nobody reported: %v", err)
	}
	quiet := w.lastLogged(cw)
	if quiet.Entry != cwPost || quiet.Reason != "" || len(quiet.Reasons) != 0 {
		t.Fatalf("a removal that closed no report: %+v, want no reason", quiet)
	}
	item, _ = w.bellItem(cw, "post_removed:"+quiet.ID)
	wantItem(t, "a removal with no reason", item, map[string]any{"kind": "post_removed", "ref": quiet.ID[:8]}, "reason")
}

// Bring it back on the Removed tab shows a removed post or comment again for
// its owner and everyone, never for its reporter, logs it, and takes it off
// the tab and off Removed before. The same again (a double click) says what
// the first said and logs nothing; for something never removed, for someone
// else, or for something decided about since, it is gone. Removed again, it
// stays on the tab up to the 30 days on the database's clock; at them it is
// off the tab, and Bring it back and Remove are refused, logging nothing.
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

	if got := w.restoreAnyway("post", post, b); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back on a post never removed: %q", got)
	}
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
	if got := w.restoreAnyway("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("Bring it back again: %q", got)
	}
	if got := w.restoreAnyway("post", post, v); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back again for the wrong owner: %q", got)
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
	// Again, it is back. Once something else was decided about it since (a
	// Dismiss that showed it again, straight in the log), it is gone.
	if got := w.restoreAnyway("comment", comment, cw); got != "/admin/removed?n=restored" || len(loggedActions(t, w.pool, cw.id)) != 2 {
		t.Fatalf("Bring the comment back again: %q, log %v", got, loggedActions(t, w.pool, cw.id))
	}
	w.exec(`insert into moderation_events (owner_id, item, action, entry_id, comment_id, item_id, at)
		values ($1, 'comment', 'shown_again', $2, $3, $3, now())`, cw.id, post, comment)
	if got := w.restoreAnyway("comment", comment, cw); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back on a comment shown again since: %q", got)
	}

	// Removed again on a new report, then moved to an hour short of the 30
	// days, then to the 30 days.
	w.ok(r2, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	removeAgain := w.cardForm("remove", "post", b, "Hate speech")
	if got := w.send("remove", removeAgain); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove again: %q", got)
	}
	if page := w.page("/admin/people/" + b.id); strings.Count(page, "Removed before: 1") != 1 {
		t.Fatalf("removed, brought back and removed: want Removed before: 1 once: %s", page)
	}
	w.exec(`update entries set removed_at = now() - interval '30 days' + interval '1 hour' where id = $1`, post)
	again := w.lastLogged(b)
	if tab := w.page("/admin/removed"); !strings.Contains(tab, `<div class="about"><p class="reasons">Hate speech</p>`) || !strings.Contains(tab, `<p class="kind">Post</p>`) ||
		!strings.Contains(tab, `<p class="by">1 day left · Ref: `+again.ID[:8]+"</p>") || strings.Contains(tab, "Spam") {
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
	if got := w.send("remove", removeAgain); got != "/admin/people/"+b.id+"?n=gone" {
		t.Fatalf("Remove again at the 30 days: %q", got)
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 3 {
		t.Fatalf("a refused Bring it back or Remove logged: %v", got)
	}
}

// Bring it back on a comment whose post, or the comment it answers, is still
// removed would leave it gone for everyone, so it is refused: nothing is
// logged, its writer hears nothing, and it stays on the Removed tab. With
// both removed, the notice names the post. Once both are back, so is it.
// Meanwhile the tab gives it the days left of the earliest removal above it.
func TestAdminModerationIntegration_ACommentComesBackOnlyAfterWhatItIsUnder(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("aw_bob", "aw_quin", "aw_wren", "aw_vera", "aw_rhea", "aw_remy", "aw_rory")
	b, qw, cw, v, r1, r2, r3 := ps[0], ps[1], ps[2], ps[3], ps[4], ps[5], ps[6]
	for _, p := range ps[1:] {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	for _, p := range ps[1:] {
		w.post(p, day)
	}
	parent := w.reply(qw, post, "First words", "")["id"].(string)
	reply := w.reply(cw, post, "Rude words", parent)["id"].(string)
	// A reporter loses what they report and all under it: the reply first,
	// then the comment it answers, then the post.
	w.ok(r1, http.MethodPost, "/v1/comments/"+reply+"/report", map[string]any{"reason": "harassment"})
	w.ok(r2, http.MethodPost, "/v1/comments/"+parent+"/report", map[string]any{"reason": "spam"})
	w.ok(r3, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	for _, rm := range []struct {
		kind  string
		owner feedPerson
		mark  string
	}{{"comment", cw, "Bullying or harassment"}, {"comment", qw, "Spam"}, {"post", b, "Hate speech"}} {
		if got := w.act("remove", rm.kind, rm.owner, rm.mark); got != "/admin/people/"+rm.owner.id+"?n=removed" {
			t.Fatalf("remove the %s of %s: %q", rm.kind, rm.owner.name, got)
		}
	}
	removal := w.lastLogged(cw)

	// The purge of a post takes its comments, and of a comment its replies,
	// so a comment's days left run from the earliest removal above it.
	w.exec(`update entries set removed_at = now() - interval '25 days' where id = $1`, post)
	w.exec(`update post_comments set removed_at = now() - interval '20 days' where id = $1`, parent)
	left := func(mark, want string) {
		t.Helper()
		if card := adminReportOf(t, w.page("/admin/removed"), mark); !strings.Contains(card, `</span></p><p class="by">`+want+" · Ref: ") {
			t.Fatalf("the Removed tab's card holding %q, want %s: %s", mark, want, card)
		}
	}
	for _, mark := range []string{"<h3>Dune</h3>", "First words", "Rude words"} {
		left(mark, "5 days left")
	}

	refused := func(notice, words string) {
		t.Helper()
		if got := w.restore("comment", reply, cw); got != "/admin/removed?n="+notice {
			t.Fatalf("bring the reply back: %q, want %s", got, notice)
		}
		if tab := w.page("/admin/removed?n=" + notice); !strings.Contains(tab, `<div class="note">`+words+`</div>`) ||
			!strings.Contains(tab, `<p class="text quote">Rude words</p>`) {
			t.Fatalf("the Removed tab, want %q and the reply still on it: %s", words, tab)
		}
		if w.one(`select (removed_at is not null)::text from post_comments where id = $1`, reply) != "true" {
			t.Fatal("a refused Bring it back brought the reply back")
		}
		if got := loggedActions(t, w.pool, cw.id); !slices.Equal(got, []string{"removed comment " + reply}) {
			t.Fatalf("a refused Bring it back logged: %v", got)
		}
		if got := w.decisionIDs(cw); !slices.Equal(got, []string{"comment_removed:" + removal.ID}) {
			t.Fatalf("a refused Bring it back reached the writer's bell: %v", got)
		}
	}
	refused("postfirst", "Bring back its post first.")
	if got := w.restore("post", post, b); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the post back: %q", got)
	}
	left("First words", "10 days left")
	left("Rude words", "10 days left")
	refused("parentfirst", "Bring back the comment it answers first.")
	if got := w.restore("comment", parent, qw); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the comment it answers back: %q", got)
	}
	if got := w.restore("comment", reply, cw); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the reply back last: %q", got)
	}
	if ids, _ := w.commentIDs(v, post); !slices.Equal(ids, []string{parent, reply}) {
		t.Fatalf("with all three back, the post's comments: %v", ids)
	}
	if got := loggedActions(t, w.pool, cw.id); !slices.Equal(got, []string{"removed comment " + reply, "restored comment " + reply}) {
		t.Fatalf("the writer's log: %v", got)
	}
}

// A removed comment goes off the Removed tab, and out of its number, at the
// moment its removed post, or the removed comment it answers, passes its 30
// days: that one can't come back, so neither can it, and the hourly purge
// takes it with them. An hour short it is still listed with 1 day left. A
// Bring it back from a tab drawn before the line says it is gone, never to
// bring back its post first.
func TestAdminModerationIntegration_ACommentLeavesTheRemovedTabWithWhatItIsUnder(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("aw_bea", "aw_cal", "aw_dot", "aw_eli")
	b, cw, qw, rw := ps[0], ps[1], ps[2], ps[3]
	for _, p := range ps[1:] {
		w.followAccepted(p, b)
	}
	w.followAccepted(rw, qw)
	const day = "2026-10-08"
	post := w.post(b, day)
	other := w.post(qw, day)
	w.post(cw, day)
	w.post(rw, day)
	comment := w.reply(cw, post, "Rude words", "")["id"].(string)
	parent := w.reply(qw, other, "First words", "")["id"].(string)
	reply := w.reply(rw, other, "Mean reply", parent)["id"].(string)
	w.removeNow("comment", comment, cw)
	w.removeNow("comment", reply, rw)
	w.removeNow("comment", parent, qw)
	tabs := func(want int) {
		t.Helper()
		if list := w.page("/admin"); !strings.Contains(list, "Removed<span class=\"n\">"+strconv.Itoa(want)+"</span>") {
			t.Fatalf("the Reports tab's Removed number, want %d: %s", want, list)
		}
	}

	// The comment under the post: an hour short of the post's 30 days it is
	// listed, with the post's day left; at the 30 days it is not.
	w.removeNow("post", post, b)
	w.exec(`update entries set removed_at = now() - interval '30 days' + interval '1 hour' where id = $1`, post)
	if card := adminReportOf(t, w.page("/admin/removed"), "Rude words"); !strings.Contains(card, `</span></p><p class="by">1 day left · Ref: `) {
		t.Fatalf("an hour short of the post's 30 days, want the comment with 1 day left: %s", card)
	}
	tabs(4)
	w.exec(`update entries set removed_at = now() - interval '30 days' where id = $1`, post)
	if tab := w.page("/admin/removed"); strings.Contains(tab, "Rude words") || strings.Contains(tab, "<h3>Dune</h3>") ||
		strings.Contains(tab, `<p class="by">0 days left`) || !strings.Contains(tab, "Removed<span class=\"n\">2</span>") {
		t.Fatalf("at the post's 30 days the tab still offers the post or its comment: %s", tab)
	}
	tabs(2)
	if got := w.restoreAnyway("comment", comment, cw); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back on the comment at its post's 30 days: %q, want gone", got)
	}

	// The reply, on another post, under the comment it answers: the same.
	w.exec(`update post_comments set removed_at = now() - interval '30 days' + interval '1 hour' where id = $1`, parent)
	if card := adminReportOf(t, w.page("/admin/removed"), "Mean reply"); !strings.Contains(card, `</span></p><p class="by">1 day left · Ref: `) {
		t.Fatalf("an hour short of its parent's 30 days, want the reply with 1 day left: %s", card)
	}
	w.exec(`update post_comments set removed_at = now() - interval '30 days' where id = $1`, parent)
	if tab := w.page("/admin/removed"); !strings.Contains(tab, "Nothing removed in the last 30 days.") ||
		!strings.Contains(tab, "Removed<span class=\"n\">0</span>") {
		t.Fatalf("at its parent's 30 days the tab still offers the reply: %s", tab)
	}
	tabs(0)
	if got := w.restoreAnyway("comment", reply, rw); got != "/admin/removed?n=gone" {
		t.Fatalf("Bring it back on the reply at its parent's 30 days: %q, want gone", got)
	}
	for _, id := range []string{comment, reply} {
		if w.one(`select (removed_at is not null)::text from post_comments where id = $1`, id) != "true" {
			t.Fatalf("a refused Bring it back brought %s back", id)
		}
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
	if !strings.Contains(hiddenCard, `<div class="type"><p class="kind">Post</p></div>`) ||
		!strings.Contains(hiddenCard, `<p class="reasons">Spam</p>`+"\n"+`<p class="flag"><span class="tag">Hidden</span> Only the owner sees it for now.</p>`) ||
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

// A comment counts as shown again only when nothing above it still keeps it
// from everyone. Dismissing its reports while its post, or the comment it
// answers, is hidden under review closes them and says only that: nothing is
// logged and its writer hears nothing. Once the post's own reports are
// dismissed, the comment shows again with it, and only the post's owner is
// told (accepted, 2026-11-18).
func TestAdminModerationIntegration_ACommentUnderSomethingHiddenIsNotShownAgain(t *testing.T) {
	w := newAdminModWorld(t)
	owners := w.people("ah_bob", "ah_olga")
	b, o := owners[0], owners[1]
	others := w.people("ah_wren", "ah_quin", "ah_rory", "ah_vera", "ah_r1", "ah_r2", "ah_r3")
	cw, qw, rw, v, r := others[0], others[1], others[2], others[3], others[4:]
	for _, p := range others {
		w.followAccepted(p, b)
		w.followAccepted(p, o)
	}
	const day = "2026-10-08"
	post, oPost := w.post(b, day), w.post(o, day)
	for _, p := range others {
		w.post(p, day)
	}
	comment := w.reply(cw, post, "Rude words", "")["id"].(string)
	parent := w.reply(qw, oPost, "First words", "")["id"].(string)
	reply := w.reply(rw, oPost, "Other words", parent)["id"].(string)
	// A reporter loses what they report and all under it, so each reports
	// from the bottom up.
	for _, p := range r {
		for _, rep := range [][3]string{{"comments", comment, "spam"}, {"posts", post, "hate"},
			{"comments", reply, "harassment"}, {"comments", parent, "other"}} {
			w.ok(p, http.MethodPost, "/v1/"+rep[0]+"/"+rep[1]+"/report", map[string]any{"reason": rep[2]})
		}
	}
	if w.sees(v, post) {
		t.Fatal("three reports did not hide the post")
	}

	for _, d := range []struct {
		writer feedPerson
		mark   string
	}{{cw, "Spam"}, {rw, "Bullying or harassment"}} {
		if got := w.act("dismiss", "comment", d.writer, d.mark); got != "/admin/people/"+d.writer.id+"?n=dismissed" {
			t.Fatalf("dismiss the comment of %s: %q", d.writer.name, got)
		}
		if got := loggedActions(t, w.pool, d.writer.id); len(got) != 0 {
			t.Fatalf("the comment of %s, still under something hidden, logged: %v", d.writer.name, got)
		}
		if got := w.decisionIDs(d.writer); len(got) != 0 {
			t.Fatalf("%s heard their comment is back: %v", d.writer.name, got)
		}
	}
	if ids, _ := w.commentIDs(v, oPost); len(ids) != 0 {
		t.Fatalf("the reply under the hidden comment shows: %v", ids)
	}

	if got := w.act("dismiss", "post", b, "Hate speech"); got != "/admin/people/"+b.id+"?n=shownagain" {
		t.Fatalf("dismiss the post: %q", got)
	}
	if ids, _ := w.commentIDs(v, post); !slices.Equal(ids, []string{comment}) {
		t.Fatalf("with the post back, its comments: %v", ids)
	}
	if got := w.decisionIDs(cw); len(got) != 0 {
		t.Fatalf("the comment came back with its post, and its writer heard: %v", got)
	}
}

// closeAsDisabledQ is a Disable's own close of the post reports about the
// person (AdminRepo.ResolvePerson).
const closeAsDisabledQ = `update post_reports set resolved_at = $3, resolution = $2 where owner_id = $1 and resolved_at is null`

// A Disable closes the reports about a person without waiting for a Dismiss
// that holds their post, so its close can land between the Dismiss's read of
// whether the post is hidden and the Dismiss's own close. That Dismiss
// closed nothing, so it logs nothing and tells nobody the post is back. The
// test holds the Disable's close in a transaction of its own, lets the
// Dismiss wait on those reports, and then lets the close land.
func TestAdminModerationIntegration_ADismissThatADisableBeatLogsNothing(t *testing.T) {
	w := newAdminModWorld(t)
	ctx := context.Background()
	ps := w.people("dr_bob", "dr_r1", "dr_r2", "dr_r3")
	b, r := ps[0], ps[1:]
	for _, p := range r {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	post := w.post(b, day)
	for _, p := range r {
		w.post(p, day)
	}
	for _, p := range r {
		w.ok(p, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "spam"})
	}
	var ids []uuid.UUID
	for _, s := range w.cardForm("dismiss", "post", b, "Spam")["id"] {
		ids = append(ids, uuid.MustParse(s))
	}
	if len(ids) != 3 {
		t.Fatalf("the card's ids: %v, want its three reports", ids)
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if tag, err := tx.Exec(ctx, closeAsDisabledQ, b.id, "disabled", time.Now().UTC()); err != nil || tag.RowsAffected() != 3 {
		t.Fatalf("the Disable's close: %v, %v", tag, err)
	}
	type outcome struct {
		closed int
		shown  bool
		err    error
	}
	dismissed := make(chan outcome, 1)
	go func() {
		closed, shown, err := postgres.NewAdminRepo(w.pool).DismissItem(ctx, "post", ids, nil, uuid.MustParse(b.id), time.Now().UTC(), w.clk.NowFor(b.id))
		dismissed <- outcome{closed, shown, err}
	}()
	waitForARowLock(t, w.pool)
	var waiting string
	if err := w.pool.QueryRow(ctx, `select query from pg_stat_activity
		where datname = current_database() and wait_event_type = 'Lock'`).Scan(&waiting); err != nil ||
		!strings.Contains(waiting, "update post_reports set resolved_at = $3, resolution = 'dismissed'") {
		t.Fatalf("the statement waiting: %q (%v), want the Dismiss's close", waiting, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-dismissed; got.closed != 0 || got.shown || got.err != nil {
		t.Fatalf("the Dismiss the Disable beat: %+v, want nothing closed and nothing shown again", got)
	}
	if got := loggedActions(t, w.pool, b.id); len(got) != 0 {
		t.Fatalf("the Dismiss that closed nothing logged: %v", got)
	}
	if got := w.decisionIDs(b); len(got) != 0 {
		t.Fatalf("the owner heard their post is back: %v", got)
	}
	if got := w.one(`select string_agg(resolution, ' ') from post_reports where entry_id = $1`, post); got != "disabled disabled disabled" {
		t.Fatalf("the reports: %s, want all three closed by the Disable", got)
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
	log := historyOf(t, page)
	var labels []string
	for _, part := range strings.Split(log, "<strong>")[1:] {
		if label := part[:strings.Index(part, "</strong>")]; !strings.Contains(label, " reported for ") {
			labels = append(labels, label)
		}
	}
	if want := []string{"Post removed", "Post brought back", "Comment removed", "Post removed"}; !slices.Equal(labels, want) {
		t.Fatalf("the History's decisions, newest first: %v, want %v", labels, want)
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

// The person page shows a person's posts from the database while a report
// about their account is open, or while it is disabled, as they are now: the
// newest first, each with its spin's category, its day and every photo in
// order, and a tag when it is reported, hidden or removed. Never one they
// archived. A report about one post shows none of the others, and a Dismiss
// of the account report hides them again (developer, 2026-11-18).
func TestAdminModerationIntegration_AnAccountReportShowsTheirPosts(t *testing.T) {
	w := newAdminModWorld(t)
	ps := w.people("ap_bob", "ap_r1", "ap_r2", "ap_r3")
	b, r1, r2, r3 := ps[0], ps[1], ps[2], ps[3]
	for _, p := range []feedPerson{r1, r2, r3} {
		w.followAccepted(p, b)
	}
	older := w.post(b, "2026-10-06")
	archived := w.post(b, "2026-10-07")
	const day = "2026-10-08"
	newest := w.post(b, day)
	for _, p := range []feedPerson{r1, r2, r3} {
		w.post(p, day)
	}
	w.ok(b, http.MethodPost, "/v1/entries/"+archived+"/archive", nil)
	category := func(entry string) string {
		return w.one(`select s.category_name from entries e join spins s on s.id = e.spin_id where e.id = $1`, entry)
	}
	// shown is each post on B's page as its day line and its tag ("" for
	// none), the page's order.
	shown := func() []string {
		t.Helper()
		var out []string
		for _, post := range postsOf(t, w.page("/admin/people/"+b.id)) {
			line := post[strings.Index(post, `<p class="day">`):]
			line = line[len(`<p class="day">`):strings.Index(line, "</p>")]
			if at := strings.Index(post, `<span class="tag">`); at >= 0 {
				tag := post[at+len(`<span class="tag">`):]
				line += " " + tag[:strings.Index(tag, "</span>")]
			}
			out = append(out, line)
		}
		return out
	}

	w.ok(r1, http.MethodPost, "/v1/posts/"+newest+"/report", map[string]any{"reason": "spam"})
	if page := w.page("/admin/people/" + b.id); postsOf(t, page) != nil || !strings.Contains(page, "Their posts show here while a report about the account is open") {
		t.Fatalf("a post report alone shows their posts: %s", page)
	}

	w.ok(r2, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "impersonation"})
	newestLine, olderLine := category(newest)+" · Oct 8, 2026", category(older)+" · Oct 6, 2026"
	if got := shown(); !slices.Equal(got, []string{newestLine + " Reported", olderLine}) {
		t.Fatalf("with the account reported: %v, want the newest (reported), then the older, never the archived", got)
	}
	cover := postsOf(t, w.page("/admin/people/"+b.id))[0]
	for _, key := range postPhotoKeys(t, w.pool, newest) {
		if !strings.Contains(cover, `<img src="`+fakePhotoLink(key)+`"`) {
			t.Fatalf("the newest post lacks its photo %s: %s", key, cover)
		}
	}

	w.ok(r2, http.MethodPost, "/v1/posts/"+newest+"/report", map[string]any{"reason": "spam"})
	w.ok(r3, http.MethodPost, "/v1/posts/"+newest+"/report", map[string]any{"reason": "spam"})
	if got := shown(); !slices.Equal(got, []string{newestLine + " Hidden", olderLine}) {
		t.Fatalf("with 3 reports on the newest: %v", got)
	}
	w.act("remove", "post", b, "Spam")
	if got := shown(); !slices.Equal(got, []string{newestLine + " Removed", olderLine}) {
		t.Fatalf("with the newest removed: %v", got)
	}

	w.act("dismiss", "person", b, "Pretending to be someone else")
	if got := shown(); got != nil {
		t.Fatalf("after the account report's Dismiss: %v, want none", got)
	}
	page := w.page("/admin/people/" + b.id)
	if rec := adminDo(t, w.feedWorld, http.MethodPost, "/admin/people/"+b.id+"/disable", w.cookie,
		url.Values{"csrf": {csrfOf(t, page)}}, ""); rec.Header().Get("Location") != "/admin/people/"+b.id+"?n=disabled" {
		t.Fatalf("disable: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if got := shown(); !slices.Equal(got, []string{newestLine + " Removed", olderLine}) {
		t.Fatalf("disabled: %v", got)
	}
}
