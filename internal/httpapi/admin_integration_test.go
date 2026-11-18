package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/adminlink"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

const adminTestPassword = "Correct-horse-battery-staple1"

// newAdminWorld is a feed world with the admin page mounted for the account
// "ad_boss@example.com", and that account.
func newAdminWorld(t *testing.T) (*feedWorld, feedPerson) {
	t.Helper()
	return newAdminWorldOn(t, requireIntegrationPool(t))
}

// newAdminWorldOn is newAdminWorld on pool, for a test that also changes the
// database itself.
func newAdminWorldOn(t *testing.T, pool *pgxpool.Pool) (*feedWorld, feedPerson) {
	t.Helper()
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
	// Cara follows B and reports nothing at first, so she shows what
	// everyone else sees of B's post: A's report takes it away from A.
	c := w.person("ad_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	aEntry := w.post(a, day)
	w.post(c, day)

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

	// No CSRF token: nothing is done, and the list says the page was out of
	// date. Another site's Origin: refused. B is still enabled.
	if rec := adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{}, ""); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin?n=stale" {
		t.Fatalf("no csrf: %d %q, want 303 to /admin?n=stale", rec.Code, rec.Header().Get("Location"))
	}
	if rec := adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross origin: %d", rec.Code)
	}
	w.ok(b, http.MethodGet, "/v1/me", nil)
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
	// A reported it, so it is gone for A anyway: B, who wrote it, shows it
	// is gone for everyone.
	for _, p := range []feedPerson{a, b} {
		if ids, _ := w.commentIDs(p, aEntry); len(ids) != 0 {
			t.Fatalf("comment still there for %s: %v", p.name, ids)
		}
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
	// B's post is checked as Cara: A reported it, so it is gone for A anyway.
	w.call(c, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	if ids := cardIDs(t, w.board(c, "")); has(ids, bEntry) {
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

	// Enable: everything comes back, except B's post for A, who reported it:
	// that is gone for its reporter for good.
	csrf = csrfOf(t, adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String())
	adminDo(t, w, http.MethodPost, "/admin/people/"+b.id+"/enable", cookie, url.Values{"csrf": {csrf}}, "")
	login = integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "ad_bob@example.com", "password": adminTestPassword})
	if login.Code != http.StatusOK {
		t.Fatalf("login after enable: %d", login.Code)
	}
	b.token = decodeBody(t, login)["access_token"].(string)
	w.ok(a, http.MethodGet, "/v1/people/"+b.id, nil)
	w.ok(c, http.MethodGet, "/v1/posts/"+bEntry, nil)
	w.call(a, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)

	// Disabling closed every report about B; a new report opens one again,
	// from Cara (A's own report is not made twice). Remove that reported post.
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.ok(c, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
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
	// Both reporters lost it already, so its owner shows it is gone.
	w.call(b, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
	w.call(b, http.MethodGet, "/v1/entries/"+bEntry, nil, http.StatusNotFound)

	// Logging out ends the session cookie.
	rec = adminDo(t, w, http.MethodPost, "/admin/logout", cookie, url.Values{"csrf": {csrfOf(t, page)}}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d", rec.Code)
	}
}

// The admin page opens only people someone reported (them, a post or a
// comment, open or closed), disabled accounts and people the decision log
// names, and shows no email address. Anyone else's page is the list, with
// nothing about them, and a disable or an enable sent for them anyway
// changes nothing.
func TestAdminIntegration_OnlyReportedOrDisabledPeopleOpen(t *testing.T) {
	pool := requireIntegrationPool(t)
	w, _ := newAdminWorldOn(t, pool)
	a := w.person("ad_alice", "Alice Ames")
	quiet := w.person("ad_quiet", "Quinn Quiet")
	reported := w.person("ad_rep", "Remy Reported")
	w.ok(a, http.MethodPost, "/v1/people/"+reported.id+"/report", map[string]any{"reason": "spam"})

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	if cookie == "" {
		t.Fatal("the admin could not log in")
	}
	list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if strings.Contains(list, "example.com") {
		t.Fatalf("the list shows an email address: %s", list)
	}
	csrf := csrfOf(t, list)
	sentToList := func(name string, rec *httptest.ResponseRecorder, want string) {
		t.Helper()
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want || strings.Contains(rec.Body.String(), "ad_quiet") {
			t.Fatalf("%s: %d %q, want the list at %q: %s", name, rec.Code, rec.Header().Get("Location"), want, rec.Body.String())
		}
	}

	// Nobody reported ad_quiet: their page is the list, a made-up notice is
	// dropped on the way, and a disable or an enable never reaches them.
	sentToList("an unreported person", adminDo(t, w, http.MethodGet, "/admin/people/"+quiet.id, cookie, nil, ""), "/admin")
	sentToList("a made-up notice", adminDo(t, w, http.MethodGet, "/admin/people/"+quiet.id+"?n=%3Cb%3E", cookie, nil, ""), "/admin")
	sentToList("no such account", adminDo(t, w, http.MethodGet, "/admin/people/"+uuid.NewString(), cookie, nil, ""), "/admin")
	for _, action := range []string{"disable", "enable"} {
		sentToList(action+" an unreported person",
			adminDo(t, w, http.MethodPost, "/admin/people/"+quiet.id+"/"+action, cookie, url.Values{"csrf": {csrf}}, ""), "/admin")
	}
	w.ok(quiet, http.MethodGet, "/v1/me", nil)

	// ad_rep was reported: their page opens without an email address, and
	// still opens once the report is dismissed.
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+reported.id, cookie, nil, "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "@ad_rep") || !strings.Contains(page.Body.String(), "Remy Reported") {
		t.Fatalf("a reported person: %d %s", page.Code, page.Body.String())
	}
	if strings.Contains(page.Body.String(), "example.com") {
		t.Fatalf("the person page shows an email address: %s", page.Body.String())
	}
	rec := adminDo(t, w, http.MethodPost, "/admin/reports/dismiss", cookie,
		url.Values{"csrf": {csrf}, "kind": {"person"}, "id": {a.id}, "person": {reported.id}}, "")
	if rec.Header().Get("Location") != "/admin/people/"+reported.id+"?n=dismissed" {
		t.Fatalf("dismiss: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if after := adminDo(t, w, http.MethodGet, rec.Header().Get("Location"), cookie, nil, "").Body.String(); !strings.Contains(after, "Dismissed. Its reports are closed.") || !strings.Contains(after, "Closed reports") {
		t.Fatalf("after the dismissal: %s", after)
	}

	// A report about a post or a comment opens its owner's page too. The
	// reporter's own page stays closed.
	poster := w.person("ad_poster", "Pia Poster")
	commenter := w.person("ad_commenter", "Cy Commenter")
	w.followAccepted(a, poster)
	w.followAccepted(commenter, a)
	const day = "2026-10-08"
	posterEntry := w.post(poster, day)
	aEntry := w.post(a, day)
	w.post(commenter, day)
	w.ok(a, http.MethodPost, "/v1/posts/"+posterEntry+"/report", map[string]any{"reason": "spam"})
	comment := w.call(commenter, http.MethodPost, "/v1/posts/"+aEntry+"/comments", map[string]any{"text": "rude"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	w.ok(a, http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": "harassment"})
	for _, p := range []feedPerson{poster, commenter} {
		if page := adminDo(t, w, http.MethodGet, "/admin/people/"+p.id, cookie, nil, ""); page.Code != http.StatusOK {
			t.Fatalf("%s, reported through a post or a comment: %d", p.name, page.Code)
		}
	}
	sentToList("the reporter", adminDo(t, w, http.MethodGet, "/admin/people/"+a.id, cookie, nil, ""), "/admin")

	// A disabled account opens with no report left about it (its only
	// reporter deleted their account), so the disabled list can enable it.
	// Once enabled it still opens: the decision log names it, and its
	// history says what was decided, newest first.
	gone := w.person("ad_gone", "Gale Gone")
	w.ok(a, http.MethodPost, "/v1/people/"+gone.id+"/report", map[string]any{"reason": "harassment"})
	rec = adminDo(t, w, http.MethodPost, "/admin/people/"+gone.id+"/disable", cookie, url.Values{"csrf": {csrf}}, "")
	if rec.Header().Get("Location") != "/admin/people/"+gone.id+"?n=disabled" {
		t.Fatalf("disable a reported person: %q", rec.Header().Get("Location"))
	}
	if _, err := pool.Exec(context.Background(), `delete from user_reports where reported_id = $1`, gone.id); err != nil {
		t.Fatal(err)
	}
	if list := adminDo(t, w, http.MethodGet, "/admin/disabled", cookie, nil, "").Body.String(); !strings.Contains(list, `href="/admin/people/`+gone.id+`"`) {
		t.Fatalf("the Disabled tab lacks them: %s", list)
	}
	if page := adminDo(t, w, http.MethodGet, "/admin/people/"+gone.id, cookie, nil, ""); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Enable account") {
		t.Fatalf("a disabled account no report names: %d %s", page.Code, page.Body.String())
	}
	rec = adminDo(t, w, http.MethodPost, "/admin/people/"+gone.id+"/enable", cookie, url.Values{"csrf": {csrf}}, "")
	if rec.Header().Get("Location") != "/admin/people/"+gone.id+"?n=enabled" {
		t.Fatalf("enable: %q", rec.Header().Get("Location"))
	}
	enabled := adminDo(t, w, http.MethodGet, rec.Header().Get("Location"), cookie, nil, "")
	if body := enabled.Body.String(); enabled.Code != http.StatusOK || !strings.Contains(body, "Account enabled again.") ||
		!strings.Contains(body, "<strong>Account enabled</strong>") ||
		strings.Index(body, "<strong>Account enabled</strong>") > strings.Index(body, "<strong>Account disabled</strong>") {
		t.Fatalf("an enabled account no report names, want its page with the notice and its history: %d %s", enabled.Code, body)
	}
	// A page that does not open keeps a fixed notice on the way to the list.
	sentToList("an unreported person with a fixed notice", adminDo(t, w, http.MethodGet, "/admin/people/"+quiet.id+"?n=enabled", cookie, nil, ""), "/admin?n=enabled")
	if list := adminDo(t, w, http.MethodGet, "/admin?n=enabled", cookie, nil, "").Body.String(); !strings.Contains(list, "Account enabled again.") {
		t.Fatalf("the list lacks the notice: %s", list)
	}
	if login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": "ad_gone@example.com", "password": adminTestPassword}); login.Code != http.StatusOK {
		t.Fatalf("log in once enabled: %d", login.Code)
	}
}

// adminPost spins and posts p's entry for date at 8:30 PM, as entryRequest
// writes it with one photo per key in order, and returns its id.
func adminPost(w *feedWorld, p feedPerson, date string, keys ...string) string {
	w.t.Helper()
	w.at(date, 20, 30)
	spun := w.ok(p, http.MethodPost, "/v1/spin", map[string]any{"for_date": date})
	body := entryRequest(date, int(dig(w.t, spun, "today", "spin", "seq").(float64)), keys...)
	return entryObject(w.t, w.call(p, http.MethodPost, "/v1/entries", body, http.StatusCreated)["entry"])["id"].(string)
}

// reasonList is the Reason list beside Remove, which names all seven report
// reasons on every open post and comment card.
var reasonList = regexp.MustCompile(`(?s)<label class="reason">.*?</label>`)

// adminReportOf returns the one report card on an admin page that holds
// mark: from its opening tag to the next card's (or the page's end). The
// mark is looked for outside the card's Reason list, which is the same on
// every card, so a reason's words find the card reported for it.
func adminReportOf(t *testing.T, page, mark string) string {
	t.Helper()
	var found []string
	for _, part := range strings.Split(page, `<article class="card`)[1:] {
		if strings.Contains(reasonList.ReplaceAllString(part, ""), mark) {
			found = append(found, part)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d reports hold %q, want 1: %s", len(found), mark, page)
	}
	return found[0]
}

// A post report shows the whole post as it is now, so the admin can check it
// before acting: its title, its day, its text (escaped, and edited since) and
// every photo in order, each opening the photo itself in a new tab. The title
// was edited since the report, so the one it had then shows too, and both
// are labelled. Nothing of the owner's other posts.
func TestAdminIntegration_APostReportShowsTheWholePost(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("ad_alice", "Alice Ames")
	b := w.person("ad_bob", "Bob Brown")
	w.followAccepted(a, b)
	const day = "2026-10-08"
	w.post(a, day)
	// Three photos whose keys sort in another order than their places.
	photos := []string{
		"entries/" + b.id + "/ffffffffffffffff.jpg",
		"entries/" + b.id + "/0000000000000000.png",
		"entries/" + b.id + "/8888888888888888.webp",
	}
	bEntry := adminPost(w, b, day, photos...)
	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "nudity"})
	// Since then B edited the post, and posted the next day.
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"title": "Dune, again", "comment": "<b>Three</b> photos\nand a note"})
	other := "entries/" + b.id + "/1111111111111111.jpg"
	adminPost(w, b, "2026-10-09", other)

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "")
	if page.Code != http.StatusOK {
		t.Fatalf("the page: %d %s", page.Code, page.Body.String())
	}
	report := adminReportOf(t, page.Body.String(), "Nudity or sexual content")
	for _, want := range []string{
		`<h3>Dune, again</h3>`,
		`<p class="by">Title when reported: Dune</p>`,
		"&lt;b&gt;Three&lt;/b&gt; photos\nand a note",
		`<p class="day">Oct 8, 2026</p>`,
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("the report lacks %q: %s", want, report)
		}
	}
	for _, bad := range []string{"<b>Three", "Slow and worth it."} {
		if strings.Contains(report, bad) {
			t.Fatalf("the report has %q: %s", bad, report)
		}
	}
	last := -1
	for i, key := range photos {
		// One signed link per photo, for both the full size link and the
		// picture.
		link := fakePhotoLink(key)
		at := strings.Index(report, `<a href="`+link+`" target="_blank" rel="noopener"><img class="thumb" src="`+link+`"`)
		if at <= last || strings.Count(report, key) != 2 {
			t.Fatalf("photo %d (%s) is missing, out of order or not signed once for both: %s", i+1, key, report)
		}
		last = at
	}
	if n := strings.Count(report, `<img class="thumb"`); n != len(photos) || strings.Contains(report, other) {
		t.Fatalf("the report shows %d photos, want only the post's %d: %s", n, len(photos), report)
	}
}

