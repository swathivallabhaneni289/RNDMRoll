---
phase: 01-foundation-accounts
plan: 08
subsystem: auth
tags: [gin, bcrypt, jwt, refresh-tokens, rate-limiting, httptest]

requires:
  - phase: 01-foundation-accounts (plan 03)
    provides: internal/user domain model, sentinel errors, Repository/RefreshTokenRepository/EmailVerificationRepository interfaces
  - phase: 01-foundation-accounts (plan 06)
    provides: internal/auth security primitives (HashPassword/ComparePassword, IssueAccessToken, RefreshService), internal/middleware (RequireAuth, RateLimit + key funcs), internal/httpapi error mapping, and the shared testsupport_test.go harness (newTestRouter/TestDeps/fakeUserRepo/fakeRefreshRepo/fakeVerificationRepo/fakeMailer)
provides:
  - internal/httpapi/auth.go: AuthHandler with Signup/Login/Refresh/Logout handlers and a Register method mounting all four routes under their rate limits
  - internal/httpapi.Mailer consumer interface (SendVerificationEmail(ctx, toEmail, token string) error) for plan 01-09's concrete implementation to satisfy
affects: [01-13]

tech-stack:
  added: []
  patterns:
    - "A handler's Register(rg *gin.RouterGroup) method is the sole place its route paths and per-route middleware (rate limits) appear -- no central router file needed for this handler"
    - "Login/Signup read the request body via c.ShouldBindBodyWith(&req, binding.JSON) rather than c.ShouldBindJSON, because KeyByIPAndField's rate-limit key function drains the body via ShouldBindBodyWith first; using the same read path in every handler keeps this from becoming a footgun if a future edit changes which route uses which key function"
    - "Login performs a real bcrypt comparison against a fixed dummy hash on the account-miss path before returning the same user.ErrInvalidCredentials as a wrong password, so the two cases are indistinguishable by both response body and timing"

key-files:
  created:
    - internal/httpapi/auth.go
    - internal/httpapi/auth_test.go
  modified: []

key-decisions:
  - "Mailer declared as SendVerificationEmail(ctx, toEmail, token string) error, not the plan's literal SendVerificationEmail(ctx, *user.User) error -- see Deviations. Recorded verbatim here for plan 01-13: `type Mailer interface { SendVerificationEmail(ctx context.Context, toEmail, token string) error }`, matching testsupport_test.go's existing `mailer` interface and fakeMailer exactly."
  - "AuthHandler depends on user.EmailVerificationRepository (not listed in the plan's struct field description) because the Mailer signature above takes a token string, not a *user.User -- Signup generates a 256-bit crypto/rand token, hashes it, and persists it via this repository before handing the raw token to the mailer, mirroring internal/auth/refresh.go's token-generation shape"
  - "A password over bcrypt's 72-byte ceiling that still passes binding's max=72 (rune count, not byte count, e.g. multi-byte-rune input) maps to validation_failed rather than a 500 -- Signup checks errors.Is(err, auth.ErrPasswordTooLong) explicitly before falling through to the generic error path"
  - "ACCT-01 marked complete in REQUIREMENTS.md: this plan delivers the full password-path create-account/log-in/stay-logged-in-across-sessions surface (signup, login, refresh rotation, logout). Sibling wave-4 plans 01-09/01-10/01-11 may also touch ACCT-01 or REQUIREMENTS.md's other rows in the same wave; if the orchestrator hits a merge conflict on this file, the intended end state has ACCT-01 checked and unrelated rows untouched by this plan."

