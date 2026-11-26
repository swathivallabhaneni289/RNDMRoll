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
// (no block with its writer, and nothing moderation keeps from them), and
// not at all when the owner wrote that top-level comment: the owner then
// gets the reply item only. Nothing on a removed post is listed, and nothing
// the owner reported or that is under review.
func (r *NotificationsRepo) Comments(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.commentRows(ctx, notifications.KindComment, `
		select c.id, c.entry_id, c.user_id, c.body, c.created_at, s.category_name,
		  (select ep.storage_key from entry_photos ep where ep.entry_id = e.id order by ep.position limit 1)
		from entries e
		join post_comments c on c.entry_id = e.id
		left join post_comments p on p.id = c.parent_id
		join spins s on s.id = e.spin_id
		where e.user_id = $1 and c.user_id <> $1 and c.created_at > $2
		  and `+postShownTo("$1::uuid", "e")+`
		  and not `+blockedEither("$1::uuid", "c.user_id")+` and `+commentShownTo("$1::uuid", "c")+`
		  and (c.parent_id is null or (p.user_id <> $1 and not `+blockedEither("$1::uuid", "p.user_id")+`
		    and `+commentShownTo("$1::uuid", "p")+`))
		order by c.created_at desc, c.id desc
		limit $3`, owner, since, limit)
}

