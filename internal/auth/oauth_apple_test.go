package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

const testAppleAudience = "com.rndmroll.app"

// appleTestFixture serves a locally generated RSA key as a JWKS from an
// httptest.Server, so Verify's signature check runs against a real JWKS
// response shape without ever reaching Apple's real endpoint.
type appleTestFixture struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string
}

func newAppleJWKSFixture(t *testing.T) *appleTestFixture {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	const kid = "test-key-1"

	jwk, err := jwkset.NewJWKFromKey(key.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{
			KID: kid,
			ALG: jwkset.AlgRS256,
			USE: jwkset.UseSig,
		},
	})
	if err != nil {
		t.Fatalf("build JWK: %v", err)
	}

	jwks := jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{jwk.Marshal()}}
	body, err := json.Marshal(jwks)
	if err != nil {
		t.Fatalf("marshal JWKS: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	return &appleTestFixture{server: server, key: key, kid: kid}
}

func (f *appleTestFixture) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = f.kid
	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func newAppleTestVerifier(t *testing.T, fixture *appleTestFixture, audiences []string) AppleVerifier {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	v, err := newAppleVerifier(ctx, audiences, fixture.server.URL)
	if err != nil {
		t.Fatalf("newAppleVerifier: %v", err)
	}
	return v
}

func validAppleClaims(sub string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":              appleIssuer,
		"aud":              testAppleAudience,
		"sub":              sub,
		"iat":              now.Unix(),
		"exp":              now.Add(time.Hour).Unix(),
		"email":            "person@privaterelay.appleid.com",
		"email_verified":   "true",
		"is_private_email": "true",
	}
}

func TestApple_ValidTokenReturnsIdentity(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	token := fixture.sign(t, validAppleClaims("apple-subject-1"))

	identity, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if identity.Subject != "apple-subject-1" {
		t.Errorf("Subject = %q, want apple-subject-1", identity.Subject)
	}
	if identity.Email != "person@privaterelay.appleid.com" {
		t.Errorf("Email = %q, want person@privaterelay.appleid.com", identity.Email)
	}
	if !identity.EmailVerified {
		t.Error("EmailVerified = false, want true")
	}
	if !identity.IsPrivateRelay {
		t.Error("IsPrivateRelay = false, want true")
	}
}

func TestApple_WrongIssuerIsRejected(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	claims := validAppleClaims("apple-subject-2")
	claims["iss"] = "https://not-apple.example.com"
	token := fixture.sign(t, claims)

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestApple_WrongAudienceIsRejected(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	claims := validAppleClaims("apple-subject-3")
	claims["aud"] = "com.someone-else.app"
	token := fixture.sign(t, claims)

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestApple_KeyAbsentFromJWKSIsRejected(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, validAppleClaims("apple-subject-4"))
	token.Header["kid"] = "unknown-key-id"
	signed, err := token.SignedString(otherKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = v.Verify(context.Background(), signed)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestApple_ExpiredTokenIsRejected(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	claims := validAppleClaims("apple-subject-5")
	claims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
	claims["exp"] = time.Now().Add(-time.Hour).Unix()
	token := fixture.sign(t, claims)

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestApple_AlgNoneIsRejected(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	token := handCraftedAppleAlgNoneToken(t, "apple-subject-6")

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestApple_EmailVerifiedAndPrivateRelayToleratesBooleanEncoding(t *testing.T) {
	fixture := newAppleJWKSFixture(t)
	v := newAppleTestVerifier(t, fixture, []string{testAppleAudience})

	claims := validAppleClaims("apple-subject-7")
	claims["email_verified"] = true
	claims["is_private_email"] = false
	token := fixture.sign(t, claims)

	identity, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if !identity.EmailVerified {
		t.Error("EmailVerified = false, want true (boolean encoding)")
	}
	if identity.IsPrivateRelay {
		t.Error("IsPrivateRelay = true, want false (boolean encoding)")
	}
}

// handCraftedAppleAlgNoneToken builds a JWT by hand with alg:none and an
// empty signature, the same technique jwt_test.go uses for the HS256 case,
// since no library will sign a token like this for us.
func handCraftedAppleAlgNoneToken(t *testing.T, subject string) string {
	t.Helper()

	header := map[string]string{"alg": "none", "typ": "JWT"}
	claims := map[string]any{
		"iss": appleIssuer,
		"aud": testAppleAudience,
		"sub": subject,
		"exp": time.Now().Add(time.Hour).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	headerSeg := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsSeg := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return headerSeg + "." + claimsSeg + "."
}
