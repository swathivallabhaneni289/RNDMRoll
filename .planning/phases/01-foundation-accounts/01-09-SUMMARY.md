---
phase: 01-foundation-accounts
plan: 09
subsystem: auth
tags: [email-verification, mail, resend, gin, middleware, tdd]

requires:
  - phase: 01-foundation-accounts (plan 03)
    provides: internal/user domain model, sentinel errors, and the EmailVerificationRepository/Repository interfaces this plan codes against
  - phase: 01-foundation-accounts (plan 06)
    provides: internal/auth (IssueAccessToken, RefreshService), internal/middleware (RequireAuth, SubjectFromContext, RateLimit/KeyByIPAndField), internal/httpapi (Respond/RespondError/RespondValidationError, newTestRouter/fakeUserRepo/fakeRefreshRepo/fakeVerificationRepo)
provides:
  - internal/mail package: Mailer, Sender, Service (NewService), NewResendSender, NewLogSender
  - internal/httpapi/verify_email.go: VerifyEmailHandler (NewVerifyEmailHandler, Register, Verify, Resend, Callback)
  - internal/middleware/verified.go: RequireVerified, UserFromContext
  - Routes: POST /v1/auth/verify-email, POST /v1/auth/verify-email/resend, GET /v1/auth/verify-email/callback
affects: [01-13]

tech-stack:
  added: []
  patterns:
    - "internal/mail.Sender is the narrow provider-swap seam; internal/mail.Service owns token issuance/hashing/message composition and is the only thing that implements the wider Mailer interface httpapi codes against"
    - "resendSender keeps its Resend API endpoint as an unexported struct field (defaulted to the real https://api.resend.com/emails constant by NewResendSender) so a same-package test can point it at an httptest.Server with zero network access, while NewResendSender's public signature stays exactly as planned"
    - "Token verification decodes+hashes a client-presented token identically to how internal/mail.Service hashed it at issuance (base64.RawURLEncoding decode, then sha256.Sum256 of the raw bytes) -- the same pattern internal/auth's refresh-token redemption already established"

key-files:
  created:
    - internal/mail/mailer.go
    - internal/mail/mailer_test.go
    - internal/mail/resend.go
    - internal/mail/log_mailer.go
    - internal/httpapi/verify_email.go
    - internal/httpapi/verify_email_test.go
    - internal/middleware/verified.go
    - internal/middleware/verified_test.go
  modified: []

key-decisions:
  - "internal/mail.Mailer is SendVerificationEmail(ctx, *user.User) error, exactly as the plan specifies, and testsupport_test.go's placeholder `mailer` interface (SendVerificationEmail(ctx, toEmail, token string) error) was deliberately left unmodified. The placeholder cannot express Service's real contract -- Service owns token generation, hashing, and DeleteForUser/Insert against the user's ID, so it structurally needs the *user.User, not a pre-made token. Reshaping the shared placeholder would also risk a merge conflict with three sibling wave-4 worktrees (01-08/01-10/01-11) touching internal/httpapi concurrently. internal/httpapi/verify_email_test.go instead declares its own local fakeVerifyMailer implementing the real mail.Mailer signature, following the established pattern of interface fakes living beside their consumer. Whichever plan wires TestDeps.Mailer to the real internal/mail.Service (likely 01-13) should reconcile testsupport_test.go's placeholder at that point."
  - "Resend's handler binds via c.ShouldBindBodyWith(&req, binding.JSON) rather than c.ShouldBindJSON. The rate-limit middleware ahead of it (KeyByIPAndField) already reads the body through ShouldBindBodyWith to compute its key, which caches the raw bytes under gin's body-cache key; ShouldBindJSON reads net/http's Request.Body directly and would see an already-drained stream (EOF) on that route. Caught by an advisor review before writing the resend test, not discovered by a failing test after the fact."
  - "Added a 7th verify_email test (TestVerifyEmail_IntegratesWithMailServiceIssuedToken) beyond the plan's 6 named behaviors: it runs a token through the real internal/mail.Service (not the hand-rolled issueTestVerificationToken helper) end to end through Verify. issueTestVerificationToken intentionally mirrors Service's own hashing, so on its own it cannot catch a real encode/hash mismatch between Service and Verify; this test closes that gap."
  - "Package-level symbols in internal/httpapi (shared package, three sibling worktrees writing into it concurrently) were deliberately prefixed to avoid collisions: verifyEmailRequest, verifyEmailResendRequest, hashVerificationToken, issueTestVerificationToken, fakeVerifyMailer, postVerifyEmailJSON, newVerifyEmailTestHandler, newVerifyEmailTestRouter. No shared top-level response-building helper was added; the Verify handler builds its gin.H user object inline to avoid declaring a package-level function another wave-4 handler plan might also want to name (e.g. an apiUser/userResponse helper)."
  - "REQUIREMENTS.md left unmodified, matching plans 01-03/01-06's precedent: this plan delivers the verification slice of ACCT-01 (create account, log in, stay logged in), not the full requirement -- 01-08 (signup/login) is a sibling wave-4 worktree not yet merged. Flipping the ACCT-01 checkbox here would be premature; it is also a shared root planning file this worktree's instructions direct not to touch."

