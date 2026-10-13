package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
		// Refuse a non-test database before anything migrates or truncates.
		if err := postgres.CheckTestDatabaseURL(testDatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "postgres: %v\n", err)
			os.Exit(1)
		}
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

func TestUserRepo_GetByUsernameCI_FindsAnyCaseAndReportsAMissingOne(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	created, err := repo.CreateComplete(ctx, newAccount("finder@example.com", "finder_name"))
	if err != nil {
		t.Fatalf("CreateComplete: %v", err)
	}

	for _, typed := range []string{"finder_name", "FINDER_Name"} {
		got, err := repo.GetByUsernameCI(ctx, typed)
		if err != nil {
			t.Fatalf("GetByUsernameCI(%q): %v", typed, err)
		}
		if got.ID != created.ID {
			t.Fatalf("GetByUsernameCI(%q): got account %v, want %v", typed, got.ID, created.ID)
		}
	}
	if _, err := repo.GetByUsernameCI(ctx, "nobody_here"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("GetByUsernameCI(unknown): got %v, want ErrNotFound", err)
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

func TestRefreshTokenRepo_Rotate_RevokedTokenIsRefusedAndMintsNothing(t *testing.T) {
	pool := requireTestPool(t)
	userRepo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	owner := mustCreateUser(t, userRepo, "rotate-revoked@example.com")

	oldHash := []byte("rotate-revoked-old-hash-000001")
	oldID, err := tokenRepo.Insert(ctx, owner.ID, oldHash, time.Now().Add(1*time.Hour), nil)
	if err != nil {
		t.Fatalf("Insert(old): %v", err)
	}
	// A revoke that lands after the caller validated the token but before Rotate.
	if err := tokenRepo.RevokeAllForUser(ctx, owner.ID); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}

	newHash := []byte("rotate-revoked-new-hash-000001")
	if _, err := tokenRepo.Rotate(ctx, oldID, newHash, time.Now().Add(1*time.Hour)); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Rotate(revoked token): got %v, want ErrTokenInvalid", err)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, newHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("GetActiveByHash(new hash after refused rotate): got %v, want ErrTokenInvalid (no row may survive)", err)
	}

	// Rotating the same token twice must not mint two live successors.
	liveHash := []byte("rotate-twice-old-hash-00000001")
	liveID, err := tokenRepo.Insert(ctx, owner.ID, liveHash, time.Now().Add(1*time.Hour), nil)
	if err != nil {
		t.Fatalf("Insert(live): %v", err)
	}
	if _, err := tokenRepo.Rotate(ctx, liveID, []byte("rotate-twice-new-hash-00000001"), time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("first Rotate: %v", err)
	}
	secondHash := []byte("rotate-twice-new-hash-00000002")
	if _, err := tokenRepo.Rotate(ctx, liveID, secondHash, time.Now().Add(1*time.Hour)); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("second Rotate of the same token: got %v, want ErrTokenInvalid", err)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, secondHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("GetActiveByHash(second successor): got %v, want ErrTokenInvalid", err)
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

// --- one-request sign-up and the birthday ---

func countUsers(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

func newAccount(email, username string) user.NewAccount {
	bio := "a short bio"
	return user.NewAccount{Email: email, PasswordHash: "bcrypt-hash", Birthday: "1990-06-15", Name: "Full Name", Username: username, Bio: &bio}
}

func TestUserRepo_CreateComplete_StoresEveryColumnInOneInsert(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	u, err := repo.CreateComplete(ctx, newAccount("complete@example.com", "complete_user"))
	if err != nil {
		t.Fatalf("CreateComplete: %v", err)
	}
	if u.Name == nil || *u.Name != "Full Name" || u.Username == nil || *u.Username != "complete_user" {
		t.Errorf("name/username = %v/%v", u.Name, u.Username)
	}
	if u.Bio == nil || *u.Bio != "a short bio" {
		t.Errorf("bio = %v", u.Bio)
	}
	if u.EmailVerified || u.EmailVerifiedVia != nil {
		t.Errorf("verified = %v via %v, want unverified", u.EmailVerified, u.EmailVerifiedVia)
	}
	if !u.HasBirthday || !u.OnboardingComplete() {
		t.Errorf("HasBirthday = %v, complete = %v, want both true", u.HasBirthday, u.OnboardingComplete())
	}
	var stored string
	if err := pool.QueryRow(ctx, `select birthday::text from users where id = $1`, u.ID).Scan(&stored); err != nil || stored != "1990-06-15" {
		t.Fatalf("stored birthday = %q, err = %v", stored, err)
	}
}

func TestUserRepo_CreateComplete_NilBioStoresNull(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	in := newAccount("nobio@example.com", "no_bio")
	in.Bio = nil
	u, err := repo.CreateComplete(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateComplete: %v", err)
	}
	if u.Bio != nil {
		t.Errorf("bio = %q, want NULL", *u.Bio)
	}
}

func TestUserRepo_CreateComplete_FailuresStoreNothing(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()
	if _, err := repo.CreateComplete(ctx, newAccount("holder@example.com", "held_name")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := repo.CreateComplete(ctx, newAccount("HOLDER@example.com", "fresh_name")); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("duplicate email: got %v, want ErrEmailTaken", err)
	}
	if _, err := repo.CreateComplete(ctx, newAccount("other@example.com", "held_name")); !errors.Is(err, user.ErrUsernameTaken) {
		t.Errorf("duplicate username: got %v, want ErrUsernameTaken", err)
	}

	badBio := newAccount("bio@example.com", "bio_user")
	long := strings.Repeat("b", 161)
	badBio.Bio = &long
	badBirthday := newAccount("old@example.com", "old_user")
	badBirthday.Birthday = "1899-12-31"
	badUsername := newAccount("charset@example.com", "BAD NAME")
	for name, c := range map[string]struct {
		in    user.NewAccount
		field string
	}{
		"bio over 160":      {badBio, "bio"},
		"birthday pre-1900": {badBirthday, "birthday"},
		"username charset":  {badUsername, "username"},
	} {
		_, err := repo.CreateComplete(ctx, c.in)
		var fieldErr *user.FieldError
		if !errors.As(err, &fieldErr) || fieldErr.Field != c.field {
			t.Errorf("%s: got %v, want FieldError for %q", name, err, c.field)
		} else if strings.Contains(err.Error(), "constraint") || strings.Contains(err.Error(), "violates") {
			t.Errorf("%s: error text carries database text: %q", name, err.Error())
		}
	}
	if n := countUsers(t, pool); n != 1 {
		t.Fatalf("user count = %d, want 1 (every failed insert must leave nothing)", n)
	}
}

func TestUserRepo_UpdateProfile_LeavesTheBirthdayAlone(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()
	created, err := repo.CreateComplete(ctx, newAccount("keeps@example.com", "keeps_birthday"))
	if err != nil {
		t.Fatalf("CreateComplete: %v", err)
	}

	name, bio := "Renamed", "a new bio"
	if _, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Name: &name, Bio: &bio}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `select birthday::text from users where id = $1`, created.ID).Scan(&stored); err != nil || stored != "1990-06-15" {
		t.Fatalf("birthday after a profile patch = %q (err %v), want 1990-06-15", stored, err)
	}

	// An account with no birthday on file (made before sign-up asked for one)
	// stays that way: a profile patch cannot add one.
	legacy := mustCreateUser(t, repo, "legacy@example.com")
	if _, err := repo.UpdateProfile(ctx, legacy.ID, user.ProfilePatch{Name: &name}); err != nil {
		t.Fatalf("legacy patch: %v", err)
	}
	got, err := repo.GetByID(ctx, legacy.ID)
	if err != nil || got.HasBirthday {
		t.Fatalf("legacy account has a birthday after a patch (has = %v, err %v)", got != nil && got.HasBirthday, err)
	}
}

