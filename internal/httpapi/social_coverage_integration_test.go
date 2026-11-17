package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// socialPassword is the password of every account these tests sign up.
const socialPassword = "Correct-horse-battery-staple1"

// socialSend sends one request as p and returns the status and the raw body,
// so a refusal can be compared byte for byte with a miss.
func socialSend(w *feedWorld, p feedPerson, method, path string, body any) (int, string) {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, method, path, p.token, body)
	return rec.Code, rec.Body.String()
}

// socialDot is the bell's dot as p's summary gives it.
func socialDot(w *feedWorld, p feedPerson) bool {
	w.t.Helper()
	return w.ok(p, http.MethodGet, "/v1/notifications/summary", nil)["unread"] == true
}

// CMT-04: a hidden comment is seen only by its writer and by the post's
// author. A reply the author hid leaves the bell of the person whose comment
// it answers, as it leaves their post page, and takes the dot with it. The
// author still lists it, and Unhide brings it back. The writer of a hidden
// top-level comment keeps it, the reply under it and that reply's item.
func TestSocialCoverage_AHiddenReplyLeavesTheBellOfTheCommentItAnswers(t *testing.T) {
	w := newFeedWorld(t)
	a := w.person("sh_alice", "Alice Ames")
	b := w.person("sh_bob", "Bob Brown")
	c := w.person("sh_cara", "Cara Cole")
	w.followAccepted(a, b)
	w.followAccepted(c, b)
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)

	aTop := w.reply(a, bEntry, "Great pick", "")["id"].(string)
	// A opens the bell before the reply comes in.
	w.ok(a, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	reply := w.reply(c, bEntry, "@sh_alice agreed", aTop)["id"].(string)
	item := "reply:" + reply
	if got := itemByID(w.notes(a), item); got == nil || got["unread"] != true || !socialDot(w, a) {
		t.Fatalf("the reply before the hide: %v (dot %v)", w.notes(a), socialDot(w, a))
	}

	// B hides the reply: gone from A's bell and A's post page, and the dot
	// with it.
	w.ok(b, http.MethodPost, "/v1/comments/"+reply+"/hide", map[string]any{})
	if list := w.notes(a); itemByID(list, item) != nil || list["unread"] != false {
		t.Fatalf("a hidden reply is still in the bell of the comment it answers: %v", list)
	}
	if socialDot(w, a) {
		t.Fatal("a hidden reply still lights the dot")
	}
	if ids, _ := w.commentIDs(a, bEntry); has(ids, reply) || !has(ids, aTop) {
		t.Fatalf("A's post page after the hide: %v", ids)
	}
	// B, the post's author, still lists it; C, its writer, still reads it.
	if itemByID(w.notes(b), "comment:"+reply) == nil {
		t.Fatalf("the author lost the hidden reply's item: %v", w.notes(b))
	}
	if ids, _ := w.commentIDs(c, bEntry); !has(ids, reply) {
		t.Fatalf("the writer lost the hidden reply: %v", ids)
	}

	// Unhide brings it back, still unread.
	w.ok(b, http.MethodPost, "/v1/comments/"+reply+"/unhide", map[string]any{})
	if got := itemByID(w.notes(a), item); got == nil || got["unread"] != true || !socialDot(w, a) {
		t.Fatalf("the reply after the unhide: %v", w.notes(a))
	}

	// B hides A's top-level comment: A, its writer, still reads it with the
	// reply under it, and keeps the reply's item.
	w.ok(b, http.MethodPost, "/v1/comments/"+aTop+"/hide", map[string]any{})
	if ids, _ := w.commentIDs(a, bEntry); !has(ids, aTop) || !has(ids, reply) {
		t.Fatalf("the writer of a hidden comment lost it or its reply: %v", ids)
	}
	if itemByID(w.notes(a), item) == nil {
		t.Fatalf("the writer of a hidden comment lost the reply item under it: %v", w.notes(a))
	}
}

