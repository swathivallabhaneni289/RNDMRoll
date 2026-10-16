// Package httpapi: follow.go serves one-way follows (find a person by exact
// username, follow, unfollow or cancel, remove a follower, the lists, accept
// and decline a request), blocks and the notification settings. The caller
// comes only from the token; ids in paths are parsed in Go, and one that is
// not a UUID is 404 not_found.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// FollowHandler serves the follow, block and notification settings routes.
type FollowHandler struct {
	svc *social.Service
}

// NewFollowHandler builds a FollowHandler over the social service.
func NewFollowHandler(svc *social.Service) *FollowHandler {
	return &FollowHandler{svc: svc}
}

// Register mounts the routes on rg (RequireAuth and RequireUser applied).
// Limits are per person, a minute.
func (h *FollowHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/people/lookup", perSubject(30), h.lookup)
	rg.GET("/follows", perSubject(60), h.lists)
	rg.POST("/follows", perSubject(20), h.follow)
	rg.POST("/follows/:user_id/unfollow", perSubject(20), h.unfollow)
	rg.POST("/follows/requests/:id/accept", perSubject(20), h.accept)
	rg.POST("/follows/requests/:id/decline", perSubject(20), h.decline)
	rg.POST("/followers/:user_id/remove", perSubject(20), h.removeFollower)
	rg.GET("/me/blocks", perSubject(60), h.blocks)
	rg.POST("/me/blocks", perSubject(20), h.block)
	rg.DELETE("/me/blocks/:user_id", perSubject(20), h.unblock)
	rg.GET("/me/notification-settings", perSubject(60), h.settings)
	rg.PATCH("/me/notification-settings", perSubject(30), h.patchSettings)
}

// UserIDRequest is the body of POST /follows and POST /me/blocks.
type UserIDRequest struct {
	UserID *string `json:"user_id"`
}

// NotificationSettingsRequest is the PATCH /me/notification-settings body;
// a missing or null switch is unchanged.
type NotificationSettingsRequest struct {
	Reactions  *bool `json:"reactions"`
	Comments   *bool `json:"comments"`
	Milestones *bool `json:"milestones"`
}

type requestView struct {
	RequestID string       `json:"request_id"`
	Person    personView   `json:"person"`
	At        spin.Instant `json:"at"`
}

func requestViews(list []social.Request) []requestView {
	out := make([]requestView, 0, len(list))
	for _, r := range list {
		out = append(out, requestView{RequestID: r.ID.String(), Person: newPersonView(r.Person), At: spin.Instant(r.At)})
	}
	return out
}

func personViews(list []social.Person) []personView {
	out := make([]personView, 0, len(list))
	for _, p := range list {
		out = append(out, newPersonView(p))
	}
	return out
}

// bodyUserID reads a required user_id from the body: missing is required,
// not a UUID is invalid (400, field user_id). The answer is written on
// failure.
func bodyUserID(c *gin.Context) (uuid.UUID, bool) {
	var req UserIDRequest
	if !bindEntryBody(c, &req) {
		return uuid.UUID{}, false
	}
	if req.UserID == nil || *req.UserID == "" {
		RespondFieldError(c, "user_id", reasonRequired)
		return uuid.UUID{}, false
	}
	s, ok := canonicalID(*req.UserID)
	if !ok {
		RespondFieldError(c, "user_id", reasonInvalid)
		return uuid.UUID{}, false
	}
	return uuid.MustParse(s), true
}

// emptyBody reads a body that carries nothing ({} or none).
func emptyBody(c *gin.Context) bool {
	var req struct{}
	return bindEntryBody(c, &req)
}

func (h *FollowHandler) lookup(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	name, reason := social.CleanUsername(c.Query("username"))
	if reason != "" {
		RespondFieldError(c, "username", reason)
		return
	}
	res, err := h.svc.Lookup(c.Request.Context(), u.ID, name)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{
		"person":      newPersonView(res.Person),
		"relation":    string(res.Relation),
		"follows_you": res.FollowsYou,
		"private":     res.Private,
	})
}

func (h *FollowHandler) lists(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	l, err := h.svc.Lists(c.Request.Context(), u.ID)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{
		"followers": personViews(l.Followers),
		"following": personViews(l.Following),
		"incoming":  requestViews(l.Incoming),
		"outgoing":  requestViews(l.Outgoing),
	})
}

func (h *FollowHandler) follow(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	target, ok := bodyUserID(c)
	if !ok {
		return
	}
	rel, err := h.svc.Follow(c.Request.Context(), u.ID, target)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"relation": string(rel)})
}

func (h *FollowHandler) unfollow(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	target, ok := pathUUID(c, "user_id")
	if !ok || !emptyBody(c) {
		return
	}
	if err := h.svc.Unfollow(c.Request.Context(), u.ID, target); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"relation": string(social.RelationNone)})
}

func (h *FollowHandler) removeFollower(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	follower, ok := pathUUID(c, "user_id")
	if !ok || !emptyBody(c) {
		return
	}
	if err := h.svc.RemoveFollower(c.Request.Context(), u.ID, follower); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{})
}

func (h *FollowHandler) accept(c *gin.Context) {
	h.resolve(c, true)
}

func (h *FollowHandler) decline(c *gin.Context) {
	h.resolve(c, false)
}

func (h *FollowHandler) resolve(c *gin.Context, accept bool) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok || !emptyBody(c) {
		return
	}
	var err error
	if accept {
		err = h.svc.Accept(c.Request.Context(), u.ID, id)
	} else {
		err = h.svc.Decline(c.Request.Context(), u.ID, id)
	}
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{})
}

type blockedView struct {
	User      personView   `json:"user"`
	BlockedAt spin.Instant `json:"blocked_at"`
}

func (h *FollowHandler) blocks(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	list, err := h.svc.Blocks(c.Request.Context(), u.ID)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	items := make([]blockedView, 0, len(list))
	for _, b := range list {
		items = append(items, blockedView{User: newPersonView(b.Person), BlockedAt: spin.Instant(b.At)})
	}
	Respond(c, http.StatusOK, gin.H{"items": items})
}

func (h *FollowHandler) block(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	target, ok := bodyUserID(c)
	if !ok {
		return
	}
	if err := h.svc.Block(c.Request.Context(), u.ID, target); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{})
}

func (h *FollowHandler) unblock(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	target, ok := pathUUID(c, "user_id")
	if !ok {
		return
	}
	if err := h.svc.Unblock(c.Request.Context(), u.ID, target); err != nil {
		respondFeedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func settingsBody(s social.Settings) gin.H {
	return gin.H{"reactions": s.Reactions, "comments": s.Comments, "milestones": s.Milestones}
}

func (h *FollowHandler) settings(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	s, err := h.svc.Settings(c.Request.Context(), u.ID)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, settingsBody(s))
}

func (h *FollowHandler) patchSettings(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req NotificationSettingsRequest
	if !bindEntryBody(c, &req) {
		return
	}
	s, err := h.svc.UpdateSettings(c.Request.Context(), u.ID, social.SettingsPatch{
		Reactions: req.Reactions, Comments: req.Comments, Milestones: req.Milestones,
	})
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, settingsBody(s))
}
