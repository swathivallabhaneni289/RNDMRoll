package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

func TestUsername_Suggest_ReturnsFreeSuggestionDerivedFromName(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "suggest@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	NewUsernameHandler(users).Register(group)

	req := httptest.NewRequest(http.MethodGet, "/v1/usernames/suggest?name=Ada+Lovelace", nil)
	req.Header.Set("Authorization", "Bearer "+mintToken(t, caller.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)

	want := user.NormalizeUsername("Ada Lovelace")
	if body["username"] != want {
		t.Errorf("expected the free base %q, got %v", want, body["username"])
	}
	alternates, ok := body["alternates"].([]any)
	if !ok {
		t.Fatalf("expected alternates to be an array, got %T: %v", body["alternates"], body["alternates"])
	}
	if len(alternates) != 3 {
		t.Errorf("expected 3 alternates, got %d: %v", len(alternates), alternates)
	}
}

func TestUsername_Available_ReportsAvailabilityAndAlternatesWhenTaken(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "checker@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	holder, err := users.Create(context.Background(), "holder2@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	taken := "already_taken"
	if _, err := users.UpdateProfile(context.Background(), holder.ID, user.ProfilePatch{Username: &taken}); err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	NewUsernameHandler(users).Register(group)
	authHeader := "Bearer " + mintToken(t, caller.ID)

	// Taken username: available=false with 3 alternates.
	req := httptest.NewRequest(http.MethodGet, "/v1/usernames/available?username=already_taken", nil)
	req.Header.Set("Authorization", authHeader)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["available"] != false {
		t.Errorf("expected available=false for a taken username, got %v", body["available"])
	}
	alternates, ok := body["alternates"].([]any)
	if !ok {
		t.Fatalf("expected alternates to be an array, got %T: %v", body["alternates"], body["alternates"])
	}
	if len(alternates) != 3 {
		t.Errorf("expected 3 alternates for a taken username, got %d: %v", len(alternates), alternates)
	}

	// Free username: available=true with an empty (not null) alternates array.
	req = httptest.NewRequest(http.MethodGet, "/v1/usernames/available?username=totally_free_1", nil)
	req.Header.Set("Authorization", authHeader)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body = decodeBody(t, w)
	if body["available"] != true {
		t.Errorf("expected available=true for a free username, got %v", body["available"])
	}
	freeAlternates, ok := body["alternates"].([]any)
	if !ok {
		t.Fatalf("expected alternates to be an array (never null) even when free, got %T: %v", body["alternates"], body["alternates"])
	}
	if len(freeAlternates) != 0 {
		t.Errorf("expected no alternates for a free username, got %v", freeAlternates)
	}
}
