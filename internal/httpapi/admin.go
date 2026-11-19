// Package httpapi: admin.go serves the admin web page at /admin (developer,
// 2026-10-18, like Instagram's review tool): open reports grouped by the
// reported person, one person's reports and decision log, what was removed
// in the last 30 days with the second looks its owners asked for, and the
// actions (dismiss a report, remove the reported post or comment, bring it
// back or keep it removed, disable or enable the account).
//
// It is a plain server-rendered page, outside /v1 and its bearer tokens:
//   - Only the accounts that had an ADMIN_EMAILS address when the server
//     started may sign in, with their app password (FindAdminAccounts). An
//     address with no account then cannot be signed up meanwhile. Without
//     ADMIN_EMAILS nothing here is mounted.
//   - Log in counts its tries at an account with the app's log in (5 a
//     minute per address and account), and past that answers as a wrong
//     password does. Log in and the links also allow 5 a minute per address,
//     and past that the log in page asks to wait (perAddress).
//   - `make admin` (cmd/admin-link) opens the page without a password: a
//     link signed with the cookie's own secret, so it opens nothing that
//     secret could not (internal/adminlink). It works for two minutes, once.
//   - The session is an HttpOnly, SameSite=Strict cookie signed for the admin
//     page only (an app token never works here, and this cookie never works
//     in the app). It carries a mark of the account's password, so a new
//     password (changed, reset or set with `make password`) ends it, as it
//     ends the app's other sessions (passwordMark). The allowlist, the
//     disabled flag and that mark are checked again on every request.
//   - Every action is a POST carrying a CSRF token tied to the session, and a
//     cross-site Origin is refused. A token from an older session (a page
//     left open across a new log in) does nothing and goes back to the list.
//   - Pages are html/template (every user-written text is escaped), with a
//     strict Content-Security-Policy and no framing.
//   - It opens only people someone reported, disabled accounts and people
//     the decision log names, and never shows an email address
//     (AdminRepo.Person, developer, 2026-10-29: privacy first). Anyone
//     else's page goes back to the list, and disabling or enabling them is
//     refused the same way. A report names its reporter by username only.
package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/adminlink"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

//go:embed admin_pages/pages.html admin_pages/style.css
var adminFiles embed.FS

const (
	adminCookie     = "rndmroll_admin"
	adminSessionTTL = 8 * time.Hour
)

// The notices a redirect may carry. Only these fixed words are ever shown:
// nothing from the query is echoed.
var adminNotices = map[string]string{
	"dismissed":   "Dismissed. Its reports are closed.",
	"shownagain":  "Dismissed. It shows again, and they were told.",
	"removed":     "Removed. It's on the Removed tab for 30 days.",
	"restored":    "Brought back. They were told.",
	"kept":        "Kept removed. They'll see it in the app.",
	"answered":    "That second look was already answered.",
	"gone":        "That post or comment is already gone.",
	"postfirst":   "Bring back its post first.",
	"parentfirst": "Bring back the comment it answers first.",
	"disabled":    "Account disabled. They are logged out and hidden from everyone.",
	"enabled":     "Account enabled again.",
	"self":        "You can't disable your own account.",
	"failed":      "Something went wrong. Please try again.",
	"signedout":   "You're logged out.",
	"badlogin":    "Wrong email or password, or this account is not an admin.",
	"expired":     "Please log in again.",
	"badlink":     "That link expired or was already used. Run make admin for a new one.",
	"noadmin":     "No admin account for that link. Check ADMIN_EMAILS, make the account if needed, then restart the server.",
	"slowdown":    "Too many tries. Wait a minute and try again.",
	"stale":       "That page was out of date. Try again.",
	"nothingopen": "That report was already closed.",
	// Why a good link opened nothing (whyNoAdmin).
	"noaccount":     "No account uses the admin email yet.",
	"admindisabled": "The admin account is disabled.",
	"admindeleting": "The admin account is set to be deleted. Log in to the app to keep it.",
}

// AdminHandler serves /admin.
type AdminHandler struct {
	repo    admin.Repository
	users   user.Repository
	entries *EntryHandler
	secret  []byte
	admins  map[uuid.UUID]bool
	// first is the first admin found when the server started: a link that
	// names no address opens the page for it.
	first  uuid.UUID
	secure bool
	pages  *template.Template
	css    []byte
	// accountLimit counts log in tries per address and account. NewServer
	// gives it the app's log in's (AuthHandler.accountLimit), so tries on
	// either page count against one share.
	accountLimit *middleware.Limiter
	// clock is the time the limits and the links count by; nil is the real
	// clock.
	clock func() time.Time
	// usedLinks are the nonces of the links that opened the page, each kept
	// until its link would have ended: a link works once.
	linkMu    sync.Mutex
	usedLinks map[string]time.Time
}