// A report made after a post's title changed saves the new title: the post's
// one card, which shows its newest report, has that title once, with no
// label. The first reporter cannot report it again: the post is gone for
// them, so that report is answered like a miss and stays as it was.
func TestAdminIntegration_AReportAfterAnEditSavesTheNewTitle(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("ad_alice", "Alice Ames")
	b := w.person("ad_bob", "Bob Brown")
	c := w.person("ad_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	w.post(a, day)
	w.post(c, day)
	bEntry := w.post(b, day)
	w.ok(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "spam"})
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"title": "Dune, again"})
	w.call(a, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "violence"}, http.StatusNotFound)
	// A minute later, so Cara's report is the newest one.
	w.clk.Advance(time.Minute)
	w.ok(c, http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "harassment"})

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "")
	if page.Code != http.StatusOK || strings.Count(page.Body.String(), `<article class="card`) != 1 {
		t.Fatalf("the page, want one card for the post: %d %s", page.Code, page.Body.String())
	}
	report := adminReportOf(t, page.Body.String(), "Bullying or harassment · Spam")
	// The Reason list beside Remove names every reason, so A's is looked for
	// outside it.
	if !strings.Contains(report, `<h3>Dune, again</h3>`) || strings.Contains(reasonList.ReplaceAllString(report, ""), "Violence") ||
		strings.Contains(report, "Title now") || strings.Contains(report, "Title when reported") {
		t.Fatalf("the card, want the new title once, no label, and no second reason from A: %s", report)
	}
}

