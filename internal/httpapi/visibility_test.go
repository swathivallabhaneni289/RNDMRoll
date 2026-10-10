package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Public or private account (Phase 2.1, plan 01): the sign-up default, the
// value in every user body, the switch on PATCH /me and the limit on that
// route. Every test goes through the full server over the in-memory fakes, so
// the real middleware chain (RequireAuth, RequireUser, the limiters) is in
// play.

// visibilitySignUp signs a person up through the full server and returns the
// decoded response and the access token.
func visibilitySignUp(t *testing.T, srv *Server, email, username, birthday string) (map[string]any, string) {
	t.Helper()
	rec := serveFrom(t, srv, integrationRemote(), http.MethodPost, "/v1/auth/signup", "", signupBody(map[string]any{
		"email":    email,
		"username": username,
		"birthday": birthday,
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("signup %s: expected 200, got %d: %s", email, rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	token, _ := body["access_token"].(string)
	if token == "" {
		t.Fatalf("signup %s: no access_token in %v", email, body)
	}
	return body, token
}

// userOf returns the "user" object of an auth response.
func userOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	u, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected a user object in %v", body)
	}
	return u
}

// patchMeVia sends PATCH /v1/me with body from a fresh address.
func patchMeVia(t *testing.T, srv *Server, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return serveFrom(t, srv, integrationRemote(), http.MethodPatch, "/v1/me", token, body)
}

// getMeVia sends GET /v1/me and decodes a 200 body.
func getMeVia(t *testing.T, srv *Server, token string) map[string]any {
	t.Helper()
	rec := serveFrom(t, srv, integrationRemote(), http.MethodGet, "/v1/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /me: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)
}

// storedVisibility reads the value the repository holds for an email.
func storedVisibility(t *testing.T, repo *fullTestRig, email string) string {
	t.Helper()
	u, err := repo.users.GetByEmailCI(context.Background(), email)
	if err != nil {
		t.Fatalf("read %s: %v", email, err)
	}
	return u.AccountVisibility
}

// dayAfter returns the YYYY-MM-DD date one day after date.
func dayAfter(t *testing.T, date string) string {
	t.Helper()
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("parse %q: %v", date, err)
	}
	return d.AddDate(0, 0, 1).Format("2006-01-02")
}

func TestVisibility_SignupAdultIsPublic(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())

	body, _ := visibilitySignUp(t, rig.srv, "adult@example.com", "adult_one", birthdayYearsAgo(30))

	if got := userOf(t, body)["account_visibility"]; got != "public" {
		t.Fatalf("a 30 year old: account_visibility = %v, want public", got)
	}
	if got := storedVisibility(t, rig, "adult@example.com"); got != "public" {
		t.Fatalf("stored value = %q, want public", got)
	}
}

func TestVisibility_SignupFifteenYearOldIsPrivate(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())

	body, _ := visibilitySignUp(t, rig.srv, "teen@example.com", "teen_one", birthdayYearsAgo(15))

	if got := userOf(t, body)["account_visibility"]; got != "private" {
		t.Fatalf("a 15 year old: account_visibility = %v, want private", got)
	}
	if got := storedVisibility(t, rig, "teen@example.com"); got != "private" {
		t.Fatalf("stored value = %q, want private", got)
	}
}

func TestVisibility_SignupOnTheEighteenthBirthdayIsPublic(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())

	body, _ := visibilitySignUp(t, rig.srv, "eighteen@example.com", "eighteen_one", birthdayYearsAgo(18))

	if got := userOf(t, body)["account_visibility"]; got != "public" {
		t.Fatalf("exactly 18 today: account_visibility = %v, want public", got)
	}
}

func TestVisibility_SignupDayBeforeEighteenthBirthdayIsPrivate(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())

	// One day short of 18: the birthday is the day after the date 18 years ago.
	body, _ := visibilitySignUp(t, rig.srv, "almost@example.com", "almost_one", dayAfter(t, birthdayYearsAgo(18)))

	if got := userOf(t, body)["account_visibility"]; got != "private" {
		t.Fatalf("the day before 18: account_visibility = %v, want private", got)
	}
}

