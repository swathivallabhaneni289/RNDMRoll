package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// TestMigrations_0022IsReversible: at the latest version admin_accounts holds
// the admin accounts the server stored. Going down to 21 drops the table,
// and going up again adds it back, empty, until the next start fills it.
func TestMigrations_0022IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 22 {
		t.Fatalf("latest migration is %d, want at least 22", latest)
	}
	count := func(pool *pgxpool.Pool, q string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	tables := func(pool *pgxpool.Pool) int {
		t.Helper()
		return count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'admin_accounts'`)
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if tables(pool) != 1 {
			t.Fatal("no admin_accounts table at the latest version")
		}
		boss, err := postgres.NewUserRepo(pool).CreateComplete(ctx, newAccount("m22_boss@example.com", "m22_boss"))
		if err != nil {
			t.Fatalf("CreateComplete: %v", err)
		}
		if err := postgres.NewAdminRepo(pool).SetAdminAccounts(ctx, []uuid.UUID{boss.ID}); err != nil {
			t.Fatalf("SetAdminAccounts: %v", err)
		}
		if n := count(pool, `select count(*) from admin_accounts`); n != 1 {
			t.Fatalf("%d admin accounts stored, want 1", n)
		}
	})

	migrateGoTo(t, 21)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 21 || dirty {
			t.Fatalf("after goto 21: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 0 {
			t.Fatal("goto 21 leaves the admin_accounts table")
		}
		if n := count(pool, `select count(*) from users where username = 'm22_boss'`); n != 1 {
			t.Fatalf("goto 21 keeps %d of the admin's account, want 1", n)
		}
	})

	migrateGoTo(t, 22)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 22 || dirty {
			t.Fatalf("after goto 22: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 1 || count(pool, `select count(*) from admin_accounts`) != 0 {
			t.Fatal("goto 22 does not add admin_accounts back empty")
		}
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

// TestAdminRepo_SetAdminAccounts_HidesExactlyThoseAccounts: what the server
// stores at start is exactly who is hidden. An admin account is in no one's
// search, username lookup, people list or suggestions, while everyone else
// is; storing the same ids again changes nothing; another list replaces it
// whole; none (nil or empty) shows everyone again; and the row goes with
// the account.
func TestAdminRepo_SetAdminAccounts_HidesExactlyThoseAccounts(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(pool)
	socialRepo := postgres.NewSocialRepo(pool)
	admins := postgres.NewAdminRepo(pool)
	mk := func(username string) *user.User {
		t.Helper()
		u, err := users.CreateComplete(ctx, newAccount(username+"@example.com", username))
		if err != nil {
			t.Fatalf("CreateComplete(%s): %v", username, err)
		}
		return u
	}
	viewer, boss, cara := mk("aa_viewer"), mk("aa_boss"), mk("aa_cara")
	set := func(ids []uuid.UUID) {
		t.Helper()
		if err := admins.SetAdminAccounts(ctx, ids); err != nil {
			t.Fatalf("SetAdminAccounts(%v): %v", ids, err)
		}
	}
	stored := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from admin_accounts`).Scan(&n); err != nil {
			t.Fatalf("count admin_accounts: %v", err)
		}
		return n
	}
	// shown says, for each of boss and cara, whether the viewer meets them in
	// all four reads, and fails when the reads disagree.
	shown := func(what string) (bossShown, caraShown bool) {
		t.Helper()
		hits, err := socialRepo.Search(ctx, viewer.ID, "aa_", 10)
		if err != nil {
			t.Fatalf("%s: Search: %v", what, err)
		}
		people, err := socialRepo.People(ctx, []uuid.UUID{boss.ID, cara.ID})
		if err != nil {
			t.Fatalf("%s: People: %v", what, err)
		}
		suggested, err := socialRepo.Suggestions(ctx, viewer.ID, 10)
		if err != nil {
			t.Fatalf("%s: Suggestions: %v", what, err)
		}
		one := func(u *user.User) bool {
			t.Helper()
			inSearch, inSuggestions := false, false
			for _, h := range hits {
				inSearch = inSearch || h.Person.ID == u.ID
			}
			for _, s := range suggested {
				inSuggestions = inSuggestions || s.Person.ID == u.ID
			}
			_, inPeople := people[u.ID]
			_, found, err := socialRepo.FindByUsername(ctx, *u.Username)
			if err != nil && !errors.Is(err, social.ErrNotFound) {
				t.Fatalf("%s: FindByUsername(%s): %v", what, *u.Username, err)
			}
			if inSearch != inPeople || inSearch != found || inSearch != inSuggestions {
				t.Fatalf("%s: %s is in search %v, people %v, lookup %v, suggestions %v: the reads disagree",
					what, *u.Username, inSearch, inPeople, found, inSuggestions)
			}
			return inSearch
		}
		return one(boss), one(cara)
	}

	if b, c := shown("before"); !b || !c {
		t.Fatalf("before: boss shown %v, cara shown %v, want both", b, c)
	}
	set([]uuid.UUID{boss.ID})
	if b, c := shown("boss is the admin"); b || !c {
		t.Fatalf("boss is the admin: boss shown %v (want false), cara shown %v (want true)", b, c)
	}
	set([]uuid.UUID{boss.ID})
	if n := stored(); n != 1 {
		t.Fatalf("the same ids again: %d rows, want 1", n)
	}
	set([]uuid.UUID{cara.ID})
	if b, c := shown("cara instead"); !b || c {
		t.Fatalf("cara instead: boss shown %v (want true), cara shown %v (want false)", b, c)
	}
	set(nil)
	if b, c := shown("no ADMIN_EMAILS"); !b || !c || stored() != 0 {
		t.Fatalf("no ADMIN_EMAILS: boss shown %v, cara shown %v, %d rows; want both shown and none stored", b, c, stored())
	}
	set([]uuid.UUID{boss.ID, cara.ID})
	set([]uuid.UUID{})
	if n := stored(); n != 0 {
		t.Fatalf("an empty list: %d rows, want 0", n)
	}
	set([]uuid.UUID{boss.ID})
	if _, err := pool.Exec(ctx, `delete from users where id = $1`, boss.ID); err != nil {
		t.Fatalf("delete boss: %v", err)
	}
	if n := stored(); n != 0 {
		t.Fatalf("after the admin's account is deleted: %d rows, want 0", n)
	}
}
