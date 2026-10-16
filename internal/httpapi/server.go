// Package httpapi (this file): assembles every Phase 1 handler into one
// route tree. This is the only file that constructs a gin.Engine; every
// wave 4 handler plan (01-08 through 01-11) owns only its own Register
// method, exactly so this assembly could be deferred to a single plan
// without creating file conflicts across the parallel wave.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// Deps bundles every dependency NewServer needs to assemble the full route
// tree: the handlers, the user repository (for RequireUser's own lookup at
// the signed-in group boundary), the JWT signing secret (for RequireAuth),
// and a logger shared by the two cross-cutting middlewares.
type Deps struct {
	Auth     *AuthHandler
	Profile  *ProfileHandler
	Username *UsernameHandler

	// Account is the signed-in person's account routes (change password). It
	// is registered only when it is not nil, so a builder that does not set it
	// serves everything else unchanged.
	Account *AccountHandler

	// Spin and Wheel are the daily spin routes. TestTools is the test-only
	// clock and pretend-day routes: it is non-nil only when the server was
	// started in test mode with the tools switched on, and its routes are
	// registered only then. Each is registered only when it is not nil.
	Spin      *SpinHandler
	Wheel     *WheelHandler
	TestTools *SpinTestTools

	// Entry is the daily entry routes: the photo upload ticket, post, read,
	// edit and delete of an entry, and the skip. It is registered only when it
	// is not nil. When both Entry and Spin are set, NewServer gives the spin
	// handler the entry photo store's link builder, so every Today carries the
	// cover photo's link.
	Entry *EntryHandler

	// Diary is GET /diary and GET /me/summary. Registered only when not nil.
	Diary *DiaryHandler

	// Follow is the follows, blocks and notification settings routes; Board
	// is the Board, posts, reactions and comments; Notifications is the
	// bell. Each is registered only when not nil. Every follow is a request
	// (no public accounts since 2026-10-16), so nothing runs on a switch.
	Follow        *FollowHandler
	Board         *BoardHandler
	Notifications *NotificationsHandler

	Users     user.Repository
	JWTSecret []byte
	Logger    *slog.Logger
}

// Server wraps the assembled gin.Engine mounting every route.
type Server struct {
	engine *gin.Engine
}

// NewServer builds the full route tree once, at construction time, not on
// every Engine() call.
//
// No proxy is trusted (SetTrustedProxies(nil)): a client address is always
// the socket address, so a spoofed X-Forwarded-For cannot dodge a per-IP
// limit. Real proxy trust is a launch item.
//
// Route layout:
//   - GET /healthz: no middleware beyond Recovery/RequestLogger, no
//     database call, so a liveness probe stays green during a database
//     blip rather than cascading a restart (T-01-SRV-07).
//   - /v1: BodyLimit is the FIRST thing registered here (gin applies Use
//     only to routes added later), so every route below, public or signed
//     in, refuses a body over 16 KB. AuthHandler mounts directly on it and
//     writes its own "/auth/..." route strings (POST /auth/signup,
//     POST /auth/login, etc.).
//   - /v1 (public subgroup): the username suggest and availability checks.
//     The sign-up page calls them before any account exists, so they need
//     no token; they are limited per IP instead.
//   - /v1 (signed-in subgroup, `authed`): RequireAuth (a valid token) then
//     RequireUser (the account row still exists, else 401 token_invalid) are
//     applied at this group -- not per route -- so ProfileHandler inherits
//     both by construction, and so does any future route mounted here
//     (T-01-SRV-03). There is no verified check: sign-up has no email step.
//     The change-password route (POST /me/password) is mounted on this group
//     when Deps carries an AccountHandler. The daily spin routes (time zone,
//     Today, spin, bonus spin, wheel) are mounted on this group when Deps
//     carries them; so are the test-only clock and pretend-day routes, which
//     exist only when the server was started in test mode (see
//     SpinTestTools). The entry routes (photo ticket, post, read, edit,
//     delete, skip) are mounted on this group when Deps carries an
//     EntryHandler, and then the spin handler is given that handler's photo
//     links so Today's entry summary has a link and never a key.
func NewServer(deps Deps) *Server {
	engine := gin.New()
	// A nil list is valid and never errors; handle the error anyway.
	if err := engine.SetTrustedProxies(nil); err != nil {
		panic("httpapi: SetTrustedProxies(nil): " + err.Error())
	}
	engine.Use(middleware.Recovery(deps.Logger), middleware.RequestLogger(deps.Logger))

	engine.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	v1 := engine.Group("/v1")
	v1.Use(middleware.BodyLimit(middleware.MaxBodyBytes))
	deps.Auth.Register(v1)

	public := v1.Group("")
	deps.Username.Register(public)

	authed := v1.Group("")
	authed.Use(middleware.RequireAuth(deps.JWTSecret), middleware.RequireUser(deps.Users))
	deps.Profile.Register(authed)
	if deps.Account != nil {
		deps.Account.Register(authed)
	}
	if deps.Spin != nil {
		if deps.Entry != nil && deps.Entry.photos != nil {
			deps.Spin.PhotoURL = deps.Entry.photos.PublicURL
		}
		deps.Spin.Register(authed)
	}
	if deps.Wheel != nil {
		deps.Wheel.Register(authed)
	}
	if deps.Entry != nil {
		deps.Entry.Register(authed)
	}
	if deps.Diary != nil {
		deps.Diary.Register(authed)
	}
	if deps.Follow != nil {
		deps.Follow.Register(authed)
	}
	if deps.Board != nil {
		deps.Board.Register(authed)
	}
	if deps.Notifications != nil {
		deps.Notifications.Register(authed)
	}
	if deps.TestTools != nil {
		deps.TestTools.Register(authed)
	}

	return &Server{engine: engine}
}

// Engine returns the assembled gin.Engine, ready to be wrapped in an
// http.Server with explicit timeouts (cmd/api/main.go).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
