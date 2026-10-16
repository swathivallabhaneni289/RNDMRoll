package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

var followFeedTables = []string{"follows", "blocks", "notification_settings", "post_reactions", "post_comments", "notification_seen"}

// TestMigrations_0008IsReversible walks from the latest version down to 7
// and back up to 8. Going down drops the six Phase 5 tables and leaves users
// and entries alone; going up makes them again, empty.
func TestMigrations_0008IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	latest := latestMigration(t)
	if latest < 8 {
		t.Fatalf("latest migration is %d, want at least 8", latest)
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
		return count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = any($1)`, followFeedTables)
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if n := tables(pool); n != len(followFeedTables) {
			t.Fatalf("%d of the Phase 5 tables at the latest version", n)
		}
		repo := postgres.NewUserRepo(pool)
		a := mustCreateUser(t, repo, "feed-migration-a@example.com")
		b := mustCreateUser(t, repo, "feed-migration-b@example.com")
		stmts := []string{
			`insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0)`,
			`insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
				select $1, '2026-10-08'::date, 'America/New_York', 'daily', 0, id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
				from wheel_categories where user_id = $1`,
			`insert into entries (user_id, spin_id, local_date, title, stars, created_at, updated_at)
				select $1, id, local_date, 'Dune', 7, now(), now() from spins where user_id = $1`,
			`insert into follows (follower_id, followee_id, status, via, requested_at, accepted_at) values ($2, $1, 'accepted', 'direct', now(), now())`,
			`insert into blocks (blocker_id, blocked_id, created_at) values ($1, $2, now())`,
			`insert into notification_settings (user_id, reactions, updated_at) values ($1, false, now())`,
			`insert into post_reactions (entry_id, user_id, reaction, created_at, updated_at) select id, $2, 'fire', now(), now() from entries where user_id = $1`,
			`insert into post_comments (entry_id, user_id, body, created_at) select id, $2, 'Nice', now() from entries where user_id = $1`,
			`insert into notification_seen (user_id, seen_at) values ($1, now())`,
		}
		args := func(s string) []any {
			if strings.Contains(s, "$2") {
				return []any{a.ID, b.ID}
			}
			return []any{a.ID}
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s, args(s)...); err != nil {
				t.Fatalf("insert (%s): %v", strings.Fields(s)[2], err)
			}
		}
		// The database refuses what the service never writes.
		refused := []string{
			`insert into follows (follower_id, followee_id, status, requested_at) values ($1, $1, 'pending', now())`,
			`insert into follows (follower_id, followee_id, status, requested_at) values ($1, $2, 'accepted', now())`,
			`insert into follows (follower_id, followee_id, status, requested_at) values ($2, $1, 'pending', now())`,
			`insert into blocks (blocker_id, blocked_id, created_at) values ($1, $1, now())`,
			`insert into post_reactions (entry_id, user_id, reaction, created_at, updated_at) select id, $1, 'clap', now(), now() from entries`,
			`insert into post_comments (entry_id, user_id, body, created_at) select id, $1, '   ', now() from entries`,
			`insert into post_comments (entry_id, user_id, body, created_at) select id, $1, repeat('a', 141), now() from entries`,
		}
		for _, s := range refused {
			if _, err := pool.Exec(ctx, s, args(s)...); err == nil {
				t.Fatalf("the database accepted: %s", s)
			}
		}
		// The reaction names are the same list as the entries' own.
		var mine, theirs string
		if err := pool.QueryRow(ctx, `select pg_get_constraintdef(oid) from pg_constraint where conname = 'post_reactions_name'`).Scan(&mine); err != nil {
			t.Fatalf("post_reactions_name: %v", err)
		}
		if err := pool.QueryRow(ctx, `select pg_get_constraintdef(oid) from pg_constraint where conname = 'entries_reaction_check'`).Scan(&theirs); err != nil {
			t.Fatalf("entries_reaction_check: %v", err)
		}
		for _, name := range []string{"love", "fire", "laughing", "wow", "sad", "sleepy"} {
			if !strings.Contains(mine, "'"+name+"'") || !strings.Contains(theirs, "'"+name+"'") {
				t.Fatalf("reaction lists differ: %s / %s", mine, theirs)
			}
		}
		if strings.Count(mine, "'") != strings.Count(theirs, "'") {
			t.Fatalf("reaction lists differ in length: %s / %s", mine, theirs)
		}
	})

	migrateGoTo(t, 7)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 7 || dirty {
			t.Fatalf("after goto 7: version %d dirty=%v", v, dirty)
		}
		if n := tables(pool); n != 0 {
			t.Fatalf("goto 7 must drop the Phase 5 tables, %d remain", n)
		}
		if countUsers(t, pool) != 2 || count(pool, `select count(*) from entries`) != 1 {
			t.Fatal("goto 7 must leave users and entries alone")
		}
	})

	migrateGoTo(t, 8)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 8 || dirty {
			t.Fatalf("after goto 8: version %d dirty=%v", v, dirty)
		}
		if n := tables(pool); n != len(followFeedTables) {
			t.Fatalf("goto 8 must make the Phase 5 tables again, found %d", n)
		}
		for _, tbl := range followFeedTables {
			if n := count(pool, `select count(*) from `+tbl); n != 0 {
				t.Errorf("%s holds %d rows after going back up", tbl, n)
			}
		}
		// Deleting an account removes its rows (cascade).
		repo := postgres.NewUserRepo(pool)
		c := mustCreateUser(t, repo, "feed-migration-c@example.com")
		d := mustCreateUser(t, repo, "feed-migration-d@example.com")
		if _, err := pool.Exec(ctx, `insert into follows (follower_id, followee_id, status, requested_at) values ($1, $2, 'pending', now())`, c.ID, d.ID); err != nil {
			t.Fatalf("insert a request: %v", err)
		}
		if _, err := pool.Exec(ctx, `delete from users where id = $1`, d.ID); err != nil {
			t.Fatalf("delete an account: %v", err)
		}
		if n := count(pool, `select count(*) from follows`); n != 0 {
			t.Fatalf("follows left after the account went: %d", n)
		}
	})
}
