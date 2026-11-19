package notifications

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The admin's decisions about the viewer's own posts and comments, and the
// hides by reports (the decision log, migration 0020), in the bell: system
// items with no actor that no switch turns off, one per row, by the rules of
// their kind.

// decisionAt is a decision of item and action on the post entry (nil for
// one deleted since), at, with the snapshot text.
func decisionAt(item, action string, at time.Time, text string, entry *uuid.UUID) Decision {
	return Decision{ID: uuid.New(), Item: item, Action: action, At: at, Snapshot: text, EntryID: entry}
}

// itemByID is the list's item with id, or nil.
func itemByID(list List, id string) *Item {
	for i := range list.Items {
		if list.Items[i].ID == id {
			return &list.Items[i]
		}
	}
	return nil
}

// NOT-01: the six kinds show with both switches off, with no actor, each
// row once as kind:<log row>, unread, and they light the dot. A decision
// about the account, or one the bell does not know, makes no item.
func TestModeration_TheAdminsDecisionsShowWhateverTheSwitches(t *testing.T) {
	bell, notes, people, now, _ := socialBell(t)
	people.settings.Comments, people.settings.Milestones = false, false
	entry := uuid.New()
	at := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour).UTC() }
	notes.decisions = []Decision{
		decisionAt("post", "removed", at(1), "Dune", &entry),
		decisionAt("comment", "removed", at(2), "Rude words", &entry),
		decisionAt("post", "restored", at(3), "Dune", &entry),
		decisionAt("comment", "shown_again", at(4), "Rude words", &entry),
		decisionAt("post", "shown_again", at(5), "Dune", &entry),
		decisionAt("comment", "restored", at(6), "Rude words", &entry),
		decisionAt("post", "hidden", at(7), "Dune", &entry),
		decisionAt("comment", "hidden", at(8), "Rude words", &entry),
		decisionAt("account", "disabled", at(9), "", nil),
		decisionAt("account", "enabled", at(10), "", nil),
		decisionAt("post", "edited", at(11), "Dune", &entry),
	}

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"post_removed", "comment_removed", "post_back", "comment_back", "post_back", "comment_back",
		"post_hidden", "comment_hidden"}
	got := []string{}
	for i, it := range list.Items {
		got = append(got, it.Kind)
		d := notes.decisions[i]
		if it.ID != it.Kind+":"+d.ID.String() || !it.At.Equal(d.At) || it.Actor != nil || !it.Unread ||
			it.AtLabel == "" || it.Section != SectionToday || it.CategoryName != nil || it.RequestID != nil {
			t.Fatalf("item %d: %+v, want %s:%s at %v, no actor, unread", i, it, it.Kind, d.ID, d.At)
		}
	}
	if !slices.Equal(got, want) || !list.Unread {
		t.Fatalf("the kinds: %v, want %v, and the list unread", got, want)
	}
	if dot, err := bell.Unread(context.Background(), uuid.New()); err != nil || !dot {
		t.Fatalf("the dot: %v %v, want it lit", dot, err)
	}
	reads := []string{}
	for _, r := range notes.reads {
		reads = append(reads, r.name)
	}
	if !slices.Contains(reads, "decisions") || slices.Contains(reads, "comments") || slices.Contains(reads, "bonuses") {
		t.Fatalf("the reads with both switches off: %v, want the decisions and neither comments nor bonuses", reads)
	}

	// Seen after the newest, none is unread and the dot is off.
	notes.seen, notes.haveSeen = now.UTC(), true
	if list, err = bell.List(context.Background(), uuid.New()); err != nil || list.Unread {
		t.Fatalf("after seen: unread %v (err %v)", list.Unread, err)
	}
	for _, it := range list.Items {
		if it.Unread {
			t.Fatalf("after seen %s is unread", it.ID)
		}
	}
}

