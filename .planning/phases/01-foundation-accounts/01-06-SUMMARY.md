---
phase: 01-foundation-accounts
plan: 06
subsystem: auth
tags: [bcrypt, jwt, golang-jwt, gin, rate-limiting, refresh-tokens, middleware]

requires:
  - phase: 01-foundation-accounts (plan 03)
    provides: internal/user domain model, sentinel errors, and the Repository/RefreshTokenRepository/EmailVerificationRepository interfaces this plan codes against
provides:
  - internal/auth package: HashPassword, ComparePassword, ErrPasswordTooLong, IssueAccessToken, ParseAccessToken, RefreshService (NewRefreshService, Issue, Redeem, Revoke, RevokeAll)
  - internal/middleware package: RequireAuth, SubjectFromContext, ErrNoSubjectInContext, RateLimit, LimitConfig, KeyByIP, KeyByIPAndField, KeyBySubject
  - internal/httpapi package: Respond, RespondError, RespondValidationError, ErrorCode + 12 wire error-code constants
  - internal/httpapi/testsupport_test.go shared test harness: newTestRouter, TestDeps, fakeUserRepo, fakeRefreshRepo, fakeVerificationRepo, fakeMailer
affects: [01-05, 01-08, 01-09, 01-10, 01-11]

tech-stack:
  added: []
  patterns:
    - "Password/token cryptographic primitives live in internal/auth; Gin-specific request/response concerns (middleware, error mapping) live in internal/middleware and internal/httpapi -- never mixed into internal/auth"
    - "Every internal/user sentinel error maps to exactly one wire error code in internal/httpapi/errors.go via errors.Is, so no handler plan constructs its own gin.H{\"error\": ...} body"
    - "In-memory fakes for a domain repository interface live beside their consumer as unexported types in a _test.go file (fakeUserRepo, fakeRefreshRepo, fakeVerificationRepo, fakeMailer in internal/httpapi/testsupport_test.go), never as a separate mocks package"
    - "Algorithm allow-listing is always two checks together: jwt.WithValidMethods([]string{\"HS256\"}) plus a keyfunc-level type assertion on *jwt.SigningMethodHMAC -- neither alone is treated as sufficient"

key-files:
  created:
    - internal/auth/password.go
    - internal/auth/password_test.go
    - internal/auth/jwt.go
    - internal/auth/jwt_test.go
    - internal/auth/refresh.go
    - internal/auth/refresh_test.go
    - internal/middleware/auth.go
    - internal/middleware/auth_test.go
    - internal/middleware/ratelimit.go
    - internal/middleware/ratelimit_test.go
    - internal/httpapi/errors.go
    - internal/httpapi/testsupport_test.go
  modified: []

key-decisions:
  - "ComparePassword and RefreshService's hashPresentedToken path both collapse a malformed/undecodable input into the single existing sentinel error (user.ErrInvalidCredentials / user.ErrTokenInvalid) rather than a new distinct error, keeping every failure mode indistinguishable to a client -- required by RESEARCH.md's enumeration-resistance guidance"
  - "internal/httpapi/testsupport_test.go declares a local, unexported `mailer` interface (SendVerificationEmail(ctx, toEmail, token string) error) as a stand-in for plan 01-09's concrete Mailer, since internal/mail does not exist yet. 01-09 should either match this signature or update this file's interface and fakeMailer when the real type lands"
  - "REQUIREMENTS.md's ACCT-01 checkbox is left unmodified, matching plan 01-03's precedent: this plan ships security primitives with no HTTP endpoint yet, so marking ACCT-01 (\"user can create an account and log in\") complete here would be a false positive until wave 4's handler plans (01-08 through 01-11) wire these primitives to actual routes"

