package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/adminlink"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// adminUnitSecret signs the unit rig's sessions and links (a link's secret
// must be at least 32 bytes).
var adminUnitSecret = []byte("admin-unit-test-secret-at-least-32-bytes")

// fakeAdminRepo is admin.Repository in memory, for the admin page's unit
// tests: the test sets what the calls answer, and the calls that change
// something are recorded.
type fakeAdminRepo struct {
	// target and owner are what ReportTarget answers for any report, and
	// open is what OpenReportIDs answers.
	target *uuid.UUID
	owner  uuid.UUID
	open   []uuid.UUID
	// commentKeys and deleteErr are what DeleteComment answers; closeErr is
	// what ResolveIDs answers.
	commentKeys []string
	deleteErr   error
	closeErr    error

	reads    int         // Summaries, DisabledPeople, Person and Reports
	resolved int         // Resolve
	deleted  []uuid.UUID // DeleteComment
	closed   []adminClose
}

// adminClose is one ResolveIDs call.
type adminClose struct {
	kind       string
	ids        []uuid.UUID
	resolution string
}

var _ admin.Repository = (*fakeAdminRepo)(nil)

func (f *fakeAdminRepo) Summaries(context.Context) ([]admin.Summary, error) {
	f.reads++
	return []admin.Summary{}, nil
}

func (f *fakeAdminRepo) DisabledPeople(context.Context) ([]admin.Summary, error) {
	f.reads++
	return []admin.Summary{}, nil
}

func (f *fakeAdminRepo) Person(_ context.Context, id uuid.UUID) (admin.Person, error) {
	f.reads++
	return admin.Person{ID: id, Username: "someone"}, nil
}

func (f *fakeAdminRepo) Reports(context.Context, uuid.UUID) ([]admin.Report, error) {
	f.reads++
	return []admin.Report{}, nil
}

func (f *fakeAdminRepo) Resolve(context.Context, string, uuid.UUID, uuid.UUID, string, time.Time) (bool, error) {
	f.resolved++
	return true, nil
}

func (f *fakeAdminRepo) OpenReportIDs(context.Context, string, uuid.UUID) ([]uuid.UUID, error) {
	return f.open, nil
}

func (f *fakeAdminRepo) ResolveIDs(_ context.Context, kind string, ids []uuid.UUID, resolution string, _ time.Time) error {
	f.closed = append(f.closed, adminClose{kind: kind, ids: ids, resolution: resolution})
	return f.closeErr
}

func (f *fakeAdminRepo) ResolvePerson(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func (f *fakeAdminRepo) ReportTarget(context.Context, string, uuid.UUID) (*uuid.UUID, uuid.UUID, error) {
	return f.target, f.owner, nil
}

func (f *fakeAdminRepo) SetDisabled(context.Context, uuid.UUID, *time.Time) error {
	return nil
}

func (f *fakeAdminRepo) DeleteComment(_ context.Context, id uuid.UUID) ([]string, error) {
	f.deleted = append(f.deleted, id)
	return f.commentKeys, f.deleteErr
}

// adminUsers is the fake user repository with a switch: while getErr is set,
// GetByID fails with it, as a database that cannot be reached does.
type adminUsers struct {
	*fakeUserRepo
	getErr error
}

func (u *adminUsers) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	if u.getErr != nil {
		return nil, u.getErr
	}
	return u.fakeUserRepo.GetByID(ctx, id)
}

// adminRig is the admin page over fakes, on the frozen limit clock, with one
// account, boss@example.com, whose password is adminTestPassword. A post is
// deleted through a spin service over the in-memory spin repository.
type adminRig struct {
	t      *testing.T
	h      *AdminHandler
	w      *feedWorld // only its engine, for adminDo and the other helpers
	users  *adminUsers
	repo   *fakeAdminRepo
	spin   *spin.Service
	photos *fakeEntryPhotoStore
	boss   *user.User
}

// newAdminRig mounts the page. bossIsAdmin says whether boss@example.com was
// found as an admin when the server started; when not, there is no admin.
func newAdminRig(t *testing.T, bossIsAdmin bool) *adminRig {
	t.Helper()
	users := &adminUsers{fakeUserRepo: newFakeUserRepo()}
	r := &adminRig{t: t, users: users, repo: &fakeAdminRepo{}, photos: &fakeEntryPhotoStore{}}
	r.boss = users.seedComplete(t, "boss@example.com", "Boss Admin", "boss", validBirthday)
	hash, err := auth.HashPassword(adminTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	setPasswordHash(users.fakeUserRepo, r.boss.ID, hash)
	var admins []uuid.UUID
	if bossIsAdmin {
		admins = []uuid.UUID{r.boss.ID}
	}
	r.spin = spin.NewService(spintest.NewMemRepo(), clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)), spintest.NewQueueRand(1))
	r.h = NewAdminHandler(r.repo, users, NewEntryHandler(r.spin, r.photos), adminUnitSecret, admins, false)
	r.h.clock = frozenClock
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r.h.Register(engine)
	r.w = &feedWorld{t: t, engine: engine}
	return r
}

