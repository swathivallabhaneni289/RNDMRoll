package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// --- ACC-07 and section 10: only a keyed hash of a reset code is stored ---

// accountsResetStore keeps the code hash each Issue was given, per account,
// and answers Try by comparing with it, as the Postgres store does. Every
// other ResetStore method is left to the nil embedded interface: these tests
// never reach it.
type accountsResetStore struct {
	ResetStore
	mu     sync.Mutex
	issued map[uuid.UUID][]byte
}

func (s *accountsResetStore) Issue(_ context.Context, userID uuid.UUID, codeHash []byte, _, _, _ time.Time, _ int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issued[userID] = bytes.Clone(codeHash)
	return true, nil
}

func (s *accountsResetStore) Try(_ context.Context, userID uuid.UUID, codeHash []byte, _ time.Time, _ int) (bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.issued[userID]
	return ok && hmac.Equal(stored, codeHash), false, nil
}

func (s *accountsResetStore) stored(id uuid.UUID) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.issued[id])
}

// A 6-digit code has a million values, so the stored value must be keyed by
// the server's secret: the code itself, or any plain hash of it, would give
// every live code away to someone who can read password_resets.
func TestAccounts_ResetCodeIsStoredAsAKeyedHash(t *testing.T) {
	users := newFakeUserRepo()
	a := users.seedComplete(t, "keyed-a@example.com", "Keyed A", "keyed_a", validBirthday)
	b := users.seedComplete(t, "keyed-b@example.com", "Keyed B", "keyed_b", validBirthday)
	secret := []byte("accounts-reset-secret-a-32-bytes-long")
	store := &accountsResetStore{issued: map[uuid.UUID][]byte{}}
	box := &captureSender{}
	h := NewPasswordResetHandler(users, store, box, secret)
	h.floor = 0
	router := newTestRouter(t, TestDeps{})
	h.Register(router.Group("/v1"))

	for _, email := range []string{a.Email, b.Email} {
		if rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/password/forgot", map[string]any{"email": email}); rec.Code != http.StatusOK {
			t.Fatalf("forgot %s: %d %s", email, rec.Code, rec.Body.String())
		}
	}
	h.Wait()
	code := box.lastCode(t, a.Email)
	stored := store.stored(a.ID)
	message := "rndmroll-password-reset:" + a.ID.String() + ":" + code

	if len(stored) == 0 {
		t.Fatal("no code hash was stored for the account")
	}
	if bytes.Contains(stored, []byte(code)) {
		t.Fatalf("the stored value %x holds the code %s itself", stored, code)
	}
	for name, plain := range map[string][32]byte{
		"sha256 of the code":                     sha256.Sum256([]byte(code)),
		"sha256 of the account and the code":     sha256.Sum256([]byte(message)),
		"sha256 of the account id and the code":  sha256.Sum256([]byte(a.ID.String() + code)),
		"sha256 of the code and the account id":  sha256.Sum256([]byte(code + a.ID.String())),
		"sha256 of the code without its account": sha256.Sum256([]byte("rndmroll-password-reset::" + code)),
	} {
		if bytes.Equal(stored, plain[:]) {
			t.Fatalf("the stored value is the unkeyed %s", name)
		}
	}

	// The keyed hash: HMAC-SHA256 with the server secret, over the account and the code.
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	if want := mac.Sum(nil); !hmac.Equal(stored, want) {
		t.Fatalf("stored %x, want HMAC-SHA256(secret, %q) = %x", stored, message, want)
	}
	// Keyed: the same account and code under another secret is another value.
	other := NewPasswordResetHandler(users, store, box, []byte("accounts-reset-secret-b-32-bytes-long"))
	if bytes.Equal(other.codeHash(a.ID, code), stored) {
		t.Fatal("another server secret gives the same hash")
	}
	// Bound to the account: the same code for another account is another value.
	if bytes.Equal(h.codeHash(b.ID, code), stored) {
		t.Fatal("the same code gives the same hash for two accounts")
	}
	if bytes.Equal(store.stored(b.ID), stored) {
		t.Fatal("two accounts' codes were stored as the same hash")
	}

	// The emailed code checks out against what was stored (verify hashes it the same way).
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/password/verify", map[string]any{"email": a.Email, "code": code})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify with the emailed code: %d %s", rec.Code, rec.Body.String())
	}
}