patterns-established:
  - "Rate limiter key functions are composable first-class values (KeyByIP, KeyByIPAndField, KeyBySubject) passed into one generic RateLimit(cfg) middleware, rather than a bespoke limiter per route"
  - "SubjectFromContext returns (uuid.UUID, error) rather than a bare UUID, so a missing-middleware wiring bug is a caught error, not a silent zero-value identity"

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "Password hashing is bcrypt-only (DefaultCost) with an explicit 72-byte guard; wrong password and a missing/malformed stored hash return the identical error"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_HashDoesNotEqualPlaintext"
        status: pass
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_HashThenCompareAccepts"
        status: pass
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_CompareRejectsWrongPassword"
        status: pass
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_CompareTreatsMissingHashSameAsWrongPassword"
        status: pass
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_HashRejectsPasswordLongerThan72Bytes"
        status: pass
      - kind: unit
        ref: "internal/auth/password_test.go#TestPassword_HashAccepts72ByteBoundary"
        status: pass
    human_judgment: false
  - id: D2
    description: "Access tokens are HS256-only via golang-jwt/v5 with explicit algorithm allow-listing; a hand-crafted alg:none token, a wrong-secret signature, and an expired token are all rejected"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/auth/jwt_test.go#TestJWT_IssueThenParseReturnsSubject"
        status: pass
      - kind: unit
        ref: "internal/auth/jwt_test.go#TestJWT_ParseRejectsAlgNone"
        status: pass
      - kind: unit
        ref: "internal/auth/jwt_test.go#TestJWT_ParseRejectsTokenSignedWithDifferentSecret"
        status: pass
      - kind: unit
        ref: "internal/auth/jwt_test.go#TestJWT_ParseRejectsExpiredToken"
        status: pass
    human_judgment: false
  - id: D3
    description: "Refresh tokens are 256 bits of crypto/rand entropy, stored only as SHA-256 digests, and single-use by construction via the repository's transactional Rotate"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_IssueDoesNotPersistTheRawToken"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_RedeemAcceptsFreshTokenOnce"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_RedeemOfSameTokenTwiceFails"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_RedeemOfExpiredTokenReturnsExpired"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_RedeemOfRevokedTokenReturnsInvalid"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_IssueProducesDistinctTokens"
        status: pass
      - kind: unit
        ref: "internal/auth/refresh_test.go#TestRefresh_RevokeOfAlreadyInvalidTokenStillSucceeds"
        status: pass
    human_judgment: false
  - id: D4
    description: "Protected routes derive identity only from the token subject: a missing header blocks the handler entirely, a valid token exposes the subject, a wrong-secret token is rejected, and reading the subject without the middleware having run returns an explicit error instead of a zero UUID"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/middleware/auth_test.go#TestAuth_MissingHeaderReturns401AndHandlerNeverRuns"
        status: pass
      - kind: unit
        ref: "internal/middleware/auth_test.go#TestAuth_ValidBearerTokenReachesHandlerWithSubject"
        status: pass
      - kind: unit
        ref: "internal/middleware/auth_test.go#TestAuth_TokenSignedByAnotherSecretReturns401"
        status: pass
      - kind: unit
        ref: "internal/middleware/auth_test.go#TestAuth_SubjectFromContextErrorsWhenMiddlewareDidNotRun"
        status: pass
    human_judgment: false
  - id: D5
    description: "Per-key token-bucket rate limiting allows exactly N requests per window and rejects the next with 429; independent keys never share a bucket"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/middleware/ratelimit_test.go#TestRateLimit_AllowsNThenRejectsTheNextWithinWindow"
        status: pass
      - kind: unit
        ref: "internal/middleware/ratelimit_test.go#TestRateLimit_DifferentKeysDoNotShareABucket"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every domain sentinel error maps to one HTTP status and one wire error code, and every code lib/api/types.ts's ApiErrorCode union requires from the server is present in both files"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "grep cross-check of invalid_credentials/email_taken/username_taken/email_not_verified/token_invalid/token_expired/token_consumed/rate_limited/validation_failed/server_error as quoted strings in both lib/api/types.ts and internal/httpapi/errors.go"
        status: pass
    human_judgment: false
  - id: D7
    description: "Shared test harness (newTestRouter/TestDeps plus fakeUserRepo/fakeRefreshRepo/fakeVerificationRepo/fakeMailer) lets each wave 4 handler plan mount its handler on a fresh gin.New() with no cmd/api/main.go and no database"
    verification:
      - kind: other
        ref: "go build ./internal/httpapi && go vet ./internal/httpapi; each fake asserted against its target interface via var _ Interface = (*fake)(nil)"
        status: pass
    human_judgment: true
    rationale: "The fakes' behavioral correctness (e.g. UpdateProfile's username-conflict path, ConsumeByHash's consumed-vs-expired ordering) will be exercised end-to-end by the wave 4 plans that actually call them through real handlers. This plan only proves the harness compiles and each fake satisfies its declared interface."

