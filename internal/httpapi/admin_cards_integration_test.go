package httpapi

import (
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
	ids := dismissIDs.FindAllStringSubmatch(account[strings.Index(account, `action="/admin/reports/dismiss"`):], -1)
	form := url.Values{"csrf": {csrfOf(t, list)}, "kind": {"person"}, "person": {b.id}, "from": {"list"}}
	for _, m := range ids {
		form.Add("id", m[1])
	}
	if len(ids) != 2 {
		t.Fatalf("the account card's Dismiss sends %d ids, want 2: %s", len(ids), account)
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
