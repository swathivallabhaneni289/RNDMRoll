---
phase: 01-foundation-accounts
plan: 07
subsystem: auth
tags: [expo-router, expo-apple-authentication, google-signin, expo-linking, react-native, design-tokens]

# Dependency graph
requires:
  - phase: 01-foundation-accounts (plan 01-04)
    provides: "UI primitives this plan's three screens compose: Screen, AppText/TextLink, MethodButton, TextField, PrimaryButton, TextButton, IconBadge, BrandMark, all hard-capped at radius.md"
  - phase: 01-foundation-accounts (plan 01-05)
    provides: "lib/api/client.ts (api.get/post/patch, ApiError with userMessage), lib/api/types.ts (AuthResult, ApiErrorCode), lib/session/store.ts (useSession/signIn), and the app/(auth)/_layout.tsx route declarations (choose-method, email, verify-email) this plan fills in"
provides:
  - "lib/onboarding/draft.ts: in-memory, non-credential onboarding draft store (OnboardingDraft, setDraft, getDraft, clearDraft, useDraft)"
  - "lib/auth/social.ts: isAppleSignInAvailable, signInWithApple, signInWithGoogle, SIGN_IN_CANCELED sentinel — both sign-in functions post only the raw provider token to the backend oauth endpoints"
  - "app/(auth)/choose-method.tsx: D-05 step 1 method chooser (email/Apple/Google), the phase's one Brand Mark + dot-grid texture screen"
  - "app/(auth)/email.tsx: combined email signup/login screen — the one screen in this phase without an approved UI-SPEC entry; carries planner-authored copy flagged below for the plan 01-15 UAT checkpoint"
  - "app/(auth)/verify-email.tsx: D-04 verification waiting screen with a 30s resend cooldown and dual-path deep-link handling (live + cold-start)"
