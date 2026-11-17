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

// cardAt is a time on 2026-11-16 at hour:minute UTC.
func cardAt(hour, minute int) time.Time { return time.Date(2026, 11, 16, hour, minute, 0, 0, time.UTC) }

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

func TestGroupReports_OneCardPerPostCommentOrAccount(t *testing.T) {
	owner := admin.Person{ID: uuid.New(), Username: "mira_cooks"}
	post, comment := uuid.New(), uuid.New()
	closedAt := cardAt(12, 0)
	reports := []admin.Report{
		{Kind: admin.KindPost, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(15, 0), EntryID: &post, TitleNow: "Ramen"},
		{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "jon_p", Reason: "impersonation", CreatedAt: cardAt(14, 0)},
		{Kind: admin.KindPost, ID: uuid.New(), Reporter: "jon_p", Reason: "violence", CreatedAt: cardAt(9, 0), EntryID: &post},
		{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "tess_r", Reason: "impersonation", CreatedAt: cardAt(8, 0)},
		{Kind: admin.KindComment, ID: uuid.New(), Reporter: "tess_r", Reason: "harassment", CreatedAt: cardAt(7, 0), CommentID: &comment},
		// Two gone posts: each keeps its own card, since neither says which
		// post it was.
		{Kind: admin.KindPost, ID: uuid.New(), Reporter: "a", Reason: "spam", CreatedAt: cardAt(6, 0)},
		{Kind: admin.KindPost, ID: uuid.New(), Reporter: "b", Reason: "spam", CreatedAt: cardAt(5, 0)},
		// A closed report on the same post is a closed card, not part of the
		// open one.
		{Kind: admin.KindPost, ID: uuid.New(), Reporter: "kai", Reason: "spam", CreatedAt: cardAt(4, 0), EntryID: &post,
			ResolvedAt: &closedAt, Resolution: admin.Dismissed},
	}
	open, closed := groupReports(owner, reports)
	if len(open) != 5 || len(closed) != 1 {
		t.Fatalf("open %d, closed %d; want 5 and 1", len(open), len(closed))
	}
	// Violence first; then bullying; then the account with two reports; then
	// the gone posts, the longest waiting first.
	first := open[0]
	if first.Kind != admin.KindPost || len(first.IDs) != 2 || first.Reasons[0] != "violence" || first.Reasons[1] != "spam" ||
		first.ReportedBy() != "@tess_r and @jon_p" || !first.NewestAt.Equal(cardAt(15, 0)) || !first.OldestAt.Equal(cardAt(9, 0)) ||
		first.Report.TitleNow != "Ramen" || first.Owner.Username != "mira_cooks" {
		t.Fatalf("the post card: %+v", first)
	}
	if open[1].Kind != admin.KindComment || open[2].Kind != admin.KindPerson || len(open[2].IDs) != 2 ||
		open[2].ReportedBy() != "@jon_p and @tess_r" {
		t.Fatalf("order: %s, %s (%d reports)", open[1].Kind, open[2].Kind, len(open[2].IDs))
	}
	if !open[3].OldestAt.Equal(cardAt(5, 0)) || !open[4].OldestAt.Equal(cardAt(6, 0)) || len(open[3].IDs) != 1 {
		t.Fatalf("gone posts: %v then %v", open[3].OldestAt, open[4].OldestAt)
	}
	if closed[0].Open || closed[0].Resolution != admin.Dismissed || closed[0].ReportedBy() != "@kai" {
		t.Fatalf("closed card: %+v", closed[0])
	}
}

func TestReportedBy_NamesUpToThree(t *testing.T) {
	for _, c := range []struct {
		who  []string
		want string
	}{
		{[]string{"a"}, "@a"},
		{[]string{"a", "b"}, "@a and @b"},
		{[]string{"a", "b", "c"}, "@a, @b and @c"},
		{[]string{"a", "b", "c", "d", "e"}, "@a, @b, @c and 2 more"},
	} {
		if got := (adminItem{Reporters: c.who}).ReportedBy(); got != c.want {
			t.Errorf("%v: %q, want %q", c.who, got, c.want)
		}
	}
}