// SOC-03: search matches the start of a username, a name or a word of a
// name, never an email address, and answers with at most 20 people, an exact
// username first. Lookup takes a username only.
func TestSocialCoverage_SearchNeverMatchesAnEmailAndAnswersAtMost20(t *testing.T) {
	w := newFeedWorld(t)
	seeker := w.person("sp_seeker", "Sam Seeker")
	// The address shares nothing with the username or the name.
	integrationOnboardAccount(t, w.engine, "hidden_mailbox@example.com", socialPassword, "Quinn Quiet", "sp_quiet")
	search := func(q string) []string {
		t.Helper()
		return w.searchNames(seeker, url.QueryEscape(q))
	}

	for _, q := range []string{"hidden", "hidden_mailbox", "hidden_mailbox@example.com", "@hidden_mailbox", "mailbox", "example", "example.com", "@example.com"} {
		if got := search(q); len(got) != 0 {
			t.Fatalf("a search for %q matched an email address: %v", q, got)
		}
	}
	// The control: the same account by its username, its name and a word of
	// its name.
	for _, q := range []string{"sp_qu", "Quinn", "quiet"} {
		if got := search(q); strings.Join(got, ",") != "sp_quiet" {
			t.Fatalf("a search for %q: %v, want sp_quiet", q, got)
		}
	}
	// Lookup: an email address is a bad shape, its first part is no one.
	if got := w.lookup(seeker, url.QueryEscape("hidden_mailbox@example.com"), http.StatusBadRequest); got["field"] != "username" {
		t.Fatalf("lookup by email: %v", got)
	}
	w.lookup(seeker, "hidden_mailbox", http.StatusNotFound)
	if got := w.lookup(seeker, "sp_quiet", http.StatusOK); dig(t, got, "person", "username") != "sp_quiet" {
		t.Fatalf("lookup by username: %v", got)
	}

	// 23 accounts start with sp_many: 20 come back, the exact one first, then
	// the rest by username.
	for i := range 22 {
		name := fmt.Sprintf("sp_many_%02d", i)
		integrationOnboardAccount(t, w.engine, name+"@example.com", socialPassword, "Many Person", name)
	}
	integrationOnboardAccount(t, w.engine, "sp_many@example.com", socialPassword, "Many Exact", "sp_many")
	want := []string{"sp_many"}
	for i := range 19 {
		want = append(want, fmt.Sprintf("sp_many_%02d", i))
	}
	if got := search("sp_many"); social.MaxSearchResults != 20 || !sameList(got, want) {
		t.Fatalf("a search with 23 matches: %d %v, want 20: %v", len(got), got, want)
	}
}

// NOT-01: the bell says "You hit your goal this week. One bonus spin added."
// (kind bonus_spin, text the goal) when the week's goal is met, and the
// Milestones switch hides it and its dot. The bell keeps the last 30 days:
// a month on, the bonus and a comment are gone and a waiting follow request
// stays.
func TestSocialCoverage_TheBellSaysABonusSpinWasAdded(t *testing.T) {
	w := newFeedWorld(t)
	w.at("2026-10-05", 9, 0)
	b := w.person("sb_bob", "Bob Brown")
	c := w.person("sb_cara", "Cara Cole")
	d := w.person("sb_dan", "Dan Dove")
	w.followAccepted(c, b)
	for _, day := range []string{"2026-10-05", "2026-10-06", "2026-10-07"} {
		w.post(b, day)
	}
	// B opens the bell at 8:15 PM on Thursday, after the wheel opened.
	const thursday = "2026-10-08"
	w.at(thursday, 20, 15)
	w.ok(b, http.MethodPost, "/v1/notifications/seen", map[string]any{})
	// The fourth post of the week meets the goal of 4 at 8:30 PM.
	bEntry := w.post(b, thursday)
	w.at(thursday, 21, 0)

	const id = "bonus_spin:2026-10-05"
	bonus := func() (map[string]any, bool) {
		t.Helper()
		list := w.notes(b)
		return itemByID(list, id), list["unread"] == true
	}
	item, unread := bonus()
	if item == nil || item["kind"] != "bonus_spin" || item["text"] != "4" || item["at_label"] != "8:30 PM" ||
		item["section"] != "today" || item["unread"] != true || item["actor"] != nil || !unread || !socialDot(w, b) {
		t.Fatalf("the bonus item: %v in %v", item, w.notes(b))
	}

	// The Milestones switch hides it, and the dot with it.
	w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"milestones": false})
	if item, unread := bonus(); item != nil || unread || socialDot(w, b) {
		t.Fatalf("milestones off: %v (list unread %v)", item, unread)
	}
	w.ok(b, http.MethodPatch, "/v1/me/notification-settings", map[string]any{"milestones": true})
	if item, unread := bonus(); item == nil || !unread || !socialDot(w, b) {
		t.Fatalf("milestones on again: %v (list unread %v)", item, unread)
	}

	// That evening D asks to follow B, and C posts and comments on B's post.
	w.at(thursday, 21, 5)
	w.follow(d, b)
	w.postAt(c, thursday, 21, 10)
	w.clk.Advance(time.Minute)
	comment := w.call(c, http.MethodPost, "/v1/posts/"+bEntry+"/comments", map[string]any{"text": "Nice week"}, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
	list := w.notes(b)
	if itemByID(list, "comment:"+comment) == nil || dig(t, itemOf(t, list, "follow_request"), "actor", "id") != d.id {
		t.Fatalf("that evening: %v", list)
	}

	// 31 days on: the bonus and the comment are gone, the request waits.
	w.clk.Advance(31 * 24 * time.Hour)
	list = w.notes(b)
	if itemByID(list, id) != nil || itemByID(list, "comment:"+comment) != nil {
		t.Fatalf("items older than 30 days: %v", list)
	}
	if dig(t, itemOf(t, list, "follow_request"), "actor", "id") != d.id {
		t.Fatalf("a waiting request whatever its age: %v", list)
	}
}

