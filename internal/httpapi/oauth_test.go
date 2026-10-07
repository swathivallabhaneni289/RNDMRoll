package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

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

// oauthRig wires the oauth, login and refresh handlers over one shared user
// repo and one shared refresh-token repo, so a test can prove what a social
// sign-in does to credentials and sessions that were issued before it.
type oauthRig struct {
	router  *gin.Engine
	users   *fakeUserRepo
	refresh *auth.RefreshService
	apple   *fakeAppleVerifier
	google  *fakeGoogleVerifier
}

func newOAuthRig(t *testing.T) *oauthRig {
	t.Helper()
	users := newFakeUserRepo()
	tokens := newFakeRefreshRepo()
	users.tokens = tokens
	refreshService := auth.NewRefreshService(tokens, 720*time.Hour)
	secret := []byte("test-secret-at-least-32-bytes!!")
	rig := &oauthRig{users: users, refresh: refreshService, apple: &fakeAppleVerifier{}, google: &fakeGoogleVerifier{}}
	rig.router = newTestRouter(t, TestDeps{Users: users})
	NewOAuthHandler(users, rig.apple, rig.google, refreshService, secret, 15*time.Minute).Register(&rig.router.RouterGroup)
	NewAuthHandler(users, refreshService, secret, 15*time.Minute).Register(&rig.router.RouterGroup)
	return rig
}

func (r *oauthRig) post(t *testing.T, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return doJSONRequest(t, r.router, http.MethodPost, path, body)
}

func (r *oauthRig) seedPasswordAccount(t *testing.T, email, password string, verified bool) *user.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	var via *user.VerificationSource
	if verified {
		v := user.VerifiedViaPasswordFlow
		via = &v
	}
	u, err := r.users.Create(context.Background(), email, &hash, verified, via)
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return u
}

func TestOAuth_SocialSignInClaimsUnverifiedPasswordAccount(t *testing.T) {
	providers := []struct {
		name string
		path string
		body map[string]any
		via  user.VerificationSource
		arm  func(r *oauthRig)
	}{
		{
			name: "google", path: "/auth/oauth/google", body: map[string]any{"id_token": "t"}, via: user.VerifiedViaGoogle,
			arm: func(r *oauthRig) {
				r.google.identity = &auth.GoogleIdentity{Subject: "g-claim", Email: "Victim@Example.com", EmailVerified: true}
			},
		},
		{
			name: "apple", path: "/auth/oauth/apple", body: map[string]any{"identity_token": "t"}, via: user.VerifiedViaApple,
			arm: func(r *oauthRig) {
				r.apple.identity = &auth.AppleIdentity{Subject: "a-claim", Email: "Victim@Example.com", EmailVerified: true}
			},
		},
	}
	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			rig := newOAuthRig(t)
			seeded := rig.seedPasswordAccount(t, "victim@example.com", "attacker-password", false)
			// A session minted before the claim, as a pre-registrant could hold.
			oldRefresh, err := rig.refresh.Issue(context.Background(), seeded.ID, nil)
			if err != nil {
				t.Fatalf("issue refresh token: %v", err)
			}
			tc.arm(rig)

			rec := rig.post(t, tc.path, tc.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			resp := decodeBody(t, rec)
			if resp["is_new_user"] != false {
				t.Errorf("is_new_user = %v, want false (linked)", resp["is_new_user"])
			}
			userBody, _ := resp["user"].(map[string]any)
			if userBody["id"] != seeded.ID.String() {
				t.Errorf("user.id = %v, want %v", userBody["id"], seeded.ID)
			}
			if userBody["email_verified"] != true {
				t.Errorf("response email_verified = %v, want true", userBody["email_verified"])
			}

			stored, err := rig.users.GetByID(context.Background(), seeded.ID)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if !stored.EmailVerified {
				t.Error("stored account still unverified")
			}
			if stored.EmailVerifiedVia == nil || *stored.EmailVerifiedVia != tc.via {
				t.Errorf("EmailVerifiedVia = %v, want %s", stored.EmailVerifiedVia, tc.via)
			}
			if stored.PasswordHash != nil {
				t.Error("password hash was kept; the pre-registered password must be discarded")
			}

			login := rig.post(t, "/auth/login", map[string]any{"email": "victim@example.com", "password": "attacker-password"})
			assertInvalidCredentials(t, login)

			refresh := rig.post(t, "/auth/refresh", map[string]any{"refresh_token": oldRefresh})
			if refresh.Code != http.StatusUnauthorized || decodeBody(t, refresh)["error"] != "token_invalid" {
				t.Errorf("old refresh token: status = %d, body = %s, want 401 token_invalid", refresh.Code, refresh.Body.String())
			}
		})
	}
}

