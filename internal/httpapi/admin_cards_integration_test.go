package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// heardReports is a report notifier that keeps each reason it hears, and
// apart from them the reasons of the reports that hid their post or comment.
type heardReports struct {
	mu      sync.Mutex
	reasons []string
	hid     []string
}

func (h *heardReports) Reported(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reasons = append(h.reasons, reason)
}

func (h *heardReports) ReportedHiding(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reasons = append(h.reasons, reason)
	h.hid = append(h.hid, reason)
}

// got is every report heard, hiding ones included, in order.
func (h *heardReports) got() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.reasons)
}

// hiding is the reports heard as the ones that hid something, in order.
func (h *heardReports) hiding() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.hid)
}

var dismissIDs = regexp.MustCompile(`<input type="hidden" name="id" value="([0-9a-f-]{36})">`)

// dismissAts are the report times a person card's Dismiss sends, in its ids'
// order.
var dismissAts = regexp.MustCompile(`<input type="hidden" name="at" value="([^"]+)">`)

// dismissForm is the Dismiss form of card as the page draws it: every id
// and, on a person's card, every time.
func dismissForm(card string, form url.Values) url.Values {
	tail := card[strings.Index(card, `action="/admin/reports/dismiss"`):]
	for _, m := range dismissIDs.FindAllStringSubmatch(tail, -1) {
		form.Add("id", m[1])
	}
	for _, m := range dismissAts.FindAllStringSubmatch(tail, -1) {
		form.Add("at", m[1])
	}
	return form
}

// The Reports tab on the database: one card per reported post and per
// reported account, across people, the most urgent first; a report kept is
// heard by the email's notifier and a refused one is not; Dismiss on a card
// closes all of its reports; a repeat report of an account opens it again;
// and the Disabled tab lists a disabled account. No email address anywhere.
func TestAdminIntegration_TheReportsTabHasOneCardPerReportedThing(t *testing.T) {
	pool := requireIntegrationPool(t)
	heard := &heardReports{}
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	adminFirst(w, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"},
		wire: func(s *social.Service, b *board.Service, _ *Deps) {
			s.SetReportNotifier(heard)
			b.SetReportNotifier(heard)
		}}, "ad_boss")
	a := w.person("ad_alice", "Alice Ames")
	c := w.person("ad_cara", "Cara Cole")
	b := w.person("ad_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)

	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
	w.ok(c, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "violence"})
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	w.ok(c, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	w.call(b, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "nope"}, http.StatusBadRequest)
	if got := heard.got(); !slices.Equal(got, []string{"spam", "violence", "harassment", "harassment"}) {
		t.Fatalf("heard %v, want the four reports kept and nothing refused", got)
	}

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if n := strings.Count(list, `<article class="card`); n != 2 || strings.Contains(list, "example.com") ||
		!strings.Contains(list, "<title>(2) Reports · RNDMRoll admin</title>") {
		t.Fatalf("want 2 cards, the count in the title and no email address; %d cards: %s", n, list)
	}
	post := adminReportOf(t, list, "Violence or threats · Spam")
	account := adminReportOf(t, list, "Bullying or harassment")
	// Both reporters, once each (the test clock gives them the same time, so
	// either may come first).
	bothReporters := func(card string) bool {
		return strings.Contains(card, "Reported by @ad_cara and @ad_alice</p>") || strings.Contains(card, "Reported by @ad_alice and @ad_cara</p>")
	}
	if strings.Index(list, "Violence or threats · Spam") > strings.Index(list, "Bullying or harassment") ||
		!bothReporters(post) || !strings.Contains(post, "<strong>@ad_bob</strong>") || !bothReporters(account) {
		t.Fatalf("the cards:\n%s\n%s", post, account)
	}

	// Dismiss on the account card closes both of its reports.
	form := dismissForm(account, url.Values{"csrf": {csrfOf(t, list)}, "kind": {"person"}, "person": {b.id}, "from": {"list"}})
	if len(form["id"]) != 2 || len(form["at"]) != 2 {
		t.Fatalf("the account card's Dismiss sends %d ids and %d times, want 2 of each: %s", len(form["id"]), len(form["at"]), account)
	}
	rec := adminDo(t, w, http.MethodPost, "/admin/reports/dismiss", cookie, form, "")
	if rec.Header().Get("Location") != "/admin?n=dismissed" {
		t.Fatalf("dismiss: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	list = adminDo(t, w, http.MethodGet, "/admin?n=dismissed", cookie, nil, "").Body.String()
	// The post card's Reason list names every reason, so the account card is
	// looked for by its reasons line.
	if strings.Count(list, `<article class="card`) != 1 || strings.Contains(list, `<p class="reasons">Bullying or harassment`) ||
		!strings.Contains(list, "Dismissed. Its reports are closed.") {
		t.Fatalf("after the dismissal: %s", list)
	}

	// The same person reporting the account again opens a report again.
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "impersonation"})
	list = adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if again := adminReportOf(t, list, "Pretending to be someone else"); !strings.Contains(again, "Reported by @ad_alice</p>") {
		t.Fatalf("the repeat report: %s", again)
	}

	// Disabled: the account moves to the Disabled tab, with Enable there.
	csrf := csrfOf(t, list)
	adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "")
	tab := adminDo(t, w, http.MethodGet, "/admin/disabled", cookie, nil, "").Body.String()
	if !strings.Contains(tab, "<strong>@ad_bob</strong>") || strings.Contains(tab, "example.com") ||
		!strings.Contains(tab, `<form method="post" action="/admin/people/`+b.id+`/enable">`) ||
		!strings.Contains(tab, `Disabled<span class="n">1</span>`) {
		t.Fatalf("the Disabled tab: %s", tab)
	}
	if list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String(); strings.Contains(list, `<article class="card`) {
		t.Fatalf("disabling closed every report about them, yet the list has cards: %s", list)
	}
}

