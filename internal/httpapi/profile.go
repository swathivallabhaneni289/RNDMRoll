// Package httpapi: profile.go implements GET/PATCH /me and the avatar
// upload-ticket endpoint. Every handler here resolves its target user
// exclusively through middleware.SubjectFromContext -- there is no route
// parameter and no request or response struct field naming a user ID, so
// there is no route through which one account could address another
// (PATTERNS.md, "Access control on /me").
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// UpdateProfileRequest is the PATCH /me body. Every field is a pointer so a
// nil (omitted) field means "leave unchanged" -- this is what lets each
// step, and later the standalone edit screen, send only the field it
// actually changed. There are no binding tags on purpose: patchMe validates
// each field by hand so every rejection names its field.
//
// Birthday is never accepted: it is set once, at sign-up, and cannot change.
// The field exists only so a request that sends one is refused by name.
//
// AccountVisibility is "public" or "private" (exactly, lower case). It is the
// Settings switch and rides on this route; there is no id in the body, so a
// person can only ever change their own.
//
// AvatarKey is the key of a profile photo the caller uploaded with POST
// /me/avatar/upload-url (that ticket's avatar_key). There is no remove: a
// missing key leaves the photo as it is.
type UpdateProfileRequest struct {
	Name              *string `json:"name"`
	Username          *string `json:"username"`
	Bio               *string `json:"bio"`
	AvatarKey         *string `json:"avatar_key"`
	Birthday          *string `json:"birthday"`
	AccountVisibility *string `json:"account_visibility"`
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
	// photoLink signs the profile photo's link (Deps.PhotoLink; NewServer
	// sets it). Nil sends no photo.
	photoLink func(string) string

	// OnBecamePublic, when set, runs after a saved PATCH /me that sets
	// account_visibility to public: it accepts every pending follow request
	// to the caller (no notification). NewServer sets it from the follow
	// handler. Nil does nothing.
	OnBecamePublic func(ctx context.Context, userID uuid.UUID) error
}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler(users user.Repository, avatars storage.AvatarStore) *ProfileHandler {
	return &ProfileHandler{users: users, avatars: avatars}
}

// Register mounts this handler's routes on rg. The caller is responsible
// for attaching RequireAuth -- and, in production, RequireUser -- to rg
// before calling Register; NewServer applies both to the signed-in group
// these handlers are mounted on.
func (h *ProfileHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/me", h.getMe)
	// PATCH /me is limited to 30 a minute per caller, so a loop of renames is
	// no way to grab or probe usernames (the unique index still decides who
	// gets one). The 429 body is the shared {"error":"rate_limited"}.
	rg.PATCH("/me", middleware.RateLimit(middleware.LimitConfig{
		Requests: 30,
		Window:   time.Minute,
		KeyFunc:  middleware.KeyBySubject,
		Clock:    limitClock,
	}), h.patchMe)
	rg.POST("/me/avatar/upload-url", h.createAvatarUploadURL)
}

// profileBody builds the ApiUser wire shape (lib/api/types.ts) as a gin.H
// literal rather than a tagged struct, so this file never declares an ID
// struct field tag -- the response carries the caller's own identifier as
// data, never as something a request could set. It never carries a
// birthday. avatar_url is the profile photo's signed link (photoLinkOf).
func profileBody(u *user.User, link func(string) string) gin.H {
	return gin.H{
		"id":                  u.ID.String(),
		"email":               u.Email,
		"name":                u.Name,
		"username":            u.Username,
		"bio":                 u.Bio,
		"avatar_url":          photoLinkOf(u.AvatarKey, link),
		"email_verified":      u.EmailVerified,
		"onboarding_complete": u.OnboardingComplete(),
		"account_visibility":  u.AccountVisibility,
	}
}

// photoLinkOf is how every answer sends a profile photo (the caller's own and
// other people's): the signed link of key, or nil (JSON null) when there is
// no key, no link func, or the key cannot be signed. A bare key never leaves.
// Who may see the person was decided before this is called.
func photoLinkOf(key *string, link func(string) string) *string {
	if key == nil || *key == "" || link == nil {
		return nil
	}
	l := link(*key)
	if l == "" {
		return nil
	}
	return &l
}

// currentUser returns the caller's account (see callerAccount).
func (h *ProfileHandler) currentUser(c *gin.Context) (*user.User, bool) {
	return callerAccount(c, h.users)
}

