---
phase: 01-foundation-accounts
plan: 10
subsystem: auth
tags: [oauth, google, apple, jwt, keyfunc, idtoken, gin, rate-limiting]

requires:
  - phase: 01-foundation-accounts (plan 06)
    provides: internal/auth (IssueAccessToken, RefreshService), internal/middleware (RateLimit, LimitConfig, KeyByIP), internal/httpapi (Respond, RespondError, RespondValidationError, newTestRouter, TestDeps, fakeUserRepo, fakeRefreshRepo)
  - phase: 01-foundation-accounts (plan 03)
    provides: user.Repository (GetByProviderSubject, LinkProviderSubject, Create, UpdateProfile), user.VerificationSource, user.ErrTokenInvalid / ErrNotFound
provides:
  - internal/auth: GoogleVerifier / NewGoogleVerifier (google.golang.org/api/idtoken-backed, audience-loop across plan 01-01's three client IDs, rejects an unverified email claim)
  - internal/auth: AppleVerifier / NewAppleVerifier (MicahParks/keyfunc/v3-backed JWKS cache, golang-jwt/v5 parse pinning RS256 + issuer + audience)
  - internal/httpapi: OAuthHandler / NewOAuthHandler mounting POST /auth/oauth/apple and /auth/oauth/google, create-or-link account flow, Apple's one-shot name capture handled without ever clearing a stored name
affects: [01-15]

tech-stack:
  added: []
  patterns:
    - "Provider verifiers are exposed as exported interfaces (GoogleVerifier, AppleVerifier) over an unexported concrete type holding an injectable seam -- a func field for Google (googleValidateFunc), an unexported constructor parameter for Apple (newAppleVerifier(ctx, audiences, jwksURL)) -- so tests substitute crafted claims or a local httptest.Server JWKS instead of ever reaching the real provider network"
    - "claimBool (internal/auth/oauth_google.go) tolerantly parses a JWT claim a provider may encode as either a JSON boolean or a boolean-valued string; reused unmodified by oauth_apple.go for email_verified and is_private_email"
    - "Social account create-or-link resolves in a fixed order -- GetByProviderSubject (repeat sign-in) -> GetByEmailCI + LinkProviderSubject (password account claiming a social identity) -> Create + LinkProviderSubject (brand new account) -- and every branch ends with the provider subject actually persisted, since Create's own signature only records verification source, not the subject"

key-files:
  created:
    - internal/auth/oauth_google.go
    - internal/auth/oauth_google_test.go
    - internal/auth/oauth_apple.go
    - internal/auth/oauth_apple_test.go
    - internal/httpapi/oauth.go
    - internal/httpapi/oauth_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Confirmed google.golang.org/api/idtoken's import path and Validate signature by reading the vendored v0.298.0 source directly (google.golang.org/api@v0.298.0/idtoken/validate.go in the local module cache) rather than only trusting training-knowledge recall, since RESEARCH.md flagged this detail unverified: func Validate(ctx context.Context, idToken string, audience string) (*Payload, error), with Payload{Issuer, Audience, Expires, IssuedAt, Subject, Claims map[string]interface{}}. pkg.go.dev renders its docs directly from this same pinned-version source, so this is the ground-truth equivalent of checking the live page."
  - "google.golang.org/api/idtoken transitively imports cloud.google.com/go/auth/credentials/idtoken, which was not yet in go.sum (google.golang.org/api was previously only an indirect, unimported dependency). Ran `go get google.golang.org/api/idtoken@v0.298.0` -- same version already pinned in go.mod -- to add the missing go.sum entries; this pulled in a sizeable transitive chain (cloud.google.com/go/auth, grpc, go.opentelemetry.io) that the idtoken package itself depends on. Same class of fix as plan 01-03's pgxpool go.sum gap."
  - "A newly created social account is linked to its provider subject via LinkProviderSubject immediately after Create, not left to a later sign-in. Create's signature (email, passwordHash, verified, via *VerificationSource) has nowhere to carry the subject, so without this explicit link call a second sign-in would never resolve through GetByProviderSubject and would instead silently fall through to the GetByEmailCI branch on every subsequent sign-in. Caught by TestOAuth_FirstAppleSignInCreatesVerifiedAccountWithName failing before this was added -- see Deviations."
  - "persistNameIfUnset is shared by both providers and only ever writes a name into a currently-empty field. This is required for Apple (full_name is null on every authorization after the first) and is a safe no-op for Google (whose token includes name on every sign-in, but an account's name should still never be silently overwritten by a later sign-in once set)."
  - "Apple's JWKS fetch is reachable only through an unexported newAppleVerifier(ctx, audiences, jwksURL) constructor; the exported NewAppleVerifier(audiences) just calls it with Apple's real endpoint and context.Background(). This is the seam the test file uses to point keyfunc.NewDefaultCtx at a local httptest.Server instead of https://appleid.apple.com/auth/keys, and to cancel the background refresh goroutine on test cleanup via the context."

patterns-established:
  - "jwt.WithAudience(v.audiences...) is called once with every configured audience rather than looped per-audience, relying on golang-jwt/v5's default expectAllAud=false (OR semantics) -- confirmed by reading validator.go directly rather than assuming"
  - "A second signing-method check is duplicated inside the keyfunc closure itself (*jwt.SigningMethodRSA type assertion) in addition to jwt.WithValidMethods, mirroring the HS256 double-check plan 01-06 established in internal/auth/jwt.go, so WithValidMethods is never the sole defense against algorithm confusion"

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "Google ID token verifier: audience-loop across every configured client ID, signature/expiry delegated to google.golang.org/api/idtoken.Validate, rejects an unverified email_verified claim, every failure maps to user.ErrTokenInvalid"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_ValidAudienceAndSignatureReturnsIdentity"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_TriesEveryConfiguredClientIDBeforeRejecting"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_AudienceMatchingNoneIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_InvalidSignatureIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_ExpiredTokenIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_UnverifiedEmailIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_google_test.go#TestGoogle_NewGoogleVerifierWiresRealValidateFunc"
        status: pass
    human_judgment: false
  - id: D2
    description: "Apple identity token verifier over Apple's JWKS: RS256-only via keyfunc/v3 + golang-jwt/v5, issuer and audience pinned explicitly, is_private_email/email_verified parsed tolerantly (bool or string), hand-crafted alg:none rejected"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_ValidTokenReturnsIdentity"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_WrongIssuerIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_WrongAudienceIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_KeyAbsentFromJWKSIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_ExpiredTokenIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_AlgNoneIsRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth_apple_test.go#TestApple_EmailVerifiedAndPrivateRelayToleratesBooleanEncoding"
        status: pass
    human_judgment: false
  - id: D3
    description: "Apple and Google sign-in endpoints: raw-token-only request DTOs (no email/provider field), create-or-link account flow, Apple's one-shot name capture never clobbered by a later null, a fresh social account reports onboarding_complete false"
    requirement: ACCT-01
    verification:
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_FirstAppleSignInCreatesVerifiedAccountWithName"
        status: pass
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_SecondAppleSignInReturnsSameAccountAndKeepsName"
        status: pass
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_GoogleSignInLinksExistingAccountByEmail"
        status: pass
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_RequestWithEmailOrProviderFieldInsteadOfTokenIsRejected"
        status: pass
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_InvalidProviderTokenReturns401TokenInvalid"
        status: pass
      - kind: unit
        ref: "internal/httpapi/oauth_test.go#TestOAuth_NewSocialAccountHasOnboardingIncomplete"
        status: pass
    human_judgment: false
  - id: D4
    description: "Live verification against Google's and Apple's real signing endpoints, and a real device sign-in with real GOOGLE_CLIENT_ID_*/APPLE_BUNDLE_ID values"
    verification: []
    human_judgment: true
    rationale: "Real Google/Apple OAuth client IDs are not configured in this environment -- the checkpoint at plan 01-01 deferred them, so GOOGLE_CLIENT_ID_IOS/_ANDROID/_WEB and APPLE_BUNDLE_ID hold non-functional placeholder values. Every test here exercises the verification logic against hand-crafted/mocked tokens and injected/mocked verifiers, never a real network call to Google's or Apple's endpoints. Real-device/real-account verification is explicitly deferred to plan 01-15's walkthrough (Wave 6 checkpoint or beyond), which the plan's own success_criteria notes as the first point this becomes meaningful."

duration: 5min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 10: Apple and Google Sign-In Summary

**Server-side Google ID token verification (google.golang.org/api/idtoken, audience-loop across three platform client IDs) and Apple identity token verification (MicahParks/keyfunc/v3 JWKS cache + golang-jwt/v5, issuer/audience pinned), wired to endpoints that create or link an account from the verified identity and never trust a client-shaped `{email, provider}` payload.**

## Performance

- **Duration:** ~5 min (commit-to-commit span across the three TDD tasks; the upfront context-loading/read phase, including confirming the idtoken signature against the vendored source and reading keyfunc/jwkset internals, is not separately timestamped)
- **Started:** 2026-09-18T13:47:32Z (first commit)
- **Completed:** 2026-09-18T13:51:54Z (last task commit)
- **Tasks:** 3 (each executed as RED -> GREEN TDD; no task needed a separate REFACTOR commit)
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- `internal/auth/oauth_google.go`: `GoogleVerifier`/`NewGoogleVerifier` wrapping `google.golang.org/api/idtoken.Validate` behind an injectable `googleValidateFunc` seam, trying every configured client ID (plan 01-01 provisions iOS/Android/web) before rejecting, and requiring the `email_verified` claim before trusting the address -- every failure maps to `user.ErrTokenInvalid`
- `internal/auth/oauth_apple.go`: `AppleVerifier`/`NewAppleVerifier` fetching and caching Apple's JWKS via `MicahParks/keyfunc/v3`, parsing with `golang-jwt/jwt/v5` pinned to `RS256` (plus a duplicated `*jwt.SigningMethodRSA` type assertion inside the keyfunc, matching plan 01-06's HS256 double-check pattern), `iss=="https://appleid.apple.com"`, and the configured bundle/service identifiers as audience
- `internal/httpapi/oauth.go`: `OAuthHandler` mounting `POST /auth/oauth/apple` and `POST /auth/oauth/google` (10 req/min per IP each), request DTOs carrying only `identity_token`/`id_token` (plus Apple's optional `full_name`), a shared `findOrCreateAccount` resolving by provider subject then email-link then create, and `persistNameIfUnset` guarding against Apple's null-after-first-authorization name delivery
- Confirmed and recorded `google.golang.org/api/idtoken`'s import path and `Validate` signature against the vendored v0.298.0 source, resolving RESEARCH.md's flagged unverified detail (see Decisions below)

## Task Commits

Each task was committed atomically as RED then GREEN (TDD, no separate REFACTOR needed):

1. **Task 1 (RED): failing test for Google ID token verification** - `d08fcce` (test)
1. **Task 1 (GREEN): Google ID token verifier** - `2885e83` (feat)
2. **Task 2 (RED): failing test for Apple identity token verification** - `b24556e` (test)
2. **Task 2 (GREEN): Apple identity token verifier over JWKS** - `9dcbd17` (feat)
3. **Task 3 (RED): failing test for Apple/Google sign-in endpoints** - `681919a` (test)
3. **Task 3 (GREEN): Apple and Google sign-in endpoints** - `4831102` (feat)

**Plan metadata:** this commit (`docs(01-10)`)

## Files Created/Modified

- `internal/auth/oauth_google.go` - `GoogleIdentity`, `GoogleVerifier`, `NewGoogleVerifier`, `claimBool` helper (also reused by oauth_apple.go)
- `internal/auth/oauth_google_test.go` - 7 test cases against an injected `validate` seam, no network call
- `internal/auth/oauth_apple.go` - `AppleIdentity`, `AppleVerifier`, `NewAppleVerifier`, unexported `newAppleVerifier(ctx, audiences, jwksURL)` test seam
- `internal/auth/oauth_apple_test.go` - 7 test cases against a locally generated RSA key + `httptest.Server`-served JWKS, no network call
- `internal/httpapi/oauth.go` - `OAuthHandler`, `NewOAuthHandler`, `.Register`, `.SignInWithApple`, `.SignInWithGoogle`, `AppleSignInRequest`, `GoogleSignInRequest`
- `internal/httpapi/oauth_test.go` - 6 test cases against `newTestRouter`/`fakeUserRepo`/`fakeRefreshRepo` plus local `fakeAppleVerifier`/`fakeGoogleVerifier`
- `go.mod` / `go.sum` - added the transitive dependency chain `google.golang.org/api/idtoken` pulls in (`cloud.google.com/go/auth`, `google.golang.org/grpc`, `go.opentelemetry.io/*`, etc.), all resolved at the same `google.golang.org/api` version already pinned

## Final Signatures (verbatim, for later phases)

```go
// internal/auth
type GoogleIdentity struct { Subject, Email string; EmailVerified bool; Name string }
type GoogleVerifier interface { Verify(ctx context.Context, idToken string) (*GoogleIdentity, error) }
func NewGoogleVerifier(clientIDs []string) GoogleVerifier

type AppleIdentity struct { Subject, Email string; EmailVerified bool; IsPrivateRelay bool }
type AppleVerifier interface { Verify(ctx context.Context, identityToken string) (*AppleIdentity, error) }
func NewAppleVerifier(audiences []string) (AppleVerifier, error)

// internal/httpapi
type AppleSignInRequest struct { IdentityToken string; FullName *string }
type GoogleSignInRequest struct { IDToken string }
type OAuthHandler struct { /* unexported */ }
func NewOAuthHandler(users user.Repository, appleVerifier auth.AppleVerifier, googleVerifier auth.GoogleVerifier, refreshTokens *auth.RefreshService, jwtSecret []byte, accessTokenTTL time.Duration) *OAuthHandler
func (h *OAuthHandler) Register(rg *gin.RouterGroup)
```

Routes mounted by `Register`: `POST /auth/oauth/apple`, `POST /auth/oauth/google` (the `/v1` prefix, per other wave 4 plans' convention, is added by whichever router group `cmd/api/main.go` eventually passes in -- this plan owns no central router file, matching 01-08's precedent).

## Decisions Made

See `key-decisions` in frontmatter. Summary: confirmed `idtoken.Validate`'s signature against the vendored v0.298.0 source directly rather than relying on recall; `go get google.golang.org/api/idtoken@v0.298.0` pulled in a real (if large) transitive chain the package needs, at the version already pinned; new social accounts are explicitly linked to their provider subject right after `Create` since `Create` itself has no subject parameter; `persistNameIfUnset` is shared by both providers and is a no-op once a name is set, which is what protects Apple's one-shot name delivery.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `go.sum` missing transitive entries for `google.golang.org/api/idtoken`**
- **Found during:** Task 1 RED phase (`go test ./internal/auth -run TestGoogle`)
- **Issue:** `missing go.sum entry for module providing package cloud.google.com/go/auth/credentials/idtoken (imported by google.golang.org/api/idtoken)`. `google.golang.org/api` was already pinned in `go.mod` from plan 01-01 but had never actually been imported by any package yet, so its own transitive dependencies had no `go.sum` hashes.
- **Fix:** `go get google.golang.org/api/idtoken@v0.298.0` -- the same version already in `go.mod`; this added the missing indirect `require` lines and their `go.sum` hashes (`cloud.google.com/go/auth`, `cloud.google.com/go/auth/oauth2adapt`, `cloud.google.com/go/compute/metadata`, `google.golang.org/grpc`, `go.opentelemetry.io/*`, and a few smaller packages) without changing any existing dependency version.
- **Verification:** `go build ./...` and `go vet ./...` both clean afterward; `go test ./internal/auth -run TestGoogle` proceeds past the compile step.
- **Files modified:** `go.mod`, `go.sum` (bundled into the Task 1 RED commit `d08fcce`).

**2. [Rule 1 - Bug caught by acceptance test] New social accounts were never linked to their provider subject**
- **Found during:** Task 3 GREEN phase, first test run (`TestOAuth_FirstAppleSignInCreatesVerifiedAccountWithName` failed: `GetByProviderSubject: user: not found`)
- **Issue:** The initial `findOrCreateAccount` implementation called `h.users.Create(...)` for a brand-new account but never called `LinkProviderSubject` afterward. `Create`'s signature (`email, passwordHash, verified, via *VerificationSource`) has no subject parameter, so the created row's `apple_subject`/`google_subject` column stayed empty. A genuine second sign-in would silently have fallen through to the `GetByEmailCI` branch every time instead of resolving via `GetByProviderSubject` as the plan's first behavior explicitly requires ("the Apple subject stored"). The second-sign-in test happened to still pass despite the bug, because it took the email-match branch instead of the intended provider-subject branch -- the direct-lookup test is what caught it.
- **Fix:** Added `h.users.LinkProviderSubject(ctx, created.ID, provider, subject)` immediately after `Create` in the new-account branch.
- **Verification:** All 6 `TestOAuth_*` cases pass, including a direct `users.GetByProviderSubject` assertion right after a first sign-in.
- **Files modified:** `internal/httpapi/oauth.go`.
- **Committed in:** `4831102` (Task 3 GREEN commit; the fix was made before that commit, not as a follow-up).

---

**Total deviations:** 2 auto-fixed (1 blocking build/tooling issue, 1 logic bug caught by the plan's own acceptance test before the GREEN commit).
**Impact on plan:** Both necessary for correctness. No scope creep -- no library beyond what the plan specified (`google.golang.org/api/idtoken`, `MicahParks/keyfunc/v3`, `golang-jwt/jwt/v5`, all already pinned by plan 01-01) was introduced.

## Issues Encountered

None beyond the two deviations above, both resolved within the task they were found in.

## User Setup Required

None for this plan's own scope -- no new external service configuration was introduced (the Google/Apple client IDs and libraries were already provisioned in plan 01-01).

**Carried-forward gap, not new to this plan:** `GOOGLE_CLIENT_ID_IOS`/`_ANDROID`/`_WEB` and `APPLE_BUNDLE_ID` still hold non-functional placeholder values in this environment (plan 01-01's checkpoint deferred real provisioning). This plan's verifiers are therefore correct by construction and by test against hand-crafted tokens, but have never been exercised against a real Google or Apple identity token. Real-device verification -- installing the mobile app, signing in with a real Apple ID and Google account, and confirming the deferred one-shot Apple name-capture bug scenario in practice -- is deferred to plan 01-15's walkthrough (Wave 6 checkpoint or beyond), which is the first point real client IDs are expected to be configured.

## Next Phase Readiness

- `internal/auth.GoogleVerifier`/`AppleVerifier` and `internal/httpapi.OAuthHandler` are ready for `cmd/api/main.go` wiring whenever that lands (no central router file exists yet in this phase; each wave 4 plan, including this one, owns only its own `Register` method per 01-08's established precedent).
- `01-15`'s device walkthrough is the first point these verifiers face real provider tokens; its plan should specifically include a sign-out/sign-in-again pass to exercise Apple's one-shot name-capture behavior end-to-end, since that is the single most fragile behavior in this plan and is invisible on a first manual test.
- No blockers for sibling wave-4 plans (01-08, 01-09, 01-11): zero file overlap, and this plan's `go.mod`/`go.sum` additions are purely additive (new indirect entries, no version bumps to anything another plan depends on).

## Self-Check: PASSED

- FOUND: internal/auth/oauth_google.go
- FOUND: internal/auth/oauth_google_test.go
- FOUND: internal/auth/oauth_apple.go
- FOUND: internal/auth/oauth_apple_test.go
- FOUND: internal/httpapi/oauth.go
- FOUND: internal/httpapi/oauth_test.go
- FOUND: commit d08fcce
- FOUND: commit 2885e83
- FOUND: commit b24556e
- FOUND: commit 9dcbd17
- FOUND: commit 681919a
- FOUND: commit 4831102
- go test ./internal/auth -run TestGoogle -v -count=1 -- PASS (7 cases)
- go test ./internal/auth -run TestApple -v -count=1 -- PASS (7 cases)
- go test ./internal/httpapi -run TestOAuth -v -count=1 -- PASS (6 cases)
- go test ./internal/auth ./internal/httpapi -count=1 -- PASS, no external network access
- go build ./... -- PASS
- go vet ./... -- PASS
- gofmt -l on all 6 new files -- clean
- All task-level `<verify>` grep/acceptance commands from 01-10-PLAN.md -- PASS

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