// A Dismiss closes only the reports its card was drawn with. A person's
// page does not redraw itself, and a second report by the same person
// replaces their first in place, so the card sends each report's time with
// its reporter. Drawn with two reports, sent after one reporter reported
// again (an hour later, a new reason), it closes the other one only, the
// newer report stays open and the page shows it. Drawn again, sent after
// that reporter reported once more on a clock moved back (the test tools
// can do that), it closes nothing and says the page was out of date. A
// fresh card then closes it, and the same form again finds it closed.
func TestAdminIntegration_ADismissFromAnOldPageClosesOnlyWhatItShowed(t *testing.T) {
	pool := requireIntegrationPool(t)
	w, _ := newAdminWorldOn(t, pool)
	a := w.person("ad_alice", "Alice Ames")
	c := w.person("ad_cara", "Cara Cole")
	b := w.person("ad_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	// A real clock reads to the nanosecond and the database keeps the
	// microsecond, so the times the card sends must match on that.
	w.clk.Advance(123456789 * time.Nanosecond)
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	w.ok(c, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	personPage := "/admin/people/" + b.id
	draw := func(mark string) url.Values {
		t.Helper()
		page := adminDo(t, w, http.MethodGet, personPage, cookie, nil, "").Body.String()
		return dismissForm(adminReportOf(t, page, mark), url.Values{"csrf": {csrfOf(t, page)}, "kind": {"person"}, "person": {b.id}})
	}
	send := func(form url.Values, notice string) string {
		t.Helper()
		rec := adminDo(t, w, http.MethodPost, "/admin/reports/dismiss", cookie, form, "")
		if rec.Header().Get("Location") != personPage+"?n="+notice {
			t.Fatalf("dismiss: %d %q, want %s", rec.Code, rec.Header().Get("Location"), notice)
		}
		return adminDo(t, w, http.MethodGet, rec.Header().Get("Location"), cookie, nil, "").Body.String()
	}
	report := func(p feedPerson) (reason string, open bool) {
		t.Helper()
		if err := pool.QueryRow(context.Background(), `select reason, resolved_at is null from user_reports
			where reporter_id = $1 and reported_id = $2`, p.id, b.id).Scan(&reason, &open); err != nil {
			t.Fatal(err)
		}
		return reason, open
	}

	old := draw("Bullying or harassment")
	if len(old["id"]) != 2 || len(old["at"]) != 2 {
		t.Fatalf("the card's Dismiss: %v, want both reports and their times", old)
	}
	w.clk.Advance(time.Hour)
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "impersonation"})
	page := send(old, "dismissed")
	if reason, open := report(a); reason != "impersonation" || !open {
		t.Fatalf("the newer report: %s, open %v; want it open", reason, open)
	}
	if _, open := report(c); open {
		t.Fatal("the report the card showed is still open")
	}
	if card := adminReportOf(t, page, "Pretending to be someone else"); !strings.Contains(card, "Reported by @ad_alice</p>") ||
		!strings.Contains(card, `action="/admin/reports/dismiss"`) {
		t.Fatalf("the page after, want the newer report open: %s", card)
	}

	again := draw("Pretending to be someone else")
	w.clk.Advance(-3 * time.Hour)
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "spam"})
	page = send(again, "stale")
	if reason, open := report(a); reason != "spam" || !open {
		t.Fatalf("the report made on a clock moved back: %s, open %v; want it open", reason, open)
	}
	if !strings.Contains(page, `<div class="note">That page was out of date. Try again.</div>`) ||
		!strings.Contains(adminReportOf(t, page, "Spam"), "Reported by @ad_alice</p>") {
		t.Fatalf("the page after, want the notice and the newest report: %s", page)
	}

	fresh := draw("Spam")
	send(fresh, "dismissed")
	if _, open := report(a); open {
		t.Fatal("a Dismiss drawn with the newest report left it open")
	}
	send(fresh, "nothingopen")
}
