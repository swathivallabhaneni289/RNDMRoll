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
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// maxAvatarURLLength bounds a saved avatar_url well above any real storage
// URL.
const maxAvatarURLLength = 2048

// UpdateProfileRequest is the PATCH /me body. Every field is a pointer so a
// nil (omitted) field means "leave unchanged" -- this is what lets each
// step, and later the standalone edit screen, send only the field it
// actually changed. There are no binding tags on purpose: patchMe validates
// each field by hand so every rejection names its field.
//
// Birthday is never accepted: it is set once, at sign-up, and cannot change.
// The field exists only so a request that sends one is refused by name.
type UpdateProfileRequest struct {
	Name      *string `json:"name"`
	Username  *string `json:"username"`
	Bio       *string `json:"bio"`
	AvatarURL *string `json:"avatar_url"`
	Birthday  *string `json:"birthday"`
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
	// publicBaseURL is the storage public base URL (S3PublicBaseURL),
	// exactly as the avatar store uses it to build a PublicURL. A saved
	// avatar_url must start with <publicBaseURL>/avatars/<caller id>/.
	publicBaseURL string
}

// NewProfileHandler constructs a ProfileHandler. publicBaseURL is the
// storage public base URL the avatar store hands out (config S3PublicBaseURL).
func NewProfileHandler(users user.Repository, avatars storage.AvatarStore, publicBaseURL string) *ProfileHandler {
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")
	return &ProfileHandler{users: users, avatars: avatars, publicBaseURL: publicBaseURL}
}

// Register mounts this handler's routes on rg. The caller is responsible
// for attaching RequireAuth -- and, in production, RequireUser -- to rg
// before calling Register; NewServer applies both to the signed-in group
// these handlers are mounted on.
func (h *ProfileHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/me", h.getMe)
	rg.PATCH("/me", h.patchMe)
	rg.POST("/me/avatar/upload-url", h.createAvatarUploadURL)
}

// profileBody builds the ApiUser wire shape (lib/api/types.ts) as a gin.H
// literal rather than a tagged struct, so this file never declares an ID
// struct field tag -- the response carries the caller's own identifier as
// data, never as something a request could set. It never carries a
// birthday.
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

// currentUser returns the caller's account: the row RequireUser already
// loaded for this request, or a fresh lookup when the route is mounted
// without it. A missing account answers 401 token_invalid (the account was
// deleted since the token was minted) and reports ok = false.
func (h *ProfileHandler) currentUser(c *gin.Context) (*user.User, bool) {
	if u, err := middleware.UserFromContext(c); err == nil {
		return u, true
	}
	subject, err := middleware.SubjectFromContext(c)
	if err != nil {
		RespondError(c, user.ErrTokenInvalid)
		return nil, false
	}
	u, err := h.users.GetByID(c.Request.Context(), subject)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			RespondError(c, user.ErrTokenInvalid)
			return nil, false
		}
		RespondError(c, err)
		return nil, false
	}
	return u, true
}

func (h *ProfileHandler) getMe(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		return
	}
	Respond(c, http.StatusOK, profileBody(u))
}

// avatarURLAllowed reports whether url lives in the caller's own avatar
// folder. A client may only save a URL the avatar store could have handed
// it, never another person's picture or an external image.
func (h *ProfileHandler) avatarURLAllowed(url string, id string) bool {
	if len(url) > maxAvatarURLLength {
		return false
	}
	prefix := h.publicBaseURL + "/avatars/" + id + "/"
	if !strings.HasPrefix(url, prefix) {
		return false
	}
	rest := url[len(prefix):]
	return avatarFileName.MatchString(rest) && !strings.Contains(rest, "..")
}

// avatarFileName is the plain file name the avatar store generates
// (random hex plus extension). Anything else, including percent escapes,
// slashes, backslashes, queries and fragments, is refused.
var avatarFileName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// patchMe applies a partial profile update. Order: bind; a birthday is
// refused (it never changes after sign-up); each other field by hand; then
// one UpdateProfile.
func (h *ProfileHandler) patchMe(c *gin.Context) {
	cur, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		RespondValidationError(c, err)
		return
	}

	if req.Birthday != nil {
		// The birthday is set once, at sign-up, and cannot be changed.
		RespondFieldError(c, "birthday", reasonNotAllow)
		return
	}

	patch := user.ProfilePatch{}

	if req.Name != nil {
		name, reason := cleanName(*req.Name)
		if reason != "" {
			RespondFieldError(c, "name", reason)
			return
		}
		patch.Name = &name
	}
	if req.Username != nil {
		if err := user.ValidateUsername(*req.Username); err != nil {
			RespondFieldError(c, "username", reasonInvalid)
			return
		}
		patch.Username = req.Username
	}
	if req.Bio != nil {
		if utf8.RuneCountInString(*req.Bio) > maxBioLength {
			RespondFieldError(c, "bio", reasonTooLong)
			return
		}
		if hasControlChar(*req.Bio, true) {
			RespondFieldError(c, "bio", reasonInvalid)
			return
		}
		patch.Bio = req.Bio
	}
	if req.AvatarURL != nil {
		if !h.avatarURLAllowed(*req.AvatarURL, cur.ID.String()) {
			RespondFieldError(c, "avatar_url", reasonInvalid)
			return
		}
		patch.AvatarURL = req.AvatarURL
	}

	updated, err := h.users.UpdateProfile(c.Request.Context(), cur.ID, patch)
	if err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			requested := ""
			if req.Username != nil {
				requested = *req.Username
			}
			respondUsernameTaken(c, h.users, requested)
			return
		}
		if errors.Is(err, user.ErrNotFound) {
			RespondError(c, user.ErrTokenInvalid)
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
	cur, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req AvatarUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	ticket, err := h.avatars.PresignAvatarUpload(c.Request.Context(), cur.ID, req.ContentType, req.ContentLength)
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
