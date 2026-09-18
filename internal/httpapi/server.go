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

// Deps bundles every dependency NewServer needs to assemble the full Phase
// 1 route tree: the five wave 4 handlers, the user repository (for
// RequireVerified's own lookup at the authenticated group boundary), the
// JWT signing secret (for RequireAuth), and a logger shared by the two
// cross-cutting middlewares.
type Deps struct {
	Auth        *AuthHandler
	VerifyEmail *VerifyEmailHandler
	OAuth       *OAuthHandler
	Profile     *ProfileHandler
	Username    *UsernameHandler

	Users     user.Repository
	JWTSecret []byte
	Logger    *slog.Logger
}

// Server wraps the assembled gin.Engine mounting every Phase 1 route.
type Server struct {
	engine *gin.Engine
}

// NewServer builds the full route tree once, at construction time, not on
// every Engine() call.
//
// Route layout:
//   - GET /healthz: no middleware beyond Recovery/RequestLogger, no
//     database call, so a liveness probe stays green during a database
//     blip rather than cascading a restart (T-01-SRV-07).
//   - /v1: AuthHandler and OAuthHandler mount directly here -- both write
//     their own "/auth/..." route strings (POST /auth/signup, POST
//     /auth/oauth/apple, etc.), so they expect the bare /v1 group.
//   - /v1/auth: VerifyEmailHandler mounts on this nested group -- it writes
//     bare route strings ("/verify-email", "/verify-email/resend",
//     "/verify-email/callback") that expect the "/auth" prefix already
//     applied by the caller, per its own SUMMARY's wiring contract. This
//     produces the same final paths (/v1/auth/verify-email, ...) that
//     AuthHandler's routes sit alongside, with no path or symbol collision.
//   - /v1 (authenticated subgroup): RequireAuth then RequireVerified are
//     applied at this group -- not per route -- so ProfileHandler and
//     UsernameHandler inherit both by construction, and so does any future
//     route mounted here (T-01-SRV-03).
func NewServer(deps Deps) *Server {
	engine := gin.New()
	engine.Use(middleware.Recovery(deps.Logger), middleware.RequestLogger(deps.Logger))

	engine.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	v1 := engine.Group("/v1")
	deps.Auth.Register(v1)
	deps.OAuth.Register(v1)

	authGroup := v1.Group("/auth")
	deps.VerifyEmail.Register(authGroup)

	authed := v1.Group("")
	authed.Use(middleware.RequireAuth(deps.JWTSecret), middleware.RequireVerified(deps.Users))
	deps.Profile.Register(authed)
	deps.Username.Register(authed)

	return &Server{engine: engine}
}

// Engine returns the assembled gin.Engine, ready to be wrapped in an
// http.Server with explicit timeouts (cmd/api/main.go).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
