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

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// TestEntryService_Postgres runs the shared entry suite, the one MemRepo also
// passes, against the real database. It includes the races (20 posts with one
// list, 20 with different lists, a post against a skip).
func TestEntryService_Postgres(t *testing.T) {
	spintest.RunEntrySuite(t, func(t *testing.T) *spintest.EntryEnv {
		env, pool := newSpinEnv(t)
		return &spintest.EntryEnv{Env: env, Repo: postgres.NewSpinRepo(pool)}
	})
}

// --- helpers ---

// newKey returns a photo key shaped like a real one for person u.
func newKey(u string) string {
	return "entries/" + u + "/" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16] + ".jpg"
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// openNewYorkDay gives person u a zone and the daily spin of 2026-10-08 (the
// clock is set to 9 PM that evening, New York).
func openNewYorkDay(t *testing.T, env *spintest.Env, u string) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.Service.SetTimeZone(ctx, u, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	env.Clock.Set(time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC))
	if _, err := env.Service.Spin(ctx, u, nil); err != nil {
		t.Fatalf("Spin: %v", err)
	}
}

func entryInput(u string, n int) spin.EntryInput {
	photos := make([]spin.NewPhoto, n)
	for i := range photos {
		photos[i] = spin.NewPhoto{Key: newKey(u), Source: spin.PhotoSourceCamera}
	}
	return spin.EntryInput{Title: "Dune", Stars: 7, Photos: photos}
}

// --- fault injection ---

var errInjected = errors.New("injected failure")

// faultRepo hands the service a Tx whose chosen method fails. With after set,
// the real write is made first and the failure comes right after it, so the
// test proves a written row is undone, not only that a later row is missing.
type faultRepo struct {
	spin.Repository
	method string
	after  bool
	hits   int
}

func (r *faultRepo) InUserTx(ctx context.Context, userID string, seed spin.Seed, fn func(spin.Tx) error) error {
	return r.Repository.InUserTx(ctx, userID, seed, func(tx spin.Tx) error {
		return fn(&faultTx{Tx: tx, r: r})
	})
}

type faultTx struct {
	spin.Tx
	r *faultRepo
}

// fail runs the real write (when the fault is set to come after it) and
// reports whether the call should fail now.
func (t *faultTx) fail(method string, write func() error) error {
	if t.r.method != method {
		return write()
	}
	t.r.hits++
	if t.r.after {
		if err := write(); err != nil {
			return err
		}
	}
	return errInjected
}

func (t *faultTx) InsertPhotos(entryID string, photos []spin.NewStoredPhoto, now time.Time) error {
	return t.fail("InsertPhotos", func() error { return t.Tx.InsertPhotos(entryID, photos, now) })
}

func (t *faultTx) SettleSpin(spinID string, result spin.SpinResult, at time.Time) error {
	return t.fail("SettleSpin", func() error { return t.Tx.SettleSpin(spinID, result, at) })
}

func (t *faultTx) InsertPostedDay(p spin.PostedDay) (bool, error) {
	if t.r.method != "InsertPostedDay" {
		return t.Tx.InsertPostedDay(p)
	}
	inserted := false
	err := t.fail("InsertPostedDay", func() (err error) {
		inserted, err = t.Tx.InsertPostedDay(p)
		return err
	})
	return inserted, err
}

