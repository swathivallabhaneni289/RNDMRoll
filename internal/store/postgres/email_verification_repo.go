package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// EmailVerificationRepo is a pgx-backed implementation of
// user.EmailVerificationRepository.
type EmailVerificationRepo struct {
	pool *pgxpool.Pool
}

// NewEmailVerificationRepo builds an EmailVerificationRepo backed by pool.
func NewEmailVerificationRepo(pool *pgxpool.Pool) user.EmailVerificationRepository {
	return &EmailVerificationRepo{pool: pool}
}

func (r *EmailVerificationRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	return errNotImplemented
}

func (r *EmailVerificationRepo) ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	return uuid.UUID{}, errNotImplemented
}

func (r *EmailVerificationRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	return errNotImplemented
}
