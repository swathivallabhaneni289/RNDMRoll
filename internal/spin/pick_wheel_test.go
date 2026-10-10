package spin

import (
	"errors"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

func seeded(a, b uint64) Rand { return rand.New(rand.NewPCG(a, b)) }

// --- Pick and PickOther ---

func TestPick_FollowsTheWeights(t *testing.T) {
	weights := []int{3, 1, 3, 1, 2, 1}
	const draws = 120000
	total := 0
	for _, w := range weights {
		total += w
	}
	counts := make([]int, len(weights))
	r := seeded(20261007, 42)
	for i := 0; i < draws; i++ {
		idx := Pick(weights, r)
		if idx < 0 || idx >= len(weights) {
			t.Fatalf("Pick = %d", idx)
		}
		counts[idx]++
	}
	for i, w := range weights {
		want := float64(w) / float64(total)
		got := float64(counts[i]) / draws
		if math.Abs(got-want) > 0.01 {
			t.Errorf("entry %d (weight %d): share %.4f, want %.4f within 1 point", i, w, got, want)
		}
	}
}

func TestPick_NothingToPickIsMinusOne(t *testing.T) {
	r := seeded(1, 1)
	for _, w := range [][]int{nil, {}, {0}, {0, 0}, {-3, 0}} {
		if got := Pick(w, r); got != -1 {
			t.Errorf("Pick(%v) = %d, want -1", w, got)
		}
	}
	if got := Pick([]int{0, 5, 0}, r); got != 1 {
		t.Errorf("Pick with one positive weight = %d, want 1", got)
	}
}

func TestPickOther_NeverReturnsTheCurrentEntry(t *testing.T) {
	entries := []WheelEntry{
		{ID: "a", Name: "Meal", Weight: 3}, {ID: "b", Name: "Movie", Weight: 1},
		{ID: "c", Name: "Song", Weight: 3}, {ID: "d", Name: "Book", Weight: 1},
		{ID: "e", Name: "Place", Weight: 2}, {ID: "f", Name: "Game", Weight: 1},
	}
	r := seeded(7, 11)
	for _, current := range []string{"a", "c", "f"} {
		for i := 0; i < 20000; i++ {
			idx, err := PickOther(entries, current, r)
			if err != nil {
				t.Fatalf("PickOther: %v", err)
			}
			if entries[idx].ID == current {
				t.Fatalf("PickOther returned the current entry %q", current)
			}
		}
	}
}

func TestPickOther_StillFollowsTheRemainingWeights(t *testing.T) {
	entries := []WheelEntry{{ID: "a", Weight: 5}, {ID: "b", Weight: 1}, {ID: "c", Weight: 3}}
	counts := map[string]int{}
	r := seeded(3, 9)
	const draws = 60000
	for i := 0; i < draws; i++ {
		idx, err := PickOther(entries, "a", r)
		if err != nil {
			t.Fatal(err)
		}
		counts[entries[idx].ID]++
	}
	// b and c remain, 1 : 3.
	if got := float64(counts["c"]) / draws; math.Abs(got-0.75) > 0.01 {
		t.Errorf("share of c = %.4f, want 0.75 within 1 point", got)
	}
}

func TestPickOther_NoOtherEntryIsAnErrorNotAPanic(t *testing.T) {
	r := seeded(1, 2)
	cases := map[string][]WheelEntry{
		"empty":         nil,
		"only current":  {{ID: "a", Weight: 3}},
		"same id twice": {{ID: "a", Weight: 3}, {ID: "a", Weight: 1}},
		"zero weights":  {{ID: "a", Weight: 3}, {ID: "b", Weight: 0}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			idx, err := PickOther(entries, "a", r)
			if !errors.Is(err, ErrNoOther) || idx != -1 {
				t.Fatalf("PickOther = %d, %v; want -1, ErrNoOther", idx, err)
			}
		})
	}
	// A current id that is not on the wheel at all leaves every entry open.
	idx, err := PickOther([]WheelEntry{{ID: "a", Weight: 1}}, "gone", r)
	if err != nil || idx != 0 {
		t.Fatalf("PickOther with an unknown current id = %d, %v", idx, err)
	}
}

// --- the wheel ---

func ptr(s string) *string { return &s }

func wheelOf(names ...string) []WheelInput {
	out := make([]WheelInput, len(names))
	for i, n := range names {
		out[i] = WheelInput{Name: n, Weight: 1}
	}
	return out
}

func wantWheelError(t *testing.T, err error, reason string, index int) {
	t.Helper()
	var we *WheelError
	if !errors.As(err, &we) {
		t.Fatalf("error = %v, want a *WheelError with reason %q", err, reason)
	}
	if !errors.Is(err, ErrWheelInvalid) {
		t.Errorf("%v does not match ErrWheelInvalid", err)
	}
	if we.Reason != reason || we.Index != index {
		t.Fatalf("reason %q index %d, want %q index %d", we.Reason, we.Index, reason, index)
	}
}

