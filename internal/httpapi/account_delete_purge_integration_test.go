package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/purge"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage/storagetest"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// purgeUserColumns lists, for every table with a column that references
// users, those columns. They are read from the database's own foreign keys,
// so a table added later is covered without changing this test. users
// itself is listed with its id.
func purgeUserColumns(t *testing.T, pool *pgxpool.Pool) map[string][]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select distinct c.conrelid::regclass::text, a.attname
		from pg_constraint c
		cross join lateral unnest(c.conkey) as k(attnum)
		join pg_attribute a on a.attrelid = c.conrelid and a.attnum = k.attnum
		where c.contype = 'f' and c.confrelid = 'users'::regclass
		order by 1, 2`)
	if err != nil {
		t.Fatalf("read the foreign keys to users: %v", err)
	}
	defer rows.Close()
	out := map[string][]string{"users": {"id"}}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatal(err)
		}
		out[table] = append(out[table], column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) < 10 {
		t.Fatalf("only %d tables reference users: %v", len(out), out)
	}
	return out
}

// purgeSnapshot is every row, as text and sorted, of every table in cols
// that names the account id in one of its users columns and names gone in
// none of them, leaving out the comments listed in skip. Taken before and
// after the purge of gone, the two must be equal: the purge may delete what
// names gone (a follow, a block) and the comments that go with gone's posts
// and comments, and nothing else.
func purgeSnapshot(t *testing.T, pool *pgxpool.Pool, cols map[string][]string, id, gone string, skip []uuid.UUID) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for table, columns := range cols {
		mine := make([]string, len(columns))
		theirs := make([]string, len(columns))
		for i, column := range columns {
			mine[i] = "t." + column + " = $1"
			theirs[i] = "t." + column + " is not distinct from $2"
		}
		q := "select t::text from " + table + " t where (" + strings.Join(mine, " or ") + ") and not (" + strings.Join(theirs, " or ") + ")"
		args := []any{uuid.MustParse(id), uuid.MustParse(gone)}
		if table == "post_comments" {
			q += " and not (t.id = any($3::uuid[]))"
			args = append(args, skip)
		}
		rows, err := pool.Query(context.Background(), q+" order by 1", args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		list := []string{}
		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			list = append(list, row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		out[table] = list
	}
	return out
}

// purgeRowsNaming counts, table by table, the rows that name id in one of
// their users columns.
func purgeRowsNaming(t *testing.T, pool *pgxpool.Pool, cols map[string][]string, id string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for table, columns := range cols {
		names := make([]string, len(columns))
		for i, column := range columns {
			names[i] = "t." + column + " = $1"
		}
		var n int
		if err := pool.QueryRow(context.Background(), "select count(*) from "+table+" t where "+strings.Join(names, " or "),
			uuid.MustParse(id)).Scan(&n); err != nil {
			t.Fatalf("count the rows of %s naming %s: %v", table, id, err)
		}
		out[table] = n
	}
	return out
}

// Once the grace period is over the purge deletes the account for good, and
// only that account. B asked over 30 days ago and D asked an hour short of
// 30 days; A and C are ordinary accounts. The bucket holds every photo the
// database names, an avatar for each of the four, an upload of B's that was
// never posted, and keys that look like B's folders but are not in them.
//
// One run purges B alone. B's row is gone, and with it every row that names
// B, B's posts and comments, and the other people's comments that went with
// them: A's comment on B's post and C's reply under B's comment. In the
// bucket, B's two folders are emptied and the photos of those two comments
// deleted, and nothing else is ever sent to storage: every other key stays,
// the look-alikes included, and every other row of A, C and D is exactly as
// it was. D, still inside the grace period, is untouched and can still keep
// the account by logging in. A second run finds nothing to do, and B's email
// and username are free again.
func TestDeleteAccountIntegration_PurgeDeletesOnlyThatAccount(t *testing.T) {
	w := newDeleteWorld(t)
	ctx := context.Background()
	a, _ := w.account("dq_alice", "Alice Ames")
	b, _ := w.account("dq_bob", "Bob Brown")
	c, _ := w.account("dq_cara", "Cara Cole")
	d, _ := w.account("dq_dan", "Dan Dorn")
	// Photo keys shaped exactly like the real store's, in each writer's own
	// folder.
	keys := &fakeEntryPhotoStore{}
	keyOf := func(p feedPerson, ext string) string { return keys.newKey(uuid.MustParse(p.id), ext) }
	avatarOf := func(p feedPerson) string { return "avatars/" + p.id + "/" + fakeAvatarKeyName }
	w.ok(b, http.MethodPatch, "/v1/me", map[string]any{"avatar_key": avatarOf(b)})

	// A and B follow each other, and C follows A. All four post one evening.
	w.followAccepted(a, b)
	w.followAccepted(b, a)
	w.followAccepted(c, a)
	const day = "2026-10-08"
	aPost := w.postAt(a, day, 20, 30)
	bPost := w.postAt(b, day, 20, 31)
	w.postAt(c, day, 20, 32)
	w.postAt(d, day, 20, 33)
	comment := func(p feedPerson, entry, text, parent, key string) string {
		t.Helper()
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
	aOnB := comment(a, bPost, "Love this", "", keyOf(a, "jpg"))
	bOnA := comment(b, aPost, "Thanks", "", keyOf(b, "png"))
	cUnderB := comment(c, aPost, "Same here", bOnA, keyOf(c, "webp"))
	aOwn := comment(a, aPost, "My own", "", keyOf(a, "jpg"))
	cOnA := comment(c, aPost, "Nice", "", "")

	// B asked over 30 days ago, D an hour short of 30 days.
	w.askToDelete(b.token)
	w.backdateAsk(b.id, deletionPastGrace)
	w.askToDelete(d.token)
	w.backdateAsk(d.id, deletionInsideGrace)
	dAsked := w.requestedAt(d.id)

	// The bucket: every photo the database names, an avatar for each, an
	// upload of B's never posted, and look-alikes of B's folders.
	var dbKeys []string
	rows, err := w.pool.Query(ctx, `select storage_key from entry_photos union all select photo_key from post_comments where photo_key is not null`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		dbKeys = append(dbKeys, k)
	}
	rows.Close()
	if len(dbKeys) != 8 {
		t.Fatalf("the database names %d photos, want 8 (4 posts, 4 comments): %v", len(dbKeys), dbKeys)
	}
	const bucketName = "rndmroll-test"
	bucket := storagetest.NewBucket(bucketName, dbKeys...)
	for _, p := range []feedPerson{a, b, c, d} {
		bucket.Put(avatarOf(p))
	}
	bucket.Put("entries/" + b.id + "/ffffffffffffffff.jpg")
	bucket.Put(
		"avatars/"+b.id+".jpg",
		"avatars/"+b.id+"x/x.jpg",
		"entries/"+b.id,
		"entries/"+b.id+"-old/0123456789abcdef.jpg",
		"entries/"+strings.ToUpper(b.id)+"/0123456789abcdef.jpg",
		"x/entries/"+b.id+"/0123456789abcdef.jpg",
		"backups/avatars/"+b.id+"/x.jpg",
	)
	before := bucket.Keys()

	// What is to go from the bucket: B's two folders (the avatar, the post's
	// photo, B's comment's photo and the upload never posted), and the photos
	// of the two comments of others that go with B.
	photoOf := func(commentID string) string {
		t.Helper()
		var k string
		if err := w.pool.QueryRow(ctx, `select photo_key from post_comments where id = $1`, uuid.MustParse(commentID)).Scan(&k); err != nil {
			t.Fatal(err)
		}
		return k
	}
	var gone []string
	for _, k := range before {
		if strings.HasPrefix(k, "avatars/"+b.id+"/") || strings.HasPrefix(k, "entries/"+b.id+"/") {
			gone = append(gone, k)
		}
	}
	if len(gone) != 4 {
		t.Fatalf("B's folders hold %v, want 4 files", gone)
	}
	gone = append(gone, photoOf(aOnB), photoOf(cUnderB))

	// What the database holds of each account before the run.
	cols := purgeUserColumns(t, w.pool)
	cascaded := []uuid.UUID{uuid.MustParse(aOnB), uuid.MustParse(cUnderB)}
	others := []feedPerson{a, c, d}
	snapshots := func() map[string]map[string][]string {
		t.Helper()
		out := map[string]map[string][]string{}
		for _, p := range others {
			out[p.id] = purgeSnapshot(t, w.pool, cols, p.id, b.id, cascaded)
		}
		return out
	}
	kept := snapshots()
	if len(kept[a.id]["users"]) != 1 || len(kept[a.id]["entries"]) != 1 || len(kept[a.id]["post_comments"]) != 1 ||
		len(kept[c.id]["post_comments"]) != 1 || len(kept[c.id]["follows"]) != 1 || len(kept[d.id]["entries"]) != 1 {
		t.Fatalf("the snapshots hold less than the test made: A %v, C %v, D %v", kept[a.id], kept[c.id], kept[d.id])
	}
	if naming := purgeRowsNaming(t, w.pool, cols, b.id); naming["users"] != 1 || naming["entries"] != 1 ||
		naming["post_comments"] != 1 || naming["follows"] != 2 || naming["refresh_tokens"] == 0 {
		t.Fatalf("the rows naming B before the run: %v", naming)
	}

	purger := purge.New(postgres.NewPurgeRepo(w.pool), storage.NewAccountFilesOver(bucket, bucketName),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if res := purger.RunOnce(ctx); res != (purge.Result{Due: 1, Purged: 1}) {
		t.Fatalf("the run: %+v, want 1 due and 1 purged", res)
	}

	// B is gone from the database, and so is every row that named B.
	for table, n := range purgeRowsNaming(t, w.pool, cols, b.id) {
		if n != 0 {
			t.Fatalf("after the purge %d rows of %s still name B", n, table)
		}
	}
	for what, id := range map[string]string{"A's comment on B's post": aOnB, "C's reply under B's comment": cUnderB, "B's comment": bOnA} {
		if n := w.countFor(`select count(*) from post_comments where id = $1`, id); n != 0 {
			t.Fatalf("after the purge %s is still there", what)
		}
	}
	for what, id := range map[string]string{"A's comment on A's own post": aOwn, "C's comment on A's post": cOnA} {
		if n := w.countFor(`select count(*) from post_comments where id = $1`, id); n != 1 {
			t.Fatalf("after the purge %s is gone", what)
		}
	}
	if n := w.countFor(`select count(*) from entries where id = $1`, aPost); n != 1 {
		t.Fatal("after the purge A's post is gone")
	}
	// Every other row of A, C and D is exactly as it was.
	after := snapshots()
	for _, p := range others {
		for table, rows := range kept[p.id] {
			if !sameList(rows, after[p.id][table]) {
				t.Fatalf("after the purge the %s rows of %s changed:\nbefore %v\nafter  %v", table, p.name, rows, after[p.id][table])
			}
		}
	}

	// The bucket: what was to go is gone, everything else is still there,
	// and storage was asked to delete exactly what was to go.
	for _, k := range before {
		if want := !slices.Contains(gone, k); bucket.Has(k) != want {
			t.Fatalf("after the purge the bucket has %s: %v, want %v", k, bucket.Has(k), want)
		}
	}
	if got := sortedCopy(bucket.Deleted()...); !sameList(got, sortedCopy(gone...)) {
		t.Fatalf("storage was asked to delete %v, want exactly %v", got, sortedCopy(gone...))
	}

	// A second run finds nothing to do, and sends nothing to storage.
	asked := len(bucket.Deleted())
	if res := purger.RunOnce(ctx); res != (purge.Result{}) {
		t.Fatalf("a second run: %+v, want nothing due", res)
	}
	if n := len(bucket.Deleted()); n != asked {
		t.Fatalf("a second run asked storage for %d more deletes", n-asked)
	}

	// D is untouched and can still keep the account.
	if at := w.requestedAt(d.id); at == nil || !at.Equal(*dAsked) {
		t.Fatalf("D's ask is %v after the purge, want it unchanged at %v", at, dAsked)
	}
	login := w.logIn("dq_dan@example.com", deleteTestPassword)
	if login.Code != http.StatusOK || decodeBody(t, login)["restored"] != true {
		t.Fatalf("D logs in inside the grace period after the purge: %d %s, want 200 with restored true", login.Code, login.Body.String())
	}

	// B's email and username can be used again.
	integrationOnboardAccount(t, w.engine, "dq_bob@example.com", deleteTestPassword, "New Bob", "dq_bob")
}
