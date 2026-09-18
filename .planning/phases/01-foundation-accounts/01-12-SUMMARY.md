---
phase: 01-foundation-accounts
plan: 12
subsystem: ui
tags: [react-native, expo-router, expo-image-picker, expo-image, username, avatar-upload, onboarding, keyboard-avoidance]

requires:
  - phase: 01-foundation-accounts (plan 04)
    provides: components/ui (TextField, PrimaryButton, IconBadge, AppText, Screen) and lib/theme/tokens.ts
  - phase: 01-foundation-accounts (plan 05)
    provides: root guard (app/_layout.tsx Stack.Protected on onboarding_complete, app/(auth)/_layout.tsx routing to /profile-setup for a half-onboarded user)
  - phase: 01-foundation-accounts (plan 07)
    provides: lib/onboarding/draft.ts (getDraft/clearDraft, suggestedName captured from a first Apple authorization)
  - phase: 01-foundation-accounts (plan 11)
    provides: GET/PATCH /me, GET /usernames/suggest, GET /usernames/available, POST /me/avatar/upload-url -- exact request/response shapes, the 409 suggestions-array-never-null contract
provides:
  - "app/(auth)/profile-setup.tsx: the D-05 (revised 2026-09-17) consolidated 'Create your profile' screen -- hero avatar with immediate upload, name field prefilled from the onboarding draft, auto-suggested/live-checked username with alternates, placeholder-only bio, single PATCH /me save"
affects: [01-15]

tech-stack:
  added: []
  patterns:
    - "A CTA that must stay pinned outside scrollable content but still keyboard-avoid wraps a ScrollView and a footer View as siblings inside one shared KeyboardAvoidingView, rather than using Screen's own scroll=true wrapper (Screen scroll=false used here instead, with the two-region layout built by hand)"
    - "A layout-stability requirement (no vertical jump across a multi-state status row) that the shared TextField component cannot express itself (its style is hardcoded internally, Omit<TextInputProps,'style'> blocks any external override) is built as an owning-screen-local fixed-minHeight sibling View below a 'bare' TextField, bypassing TextField's own built-in status/error rendering for that one field -- see Deviations"
    - "A stale-response token counter (checkTokenRef) guards a debounced network check against a slow, superseded response overwriting state a newer, faster check already set"

key-files:
  created:
    - app/(auth)/profile-setup.tsx
  modified: []

key-decisions:
  - "Username status/alternates rendering is built as profile-setup.tsx-owned custom JSX (a fixed minHeight:24 View plus a conditional chip-wrap row) rather than through TextField's own status/error props, because TextField renders 0dp of height in its idle state and this plan's file scope does not include components/ui/TextField.tsx (only app/(auth)/profile-setup.tsx is listed in files_modified). This is the only way to satisfy the plan's own explicitly named must_have truth ('the bio field does not shift vertically as the username status changes'), and is a reconciliation of Task 1's text ('this task establishes their position and the reserved height') with Task 2's more literal 'driven through the TextField status and error props' phrasing -- not a scope change, since no file outside profile-setup.tsx was touched."
  - "Avatar upload's content_length is read from the fetched blob's own .size, not the picker's asset.fileSize -- see Deviations for the self-caught fix commit."
  - "The username suggestion fetch fires at most once per screen lifetime (mount if a draft suggestedName exists, else the Name field's first blur), matching the plan's literal trigger; a later Name edit never re-fires it, and usernameEditedRef discards a late-arriving suggestion once the user has typed into the username field themselves."

patterns-established:
  - "Onboarding-screen submit handlers that can hit a save-time uniqueness conflict reuse the same live-check 'taken' rendering for the 409 response, rather than a separate error path -- the alternates array from the error body drives the same chip row the live check would have shown."

requirements-completed: []
# ACCT-01 (this plan's sole listed requirement) is left unmarked for the same
# reason plan 01-11 (same wave) left ACCT-01/ACCT-03 unmarked: REQUIREMENTS.md's
# checkbox/Traceability lines are a shared-file conflict risk across this
# wave's parallel worktrees, and ACCT-01 as a user-facing statement still
# needs the sibling wave plans (01-08/01-09/01-10, and this wave's 01-13/01-14)
# merged before it is genuinely true end-to-end. The orchestrator or a later
# phase-completion pass should reconcile REQUIREMENTS.md once the full wave lands.

