package purge

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeSignups is a SignupStore: each sweep deletes n sign-ups, or fails.
type fakeSignups struct {
	steps *steps
	n     int
	err   error
	// onSweep runs at the start of every sweep, outside any lock (it may
	// panic).
	onSweep func()
	// deadline is whether the last sweep's context had a deadline.
	deadline bool
	sweeps   int
}

func (s *fakeSignups) SweepSignups(ctx context.Context) (int, error) {
	if s.onSweep != nil {
		s.onSweep()
	}
	s.sweeps++
	_, s.deadline = ctx.Deadline()
	if s.err != nil {
		return 0, s.err
	}
	return s.n, nil
}

// A sweep counts what it deleted and logs it only when it deleted
// something; an error is logged and changes nothing; with no store, or a
// context already done, nothing runs. Each sweep has its own deadline.
func TestRunSignupsOnce_SweepsCountsAndLogs(t *testing.T) {
	s := &steps{}
	p, l := newPurger(newFakeStore(s), &fakeFiles{steps: s})
	if res := p.RunSignupsOnce(context.Background()); res != (Result{}) {
		t.Fatalf("with no store: %+v", res)
	}

	store := &fakeSignups{steps: s, n: 3}
	p.SetSignups(store)
	if p.itemDeadline != defaultItemDeadline {
		t.Fatalf("itemDeadline = %v, want %v", p.itemDeadline, defaultItemDeadline)
	}
	if res := p.RunSignupsOnce(context.Background()); res != (Result{Due: 3, Purged: 3}) || !store.deadline {
		t.Fatalf("a sweep of 3: %+v (deadline %v)", res, store.deadline)
	}
	store.n = 0
	if res := p.RunSignupsOnce(context.Background()); res != (Result{}) {
		t.Fatalf("a sweep of none: %+v", res)
	}
	store.err = errors.New("db down")
	if res := p.RunSignupsOnce(context.Background()); res != (Result{}) {
		t.Fatalf("a failed sweep: %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := store.sweeps
	if res := p.RunSignupsOnce(ctx); res != (Result{}) || store.sweeps != before {
		t.Fatalf("a context already done: %+v, %d sweeps run", res, store.sweeps-before)
	}

	var info, warn int
	for _, r := range l.records(t) {
		switch {
		case r["level"] == "INFO" && r["msg"] == "unfinished sign-ups deleted" && r["sign_ups"] == float64(3):
			info++
		case r["level"] == "WARN" && r["error"] == "db down":
			warn++
		}
	}
	if info != 1 || warn != 1 || len(l.records(t)) != 2 {
		t.Fatalf("log = %v, want one count and one warning", l.records(t))
	}
}

// Each run of the loop sweeps the unfinished sign-ups last, after the
// accounts, what was removed and the second-look photos; a panic in the
// sweep is logged and the loop goes on.
func TestStart_EachRunSweepsTheSignUpsLast(t *testing.T) {
	s := &steps{}
	accounts := newFakeStore(s)
	accounts.onDue = func() { s.add("accounts") }
	removed := newFakeRemoved(s)
	removed.onDue = func() { s.add("removed") }
	proofs := &fakeProofs{steps: s, onDue: func() { s.add("proofs") }}
	ran := make(chan struct{}, 16)
	calls := 0 // only the loop's goroutine touches it
	signups := &fakeSignups{steps: s, onSweep: func() {
		s.add("signups")
		calls++
		if calls == 1 {
			panic("bug in the sweep")
		}
		signal(ran)
	}}
	p, l := newPurger(accounts, &fakeFiles{steps: s})
	p.SetRemoved(removed)
	p.SetSecondLooks(proofs)
	p.SetSignups(signups)
	p.firstRun, p.every = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitFor(t, ran, "a run after the panic")
	cancel()
	waitReturns(t, p)
	got := s.all()
	if len(got) < 8 {
		t.Fatalf("steps = %q, want at least two runs", got)
	}
	for i, step := range got[:8] {
		if want := []string{"accounts", "removed", "proofs", "signups"}[i%4]; step != want {
			t.Fatalf("steps = %q: step %d is %q, want %q", got, i, step, want)
		}
	}
	found := false
	for _, r := range l.records(t) {
		if r["level"] == "ERROR" && r["panic"] == "bug in the sweep" {
			found = true
		}
	}
	if !found {
		t.Fatalf("log = %v, want the panic logged", l.records(t))
	}
}
