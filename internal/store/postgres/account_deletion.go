package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// A deletion the person asked for (migration 0018) is judged against one
// line, graceCutoff: now on the database's clock less user.DeletionGracePeriod.
// An ask made after the line is inside the grace period: log in and a Forgot
// password reset clear it, and Forgot password still sends codes. An ask made
// at the line or before it is past the grace period: the purge may delete the
// account, and nothing restores it. insideGrace and pastGrace are each
// other's exact opposite over the same expression, and the line only moves
// forward, so no account is ever on both sides, and one that is past stays
// past. The purge also holds the row while it works (PurgeRepo.Purge), so a
// restore that read the line just before it crossed waits for the purge and
// then finds no account.
//
// Every rule reads now(), never a time sent from Go: the spin service's
// clock is shifted per account in test mode, and the server's clock and the
// database's need not agree.
var (
	graceInterval = fmt.Sprintf("interval '%d seconds'", int64(user.DeletionGracePeriod/time.Second))
	graceCutoff   = "(now() - " + graceInterval + ")"
)

// insideGrace is a where fragment true when the deletion_requested_at
// column col is set and inside the grace period.
func insideGrace(col string) string { return "(" + col + " > " + graceCutoff + ")" }

// pastGrace is a where fragment true when col is set and past the grace
// period.
func pastGrace(col string) string { return "(" + col + " <= " + graceCutoff + ")" }

// goneAccount is a where fragment true when the users row under alias is
// gone for everyone else: an admin disabled it (migration 0012), or its owner
// asked for it to be deleted (migration 0018), inside the grace period or
// past it. Every read that hides a disabled account from other people hides
// an account being deleted the same way.
func goneAccount(alias string) string {
	return "(" + alias + ".disabled_at is not null or " + alias + ".deletion_requested_at is not null)"
}

var _ user.AccountDeleter = (*UserRepo)(nil)

// RequestDeletion implements user.AccountDeleter. One transaction, in the
// order of session_locks.go: the Forgot password code goes first, then the
// account is marked (only one that is not disabled and not already marked),
// then every refresh token is revoked, twice as in a password change
// (UserRepo.ChangePassword says why), and the push tokens go last. So the
// account is logged out everywhere at once, and its phones stop getting
// pushes. The end of the grace period is worked out by the database from
// the moment it stored, so the answer and every later rule agree.
func (r *UserRepo) RequestDeletion(ctx context.Context, userID uuid.UUID) (user.DeletionRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return user.DeletionRequest{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, `delete from password_resets where user_id = $1`, userID); err != nil {
		return user.DeletionRequest{}, err
	}
	var out user.DeletionRequest
	err = tx.QueryRow(ctx, `
		update users set deletion_requested_at = now()
		where id = $1 and deletion_requested_at is null and disabled_at is null
		returning deletion_requested_at + `+graceInterval+`,
			coalesce((select z.tz_name from user_time_zones z where z.user_id = $1), '')`,
		userID).Scan(&out.DeleteAfter, &out.Zone)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.DeletionRequest{}, unchangedAccount(ctx, tx, userID)
	}
	if err != nil {
		return user.DeletionRequest{}, err
	}
	for range 2 {
		if _, err := tx.Exec(ctx, revokeAllQ, userID); err != nil {
			return user.DeletionRequest{}, err
		}
	}
	if _, err := tx.Exec(ctx, deletePushTokensQ, userID); err != nil {
		return user.DeletionRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return user.DeletionRequest{}, err
	}
	out.DeleteAfter = out.DeleteAfter.UTC()
	return out, nil
}

// CancelDeletion implements user.AccountDeleter in one statement, guarded by
// the grace period. Its row lock waits for a purge of the account in
// progress, which then leaves no row to restore.
func (r *UserRepo) CancelDeletion(ctx context.Context, userID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		update users set deletion_requested_at = null
		where id = $1 and disabled_at is null and `+insideGrace("deletion_requested_at"), userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// PurgeRepo is purge.Store over the pool: the accounts whose grace period is
// over, and their deletion for good.
type PurgeRepo struct {
	pool *pgxpool.Pool
}

// NewPurgeRepo builds a PurgeRepo backed by pool.
func NewPurgeRepo(pool *pgxpool.Pool) *PurgeRepo {
	return &PurgeRepo{pool: pool}
}

var _ purge.Store = (*PurgeRepo)(nil)

// Due implements purge.Store: up to limit accounts past the grace period,
// the oldest ask first.
func (r *PurgeRepo) Due(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		select id from users where `+pastGrace("deletion_requested_at")+`
		order by deletion_requested_at, id
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Purge implements purge.Store in one transaction. The Forgot password code
// goes first, as every session write takes the tables (session_locks.go).
// Then the account's row is held for update until the end, and only while it
// is still past the grace period: a log in or a reset that would restore it
// waits, then finds no account, and one that restored it first leaves
// nothing to hold here (purged false, and the code comes back with the
// rollback). Holding it for update, not for no key update as other session
// writes do, also makes any write that would name the account wait and then
// fail: nothing new may point at an account being deleted.
//
// The photos of other people's comments that the cascade will delete are
// read under that hold, deleteFiles deletes them and the account's own
// folders, and only when it succeeded does the row go. A failure leaves
// everything as it was, apart from files already deleted, and the next run
// starts again.
func (r *PurgeRepo) Purge(ctx context.Context, id uuid.UUID, deleteFiles func(ctx context.Context, photos []purge.CommentPhoto) error) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, `delete from password_resets where user_id = $1`, id); err != nil {
		return false, err
	}
	var held uuid.UUID
	err = tx.QueryRow(ctx, `select id from users where id = $1 and `+pastGrace("deletion_requested_at")+` for update`, id).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	photos, err := commentPhotosGoingWith(ctx, tx, id)
	if err != nil {
		return false, err
	}
	if err := deleteFiles(ctx, photos); err != nil {
		return false, err
	}

	// The cascades from users take everything else: posts and their photos'
	// rows, comments, reactions, follows, blocks, mutes, reports, spins,
	// sessions and push tokens.
	tag, err := tx.Exec(ctx, `delete from users where id = $1 and `+pastGrace("deletion_requested_at"), id)
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

// commentPhotosGoingWith lists the photos of OTHER people's comments that the
// cascade from the account's users row deletes: their comments and replies
// on its posts (post_comments.entry_id cascades from entries), and their
// replies under its comments on anyone's post (post_comments.parent_id
// cascades, migration 0009). Replies are one level deep, so that is all of
// them. Their files are in their writers' own folders, which the purge
// otherwise never touches; the account's own photos are in its own folders,
// which it deletes whole. A file that one of the writer's own posts also
// names is left alone: the comment insert refuses a post's key, but the post
// side does not check comment keys, so a hand-made request could share one.
func commentPhotosGoingWith(ctx context.Context, tx pgx.Tx, id uuid.UUID) ([]purge.CommentPhoto, error) {
	rows, err := tx.Query(ctx, `
		select c.photo_key, c.user_id from post_comments c
		where c.photo_key is not null and c.user_id <> $1
		  and (c.entry_id in (select e.id from entries e where e.user_id = $1)
		       or c.parent_id in (select p.id from post_comments p where p.user_id = $1))
		  and not exists (select 1 from entry_photos ep where ep.storage_key = c.photo_key)
		order by c.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []purge.CommentPhoto{}
	for rows.Next() {
		var p purge.CommentPhoto
		if err := rows.Scan(&p.Key, &p.Author); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
