package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

const adminTestPassword = "Correct-horse-battery-staple1"

// newAdminWorld is a feed world with the admin page mounted for the account
// "ad_boss@example.com", and that account.
func newAdminWorld(t *testing.T) (*feedWorld, feedPerson) {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	boss := adminFirst(w, pool, integrationOptions{clock: clk, adminEmails: []string{"ad_boss@example.com"}}, "ad_boss")
	return w, boss
}

// adminFirst signs the admin (username@example.com) up through a server
// without the admin page, then points w at the server with it, built from
// opts: the admins are the accounts their addresses have when the server
// starts, and sign-up refuses an admin address that has none. One database
// and one secret, so the admin's session works on both.
func adminFirst(w *feedWorld, pool *pgxpool.Pool, opts integrationOptions, username string) feedPerson {
	w.t.Helper()
	plain := opts
	plain.adminEmails = nil
	w.engine = buildIntegrationServerWith(w.t, pool, plain).Engine()
	admin := w.person(username, "Boss Admin")
	w.engine = buildIntegrationServerWith(w.t, pool, opts).Engine()
	return admin
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
	w, boss := newAdminWorld(t)
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

// adminLoginFrom signs in to the admin page from remote and returns where it
// sent the browser and the session cookie ("" when refused).
func adminLoginFrom(t *testing.T, w *feedWorld, remote, email, password string) (location, cookie string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(url.Values{"email": {email}, "password": {password}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	w.engine.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookie && c.Value != "" {
			cookie = c.Value
		}
	}
	return rec.Header().Get("Location"), cookie
}

// The admins are the accounts the ADMIN_EMAILS addresses had when the server
// started. An address with no account then is reported, answered as taken at
// sign-up (byte for byte, nothing stored), and is no admin even once an
// account has it (one made through a server that does not list it).
func TestAdminIntegration_AdminsAreTheAccountsFoundAtStart(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	plain := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk}).Engine()
	opts := integrationOptions{clock: clk, adminEmails: []string{"as_boss@example.com", "AS_Late@Example.com"}}
	adminFirst(w, pool, opts, "as_boss")

	ids, missing, err := FindAdminAccounts(context.Background(), postgres.NewUserRepo(pool), opts.adminEmails)
	if err != nil || len(ids) != 1 || len(missing) != 1 || missing[0] != "as_late@example.com" {
		t.Fatalf("FindAdminAccounts: ids %v, missing %v, err %v", ids, missing, err)
	}

	signUp := func(email, username string) *httptest.ResponseRecorder {
		return integrationDo(t, w.engine, http.MethodPost, "/v1/auth/signup", "", map[string]any{
			"email": email, "password": adminTestPassword, "birthday": validBirthday, "name": "Late Comer", "username": username,
		})
	}
	taken := signUp("as_boss@example.com", "as_taken")
	if taken.Code != http.StatusConflict {
		t.Fatalf("a taken email: %d %s", taken.Code, taken.Body.String())
	}
	before := integrationUserCount(t, pool)
	for _, email := range []string{"as_late@example.com", "AS_LATE@example.com"} {
		if rec := signUp(email, "as_late"); rec.Code != taken.Code || rec.Body.String() != taken.Body.String() {
			t.Fatalf("sign-up with the listed address %s: %d %s, want the taken answer %d %s",
				email, rec.Code, rec.Body.String(), taken.Code, taken.Body.String())
		}
	}
	if n := integrationUserCount(t, pool); n != before {
		t.Fatalf("a refused sign-up stored an account: %d, want %d", n, before)
	}

	// The account comes after the server started: not an admin.
	integrationOnboardAccount(t, plain, "as_late@example.com", adminTestPassword, "Late Comer", "as_late")
	if loc, cookie := adminLoginFrom(t, w, "198.51.100.80:1", "as_late@example.com", adminTestPassword); cookie != "" || !strings.Contains(loc, "n=badlogin") {
		t.Fatalf("an account made after the start got in: %s", loc)
	}
	if _, cookie := adminLoginFrom(t, w, "198.51.100.80:2", "as_boss@example.com", adminTestPassword); cookie == "" {
		t.Fatal("the admin found at the start could not log in")
	}
}

// The admin page's log in counts tries at an account together with the app's
// Log in, per address: five wrong tries on either page use up the share, and
// then the right password gets each page's usual wrong-password answer from
// that address, while another address still gets in.
func TestAdminIntegration_LogInSharesTheAppsPerAccountLimit(t *testing.T) {
	w, _ := newAdminWorld(t)
	appLogin := func(remote, login, password string) *httptest.ResponseRecorder {
		return postRawJSON(t, w.engine, remote, "/v1/auth/login", fmt.Sprintf(`{"login":%q,"password":%q}`, login, password))
	}

	// Five wrong tries in the app, by the username; then the admin page, by
	// the email, with the right password.
	const app = "198.51.100.81:1"
	for range 5 {
		assertInvalidCredentials(t, appLogin(app, "ad_boss", "Wrong-password-1!"))
	}
	if loc, cookie := adminLoginFrom(t, w, app, "ad_boss@example.com", adminTestPassword); cookie != "" || !strings.Contains(loc, "n=badlogin") {
		t.Fatalf("the admin page past the app's tries: %s (cookie %v)", loc, cookie != "")
	}
	if _, cookie := adminLoginFrom(t, w, "198.51.100.82:1", "ad_boss@example.com", adminTestPassword); cookie == "" {
		t.Fatal("another address was refused")
	}

	// Five wrong tries on the admin page; then the app, with the right
	// password: the wrong-password answer.
	const page = "198.51.100.83:1"
	for range 5 {
		if loc, _ := adminLoginFrom(t, w, page, "ad_boss@example.com", "Wrong-password-1!"); !strings.Contains(loc, "n=badlogin") {
			t.Fatalf("a wrong admin password: %s", loc)
		}
	}
	assertInvalidCredentials(t, appLogin(page, "ad_boss", adminTestPassword))
	if rec := appLogin("198.51.100.84:1", "ad_boss", adminTestPassword); rec.Code != http.StatusOK {
		t.Fatalf("the app from another address: %d %s", rec.Code, rec.Body.String())
	}
}