patterns-established:
  - "A provider adapter that must hit a fixed, documented external endpoint (Resend) keeps that endpoint as an unexported, constructor-defaulted struct field rather than hardcoding the URL inline in the request call, so same-package tests can redirect it to an httptest.Server without changing the public constructor signature or touching the real network."

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "internal/mail: Mailer/Sender/Service issue a single-use verification token (32 bytes crypto/rand, base64url-encoded), store only its SHA-256 digest, supersede any outstanding token on every issue via DeleteForUser, and hand a composed subject/HTML/text message to a Sender; NewResendSender posts to https://api.resend.com/emails with a bearer key and a 10s-timeout client, surfacing non-2xx as an error carrying only the status code; NewLogSender writes the recipient and full message (including the link) to an io.Writer with no network call"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestService_SendVerificationEmail_StoresHashedTokenMatchingLinkPlaintext"
        status: pass
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestService_SendVerificationEmail_SecondIssueDeletesFirst"
        status: pass
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestLogSender_WritesRecipientAndLinkWithNoNetworkCall"
        status: pass
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestResendSender_PostsWithBearerAndSurfacesStatusWithoutBody"
        status: pass
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestService_SendVerificationEmail_LinkUsesBaseURLAndCallbackPath"
        status: pass
      - kind: unit
        ref: "internal/mail/mailer_test.go#TestService_SendVerificationEmail_MessageCopyMatchesConstraints"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/httpapi.VerifyEmailHandler: Verify consumes a token exactly once and mints the account's first access+refresh session on success (409/401 for consumed/expired/invalid tokens); Resend always returns a byte-identical 202 regardless of whether the address is registered, unregistered, or already verified, rate limited to 1/30s by IP+email; Callback HTML-escapes the token before interpolating it into a redirect page and performs no verification itself"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmail_FreshTokenReturns200WithSession"
        status: pass
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmail_SameTokenTwiceReturns409TokenConsumed"
        status: pass
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmail_ExpiredTokenReturns401TokenExpired"
        status: pass
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmail_NeverIssuedTokenReturns401TokenInvalid"
        status: pass
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmailResend_RegisteredAndUnregisteredByteIdentical"
        status: pass
      - kind: unit
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmailCallback_EscapesTokenAndRedirectsToDeepLink"
        status: pass
      - kind: integration
        ref: "internal/httpapi/verify_email_test.go#TestVerifyEmail_IntegratesWithMailServiceIssuedToken"
        status: pass
    human_judgment: false
  - id: D3
    description: "internal/middleware.RequireVerified gates verified-only routes: a verified account reaches the handler, an unverified account is rejected with 403 email_not_verified, a subject that resolves to no account is rejected with 401 token_invalid, and the handler never runs on either rejection path; UserFromContext exposes the loaded *user.User to downstream handlers and errors when the middleware did not run"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/middleware/verified_test.go#TestRequireVerified_VerifiedAccountReachesHandler"
        status: pass
      - kind: unit
        ref: "internal/middleware/verified_test.go#TestRequireVerified_UnverifiedAccountReturns403AndHandlerNeverRuns"
        status: pass
      - kind: unit
        ref: "internal/middleware/verified_test.go#TestRequireVerified_UnknownSubjectReturns401AndHandlerNeverRuns"
        status: pass
      - kind: unit
        ref: "internal/middleware/verified_test.go#TestRequireVerified_MissingAuthReturns401AndHandlerNeverRuns"
        status: pass
      - kind: unit
        ref: "internal/middleware/verified_test.go#TestUserFromContext_ReturnsErrorWhenMiddlewareDidNotRun"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 09: Email Verification Summary

**Hashed single-use verification tokens (internal/mail, Resend + log drivers), verify/resend/callback HTTP handlers, and a RequireVerified route-boundary gate enforcing D-04 outside the UI.**

## Performance

- **Duration:** 45 min
- **Tasks:** 3 (each executed as RED then GREEN TDD, per 01-03/01-06 precedent)
- **Files modified:** 8 created (0 modified)

