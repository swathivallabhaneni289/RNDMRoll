package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
)

// NotificationsRepo is notifications.Repository over the pool.
type NotificationsRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationsRepo builds a NotificationsRepo backed by pool.
func NewNotificationsRepo(pool *pgxpool.Pool) *NotificationsRepo {
	return &NotificationsRepo{pool: pool}
}

var _ notifications.Repository = (*NotificationsRepo)(nil)

// Reactions implements notifications.Repository.
func (r *NotificationsRepo) Reactions(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	rows, err := r.pool.Query(ctx, `
		select pr.entry_id, pr.user_id, pr.reaction, pr.updated_at, s.category_name
		from entries e
		join post_reactions pr on pr.entry_id = e.id
		join spins s on s.id = e.spin_id
		where e.user_id = $1 and pr.user_id <> $1 and pr.updated_at > $2
		  and not `+blockedEither("$1::uuid", "pr.user_id")+`
		order by pr.updated_at desc, pr.entry_id desc, pr.user_id desc
		limit $3`, owner, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Raw{}
	for rows.Next() {
		var entryID, actor uuid.UUID
		var reaction, category string
		var at time.Time
		if err := rows.Scan(&entryID, &actor, &reaction, &at, &category); err != nil {
			return nil, err
		}
		out = append(out, notifications.Raw{
			Kind: notifications.KindReaction, ID: "reaction:" + entryID.String() + ":" + actor.String(),
			At: at.UTC(), ActorID: actor, EntryID: &entryID, CategoryName: &category, Reaction: &reaction,
		})
	}
	return out, rows.Err()
}

// Comments implements notifications.Repository.
func (r *NotificationsRepo) Comments(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	rows, err := r.pool.Query(ctx, `
		select c.id, c.entry_id, c.user_id, c.body, c.created_at, s.category_name
		from entries e
		join post_comments c on c.entry_id = e.id
		join spins s on s.id = e.spin_id
		where e.user_id = $1 and c.user_id <> $1 and c.created_at > $2
		  and not `+blockedEither("$1::uuid", "c.user_id")+`
		order by c.created_at desc, c.id desc
		limit $3`, owner, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Raw{}
	for rows.Next() {
		var id, entryID, actor uuid.UUID
		var body, category string
		var at time.Time
		if err := rows.Scan(&id, &entryID, &actor, &body, &at, &category); err != nil {
			return nil, err
		}
		text := notifications.CutText(body, notifications.TextRunes)
		out = append(out, notifications.Raw{
			Kind: notifications.KindComment, ID: "comment:" + id.String(),
			At: at.UTC(), ActorID: actor, EntryID: &entryID, CategoryName: &category, Text: &text,
		})
	}
	return out, rows.Err()
}

// followRows reads (follow id, other person, instant) rows into items of
// kind, with the item id kind:<other person>, and the request id when
// withRequest.
func (r *NotificationsRepo) followRows(ctx context.Context, kind string, withRequest bool, q string, args ...any) ([]notifications.Raw, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Raw{}
	for rows.Next() {
		var id, other uuid.UUID
		var at time.Time
		if err := rows.Scan(&id, &other, &at); err != nil {
			return nil, err
		}
		raw := notifications.Raw{Kind: kind, ID: kind + ":" + other.String(), At: at.UTC(), ActorID: other}
		if withRequest {
			reqID := id
			raw.ID = kind + ":" + id.String()
			raw.RequestID = &reqID
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

// Follows implements notifications.Repository.
func (r *NotificationsRepo) Follows(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.followRows(ctx, notifications.KindFollow, false, `
		select f.id, f.follower_id, f.accepted_at from follows f
		where f.followee_id = $1 and f.status = 'accepted' and f.via = 'direct' and f.accepted_at > $2
		  and not `+blockedEither("$1::uuid", "f.follower_id")+`
		order by f.accepted_at desc, f.id desc
		limit $3`, owner, since, limit)
}

// Requests implements notifications.Repository.
func (r *NotificationsRepo) Requests(ctx context.Context, owner uuid.UUID, limit int) ([]notifications.Raw, error) {
	return r.followRows(ctx, notifications.KindFollowRequest, true, `
		select f.id, f.follower_id, f.requested_at from follows f
		where f.followee_id = $1 and f.status = 'pending'
		  and not `+blockedEither("$1::uuid", "f.follower_id")+`
		order by f.requested_at desc, f.id desc
		limit $2`, owner, limit)
}

// Accepted implements notifications.Repository.
func (r *NotificationsRepo) Accepted(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.followRows(ctx, notifications.KindFollowAccepted, false, `
		select f.id, f.followee_id, f.accepted_at from follows f
		where f.follower_id = $1 and f.status = 'accepted' and f.via = 'request' and f.accepted_at > $2
		  and not `+blockedEither("$1::uuid", "f.followee_id")+`
		order by f.accepted_at desc, f.id desc
		limit $3`, viewer, since, limit)
}

// Bonuses implements notifications.Repository.
func (r *NotificationsRepo) Bonuses(ctx context.Context, viewer uuid.UUID, since time.Time) ([]notifications.Bonus, error) {
	rows, err := r.pool.Query(ctx, `
		select week_start::text, goal, goal_met_at from goal_weeks
		where user_id = $1 and bonus_granted and goal_met_at > $2
		order by goal_met_at desc`, viewer, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Bonus{}
	for rows.Next() {
		var b notifications.Bonus
		if err := rows.Scan(&b.WeekStart, &b.Goal, &b.At); err != nil {
			return nil, err
		}
		b.At = b.At.UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}

// Seen implements notifications.Repository.
func (r *NotificationsRepo) Seen(ctx context.Context, viewer uuid.UUID) (time.Time, bool, error) {
	var at time.Time
	err := r.pool.QueryRow(ctx, `select seen_at from notification_seen where user_id = $1`, viewer).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}

// MarkSeen implements notifications.Repository.
func (r *NotificationsRepo) MarkSeen(ctx context.Context, viewer uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into notification_seen (user_id, seen_at) values ($1, $2)
		on conflict (user_id) do update set seen_at = greatest(notification_seen.seen_at, excluded.seen_at)`, viewer, at)
	return err
}
