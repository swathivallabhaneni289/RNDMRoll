---
phase: 01-foundation-accounts
plan: 13
subsystem: api
tags: [gin, slog, http-server, graceful-shutdown, integration-testing, go-mod-tidy]

requires:
  - phase: 01-foundation-accounts (plan 06)
    provides: internal/auth security primitives, internal/middleware (RequireAuth, RateLimit), internal/httpapi error mapping and shared test harness
  - phase: 01-foundation-accounts (plan 08)
    provides: AuthHandler (signup/login/refresh/logout)
  - phase: 01-foundation-accounts (plan 09)
    provides: internal/mail (Mailer, Service, Resend/log senders), VerifyEmailHandler, middleware.RequireVerified
  - phase: 01-foundation-accounts (plan 10)
    provides: OAuthHandler, Apple/Google verifiers
  - phase: 01-foundation-accounts (plan 11)
    provides: ProfileHandler, UsernameHandler, internal/storage.AvatarStore
provides:
  - internal/middleware.Recovery and RequestLogger (cross-cutting middleware, T-01-SRV-01/02/04)
  - internal/httpapi.Server / NewServer / Deps / (*Server).Engine (full route assembly)
  - cmd/api/main.go (process entrypoint with graceful shutdown)
  - internal/httpapi/integration_test.go (5 end-to-end tests against real Postgres)
  - Single reconciled internal/mail.Mailer contract used by both AuthHandler and VerifyEmailHandler
affects: [01-15]

tech-stack:
  added: []
  patterns:
    - "NewServer builds the gin.Engine once at construction (Engine() returns the stored value), not per call, so RateLimit's per-route bucket state is stable across a server's lifetime"
    - "A handler whose own Register writes a full route string beginning with a shared path segment (AuthHandler/OAuthHandler both write \"/auth/...\") is mounted on the bare parent group; a handler whose Register writes a bare route string assuming that segment is already applied (VerifyEmailHandler writes \"/verify-email\", not \"/auth/verify-email\") is mounted on a nested sub-group for that segment instead -- both produce the same final path with no collision"
    - "RequestLogger never logs a header or request body; the one place a credential-shaped value could still reach a log line (a query parameter, e.g. the verify-email callback's ?token=) is closed by a single redactSensitiveQuery helper, not by trusting every future log call site to remember the rule"

key-files:
  created:
    - internal/middleware/recovery.go
    - internal/middleware/recovery_test.go
    - internal/middleware/logging.go
    - internal/middleware/logging_test.go
    - internal/httpapi/server.go
    - internal/httpapi/server_test.go
    - internal/httpapi/integration_test.go
    - cmd/api/main.go
  modified:
    - internal/httpapi/auth.go
    - internal/httpapi/auth_test.go
    - internal/httpapi/testsupport_test.go
    - go.mod
    - go.sum
    - Makefile

