// Package httpapi: person.go serves another person's page (GET
// /people/:user_id): their name, photo and numbers, the caller's relation,
// and, for an accepted follower only, the posts grid under the Board rule.
// A block either way, an unknown id and yourself are all 404 not_found.
package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/diary"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// PersonHandler serves the person page.
type PersonHandler struct {
	social *social.Service
	board  *board.Service
	diary  *diary.Service
}

// NewPersonHandler builds a PersonHandler.
func NewPersonHandler(soc *social.Service, b *board.Service, d *diary.Service) *PersonHandler {
	return &PersonHandler{social: soc, board: b, diary: d}
}

// Register mounts the route on rg (RequireAuth and RequireUser applied).
func (h *PersonHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/people/:user_id", perSubject(120), h.person)
	rg.GET("/people/:user_id/followers", perSubject(60), h.followers)
	rg.GET("/people/:user_id/following", perSubject(60), h.following)
	rg.POST("/people/:user_id/mute", perSubject(30), h.mute)
	rg.POST("/people/:user_id/unmute", perSubject(30), h.unmute)
	rg.POST("/people/:user_id/report", perSubject(10), h.report)
}

// ReportRequest is the POST /people/:user_id/report body.
type ReportRequest struct {
	Reason *string `json:"reason"`
}

type personStatsView struct {
	Posts      int `json:"posts"`
	Spins      int `json:"spins"`
	WeekStreak int `json:"week_streak"`
	Followers  int `json:"followers"`
	Following  int `json:"following"`
}

type personPostView struct {
	EntryID      string `json:"entry_id"`
	LocalDate    string `json:"local_date"`
	CategoryName string `json:"category_name"`
	Title        string `json:"title"`
	PhotoURL     string `json:"photo_url"`
	PhotoCount   int    `json:"photo_count"`
}

type personPageView struct {
	Person personView `json:"person"`
	// Bio is left out when empty. It is the one place another person's bio
	// is sent (the person page, like Instagram's).
	Bio        string            `json:"bio,omitempty"`
	Relation   social.Relation   `json:"relation"`
	FollowsYou bool              `json:"follows_you"`
	Muted      bool              `json:"muted"`
	Stats      personStatsView   `json:"stats"`
	Posts      *[]personPostView `json:"posts"`
	NextCursor *string           `json:"next_cursor"`
}

func (h *PersonHandler) person(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "user_id")
	if !ok {
		return
	}
	var after *uuid.UUID
	if raw, present := c.GetQuery("cursor"); present {
		s, ok := canonicalID(raw)
		if !ok {
			RespondFieldError(c, "cursor", reasonInvalid)
			return
		}
		a := uuid.MustParse(s)
		after = &a
	}
	limit := board.DefaultPersonPosts
	if raw, present := c.GetQuery("limit"); present {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > board.MaxPersonPosts {
			RespondFieldError(c, "limit", reasonInvalid)
			return
		}
		limit = n
	}
	ctx := c.Request.Context()
	p, err := h.social.Profile(ctx, u.ID, id)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	sum, err := h.diary.Summary(ctx, id.String())
	if err != nil {
		respondFeedError(c, err)
		return
	}
	posts, err := h.board.PersonPostCount(ctx, id)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	out := personPageView{
		Person:     newPersonView(p.Person),
		Bio:        p.Bio,
		Relation:   p.Relation,
		FollowsYou: p.FollowsYou,
		Muted:      p.Muted,
		Stats: personStatsView{
			Posts:      posts,
			Spins:      sum.Spins,
			WeekStreak: sum.WeekStreak,
			Followers:  sum.Followers,
			Following:  sum.Following,
		},
	}
	// Only an accepted follower sees the grid; anyone else gets the head.
	if p.Relation == social.RelationFollowing {
		posts, next, err := h.board.PersonPosts(ctx, u.ID, id, after, limit)
		if err != nil {
			respondFeedError(c, err)
			return
		}
		views := make([]personPostView, 0, len(posts))
		for _, post := range posts {
			views = append(views, personPostView{
				EntryID:      post.EntryID.String(),
				LocalDate:    post.LocalDate,
				CategoryName: post.CategoryName,
				Title:        post.Title,
				PhotoURL:     post.PhotoURL,
				PhotoCount:   post.PhotoCount,
			})
		}
		out.Posts = &views
		if next != nil {
			s := next.String()
			out.NextCursor = &s
		}
	}
	Respond(c, http.StatusOK, out)
}

// mute answers 200 {"muted": true}. Silent: the person is never told.
func (h *PersonHandler) mute(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "user_id")
	if !ok {
		return
	}
	if err := h.social.Mute(c.Request.Context(), u.ID, id); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"muted": true})
}

// unmute answers 200 {"muted": false}.
func (h *PersonHandler) unmute(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "user_id")
	if !ok {
		return
	}
	if err := h.social.Unmute(c.Request.Context(), u.ID, id); err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"muted": false})
}

// report answers 200 {}. A missing or unknown reason is 400 field reason.
func (h *PersonHandler) report(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "user_id")
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
	err := h.social.Report(c.Request.Context(), u.ID, id, *req.Reason)
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

// followers answers {"people": [Person]}: their followers, for an accepted
// follower only (404 not_found for anyone else).
func (h *PersonHandler) followers(c *gin.Context) { h.peopleOf(c, true) }

// following answers {"people": [Person]}: whom they follow, for an accepted
// follower only.
func (h *PersonHandler) following(c *gin.Context) { h.peopleOf(c, false) }

func (h *PersonHandler) peopleOf(c *gin.Context, followers bool) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "user_id")
	if !ok {
		return
	}
	list, err := h.social.PeopleOf(c.Request.Context(), u.ID, id, followers)
	if err != nil {
		respondFeedError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"people": personViews(list)})
}