## Accomplishments

- `internal/mail`: `Mailer`/`Sender`/`Service` issue a 256-bit verification token, store only its SHA-256 digest, supersede any outstanding token on every issue (`DeleteForUser` before `Insert`), and compose the callback link and message copy; `NewResendSender` posts to `https://api.resend.com/emails` with a bearer key and a 10s-timeout client, never leaking the provider's response body; `NewLogSender` is a first-class `MAIL_DRIVER=log` driver with identical token semantics, just a different transport
- `internal/httpapi.VerifyEmailHandler`: `Verify` consumes a token exactly once and mints the account's first session on success; `Resend` always returns a byte-identical 202 (registered, unregistered, or already-verified addresses are indistinguishable to the caller), rate limited to 1/30s by IP+email to match the client's countdown; `Callback` HTML-escapes the token before interpolating it into a redirect page and performs no verification itself
- `internal/middleware.RequireVerified`/`UserFromContext`: makes D-04 (email verification required) a route-boundary property instead of only a UI convention -- a direct API call from an unverified session cannot reach a verified-only route

## Task Commits

Each task was committed atomically as RED then GREEN (TDD):

1. **Task 1 (RED): failing tests for mail Service, log driver, Resend adapter** - `b2e352b` (test)
1. **Task 1 (GREEN): mailer interface, Resend adapter, log driver, token issuing** - `1457378` (feat)
2. **Task 2 (RED): failing tests for verify/resend/callback handlers** - `9d0ef66` (test)
2. **Task 2 (GREEN): verify, resend, and browser callback handlers** - `2d8be59` (feat)
3. **Task 3 (RED): failing tests for the verified-only middleware gate** - `1811043` (test)
3. **Task 3 (GREEN): verified-only middleware gate** - `3788373` (feat)

**Plan metadata:** this commit (`docs(01-09)`)

## Files Created/Modified

- `internal/mail/mailer.go` - `Mailer`, `Sender`, `Service`, `NewService`, token issuance/hashing, message composition
- `internal/mail/mailer_test.go` - 6 test cases against local fakes (`fakeMailVerificationRepo`, `fakeSender`)
- `internal/mail/resend.go` - `NewResendSender`, `resendSender` (endpoint kept as a field so tests can redirect it)
- `internal/mail/log_mailer.go` - `NewLogSender`, `logSender`
- `internal/httpapi/verify_email.go` - `VerifyEmailHandler`, `NewVerifyEmailHandler`, `Register`, `Verify`, `Resend`, `Callback`
- `internal/httpapi/verify_email_test.go` - 7 test cases, including a cross-package integration test against the real `internal/mail.Service`
- `internal/middleware/verified.go` - `RequireVerified`, `UserFromContext`, `ErrNoUserInContext`
- `internal/middleware/verified_test.go` - 5 test cases against a local `fakeVerifiedRepo`

## Decisions Made

See `key-decisions` in frontmatter. Summary: `Mailer`'s signature follows the plan exactly (`SendVerificationEmail(ctx, *user.User) error`); `testsupport_test.go`'s placeholder `mailer` interface was deliberately left unmodified (structurally can't express `Service`'s contract, and is shared with three in-flight sibling worktrees) -- `verify_email_test.go` declares its own local `fakeVerifyMailer` against the real interface instead. The Resend handler binds via `ShouldBindBodyWith` (not `ShouldBindJSON`) because the rate-limit middleware ahead of it already drains and caches the body computing its key. Package-level symbols in the shared `internal/httpapi` package were prefixed (`verifyEmailRequest`, `hashVerificationToken`, etc.) to avoid collisions with 01-08/01-10/01-11. `REQUIREMENTS.md` was left unmodified, matching 01-03/01-06's precedent, since this plan delivers only the verification slice of ACCT-01.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Resend handler would 400 instead of 202 with a naive `ShouldBindJSON`**
- **Found during:** Task 2, before writing the resend test (caught by an advisor review of the design, not by a failing test)
- **Issue:** The plan's `Register` attaches `RateLimit(LimitConfig{KeyFunc: KeyByIPAndField("email")})` to the resend route. `KeyByIPAndField` calls `c.ShouldBindBodyWith(&body, binding.JSON)` to compute its key, which drains `c.Request.Body` and caches the bytes under gin's body-cache key. A handler that then calls `c.ShouldBindJSON` reads `Request.Body` directly (not the cache) and gets `EOF`, so every resend request would 400 rather than 202 -- and a byte-identity assertion between two 400 `validation_failed` bodies would pass while the endpoint was actually broken.
- **Fix:** `Resend` binds via `c.ShouldBindBodyWith(&req, binding.JSON)` instead, reading the same cached body the rate limiter already populated.
- **Files modified:** internal/httpapi/verify_email.go (no separate commit; folded into the Task 2 GREEN commit since it was applied before the first implementation attempt, not as a later fix)
- **Verification:** `TestVerifyEmailResend_RegisteredAndUnregisteredByteIdentical` asserts `202` explicitly (not just byte-identity) and that the fake mailer recorded exactly one send, for the registered address only

