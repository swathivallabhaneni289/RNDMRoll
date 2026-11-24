package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// BRD-06: a like on a post that is gone by the time it is saved (the owner's
// delete, the removed-post purge or the account purge, racing the visibility
// read) is not found, as a comment is, so the route answers 404 and not 500.
// Before, the foreign key failure (23503) came back raw.
func TestBoardRepo_ALikeOnAPostThatIsGoneIsNotFound(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	u, err := postgres.NewUserRepo(pool).Create(ctx, "like-gone-"+uuid.NewString()+"@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	err = postgres.NewBoardRepo(pool).SetReaction(ctx, uuid.New(), u.ID, "love", time.Now().UTC())
	if !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("a like on a post that is gone: %v, want board.ErrNotFound", err)
	}
}
