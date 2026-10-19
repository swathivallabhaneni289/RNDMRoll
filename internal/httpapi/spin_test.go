package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// spinRig is a Server over the in-memory fakes with helpers to make people,
// move the clock and call routes. Every call comes from its own socket
// address, so only the per-person limits can ever answer 429.
type spinRig struct {
	t     *testing.T
	rig   *fullTestRig
	users *fakeUserRepo
	n     int
}

func newSpinRig(t *testing.T, opts rigOptions) *spinRig {
	t.Helper()
	users := newFakeUserRepo()
	return &spinRig{t: t, rig: newFullTestRigWith(t, users, opts), users: users, n: 700}
}

type spinPerson struct {
	id    uuid.UUID
	email string
	token string
}

// person makes a finished account and a token for it.
func (r *spinRig) person(email, username string) spinPerson {
	r.t.Helper()
	u := r.users.seedComplete(r.t, email, "Person "+username, username, validBirthday)
	token, err := auth.IssueAccessToken(u.ID, testServerJWTSecret, 15*time.Minute)
	if err != nil {
		r.t.Fatalf("issue token: %v", err)
	}
	return spinPerson{id: u.ID, email: email, token: token}
}

func (r *spinRig) call(method, path, token string, body any) *httptest.ResponseRecorder {
	r.t.Helper()
	r.n++
	return serveFrom(r.t, r.rig.srv, remoteN(r.n), method, path, token, body)
}

// raw sends a body as given (not marshalled), optionally with no declared
// length.
func (r *spinRig) raw(method, path, token, body string, chunked bool) *httptest.ResponseRecorder {
	r.t.Helper()
	r.n++
	var reader io.Reader = strings.NewReader(body)
	if chunked {
		reader = io.NopCloser(strings.NewReader(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if chunked {
		req.ContentLength = -1
	}
	req.RemoteAddr = remoteN(r.n)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.rig.srv.Engine().ServeHTTP(rec, req)
	return rec
}

// clockAt sets the Fixed clock to a wall clock reading in zone.
func (r *spinRig) clockAt(zone, date string, h, m, s int) {
	r.t.Helper()
	if r.rig.clock == nil {
		r.t.Fatal("this rig has no Fixed clock")
	}
	r.rig.clock.Set(localInstant(r.t, zone, date, h, m, s))
}

func localInstant(t *testing.T, zone, date string, h, m, s int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatalf("zone %s: %v", zone, err)
	}
	d, err := spin.ParseDate(date)
	if err != nil {
		t.Fatalf("date %s: %v", date, err)
	}
	got, ok := spin.InstantOfLocal(time.Date(d.Year, d.Month, d.Day, h, m, s, 0, time.UTC), loc)
	if !ok {
		t.Fatalf("%s %02d:%02d:%02d does not exist in %s", date, h, m, s, zone)
	}
	return got
}

func (r *spinRig) setZone(p spinPerson, zone string) {
	r.t.Helper()
	rec := r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": zone})
	if rec.Code != http.StatusOK {
		r.t.Fatalf("POST /v1/time-zone %s: %d %s", zone, rec.Code, rec.Body.String())
	}
}

func (r *spinRig) today(p spinPerson) map[string]any {
	r.t.Helper()
	rec := r.call(http.MethodGet, "/v1/today", p.token, nil)
	if rec.Code != http.StatusOK {
		r.t.Fatalf("GET /v1/today: %d %s", rec.Code, rec.Body.String())
	}
	return decodeBody(r.t, rec)
}

// dig reads a value from nested JSON objects.
func dig(t *testing.T, v any, path ...string) any {
	t.Helper()
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("dig %v: %q is not inside an object (%T)", path, key, v)
		}
		v, ok = m[key]
		if !ok {
			t.Fatalf("dig %v: no key %q", path, key)
		}
	}
	return v
}

func wantCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["error"] != code {
		t.Fatalf("error = %v, want %q: %s", body["error"], code, rec.Body.String())
	}
	return body
}

// --- every route: no token, deleted user, unfinished account, size cap, wrong type ---

var spinRoutes = []struct {
	name, method, path string
	body               any
	wrongType          any
}{
	{"time zone", http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": "Europe/London"}, map[string]any{"time_zone": 5}},
	{"today", http.MethodGet, "/v1/today", nil, nil},
	{"spin", http.MethodPost, "/v1/spin", map[string]any{}, map[string]any{"for_date": 5}},
	{"bonus", http.MethodPost, "/v1/spin/bonus", map[string]any{"after_seq": 0, "for_date": "2026-10-07"}, map[string]any{"after_seq": "zero", "for_date": "2026-10-07"}},
	{"wheel read", http.MethodGet, "/v1/wheel", nil, nil},
	{"wheel save", http.MethodPatch, "/v1/wheel", map[string]any{"version": 1, "categories": []any{}}, map[string]any{"version": "one", "categories": []any{}}},
	{"test clock", http.MethodPost, "/v1/test/clock", map[string]any{"reset": true}, map[string]any{"reset": "yes"}},
	{"test posted days", http.MethodPost, "/v1/test/posted-days", map[string]any{"dates": []string{"2026-10-05"}}, map[string]any{"dates": "2026-10-05"}},
	{"test reset", http.MethodPost, "/v1/test/reset", map[string]any{}, nil},
}

func TestSpinRoutes_RefuseNoTokenDeletedAndUnfinishedAccounts(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"unfinished@example.com", "gone@example.com"}})

	gone := r.users.seedComplete(t, "gone@example.com", "Gone", "gone_user", validBirthday)
	goneToken, _ := auth.IssueAccessToken(gone.ID, testServerJWTSecret, 15*time.Minute)
	r.users.remove(t, gone.ID)

	unfinished, err := r.users.Create(context.Background(), "unfinished@example.com", nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	unfinishedToken, _ := auth.IssueAccessToken(unfinished.ID, testServerJWTSecret, 15*time.Minute)

	for _, route := range spinRoutes {
		t.Run(route.name, func(t *testing.T) {
			wantCode(t, r.call(route.method, route.path, "", route.body), http.StatusUnauthorized, "token_invalid")
			wantCode(t, r.call(route.method, route.path, goneToken, route.body), http.StatusUnauthorized, "token_invalid")
			wantCode(t, r.call(route.method, route.path, unfinishedToken, route.body), http.StatusForbidden, "profile_incomplete")
		})
	}
}

