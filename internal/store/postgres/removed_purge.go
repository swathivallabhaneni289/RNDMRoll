package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
)

// RemovedPurgeRepo is purge.RemovedStore over the pool: the posts and
// comments an admin removed whose admin.RemovalGrace is over, and their
// deletion for good. It is never the owner's delete (spin.Service), which
// needs the owner's zone and lock and deletes the row before the files.
type RemovedPurgeRepo struct {
	pool *pgxpool.Pool
}

// NewRemovedPurgeRepo builds a RemovedPurgeRepo backed by pool.
func NewRemovedPurgeRepo(pool *pgxpool.Pool) *RemovedPurgeRepo {
	return &RemovedPurgeRepo{pool: pool}
}

var _ purge.RemovedStore = (*RemovedPurgeRepo)(nil)

// removedTables are the tables of the two kinds the purge deletes.
var removedTables = map[string]string{admin.KindPost: "entries", admin.KindComment: "post_comments"}

// Due implements purge.RemovedStore: up to limit removed posts and comments
// past the line (pastRemovalGrace, the exact opposite of the line Bring it
// back and the Removed tab use), the oldest removal first.
func (r *RemovedPurgeRepo) Due(ctx context.Context, limit int) ([]purge.Removed, error) {
	rows, err := r.pool.Query(ctx, `
		select kind, id from (
		  select 'post' as kind, id, removed_at from entries where `+pastRemovalGrace("removed_at")+`
		  union all
		  select 'comment', id, removed_at from post_comments where `+pastRemovalGrace("removed_at")+`
		) x
		order by removed_at, kind, id
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []purge.Removed{}
	for rows.Next() {
		var it purge.Removed
		if err := rows.Scan(&it.Kind, &it.ID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Purge implements purge.RemovedStore in one transaction. The post or
// comment is held for update until the end, and only while it is still
// removed and past the line: a Bring it back that read the line just before
// it crossed waits for this, then finds no row, and one that brought it
// back first leaves nothing to hold here (purged false). Holding it also
// makes a comment or a reaction that would name it wait and then fail.
//
// Under that hold its photos are read with the rules of the owner's delete
// (pgTx.DeleteEntry): for a post, its own photos, then those of the comments
// and replies on it; for a comment, its own photo and its replies'. A
// comment's file that a photo row also names is left out, since that post
// still shows it. Nothing here asks whether a comment is removed or hidden:
// the cascade takes them all, so their files must go too. deleteFiles
// deletes the files, and only when it succeeded does the row go, with the
// cascades taking its photo rows, comments, replies and reactions, and
// setting the reports' and the decision log's links to null (the log keeps
// item_id, so Removed before still counts it). A failure leaves everything
// as it was, apart from files already deleted, and the next run starts
// again.
func (r *RemovedPurgeRepo) Purge(ctx context.Context, item purge.Removed, deleteFiles func(ctx context.Context, photos []purge.Photo) error) (bool, error) {
	table, ok := removedTables[item.Kind]
	if !ok {
		return false, fmt.Errorf("postgres: no removed %q to purge", item.Kind)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var owner uuid.UUID
	err = tx.QueryRow(ctx, `select user_id from `+table+`
		where id = $1 and `+pastRemovalGrace("removed_at")+` for update`, item.ID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var photos []purge.Photo
	if item.Kind == admin.KindPost {
		photos, err = collectPhotos(tx.Query(ctx, `
			select storage_key, user_id from entry_photos
			where entry_id = $1 and user_id = $2
			order by position`, item.ID, owner))
		if err != nil {
			return false, err
		}
		comments, err := collectPhotos(tx.Query(ctx, commentPhotosWhere(
			`c.entry_id = $1 or c.parent_id in (select p.id from post_comments p where p.entry_id = $1)`), item.ID))
		if err != nil {
			return false, err
		}
		photos = append(photos, comments...)
	} else {
		photos, err = collectPhotos(tx.Query(ctx, commentPhotosWhere(`c.id = $1 or c.parent_id = $1`), item.ID))
		if err != nil {
			return false, err
		}
	}
	if err := deleteFiles(ctx, photos); err != nil {
		return false, err
	}

	tag, err := tx.Exec(ctx, `delete from `+table+` where id = $1 and `+pastRemovalGrace("removed_at"), item.ID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() != 1 {
		return false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// commentPhotosWhere reads (photo key, writer) of the comments matching
// where (over post_comments as c, with the item's id as $1) whose file no
// photo row names, the oldest first: the rule commentPhotoKeysOf and
// commentPhotosGoingWith use.
func commentPhotosWhere(where string) string {
	return `
		select c.photo_key, c.user_id from post_comments c
		where c.photo_key is not null and (` + where + `)
		  and not exists (select 1 from entry_photos ep where ep.storage_key = c.photo_key)
		order by c.created_at, c.id`
}

// collectPhotos reads (key, writer) rows, closing them.
func collectPhotos(rows pgx.Rows, err error) ([]purge.Photo, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []purge.Photo{}
	for rows.Next() {
		var p purge.Photo
		if err := rows.Scan(&p.Key, &p.Writer); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
