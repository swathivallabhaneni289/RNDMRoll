package spintest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// EntryEnv is what one run of the entry suite needs: the same hooks as the
// service suite, plus the repository itself so the suite can read stored rows
// (entries, photo rows, a spin's result and settled time, a posted day)
// through Repo.InUserTx. One suite then serves MemRepo and Postgres.
type EntryEnv struct {
	*Env
	// Repo is the repository the Service was built with.
	Repo spin.Repository
}

// entryRacers is how many calls race in the concurrency cases.
const entryRacers = 20

func str(s string) *string { return &s }
func num(n int) *int       { return &n }

// --- fixtures ---

// key returns a photo key shaped like a real one, in the person's own folder:
// entries/<user id>/<16 hex digits>.jpg. The user id keeps keys unique across
// runs against one database.
func (e *EntryEnv) key(u string) string {
	return "entries/" + u + "/" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16] + ".jpg"
}

// photos returns n new photos with fresh keys, alternating camera and library.
func (e *EntryEnv) photos(u string, n int) []spin.NewPhoto {
	list := make([]spin.NewPhoto, n)
	for i := range list {
		source := spin.PhotoSourceLibrary
		if i%2 == 0 {
			source = spin.PhotoSourceCamera
		}
		list[i] = spin.NewPhoto{Key: e.key(u), Source: source}
	}
	return list
}

// input returns a good entry with n photos.
func (e *EntryEnv) input(u string, n int) spin.EntryInput {
	return spin.EntryInput{Title: "Dune", Stars: 7, Photos: e.photos(u, n)}
}

func keysOf(list []spin.NewPhoto) []string {
	keys := make([]string, len(list))
	for i, p := range list {
		keys[i] = p.Key
	}
	return keys
}

func keysOfPhotos(list []spin.EntryPhoto) []string {
	keys := make([]string, len(list))
	for i, p := range list {
		keys[i] = p.Key
	}
	return keys
}

func idsOf(list []spin.EntryPhoto) []string {
	ids := make([]string, len(list))
	for i, p := range list {
		ids[i] = p.ID
	}
	return ids
}

// keepRefs turns photo ids into the list of an edit that keeps them in order.
func keepRefs(ids ...string) []spin.PhotoRef {
	refs := make([]spin.PhotoRef, len(ids))
	for i, id := range ids {
		refs[i] = spin.PhotoRef{ID: id}
	}
	return refs
}

// openDay moves the clock to 8:30 PM on day, makes the daily spin and returns
// it.
func (e *EntryEnv) openDay(t *testing.T, u, day string) *spin.SpinView {
	t.Helper()
	e.setClock(t, ny, day, 20, 30, 0)
	out := e.spin(t, u)
	if out.Today.Spin == nil {
		t.Fatalf("no spin on %s: %+v", day, out.Today)
	}
	return out.Today.Spin
}

// --- service shortcuts ---

func (e *EntryEnv) tryPost(t *testing.T, u, day string, seq int, in spin.EntryInput) (spin.EntryOutcome, error) {
	t.Helper()
	return e.Service.PostEntry(ctx, u, date(t, day), seq, in)
}

func (e *EntryEnv) post(t *testing.T, u, day string, seq int, in spin.EntryInput) spin.EntryOutcome {
	t.Helper()
	out, err := e.tryPost(t, u, day, seq, in)
	if err != nil {
		t.Fatalf("PostEntry(%s, seq %d): %v", day, seq, err)
	}
	return out
}

func (e *EntryEnv) skip(t *testing.T, u, day string, seq int) spin.SkipOutcome {
	t.Helper()
	out, err := e.Service.SkipDay(ctx, u, date(t, day), seq)
	if err != nil {
		t.Fatalf("SkipDay(%s, seq %d): %v", day, seq, err)
	}
	return out
}

func (e *EntryEnv) get(t *testing.T, u, id string) spin.Entry {
	t.Helper()
	got, err := e.Service.GetEntry(ctx, u, id)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	return got
}

func (e *EntryEnv) edit(t *testing.T, u, id string, p spin.EntryPatch) spin.EditOutcome {
	t.Helper()
	out, err := e.Service.EditEntry(ctx, u, id, p)
	if err != nil {
		t.Fatalf("EditEntry: %v", err)
	}
	return out
}

func (e *EntryEnv) remove(t *testing.T, u, id string) spin.DeleteOutcome {
	t.Helper()
	out, err := e.Service.DeleteEntry(ctx, u, id)
	if err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
	return out
}

// --- stored rows, read through the repository ---

// raw runs fn inside a transaction of the repository itself. fn must not call
// the service: the person's lock is held.
func (e *EntryEnv) raw(t *testing.T, u string, fn func(tx spin.Tx) error) {
	t.Helper()
	at := e.Clock.NowFor(u)
	seed := spin.Seed{At: at, LocalDate: spin.DateOf(at.UTC()), TZName: ny, Goal: spin.DefaultWeeklyGoal}
	if err := e.Repo.InUserTx(ctx, u, seed, fn); err != nil {
		t.Fatalf("reading stored rows: %v", err)
	}
}

func (e *EntryEnv) entryOn(t *testing.T, u, day string) *spin.Entry {
	t.Helper()
	d := date(t, day)
	var got *spin.Entry
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.EntryOfDate(d)
		return err
	})
	return got
}

// entryRow reads an entry by id straight from the repository.
func (e *EntryEnv) entryRow(t *testing.T, u, id string) (*spin.Entry, error) {
	t.Helper()
	var got *spin.Entry
	var missing error
	e.raw(t, u, func(tx spin.Tx) error {
		var err error
		got, err = tx.EntryByID(id)
		if errors.Is(err, spin.ErrEntryNotFound) {
			missing = err
			return nil
		}
		return err
	})
	return got, missing
}

func (e *EntryEnv) photoRows(t *testing.T, u, entryID string) []spin.EntryPhoto {
	t.Helper()
	var got []spin.EntryPhoto
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.PhotosOfEntry(entryID)
		return err
	})
	return got
}

// spinRow reads the date's latest spin as stored (its result is the stored
// value and SettledAt is the stored settled time).
func (e *EntryEnv) spinRow(t *testing.T, u, day string) *spin.Spin {
	t.Helper()
	d := date(t, day)
	var got *spin.Spin
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.LatestSpinOfDate(d)
		return err
	})
	return got
}

func (e *EntryEnv) dailyRow(t *testing.T, u, day string) *spin.Spin {
	t.Helper()
	d := date(t, day)
	var got *spin.Spin
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.DailySpinOfDate(d)
		return err
	})
	return got
}

func (e *EntryEnv) postedRow(t *testing.T, u, day string) *spin.PostedDay {
	t.Helper()
	d := date(t, day)
	var got *spin.PostedDay
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.PostedDay(d)
		return err
	})
	return got
}

// keysUsed returns which of keys belong to a photo row.
func (e *EntryEnv) keysUsed(t *testing.T, u string, keys []string) map[string]bool {
	t.Helper()
	var got map[string]bool
	e.raw(t, u, func(tx spin.Tx) (err error) {
		got, err = tx.PhotoKeysInUse(keys)
		return err
	})
	return got
}

// --- assertions ---

func wantProblem(t *testing.T, err error, field, reason string) {
	t.Helper()
	var fp *spin.FieldProblem
	if !errors.As(err, &fp) {
		t.Fatalf("error = %v (%T), want a *FieldProblem %s/%s", err, err, field, reason)
	}
	if fp.Field != field || fp.Reason != reason {
		t.Fatalf("problem = %s/%s, want %s/%s", fp.Field, fp.Reason, field, reason)
	}
}

// wantPhotos checks a photo list: the keys in order, positions 0 to n-1 and a
// distinct id on every row.
func wantPhotos(t *testing.T, label string, got []spin.EntryPhoto, keys []string) {
	t.Helper()
	if len(got) != len(keys) {
		t.Fatalf("%s: %d photos %v, want %d %v", label, len(got), keysOfPhotos(got), len(keys), keys)
	}
	seen := make(map[string]bool, len(got))
	for i, p := range got {
		if p.Position != i || p.Key != keys[i] {
			t.Fatalf("%s: photo %d is position %d key %q, want position %d key %q", label, i, p.Position, p.Key, i, keys[i])
		}
		if p.ID == "" || seen[p.ID] {
			t.Fatalf("%s: photo %d has id %q (empty or repeated)", label, i, p.ID)
		}
		seen[p.ID] = true
	}
}

// wantSameKeys compares two key lists without regard to order.
func wantSameKeys(t *testing.T, label string, got, want []string) {
	t.Helper()
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("%s: keys %v, want %v", label, got, want)
	}
}

func wantInstant(t *testing.T, label string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s: %v, want %v", label, got, want)
	}
}

