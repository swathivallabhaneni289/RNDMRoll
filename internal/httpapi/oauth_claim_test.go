package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// The claim in findOrCreateAccount is a ClaimAndRevoke (one transaction)
// followed by a subject link. These tests make one write fail once and prove
// the next plain retry converges to a claimed account, which is what the
// write order guarantees.

var errInjected = errors.New("injected repository failure")

// flakyUserRepo fails the named write the first n times it is called.
type flakyUserRepo struct {
	user.Repository
	claimFailures int
	linkFailures  int
}

func (f *flakyUserRepo) ClaimAndRevoke(ctx context.Context, id uuid.UUID, via user.VerificationSource) (bool, error) {
	if f.claimFailures > 0 {
		f.claimFailures--
		return false, errInjected
	}
	return f.Repository.ClaimAndRevoke(ctx, id, via)
}

func (f *flakyUserRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider user.VerificationSource, subject string) error {
	if f.linkFailures > 0 {
		f.linkFailures--
		return errInjected
	}
	return f.Repository.LinkProviderSubject(ctx, id, provider, subject)
}

type flakyClaimRig struct {
	router  *gin.Engine
	users   *fakeUserRepo
	flakyU  *flakyUserRepo
	refresh *auth.RefreshService
	google  *fakeGoogleVerifier
}

func newFlakyClaimRig(t *testing.T) *flakyClaimRig {
	t.Helper()
	users := newFakeUserRepo()
	tokens := newFakeRefreshRepo()
	users.tokens = tokens
	flakyU := &flakyUserRepo{Repository: users}
	refreshService := auth.NewRefreshService(tokens, 720*time.Hour)
	secret := []byte("test-secret-at-least-32-bytes!!")
	rig := &flakyClaimRig{users: users, flakyU: flakyU, refresh: refreshService, google: &fakeGoogleVerifier{}}
	rig.router = newTestRouter(t, TestDeps{Users: users})
	NewOAuthHandler(flakyU, &fakeAppleVerifier{}, rig.google, refreshService, secret, 15*time.Minute).Register(&rig.router.RouterGroup)
	NewAuthHandler(users, refreshService, secret, 15*time.Minute).Register(&rig.router.RouterGroup)
	return rig
}

func (r *flakyClaimRig) signInGoogle(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	return doJSONRequest(t, r.router, http.MethodPost, "/auth/oauth/google", map[string]any{"id_token": "t"})
}

