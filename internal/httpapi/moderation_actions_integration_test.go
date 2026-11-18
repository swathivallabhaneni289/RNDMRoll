package httpapi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// Moderation's writes and what the owner is told (migration 0020). A person
// reports a post or a comment once: a repeat is answered exactly like a miss
// and changes nothing, and the report notifier hears only a report that was
// kept. A comment report is checked against the post page's own list, so a
// reply goes with the comment it answers. What an admin removed is out of
// its owner's reach (edit, archive, delete, hide), while a post or comment
// under review keeps every action. Today says when the day's post was
// removed. The under review flag goes to the owner or the writer only.

// aMiss sends p's request and wants exactly the answer to the same request
// about something that is not there: the path with id swapped for a made-up
// one.
func aMiss(w *modWorld, label string, p feedPerson, method, path, id string, body any) {
	w.t.Helper()
	missCode, missBody := socialSend(w.feedWorld, p, method, strings.Replace(path, id, uuid.NewString(), 1), body)
	code, got := socialSend(w.feedWorld, p, method, path, body)
	if missCode != http.StatusNotFound || code != missCode || got != missBody {
		w.t.Fatalf("%s: %d %s, want the miss %d %s", label, code, got, missCode, missBody)
	}
}

// stored reads one line of text straight from the database.
func (w *modWorld) stored(q string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), q, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return s
}

// underReview is the under_review key of an answer's object: true, false, or
// "absent" when the key is not there.
func underReview(m any) any {
	v, present := m.(map[string]any)["under_review"]
	if !present {
		return "absent"
	}
	return v
}

// A repeat report of a post or a comment, with another reason, is answered
// exactly like a miss, also once the first was dismissed, and changes
// nothing: the first reason and the title kept with it stay, a dismissed
// report stays closed, and the report notifier hears only the first.
func TestModerationIntegration_ARepeatReportIsAMissAndChangesNothing(t *testing.T) {
	heard := &heardReports{}
	w := newModWorldWith(t, func(b *board.Service) { b.SetReportNotifier(heard) })
	o := w.person("ma_olive", "Olive Owner")
	r := w.person("ma_rhea", "Rhea Reporter")
	cw := w.person("ma_wren", "Wren Writer")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{r, cw} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	comment := w.reply(cw, entry, "Rude", "")["id"].(string)
	title := entryObject(t, w.ok(o, http.MethodGet, "/v1/entries/"+entry, nil)["entry"])["title"].(string)
	postPath, commentPath := "/v1/posts/"+entry+"/report", "/v1/comments/"+comment+"/report"

	w.ok(r, http.MethodPost, commentPath, map[string]any{"reason": "harassment"})
	w.ok(r, http.MethodPost, postPath, map[string]any{"reason": "spam"})
	// The owner renames the post after the report.
	w.ok(o, http.MethodPatch, "/v1/entries/"+entry, map[string]any{"title": "Dune, again"})
	reports := func() string {
		t.Helper()
		return w.stored(`select
			(select reason || ' ' || title || ' ' || coalesce(resolution, 'open') from post_reports where reporter_id = $1) || ' | ' ||
			(select reason || ' ' || body || ' ' || coalesce(resolution, 'open') from comment_reports where reporter_id = $1)`, r.id)
	}
	first := reports()
	if first != "spam "+title+" open | harassment Rude open" {
		t.Fatalf("the reports kept: %s", first)
	}

	aMiss(w, "the post again", r, http.MethodPost, postPath, entry, map[string]any{"reason": "violence"})
	aMiss(w, "the comment again", r, http.MethodPost, commentPath, comment, map[string]any{"reason": "nudity"})
	if got := reports(); got != first {
		t.Fatalf("a repeat changed the reports: %s, want %s", got, first)
	}

	// Dismissed: a repeat still finds the post and the comment gone for good,
	// and opens nothing.
	w.sql(`update post_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r.id)
	w.sql(`update comment_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r.id)
	aMiss(w, "the post again, once dismissed", r, http.MethodPost, postPath, entry, map[string]any{"reason": "hate"})
	aMiss(w, "the comment again, once dismissed", r, http.MethodPost, commentPath, comment, map[string]any{"reason": "hate"})
	if got := reports(); got != "spam "+title+" dismissed | harassment Rude dismissed" {
		t.Fatalf("a repeat after the dismissal: %s", got)
	}
	if got := heard.got(); !slices.Equal(got, []string{"harassment", "spam"}) {
		t.Fatalf("heard %v, want the two first reports only", got)
	}
}

