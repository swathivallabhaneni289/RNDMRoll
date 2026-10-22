package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// liveToken reports whether the refresh token with this digest is live.
func liveToken(t *testing.T, refresh user.RefreshTokenRepository, hash []byte) bool {
	t.Helper()
	_, err := refresh.GetActiveByHash(context.Background(), hash)
	if err != nil && !errors.Is(err, user.ErrTokenInvalid) && !errors.Is(err, user.ErrTokenExpired) {
		t.Fatalf("GetActiveByHash: %v", err)
	}
	return err == nil
}

// RevokeByHash ends the whole session a token belongs to. A rotated token
// leads along rotated_to to the token the session holds now, which it
// revokes, and the push token tied to it stops with it: one refresh answer
// lost on its way to the phone (the phone then logs out with the token from
// before it), or a stolen token presented after the phone's refresh. A token
// a log out revoked, or one nobody issued, ends nothing.
func TestRefreshTokenRepo_RevokeByHash_EndsTheSessionARotatedTokenLedTo(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "chain@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	pushes := postgres.NewPushRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	const tok = "ExponentPushToken[chain-phone]"

	h1, h2, h3 := []byte("chain-token-1-0000000000000001"), []byte("chain-token-2-0000000000000001"), []byte("chain-token-3-0000000000000001")
	first, err := refresh.Insert(ctx, owner.ID, h1, expires, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := refresh.Rotate(ctx, first, h2, expires)
	if err != nil {
		t.Fatal(err)
	}
	if err := pushes.SaveToken(ctx, owner.ID, second, tok, "ios", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := refresh.Rotate(ctx, second, h3, expires); err != nil {
		t.Fatal(err)
	}
	wantSentTo(t, pushes, owner.ID, tok)

	// The oldest token ends the session two refreshes on.
	if err := refresh.RevokeByHash(ctx, h1); err != nil {
		t.Fatalf("RevokeByHash with the first token: %v", err)
	}
	if liveToken(t, refresh, h3) {
		t.Fatal("the session's live token survived")
	}
	wantSentTo(t, pushes, owner.ID)
	// Nothing is left to end: every token of the chain says so.
	for name, h := range map[string][]byte{"first": h1, "second": h2, "third": h3} {
		if err := refresh.RevokeByHash(ctx, h); !errors.Is(err, user.ErrTokenInvalid) {
			t.Fatalf("RevokeByHash with the %s token again: %v, want ErrTokenInvalid", name, err)
		}
	}

	// A live token is revoked by itself, once; another session of the same
	// account is untouched.
	h4, h5 := []byte("chain-token-4-0000000000000001"), []byte("chain-token-5-0000000000000001")
	if _, err := refresh.Insert(ctx, owner.ID, h4, expires, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := refresh.Insert(ctx, owner.ID, h5, expires, nil); err != nil {
		t.Fatal(err)
	}
	if err := refresh.RevokeByHash(ctx, h4); err != nil {
		t.Fatalf("log out: %v", err)
	}
	if liveToken(t, refresh, h4) || !liveToken(t, refresh, h5) {
		t.Fatal("log out ended the wrong session")
	}
	if err := refresh.RevokeByHash(ctx, h4); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("a second log out: %v, want ErrTokenInvalid", err)
	}
	if err := refresh.RevokeByHash(ctx, []byte("chain-token-unknown-0000000001")); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("a token nobody issued: %v, want ErrTokenInvalid", err)
	}
	if !liveToken(t, refresh, h5) {
		t.Fatal("another session ended")
	}
}

// A refresh of the session's live token that is in flight when an old token
// ends the session cannot leave a live token behind: RevokeByHash waits for
// it and then follows it to the token it made. The same holds for a log out
// sent while the phone's own refresh with the same token is in flight.
func TestRefreshTokenRepo_RevokeByHash_FollowsARefreshInFlight(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "chain-inflight@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	for name, presented := range map[string]int{"an older token": 0, "the token being refreshed": 1} {
		t.Run(name, func(t *testing.T) {
			hashes := [][]byte{
				[]byte("inflight-a-" + uuid.NewString()),
				[]byte("inflight-b-" + uuid.NewString()),
				[]byte("inflight-c-" + uuid.NewString()),
			}
			first, err := refresh.Insert(ctx, owner.ID, hashes[0], expires, nil)
			if err != nil {
				t.Fatal(err)
			}
			second, err := refresh.Rotate(ctx, first, hashes[1], expires)
			if err != nil {
				t.Fatal(err)
			}
			// The refresh of the second token, stopped just before its commit.
			rotating, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rotating.Rollback(ctx) //nolint:errcheck // a no-op once committed
			var third uuid.UUID
			if err := rotating.QueryRow(ctx, `insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, $3) returning id`,
				owner.ID, hashes[2], expires).Scan(&third); err != nil {
				t.Fatal(err)
			}
			if _, err := rotating.Exec(ctx, `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2`, third, second); err != nil {
				t.Fatal(err)
			}

			done := make(chan error, 1)
			go func() { done <- refresh.RevokeByHash(ctx, hashes[presented]) }()
			waitForALockWait(t, pool)
			if err := rotating.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("RevokeByHash: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("RevokeByHash never finished")
			}
			if liveToken(t, refresh, hashes[2]) {
				t.Fatal("the token the refresh in flight made is still live")
			}
		})
	}
}

