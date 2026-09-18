package auth

import (
	"context"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

const (
	// appleJWKSURL is Apple's public signing-key endpoint.
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
	// appleIssuer is the only issuer Apple ever puts in an identity token.
	appleIssuer = "https://appleid.apple.com"
)

// AppleIdentity is the verified identity extracted from an Apple identity
// token after its signature, issuer, audience, and expiry all check out
// server-side.
type AppleIdentity struct {
	Subject        string
	Email          string
	EmailVerified  bool
	IsPrivateRelay bool
}

// AppleVerifier verifies a raw Apple identity token server-side and returns
// the identity it asserts. Implementations must never trust a
// client-supplied identity payload -- only a token whose signature Apple
// itself issued.
type AppleVerifier interface {
	Verify(ctx context.Context, identityToken string) (*AppleIdentity, error)
}

type appleVerifier struct {
	audiences []string
	keys      keyfunc.Keyfunc
}

// NewAppleVerifier fetches and caches Apple's public signing keys from
// Apple's JWKS endpoint via MicahParks/keyfunc/v3, which handles the
// background refresh and key-rotation logic RESEARCH.md's Don't Hand-Roll
// table warns against writing by hand.
func NewAppleVerifier(audiences []string) (AppleVerifier, error) {
	return newAppleVerifier(context.Background(), audiences, appleJWKSURL)
}

// newAppleVerifier is the test seam: it takes the JWKS URL and a
// cancelable context directly, so tests can point it at a local
// httptest.Server instead of Apple's real endpoint and stop the
// background refresh goroutine on cleanup.
func newAppleVerifier(ctx context.Context, audiences []string, jwksURL string) (AppleVerifier, error) {
	keys, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("auth: fetch apple jwks: %w", err)
	}
	return &appleVerifier{audiences: audiences, keys: keys}, nil
}

// keyFunc wraps keyfunc's key lookup with an explicit signing-method type
// assertion, mirroring jwt.go's HS256 allow-listing pattern from plan
// 01-06 so jwt.WithValidMethods is never the sole defense against
// algorithm-confusion attacks.
func (v *appleVerifier) keyFunc(ctx context.Context) jwt.Keyfunc {
	inner := v.keys.KeyfuncCtx(ctx)
	return func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return inner(t)
	}
}

// Verify parses identityToken against Apple's cached public keys, pinning
// the signing method to RS256, the issuer to Apple's own, and the audience
// to one of the configured bundle/service identifiers. A validly signed
// Apple token minted for a different application must still be rejected,
// so iss and aud are checked explicitly rather than trusting signature
// validity alone.
func (v *appleVerifier) Verify(ctx context.Context, identityToken string) (*AppleIdentity, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(identityToken, claims, v.keyFunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.audiences...),
	)
	if err != nil || token == nil || !token.Valid {
		return nil, user.ErrTokenInvalid
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, user.ErrTokenInvalid
	}
	email, _ := claims["email"].(string)

	return &AppleIdentity{
		Subject: sub,
		Email:   email,
		// Apple sends email_verified and is_private_email as either a JSON
		// boolean or a boolean-valued string depending on token type;
		// claimBool (internal/auth/oauth_google.go) reads either shape.
		// IsPrivateRelay is recorded but never acted on in this phase: no
		// mail is ever sent to a social account, so the sender-domain
		// registration a privaterelay.appleid.com address would require is
		// never triggered.
		EmailVerified:  claimBool(claims["email_verified"]),
		IsPrivateRelay: claimBool(claims["is_private_email"]),
	}, nil
}
