package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testJWTSecret is shared by profile_test.go and username_test.go.
var testJWTSecret = []byte("test-secret-at-least-32-bytes!!")

// mintToken issues a short-lived access token for userID, shared by
// profile_test.go and username_test.go.
func mintToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	token, err := auth.IssueAccessToken(userID, testJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}
	return token
}

// newAuthedGroup returns a router with a "/v1" group already carrying
// RequireAuth. The production group also carries RequireUser; the profile
// handler loads the account itself when that middleware did not run, so
// these tests exercise it either way. Register itself applies no
// middleware, so the caller (a real server or this test) controls what
// wraps the group.
func newAuthedGroup(t *testing.T, deps TestDeps) (*gin.Engine, *gin.RouterGroup) {
	t.Helper()
	router := newTestRouter(t, deps)
	group := router.Group("/v1")
	group.Use(middleware.RequireAuth(testJWTSecret))
	return router, group
}

// fakeAvatarKeyName is the file name of every key fakeAvatarStore hands out:
// 16 lowercase hex and an extension, like a real one.
const fakeAvatarKeyName = "abcdef0123456789.jpg"

// fakeAvatarKey is the key fakeAvatarStore hands userID, which PATCH /me
// saves for that account.
func fakeAvatarKey(userID string) string {
	return "avatars/" + userID + "/" + fakeAvatarKeyName
}

// newTestProfileHandler builds the profile handler as NewServer leaves it:
// its profile photos are signed with fakePhotoLink.
func newTestProfileHandler(users user.Repository, avatars storage.AvatarStore) *ProfileHandler {
	h := NewProfileHandler(users, avatars)
	h.photoLink = fakePhotoLink
	return h
}

// fakeAvatarStore is an in-memory storage.AvatarStore for profile_test.go.
// It records the last call's arguments so createAvatarUploadURL's
// caller-scoping can be asserted without a real S3-compatible endpoint.
// Every DeleteAvatar call is recorded in order, whatever its key (the fence
// that matters is the handler's), and deleteErr makes them fail.
type fakeAvatarStore struct {
	ticket *storage.UploadTicket
	err    error

	lastUserID        uuid.UUID
	lastContentType   string
	lastContentLength int64

	mu        sync.Mutex
	deleted   []fakeAvatarDelete
	deleteErr error
}

// fakeAvatarDelete is one DeleteAvatar call.
type fakeAvatarDelete struct {
	Owner uuid.UUID
	Key   string
}

func (f *fakeAvatarStore) DeleteAvatar(ctx context.Context, userID uuid.UUID, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, fakeAvatarDelete{Owner: userID, Key: key})
	return f.deleteErr
}

// deletes returns every DeleteAvatar call, in call order.
func (f *fakeAvatarStore) deletes() []fakeAvatarDelete {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.deleted)
}

// failDeletesWith makes every later DeleteAvatar call return err (nil stops
// failing). The call is still recorded.
func (f *fakeAvatarStore) failDeletesWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteErr = err
}

func (f *fakeAvatarStore) PresignAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*storage.UploadTicket, error) {
	f.lastUserID = userID
	f.lastContentType = contentType
	f.lastContentLength = contentLength
	if f.err != nil {
		return nil, f.err
	}
	if f.ticket != nil {
		return f.ticket, nil
	}
	return &storage.UploadTicket{
		UploadURL: "https://fake-bucket.example.test/signed-put?sig=abc",
		Key:       fakeAvatarKey(userID.String()),
		ExpiresIn: 300,
	}, nil
}

var _ storage.AvatarStore = (*fakeAvatarStore)(nil)

