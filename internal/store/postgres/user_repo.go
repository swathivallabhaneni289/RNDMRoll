package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// UserRepo is a pgx-backed implementation of user.Repository.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo builds a UserRepo backed by pool.
func NewUserRepo(pool *pgxpool.Pool) user.Repository {
	return &UserRepo{pool: pool}
}

var errNotImplemented = errors.New("postgres: not implemented")

func (r *UserRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *user.VerificationSource) (*user.User, error) {
	return nil, errNotImplemented
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return nil, errNotImplemented
}

func (r *UserRepo) GetByEmailCI(ctx context.Context, email string) (*user.User, error) {
	return nil, errNotImplemented
}

func (r *UserRepo) GetByProviderSubject(ctx context.Context, provider user.VerificationSource, subject string) (*user.User, error) {
	return nil, errNotImplemented
}

func (r *UserRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider user.VerificationSource, subject string) error {
	return errNotImplemented
}

func (r *UserRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	return false, errNotImplemented
}

func (r *UserRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, via user.VerificationSource) error {
	return errNotImplemented
}

func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p user.ProfilePatch) (*user.User, error) {
	return nil, errNotImplemented
}
