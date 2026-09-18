package auth

import (
	"context"

	"google.golang.org/api/idtoken"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// GoogleIdentity is the verified identity extracted from a Google ID token
// after its signature, audience, and expiry all check out server-side, and
// its email_verified claim confirms Google itself trusts the address.
type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// GoogleVerifier verifies a raw Google ID token server-side and returns the
// identity it asserts. Implementations must never trust a client-supplied
// identity payload -- only a token whose signature Google itself issued.
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (*GoogleIdentity, error)
}

// googleValidateFunc matches google.golang.org/api/idtoken.Validate's
// signature (confirmed against the vendored v0.298.0 source at
// google.golang.org/api/idtoken/validate.go, since Go subpackages are not
// separately versioned on pkg.go.dev). Defined as a named type so tests can
// substitute a stub that returns crafted claims without ever reaching
// Google's network endpoints.
type googleValidateFunc func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)

// googleVerifier implements GoogleVerifier over an injectable validate seam.
type googleVerifier struct {
	clientIDs []string
	validate  googleValidateFunc
}

// NewGoogleVerifier returns a GoogleVerifier that checks a token's audience
// against every client ID in clientIDs. Plan 01-01 provisions one client ID
// per platform (iOS, Android, web), and a Google ID token's audience varies
// with which platform client initiated the sign-in, so every configured ID
// is tried before the token is rejected.
func NewGoogleVerifier(clientIDs []string) GoogleVerifier {
	return &googleVerifier{
		clientIDs: clientIDs,
		validate:  idtoken.Validate,
	}
}

// Verify tries idtoken.Validate once per configured client ID -- which
// checks the token's signature against Google's public keys, its expiry,
// and the given audience -- until one succeeds. Every failure (no audience
// matched, an invalid signature, an expired token, or an unverified email)
// maps to user.ErrTokenInvalid so the caller cannot accidentally leak
// provider internals.
func (v *googleVerifier) Verify(ctx context.Context, idToken string) (*GoogleIdentity, error) {
	var payload *idtoken.Payload
	matched := false
	for _, clientID := range v.clientIDs {
		p, err := v.validate(ctx, idToken, clientID)
		if err == nil {
			payload = p
			matched = true
			break
		}
	}
	if !matched {
		return nil, user.ErrTokenInvalid
	}

	// The provider's own verification is what lets this phase treat a
	// social account as already verified (RESEARCH.md Open Question 1's
	// adopted branch), so a Google account with an unverified address must
	// not inherit that trust.
	emailVerified := claimBool(payload.Claims["email_verified"])
	if !emailVerified {
		return nil, user.ErrTokenInvalid
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)

	return &GoogleIdentity{
		Subject:       payload.Subject,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
	}, nil
}

// claimBool tolerantly reads a JWT claim that a provider may encode as
// either a JSON boolean or a boolean-valued string ("true"/"false") --
// Apple's email_verified and is_private_email claims are documented to do
// exactly this depending on token type. Any other shape (including an
// absent claim) is treated as false, the safe default for a trust claim.
func claimBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}