func TestProfile_GetMe_ReturnsCallerProfileWithOnboardingComplete(t *testing.T) {
	users := newFakeUserRepo()
	created, err := users.Create(context.Background(), "ada@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	name, username := "Ada Lovelace", "ada"
	if _, err := users.UpdateProfile(context.Background(), created.ID, user.ProfilePatch{Name: &name, Username: &username}); err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+mintToken(t, created.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["id"] != created.ID.String() {
		t.Errorf("expected id %q, got %v", created.ID.String(), body["id"])
	}
	if body["email"] != "ada@example.com" {
		t.Errorf("expected email ada@example.com, got %v", body["email"])
	}
	if body["name"] != "Ada Lovelace" {
		t.Errorf("expected name %q, got %v", "Ada Lovelace", body["name"])
	}
	if body["username"] != "ada" {
		t.Errorf("expected username %q, got %v", "ada", body["username"])
	}
	if body["onboarding_complete"] != true {
		t.Errorf("expected onboarding_complete true, got %v", body["onboarding_complete"])
	}
}

func TestProfile_GetMe_NoBearerTokenReturns401(t *testing.T) {
	users := newFakeUserRepo()
	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProfile_PatchMe_IgnoresUserIDInBody(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "caller@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	other, err := users.Create(context.Background(), "other@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	otherOriginalName := "Original Other Name"
	if _, err := users.UpdateProfile(context.Background(), other.ID, user.ProfilePatch{Name: &otherOriginalName}); err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)

	// The request body carries another user's ID under both common field
	// names an attacker might try. Neither UpdateProfileRequest field
	// exists, so Gin's JSON binder silently ignores them -- the update
	// must still land on the caller's own row.
	payload := `{"id":"` + other.ID.String() + `","user_id":"` + other.ID.String() + `","name":"Caller New Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mintToken(t, caller.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["id"] != caller.ID.String() {
		t.Fatalf("expected the updated row to be the caller's own (%s), got %v", caller.ID, body["id"])
	}
	if body["name"] != "Caller New Name" {
		t.Errorf("expected the caller's name to update, got %v", body["name"])
	}

	untouched, err := users.GetByID(context.Background(), other.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if untouched.Name == nil || *untouched.Name != otherOriginalName {
		t.Errorf("expected other user's row to be untouched, got name %v", untouched.Name)
	}
}

func TestProfile_PatchMe_UpdatesFieldsIndependently(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "indep@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	name, username, bio := "Original Name", "original_user", "Original bio"
	if _, err := users.UpdateProfile(context.Background(), caller.ID, user.ProfilePatch{Name: &name, Username: &username, Bio: &bio}); err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)
	token := "Bearer " + mintToken(t, caller.ID)

	// Update only bio.
	req := httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewBufferString(`{"bio":"Updated bio only"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["bio"] != "Updated bio only" {
		t.Errorf("expected bio to update, got %v", body["bio"])
	}
	if body["name"] != "Original Name" {
		t.Errorf("expected name unchanged, got %v", body["name"])
	}
	if body["username"] != "original_user" {
		t.Errorf("expected username unchanged, got %v", body["username"])
	}

	// Update only avatar_key; bio from the previous step must persist.
	ownAvatar := fakeAvatarKey(caller.ID.String())
	req = httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewBufferString(`{"avatar_key":"`+ownAvatar+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body = decodeBody(t, w)
	if body["avatar_url"] != fakePhotoLink(ownAvatar) {
		t.Errorf("expected avatar_url to be the new photo's signed link, got %v", body["avatar_url"])
	}
	if body["bio"] != "Updated bio only" {
		t.Errorf("expected bio to remain from the previous independent update, got %v", body["bio"])
	}
	if body["name"] != "Original Name" {
		t.Errorf("expected name unchanged, got %v", body["name"])
	}
}

func TestProfile_PatchMe_UsernameTakenReturns409WithAlternates(t *testing.T) {
	users := newFakeUserRepo()
	holder, err := users.Create(context.Background(), "holder@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	taken := "taken_handle"
	if _, err := users.UpdateProfile(context.Background(), holder.ID, user.ProfilePatch{Username: &taken}); err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	caller, err := users.Create(context.Background(), "wantsit@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)

	req := httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewBufferString(`{"username":"taken_handle"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mintToken(t, caller.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["error"] != "username_taken" {
		t.Errorf("expected error code username_taken, got %v", body["error"])
	}
	suggestions, ok := body["suggestions"].([]any)
	if !ok {
		t.Fatalf("expected suggestions to be an array, got %T: %v", body["suggestions"], body["suggestions"])
	}
	if len(suggestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d: %v", len(suggestions), suggestions)
	}
	for _, s := range suggestions {
		if s == "taken_handle" {
			t.Errorf("suggestion must not be the taken username itself: %v", suggestions)
		}
	}
}

func TestProfile_PatchMe_BioOverLimitReturns400(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "toolong@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, &fakeAvatarStore{}).Register(group)

	overLong := strings.Repeat("a", 161)
	payload, err := json.Marshal(map[string]string{"bio": overLong})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mintToken(t, caller.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["error"] != "validation_failed" {
		t.Errorf("expected error code validation_failed, got %v", body["error"])
	}
}

func TestProfile_AvatarUploadURL_ReturnsTicketForCaller(t *testing.T) {
	users := newFakeUserRepo()
	caller, err := users.Create(context.Background(), "avatar@example.com", nil, true, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	avatars := &fakeAvatarStore{}

	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, avatars).Register(group)

	payload := `{"content_type":"image/png","content_length":2048}`
	req := httptest.NewRequest(http.MethodPost, "/v1/me/avatar/upload-url", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mintToken(t, caller.ID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["upload_url"] == nil || body["upload_url"] == "" {
		t.Errorf("expected a non-empty upload_url, got %v", body["upload_url"])
	}
	if body["avatar_key"] != fakeAvatarKey(caller.ID.String()) {
		t.Errorf("expected the ticket's avatar_key in the caller's folder, got %v", body["avatar_key"])
	}
	if got := entrySortedKeys(body); strings.Join(got, ",") != "avatar_key,expires_in,upload_url" {
		t.Errorf("ticket members = %v, want avatar_key, expires_in and upload_url only", got)
	}
	if body["expires_in"] != float64(300) {
		t.Errorf("expected expires_in 300, got %v", body["expires_in"])
	}
	if avatars.lastUserID != caller.ID {
		t.Errorf("expected the ticket to be scoped to the caller %s, got %s", caller.ID, avatars.lastUserID)
	}
	if avatars.lastContentType != "image/png" {
		t.Errorf("expected content type to be forwarded, got %v", avatars.lastContentType)
	}
	if avatars.lastContentLength != 2048 {
		t.Errorf("expected content length to be forwarded, got %v", avatars.lastContentLength)
	}
}

// --- avatar_key, birthday and name rules on PATCH /me ---

// profileRig mounts the profile handler behind RequireAuth over a fake user
// repo wired to a fake refresh-token repo, so a test can prove what happens
// to sessions when an account is removed.
type profileRig struct {
	router  *gin.Engine
	users   *fakeUserRepo
	tokens  *fakeRefreshRepo
	svc     *auth.RefreshService
	avatars *fakeAvatarStore
}

func newProfileRig(t *testing.T) *profileRig {
	t.Helper()
	users := newFakeUserRepo()
	tokens := newFakeRefreshRepo()
	users.tokens = tokens
	avatars := &fakeAvatarStore{}
	router, group := newAuthedGroup(t, TestDeps{Users: users})
	newTestProfileHandler(users, avatars).Register(group)
	return &profileRig{router: router, users: users, tokens: tokens, svc: auth.NewRefreshService(tokens, time.Hour), avatars: avatars}
}

func (r *profileRig) do(t *testing.T, method, path string, id uuid.UUID, body any) *httptest.ResponseRecorder {
	t.Helper()
	return integrationDoAuthedRequest(t, r.router, method, path, mintToken(t, id), body)
}

// seedLegacy stores an account like the three existing dev accounts:
// complete, no birthday on file.
func (r *profileRig) seedLegacy(t *testing.T, email, name, username string) *user.User {
	t.Helper()
	u, err := r.users.Create(context.Background(), email, nil, true, nil)
	if err != nil {
		t.Fatalf("seed legacy account: %v", err)
	}
	if _, err := r.users.UpdateProfile(context.Background(), u.ID, user.ProfilePatch{Name: &name, Username: &username}); err != nil {
		t.Fatalf("complete legacy account: %v", err)
	}
	return u
}

func TestProfile_PatchMe_AvatarKeyMustBeTheCallersOwn(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "av1@example.com", "Av One", "av_one")
	other := rig.seedLegacy(t, "av2@example.com", "Av Two", "av_two")

	own := "avatars/" + caller.ID.String() + "/0123456789abcdef.jpg"
	rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"avatar_key": own})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["avatar_url"] != fakePhotoLink(own) {
		t.Fatalf("own avatar: %d %s", rec.Code, rec.Body.String())
	}

	mine := "avatars/" + caller.ID.String() + "/"
	bad := map[string]any{
		"another person's key":  "avatars/" + other.ID.String() + "/0123456789abcdef.jpg",
		"a full URL":            "https://cdn.example.test/" + own,
		"a signed link":         fakePhotoLink(own),
		"another host's link":   "https://evil.example.org/" + own,
		"a 15 hex name":         mine + "0123456789abcde.jpg",
		"a 17 hex name":         mine + "0123456789abcdef0.jpg",
		"upper case hex":        mine + "0123456789ABCDEF.jpg",
		"a dot dot segment":     mine + "../" + other.ID.String() + "/0123456789abcdef.jpg",
		"encoded dot segments":  mine + "%2e%2e/" + other.ID.String() + "/0123456789abcdef.jpg",
		"a query":               own + "?x=1",
		"a backslash":           mine + "0123456789abcdef\\.jpg",
		"an entry photo key":    "entries/" + caller.ID.String() + "/0123456789abcdef.jpg",
		"another folder":        "uploads/" + caller.ID.String() + "/0123456789abcdef.jpg",
		"just the folder":       mine,
		"an extension over ten": mine + "0123456789abcdef.abcdefghijk",
		"not a key":             "javascript:alert(1)",
		"empty":                 "",
		"a number":              5,
		"an object":             map[string]any{"key": own},
	}
	for name, value := range bad {
		rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"avatar_key": value, "bio": "changed"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
			continue
		}
		if got := decodeBody(t, rec)["field"]; got != "avatar_key" {
			t.Errorf("%s: field = %v, want avatar_key", name, got)
		}
	}
	// The old field is not read at all: it saves nothing.
	if rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"avatar_url": "avatars/" + caller.ID.String() + "/fedcba9876543210.jpg"}); rec.Code != http.StatusOK {
		t.Fatalf("a body with only avatar_url: %d %s", rec.Code, rec.Body.String())
	}
	stored, _ := rig.users.GetByID(context.Background(), caller.ID)
	if stored.AvatarKey == nil || *stored.AvatarKey != own || stored.Bio != nil {
		t.Errorf("a refused avatar_key changed what is stored: key %v, bio %v", stored.AvatarKey, stored.Bio)
	}
}

// GET /me and the PATCH /me answer send the signed link of the stored key,
// and never the key itself. Without a link func the photo is null, never the
// key.
func TestProfile_TheProfilePhotoIsSentAsASignedLink(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "signed@example.com", "Signed Photo", "signed_photo")
	key := "avatars/" + caller.ID.String() + "/0123456789abcdef.png"
	if _, err := rig.users.UpdateProfile(context.Background(), caller.ID, user.ProfilePatch{AvatarKey: &key}); err != nil {
		t.Fatal(err)
	}
	for label, rec := range map[string]*httptest.ResponseRecorder{
		"GET /v1/me":   rig.do(t, http.MethodGet, "/v1/me", caller.ID, nil),
		"PATCH /v1/me": rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"bio": "hello"}),
	} {
		if rec.Code != http.StatusOK || decodeBody(t, rec)["avatar_url"] != fakePhotoLink(key) {
			t.Fatalf("%s: %d %s, want avatar_url %s", label, rec.Code, rec.Body.String(), fakePhotoLink(key))
		}
		if strings.Count(rec.Body.String(), key) != 1 || strings.Contains(rec.Body.String(), "avatar_key") {
			t.Fatalf("%s: the key is sent outside its link: %s", label, rec.Body.String())
		}
	}

	users := rig.users
	router, group := newAuthedGroup(t, TestDeps{Users: users})
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)
	rec := integrationDoAuthedRequest(t, router, http.MethodGet, "/v1/me", mintToken(t, caller.ID), nil)
	body := decodeBody(t, rec)
	if v, present := body["avatar_url"]; rec.Code != http.StatusOK || !present || v != nil {
		t.Fatalf("no link func: %d %s, want avatar_url null", rec.Code, rec.Body.String())
	}
}

// Through the whole server, wired as cmd/api wires it: the ticket hands out a
// key, PATCH /me saves it, and the PATCH answer, GET /me and log in all send
// its signed link, never the key itself.
func TestProfile_TheTicketsKeyComesBackAsASignedLinkEverywhere(t *testing.T) {
	rig := newFullTestRig(t, newFakeUserRepo())
	signup := signUpWithCode(t, rig.srv.Engine(), remoteN(31), signupBody(nil))
	if signup.Code != http.StatusOK {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body.String())
	}
	created := decodeBody(t, signup)
	access, _ := created["access_token"].(string)
	id, _ := dig(t, created, "user", "id").(string)
	if v, present := created["user"].(map[string]any)["avatar_url"]; !present || v != nil {
		t.Fatalf("a new account's avatar_url = %v (present %v), want null", v, present)
	}

	ticket := serveFrom(t, rig.srv, remoteN(32), http.MethodPost, "/v1/me/avatar/upload-url", access, map[string]any{"content_type": "image/jpeg", "content_length": 2048})
	key, _ := decodeBody(t, ticket)["avatar_key"].(string)
	if ticket.Code != http.StatusOK || !regexp.MustCompile(`^avatars/`+id+`/[0-9a-f]{16}\.jpg$`).MatchString(key) {
		t.Fatalf("ticket: %d %s", ticket.Code, ticket.Body.String())
	}

	want := fakePhotoLink(key)
	for label, rec := range map[string]*httptest.ResponseRecorder{
		"PATCH /v1/me": serveFrom(t, rig.srv, remoteN(33), http.MethodPatch, "/v1/me", access, map[string]any{"avatar_key": key}),
		"GET /v1/me":   serveFrom(t, rig.srv, remoteN(34), http.MethodGet, "/v1/me", access, nil),
		"log in": serveFrom(t, rig.srv, remoteN(35), http.MethodPost, "/v1/auth/login", "",
			map[string]any{"login": signupBody(nil)["email"], "password": signupBody(nil)["password"]}),
	} {
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", label, rec.Code, rec.Body.String())
		}
		u := decodeBody(t, rec)
		if label == "log in" {
			u, _ = u["user"].(map[string]any)
		}
		if u["avatar_url"] != want {
			t.Fatalf("%s: avatar_url = %v, want %s", label, u["avatar_url"], want)
		}
		if strings.Count(rec.Body.String(), key) != 1 {
			t.Fatalf("%s: the key is sent outside its link: %s", label, rec.Body.String())
		}
	}
}

// --- the file of a replaced profile photo ---

// A new profile photo removes the file of the one it replaced, once the save
// is done: exactly that old key, for the caller. The first photo, the same
// photo sent again, a save with no photo and a refused photo remove nothing.
func TestProfile_PatchMe_ANewPhotoRemovesTheOldFile(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "old1@example.com", "Old One", "old_one")
	mine := "avatars/" + caller.ID.String() + "/"
	first, second := mine+"0123456789abcdef.jpg", mine+"fedcba9876543210.png"
	patch := func(body map[string]any, want int) {
		t.Helper()
		if rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, body); rec.Code != want {
			t.Fatalf("PATCH %v: %d %s, want %d", body, rec.Code, rec.Body.String(), want)
		}
	}

	patch(map[string]any{"avatar_key": first}, http.StatusOK)
	patch(map[string]any{"avatar_key": first}, http.StatusOK)
	patch(map[string]any{"bio": "No new photo."}, http.StatusOK)
	patch(map[string]any{"avatar_key": mine + "0123456789ABCDEF.jpg"}, http.StatusBadRequest)
	if got := rig.avatars.deletes(); len(got) != 0 {
		t.Fatalf("no photo was replaced, yet the store removed %v", got)
	}

	patch(map[string]any{"avatar_key": second}, http.StatusOK)
	want := []fakeAvatarDelete{{Owner: caller.ID, Key: first}}
	if got := rig.avatars.deletes(); !slices.Equal(got, want) {
		t.Fatalf("removed %v, want only %v", got, want)
	}
	if stored, _ := rig.users.GetByID(context.Background(), caller.ID); stored.AvatarKey == nil || *stored.AvatarKey != second {
		t.Fatalf("stored photo %v, want %s", stored.AvatarKey, second)
	}
}

// A save that fails removes nothing: a new photo sent with a username someone
// else has is 409, and the old photo stays, its file too.
func TestProfile_PatchMe_AFailedSaveKeepsTheOldFile(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "keep1@example.com", "Keep One", "keep_one")
	rig.seedLegacy(t, "keep2@example.com", "Keep Two", "keep_two")
	old := "avatars/" + caller.ID.String() + "/0123456789abcdef.jpg"
	if _, err := rig.users.UpdateProfile(context.Background(), caller.ID, user.ProfilePatch{AvatarKey: &old}); err != nil {
		t.Fatal(err)
	}

	rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{
		"username":   "keep_two",
		"avatar_key": "avatars/" + caller.ID.String() + "/fedcba9876543210.jpg",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("a taken username: %d %s, want 409", rec.Code, rec.Body.String())
	}
	if got := rig.avatars.deletes(); len(got) != 0 {
		t.Fatalf("a failed save removed %v", got)
	}
	if stored, _ := rig.users.GetByID(context.Background(), caller.ID); stored.AvatarKey == nil || *stored.AvatarKey != old {
		t.Fatalf("stored photo %v, want the old %s", stored.AvatarKey, old)
	}
}

// Only a profile photo key in the caller's own folder is ever removed. A
// stored value of any other shape (seeded straight into the fake, which has
// no database check) is never sent to the store, whatever replaces it.
func TestProfile_PatchMe_NeverRemovesAnOldValueOfAnyOtherShape(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "shape1@example.com", "Shape One", "shape_one")
	other := rig.seedLegacy(t, "shape2@example.com", "Shape Two", "shape_two")
	mine := "avatars/" + caller.ID.String() + "/"
	n := 0
	for name, old := range map[string]string{
		"another person's key": "avatars/" + other.ID.String() + "/0123456789abcdef.jpg",
		"an entry photo key":   "entries/" + caller.ID.String() + "/0123456789abcdef.jpg",
		"a full URL":           "https://cdn.example.test/" + mine + "0123456789abcdef.jpg",
		"a signed link":        fakePhotoLink(mine + "0123456789abcdef.jpg"),
		"a dot dot segment":    mine + "../" + other.ID.String() + "/0123456789abcdef.jpg",
		"upper case hex":       mine + "0123456789ABCDEF.jpg",
		"a 15 hex name":        mine + "0123456789abcde.jpg",
		"the folder":           mine,
		"the avatar folder":    "avatars/",
		"empty":                "",
	} {
		if _, err := rig.users.UpdateProfile(context.Background(), caller.ID, user.ProfilePatch{AvatarKey: &old}); err != nil {
			t.Fatal(err)
		}
		n++
		next := fmt.Sprintf("%s%016x.jpg", mine, n)
		if rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"avatar_key": next}); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if got := rig.avatars.deletes(); len(got) != 0 {
			t.Fatalf("%s: the store was asked to remove %v", name, got)
		}
	}
}

// A store that fails to remove the old file does not fail the request: the
// new photo is saved and sent. The log line names neither the key, nor the
// account, nor its email, even when the store's error repeats the key.
func TestProfile_PatchMe_AFailedRemovalStillSavesAndLogsNoKey(t *testing.T) {
	logs := &syncBuffer{}
	previous := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "gone1@example.com", "Gone One", "gone_one")
	old := "avatars/" + caller.ID.String() + "/0123456789abcdef.jpg"
	next := "avatars/" + caller.ID.String() + "/fedcba9876543210.jpg"
	if _, err := rig.users.UpdateProfile(context.Background(), caller.ID, user.ProfilePatch{AvatarKey: &old}); err != nil {
		t.Fatal(err)
	}
	rig.avatars.failDeletesWith(errors.New(`Delete "https://cdn.example.test/photos/` + old + `?x-id=DeleteObject": storage is down`))

	rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"avatar_key": next})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["avatar_url"] != fakePhotoLink(next) {
		t.Fatalf("PATCH while the store fails: %d %s", rec.Code, rec.Body.String())
	}
	if got, want := rig.avatars.deletes(), []fakeAvatarDelete{{Owner: caller.ID, Key: old}}; !slices.Equal(got, want) {
		t.Fatalf("removals %v, want %v", got, want)
	}
	if stored, _ := rig.users.GetByID(context.Background(), caller.ID); stored.AvatarKey == nil || *stored.AvatarKey != next {
		t.Fatalf("stored photo %v, want %s", stored.AvatarKey, next)
	}
	line := logs.String()
	if !strings.Contains(line, "could not remove a replaced profile photo file") || !strings.Contains(line, "storage is down") {
		t.Fatalf("no log line for the failed removal: %q", line)
	}
	for _, private := range []string{old, caller.ID.String(), "gone1@example.com", "gone_one"} {
		if strings.Contains(line, private) {
			t.Fatalf("the log line carries %q: %q", private, line)
		}
	}
}

func TestProfile_PatchMe_NameIsTrimmedAndMustNotBeEmpty(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "nm@example.com", "Old Name", "nm_user")

	rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"name": "   "})
	assertFieldError(t, rec, "name")
	rec = rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"name": strings.Repeat("n", 51)})
	assertFieldError(t, rec, "name")

	rec = rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"name": "  New Name  "})
	if rec.Code != http.StatusOK || decodeBody(t, rec)["name"] != "New Name" {
		t.Fatalf("trim: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProfile_PatchMe_InvalidUsernameNamesTheFieldWithoutEchoingIt(t *testing.T) {
	rig := newProfileRig(t)
	caller := rig.seedLegacy(t, "un@example.com", "Un User", "un_user")

	rec := rig.do(t, http.MethodPatch, "/v1/me", caller.ID, map[string]any{"username": "Not Valid!"})
	assertFieldError(t, rec, "username")
	if strings.Contains(rec.Body.String(), "Not Valid") {
		t.Fatalf("error echoes the input: %s", rec.Body.String())
	}
}

func TestProfile_PatchMe_NULInNameOrBioIs400NotA500(t *testing.T) {
	rig := newProfileRig(t)
	legacy := rig.seedLegacy(t, "nul@example.com", "Nul User", "nul_user")
	rec := rig.do(t, http.MethodPatch, "/v1/me", legacy.ID, map[string]any{"name": "a\u0000b"})
	assertFieldError(t, rec, "name")
	rec = rig.do(t, http.MethodPatch, "/v1/me", legacy.ID, map[string]any{"bio": "a\u0000b"})
	assertFieldError(t, rec, "bio")
	rec = rig.do(t, http.MethodPatch, "/v1/me", legacy.ID, map[string]any{"bio": "line one\nline two"})
	if rec.Code != http.StatusOK {
		t.Fatalf("a bio with a line break should be fine: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProfile_PatchMe_CompleteAccountWithNoBirthdayCanEditButNotSendOne(t *testing.T) {
	rig := newProfileRig(t)
	legacy := rig.seedLegacy(t, "legacy@example.com", "Legacy User", "legacy_user")
	if legacy.HasBirthday {
		t.Fatal("seed should have no birthday")
	}

	own := "avatars/" + legacy.ID.String() + "/0123456789abcdef.png"
	rec := rig.do(t, http.MethodPatch, "/v1/me", legacy.ID, map[string]any{"name": "Legacy Renamed", "bio": "new bio", "avatar_key": own})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["name"] != "Legacy Renamed" || body["bio"] != "new bio" || body["avatar_url"] != fakePhotoLink(own) || body["onboarding_complete"] != true {
		t.Fatalf("unexpected body %v", body)
	}

	rec = rig.do(t, http.MethodPatch, "/v1/me", legacy.ID, map[string]any{"birthday": validBirthday})
	assertFieldError(t, rec, "birthday")
	if rig.users.storedBirthday(legacy.ID) != "" {
		t.Fatal("a complete account stored a birthday")
	}
}

func TestProfile_PatchMe_ABirthdayIsAlwaysRefusedAndNothingElseIsApplied(t *testing.T) {
	values := map[string]string{
		"valid date":  "1995-05-05",
		"under 13":    birthdayYearsAgo(12),
		"nonexistent": "2001-02-29",
		"wrong shape": "05/05/1995",
		"empty":       "",
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			rig := newProfileRig(t)
			account := rig.users.seedComplete(t, "set-once@example.com", "Set Once", "set_once", "1990-01-01")

			rec := rig.do(t, http.MethodPatch, "/v1/me", account.ID, map[string]any{"birthday": value, "name": "Changed Name"})

			assertFieldError(t, rec, "birthday")
			stored, _ := rig.users.GetByID(context.Background(), account.ID)
			if stored.Name == nil || *stored.Name != "Set Once" {
				t.Fatalf("the refused patch was partly applied: name = %v", stored.Name)
			}
			if got := rig.users.storedBirthday(account.ID); got != "1990-01-01" {
				t.Fatalf("stored birthday changed to %q", got)
			}
			if rig.users.count() != 1 {
				t.Fatal("a refused birthday must never delete the account")
			}
		})
	}
}

func TestProfile_AvatarUploadURL_DeletedAccountGets401(t *testing.T) {
	rig := newProfileRig(t)
	gone := rig.seedLegacy(t, "gone2@example.com", "Gone Two", "gone_two")
	rig.users.remove(t, gone.ID)
	rec := rig.do(t, http.MethodPost, "/v1/me/avatar/upload-url", gone.ID, map[string]any{"content_type": "image/png", "content_length": 100})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
