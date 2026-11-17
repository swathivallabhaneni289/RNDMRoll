package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/adminlink"
)

// socialAdminCookie sends a request to engine from remote and returns the
// admin cookie the answer sets, nil for none.
func socialAdminCookie(t *testing.T, engine *gin.Engine, req *http.Request, remote string) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec, adminCookieOf(rec)
}

// socialAdminLogin is a log in form for boss@example.com with password.
func socialAdminLogin(password string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/admin/login",
		strings.NewReader(url.Values{"email": {"boss@example.com"}, "password": {password}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// ADM-01 the session: it lasts 8 hours from log in, and a cookie changed in
// any part is refused. Each refusal goes back to log in without running the
// page or an action behind it. The cookie log in (and a make admin link)
// sets lives 8 hours in the browser too, is HttpOnly and SameSite=Strict,
// and is Secure on a handler built for https.
func TestSocialCoverage_AnAdminSessionLastsEightHoursAndAChangedCookieIsRefused(t *testing.T) {
	r := newAdminRig(t, true)
	get := func(cookie string) *httptest.ResponseRecorder {
		t.Helper()
		return adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "")
	}
	refused := func(name, cookie string) {
		t.Helper()
		reads := r.repo.reads
		sentToLogin(t, name, get(cookie), "expired", false)
		form := url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {admin.KindPerson}, "id": {uuid.NewString()}, "person": {uuid.NewString()}}
		sentToLogin(t, name+", a dismiss", adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", cookie, form, ""), "expired", false)
		if r.repo.reads != reads || r.repo.resolved != 0 {
			t.Fatalf("%s: the page behind ran (%d reads, %d dismissals)", name, r.repo.reads-reads, r.repo.resolved)
		}
	}

	// Log in: the cookie lasts 8 hours, as its expiry part says.
	before := time.Now()
	rec, c := socialAdminCookie(t, r.w.engine, socialAdminLogin(adminTestPassword), "198.51.100.201:1")
	if rec.Code != http.StatusSeeOther || c == nil || c.MaxAge != 28800 || int(adminSessionTTL.Seconds()) != 28800 ||
		!c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/admin" || c.Secure {
		t.Fatalf("the log in cookie: %d %+v", rec.Code, c)
	}
	parts := strings.Split(c.Value, ".")
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if len(parts) != 4 || err != nil || exp < before.Add(adminSessionTTL).Unix() || exp > time.Now().Add(adminSessionTTL).Unix() {
		t.Fatalf("the session's expiry: %q, want 8 hours from now", c.Value)
	}
	if rec := get(c.Value); rec.Code != http.StatusOK {
		t.Fatalf("the new session: %d", rec.Code)
	}

	// The 8 hours, on the page's own clock: a minute left works; 8 hours
	// and a second, or exactly 8 hours, does not.
	if rec := get(r.session(time.Now().Add(-adminSessionTTL + time.Minute))); rec.Code != http.StatusOK {
		t.Fatalf("a session with a minute left: %d", rec.Code)
	}
	refused("a session 8 hours and a second old", r.session(time.Now().Add(-adminSessionTTL-time.Second)))
	refused("a session exactly 8 hours old", r.session(time.Now().Add(-adminSessionTTL)))
	at := time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)
	signed := r.h.newSession(r.boss.ID, "mark", at)
	if _, _, ok := r.h.sessionUser(signed, at.Add(adminSessionTTL-time.Second)); !ok {
		t.Fatal("a session a second before its end was refused")
	}
	if _, _, ok := r.h.sessionUser(signed, at.Add(adminSessionTTL)); ok {
		t.Fatal("a session was kept at its end")
	}

	// A cookie changed in any part, with the signature it had.
	good := r.session(time.Now())
	p := strings.Split(good, ".")
	flip := func(s string) string {
		last := "A"
		if strings.HasSuffix(s, "A") {
			last = "B"
		}
		return s[:len(s)-1] + last
	}
	for name, cookie := range map[string]string{
		"a later expiry":           p[0] + "." + strconv.FormatInt(time.Now().Add(100*time.Hour).Unix(), 10) + "." + p[2] + "." + p[3],
		"another account":          uuid.NewString() + "." + p[1] + "." + p[2] + "." + p[3],
		"another password mark":    p[0] + "." + p[1] + "." + flip(p[2]) + "." + p[3],
		"one letter of the seal":   p[0] + "." + p[1] + "." + p[2] + "." + flip(p[3]),
		"no seal":                  p[0] + "." + p[1] + "." + p[2] + ".",
		"a fifth part":             good + ".x",
		"the payload alone":        p[0] + "." + p[1] + "." + p[2],
		"signed with another key":  NewAdminHandler(r.repo, r.users, nil, []byte("another-secret-that-is-at-least-32-bytes"), nil, false).newSession(r.boss.ID, r.h.passwordMark(r.boss), time.Now()),
		"signed for another use":   p[0] + "." + p[1] + "." + p[2] + "." + r.h.sign("csrf", p[0]+"."+p[1]+"."+p[2]),
		"the CSRF token as cookie": r.h.sign("csrf", good),
	} {
		refused(name, cookie)
	}
	if rec := get(good); rec.Code != http.StatusOK {
		t.Fatalf("the cookie as signed: %d", rec.Code)
	}

	// Signed properly, but the expiry is not a number or the account not an
	// id: each is refused at its own check, and the cookie is not cleared
	// (no account was ever looked up).
	resign := func(id, expiry string) string {
		payload := id + "." + expiry + "." + p[2]
		return payload + "." + r.h.sign("session", payload)
	}
	far := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	for name, cookie := range map[string]string{
		"an expiry that is not a number": resign(r.boss.ID.String(), "soon"),
		"an empty expiry":                resign(r.boss.ID.String(), ""),
		"an account that is not an id":   resign("boss", far),
	} {
		if _, _, ok := r.h.sessionUser(cookie, time.Now()); ok {
			t.Fatalf("%s: sessionUser kept it", name)
		}
		refused(name, cookie)
	}

	// A make admin link sets the same 8-hour cookie.
	token, err := adminlink.New(adminUnitSecret, "", frozenClock())
	if err != nil {
		t.Fatal(err)
	}
	rec, c = socialAdminCookie(t, r.w.engine, httptest.NewRequest(http.MethodGet, "/admin/link?token="+url.QueryEscape(token), nil), "198.51.100.202:1")
	if rec.Code != http.StatusOK || c == nil || c.MaxAge != 28800 || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("the link's cookie: %d %+v", rec.Code, c)
	}

	// Log out clears it.
	out := adminDo(t, r.w, http.MethodPost, "/admin/logout", good, url.Values{"csrf": {r.h.sign("csrf", good)}}, "")
	if c := adminCookieOf(out); c == nil || c.Value != "" || c.MaxAge >= 0 || out.Header().Get("Location") != "/admin/login?n=signedout" {
		t.Fatalf("log out: %d %q %+v", out.Code, out.Header().Get("Location"), c)
	}

	// Built for https (secure, as cmd/api does outside test mode), the cookie
	// is Secure.
	secure := NewAdminHandler(r.repo, r.users, NewEntryHandler(r.spin, r.photos), adminUnitSecret, []uuid.UUID{r.boss.ID}, true)
	secure.clock = frozenClock
	engine := gin.New()
	secure.Register(engine)
	rec, c = socialAdminCookie(t, engine, socialAdminLogin(adminTestPassword), "198.51.100.203:1")
	if rec.Code != http.StatusSeeOther || c == nil || !c.Secure || c.MaxAge != 28800 || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("the cookie of a handler built for https: %d %+v", rec.Code, c)
	}
}