// NewAdminHandler builds the admin page. admins are the admin accounts' ids,
// found once when the server starts (FindAdminAccounts); secure sets the
// Secure flag on the cookie (off only in test mode, which runs over plain
// http on localhost).
func NewAdminHandler(repo admin.Repository, users user.Repository, entries *EntryHandler,
	secret []byte, admins []uuid.UUID, secure bool) *AdminHandler {
	h := &AdminHandler{repo: repo, users: users, entries: entries, secret: secret, secure: secure,
		admins: map[uuid.UUID]bool{}, usedLinks: map[string]time.Time{}}
	for _, id := range admins {
		h.admins[id] = true
	}
	if len(admins) > 0 {
		h.first = admins[0]
	}
	h.accountLimit = middleware.NewLimiter(loginAccountTries, time.Minute, h.now)
	h.pages = template.Must(template.New("admin").Funcs(template.FuncMap{
		"when": func(t any) string {
			switch v := t.(type) {
			case time.Time:
				return v.Local().Format("Jan 2, 2006 3:04 PM")
			case *time.Time:
				if v == nil {
					return ""
				}
				return v.Local().Format("Jan 2, 2006 3:04 PM")
			}
			return ""
		},
		// day writes a post's day (YYYY-MM-DD). It is a calendar date, so no
		// time zone moves it, unlike when.
		"day": func(date string) string {
			d, err := time.Parse("2006-01-02", date)
			if err != nil {
				return date
			}
			return d.Format("Jan 2, 2006")
		},
		"reason": func(r string) string {
			if w, ok := admin.ReasonWords[r]; ok {
				return w
			}
			return r
		},
		// reasonLower is a report reason in words inside a sentence, as in
		// "Post reported for spam".
		"reasonLower": func(r string) string {
			if w, ok := admin.ReasonWords[r]; ok {
				return strings.ToLower(w)
			}
			return r
		},
		// reportedTimes is how often a person was reported, above their
		// History.
		"reportedTimes": func(n int) string {
			if n == 1 {
				return "Reported once."
			}
			return "Reported " + strconv.Itoa(n) + " times."
		},
		// said is what a report was about, for the History: the post's title
		// (as it is now, or as it was once the post is gone), the comment's
		// words, or nothing for an account.
		"said": func(r admin.Report) string {
			switch r.Kind {
			case admin.KindPost:
				if r.TitleNow != "" {
					return r.TitleNow
				}
				return r.Title
			case admin.KindComment:
				if r.Body == "" {
					return "(photo only)"
				}
				return r.Body
			}
			return ""
		},
		"kind": func(k string) string {
			switch k {
			case admin.KindPost:
				return "Post"
			case admin.KindComment:
				return "Comment"
			}
			return "Account"
		},
		// reasons writes report reasons in words, the most serious first, for
		// the history: commas, so no line there reads like a card's reasons.
		"reasons": func(rs []string) string {
			words := make([]string, 0, len(rs))
			for _, r := range rs {
				if w, ok := admin.ReasonWords[r]; ok {
					r = w
				}
				words = append(words, r)
			}
			return strings.Join(words, ", ")
		},
		// decision names one row of the decision log.
		"decision": func(e admin.Event) string {
			what := "Account"
			switch e.Item {
			case admin.KindPost:
				what = "Post"
			case admin.KindComment:
				what = "Comment"
			}
			switch e.Action {
			case admin.LogRemoved:
				return what + " removed"
			case admin.LogRestored:
				return what + " brought back"
			case admin.LogShownAgain:
				return what + " shown again"
			case admin.LogDisabled:
				return what + " disabled"
			case admin.LogEnabled:
				return what + " enabled"
			case admin.LogHidden:
				return what + " hidden by reports"
			case admin.LogKept:
				return what + " kept removed"
			}
			return what + " " + e.Action
		},
		// daysLeft is how long a removed post or comment has before it is
		// deleted for good.
		"daysLeft": func(days int) string {
			if days == 1 {
				return "1 day left"
			}
			return strconv.Itoa(days) + " days left"
		},
		// ref is the short reference of a log row.
		"ref": admin.Ref,
		// reasonKeys are the report reasons, the most serious first: the
		// Reason list beside Remove.
		"reasonKeys": func() []string { return removeReasons },
		"closed": func(resolution string) string {
			switch resolution {
			case admin.Dismissed:
				return "Dismissed"
			case admin.Removed:
				return "Removed"
			case admin.Disabled:
				return "Closed when the account was disabled"
			}
			return "Closed"
		},
		// card hands the "item" template one item with the page's CSRF token
		// and which page it is on.
		"card": func(it adminItem, csrf, from string) adminCard {
			return adminCard{Item: it, CSRF: csrf, From: from}
		},
		// initial is the letter shown for a person with no profile photo.
		"initial": func(username string) string {
			for _, r := range username {
				return strings.ToUpper(string(r))
			}
			return "?"
		},
		// photo signs the link of a post, comment or profile photo key.
		// The page signs each photo once, for both its link and its
		// picture, and shows nothing when no link comes back.
		"photo": func(key string) string {
			if entries == nil || entries.photos == nil {
				return ""
			}
			return entries.photos.Link(key)
		},
	}).ParseFS(adminFiles, "admin_pages/pages.html"))
	h.css, _ = adminFiles.ReadFile("admin_pages/style.css")
	return h
}

