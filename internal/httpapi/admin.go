// Package httpapi: admin.go serves the admin web page at /admin (developer,
// 2026-10-18, like Instagram's review tool): open reports grouped by the
// reported person, one person's reports, and the actions (dismiss a report,
// remove the reported post or comment, disable or enable the account).
//
// It is a plain server-rendered page, outside /v1 and its bearer tokens:
//   - Only the accounts in ADMIN_EMAILS may sign in, with their app password.
//     Without ADMIN_EMAILS nothing here is mounted.
//   - The session is an HttpOnly, SameSite=Strict cookie signed for the admin
//     page only (an app token never works here, and this cookie never works
//     in the app). The allowlist and the disabled flag are checked again on
//     every request.
//   - Every action is a POST carrying a CSRF token tied to the session, and a
//     cross-site Origin is refused.
//   - Pages are html/template (every user-written text is escaped), with a
//     strict Content-Security-Policy and no framing.
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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
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

// The reasons as the admin reads them (the same list the app shows).
var adminReasonWords = map[string]string{
	"spam":          "Spam",
	"nudity":        "Nudity or sexual content",
	"harassment":    "Bullying or harassment",
	"hate":          "Hate speech",
	"violence":      "Violence or threats",
	"impersonation": "Pretending to be someone else",
	"other":         "Something else",
}

// The notices a redirect may carry. Only these fixed words are ever shown:
// nothing from the query is echoed.
var adminNotices = map[string]string{
	"dismissed":   "Report dismissed.",
	"removed":     "Removed. Its reports are closed.",
	"gone":        "That post or comment is already gone.",
	"disabled":    "Account disabled. They are logged out and hidden from everyone.",
	"enabled":     "Account enabled again.",
	"self":        "You can't disable your own account.",
	"failed":      "Something went wrong. Please try again.",
	"signedout":   "You're logged out.",
	"badlogin":    "Wrong email or password, or this account is not an admin.",
	"expired":     "Please log in again.",
	"nothingopen": "That report was already closed.",
}

// AdminHandler serves /admin.
type AdminHandler struct {
	repo    admin.Repository
	users   user.Repository
	entries *EntryHandler
	refresh *auth.RefreshService
	secret  []byte
	emails  map[string]bool
	secure  bool
	pages   *template.Template
	css     []byte
}

// NewAdminHandler builds the admin page. emails are the admin accounts
// (lower case); secure sets the Secure flag on the cookie (off only in test
// mode, which runs over plain http on localhost).
func NewAdminHandler(repo admin.Repository, users user.Repository, entries *EntryHandler, refresh *auth.RefreshService,
	secret []byte, emails []string, secure bool) *AdminHandler {
	h := &AdminHandler{repo: repo, users: users, entries: entries, refresh: refresh, secret: secret, secure: secure,
		emails: map[string]bool{}}
	for _, e := range emails {
		h.emails[strings.ToLower(strings.TrimSpace(e))] = true
	}
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
		"reason": func(r string) string {
			if w, ok := adminReasonWords[r]; ok {
				return w
			}
			return r
		},
		"kind": func(k string) string {
			switch k {
			case admin.KindPost:
				return "Post"
			case admin.KindComment:
				return "Comment"
			}
			return "The account"
		},
		"photo": func(key string) string {
			if entries == nil || entries.photos == nil {
				return ""
			}
			return entries.photos.PublicURL(key)
		},
	}).ParseFS(adminFiles, "admin_pages/pages.html"))
	h.css, _ = adminFiles.ReadFile("admin_pages/style.css")
	return h
}

// Register mounts /admin on the engine (outside /v1).
func (h *AdminHandler) Register(engine *gin.Engine) {
	g := engine.Group("/admin")
	g.Use(middleware.BodyLimit(middleware.MaxBodyBytes), adminHeaders)
	g.GET("/style.css", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/css; charset=utf-8", h.css)
	})
	g.GET("/login", h.loginPage)
	g.POST("/login", middleware.RateLimit(middleware.LimitConfig{
		Requests: 5, Window: time.Minute, KeyFunc: middleware.KeyByIP,
	}), h.sameOrigin, h.login)

	in := g.Group("")
	in.Use(h.requireAdmin)
	in.GET("", h.list)
	in.GET("/people/:id", h.person)
	in.POST("/logout", h.sameOrigin, h.checkCSRF, h.logout)
	in.POST("/reports/dismiss", h.sameOrigin, h.checkCSRF, h.dismiss)
	in.POST("/reports/remove", h.sameOrigin, h.checkCSRF, h.remove)
	in.POST("/people/:id/disable", h.sameOrigin, h.checkCSRF, h.disable)
	in.POST("/people/:id/enable", h.sameOrigin, h.checkCSRF, h.enable)
}

// adminHeaders keeps the page from being framed, cached or loading anything
// from elsewhere (photos excepted: they come from the photo store).
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

func (h *AdminHandler) newSession(id uuid.UUID, now time.Time) string {
	payload := id.String() + "." + strconv.FormatInt(now.Add(adminSessionTTL).Unix(), 10)
	return payload + "." + h.sign("session", payload)
}

// sessionUser reads a signed, unexpired session cookie value.
func (h *AdminHandler) sessionUser(value string, now time.Time) (uuid.UUID, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return uuid.UUID{}, false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(h.sign("session", payload))) {
		return uuid.UUID{}, false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() >= exp {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(parts[0])
	return id, err == nil
}

func (h *AdminHandler) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: adminCookie, Value: value, Path: "/admin", MaxAge: maxAge,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode,
	})
}

// isAdmin: the account exists, is on the list and is not disabled.
func (h *AdminHandler) isAdmin(u *user.User) bool {
	return u != nil && !u.Disabled && h.emails[strings.ToLower(u.Email)]
}