func TestSpinRoutes_RefuseOversizeBodiesAndWrongTypes(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"sizes@example.com"}})
	p := r.person("sizes@example.com", "sizes_user")

	big := `{"time_zone":"` + strings.Repeat("a", 20<<10) + `"}`
	for _, route := range spinRoutes {
		if route.method == http.MethodGet {
			continue
		}
		t.Run(route.name+" over 16 KB", func(t *testing.T) {
			for _, chunked := range []bool{false, true} {
				rec := r.raw(route.method, route.path, p.token, big, chunked)
				wantCode(t, rec, http.StatusRequestEntityTooLarge, "payload_too_large")
			}
		})
		if route.wrongType != nil {
			t.Run(route.name+" wrong type", func(t *testing.T) {
				rec := r.call(route.method, route.path, p.token, route.wrongType)
				wantCode(t, rec, http.StatusBadRequest, "validation_failed")
			})
		}
		t.Run(route.name+" malformed json", func(t *testing.T) {
			rec := r.raw(route.method, route.path, p.token, `{"oops":`, false)
			wantCode(t, rec, http.StatusBadRequest, "validation_failed")
		})
	}
}

func TestSpinRoutes_AnEmptySpinBodyIsAccepted(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("empty@example.com", "empty_body")
	r.setZone(p, "America/New_York")
	r.clockAt("America/New_York", "2026-10-07", 20, 30, 0)

	rec := r.raw(http.MethodPost, "/v1/spin", p.token, "", false)
	if rec.Code != http.StatusCreated {
		t.Fatalf("an empty body: %d %s, want 201", rec.Code, rec.Body.String())
	}
	rec = r.raw(http.MethodPost, "/v1/spin", p.token, "{}", false)
	if rec.Code != http.StatusOK || decodeBody(t, rec)["already"] != true {
		t.Fatalf("{}: %d %s, want 200 already", rec.Code, rec.Body.String())
	}
	rec = r.raw(http.MethodPost, "/v1/spin", p.token, "null", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("null: %d %s, want 200", rec.Code, rec.Body.String())
	}
}

// --- the full day ---

