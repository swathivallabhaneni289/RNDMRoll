// Package purge deletes, for good, the accounts whose deletion grace period
// (user.DeletionGracePeriod) is over. For each account it first deletes the
// files in storage (the account's own folders, and the photos of other
// people's comments that go with it) and only then the users row, whose
// cascades take everything else. The row goes last because it is the only
// record that the account is due: once it is gone no later run could find
// the files, while files deleted with the row kept are just listed again,
// and found gone, by the next run.
//
// The purge runs in the background on a timer, one account at a time, and
// nothing it deletes can be brought back. Each run then purges, the same
// way, the posts and comments an admin removed whose grace is over
// (removed.go).
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// CommentPhoto is the photo of another person's comment that goes with the
// purged account (their comment on its post, or their reply under its
// comment).
type CommentPhoto struct {
	Key    string
	Author uuid.UUID
}

// Store is the database side (postgres.PurgeRepo).
type Store interface {
	// Due lists up to limit accounts whose grace period is over, oldest ask
	// first.
	Due(ctx context.Context, limit int) ([]uuid.UUID, error)
	// Purge holds the account only while its grace period is still over,
	// hands deleteFiles the photos of other people's comments the cascade
	// will delete, and deletes the users row only when deleteFiles returned
	// nil, all in one transaction. purged is false when there was nothing to
	// purge (the account was restored or is gone). An error from
	// deleteFiles is returned and the row stays.
	Purge(ctx context.Context, id uuid.UUID, deleteFiles func(ctx context.Context, photos []CommentPhoto) error) (purged bool, err error)
}

// Files is the storage side (*storage.AccountFiles).
type Files interface {
	// DeleteAccountFolders deletes every file in the account's folders and
	// returns how many it deleted, also when it fails part way.
	DeleteAccountFolders(ctx context.Context, id uuid.UUID) (int, error)
	// DeletePhoto deletes one entry photo by its key.
	DeletePhoto(ctx context.Context, key string) error
}

// The storage package's AccountFiles is the Files the server uses.
var _ Files = (*storage.AccountFiles)(nil)

const (
	// defaultBatch bounds one run, so a backlog is worked off over several
	// runs instead of in one long one.
	defaultBatch = 50
	// defaultFirstRun keeps the purge out of the way while the server
	// starts, and means a server that fails soon after starting never
	// touches storage.
	defaultFirstRun = time.Minute
	// defaultEvery is the wait from the end of one run to the start of the
	// next. Accounts fall due one by one as their 30 days end, and being an
	// hour late changes nothing for anyone.
	defaultEvery = time.Hour
	// defaultAccountDeadline bounds the work on one account, so a storage
	// call that hangs cannot hold up the run. An account with more files
	// than fit in it is finished by later runs, since what was deleted stays
	// deleted.
	defaultAccountDeadline = 10 * time.Minute
)

// Purger deletes the accounts that are due, in the background (Start) or
// once (RunOnce).
type Purger struct {
	store  Store
	files  Files
	logger *slog.Logger

	// batch is the most accounts one run takes from Due.
	batch int
	// firstRun is the wait between Start and the first run, every the wait
	// after each run.
	firstRun time.Duration
	every    time.Duration
	// accountDeadline bounds the work on one account.
	accountDeadline time.Duration

	// removed, when set (SetRemoved), is the store of the posts and
	// comments an admin removed, which each run purges after the accounts;
	// itemDeadline bounds the work on one of them.
	removed      RemovedStore
	itemDeadline time.Duration

	wg sync.WaitGroup
}

// New builds a Purger with the default settings: up to 50 accounts a run,
// the first run a minute after Start and then one an hour, and 10 minutes
// for each account. A nil logger is slog.Default().
func New(store Store, files Files, logger *slog.Logger) *Purger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Purger{
		store:           store,
		files:           files,
		logger:          logger,
		batch:           defaultBatch,
		firstRun:        defaultFirstRun,
		every:           defaultEvery,
		accountDeadline: defaultAccountDeadline,
	}
}

