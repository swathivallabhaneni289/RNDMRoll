// Package httpapi: spin_testtools.go holds the test-only routes that move one
// account's clock, mark pretend posted days and clear an account's spins, so
// a person can walk a whole week in minutes.
//
// They exist only when the server was started with APP_ENV=test,
// SPIN_TEST_TOOLS=1, a non-empty SPIN_TEST_USER_EMAILS and a _dev or _test
// database (config.Load refuses to start otherwise), and NewServer registers
// them only when a handler is passed. Each route checks again: the handler
// must be enabled, and the caller's email must be on the list (404
// otherwise, so the route looks absent). A real account never goes on the
// list. Every route acts on the caller only, and the clock it moves is the
// spin slice's: tokens, the rate limiter and refresh expiry never see it.
package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// TestClock is a clock a test route can read and move.
type TestClock interface {
	clock.Clock
	clock.ClockControl
}

// maxTestDatesPerCall is the most pretend days one call may mark.
const maxTestDatesPerCall = 7

// SpinTestTools serves POST /test/clock, POST /test/posted-days and
// POST /test/reset.
type SpinTestTools struct {
	enabled bool
	svc     *spin.Service
	clk     TestClock
	allowed map[string]bool
}

// NewSpinTestTools builds the handler. enabled mirrors config's
// SpinTestTools; emails is the allow-list (compared lower-cased).
func NewSpinTestTools(enabled bool, svc *spin.Service, clk TestClock, emails []string) *SpinTestTools {
	allowed := make(map[string]bool, len(emails))
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			allowed[e] = true
		}
	}
	return &SpinTestTools{enabled: enabled, svc: svc, clk: clk, allowed: allowed}
}

// Register mounts the routes on rg (RequireAuth and RequireUser applied by
// the caller), 60 calls a minute per person across the three.
func (h *SpinTestTools) Register(rg *gin.RouterGroup) {
	g := rg.Group("/test", perSubject(60))
	g.POST("/clock", h.setClock)
	g.POST("/posted-days", h.postedDays)
	g.POST("/reset", h.reset)
}

// gate runs the checks every test route makes, in order: the tools are on, the
// caller is a listed account (404 otherwise, as if the route did not exist),
// and the account is finished. It returns the caller's id.
func (h *SpinTestTools) gate(c *gin.Context) (string, bool) {
	if !h.enabled {
		RespondError(c, user.ErrNotFound)
		return "", false
	}
	u, err := middleware.UserFromContext(c)
	if err != nil {
		RespondError(c, user.ErrTokenInvalid)
		return "", false
	}
	if !h.allowed[strings.ToLower(u.Email)] {
		RespondError(c, user.ErrNotFound)
		return "", false
	}
	if !u.OnboardingComplete() {
		Respond(c, http.StatusForbidden, gin.H{"error": string(CodeProfileIncomplete)})
		return "", false
	}
	return u.ID.String(), true
}

// SetClockRequest is the POST /test/clock body: exactly one of now (an RFC
// 3339 instant), local (a wall clock reading in the caller's zone, written
// 2026-10-07T19:59:30) or reset (true).
type SetClockRequest struct {
	Now   *string `json:"now"`
	Local *string `json:"local"`
	Reset *bool   `json:"reset"`
}

func (h *SpinTestTools) setClock(c *gin.Context) {
	userID, ok := h.gate(c)
	if !ok {
		return
	}
	var req SetClockRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	given := 0
	for _, present := range []bool{req.Now != nil, req.Local != nil, req.Reset != nil} {
		if present {
			given++
		}
	}
	if given != 1 {
		// Send exactly one of now, local and reset.
		RespondFieldError(c, "now", reasonInvalid)
		return
	}

	switch {
	case req.Reset != nil:
		if !*req.Reset {
			RespondFieldError(c, "reset", reasonInvalid)
			return
		}
		h.clk.Reset(userID)

	case req.Now != nil:
		target, err := time.Parse(time.RFC3339, *req.Now)
		if err != nil {
			RespondFieldError(c, "now", reasonInvalid)
			return
		}
		h.clk.SetNow(userID, target)

	default:
		civil, err := time.Parse("2006-01-02T15:04:05", *req.Local)
		if err != nil {
			RespondFieldError(c, "local", reasonInvalid)
			return
		}
		loc, _, err := h.svc.ZoneOf(c.Request.Context(), userID)
		if err != nil {
			respondSpinError(c, err)
			return
		}
		// A reading that happens twice takes the first; one that falls in a
		// spring-forward gap never happens and is refused.
		target, ok := spin.InstantOfLocal(civil, loc)
		if !ok {
			RespondFieldError(c, "local", reasonInvalid)
			return
		}
		h.clk.SetNow(userID, target)
	}

	Respond(c, http.StatusOK, gin.H{
		"now":     spin.Instant(h.clk.NowFor(userID)),
		"shifted": h.clk.Shifted(userID),
	})
}

// PostedDaysRequest is the POST /test/posted-days body: 1 to 7 dates
// (YYYY-MM-DD), none after the caller's local today and none older than 60
// days.
type PostedDaysRequest struct {
	Dates []string `json:"dates"`
}

func (h *SpinTestTools) postedDays(c *gin.Context) {
	userID, ok := h.gate(c)
	if !ok {
		return
	}
	var req PostedDaysRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if len(req.Dates) == 0 {
		RespondFieldError(c, "dates", reasonRequired)
		return
	}
	if len(req.Dates) > maxTestDatesPerCall {
		RespondFieldError(c, "dates", reasonInvalid)
		return
	}
	dates := make([]spin.Date, len(req.Dates))
	for i, s := range req.Dates {
		d, err := spin.ParseDate(s)
		if err != nil {
			RespondFieldError(c, "dates", reasonInvalid)
			return
		}
		dates[i] = d
	}
	res, err := h.svc.MarkTestDays(c.Request.Context(), userID, dates)
	if err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"week": res.Week, "bonus_held": res.BonusHeld})
}

// ResetRequest is the POST /test/reset body: nothing is read from it, but an
// oversize or malformed one is refused like on every other route.
type ResetRequest struct{}

func (h *SpinTestTools) reset(c *gin.Context) {
	userID, ok := h.gate(c)
	if !ok {
		return
	}
	var req ResetRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil && !errors.Is(err, io.EOF) {
		RespondValidationError(c, err)
		return
	}
	if err := h.svc.ResetTestData(c.Request.Context(), userID); err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"ok": true})
}
