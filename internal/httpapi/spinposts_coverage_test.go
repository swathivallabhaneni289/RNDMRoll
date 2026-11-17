package httpapi

// Spin, wheel and post routes over the in-memory fakes: the per-person limits
// of the spin, wheel, archive and test routes, the answer when a person's
// spin lock is busy, and post ids that are not UUIDs. Every name in this file
// starts with spinposts or TestSpinposts.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin/spintest"
)

// G06, the spin half. Each route answers 429 rate_limited after its limit a
// minute, per person: time zone 20, Today 60, bonus 10, wheel read 60, wheel
// save 20, archive and unarchive 20, the archive list 60. The limit counts
// every request, answered or refused, and the limit clock stands still.
func TestSpinpostsSpinRoutes_AreLimitedPerPersonAMinute(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	id := uuid.NewString()
	for _, c := range []struct {
		name, method, path string
		body               any
		limit              int
	}{
		{"time zone", http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": "America/New_York"}, 20},
		{"today", http.MethodGet, "/v1/today", nil, 60},
		{"bonus", http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": "2026-10-07"}, 10},
		{"wheel read", http.MethodGet, "/v1/wheel", nil, 60},
		{"wheel save", http.MethodPatch, "/v1/wheel", map[string]any{"version": 1, "categories": []any{}}, 20},
		{"archive", http.MethodPost, "/v1/entries/" + id + "/archive", map[string]any{}, 20},
		{"unarchive", http.MethodPost, "/v1/entries/" + id + "/unarchive", map[string]any{}, 20},
		{"archive list", http.MethodGet, "/v1/me/archive", nil, 60},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := r.anyone()
			for i := 1; i <= c.limit; i++ {
				if rec := r.call(c.method, c.path, p.token, c.body); rec.Code == http.StatusTooManyRequests {
					t.Fatalf("request %d of %d was limited: %s", i, c.limit, rec.Body.String())
				}
			}
			wantCode(t, r.call(c.method, c.path, p.token, c.body), http.StatusTooManyRequests, "rate_limited")
			if rec := r.call(c.method, c.path, r.anyone().token, c.body); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("another person was limited: %s", rec.Body.String())
			}
		})
	}
}

// G06, the test routes. POST /test/clock, /test/posted-days and /test/reset
// share one allowance of 60 a minute per person.
func TestSpinpostsTestTools_ShareOneLimitOfSixtyAMinute(t *testing.T) {
	const one, two = "spinposts-tools-1@example.com", "spinposts-tools-2@example.com"
	r := newSpinRig(t, rigOptions{toolEmails: []string{one, two}})
	calls := []struct {
		path string
		body any
	}{
		{"/v1/test/clock", map[string]any{"reset": true}},
		{"/v1/test/posted-days", map[string]any{"dates": []string{"2026-10-05"}}},
		{"/v1/test/reset", map[string]any{}},
	}
	p := r.person(one, "spinposts_tools_1")
	// 20 of each: 60 together.
	for i := 0; i < 60; i++ {
		c := calls[i%len(calls)]
		if rec := r.call(http.MethodPost, c.path, p.token, c.body); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d (%s) was limited: %s", i+1, c.path, rec.Body.String())
		}
	}
	for _, c := range calls {
		wantCode(t, r.call(http.MethodPost, c.path, p.token, c.body), http.StatusTooManyRequests, "rate_limited")
	}
	q := r.person(two, "spinposts_tools_2")
	for _, c := range calls {
		if rec := r.call(http.MethodPost, c.path, q.token, c.body); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("another person was limited on %s: %s", c.path, rec.Body.String())
		}
	}
}

// spinpostsBusyRepo is a spin repository whose person lock can be made busy:
// while busy is set, every transaction fails the way a lock wait over 5
// seconds does in Postgres (spin.ErrBusy).
type spinpostsBusyRepo struct {
	spin.Repository
	busy atomic.Bool
}

func (r *spinpostsBusyRepo) InUserTx(ctx context.Context, userID string, seed spin.Seed, fn func(spin.Tx) error) error {
	if r.busy.Load() {
		return spin.ErrBusy
	}
	return r.Repository.InUserTx(ctx, userID, seed, fn)
}

// spinpostsServer is a Server over the in-memory fakes with the spin, wheel
// and entry routes on spinRepo, and the Board routes on board when it is not
// nil. It returns the server, one finished person's id and token, the Fixed
// spin clock and the photo store.
func spinpostsServer(t *testing.T, spinRepo spin.Repository, board *BoardHandler) (*Server, uuid.UUID, string, *clock.Fixed, *fakeEntryPhotoStore) {
	t.Helper()
	testLimitClock = frozenClock
	t.Cleanup(func() { testLimitClock = nil })
	users := newFakeUserRepo()
	tokens := newFakeRefreshRepo()
	users.tokens = tokens
	authHandler := NewAuthHandler(users, auth.NewRefreshService(tokens, 30*24*time.Hour), testServerJWTSecret, 15*time.Minute)
	authHandler.clock = frozenClock
	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	svc := spin.NewService(spinRepo, clk, spintest.NewQueueRand(20261007))
	photos := &fakeEntryPhotoStore{}
	deps := Deps{
		Auth:      authHandler,
		Profile:   NewProfileHandler(users, &fakeAvatarStore{}),
		Username:  NewUsernameHandler(users),
		Spin:      NewSpinHandler(svc),
		Wheel:     NewWheelHandler(svc),
		Entry:     NewEntryHandler(svc, photos),
		Board:     board,
		Users:     users,
		JWTSecret: testServerJWTSecret,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		PhotoLink: photos.Link,
	}
	u := users.seedComplete(t, "spinposts-person@example.com", "Spin Person", "spinposts_person", validBirthday)
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return NewServer(deps), u.ID, token, clk, photos
}

