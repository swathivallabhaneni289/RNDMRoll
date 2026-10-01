# 01-15 sweep record (working file, NOT the plan SUMMARY)

Written 2026-09-30 after a parallel verification run (workflow `wf_3d34fa34-850`, 9 agents, read-only; live tree, Metro, API and dev database untouched, verified afterwards). HEAD `a2006b5`.

This is raw material for `01-15-SUMMARY.md`. Do not create that file until the developer types "approved": GSD treats a SUMMARY's existence as "plan complete".

Raw agent output: `~/.claude/projects/-Users-swathivallabhaneni-code-RNDMRoll/ee8f8615-27d8-4e7a-9a64-f1271f7a179b/subagents/workflows/wf_3d34fa34-850/journal.jsonl`

## Task 1: gate results

| Gate | Command | Result |
|------|---------|--------|
| Go build | `go build ./...` | exit 0 |
| Go vet | `go vet ./...` | exit 0 |
| Go suite | `make test-all-integration` (`go test ./... -p 1 -count=1`, database `rndmroll_test`) | exit 0, 133 passed, 0 failed, 0 skipped (counts include subtests) |
| TypeScript | `npx tsc --noEmit` | exit 0, 0 errors (covers `lab/` and `app/lab/`) |
| Shippable bundle | `expo export --platform ios` in a `git archive HEAD` clone (no `app/lab`) | exit 0, about 6 s (warm cache), 1759 modules, Hermes 4.1 MB, 15 route files bundled, no lab route |
| Em dash | U+2014 over `app/ components/ lib/` | 0 hits |
| Raw hex | the plan's literal segment | exit 1 as written, see deviation. Amended segment: exit 0 |
| Radius token sets | `radius.lg`, `radius.full` | only the permitted files; no bare numeric `borderRadius` above 8 |
| Emoji as icons | Unicode-range scan | 0 hits |
| Retired artifacts | `StepDots.tsx`, `name`, `username`, `photo` routes | all absent |
| Light lock | `app.json` top level, ios, android | light in all three |
| Routes | nine UI-SPEC screens plus the email screen | all present (15 route files in total) |

Passed per package: auth 31, config 6, httpapi 49, mail 6, middleware 21, storage 7, store/postgres 6, user 7. `cmd/api` has no tests.

Per-task verification map (`01-VALIDATION.md`), all six rows pass with no skips:
1. `TestSignup_ValidBody_Returns201AndHashesPassword` (auth_test.go:94)
2. `TestLogin_CorrectCredentials_Returns200WithTokensAndUser` (:199), `TestLogin_WrongPassword_Returns401InvalidCredentials` (:227)
3. `TestRefresh_ValidToken_Returns200WithNewTokenPairDifferentFromPresented` (:326), `TestRefresh_ExpiredToken_Returns401TokenExpired` (:366)
4. `TestRequireVerified_UnverifiedAccountReturns403AndHandlerNeverRuns` (verified_test.go:118), `TestUnverifiedCannotReachProfile` (integration_test.go:423)
5. `TestProfile_GetMe_ReturnsCallerProfileWithOnboardingComplete` (profile_test.go:84), `TestProfile_GetMe_NoBearerTokenReturns401` (:124)
6. `TestProfile_PatchMe_UpdatesFieldsIndependently` (:187), `TestProfile_PatchMe_UsernameTakenReturns409WithAlternates` (:243)

### Deviation: raw-hex gate

The plan's literal segment excludes only `lib/theme/tokens.ts`, so it fails on `lib/theme/tokens.test-assert.ts`: 8 `satisfies '#RRGGBB'` compile-time pins created by plan 01-02 (`aa18d4f`). The file is imported nowhere and its header forbids silent edits. Default resolution, which the developer can override: allow-list that one file, i.e. `grep -v -E '^lib/theme/tokens(\.test-assert)?\.ts$'` (amended segment exits 0). No runtime hex literal exists outside `tokens.ts`.

## Live configuration facts (non-secret), checked 2026-09-30

