---
phase: 01-foundation-accounts
plan: 11
subsystem: api
tags: [gin, aws-sdk-go-v2, s3, presigned-url, username, profile, go]

requires:
  - phase: 01-foundation-accounts (plan 03)
    provides: internal/user domain model, ProfilePatch, Repository interface (UsernameTaken, UpdateProfile, GetByID)
  - phase: 01-foundation-accounts (plan 06)
    provides: internal/middleware (RequireAuth, SubjectFromContext, RateLimit, KeyBySubject), internal/httpapi (Respond, RespondError, RespondValidationError, ErrorCode), internal/httpapi/testsupport_test.go shared harness (newTestRouter, TestDeps, fakeUserRepo)
provides:
  - internal/user: NormalizeUsername, ValidateUsername, SuggestUsername, SuggestAlternates
  - internal/storage: UploadTicket, AvatarStore, Config, NewAvatarStore, PresignAvatarUpload (presign-only, stub-tested, no real bucket reached)
  - internal/httpapi: ProfileHandler, NewProfileHandler, UpdateProfileRequest, AvatarUploadRequest, UsernameHandler, NewUsernameHandler
  - Routes (unmounted until plan 01-13 assembles cmd/api/main.go): GET /v1/me, PATCH /v1/me, POST /v1/me/avatar/upload-url, GET /v1/usernames/suggest, GET /v1/usernames/available
affects: [01-12, 01-13, 01-14, 01-15]

tech-stack:
  added: []
  patterns:
    - "storage.Config is a narrow, package-local struct (6 S3 fields, field-for-field matching internal/config.Config) rather than importing the full app Config -- keeps internal/storage decoupled, matching internal/auth.NewRefreshService(repo, ttl)'s narrow-dependency precedent from plan 01-06"
    - "Register(rg *gin.RouterGroup) methods add bare routes only; RequireAuth/RequireVerified are the caller's responsibility to attach to rg before Register runs -- production wiring is plan 01-13's job (cmd/api/main.go, internal/httpapi/server.go), tests attach RequireAuth themselves"
    - "HTTP response bodies that must never resemble a request-side user-ID field are built as gin.H literals, not tagged structs, in internal/httpapi/profile.go -- keeps the access-control grep gate meaningful"
    - "Every JSON array field that can be empty (alternates, suggestions) is built via make([]string, 0, n) / explicit []string{} on every code path, never left as a nil slice, so it marshals as [] and never null"

key-files:
  created:
    - internal/user/username.go
    - internal/user/username_test.go
    - internal/storage/s3.go
    - internal/storage/s3_test.go
    - internal/httpapi/profile.go
    - internal/httpapi/profile_test.go
    - internal/httpapi/username.go
    - internal/httpapi/username_test.go
  modified: []