// socialScriptAttr finds an event handler attribute inside a tag.
var socialScriptAttr = regexp.MustCompile(`(?i)<[a-z][^>]*\son[a-z]+\s*=`)

// socialNoScript fails when a page body could run a script: a script tag, an
// event handler attribute or a javascript: link.
func socialNoScript(t *testing.T, name, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	if strings.Contains(lower, "<script") || strings.Contains(lower, "javascript:") || socialScriptAttr.MatchString(body) {
		t.Fatalf("%s could run a script: %s", name, body)
	}
}

// ADM-01 protection: every admin answer, pages, redirects, the stylesheet
// and the failure page alike, runs no scripts and carries the full strict
// header set, so no page can be framed, sniffed, cached or followed by a
// referrer that holds a signed photo link. The invite page has its own
// strict policy.
func TestSocialCoverage_EveryAdminPageRunsNoScriptAndCarriesTheStrictHeaders(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	owner := admin.Person{ID: uuid.New(), Username: "some_one", Name: "Some One", Bio: "Hi there", CreatedAt: time.Now()}
	now := time.Now()
	report := admin.Report{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "rep_one", Reason: "spam", CreatedAt: now}
	r.repo.openReports = []admin.OpenReport{{Owner: owner, Report: report}}
	r.repo.reports = []admin.Report{report}
	r.repo.disabled = []admin.Person{{ID: uuid.New(), Username: "off_one", Name: "Off One", DisabledAt: &now}}
	dismiss := url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {admin.KindPerson}, "id": {report.ID.String()}, "person": {owner.ID.String()}}

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; img-src 'self' https: http:; style-src 'self'; script-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
	}
	check := func(name string, rec *httptest.ResponseRecorder, code int) {
		t.Helper()
		if rec.Code != code {
			t.Fatalf("%s: %d, want %d: %s", name, rec.Code, code, rec.Body.String())
		}
		for header, value := range want {
			if got := rec.Header().Values(header); len(got) != 1 || got[0] != value {
				t.Fatalf("%s: %s is %q, want %q", name, header, got, value)
			}
		}
		socialNoScript(t, name, rec.Body.String())
	}
	for _, c := range []struct {
		name, method, path, cookie string
		form                       url.Values
		code                       int
	}{
		{"the log in page", http.MethodGet, "/admin/login", "", nil, http.StatusOK},
		{"the log in page with a notice", http.MethodGet, "/admin/login?n=badlogin", "", nil, http.StatusOK},
		{"the report email's hop", http.MethodGet, "/admin/go", "", nil, http.StatusOK},
		{"the stylesheet", http.MethodGet, "/admin/style.css", "", nil, http.StatusOK},
		{"the Reports tab", http.MethodGet, "/admin", cookie, nil, http.StatusOK},
		{"the Disabled tab", http.MethodGet, "/admin/disabled", cookie, nil, http.StatusOK},
		{"a person's page", http.MethodGet, "/admin/people/" + owner.ID.String(), cookie, nil, http.StatusOK},
		{"the way to log in without a session", http.MethodGet, "/admin", "", nil, http.StatusSeeOther},
		{"a wrong log in", http.MethodPost, "/admin/login", "", url.Values{"email": {"boss@example.com"}, "password": {"Wrong-password-1!"}}, http.StatusSeeOther},
		{"a dismiss", http.MethodPost, "/admin/reports/dismiss", cookie, dismiss, http.StatusSeeOther},
	} {
		check(c.name, adminDo(t, r.w, c.method, c.path, c.cookie, c.form, ""), c.code)
	}
	if page := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "").Body.String(); !strings.Contains(page, "@some_one") || !strings.Contains(page, "<form") {
		t.Fatalf("the Reports tab drew no card, so the scan above saw no form: %s", page)
	}

	// The failure page, when the account cannot be read.
	r.users.getErr = errors.New("connection reset by peer")
	check("the failure page", adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, ""), http.StatusInternalServerError)
	r.users.getErr = nil

	// The invite page: its own strict policy, no script.
	gin.SetMode(gin.TestMode)
	invite := gin.New()
	RegisterInvite(invite, "rndmroll")
	rec := httptest.NewRecorder()
	invite.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/i/bob_brown", nil))
	for header, value := range map[string]string{
		"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	} {
		if got := rec.Header().Get(header); got != value {
			t.Fatalf("the invite page's %s: %q, want %q", header, got, value)
		}
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "@bob_brown invited you") {
		t.Fatalf("the invite page: %d %s", rec.Code, rec.Body.String())
	}
	socialNoScript(t, "the invite page", rec.Body.String())
}