func TestVisibility_BodiesCarryTheValue(t *testing.T) {
	for _, tc := range []struct {
		name, email, username, birthday, want string
	}{
		{"adult", "body-adult@example.com", "body_adult", birthdayYearsAgo(30), "public"},
		{"teen", "body-teen@example.com", "body_teen", birthdayYearsAgo(15), "private"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newFullTestRig(t, newFakeUserRepo())

			signup, token := visibilitySignUp(t, rig.srv, tc.email, tc.username, tc.birthday)
			if got := userOf(t, signup)["account_visibility"]; got != tc.want {
				t.Errorf("sign-up body: %v, want %q", got, tc.want)
			}

			if got := getMeVia(t, rig.srv, token)["account_visibility"]; got != tc.want {
				t.Errorf("GET /me body: %v, want %q", got, tc.want)
			}

			login := serveFrom(t, rig.srv, integrationRemote(), http.MethodPost, "/v1/auth/login", "", map[string]any{
				"email": tc.email, "password": "correct-horse-battery",
			})
			if login.Code != http.StatusOK {
				t.Fatalf("login: %d %s", login.Code, login.Body.String())
			}
			if got := userOf(t, decodeBody(t, login))["account_visibility"]; got != tc.want {
				t.Errorf("log in body: %v, want %q", got, tc.want)
			}

			patch := patchMeVia(t, rig.srv, token, map[string]any{"bio": "hello"})
			if patch.Code != http.StatusOK {
				t.Fatalf("PATCH /me: %d %s", patch.Code, patch.Body.String())
			}
			if got := decodeBody(t, patch)["account_visibility"]; got != tc.want {
				t.Errorf("PATCH /me body: %v, want %q", got, tc.want)
			}
		})
	}
}

func TestVisibility_PatchMeSwitchesBothWays(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	_, token := visibilitySignUp(t, rig.srv, "switch@example.com", "switch_one", birthdayYearsAgo(30))
	before := getMeVia(t, rig.srv, token)
	if before["account_visibility"] != "public" {
		t.Fatalf("expected a new adult to start public, got %v", before["account_visibility"])
	}

	toPrivate := patchMeVia(t, rig.srv, token, map[string]any{"account_visibility": "private"})
	if toPrivate.Code != http.StatusOK {
		t.Fatalf("to private: %d %s", toPrivate.Code, toPrivate.Body.String())
	}
	privateBody := decodeBody(t, toPrivate)
	if privateBody["account_visibility"] != "private" {
		t.Fatalf("to private: body says %v", privateBody["account_visibility"])
	}
	if got := storedVisibility(t, rig, "switch@example.com"); got != "private" {
		t.Fatalf("stored value after the first switch = %q, want private", got)
	}
	if got := getMeVia(t, rig.srv, token)["account_visibility"]; got != "private" {
		t.Fatalf("GET /me after the first switch = %v, want private", got)
	}

	toPublic := patchMeVia(t, rig.srv, token, map[string]any{"account_visibility": "public"})
	if toPublic.Code != http.StatusOK {
		t.Fatalf("to public: %d %s", toPublic.Code, toPublic.Body.String())
	}
	publicBody := decodeBody(t, toPublic)
	if publicBody["account_visibility"] != "public" {
		t.Fatalf("to public: body says %v", publicBody["account_visibility"])
	}
	if got := storedVisibility(t, rig, "switch@example.com"); got != "public" {
		t.Fatalf("stored value after the second switch = %q, want public", got)
	}

	// Nothing else in the body changed on either switch.
	for _, key := range []string{"id", "email", "name", "username", "bio", "avatar_url", "email_verified", "onboarding_complete"} {
		if before[key] != privateBody[key] || before[key] != publicBody[key] {
			t.Errorf("%s changed: before %v, after the switches %v and %v", key, before[key], privateBody[key], publicBody[key])
		}
	}
}

func TestVisibility_PatchMeRefusesAnyOtherValue(t *testing.T) {
	values := map[string]any{
		"empty":        "",
		"capital P":    "Public",
		"all capitals": "PRIVATE",
		"padded":       " private",
		"friends":      "friends",
		"secret":       "secret",
		"number":       5,
		"boolean":      true,
		"object":       map[string]any{"value": "private"},
		"array":        []string{"private"},
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			rig := newFullTestRig(t, newFakeUserRepo())
			_, token := visibilitySignUp(t, rig.srv, "refuse@example.com", "refuse_one", birthdayYearsAgo(30))

			// A bio rides along: a refused patch applies none of it.
			rec := patchMeVia(t, rig.srv, token, map[string]any{"bio": "must not be saved", "account_visibility": value})

			assertFieldError(t, rec, "account_visibility")
			if got := storedVisibility(t, rig, "refuse@example.com"); got != "public" {
				t.Fatalf("a refused value changed the stored value to %q", got)
			}
			u, err := rig.users.GetByEmailCI(context.Background(), "refuse@example.com")
			if err != nil {
				t.Fatalf("read account: %v", err)
			}
			if u.Bio != nil {
				t.Fatalf("a refused patch was partly applied: bio = %q", *u.Bio)
			}
		})
	}
}