// callerAccount returns the signed-in caller's account: the row RequireUser
// already loaded for this request, or a fresh lookup in users when the route
// is mounted without it. A missing account answers 401 token_invalid (the
// account was deleted since the token was minted) and reports ok = false. The
// id comes only from the token's subject, never from the request. The profile
// and account handlers both find the caller this way.
func callerAccount(c *gin.Context, users user.Repository) (*user.User, bool) {
	if u, err := middleware.UserFromContext(c); err == nil {
		return u, true
	}
	subject, err := middleware.SubjectFromContext(c)
	if err != nil {
		RespondError(c, user.ErrTokenInvalid)
		return nil, false
	}
	u, err := users.GetByID(c.Request.Context(), subject)
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
	Respond(c, http.StatusOK, profileBody(u, h.photoLink))
}

// patchMe applies a partial profile update. Order: bind; a birthday is
// refused (it never changes after sign-up); each other field by hand; then
// one UpdateProfile. A value of account_visibility that is not exactly
// "public" or "private" is 400 validation_failed naming that field, and
// nothing is stored. So is an avatar_key that is not exactly a key the avatar
// store could have handed the caller (storage.AvatarKeyOwnedBy): never
// another person's photo, a link or anything else. Once a new photo is saved,
// the file of the one it replaced is removed (removeReplacedAvatar).
func (h *ProfileHandler) patchMe(c *gin.Context) {
	cur, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		// A number, a boolean or an object where the word "public" or
		// "private", or a photo key, belongs is refused by name, like any
		// other bad value of these fields. Every other bind failure stays the
		// one generic answer.
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && (typeErr.Field == "account_visibility" || typeErr.Field == "avatar_key") {
			RespondFieldError(c, typeErr.Field, reasonInvalid)
			return
		}
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
	if req.AvatarKey != nil {
		if !storage.AvatarKeyOwnedBy(*req.AvatarKey, cur.ID) {
			RespondFieldError(c, "avatar_key", reasonInvalid)
			return
		}
		patch.AvatarKey = req.AvatarKey
	}
	if req.AccountVisibility != nil {
		if !user.ValidVisibility(*req.AccountVisibility) {
			RespondFieldError(c, "account_visibility", reasonInvalid)
			return
		}
		patch.AccountVisibility = req.AccountVisibility
	}

	// The photo a new one replaces, read before the save: its file goes once
	// the save is done.
	replaced := ""
	if cur.AvatarKey != nil {
		replaced = *cur.AvatarKey
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

	if h.OnBecamePublic != nil && req.AccountVisibility != nil && *req.AccountVisibility == user.VisibilityPublic {
		// The switch is saved; a failure here leaves the requests pending
		// (the person can still accept them) and is only logged.
		if err := h.OnBecamePublic(c.Request.Context(), updated.ID); err != nil {
			log.Printf("httpapi: accept pending follow requests: %v", err)
		}
	}
	if req.AvatarKey != nil {
		h.removeReplacedAvatar(c.Request.Context(), cur.ID, replaced, updated.AvatarKey)
	}

	Respond(c, http.StatusOK, profileBody(updated, h.photoLink))
}

// removeReplacedAvatar deletes the file of the profile photo that a saved
// PATCH /me replaced (developer, 2026-11-09: no leftover files). It runs only
// after the save, and only for an old key that is a profile photo key in the
// caller's own folder (storage.AvatarKeyOwnedBy) and is not the photo now
// saved: the same photo sent again is kept, and a stored value of any other
// shape never reaches storage. It is best effort: a failure is logged without
// the key, which names the caller's folder, and the request never fails
// because of it. It has one short deadline, and a caller that hangs up does
// not cancel it.
func (h *ProfileHandler) removeReplacedAvatar(ctx context.Context, owner uuid.UUID, old string, saved *string) {
	if h.avatars == nil || old == "" || saved == nil || old == *saved || !storage.AvatarKeyOwnedBy(old, owner) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fileRemovalTimeout)
	defer cancel()
	if err := h.avatars.DeleteAvatar(ctx, owner, old); err != nil {
		log.Printf("httpapi: could not remove a replaced profile photo file: %s", withoutKey(err, old))
	}
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

	// The key goes back through PATCH /me (avatar_key) once the upload is
	// done. No link is sent: the photo shows from the link GET /me signs.
	Respond(c, http.StatusOK, gin.H{
		"upload_url": ticket.UploadURL,
		"avatar_key": ticket.Key,
		"expires_in": ticket.ExpiresIn,
	})
}
