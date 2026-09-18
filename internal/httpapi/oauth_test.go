package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// fakeAppleVerifier and fakeGoogleVerifier are local test doubles for the
// plan 01-10 verifier interfaces -- oauth.go must never reach Apple's or
// Google's real network endpoints, so every test wires a canned identity or
// error here instead of a real token.

type fakeAppleVerifier struct {
	identity *auth.AppleIdentity
	err      error
}

func (f *fakeAppleVerifier) Verify(ctx context.Context, identityToken string) (*auth.AppleIdentity, error) {
	return f.identity, f.err
}

type fakeGoogleVerifier struct {
	identity *auth.GoogleIdentity
	err      error
}

func (f *fakeGoogleVerifier) Verify(ctx context.Context, idToken string) (*auth.GoogleIdentity, error) {
	return f.identity, f.err
}

func oauthStrPtr(s string) *string { return &s }

func newOAuthTestHandler(users user.Repository, appleVerifier auth.AppleVerifier, googleVerifier auth.GoogleVerifier) *OAuthHandler {
	refreshService := auth.NewRefreshService(newFakeRefreshRepo(), 720*time.Hour)
	return NewOAuthHandler(users, appleVerifier, googleVerifier, refreshService, []byte("test-secret-at-least-32-bytes!!"), 15*time.Minute)
}

