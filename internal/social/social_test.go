package social

import "testing"

func TestCanViewMatrix(t *testing.T) {
	for i := 0; i < 32; i++ {
		f := Facts{
			IsOwner:         i&1 != 0,
			Follows:         i&2 != 0,
			Blocked:         i&4 != 0,
			Archived:        i&8 != 0,
			ViewerPostedDay: i&16 != 0,
		}
		want := f.IsOwner || (f.Follows && !f.Blocked && !f.Archived && f.ViewerPostedDay)
		if got := CanView(f); got != want {
			t.Errorf("%+v: CanView %v, want %v", f, got, want)
		}
		if CanReact(f) != (want && !f.IsOwner) {
			t.Errorf("%+v: CanReact", f)
		}
		if CanComment(f, true) {
			t.Errorf("%+v: comments off still allows a comment", f)
		}
		if CanComment(f, false) != want {
			t.Errorf("%+v: CanComment", f)
		}
	}
	// The named cases of the gate.
	cases := []struct {
		name string
		f    Facts
		want bool
	}{
		{"owner", Facts{IsOwner: true}, true},
		{"owner of an archived entry", Facts{IsOwner: true, Archived: true}, true},
		{"follower who posted the day", Facts{Follows: true, ViewerPostedDay: true}, true},
		{"follower who did not post (or skipped)", Facts{Follows: true}, false},
		{"archived", Facts{Follows: true, ViewerPostedDay: true, Archived: true}, false},
		{"blocked", Facts{Follows: true, ViewerPostedDay: true, Blocked: true}, false},
		{"stranger who posted", Facts{ViewerPostedDay: true}, false},
	}
	for _, c := range cases {
		if got := CanView(c.f); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if !CanDeleteComment(true, false) || !CanDeleteComment(false, true) || CanDeleteComment(false, false) {
		t.Error("CanDeleteComment: writer or owner only")
	}
}

func TestCleanUsername(t *testing.T) {
	good := map[string]string{"Maya": "maya", "@maya": "maya", "  @Maya_2  ": "maya_2", "abc": "abc"}
	for in, want := range good {
		if got, reason := CleanUsername(in); got != want || reason != "" {
			t.Errorf("%q: %q %q, want %q", in, got, reason, want)
		}
	}
	bad := map[string]string{
		"": "required", "  ": "required", "@": "required", "ab": "invalid", "has space": "invalid",
		"a-b": "invalid", "@@maya": "invalid", "maya!": "invalid", "abcdefghijklmnopqrstu": "invalid", "maïa": "invalid",
	}
	for in, want := range bad {
		if _, reason := CleanUsername(in); reason != want {
			t.Errorf("%q: reason %q, want %q", in, reason, want)
		}
	}
}

func TestSettingsPatch(t *testing.T) {
	off := false
	got := SettingsPatch{Comments: &off}.Apply(DefaultSettings)
	if got != (Settings{Reactions: true, Comments: false, Milestones: true}) {
		t.Fatalf("patch: %+v", got)
	}
}