- API process: `MAIL_DRIVER=log` (step 11 link prints to the API console), `APP_BASE_URL=http://localhost:8080`, `APPLE_BUNDLE_ID=com.rndmroll.app` (matches `app.json` `ios.bundleIdentifier`), `RESEND_API_KEY` empty (fine with the log driver).
- Google client IDs are `deferred-*` placeholders; the S3 endpoint and public base hosts are `deferred-*.example.com`. Deferred at plan 01-01, tracked by GitHub issue #2.
- `app.json` has no `@react-native-google-signin/google-signin` plugin entry and the generated `Info.plist` has no Google URL scheme, so iOS Google sign-in cannot complete even with real IDs until the plugin is configured (`iosUrlScheme`), a prebuild is run and a new dev client is built.

### Consequence for the walkthrough (decision pending: waive or provision)

Cannot pass as written: step 20 (Google); the photo part of step 15 (needs S3, and the camera also needs a physical device, not a simulator); the avatar change in step 18 (needs S3). Proposed: waive for Phase 1, record in the SUMMARY, follow up under issue #2.

## Static trace digest

37 findings across three areas. None refuted, several corrected (mis-cited lines, overstated impact). Nothing was run on a device.

### Look at these on the device, by step

| Step | Expect / check |
|------|----------------|
| 1 | Judge contrast after about 0.8 s. Tagline clears the clock. A brief 6dp side gap while the photo scales in (0.97 to 1 over 700 ms). |
| 2 | Light fill, dark text, 8dp corners. Press shrinks to 0.97 and springs back. |
| 3-6 | `01 / 03` as digits, body says "your wheel", swipe left, back gesture, chevron, dissolve into choose-method. |
| 7 | Watch the first 0.5 s of a relaunch for Welcome text or dark scrim bars before choose-method. Repeat several cold launches. Code path is real (D4), paint unverified. |
| 8 | Dot grid faint, choose-method only. On iOS the Apple row appears after an async check and shifts Google down. No phone option. |
| 9 | Address shown equals typed. Email is not trimmed, so a trailing space gives a generic error. |
| 10 | Expect greyed `Resend email` for 30 s with NO visible number (seconds only in the accessibility label). Differs from the plan wording. |
| 11 | Use the NEWEST link: Resend deletes older tokens and an old link shows a generic error. Keep the app open (warm) when opening it. Verified shows about 1 s, then Create your profile. |
| 13 | Username stays blank until you leave the Name field, then fills; status goes checking to idle, never "available". Clear the username under 3 characters to test whether Bio jumps. |
| 14 | Needs a real taken username (no reserved list): create one first. After typing over a taken result the taken text lingers about 400 ms. |
| 15 | No skip button, Finish works without an avatar. Photo needs S3 and will fail on the placeholder. Camera needs a physical device. |
| 16 | Relaunch lands in profile. Look for a blank white frame. Needs the API reachable (D1). |
| 17 | Only avatar, name, `@username`, bio. Clear the bio to see `Add a bio` with the pencil badge. |
| 18 | Four fields persist. Avatar depends on S3. Log out sheet text is exact. Destination is choose-method; watch the transit for a blank or Welcome frame (D4). |
| 19 | Finish Create your profile after the first Apple pass WITHOUT editing Name, log out, sign in with Apple again, read Name on Edit profile. Code protects the stored name (`oauth.go:189,192`). Use an Apple ID that never authorized the app, and an email not used in step 9. Needs an Apple ID signed in on the simulator. |
| 20 | Will fail on iOS (no URL scheme, placeholder IDs). Waive. |
| F | The six strings match code exactly. `Log in` is both heading and button in login mode. No Forgot password anywhere. Scrim top zone is computed from the safe-area inset; Get started routes to The Ritual (check on a notched device). |

### Extra planner-authored strings needing approval

Section F lists six strings. Checked against `01-UI-SPEC.md`, 20 further strings have no source there (only `Add profile photo` and `Resend email` do).

Create your profile (`app/(auth)/profile-setup.tsx`): `Enter your name.` / `Name must be 50 characters or fewer.` / `Use lowercase letters, numbers, and underscores, 3 to 20 characters.` / `Take Photo` / `Choose from Library` / `Cancel` / `We need camera access to take a profile photo. You can add one later from your profile.` / `We need photo library access to add a profile photo. You can add one later from your profile.` / `Couldn't upload your photo. Try again.`

