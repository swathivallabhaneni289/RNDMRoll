package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// A like shows and counts only while its maker is an accepted follower of
// the post's owner. After Alice unfollows Bob, or Bob removes her as a
// follower, she can no longer open his post to undo her like, so Bob no
// longer sees it either: not in the Likes list, not in the number beside the
// heart on his posts list, not in his bell, and it is not pushed. Nothing is
// deleted, so when she follows again her like is back. Cara's like, made
// while she follows Bob, stays throughout.
func TestBoardIntegration_ALikeShowsOnlyWhileItsMakerFollowsTheOwner(t *testing.T) {
	w := newModWorld(t)
	a := w.person("lf_alice", "Alice Ames")
	b := w.person("lf_bob", "Bob Brown")
	c := w.person("lf_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)
	path := "/v1/posts/" + bEntry
	w.ok(a, http.MethodPost, path+"/reaction", map[string]any{"reaction": "love"})
	w.ok(c, http.MethodPost, path+"/reaction", map[string]any{"reaction": "fire"})
	push := postgres.NewPushRepo(w.pool)

	// shown checks every place Bob meets Alice's like, and Cara's as the
	// control.
	shown := func(label string, want bool) {
		t.Helper()
		likers := []string{}
		for _, r := range w.ok(b, http.MethodGet, path, nil)["received"].([]any) {
			likers = append(likers, dig(t, r, "person", "id").(string))
		}
		wantN := 1
		if want {
			wantN = 2
		}
		if has(likers, a.id) != want || !has(likers, c.id) || len(likers) != wantN {
			t.Fatalf("%s: Bob's Likes list %v, want Alice there: %v, and Cara", label, likers, want)
		}
		if n := w.likeCount(b, bEntry); n != float64(wantN) {
			t.Fatalf("%s: Bob's posts list: like_count %v, want %d", label, n, wantN)
		}
		bell := w.bell(b)
		if has(bell, "like:"+bEntry+":"+a.id) != want || !has(bell, "like:"+bEntry+":"+c.id) {
			t.Fatalf("%s: Bob's bell %v, want Alice's like line: %v, and Cara's", label, bell, want)
		}
		for _, p := range []feedPerson{a, c} {
			owner, ok, err := push.LikeTarget(context.Background(), uuid.MustParse(bEntry), uuid.MustParse(p.id))
			if wantOK := want || p == c; err != nil || ok != wantOK || (ok && owner.String() != b.id) {
				t.Fatalf("%s: the push of %s's like: owner %v ok %v (err %v), want ok %v", label, p.name, owner, ok, err, wantOK)
			}
		}
	}
	shown("both follow Bob", true)

	w.ok(a, http.MethodPost, "/v1/follows/"+b.id+"/unfollow", map[string]any{})
	if w.seesPost(a, bEntry) {
		t.Fatal("Alice still opens Bob's post after the unfollow")
	}
	shown("after Alice unfollows", false)

	w.followAccepted(a, b)
	shown("after Alice follows again", true)

	w.ok(b, http.MethodPost, "/v1/followers/"+a.id+"/remove", map[string]any{})
	if w.seesPost(a, bEntry) {
		t.Fatal("Alice still opens Bob's post after Bob removes her")
	}
	shown("after Bob removes Alice as a follower", false)

	// A request Bob has not accepted is not a follow.
	if rel := w.follow(a, b); rel != "requested" {
		t.Fatalf("Alice's new request: %s", rel)
	}
	shown("while Alice's new request waits", false)
}
