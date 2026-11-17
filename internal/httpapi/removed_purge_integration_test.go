package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage/storagetest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// The purge of what an admin removed (spec section 7): once the 30 days are
// over, a removed post or comment is deleted for good with its photos, in
// the account purge's loop and after it, the files first and then the row.
// Removals are moved past the line in SQL, as the account tests move a
// deletion ask.

// purgeBucketName is the in-memory bucket's name.
const purgeBucketName = "rndmroll-test"

// storedKeys is every photo key the database names, sorted.
func storedKeys(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select storage_key from entry_photos
		union all
		select photo_key from post_comments where photo_key is not null
		order by 1`)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// purgerOver is the server's purger, with the removed content added as main
// adds it, over bucket.
func purgerOver(pool *pgxpool.Pool, bucket *storagetest.Bucket) *purge.Purger {
	p := purge.New(postgres.NewPurgeRepo(pool), storage.NewAccountFilesOver(bucket, purgeBucketName),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.SetRemoved(postgres.NewRemovedPurgeRepo(pool))
	return p
}

// commentOn posts p's comment (a reply under parent when set, with the
// photo key when set) a minute after the last one, and returns its id.
func (w *adminModWorld) commentOn(p feedPerson, entry, text, parent, key string) string {
	w.t.Helper()
	w.clk.Advance(time.Minute)
	body := map[string]any{"text": text}
	if parent != "" {
		body["parent_id"] = parent
	}
	if key != "" {
		body["photo_key"] = key
	}
	return w.call(p, http.MethodPost, "/v1/posts/"+entry+"/comments", body, http.StatusCreated)["comment"].(map[string]any)["id"].(string)
}

// photoOf is the photo key of a comment.
func (w *adminModWorld) photoOf(comment string) string {
	w.t.Helper()
	return w.one(`select photo_key from post_comments where id = $1`, comment)
}

// uuids parses ids.
func uuids(ids ...string) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.MustParse(id)
	}
	return out
}

// count reads one count straight from the database.
func (w *adminModWorld) count(q string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// Inside the 30 days the purge touches nothing. Past them it deletes the
// removed post's photos and then those of the comments and replies under
// it, whoever wrote them, and then the post, whose cascade takes the rest;
// and a removed comment with its replies the same way. A file storage cannot
// delete keeps the whole post for the next run, which finishes it. Only a
// key in its writer's own folder is ever sent to storage: a key in someone
// else's folder and a malformed one are skipped, and a file one of the
// writer's own posts still shows is kept. The reports and the decision log
// outlive the purge (their links go to null), so the owner's bell still
// says what was removed, and Removed before still counts it.
func TestRemovedPurgeIntegration_TheFilesGoFirstAndOnlyAfterThe30Days(t *testing.T) {
	w := newAdminModWorld(t)
	ctx := context.Background()
	ps := w.people("rp_bob", "rp_ann", "rp_cat", "rp_dan", "rp_vic")
	b, a, c, d, v := ps[0], ps[1], ps[2], ps[3], ps[4]
	for _, p := range []feedPerson{a, c, d, v} {
		w.followAccepted(p, b)
	}
	for _, p := range []feedPerson{a, d, v} {
		w.followAccepted(p, c)
	}
	const day = "2026-10-08"
	bPost := w.post(b, day)
	cPost := w.post(c, day)
	for _, p := range []feedPerson{a, d, v} {
		w.post(p, day)
	}
	keyOf := func(p feedPerson, ext string) string { return w.photos.newKey(uuid.MustParse(p.id), ext) }

	// On B's post: A's comment with C's and B's replies under it, and three
	// comments of C's that the database holds with photos no request could
	// set.
	aOnB := w.commentOn(a, bPost, "Love this", "", keyOf(a, "jpg"))
	cUnderA := w.commentOn(c, bPost, "Same", aOnB, keyOf(c, "webp"))
	bUnderA := w.commentOn(b, bPost, "Thanks", aOnB, keyOf(b, "png"))
	foreign := w.commentOn(c, bPost, "Someone else's file", "", "")
	foreignKey := keyOf(a, "jpg")
	setCommentPhoto(t, w.pool, foreign, foreignKey)
	malformed := w.commentOn(c, bPost, "A bad key", "", "")
	malformedKey := "entries/" + c.id + "/../" + a.id + "/0123456789abcdef.jpg"
	setCommentPhoto(t, w.pool, malformed, malformedKey)
	cPostPhoto := postPhotoKeys(t, w.pool, cPost)[0]
	shared := w.commentOn(c, bPost, "My own post's file", "", "")
	setCommentPhoto(t, w.pool, shared, cPostPhoto)
	// On C's post: A's comment with V's reply.
	aOnC := w.commentOn(a, cPost, "Rude words", "", keyOf(a, "jpg"))
	vUnderA := w.commentOn(v, cPost, "Agreed", aOnC, keyOf(v, "png"))

	w.ok(d, http.MethodPost, "/v1/comments/"+aOnC+"/report", map[string]any{"reason": "harassment"})
	w.ok(d, http.MethodPost, "/v1/posts/"+bPost+"/report", map[string]any{"reason": "spam"})
	if got := w.act("remove", "post", b, "Spam"); got != "/admin/people/"+b.id+"?n=removed" {
		t.Fatalf("remove the post: %q", got)
	}
	if got := w.act("remove", "comment", a, "Bullying or harassment"); got != "/admin/people/"+a.id+"?n=removed" {
		t.Fatalf("remove the comment: %q", got)
	}
	postRemoval := w.lastLogged(b)

	bPhotos := postPhotoKeys(t, w.pool, bPost)
	gone := append(slices.Clone(bPhotos), w.photoOf(aOnB), w.photoOf(cUnderA), w.photoOf(bUnderA), w.photoOf(aOnC), w.photoOf(vUnderA))
	unused := keyOf(a, "jpg")
	bucket := storagetest.NewBucket(purgeBucketName, append(storedKeys(t, w.pool), unused, "avatars/"+b.id+"/0000000000000001.jpg")...)
	before := bucket.Keys()
	purger := purgerOver(w.pool, bucket)

	if res := purger.RunRemovedOnce(ctx); res != (purge.Result{}) || len(bucket.Deleted()) != 0 {
		t.Fatalf("inside the 30 days: %+v, deleted %v; want nothing", res, bucket.Deleted())
	}
	w.exec(`update entries set removed_at = now() - interval '30 days 2 minutes' where id = $1`, bPost)
	w.exec(`update post_comments set removed_at = now() - interval '30 days 1 minute' where id = $1`, aOnC)

	// Storage fails on the first comment photo: the post's own photos went,
	// and the post, its photo rows and its comments all stay, still removed.
	// The comment on C's post goes, with its reply.
	bucket.FailDelete = map[string]error{w.photoOf(aOnB): errors.New("storage is down")}
	if res := purger.RunRemovedOnce(ctx); res != (purge.Result{Due: 2, Purged: 1, Failed: 1}) {
		t.Fatalf("the run with storage failing: %+v", res)
	}
	if w.count(`select count(*) from entries where id = $1 and removed_at is not null`, bPost) != 1 ||
		w.count(`select count(*) from entry_photos where entry_id = $1`, bPost) != len(bPhotos) ||
		w.count(`select count(*) from post_comments where entry_id = $1`, bPost) != 6 {
		t.Fatal("a file that could not be deleted did not keep the post with its photo rows and comments")
	}
	firstRun := append(append(slices.Clone(bPhotos), w.photoOf(aOnB)), gone[len(gone)-2:]...)
	if got := bucket.Deleted(); !slices.Equal(got, firstRun) {
		t.Fatalf("delete calls = %q\nwant the post's own photos, the failed one and then the comment's two: %q", got, firstRun)
	}
	if w.count(`select count(*) from post_comments where id = any($1::uuid[])`, uuids(aOnC, vUnderA)) != 0 {
		t.Fatal("the removed comment or its reply is still there")
	}
	if w.count(`select count(*) from entries where id = $1`, cPost) != 1 || !slices.Equal(postPhotoKeys(t, w.pool, cPost), []string{cPostPhoto}) {
		t.Fatal("the purge of a comment took its post")
	}

	// Storage is back: the next run finishes the post.
	bucket.FailDelete = nil
	if res := purger.RunRemovedOnce(ctx); res != (purge.Result{Due: 1, Purged: 1}) {
		t.Fatalf("the run with storage back: %+v", res)
	}
	if w.count(`select count(*) from entries where id = $1`, bPost) != 0 ||
		w.count(`select count(*) from entry_photos where entry_id = $1`, bPost) != 0 ||
		w.count(`select count(*) from post_comments where id = any($1::uuid[])`,
			uuids(aOnB, cUnderA, bUnderA, foreign, malformed, shared)) != 0 {
		t.Fatal("after the purge the post, its photo rows or its comments are still there")
	}
	for _, k := range before {
		if want := !slices.Contains(gone, k); bucket.Has(k) != want {
			t.Fatalf("after the purge the bucket has %s: %v, want %v", k, bucket.Has(k), want)
		}
	}
	for _, k := range []string{foreignKey, malformedKey, cPostPhoto, unused} {
		if slices.Contains(bucket.Deleted(), k) {
			t.Fatalf("storage was asked to delete %s", k)
		}
	}
	if res := purger.RunRemovedOnce(ctx); res != (purge.Result{}) {
		t.Fatalf("a run after: %+v, want nothing due", res)
	}

	// What outlives the purge: the report, the log with its item, the
	// owner's bell item and Removed before.
	if w.count(`select count(*) from post_reports where reporter_id = $1 and entry_id is null`, d.id) != 1 ||
		w.count(`select count(*) from moderation_events where item_id = $1 and entry_id is null and action = 'removed'`, bPost) != 1 ||
		w.count(`select count(*) from moderation_events where item_id = $1 and comment_id is null and action = 'removed'`, aOnC) != 1 {
		t.Fatal("the report or the decision log did not outlive the purge")
	}
	item, _ := w.bellItem(b, "post_removed:"+postRemoval.ID)
	wantItem(t, "the removal after the purge", item, map[string]any{"text": "Dune", "entry_id": nil, "photo_url": "", "ref": postRemoval.ID[:8]})
	if page := w.page("/admin/people/" + b.id); strings.Count(page, "Removed before: 1") != 1 {
		t.Fatalf("Removed before after the purge: %s", page)
	}
}

// An account whose deletion is due and that has a removed post, removed
// comments and a removed comment on its post leaves nothing behind. The
// loop purges the account first: its folders, the photos of the comments
// and replies that go with it (removed or not), and its row, whose cascade
// takes them all. The purge of removed content then finds nothing left to
// do. Every key a deleted row named is gone from the bucket, and every key a
// row still names is still in it.
func TestRemovedPurgeIntegration_AnAccountPurgeAndRemovedContentLeaveNothingBehind(t *testing.T) {
	w := newAdminModWorld(t)
	ctx := context.Background()
	ps := w.people("ro_bob", "ro_ann", "ro_cat", "ro_dan")
	b, a, c, d := ps[0], ps[1], ps[2], ps[3]
	for _, p := range []feedPerson{a, c, d} {
		w.followAccepted(p, b)
	}
	for _, p := range []feedPerson{b, c, d} {
		w.followAccepted(p, a)
	}
	const day = "2026-10-08"
	bPost := w.post(b, day)
	aPost := w.post(a, day)
	w.post(c, day)
	w.post(d, day)
	keyOf := func(p feedPerson, ext string) string { return w.photos.newKey(uuid.MustParse(p.id), ext) }
	aOnB := w.commentOn(a, bPost, "Hmm", "", keyOf(a, "jpg"))
	w.commentOn(c, bPost, "Same", aOnB, keyOf(c, "webp"))
	w.commentOn(c, bPost, "Hi", "", keyOf(c, "png"))
	bOnA := w.commentOn(b, aPost, "Rude words", "", keyOf(b, "jpg"))
	w.commentOn(c, aPost, "Hm", bOnA, keyOf(c, "jpg"))
	aOwn := w.commentOn(a, aPost, "Mine", "", keyOf(a, "png"))

	w.ok(d, http.MethodPost, "/v1/comments/"+aOnB+"/report", map[string]any{"reason": "hate"})
	w.ok(d, http.MethodPost, "/v1/comments/"+bOnA+"/report", map[string]any{"reason": "harassment"})
	w.ok(d, http.MethodPost, "/v1/posts/"+bPost+"/report", map[string]any{"reason": "spam"})
	for _, rm := range []struct {
		kind  string
		owner feedPerson
		mark  string
	}{{"comment", a, "Hate speech"}, {"comment", b, "Bullying or harassment"}, {"post", b, "Spam"}} {
		if got := w.act("remove", rm.kind, rm.owner, rm.mark); got != "/admin/people/"+rm.owner.id+"?n=removed" {
			t.Fatalf("remove the %s of %s: %q", rm.kind, rm.owner.name, got)
		}
	}
	// B asked over 30 days ago; the two comments were removed over 30 days
	// ago, B's post an hour ago.
	w.exec(`update users set deletion_requested_at = now() - interval '30 days 1 minute' where id = $1`, b.id)
	w.exec(`update post_comments set removed_at = now() - interval '30 days 1 minute' where id = $1`, aOnB)
	w.exec(`update post_comments set removed_at = now() - interval '30 days 1 minute' where id = $1`, bOnA)
	w.exec(`update entries set removed_at = now() - interval '1 hour' where id = $1`, bPost)

	stored := storedKeys(t, w.pool)
	bucket := storagetest.NewBucket(purgeBucketName, append(slices.Clone(stored), "avatars/"+b.id+"/0000000000000001.jpg")...)
	purger := purgerOver(w.pool, bucket)

	// One run of the loop, in its order.
	if res := purger.RunOnce(ctx); res != (purge.Result{Due: 1, Purged: 1}) {
		t.Fatalf("the account run: %+v", res)
	}
	if res := purger.RunRemovedOnce(ctx); res != (purge.Result{}) {
		t.Fatalf("the removed run after it: %+v, want nothing left", res)
	}

	left := storedKeys(t, w.pool)
	for _, k := range stored {
		if want := slices.Contains(left, k); bucket.Has(k) != want {
			t.Fatalf("the bucket has %s: %v, want %v (a row still names it: %v)", k, bucket.Has(k), want, want)
		}
	}
	if len(left) != 4 || bucket.Has("avatars/"+b.id+"/0000000000000001.jpg") {
		t.Fatalf("left: %v, want the photos of A's, C's and D's posts and of A's own comment; B's avatar gone", left)
	}
	if w.count(`select count(*) from post_comments where entry_id = $1 and id <> $2`, aPost, aOwn) != 0 ||
		w.count(`select count(*) from entries where user_id = $1`, b.id) != 0 ||
		w.count(`select count(*) from moderation_events where owner_id = $1`, b.id) != 0 {
		t.Fatal("rows of B, or comments that went with B, are still there")
	}
	if w.count(`select count(*) from moderation_events where owner_id = $1 and item_id = $2 and comment_id is null`, a.id, aOnB) != 1 {
		t.Fatal("A's decision log lost the removal of A's comment, which went with B's post")
	}
}

// removalInterval is admin.RemovalGrace as an SQL interval.
var removalInterval = fmt.Sprintf("interval '%d seconds'", int64(admin.RemovalGrace/time.Second))

// bringBackQ is Bring it back's own statement (AdminRepo.RestoreItem): it
// takes the row only while the post is inside the 30 days by the clock of
// its transaction, which stands still from the transaction's start.
var bringBackQ = `update entries set removed_at = null
	where id = $1 and user_id = $2 and removed_at > now() - ` + removalInterval

// dbClockPast waits until the database's clock is past at.
func dbClockPast(t *testing.T, pool *pgxpool.Pool, at time.Time) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		var past bool
		if err := pool.QueryRow(context.Background(), `select clock_timestamp() > $1`, at).Scan(&past); err != nil {
			t.Fatal(err)
		}
		if past {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the database's clock never passed the line")
		}
	}
}

// waitForARowLock waits until some statement is waiting on a lock, so a
// test knows it is blocked before it lets the holder go.
func waitForARowLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		var n int
		if err := pool.QueryRow(context.Background(),
			`select count(*) from pg_stat_activity where datname = current_database() and wait_event_type = 'Lock'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no statement ever waited on the row")
		}
	}
}