key-decisions:
  - "Mailer interface reconciliation (the tension both 01-08 and 01-09 flagged for this plan): AuthHandler now depends on internal/mail.Mailer (SendVerificationEmail(ctx, *user.User) error) directly, the same interface VerifyEmailHandler already used, instead of its own placeholder-shaped Mailer (toEmail, token string) with its own EmailVerificationRepository-based token-generation path. AuthHandler lost its verifications field and its sendVerificationEmail/generateVerificationToken methods entirely. testsupport_test.go's placeholder mailer interface (the thing 01-06's own SUMMARY flagged as needing reconciliation) is retired; TestDeps.Mailer and fakeMailer are now typed against the real mail.Mailer. Net effect: exactly one code path issues, hashes, persists, and supersedes email verification tokens -- internal/mail.Service, constructed once in cmd/api/main.go and handed to both handlers. This was done as its own commit (3ea9a6e) before Task 1, so every subsequent build/test in this plan ran against the final interface shape. Modifies files outside this plan's declared files_modified (auth.go, auth_test.go, testsupport_test.go, owned by plans 01-06/01-08) -- called out here as the deliberate exception the orchestrator's own instructions for this plan required."
  - "A second, smaller wiring detail this plan had to resolve on its own (not explicitly flagged by any prior SUMMARY, discovered by writing server.go): AuthHandler and OAuthHandler write full route strings starting with \"/auth/...\" (expecting to be mounted on the bare /v1 group), but VerifyEmailHandler writes bare route strings like \"/verify-email\" (expecting to already be under a \"/auth\" prefix, per its own SUMMARY's wiring-contract note). NewServer resolves this by mounting VerifyEmailHandler on a nested v1.Group(\"/auth\") while mounting Auth/OAuth directly on v1 -- both produce the documented final paths (/v1/auth/signup, /v1/auth/verify-email, ...) with no path or symbol collision. Proven both by server_test.go's TestServer_VerifyEmailIsMountedUnderAuthPrefix and by the real binary's GIN-debug route dump during the Task 2 boot smoke test."
  - "RequestLogger never logs a request header or request body under any circumstance -- not even a redacted form -- so Authorization/password values can never reach a log line through it regardless of route. The only per-request value it derives a logged field from besides method/path/status/duration/client_ip is the query string, and only through redactSensitiveQuery, which is what actually protects the verify-email callback's ?token=... case."
  - "Found via advisor review after the plan's own tasks were otherwise complete, not by a failing test: internal/httpapi's new integration_test.go and internal/store/postgres's existing postgres_test.go both truncate the same `users` table against the same TEST_DATABASE_URL. go test's default package-level parallelism (-p, defaulting to GOMAXPROCS) can run those two packages' test binaries concurrently, so one package's t.Cleanup truncate can wipe rows a concurrently-running test in the other package still needs -- a real latent flake in `TEST_DATABASE_URL=... go test ./...`, the exact command 01-VALIDATION.md documents as the full-suite command. Fixed two ways: (1) requireIntegrationPool now truncates on entry as well as on cleanup, guarding against residue from a crashed prior run; (2) added a `make test-all-integration` target (`go test ./... -p 1 -count=1`) that serializes package execution, and documented in both the Makefile and requireIntegrationPool's own comment that this -- not a bare `TEST_DATABASE_URL=... go test ./...` -- is the safe way to run the full suite against a real database. Plain `go test ./...` (no TEST_DATABASE_URL) is unaffected either way, since every DB-touching test in both packages skips."
  - "cmd/api/main.go fails fast (os.Exit(1) after a logged error) if auth.NewAppleVerifier's JWKS fetch fails at construction, matching NewPool/MustLoad's fail-fast precedent (T-01-SRV-05). Verified empirically before writing this: NewAppleVerifier(...) against the real https://appleid.apple.com/auth/keys endpoint (not gated by APPLE_BUNDLE_ID being a real value -- the JWKS is public) resolved in ~780ms with no error in this environment, using a throwaway internal/auth test file deleted before any commit. If this plan's environment ever loses network egress to Apple, this becomes a boot blocker; that tradeoff is accepted here as consistent with the rest of the threat model's fail-fast philosophy, not re-litigated."
  - "The Task 2 boot smoke test and TestUnverifiedCannotReachProfile both needed real credential-shaped values (JWT secret, DATABASE_URL, S3/OAuth placeholders). Per this worktree's instruction never to Read/Write/cat any .env or .env.example file, every credential was supplied as an inline literal environment variable on the command line (e.g. DATABASE_URL=postgres://localhost:5432/rndmroll_dev?sslmode=disable JWT_SECRET=... go run ./cmd/api), never by sourcing .env. The Makefile's new `dev` target does reference `./.env` in its recipe text (`set -a && . ./.env && set +a && go run ./cmd/api`) -- that text is never read or executed by this session; it only runs when a human later invokes `make dev`."
  - "REQUIREMENTS.md is left unmodified, matching the precedent 01-08/01-09/01-11 already set for this shared file: ACCT-01 is already checked (by 01-08); ACCT-03 is intentionally still unchecked because its UI half (profile view/edit screens) is plan 01-14, a sibling wave-5 worktree not yet merged as of this plan's execution. This plan proves ACCT-03's full backend surface end-to-end (TestFullOnboardingFlow, TestCrossAccountIsolation) but does not claim the user-facing requirement complete on its own."

patterns-established:
  - "A panicIfTouchedUserRepo{ user.Repository } pattern (embed a nil interface, override nothing) proves a code path never calls a given dependency, rather than merely asserting a status code that could pass for the wrong reason -- used to prove GET /healthz makes no database call."
  - "Integration tests that build the real Server against a real database live in the same package as the unit-level handler tests (internal/httpapi), reusing that package's existing doJSONRequest/decodeBody helpers directly rather than redeclaring them, with every new integration-only symbol prefixed integration*/Integration* to avoid the collisions this package has already hit twice across the wave-4 merges (367c42a, a541912)."

