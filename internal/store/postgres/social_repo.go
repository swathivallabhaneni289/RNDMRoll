package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// SocialRepo is social.Repository over the pool. A person is read as id,
// name, username and avatar link only, and only for a finished account (a
// name and a username).
type SocialRepo struct {
	pool *pgxpool.Pool
}

// NewSocialRepo builds a SocialRepo backed by pool.
func NewSocialRepo(pool *pgxpool.Pool) *SocialRepo {
	return &SocialRepo{pool: pool}
}

var _ social.Repository = (*SocialRepo)(nil)

// personColumns are the only user columns this repository reads, with the
// table alias u.
const personColumns = `u.id, coalesce(u.name, ''), coalesce(u.username, ''), u.avatar_url`

// blockedEither is a where fragment true when $a and $b have a block either
// way. It is built with the two placeholders.
func blockedEither(a, b string) string {
	return `exists (select 1 from blocks bk where (bk.blocker_id = ` + a + ` and bk.blocked_id = ` + b +
		`) or (bk.blocker_id = ` + b + ` and bk.blocked_id = ` + a + `))`
}

func scanPerson(row interface{ Scan(...any) error }, extra ...any) (social.Person, error) {
	var p social.Person
	dest := append([]any{&p.ID, &p.Name, &p.Username, &p.PhotoURL}, extra...)
	err := row.Scan(dest...)
	return p, err
}

// pairLock takes the transaction lock for the pair of people, in a fixed
// order, so writes about the same two people run one at a time.
func pairLock(ctx context.Context, tx pgx.Tx, a, b uuid.UUID) error {
	lo, hi := a.String(), b.String()
	if hi < lo {
		lo, hi = hi, lo
	}
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('follow:' || $1::text || ':' || $2::text, 0))`, lo, hi)
	return err
}

func (r *SocialRepo) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindByUsername implements social.Repository.
func (r *SocialRepo) FindByUsername(ctx context.Context, username string) (social.Person, bool, error) {
	p, err := scanPerson(r.pool.QueryRow(ctx, `
		select `+personColumns+`
		from users u
		where lower(u.username) = $1 and u.name is not null and u.name <> ''`, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return social.Person{}, false, social.ErrNotFound
	}
	if err != nil {
		return social.Person{}, false, err
	}
	// Every account acts as private (2026-10-16): a follow is always a request.
	return p, true, nil
}

// RelationOf implements social.Repository.
func (r *SocialRepo) RelationOf(ctx context.Context, viewer, other uuid.UUID) (social.Relation, bool, error) {
	var mine *string
	var followsYou bool
	err := r.pool.QueryRow(ctx, `
		select
			(select status from follows where follower_id = $1 and followee_id = $2),
			exists (select 1 from follows where follower_id = $2 and followee_id = $1 and status = 'accepted')`,
		viewer, other).Scan(&mine, &followsYou)
	if err != nil {
		return "", false, err
	}
	rel := social.RelationNone
	if mine != nil {
		rel = social.RelationRequested
		if *mine == social.StatusAccepted {
			rel = social.RelationFollowing
		}
	}
	return rel, followsYou, nil
}

// People implements social.Repository.
func (r *SocialRepo) People(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]social.Person, error) {
	out := make(map[uuid.UUID]social.Person, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `select `+personColumns+` from users u where u.id = any($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

// HasBlock implements social.Repository.
func (r *SocialRepo) HasBlock(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var blocked bool
	err := r.pool.QueryRow(ctx, `select `+blockedEither("$1::uuid", "$2::uuid"), a, b).Scan(&blocked)
	return blocked, err
}

// Follow implements social.Repository. Every follow starts as a request; the
// followee's users row is read with FOR SHARE so it cannot be deleted under
// the insert.
func (r *SocialRepo) Follow(ctx context.Context, follower, followee uuid.UUID, at time.Time) (social.Relation, error) {
	var rel social.Relation
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		if err := pairLock(ctx, tx, follower, followee); err != nil {
			return err
		}
		var exists int
		err := tx.QueryRow(ctx, `
			select 1 from users
			where id = $1 and name is not null and name <> '' and username is not null
			for share`, followee).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) {
			return social.ErrNotFound
		}
		if err != nil {
			return err
		}
		var blocked bool
		if err := tx.QueryRow(ctx, `select `+blockedEither("$1::uuid", "$2::uuid"), follower, followee).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return social.ErrNotFound
		}
		var status string
		err = tx.QueryRow(ctx, `select status from follows where follower_id = $1 and followee_id = $2`, follower, followee).Scan(&status)
		switch {
		case err == nil:
			rel = social.RelationRequested
			if status == social.StatusAccepted {
				rel = social.RelationFollowing
			}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		// Every account acts as private (developer, 2026-10-16): a follow is
		// always a request the other person accepts or declines.
		_, err = tx.Exec(ctx, `
			insert into follows (follower_id, followee_id, status, requested_at)
			values ($1, $2, 'pending', $3)`, follower, followee, at)
		rel = social.RelationRequested
		return err
	})
	return rel, err
}

