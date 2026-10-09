---
phase: 01-foundation-accounts
plan: 19
subsystem: auth
tags: [signup, birthday, age-gate, migrations, gin, pgx, expo-router, profile-form]
requires:
  - phase: 01-foundation-accounts (plans 12, 14, 17)
    provides: the profile screens and the hardened session and claim code this plan reworks
provides:
  - POST /v1/auth/signup makes a complete account in one request and signs it in
  - A private birthday (migration 0002) with a 13+ rule on the server, and a RequireUser gate in place of the verified-email gate
  - ProfileForm in signup, finish and edit modes, a Log in page, and username checks that need no token
affects: [01-20, 01-21, 02-daily-roll]
tech-stack:
  added: []
  patterns: ["Form mode is chosen once on mount, so the page does not change shape when the session changes", "Test suites refuse to run unless the database name ends in _test (CheckTestDatabaseURL)"]
key-files:
  created: [migrations/0002_user_birthday.up.sql, internal/user/birthdate.go, internal/middleware/bodylimit.go, internal/store/postgres/testdb.go, components/profile/ProfileForm.tsx, lib/profile/birthday.ts, app/(auth)/make-it-yours.tsx]
  modified: [internal/httpapi/auth.go, internal/httpapi/profile.go, internal/httpapi/server.go, internal/middleware/verified.go, internal/store/postgres/user_repo.go, cmd/api/main.go, app/(auth)/_layout.tsx, app/(auth)/login.tsx]
key-decisions:
  - "One INSERT makes the whole account, so any failure stores nothing; under 13 gets 403 under_minimum_age before the insert"
  - "Email is not verified at sign-up (developer, 2026-10-06): the verified-email gate and the link flow are gone; internal/mail and the token table stay unused"
  - "Open decision (a): Apple and Google accounts also enter a birthday. Built as (a); the developer's answer is still to be asked in Task 4"
  - "The Apple and Google claim of an unverified email account is one transaction (ClaimAndRevoke)"
patterns-established:
  - "A Phase 2 route that needs a finished profile checks OnboardingComplete() on top of the RequireAuth and RequireUser group gate"
requirements-completed: [ACCT-01, ACCT-03]  # plan requirement IDs, not yet signed off
coverage:
  - id: D1
    description: "One-request sign-up, age rule, birthday privacy, body cap, public username checks and the Apple and Google claim on the server"
    requirement: ACCT-01
    verification:
      - kind: integration
        ref: "go test ./... -p 1 -count=1 on rndmroll_test: 254 passes, 0 failures, 0 skips at 7ca2471 (01-VALIDATION.md); green again in the 2026-10-09 check"
        status: pass
    human_judgment: false
  - id: D2
    description: "The 'Make it yours.' form in three modes, the Log in page and routing"
    requirement: ACCT-03
    verification: []
    human_judgment: true
    rationale: "No mobile test framework in Phase 1; walkthrough Task 4 is the check and is not finished"
duration: not recorded
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 19: One-Request Sign-Up and the Shared Profile Form Summary

**One request makes an account (email, password, birthday, name, username), the birthday is private and checked for 13+ on the server, the verified-email gate is gone, and one "Make it yours." form serves sign-up, finish and edit.**

## Performance
- **Duration:** not recorded. **Started:** 2026-10-07 (plan commit `383d789`, "developer said start")
- **Completed:** 2026-10-09 (approved) (Tasks 1, 2, 3A and 3B built 2026-10-07; the Task 4 walkthrough is still running)
- **Tasks:** 5 (1, 3A, 2, 3B built; 4 open). **Files modified:** 35 in `7ca2471`, 15 in `bf755de`, 8 in `739bcfc`