// A comment report shows the comment's photo, so the admin can check it
// before acting.
func TestAdminIntegration_ACommentReportShowsItsPhoto(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("ad_alice", "Alice Ames")
	b := w.person("ad_bob", "Bob Brown")
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	const day = "2026-10-08"
	aEntry := w.post(a, day)
	w.post(b, day)
	key := "entries/" + b.id + "/abcdefabcdefabcd.jpg"
	c := w.call(b, http.MethodPost, "/v1/posts/"+aEntry+"/comments",
		map[string]any{"text": "look", "photo_key": key}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	w.ok(a, http.MethodPost, "/v1/comments/"+c+"/report", map[string]any{"reason": "harassment"})
	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "")
	if page.Code != http.StatusOK {
		t.Fatalf("the page: %d %s", page.Code, page.Body.String())
	}
	report := adminReportOf(t, page.Body.String(), "Bullying or harassment")
	link := fakePhotoLink(key)
	if !strings.Contains(report, `<a href="`+link+`" target="_blank" rel="noopener"><img class="thumb" src="`+link+`"`) ||
		strings.Count(report, key) != 2 {
		t.Fatalf("the comment report lacks its photo, signed once for both the link and the picture: %s", report)
	}
}

// A reported post that is gone (its owner deleted it) keeps the title it had
// when it was reported and says it is gone, with no day, text or photo, not
// even one of the owner's other posts.
func TestAdminIntegration_AGonePostShowsNoPhoto(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("ad_alice", "Alice Ames")
	g := w.person("ad_gone", "Gale Gone")
	w.followAccepted(a, g)
	const day = "2026-10-08"
	w.post(a, day)
	gone := w.post(g, day)
	w.ok(a, http.MethodPost, "/v1/posts/"+gone+"/report", map[string]any{"reason": "spam"})
	w.ok(g, http.MethodPost, "/v1/entries/"+gone+"/delete", map[string]any{})
	w.post(g, "2026-10-09")

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+g.id, cookie, nil, "")
	if page.Code != http.StatusOK {
		t.Fatalf("the page: %d %s", page.Code, page.Body.String())
	}
	report := adminReportOf(t, page.Body.String(), "The post is gone.")
	if !strings.Contains(report, `<h3>Dune</h3>`) {
		t.Fatalf("the report lacks the title: %s", report)
	}
	for _, bad := range []string{"<img", fakePhotoLinkBase, `class="day"`, "Slow and worth it."} {
		if strings.Contains(report, bad) {
			t.Fatalf("the gone post's report has %q: %s", bad, report)
		}
	}
}