Verify email (`app/(auth)/verify-email.tsx`): `Verified` / `We sent a verification link to {email}. Tap it to continue.` / `Something went wrong. Please try again.`

Edit profile (`app/(app)/profile/edit.tsx`): `3-20 lowercase letters, numbers, or underscores.` / `Name is required.` / `Change photo` / `Take photo` / `Choose from library` / `Camera access is needed to take a photo.` / `Photo library access is needed to choose a photo.` / `{n} characters left`

Also: the same username rule and the same photo actions are worded differently on the two screens (`Take Photo` vs `Take photo`), and the first-use iOS permission prompts are Expo defaults because the `expo-image-picker` plugin has no config (`app.json:22`).

### Defects found (proposed: record as follow-ups; none blocks the plan's acceptance criteria)

| ID | Sev | Where | Problem |
|----|-----|-------|---------|
| D1 | med-high | `lib/session/store.ts:139-142`, `:116-117` | Any refresh error, including a network error at launch, calls `doSignOut`, which deletes both tokens. An offline launch logs the user out permanently, against ACCT-01's "stays logged in". Verified by reading the code. Fix: sign out only on a definitive auth failure. |
| D2 | med | `profile-setup.tsx:283,242,274-275,182` | Finish is enabled during avatar upload and after a failed upload, and saves without the avatar while the preview still shows one. |
| D3 | med | `app/(auth)/_layout.tsx:66` | No `gestureEnabled: false` on profile-setup, so back reaches signup or intro screens while signed in, losing the entered state. |
| D4 | med | `app/index.tsx:40,54`; `app/(auth)/_layout.tsx:23,45-47`; `profile/index.tsx:33-37`; root `_layout.tsx:49` | Returning-user launch and logout pass through Welcome or a blank frame before choose-method (index ignores the intro flag). Paint unverified. A fix touches the fragile routing files, so it needs a real cold-launch check. |
| D5 | med | `internal/mail/mailer.go:63`; `verify-email.tsx:57-58,164-170` | Resend deletes older tokens; a superseded link shows a generic error. |
| D6 | med | `profile/edit.tsx:192-196` | Sends the picker `fileSize` as `content_length`; setup uses `blob.size`. A mismatch fails the presigned PUT even with real S3. |
| D7 | med | `internal/httpapi/oauth.go:158-164` | Social sign-in links to an existing unverified email account without marking it verified, so that user stays blocked (403 on PATCH /me). Only the verified-account case is tested (`oauth_test.go:154`). |
| D8 | low-med | `profile-setup.tsx:57,88-92` | Name prefill reads only the in-memory draft: Google users see a blank Name; a killed app loses it. |
| D9 | low | `lib/auth/social.ts:70-80` | Apple's one-shot name is lost if the first POST fails (POST runs before `setDraft`). |
| D10 | low | `profile-setup.tsx:210,234-237` | Camera path has no try/catch; on the simulator "Take Photo" likely throws. |
| D11 | low | various | Username rate limit fails silently (`:133-136`); stale status during debounce (`:139-153`); "taken" with zero chips (`username.go:89-92`); username case handled differently on the two screens; email not trimmed (`email.tsx:84`); whitespace-only bio stored (`profile/index.tsx:31`). |
| D12 | low | `oauth.go:169`; `oauth_apple.go:98-111` | Apple handler ignores `email_verified` and accepts an empty email. |
| D13 | low | `app/(auth)/_layout.tsx:46-47` | A cold-start verification link may lose its token (needs a device check). |
| D14 | low | various | Log out sheet lacks the card shadow (`profile/index.tsx:137`); chevron has no entrance (`AdvanceControl.tsx:45`); stale comment on the `initialRouteName` pin (`app/_layout.tsx:13-16`); ` -- ` in several comments; three em dashes in Go comments (`internal/user/model.go:46`, `internal/store/postgres/user_repo.go:53,183`), outside the scan scope. |