// session is a session cookie for the boss signed at at, as log in makes
// one: tied to the password the account has now.
func (r *adminRig) session(at time.Time) string {
	r.t.Helper()
	u, err := r.users.fakeUserRepo.GetByID(context.Background(), r.boss.ID)
	if err != nil {
		r.t.Fatal(err)
	}
	return r.h.newSession(u.ID, r.h.passwordMark(u), at)
}

// setPasswordHash stores hash as the account's password hash, as a new
// password does.
func setPasswordHash(f *fakeUserRepo, id uuid.UUID, hash string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[id].PasswordHash = &hash
}

// adminCookieOf is the admin cookie rec sets or clears, nil when it does
// neither.
func adminCookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookie {
			return c
		}
	}
	return nil
}

// sentToLogin fails unless rec sends the browser to the log in page with
// notice and, when cleared, clears the admin cookie.
func sentToLogin(t *testing.T, name string, rec *httptest.ResponseRecorder, notice string, cleared bool) {
	t.Helper()
	c := adminCookieOf(rec)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n="+notice ||
		cleared != (c != nil && c.Value == "" && c.MaxAge < 0) {
		t.Fatalf("%s: %d %q, cookie %+v, want the log in page with %q (cookie cleared: %v)",
			name, rec.Code, rec.Header().Get("Location"), c, notice, cleared)
	}
}

// A new password ends the admin session, as it ends the app's other
// sessions: the cookie carries a mark of the password hash, so the session
// signed before the change goes back to log in and its cookie is cleared,
// even for an action, while one signed since works. A cookie from before the
// mark works no more.
func TestAdminSession_ANewPasswordEndsIt(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	if rec := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("the session: %d %s", rec.Code, rec.Body.String())
	}

	setPasswordHash(r.users.fakeUserRepo, r.boss.ID, "the-new-password-hash")
	sentToLogin(t, "the list after a new password", adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, ""), "expired", true)
	form := url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {admin.KindPerson}, "id": {uuid.NewString()}, "person": {uuid.NewString()}}
	sentToLogin(t, "a dismiss after a new password", adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", cookie, form, ""), "expired", true)
	if r.repo.resolved != 0 {
		t.Fatal("the old session dismissed a report")
	}

	if rec := adminDo(t, r.w, http.MethodGet, "/admin", r.session(time.Now()), nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("a session signed after the new password: %d", rec.Code)
	}
	payload := r.boss.ID.String() + "." + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	old := payload + "." + r.h.sign("session", payload)
	sentToLogin(t, "a cookie from before the mark", adminDo(t, r.w, http.MethodGet, "/admin", old, nil, ""), "expired", false)
}

// Only a definite no ends the admin session: when the account cannot be
// read, the page and an action behind it answer the failure page, run
// nothing and leave the cookie, which works once the database is back. No
// such account still ends it.
func TestAdminSession_ALookupThatFailsKeepsIt(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	r.users.getErr = errors.New("connection reset by peer")
	form := url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {admin.KindPerson}, "id": {uuid.NewString()}, "person": {uuid.NewString()}}
	for _, c := range []struct {
		name, method, path string
		form               url.Values
	}{
		{"the list", http.MethodGet, "/admin", nil},
		{"a dismiss", http.MethodPost, "/admin/reports/dismiss", form},
	} {
		rec := adminDo(t, r.w, c.method, c.path, cookie, c.form, "")
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "Something went wrong. Please try again." || adminCookieOf(rec) != nil {
			t.Fatalf("%s while the account cannot be read: %d %q, cookie %+v, want the failure page and the cookie left alone",
				c.name, rec.Code, rec.Body.String(), adminCookieOf(rec))
		}
	}
	if r.repo.reads != 0 || r.repo.resolved != 0 {
		t.Fatalf("the page behind ran: %d reads, %d dismissals", r.repo.reads, r.repo.resolved)
	}

	r.users.getErr = nil
	if rec := adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("the session once the database is back: %d", rec.Code)
	}
	r.users.getErr = user.ErrNotFound
	sentToLogin(t, "no such account", adminDo(t, r.w, http.MethodGet, "/admin", cookie, nil, ""), "expired", true)
}

