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

// Comments implements notifications.Repository. A reply counts as a comment
// for the post's owner while its top-level comment is shown to the owner
// (no block with its writer), and not at all when the owner wrote that
// top-level comment: the owner then gets the reply item only.
func (r *NotificationsRepo) Comments(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.commentRows(ctx, notifications.KindComment, `
		select c.id, c.entry_id, c.user_id, c.body, c.created_at, s.category_name,
		  (select ep.storage_key from entry_photos ep where ep.entry_id = e.id order by ep.position limit 1)
		from entries e
		join post_comments c on c.entry_id = e.id
		left join post_comments p on p.id = c.parent_id
		join spins s on s.id = e.spin_id
		where e.user_id = $1 and c.user_id <> $1 and c.created_at > $2
		  and not `+blockedEither("$1::uuid", "c.user_id")+`
		  and (c.parent_id is null or (p.user_id <> $1 and not `+blockedEither("$1::uuid", "p.user_id")+`))
		order by c.created_at desc, c.id desc
		limit $3`, owner, since, limit)
}

// Replies implements notifications.Repository: replies by someone else to a
// top-level comment the viewer wrote, with no block between the viewer and
// the reply's writer, on a post the viewer can still see (the rule of
// social.CanView: the viewer's own, or a followed owner's with no block, not
// archived, on a day the viewer posted).
func (r *NotificationsRepo) Replies(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.commentRows(ctx, notifications.KindReply, `
		select c.id, c.entry_id, c.user_id, c.body, c.created_at, s.category_name,
		  (select ep.storage_key from entry_photos ep where ep.entry_id = e.id order by ep.position limit 1)
		from post_comments p
		join post_comments c on c.parent_id = p.id
		join entries e on e.id = c.entry_id
		join spins s on s.id = e.spin_id
		where p.user_id = $1 and c.user_id <> $1 and c.created_at > $2
		  and not `+blockedEither("$1::uuid", "c.user_id")+`
		  and (e.user_id = $1 or (
		    e.archived_at is null
		    and exists (select 1 from follows f where f.follower_id = $1 and f.followee_id = e.user_id and f.status = 'accepted')
		    and not `+blockedEither("$1::uuid", "e.user_id")+`
		    and exists (select 1 from posted_days pd where pd.user_id = $1 and pd.local_date = e.local_date)))
		order by c.created_at desc, c.id desc
		limit $3`, viewer, since, limit)
}

// commentRows reads (comment id, entry, writer, body, instant, category,
// cover photo key or null) rows into items of kind with the id kind:<comment id> and the text cut.
func (r *NotificationsRepo) commentRows(ctx context.Context, kind string, q string, args ...any) ([]notifications.Raw, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Raw{}
	for rows.Next() {
		var id, entryID, actor uuid.UUID
		var body, category string
		var photoKey *string
		var at time.Time
		if err := rows.Scan(&id, &entryID, &actor, &body, &at, &category, &photoKey); err != nil {
			return nil, err
		}
		// A comment that is only a photo has no text: the app then says
		// "sent a photo." (migration 0013).
		var textp *string
		if text := notifications.CutText(body, notifications.TextRunes); text != "" {
			textp = &text
		}
		out = append(out, notifications.Raw{
			Kind: kind, ID: kind + ":" + id.String(),
			At: at.UTC(), ActorID: actor, EntryID: &entryID, CategoryName: &category, Text: textp,
			PhotoKey: photoKey,
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

// Follows implements notifications.Repository: a follow made at once, and a
// request the owner accepted (developer, 2026-10-17, like Instagram: the
// request row becomes "started following you"). An accepted request keeps
// the time it was asked, so the owner's own Accept never lights the dot.
func (r *NotificationsRepo) Follows(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.followRows(ctx, notifications.KindFollow, false, `
		select id, follower_id, at from (
		  select f.id, f.follower_id,
		    case when f.via = 'request' then f.requested_at else f.accepted_at end as at
		  from follows f
		  where f.followee_id = $1 and f.status = 'accepted' and f.via in ('direct', 'request')
		    and not `+blockedEither("$1::uuid", "f.follower_id")+`
		) x
		where at > $2
		order by at desc, id desc
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