coverage:
  - id: D1
    description: "Screen shell: two-region layout (scrollable form + CTA footer, both inside one KeyboardAvoidingView), 'Create your profile' Heading-role heading, 120dp hero avatar with camera/library picker and immediate upload to POST /me/avatar/upload-url, 'Optional' caption with no second skip CTA"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "npx tsc --noEmit (0 errors); grep gate for all 18 Task 1 acceptance criteria (heading/copy strings, 120dp, create-outline, surface=\"secondary\", inkAvatarPlaceholder, launchCameraAsync, launchImageLibraryAsync, avatar/upload-url, accessibilityLabel, KeyboardAvoidingView, role=\"heading\", zero role=\"display\", zero 'Skip for now', zero TextButton, getDraft usage) -- all pass"
        status: pass
      - kind: other
        ref: "npx expo export --platform ios (all nine auth routes, including profile-setup, bundle and export cleanly)"
        status: pass
    human_judgment: true
    rationale: "String/compile checks prove the required elements are present in the JSX, not that the picker flow, permission-denial handling, or upload-in-progress affordance behave correctly on a device or simulator -- no test or manual run exercised the actual camera/library picker or a real upload. A human must verify this at the plan 01-15 walkthrough."
  - id: D2
    description: "Name field: prefilled from the onboarding draft's suggestedName when present, validated on blur (required, 1-50 chars)"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "npx tsc --noEmit; grep -qE 'getDraft|useDraft' app/(auth)/profile-setup.tsx"
        status: pass
    human_judgment: true
    rationale: "No test exercises the blur-validation timing or the Apple-draft prefill path against a real draft value; grep only proves getDraft is referenced somewhere in the file."
  - id: D3
    description: "Username field: never rendered blank on the Apple-draft path (prefilled from GET /usernames/suggest on mount), five-state machine (idle/checking/available/taken/insert-time-conflict) with a 400ms debounce on GET /usernames/available, client-side ValidateUsername charset/length pre-check, up to three tappable alternate chips, a fixed 24dp reserved status row so the bio field never shifts across idle/checking/available"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "npx tsc --noEmit; grep gate for all 12 Task 2 acceptance criteria (taken-message string, 400ms, Checking/Available text, usernames/suggest, usernames/available, 409|username_taken, radius.sm, reloadUser, clearDraft, single api.patch call, no explicit app-group navigation) -- all pass"
        status: pass
    human_judgment: true
    rationale: "This is the plan's highest-risk deliverable and is entirely unverified by any test or manual run: the debounce timing, the stale-response token guard, the taken-state-to-available-state layout collapse, the alternate-chip tap re-check, and the reserved-24dp-row's actual visual stability all need a human to drive the screen. Additionally, the never-blank-field truth only holds on the Apple-signup path in this build -- see Deviations. Flagged for the plan 01-15 walkthrough."
  - id: D4
    description: "Bio field: placeholder-only ('Tell people what you're rolling for.'), never a default value, capped at 160 characters"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "grep -q \"Tell people what you're rolling for.\" app/(auth)/profile-setup.tsx; source inspection confirms bio's useState initializes to '' and onChangeText slices to BIO_MAX_LENGTH=160"
        status: pass
    human_judgment: false
  - id: D5
    description: "Single authoritative save: one PATCH /me carrying name+username always, bio only if non-empty, avatar_url only if the upload produced one (absent keys omitted, never null); on success reloadUser() then clearDraft(); a 409 username_taken response re-renders the taken state with the error body's fresh alternates instead of a generic error; no explicit navigation into the app group"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "grep -c 'api\\.patch' app/(auth)/profile-setup.tsx == 1; grep -q reloadUser; grep -q clearDraft; grep -q username_taken; ! grep -qE \"router\\.(replace|push)\\(['\\\"]/\\(app\\)\" -- all pass. See Deviations for the plan's own verify command's gate-command bug on this same check."
        status: pass
    human_judgment: true
    rationale: "The 409-recovery path, the nil-means-unchanged key-omission behavior, and the root-guard hand-off (no explicit navigation, onboarding_complete flips server-side) are unexercised by any test against a live backend. A human must confirm end-to-end at the plan 01-15 walkthrough, once 01-13 mounts these routes."

duration: "commit-to-commit span ~5 min across 3 commits (2 feat, 1 self-caught fix); upfront context-loading/read phase not separately timestamped"
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 12: Create Your Profile (Consolidated) Summary

**Single-screen "Create your profile" onboarding step (avatar + name + auto-suggested/live-checked username + bio) replacing the prior three-screen name/username/photo sequence, built against plan 01-11's exact GET/PATCH /me, usernames/suggest, usernames/available, and avatar/upload-url wire contract.**

## Performance

- **Duration:** ~5 min commit-to-commit (2 feat commits, one per plan task, plus a self-caught fix commit)
- **Started:** 2026-09-18T19:38:18+05:30 (first commit)
- **Completed:** 2026-09-18T19:43:25+05:30 (last commit)
- **Tasks:** 2 (plus one self-review fix)
- **Files modified:** 1 (1 created, 0 modified)

## Accomplishments