// Unfollow implements social.Repository.
func (r *SocialRepo) Unfollow(ctx context.Context, follower, followee uuid.UUID) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := pairLock(ctx, tx, follower, followee); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `delete from follows where follower_id = $1 and followee_id = $2`, follower, followee)
		return err
	})
}

// RemoveFollower implements social.Repository.
func (r *SocialRepo) RemoveFollower(ctx context.Context, owner, follower uuid.UUID) error {
	return r.Unfollow(ctx, follower, owner)
}

// Accept implements social.Repository.
func (r *SocialRepo) Accept(ctx context.Context, owner, requestID uuid.UUID, at time.Time) error {
	return r.resolveRequest(ctx, owner, requestID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update follows set status = 'accepted', via = 'request', accepted_at = $2
			where id = $1 and status = 'pending'`, requestID, at)
		return err
	})
}

// Decline implements social.Repository.
func (r *SocialRepo) Decline(ctx context.Context, owner, requestID uuid.UUID) error {
	return r.resolveRequest(ctx, owner, requestID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `delete from follows where id = $1 and status = 'pending'`, requestID)
		return err
	})
}

// resolveRequest finds a pending request addressed to owner, takes the pair
// lock, reads it again under the lock, and runs fn. Anything else is
// ErrNotFound.
func (r *SocialRepo) resolveRequest(ctx context.Context, owner, requestID uuid.UUID, fn func(pgx.Tx) error) error {
	var follower uuid.UUID
	err := r.pool.QueryRow(ctx, `
		select follower_id from follows where id = $1 and followee_id = $2 and status = 'pending'`,
		requestID, owner).Scan(&follower)
	if errors.Is(err, pgx.ErrNoRows) {
		return social.ErrNotFound
	}
	if err != nil {
		return err
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := pairLock(ctx, tx, owner, follower); err != nil {
			return err
		}
		var still bool
		if err := tx.QueryRow(ctx, `
			select exists (select 1 from follows where id = $1 and followee_id = $2 and status = 'pending')`,
			requestID, owner).Scan(&still); err != nil {
			return err
		}
		if !still {
			return social.ErrNotFound
		}
		return fn(tx)
	})
}

// AcceptAllPending implements social.Repository.
func (r *SocialRepo) AcceptAllPending(ctx context.Context, owner uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update follows set status = 'accepted', via = 'silent', accepted_at = $2
		where followee_id = $1 and status = 'pending'`, owner, at)
	return err
}

// Lists implements social.Repository.
func (r *SocialRepo) Lists(ctx context.Context, viewer uuid.UUID) (social.Lists, error) {
	out := social.Lists{Followers: []social.Person{}, Following: []social.Person{}, Incoming: []social.Request{}, Outgoing: []social.Request{}}
	people := func(q string) ([]social.Person, error) {
		rows, err := r.pool.Query(ctx, q, viewer)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		list := []social.Person{}
		for rows.Next() {
			p, err := scanPerson(rows)
			if err != nil {
				return nil, err
			}
			list = append(list, p)
		}
		return list, rows.Err()
	}
	requests := func(q string) ([]social.Request, error) {
		rows, err := r.pool.Query(ctx, q, viewer)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		list := []social.Request{}
		for rows.Next() {
			var req social.Request
			p, err := scanPerson(rows, &req.ID, &req.At)
			if err != nil {
				return nil, err
			}
			req.Person, req.At = p, req.At.UTC()
			list = append(list, req)
		}
		return list, rows.Err()
	}
	var err error
	if out.Followers, err = people(`
		select ` + personColumns + ` from follows f join users u on u.id = f.follower_id
		where f.followee_id = $1 and f.status = 'accepted' and not ` + blockedEither("$1::uuid", "u.id") + `
		order by lower(u.name), u.username`); err != nil {
		return social.Lists{}, err
	}
	if out.Following, err = people(`
		select ` + personColumns + ` from follows f join users u on u.id = f.followee_id
		where f.follower_id = $1 and f.status = 'accepted' and not ` + blockedEither("$1::uuid", "u.id") + `
		order by lower(u.name), u.username`); err != nil {
		return social.Lists{}, err
	}
	if out.Incoming, err = requests(`
		select ` + personColumns + `, f.id, f.requested_at from follows f join users u on u.id = f.follower_id
		where f.followee_id = $1 and f.status = 'pending' and not ` + blockedEither("$1::uuid", "u.id") + `
		order by f.requested_at desc, f.id desc`); err != nil {
		return social.Lists{}, err
	}
	if out.Outgoing, err = requests(`
		select ` + personColumns + `, f.id, f.requested_at from follows f join users u on u.id = f.followee_id
		where f.follower_id = $1 and f.status = 'pending' and not ` + blockedEither("$1::uuid", "u.id") + `
		order by f.requested_at desc, f.id desc`); err != nil {
		return social.Lists{}, err
	}
	return out, nil
}