func TestVisibility_PatchMeWithoutTheFieldLeavesItAlone(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	_, token := visibilitySignUp(t, rig.srv, "leave@example.com", "leave_one", birthdayYearsAgo(30))
	if rec := patchMeVia(t, rig.srv, token, map[string]any{"account_visibility": "private"}); rec.Code != http.StatusOK {
		t.Fatalf("set private: %d %s", rec.Code, rec.Body.String())
	}

	// A patch of other fields, and a patch that names the field as null, both
	// leave the stored choice alone.
	for name, body := range map[string]map[string]any{
		"bio only":   {"bio": "just a bio"},
		"name only":  {"name": "Someone Else"},
		"null value": {"account_visibility": nil, "bio": "with a null"},
		"empty body": {},
	} {
		rec := patchMeVia(t, rig.srv, token, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if got := decodeBody(t, rec)["account_visibility"]; got != "private" {
			t.Errorf("%s: body says %v, want private", name, got)
		}
		if got := storedVisibility(t, rig, "leave@example.com"); got != "private" {
			t.Errorf("%s: stored value = %q, want private", name, got)
		}
	}
}

func TestVisibility_PatchMeDoesNotTouchOtherAccounts(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	bodyA, tokenA := visibilitySignUp(t, rig.srv, "iso-a@example.com", "iso_a", birthdayYearsAgo(30))
	_, tokenB := visibilitySignUp(t, rig.srv, "iso-b@example.com", "iso_b", birthdayYearsAgo(30))
	idA, _ := userOf(t, bodyA)["id"].(string)

	// B names A's id in the body, the way a caller trying to reach another
	// account would. There is no such field: the switch lands on B.
	rec := patchMeVia(t, rig.srv, tokenB, map[string]any{
		"id": idA, "user_id": idA, "account_visibility": "private",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("B's patch: %d %s", rec.Code, rec.Body.String())
	}

	if got := storedVisibility(t, rig, "iso-b@example.com"); got != "private" {
		t.Errorf("B stored value = %q, want private", got)
	}
	if got := storedVisibility(t, rig, "iso-a@example.com"); got != "public" {
		t.Errorf("A stored value = %q, want public (untouched)", got)
	}
	if got := getMeVia(t, rig.srv, tokenA)["account_visibility"]; got != "public" {
		t.Errorf("A's own GET /me = %v, want public", got)
	}
}

func TestPatchMe_IsRateLimitedPerCaller(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	_, tokenA := visibilitySignUp(t, rig.srv, "limit-a@example.com", "limit_a", birthdayYearsAgo(30))
	_, tokenB := visibilitySignUp(t, rig.srv, "limit-b@example.com", "limit_b", birthdayYearsAgo(30))

	// 30 calls in a minute are answered (each from its own address, so only
	// the per-caller limit can be what stops the 31st).
	for i := 1; i <= 30; i++ {
		rec := patchMeVia(t, rig.srv, tokenA, map[string]any{"bio": "call"})
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: expected 200, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := patchMeVia(t, rig.srv, tokenA, map[string]any{"bio": "one too many"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the 31st call: expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["error"]; got != "rate_limited" {
		t.Fatalf("the 31st call: error = %v, want rate_limited", got)
	}
	// The refused call changed nothing.
	if got := getMeVia(t, rig.srv, tokenA)["bio"]; got != "call" {
		t.Fatalf("bio after the refused call = %v, want the last accepted value", got)
	}

	// A second caller has a bucket of their own.
	other := patchMeVia(t, rig.srv, tokenB, map[string]any{"bio": "me too"})
	if other.Code != http.StatusOK {
		t.Fatalf("a second caller should not be limited: %d %s", other.Code, other.Body.String())
	}
}