// The person page shows the profile photo when there is one, and works the
// same without it.
func TestAdminIntegration_ThePersonPageShowsTheProfilePhoto(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("ad_alice", "Alice Ames")
	pictured := w.person("ad_pictured", "Pat Pictured")
	plain := w.person("ad_plain", "Nat Plain")
	photo := "avatars/" + pictured.id + "/" + fakeAvatarKeyName
	w.ok(pictured, http.MethodPatch, "/v1/me", map[string]any{"avatar_key": photo})
	for _, p := range []feedPerson{pictured, plain} {
		w.ok(a, http.MethodPost, "/v1/people/"+p.id+"/report", map[string]any{"reason": "impersonation"})
	}

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+pictured.id, cookie, nil, "")
	link := fakePhotoLink(photo)
	if page.Code != http.StatusOK ||
		!strings.Contains(page.Body.String(), `<a href="`+link+`" target="_blank" rel="noopener"><img class="avatar" src="`+link+`"`) ||
		strings.Count(page.Body.String(), photo) != 2 {
		t.Fatalf("a person with a profile photo, signed once for both the link and the picture: %d %s", page.Code, page.Body.String())
	}
	page = adminDo(t, w, http.MethodGet, "/admin/people/"+plain.id, cookie, nil, "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Nat Plain") ||
		strings.Contains(page.Body.String(), `class="avatar"`) {
		t.Fatalf("a person without one: %d %s", page.Code, page.Body.String())
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

// adminLinkFrom opens a `make admin` link from remote, and returns the answer
// and the session cookie it set ("" for none).
func adminLinkFrom(t *testing.T, w *feedWorld, remote, token string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/link?token="+url.QueryEscape(token), nil)
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	w.engine.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookie && c.Value != "" {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/admin" {
				t.Fatalf("cookie flags: %+v", c)
			}
			return rec, c.Value
		}
	}
	return rec, ""
}

