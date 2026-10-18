package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

const adminTestPassword = "Correct-horse-battery-staple1"

// newAdminWorld is a feed world with the admin page mounted for the account
// "ad_boss@example.com".
func newAdminWorld(t *testing.T) *feedWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"}}).Engine()
	return &feedWorld{t: t, engine: engine, clk: clk}
}

// adminForm sends one form POST (or a GET when form is nil) to the admin
// page with the cookie, and returns the response.
func adminDo(t *testing.T, w *feedWorld, method, path, cookie string, form url.Values, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: adminCookie, Value: cookie})
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	w.engine.ServeHTTP(rec, req)
	return rec
}

// adminLogin signs in and returns the session cookie ("" when refused).
func adminLogin(t *testing.T, w *feedWorld, email, password string) string {
	t.Helper()
	rec := adminDo(t, w, http.MethodPost, "/admin/login", "", url.Values{"email": {email}, "password": {password}}, "")
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookie && c.Value != "" {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/admin" {
				t.Fatalf("cookie flags: %+v", c)
			}
			return c.Value
		}
	}
	return ""
}

// csrfOf reads the CSRF token printed on an admin page.
func csrfOf(t *testing.T, page string) string {
	t.Helper()
	const mark = `name="csrf" value="`
	i := strings.Index(page, mark)
	if i < 0 {
		t.Fatalf("no csrf on the page")
	}
	rest := page[i+len(mark):]
	return rest[:strings.Index(rest, `"`)]
}

