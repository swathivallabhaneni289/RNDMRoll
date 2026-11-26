package httpapi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// Moderation on every read (migration 0020): a post or comment the viewer
// reported is gone for them for good; one with social.ReviewThreshold open
// reports from accounts that are not gone is hidden while under review from
// everyone but its owner (for a comment, its writer); one an admin removed is
// gone for everyone, its owner included. Reports go through the app. A
// dismissed report, a gone reporter and a removal are set straight in the
// database, as the admin page does.

// modWorld is a feed world over a pool the test also writes to, with the
// board and spin services on the same database, so a test can read the
// owner's under review flags straight from the services as well as from the
// answers. rand picks the spins' winners, so a test can choose a post's
// category.
type modWorld struct {
	*feedWorld
	pool     *pgxpool.Pool
	boardSvc *board.Service
	spinSvc  *spin.Service
	rand     *spintest.QueueRand
}

func newModWorld(t *testing.T) *modWorld {
	t.Helper()
	return newModWorldWith(t, nil)
}

// newModWorldWith is newModWorld with wire run on the server's board service
// as it is built (to set its report notifier, say); nil for nothing.
func newModWorldWith(t *testing.T, wire func(*board.Service)) *modWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	clk := clock.NewFixed(time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC))
	w := &modWorld{pool: pool, rand: spintest.NewQueueRand(20)}
	engine := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, rand: w.rand,
		wire: func(_ *social.Service, b *board.Service, _ *Deps) {
			w.boardSvc = b
			if wire != nil {
				wire(b)
			}
		}}).Engine()
	w.feedWorld = &feedWorld{t: t, engine: engine, clk: clk}
	w.spinSvc = spin.NewService(postgres.NewSpinRepo(pool), clk, spin.NewSystemRand())
	return w
}

// sql runs one statement straight on the database and wants it to change
// exactly one row.
func (w *modWorld) sql(q string, args ...any) {
	w.t.Helper()
	tag, err := w.pool.Exec(context.Background(), q, args...)
	if err != nil || tag.RowsAffected() != 1 {
		w.t.Fatalf("%s: %v, %d rows changed, want one", q, err, tag.RowsAffected())
	}
}

// report sends p's report of a post or a comment ("posts" or "comments")
// through the app.
func (w *modWorld) report(p feedPerson, what, id string) {
	w.t.Helper()
	w.ok(p, http.MethodPost, "/v1/"+what+"/"+id+"/report", map[string]any{"reason": "spam"})
}

// seesPost reports whether p can open the post page of entry; anything but
// the page or the miss fails the test.
func (w *modWorld) seesPost(p feedPerson, entry string) bool {
	w.t.Helper()
	rec := integrationDo(w.t, w.engine, http.MethodGet, "/v1/posts/"+entry, p.token, nil)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		w.t.Fatalf("the post as %s: %d %s", p.name, rec.Code, rec.Body.String())
	}
	return rec.Code == http.StatusOK
}

// onBoard reports whether entry has a card on p's Board.
func (w *modWorld) onBoard(p feedPerson, entry string) bool {
	w.t.Helper()
	return has(cardIDs(w.t, w.board(p, "")), entry)
}

// grid is owner's page as p sees it: the entry ids of its grid and its
// Posts figure.
func (w *modWorld) grid(p, owner feedPerson) ([]string, float64) {
	w.t.Helper()
	page := w.ok(p, http.MethodGet, "/v1/people/"+owner.id, nil)
	ids := []string{}
	posts, _ := page["posts"].([]any)
	for _, v := range posts {
		ids = append(ids, v.(map[string]any)["entry_id"].(string))
	}
	return ids, dig(w.t, page, "stats", "posts").(float64)
}

// locked is p's locked card: the names on it and its posted count.
func (w *modWorld) locked(p feedPerson) ([]string, float64) {
	w.t.Helper()
	b := w.board(p, "")
	if b["state"] != board.StateLocked {
		w.t.Fatalf("%s's Board is %v, want locked", p.name, b["state"])
	}
	names := []string{}
	for _, n := range dig(w.t, b, "locked", "names").([]any) {
		names = append(names, n.(string))
	}
	return names, dig(w.t, b, "locked", "posted_count").(float64)
}

// bell lists the ids of p's bell items.
func (w *modWorld) bell(p feedPerson) []string {
	w.t.Helper()
	out := []string{}
	for _, v := range w.notes(p)["items"].([]any) {
		out = append(out, v.(map[string]any)["id"].(string))
	}
	return out
}

