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

func TestUserRepo_ClaimAndRevoke_VerifiesDropsPasswordAndRevokesTokensTogether(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	hash := "bcrypt-hash"
	created, err := repo.Create(ctx, "claim@example.com", &hash, false, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	liveHash := []byte("claim-live-token-hash-00000001")
	if _, err := tokenRepo.Insert(ctx, created.ID, liveHash, time.Now().Add(time.Hour), nil); err != nil {
		t.Fatalf("Insert token: %v", err)
	}

	claimed, err := repo.ClaimAndRevoke(ctx, created.ID, user.VerifiedViaGoogle)
	if err != nil || !claimed {
		t.Fatalf("ClaimAndRevoke: claimed = %v, err = %v, want true, nil", claimed, err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.EmailVerified {
		t.Error("EmailVerified = false, want true")
	}
	if got.EmailVerifiedVia == nil || *got.EmailVerifiedVia != user.VerifiedViaGoogle {
		t.Errorf("EmailVerifiedVia = %v, want google", got.EmailVerifiedVia)
	}
	if got.PasswordHash != nil {
		t.Errorf("PasswordHash = %q, want NULL", *got.PasswordHash)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, liveHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Errorf("GetActiveByHash(token issued before the claim) = %v, want ErrTokenInvalid", err)
	}
}

func TestUserRepo_ClaimAndRevoke_LeavesVerifiedAccountAndItsSessionsUntouched(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	hash := "bcrypt-hash"
	via := user.VerifiedViaPasswordFlow
	created, err := repo.Create(ctx, "verified@example.com", &hash, true, &via)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	liveHash := []byte("verified-live-token-hash-00001")
	if _, err := tokenRepo.Insert(ctx, created.ID, liveHash, time.Now().Add(time.Hour), nil); err != nil {
		t.Fatalf("Insert token: %v", err)
	}

	claimed, err := repo.ClaimAndRevoke(ctx, created.ID, user.VerifiedViaApple)
	if err != nil || claimed {
		t.Fatalf("ClaimAndRevoke: claimed = %v, err = %v, want false, nil", claimed, err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PasswordHash == nil || *got.PasswordHash != hash {
		t.Errorf("PasswordHash = %v, want %q (unchanged)", got.PasswordHash, hash)
	}
	if got.EmailVerifiedVia == nil || *got.EmailVerifiedVia != user.VerifiedViaPasswordFlow {
		t.Errorf("EmailVerifiedVia = %v, want password_flow (unchanged)", got.EmailVerifiedVia)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, liveHash); err != nil {
		t.Errorf("a verified account's session was revoked: %v", err)
	}
}

// --- one-request sign-up, birthday, delete ---

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

func TestUserRepo_UpdateProfile_BirthdayIsWriteOnce(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()
	created := mustCreateUser(t, repo, "once@example.com")
	if created.HasBirthday {
		t.Fatal("a Create'd account must have no birthday")
	}

	first, second := "1994-04-04", "1980-01-01"
	u, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Birthday: &first})
	if err != nil || !u.HasBirthday {
		t.Fatalf("first write: %v, has = %v", err, u != nil && u.HasBirthday)
	}
	if _, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Birthday: &second}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `select birthday::text from users where id = $1`, created.ID).Scan(&stored); err != nil || stored != first {
		t.Fatalf("stored birthday = %q (err %v), want the first value %q", stored, err, first)
	}

	// A patch without a birthday leaves it alone.
	name := "Someone"
	if _, err := repo.UpdateProfile(ctx, created.ID, user.ProfilePatch{Name: &name}); err != nil {
		t.Fatalf("name-only patch: %v", err)
	}
	if err := pool.QueryRow(ctx, `select birthday::text from users where id = $1`, created.ID).Scan(&stored); err != nil || stored != first {
		t.Fatalf("birthday after a name-only patch = %q (err %v)", stored, err)
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
	old := "1899-01-01"
	_, err = repo.UpdateProfile(context.Background(), created.ID, user.ProfilePatch{Birthday: &old})
	if !errors.As(err, &fieldErr) || fieldErr.Field != "birthday" {
		t.Fatalf("got %v, want FieldError for birthday", err)
	}
}

func TestUserRepo_Delete_OnlyWhileNoBirthdayAndTokensGoWithIt(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewUserRepo(pool)
	tokenRepo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	social := mustCreateUser(t, repo, "social-kid@example.com")
	tokenHash := []byte("delete-token-hash-000000000001")
	if _, err := tokenRepo.Insert(ctx, social.ID, tokenHash, time.Now().Add(time.Hour), nil); err != nil {
		t.Fatalf("Insert token: %v", err)
	}
	deleted, err := repo.Delete(ctx, social.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete: deleted = %v, err = %v, want true, nil", deleted, err)
	}
	if _, err := repo.GetByID(ctx, social.ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("GetByID after delete: %v, want ErrNotFound", err)
	}
	if _, err := tokenRepo.GetActiveByHash(ctx, tokenHash); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("token after delete: %v, want ErrTokenInvalid (cascade)", err)
	}

	// An account with a birthday on file is never deleted through this path.
	real, err := repo.CreateComplete(ctx, newAccount("real@example.com", "real_person"))
	if err != nil {
		t.Fatalf("CreateComplete: %v", err)
	}
	deleted, err = repo.Delete(ctx, real.ID)
	if err != nil || deleted {
		t.Fatalf("Delete of an account with a birthday: deleted = %v, err = %v, want false, nil", deleted, err)
	}
	if _, err := repo.GetByID(ctx, real.ID); err != nil {
		t.Fatalf("the account was deleted: %v", err)
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

// TestMigrations_0002IsReversible walks the test database down to version 1
// and back up to 2 with explicit `goto` steps (never `down 2`, which would
// drop 0001 and every table). It restores version 2 in a defer so a failure
// cannot leave the shared test database behind the schema other tests need.
// It runs under -p 1 with the rest of the suite and only on a *_test database.
// The whole suite MUST run with -p 1: this test drops the birthday column
// mid-run, so any other package using the same database in parallel would
// fail.
func TestMigrations_0002IsReversible(t *testing.T) {
	if testDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping postgres integration test")
	}
	if err := postgres.CheckTestDatabaseURL(testDatabaseURL); err != nil {
		t.Fatalf("%v", err)
	}
	ctx := context.Background()
	migrationsDir := filepath.Join(repoRoot(), "migrations")
	goTo := func(version int) {
		t.Helper()
		cmd := exec.Command("migrate", "-path", migrationsDir, "-database", testDatabaseURL, "goto", fmt.Sprint(version))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("migrate goto %d: %v: %s", version, err, out)
		}
	}
	defer func() {
		cmd := exec.Command("migrate", "-path", migrationsDir, "-database", testDatabaseURL, "goto", "2")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("restoring migration version 2 failed: %v: %s", err, out)
		}
	}()

	// A fresh pool for each phase, so no connection holds a statement cached
	// against the other schema.
	withPool := func(fn func(pool *pgxpool.Pool)) {
		t.Helper()
		pool, err := postgres.NewPool(ctx, testDatabaseURL)
		if err != nil {
			t.Fatalf("NewPool: %v", err)
		}
		defer pool.Close()
		fn(pool)
	}
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
	version := func(pool *pgxpool.Pool) (int, bool) {
		var v int
		var dirty bool
		if err := pool.QueryRow(ctx, `select version, dirty from schema_migrations`).Scan(&v, &dirty); err != nil {
			t.Fatalf("read schema_migrations: %v", err)
		}
		return v, dirty
	}

	withPool(func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if v, dirty := version(pool); v != 2 || dirty {
			t.Fatalf("expected version 2 (clean) before the test, got %d dirty=%v", v, dirty)
		}
		if !hasColumn(pool) || !hasConstraint(pool) {
			t.Fatal("expected the birthday column and users_birthday_min at version 2")
		}
		mustCreateUser(t, postgres.NewUserRepo(pool), "before-down@example.com")
	})

	goTo(1)
	withPool(func(pool *pgxpool.Pool) {
		if v, dirty := version(pool); v != 1 || dirty {
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

	goTo(2)
	withPool(func(pool *pgxpool.Pool) {
		if v, dirty := version(pool); v != 2 || dirty {
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
