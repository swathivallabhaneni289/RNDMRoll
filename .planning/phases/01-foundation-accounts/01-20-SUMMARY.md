---
phase: 01-foundation-accounts
plan: 20
subsystem: ui
tags: [signup, expo-router, session-flag, profile-form]
requires:
  - phase: 01-foundation-accounts (plan 19)
    provides: one-request sign-up and ProfileForm with signup, finish and edit modes
provides:
  - Two-page sign-up. Page 1 "Create your account." makes the account; page 2 "Make it yours." (photo and bio) shows once
  - ProfileForm mode extras and the in-memory extrasPending session flag
affects: [01-21, 02-daily-roll]
tech-stack:
  added: []
  patterns: ["A one-time page driven by an in-memory session flag read in the landing route, so no gated routing file is edited"]
key-files:
  created: []
  modified: [components/profile/ProfileForm.tsx, lib/session/store.ts, app/(app)/profile/index.tsx, lib/api/client.ts, lib/api/profile.ts]
key-decisions:
  - "Name and Username stay on page 1; only photo and bio move to page 2 (the developer said 'bio and photo' and nobody objected)"
  - "Page 2 never returns after a relaunch and never shows after a plain log in; the flag lives in memory only"
  - "Create your account. uses the Heading role (22) because Display (40) would wrap to two lines; Make it yours. keeps Display"
patterns-established:
  - "The landing route renders ProfileForm with key={mode}, so nothing typed on one page carries to the other"
requirements-completed: [ACCT-01, ACCT-03]  # plan requirement IDs, not yet signed off
coverage:
  - id: D1
    description: "Two-page sign-up with photo and bio shown once after sign-up"
    requirement: ACCT-01
    verification: []
    human_judgment: true
    rationale: "No mobile test framework in Phase 1; the device walkthrough (01-19 Task 4) is the check and is not finished"
duration: not recorded
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 20: Photo and Bio on Their Own Page Summary

**Sign-up is now two pages: account details on page 1, then an optional photo and bio page shown once, driven by an in-memory session flag with no routing file edited.**

## Performance
- **Duration:** not recorded. **Started:** 2026-10-08 (the developer's request during the 01-19 walkthrough)
- **Completed:** 2026-10-09 (approved) (built and committed 2026-10-08)
- **Tasks:** 3 (Build: done; Docs: done; Walkthrough: open). **Files modified:** 6 in `255c8ad`, 9 in `3bfcb80`

## Accomplishments
- Page 1, "Create your account.", has Email, Password, Birthday, Name and Username and still makes the account in one request. Finish mode (an unfinished Apple or Google account) shows Birthday, Name, Username and Log out. Neither page has a photo or a bio.
- Page 2, "Make it yours." (`ProfileForm` mode `extras`): camera circle, Bio (Optional), Continue and a "Skip for now" link; no Back and no Log out. A failed photo upload keeps the person there with the message under the circle and saves nothing. Otherwise one save, then the landing page.
- `lib/session/store.ts` gained `extrasPending`: `signIn(result, { extras: true })` sets it, `startExtras()` and `finishExtras()` set and clear it, and sign-out clears it. The landing route `app/(app)/profile/index.tsx` shows `extras` or `edit`. The server did not change.
- Build gates (`255c8ad` message, 2026-10-08): `tsc` exit 0; `expo export` for iOS exit 0; no em dash, emoji or raw hex. A cold launch of the dev client landed on Welcome (`01-15-PRETRACE.md` section 6). `tsc` and `expo export` passed again in the 2026-10-09 check.
- Review by three read-only lenses, with a second reader per finding: no confirmed defects (`255c8ad` message). Two small fixes followed (see Deviations).
- Walkthrough: check 4 (sign-up, photo and bio on page 2, landing page) passed with the developer. The Skip for now path has not been seen yet.

## Task Commits
1. **Task 1: build** - `255c8ad` (feat)
2. **Task 2: docs** - `3bfcb80` (docs: plan, UI-SPEC revision 14, walkthrough Groups 2 to 5 rewritten)
3. **Task 3: walkthrough** - open, no commit

## Files Created/Modified
See `key-files`. `app/(app)/profile/edit.tsx` changed by one comment.

## Decisions Made
See `key-decisions`. The developer picked option a (a new page with just photo and bio, shown once) after asking how Instagram does it. What could be verified: the 2022 iPhone sign-up had the photo as its own page after the account details; the 2024 Android recording went straight into the app with no photo page; neither asks for a bio at sign-up.

## Deviations from Plan
No design deviation recorded; the old photo-failed Alert was removed as planned. Review found two small things, both fixed (`01-15-PRETRACE.md` section 6): a stale photo message kept across a retry, and unused explicit-token plumbing from the old sign-up photo upload, removed from `lib/api/client.ts` and `lib/api/profile.ts`. The plan's file list had nothing for `client.ts` and only one comment for `profile.ts`.

## Issues Encountered
- On 2026-10-08 an app restart in the middle of a sign-up skipped a tester's page 2, because the flag is in memory. Going Home and tapping the app icon does not reload the JavaScript; only a swipe-away or a `simctl` relaunch does. Relaunch only at a boundary.
- Known gap: a photo the server rejects cannot be removed, so the way past it is another photo or Skip for now, which drops a typed bio.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Open: the Skip for now path on the phone, then keep-or-change on "Create your account." and "Skip for now", then "approved".
- Not in this plan: moving Name and Username to page 2, a Back from page 2, showing page 2 again after a relaunch, any server change.
- Deferred: check 5 (the Apple and Google finish path into page 2) waits for real Apple or Google sign-in. Launch list: Forgot password, email verification, real Terms and Privacy text, custom domain, favicon, removing the AI tag.

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Phase: 01-foundation-accounts*
*Completed: 2026-10-09*
