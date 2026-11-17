package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Section 10 Privacy: people search never matches an email address or any
// part of one. The hidden account's email shares nothing with its username or
// its name, so only a match on the email itself could find it by these words.
func TestAccounts_SearchNeverMatchesEmail(t *testing.T) {
	w := newFeedWorld(t)
	finder := w.person("se_finder", "Finder One")
	integrationOnboardAccount(t, w.engine, "hidden.mailbox@example.com", "Correct-horse-battery-staple1", "Zed Quinn", "se_zed")

	// The account can be found, by its username and by a word of its name.
	for _, q := range []string{"se_zed", "quinn"} {
		if got := w.searchNames(finder, url.QueryEscape(q)); len(got) != 1 || got[0] != "se_zed" {
			t.Fatalf("search %q: %v, want [se_zed]", q, got)
		}
	}
	// Never by its email, or by a part of it.
	for _, q := range []string{
		"hidden",
		"hidden.mailbox",
		"HIDDEN.MAILBOX",
		"mailbox",
		"hidden.mailbox@",
		"hidden.mailbox@example.com",
		"@hidden.mailbox@example.com",
		"example.com",
		"@example.com",
		"example",
	} {
		if got := w.searchNames(finder, url.QueryEscape(q)); len(got) != 0 {
			t.Fatalf("search %q found %v by an email address", q, got)
		}
	}
	// Nor does the exact-username lookup take an email.
	rec := integrationDo(t, w.engine, http.MethodGet, "/v1/people/lookup?username="+url.QueryEscape("hidden.mailbox@example.com"), finder.token, nil)
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "se_zed") || strings.Contains(rec.Body.String(), "Zed Quinn") {
		t.Fatalf("lookup by email: %d %s, want a refusal that names nobody", rec.Code, rec.Body.String())
	}
}

// SES-01: the refresh token lasts 30 days from its last use. A sign-up's
// token ends 30 days from now, and each refresh gives the new token a fresh
// 30 days in the database, however little the old one had left.
func TestAccounts_RefreshGivesTheSessionAFresh30DaysInTheDatabase(t *testing.T) {
	const ttl = 30 * 24 * time.Hour // buildIntegrationServerWith's refresh TTL
	const slack = 5 * time.Second
	ctx := context.Background()
	pool := requireIntegrationPool(t)
	engine := buildIntegrationServer(t, pool).Engine()
	const email = "sliding@example.com"

	liveExpiry := func() time.Time {
		t.Helper()
		var n int
		var expires time.Time
		err := pool.QueryRow(ctx, `
			select count(*), max(expires_at) from refresh_tokens
			where revoked_at is null and user_id = (select id from users where email = $1)`, email).Scan(&n, &expires)
		if err != nil {
			t.Fatalf("read the live refresh token: %v", err)
		}
		if n != 1 {
			t.Fatalf("%d live refresh tokens, want 1", n)
		}
		return expires
	}
	within := func(label string, got, from, to time.Time) {
		t.Helper()
		if got.Before(from.Add(ttl-slack)) || got.After(to.Add(ttl+slack)) {
			t.Fatalf("%s: expires %v, want 30 days after %v (to %v)", label, got, from, to)
		}
	}

	before := time.Now()
	session := integrationOnboardAccount(t, engine, email, "Correct-horse-battery-staple1", "Sliding Session", "sliding_x")
	within("the sign-up's token", liveExpiry(), before, time.Now())

	// As if 29 days and 23 hours had passed since: one hour left.
	if _, err := pool.Exec(ctx, `
		update refresh_tokens set expires_at = now() + interval '1 hour'
		where revoked_at is null and user_id = (select id from users where email = $1)`, email); err != nil {
		t.Fatalf("age the token: %v", err)
	}
	old := liveExpiry()

	before = time.Now()
	rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": session.RefreshToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d (error %v)", rec.Code, decodeBody(t, rec)["error"])
	}
	fresh := liveExpiry()
	within("the refreshed token", fresh, before, time.Now())
	if !fresh.After(old.Add(ttl - 2*time.Hour)) {
		t.Fatalf("the refreshed token ends %v, the old one %v: the 30 days did not start again", fresh, old)
	}

	// A log in starts its own 30 days too.
	before = time.Now()
	login := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": "Correct-horse-battery-staple1"})
	if login.Code != http.StatusOK {
		t.Fatalf("log in: %d (error %v)", login.Code, decodeBody(t, login)["error"])
	}
	var newest time.Time
	if err := pool.QueryRow(ctx, `
		select max(expires_at) from refresh_tokens
		where revoked_at is null and user_id = (select id from users where email = $1)`, email).Scan(&newest); err != nil {
		t.Fatalf("read the log in's token: %v", err)
	}
	within("the log in's token", newest, before, time.Now())
}