// A link from `make admin` (cmd/admin-link) opens the page with the usual
// session for the admin it names, or for the first admin when it names none,
// and works once. A link that was used, has ended or is signed with another
// secret lands on the log in page with one notice; one that names an account
// that is no admin with another that says what to check, and one that names
// no account with a third that says so. None sets a cookie.
func TestAdminIntegration_MakeAdminLinkOpensThePageOnce(t *testing.T) {
	w, _ := newAdminWorld(t)
	w.person("ad_linker", "Lee Linker")
	now := frozenClock()
	mint := func(secret []byte, email string, at time.Time) string {
		t.Helper()
		token, err := adminlink.New(secret, email, at)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	// Each try from its own address, so the per-address limit never answers.
	n := 0
	from := func() string {
		n++
		return fmt.Sprintf("198.51.100.%d:1", 100+n)
	}
	refused := func(name string, rec *httptest.ResponseRecorder, cookie, notice string) {
		t.Helper()
		if cookie != "" || rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n="+notice {
			t.Fatalf("%s: %d %q, cookie %q, want %s", name, rec.Code, rec.Header().Get("Location"), cookie, notice)
		}
	}

	for _, email := range []string{"", "AD_Boss@example.com"} {
		token := mint(integrationJWTSecret, email, now)
		rec, cookie := adminLinkFrom(t, w, from(), token)
		if rec.Code != http.StatusOK || cookie == "" || !strings.Contains(rec.Body.String(), `<meta http-equiv="refresh" content="0;url=/admin">`) {
			t.Fatalf("a link for %q: %d, cookie %q, %s", email, rec.Code, cookie, rec.Body.String())
		}
		if page := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, ""); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `aria-current="page">Reports`) {
			t.Fatalf("the session a link for %q opened: %d", email, page.Code)
		}
		again, cookie := adminLinkFrom(t, w, from(), token)
		refused("a used link", again, cookie, "badlink")
	}

	for _, c := range []struct{ name, token, notice string }{
		{"an ended link", mint(integrationJWTSecret, "", now.Add(-adminlink.TTL)), "badlink"},
		{"another secret", mint([]byte("another-secret-that-is-at-least-32-bytes"), "", now), "badlink"},
		{"not an admin", mint(integrationJWTSecret, "ad_linker@example.com", now), "noadmin"},
		{"no account", mint(integrationJWTSecret, "ad_nobody@example.com", now), "noaccount"},
		{"garbled", "x.y.z.w", "badlink"},
		{"no token", "", "badlink"},
	} {
		rec, cookie := adminLinkFrom(t, w, from(), c.token)
		refused(c.name, rec, cookie, c.notice)
	}

	// The log in page names the way in, and a refused link says what to do.
	for notice, words := range map[string]string{
		"badlink":       "That link expired or was already used. Run make admin for a new one.",
		"noadmin":       "No admin account for that link. Check ADMIN_EMAILS, make the account if needed, then restart the server.",
		"noaccount":     "No account uses the admin email yet.",
		"admindisabled": "The admin account is disabled.",
		"admindeleting": "The admin account is set to be deleted. Log in to the app to keep it.",
	} {
		login := adminDo(t, w, http.MethodGet, "/admin/login?n="+notice, "", nil, "").Body.String()
		for _, want := range []string{"Run <code>make admin</code> in the RNDMRoll folder", words} {
			if !strings.Contains(login, want) {
				t.Fatalf("the log in page with %s lacks %q", notice, want)
			}
		}
	}
}

