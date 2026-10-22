package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestJWT_SessionTokenCarriesItsSession(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID, sessionID := uuid.New(), uuid.New()

	token, err := IssueSessionAccessToken(userID, sessionID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueSessionAccessToken returned error: %v", err)
	}
	subject, session, err := ParseSessionAccessToken(token, secret)
	if err != nil || subject != userID || session != sessionID {
		t.Fatalf("parsed subject %s session %s (err %v), want %s and %s", subject, session, err, userID, sessionID)
	}
	// The older parser still reads the subject.
	if subject, err := ParseAccessToken(token, secret); err != nil || subject != userID {
		t.Fatalf("ParseAccessToken: %s, %v", subject, err)
	}
}

// A token from before sessions were named has no sid and still works, with
// no session.
func TestJWT_TokenWithoutASessionStillParses(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	userID := uuid.New()

	token, err := IssueAccessToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}
	subject, session, err := ParseSessionAccessToken(token, secret)
	if err != nil || subject != userID || session != uuid.Nil {
		t.Fatalf("parsed subject %s session %s (err %v), want %s and no session", subject, session, err, userID)
	}
	payload := decodeSegment(t, token, 1)
	if _, ok := payload["sid"]; ok {
		t.Fatalf("a token without a session carries sid: %v", payload)
	}
}

// A signed token whose sid is not a UUID is invalid, not a token without a
// session.
func TestJWT_ParseRejectsASessionThatIsNotAUUID(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes!!")
	claims := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		SessionID: "not-a-uuid",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, _, err := ParseSessionAccessToken(token, secret); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

// decodeSegment decodes one dot-separated segment of a JWT as JSON.
func decodeSegment(t *testing.T, token string, i int) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[i])
	if err != nil {
		t.Fatalf("decode segment %d: %v", i, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("segment %d is not JSON: %v", i, err)
	}
	return out
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