// seedUnverified registers an unverified password account holding one live
// refresh token, as a pre-registrant would, and arms the google verifier
// with a verified identity for the same address.
func (r *flakyClaimRig) seedUnverified(t *testing.T) (*user.User, string) {
	t.Helper()
	hash, err := auth.HashPassword("attacker-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u, err := r.users.Create(context.Background(), "victim@example.com", &hash, false, nil)
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	oldRefresh, err := r.refresh.Issue(context.Background(), u.ID, nil)
	if err != nil {
		t.Fatalf("issue refresh token: %v", err)
	}
	r.google.identity = &auth.GoogleIdentity{Subject: "g-flaky", Email: "victim@example.com", EmailVerified: true}
	return u, oldRefresh
}

func (r *flakyClaimRig) assertClaimed(t *testing.T, id uuid.UUID, oldRefresh string) {
	t.Helper()
	stored, err := r.users.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !stored.EmailVerified {
		t.Error("account is still unverified after the retry")
	}
	if stored.PasswordHash != nil {
		t.Error("password hash was kept after the retry")
	}
	if stored.GoogleSubject == nil || *stored.GoogleSubject != "g-flaky" {
		t.Errorf("GoogleSubject = %v, want g-flaky", stored.GoogleSubject)
	}
	refresh := doJSONRequest(t, r.router, http.MethodPost, "/auth/refresh", map[string]any{"refresh_token": oldRefresh})
	if refresh.Code != http.StatusUnauthorized {
		t.Errorf("old refresh token after the retry: status = %d, body = %s, want 401", refresh.Code, refresh.Body.String())
	}
	login := doJSONRequest(t, r.router, http.MethodPost, "/auth/login", map[string]any{"email": "victim@example.com", "password": "attacker-password"})
	assertInvalidCredentials(t, login)
}

func TestOAuth_ClaimConvergesAfterClaimFailsOnce(t *testing.T) {
	rig := newFlakyClaimRig(t)
	seeded, oldRefresh := rig.seedUnverified(t)
	rig.flakyU.claimFailures = 1

	if rec := rig.signInGoogle(t); rec.Code != http.StatusInternalServerError {
		t.Fatalf("first attempt: status = %d, body = %s, want 500", rec.Code, rec.Body.String())
	}
	// Claim runs before link, so a failed claim leaves the subject unlinked and the
	// retry matches by email again instead of returning early by subject.
	if _, err := rig.users.GetByProviderSubject(context.Background(), user.VerifiedViaGoogle, "g-flaky"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("subject linked after a failed claim: err = %v, want ErrNotFound", err)
	}
	if rec := rig.signInGoogle(t); rec.Code != http.StatusOK {
		t.Fatalf("retry: status = %d, body = %s, want 200", rec.Code, rec.Body.String())
	}
	rig.assertClaimed(t, seeded.ID, oldRefresh)
}

func TestOAuth_ClaimConvergesAfterLinkFailsOnce(t *testing.T) {
	rig := newFlakyClaimRig(t)
	seeded, oldRefresh := rig.seedUnverified(t)
	rig.flakyU.linkFailures = 1

	if rec := rig.signInGoogle(t); rec.Code != http.StatusInternalServerError {
		t.Fatalf("first attempt: status = %d, body = %s, want 500", rec.Code, rec.Body.String())
	}
	if rec := rig.signInGoogle(t); rec.Code != http.StatusOK {
		t.Fatalf("retry: status = %d, body = %s, want 200", rec.Code, rec.Body.String())
	}
	rig.assertClaimed(t, seeded.ID, oldRefresh)
}

// An account already linked by subject but still unverified (the shape a
// half-failed earlier attempt left) is claimed on its next sign-in.
func TestOAuth_SubjectMatchHealsUnverifiedLinkedAccount(t *testing.T) {
	rig := newFlakyClaimRig(t)
	seeded, oldRefresh := rig.seedUnverified(t)
	if err := rig.users.LinkProviderSubject(context.Background(), seeded.ID, user.VerifiedViaGoogle, "g-flaky"); err != nil {
		t.Fatalf("pre-link subject: %v", err)
	}

	rec := rig.signInGoogle(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["is_new_user"] != false {
		t.Errorf("is_new_user = %v, want false", body["is_new_user"])
	}
	if userBody, _ := body["user"].(map[string]any); userBody["email_verified"] != true {
		t.Errorf("response email_verified = %v, want true", userBody["email_verified"])
	}
	rig.assertClaimed(t, seeded.ID, oldRefresh)
}

// The healing path must not claim for an address the provider did not just
// vouch for: an unverified provider claim, or a different stored email.
func TestOAuth_SubjectMatchDoesNotHealWithoutMatchingVerifiedEmail(t *testing.T) {
	cases := []struct {
		name     string
		identity auth.GoogleIdentity
	}{
		{"provider email unverified", auth.GoogleIdentity{Subject: "g-flaky", Email: "victim@example.com", EmailVerified: false}},
		{"different email", auth.GoogleIdentity{Subject: "g-flaky", Email: "other@example.com", EmailVerified: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newFlakyClaimRig(t)
			seeded, _ := rig.seedUnverified(t)
			if err := rig.users.LinkProviderSubject(context.Background(), seeded.ID, user.VerifiedViaGoogle, "g-flaky"); err != nil {
				t.Fatalf("pre-link subject: %v", err)
			}
			identity := tc.identity
			rig.google.identity = &identity

			if rec := rig.signInGoogle(t); rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s, want 200", rec.Code, rec.Body.String())
			}
			stored, err := rig.users.GetByID(context.Background(), seeded.ID)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if stored.EmailVerified || stored.PasswordHash == nil {
				t.Errorf("account was claimed (verified = %v, password kept = %v), want untouched", stored.EmailVerified, stored.PasswordHash != nil)
			}
		})
	}
}

// Claim keeps everything the pre-registrant filled in: name, username, avatar
// and birthday stay, the account stays complete, the old password is gone and
// so is every session issued before the claim.
func TestOAuth_ClaimKeepsProfileAndBirthdayAndEndsOldSessions(t *testing.T) {
	rig := newFlakyClaimRig(t)
	hash, err := auth.HashPassword("attacker-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	bio := "pre-registered bio"
	seeded, err := rig.users.CreateComplete(context.Background(), user.NewAccount{
		Email: "victim@example.com", PasswordHash: hash, Birthday: "1991-03-04",
		Name: "Victim Name", Username: "victim_name", Bio: &bio,
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	avatar := testAvatarBaseURL + "/avatars/" + seeded.ID.String() + "/a.png"
	if _, err := rig.users.UpdateProfile(context.Background(), seeded.ID, user.ProfilePatch{AvatarURL: &avatar}); err != nil {
		t.Fatalf("set avatar: %v", err)
	}
	rig.google.identity = &auth.GoogleIdentity{Subject: "g-flaky", Email: "victim@example.com", EmailVerified: true}

	// The pre-registrant logs in (an unverified account may) and holds a session.
	login := doJSONRequest(t, rig.router, http.MethodPost, "/auth/login", map[string]any{"email": "victim@example.com", "password": "attacker-password"})
	if login.Code != http.StatusOK {
		t.Fatalf("pre-registrant login: %d %s", login.Code, login.Body.String())
	}
	oldRefresh, _ := decodeBody(t, login)["refresh_token"].(string)

	rec := rig.signInGoogle(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("claim: %d %s", rec.Code, rec.Body.String())
	}
	userBody, _ := decodeBody(t, rec)["user"].(map[string]any)
	if userBody["name"] != "Victim Name" || userBody["username"] != "victim_name" || userBody["avatar_url"] != avatar || userBody["onboarding_complete"] != true {
		t.Errorf("claim dropped the profile: %v", userBody)
	}
	stored, err := rig.users.GetByID(context.Background(), seeded.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !stored.HasBirthday || rig.users.storedBirthday(seeded.ID) != "1991-03-04" {
		t.Errorf("claim lost the birthday (stored %q)", rig.users.storedBirthday(seeded.ID))
	}
	if stored.Bio == nil || *stored.Bio != bio {
		t.Errorf("claim lost the bio: %v", stored.Bio)
	}

	refresh := doJSONRequest(t, rig.router, http.MethodPost, "/auth/refresh", map[string]any{"refresh_token": oldRefresh})
	if refresh.Code != http.StatusUnauthorized {
		t.Errorf("pre-registrant refresh after the claim: %d %s, want 401", refresh.Code, refresh.Body.String())
	}
	relogin := doJSONRequest(t, rig.router, http.MethodPost, "/auth/login", map[string]any{"email": "victim@example.com", "password": "attacker-password"})
	assertInvalidCredentials(t, relogin)
}

// Real postgres repositories: a failed claim then a plain retry must end with
// a verified, password-less, linked account that keeps its profile and
// birthday, and no live session from before the claim.
func TestIntegrationOAuth_ClaimRetryEndStateInDatabase(t *testing.T) {
	pool := requireIntegrationPool(t)
	ctx := context.Background()

	realUsers := postgres.NewUserRepo(pool)
	flakyU := &flakyUserRepo{Repository: realUsers}
	refreshService := auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), 720*time.Hour)
	google := &fakeGoogleVerifier{identity: &auth.GoogleIdentity{Subject: "g-db", Email: "victim@example.com", EmailVerified: true}}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewOAuthHandler(flakyU, &fakeAppleVerifier{}, google, refreshService, []byte("test-secret-at-least-32-bytes!!"), 15*time.Minute).Register(&router.RouterGroup)

	hash, err := auth.HashPassword("attacker-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	seeded, err := realUsers.CreateComplete(ctx, user.NewAccount{
		Email: "victim@example.com", PasswordHash: hash, Birthday: "1991-03-04", Name: "Victim Name", Username: "victim_name",
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := refreshService.Issue(ctx, seeded.ID, nil); err != nil {
			t.Fatalf("issue refresh token: %v", err)
		}
	}
	var seededIDs []string
	if err := pool.QueryRow(ctx, `select array_agg(id::text) from refresh_tokens where user_id = $1`, seeded.ID).Scan(&seededIDs); err != nil {
		t.Fatalf("read seeded token ids: %v", err)
	}

	type state struct {
		verified    bool
		hasPassword bool
		googleSub   *string
		name        *string
		birthday    *string
		liveTokens  int
	}
	read := func() state {
		var s state
		const q = `select email_verified, password_hash is not null, google_subject, name, birthday::text,
			(select count(*) from refresh_tokens where user_id = users.id and revoked_at is null) from users where id = $1`
		if err := pool.QueryRow(ctx, q, seeded.ID).Scan(&s.verified, &s.hasPassword, &s.googleSub, &s.name, &s.birthday, &s.liveTokens); err != nil {
			t.Fatalf("read account state: %v", err)
		}
		return s
	}

	flakyU.claimFailures = 1
	first := doJSONRequest(t, router, http.MethodPost, "/auth/oauth/google", map[string]any{"id_token": "t"})
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first attempt: status = %d, body = %s, want 500", first.Code, first.Body.String())
	}
	// Nothing may be half-applied: a retry must still find an unclaimed,
	// unlinked account with its tokens alive.
	if s := read(); s.verified || !s.hasPassword || s.googleSub != nil || s.liveTokens != 2 {
		t.Fatalf("state after failed attempt = %+v, want unverified, password kept, unlinked, 2 live tokens", s)
	}

	retry := doJSONRequest(t, router, http.MethodPost, "/auth/oauth/google", map[string]any{"id_token": "t"})
	if retry.Code != http.StatusOK {
		t.Fatalf("retry: status = %d, body = %s, want 200", retry.Code, retry.Body.String())
	}
	s := read()
	if !s.verified {
		t.Error("email_verified = false after retry")
	}
	if s.hasPassword {
		t.Error("password_hash kept after retry")
	}
	if s.googleSub == nil || *s.googleSub != "g-db" {
		t.Errorf("google_subject = %v, want g-db", s.googleSub)
	}
	if s.name == nil || *s.name != "Victim Name" {
		t.Errorf("name = %v, want kept", s.name)
	}
	if s.birthday == nil || *s.birthday != "1991-03-04" {
		t.Errorf("birthday = %v, want kept", s.birthday)
	}
	// The retry itself issues one fresh session; the seeded ones must be gone.
	if s.liveTokens != 1 {
		t.Errorf("live refresh tokens = %d, want 1 (only the retry's own session)", s.liveTokens)
	}
	var seededLive int
	const seededQ = `select count(*) from refresh_tokens where id::text = any($1) and revoked_at is null`
	if err := pool.QueryRow(ctx, seededQ, seededIDs).Scan(&seededLive); err != nil {
		t.Fatalf("count seeded tokens: %v", err)
	}
	if seededLive != 0 {
		t.Errorf("seeded refresh tokens still live = %d, want 0", seededLive)
	}
}
