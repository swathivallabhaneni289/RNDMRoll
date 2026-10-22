package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// accessClaims are an access token's claims: the registered ones, plus sid,
// the id of the refresh token the access token was issued with (its session).
// A token issued before sessions were named carries no sid.
type accessClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid,omitempty"`
}

// IssueAccessToken signs a short-lived HS256 access token whose subject is
// userID.String(), expiring ttl from now. It names no session (no sid claim),
// the shape every token had before sessions were named; the app's sessions get
// their tokens from IssueSessionAccessToken.
func IssueAccessToken(userID uuid.UUID, secret []byte, ttl time.Duration) (string, error) {
	return IssueSessionAccessToken(userID, uuid.Nil, secret, ttl)
}

// IssueSessionAccessToken is IssueAccessToken for one session: sessionID is
// the id of the refresh token issued with this access token, carried as the
// sid claim (none when sessionID is uuid.Nil). What a session leaves on the
// server, such as the phone's push token, is tied to that refresh token, so
// it ends when the session does (log out, expiry, a password change).
func IssueSessionAccessToken(userID, sessionID uuid.UUID, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := accessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}}
	if sessionID != uuid.Nil {
		claims.SessionID = sessionID.String()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseAccessToken verifies tokenString's signature and claims against
// secret and returns its subject. ParseSessionAccessToken says how.
func ParseAccessToken(tokenString string, secret []byte) (uuid.UUID, error) {
	subject, _, err := ParseSessionAccessToken(tokenString, secret)
	return subject, err
}

// ParseSessionAccessToken verifies tokenString's signature and claims against
// secret. It explicitly allow-lists HS256 via jwt.WithValidMethods and
// re-asserts the token's method is *jwt.SigningMethodHMAC inside the
// keyfunc -- the mitigation RESEARCH.md's Security Domain names for
// algorithm-confusion attacks (e.g. alg: none), rather than relying on
// library defaults alone.
//
// It returns the subject and the session (the sid claim), which is uuid.Nil
// for a token issued without one: such a token still works for everything.
// A specifically expired token maps to user.ErrTokenExpired; any other parse
// or validation failure, a sid that is not a UUID among them, maps to
// user.ErrTokenInvalid.
func ParseSessionAccessToken(tokenString string, secret []byte) (subject, sessionID uuid.UUID, err error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.UUID{}, uuid.UUID{}, user.ErrTokenExpired
		}
		return uuid.UUID{}, uuid.UUID{}, user.ErrTokenInvalid
	}
	if !token.Valid {
		return uuid.UUID{}, uuid.UUID{}, user.ErrTokenInvalid
	}

	sub, err := token.Claims.GetSubject()
	if err != nil || sub == "" {
		return uuid.UUID{}, uuid.UUID{}, user.ErrTokenInvalid
	}
	subject, err = uuid.Parse(sub)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, user.ErrTokenInvalid
	}
	if claims.SessionID != "" {
		sessionID, err = uuid.Parse(claims.SessionID)
		if err != nil {
			return uuid.UUID{}, uuid.UUID{}, user.ErrTokenInvalid
		}
	}
	return subject, sessionID, nil
}
