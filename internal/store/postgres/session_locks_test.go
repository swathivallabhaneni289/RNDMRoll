package postgres_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// randomTokenHash stands in for a refresh token's digest.
func randomTokenHash(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// isDeadlock reports Postgres's deadlock_detected (SQLSTATE 40P01).
func isDeadlock(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

// Password changes, resets, refreshes, push token saves and deletes, log
// outs, old refresh tokens presented again and an admin's disable and enable,
// all at once on one account, never deadlock: every path takes the tables in
// the order of session_locks.go and never goes back. Each phone refreshes its
// session and saves its push token straight after, as the app does, while
// password changes and resets end every session over and over. (Before the
// order was fixed, a password change went back to refresh_tokens after
// push_tokens, and this test met dozens of deadlocks.)
func TestSessionLocks_EverythingAtOnceNeverDeadlocks(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	refresh := postgres.NewRefreshTokenRepo(pool)
	pushes := postgres.NewPushRepo(pool)
	resets := postgres.NewPasswordResetRepo(pool)
	admins := postgres.NewAdminRepo(pool)
	owner, err := users.CreateComplete(context.Background(), newAccount("stress@example.com", "stress_me"))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	const phones, changers, rounds = 6, 2, 120
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		deadlocks int
		others    []error
		stale     [][]byte // refresh tokens already rotated, for the thief
	)
	fail := func(what string, err error) {
		mu.Lock()
		defer mu.Unlock()
		if isDeadlock(err) {
			deadlocks++
			return
		}
		others = append(others, fmt.Errorf("%s: %w", what, err))
	}
	rotated := func(hash []byte) {
		mu.Lock()
		defer mu.Unlock()
		stale = append(stale, hash)
	}
	staleOne := func(i int) []byte {
		mu.Lock()
		defer mu.Unlock()
		if len(stale) == 0 {
			return nil
		}
		return stale[i%len(stale)]
	}
	// expected are the refusals the paths give while sessions end under them.
	expected := func(err error) bool {
		return err == nil || errors.Is(err, user.ErrTokenInvalid) || errors.Is(err, user.ErrAccountDisabled) ||
			errors.Is(err, user.ErrResetCodeUsed) || errors.Is(err, user.ErrResetCodeReplaced)
	}

	for p := range phones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token := fmt.Sprintf("ExponentPushToken[stress-%d]", p)
			signIn := func() (uuid.UUID, []byte) {
				hash := randomTokenHash(t)
				id, err := refresh.Insert(ctx, owner.ID, hash, expires, nil)
				if err != nil {
					fail("sign in", err)
				}
				return id, hash
			}
			session, hash := signIn()
			first := session
			for round := range rounds {
				newHash := randomTokenHash(t)
				next, err := refresh.Rotate(ctx, session, newHash, expires)
				switch {
				case errors.Is(err, user.ErrTokenInvalid):
					// A password change ended the session: sign in again.
					session, hash = signIn()
					first = session
					continue
				case err != nil:
					fail("refresh", err)
					continue
				}
				rotated(hash)
				session, hash = next, newHash
				// The token goes with the access token the phone holds: the new
				// session's, and now and then the one from before the refresh.
				saveWith := session
				if round%3 == 0 {
					saveWith = first
				}
				if err := pushes.SaveToken(ctx, owner.ID, saveWith, token, "ios", time.Now()); err != nil {
					fail("save the push token", err)
				}
				switch round % 10 {
				case 4:
					if err := deletePushToken(ctx, pushes, owner.ID, session, token); err != nil {
						fail("delete the push token", err)
					}
				case 7:
					// A log out, then a new sign in on the phone.
					if err := refresh.RevokeByHash(ctx, hash); err != nil && !errors.Is(err, user.ErrTokenInvalid) {
						fail("log out", err)
					}
					session, hash = signIn()
					first = session
				}
			}
		}()
	}
	for c := range changers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := range rounds / 2 {
				err := users.ChangePassword(ctx, owner.ID, fmt.Sprintf("hash-%d-%d", c, round), uuid.New(),
					randomTokenHash(t), expires, nil)
				if !expected(err) {
					fail("change password", err)
				}
				if _, err := pushes.Tokens(ctx, owner.ID); err != nil {
					fail("read the tokens", err)
				}
			}
		}()
	}
	// A thief presenting tokens the phones already rotated: each ends the
	// session it grew into.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := range rounds {
			if hash := staleOne(round * 7); hash != nil {
				if err := refresh.RevokeByHash(ctx, hash); !expected(err) {
					fail("present a rotated token", err)
				}
			}
		}
	}()
	// Forgot password: a code, the right try, the spend with its change.
	wg.Add(1)
	go func() {
		defer wg.Done()
		at := time.Now()
		for round := range rounds / 4 {
			code := []byte(fmt.Sprintf("stress-code-%d", round))
			at = at.Add(31 * time.Minute) // each code after the last expired
			if _, err := resets.Issue(ctx, owner.ID, code, at.Add(30*time.Minute), at, at.Add(-time.Minute), 5); err != nil {
				fail("issue a code", err)
				continue
			}
			if _, _, err := resets.Try(ctx, owner.ID, code, at, 5); err != nil {
				fail("try the code", err)
			}
			err := resets.SpendAndChangePassword(ctx, owner.ID, code, "reset-hash", uuid.New(), randomTokenHash(t), expires, nil)
			if !expected(err) {
				fail("reset", err)
			}
		}
	}()
	// An admin disabling the account and enabling it again.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rounds / 20 {
			at := time.Now()
			if err := admins.SetDisabled(ctx, owner.ID, &at); err != nil {
				fail("disable", err)
			}
			if err := refresh.RevokeAllForUser(ctx, owner.ID); err != nil {
				fail("end every session", err)
			}
			if err := admins.SetDisabled(ctx, owner.ID, nil); err != nil {
				fail("enable", err)
			}
		}
	}()
	wg.Wait()

	if deadlocks > 0 {
		t.Errorf("%d deadlocks", deadlocks)
	}
	for _, err := range others {
		t.Errorf("unexpected error: %v", err)
	}

	// Once everything has stopped, one more change leaves exactly its own
	// session, and no phone is sent to until it saves its token again.
	last := uuid.New()
	if err := users.ChangePassword(ctx, owner.ID, "final-hash", last, randomTokenHash(t), expires, nil); err != nil {
		t.Fatalf("the last change: %v", err)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 1 {
		t.Fatalf("live sessions after the last change: %d, want 1", n)
	}
	wantSentTo(t, pushes, owner.ID)
}

// deletePushToken is the phone's DELETE of its push token.
func deletePushToken(ctx context.Context, pushes *postgres.PushRepo, userID, session uuid.UUID, token string) error {
	return pushes.DeleteToken(ctx, userID, session, token)
}
