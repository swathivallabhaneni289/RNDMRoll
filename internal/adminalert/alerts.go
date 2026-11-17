// Package adminalert emails the admins when someone reports a person, a post
// or a comment (developer, 2026-11-17: "notifications so we know that there
// are new reports"). A report of violence or nudity goes out at once, at most
// one email a minute; anything else at most one email an hour, counted
// together. An email says how many reports came in and why, and links to the
// admin page. It never names anyone or shows what was reported: the page
// does that, behind its log in.
//
// What is waiting is kept in memory, so Close sends it before the server
// stops (a free server is put to sleep when it is idle).
package adminalert

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

const (
	// UrgentGap is the least time between two emails when a waiting report
	// is violence or nudity.
	UrgentGap = time.Minute
	// CalmGap is the least time between two emails otherwise.
	CalmGap = time.Hour

	sendTimeout = 30 * time.Second
)

// Sender delivers one email (mail.Sender: Resend, or the log in test mode).
type Sender interface {
	Send(ctx context.Context, to, subject, html, text string) error
}

type stopper interface{ Stop() bool }

// Alerts counts new reports and emails the admins when an email is due. A
// nil *Alerts ignores reports.
type Alerts struct {
	sender  Sender
	to      []string
	pageURL string
	logger  *slog.Logger
	now     func() time.Time
	after   func(time.Duration, func()) stopper
	// deliver sends one composed email, in the background.
	deliver func(email)
	sending sync.WaitGroup

	mu       sync.Mutex
	pending  map[string]int
	lastSent time.Time
	timer    stopper
	closed   bool
}

type email struct{ subject, text, html string }

// New emails to (the admins' addresses) through sender. pageURL is the link
// in the email: the admin page's /admin/go.
func New(sender Sender, to []string, pageURL string, logger *slog.Logger) *Alerts {
	a := &Alerts{sender: sender, to: to, pageURL: pageURL, logger: logger, now: time.Now,
		after:   func(d time.Duration, f func()) stopper { return time.AfterFunc(d, f) },
		pending: map[string]int{}}
	a.deliver = func(e email) {
		a.sending.Add(1)
		go func() {
			defer a.sending.Done()
			a.send(e)
		}()
	}
	return a
}

// Reported counts one report kept (the board and social services call it
// after the save) and sends the email now when one is due, or sets a timer
// for when it will be.
func (a *Alerts) Reported(reason string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	a.pending[reason]++
	a.plan()
}

// plan sends what is waiting once the gap since the last email has passed,
// or sets the timer for then. Called with mu held.
func (a *Alerts) plan() {
	if len(a.pending) == 0 {
		return
	}
	gap := CalmGap
	for reason := range a.pending {
		if admin.Urgent(reason) {
			gap = UrgentGap
		}
	}
	now := a.now()
	due := a.lastSent.Add(gap)
	if !now.Before(due) {
		a.flush(now)
		return
	}
	if a.timer != nil {
		a.timer.Stop()
	}
	a.timer = a.after(due.Sub(now), a.fire)
}

func (a *Alerts) fire() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.timer = nil
	if !a.closed {
		a.plan()
	}
}

// flush sends what is waiting as one email and starts the gap again. Called
// with mu held.
func (a *Alerts) flush(now time.Time) {
	e := compose(a.pending, a.pageURL)
	a.pending = map[string]int{}
	a.lastSent = now
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.deliver(e)
}

func (a *Alerts) send(e email) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	for _, to := range a.to {
		if err := a.sender.Send(ctx, to, e.subject, e.html, e.text); err != nil {
			// The address stays out of the log.
			a.logger.Error("admin report email failed", "error", err)
		}
	}
}

// Close sends what is still waiting, at once, and waits for the emails under
// way until ctx ends. Reports after it are not counted.
func (a *Alerts) Close(ctx context.Context) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		if len(a.pending) > 0 {
			a.flush(a.now())
		}
		if a.timer != nil {
			a.timer.Stop()
			a.timer = nil
		}
	}
	a.mu.Unlock()
	done := make(chan struct{})
	go func() {
		a.sending.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// compose writes the email: how many reports, each reason with its count (the
// most serious first), and the link. No names, no text, no photos.
func compose(counts map[string]int, pageURL string) email {
	reasons := make([]string, 0, len(counts))
	total := 0
	for reason, n := range counts {
		reasons = append(reasons, reason)
		total += n
	}
	slices.SortFunc(reasons, func(x, y string) int {
		if d := admin.ReasonRank(x) - admin.ReasonRank(y); d != 0 {
			return d
		}
		return strings.Compare(x, y)
	})
	noun := "reports"
	if total == 1 {
		noun = "report"
	}
	head := fmt.Sprintf("%d new %s on RNDMRoll.", total, noun)
	lines := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		words := admin.ReasonWords[reason]
		if words == "" {
			words = reason
		}
		lines = append(lines, fmt.Sprintf("%s: %d", words, counts[reason]))
	}
	text := head + "\n\n" + strings.Join(lines, "\n") + "\n\nSee them on the admin page: " + pageURL + "\n"
	var b strings.Builder
	b.WriteString("<p>" + html.EscapeString(head) + "</p><p>")
	for i, line := range lines {
		if i > 0 {
			b.WriteString("<br>")
		}
		b.WriteString(html.EscapeString(line))
	}
	b.WriteString(`</p><p><a href="` + html.EscapeString(pageURL) + `">See them on the admin page</a></p>`)
	return email{subject: fmt.Sprintf("RNDMRoll: %d new %s", total, noun), text: text, html: b.String()}
}