// --- Section 10 Security: an unknown account costs the same work ---

// A log in that finds no account (an unknown email or username, text that can
// never be a username) or one without a password compares once, against the
// dummy hash, as a wrong password compares once against the account's own: no
// shortcut tells the two apart by timing.
func TestAccounts_LoginWithNoAccountComparesOnceWithTheDummyHash(t *testing.T) {
	const password = "Correct-horse-battery1"
	ctx := context.Background()
	router, h, deps := newAuthTestHandler(t)
	seedLoginAccount(t, deps, "counted@example.com", "counted_x", password)
	if _, err := deps.Users.CreateComplete(ctx, user.NewAccount{
		Email: "nopass@example.com", PasswordHash: "", Birthday: validBirthday, Name: "No Pass", Username: "no_pass_x",
	}); err != nil {
		t.Fatalf("seed an account without a password: %v", err)
	}
	real, err := deps.Users.GetByEmailCI(ctx, "counted@example.com")
	if err != nil || real.PasswordHash == nil {
		t.Fatalf("read the seeded account: %v", err)
	}

	var seen []string
	h.compare = func(hash, plain string) error {
		seen = append(seen, hash)
		return auth.ComparePassword(hash, plain)
	}
	cases := []struct {
		name, login, wantHash string
	}{
		{"an unknown email", "nobody@example.com", dummyPasswordHash},
		{"an unknown username", "nobody_x", dummyPasswordHash},
		{"text that can never be a username", "not a username!", dummyPasswordHash},
		{"an account without a password, by username", "no_pass_x", dummyPasswordHash},
		{"an account without a password, by email", "nopass@example.com", dummyPasswordHash},
		{"a real account with a wrong password", "counted_x", *real.PasswordHash},
	}
	first := ""
	for i, tc := range cases {
		seen = nil
		// Each from its own address, so no limit answers instead.
		rec := postRawJSON(t, router, fmt.Sprintf("198.51.100.%d:1", 140+i), "/v1/auth/login",
			fmt.Sprintf(`{"login":%q,"password":"wrong-guess-1"}`, tc.login))
		assertInvalidCredentials(t, rec)
		if len(seen) != 1 {
			t.Fatalf("%s: %d password comparisons, want exactly 1", tc.name, len(seen))
		}
		if seen[0] != tc.wantHash {
			t.Fatalf("%s: compared against the wrong hash (the dummy is %t)", tc.name, seen[0] == dummyPasswordHash)
		}
		if first == "" {
			first = rec.Body.String()
		} else if rec.Body.String() != first {
			t.Fatalf("%s: answer %q differs from %q", tc.name, rec.Body.String(), first)
		}
	}
}

// --- ACC-10 and SES-06: a log in racing a deletion or a disable ---

// accountsRaceUsers is the in-memory repository whose GetByID answers, after
// the cancel, whatever the case's again makes of the stored account: as if
// another request changed it between the cancel and the read.
type accountsRaceUsers struct {
	*fakeUserRepo
	again func(stored *user.User) (*user.User, error)
	reads int
}

func (r *accountsRaceUsers) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	r.reads++
	u, err := r.fakeUserRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.again(u)
}

// accountsDeleter is a user.AccountDeleter whose CancelDeletion answers as
// told.
type accountsDeleter struct {
	restored bool
	err      error
	cancels  int
}

func (d *accountsDeleter) RequestDeletion(context.Context, uuid.UUID) (user.DeletionRequest, error) {
	return user.DeletionRequest{}, errors.New("accounts test: RequestDeletion is not used here")
}

func (d *accountsDeleter) CancelDeletion(context.Context, uuid.UUID) (bool, error) {
	d.cancels++
	return d.restored, d.err
}