key-decisions:
  - "storage.Config is a new package-local struct (not internal/config.Config passed directly), holding only the 6 S3 fields with identical names -- plan 01-13 constructs one field-for-field from the real config.Config when wiring cmd/api/main.go"
  - "Register(rg *gin.RouterGroup) on both handlers adds routes only; it does not call rg.Use(RequireAuth(...)) or reference RequireVerified. Plan 01-13-PLAN.md (line 94) confirms this is the intended shape: it creates a nested authenticated group applying RequireAuth(secret) then RequireVerified(repo) and mounts ProfileHandler/UsernameHandler there. internal/middleware.RequireVerified does not exist in this worktree (it is plan 01-09's deliverable, building in a sibling worktree not yet merged) and this plan's verify gates never grep for it -- this is a wiring-contract note, not a deviation; nothing here deviated from the plan."
  - "PATCH /me's 409 conflict handler defaults the requested-username value to empty string if req.Username is somehow nil when ErrUsernameTaken is returned (defensive; the real repository can only raise that error when a username was actually submitted, matching plan 01-03's UpdateProfile/coalesce semantics)"
  - "GET /usernames/suggest's alternates are derived from NormalizeUsername(name) (the plain base), not re-normalizing the already-suffixed `username` suggestion -- so alternates are independent suffixed variants of the same base rather than variants of the specific suggested handle"
  - "REQUIREMENTS.md left unmodified for ACCT-01 and ACCT-03. This plan genuinely ships a working, fully unit-tested GET /v1/me -- the backend half of ACCT-03 -- so the 01-01/01-03 precedent's literal reasoning ('no HTTP endpoint exists yet') does not transfer cleanly here and is not the operative reason. The actual reasons: (1) REQUIREMENTS.md's checkbox lines and Traceability table rows are a shared-file conflict risk across this wave's four parallel worktrees (01-08/01-09/01-10/01-11), each of which lists ACCT-01 and/or ACCT-03 in its own frontmatter and could independently edit the same lines on merge -- the orchestrator is the single writer for exactly this reason with STATE.md/ROADMAP.md, and the same logic applies here even though the tooling doesn't enforce it; (2) ACCT-03 as a user-facing statement ('User can view their own profile') also needs its UI half, which doesn't exist until plan 01-14's profile view screen, and ACCT-01 additionally needs the sibling wave-4 plans (01-08 signup/login, 01-09 verification, 01-10 OAuth) merged. Marking either complete from this worktree alone would risk a false positive and a merge conflict; the orchestrator (or a later phase-completion pass) should reconcile REQUIREMENTS.md once the full wave is merged and 01-13/01-14 land."

patterns-established:
  - "Advisory availability/suggestion endpoints (GET /usernames/*) always return a non-nil alternates array, even when empty, so the wire contract's `alternates: string[]` never marshals as null"
  - "A save-time uniqueness conflict (ErrUsernameTaken from the repository) is handled by a dedicated respond*Taken helper that regenerates fresh alternates at conflict time, rather than reusing any pre-fetch suggestion state -- the conflict is always answered with current data"

requirements-completed: []

