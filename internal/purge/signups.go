package purge

import (
	"context"
)

// The sweep of sign-ups that were never finished (developer, 2026-11-24).
// Sign-up emails a 6-digit code and makes the account only once the code is
// typed into the app; until then the sign-up waits in the database with a
// password hash, a name and a birthday. Making the account deletes every
// sign-up of its email, and this sweep deletes the others once their code
// has expired and they started more than user.SignupCapWindow ago (the cap
// of sign-ups an hour per email counts them until then). It runs in the same
// loop, last: the rows belong to no account, so it never holds anything the
// other purges want. A failure leaves them for the next run.

// SignupStore is the database side of the sweep (postgres.PendingSignupRepo).
type SignupStore interface {
	// SweepSignups deletes every sign-up whose code has expired and that
	// started more than user.SignupCapWindow ago, and returns how many it
	// deleted.
	SweepSignups(ctx context.Context) (int, error)
}

// SetSignups adds the sweep of unfinished sign-ups to every run of the loop
// Start begins, after the second-look photos. Call it before Start.
func (p *Purger) SetSignups(store SignupStore) {
	p.signups = store
	p.itemDeadline = defaultItemDeadline
}

// RunSignupsOnce sweeps the sign-ups that are due now, in one statement on
// its own deadline. Purged (and Due) is how many it deleted. It does nothing
// when no SignupStore was set or ctx is already done; an error is logged and
// the rows wait for the next run; a run with nothing due logs nothing.
func (p *Purger) RunSignupsOnce(ctx context.Context) Result {
	if p.signups == nil || ctx.Err() != nil {
		return Result{}
	}
	ctx, cancel := context.WithTimeout(ctx, p.itemDeadline)
	defer cancel()
	n, err := p.signups.SweepSignups(ctx)
	if err != nil {
		p.logger.Warn("sign-up sweep failed; it tries again at the next run", "error", err)
		return Result{}
	}
	if n > 0 {
		p.logger.Info("unfinished sign-ups deleted", "sign_ups", n)
	}
	return Result{Due: n, Purged: n}
}

// runSignupsSafely is one run of RunSignupsOnce that recovers a panic, as
// runRemovedSafely does, so a panic neither skips the other runs nor ends
// the loop.
func (p *Purger) runSignupsSafely(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("sign-up sweep panicked; the next run goes ahead as planned", "panic", r)
		}
	}()
	p.RunSignupsOnce(ctx)
}
