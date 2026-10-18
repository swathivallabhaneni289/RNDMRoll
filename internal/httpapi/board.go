// Package httpapi: board.go serves the Board, the post page, the one-tap
// reaction and comments. Every photo link and time label comes from the
// board service; this file formats instants only as RFC 3339.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// BoardHandler serves GET /board, GET /posts/:id, POST /posts/:id/reaction,
// POST /posts/:id/comments and POST /comments/:id/delete.
type BoardHandler struct {
	svc *board.Service
	// removeFiles deletes photo files once their rows are gone (a deleted
	// comment's photo). NewServer sets it from the entry handler; nil skips.
	removeFiles func(ctx context.Context, keys []string)
}

// NewBoardHandler builds a BoardHandler over the board service.
func NewBoardHandler(svc *board.Service) *BoardHandler {
	return &BoardHandler{svc: svc}
}

// Register mounts the routes on rg. Limits are per person, a minute.
func (h *BoardHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/board", perSubject(60), h.board)
	rg.GET("/posts/:id", perSubject(60), h.post)
	rg.POST("/posts/:id/reaction", perSubject(60), h.reaction)
	rg.POST("/posts/:id/comments", perSubject(20), h.comment)
	rg.POST("/comments/:id/delete", perSubject(20), h.deleteComment)
	rg.POST("/posts/:id/report", perSubject(10), h.reportPost)
	rg.POST("/comments/:id/report", perSubject(10), h.reportComment)
	rg.POST("/comments/:id/hide", perSubject(30), h.hideComment)
	rg.POST("/comments/:id/unhide", perSubject(30), h.unhideComment)
}

// ReactionRequest is the POST /posts/:id/reaction body. The key must be
// present: a name sets, null clears.
type ReactionRequest struct {
	Reaction json.RawMessage `json:"reaction"`
}

// CommentRequest is the POST /posts/:id/comments body. ParentID, when set,
// makes the comment a reply to that comment (missing or null: a top-level
// comment).
type CommentRequest struct {
	Text     *string `json:"text"`
	ParentID *string `json:"parent_id"`
	// PhotoKey is one photo uploaded with POST /entry-photos/upload-url
	// (the caller's own key). With a photo the text may be empty.
	PhotoKey *string `json:"photo_key"`
}

type cardView struct {
	EntryID         string           `json:"entry_id"`
	Author          personView       `json:"author"`
	Mine            bool             `json:"mine"`
	LocalDate       string           `json:"local_date"`
	Category        spin.CategoryRef `json:"category"`
	Title           string           `json:"title"`
	Stars           int              `json:"stars"`
	Reaction        *string          `json:"reaction"`
	PhotoURL        string           `json:"photo_url"`
	Late            bool             `json:"late"`
	PostedAt        spin.Instant     `json:"posted_at"`
	PostedLocalTime string           `json:"posted_local_time"`
	CommentCount    int              `json:"comment_count"`
	MyReaction      *string          `json:"my_reaction"`
}

func newCardView(c board.Card) cardView {
	return cardView{
		EntryID:         c.EntryID.String(),
		Author:          newPersonView(c.Author),
		Mine:            c.Mine,
		LocalDate:       c.LocalDate,
		Category:        spin.CategoryRef{ID: c.CategoryID, Name: c.CategoryName},
		Title:           c.Title,
		Stars:           c.Stars,
		Reaction:        c.Reaction,
		PhotoURL:        c.PhotoURL,
		Late:            c.Late,
		PostedAt:        spin.Instant(c.PostedAt),
		PostedLocalTime: c.PostedLocalTime,
		CommentCount:    c.CommentCount,
		MyReaction:      c.MyReaction,
	}
}

type commentView struct {
	ID        string       `json:"id"`
	Author    personView   `json:"author"`
	Text      string       `json:"text"`
	At        spin.Instant `json:"at"`
	AtLabel   string       `json:"at_label"`
	CanDelete bool         `json:"can_delete"`
	// CanHide: the viewer owns the post and did not write the comment.
	CanHide bool `json:"can_hide"`
	// Hidden: the viewer (the post's owner) hid it. Never true for its writer.
	Hidden bool `json:"hidden"`
	// PhotoURL is the comment's photo, "" for none.
	PhotoURL string `json:"photo_url"`
	// ParentID is the top-level comment of a reply, null for a top-level
	// comment.
	ParentID *string `json:"parent_id"`
}

func newCommentView(c board.Comment) commentView {
	v := commentView{
		ID: c.ID.String(), Author: newPersonView(c.Author), Text: c.Text,
		At: spin.Instant(c.At), AtLabel: c.AtLabel, CanDelete: c.CanDelete,
		CanHide: c.CanHide, Hidden: c.Hidden, PhotoURL: c.PhotoURL,
	}
	if c.ParentID != nil {
		parent := c.ParentID.String()
		v.ParentID = &parent
	}
	return v
}

type receivedView struct {
	Person   personView   `json:"person"`
	Reaction string       `json:"reaction"`
	At       spin.Instant `json:"at"`
}

type lockedView struct {
	PostedCount int      `json:"posted_count"`
	Names       []string `json:"names"`
	Category    *string  `json:"category"`
	Action      string   `json:"action"`
}