// A removal carries its title or text, its ref, its full id to ask for a
// second look with and that second look's state, and no way back to what
// was removed: no post and no photo, even when the read handed it both while
// the post is still kept for Bring it back. A Keep it removed (developer,
// 2026-11-19) is its own kind for a post and a comment, with the text and
// nothing else. A post or comment that is back carries none of them.
func TestModeration_ARemovalCarriesItsRefAndItsSecondLookAndNoWayBack(t *testing.T) {
	bell, notes, _, now, _ := socialBell(t)
	entry := uuid.New()
	photo := "entries/" + uuid.NewString() + "/0123456789abcdef.jpg"
	post := decisionAt("post", "removed", now.Add(-time.Hour).UTC(), "Dune", &entry)
	post.SecondLook = SecondLookAsked
	comment := decisionAt("comment", "removed", now.Add(-2*time.Hour).UTC(), "Rude words", &entry)
	back := decisionAt("post", "restored", now.Add(-3*time.Hour).UTC(), "Dune", &entry)
	keptPost := decisionAt("post", "kept", now.Add(-4*time.Hour).UTC(), "Dune", &entry)
	keptComment := decisionAt("comment", "kept", now.Add(-5*time.Hour).UTC(), "Rude words", &entry)
	for _, d := range []*Decision{&post, &comment, &back, &keptPost, &keptComment} {
		d.Visible, d.PhotoKey = true, &photo
	}
	notes.decisions = []Decision{post, comment, back, keptPost, keptComment}

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		d    Decision
		kind string
		text string
		look string
	}{{post, KindPostRemoved, "Dune", SecondLookAsked}, {comment, KindCommentRemoved, "Rude words", SecondLookNone}} {
		it := itemByID(list, c.kind+":"+c.d.ID.String())
		if it == nil {
			t.Fatalf("no %s item: %+v", c.kind, list.Items)
		}
		if it.Text == nil || *it.Text != c.text || it.EntryID != nil || it.PhotoKey != nil ||
			it.Ref != c.d.ID.String()[:8] || it.Removal == nil || *it.Removal != c.d.ID || it.SecondLook != c.look {
			t.Fatalf("the %s item: %+v, want its text, the ref %s, its id, second look %q and no post or photo", c.kind, it, c.d.ID.String()[:8], c.look)
		}
	}
	for _, c := range []struct {
		d    Decision
		kind string
		text string
	}{{keptPost, KindPostKept, "Dune"}, {keptComment, KindCommentKept, "Rude words"}} {
		it := itemByID(list, c.kind+":"+c.d.ID.String())
		if it == nil || it.Text == nil || *it.Text != c.text || it.EntryID != nil || it.PhotoKey != nil || it.Ref != "" ||
			it.Removal != nil || it.SecondLook != "" || it.Reason != "" {
			t.Fatalf("the %s item: %+v, want its text and nothing else", c.kind, it)
		}
	}
	if it := itemByID(list, "post_back:"+back.ID.String()); it == nil || it.Ref != "" || it.Removal != nil || it.SecondLook != "" {
		t.Fatalf("the post_back item: %+v, want no ref, no id and no second look", it)
	}
}

// A removal carries the reason the admin gave, as the log keeps it (the
// report reason's key: the app words it), and one with none carries none
// (developer, 2026-11-18). Nothing else carries a reason, even from a row
// that has one.
func TestModeration_OnlyARemovalCarriesTheReason(t *testing.T) {
	bell, notes, _, now, _ := socialBell(t)
	entry := uuid.New()
	at := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour).UTC() }
	post := decisionAt("post", "removed", at(1), "Dune", &entry)
	post.Reason = "hate"
	comment := decisionAt("comment", "removed", at(2), "Rude words", &entry)
	comment.Reason = "harassment"
	none := decisionAt("post", "removed", at(3), "Dune", &entry)
	others := []Decision{
		decisionAt("post", "restored", at(4), "Dune", &entry),
		decisionAt("comment", "restored", at(5), "Rude words", &entry),
		decisionAt("post", "shown_again", at(6), "Dune", &entry),
		decisionAt("comment", "shown_again", at(7), "Rude words", &entry),
		decisionAt("post", "hidden", at(8), "Dune", &entry),
		decisionAt("comment", "hidden", at(9), "Rude words", &entry),
	}
	for i := range others {
		others[i].Reason = "spam"
	}
	notes.decisions = append([]Decision{post, comment, none}, others...)

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != len(notes.decisions) {
		t.Fatalf("%d items, want one for each of the %d rows: %+v", len(list.Items), len(notes.decisions), list.Items)
	}
	want := map[string]string{
		KindPostRemoved + ":" + post.ID.String():       "hate",
		KindCommentRemoved + ":" + comment.ID.String(): "harassment",
	}
	for _, it := range list.Items {
		if it.Reason != want[it.ID] {
			t.Fatalf("%s: the reason %q, want %q", it.ID, it.Reason, want[it.ID])
		}
	}
}