// Of a Bring it back and the purge at the 30-day line, only one wins. The
// only Bring it back that could still find a post inside the line once the
// purge finds it past is one whose transaction began just before the line
// crossed (a Bring it back begun later finds it past and is refused at
// once), so the test holds such a transaction itself and runs Bring it
// back's own statement in it.
//
// The purge first: it holds the post while its files go, Bring it back waits
// for it, and then finds nothing to bring back. Bring it back first: the
// purge waits for it, then finds the post brought back, and sends no file
// to storage and deletes nothing.
func TestRemovedPurgeIntegration_ABringItBackAndThePurgeNeverBothWin(t *testing.T) {
	w := newAdminModWorld(t)
	ctx := context.Background()
	ps := w.people("rr_bob", "rr_cat", "rr_rhea")
	b, c, r := ps[0], ps[1], ps[2]
	w.followAccepted(r, b)
	w.followAccepted(r, c)
	const day = "2026-10-08"
	first := w.post(b, day)
	second := w.post(c, day)
	w.post(r, day)
	for _, rm := range []struct {
		post  string
		owner feedPerson
	}{{first, b}, {second, c}} {
		w.ok(r, http.MethodPost, "/v1/posts/"+rm.post+"/report", map[string]any{"reason": "spam"})
		if got := w.act("remove", "post", rm.owner, "Spam"); got != "/admin/people/"+rm.owner.id+"?n=removed" {
			t.Fatalf("remove: %q", got)
		}
	}
	repo := postgres.NewRemovedPurgeRepo(w.pool)
	item := func(post string) purge.Removed { return purge.Removed{Kind: "post", ID: uuid.MustParse(post)} }
	type outcome struct {
		won bool
		err error
	}

	// atTheLine begins Bring it back's transaction and moves the post's
	// removal to 200 ms inside the 30 days by that transaction's clock, then
	// waits until the line has crossed for any transaction begun after.
	atTheLine := func(post string) pgx.Tx {
		t.Helper()
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		var began time.Time
		if err := tx.QueryRow(ctx, `select now()`).Scan(&began); err != nil {
			t.Fatal(err)
		}
		w.exec(`update entries set removed_at = $2::timestamptz - `+removalInterval+` + interval '200 milliseconds' where id = $1`, post, began)
		dbClockPast(t, w.pool, began.Add(300*time.Millisecond))
		return tx
	}

	// The purge first.
	tx := atTheLine(first)
	holding, release := make(chan struct{}), make(chan struct{})
	purged := make(chan outcome, 1)
	go func() {
		won, err := repo.Purge(ctx, item(first), func(context.Context, []purge.Photo) error {
			close(holding)
			<-release
			return nil
		})
		purged <- outcome{won, err}
	}()
	select {
	case <-holding:
	case <-time.After(5 * time.Second):
		t.Fatal("the purge never held the post")
	}
	restored := make(chan outcome, 1)
	go func() {
		tag, err := tx.Exec(ctx, bringBackQ, first, b.id)
		restored <- outcome{tag.RowsAffected() == 1, err}
	}()
	waitForARowLock(t, w.pool)
	close(release)
	if got := <-purged; !got.won || got.err != nil {
		t.Fatalf("the purge holding the post first: %+v, want it deleted", got)
	}
	if got := <-restored; got.won || got.err != nil {
		t.Fatalf("Bring it back after the purge: %+v, want nothing brought back", got)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if w.count(`select count(*) from entries where id = $1`, first) != 0 {
		t.Fatal("the purged post is still there")
	}

	// Bring it back first.
	tx = atTheLine(second)
	if tag, err := tx.Exec(ctx, bringBackQ, second, c.id); err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("Bring it back inside the line by its own clock: %v, %v", tag, err)
	}
	sent := make(chan struct{}, 1)
	go func() {
		won, err := repo.Purge(ctx, item(second), func(context.Context, []purge.Photo) error {
			sent <- struct{}{}
			return nil
		})
		purged <- outcome{won, err}
	}()
	waitForARowLock(t, w.pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-purged; got.won || got.err != nil {
		t.Fatalf("the purge after Bring it back: %+v, want nothing purged", got)
	}
	select {
	case <-sent:
		t.Fatal("the purge sent files of a post that was brought back")
	default:
	}
	if w.count(`select count(*) from entries where id = $1 and removed_at is null`, second) != 1 {
		t.Fatal("the post brought back is not there")
	}
	w.call(c, http.MethodGet, "/v1/entries/"+second, nil, http.StatusOK)
}
