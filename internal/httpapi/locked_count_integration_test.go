package httpapi

import (
	"net/http"
	"slices"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// The locked Board counts and names the same posts: the ones its day shows
// once the viewer posts. Lou follows six people who post the same day, and
// then, one at a time, a post is deleted by its owner, archived, removed by
// an admin, hidden under review, and reported by Lou. Each leaves the count
// and the names at once, and the names are always the people counted (at
// most board.MaxNames of them). Before, the count read the posted days, so
// a post that was gone or kept from Lou still counted. Once Lou posts, the
// day's cards are Lou's own and the one post the card counted and named.
func TestBoardIntegration_TheLockedCountIsThePostsTheDayShowsOnceUnlocked(t *testing.T) {
	w := newModWorld(t)
	l := w.person("lc_lou", "Lou Locked")
	const day = "2026-10-08"
	type owner struct {
		person feedPerson
		first  string
		entry  string
	}
	owners := []*owner{}
	add := func(username, name, first string) *owner {
		t.Helper()
		p := w.person(username, name)
		w.followAccepted(l, p)
		o := &owner{person: p, first: first, entry: w.post(p, day)}
		owners = append(owners, o)
		return o
	}
	sam := add("lc_sam", "Sam Shown", "Sam")
	dana := add("lc_dana", "Dana Deleted", "Dana")
	ari := add("lc_ari", "Ari Archived", "Ari")
	remi := add("lc_remi", "Remi Removed", "Remi")
	rue := add("lc_rue", "Rue Review", "Rue")
	rory := add("lc_rory", "Rory Reported", "Rory")

	// counted is who is still counted; each step takes one out.
	counted := slices.Clone(owners)
	check := func(label string, gone *owner) {
		t.Helper()
		if gone != nil {
			counted = slices.DeleteFunc(counted, func(o *owner) bool { return o == gone })
		}
		names, n := w.locked(l)
		firsts := []string{}
		for _, o := range counted {
			firsts = append(firsts, o.first)
		}
		if int(n) != len(counted) || len(names) != min(len(counted), board.MaxNames) {
			t.Fatalf("%s: Lou's locked card says %v posted, names %v; want %d posted, of %v", label, n, names, len(counted), firsts)
		}
		for _, name := range names {
			if !slices.Contains(firsts, name) {
				t.Fatalf("%s: the name %s is not one of the people counted, %v", label, name, firsts)
			}
		}
	}
	check("all six posted", nil)

	w.ok(dana.person, http.MethodPost, "/v1/entries/"+dana.entry+"/delete", map[string]any{})
	check("a post its owner deleted", dana)

	w.ok(ari.person, http.MethodPost, "/v1/entries/"+ari.entry+"/archive", nil)
	check("an archived post", ari)

	w.sql(`update entries set removed_at = now() where id = $1`, remi.entry)
	check("a post an admin removed", remi)

	// Three people who see Rue's post report it: under review.
	for i, username := range []string{"lc_rhea", "lc_remy", "lc_rose"} {
		r := w.person(username, "Reporter "+username)
		w.followAccepted(r, rue.person)
		w.post(r, day)
		w.report(r, "posts", rue.entry)
		if i < 2 {
			check("a post with fewer than three reports", nil)
		}
	}
	check("a post under review", rue)

	// Lou cannot open a post of a day Lou has not posted, so Lou's report is
	// set straight in the database: one report is not a review, but it keeps
	// the post from its reporter for good.
	w.sql(`insert into post_reports (reporter_id, entry_id, reason, created_at, owner_id, title)
		select $1, e.id, 'spam', now(), e.user_id, e.title from entries e where e.id = $2`, l.id, rory.entry)
	check("a post Lou reported", rory)

	// Lou posts: the day shows Lou's own post and Sam's, the one counted.
	own := w.post(l, day)
	got := spinpostsDayCards(t, w.board(l, ""))[day]
	want := []string{own, sam.entry}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("Lou's day once posted: %v, want Lou's %s and Sam's %s", got, own, sam.entry)
	}
}