// Change password refuses an account an admin disabled, inside its
// transaction: nothing changes, and it says why.
func TestUserRepo_ChangePassword_RefusesADisabledAccount(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	refresh := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	owner, err := users.CreateComplete(ctx, newAccount("pw-disabled@example.com", "pw_disabled"))
	if err != nil {
		t.Fatal(err)
	}
	existing := []byte("pw-disabled-existing-000000001")
	if _, err := refresh.Insert(ctx, owner.ID, existing, expires, nil); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	if err := postgres.NewAdminRepo(pool).SetDisabled(ctx, owner.ID, &at); err != nil {
		t.Fatal(err)
	}
	err = users.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", uuid.New(), []byte("pw-disabled-new-00000000000001"), expires, nil)
	if !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("ChangePassword on a disabled account: %v, want ErrAccountDisabled", err)
	}
	if got := storedPasswordHash(t, pool, owner.ID); got != "bcrypt-hash" {
		t.Fatalf("the hash changed to %q", got)
	}
	var rows int
	if err := pool.QueryRow(ctx, `select count(*) from refresh_tokens where user_id = $1`, owner.ID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("refresh tokens %d (err %v), want the 1 there was", rows, err)
	}
	// Enabled again, it changes.
	if err := postgres.NewAdminRepo(pool).SetDisabled(ctx, owner.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := users.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", uuid.New(), []byte("pw-disabled-new-00000000000002"), expires, nil); err != nil {
		t.Fatalf("ChangePassword once enabled: %v", err)
	}
}

