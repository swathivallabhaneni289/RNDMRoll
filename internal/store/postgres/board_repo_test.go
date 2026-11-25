package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// BRD-06: a like on a post that is gone by the time it is saved (the owner's
// delete, the removed-post purge or the account purge, racing the visibility
// read) is not found, as a comment is, so the route answers 404 and not 500.
// Before, the foreign key failure (23503) came back raw.
func TestBoardRepo_ALikeOnAPostThatIsGoneIsNotFound(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	u, err := postgres.NewUserRepo(pool).Create(ctx, "like-gone-"+uuid.NewString()+"@example.com", nil, false, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	inserted, err := postgres.NewBoardRepo(pool).SetReaction(ctx, uuid.New(), u.ID, "love", time.Now().UTC())
	if !errors.Is(err, board.ErrNotFound) || inserted {
		t.Fatalf("a like on a post that is gone: %v (inserted %v), want board.ErrNotFound", err, inserted)
	}
}

// postOf stores a post of owner, each on a day and with a category of its
// own, and returns its id.
func postOf(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, title string) uuid.UUID {
	t.Helper()
	var entry uuid.UUID
	if err := pool.QueryRow(context.Background(), `with n as (select count(*)::int as n from spins where user_id = $1),
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
	return entry
}

// Only a like that makes the row is new, so only that one is pushed
// (board.Service.SetReaction): the same name sent again changes nothing and
// returns no row, and a change of name updates the row, keeping its
// created_at. An undo deletes the row, so the like after it is new again
// (the push's own memo keeps that one from buzzing twice in a day).
func TestBoardRepo_SetReactionSaysWhetherItMadeTheRow(t *testing.T) {
	pool := requireTestPool(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(pool)
	owner := mustCreateUser(t, users, "like-made-owner@example.com")
	liker := mustCreateUser(t, users, "like-made-liker@example.com")
	entry := postOf(t, pool, owner.ID, "Dune")
	repo := postgres.NewBoardRepo(pool)
	first := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	stored := func() (name string, created, updated time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx, `select reaction, created_at, updated_at from post_reactions where entry_id = $1 and user_id = $2`,
			entry, liker.ID).Scan(&name, &created, &updated); err != nil {
			t.Fatalf("the stored like: %v", err)
		}
		return name, created.UTC(), updated.UTC()
	}
	set := func(what, name string, at time.Time, want bool) {
		t.Helper()
		inserted, err := repo.SetReaction(ctx, entry, liker.ID, name, at)
		if err != nil || inserted != want {
			t.Fatalf("%s: inserted %v (err %v), want %v", what, inserted, err, want)
		}
	}

	set("a first like", "love", first, true)
	set("the same like sent again", "love", first.Add(time.Minute), false)
	if name, created, updated := stored(); name != "love" || !created.Equal(first) || !updated.Equal(first) {
		t.Fatalf("after the same like again: %s %v %v, want love and both times unchanged", name, created, updated)
	}
	set("a change of name", "fire", first.Add(2*time.Minute), false)
	if name, created, updated := stored(); name != "fire" || !created.Equal(first) || !updated.Equal(first.Add(2*time.Minute)) {
		t.Fatalf("after a change of name: %s %v %v, want fire, the first created_at and a new updated_at", name, created, updated)
	}
	if err := repo.ClearReaction(ctx, entry, liker.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	set("a like after the undo", "love", first.Add(3*time.Minute), true)
}