func TestSpinFullDay(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"day@example.com"}})
	p := r.person("day@example.com", "day_person")
	const ny = "America/New_York"

	// The zone report answers with the same name.
	rec := r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": ny})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["time_zone"] != ny {
		t.Fatalf("time zone: %d %s", rec.Code, rec.Body.String())
	}

	// A spin two evenings ago, so the first spin rule (a person who has
	// never spun may spin at once) does not open the wheel at 4 PM.
	r.spinEvening(p, "2026-10-05")

	// 4 PM: waiting. The server writes the open time and the weekday.
	r.clockAt(ny, "2026-10-07", 16, 0, 0)
	today := r.today(p)
	if today["state"] != "waiting" || today["local_date"] != "2026-10-07" || today["time_zone"] != ny {
		t.Fatalf("4 PM: %v", today)
	}
	if today["next_open_at"] != "2026-10-08T00:00:00.000Z" || today["next_open_weekday"] != "Wednesday" {
		t.Fatalf("4 PM next open: %v %v", today["next_open_at"], today["next_open_weekday"])
	}
	if today["spin"] != nil {
		t.Fatalf("a spin before any spin: %v", today["spin"])
	}
	wheel := today["wheel"].([]any)
	if len(wheel) != 6 || dig(t, wheel[0], "name") != "Meal" || dig(t, wheel[1], "name") != "Movie" {
		t.Fatalf("starting wheel: %v", wheel)
	}
	win := today["window"].(map[string]any)
	if win["opens_at"] != "2026-10-08T00:00:00.000Z" || win["closes_at"] != "2026-10-08T04:00:00.000Z" {
		t.Fatalf("window: %v", win)
	}
	week := today["week"].(map[string]any)
	if week["start"] != "2026-10-05" || week["goal"] != float64(4) || week["posted"] != float64(0) || week["today_index"] != float64(2) {
		t.Fatalf("week: %v", week)
	}
	days := week["days"].([]any)
	if len(days) != 7 || dig(t, days[0], "letter") != "M" || dig(t, days[2], "weekday") != "Wednesday" {
		t.Fatalf("week days: %v", days)
	}

	// A spin before 8 PM is refused.
	wantCode(t, r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{}), http.StatusConflict, "spin_not_open")

	// Exactly 8 PM: open.
	r.clockAt(ny, "2026-10-07", 20, 0, 0)
	if got := r.today(p)["state"]; got != "open" {
		t.Fatalf("8:00:00 PM state = %v", got)
	}

	// 8:02 PM: the spin. 201, and the stamp is written by the server.
	r.clockAt(ny, "2026-10-07", 20, 2, 0)
	rec = r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{"for_date": "2026-10-07"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
	}
	first := decodeBody(t, rec)
	if first["already"] != false {
		t.Fatalf("already = %v", first["already"])
	}
	spun := dig(t, first, "today", "spin").(map[string]any)
	if dig(t, first, "today", "state") != "result" || spun["kind"] != "daily" || spun["seq"] != float64(0) || spun["result"] != "open" {
		t.Fatalf("after the spin: %v", first["today"])
	}
	if spun["spun_local_time"] != "8:02 PM" {
		t.Fatalf("spun_local_time = %v, want 8:02 PM", spun["spun_local_time"])
	}
	if dig(t, first, "today", "next_open_weekday") != "Thursday" {
		t.Fatalf("next_open_weekday = %v, want Thursday", dig(t, first, "today", "next_open_weekday"))
	}
	if spun["spun_at"] != "2026-10-08T00:02:00.000Z" {
		t.Fatalf("spun_at = %v", spun["spun_at"])
	}
	cat := spun["category"].(map[string]any)
	spunWheel := spun["wheel"].([]any)
	if idx := int(spun["winner_index"].(float64)); dig(t, spunWheel[idx], "id") != cat["id"] || dig(t, spunWheel[idx], "name") != cat["name"] {
		t.Fatalf("winner_index %d does not point at %v in %v", idx, cat, spunWheel)
	}

	// Again: 200, the same spin.
	r.clockAt(ny, "2026-10-07", 20, 5, 0)
	rec = r.call(http.MethodPost, "/v1/spin", p.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("second spin: %d %s", rec.Code, rec.Body.String())
	}
	second := decodeBody(t, rec)
	if second["already"] != true || dig(t, second, "today", "spin", "id") != spun["id"] {
		t.Fatalf("second spin: %v", second)
	}
	if got := r.today(p); got["state"] != "result" || dig(t, got, "spin", "id") != spun["id"] {
		t.Fatalf("today after the spin: %v", got["state"])
	}

	// Past midnight: late, with last night's spin and last night's time.
	r.clockAt(ny, "2026-10-08", 0, 30, 0)
	late := r.today(p)
	if late["state"] != "late" || late["local_date"] != "2026-10-08" {
		t.Fatalf("00:30: %v %v", late["state"], late["local_date"])
	}
	if dig(t, late, "spin", "id") != spun["id"] || dig(t, late, "spin", "local_date") != "2026-10-07" || dig(t, late, "spin", "spun_local_time") != "8:02 PM" {
		t.Fatalf("late spin: %v", late["spin"])
	}

	// The zone changes: the same spin keeps the time of its own zone, and
	// the next open weekday is the one in the person's current zone.
	r.setZone(p, "Asia/Tokyo")
	tokyo := r.today(p)
	if dig(t, tokyo, "spin", "spun_local_time") != "8:02 PM" {
		t.Fatalf("after the zone change spun_local_time = %v, want the New York time 8:02 PM", dig(t, tokyo, "spin", "spun_local_time"))
	}
	if tokyo["time_zone"] != "Asia/Tokyo" || tokyo["state"] != "late" {
		t.Fatalf("tokyo: zone %v state %v", tokyo["time_zone"], tokyo["state"])
	}
	r.setZone(p, ny)

	// Post yesterday: waiting.
	rec = r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-10-07"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("posted-days: %d %s", rec.Code, rec.Body.String())
	}
	if got := r.today(p); got["state"] != "waiting" || got["spin"] != nil {
		t.Fatalf("after posting yesterday: state %v spin %v", got["state"], got["spin"])
	}
	// 2026-10-07 is the Wednesday of this week: its mark is set.
	week = r.today(p)["week"].(map[string]any)
	if dig(t, week["days"].([]any)[2], "posted") != true || week["posted"] != float64(1) {
		t.Fatalf("week after posting: %v", week)
	}

	// 12:30 AM spun time, and a spin closed by an old date.
	r.clockAt(ny, "2026-10-08", 20, 0, 0)
	wantCode(t, r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{"for_date": "2026-10-06"}), http.StatusConflict, "spin_closed")
	wantCode(t, r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{"for_date": "2026-10-09"}), http.StatusConflict, "spin_not_open")
}

func TestSpinStampIsRightInAHalfHourZone(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("late-stamp@example.com", "late_stamp")
	r.setZone(p, "Asia/Kolkata")
	// 20:00 in Kolkata opens; a spin at 20:05 there is "8:05 PM".
	r.clockAt("Asia/Kolkata", "2026-10-07", 20, 5, 0)
	rec := r.call(http.MethodPost, "/v1/spin", p.token, nil)
	if rec.Code != http.StatusCreated || dig(t, decodeBody(t, rec), "today", "spin", "spun_local_time") != "8:05 PM" {
		t.Fatalf("Kolkata: %d %s", rec.Code, rec.Body.String())
	}
}

// --- the bonus spin ---

