package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// TestFirstSpinIntegration_NewPersonSpinsAtTenAMOverPostgres walks the first
// spin rule over HTTP against the real database: a brand-new person (no spin
// at all) sees the wheel open with first_spin at 10 AM, spins, and posts the
// entry for that date; the same evening offers no second daily spin; the
// next day waits for 8 PM as usual. A person who already has a spin gets no
// early spin.
func TestFirstSpinIntegration_NewPersonSpinsAtTenAMOverPostgres(t *testing.T) {
	pool := requireIntegrationPool(t)
	ctx := context.Background()
	const ny, day = "America/New_York", "2026-10-07"

	clk := clock.NewFixed(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	store := &fakeEntryPhotoStore{}
	server := buildIntegrationServerWith(t, pool, integrationOptions{clock: clk, entryPhotos: store})
	engine := server.Engine()
	fresh := integrationOnboardAccount(t, engine, "int-first-spin@example.com", "Correct-horse-battery-staple1", "First Spinner", "first_spinner")
	veteran := integrationOnboardAccount(t, engine, "int-first-spin-old@example.com", "Correct-horse-battery-staple1", "Old Spinner", "old_spinner")

	at := func(date string, h, m, s int) {
		t.Helper()
		clk.Set(localInstant(t, ny, date, h, m, s))
	}
	do := func(token, method, path string, body any, want int) map[string]any {
		t.Helper()
		rec := integrationDo(t, engine, method, path, token, body)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body.String(), want)
		}
		return decodeBody(t, rec)
	}
	spinRows := func(email string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from spins where user_id = (select id from users where email = $1)`, email).Scan(&n); err != nil {
			t.Fatalf("count spins: %v", err)
		}
		return n
	}

	do(fresh.AccessToken, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": ny}, http.StatusOK)
	do(veteran.AccessToken, http.MethodPost, "/v1/time-zone", map[string]any{"time_zone": ny}, http.StatusOK)

	// The veteran spun on Monday evening.
	at("2026-10-05", 20, 30, 0)
	do(veteran.AccessToken, http.MethodPost, "/v1/spin", map[string]any{"for_date": "2026-10-05"}, http.StatusCreated)

	// Wednesday 10 AM: the new person's wheel is open with first_spin.
	at(day, 10, 0, 0)
	today := do(fresh.AccessToken, http.MethodGet, "/v1/today", nil, http.StatusOK)
	if today["state"] != "open" || today["first_spin"] != true || today["spin"] != nil {
		t.Fatalf("10 AM before the first spin: %v", today)
	}
	// The veteran waits, and is refused.
	old := do(veteran.AccessToken, http.MethodGet, "/v1/today", nil, http.StatusOK)
	if old["state"] != "waiting" || old["first_spin"] != false {
		t.Fatalf("10 AM, a person with a spin: %v", old)
	}
	wantCode(t, integrationDo(t, engine, http.MethodPost, "/v1/spin", veteran.AccessToken, map[string]any{"for_date": day}), http.StatusConflict, "spin_not_open")

	// The first spin, and the post for that date.
	spun := do(fresh.AccessToken, http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusCreated)
	if dig(t, spun, "today", "state") != "result" || dig(t, spun, "today", "first_spin") != false ||
		dig(t, spun, "today", "spin", "local_date") != day || dig(t, spun, "today", "spin", "spun_local_time") != "10:00 AM" {
		t.Fatalf("the first spin: %v", spun)
	}
	if dig(t, spun, "today", "next_open_weekday") != "Thursday" {
		t.Fatalf("next opening after the first spin: %v", dig(t, spun, "today", "next_open_at"))
	}
	seq := int(dig(t, spun, "today", "spin", "seq").(float64))
	ticket := do(fresh.AccessToken, http.MethodPost, "/v1/entry-photos/upload-url", map[string]any{"content_type": "image/jpeg", "content_length": 5000}, http.StatusOK)
	created := do(fresh.AccessToken, http.MethodPost, "/v1/entries", entryRequest(day, seq, ticket["photo_key"].(string)), http.StatusCreated)
	if dig(t, created, "entry", "local_date") != day || dig(t, created, "entry", "late") != false || dig(t, created, "today", "state") != "posted" {
		t.Fatalf("the post after the first spin: %v", created)
	}

	// 8 PM the same day: no second daily spin.
	at(day, 20, 0, 0)
	evening := do(fresh.AccessToken, http.MethodGet, "/v1/today", nil, http.StatusOK)
	if evening["state"] != "posted" || evening["first_spin"] != false {
		t.Fatalf("8 PM after the first spin: %v", evening)
	}
	again := do(fresh.AccessToken, http.MethodPost, "/v1/spin", map[string]any{"for_date": day}, http.StatusOK)
	if again["already"] != true {
		t.Fatalf("a spin at 8 PM: %v", again)
	}
	if n := spinRows("int-first-spin@example.com"); n != 1 {
		t.Fatalf("spin rows = %d, want 1", n)
	}

	// Thursday: waiting at 10 AM, open at 8 PM.
	at("2026-10-08", 10, 0, 0)
	if got := do(fresh.AccessToken, http.MethodGet, "/v1/today", nil, http.StatusOK); got["state"] != "waiting" || got["first_spin"] != false {
		t.Fatalf("Thursday 10 AM: %v", got)
	}
	wantCode(t, integrationDo(t, engine, http.MethodPost, "/v1/spin", fresh.AccessToken, nil), http.StatusConflict, "spin_not_open")
	at("2026-10-08", 20, 0, 0)
	if got := do(fresh.AccessToken, http.MethodGet, "/v1/today", nil, http.StatusOK); got["state"] != "open" || got["first_spin"] != false {
		t.Fatalf("Thursday 8 PM: %v", got)
	}
	do(fresh.AccessToken, http.MethodPost, "/v1/spin", nil, http.StatusCreated)
	if n := spinRows("int-first-spin@example.com"); n != 2 {
		t.Fatalf("spin rows = %d, want 2", n)
	}
}
