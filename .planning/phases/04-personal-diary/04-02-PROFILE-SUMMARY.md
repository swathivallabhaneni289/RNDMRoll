# Plan 04-02 (Profile part) summary: the Profile page, the Edit page and Settings (D-16)

**Built:** 2026-10-13, on branch `phase-01-foundation-accounts`, local commits only, nothing pushed. This is the PROFILE PART of plan 04-02 (the paper's components 4.1 to 4.6) plus the developer's Settings decisions of the same day (Phase 2.1 decision D-16). The Diary list, the Diary tab, the entry routes and the bar of four are NOT in it.

## Commits

| Commit | Message |
|---|---|
| `1fe35fe` | feat(app): profile page pieces, summary call and the shared chip (profile task 1) |
| `039fc1c` | feat(app): Profile tab shows the new page, Edit opens the form pushed (profile task 2) |
| `ed34a92` | feat(app): Settings is Notifications, Blocked accounts, Private account, Password, Log out (D-16) |
| this file's own commit | docs: Profile part summary, D-16 on the Settings papers |

## What was built

**Profile page** (`components/profile/ProfileScreen.tsx`, route `app/(app)/profile/index.tsx`). Top to bottom: a slim 44dp row with the Settings gear at the right ('Saved.' at the left for 3 seconds after the Edit page saves); the photo in the existing `DialAvatar` circle (72dp, the camera stand-in when there is none) with the name beside it (serif), the @username under it and the outlined 'Edit' pill at the right; the bio in place (`BioInline`, no label) full width under that row; a dark rule and the three figures (Spins, Week streak, Friends); 'YOUR TASTES . AVG RATING' with the rounded boxes (category left, average right, three to a row); 'RECENT ACTIVITY' with 'See diary' and up to three rows (thumbnail, serif title, 'MEAL . OCT 8'). The drawing is `ProfileView` (props only); `ProfileScreen` wires the session user, `GET /me/summary` (`useFocusEffect`, reload on every focus), and the notice store. First run is untouched (Phase 1's page, no bar, no gear).

**Edit page** (`app/(app)/profile/edit.tsx` = `ProfileForm mode="edit" pushed`). The Phase 1 form pushed over the Profile page: Back, the bar showing, photo with the text 'Edit your picture' under it (same camera or library choices), name, username with the live check (green 'Looks good.' for a free name, the alternates for a taken one), bio, 'Save changes' (grey until something changes), Log out as before. A save goes back and the Profile page shows 'Saved.'.

**Settings** (D-16). Rows: Notifications, Blocked accounts, Private account, Password, Log out. No Bio block, no 'Edit profile', no 'Username'; the Username page, its route and its check hook are removed. New pages: Notifications (three switches ON by default, each saves on toggle, inline error and revert) and Blocked accounts (round photo, name, @username, 'Unblock'; "You haven't blocked anyone." when empty).

**Shared pieces.** `components/ui/Chip.tsx` (the one new `radius.full` file; tones selected, soft, outline, strong), `lib/api/summary.ts` (summary call and types), `lib/api/account.ts` (notification and block calls and types), `lib/api/client.ts` (one new method, `delete`), `components/profile/` (`summaryText.ts` pure words and the month table, `useProfileSummary.ts`, `SizedAvatar.tsx`, `SectionLabel.tsx`, `StatsRow.tsx`, `TastesSection.tsx`, `RecentActivity.tsx`).

## Gates

| Gate | Result |
|---|---|
| `npx tsc --noEmit` | exit 0 after each task and at the end |
| Pure text checks (month table for every month's first and last day and a leap day, averages, spoken strings, singular forms, no em dash) | all passed (script is in the scratchpad, not the repo: `scripts/` was not on the path list) |
| `scripts/check-settings.ts` (rules and notice store, not edited) | 50 checks, 0 failures |
| Em dash, hex or `rgba`, emoji, non-ASCII, the word "roll", `console.`, `Math.random`, `new Date` over the files of this work | nothing (the `new Date()` calls in `ProfileForm.tsx` are Phase 1's birthday checks) |
| `radius.full` list | exactly `components/brand/DialAvatar.tsx`, `components/nav/SpinTabButton.tsx`, `components/ui/IconBadge.tsx`, `components/ui/Chip.tsx` (new, named by the paper), `lib/theme/tokens.test-assert.ts` (5 files) |
| `radius.lg` list | unchanged: `PhotoPanel.tsx`, `ConfirmSheet.tsx`, `ProfileForm.tsx`, `BirthdayPickerField.tsx`, `tokens.test-assert.ts` |
| `expo lint`, new package, `app.json`, `ios`, `android` | not run, none added, not touched |
| Files of other windows | not touched (`lib/api/types.ts`, `lib/api/spin.ts`, `lib/wheel/format.ts` held another plan's uncommitted lines and were left alone; `monthDay` and the summary types live in this work's own files) |

## What I saw (simulator, iPhone 17 Pro, signed in as the throwaway walkthrough account)

- The Profile page, Edit page, Settings, Notifications and Blocked accounts were opened by deep link and screenshotted. The summary route is not live, so the Profile page showed its real empty state (three zeros, the two empty lines); the full layout was checked with a temporary fixture (Ava Stone's numbers) that was reverted before every commit (no marker left in the tree).
- The API log shows the right calls: `GET /v1/me/summary`, `GET /v1/me/notification-settings` and `GET /v1/me/blocks`, each 404 for now, and no doubled `/v1`.
- Adding the two route files and removing the Username route made Metro reload the whole app once (14:47). The phone stayed signed in (refresh 200, then `/me` 200, no logout) and landed on Today. The yellow "Open debugger to view warnings" bar is the one Phase 1 noted as showing on every launch.
- Measured against `docs/reference/profile-page-2026-10-13.png` (both in points; the picture scaled as a 393pt screen):

| Thing | Picture | Built |
|---|---|---|
| Photo circle | 74 | 72 |
| Photo row to the dark rule | 23 | 24 (+ the bio row, see below) |
| Rule to closing rule (the figures) | 76 | 81 |
| Taste box height, gap | 41.5, 7.5 | 40, 8 |
| Label to first row of boxes | 14 | 12.7 |
| Last box to RECENT ACTIVITY | 27.5 | 28.7 |
| Label to first thumbnail | 12.3 | 12.0 |
| Thumbnail, row pitch | 47, 67.6 | 48, 69 |
| Edit pill height | 32 | 32 |
| Side margins | 23 | 24 |

## Differences from the picture (on purpose or forced)

1. **A gear row above the photo (44dp) and the bio under the photo row** (asked for in words; the picture draws neither). The photo starts about 44dp lower than in the picture; with no bio the 'Add a bio' link row (44dp) sits between the photo and the dark rule.
2. **Section labels are 14pt** (the app's Label size), the picture's are about 11pt. The tokens have nothing smaller (11 is the wheel's only).
3. **Serif sizes.** The picture's name, figures and titles are drawn in a narrower serif than Domine. Built with existing roles: the name and the three figures use `note` (Domine Regular 20/28), not `heading` (22 semi-bold) and not `title` (32), because the picture's name and figures are light and about 20 to 23pt. The paper says `heading` for the name and `title` for the figures; the picture won. Recent titles use `note` as the paper says (they are larger than the picture's about 15pt: no smaller serif role exists).
4. **The averages in the taste boxes are in the serif**, as the picture draws them (the paper said Body). Flip: one style in `components/ui/Chip.tsx` (`value` text).
5. **The Edit pill is 32dp high** (the picture), not the paper's 36; the touch area is 44dp.
6. **'Saved.' is at the top left of the slim row**, not under the bio (no layout jump when it appears and goes; announced to VoiceOver once). It shows only for the exact text 'Saved.', so the password notice never leaks onto this page.
7. **The photo is `DialAvatar` scaled** inside a 72dp box (`SizedAvatar`): `DialAvatar` has one fixed size and `components/brand` was not on the path list. A `size` prop on `DialAvatar` would replace the wrapper.

## Differences from the paper (04-UI-SPEC and plan 04-02) and from the brief

- **Recent rows and taste boxes are plain, not buttons, for now.** The entry page (Phase 3's `EntryView`, no `components/entry` yet) and the Diary's category filter do not exist, so a tap would have nowhere to go. 'See diary' is a real button (it opens the Diary tab). The seams are two optional props, `onOpenEntry` and `onSelectTaste`, on `ProfileView`; the wiring is listed below.
- **A failed or missing summary shows zeros and two empty lines** ('Your tastes show here after your first posts.' and 'Nothing here yet.'), as the brief says, instead of the paper's Offline block and 'Try again'. A failed RELOAD keeps the last good summary on screen. The only loading state is the line 'Loading your profile.' before the first answer settles.
- **'See diary' and both section labels are always drawn** (the paper left the sections out when empty).
- **The Edit page has no 'Continue'**, its photo link reads 'Edit your picture' only when pushed (the first-run page keeps 'Add photo'), and a free username shows the green 'Looks good.' instead of the form's own 'Available' (pushed only). A save goes back and shows 'Saved.' on the Profile page (paper 4.6), not a 'Saved.' line under the button as the tab version did.
- **The client has a `delete` method now** (`lib/api/client.ts`, a one-line union member and one method) for `DELETE /me/blocks/{user_id}`. Plan 04-02's scan "`grep -rn "'DELETE'" lib app components` prints nothing" and its "`lib/api/client.ts` not edited" now fail on purpose; update them.
- **Types for the summary, notifications and blocks are in `lib/api/summary.ts` and `lib/api/account.ts`**, not `lib/api/types.ts` (that file held another plan's lines; move them when it is clean).
- The paper's `lib/diary/useSummary.ts` is `components/profile/useProfileSummary.ts`; its `lib/api/diary.ts` summary types are `lib/api/summary.ts`. The Diary build should import from there.
- Plan 04-02 scans that name `components/settings`, `lib/settings` and the Settings route files as "never edited" are no longer true (decision D-16 edited and removed files there).

## Waiting for other work

| What | Waiting for | Where it plugs in |
|---|---|---|
| Real figures, tastes, recent rows | Plan 04-01, `GET /v1/me/summary` `{spins, week_streak, friends, tastes, recent}` | Nothing to change in the app; the Profile page already reads it (404 now) |
| A recent row opens its entry | Phase 3's `EntryView` and the thin route `app/(app)/profile/entry.tsx` with `from: 'profile'` | pass `onOpenEntry` in `ProfileScreen` |
| A taste box opens the Diary on that category | the Diary build (`lib/diary/diaryStore.ts` or equal) | pass `onSelectTaste` in `ProfileScreen` (and set the hint to 'Opens your diary on {name}.') |
| Notifications switches really save | server routes `GET` and `PATCH /v1/me/notification-settings` (built after Phase 3's server) | none; until then a flipped switch goes back with a red line (the specified failure) |
| Blocked list has people, Unblock works | `GET /v1/me/blocks` and `DELETE /v1/me/blocks/{user_id}`; the block action itself is Phases 5 and 6 | none |
| Friends figure is real | Phase 5 | none (0 is printed as sent) |

## Stale papers to fold in (not edited here, outside the allowed list)

`04-CONTEXT.md` (the "Settings holds Edit profile, Username..." sentence), `04-02-PLAN.md` (Task 5 steps 7c and 10, the `components/settings` "never edited" scans, the `'DELETE'` scan, the gear "in the photo row" wording), `04-VALIDATION.md` if it lists the Settings rows. `02.1-02-PLAN.md` and `02.1-VALIDATION.md` still describe the first Settings.

## Device steps (short)

0. If the phone shows Welcome (a reload can do that), log in again: Welcome, Get started, the outlined Log in button.
1. Tap Profile. Check: photo circle (a camera if none), name beside it, @username, outlined 'Edit', a small gear above at the right, your bio under it (or 'Add a bio'), then 0, 0, 0 for Spins, Week streak and Friends, 'YOUR TASTES', 'RECENT ACTIVITY' and 'See diary'. Zeros are expected until the summary route is built.
2. Tap your bio (or 'Add a bio'), type a few words, tap Save: the text shows in place. Tap it again, change it, tap Cancel: the old text stays.
3. Tap 'See diary': the Diary tab opens.
4. Tap 'Edit': a page with Back, 'Edit your picture' under the photo and a grey 'Save changes'. Tap 'Edit your picture': Take photo, Choose from library, Cancel. Type a taken username: a red line with choices. Type a free one: green 'Looks good.'. Change the name, tap 'Save changes': you are on the Profile page, 'Saved.' shows top left for 3 seconds and the new name shows.
5. Tap the gear: Settings with Notifications, Blocked accounts, Private account, Password, Log out. No bio, no Edit profile, no Username.
6. Tap Notifications: three switches, all ON. Flip one: until the server routes exist it flips back with a red line (expected, not a bug).
7. Back, tap Blocked accounts: "You haven't blocked anyone."
8. Back. Password and the Private account switch work as before. Do not tap Log out.
9. First run (optional, new throwaway sign-up): after page 1 the photo-and-bio page still has no bar and no gear; Skip for now lands on Today with the bar.