// wantNoEntry checks that a refused post wrote no entry and no posted day,
// and that none of keys is held by a photo row.
func (e *EntryEnv) wantNoEntry(t *testing.T, u, day string, keys []string) {
	t.Helper()
	if got := e.entryOn(t, u, day); got != nil {
		t.Fatalf("an entry was written for %s: %+v", day, got)
	}
	if pd := e.postedRow(t, u, day); pd != nil {
		t.Fatalf("a posted day was written for %s: %+v", day, pd)
	}
	if used := e.keysUsed(t, u, keys); len(used) != 0 {
		t.Fatalf("photo rows were written for %v", used)
	}
}

// wantNothingWritten checks that a refused call left an open date alone: as
// wantNoEntry, and the spin still open and unsettled.
func (e *EntryEnv) wantNothingWritten(t *testing.T, u, day string, keys []string) {
	t.Helper()
	e.wantNoEntry(t, u, day, keys)
	if s := e.spinRow(t, u, day); s != nil && (s.Result != spin.ResultOpen || s.SettledAt != nil) {
		t.Fatalf("the spin of %s was settled: result %s settled %v", day, s.Result, s.SettledAt)
	}
}

// wantSummary checks the entry summary Today carries.
func wantSummary(t *testing.T, got *spin.EntryRef, id, title string, stars int, reaction *string, cover string, count int) {
	t.Helper()
	if got == nil {
		t.Fatal("today has no entry summary")
	}
	if got.ID != id || got.Title != title || got.Stars != stars || got.CoverKey != cover || got.PhotoCount != count {
		t.Fatalf("summary = %+v, want id %s title %q stars %d cover %q count %d", got, id, title, stars, cover, count)
	}
	switch {
	case reaction == nil && got.Reaction != nil:
		t.Fatalf("summary reaction = %q, want none", *got.Reaction)
	case reaction != nil && (got.Reaction == nil || *got.Reaction != *reaction):
		t.Fatalf("summary reaction = %v, want %q", got.Reaction, *reaction)
	}
}

func skippedFlags(w spin.WeekView) []spin.Date {
	var out []spin.Date
	for _, d := range w.Days {
		if d.Skipped {
			out = append(out, d.Date)
		}
	}
	return out
}

