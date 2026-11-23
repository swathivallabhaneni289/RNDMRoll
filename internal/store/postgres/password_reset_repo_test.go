package postgres_test

import (
	"context"
	"errors"
	"sync"
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

// Try counts a try only for a wrong code. The right code and the code a newer
// one replaced cost nothing; a used-up or expired code, and no code at all,
// answer neither right nor replaced, even for the right or the previous code.
// The previous code is kept only for an unexpired code that was replaced, and
// goes with the row when the code is spent.
func TestPasswordResetRepo_Try_CountsOnlyAWrongCode(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "try@example.com")
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	start := time.Now()
	const maxTries = 5
	a, b, c, d := []byte("code-a"), []byte("code-b"), []byte("code-c"), []byte("code-d")
	wrongCode := []byte("wrong-code")

	issue := func(codeHash []byte, at time.Time) {
		t.Helper()
		stored, err := repo.Issue(ctx, owner.ID, codeHash, at.Add(30*time.Minute), at, at.Add(-time.Minute), maxTries)
		if err != nil || !stored {
			t.Fatalf("Issue(%s): stored %v, err %v", codeHash, stored, err)
		}
	}
	try := func(userID uuid.UUID, codeHash []byte, at time.Time, wantRight, wantReplaced bool, wantTries int) {
		t.Helper()
		right, replaced, err := repo.Try(ctx, userID, codeHash, at, maxTries)
		if err != nil || right != wantRight || replaced != wantReplaced {
			t.Fatalf("Try(%s): right %v replaced %v err %v, want right %v replaced %v", codeHash, right, replaced, err, wantRight, wantReplaced)
		}
		if tries, _ := resetTries(t, pool, owner.ID); tries != wantTries {
			t.Fatalf("after Try(%s) the code has %d tries, want %d", codeHash, tries, wantTries)
		}
	}

	// No code yet: nothing is right and nothing is stored.
	try(owner.ID, a, start, false, false, 0)
	if _, ok := resetTries(t, pool, owner.ID); ok {
		t.Fatal("a try made a code")
	}

	issue(a, start)
	try(owner.ID, a, start, true, false, 0)
	try(owner.ID, a, start, true, false, 0) // the right code again: still free
	try(owner.ID, wrongCode, start, false, false, 1)

	// B replaces A two minutes on and keeps its try: A is now the previous
	// code, which costs nothing either.
	issue(b, start.Add(2*time.Minute))
	try(owner.ID, a, start.Add(2*time.Minute), false, true, 1)
	try(owner.ID, b, start.Add(2*time.Minute), true, false, 1)
	try(owner.ID, wrongCode, start.Add(2*time.Minute), false, false, 2)

	// C replaces B: only the code just replaced is the previous one, so A is
	// now simply wrong.
	issue(c, start.Add(4*time.Minute))
	try(owner.ID, b, start.Add(4*time.Minute), false, true, 2)
	try(owner.ID, a, start.Add(4*time.Minute), false, false, 3)

	// Used up: no code is right or replaced any more, and nothing more is
	// counted.
	try(owner.ID, wrongCode, start.Add(4*time.Minute), false, false, 4)
	try(owner.ID, wrongCode, start.Add(4*time.Minute), false, false, 5)
	try(owner.ID, c, start.Add(4*time.Minute), false, false, 5)
	try(owner.ID, b, start.Add(4*time.Minute), false, false, 5)

	// After C expires a new code D starts from zero, and C, which expired, is
	// not kept as the previous code.
	later := start.Add(40 * time.Minute)
	issue(d, later)
	try(owner.ID, c, later, false, false, 1)
	try(owner.ID, d, later, true, false, 1)

	// Another account, and no account at all (uuid.Nil), never match D.
	try(uuid.New(), d, later, false, false, 1)
	try(uuid.Nil, d, later, false, false, 1)

	// Expired: D is neither right nor counted.
	try(owner.ID, d, later.Add(31*time.Minute), false, false, 1)
	try(owner.ID, wrongCode, later.Add(31*time.Minute), false, false, 1)

	// E replaces D within its life; once E is spent, the row and D with it are
	// gone.
	e := []byte("code-e")
	issue(e, later.Add(2*time.Minute))
	try(owner.ID, d, later.Add(2*time.Minute), false, true, 1)
	if _, err := repo.SpendAndChangePassword(ctx, owner.ID, e, "new-bcrypt-hash"); err != nil {
		t.Fatalf("SpendAndChangePassword: %v", err)
	}
	if _, ok := resetTries(t, pool, owner.ID); ok {
		t.Fatal("the spent code is still there")
	}
	for _, code := range [][]byte{d, e} {
		if right, replaced, err := repo.Try(ctx, owner.ID, code, later.Add(2*time.Minute), maxTries); err != nil || right || replaced {
			t.Fatalf("Try(%s) after the spend: right %v replaced %v err %v", code, right, replaced, err)
		}
	}
}