// When the right code can no longer be spent, SpendAndChangePassword says
// why and changes nothing: a newer code replaced it meanwhile, another
// request spent it, or the account is disabled or gone. A disable deletes
// the code, and a disabled account keeps its password even with its code.
func TestPasswordResetRepo_Spend_SaysWhyItCouldNotSpend(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	repo := postgres.NewPasswordResetRepo(pool)
	admins := postgres.NewAdminRepo(pool)
	ctx := context.Background()
	start := time.Now()
	owner, err := users.CreateComplete(ctx, newAccount("spend-why@example.com", "spend_why"))
	if err != nil {
		t.Fatal(err)
	}
	issue := func(code string, at time.Time) {
		t.Helper()
		stored, err := repo.Issue(ctx, owner.ID, []byte(code), at.Add(30*time.Minute), at, at.Add(-time.Minute), 5)
		if err != nil || !stored {
			t.Fatalf("Issue(%s): stored %v, err %v", code, stored, err)
		}
	}
	spend := func(code string) error {
		return repo.SpendAndChangePassword(ctx, owner.ID, []byte(code), "hash-after-"+code, uuid.New(),
			[]byte("spend-why-session-"+uuid.NewString()), start.Add(time.Hour), nil)
	}
	unchanged := func(what string) {
		t.Helper()
		if got := storedPasswordHash(t, pool, owner.ID); got != "bcrypt-hash" {
			t.Fatalf("%s: the hash is %q, want the old one", what, got)
		}
	}

	issue("code-a", start)
	issue("code-b", start.Add(2*time.Minute)) // b replaces a
	if err := spend("code-a"); !errors.Is(err, user.ErrResetCodeReplaced) {
		t.Fatalf("the replaced code: %v, want ErrResetCodeReplaced", err)
	}
	unchanged("the replaced code")

	// An account an admin disabled keeps its password and its code when the
	// code is still there (here the disable is set by hand, without the
	// deletion SetDisabled does).
	if _, err := pool.Exec(ctx, `update users set disabled_at = now() where id = $1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if err := spend("code-b"); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("a disabled account with its code: %v, want ErrAccountDisabled", err)
	}
	unchanged("a disabled account")
	if _, ok := resetTries(t, pool, owner.ID); !ok {
		t.Fatal("the disabled account's code was spent")
	}
	// And with no code (a disable deletes it), still disabled, not used.
	if err := spend("code-a"); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("a disabled account's replaced code: %v, want ErrAccountDisabled", err)
	}
	if _, err := pool.Exec(ctx, `update users set disabled_at = null where id = $1`, owner.ID); err != nil {
		t.Fatal(err)
	}

	if err := spend("code-b"); err != nil {
		t.Fatalf("the right code: %v", err)
	}
	if err := spend("code-b"); !errors.Is(err, user.ErrResetCodeUsed) {
		t.Fatalf("the right code again: %v, want ErrResetCodeUsed", err)
	}
	// Spent, then a new code asked for: the spent one is still used, not
	// replaced (a spend leaves no previous code).
	issue("code-c", start.Add(4*time.Minute))
	if err := spend("code-b"); !errors.Is(err, user.ErrResetCodeUsed) {
		t.Fatalf("a spent code after a new one: %v, want ErrResetCodeUsed", err)
	}

	// SetDisabled deletes the code.
	at := time.Now()
	if err := admins.SetDisabled(ctx, owner.ID, &at); err != nil {
		t.Fatal(err)
	}
	if _, ok := resetTries(t, pool, owner.ID); ok {
		t.Fatal("a disable kept the account's code")
	}
	if err := spend("code-c"); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("the code of a disabled account: %v, want ErrAccountDisabled", err)
	}

	// An account that is gone.
	err = repo.SpendAndChangePassword(ctx, uuid.New(), []byte("code-x"), "hash", uuid.New(),
		[]byte("spend-why-gone-0000000000000001"), start.Add(time.Hour), nil)
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("an account that is gone: %v, want ErrNotFound", err)
	}
}

// A refresh in flight while an admin disables the account (it has made its
// new token, holds the lock on its old one, and has moved the phone's push
// token, and has not committed) must not leave a live session behind:
// SetDisabled revokes every refresh token in its own transaction, and its
// second revoke ends the token the refresh commits while the first one waits.
// Enabled again, the account has no session and no phone to push to.
func TestAdminRepo_SetDisabled_RevokesARefreshThatCommitsWhileItRuns(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	refresh := postgres.NewRefreshTokenRepo(pool)
	pushes := postgres.NewPushRepo(pool)
	admins := postgres.NewAdminRepo(pool)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	owner, err := users.CreateComplete(ctx, newAccount("disable-inflight@example.com", "disable_inflight"))
	if err != nil {
		t.Fatal(err)
	}
	oldID, err := refresh.Insert(ctx, owner.ID, []byte("disable-inflight-old-000000001"), expires, nil)
	if err != nil {
		t.Fatal(err)
	}
	const tok = "ExponentPushToken[disable-inflight]"
	if err := pushes.SaveToken(ctx, owner.ID, oldID, tok, "ios", time.Now()); err != nil {
		t.Fatal(err)
	}

	// The phone's refresh, as Rotate runs it, stopped just before its commit.
	rotating, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rotating.Rollback(ctx) //nolint:errcheck // a no-op once committed; frees the locks if the test stops early
	rotatedHash := []byte("disable-inflight-new-000000001")
	var rotatedID uuid.UUID
	if err := rotating.QueryRow(ctx, `insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, $3) returning id`,
		owner.ID, rotatedHash, expires).Scan(&rotatedID); err != nil {
		t.Fatal(err)
	}
	if _, err := rotating.Exec(ctx, `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2 and revoked_at is null`,
		rotatedID, oldID); err != nil {
		t.Fatal(err)
	}
	if _, err := rotating.Exec(ctx, `update push_tokens set refresh_token_id = $1 where refresh_token_id = $2`, rotatedID, oldID); err != nil {
		t.Fatal(err)
	}

	// The admin disables the account, and has to wait for that refresh.
	at := time.Now()
	done := make(chan error, 1)
	go func() { done <- admins.SetDisabled(ctx, owner.ID, &at) }()
	waitForALockWait(t, pool)
	if err := rotating.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SetDisabled: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SetDisabled never finished")
	}

	if liveToken(t, refresh, rotatedHash) {
		t.Error("the token the refresh in flight committed is still live after the disable")
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 0 {
		t.Errorf("%d live refresh tokens after the disable, want none", n)
	}
	if err := admins.SetDisabled(ctx, owner.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := pushes.Tokens(ctx, owner.ID); err != nil || len(got) != 0 {
		t.Errorf("enabled again, pushes go to %v (err %v), want nowhere", got, err)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 0 {
		t.Errorf("enabled again, %d live refresh tokens, want none", n)
	}
}

// The walk from a reused token to the token its session holds now takes at
// most 50 steps: a token 50 refreshes old still ends the session, one 51
// refreshes old is refused and ends nothing.
func TestRefreshTokenRepo_RevokeByHash_WalksAtMost50Steps(t *testing.T) {
	pool := requireTestPool(t)
	refresh := postgres.NewRefreshTokenRepo(pool)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "walk-cap@example.com")
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	// chain makes a session refreshed n times and returns its token hashes,
	// the first one issued first.
	chain := func(name string, n int) [][]byte {
		t.Helper()
		hashes := [][]byte{[]byte(name + "-0")}
		id, err := refresh.Insert(ctx, owner.ID, hashes[0], expires, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= n; i++ {
			hash := []byte(fmt.Sprintf("%s-%d", name, i))
			if id, err = refresh.Rotate(ctx, id, hash, expires); err != nil {
				t.Fatal(err)
			}
			hashes = append(hashes, hash)
		}
		return hashes
	}

	within := chain("walk-cap-50", 50)
	if err := refresh.RevokeByHash(ctx, within[0]); err != nil {
		t.Fatalf("a token 50 refreshes old: %v, want the session ended", err)
	}
	if liveToken(t, refresh, within[50]) {
		t.Fatal("a token 50 refreshes old did not end the session")
	}

	past := chain("walk-cap-51", 51)
	if err := refresh.RevokeByHash(ctx, past[0]); !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("a token 51 refreshes old: %v, want ErrTokenInvalid", err)
	}
	if !liveToken(t, refresh, past[51]) {
		t.Fatal("a token 51 refreshes old ended the session")
	}
	// One refresh later than that, the same session ends.
	if err := refresh.RevokeByHash(ctx, past[1]); err != nil || liveToken(t, refresh, past[51]) {
		t.Fatalf("a token 50 refreshes old: %v, live %v; want the session ended", err, liveToken(t, refresh, past[51]))
	}
}
