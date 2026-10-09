---
phase: 01-foundation-accounts
plan: 17
subsystem: auth
tags: [session, refresh-token, oauth-claim, apple-name, profile-screens, gap-closure]
requires:
  - phase: 01-foundation-accounts (plan 15)
    provides: the 01-15-SWEEP.md defect table D1 to D14 from the 2026-09-30 static review
provides:
  - A session that keeps its tokens on network errors and no longer hangs the splash on an expired refresh token
  - A safe Apple and Google claim of an unverified email account, and a repeat Apple sign-in that keeps the stored name
affects: [01-19]
tech-stack:
  added: []
  patterns: ["Fixes written in an isolated copy of the repo at HEAD, reviewed by adversarial lenses, dry-run on the live tree, then applied at a walkthrough boundary"]
key-files:
  created: [internal/httpapi/oauth_claim_test.go]
  modified: [lib/session/store.ts, lib/api/client.ts, lib/auth/social.ts, components/ui/TextField.tsx, internal/httpapi/oauth.go, internal/store/postgres/user_repo.go, internal/store/postgres/refresh_token_repo.go]
key-decisions:
  - "Sign out and delete tokens only when the server definitively rejects the refresh token; network errors, timeouts, 5xx and 429 keep them"
  - "A social sign-in that links to an unverified email account claims it: revoke its refresh tokens, clear its password, then link, each step safe to repeat"
patterns-established:
  - "Routing files are fragile (255ca31, 61855e2, a4fffa5): change them only with a cold-launch check"
requirements-completed: [ACCT-01, ACCT-03]  # plan requirement IDs, not yet signed off
coverage:
  - id: D1
    description: "Backend claim, Apple name safety and refresh rotation guard"
    requirement: ACCT-01
    verification:
      - kind: integration
        ref: "go test ./... -p 1 -count=1 -v on rndmroll_test: 158 passes, 0 failures, 0 skips (2026-10-03, 01-17-WORKLOG.md)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Session, profile-screen and auth-stack fixes in the app"
    requirement: ACCT-01
    verification: []
    human_judgment: true
    rationale: "No mobile test framework in Phase 1; the device walkthrough is the check and is not finished"
duration: not recorded
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 17: Walkthrough Defect Fixes Summary

**Fixes for 12 of the 14 defects in the static walkthrough review, committed as a mobile patch and a Go patch, with the Go suite at 158 passes and 0 skipped; plan 01-19 later reshaped part of the code.**

## Performance
- **Duration:** not recorded. **Started:** 2026-10-01 (plan written; the patch was ready outside the repo that day)
- **Completed:** 2026-10-09 (approved) (applied to the live tree 2026-10-03, committed 2026-10-05)
- **Tasks:** 5 (session; create-profile and social; edit and profile view; auth stack and verify email; backend). **Files modified:** 10 in `479b170`, 12 in `ba57e39`

## Accomplishments
- D1: the session signs out only on a definite refusal. An expired refresh token no longer hangs the splash (a deadlock in `lib/api/client.ts`), tokens that a newer sign-in replaced are never deleted, and the app retries once on return from the background.
- Profile screens: Finish disabled while an avatar uploads; camera and library failures handled; username lowercased and check failures shown; Name prefilled; upload size taken from the real blob; name and bio trimmed; back gesture off on Create your profile; a superseded verification link has its own message.
- Backend: the social claim, Apple needs an email, a repeat Apple sign-in with no name keeps the stored name (T-01-UAT-01), refresh rotation guarded, em dashes out of Go comments. New `oauth_claim_test.go`.
- Gates on the patched tree, 2026-10-03: `tsc` exit 0; `go build`, `go vet` ok; `go test ./... -p 1 -count=1 -v` on `rndmroll_test` 158 passes, 0 failures, 0 skips (133 before); design scans clean. A smoke test through the live API passed 19 of 19, on the email-link flow that 01-19 later removed. `expo export` was not run then and passed on 2026-10-09. The worklog records two rounds of isolated fixing and adversarial review plus a hand round before applying.

## Defect outcomes (IDs from 01-15-SWEEP.md)
| ID | Outcome | Today |
|----|---------|-------|
| D1 | Fixed, `479b170` | Stands (`isDefinitiveAuthFailure` in `lib/session/store.ts`) |
| D2, D3, D5, D8 | Fixed, `479b170` | `bf755de` deleted profile-setup and verify-email and turned email into `login.tsx`; `ProfileForm` took over profile-setup's job, and the back-gesture rule now targets make-it-yours in `app/(auth)/_layout.tsx` |
| D6, D10, D11 | Fixed, `479b170` (D11's whitespace-only bio prompt was in `06d8a8a`) | Camera, username, trim and upload-size fixes are now in `ProfileForm`; upload reworked in `fdb457a`; the bio prompt went with the profile view |
| D7 | Fixed, `ba57e39` | Replaced by the single-transaction `ClaimAndRevoke` in `7ca2471` |
| D9 | Fixed, `479b170` | 01-19 accepted a regression: with the onboarding draft deleted, a failed first Apple request leaves an empty Name on retry |
| D12, D14 | D12 `ba57e39`. D14 in part: stale comment in `app/_layout.tsx` `479b170`, Go comment dashes `ba57e39`, sheet elevation `06d8a8a` | D12 stands (an empty Apple email is refused where an account would be created); the sheet went when `6054474` removed the profile view |
| D4, D13, D15 | Not changed | D4 is checked by 01-19 Task 4 step 12, and no source says the flash is gone; D13 is moot since `7ca2471` removed the link flow; D15 waits for the Phase 5 native rebuild |

## Task Commits
1. **Mobile fixes (Tasks 1 to 4)** - `479b170` (fix)
2. **Backend (Task 5)** - `ba57e39` (fix)

**Plan metadata:** `d36de8d` (plan and worklog written)

## Files Created/Modified
See `key-files`. The patch was 22 files: 21 modified and the new `oauth_claim_test.go`.

## Decisions Made
See `key-decisions`. The taller Bio box with an "n / max" counter (`TextField`) was added to `479b170` at the developer's request on 2026-10-05; it is not in the plan.

## Deviations from Plan
None recorded in the defect fixes. Outside the plan's file list: the Bio box height and counter in `TextField` (developer request), a refresh-rotation guard in `refresh_token_repo.go`, and `06d8a8a`, a draft profile redesign committed alongside so it could be undone (`6054474` replaced it).

## Issues Encountered
- `patch -R` stopped applying once the 2026-10-04 commits touched `app/(auth)/_layout.tsx`, so `git revert` is the undo.
- Device evidence is thin. The worklog records a cold launch with no error screen after the patch, but not the expired-token or API-stopped relaunch checks. Staying signed in later passed in 01-19 Task 4.

## User Setup Required
None. The smoke test used local test storage (MinIO) and `MAIL_DRIVER=log`.

## Next Phase Readiness
- Strings: none approved yet. This plan's new or changed strings are in `01-15-PRETRACE.md` section 5, group C; some sit on screens 01-19 deleted.
- Deferred: real Apple and Google sign-in (walkthrough steps 19 and 20), universal links and email verification (the link flow is gone), D15 permission prompt text (native rebuild with Phase 5).

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Phase: 01-foundation-accounts*
*Completed: 2026-10-09*