// A good link that opens no admin says why, on the real database. An
// ADMIN_EMAILS address that had no account when the server started is no
// account, whether a link names it or a server that found no admin gets a
// link that names nobody. The admin set to be deleted gets its notice until
// a log in to the app keeps the account, the admin disabled gets its own,
// and past the grace period of a deletion the plain one comes back. Log in
// with the right password says no more than a wrong one.
func TestAdminIntegration_AGoodLinkForNoAdminSaysWhy(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	const bossEmail, lateEmail = "aw_boss@example.com", "aw_late@example.com"
	boss := adminFirst(w, pool, integrationOptions{clock: clk, adminEmails: []string{bossEmail, lateEmail}}, "aw_boss")
	// Each try from its own address, so no limit answers.
	n := 0
	from := func() string {
		n++
		return fmt.Sprintf("198.51.100.%d:1", 140+n)
	}
	link := func(on *feedWorld, email string) (*httptest.ResponseRecorder, string) {
		t.Helper()
		token, err := adminlink.New(integrationJWTSecret, email, frozenClock())
		if err != nil {
			t.Fatal(err)
		}
		return adminLinkFrom(t, on, from(), token)
	}
	refused := func(name string, rec *httptest.ResponseRecorder, cookie, notice string) {
		t.Helper()
		if cookie != "" || rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n="+notice {
			t.Fatalf("%s: %d %q, cookie %q, want %s", name, rec.Code, rec.Header().Get("Location"), cookie, notice)
		}
	}
	vague := func(name string) {
		t.Helper()
		if loc, cookie := adminLoginFrom(t, w, from(), bossEmail, adminTestPassword); loc != "/admin/login?n=badlogin" || cookie != "" {
			t.Fatalf("%s, log in with the right password: %q (cookie given: %v), want badlogin", name, loc, cookie != "")
		}
	}
	setBoss := func(name, set string, args ...any) {
		t.Helper()
		tag, err := pool.Exec(context.Background(), "update users set "+set+" where id = $1", append([]any{uuid.MustParse(boss.id)}, args...)...)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("%s: %d rows, err %v", name, tag.RowsAffected(), err)
		}
	}

	rec, cookie := link(w, lateEmail)
	refused("a link for the address with no account", rec, cookie, "noaccount")
	noAdmin := &feedWorld{t: t, clk: clk,
		engine: buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, adminEmails: []string{lateEmail}}).Engine()}
	rec, cookie = link(noAdmin, "")
	refused("a link on a server that found no admin", rec, cookie, "noaccount")

	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/me/delete", boss.token, map[string]any{"password": adminTestPassword}); rec.Code != http.StatusOK {
		t.Fatalf("the admin asks for the account to be deleted: %d %s", rec.Code, rec.Body.String())
	}
	rec, cookie = link(w, "")
	refused("an admin set to be deleted", rec, cookie, "admindeleting")
	vague("an admin set to be deleted")
	login := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": bossEmail, "password": adminTestPassword})
	if login.Code != http.StatusOK || decodeBody(t, login)["restored"] != true {
		t.Fatalf("the admin logs in to the app: %d %s, want 200 with restored true", login.Code, login.Body.String())
	}
	if rec, cookie := link(w, ""); rec.Code != http.StatusOK || cookie == "" {
		t.Fatalf("a link once the account is kept: %d (cookie given: %v)", rec.Code, cookie != "")
	}

	setBoss("disable the admin", "disabled_at = now()")
	rec, cookie = link(w, bossEmail)
	refused("a disabled admin", rec, cookie, "admindisabled")
	vague("a disabled admin")

	setBoss("move a new ask past the grace period", "disabled_at = null, deletion_requested_at = now() - $2::interval", deletionPastGrace)
	rec, cookie = link(w, "")
	refused("an admin past the grace period", rec, cookie, "noadmin")
}

