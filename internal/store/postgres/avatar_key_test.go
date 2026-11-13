package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// TestMigrations_0019IsReversible walks from the latest version down to 18,
// stores profile photo links the way version 18 did, and goes up again.
// Going up turns each link into the key at its end, in the owner's own
// folder, and every other value into no photo; the check then refuses a key
// of any other shape. Going down renames the column back with the bare keys
// in it, and going up once more changes nothing.
func TestMigrations_0019IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 19 {
		t.Fatalf("latest migration is %d, want at least 19", latest)
	}
	column := func(pool *pgxpool.Pool, name string) bool {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from information_schema.columns where table_name = 'users' and column_name = $1`, name).Scan(&n); err != nil {
			t.Fatalf("column %s: %v", name, err)
		}
		return n == 1
	}
	shapeCheck := func(pool *pgxpool.Pool) bool {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from pg_constraint where conname = 'users_avatar_key_shape'`).Scan(&n); err != nil {
			t.Fatalf("constraint check: %v", err)
		}
		return n == 1
	}

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if !column(pool, "avatar_key") || column(pool, "avatar_url") || !shapeCheck(pool) {
			t.Fatal("expected avatar_key with its check, and no avatar_url, at the latest version")
		}
	})

	// At version 18 the column held whole links. Each person's row is
	// written by plain SQL: the user repository reads avatar_key, which
	// version 18 does not have.
	migrateGoTo(t, 18)
	type person struct {
		email string
		// stored is what version 18 holds, built from the person's own id
		// and the id of a; want is the key after going up ("" for none).
		stored func(own, a string) *string
		want   func(own string) string
	}
	str := func(s string) *string { return &s }
	people := []person{
		{"minio@example.com",
			func(own, _ string) *string {
				return str("http://127.0.0.1:9000/rndmroll-avatars/avatars/" + own + "/0123456789abcdef.jpg")
			},
			func(own string) string { return "avatars/" + own + "/0123456789abcdef.jpg" }},
		{"someone-elses-folder@example.com",
			func(_, a string) *string {
				return str("http://127.0.0.1:9000/rndmroll-avatars/avatars/" + a + "/fedcba9876543210.jpg")
			},
			func(string) string { return "" }},
		{"bucket-named-avatars@example.com",
			func(own, _ string) *string {
				return str("https://project.example.test/storage/v1/object/public/avatars/avatars/" + own + "/00112233445566ff.heic")
			},
			func(own string) string { return "avatars/" + own + "/00112233445566ff.heic" }},
		{"already-a-key@example.com",
			func(own, _ string) *string { return str("avatars/" + own + "/0123456789abcdef.webp") },
			func(own string) string { return "avatars/" + own + "/0123456789abcdef.webp" }},
		{"outside-link@example.com",
			func(string, string) *string { return str("https://evil.example.org/x.png") },
			func(string) string { return "" }},
		{"upper-case-hex@example.com",
			func(own, _ string) *string {
				return str("http://127.0.0.1:9000/rndmroll-avatars/avatars/" + own + "/0123456789ABCDEF.jpg")
			},
			func(string) string { return "" }},
		{"key-not-at-the-end@example.com",
			func(own, _ string) *string {
				return str("http://127.0.0.1:9000/rndmroll-avatars/avatars/" + own + "/0123456789abcdef.jpg?v=1")
			},
			func(string) string { return "" }},
		{"no-photo@example.com",
			func(string, string) *string { return nil },
			func(string) string { return "" }},
	}
	ids := make([]uuid.UUID, len(people))
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 18 || dirty {
			t.Fatalf("after goto 18: version %d dirty=%v", v, dirty)
		}
		if !column(pool, "avatar_url") || column(pool, "avatar_key") || shapeCheck(pool) {
			t.Fatal("goto 18 must leave avatar_url and no check")
		}
		for i, p := range people {
			if err := pool.QueryRow(ctx, `insert into users (email) values ($1) returning id`, p.email).Scan(&ids[i]); err != nil {
				t.Fatalf("insert %s: %v", p.email, err)
			}
		}
		for i, p := range people {
			if _, err := pool.Exec(ctx, `update users set avatar_url = $2 where id = $1`, ids[i], p.stored(ids[i].String(), ids[0].String())); err != nil {
				t.Fatalf("store the link of %s: %v", p.email, err)
			}
		}
	})

	// wantKeys expects col of every person's row to hold the key they should
	// end with, or null for none.
	wantKeys := func(pool *pgxpool.Pool, label, col string) {
		t.Helper()
		for i, p := range people {
			var got *string
			if err := pool.QueryRow(ctx, `select `+col+` from users where id = $1`, ids[i]).Scan(&got); err != nil {
				t.Fatalf("%s: read %s: %v", label, p.email, err)
			}
			want := p.want(ids[i].String())
			if (want == "" && got != nil) || (want != "" && (got == nil || *got != want)) {
				t.Errorf("%s: %s holds %v, want %q", label, p.email, deref(got), want)
			}
		}
	}

	migrateGoTo(t, 19)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 19 || dirty {
			t.Fatalf("after goto 19: version %d dirty=%v", v, dirty)
		}
		if !column(pool, "avatar_key") || column(pool, "avatar_url") || !shapeCheck(pool) {
			t.Fatal("goto 19 must rename the column and add its check")
		}
		wantKeys(pool, "after goto 19", "avatar_key")

		// The check refuses every key that is not the person's own, and lets
		// the repository name the field.
		ownerID := ids[len(ids)-1]
		owner := ownerID.String()
		for name, bad := range map[string]string{
			"another person's folder": "avatars/" + ids[0].String() + "/0123456789abcdef.jpg",
			"a whole link":            "https://cdn.example.test/avatars/" + owner + "/0123456789abcdef.jpg",
			"a 15 hex name":           "avatars/" + owner + "/0123456789abcde.jpg",
			"upper case hex":          "avatars/" + owner + "/0123456789ABCDEF.jpg",
			"a dot dot segment":       "avatars/" + owner + "/../" + ids[0].String() + "/0123456789abcdef.jpg",
			"the entry folder":        "entries/" + owner + "/0123456789abcdef.jpg",
			"an extension over ten":   "avatars/" + owner + "/0123456789abcdef.abcdefghijk",
			"empty":                   "",
		} {
			var pgErr *pgconn.PgError
			_, err := pool.Exec(ctx, `update users set avatar_key = $2 where id = $1`, ownerID, bad)
			if !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_avatar_key_shape" {
				t.Errorf("%s: got %v, want a users_avatar_key_shape violation", name, err)
			}
			var fieldErr *user.FieldError
			_, err = postgres.NewUserRepo(pool).UpdateProfile(ctx, ownerID, user.ProfilePatch{AvatarKey: &bad})
			if !errors.As(err, &fieldErr) || fieldErr.Field != "avatar_key" {
				t.Errorf("%s: the repository answered %v, want a field error for avatar_key", name, err)
			}
		}
		own := "avatars/" + owner + "/0123456789abcdef.jpg"
		u, err := postgres.NewUserRepo(pool).UpdateProfile(ctx, ownerID, user.ProfilePatch{AvatarKey: &own})
		if err != nil || u.AvatarKey == nil || *u.AvatarKey != own {
			t.Fatalf("the person's own key: %v, error %v", u, err)
		}
		if _, err := pool.Exec(ctx, `update users set avatar_key = null where id = $1`, ownerID); err != nil {
			t.Fatalf("clear the key again: %v", err)
		}
	})

	// Down: the column is avatar_url again, with the bare keys in it (the
	// old links cannot be made again), and no check.
	migrateGoTo(t, 18)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 18 || dirty {
			t.Fatalf("after going back to 18: version %d dirty=%v", v, dirty)
		}
		if !column(pool, "avatar_url") || column(pool, "avatar_key") || shapeCheck(pool) {
			t.Fatal("going down must rename the column back and drop its check")
		}
		wantKeys(pool, "after going back to 18", "avatar_url")
	})

	// Up again: the bare keys stay as they are.
	migrateGoTo(t, 19)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		wantKeys(pool, "after going up again", "avatar_key")
		if _, err := pool.Exec(ctx, "truncate users cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
