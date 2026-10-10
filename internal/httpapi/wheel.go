// Package httpapi: wheel.go serves a person's own wheel: read it, and
// replace it whole. A save is one PATCH of the entire ordered list with the
// version that was read, so it is all or nothing and a stale screen cannot
// overwrite a newer wheel.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// WheelHandler serves GET /wheel and PATCH /wheel.
type WheelHandler struct {
	svc *spin.Service
}

// NewWheelHandler constructs a WheelHandler over the spin service.
func NewWheelHandler(svc *spin.Service) *WheelHandler {
	return &WheelHandler{svc: svc}
}

// Register mounts the routes on rg (RequireAuth and RequireUser applied by
// the caller): 60 reads and 20 saves a minute per person.
func (h *WheelHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/wheel", perSubject(60), h.get)
	rg.PATCH("/wheel", perSubject(20), h.save)
}

// WheelCategoryBody is one category in a PATCH /wheel body. id is null for a
// new category and the id of one of the caller's live categories otherwise.
type WheelCategoryBody struct {
	ID     *string `json:"id"`
	Name   string  `json:"name"`
	Weight int     `json:"weight"`
}

// SaveWheelRequest is the PATCH /wheel body: the version that was read and
// the whole new list, in order.
type SaveWheelRequest struct {
	Version    *int                `json:"version"`
	Categories []WheelCategoryBody `json:"categories"`
}

// wheelBody is the GET /wheel and PATCH /wheel answer: the live categories
// (archived ones are never listed), the version and the limits the screen
// enforces.
func wheelBody(w spin.WheelView) gin.H {
	categories := make([]gin.H, len(w.Categories))
	for i, c := range w.Categories {
		categories[i] = gin.H{"id": c.ID, "name": c.Name, "weight": c.Weight, "position": c.Position}
	}
	return gin.H{
		"version":    w.Version,
		"categories": categories,
		"limits": gin.H{
			"min":        spin.MinWheelEntries,
			"max":        spin.MaxWheelEntries,
			"name_max":   spin.MaxWheelNameRunes,
			"weight_min": spin.MinWeight,
			"weight_max": spin.MaxWeight,
		},
	}
}

func (h *WheelHandler) get(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	w, err := h.svc.Wheel(c.Request.Context(), u.ID.String())
	if err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, wheelBody(w))
}

func (h *WheelHandler) save(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req SaveWheelRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}
	if req.Version == nil {
		RespondFieldError(c, "version", reasonRequired)
		return
	}
	if req.Categories == nil {
		RespondFieldError(c, "categories", reasonRequired)
		return
	}
	list := make([]spin.WheelInput, len(req.Categories))
	for i, cat := range req.Categories {
		list[i] = spin.WheelInput{ID: cat.ID, Name: cat.Name, Weight: cat.Weight}
	}
	w, err := h.svc.SaveWheel(c.Request.Context(), u.ID.String(), *req.Version, list)
	if err != nil {
		respondSpinError(c, err)
		return
	}
	Respond(c, http.StatusOK, wheelBody(w))
}
