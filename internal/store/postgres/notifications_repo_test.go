package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// The bell's calls, counted in round trips (roundTrips). The dot (Unread)
// builds the same list, so it costs the same as the list. The HTTP layer
// adds one to each for the account lookup (RequireUser). With nobody in the
// list, People sends no query.
func TestNotificationsRepo_RoundTripsOfTheBell(t *testing.T) {
	requireTestPool(t)
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	trips := &roundTrips{}
	cfg.ConnConfig.Tracer = trips
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFixed(time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	spins := spin.NewService(postgres.NewSpinRepo(pool), clk, spintest.NewQueueRand(20261118))
	soc := social.NewService(postgres.NewSocialRepo(pool), clk)
	bell := notifications.NewService(postgres.NewNotificationsRepo(pool), soc, spins)

	u, err := postgres.NewUserRepo(pool).Create(ctx, "bell-trips-"+uuid.NewString()+"@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := spins.SetTimeZone(ctx, u.ID.String(), "America/New_York"); err != nil {
		t.Fatal(err)
	}
	list := func() {
		if _, err := bell.List(ctx, u.ID); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	dot := func() {
		if _, err := bell.Unread(ctx, u.ID); err != nil {
			t.Fatalf("Unread: %v", err)
		}
	}
	want := func(label string, got, want int) {
		t.Helper()
		t.Logf("%s: %d round trips", label, got)
		if got != want {
			t.Errorf("%s: %d round trips, want %d", label, got, want)
		}
	}

	// A first list makes the spin state row (SpinOpenedTonight, as Today
	// does), so the counts are of a list after that.
	list()

	// Both switches on (the defaults): the zone, the switches, Comments,
	// Replies, Follows, Requests, Accepted, the spin window (8: the zone,
	// begin, the state row, the lock, the zone again, the date's daily spin,
	// the last daily spin, commit), Bonuses, and the admin's decisions with
	// the seen time in one batch.
	want("List, both switches on", trips.of(list), 17)
	want("Unread, both switches on", trips.of(dot), 17)

	// Both switches off, with an empty decision log: the zone, the
	// switches, Follows, Requests, Accepted, and the decisions with the seen
	// time in one batch.
	off := false
	if _, err := soc.UpdateSettings(ctx, u.ID, social.SettingsPatch{Comments: &off, Milestones: &off}); err != nil {
		t.Fatal(err)
	}
	want("List, both switches off", trips.of(list), 6)
	want("Unread, both switches off", trips.of(dot), 6)
}
