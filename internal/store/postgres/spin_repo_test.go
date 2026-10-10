package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// newSpinEnv builds a suite environment over the test database. Every table
// the spin slice owns references users, so the cleanup in requireTestPool
// (truncate users cascade) clears them all.
func newSpinEnv(t *testing.T) (*spintest.Env, *pgxpool.Pool) {
	t.Helper()
	pool := requireTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
		t.Fatalf("entry truncate: %v", err)
	}
	repo := postgres.NewSpinRepo(pool)
	users := postgres.NewUserRepo(pool)
	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	rnd := spintest.NewQueueRand(20261007)

	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", strings.Fields(sql)[0], err)
		}
	}
	return &spintest.Env{
		Service: spin.NewService(repo, clk, rnd),
		Clock:   clk,
		Rand:    rnd,
		NewUser: func(t *testing.T) string {
			t.Helper()
			u, err := users.Create(ctx, fmt.Sprintf("spin-%s@example.com", uuid.NewString()), nil, false, nil)
			if err != nil {
				t.Fatalf("create user: %v", err)
			}
			return u.ID.String()
		},
		State: func(t *testing.T, userID string) spin.UserState {
			t.Helper()
			var st spin.UserState
			var date string
			err := pool.QueryRow(ctx, `
				select seeded_at, seeded_local_date::text, seeded_tz, weekly_goal, bonus_held, wheel_version
				from user_spin_state where user_id = $1`, userID).
				Scan(&st.SeededAt, &date, &st.SeededTZ, &st.WeeklyGoal, &st.BonusHeld, &st.WheelVersion)
			if err != nil {
				t.Fatalf("read state: %v", err)
			}
			if st.SeededLocalDate, err = spin.ParseDate(date); err != nil {
				t.Fatal(err)
			}
			return st
		},
		Counts: func(t *testing.T, userID string) (int, int) {
			t.Helper()
			var spins, cats int
			if err := pool.QueryRow(ctx, `select count(*) from spins where user_id = $1`, userID).Scan(&spins); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `select count(*) from wheel_categories where user_id = $1`, userID).Scan(&cats); err != nil {
				t.Fatal(err)
			}
			return spins, cats
		},
		SetSpinResult: func(t *testing.T, userID, spinID string, result spin.SpinResult) {
			t.Helper()
			exec(t, `update spins set result = $3 where id = $1 and user_id = $2`, spinID, userID, string(result))
		},
		ForceZone: func(t *testing.T, userID, name string) {
			t.Helper()
			exec(t, `insert into user_time_zones (user_id, tz_name) values ($1, $2)
				on conflict (user_id) do update set tz_name = excluded.tz_name`, userID, name)
		},
		FillArchived: func(t *testing.T, userID string, unreferenced, referenced int) {
			t.Helper()
			if unreferenced > 0 {
				exec(t, `
					insert into wheel_categories (user_id, name, weight, position, archived_at)
					select $1, 'Filler', 1, 1000 + g, timestamptz '2020-01-01 12:00+00'
					from generate_series(1, $2::int) g`, userID, unreferenced)
			}
			if referenced > 0 {
				exec(t, `
					with cats as (
						insert into wheel_categories (user_id, name, weight, position, archived_at)
						select $1, 'Filler', 1, 2000 + g, timestamptz '2020-01-01 12:00+00'
						from generate_series(1, $2::int) g
						returning id, position
					)
					insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
					                   winner_index, spun_at, window_opens_at, window_closes_at)
					select $1, date '2019-01-01' + (position - 2001), 'UTC', 'daily', 0, id, 'Filler',
					       jsonb_build_array(jsonb_build_object('id', id, 'name', 'Filler', 'weight', 1)), 0,
					       timestamptz '2019-01-01 12:00+00', timestamptz '2019-01-01 12:00+00', timestamptz '2019-01-01 12:00+00'
					from cats`, userID, referenced)
			}
		},
	}, pool
}

// TestSpinService_Postgres runs the shared service suite, the same one the
// in-memory repository passes, against the real database. It includes the
// concurrency cases (20 spins, 5 bonus calls, 4 posted days, 10 first calls,
// 5 saves).
func TestSpinService_Postgres(t *testing.T) {
	spintest.RunServiceSuite(t, func(t *testing.T) *spintest.Env {
		env, _ := newSpinEnv(t)
		return env
	})
}

// --- the database refuses what the service never sends ---

func wantConstraint(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want a database error naming %s", err, constraint)
	}
	if pgErr.ConstraintName != constraint {
		t.Fatalf("constraint = %q (%s), want %q", pgErr.ConstraintName, pgErr.Message, constraint)
	}
}

