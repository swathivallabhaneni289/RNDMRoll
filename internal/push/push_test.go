package push

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestValidToken(t *testing.T) {
	for _, ok := range []string{"ExponentPushToken[xxxxxxxxxxxxxxxxxxxxxx]", "ExpoPushToken[abc-DEF_123]"} {
		if !ValidToken(ok) {
			t.Errorf("ValidToken(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "ExponentPushToken[]", "ExponentPushToken[a b]", "abc", "ExponentPushToken[x]junk",
		"ExponentPushToken[" + strings.Repeat("a", 200) + "]"} {
		if ValidToken(bad) {
			t.Errorf("ValidToken(%q) = true", bad)
		}
	}
	if !ValidPlatform("ios") || !ValidPlatform("android") || ValidPlatform("web") {
		t.Error("ValidPlatform")
	}
}

func TestTexts(t *testing.T) {
	if got := FirstName(social.Person{Name: "Alice  Ames", Username: "al"}); got != "Alice" {
		t.Errorf("FirstName = %q", got)
	}
	if got := FirstName(social.Person{Name: " ", Username: "al"}); got != "al" {
		t.Errorf("FirstName fallback = %q", got)
	}
	cases := map[[2]string]string{
		{KindComment, "nice"}:                  `Alice commented: "nice"`,
		{KindReply, "thanks"}:                  `Alice replied to your comment: "thanks"`,
		{KindComment, ""}:                      "Alice sent a photo.",
		{KindReply, "  "}:                      "Alice sent a photo.",
		{KindComment, "a\nb"}:                  `Alice commented: "a b"`,
		{KindComment, strings.Repeat("x", 70)}: `Alice commented: "` + strings.Repeat("x", 60) + `..."`,
	}
	for in, want := range cases {
		if got := CommentBody(in[0], "Alice", in[1]); got != want {
			t.Errorf("CommentBody(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
	if got := LikeBody("Alice"); got != "Alice liked your post." {
		t.Errorf("LikeBody = %q", got)
	}
}

type fakeTokens struct {
	mu      sync.Mutex
	byUser  map[uuid.UUID][]string
	removed []string
}

func (f *fakeTokens) Tokens(_ context.Context, id uuid.UUID) ([]string, error) {
	return f.byUser[id], nil
}

func (f *fakeTokens) RemoveTokens(_ context.Context, tokens []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, tokens...)
	return nil
}

func TestExpoSenderBatchesAuthAndRemovesDeadTokens(t *testing.T) {
	var mu sync.Mutex
	var batches [][]expoMessage
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got []expoMessage
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		batches = append(batches, got)
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		var data []map[string]any
		for _, m := range got {
			if m.To == "ExponentPushToken[dead]" {
				data = append(data, map[string]any{"status": "error", "message": "gone", "details": map[string]any{"error": "DeviceNotRegistered"}})
			} else {
				data = append(data, map[string]any{"status": "ok", "id": "x"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()

	alice, bob := uuid.New(), uuid.New()
	many := make([]string, 0, 150)
	for i := 0; i < 149; i++ {
		many = append(many, "ExponentPushToken[a"+strings.Repeat("b", i%5)+"]")
	}
	many = append(many, "ExponentPushToken[dead]")
	tokens := &fakeTokens{byUser: map[uuid.UUID][]string{alice: many, bob: {}}}
	s := NewExpoSender(tokens, "secret", quiet())
	s.URL = srv.URL
	s.Send(context.Background(), []Message{
		{UserID: alice, Title: Title, Body: "hi", Data: map[string]string{"kind": KindComment}},
		{UserID: bob, Title: Title, Body: "no phone"},
	})

	if len(batches) != 2 || len(batches[0]) != 100 || len(batches[1]) != 50 {
		t.Fatalf("batches: %d", len(batches))
	}
	if batches[0][0].Title != "RNDMRoll" || batches[0][0].Body != "hi" || batches[0][0].Data["kind"] != KindComment {
		t.Errorf("message: %+v", batches[0][0])
	}
	if auth[0] != "Bearer secret" {
		t.Errorf("auth = %q", auth[0])
	}
	if len(tokens.removed) != 1 || tokens.removed[0] != "ExponentPushToken[dead]" {
		t.Errorf("removed = %v", tokens.removed)
	}
}

func TestExpoSenderSurvivesErrorsAndSlowServer(t *testing.T) {
	// Counted atomically: the slow second call is still running in the
	// server when the client gives up and the test reads the count.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Errorf("no access token set, got Authorization %q", r.Header.Get("Authorization"))
		}
		if n == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	alice := uuid.New()
	tokens := &fakeTokens{byUser: map[uuid.UUID][]string{alice: {"ExponentPushToken[a]"}}}
	s := NewExpoSender(tokens, "", quiet())
	s.URL = srv.URL
	s.Client.Timeout = 50 * time.Millisecond
	s.Send(context.Background(), []Message{{UserID: alice, Title: Title, Body: "x"}})
	s.Send(context.Background(), []Message{{UserID: alice, Title: Title, Body: "x"}})
	if calls.Load() != 2 || len(tokens.removed) != 0 {
		t.Errorf("calls %d removed %v", calls.Load(), tokens.removed)
	}
}

// recorder is a Sender that keeps what it was given.
type recorder struct {
	mu   sync.Mutex
	msgs []Message
}

func (r *recorder) Send(_ context.Context, msgs []Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msgs...)
}

type fakeRepo struct {
	Repository
	targets []Target
	pair    [2]uuid.UUID
	// owner and liked answer LikeTarget for any like (liked false: the like
	// is gone or the bell's rules leave it out); reads counts every read.
	owner uuid.UUID
	liked bool
	reads atomic.Int32
}

func (f *fakeRepo) CommentTargets(context.Context, uuid.UUID) ([]Target, error) {
	f.reads.Add(1)
	return f.targets, nil
}
func (f *fakeRepo) RequestPair(context.Context, uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	f.reads.Add(1)
	return f.pair[0], f.pair[1], true, nil
}
func (f *fakeRepo) LikeTarget(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, bool, error) {
	f.reads.Add(1)
	return f.owner, f.liked, nil
}

type fakePeople struct {
	people   map[uuid.UUID]social.Person
	settings map[uuid.UUID]social.Settings
}

func (f *fakePeople) People(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]social.Person, error) {
	out := map[uuid.UUID]social.Person{}
	for _, id := range ids {
		if p, ok := f.people[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func (f *fakePeople) Settings(_ context.Context, id uuid.UUID) (social.Settings, error) {
	if s, ok := f.settings[id]; ok {
		return s, nil
	}
	return social.DefaultSettings, nil
}

func TestNotifierRespectsSettingsAndSelf(t *testing.T) {
	alice, bob, cara := uuid.New(), uuid.New(), uuid.New()
	entry := uuid.New()
	people := &fakePeople{
		people:   map[uuid.UUID]social.Person{alice: {ID: alice, Name: "Alice Ames"}, bob: {ID: bob, Name: "Bob Brown"}},
		settings: map[uuid.UUID]social.Settings{cara: {Comments: false}},
	}
	repo := &fakeRepo{targets: []Target{
		{UserID: bob, Kind: KindComment, ActorID: alice, EntryID: entry, Body: "nice", Own: true},
		{UserID: cara, Kind: KindReply, ActorID: alice, EntryID: entry, Body: "nice"},
		{UserID: alice, Kind: KindReply, ActorID: alice, EntryID: entry, Body: "me"},
	}, pair: [2]uuid.UUID{alice, bob}}
	rec := &recorder{}
	n := NewNotifier(repo, people, rec, quiet())
	n.CommentAdded(uuid.New())
	n.FollowRequested(alice, bob)
	n.FollowRequested(bob, bob)
	n.FollowAccepted(uuid.New())
	n.Wait()

	got := map[string]Message{}
	for _, m := range rec.msgs {
		got[m.Data["kind"]+":"+m.UserID.String()] = m
	}
	if len(rec.msgs) != 3 {
		t.Fatalf("messages: %+v", rec.msgs)
	}
	if m := got["comment:"+bob.String()]; m.Body != `Alice commented: "nice"` || m.Data["entry_id"] != entry.String() || m.Data["own"] != "true" || m.Title != "RNDMRoll" {
		t.Errorf("comment: %+v", m)
	}
	if m := got["follow_request:"+bob.String()]; m.Body != "Alice wants to follow you." || m.Data["actor_id"] != alice.String() {
		t.Errorf("request: %+v", m)
	}
	if m := got["follow_accepted:"+alice.String()]; m.Body != "Bob accepted your follow request." || m.Data["actor_id"] != bob.String() {
		t.Errorf("accepted: %+v", m)
	}
}

func TestNotifierWithNoopSenderDoesNothing(t *testing.T) {
	// With the no-op sender nothing is looked up at all, and a like leaves
	// no mark in the memo.
	repo := &fakeRepo{owner: uuid.New(), liked: true}
	people := &fakePeople{}
	n := NewNotifier(repo, people, NoopSender{}, quiet())
	n.CommentAdded(uuid.New())
	n.FollowAccepted(uuid.New())
	n.Liked(uuid.New(), uuid.New())
	n.Wait()
	if got := repo.reads.Load(); got != 0 {
		t.Fatalf("%d reads with the no-op sender, want none", got)
	}
	if len(n.liked) != 0 {
		t.Fatalf("the memo holds %d likes with the no-op sender, want none", len(n.liked))
	}
}

// likeWorld is a notifier over the fakes with a clock the test moves: Alice
// owns the post, Bob and Cara like it.
type likeWorld struct {
	n                 *Notifier
	repo              *fakeRepo
	people            *fakePeople
	rec               *recorder
	alice, bob, cara  uuid.UUID
	entry, otherEntry uuid.UUID
	at                time.Time
}

func newLikeWorld() *likeWorld {
	w := &likeWorld{alice: uuid.New(), bob: uuid.New(), cara: uuid.New(), entry: uuid.New(), otherEntry: uuid.New(),
		at: time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC), rec: &recorder{}}
	w.repo = &fakeRepo{owner: w.alice, liked: true}
	w.people = &fakePeople{
		people: map[uuid.UUID]social.Person{
			w.alice: {ID: w.alice, Name: "Alice Ames", Username: "alice_a"},
			w.bob:   {ID: w.bob, Name: "Bob Brown", Username: "bob_b"},
			w.cara:  {ID: w.cara, Name: "", Username: "cara_c"},
		},
		settings: map[uuid.UUID]social.Settings{},
	}
	w.n = NewNotifier(w.repo, w.people, w.rec, quiet())
	// Read in the background only between the test's own Waits.
	w.n.now = func() time.Time { return w.at }
	return w
}

// like has liker like entry and waits for the push, and answers the
// messages sent since the last call.
func (w *likeWorld) like(entry, liker uuid.UUID) []Message {
	w.n.Liked(entry, liker)
	w.n.Wait()
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	got := w.rec.msgs
	w.rec.msgs = nil
	return got
}

// A new like sends one push to the post's owner: the bell's words, by the
// first name (the username without one), and the data a tap needs to open
// the owner's own entry page.
func TestNotifierLikedSendsOnePushToTheOwner(t *testing.T) {
	w := newLikeWorld()
	got := w.like(w.entry, w.bob)
	if len(got) != 1 {
		t.Fatalf("messages: %+v, want one", got)
	}
	m := got[0]
	want := map[string]string{"kind": "like", "entry_id": w.entry.String(), "own": "true", "actor_id": w.bob.String()}
	if m.UserID != w.alice || m.Title != "RNDMRoll" || m.Body != "Bob liked your post." || len(m.Data) != len(want) {
		t.Fatalf("the like push: %+v", m)
	}
	for k, v := range want {
		if m.Data[k] != v {
			t.Fatalf("data %q = %q, want %q (%+v)", k, m.Data[k], v, m.Data)
		}
	}
	if KindLike != "like" {
		t.Fatalf("KindLike = %q", KindLike)
	}
	if got := w.like(w.entry, w.cara); len(got) != 1 || got[0].Body != "cara_c liked your post." {
		t.Fatalf("a liker with no name: %+v", got)
	}
}

// The Likes switch (the stored Reactions value) turns like pushes off, and
// the Comments switch does not. A like that sent nothing leaves no mark, so
// with the switch back on the next like is pushed.
func TestNotifierLikedFollowsTheLikesSwitch(t *testing.T) {
	w := newLikeWorld()
	w.people.settings[w.alice] = social.Settings{Reactions: false, Comments: true, Milestones: true}
	if got := w.like(w.entry, w.bob); len(got) != 0 {
		t.Fatalf("with the Likes switch off: %+v", got)
	}
	w.people.settings[w.alice] = social.Settings{Reactions: true, Comments: false, Milestones: false}
	if got := w.like(w.entry, w.bob); len(got) != 1 || got[0].UserID != w.alice {
		t.Fatalf("with only the Likes switch on: %+v, want one push", got)
	}
}

// One person's like on one post is pushed once a day: undone and made again
// within likeMemoFor of the push, it sends nothing; from then on it sends
// again. Each person and post is its own: someone else's like on the same
// post, and the same person's like on another post, are pushed.
func TestNotifierLikedOncePerPersonAndPostInADay(t *testing.T) {
	w := newLikeWorld()
	start := w.at
	if got := w.like(w.entry, w.bob); len(got) != 1 {
		t.Fatalf("the first like: %+v", got)
	}
	w.at = start.Add(likeMemoFor - time.Minute)
	if got := w.like(w.entry, w.bob); len(got) != 0 {
		t.Fatalf("liked again within a day: %+v, want nothing", got)
	}
	if got := w.like(w.entry, w.cara); len(got) != 1 {
		t.Fatalf("someone else's like on the same post: %+v", got)
	}
	if got := w.like(w.otherEntry, w.bob); len(got) != 1 {
		t.Fatalf("the same person's like on another post: %+v", got)
	}
	w.at = start.Add(likeMemoFor)
	if got := w.like(w.entry, w.bob); len(got) != 1 {
		t.Fatalf("liked again a day after the push: %+v, want one", got)
	}
	w.at = start.Add(likeMemoFor + time.Minute)
	if got := w.like(w.entry, w.bob); len(got) != 0 {
		t.Fatalf("liked again a minute after that push: %+v, want nothing", got)
	}
	if likeMemoFor != 24*time.Hour {
		t.Fatalf("likeMemoFor is %v, want 24h", likeMemoFor)
	}

	// Two pushes of one like at once (a like, an undo and a like in a row)
	// send once: the memo's check and write are one step.
	w.n.Liked(w.otherEntry, w.cara)
	w.n.Liked(w.otherEntry, w.cara)
	w.n.Wait()
	if got := w.like(w.otherEntry, w.cara); len(got) != 1 {
		t.Fatalf("one like pushed twice at once: %+v, want one message", got)
	}
}

// The like is read when the push runs: one undone by then (or one the bell's
// rules leave out) sends nothing and leaves no mark, so a later like is
// pushed. So is one whose maker is not found any more, and the owner's own.
func TestNotifierLikedSendsNothingForALikeThatIsGone(t *testing.T) {
	w := newLikeWorld()
	w.repo.liked = false
	if got := w.like(w.entry, w.bob); len(got) != 0 {
		t.Fatalf("an undone like: %+v", got)
	}
	if got := w.repo.reads.Load(); got != 1 {
		t.Fatalf("%d reads, want the like read once", got)
	}
	w.repo.liked = true
	if got := w.like(w.entry, w.bob); len(got) != 1 {
		t.Fatalf("the like made again: %+v, want one push", got)
	}
	gone := uuid.New()
	if got := w.like(w.entry, gone); len(got) != 0 {
		t.Fatalf("a liker not found: %+v", got)
	}
	if got := w.like(w.entry, w.alice); len(got) != 0 {
		t.Fatalf("the owner's own like: %+v", got)
	}
	if len(w.n.liked) != 1 {
		t.Fatalf("the memo holds %d likes, want only the one pushed", len(w.n.liked))
	}
}

// The memo keeps a day of pushes: entries past likeMemoFor are swept out, at
// most once a likePrunePause, and it never holds more than likeMemoMax.
// Full, it pushes no new like until older ones age out.
func TestNotifierLikeMemoIsPrunedAndBounded(t *testing.T) {
	w := newLikeWorld()
	start := w.at
	for range 3 {
		if !w.n.claimLike(uuid.New(), w.bob) {
			t.Fatal("a first claim was refused")
		}
	}
	if len(w.n.liked) != 3 {
		t.Fatalf("the memo holds %d, want 3", len(w.n.liked))
	}
	w.at = start.Add(likeMemoFor + time.Second)
	if !w.n.claimLike(w.entry, w.bob) || len(w.n.liked) != 1 {
		t.Fatalf("after a day the memo holds %d, want only the new like", len(w.n.liked))
	}

	// Full: a new like is refused until a sweep after likePrunePause finds
	// old entries to drop.
	w.n.liked = make(map[likeKey]time.Time, likeMemoMax)
	for len(w.n.liked) < likeMemoMax {
		w.n.liked[likeKey{entry: uuid.New(), liker: w.cara}] = w.at
	}
	if w.n.claimLike(w.otherEntry, w.bob) || len(w.n.liked) != likeMemoMax {
		t.Fatalf("a claim on a full memo: it holds %d, want %d and the claim refused", len(w.n.liked), likeMemoMax)
	}
	w.at = w.at.Add(likeMemoFor + likePrunePause)
	if !w.n.claimLike(w.otherEntry, w.bob) || len(w.n.liked) != 1 {
		t.Fatalf("a day later the memo holds %d, want only the new like", len(w.n.liked))
	}
}