**2. [Rule 2 - Missing Critical] Added a cross-package integration test beyond the plan's 6 named behaviors**
- **Found during:** Task 2, design review
- **Issue:** The plan's 6 acceptance behaviors are all satisfiable using `issueTestVerificationToken`, a hand-rolled helper that mirrors `internal/mail.Service`'s own hashing. If `Service` and `Verify` ever disagreed on how a token is encoded or hashed, every planned test would still pass while real verification was dead on arrival, because the helper encodes the same assumption the code under test makes.
- **Fix:** Added `TestVerifyEmail_IntegratesWithMailServiceIssuedToken`, which constructs a real `mail.NewService`, calls `SendVerificationEmail`, extracts the token from the captured message, and POSTs it to `/v1/auth/verify-email` expecting 200 -- the only test in this plan that would catch that specific class of drift.
- **Files modified:** internal/httpapi/verify_email_test.go
- **Verification:** Test passes; confirms `Service`'s `base64.RawURLEncoding` + `sha256.Sum256(raw)` and `Verify`'s `hashVerificationToken` agree end to end

---

**Total deviations:** 2 auto-fixed (both Rule 2 - missing critical, caught during design/advisor review before implementation rather than after a failing test)
**Impact on plan:** Both necessary for correctness (#1 fixes a genuine bug the plan's own acceptance criteria would not have caught; #2 closes a test-suite blind spot). No scope creep -- no new files, libraries, or routes beyond what the plan specified.

## Issues Encountered

None beyond the two deviations above, both resolved before the first implementation attempt.

## User Setup Required

None - no external service configuration required. `MAIL_DRIVER=log` (already configured per 01-01) exercises the log driver path this plan's tests and any local dev run will use; the Resend adapter compiles and is unit-tested against an `httptest.Server` standing in for the real endpoint, but is not invoked with a real API key in this environment (consistent with 01-01's deferral).

## Next Phase Readiness

- `mail.NewService(repo user.EmailVerificationRepository, sender mail.Sender, baseURL string, ttl time.Duration) *mail.Service` is the exact signature plan 01-13 wires up, selecting `mail.NewResendSender(cfg.ResendAPIKey, cfg.MailFromAddress, cfg.MailFromName)` or `mail.NewLogSender(os.Stdout)` (or similar) by `cfg.MailDriver`.
- `middleware.RequireVerified(users user.Repository) gin.HandlerFunc` is ready for plan 01-11 (every profile route) and plan 01-13 (the authenticated route group as a whole) to attach.
- `VerifyEmailHandler.Register(rg *gin.RouterGroup)` expects to be mounted on a `/v1/auth` group alongside 01-08's signup/login/refresh routes and 01-10's OAuth routes; no route path or package-level symbol collision exists with either sibling plan as far as this worktree's own file scope can confirm (only 01-08 and 01-10 share `internal/httpapi`'s package namespace with this plan; 01-11 stays in its own file prefixes per the wave's file-overlap guarantee).
- `testsupport_test.go`'s placeholder `mailer` interface still does not match the real `internal/mail.Mailer` signature -- flagged again here (as 01-06's SUMMARY.md already flagged it) for whichever plan next touches that file, most likely 01-13's wiring work or a future consolidation pass.
- No blockers.

## Self-Check: PASSED
- FOUND: internal/mail/mailer.go
- FOUND: internal/mail/mailer_test.go
- FOUND: internal/mail/resend.go
- FOUND: internal/mail/log_mailer.go
- FOUND: internal/httpapi/verify_email.go
- FOUND: internal/httpapi/verify_email_test.go
- FOUND: internal/middleware/verified.go
- FOUND: internal/middleware/verified_test.go
- FOUND: commit b2e352b
- FOUND: commit 1457378
- FOUND: commit 9d0ef66
- FOUND: commit 2d8be59
- FOUND: commit 1811043
- FOUND: commit 3788373
- go test ./internal/mail ./internal/middleware ./internal/httpapi -count=1 -- PASS
- go build ./... -- PASS
- No file under internal/mail imports internal/httpapi -- confirmed via grep
- No em dash character in any file this plan touched -- confirmed via grep

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
