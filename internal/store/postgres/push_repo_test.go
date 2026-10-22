package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// pushSession is the refresh token a push token is tied to (nil for none),
// and whether the token is kept at all.
func pushSession(t *testing.T, pool *pgxpool.Pool, token string) (session *uuid.UUID, kept bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `select refresh_token_id from push_tokens where token = $1`, token).Scan(&session)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("read the token's session: %v", err)
	}
	return session, true
}

// sentTo is what the notifier would send to for userID.
func sentTo(t *testing.T, repo *postgres.PushRepo, userID uuid.UUID) []string {
	t.Helper()
	tokens, err := repo.Tokens(context.Background(), userID)
	if err != nil {
		t.Fatalf("Tokens: %v", err)
	}
	return tokens
}

// wantSentTo checks what the notifier would send to for userID, in any order.
func wantSentTo(t *testing.T, repo *postgres.PushRepo, userID uuid.UUID, want ...string) {
	t.Helper()
	got := sentTo(t, repo, userID)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sent to %q, want %q", got, want)
	}
}

// A push token is sent to only while the session that saved it lasts: a
// refresh keeps it (the token moves to the new refresh token), a log out or
// an expiry ends it, and a token saved with no session, or with someone
// else's, is never sent to.
func TestPushRepo_TokensFollowTheSession(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	owner := mustCreateUser(t, users, "push-owner@example.com")
	other := mustCreateUser(t, users, "push-other@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPushRepo(pool)
	ctx := context.Background()
	now := time.Now()
	const tok = "ExponentPushToken[session-one]"

	first, err := refresh.Insert(ctx, owner.ID, []byte("push-session-first-00000000001"), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := repo.SaveToken(ctx, owner.ID, first, tok, "ios", now); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	wantSentTo(t, repo, owner.ID, tok)

	// A refresh: the token moves to the new refresh token and is still sent to.
	second, err := refresh.Rotate(ctx, first, []byte("push-session-second-0000000001"), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if s, _ := pushSession(t, pool, tok); s == nil || *s != second {
		t.Fatalf("after a refresh the token is tied to %v, want %v", s, second)
	}
	wantSentTo(t, repo, owner.ID, tok)

	// Saved again with an access token from before the refresh: it is tied to
	// the session's current refresh token, not the old one.
	if err := repo.SaveToken(ctx, owner.ID, first, tok, "ios", now); err != nil {
		t.Fatalf("SaveToken with the old session: %v", err)
	}
	if s, _ := pushSession(t, pool, tok); s == nil || *s != second {
		t.Fatalf("saved with the old session, the token is tied to %v, want %v", s, second)
	}
	wantSentTo(t, repo, owner.ID, tok)

	// Log out (the refresh token is revoked), with no DELETE of the token: it
	// is kept, and no longer sent to, also when saved again by that session.
	if err := refresh.RevokeByHash(ctx, []byte("push-session-second-0000000001")); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	wantSentTo(t, repo, owner.ID)
	if err := repo.SaveToken(ctx, owner.ID, first, tok, "ios", now); err != nil {
		t.Fatalf("SaveToken after log out: %v", err)
	}
	wantSentTo(t, repo, owner.ID)

	// A new sign in on the phone saves it with the new session; when that
	// session expires, the token is not sent to.
	third, err := refresh.Insert(ctx, owner.ID, []byte("push-session-third-00000000001"), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := repo.SaveToken(ctx, owner.ID, third, tok, "ios", now); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	wantSentTo(t, repo, owner.ID, tok)
	if _, err := pool.Exec(ctx, `update refresh_tokens set expires_at = now() - interval '1 second' where id = $1`, third); err != nil {
		t.Fatalf("expire the session: %v", err)
	}
	wantSentTo(t, repo, owner.ID)

	// No session (an access token from before sessions were named), and
	// another person's session, tie the token to nothing, so neither takes it
	// from the live session it is tied to.
	fourthHash := []byte("push-session-fourth-0000000001")
	fourth, err := refresh.Insert(ctx, owner.ID, fourthHash, now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := repo.SaveToken(ctx, owner.ID, fourth, tok, "ios", now); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	wantSentTo(t, repo, owner.ID, tok)
	othersSession, err := refresh.Insert(ctx, other.ID, []byte("push-session-other-00000000001"), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	for name, session := range map[string]uuid.UUID{"no session": uuid.Nil, "another person's session": othersSession} {
		if err := repo.SaveToken(ctx, owner.ID, session, tok, "ios", now); err != nil {
			t.Fatalf("SaveToken with %s: %v", name, err)
		}
		if s, _ := pushSession(t, pool, tok); s == nil || *s != fourth {
			t.Fatalf("saved with %s over a live session, the token is tied to %v, want %v", name, s, fourth)
		}
		wantSentTo(t, repo, owner.ID, tok)
	}
	// Once that session has ended they do: the token is kept, tied to
	// nothing, and not sent to.
	if err := refresh.RevokeByHash(ctx, fourthHash); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	for name, session := range map[string]uuid.UUID{"no session": uuid.Nil, "another person's session": othersSession} {
		if err := repo.SaveToken(ctx, owner.ID, session, tok, "ios", now); err != nil {
			t.Fatalf("SaveToken with %s: %v", name, err)
		}
		if s, kept := pushSession(t, pool, tok); !kept || s != nil {
			t.Fatalf("saved with %s over an ended session: kept %v, tied to %v", name, kept, s)
		}
		wantSentTo(t, repo, owner.ID)
		wantSentTo(t, repo, other.ID)
	}
}

// pushOwner is the account the token is kept for ("" when it is not kept).
func pushOwner(t *testing.T, pool *pgxpool.Pool, token string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `select user_id::text from push_tokens where token = $1`, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	if err != nil {
		t.Fatalf("read the token's account: %v", err)
	}
	return id
}

// A save or a delete that arrives late, from a session that has ended (or
// from an access token that names none), never takes the phone's pushes from
// the live session signed in on it now; a save from a live session always
// takes them, and a delete from the session the token is tied to, or after
// that session ended, deletes it.
func TestPushRepo_ALateSaveOrDeleteNeverTakesALiveSessionsToken(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	alice := mustCreateUser(t, users, "late-alice@example.com")
	bob := mustCreateUser(t, users, "late-bob@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPushRepo(pool)
	ctx := context.Background()
	now := time.Now()
	const tok = "ExponentPushToken[late-phone]"
	signIn := func(u uuid.UUID, hash string) uuid.UUID {
		t.Helper()
		id, err := refresh.Insert(ctx, u, []byte(hash), now.Add(time.Hour), nil)
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		return id
	}
	save := func(u, session uuid.UUID) {
		t.Helper()
		if err := repo.SaveToken(ctx, u, session, tok, "ios", now); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}
	}
	remove := func(u, session uuid.UUID) {
		t.Helper()
		if err := repo.DeleteToken(ctx, u, session, tok); err != nil {
			t.Fatalf("DeleteToken: %v", err)
		}
	}
	tiedTo := func(want uuid.UUID, wantOwner uuid.UUID, what string) {
		t.Helper()
		if s, _ := pushSession(t, pool, tok); s == nil || *s != want {
			t.Fatalf("%s: the token is tied to %v, want %v", what, s, want)
		}
		if got := pushOwner(t, pool, tok); got != wantOwner.String() {
			t.Fatalf("%s: the token is kept for %s, want %s", what, got, wantOwner)
		}
	}

	// Alice's phone; she logs out, and Bob signs in on it.
	aliceOld := signIn(alice.ID, "late-alice-old-000000000000001")
	save(alice.ID, aliceOld)
	if err := refresh.RevokeByHash(ctx, []byte("late-alice-old-000000000000001")); err != nil {
		t.Fatal(err)
	}
	bobs := signIn(bob.ID, "late-bob-000000000000000000001")
	save(bob.ID, bobs)
	tiedTo(bobs, bob.ID, "Bob signed in")

	// Alice's save from her ended session, or with no session, comes late:
	// the phone stays Bob's.
	save(alice.ID, aliceOld)
	tiedTo(bobs, bob.ID, "Alice's late save")
	save(alice.ID, uuid.Nil)
	tiedTo(bobs, bob.ID, "Alice's late save with no session")
	wantSentTo(t, repo, bob.ID, tok)
	wantSentTo(t, repo, alice.ID)
	// Her late delete is not hers to make.
	remove(alice.ID, aliceOld)
	tiedTo(bobs, bob.ID, "Alice's late delete")

	// Bob logs out, and Alice signs in again on the phone: a live session
	// takes the token, whoever had it.
	if err := refresh.RevokeByHash(ctx, []byte("late-bob-000000000000000000001")); err != nil {
		t.Fatal(err)
	}
	aliceNew := signIn(alice.ID, "late-alice-new-000000000000001")
	save(alice.ID, aliceNew)
	tiedTo(aliceNew, alice.ID, "Alice signed in again")
	// Bob's late save from his ended session leaves it with her.
	save(bob.ID, bobs)
	tiedTo(aliceNew, alice.ID, "Bob's late save")

	// A late delete from Alice's earlier session, or with no session, leaves
	// her new session's token; a delete from the new session deletes it.
	remove(alice.ID, aliceOld)
	tiedTo(aliceNew, alice.ID, "a late delete from her earlier session")
	remove(alice.ID, uuid.Nil)
	tiedTo(aliceNew, alice.ID, "a delete with no session")
	// A refresh moves the token on; a delete with the access token from
	// before the refresh is still the session's own.
	refreshed, err := refresh.Rotate(ctx, aliceNew, []byte("late-alice-refreshed-000000001"), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	tiedTo(refreshed, alice.ID, "after a refresh")
	remove(alice.ID, aliceNew)
	if got := pushOwner(t, pool, tok); got != "" {
		t.Fatal("a delete from the session the token is tied to kept it")
	}

	// Tied to a session that has ended, or to none, any delete of hers
	// deletes it.
	save(alice.ID, aliceOld)
	tiedTo(aliceOld, alice.ID, "saved by an ended session on no row")
	remove(alice.ID, uuid.Nil)
	if got := pushOwner(t, pool, tok); got != "" {
		t.Fatal("a delete kept a token tied to an ended session")
	}
	save(alice.ID, uuid.Nil)
	remove(alice.ID, aliceOld)
	if got := pushOwner(t, pool, tok); got != "" {
		t.Fatal("a delete kept a token tied to no session")
	}
}

// Change password and an admin's disable stop the pushes too.
func TestPushRepo_ChangePasswordAndDisableStopPushes(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	owner := mustCreateUser(t, users, "push-change@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPushRepo(pool)
	ctx := context.Background()
	now := time.Now()
	const phone, otherPhone = "ExponentPushToken[change-one]", "ExponentPushToken[change-two]"

	save := func(token string) uuid.UUID {
		t.Helper()
		session, err := refresh.Insert(ctx, owner.ID, []byte("push-change-"+token), now.Add(time.Hour), nil)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		if err := repo.SaveToken(ctx, owner.ID, session, token, "ios", now); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}
		return session
	}
	save(phone)
	save(otherPhone)
	wantSentTo(t, repo, owner.ID, otherPhone, phone)

	// Change password on the first phone: every session ends, the new one
	// included until the phone saves its token again with it.
	newSession := uuid.New()
	if err := users.ChangePassword(ctx, owner.ID, "new-bcrypt-hash", newSession,
		[]byte("push-change-new-session-000001"), now.Add(time.Hour), nil); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	wantSentTo(t, repo, owner.ID)
	if err := repo.SaveToken(ctx, owner.ID, newSession, phone, "ios", now); err != nil {
		t.Fatalf("SaveToken with the new session: %v", err)
	}
	wantSentTo(t, repo, owner.ID, phone)

	// An admin disables the account: nothing is sent, also for a token saved
	// after it (the disable and the end of every session are separate steps).
	disabledAt := time.Now()
	if err := postgres.NewAdminRepo(pool).SetDisabled(ctx, owner.ID, &disabledAt); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	wantSentTo(t, repo, owner.ID)
	if err := repo.SaveToken(ctx, owner.ID, newSession, phone, "ios", now); err != nil {
		t.Fatalf("SaveToken while disabled: %v", err)
	}
	wantSentTo(t, repo, owner.ID)
	if err := refresh.RevokeAllForUser(ctx, owner.ID); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}
	// Enabled again, the old sessions stay over.
	if err := postgres.NewAdminRepo(pool).SetDisabled(ctx, owner.ID, nil); err != nil {
		t.Fatalf("enable: %v", err)
	}
	wantSentTo(t, repo, owner.ID)
}

// A token saved while a refresh of its session is in flight ends up tied to
// the refresh token the session holds once the refresh commits, whichever of
// the two gets to the old refresh token first.
func TestPushRepo_SaveTokenAndARefreshAtOnce(t *testing.T) {
	pool := requireTestPool(t)
	owner := mustCreateUser(t, postgres.NewUserRepo(pool), "push-race@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPushRepo(pool)
	ctx := context.Background()
	now := time.Now()
	expires := now.Add(time.Hour)

	t.Run("the refresh first", func(t *testing.T) {
		const tok = "ExponentPushToken[race-refresh-first]"
		old, err := refresh.Insert(ctx, owner.ID, []byte("push-race-a-old-0000000000001"), expires, nil)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		// The refresh, stopped just before its commit: it holds the old row.
		rotating, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer rotating.Rollback(ctx) //nolint:errcheck // a no-op once committed
		var next uuid.UUID
		if err := rotating.QueryRow(ctx, `insert into refresh_tokens (user_id, token_hash, expires_at) values ($1, $2, $3) returning id`,
			owner.ID, []byte("push-race-a-new-0000000000001"), expires).Scan(&next); err != nil {
			t.Fatalf("in-flight insert: %v", err)
		}
		if _, err := rotating.Exec(ctx, `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2`, next, old); err != nil {
			t.Fatalf("in-flight rotate: %v", err)
		}
		if _, err := rotating.Exec(ctx, `update push_tokens set refresh_token_id = $1 where refresh_token_id = $2`, next, old); err != nil {
			t.Fatalf("in-flight move: %v", err)
		}

		// The phone saves its token with an access token naming the old
		// session: it waits for the refresh.
		done := make(chan error, 1)
		go func() { done <- repo.SaveToken(ctx, owner.ID, old, tok, "ios", now) }()
		waitForALockWait(t, pool)
		if err := rotating.Commit(ctx); err != nil {
			t.Fatalf("commit the refresh: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("SaveToken: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("SaveToken never finished")
		}
		if s, _ := pushSession(t, pool, tok); s == nil || *s != next {
			t.Fatalf("the token is tied to %v, want the refreshed session %v", s, next)
		}
		wantSentTo(t, repo, owner.ID, tok)
	})

	t.Run("the save first", func(t *testing.T) {
		const tok = "ExponentPushToken[race-save-first]"
		if _, err := pool.Exec(ctx, `delete from push_tokens`); err != nil {
			t.Fatal(err)
		}
		oldHash := []byte("push-race-b-old-0000000000001")
		old, err := refresh.Insert(ctx, owner.ID, oldHash, expires, nil)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		// The save, stopped just before its commit, as SaveToken leaves it:
		// the old refresh token held, the token tied to it.
		saving, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer saving.Rollback(ctx) //nolint:errcheck // a no-op once committed
		if _, err := saving.Exec(ctx, `select 1 from refresh_tokens where id = $1 for share`, old); err != nil {
			t.Fatalf("hold the session: %v", err)
		}
		if _, err := saving.Exec(ctx, `insert into push_tokens (user_id, token, platform, refresh_token_id) values ($1, $2, 'ios', $3)`,
			owner.ID, tok, old); err != nil {
			t.Fatalf("in-flight save: %v", err)
		}

		// The refresh waits for the save, then moves the saved token.
		type rotated struct {
			id  uuid.UUID
			err error
		}
		done := make(chan rotated, 1)
		go func() {
			id, err := refresh.Rotate(ctx, old, []byte("push-race-b-new-0000000000001"), expires)
			done <- rotated{id, err}
		}()
		waitForALockWait(t, pool)
		if err := saving.Commit(ctx); err != nil {
			t.Fatalf("commit the save: %v", err)
		}
		var got rotated
		select {
		case got = <-done:
			if got.err != nil {
				t.Fatalf("Rotate: %v", got.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Rotate never finished")
		}
		if s, _ := pushSession(t, pool, tok); s == nil || *s != got.id {
			t.Fatalf("the token is tied to %v, want the refreshed session %v", s, got.id)
		}
		wantSentTo(t, repo, owner.ID, tok)
	})
}

// A person signs in again on a phone while the session they left is still
// live (its log out never reached the server). A save from the newer session
// (its refresh token was made later) always takes the token; a late save from
// the older one does not take it back, so once the older session's log out
// lands, the phone still gets pushes. The rule is the person's own: another
// person's live session still takes the phone.
func TestPushRepo_ALateSaveFromAnOlderLiveSessionLeavesTheNewerOnesToken(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	owner := mustCreateUser(t, users, "late-live@example.com")
	other := mustCreateUser(t, users, "late-live-other@example.com")
	refresh := postgres.NewRefreshTokenRepo(pool)
	repo := postgres.NewPushRepo(pool)
	ctx := context.Background()
	now := time.Now()
	const tok = "ExponentPushToken[late-live]"
	// signIn makes a live session whose refresh token was made age ago.
	signIn := func(u uuid.UUID, hash string, age time.Duration) uuid.UUID {
		t.Helper()
		id, err := refresh.Insert(ctx, u, []byte(hash), now.Add(time.Hour), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `update refresh_tokens set created_at = now() - $2::interval where id = $1`,
			id, fmt.Sprintf("%d seconds", int(age.Seconds()))); err != nil {
			t.Fatal(err)
		}
		return id
	}
	save := func(u, session uuid.UUID) {
		t.Helper()
		if err := repo.SaveToken(ctx, u, session, tok, "ios", now); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}
	}
	tiedTo := func(want uuid.UUID, what string) {
		t.Helper()
		if s, _ := pushSession(t, pool, tok); s == nil || *s != want {
			t.Fatalf("%s: the token is tied to %v, want %v", what, s, want)
		}
	}

	older := signIn(owner.ID, "late-live-older-0000000000001", 2*time.Hour)
	newer := signIn(owner.ID, "late-live-newer-0000000000001", time.Hour)
	save(owner.ID, older)
	tiedTo(older, "the older session's save")
	save(owner.ID, newer)
	tiedTo(newer, "the newer session's save")
	save(owner.ID, older)
	tiedTo(newer, "a late save from the older live session")
	if err := refresh.RevokeByHash(ctx, []byte("late-live-older-0000000000001")); err != nil {
		t.Fatal(err)
	}
	wantSentTo(t, repo, owner.ID, tok)

	// Another person's live session takes it, even one made earlier.
	others := signIn(other.ID, "late-live-other-0000000000001", 3*time.Hour)
	save(other.ID, others)
	tiedTo(others, "another person's live session")
}