// pushTo lists who a push about the comment would go to, as kind:user id,
// sorted.
func (w *modWorld) pushTo(comment string) []string {
	w.t.Helper()
	targets, err := postgres.NewPushRepo(w.pool).CommentTargets(context.Background(), uuid.MustParse(comment))
	if err != nil {
		w.t.Fatalf("push targets: %v", err)
	}
	out := []string{}
	for _, tg := range targets {
		out = append(out, tg.Kind+":"+tg.UserID.String())
	}
	sort.Strings(out)
	return out
}

// diary is p's diary page: its entry rows' ids in order, their categories
// by id, the comment count on each, its skipped dates and its category
// chips.
type diaryPage struct {
	entries  []string
	category map[string]string
	comments map[string]float64
	skipped  []string
	chips    []string
}

func (w *modWorld) diary(p feedPerson) diaryPage {
	w.t.Helper()
	got := w.ok(p, http.MethodGet, "/v1/diary", nil)
	out := diaryPage{category: map[string]string{}, comments: map[string]float64{}}
	for _, v := range got["items"].([]any) {
		row := v.(map[string]any)
		if row["kind"] != "entry" {
			out.skipped = append(out.skipped, row["local_date"].(string))
			continue
		}
		id := dig(w.t, row, "entry", "id").(string)
		out.entries = append(out.entries, id)
		out.category[id] = dig(w.t, row, "entry", "category", "id").(string)
		out.comments[id] = dig(w.t, row, "social", "comment_count").(float64)
	}
	for _, c := range got["categories"].([]any) {
		out.chips = append(out.chips, c.(map[string]any)["id"].(string))
	}
	sort.Strings(out.chips)
	return out
}

// ownFlags is what the services tell the owner about their own post: the
// under review flag of its card on the post page and on their Board, and of
// the entry alone, in Today, and in the diary.
func (w *modWorld) ownFlags(owner feedPerson, entry string) []bool {
	w.t.Helper()
	ctx := context.Background()
	oid, eid := uuid.MustParse(owner.id), uuid.MustParse(entry)
	post, err := w.boardSvc.Post(ctx, oid, eid)
	if err != nil {
		w.t.Fatalf("the owner's post page: %v", err)
	}
	b, err := w.boardSvc.Board(ctx, oid, nil, board.DefaultDays)
	if err != nil {
		w.t.Fatalf("the owner's Board: %v", err)
	}
	var card *board.Card
	for _, d := range b.Days {
		for i := range d.Cards {
			if d.Cards[i].EntryID == eid {
				card = &d.Cards[i]
			}
		}
	}
	if card == nil {
		w.t.Fatalf("no card of %s on the owner's Board", entry)
	}
	e, err := w.spinSvc.GetEntry(ctx, owner.id, entry)
	if err != nil {
		w.t.Fatalf("the owner's entry: %v", err)
	}
	today, err := w.spinSvc.Today(ctx, owner.id)
	if err != nil || today.Entry == nil || today.Entry.ID != entry {
		w.t.Fatalf("the owner's Today: %+v, %v", today.Entry, err)
	}
	rows, err := postgres.NewDiaryReader(w.pool).Rows(ctx, owner.id, nil, "", 5)
	if err != nil {
		w.t.Fatalf("the owner's diary: %v", err)
	}
	inDiary := false
	for _, r := range rows {
		if r.Entry != nil && r.Entry.ID == entry {
			inDiary = r.Entry.UnderReview
		}
	}
	return []bool{post.Card.UnderReview, card.UnderReview, e.UnderReview, today.Entry.UnderReview, inDiary}
}

