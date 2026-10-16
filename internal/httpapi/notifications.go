// Package httpapi: notifications.go serves the bell: the derived list, the
// unread check for the dot, and the seen mark. Sections, unread flags and
// labels are the service's; this file formats instants only as RFC 3339.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// NotificationsHandler serves GET /notifications, GET /notifications/summary
// and POST /notifications/seen.
type NotificationsHandler struct {
	svc *notifications.Service
}

// NewNotificationsHandler builds a NotificationsHandler.
func NewNotificationsHandler(svc *notifications.Service) *NotificationsHandler {
	return &NotificationsHandler{svc: svc}
}

// Register mounts the routes on rg. Limits are per person, a minute.
func (h *NotificationsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/notifications", perSubject(60), h.list)
	rg.GET("/notifications/summary", perSubject(120), h.summary)
	rg.POST("/notifications/seen", perSubject(30), h.seen)
}

type itemView struct {
	ID           string       `json:"id"`
	Kind         string       `json:"kind"`
	At           spin.Instant `json:"at"`
	AtLabel      string       `json:"at_label"`
	Section      string       `json:"section"`
	Unread       bool         `json:"unread"`
	Actor        *personView  `json:"actor"`
	EntryID      *string      `json:"entry_id"`
	CategoryName *string      `json:"category_name"`
	Reaction     *string      `json:"reaction"`
	Text         *string      `json:"text"`
	RequestID    *string      `json:"request_id"`
}

func newItemView(it notifications.Item) itemView {
	v := itemView{
		ID: it.ID, Kind: it.Kind, At: spin.Instant(it.At), AtLabel: it.AtLabel, Section: it.Section,
		Unread: it.Unread, CategoryName: it.CategoryName, Reaction: it.Reaction, Text: it.Text,
	}
	if it.Actor != nil {
		p := newPersonView(*it.Actor)
		v.Actor = &p
	}
	if it.EntryID != nil {
		s := it.EntryID.String()
		v.EntryID = &s
	}
	if it.RequestID != nil {
		s := it.RequestID.String()
		v.RequestID = &s
	}
	return v
}

func (h *NotificationsHandler) list(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	l, err := h.svc.List(c.Request.Context(), u.ID)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	items := make([]itemView, 0, len(l.Items))
	for _, it := range l.Items {
		items = append(items, newItemView(it))
	}
	Respond(c, http.StatusOK, gin.H{"unread": l.Unread, "items": items})
}

func (h *NotificationsHandler) summary(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	unread, err := h.svc.Unread(c.Request.Context(), u.ID)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"unread": unread})
}

func (h *NotificationsHandler) seen(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	if !emptyBody(c) {
		return
	}
	if err := h.svc.MarkSeen(c.Request.Context(), u.ID); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"unread": false})
}