// FindAdminAccounts finds the account of each ADMIN_EMAILS address, once,
// when the server starts: only those accounts are admins until it starts
// again. missing are the addresses no account has (cmd/api warns about each).
// Sign-up answers them as taken meanwhile (AuthHandler.SetAdminEmails), so
// nobody can become an admin by signing up with one first.
func FindAdminAccounts(ctx context.Context, users user.Repository, emails []string) (ids []uuid.UUID, missing []string, err error) {
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		u, err := users.GetByEmailCI(ctx, email)
		if errors.Is(err, user.ErrNotFound) {
			missing = append(missing, email)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, u.ID)
	}
	return ids, missing, nil
}

// now is the limits' clock: clock, or the real one.
func (h *AdminHandler) now() time.Time {
	if h.clock != nil {
		return h.clock()
	}
	return time.Now()
}

// Register mounts /admin on the engine (outside /v1).
func (h *AdminHandler) Register(engine *gin.Engine) {
	g := engine.Group("/admin")
	g.Use(middleware.BodyLimit(adminBodyBytes), adminHeaders)
	g.GET("/style.css", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/css; charset=utf-8", h.css)
	})
	g.GET("/login", h.loginPage)
	g.POST("/login", h.perAddress(), h.sameOrigin, h.login)
	g.GET("/link", h.perAddress(), h.openLink)
	g.GET("/go", h.goPage)

	in := g.Group("")
	in.Use(h.requireAdmin)
	in.GET("", h.list)
	in.GET("/disabled", h.disabledPage)
	in.GET("/removed", h.removedPage)
	in.GET("/people/:id", h.person)
	in.POST("/logout", h.sameOrigin, h.checkCSRF, h.logout)
	in.POST("/reports/dismiss", h.sameOrigin, h.checkCSRF, h.dismiss)
	in.POST("/reports/remove", h.sameOrigin, h.checkCSRF, h.remove)
	in.POST("/removed/restore", h.sameOrigin, h.checkCSRF, h.restore)
	in.POST("/removed/keep", h.sameOrigin, h.checkCSRF, h.keep)
	in.POST("/people/:id/disable", h.sameOrigin, h.checkCSRF, h.disable)
	in.POST("/people/:id/enable", h.sameOrigin, h.checkCSRF, h.enable)
}

// perAddress allows one route 5 requests a minute per address (KeyByIP), each
// route counting on its own. Past that it answers with the log in page asking
// to wait, not the API's JSON refusal: this is a page in a browser. The log
// in page itself has no limit, so that answer always shows.
func (h *AdminHandler) perAddress() gin.HandlerFunc {
	limit := middleware.NewLimiter(5, time.Minute, h.now)
	return func(c *gin.Context) {
		if !limit.Allow(middleware.KeyByIP(c)) {
			h.toLogin(c, "slowdown")
			return
		}
		c.Next()
	}
}

// adminHeaders keeps the page from being framed, cached or loading anything
// from elsewhere (photos excepted: they come from the photo store). Nothing
// is cached and no referrer is sent, so a signed photo link stays on the
// page, and a reload signs fresh ones.
func adminHeaders(c *gin.Context) {
	hdr := c.Writer.Header()
	hdr.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: http:; style-src 'self'; "+
		"script-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	hdr.Set("X-Frame-Options", "DENY")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "no-referrer")
	hdr.Set("Cache-Control", "no-store")
	c.Next()
}

// --- the session ---

func (h *AdminHandler) sign(purpose, payload string) string {
	m := hmac.New(sha256.New, h.secret)
	m.Write([]byte("rndmroll-admin-" + purpose + ":" + payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// passwordMark ties a session to the account's password: a short signature
// of its hash. Every new password has a new hash (bcrypt salts each one), so
// changing, resetting or setting the password ends every admin session, as it
// ends the app's other sessions. Without the secret the mark says nothing
// about the hash.
func (h *AdminHandler) passwordMark(u *user.User) string {
	hash := ""
	if u.PasswordHash != nil {
		hash = *u.PasswordHash
	}
	return h.sign("password", u.ID.String()+":"+hash)[:22]
}

// newSession is the cookie value of a session for the account id whose
// password has mark: id.expiry.mark.signature.
func (h *AdminHandler) newSession(id uuid.UUID, mark string, now time.Time) string {
	payload := id.String() + "." + strconv.FormatInt(now.Add(adminSessionTTL).Unix(), 10) + "." + mark
	return payload + "." + h.sign("session", payload)
}

// sessionUser reads a signed, unexpired session cookie value: the account and
// the password mark it was signed with. A cookie from before the mark (three
// parts) is refused, so its admin logs in once more.
func (h *AdminHandler) sessionUser(value string, now time.Time) (id uuid.UUID, mark string, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return uuid.UUID{}, "", false
	}
	payload := parts[0] + "." + parts[1] + "." + parts[2]
	if !hmac.Equal([]byte(parts[3]), []byte(h.sign("session", payload))) {
		return uuid.UUID{}, "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() >= exp {
		return uuid.UUID{}, "", false
	}
	id, err = uuid.Parse(parts[0])
	if err != nil {
		return uuid.UUID{}, "", false
	}
	return id, parts[2], true
}

func (h *AdminHandler) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: adminCookie, Value: value, Path: "/admin", MaxAge: maxAge,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode,
	})
}