// Remove closes the reports only once the post or comment is gone. A delete
// that fails closes nothing, so the page still offers Remove; a delete that
// works and a close that fails says so. Once both work, every report that
// was open on it is closed as removed, and its photo file goes. A post that
// cannot be deleted, or that is no longer there, closes nothing either.
func TestAdminRemove_ClosesTheReportsOnlyOnceItIsGone(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	owner, target := uuid.New(), uuid.New()
	r.repo.target, r.repo.owner = &target, owner
	r.repo.open = []uuid.UUID{uuid.New(), uuid.New()}
	remove := func(kind string) string {
		t.Helper()
		rec := adminDo(t, r.w, http.MethodPost, "/admin/reports/remove", cookie,
			url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {kind}, "id": {r.repo.open[0].String()}, "person": {owner.String()}}, "")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("remove %s: %d %s", kind, rec.Code, rec.Body.String())
		}
		return rec.Header().Get("Location")
	}
	back := func(notice string) string { return "/admin/people/" + owner.String() + "?n=" + notice }

	r.repo.deleteErr = errors.New("conn closed")
	if got := remove(admin.KindComment); got != back("failed") || len(r.repo.deleted) != 1 || len(r.repo.closed) != 0 {
		t.Fatalf("a delete that fails: %q, %d deletes, closes %v; want failed and nothing closed", got, len(r.repo.deleted), r.repo.closed)
	}

	r.repo.deleteErr, r.repo.closeErr = nil, errors.New("conn closed")
	if got := remove(admin.KindComment); got != back("failed") || len(r.repo.deleted) != 2 || len(r.repo.closed) != 1 {
		t.Fatalf("a close that fails after the delete: %q, %d deletes, closes %v; want failed", got, len(r.repo.deleted), r.repo.closed)
	}

	r.repo.closeErr = nil
	r.repo.commentKeys = []string{"entries/" + owner.String() + "/abcdefabcdefabcd.jpg"}
	if got := remove(admin.KindComment); got != back("removed") {
		t.Fatalf("a remove that works: %q", got)
	}
	want := adminClose{kind: admin.KindComment, ids: r.repo.open, resolution: admin.Removed}
	if last := r.repo.closed[len(r.repo.closed)-1]; last.kind != want.kind || !slices.Equal(last.ids, want.ids) || last.resolution != want.resolution {
		t.Fatalf("closed %+v, want %+v", last, want)
	}
	if got := r.photos.deletes(); !slices.Equal(got, r.repo.commentKeys) {
		t.Fatalf("removed files %v, want %v", got, r.repo.commentKeys)
	}

	// The owner has no time zone, so the post cannot be deleted; then the
	// post is no longer there.
	closes := len(r.repo.closed)
	if got := remove(admin.KindPost); got != back("failed") || len(r.repo.closed) != closes {
		t.Fatalf("a post that cannot be deleted: %q, closes %v", got, r.repo.closed[closes:])
	}
	if _, err := r.spin.SetTimeZone(context.Background(), owner.String(), "America/New_York"); err != nil {
		t.Fatal(err)
	}
	if got := remove(admin.KindPost); got != back("gone") || len(r.repo.closed) != closes {
		t.Fatalf("a post that is no longer there: %q, closes %v", got, r.repo.closed[closes:])
	}
}

// A good link for no admin says what to check, and sets no cookie: when no
// admin was found at the start, and for an account that is no admin, none,
// or a disabled admin. The link is spent all the same, and a bad one still
// gets its own notice.
func TestAdminLink_ForNoAdminSaysWhatToCheck(t *testing.T) {
	link := func(email string) string {
		t.Helper()
		token, err := adminlink.New(adminUnitSecret, email, frozenClock())
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	n := 0
	open := func(r *adminRig, token string) (*httptest.ResponseRecorder, string) {
		t.Helper()
		n++
		return adminLinkFrom(t, r.w, "198.51.100."+strconv.Itoa(n)+":1", token)
	}
	refused := func(name string, rec *httptest.ResponseRecorder, cookie, notice string) {
		t.Helper()
		if cookie != "" || rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n="+notice {
			t.Fatalf("%s: %d %q, cookie %q, want %s", name, rec.Code, rec.Header().Get("Location"), cookie, notice)
		}
	}

	none := newAdminRig(t, false)
	token := link("")
	rec, cookie := open(none, token)
	refused("no admin found at the start", rec, cookie, "noadmin")
	rec, cookie = open(none, token)
	refused("that link again", rec, cookie, "badlink")

	r := newAdminRig(t, true)
	r.users.seedComplete(t, "other@example.com", "Oli Other", "other", validBirthday)
	for _, email := range []string{"other@example.com", "nobody@example.com"} {
		rec, cookie := open(r, link(email))
		refused("a link for "+email, rec, cookie, "noadmin")
	}
	r.users.setDisabled(r.boss.ID)
	rec, cookie = open(r, link(""))
	refused("a disabled admin", rec, cookie, "noadmin")

	page := adminDo(t, r.w, http.MethodGet, "/admin/login?n=noadmin", "", nil, "").Body.String()
	if !strings.Contains(page, adminNotices["noadmin"]) || !strings.Contains(page, "Check ADMIN_EMAILS") {
		t.Fatalf("the log in page lacks the notice: %s", page)
	}
}

