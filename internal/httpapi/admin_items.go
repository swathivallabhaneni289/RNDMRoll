package httpapi

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

// admin_items.go puts the reports on one post, comment or account together,
// so the admin page shows each reported thing once, with what it is, who
// reported it and why (developer, 2026-11-17: "the reported posts can be
// shown in a better way").

// adminItem is one reported post, comment or account as the page shows it:
// every report on it, together.
type adminItem struct {
	Kind string
	// Owner is whose post, comment or account it is.
	Owner admin.Person
	// Report is the newest report in the group. It carries the post or the
	// comment as it is now, and its id is the one Remove names.
	Report admin.Report
	// IDs are the ids of every report in the group: what Dismiss closes.
	// Ats are their times, in the same order, which a person's Dismiss
	// sends too (AtValues): a person's report is named by its reporter, and
	// a report made again since has a new time.
	IDs []uuid.UUID
	Ats []time.Time
	// Reporters are their usernames, newest first, each once.
	Reporters []string
	// Reasons are each reason once, the most serious first (admin.ReasonRank).
	Reasons  []string
	NewestAt time.Time
	OldestAt time.Time
	// Open is false for a group of closed reports, which then carries how
	// and when they were closed (one action closed them all).
	Open       bool
	Resolution string
	ResolvedAt *time.Time
	// Hidden says an open post or comment is hidden while under review now,
	// as the database worked it out (admin.Report.Hidden). Never on a closed
	// group, and never counted from IDs or Reporters: those stop at the
	// page's limit and leave out reporters with no username.
	Hidden bool
}

// adminCard is what the "item" template draws: an item, the page's CSRF
// token, and the page it is on ("list" or "person"), which decides whether
// the owner shows and where Dismiss goes back to.
type adminCard struct {
	Item adminItem
	CSRF string
	From string
}

// tier is how urgent it is by its most serious reason: 0 for violence or
// nudity, 1 for bullying or hate, 2 for anything else.
func (it adminItem) tier() int {
	if len(it.Reasons) == 0 {
		return 2
	}
	switch rank := admin.ReasonRank(it.Reasons[0]); {
	case admin.Urgent(it.Reasons[0]):
		return 0
	case rank <= admin.ReasonRank("hate"):
		return 1
	}
	return 2
}