// isAdmin: the account exists, is one of the admin accounts found when the
// server started, is not disabled, and its owner has not asked for it to be
// deleted (an admin who did is no admin from that moment, as for any other
// session of the account).
func (h *AdminHandler) isAdmin(u *user.User) bool {
	return u != nil && !u.Disabled && !u.DeletionRequested && h.admins[u.ID]
}

const adminUserKey = "admin_user"

// requireAdmin lets a request through only with a session of an admin whose
// password has not changed since it began. Only a definite no ends the
// session: a lookup that fails for any reason but "no such account" is a
// failure page, and the cookie stays.
func (h *AdminHandler) requireAdmin(c *gin.Context) {
	cookie, err := c.Cookie(adminCookie)
	id, mark, ok := h.sessionUser(cookie, time.Now())
	if err != nil || !ok {
		h.toLogin(c, "expired")
		return
	}
	u, err := h.users.GetByID(c.Request.Context(), id)
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		h.fail(c, err)
		c.Abort()
		return
	}
	if err != nil || !h.isAdmin(u) || !hmac.Equal([]byte(mark), []byte(h.passwordMark(u))) {
		h.setCookie(c, "", -1)
		h.toLogin(c, "expired")
		return
	}
	c.Set(adminUserKey, u)
	c.Next()
}

func (h *AdminHandler) toLogin(c *gin.Context, notice string) {
	c.Redirect(http.StatusSeeOther, "/admin/login?n="+notice)
	c.Abort()
}

func adminUser(c *gin.Context) *user.User {
	v, _ := c.Get(adminUserKey)
	u, _ := v.(*user.User)
	return u
}

func (h *AdminHandler) csrf(c *gin.Context) string {
	cookie, _ := c.Cookie(adminCookie)
	return h.sign("csrf", cookie)
}

// checkCSRF refuses an action whose form token is not this session's. Each
// log in makes a new cookie and so a new token: a form from a page opened
// before the latest log in, or from no admin page at all, does nothing and
// goes back to the list, which says the page was out of date.
func (h *AdminHandler) checkCSRF(c *gin.Context) {
	if !hmac.Equal([]byte(c.PostForm("csrf")), []byte(h.csrf(c))) {
		c.Redirect(http.StatusSeeOther, "/admin?n=stale")
		c.Abort()
		return
	}
	c.Next()
}

// sameOrigin refuses a POST sent from another site.
func (h *AdminHandler) sameOrigin(c *gin.Context) {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != c.Request.Host {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}
	c.Next()
}

// --- pages ---

type adminPage struct {
	Title string
	// Refresh moves the page on to /admin at once (the page a link opens),
	// or to Next when it is set.
	Refresh bool
	Next    string
	CSRF    string
	Notice  string
	Person  admin.Person
	Self    bool
	// Tab is the section the page is in ("reports", "disabled" or
	// "removed"; none on a person's page), and OpenCount, DisabledCount and
	// RemovedCount are the tabs' numbers.
	Tab           string
	OpenCount     int
	DisabledCount int
	RemovedCount  int
	// Items are every open report, put together (the Reports tab); Open are
	// one person's (their page).
	Items []adminItem
	Open  []adminItem
	// Capped says Items holds only the newest admin.OpenReportsLimit open
	// reports.
	Capped bool
	// People are the disabled accounts (the Disabled tab).
	People []admin.Person
	// Removed is what the Removed tab lists.
	Removed []admin.RemovedItem
	// History is every report about the person, every decision about them
	// and every second look they asked for, newest first, and Reported how
	// many reports there are (their page).
	History  []historyEntry
	Reported int
	// Back and BackLabel are where their page's back arrow goes and that
	// tab's name (backArrow). Posts are their posts when admin.ShowsPosts
	// (PostsShown), the newest admin.PostsLimit, and PostsCapped says they
	// have more.
	Back        string
	BackLabel   string
	PostsShown  bool
	Posts       []admin.Post
	PostsCapped bool
}

// backArrow is where the back arrow on a person's page goes, and that tab's
// name: the tab whose Open profile link opened it (?from=removed or
// disabled), else Reports. Like backTo, only these fixed paths come out,
// never the parameter.
func backArrow(from string) (path, label string) {
	switch from {
	case "removed":
		return "/admin/removed", "Removed"
	case "disabled":
		return "/admin/disabled", "Disabled"
	}
	return "/admin", "Reports"
}