requirements-completed: [ACCT-01, ACCT-03]

coverage:
  - id: D1
    description: "Recovery middleware: a panicking handler returns 500 with a fixed {\"error\":\"server_error\"} body containing no stack trace or panic message, the process keeps serving afterward, and the panic value/stack are logged server-side only"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/middleware/recovery_test.go#TestRecovery_PanicReturns500WithGenericBody"
        status: pass
      - kind: unit
        ref: "internal/middleware/recovery_test.go#TestRecovery_PanicBodyContainsNoStackTraceOrPanicMessage"
        status: pass
      - kind: unit
        ref: "internal/middleware/recovery_test.go#TestRecovery_ProcessKeepsServingAfterPanic"
        status: pass
      - kind: unit
        ref: "internal/middleware/recovery_test.go#TestRecovery_LogsPanicValueAndStackServerSide"
        status: pass
      - kind: unit
        ref: "internal/middleware/recovery_test.go#TestRecovery_NoPanicPassesThroughUnaffected"
        status: pass
    human_judgment: false
  - id: D2
    description: "RequestLogger writes one structured line per request (method/path/status/duration/client_ip) and never logs an Authorization header value, a password/token request-body field, or a sensitive query parameter value"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/middleware/logging_test.go#TestRequestLogger_LogsMethodPathStatusAndDuration"
        status: pass
      - kind: unit
        ref: "internal/middleware/logging_test.go#TestRequestLogger_NeverLogsAuthorizationHeaderOrPasswordFieldValue"
        status: pass
      - kind: unit
        ref: "internal/middleware/logging_test.go#TestRequestLogger_RedactsSensitiveQueryParamValues"
        status: pass
      - kind: unit
        ref: "internal/middleware/logging_test.go#TestRedactSensitiveQuery_KeepsNonSensitiveParamsAndRedactsSensitiveOnes"
        status: pass
      - kind: unit
        ref: "internal/middleware/logging_test.go#TestRequestLogger_LogsClientIP"
        status: pass
    human_judgment: false
  - id: D3
    description: "Server/NewServer assembles every Phase 1 route in one place: GET /healthz answers without touching any dependency, every route (including the /v1/auth nesting VerifyEmailHandler needs) is reachable at its documented path, and RequireAuth+RequireVerified are applied once at the authenticated group boundary so an unverified account is rejected and a verified one succeeds"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_HealthzReturns200WithoutTouchingAnyDependency"
        status: pass
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_SignupIsReachableWithoutAuthentication"
        status: pass
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_VerifyEmailIsMountedUnderAuthPrefix"
        status: pass
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_MeRequiresAuthentication"
        status: pass
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_MeRejectsUnverifiedAccountWith403"
        status: pass
      - kind: unit
        ref: "internal/httpapi/server_test.go#TestServer_MeAllowsVerifiedAccount"
        status: pass
    human_judgment: false
  - id: D4
    description: "cmd/api/main.go: one command starts a server that serves every Phase 1 route against real Postgres, refuses to start on bad configuration, sets explicit http.Server timeouts, and shuts down gracefully on SIGINT/SIGTERM within a bounded context before closing the pool"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... clean; built binary run against real local Postgres (rndmroll_dev, migrated) with inline placeholder env vars (not .env): GIN-debug route dump matched every documented path, GET /healthz returned 200, and SIGTERM produced structured log lines \"shutdown signal received\" then \"server stopped\" before the process exited"
        status: pass
    human_judgment: false
  - id: D5
    description: "Mailer interface reconciliation: AuthHandler and VerifyEmailHandler share exactly one internal/mail.Mailer-shaped dependency and exactly one token-issuing code path (internal/mail.Service); no placeholder or duplicate token-generation path remains anywhere in internal/httpapi"
    verification:
      - kind: unit
        ref: "internal/httpapi/auth_test.go (full package, run against real Postgres) -- all signup/login/refresh/logout cases pass against the reconciled AuthHandler"
        status: pass
      - kind: other
        ref: "grep confirms no remaining reference to a local httpapi.Mailer type or testsupport_test.go's retired placeholder mailer interface; go build ./... and go vet ./... clean"
        status: pass
    human_judgment: false
  - id: D6
    description: "End-to-end integration suite against real Postgres proves both ACCT-01 and ACCT-03 over HTTP: full onboarding (signup through onboarding_complete), session survives a simulated relaunch via refresh-token-only, cross-account isolation under an injected foreign id, an unverified account is rejected at the route boundary, and a redeemed refresh token cannot be replayed"
    requirement: ACCT-03
    verification:
      - kind: integration
        ref: "internal/httpapi/integration_test.go#TestFullOnboardingFlow"
        status: pass
      - kind: integration
        ref: "internal/httpapi/integration_test.go#TestSessionSurvivesRelaunch"
        status: pass
      - kind: integration
        ref: "internal/httpapi/integration_test.go#TestCrossAccountIsolation"
        status: pass
      - kind: integration
        ref: "internal/httpapi/integration_test.go#TestUnverifiedCannotReachProfile"
        status: pass
      - kind: integration
        ref: "internal/httpapi/integration_test.go#TestRefreshTokenSingleUse"
        status: pass
      - kind: other
        ref: "TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -v -count=1 -- 49/49 pass, zero regressions in wave 4's own tests; TEST_DATABASE_URL unset -- all 5 integration tests skip cleanly with an explanatory message; make test-all-integration (go test ./... -p 1 -count=1, serialized against the cross-package truncate race -- see key-decisions) -- 8/8 packages pass"
        status: pass
    human_judgment: true
    rationale: "The suite proves the backend contract for ACCT-03 completely, but ACCT-03 as a user-facing requirement (\"User can view their own profile\") also needs its UI half -- plan 01-14's profile view/edit screens, a sibling wave-5 worktree not yet merged as of this plan's execution. A human (or a later phase-completion pass, once 01-14 lands) should confirm the UI half before REQUIREMENTS.md's ACCT-03 checkbox is flipped."

