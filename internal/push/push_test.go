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
}

func (f *fakeRepo) CommentTargets(context.Context, uuid.UUID) ([]Target, error) {
	return f.targets, nil
}
func (f *fakeRepo) RequestPair(context.Context, uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	return f.pair[0], f.pair[1], true, nil
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
	// With the no-op sender nothing is looked up at all.
	repo := &fakeRepo{}
	people := &fakePeople{}
	n := NewNotifier(repo, people, NoopSender{}, quiet())
	n.CommentAdded(uuid.New())
	n.FollowAccepted(uuid.New())
	n.Wait()
}