// A post that is back or hidden opens the post, and shows its first photo
// only while the viewer can see it; once the post is deleted it opens
// nothing. A comment that is back or hidden opens its post only while the
// viewer can see that post, and never shows a photo.
func TestModeration_BackOpensOnlyWhatTheViewerCanSee(t *testing.T) {
	bell, notes, _, now, _ := socialBell(t)
	entry, other := uuid.New(), uuid.New()
	photo := "entries/" + uuid.NewString() + "/0123456789abcdef.jpg"
	at := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour).UTC() }
	seen := decisionAt("post", "restored", at(1), "Dune", &entry)
	seen.Visible, seen.PhotoKey = true, &photo
	unseen := decisionAt("post", "shown_again", at(2), "Dune", &entry)
	unseen.PhotoKey = &photo
	deleted := decisionAt("post", "restored", at(3), "Dune", nil)
	onSeen := decisionAt("comment", "restored", at(4), "Nice", &other)
	onSeen.Visible, onSeen.PhotoKey = true, &photo
	onUnseen := decisionAt("comment", "shown_again", at(5), "Nice", &other)
	hidSeen := decisionAt("post", "hidden", at(6), "Dune", &entry)
	hidSeen.Visible, hidSeen.PhotoKey = true, &photo
	hidUnseen := decisionAt("post", "hidden", at(7), "Dune", &entry)
	hidUnseen.PhotoKey = &photo
	hidOnSeen := decisionAt("comment", "hidden", at(8), "Nice", &other)
	hidOnSeen.Visible, hidOnSeen.PhotoKey = true, &photo
	hidOnUnseen := decisionAt("comment", "hidden", at(9), "Nice", &other)
	notes.decisions = []Decision{seen, unseen, deleted, onSeen, onUnseen, hidSeen, hidUnseen, hidOnSeen, hidOnUnseen}

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		kind  string
		d     Decision
		entry *uuid.UUID
		photo bool
	}{
		{"a post the viewer can see", KindPostBack, seen, &entry, true},
		{"a post the viewer cannot see", KindPostBack, unseen, &entry, false},
		{"a post deleted since", KindPostBack, deleted, nil, false},
		{"a comment on a post the viewer can see", KindCommentBack, onSeen, &other, false},
		{"a comment on a post the viewer cannot see", KindCommentBack, onUnseen, nil, false},
		{"a hidden post the viewer can see", KindPostHidden, hidSeen, &entry, true},
		{"a hidden post the viewer cannot see", KindPostHidden, hidUnseen, &entry, false},
		{"a hidden comment on a post the viewer can see", KindCommentHidden, hidOnSeen, &other, false},
		{"a hidden comment on a post the viewer cannot see", KindCommentHidden, hidOnUnseen, nil, false},
	} {
		kind := c.kind
		it := itemByID(list, kind+":"+c.d.ID.String())
		if it == nil {
			t.Fatalf("%s: no %s item in %+v", c.name, kind, list.Items)
		}
		if (it.EntryID == nil) != (c.entry == nil) || (it.EntryID != nil && *it.EntryID != *c.entry) {
			t.Fatalf("%s: entry %v, want %v", c.name, it.EntryID, c.entry)
		}
		if (it.PhotoKey != nil) != c.photo || (c.photo && *it.PhotoKey != photo) {
			t.Fatalf("%s: photo %v, want it %v", c.name, it.PhotoKey, c.photo)
		}
		if it.Text == nil || *it.Text != c.d.Snapshot || it.Ref != "" || it.Removal != nil || it.SecondLook != "" {
			t.Fatalf("%s: %+v, want the text and no ref, removal or second look", c.name, it)
		}
	}
}

