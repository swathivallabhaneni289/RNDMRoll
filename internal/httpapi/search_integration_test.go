package httpapi

import (
	"net/http"
	"testing"
)

// searchNames runs GET /v1/people/search?q= as p and returns the usernames
// in the order the server sent them.
func (w *feedWorld) searchNames(p feedPerson, q string) []string {
	w.t.Helper()
	got := w.ok(p, http.MethodGet, "/v1/people/search?q="+q, nil)
	list, _ := got["people"].([]any)
	names := []string{}
	for _, item := range list {
		hit := item.(map[string]any)
		person := hit["person"].(map[string]any)
		names = append(names, person["username"].(string))
	}
	return names
}

func TestFollowIntegration_SearchByStartOfUsernameOrName(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("se_alice", "Alice Ames")
	b := w.person("se_bob", "Bob Brown")
	c := w.person("se_bobby", "Roberta Stone")
	d := w.person("se_dana", "Dana Bobson")

	// The start of a username, an exact one first; the caller is left out.
	if got := w.searchNames(a, "se_bob"); len(got) != 2 || got[0] != "se_bob" || got[1] != "se_bobby" {
		t.Fatalf("username prefix: %v", got)
	}
	if got := w.searchNames(b, "se_bob"); len(got) != 1 || got[0] != "se_bobby" {
		t.Fatalf("yourself is left out: %v", got)
	}
	// A leading @ and capitals are fine; a word of a name counts too.
	if got := w.searchNames(a, "@SE_DA"); len(got) != 1 || got[0] != "se_dana" {
		t.Fatalf("@ and case: %v", got)
	}
	if got := w.searchNames(a, "stone"); len(got) != 1 || got[0] != "se_bobby" {
		t.Fatalf("second word of a name: %v", got)
	}
	// LIKE characters are matched literally.
	if got := w.searchNames(a, "%"); len(got) != 0 {
		t.Fatalf("percent is literal: %v", got)
	}
	if got := w.searchNames(a, ""); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}

	// The relation comes with each hit.
	w.follow(a, b)
	got := w.ok(a, http.MethodGet, "/v1/people/search?q=se_bob", nil)
	first := got["people"].([]any)[0].(map[string]any)
	if first["relation"] != "requested" || first["follows_you"] != false {
		t.Fatalf("relation: %v", first)
	}

	// A block either way hides both people from each other.
	w.ok(c, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	if got := w.searchNames(a, "se_bob"); len(got) != 1 || got[0] != "se_bob" {
		t.Fatalf("blocked by c: %v", got)
	}
	if got := w.searchNames(c, "se_al"); len(got) != 0 {
		t.Fatalf("c blocked a: %v", got)
	}
	_ = d
}