// The Reports tab: a card for each reported thing across people, the most
// urgent first, with the owner and Open account; the counts in the tabs and
// in the window title; and a refresh each minute.
func TestAdminList_ShowsEveryReportedThingAsACard(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	mira := admin.Person{ID: uuid.New(), Username: "mira_cooks", Name: "Mira"}
	dev := admin.Person{ID: uuid.New(), Username: "dev_runs", Name: "Dev"}
	post := uuid.New()
	spam1, spam2, violent := uuid.New(), uuid.New(), uuid.New()
	r.repo.openReports = []admin.OpenReport{
		{Owner: mira, Report: admin.Report{Kind: admin.KindPost, ID: spam1, Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(15, 0),
			EntryID: &post, Title: "Ramen", TitleNow: "Ramen", LocalDate: "2026-11-15", PhotoKeys: []string{"k1", "k2"}}},
		{Owner: dev, Report: admin.Report{Kind: admin.KindPost, ID: violent, Reporter: "lena_k", Reason: "violence", CreatedAt: cardAt(14, 0),
			EntryID: uuidPtr(uuid.New()), Title: "Drive", TitleNow: "Drive", LocalDate: "2026-11-16"}},
		{Owner: mira, Report: admin.Report{Kind: admin.KindPost, ID: spam2, Reporter: "jon_p", Reason: "spam", CreatedAt: cardAt(9, 0),
			EntryID: &post, Title: "Ramen", TitleNow: "Ramen", LocalDate: "2026-11-15", PhotoKeys: []string{"k1", "k2"}}},
	}
	r.repo.disabled = []admin.Person{{ID: uuid.New(), Username: "loud_larry"}}
	rec := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "")
	page := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Count(page, `<article class="card`) != 2 {
		t.Fatalf("list: %d, %d cards: %s", rec.Code, strings.Count(page, `<article class="card`), page)
	}
	if strings.Index(page, "Violence or threats") > strings.Index(page, "Reported by @tess_r and @jon_p") {
		t.Fatal("the violence card is not first")
	}
	for _, want := range []string{
		"<title>(2) Reports · RNDMRoll admin</title>",
		`<meta http-equiv="refresh" content="60;url=/admin">`,
		`class="on" aria-current="page">Reports<span class="n">2</span>`,
		`Disabled<span class="n">1</span>`,
		`<strong>@mira_cooks</strong>`,
		`<a class="button push" href="/admin/people/` + mira.ID.String() + `">Open account</a>`,
		// The card's Dismiss closes both of its reports, then comes back here.
		`<input type="hidden" name="id" value="` + spam1.String() + `"><input type="hidden" name="id" value="` + spam2.String() + `">`,
		`<input type="hidden" name="from" value="list"><button>Dismiss</button>`,
		`<button class="strong">Remove post</button>`,
		`<img class="thumb" src="` + fakePhotoLink("k1") + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the list lacks %q", want)
		}
	}
	if t.Failed() {
		t.Log(page)
	}

	r.repo.openReports = nil
	page = adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if !strings.Contains(page, "No open reports. All quiet.") || !strings.Contains(page, "<title>Reports · RNDMRoll admin</title>") {
		t.Fatalf("the empty list: %s", page)
	}
}