// Result counts one run. Due is how many accounts the store listed, Purged
// how many were deleted for good, and Failed how many hit an error and were
// left for the next run. An account that was restored or already gone when
// its turn came, or that the run did not reach because ctx was done, counts
// in Due only.
type Result struct {
	Due, Purged, Failed int
}

// RunOnce purges the accounts that are due now, up to the batch, one at a
// time, and stops before the next account once ctx is done. One account at
// a time keeps the load on storage and the database small, and means a
// failure stays with its own account: the next one is still tried. Each
// account has its own deadline. A run with nothing due logs nothing.
func (p *Purger) RunOnce(ctx context.Context) Result {
	ids, err := p.store.Due(ctx, p.batch)
	if err != nil {
		p.logger.Warn("account purge could not list the accounts that are due; it tries again at the next run", "error", err)
		return Result{}
	}
	res := Result{Due: len(ids)}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		purged, err := p.purge(ctx, id)
		switch {
		case err != nil:
			res.Failed++
			p.logger.Warn("account purge failed; it is left for the next run", "user_id", id.String(), "error", err)
		case purged:
			res.Purged++
		}
	}
	return res
}

// purge deletes one account inside the store's transaction: its files
// first, then its row. It reports whether the row was deleted.
func (p *Purger) purge(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.accountDeadline)
	defer cancel()

	var files, photos, skipped int
	purged, err := p.store.Purge(ctx, id, func(ctx context.Context, comments []CommentPhoto) error {
		// Counted afresh on every call, so the log line describes the
		// files of the call whose result the store kept.
		files, photos, skipped = 0, 0, 0
		n, err := p.files.DeleteAccountFolders(ctx, id)
		files = n
		if err != nil {
			return fmt.Errorf("delete the account's folders: %w", err)
		}
		for _, photo := range comments {
			if !deletable(photo, id) {
				skipped++
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := p.files.DeletePhoto(ctx, photo.Key); err != nil {
				return fmt.Errorf("delete a comment photo: %w", err)
			}
			photos++
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if purged {
		p.logger.Info("account purged", "user_id", id.String(), "files", files, "comment_photos", photos, "skipped", skipped)
	}
	return purged, nil
}

// deletable reports whether the photo of another person's comment may be
// deleted with the purged account: its key must be an entry photo key in
// its author's own folder, and the author must be a real account other than
// the purged one, whose own files went with its folders. The database's word
// alone is not enough to delete a file, so a key outside its author's folder
// (another person's file, or something that is not an entry photo at all) is
// skipped and never sent to storage.
func deletable(photo CommentPhoto, purged uuid.UUID) bool {
	return photo.Author != uuid.Nil && photo.Author != purged && storage.EntryPhotoKeyOwnedBy(photo.Key, photo.Author)
}

// Start runs the purge in one background goroutine: the first run firstRun
// after Start, then one every `every` after the previous run ended, so runs
// never overlap, until ctx is done. Each run purges the accounts, and then
// what an admin removed (SetRemoved). A panic in a run is logged and the
// loop goes on. To stop it, cancel ctx and then call Wait.
func (p *Purger) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		timer := time.NewTimer(p.firstRun)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			if ctx.Err() != nil {
				return
			}
			p.runSafely(ctx)
			p.runRemovedSafely(ctx)
			timer.Reset(p.every)
		}
	}()
}

// runSafely is one run that recovers a panic, so a bug that panics in one
// run leaves the next one planned instead of ending the loop, or the whole
// server with it.
func (p *Purger) runSafely(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("account purge panicked; the next run goes ahead as planned", "panic", r)
		}
	}()
	p.RunOnce(ctx)
}

// Wait returns once the goroutine Start began has ended, so the server can
// cancel ctx at shutdown and then wait for a run in progress to stop. It
// returns at once when Start was never called.
func (p *Purger) Wait() { p.wg.Wait() }
