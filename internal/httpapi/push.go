// Package httpapi: push.go keeps the phone's Expo push token for the
// signed-in person (POST after sign-in, DELETE on log out). The tokens are
// kept whatever PUSH_DRIVER says; sending is internal/push's. A token is tied
// to the session that sent it and is sent to only while that session lasts.
package httpapi

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/push"
)

// PushTokenHandler serves POST and DELETE /me/push-tokens.
type PushTokenHandler struct {
	repo push.Repository
}

// NewPushTokenHandler builds a PushTokenHandler.
func NewPushTokenHandler(repo push.Repository) *PushTokenHandler {
	return &PushTokenHandler{repo: repo}
}

// Register mounts the routes on rg. Limits are per person, a minute.
func (h *PushTokenHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/me/push-tokens", perSubject(20), h.save)
	rg.DELETE("/me/push-tokens", perSubject(20), h.remove)
}

// PushTokenRequest is the body of both routes; platform only on POST.
type PushTokenRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *PushTokenHandler) save(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req PushTokenRequest
	if !bindEntryBody(c, &req) {
		return
	}
	token := strings.TrimSpace(req.Token)
	if !push.ValidToken(token) {
		RespondFieldError(c, "token", reasonInvalid)
		return
	}
	if !push.ValidPlatform(req.Platform) {
		RespondFieldError(c, "platform", reasonInvalid)
		return
	}
	// The token is tied to the session this access token belongs to, so it is
	// sent to only while that session lasts. A token issued before sessions
	// were named has none: the phone's token is then kept but not sent to
	// until the app sends it again with a newer access token.
	session, _ := middleware.SessionFromContext(c)
	if err := h.repo.SaveToken(c.Request.Context(), u.ID, session, token, req.Platform, time.Now().UTC()); err != nil {
		log.Printf("httpapi: push token save: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PushTokenHandler) remove(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req PushTokenRequest
	if !bindEntryBody(c, &req) {
		return
	}
	token := strings.TrimSpace(req.Token)
	if !push.ValidToken(token) {
		RespondFieldError(c, "token", reasonInvalid)
		return
	}
	// Only this session's token, or one whose session is over, is deleted:
	// a token a newer sign in on the phone has saved again stays (204 all the
	// same), so a DELETE that arrives late never takes that sign in's pushes.
	session, _ := middleware.SessionFromContext(c)
	if err := h.repo.DeleteToken(c.Request.Context(), u.ID, session, token); err != nil {
		log.Printf("httpapi: push token delete: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
		return
	}
	c.Status(http.StatusNoContent)
}
