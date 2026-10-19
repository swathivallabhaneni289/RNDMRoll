package httpapi

import (
	"net/http"
	"testing"
)

func suggested(t *testing.T, w *feedWorld, p feedPerson) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, item := range w.ok(p, http.MethodGet, "/v1/people/suggestions", nil)["people"].([]any) {
		out = append(out, item.(map[string]any))
	}
	return out
}

func TestPersonIntegration_FindYourFriendsSuggestions(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("sg_alice", "Alice Ames")
	b := w.person("sg_bob", "Bob Brown")
	c := w.person("sg_cara", "Cara Cole")
	d := w.person("sg_dan", "Dan Dorn")
	e := w.person("sg_eve", "Eve East")

	// Nobody yet: nothing to suggest.
	if got := suggested(t, w, a); len(got) != 0 {
		t.Fatalf("new person: %v", got)
	}
	// A follows B; B follows C and D; E follows A.
	w.followAccepted(a, b)
	w.followAccepted(b, c)
	w.followAccepted(b, d)
	w.followAccepted(e, a)
	got := suggested(t, w, a)
	if len(got) != 3 || dig(t, got[0], "person", "username") != "sg_eve" || got[0]["follows_you"] != true {
		t.Fatalf("suggestions: %v", got)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[dig(t, s, "person", "username").(string)] = true
	}
	if !names["sg_cara"] || !names["sg_dan"] || names["sg_bob"] || names["sg_alice"] {
		t.Fatalf("names: %v", names)
	}
	// Asking someone takes them off; a block takes them off.
	w.follow(a, c)
	w.ok(d, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	if got := suggested(t, w, a); len(got) != 1 || dig(t, got[0], "person", "username") != "sg_eve" {
		t.Fatalf("after a request and a block: %v", got)
	}
}
