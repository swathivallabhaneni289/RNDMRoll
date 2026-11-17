package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// rowsOf counts the account's rows in table.
func rowsOf(t *testing.T, pool *pgxpool.Pool, table string, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from `+table+` where user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// signedIn gives the account two sessions and a push token on the second.
func signedIn(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, tag string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	var session uuid.UUID
	for i := range 2 {
		id, err := postgres.NewRefreshTokenRepo(pool).Insert(ctx, owner, []byte(fmt.Sprintf("set-pw-%s-%d-0000000000000000", tag, i)), now.Add(time.Hour), nil)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		session = id
	}
	if err := postgres.NewPushRepo(pool).SaveToken(ctx, owner, session, "ExponentPushToken[set-pw-"+tag+"]", "ios", now); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
}

// askedForACode gives the account a Forgot password code.
func askedForACode(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, tag string) {
	t.Helper()
	now := time.Now()
	stored, err := postgres.NewPasswordResetRepo(pool).Issue(context.Background(), owner,
		[]byte("set-pw-"+tag+"-code-0000000000000"), now.Add(30*time.Minute), now, now.Add(-time.Minute), 5)
	if err != nil || !stored {
		t.Fatalf("Issue: stored %v, err %v", stored, err)
	}
}

// `make password` stores the hash, ends every session the account had,
// deletes its push tokens and a code asked for before, and leaves every
// other account as it was.
func TestUserRepo_SetPassword_EndsEverySessionAndTheCode(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	ctx := context.Background()
	owner, err := users.CreateComplete(ctx, newAccount("set-pw@example.com", "set_pw"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.CreateComplete(ctx, newAccount("set-pw-other@example.com", "set_pw_other"))
	if err != nil {
		t.Fatal(err)
	}
	for id, tag := range map[uuid.UUID]string{owner.ID: "owner", other.ID: "other"} {
		signedIn(t, pool, id, tag)
		askedForACode(t, pool, id, tag)
	}

	if err := users.SetPassword(ctx, owner.ID, "terminal-bcrypt-hash"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if got := storedPasswordHash(t, pool, owner.ID); got != "terminal-bcrypt-hash" {
		t.Fatalf("stored hash = %q, want the new one", got)
	}
	if n := activeTokenCount(t, pool, owner.ID); n != 0 {
		t.Errorf("%d sessions still live, want none", n)
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

// Like change password, it never sets the password of an account an admin
// disabled or whose owner asked for it to be deleted, and changes nothing
// then: the code asked for inside the grace period stays. An account that is
// gone is ErrNotFound.
func TestUserRepo_SetPassword_RefusesADisabledOrDeletedAccount(t *testing.T) {
	pool := requireTestPool(t)
	users := postgres.NewUserRepo(pool)
	ctx := context.Background()

	disabled, err := users.CreateComplete(ctx, newAccount("set-pw-disabled@example.com", "set_pw_disabled"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	if err := postgres.NewAdminRepo(pool).SetDisabled(ctx, disabled.ID, &at, at); err != nil {
		t.Fatal(err)
	}
	if err := users.SetPassword(ctx, disabled.ID, "terminal-bcrypt-hash"); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("SetPassword on a disabled account: %v, want ErrAccountDisabled", err)
	}
	if got := storedPasswordHash(t, pool, disabled.ID); got != "bcrypt-hash" {
		t.Errorf("a disabled account's hash changed to %q", got)
	}

	leaving, err := users.CreateComplete(ctx, newAccount("set-pw-leaving@example.com", "set_pw_leaving"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.RequestDeletion(ctx, leaving.ID); err != nil {
		t.Fatal(err)
	}
	askedForACode(t, pool, leaving.ID, "leaving")
	if err := users.SetPassword(ctx, leaving.ID, "terminal-bcrypt-hash"); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("SetPassword on an account set to be deleted: %v, want ErrAccountDisabled", err)
	}
	if got := storedPasswordHash(t, pool, leaving.ID); got != "bcrypt-hash" {
		t.Errorf("the hash changed to %q", got)
	}
	if n := rowsOf(t, pool, "password_resets", leaving.ID); n != 1 {
		t.Errorf("%d codes left, want the 1 there was", n)
	}

	if err := users.SetPassword(ctx, uuid.New(), "terminal-bcrypt-hash"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("SetPassword for an unknown account: %v, want ErrNotFound", err)
	}
}
