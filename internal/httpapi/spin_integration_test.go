package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// TestSpinIntegration_FullDayOverPostgres walks a person through the day over
// HTTP against the real database: a one-request sign-up, the zone report, the
// wait, the spin and its retry, a pretend week, the bonus spin and the wheel.
// It proves the pieces fit together with the real repository (jsonb
// snapshots, the partial unique index, the state row lock) and that the
// users table is not touched by any of it.
func TestSpinIntegration_FullDayOverPostgres(t *testing.T) {
	pool := requireIntegrationPool(t)
	ctx := context.Background()
	const email = "int-spin@example.com"
	const ny = "America/New_York"

	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	server := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, toolEmails: []string{email}})
	engine := server.Engine()

	session := integrationOnboardAccount(t, engine, email, "Correct-horse-battery-staple1", "Spin Person", "spin_person")
	other := integrationOnboardAccount(t, engine, "int-spin-other@example.com", "Correct-horse-battery-staple1", "Other Person", "other_person")
	usersBefore := integrationUserCount(t, pool)

	at := func(date string, h, m, s int) {
		t.Helper()
		clk.Set(localInstant(t, ny, date, h, m, s))
	}
	do := func(method, path, token string, body any) map[string]any {
		t.Helper()
		rec := integrationDo(t, engine, method, path, token, body)
		if rec.Code >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return decodeBody(t, rec)
	}

	do(http.MethodPost, "/v1/time-zone", session.AccessToken, map[string]any{"time_zone": ny})
	do(http.MethodPost, "/v1/time-zone", other.AccessToken, map[string]any{"time_zone": ny})

	// A spin on Wednesday evening, so the first spin rule (a person who has
	// never spun may spin at once) does not open the wheel early on Friday.
	at("2026-10-07", 20, 30, 0)
	do(http.MethodPost, "/v1/spin", session.AccessToken, map[string]any{"for_date": "2026-10-07"})

	// Four pretend days earn the bonus.
	at("2026-10-08", 12, 0, 0)
	posted := do(http.MethodPost, "/v1/test/posted-days", session.AccessToken, map[string]any{
		"dates": []string{"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08"},
	})
	if posted["bonus_held"] != true {
		t.Fatalf("posted days: %v", posted)
	}

	// Friday: waiting at 4 PM, open at 8, a spin at 8:02.
	at("2026-10-09", 16, 0, 0)
	today := do(http.MethodGet, "/v1/today", session.AccessToken, nil)
	if today["state"] != "waiting" || today["next_open_weekday"] != "Friday" {
		t.Fatalf("4 PM: %v %v", today["state"], today["next_open_weekday"])
	}
	at("2026-10-09", 20, 2, 0)
	spun := do(http.MethodPost, "/v1/spin", session.AccessToken, map[string]any{"for_date": "2026-10-09"})
	daily := dig(t, spun, "today", "spin").(map[string]any)
	if spun["already"] != false || daily["spun_local_time"] != "8:02 PM" || dig(t, spun, "today", "next_open_weekday") != "Saturday" {
		t.Fatalf("spin: %v", spun)
	}
	again := do(http.MethodPost, "/v1/spin", session.AccessToken, nil)
	if again["already"] != true || dig(t, again, "today", "spin", "id") != daily["id"] {
		t.Fatalf("again: %v", again)
	}

	// The bonus spin, and its retry.
	bonusBody := map[string]any{"after_seq": 0, "for_date": "2026-10-09"}
	bonus := do(http.MethodPost, "/v1/spin/bonus", session.AccessToken, bonusBody)
	b := dig(t, bonus, "today", "spin").(map[string]any)
	if bonus["already"] != false || b["seq"] != float64(1) || dig(t, b, "category", "id") == dig(t, daily, "category", "id") {
		t.Fatalf("bonus: %v", bonus)
	}
	retry := do(http.MethodPost, "/v1/spin/bonus", session.AccessToken, bonusBody)
	if retry["already"] != true || dig(t, retry, "today", "spin", "id") != b["id"] {
		t.Fatalf("bonus retry: %v", retry)
	}
	var spinRows, held int
	if err := pool.QueryRow(ctx, `select count(*) from spins`).Scan(&spinRows); err != nil || spinRows != 3 {
		t.Fatalf("spin rows = %d (err %v), want 3", spinRows, err)
	}
	if err := pool.QueryRow(ctx, `select bonus_held from user_spin_state where user_id = (select id from users where email = $1)`, email).Scan(&held); err != nil || held != 0 {
		t.Fatalf("bonus_held = %d (err %v), want 0", held, err)
	}

	// The wheel: an edit after the spin changes the live wheel, not today's.
	wheel := do(http.MethodGet, "/v1/wheel", session.AccessToken, nil)
	cats := wheel["categories"].([]any)
	saved := do(http.MethodPatch, "/v1/wheel", session.AccessToken, map[string]any{
		"version": wheel["version"],
		"categories": []any{
			map[string]any{"id": dig(t, cats[0], "id"), "name": "Dinner", "weight": 4},
			map[string]any{"id": dig(t, cats[1], "id"), "name": "Film", "weight": 2},
			map[string]any{"id": nil, "name": "Walk", "weight": 1},
		},
	})
	if saved["version"] != float64(2) || len(saved["categories"].([]any)) != 3 {
		t.Fatalf("saved: %v", saved)
	}
	after := do(http.MethodGet, "/v1/today", session.AccessToken, nil)
	if w := after["wheel"].([]any); len(w) != 6 || dig(t, w[0], "name") != "Meal" {
		t.Fatalf("today's wheel changed after an edit: %v", w)
	}
	if dig(t, after, "spin", "id") != b["id"] {
		t.Fatalf("today's latest spin is not the bonus spin")
	}

	// The other person is untouched by all of it.
	otherWheel := do(http.MethodGet, "/v1/wheel", other.AccessToken, nil)
	if otherWheel["version"] != float64(1) || len(otherWheel["categories"].([]any)) != 6 {
		t.Fatalf("the other person's wheel: %v", otherWheel)
	}
	otherToday := do(http.MethodGet, "/v1/today", other.AccessToken, nil)
	if otherToday["spin"] != nil {
		t.Fatalf("the other person has a spin: %v", otherToday["spin"])
	}

	// A day later the stamp is last night's and the screen is late, until the
	// person posts: then it waits.
	at("2026-10-10", 0, 30, 0)
	late := do(http.MethodGet, "/v1/today", session.AccessToken, nil)
	if late["state"] != "late" || dig(t, late, "spin", "id") != b["id"] || dig(t, late, "spin", "spun_local_time") != "8:02 PM" {
		t.Fatalf("late: %v", late)
	}
	do(http.MethodPost, "/v1/test/reset", session.AccessToken, map[string]any{})
	// The reset deletes every spin, so the person has never spun again and
	// the first spin rule opens the wheel at once.
	if got := do(http.MethodGet, "/v1/today", session.AccessToken, nil); got["state"] != "open" || got["first_spin"] != true || got["spin"] != nil {
		t.Fatalf("after reset: %v", got)
	}

	// Nothing above touched an account row.
	if n := integrationUserCount(t, pool); n != usersBefore {
		t.Fatalf("user count changed from %d to %d", usersBefore, n)
	}
}
