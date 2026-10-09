---
phase: 01-foundation-accounts
plan: 15
subsystem: testing
tags: [verification, uat, go-test, tsc, expo-export, design-scan]
requires:
  - phase: 01-foundation-accounts (plans 12, 13, 14, 16)
    provides: the screens, API wiring and profile UI this plan checks
provides:
  - Automated gate results for both stacks, green on 2026-10-09
  - A reconciled 01-VALIDATION.md map (11 rows, no TBD columns) and a walkthrough record that is still open
affects: [01-17, 01-19, 01-20, 01-21, 02-daily-roll]
tech-stack:
  added: []
  patterns: ["Design-constraint scans run as shell commands over app, components and lib with app/lab excluded"]
key-files:
  created: [components/brand/SpinningMark.tsx, components/brand/WheelBackground.tsx, components/brand/CategoryRow.tsx]
  modified: [.planning/phases/01-foundation-accounts/01-VALIDATION.md, .planning/phases/01-foundation-accounts/01-15-PLAN.md, app/(auth)/welcome.tsx, lib/auth/social.ts]
key-decisions:
  - "The raw hex scan also allows lib/theme/tokens.test-assert.ts (eight compile-time color pins from plan 01-02); the plan's scan command says so since 739bcfc"
  - "Welcome is one page shown on every signed-out launch (UI-SPEC revision 11), replacing the four marketing screens"
  - "Walkthrough steps 8 to 18 were replaced by 01-19 Task 4, walked in small groups because Claude cannot tap or type on the Simulator"
patterns-established:
  - "The emoji scan uses byte ranges with LC_ALL=C because the macOS grep has no -P; the full database suite runs with -p 1 so two packages never truncate the same tables"
requirements-completed: [ACCT-01, ACCT-03]  # plan requirement IDs, not yet signed off
coverage:
  - id: D1
    description: "Automated gates for both stacks and the design-constraint scans"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./... -p 1 on rndmroll_test && npx tsc --noEmit && npx expo export --platform ios, plus the em dash, raw hex, emoji, radius token, retired-file and light-lock scans (2026-10-09 run)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Device walkthrough and the developer's decision on planner-authored copy"
    requirement: ACCT-03
    verification: []
    human_judgment: true
    rationale: "No mobile test framework exists in Phase 1 (01-VALIDATION.md); the developer's walkthrough and approval are the check"
duration: not recorded
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 15: Phase Verification Summary

**Phase 1 gates are green on both stacks as of 2026-10-09; the device walkthrough and the developer's approval are still open.**

## Performance
- **Duration:** not recorded. **Started:** by 2026-09-21 (255ca31 came from the first walkthrough); first sweep by 2026-09-23 (1582a05); re-run 2026-09-30, 2026-10-03, 2026-10-07, 2026-10-09
- **Completed:** 2026-10-09 (approved)
- **Tasks:** 2 (Task 1 automated sweep: green; Task 2 device walkthrough: open). The plan creates no files; the commits below are fixes it prompted.

## Accomplishments
- Gates, 2026-10-09, all green: `go build` and `go vet` pass; `go test ./... -p 1` on `rndmroll_test` passes (8 packages with tests ok; cmd/api has no test files); `npx tsc --noEmit` exit 0; `npx expo export --platform ios` exit 0.
- Scans, 2026-10-09, clean: em dash, raw hex, emoji. The 24dp radius token is only in `PhotoPanel.tsx`, `ProfileForm.tsx` and `BirthdayPickerField.tsx` (plus the tokens assert file); the circular token only in `IconBadge.tsx` and `DialAvatar.tsx` (plus the tokens assert file). No retired file is present. The light lock appears 3 times in `app.json`.
- Go test counts, each from its own source, all 0 skips: 133 passes on 2026-09-30 (`01-15-SWEEP.md`), 158 after 01-17 (`ba57e39`), 254 after 01-19 Task 1 (`7ca2471`). The 2026-10-09 run has no recorded count.
- `01-VALIDATION.md` has 11 rows, all green, `nyquist_compliant: true`. Its Task ID, Plan and Wave columns were filled in by `1582a05`. No original row needed a new test; 01-19 added five rows (age, birthday privacy, body cap, public username checks, claim). Its approval line still reads "pending device walkthrough".
- Walkthrough Section A (Welcome, steps 1 to 7) was redone on 2026-10-04 after the Welcome rewrite. The developer confirmed the Welcome page, Get started to the method chooser, and relaunch back to Welcome. Claude checked Reduce Motion from simulator screenshots.
- Old steps 8 to 18 are now 01-19 Task 4. Passed with the developer: Group 1; Group 2 (twice, the second time with the date scroller and green notes); the duplicate-email message; photo upload on the main page; check 4 (sign-up, photo and bio on page 2, landing page); staying signed in. Not yet seen on the phone: the new Sign up and Log in buttons; the Continue button and "You're in." page; Skip for now; edit-and-save, Take photo message, wrong password message and the Log out sheet.