// The Disabled tab lists the disabled accounts with Enable right there, which
// comes back to this tab; it does not refresh itself.
func TestAdminDisabledTab_ListsThemWithEnable(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	at := cardAt(10, 2)
	larry := admin.Person{ID: uuid.New(), Username: "loud_larry", Name: "Larry", DisabledAt: &at}
	r.repo.disabled = []admin.Person{larry}
	rec := adminDo(t, r.w, http.MethodGet, "/admin/disabled", cookie, nil, "")
	page := rec.Body.String()
	for _, want := range []string{
		`class="on" aria-current="page">Disabled<span class="n">1</span>`,
		`<strong>@loud_larry</strong>`,
		`<form method="post" action="/admin/people/` + larry.ID.String() + `/enable">`,
		`<input type="hidden" name="from" value="disabled"><button>Enable</button>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the Disabled tab lacks %q", want)
		}
	}
	if rec.Code != http.StatusOK || strings.Contains(page, `http-equiv="refresh"`) {
		t.Fatalf("Disabled tab: %d, or it refreshes: %s", rec.Code, page)
	}
	enable := adminDo(t, r.w, http.MethodPost, "/admin/people/"+larry.ID.String()+"/enable", cookie,
		url.Values{"csrf": {r.h.sign("csrf", cookie)}, "from": {"disabled"}}, "")
	if enable.Code != http.StatusSeeOther || enable.Header().Get("Location") != "/admin/disabled?n=enabled" {
		t.Fatalf("enable from the tab: %d %q", enable.Code, enable.Header().Get("Location"))
	}

	r.repo.disabled = nil
	if page := adminDo(t, r.w, http.MethodGet, "/admin/disabled", cookie, nil, "").Body.String(); !strings.Contains(page, "No disabled accounts.") {
		t.Fatalf("the empty tab: %s", page)
	}
}

// Dismiss closes every report the card sent in one call (DismissItem checks
// each is open and about that person, and holds the post or comment while
// it works), and goes back where it came from: only the Reports tab, the
// Disabled tab, the Removed tab or the person's page. A dismissal that shows
// a hidden post or comment again says so; one that closed nothing, or
// failed, says that. A post's or comment's dismissal is logged on its
// owner's own clock.
func TestAdminDismiss_ClosesEveryReportOnTheCard(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person, a, b := uuid.New(), uuid.New(), uuid.New()
	dismiss := func(form url.Values) (int, string) {
		t.Helper()
		form.Set("csrf", r.h.sign("csrf", cookie))
		rec := adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", cookie, form, "")
		return rec.Code, rec.Header().Get("Location")
	}
	code, to := dismiss(url.Values{"kind": {admin.KindPost}, "id": {a.String(), b.String()}, "person": {person.String()}, "from": {"list"}})
	if code != http.StatusSeeOther || to != "/admin?n=dismissed" || len(r.repo.dismissals) != 1 || r.repo.resolved != 1 {
		t.Fatalf("dismiss from the list: %d %q, %d calls", code, to, len(r.repo.dismissals))
	}
	if got := r.repo.dismissals[0]; got.kind != admin.KindPost || got.person != person || !slices.Equal(got.ids, []uuid.UUID{a, b}) {
		t.Fatalf("the call: %+v, want the post, both ids and the person", got)
	}
	// The owner has no time zone in the spin store, so their own clock
	// cannot be read: the real one stands in. Once they have one, it is
	// the spin clock's.
	if at := r.repo.dismissals[0].loggedAt; time.Since(at) < 0 || time.Since(at) > time.Minute {
		t.Fatalf("logged at %v, want now when the owner's clock cannot be read", at)
	}
	if _, err := r.spin.SetTimeZone(context.Background(), person.String(), "America/New_York"); err != nil {
		t.Fatal(err)
	}
	dismiss(url.Values{"kind": {admin.KindComment}, "id": {a.String()}, "person": {person.String()}, "from": {"list"}})
	if got := r.repo.dismissals[1]; got.kind != admin.KindComment || !got.loggedAt.Equal(adminRigNow) {
		t.Fatalf("a comment's dismissal: %+v, want it logged at the owner's %v", got, adminRigNow)
	}
	// From the person's page, the Removed tab, or a made-up place.
	for from, want := range map[string]string{
		"":                      "/admin/people/" + person.String() + "?n=dismissed",
		"https://evil.example/": "/admin/people/" + person.String() + "?n=dismissed",
		"/admin/people":         "/admin/people/" + person.String() + "?n=dismissed",
		"disabled":              "/admin/disabled?n=dismissed",
		"removed":               "/admin/removed?n=dismissed",
	} {
		if _, to := dismiss(url.Values{"kind": {admin.KindPerson}, "id": {a.String()}, "person": {person.String()}, "from": {from}}); to != want {
			t.Fatalf("from %q: %q, want %q", from, to, want)
		}
	}
	// It showed a hidden post again.
	r.repo.shownAgain = true
	if _, to := dismiss(url.Values{"kind": {admin.KindPost}, "id": {a.String()}, "person": {person.String()}, "from": {"list"}}); to != "/admin?n=shownagain" {
		t.Fatalf("a dismissal that shows it again: %q", to)
	}
	if page := adminDo(t, r.w, http.MethodGet, "/admin?n=shownagain", cookie, nil, "").Body.String(); !strings.Contains(page, "Dismissed. It shows again, and they were told.") {
		t.Fatalf("the list lacks the notice: %s", page)
	}
	r.repo.shownAgain = false
	// Nothing was open any more.
	r.repo.notOpen = map[uuid.UUID]bool{a: true, b: true}
	if _, to := dismiss(url.Values{"kind": {admin.KindPost}, "id": {a.String(), b.String()}, "person": {person.String()}, "from": {"list"}}); to != "/admin?n=nothingopen" {
		t.Fatalf("all closed already: %q", to)
	}
	r.repo.notOpen = nil
	r.repo.dismissErr = errors.New("conn closed")
	if _, to := dismiss(url.Values{"kind": {admin.KindPost}, "id": {a.String()}, "person": {person.String()}, "from": {"list"}}); to != "/admin?n=failed" {
		t.Fatalf("a dismissal that fails: %q", to)
	}
	r.repo.dismissErr = nil
	// A form the page never draws closes nothing.
	before := r.repo.resolved
	tooMany := make([]string, adminDismissMost+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	for name, form := range map[string]url.Values{
		"no id":        {"kind": {admin.KindPost}, "person": {person.String()}},
		"a bad id":     {"kind": {admin.KindPost}, "id": {a.String(), "nope"}, "person": {person.String()}},
		"a bad kind":   {"kind": {"entry"}, "id": {a.String()}, "person": {person.String()}},
		"no person":    {"kind": {admin.KindPost}, "id": {a.String()}},
		"too many ids": {"kind": {admin.KindPost}, "id": tooMany, "person": {person.String()}},
	} {
		if _, to := dismiss(form); to != "/admin" {
			t.Errorf("%s: %q, want the list", name, to)
		}
	}
	if r.repo.resolved != before {
		t.Fatalf("a bad form dismissed %d times", r.repo.resolved-before)
	}
}

// The report email links to /admin/go: it needs no session (a click from a
// mail app arrives without the SameSite=Strict cookie) and moves on to the
// list itself, from this site, so the cookie goes along.
func TestAdminGoPage_MovesOnToTheList(t *testing.T) {
	r := newAdminRig(t, true)
	rec := adminDo(t, r.w, http.MethodGet, "/admin/go", "", nil, "")
	page := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(page, `<meta http-equiv="refresh" content="0;url=/admin">`) ||
		!strings.Contains(page, `<a class="button" href="/admin">Open the reports</a>`) ||
		!strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("go page: %d %s", rec.Code, page)
	}
	if r.repo.reads != 0 {
		t.Fatal("the go page read the reports")
	}
}
