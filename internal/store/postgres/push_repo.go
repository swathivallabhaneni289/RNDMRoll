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

// PushRepo is push.Repository over the pool (migration 0015).
type PushRepo struct {
	pool *pgxpool.Pool
}

// NewPushRepo builds a PushRepo backed by pool.
func NewPushRepo(pool *pgxpool.Pool) *PushRepo {
	return &PushRepo{pool: pool}
}

var _ push.Repository = (*PushRepo)(nil)

// SaveToken implements push.Repository. A token already kept for another
// account moves to this one: the phone now belongs to whoever signed in.
func (r *PushRepo) SaveToken(ctx context.Context, userID uuid.UUID, token, platform string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into push_tokens (user_id, token, platform, created_at, last_seen_at)
		values ($1, $2, $3, $4, $4)
		on conflict (token) do update set
		  user_id = excluded.user_id, platform = excluded.platform, last_seen_at = excluded.last_seen_at,
		  created_at = case when push_tokens.user_id = excluded.user_id then push_tokens.created_at else excluded.created_at end`,
		userID, token, platform, at)
	return err
}

// DeleteToken implements push.Repository.
func (r *PushRepo) DeleteToken(ctx context.Context, userID uuid.UUID, token string) error {
	_, err := r.pool.Exec(ctx, `delete from push_tokens where user_id = $1 and token = $2`, userID, token)
	return err
}

// Tokens implements push.Repository. A disabled account gets nothing.
func (r *PushRepo) Tokens(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		select t.token from push_tokens t join users u on u.id = t.user_id
		where t.user_id = $1 and u.disabled_at is null
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
	_, err := r.pool.Exec(ctx, `delete from push_tokens where token = any($1::text[])`, tokens)
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