// A new password ends the admin session, as it ends the app's other
// sessions, whichever way it comes: Change password in the app, a Forgot
// password reset, or `make password` (UserRepo.SetPassword). Each time the
// old cookie goes back to log in, and the new password logs in.
func TestAdminIntegration_ANewPasswordEndsTheAdminSession(t *testing.T) {
	pool := requireIntegrationPool(t)
	w, boss := newAdminWorldOn(t, pool)
	const email = "ad_boss@example.com"
	// Each log in from its own address, so no limit answers.
	n := 0
	login := func(password string) string {
		t.Helper()
		n++
		_, cookie := adminLoginFrom(t, w, fmt.Sprintf("198.51.100.%d:1", 120+n), email, password)
		if cookie == "" {
			t.Fatalf("the admin could not log in with %q", password)
		}
		if rec := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, ""); rec.Code != http.StatusOK {
			t.Fatalf("a new session: %d", rec.Code)
		}
		return cookie
	}
	ended := func(how, cookie string) {
		t.Helper()
		rec := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n=expired" {
			t.Fatalf("the admin session after %s: %d %q, want 303 to /admin/login?n=expired", how, rec.Code, rec.Header().Get("Location"))
		}
	}

	cookie := login(adminTestPassword)
	const changed = "Changed-horse-battery-staple2"
	w.ok(boss, http.MethodPost, "/v1/me/password", map[string]any{"current_password": adminTestPassword, "new_password": changed})
	ended("Change password", cookie)

	// Forgot password, through its own routes on the same database (as
	// newResetWorld wires them, which would empty it first).
	cookie = login(changed)
	sender := &captureSender{}
	reset := NewPasswordResetHandler(postgres.NewUserRepo(pool), auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), time.Hour),
		postgres.NewPasswordResetRepo(pool), sender, integrationJWTSecret, 15*time.Minute)
	reset.floor = 0
	t.Cleanup(reset.Wait)
	routes := gin.New()
	reset.Register(routes.Group("/v1"))
	resetCall := func(path string, body map[string]any) {
		t.Helper()
		rec := integrationDo(t, routes, http.MethodPost, path, "", body)
		reset.Wait()
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	resetCall("/v1/auth/password/forgot", map[string]any{"email": email})
	code := sender.lastCode(t, email)
	resetCall("/v1/auth/password/verify", map[string]any{"email": email, "code": code})
	const resetTo = "Reset-horse-battery-staple3"
	resetCall("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": resetTo})
	ended("a Forgot password reset", cookie)

	cookie = login(resetTo)
	const setTo = "Set-horse-battery-staple4"
	hash, err := auth.HashPassword(setTo)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.NewUserRepo(pool).SetPassword(context.Background(), uuid.MustParse(boss.id), hash); err != nil {
		t.Fatal(err)
	}
	ended("make password", cookie)
	login(setTo)
}

// hiddenLogged is the hides by reports of owner's posts and comments in the
// log, oldest first, a row as its item, item id and post.
func hiddenLogged(t *testing.T, pool *pgxpool.Pool, owner string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select item || ' ' || item_id::text || ' on ' || coalesce(entry_id::text, '-')
		from moderation_events where owner_id = $1 and action = 'hidden' order by created_at, id`, owner)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// loggedActions is the admin's decisions about owner in the log, oldest
// first, a row as its action, item and item id. The hides by reports are left
// out: hiddenLogged reads those.
func loggedActions(t *testing.T, pool *pgxpool.Pool, owner string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select action || ' ' || item || ' ' || item_id::text
		from moderation_events where owner_id = $1 and action <> 'hidden' order by created_at, id`, owner)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// adminReportID is the report id an open report's forms carry.
func adminReportID(t *testing.T, report string) string {
	t.Helper()
	const mark = `name="id" value="`
	i := strings.Index(report, mark)
	if i < 0 {
		t.Fatalf("the report has no form: %s", report)
	}
	return report[i+len(mark) : i+len(mark)+36]
}

// selectedReason is a Reason list's selected option.
var selectedReason = regexp.MustCompile(`<option value="([a-z]+)" selected>`)

// adminRemoveReason is the reason an open report card's Remove form sends:
// the one its Reason list has selected. The test fails unless the card has
// a Remove form with exactly one reason selected.
func adminRemoveReason(t *testing.T, report string) string {
	t.Helper()
	start := strings.Index(report, `action="/admin/reports/remove"`)
	if start < 0 {
		t.Fatalf("the report has no Remove form: %s", report)
	}
	form, _, _ := strings.Cut(report[start:], "</form>")
	picked := selectedReason.FindAllStringSubmatch(form, -1)
	if len(picked) != 1 {
		t.Fatalf("the Remove form has %d reasons selected, want 1: %s", len(picked), report)
	}
	return picked[0][1]
}