func TestSpinBonusFlow(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"bonus@example.com"}})
	p := r.person("bonus@example.com", "bonus_person")
	const ny = "America/New_York"
	r.setZone(p, ny)

	// Earn it with four pretend days.
	r.clockAt(ny, "2026-10-08", 12, 0, 0)
	rec := r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{
		"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("posted-days: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["bonus_held"] != true || dig(t, body, "week", "posted") != float64(4) || dig(t, body, "week", "goal_met") != true {
		t.Fatalf("posted-days answer: %v", body)
	}
	if got := r.today(p); dig(t, got, "bonus", "held") != true || dig(t, got, "bonus", "can_use") != false {
		t.Fatalf("held but no spin yet: %v", got["bonus"])
	}

	// Friday evening: spin, then spend.
	r.clockAt(ny, "2026-10-09", 20, 30, 0)
	rec = r.call(http.MethodPost, "/v1/spin", p.token, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
	}
	daily := dig(t, decodeBody(t, rec), "today", "spin").(map[string]any)
	if got := r.today(p); dig(t, got, "bonus", "can_use") != true {
		t.Fatalf("can_use after the spin: %v", got["bonus"])
	}

	// after_seq and for_date are both required.
	wantField(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"for_date": "2026-10-09"}), "after_seq")
	wantField(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0}), "for_date")
	wantField(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "friday"}), "for_date")
	wantField(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": -1, "for_date": "2026-10-09"}), "after_seq")

	rec = r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-09"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("bonus: %d %s", rec.Code, rec.Body.String())
	}
	spent := decodeBody(t, rec)
	bonus := dig(t, spent, "today", "spin").(map[string]any)
	if spent["already"] != false || bonus["kind"] != "bonus" || bonus["seq"] != float64(1) {
		t.Fatalf("bonus spin: %v", bonus)
	}
	if dig(t, bonus, "category", "id") == dig(t, daily, "category", "id") {
		t.Fatal("the bonus repeated the category")
	}
	if dig(t, spent, "today", "bonus", "held") != false || dig(t, spent, "today", "bonus", "can_use") != false {
		t.Fatalf("bonus after spending: %v", dig(t, spent, "today", "bonus"))
	}

	// The same call again: 200, the same spin, nothing spent.
	rec = r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-09"})
	if rec.Code != http.StatusOK {
		t.Fatalf("retried bonus: %d %s", rec.Code, rec.Body.String())
	}
	again := decodeBody(t, rec)
	if again["already"] != true || dig(t, again, "today", "spin", "id") != bonus["id"] {
		t.Fatalf("retried bonus: %v", again)
	}

	// Posting the day settles it.
	rec = r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-10-09"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("posted-days: %d %s", rec.Code, rec.Body.String())
	}
	wantCode(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 1, "for_date": "2026-10-09"}), http.StatusConflict, "day_settled")
	// The retry of the spent one still answers.
	rec = r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-09"})
	if rec.Code != http.StatusOK || dig(t, decodeBody(t, rec), "today", "spin", "id") != bonus["id"] {
		t.Fatalf("retry after posting: %d %s", rec.Code, rec.Body.String())
	}
}

func wantField(t *testing.T, rec *httptest.ResponseRecorder, field string) {
	t.Helper()
	assertFieldError(t, rec, field)
}

func TestSpinBonusErrorCodes(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"codes@example.com"}})
	p := r.person("codes@example.com", "codes_person")
	const ny = "America/New_York"
	r.setZone(p, ny)

	// Before 8 PM with no spin: not open.
	r.clockAt(ny, "2026-10-07", 12, 0, 0)
	wantCode(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-07"}), http.StatusConflict, "spin_not_open")

	// After a spin, with none held.
	r.clockAt(ny, "2026-10-07", 20, 30, 0)
	r.call(http.MethodPost, "/v1/spin", p.token, nil)
	wantCode(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-07"}), http.StatusConflict, "no_bonus")
	// An after_seq above the latest spin is a bad request, naming the field.
	wantField(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 4, "for_date": "2026-10-07"}), "after_seq")
	// A date that has closed.
	r.clockAt(ny, "2026-10-08", 9, 0, 0)
	wantCode(t, r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-07"}), http.StatusConflict, "spin_closed")
}

func TestSpinZoneErrors(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("zones@example.com", "zone_person")

	// No zone reported yet.
	assertFieldError(t, r.call(http.MethodGet, "/v1/today", p.token, nil), "time_zone")
	assertFieldError(t, r.call(http.MethodPost, "/v1/spin", p.token, nil), "time_zone")
	assertFieldError(t, r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{}), "time_zone")
	assertFieldError(t, r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": ""}), "time_zone")
	for _, bad := range []string{"america/new_york", "Local", "../x", "Nowhere/Land", strings.Repeat("A", 65)} {
		rec := r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": bad})
		assertFieldError(t, rec, "time_zone")
		if strings.Contains(rec.Body.String(), bad) && len(bad) > 6 {
			t.Errorf("the refusal echoes the input %q: %s", bad, rec.Body.String())
		}
	}
	// A refusal stores nothing: Today still asks for a zone.
	assertFieldError(t, r.call(http.MethodGet, "/v1/today", p.token, nil), "time_zone")

	// Malformed dates name the field.
	r.setZone(p, "America/New_York")
	for _, bad := range []string{"2026-1-5", "tomorrow", "2026-02-30", "20261007", ""} {
		assertFieldError(t, r.call(http.MethodPost, "/v1/spin", p.token, map[string]any{"for_date": bad}), "for_date")
	}
}

// --- limits ---