// A post the viewer reported is gone for them for good: the post page and
// all that hangs off it, the Board, the person page's grid and Posts figure,
// the bell and the push. Reporting it again is answered exactly like a miss.
// Nothing changes for anyone else.
func TestModerationIntegration_AReportedPostIsGoneForItsReporterOnly(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mr_olive", "Olive Owner")
	r := w.person("mr_rhea", "Rhea Reporter")
	v := w.person("mr_vera", "Vera Viewer")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{r, v} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	// R comments and O answers: the answer is in R's bell, and its push goes
	// to R.
	rComment := w.reply(r, entry, "Looks good", "")["id"].(string)
	answer := w.reply(o, entry, "Thanks", rComment)["id"].(string)
	if got := w.bell(r); !has(got, "reply:"+answer) {
		t.Fatalf("R's bell before the report: %v", got)
	}
	if got := w.pushTo(answer); !slices.Equal(got, []string{"reply:" + r.id}) {
		t.Fatalf("the answer's push before the report: %v", got)
	}

	w.report(r, "posts", entry)

	miss := integrationDo(t, w.engine, http.MethodPost, "/v1/posts/"+uuid.NewString()+"/report", r.token, map[string]any{"reason": "spam"})
	again := integrationDo(t, w.engine, http.MethodPost, "/v1/posts/"+entry+"/report", r.token, map[string]any{"reason": "hate"})
	if again.Code != http.StatusNotFound || again.Body.String() != miss.Body.String() {
		t.Fatalf("a report again: %d %s, want the miss's %d %s", again.Code, again.Body.String(), miss.Code, miss.Body.String())
	}
	if w.seesPost(r, entry) || w.onBoard(r, entry) {
		t.Fatal("R still sees the post they reported")
	}
	w.call(r, http.MethodPost, "/v1/posts/"+entry+"/reaction", map[string]any{"reaction": "love"}, http.StatusNotFound)
	w.call(r, http.MethodPost, "/v1/posts/"+entry+"/comments", map[string]any{"text": "Hello?"}, http.StatusNotFound)
	if ids, posts := w.grid(r, o); len(ids) != 0 || posts != 0 {
		t.Fatalf("O's page as R: grid %v, Posts %v, want neither to have the post", ids, posts)
	}
	later := w.reply(o, entry, "Anyone there?", rComment)["id"].(string)
	if got := w.bell(r); has(got, "reply:"+answer) || has(got, "reply:"+later) {
		t.Fatalf("R's bell after the report: %v", got)
	}
	if got := w.pushTo(later); len(got) != 0 {
		t.Fatalf("a push about the post R reported: %v", got)
	}

	// Its owner and Vera still see it all.
	for _, p := range []feedPerson{o, v} {
		if !w.seesPost(p, entry) || !w.onBoard(p, entry) {
			t.Fatalf("%s lost the post R reported", p.name)
		}
	}
	if ids, posts := w.grid(v, o); !slices.Equal(ids, []string{entry}) || posts != 1 {
		t.Fatalf("O's page as Vera: grid %v, Posts %v", ids, posts)
	}
	if got := w.ownFlags(o, entry); slices.Contains(got, true) {
		t.Fatalf("one report does not put the post under review: %v", got)
	}
}

// A comment the viewer reported is gone for them with the replies under it,
// wherever it shows: the post page and its count, the Board's count, the
// diary, the bell and the push. Here the post's owner reports it, and then
// its writer reports a reply under it. Everyone else still sees it all.
func TestModerationIntegration_AReportedCommentTakesItsRepliesForItsReporterOnly(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mc_olive", "Olive Owner")
	cw := w.person("mc_wren", "Wren Writer")
	y := w.person("mc_yuri", "Yuri Replier")
	v := w.person("mc_vera", "Vera Viewer")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, y, v} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	rude := w.reply(cw, entry, "Rude", "")["id"].(string)
	under := w.reply(y, entry, "Agreed", rude)["id"].(string)
	kind := w.reply(v, entry, "Lovely", "")["id"].(string)
	if got := w.bell(o); !has(got, "comment:"+rude) || !has(got, "comment:"+under) || !has(got, "comment:"+kind) {
		t.Fatalf("O's bell before the report: %v", got)
	}
	if got := w.pushTo(under); !slices.Equal(got, sortedCopy("comment:"+o.id, "reply:"+cw.id)) {
		t.Fatalf("the reply's push before the report: %v", got)
	}
	if got := w.pushTo(rude); !slices.Equal(got, []string{"comment:" + o.id}) {
		t.Fatalf("the comment's push before the report: %v", got)
	}

	w.report(o, "comments", rude)
	if got := w.pushTo(rude); len(got) != 0 {
		t.Fatalf("a push to O about the comment O reported: %v", got)
	}

	if ids, n := w.commentIDs(o, entry); !slices.Equal(ids, []string{kind}) || n != 1 {
		t.Fatalf("the post as O: %v, count %v, want only Vera's comment", ids, n)
	}
	if n := w.boardCount(o, entry); n != 1 {
		t.Fatalf("O's Board count: %v, want 1", n)
	}
	if n := w.diary(o).comments[entry]; n != 1 {
		t.Fatalf("O's diary count: %v, want 1", n)
	}
	if got := w.bell(o); has(got, "comment:"+rude) || has(got, "comment:"+under) || !has(got, "comment:"+kind) {
		t.Fatalf("O's bell after the report: %v", got)
	}
	// O lost the comment, so a push about the reply under it goes to its
	// writer only, and a new answer under it to no one.
	if got := w.pushTo(under); !slices.Equal(got, []string{"reply:" + cw.id}) {
		t.Fatalf("the reply's push after the report: %v", got)
	}
	again := w.reply(cw, entry, "Still here", rude)["id"].(string)
	if got := w.pushTo(again); len(got) != 0 {
		t.Fatalf("a push about an answer under the comment O reported: %v", got)
	}
	answer := w.reply(cw, entry, "Thanks", kind)["id"].(string)
	if got := w.pushTo(answer); !slices.Equal(got, sortedCopy("comment:"+o.id, "reply:"+v.id)) {
		t.Fatalf("an answer under Vera's comment: %v", got)
	}
	if ids, n := w.commentIDs(o, entry); !slices.Equal(ids, []string{kind, answer}) || n != 2 {
		t.Fatalf("the post as O at the end: %v, count %v", ids, n)
	}
	thread := []string{rude, under, again, kind, answer}
	for _, p := range []feedPerson{cw, y, v} {
		if ids, n := w.commentIDs(p, entry); !slices.Equal(ids, thread) || n != 5 {
			t.Fatalf("the post as %s: %v, count %v, want all of %v", p.name, ids, n, thread)
		}
	}

	// Wren reports Yuri's reply under Wren's own comment: it leaves Wren's
	// bell and Wren's page, and nobody else's.
	if got := w.bell(cw); !has(got, "reply:"+under) {
		t.Fatalf("Wren's bell before: %v", got)
	}
	w.report(cw, "comments", under)
	if got := w.bell(cw); has(got, "reply:"+under) {
		t.Fatalf("Wren's bell after reporting the reply: %v", got)
	}
	if got := w.pushTo(under); len(got) != 0 {
		t.Fatalf("a push to Wren about the reply Wren reported: %v", got)
	}
	if ids, n := w.commentIDs(cw, entry); !slices.Equal(ids, []string{rude, again, kind, answer}) || n != 4 {
		t.Fatalf("the post as Wren: %v, count %v", ids, n)
	}
	if ids, n := w.commentIDs(v, entry); !slices.Equal(ids, thread) || n != 5 {
		t.Fatalf("the post as Vera after Wren's report: %v, count %v", ids, n)
	}
}

