package adminalert

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type sent struct{ to, subject, html, text string }

type fakeSender struct {
	got []sent
	err error
}

func (f *fakeSender) Send(_ context.Context, to, subject, html, text string) error {
	f.got = append(f.got, sent{to, subject, html, text})
	return f.err
}

type fakeTimer struct {
	d       time.Duration
	due     time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	t.stopped = true
	return true
}

// rig is Alerts on a fake clock, fake timers and a fake sender, sending in
// the foreground.
type rig struct {
	t      *testing.T
	a      *Alerts
	sender *fakeSender
	clock  time.Time
	timers []*fakeTimer
	log    bytes.Buffer
}

const pageURL = "https://rndmroll.example.test/admin/go"

func newRig(t *testing.T, to ...string) *rig {
	t.Helper()
	if len(to) == 0 {
		to = []string{"boss@example.com"}
	}
	r := &rig{t: t, sender: &fakeSender{}, clock: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)}
	r.a = New(r.sender, to, pageURL, slog.New(slog.NewTextHandler(&r.log, nil)))
	r.a.now = func() time.Time { return r.clock }
	r.a.after = func(d time.Duration, f func()) stopper {
		ft := &fakeTimer{d: d, due: r.clock.Add(d), f: f}
		r.timers = append(r.timers, ft)
		return ft
	}
	r.a.deliver = r.a.send
	return r
}

// live is the one timer still set, or nil.
func (r *rig) live() *fakeTimer {
	r.t.Helper()
	var out *fakeTimer
	for _, ft := range r.timers {
		if !ft.stopped {
			if out != nil {
				r.t.Fatalf("two timers set")
			}
			out = ft
		}
	}
	return out
}

// pass moves the clock by d and fires the live timer when it is due.
func (r *rig) pass(d time.Duration) {
	r.t.Helper()
	r.clock = r.clock.Add(d)
	if ft := r.live(); ft != nil && !r.clock.Before(ft.due) {
		ft.stopped = true
		ft.f()
	}
}

func (r *rig) emails() int { return len(r.sender.got) }

func (r *rig) last() sent {
	r.t.Helper()
	if len(r.sender.got) == 0 {
		r.t.Fatal("no email sent")
	}
	return r.sender.got[len(r.sender.got)-1]
}

func TestTheFirstReportIsEmailedAtOnceWithCountsAndTheLinkOnly(t *testing.T) {
	r := newRig(t)
	r.a.Reported("spam")
	if r.emails() != 1 {
		t.Fatalf("emails: %d, want 1 at once", r.emails())
	}
	got := r.last()
	if got.to != "boss@example.com" || got.subject != "RNDMRoll: 1 new report" {
		t.Fatalf("to %q, subject %q", got.to, got.subject)
	}
	want := "1 new report on RNDMRoll.\n\nSpam: 1\n\nSee them on the admin page: " + pageURL + "\n"
	if got.text != want {
		t.Fatalf("text:\n%q\nwant\n%q", got.text, want)
	}
	if !strings.Contains(got.html, `<a href="`+pageURL+`">See them on the admin page</a>`) || !strings.Contains(got.html, "Spam: 1") {
		t.Fatalf("html: %s", got.html)
	}
	if r.live() != nil {
		t.Fatal("a timer is set with nothing waiting")
	}
}

func TestCalmReportsWaitForTheHourAndGoOutTogether(t *testing.T) {
	r := newRig(t)
	r.a.Reported("spam") // sent at once
	r.pass(10 * time.Minute)
	r.a.Reported("spam")
	r.pass(10 * time.Minute)
	r.a.Reported("impersonation")
	if r.emails() != 1 {
		t.Fatalf("emails within the hour: %d, want still 1", r.emails())
	}
	if ft := r.live(); ft == nil || ft.d != 40*time.Minute {
		t.Fatalf("timer: %+v, want one for the 40 minutes left of the hour", ft)
	}
	r.pass(39 * time.Minute)
	if r.emails() != 1 {
		t.Fatal("sent before the hour was up")
	}
	r.pass(time.Minute)
	if r.emails() != 2 {
		t.Fatalf("emails at the hour: %d, want 2", r.emails())
	}
	got := r.last()
	// The first spam went out at once; the hour's email has the two after it.
	if got.subject != "RNDMRoll: 2 new reports" ||
		!strings.Contains(got.text, "2 new reports on RNDMRoll.\n\nPretending to be someone else: 1\nSpam: 1\n") {
		t.Fatalf("the hour's email: %q\n%s", got.subject, got.text)
	}
}

func TestViolenceOrNudityGoesOutWithinAMinute(t *testing.T) {
	r := newRig(t)
	r.a.Reported("spam") // sent at once
	r.pass(20 * time.Second)
	r.a.Reported("spam")
	r.a.Reported("violence")
	if ft := r.live(); ft == nil || ft.d != 40*time.Second {
		t.Fatalf("timer: %+v, want the 40 seconds left of the minute", ft)
	}
	r.pass(40 * time.Second)
	if r.emails() != 2 {
		t.Fatalf("emails: %d, want the urgent one sent", r.emails())
	}
	if got := r.last(); !strings.Contains(got.text, "2 new reports on RNDMRoll.\n\nViolence or threats: 1\nSpam: 1\n") {
		t.Fatalf("the urgent email, most serious first: %s", got.text)
	}
	// A minute after that one, nudity goes out the moment it comes in.
	r.pass(time.Minute)
	r.a.Reported("nudity")
	if r.emails() != 3 || !strings.Contains(r.last().text, "Nudity or sexual content: 1") {
		t.Fatalf("emails: %d, want nudity sent at once", r.emails())
	}
}