func TestSpinTheEleventhSpinInAMinuteIsRateLimited(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("limits@example.com", "limit_person")
	r.setZone(p, "America/New_York")
	r.clockAt("America/New_York", "2026-10-07", 20, 30, 0)

	for i := 1; i <= 10; i++ {
		rec := r.call(http.MethodPost, "/v1/spin", p.token, nil)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("spin %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	wantCode(t, r.call(http.MethodPost, "/v1/spin", p.token, nil), http.StatusTooManyRequests, "rate_limited")

	// The limit is per person: another person is not affected.
	q := r.person("limits-two@example.com", "limit_two")
	r.setZone(q, "America/New_York")
	if rec := r.call(http.MethodPost, "/v1/spin", q.token, nil); rec.Code != http.StatusCreated {
		t.Fatalf("another person's first spin: %d %s", rec.Code, rec.Body.String())
	}
	// And Today has its own bucket.
	if rec := r.call(http.MethodGet, "/v1/today", p.token, nil); rec.Code != http.StatusOK {
		t.Fatalf("today while spins are limited: %d", rec.Code)
	}
}

// --- nobody sees another person's data ---

func TestSpinPeopleNeverSeeEachOthersData(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	a := r.person("iso-a@example.com", "iso_a")
	b := r.person("iso-b@example.com", "iso_b")
	const ny = "America/New_York"
	r.setZone(a, ny)
	r.setZone(b, ny)
	r.clockAt(ny, "2026-10-07", 20, 30, 0)

	// A spins and edits the wheel.
	rec := r.call(http.MethodPost, "/v1/spin", a.token, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("a spin: %d %s", rec.Code, rec.Body.String())
	}
	aWheel := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", a.token, nil))
	aCats := aWheel["categories"].([]any)
	aMeal := aCats[0].(map[string]any)
	rec = r.call(http.MethodPatch, "/v1/wheel", a.token, map[string]any{
		"version": aWheel["version"],
		"categories": []any{
			map[string]any{"id": aMeal["id"], "name": "Only A", "weight": 9},
			aCats[1],
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("a save: %d %s", rec.Code, rec.Body.String())
	}

	// B has no spin, the starting wheel and version 1.
	bToday := r.today(b)
	if bToday["spin"] != nil || bToday["state"] != "open" {
		t.Fatalf("b's today: state %v spin %v", bToday["state"], bToday["spin"])
	}
	bWheel := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", b.token, nil))
	if bWheel["version"] != float64(1) || len(bWheel["categories"].([]any)) != 6 {
		t.Fatalf("b's wheel: %v", bWheel)
	}
	if strings.Contains(fmt.Sprint(bWheel), "Only A") {
		t.Fatal("b sees a's category name")
	}

	// B naming A's category id changes nothing and names the row.
	rec = r.call(http.MethodPatch, "/v1/wheel", b.token, map[string]any{
		"version": 1,
		"categories": []any{
			map[string]any{"id": aMeal["id"], "name": "Stolen", "weight": 10},
			map[string]any{"id": nil, "name": "Mine", "weight": 1},
		},
	})
	body := wantCode(t, rec, http.StatusBadRequest, "wheel_invalid")
	if body["reason"] != "unknown_id" || body["index"] != float64(0) {
		t.Fatalf("foreign id: %v", body)
	}
	aAfter := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", a.token, nil))
	if dig(t, aAfter["categories"].([]any)[0], "name") != "Only A" {
		t.Fatalf("a's wheel was changed by b: %v", aAfter)
	}
	bAfter := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", b.token, nil))
	if bAfter["version"] != float64(1) || len(bAfter["categories"].([]any)) != 6 {
		t.Fatalf("b's wheel changed: %v", bAfter)
	}

	// B's user id in a body is ignored: the spin is B's own.
	rec = r.call(http.MethodPost, "/v1/spin", b.token, map[string]any{"user_id": a.id.String(), "for_date": "2026-10-07"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("b's spin: %d %s", rec.Code, rec.Body.String())
	}
	if dig(t, decodeBody(t, rec), "today", "spin", "id") == dig(t, r.today(a), "spin", "id") {
		t.Fatal("b got a's spin")
	}
}

// --- nothing private in any answer ---

func TestSpinNoResponseCarriesEmailBirthdayOrPasswordHash(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"private@example.com"}})
	p := r.person("private@example.com", "private_person")
	const ny = "America/New_York"

	forbidden := map[string]bool{"birthday": true, "email": true, "password_hash": true}
	check := func(label string, rec *httptest.ResponseRecorder) {
		t.Helper()
		var v any
		if len(bytes.TrimSpace(rec.Body.Bytes())) == 0 {
			return
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("%s: not JSON: %s", label, rec.Body.String())
		}
		var walk func(v any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for k, child := range x {
					if forbidden[strings.ToLower(k)] || strings.Contains(strings.ToLower(k), "birth") {
						t.Errorf("%s: key %q in %s", label, k, rec.Body.String())
					}
					walk(child)
				}
			case []any:
				for _, child := range x {
					walk(child)
				}
			}
		}
		walk(v)
		if strings.Contains(rec.Body.String(), "private@example.com") {
			t.Errorf("%s: the account's email is in the body: %s", label, rec.Body.String())
		}
	}

	check("time zone", r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": ny}))
	check("bad zone", r.call(http.MethodPost, "/v1/time-zone", p.token, map[string]any{"time_zone": "Nope/Nope"}))
	r.clockAt(ny, "2026-10-08", 12, 0, 0)
	check("today", r.call(http.MethodGet, "/v1/today", p.token, nil))
	check("wheel", r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	check("posted days", r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"}}))
	check("clock", r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-10-09T20:30:00"}))
	check("spin", r.call(http.MethodPost, "/v1/spin", p.token, nil))
	check("spin again", r.call(http.MethodPost, "/v1/spin", p.token, nil))
	check("bonus", r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 0, "for_date": "2026-10-09"}))
	check("bonus refused", r.call(http.MethodPost, "/v1/spin/bonus", p.token, map[string]any{"after_seq": 1, "for_date": "2026-10-09"}))
	check("wheel save", r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": 1, "categories": []any{}}))
	check("reset", r.call(http.MethodPost, "/v1/test/reset", p.token, map[string]any{}))
}

// --- the wheel routes ---

func TestWheelRoutes_ReadAndSave(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("wheel@example.com", "wheel_person")

	// The wheel needs no zone and is made on first use.
	rec := r.call(http.MethodGet, "/v1/wheel", p.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/wheel: %d %s", rec.Code, rec.Body.String())
	}
	got := decodeBody(t, rec)
	cats := got["categories"].([]any)
	if got["version"] != float64(1) || len(cats) != 6 {
		t.Fatalf("wheel = %v", got)
	}
	wantNames := []string{"Meal", "Movie", "Song", "Book", "Place", "Game"}
	wantWeights := []float64{1, 1, 1, 1, 1, 1}
	for i, c := range cats {
		m := c.(map[string]any)
		if m["name"] != wantNames[i] || m["weight"] != wantWeights[i] || m["position"] != float64(i) {
			t.Errorf("category %d = %v", i, m)
		}
		if _, err := uuid.Parse(m["id"].(string)); err != nil {
			t.Errorf("category %d id %v", i, m["id"])
		}
	}
	limits := got["limits"].(map[string]any)
	for k, want := range map[string]float64{"min": 2, "max": 12, "name_max": 24, "weight_min": 1, "weight_max": 10} {
		if limits[k] != want {
			t.Errorf("limits[%s] = %v, want %v", k, limits[k], want)
		}
	}

	// Save: rename, reweight, add, archive. The answer is the GET body with a
	// new version.
	list := []any{
		map[string]any{"id": cats[0].(map[string]any)["id"], "name": "Dinner", "weight": 5},
		map[string]any{"id": cats[2].(map[string]any)["id"], "name": "Song", "weight": 1},
		map[string]any{"id": nil, "name": "Walk", "weight": 2},
	}
	rec = r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": 1, "categories": list})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	saved := decodeBody(t, rec)
	if saved["version"] != float64(2) || len(saved["categories"].([]any)) != 3 || saved["limits"] == nil {
		t.Fatalf("saved = %v", saved)
	}
	again := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	if fmt.Sprint(again["categories"]) != fmt.Sprint(saved["categories"]) || again["version"] != float64(2) {
		t.Fatalf("GET after PATCH differs: %v vs %v", again, saved)
	}

	// A stale version changes nothing.
	rec = r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": 1, "categories": list})
	wantCode(t, rec, http.StatusConflict, "wheel_stale")
	if after := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil)); after["version"] != float64(2) {
		t.Fatalf("a stale save changed the version: %v", after["version"])
	}

	// Required fields are named.
	assertFieldError(t, r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"categories": list}), "version")
	assertFieldError(t, r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": 2}), "categories")
}

func TestWheelRoutes_InvalidWheelsNameTheReasonAndTheRow(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("wheel-bad@example.com", "wheel_bad")
	wheel := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	cats := wheel["categories"].([]any)
	id0 := cats[0].(map[string]any)["id"]

	cases := []struct {
		name       string
		categories []any
		reason     string
		index      float64
	}{
		{"too few", []any{map[string]any{"id": nil, "name": "A", "weight": 1}}, "too_few", -1},
		{"too many", func() []any {
			var l []any
			for i := 0; i < 13; i++ {
				l = append(l, map[string]any{"id": nil, "name": fmt.Sprintf("C%d", i), "weight": 1})
			}
			return l
		}(), "too_many", -1},
		{"blank name", []any{map[string]any{"id": nil, "name": "A", "weight": 1}, map[string]any{"id": nil, "name": "   ", "weight": 1}}, "name_required", 1},
		{"long name", []any{map[string]any{"id": nil, "name": "A", "weight": 1}, map[string]any{"id": nil, "name": strings.Repeat("n", 25), "weight": 1}}, "name_too_long", 1},
		{"same name", []any{map[string]any{"id": nil, "name": "Meal", "weight": 1}, map[string]any{"id": nil, "name": "MEAL", "weight": 1}}, "duplicate_name", 1},
		{"weight 11", []any{map[string]any{"id": nil, "name": "A", "weight": 1}, map[string]any{"id": nil, "name": "B", "weight": 11}}, "weight_range", 1},
		{"same id", []any{map[string]any{"id": id0, "name": "A", "weight": 1}, map[string]any{"id": id0, "name": "B", "weight": 1}}, "duplicate_id", 1},
		{"unknown id", []any{map[string]any{"id": uuid.NewString(), "name": "A", "weight": 1}, map[string]any{"id": nil, "name": "B", "weight": 1}}, "unknown_id", 0},
		{"not a uuid", []any{map[string]any{"id": "abc", "name": "A", "weight": 1}, map[string]any{"id": nil, "name": "B", "weight": 1}}, "unknown_id", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{"version": 1, "categories": c.categories})
			body := wantCode(t, rec, http.StatusBadRequest, "wheel_invalid")
			if body["reason"] != c.reason || body["index"] != c.index {
				t.Fatalf("body = %v, want reason %s index %v", body, c.reason, c.index)
			}
			if len(body) != 3 {
				t.Errorf("body has keys beyond error, reason and index: %v", body)
			}
		})
	}
	// Nothing changed.
	after := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	if after["version"] != float64(1) || fmt.Sprint(after["categories"]) != fmt.Sprint(wheel["categories"]) {
		t.Fatalf("an invalid save changed the wheel: %v", after)
	}
}