duration: 32min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 06: Security Primitives Summary

**bcrypt password hashing, HS256 access tokens with explicit algorithm allow-listing, SHA-256-hashed opaque refresh tokens with rotate-on-redeem, Gin auth/rate-limit middleware, and a shared httpapi error contract + test harness for the five wave 4 handler plans**

## Performance

- **Duration:** 32 min (commit-to-commit span across the three TDD tasks; the upfront context-loading/read phase is not separately timestamped)
- **Started:** 2026-09-18T13:26:53Z (first commit)
- **Completed:** 2026-09-18T13:31:33Z (last verification pass)
- **Tasks:** 3 (each executed as RED → GREEN TDD; no task needed a separate REFACTOR commit)
- **Files modified:** 12 (12 created, 0 modified)

## Accomplishments

- `internal/auth`: bcrypt password hashing (`DefaultCost`) with an explicit 72-byte guard (`ErrPasswordTooLong`) that fails before ever calling bcrypt, and `ComparePassword` that returns the identical `user.ErrInvalidCredentials` for a wrong password or an empty/malformed stored hash
- `internal/auth`: HS256 access tokens (`golang-jwt/jwt/v5`) issued and parsed with explicit algorithm allow-listing -- `jwt.WithValidMethods` plus a keyfunc-level `*jwt.SigningMethodHMAC` assertion -- proven by a hand-crafted `alg:none` token that the parser rejects
- `internal/auth`: opaque 256-bit refresh tokens (`crypto/rand`, `base64.RawURLEncoding`) stored only as SHA-256 digests, single-use by construction via the plan 01-03 repository's transactional `Rotate`
- `internal/middleware`: `RequireAuth`/`SubjectFromContext` deriving identity exclusively from the token subject (never a client-supplied ID), and a per-key token-bucket `RateLimit` middleware with three composable key functions (`KeyByIP`, `KeyByIPAndField`, `KeyBySubject`)
- `internal/httpapi`: `RespondError` mapping every `internal/user` sentinel error to its HTTP status and wire code -- all ten codes the client's `ApiErrorCode` union requires are present in both `lib/api/types.ts` and `errors.go` -- plus the `newTestRouter`/`TestDeps`/`fakeUserRepo`/`fakeRefreshRepo`/`fakeVerificationRepo`/`fakeMailer` harness wave 4 plans build on

## Task Commits

Each task was committed atomically as RED then GREEN (TDD, no separate REFACTOR needed):

1. **Task 1 (RED): failing tests for password hashing and access tokens** - `2b401da` (test)
1. **Task 1 (GREEN): bcrypt hashing + HS256 issue/parse** - `679897d` (feat)
2. **Task 2 (RED): failing tests for refresh-token rotation** - `ef5720d` (test)
2. **Task 2 (GREEN): opaque refresh token service** - `f17f26b` (feat)
3. **Task 3 (RED): failing tests for auth middleware, rate limiting, test harness** - `b27d00a` (test)
3. **Task 3 (GREEN): middleware + shared error mapping** - `8edd355` (feat)

**Plan metadata:** this commit (`docs(01-06)`)

## Files Created/Modified