type dayView struct {
	LocalDate string     `json:"local_date"`
	Label     string     `json:"label"`
	Cards     []cardView `json:"cards"`
}

func (h *BoardHandler) board(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var before *string
	if raw, present := c.GetQuery("before"); present {
		d, err := spin.ParseDate(raw)
		if err != nil {
			RespondFieldError(c, "before", reasonInvalid)
			return
		}
		s := d.String()
		before = &s
	}
	limit := board.DefaultDays
	if raw, present := c.GetQuery("limit"); present {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > board.MaxDays {
			RespondFieldError(c, "limit", reasonInvalid)
			return
		}
		limit = n
	}
	b, err := h.svc.Board(c.Request.Context(), u.ID, before, limit)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	days := make([]dayView, 0, len(b.Days))
	for _, d := range b.Days {
		cards := make([]cardView, 0, len(d.Cards))
		for _, card := range d.Cards {
			cards = append(cards, newCardView(card))
		}
		days = append(days, dayView{LocalDate: d.LocalDate, Label: d.Label, Cards: cards})
	}
	var locked *lockedView
	if b.Locked != nil {
		names := b.Locked.Names
		if names == nil {
			names = []string{}
		}
		locked = &lockedView{PostedCount: b.Locked.PostedCount, Names: names, Category: b.Locked.Category, Action: b.Locked.Action}
	}
	Respond(c, http.StatusOK, gin.H{
		"state":       b.State,
		"day_label":   b.DayLabel,
		"locked":      locked,
		"days":        days,
		"next_before": b.NextBefore,
	})
}

func (h *BoardHandler) post(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.Post(c.Request.Context(), u.ID, id)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	comments := make([]commentView, 0, len(p.Comments))
	for _, cm := range p.Comments {
		comments = append(comments, newCommentView(cm))
	}
	photos := make([]entryPhotoView, 0, len(p.Photos))
	for _, ph := range p.Photos {
		photos = append(photos, entryPhotoView{ID: ph.ID, URL: ph.URL, Position: ph.Position})
	}
	var received []receivedView
	if p.Card.Mine {
		received = make([]receivedView, 0, len(p.Received))
		for _, r := range p.Received {
			received = append(received, receivedView{Person: newPersonView(r.Person), Reaction: r.Reaction, At: spin.Instant(r.At)})
		}
	}
	Respond(c, http.StatusOK, gin.H{
		"card":         newCardView(p.Card),
		"note":         p.Note,
		"photos":       photos,
		"comments":     comments,
		"can_react":    p.CanReact,
		"can_comment":  p.CanComment,
		"comments_off": p.CommentsOff,
		"received":     received,
	})
}

func (h *BoardHandler) reaction(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req ReactionRequest
	if !bindEntryBody(c, &req) {
		return
	}
	raw := bytes.TrimSpace(req.Reaction)
	if len(raw) == 0 {
		RespondFieldError(c, "reaction", reasonRequired)
		return
	}
	var name *string
	if !bytes.Equal(raw, []byte("null")) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			RespondFieldError(c, "reaction", reasonInvalid)
			return
		}
		name = &s
	}
	mine, err := h.svc.SetReaction(c.Request.Context(), u.ID, id, name)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"my_reaction": mine})
}

func (h *BoardHandler) comment(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req CommentRequest
	if !bindEntryBody(c, &req) {
		return
	}
	text := ""
	if req.Text != nil {
		text = *req.Text
	}
	var parent *uuid.UUID
	if req.ParentID != nil {
		s, ok := canonicalID(*req.ParentID)
		if !ok {
			RespondFieldError(c, "parent_id", reasonInvalid)
			return
		}
		p := uuid.MustParse(s)
		parent = &p
	}
	if req.PhotoKey != nil && !storage.EntryPhotoKeyOwnedBy(*req.PhotoKey, u.ID) {
		RespondFieldError(c, "photo", reasonInvalid)
		return
	}
	cm, err := h.svc.AddComment(c.Request.Context(), u.ID, id, text, parent, req.PhotoKey)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusCreated, gin.H{"comment": newCommentView(cm)})
}

func (h *BoardHandler) deleteComment(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok || !emptyBody(c) {
		return
	}
	keys, err := h.svc.DeleteComment(c.Request.Context(), u.ID, id)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	if h.removeFiles != nil {
		h.removeFiles(c.Request.Context(), keys)
	}
	Respond(c, http.StatusOK, gin.H{})
}

// hideComment answers 200 {"hidden": true}: the post's owner hides a comment
// on their post. Anyone else, and the owner's own comment, is 404.
func (h *BoardHandler) hideComment(c *gin.Context) { h.setHidden(c, true) }

// unhideComment answers 200 {"hidden": false}.
func (h *BoardHandler) unhideComment(c *gin.Context) { h.setHidden(c, false) }

func (h *BoardHandler) setHidden(c *gin.Context, hidden bool) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok || !emptyBody(c) {
		return
	}
	if err := h.svc.SetCommentHidden(c.Request.Context(), u.ID, id, hidden); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"hidden": hidden})
}
