package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// ADM-03: a person's page lists at most 200 of their reports, open ones
// first, newest first within each. 201 reports, the five newest closed: the
// 196 open ones come first, then the newest closed ones, and the one left
// out is the oldest closed report, not an open one.
func TestSocialCoverage_AdminReportsListsAtMost200OpenOnesFirst(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
		t.Fatal(err)
	}
	var person uuid.UUID
	if err := pool.QueryRow(ctx, `insert into users (email, username, name)
		values ('sr_target@example.com', 'sr_target', 'Ria Target') returning id`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	// 201 reporters, report i made at base plus i minutes.
	base := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `insert into users (email, username, name)
		select 'sr_rep_' || i || '@example.com', 'sr_rep_' || i, 'Rep ' || i from generate_series(0, 200) i`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into user_reports (reporter_id, reported_id, reason, created_at)
		select u.id, $1, 'spam', $2::timestamptz + substring(u.username from 8)::int * interval '1 minute'
		from users u where u.username like 'sr\_rep\_%'`, person, base); err != nil {
		t.Fatal(err)
	}
	tag, err := pool.Exec(ctx, `update user_reports set resolved_at = $2, resolution = 'dismissed'
		where reported_id = $1 and created_at >= $3`, person, base.Add(300*time.Minute), base.Add(196*time.Minute))
	if err != nil || tag.RowsAffected() != 5 {
		t.Fatalf("close the five newest: %d rows, %v", tag.RowsAffected(), err)
	}

	got, err := postgres.NewAdminRepo(pool).Reports(ctx, person)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 200 {
		t.Fatalf("%d reports, want at most 200", len(got))
	}
	for i, r := range got {
		open := i < 196
		want := base.Add(time.Duration(195-i) * time.Minute)
		if !open {
			want = base.Add(time.Duration(200-(i-196)) * time.Minute)
		}
		if (r.ResolvedAt == nil) != open || !r.CreatedAt.Equal(want) || r.Kind != "person" {
			t.Fatalf("report %d: open %v made %v kind %s, want open %v made %v", i, r.ResolvedAt == nil, r.CreatedAt, r.Kind, open, want)
		}
	}
}