// ADM-04: Dismiss closes every report on the card and only those. A report
// of the same comment that came in after the page was drawn stays open, alone
// on its card, and the Reports tab still counts it.
func TestSocialCoverage_DismissClosesOnlyTheReportsOnTheCardAsItWasDrawn(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("sd_alice", "Alice Ames")
	b := w.person("sd_bob", "Bob Brown")
	r1 := w.person("sd_rae", "Rae One")
	r2 := w.person("sd_rex", "Rex Two")
	r3 := w.person("sd_roy", "Roy Three")
	const day = "2026-10-08"
	others := []feedPerson{b, r1, r2, r3}
	for _, p := range others {
		w.followAccepted(p, a)
	}
	aEntry := w.post(a, day)
	for _, p := range others {
		w.post(p, day)
	}
	comment := w.reply(b, aEntry, "rude", "")["id"].(string)
	report := "/v1/comments/" + comment + "/report"
	w.ok(r1, http.MethodPost, report, map[string]any{"reason": "harassment"})
	w.ok(r2, http.MethodPost, report, map[string]any{"reason": "spam"})

	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	if cookie == "" {
		t.Fatal("the admin could not log in")
	}
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	card := adminReportOf(t, page, "Bullying or harassment · Spam")
	form := url.Values{"csrf": {csrfOf(t, page)}, "kind": {"comment"}, "person": {b.id}}
	for _, m := range dismissIDs.FindAllStringSubmatch(card[strings.Index(card, `action="/admin/reports/dismiss"`):], -1) {
		form.Add("id", m[1])
	}
	if len(form["id"]) != 2 {
		t.Fatalf("the card's Dismiss sends %d ids, want 2: %s", len(form["id"]), card)
	}

	// R3 reports the comment after the page was drawn; then Dismiss.
	w.ok(r3, http.MethodPost, report, map[string]any{"reason": "hate"})
	rec := adminDo(t, w, http.MethodPost, "/admin/reports/dismiss", cookie, form, "")
	if rec.Header().Get("Location") != "/admin/people/"+b.id+"?n=dismissed" {
		t.Fatalf("dismiss: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	page = adminDo(t, w, http.MethodGet, "/admin/people/"+b.id, cookie, nil, "").Body.String()
	open := adminReportOf(t, page, "Hate speech")
	if !strings.Contains(open, "Reported by @sd_roy ·") || strings.Contains(open, "sd_rae") || strings.Contains(open, "sd_rex") ||
		!strings.Contains(open, "<button>Dismiss</button>") || !strings.Contains(open, "Remove comment") {
		t.Fatalf("the report that came in later, want it open and alone: %s", open)
	}
	closed := adminReportOf(t, page, "Bullying or harassment · Spam")
	if !strings.Contains(closed, `<p class="by">Dismissed · `) || strings.Contains(closed, "<button>Dismiss</button>") {
		t.Fatalf("the dismissed card: %s", closed)
	}
	if at := strings.Index(page, "Closed reports"); at < 0 || strings.Index(page, "Hate speech") > at {
		t.Fatalf("the open card is not ahead of Closed reports: %s", page)
	}
	list := adminDo(t, w, http.MethodGet, "/admin", cookie, nil, "").Body.String()
	if !strings.Contains(list, "<title>(1) Reports · RNDMRoll admin</title>") || strings.Count(list, `<article class="card`) != 1 ||
		!strings.Contains(list, "Hate speech") {
		t.Fatalf("the Reports tab, want the one open card: %s", list)
	}
}

// ADM-06: a report of a comment that is kept reaches the admin email's
// notifier, as a report of a person or a post does. Refused ones do not.
func TestSocialCoverage_ACommentReportIsHeardByTheAdminEmail(t *testing.T) {
	pool := requireIntegrationPool(t)
	heard := &heardReports{}
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &feedWorld{t: t, clk: clk}
	w.engine = buildIntegrationServerWith(t, pool, integrationOptions{clock: clk,
		wire: func(s *social.Service, b *board.Service, _ *Deps) {
			s.SetReportNotifier(heard)
			b.SetReportNotifier(heard)
		}}).Engine()
	owner := w.person("se_owen", "Owen Owner")
	fan := w.person("se_fay", "Fay Fan")
	writer := w.person("se_wes", "Wes Writer")
	stranger := w.person("se_sam", "Sam Stone")
	w.followAccepted(fan, owner)
	w.followAccepted(writer, owner)
	const day = "2026-10-08"
	entry := w.post(owner, day)
	w.post(fan, day)
	w.post(writer, day)
	path := "/v1/comments/" + w.reply(writer, entry, "Not nice", "")["id"].(string) + "/report"

	w.call(writer, http.MethodPost, path, map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.call(stranger, http.MethodPost, path, map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.call(fan, http.MethodPost, path, map[string]any{"reason": "nope"}, http.StatusBadRequest)
	w.ok(fan, http.MethodPost, path, map[string]any{"reason": "nudity"})
	if got := heard.got(); !slices.Equal(got, []string{"nudity"}) {
		t.Fatalf("heard %v, want only the kept comment report", got)
	}
}

// SAFE-01: a comment report is refused with the reason's own 400 (checked
// first), or with the very answer of a comment that is not there: your own
// comment, a post you cannot see, a comment the post's author hid (unless you
// are that author), and a block either way with its writer (ReportComment:
// "a writer with a block either way are ErrNotFound"; the app hides a
// blocked person's comments both ways). A report kept is 200 with an empty
// body and shows on the admin page.
func TestSocialCoverage_ACommentReportIsRefusedLikeAMiss(t *testing.T) {
	w, _ := newAdminWorld(t)
	a := w.person("sr_alice", "Alice Ames")
	b := w.person("sr_bob", "Bob Brown")  // the post's author
	c := w.person("sr_cara", "Cara Cole") // the comment's writer
	d := w.person("sr_dan", "Dan Dove")   // follows B, did not post that day
	stranger := w.person("sr_sam", "Sam Stone")
	for _, p := range []feedPerson{a, c, d} {
		w.followAccepted(p, b)
	}
	const day = "2026-10-08"
	bEntry := w.post(b, day)
	w.post(a, day)
	w.post(c, day)
	comment := w.reply(c, bEntry, "Hello there", "")["id"].(string)
	path := "/v1/comments/" + comment + "/report"
	spam := map[string]any{"reason": "spam"}

	missCode, missBody := socialSend(w, a, http.MethodPost, "/v1/comments/"+uuid.NewString()+"/report", spam)
	if missCode != http.StatusNotFound {
		t.Fatalf("a comment that is not there: %d %s", missCode, missBody)
	}
	likeAMiss := func(label string, p feedPerson) {
		t.Helper()
		if code, body := socialSend(w, p, http.MethodPost, path, spam); code != missCode || body != missBody {
			t.Fatalf("%s: %d %s, want the miss %d %s", label, code, body, missCode, missBody)
		}
	}

	// The reason is checked first, also for a comment that is not there and
	// for someone who cannot see the post. (The stranger sends these: each
	// person may send 10 comment reports a minute, and the frozen test clock
	// never lets that refill.)
	for _, bad := range []struct {
		body   map[string]any
		reason string
	}{
		{map[string]any{"reason": "nope"}, "invalid"},
		{map[string]any{}, "required"},
		{map[string]any{"reason": nil}, "required"},
	} {
		for _, target := range []string{path, "/v1/comments/" + uuid.NewString() + "/report"} {
			if got := w.call(stranger, http.MethodPost, target, bad.body, http.StatusBadRequest); got["field"] != "reason" || got["reason"] != bad.reason {
				t.Fatalf("%v on %s: %v, want reason %s", bad.body, target, got, bad.reason)
			}
		}
	}
	likeAMiss("its own writer", c)
	likeAMiss("a stranger who follows nobody", stranger)
	likeAMiss("a follower who did not post that day", d)

	// Hidden by the post's author: refused for anyone else, kept for the
	// author.
	w.ok(b, http.MethodPost, "/v1/comments/"+comment+"/hide", map[string]any{})
	likeAMiss("someone else, once the author hid it", a)
	likeAMiss("its own writer, hidden", c)
	w.ok(b, http.MethodPost, path, map[string]any{"reason": "harassment"})
	w.ok(b, http.MethodPost, "/v1/comments/"+comment+"/unhide", map[string]any{})

	// A block either way with its writer.
	w.ok(c, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": a.id})
	likeAMiss("blocked by its writer", a)
	w.call(c, http.MethodDelete, "/v1/me/blocks/"+a.id, nil, http.StatusNoContent)
	w.ok(a, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": c.id})
	likeAMiss("its writer blocked by the reporter", a)
	w.call(a, http.MethodDelete, "/v1/me/blocks/"+c.id, nil, http.StatusNoContent)

	// Kept: 200 with an empty body, on the admin page with both reporters.
	if code, body := socialSend(w, a, http.MethodPost, path, spam); code != http.StatusOK || body != "{}" {
		t.Fatalf("a report kept: %d %s", code, body)
	}
	cookie := adminLogin(t, w, "ad_boss@example.com", adminTestPassword)
	page := adminDo(t, w, http.MethodGet, "/admin/people/"+c.id, cookie, nil, "").Body.String()
	card := adminReportOf(t, page, "Bullying or harassment · Spam")
	if !(strings.Contains(card, "Reported by @sr_alice and @sr_bob ·") || strings.Contains(card, "Reported by @sr_bob and @sr_alice ·")) ||
		!strings.Contains(card, "Hello there") || !strings.Contains(card, `<p class="kind">Comment</p>`) {
		t.Fatalf("the comment's card: %s", card)
	}
}

// Rate limits on the social routes (Appendix B and every Register): each
// route allows its number of requests a minute per person, answered or not,
// then 429 rate_limited, and another person is not held back. Each route
// runs with a person of its own, so no route's count can reach another's.
// Whether the three report routes should share one count of 10 (SAFE-01
// says "10 reports a minute per person") is the developer's call, so this
// test does not decide it.
func TestSocialCoverage_SocialRoutesAreLimitedPerPersonAMinute(t *testing.T) {
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk,
		wire: func(_ *social.Service, _ *board.Service, deps *Deps) {
			deps.PushTokens = NewPushTokenHandler(postgres.NewPushRepo(pool))
		}}).Engine()
	n := 0
	fresh := func(t *testing.T) string {
		t.Helper()
		n++
		name := fmt.Sprintf("sl_p%02d", n)
		return integrationOnboardAccount(t, engine, name+"@example.com", socialPassword, "Lim Person", name).AccessToken
	}
	other := fresh(t)
	id := uuid.NewString()
	token := map[string]any{"token": "ExponentPushToken[socialcoverage]"}
	for _, c := range []struct {
		name, method, path string
		body               any
		limit              int
	}{
		{"lookup", http.MethodGet, "/v1/people/lookup?username=nobody_here", nil, 30},
		{"search", http.MethodGet, "/v1/people/search?q=zz", nil, 120},
		{"follows", http.MethodGet, "/v1/follows", nil, 60},
		{"follow", http.MethodPost, "/v1/follows", map[string]any{"user_id": id}, 20},
		{"unfollow", http.MethodPost, "/v1/follows/" + id + "/unfollow", map[string]any{}, 20},
		{"accept", http.MethodPost, "/v1/follows/requests/" + id + "/accept", map[string]any{}, 20},
		{"decline", http.MethodPost, "/v1/follows/requests/" + id + "/decline", map[string]any{}, 20},
		{"remove follower", http.MethodPost, "/v1/followers/" + id + "/remove", map[string]any{}, 20},
		{"blocks", http.MethodGet, "/v1/me/blocks", nil, 60},
		{"block", http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": id}, 20},
		{"unblock", http.MethodDelete, "/v1/me/blocks/" + id, nil, 20},
		{"settings", http.MethodGet, "/v1/me/notification-settings", nil, 60},
		{"settings change", http.MethodPatch, "/v1/me/notification-settings", map[string]any{}, 30},
		{"suggestions", http.MethodGet, "/v1/people/suggestions", nil, 60},
		{"person", http.MethodGet, "/v1/people/" + id, nil, 120},
		{"their followers", http.MethodGet, "/v1/people/" + id + "/followers", nil, 60},
		{"their following", http.MethodGet, "/v1/people/" + id + "/following", nil, 60},
		{"mute", http.MethodPost, "/v1/people/" + id + "/mute", map[string]any{}, 30},
		{"unmute", http.MethodPost, "/v1/people/" + id + "/unmute", map[string]any{}, 30},
		{"person report", http.MethodPost, "/v1/people/" + id + "/report", map[string]any{"reason": "spam"}, 10},
		{"board", http.MethodGet, "/v1/board", nil, 60},
		{"post", http.MethodGet, "/v1/posts/" + id, nil, 60},
		{"reaction", http.MethodPost, "/v1/posts/" + id + "/reaction", map[string]any{"reaction": "fire"}, 60},
		{"comment", http.MethodPost, "/v1/posts/" + id + "/comments", map[string]any{"text": "hi"}, 20},
		{"delete comment", http.MethodPost, "/v1/comments/" + id + "/delete", map[string]any{}, 20},
		{"post report", http.MethodPost, "/v1/posts/" + id + "/report", map[string]any{"reason": "spam"}, 10},
		{"comment report", http.MethodPost, "/v1/comments/" + id + "/report", map[string]any{"reason": "spam"}, 10},
		{"hide", http.MethodPost, "/v1/comments/" + id + "/hide", map[string]any{}, 30},
		{"unhide", http.MethodPost, "/v1/comments/" + id + "/unhide", map[string]any{}, 30},
		{"notifications", http.MethodGet, "/v1/notifications", nil, 60},
		{"notifications summary", http.MethodGet, "/v1/notifications/summary", nil, 120},
		{"notifications seen", http.MethodPost, "/v1/notifications/seen", map[string]any{}, 30},
		{"push token", http.MethodPost, "/v1/me/push-tokens", token, 20},
		{"push token removal", http.MethodDelete, "/v1/me/push-tokens", token, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := fresh(t)
			// Refused requests count too: a made-up id answers 404 and uses
			// up the same allowance. A route that is not mounted at all
			// (gin's own 404 page) would never be limited, so it fails here.
			for i := 1; i <= c.limit; i++ {
				rec := integrationDo(t, engine, c.method, c.path, p, c.body)
				if rec.Code == http.StatusTooManyRequests || (rec.Code == http.StatusNotFound && strings.Contains(rec.Body.String(), "404 page not found")) {
					t.Fatalf("request %d of %d: %d %s", i, c.limit, rec.Code, rec.Body.String())
				}
			}
			rec := integrationDo(t, engine, c.method, c.path, p, c.body)
			if rec.Code != http.StatusTooManyRequests || decodeBody(t, rec)["error"] != "rate_limited" {
				t.Fatalf("request %d: %d %s, want 429 rate_limited", c.limit+1, rec.Code, rec.Body.String())
			}
			if rec := integrationDo(t, engine, c.method, c.path, other, c.body); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("another person was limited: %s", rec.Body.String())
			}
		})
	}
}
