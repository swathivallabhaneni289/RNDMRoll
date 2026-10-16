package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// reply writes text on entry as p, a reply to parent when parent is set, and
// returns the stored comment. Every call moves the shared clock one minute
// first, so no two comments share an instant.
func (w *feedWorld) reply(p feedPerson, entry, text, parent string) map[string]any {
	w.t.Helper()
	w.clk.Advance(time.Minute)
	body := map[string]any{"text": text}
	if parent != "" {
		body["parent_id"] = parent
	}
	return w.call(p, http.MethodPost, "/v1/posts/"+entry+"/comments", body, http.StatusCreated)["comment"].(map[string]any)
}

// commentIDs lists a post's comments in page order with each parent_id
// ("" for a top-level comment, which must send the key as null).
func commentIDs(t *testing.T, post map[string]any) (ids, parents []string) {
	t.Helper()
	for _, v := range post["comments"].([]any) {
		m := v.(map[string]any)
		parent, present := m["parent_id"]
		if !present {
			t.Fatalf("comment without the parent_id key: %v", m)
		}
		ids = append(ids, m["id"].(string))
		if parent == nil {
			parents = append(parents, "")
		} else {
			parents = append(parents, parent.(string))
		}
	}
	return ids, parents
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// boardCount is the comment_count on the Board card of entry as p sees it.
func (w *feedWorld) boardCount(p feedPerson, entry string) float64 {
	w.t.Helper()
	for _, d := range w.board(p, "")["days"].([]any) {
		for _, c := range d.(map[string]any)["cards"].([]any) {
			if card := c.(map[string]any); card["entry_id"] == entry {
				return card["comment_count"].(float64)
			}
		}
	}
	w.t.Fatalf("no card %s on %s's Board", entry, p.name)
	return 0
}

func TestReplyIntegration_RoundTripOrderAndOneLevel(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("rp_alice", "Alice Ames")
	b := w.person("rp_bob", "Bob Brown")
	c := w.person("rp_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	aEntry := w.post(a, day)
	w.post(c, day)
	path := "/v1/posts/" + bEntry

	top1 := w.reply(a, bEntry, "First", "")
	if top1["parent_id"] != nil {
		t.Fatalf("a top-level comment has a null parent_id: %v", top1)
	}
	top2 := w.reply(c, bEntry, "Second", "")
	r1 := w.reply(b, bEntry, "  @rp_alice thanks  ", top1["id"].(string))
	if r1["parent_id"] != top1["id"] || r1["text"] != "@rp_alice thanks" || r1["can_delete"] != true || dig(t, r1, "author", "id") != b.id {
		t.Fatalf("reply: %v", r1)
	}
	// A reply to a reply is stored under the top-level comment.
	r2 := w.reply(c, bEntry, "@rp_bob same", r1["id"].(string))
	if r2["parent_id"] != top1["id"] {
		t.Fatalf("a reply to a reply flattens to the top-level comment: %v", r2)
	}

	// One flat list: each top-level comment, oldest first, directly followed by its replies.
	for _, viewer := range []feedPerson{a, b, c} {
		post := w.ok(viewer, http.MethodGet, path, nil)
		ids, parents := commentIDs(t, post)
		wantIDs := []string{top1["id"].(string), r1["id"].(string), r2["id"].(string), top2["id"].(string)}
		wantParents := []string{"", top1["id"].(string), top1["id"].(string), ""}
		if !sameList(ids, wantIDs) || !sameList(parents, wantParents) {
			t.Fatalf("%s's page order: %v %v, want %v %v", viewer.name, ids, parents, wantIDs, wantParents)
		}
		if dig(t, post, "card", "comment_count") != float64(4) {
			t.Fatalf("replies count as comments: %v", post["card"])
		}
		// b follows nobody, so b's Board lists no days.
		if viewer.id != b.id {
			if n := w.boardCount(viewer, bEntry); n != 4 {
				t.Fatalf("%s's Board count: %v", viewer.name, n)
			}
		}
	}

	// The parent must be a visible comment on the same post.
	aComment := w.reply(a, aEntry, "On my own", "")
	if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Wrong post", "parent_id": aComment["id"]}, http.StatusNotFound); got["error"] != "not_found" {
		t.Fatalf("parent on another post: %v", got)
	}
	w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Nobody", "parent_id": "00000000-0000-0000-0000-000000000001"}, http.StatusNotFound)
	for _, bad := range []any{"nope", "", 7} {
		if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Bad", "parent_id": bad}, http.StatusBadRequest); got["field"] != "parent_id" {
			t.Fatalf("parent_id %v: %v", bad, got)
		}
	}
	// null is a top-level comment; the text rules are the same for a reply.
	if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "Top", "parent_id": nil}, http.StatusCreated); dig(t, got, "comment", "parent_id") != nil {
		t.Fatalf("null parent: %v", got)
	}
	if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": strings.Repeat("a", 141), "parent_id": top1["id"]}, http.StatusBadRequest); got["field"] != "text" {
		t.Fatalf("a long reply: %v", got)
	}
	if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "  ", "parent_id": top1["id"]}, http.StatusBadRequest); got["field"] != "text" {
		t.Fatalf("an empty reply: %v", got)
	}
}

