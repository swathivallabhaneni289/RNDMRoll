// Package httpapi: profile.go implements GET/PATCH /me and the avatar
// upload-ticket endpoint. Every handler here resolves its target user
// exclusively through middleware.SubjectFromContext -- there is no route
// parameter and no request or response struct field naming a user ID, so
// there is no route through which one account could address another
// (PATTERNS.md, "Access control on /me").
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// UpdateProfileRequest is the PATCH /me body. Every field is a pointer so a
// nil (omitted) field means "leave unchanged" -- this is what lets each
// onboarding step, and later the standalone edit screen, send only the
// field it actually changed.
type UpdateProfileRequest struct {
	Name      *string `json:"name" binding:"omitempty,min=1,max=50"`
	Username  *string `json:"username" binding:"omitempty,min=3,max=20"`
	Bio       *string `json:"bio" binding:"omitempty,max=160"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url"`
}

// AvatarUploadRequest is the POST /me/avatar/upload-url body.
type AvatarUploadRequest struct {
	ContentType   string `json:"content_type" binding:"required"`
	ContentLength int64  `json:"content_length" binding:"required,gt=0"`
}

// ProfileHandler serves GET/PATCH /me and POST /me/avatar/upload-url.
type ProfileHandler struct {
	users   user.Repository
	avatars storage.AvatarStore
}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler(users user.Repository, avatars storage.AvatarStore) *ProfileHandler {
	return &ProfileHandler{users: users, avatars: avatars}
}

// Register mounts this handler's routes on rg. The caller is responsible
// for attaching RequireAuth -- and, in production, RequireVerified -- to rg
// before calling Register; plan 01-13 owns cmd/api/main.go and applies both
// to the nested authenticated group these handlers are mounted on.
func (h *ProfileHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/me", h.getMe)
	rg.PATCH("/me", h.patchMe)
	rg.POST("/me/avatar/upload-url", h.createAvatarUploadURL)
}

// profileBody builds the ApiUser wire shape (lib/api/types.ts) as a gin.H
// literal rather than a tagged struct, so this file never declares an ID
// struct field tag -- the response carries the caller's own identifier as
// data, never as something a request could set.
func profileBody(u *user.User) gin.H {
	return gin.H{
		"id":                  u.ID.String(),
		"email":               u.Email,
		"name":                u.Name,
		"username":            u.Username,
		"bio":                 u.Bio,
		"avatar_url":          u.AvatarURL,
		"email_verified":      u.EmailVerified,
		"onboarding_complete": u.OnboardingComplete(),
	}
}

func (h *ProfileHandler) getMe(c *gin.Context) {
	subject, err := middleware.SubjectFromContext(c)
	if err != nil {
		RespondError(c, err)
		return
	}

	u, err := h.users.GetByID(c.Request.Context(), subject)
	if err != nil {
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, profileBody(u))
}

func (h *ProfileHandler) patchMe(c *gin.Context) {
	subject, err := middleware.SubjectFromContext(c)
	if err != nil {
		RespondError(c, err)
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	if req.Username != nil {
		if err := user.ValidateUsername(*req.Username); err != nil {
			RespondValidationError(c, err)
			return
		}
	}

	patch := user.ProfilePatch{
		Name:      req.Name,
		Username:  req.Username,
		Bio:       req.Bio,
		AvatarURL: req.AvatarURL,
	}

	updated, err := h.users.UpdateProfile(c.Request.Context(), subject, patch)
	if err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			requested := ""
			if req.Username != nil {
				requested = *req.Username
			}
			respondUsernameTaken(c, h.users, requested)
			return
		}
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, profileBody(updated))
}

// respondUsernameTaken writes the 409 conflict body for a save-time
// username collision -- the authoritative rejection PATTERNS.md describes,
// distinct from the advisory GET /usernames/available check -- attaching
// three fresh alternates. alternates is always a non-nil slice so the JSON
// field marshals as `[]`, never `null`.
func respondUsernameTaken(c *gin.Context, repo user.Repository, requested string) {
	alternates, err := user.SuggestAlternates(c.Request.Context(), repo, requested, 3)
	if err != nil {
		alternates = []string{}
	}
	Respond(c, http.StatusConflict, gin.H{
		"error":       string(CodeUsernameTaken),
		"suggestions": alternates,
	})
}

func (h *ProfileHandler) createAvatarUploadURL(c *gin.Context) {
	subject, err := middleware.SubjectFromContext(c)
	if err != nil {
		RespondError(c, err)
		return
	}

	var req AvatarUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	ticket, err := h.avatars.PresignAvatarUpload(c.Request.Context(), subject, req.ContentType, req.ContentLength)
	if err != nil {
		if errors.Is(err, storage.ErrUnsupportedContentType) || errors.Is(err, storage.ErrContentTooLarge) {
			RespondValidationError(c, err)
			return
		}
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"upload_url": ticket.UploadURL,
		"public_url": ticket.PublicURL,
		"expires_in": ticket.ExpiresIn,
	})
}
