package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/secondlook"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
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
	// Each post of the owner is on a day of its own, with a category of its
	// own.
	if err := pool.QueryRow(ctx, `with n as (select count(*)::int as n from spins where user_id = $1),
		c as (
			insert into wheel_categories (user_id, name, weight, position)
			select $1, 'Meal ' || n.n, 1, n.n from n returning id),
		s as (
			insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
				winner_index, spun_at, window_opens_at, window_closes_at)
			select $1, '2026-10-08'::date + n.n, 'America/New_York', 'daily', 0, c.id, 'Meal', '[]'::jsonb, 0, now(), now(), now()
			from c, n
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
// would, and returns its id.
func askedAbout(t *testing.T, pool *pgxpool.Pool, owner, removal, item uuid.UUID) uuid.UUID {
	t.Helper()
	var look uuid.UUID
	if err := pool.QueryRow(context.Background(), `insert into second_looks (owner_id, removal_id, item, item_id, body, at)
		values ($1, $2, 'post', $3, 'It was a joke.', now()) returning id`, owner, removal, item).Scan(&look); err != nil {
		t.Fatalf("ask: %v", err)
	}
	return look
}

// Keep it removed answers the open second look about the newest removal,
// logs kept on the owner's clock with the title, and changes nothing more on
// a second click; with no open request, for someone else, once it is back
// (no request is open then: the Bring it back answered it) or past the 30
// days it changes nothing. Bring it back answers the request too, also one
// a Keep answered, and a removal after that has no request until one is
// asked; a Keep naming the first request never answers that new one.
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

	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, uuid.New(), loggedAt); !errors.Is(err, admin.ErrNoRequest) {
		t.Fatalf("Keep with nothing asked: %v, want ErrNoRequest", err)
	}
	look := askedAbout(t, pool, a.ID, removal, post)
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, b.ID, look, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep naming someone else: %v, want ErrNotFound", err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindComment, post, a.ID, look, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep of a comment that is a post: %v, want ErrNotFound", err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, uuid.New(), loggedAt); !errors.Is(err, admin.ErrNoRequest) || kept() != 0 {
		t.Fatalf("Keep naming another request: %v with %d kept rows, want ErrNoRequest and 0", err, kept())
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, look, loggedAt); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	if got := lookAnswer(t, pool, removal); got != "kept" || kept() != 1 {
		t.Fatalf("after Keep the request is %q with %d kept rows, want kept and 1", got, kept())
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, look, loggedAt); !errors.Is(err, admin.ErrAlreadyDone) || kept() != 1 {
		t.Fatalf("Keep again: %v with %d kept rows, want ErrAlreadyDone and 1", err, kept())
	}
	if err := repo.RestoreItem(ctx, admin.KindPost, post, a.ID, loggedAt); err != nil {
		t.Fatalf("Bring it back after Keep: %v", err)
	}
	if got := lookAnswer(t, pool, removal); got != "restored" {
		t.Fatalf("after Bring it back the request is %q, want restored", got)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, look, loggedAt); !errors.Is(err, admin.ErrNoRequest) || kept() != 1 {
		t.Fatalf("Keep once it is back: %v with %d kept rows, want ErrNoRequest (the Bring it back answered it) and 1", err, kept())
	}

	// Removed again: a new removal with no request of its own.
	now := time.Now()
	if err := repo.RemoveItem(ctx, admin.KindPost, post, a.ID, "", now, now); err != nil {
		t.Fatalf("remove again: %v", err)
	}
	again := newestRemovalOf(t, pool, post)
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, look, loggedAt); !errors.Is(err, admin.ErrNoRequest) {
		t.Fatalf("Keep on the new removal with nothing asked: %v, want ErrNoRequest", err)
	}
	askedAbout(t, pool, a.ID, again, post)
	if err := repo.KeepRemoved(ctx, admin.KindPost, post, a.ID, look, loggedAt); !errors.Is(err, admin.ErrNoRequest) ||
		lookAnswer(t, pool, again) != "open" || kept() != 1 {
		t.Fatalf("Keep from a page drawn for the first request: %v, the new request %q, %d kept rows; want ErrNoRequest, open and 1",
			err, lookAnswer(t, pool, again), kept())
	}
	if err := repo.RestoreItem(ctx, admin.KindPost, post, a.ID, loggedAt); err != nil {
		t.Fatalf("Bring it back on an open request: %v", err)
	}
	if lookAnswer(t, pool, again) != "restored" || lookAnswer(t, pool, removal) != "restored" || kept() != 1 {
		t.Fatalf("after the second Bring it back: %q and %q, %d kept rows", lookAnswer(t, pool, again), lookAnswer(t, pool, removal), kept())
	}

	// Past the 30 days nothing is answered: only the purge touches it.
	late, lateRemoval := removedPostOf(t, pool, b.ID, "Old")
	lateLook := askedAbout(t, pool, b.ID, lateRemoval, late)
	if _, err := pool.Exec(ctx, `update entries set removed_at = now() - interval '30 days' where id = $1`, late); err != nil {
		t.Fatal(err)
	}
	if err := repo.KeepRemoved(ctx, admin.KindPost, late, b.ID, lateLook, loggedAt); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("Keep past the 30 days: %v, want ErrNotFound", err)
	}
	if got := lookAnswer(t, pool, lateRemoval); got != "open" {
		t.Fatalf("past the 30 days the request is %q, want open", got)
	}
}

// The bell's read of the decision log says closed on a removal nobody asked
// about that can no longer be asked about (past the 30 days, or deleted),
// and keeps every removal that can still be asked about past its limit, as
// that line is the only way to ask.
func TestNotificationsRepo_DecisionsKeepWhatCanStillBeAskedAbout(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "bell-looks@example.com")
	removedPostOf(t, pool, a.ID, "Old")
	gone, _ := removedPostOf(t, pool, a.ID, "Gone")
	late, lateRemoval := removedPostOf(t, pool, a.ID, "Late")
	asked, askedRemoval := removedPostOf(t, pool, a.ID, "Asked")
	removedPostOf(t, pool, a.ID, "New")
	askedAbout(t, pool, a.ID, askedRemoval, asked)
	if _, err := pool.Exec(ctx, `delete from entries where id = $1`, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `with e as (update entries set removed_at = now() - interval '30 days' where id = $1)
		update moderation_events set created_at = now() - interval '30 days' where id = $2`, late, lateRemoval); err != nil {
		t.Fatal(err)
	}
	read := func(limit int) []string {
		t.Helper()
		decisions, err := postgres.NewNotificationsRepo(pool).Decisions(ctx, a.ID, time.Now().Add(-time.Hour), limit)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, d := range decisions {
			out = append(out, d.Snapshot+" "+d.SecondLook)
		}
		return out
	}
	if got, want := read(10), []string{"New ", "Asked asked", "Late closed", "Gone closed", "Old "}; !slices.Equal(got, want) {
		t.Fatalf("the removals: %q, want %q", got, want)
	}
	if got, want := read(1), []string{"New ", "Old "}; !slices.Equal(got, want) {
		t.Fatalf("the newest one: %q, want %q, the old one too as it can still be asked about", got, want)
	}
}

// commentOf stores a comment of writer on post, with photo key (none when
// ""), and returns its id.
func commentOf(t *testing.T, pool *pgxpool.Pool, post, writer uuid.UUID, body, key string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `insert into post_comments (entry_id, user_id, body, created_at, photo_key)
		values ($1, $2, $3, now(), nullif($4, '')) returning id`, post, writer, body, key).Scan(&id); err != nil {
		t.Fatalf("a comment: %v", err)
	}
	return id
}

// asked is the stored request about removal: its item, post, comment, words
// and photo keys, one line.
func asked(t *testing.T, pool *pgxpool.Pool, removal uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `select item || ' ' || item_id || ' ' || coalesce(entry_id::text, '-') || ' ' ||
		coalesce(comment_id::text, '-') || ' ' || body || ' ' || array_to_string(photo_keys, ',') from second_looks where removal_id = $1`,
		removal).Scan(&s); err != nil {
		t.Fatalf("the request: %v", err)
	}
	return s
}

// Asking follows the rules: only the owner's own removal of a post or
// comment, once (the same request again is a retry), not after a Bring it
// back or a newer removal, not past the 30 days or once purged, and with
// files no photo row, comment or other request uses. A comment under a
// removed post can ask too.
func TestSecondLookRepo_AskFollowsTheRules(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSecondLookRepo(pool)
	admins := postgres.NewAdminRepo(pool)
	users := postgres.NewUserRepo(pool)
	a := mustCreateUser(t, users, "ask-owner@example.com")
	b := mustCreateUser(t, users, "ask-other@example.com")
	at := time.Date(2026, 11, 19, 21, 5, 0, 0, time.UTC)
	post, removal := removedPostOf(t, pool, a.ID, "Dune")
	k1, k2 := newKey(a.ID.String()), newKey(a.ID.String())
	ask := func(owner, removal uuid.UUID, body string, keys ...string) (bool, error) {
		t.Helper()
		if keys == nil {
			keys = []string{}
		}
		return repo.Ask(ctx, owner, removal, body, keys, at)
	}
	var hidden uuid.UUID
	if err := pool.QueryRow(ctx, `insert into moderation_events (owner_id, item, action, entry_id, item_id, at)
		values ($1, 'post', 'hidden', $2, $2, now()) returning id`, a.ID, post).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string][2]uuid.UUID{
		"someone else's removal": {b.ID, removal},
		"no such removal":        {a.ID, uuid.New()},
		"a hide, not a removal":  {a.ID, hidden},
	} {
		if _, err := ask(id[0], id[1], "Why"); !errors.Is(err, secondlook.ErrNotFound) {
			t.Fatalf("%s: %v, want ErrNotFound", name, err)
		}
	}

	if made, err := ask(a.ID, removal, "It was a joke.", k1, k2); err != nil || !made {
		t.Fatalf("ask: %v, %v", made, err)
	}
	if got, want := asked(t, pool, removal), "post "+post.String()+" "+post.String()+" - It was a joke. "+k1+","+k2; got != want {
		t.Fatalf("stored %q, want %q", got, want)
	}
	var keptAt time.Time
	if err := pool.QueryRow(ctx, `select at from second_looks where removal_id = $1`, removal).Scan(&keptAt); err != nil || !keptAt.Equal(at) {
		t.Fatalf("kept at %v (%v), want the owner's %v", keptAt, err, at)
	}
	if made, err := ask(a.ID, removal, "It was a joke.", k1, k2); err != nil || made {
		t.Fatalf("the same request again: %v, %v, want a retry", made, err)
	}
	for name, keys := range map[string][]string{"other words": {k1, k2}, "other photos": {k1}} {
		body := "It was a joke."
		if name == "other words" {
			body = "Please"
		}
		if _, err := ask(a.ID, removal, body, keys...); !errors.Is(err, secondlook.ErrAsked) {
			t.Fatalf("a second request with %s: %v, want ErrAsked", name, err)
		}
	}

	// One file, one row: a post's photo, a comment's, another request's.
	second, secondRemoval := removedPostOf(t, pool, a.ID, "Arrival")
	onPost, onComment := newKey(a.ID.String()), newKey(a.ID.String())
	if _, err := pool.Exec(ctx, `insert into entry_photos (entry_id, user_id, position, storage_key, content_type, source, created_at)
		values ($1, $2, 0, $3, 'image/jpeg', 'library', now())`, second, a.ID, onPost); err != nil {
		t.Fatal(err)
	}
	commentOf(t, pool, second, a.ID, "", onComment)
	for name, k := range map[string]string{"a post's photo": onPost, "a comment's photo": onComment, "another request's photo": k2} {
		var problem *secondlook.FieldProblem
		if _, err := ask(a.ID, secondRemoval, "Why", newKey(a.ID.String()), k); !errors.As(err, &problem) || problem.Field != "photo_keys" {
			t.Fatalf("%s as proof: %v, want photo_keys invalid", name, err)
		}
	}

	// A comment under the removed post, removed too, asks about itself.
	note := commentOf(t, pool, second, b.ID, "Rude", "")
	now := time.Now()
	if err := admins.RemoveItem(ctx, admin.KindComment, note, b.ID, "", now, now); err != nil {
		t.Fatal(err)
	}
	noteRemoval := newestRemovalOf(t, pool, note)
	if made, err := ask(b.ID, noteRemoval, "Not rude"); err != nil || !made {
		t.Fatalf("a comment under a removed post: %v, %v", made, err)
	}
	if got, want := asked(t, pool, noteRemoval), "comment "+note.String()+" "+second.String()+" "+note.String()+" Not rude "; got != want {
		t.Fatalf("stored %q, want %q", got, want)
	}

	// Brought back, and removed again: the old removal is answered.
	if err := admins.RestoreItem(ctx, admin.KindPost, second, a.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := ask(a.ID, secondRemoval, "Why"); !errors.Is(err, secondlook.ErrBack) {
		t.Fatalf("after Bring it back: %v, want ErrBack", err)
	}
	if err := admins.RemoveItem(ctx, admin.KindPost, second, a.ID, "", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := ask(a.ID, secondRemoval, "Why"); !errors.Is(err, secondlook.ErrBack) {
		t.Fatalf("the older removal of one removed again: %v, want ErrBack", err)
	}
	if made, err := ask(a.ID, newestRemovalOf(t, pool, second), "Why"); err != nil || !made {
		t.Fatalf("the newer removal: %v, %v", made, err)
	}

	// Past the 30 days, purged or not yet, and gone another way before.
	late, lateRemoval := removedPostOf(t, pool, b.ID, "Old")
	backdate := func(item, removal uuid.UUID) {
		t.Helper()
		if _, err := pool.Exec(ctx, `with e as (update entries set removed_at = now() - interval '30 days' where id = $1)
			update moderation_events set created_at = now() - interval '30 days' where id = $2`, item, removal); err != nil {
			t.Fatal(err)
		}
	}
	backdate(late, lateRemoval)
	if _, err := ask(b.ID, lateRemoval, "Why"); !errors.Is(err, secondlook.ErrTooLate) {
		t.Fatalf("past the 30 days: %v, want ErrTooLate", err)
	}
	if _, err := pool.Exec(ctx, `delete from entries where id = $1`, late); err != nil {
		t.Fatal(err)
	}
	if _, err := ask(b.ID, lateRemoval, "Why"); !errors.Is(err, secondlook.ErrTooLate) {
		t.Fatalf("purged: %v, want ErrTooLate", err)
	}
	gone, goneRemoval := removedPostOf(t, pool, b.ID, "Gone")
	if _, err := pool.Exec(ctx, `delete from entries where id = $1`, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := ask(b.ID, goneRemoval, "Why"); !errors.Is(err, secondlook.ErrNotFound) {
		t.Fatalf("deleted before the 30 days: %v, want ErrNotFound", err)
	}
	if n := countRows(t, pool, `select count(*) from second_looks`); n != 3 {
		t.Fatalf("%d requests stored, want 3", n)
	}
}

// A Bring it back under way when the request comes holds the post: the
// request waits for it and then finds the removal answered.
func TestSecondLookRepo_AskWaitsForABringItBackUnderWay(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "ask-race-back@example.com")
	post, removal := removedPostOf(t, pool, a.ID, "Dune")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed
	if _, err := tx.Exec(ctx, `update entries set removed_at = null where id = $1`, post); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `insert into moderation_events (owner_id, item, action, entry_id, item_id, at)
		values ($1, 'post', 'restored', $2, $2, now())`, a.ID, post); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewSecondLookRepo(pool).Ask(ctx, a.ID, removal, "Why", []string{}, time.Now())
		done <- err
	}()
	waitForALockWait(t, pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, secondlook.ErrBack) {
		t.Fatalf("the request after the Bring it back it waited for: %v, want ErrBack", err)
	}
	if n := countRows(t, pool, `select count(*) from second_looks`); n != 0 {
		t.Fatalf("%d requests stored, want none", n)
	}
}

// A request under way holds the post for share: a Bring it back waits for
// it and then answers the request it saved.
func TestSecondLookRepo_BringItBackWaitsForARequestUnderWay(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "ask-race-ask@example.com")
	post, removal := removedPostOf(t, pool, a.ID, "Dune")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed
	if _, err := tx.Exec(ctx, `select 1 from entries where id = $1 for share`, post); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `insert into second_looks (owner_id, removal_id, item, item_id, entry_id, body, at)
		values ($1, $2, 'post', $3, $3, 'Why', now())`, a.ID, removal, post); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- postgres.NewAdminRepo(pool).RestoreItem(ctx, admin.KindPost, post, a.ID, time.Now()) }()
	waitForALockWait(t, pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Bring it back after the request: %v", err)
	}
	if got := lookAnswer(t, pool, removal); got != "restored" {
		t.Fatalf("the request is %q, want restored", got)
	}
}

// Two requests of one owner take turns, so a file the first takes is not
// free for the second, though it asks about another removal.
func TestSecondLookRepo_TwoRequestsTakeTurnsOverAFile(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "ask-race-file@example.com")
	first, firstRemoval := removedPostOf(t, pool, a.ID, "Dune")
	_, secondRemoval := removedPostOf(t, pool, a.ID, "Arrival")
	k := newKey(a.ID.String())
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('second_look:' || $1::text, 0))`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `insert into second_looks (owner_id, removal_id, item, item_id, entry_id, body, photo_keys, at)
		values ($1, $2, 'post', $3, $3, 'Why', $4, now())`, a.ID, firstRemoval, first, []string{k}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewSecondLookRepo(pool).Ask(ctx, a.ID, secondRemoval, "Why", []string{k}, time.Now())
		done <- err
	}()
	waitForALockWait(t, pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var problem *secondlook.FieldProblem
	if err := <-done; !errors.As(err, &problem) || problem.Field != "photo_keys" {
		t.Fatalf("the second request with the first one's file: %v, want photo_keys invalid", err)
	}
}