const adminUserKey = "admin_user"

func (h *AdminHandler) requireAdmin(c *gin.Context) {
	cookie, err := c.Cookie(adminCookie)
	id, ok := h.sessionUser(cookie, time.Now())
	if err != nil || !ok {
		h.toLogin(c, "expired")
		return
	}
	u, err := h.users.GetByID(c.Request.Context(), id)
	if err != nil || !h.isAdmin(u) {
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

func (h *AdminHandler) checkCSRF(c *gin.Context) {
	if !hmac.Equal([]byte(c.PostForm("csrf")), []byte(h.csrf(c))) {
		c.AbortWithStatus(http.StatusForbidden)
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
	Title     string
	CSRF      string
	Notice    string
	Summaries []admin.Summary
	Disabled  []admin.Summary
	Person    admin.Person
	Reports   []admin.Report
	Self      bool
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

func (h *AdminHandler) login(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")
	u, err := h.users.GetByEmailCI(c.Request.Context(), email)
	if err != nil || u.PasswordHash == nil {
		_ = auth.ComparePassword(dummyPasswordHash, password)
		h.toLogin(c, "badlogin")
		return
	}
	if auth.ComparePassword(*u.PasswordHash, password) != nil || !h.isAdmin(u) {
		h.toLogin(c, "badlogin")
		return
	}
	h.setCookie(c, h.newSession(u.ID, time.Now()), int(adminSessionTTL.Seconds()))
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (h *AdminHandler) logout(c *gin.Context) {
	h.setCookie(c, "", -1)
	c.Redirect(http.StatusSeeOther, "/admin/login?n=signedout")
}

func (h *AdminHandler) list(c *gin.Context) {
	ctx := c.Request.Context()
	sums, err := h.repo.Summaries(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	off, err := h.repo.DisabledPeople(ctx)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.render(c, "list", adminPage{Title: "Reports", CSRF: h.csrf(c), Summaries: sums, Disabled: off})
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
		c.Redirect(http.StatusSeeOther, "/admin")
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
	h.render(c, "person", adminPage{Title: "@" + p.Username, CSRF: h.csrf(c), Person: p, Reports: reports,
		Self: id == adminUser(c).ID})
}

func (h *AdminHandler) fail(c *gin.Context, err error) {
	log.Printf("httpapi: admin: %v", err)
	c.String(http.StatusInternalServerError, "Something went wrong. Please try again.")
}

// back returns to the person's page with one of the fixed notices.
func back(c *gin.Context, person uuid.UUID, notice string) {
	c.Redirect(http.StatusSeeOther, "/admin/people/"+person.String()+"?n="+notice)
}

// --- actions ---

func formIDs(c *gin.Context) (kind string, id, person uuid.UUID, ok bool) {
	kind = c.PostForm("kind")
	var err1, err2 error
	id, err1 = uuid.Parse(c.PostForm("id"))
	person, err2 = uuid.Parse(c.PostForm("person"))
	ok = err1 == nil && err2 == nil &&
		(kind == admin.KindPerson || kind == admin.KindPost || kind == admin.KindComment)
	return
}

func (h *AdminHandler) dismiss(c *gin.Context) {
	kind, id, person, ok := formIDs(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	closed, err := h.repo.Resolve(c.Request.Context(), kind, id, person, admin.Dismissed, time.Now().UTC())
	if err != nil {
		back(c, person, "failed")
		return
	}
	if !closed {
		back(c, person, "nothingopen")
		return
	}
	back(c, person, "dismissed")
}

// remove takes down the reported post (through the app's own delete, so its
// photos and the posted day are handled as when its owner deletes it) or the
// reported comment, and closes every open report on it.
func (h *AdminHandler) remove(c *gin.Context) {
	kind, id, person, ok := formIDs(c)
	if !ok || kind == admin.KindPerson {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	ctx := c.Request.Context()
	target, owner, err := h.repo.ReportTarget(ctx, kind, id)
	if err != nil || owner != person {
		back(c, person, "failed")
		return
	}
	if target == nil {
		back(c, person, "gone")
		return
	}
	now := time.Now().UTC()
	// Closed first: once the post or comment is gone its reports no longer
	// point at it.
	if err := h.repo.ResolveTarget(ctx, kind, *target, admin.Removed, now); err != nil {
		back(c, person, "failed")
		return
	}
	if kind == admin.KindComment {
		err = h.repo.DeleteComment(ctx, *target)
	} else {
		err = h.removePost(ctx, owner, *target)
	}
	if err != nil {
		log.Printf("httpapi: admin remove %s: %v", kind, err)
		back(c, person, "failed")
		return
	}
	back(c, person, "removed")
}

func (h *AdminHandler) removePost(ctx context.Context, owner, entryID uuid.UUID) error {
	out, err := h.entries.svc.DeleteEntry(ctx, owner.String(), entryID.String())
	if err != nil {
		return err
	}
	h.entries.removeFiles(ctx, out.Dropped)
	return nil
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
	ctx := c.Request.Context()
	now := time.Now().UTC()
	if err := h.repo.SetDisabled(ctx, id, &now); err != nil {
		back(c, id, "failed")
		return
	}
	// Logged out everywhere: no refresh token works again, and every request
	// with an old access token is refused (RequireUser).
	if err := h.refresh.RevokeAll(ctx, id); err != nil {
		log.Printf("httpapi: admin disable revoke: %v", err)
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
	if err := h.repo.SetDisabled(c.Request.Context(), id, nil); err != nil {
		back(c, id, "failed")
		return
	}
	back(c, id, "enabled")
}
