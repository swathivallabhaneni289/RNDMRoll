// Package httpapi: spin.go serves the daily spin: the time zone report,
// Today, the spin and the bonus spin. Every route is scoped to the caller:
// the person comes only from the token (finishedUser), no route takes a
// person's or a spin's id, and the server decides the date, the window and
// the winner. The phone only reports its time zone.
package httpapi

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// perSubject limits a route to requests a minute per signed-in person.
func perSubject(requests int) gin.HandlerFunc {
	return middleware.RateLimit(middleware.LimitConfig{
		Requests: requests,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyBySubject,
	})
}

// SpinHandler serves POST /time-zone, GET /today, POST /spin and
// POST /spin/bonus.
type SpinHandler struct {
	svc *spin.Service
}

// NewSpinHandler constructs a SpinHandler over the spin service.
func NewSpinHandler(svc *spin.Service) *SpinHandler {
	return &SpinHandler{svc: svc}
}

// Register mounts the routes on rg, which must carry RequireAuth and
// RequireUser (NewServer applies both to the signed-in group). Limits are
// per person: 20 zone reports, 60 Today reads, 10 spins and 10 bonus spins a
// minute.
func (h *SpinHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/time-zone", perSubject(20), h.setTimeZone)
	rg.GET("/today", perSubject(60), h.today)
	rg.POST("/spin", perSubject(10), h.spin)
	rg.POST("/spin/bonus", perSubject(10), h.bonus)
}

// SetTimeZoneRequest is the POST /time-zone body.
type SetTimeZoneRequest struct {
	TimeZone *string `json:"time_zone"`
}

// SpinRequest is the POST /spin body. for_date is optional (an empty body
// counts as {}); when sent it is the date the phone believes it is spinning
// for.
type SpinRequest struct {
	ForDate *string `json:"for_date"`
}

// BonusRequest is the POST /spin/bonus body. Both fields are required:
// after_seq is the number of the spin the phone is looking at and for_date
// the date it believes it is, so a retry or a stale screen never spends a
// bonus spin by mistake.
type BonusRequest struct {
	AfterSeq *int    `json:"after_seq"`
	ForDate  *string `json:"for_date"`
}

func (h *SpinHandler) setTimeZone(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req SetTimeZoneRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if req.TimeZone == nil || *req.TimeZone == "" {
		RespondFieldError(c, "time_zone", reasonRequired)
		return
	}
	name, err := h.svc.SetTimeZone(c.Request.Context(), u.ID.String(), *req.TimeZone)
	if err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"time_zone": name})
}

func (h *SpinHandler) today(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	today, err := h.svc.Today(c.Request.Context(), u.ID.String())
	if err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, today)
}

// spin answers 201 for a new spin and 200 for the same spin again, both
// with {"already": bool, "today": Today}.
func (h *SpinHandler) spin(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req SpinRequest
	// An empty body is the same as {}: io.EOF is not an error here.
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil && !errors.Is(err, io.EOF) {
		RespondValidationError(c, err)
		return
	}
	var forDate *spin.Date
	if req.ForDate != nil {
		d, err := spin.ParseDate(*req.ForDate)
		if err != nil {
			RespondFieldError(c, "for_date", reasonInvalid)
			return
		}
		forDate = &d
	}
	out, err := h.svc.Spin(c.Request.Context(), u.ID.String(), forDate)
	if err != nil {
		respondSpinError(c, err)
		return
	}
	status := http.StatusCreated
	if out.Already {
		status = http.StatusOK
	}
	Respond(c, status, gin.H{"already": out.Already, "today": out.Today})
}

func (h *SpinHandler) bonus(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req BonusRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if req.AfterSeq == nil {
		RespondFieldError(c, "after_seq", reasonRequired)
		return
	}
	if *req.AfterSeq < 0 {
		RespondFieldError(c, "after_seq", reasonInvalid)
		return
	}
	if req.ForDate == nil {
		RespondFieldError(c, "for_date", reasonRequired)
		return
	}
	forDate, err := spin.ParseDate(*req.ForDate)
	if err != nil {
		RespondFieldError(c, "for_date", reasonInvalid)
		return
	}
	out, err := h.svc.Bonus(c.Request.Context(), u.ID.String(), *req.AfterSeq, forDate)
	if err != nil {
		respondSpinError(c, err)
		return
	}
	status := http.StatusCreated
	if out.Already {
		status = http.StatusOK
	}
	Respond(c, status, gin.H{"already": out.Already, "today": out.Today})
}