patterns-established:
  - "Rate-limit-body-drain hazard: any route wrapped in RateLimit(LimitConfig{KeyFunc: KeyByIPAndField(...)}) must read its own body in the handler via ShouldBindBodyWith, not ShouldBindJSON, or the handler sees an already-drained reader and every request 400s"

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "Signup creates a user with a bcrypt password hash (never the plaintext) and email_verified false, and requests exactly one verification email; a password under 8 or over 72 bytes is rejected before hashing"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestSignup_ValidBody_Returns201AndHashesPassword"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestSignup_SetsEmailUnverifiedAndSendsVerificationEmailOnce"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestSignup_PasswordUnder8Bytes_Returns400ValidationFailed"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestSignup_PasswordOver72Bytes_Returns400ValidationFailed"
        status: pass
    human_judgment: false
  - id: D2
    description: "Signup with an already-registered address (including a different letter case) returns 409 email_taken instead of a second account"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestSignup_DuplicateEmailDifferentCase_Returns409EmailTaken"
        status: pass
    human_judgment: false
  - id: D3
    description: "Login with correct, verified credentials returns 200 with access_token, refresh_token, expires_in, and the full user object"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogin_CorrectCredentials_Returns200WithTokensAndUser"
        status: pass
    human_judgment: false
  - id: D4
    description: "Login with a wrong password, and login for an address with no account, return byte-identical 401 invalid_credentials bodies -- the account-miss path runs the same bcrypt comparison work as a real mismatch"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogin_WrongPassword_Returns401InvalidCredentials"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogin_UnknownAccountByteIdenticalToWrongPassword"
        status: pass
    human_judgment: false
  - id: D5
    description: "Login for an unverified account returns 403 email_not_verified rather than a session"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogin_UnverifiedAccount_Returns403EmailNotVerified"
        status: pass
    human_judgment: false
  - id: D6
    description: "Refresh exchanges a valid refresh token for a new access+refresh pair, the returned refresh token differs from the one presented, a second use of the same token is rejected as token_invalid, and an expired token is rejected as token_expired"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestRefresh_ValidToken_Returns200WithNewTokenPairDifferentFromPresented"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestRefresh_SecondUseOfSameToken_Returns401TokenInvalid"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestRefresh_ExpiredToken_Returns401TokenExpired"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestRefresh_MissingRefreshToken_Returns400ValidationFailed"
        status: pass
    human_judgment: false
  - id: D7
    description: "Logout revokes the presented refresh token (a later refresh with it 401s) and returns 204 even for an already-invalid token; neither refresh nor logout sits behind RequireAuth"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogout_RevokesToken_SubsequentRefreshReturns401"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogout_AlreadyInvalidToken_Returns204"
        status: pass
      - kind: unit
        ref: "internal/httpapi/auth_test.go#TestLogout_MissingRefreshToken_Returns400ValidationFailed"
        status: pass
      - kind: other
        ref: "grep -nE 'auth/(refresh|logout)[^)]*RequireAuth' internal/httpapi/auth.go (0 matches)"
        status: pass
    human_judgment: false
  - id: D8
    description: "Signup and login routes carry their plan 01-06 rate limits (3/min per IP on signup, 5/min per IP+email on login)"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "grep -c 'RateLimit' internal/httpapi/auth.go (2); grep 'KeyByIPAndField' internal/httpapi/auth.go"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 08: Password-Path Authentication Endpoints Summary

**Signup/login/refresh/logout handlers on Gin with bcrypt hashing, byte-identical unknown-account/wrong-password responses, and refresh-token rotation via the plan 01-06 RefreshService**

## Performance

- **Duration:** 25 min
- **Started:** 2026-09-18T13:23:00Z (approx.)
- **Completed:** 2026-09-18T13:48:03Z
- **Tasks:** 2 (each executed as RED/GREEN TDD)
- **Files modified:** 2 (2 created)

## Accomplishments

- `internal/httpapi/auth.go`: `AuthHandler`/`NewAuthHandler` with a `Register(rg *gin.RouterGroup)` method mounting `POST /auth/signup`, `/auth/login`, `/auth/refresh`, `/auth/logout` -- the only place these route paths appear
- Signup: `bcrypt`-hashes the password (rejecting >72 bytes as `validation_failed`, not a 500), creates the account with `email_verified=false`, generates and persists a hashed/expiring verification token, and sends it via the `Mailer` interface -- a mail-send failure is logged, not surfaced, so a transient provider outage doesn't fail the signup
- Login: looks the account up case-insensitively, runs a real bcrypt comparison against a fixed dummy hash on an account-miss (so the account-miss and wrong-password paths do the same cryptographic work), returns the identical `invalid_credentials` body for both, gates on `EmailVerified`, and otherwise issues an access+refresh pair
- Refresh: redeems and rotates the presented token in one call to `RefreshService.Redeem`, always returning the newly rotated refresh token alongside a fresh access token; `token_expired`/`token_invalid` propagate unchanged for the mobile client's branching logic
- Logout: revokes the presented token and returns 204 unconditionally (including for an already-invalid token), matching `RefreshService.Revoke`'s best-effort tolerance
- 16 test cases in `internal/httpapi/auth_test.go` against the shared `newTestRouter`/`TestDeps`/fakes harness from plan 01-06, each test constructing its own router+fakes to avoid rate-limit bleed between cases