- `internal/auth/password.go` - `HashPassword`, `ComparePassword`, `ErrPasswordTooLong`
- `internal/auth/password_test.go` - 6 test cases covering hash/compare/72-byte-boundary behavior
- `internal/auth/jwt.go` - `IssueAccessToken`, `ParseAccessToken` (HS256-only, algorithm allow-listed)
- `internal/auth/jwt_test.go` - 4 test cases including a hand-crafted `alg:none` token
- `internal/auth/refresh.go` - `RefreshService` (`NewRefreshService`, `Issue`, `Redeem`, `Revoke`, `RevokeAll`)
- `internal/auth/refresh_test.go` - 7 test cases plus an in-memory `fakeRefreshRepo` implementing `user.RefreshTokenRepository`
- `internal/middleware/auth.go` - `RequireAuth`, `SubjectFromContext`, `ErrNoSubjectInContext`
- `internal/middleware/auth_test.go` - 4 test cases
- `internal/middleware/ratelimit.go` - `RateLimit`, `LimitConfig`, `KeyByIP`, `KeyByIPAndField`, `KeyBySubject`
- `internal/middleware/ratelimit_test.go` - 2 test cases
- `internal/httpapi/errors.go` - `Respond`, `RespondError`, `RespondValidationError`, `ErrorCode` + 12 wire code constants
- `internal/httpapi/testsupport_test.go` - `newTestRouter`, `TestDeps`, `fakeUserRepo`, `fakeRefreshRepo`, `fakeVerificationRepo`, `fakeMailer`, local `mailer` interface

## Final Signatures (verbatim, for wave 4)

```go
// internal/auth
func HashPassword(plain string) (string, error)
func ComparePassword(hash, plain string) error
var ErrPasswordTooLong = errors.New("auth: password exceeds 72 bytes")

func IssueAccessToken(userID uuid.UUID, secret []byte, ttl time.Duration) (string, error)
func ParseAccessToken(tokenString string, secret []byte) (uuid.UUID, error)

type RefreshService struct{ /* unexported */ }
func NewRefreshService(repo user.RefreshTokenRepository, ttl time.Duration) *RefreshService
func (s *RefreshService) Issue(ctx context.Context, userID uuid.UUID, userAgent *string) (string, error)
func (s *RefreshService) Redeem(ctx context.Context, token string, userAgent *string) (uuid.UUID, string, error)
func (s *RefreshService) Revoke(ctx context.Context, token string) error
func (s *RefreshService) RevokeAll(ctx context.Context, userID uuid.UUID) error

// internal/middleware
func RequireAuth(secret []byte) gin.HandlerFunc
func SubjectFromContext(c *gin.Context) (uuid.UUID, error)
var ErrNoSubjectInContext = errors.New("middleware: no authenticated subject in context")

type LimitConfig struct { Requests int; Window time.Duration; KeyFunc func(*gin.Context) string }
func RateLimit(cfg LimitConfig) gin.HandlerFunc
func KeyByIP(c *gin.Context) string
func KeyByIPAndField(field string) func(*gin.Context) string
func KeyBySubject(c *gin.Context) string

// internal/httpapi
type ErrorCode string
func Respond(c *gin.Context, status int, body any)
func RespondError(c *gin.Context, err error)
func RespondValidationError(c *gin.Context, err error)

// internal/httpapi (testsupport_test.go, same package -- available to any *_test.go
// file wave 4 plans add under internal/httpapi)
type TestDeps struct {
    Users         user.Repository
    RefreshTokens user.RefreshTokenRepository
    Verifications user.EmailVerificationRepository
    Mailer        mailer // local interface: SendVerificationEmail(ctx, toEmail, token string) error
}
func newTestRouter(t *testing.T, deps TestDeps) *gin.Engine
func newFakeUserRepo() *fakeUserRepo                 // implements user.Repository
func newFakeRefreshRepo() *fakeRefreshRepo           // implements user.RefreshTokenRepository
func newFakeVerificationRepo() *fakeVerificationRepo // implements user.EmailVerificationRepository
func newFakeMailer() *fakeMailer                     // implements mailer
```

