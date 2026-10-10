---
phase: 4
slug: personal-diary
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-10
---

# Phase 4: Validation Map (Personal Diary and the Profile page)

> How each behaviour is proven. PHASE 4. NOTHING IS BUILT. Written on paper on 2026-10-10 against the Phase 2 plans (UI spec revision 6), the Phase 2.1 Settings plans (commit 0557d19) and the Phase 3 plans (UI spec revision 2), for the developer's pictures of screens 07 and 12. Go tests run only on a database whose name ends in `test` (`rndmroll_test`), with `-p 1`. There is no JS test runner (a throwaway node script, `scripts/check-diary.ts`, checks the pure label, filter and number logic), so screen checks are device steps in `04-02-PLAN.md` Task 5 ('group done' or what looked wrong).

## Commands

| What | Command |
|---|---|
| Pure rules, no database | `go test ./internal/diary ./internal/clock ./internal/spin ./internal/storage -count=1` |
| Full suite | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v \| grep -c '^--- SKIP'` prints 0 |
| Query plans (read by a person) | `DIARY_PLANS=1 TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/store/postgres -run TestDiaryPlans -p 1 -count=1 -v` |
| App types | `npx tsc --noEmit` |
| Diary logic | `node --experimental-strip-types scripts/check-diary.ts` prints no failure; Phase 2's `scripts/check-wheel.ts` and Phase 3's `scripts/check-entry.ts` still print no failure |
| App bundle | `npx expo export --platform ios --output-dir $(mktemp -d)` |
| Cold launch | `xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`, with the first frames recorded |
| Emoji allow-list | `LC_ALL=C grep -rlE --exclude-dir=lab $'\xf0\x9f\|\xe2[\x98-\x9e]' app components lib scripts` prints exactly `lib/entry/reactions.ts`; the same scan over `internal cmd migrations` prints nothing |
| Radius allow-lists | `grep -rln --exclude-dir=lab "radius\.full" app components lib` prints exactly `components/brand/DialAvatar.tsx`, `components/nav/SpinTabButton.tsx`, `components/ui/Chip.tsx`, `components/ui/IconBadge.tsx`, `lib/theme/tokens.test-assert.ts`; `"radius\.lg"` prints the same five files as at Phase 3's close; neither appears in `components/diary` or `app/(app)` |
| Route listings | `ls "app/(app)"` lists exactly `_layout.tsx`, `diary`, `profile`, `spin.tsx`, `today`; `ls "app/(app)/diary"` lists exactly `_layout.tsx`, `edit-entry.tsx`, `entry.tsx`, `index.tsx`; `ls "app/(app)/profile"` lists exactly `_layout.tsx`, `edit-entry.tsx`, `edit.tsx`, `entry.tsx`, `index.tsx`, `password.tsx`, `settings.tsx`, `username.tsx` (the last three are Phase 2.1's); `ls migrations` lists exactly 0001 to 0006 (twelve files; 0005 is Phase 2.1's, 0006 is Phase 3's), or 0001 to 0007 (fourteen files) when 04-01 proved an index was needed |
| One place hides the bar | `grep -rn "tabBarStyle" app components` prints only `app/(app)/_layout.tsx` (the bar's own style and the first-run hide) and `components/nav/useHideTabBar.ts` |
| Friends number is not a control | `grep -n "Pressable\|onPress" components/profile/StatsRow.tsx` prints nothing |
| Phase 2.1's pieces are mounted, not edited | `git diff --stat pre-phase4 -- components/settings lib/settings "app/(app)/profile/settings.tsx" "app/(app)/profile/username.tsx" "app/(app)/profile/password.tsx"` prints nothing; `grep -rln "SettingsGear\|BioInline" app components \| grep -v "^components/settings/"` prints only `components/profile/ProfileScreen.tsx` and `components/profile/ProfileForm.tsx` |
| No DELETE in the client | `grep -rn "'DELETE'" lib app components` prints nothing |
| Key, not link | `grep -rn "photo_url" internal/spin internal/store internal/diary \| grep -v _test` prints nothing |
| No lock in reads | `grep -rn "InUserTx" internal/diary internal/store/postgres/diary_repo.go` prints nothing |
| No phone-side date | `grep -rn "new Date\|Date.now\|Intl\.\|toLocale" lib/diary components/diary components/ui/Chip.tsx components/profile/ProfileScreen.tsx components/profile/StatsRow.tsx components/profile/TastesSection.tsx components/profile/RecentActivity.tsx` prints nothing |

Never run `expo lint`. Never run Go tests on `rndmroll_dev`.

## Map

| Requirement | Behaviour | Proof | Where |
|---|---|---|---|
| DIARY-01 | The diary lists the caller's own entries newest first, one row per date; a deleted entry and a pretend posted day make no row | Go suite on both repositories, HTTP test | 04-01 Tasks 2, 3 |
| DIARY-01 | Each row carries the category name as written the day it was posted; a rename or a removal never rewrites it | Go suite (renamed and removed category), walkthrough step 2 | 04-01 Task 2; 04-02 Task 5 |
| DIARY-01 | A skipped day is a quiet row (date and the word 'Skipped'), only without a category filter; a missed day shows nothing | Go suite, check script (`skippedLabel`), walkthrough steps 2, 3 | 04-01 Task 2; 04-02 Tasks 1, 5 |
| DIARY-01 | A late entry shows 'Late' after its category; an on-time one shows nothing | Go suite (`late` from the posted day), check script (`rowLabel`), walkthrough step 2 | 04-01 Task 2; 04-02 Tasks 1, 5 |
| DIARY-01 | Paging: 20 a page, a cursor that is the last date, no duplicate, no gap, no empty extra page at an exact multiple; a bad cursor or limit is refused with its field named | Go table tests of cursor and limit, Go suite, HTTP tests, walkthrough step 4 | 04-01 Tasks 1, 2, 3; 04-02 Task 5 |
| DIARY-01 | The row shows the small photo, 'Oct 8 \u00B7 Meal', the title, the stars with one decimal, and the author's note in quotes; no reaction and no month heading | Check script (`rowLabel`, `starsFigure`), walkthrough step 2 | 04-02 Tasks 1, 5 |
| DIARY-01 | A tap opens Phase 3's entry page from the Diary stack with no bar and the Back label 'Diary'; Edit and Delete work and come back to a list that reloads | Route listings, one-place-hides-the-bar scan, walkthrough steps 5, 6 | 04-02 Tasks 3, 5 |
| DIARY-01 | Empty, empty for a category, loading and offline states say the plain lines of the spec; 'Go to Today' goes to Today | Check script (`emptyKind`, `emptyLine`), cold launch offline case, walkthrough Group 4 | 04-02 Tasks 1, 3, 5 |
| DIARY-02 | The chips are All, then categories with at least one entry in wheel order, then removed ones with their last name; a category with no entry has no chip | Go suite, check script (`chipOptions`, `showChips`), walkthrough step 3 | 04-01 Task 2; 04-02 Tasks 1, 5 |
| DIARY-02 | Filtering is by category id: a renamed or removed category still filters its old entries; another person's id gives an empty page | Go suite, HTTP test | 04-01 Tasks 2, 3 |
| DIARY-02 | The chosen chip is kept while the app is open, starts at All after a relaunch, and a tap on a taste or on 'See diary' sets it | Walkthrough steps 3, 8 | 04-02 Task 5 |
| DIARY-03 | The summary: days spun (a bonus day once), week streak (current or last week, consecutive), tastes (average to one decimal and count), the newest three entries, friends 0; no zone gives streak 0 and no error | Go table tests (average, streak), Go suite, HTTP test | 04-01 Tasks 1, 2, 3 |
| DIARY-03 | The Profile page shows the photo, name, @username, Edit, Phase 2.1's gear and bio in place, the three numbers (Spins, Week streak, Friends), tastes, recent activity and 'See diary'; there is no Log out on it | Check script (spoken strings), walkthrough steps 7, 7b, 7c | 04-02 Tasks 1, 5 |
| DIARY-03 | The gear (far right of the photo row, right of the Edit chip, both centred on the photo) and the bio in place (under the row, label off) are Phase 2.1's components mounted as they are, drawn in every state; the gear opens Settings and the bio saves in place and shows the new text | Mount scan and `git diff` on `components/settings` and `lib/settings`, walkthrough steps 7b, 7c, 10 | 04-02 Tasks 2, 4, 5 |
| DIARY-03 | Log out is on Settings (Phase 2.1's own row) and at the bottom of the form page (Phase 1's); the Profile page has none | Code read of `ProfileScreen`, walkthrough steps 7c and 10 (looked at, not tapped) | 04-02 Tasks 3, 5 |
| DIARY-03 | The Friends number is plain text and shows 0 | Friends-number scan, walkthrough step 7 | 04-02 Tasks 2, 5 |
| DIARY-03 | A taste opens the Diary on that category; a recent post opens the entry page with the Back label 'Profile' | Walkthrough steps 8, 9 | 04-02 Task 5 |
| DIARY-03 | Edit (and Settings' 'Edit profile' row) opens Phase 1's form with a Back control and a grey 'Save changes' (no 'Continue', no gear); a save goes back to the page that opened it with 'Saved.' from Phase 2.1's notice store; the Bio box and the bio in place are the same field | Walkthrough steps 7c, 10 | 04-02 Tasks 3, 5 |
| DIARY-03 | First run is unchanged: the photo-and-bio page after a sign-up, no bar, Continue or Skip for now end on Today; a brand-new account's Profile and Diary say the plain 'Nothing here yet' line | Cold launch case (c), walkthrough Group 4 | 04-02 Tasks 3, 5 |
| Bar | The bar is Home, Diary, Spin, Profile; the Diary icon is the outline document glyph; Spin is third of four; the bar is hidden on the first-run page and on entry pages and shown on the Edit page | Cold launches, route listings, walkthrough steps 1, 5, 10 | 04-02 Tasks 3, 4, 5 |
| Refactor | Phase 3's entry page, edit and delete work from Today exactly as before after the three-stack refactor | Phase 3's check script, `git diff` read of Task 3, walkthrough step 11 | 04-02 Tasks 3, 5 |
| Safety | Another person's entries, chips, numbers and photo links are never returned, with any query values | Go suite and HTTP tests with two accounts | 04-01 Tasks 2, 3 |
| Safety | No token 401, a deleted user 401, an unfinished account 403, the 61st call in a minute 429, a bad query value 400 naming the field | HTTP tests | 04-01 Task 3 |
| Safety | No diary or summary response carries `photo_key`, birthday, email, a password hash or `account_visibility`; photo links are built only in the HTTP layer | HTTP response-scan test, key-not-link scan | 04-01 Task 3; 04-02 Task 4 |
| Safety | Reads take no per-person lock, so a held spin or post transaction never blocks the diary | Postgres test with a held `InUserTx`, no-lock scan | 04-01 Task 2 |
| Safety | A query on a long history does not scan whole tables; an index is added only if the printed plans show one is needed | Query-plan run read by a person, `ls migrations` | 04-01 Task 2 |
| Safety | This phase adds nothing to the users table (Phase 2.1's column came first), leaves earlier migrations alone, and the four real accounts are untouched | `git diff --stat pre-phase4` on those paths, user count before and after | 04-02 Task 4 |
| Safety | The cutover has a binary backup and a rollback; no data is changed (or only the one index) | Cutover steps | 04-02 Task 4 |
| Safety | No new native module | `git diff --stat pre-phase4 -- package.json package-lock.json app.json ios android` prints nothing | 04-02 Task 4 |
| Launch | The first screen is still Today with no Unmatched Route or Welcome flash; the only launch files changed are the Tabs layout and the Profile route; the bar has four items | Cold launches (a), (b), (c), `git diff` on the launch files, `ls` listings | 04-02 Tasks 3, 4 |
| Copy | No em dash, no 'roll' on screen, tokens only, exactly one new file with the round radius, one new text role, no new colour or space, and the phone computes no date | Scans | 04-02 Task 4 |
| Copy | The developer approves the strings and the listed choices | Section 10 rows 1 to 8 of the UI spec and the choices list | 04-02 Task 5 |
| Risk | The note line may draw upright (the serif has no italic) | Walkthrough step 2, the answer goes in the summary | 04-02 Task 5 |
| Visual | Each screen matches its picture: the phone screenshot is compared side by side with the picture, and every difference is in the UI spec's choices list or is fixed (the developer's rule of 2026-10-10: exactly the same visuals); the gear and the bio, which picture 12 does not draw, are in that list as asked for in words | Side-by-side screenshot check per screen, differences listed in the task summary | 04-02 Task 5 |

## Sign-off

- [ ] Every row proven
- [ ] The Phase 4 criteria in ROADMAP are ticked in the close-out
- [ ] `nyquist_compliant` set to true by the close-out
