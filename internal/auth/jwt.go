package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// IssueAccessToken signs a short-lived HS256 access token whose subject is
// userID.String(), expiring ttl from now.
func IssueAccessToken(userID uuid.UUID, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseAccessToken verifies tokenString's signature and claims against
// secret. It explicitly allow-lists HS256 via jwt.WithValidMethods and
// re-asserts the token's method is *jwt.SigningMethodHMAC inside the
// keyfunc -- the mitigation RESEARCH.md's Security Domain names for
// algorithm-confusion attacks (e.g. alg: none), rather than relying on
// library defaults alone.
//
// A specifically expired token maps to user.ErrTokenExpired; any other
// parse or validation failure maps to user.ErrTokenInvalid.
func ParseAccessToken(tokenString string, secret []byte) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.UUID{}, user.ErrTokenExpired
		}
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	if !token.Valid {
		return uuid.UUID{}, user.ErrTokenInvalid
	}

	subject, err := token.Claims.GetSubject()
	if err != nil || subject == "" {
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	id, err := uuid.Parse(subject)
	if err != nil {
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	return id, nil
}