// Remove closes every report still open on the post or comment, whoever made
// it, and leaves a report already dismissed as it was. With nothing left
// open, the person leaves the open reports. The post's three reports hid it,
// so their dismissal shows it again, once, and says so.
func TestAdminIntegration_RemoveClosesEveryReportOpenOnIt(t *testing.T) {
	pool := requireIntegrationPool(t)
	w, _ := newAdminWorldOn(t, pool)
	b := w.person("ad_bob", "Bob Brown")
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	comment := w.call(b, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"text": "rude"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	var reporters []feedPerson
	for _, username := range []string{"ad_alice", "ad_cara", "ad_dana", "ad_erin"} {
		p := w.person(username, "Reporter "+username)
		w.followAccepted(p, b)
		w.post(p, day)
		reporters = append(reporters, p)
	}
	// The comment first: once its reporters report the post too, the post is
	// gone for them, and with three reports it is hidden from everyone but B.
	for i, reason := range []string{"harassment", "nudity"} {
		w.ok(reporters[i], http.MethodPost, "/v1/comments/"+comment+"/report", map[string]any{"reason": reason})
	}
	for i, reason := range []string{"spam", "hate", "violence"} {
		w.ok(reporters[i], http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": reason})
	}

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	if cookie == "" {
		t.Fatal("the admin could not log in")
	}
	personPage := func() string {
		t.Helper()
		return adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	}
	// act sends a card's own form: Remove with the id it carries and the
	// reason it has selected, Dismiss with every id on the card.
	act := func(action, kind, mark, notice string) {
		t.Helper()
		page := personPage()
		card := adminReportOf(t, page, mark)
		form := url.Values{"csrf": {csrfOf(t, page)}, "kind": {kind}, "person": {b.id}}
		if action == "dismiss" {
			for _, m := range dismissIDs.FindAllStringSubmatch(card[strings.Index(card, `action="/admin/reports/dismiss"`):], -1) {
				form.Add("id", m[1])
			}
		} else {
			form.Set("id", adminReportID(t, card))
			form.Set("reason", adminRemoveReason(t, card))
		}
		rec := adminDo(t, w, http.MethodPost, "/admin/reports/"+action, cookie, form, "")
		if rec.Header().Get("Location") != "/admin/people/"+b.id+"?n="+notice {
			t.Fatalf("%s the card %q: %d %q, want %s", action, mark, rec.Code, rec.Header().Get("Location"), notice)
		}
	}
	// The post's three reports are one card, and the comment's two another.
	if n := strings.Count(personPage(), `<article class="card`); n != 2 {
		t.Fatalf("%d cards, want one for the post and one for the comment", n)
	}
	act("dismiss", "post", "Violence or threats · Hate speech · Spam", "shownagain")
	if got := loggedActions(t, pool, b.id); !slices.Equal(got, []string{"shown_again post " + bEntry}) {
		t.Fatalf("the log after the dismissal: %v, want the post shown again once", got)
	}
	// Alice cannot report the post again: it is gone for her for good. Erin,
	// who never reported it, does: her report is open, alone on a card.
	w.call(reporters[0], http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "violence"}, http.StatusNotFound)
	w.ok(reporters[3], http.MethodPost, "/v1/posts/"+bEntry+"/report", map[string]any{"reason": "violence"})
	act("remove", "comment", "Nudity or sexual content · Bullying or harassment", "removed")
	act("remove", "post", `Violence or threats</p><p class="kind">Post`, "removed")

	// Each closed card is one action: what Remove closed, and the three
	// reports dismissed before, which it left as they were.
	page := personPage()
	for mark, closed := range map[string]string{
		`Violence or threats</p><p class="kind">Post`:       "Removed",
		"Violence or threats · Hate speech · Spam</p>":      "Dismissed",
		"Nudity or sexual content · Bullying or harassment": "Removed",
	} {
		if card := adminReportOf(t, page, mark); !strings.Contains(card, `<p class="by">`+closed+` · `) {
			t.Fatalf("the card %q, want %s: %s", mark, closed, card)
		}
	}
	if strings.Contains(page, "<button>Dismiss</button>") || strings.Contains(page, "Remove post") || strings.Contains(page, "Remove comment") {
		t.Fatalf("a closed report still offers an action: %s", page)
	}
	if list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String(); !strings.Contains(list, "No open reports.") {
		t.Fatalf("the list still has open reports: %s", list)
	}
	// Every reporter lost the post when they reported it, so its owner shows
	// that it is gone.
	w.call(b, http.MethodGet, "/v1/posts/"+bEntry, nil, http.StatusNotFound)
}