func TestEntryPost_AFailureInsideThePostLeavesNothingBehind(t *testing.T) {
	for _, method := range []string{"InsertPhotos", "SettleSpin", "InsertPostedDay"} {
		for _, after := range []bool{false, true} {
			when := "instead of the write"
			if after {
				when = "right after the write"
			}
			t.Run(method+" fails "+when, func(t *testing.T) {
				env, pool := newSpinEnv(t)
				ctx := context.Background()
				u := env.NewUser(t)
				openNewYorkDay(t, env, u)
				day := mustDate(t, "2026-10-08")

				fault := &faultRepo{Repository: postgres.NewSpinRepo(pool), method: method, after: after}
				faulty := spin.NewService(fault, env.Clock, env.Rand)
				in := entryInput(u, 3)

				_, err := faulty.PostEntry(ctx, u, day, 0, in)
				if !errors.Is(err, errInjected) {
					t.Fatalf("PostEntry error = %v, want the injected failure", err)
				}
				if fault.hits != 1 {
					t.Fatalf("the fault was reached %d times, want 1", fault.hits)
				}
				for _, tbl := range []string{"entries", "entry_photos", "posted_days", "goal_weeks"} {
					if n := countRows(t, pool, `select count(*) from `+tbl+` where user_id = $1`, u); n != 0 {
						t.Errorf("%s holds %d rows after a failed post, want 0", tbl, n)
					}
				}
				var result string
				var settled *time.Time
				if err := pool.QueryRow(ctx, `select result, settled_at from spins where user_id = $1`, u).Scan(&result, &settled); err != nil {
					t.Fatal(err)
				}
				if result != "open" || settled != nil {
					t.Errorf("spin result = %q settled_at = %v after a failed post, want open and null", result, settled)
				}

				// Nothing is stuck: the same post goes through afterwards.
				out, err := env.Service.PostEntry(ctx, u, day, 0, in)
				if err != nil || out.Already || len(out.Entry.Photos) != 3 {
					t.Fatalf("post after the failure: already %v photos %d err %v", out.Already, len(out.Entry.Photos), err)
				}
			})
		}
	}
}