## Task Commits

Each task was committed atomically as RED then GREEN (TDD):

1. **Task 1 (RED): failing tests for signup and login** - `bf4f9d5` (test)
1. **Task 1 (GREEN): signup and login handlers** - `9273e31` (feat)
2. **Task 2 (RED): failing tests for refresh and logout** - `398011e` (test)
2. **Task 2 (GREEN): refresh and logout handlers** - `850ecd0` (feat)

**Plan metadata:** this commit (`docs(01-08)`)

## Files Created/Modified

- `internal/httpapi/auth.go` - `AuthHandler`, `NewAuthHandler`, `Register`, `Signup`, `Login`, `Refresh`, `Logout`, `Mailer` interface, `SignupRequest`/`LoginRequest`/`RefreshRequest`/`LogoutRequest`, `apiUser` mapper, verification-token generation, dummy-hash precomputation
- `internal/httpapi/auth_test.go` - 16 test cases plus shared helpers (`newAuthTestHandler`, `doJSONRequest`, `decodeBody`, `assertValidationFailed`, `assertInvalidCredentials`, `loginAndGetTokens`)

## Decisions Made

See `key-decisions` in frontmatter. Summary: the `Mailer` interface matches `testsupport_test.go`'s already-committed shape (`toEmail, token string`) instead of the plan's literal `*user.User` signature, which required adding an `EmailVerificationRepository` dependency the plan's struct description omitted; a `validation_failed` (not 500) response for a password that clears binding's rune-counted `max=72` but exceeds bcrypt's byte-counted 72-byte ceiling; `ACCT-01` marked complete in `REQUIREMENTS.md` since this plan delivers the full password-path create/login/stay-logged-in surface.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `Mailer` interface signature changed to match the already-shipped test harness**
- **Found during:** Task 1 (before writing `auth.go`, confirmed via advisor consultation)
- **Issue:** PLAN.md specifies `Mailer` as `interface { SendVerificationEmail(ctx context.Context, u *user.User) error }`. Plan 01-06's already-committed `internal/httpapi/testsupport_test.go` declares a structurally different `mailer` interface (`SendVerificationEmail(ctx context.Context, toEmail, token string) error`), and `TestDeps.Mailer`/`fakeMailer` are typed against that shape. The plan's own Task 1 instruction requires writing tests against this shared harness's fakes -- with the plan's literal signature, `fakeMailer` cannot satisfy `AuthHandler`'s dependency, making the plan's own test instruction unsatisfiable. Editing `testsupport_test.go` was not an option: it is owned by plan 01-06 and this wave's file-overlap analysis assigns zero overlap between 01-08 and the other wave-4 plans.
- **Fix:** Declared `Mailer` in `auth.go` with the harness's signature (`toEmail, token string`) instead of the plan's literal signature. This is a strict match to `testsupport_test.go`'s existing `mailer` interface (identical method set, so `deps.Mailer` is assignable to `AuthHandler`'s `Mailer` field with no adapter needed). Since this signature carries a token string rather than a `*user.User`, `Signup` now generates a 256-bit `crypto/rand` verification token itself, persists its hash via a new `user.EmailVerificationRepository` dependency on `AuthHandler` (not in the plan's original struct field list), and passes the raw token and the user's email to the mailer.
- **Files modified:** `internal/httpapi/auth.go`, `internal/httpapi/auth_test.go`
- **Verification:** `TestSignup_SetsEmailUnverifiedAndSendsVerificationEmailOnce` asserts the fake mailer recorded exactly one send; full suite passes.
- **Commit:** `9273e31` (Task 1 GREEN commit)

**2. [Rule 2 - Missing Critical] `KeyByIPAndField`'s body-draining side effect on the login route**
- **Found during:** Task 1, before writing tests (flagged proactively via advisor consultation, not discovered by a failing test)
- **Issue:** `middleware.KeyByIPAndField("email")` (plan 01-06) reads the request body via `c.ShouldBindBodyWith(&body, binding.JSON)` to extract the rate-limit key. If the handler then reads the body with a plain `c.ShouldBindJSON(&req)`, it sees an already-drained reader and every login request would fail binding with a spurious 400.
- **Fix:** All four handlers (`Signup`, `Login`, `Refresh`, `Logout`) read their request bodies via `c.ShouldBindBodyWith(&req, binding.JSON)`, which caches the raw body so a route wrapped in `KeyByIPAndField` and a route that isn't both work the same way. Applying it uniformly (not just on `Login`) avoids this becoming a footgun if a future edit moves which route uses which key function.
- **Files modified:** `internal/httpapi/auth.go`
- **Verification:** `TestLogin_CorrectCredentials_Returns200WithTokensAndUser` (and all other login tests) pass through the actual registered route with its rate-limit middleware attached, not by calling the handler method directly.
- **Commit:** `9273e31` (Task 1 GREEN commit)

**3. [Rule 2 - Missing Critical] `validation_failed` mapping for a rune-vs-byte length mismatch**
- **Found during:** Task 1, code review pass before committing
- **Issue:** Gin's `max=72` binding tag counts UTF-8 runes; bcrypt's ceiling counts bytes. A password with multi-byte runes could pass binding validation yet still exceed 72 bytes, hitting `auth.HashPassword`'s `ErrPasswordTooLong` -- which, unmapped, falls through `RespondError`'s default case to an opaque 500 on ordinary (if unusual) user input.
- **Fix:** `Signup` checks `errors.Is(err, auth.ErrPasswordTooLong)` explicitly and responds `400 validation_failed` instead of falling through to the generic error path.
- **Files modified:** `internal/httpapi/auth.go`
- **Verification:** Code path only; no dedicated multi-byte-rune test was added (the plan's password-length test cases use single-byte-rune ASCII strings, which are already covered by the binding tag alone). Documented here rather than left silent.
- **Commit:** `9273e31` (Task 1 GREEN commit)

---

**Total deviations:** 3 auto-fixed (1 blocking interface-signature conflict, 2 missing-critical correctness gaps)
**Impact on plan:** All three were necessary for the plan's own test instructions and acceptance criteria to be satisfiable at all, or to avoid an opaque 500 on ordinary input. No scope creep beyond what Task 1's own "otherwise issue an access token..." and Task 2's stated behaviors required.

## Issues Encountered

None beyond the three deviations above, each resolved within the task it was found in.

## User Setup Required

None - no external service configuration required. All dependencies (`gin`, `bcrypt`, `golang-jwt`, `golang.org/x/time/rate`) were already present in `go.mod` from plans 01-01/01-06.

## Next Phase Readiness

- Plan 01-13 (mailer wiring) should implement `internal/mail`'s concrete type against the `Mailer` interface recorded verbatim in this SUMMARY's key-decisions -- `SendVerificationEmail(ctx context.Context, toEmail, token string) error` -- which matches `testsupport_test.go`'s `mailer` interface exactly, not the original PLAN.md's `*user.User`-based signature.
- The mobile client (plan 01-05) can complete a full launch-restore cycle against these four endpoints: `POST /v1/auth/signup`, `/login`, `/refresh`, `/logout`, with no additional server work needed for the password path.
- `ACCT-01` is now checked in `REQUIREMENTS.md`. Sibling wave-4 plans (01-09 mail/verify-email, 01-10 OAuth, 01-11 username/profile) may touch other rows of the same file in parallel; if the orchestrator's merge hits a conflict there, this plan's only intended change is the `ACCT-01` checkbox and its traceability-table row.
- No blockers.

## Self-Check: PASSED

- FOUND: internal/httpapi/auth.go
- FOUND: internal/httpapi/auth_test.go
- FOUND: commit bf4f9d5
- FOUND: commit 9273e31
- FOUND: commit 398011e
- FOUND: commit 850ecd0
- go test ./internal/httpapi -run 'TestSignup|TestLogin' -v -count=1 -- PASS (9 cases)
- go test ./internal/httpapi -run 'TestRefresh|TestLogout' -v -count=1 -- PASS (7 cases)
- go test ./internal/httpapi -count=1 -- PASS (full package)
- go build ./... -- PASS
- go vet ./internal/... -- PASS
- grep checks for `binding:"required,min=8,max=72"`, `binding:"required,email"`, `RateLimit` (2x), `KeyByIPAndField`, `ErrEmailNotVerified`, `auth/refresh`, `auth/logout`, `refresh_token`, absence of `RequireAuth` on refresh/logout -- all PASS

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
