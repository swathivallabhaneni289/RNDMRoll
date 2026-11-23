package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// buildChangeServer is buildIntegrationServer with the change-password
// handler and the emails it sent in reach.
func buildChangeServer(t *testing.T, pool *pgxpool.Pool) (*Server, *AccountHandler, *captureSender) {
	t.Helper()
	box := &captureSender{}
	var account *AccountHandler
	server := buildIntegrationServerWith(t, pool, integrationOptions{mail: box,
		wire: func(_ *social.Service, _ *board.Service, deps *Deps) { account = deps.Account }})
	return server, account, box
}

// TestIntegration_ChangePassword_EndToEnd walks the whole change over HTTP
// against the real database: sign up, a second phone logs in, a wrong current
// password changes nothing, the right one replaces the password and every
// session, the new password logs in and the old one does not, only the new
// refresh token redeems, and PATCH /me (including account_visibility) works
// with the new tokens.
func TestIntegration_ChangePassword_EndToEnd(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, account, box := buildChangeServer(t, pool)
	engine := server.Engine()
	ctx := context.Background()

	const email = "change-e2e@example.com"
	const oldPassword, newPassword = "old-Correct-horse-battery1", "new-Correct-horse-battery1"
	phoneA := integrationOnboardAccount(t, engine, email, oldPassword, "Change E2E", "change_e2e")

	// A second phone logs in with the old password.
	loginB := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"email": email, "password": oldPassword})
	if loginB.Code != http.StatusOK {
		t.Fatalf("second phone log in: %d %s", loginB.Code, loginB.Body.String())
	}
	refreshB, _ := decodeBody(t, loginB)["refresh_token"].(string)

	var hashBefore string
	if err := pool.QueryRow(ctx, `select password_hash from users where email = $1`, email).Scan(&hashBefore); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	var activeBefore int
	const activeQ = `select count(*) from refresh_tokens where user_id = (select id from users where email = $1) and revoked_at is null`
	if err := pool.QueryRow(ctx, activeQ, email).Scan(&activeBefore); err != nil || activeBefore != 2 {
		t.Fatalf("active sessions before = %d (err %v), want 2", activeBefore, err)
	}

	// A wrong current password changes nothing and is not a 401.
	wrong := integrationDo(t, engine, http.MethodPost, "/v1/me/password", phoneA.AccessToken, map[string]any{
		"current_password": "not-the-password", "new_password": newPassword,
	})
	if wrong.Code != http.StatusBadRequest || decodeBody(t, wrong)["field"] != "current_password" {
		t.Fatalf("wrong current password: expected 400 current_password, got %d: %s", wrong.Code, wrong.Body.String())
	}
	var hashAfterWrong string
	if err := pool.QueryRow(ctx, `select password_hash from users where email = $1`, email).Scan(&hashAfterWrong); err != nil || hashAfterWrong != hashBefore {
		t.Fatalf("a refused change moved the hash (err %v)", err)
	}

	// The right current password: the change.
	changed := integrationDo(t, engine, http.MethodPost, "/v1/me/password", phoneA.AccessToken, map[string]any{
		"current_password": oldPassword, "new_password": newPassword,
	})
	if changed.Code != http.StatusOK {
		t.Fatalf("change: expected 200, got %d: %s", changed.Code, changed.Body.String())
	}
	changedBody := decodeBody(t, changed)
	newAccess, _ := changedBody["access_token"].(string)
	newRefresh, _ := changedBody["refresh_token"].(string)
	if newAccess == "" || newRefresh == "" {
		t.Fatalf("expected new tokens, got %v", changedBody)
	}

	// Stored as a bcrypt hash of the new password.
	var stored string
	if err := pool.QueryRow(ctx, `select password_hash from users where email = $1`, email).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if stored == newPassword || auth.ComparePassword(stored, newPassword) != nil || auth.ComparePassword(stored, oldPassword) == nil {
		t.Fatal("the stored hash is not a hash of the new password alone")
	}
	var active int
	if err := pool.QueryRow(ctx, activeQ, email).Scan(&active); err != nil || active != 1 {
		t.Fatalf("active sessions after the change = %d (err %v), want exactly 1", active, err)
	}

	// The old password no longer logs in; the new one does.
	old := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"email": email, "password": oldPassword})
	if old.Code != http.StatusUnauthorized || decodeBody(t, old)["error"] != "invalid_credentials" {
		t.Fatalf("log in with the old password: expected 401 invalid_credentials, got %d: %s", old.Code, old.Body.String())
	}
	fresh := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"email": email, "password": newPassword})
	if fresh.Code != http.StatusOK {
		t.Fatalf("log in with the new password: %d %s", fresh.Code, fresh.Body.String())
	}

	// Both old refresh tokens are refused; the new one redeems.
	for name, token := range map[string]string{"this phone's old token": phoneA.RefreshToken, "the other phone's token": refreshB} {
		rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": token})
		if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["error"] != "token_invalid" {
			t.Errorf("%s: expected 401 token_invalid, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
	redeemed := integrationDo(t, engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": newRefresh})
	if redeemed.Code != http.StatusOK {
		t.Fatalf("the new refresh token: %d %s", redeemed.Code, redeemed.Body.String())
	}
	redeemedAccess, _ := decodeBody(t, redeemed)["access_token"].(string)

	// The new tokens carry the rest of the signed-in app: the Settings switch.
	for name, token := range map[string]string{"the access token from the change": newAccess, "the one from the refresh": redeemedAccess} {
		rec := integrationDo(t, engine, http.MethodPatch, "/v1/me", token, map[string]any{"account_visibility": "private"})
		if rec.Code != http.StatusOK || decodeBody(t, rec)["account_visibility"] != "private" {
			t.Fatalf("PATCH /me with %s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	var visibility string
	if err := pool.QueryRow(ctx, `select account_visibility from users where email = $1`, email).Scan(&visibility); err != nil || visibility != "private" {
		t.Fatalf("stored visibility = %q (err %v), want private", visibility, err)
	}
	if n := integrationUserCount(t, pool); n != 1 {
		t.Fatalf("user count = %d, want 1", n)
	}

	// The right password, and only it, emailed the owner once.
	account.Wait()
	box.wantChanged(t, "a change in Settings", email)
}

// A change given up as it commits still goes through, is answered, and
// emails the owner: the commit runs to its end (UserRepo.ChangePassword), as
// a reset's does.
func TestIntegration_ChangePassword_GivenUpAsItCommitsStillChangesAndEmails(t *testing.T) {
	pool := requireIntegrationPool(t)
	server, account, box := buildChangeServer(t, pool)
	engine := server.Engine()
	const email, oldPassword, newPassword = "change-commit@example.com", "old-Correct-horse-battery1", "new-Correct-horse-battery1"
	phone := integrationOnboardAccount(t, engine, email, oldPassword, "Change Commit", "change_commit")

	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	account.passwords = postgres.NewUserRepo(tracedPool(t, &cancelsAtCommit{cancel: cancel}))
	req := integrationRequest(t, http.MethodPost, "/v1/me/password", phone.AccessToken,
		map[string]any{"current_password": oldPassword, "new_password": newPassword}).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	account.Wait()

	if reqCtx.Err() == nil {
		t.Fatal("the request's context never ended: the change did not get as far as its commit")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("a change given up as it commits: %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": newPassword}); rec.Code != http.StatusOK {
		t.Fatalf("the new password: %d %s", rec.Code, rec.Body.String())
	}
	if rec := integrationDo(t, engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": oldPassword}); rec.Code == http.StatusOK {
		t.Fatal("the old password still works")
	}
	box.wantChanged(t, "a change given up as it commits", email)
}