// --- the database refuses what the service never sends ---

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func TestEntryTables_RefuseWhatTheServiceNeverSends(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	a, b := env.NewUser(t), env.NewUser(t)
	for _, u := range []string{a, b} {
		if _, err := env.Service.SetTimeZone(ctx, u, "America/New_York"); err != nil {
			t.Fatal(err)
		}
	}
	category := func(u string) string {
		var id string
		if err := pool.QueryRow(ctx, `select id::text from wheel_categories where user_id = $1 order by position limit 1`, u).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	catA, catB := category(a), category(b)

	// freshSpin makes one more daily spin on its own date, so each refusal
	// below would be accepted if its constraint were missing.
	days := 0
	freshSpin := func(user, cat string) (spinID, day string) {
		t.Helper()
		days++
		day = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days).Format("2006-01-02")
		err := pool.QueryRow(ctx, `
			insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
			                   winner_index, spun_at, window_opens_at, window_closes_at)
			values ($1, $2::date, 'UTC', 'daily', 0, $3, 'Meal', '[]'::jsonb, 0, now(), now(), now())
			returning id::text`, user, day, cat).Scan(&spinID)
		if err != nil {
			t.Fatalf("insert a spin: %v", err)
		}
		return spinID, day
	}
	insertEntry := func(user, spinID, day, title string, stars int, reaction, comment *string) error {
		_, err := pool.Exec(ctx, `
			insert into entries (user_id, spin_id, local_date, title, stars, reaction, comment, created_at, updated_at)
			values ($1, $2, $3::date, $4, $5, $6, $7, now(), now())`,
			user, spinID, day, title, stars, reaction, comment)
		return err
	}
	text := func(s string) *string { return &s }

	// A good entry, and its boundaries, are accepted.
	spinA, dayA := freshSpin(a, catA)
	if err := insertEntry(a, spinA, dayA, "Dune", 7, nil, nil); err != nil {
		t.Fatalf("a valid entry: %v", err)
	}
	sp, d := freshSpin(a, catA)
	if err := insertEntry(a, sp, d, strings.Repeat("t", 80), 10, text("sleepy"), text(strings.Repeat("c", 140))); err != nil {
		t.Fatalf("an entry at every limit: %v", err)
	}
	sp, d = freshSpin(a, catA)
	if err := insertEntry(a, sp, d, "x", 1, text("love"), nil); err != nil {
		t.Fatalf("an entry with one star: %v", err)
	}

	checks := []struct {
		name       string
		title      string
		stars      int
		reaction   *string
		comment    *string
		constraint string
	}{
		{"an empty title", "", 5, nil, nil, "entries_title_len"},
		{"a title of spaces", "    ", 5, nil, nil, "entries_title_len"},
		{"a title of 81 characters", strings.Repeat("t", 81), 5, nil, nil, "entries_title_len"},
		{"0 stars", "Dune", 0, nil, nil, "entries_stars_range"},
		{"11 stars", "Dune", 11, nil, nil, "entries_stars_range"},
		{"the reaction thumbsdown", "Dune", 5, text("thumbsdown"), nil, "entries_reaction_check"},
		{"the older reaction clap", "Dune", 5, text("clap"), nil, "entries_reaction_check"},
		{"a comment of 141 characters", "Dune", 5, nil, text(strings.Repeat("c", 141)), "entries_comment_len"},
	}
	for _, c := range checks {
		sp, d := freshSpin(a, catA)
		err := insertEntry(a, sp, d, c.title, c.stars, c.reaction, c.comment)
		if err == nil {
			t.Errorf("%s was accepted", c.name)
			continue
		}
		wantConstraint(t, err, c.constraint)
	}

	// A second entry for one person and date (another spin of the same date).
	otherSpin, _ := freshSpin(a, catA)
	wantConstraint(t, insertEntry(a, otherSpin, dayA, "Again", 5, nil, nil), "entries_one_per_day")
	// A second entry for one spin (another date).
	_, freeDay := freshSpin(a, catA)
	wantConstraint(t, insertEntry(a, spinA, freeDay, "Again", 5, nil, nil), "entries_one_per_spin")
	// An entry whose spin belongs to another person.
	spinB, _ := freshSpin(b, catB)
	_, anotherDay := freshSpin(a, catA)
	wantConstraint(t, insertEntry(a, spinB, anotherDay, "Not mine", 5, nil, nil), "entries_spin_fk")

	// --- photos ---
	var entryA string
	if err := pool.QueryRow(ctx, `select id::text from entries where user_id = $1 and local_date = $2::date`, a, dayA).Scan(&entryA); err != nil {
		t.Fatal(err)
	}
	spinB2, dayB := freshSpin(b, catB)
	if err := insertEntry(b, spinB2, dayB, "Mine", 5, nil, nil); err != nil {
		t.Fatal(err)
	}
	var entryB string
	if err := pool.QueryRow(ctx, `select id::text from entries where user_id = $1`, b).Scan(&entryB); err != nil {
		t.Fatal(err)
	}

	insertPhoto := func(x execer, user, entry string, position int, key, contentType, source string, size any) error {
		_, err := x.Exec(ctx, `
			insert into entry_photos (entry_id, user_id, position, storage_key, content_type, size_bytes, source, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, now())`, entry, user, position, key, contentType, size, source)
		return err
	}
	good := func(position int) error {
		return insertPhoto(pool, a, entryA, position, newKey(a), "image/jpeg", "camera", nil)
	}
	for _, p := range []int{0, 9} {
		if err := good(p); err != nil {
			t.Fatalf("a photo at position %d: %v", p, err)
		}
	}
	wantConstraint(t, good(10), "entry_photos_position_range")
	wantConstraint(t, good(-1), "entry_photos_position_range")

	// Two rows at one position are accepted by each statement and refused at
	// the commit, because the unique position is deferred.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := insertPhoto(tx, a, entryA, 5, newKey(a), "image/png", "library", nil); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("a photo at position 5 (number %d) inside the transaction: %v", i+1, err)
		}
	}
	wantConstraint(t, tx.Commit(ctx), "entry_photos_position_unique")
	if n := countRows(t, pool, `select count(*) from entry_photos where entry_id = $1 and position = 5`, entryA); n != 0 {
		t.Errorf("%d rows at position 5 after the refused commit, want 0", n)
	}

	// A swap inside one transaction passes (the deferred check sees the end).
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		update entry_photos set position = 9 - position where entry_id = $1 and position in (0, 9)`, entryA); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("swap: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("a swap of two positions was refused at commit: %v", err)
	}

	// A file belongs to one photo row, across entries and across people.
	var taken string
	if err := pool.QueryRow(ctx, `select storage_key from entry_photos where entry_id = $1 and position = 0`, entryA).Scan(&taken); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, insertPhoto(pool, a, entryA, 1, taken, "image/jpeg", "camera", nil), "entry_photos_key_unique")
	wantConstraint(t, insertPhoto(pool, b, entryB, 0, taken, "image/jpeg", "camera", nil), "entry_photos_key_unique")

	// A row can never point at another person's entry.
	wantConstraint(t, insertPhoto(pool, b, entryA, 2, newKey(b), "image/jpeg", "camera", nil), "entry_photos_entry_fk")

	wantConstraint(t, insertPhoto(pool, a, entryA, 2, newKey(a), "image/gif", "camera", nil), "entry_photos_content_type_check")
	wantConstraint(t, insertPhoto(pool, a, entryA, 2, newKey(a), "image/jpeg", "webcam", nil), "entry_photos_source_check")
	wantConstraint(t, insertPhoto(pool, a, entryA, 2, newKey(a), "image/jpeg", "camera", 0), "entry_photos_size_range")
	wantConstraint(t, insertPhoto(pool, a, entryA, 2, newKey(a), "image/jpeg", "camera", 52428801), "entry_photos_size_range")
	wantConstraint(t, insertPhoto(pool, a, entryA, 2, "", "image/jpeg", "camera", nil), "entry_photos_key_len")
	wantConstraint(t, insertPhoto(pool, a, entryA, 2, strings.Repeat("k", 201), "image/jpeg", "camera", nil), "entry_photos_key_len")
	// The limits themselves are fine.
	if err := insertPhoto(pool, a, entryA, 2, newKey(a), "image/webp", "library", 52428800); err != nil {
		t.Errorf("a size of 52428800: %v", err)
	}
	if err := insertPhoto(pool, a, entryA, 3, newKey(a), "image/jpeg", "camera", 1); err != nil {
		t.Errorf("a size of 1: %v", err)
	}

	// Deleting an entry takes its photo rows with it, and frees their keys.
	if n := countRows(t, pool, `select count(*) from entry_photos where entry_id = $1`, entryA); n != 4 {
		t.Fatalf("photo rows of the entry = %d before the delete, want 4", n)
	}
	if _, err := pool.Exec(ctx, `delete from entries where id = $1`, entryA); err != nil {
		t.Fatalf("delete entry: %v", err)
	}
	if n := countRows(t, pool, `select count(*) from entry_photos where entry_id = $1`, entryA); n != 0 {
		t.Errorf("%d photo rows remain after their entry was deleted, want 0", n)
	}
	if err := insertPhoto(pool, b, entryB, 0, taken, "image/jpeg", "camera", nil); err != nil {
		t.Errorf("a key freed by a deleted entry could not be used again: %v", err)
	}
}

// --- InsertEntry on a conflict ---

// A second InsertEntry for a date or a spin that already has an entry returns
// the existing row with inserted false, and the transaction can still be used
// (a unique violation would have aborted it).
func TestEntryRepo_InsertEntryConflictReturnsTheExistingRowAndKeepsTheTransaction(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	repo := postgres.NewSpinRepo(pool)
	day := mustDate(t, "2026-10-07")
	seed := spin.Seed{At: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), LocalDate: day, TZName: "UTC", Goal: 4}
	at := time.Date(2026, 10, 7, 20, 30, 0, 123456000, time.UTC)

	err := repo.InUserTx(ctx, u, seed, func(tx spin.Tx) error {
		cats, err := tx.LiveCategories()
		if err != nil || len(cats) != 6 {
			return fmt.Errorf("seeded categories: %d, %v", len(cats), err)
		}
		mkSpin := func(kind spin.SpinKind, seq int, cat spin.Category) (spin.Spin, error) {
			s, inserted, err := tx.InsertSpin(spin.Spin{
				LocalDate: day, TZName: "UTC", Kind: kind, Seq: seq,
				CategoryID: cat.ID, CategoryName: cat.Name,
				Wheel:       []spin.WheelEntry{{ID: cat.ID, Name: cat.Name, Weight: cat.Weight}},
				WinnerIndex: 0, SpunAt: at, WindowOpensAt: at, WindowClosesAt: at,
			})
			if err != nil || !inserted {
				return spin.Spin{}, fmt.Errorf("insert spin: inserted %v err %v", inserted, err)
			}
			return s, nil
		}
		daily, err := mkSpin(spin.KindDaily, 0, cats[0])
		if err != nil {
			return err
		}
		bonus, err := mkSpin(spin.KindBonus, 1, cats[1])
		if err != nil {
			return err
		}
		if daily.SettledAt != nil {
			return fmt.Errorf("a new spin has settled_at %v, want nil", daily.SettledAt)
		}

		reaction := "fire"
		first, inserted, err := tx.InsertEntry(spin.NewEntry{SpinID: daily.ID, LocalDate: day, Title: "Dune", Stars: 7, Reaction: &reaction, At: at})
		if err != nil || !inserted {
			return fmt.Errorf("first insert: inserted %v err %v", inserted, err)
		}
		if first.ID == "" || first.SpinID != daily.ID || first.CategoryID != cats[0].ID || first.CategoryName != cats[0].Name {
			return fmt.Errorf("first entry = %+v, want the daily spin's category", first)
		}
		if first.Late || len(first.Photos) != 0 || !first.CreatedAt.Equal(at) || !first.UpdatedAt.Equal(at) {
			return fmt.Errorf("first entry = %+v, want not late, no photos and both times %v", first, at)
		}
		if err := tx.InsertPhotos(first.ID, []spin.NewStoredPhoto{
			{Position: 1, Key: "entries/x/b.png", ContentType: "image/png", Source: spin.PhotoSourceLibrary},
			{Position: 0, Key: "entries/x/a.jpg", ContentType: "image/jpeg", Source: spin.PhotoSourceCamera},
		}, at); err != nil {
			return fmt.Errorf("insert photos: %w", err)
		}

		// The same date and the same spin.
		again, inserted, err := tx.InsertEntry(spin.NewEntry{SpinID: daily.ID, LocalDate: day, Title: "Other", Stars: 3, At: at})
		if err != nil {
			return fmt.Errorf("second insert: %w", err)
		}
		if inserted || again.ID != first.ID || again.Title != "Dune" {
			return fmt.Errorf("second insert: inserted %v entry %+v, want the first entry back", inserted, again)
		}
		if len(again.Photos) != 2 || again.Photos[0].Key != "entries/x/a.jpg" || again.Photos[1].Key != "entries/x/b.png" {
			return fmt.Errorf("the entry that came back has photos %+v, want a.jpg then b.png", again.Photos)
		}
		// Another spin of the same date: the date's unique key is the one hit.
		sameDate, inserted, err := tx.InsertEntry(spin.NewEntry{SpinID: bonus.ID, LocalDate: day, Title: "Other", Stars: 3, At: at})
		if err != nil || inserted || sameDate.ID != first.ID {
			return fmt.Errorf("same date, other spin: inserted %v id %s err %v, want the first entry", inserted, sameDate.ID, err)
		}
		// Another date for the same spin: the spin's unique key is the one hit.
		sameSpin, inserted, err := tx.InsertEntry(spin.NewEntry{SpinID: daily.ID, LocalDate: mustDate(t, "2026-10-06"), Title: "Other", Stars: 3, At: at})
		if err != nil || inserted || sameSpin.ID != first.ID {
			return fmt.Errorf("same spin, other date: inserted %v id %s err %v, want the first entry", inserted, sameSpin.ID, err)
		}

		// The transaction is still usable, and Late follows the posted day.
		if _, err := tx.InsertPostedDay(spin.PostedDay{Date: day, Source: spin.SourcePost, Late: true}); err != nil {
			return err
		}
		read, err := tx.EntryOfDate(day)
		if err != nil || read == nil || read.ID != first.ID || !read.Late {
			return fmt.Errorf("read after the conflicts: %+v %v, want the first entry with late true", read, err)
		}
		byID, err := tx.EntryByID(first.ID)
		if err != nil || byID.CategoryName != cats[0].Name || len(byID.Photos) != 2 {
			return fmt.Errorf("EntryByID: %+v %v", byID, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool, `select count(*) from entries where user_id = $1`, u); n != 1 {
		t.Fatalf("entry rows = %d, want 1", n)
	}
	if n := countRows(t, pool, `select count(*) from entry_photos where user_id = $1`, u); n != 2 {
		t.Fatalf("photo rows = %d, want 2", n)
	}
}

// --- keys are global, entries are personal ---

func TestEntryRepo_AKeyHeldByAnotherPersonIsInUse(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	a, b := env.NewUser(t), env.NewUser(t)
	openNewYorkDay(t, env, a)
	if _, err := env.Service.SetTimeZone(ctx, b, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Service.Spin(ctx, b, nil); err != nil {
		t.Fatal(err)
	}
	day := mustDate(t, "2026-10-08")

	in := entryInput(a, 2)
	posted, err := env.Service.PostEntry(ctx, a, day, 0, in)
	if err != nil {
		t.Fatal(err)
	}

	// Person b asks about a's keys and about a fresh one.
	fresh := newKey(b)
	repo := postgres.NewSpinRepo(pool)
	seed := spin.Seed{At: env.Clock.NowFor(b), LocalDate: day, TZName: "America/New_York", Goal: 4}
	err = repo.InUserTx(ctx, b, seed, func(tx spin.Tx) error {
		used, err := tx.PhotoKeysInUse([]string{in.Photos[0].Key, in.Photos[1].Key, fresh})
		if err != nil {
			return err
		}
		if len(used) != 2 || !used[in.Photos[0].Key] || !used[in.Photos[1].Key] || used[fresh] {
			return fmt.Errorf("keys in use = %v, want a's two keys and not the fresh one", used)
		}
		none, err := tx.PhotoKeysInUse(nil)
		if err != nil || none == nil || len(none) != 0 {
			return fmt.Errorf("an empty list gave %v (err %v), want an empty map", none, err)
		}
		if _, err := tx.EntryByID(posted.Entry.ID); !errors.Is(err, spin.ErrEntryNotFound) {
			return fmt.Errorf("b read a's entry by id: %v, want ErrEntryNotFound", err)
		}
		if _, err := tx.DeleteEntry(posted.Entry.ID); !errors.Is(err, spin.ErrEntryNotFound) {
			return fmt.Errorf("b deleted a's entry: %v, want ErrEntryNotFound", err)
		}
		if photos, err := tx.PhotosOfEntry(posted.Entry.ID); err != nil || len(photos) != 0 {
			return fmt.Errorf("b read a's photos: %v %v, want none", photos, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// And a post that names a's key is refused whole.
	stolen := spin.EntryInput{Title: "Mine now", Stars: 5, Photos: []spin.NewPhoto{
		{Key: fresh, Source: spin.PhotoSourceCamera},
		{Key: in.Photos[0].Key, Source: spin.PhotoSourceCamera},
	}}
	_, err = env.Service.PostEntry(ctx, b, day, 0, stolen)
	var problem *spin.FieldProblem
	if !errors.As(err, &problem) || problem.Field != "photos" || problem.Reason != spin.ProblemInvalid {
		t.Fatalf("post with a's key: %v, want a photos invalid problem", err)
	}
	if n := countRows(t, pool, `select count(*) from entries where user_id = $1`, b); n != 0 {
		t.Errorf("b has %d entries after the refused post, want 0", n)
	}
	if n := countRows(t, pool, `select count(*) from entry_photos where storage_key = $1`, in.Photos[0].Key); n != 1 {
		t.Errorf("a's key is held by %d rows, want 1", n)
	}
}

// --- deleting an account ---

func TestEntryTables_DeletingAnAccountClearsThemToo(t *testing.T) {
	env, pool := newSpinEnv(t)
	ctx := context.Background()
	u := env.NewUser(t)
	openNewYorkDay(t, env, u)
	if _, err := env.Service.PostEntry(ctx, u, mustDate(t, "2026-10-08"), 0, entryInput(u, 3)); err != nil {
		t.Fatal(err)
	}
	for tbl, want := range map[string]int{"entries": 1, "entry_photos": 3} {
		if n := countRows(t, pool, `select count(*) from `+tbl+` where user_id = $1`, u); n != want {
			t.Fatalf("%s: %d rows before the delete, want %d", tbl, n, want)
		}
	}
	if _, err := pool.Exec(ctx, `delete from users where id = $1`, u); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	for _, tbl := range []string{"entries", "entry_photos", "spins", "posted_days"} {
		if n := countRows(t, pool, `select count(*) from `+tbl); n != 0 {
			t.Errorf("%s still holds %d rows after the account went", tbl, n)
		}
	}
}