coverage:
  - id: D1
    description: "Username normalization/validation/suggestion (internal/user/username.go) -- NormalizeUsername strips to a valid [a-z0-9_]{3,20} handle and falls back to a usable base (never blank) for empty/non-Latin/all-punctuation input, truncates long names, ValidateUsername matches the DB check constraint exactly, SuggestUsername/SuggestAlternates use crypto/rand suffixes (never a predictable increment) against the advisory UsernameTaken check"
    requirement: ACCT-03
    verification:
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_NormalizeStripsSpacesCapsAndPunctuation"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_NormalizeFallsBackToUsableBaseWhenEmpty"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_NormalizeTruncatesLongNamesRatherThanFailing"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_ValidateRejectsUppercasePunctuationAndBadLengths"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_SuggestReturnsPlainBaseWhenFree"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_SuggestReturnsSuffixedVariantWhenBaseTakenAndSuffixIsNotPredictable"
        status: pass
      - kind: unit
        ref: "internal/user/username_test.go#TestUsername_SuggestAlternatesReturnsThreeDistinctFreeCandidates"
        status: pass
      - kind: other
        ref: "go test ./internal/user -run TestUsername -v -count=1 (7/7 pass); go vet ./internal/user; ValidateUsername's ^[a-z0-9_]{3,20}$ pattern confirmed present verbatim in migrations/0001_init.up.sql via grep -F (see Deviations -- the plan's own verify command uses an unescaped BRE and cannot match this pattern regardless of code correctness)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Presigned avatar upload tickets (internal/storage/s3.go) -- allows only image/jpeg, image/png, image/webp, caps content length at 5 MiB bound into the presigned PUT request itself, object key is avatars/{userID}/{random16hex}.{ext}, presign expiry is an explicit 300s, tested against a stub presigner seam with no network access"
    requirement: ACCT-03
    verification:
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_AllowedImageTypeReturnsPresignedURLAndPublicURL"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_DisallowedContentTypeRejected"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_ContentLengthAboveCapRejected"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_ObjectKeyIncludesOwningUserID"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_ExpiryIsSetExplicitly"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestS3_PresignAvatarUpload_DifferentUploadsGetDistinctObjectKeys"
        status: pass
      - kind: unit
        ref: "internal/storage/s3_test.go#TestNewAvatarStore_MissingConfigFieldReturnsError"
        status: pass
      - kind: other
        ref: "go test ./internal/storage -v -count=1 (7/7 pass, no network access); go vet ./internal/storage"
        status: pass
    human_judgment: true
    rationale: "Real-bucket verification (that a client can actually PUT to the presigned URL against a live S3-compatible endpoint and that the resulting object is fetchable at PublicURL) is explicitly deferred -- S3_ENDPOINT/S3_BUCKET/S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY/S3_PUBLIC_BASE_URL are non-functional placeholders in this environment per plan 01-01's checkpoint. A human must verify this against a provisioned bucket at plan 01-15's walkthrough or later."
  - id: D3
    description: "Profile and username HTTP endpoints (internal/httpapi/profile.go, username.go) -- GET/PATCH /me and POST /me/avatar/upload-url derive the target user exclusively from SubjectFromContext (no path parameter, no request/response ID field), PATCH /me updates the four profile fields independently via nil-means-unchanged pointers, a save-time username conflict returns 409 with 3 fresh alternates, bio over 160 chars returns 400 validation_failed, GET /usernames/suggest and /usernames/available are rate-limited 20/min by subject"
    requirement: ACCT-03
    verification:
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_GetMe_ReturnsCallerProfileWithOnboardingComplete"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_GetMe_NoBearerTokenReturns401"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_PatchMe_IgnoresUserIDInBody"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_PatchMe_UpdatesFieldsIndependently"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_PatchMe_UsernameTakenReturns409WithAlternates"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_PatchMe_BioOverLimitReturns400"
        status: pass
      - kind: unit
        ref: "internal/httpapi/profile_test.go#TestProfile_AvatarUploadURL_ReturnsTicketForCaller"
        status: pass
      - kind: unit
        ref: "internal/httpapi/username_test.go#TestUsernameEndpoint_Suggest_ReturnsFreeSuggestionDerivedFromName"
        status: pass
      - kind: unit
        ref: "internal/httpapi/username_test.go#TestUsernameEndpoint_Available_ReportsAvailabilityAndAlternatesWhenTaken"
        status: pass
      - kind: other
        ref: "go test ./internal/httpapi -run 'TestProfile|TestUsername' -v -count=1 (9/9 pass); go vet ./internal/httpapi; grep gates for no :id/:userID/:user_id path params, no json:\"id\"/json:\"user_id\" struct tags, SubjectFromContext, onboarding_complete, KeyBySubject, suggestions, and bio's max=160 binding all pass"
        status: pass
    human_judgment: true
    rationale: "These endpoints are not reachable by any real client yet -- cmd/api/main.go does not exist until plan 01-13 mounts ProfileHandler/UsernameHandler behind RequireAuth+RequireVerified. Functional correctness is fully proven by the harness tests above; end-to-end reachability is 01-13's and later a human's concern."

duration: "commit-to-commit span ~5 min (test/RED and feat/GREEN pairs across three tasks); upfront context-loading/read/research phase not separately timestamped, matching plan 01-06's precedent"
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 11: Profile, Username, and Avatar Upload Endpoints Summary

**GET/PATCH /me with independently-editable profile fields and save-time username conflict handling, advisory username suggest/availability endpoints rate-limited by subject, and presigned S3-compatible avatar upload tickets bounded by type/size at the provider -- all tested offline against in-memory/stub fakes.**

## Performance

- **Duration:** ~5 min commit-to-commit (three TDD tasks, each RED then GREEN; no REFACTOR commit needed for any)
- **Started:** 2026-09-18T19:18:38+05:30 (first commit)
- **Completed:** 2026-09-18T19:23:39+05:30 (last commit)
- **Tasks:** 3 (each executed as RED -> GREEN TDD)
- **Files modified:** 8 (8 created, 0 modified)

## Accomplishments

