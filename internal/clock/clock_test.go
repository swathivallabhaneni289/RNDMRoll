package clock

import (
	"testing"
	"time"
)

func TestReal_ReadsTheWallClock(t *testing.T) {
	before := time.Now()
	got := Real{}.NowFor("anyone")
	after := time.Now()
	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Fatalf("Real.NowFor = %v, want between %v and %v", got, before, after)
	}
}

func TestPerUser_KeepsTickingAfterSetNow(t *testing.T) {
	c := NewPerUser()
	target := time.Date(2030, 1, 2, 20, 0, 0, 0, time.UTC)
	c.SetNow("a", target)

	first := c.NowFor("a")
	if d := first.Sub(target); d < 0 || d > 2*time.Second {
		t.Fatalf("first reading is %v from the target, want 0 to 2s", d)
	}
	time.Sleep(60 * time.Millisecond)
	second := c.NowFor("a")
	if !second.After(first) {
		t.Fatalf("second reading %v is not after the first %v: the moved clock stopped", second, first)
	}
	if d := second.Sub(first); d < 50*time.Millisecond || d > 2*time.Second {
		t.Fatalf("clock advanced %v in a 60ms sleep", d)
	}
}

func TestPerUser_ResetReturnsToRealTime(t *testing.T) {
	c := NewPerUser()
	c.SetNow("a", time.Date(2040, 6, 1, 12, 0, 0, 0, time.UTC))
	if !c.Shifted("a") {
		t.Fatal("Shifted = false right after SetNow")
	}
	c.Reset("a")
	if c.Shifted("a") {
		t.Fatal("Shifted = true after Reset")
	}
	if d := time.Since(c.NowFor("a")); d < -2*time.Second || d > 2*time.Second {
		t.Fatalf("after Reset the clock is %v away from real time", d)
	}
}

func TestPerUser_OneAccountMovedLeavesTheOtherReal(t *testing.T) {
	c := NewPerUser()
	c.SetNow("a", time.Date(2035, 3, 4, 5, 6, 7, 0, time.UTC))

	if d := time.Since(c.NowFor("b")); d < -2*time.Second || d > 2*time.Second {
		t.Fatalf("account b is %v from real time after only a was moved", d)
	}
	if c.Shifted("b") {
		t.Fatal("account b reports a shift it never had")
	}
	if got := c.NowFor("a"); got.Year() != 2035 {
		t.Fatalf("account a reads %v, want a moved time in 2035", got)
	}
}

func TestPerUser_CanMoveBackwards(t *testing.T) {
	c := NewPerUser()
	past := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)
	c.SetNow("a", past)
	if d := c.NowFor("a").Sub(past); d < 0 || d > 2*time.Second {
		t.Fatalf("a clock moved into the past reads %v from the target", d)
	}
}

func TestFixed_SetAndAdvance(t *testing.T) {
	start := time.Date(2026, 10, 7, 19, 59, 59, 0, time.UTC)
	f := NewFixed(start)
	if got := f.NowFor("x"); !got.Equal(start) {
		t.Fatalf("NowFor = %v, want %v", got, start)
	}
	time.Sleep(20 * time.Millisecond)
	if got := f.NowFor("y"); !got.Equal(start) {
		t.Fatalf("a Fixed clock moved on its own: %v", got)
	}
	f.Advance(time.Second)
	if got := f.NowFor("x"); !got.Equal(start.Add(time.Second)) {
		t.Fatalf("after Advance(1s) = %v", got)
	}
	f.Advance(-2 * time.Second)
	if got := f.NowFor("x"); !got.Equal(start.Add(-time.Second)) {
		t.Fatalf("after Advance(-2s) = %v", got)
	}
	later := start.Add(48 * time.Hour)
	f.Set(later)
	if got := f.NowFor("x"); !got.Equal(later) {
		t.Fatalf("after Set = %v, want %v", got, later)
	}
}

func TestFixed_SatisfiesClockControl(t *testing.T) {
	f := NewFixed(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	var ctl ClockControl = f
	target := time.Date(2026, 5, 5, 5, 5, 5, 0, time.UTC)
	ctl.SetNow("someone", target)
	if got := f.NowFor("other"); !got.Equal(target) {
		t.Fatalf("SetNow did not move the Fixed clock: %v", got)
	}
	ctl.Reset("someone")
	if ctl.Shifted("someone") {
		t.Fatal("a Fixed clock reports a shift")
	}
}