func TestWheelRoutes_TodayKeepsTheSpinsOwnWheelAfterAnEdit(t *testing.T) {
	r := newSpinRig(t, rigOptions{})
	p := r.person("wheel-snap@example.com", "wheel_snap")
	const ny = "America/New_York"
	r.setZone(p, ny)
	r.clockAt(ny, "2026-10-07", 20, 30, 0)
	r.rig.rand.Queue(2) // Song: third of six equal wedges in the starting wheel
	rec := r.call(http.MethodPost, "/v1/spin", p.token, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
	}

	wheel := decodeBody(t, r.call(http.MethodGet, "/v1/wheel", p.token, nil))
	cats := wheel["categories"].([]any)
	rec = r.call(http.MethodPatch, "/v1/wheel", p.token, map[string]any{
		"version": 1,
		"categories": []any{
			map[string]any{"id": cats[0].(map[string]any)["id"], "name": "Renamed Meal", "weight": 1},
			map[string]any{"id": cats[2].(map[string]any)["id"], "name": "Renamed Song", "weight": 1},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	today := r.today(p)
	if dig(t, today, "spin", "category", "name") != "Song" {
		t.Fatalf("the spin's category = %v, want the name it had", dig(t, today, "spin", "category", "name"))
	}
	if w := today["wheel"].([]any); len(w) != 6 || dig(t, w[0], "name") != "Meal" || dig(t, w[2], "name") != "Song" {
		t.Fatalf("today's wheel changed after an edit: %v", w)
	}
}

// --- the test-only routes ---

func TestSpinTestTools_AreNotRegisteredWhenOff(t *testing.T) {
	r := newSpinRig(t, rigOptions{}) // no toolEmails: not registered
	p := r.person("off@example.com", "tools_off")
	for _, path := range []string{"/v1/test/clock", "/v1/test/posted-days", "/v1/test/reset"} {
		rec := r.call(http.MethodPost, path, p.token, map[string]any{"reset": true})
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s with the tools off: %d, want 404", path, rec.Code)
		}
	}
}

func TestSpinTestTools_AreRefusedForAnyoneNotListed(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"listed@example.com"}})
	listed := r.person("Listed@Example.com", "listed_one") // the list is compared lower-cased
	other := r.person("not-listed@example.com", "not_listed")

	for _, path := range []string{"/v1/test/clock", "/v1/test/posted-days", "/v1/test/reset"} {
		rec := r.call(http.MethodPost, path, other.token, map[string]any{"reset": true, "dates": []string{"2026-10-05"}})
		wantCode(t, rec, http.StatusNotFound, "not_found")
	}
	// The listed account may.
	if rec := r.call(http.MethodPost, "/v1/test/clock", listed.token, map[string]any{"reset": true}); rec.Code != http.StatusOK {
		t.Fatalf("listed account: %d %s", rec.Code, rec.Body.String())
	}
	// And the refused account's clock was not moved or touched.
	if rec := r.call(http.MethodGet, "/v1/today", other.token, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("other's today without a zone: %d", rec.Code)
	}
}