func TestSpinTables_RefuseWhatTheServiceNeverSends(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	a := env.NewUser(t)
	b := env.NewUser(t)
	if _, err := env.Service.SetTimeZone(ctx, a, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Service.SetTimeZone(ctx, b, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	var aCat, bCat string
	if err := pool.QueryRow(ctx, `select id::text from wheel_categories where user_id = $1 order by position limit 1`, a).Scan(&aCat); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select id::text from wheel_categories where user_id = $1 order by position limit 1`, b).Scan(&bCat); err != nil {
		t.Fatal(err)
	}

	insertSpin := func(user, category, kind string, seq int, date string) error {
		_, err := pool.Exec(ctx, `
			insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
			                   winner_index, spun_at, window_opens_at, window_closes_at)
			values ($1, $2::date, 'UTC', $3, $4, $5, 'Meal', '[]'::jsonb, 0, now(), now(), now())`,
			user, date, kind, seq, category)
		return err
	}

	if err := insertSpin(a, aCat, "daily", 0, "2026-10-07"); err != nil {
		t.Fatalf("a valid daily spin: %v", err)
	}
	// A second bonus row for one date.
	if err := insertSpin(a, aCat, "bonus", 1, "2026-10-07"); err != nil {
		t.Fatalf("a valid bonus spin: %v", err)
	}
	wantConstraint(t, insertSpin(a, aCat, "bonus", 2, "2026-10-07"), "spins_one_bonus_per_day")
	// The same key twice.
	wantConstraint(t, insertSpin(a, aCat, "daily", 0, "2026-10-07"), "spins_user_date_seq_unique")
	// A spin on another person's category.
	wantConstraint(t, insertSpin(a, bCat, "daily", 0, "2026-10-08"), "spins_category_fk")
	// kind and seq must agree.
	wantConstraint(t, insertSpin(a, aCat, "daily", 1, "2026-10-09"), "spins_kind_seq_check")
	wantConstraint(t, insertSpin(a, aCat, "bonus", 0, "2026-10-09"), "spins_kind_seq_check")
	// A kind, a result and a source outside their lists.
	if _, err := pool.Exec(ctx, `update spins set result = 'maybe' where user_id = $1`, a); err == nil {
		t.Error("a result of 'maybe' was accepted")
	}
	if _, err := pool.Exec(ctx, `insert into posted_days (user_id, local_date, source) values ($1, '2026-10-01', 'robot')`, a); err == nil {
		t.Error("a posted-day source of 'robot' was accepted")
	}

	checks := []struct {
		name string
		sql  string
		args []any
	}{
		{"weight 0", `insert into wheel_categories (user_id, name, weight, position) values ($1, 'X', 0, 50)`, []any{a}},
		{"weight 11", `insert into wheel_categories (user_id, name, weight, position) values ($1, 'X', 11, 50)`, []any{a}},
		{"empty name", `insert into wheel_categories (user_id, name, weight, position) values ($1, '   ', 1, 50)`, []any{a}},
		{"name of 25", `insert into wheel_categories (user_id, name, weight, position) values ($1, repeat('n', 25), 1, 50)`, []any{a}},
		{"bonus_held 2", `update user_spin_state set bonus_held = 2 where user_id = $1`, []any{a}},
		{"weekly_goal 0", `update user_spin_state set weekly_goal = 0 where user_id = $1`, []any{a}},
		{"weekly_goal 8", `update user_spin_state set weekly_goal = 8 where user_id = $1`, []any{a}},
		{"empty zone", `update user_time_zones set tz_name = '' where user_id = $1`, []any{a}},
		{"zone of 65", `update user_time_zones set tz_name = repeat('Z', 65) where user_id = $1`, []any{a}},
	}
	for _, c := range checks {
		if _, err := pool.Exec(ctx, c.sql, c.args...); err == nil {
			t.Errorf("%s was accepted", c.name)
		}
	}
	// 24 characters and weight 10 are fine.
	if _, err := pool.Exec(ctx, `insert into wheel_categories (user_id, name, weight, position) values ($1, repeat('n', 24), 10, 50)`, a); err != nil {
		t.Errorf("a 24 character name with weight 10: %v", err)
	}
}

func TestSpinTables_DeletingAnAccountClearsThemAll(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	env.Clock.Set(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC))
	if _, err := env.Service.SetTimeZone(ctx, u, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Service.MarkPosted(ctx, u, mustDate(t, "2026-10-08"), spin.SourceTest); err != nil {
		t.Fatal(err)
	}
	env.Clock.Set(time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)) // 21:00 on the 8th, New York
	if _, err := env.Service.Spin(ctx, u, nil); err != nil {
		t.Fatal(err)
	}
	// A goal week row, written directly.
	if _, err := pool.Exec(ctx, `insert into goal_weeks (user_id, week_start, goal, goal_met_at, bonus_granted)
		values ($1, '2026-10-05', 4, now(), true)`, u); err != nil {
		t.Fatal(err)
	}
	tables := []string{"user_time_zones", "user_spin_state", "wheel_categories", "spins", "posted_days", "goal_weeks"}
	for _, tbl := range tables {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from `+tbl+` where user_id = $1`, u).Scan(&n); err != nil || n == 0 {
			t.Fatalf("%s: %d rows before the delete (err %v), want some", tbl, n, err)
		}
	}
	if _, err := pool.Exec(ctx, `delete from users where id = $1`, u); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	for _, tbl := range tables {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from `+tbl).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s still holds %d rows after the account went", tbl, n)
		}
	}
}

func mustDate(t *testing.T, s string) spin.Date {
	t.Helper()
	d, err := spin.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// A second InsertSpin for one key returns the existing row, and the
// transaction can still be used (a unique violation would have aborted it).
func TestSpinRepo_InsertSpinConflictReturnsTheExistingRowAndKeepsTheTransaction(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	repo := postgres.NewSpinRepo(pool)
	seed := spin.Seed{At: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), LocalDate: mustDate(t, "2026-10-07"), TZName: "UTC", Goal: 4}

	err := repo.InUserTx(ctx, u, seed, func(tx spin.Tx) error {
		cats, err := tx.LiveCategories()
		if err != nil || len(cats) != 6 {
			return fmt.Errorf("seeded categories: %d, %v", len(cats), err)
		}
		wheel := []spin.WheelEntry{{ID: cats[0].ID, Name: cats[0].Name, Weight: cats[0].Weight}}
		at := time.Date(2026, 10, 7, 20, 30, 0, 123456000, time.UTC)
		s := spin.Spin{
			LocalDate: mustDate(t, "2026-10-07"), TZName: "UTC", Kind: spin.KindDaily, Seq: 0,
			CategoryID: cats[0].ID, CategoryName: cats[0].Name, Wheel: wheel, WinnerIndex: 0,
			SpunAt: at, WindowOpensAt: at, WindowClosesAt: at,
		}
		first, inserted, err := tx.InsertSpin(s)
		if err != nil || !inserted {
			return fmt.Errorf("first insert: inserted %v err %v", inserted, err)
		}
		s.CategoryID, s.CategoryName = cats[1].ID, cats[1].Name
		again, inserted, err := tx.InsertSpin(s)
		if err != nil {
			return fmt.Errorf("second insert: %w", err)
		}
		if inserted || again.ID != first.ID || again.CategoryID != cats[0].ID {
			return fmt.Errorf("second insert: inserted %v id %s, want the first row %s back", inserted, again.ID, first.ID)
		}
		if !again.SpunAt.Equal(at) {
			return fmt.Errorf("spun_at = %v, want %v (microsecond precision)", again.SpunAt, at)
		}
		// The transaction is still usable.
		latest, err := tx.LatestSpinOfDate(mustDate(t, "2026-10-07"))
		if err != nil || latest == nil || latest.ID != first.ID {
			return fmt.Errorf("read after the conflict: %v %v", latest, err)
		}
		if len(latest.Wheel) != 1 || latest.Wheel[0].ID != cats[0].ID {
			return fmt.Errorf("stored wheel = %+v", latest.Wheel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from spins where user_id = $1`, u).Scan(&n); err != nil || n != 1 {
		t.Fatalf("spin rows = %d (err %v), want 1", n, err)
	}
}