// Replies implements notifications.Repository: replies by someone else to a
// top-level comment the viewer wrote, with no block between the viewer and
// the reply's writer, that the viewer may read (not hidden by the post's
// owner, unless the viewer is that owner: shownComment, as on the post
// page; and nothing moderation keeps from them, on the reply or on their own
// comment), on a post the viewer can still see (social.CanView,
// postVisibleTo).
func (r *NotificationsRepo) Replies(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	return r.commentRows(ctx, notifications.KindReply, `
		select c.id, c.entry_id, c.user_id, c.body, c.created_at, s.category_name,
		  (select ep.storage_key from entry_photos ep where ep.entry_id = e.id order by ep.position limit 1)
		from post_comments p
		join post_comments c on c.parent_id = p.id
		join entries e on e.id = c.entry_id
		join spins s on s.id = e.spin_id
		where p.user_id = $1 and c.user_id <> $1 and c.created_at > $2
		  and not `+blockedEither("$1::uuid", "c.user_id")+` and `+shownComment("c", "e")+`
		  and `+commentShownTo("$1::uuid", "c")+` and `+commentShownTo("$1::uuid", "p")+`
		  and `+postVisibleTo("$1::uuid", "e")+`
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

// Likes implements notifications.Repository by the rules of Comments:
// nothing on a post an admin removed (postShownTo), and no like by anyone
// with a block either way, whose account is gone or who no longer follows
// the owner (likeShownTo, the rule of the Likes list). Archived and
// under-review posts keep theirs, as they keep their comments. An undo
// deletes the row, so its item goes with it.
// The item id is like:<entry>:<actor>, and its time the like's last change.
func (r *NotificationsRepo) Likes(ctx context.Context, owner uuid.UUID, since time.Time, limit int) ([]notifications.Raw, error) {
	rows, err := r.pool.Query(ctx, `
		select pr.entry_id, pr.user_id, pr.updated_at, s.category_name,
		  (select ep.storage_key from entry_photos ep where ep.entry_id = e.id order by ep.position limit 1)
		from entries e
		join post_reactions pr on pr.entry_id = e.id
		join spins s on s.id = e.spin_id
		where e.user_id = $1 and pr.updated_at > $2
		  and `+postShownTo("$1::uuid", "e")+` and `+likeShownTo("$1::uuid", "pr")+`
		order by pr.updated_at desc, pr.entry_id desc, pr.user_id desc
		limit $3`, owner, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notifications.Raw{}
	for rows.Next() {
		var entryID, actor uuid.UUID
		var category string
		var photoKey *string
		var at time.Time
		if err := rows.Scan(&entryID, &actor, &at, &category, &photoKey); err != nil {
			return nil, err
		}
		out = append(out, notifications.Raw{
			Kind: notifications.KindLike, ID: notifications.KindLike + ":" + entryID.String() + ":" + actor.String(),
			At: at.UTC(), ActorID: actor, EntryID: &entryID, CategoryName: &category, PhotoKey: photoKey,
		})
	}
	return out, rows.Err()
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

// decisionsSQL is the decision log's rows about the viewer's ($1) own posts
// and comments (removed, restored, shown again, hidden by reports and kept
// removed after a second look; never an account's) after $2, the newest $3
// and every removal that can still be asked about, newest first by the
// viewer's own clock, with the reason the admin gave at a removal ("" on
// every other row). A removal carries its second look (migration 0021): the
// answer, or asked while there is none; else restored when a Bring it back
// came after it (by created_at, the database's clock, as whyNotBack reads
// the log), so there is nothing to ask; else closed when asking would be
// refused, by the lines SecondLookRepo.Ask holds to: the removal is past
// the grace, or its post or comment is deleted, not removed or past the
// grace; else none.
// Each carries its post while that is in the database, whether the viewer
// can see it now (postVisibleTo, the whole of social.CanView), and the
// post's first photo, read only for a post that is back or hidden (never
// removed or kept removed) and that the viewer can see. A post deleted since leaves no row to join, so the
// check says so outright rather than trust a NULL to fall through the
// fragments.
var decisionsSQL = `
	select x.id, x.item, x.action, x.at, x.snapshot, x.reason, x.second_look, x.entry, x.visible,
	  case when x.visible and x.item = 'post' and x.action not in ('removed', 'kept') then
	    (select ep.storage_key from entry_photos ep where ep.entry_id = x.entry order by ep.position limit 1) end
	from (
	  select me.id, me.item, me.action, me.at, me.snapshot, me.reason, e.id as entry,
	    (e.id is not null and ` + postVisibleTo("$1::uuid", "e") + `) as visible,
	    case when me.action = 'removed' then coalesce(
	      (select coalesce(sl.decision, 'asked') from second_looks sl where sl.removal_id = me.id),
	      case when exists (select 1 from moderation_events mr where mr.owner_id = me.owner_id and mr.item_id = me.item_id
	                        and mr.action = 'restored' and mr.created_at > me.created_at) then 'restored' end,
	      case when ` + pastRemovalGrace("me.created_at") + ` or not exists (
	          select 1 from entries mi where me.item = 'post' and mi.id = me.item_id and mi.user_id = me.owner_id
	            and ` + insideRemovalGrace("mi.removed_at") + `
	          union all
	          select 1 from post_comments mi where me.item = 'comment' and mi.id = me.item_id and mi.user_id = me.owner_id
	            and ` + insideRemovalGrace("mi.removed_at") + `) then 'closed' end,
	      '') else '' end as second_look,
	    row_number() over (order by me.at desc, me.id desc) as n
	  from moderation_events me
	  left join entries e on e.id = me.entry_id
	  where me.owner_id = $1 and me.item in ('post', 'comment') and me.action in ('removed', 'restored', 'shown_again', 'hidden', 'kept')
	    and me.at > $2
	) x
	where x.n <= $3 or (x.action = 'removed' and x.second_look = '')
	order by x.at desc, x.id desc`

// seenSQL is the viewer's ($1) seen time.
const seenSQL = `select seen_at from notification_seen where user_id = $1`

// scanDecisions reads decisionsSQL's rows and closes them.
func scanDecisions(rows pgx.Rows) ([]notifications.Decision, error) {
	defer rows.Close()
	out := []notifications.Decision{}
	for rows.Next() {
		var d notifications.Decision
		if err := rows.Scan(&d.ID, &d.Item, &d.Action, &d.At, &d.Snapshot, &d.Reason, &d.SecondLook, &d.EntryID, &d.Visible, &d.PhotoKey); err != nil {
			return nil, err
		}
		d.At = d.At.UTC()
		out = append(out, d)
	}
	return out, rows.Err()
}

// scanSeen reads seenSQL's row. No row is no seen time yet, not an error.
func scanSeen(row pgx.Row) (time.Time, bool, error) {
	var at time.Time
	err := row.Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}

// Decisions is DecisionsAndSeen's decision log read on its own
// (decisionsSQL).
func (r *NotificationsRepo) Decisions(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]notifications.Decision, error) {
	rows, err := r.pool.Query(ctx, decisionsSQL, viewer, since, limit)
	if err != nil {
		return nil, err
	}
	return scanDecisions(rows)
}

// DecisionsAndSeen implements notifications.Repository. Every list makes
// both reads whatever the switches, so they go as one batch: one round trip,
// plus one on a connection that has not prepared them yet (both are
// prepared together). The batch is read to the end before it returns.
func (r *NotificationsRepo) DecisionsAndSeen(ctx context.Context, viewer uuid.UUID, since time.Time, limit int) ([]notifications.Decision, time.Time, bool, error) {
	var decisions []notifications.Decision
	var seen time.Time
	var haveSeen bool
	b := &pgx.Batch{}
	b.Queue(decisionsSQL, viewer, since, limit).Query(func(rows pgx.Rows) (err error) {
		decisions, err = scanDecisions(rows)
		return err
	})
	b.Queue(seenSQL, viewer).QueryRow(func(row pgx.Row) (err error) {
		seen, haveSeen, err = scanSeen(row)
		return err
	})
	if err := r.pool.SendBatch(ctx, b).Close(); err != nil {
		return nil, time.Time{}, false, err
	}
	return decisions, seen, haveSeen, nil
}

// MarkSeen implements notifications.Repository.
func (r *NotificationsRepo) MarkSeen(ctx context.Context, viewer uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into notification_seen (user_id, seen_at) values ($1, $2)
		on conflict (user_id) do update set seen_at = greatest(notification_seen.seen_at, excluded.seen_at)`, viewer, at)
	return err
}
