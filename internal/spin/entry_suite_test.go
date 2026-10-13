package spin_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// TestEntrySuite_MemRepo runs the shared entry suite against the in-memory
// repository. The Postgres run of the same suite is in
// internal/store/postgres.
func TestEntrySuite_MemRepo(t *testing.T) {
	spintest.RunEntrySuite(t, func(t *testing.T) *spintest.EntryEnv {
		repo := spintest.NewMemRepo()
		clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
		rnd := spintest.NewQueueRand(20261007)
		return &spintest.EntryEnv{
			Env: &spintest.Env{
				Service: spin.NewService(repo, clk, rnd),
				Clock:   clk,
				Rand:    rnd,
				NewUser: func(t *testing.T) string { return uuid.NewString() },
				State: func(t *testing.T, userID string) spin.UserState {
					t.Helper()
					st, ok := repo.State(userID)
					if !ok {
						t.Fatalf("no state row for %s", userID)
					}
					return st
				},
				Counts: func(t *testing.T, userID string) (int, int) {
					t.Helper()
					return repo.Counts(userID)
				},
				SetSpinResult: func(t *testing.T, userID, spinID string, result spin.SpinResult) {
					t.Helper()
					if err := repo.SetSpinResult(userID, spinID, result); err != nil {
						t.Fatal(err)
					}
				},
				ForceZone: func(t *testing.T, userID, name string) {
					t.Helper()
					if err := repo.ForceZone(userID, name); err != nil {
						t.Fatal(err)
					}
				},
				FillArchived: func(t *testing.T, userID string, unreferenced, referenced int) {
					t.Helper()
					if err := repo.FillArchived(userID, unreferenced, referenced); err != nil {
						t.Fatal(err)
					}
				},
			},
			Repo: repo,
		}
	})
}