func (h *AdminHandler) render(c *gin.Context, name string, p adminPage) {
	p.Notice = adminNotices[c.Query("n")]
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	if err := h.pages.ExecuteTemplate(c.Writer, name, p); err != nil {
		log.Printf("httpapi: admin page %s: %v", name, err)
	}
}

func (h *AdminHandler) loginPage(c *gin.Context) {
	h.render(c, "login", adminPage{Title: "Log in"})
}

// login signs an admin in. Like the app's Log in: a lookup that fails for
// any reason but "no such account" is a failure, not a bad login; the
// password is compared for every account, and with the dummy hash for none;
// and past the account's limit (shared with the app's Log in) the answer is
// the bad-login page, after the same comparison.
func (h *AdminHandler) login(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")
	u, err := h.users.GetByEmailCI(c.Request.Context(), email)
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		h.fail(c, err)
		return
	}
	allowed := true
	if err == nil {
		allowed = h.accountLimit.Allow(middleware.KeyByIP(c) + "|" + u.ID.String())
	} else {
		u = nil
	}
	if !passwordMatches(auth.ComparePassword, u, password, allowed) || !h.isAdmin(u) {
		h.toLogin(c, "badlogin")
		return
	}
	h.setCookie(c, h.newSession(u.ID, h.passwordMark(u), time.Now()), int(adminSessionTTL.Seconds()))
	c.Redirect(http.StatusSeeOther, "/admin")
}

// openLink answers a link from `make admin` (cmd/admin-link). A link signed
// with the page's secret, unexpired and unused, opens the page for the admin
// it names, or for the first admin when it names none. It is spent before the
// account is looked up, so a link that fails once never works later. A bad,
// ended or used link is one notice. A good link for no admin gets a notice
// that says why (whyNoAdmin): it tells nothing to anyone without the secret,
// as only a link signed with it gets that far. Log in with a password never
// says why (badlogin).
//
// The answer is a page that moves on to /admin, not a redirect: a browser
// that counts the link's own navigation as coming from elsewhere (opened
// from the terminal) would hold the SameSite=Strict cookie back on a
// redirect, while the page's own move is same-site.
func (h *AdminHandler) openLink(c *gin.Context) {
	now := h.now()
	l, err := adminlink.Read(h.secret, c.Query("token"), now)
	if err != nil || !h.spendLink(l, now) {
		h.toLogin(c, "badlink")
		return
	}
	ctx := c.Request.Context()
	var u *user.User
	if l.Email == "" {
		u, err = h.users.GetByID(ctx, h.first)
	} else {
		u, err = h.users.GetByEmailCI(ctx, l.Email)
	}
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		h.fail(c, err)
		return
	}
	if err != nil {
		u = nil
	}
	if !h.isAdmin(u) {
		h.toLogin(c, h.whyNoAdmin(u))
		return
	}
	log.Printf("httpapi: admin page opened with a link for account %s", u.ID)
	h.setCookie(c, h.newSession(u.ID, h.passwordMark(u), time.Now()), int(adminSessionTTL.Seconds()))
	h.render(c, "opened", adminPage{Title: "Opening", Refresh: true})
}

// whyNoAdmin is the notice for a good link that opened nothing, u being the
// account it names (nil for none: no admin was found when the server
// started, or no account has the address). An account that is no admin, or
// past the grace period of a deletion its owner asked for, gets the plain
// noadmin. An admin account that is disabled, or set to be deleted (a log in
// to the app keeps it), gets its own. Disabled comes first, as in the app's
// log in: logging in keeps no disabled account.
func (h *AdminHandler) whyNoAdmin(u *user.User) string {
	switch {
	case u == nil:
		return "noaccount"
	case !h.admins[u.ID] || u.DeletionDue:
		return "noadmin"
	case u.Disabled:
		return "admindisabled"
	case u.DeletionRequested:
		return "admindeleting"
	}
	return "noadmin"
}

// spendLink marks a link used, and says whether it was unused. The nonces of
// links that have ended are dropped first.
func (h *AdminHandler) spendLink(l adminlink.Link, now time.Time) bool {
	h.linkMu.Lock()
	defer h.linkMu.Unlock()
	for nonce, end := range h.usedLinks {
		if !now.Before(end) {
			delete(h.usedLinks, nonce)
		}
	}
	if _, used := h.usedLinks[l.Nonce]; used {
		return false
	}
	h.usedLinks[l.Nonce] = l.Expires
	return true
}

func (h *AdminHandler) logout(c *gin.Context) {
	h.setCookie(c, "", -1)
	c.Redirect(http.StatusSeeOther, "/admin/login?n=signedout")
}