// racedReads is the Postgres board repository with its two visibility reads
// frozen as they were before any report was made: two reports that race
// both pass the service's check, and only the store can tell them apart.
type racedReads struct {
	*postgres.BoardRepo
	entry    board.EntryRow
	comments []board.CommentRow
}

func (r *racedReads) EntryFor(context.Context, uuid.UUID, uuid.UUID) (board.EntryRow, error) {
	return r.entry, nil
}

func (r *racedReads) Comments(context.Context, uuid.UUID, uuid.UUID) ([]board.CommentRow, error) {
	return r.comments, nil
}

// Of two reports of the same post, or the same comment, by one person that
// race past the visibility check, the store keeps the first and the second
// is a miss that the report notifier never hears. The store refuses a repeat
// on its own, whatever became of the first report, and a report of a post
// or comment that is gone.
func TestModerationIntegration_TwoReportsThatRaceAreKeptOnce(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mk_olive", "Olive Owner")
	r := w.person("mk_rhea", "Rhea Reporter")
	cw := w.person("mk_wren", "Wren Writer")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{r, cw} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	comment := w.reply(cw, entry, "Rude", "")["id"].(string)

	ctx := context.Background()
	repo := postgres.NewBoardRepo(w.pool)
	viewer, eid, cid := uuid.MustParse(r.id), uuid.MustParse(entry), uuid.MustParse(comment)
	row, err := repo.EntryFor(ctx, viewer, eid)
	if err != nil {
		t.Fatalf("the post as Rhea: %v", err)
	}
	comments, err := repo.Comments(ctx, viewer, eid)
	if err != nil || len(comments) != 1 {
		t.Fatalf("the comments as Rhea: %v, %v", comments, err)
	}
	heard := &heardReports{}
	svc := board.NewService(&racedReads{BoardRepo: repo, entry: row, comments: comments},
		social.NewService(postgres.NewSocialRepo(w.pool), w.clk), w.spinSvc, w.clk, nil)
	svc.SetReportNotifier(heard)

	if err := svc.ReportPost(ctx, viewer, eid, "spam"); err != nil {
		t.Fatalf("the first post report: %v", err)
	}
	if err := svc.ReportPost(ctx, viewer, eid, "violence"); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("the racing post report: %v, want ErrNotFound", err)
	}
	if err := svc.ReportComment(ctx, viewer, cid, "harassment"); err != nil {
		t.Fatalf("the first comment report: %v", err)
	}
	if err := svc.ReportComment(ctx, viewer, cid, "nudity"); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("the racing comment report: %v, want ErrNotFound", err)
	}
	if got := heard.got(); !slices.Equal(got, []string{"spam", "harassment"}) {
		t.Fatalf("heard %v, want each first report once", got)
	}
	reports := func() string {
		t.Helper()
		return w.stored(`select
			(select reason || ' ' || coalesce(resolution, 'open') from post_reports where reporter_id = $1) || ' | ' ||
			(select reason || ' ' || body || ' ' || coalesce(resolution, 'open') from comment_reports where reporter_id = $1)`, r.id)
	}
	if got := reports(); got != "spam open | harassment Rude open" {
		t.Fatalf("the reports kept: %s", got)
	}

	// The store alone, once both were dismissed.
	w.sql(`update post_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r.id)
	w.sql(`update comment_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r.id)
	owner, writer := uuid.MustParse(o.id), uuid.MustParse(cw.id)
	if _, err := repo.ReportPost(ctx, viewer, eid, owner, "Another title", "hate", time.Now(), time.Now()); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("the store's repeat post report: %v, want ErrNotFound", err)
	}
	if _, err := repo.ReportComment(ctx, viewer, cid, writer, "Other words", "hate", time.Now(), time.Now()); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("the store's repeat comment report: %v, want ErrNotFound", err)
	}
	if got := reports(); got != "spam dismissed | harassment Rude dismissed" {
		t.Fatalf("the reports after the store's repeats: %s", got)
	}
	if _, err := repo.ReportPost(ctx, viewer, uuid.New(), owner, "Gone", "spam", time.Now(), time.Now()); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("a report of a post that is gone: %v, want ErrNotFound", err)
	}
	if _, err := repo.ReportComment(ctx, viewer, uuid.New(), writer, "Gone", "spam", time.Now(), time.Now()); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("a report of a comment that is gone: %v, want ErrNotFound", err)
	}
}

