package user

import "testing"

func TestDefaultVisibility_PrivateUnderEighteenAndPublicFromEighteen(t *testing.T) {
	cases := []struct {
		age  int
		want string
	}{
		{0, VisibilityPrivate},
		{13, VisibilityPrivate},
		{15, VisibilityPrivate},
		{17, VisibilityPrivate},
		{18, VisibilityPublic},
		{19, VisibilityPublic},
		{30, VisibilityPublic},
		{120, VisibilityPublic},
	}
	for _, c := range cases {
		if got := DefaultVisibility(c.age); got != c.want {
			t.Errorf("DefaultVisibility(%d) = %q, want %q", c.age, got, c.want)
		}
	}
}

func TestDefaultVisibility_TheThresholdIsEighteenAndTheValuesAreTheTwoWords(t *testing.T) {
	// The under-18 rule is one recommendation in one place: this pins the
	// number and the two stored words so a change is a deliberate edit.
	if PrivateUnderAge != 18 {
		t.Errorf("PrivateUnderAge = %d, want 18", PrivateUnderAge)
	}
	if VisibilityPublic != "public" || VisibilityPrivate != "private" {
		t.Errorf("values = %q and %q, want \"public\" and \"private\"", VisibilityPublic, VisibilityPrivate)
	}
}

func TestValidVisibility_OnlyTheTwoLowerCaseWords(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"public", true},
		{"private", true},
		{"", false},
		{"Public", false},
		{"PRIVATE", false},
		{" public", false},
		{"public ", false},
		{"friends", false},
		{"secret", false},
		{"1", false},
		{"null", false},
	}
	for _, c := range cases {
		if got := ValidVisibility(c.in); got != c.want {
			t.Errorf("ValidVisibility(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
