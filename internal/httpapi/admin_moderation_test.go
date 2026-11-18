package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

// adminWhen is a time as the admin page writes it.
func adminWhen(t time.Time) string { return t.Local().Format("Jan 2, 2006 3:04 PM") }

// The Removed tab lists what was removed in the last 30 days as the store
// reads it, each card with what it was (the post's title, day, text and
// photos, signed; the comment's text, escaped, and photo), why (the reason
// the admin gave, or for a removal from before the Reason list, the
// reports' reasons), when, the days left, the removal's ref and Bring it
// back, which comes back to this tab. It does not refresh itself, never
// shows Removed before, and every tab carries its count.
func TestAdminRemovedTab_ListsWhatWasRemovedWithBringItBack(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	disabledAt := cardAt(8, 0)
	mira := admin.Person{ID: uuid.New(), Username: "mira_cooks", Name: "Mira", RemovedBefore: 3}
	dev := admin.Person{ID: uuid.New(), Username: "dev_runs", Name: "Dev", DisabledAt: &disabledAt}
	post, comment := uuid.New(), uuid.New()
	removedAt := cardAt(9, 30)
	r.repo.removed = []admin.RemovedItem{
		{Kind: admin.KindComment, ID: comment, Owner: dev, Body: "<b>rude</b>", PhotoKey: "c1", RemovedAt: removedAt,
			DaysLeft: 1, Reasons: []string{"harassment"}, Reason: "hate", Ref: "0a1b2c3d"},
		{Kind: admin.KindPost, ID: post, Owner: mira, Title: "Ramen", LocalDate: "2026-11-15", Note: "Slurp", PhotoKeys: []string{"k1", "k2"},
			RemovedAt: removedAt, DaysLeft: 29, Reasons: []string{"violence", "spam"}, Ref: "9f8e7d6c"},
	}
	rec := adminDo(t, r.w, http.MethodGet, "/admin/removed", cookie, nil, "")
	page := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(page, `http-equiv="refresh"`) || strings.Count(page, `<article class="card">`) != 2 {
		t.Fatalf("the Removed tab: %d, want two cards and no refresh: %s", rec.Code, page)
	}
	socialNoScript(t, "the Removed tab", page)
	restoreForm := func(kind string, id uuid.UUID, owner admin.Person) string {
		return `<form method="post" action="/admin/removed/restore"><input type="hidden" name="csrf" value="` + r.h.sign("csrf", cookie) +
			`"><input type="hidden" name="kind" value="` + kind + `"><input type="hidden" name="id" value="` + id.String() +
			`"><input type="hidden" name="person" value="` + owner.ID.String() +
			`"><input type="hidden" name="from" value="removed"><button class="strong">Bring it back</button></form>`
	}
	postCard, commentCard := adminReportOf(t, page, "<h3>Ramen</h3>"), adminReportOf(t, page, "&lt;b&gt;rude&lt;/b&gt;")
	for card, wants := range map[string][]string{
		postCard: {
			`Violence or threats · Spam</p><p class="kind">Post</p>`,
			`<p class="by">Removed <span class="when">` + adminWhen(removedAt) + `</span> · 29 days left · Ref: 9f8e7d6c</p>`,
			`<p class="day">Nov 15, 2026</p><h3>Ramen</h3><p class="text">Slurp</p>`,
			`<a href="` + fakePhotoLink("k1") + `" target="_blank" rel="noopener"><img class="thumb" src="` + fakePhotoLink("k1") + `"`,
			`<img class="thumb" src="` + fakePhotoLink("k2") + `"`,
			restoreForm(admin.KindPost, post, mira),
			`<a class="button push" href="/admin/people/` + mira.ID.String() + `?from=removed">Open profile</a>`,
		},
		commentCard: {
			`<p class="reasons">Hate speech</p><p class="kind">Comment</p>`,
			` · 1 day left · Ref: 0a1b2c3d</p>`,
			`<p class="text quote">&lt;b&gt;rude&lt;/b&gt;</p>`,
			`<img class="thumb" src="` + fakePhotoLink("c1") + `"`,
			`<strong>@dev_runs</strong> <span class="muted">Dev</span></p></div><span class="tag">Disabled</span></div>`,
			restoreForm(admin.KindComment, comment, dev),
		},
	} {
		for _, want := range wants {
			if !strings.Contains(card, want) {
				t.Errorf("a Removed card lacks %q", want)
			}
		}
	}
	for _, want := range []string{
		`<title>Removed · RNDMRoll admin</title>`,
		`class="on" aria-current="page">Removed<span class="n">2</span></a></nav>`,
		`<p class="lede">Hidden from everyone, the owner too. Deleted for good 30 days after removal.</p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the Removed tab lacks %q", want)
		}
	}
	if strings.Contains(page, "Removed before") || strings.Index(page, "rude") > strings.Index(page, "Ramen") {
		t.Errorf("the Removed tab shows Removed before, or not in the store's order")
	}
	if strings.Contains(commentCard, "Bullying or harassment") {
		t.Errorf("the comment's card shows the reports' reason beside the one the admin gave")
	}
	if t.Failed() {
		t.Log(page)
	}

	// The other tabs count it, and a removal with no log row has no ref.
	if list := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String(); !strings.Contains(list, `<a href="/admin/removed">Removed<span class="n">2</span></a></nav>`) {
		t.Fatalf("the Reports tab's count of removed: %s", list)
	}
	r.repo.removed = r.repo.removed[1:]
	r.repo.removed[0].Ref = ""
	if page := adminDo(t, r.w, http.MethodGet, "/admin/removed", cookie, nil, "").Body.String(); strings.Contains(page, "Ref:") ||
		!strings.Contains(page, " · 29 days left</p>") {
		t.Fatalf("a removal with no ref: %s", page)
	}
	r.repo.removed = nil
	if page := adminDo(t, r.w, http.MethodGet, "/admin/removed", cookie, nil, "").Body.String(); !strings.Contains(page, `<p class="empty">Nothing removed in the last 30 days.</p>`) ||
		!strings.Contains(page, `Removed<span class="n">0</span>`) || strings.Contains(page, "lede") {
		t.Fatalf("the empty tab: %s", page)
	}
}

// Bring it back is one call with the card's kind, target and owner, logged
// on the owner's own clock, and goes back to the Removed tab (or the person's
// page), which says it worked, or that it was gone, or failed. A form the
// page never draws, an old page's token and another site do nothing.
func TestAdminRestore_BringsItBackAndGoesBackToTheTab(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	owner, target := uuid.New(), uuid.New()
	restore := func(form url.Values) string {
		t.Helper()
		if !form.Has("csrf") {
			form.Set("csrf", r.h.sign("csrf", cookie))
		}
		rec := adminDo(t, r.w, http.MethodPost, "/admin/removed/restore", cookie, form, "")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("bring it back: %d %s", rec.Code, rec.Body.String())
		}
		return rec.Header().Get("Location")
	}
	form := func(kind string) url.Values {
		return url.Values{"kind": {kind}, "id": {target.String()}, "person": {owner.String()}, "from": {"removed"}}
	}
	if got := restore(form(admin.KindPost)); got != "/admin/removed?n=restored" || len(r.repo.restores) != 1 {
		t.Fatalf("bring a post back: %q, %d calls", got, len(r.repo.restores))
	}
	if got := r.repo.restores[0]; got.kind != admin.KindPost || got.target != target || got.person != owner ||
		time.Since(got.loggedAt) < 0 || time.Since(got.loggedAt) > time.Minute {
		t.Fatalf("the call: %+v, want the post, its owner and now (no clock of theirs)", got)
	}
	if page := adminDo(t, r.w, http.MethodGet, "/admin/removed?n=restored", cookie, nil, "").Body.String(); !strings.Contains(page, `<div class="note">Brought back. They were told.</div>`) {
		t.Fatalf("the tab lacks the notice: %s", page)
	}
	if _, err := r.spin.SetTimeZone(context.Background(), owner.String(), "America/New_York"); err != nil {
		t.Fatal(err)
	}
	restore(form(admin.KindComment))
	if got := r.repo.restores[1]; got.kind != admin.KindComment || !got.loggedAt.Equal(adminRigNow) {
		t.Fatalf("a comment brought back: %+v, want it logged at the owner's %v", got, adminRigNow)
	}
	noFrom := form(admin.KindPost)
	noFrom.Del("from")
	if got := restore(noFrom); got != "/admin/people/"+owner.String()+"?n=restored" {
		t.Fatalf("from no tab: %q, want the person's page", got)
	}

	r.repo.restoreErr = admin.ErrAlreadyDone
	if got := restore(form(admin.KindPost)); got != "/admin/removed?n=restored" {
		t.Fatalf("brought back already (a double click): %q", got)
	}
	r.repo.restoreErr = admin.ErrNotFound
	if got := restore(form(admin.KindPost)); got != "/admin/removed?n=gone" {
		t.Fatalf("past the 30 days, never removed or deleted: %q", got)
	}
	for err, words := range map[error]string{admin.ErrPostRemoved: "Bring back its post first.",
		admin.ErrParentRemoved: "Bring back the comment it answers first."} {
		r.repo.restoreErr = err
		got := restore(form(admin.KindComment))
		if page := adminDo(t, r.w, http.MethodGet, got, cookie, nil, "").Body.String(); !strings.HasPrefix(got, "/admin/removed?n=") ||
			!strings.Contains(page, `<div class="note">`+words+`</div>`) {
			t.Fatalf("a comment under something still removed (%v): %q, want the tab saying %q", err, got, words)
		}
	}
	r.repo.restoreErr = errors.New("conn closed")
	if got := restore(form(admin.KindPost)); got != "/admin/removed?n=failed" {
		t.Fatalf("a failure: %q", got)
	}
	r.repo.restoreErr = nil

	calls := len(r.repo.restores)
	for name, f := range map[string]url.Values{
		"a person":     {"kind": {admin.KindPerson}, "id": {target.String()}, "person": {owner.String()}, "from": {"removed"}},
		"a bad kind":   {"kind": {"entry"}, "id": {target.String()}, "person": {owner.String()}, "from": {"removed"}},
		"a bad id":     {"kind": {admin.KindPost}, "id": {"nope"}, "person": {owner.String()}, "from": {"removed"}},
		"no person":    {"kind": {admin.KindPost}, "id": {target.String()}, "from": {"removed"}},
		"no target id": {"kind": {admin.KindPost}, "person": {owner.String()}, "from": {"removed"}},
	} {
		if got := restore(f); got != "/admin/removed" {
			t.Errorf("%s: %q, want the tab", name, got)
		}
	}
	stale := form(admin.KindPost)
	stale.Set("csrf", r.h.sign("csrf", r.session(time.Now().Add(-time.Minute))))
	if got := restore(stale); got != "/admin?n=stale" {
		t.Fatalf("an older page's token: %q", got)
	}
	cross := form(admin.KindPost)
	cross.Set("csrf", r.h.sign("csrf", cookie))
	if rec := adminDo(t, r.w, http.MethodPost, "/admin/removed/restore", cookie, cross, "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("another site's Origin: %d", rec.Code)
	}
	if len(r.repo.restores) != calls {
		t.Fatalf("a refused form brought back %d", len(r.repo.restores)-calls)
	}
}

// A post or comment hidden while under review has the Hidden tag, and its
// card comes first on the Reports tab, ahead even of violence. The store
// sets the flag on each open report; one flagged report is enough for its
// card. A closed card never has the tag.
func TestAdminList_HiddenCardsComeFirstWithTheirTag(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	mira := admin.Person{ID: uuid.New(), Username: "mira_cooks", Name: "Mira"}
	dev := admin.Person{ID: uuid.New(), Username: "dev_runs", Name: "Dev"}
	hiddenComment, violentPost := uuid.New(), uuid.New()
	r.repo.openReports = []admin.OpenReport{
		{Owner: dev, Report: admin.Report{Kind: admin.KindPost, ID: uuid.New(), Reporter: "lena_k", Reason: "violence", CreatedAt: cardAt(14, 0),
			EntryID: &violentPost, Title: "Drive", TitleNow: "Drive", LocalDate: "2026-11-16"}},
		{Owner: mira, Report: admin.Report{Kind: admin.KindComment, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(13, 0),
			CommentID: &hiddenComment, Body: "buy now", Hidden: true}},
		{Owner: mira, Report: admin.Report{Kind: admin.KindComment, ID: uuid.New(), Reporter: "jon_p", Reason: "spam", CreatedAt: cardAt(12, 0),
			CommentID: &hiddenComment, Body: "buy now"}},
	}
	page := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	hidden, violent := adminReportOf(t, page, "buy now"), adminReportOf(t, page, "<h3>Drive</h3>")
	if !strings.Contains(hidden, `Spam</p><p class="kind">Comment</p></div>`+"\n"+
		`<p class="flag"><span class="tag">Hidden</span> Only the owner sees it for now.</p>`+"\n"+`<p class="by">Reported by @tess_r and @jon_p · `) {
		t.Fatalf("the hidden card, want its tag between the reasons and who reported it: %s", hidden)
	}
	if strings.Contains(violent, "Hidden") || strings.Count(page, `<span class="tag">Hidden</span>`) != 1 ||
		strings.Index(page, "buy now") > strings.Index(page, "<h3>Drive</h3>") {
		t.Fatalf("want only the hidden card tagged, and it first: %s", page)
	}

	// Not flagged: back to the most urgent first.
	r.repo.openReports[1].Hidden = false
	if page := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String(); strings.Contains(page, "Hidden") ||
		strings.Index(page, "buy now") < strings.Index(page, "<h3>Drive</h3>") {
		t.Fatalf("with no hidden card: %s", page)
	}

	// A closed report keeps the flag the store read, and its card has no tag.
	closedAt := cardAt(15, 0)
	r.repo.reports = []admin.Report{{Kind: admin.KindComment, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(13, 0),
		CommentID: &hiddenComment, Body: "buy now", Hidden: true, ResolvedAt: &closedAt, Resolution: admin.Dismissed}}
	if page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+mira.ID.String(), cookie, nil, "").Body.String(); strings.Contains(page, `class="flag"`) ||
		!strings.Contains(page, "buy now") {
		t.Fatalf("a closed card with the flag: %s", page)
	}
	if open, closed := groupReports(mira, r.repo.reports); len(open) != 0 || len(closed) != 1 || closed[0].Hidden {
		t.Fatalf("groupReports of a closed flagged report: %+v %+v", open, closed)
	}
}

// Removed before shows on the Reports tab in a card's owner row, and in a
// person card's body, only when it is more than 0; on the person page once,
// in the profile card.
func TestAdminCards_RemovedBeforeShowsOnlyWhenThereIsSome(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	mira := admin.Person{ID: uuid.New(), Username: "mira_cooks", Name: "Mira", RemovedBefore: 2}
	dev := admin.Person{ID: uuid.New(), Username: "dev_runs", Name: "Dev"}
	r.repo.openReports = []admin.OpenReport{
		{Owner: mira, Report: admin.Report{Kind: admin.KindPost, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(15, 0),
			EntryID: uuidPtr(uuid.New()), Title: "Ramen", TitleNow: "Ramen", LocalDate: "2026-11-15"}},
		{Owner: mira, Report: admin.Report{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "jon_p", Reason: "impersonation", CreatedAt: cardAt(14, 0)}},
		{Owner: dev, Report: admin.Report{Kind: admin.KindComment, ID: uuid.New(), Reporter: "lena_k", Reason: "hate", CreatedAt: cardAt(13, 0),
			CommentID: uuidPtr(uuid.New()), Body: "rude"}},
	}
	list := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if card := adminReportOf(t, list, "<h3>Ramen</h3>"); !strings.Contains(card,
		`<strong>@mira_cooks</strong> <span class="muted">Mira</span></p></div><span class="before">Removed before: 2</span></div>`) {
		t.Fatalf("the post card's owner row: %s", card)
	}
	if card := adminReportOf(t, list, "Pretending to be someone else"); !strings.Contains(card, `<p class="by">Removed before: 2</p></div>`) {
		t.Fatalf("the person card: %s", card)
	}
	if card := adminReportOf(t, list, "Hate speech"); strings.Contains(card, "Removed before") {
		t.Fatalf("nothing removed, yet the card says so: %s", card)
	}
	if n := strings.Count(list, "Removed before"); n != 2 {
		t.Fatalf("Removed before %d times on the Reports tab, want 2", n)
	}

	r.repo.removedBefore = 3
	page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+mira.ID.String(), cookie, nil, "").Body.String()
	if strings.Count(page, "Removed before") != 1 ||
		!strings.Contains(page, `</p></div></div>`+"\n"+`<p class="by">Joined <span class="when">`) ||
		!strings.Contains(page, `</span></p>`+"\n"+`<p class="by">Removed before: 3</p>`) ||
		strings.Index(page, "Removed before: 3") > strings.Index(page, "</section>") {
		t.Fatalf("the person page, want Removed before: 3 once, in the profile card: %s", page)
	}
	r.repo.removedBefore = 0
	if page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+mira.ID.String(), cookie, nil, "").Body.String(); strings.Contains(page, "Removed before") {
		t.Fatalf("nothing removed, yet the person page says so: %s", page)
	}
}

// The person page lists every decision in the log in the store's order (the
// newest first), with when, on its own list after the open reports and the
// posts: no new card, no form, nothing a card's check could match. A removal says its ref
// and the reason the admin gave (for one from before the Reason list, the
// reports' reasons), and what was decided about shows as it was then,
// escaped. With no decision, no History.
func TestAdminPerson_HistoryListsEveryDecision(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person := uuid.New()
	picked := uuid.MustParse("9c8d7e6f-0000-4000-8000-000000000003")
	removal := uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000001")
	photoOnly := uuid.MustParse("5e6f7a8b-0000-4000-8000-000000000002")
	r.repo.events = []admin.Event{
		{ID: picked, Item: admin.KindPost, Action: admin.LogRemoved, Snapshot: "Pho", Reasons: []string{"spam"}, Reason: "hate", At: cardAt(16, 0)},
		{ID: uuid.New(), Item: admin.ItemAccount, Action: admin.LogEnabled, ItemID: person, At: cardAt(15, 0)},
		{ID: uuid.New(), Item: admin.ItemAccount, Action: admin.LogDisabled, ItemID: person, At: cardAt(14, 0)},
		{ID: uuid.New(), Item: admin.KindComment, Action: admin.LogShownAgain, Snapshot: "<i>fine</i>", At: cardAt(13, 0)},
		{ID: uuid.New(), Item: admin.KindPost, Action: admin.LogRestored, Snapshot: "Ramen", At: cardAt(12, 0)},
		{ID: removal, Item: admin.KindPost, Action: admin.LogRemoved, Snapshot: "Ramen", Reasons: []string{"violence", "spam"}, At: cardAt(11, 0)},
		{ID: photoOnly, Item: admin.KindComment, Action: admin.LogRemoved, At: cardAt(10, 0)},
	}
	r.repo.reports = []admin.Report{{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(9, 0)}}
	page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
	start := strings.Index(page, "<h2>History</h2>")
	if start < 0 || strings.Index(page, "<h2>Posts</h2>") > start || strings.Index(page, `<article class="card`) > start ||
		strings.Count(page, `<article class="card`) != 1 || !strings.Contains(page[start:], "</ol>\n") {
		t.Fatalf("want the History after the one open report and the posts: %s", page)
	}
	log := page[start : start+strings.Index(page[start:], "</ol>\n")+len("</ol>\n")]
	want := `<h2>History</h2>` + "\n" + `<ol class="history">` +
		`<li><p><strong>Post removed</strong> <span class="muted when">` + adminWhen(cardAt(16, 0)) + `</span></p><p class="by">Ref: 9c8d7e6f · Hate speech</p><p class="text quote">Pho</p></li>` +
		`<li><p><strong>Account enabled</strong> <span class="muted when">` + adminWhen(cardAt(15, 0)) + `</span></p></li>` +
		`<li><p><strong>Account disabled</strong> <span class="muted when">` + adminWhen(cardAt(14, 0)) + `</span></p></li>` +
		`<li><p><strong>Comment shown again</strong> <span class="muted when">` + adminWhen(cardAt(13, 0)) + `</span></p><p class="text quote">&lt;i&gt;fine&lt;/i&gt;</p></li>` +
		`<li><p><strong>Post brought back</strong> <span class="muted when">` + adminWhen(cardAt(12, 0)) + `</span></p><p class="text quote">Ramen</p></li>` +
		`<li><p><strong>Post removed</strong> <span class="muted when">` + adminWhen(cardAt(11, 0)) + `</span></p><p class="by">Ref: 1a2b3c4d · Violence or threats, Spam</p><p class="text quote">Ramen</p></li>` +
		`<li><p><strong>Comment removed</strong> <span class="muted when">` + adminWhen(cardAt(10, 0)) + `</span></p><p class="by">Ref: 5e6f7a8b</p></li>` +
		`</ol>` + "\n"
	if log != want {
		t.Fatalf("the History:\n%s\nwant\n%s", log, want)
	}
	socialNoScript(t, "the person page", page)

	r.repo.events = nil
	if page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String(); strings.Contains(page, "History") {
		t.Fatalf("no decision, yet a History: %s", page)
	}
}

// Remove's Reason list is the seven report reasons, the most serious first,
// and a card's list starts on the most serious reason it was reported for:
// Something else for a card with none the list has (developer, 2026-11-18).
func TestAdminCards_RemoveStartsOnTheMostSeriousReason(t *testing.T) {
	if want := []string{"violence", "nudity", "harassment", "hate", "impersonation", "spam", "other"}; !slices.Equal(removeReasons, want) {
		t.Fatalf("the Reason list: %v, want %v", removeReasons, want)
	}
	for _, c := range []struct {
		reasons []string
		want    string
	}{
		{[]string{"violence", "spam"}, "violence"},
		{[]string{"hate"}, "hate"},
		{[]string{"other"}, "other"},
		{nil, "other"},
		{[]string{"no_such_reason"}, "other"},
	} {
		if got := (adminItem{Reasons: c.reasons}).RemoveReason(); got != c.want {
			t.Errorf("reported for %v: the list starts on %q, want %q", c.reasons, got, c.want)
		}
	}
}