## Accomplishments
- Server (`7ca2471`): migration 0002 adds `users.birthday`; responses carry `has_birthday`, never the date. Sign-up is one INSERT and under 13 stores nothing. Every `/v1` route has a 16 KB body cap (413), `PATCH /me` takes an `avatar_url` only inside the caller's own folder, and no proxy header is trusted. Go tests went from 158 to 254 passes, 0 skipped (`7ca2471` message; `-p 1` on `rndmroll_test`).
- App (`bf755de`): `ProfileForm` plus thin routes `make-it-yours.tsx` and `login.tsx`. Deleted: the email-link screens, the old create-profile screen, the onboarding draft, `verify_email.go` and its test. The raw `#111111` in `WheelBackground.tsx` now uses the ink token. `app/index.tsx` and `app/_layout.tsx` are untouched.
- Docs (`739bcfc`): the scan lists and automated command in 01-15 match the new files, and UI-SPEC revision 13 describes the page. Plan Task 2g triages the `01-15-PRETRACE.md` section 4 defects.
- Walkthrough Task 4, as rewritten by 01-20 and 01-21 (full status in `01-15-SUMMARY.md`). Passed with the developer: Group 1; Group 2; the duplicate-email message; photo upload on the main page; check 4 (sign-up, photo and bio on page 2, landing page); staying signed in. Not yet seen: the new Sign up and Log in buttons, the Continue button and "You're in." page, Skip for now, and step 22 (the developer's older account still logs in and saves an edit).

## Task Commits
1. **Task 1: server** - `7ca2471` (feat)
2. **Task 3A: cutover** - no commit; done per `HANDOFF.json`. Rollback is tag `pre-19` plus the saved binary `~/.local/share/rndmroll-api/api.pre-19`
3. **Task 2: app** - `bf755de` (feat)
4. **Task 3B: gates, scans, docs** - `739bcfc` (docs)
5. **Walkthrough follow-ups, listed once here:**
   - `b00cbf0` login Back; `c864e16` black Sign up and outlined Log in buttons on the first screen (the developer said yes to the pair on 2026-10-09; not yet seen on the phone)
   - `fdb457a` "That email is already in use." under the Email box, Bio box 2 lines, any image up to 50 MiB (`internal/storage/s3.go`), upload through `expo-file-system` `uploadAsync`; its Done-always-on part is under 01-21
   - `0f25ea5`, `a563b5f` one bottom button on the main page: Continue (opens the placeholder `app/(app)/home.tsx`) or Save changes
   - `02ab586`, `d14a778`, `82c6213`, `4f5818a`, `2b4ddd7` Apple and Google buttons removed, restored, then hidden behind two constants, both false

**Plan metadata:** `383d789` (plan), `7419f3a` (01-18 superseded). `6054474` built the "Make it yours." page that became edit mode.

## Files Created/Modified
See `key-files`. Also deleted: `profile-setup.tsx`, `verify-email.tsx`, `lib/onboarding/draft.ts`; `email.tsx` became `login.tsx`.

## Decisions Made
See `key-decisions`. Accepted risk from the plan's threat model: email is not verified, so someone can register another person's address; the Apple and Google claim takes such an account over. Verification is on the launch list.

## Deviations from Plan
Outside the plan's file list: `7ca2471` also changed the Makefile (a comment saying `-p 1` is required because `TestMigrations_0002IsReversible` drops and re-adds a column) and added `internal/store/postgres/testdb.go`. No other deviation is recorded in the code tasks. The plan text changed after it was written: Task 4 was rewritten for two pages (01-20) and the date scroller (01-21). Check 5, the details page for an unfinished Apple or Google account, was deferred on 2026-10-09 until real Apple or Google sign-in exists, because a Log in that asks for birthday, name and username makes no sense to the developer. Task 2's acceptance line "cold launch done in each reachable state" is still open for finish mode, which moved to check 5.

## Issues Encountered
- The login page had no way back, and "Sign up instead" skipped the Apple and Google choice (`b00cbf0`).
- Photos from the phone did not reach storage with a fetch and Blob PUT, and the old rule refused all but JPEG, PNG and WebP up to 5 MiB. Fixed in `fdb457a`, confirmed in the phone and server log.
- A test step said "Tap Log in" while the sign-up page was open, and a login was typed into the sign-up form on 2026-10-09. Test steps must name the page. Accepted in the plan: if Apple's first request fails, the retry shows an empty Name because the onboarding draft is gone.

## User Setup Required
Local only: the API in test mode (`~/.local/share/rndmroll-work/start-api-test-mode.sh`), test storage (`start-test-storage.sh`) and the `migrate` CLI for 0002. Real accounts are deferred.

## Next Phase Readiness
- Open: the walkthrough items above; keep-or-change on every new string (this plan's table, 01-20, 01-21, `01-15-PRETRACE.md` sections 5 and 6); open decision (a); "approved".
- Phase 2 gate: `authed` needs `RequireAuth` plus `RequireUser`; a route that needs a finished profile also checks `OnboardingComplete()`. The next free migration is 0003.
- Deferred to the launch list: check 5, real Apple, Google, S3 and Resend accounts, universal links, Forgot password, email verification, real Terms and Privacy text (from the developer), custom domain, favicon, removing the AI tag, proxy trust.

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Phase: 01-foundation-accounts*
*Completed: 2026-10-09*