affects: [01-12 (profile-setup prefills its name field from draft.suggestedName), "01-08 through 01-11 (Go backend auth/oauth/signup/login/verify-email endpoints this client already assumes)", "01-15 (UAT checkpoint must review email.tsx's and verify-email.tsx's planner-authored copy)"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SIGN_IN_CANCELED sentinel return value (not a thrown error) distinguishes a user cancel from a real failure, matching the installed @react-native-google-signin/google-signin v16 API, which resolves signIn() with a typed {type:'success'|'cancelled'} response rather than throwing SIGN_IN_CANCELLED"
    - "Module-scoped pub/sub store with a useDraft() subscriber hook (lib/onboarding/draft.ts), for state written from outside React (lib/auth/social.ts, a plain module) that a screen still needs to re-render on"
    - "Key-bump remount + autoFocus for programmatic focus-jump-on-validation-error (email.tsx), since TextField (01-04) is a plain function component, not ref-forwarding, so an imperative .focus() call isn't reachable from a parent screen"
    - "accessible wrapper View carrying a live-updating accessibilityLabel around a TextButton (verify-email.tsx's resend control), since TextButton (01-04) has no accessibilityLabel prop of its own"

key-files:
  created:
    - lib/onboarding/draft.ts
    - lib/auth/social.ts
    - "app/(auth)/choose-method.tsx"
    - "app/(auth)/email.tsx"
    - "app/(auth)/verify-email.tsx"
  modified: []

key-decisions:
  - "No new npm dependency: expo-apple-authentication, @react-native-google-signin/google-signin, and expo-linking were already present in package.json from plans 01-01/01-02. Re-verified all three against their registry repository.url anyway per this dispatch's instruction (see Self-Check) even though nothing was installed: all three resolve to their expected repos (github.com/expo/expo for the first and third, github.com/react-native-google-signin/google-signin for the second)."
  - "signInWithGoogle() detects a cancel via the installed library's typed response (response.type === 'cancelled'), not a caught SIGN_IN_CANCELLED error. Checked node_modules/@react-native-google-signin/google-signin's own .d.ts files directly (GoogleSignin.d.ts, types.d.ts): this version's signIn() resolves a SignInResponse discriminated union (SignInSuccessResponse | CancelledResponse) rather than throwing on cancel. The plan's prose described the older throw-based pattern; using the actual installed API is more correct and still satisfies every acceptance criterion (the SIGN_IN_CANCELED sentinel name itself contains the required CANCELED/CANCELLED substring)."
  - "email.tsx's 401 invalid_credentials path renders ApiError.userMessage (falls through to the existing generic fallback string already wired in lib/api/client.ts from plan 01-05) rather than inventing a new specific message. This is deliberate, not an oversight: the plan's own success_criteria list of planner-authored strings for this screen doesn't include one, and the server's whole point in returning a single undifferentiated code for both wrong-password and unknown-account is that the UI shouldn't say anything more specific either — inventing a more precise-sounding message risked re-introducing the exact distinction the backend is refusing to make."
  - "email.tsx's 409 email_taken path hardcodes the literal contract string directly in the file (rather than only delegating to ApiError.userMessage, which already maps this code to the same string) because this plan's own Task 3 verify block greps for that literal substring inside email.tsx itself, not just inside lib/api/client.ts. Same pattern 01-04 used for StepProgress/TextField's direct token references."
  - "verify-email.tsx guards against a cold-start deep link landing on an empty draft (see Deviations below) — a fix applied after an advisor review, committed separately (6cd3f1d) before this summary."

patterns-established:
  - "Screens read/write lib/onboarding/draft.ts only through setDraft/getDraft/useDraft, never a direct module-level mutation — keeps the pub/sub notification path the sole write path."
  - "Any screen introducing copy with no Copywriting Contract entry must be built from already-approved tokens/primitives only and must enumerate every new string in that plan's SUMMARY.md for the UAT checkpoint, not just the strings the plan's own prose happened to call out by name."

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "Onboarding draft store (in-memory, non-credential) and native Apple/Google sign-in wrappers that post only the raw provider token"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; Task 1 acceptance_criteria/verify block in 01-07-PLAN.md (identity_token/id_token/isAvailableAsync/CANCELED literals, no expo-secure-store import, suggestedName present)"
        status: pass
      - kind: other
        ref: "must_haves.artifacts (social.ts exports signInWithApple/signInWithGoogle/isAppleSignInAvailable) and key_links (social.ts -> auth/oauth) from 01-07-PLAN.md frontmatter"
        status: pass
    human_judgment: true
    rationale: "No live backend or real device exists in this environment to exercise an actual Apple/Google round trip, a genuine first-vs-subsequent Apple authorization, or GoogleSignin.configure() against real client IDs (EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS/WEB are unset here) — static checks confirm the wire shape and cancel-handling structure only."
  - id: D2
    description: "Choose-method entry screen: three D-01 methods, Apple gated to iOS, no phone option, the phase's one Brand Mark + texture moment, all-three-disabled-during-any-round-trip contract"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; Task 2 acceptance_criteria/verify block in 01-07-PLAN.md; must_haves.artifacts (contains BrandMark)"
        status: pass
    human_judgment: true
    rationale: "Static checks confirm the literal labels, platform gate, and disabled-state wiring; actual on-device Apple-availability detection and the visual composition need a human/device pass, deferred to plan 01-15's UAT checkpoint."
  - id: D3
    description: "Email signup/login screen: blur validation, submit-time focus jump, verbatim email_taken copy, single generic invalid_credentials message, email_not_verified redirect to the waiting screen"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; Task 3 acceptance_criteria/verify block in 01-07-PLAN.md; must_haves.artifacts (contains secureTextEntry)"
        status: pass
    human_judgment: true
    rationale: "No live backend exists yet (lands in plans 01-08 through 01-11), so the 201/409/401/403 branches are verified by code inspection against the wire contract only, not exercised end to end. The screen's copy is also explicitly planner-authored and flagged for human review at plan 01-15, independent of correctness."
  - id: D4
    description: "Verify-email waiting screen: IconBadge + Display heading, 30s resend cooldown, live deep-link handling plus cold-start getInitialURL handling with token dedup, brief Verified confirmation before automatic advance"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; Task 3 acceptance_criteria/verify block in 01-07-PLAN.md; must_haves.artifacts (contains 'Check your email') and key_links (verify-email.tsx -> signIn)"
        status: pass
    human_judgment: true
    rationale: "No live backend or real device exists to fire an actual rndmroll://verify-email link (warm or cold) against a real token, so the dedup-by-token guard, the 900ms confirmation-then-advance timing, and the cooldown countdown are verified by code inspection only. Needs a real device/backend run at plan 01-15."

duration: ~2h (not separately instrumented; includes a 600s watchdog stall mid-session, unrelated to task work — resumed cleanly from the last completed commit per the coordinator's own diagnosis)
completed: 2026-09-18
status: complete
---

# Phase 01-foundation-accounts, Plan 07: D-05 Steps 1-2 (Method Chooser, Email, Verify-Email) Summary

**Choose-method entry screen, a combined email signup/login screen (planner-authored, no prior UI-SPEC entry), and the verify-email waiting screen with a 30s resend cooldown and dual-path (live + cold-start) deep-link handling, plus the onboarding draft store and native Apple/Google sign-in wrappers they all depend on.**

## Performance

- **Duration:** ~2h (not separately instrumented at task granularity; includes an unrelated 600s watchdog stall mid-session)
- **Completed:** 2026-09-18
- **Tasks:** 3 (all `type="auto"`, no checkpoints — confirmed against the plan's own frontmatter, `autonomous: true`)
- **Files modified:** 5 created, 0 modified (plus one same-plan follow-up fix to a file this plan itself created)

## Accomplishments

- `lib/onboarding/draft.ts` and `lib/auth/social.ts`: an in-memory, non-credential onboarding draft store with a subscriber hook, plus native Apple/Google sign-in wrappers that post only the raw `identity_token`/`id_token` to the backend oauth endpoints (T-01-ONB-01's mitigation) and return a `SIGN_IN_CANCELED` sentinel so callers can tell a deliberate cancel apart from a real failure.
- `app/(auth)/choose-method.tsx`: D-05 step 1, all three D-01 methods (Apple iOS-gated per D-02, no phone option per D-03), the phase's one Brand Mark + dot-grid texture screen, all three buttons disabled during any one social round-trip.
- `app/(auth)/email.tsx`: the one screen in this phase without an approved UI-SPEC entry, built from already-approved tokens/primitives, with blur-time validation, a focus-jump-on-submit workaround for `TextField`'s lack of ref-forwarding, and the exact contract copy for a duplicate-email 409.
- `app/(auth)/verify-email.tsx`: D-04's verification gate, a 30s resend cooldown, and deep-link handling for both a live `rndmroll://verify-email` return and a cold start, deduped by token so a link that fires both paths is only verified once — plus a follow-up guard (see Deviations) so a cold-start launch with no in-memory draft doesn't dead-end on an empty address and a no-op resend.
- Verified `must_haves.artifacts` and `must_haves.key_links` from the plan's own frontmatter directly (not just the per-task `<verify>` blocks) — all pass; see Self-Check.

## Task Commits

Each task was committed atomically:

1. **Task 1: Onboarding draft store and native social sign-in wrappers** - `b54c7ea` (feat)
2. **Task 2: Choose-method entry screen** - `1260113` (feat)
3. **Task 3: Email signup/login screen, and the verification waiting screen** - `ef1cf0c` (feat)

**Follow-up fix (same plan, post-advisor-review):** `6cd3f1d` (fix) — see Deviations below.

_No separate plan-metadata commit: worktree-mode dispatch, STATE.md/ROADMAP.md excluded per orchestrator instructions; this SUMMARY.md is committed separately below._

## Files Created/Modified

- `lib/onboarding/draft.ts` - `OnboardingDraft { email?, suggestedName?, provider? }`, `setDraft`, `getDraft`, `clearDraft`, `useDraft()` — module-scoped, in-memory, never imports `expo-secure-store`
- `lib/auth/social.ts` - `isAppleSignInAvailable()`, `signInWithApple()`, `signInWithGoogle()`, `SIGN_IN_CANCELED` sentinel, `SocialSignInResult` type — posts only `identity_token`/`id_token` (plus untrusted `full_name` on Apple) to `/auth/oauth/apple` and `/auth/oauth/google`
- `app/(auth)/choose-method.tsx` - `<Screen texture>`, `BrandMark size={64}`, `RNDMRoll` Display wordmark, three `MethodButton`s, Apple gated on `Platform.OS === 'ios'` + `isAppleSignInAvailable()`
- `app/(auth)/email.tsx` - one screen, `signup`/`login` toggle, two `TextField`s, blur validation, submit-time focus jump via key-bump remount, `Create account`/`Log in` CTA, `Log in instead`/`Sign up instead` toggle link
- `app/(auth)/verify-email.tsx` - `IconBadge`, `Check your email` (Display), 30s resend cooldown, `Linking.addEventListener('url', ...)` plus `Linking.getInitialURL()` deep-link handling deduped by token, brief `Verified` confirmation before `signIn()`

## Decisions Made

See `key-decisions` in frontmatter above. Summary:
1. No new dependency — all three native libraries this plan uses were already installed; re-verified their registry `repository.url` anyway (see Self-Check).
2. `signInWithGoogle()` follows the actually-installed library's typed cancel response instead of the plan's described throw-based pattern (verified against the package's own `.d.ts` files).
3. `invalid_credentials` deliberately reuses the existing generic fallback message rather than inventing a new one, to avoid leaking the wrong-password/unknown-account distinction the server refuses to make.
4. `email_taken`'s exact contract string is hardcoded directly in `email.tsx` (not only reached indirectly via `ApiError.userMessage`) so the plan's own literal-substring verify check passes.
5. Resend's live-updating `accessibilityLabel` is delivered via an `accessible` wrapper `View` around `TextButton`, since `TextButton` (01-04) exposes no `accessibilityLabel` prop of its own.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `signInWithGoogle()`'s cancel path used a caught error in the plan's prose; the installed library actually returns a typed response**
- **Found during:** Task 1, before writing `signInWithGoogle()`
- **Issue:** The plan's action text says "Handle `SIGN_IN_CANCELLED` the same way as the Apple cancel path" (implying a caught error code). Checked `node_modules/@react-native-google-signin/google-signin`'s own `.d.ts` files directly: this installed version (`^16.1.5`) resolves `GoogleSignin.signIn()` with a discriminated union (`SignInSuccessResponse | CancelledResponse`, `{type:'success'|'cancelled'}`), not a thrown error.
- **Fix:** Check `response.type === 'cancelled'` and return the shared `SIGN_IN_CANCELED` sentinel, matching the actual API rather than the assumed one.
- **Files modified:** `lib/auth/social.ts`
- **Verification:** `npx tsc --noEmit` passes; Task 1's `grep -qE 'CANCELED|CANCELLED'` check still passes since the sentinel's own name contains the required substring.
- **Committed in:** `b54c7ea`

**2. [Rule 1 - Bug] `TextField` has no ref-forwarding, so "focus jumps to the first invalid field on submit" (UI-SPEC form validation timing) isn't reachable via a normal `ref`**
- **Found during:** Task 3, writing `email.tsx`'s submit handler
- **Issue:** `components/ui/TextField.tsx` (from plan 01-04, out of this plan's `files_modified` scope) is a plain function component, not wrapped in `React.forwardRef`. Passing `ref={...}` to it would warn at runtime and never actually focus the underlying `TextInput`.
- **Fix:** Track an `attemptId` that increments on each submit and pass it as each `TextField`'s `key`, plus an `autoFocusField` state read into each field's `autoFocus` prop. Bumping `attemptId` on a failed submit remounts the invalid field fresh with `autoFocus` set, which moves the keyboard cursor there without needing `TextField` to expose a ref. Typed values survive the remount (they're parent-controlled state); only `TextField`'s internal `focused` UI flag resets, which is harmless.
- **Files modified:** `app/(auth)/email.tsx`
- **Verification:** `npx tsc --noEmit` passes; logic inspected against React Native's documented `autoFocus`-on-mount behavior. Not exercised on a real device/simulator in this environment — flagged for the plan 01-15 UAT pass.
- **Committed in:** `ef1cf0c`

**3. [Rule 1 - Bug, found via advisor review] Cold-start deep link lands on an empty onboarding draft, producing an empty-address sentence and a silently no-op Resend**
- **Found during:** post-Task-3 advisor review, before writing this summary
- **Issue:** `lib/onboarding/draft.ts` is deliberately in-memory only (see its own header comment — never AsyncStorage/SecureStore-backed). The plan explicitly requires handling a cold start via `Linking.getInitialURL()` (a fresh process launch). On that path `draft.email` is `undefined`, so `verify-email.tsx` was rendering "We sent a verification link to . Tap it to continue." (a hole where the address belongs) and, worse, a tap on "Resend email" during a `token_expired`/`token_consumed` state would `POST` with an empty `email` and silently do nothing — exactly the dead-end that inline message exists to prevent per the plan's own wording ("rather than dead-ending").
- **Fix:** Only render the address paragraph when `email.length > 0`; disable Resend whenever `email.length === 0` (added to the existing `cooldown > 0 || resending` condition). No new copy — reuses the existing literal template and `TextButton` wiring; the `Tap it to continue.` literal stays in source (conditionally rendered, not removed) so the plan's verify grep is unaffected.
- **Files modified:** `app/(auth)/verify-email.tsx`
- **Verification:** `npx tsc --noEmit` passes; Task 3's full verify block re-run and still passes (the literal strings are in source regardless of the runtime condition).
- **Committed in:** `6cd3f1d` (separate commit, after the three task commits, before this summary)

---

**Total deviations:** 3 auto-fixed (2 bugs found during execution, 1 bug found via advisor review before completion). **Impact on plan:** All three were necessary for correctness; none change any exported function signature or component prop contract from what the plan specifies. No scope creep — deviation 2 stayed inside `email.tsx` rather than modifying the shared `TextField.tsx`, and deviation 3 stayed inside the file this plan itself created.

## Issues Encountered

- **Session interruption (infrastructure, not a quality issue):** this session hit a 600s watchdog stall partway through orientation (after Task 1's commit `b54c7ea` had already landed). Resumed per the coordinator's explicit diagnosis: re-ran the HEAD assertion (three separate commands, since the combined script form is rejected by this environment's worktree path-safety checker), confirmed `b54c7ea` was intact and `git status`/`npx tsc --noEmit` were clean, then continued with Tasks 2-3 without redoing Task 1.
- **Worktree `node_modules/` absent on start**, same precedent as plans 01-04/01-05 (git worktrees don't share gitignored directories with the checkout they're created from). Ran `npm ci` (not `npm install`) to restore it exactly from the existing, unmodified `package-lock.json`; confirmed `git status --porcelain package-lock.json` was empty immediately after — no lockfile drift. Recreated the untracked `expo-env.d.ts` (same one-line content and precedent as 01-02/01-05) so `tsc` resolves `process.env.EXPO_PUBLIC_*`; left it untracked, never staged.
- **Complex/compound bash commands are rejected in this worktree** ("command is too complex to verify it stays inside the worktree"). Every multi-line `&&`/`if`-chained command in this session (including the mandated HEAD assertion script) had to be split into separate single-purpose `Bash` calls. Noted here in case it affects how future plans in this same environment are dispatched.

## User Setup Required

None required to build. One item worth a human's attention before this becomes load-bearing:

- **`GoogleSignin.configure({ iosClientId, webClientId })` runs once at module load** (per the plan's explicit "once at module load" instruction), reading `EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS`/`EXPO_PUBLIC_GOOGLE_CLIENT_ID_WEB`. Neither is set in this environment (no `.env`/`.env.example` exists yet, and this dispatch is explicitly blocked from writing or reading either), so `configure()` currently runs with both values `undefined`. This module loads as soon as `choose-method.tsx` (which imports it transitively is not the case — only screens that import `lib/auth/social.ts` load it; currently that's `choose-method.tsx`) is bundled. This is unverified against a real native module in this environment — if the native module throws on `configure()` with undefined client IDs (rather than no-op-ing), that would take out `choose-method` at import time. Flagging as the top item to exercise on a real device/simulator at plan 01-15, before or alongside provisioning the two env keys.

## Next Phase Readiness

- All three D-05 sequence screens this plan owns (`choose-method`, `email`, `verify-email`) are wired into `app/(auth)/_layout.tsx`'s existing route declarations (from plan 01-05) and typecheck/export cleanly; nothing downstream is blocked on file existence.
- Plan 01-12 (Create your profile) can read `lib/onboarding/draft.ts`'s `suggestedName`/`email`/`provider` fields directly — the store's shape (`OnboardingDraft`) is stable and matches the plan's own declared artifact.
- **Planner-authored copy pending human review at plan 01-15's UAT checkpoint** (this plan's own `<output>` instruction: list every planner-authored string on the email screen; extended here to verify-email.tsx's two new inline messages, found beyond the plan's own enumerated four):
  - `email.tsx` (already flagged by the plan itself): `"Sign up with email"`, `"Log in"`, `"At least 8 characters."`, `"Create account"`
  - `email.tsx` (found during execution, not in the plan's own list — labels on the two form fields): `"Email"`, `"Password"`
  - `email.tsx` (found during execution — field-level validation errors, unavoidable for a working form since neither UI-SPEC nor the plan gives literal text): `"Enter a valid email address."`, `"Password must be at least 8 characters."`, `"Enter your password."`
  - `verify-email.tsx` (found during execution — the plan's Task 3 action text requires "an inline message that directs the user to the resend button rather than dead-ending" for `token_expired`/`token_consumed`, but gives no literal wording; note this sits in slight tension with the plan's `success_criteria` line that scopes planner-authored copy to "the email screen only" — flagging that tension explicitly rather than silently resolving it): `"That link expired. Send a new one below."`, `"That link was already used. Send a new one below."`
  - `verify-email.tsx` (screen-reader-only, not visible copy, lower priority): the live-updating `accessibilityLabel` `"Resend email, available in N seconds"`
- Not yet built: the Go backend endpoints this client assumes (`/v1/auth/signup`, `/v1/auth/login`, `/v1/auth/verify-email`, `/v1/auth/verify-email/resend`, `/v1/auth/oauth/apple`, `/v1/auth/oauth/google`) — land in plans 01-08 through 01-11. Nothing in this plan can be exercised end-to-end against a real backend until those land, matching the same caveat 01-05's SUMMARY recorded for its own client/session layer.
- Not yet built/verified: any real device or simulator run (Apple/Google sign-in, a real deep link, font/session gating on a live screen) — this environment has no simulator. Every `human_judgment: true` coverage entry above defers to plan 01-15.

## Self-Check

- All 5 `key-files.created` verified present on disk.
- `git log --oneline --all --grep="01-07"` and direct hash lookup both confirm all four commits (`b54c7ea`, `1260113`, `ef1cf0c`, `6cd3f1d`) are on this worktree's branch.
- `npx tsc --noEmit` re-run clean (exit 0) after all four commits, including the post-review fix.
- All three tasks' `<acceptance_criteria>`/`<verify>` blocks re-run verbatim from `01-07-PLAN.md` after the fix: Task 1 PASS, Task 2 PASS, Task 3 PASS.
- Plan-level `<verification>` re-run: `npx tsc --noEmit` clean; `grep -rEn '#[0-9A-Fa-f]{3,8}'` across `app/(auth)/`, `lib/onboarding/draft.ts`, `lib/auth/social.ts` found zero raw hex literals; `npx expo export --platform ios` completed cleanly (confirmed all routes, including the two newly-added screens beyond `choose-method`, resolve).
- Plan frontmatter `must_haves.artifacts` re-checked directly (not just via the per-task verify blocks): `choose-method.tsx` contains `BrandMark`, `email.tsx` contains `secureTextEntry`, `verify-email.tsx` contains `Check your email`, `social.ts` exports `signInWithApple`/`signInWithGoogle`/`isAppleSignInAvailable` — all pass.
- Plan frontmatter `must_haves.key_links` re-checked directly: `social.ts` contains the `auth/oauth` pattern, `verify-email.tsx` contains the `signIn` pattern — both pass.
- `npm view` re-run for all three native libraries this plan uses for the first time (none newly installed): `expo-apple-authentication` and `expo-linking` -> `git+https://github.com/expo/expo.git`; `@react-native-google-signin/google-signin` -> `git+https://github.com/react-native-google-signin/google-signin.git` — all match RESEARCH.md's Standard Stack citations.
- `git status --porcelain` clean except the expected untracked, gitignore-precedent `expo-env.d.ts` (not staged, matching plans 01-02/01-05's own handling of the same file).
- `package.json`/`package-lock.json` diff against the plan's base commit (`0818321`) is empty — no dependency changes.

## Self-Check: PASSED

---
*Phase: 01-foundation-accounts*
*Plan: 07*
*Completed: 2026-09-18*