Wire error codes in `internal/httpapi/errors.go` (`ErrorCode` constants): `invalid_credentials`, `email_taken`, `username_taken`, `email_not_verified`, `not_found`, `token_invalid`, `token_expired`, `token_consumed`, `subject_linked`, `rate_limited`, `validation_failed`, `server_error`. The first ten of these also appear verbatim in `lib/api/types.ts`'s `ApiErrorCode` union (`not_found` and `subject_linked` are server-only extras per the plan; `network_unavailable` in the client union is client-only and has no server-side mapping).

## Decisions Made

See `key-decisions` in frontmatter. Summary: malformed/undecodable input collapses into an existing sentinel error rather than a new one (enumeration resistance); `testsupport_test.go`'s `mailer` interface is a documented placeholder for plan 01-09's real `Mailer` type; `REQUIREMENTS.md`'s `ACCT-01` checkbox stays unflipped until a wave 4 plan actually wires these primitives to an HTTP route, matching plan 01-03's precedent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. All three tasks passed RED (compile failure due to undefined identifiers) then GREEN (all acceptance criteria + tests passing) on the first implementation attempt.

## User Setup Required

None - no external service configuration required. No new Go dependencies were needed: `golang.org/x/crypto/bcrypt`, `github.com/golang-jwt/jwt/v5`, `golang.org/x/time/rate`, `github.com/gin-gonic/gin`, and `github.com/google/uuid` were all already present in `go.mod` from plan 01-01 (confirmed by reading `go.mod` directly at the start of this plan, consistent with the prior attempt's finding).

## Next Phase Readiness

- Wave 4 handler plans (01-08 through 01-11) can import `internal/auth`, `internal/middleware`, and `internal/httpapi` directly, and add their own `_test.go` files in package `httpapi` to reuse `newTestRouter`/`TestDeps`/`fakeUserRepo`/`fakeRefreshRepo`/`fakeVerificationRepo`/`fakeMailer` without touching `cmd/api/main.go` or a database.
- Plan 01-09 (mailer) should reconcile its concrete `Mailer` interface with `testsupport_test.go`'s local placeholder (`SendVerificationEmail(ctx, toEmail, token string) error`), either matching the signature or updating the fake/interface in that plan.
- `REQUIREMENTS.md`'s `ACCT-01` line is intentionally still unchecked -- the wave 4 plans that wire these primitives to real signup/login/refresh/me routes are what should flip it.
- No blockers.

## Self-Check: PASSED

- FOUND: internal/auth/password.go
- FOUND: internal/auth/password_test.go
- FOUND: internal/auth/jwt.go
- FOUND: internal/auth/jwt_test.go
- FOUND: internal/auth/refresh.go
- FOUND: internal/auth/refresh_test.go
- FOUND: internal/middleware/auth.go
- FOUND: internal/middleware/auth_test.go
- FOUND: internal/middleware/ratelimit.go
- FOUND: internal/middleware/ratelimit_test.go
- FOUND: internal/httpapi/errors.go
- FOUND: internal/httpapi/testsupport_test.go
- FOUND: commit 2b401da
- FOUND: commit 679897d
- FOUND: commit ef5720d
- FOUND: commit f17f26b
- FOUND: commit b27d00a
- FOUND: commit 8edd355
- go test ./internal/auth -run 'TestPassword|TestJWT' -v -count=1 — PASS (10 cases)
- go test ./internal/auth -run TestRefresh -v -count=1 — PASS (7 cases)
- go test ./internal/middleware -v -count=1 — PASS (6 cases)
- go build ./internal/httpapi — PASS
- go build ./... — PASS
- go vet ./internal/... — PASS
- go test ./internal/auth ./internal/middleware -count=1 — PASS (no database required)
- Error-code cross-check (10 codes) against lib/api/types.ts and internal/httpapi/errors.go — PASS

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
