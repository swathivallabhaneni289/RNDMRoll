package purge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage/storagetest"
)

// fakeRemoved keeps a row per removed post or comment and, like the real
// store, deletes a row only when deleteFiles returned nil. Set the fields
// before use.
type fakeRemoved struct {
	steps  *steps
	dueErr error
	photos map[uuid.UUID][]Photo
	// onDue runs at the start of every Due call, outside the lock (it may
	// panic).
	onDue func()
	// block makes Purge of these wait for its context to end.
	block map[uuid.UUID]bool
	// afterPurge runs once a row is deleted.
	afterPurge func(id uuid.UUID)

	mu         sync.Mutex
	due        []Removed
	rows       map[uuid.UUID]bool
	limits     []int
	noDeadline []uuid.UUID
}

// newFakeRemoved returns a store where every item in due has its row.
func newFakeRemoved(s *steps, due ...Removed) *fakeRemoved {
	rows := make(map[uuid.UUID]bool, len(due))
	for _, it := range due {
		rows[it.ID] = true
	}
	return &fakeRemoved{steps: s, due: due, rows: rows, photos: map[uuid.UUID][]Photo{}}
}

func (s *fakeRemoved) Due(_ context.Context, limit int) ([]Removed, error) {
	s.mu.Lock()
	s.limits = append(s.limits, limit)
	due := slices.Clone(s.due)
	s.mu.Unlock()
	if s.onDue != nil {
		s.onDue()
	}
	if s.dueErr != nil {
		return nil, s.dueErr
	}
	if len(due) > limit {
		due = due[:limit]
	}
	return due, nil
}

func (s *fakeRemoved) Purge(ctx context.Context, item Removed, deleteFiles func(context.Context, []Photo) error) (bool, error) {
	s.mu.Lock()
	present := s.rows[item.ID]
	if _, ok := ctx.Deadline(); !ok {
		s.noDeadline = append(s.noDeadline, item.ID)
	}
	s.mu.Unlock()
	if !present {
		return false, nil
	}
	if s.block[item.ID] {
		// Given no deadline, it gives up after a while rather than hold
		// the test forever, and answers as a slow store that got through.
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		s.steps.add("row kept %s", item.ID)
		return false, ctx.Err()
	}
	if err := deleteFiles(ctx, s.photos[item.ID]); err != nil {
		s.steps.add("row kept %s", item.ID)
		return false, err
	}
	s.mu.Lock()
	delete(s.rows, item.ID)
	s.mu.Unlock()
	s.steps.add("row deleted %s", item.ID)
	if s.afterPurge != nil {
		s.afterPurge(item.ID)
	}
	return true, nil
}

func (s *fakeRemoved) hasRow(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows[id]
}

func (s *fakeRemoved) limitsSeen() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.limits)
}

// removedPurger is a purger with no accounts due that purges store.
func removedPurger(s *steps, store *fakeRemoved, files Files) (*Purger, *logs) {
	p, l := newPurger(newFakeStore(s), files)
	p.SetRemoved(store)
	return p, l
}

var (
	postA    = Removed{Kind: "post", ID: uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001")}
	commentB = Removed{Kind: "comment", ID: uuid.MustParse("bbbbbbbb-0000-4000-8000-000000000002")}
	postC    = Removed{Kind: "post", ID: uuid.MustParse("cccccccc-0000-4000-8000-000000000003")}
)

func TestSetRemoved_Defaults(t *testing.T) {
	p := New(newFakeStore(&steps{}), &fakeFiles{}, nil)
	if p.removed != nil || p.RunRemovedOnce(context.Background()) != (Result{}) {
		t.Fatal("a purger with no RemovedStore has one, or runs it")
	}
	p.SetRemoved(newFakeRemoved(&steps{}))
	if p.removed == nil || p.itemDeadline != time.Minute || p.batch != 50 {
		t.Errorf("store %v, item deadline %v, batch %d", p.removed, p.itemDeadline, p.batch)
	}
}

// The photos go first, the post's own and then its comments', and only then
// the row; the run is logged with counts, never with a key.
func TestRunRemovedOnce_DeletesTheFilesBeforeTheRow(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA)
	store.photos[postA.ID] = []Photo{
		{Key: photoKey(alice, 1), Writer: alice},
		{Key: photoKey(alice, 2), Writer: alice},
		{Key: photoKey(bob, 3), Writer: bob},
	}
	p, l := removedPurger(s, store, &fakeFiles{steps: s})

	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	want := []string{"photo " + photoKey(alice, 1), "photo " + photoKey(alice, 2), "photo " + photoKey(bob, 3), "row deleted " + postA.ID.String()}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	recs := l.records(t)
	if len(recs) != 1 {
		t.Fatalf("log = %v, want one record", recs)
	}
	if r := recs[0]; r["level"] != "INFO" || r["msg"] != "removed content purged" || r["kind"] != "post" ||
		r["id"] != postA.ID.String() || r["photos"] != 3.0 || r["skipped"] != 0.0 {
		t.Errorf("log record = %v", r)
	}
	for _, r := range recs {
		if strings.Contains(fmt.Sprint(r), "entries/") {
			t.Errorf("a key was logged: %v", r)
		}
	}
}

