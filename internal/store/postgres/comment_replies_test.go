package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// TestMigrations_0009IsReversible: at the latest version a reply points at
// its top-level comment and goes with it (cascade); going down to 8 drops
// parent_id and keeps the comments; going up to 9 adds it again, null.
func TestMigrations_0009IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 9 {
		t.Fatalf("latest migration is %d, want at least 9", latest)
	}
	count := func(pool *pgxpool.Pool, q string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	hasParent := func(pool *pgxpool.Pool) bool {
		return count(pool, `select count(*) from information_schema.columns
			where table_schema = 'public' and table_name = 'post_comments' and column_name = 'parent_id'`) == 1
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if !hasParent(pool) {
			t.Fatal("parent_id is missing at the latest version")
		}
		repo := postgres.NewUserRepo(pool)
		a := mustCreateUser(t, repo, "reply-migration-a@example.com")
		b := mustCreateUser(t, repo, "reply-migration-b@example.com")
		stmts := []string{
			`insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0)`,
			`insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
				select $1, '2026-10-08'::date, 'America/New_York', 'daily', 0, id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
				from wheel_categories where user_id = $1`,
			`insert into entries (user_id, spin_id, local_date, title, stars, created_at, updated_at)
				select $1, id, local_date, 'Dune', 7, now(), now() from spins where user_id = $1`,
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s, a.ID); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
		var top, keep string
		if err := pool.QueryRow(ctx, `insert into post_comments (entry_id, user_id, body, created_at)
			select id, $1, 'Top', now() from entries returning id::text`, b.ID).Scan(&top); err != nil {
			t.Fatalf("top comment: %v", err)
		}
		if err := pool.QueryRow(ctx, `insert into post_comments (entry_id, user_id, body, created_at)
			select id, $1, 'Kept', now() from entries returning id::text`, b.ID).Scan(&keep); err != nil {
			t.Fatalf("second comment: %v", err)
		}
		for _, parent := range []string{top, top, keep} {
			if _, err := pool.Exec(ctx, `insert into post_comments (entry_id, user_id, body, created_at, parent_id)
				select id, $1, 'Reply', now(), $2::uuid from entries`, a.ID, parent); err != nil {
				t.Fatalf("reply: %v", err)
			}
		}
		if _, err := pool.Exec(ctx, `insert into post_comments (entry_id, user_id, body, created_at, parent_id)
			select id, $1, 'Orphan', now(), gen_random_uuid() from entries`, a.ID); err == nil {
			t.Fatal("the database accepted a reply to no comment")
		}
		if _, err := pool.Exec(ctx, `delete from post_comments where id = $1::uuid`, top); err != nil {
			t.Fatalf("delete top: %v", err)
		}
		if n := count(pool, `select count(*) from post_comments`); n != 2 {
			t.Fatalf("deleting a comment must take its two replies: %d rows left", n)
		}
	})

	migrateGoTo(t, 8)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 8 || dirty {
			t.Fatalf("after goto 8: version %d dirty=%v", v, dirty)
		}
		if hasParent(pool) {
			t.Fatal("goto 8 must drop parent_id")
		}
		if n := count(pool, `select count(*) from post_comments`); n != 2 {
			t.Fatalf("goto 8 keeps the comments: %d", n)
		}
	})

	migrateGoTo(t, 9)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 9 || dirty {
			t.Fatalf("after goto 9: version %d dirty=%v", v, dirty)
		}
		if !hasParent(pool) || count(pool, `select count(*) from post_comments where parent_id is not null`) != 0 {
			t.Fatal("goto 9 adds parent_id again, null on every row")
		}
	})
}
