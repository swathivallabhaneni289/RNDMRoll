// Package httpapi: second_looks.go serves asking for a second look at a post
// or comment an admin removed (POST /removals/:id/second-look, developer,
// 2026-11-19). :id is the removal's full id, the bell item's removal field;
// the body is {"text", "photo_keys"}: the owner's words, 1 to 500 characters
// after trimming, and up to 3 photos uploaded with POST
// /entry-photos/upload-url. Someone else's removal is the same 404 as a
// miss. The words are never logged.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/secondlook"
)

// The refusals of a second look, each a 409 the app words in a few words.
const (
	// CodeSecondLookAsked: one request per removal, and it was made.
	CodeSecondLookAsked ErrorCode = "second_look_asked"
	// CodeSecondLookTooLate: the removal's 30 days are over.
	CodeSecondLookTooLate ErrorCode = "second_look_too_late"
	// CodeSecondLookBack: a Bring it back answered the removal already.
	CodeSecondLookBack ErrorCode = "second_look_back"
)

// secondLookAsked is the bell's state of a removal once a second look is
// asked about it.
const secondLookAsked = "asked"

// SecondLookHandler serves the owner's request for a second look.
type SecondLookHandler struct {
	svc *secondlook.Service
}

// NewSecondLookHandler builds the handler over svc.
func NewSecondLookHandler(svc *secondlook.Service) *SecondLookHandler {
	return &SecondLookHandler{svc: svc}
}

// Register mounts the route on rg. The limit is per person, a minute.
func (h *SecondLookHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/removals/:id/second-look", perSubject(10), h.ask)
}

// SecondLookRequest is the POST /removals/:id/second-look body.
type SecondLookRequest struct {
	Text *string `json:"text"`
	// PhotoKeys are up to 3 of the caller's own uploads, none used by
	// anything else.
	PhotoKeys []string `json:"photo_keys"`
}

// ask answers 201 {"second_look": "asked"} for a new request, the state the
// bell item shows from then on, and 200 with the same for the phone's retry
// of one stored already, which tells no one again.
func (h *SecondLookHandler) ask(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req SecondLookRequest
	if !bindEntryBody(c, &req) {
		return
	}
	text := ""
	if req.Text != nil {
		text = *req.Text
	}
	made, err := h.svc.Ask(c.Request.Context(), u.ID, id, text, req.PhotoKeys)
	var problem *secondlook.FieldProblem
	switch {
	case errors.As(err, &problem):
		RespondFieldError(c, problem.Field, problem.Reason)
	case errors.Is(err, secondlook.ErrNotFound):
		respondNotFound(c)
	case errors.Is(err, secondlook.ErrAsked):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSecondLookAsked)})
	case errors.Is(err, secondlook.ErrTooLate):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSecondLookTooLate)})
	case errors.Is(err, secondlook.ErrBack):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSecondLookBack)})
	case err != nil:
		respondFeedError(c, err)
	case made:
		Respond(c, http.StatusCreated, gin.H{"second_look": secondLookAsked})
	default:
		Respond(c, http.StatusOK, gin.H{"second_look": secondLookAsked})
	}
}