func TestUserRepo_UpdateProfile_CheckViolationsAreFieldErrors(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	created := mustCreateUser(t, repo, "checks@example.com")

	bad := "NOT valid"
	_, err := repo.UpdateProfile(context.Background(), created.ID, user.ProfilePatch{Username: &bad})
	var fieldErr *user.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "username" {
		t.Fatalf("got %v, want FieldError for username", err)
	}
}

func TestCheckTestDatabaseURL(t *testing.T) {
	good := []string{
		"postgres://localhost:5432/rndmroll_test?sslmode=disable",
		"postgresql://user:pw@h/some_test",
	}
	for _, u := range good {
		if err := postgres.CheckTestDatabaseURL(u); err != nil {
			t.Errorf("CheckTestDatabaseURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"postgres://h/rndmroll_dev?sslmode=disable",
		"postgres://localhost:5432/rndmroll_test_backup",
		"postgres://localhost:5432/",
		"postgres://localhost:5432",
		"postgres://localhost:5432/rndmroll_test?dbname=rndmroll_dev",
		"host=localhost dbname=rndmroll_test",
		"host=localhost dbname=rndmroll_dev",
		"",
		"http://localhost/rndmroll_test",
	}
	for _, u := range bad {
		err := postgres.CheckTestDatabaseURL(u)
		if err == nil {
			t.Errorf("CheckTestDatabaseURL(%q) = nil, want an error", u)
		} else if u != "" && strings.Contains(err.Error(), "localhost:5432") {
			t.Errorf("error text echoes the URL: %v", err)
		}
	}
}

// latestMigration returns the highest migration number on disk, which is the
// version the shared test database sits at once TestMain has run `migrate up`.
// Reading it from the files means a new migration needs no edit to the tests
// below. They walk away from the latest version with explicit `goto` steps
// (never `down N`, which would drop 0001 and every table) and always restore
// it in a cleanup, so a failure cannot leave the shared test database behind
// the schema the other tests need. They run only on a *_test database. The
// whole suite MUST run with -p 1: these tests drop and re-add columns
// mid-run, so any other package using the same database in parallel would
// fail.
func latestMigration(t *testing.T) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(), "migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	highest := 0
	for _, f := range files {
		var n int
		if _, err := fmt.Sscanf(filepath.Base(f), "%d_", &n); err == nil && n > highest {
			highest = n
		}
	}
	if highest == 0 {
		t.Fatal("no migration files found")
	}
	return highest
}

// requireMigrationDatabase skips when no test database is configured, refuses
// anything that is not a *_test database, and registers the restore of the
// latest version.
func requireMigrationDatabase(t *testing.T) {
	t.Helper()
	if testDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping postgres integration test")
	}
	if err := postgres.CheckTestDatabaseURL(testDatabaseURL); err != nil {
		t.Fatalf("%v", err)
	}
	latest := latestMigration(t)
	t.Cleanup(func() {
		cmd := exec.Command("migrate", "-path", filepath.Join(repoRoot(), "migrations"), "-database", testDatabaseURL, "goto", fmt.Sprint(latest))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("restoring migration version %d failed: %v: %s", latest, err, out)
		}
	})
}

