package admin_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

// A person's page shows their posts only while a report about the account
// itself is open, or while it is disabled (Disable closes those reports): a
// report about one post or comment, or an account report already closed,
// shows none (developer, 2026-11-18).
func TestShowsPosts_OnlyWhileTheAccountIsReportedOrDisabled(t *testing.T) {
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	open := func(kind string) admin.Report { return admin.Report{Kind: kind, ID: uuid.New()} }
	closed := func(kind string) admin.Report {
		r := open(kind)
		r.ResolvedAt = &at
		return r
	}
	for _, c := range []struct {
		name    string
		person  admin.Person
		reports []admin.Report
		want    bool
	}{
		{"no reports", admin.Person{}, nil, false},
		{"an open account report", admin.Person{}, []admin.Report{open(admin.KindPost), open(admin.KindPerson)}, true},
		{"an account report that is closed", admin.Person{}, []admin.Report{closed(admin.KindPerson)}, false},
		{"open post and comment reports only", admin.Person{}, []admin.Report{open(admin.KindPost), open(admin.KindComment)}, false},
		{"disabled", admin.Person{DisabledAt: &at}, []admin.Report{closed(admin.KindPerson)}, true},
	} {
		if got := admin.ShowsPosts(c.person, c.reports); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