// FollowerCount implements social.Repository.
func (r *SocialRepo) FollowerCount(ctx context.Context, owner uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		select count(*) from follows f
		where f.followee_id = $1 and f.status = 'accepted' and not `+blockedEither("$1::uuid", "f.follower_id"), owner).Scan(&n)
	return n, err
}

// FollowingCount implements social.Repository.
func (r *SocialRepo) FollowingCount(ctx context.Context, viewer uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		select count(*) from follows f
		where f.follower_id = $1 and f.status = 'accepted' and not `+blockedEither("$1::uuid", "f.followee_id"), viewer).Scan(&n)
	return n, err
}

// Block implements social.Repository.
func (r *SocialRepo) Block(ctx context.Context, blocker, blocked uuid.UUID, at time.Time) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := pairLock(ctx, tx, blocker, blocked); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `select exists (select 1 from users where id = $1)`, blocked).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return social.ErrNotFound
		}
		if _, err := tx.Exec(ctx, `
			insert into blocks (blocker_id, blocked_id, created_at) values ($1, $2, $3)
			on conflict do nothing`, blocker, blocked, at); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			delete from follows
			where (follower_id = $1 and followee_id = $2) or (follower_id = $2 and followee_id = $1)`, blocker, blocked)
		return err
	})
}

// Unblock implements social.Repository.
func (r *SocialRepo) Unblock(ctx context.Context, blocker, blocked uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from blocks where blocker_id = $1 and blocked_id = $2`, blocker, blocked)
	return err
}

// Blocks implements social.Repository.
func (r *SocialRepo) Blocks(ctx context.Context, blocker uuid.UUID) ([]social.Blocked, error) {
	rows, err := r.pool.Query(ctx, `
		select `+personColumns+`, b.created_at from blocks b join users u on u.id = b.blocked_id
		where b.blocker_id = $1
		order by b.created_at desc, u.id`, blocker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []social.Blocked{}
	for rows.Next() {
		var at time.Time
		p, err := scanPerson(rows, &at)
		if err != nil {
			return nil, err
		}
		out = append(out, social.Blocked{Person: p, At: at.UTC()})
	}
	return out, rows.Err()
}

// Settings implements social.Repository.
func (r *SocialRepo) Settings(ctx context.Context, userID uuid.UUID) (social.Settings, error) {
	s := social.DefaultSettings
	err := r.pool.QueryRow(ctx, `
		select reactions, comments, milestones from notification_settings where user_id = $1`, userID).
		Scan(&s.Reactions, &s.Comments, &s.Milestones)
	if errors.Is(err, pgx.ErrNoRows) {
		return social.DefaultSettings, nil
	}
	return s, err
}

// SaveSettings implements social.Repository.
func (r *SocialRepo) SaveSettings(ctx context.Context, userID uuid.UUID, s social.Settings, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into notification_settings (user_id, reactions, comments, milestones, updated_at)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id) do update set reactions = excluded.reactions, comments = excluded.comments,
			milestones = excluded.milestones, updated_at = excluded.updated_at`,
		userID, s.Reactions, s.Comments, s.Milestones, at)
	return err
}

// ViewFacts implements social.Repository.
func (r *SocialRepo) ViewFacts(ctx context.Context, viewer, owner uuid.UUID, localDate string) (social.Facts, error) {
	var f social.Facts
	err := r.pool.QueryRow(ctx, `
		select
			exists (select 1 from follows where follower_id = $1 and followee_id = $2 and status = 'accepted'),
			`+blockedEither("$1::uuid", "$2::uuid")+`,
			exists (select 1 from posted_days where user_id = $1 and local_date = $3::date)`,
		viewer, owner, localDate).Scan(&f.Follows, &f.Blocked, &f.ViewerPostedDay)
	return f, err
}
