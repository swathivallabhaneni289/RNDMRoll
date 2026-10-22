package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// resetTries is how many tries the person's code has used, and whether they
// have a code at all.
func resetTries(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) (tries int, ok bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `select attempts from password_resets where user_id = $1`, userID).Scan(&tries)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("read the code's tries: %v", err)
	}
	return tries, true
}

// RefundTry gives back one try at the code it names and nothing else: never a
// try at another code, never below zero, and never a code that is gone or was
// replaced.
func TestPasswordResetRepo_RefundTry_GivesBackOnlyOneTryAtItsOwnCode(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "refund@example.com")
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	now := time.Now()
	right, wrong := []byte("right-code-hash"), []byte("wrong-code-hash")
	const maxTries = 5

	issue := func(codeHash []byte, at time.Time) {
		t.Helper()
		stored, err := repo.Issue(ctx, owner.ID, codeHash, at.Add(30*time.Minute), at, at.Add(-time.Minute), maxTries)
		if err != nil || !stored {
			t.Fatalf("Issue: stored %v, err %v", stored, err)
		}
	}
	try := func() {
		t.Helper()
		if _, ok, err := repo.Try(ctx, owner.ID, now, maxTries); err != nil || !ok {
			t.Fatalf("Try: ok %v, err %v", ok, err)
		}
	}
	refund := func(codeHash []byte, wantTries int, wantCode bool) {
		t.Helper()
		if err := repo.RefundTry(ctx, owner.ID, codeHash); err != nil {
			t.Fatalf("RefundTry: %v", err)
		}
		if tries, ok := resetTries(t, pool, owner.ID); tries != wantTries || ok != wantCode {
			t.Fatalf("after RefundTry(%s): %d tries (code there: %v), want %d (%v)", codeHash, tries, ok, wantTries, wantCode)
		}
	}

	issue(right, now)
	refund(right, 0, true) // nothing counted yet: nothing to give back
	try()
	try()
	refund(wrong, 2, true) // another code's hash gives nothing back
	refund(right, 1, true)
	refund(right, 0, true)
	refund(right, 0, true) // never below zero

	// Spent: the code is gone, and a refund does not bring it back.
	if _, err := pool.Exec(ctx, `delete from password_resets where user_id = $1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	refund(right, 0, false)

	// A new code in its place is not the old one: a refund for the old one
	// leaves its tries alone.
	issue([]byte("next-code-hash"), now)
	try()
	refund(right, 1, true)
}