func migrateGoTo(t *testing.T, version int) {
	t.Helper()
	cmd := exec.Command("migrate", "-path", filepath.Join(repoRoot(), "migrations"), "-database", testDatabaseURL, "goto", fmt.Sprint(version))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migrate goto %d: %v: %s", version, err, out)
	}
}

// withFreshPool gives fn a new pool, so no connection holds a statement cached
// against the other schema.
func withFreshPool(t *testing.T, fn func(pool *pgxpool.Pool)) {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	fn(pool)
}

func schemaVersion(t *testing.T, pool *pgxpool.Pool) (int, bool) {
	t.Helper()
	var v int
	var dirty bool
	if err := pool.QueryRow(context.Background(), `select version, dirty from schema_migrations`).Scan(&v, &dirty); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	return v, dirty
}

// TestMigrations_0002IsReversible walks the test database from the latest
// version down to 1 and back up to 2. Going down must drop the birthday column
// and its constraint without losing rows; going up must restore both and leave
// every existing row NULL.
func TestMigrations_0002IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)

	hasColumn := func(pool *pgxpool.Pool) bool {
		var n int
		const q = `select count(*) from information_schema.columns where table_name = 'users' and column_name = 'birthday'`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("column check: %v", err)
		}
		return n == 1
	}
	hasConstraint := func(pool *pgxpool.Pool) bool {
		var n int
		const q = `select count(*) from pg_constraint where conname = 'users_birthday_min'`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("constraint check: %v", err)
		}
		return n == 1
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := schemaVersion(t, pool); v != latest || dirty {
			t.Fatalf("expected version %d (clean) before the test, got %d dirty=%v", latest, v, dirty)
		}
		if !hasColumn(pool) || !hasConstraint(pool) {
			t.Fatal("expected the birthday column and users_birthday_min at the latest version")
		}
		mustCreateUser(t, postgres.NewUserRepo(pool), "before-down@example.com")
	})

	migrateGoTo(t, 1)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 1 || dirty {
			t.Fatalf("after goto 1: version %d dirty=%v", v, dirty)
		}
		if hasColumn(pool) || hasConstraint(pool) {
			t.Fatal("goto 1 must drop the birthday column and its constraint")
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("goto 1 changed the user count to %d, want 1", n)
		}
		if _, err := pool.Exec(ctx, `insert into users (email) values ('written-at-v1@example.com')`); err != nil {
			t.Fatalf("insert at version 1: %v", err)
		}
	})

	migrateGoTo(t, 2)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 2 || dirty {
			t.Fatalf("after goto 2: version %d dirty=%v", v, dirty)
		}
		if !hasColumn(pool) || !hasConstraint(pool) {
			t.Fatal("goto 2 must restore the birthday column and constraint")
		}
		var withBirthday int
		if err := pool.QueryRow(ctx, `select count(*) from users where birthday is not null`).Scan(&withBirthday); err != nil || withBirthday != 0 {
			t.Fatalf("existing rows must stay NULL: %d (err %v)", withBirthday, err)
		}
		if n := countUsers(t, pool); n != 2 {
			t.Fatalf("user count after goto 2 = %d, want 2", n)
		}
		var pgErr *pgconn.PgError
		_, err := pool.Exec(ctx, `update users set birthday = date '1899-12-31'`)
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_birthday_min" {
			t.Fatalf("pre-1900 birthday: got %v, want a users_birthday_min violation", err)
		}
		if _, err := pool.Exec(ctx, `update users set birthday = date '1900-01-01'`); err != nil {
			t.Fatalf("1900-01-01 must be allowed: %v", err)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// TestMigrations_0003IsReversible walks the test database from the latest
// version down to 2 and back up. At version 3 the Apple and Google columns,
// their two unique indexes and the provider values of the verification check
// are gone; at version 2 they are all back and the unique indexes work. An
// existing account survives the trip in both directions.
func TestMigrations_0003IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)

	providerColumns := func(pool *pgxpool.Pool) int {
		var n int
		const q = `select count(*) from information_schema.columns where table_name = 'users' and column_name in ('apple_subject', 'google_subject')`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("column check: %v", err)
		}
		return n
	}
	providerIndexes := func(pool *pgxpool.Pool) int {
		var n int
		const q = `select count(*) from pg_indexes where tablename = 'users' and indexname in ('users_apple_subject_idx', 'users_google_subject_idx')`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("index check: %v", err)
		}
		return n
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := schemaVersion(t, pool); v != latest || dirty {
			t.Fatalf("expected version %d (clean) before the test, got %d dirty=%v", latest, v, dirty)
		}
		if providerColumns(pool) != 0 || providerIndexes(pool) != 0 {
			t.Fatal("the latest version must have no Apple or Google column or index")
		}
		// The narrowed check: only password_flow is a known verification source.
		var pgErr *pgconn.PgError
		_, err := pool.Exec(ctx, `insert into users (email, email_verified_via) values ('apple-source@example.com', 'apple')`)
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_email_verified_via_check" {
			t.Fatalf("an 'apple' verification source at version 3: got %v, want a users_email_verified_via_check violation", err)
		}
		if _, err := pool.Exec(ctx, `insert into users (email, email_verified_via) values ('flow-source@example.com', 'password_flow')`); err != nil {
			t.Fatalf("password_flow must stay allowed: %v", err)
		}
	})

	migrateGoTo(t, 2)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 2 || dirty {
			t.Fatalf("after goto 2: version %d dirty=%v", v, dirty)
		}
		if providerColumns(pool) != 2 || providerIndexes(pool) != 2 {
			t.Fatal("goto 2 must restore both provider columns and both unique indexes")
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("goto 2 changed the user count to %d, want 1", n)
		}
		if _, err := pool.Exec(ctx, `insert into users (email, email_verified_via, apple_subject) values ('a1@example.com', 'apple', 'sub-1')`); err != nil {
			t.Fatalf("an Apple account at version 2: %v", err)
		}
		var pgErr *pgconn.PgError
		_, err := pool.Exec(ctx, `insert into users (email, apple_subject) values ('a2@example.com', 'sub-1')`)
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_apple_subject_idx" {
			t.Fatalf("a repeated Apple subject: got %v, want a users_apple_subject_idx violation", err)
		}
		// Going up narrows the check, so no provider row may be left behind.
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate before going up: %v", err)
		}
		if _, err := pool.Exec(ctx, `insert into users (email) values ('survives-up@example.com')`); err != nil {
			t.Fatalf("insert at version 2: %v", err)
		}
	})

	migrateGoTo(t, 3)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 3 || dirty {
			t.Fatalf("after goto 3: version %d dirty=%v", v, dirty)
		}
		if providerColumns(pool) != 0 || providerIndexes(pool) != 0 {
			t.Fatal("goto 3 must drop both provider columns and both unique indexes")
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("user count after goto 3 = %d, want 1", n)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// TestMigrations_0004IsReversible walks the test database from the latest
// version down to 3 and back up to 4. At version 3 the six spin tables are
// gone and the users are untouched; going back up to 4 makes the tables
// again, empty. A user and one row in each new table go in first, so the
// down step has real rows to drop.
func TestMigrations_0004IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)
	if latest < 4 {
		t.Fatalf("latest migration is %d, want at least 4", latest)
	}
	spinTables := []string{"user_time_zones", "user_spin_state", "wheel_categories", "spins", "posted_days", "goal_weeks"}

	tablesPresent := func(pool *pgxpool.Pool) int {
		var n int
		const q = `select count(*) from information_schema.tables
			where table_schema = 'public' and table_name = any($1::text[])`
		if err := pool.QueryRow(ctx, q, spinTables).Scan(&n); err != nil {
			t.Fatalf("table check: %v", err)
		}
		return n
	}
	rowsIn := func(pool *pgxpool.Pool, table string) int {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := schemaVersion(t, pool); v != latest || dirty {
			t.Fatalf("expected version %d (clean) before the test, got %d dirty=%v", latest, v, dirty)
		}
		if n := tablesPresent(pool); n != len(spinTables) {
			t.Fatalf("expected all %d spin tables at the latest version, found %d", len(spinTables), n)
		}
		u := mustCreateUser(t, postgres.NewUserRepo(pool), "spin-migration@example.com")
		stmts := []string{
			`insert into user_time_zones (user_id, tz_name) values ($1, 'America/New_York')`,
			`insert into user_spin_state (user_id, seeded_at, seeded_local_date, seeded_tz, weekly_goal)
				values ($1, now(), current_date, 'America/New_York', 4)`,
			`insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0)`,
			`insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
				select $1, current_date, 'America/New_York', 'daily', 0, id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
				from wheel_categories where user_id = $1`,
			`insert into posted_days (user_id, local_date, source) values ($1, current_date, 'test')`,
			`insert into goal_weeks (user_id, week_start, goal, goal_met_at, bonus_granted)
				values ($1, current_date, 4, now(), true)`,
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s, u.ID); err != nil {
				t.Fatalf("insert a row (%s): %v", strings.Fields(s)[2], err)
			}
		}
		for _, tbl := range spinTables {
			if rowsIn(pool, tbl) != 1 {
				t.Fatalf("%s should hold one row before the down step", tbl)
			}
		}
	})

	migrateGoTo(t, 3)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 3 || dirty {
			t.Fatalf("after goto 3: version %d dirty=%v", v, dirty)
		}
		if n := tablesPresent(pool); n != 0 {
			t.Fatalf("goto 3 must drop all six spin tables, %d remain", n)
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("goto 3 changed the user count to %d, want 1", n)
		}
		// set_updated_at() belongs to 0001 and must survive 0004's down file.
		var fn int
		if err := pool.QueryRow(ctx, `select count(*) from pg_proc where proname = 'set_updated_at'`).Scan(&fn); err != nil || fn != 1 {
			t.Fatalf("set_updated_at() count = %d (err %v), want 1", fn, err)
		}
	})

	migrateGoTo(t, 4)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 4 || dirty {
			t.Fatalf("after goto 4: version %d dirty=%v", v, dirty)
		}
		if n := tablesPresent(pool); n != len(spinTables) {
			t.Fatalf("goto 4 must make all six spin tables again, found %d", n)
		}
		for _, tbl := range spinTables {
			if n := rowsIn(pool, tbl); n != 0 {
				t.Errorf("%s holds %d rows after going back up, want it empty", tbl, n)
			}
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("user count after goto 4 = %d, want 1", n)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// --- public or private account (migration 0005) ---

func TestUserRepo_AccountVisibility_DefaultsToPublicAndCheckRefusesOthers(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	// An insert that names no value is public: the column default.
	created := mustCreateUser(t, repo, "visibility-default@example.com")
	if created.AccountVisibility != "public" {
		t.Fatalf("a new account's value = %q, want public", created.AccountVisibility)
	}

	// Any other value written straight to the table is a check violation.
	var pgErr *pgconn.PgError
	_, err := pool.Exec(ctx, `update users set account_visibility = 'secret' where id = $1`, created.ID)
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_account_visibility_check" {
		t.Fatalf("a value of 'secret': got %v, want a users_account_visibility_check violation", err)
	}
	_, err = pool.Exec(ctx, `update users set account_visibility = null where id = $1`, created.ID)
	if !errors.As(err, &pgErr) || pgErr.Code != "23502" {
		t.Fatalf("a null value: got %v, want a not-null violation", err)
	}

	// The repository turns the violation into a field error with fixed text.
	bad := "secret"
	_, err = repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{AccountVisibility: &bad})
	var fieldErr *user.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "account_visibility" {
		t.Fatalf("UpdateProfile with 'secret': got %v, want a FieldError for account_visibility", err)
	}
	if strings.Contains(err.Error(), "constraint") || strings.Contains(err.Error(), "violates") {
		t.Fatalf("error text carries database text: %q", err.Error())
	}
	if got, err := repo.GetByID(ctx, created.ID); err != nil || got.AccountVisibility != "public" {
		t.Fatalf("after a refused update: %v (err %v), want public", got, err)
	}

	// A value sets it; a nil field leaves it.
	private := "private"
	updated, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{AccountVisibility: &private})
	if err != nil || updated.AccountVisibility != "private" {
		t.Fatalf("UpdateProfile(private): %v (err %v)", updated, err)
	}
	name := "Renamed Person"
	renamed, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Name: &name})
	if err != nil {
		t.Fatalf("UpdateProfile(name): %v", err)
	}
	if renamed.AccountVisibility != "private" {
		t.Fatalf("a patch with a nil field changed the value to %q", renamed.AccountVisibility)
	}
	var stored string
	if err := pool.QueryRow(ctx, `select account_visibility from users where id = $1`, created.ID).Scan(&stored); err != nil || stored != "private" {
		t.Fatalf("stored value = %q (err %v), want private", stored, err)
	}
}