duration: 11min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 13: Server Assembly, Process Entrypoint, and End-to-End Verification Summary

**Recovery/logging middleware, full Phase 1 route assembly (including resolving a route-nesting mismatch between AuthHandler/OAuthHandler and VerifyEmailHandler), a graceful-shutdown process entrypoint proven by booting the real binary against real Postgres, and a 5-test end-to-end suite proving ACCT-01 and ACCT-03 over HTTP -- plus reconciling the Mailer interface tension plans 01-08 and 01-09 each independently flagged for this plan.**

## Performance

- **Duration:** 11 min (commit-to-commit span across five commits; the upfront context-loading/read phase, including reading both 01-08 and 01-09's SUMMARY key-decisions in full and empirically testing NewAppleVerifier's network behavior before writing main.go, is not separately timestamped)
- **Started:** 2026-09-18T19:41:07+05:30 (first commit)
- **Completed:** 2026-09-18T19:52:11+05:30 (last commit)
- **Tasks:** 3 plan tasks, plus one prerequisite reconciliation commit before Task 1 (required by this plan's own instructions, touching files outside its declared scope)
- **Files modified:** 14 (8 created, 6 modified)

## Accomplishments

- **Mailer interface reconciliation** (the explicit prerequisite this plan's instructions called out): `AuthHandler` now depends on the real `internal/mail.Mailer` directly and no longer generates or persists its own verification token -- `internal/mail.Service` is the single code path that issues, hashes, persists, and supersedes verification tokens for both signup and resend. `testsupport_test.go`'s long-flagged placeholder `mailer` interface (something 01-06's own SUMMARY asked to be reconciled) is retired.
- `internal/middleware`: `Recovery` (fixed opaque 500 body, no stack trace or panic message ever reaches a caller, panic logged server-side) and `RequestLogger` (structured method/path/status/duration/client_ip line, never logs a header or request body, and closes the one remaining leak path -- a sensitive query parameter like the verify-email callback's `?token=` -- with an explicit `redactSensitiveQuery` helper)
- `internal/httpapi/server.go`: `Server`/`NewServer` assembles every Phase 1 route once, at construction. Resolved a second wiring detail beyond the Mailer question (not previously flagged by any SUMMARY): `AuthHandler`/`OAuthHandler` write full `/auth/...` route strings expecting the bare `/v1` group, while `VerifyEmailHandler` writes bare route strings expecting a `/v1/auth` group already applied -- `NewServer` mounts each accordingly, producing the documented final paths with no collision. `RequireAuth`+`RequireVerified` are applied once at the authenticated group boundary.
- `cmd/api/main.go`: process entrypoint wiring `config.MustLoad`, an slog JSON logger, `postgres.NewPool`, the three pgx repositories, exactly one `mail.Service` instance (selecting Resend or log sender by `MailDriver`) handed to both `AuthHandler` and `VerifyEmailHandler`, both OAuth verifiers, the avatar store, all five handlers, and `httpapi.NewServer`, wrapped in an `http.Server` with explicit read/write timeouts and graceful SIGINT/SIGTERM shutdown. Verified by actually booting the built binary against real local Postgres and confirming `/healthz` plus a clean shutdown log sequence -- not just a `go build` pass.
- `go mod tidy`: promoted 12 directly-imported modules out of the `// indirect` block and removed 6 modules plan 01-01 pinned but that turned out unused (`internal/storage/s3.go` authenticates with static credentials directly, never touching the AWS SDK's default-config/SSO/STS credential chain).
- `internal/httpapi/integration_test.go`: 5 end-to-end tests against real Postgres (`TestFullOnboardingFlow`, `TestSessionSurvivesRelaunch`, `TestCrossAccountIsolation`, `TestUnverifiedCannotReachProfile`, `TestRefreshTokenSingleUse`), skipping cleanly when `TEST_DATABASE_URL` is unset. All 49 tests in `internal/httpapi` pass together against `rndmroll_test` with zero regressions.

## Task Commits

Each task was committed atomically; Task 1 followed RED then GREEN (TDD):

0. **Mailer reconciliation (prerequisite to Task 1)** - `3ea9a6e` (fix)
1. **Task 1 (RED): failing tests for recovery/logging middleware and route assembly** - `17cee52` (test)
1. **Task 1 (GREEN): recovery/logging middleware and full route assembly** - `88b17b0` (feat)
2. **Task 2: process entrypoint with graceful shutdown** - `aa79298` (feat)
3. **Task 3: end-to-end integration suite against real Postgres** - `c89bcba` (test)

**Plan metadata:** this commit (`docs(01-13)`)

## Files Created/Modified

- `internal/httpapi/auth.go` - `AuthHandler` now depends on `mail.Mailer`; lost its own token-generation path and `EmailVerificationRepository` dependency
- `internal/httpapi/auth_test.go` - `NewAuthHandler` call sites updated to the new 5-arg signature
- `internal/httpapi/testsupport_test.go` - `TestDeps.Mailer`/`fakeMailer` retyped against the real `mail.Mailer`; placeholder `mailer` interface removed
- `internal/middleware/recovery.go` / `recovery_test.go` - panic recovery, opaque 500, server-side logging
- `internal/middleware/logging.go` / `logging_test.go` - structured request logging, `redactSensitiveQuery`
- `internal/httpapi/server.go` / `server_test.go` - `Server`, `NewServer`, `Deps`, `(*Server).Engine`, full route tree
- `cmd/api/main.go` - process entrypoint
- `internal/httpapi/integration_test.go` - 5 end-to-end tests, `TestMain`, migration bootstrap, helpers
- `go.mod` / `go.sum` - `go mod tidy` reconciliation (see Decisions)
- `Makefile` - `dev` and `test-integration` targets

## Decisions Made

See `key-decisions` in frontmatter for full detail. Summary: (1) Mailer reconciliation resolved by making `AuthHandler` depend on the real `mail.Mailer` and deleting its duplicate token-issuing path -- done as a standalone commit before Task 1, touching files outside this plan's declared scope by explicit instruction; (2) a second wiring mismatch (VerifyEmailHandler's route-string assumptions vs. AuthHandler/OAuthHandler's) resolved by nesting VerifyEmailHandler under `v1.Group("/auth")`; (3) `RequestLogger` never logs headers or bodies at all, closing the query-string leak path specifically; (4) `cmd/api/main.go` fails fast on `NewAppleVerifier`'s JWKS fetch, verified empirically to resolve in under a second in this environment before committing to that choice; (5) every credential used in this plan's own verification (boot smoke test, integration test JWT secret) was supplied as an inline env var, never by reading or sourcing `.env`; (6) `REQUIREMENTS.md` left unmodified, matching wave 4 precedent -- ACCT-01 already checked, ACCT-03 intentionally not (its UI half is sibling plan 01-14, not yet merged).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Mailer interface reconciliation required editing files outside this plan's declared `files_modified`**
- **Found during:** Pre-Task-1 orientation, reading 01-08 and 01-09's SUMMARY key-decisions in full per this plan's own explicit instructions
- **Issue:** `AuthHandler` (01-08) depended on a local `Mailer` interface (`toEmail, token string`) with its own `EmailVerificationRepository`-based token generation; the real `internal/mail.Mailer` (01-09) is `SendVerificationEmail(ctx, *user.User) error`, where `mail.Service` owns token issuance itself. These are structurally incompatible -- `cmd/api/main.go` cannot construct one `mail.Service` and hand it to both handlers without reconciling this first. Both 01-08 and 01-09's SUMMARY files explicitly flagged this plan as the reconciliation point.
- **Fix:** `AuthHandler` now depends on `mail.Mailer` directly, dropped its own `verifications` field and token-generation methods; `testsupport_test.go`'s placeholder `mailer` interface (itself flagged for reconciliation by 01-06's SUMMARY) is retired, with `TestDeps.Mailer`/`fakeMailer` retyped against the real interface.
- **Files modified:** `internal/httpapi/auth.go`, `internal/httpapi/auth_test.go`, `internal/httpapi/testsupport_test.go` (all outside this plan's declared `files_modified`, owned by plans 01-06/01-08 -- modified here because this plan's own instructions explicitly required this exact reconciliation)
- **Verification:** `go build ./...` and `go vet ./...` clean; full `go test ./... -count=1` passes (including every existing `auth_test.go` case, unchanged in behavior); later proven again end-to-end by `TestSignup_SetsEmailUnverifiedAndSendsVerificationEmailOnce` and the new integration suite
- **Committed in:** `3ea9a6e` (standalone commit before Task 1)

**2. [Rule 2 - Missing Critical] VerifyEmailHandler's route-nesting assumption vs. AuthHandler/OAuthHandler's, discovered while writing server.go**
- **Found during:** Task 1, writing `server.go`'s route assembly
- **Issue:** `AuthHandler.Register` and `OAuthHandler.Register` write full route strings beginning `/auth/...`, designed for the bare `/v1` group. `VerifyEmailHandler.Register` writes bare route strings (`/verify-email`, not `/auth/verify-email`), designed for a `/v1/auth` group already applied by the caller -- per that plan's own SUMMARY wiring-contract note, which this plan's file list did not surface as a conflict since no shared file was touched by both plans.
- **Fix:** `NewServer` mounts `AuthHandler`/`OAuthHandler` directly on `v1` and `VerifyEmailHandler` on a nested `v1.Group("/auth")`, producing identical final paths (`/v1/auth/verify-email`, etc.) with no collision.
- **Files modified:** `internal/httpapi/server.go`
- **Verification:** `server_test.go#TestServer_VerifyEmailIsMountedUnderAuthPrefix` (route reachable, not 404); independently reconfirmed by the real binary's `GIN-debug` route dump during the Task 2 boot smoke test, which lists every route at its documented path
- **Committed in:** `88b17b0` (Task 1 GREEN commit)

**3. [Rule 2 - Missing Critical] Cross-package test-database race between internal/httpapi and internal/store/postgres**
- **Found during:** Post-implementation advisor review, not by a failing test -- the race is timing-dependent and two manual reruns of `TEST_DATABASE_URL=... go test ./... -count=1` both happened to pass
- **Issue:** `internal/httpapi/integration_test.go` (this plan) and `internal/store/postgres/postgres_test.go` (plan 01-03, unmodified) both run `truncate users cascade` -- one in `t.Cleanup`, the other likewise -- against the same `TEST_DATABASE_URL`. `go test ./...` runs each package's test binary as a separate process, with up to `GOMAXPROCS` running concurrently by default (`-p`). When `TEST_DATABASE_URL` is exported for the whole `go test ./...` invocation (01-VALIDATION.md's documented full-suite command, once a test database is configured), these two packages' test binaries can run at the same time, and a truncate from one package's cleanup can wipe rows a concurrently-running test in the other package still needs.
- **Fix:** `requireIntegrationPool` now truncates on entry in addition to on cleanup (guards against residue from a crashed prior run). Added a `make test-all-integration` target (`go test ./... -p 1 -count=1`) that serializes package execution, with the safety reasoning documented both in the Makefile and in `requireIntegrationPool`'s own comment. Plain `go test ./...` with no `TEST_DATABASE_URL` (this plan's own Task 3 verify command, and CI without a provisioned test database) is unaffected either way, since every DB-touching test in both packages skips cleanly.
- **Files modified:** `internal/httpapi/integration_test.go`, `Makefile`
- **Verification:** `make test-all-integration` -- 8/8 packages pass, serialized; `make test-integration` (package-scoped, always safe) -- unaffected, still passes
- **Committed in:** (this SUMMARY's own commit, since it was found during final review rather than during a task)

---

**Total deviations:** 3 auto-fixed (1 blocking, explicitly anticipated and required by this plan's own instructions; 2 missing-critical, one discovered during implementation and one during final review)
**Impact on plan:** All three were necessary -- for `cmd/api/main.go` to be constructible at all with a single coherent `Mailer`/`mail.Service` contract, for the server to route correctly, and for the documented full-suite command to be reliable rather than flaky. No scope creep beyond what the plan's own text and explicit prerequisite instructions required.

## Issues Encountered

None beyond the two deviations above, both resolved within the task (or prerequisite step) they were found in.

## User Setup Required

None - no external service configuration required. Real Resend/Google/Apple/S3 credentials remain deferred placeholders per plan 01-01's checkpoint; `MAIL_DRIVER=log` is what this environment actually exercises, and the integration suite and boot smoke test were both designed and verified against that configuration plus real local Postgres, never against real external connectivity.

One caveat worth carrying forward: `cmd/api/main.go` fails fast if `auth.NewAppleVerifier`'s JWKS fetch to `https://appleid.apple.com/auth/keys` fails at boot. This resolved in ~780ms with no error in this environment (network egress confirmed available), so it does not block this plan's own boot smoke test, but a future fully network-isolated deployment environment would need this reconsidered (retry/backoff, or a degraded-Apple-signin mode) -- out of scope here, flagged for whoever owns deployment (plan 01-15's walkthrough or later).

## 01-VALIDATION.md Per-Task Verification Map: Closure

`01-VALIDATION.md`'s per-task verification map (written before task IDs were assigned) names tests by a provisional convention that doesn't literally match what wave 4 and this plan actually shipped (e.g. `TestEmailVerificationGate`, `TestGetProfile`, `TestUpdateProfile` were never the real test names). Every row's behavior is genuinely covered; this table is the mapping the map itself deferred to "the planner aligns task IDs to these rows" -- recorded here since Task 3's acceptance criteria explicitly required this closure and nobody reading only the VALIDATION doc could otherwise see it:

| VALIDATION row (requirement, behavior) | Actual covering test(s) |
|---|---|
| ACCT-01: Signup creates a user with `email_verified=false` and a hashed password | `internal/httpapi/auth_test.go#TestSignup_ValidBody_Returns201AndHashesPassword`, `#TestSignup_SetsEmailUnverifiedAndSendsVerificationEmailOnce` (plan 01-08) |
| ACCT-01: Login with correct credentials returns tokens; wrong password rejected generically | `internal/httpapi/auth_test.go#TestLogin_CorrectCredentials_Returns200WithTokensAndUser`, `#TestLogin_WrongPassword_Returns401InvalidCredentials`, `#TestLogin_UnknownAccountByteIdenticalToWrongPassword` (plan 01-08) |
| ACCT-01: Refresh issues a new access token for a valid refresh token; rejects expired/revoked | `internal/httpapi/auth_test.go#TestRefresh_ValidToken_Returns200WithNewTokenPairDifferentFromPresented`, `#TestRefresh_ExpiredToken_Returns401TokenExpired` (plan 01-08); `internal/httpapi/integration_test.go#TestRefreshTokenSingleUse`, `#TestSessionSurvivesRelaunch` (this plan, end-to-end) |
| ACCT-01: Unverified user cannot access `(app)`-gated endpoints (D-04) | `internal/middleware/verified_test.go#TestRequireVerified_UnverifiedAccountReturns403AndHandlerNeverRuns` (plan 01-09, unit); `internal/httpapi/integration_test.go#TestUnverifiedCannotReachProfile` (this plan, end-to-end over real HTTP against real Postgres) |
| ACCT-03: `GET /me` returns the caller's own profile only, derived from the token subject | `internal/httpapi/profile_test.go#TestProfile_GetMe_ReturnsCallerProfileWithOnboardingComplete` (plan 01-11, unit); `internal/httpapi/integration_test.go#TestFullOnboardingFlow`, `#TestCrossAccountIsolation` (this plan, end-to-end) |
| ACCT-03: `PATCH /me` updates name/username/bio, rejects a taken username | `internal/httpapi/profile_test.go#TestProfile_PatchMe_UpdatesFieldsIndependently`, `#TestProfile_PatchMe_UsernameTakenReturns409WithAlternates` (plan 01-11, unit); `internal/httpapi/integration_test.go#TestFullOnboardingFlow`, `#TestCrossAccountIsolation` (this plan, end-to-end) |

## Next Phase Readiness

- The backend half of Phase 1 is complete and runnable with one command (`go run ./cmd/api` or `make dev`), proven by booting the real binary against real Postgres and by 49/49 tests passing in `internal/httpapi` against `rndmroll_test`.
- Server start command for plan 01-15's walkthrough: `go run ./cmd/api` (or the built binary), with `DATABASE_URL`, `JWT_SECRET` (>=32 bytes), `APP_BASE_URL`, `MAIL_DRIVER` (`log` or `resend`), `MAIL_FROM_ADDRESS`, `GOOGLE_CLIENT_ID_IOS`/`_ANDROID`/`_WEB`, `APPLE_BUNDLE_ID`, and the five `S3_*` variables all set (see `internal/config/config.go` for the full list and defaults) -- `make dev` loads these from a local `.env` this session never read.
- `internal/httpapi/integration_test.go`'s five tests are the closest automated proxy for both ACCT-01 and ACCT-03; `TestUnverifiedCannotReachProfile` and `TestCrossAccountIsolation`'s patterns (mint a token directly with `auth.IssueAccessToken` when no HTTP path to one exists; inject a foreign id alongside a real mutating field so the isolation assertion isn't vacuous) are reusable for any future route added to the authenticated group.
- `REQUIREMENTS.md`'s `ACCT-03` line is intentionally still unchecked -- plan 01-14 (profile view/edit UI), a sibling wave-5 worktree, needs to land before that requirement is genuinely complete end-to-end for a real user, even though this plan proves its full backend contract.
- No blockers for plan 01-15's walkthrough.

## Self-Check: PASSED

- FOUND: internal/middleware/recovery.go
- FOUND: internal/middleware/recovery_test.go
- FOUND: internal/middleware/logging.go
- FOUND: internal/middleware/logging_test.go
- FOUND: internal/httpapi/server.go
- FOUND: internal/httpapi/server_test.go
- FOUND: cmd/api/main.go
- FOUND: internal/httpapi/integration_test.go
- FOUND: commit 3ea9a6e
- FOUND: commit 17cee52
- FOUND: commit 88b17b0
- FOUND: commit aa79298
- FOUND: commit c89bcba
- go test ./internal/middleware -run 'TestRecovery|TestRequestLogger' -v -count=1 -- PASS (10 cases)
- go build ./... -- PASS
- go vet ./... -- PASS
- go mod tidy && go build ./... -- PASS (module state consistent after tidy)
- Built binary boots against real local Postgres, curl /healthz -- 200; SIGTERM -- clean graceful shutdown log sequence
- TEST_DATABASE_URL unset: go test ./internal/httpapi -- all 5 integration tests skip cleanly
- TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -v -count=1 -- PASS (49/49 cases, zero regressions)
- TEST_DATABASE_URL=... go test ./... -count=1 -- PASS (all 8 packages)
- grep checks for gin.New(), RequireVerified, healthz, server_error, no c.Request.Header in logging.go -- all PASS
- grep checks for func TestFullOnboardingFlow/TestSessionSurvivesRelaunch/TestCrossAccountIsolation/TestUnverifiedCannotReachProfile/TestRefreshTokenSingleUse -- all PASS

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