// removeReasons are the report reasons in admin.ReasonRank order, the most
// serious first: the Reason list beside Remove (developer, 2026-11-18).
var removeReasons = func() []string {
	keys := make([]string, 0, len(admin.ReasonWords))
	for k := range admin.ReasonWords {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if d := admin.ReasonRank(a) - admin.ReasonRank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return keys
}()

// RemoveReason is the reason Remove's list starts on: the most serious one
// it was reported for, or Something else ("other") for a group with no
// reason the list has.
func (it adminItem) RemoveReason() string {
	if len(it.Reasons) > 0 {
		if _, ok := admin.ReasonWords[it.Reasons[0]]; ok {
			return it.Reasons[0]
		}
	}
	return "other"
}

// AtValues are Ats as the Dismiss form sends them: in UTC to the
// nanosecond, so each parses back to exactly the time the database gave.
func (it adminItem) AtValues() []string {
	out := make([]string, len(it.Ats))
	for i, at := range it.Ats {
		out[i] = at.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// ReportedBy names who reported it: up to three usernames, then how many
// more.
func (it adminItem) ReportedBy() string {
	names := make([]string, 0, 3)
	for _, r := range it.Reporters {
		if len(names) == 3 {
			break
		}
		names = append(names, "@"+r)
	}
	if more := len(it.Reporters) - len(names); more > 0 {
		return strings.Join(names, ", ") + " and " + strconv.Itoa(more) + " more"
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// historyEntry is one line of a person's History: a report about them, a
// decision from the log (developer, 2026-11-19: "the history section, we'll
// have it there"), or a second look they asked for.
type historyEntry struct {
	At         time.Time
	Report     *admin.Report
	Event      *admin.Event
	SecondLook *admin.SecondLook
	// stored is when a decision or a second look was stored, on the
	// database's clock. Both are dated on the owner's clock, which can stand
	// still (a test clock): at the same At, the one stored later is newer,
	// so a request shows between its removal and its answer.
	stored time.Time
}

// history is every report about the person, every decision about them and
// every second look they asked for, the newest first. At the same moment
// the decisions and second looks come before the reports, the one stored
// later first, and each list keeps its own order otherwise.
func history(reports []admin.Report, events []admin.Event, looks []admin.SecondLook) []historyEntry {
	out := make([]historyEntry, 0, len(reports)+len(events)+len(looks))
	for i := range events {
		out = append(out, historyEntry{At: events[i].At, Event: &events[i], stored: events[i].CreatedAt})
	}
	for i := range looks {
		out = append(out, historyEntry{At: looks[i].At, SecondLook: &looks[i], stored: looks[i].CreatedAt})
	}
	for i := range reports {
		out = append(out, historyEntry{At: reports[i].CreatedAt, Report: &reports[i]})
	}
	slices.SortStableFunc(out, func(a, b historyEntry) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		if (a.Report == nil) != (b.Report == nil) {
			if a.Report == nil {
				return -1
			}
			return 1
		}
		return b.stored.Compare(a.stored)
	})
	return out
}

// groupKey is what reports about the same thing share: the post, the
// comment, or the account itself. An open report whose post or comment is
// gone keeps a card of its own, since it no longer says which one it was.
// Closed reports share a card when one action closed them: the same thing
// (or, once it is gone, the same kind), the same way, at the same time.
func groupKey(r admin.Report) string {
	var key string
	switch {
	case r.Kind == admin.KindPost && r.EntryID != nil:
		key = "post:" + r.EntryID.String()
	case r.Kind == admin.KindComment && r.CommentID != nil:
		key = "comment:" + r.CommentID.String()
	case r.Kind == admin.KindPerson:
		key = "person"
	case r.ResolvedAt != nil:
		key = r.Kind + ":gone"
	default:
		return r.Kind + ":report:" + r.ID.String()
	}
	if r.ResolvedAt != nil {
		key += "|" + r.Resolution + "|" + r.ResolvedAt.UTC().Format(time.RFC3339Nano)
	}
	return key
}

// groupReports puts one person's reports (as Reports reads them: open first,
// newest first) together by what they are about. Open groups come back in
// sortOpen's order, closed ones the most recently closed first.
func groupReports(owner admin.Person, reports []admin.Report) (open, closed []adminItem) {
	openAt, closedAt := map[string]int{}, map[string]int{}
	for _, r := range reports {
		list, at := &open, openAt
		if r.ResolvedAt != nil {
			list, at = &closed, closedAt
		}
		key := groupKey(r)
		i, ok := at[key]
		if !ok {
			i = len(*list)
			at[key] = i
			*list = append(*list, adminItem{Kind: r.Kind, Owner: owner, Report: r, NewestAt: r.CreatedAt,
				OldestAt: r.CreatedAt, Open: r.ResolvedAt == nil, Resolution: r.Resolution, ResolvedAt: r.ResolvedAt})
		}
		it := &(*list)[i]
		it.IDs = append(it.IDs, r.ID)
		it.Ats = append(it.Ats, r.CreatedAt)
		it.Hidden = it.Hidden || (r.ResolvedAt == nil && r.Hidden)
		if r.Reporter != "" && !slices.Contains(it.Reporters, r.Reporter) {
			it.Reporters = append(it.Reporters, r.Reporter)
		}
		if !slices.Contains(it.Reasons, r.Reason) {
			it.Reasons = append(it.Reasons, r.Reason)
		}
		if r.CreatedAt.After(it.NewestAt) {
			it.NewestAt, it.Report = r.CreatedAt, r
		}
		if r.CreatedAt.Before(it.OldestAt) {
			it.OldestAt = r.CreatedAt
		}
	}
	for _, list := range [][]adminItem{open, closed} {
		for i := range list {
			slices.SortStableFunc(list[i].Reasons, func(a, b string) int { return admin.ReasonRank(a) - admin.ReasonRank(b) })
		}
	}
	sortOpen(open)
	slices.SortStableFunc(closed, func(a, b adminItem) int { return b.ResolvedAt.Compare(*a.ResolvedAt) })
	return open, closed
}

// sortOpen puts open groups in the order the admin works through them: the
// ones hidden while under review first (nobody but their owners sees them
// until the admin decides), then by tier, then the most reported, then the
// longest waiting.
func sortOpen(items []adminItem) {
	slices.SortStableFunc(items, func(a, b adminItem) int {
		if a.Hidden != b.Hidden {
			if a.Hidden {
				return -1
			}
			return 1
		}
		if a.tier() != b.tier() {
			return a.tier() - b.tier()
		}
		if len(a.IDs) != len(b.IDs) {
			return len(b.IDs) - len(a.IDs)
		}
		return a.OldestAt.Compare(b.OldestAt)
	})
}