func TestUserRepo_CreateComplete_StoresTheVisibilityGiven(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	for i, tc := range []struct{ given, want string }{
		{"private", "private"},
		{"public", "public"},
		{"", "public"}, // an empty value is stored as public, never as ''
	} {
		in := newAccount(fmt.Sprintf("vis-%d@example.com", i), fmt.Sprintf("vis_user_%d", i))
		in.AccountVisibility = tc.given
		u, err := repo.CreateComplete(ctx, in)
		if err != nil {
			t.Fatalf("CreateComplete(%q): %v", tc.given, err)
		}
		if u.AccountVisibility != tc.want {
			t.Errorf("CreateComplete(%q): returned %q, want %q", tc.given, u.AccountVisibility, tc.want)
		}
		var stored string
		if err := pool.QueryRow(ctx, `select account_visibility from users where id = $1`, u.ID).Scan(&stored); err != nil || stored != tc.want {
			t.Errorf("CreateComplete(%q): stored %q (err %v), want %q", tc.given, stored, err, tc.want)
		}
	}

	// A value the check refuses is a field error and stores nothing.
	before := countUsers(t, pool)
	in := newAccount("vis-bad@example.com", "vis_bad")
	in.AccountVisibility = "secret"
	_, err := repo.CreateComplete(ctx, in)
	var fieldErr *user.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "account_visibility" {
		t.Fatalf("CreateComplete('secret'): got %v, want a FieldError for account_visibility", err)
	}
	if n := countUsers(t, pool); n != before {
		t.Fatalf("a refused insert changed the user count from %d to %d", before, n)
	}
}