func TestSpinRepo_ACallbackErrorRollsBackEverythingIncludingTheFirstSeed(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	repo := postgres.NewSpinRepo(pool)
	seed := spin.Seed{At: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), LocalDate: mustDate(t, "2026-10-07"), TZName: "UTC", Goal: 4}
	boom := errors.New("boom")

	err := repo.InUserTx(ctx, u, seed, func(tx spin.Tx) error {
		if _, err := tx.InsertPostedDay(spin.PostedDay{Date: mustDate(t, "2026-10-07"), Source: spin.SourceTest}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want boom", err)
	}
	for _, tbl := range []string{"user_spin_state", "wheel_categories", "posted_days"} {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from `+tbl+` where user_id = $1`, u).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s holds %d rows after a rolled back first call (err %v)", tbl, n, err)
		}
	}
	// A call that succeeds afterwards seeds normally.
	if _, err := env.Service.Wheel(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, cats := env.Counts(t, u); cats != 6 {
		t.Fatalf("category rows = %d, want 6", cats)
	}
}

// A holder that keeps a person's lock longer than 5 seconds makes the next
// caller fail with ErrBusy (the HTTP layer answers 503), and a retry works
// once the holder lets go: nothing is left pinned.
func TestSpinRepo_LockTimeoutIsErrBusyAndFreesTheCaller(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	env.Clock.Set(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	if _, err := env.Service.SetTimeZone(ctx, u, "America/New_York"); err != nil {
		t.Fatal(err)
	}

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = holder.Rollback(ctx)
		}
	}()
	var id uuid.UUID
	if err := holder.QueryRow(ctx, `select user_id from user_spin_state where user_id = $1 for update`, u).Scan(&id); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	_, err = env.Service.Today(ctx, u)
	waited := time.Since(started)
	if !errors.Is(err, spin.ErrBusy) {
		t.Fatalf("Today with the lock held: %v, want ErrBusy", err)
	}
	if waited < 4*time.Second || waited > 9*time.Second {
		t.Errorf("waited %v for the lock, want about 5 seconds", waited)
	}

	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	released = true
	if _, err := env.Service.Today(ctx, u); err != nil {
		t.Fatalf("Today after the holder let go: %v", err)
	}
}
