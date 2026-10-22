package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/push"
)

// PushRepo is push.Repository over the pool (migrations 0015 and 0017).
type PushRepo struct {
	pool *pgxpool.Pool
}

// NewPushRepo builds a PushRepo backed by pool.
func NewPushRepo(pool *pgxpool.Pool) *PushRepo {
	return &PushRepo{pool: pool}
}

var _ push.Repository = (*PushRepo)(nil)

// SaveToken implements push.Repository. The token is tied to the session's
// current refresh token (currentSession), so a refresh that rotated it while
// this request was on its way does not leave the token tied to the old one.
// A token already kept moves to this account and session (the phone now
// belongs to whoever signed in on it), except when it stays where it is
// (staysBound): a late save from a session that has ended, from an access
// token that names none, or from an older live session of the person whose
// newer live session holds the token, never takes the phone's pushes from
// the one now signed in. A save from the newer session always takes it.
//
// Locks come in the order of session_locks.go: the session's refresh tokens
// (held for share by currentSession), then the push token's row. Whether the
// session the row is tied to is live is read only once the row is held, so
// a refresh that moved the row meanwhile is seen.
func (r *PushRepo) SaveToken(ctx context.Context, userID, session uuid.UUID, token, platform string, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	current, live, err := currentSession(ctx, tx, userID, session)
	if err != nil {
		return err
	}
	// A token deleted between the insert and the hold below is inserted again
	// on the next turn; a second turn is already rare.
	for range 3 {
		tag, err := tx.Exec(ctx, `
			insert into push_tokens (user_id, token, platform, created_at, last_seen_at, refresh_token_id)
			values ($1, $2, $3, $4, $4, $5)
			on conflict (token) do nothing`,
			userID, token, platform, at, current)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			return tx.Commit(ctx)
		}
		var bound *uuid.UUID
		err = tx.QueryRow(ctx, `select refresh_token_id from push_tokens where token = $1 for update`, token).Scan(&bound)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if bound != nil && (current == nil || *bound != *current) {
			stays, err := staysBound(ctx, tx, userID, *bound, current, live)
			if err != nil {
				return err
			}
			if stays {
				return tx.Commit(ctx)
			}
		}
		if _, err := tx.Exec(ctx, `
			update push_tokens set user_id = $1, platform = $3, last_seen_at = $4, refresh_token_id = $5,
			  created_at = case when user_id = $1 then created_at else $4 end
			where token = $2`,
			userID, token, platform, at, current); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return errors.New("postgres: push token kept being deleted while it was saved")
}

// currentSession follows session along its refreshes (rotated_to) to the
// refresh token it holds now, and keeps every row of the walk held (for
// share) until tx ends: a refresh rotating it meanwhile waits, then moves the
// saved token on (RefreshTokenRepo.Rotate), and nothing can end it while tx
// runs. live reports whether that token is live (not revoked, not expired). A
// session that has ended is still returned. It is nil, tying the token to
// nothing, when no session is named or it is not one of userID's.
func currentSession(ctx context.Context, tx pgx.Tx, userID, session uuid.UUID) (current *uuid.UUID, live bool, err error) {
	if session == uuid.Nil {
		return nil, false, nil
	}
	id := session
	for range maxChainSteps {
		var next *uuid.UUID
		err := tx.QueryRow(ctx, `
			select rotated_to, revoked_at is null and expires_at > now() from refresh_tokens
			where id = $1 and user_id = $2 for share`,
			id, userID).Scan(&next, &live)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if next == nil {
			return &id, live, nil
		}
		id = *next
	}
	return nil, false, nil
}

// staysBound reports whether a token tied to the session bound stays there
// rather than move to the saving session, current (live says whether that
// one is live). It stays when bound is live and current is not. It also
// stays, when both are live, if bound is the newer session of the same
// person: its refresh token was made later than current's. A person who
// signs in again on a phone (a log out that never reached the server leaves
// the old session live) uses the newer session, so a late save from the
// older one must not take its pushes. Plain reads, taking no lock: the
// caller holds the token's row, so bound cannot move meanwhile.
func staysBound(ctx context.Context, tx pgx.Tx, userID, bound uuid.UUID, current *uuid.UUID, live bool) (bool, error) {
	if !live {
		return sessionLive(ctx, tx, bound)
	}
	var newer bool
	err := tx.QueryRow(ctx, `
		select exists(select 1 from refresh_tokens b join refresh_tokens c on c.id = $2
		where b.id = $1 and b.user_id = $3 and c.user_id = $3
		  and b.revoked_at is null and b.expires_at > now()
		  and b.created_at > c.created_at)`,
		bound, *current, userID).Scan(&newer)
	return newer, err
}

