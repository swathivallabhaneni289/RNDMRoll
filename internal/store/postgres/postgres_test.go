package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// testDatabaseURL is set from TEST_DATABASE_URL in TestMain. Empty means no
// test database is configured; every test skips via requireTestPool.
var testDatabaseURL string

func TestMain(m *testing.M) {
	testDatabaseURL = os.Getenv("TEST_DATABASE_URL")
	if testDatabaseURL != "" {
		if err := runMigrations(testDatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "postgres: migration setup failed: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// runMigrations shells out to the migrate CLI to bring testDatabaseURL to
// the latest schema version before any test runs.
func runMigrations(dbURL string) error {
	if _, err := exec.LookPath("migrate"); err != nil {
		return fmt.Errorf("migrate CLI not found on PATH: %w", err)
	}
	migrationsDir := filepath.Join(repoRoot(), "migrations")
	cmd := exec.Command("migrate", "-path", migrationsDir, "-database", dbURL, "up")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("migrate up failed: %w: %s", err, string(out))
	}
	return nil
}

// repoRoot resolves the repository root from this test file's own location,
// independent of the working directory `go test` happens to run from.
func repoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("postgres: could not determine test file path")
	}
	// this file lives at <repoRoot>/internal/store/postgres/postgres_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// requireTestPool skips the calling test when TEST_DATABASE_URL is unset,
// otherwise returns a fresh pool and truncates every account table on
// cleanup so tests do not leak state into one another.
func requireTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping postgres integration test")
	}
	pool, err := postgres.NewPool(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("postgres: NewPool: %v", err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "truncate users cascade")
		if err != nil {
			t.Logf("postgres: cleanup truncate failed: %v", err)
		}
		pool.Close()
	})
	return pool
}

func mustCreateUser(t *testing.T, repo user.Repository, email string) *user.User {
	t.Helper()
	u, err := repo.Create(context.Background(), email, nil, false, nil)
	if err != nil {
		t.Fatalf("postgres: Create(%q): %v", email, err)
	}
	return u
}

func TestUserRepo_Create_CaseInsensitiveEmailConflict(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	mustCreateUser(t, repo, "Swathi@Example.com")

	_, err := repo.Create(ctx, "swathi@example.com", nil, false, nil)
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("Create with case-varied duplicate email: got %v, want ErrEmailTaken", err)
	}
}

func TestUserRepo_UpdateProfile_UsernameConflictReturnsErrUsernameTaken(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	first := mustCreateUser(t, repo, "first@example.com")
	second := mustCreateUser(t, repo, "second@example.com")

	takenUsername := "swathi_v"
	if _, err := repo.UpdateProfile(ctx, first.ID, user.ProfilePatch{Username: &takenUsername}); err != nil {
		t.Fatalf("UpdateProfile(first, username): %v", err)
	}

	_, err := repo.UpdateProfile(ctx, second.ID, user.ProfilePatch{Username: &takenUsername})
	if !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("UpdateProfile with already-held username: got %v, want ErrUsernameTaken", err)
	}
}

func TestUserRepo_UpdateProfile_NilFieldLeavesColumnUnchanged(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	created := mustCreateUser(t, repo, "unchanged@example.com")

	name := "Swathi V"
	if _, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Name: &name}); err != nil {
		t.Fatalf("UpdateProfile(name): %v", err)
	}

	bio := "building rndmroll"
	updated, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Bio: &bio})
	if err != nil {
		t.Fatalf("UpdateProfile(bio): %v", err)
	}

	if updated.Name == nil || *updated.Name != name {
		t.Fatalf("Name after unrelated update: got %v, want %q (should be unchanged)", updated.Name, name)
	}
	if updated.Bio == nil || *updated.Bio != bio {
		t.Fatalf("Bio: got %v, want %q", updated.Bio, bio)
	}
}

func TestRefreshTokenRepo_GetActiveByHash_ExpiredAndRevoked(t *testing.T) {
	pool := requireTestPool(t)
	userRepo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	owner := mustCreateUser(t, userRepo, "tokens@example.com")

	expiredHash := []byte("expired-token-hash-000000000001")
	if _, err := tokenRepo.Insert(ctx, owner.ID, expiredHash, time.Now().Add(-1*time.Hour), nil); err != nil {
		t.Fatalf("Insert(expired): %v", err)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, expiredHash); !errors.Is(err, user.ErrTokenExpired) {
		t.Fatalf("GetActiveByHash(expired): got %v, want ErrTokenExpired", err)
	}

	revokedHash := []byte("revoked-token-hash-000000000001")
	if _, err := tokenRepo.Insert(ctx, owner.ID, revokedHash, time.Now().Add(1*time.Hour), nil); err != nil {
		t.Fatalf("Insert(revoked): %v", err)
	}
	if err := tokenRepo.RevokeByHash(ctx, revokedHash); err != nil {
		t.Fatalf("RevokeByHash: %v", err)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, revokedHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("GetActiveByHash(revoked): got %v, want ErrTokenInvalid", err)
	}
}

func TestRefreshTokenRepo_Rotate_OldHashNoLongerResolves(t *testing.T) {
	pool := requireTestPool(t)
	userRepo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	owner := mustCreateUser(t, userRepo, "rotate@example.com")

	oldHash := []byte("rotate-old-token-hash-00000001")
	oldID, err := tokenRepo.Insert(ctx, owner.ID, oldHash, time.Now().Add(1*time.Hour), nil)
	if err != nil {
		t.Fatalf("Insert(old): %v", err)
	}

	newHash := []byte("rotate-new-token-hash-00000001")
	newID, err := tokenRepo.Rotate(ctx, oldID, newHash, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newID == uuid.Nil {
		t.Fatal("Rotate returned a nil UUID for the new token")
	}

	if _, err := tokenRepo.GetActiveByHash(ctx, oldHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("GetActiveByHash(old hash after rotate): got %v, want ErrTokenInvalid", err)
	}

	active, err := tokenRepo.GetActiveByHash(ctx, newHash)
	if err != nil {
		t.Fatalf("GetActiveByHash(new hash after rotate): %v", err)
	}
	if active.ID != newID {
		t.Fatalf("GetActiveByHash(new hash) returned ID %v, want %v", active.ID, newID)
	}
}

func TestEmailVerificationRepo_ConsumeByHash_SecondCallReturnsErrTokenConsumed(t *testing.T) {
	pool := requireTestPool(t)
	userRepo := postgres.NewUserRepo(pool)
	verifyRepo := postgres.NewEmailVerificationRepo(pool)
	ctx := context.Background()

	owner := mustCreateUser(t, userRepo, "verify@example.com")

	tokenHash := []byte("verify-token-hash-000000000001")
	if err := verifyRepo.Insert(ctx, owner.ID, tokenHash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	now := time.Now()
	gotUserID, err := verifyRepo.ConsumeByHash(ctx, tokenHash, now)
	if err != nil {
		t.Fatalf("ConsumeByHash (first call): %v", err)
	}
	if gotUserID != owner.ID {
		t.Fatalf("ConsumeByHash returned user %v, want %v", gotUserID, owner.ID)
	}

	if _, err := verifyRepo.ConsumeByHash(ctx, tokenHash, now); !errors.Is(err, user.ErrTokenConsumed) {
		t.Fatalf("ConsumeByHash (second call): got %v, want ErrTokenConsumed", err)
	}
}
