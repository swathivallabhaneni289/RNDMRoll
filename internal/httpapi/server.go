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

	return &Server{engine: engine}
}

// Engine returns the assembled gin.Engine, ready to be wrapped in an
// http.Server with explicit timeouts (cmd/api/main.go).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
