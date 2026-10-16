package board

import (
	"strings"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

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
