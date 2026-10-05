package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// AppleSignInRequest is the only shape POST /auth/oauth/apple accepts.
// There is deliberately no email or provider field: identity comes
// exclusively from the verified token's subject claim, so a client that
// sends one gets it silently discarded by Gin's binding rather than
// honored. FullName is untrusted display text used only to prefill the
// name step -- Apple returns it (and email) only on a user's very first
// authorization for a given Apple ID and bundle pair.
type AppleSignInRequest struct {
	IdentityToken string  `json:"identity_token" binding:"required"`
	FullName      *string `json:"full_name"`
}

// GoogleSignInRequest is the only shape POST /auth/oauth/google accepts.
type GoogleSignInRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// OAuthHandler mounts the Apple and Google sign-in endpoints. Both verify a
// raw provider identity token server-side before creating or linking an
// account -- neither endpoint ever trusts a client-asserted identity.
type OAuthHandler struct {
	users          user.Repository
	appleVerifier  auth.AppleVerifier
	googleVerifier auth.GoogleVerifier
	refreshTokens  *auth.RefreshService
	jwtSecret      []byte
	accessTokenTTL time.Duration
}

// NewOAuthHandler constructs an OAuthHandler.
func NewOAuthHandler(
	users user.Repository,
	appleVerifier auth.AppleVerifier,
	googleVerifier auth.GoogleVerifier,
	refreshTokens *auth.RefreshService,
	jwtSecret []byte,
	accessTokenTTL time.Duration,
) *OAuthHandler {
	return &OAuthHandler{
		users:          users,
		appleVerifier:  appleVerifier,
		googleVerifier: googleVerifier,
		refreshTokens:  refreshTokens,
		jwtSecret:      jwtSecret,
		accessTokenTTL: accessTokenTTL,
	}
}

// Register mounts the oauth routes onto rg. Both are rate limited per IP:
// RESEARCH.md's denial-of-service mitigation for provider-verification
// traffic, since every request here drives an outbound verification call.
func (h *OAuthHandler) Register(rg *gin.RouterGroup) {
	limit := middleware.RateLimit(middleware.LimitConfig{Requests: 10, Window: time.Minute, KeyFunc: middleware.KeyByIP})
	rg.POST("/auth/oauth/apple", limit, h.SignInWithApple)
	rg.POST("/auth/oauth/google", limit, h.SignInWithGoogle)
}

// SignInWithApple verifies the presented Apple identity token, creates or
// links an account from the verified subject, persists the supplied name
// on a first authorization without ever clearing a stored name on a later
// one (Apple returns full_name as null after the first authorization), and
// responds with a fresh session.
func (h *OAuthHandler) SignInWithApple(c *gin.Context) {
	var req AppleSignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	identity, err := h.appleVerifier.Verify(c.Request.Context(), req.IdentityToken)
	if err != nil {
		RespondError(c, err)
		return
	}

	u, isNewUser, err := h.findOrCreateAccount(c.Request.Context(), user.VerifiedViaApple, identity.Subject, identity.Email, identity.EmailVerified)
	if err != nil {
		RespondError(c, err)
		return
	}

	fullName := ""
	if req.FullName != nil {
		fullName = *req.FullName
	}
	u, err = h.persistNameIfUnset(c.Request.Context(), u, fullName)
	if err != nil {
		RespondError(c, err)
		return
	}

	h.respondWithSession(c, u, isNewUser)
}

// SignInWithGoogle verifies the presented Google ID token, creates or links
// an account from the verified subject, and responds with a fresh session.
func (h *OAuthHandler) SignInWithGoogle(c *gin.Context) {
	var req GoogleSignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	identity, err := h.googleVerifier.Verify(c.Request.Context(), req.IDToken)
	if err != nil {
		RespondError(c, err)
		return
	}

	u, isNewUser, err := h.findOrCreateAccount(c.Request.Context(), user.VerifiedViaGoogle, identity.Subject, identity.Email, identity.EmailVerified)
	if err != nil {
		RespondError(c, err)
		return
	}

	u, err = h.persistNameIfUnset(c.Request.Context(), u, identity.Name)
	if err != nil {
		RespondError(c, err)
		return
	}

	h.respondWithSession(c, u, isNewUser)
}

