package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// The app's side of moderation was built from the spec, not from this
// server. This test pins the answers the app reads for each new key and
// kind, as the server really sends them: one bell item of each decision
// kind, Today once the day's post was removed, the owner's Board, post page,
// entry and Today while their post is under review, and a writer's own
// comment under review. Ids, instants and signed photo links change from run
// to run, so they are swapped for fixed placeholders, and the result must
// equal moderationContractFile. scripts/check-spinposts-extra.ts runs that
// file through the app's own readers, so a key or kind the app would drop
// fails there. UPDATE_GOLDEN=1 writes the file again: read the diff, and run
// that script, before committing it.

const moderationContractFile = "testdata/moderation_contract.json"

var (
	contractUUID    = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	contractInstant = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
)

// contractPlaceholders swaps ids, instants and signed photo links for fixed
// placeholders. An id keeps the shape of a UUID and a link the shape of a
// signed link, because the app checks those shapes (an entry id must be a
// UUID before a page opens it). The same id or link always gets the same
// placeholder, so what points at what survives: post_back's entry_id is
// still the post's own. A placeholder id's first 8 characters are unique,
// so a removal's ref stays the start of its log row's id.
type contractPlaceholders struct {
	ids   map[string]string
	links map[string]string
	refs  map[string]string
}

func newContractPlaceholders() *contractPlaceholders {
	return &contractPlaceholders{ids: map[string]string{}, links: map[string]string{}, refs: map[string]string{}}
}

// id is the placeholder of a real id, numbered in the order ids are first
// met.
func (p *contractPlaceholders) id(real string) string {
	if v, ok := p.ids[real]; ok {
		return v
	}
	n := len(p.ids) + 1
	v := fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n)
	p.ids[real] = v
	return v
}

// ref makes the ref of the removal logged as event the start of the
// event's placeholder.
func (p *contractPlaceholders) ref(event string) {
	p.refs[event[:8]] = p.id(event)[:8]
}

// value is v with every placeholder in, keys walked in sorted order so the
// numbering never depends on map order.
func (p *contractPlaceholders) value(key string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = p.value(k, x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = p.value(key, x[i])
		}
		return out
	case string:
		return p.text(key, x)
	}
	return v
}

func (p *contractPlaceholders) text(key, s string) string {
	if strings.HasPrefix(s, fakePhotoLinkBase) && strings.HasSuffix(s, fakePhotoLinkQuery) {
		if v, ok := p.links[s]; ok {
			return v
		}
		v := fmt.Sprintf("%sphoto-%d.jpg%s", fakePhotoLinkBase, len(p.links)+1, fakePhotoLinkQuery)
		p.links[s] = v
		return v
	}
	if key == "ref" {
		if v, ok := p.refs[s]; ok {
			return v
		}
		return s
	}
	if contractInstant.MatchString(s) {
		return "<instant>"
	}
	return contractUUID.ReplaceAllStringFunc(s, p.id)
}

// newContractWorld is an admin world whose bell sends bellContact and
// whose spins come from a seeded source, so a post's category, which the
// answers carry, is the same on every run. It returns that source too.
func newContractWorld(t *testing.T) (*adminModWorld, *spintest.QueueRand) {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	rnd := spintest.NewQueueRand(7)
	w := &adminModWorld{feedWorld: &feedWorld{t: t, clk: clk}, pool: pool, photos: &fakeEntryPhotoStore{}, heard: &heardReports{}}
	adminFirst(w.feedWorld, pool, integrationOptions{clock: clk, rand: rnd, adminEmails: []string{"ct_boss@example.com"},
		entryPhotos: w.photos, contactEmail: bellContact}, "ct_boss")
	if w.cookie = adminLogin(t, w.feedWorld, "ct_boss@example.com", adminTestPassword); w.cookie == "" {
		t.Fatal("the admin could not log in")
	}
	return w, rnd
}

// firstDifference names the first line where got and want differ.
func firstDifference(want, got []byte) string {
	a, b := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return fmt.Sprintf("line %d:\n  file: %s\n  now:  %s", i+1, x, y)
		}
	}
	return "no line differs"
}