## Task Commits
1. **Task 1 sweep fix** - `1582a05` (30 em dashes out of source comments, none in UI copy; validation map filled in)
2. **Task 2 fix from the walkthrough** - `255ca31` (Google Sign-In import deferred to first use)
3. **Task 2 Welcome rewrite, which also changed steps 1 to 7** - `5610b03` (wheel logo, spin wording), `25973ef` (one Welcome page), `2f6e292` (Welcome always first)
4. **Related, not named for 01-15 in their messages** - `61855e2`, `a4fffa5` (expo-router first route, found by launching the dev client), `2038e1d` (gitignore native output)

**Plan metadata:** `27229c3` (checklist corrections, `01-15-PRETRACE.md`); scan lists and radius allow-list kept current by `739bcfc` (01-19) and `a0b7d98` (01-21)

## Files Created/Modified
See `key-files`. The Welcome files came from the rewrite in item 3; the planning files were updated with each later plan.

## Decisions Made
See `key-decisions`. No keep-or-change answers are recorded yet for the planner-authored strings (current list: `01-15-PRETRACE.md` section 5). Forgot password, the omission this plan asked about, is on the launch list as a 6-digit code flow (`PROJECT.md`, 2026-10-05).

## Deviations from Plan
- The raw hex command as first written failed on `lib/theme/tokens.test-assert.ts`; the scan now allows that file.
- Section F's six strings and both cover-screen resolutions are stale: the cover was replaced by Welcome, and most of those strings sit on deleted screens.
- One bullet in `01-15-PLAN.md` is stale: the radius acceptance criterion lists two files for the 24dp token and says four distinct files. The scan paragraph and this summary use the current three.

## Issues Encountered
- Cold launch of the dev client hit "Unmatched Route" while `tsc` and `expo export` stayed green. Only launching the app showed it (`61855e2`, `a4fffa5`), so routing changes need a cold launch. The Google Sign-In module also threw at load in Expo Go and on web and took down the whole auth stack (`255ca31`).

## User Setup Required
None for the sweep. The walkthrough needs the local API in test mode, MinIO test storage and the iOS Simulator (`.continue-here.md`, Infrastructure State).

## Next Phase Readiness
- To close this plan: the developer looks at the new Sign up and Log in buttons and runs the new-account test; answers the open questions (keep or remove the "Log in instead" link, phone-number sign-up, email code) and open decision (a), whether Apple and Google accounts also enter a birthday; says keep or change on the list of new strings; types "approved".
- After "approved": close ROADMAP and STATE, delete the throwaway test accounts by exact email (list in `HANDOFF.json`), declare `expo-file-system` in `package.json` (undeclared at HEAD), one close-out commit, no push.
- Phase 2 gate (from 01-19): the signed-in API group needs `RequireAuth` plus `RequireUser`; a route that needs a finished profile also checks `OnboardingComplete()`. The next free migration is 0003.
- Deferred to the launch list: check 5 (the details page for an unfinished Apple or Google account, until real Apple or Google sign-in exists), real Apple and Google sign-in (steps 19 and 20), universal links, Forgot password, email verification, real Terms and Privacy text (from the developer), custom domain, favicon, removing the AI tag.

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Phase: 01-foundation-accounts*
*Completed: 2026-10-09*