func TestAdminIntegration_LoginReportsAndActions(t *testing.T) {
	w := newAdminWorld(t)
	boss := w.person("ad_boss", "Boss Admin")
	a := w.person("ad_alice", "Alice Ames")
	b := w.person("ad_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	aEntry := w.post(a, day)

	// Reports from the app: a post, a comment with markup in it, a person.
	w.call(b, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "nope"}, http.StatusBadRequest)
	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
	bad := w.call(b, http.MethodPost, "/v1/posts/"+aEntry+"/comments", map[string]any{"text": "<script>alert(1)</script>"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	w.call(b, http.MethodPost, "/v1/comments/"+bad+"/report", map[string]any{"reason": "harassment"}, http.StatusNotFound)
	w.ok(a, http.MethodPost, "/v1/comments/"+bad+"/report", map[string]any{"reason": "harassment"})
	w.ok(a, http.MethodPost, "/v1/people/"+b.id+"/report", map[string]any{"reason": "harassment"})
	// A stranger cannot report a post they cannot see.
	stranger := w.person("ad_stranger", "Sam Stone")
	w.call(stranger, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)

	// Only an admin, with the right password, gets in.
	if adminLogin(t, w, "ad_alice@example.com", adminTestPassword) != "" {
		t.Fatal("a non-admin got in")
	}
	if adminLogin(t, w, "ad_boss@example.com", "wrong") != "" {
		t.Fatal("a wrong password got in")
	}
	if rec := adminDo(t, w, http.MethodGet, "/admin", "", nil, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("no cookie: %d", rec.Code)
	}
	if rec := adminDo(t, w, http.MethodGet, "/admin", "forged.1.x", nil, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("forged cookie: %d", rec.Code)
	}
	// An app token is not a session.
	if rec := adminDo(t, w, http.MethodGet, "/admin", a.token, nil, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("app token: %d", rec.Code)
	}
	cookie := adminLogin(t, w, "AD_BOSS@example.com", adminTestPassword)
	if cookie == "" {
		t.Fatal("the admin could not log in")
	}

	list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "@ad_bob") ||
		!strings.Contains(list.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	if strings.Contains(page, "<script>alert") || !strings.Contains(page, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("comment not escaped: %s", page)
	}
	csrf := csrfOf(t, page)

	// No CSRF token, or another site's Origin: refused.
	if rec := adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{}, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf: %d", rec.Code)
	}
	if rec := adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross origin: %d", rec.Code)
	}
	// The admin cannot disable themselves.
	adminDo(t, w, http.MethodPost, "/admin/people/"+boss.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "")
	w.ok(boss, http.MethodGet, "/v1/me", nil)

	// Remove the reported comment: gone for everyone, its report closed.
	var commentReport string
	for _, part := range strings.Split(page, `name="kind" value="comment"><input type="hidden" name="id" value="`)[1:] {
		commentReport = part[:36]
	}
	rec := adminDo(t, w, http.MethodPost, "/admin/reports/remove", cookie,
		url.Values{"csrf": {csrf}, "kind": {"comment"}, "id": {commentReport}, "person": {b.id}}, "")
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "n=removed") {
		t.Fatalf("remove comment: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if ids, _ := w.commentIDs(a, aEntry); len(ids) != 0 {
		t.Fatalf("comment still there: %v", ids)
	}

	// Disable B: logged out, refused, and gone for A everywhere.
	rec = adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "")
	if !strings.Contains(rec.Header().Get("Location"), "n=disabled") {
		t.Fatalf("disable: %s", rec.Header().Get("Location"))
	}
	if got := w.call(b, http.MethodGet, "/v1/me", nil, http.StatusUnauthorized); got["error"] != "account_disabled" {
		t.Fatalf("old token: %v", got)
	}
	login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "ad_bob@example.com", "password": adminTestPassword})
	if login.Code != http.StatusForbidden || !strings.Contains(login.Body.String(), "account_disabled") {
		t.Fatalf("login while disabled: %d %s", login.Code, login.Body.String())
	}
	bad2 := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "ad_bob@example.com", "password": "wrong-password-1A"})
	if strings.Contains(bad2.Body.String(), "account_disabled") {
		t.Fatalf("a wrong password learns the account is disabled: %s", bad2.Body.String())
	}
	w.call(a, http.MethodGet, "/v1/people/"+b.id, nil, http.StatusNotFound)
	w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	if ids := cardIDs(t, w.board(a, "")); has(ids, bEntry) {
		t.Fatalf("board still has b: %v", ids)
	}
	if got := w.searchNames(a, "ad_bob"); len(got) != 0 {
		t.Fatalf("search still finds b: %v", got)
	}
	if got := usernames(t, w.lists(a)["followers"]); has(got, "ad_bob") {
		t.Fatalf("followers still list b: %v", got)
	}
	if w.followers(a) != 0 {
		t.Fatalf("follower count still counts b")
	}

	// Enable: everything comes back.
	csrf = csrfOf(t, adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String())
	adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/enable", cookie, url.Values{"csrf": {csrf}}, "")
	if login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "ad_bob@example.com", "password": adminTestPassword}); login.Code != http.StatusOK {
		t.Fatalf("login after enable: %d", login.Code)
	}
	w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	w.ok(a, http.MethodGet, "/v1/posts/"+bEntry, nil)

	// Disabling closed every report about B; a new report opens one again.
	// Remove that reported post through the app's own delete.
	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
	page = adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	var postReport string
	for _, part := range strings.Split(page, `name="kind" value="post"><input type="hidden" name="id" value="`)[1:] {
		postReport = part[:36]
	}
	if postReport == "" {
		t.Fatalf("the post report is not offered for removal: %s", page)
	}
	rec = adminDo(t, w, http.MethodPost, "/admin/reports/remove", cookie,
		url.Values{"csrf": {csrfOf(t, page)}, "kind": {"post"}, "id": {postReport}, "person": {b.id}}, "")
	if !strings.Contains(rec.Header().Get("Location"), "n=removed") {
		t.Fatalf("remove post: %s", rec.Header().Get("Location"))
	}
	w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)

	// Logging out ends the session cookie.
	rec = adminDo(t, w, http.MethodPost, "/admin/logout", cookie, url.Values{"csrf": {csrfOf(t, page)}}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d", rec.Code)
	}
}