// A proof's file is refused by a post and a comment, and no delete of a
// post or a comment deletes it, though a hand-made race left it on both.
func TestPhotoKeys_AProofsFileIsItsAlone(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "proof-file@example.com")
	_, removal := removedPostOf(t, pool, a.ID, "Dune")
	k := newKey(a.ID.String())
	if made, err := postgres.NewSecondLookRepo(pool).Ask(ctx, a.ID, removal, "Why", []string{k}, time.Now()); err != nil || !made {
		t.Fatalf("ask: %v, %v", made, err)
	}
	seed := spin.Seed{At: time.Now(), LocalDate: mustDate(t, "2026-10-08"), TZName: "America/New_York", Goal: 4}
	if err := postgres.NewSpinRepo(pool).InUserTx(ctx, a.ID.String(), seed, func(tx spin.Tx) error {
		used, err := tx.PhotoKeysInUse([]string{k, newKey(a.ID.String())})
		if err != nil {
			return err
		}
		if len(used) != 1 || !used[k] {
			return fmt.Errorf("keys in use = %v, want the proof's only", used)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A live post of a's, with a comment, for the comment insert.
	live, _ := removedPostOf(t, pool, a.ID, "Live")
	if err := postgres.NewAdminRepo(pool).RestoreItem(ctx, admin.KindPost, live, a.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := postgres.NewBoardRepo(pool).InsertComment(ctx, live, a.ID, "", time.Now(), nil, &k)
	var problem *board.FieldProblem
	if !errors.As(err, &problem) || problem.Field != "photo" {
		t.Fatalf("a comment with a proof's file: %v, want photo invalid", err)
	}

	// The race the checks cannot stop: the file on a photo row and a comment
	// as well. Deleting them leaves it alone.
	if _, err := pool.Exec(ctx, `insert into entry_photos (entry_id, user_id, position, storage_key, content_type, source, created_at)
		values ($1, $2, 0, $3, 'image/jpeg', 'library', now())`, live, a.ID, k); err != nil {
		t.Fatal(err)
	}
	commentOf(t, pool, live, a.ID, "", k)
	var deleted []string
	if err := postgres.NewSpinRepo(pool).InUserTx(ctx, a.ID.String(), seed, func(tx spin.Tx) error {
		var err error
		deleted, err = tx.DeleteEntry(live.String())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleting the post deletes %v, want nothing", deleted)
	}
}

// proofFiles is the photo store the sweep deletes from: it records each key.
type proofFiles struct{ deleted []string }

func (f *proofFiles) DeleteAccountFolders(context.Context, uuid.UUID) (int, error) { return 0, nil }

func (f *proofFiles) DeletePhoto(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

// The purge's run deletes a second look's photos 30 days after the removal,
// on the purge's own line and whatever the answer (a request about a post
// that came back included), and empties its keys; a request inside the 30
// days keeps them, and a file a post's photo also names (the hand-made race
// the checks cannot stop) is left alone. The request stays, with its words,
// also once the purge deletes its post.
func TestSecondLookPurge_DeletesTheProofPhotos30DaysAfterTheRemoval(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	a := mustCreateUser(t, postgres.NewUserRepo(pool), "proof-sweep@example.com")
	repo := postgres.NewSecondLookRepo(pool)
	k := []string{newKey(a.ID.String()), newKey(a.ID.String()), newKey(a.ID.String()), newKey(a.ID.String())}
	ask := func(removal uuid.UUID, keys ...string) {
		t.Helper()
		if made, err := repo.Ask(ctx, a.ID, removal, "Why", keys, time.Now()); err != nil || !made {
			t.Fatalf("ask: %v, %v", made, err)
		}
	}
	backdate := func(item, removal uuid.UUID) {
		t.Helper()
		if _, err := pool.Exec(ctx, `with e as (update entries set removed_at = now() - interval '30 days' where id = $1 and removed_at is not null)
			update moderation_events set created_at = now() - interval '30 days' where id = $2`, item, removal); err != nil {
			t.Fatal(err)
		}
	}
	old, oldRemoval := removedPostOf(t, pool, a.ID, "Old")
	ask(oldRemoval, k[0], k[1])
	_, newRemoval := removedPostOf(t, pool, a.ID, "New")
	ask(newRemoval, k[2])
	back, backRemoval := removedPostOf(t, pool, a.ID, "Back")
	ask(backRemoval, k[3])
	if err := postgres.NewAdminRepo(pool).RestoreItem(ctx, admin.KindPost, back, a.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	backdate(old, oldRemoval)
	backdate(back, backRemoval)
	if _, err := pool.Exec(ctx, `insert into entry_photos (entry_id, user_id, position, storage_key, content_type, source, created_at)
		values ($1, $2, 0, $3, 'image/jpeg', 'library', now())`, back, a.ID, k[1]); err != nil {
		t.Fatal(err)
	}

	files := &proofFiles{}
	p := purge.New(postgres.NewPurgeRepo(pool), files, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.SetSecondLooks(postgres.NewSecondLookPurgeRepo(pool))
	if res := p.RunSecondLookPhotosOnce(ctx); res != (purge.Result{Due: 2, Purged: 2}) {
		t.Fatalf("the sweep: %+v, want 2 due and both cleared", res)
	}
	if !slices.Equal(files.deleted, []string{k[0], k[3]}) {
		t.Fatalf("deleted %v, want %v", files.deleted, []string{k[0], k[3]})
	}
	for removal, want := range map[uuid.UUID]string{oldRemoval: "", backRemoval: "", newRemoval: k[2]} {
		var keys string
		if err := pool.QueryRow(ctx, `select array_to_string(photo_keys, ',') from second_looks where removal_id = $1`, removal).Scan(&keys); err != nil || keys != want {
			t.Fatalf("the request about %v keeps %q (%v), want %q", removal, keys, err, want)
		}
	}
	if res := p.RunSecondLookPhotosOnce(ctx); res != (purge.Result{}) || len(files.deleted) != 2 {
		t.Fatalf("a second run: %+v, %d deleted", res, len(files.deleted))
	}
	if _, err := pool.Exec(ctx, `delete from entries where id = $1`, old); err != nil {
		t.Fatal(err)
	}
	if got := asked(t, pool, oldRemoval); got != "post "+old.String()+" - - Why " {
		t.Fatalf("the request once its post is purged: %q", got)
	}
	// The account going takes its requests (and the account purge its
	// folders, with any photo still there).
	if _, err := pool.Exec(ctx, `delete from users where id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool, `select count(*) from second_looks`); n != 0 {
		t.Fatalf("%d requests outlive their account", n)
	}
}

// A delete of a post holds the post, then deletes its comments. A request
// about a removed comment under it takes the post before the comment, in
// that same order, so the two take turns instead of deadlocking: here the
// delete goes first, and the request finds the comment gone.
func TestSecondLookRepo_ARequestAndADeleteOfItsPostTakeTurns(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(pool)
	a := mustCreateUser(t, users, "turns-post-owner@example.com")
	b := mustCreateUser(t, users, "turns-writer@example.com")
	post, _ := removedPostOf(t, pool, a.ID, "Dune")
	if err := postgres.NewAdminRepo(pool).RestoreItem(ctx, admin.KindPost, post, a.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	note := commentOf(t, pool, post, b.ID, "Rude", "")
	now := time.Now()
	if err := postgres.NewAdminRepo(pool).RemoveItem(ctx, admin.KindComment, note, b.ID, "", now, now); err != nil {
		t.Fatal(err)
	}
	removal := newestRemovalOf(t, pool, note)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed
	if _, err := tx.Exec(ctx, `select id from entries where id = $1 for update`, post); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewSecondLookRepo(pool).Ask(ctx, b.ID, removal, "Why", []string{}, time.Now())
		done <- err
	}()
	waitForALockWait(t, pool)
	if _, err := tx.Exec(ctx, `delete from entries where id = $1`, post); err != nil {
		t.Fatalf("the delete of the post: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit the delete: %v", err)
	}
	if err := <-done; !errors.Is(err, secondlook.ErrNotFound) {
		t.Fatalf("the request after its post was deleted: %v, want ErrNotFound", err)
	}
}