// ADM-02: past the newest 500 open reports, the Reports tab says only the
// newest fit and shows those 500 (the card names 3 reporters and "497
// more"). At 500 or fewer it says nothing.
func TestSocialCoverage_TheReportsTabSaysWhenOnlyTheNewestFit(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	const line = "Only the newest open reports fit here. Close some to see the rest."
	owner := admin.Person{ID: uuid.New(), Username: "much_reported", Name: "Much Reported"}
	reports := func(n int) []admin.OpenReport {
		out := make([]admin.OpenReport, n)
		at := time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)
		for i := range out {
			out[i] = admin.OpenReport{Owner: owner, Report: admin.Report{Kind: admin.KindPerson, ID: uuid.New(),
				Reporter: "rep_" + strconv.Itoa(i), Reason: "spam", CreatedAt: at.Add(-time.Duration(i) * time.Minute)}}
		}
		return out
	}
	for _, c := range []struct {
		n    int
		says bool
	}{{0, false}, {1, false}, {admin.OpenReportsLimit - 1, false}, {admin.OpenReportsLimit, false}, {admin.OpenReportsLimit + 1, true}} {
		r.repo.openReports = reports(c.n)
		page := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, "")
		if page.Code != http.StatusOK || strings.Contains(page.Body.String(), line) != c.says {
			t.Fatalf("%d open reports: %d, says only the newest fit: %v, want %v", c.n, page.Code, !c.says, c.says)
		}
		if c.n >= admin.OpenReportsLimit && !strings.Contains(page.Body.String(), " and 497 more ·") {
			t.Fatalf("%d open reports: the card does not hold the newest 500: %s", c.n, page.Body.String())
		}
	}
	if admin.OpenReportsLimit != 500 {
		t.Fatalf("OpenReportsLimit is %d, want 500 (ADM-02)", admin.OpenReportsLimit)
	}
}
