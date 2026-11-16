package spin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// txCountingRepo is the in-memory repository, counting the transactions it
// opens.
type txCountingRepo struct {
	*spintest.MemRepo
	txs int
}

func (r *txCountingRepo) InUserTx(ctx context.Context, userID string, seed spin.Seed, fn func(spin.Tx) error) error {
	r.txs++
	return r.MemRepo.InUserTx(ctx, userID, seed, fn)
}

// SetTimeZone with the zone already stored opens no transaction, so it never
// waits on the person's lock. A new zone does open one.
func TestSetTimeZone_TheStoredZoneAgainOpensNoTransaction(t *testing.T) {
	repo := &txCountingRepo{MemRepo: spintest.NewMemRepo()}
	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	svc := spin.NewService(repo, clk, spintest.NewQueueRand(20261007))
	u := uuid.NewString()
	steps := []struct {
		zone string
		txs  int
	}{
		{"America/New_York", 1},
		{"America/New_York", 1},
		{"Asia/Tokyo", 2},
		{"Asia/Tokyo", 2},
		{"America/New_York", 3},
	}
	for i, s := range steps {
		name, err := svc.SetTimeZone(context.Background(), u, s.zone)
		if err != nil || name != s.zone {
			t.Fatalf("step %d: SetTimeZone(%s) = %q, %v", i, s.zone, name, err)
		}
		if repo.txs != s.txs {
			t.Fatalf("step %d: %d transactions after reporting %s, want %d", i, repo.txs, s.zone, s.txs)
		}
		if got, ok, _ := repo.Zone(context.Background(), u); !ok || got != s.zone {
			t.Fatalf("step %d: stored zone = %q, want %s", i, got, s.zone)
		}
	}
}