// ownClocks is a clock on which the accounts in own read their own time and
// every other account reads base's.
type ownClocks struct {
	base clock.Clock
	own  map[string]time.Time
}

func (c ownClocks) NowFor(id string) time.Time {
	if t, ok := c.own[id]; ok {
		return t
	}
	return c.base.NowFor(id)
}

// The report that hides a post or comment logs it for its owner (the bell's
// "hidden while we take a look"), on the owner's own clock and not the
// reporter's, since the bell reads the log by the owner's clock. The row
// names the post, for a comment its post, and keeps the title or the words.
// Fewer reports log nothing, and so does one past the line: a report that
// raced the third, here the store alone.
func TestModerationIntegration_TheReportThatHidesItIsLoggedOnTheOwnersClock(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mo_olive", "Olive Owner")
	cw := w.person("mo_wren", "Wren Writer")
	rs := []feedPerson{w.person("mo_rhea", "Rhea One"), w.person("mo_remy", "Remy Two"), w.person("mo_rose", "Rose Three"),
		w.person("mo_rory", "Rory Four")}
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range append([]feedPerson{cw}, rs...) {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	comment := w.reply(cw, entry, "Rude words", "")["id"].(string)

	ctx := context.Background()
	repo := postgres.NewBoardRepo(w.pool)
	oliveNow := time.Date(2026, 10, 9, 9, 30, 0, 0, time.UTC)
	wrenNow := time.Date(2026, 10, 10, 21, 15, 0, 0, time.UTC)
	svc := board.NewService(repo, social.NewService(postgres.NewSocialRepo(w.pool), w.clk), w.spinSvc,
		ownClocks{base: w.clk, own: map[string]time.Time{o.id: oliveNow, cw.id: wrenNow}}, nil)
	hides := func() string {
		t.Helper()
		return w.stored(`select coalesce(string_agg(item || ' ' || item_id || ' on ' || coalesce(entry_id::text, '-') || ' ' ||
			coalesce(comment_id::text, '-') || ' ' || snapshot || ' ' || cardinality(reasons) || ' ' ||
			to_char(at at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), ' | ' order by created_at, id), '')
			from moderation_events where action = 'hidden' and owner_id in ($1, $2)`, o.id, cw.id)
	}
	eid, cid := uuid.MustParse(entry), uuid.MustParse(comment)
	for _, r := range rs[:2] {
		if err := svc.ReportComment(ctx, uuid.MustParse(r.id), cid, "spam"); err != nil {
			t.Fatalf("a comment report: %v", err)
		}
		if err := svc.ReportPost(ctx, uuid.MustParse(r.id), eid, "spam"); err != nil {
			t.Fatalf("a post report: %v", err)
		}
	}
	if got := hides(); got != "" {
		t.Fatalf("two reports each logged %q, want nothing", got)
	}
	third := uuid.MustParse(rs[2].id)
	if err := svc.ReportComment(ctx, third, cid, "spam"); err != nil {
		t.Fatalf("the third comment report: %v", err)
	}
	if err := svc.ReportPost(ctx, third, eid, "spam"); err != nil {
		t.Fatalf("the third post report: %v", err)
	}
	want := "comment " + comment + " on " + entry + " " + comment + " Rude words 0 2026-10-10 21:15 | " +
		"post " + entry + " on " + entry + " - Dune 0 2026-10-09 09:30"
	if got := hides(); got != want {
		t.Fatalf("the hides logged:\n%s\nwant\n%s", got, want)
	}
	reportersNow := w.clk.NowFor("").UTC().Format("2006-01-02 15:04")
	if got := w.stored(`select to_char(max(created_at) at time zone 'UTC', 'YYYY-MM-DD HH24:MI') from post_reports where entry_id = $1`, entry); got != reportersNow {
		t.Fatalf("the reports were made at %s, want the reporters' own clock (%s)", got, reportersNow)
	}

	// One past the line, as a report that raced the third would land.
	fourth := uuid.MustParse(rs[3].id)
	if hid, err := repo.ReportPost(ctx, fourth, eid, uuid.MustParse(o.id), "Dune", "spam", time.Now(), time.Now()); err != nil || hid {
		t.Fatalf("a fourth post report: hid %v, %v", hid, err)
	}
	if hid, err := repo.ReportComment(ctx, fourth, cid, uuid.MustParse(cw.id), "Rude words", "spam", time.Now(), time.Now()); err != nil || hid {
		t.Fatalf("a fourth comment report: hid %v, %v", hid, err)
	}
	if got := hides(); got != want {
		t.Fatalf("after a fourth report each:\n%s\nwant\n%s", got, want)
	}
}

// A reply is reported only while the post page lists it, so it goes with the
// comment it answers: the reply's report is a miss for someone the
// top-level comment is kept from (the post's owner hid it, they reported it,
// it is under review, its writer blocked them, or an admin removed it).
func TestModerationIntegration_AReplyReportChecksTheCommentItAnswers(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mp_olive", "Olive Owner")
	cw := w.person("mp_wren", "Wren Writer")
	y := w.person("mp_yuri", "Yuri Replier")
	v := w.person("mp_vera", "Vera Viewer")
	u := w.person("mp_umi", "Umi Viewer")
	k := w.person("mp_kit", "Kit Keeper")
	r2 := w.person("mp_remy", "Remy Reporter")
	r3 := w.person("mp_rose", "Rose Reporter")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, y, v, u, k, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	top := w.reply(cw, entry, "Rude", "")["id"].(string)
	reply := w.reply(y, entry, "Agreed", top)["id"].(string)
	path := "/v1/comments/" + reply + "/report"
	spam := map[string]any{"reason": "spam"}

	// Under a comment everyone sees, the reply's report is kept.
	w.ok(k, http.MethodPost, path, spam)

	w.ok(o, http.MethodPost, "/v1/comments/"+top+"/hide", map[string]any{})
	aMiss(w, "under a comment the post's owner hid", v, http.MethodPost, path, reply, spam)
	w.ok(o, http.MethodPost, "/v1/comments/"+top+"/unhide", map[string]any{})

	w.report(v, "comments", top)
	aMiss(w, "under a comment she reported", v, http.MethodPost, path, reply, spam)

	w.report(r2, "comments", top)
	w.report(r3, "comments", top)
	aMiss(w, "under a comment under review", u, http.MethodPost, path, reply, spam)

	// One report dismissed, it is back for Umi, but its writer blocks her.
	w.sql(`update comment_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r2.id)
	if ids, _ := w.commentIDs(u, entry); !slices.Equal(ids, []string{top, reply}) {
		t.Fatalf("the post as Umi after the dismissal: %v", ids)
	}
	w.ok(cw, http.MethodPost, "/v1/me/blocks", map[string]any{"user_id": u.id})
	aMiss(w, "under a comment whose writer blocked her", u, http.MethodPost, path, reply, spam)
	w.call(cw, http.MethodDelete, "/v1/me/blocks/"+u.id, nil, http.StatusNoContent)

	w.sql(`update post_comments set removed_at = now() where id = $1`, top)
	aMiss(w, "under a removed comment", u, http.MethodPost, path, reply, spam)

	if got := w.stored(`select string_agg(reporter_id::text, ',') from comment_reports where comment_id = $1`, reply); got != k.id {
		t.Fatalf("the reply's reporters: %s, want Kit alone", got)
	}
}

// What an admin removed is out of its owner's reach: an edit, an archive or
// unarchive and a delete of the post are answered like a miss and change
// nothing, so it is still there to bring back. A post under review keeps
// every action.
func TestModerationIntegration_ARemovedPostIsOutOfItsOwnersReach(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mo_olive", "Olive Owner")
	r1 := w.person("mo_rhea", "Rhea Reporter")
	r2 := w.person("mo_remy", "Remy Reporter")
	r3 := w.person("mo_rose", "Rose Reporter")
	gone := w.post(o, "2026-10-07")
	const day = "2026-10-08"
	held := w.post(o, day)
	for _, p := range []feedPerson{r1, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// It was archived before an admin removed it.
	w.ok(o, http.MethodPost, "/v1/entries/"+gone+"/archive", map[string]any{})
	w.sql(`update entries set removed_at = now() where id = $1`, gone)
	row := func() string {
		t.Helper()
		return w.stored(`select title || ' archived ' || (archived_at is not null)::text || ' removed ' || (removed_at is not null)::text ||
			' comments_off ' || comments_off::text from entries where id = $1`, gone)
	}
	before := row()
	if !strings.HasSuffix(before, " archived true removed true comments_off false") {
		t.Fatalf("the removed post: %s", before)
	}

	for _, a := range []struct {
		label, method, path string
		body                any
	}{
		{"an edit", http.MethodPatch, "/v1/entries/" + gone, map[string]any{"title": "Mine again"}},
		{"comments off", http.MethodPatch, "/v1/entries/" + gone, map[string]any{"comments_off": true}},
		{"an unarchive", http.MethodPost, "/v1/entries/" + gone + "/unarchive", map[string]any{}},
		{"an archive", http.MethodPost, "/v1/entries/" + gone + "/archive", map[string]any{}},
		{"a delete", http.MethodPost, "/v1/entries/" + gone + "/delete", map[string]any{}},
	} {
		aMiss(w, a.label+" of a removed post", o, a.method, a.path, gone, a.body)
	}
	if got := row(); got != before {
		t.Fatalf("the removed post changed: %s, want %s", got, before)
	}

	// The writes refuse it themselves, for a post removed after the owner's
	// read.
	ctx := context.Background()
	stop := errors.New("roll back")
	thursday, err := spin.ParseDate(day)
	if err != nil {
		t.Fatal(err)
	}
	seed := spin.Seed{At: time.Now(), LocalDate: thursday, TZName: feedZone, Goal: spin.DefaultWeeklyGoal}
	err = postgres.NewSpinRepo(w.pool).InUserTx(ctx, o.id, seed, func(tx spin.Tx) error {
		if err := tx.UpdateEntry(gone, spin.EntryUpdate{Title: "Mine again", Stars: 5}, time.Now()); !errors.Is(err, spin.ErrEntryNotFound) {
			t.Errorf("UpdateEntry of the removed post: %v, want ErrEntryNotFound", err)
		}
		for _, archived := range []bool{false, true} {
			if err := tx.SetEntryArchived(gone, archived, time.Now()); !errors.Is(err, spin.ErrEntryNotFound) {
				t.Errorf("SetEntryArchived(%v) of the removed post: %v, want ErrEntryNotFound", archived, err)
			}
		}
		if _, err := tx.DeleteEntry(gone); !errors.Is(err, spin.ErrEntryNotFound) {
			t.Errorf("DeleteEntry of the removed post: %v, want ErrEntryNotFound", err)
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("the writes: %v", err)
	}

	// Under review, its owner edits, archives, unarchives and deletes it.
	for _, p := range []feedPerson{r1, r2, r3} {
		w.report(p, "posts", held)
	}
	if w.seesPost(r1, held) {
		t.Fatal("a reporter still sees the post")
	}
	edited := entryObject(t, w.ok(o, http.MethodPatch, "/v1/entries/"+held, map[string]any{"title": "Still mine"})["entry"])
	if edited["title"] != "Still mine" || edited["under_review"] != true {
		t.Fatalf("the edit of the post under review: %v", edited)
	}
	w.ok(o, http.MethodPatch, "/v1/entries/"+held, map[string]any{"comments_off": true})
	w.ok(o, http.MethodPost, "/v1/entries/"+held+"/archive", map[string]any{})
	w.ok(o, http.MethodPost, "/v1/entries/"+held+"/unarchive", map[string]any{})
	w.ok(o, http.MethodPost, "/v1/entries/"+held+"/delete", map[string]any{})
	w.call(o, http.MethodGet, "/v1/entries/"+held, nil, http.StatusNotFound)
}

// A comment an admin removed is out of reach too, and so is a reply under it
// and a comment on a removed post: its writer's delete and the post owner's
// delete, hide and unhide are answered like a miss and change nothing. A
// comment under review keeps every action: the post's owner hides and shows
// it, and its writer deletes it.
func TestModerationIntegration_ARemovedCommentIsOutOfReach(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mq_olive", "Olive Owner")
	cw := w.person("mq_wren", "Wren Writer")
	y := w.person("mq_yuri", "Yuri Replier")
	v := w.person("mq_vera", "Vera Viewer")
	q := w.person("mq_quinn", "Quinn Writer")
	r1 := w.person("mq_rhea", "Rhea Reporter")
	r2 := w.person("mq_remy", "Remy Reporter")
	r3 := w.person("mq_rose", "Rose Reporter")
	old := w.post(o, "2026-10-07")
	w.followAccepted(v, o)
	w.post(v, "2026-10-07")
	onOld := w.reply(v, old, "From yesterday", "")["id"].(string)
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, y, q, r1, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	rude := w.reply(cw, entry, "Rude", "")["id"].(string)
	under := w.reply(y, entry, "Agreed", rude)["id"].(string)
	held := w.reply(q, entry, "Hmm", "")["id"].(string)

	w.sql(`update post_comments set removed_at = now() where id = $1`, rude)
	w.sql(`update entries set removed_at = now() where id = $1`, old)
	for _, a := range []struct {
		label  string
		p      feedPerson
		id, do string
	}{
		{"its writer's delete", cw, rude, "delete"},
		{"the post owner's delete", o, rude, "delete"},
		{"the post owner's hide", o, rude, "hide"},
		{"the post owner's unhide", o, rude, "unhide"},
		{"the delete of a reply under it", y, under, "delete"},
		{"the post owner's hide of that reply", o, under, "hide"},
		{"the delete of a comment on a removed post", v, onOld, "delete"},
		{"the post owner's hide of it", o, onOld, "hide"},
	} {
		aMiss(w, a.label, a.p, http.MethodPost, "/v1/comments/"+a.id+"/"+a.do, a.id, map[string]any{})
	}
	if got := w.stored(`select count(*)::text from post_comments where id = any($1::uuid[]) and hidden_at is null`,
		[]string{rude, under, onOld}); got != "3" {
		t.Fatalf("%s of the three comments are still there and shown, want all", got)
	}
	// The store keeps a removed comment from a hide that slips past the read.
	at := time.Now()
	if err := postgres.NewBoardRepo(w.pool).SetCommentHidden(context.Background(), uuid.MustParse(rude), &at); !errors.Is(err, board.ErrNotFound) {
		t.Fatalf("the store's hide of a removed comment: %v, want ErrNotFound", err)
	}

	// Under review.
	for _, p := range []feedPerson{r1, r2, r3} {
		w.report(p, "comments", held)
	}
	if ids, _ := w.commentIDs(o, entry); has(ids, held) {
		t.Fatalf("the post's owner still sees the comment under review: %v", ids)
	}
	if got := w.ok(o, http.MethodPost, "/v1/comments/"+held+"/hide", map[string]any{}); got["hidden"] != true {
		t.Fatalf("the hide of a comment under review: %v", got)
	}
	w.ok(o, http.MethodPost, "/v1/comments/"+held+"/unhide", map[string]any{})
	w.ok(q, http.MethodPost, "/v1/comments/"+held+"/delete", map[string]any{})
	if got := w.stored(`select count(*)::text from post_comments where id = $1`, held); got != "0" {
		t.Fatalf("the comment under review is still there after its writer's delete")
	}
}

// Today says why the day's post is missing when an admin removed it, in the
// evening and the next morning alike, while the day still counts. The
// owner's own delete, the other date's removal and a post brought back
// leave nothing to explain.
func TestModerationIntegration_TodaySaysThePostWasRemoved(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mt_olive", "Olive Owner")
	p := w.person("mt_pia", "Pia Poster")
	yesterday := w.post(o, "2026-10-07")
	const day = "2026-10-08"
	entry := w.post(o, day)
	pEntry := w.post(p, day)
	today := func(q feedPerson) map[string]any {
		t.Helper()
		return w.ok(q, http.MethodGet, "/v1/today", nil)
	}
	explained := func(m map[string]any) bool {
		t.Helper()
		v, present := m["entry_removed"]
		if present && v != true {
			t.Fatalf("entry_removed is %v, want it sent only as true: %v", v, m)
		}
		return present
	}

	before := today(o)
	if before["state"] != string(spin.StatePosted) || dig(t, before, "entry", "id") != entry || explained(before) {
		t.Fatalf("Thursday before: %v", before)
	}
	w.sql(`update entries set removed_at = now() where id = $1`, yesterday)
	if got := today(o); dig(t, got, "entry", "id") != entry || explained(got) {
		t.Fatalf("Thursday after Wednesday's post was removed: %v", got)
	}

	w.sql(`update entries set removed_at = now() where id = $1`, entry)
	after := today(o)
	if after["state"] != before["state"] || after["entry"] != nil || !explained(after) ||
		!reflect.DeepEqual(after["week"], before["week"]) || !reflect.DeepEqual(after["bonus"], before["bonus"]) {
		t.Fatalf("Thursday after the removal: %v\nbefore: %v", after, before)
	}
	deleted := w.ok(p, http.MethodPost, "/v1/entries/"+pEntry+"/delete", map[string]any{})["today"].(map[string]any)
	if deleted["state"] != string(spin.StatePosted) || deleted["entry"] != nil || explained(deleted) {
		t.Fatalf("Pia's Today after her own delete: %v", deleted)
	}

	// Friday morning, Today still shows Thursday's spin.
	w.at("2026-10-09", 10, 0)
	morning := today(o)
	if morning["state"] != string(spin.StatePosted) || dig(t, morning, "spin", "local_date") != day || morning["entry"] != nil || !explained(morning) {
		t.Fatalf("Friday morning: %v", morning)
	}
	if got := today(p); got["state"] != string(spin.StatePosted) || got["entry"] != nil || explained(got) {
		t.Fatalf("Pia's Friday morning: %v", got)
	}

	w.sql(`update entries set removed_at = null where id = $1`, entry)
	if got := today(o); dig(t, got, "entry", "id") != entry || explained(got) {
		t.Fatalf("Friday morning once it was brought back: %v", got)
	}
}

// The under review flag is sent to the post's owner wherever they see their
// post (the Board, the post page, the entry, Today, the archive, the diary
// and its summary), and to a comment's writer on their own comment only,
// as "under_review": true. Nobody else ever gets the key, and it is gone
// again once a report is dismissed.
func TestModerationIntegration_UnderReviewIsSentToItsOwnerOrWriterOnly(t *testing.T) {
	w := newModWorld(t)
	o := w.person("mj_olive", "Olive Owner")
	cw := w.person("mj_wren", "Wren Writer")
	v := w.person("mj_vera", "Vera Viewer")
	r1 := w.person("mj_rhea", "Rhea Reporter")
	r2 := w.person("mj_remy", "Remy Reporter")
	r3 := w.person("mj_rose", "Rose Reporter")
	const day = "2026-10-08"
	entry := w.post(o, day)
	for _, p := range []feedPerson{cw, v, r1, r2, r3} {
		w.followAccepted(p, o)
		w.post(p, day)
	}
	// O follows someone, so O's Board has cards.
	w.followAccepted(o, v)
	wc := w.reply(cw, entry, "Rude", "")["id"].(string)
	vc := w.reply(v, entry, "Lovely", "")["id"].(string)

	// card is entry's card on p's Board.
	card := func(p feedPerson) any {
		t.Helper()
		for _, d := range w.board(p, "")["days"].([]any) {
			for _, c := range d.(map[string]any)["cards"].([]any) {
				if c.(map[string]any)["entry_id"] == entry {
					return c
				}
			}
		}
		t.Fatalf("no card of the post on %s's Board", p.name)
		return nil
	}
	// owners is the flag in each place the owner sees the post.
	owners := func() []any {
		t.Helper()
		var row, recent any
		for _, it := range w.ok(o, http.MethodGet, "/v1/diary", nil)["items"].([]any) {
			if e, _ := it.(map[string]any)["entry"].(map[string]any); e != nil && e["id"] == entry {
				row = e
			}
		}
		for _, e := range w.ok(o, http.MethodGet, "/v1/me/summary", nil)["recent"].([]any) {
			if e.(map[string]any)["id"] == entry {
				recent = e
			}
		}
		if row == nil || recent == nil {
			t.Fatalf("the post is missing from the owner's diary (%v) or summary (%v)", row, recent)
		}
		return []any{
			underReview(card(o)),
			underReview(w.ok(o, http.MethodGet, "/v1/posts/"+entry, nil)["card"]),
			underReview(w.ok(o, http.MethodGet, "/v1/entries/"+entry, nil)["entry"]),
			underReview(w.ok(o, http.MethodGet, "/v1/today", nil)["entry"]),
			underReview(row),
			underReview(recent),
		}
	}
	// comments is the flag on each comment of the post page as p reads it.
	comments := func(p feedPerson) map[string]any {
		t.Helper()
		out := map[string]any{}
		for _, c := range w.ok(p, http.MethodGet, "/v1/posts/"+entry, nil)["comments"].([]any) {
			out[c.(map[string]any)["id"].(string)] = underReview(c)
		}
		return out
	}
	absent := []any{"absent", "absent", "absent", "absent", "absent", "absent"}
	everyone := map[string]any{wc: "absent", vc: "absent"}

	if got := owners(); !slices.Equal(got, absent) {
		t.Fatalf("the owner before any report: %v", got)
	}
	for _, p := range []feedPerson{o, cw, v} {
		if got := comments(p); !reflect.DeepEqual(got, everyone) {
			t.Fatalf("the comments as %s before any report: %v", p.name, got)
		}
	}
	if got := underReview(card(v)); got != "absent" {
		t.Fatalf("Vera's card before any report: %v", got)
	}

	// Wren's comment goes under review: only Wren is told, on it alone.
	for _, p := range []feedPerson{r1, r2, r3} {
		w.report(p, "comments", wc)
	}
	if got := comments(cw); !reflect.DeepEqual(got, map[string]any{wc: true, vc: "absent"}) {
		t.Fatalf("the comments as their writer: %v", got)
	}
	for _, p := range []feedPerson{o, v} {
		if got := comments(p); !reflect.DeepEqual(got, map[string]any{vc: "absent"}) {
			t.Fatalf("the comments as %s: %v", p.name, got)
		}
	}

	// The post goes under review: its owner is told everywhere.
	for _, p := range []feedPerson{r1, r2, r3} {
		w.report(p, "posts", entry)
	}
	if got := owners(); !slices.Equal(got, []any{true, true, true, true, true, true}) {
		t.Fatalf("the owner, under review: %v", got)
	}
	if got := entryObject(t, w.ok(o, http.MethodPost, "/v1/entries/"+entry+"/archive", map[string]any{})["entry"]); got["under_review"] != true {
		t.Fatalf("the archive's answer: %v", got)
	}
	archived := w.ok(o, http.MethodGet, "/v1/me/archive", nil)["entries"].([]any)
	if len(archived) != 1 || underReview(archived[0]) != true {
		t.Fatalf("the archive: %v", archived)
	}
	w.ok(o, http.MethodPost, "/v1/entries/"+entry+"/unarchive", map[string]any{})

	// One dismissal each, and nobody is told anything again.
	w.sql(`update post_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r1.id)
	w.sql(`update comment_reports set resolved_at = now(), resolution = 'dismissed' where reporter_id = $1`, r1.id)
	if got := owners(); !slices.Equal(got, absent) {
		t.Fatalf("the owner after the dismissal: %v", got)
	}
	for _, p := range []feedPerson{o, cw, v} {
		if got := comments(p); !reflect.DeepEqual(got, everyone) {
			t.Fatalf("the comments as %s after the dismissal: %v", p.name, got)
		}
	}
}