// A file that cannot be deleted keeps its item's row, and the photos after
// it are not tried; the next item still goes.
func TestRunRemovedOnce_AFileErrorKeepsTheRowAndTheNextIsTried(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA, commentB)
	store.photos[postA.ID] = []Photo{{Key: photoKey(alice, 1), Writer: alice}, {Key: photoKey(alice, 2), Writer: alice}, {Key: photoKey(bob, 3), Writer: bob}}
	store.photos[commentB.ID] = []Photo{{Key: photoKey(cara, 4), Writer: cara}}
	files := &fakeFiles{steps: s, photoErr: map[string]error{photoKey(alice, 2): errors.New("storage is down")}}
	p, l := removedPurger(s, store, files)

	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	want := []string{
		"photo " + photoKey(alice, 1), "photo " + photoKey(alice, 2), "row kept " + postA.ID.String(),
		"photo " + photoKey(cara, 4), "row deleted " + commentB.ID.String(),
	}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	if !store.hasRow(postA.ID) || store.hasRow(commentB.ID) {
		t.Errorf("rows: post %v, comment %v, want the post's kept and the comment's deleted", store.hasRow(postA.ID), store.hasRow(commentB.ID))
	}
	recs := l.records(t)
	if len(recs) != 2 {
		t.Fatalf("log = %v, want two records", recs)
	}
	if w := recs[0]; w["level"] != "WARN" || w["msg"] != "removed content purge failed; it is left for the next run" ||
		w["kind"] != "post" || w["id"] != postA.ID.String() || !strings.Contains(fmt.Sprint(w["error"]), "storage is down") {
		t.Errorf("failure record = %v", w)
	}
	if r := recs[1]; r["msg"] != "removed content purged" || r["kind"] != "comment" || r["id"] != commentB.ID.String() {
		t.Errorf("purge record = %v", r)
	}
}

// Only a key in its writer's own folder is sent to storage, whoever's post
// it is: one in someone else's folder, a malformed one, an avatar, one with
// no writer and an empty one are skipped, and counted.
func TestRunRemovedOnce_SkipsPhotosOutsideTheirWritersFolder(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA)
	good1, good2 := photoKey(alice, 1), photoKey(bob, 7)
	store.photos[postA.ID] = []Photo{
		{Key: good1, Writer: alice},
		{Key: photoKey(cara, 2), Writer: alice},
		{Key: "entries/" + alice.String() + "/../" + bob.String() + "/0000000000000003.jpg", Writer: alice},
		{Key: fmt.Sprintf("entries/%s/%016X.jpg", bob, 0xabc), Writer: bob},
		{Key: "avatars/" + bob.String() + "/0000000000000004.jpg", Writer: bob},
		{Key: photoKey(uuid.Nil, 5), Writer: uuid.Nil},
		{Key: "", Writer: bob},
		{Key: good2, Writer: bob},
	}
	files := &fakeFiles{steps: s}
	p, l := removedPurger(s, store, files)

	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	if got := files.deletedPhotos(); !slices.Equal(got, []string{good1, good2}) {
		t.Errorf("photos sent to storage = %q, want only the two in their writers' folders", got)
	}
	if recs := l.records(t); len(recs) != 1 || recs[0]["photos"] != 2.0 || recs[0]["skipped"] != 6.0 {
		t.Errorf("log = %v, want 2 photos and 6 skipped", recs)
	}
}