func TestOAuth_SocialSignInLeavesVerifiedPasswordAccountUnchanged(t *testing.T) {
	rig := newOAuthRig(t)
	seeded := rig.seedPasswordAccount(t, "owner@example.com", "real-password", true)
	oldRefresh, err := rig.refresh.Issue(context.Background(), seeded.ID, nil)
	if err != nil {
		t.Fatalf("issue refresh token: %v", err)
	}
	rig.google.identity = &auth.GoogleIdentity{Subject: "g-keep", Email: "owner@example.com", EmailVerified: true}

	rec := rig.post(t, "/auth/oauth/google", map[string]any{"id_token": "t"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	stored, err := rig.users.GetByID(context.Background(), seeded.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.PasswordHash == nil || *stored.PasswordHash != *seeded.PasswordHash {
		t.Error("password hash of a verified account changed")
	}
	if stored.EmailVerifiedVia == nil || *stored.EmailVerifiedVia != user.VerifiedViaPasswordFlow {
		t.Errorf("EmailVerifiedVia = %v, want password_flow (unchanged)", stored.EmailVerifiedVia)
	}
	if stored.GoogleSubject == nil || *stored.GoogleSubject != "g-keep" {
		t.Errorf("GoogleSubject = %v, want g-keep (linked)", stored.GoogleSubject)
	}

	login := rig.post(t, "/auth/login", map[string]any{"email": "owner@example.com", "password": "real-password"})
	if login.Code != http.StatusOK {
		t.Errorf("password login after link: status = %d, body = %s, want 200", login.Code, login.Body.String())
	}
	refresh := rig.post(t, "/auth/refresh", map[string]any{"refresh_token": oldRefresh})
	if refresh.Code != http.StatusOK {
		t.Errorf("existing refresh token after link: status = %d, body = %s, want 200", refresh.Code, refresh.Body.String())
	}
}

func TestOAuth_AppleUnverifiedEmailDoesNotLinkToExistingAccount(t *testing.T) {
	rig := newOAuthRig(t)
	seeded := rig.seedPasswordAccount(t, "taken@example.com", "real-password", true)
	rig.apple.identity = &auth.AppleIdentity{Subject: "a-unverified", Email: "taken@example.com", EmailVerified: false}

	rec := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t"})
	if rec.Code != http.StatusConflict || decodeBody(t, rec)["error"] != "email_taken" {
		t.Fatalf("status = %d, body = %s, want 409 email_taken", rec.Code, rec.Body.String())
	}
	stored, err := rig.users.GetByID(context.Background(), seeded.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.AppleSubject != nil {
		t.Errorf("AppleSubject = %v, want nil (an unverified email must not link)", *stored.AppleSubject)
	}
}

func TestOAuth_AppleNewAccountWithEmptyEmailIsRejected(t *testing.T) {
	rig := newOAuthRig(t)
	rig.apple.identity = &auth.AppleIdentity{Subject: "a-no-email", Email: "", EmailVerified: true}

	rec := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t", "full_name": "No Email"})
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["error"] != "validation_failed" {
		t.Fatalf("status = %d, body = %s, want 400 validation_failed", rec.Code, rec.Body.String())
	}
	if _, err := rig.users.GetByProviderSubject(context.Background(), user.VerifiedViaApple, "a-no-email"); err == nil {
		t.Error("an account was created for an Apple identity with no email")
	}
}

func TestOAuth_AppleRepeatSignInWithoutEmailStillResolvesBySubject(t *testing.T) {
	rig := newOAuthRig(t)
	rig.apple.identity = &auth.AppleIdentity{Subject: "a-repeat", Email: "repeat@example.com", EmailVerified: true}
	if rec := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t"}); rec.Code != http.StatusOK {
		t.Fatalf("first sign-in status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Apple may omit the email on later authorizations; the subject alone
	// must still sign the user in.
	rig.apple.identity = &auth.AppleIdentity{Subject: "a-repeat", Email: "", EmailVerified: false}
	rec := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t"})
	if rec.Code != http.StatusOK {
		t.Fatalf("repeat sign-in status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec)["is_new_user"] != false {
		t.Error("repeat sign-in was treated as a new user")
	}
}

func TestOAuth_AppleNewAccountTakesVerifiedFromEmailVerifiedClaim(t *testing.T) {
	cases := []struct {
		name         string
		verified     bool
		wantVerified bool
		wantVia      bool
	}{
		{name: "verified claim", verified: true, wantVerified: true, wantVia: true},
		{name: "unverified claim", verified: false, wantVerified: false, wantVia: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newOAuthRig(t)
			rig.apple.identity = &auth.AppleIdentity{Subject: "a-claim-" + tc.name, Email: "claim@example.com", EmailVerified: tc.verified}

			rec := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t"})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			stored, err := rig.users.GetByProviderSubject(context.Background(), user.VerifiedViaApple, "a-claim-"+tc.name)
			if err != nil {
				t.Fatalf("GetByProviderSubject: %v", err)
			}
			if stored.EmailVerified != tc.wantVerified {
				t.Errorf("EmailVerified = %v, want %v", stored.EmailVerified, tc.wantVerified)
			}
			if (stored.EmailVerifiedVia != nil) != tc.wantVia {
				t.Errorf("EmailVerifiedVia = %v, want set = %v", stored.EmailVerifiedVia, tc.wantVia)
			}
		})
	}
}

// T-01-UAT-01: Apple delivers the name only on the first authorization, so a
// later sign-in (null or empty full_name) must never overwrite or clear it.
func TestOAuth_RepeatAppleSignInNeverOverwritesStoredName(t *testing.T) {
	cases := []struct {
		name   string
		second map[string]any
	}{
		{name: "full_name null", second: map[string]any{"identity_token": "t2", "full_name": nil}},
		{name: "full_name empty string", second: map[string]any{"identity_token": "t2", "full_name": ""}},
		{name: "full_name a different name", second: map[string]any{"identity_token": "t2", "full_name": "Someone Else"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newOAuthRig(t)
			rig.apple.identity = &auth.AppleIdentity{Subject: "a-name", Email: "ada@example.com", EmailVerified: true}

			first := rig.post(t, "/auth/oauth/apple", map[string]any{"identity_token": "t1", "full_name": "Ada Lovelace"})
			if first.Code != http.StatusOK {
				t.Fatalf("first sign-in status = %d, body = %s", first.Code, first.Body.String())
			}
			stored, err := rig.users.GetByProviderSubject(context.Background(), user.VerifiedViaApple, "a-name")
			if err != nil {
				t.Fatalf("GetByProviderSubject: %v", err)
			}
			if stored.Name == nil || *stored.Name != "Ada Lovelace" {
				t.Fatalf("name after first sign-in = %v, want Ada Lovelace", stored.Name)
			}

			second := rig.post(t, "/auth/oauth/apple", tc.second)
			if second.Code != http.StatusOK {
				t.Fatalf("second sign-in status = %d, body = %s", second.Code, second.Body.String())
			}
			secondUser, _ := decodeBody(t, second)["user"].(map[string]any)
			if secondUser["name"] != "Ada Lovelace" {
				t.Errorf("response name = %v, want Ada Lovelace", secondUser["name"])
			}
			stored, err = rig.users.GetByProviderSubject(context.Background(), user.VerifiedViaApple, "a-name")
			if err != nil {
				t.Fatalf("GetByProviderSubject: %v", err)
			}
			if stored.Name == nil || *stored.Name != "Ada Lovelace" {
				t.Errorf("stored name after second sign-in = %v, want Ada Lovelace", stored.Name)
			}
		})
	}
}
