package purge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage/storagetest"
)

var (
	alice = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	bob   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	cara  = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

// photoKey is a server shaped entry photo key in id's folder.
func photoKey(id uuid.UUID, n int) string {
	return fmt.Sprintf("entries/%s/%016x.jpg", id, n)
}

// steps is the order things happened in, shared by the fake store and the
// fake files.
type steps struct {
	mu   sync.Mutex
	list []string
}

func (s *steps) add(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, fmt.Sprintf(format, args...))
}

func (s *steps) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

// fakeStore keeps a users row per account and, like the real store, deletes
// a row only when deleteFiles returned nil. Set the fields before use.
type fakeStore struct {
	steps  *steps
	dueErr error
	photos map[uuid.UUID][]CommentPhoto
	// onDue runs at the start of every Due call, outside the lock (it may
	// panic).
	onDue func()
	// afterPurge runs once an account's row is deleted.
	afterPurge func(id uuid.UUID)

	mu         sync.Mutex
	due        []uuid.UUID
	rows       map[uuid.UUID]bool
	limits     []int
	noDeadline []uuid.UUID
}

// newFakeStore returns a store where every account in due has its row.
func newFakeStore(s *steps, due ...uuid.UUID) *fakeStore {
	rows := make(map[uuid.UUID]bool, len(due))
	for _, id := range due {
		rows[id] = true
	}
	return &fakeStore{steps: s, due: due, rows: rows, photos: map[uuid.UUID][]CommentPhoto{}}
}

func (s *fakeStore) Due(_ context.Context, limit int) ([]uuid.UUID, error) {
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

func (s *fakeStore) Purge(ctx context.Context, id uuid.UUID, deleteFiles func(context.Context, []CommentPhoto) error) (bool, error) {
	s.mu.Lock()
	present := s.rows[id]
	if _, ok := ctx.Deadline(); !ok {
		s.noDeadline = append(s.noDeadline, id)
	}
	s.mu.Unlock()
	if !present {
		return false, nil
	}
	if err := deleteFiles(ctx, s.photos[id]); err != nil {
		s.steps.add("row kept %s", id)
		return false, err
	}
	s.mu.Lock()
	delete(s.rows, id)
	s.mu.Unlock()
	s.steps.add("row deleted %s", id)
	if s.afterPurge != nil {
		s.afterPurge(id)
	}
	return true, nil
}

func (s *fakeStore) hasRow(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows[id]
}

func (s *fakeStore) limitsSeen() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.limits)
}

// fakeFiles records what it was asked to delete. Set the fields before use.
type fakeFiles struct {
	steps *steps
	// perAccount is the count DeleteAccountFolders reports.
	perAccount int
	folderErr  map[uuid.UUID]error
	photoErr   map[string]error
	// block makes DeleteAccountFolders wait for its context to end.
	block map[uuid.UUID]bool

	mu     sync.Mutex
	photos []string
}

func (f *fakeFiles) DeleteAccountFolders(ctx context.Context, id uuid.UUID) (int, error) {
	f.steps.add("folders %s", id)
	if f.block[id] {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	if err := f.folderErr[id]; err != nil {
		return 1, err
	}
	return f.perAccount, nil
}

func (f *fakeFiles) DeletePhoto(_ context.Context, key string) error {
	f.steps.add("photo %s", key)
	f.mu.Lock()
	f.photos = append(f.photos, key)
	f.mu.Unlock()
	return f.photoErr[key]
}

func (f *fakeFiles) deletedPhotos() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.photos)
}

