package postgres_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// askedAgo moves the account's deletion ask to the given time ago, on the
// database's clock, as if it had been made then. The interval is in hours,
// which are exact; days follow the session's zone across a clock change.
func askedAgo(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, ago string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`update users set deletion_requested_at = now() - $2::interval where id = $1`, id, ago); err != nil {
		t.Fatalf("back-date the ask: %v", err)
	}
}

// TestAccountDeletion_OneGraceLine walks one account across the line: inside
// the grace period it is not due, a code can be issued and the ask can be
// cancelled; one minute past it, it is due, no code is issued and nothing
// cancels it. The end of the grace period the request answers is the stored
// ask plus user.DeletionGracePeriod.
func TestAccountDeletion_OneGraceLine(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(pool)
	purges := postgres.NewPurgeRepo(pool)
	resets := postgres.NewPasswordResetRepo(pool)
	u := mustCreateUser(t, users, "grace-line@example.com")

	asked, err := users.RequestDeletion(ctx, u.ID)
	if err != nil {
		t.Fatalf("RequestDeletion: %v", err)
	}
	var stored time.Time
	if err := pool.QueryRow(ctx, `select deletion_requested_at from users where id = $1`, u.ID).Scan(&stored); err != nil {
		t.Fatalf("read the ask: %v", err)
	}
	if want := stored.Add(user.DeletionGracePeriod); !asked.DeleteAfter.Equal(want) {
		t.Fatalf("DeleteAfter %s, want the ask plus the grace period %s", asked.DeleteAfter, want)
	}
	if asked.Zone != "" {
		t.Fatalf("zone %q for an account with none", asked.Zone)
	}
	if _, err := users.RequestDeletion(ctx, u.ID); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("asking twice: %v, want ErrAccountDisabled", err)
	}

	due := func(want bool, what string) {
		t.Helper()
		ids, err := purges.Due(ctx, 10)
		if err != nil {
			t.Fatalf("Due: %v", err)
		}
		if got := slices.Contains(ids, u.ID); got != want {
			t.Fatalf("%s: due %v, want %v", what, got, want)
		}
		got, err := users.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if !got.DeletionRequested || got.DeletionDue != want {
			t.Fatalf("%s: requested %v due %v, want true %v", what, got.DeletionRequested, got.DeletionDue, want)
		}
	}
	issue := func() bool {
		t.Helper()
		now := time.Now().UTC()
		stored, err := resets.Issue(ctx, u.ID, []byte("code-"+uuid.NewString()), now.Add(30*time.Minute), now, now.Add(-time.Minute), 5)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		_, _ = pool.Exec(ctx, `delete from password_resets where user_id = $1`, u.ID)
		return stored
	}

	askedAgo(t, pool, u.ID, "719 hours 59 minutes")
	due(false, "a minute inside")
	if !issue() {
		t.Fatal("a minute inside: no code issued")
	}
	if ok, err := users.CancelDeletion(ctx, u.ID); err != nil || !ok {
		t.Fatalf("a minute inside: CancelDeletion %v %v, want true", ok, err)
	}
	if ok, err := users.CancelDeletion(ctx, u.ID); err != nil || ok {
		t.Fatalf("nothing left to cancel: %v %v, want false", ok, err)
	}

	if _, err := users.RequestDeletion(ctx, u.ID); err != nil {
		t.Fatalf("ask again: %v", err)
	}
	askedAgo(t, pool, u.ID, "720 hours 1 minute")
	due(true, "a minute past")
	if issue() {
		t.Fatal("a minute past: a code was issued")
	}
	if ok, err := users.CancelDeletion(ctx, u.ID); err != nil || ok {
		t.Fatalf("a minute past: CancelDeletion %v %v, want false", ok, err)
	}
	due(true, "still past")
}

// TestAccountDeletion_RequestRefusesDisabledAndGone: a disabled account is
// not marked (and keeps its code), and an id with no account is ErrNotFound.
// A disabled account inside the grace period is not restored either.
func TestAccountDeletion_RequestRefusesDisabledAndGone(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(pool)
	u := mustCreateUser(t, users, "disabled-asks@example.com")
	if _, err := pool.Exec(ctx, `update users set disabled_at = now() where id = $1`, u.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into password_resets (user_id, code_hash, expires_at, attempts, created_at)
		values ($1, 'x', now() + interval '30 minutes', 0, now())`, u.ID); err != nil {
		t.Fatalf("seed a code: %v", err)
	}
	if _, err := users.RequestDeletion(ctx, u.ID); !errors.Is(err, user.ErrAccountDisabled) {
		t.Fatalf("disabled: %v, want ErrAccountDisabled", err)
	}
	var marked, codes int
	if err := pool.QueryRow(ctx, `select count(*) filter (where deletion_requested_at is not null),
		(select count(*) from password_resets where user_id = $1) from users where id = $1`, u.ID).Scan(&marked, &codes); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if marked != 0 || codes != 1 {
		t.Fatalf("disabled: marked %d codes %d, want 0 and 1 (nothing changed)", marked, codes)
	}
	if _, err := users.RequestDeletion(ctx, uuid.New()); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("no account: %v, want ErrNotFound", err)
	}

	if _, err := pool.Exec(ctx, `update users set deletion_requested_at = now() where id = $1`, u.ID); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if ok, err := users.CancelDeletion(ctx, u.ID); err != nil || ok {
		t.Fatalf("disabled inside the grace period: CancelDeletion %v %v, want false", ok, err)
	}
}