func TestReplyIntegration_CommentsOffDeleteAndBlocks(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("rq_alice", "Alice Ames")
	b := w.person("rq_bob", "Bob Brown")
	c := w.person("rq_cara", "Cara Cole")
	d := w.person("rq_dan", "Dan Dove")
	for _, p := range []feedPerson{a, c, d} {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	for _, p := range []feedPerson{a, c, d} {
		w.post(p, day)
	}
	path := "/v1/posts/" + bEntry

	top1 := w.reply(a, bEntry, "First", "")
	top2 := w.reply(c, bEntry, "Second", "")
	r1 := w.reply(d, bEntry, "Reply one", top1["id"].(string))
	r2 := w.reply(c, bEntry, "Reply two", top1["id"].(string))
	r3 := w.reply(a, bEntry, "Reply three", top2["id"].(string))

	// comments_off refuses a reply with 409, before the parent is looked at.
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"comments_off": true})
	for _, parent := range []any{top1["id"], "00000000-0000-0000-0000-000000000001"} {
		if got := w.call(c, http.MethodPost, path+"/comments", map[string]any{"text": "More", "parent_id": parent}, http.StatusConflict); got["error"] != "comments_off" {
			t.Fatalf("comments off refuses a reply: %v", got)
		}
	}
	w.ok(b, http.MethodPatch, "/v1/entries/"+bEntry, map[string]any{"comments_off": false})

	// A block hides a reply like a comment, and hides the replies under a hidden comment.
	w.ok(c, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": d.id})
	ids, _ := commentIDs(t, w.ok(c, http.MethodGet, path, nil))
	if want := []string{top1["id"].(string), r2["id"].(string), top2["id"].(string), r3["id"].(string)}; !sameList(ids, want) {
		t.Fatalf("c no longer sees d's reply: %v, want %v", ids, want)
	}
	w.call(c, http.MethodDelete, "/v1/me/blocks/"+d.id, nil, http.StatusNoContent)
	w.ok(d, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	post := w.ok(d, http.MethodGet, path, nil)
	ids, _ = commentIDs(t, post)
	if want := []string{top2["id"].(string)}; !sameList(ids, want) || dig(t, post, "card", "comment_count") != float64(1) {
		t.Fatalf("d sees neither a's comments nor the replies under a's comment: %v", post)
	}
	if n := w.boardCount(d, bEntry); n != 1 {
		t.Fatalf("d's Board count: %v", n)
	}
	w.call(d, http.MethodPost, path+"/comments", map[string]any{"text": "Hidden", "parent_id": top1["id"]}, http.StatusNotFound)
	w.call(d, http.MethodPost, path+"/comments", map[string]any{"text": "Hidden", "parent_id": r2["id"]}, http.StatusNotFound)
	w.call(d, http.MethodDelete, "/v1/me/blocks/"+a.id, nil, http.StatusNoContent)

	// Deleting a reply removes only that reply; deleting a top-level comment removes its replies.
	w.call(a, http.MethodPost, "/v1/comments/"+r2["id"].(string)+"/delete", map[string]any{}, http.StatusNotFound)
	w.ok(c, http.MethodPost, "/v1/comments/"+r2["id"].(string)+"/delete", map[string]any{})
	ids, _ = commentIDs(t, w.ok(b, http.MethodGet, path, nil))
	if want := []string{top1["id"].(string), r1["id"].(string), top2["id"].(string), r3["id"].(string)}; !sameList(ids, want) {
		t.Fatalf("after deleting a reply: %v, want %v", ids, want)
	}
	w.ok(a, http.MethodPost, "/v1/comments/"+top1["id"].(string)+"/delete", map[string]any{})
	post = w.ok(b, http.MethodGet, path, nil)
	ids, _ = commentIDs(t, post)
	if want := []string{top2["id"].(string), r3["id"].(string)}; !sameList(ids, want) || dig(t, post, "card", "comment_count") != float64(2) {
		t.Fatalf("deleting a comment takes its replies: %v", post)
	}
	w.call(d, http.MethodPost, "/v1/comments/"+r1["id"].(string)+"/delete", map[string]any{}, http.StatusNotFound)
	// The owner deletes a top-level comment with its replies too.
	w.ok(b, http.MethodPost, "/v1/comments/"+top2["id"].(string)+"/delete", map[string]any{})
	if got := w.ok(a, http.MethodGet, path, nil); len(got["comments"].([]any)) != 0 || dig(t, got, "card", "comment_count") != float64(0) {
		t.Fatalf("all gone: %v", got)
	}
}

// itemByID finds a notification by its id, or nil.
func itemByID(list map[string]any, id string) map[string]any {
	for _, v := range list["items"].([]any) {
		if m := v.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func TestReplyIntegration_Notifications(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("rn_alice", "Alice Ames")
	b := w.person("rn_bob", "Bob Brown")
	c := w.person("rn_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)

	aTop := w.reply(a, bEntry, "Great pick", "")
	long := "@rn_alice " + strings.Repeat("x", 70)
	cReply := w.reply(c, bEntry, long, aTop["id"].(string))

	// The parent's writer gets a reply item.
	item := itemByID(w.notes(a), "reply:"+cReply["id"].(string))
	if item == nil || item["kind"] != "reply" || item["entry_id"] != bEntry || item["category_name"] == nil ||
		dig(t, item, "actor", "id") != c.id || item["text"] != long[:60]+"..." || item["unread"] != true {
		t.Fatalf("reply item: %v", w.notes(a))
	}
	// The owner still gets a comment item for someone else's reply.
	if cm := itemByID(w.notes(b), "comment:"+cReply["id"].(string)); cm == nil || cm["kind"] != "comment" {
		t.Fatalf("owner's comment item for a reply: %v", w.notes(b))
	}
	if itemByID(w.notes(b), "reply:"+cReply["id"].(string)) != nil {
		t.Fatal("the owner did not write the parent")
	}

	// A reply to the owner's own comment: the reply item only, no duplicate comment item.
	bTop := w.reply(b, bEntry, "Thanks all", "")
	cToB := w.reply(c, bEntry, "@rn_bob sure", bTop["id"].(string))
	bList := w.notes(b)
	if r := itemByID(bList, "reply:"+cToB["id"].(string)); r == nil || r["text"] != "@rn_bob sure" || dig(t, r, "actor", "id") != c.id {
		t.Fatalf("owner's reply item: %v", bList)
	}
	if itemByID(bList, "comment:"+cToB["id"].(string)) != nil {
		t.Fatalf("no comment item next to the reply item: %v", bList)
	}

	// Your own reply to your own comment is no item for you; the owner sees it as a comment.
	aSelf := w.reply(a, bEntry, "Also this", aTop["id"].(string))
	if itemByID(w.notes(a), "reply:"+aSelf["id"].(string)) != nil {
		t.Fatal("your own reply is not an item for you")
	}
	if itemByID(w.notes(b), "comment:"+aSelf["id"].(string)) == nil {
		t.Fatal("the owner's comment item for a's reply")
	}
	// The owner replying under someone's comment: a reply item for that writer, nothing for the owner.
	bToA := w.reply(b, bEntry, "@rn_alice yes", aTop["id"].(string))
	if itemByID(w.notes(a), "reply:"+bToA["id"].(string)) == nil {
		t.Fatal("a gets the owner's reply")
	}
	if l := w.notes(b); itemByID(l, "comment:"+bToA["id"].(string)) != nil || itemByID(l, "reply:"+bToA["id"].(string)) != nil {
		t.Fatalf("the owner's own reply: %v", l)
	}

	// The comments setting filters reply items too, and the dot with them.
	w.ok(a, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	w.ok(a, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": false})
	w.reply(c, bEntry, "One more", aTop["id"].(string))
	if k := kinds(t, w.notes(a)); has(k, "reply") || has(k, "comment") {
		t.Fatalf("comments off hides replies: %v", k)
	}
	if got := w.ok(a, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != false {
		t.Fatalf("a filtered reply does not light the dot: %v", got)
	}
	w.ok(a, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"comments": true})
	if got := w.ok(a, http.MethodGet, "/v1/notifications/summary", nil); got["unread"] != true {
		t.Fatalf("comments on again: %v", got)
	}
	if k := kinds(t, w.notes(a)); !has(k, "reply") {
		t.Fatalf("reply items back: %v", k)
	}

	// A block with the writer hides the item; a post you can no longer see hides it too.
	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": c.id})
	if itemByID(w.notes(a), "reply:"+cReply["id"].(string)) != nil {
		t.Fatal("a blocked writer's reply item is gone")
	}
	w.call(a, http.MethodDelete, "/v1/me/blocks/"+c.id, nil, http.StatusNoContent)
	w.ok(a, http.MethodPost, "/v1/follows/"+b.id+"/unfollow", map[string]any{})
	if k := kinds(t, w.notes(a)); has(k, "reply") {
		t.Fatalf("replies on a post a can no longer see: %v", k)
	}
}
