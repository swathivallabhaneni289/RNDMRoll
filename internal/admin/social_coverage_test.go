package admin_test

import (
	"slices"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// SAFE-01 reasons: the reasons the server takes are the seven of the PRD, in
// its order, and the admin page and the report email have words for each,
// the same words the app shows, and a place in the order the admin works
// through them. Violence and nudity, and only they, are urgent; a reason
// that is not one of them comes after every real one.
func TestSocialCoverage_EveryReportReasonHasWordsAndARank(t *testing.T) {
	want := []struct{ key, words string }{
		{"spam", "Spam"},
		{"nudity", "Nudity or sexual content"},
		{"harassment", "Bullying or harassment"},
		{"hate", "Hate speech"},
		{"violence", "Violence or threats"},
		{"impersonation", "Pretending to be someone else"},
		{"other", "Something else"},
	}
	keys := []string{}
	for _, w := range want {
		keys = append(keys, w.key)
	}
	if !slices.Equal(social.ReportReasons, keys) {
		t.Fatalf("social.ReportReasons = %v, want %v", social.ReportReasons, keys)
	}
	if len(admin.ReasonWords) != len(want) {
		t.Fatalf("admin.ReasonWords has %d reasons, want %d", len(admin.ReasonWords), len(want))
	}
	unknown := admin.ReasonRank("no_such_reason")
	ranks := map[int]string{}
	for _, w := range want {
		if got := admin.ReasonWords[w.key]; got != w.words {
			t.Fatalf("admin.ReasonWords[%q] = %q, want %q", w.key, got, w.words)
		}
		rank := admin.ReasonRank(w.key)
		if rank < 0 || rank >= unknown || ranks[rank] != "" {
			t.Fatalf("ReasonRank(%q) = %d: below 0, not ahead of an unknown reason (%d), or shared with %q", w.key, rank, unknown, ranks[rank])
		}
		ranks[rank] = w.key
		if urgent := admin.Urgent(w.key); urgent != (w.key == "violence" || w.key == "nudity") {
			t.Fatalf("Urgent(%q) = %v", w.key, urgent)
		}
	}
	if unknown != len(want) || admin.Urgent("no_such_reason") {
		t.Fatalf("an unknown reason ranks %d (urgent %v), want %d and not urgent", unknown, admin.Urgent("no_such_reason"), len(want))
	}
	// The order the admin works in: violence and nudity, then bullying and
	// hate, then the rest.
	order := []string{}
	for rank := range len(want) {
		order = append(order, ranks[rank])
	}
	if !slices.Equal(order[:2], []string{"violence", "nudity"}) || !slices.Equal(order[2:4], []string{"harassment", "hate"}) {
		t.Fatalf("the admin's order: %v", order)
	}
}
