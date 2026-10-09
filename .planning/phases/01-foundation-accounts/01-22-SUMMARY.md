---
phase: 01-foundation-accounts
plan: 22
subsystem: auth
tags: [login, username, rate-limit, text-input]
requires:
  - phase: 01-foundation-accounts (plans 19 to 21)
    provides: the one-request sign-up, the profile form and the username rules
provides:
  - Log in takes an email OR a username plus the account's one password
  - Text boxes that never cut off the underscore or the tails of g, j, p, q and y
affects: [login, sign-up, profile]
tech-stack:
  added: []
  patterns: ["An email is any login with an @, anything else is a username; text that could never be a username is an unknown account, not a format error"]
key-files:
  created: []
  modified: [internal/httpapi/auth.go, internal/user/repository.go, internal/store/postgres/user_repo.go, internal/middleware/ratelimit.go, app/(auth)/login.tsx, lib/api/client.ts, components/ui/TextField.tsx]
key-decisions:
  - "ONE password per account, typed once on sign-up and used with the email and the username (a confirmation box, then two separate passwords, were built and removed the same evening)"
  - "The Log in request field is login (email stays accepted as the older name); an email-shaped or empty login is checked like a sign-up address and answers 400 naming login"
  - "Login is limited to 20 a minute per address in total and 5 a minute per address and login, because a username is shown to friends and an email is not"
  - "An account with no password (Apple or Google) runs the same dummy comparison and gets the same answer; the dummy hash is never used as the account's own"
requirements-completed: [ACCT-01]
coverage:
  - id: D1
    description: "Log in by username and by email, same message for a wrong password and an unknown username"
    requirement: ACCT-01
    verification: ["go test ./... -p 1 (handler, full-server, middleware, Postgres and end-to-end tests)", "live API checks with curl"]
    human_judgment: false
    rationale: "Covered by automated tests and by the developer's check on the phone (2026-10-09)"
  - id: D2
    description: "Typed text is never clipped in a focused text box"
    requirement: ACCT-01
    verification: []
    human_judgment: true
    rationale: "Seen on the phone with a close-up screenshot; no mobile test framework exists in Phase 1"
duration: one evening
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 22: Log in with an email or a username Summary

**The Log in page's first box takes an email or a username, both checked against the account's one password, and the text boxes no longer cut off what you type.**

## Performance
- **Duration:** one evening. **Started:** 2026-10-09 (the developer: a username and a password should be enough to log in)
- **Completed:** 2026-10-09 (approved)
- **Tasks:** server and tests, app, applied to the running API and phone, developer check. **Files modified:** 7 source files plus tests and docs

## Accomplishments
- Server: `GetByUsernameCI`, the `login` request field, the two login limits, the dummy comparison for accounts without a password. Tests added in the handler, full-server, middleware, Postgres and real-database end-to-end suites; the whole Go suite passes on `rndmroll_test`.
- App: the first box is labelled "Email or username"; empty shows "Enter your email or username."; the wrong-login message is "That email, username or password isn't right."
- Text boxes: single-line inputs have no line height and a taller frame, because a focused iOS field clips to its frame and the old 24dp line height put the text low in it (UI-SPEC revision 18).
- Rolled back the same evening: a Confirm password box and two separate passwords (a migration column, plans 01-23 and 01-24). The patch is kept outside the repo at `~/.local/share/rndmroll-work/two-password-attempt-2026-10-09.patch`; both databases are at migration version 2.

## Decisions Made
See key-decisions. UI-SPEC revisions 16 to 18 and the PROJECT.md decision row record the developer's choices.

## Deviations from Plan
The plan grew twice during the evening (confirmation, second password) and both were removed again. A hot update of the form once did not reach the phone; the app was restarted at a boundary.

## Issues Encountered
Accounts made before this have only the one password, so nothing about them changed. Hint words inside the sign-up boxes were tried and rejected.

## Next Phase Readiness
Phase 2 starts from migration 0003 (the Phase 2 documents use that number). Phone-number sign-up, an email code, Forgot password and real Google and Apple sign-in stay on the launch list.

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Completed: 2026-10-09*