func TestRunRemovedOnce_TakesAtMostTheBatchAndStopsOnceCtxIsDone(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA, commentB, postC)
	p, _ := removedPurger(s, store, &fakeFiles{steps: s})
	p.batch = 2
	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 2, Purged: 2}) {
		t.Errorf("result = %+v", got)
	}
	if got := store.limitsSeen(); !slices.Equal(got, []int{2}) || !store.hasRow(postC.ID) {
		t.Errorf("Due limits %v, the third kept %v; want 2 and kept", got, store.hasRow(postC.ID))
	}

	s = &steps{}
	store = newFakeRemoved(s, postA, commentB, postC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.afterPurge = func(uuid.UUID) { cancel() }
	p, _ = removedPurger(s, store, &fakeFiles{steps: s})
	if got := p.RunRemovedOnce(ctx); got != (Result{Due: 3, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	if !store.hasRow(commentB.ID) || !store.hasRow(postC.ID) {
		t.Error("an item after the cancel was purged")
	}
	// A run that starts with ctx done does not even list what is due.
	if got := p.RunRemovedOnce(ctx); got != (Result{}) || len(store.limitsSeen()) != 1 {
		t.Errorf("a run after the cancel: %+v, Due called %d times", got, len(store.limitsSeen()))
	}
}

func TestRunRemovedOnce_ADueErrorIsLoggedAndNothingDueLogsNothing(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA)
	store.dueErr = errors.New("database is down")
	p, l := removedPurger(s, store, &fakeFiles{steps: s})
	if got := p.RunRemovedOnce(context.Background()); got != (Result{}) || len(s.all()) != 0 {
		t.Errorf("result = %+v, steps %q", got, s.all())
	}
	recs := l.records(t)
	if len(recs) != 1 || recs[0]["level"] != "WARN" || !strings.Contains(fmt.Sprint(recs[0]["error"]), "database is down") {
		t.Errorf("log = %v, want one warning with the error", recs)
	}

	s = &steps{}
	p, l = removedPurger(s, newFakeRemoved(s), &fakeFiles{steps: s})
	if got := p.RunRemovedOnce(context.Background()); got != (Result{}) || len(l.records(t)) != 0 {
		t.Errorf("nothing due: %+v, log %v", got, l.records(t))
	}
}

// Brought back, or gone, between Due and its turn: nothing is touched.
func TestRunRemovedOnce_AnItemBroughtBackSinceDueIsLeftAlone(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA)
	store.photos[postA.ID] = []Photo{{Key: photoKey(alice, 1), Writer: alice}}
	delete(store.rows, postA.ID)
	p, l := removedPurger(s, store, &fakeFiles{steps: s})
	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 1}) {
		t.Errorf("result = %+v", got)
	}
	if got := s.all(); len(got) != 0 || len(l.records(t)) != 0 {
		t.Errorf("steps %q, log %v, want nothing", got, l.records(t))
	}
}

func TestRunRemovedOnce_EachItemHasItsOwnDeadline(t *testing.T) {
	s := &steps{}
	store := newFakeRemoved(s, postA, commentB)
	store.block = map[uuid.UUID]bool{postA.ID: true}
	p, l := removedPurger(s, store, &fakeFiles{steps: s})
	p.itemDeadline = 20 * time.Millisecond

	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	store.mu.Lock()
	noDeadline := slices.Clone(store.noDeadline)
	store.mu.Unlock()
	if len(noDeadline) != 0 {
		t.Errorf("items purged with no deadline: %v", noDeadline)
	}
	if !store.hasRow(postA.ID) || store.hasRow(commentB.ID) {
		t.Errorf("rows: post %v, comment %v, want the post's kept and the comment's deleted", store.hasRow(postA.ID), store.hasRow(commentB.ID))
	}
	if recs := l.records(t); len(recs) != 2 || !strings.Contains(fmt.Sprint(recs[0]["error"]), context.DeadlineExceeded.Error()) {
		t.Errorf("log = %v, want the deadline for the post, then the comment purged", recs)
	}
}

