// Package httpapi: notifications.go serves the bell: the derived list, the
// unread check for the dot, and the seen mark. Sections, unread flags and
// labels are the service's; this file formats instants only as RFC 3339.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// NotificationsHandler serves GET /notifications, GET /notifications/summary
// and POST /notifications/seen.
type NotificationsHandler struct {
	svc    *notifications.Service
	photos storage.EntryPhotoStore
	// photoLink signs the profile photos of the people in the list
	// (Deps.PhotoLink; NewServer sets it). Nil sends none.
	photoLink func(string) string
}

// NewNotificationsHandler builds a NotificationsHandler. photos signs the
// links of the small post pictures.
func NewNotificationsHandler(svc *notifications.Service, photos storage.EntryPhotoStore) *NotificationsHandler {
	return &NotificationsHandler{svc: svc, photos: photos}
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
	Text         *string      `json:"text"`
	RequestID    *string      `json:"request_id"`
	// PhotoURL is the post's first photo, "" when the item has none.
	PhotoURL string `json:"photo_url"`
	// Ref, Reason, Removal and SecondLook are sent on an admin's removal
	// only (post_removed, comment_removed): the removal's short reference,
	// the reason the admin gave, left out with none, the removal's full id,
	// which POST /removals/:id/second-look takes (the app never reads it
	// from the item id), and the second look on it: "asked", "kept" or
	// "restored", left out while none was asked and it is not back. The
	// reason is a report reason's key ("hate"), never words: the app words
	// it. The key is never "email": no feed answer carries one.
	Ref        string `json:"ref,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Removal    string `json:"removal,omitempty"`
	SecondLook string `json:"second_look,omitempty"`
}

// newItemView shows one item: photoURL signs the post's picture, personLink
// the actor's profile photo. The notification queries already left out what
// the caller may not see.
func newItemView(it notifications.Item, photoURL, personLink func(string) string) itemView {
	v := itemView{
		ID: it.ID, Kind: it.Kind, At: spin.Instant(it.At), AtLabel: it.AtLabel, Section: it.Section,
		Unread: it.Unread, CategoryName: it.CategoryName, Text: it.Text, Ref: it.Ref, Reason: it.Reason,
		SecondLook: it.SecondLook,
	}
	if it.Removal != nil {
		v.Removal = it.Removal.String()
	}
	if it.Actor != nil {
		p := newPersonView(*it.Actor, personLink)
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
	if it.PhotoKey != nil && *it.PhotoKey != "" {
		v.PhotoURL = photoURL(*it.PhotoKey)
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
		items = append(items, newItemView(it, h.photos.Link, h.photoLink))
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
