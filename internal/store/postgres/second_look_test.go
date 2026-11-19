package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// TestMigrations_0021IsReversible: at the latest version a removal can have
// one second look, whose words are 1 to 500 characters after trimming, with
// at most 3 photo keys and an answer only with its time, and the log takes a
// Keep it removed (kept). Going down to 20 with all of that stored drops the
// requests and every kept row, keeps the other decisions, and the log
// refuses kept again; going up again adds the table back, empty, and takes
// kept again.
func TestMigrations_0021IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 21 {
		t.Fatalf("latest migration is %d, want at least 21", latest)
	}
	count := func(pool *pgxpool.Pool, q string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	tables := func(pool *pgxpool.Pool) int {
		t.Helper()
		return count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'second_looks'`)
	}
	kept := func(pool *pgxpool.Pool, owner any) error {
		_, err := pool.Exec(ctx, `insert into moderation_events (owner_id, item, action, item_id, at) values ($1, 'post', 'kept', $1, now())`, owner)
		return err
	}

	var owner string
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if tables(pool) != 1 {
			t.Fatal("second_looks is not there at the latest version")
		}
		a := mustCreateUser(t, postgres.NewUserRepo(pool), "second-look-migration@example.com")
		owner = a.ID.String()
		var removal, other string
		if err := pool.QueryRow(ctx, `insert into moderation_events (owner_id, item, action, item_id, snapshot, at)
			values ($1, 'post', 'removed', $1, 'Dune', now()) returning id::text`, a.ID).Scan(&removal); err != nil {
			t.Fatalf("removal: %v", err)
		}
		if err := pool.QueryRow(ctx, `insert into moderation_events (owner_id, item, action, item_id, at)
			values ($1, 'comment', 'removed', $1, now()) returning id::text`, a.ID).Scan(&other); err != nil {
			t.Fatalf("second removal: %v", err)
		}
		if err := kept(pool, a.ID); err != nil {
			t.Fatalf("the log refuses kept at the latest version: %v", err)
		}
		ask := `insert into second_looks (owner_id, removal_id, item, item_id, body, photo_keys, at, decision, decided_at)
			values ($1, $2, 'post', $1, $3, $4, now(), $5, $6)`
		if _, err := pool.Exec(ctx, ask, a.ID, removal, "  It was a joke.  ", []string{"entries/a/1.jpg", "entries/a/2.jpg", "entries/a/3.jpg"}, nil, nil); err != nil {
			t.Fatalf("a second look: %v", err)
		}
		for name, args := range map[string][]any{
			"a second one for the same removal": {a.ID, removal, "Again", []string{}, nil, nil},
			"no words":                          {a.ID, other, "   ", []string{}, nil, nil},
			"501 characters":                    {a.ID, other, strings.Repeat("x", 501), []string{}, nil, nil},
			"4 photos":                          {a.ID, other, "Why", []string{"k1", "k2", "k3", "k4"}, nil, nil},
			"an answer with no time":            {a.ID, other, "Why", []string{}, "kept", nil},
			"a time with no answer":             {a.ID, other, "Why", []string{}, nil, "2026-11-19T00:00:00Z"},
			"an answer it does not know":        {a.ID, other, "Why", []string{}, "deleted", "2026-11-19T00:00:00Z"},
		} {
			if _, err := pool.Exec(ctx, ask, args...); err == nil {
				t.Fatalf("second_looks took %s", name)
			}
		}
		if _, err := pool.Exec(ctx, ask, a.ID, other, strings.Repeat("é", 500), []string{}, "kept", "2026-11-19T00:00:00Z"); err != nil {
			t.Fatalf("500 characters, answered: %v", err)
		}
		if n := count(pool, `select count(*) from second_looks`); n != 2 {
			t.Fatalf("%d second looks stored, want 2", n)
		}
	})

	migrateGoTo(t, 20)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 20 || dirty {
			t.Fatalf("after goto 20: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 0 {
			t.Fatal("goto 20 leaves second_looks")
		}
		if n := count(pool, `select count(*) from moderation_events where action = 'kept'`); n != 0 {
			t.Fatalf("goto 20 leaves %d kept rows", n)
		}
		if n := count(pool, `select count(*) from moderation_events where action = 'removed'`); n != 2 {
			t.Fatalf("goto 20 keeps %d removals, want 2", n)
		}
		if err := kept(pool, owner); err == nil {
			t.Fatal("at 20 the log takes kept")
		}
	})

	migrateGoTo(t, 21)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 21 || dirty {
			t.Fatalf("after goto 21: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 1 || count(pool, `select count(*) from second_looks`) != 0 {
			t.Fatal("goto 21 does not add second_looks back empty")
		}
		if err := kept(pool, owner); err != nil {
			t.Fatalf("after going up again the log refuses kept: %v", err)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// removedPostOf stores a post of owner and removes it with the admin's
// Remove, and returns the post's id and the removal's.
func removedPostOf(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, title string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var entry uuid.UUID
	if err := pool.QueryRow(ctx, `with c as (
			insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0) returning id),
		s as (
			insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
			select $1, '2026-10-08'::date, 'America/New_York', 'daily', 0, c.id, 'Meal', '[]'::jsonb, 0, now(), now(), now() from c
			returning id, local_date)
		insert into entries (user_id, spin_id, local_date, title, stars, created_at, updated_at)
		select $1, s.id, s.local_date, $2, 7, now(), now() from s returning id`, owner, title).Scan(&entry); err != nil {
		t.Fatalf("a post: %v", err)
	}
	now := time.Now()
	if err := postgres.NewAdminRepo(pool).RemoveItem(ctx, admin.KindPost, entry, owner, "spam", now, now); err != nil {
		t.Fatalf("remove: %v", err)
	}
	return entry, newestRemovalOf(t, pool, entry)
}

// newestRemovalOf is the id of the newest removal of item in the log.
func newestRemovalOf(t *testing.T, pool *pgxpool.Pool, item uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `select id from moderation_events
		where item_id = $1 and action = 'removed' order by created_at desc limit 1`, item).Scan(&id); err != nil {
		t.Fatalf("the removal: %v", err)
	}
	return id
}

// lookAnswer is the answer stored on the second look about removal: "open"
// while there is none.
func lookAnswer(t *testing.T, pool *pgxpool.Pool, removal uuid.UUID) string {
	t.Helper()
	var answer string
	if err := pool.QueryRow(context.Background(), `select coalesce(decision, 'open') || case when (decision is null) = (decided_at is null) then '' else ' (time wrong)' end
		from second_looks where removal_id = $1`, removal).Scan(&answer); err != nil {
		t.Fatalf("the second look: %v", err)
	}
	return answer
}

// askedAbout stores an open second look about removal, as the app's form
// would.
func askedAbout(t *testing.T, pool *pgxpool.Pool, owner, removal, item uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `insert into second_looks (owner_id, removal_id, item, item_id, body, at)
		values ($1, $2, 'post', $3, 'It was a joke.', now())`, owner, removal, item); err != nil {
		t.Fatalf("ask: %v", err)
	}
}

// Keep it removed answers the open second look about the newest removal,
// logs kept on the owner's clock with the title, and changes nothing more on
// a second click; with no open request, for someone else, once it is back
// or past the 30 days it changes nothing. Bring it back answers the request
// too, also one a Keep answered, and a removal after that has no request
// until one is asked.
func TestAdminRepo_KeepRemovedAndBringItBackAnswerTheSecondLook(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	repo := postgres.NewAdminRepo(pool)
	users := postgres.NewUserRepo(pool)
	a := mustCreateUser(t, users, "keep-owner@example.com")
	b := mustCreateUser(t, users, "keep-other@example.com")
	post, removal := removedPostOf(t, pool, a.ID, "Dune")
	loggedAt := time.Date(2026, 11, 19, 9, 30, 0, 0, time.UTC)
	kept := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from moderation_events where action = 'kept' and owner_id = $1
			and item = 'post' and item_id = $2 and entry_id = $2 and snapshot = 'Dune' and at = $3`, a.ID, post, loggedAt).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, loggedAt); !errors.Is(err, admin.ErrNoRequest) {
		t.Fatalf("Keep with nothing asked: %v, want ErrNoRequest", err)
	}
	askedAbout(t, pool, a.ID, removal, post)
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, b.ID, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep naming someone else: %v, want ErrNotFound", err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindComment, post, a.ID, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep of a comment that is a post: %v, want ErrNotFound", err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, loggedAt); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	if got := lookAnswer(t, pool, removal); got != "kept" || kept() != 1 {
		t.Fatalf("after Keep the request is %q with %d kept rows, want kept and 1", got, kept())
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, loggedAt); !errors.Is(err, admin.ErrAlreadyDone) || kept() != 1 {
		t.Fatalf("Keep again: %v with %d kept rows, want ErrAlreadyDone and 1", err, kept())
	}
	if err := repo.RestoreItem(ctx, admin.KindPost, post, a.ID, loggedAt); err != nil {
		t.Fatalf("Bring it back after Keep: %v", err)
	}
	if got := lookAnswer(t, pool, removal); got != "restored" {
		t.Fatalf("after Bring it back the request is %q, want restored", got)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep once it is back: %v, want ErrNotFound", err)
	}

	// Removed again: a new removal with no request of its own.
	now := time.Now()
	if err := repo.RemoveItem(ctx, admin.KindPost, post, a.ID, "", now, now); err != nil {
		t.Fatalf("remove again: %v", err)
	}
	again := newestRemovalOf(t, pool, post)
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, loggedAt); !errors.Is(err, admin.ErrNoRequest) {
		t.Fatalf("Keep on the new removal with nothing asked: %v, want ErrNoRequest", err)
	}
	askedAbout(t, pool, a.ID, again, post)
	if err := repo.RestoreItem(ctx, admin.KindPost, post, a.ID, loggedAt); err != nil {
		t.Fatalf("Bring it back on an open request: %v", err)
	}
	if lookAnswer(t, pool, again) != "restored" || lookAnswer(t, pool, removal) != "restored" || kept() != 1 {
		t.Fatalf("after the second Bring it back: %q and %q, %d kept rows", lookAnswer(t, pool, again), lookAnswer(t, pool, removal), kept())
	}

	// Past the 30 days nothing is answered: only the purge touches it.
	late, lateRemoval := removedPostOf(t, pool, b.ID, "Old")
	askedAbout(t, pool, b.ID, lateRemoval, late)
	if _, err := pool.Exec(ctx, `update entries set removed_at = now() - interval '30 days' where id = $1`, late); err != nil {
		t.Fatal(err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, late, b.ID, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep past the 30 days: %v, want ErrNotFound", err)
	}
	if got := lookAnswer(t, pool, lateRemoval); got != "open" {
		t.Fatalf("past the 30 days the request is %q, want open", got)
	}
}