// findOrCreateAccount implements the shared social sign-in flow: a repeat
// sign-in resolves by provider subject; a first sign-in whose email matches
// an existing account links the subject to it rather than creating a
// duplicate; otherwise a new account is created. emailVerified is the
// provider's own email_verified claim. Only a verified claim may link to an
// existing account: an unverified one proves nothing about who owns the
// address, so a match is refused with ErrEmailTaken instead.
//
// Linking to an account that is still unverified is the provider proving
// ownership of the address, so the account is claimed: marked verified, its
// password credential discarded and its refresh tokens revoked, because
// whoever pre-registered the address with a password never proved they own
// it and must not keep access to the account.
//
// The claim is several writes with no transaction, so its order decides what
// a failure leaves behind, and every step is idempotent so a plain retry
// converges: (a) revoke refresh tokens, (b) claim, (c) link the subject.
//   - (a) fails: nothing changed; the retry matches by email again.
//   - (b) fails: tokens are gone but the account is still unverified and
//     unlinked; the retry matches by email again and repeats (a) and (b).
//   - (c) fails: the account is already claimed; the retry sees a verified
//     account and only links.
//
// Between (a) and (b) the old password still exists, but it cannot mint a
// session: Login refuses an unverified account and Signup issues none, so (a)
// is defense in depth for tokens issued before the claim, not a gate. An access
// token issued earlier stays valid until it expires, which is short.
//
// Linking first would be wrong: the retry would then resolve by subject and
// return early, leaving the password and the old tokens alive. The subject
// branch also runs (a) and (b) while the account is unverified, which heals
// an account left linked but unclaimed by an earlier attempt that failed.
func (h *OAuthHandler) findOrCreateAccount(ctx context.Context, provider user.VerificationSource, subject, email string, emailVerified bool) (*user.User, bool, error) {
	existing, err := h.users.GetByProviderSubject(ctx, provider, subject)
	switch {
	case err == nil:
		// Only claim for the address the provider vouches for: the account
		// is already bound by subject, but its stored email must be the one
		// the provider just verified.
		if !existing.EmailVerified && emailVerified && strings.EqualFold(existing.Email, email) {
			if err := h.claimUnverifiedAccount(ctx, existing.ID, provider); err != nil {
				return nil, false, err
			}
			healed, err := h.users.GetByID(ctx, existing.ID)
			if err != nil {
				return nil, false, err
			}
			return healed, false, nil
		}
		return existing, false, nil
	case !errors.Is(err, user.ErrNotFound):
		return nil, false, err
	}

	// Apple only sends the email on a first authorization and may omit it,
	// so an empty one is only fatal here, where an account would be created.
	if email == "" {
		return nil, false, user.ErrProviderEmailMissing
	}

	byEmail, err := h.users.GetByEmailCI(ctx, email)
	switch {
	case err == nil:
		if !emailVerified {
			return nil, false, user.ErrEmailTaken
		}
		if byEmail.EmailVerified {
			if err := h.users.LinkProviderSubject(ctx, byEmail.ID, provider, subject); err != nil {
				return nil, false, err
			}
			return byEmail, false, nil
		}
		if err := h.claimUnverifiedAccount(ctx, byEmail.ID, provider); err != nil {
			return nil, false, err
		}
		if err := h.users.LinkProviderSubject(ctx, byEmail.ID, provider, subject); err != nil {
			return nil, false, err
		}
		claimed, err := h.users.GetByID(ctx, byEmail.ID)
		if err != nil {
			return nil, false, err
		}
		return claimed, false, nil
	case !errors.Is(err, user.ErrNotFound):
		return nil, false, err
	}

	var via *user.VerificationSource
	if emailVerified {
		via = &provider
	}
	created, err := h.users.Create(ctx, email, nil, emailVerified, via)
	if err != nil {
		return nil, false, err
	}
	// Create records only how the email was verified, not the provider
	// subject itself -- that linkage is what lets a later sign-in resolve
	// straight through the GetByProviderSubject branch above instead of
	// falling back to an email match every time.
	if err := h.users.LinkProviderSubject(ctx, created.ID, provider, subject); err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// claimUnverifiedAccount cuts off the pre-registrant: refresh tokens are
// revoked first, then the password is discarded and the email marked
// verified. Both steps are idempotent (revoking nothing is fine, and the
// claim is a guarded UPDATE that does nothing on a verified account), so
// it is safe to run again after a failure or alongside a concurrent claim.
func (h *OAuthHandler) claimUnverifiedAccount(ctx context.Context, id uuid.UUID, provider user.VerificationSource) error {
	if err := h.refreshTokens.RevokeAll(ctx, id); err != nil {
		return err
	}
	_, err := h.users.ClaimUnverifiedEmail(ctx, id, provider)
	return err
}

// persistNameIfUnset writes name to the account only when it is non-empty
// and the account has no name yet. This is the guard that keeps Apple's
// one-shot name delivery from ever clobbering a name already captured: on
// a second authorization req.FullName is nil (name == ""), so this returns
// u unchanged rather than overwriting the stored name with a blank one.
func (h *OAuthHandler) persistNameIfUnset(ctx context.Context, u *user.User, name string) (*user.User, error) {
	if name == "" {
		return u, nil
	}
	if u.Name != nil && *u.Name != "" {
		return u, nil
	}
	return h.users.UpdateProfile(ctx, u.ID, user.ProfilePatch{Name: &name})
}

// respondWithSession issues a fresh access/refresh token pair for u and
// writes the 200 response in the client's AuthResult shape (lib/api/types.ts):
// access_token, refresh_token, expires_in, user, and is_new_user.
func (h *OAuthHandler) respondWithSession(c *gin.Context, u *user.User, isNewUser bool) {
	accessToken, err := auth.IssueAccessToken(u.ID, h.jwtSecret, h.accessTokenTTL)
	if err != nil {
		RespondError(c, err)
		return
	}

	userAgent := c.Request.UserAgent()
	refreshToken, err := h.refreshTokens.Issue(c.Request.Context(), u.ID, &userAgent)
	if err != nil {
		RespondError(c, err)
		return
	}

	Respond(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessTokenTTL.Seconds()),
		"user":          oauthAPIUser(u),
		"is_new_user":   isNewUser,
	})
}

// oauthAPIUser builds the ApiUser wire shape (lib/api/types.ts).
func oauthAPIUser(u *user.User) gin.H {
	return gin.H{
		"id":                  u.ID,
		"email":               u.Email,
		"name":                u.Name,
		"username":            u.Username,
		"bio":                 u.Bio,
		"avatar_url":          u.AvatarURL,
		"email_verified":      u.EmailVerified,
		"onboarding_complete": u.OnboardingComplete(),
	}
}