func TestSpinTestTools_ClockMovesOnlyTheCaller(t *testing.T) {
	perUser := clock.NewPerUser()
	r := newSpinRig(t, rigOptions{perUser: perUser, toolEmails: []string{"mover@example.com"}})
	a := r.person("mover@example.com", "mover_a")
	b := r.person("bystander@example.com", "bystander_b")
	r.setZone(a, "America/New_York")
	r.setZone(b, "America/New_York")

	// A moves to a day in 2031; its Today reads that day, and the answer says
	// the clock is shifted.
	rec := r.call(http.MethodPost, "/v1/test/clock", a.token, map[string]any{"now": "2031-03-04T15:00:00Z"})
	if rec.Code != http.StatusOK {
		t.Fatalf("clock now: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["shifted"] != true || !strings.HasPrefix(body["now"].(string), "2031-03-04T15:00:") {
		t.Fatalf("clock answer: %v", body)
	}
	aToday := r.today(a)
	if !strings.HasPrefix(aToday["now"].(string), "2031-03-04T15:00:") || aToday["local_date"] != "2031-03-04" {
		t.Fatalf("a's today: now %v date %v", aToday["now"], aToday["local_date"])
	}

	// B is untouched: real time.
	bToday := r.today(b)
	bNow, err := time.Parse(time.RFC3339, bToday["now"].(string))
	if err != nil {
		t.Fatalf("b's now: %v", err)
	}
	if d := time.Since(bNow); d < -5*time.Second || d > 5*time.Second {
		t.Fatalf("b's now is %v from real time, want within 5 seconds", d)
	}

	// Token expiry is unmoved: A's 15 minute token still works with its clock
	// ten years ahead (and a year behind).
	for _, target := range []string{"2041-03-04T15:00:00Z", "2001-03-04T15:00:00Z"} {
		if rec := r.call(http.MethodPost, "/v1/test/clock", a.token, map[string]any{"now": target}); rec.Code != http.StatusOK {
			t.Fatalf("clock %s: %d %s", target, rec.Code, rec.Body.String())
		}
		if rec := r.call(http.MethodGet, "/v1/today", a.token, nil); rec.Code != http.StatusOK {
			t.Fatalf("a's token stopped working with the clock at %s: %d %s", target, rec.Code, rec.Body.String())
		}
	}

	// Reset returns A to real time.
	rec = r.call(http.MethodPost, "/v1/test/clock", a.token, map[string]any{"reset": true})
	body = decodeBody(t, rec)
	if rec.Code != http.StatusOK || body["shifted"] != false {
		t.Fatalf("reset: %d %v", rec.Code, body)
	}
	aNow, _ := time.Parse(time.RFC3339, r.today(a)["now"].(string))
	if d := time.Since(aNow); d < -5*time.Second || d > 5*time.Second {
		t.Fatalf("a's now after reset is %v from real time", d)
	}
}

func TestSpinTestTools_LocalTimesAndTheirRefusals(t *testing.T) {
	perUser := clock.NewPerUser()
	r := newSpinRig(t, rigOptions{perUser: perUser, toolEmails: []string{"local@example.com"}})
	p := r.person("local@example.com", "local_person")

	// No zone yet: a local time cannot be read.
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-10-07T19:59:30"}), "time_zone")

	r.setZone(p, "America/New_York")
	// A spin two evenings ago, so the first spin rule does not open the
	// wheel before 8 PM.
	if rec := r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-10-05T20:30:00"}); rec.Code != http.StatusOK {
		t.Fatalf("local: %d %s", rec.Code, rec.Body.String())
	}
	if rec := r.call(http.MethodPost, "/v1/spin", p.token, nil); rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d %s", rec.Code, rec.Body.String())
	}
	rec := r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-10-07T19:59:30"})
	if rec.Code != http.StatusOK {
		t.Fatalf("local: %d %s", rec.Code, rec.Body.String())
	}
	if now := decodeBody(t, rec)["now"].(string); !strings.HasPrefix(now, "2026-10-07T23:59:3") {
		t.Fatalf("19:59:30 New York is 23:59:30Z, got %s", now)
	}
	today := r.today(p)
	if today["state"] != "waiting" || today["local_date"] != "2026-10-07" {
		t.Fatalf("today at 19:59:30: %v %v", today["state"], today["local_date"])
	}

	// A time inside the spring-forward gap does not exist.
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-03-08T02:30:00"}), "local")
	// A repeated time takes the first (EDT, 05:30Z).
	rec = r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-11-01T01:30:00"})
	if now := decodeBody(t, rec)["now"].(string); rec.Code != http.StatusOK || !strings.HasPrefix(now, "2026-11-01T05:30:") {
		t.Fatalf("repeated time: %d %s", rec.Code, rec.Body.String())
	}

	// Exactly one of now, local and reset.
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{}), "now")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"reset": true, "now": "2026-10-07T00:00:00Z"}), "now")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"reset": false}), "reset")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"now": "yesterday"}), "now")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/clock", p.token, map[string]any{"local": "2026-10-07 19:59"}), "local")
}

