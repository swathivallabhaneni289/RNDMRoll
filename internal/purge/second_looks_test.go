package purge

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeProofs is a ProofStore: the requests with photos due, each cleared
// only when deleteFiles returns nil.
type fakeProofs struct {
	steps  *steps
	dueErr error
	onDue  func()

	mu      sync.Mutex
	due     []uuid.UUID
	photos  map[uuid.UUID][]Photo
	cleared []uuid.UUID
}

func (s *fakeProofs) DueProofs(_ context.Context, limit int) ([]uuid.UUID, error) {
	if s.onDue != nil {
		s.onDue()
	}
	if s.dueErr != nil {
		return nil, s.dueErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.due[:min(limit, len(s.due))]), nil
}

func (s *fakeProofs) ClearProofs(ctx context.Context, id uuid.UUID, deleteFiles func(context.Context, []Photo) error) (bool, error) {
	s.mu.Lock()
	photos, ok := s.photos[id]
	s.mu.Unlock()
	if !ok {
		return false, nil
	}
	if err := deleteFiles(ctx, photos); err != nil {
		return false, err
	}
	s.steps.add("cleared %s", id)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.photos, id)
	s.cleared = append(s.cleared, id)
	return true, nil
}

var (
	lookA  = uuid.MustParse("aaaaaaaa-0000-4000-8000-0000000000a1")
	lookB  = uuid.MustParse("bbbbbbbb-0000-4000-8000-0000000000b2")
	ownerA = uuid.MustParse("aaaaaaaa-1111-4000-8000-0000000000a1")
	ownerB = uuid.MustParse("bbbbbbbb-1111-4000-8000-0000000000b2")
)

func proofKey(owner uuid.UUID, name string) string { return "entries/" + owner.String() + "/" + name }

// Each due request's photos are deleted before its keys are emptied; a
// photo outside its owner's folder is never sent to storage; a file error
// keeps that request's keys for the next run and the next request is
// tried.
func TestRunSecondLookPhotosOnce_DeletesThePhotosThenEmptiesTheKeys(t *testing.T) {
	s := &steps{}
	files := &fakeFiles{steps: s, photoErr: map[string]error{proofKey(ownerA, "00000000000000a2.jpg"): errors.New("storage down")}}
	store := &fakeProofs{steps: s, due: []uuid.UUID{lookA, lookB}, photos: map[uuid.UUID][]Photo{
		lookA: {{Key: proofKey(ownerA, "00000000000000a1.jpg"), Writer: ownerA}, {Key: proofKey(ownerA, "00000000000000a2.jpg"), Writer: ownerA}},
		lookB: {{Key: proofKey(ownerB, "00000000000000b1.png"), Writer: ownerB}, {Key: proofKey(ownerA, "00000000000000a9.jpg"), Writer: ownerB}},
	}}
	p, _ := newPurger(newFakeStore(s), files)
	p.SetSecondLooks(store)
	if p.itemDeadline != defaultItemDeadline {
		t.Fatalf("itemDeadline = %v, want %v", p.itemDeadline, defaultItemDeadline)
	}
	res := p.RunSecondLookPhotosOnce(context.Background())
	if res != (Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Fatalf("result = %+v", res)
	}
	want := []string{"photo " + proofKey(ownerA, "00000000000000a1.jpg"), "photo " + proofKey(ownerA, "00000000000000a2.jpg"),
		"photo " + proofKey(ownerB, "00000000000000b1.png"), "cleared " + lookB.String()}
	if got := s.all(); !slices.Equal(got, want) {
		t.Fatalf("steps = %q, want %q", got, want)
	}
	if !slices.Equal(store.cleared, []uuid.UUID{lookB}) {
		t.Fatalf("cleared %v, want only %v", store.cleared, lookB)
	}
}

// With no store set nothing runs; a list error is logged and nothing is
// cleared; at most the batch is taken.
func TestRunSecondLookPhotosOnce_NoStoreAListErrorAndTheBatch(t *testing.T) {
	s := &steps{}
	p, _ := newPurger(newFakeStore(s), &fakeFiles{steps: s})
	if res := p.RunSecondLookPhotosOnce(context.Background()); res != (Result{}) {
		t.Fatalf("with no store: %+v", res)
	}
	p.SetSecondLooks(&fakeProofs{steps: s, dueErr: errors.New("db down")})
	if res := p.RunSecondLookPhotosOnce(context.Background()); res != (Result{}) || len(s.all()) != 0 {
		t.Fatalf("a list error: %+v, steps %q", res, s.all())
	}
	store := &fakeProofs{steps: s, due: []uuid.UUID{lookA, lookB}, photos: map[uuid.UUID][]Photo{lookA: nil, lookB: nil}}
	p.SetSecondLooks(store)
	p.batch = 1
	if res := p.RunSecondLookPhotosOnce(context.Background()); res != (Result{Due: 1, Purged: 1}) {
		t.Fatalf("a batch of 1: %+v", res)
	}
}

// Each run of the loop purges the accounts, then what was removed, then the
// second-look photos due.
func TestStart_EachRunSweepsTheSecondLookPhotosLast(t *testing.T) {
	s := &steps{}
	accounts := newFakeStore(s)
	accounts.onDue = func() { s.add("accounts") }
	removed := newFakeRemoved(s)
	removed.onDue = func() { s.add("removed") }
	ran := make(chan struct{}, 16)
	proofs := &fakeProofs{steps: s, onDue: func() {
		s.add("proofs")
		signal(ran)
	}}
	p, _ := newPurger(accounts, &fakeFiles{steps: s})
	p.SetRemoved(removed)
	p.SetSecondLooks(proofs)
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, ran, "the first run")
	waitFor(t, ran, "a later run")
	cancel()
	waitReturns(t, p)
	got := s.all()
	if len(got) < 6 {
		t.Fatalf("steps = %q, want at least two runs", got)
	}
	for i, step := range got[:6] {
		if want := []string{"accounts", "removed", "proofs"}[i%3]; step != want {
			t.Fatalf("steps = %q: step %d is %q, want %q", got, i, step, want)
		}
	}
}
