package board

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// TestThread puts each top-level comment before its replies, keeps the given
// order inside each group, and drops a reply whose top-level comment is not
// in the list.
func TestThread(t *testing.T) {
	id := func() uuid.UUID { return uuid.New() }
	top1, top2, hidden := id(), id(), id()
	r1, r2, orphan := id(), id(), id()
	row := func(i uuid.UUID, parent *uuid.UUID) CommentRow { return CommentRow{ID: i, ParentID: parent} }
	// Oldest first, as the repository reads them.
	rows := []CommentRow{row(top1, nil), row(top2, nil), row(r1, &top1), row(orphan, &hidden), row(r2, &top1)}
	got := Thread(rows)
	want := []uuid.UUID{top1, r1, r2, top2}
	if len(got) != len(want) {
		t.Fatalf("got %d comments, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("position %d: got %v, want %v", i, got[i].ID, want[i])
		}
	}
	if len(Thread(nil)) != 0 {
		t.Fatal("no comments")
	}
}

func TestCleanComment(t *testing.T) {
	good := map[string]string{
		"  hi  ":                 "hi",
		"a\r\nb":                 "a\nb",
		strings.Repeat("é", 140): strings.Repeat("é", 140),
	}
	for in, want := range good {
		if got, p := CleanComment(in); p != nil || got != want {
			t.Errorf("%q: %q %v", in, got, p)
		}
	}
	bad := map[string]string{
		"":                       "required",
		"   ":                    "required",
		strings.Repeat("é", 141): "too_long",
		"a\u0007b":               "invalid",
		"a\tb":                   "invalid",
	}
	for in, want := range bad {
		if _, p := CleanComment(in); p == nil || p.Reason != want || p.Field != "text" {
			t.Errorf("%q: %v, want %s", in, p, want)
		}
	}
}

func TestDecide(t *testing.T) {
	for i := 0; i < 8; i++ {
		follows, posted, skipped := i&1 != 0, i&2 != 0, i&4 != 0
		want := StateLocked
		switch {
		case !follows:
			want = StateNoFollows
		case posted:
			want = StateUnlocked
		case skipped:
			want = StateSkipped
		}
		if got := Decide(follows, posted, skipped); got != want {
			t.Errorf("%v %v %v: %s, want %s", follows, posted, skipped, got, want)
		}
	}
}

func TestDayLabelAndFirstName(t *testing.T) {
	d := func(s string) spin.Date {
		v, err := spin.ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := [][3]string{
		{"2026-10-08", "2026-10-08", LabelToday},
		{"2026-10-07", "2026-10-08", LabelYesterday},
		{"2026-09-30", "2026-10-01", LabelYesterday},
		{"2026-12-31", "2027-01-01", LabelYesterday},
		{"2026-10-06", "2026-10-08", LabelEarlier},
	}
	for _, c := range cases {
		if got := DayLabel(d(c[0]), d(c[1])); got != c[2] {
			t.Errorf("%s vs %s: %s, want %s", c[0], c[1], got, c[2])
		}
	}
	if FirstName("Bob Brown") != "Bob" || FirstName("  Ann") != "Ann" || FirstName("") != "" ||
		len([]rune(FirstName(strings.Repeat("x", 30)))) != 20 {
		t.Error("FirstName")
	}
}
