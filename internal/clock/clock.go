// Package clock is the one place the spin slice learns what time it is.
//
// The daily spin, the weekly bonus and the pretend-day test tools all read
// the time through a Clock, so a test can move one account's clock without
// touching tokens, the rate limiter or refresh expiry (none of those import
// this package). There is no global clock: whoever builds the spin service
// chooses one and passes it in.
package clock

import (
	"sync"
	"time"
)

// Clock tells the spin slice what time it is for one account.
type Clock interface {
	NowFor(userID string) time.Time
}

// ClockControl is what the test-only clock tool needs: move one account's
// clock, put it back, and say whether it is moved. PerUser and Fixed both
// satisfy it.
type ClockControl interface {
	SetNow(userID string, target time.Time)
	Reset(userID string)
	Shifted(userID string) bool
}

// Real is the wall clock. It is what production uses.
type Real struct{}

// NowFor returns the current time. The monotonic reading is stripped so a
// value compares the same way as one read back from the database.
func (Real) NowFor(string) time.Time {
	return time.Now().Round(0)
}

// PerUser is a wall clock with a per-account shift. SetNow makes an
// account's time read as the target from then on, and time keeps running
// from there. Reset puts the account back to real time. One account's shift
// never changes another's reading.
type PerUser struct {
	mu     sync.Mutex
	shifts map[string]time.Duration
}

// NewPerUser returns a PerUser clock with no account moved.
func NewPerUser() *PerUser {
	return &PerUser{shifts: make(map[string]time.Duration)}
}

// SetNow makes userID's clock read target now, and keep running.
func (p *PerUser) SetNow(userID string, target time.Time) {
	shift := target.Sub(time.Now())
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shifts[userID] = shift
}

// Reset returns userID's clock to real time.
func (p *PerUser) Reset(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.shifts, userID)
}

// Shifted reports whether userID's clock is currently moved.
func (p *PerUser) Shifted(userID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.shifts[userID]
	return ok
}

// NowFor returns the current time for userID: real time plus the account's
// shift, if it has one.
func (p *PerUser) NowFor(userID string) time.Time {
	p.mu.Lock()
	shift := p.shifts[userID]
	p.mu.Unlock()
	return time.Now().Add(shift).Round(0)
}

// Fixed is a clock that stands still until a test moves it. It is the same
// for every account. It exists for tests.
type Fixed struct {
	mu sync.Mutex
	t  time.Time
}

// NewFixed returns a Fixed clock reading t.
func NewFixed(t time.Time) *Fixed {
	return &Fixed{t: t}
}

// Set moves the clock to t.
func (f *Fixed) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = t
}

// Advance moves the clock forward by d (backward when d is negative).
func (f *Fixed) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

// NowFor returns the fixed time.
func (f *Fixed) NowFor(string) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

// SetNow moves the clock to target for everyone, so a Fixed clock can sit
// behind the test-only clock route in a test. userID is ignored.
func (f *Fixed) SetNow(_ string, target time.Time) { f.Set(target) }

// Reset does nothing: a Fixed clock has no real time to return to.
func (f *Fixed) Reset(string) {}

// Shifted is always false.
func (f *Fixed) Shifted(string) bool { return false }

var (
	_ Clock        = Real{}
	_ Clock        = (*PerUser)(nil)
	_ Clock        = (*Fixed)(nil)
	_ ClockControl = (*PerUser)(nil)
	_ ClockControl = (*Fixed)(nil)
)
