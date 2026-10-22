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
// itself persisted. id is the stored token's id: the session the access
// tokens issued with it name (IssueSessionAccessToken).
func (s *RefreshService) Issue(ctx context.Context, userID uuid.UUID, userAgent *string) (raw string, id uuid.UUID, err error) {
	raw, hash, err := generateRefreshToken()
	if err != nil {
		return "", uuid.UUID{}, err
	}
	id, err = s.repo.Insert(ctx, userID, hash, time.Now().Add(s.ttl), userAgent)
	if err != nil {
		return "", uuid.UUID{}, err
	}
	return raw, id, nil
}

// IssueInTransaction makes a new refresh token and hands its digest and expiry
// to persist, which stores it as a step of the caller's own transaction (a
// password change replaces every session with this one token, in one
// transaction), under an id the caller chose: the session its access token,
// made before anything is stored, already names. The raw token is returned
// only when persist succeeded, so a token that was never stored is never
// handed out. Only the digest crosses into persist; the raw token never
// touches storage.
func (s *RefreshService) IssueInTransaction(ctx context.Context, persist func(hash []byte, expiresAt time.Time) error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	raw, hash, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	if err := persist(hash, time.Now().Add(s.ttl)); err != nil {
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
// already revoked. newID is the replacement's id, the session the new access
// token names; the session carries on under it (Rotate moves what was tied
// to the old id).
//
// A token that was already rotated is never taken twice: presenting it again
// (the answer to its refresh was lost on the way to the phone, or a thief
// has a copy) ends the whole session, the token it was rotated to and every
// one after it (endSession), so neither the phone nor a thief keeps a live
// session, or the pushes tied to it. The answer is ErrTokenInvalid either
// way. Two refreshes with one token at once end the session the same way. A
// token too many refreshes old (the repository caps its walk along the chain)
// is refused and ends nothing.
func (s *RefreshService) Redeem(ctx context.Context, token string, userAgent *string) (userID uuid.UUID, newToken string, newID uuid.UUID, err error) {
	hash, err := hashPresentedToken(token)
	if err != nil {
		return uuid.UUID{}, "", uuid.UUID{}, err
	}

	record, err := s.repo.GetActiveByHash(ctx, hash)
	if errors.Is(err, user.ErrTokenInvalid) {
		return uuid.UUID{}, "", uuid.UUID{}, s.endSession(ctx, hash)
	}
	if err != nil {
		return uuid.UUID{}, "", uuid.UUID{}, err
	}

	newRaw, newHash, err := generateRefreshToken()
	if err != nil {
		return uuid.UUID{}, "", uuid.UUID{}, err
	}
	newID, err = s.repo.Rotate(ctx, record.ID, newHash, time.Now().Add(s.ttl))
	if errors.Is(err, user.ErrTokenInvalid) {
		// Rotated (or revoked) since the check above.
		return uuid.UUID{}, "", uuid.UUID{}, s.endSession(ctx, hash)
	}
	if err != nil {
		return uuid.UUID{}, "", uuid.UUID{}, err
	}
	return record.UserID, newRaw, newID, nil
}

// endSession ends the session of a presented token that is no longer live
// (RevokeByHash follows a rotated one to the end of its chain; one revoked by
// a log out or a password change has nothing left to end) and answers
// ErrTokenInvalid, unless the revoke itself failed.
func (s *RefreshService) endSession(ctx context.Context, hash []byte) error {
	if err := s.repo.RevokeByHash(ctx, hash); err != nil && !errors.Is(err, user.ErrTokenInvalid) {
		return err
	}
	return user.ErrTokenInvalid
}

// Revoke ends the session of token, tolerating a miss (an already-invalid or
// malformed token) so that signing out with a stale token still succeeds. A
// token that was rotated since the phone stored it (a refresh answer that
// never arrived) still ends its session: RevokeByHash follows it to the token
// the session holds now. Any other repository error is propagated.
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
