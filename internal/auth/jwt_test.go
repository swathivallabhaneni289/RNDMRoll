package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

func TestJWT_IssueThenParseReturnsSubject(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID := uuid.New()

	token, err := IssueAccessToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	subject, err := ParseAccessToken(token, secret)
	if err != nil {
		t.Fatalf("ParseAccessToken rejected a freshly issued token: %v", err)
	}
	if subject != userID {
		t.Fatalf("expected subject %s, got %s", userID, subject)
	}
}

func TestJWT_ParseRejectsAlgNone(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	token := handCraftedAlgNoneToken(t, uuid.New().String())

	if _, err := ParseAccessToken(token, secret); err == nil {
		t.Fatal("expected ParseAccessToken to reject an alg:none token")
	}
}

func TestJWT_ParseRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	otherSecret := []byte("a-totally-different-secret-here")

	token, err := IssueAccessToken(uuid.New(), otherSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	if _, err := ParseAccessToken(token, secret); err == nil {
		t.Fatal("expected ParseAccessToken to reject a token signed with a different secret")
	}
}

func TestJWT_ParseRejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")

	token, err := IssueAccessToken(uuid.New(), secret, -1*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	_, err = ParseAccessToken(token, secret)
	if !errors.Is(err, user.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

// handCraftedAlgNoneToken builds a JWT by hand with alg:none and an empty
// signature. golang-jwt will not sign a token like this for us, so the
// header and payload segments are base64url-encoded and joined manually,
// per RESEARCH.md's algorithm-confusion mitigation this test proves.
func handCraftedAlgNoneToken(t *testing.T, subject string) string {
	t.Helper()

	header := map[string]string{"alg": "none", "typ": "JWT"}
	claims := map[string]any{
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