// When cancelling a deletion clears nothing, log in reads the account again
// and signs in only a normal account: never a disabled one, one past its
// grace period, one still marked for deletion, or one that is gone.
func TestAccounts_LoginWhenTheCancelClearsNothing(t *testing.T) {
	const password = "Correct-horse-battery1"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	keep := func(u *user.User) (*user.User, error) { return u, nil }
	cases := []struct {
		name      string
		deleter   *accountsDeleter // nil: no canceller wired
		again     func(*user.User) (*user.User, error)
		wantCode  int
		wantError string // "" for a session
		restored  bool
		wantReads int
	}{
		{"another log in cancelled it a moment ago", &accountsDeleter{},
			func(u *user.User) (*user.User, error) { u.DeletionRequested = false; return u, nil }, http.StatusOK, "", false, 1},
		{"an admin disabled it meanwhile", &accountsDeleter{},
			func(u *user.User) (*user.User, error) { u.Disabled = true; return u, nil }, http.StatusForbidden, "account_disabled", false, 1},
		{"disabled, with the ask cleared", &accountsDeleter{},
			func(u *user.User) (*user.User, error) { u.Disabled, u.DeletionRequested = true, false; return u, nil }, http.StatusForbidden, "account_disabled", false, 1},
		{"the grace period ended in between", &accountsDeleter{},
			func(u *user.User) (*user.User, error) { u.DeletionDue = true; return u, nil }, http.StatusUnauthorized, "invalid_credentials", false, 1},
		{"past the grace period and disabled", &accountsDeleter{},
			func(u *user.User) (*user.User, error) { u.DeletionDue, u.Disabled = true, true; return u, nil }, http.StatusUnauthorized, "invalid_credentials", false, 1},
		{"still marked for deletion", &accountsDeleter{}, keep, http.StatusUnauthorized, "invalid_credentials", false, 1},
		{"the account is gone", &accountsDeleter{},
			func(*user.User) (*user.User, error) { return nil, user.ErrNotFound }, http.StatusUnauthorized, "invalid_credentials", false, 1},
		{"reading it again fails", &accountsDeleter{},
			func(*user.User) (*user.User, error) { return nil, errors.New("accounts test: the database is down") }, http.StatusInternalServerError, "server_error", false, 1},
		{"the cancel itself fails", &accountsDeleter{err: errors.New("accounts test: the database is down")},
			keep, http.StatusInternalServerError, "server_error", false, 0},
		{"no canceller is wired", nil, keep, http.StatusUnauthorized, "invalid_credentials", false, 1},
		{"the cancel cleared it", &accountsDeleter{restored: true}, keep, http.StatusOK, "", true, 0},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := newFakeUserRepo()
			u, err := users.CreateComplete(context.Background(), user.NewAccount{
				Email: "racer@example.com", PasswordHash: hash, Birthday: validBirthday, Name: "Racer", Username: "racer_x",
			})
			if err != nil {
				t.Fatalf("seed: %v", err)
			}
			users.mu.Lock()
			users.users[u.ID].DeletionRequested = true
			users.mu.Unlock()
			race := &accountsRaceUsers{fakeUserRepo: users, again: tc.again}
			tokens := newFakeRefreshRepo()
			router := newTestRouter(t, TestDeps{})
			h := NewAuthHandler(race, auth.NewRefreshService(tokens, time.Hour), []byte("test-secret"), 15*time.Minute)
			h.clock = frozenClock
			if tc.deleter != nil {
				h.deletions = tc.deleter
			}
			h.Register(router.Group("/v1"))

			rec := postRawJSON(t, router, fmt.Sprintf("203.0.113.%d:1", 10+i), "/v1/auth/login",
				fmt.Sprintf(`{"login":"racer@example.com","password":%q}`, password))
			body := decodeBody(t, rec)
			// Only the status and the error code are printed: a session's answer holds tokens.
			if rec.Code != tc.wantCode {
				t.Fatalf("log in: %d (error %v), want %d", rec.Code, body["error"], tc.wantCode)
			}
			if race.reads != tc.wantReads {
				t.Fatalf("the account was read again %d times, want %d", race.reads, tc.wantReads)
			}
			tokens.mu.Lock()
			issued := len(tokens.rows)
			tokens.mu.Unlock()
			if tc.wantError == "" {
				if _, ok := body["access_token"].(string); !ok || issued != 1 {
					t.Fatalf("want a session (one refresh token), got %d tokens", issued)
				}
				if got, has := body["restored"]; has != tc.restored || (has && got != true) {
					t.Fatalf("restored = %v (present %t), want present %t", got, has, tc.restored)
				}
				return
			}
			if issued != 0 || strings.Contains(rec.Body.String(), "token") {
				t.Fatalf("a refused log in issued %d refresh tokens (error %v)", issued, body["error"])
			}
			if body["error"] != tc.wantError {
				t.Fatalf("error %v, want %s", body["error"], tc.wantError)
			}
			if tc.wantError == "invalid_credentials" {
				// The unknown account's answer, byte for byte.
				unknown := postRawJSON(t, router, fmt.Sprintf("203.0.113.%d:2", 10+i), "/v1/auth/login",
					fmt.Sprintf(`{"login":"nobody-race@example.com","password":%q}`, password))
				if unknown.Code != rec.Code || unknown.Body.String() != rec.Body.String() {
					t.Fatalf("answer %d %q, the unknown account's is %d %q", rec.Code, rec.Body.String(), unknown.Code, unknown.Body.String())
				}
			}
			if tc.wantError == "server_error" && rec.Body.String() != `{"error":"server_error"}` {
				t.Fatalf("a failure answers fixed text only, got %s", rec.Body.String())
			}
		})
	}
}

