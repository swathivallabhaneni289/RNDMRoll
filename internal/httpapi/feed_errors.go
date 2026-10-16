// Package httpapi: feed_errors.go holds the error codes of the Phase 5
// routes (follows, the Board, posts, comments, notifications) and the one
// function that maps their errors. errors.go is not edited.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

const (
	// CodeOwnPost: a reaction on your own post. 409.
	CodeOwnPost ErrorCode = "own_post"
	// CodeCommentsOff: the owner turned comments off on this post. 409.
	CodeCommentsOff ErrorCode = "comments_off"
)

// respondFeedError maps the Phase 5 errors. A person, request, post or
// comment the caller may not reach is always the same 404 not_found as one
// that does not exist, never a 403. Anything else goes to respondSpinError
// (a missing zone is 400 field time_zone) and then RespondError.
func respondFeedError(c *gin.Context, err error) {
	var problem *board.FieldProblem
	switch {
	case errors.Is(err, social.ErrNotFound), errors.Is(err, board.ErrNotFound):
		Respond(c, http.StatusNotFound, gin.H{"error": string(CodeNotFound)})
	case errors.Is(err, board.ErrOwnPost):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeOwnPost)})
	case errors.Is(err, board.ErrCommentsOff):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeCommentsOff)})
	case errors.As(err, &problem):
		RespondFieldError(c, problem.Field, problem.Reason)
	default:
		respondSpinError(c, err)
	}
}

// respondNotFound writes the one 404 body.
func respondNotFound(c *gin.Context) {
	Respond(c, http.StatusNotFound, gin.H{"error": string(CodeNotFound)})
}

// personView is how another account is shown. It is built in one place so
// a new field can never leak by accident.
type personView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Username string  `json:"username"`
	PhotoURL *string `json:"photo_url"`
}

func newPersonView(p social.Person) personView {
	return personView{ID: p.ID.String(), Name: p.Name, Username: p.Username, PhotoURL: p.PhotoURL}
}

// pathUUID reads a path id. Anything but a canonical UUID is 404 not_found
// (written here) and ok false; it is never sent to SQL.
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	s, ok := canonicalID(c.Param(name))
	if !ok {
		respondNotFound(c)
		return uuid.UUID{}, false
	}
	return uuid.MustParse(s), true
}