func TestDefaultWheel_IsTheStartingWheelInOrder(t *testing.T) {
	got := DefaultWheel()
	want := []struct {
		name   string
		weight int
	}{{"Meal", 3}, {"Movie", 1}, {"Song", 3}, {"Book", 1}, {"Place", 2}, {"Game", 1}}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Weight != w.weight || got[i].ID != nil {
			t.Errorf("entry %d = %+v, want %s %d with no id", i, got[i], w.name, w.weight)
		}
	}
	if err := ValidateWheel(got); err != nil {
		t.Errorf("the starting wheel is not valid: %v", err)
	}
}

func TestValidateWheel_CountsAndBoundaries(t *testing.T) {
	wantWheelError(t, ValidateWheel(nil), ReasonTooFew, -1)
	wantWheelError(t, ValidateWheel(wheelOf("A")), ReasonTooFew, -1)
	if err := ValidateWheel(wheelOf("A", "B")); err != nil {
		t.Errorf("2 entries: %v", err)
	}
	twelve := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}
	if err := ValidateWheel(wheelOf(twelve...)); err != nil {
		t.Errorf("12 entries: %v", err)
	}
	wantWheelError(t, ValidateWheel(wheelOf(append(twelve, "M")...)), ReasonTooMany, -1)
}

func TestValidateWheel_NamesAndWeights(t *testing.T) {
	ok24 := strings.Repeat("n", 24)
	if err := ValidateWheel([]WheelInput{{Name: ok24, Weight: 1}, {Name: "B", Weight: 10}}); err != nil {
		t.Errorf("24 characters and weight 10: %v", err)
	}
	wantWheelError(t, ValidateWheel([]WheelInput{{Name: "A", Weight: 1}, {Name: strings.Repeat("n", 25), Weight: 1}}), ReasonNameTooLong, 1)
	// 24 multi-byte characters are fine, 25 are not (characters, not bytes).
	if err := ValidateWheel([]WheelInput{{Name: strings.Repeat("é", 24), Weight: 1}, {Name: "B", Weight: 1}}); err != nil {
		t.Errorf("24 two-byte characters: %v", err)
	}
	wantWheelError(t, ValidateWheel([]WheelInput{{Name: "A", Weight: 1}, {Name: strings.Repeat("é", 25), Weight: 1}}), ReasonNameTooLong, 1)
	// Spaces around a 24 character name do not count against it.
	if err := ValidateWheel([]WheelInput{{Name: "  " + ok24 + "  ", Weight: 1}, {Name: "B", Weight: 1}}); err != nil {
		t.Errorf("24 characters with spaces: %v", err)
	}

	for _, name := range []string{"", "   ", "\t", "A\x00B", "tab\there", "line\nbreak", "​", "bell\a", "bad\xffutf8"} {
		wantWheelError(t, ValidateWheel([]WheelInput{{Name: "A", Weight: 1}, {Name: name, Weight: 1}}), ReasonNameRequired, 1)
	}

	for _, w := range []int{0, -1, 11, 100} {
		wantWheelError(t, ValidateWheel([]WheelInput{{Name: "A", Weight: 1}, {Name: "B", Weight: w}}), ReasonWeightRange, 1)
	}
	for _, w := range []int{1, 10} {
		if err := ValidateWheel([]WheelInput{{Name: "A", Weight: w}, {Name: "B", Weight: w}}); err != nil {
			t.Errorf("weight %d: %v", w, err)
		}
	}
}

func TestValidateWheel_DuplicatesIgnoreCapitalsAndSpaces(t *testing.T) {
	wantWheelError(t, ValidateWheel(wheelOf("Meal", "Song", "meal")), ReasonDuplicateName, 2)
	wantWheelError(t, ValidateWheel(wheelOf("Meal", " MEAL ")), ReasonDuplicateName, 1)
	if err := ValidateWheel(wheelOf("Meal", "Meals")); err != nil {
		t.Errorf("different names: %v", err)
	}

	same := ptr("0b8a3c1e-1111-4222-8333-444455556666")
	list := []WheelInput{{ID: same, Name: "A", Weight: 1}, {ID: same, Name: "B", Weight: 1}}
	wantWheelError(t, ValidateWheel(list), ReasonDuplicateID, 1)
	// The same id in another case is still the same id.
	upper := ptr("0B8A3C1E-1111-4222-8333-444455556666")
	list = []WheelInput{{ID: same, Name: "A", Weight: 1}, {ID: upper, Name: "B", Weight: 1}}
	wantWheelError(t, ValidateWheel(list), ReasonDuplicateID, 1)
	// Two new categories (no id) are not duplicates of each other.
	if err := ValidateWheel(wheelOf("A", "B")); err != nil {
		t.Errorf("two new rows: %v", err)
	}
}

func TestValidateWheel_ReportsTheFirstProblemInRowOrder(t *testing.T) {
	list := []WheelInput{
		{Name: "A", Weight: 1},
		{Name: "B", Weight: 99}, // weight problem at 1
		{Name: "", Weight: 1},   // name problem at 2
	}
	wantWheelError(t, ValidateWheel(list), ReasonWeightRange, 1)
}

func TestCleanName_TrimsOnly(t *testing.T) {
	if got := CleanName("  Meal \t"); got != "Meal" {
		t.Errorf("CleanName = %q", got)
	}
	if got := CleanName("a  b"); got != "a  b" {
		t.Errorf("CleanName changed inner spaces: %q", got)
	}
}
