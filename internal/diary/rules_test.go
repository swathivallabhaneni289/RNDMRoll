package diary

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

func TestDiaryCursorRoundTripAndRefusals(t *testing.T) {
	for _, d := range []string{"2026-10-08", "2028-02-29"} {
		got, err := DecodeCursor(EncodeCursor(d))
		if err != nil || got != d {
			t.Fatalf("round trip %s: %q %v", d, got, err)
		}
	}
	if d, err := DecodeCursor(""); d != "" || err != nil {
		t.Fatalf("empty cursor: %q %v", d, err)
	}
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, bad := range []string{
		"abc", b64("v2:2026-10-08"), b64("v1:2026-13-01"), b64("v1:2026-02-30"),
		base64.URLEncoding.EncodeToString([]byte("v1:2026-10-0")), strings.Repeat("a", 200),
	} {
		if _, err := DecodeCursor(bad); !errors.Is(err, ErrBadCursor) {
			t.Errorf("cursor %q: %v, want ErrBadCursor", bad, err)
		}
	}
}

func TestDiaryLimit(t *testing.T) {
	if n, err := ParseLimit(""); n != 20 || err != nil {
		t.Fatalf("empty: %d %v", n, err)
	}
	for _, ok := range []string{"1", "50"} {
		if _, err := ParseLimit(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"0", "51", "-1", "abc", "1.5", " 5", "1e1", "+5", "0x10"} {
		if _, err := ParseLimit(bad); !errors.Is(err, ErrBadLimit) {
			t.Errorf("%q: %v, want ErrBadLimit", bad, err)
		}
	}
}

func TestDiaryAverageTenths(t *testing.T) {
	for _, c := range [][3]int{{9, 1, 45}, {10, 1, 50}, {1, 1, 5}, {19, 2, 48}, {7, 2, 18}, {83, 9, 46}, {100, 10, 50}, {1000, 1000, 5}} {
		if got := AverageTenths(c[0], c[1]); got != c[2] {
			t.Errorf("AverageTenths(%d,%d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
	if AverageTenths(0, 0) != 0 {
		t.Error("count 0 is not 0")
	}
}

func TestDiaryWeekStreak(t *testing.T) {
	cases := []struct {
		name string
		cur  string
		set  []string
		want int
	}{
		{"none", "2026-10-05", nil, 0},
		{"current alone", "2026-10-05", []string{"2026-10-05"}, 1},
		{"current plus three", "2026-10-05", []string{"2026-10-05", "2026-09-28", "2026-09-21", "2026-09-14"}, 4},
		{"current missing", "2026-10-05", []string{"2026-09-28", "2026-09-21", "2026-09-14"}, 3},
		{"gap", "2026-10-05", []string{"2026-10-05", "2026-09-21"}, 1},
		{"long ago", "2026-10-05", []string{"2026-01-05"}, 0},
		{"junk and duplicates", "2026-10-05", []string{"x", "2026-10-05", "2026-10-05", ""}, 1},
		{"year end", "2026-12-28", []string{"2026-12-28", "2026-12-21"}, 2},
		{"leap day", "2028-03-06", []string{"2028-03-06", "2028-02-28"}, 2},
	}
	for _, c := range cases {
		if got := WeekStreak(c.cur, c.set); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

// fakeReader records calls and returns canned data.
type fakeReader struct {
	rows  []Row
	zone  string
	weeks []string
	calls int
}

func (f *fakeReader) Rows(_ context.Context, _ string, _ *string, _ string, limit int) ([]Row, error) {
	f.calls++
	if len(f.rows) > limit {
		return f.rows[:limit], nil
	}
	return f.rows, nil
}
func (f *fakeReader) Chips(context.Context, string) ([]Chip, error) {
	f.calls++
	return []Chip{{ID: "c", Name: "Meal"}}, nil
}
func (f *fakeReader) SpinDays(context.Context, string) (int, error) { f.calls++; return 7, nil }
func (f *fakeReader) GoalWeekStarts(context.Context, string, int) ([]string, error) {
	f.calls++
	return f.weeks, nil
}
func (f *fakeReader) Tastes(context.Context, string) ([]RawTaste, error) {
	f.calls++
	return []RawTaste{{Chip: Chip{ID: "c", Name: "Meal"}, SumStars: 19, Count: 2}}, nil
}
func (f *fakeReader) Recent(context.Context, string, int) ([]spin.Entry, error) {
	f.calls++
	return nil, nil
}
func (f *fakeReader) TimeZone(context.Context, string) (string, error) { f.calls++; return f.zone, nil }

type fiveFriends struct{}

func (fiveFriends) Count(context.Context, string) (int, error) { return 5, nil }

func TestDiaryServicePaging(t *testing.T) {
	f := &fakeReader{rows: []Row{{LocalDate: "2026-10-09"}, {LocalDate: "2026-10-08"}, {LocalDate: "2026-10-07"}}}
	s := NewService(f, clock.NewFixed(time.Now()), nil)
	p, err := s.Page(context.Background(), "u", PageParams{Limit: 2})
	if err != nil || len(p.Rows) != 2 || p.NextCursor != EncodeCursor("2026-10-08") || len(p.Chips) != 1 {
		t.Fatalf("page: %+v %v", p, err)
	}
	p, _ = s.Page(context.Background(), "u", PageParams{Limit: 3})
	if len(p.Rows) != 3 || p.NextCursor != "" {
		t.Fatalf("exact page invents a cursor: %+v", p)
	}
	before := f.calls
	if _, err := s.Page(context.Background(), "u", PageParams{Cursor: "abc", Limit: 2}); !errors.Is(err, ErrBadCursor) || f.calls != before {
		t.Fatalf("bad cursor read the database: %v", err)
	}
}

func TestDiaryServiceSummaryUsesLocalWeek(t *testing.T) {
	// 2026-10-11 20:00 UTC is Monday 2026-10-12 in Kiritimati (UTC+14), so the
	// current week starts 2026-10-12, not 2026-10-05.
	now := time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	f := &fakeReader{zone: "Pacific/Kiritimati", weeks: []string{"2026-10-12", "2026-10-05"}}
	s := NewService(f, clock.NewFixed(now), fiveFriends{})
	sum, err := s.Summary(context.Background(), "u")
	if err != nil || sum.WeekStreak != 2 || sum.Friends != 5 || sum.Spins != 7 {
		t.Fatalf("summary: %+v %v", sum, err)
	}
	if sum.Tastes[0].Tenths != 48 || sum.Tastes[0].Count != 2 {
		t.Fatalf("taste: %+v", sum.Tastes[0])
	}
	f.zone = ""
	sum, err = s.Summary(context.Background(), "u")
	if err != nil || sum.WeekStreak != 0 {
		t.Fatalf("no zone: %+v %v", sum, err)
	}
}