// Wrong guesses sent at once stop at the limit, and the right code sent with
// them is never counted.
func TestPasswordResetRepo_Try_GuessesAtOnceStopAtTheLimit(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "try-at-once@example.com")
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	now := time.Now()
	const maxTries = 5
	right := []byte("right-code")
	if stored, err := repo.Issue(ctx, owner.ID, right, now.Add(30*time.Minute), now, now.Add(-time.Minute), maxTries); err != nil || !stored {
		t.Fatalf("Issue: stored %v, err %v", stored, err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 30)
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			code := []byte("wrong-code")
			if i%3 == 0 {
				code = right
			}
			if _, _, err := repo.Try(ctx, owner.ID, code, now, maxTries); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Try: %v", err)
	}
	if tries, ok := resetTries(t, pool, owner.ID); !ok || tries != maxTries {
		t.Fatalf("20 wrong guesses at once left %d tries (code there: %v), want %d", tries, ok, maxTries)
	}
}

// Issue gives a disabled account no code: it stores none, and it does not
// replace the one the account had before the disable. Enabled again, the
// account gets codes as before.
func TestPasswordResetRepo_Issue_GivesADisabledAccountNoCode(t *testing.T) {
	pool := requireTestPool(t)
	repo := postgres.NewPasswordResetRepo(pool)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "issue-disabled@example.com")
	ctx := context.Background()
	now := time.Now()
	issue := func(hash string, at time.Time) bool {
		t.Helper()
		stored, err := repo.Issue(ctx, owner.ID, []byte(hash), at.Add(30*time.Minute), at, at.Add(-time.Minute), 5)
		if err != nil {
			t.Fatalf("Issue(%s): %v", hash, err)
		}
		return stored
	}
	// Straight to the column: SetDisabled would also delete the code.
	setDisabled := func(disabled bool) {
		t.Helper()
		q := `update users set disabled_at = null where id = $1`
		if disabled {
			q = `update users set disabled_at = now() where id = $1`
		}
		if _, err := pool.Exec(ctx, q, owner.ID); err != nil {
			t.Fatal(err)
		}
	}
	code := func() string {
		t.Helper()
		var hash []byte
		err := pool.QueryRow(ctx, `select code_hash from password_resets where user_id = $1`, owner.ID).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(hash)
	}

	setDisabled(true)
	if issue("issue-disabled-1", now) || code() != "" {
		t.Fatalf("a disabled account was given a code (%q)", code())
	}
	setDisabled(false)
	if !issue("issue-disabled-2", now) || code() != "issue-disabled-2" {
		t.Fatalf("an account that is not disabled got no code (%q)", code())
	}
	// Two minutes on, the code may be replaced, but not while disabled.
	setDisabled(true)
	if issue("issue-disabled-3", now.Add(2*time.Minute)) || code() != "issue-disabled-2" {
		t.Fatalf("a disabled account's code was replaced: %q", code())
	}
	setDisabled(false)
	if !issue("issue-disabled-4", now.Add(2*time.Minute)) || code() != "issue-disabled-4" {
		t.Fatalf("enabled again, the code was not replaced: %q", code())
	}
}

