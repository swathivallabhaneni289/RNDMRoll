package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// The purge of what an admin removed (developer, 2026-11-17). A post or
// comment an admin removed is kept, hidden from everyone, for
// admin.RemovalGrace so Bring it back can undo it; after that this deletes
// it for good with its photos. It runs in the same loop as the accounts,
// after them, one item at a time, and in the same order as an account: the
// files first, then the row, which is the only record that the item is due.
// A failure leaves the row for the next run.

// Removed is a post or a comment an admin removed whose grace is over.
type Removed struct {
	// Kind is "post" or "comment".
	Kind string
	ID   uuid.UUID
}

// Photo is one file that goes with a removed post or comment, and the person
// whose folder it must be in: the post's owner for the post's own photos,
// the writer for a comment's or a reply's.
type Photo struct {
	Key    string
	Writer uuid.UUID
}

// RemovedStore is the database side of the purge of removed posts and
// comments (postgres.RemovedPurgeRepo).
type RemovedStore interface {
	// Due lists up to limit removed posts and comments whose grace is over,
	// the oldest removal first.
	Due(ctx context.Context, limit int) ([]Removed, error)
	// Purge holds the post or comment only while it is still removed and
	// its grace is over, hands deleteFiles the photos that go with it (its
	// own, and those of the comments and replies under it), and deletes its
	// row only when deleteFiles returned nil, all in one transaction. purged
	// is false when there was nothing to purge (it was brought back, or it is
	// gone). An error from deleteFiles is returned and the row stays.
	Purge(ctx context.Context, item Removed, deleteFiles func(ctx context.Context, photos []Photo) error) (purged bool, err error)
}

// defaultItemDeadline bounds the work on one removed post or comment: a
// post's photos and its comments' are a handful of files, and the row stays
// held while they go.
const defaultItemDeadline = time.Minute

// SetRemoved adds the purge of the posts and comments an admin removed to
// every run of the loop Start begins, after the accounts. One loop for both
// means the two never run at the same moment, so neither waits on rows the
// other holds (an account's cascade and its removed post, say). Call it
// before Start.
func (p *Purger) SetRemoved(store RemovedStore) {
	p.removed = store
	p.itemDeadline = defaultItemDeadline
}

// RunRemovedOnce purges the removed posts and comments that are due now, up
// to the batch, one at a time, as RunOnce does accounts, and stops before
// the next one once ctx is done. Each has its own deadline. It does nothing
// when no RemovedStore was set or ctx is already done, and a run with
// nothing due logs nothing.
func (p *Purger) RunRemovedOnce(ctx context.Context) Result {
	if p.removed == nil || ctx.Err() != nil {
		return Result{}
	}
	items, err := p.removed.Due(ctx, p.batch)
	if err != nil {
		p.logger.Warn("removed content purge could not list what is due; it tries again at the next run", "error", err)
		return Result{}
	}
	res := Result{Due: len(items)}
	for _, it := range items {
		if ctx.Err() != nil {
			break
		}
		purged, err := p.purgeRemoved(ctx, it)
		switch {
		case err != nil:
			res.Failed++
			p.logger.Warn("removed content purge failed; it is left for the next run", "kind", it.Kind, "id", it.ID.String(), "error", err)
		case purged:
			res.Purged++
		}
	}
	return res
}

// purgeRemoved deletes one removed post or comment inside the store's
// transaction: its files first, then its row. It reports whether the row
// was deleted.
func (p *Purger) purgeRemoved(ctx context.Context, it Removed) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.itemDeadline)
	defer cancel()

	var photos, skipped int
	purged, err := p.removed.Purge(ctx, it, func(ctx context.Context, list []Photo) error {
		// Counted afresh on every call, as in purge.
		photos, skipped = 0, 0
		for _, photo := range list {
			if !ownPhoto(photo) {
				skipped++
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := p.files.DeletePhoto(ctx, photo.Key); err != nil {
				return fmt.Errorf("delete a photo: %w", err)
			}
			photos++
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if purged {
		p.logger.Info("removed content purged", "kind", it.Kind, "id", it.ID.String(), "photos", photos, "skipped", skipped)
	}
	return purged, nil
}

// ownPhoto reports whether a photo of a removed post or comment may be
// deleted: its key must be an entry photo key in its writer's own folder,
// as deletable asks of an account's. The database's word alone is not
// enough to delete a file, so any other key (another person's file, or
// something that is not an entry photo at all) is skipped and never sent to
// storage; sent, storage would refuse it, and the item would fail on every
// run.
func ownPhoto(photo Photo) bool {
	return photo.Writer != uuid.Nil && storage.EntryPhotoKeyOwnedBy(photo.Key, photo.Writer)
}

// runRemovedSafely is one run of RunRemovedOnce that recovers a panic, as
// runSafely does for the accounts, so a panic in either run neither skips
// the other nor ends the loop.
func (p *Purger) runRemovedSafely(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("removed content purge panicked; the next run goes ahead as planned", "panic", r)
		}
	}()
	p.RunRemovedOnce(ctx)
}