// The report that makes a post or comment hidden while under review is
// urgent whatever its reason: a spam report that hides something goes out
// within a minute of the last email, not the hour, together with what was
// waiting. The email counts it among the reports and says how many hid
// something; an email with none says nothing about it.
func TestAHidingReportGoesOutWithinAMinute(t *testing.T) {
	r := newRig(t)
	r.a.Reported("spam") // sent at once
	if strings.Contains(r.last().text, "Hidden") || strings.Contains(r.last().html, "Hidden") {
		t.Fatalf("an email with no hiding report mentions it: %s", r.last().text)
	}
	r.pass(20 * time.Second)
	r.a.Reported("other")
	if ft := r.live(); ft == nil || ft.d != CalmGap-20*time.Second {
		t.Fatalf("timer: %+v, want the rest of the hour for a calm report", ft)
	}
	r.a.ReportedHiding("spam")
	if ft := r.live(); ft == nil || ft.d != 40*time.Second {
		t.Fatalf("timer: %+v, want the 40 seconds left of the minute", ft)
	}
	r.pass(40 * time.Second)
	if r.emails() != 2 {
		t.Fatalf("emails: %d, want the hiding one sent within the minute", r.emails())
	}
	got := r.last()
	if want := "2 new reports on RNDMRoll.\n\nSpam: 1\nSomething else: 1\n\nHidden while you take a look: 1\n\nSee them on the admin page: " + pageURL + "\n"; got.text != want {
		t.Fatalf("text:\n%q\nwant\n%q", got.text, want)
	}
	if got.subject != "RNDMRoll: 2 new reports" || !strings.Contains(got.html, "</p><p>Hidden while you take a look: 1</p><p><a href=") {
		t.Fatalf("subject %q, html %s", got.subject, got.html)
	}
	// Counted once and then cleared: the next calm report waits the hour
	// again, and its email says nothing of hiding.
	r.pass(2 * time.Minute)
	r.a.Reported("spam")
	if ft := r.live(); ft == nil || ft.d != CalmGap-2*time.Minute {
		t.Fatalf("timer after the hiding email: %+v, want the calm hour", ft)
	}
	r.pass(CalmGap)
	if r.emails() != 3 || strings.Contains(r.last().text, "Hidden") {
		t.Fatalf("emails: %d, the next one: %s", r.emails(), r.last().text)
	}
	// With nothing sent for a minute, it goes out the moment it comes in,
	// and a nil Alerts ignores it.
	r.pass(time.Minute)
	r.a.ReportedHiding("harassment")
	if r.emails() != 4 || !strings.Contains(r.last().text, "Bullying or harassment: 1\n\nHidden while you take a look: 1") {
		t.Fatalf("emails: %d, want it sent at once: %s", r.emails(), r.last().text)
	}
	var none *Alerts
	none.ReportedHiding("spam")
}

func TestCloseSendsWhatIsWaitingAndThenCountsNothing(t *testing.T) {
	r := newRig(t)
	r.a.Reported("spam")
	r.pass(5 * time.Minute)
	r.a.Reported("hate")
	if r.emails() != 1 {
		t.Fatalf("emails before Close: %d", r.emails())
	}
	r.a.Close(context.Background())
	if r.emails() != 2 || !strings.Contains(r.last().text, "Hate speech: 1") {
		t.Fatalf("Close did not send the waiting report: %d", r.emails())
	}
	if r.live() != nil {
		t.Fatal("a timer is still set after Close")
	}
	r.pass(2 * time.Hour)
	r.a.Reported("violence")
	if r.emails() != 2 {
		t.Fatal("a report after Close was emailed")
	}
	r.a.Close(context.Background()) // twice is fine
}

func TestEveryAdminIsEmailedAndAFailureLogsNoAddress(t *testing.T) {
	r := newRig(t, "one@example.com", "two@example.com")
	r.sender.err = errors.New("resend is down")
	r.a.Reported("other")
	if r.emails() != 2 || r.sender.got[0].to != "one@example.com" || r.sender.got[1].to != "two@example.com" {
		t.Fatalf("sent: %+v", r.sender.got)
	}
	if !strings.Contains(r.log.String(), "resend is down") || strings.Contains(r.log.String(), "@example.com") {
		t.Fatalf("log: %s", r.log.String())
	}
}

func TestTheEmailEscapesItsLink(t *testing.T) {
	e := compose(map[string]int{"spam": 1}, 0, `https://x.test/admin/go?a=1&b="2"`)
	if !strings.Contains(e.html, `href="https://x.test/admin/go?a=1&amp;b=&#34;2&#34;"`) {
		t.Fatalf("html: %s", e.html)
	}
}

func TestANilAlertsIgnoresEverything(t *testing.T) {
	var a *Alerts
	a.Reported("violence")
	a.Close(context.Background())
}

// The real deliver sends in the background, and Close waits for it.
func TestCloseWaitsForAnEmailUnderWay(t *testing.T) {
	sender := &fakeSender{}
	a := New(sender, []string{"boss@example.com"}, pageURL, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	a.Reported("violence")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.Close(ctx)
	if len(sender.got) != 1 {
		t.Fatalf("emails once Close returned: %d, want 1", len(sender.got))
	}
}