- `app/(auth)/profile-setup.tsx` Task 1: two-region layout (`Screen scroll={false}` > `KeyboardAvoidingView` > `ScrollView` form region + sibling footer `View` holding the CTA, so the CTA stays keyboard-reachable without scrolling away), 120dp hero avatar (`Pressable` + `IconBadge` edit affordance + `Optional` caption, no second skip CTA), camera/library picker via `expo-image-picker` with on-demand permission requests and immediate upload to `POST /me/avatar/upload-url`, name field prefilled from `getDraft().suggestedName` and validated on blur, placeholder-only bio capped at 160 characters.
- `app/(auth)/profile-setup.tsx` Task 2: username suggestion fetched once (mount or Name's first blur) from `GET /usernames/suggest`, a 400ms-debounced `GET /usernames/available` on edit guarded by a stale-response token, client-side `ValidateUsername` charset/length pre-check, up to three tappable alternate chips on a taken result, a fixed 24dp status row reserved across idle/checking/available so the bio field never jumps, a save-time 409 reusing the same taken rendering with fresh alternates from the error body, and the screen's one authoritative `PATCH /me` (name+username always, bio/avatar_url only when present, keys omitted rather than nulled) followed by `reloadUser()` and `clearDraft()` with no explicit app-group navigation.

## Task Commits

Each task was committed atomically:

1. **Task 1: Screen shell, hero avatar, name field, bio field, pinned CTA** - `ee389e1` (feat)
2. **Task 2: Username state machine, alternates, single authoritative save** - `fe19283` (feat)

**Self-caught fix (not part of either task's own scope, found during pre-summary review):** `0d5344b` (fix) -- see Deviations.

**Plan metadata:** this commit (`docs(01-12)`)

## Files Created/Modified

- `app/(auth)/profile-setup.tsx` - The consolidated "Create your profile" onboarding screen (D-05 revised 2026-09-17)

## Decisions Made

See `key-decisions` in frontmatter. Summary: the username status/alternates row is custom JSX owned by this screen rather than TextField's built-in status/error rendering (TextField is out of this plan's file scope and cannot reserve height in its idle state); avatar upload's `content_length` is read from the fetched blob rather than picker metadata; the suggestion fetch fires at most once per the plan's literal trigger, with a ref guard against a late response overwriting a user-typed value.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Avatar upload's `content_length` preferred picker metadata over the actual PUT body's size**
- **Found during:** Self-review before finalizing this summary (advisor consult), not surfaced by any of the plan's own automated gates
- **Issue:** The first implementation sent `content_length: asset.fileSize ?? blob.size` to `POST /me/avatar/upload-url`, but the value actually PUT to the presigned URL is the fetched `blob`, not the original picker asset. Per 01-11-SUMMARY.md, the size cap is bound into the presigned request itself, so a declared length that disagrees with the real body (e.g. because `quality: 0.8`/`allowsEditing: true` re-compressed the file) risks a mismatched or rejected upload.
- **Fix:** Changed to `content_length: blob.size` -- the same blob passed as the PUT body, guaranteed to match.
- **Files modified:** `app/(auth)/profile-setup.tsx`
- **Verification:** `npx tsc --noEmit` clean; both tasks' full grep gates re-run and still pass.
- **Committed in:** `0d5344b` (separate fix commit, not amended into either task commit)

**2. [Rule 3 - Blocking, gate-command bug] Task 2's own single-PATCH verify command cannot match its own call site**
- **Found during:** Self-review before finalizing this summary (advisor consult)
- **Issue:** Task 2's `<verify>` command counts occurrences of `patch('/me'` / `` patch(`/me `` / `patch("/me"` to assert at most one `PATCH /me` call. The actual call site is `api.patch<ApiUser>('/me', body)` -- the generic type parameter `<ApiUser>` sits between `patch` and `('/me'`, so none of the three literal patterns match and the grep count is `0`, not `1`. `[ "0" -le 1 ]` is true, so the gate prints PASS without ever matching the real call. This mirrors 01-11-SUMMARY.md's own precedent (its Task 1 DB-constraint cross-check had the same class of bug: a correct requirement, an incorrectly-written gate command).
- **Fix:** No code change -- the underlying requirement (exactly one `PATCH /me` call, absent-key omission) is genuinely satisfied. Verified with a corrected command instead: `grep -c 'api\.patch' app/(auth)/profile-setup.tsx` returns `1`.
- **Files modified:** none.
- **Verification:** `grep -c 'api\.patch' "app/(auth)/profile-setup.tsx"` == `1`.
- **Committed in:** No separate commit needed (no source changed); documented here per 01-11's precedent.

---

**Total deviations:** 2 (1 auto-fixed real bug, 1 gate-command bug with no source fault)
**Impact on plan:** The content_length fix is a genuine correctness improvement over the original implementation, caught before merge rather than at 01-15's real-bucket walkthrough. The gate-command note has zero code impact; the actual requirement was independently re-verified.

## Issues Encountered

**Acceptance criterion only fully satisfiable on one of two onboarding paths.** Task 2's acceptance criteria list "The username field is never rendered with an empty initial value" as a plain criterion, but it is absent from the task's own `<automated>` verify block, so nothing in this plan's tooling actually exercises it. On inspection: the criterion holds unconditionally on the Apple-signup path (`getDraft().suggestedName` present -> suggestion fetched on mount) but **not** on the email-signup path (D-03's default), where the draft carries no `suggestedName` and the field is genuinely empty until the Name field's first blur fires the suggestion fetch. This is not a bug in this implementation -- it is inherent to the trigger Task 2's own action text specifies verbatim ("Fire it once on mount when `getDraft().suggestedName` is present, and otherwise on the first blur of the Name field with a non-empty value"), and there is no way to derive a username suggestion from a name the user has not yet entered. Flagged here rather than silently passed over, per the plan's own hard gate on acceptance criteria; a human should confirm at the plan 01-15 walkthrough whether a blank-until-first-blur username field on the email path is acceptable or needs a follow-up decision (e.g. suggesting from the email's local-part instead).

