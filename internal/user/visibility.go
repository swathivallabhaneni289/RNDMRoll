package user

// The two values of users.account_visibility. Nothing else is ever stored:
// the column has a check for exactly these two lower-case words.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// PrivateUnderAge is the age below which a new account STARTS private. It is
// only the starting value: the person can switch to public at any time, like
// anyone else.
const PrivateUnderAge = 18

// ValidVisibility reports whether s is exactly "public" or "private". Case
// matters: "Public", "PRIVATE" and "" are not valid.
func ValidVisibility(s string) bool {
	return s == VisibilityPublic || s == VisibilityPrivate
}

// DefaultVisibility is the value a NEW account starts with, given its age on
// the day it is created: private under PrivateUnderAge, public otherwise.
//
// This one function is the whole under-18 rule, and it is a recommendation the
// developer can drop: making it return VisibilityPublic for every age removes
// the rule. Sign-up is the only caller.
func DefaultVisibility(age int) string {
	if age < PrivateUnderAge {
		return VisibilityPrivate
	}
	return VisibilityPublic
}
