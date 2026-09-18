package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// RefreshTokenRepo is a pgx-backed implementation of user.RefreshTokenRepository.
type RefreshTokenRepo struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepo builds a RefreshTokenRepo backed by pool.
func NewRefreshTokenRepo(pool *pgxpool.Pool) user.RefreshTokenRepository {
	return &RefreshTokenRepo{pool: pool}
}

func (r *RefreshTokenRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error) {
	return uuid.UUID{}, errNotImplemented
}

func (r *RefreshTokenRepo) GetActiveByHash(ctx context.Context, tokenHash []byte) (*user.RefreshToken, error) {
	return nil, errNotImplemented
}

func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	return uuid.UUID{}, errNotImplemented
}

func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, tokenHash []byte) error {
	return errNotImplemented
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return errNotImplemented
}