// RunEntrySuite runs the entry behaviour every spin repository must give:
// post, read, edit, delete and skip, the Today states they cause, and the
// races between them. Each subtest gets a fresh environment from newEnv.
func RunEntrySuite(t *testing.T, newEnv func(t *testing.T) *EntryEnv) {
	t.Helper()
	run := func(name string, fn func(t *testing.T, e *EntryEnv)) {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			fn(t, e)
		})
	}

	// --- posting ---

	run("a post with three photos writes the entry and three photo rows in order", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		sp := e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		in.Reaction, in.Comment = str("fire"), str("Slow start, strong end.")
		out := e.post(t, u, "2026-10-07", 0, in)
		if out.Already {
			t.Fatal("the first post says already")
		}
		got := out.Entry
		if got.ID == "" || got.SpinID != sp.ID || got.LocalDate != date(t, "2026-10-07") {
			t.Fatalf("entry = %+v, want a new entry of spin %s on 2026-10-07", got, sp.ID)
		}
		keys := keysOf(in.Photos)
		wantPhotos(t, "answer", got.Photos, keys)
		for i, p := range got.Photos {
			if p.Source != in.Photos[i].Source || p.ContentType != "image/jpeg" {
				t.Errorf("photo %d = %+v, want source %s and image/jpeg", i, p, in.Photos[i].Source)
			}
		}
		wantPhotos(t, "stored rows", e.photoRows(t, u, got.ID), keys)
		wantPhotos(t, "read", e.get(t, u, got.ID).Photos, keys)
		if s := out.Today.Entry; s == nil || s.CoverKey != keys[0] || s.PhotoCount != 3 {
			t.Fatalf("today summary = %+v, want cover %q and 3 photos", s, keys[0])
		}
	})

	run("the content type of a photo comes from the key's extension", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		base := strings.TrimSuffix(e.key(u), ".jpg")
		in := spin.EntryInput{Title: "Dune", Stars: 7, Photos: []spin.NewPhoto{
			{Key: base + ".jpg", Source: spin.PhotoSourceCamera},
			{Key: strings.TrimSuffix(e.key(u), ".jpg") + ".png", Source: spin.PhotoSourceLibrary, SizeBytes: num(2345678)},
			{Key: strings.TrimSuffix(e.key(u), ".jpg") + ".webp", Source: spin.PhotoSourceLibrary},
		}}
		out := e.post(t, u, "2026-10-07", 0, in)
		want := []string{"image/jpeg", "image/png", "image/webp"}
		for i, p := range out.Entry.Photos {
			if p.ContentType != want[i] {
				t.Errorf("photo %d content type = %q, want %q", i, p.ContentType, want[i])
			}
		}
		if p := e.photoRows(t, u, out.Entry.ID)[1]; p.SizeBytes == nil || *p.SizeBytes != 2345678 {
			t.Errorf("declared size of photo 1 = %v, want 2345678", p.SizeBytes)
		}
	})

	run("ten photos are accepted, eleven are refused before any write, none is required", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		ten := e.input(u, 10)
		out := e.post(t, u, "2026-10-07", 0, ten)
		wantPhotos(t, "ten photos", out.Entry.Photos, keysOf(ten.Photos))
		if out.Today.Entry == nil || out.Today.Entry.PhotoCount != 10 {
			t.Fatalf("today summary = %+v, want 10 photos", out.Today.Entry)
		}

		e.openDay(t, u, "2026-10-08")
		eleven := e.input(u, 11)
		_, err := e.tryPost(t, u, "2026-10-08", 0, eleven)
		wantProblem(t, err, "photos", spin.ProblemTooMany)
		e.wantNothingWritten(t, u, "2026-10-08", keysOf(eleven.Photos))

		empty := e.input(u, 0)
		_, err = e.tryPost(t, u, "2026-10-08", 0, empty)
		wantProblem(t, err, "photos", spin.ProblemRequired)
		missing := e.input(u, 0)
		missing.Photos = nil
		_, err = e.tryPost(t, u, "2026-10-08", 0, missing)
		wantProblem(t, err, "photos", spin.ProblemRequired)
		e.wantNothingWritten(t, u, "2026-10-08", nil)
	})

	run("a key twice, or one already on another entry, is a photos problem and nothing is written", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		first := e.post(t, u, "2026-10-07", 0, e.input(u, 1))
		taken := first.Entry.Photos[0].Key

		e.openDay(t, u, "2026-10-08")
		fresh := e.key(u)
		twice := e.input(u, 0)
		twice.Photos = []spin.NewPhoto{
			{Key: fresh, Source: spin.PhotoSourceCamera},
			{Key: fresh, Source: spin.PhotoSourceLibrary},
		}
		_, err := e.tryPost(t, u, "2026-10-08", 0, twice)
		wantProblem(t, err, "photos", spin.ProblemInvalid)
		e.wantNothingWritten(t, u, "2026-10-08", []string{fresh})

		attached := e.input(u, 0)
		attached.Photos = []spin.NewPhoto{
			{Key: fresh, Source: spin.PhotoSourceCamera},
			{Key: taken, Source: spin.PhotoSourceLibrary},
		}
		_, err = e.tryPost(t, u, "2026-10-08", 0, attached)
		wantProblem(t, err, "photos", spin.ProblemInvalid)
		e.wantNothingWritten(t, u, "2026-10-08", []string{fresh})
		wantPhotos(t, "the first entry", e.photoRows(t, u, first.Entry.ID), []string{taken})

		gif := e.input(u, 1)
		gif.Photos[0].Key = strings.TrimSuffix(gif.Photos[0].Key, ".jpg") + ".gif"
		_, err = e.tryPost(t, u, "2026-10-08", 0, gif)
		wantProblem(t, err, "photos", spin.ProblemInvalid)
		e.wantNothingWritten(t, u, "2026-10-08", keysOf(gif.Photos))
	})

	run("a post on an open day writes the entry, the posted day and the spin's result", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		sp := e.openDay(t, u, "2026-10-07")
		at := e.Clock.NowFor(u)
		in := e.input(u, 2)
		in.Reaction = str("fire")
		out := e.post(t, u, "2026-10-07", 0, in)

		ent := out.Entry
		if ent.LocalDate != sp.LocalDate || ent.SpinID != sp.ID || ent.CategoryID != sp.Category.ID || ent.CategoryName != sp.Category.Name {
			t.Fatalf("entry = %+v, want the date, spin and category of %+v", ent, sp)
		}
		if ent.Title != "Dune" || ent.Stars != 7 || ent.Reaction == nil || *ent.Reaction != "fire" || ent.Comment != nil || ent.Late {
			t.Fatalf("entry fields = %+v", ent)
		}
		wantInstant(t, "created_at", ent.CreatedAt, at)
		wantInstant(t, "updated_at", ent.UpdatedAt, at)

		pd := e.postedRow(t, u, "2026-10-07")
		if pd == nil || pd.Source != spin.SourcePost || pd.Late {
			t.Fatalf("posted day = %+v, want a real post that is not late", pd)
		}
		row := e.spinRow(t, u, "2026-10-07")
		if row == nil || row.Result != spin.ResultPosted || row.SettledAt == nil {
			t.Fatalf("spin row = %+v, want posted and settled", row)
		}
		wantInstant(t, "settled_at", *row.SettledAt, at)

		tdy := out.Today
		if tdy.State != spin.StatePosted || tdy.Spin == nil || tdy.Spin.ID != sp.ID || tdy.Spin.Result != spin.ResultPosted {
			t.Fatalf("today = state %s spin %+v, want posted with the spin", tdy.State, tdy.Spin)
		}
		if tdy.Week.Posted != 1 || tdy.Bonus.CanUse {
			t.Fatalf("today week %+v bonus %+v, want one posted day and no bonus use", tdy.Week, tdy.Bonus)
		}
		wantSummary(t, tdy.Entry, ent.ID, "Dune", 7, str("fire"), in.Photos[0].Key, 2)
		if tdy.Entry.Late {
			t.Error("the summary says late")
		}
	})

	run("an edit shows in the next Today summary", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		in.Reaction = str("love")
		out := e.post(t, u, "2026-10-07", 0, in)
		id, keys, ids := out.Entry.ID, keysOf(in.Photos), idsOf(out.Entry.Photos)

		e.Clock.Advance(time.Hour)
		e.edit(t, u, id, spin.EntryPatch{Title: str("Dune, again"), Stars: num(9), Reaction: str("wow")})
		wantSummary(t, e.today(t, u).Entry, id, "Dune, again", 9, str("wow"), keys[0], 3)

		drop := keepRefs(ids[1], ids[2])
		e.edit(t, u, id, spin.EntryPatch{Photos: &drop})
		wantSummary(t, e.today(t, u).Entry, id, "Dune, again", 9, str("wow"), keys[1], 2)

		swap := keepRefs(ids[2], ids[1])
		e.edit(t, u, id, spin.EntryPatch{Photos: &swap})
		wantSummary(t, e.today(t, u).Entry, id, "Dune, again", 9, str("wow"), keys[2], 2)

		added := append(keepRefs(ids[2], ids[1]), spin.PhotoRef{Key: e.key(u), Source: spin.PhotoSourceCamera})
		e.edit(t, u, id, spin.EntryPatch{Photos: &added})
		wantSummary(t, e.today(t, u).Entry, id, "Dune, again", 9, str("wow"), keys[2], 3)
	})

	run("a post after a bonus spin belongs to the bonus spin", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		wantHeld(t, e.Env, u, 1)
		daily := e.openDay(t, u, "2026-10-09")
		bonus := e.bonus(t, u, 0, "2026-10-09").Today.Spin
		if bonus == nil || bonus.Seq != 1 || bonus.Category.ID == daily.Category.ID {
			t.Fatalf("bonus spin = %+v, want seq 1 on another category than %+v", bonus, daily.Category)
		}

		in := e.input(u, 1)
		_, err := e.tryPost(t, u, "2026-10-09", 0, in)
		wantErr(t, err, spin.ErrSpinChanged)
		e.wantNothingWritten(t, u, "2026-10-09", keysOf(in.Photos))

		out := e.post(t, u, "2026-10-09", 1, in)
		ent := out.Entry
		if ent.SpinID != bonus.ID || ent.CategoryID != bonus.Category.ID || ent.CategoryName != bonus.Category.Name {
			t.Fatalf("entry = %+v, want it on the bonus spin %+v", ent, bonus)
		}
		dailyRow := e.dailyRow(t, u, "2026-10-09")
		if dailyRow == nil || dailyRow.Result != spin.ResultOpen || dailyRow.SettledAt != nil {
			t.Fatalf("daily spin row = %+v, want it open for good", dailyRow)
		}
		latest := e.spinRow(t, u, "2026-10-09")
		if latest == nil || latest.Seq != 1 || latest.Result != spin.ResultPosted || latest.SettledAt == nil {
			t.Fatalf("latest spin row = %+v, want seq 1 posted", latest)
		}
		if out.Today.State != spin.StatePosted || out.Today.Spin == nil || out.Today.Spin.Seq != 1 {
			t.Fatalf("today = state %s spin %+v, want posted on seq 1", out.Today.State, out.Today.Spin)
		}
	})

	run("a rename after posting leaves the entry's category name as written", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		sp := e.openDay(t, u, "2026-10-07")
		out := e.post(t, u, "2026-10-07", 0, e.input(u, 1))

		w := e.wheel(t, u)
		list := make([]spin.WheelInput, len(w.Categories))
		for i, c := range w.Categories {
			id, name := c.ID, c.Name
			if id == sp.Category.ID {
				name = "Renamed " + name
			}
			list[i] = spin.WheelInput{ID: &id, Name: name, Weight: c.Weight}
		}
		if _, err := e.Service.SaveWheel(ctx, u, w.Version, list); err != nil {
			t.Fatalf("SaveWheel: %v", err)
		}
		got := e.get(t, u, out.Entry.ID)
		if got.CategoryID != sp.Category.ID || got.CategoryName != sp.Category.Name {
			t.Fatalf("category = %s %q, want %s %q as written on the day", got.CategoryID, got.CategoryName, sp.Category.ID, sp.Category.Name)
		}
	})

	run("the same title on two different dates makes two entries", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		ids := make(map[string]bool)
		for _, day := range []string{"2026-10-07", "2026-10-08"} {
			e.openDay(t, u, day)
			out := e.post(t, u, day, 0, e.input(u, 1))
			if out.Entry.Title != "Dune" || out.Entry.LocalDate != date(t, day) {
				t.Fatalf("entry on %s = %+v", day, out.Entry)
			}
			ids[out.Entry.ID] = true
		}
		if len(ids) != 2 {
			t.Fatalf("two dates gave %d entries, want 2", len(ids))
		}
	})

	run("the same ordered list again returns the same entry and any other list is ErrDaySettled", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		keys := keysOf(in.Photos)
		first := e.post(t, u, "2026-10-07", 0, in)

		e.Clock.Advance(time.Minute)
		again := e.post(t, u, "2026-10-07", 0, in)
		if !again.Already || again.Entry.ID != first.Entry.ID {
			t.Fatalf("retry: already %v id %s, want already and %s", again.Already, again.Entry.ID, first.Entry.ID)
		}
		wantInstant(t, "updated_at after a retry", again.Entry.UpdatedAt, first.Entry.UpdatedAt)
		wantPhotos(t, "photo rows after a retry", e.photoRows(t, u, first.Entry.ID), keys)
		if again.Today.State != spin.StatePosted || again.Today.Entry == nil || again.Today.Entry.ID != first.Entry.ID {
			t.Fatalf("retry today = %+v", again.Today)
		}

		reversed := e.input(u, 0)
		for i := len(in.Photos) - 1; i >= 0; i-- {
			reversed.Photos = append(reversed.Photos, in.Photos[i])
		}
		fewer := e.input(u, 0)
		fewer.Photos = in.Photos[:2]
		for name, other := range map[string]spin.EntryInput{
			"other keys":     e.input(u, 3),
			"fewer keys":     fewer,
			"another order":  reversed,
			"one new key":    e.input(u, 1),
			"one more photo": {Title: "Dune", Stars: 7, Photos: append(append([]spin.NewPhoto(nil), in.Photos...), e.photos(u, 1)...)},
		} {
			_, err := e.tryPost(t, u, "2026-10-07", 0, other)
			if !errors.Is(err, spin.ErrDaySettled) {
				t.Errorf("%s: error = %v, want ErrDaySettled", name, err)
			}
		}
		wantPhotos(t, "photo rows after refused posts", e.photoRows(t, u, first.Entry.ID), keys)
	})

	run("a wrong spin number is ErrSpinChanged and writes nothing", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 1)
		for _, seq := range []int{1, 2, -1} {
			_, err := e.tryPost(t, u, "2026-10-07", seq, in)
			wantErr(t, err, spin.ErrSpinChanged)
			e.wantNothingWritten(t, u, "2026-10-07", keysOf(in.Photos))
		}
		e.post(t, u, "2026-10-07", 0, in)
	})

	run("no spin and a future date are ErrNoSpin", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 20, 30, 0)
		in := e.input(u, 1)
		for _, day := range []string{"2026-10-07", "2026-10-08", "2026-10-06", "2026-12-25"} {
			_, err := e.tryPost(t, u, day, 0, in)
			if !errors.Is(err, spin.ErrNoSpin) {
				t.Errorf("post for %s: %v, want ErrNoSpin", day, err)
			}
			_, err = e.Service.SkipDay(ctx, u, date(t, day), 0)
			if !errors.Is(err, spin.ErrNoSpin) {
				t.Errorf("skip for %s: %v, want ErrNoSpin", day, err)
			}
		}
		e.wantNothingWritten(t, u, "2026-10-07", keysOf(in.Photos))
		wantSpinRows(t, e.Env, u, 0)
	})

	run("a post at exactly late_until is ErrClosed and one nanosecond earlier is accepted as late", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		w := spin.WindowFor(date(t, "2026-10-07"), zone(t, ny))
		in := e.input(u, 1)

		e.Clock.Set(w.LateUntil)
		_, err := e.tryPost(t, u, "2026-10-07", 0, in)
		wantErr(t, err, spin.ErrClosed)
		e.wantNothingWritten(t, u, "2026-10-07", keysOf(in.Photos))

		e.Clock.Set(w.LateUntil.Add(-time.Nanosecond))
		out := e.post(t, u, "2026-10-07", 0, in)
		if !out.Entry.Late {
			t.Error("the entry is not late")
		}
		pd := e.postedRow(t, u, "2026-10-07")
		if pd == nil || !pd.Late {
			t.Fatalf("posted day = %+v, want late", pd)
		}
		row := e.spinRow(t, u, "2026-10-07")
		if row == nil || row.Result != spin.ResultPosted || row.SettledAt == nil || row.SettledAt.Before(w.ClosesAt) {
			t.Fatalf("spin row = %+v, want posted and settled at or after the close %v", row, w.ClosesAt)
		}
	})

	run("four real posts grant one bonus, a fifth none, and a paid week pays no more", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		postDay := func(day string, seq int) spin.Today {
			t.Helper()
			e.openDay(t, u, day)
			if seq > 0 {
				e.bonus(t, u, 0, day)
			}
			return e.post(t, u, day, seq, e.input(u, 1)).Today
		}

		// Week A, Monday to Friday.
		for i, day := range []string{"2026-10-05", "2026-10-06", "2026-10-07"} {
			if tdy := postDay(day, 0); tdy.Week.Posted != i+1 {
				t.Fatalf("%s: week posted = %d, want %d", day, tdy.Week.Posted, i+1)
			}
			wantHeld(t, e.Env, u, 0)
		}
		tdy := postDay("2026-10-08", 0)
		if tdy.Week.Posted != 4 || !tdy.Week.GoalMet || !tdy.Bonus.Held {
			t.Fatalf("fourth post: week %+v bonus %+v, want the goal met and a bonus held", tdy.Week, tdy.Bonus)
		}
		wantHeld(t, e.Env, u, 1)
		tdy = postDay("2026-10-09", 0)
		if tdy.Week.Posted != 5 {
			t.Fatalf("fifth post: week posted = %d, want 5", tdy.Week.Posted)
		}
		wantHeld(t, e.Env, u, 1)

		// Week B hits its goal while one is held: no second bonus. Spending
		// the held one and posting a fifth day pays nothing more.
		for _, day := range []string{"2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15"} {
			tdy = postDay(day, 0)
		}
		if tdy.Week.Posted != 4 || !tdy.Week.GoalMet {
			t.Fatalf("week B: %+v", tdy.Week)
		}
		wantHeld(t, e.Env, u, 1)
		tdy = postDay("2026-10-16", 1)
		if tdy.Week.Posted != 5 {
			t.Fatalf("week B fifth post: week posted = %d, want 5", tdy.Week.Posted)
		}
		wantHeld(t, e.Env, u, 0)

		// Week C pays again.
		for _, day := range []string{"2026-10-19", "2026-10-20", "2026-10-21", "2026-10-22"} {
			postDay(day, 0)
		}
		wantHeld(t, e.Env, u, 1)
	})

	run("a late post just after midnight on a Monday counts for Sunday's week", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-11", 12, 0, 0)
		if _, err := e.Service.MarkTestDays(ctx, u, []spin.Date{date(t, "2026-10-05"), date(t, "2026-10-06"), date(t, "2026-10-07")}); err != nil {
			t.Fatal(err)
		}
		wantHeld(t, e.Env, u, 0)
		e.openDay(t, u, "2026-10-11")
		e.setClock(t, ny, "2026-10-12", 0, 30, 0)

		out := e.post(t, u, "2026-10-11", 0, e.input(u, 1))
		if !out.Entry.Late {
			t.Error("the entry is not late")
		}
		// Sunday's week reached its goal: the fourth day of that week.
		wantHeld(t, e.Env, u, 1)
		if out.Today.Week.Start != date(t, "2026-10-12") || out.Today.Week.Posted != 0 {
			t.Fatalf("today week = %+v, want the new week with nothing counted", out.Today.Week)
		}
		if out.Today.State != spin.StatePosted || out.Today.Spin == nil || out.Today.Spin.LocalDate != date(t, "2026-10-11") {
			t.Fatalf("today = state %s spin %+v, want posted with Sunday's spin", out.Today.State, out.Today.Spin)
		}

		// A skip made at the same hour settles Sunday's spin, not Monday's.
		u2 := e.ready(t, ny)
		e.openDay(t, u2, "2026-10-11")
		e.setClock(t, ny, "2026-10-12", 0, 30, 0)
		sk := e.skip(t, u2, "2026-10-11", 0)
		if sk.Today.State != spin.StateSkipped || sk.Today.Spin == nil || sk.Today.Spin.LocalDate != date(t, "2026-10-11") {
			t.Fatalf("skip today = state %s spin %+v", sk.Today.State, sk.Today.Spin)
		}
		if flags := skippedFlags(sk.Today.Week); len(flags) != 0 {
			t.Fatalf("the new week flags %v as skipped, want none (Sunday is last week)", flags)
		}
		row := e.spinRow(t, u2, "2026-10-11")
		w := spin.WindowFor(date(t, "2026-10-11"), zone(t, ny))
		if row == nil || row.Result != spin.ResultSkipped || row.SettledAt == nil || row.SettledAt.Before(w.ClosesAt) {
			t.Fatalf("spin row = %+v, want skipped and settled at or after the close", row)
		}
		if pd := e.postedRow(t, u2, "2026-10-11"); pd != nil {
			t.Fatalf("a skip wrote a posted day: %+v", pd)
		}
	})

	run("a pretend test posted day makes a real post ErrDaySettled", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		if _, err := e.Service.MarkPosted(ctx, u, date(t, "2026-10-07"), spin.SourceTest); err != nil {
			t.Fatal(err)
		}
		in := e.input(u, 1)
		_, err := e.tryPost(t, u, "2026-10-07", 0, in)
		wantErr(t, err, spin.ErrDaySettled)
		if got := e.entryOn(t, u, "2026-10-07"); got != nil {
			t.Fatalf("an entry was written: %+v", got)
		}
		if used := e.keysUsed(t, u, keysOf(in.Photos)); len(used) != 0 {
			t.Fatalf("photo rows were written for %v", used)
		}
		_, err = e.Service.SkipDay(ctx, u, date(t, "2026-10-07"), 0)
		wantErr(t, err, spin.ErrDaySettled)
	})

	// --- skipping ---

	run("a skip settles the day and writes no posted day", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		sp := e.openDay(t, u, "2026-10-09")
		at := e.Clock.NowFor(u)
		before := e.today(t, u).Week.Posted

		out := e.skip(t, u, "2026-10-09", 0)
		if out.Already {
			t.Fatal("the first skip says already")
		}
		tdy := out.Today
		if tdy.State != spin.StateSkipped || tdy.Spin == nil || tdy.Spin.ID != sp.ID || tdy.Spin.Result != spin.ResultSkipped {
			t.Fatalf("today = state %s spin %+v, want skipped on the spin", tdy.State, tdy.Spin)
		}
		if tdy.Entry != nil || tdy.Bonus.CanUse || tdy.Week.Posted != before {
			t.Fatalf("today entry %+v bonus %+v week posted %d (was %d)", tdy.Entry, tdy.Bonus, tdy.Week.Posted, before)
		}
		flags := skippedFlags(tdy.Week)
		if len(flags) != 1 || flags[0] != date(t, "2026-10-09") || tdy.Week.Days[tdy.Week.TodayIndex].Posted {
			t.Fatalf("week day flags = %v, want only 2026-10-09 skipped and not posted", flags)
		}
		row := e.spinRow(t, u, "2026-10-09")
		if row == nil || row.Result != spin.ResultSkipped || row.SettledAt == nil {
			t.Fatalf("spin row = %+v, want skipped and settled", row)
		}
		wantInstant(t, "settled_at", *row.SettledAt, at)
		if pd := e.postedRow(t, u, "2026-10-09"); pd != nil {
			t.Fatalf("a skip wrote a posted day: %+v", pd)
		}

		// A held bonus cannot be spent on a skipped day.
		_, err := e.Service.Bonus(ctx, u, 0, date(t, "2026-10-09"))
		wantErr(t, err, spin.ErrDaySettled)
		wantHeld(t, e.Env, u, 1)

		// A repeat is the same answer and changes nothing.
		e.Clock.Advance(time.Minute)
		again := e.skip(t, u, "2026-10-09", 0)
		if !again.Already || again.Today.State != spin.StateSkipped {
			t.Fatalf("repeat skip: already %v state %s", again.Already, again.Today.State)
		}
		row = e.spinRow(t, u, "2026-10-09")
		wantInstant(t, "settled_at after a repeat", *row.SettledAt, at)

		// A post after a skip is refused.
		in := e.input(u, 1)
		_, err = e.tryPost(t, u, "2026-10-09", 0, in)
		wantErr(t, err, spin.ErrDaySettled)
		e.wantNoEntry(t, u, "2026-10-09", keysOf(in.Photos))
		if got := e.spinRow(t, u, "2026-10-09"); got.Result != spin.ResultSkipped {
			t.Fatalf("spin result = %s after a refused post", got.Result)
		}
	})

	run("a skip after a post is ErrDaySettled and leaves the entry", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		out := e.post(t, u, "2026-10-07", 0, e.input(u, 1))
		_, err := e.Service.SkipDay(ctx, u, date(t, "2026-10-07"), 0)
		wantErr(t, err, spin.ErrDaySettled)
		if got := e.entryOn(t, u, "2026-10-07"); got == nil || got.ID != out.Entry.ID {
			t.Fatalf("entry after a refused skip = %+v", got)
		}
		if row := e.spinRow(t, u, "2026-10-07"); row.Result != spin.ResultPosted {
			t.Fatalf("spin result = %s, want posted", row.Result)
		}
	})

	run("a skip at late_until is ErrClosed, one nanosecond earlier is accepted, a wrong spin number is refused", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		w := spin.WindowFor(date(t, "2026-10-07"), zone(t, ny))

		e.Clock.Set(w.LateUntil)
		_, err := e.Service.SkipDay(ctx, u, date(t, "2026-10-07"), 0)
		wantErr(t, err, spin.ErrClosed)
		if row := e.spinRow(t, u, "2026-10-07"); row.Result != spin.ResultOpen || row.SettledAt != nil {
			t.Fatalf("spin row = %+v after a refused skip", row)
		}

		e.Clock.Set(w.LateUntil.Add(-time.Nanosecond))
		_, err = e.Service.SkipDay(ctx, u, date(t, "2026-10-07"), 1)
		wantErr(t, err, spin.ErrSpinChanged)
		out := e.skip(t, u, "2026-10-07", 0)
		if out.Already || out.Today.State != spin.StateSkipped {
			t.Fatalf("skip: already %v state %s", out.Already, out.Today.State)
		}
		row := e.spinRow(t, u, "2026-10-07")
		if row.Result != spin.ResultSkipped || row.SettledAt == nil || row.SettledAt.Before(w.ClosesAt) {
			t.Fatalf("spin row = %+v, want skipped and settled at or after the close", row)
		}
	})

	run("bonus then skip leaves the daily spin open, is not late next morning and flags the day once", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		e.postWeekDays(t, u, "2026-10-08")
		e.openDay(t, u, "2026-10-09")
		b := e.bonus(t, u, 0, "2026-10-09").Today.Spin
		if b == nil || b.Seq != 1 {
			t.Fatalf("bonus spin = %+v", b)
		}
		out := e.skip(t, u, "2026-10-09", 1)
		if out.Today.State != spin.StateSkipped || out.Today.Spin == nil || out.Today.Spin.Seq != 1 {
			t.Fatalf("today = state %s spin %+v, want skipped on seq 1", out.Today.State, out.Today.Spin)
		}
		daily := e.dailyRow(t, u, "2026-10-09")
		if daily == nil || daily.Result != spin.ResultOpen || daily.SettledAt != nil {
			t.Fatalf("daily spin row = %+v, want it open for good", daily)
		}
		latest := e.spinRow(t, u, "2026-10-09")
		if latest == nil || latest.Seq != 1 || latest.Result != spin.ResultSkipped {
			t.Fatalf("latest spin row = %+v, want seq 1 skipped", latest)
		}

		for _, hm := range [][2]int{{0, 30}, {9, 0}} {
			e.setClock(t, ny, "2026-10-10", hm[0], hm[1], 0)
			got := e.today(t, u)
			if got.State != spin.StateWaiting || got.Spin != nil || got.Entry != nil {
				t.Fatalf("%02d:%02d next day: state %s spin %+v, want waiting and none", hm[0], hm[1], got.State, got.Spin)
			}
			if flags := skippedFlags(got.Week); len(flags) != 1 || flags[0] != date(t, "2026-10-09") {
				t.Fatalf("%02d:%02d next day: skipped flags %v, want 2026-10-09 once", hm[0], hm[1], flags)
			}
		}
	})

	// --- editing ---

	run("an edit changes the scalar fields and updated_at and nothing else", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		in.Reaction, in.Comment = str("fire"), str("Loved it.")
		before := e.post(t, u, "2026-10-07", 0, in).Entry
		pdBefore := e.postedRow(t, u, "2026-10-07")
		spBefore := e.spinRow(t, u, "2026-10-07")

		e.Clock.Advance(2 * time.Hour)
		res := e.edit(t, u, before.ID, spin.EntryPatch{
			Title: str("Dune, part two"), Stars: num(9), Reaction: str("wow"), Comment: str("Better the second time."),
		})
		got := res.Entry
		if got.Title != "Dune, part two" || got.Stars != 9 || got.Reaction == nil || *got.Reaction != "wow" ||
			got.Comment == nil || *got.Comment != "Better the second time." {
			t.Fatalf("entry after the edit = %+v", got)
		}
		if !got.UpdatedAt.After(before.UpdatedAt) {
			t.Errorf("updated_at = %v, want after %v", got.UpdatedAt, before.UpdatedAt)
		}
		wantInstant(t, "created_at", got.CreatedAt, before.CreatedAt)
		if got.ID != before.ID || got.SpinID != before.SpinID || got.LocalDate != before.LocalDate ||
			got.CategoryID != before.CategoryID || got.CategoryName != before.CategoryName || got.Late != before.Late {
			t.Errorf("an edit changed more than the fields: %+v -> %+v", before, got)
		}
		if len(res.Dropped) != 0 {
			t.Errorf("dropped = %v, want none", res.Dropped)
		}
		wantPhotos(t, "photos after a scalar edit", got.Photos, keysOf(in.Photos))
		if fmt.Sprint(idsOf(got.Photos)) != fmt.Sprint(idsOf(before.Photos)) {
			t.Errorf("photo ids changed: %v -> %v", idsOf(before.Photos), idsOf(got.Photos))
		}
		pd := e.postedRow(t, u, "2026-10-07")
		if pd == nil || pd.Date != pdBefore.Date || pd.Source != pdBefore.Source || pd.Late != pdBefore.Late {
			t.Errorf("posted day = %+v, was %+v", pd, pdBefore)
		}
		sp := e.spinRow(t, u, "2026-10-07")
		if sp.Result != spin.ResultPosted || sp.SettledAt == nil {
			t.Fatalf("spin row = %+v after an edit", sp)
		}
		wantInstant(t, "settled_at after an edit", *sp.SettledAt, *spBefore.SettledAt)
	})

	run("an empty patch, or one that repeats the stored values, writes nothing", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 2)
		in.Reaction = str("sad")
		before := e.post(t, u, "2026-10-07", 0, in).Entry

		e.Clock.Advance(time.Hour)
		res := e.edit(t, u, before.ID, spin.EntryPatch{})
		wantInstant(t, "updated_at after an empty patch", res.Entry.UpdatedAt, before.UpdatedAt)
		if len(res.Dropped) != 0 {
			t.Errorf("dropped = %v, want none", res.Dropped)
		}

		same := keepRefs(idsOf(before.Photos)...)
		res = e.edit(t, u, before.ID, spin.EntryPatch{Title: str(before.Title), Stars: num(before.Stars), Reaction: str("sad"), Photos: &same})
		wantInstant(t, "updated_at after a patch of the same values", res.Entry.UpdatedAt, before.UpdatedAt)
		wantPhotos(t, "photos", res.Entry.Photos, keysOf(in.Photos))
	})

	run("empty strings clear the reaction and the comment", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 1)
		in.Reaction, in.Comment = str("fire"), str("Nice.")
		id := e.post(t, u, "2026-10-07", 0, in).Entry.ID

		e.Clock.Advance(time.Minute)
		res := e.edit(t, u, id, spin.EntryPatch{Reaction: str(""), Comment: str("")})
		if res.Entry.Reaction != nil || res.Entry.Comment != nil {
			t.Fatalf("after clearing: reaction %v comment %v, want none", res.Entry.Reaction, res.Entry.Comment)
		}
		if got := e.get(t, u, id); got.Reaction != nil || got.Comment != nil {
			t.Fatalf("read after clearing: reaction %v comment %v", got.Reaction, got.Comment)
		}
		res = e.edit(t, u, id, spin.EntryPatch{Reaction: str("sleepy")})
		if res.Entry.Reaction == nil || *res.Entry.Reaction != "sleepy" || res.Entry.Comment != nil {
			t.Fatalf("after setting a reaction: %+v", res.Entry)
		}
		if got := e.today(t, u).Entry; got == nil || got.Reaction == nil || *got.Reaction != "sleepy" {
			t.Fatalf("today summary = %+v", got)
		}
	})

	run("an edit adds, removes, replaces and reorders photos and keeps positions packed", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		a, b, c := in.Photos[0].Key, in.Photos[1].Key, in.Photos[2].Key
		out := e.post(t, u, "2026-10-07", 0, in)
		id := out.Entry.ID
		ids := idsOf(out.Entry.Photos)
		e.Clock.Advance(time.Minute)

		// Every photo change is a change: it bumps updated_at.
		lastUpdated := out.Entry.UpdatedAt
		bumped := func(label string, got spin.Entry) {
			t.Helper()
			if !got.UpdatedAt.After(lastUpdated) {
				t.Fatalf("%s: updated_at = %v, want after %v", label, got.UpdatedAt, lastUpdated)
			}
			lastUpdated = got.UpdatedAt
			e.Clock.Advance(time.Minute)
		}

		// A photo needs a source.
		noSource := append(keepRefs(ids...), spin.PhotoRef{Key: e.key(u)})
		_, err := e.Service.EditEntry(ctx, u, id, spin.EntryPatch{Photos: &noSource})
		wantProblem(t, err, "photos", spin.ProblemInvalid)
		wantPhotos(t, "after a refused edit", e.photoRows(t, u, id), []string{a, b, c})

		// Add one at the end.
		n1 := e.key(u)
		add := append(keepRefs(ids...), spin.PhotoRef{Key: n1, Source: spin.PhotoSourceLibrary, SizeBytes: num(1024)})
		res := e.edit(t, u, id, spin.EntryPatch{Photos: &add})
		wantPhotos(t, "after an add", res.Entry.Photos, []string{a, b, c, n1})
		bumped("an add", res.Entry)
		if len(res.Dropped) != 0 {
			t.Errorf("an add dropped %v", res.Dropped)
		}
		if fmt.Sprint(idsOf(res.Entry.Photos)[:3]) != fmt.Sprint(ids) {
			t.Errorf("kept photos changed id: %v -> %v", ids, idsOf(res.Entry.Photos))
		}
		last := res.Entry.Photos[3]
		if last.ContentType != "image/jpeg" || last.Source != spin.PhotoSourceLibrary || last.SizeBytes == nil || *last.SizeBytes != 1024 {
			t.Errorf("the added photo = %+v", last)
		}
		idsNow := idsOf(res.Entry.Photos)

		// Remove the middle one.
		remove := keepRefs(idsNow[0], idsNow[2], idsNow[3])
		res = e.edit(t, u, id, spin.EntryPatch{Photos: &remove})
		wantPhotos(t, "after a removal", res.Entry.Photos, []string{a, c, n1})
		bumped("a removal", res.Entry)
		wantSameKeys(t, "dropped by a removal", res.Dropped, []string{b})
		if used := e.keysUsed(t, u, []string{b}); len(used) != 0 {
			t.Errorf("the removed key is still held: %v", used)
		}
		idsNow = idsOf(res.Entry.Photos)

		// Reorder: a swap passes.
		reorder := keepRefs(idsNow[2], idsNow[1], idsNow[0])
		res = e.edit(t, u, id, spin.EntryPatch{Photos: &reorder})
		wantPhotos(t, "after a reorder", res.Entry.Photos, []string{n1, c, a})
		bumped("a reorder", res.Entry)
		if len(res.Dropped) != 0 {
			t.Errorf("a reorder dropped %v", res.Dropped)
		}
		idsNow = idsOf(res.Entry.Photos)

		// Replace one: the old key is dropped, the new one takes its place.
		n2 := e.key(u)
		replace := []spin.PhotoRef{{ID: idsNow[0]}, {Key: n2, Source: spin.PhotoSourceCamera}, {ID: idsNow[2]}}
		res = e.edit(t, u, id, spin.EntryPatch{Photos: &replace})
		wantPhotos(t, "after a replace", res.Entry.Photos, []string{n1, n2, a})
		bumped("a replace", res.Entry)
		wantSameKeys(t, "dropped by a replace", res.Dropped, []string{c})
		wantPhotos(t, "stored rows", e.photoRows(t, u, id), []string{n1, n2, a})
		if used := e.keysUsed(t, u, []string{n1, n2, a, c}); len(used) != 3 || used[c] {
			t.Errorf("keys held = %v, want n1, n2 and a only", used)
		}
		wantSummary(t, e.today(t, u).Entry, id, "Dune", 7, nil, n1, 3)
	})

	run("an edit that would leave no photos or more than ten is refused and writes nothing", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 10)
		out := e.post(t, u, "2026-10-07", 0, in)
		id, keys := out.Entry.ID, keysOf(in.Photos)
		e.Clock.Advance(time.Minute)

		none := []spin.PhotoRef{}
		_, err := e.Service.EditEntry(ctx, u, id, spin.EntryPatch{Title: str("Changed"), Photos: &none})
		wantProblem(t, err, "photos", spin.ProblemRequired)

		eleven := append(keepRefs(idsOf(out.Entry.Photos)...), spin.PhotoRef{Key: e.key(u), Source: spin.PhotoSourceCamera})
		_, err = e.Service.EditEntry(ctx, u, id, spin.EntryPatch{Title: str("Changed"), Photos: &eleven})
		wantProblem(t, err, "photos", spin.ProblemTooMany)

		got := e.get(t, u, id)
		if got.Title != "Dune" {
			t.Fatalf("title = %q after refused edits, want Dune", got.Title)
		}
		wantInstant(t, "updated_at", got.UpdatedAt, out.Entry.UpdatedAt)
		wantPhotos(t, "photos", got.Photos, keys)
	})

	run("an edit refuses a foreign photo id, a key in use and a repeat, and writes nothing", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		other := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		first := e.post(t, u, "2026-10-07", 0, e.input(u, 2))
		e.openDay(t, u, "2026-10-08")
		second := e.post(t, u, "2026-10-08", 0, e.input(u, 1))
		e.openDay(t, other, "2026-10-08")
		foreign := e.post(t, other, "2026-10-08", 0, e.input(other, 1))
		e.Clock.Advance(time.Minute)

		own := idsOf(first.Entry.Photos)
		ownKeys := keysOfPhotos(first.Entry.Photos)
		gifKey := strings.TrimSuffix(e.key(u), ".jpg") + ".gif"
		cases := []struct {
			name string
			list []spin.PhotoRef
		}{
			{"a photo of another entry", append(keepRefs(own[0]), spin.PhotoRef{ID: second.Entry.Photos[0].ID})},
			{"a photo of another person", append(keepRefs(own[0]), spin.PhotoRef{ID: foreign.Entry.Photos[0].ID})},
			{"a key held by another entry", append(keepRefs(own[0]), spin.PhotoRef{Key: second.Entry.Photos[0].Key, Source: spin.PhotoSourceLibrary})},
			{"a key held by this entry as a new key", append(keepRefs(own[0]), spin.PhotoRef{Key: ownKeys[1], Source: spin.PhotoSourceLibrary})},
			{"an id that is not an id", []spin.PhotoRef{{ID: "not-a-uuid"}}},
			{"an id twice", keepRefs(own[0], own[0])},
			{"an id and its upper-case spelling", keepRefs(own[0], strings.ToUpper(own[0]))},
			{"a new key with an unknown extension", append(keepRefs(own[0]), spin.PhotoRef{Key: gifKey, Source: spin.PhotoSourceCamera})},
		}
		for _, tc := range cases {
			list := tc.list
			_, err := e.Service.EditEntry(ctx, u, first.Entry.ID, spin.EntryPatch{Title: str("Changed"), Photos: &list})
			var fp *spin.FieldProblem
			if !errors.As(err, &fp) || fp.Field != "photos" || fp.Reason != spin.ProblemInvalid {
				t.Errorf("%s: error = %v, want a photos invalid problem", tc.name, err)
			}
		}
		got := e.get(t, u, first.Entry.ID)
		if got.Title != "Dune" {
			t.Fatalf("title = %q after refused edits, want Dune", got.Title)
		}
		wantInstant(t, "updated_at", got.UpdatedAt, first.Entry.UpdatedAt)
		wantPhotos(t, "photos", got.Photos, ownKeys)
		wantPhotos(t, "the other entry's photos", e.photoRows(t, u, second.Entry.ID), keysOfPhotos(second.Entry.Photos))
		wantPhotos(t, "the other person's photos", e.photoRows(t, other, foreign.Entry.ID), keysOfPhotos(foreign.Entry.Photos))
	})

	run("an edit works after the day has closed", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		before := e.post(t, u, "2026-10-07", 0, e.input(u, 1)).Entry
		e.setClock(t, ny, "2026-10-20", 12, 0, 0)
		res := e.edit(t, u, before.ID, spin.EntryPatch{Title: str("Dune, years later"), Stars: num(10)})
		if res.Entry.Title != "Dune, years later" || res.Entry.Stars != 10 {
			t.Fatalf("entry = %+v", res.Entry)
		}
		if res.Entry.LocalDate != before.LocalDate || res.Entry.Late != before.Late || res.Entry.CategoryID != before.CategoryID {
			t.Errorf("an old edit changed more than the fields: %+v -> %+v", before, res.Entry)
		}
		if !res.Entry.UpdatedAt.After(before.UpdatedAt) {
			t.Errorf("updated_at = %v, want after %v", res.Entry.UpdatedAt, before.UpdatedAt)
		}
	})

	run("another person's entry, a deleted entry and a bad id are not found", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		other := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		out := e.post(t, u, "2026-10-07", 0, e.input(u, 1))
		id := out.Entry.ID

		check := func(label, who, entryID string) {
			t.Helper()
			if _, err := e.Service.GetEntry(ctx, who, entryID); !errors.Is(err, spin.ErrEntryNotFound) {
				t.Errorf("%s: GetEntry = %v, want ErrEntryNotFound", label, err)
			}
			if _, err := e.Service.EditEntry(ctx, who, entryID, spin.EntryPatch{Title: str("Changed")}); !errors.Is(err, spin.ErrEntryNotFound) {
				t.Errorf("%s: EditEntry = %v, want ErrEntryNotFound", label, err)
			}
			if _, err := e.Service.DeleteEntry(ctx, who, entryID); !errors.Is(err, spin.ErrEntryNotFound) {
				t.Errorf("%s: DeleteEntry = %v, want ErrEntryNotFound", label, err)
			}
		}
		e.setClock(t, ny, "2026-10-07", 21, 0, 0)
		// Someone else's person cannot read, change or delete it.
		check("another person", other, id)
		for _, bad := range []string{"", "x", id[:35], "{" + id + "}", id + "0", strings.ReplaceAll(id, "-", "")} {
			check(fmt.Sprintf("bad id %q", bad), u, bad)
		}
		if got := e.get(t, u, id); got.Title != "Dune" || len(got.Photos) != 1 {
			t.Fatalf("the entry changed: %+v", got)
		}

		e.remove(t, u, id)
		check("a deleted entry", u, id)
	})

	// --- deleting ---

	run("a delete removes the entry and its photos and keeps the posted day and the spin result", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		sp := e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		keys := keysOf(in.Photos)
		out := e.post(t, u, "2026-10-07", 0, in)
		id := out.Entry.ID
		pdBefore := e.postedRow(t, u, "2026-10-07")

		res := e.remove(t, u, id)
		wantSameKeys(t, "dropped by a delete", res.Dropped, keys)
		if res.Today.State != spin.StatePosted || res.Today.Entry != nil || res.Today.Spin == nil || res.Today.Spin.ID != sp.ID {
			t.Fatalf("today after a delete = state %s entry %+v spin %+v", res.Today.State, res.Today.Entry, res.Today.Spin)
		}
		if res.Today.Week.Posted != 1 {
			t.Fatalf("week posted = %d after a delete, want 1 (a deleted post still counts)", res.Today.Week.Posted)
		}

		if got := e.entryOn(t, u, "2026-10-07"); got != nil {
			t.Fatalf("entry row after a delete: %+v", got)
		}
		if _, missing := e.entryRow(t, u, id); !errors.Is(missing, spin.ErrEntryNotFound) {
			t.Fatalf("EntryByID after a delete: %v, want ErrEntryNotFound", missing)
		}
		if rows := e.photoRows(t, u, id); len(rows) != 0 {
			t.Fatalf("photo rows after a delete: %+v", rows)
		}
		if used := e.keysUsed(t, u, keys); len(used) != 0 {
			t.Fatalf("keys still held after a delete: %v", used)
		}
		pd := e.postedRow(t, u, "2026-10-07")
		if pd == nil || pd.Source != spin.SourcePost || pd.Late != pdBefore.Late {
			t.Fatalf("posted day after a delete = %+v, want it kept", pd)
		}
		if row := e.spinRow(t, u, "2026-10-07"); row.Result != spin.ResultPosted || row.SettledAt == nil {
			t.Fatalf("spin row after a delete = %+v, want posted and settled", row)
		}

		// The day cannot be posted again, with the old keys or new ones.
		_, err := e.tryPost(t, u, "2026-10-07", 0, in)
		wantErr(t, err, spin.ErrDaySettled)
		_, err = e.tryPost(t, u, "2026-10-07", 0, e.input(u, 1))
		wantErr(t, err, spin.ErrDaySettled)
		if got := e.entryOn(t, u, "2026-10-07"); got != nil {
			t.Fatalf("a refused post wrote an entry: %+v", got)
		}

		// A second delete and a later read find nothing.
		_, err = e.Service.DeleteEntry(ctx, u, id)
		wantErr(t, err, spin.ErrEntryNotFound)
		_, err = e.Service.GetEntry(ctx, u, id)
		wantErr(t, err, spin.ErrEntryNotFound)
	})

	run("after a delete Today is still posted with a null entry, also in the late window", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		id := e.post(t, u, "2026-10-07", 0, e.input(u, 2)).Entry.ID
		e.remove(t, u, id)
		got := e.today(t, u)
		if got.State != spin.StatePosted || got.Entry != nil || got.Spin == nil {
			t.Fatalf("same evening: state %s entry %+v spin %+v", got.State, got.Entry, got.Spin)
		}

		// A late post, then a delete, still shows last night's spin as posted.
		u2 := e.ready(t, ny)
		e.openDay(t, u2, "2026-10-07")
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		id2 := e.post(t, u2, "2026-10-07", 0, e.input(u2, 1)).Entry.ID
		res := e.remove(t, u2, id2)
		if res.Today.State != spin.StatePosted || res.Today.Entry != nil || res.Today.Spin == nil || res.Today.Spin.LocalDate != date(t, "2026-10-07") {
			t.Fatalf("late delete: state %s entry %+v spin %+v", res.Today.State, res.Today.Entry, res.Today.Spin)
		}
	})

	// --- the late window as the screen shows it ---

	run("the late window shows posted and skipped until the next opening", func(t *testing.T, e *EntryEnv) {
		// A late post: late, then posted with last night's spin until 8 PM.
		u := e.ready(t, ny)
		sp := e.openDay(t, u, "2026-10-07")
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		if got := e.today(t, u); got.State != spin.StateLate || got.Spin == nil || got.Spin.ID != sp.ID {
			t.Fatalf("00:30: state %s spin %+v, want late with last night's spin", got.State, got.Spin)
		}
		out := e.post(t, u, "2026-10-07", 0, e.input(u, 2))
		if !out.Entry.Late {
			t.Error("the late entry is not late")
		}
		if out.Today.State != spin.StatePosted || out.Today.Spin == nil || out.Today.Spin.ID != sp.ID {
			t.Fatalf("after the late post: state %s spin %+v", out.Today.State, out.Today.Spin)
		}
		wantSummary(t, out.Today.Entry, out.Entry.ID, "Dune", 7, nil, out.Entry.Photos[0].Key, 2)
		if !out.Today.Entry.Late {
			t.Error("the summary does not say late")
		}
		if out.Today.Bonus.CanUse {
			t.Error("a bonus can be used on a posted day")
		}
		e.setClock(t, ny, "2026-10-08", 12, 0, 0)
		if got := e.today(t, u); got.State != spin.StatePosted || got.Entry == nil || got.Spin == nil || got.Spin.ID != sp.ID {
			t.Fatalf("noon: state %s entry %+v spin %+v, want posted", got.State, got.Entry, got.Spin)
		}
		e.setClock(t, ny, "2026-10-08", 19, 59, 59)
		if got := e.today(t, u); got.State != spin.StatePosted {
			t.Fatalf("19:59:59: state %s, want posted", got.State)
		}
		e.setClock(t, ny, "2026-10-08", 20, 0, 0)
		if got := e.today(t, u); got.State != spin.StateOpen || got.Spin != nil || got.Entry != nil {
			t.Fatalf("20:00: state %s spin %+v entry %+v, want open and none", got.State, got.Spin, got.Entry)
		}

		// A post made before midnight is not shown after it.
		u2 := e.ready(t, ny)
		e.openDay(t, u2, "2026-10-07")
		e.post(t, u2, "2026-10-07", 0, e.input(u2, 1))
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		if got := e.today(t, u2); got.State != spin.StateWaiting || got.Spin != nil || got.Entry != nil {
			t.Fatalf("posted before midnight: state %s spin %+v entry %+v, want waiting", got.State, got.Spin, got.Entry)
		}

		// A skip after midnight is shown as skipped until 8 PM.
		u3 := e.ready(t, ny)
		sp3 := e.openDay(t, u3, "2026-10-07")
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		sk := e.skip(t, u3, "2026-10-07", 0)
		if sk.Today.State != spin.StateSkipped || sk.Today.Spin == nil || sk.Today.Spin.ID != sp3.ID || sk.Today.Entry != nil {
			t.Fatalf("late skip: state %s spin %+v entry %+v", sk.Today.State, sk.Today.Spin, sk.Today.Entry)
		}
		e.setClock(t, ny, "2026-10-08", 20, 0, 0)
		if got := e.today(t, u3); got.State != spin.StateOpen {
			t.Fatalf("20:00 after a late skip: state %s, want open", got.State)
		}

		// A pretend posted day has no settled time and shows as waiting.
		u4 := e.ready(t, ny)
		e.openDay(t, u4, "2026-10-07")
		e.setClock(t, ny, "2026-10-08", 0, 30, 0)
		if _, err := e.Service.MarkPosted(ctx, u4, date(t, "2026-10-07"), spin.SourceTest); err != nil {
			t.Fatal(err)
		}
		if got := e.today(t, u4); got.State != spin.StateWaiting || got.Spin != nil || got.Entry != nil {
			t.Fatalf("pretend day: state %s spin %+v entry %+v, want waiting", got.State, got.Spin, got.Entry)
		}
	})

	// --- the test reset ---

	run("the reset removes entries and every posted day and spin and goal week", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.setClock(t, ny, "2026-10-07", 12, 0, 0)
		if _, err := e.Service.MarkTestDays(ctx, u, []spin.Date{date(t, "2026-10-05"), date(t, "2026-10-06")}); err != nil {
			t.Fatal(err)
		}
		e.openDay(t, u, "2026-10-07")
		in7 := e.input(u, 2)
		out7 := e.post(t, u, "2026-10-07", 0, in7)
		e.openDay(t, u, "2026-10-08")
		in8 := e.input(u, 1)
		out8 := e.post(t, u, "2026-10-08", 0, in8)
		wantHeld(t, e.Env, u, 1)
		wheelBefore := e.wheel(t, u)

		if err := e.Service.ResetTestData(ctx, u); err != nil {
			t.Fatal(err)
		}
		wantHeld(t, e.Env, u, 0)
		wantSpinRows(t, e.Env, u, 0)
		for _, day := range []string{"2026-10-07", "2026-10-08"} {
			if got := e.entryOn(t, u, day); got != nil {
				t.Errorf("entry on %s survived the reset: %+v", day, got)
			}
		}
		for _, id := range []string{out7.Entry.ID, out8.Entry.ID} {
			if _, missing := e.entryRow(t, u, id); !errors.Is(missing, spin.ErrEntryNotFound) {
				t.Errorf("entry %s after the reset: %v, want ErrEntryNotFound", id, missing)
			}
			if rows := e.photoRows(t, u, id); len(rows) != 0 {
				t.Errorf("photo rows of %s survived the reset: %+v", id, rows)
			}
		}
		if used := e.keysUsed(t, u, append(keysOf(in7.Photos), keysOf(in8.Photos)...)); len(used) != 0 {
			t.Errorf("keys still held after the reset: %v", used)
		}
		for _, day := range []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"} {
			if pd := e.postedRow(t, u, day); pd != nil {
				t.Errorf("posted day %s survived the reset: %+v (any source goes)", day, pd)
			}
		}
		got := e.today(t, u)
		if got.State != spin.StateOpen || got.Spin != nil || got.Entry != nil || got.Week.Posted != 0 {
			t.Fatalf("today after the reset: state %s spin %+v entry %+v posted %d", got.State, got.Spin, got.Entry, got.Week.Posted)
		}
		wheelAfter := e.wheel(t, u)
		for i := range wheelBefore.Categories {
			if wheelAfter.Categories[i] != wheelBefore.Categories[i] {
				t.Errorf("category %d changed", i)
			}
		}

		// The same keys and the same day can be used again, and the week pays
		// its bonus again (its goal week is gone).
		e.openDay(t, u, "2026-10-08")
		again := e.post(t, u, "2026-10-08", 0, in8)
		if again.Already {
			t.Error("the post after the reset says already")
		}
		res, err := e.Service.MarkTestDays(ctx, u, []spin.Date{date(t, "2026-10-05"), date(t, "2026-10-06"), date(t, "2026-10-07")})
		if err != nil {
			t.Fatal(err)
		}
		if !res.BonusHeld || res.Week.Posted != 4 {
			t.Fatalf("after the reset: week %+v bonus held %v, want 4 days and a bonus", res.Week, res.BonusHeld)
		}
	})

	// --- concurrency ---

	run("twenty simultaneous posts with one list make one entry", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		in := e.input(u, 3)
		d := date(t, "2026-10-07")
		results := make([]spin.EntryOutcome, entryRacers)
		errs := make([]error, entryRacers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < entryRacers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results[i], errs[i] = e.Service.PostEntry(ctx, u, d, 0, in)
			}(i)
		}
		close(start)
		wg.Wait()

		fresh := 0
		var id string
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("racer %d: %v", i, errs[i])
			}
			if !results[i].Already {
				fresh++
			}
			if id == "" {
				id = results[i].Entry.ID
			} else if results[i].Entry.ID != id {
				t.Fatalf("racer %d got entry %s, others %s", i, results[i].Entry.ID, id)
			}
			if results[i].Today.State != spin.StatePosted {
				t.Fatalf("racer %d: today state %s, want posted", i, results[i].Today.State)
			}
		}
		if fresh != 1 {
			t.Fatalf("%d racers made the entry, want exactly 1", fresh)
		}
		wantPhotos(t, "photo rows", e.photoRows(t, u, id), keysOf(in.Photos))
		if got := e.entryOn(t, u, "2026-10-07"); got == nil || got.ID != id {
			t.Fatalf("entry of the date = %+v, want %s", got, id)
		}
		if tdy := e.today(t, u); tdy.Week.Posted != 1 {
			t.Fatalf("week posted = %d, want 1", tdy.Week.Posted)
		}
	})

	run("twenty simultaneous posts with different lists make one entry and nineteen refusals", func(t *testing.T, e *EntryEnv) {
		u := e.ready(t, ny)
		e.openDay(t, u, "2026-10-07")
		d := date(t, "2026-10-07")
		inputs := make([]spin.EntryInput, entryRacers)
		for i := range inputs {
			inputs[i] = e.input(u, 1+i%3)
		}
		errs := make([]error, entryRacers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < entryRacers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = e.Service.PostEntry(ctx, u, d, 0, inputs[i])
			}(i)
		}
		close(start)
		wg.Wait()

		winner := -1
		for i, err := range errs {
			switch {
			case err == nil && winner < 0:
				winner = i
			case err == nil:
				t.Fatalf("racers %d and %d both made the entry", winner, i)
			case !errors.Is(err, spin.ErrDaySettled):
				t.Fatalf("racer %d: %v, want ErrDaySettled", i, err)
			}
		}
		if winner < 0 {
			t.Fatal("no racer made the entry")
		}
		got := e.entryOn(t, u, "2026-10-07")
		if got == nil {
			t.Fatal("no entry after the race")
		}
		wantPhotos(t, "the winner's photo rows", got.Photos, keysOf(inputs[winner].Photos))
		for i, in := range inputs {
			if i == winner {
				continue
			}
			if used := e.keysUsed(t, u, keysOf(in.Photos)); len(used) != 0 {
				t.Fatalf("the refused racer %d left photo rows: %v", i, used)
			}
		}
	})

	run("a post and a skip racing give one winner and the other is ErrDaySettled", func(t *testing.T, e *EntryEnv) {
		for round := 0; round < 5; round++ {
			u := e.ready(t, ny)
			e.openDay(t, u, "2026-10-07")
			d := date(t, "2026-10-07")
			in := e.input(u, 1)
			var postErr, skipErr error
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_, postErr = e.Service.PostEntry(ctx, u, d, 0, in)
			}()
			go func() {
				defer wg.Done()
				<-start
				_, skipErr = e.Service.SkipDay(ctx, u, d, 0)
			}()
			close(start)
			wg.Wait()

			row := e.spinRow(t, u, "2026-10-07")
			entry := e.entryOn(t, u, "2026-10-07")
			switch {
			case postErr == nil && skipErr == nil:
				t.Fatalf("round %d: the post and the skip both won", round)
			case postErr == nil:
				if !errors.Is(skipErr, spin.ErrDaySettled) {
					t.Fatalf("round %d: skip = %v, want ErrDaySettled", round, skipErr)
				}
				if entry == nil || row.Result != spin.ResultPosted {
					t.Fatalf("round %d: post won but entry %+v result %s", round, entry, row.Result)
				}
			case skipErr == nil:
				if !errors.Is(postErr, spin.ErrDaySettled) {
					t.Fatalf("round %d: post = %v, want ErrDaySettled", round, postErr)
				}
				if entry != nil || row.Result != spin.ResultSkipped {
					t.Fatalf("round %d: skip won but entry %+v result %s", round, entry, row.Result)
				}
				if pd := e.postedRow(t, u, "2026-10-07"); pd != nil {
					t.Fatalf("round %d: skip won but a posted day exists: %+v", round, pd)
				}
			default:
				t.Fatalf("round %d: post = %v, skip = %v, want one winner", round, postErr, skipErr)
			}
		}
	})
}
