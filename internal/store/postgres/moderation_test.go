package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// TestMigrations_0020IsReversible: at the latest version a post and a comment
// can be removed, the reports are indexed by what they are about, and the
// decision log keeps its events (a removal with the reason it was given, a
// report reason's key or none), whose item id outlives the post or comment.
// Going down to 19 with all of that stored drops exactly what 0020 added and
// keeps the posts, comments and reports; going up again adds it back, with
// nothing removed and an empty log.
func TestMigrations_0020IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 20 {
		t.Fatalf("latest migration is %d, want at least 20", latest)
	}
	count := func(pool *pgxpool.Pool, q string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	indexes := []string{"entries_removed_idx", "post_comments_removed_idx", "post_reports_entry_idx",
		"comment_reports_comment_idx", "moderation_events_owner_idx"}
	// added counts what 0020 adds that is there: the two columns, the five
	// indexes and the table.
	added := func(pool *pgxpool.Pool) int {
		t.Helper()
		return count(pool, `select count(*) from information_schema.columns
			where table_schema = 'public' and table_name in ('entries', 'post_comments') and column_name = 'removed_at'`) +
			count(pool, `select count(*) from pg_indexes where schemaname = 'public' and indexname = any($1)`, indexes) +
			count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'moderation_events'`)
	}
	const all = 2 + 5 + 1
	// kept counts the posts, comments and reports.
	kept := func(pool *pgxpool.Pool) int {
		t.Helper()
		return count(pool, `select (select count(*) from entries) + (select count(*) from post_comments)
			+ (select count(*) from post_reports) + (select count(*) from comment_reports)`)
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if n := added(pool); n != all {
			t.Fatalf("%d of the %d things 0020 adds are there at the latest version", n, all)
		}
		repo := postgres.NewUserRepo(pool)
		a := mustCreateUser(t, repo, "moderation-migration-a@example.com")
		b := mustCreateUser(t, repo, "moderation-migration-b@example.com")
		for _, s := range []string{
			`insert into wheel_categories (user_id, name, weight, position) values ($1, 'Meal', 1, 0)`,
			`insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
				select $1, '2026-10-08'::date, 'America/New_York', 'daily', 0, id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
				from wheel_categories where user_id = $1`,
			`insert into entries (user_id, spin_id, local_date, title, stars, created_at, updated_at, removed_at)
				select $1, id, local_date, 'Dune', 7, now(), now(), now() from spins where user_id = $1`,
		} {
			if _, err := pool.Exec(ctx, s, a.ID); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
		var entry, rude, gone string
		if err := pool.QueryRow(ctx, `select id::text from entries`).Scan(&entry); err != nil {
			t.Fatalf("entry: %v", err)
		}
		if err := pool.QueryRow(ctx, `insert into post_comments (entry_id, user_id, body, created_at, removed_at)
			values ($1, $2, 'Rude', now(), now()) returning id::text`, entry, b.ID).Scan(&rude); err != nil {
			t.Fatalf("removed comment: %v", err)
		}
		if err := pool.QueryRow(ctx, `insert into post_comments (entry_id, user_id, body, created_at)
			values ($1, $2, 'Gone soon', now()) returning id::text`, entry, b.ID).Scan(&gone); err != nil {
			t.Fatalf("second comment: %v", err)
		}
		for _, s := range []struct {
			q    string
			args []any
		}{
			{`insert into post_reports (reporter_id, entry_id, reason, created_at, owner_id, title)
				values ($1, $2, 'spam', now(), $3, 'Dune')`, []any{b.ID, entry, a.ID}},
			{`insert into comment_reports (reporter_id, comment_id, reason, created_at, owner_id, body)
				values ($1, $2, 'harassment', now(), $3, 'Rude')`, []any{a.ID, rude, b.ID}},
			{`insert into moderation_events (owner_id, item, action, entry_id, item_id, snapshot, reasons, reason, at)
				values ($1, 'post', 'removed', $2, $2, 'Dune', '{spam}', 'hate', now())`, []any{a.ID, entry}},
			{`insert into moderation_events (owner_id, item, action, entry_id, comment_id, item_id, snapshot, at)
				values ($1, 'comment', 'restored', $2, $3, $3, 'Gone soon', now())`, []any{b.ID, entry, gone}},
			{`insert into moderation_events (owner_id, item, action, item_id, at)
				values ($1, 'account', 'disabled', $1, now())`, []any{b.ID}},
			{`insert into moderation_events (owner_id, item, action, entry_id, item_id, snapshot, at)
				values ($1, 'post', 'hidden', $2, $2, 'Dune', now())`, []any{a.ID, entry}},
		} {
			if _, err := pool.Exec(ctx, s.q, s.args...); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
		for _, bad := range []string{
			`insert into moderation_events (owner_id, item, action, item_id, at) values ($1, 'person', 'removed', $1, now())`,
			`insert into moderation_events (owner_id, item, action, item_id, at) values ($1, 'post', 'deleted', $1, now())`,
			// The reason the admin gives is a report reason's key, or none.
			`insert into moderation_events (owner_id, item, action, item_id, reason, at) values ($1, 'post', 'removed', $1, 'rude', now())`,
		} {
			if _, err := pool.Exec(ctx, bad, a.ID); err == nil {
				t.Fatalf("the log took an event its checks refuse: %s", bad)
			}
		}
		// The removal keeps the reason it was given, and every other row has
		// none.
		if n := count(pool, `select count(*) from moderation_events where action = 'removed' and reason = 'hate'`); n != 1 {
			t.Fatalf("%d removals keep the reason hate, want 1", n)
		}
		if n := count(pool, `select count(*) from moderation_events where action <> 'removed' and reason = ''`); n != 3 {
			t.Fatalf("%d other rows have no reason, want all 3", n)
		}
		// A deleted comment leaves its event, which still names it by item id.
		if _, err := pool.Exec(ctx, `delete from post_comments where id = $1`, gone); err != nil {
			t.Fatalf("delete the comment: %v", err)
		}
		if n := count(pool, `select count(*) from moderation_events
			where item_id = $1 and comment_id is null and entry_id = $2`, gone, entry); n != 1 {
			t.Fatalf("the deleted comment's event: %d rows, want one with no comment id", n)
		}
		if n := kept(pool); n != 4 {
			t.Fatalf("%d posts, comments and reports stored, want 4", n)
		}
	})

	migrateGoTo(t, 19)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 19 || dirty {
			t.Fatalf("after goto 19: version %d dirty=%v", v, dirty)
		}
		if n := added(pool); n != 0 {
			t.Fatalf("goto 19 leaves %d of the things 0020 added", n)
		}
		if n := kept(pool); n != 4 {
			t.Fatalf("goto 19 keeps the posts, comments and reports: %d, want 4", n)
		}
	})

	migrateGoTo(t, 20)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 20 || dirty {
			t.Fatalf("after goto 20: version %d dirty=%v", v, dirty)
		}
		if n := added(pool); n != all {
			t.Fatalf("goto 20 adds back %d of the %d things", n, all)
		}
		if n := count(pool, `select (select count(*) from entries where removed_at is not null)
			+ (select count(*) from post_comments where removed_at is not null)
			+ (select count(*) from moderation_events)`); n != 0 {
			t.Fatalf("after going up again %d things are removed or logged, want none", n)
		}
		if n := kept(pool); n != 4 {
			t.Fatalf("goto 20 keeps the posts, comments and reports: %d, want 4", n)
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}
