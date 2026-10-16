package notifications

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

func TestSectionsAgreeWithLabels(t *testing.T) {
	clockRe := regexp.MustCompile(`^\d{1,2}:\d{2} (AM|PM)$`)
	weekdays := "Mon Tue Wed Thu Fri Sat Sun"
	monthDay := regexp.MustCompile(`^[A-Z][a-z]{2} [1-9]\d?$`)
	for _, zone := range []string{"America/New_York", "Asia/Kolkata", "Pacific/Kiritimati"} {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
		for _, now := range []time.Time{
			time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 8, 17, 0, 0, 0, time.UTC),
		} {
			for at := now.Add(-40 * 24 * time.Hour); at.Before(now.Add(2 * time.Hour)); at = at.Add(97 * time.Minute) {
				section := SectionOf(at, now, loc)
				label := spin.ViewerLabel(at, now, loc)
				ok := false
				switch section {
				case SectionToday:
					ok = clockRe.MatchString(label)
				case SectionWeek:
					ok = strings.Contains(weekdays, label) && len(label) == 3 && label == at.In(loc).Format("Mon")
				case SectionEarlier:
					ok = monthDay.MatchString(label)
				}
				if !ok {
					t.Fatalf("%s %v (now %v): section %s label %q", zone, at, now, section, label)
				}
			}
		}
	}
}

func TestSectionEdges(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 10, 8, 21, 0, 0, 0, ny)
	cases := []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 10, 8, 0, 0, 0, 0, ny), SectionToday},
		{time.Date(2026, 10, 7, 23, 59, 59, 0, ny), SectionWeek},
		{time.Date(2026, 10, 2, 0, 0, 0, 0, ny), SectionWeek},
		{time.Date(2026, 10, 1, 23, 59, 59, 0, ny), SectionEarlier},
		{now.Add(time.Hour), SectionToday},
	}
	for _, c := range cases {
		if got := SectionOf(c.at, now, ny); got != c.want {
			t.Errorf("%v: %s, want %s", c.at, got, c.want)
		}
	}
}

func TestCutText(t *testing.T) {
	s60 := strings.Repeat("é", 60)
	if CutText(s60, 60) != s60 {
		t.Error("60 stays")
	}
	if got := CutText(s60+"x", 60); got != s60+"..." {
		t.Errorf("61: %q", got)
	}
	if got := CutText("a\nb", 60); got != "a b" {
		t.Errorf("line break: %q", got)
	}
}