// G10. Section 10, Reliability: a person's spin work waits at most 5 seconds
// for its lock, then answers try again. Every route that takes the lock
// answers 503 with the fixed body {"error":"server_error"} (the phone retries
// it), never a logged 500.
func TestSpinpostsSpinRoutes_ABusyLockAnswersTryAgain(t *testing.T) {
	repo := &spinpostsBusyRepo{Repository: spintest.NewMemRepo()}
	srv, id, token, clk, photos := spinpostsServer(t, repo, nil)
	n := 0
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		n++
		return serveFrom(t, srv, remoteN(900+n), method, path, token, body)
	}
	want := func(method, path string, body any, status int) map[string]any {
		t.Helper()
		rec := call(method, path, body)
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body.String(), status)
		}
		return decodeBody(t, rec)
	}

	// Free: the zone, an evening spin and the wheel as it is.
	const ny, day = "America/New_York", "2026-10-07"
	want(http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": ny}, http.StatusOK)
	clk.Set(localInstant(t, ny, day, 20, 30, 0))
	want(http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusCreated)
	wheel := want(http.MethodGet, "/v1/wheel", nil, http.StatusOK)
	categories := []any{}
	for _, c := range wheel["categories"].([]any) {
		cat := c.(map[string]any)
		categories = append(categories, map[string]any{"id": cat["id"], "name": cat["name"], "weight": cat["weight"]})
	}

	repo.busy.Store(true)
	entry := uuid.NewString()
	for _, c := range []struct {
		name, method, path string
		body               any
	}{
		{"time zone", http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": "Europe/London"}},
		{"today", http.MethodGet, "/v1/today", nil},
		{"spin", http.MethodPost, "/v1/spin", map[string]any{"for_date": day}},
		{"bonus", http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": day}},
		{"skip", http.MethodPost, "/v1/spin/skip", map[string]any{"for_date": day, "spin_seq": 0}},
		{"post", http.MethodPost, "/v1/entries", entryRequest(day, 0, photos.newKey(id, "jpg"))},
		{"wheel read", http.MethodGet, "/v1/wheel", nil},
		{"wheel save", http.MethodPatch, "/v1/wheel", map[string]any{"version": wheel["version"], "categories": categories}},
		{"entry read", http.MethodGet, "/v1/entries/" + entry, nil},
		{"entry edit", http.MethodPatch, "/v1/entries/" + entry, map[string]any{"title": "Arrival"}},
		{"entry delete", http.MethodPost, "/v1/entries/" + entry + "/delete", map[string]any{}},
		{"archive", http.MethodPost, "/v1/entries/" + entry + "/archive", map[string]any{}},
		{"unarchive", http.MethodPost, "/v1/entries/" + entry + "/unarchive", map[string]any{}},
		{"archive list", http.MethodGet, "/v1/me/archive", nil},
	} {
		rec := call(c.method, c.path, c.body)
		if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != `{"error":"server_error"}` {
			t.Errorf("%s while the lock is busy: %d %s, want 503 {\"error\":\"server_error\"}", c.name, rec.Code, rec.Body.String())
		}
	}

	// Free again: the same person's spin is still there.
	repo.busy.Store(false)
	if got := want(http.MethodGet, "/v1/today", nil, http.StatusOK); dig(t, got, "spin", "seq") != float64(0) {
		t.Fatalf("Today after the lock is free: %v", got)
	}
}

// G11. Section 10, Security: a path id that is not a UUID never reaches the
// database. The post routes answer 404 not_found for it from the handler,
// before the board service is called: here there is no service at all, and a
// canonical id (the control) does reach it.
func TestSpinpostsPostRoutes_ABadIDNeverReachesTheService(t *testing.T) {
	srv, _, token, _, _ := spinpostsServer(t, spintest.NewMemRepo(), NewBoardHandler(nil))
	n := 0
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		n++
		return serveFrom(t, srv, remoteN(1200+n), method, path, token, body)
	}
	for _, bad := range []string{"not-a-uuid", "0123456789abcdef0123456789abcdef", "{" + uuid.NewString() + "}", "x"} {
		for _, r := range []struct {
			method, path string
			body         any
		}{
			{http.MethodGet, "/v1/posts/" + bad, nil},
			{http.MethodPost, "/v1/posts/" + bad + "/reaction", map[string]any{"reaction": "fire"}},
			{http.MethodPost, "/v1/posts/" + bad + "/comments", map[string]any{"text": "Hi"}},
			{http.MethodPost, "/v1/posts/" + bad + "/report", map[string]any{"reason": "spam"}},
			{http.MethodPost, "/v1/comments/" + bad + "/delete", map[string]any{}},
		} {
			rec := call(r.method, r.path, r.body)
			if rec.Code != http.StatusNotFound || rec.Body.String() != `{"error":"not_found"}` {
				t.Errorf("%s %s: %d %s, want 404 not_found", r.method, r.path, rec.Code, rec.Body.String())
			}
		}
	}
	// The control: a canonical id goes on to the service, which is missing here.
	if rec := call(http.MethodGet, "/v1/posts/"+uuid.NewString(), nil); rec.Code != http.StatusInternalServerError {
		t.Fatalf("a canonical id did not reach the service: %d %s", rec.Code, rec.Body.String())
	}
}