// goPage is where the report email links (internal/adminalert). The session
// cookie is SameSite=Strict, so a click in a mail app or webmail arrives
// without it; this page asks for nothing and moves on to the list itself,
// and that step, from this site, carries the cookie. ?to=removed, the
// second-look link (developer, 2026-11-19), moves on to the Removed tab
// instead; the query only picks one of the two fixed pages.
func (h *AdminHandler) goPage(c *gin.Context) {
	if c.Query("to") == "removed" {
		h.render(c, "go", adminPage{Title: "Removed", Refresh: true, Next: "/admin/removed"})
		return
	}
	h.render(c, "go", adminPage{Title: "Reports", Refresh: true})
}

func (h *AdminHandler) list(c *gin.Context) {
	ctx := c.Request.Context()
	items, capped, err := h.openItems(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	people, err := h.repo.DisabledPeople(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	removed, err := h.repo.RemovedCount(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.render(c, "list", adminPage{Title: "Reports", CSRF: h.csrf(c), Tab: "reports", Items: items, Capped: capped,
		OpenCount: len(items), DisabledCount: len(people), RemovedCount: removed})
}

func (h *AdminHandler) disabledPage(c *gin.Context) {
	ctx := c.Request.Context()
	people, err := h.repo.DisabledPeople(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	items, _, err := h.openItems(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	removed, err := h.repo.RemovedCount(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.render(c, "disabled", adminPage{Title: "Disabled", CSRF: h.csrf(c), Tab: "disabled", People: people,
		OpenCount: len(items), DisabledCount: len(people), RemovedCount: removed})
}

// removedPage is the Removed tab: every post and comment removed in the
// last admin.RemovalGrace, each with Bring it back. Those whose owner asked
// for a second look come first, with what they wrote, their photos and Keep
// it removed (developer, 2026-11-19); then the most recently removed first.
// Unlike the Reports tab it does not refresh itself.
func (h *AdminHandler) removedPage(c *gin.Context) {
	ctx := c.Request.Context()
	removed, err := h.repo.Removed(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	items, _, err := h.openItems(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	people, err := h.repo.DisabledPeople(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.render(c, "removed", adminPage{Title: "Removed", CSRF: h.csrf(c), Tab: "removed", Removed: removed,
		OpenCount: len(items), DisabledCount: len(people), RemovedCount: len(removed)})
}

// openItems is every open report put together by what it is about, in the
// order the admin works through them (sortOpen). capped says more than
// admin.OpenReportsLimit are open, so the oldest are not among them.
func (h *AdminHandler) openItems(ctx context.Context) (items []adminItem, capped bool, err error) {
	open, err := h.repo.OpenReports(ctx)
	if err != nil {
		return nil, false, err
	}
	// The read holds one more than the page shows, so exactly a full page is
	// told apart from a cut one.
	if capped = len(open) > admin.OpenReportsLimit; capped {
		open = open[:admin.OpenReportsLimit]
		log.Printf("httpapi: admin: more than %d open reports; the page shows the newest", admin.OpenReportsLimit)
	}
	var owners []admin.Person
	byOwner := map[uuid.UUID][]admin.Report{}
	for _, r := range open {
		if _, seen := byOwner[r.Owner.ID]; !seen {
			owners = append(owners, r.Owner)
		}
		byOwner[r.Owner.ID] = append(byOwner[r.Owner.ID], r.Report)
	}
	for _, o := range owners {
		group, _ := groupReports(o, byOwner[o.ID])
		items = append(items, group...)
	}
	sortOpen(items)
	return items, capped, nil
}

func (h *AdminHandler) person(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	ctx := c.Request.Context()
	p, err := h.repo.Person(ctx, id)
	if errors.Is(err, admin.ErrNotFound) {
		toList(c)
		return
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	reports, err := h.repo.Reports(ctx, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	items, _, err := h.openItems(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	people, err := h.repo.DisabledPeople(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	removed, err := h.repo.RemovedCount(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	events, err := h.repo.Events(ctx, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	looks, err := h.repo.SecondLooks(ctx, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	open, _ := groupReports(p, reports)
	page := adminPage{Title: "@" + p.Username, CSRF: h.csrf(c), Person: p, Open: open,
		History: history(reports, events, looks), Reported: len(reports), Self: id == adminUser(c).ID, OpenCount: len(items), DisabledCount: len(people), RemovedCount: removed}
	page.Back, page.BackLabel = backArrow(c.Query("from"))
	if admin.ShowsPosts(p, reports) {
		posts, err := h.repo.Posts(ctx, id)
		if err != nil {
			h.fail(c, err)
			return
		}
		if len(posts) > admin.PostsLimit {
			posts, page.PostsCapped = posts[:admin.PostsLimit], true
		}
		page.PostsShown, page.Posts = true, posts
	}
	h.render(c, "person", page)
}

func (h *AdminHandler) fail(c *gin.Context, err error) {
	log.Printf("httpapi: admin: %v", err)
	c.String(http.StatusInternalServerError, "Something went wrong. Please try again.")
}

// back returns to the person's page with one of the fixed notices.
func back(c *gin.Context, person uuid.UUID, notice string) {
	c.Redirect(http.StatusSeeOther, "/admin/people/"+person.String()+"?n="+notice)
}

// backTo returns to the page the action was sent from, as its form's "from"
// says: the Reports tab ("list"), the Disabled tab ("disabled"), the Removed
// tab ("removed"), or else the person's page. Only these fixed places, never
// a path from the request.
func backTo(c *gin.Context, person uuid.UUID, notice string) {
	switch c.PostForm("from") {
	case "list":
		c.Redirect(http.StatusSeeOther, "/admin?n="+notice)
	case "disabled":
		c.Redirect(http.StatusSeeOther, "/admin/disabled?n="+notice)
	case "removed":
		c.Redirect(http.StatusSeeOther, "/admin/removed?n="+notice)
	default:
		back(c, person, notice)
	}
}

// toList answers a person's page the admin may not open (nobody reported
// them, they are not disabled and the decision log names nothing of theirs,
// or there is no such account) with the list, saying nothing about them. It
// keeps a fixed notice, so an action that went back to a page that no
// longer opens still says how it went.
func toList(c *gin.Context) {
	to := "/admin"
	if n := c.Query("n"); adminNotices[n] != "" {
		to += "?n=" + n
	}
	c.Redirect(http.StatusSeeOther, to)
}

// mayOpen reports whether the admin may act on the person: the same rule as
// their page (AdminRepo.Person). When not, it has answered already.
func (h *AdminHandler) mayOpen(c *gin.Context, person uuid.UUID) bool {
	_, err := h.repo.Person(c.Request.Context(), person)
	if errors.Is(err, admin.ErrNotFound) {
		toList(c)
		return false
	}
	if err != nil {
		back(c, person, "failed")
		return false
	}
	return true
}

// --- actions ---

// ownerNow is the owner's own now, which a decision about their post,
// comment or account is logged at (the bell and the history read it): the
// spin slice's clock for them, or the real one when that cannot be read (no
// stored time zone, say).
func (h *AdminHandler) ownerNow(ctx context.Context, owner uuid.UUID) time.Time {
	if h.entries != nil && h.entries.svc != nil {
		if now, _, err := h.entries.svc.ViewerClock(ctx, owner.String()); err == nil {
			return now
		}
	}
	return time.Now()
}

func formIDs(c *gin.Context) (kind string, id, person uuid.UUID, ok bool) {
	kind = c.PostForm("kind")
	var err1, err2 error
	id, err1 = uuid.Parse(c.PostForm("id"))
	person, err2 = uuid.Parse(c.PostForm("person"))
	ok = err1 == nil && err2 == nil &&
		(kind == admin.KindPerson || kind == admin.KindPost || kind == admin.KindComment)
	return
}

// adminDismissMost bounds the reports one Dismiss closes: a card's reports
// (a card holds at most admin.OpenReportsLimit).
const adminDismissMost = admin.OpenReportsLimit

// adminBodyBytes is the largest form the admin page takes: room for a
// Dismiss that sends adminDismissMost report ids (about 40 bytes each).
const adminBodyBytes = 64 << 10

// dismiss closes the reports on one card: every id the form sends, each
// only while it is open and about that person (DismissItem checks both, in
// one transaction). A post or comment that was hidden while under review
// and is not once they are closed shows again, and its owner is told: the
// notice says so.
func (h *AdminHandler) dismiss(c *gin.Context) {
	kind := c.PostForm("kind")
	person, err := uuid.Parse(c.PostForm("person"))
	raw := c.PostFormArray("id")
	if err != nil || len(raw) == 0 || len(raw) > adminDismissMost ||
		(kind != admin.KindPerson && kind != admin.KindPost && kind != admin.KindComment) {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}
		ids = append(ids, id)
	}
	ctx := c.Request.Context()
	loggedAt := time.Now()
	if kind != admin.KindPerson {
		loggedAt = h.ownerNow(ctx, person)
	}
	closed, shownAgain, err := h.repo.DismissItem(ctx, kind, ids, person, time.Now().UTC(), loggedAt)
	switch {
	case err != nil:
		log.Printf("httpapi: admin dismiss %s: %v", kind, err)
		backTo(c, person, "failed")
	case closed == 0:
		backTo(c, person, "nothingopen")
	case shownAgain:
		backTo(c, person, "shownagain")
	default:
		backTo(c, person, "dismissed")
	}
}

// remove takes down the reported post or comment for everyone, its owner
// too, in one transaction that also closes every report open on it and logs
// the removal (RemoveItem). Nothing is deleted: Bring it back on the Removed
// tab undoes it for admin.RemovalGrace, and the purge deletes it and its
// photos after. The card names a report of it, whose target and owner come
// from the database, so a card for someone else changes nothing. A second
// Remove inside the grace (a double click) says what the first said, and
// changes nothing, its reason included.
//
// The form's reason is the one the admin picked from the report reasons
// (developer, 2026-11-18: "answer them why we are removing it"), and the
// owner's bell names it. A form with none takes the most serious reason of
// the reports it closes; one with any other word is malformed and removes
// nothing.
func (h *AdminHandler) remove(c *gin.Context) {
	kind, id, person, ok := formIDs(c)
	reason := c.PostForm("reason")
	if _, known := admin.ReasonWords[reason]; !ok || kind == admin.KindPerson || (reason != "" && !known) {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	ctx := c.Request.Context()
	target, owner, err := h.repo.ReportTarget(ctx, kind, id)
	if err != nil || owner != person {
		backTo(c, person, "failed")
		return
	}
	if target == nil {
		backTo(c, person, "gone")
		return
	}
	err = h.repo.RemoveItem(ctx, kind, *target, owner, reason, time.Now().UTC(), h.ownerNow(ctx, owner))
	switch {
	case err == nil, errors.Is(err, admin.ErrAlreadyDone):
		backTo(c, person, "removed")
	case errors.Is(err, admin.ErrNotFound):
		// Deleted by its owner meanwhile, or removed past the grace: this
		// changes nothing.
		backTo(c, person, "gone")
	default:
		log.Printf("httpapi: admin remove %s: %v", kind, err)
		backTo(c, person, "failed")
	}
}

// restore is Bring it back on the Removed tab: the post or comment shows
// again, and its owner is told, while it is inside admin.RemovalGrace. Its
// reports stay closed, so its reporters still never see it, and a second
// look its owner asked for is answered with it, also one Keep it removed
// answered before. A comment whose post or the comment it answers is still
// removed stays removed, its request open, and the notice says which to
// bring back first. A second Bring it back of one it brought back (a double
// click) says what the first said, and changes nothing. Past the grace,
// never removed or deleted, it is gone.
func (h *AdminHandler) restore(c *gin.Context) {
	kind, id, person, ok := formIDs(c)
	if !ok || kind == admin.KindPerson {
		c.Redirect(http.StatusSeeOther, "/admin/removed")
		return
	}
	ctx := c.Request.Context()
	err := h.repo.RestoreItem(ctx, kind, id, person, h.ownerNow(ctx, person))
	switch {
	case err == nil, errors.Is(err, admin.ErrAlreadyDone):
		backTo(c, person, "restored")
	case errors.Is(err, admin.ErrPostRemoved):
		backTo(c, person, "postfirst")
	case errors.Is(err, admin.ErrParentRemoved):
		backTo(c, person, "parentfirst")
	case errors.Is(err, admin.ErrNotFound):
		backTo(c, person, "gone")
	default:
		log.Printf("httpapi: admin restore %s: %v", kind, err)
		backTo(c, person, "failed")
	}
}

// keep is Keep it removed on the Removed tab, the answer to a second look
// its owner asked for (developer, 2026-11-19): it stays removed, the request
// is closed and logged on the owner's own clock, and their bell says so. The
// tab draws it only on a line with an open request. A second Keep (a double
// click) says what the first said, and changes nothing. One whose request a
// Bring it back answered meanwhile says it was answered; past the grace or
// deleted, it is gone.
func (h *AdminHandler) keep(c *gin.Context) {
	kind, id, person, ok := formIDs(c)
	if !ok || kind == admin.KindPerson {
		c.Redirect(http.StatusSeeOther, "/admin/removed")
		return
	}
	ctx := c.Request.Context()
	err := h.repo.KeepRemoved(ctx, kind, id, person, h.ownerNow(ctx, person))
	switch {
	case err == nil, errors.Is(err, admin.ErrAlreadyDone):
		backTo(c, person, "kept")
	case errors.Is(err, admin.ErrNoRequest):
		backTo(c, person, "answered")
	case errors.Is(err, admin.ErrNotFound):
		backTo(c, person, "gone")
	default:
		log.Printf("httpapi: admin keep %s: %v", kind, err)
		backTo(c, person, "failed")
	}
}

func (h *AdminHandler) disable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	if id == adminUser(c).ID {
		back(c, id, "self")
		return
	}
	if !h.mayOpen(c, id) {
		return
	}
	ctx := c.Request.Context()
	now := time.Now().UTC()
	// Logged out everywhere: SetDisabled revokes every refresh token in the
	// transaction that disables the account, so no refresh token works again,
	// and every request with an old access token is refused (RequireUser).
	// It logs the decision in that transaction too.
	if err := h.repo.SetDisabled(ctx, id, &now, h.ownerNow(ctx, id)); err != nil {
		back(c, id, "failed")
		return
	}
	if err := h.repo.ResolvePerson(ctx, id, admin.Disabled, now); err != nil {
		log.Printf("httpapi: admin disable resolve: %v", err)
	}
	back(c, id, "disabled")
}

func (h *AdminHandler) enable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	if !h.mayOpen(c, id) {
		return
	}
	ctx := c.Request.Context()
	if err := h.repo.SetDisabled(ctx, id, nil, h.ownerNow(ctx, id)); err != nil {
		backTo(c, id, "failed")
		return
	}
	backTo(c, id, "enabled")
}