// --- Section 10 Security: profile photo uploads limited to type and size ---

// A type or a size the store refuses is the app's photo problem (400
// validation_failed), never a server error; anything else the store fails
// with is a fixed 500. Neither hands out an upload link.
func TestAccounts_AvatarUploadURLRefusalsAndFailures(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantErr  string
	}{
		{"a type that is not allowed", storage.ErrUnsupportedContentType, http.StatusBadRequest, "validation_failed"},
		{"a size over the cap", storage.ErrContentTooLarge, http.StatusBadRequest, "validation_failed"},
		{"a wrapped type refusal", fmt.Errorf("presign: %w", storage.ErrUnsupportedContentType), http.StatusBadRequest, "validation_failed"},
		{"a wrapped size refusal", fmt.Errorf("presign: %w", storage.ErrContentTooLarge), http.StatusBadRequest, "validation_failed"},
		{"any other storage failure", errors.New("accounts test: storage is down"), http.StatusInternalServerError, "server_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newProfileRig(t)
			u := rig.seedLegacy(t, "uploader@example.com", "Up Loader", "up_loader")
			rig.avatars.err = tc.err
			rec := rig.do(t, http.MethodPost, "/v1/me/avatar/upload-url", u.ID, map[string]any{"content_type": "image/png", "content_length": 2048})
			body := decodeBody(t, rec)
			if rec.Code != tc.wantCode {
				t.Fatalf("got %d (error %v), want %d", rec.Code, body["error"], tc.wantCode)
			}
			if body["error"] != tc.wantErr {
				t.Fatalf("error %v, want %s", body["error"], tc.wantErr)
			}
			for _, key := range []string{"upload_url", "avatar_key", "expires_in"} {
				if _, has := body[key]; has {
					t.Fatalf("a refusal carries %s: %s", key, rec.Body.String())
				}
			}
			if strings.Contains(rec.Body.String(), "storage") {
				t.Fatalf("the answer echoes the store's error text: %s", rec.Body.String())
			}
			if tc.wantCode == http.StatusInternalServerError && rec.Body.String() != `{"error":"server_error"}` {
				t.Fatalf("a failure answers fixed text only, got %s", rec.Body.String())
			}
			if rig.avatars.lastUserID != u.ID {
				t.Fatalf("the store was asked for %s, want the caller %s", rig.avatars.lastUserID, u.ID)
			}
		})
	}
}
