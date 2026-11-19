package purge

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// The sweep of second-look proof photos (developer, 2026-11-19). The owner of
// a post or comment an admin removed may add up to 3 photos when they ask
// for a second look. Those files are deleted 30 days after the removal,
// whatever the answer: on the purge's own line (admin.RemovalGrace from the
// removal), in the same loop, after the removed posts and comments, so a
// request about one the purge deletes loses its photos in the same run.
// Bring it back, the owner's own delete or another purge never reach them
// (the files are no row's but the request's), so this one sweep is what
// bounds them all. The request itself stays, with its words, for the
// History. A failure leaves the photos for the next run.

// ProofStore is the database side of the sweep (postgres.SecondLookPurgeRepo).
type ProofStore interface {
	// DueProofs lists up to limit second looks that still have photos and
	// whose removal is past the line, the oldest removal first.
	DueProofs(ctx context.Context, limit int) ([]uuid.UUID, error)
	// ClearProofs holds the request only while it still has photos and its
	// removal is past the line, hands deleteFiles its photos, each with its
	// owner as the writer, and empties its photo keys only when deleteFiles
	// returned nil, all in one transaction. cleared is false when there was
	// nothing to clear. An error from deleteFiles is returned and the keys
	// stay.
	ClearProofs(ctx context.Context, id uuid.UUID, deleteFiles func(ctx context.Context, photos []Photo) error) (cleared bool, err error)
}

// SetSecondLooks adds the sweep of second-look proof photos to every run of
// the loop Start begins, after the removed posts and comments. Call it
// before Start.
func (p *Purger) SetSecondLooks(store ProofStore) {
	p.proofs = store
	p.itemDeadline = defaultItemDeadline
}

// RunSecondLookPhotosOnce deletes the proof photos that are due now, up to
// the batch, one request at a time, and stops before the next one once ctx
// is done. Each has its own deadline. It does nothing when no ProofStore was
// set or ctx is already done, and a run with nothing due logs nothing.
func (p *Purger) RunSecondLookPhotosOnce(ctx context.Context) Result {
	if p.proofs == nil || ctx.Err() != nil {
		return Result{}
	}
	ids, err := p.proofs.DueProofs(ctx, p.batch)
	if err != nil {
		p.logger.Warn("second look photo sweep could not list what is due; it tries again at the next run", "error", err)
		return Result{}
	}
	res := Result{Due: len(ids)}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		cleared, err := p.clearProofs(ctx, id)
		switch {
		case err != nil:
			res.Failed++
			p.logger.Warn("second look photo sweep failed; it is left for the next run", "id", id.String(), "error", err)
		case cleared:
			res.Purged++
		}
	}
	return res
}

// clearProofs deletes one request's photos inside the store's transaction,
// and reports whether its keys were emptied. A key outside its owner's own
// folder is skipped, as ownPhoto says.
func (p *Purger) clearProofs(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.itemDeadline)
	defer cancel()

	var photos, skipped int
	cleared, err := p.proofs.ClearProofs(ctx, id, func(ctx context.Context, list []Photo) error {
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
	if cleared {
		p.logger.Info("second look photos deleted", "id", id.String(), "photos", photos, "skipped", skipped)
	}
	return cleared, nil
}

// runSecondLooksSafely is one run of RunSecondLookPhotosOnce that recovers a
// panic, as runRemovedSafely does, so a panic neither skips the other runs
// nor ends the loop.
func (p *Purger) runSecondLooksSafely(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("second look photo sweep panicked; the next run goes ahead as planned", "panic", r)
		}
	}()
	p.RunSecondLookPhotosOnce(ctx)
}