// Every run of the loop purges the accounts and then what was removed, never
// the other way round and never at the same moment.
func TestStart_EachRunPurgesTheAccountsThenWhatWasRemoved(t *testing.T) {
	s := &steps{}
	accounts := newFakeStore(s)
	accounts.onDue = func() { s.add("accounts") }
	removed := newFakeRemoved(s)
	ran := make(chan struct{}, 16)
	removed.onDue = func() {
		s.add("removed")
		signal(ran)
	}
	p, _ := newPurger(accounts, &fakeFiles{steps: s})
	p.SetRemoved(removed)
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, ran, "the first run")
	waitFor(t, ran, "a later run")
	cancel()
	waitReturns(t, p)
	got := s.all()
	if len(got) < 4 {
		t.Fatalf("steps = %q, want at least two runs", got)
	}
	for i, step := range got {
		if want := []string{"accounts", "removed"}[i%2]; step != want && !(i == len(got)-1 && step == "accounts") {
			t.Fatalf("steps = %q: step %d is %q, want %q", got, i, step, want)
		}
	}
}

// A panic in the account run does not skip the removed run after it, and a
// panic in the removed run is logged and the loop goes on.
func TestStart_APanicInEitherRunLeavesTheOtherAndTheLoop(t *testing.T) {
	s := &steps{}
	accounts := newFakeStore(s)
	accountCalls := 0 // only the loop's goroutine touches these
	accounts.onDue = func() {
		accountCalls++
		s.add("accounts %d", accountCalls)
		if accountCalls == 1 {
			panic("bug in the account run")
		}
	}
	removed := newFakeRemoved(s, postA)
	removedCalls := 0
	removed.onDue = func() {
		removedCalls++
		s.add("removed %d", removedCalls)
		if removedCalls == 1 {
			panic("bug in the removed run")
		}
	}
	purged := make(chan struct{}, 1)
	removed.afterPurge = func(uuid.UUID) { signal(purged) }
	p, l := newPurger(accounts, &fakeFiles{steps: s})
	p.SetRemoved(removed)
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, purged, "the removed post purged by a run after both panicked")
	cancel()
	waitReturns(t, p)

	want := []string{"accounts 1", "removed 1", "accounts 2", "removed 2", "row deleted " + postA.ID.String()}
	if got := s.all(); len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Errorf("steps = %q, want them to start %q", got, want)
	}
	panics := map[string]bool{}
	for _, r := range l.records(t) {
		if r["level"] == "ERROR" && strings.Contains(fmt.Sprint(r["msg"]), "panicked") {
			panics[fmt.Sprint(r["panic"])] = true
		}
	}
	if !panics["bug in the account run"] || !panics["bug in the removed run"] {
		t.Errorf("log = %v, want both panics logged", l.records(t))
	}
}

// The purge over the real storage code and an in-memory bucket: the post's
// photos and its comments' photos go, and nobody else's files.
func TestRunRemovedOnce_WithTheRealFileStore(t *testing.T) {
	const bucketName = "rndmroll-photos-test"
	postFiles := []string{photoKey(alice, 1), photoKey(alice, 2)}
	bobsComment := photoKey(bob, 0xb1)
	survivors := []string{photoKey(alice, 3), photoKey(bob, 0xb2), photoKey(cara, 0xc1)}
	bucket := storagetest.NewBucket(bucketName, append(append(slices.Clone(postFiles), bobsComment), survivors...)...)

	s := &steps{}
	store := newFakeRemoved(s, postA)
	store.photos[postA.ID] = []Photo{
		{Key: postFiles[0], Writer: alice},
		{Key: postFiles[1], Writer: alice},
		{Key: bobsComment, Writer: bob},
		// Said to be bob's, but in cara's folder: skipped.
		{Key: photoKey(cara, 0xc1), Writer: bob},
	}
	p, l := removedPurger(s, store, storage.NewAccountFilesOver(bucket, bucketName))

	if got := p.RunRemovedOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Fatalf("result = %+v, log %v", got, l.records(t))
	}
	wantLeft := slices.Clone(survivors)
	slices.Sort(wantLeft)
	if got := bucket.Keys(); !slices.Equal(got, wantLeft) {
		t.Errorf("left in the bucket = %q\nwant %q", got, wantLeft)
	}
	if got, want := bucket.Deleted(), append(slices.Clone(postFiles), bobsComment); !slices.Equal(got, want) {
		t.Errorf("delete calls = %q\nwant %q", got, want)
	}
}
