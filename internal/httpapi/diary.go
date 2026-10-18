// Package httpapi: diary.go serves the two read routes of Phase 4: GET
// /v1/diary (a page of the caller's rows with the category chips) and GET
// /v1/me/summary (the Profile numbers, the tastes and the last three posts).
// The person comes only from the token. The query values are checked by hand
// before any read, and photo links are built here, never below.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/diary"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// DiaryHandler serves GET /diary and GET /me/summary.
type DiaryHandler struct {
	svc    *diary.Service
	photos storage.EntryPhotoStore
}

// NewDiaryHandler constructs a DiaryHandler. photos builds the links.
func NewDiaryHandler(svc *diary.Service, photos storage.EntryPhotoStore) *DiaryHandler {
	return &DiaryHandler{svc: svc, photos: photos}
}

// Register mounts the routes on rg, which carries RequireAuth and
// RequireUser. Both are limited to 60 a minute per person.
func (h *DiaryHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/diary", perSubject(60), h.page)
	rg.GET("/me/summary", perSubject(60), h.summary)
}

type diaryRowView struct {
	Kind      string            `json:"kind"`
	Entry     *entryView        `json:"entry,omitempty"`
	Social    *postSocialView   `json:"social,omitempty"`
	LocalDate string            `json:"local_date,omitempty"`
	Category  *spin.CategoryRef `json:"category,omitempty"`
}

// postSocialView is the comments on an entry row's post, sent on every entry
// row (0 and an empty list when none): the comment count and the newest
// (diary.LatestCount).
type postSocialView struct {
	CommentCount   int                 `json:"comment_count"`
	LatestComments []latestCommentView `json:"latest_comments"`
}

type latestCommentView struct {
	Author latestAuthorView `json:"author"`
	Text   string           `json:"text"`
}

type latestAuthorView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Username string  `json:"username"`
	PhotoURL *string `json:"photo_url"`
}

func newPostSocialView(s *diary.PostSocial) *postSocialView {
	v := &postSocialView{LatestComments: []latestCommentView{}}
	if s == nil {
		return v
	}
	v.CommentCount = s.Comments
	for _, c := range s.Latest {
		v.LatestComments = append(v.LatestComments, latestCommentView{
			Author: latestAuthorView{ID: c.AuthorID, Name: c.Name, Username: c.Username, PhotoURL: c.PhotoURL},
			Text:   c.Text,
		})
	}
	return v
}

type tasteView struct {
	Category diary.Chip `json:"category"`
	Average  float64    `json:"average"`
	Count    int        `json:"count"`
}

func (h *DiaryHandler) page(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var categoryID *string
	if raw, present := c.GetQuery("category_id"); present {
		id, ok := canonicalID(raw)
		if !ok {
			RespondFieldError(c, "category_id", reasonInvalid)
			return
		}
		categoryID = &id
	}
	cursor := c.Query("cursor")
	if _, err := diary.DecodeCursor(cursor); err != nil {
		RespondFieldError(c, "cursor", reasonInvalid)
		return
	}
	limit, err := diary.ParseLimit(c.Query("limit"))
	if err != nil {
		RespondFieldError(c, "limit", reasonInvalid)
		return
	}

	page, err := h.svc.Page(c.Request.Context(), u.ID.String(), diary.PageParams{
		CategoryID: categoryID, Cursor: cursor, Limit: limit,
	})
	if err != nil {
		if errors.Is(err, diary.ErrBadCursor) {
			RespondFieldError(c, "cursor", reasonInvalid)
			return
		}
		RespondError(c, err)
		return
	}

	items := make([]diaryRowView, 0, len(page.Rows))
	for _, r := range page.Rows {
		switch r.Kind {
		case diary.KindEntry:
			v := newEntryView(*r.Entry, h.photos.PublicURL)
			items = append(items, diaryRowView{Kind: "entry", Entry: &v, Social: newPostSocialView(r.Social)})
		default:
			cat := r.Category
			items = append(items, diaryRowView{Kind: "skipped", LocalDate: r.LocalDate, Category: &cat})
		}
	}
	var next any
	if page.NextCursor != "" {
		next = page.NextCursor
	}
	chips := page.Chips
	if chips == nil {
		chips = []diary.Chip{}
	}
	Respond(c, http.StatusOK, gin.H{"items": items, "next_cursor": next, "categories": chips})
}

func (h *DiaryHandler) summary(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	s, err := h.svc.Summary(c.Request.Context(), u.ID.String())
	if err != nil {
		RespondError(c, err)
		return
	}
	tastes := make([]tasteView, 0, len(s.Tastes))
	for _, t := range s.Tastes {
		tastes = append(tastes, tasteView{Category: t.Category, Average: float64(t.Tenths) / 10, Count: t.Count})
	}
	recent := make([]entryView, 0, len(s.Recent))
	for _, e := range s.Recent {
		recent = append(recent, newEntryView(e, h.photos.PublicURL))
	}
	Respond(c, http.StatusOK, gin.H{
		"spins":       s.Spins,
		"week_streak": s.WeekStreak,
		"followers":   s.Followers,
		"following":   s.Following,
		"tastes":      tastes,
		"recent":      recent,
	})
}