- `internal/user/username.go`: `NormalizeUsername` (lowercase, whitespace-to-underscore, charset-strip, collapse, trim, truncate-to-20, fallback to `"user"` when nothing usable survives -- never a blank handle), `ValidateUsername` (exactly `^[a-z0-9_]{3,20}$`, identical to the `users_username_charset` DB check constraint), `SuggestUsername`/`SuggestAlternates` (crypto/rand digit suffixes, never a predictable increment, against the advisory `UsernameTaken` check -- the authoritative rejection stays the insert-time unique-constraint violation plan 01-03 already maps to `ErrUsernameTaken`)
- `internal/storage/s3.go`: `NewAvatarStore`/`PresignAvatarUpload` build an aws-sdk-go-v2 S3 presign client (static credentials, path-style addressing, explicit `BaseEndpoint`) restricted to `image/jpeg`/`image/png`/`image/webp` and a 5 MiB cap bound into the presigned PUT request itself, with an owner-scoped, collision-free object key (`avatars/{userID}/{random16hex}.{ext}`) and an explicit 300s expiry -- exercised entirely against a stub presigner seam, no network access
- `internal/httpapi/profile.go`: `GET /me` (caller's own `ApiUser` shape, server-computed `onboarding_complete`), `PATCH /me` (four independently-editable fields via nil-means-unchanged pointers, save-time username conflict -> 409 with 3 fresh alternates), `POST /me/avatar/upload-url` (forwards to `AvatarStore`, surfaces type/size rejection as 400) -- every handler resolves its target exclusively via `middleware.SubjectFromContext`, with no route parameter and no request/response struct field naming a user ID anywhere in the file
- `internal/httpapi/username.go`: `GET /usernames/suggest` and `GET /usernames/available`, both rate-limited to 20 req/min keyed by the authenticated subject against enumeration

## Task Commits

Each task was committed atomically as RED then GREEN (TDD; no task needed a separate REFACTOR commit):

1. **Task 1 (RED): failing tests for username normalization/validation/suggestion** - `bd81e65` (test)
1. **Task 1 (GREEN): username normalization, validation, and suggestion** - `5d667bd` (feat)
2. **Task 2 (RED): failing tests for presigned avatar upload tickets** - `5a74d4d` (test)
2. **Task 2 (GREEN): presigned avatar upload tickets** - `3873be1` (feat)
3. **Task 3 (RED): failing tests for profile and username endpoints** - `93dfbbd` (test)
3. **Task 3 (GREEN): profile and username endpoints** - `d7785c9` (feat)

**Plan metadata:** this commit (`docs(01-11)`)

## Files Created/Modified

- `internal/user/username.go` / `username_test.go` - `NormalizeUsername`, `ValidateUsername`, `SuggestUsername`, `SuggestAlternates`; 7 tests against an in-memory `fakeUsernameRepo`
- `internal/storage/s3.go` / `s3_test.go` - `UploadTicket`, `AvatarStore`, `Config`, `NewAvatarStore`, `PresignAvatarUpload`; 7 tests against a stub `presigner` seam
- `internal/httpapi/profile.go` / `profile_test.go` - `ProfileHandler`, `NewProfileHandler`, `UpdateProfileRequest`, `AvatarUploadRequest`; 7 tests
- `internal/httpapi/username.go` / `username_test.go` - `UsernameHandler`, `NewUsernameHandler`; 2 tests

## Final Signatures and API Contract (verbatim, for plans 01-12 and 01-14)

### Go signatures

```go
// internal/user
func NormalizeUsername(displayName string) string
func ValidateUsername(s string) error
func SuggestUsername(ctx context.Context, repo Repository, displayName string) (string, error)
func SuggestAlternates(ctx context.Context, repo Repository, base string, n int) ([]string, error)

// internal/storage
type UploadTicket struct {
    UploadURL string
    PublicURL string
    ExpiresIn int
}
type AvatarStore interface {
    PresignAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*UploadTicket, error)
}
// Config: field names match internal/config.Config's S3 fields 1:1.
type Config struct {
    S3Endpoint        string
    S3Region          string
    S3Bucket          string
    S3AccessKeyID     string
    S3SecretAccessKey string
    S3PublicBaseURL   string
}
func NewAvatarStore(cfg Config) (AvatarStore, error)

// internal/httpapi
type UpdateProfileRequest struct {
    Name      *string `json:"name" binding:"omitempty,min=1,max=50"`
    Username  *string `json:"username" binding:"omitempty,min=3,max=20"`
    Bio       *string `json:"bio" binding:"omitempty,max=160"`
    AvatarURL *string `json:"avatar_url" binding:"omitempty,url"`
}
type AvatarUploadRequest struct {
    ContentType   string `json:"content_type" binding:"required"`
    ContentLength int64  `json:"content_length" binding:"required,gt=0"`
}
type ProfileHandler struct{ /* unexported */ }
func NewProfileHandler(users user.Repository, avatars storage.AvatarStore) *ProfileHandler
func (h *ProfileHandler) Register(rg *gin.RouterGroup) // adds routes only -- caller attaches RequireAuth/RequireVerified to rg first

type UsernameHandler struct{ /* unexported */ }
func NewUsernameHandler(users user.Repository) *UsernameHandler
func (h *UsernameHandler) Register(rg *gin.RouterGroup) // adds routes only -- caller attaches RequireAuth/RequireVerified to rg first
```

### Wiring contract for plan 01-13

`Register(rg)` on both handlers adds bare routes and applies no auth middleware itself. Plan 01-13 (`cmd/api/main.go` / `internal/httpapi/server.go`) is expected to build a nested group under `/v1` with `RequireAuth(secret)` then `RequireVerified(repo)` applied, and mount `ProfileHandler`/`UsernameHandler` on that group -- exactly as 01-13-PLAN.md already specifies. `internal/middleware.RequireVerified` does not exist in this worktree (plan 01-09's deliverable, a sibling wave-4 plan not yet merged); this plan's tests attach only `RequireAuth` to prove the 401-with-no-token behavior, which is all this plan's verify gates require.

### Routes (all under `/v1`, all require a bearer token once mounted by 01-13)

**`GET /v1/me`**
Request: no body.
200 response (`ApiUser`):
```json
{
  "id": "uuid-string",
  "email": "user@example.com",
  "name": "string or null",
  "username": "string or null",
  "bio": "string or null",
  "avatar_url": "string or null",
  "email_verified": true,
  "onboarding_complete": true
}
```
`onboarding_complete` is server-computed (`User.OnboardingComplete()` from plan 01-03: `email_verified && name != "" && username != ""`).
401 `token_invalid` / `token_expired` if the bearer token is missing or invalid (from `RequireAuth`, not this handler).

**`PATCH /v1/me`**
Request body -- every field optional, `null`/omitted means unchanged, present-and-non-null means "set this field":
```json
{ "name": "string?", "username": "string?", "bio": "string?", "avatar_url": "string?" }
```
Validation: `name` 1-50 chars, `username` 3-20 chars (further validated against `^[a-z0-9_]{3,20}$` before the repository call), `bio` <=160 chars, `avatar_url` must be a valid URL if present.
200 response: same `ApiUser` shape as `GET /me`, reflecting the merged update.
400 `validation_failed`: `{"error": "validation_failed", "message": "<binder or ValidateUsername detail>"}`.
409 `username_taken` (save-time conflict, the authoritative rejection):
```json
{ "error": "username_taken", "suggestions": ["alt1", "alt2", "alt3"] }
```
`suggestions` is always a 3-element array (or fewer only if the alternates generator itself fails, in which case it degrades to `[]` rather than 500ing the whole request) and is always present as `[]` at minimum, never `null`.
A request body containing an `id`/`user_id` field is silently ignored by the JSON binder (no such field exists on `UpdateProfileRequest`) -- the row updated is always the caller's own, from `SubjectFromContext`.

**`POST /v1/me/avatar/upload-url`**
Request body:
```json
{ "content_type": "image/jpeg" | "image/png" | "image/webp", "content_length": 123456 }
```
200 response (`AvatarUploadTicket`):
```json
{ "upload_url": "https://...", "public_url": "https://...", "expires_in": 300 }
```
`expires_in` is always `300`. The client `PUT`s the file bytes directly to `upload_url` (bypassing this API), then sends `public_url` back through `PATCH /me`'s `avatar_url` field once the upload succeeds.
400 `validation_failed` if `content_type` is outside the three allowed image types or `content_length` exceeds 5 MiB (5,242,880 bytes) or is <= 0.

**`GET /v1/usernames/suggest?name=<display name>`**
200 response (`UsernameSuggestion`):
```json
{ "username": "derived_handle", "alternates": ["alt1", "alt2", "alt3"] }
```
`username` is `NormalizeUsername(name)` if free, else a randomly-suffixed variant. `alternates` are three further free candidates derived from the same normalized base and are always a 3-element array (degrading to `[]` only if the generator itself fails).

**`GET /v1/usernames/available?username=<exact handle>`**
200 response (`UsernameAvailability`):
```json
{ "available": true, "alternates": [] }
```
or
```json
{ "available": false, "alternates": ["alt1", "alt2", "alt3"] }
```
`alternates` is `[]` (present, empty array -- never `null`) when available, and 3 free candidates when taken.
400 `validation_failed` if `username` itself fails `ValidateUsername` (wrong charset/length) before any availability check runs.

Both username routes are rate-limited to 20 requests/minute keyed by the authenticated subject (`KeyBySubject`), returning 429 `rate_limited` (from the shared `RateLimit` middleware, plan 01-06) beyond that.

### S3 avatar upload -- real-bucket verification still outstanding

`S3_ENDPOINT`/`S3_BUCKET`/`S3_ACCESS_KEY_ID`/`S3_SECRET_ACCESS_KEY`/`S3_PUBLIC_BASE_URL` hold non-functional placeholder values in this environment (deferred at plan 01-01's checkpoint). `PresignAvatarUpload`'s logic (type/size bounds, key scoping, expiry) is fully unit-tested against a stub `presigner` seam satisfying the same interface the real `s3.PresignClient` implements, so no test in this plan reaches a network endpoint. What remains unverified: that a client can actually `PUT` bytes to a URL this code presigns against a real S3-compatible bucket, and that the resulting object is fetchable at the returned `public_url`. This should be exercised once real credentials are provisioned, at plan 01-15's walkthrough checkpoint or later.

## Decisions Made

See `key-decisions` in frontmatter. Summary: `storage.Config` is a narrow package-local struct (not the full app `config.Config`) with identical S3 field names for a mechanical mapping in 01-13; `Register(rg)` adds routes only and leaves `RequireAuth`/`RequireVerified` wiring to 01-13, matching what 01-13-PLAN.md itself already specifies.

`REQUIREMENTS.md` is left unmodified for `ACCT-01`/`ACCT-03` -- but *not* for the 01-01/01-03 reason ("no HTTP endpoint exists yet"), which no longer applies: this plan ships a working, tested `GET /v1/me`, the backend half of ACCT-03. The actual reasons are (1) REQUIREMENTS.md's checkbox/Traceability lines are a shared-file conflict risk across this wave's four parallel worktrees, each of which could independently edit the same lines on merge, and (2) ACCT-03's UI half is still pending plan 01-14 and ACCT-01 still needs the sibling wave-4 plans merged. This is flagged for the orchestrator or a later phase-completion pass to reconcile once the full wave lands.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, gate-command bug] Task 1's DB-constraint cross-check verify command cannot match its own extracted pattern**
- **Found during:** Task 1 verify
- **Issue:** The plan's verify command extracts `PAT='^[a-z0-9_]{3,20}$'` from `username.go` via `grep -oE`, then checks it against `migrations/0001_init.up.sql` with a bare `grep -q "$PAT"` -- no `-F` and no `-E`. As a basic regular expression, `^` anchors to the start of the line, but the migration's constraint line is indented (`    constraint users_username_charset check (username ~ '^[a-z0-9_]{3,20}$')`), so the anchored pattern cannot match regardless of how correct `username.go` is. This is a bug in the gate command itself, not in the code it's checking.
- **Fix:** No code change (there is nothing to fix in `username.go` -- the pattern is already verbatim-identical to the constraint). Verified the real requirement with a fixed-string match instead: `grep -F "^[a-z0-9_]{3,20}$" migrations/0001_init.up.sql` succeeds, confirming `ValidateUsername`'s pattern is exactly the database check constraint's pattern.
- **Files modified:** none.
- **Verification:** `grep -F "$PAT" migrations/0001_init.up.sql` returns the matching constraint line; `go test ./internal/user -run TestUsername -v -count=1` passes 7/7.
- **Committed in:** `5d667bd` (Task 1 GREEN commit) -- no separate fix commit needed since no source changed.

---

**Total deviations:** 1 auto-fixed (1 blocking gate-command bug, no source-code fault)
**Impact on plan:** No scope creep, no code changed to work around the gate. The underlying acceptance criterion ("ValidateUsername uses the same pattern as the database check constraint") is genuinely satisfied and independently re-verified with a correct match.

## Issues Encountered

None beyond the gate-command bug above, resolved within Task 1.

## User Setup Required

None new. Real S3 credentials remain outstanding from plan 01-01's deferral (see "S3 avatar upload -- real-bucket verification still outstanding" above); no additional external service configuration is introduced by this plan.

## Next Phase Readiness

- Plan 01-13 (`cmd/api/main.go`, `internal/httpapi/server.go`) can construct `storage.Config` field-for-field from `config.Config`, call `storage.NewAvatarStore`, construct `httpapi.NewProfileHandler`/`httpapi.NewUsernameHandler`, and mount both on the nested `RequireAuth` + `RequireVerified` group per its own plan text -- no changes needed to this plan's files.
- Plan 01-12 (profile-setup screen) and plan 01-14 (profile view/edit screen) can code directly against the "Routes" contract above -- request/response JSON shapes, the 409 `suggestions` array, the 400 avatar-ticket rejection, and the `alternates`-never-null guarantee are all locked and tested.
- Real-bucket S3 verification is outstanding -- flagged for plan 01-15's walkthrough checkpoint or later, per plan 01-01's original deferral.
- `internal/middleware.RequireVerified` (plan 01-09) is assumed but not imported anywhere in this plan's files -- confirmed no compile-time or grep-gate dependency on it exists in this worktree.
- No blockers.

## Self-Check: PASSED

- FOUND: internal/user/username.go
- FOUND: internal/user/username_test.go
- FOUND: internal/storage/s3.go
- FOUND: internal/storage/s3_test.go
- FOUND: internal/httpapi/profile.go
- FOUND: internal/httpapi/profile_test.go
- FOUND: internal/httpapi/username.go
- FOUND: internal/httpapi/username_test.go
- FOUND: commit bd81e65
- FOUND: commit 5d667bd
- FOUND: commit 5a74d4d
- FOUND: commit 3873be1
- FOUND: commit 93dfbbd
- FOUND: commit d7785c9
- go build ./... -- PASS
- go vet ./... -- PASS
- go test ./internal/user ./internal/storage ./internal/httpapi -count=1 -- PASS (all 3 packages)
- go test ./... -count=1 -- PASS (all 7 packages, no regressions in auth/config/middleware/store/postgres)
- Task 1 acceptance criteria -- PASS (7/7 tests; DB-constraint cross-check re-verified with `-F`, see Deviations)
- Task 2 acceptance criteria -- PASS (7/7 tests, no network access; all grep gates pass)
- Task 3 acceptance criteria -- PASS (9/9 tests; all grep gates pass after fixing a doc-comment false-positive on the `json:"id"` gate)
- No route registered in this plan accepts a user identifier from the caller -- confirmed by grep gates and `TestProfile_PatchMe_IgnoresUserIDInBody`

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