// A spend changes the password and ends every session the account had, on
// every phone, and makes none of its own: no refresh token of the account is
// live after it, none was added, and its push tokens are gone with the code.
// Another account keeps its own. The person then logs in with the new
// password.
func TestPasswordResetRepo_Spend_EndsEverySessionAndMakesNone(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	now := time.Now()
	owner, err := users.CreateComplete(ctx, newAccount("spend-ends@example.com", "spend_ends"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.CreateComplete(ctx, newAccount("spend-ends-other@example.com", "spend_ends_other"))
	if err != nil {
		t.Fatal(err)
	}
	codes := map[uuid.UUID][]byte{owner.ID: []byte("spend-ends-owner-code"), other.ID: []byte("spend-ends-other-code")}
	for id, tag := range map[uuid.UUID]string{owner.ID: "spend-owner", other.ID: "spend-other"} {
		signedIn(t, pool, id, tag)
		if stored, err := repo.Issue(ctx, id, codes[id], now.Add(30*time.Minute), now, now.Add(-time.Minute), 5); err != nil || !stored {
			t.Fatalf("Issue: stored %v, err %v", stored, err)
		}
	}
	sessions := rowsOf(t, pool, "refresh_tokens", owner.ID)

	restored, err := repo.SpendAndChangePassword(ctx, owner.ID, codes[owner.ID], "reset-bcrypt-hash")
	if err != nil || restored {
		t.Fatalf("SpendAndChangePassword: restored %v, err %v, want false and no error", restored, err)
	}
	if got := storedPasswordHash(t, pool, owner.ID); got != "reset-bcrypt-hash" {
		t.Fatalf("stored hash = %q, want the new one", got)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 0 {
		t.Errorf("%d sessions still live, want none", n)
	}
	if n := rowsOf(t, pool, "refresh_tokens", owner.ID); n != sessions {
		t.Errorf("%d refresh tokens stored, want the %d there were: a spend makes no session", n, sessions)
	}
	for _, table := range []string{"push_tokens", "password_resets"} {
		if n := rowsOf(t, pool, table, owner.ID); n != 0 {
			t.Errorf("%d %s rows left, want none", n, table)
		}
	}

	if got := storedPasswordHash(t, pool, other.ID); got != "bcrypt-hash" {
		t.Errorf("another account's hash changed to %q", got)
	}
	if n := activeTokenCount(t, pool, other.ID); n != 2 {
		t.Errorf("another account has %d live sessions, want its 2", n)
	}
	for _, table := range []string{"push_tokens", "password_resets"} {
		if n := rowsOf(t, pool, table, other.ID); n != 1 {
			t.Errorf("another account has %d %s rows, want its 1", n, table)
		}
	}
}

// Inside the grace period of a deletion its owner asked for, a spend also
// cancels the deletion and says so (restored). A later spend, with nothing
// asked any more, restores nothing.
func TestPasswordResetRepo_Spend_CancelsADeletionInsideTheGracePeriod(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	now := time.Now()
	leaving, err := users.CreateComplete(ctx, newAccount("spend-leaving@example.com", "spend_leaving"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.RequestDeletion(ctx, leaving.ID); err != nil {
		t.Fatal(err)
	}
	spend := func(code, hash string, at time.Time) bool {
		t.Helper()
		if stored, err := repo.Issue(ctx, leaving.ID, []byte(code), at.Add(30*time.Minute), at, at.Add(-time.Minute), 5); err != nil || !stored {
			t.Fatalf("Issue(%s): stored %v, err %v", code, stored, err)
		}
		restored, err := repo.SpendAndChangePassword(ctx, leaving.ID, []byte(code), hash)
		if err != nil {
			t.Fatalf("SpendAndChangePassword(%s): %v", code, err)
		}
		return restored
	}

	if !spend("spend-leaving-code-1", "reset-bcrypt-hash", now) {
		t.Fatal("a spend inside the grace period did not say it restored the account")
	}
	var asked *time.Time
	if err := pool.QueryRow(ctx, `select deletion_requested_at from users where id = $1`, leaving.ID).Scan(&asked); err != nil || asked != nil {
		t.Fatalf("after the spend deletion_requested_at is %v (err %v), want none", asked, err)
	}
	if got := storedPasswordHash(t, pool, leaving.ID); got != "reset-bcrypt-hash" {
		t.Fatalf("stored hash = %q, want the new one", got)
	}
	if spend("spend-leaving-code-2", "reset-bcrypt-hash-2", now.Add(2*time.Minute)) {
		t.Fatal("a later spend said it restored the account again")
	}
}

// A refresh on another phone that is in flight while a reset spends its code
// (it has inserted its new token and holds the lock on its old one, and has
// not committed) must not leave a live session behind. The spend waits for
// that lock; the token the refresh then commits was invisible to its first
// revoke, so only the second revoke can end it. No session of the account is
// live after the spend.
func TestPasswordResetRepo_Spend_RevokesARefreshThatCommitsWhileItRuns(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	tokens := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPasswordResetRepo(pool)
	ctx := context.Background()
	now := time.Now()
	expires := now.Add(time.Hour)
	owner, err := users.CreateComplete(ctx, newAccount("spend-inflight@example.com", "spend_inflight"))
	if err != nil {
		t.Fatal(err)
	}
	oldID, err := tokens.Insert(ctx, owner.ID, []byte("spend-inflight-old-token-00001"), expires, nil)
	if err != nil {
		t.Fatal(err)
	}
	code := []byte("spend-inflight-code")
	if stored, err := repo.Issue(ctx, owner.ID, code, now.Add(30*time.Minute), now, now.Add(-time.Minute), 5); err != nil || !stored {
		t.Fatalf("Issue: stored %v, err %v", stored, err)
	}

	// The other phone's refresh, stopped just before its commit.
	rotating, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rotating.Rollback(ctx) //nolint:errcheck // a no-op once committed; frees the lock if the test stops early
	rotatedHash := []byte("spend-inflight-rotated-0000001")
	var rotatedID uuid.UUID
	if err := rotating.QueryRow(ctx, `insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, $3) returning id`,
		owner.ID, rotatedHash, expires).Scan(&rotatedID); err != nil {
		t.Fatal(err)
	}
	if _, err := rotating.Exec(ctx, `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2 and revoked_at is null`,
		rotatedID, oldID); err != nil {
		t.Fatal(err)
	}

	// The reset spends its code and has to wait for that row.
	done := make(chan error, 1)
	go func() {
		_, err := repo.SpendAndChangePassword(ctx, owner.ID, code, "reset-bcrypt-hash")
		done <- err
	}()
	waitForALockWait(t, pool)
	if err := rotating.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SpendAndChangePassword: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SpendAndChangePassword never finished")
	}

	if got := storedPasswordHash(t, pool, owner.ID); got != "reset-bcrypt-hash" {
		t.Errorf("stored hash = %q, want the new one", got)
	}
	if liveToken(t, tokens, rotatedHash) {
		t.Error("the token the refresh in flight committed is still live after the reset")
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 0 {
		t.Errorf("%d live refresh tokens after the reset, want none", n)
	}
}
