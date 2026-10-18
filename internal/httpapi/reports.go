// Package httpapi: reports.go serves reporting a post or a comment (POST
// /posts/:id/report and /comments/:id/report, body {"reason"}). Anything the
// caller cannot see, and their own, is the same 404 as a miss; the reported
// person is never told. An admin reviews reports on the admin page.
package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

func (h *BoardHandler) reportPost(c *gin.Context) { h.report(c, h.svc.ReportPost) }

func (h *BoardHandler) reportComment(c *gin.Context) { h.report(c, h.svc.ReportComment) }

func (h *BoardHandler) report(c *gin.Context, keep func(ctx context.Context, viewer, id uuid.UUID, reason string) error) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req ReportRequest
	if !bindEntryBody(c, &req) {
		return
	}
	if req.Reason == nil {
		RespondFieldError(c, "reason", reasonRequired)
		return
	}
	err := keep(c.Request.Context(), u.ID, id, *req.Reason)
	if errors.Is(err, social.ErrBadReason) {
		RespondFieldError(c, "reason", reasonInvalid)
		return
	}
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{})
}