func doOAuthRequest(t *testing.T, users user.Repository, appleVerifier auth.AppleVerifier, googleVerifier auth.GoogleVerifier, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	router := newTestRouter(t, TestDeps{Users: users})
	h := newOAuthTestHandler(users, appleVerifier, googleVerifier)
	h.Register(&router.RouterGroup)

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestOAuth_FirstAppleSignInCreatesVerifiedAccountWithName(t *testing.T) {
	users := newFakeUserRepo()
	appleVerifier := &fakeAppleVerifier{identity: &auth.AppleIdentity{
		Subject:       "apple-sub-1",
		Email:         "person@example.com",
		EmailVerified: true,
	}}

	rec := doOAuthRequest(t, users, appleVerifier, &fakeGoogleVerifier{}, "/auth/oauth/apple", map[string]any{
		"identity_token": "whatever-apple-issued",
		"full_name":      "Alex Doe",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["is_new_user"] != true {
		t.Errorf("is_new_user = %v, want true", resp["is_new_user"])
	}
	userBody, _ := resp["user"].(map[string]any)
	if userBody["name"] != "Alex Doe" {
		t.Errorf("user.name = %v, want Alex Doe", userBody["name"])
	}
	if userBody["email_verified"] != true {
		t.Errorf("user.email_verified = %v, want true", userBody["email_verified"])
	}

	stored, err := users.GetByProviderSubject(context.Background(), user.VerifiedViaApple, "apple-sub-1")
	if err != nil {
		t.Fatalf("GetByProviderSubject: %v", err)
	}
	if stored.EmailVerifiedVia == nil || *stored.EmailVerifiedVia != user.VerifiedViaApple {
		t.Errorf("EmailVerifiedVia = %v, want apple", stored.EmailVerifiedVia)
	}
	if stored.Name == nil || *stored.Name != "Alex Doe" {
		t.Errorf("stored name = %v, want Alex Doe", stored.Name)
	}
}

func TestOAuth_SecondAppleSignInReturnsSameAccountAndKeepsName(t *testing.T) {
	users := newFakeUserRepo()
	appleVerifier := &fakeAppleVerifier{identity: &auth.AppleIdentity{
		Subject:       "apple-sub-2",
		Email:         "person2@example.com",
		EmailVerified: true,
	}}

	first := doOAuthRequest(t, users, appleVerifier, &fakeGoogleVerifier{}, "/auth/oauth/apple", map[string]any{
		"identity_token": "first-token",
		"full_name":      "Sam Rivera",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first sign-in status = %d, body = %s", first.Code, first.Body.String())
	}
	var firstResp map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	firstUser, _ := firstResp["user"].(map[string]any)
	firstID := firstUser["id"]

	// Apple returns null for full_name on every authorization after the
	// first; the second sign-in in this test simulates that omission.
	second := doOAuthRequest(t, users, appleVerifier, &fakeGoogleVerifier{}, "/auth/oauth/apple", map[string]any{
		"identity_token": "second-token",
	})
	if second.Code != http.StatusOK {
		t.Fatalf("second sign-in status = %d, body = %s", second.Code, second.Body.String())
	}
	var secondResp map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("unmarshal second response: %v", err)
	}
	if secondResp["is_new_user"] != false {
		t.Errorf("is_new_user = %v, want false (existing account)", secondResp["is_new_user"])
	}
	secondUser, _ := secondResp["user"].(map[string]any)
	if secondUser["id"] != firstID {
		t.Errorf("second sign-in id = %v, want %v (same account)", secondUser["id"], firstID)
	}
	if secondUser["name"] != "Sam Rivera" {
		t.Errorf("name after second sign-in = %v, want Sam Rivera (must not be cleared)", secondUser["name"])
	}
}

func TestOAuth_GoogleSignInLinksExistingAccountByEmail(t *testing.T) {
	users := newFakeUserRepo()
	existing, err := users.Create(context.Background(), "shared@example.com", oauthStrPtr("bcrypt-hash"), true, nil)
	if err != nil {
		t.Fatalf("seed existing account: %v", err)
	}

	googleVerifier := &fakeGoogleVerifier{identity: &auth.GoogleIdentity{
		Subject:       "google-sub-1",
		Email:         "shared@example.com",
		EmailVerified: true,
	}}

	rec := doOAuthRequest(t, users, &fakeAppleVerifier{}, googleVerifier, "/auth/oauth/google", map[string]any{
		"id_token": "whatever-google-issued",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["is_new_user"] != false {
		t.Errorf("is_new_user = %v, want false (linked, not created)", resp["is_new_user"])
	}
	userBody, _ := resp["user"].(map[string]any)
	if userBody["id"] != existing.ID.String() {
		t.Errorf("user.id = %v, want %v (same account, linked not duplicated)", userBody["id"], existing.ID)
	}

	linked, err := users.GetByProviderSubject(context.Background(), user.VerifiedViaGoogle, "google-sub-1")
	if err != nil {
		t.Fatalf("GetByProviderSubject after link: %v", err)
	}
	if linked.ID != existing.ID {
		t.Errorf("linked account ID = %v, want %v", linked.ID, existing.ID)
	}
}

func TestOAuth_RequestWithEmailOrProviderFieldInsteadOfTokenIsRejected(t *testing.T) {
	users := newFakeUserRepo()

	rec := doOAuthRequest(t, users, &fakeAppleVerifier{}, &fakeGoogleVerifier{}, "/auth/oauth/apple", map[string]any{
		"email":    "attacker@example.com",
		"provider": "apple",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (identity_token missing)", rec.Code)
	}
}

func TestOAuth_InvalidProviderTokenReturns401TokenInvalid(t *testing.T) {
	users := newFakeUserRepo()
	appleVerifier := &fakeAppleVerifier{err: user.ErrTokenInvalid}

	rec := doOAuthRequest(t, users, appleVerifier, &fakeGoogleVerifier{}, "/auth/oauth/apple", map[string]any{
		"identity_token": "garbage",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "token_invalid" {
		t.Errorf("error = %v, want token_invalid", resp["error"])
	}
}

func TestOAuth_NewSocialAccountHasOnboardingIncomplete(t *testing.T) {
	users := newFakeUserRepo()
	googleVerifier := &fakeGoogleVerifier{identity: &auth.GoogleIdentity{
		Subject:       "google-sub-2",
		Email:         "newperson@example.com",
		EmailVerified: true,
	}}

	rec := doOAuthRequest(t, users, &fakeAppleVerifier{}, googleVerifier, "/auth/oauth/google", map[string]any{
		"id_token": "whatever",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	userBody, _ := resp["user"].(map[string]any)
	if userBody["username"] != nil {
		t.Errorf("username = %v, want nil (no username yet)", userBody["username"])
	}
	if userBody["onboarding_complete"] != false {
		t.Errorf("onboarding_complete = %v, want false", userBody["onboarding_complete"])
	}
}