// A post with three open reports is hidden while under review from everyone
// but its owner, who still sees it, flagged. Two reports are not enough, and
// neither a dismissed report nor one from a gone account (disabled, or being
// deleted) counts. The count is live: the post is hidden the moment a
// reporter is enabled again, and back the moment one report is dismissed.
func TestModerationIntegration_APostUnderReviewIsHiddenFromAllButItsOwner(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mu_olive", "Olive Owner")
	v := w.person("mu_vera", "Vera Viewer")
	l := w.person("mu_lou", "Lou Locked")
	gale := w.person("mu_gale", "Gale Disabled")
	dell := w.person("mu_dell", "Dell Deleting")
	r1 := w.person("mu_rhea", "Rhea Reporter")
	r2 := w.person("mu_remy", "Remy Reporter")
	r3 := w.person("mu_rose", "Rose Reporter")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{v, gale, dell, r1, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	// Lou follows O but has not posted today: Lou's Board is the locked card.
	w.followAccepted(l, o)
	vComment := w.reply(v, entry, "Nice", "")["id"].(string)
	answer := w.reply(o, entry, "Thanks", vComment)["id"].(string)

	shown := func(label string, want bool) {
		t.Helper()
		if w.seesPost(v, entry) != want || w.onBoard(v, entry) != want {
			t.Fatalf("%s: Vera sees the post: %v, want %v", label, !want, want)
		}
		if ids, posts := w.grid(v, o); has(ids, entry) != want || (posts == 1) != want {
			t.Fatalf("%s: O's page as Vera: grid %v, Posts %v", label, ids, posts)
		}
		if names, n := w.locked(l); slices.Contains(names, "Olive") != want || (n == 1) != want {
			t.Fatalf("%s: Lou's locked card: %v, %v posted", label, names, n)
		}
		if got := w.ownFlags(o, entry); !slices.Equal(got, []bool{!want, !want, !want, !want, !want}) {
			t.Fatalf("%s: the owner's under review flags %v, want all %v", label, got, !want)
		}
	}
	shown("before any report", true)
	if got := w.bell(v); !has(got, "reply:"+answer) {
		t.Fatalf("Vera's bell before: %v", got)
	}

	// Two people report and then go: their reports stay open but do not count.
	w.report(gale, "posts", entry)
	w.report(dell, "posts", entry)
	w.sql(`update users set disabled_at = now() where id = $1`, gale.id)
	w.sql(`update users set deletion_requested_at = now() where id = $1`, dell.id)
	// One report dismissed, and two open.
	w.report(r1, "posts", entry)
	w.sql(`update post_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r1.id)
	w.report(r2, "posts", entry)
	w.report(r3, "posts", entry)
	shown("two open reports, one dismissed and two from gone accounts", true)

	// Gale is enabled again, so three count: hidden from all but its owner.
	w.sql(`update users set disabled_at = null where id = $1`, gale.id)
	shown("three open reports", false)
	w.call(v, http.MethodPost, "/v1/posts/"+entry+"/report", map[string]any{"reason": "spam"}, http.StatusNotFound)
	w.call(v, http.MethodPost, "/v1/posts/"+entry+"/reaction", map[string]any{"reaction": "love"}, http.StatusNotFound)
	w.call(v, http.MethodPost, "/v1/posts/"+entry+"/comments", map[string]any{"text": "Hello?"}, http.StatusNotFound)
	if got := w.bell(v); has(got, "reply:"+answer) {
		t.Fatalf("Vera's bell while it is under review: %v", got)
	}
	later := w.reply(o, entry, "Still here", vComment)["id"].(string)
	if got := w.pushTo(later); len(got) != 0 {
		t.Fatalf("a push to Vera about a post under review: %v", got)
	}
	// Its owner keeps the post, their own entry, Today and the bell.
	if !w.seesPost(o, entry) || !w.onBoard(o, entry) {
		t.Fatal("the owner lost the post under review")
	}
	w.ok(o, http.MethodGet, "/v1/entries/"+entry, nil)
	if got := dig(t, w.ok(o, http.MethodGet, "/v1/today", nil), "entry", "id"); got != entry {
		t.Fatalf("the owner's Today entry: %v", got)
	}
	if got := w.bell(o); !has(got, "comment:"+vComment) {
		t.Fatalf("the owner's bell: %v", got)
	}

	// Remy's report is dismissed: two count, and it is back for everyone.
	w.sql(`update post_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r2.id)
	shown("after a dismissal", true)
	if got := w.bell(v); !has(got, "reply:"+answer) || !has(got, "reply:"+later) {
		t.Fatalf("Vera's bell after the dismissal: %v", got)
	}
}

// A comment with three open reports is hidden while under review from
// everyone but its writer, who still sees it, flagged, and the replies under
// it go with it, for the reply's own writer too. The post's owner gets no
// exception. A gone reporter does not count, and one dismissal brings it
// back.
func TestModerationIntegration_ACommentUnderReviewIsShownOnlyToItsWriter(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mv_olive", "Olive Owner")
	cw := w.person("mv_wren", "Wren Writer")
	y := w.person("mv_yuri", "Yuri Replier")
	v := w.person("mv_vera", "Vera Viewer")
	gale := w.person("mv_gale", "Gale Gone")
	r1 := w.person("mv_rhea", "Rhea Reporter")
	r2 := w.person("mv_remy", "Remy Reporter")
	r3 := w.person("mv_rose", "Rose Reporter")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, y, v, gale, r1, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	rude := w.reply(cw, entry, "Rude", "")["id"].(string)
	under := w.reply(y, entry, "Agreed", rude)["id"].(string)
	kind := w.reply(v, entry, "Lovely", "")["id"].(string)

	w.report(gale, "comments", rude)
	w.sql(`update users set deletion_requested_at = now() where id = $1`, gale.id)
	w.report(r1, "comments", rude)
	w.report(r2, "comments", rude)
	// Two open reports that count are not enough.
	for _, p := range []feedPerson{v, o, y} {
		if ids, n := w.commentIDs(p, entry); !slices.Equal(ids, []string{rude, under, kind}) || n != 3 {
			t.Fatalf("two reports, the post as %s: %v, count %v", p.name, ids, n)
		}
	}
	w.report(r3, "comments", rude)

	// Hidden from Vera, from the post's owner and from the reply's writer,
	// with the reply under it.
	for _, p := range []feedPerson{v, o, y} {
		if ids, n := w.commentIDs(p, entry); !slices.Equal(ids, []string{kind}) || n != 1 {
			t.Fatalf("under review, the post as %s: %v, count %v, want only Vera's comment", p.name, ids, n)
		}
		if n := w.boardCount(p, entry); n != 1 {
			t.Fatalf("under review, %s's Board count: %v", p.name, n)
		}
	}
	if n := w.diary(o).comments[entry]; n != 1 {
		t.Fatalf("O's diary count: %v, want 1", n)
	}
	if got := w.bell(o); has(got, "comment:"+rude) || has(got, "comment:"+under) || !has(got, "comment:"+kind) {
		t.Fatalf("O's bell: %v", got)
	}
	w.call(y, http.MethodPost, "/v1/posts/"+entry+"/comments", map[string]any{"text": "Me too", "parent_id": rude}, http.StatusNotFound)

	// Its writer still sees it all, and only their own comment is flagged.
	if ids, n := w.commentIDs(cw, entry); !slices.Equal(ids, []string{rude, under, kind}) || n != 3 {
		t.Fatalf("the post as its writer: %v, count %v", ids, n)
	}
	if n := w.boardCount(cw, entry); n != 3 {
		t.Fatalf("the writer's Board count: %v", n)
	}
	post, err := w.boardSvc.Post(context.Background(), uuid.MustParse(cw.id), uuid.MustParse(entry))
	if err != nil {
		t.Fatalf("the writer's post page: %v", err)
	}
	flagged := map[string]bool{}
	for _, c := range post.Comments {
		flagged[c.ID.String()] = c.UnderReview
	}
	if !flagged[rude] || flagged[under] || flagged[kind] || len(flagged) != 3 {
		t.Fatalf("the writer's flags: %v, want only their own comment", flagged)
	}
	if got := w.bell(cw); !has(got, "reply:"+under) {
		t.Fatalf("the writer's bell: %v", got)
	}
	// The writer answers under it: no push to the post's owner, who cannot
	// see it.
	again := w.reply(cw, entry, "Still here", rude)["id"].(string)
	if got := w.pushTo(again); len(got) != 0 {
		t.Fatalf("a push about an answer under a comment under review: %v", got)
	}

	// Rhea's report is dismissed: two count, and it is back for everyone.
	w.sql(`update comment_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r1.id)
	for _, p := range []feedPerson{v, o, y} {
		if ids, n := w.commentIDs(p, entry); !slices.Equal(ids, []string{rude, under, again, kind}) || n != 4 {
			t.Fatalf("after the dismissal, the post as %s: %v, count %v", p.name, ids, n)
		}
	}
	post, err = w.boardSvc.Post(context.Background(), uuid.MustParse(cw.id), uuid.MustParse(entry))
	if err != nil || post.Comments[0].ID.String() != rude || post.Comments[0].UnderReview {
		t.Fatalf("the writer's comment after the dismissal: %+v, %v", post.Comments, err)
	}
}

// A post an admin removed is gone for everyone, its owner included, as if
// deleted: the post page, the owner's entry, archive, diary and Today, the
// Board, the person page, the locked card's names and count, the bell and
// the push. Its day still counts: the posted days, the week and its bonus,
// and the Board's unlock stay as they were, and no new post is allowed that
// day.
func TestModerationIntegration_ARemovedPostIsGoneForEveryoneAndItsDayStillCounts(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mx_olive", "Olive Owner")
	v := w.person("mx_vera", "Vera Viewer")
	l := w.person("mx_lou", "Lou Locked")
	// Monday to Thursday earn O the week's bonus. Monday and Tuesday are
	// archived, and Tuesday is removed too. Six equal wedges: Monday to
	// Wednesday land on Meal and Thursday on Movie, so the removed post has
	// a category of its own.
	w.rand.Queue(0, 0, 0, 1)
	monday := w.post(o, "2026-10-05")
	tuesday := w.post(o, "2026-10-06")
	wednesday := w.post(o, "2026-10-07")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, id := range []string{monday, tuesday} {
		w.ok(o, http.MethodPost, "/v1/entries/"+id+"/archive", map[string]any{})
	}
	w.followAccepted(v, o)
	w.post(v, day)
	w.followAccepted(l, o)
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	vComment := w.reply(v, entry, "Nice", "")["id"].(string)
	w.ok(v, http.MethodPost, "/v1/posts/"+entry+"/reaction", map[string]any{"reaction": "love"})
	vLike := "like:" + entry + ":" + v.id
	answer := w.reply(o, entry, "Thanks", vComment)["id"].(string)

	ctx := context.Background()
	daysAndWeeks := func() string {
		t.Helper()
		var s string
		if err := w.pool.QueryRow(ctx, `select
			(select string_agg(local_date::text || ' ' || source || ' ' || late::text, ', ' order by local_date)
			 from posted_days where user_id = $1) || ' | ' ||
			(select string_agg(week_start::text || ' ' || goal::text || ' ' || bonus_granted::text || ' ' || goal_met_at::text, ', ' order by week_start)
			 from goal_weeks where user_id = $1) || ' | ' ||
			(select bonus_held::text from user_spin_state where user_id = $1)`, o.id).Scan(&s); err != nil {
			t.Fatalf("the posted days and goal weeks: %v", err)
		}
		return s
	}
	tastes := func() float64 {
		t.Helper()
		var n float64
		for _, v := range w.ok(o, http.MethodGet, "/v1/me/summary", nil)["tastes"].([]any) {
			n += v.(map[string]any)["count"].(float64)
		}
		return n
	}
	recent := func() []string {
		t.Helper()
		ids := []string{}
		for _, v := range w.ok(o, http.MethodGet, "/v1/me/summary", nil)["recent"].([]any) {
			ids = append(ids, v.(map[string]any)["id"].(string))
		}
		return ids
	}
	archive := func() []string {
		t.Helper()
		ids := []string{}
		for _, v := range w.ok(o, http.MethodGet, "/v1/me/archive", nil)["entries"].([]any) {
			ids = append(ids, v.(map[string]any)["id"].(string))
		}
		return ids
	}

	before := w.ok(o, http.MethodGet, "/v1/today", nil)
	if before["state"] != string(spin.StatePosted) || dig(t, before, "entry", "id") != entry ||
		dig(t, before, "week", "posted") != float64(4) || dig(t, before, "bonus", "held") != true {
		t.Fatalf("Thursday before the removal: %v", before)
	}
	stored := daysAndWeeks()
	if got := archive(); !slices.Equal(got, []string{tuesday, monday}) {
		t.Fatalf("the archive before: %v", got)
	}
	if got := w.diary(o).entries; !slices.Equal(got, []string{entry, wednesday}) {
		t.Fatalf("the diary before: %v", got)
	}
	if got := recent(); !slices.Equal(got, []string{entry, wednesday}) || tastes() != 2 {
		t.Fatalf("the summary before: recent %v", got)
	}
	if ids, posts := w.grid(v, o); !slices.Equal(ids, []string{entry}) || posts != 2 {
		t.Fatalf("O's page as Vera before: %v, Posts %v", ids, posts)
	}
	if names, _ := w.locked(l); !slices.Equal(names, []string{"Olive"}) {
		t.Fatalf("Lou's locked card before: %v", names)
	}
	if got := w.bell(o); !has(got, "comment:"+vComment) || !has(got, vLike) {
		t.Fatalf("O's bell before: %v, want Vera's comment and like", got)
	}
	if got := w.bell(v); !has(got, "reply:"+answer) {
		t.Fatalf("Vera's bell before: %v", got)
	}
	if got := w.pushTo(answer); !slices.Equal(got, []string{"reply:" + v.id}) {
		t.Fatalf("the answer's push before: %v", got)
	}
	if got := w.pushTo(vComment); !slices.Equal(got, []string{"comment:" + o.id}) {
		t.Fatalf("Vera's comment's push before: %v", got)
	}
	if page := w.diary(o); page.category[entry] == page.category[wednesday] ||
		!slices.Equal(page.chips, sortedCopy(page.category[entry], page.category[wednesday])) {
		t.Fatalf("the diary's chips before: %+v", page)
	}

	w.sql(`update entries set removed_at = now() where id = $1`, entry)
	w.sql(`update entries set removed_at = now() where id = $1`, tuesday)

	// Gone for its owner.
	if w.seesPost(o, entry) || w.onBoard(o, entry) {
		t.Fatal("the owner still sees the removed post")
	}
	w.call(o, http.MethodGet, "/v1/entries/"+entry, nil, http.StatusNotFound)
	w.call(o, http.MethodGet, "/v1/entries/"+tuesday, nil, http.StatusNotFound)
	if got := archive(); !slices.Equal(got, []string{monday}) {
		t.Fatalf("the archive after: %v, want Monday's post alone", got)
	}
	page := w.diary(o)
	if !slices.Equal(page.entries, []string{wednesday}) || len(page.skipped) != 0 ||
		!slices.Equal(page.chips, []string{page.category[wednesday]}) {
		t.Fatalf("the diary after: %+v", page)
	}
	if got := recent(); !slices.Equal(got, []string{wednesday}) || tastes() != 1 {
		t.Fatalf("the summary after: recent %v", got)
	}
	if got := w.bell(o); has(got, "comment:"+vComment) || has(got, vLike) {
		t.Fatalf("O's bell after: %v, want neither Vera's comment nor her like", got)
	}
	if got := w.pushTo(vComment); len(got) != 0 {
		t.Fatalf("a push to O about a comment on the removed post: %v", got)
	}
	// The owner's photo writes find no entry either (the app reads the
	// entry first, so only the repository shows it).
	stop := errors.New("roll back")
	thursday, err := spin.ParseDate(day)
	if err != nil {
		t.Fatal(err)
	}
	seed := spin.Seed{At: time.Now(), LocalDate: thursday, TZName: feedZone, Goal: spin.DefaultWeeklyGoal}
	err = postgres.NewSpinRepo(w.pool).InUserTx(ctx, o.id, seed, func(tx spin.Tx) error {
		photo := spin.NewStoredPhoto{Position: 1, Key: "entries/" + o.id + "/0123456789abcdef.jpg",
			ContentType: "image/jpeg", Source: spin.PhotoSourceLibrary}
		if err := tx.InsertPhotos(entry, []spin.NewStoredPhoto{photo}, time.Now()); !errors.Is(err, spin.ErrEntryNotFound) {
			t.Errorf("adding a photo to the removed post: %v, want ErrEntryNotFound", err)
		}
		if _, err := tx.ReplacePhotos(entry, nil, time.Now()); !errors.Is(err, spin.ErrEntryNotFound) {
			t.Errorf("replacing the removed post's photos: %v, want ErrEntryNotFound", err)
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("the photo writes: %v", err)
	}

	// Its day still counts: Today is posted with no entry, the week and the
	// bonus are as they were, the Board stays open, and the day is settled.
	after := w.ok(o, http.MethodGet, "/v1/today", nil)
	if after["state"] != before["state"] || after["entry"] != nil ||
		!reflect.DeepEqual(after["week"], before["week"]) || !reflect.DeepEqual(after["bonus"], before["bonus"]) {
		t.Fatalf("Thursday after the removal: %v\nbefore: %v", after, before)
	}
	if got := daysAndWeeks(); got != stored {
		t.Fatalf("the posted days and goal weeks changed: %s, want %s", got, stored)
	}
	if b := w.board(o, ""); b["state"] != board.StateUnlocked {
		t.Fatalf("the owner's Board: %v", b["state"])
	}
	ticket := w.ok(o, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000})
	settled := w.call(o, http.MethodPost, "/v1/entries", entryRequest(day, 0, ticket["photo_key"].(string)), http.StatusConflict)
	if settled["error"] != string(CodeDaySettled) {
		t.Fatalf("a new post on the removed post's day: %v", settled)
	}

	// Gone for everyone else.
	if w.seesPost(v, entry) || w.onBoard(v, entry) {
		t.Fatal("Vera still sees the removed post")
	}
	if ids, posts := w.grid(v, o); len(ids) != 0 || posts != 1 {
		t.Fatalf("O's page as Vera after: %v, Posts %v, want Wednesday's post counted alone", ids, posts)
	}
	if names, n := w.locked(l); len(names) != 0 || n != 0 {
		t.Fatalf("Lou's locked card after: %v, %v posted, want no name and no count", names, n)
	}
	if got := w.bell(v); has(got, "reply:"+answer) {
		t.Fatalf("Vera's bell after: %v", got)
	}
	if got := w.pushTo(answer); len(got) != 0 {
		t.Fatalf("a push about a removed post: %v", got)
	}
}

// A comment an admin removed is gone for everyone, its writer included, and
// the replies under it go with it: the post page and the counts, the diary,
// the bell and the push. No one can answer under it any more.
func TestModerationIntegration_ARemovedCommentIsGoneForEveryoneWithItsReplies(t *testing.T) {
	w := newModWorld(t)
	o := w.person("my_olive", "Olive Owner")
	cw := w.person("my_wren", "Wren Writer")
	y := w.person("my_yuri", "Yuri Replier")
	v := w.person("my_vera", "Vera Viewer")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, y, v} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	rude := w.reply(cw, entry, "Rude", "")["id"].(string)
	under := w.reply(y, entry, "Agreed", rude)["id"].(string)
	kind := w.reply(v, entry, "Lovely", "")["id"].(string)
	if got := w.bell(o); !has(got, "comment:"+rude) || !has(got, "comment:"+under) {
		t.Fatalf("O's bell before: %v", got)
	}
	if got := w.bell(cw); !has(got, "reply:"+under) {
		t.Fatalf("Wren's bell before: %v", got)
	}
	if got := w.pushTo(under); !slices.Equal(got, sortedCopy("comment:"+o.id, "reply:"+cw.id)) {
		t.Fatalf("the reply's push before: %v", got)
	}

	w.sql(`update post_comments set removed_at = now() where id = $1`, rude)

	for _, p := range []feedPerson{o, cw, y, v} {
		if ids, n := w.commentIDs(p, entry); !slices.Equal(ids, []string{kind}) || n != 1 {
			t.Fatalf("the post as %s: %v, count %v, want only Vera's comment", p.name, ids, n)
		}
		if n := w.boardCount(p, entry); n != 1 {
			t.Fatalf("%s's Board count: %v", p.name, n)
		}
	}
	if n := w.diary(o).comments[entry]; n != 1 {
		t.Fatalf("O's diary count: %v, want 1", n)
	}
	if got := w.bell(o); has(got, "comment:"+rude) || has(got, "comment:"+under) || !has(got, "comment:"+kind) {
		t.Fatalf("O's bell after: %v", got)
	}
	if got := w.bell(cw); has(got, "reply:"+under) {
		t.Fatalf("Wren's bell after: %v", got)
	}
	if got := w.pushTo(under); len(got) != 0 {
		t.Fatalf("a push about a reply under a removed comment: %v", got)
	}
	for _, p := range []feedPerson{cw, y} {
		for _, parent := range []string{rude, under} {
			w.call(p, http.MethodPost, "/v1/posts/"+entry+"/comments", map[string]any{"text": "Hello?", "parent_id": parent}, http.StatusNotFound)
		}
	}
}
