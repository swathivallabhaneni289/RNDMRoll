---
phase: 01-foundation-accounts
plan: 21
subsystem: ui
tags: [date-picker, expo-ui, swiftui, form-feedback, profile-form]
requires:
  - phase: 01-foundation-accounts (plan 20)
    provides: page 1 "Create your account." in the ProfileForm signup and finish modes
provides:
  - An iOS scrolling date picker for the birthday, in a bottom sheet with an always-available Done
  - A green "Looks good." note under Email, Password, Name and Birthday on the sign-up and finish pages
affects: [sign-up, apple-google-finish]
tech-stack:
  added: []
  patterns: ["Native pieces from @expo/ui/swift-ui are loaded with require inside try/catch behind Platform.OS === 'ios', and the form falls back to the typed boxes when they are missing"]
key-files:
  created: [components/profile/BirthdayPickerField.tsx]
  modified: [components/profile/ProfileForm.tsx, components/ui/TextField.tsx]
key-decisions:
  - "No new native module and no dev client rebuild: @expo/ui was already compiled in"
  - "Done is always available, because someone born on the picker's starting date (January 1, 25 years ago) must be able to pick it; the chosen date is shown in the box because the birthday cannot be changed after sign-up"
  - "The picked date goes through the same checkBirthday rules as the typed boxes; the finish page leaves the 13+ rule to the server"
patterns-established:
  - "Dates are built at local noon and read back by local calendar parts, so no time zone shifts the day"
requirements-completed: [ACCT-01]  # plan requirement IDs, not yet signed off
coverage:
  - id: D1
    description: "Birthday date scroller on iOS and green Looks good. notes on the sign-up and finish pages"
    requirement: ACCT-01
    verification: []
    human_judgment: true
    rationale: "No mobile test framework in Phase 1; the device walkthrough (01-19 Task 4 Group 2) is the check and is not finished"
duration: not recorded
completed: 2026-10-09
status: complete
---

# Phase 1 Plan 21: Date Scroller and Green Notes Summary

**On iOS the birthday is picked on the system's scrolling date picker in a bottom sheet instead of typed, and checked fields on the sign-up and finish pages show a green "Looks good." note.**

## Performance
- **Duration:** not recorded. **Started:** 2026-10-08 (the developer's "less typing" request after Group 2 passed)
- **Completed:** 2026-10-09 (approved) (built and committed 2026-10-08)
- **Tasks:** 3 (Build: done; Walkthrough: Group 2 re-check passed, the rest open; Commit: done). **Files modified:** 3 in `9537230`, 10 in `a0b7d98`

## Accomplishments
- `BirthdayPickerField.tsx` (new): a soft box reading "Select your birthday" until a date is chosen, then the date with a down chevron. Tapping it opens a sheet shaped like the Log out sheet, with the SwiftUI `DatePicker` from `@expo/ui` (wheel style, fixed height 216, light color scheme) and a Done button. The picker runs from 1 January 1900 to today, and the dark area above the sheet closes it without changing the date. Off iOS, or if the native view is missing, the three typed boxes remain.
- Green notes: `TextField` gained a `success` prop and an exported `FieldSuccess`. "Looks good." shows under Email as you type, Password at 8 or more characters (replacing the grey "At least 8 characters." line), Name after you leave the box, and Birthday once a valid date is chosen. An error always replaces the note. The username keeps its own "Available". The edit page and the photo and bio page have no notes.
- Build gates (`9537230` message, 2026-10-08): `tsc` exit 0; `expo export` for iOS exit 0 (the plan says this proves the `@expo/ui/swift-ui/modifiers` import resolves); no em dash, emoji or raw hex. `tsc` and `expo export` passed again in the 2026-10-09 check.
- Review by three read-only lenses, with the picker code checked line by line against the `@expo/ui` source and a second reader per finding: no confirmed defects, wording fixes only.
- The developer saw the scroller, the sheet and the notes on the phone on 2026-10-08: "Looks good to me" (the Group 2 re-check).

## Task Commits
1. **Task 1: build** - `9537230` (feat)
2. **Task 2: walkthrough** - Group 2 re-check passed 2026-10-08; the scroller on the finish page belongs to check 5 and waits
3. **Task 3: commit** - `9537230`, made after the developer saw it (rollback point was `3bfcb80`)

**Plan metadata:** `a0b7d98` (docs: plan, UI-SPEC revision 15, 01-15 radius allow-list). Follow-up `fdb457a`, Done part only: the first build kept Done off until the picker moved, which blocked the developer's own birthday because it matched the starting date. Its other parts are listed under 01-19.

## Files Created/Modified
See `key-files`.

## Decisions Made
See `key-decisions`. The 24dp radius token (`radius.lg`) is now used in a third file, `BirthdayPickerField.tsx` (the date sheet only); the allow-list is in `01-15-PLAN.md` and UI-SPEC revision 15. Of five picks offered, the developer chose the birthday scroller and the green notes. They declined the other four (keyboard Next key, hints inside the boxes, tap-to-pick usernames, Apple and Google first) and said Face ID is "not yet".

## Deviations from Plan
None recorded against the plan as it now reads. The first build's Done rule was corrected after the developer's follow-up (`fdb457a`).

## Issues Encountered
- Done stayed off until the picker moved, so a birthday equal to the starting date could not be picked. Fixed in `fdb457a`.
- On iOS the typed-only birthday messages ("Enter your birthday as month, day and year.", "Enter a four-digit year.", "That date doesn't exist. Check the day and month.", "That date is in the future.") can no longer be reached; they remain for other platforms.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Open: keep-or-change on "Looks good.", "Select your birthday" and "Done", then "approved". The developer also changed one string themselves during Group 3, "That email is already in use." (see 01-19).
- Waiting on check 5: the scroller on the Apple and Google finish page, deferred until real Apple or Google sign-in exists.
- Not in this plan: the keyboard Next key, hints inside the empty boxes, tap-to-pick usernames, Apple and Google above Email, email-domain buttons, any server change. Face ID is a later phase and needs a dev client rebuild.

## Developer sign-off
Approved by the developer on 2026-10-09 (they typed "Approve.") after the final check on the phone: sign-up with one password, log in by username and by email. The earlier new-account test of the first screen, Skip for now and the Continue page was not re-run in the last session: the developer said those parts had been tested earlier and asked to drop it.

---
*Phase: 01-foundation-accounts*
*Completed: 2026-10-09*
