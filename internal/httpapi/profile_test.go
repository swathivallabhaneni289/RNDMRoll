package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
// RequireAuth, mirroring the nested authenticated group plan 01-13 builds
// in production (RequireAuth then RequireVerified). This plan's handlers
// only assume RequireAuth ran -- Register itself applies no middleware, so
// the caller (a real server or this test) controls what wraps the group.
func newAuthedGroup(t *testing.T, deps TestDeps) (*gin.Engine, *gin.RouterGroup) {
	t.Helper()
	router := newTestRouter(t, deps)
	group := router.Group("/v1")
	group.Use(middleware.RequireAuth(testJWTSecret))
	return router, group
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("failed to decode JSON response %q: %v", w.Body.String(), err)
	}
	return m
}

// fakeAvatarStore is an in-memory storage.AvatarStore for profile_test.go.
// It records the last call's arguments so createAvatarUploadURL's
// caller-scoping can be asserted without a real S3-compatible endpoint.
type fakeAvatarStore struct {
	ticket *storage.UploadTicket
	err    error

	lastUserID        uuid.UUID
	lastContentType   string
	lastContentLength int64
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
		PublicURL: "https://cdn.example.test/avatars/" + userID.String() + "/abc123.jpg",
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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)

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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)

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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)

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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)
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

	// Update only avatar_url; bio from the previous step must persist.
	req = httptest.NewRequest(http.MethodPatch, "/v1/me", bytes.NewBufferString(`{"avatar_url":"https://cdn.example.test/a.jpg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body = decodeBody(t, w)
	if body["avatar_url"] != "https://cdn.example.test/a.jpg" {
		t.Errorf("expected avatar_url to update, got %v", body["avatar_url"])
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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)

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
	NewProfileHandler(users, &fakeAvatarStore{}).Register(group)

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
	NewProfileHandler(users, avatars).Register(group)

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
	if body["public_url"] == nil || body["public_url"] == "" {
		t.Errorf("expected a non-empty public_url, got %v", body["public_url"])
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