func TestSpinTestTools_PostedDaysAndReset(t *testing.T) {
	r := newSpinRig(t, rigOptions{toolEmails: []string{"days@example.com"}})
	p := r.person("days@example.com", "days_person")
	const ny = "America/New_York"
	r.setZone(p, ny)
	r.clockAt(ny, "2026-10-07", 12, 0, 0)

	// Validation: none after today, none older than 60 days, 1 to 7, real dates.
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{}), "dates")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{}}), "dates")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-10-08"}}), "dates")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-08-07"}}), "dates")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"nope"}}), "dates")
	assertFieldError(t, r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{
		"dates": []string{"2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07", "2026-09-30"},
	}), "dates")
	if got := r.today(p); dig(t, got, "week", "posted") != float64(0) {
		t.Fatalf("a refused call marked days: %v", got["week"])
	}

	rec := r.call(http.MethodPost, "/v1/test/posted-days", p.token, map[string]any{"dates": []string{"2026-10-05", "2026-10-06"}})
	body := decodeBody(t, rec)
	if rec.Code != http.StatusOK || dig(t, body, "week", "posted") != float64(2) || body["bonus_held"] != false {
		t.Fatalf("two days: %d %v", rec.Code, body)
	}

	// Reset clears spins and pretend days, and the zone and wheel stay.
	r.clockAt(ny, "2026-10-07", 20, 30, 0)
	if rec := r.call(http.MethodPost, "/v1/spin", p.token, nil); rec.Code != http.StatusCreated {
		t.Fatalf("spin: %d", rec.Code)
	}
	rec = r.call(http.MethodPost, "/v1/test/reset", p.token, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	today := r.today(p)
	if today["state"] != "open" || today["spin"] != nil || dig(t, today, "week", "posted") != float64(0) || today["time_zone"] != ny {
		t.Fatalf("today after reset: %v", today)
	}
	if rec := r.call(http.MethodPost, "/v1/spin", p.token, nil); rec.Code != http.StatusCreated {
		t.Fatalf("spin after reset: %d %s", rec.Code, rec.Body.String())
	}
}
