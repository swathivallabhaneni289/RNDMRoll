package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/secondlook"
)

// SecondLookRepo stores the owners' requests for a second look at what an
// admin removed (migration 0021's second_looks).
type SecondLookRepo struct {
	pool *pgxpool.Pool
}

// NewSecondLookRepo builds a SecondLookRepo backed by pool.
func NewSecondLookRepo(pool *pgxpool.Pool) *SecondLookRepo {
	return &SecondLookRepo{pool: pool}
}

var _ secondlook.Repository = (*SecondLookRepo)(nil)

// proofKeyHeld is a where fragment true when the photo file named by the
// text column or value col is a second look's proof: that file is the
// request's alone, so no post, comment or profile photo may take it, and no
// delete of one may delete it.
func proofKeyHeld(col string) string {
	return "exists (select 1 from second_looks slk where slk.photo_keys && array[" + col + "]::text[])"
}

// Ask implements secondlook.Repository, in one transaction:
//  1. the owner's requests take turns (a lock on the owner for the
//     transaction), so two sends never both find the removal unasked, nor a
//     file free that the other takes;
//  2. the removal is read: the owner's, and a removal of a post or comment;
//  3. the post or comment is held for share. A foreign key's key share
//     would not do: it lets Bring it back's update through. Held for
//     share, a Bring it back or the purge under way finishes first, and the
//     reads below see what it did, or waits until this is done;
//  4. what stands now is read: a request about the removal (the same one
//     again is the phone's retry, made false), a Bring it back or a newer
//     removal since, the 30 days, on the database's clock (the purge's
//     line), and the post or comment still removed;
//  5. no photo row, comment or other request may use one of the files: one
//     file, one row. A profile photo never could: its key is avatars/ by
//     0019's check, and the service takes only entries/ keys;
//  6. the request is stored, with the post (for a comment, its post) and
//     the comment, which go to null once the purge deletes them.
func (r *SecondLookRepo) Ask(ctx context.Context, owner, removal uuid.UUID, body string, keys []string, at time.Time) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('second_look:' || $1::text, 0))`, owner); err != nil {
		return false, err
	}
	var kind string
	var item uuid.UUID
	var removedAt time.Time
	err = tx.QueryRow(ctx, `select item, item_id, created_at from moderation_events
		where id = $1 and owner_id = $2 and action = '`+admin.LogRemoved+`' and item in ('post', 'comment')`,
		removal, owner).Scan(&kind, &item, &removedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, secondlook.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	m := moderatedItems[kind]
	var held int
	err = tx.QueryRow(ctx, `select 1 from `+m.table+` where id = $1 and user_id = $2 for share`, item, owner).Scan(&held)
	there := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	// Read after the hold, so a Bring it back or a purge it waited for is
	// seen.
	var asked bool
	var askedBody string
	var askedKeys []string
	var answered, past, removed, inside bool
	var entry *uuid.UUID
	err = tx.QueryRow(ctx, `
		select sl.id is not null, coalesce(sl.body, ''), coalesce(sl.photo_keys, '{}'),
		  exists (select 1 from moderation_events later where later.owner_id = $2 and later.item_id = $3
		          and later.action in ('`+admin.LogRemoved+`', '`+admin.LogRestored+`') and later.created_at > $4),
		  `+pastRemovalGrace("$4::timestamptz")+`,
		  coalesce((select x.removed_at is not null from `+m.table+` x where x.id = $3 and x.user_id = $2), false),
		  coalesce((select x.removed_at is not null and `+insideRemovalGrace("x.removed_at")+` from `+m.table+` x
		            where x.id = $3 and x.user_id = $2), false),
		  (select x.`+m.entry+` from `+m.table+` x where x.id = $3 and x.user_id = $2)
		from (select 1) one
		left join second_looks sl on sl.removal_id = $1`,
		removal, owner, item, removedAt).Scan(&asked, &askedBody, &askedKeys, &answered, &past, &removed, &inside, &entry)
	if err != nil {
		return false, err
	}
	switch {
	case asked && askedBody == body && slices.Equal(askedKeys, keys):
		return false, nil
	case asked:
		return false, secondlook.ErrAsked
	case answered:
		return false, secondlook.ErrBack
	case past:
		return false, secondlook.ErrTooLate
	case !there:
		return false, secondlook.ErrNotFound
	case !removed:
		return false, secondlook.ErrBack
	case !inside:
		return false, secondlook.ErrTooLate
	}

	if len(keys) > 0 {
		var used bool
		if err := tx.QueryRow(ctx, `select
			exists (select 1 from entry_photos where storage_key = any($1::text[])) or
			exists (select 1 from post_comments where photo_key = any($1::text[])) or
			exists (select 1 from second_looks where photo_keys && $1::text[])`, keys).Scan(&used); err != nil {
			return false, err
		}
		if used {
			return false, &secondlook.FieldProblem{Field: "photo_keys", Reason: "invalid"}
		}
	}
	var comment *uuid.UUID
	if kind == admin.KindComment {
		comment = &item
	}
	_, err = tx.Exec(ctx, `insert into second_looks (owner_id, removal_id, item, item_id, entry_id, comment_id, body, photo_keys, at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, owner, removal, kind, item, entry, comment, body, keys, at)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return false, secondlook.ErrAsked
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// keysQuerier is a transaction or the pool.
type keysQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// withoutProofKeys is keys, in order, less those a second look holds
// (proofKeyHeld): a delete of a post's photos leaves those files alone. The
// checks on both sides keep a file to one row, but they are separate, so two
// hand-made requests at the same moment could still share one.
func withoutProofKeys(ctx context.Context, q keysQuerier, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return keys, nil
	}
	held, err := collect[string](q.Query(ctx, `select k from unnest($1::text[]) k where `+proofKeyHeld("k"), keys))
	if err != nil {
		return nil, err
	}
	if len(held) == 0 {
		return keys, nil
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !slices.Contains(held, k) {
			out = append(out, k)
		}
	}
	return out, nil
}