// sessionLive reports whether the refresh token id is live: not revoked and
// not expired. It is a plain read, taking no lock.
func sessionLive(ctx context.Context, tx pgx.Tx, id uuid.UUID) (bool, error) {
	var live bool
	err := tx.QueryRow(ctx, `select exists(select 1 from refresh_tokens where id = $1 and revoked_at is null and expires_at > now())`,
		id).Scan(&live)
	return live, err
}

// DeleteToken implements push.Repository: the phone's token is deleted when
// it is userID's and tied to the caller's own session (where session leads
// now, as for SaveToken), to a session that has ended, or to none. A token
// tied to another live session stays: a DELETE from an older session that
// arrives after a newer sign in on the phone must not take that session's
// pushes. Locks come in the order of session_locks.go, as for SaveToken.
func (r *PushRepo) DeleteToken(ctx context.Context, userID, session uuid.UUID, token string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	own, _, err := currentSession(ctx, tx, userID, session)
	if err != nil {
		return err
	}
	var bound *uuid.UUID
	err = tx.QueryRow(ctx, `select refresh_token_id from push_tokens where user_id = $1 and token = $2 for update`,
		userID, token).Scan(&bound)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if bound != nil && (own == nil || *bound != *own) {
		live, err := sessionLive(ctx, tx, *bound)
		if err != nil {
			return err
		}
		if live {
			return nil
		}
	}
	if _, err := tx.Exec(ctx, `delete from push_tokens where token = $1`, token); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Tokens implements push.Repository: only tokens whose session is live. A
// log out revokes the session's refresh token, a password change revokes all
// of them (and deletes the tokens), and a session left alone expires; none of
// those needs the phone's DELETE to arrive. A disabled account gets nothing.
func (r *PushRepo) Tokens(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		select t.token from push_tokens t
		join users u on u.id = t.user_id
		join refresh_tokens s on s.id = t.refresh_token_id and s.user_id = t.user_id
		where t.user_id = $1 and u.disabled_at is null
		  and s.revoked_at is null and s.expires_at > now()
		order by t.last_seen_at desc
		limit 20`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RemoveTokens implements push.Repository.
func (r *PushRepo) RemoveTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `delete from push_tokens where id in (
		select id from push_tokens where token = any($1::text[]) order by id for update)`, tokens)
	return err
}

// CommentTargets implements push.Repository with the bell's rules
// (NotificationsRepo.Comments and Replies) for this one comment:
//   - the post's owner gets "comment" for a comment or a reply by someone
//     else with no block, except a reply under the owner's own comment or
//     under a writer the owner has a block with;
//   - the top-level comment's writer gets "reply" for a reply by someone else
//     with no block, on a post they can still see.
func (r *PushRepo) CommentTargets(ctx context.Context, commentID uuid.UUID) ([]push.Target, error) {
	rows, err := r.pool.Query(ctx, `
		select e.user_id, 'comment', c.user_id, c.entry_id, c.body, true
		from post_comments c
		join entries e on e.id = c.entry_id
		left join post_comments p on p.id = c.parent_id
		where c.id = $1 and c.user_id <> e.user_id
		  and not `+blockedEither("e.user_id", "c.user_id")+`
		  and (c.parent_id is null or (p.user_id <> e.user_id and not `+blockedEither("e.user_id", "p.user_id")+`))
		union all
		select p.user_id, 'reply', c.user_id, c.entry_id, c.body, e.user_id = p.user_id
		from post_comments c
		join post_comments p on p.id = c.parent_id
		join entries e on e.id = c.entry_id
		where c.id = $1 and c.user_id <> p.user_id
		  and not `+blockedEither("p.user_id", "c.user_id")+`
		  and (e.user_id = p.user_id or (
		    e.archived_at is null
		    and exists (select 1 from follows f where f.follower_id = p.user_id and f.followee_id = e.user_id and f.status = 'accepted')
		    and not `+blockedEither("p.user_id", "e.user_id")+`
		    and exists (select 1 from posted_days pd where pd.user_id = p.user_id and pd.local_date = e.local_date)))`,
		commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []push.Target{}
	for rows.Next() {
		var t push.Target
		if err := rows.Scan(&t.UserID, &t.Kind, &t.ActorID, &t.EntryID, &t.Body, &t.Own); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RequestPair implements push.Repository.
func (r *PushRepo) RequestPair(ctx context.Context, requestID uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	var follower, followee uuid.UUID
	err := r.pool.QueryRow(ctx, `select follower_id, followee_id from follows where id = $1`, requestID).Scan(&follower, &followee)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, uuid.UUID{}, false, nil
	}
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, false, err
	}
	return follower, followee, true, nil
}
