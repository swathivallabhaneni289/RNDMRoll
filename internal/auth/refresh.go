package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// refreshTokenBytes is the amount of entropy read from crypto/rand for each
// opaque refresh token: 256 bits.
const refreshTokenBytes = 32

// RefreshService issues, redeems, and revokes opaque refresh tokens backed
// by a user.RefreshTokenRepository. Only a SHA-256 digest of each token is
// ever persisted -- RESEARCH.md's Security Domain requires refresh tokens
// be stored hashed so a database disclosure does not hand over live
// sessions.
type RefreshService struct {
	repo user.RefreshTokenRepository
	ttl  time.Duration
}

// NewRefreshService constructs a RefreshService backed by repo, issuing
// tokens with the given lifetime.
func NewRefreshService(repo user.RefreshTokenRepository, ttl time.Duration) *RefreshService {
	return &RefreshService{repo: repo, ttl: ttl}
}

// generateRefreshToken reads 32 bytes from crypto/rand, returning both the
// client-facing raw token (base64url-encoded) and the SHA-256 digest of the
// raw bytes that gets persisted.
func generateRefreshToken() (raw string, hash []byte, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)
	return raw, sum[:], nil
}

// hashPresentedToken decodes and hashes a client-presented token the same
// way generateRefreshToken hashed it at issuance, so a lookup by digest
// finds the same row. A token that fails to decode cannot possibly match a
// stored digest, so it is treated as invalid rather than propagated as a
// decode error.
func hashPresentedToken(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, user.ErrTokenInvalid
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

// Issue creates a new refresh token for userID, persisting only its digest.
// The raw token is returned once, for the client to store, and is never
// itself persisted.
func (s *RefreshService) Issue(ctx context.Context, userID uuid.UUID, userAgent *string) (string, error) {
	raw, hash, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	if _, err := s.repo.Insert(ctx, userID, hash, time.Now().Add(s.ttl), userAgent); err != nil {
		return "", err
	}
	return raw, nil
}

// Redeem exchanges a presented refresh token for the owning user ID and a
// freshly rotated replacement token, revoking the presented token and
// inserting the replacement in one transactional call to the repository's
// Rotate. Returning a new token on every redemption is the replay-window
// mitigation RESEARCH.md prescribes: a stolen token is useful only until
// the legitimate client's next refresh, at which point the thief's copy is
// already revoked.
func (s *RefreshService) Redeem(ctx context.Context, token string, userAgent *string) (uuid.UUID, string, error) {
	hash, err := hashPresentedToken(token)
	if err != nil {
		return uuid.UUID{}, "", err
	}

	record, err := s.repo.GetActiveByHash(ctx, hash)
	if err != nil {
		return uuid.UUID{}, "", err
	}

	newRaw, newHash, err := generateRefreshToken()
	if err != nil {
		return uuid.UUID{}, "", err
	}
	if _, err := s.repo.Rotate(ctx, record.ID, newHash, time.Now().Add(s.ttl)); err != nil {
		return uuid.UUID{}, "", err
	}
	return record.UserID, newRaw, nil
}

// Revoke invalidates token, tolerating a miss (an already-invalid or
// malformed token) so that signing out with a stale token still succeeds.
// Any other repository error is propagated.
func (s *RefreshService) Revoke(ctx context.Context, token string) error {
	hash, err := hashPresentedToken(token)
	if err != nil {
		return nil
	}
	if err := s.repo.RevokeByHash(ctx, hash); err != nil && !errors.Is(err, user.ErrTokenInvalid) {
		return err
	}
	return nil
}

// RevokeAll invalidates every refresh token belonging to userID (e.g. "log
// out everywhere").
func (s *RefreshService) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.RevokeAllForUser(ctx, userID)
}