// The answers the app reads for moderation, from the real server, equal the
// golden file, and they follow the spec: a removal carries the address and
// its ref but no post and no photo; a post that is back opens the post with
// its first photo, a comment that is back opens its post with no photo;
// Today says the post was removed, with no entry; the owner is told their
// post is under review on their Board card, post page, entry and Today; and
// only the writer is told their comment is.
func TestModerationContractIntegration_TheAnswersTheAppReads(t *testing.T) {
	w, rnd := newContractWorld(t)
	o := w.person("ct_olive", "Olive Owner")
	cw := w.person("ct_wren", "Wren Writer")
	v := w.person("ct_vera", "Vera Viewer")
	reporters := []feedPerson{w.person("ct_rhea", "Rhea Reporter"), w.person("ct_remy", "Remy Reporter"), w.person("ct_rose", "Rose Reporter")}
	// Olive follows someone who never posts, so Olive's Board holds her own
	// card alone: two cards posted at the same minute would come in id order,
	// which changes from run to run.
	quiet := w.person("ct_nell", "Nell Quiet")
	for _, p := range append([]feedPerson{cw, v}, reporters...) {
		w.followAccepted(p, o)
	}
	w.followAccepted(o, quiet)
	const day = "2026-10-08"
	// Olive's spin lands on the first category of her wheel.
	rnd.Queue(0)
	post := w.post(o, day)
	for _, p := range append([]feedPerson{cw, v}, reporters...) {
		w.post(p, day)
	}
	photos := postPhotoKeys(t, w.pool, post)
	wc := w.reply(cw, post, "Rude words", "")["id"].(string)
	vc := w.reply(v, post, "Lovely light", "")["id"].(string)
	oc := w.reply(o, post, "Thanks for watching", "")["id"].(string)
	answers := map[string]map[string]any{}

	// A post someone reported is gone for them, so the comments are
	// reported first. Three reports put Wren's comment under review.
	w.clk.Advance(time.Minute)
	w.ok(reporters[0], http.MethodPost, "/v1/comments/"+oc+"/report", map[string]any{"reason": "harassment"})
	for _, p := range reporters {
		w.ok(p, http.MethodPost, "/v1/comments/"+wc+"/report", map[string]any{"reason": "spam"})
	}
	answers["post_page_writer"] = w.ok(cw, http.MethodGet, "/v1/posts/"+post, nil)

	// Three reports put Olive's post under review.
	for _, p := range reporters {
		w.ok(p, http.MethodPost, "/v1/posts/"+post+"/report", map[string]any{"reason": "hate"})
	}
	answers["board_owner"] = w.board(o, "")
	answers["post_page_owner"] = w.ok(o, http.MethodGet, "/v1/posts/"+post, nil)
	answers["entry_owner"] = w.ok(o, http.MethodGet, "/v1/entries/"+post, nil)
	answers["today_under_review"] = w.ok(o, http.MethodGet, "/v1/today", nil)

	// The admin removes Olive's comment, then her post, and brings both
	// back, a minute apart, so the bell's order never rests on ids.
	w.clk.Advance(time.Minute)
	if got := w.act("remove", "comment", o, "Bullying or harassment"); got != "/admin/people/"+o.id+"?n=removed" {
		t.Fatalf("remove the comment: %q", got)
	}
	w.clk.Advance(time.Minute)
	if got := w.act("remove", "post", o, "Hate speech"); got != "/admin/people/"+o.id+"?n=removed" {
		t.Fatalf("remove the post: %q", got)
	}
	answers["today_entry_removed"] = w.ok(o, http.MethodGet, "/v1/today", nil)
	w.clk.Advance(time.Minute)
	if got := w.restore("post", post, o); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the post back: %q", got)
	}
	w.clk.Advance(time.Minute)
	if got := w.restore("comment", oc, o); got != "/admin/removed?n=restored" {
		t.Fatalf("bring the comment back: %q", got)
	}
	bell := w.notes(o)
	decisions := []any{}
	events := map[string]string{}
	for _, it := range bell["items"].([]any) {
		item := it.(map[string]any)
		kind := item["kind"].(string)
		if slices.Contains(decisionKinds, kind) {
			if _, twice := events[kind]; twice {
				t.Fatalf("two %s items: %v", kind, bell["items"])
			}
			events[kind] = strings.TrimPrefix(item["id"].(string), kind+":")
			decisions = append(decisions, item)
		}
	}
	answers["bell"] = map[string]any{"unread": bell["unread"], "items": decisions}

	// The spec, on the answers themselves.
	for _, kind := range decisionKinds {
		if events[kind] == "" {
			t.Fatalf("no %s item in Olive's bell: %v", kind, bell["items"])
		}
	}
	for _, it := range decisions {
		item := it.(map[string]any)
		kind := item["kind"].(string)
		text := "Dune"
		if strings.HasPrefix(kind, "comment_") {
			text = "Thanks for watching"
		}
		common := map[string]any{"actor": nil, "category_name": nil, "request_id": nil, "text": text, "unread": true}
		switch kind {
		case "post_removed", "comment_removed":
			common["entry_id"], common["photo_url"], common["contact"], common["ref"] = nil, "", bellContact, events[kind][:8]
			wantItem(t, kind, item, common)
		case "post_back":
			common["entry_id"], common["photo_url"] = post, fakePhotoLink(photos[0])
			wantItem(t, kind, item, common, "contact", "ref")
		case "comment_back":
			common["entry_id"], common["photo_url"] = post, ""
			wantItem(t, kind, item, common, "contact", "ref")
		}
	}
	if removed := answers["today_entry_removed"]; removed["state"] != "posted" || removed["entry"] != nil || removed["entry_removed"] != true {
		t.Fatalf("Today after the removal: %v", removed)
	}
	if today := answers["today_under_review"]; dig(t, today, "entry", "under_review") != true || today["entry_removed"] != nil {
		t.Fatalf("Today under review: %v", today)
	}
	if got := cardIDs(t, answers["board_owner"]); !slices.Equal(got, []string{post}) {
		t.Fatalf("Olive's Board holds %v, want her own card alone", got)
	}
	card := answers["board_owner"]["days"].([]any)[0].(map[string]any)["cards"].([]any)[0]
	if dig(t, card, "mine") != true || dig(t, card, "under_review") != true {
		t.Fatalf("Olive's own card: %v", card)
	}
	if dig(t, answers["post_page_owner"], "card", "under_review") != true || dig(t, answers["entry_owner"], "entry", "under_review") != true {
		t.Fatalf("the owner's post page %v, entry %v", answers["post_page_owner"]["card"], answers["entry_owner"]["entry"])
	}
	flags := func(page map[string]any) map[string]any {
		out := map[string]any{}
		for _, c := range page["comments"].([]any) {
			out[c.(map[string]any)["id"].(string)] = underReview(c)
		}
		return out
	}
	if got, want := flags(answers["post_page_owner"]), map[string]any{vc: "absent", oc: "absent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the comments as the post's owner: %v, want %v", got, want)
	}
	if got, want := flags(answers["post_page_writer"]), map[string]any{wc: true, vc: "absent", oc: "absent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the comments as Wren: %v, want %v", got, want)
	}
	if got := underReview(answers["post_page_writer"]["card"]); got != "absent" {
		t.Fatalf("Wren is told Olive's post is under review: %v", got)
	}
	for name, a := range answers {
		wantSignedPhotoLinks(t, name, a)
	}

	// The placeholders: the people, the post and the comments first, then
	// the decisions, so their numbers never move; any other id is numbered
	// as it is met.
	ph := newContractPlaceholders()
	roles := []struct{ name, id string }{
		{"owner", o.id}, {"writer", cw.id}, {"viewer", v.id}, {"post", post},
		{"writer_comment", wc}, {"viewer_comment", vc}, {"owner_comment", oc},
	}
	for _, kind := range decisionKinds {
		roles = append(roles, struct{ name, id string }{kind, events[kind]})
	}
	legend := map[string]any{}
	for _, r := range roles {
		legend[r.name] = ph.id(r.id)
	}
	ph.ref(events["post_removed"])
	ph.ref(events["comment_removed"])
	golden := map[string]any{"ids": legend}
	sections := make([]string, 0, len(answers))
	for name := range answers {
		sections = append(sections, name)
	}
	slices.Sort(sections)
	for _, name := range sections {
		golden[name] = ph.value(name, answers[name])
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(golden); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(moderationContractFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(moderationContractFile, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", moderationContractFile)
		return
	}
	want, err := os.ReadFile(moderationContractFile)
	if err != nil {
		t.Fatalf("read %s: %v (UPDATE_GOLDEN=1 writes it)", moderationContractFile, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the moderation answers differ from %s at %s\nIf the change is meant, UPDATE_GOLDEN=1 writes the file again; "+
			"then run scripts/check-spinposts-extra.ts, which reads it through the app.", moderationContractFile, firstDifference(want, got))
	}
}