**Bio field does not render as a fixed three-line box.** UI-SPEC describes the bio field as "roughly three lines tall." `TextField`'s own style is hardcoded internally (`Omit<TextInputProps, 'style'>` blocks any external override) with only `minHeight: minTouchTarget` (44dp), no explicit multiline height. `multiline` + `numberOfLines={3}` were passed, which gives Android a sizing hint and lets the box auto-grow with content on both platforms, but neither platform is forced to a fixed three-line rest height from outside `TextField`. Not fixable within this plan's file scope (`components/ui/TextField.tsx` is not in `files_modified`). Cosmetic gap only; functionally the field is multiline, placeholder-only, and capped at 160 characters as required.

**`asset.mimeType` could theoretically be `image/heic` on iOS.** 01-11's avatar endpoint allow-lists only `image/jpeg`/`image/png`/`image/webp`. `allowsEditing: true` typically normalizes a camera/library selection to JPEG before this code sees it, so this is believed unreachable in practice, but it is not explicitly guarded against -- an unconverted HEIC selection would surface the generic "Something went wrong" fallback (via `ApiError.userMessage`'s fallback for an unmapped `validation_failed`) rather than a specific message. Noted for awareness, not treated as a blocking gap.

## User Setup Required

None - no external service configuration required by this plan. (Real S3 credentials remain outstanding from plan 01-01's deferral, unrelated to this plan's own scope.)

## Next Phase Readiness

- This screen is not reachable by any real client yet: `POST /v1/me/avatar/upload-url`, `GET /v1/usernames/suggest`, `GET /v1/usernames/available`, and `PATCH /v1/me` are all mounted by plan 01-13 (`cmd/api/main.go`), a sibling wave-5 worktree with zero file overlap with this plan.
- `app/(auth)/_layout.tsx` (already merged, plan 01-05) already references `profile-setup` as a registered route and redirects a half-onboarded authenticated user there -- no routing changes were needed in this plan.
- Every string this screen renders is verbatim from UI-SPEC revision 9's Copywriting Contract; this plan introduces no planner-authored copy, so plan 01-15's UAT checkpoint needs to review only the plan 01-07 and plan 01-14 strings, per this plan's own `<output>` instruction.
- Flagged for a human at plan 01-15's walkthrough: (1) the email-signup path's blank-until-first-blur username field (see Issues Encountered), (2) full picker/upload/debounce/layout-stability behavior on a device, (3) the 409-conflict recovery path against a live backend, (4) real-bucket S3 upload verification (already outstanding from plan 01-01/01-11).
- No blockers for sibling plans 01-13/01-14, which have zero file overlap with `app/(auth)/profile-setup.tsx`.

## Self-Check: PASSED

- FOUND: app/(auth)/profile-setup.tsx
- FOUND: commit ee389e1 (Task 1)
- FOUND: commit fe19283 (Task 2)
- FOUND: commit 0d5344b (self-caught fix)
- npx tsc --noEmit -- PASS (0 errors)
- npx expo export --platform ios -- PASS (all nine auth routes bundle, including profile-setup)
- Task 1 acceptance criteria -- PASS (18/18 grep gates, re-verified after Task 2 and after the fix commit)
- Task 2 acceptance criteria -- PASS (12/12 grep gates; single-PATCH gate re-verified with a corrected command, see Deviations)
- No raw hex color literal in app/(auth)/profile-setup.tsx -- confirmed via grep
- No explicit navigation into the app group anywhere in the file -- confirmed via grep

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