// The text is the title or the comment text as it was decided, whole, with
// its line breaks as spaces; a comment that was only a photo has none.
func TestModeration_TheTextIsTheSnapshotOnOneLine(t *testing.T) {
	bell, notes, _, now, _ := socialBell(t)
	// Longer than the 60 characters other bell texts are cut to.
	long := "One\ntwo three four five six seven eight nine ten eleven twelve thirteen fourteen"
	lines := decisionAt("comment", "removed", now.Add(-time.Hour).UTC(), long, nil)
	photoOnly := decisionAt("comment", "removed", now.Add(-2*time.Hour).UTC(), "", nil)
	notes.decisions = []Decision{lines, photoOnly}

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	want := "One two three four five six seven eight nine ten eleven twelve thirteen fourteen"
	if it := itemByID(list, "comment_removed:"+lines.ID.String()); it == nil || it.Text == nil || *it.Text != want {
		t.Fatalf("the text: %+v, want %q whole", it, want)
	}
	if it := itemByID(list, "comment_removed:"+photoOnly.ID.String()); it == nil || it.Text != nil {
		t.Fatalf("a photo-only comment: %+v, want no text", it)
	}
}

// Decisions take their place among the other items by their own time, each
// one an item of its own (a post removed, brought back and removed again
// makes three), and fall under the same 50 newest as everything else, but
// for a removal its owner can still ask a second look about: that one stays
// at the end, as its line is the only way to ask.
func TestModeration_EachDecisionIsAnItemInItsPlace(t *testing.T) {
	bell, notes, _, now, actor := socialBell(t)
	entry := uuid.New()
	category, text := "Movie", "hi"
	at := func(m int) time.Time { return now.Add(-time.Duration(m) * time.Minute).UTC() }
	removed := decisionAt("post", "removed", at(30), "Dune", &entry)
	removed.SecondLook = SecondLookRestored
	back := decisionAt("post", "restored", at(20), "Dune", &entry)
	again := decisionAt("post", "removed", at(10), "Dune", &entry)
	notes.decisions = []Decision{again, back, removed}
	notes.comments = []Raw{
		{Kind: KindComment, ID: "comment:a", At: at(25), ActorID: actor, EntryID: &entry, CategoryName: &category, Text: &text},
		{Kind: KindComment, ID: "comment:b", At: at(5), ActorID: actor, EntryID: &entry, CategoryName: &category, Text: &text},
	}
	notes.seen, notes.haveSeen = at(15), true

	list, err := bell.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"comment:b", "post_removed:" + again.ID.String(), "post_back:" + back.ID.String(),
		"comment:a", "post_removed:" + removed.ID.String()}
	got := []string{}
	unread := []bool{}
	for _, it := range list.Items {
		got = append(got, it.ID)
		unread = append(unread, it.Unread)
	}
	if !slices.Equal(got, want) || !slices.Equal(unread, []bool{true, true, false, false, false}) {
		t.Fatalf("the list: %v unread %v, want %v with the two after the seen time unread", got, unread, want)
	}

	// 60 comments newer than every decision: the decisions fall off the
	// end like any other item, all but the newest removal, which can still
	// be asked about.
	notes.comments = nil
	for m := 1; m <= 60; m++ {
		notes.comments = append(notes.comments, Raw{Kind: KindComment, ID: "comment:" + time.Duration(m).String(), At: now.Add(-time.Duration(m) * time.Second).UTC(),
			ActorID: actor, EntryID: &entry, CategoryName: &category, Text: &text})
	}
	if list, err = bell.List(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != MaxItems+1 {
		t.Fatalf("%d items, want %d", len(list.Items), MaxItems+1)
	}
	for _, it := range list.Items[:MaxItems] {
		if it.Kind != KindComment {
			t.Fatalf("a decision older than 50 newer items is among them: %+v", it)
		}
	}
	if last := list.Items[MaxItems]; last.ID != "post_removed:"+again.ID.String() || last.Removal == nil || *last.Removal != again.ID {
		t.Fatalf("past the 50, want only the removal that can still be asked about: %+v", last)
	}
	for _, look := range []string{SecondLookAsked, SecondLookKept, SecondLookRestored, SecondLookClosed} {
		notes.decisions[0].SecondLook = look
		if list, err = bell.List(context.Background(), uuid.New()); err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != MaxItems {
			t.Fatalf("a removal with its second look %q stays past the 50: %d items", look, len(list.Items))
		}
	}
}