// Past 5 a minute from one address, log in and the links answer with the log
// in page asking to wait, not JSON. Each route counts on its own, another
// address is not held back, and the log in page itself shows.
func TestAdminLimits_ArePagesNotJSON(t *testing.T) {
	r := newAdminRig(t, true)
	const loginFrom, linkFrom = "198.51.100.71:1", "198.51.100.72:1"
	for i := range 5 {
		if loc, _ := adminLoginFrom(t, r.w, loginFrom, "nobody@example.com", "Wrong-password-1!"); loc != "/admin/login?n=badlogin" {
			t.Fatalf("log in try %d: %q", i+1, loc)
		}
		if rec, _ := adminLinkFrom(t, r.w, linkFrom, "x.y.z.w"); rec.Header().Get("Location") != "/admin/login?n=badlink" {
			t.Fatalf("link try %d: %d %q", i+1, rec.Code, rec.Header().Get("Location"))
		}
	}
	if loc, cookie := adminLoginFrom(t, r.w, loginFrom, "boss@example.com", adminTestPassword); loc != "/admin/login?n=slowdown" || cookie != "" {
		t.Fatalf("a sixth log in, with the right password: %q (cookie given: %v), want slowdown", loc, cookie != "")
	}
	if rec, _ := adminLinkFrom(t, r.w, linkFrom, "x.y.z.w"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?n=slowdown" ||
		strings.Contains(rec.Body.String(), "rate_limited") {
		t.Fatalf("a sixth link: %d %q %s, want slowdown", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	// Each route its own count, and each address its own.
	if rec, _ := adminLinkFrom(t, r.w, loginFrom, "x.y.z.w"); rec.Header().Get("Location") != "/admin/login?n=badlink" {
		t.Fatalf("a link from the address that used up log in: %q", rec.Header().Get("Location"))
	}
	if _, cookie := adminLoginFrom(t, r.w, "198.51.100.73:1", "boss@example.com", adminTestPassword); cookie == "" {
		t.Fatal("another address could not log in")
	}
	page := adminDo(t, r.w, http.MethodGet, "/admin/login?n=slowdown", "", nil, "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Too many tries. Wait a minute and try again.") {
		t.Fatalf("the log in page: %d %s", page.Code, page.Body.String())
	}
}

// A form from a page opened before the latest log in carries the old
// session's token: it does nothing and goes back to the list, which says the
// page was out of date. The same goes for no token. The current token works,
// and another site's Origin is still refused outright.
func TestAdminCSRF_AnOlderPagesTokenGoesBackToTheList(t *testing.T) {
	r := newAdminRig(t, true)
	at := time.Now()
	first, second := r.session(at), r.session(at.Add(time.Second))
	form := func(cookie string) url.Values {
		return url.Values{"csrf": {r.h.sign("csrf", cookie)}, "kind": {admin.KindPerson}, "id": {uuid.NewString()}, "person": {uuid.NewString()}}
	}
	noToken := form(second)
	noToken.Del("csrf")
	for name, f := range map[string]url.Values{"the first page's token": form(first), "no token": noToken} {
		rec := adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", second, f, "")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin?n=stale" {
			t.Fatalf("%s: %d %q, want the list with stale", name, rec.Code, rec.Header().Get("Location"))
		}
	}
	if r.repo.resolved != 0 {
		t.Fatal("a stale form dismissed a report")
	}
	if rec := adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", second, form(second), "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("another site's Origin: %d", rec.Code)
	}
	if rec := adminDo(t, r.w, http.MethodPost, "/admin/reports/dismiss", second, form(second), ""); !strings.HasSuffix(rec.Header().Get("Location"), "?n=dismissed") || r.repo.resolved != 1 {
		t.Fatalf("the current token: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if list := adminDo(t, r.w, http.MethodGet, "/admin?n=stale", second, nil, "").Body.String(); !strings.Contains(list, "That page was out of date. Try again.") {
		t.Fatalf("the list lacks the notice: %s", list)
	}
}