// logs collects what a purger logs, one JSON object per record. Its lock
// lets a test read while the background goroutine writes.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) records(t *testing.T) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	dec := json.NewDecoder(bytes.NewReader(l.buf.Bytes()))
	for dec.More() {
		var r map[string]any
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func newPurger(store Store, files Files) (*Purger, *logs) {
	l := &logs{}
	logger := slog.New(slog.NewJSONHandler(l, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return New(store, files, logger), l
}

func TestNew_Defaults(t *testing.T) {
	p := New(newFakeStore(&steps{}), &fakeFiles{}, nil)
	if p.batch != 50 || p.firstRun != time.Minute || p.every != time.Hour || p.accountDeadline != 10*time.Minute {
		t.Errorf("batch %d, first run %v, every %v, account deadline %v", p.batch, p.firstRun, p.every, p.accountDeadline)
	}
	if p.logger == nil {
		t.Error("a nil logger was kept")
	}
}

func TestRunOnce_DeletesTheFilesBeforeTheRow(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	store.photos[alice] = []CommentPhoto{{Key: photoKey(bob, 1), Author: bob}}
	p, l := newPurger(store, &fakeFiles{steps: s, perAccount: 7})

	if got := p.RunOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	want := []string{"folders " + alice.String(), "photo " + photoKey(bob, 1), "row deleted " + alice.String()}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	recs := l.records(t)
	if len(recs) != 1 {
		t.Fatalf("log = %v, want one record", recs)
	}
	r := recs[0]
	if r["level"] != "INFO" || r["msg"] != "account purged" || r["user_id"] != alice.String() ||
		r["files"] != 7.0 || r["comment_photos"] != 1.0 || r["skipped"] != 0.0 {
		t.Errorf("log record = %v", r)
	}
}

func TestRunOnce_AFolderErrorKeepsTheRowAndTheNextAccountIsTried(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice, bob)
	store.photos[alice] = []CommentPhoto{{Key: photoKey(cara, 1), Author: cara}}
	files := &fakeFiles{steps: s, perAccount: 3, folderErr: map[uuid.UUID]error{alice: errors.New("storage is down")}}
	p, l := newPurger(store, files)

	if got := p.RunOnce(context.Background()); got != (Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	// The comment photo is never touched once the folders failed.
	want := []string{
		"folders " + alice.String(), "row kept " + alice.String(),
		"folders " + bob.String(), "row deleted " + bob.String(),
	}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	if !store.hasRow(alice) || store.hasRow(bob) {
		t.Errorf("rows: alice %v, bob %v, want alice's kept and bob's deleted", store.hasRow(alice), store.hasRow(bob))
	}
	recs := l.records(t)
	if len(recs) != 2 {
		t.Fatalf("log = %v, want two records", recs)
	}
	if w := recs[0]; w["level"] != "WARN" || w["msg"] != "account purge failed; it is left for the next run" ||
		w["user_id"] != alice.String() || !strings.Contains(fmt.Sprint(w["error"]), "storage is down") {
		t.Errorf("failure record = %v", w)
	}
	if r := recs[1]; r["msg"] != "account purged" || r["user_id"] != bob.String() {
		t.Errorf("purge record = %v", r)
	}
}

func TestRunOnce_ACommentPhotoErrorKeepsTheRow(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	store.photos[alice] = []CommentPhoto{{Key: photoKey(bob, 1), Author: bob}, {Key: photoKey(cara, 2), Author: cara}}
	files := &fakeFiles{steps: s, photoErr: map[string]error{photoKey(bob, 1): errors.New("storage is down")}}
	p, _ := newPurger(store, files)

	if got := p.RunOnce(context.Background()); got != (Result{Due: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	// The photo after the failed one is not tried.
	want := []string{"folders " + alice.String(), "photo " + photoKey(bob, 1), "row kept " + alice.String()}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	if !store.hasRow(alice) {
		t.Error("the row was deleted although a file was not")
	}
}

func TestRunOnce_SkipsCommentPhotosItMustNotDelete(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	good1, good2 := photoKey(bob, 1), photoKey(cara, 7)
	store.photos[alice] = []CommentPhoto{
		{Key: good1, Author: bob},
		{Key: "entries/../x.jpg", Author: bob},
		{Key: fmt.Sprintf("entries/%s/%016X.jpg", bob, 0xabc), Author: bob},
		{Key: "avatars/" + bob.String() + "/0000000000000003.jpg", Author: bob},
		{Key: photoKey(cara, 4), Author: bob},
		{Key: photoKey(alice, 5), Author: alice},
		{Key: photoKey(uuid.Nil, 6), Author: uuid.Nil},
		{Key: "", Author: bob},
		{Key: good2, Author: cara},
	}
	files := &fakeFiles{steps: s, perAccount: 2}
	p, l := newPurger(store, files)

	if got := p.RunOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	if got := files.deletedPhotos(); !slices.Equal(got, []string{good1, good2}) {
		t.Errorf("photos sent to storage = %q, want only the two in their authors' folders", got)
	}
	recs := l.records(t)
	if len(recs) != 1 || recs[0]["comment_photos"] != 2.0 || recs[0]["skipped"] != 7.0 {
		t.Errorf("log = %v, want 2 comment photos and 7 skipped", recs)
	}
}

func TestRunOnce_TakesAtMostTheBatchFromDue(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice, bob, cara)
	p, _ := newPurger(store, &fakeFiles{steps: s})
	p.RunOnce(context.Background())
	if got := store.limitsSeen(); !slices.Equal(got, []int{50}) {
		t.Errorf("Due limits = %v, want the default 50", got)
	}

	s = &steps{}
	store = newFakeStore(s, alice, bob, cara)
	p, _ = newPurger(store, &fakeFiles{steps: s})
	p.batch = 2
	if got := p.RunOnce(context.Background()); got != (Result{Due: 2, Purged: 2}) {
		t.Errorf("result = %+v", got)
	}
	if got := store.limitsSeen(); !slices.Equal(got, []int{2}) {
		t.Errorf("Due limits = %v, want 2", got)
	}
	if !store.hasRow(cara) {
		t.Error("an account past the batch was purged")
	}
}

func TestRunOnce_StopsBeforeTheNextAccountOnceCtxIsDone(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice, bob, cara)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.afterPurge = func(uuid.UUID) { cancel() }
	p, _ := newPurger(store, &fakeFiles{steps: s})

	if got := p.RunOnce(ctx); got != (Result{Due: 3, Purged: 1}) {
		t.Errorf("result = %+v", got)
	}
	want := []string{"folders " + alice.String(), "row deleted " + alice.String()}
	if got := s.all(); !slices.Equal(got, want) {
		t.Errorf("steps = %q\nwant %q", got, want)
	}
	if !store.hasRow(bob) || !store.hasRow(cara) {
		t.Error("an account after the cancel was purged")
	}
}

func TestRunOnce_ADueErrorIsLoggedAndEndsTheRun(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	store.dueErr = errors.New("database is down")
	p, l := newPurger(store, &fakeFiles{steps: s})

	if got := p.RunOnce(context.Background()); got != (Result{}) {
		t.Errorf("result = %+v", got)
	}
	if got := s.all(); len(got) != 0 {
		t.Errorf("steps = %q, want none", got)
	}
	recs := l.records(t)
	if len(recs) != 1 || recs[0]["level"] != "WARN" || !strings.Contains(fmt.Sprint(recs[0]["error"]), "database is down") {
		t.Errorf("log = %v, want one warning with the error", recs)
	}
}

func TestRunOnce_NothingDueLogsNothing(t *testing.T) {
	s := &steps{}
	p, l := newPurger(newFakeStore(s), &fakeFiles{steps: s})

	if got := p.RunOnce(context.Background()); got != (Result{}) {
		t.Errorf("result = %+v", got)
	}
	if recs := l.records(t); len(recs) != 0 {
		t.Errorf("log = %v, want nothing", recs)
	}
}

func TestRunOnce_AnAccountRestoredSinceDueIsLeftAlone(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	// Restored, or gone, between Due and its turn.
	delete(store.rows, alice)
	p, l := newPurger(store, &fakeFiles{steps: s})

	if got := p.RunOnce(context.Background()); got != (Result{Due: 1}) {
		t.Errorf("result = %+v", got)
	}
	if got := s.all(); len(got) != 0 {
		t.Errorf("steps = %q, want no file touched", got)
	}
	if recs := l.records(t); len(recs) != 0 {
		t.Errorf("log = %v, want nothing", recs)
	}
}

func TestRunOnce_EachAccountHasItsOwnDeadline(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice, bob)
	files := &fakeFiles{steps: s, block: map[uuid.UUID]bool{alice: true}}
	p, l := newPurger(store, files)
	p.accountDeadline = 20 * time.Millisecond

	if got := p.RunOnce(context.Background()); got != (Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	store.mu.Lock()
	noDeadline := slices.Clone(store.noDeadline)
	store.mu.Unlock()
	if len(noDeadline) != 0 {
		t.Errorf("accounts purged with no deadline: %v", noDeadline)
	}
	if !store.hasRow(alice) || store.hasRow(bob) {
		t.Errorf("rows: alice %v, bob %v, want alice's kept and bob's deleted", store.hasRow(alice), store.hasRow(bob))
	}
	recs := l.records(t)
	if len(recs) != 2 || !strings.Contains(fmt.Sprint(recs[0]["error"]), context.DeadlineExceeded.Error()) {
		t.Errorf("log = %v, want the deadline for alice, then bob purged", recs)
	}
}

// The purge over the real storage code and an in-memory bucket: the purged
// account's folders and one comment photo go, and nobody else's files.
func TestRunOnce_WithTheRealFileStore(t *testing.T) {
	const bucketName = "rndmroll-photos-test"
	aliceFiles := []string{
		"avatars/" + alice.String() + "/0000000000000001.jpg",
		photoKey(alice, 1), photoKey(alice, 2), photoKey(alice, 3),
	}
	bobsComment := photoKey(bob, 0xb1)
	survivors := []string{
		photoKey(bob, 0xb2),
		"avatars/" + bob.String() + "/0000000000000001.jpg",
		photoKey(cara, 0xc1),
		"entries/" + alice.String(),
	}
	all := append(append(slices.Clone(aliceFiles), bobsComment), survivors...)
	bucket := storagetest.NewBucket(bucketName, all...)

	s := &steps{}
	store := newFakeStore(s, alice)
	store.photos[alice] = []CommentPhoto{
		{Key: bobsComment, Author: bob},
		// Said to be bob's, but in cara's folder: skipped.
		{Key: photoKey(cara, 0xc1), Author: bob},
		// Alice's own: it went with her folders.
		{Key: photoKey(alice, 2), Author: alice},
	}
	p, l := newPurger(store, storage.NewAccountFilesOver(bucket, bucketName))

	if got := p.RunOnce(context.Background()); got != (Result{Due: 1, Purged: 1}) {
		t.Fatalf("result = %+v, log %v", got, l.records(t))
	}
	wantLeft := slices.Clone(survivors)
	slices.Sort(wantLeft)
	if got := bucket.Keys(); !slices.Equal(got, wantLeft) {
		t.Errorf("left in the bucket = %q\nwant %q", got, wantLeft)
	}
	if got, want := bucket.Deleted(), append(slices.Clone(aliceFiles), bobsComment); !slices.Equal(got, want) {
		t.Errorf("delete calls = %q\nwant %q", got, want)
	}
	recs := l.records(t)
	if len(recs) != 1 || recs[0]["files"] != 4.0 || recs[0]["comment_photos"] != 1.0 || recs[0]["skipped"] != 2.0 {
		t.Errorf("log = %v, want 4 files, 1 comment photo and 2 skipped", recs)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// waitReturns fails the test when Wait does not return soon.
func waitReturns(t *testing.T, p *Purger) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after ctx was cancelled")
	}
}

// signal sends on ch without blocking, so a full channel never holds up the
// loop.
func signal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func TestStart_RunsThenRunsAgainAndStopsWhenCancelled(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s)
	ran := make(chan struct{}, 16)
	store.onDue = func() { signal(ran) }
	p, _ := newPurger(store, &fakeFiles{steps: s})
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, ran, "the first run")
	waitFor(t, ran, "a later run")
	cancel()
	waitReturns(t, p)
}

func TestStart_APanicInARunIsLoggedAndTheLoopGoesOn(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s)
	ran := make(chan struct{}, 16)
	calls := 0 // only the loop's goroutine touches it
	store.onDue = func() {
		calls++
		if calls == 1 {
			panic("bug in a run")
		}
		signal(ran)
	}
	p, l := newPurger(store, &fakeFiles{steps: s})
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, ran, "a run after the panic")
	cancel()
	waitReturns(t, p)

	found := false
	for _, r := range l.records(t) {
		if r["level"] == "ERROR" && strings.Contains(fmt.Sprint(r["msg"]), "panicked") && r["panic"] == "bug in a run" {
			found = true
		}
	}
	if !found {
		t.Errorf("log = %v, want the panic logged", l.records(t))
	}
}

func TestStart_CancelBeforeTheFirstRunStopsPromptly(t *testing.T) {
	s := &steps{}
	store := newFakeStore(s, alice)
	p, _ := newPurger(store, &fakeFiles{steps: s})
	p.firstRun = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	cancel()
	waitReturns(t, p)
	if got := store.limitsSeen(); len(got) != 0 {
		t.Errorf("Due was called %d times, want no run", len(got))
	}
	if !store.hasRow(alice) {
		t.Error("an account was purged before the first run was due")
	}
}

func TestWait_WithoutStartReturns(t *testing.T) {
	p, _ := newPurger(newFakeStore(&steps{}), &fakeFiles{})
	waitReturns(t, p)
}