// TestMigrations_0005IsReversible walks the test database from the latest
// version down to 4 and back up to 5. At version 4 the account_visibility
// column and its check are gone and the user row is still there; going back up
// restores both, and every surviving row reads 'public'. Nothing here goes
// through the repository at version 4, because its column list names the
// column: the checks are plain SQL.
func TestMigrations_0005IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)
	if latest < 5 {
		t.Fatalf("latest migration is %d, want at least 5", latest)
	}

	hasColumn := func(pool *pgxpool.Pool) bool {
		var n int
		const q = `select count(*) from information_schema.columns where table_name = 'users' and column_name = 'account_visibility'`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("column check: %v", err)
		}
		return n == 1
	}
	hasConstraint := func(pool *pgxpool.Pool) bool {
		var n int
		const q = `select count(*) from pg_constraint where conname = 'users_account_visibility_check'`
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("constraint check: %v", err)
		}
		return n == 1
	}
	refusesSecret := func(pool *pgxpool.Pool) {
		t.Helper()
		var pgErr *pgconn.PgError
		_, err := pool.Exec(ctx, `update users set account_visibility = 'secret'`)
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_account_visibility_check" {
			t.Fatalf("a value of 'secret': got %v, want a users_account_visibility_check violation", err)
		}
	}
	valueOf := func(pool *pgxpool.Pool, email string) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, `select account_visibility from users where email = $1`, email).Scan(&v); err != nil {
			t.Fatalf("read the value for %s: %v", email, err)
		}
		return v
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := schemaVersion(t, pool); v != latest || dirty {
			t.Fatalf("expected version %d (clean) before the test, got %d dirty=%v", latest, v, dirty)
		}
		if !hasColumn(pool) || !hasConstraint(pool) {
			t.Fatal("expected the account_visibility column and its check at the latest version")
		}
		var def string
		const q = `select column_default from information_schema.columns where table_name = 'users' and column_name = 'account_visibility'`
		if err := pool.QueryRow(ctx, q).Scan(&def); err != nil || def != "'public'::text" {
			t.Fatalf("column default = %q (err %v), want 'public'::text", def, err)
		}
		if _, err := pool.Exec(ctx, `insert into users (email) values ('visibility-before-down@example.com')`); err != nil {
			t.Fatalf("insert before the down step: %v", err)
		}
		if got := valueOf(pool, "visibility-before-down@example.com"); got != "public" {
			t.Fatalf("a row with no value reads %q, want public", got)
		}
		refusesSecret(pool)
		// A private row: the down step drops the choice, so the row reads
		// 'public' after the round trip. That loss is the documented price of
		// going down; the row itself must survive.
		if _, err := pool.Exec(ctx, `update users set account_visibility = 'private'`); err != nil {
			t.Fatalf("set private: %v", err)
		}
	})

	migrateGoTo(t, 4)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 4 || dirty {
			t.Fatalf("after goto 4: version %d dirty=%v", v, dirty)
		}
		if hasColumn(pool) || hasConstraint(pool) {
			t.Fatal("goto 4 must drop the account_visibility column and its check")
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("goto 4 changed the user count to %d, want 1", n)
		}
		if _, err := pool.Exec(ctx, `insert into users (email) values ('written-at-v4@example.com')`); err != nil {
			t.Fatalf("insert at version 4: %v", err)
		}
	})

	migrateGoTo(t, 5)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 5 || dirty {
			t.Fatalf("after goto 5: version %d dirty=%v", v, dirty)
		}
		if !hasColumn(pool) || !hasConstraint(pool) {
			t.Fatal("goto 5 must restore the account_visibility column and its check")
		}
		if n := countUsers(t, pool); n != 2 {
			t.Fatalf("user count after goto 5 = %d, want 2", n)
		}
		for _, email := range []string{"visibility-before-down@example.com", "written-at-v4@example.com"} {
			if got := valueOf(pool, email); got != "public" {
				t.Errorf("%s reads %q after going back up, want public", email, got)
			}
		}
		refusesSecret(pool)
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// TestMigrations_0006IsReversible walks the test database from the latest
// version down to 5 and back up to 6. At version 5 the entries and
// entry_photos tables, spins.settled_at and the unique pair key on spins are
// gone while the users and their spins are untouched (and Phase 2.1's
// account_visibility column is still there); going back up makes them again,
// the two tables empty. A user, a spin, an entry and two photo rows go in
// first, so the down step has real rows to drop.
func TestMigrations_0006IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)
	if latest < 6 {
		t.Fatalf("latest migration is %d, want at least 6", latest)
	}

	count := func(pool *pgxpool.Pool, q string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	tablePresent := func(pool *pgxpool.Pool, name string) bool {
		return count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = $1`, name) == 1
	}
	columnPresent := func(pool *pgxpool.Pool, table, column string) bool {
		return count(pool, `select count(*) from information_schema.columns where table_name = $1 and column_name = $2`, table, column) == 1
	}
	constraintPresent := func(pool *pgxpool.Pool, name string) bool {
		return count(pool, `select count(*) from pg_constraint where conname = $1`, name) == 1
	}
	rowsIn := func(pool *pgxpool.Pool, table string) int {
		t.Helper()
		return count(pool, `select count(*) from `+table)
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := schemaVersion(t, pool); v != latest || dirty {
			t.Fatalf("expected version %d (clean) before the test, got %d dirty=%v", latest, v, dirty)
		}
		if !tablePresent(pool, "entries") || !tablePresent(pool, "entry_photos") ||
			!columnPresent(pool, "spins", "settled_at") || !constraintPresent(pool, "spins_id_user_unique") {
			t.Fatal("expected the entry tables, spins.settled_at and spins_id_user_unique at the latest version")
		}
		u := mustCreateUser(t, postgres.NewUserRepo(pool), "entry-migration@example.com")
		stmts := []string{
			`insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0)`,
			`insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
				select $1, '2026-10-08'::date, 'America/New_York', 'daily', 0, id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
				from wheel_categories where user_id = $1`,
			`insert into entries (user_id, spin_id, local_date, title, stars, created_at, updated_at)
				select $1, id, local_date, 'Dune', 7, now(), now() from spins where user_id = $1`,
			`insert into entry_photos (entry_id, user_id, position, storage_key, content_type, source, created_at)
				select e.id, e.user_id, g, 'entries/migration/' || g || '.jpg', 'image/jpeg', 'camera', now()
				from entries e, generate_series(0, 1) g where e.user_id = $1`,
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s, u.ID); err != nil {
				t.Fatalf("insert a row (%s): %v", strings.Fields(s)[2], err)
			}
		}
		if rowsIn(pool, "entries") != 1 || rowsIn(pool, "entry_photos") != 2 || rowsIn(pool, "spins") != 1 {
			t.Fatal("expected one entry, two photo rows and one spin before the down step")
		}
	})

	migrateGoTo(t, 5)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 5 || dirty {
			t.Fatalf("after goto 5: version %d dirty=%v", v, dirty)
		}
		if tablePresent(pool, "entries") || tablePresent(pool, "entry_photos") {
			t.Fatal("goto 5 must drop the entries and entry_photos tables")
		}
		if columnPresent(pool, "spins", "settled_at") || constraintPresent(pool, "spins_id_user_unique") {
			t.Fatal("goto 5 must drop spins.settled_at and spins_id_user_unique")
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("goto 5 changed the user count to %d, want 1", n)
		}
		if n := rowsIn(pool, "spins"); n != 1 {
			t.Fatalf("goto 5 changed the spin count to %d, want 1", n)
		}
		if !columnPresent(pool, "users", "account_visibility") {
			t.Fatal("goto 5 must keep users.account_visibility (it belongs to 0005)")
		}
	})

	migrateGoTo(t, 6)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 6 || dirty {
			t.Fatalf("after goto 6: version %d dirty=%v", v, dirty)
		}
		if !tablePresent(pool, "entries") || !tablePresent(pool, "entry_photos") {
			t.Fatal("goto 6 must make the entries and entry_photos tables again")
		}
		if !columnPresent(pool, "spins", "settled_at") || !constraintPresent(pool, "spins_id_user_unique") {
			t.Fatal("goto 6 must restore spins.settled_at and spins_id_user_unique")
		}
		for _, tbl := range []string{"entries", "entry_photos"} {
			if n := rowsIn(pool, tbl); n != 0 {
				t.Errorf("%s holds %d rows after going back up, want it empty", tbl, n)
			}
		}
		if n := countUsers(t, pool); n != 1 {
			t.Fatalf("user count after goto 6 = %d, want 1", n)
		}
		if n := count(pool, `select count(*) from spins where settled_at is not null`); n != 0 {
			t.Fatalf("%d spins have a settled_at after going back up, want none", n)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// --- change password ---

func activeTokenCount(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int {
	t.Helper()
	var n int
	const q = `select count(*) from refresh_tokens where user_id = $1 and revoked_at is null`
	if err := pool.QueryRow(context.Background(), q, userID).Scan(&n); err != nil {
		t.Fatalf("count active tokens: %v", err)
	}
	return n
}

func storedPasswordHash(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var h *string
	if err := pool.QueryRow(context.Background(), `select password_hash from users where id = $1`, id).Scan(&h); err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if h == nil {
		return ""
	}
	return *h
}

func TestUserRepo_ChangePassword_ReplacesHashAndEverySession(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokens := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	owner, err := repo.CreateComplete(ctx, newAccount("pw-change@example.com", "pw_change"))
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := repo.CreateComplete(ctx, newAccount("pw-other@example.com", "pw_other"))
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	hashA := []byte("pw-change-token-a-000000000001")
	hashB := []byte("pw-change-token-b-000000000001")
	hashC := []byte("pw-change-token-c-000000000001")
	hashOther := []byte("pw-change-token-o-000000000001")
	for _, h := range [][]byte{hashA, hashB, hashC} {
		if _, err := tokens.Insert(ctx, owner.ID, h, expires, nil); err != nil {
			t.Fatalf("insert token: %v", err)
		}
	}
	if _, err := tokens.Insert(ctx, other.ID, hashOther, expires, nil); err != nil {
		t.Fatalf("insert other's token: %v", err)
	}
	// C was revoked earlier (a log out); the change must leave that moment as
	// it was, not stamp it again.
	if err := tokens.RevokeByHash(ctx, hashC); err != nil {
		t.Fatalf("revoke C: %v", err)
	}
	var revokedBefore time.Time
	const revokedAtQ = `select revoked_at from refresh_tokens where token_hash = $1`
	if err := pool.QueryRow(ctx, revokedAtQ, hashC).Scan(&revokedBefore); err != nil {
		t.Fatalf("read C's revoked_at: %v", err)
	}
	var updatedBefore time.Time
	if err := pool.QueryRow(ctx, `select updated_at from users where id = $1`, owner.ID).Scan(&updatedBefore); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 2 {
		t.Fatalf("expected 2 active tokens before the change, got %d", n)
	}

	newTokenHash := []byte("pw-change-token-new-0000000001")
	agent := "RNDMRoll test phone"
	if err := repo.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", newTokenHash, expires, &agent); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if got := storedPasswordHash(t, pool, owner.ID); got != "new-bcrypt-hash" {
		t.Fatalf("stored hash = %q, want the new one", got)
	}
	for name, h := range map[string][]byte{"A": hashA, "B": hashB, "C": hashC} {
		if _, err := tokens.GetActiveByHash(ctx, h); !errors.Is(err, user.ErrTokenInvalid) {
			t.Errorf("token %s after the change: got %v, want ErrTokenInvalid", name, err)
		}
	}
	live, err := tokens.GetActiveByHash(ctx, newTokenHash)
	if err != nil {
		t.Fatalf("the new token should be active: %v", err)
	}
	if live.UserID != owner.ID {
		t.Fatalf("the new token belongs to %v, want %v", live.UserID, owner.ID)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 1 {
		t.Fatalf("active tokens after the change = %d, want exactly 1", n)
	}
	var revokedAfter time.Time
	if err := pool.QueryRow(ctx, revokedAtQ, hashC).Scan(&revokedAfter); err != nil || !revokedAfter.Equal(revokedBefore) {
		t.Errorf("C's revoked_at changed from %v to %v (err %v)", revokedBefore, revokedAfter, err)
	}
	var storedAgent *string
	if err := pool.QueryRow(ctx, `select user_agent from refresh_tokens where token_hash = $1`, newTokenHash).Scan(&storedAgent); err != nil || storedAgent == nil || *storedAgent != agent {
		t.Errorf("the new token's user agent = %v (err %v), want %q", storedAgent, err, agent)
	}
	var updatedAfter time.Time
	if err := pool.QueryRow(ctx, `select updated_at from users where id = $1`, owner.ID).Scan(&updatedAfter); err != nil || !updatedAfter.After(updatedBefore) {
		t.Errorf("updated_at did not move: %v then %v (err %v)", updatedBefore, updatedAfter, err)
	}

	// The other account is untouched.
	if _, err := tokens.GetActiveByHash(ctx, hashOther); err != nil {
		t.Errorf("another account's token was revoked: %v", err)
	}
	if got := storedPasswordHash(t, pool, other.ID); got != "bcrypt-hash" {
		t.Errorf("another account's hash changed to %q", got)
	}
	if n := countUsers(t, pool); n != 2 {
		t.Errorf("user count = %d, want 2", n)
	}
}

func TestUserRepo_ChangePassword_UnknownAccountChangesNothing(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokens := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	owner, err := repo.CreateComplete(ctx, newAccount("pw-bystander@example.com", "pw_bystander"))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	existing := []byte("pw-unknown-existing-0000000001")
	if _, err := tokens.Insert(ctx, owner.ID, existing, expires, nil); err != nil {
		t.Fatalf("insert token: %v", err)
	}

	newTokenHash := []byte("pw-unknown-new-token-000000001")
	err = repo.ChangePassword(ctx, uuid.New(), "new-bcrypt-hash", newTokenHash, expires, nil)
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("ChangePassword for an unknown account: got %v, want ErrNotFound", err)
	}

	var rows int
	if err := pool.QueryRow(ctx, `select count(*) from refresh_tokens`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("refresh_tokens rows = %d (err %v), want 1: nothing may be inserted", rows, err)
	}
	if _, err := tokens.GetActiveByHash(ctx, existing); err != nil {
		t.Errorf("a bystander's token changed: %v", err)
	}
	if got := storedPasswordHash(t, pool, owner.ID); got != "bcrypt-hash" {
		t.Errorf("a bystander's hash changed to %q", got)
	}
}

func TestUserRepo_ChangePassword_RollsBackWhenTheTokenInsertFails(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokens := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	owner, err := repo.CreateComplete(ctx, newAccount("pw-rollback@example.com", "pw_rollback"))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	hashA := []byte("pw-rollback-token-a-0000000001")
	hashB := []byte("pw-rollback-token-b-0000000001")
	for _, h := range [][]byte{hashA, hashB} {
		if _, err := tokens.Insert(ctx, owner.ID, h, expires, nil); err != nil {
			t.Fatalf("insert token: %v", err)
		}
	}

	// A new token whose digest is already stored: the insert breaks the unique
	// index after the hash was written and the sessions revoked.
	err = repo.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", hashA, expires, nil)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("ChangePassword with a duplicate token hash: got %v, want a unique violation", err)
	}

	if got := storedPasswordHash(t, pool, owner.ID); got != "bcrypt-hash" {
		t.Errorf("the hash is %q after a failed change, want the old one", got)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 2 {
		t.Errorf("active tokens = %d after a failed change, want the 2 the account had", n)
	}
	for name, h := range map[string][]byte{"A": hashA, "B": hashB} {
		if _, err := tokens.GetActiveByHash(ctx, h); err != nil {
			t.Errorf("token %s after a failed change: %v", name, err)
		}
	}
}

// waitForALockWait polls until some backend is waiting on a lock, so a test
// can be sure a statement is blocked before it lets the holder go.
func waitForALockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		const q = `select count(*) from pg_stat_activity where datname = current_database() and wait_event_type = 'Lock'`
		if err := pool.QueryRow(context.Background(), q).Scan(&n); err == nil && n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no statement ever waited on a row lock")
}

// A refresh on another phone that is in flight while the password changes (it
// has inserted its new token and holds the lock on its old one, and has not
// committed) must not leave a live session behind. The change waits for that
// lock; the token the refresh then commits was invisible to the change's
// first revoke, so only the second revoke can end it.
func TestUserRepo_ChangePassword_RevokesARefreshThatCommitsWhileItRuns(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokens := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	owner, err := repo.CreateComplete(ctx, newAccount("pw-inflight@example.com", "pw_inflight"))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	oldHash := []byte("pw-inflight-old-token-00000001")
	oldID, err := tokens.Insert(ctx, owner.ID, oldHash, expires, nil)
	if err != nil {
		t.Fatalf("insert old token: %v", err)
	}

	// The other phone's refresh, stopped just before its commit.
	rotating, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer rotating.Rollback(ctx) //nolint:errcheck // a no-op once committed; frees the lock if the test stops early
	rotatedHash := []byte("pw-inflight-rotated-0000000001")
	var rotatedID uuid.UUID
	const insertQ = `insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, $3) returning id`
	if err := rotating.QueryRow(ctx, insertQ, owner.ID, rotatedHash, expires).Scan(&rotatedID); err != nil {
		t.Fatalf("in-flight insert: %v", err)
	}
	const rotateQ = `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2 and revoked_at is null`
	if _, err := rotating.Exec(ctx, rotateQ, rotatedID, oldID); err != nil {
		t.Fatalf("in-flight rotate: %v", err)
	}

	// The password change starts and has to wait for that row.
	newHash := []byte("pw-inflight-new-token-00000001")
	done := make(chan error, 1)
	go func() {
		done <- repo.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", newHash, expires, nil)
	}()
	waitForALockWait(t, pool)

	if err := rotating.Commit(ctx); err != nil {
		t.Fatalf("commit the in-flight refresh: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ChangePassword: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ChangePassword never finished")
	}

	if got := storedPasswordHash(t, pool, owner.ID); got != "new-bcrypt-hash" {
		t.Errorf("stored hash = %q, want the new one", got)
	}
	if _, err := tokens.GetActiveByHash(ctx, rotatedHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Errorf("the token the in-flight refresh committed is still live: got %v, want ErrTokenInvalid", err)
	}
	if _, err := tokens.GetActiveByHash(ctx, newHash); err != nil {
		t.Errorf("the new token should be active: %v", err)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 1 {
		t.Errorf("active tokens = %d, want exactly the new one", n)
	}
}
