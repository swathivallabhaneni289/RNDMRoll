package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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

	u, isNewUser, err := h.findOrCreateAccount(c.Request.Context(), user.VerifiedViaApple, identity.Subject, identity.Email)
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

	u, isNewUser, err := h.findOrCreateAccount(c.Request.Context(), user.VerifiedViaGoogle, identity.Subject, identity.Email)
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
// sign-in resolves by provider subject; a first sign-in whose verified
// email matches an existing password account links the subject to it
// rather than creating a duplicate; otherwise a new account is created
// already verified. Both providers verify the address before issuing a
// token -- and the Google verifier additionally rejects an unverified
// email claim before this is ever reached -- so creating with
// email_verified true here satisfies D-04 at the API boundary the same way
// RequireVerified reads the column for a password account.
func (h *OAuthHandler) findOrCreateAccount(ctx context.Context, provider user.VerificationSource, subject, email string) (*user.User, bool, error) {
	existing, err := h.users.GetByProviderSubject(ctx, provider, subject)
	switch {
	case err == nil:
		return existing, false, nil
	case !errors.Is(err, user.ErrNotFound):
		return nil, false, err
	}

	byEmail, err := h.users.GetByEmailCI(ctx, email)
	switch {
	case err == nil:
		if linkErr := h.users.LinkProviderSubject(ctx, byEmail.ID, provider, subject); linkErr != nil {
			return nil, false, linkErr
		}
		return byEmail, false, nil
	case !errors.Is(err, user.ErrNotFound):
		return nil, false, err
	}

	created, err := h.users.Create(ctx, email, nil, true, &provider)
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
