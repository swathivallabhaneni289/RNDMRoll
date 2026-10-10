---
phase: 6
slug: public-global
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-10
---

# Phase 6: Validation Map (Public Accounts, Global and the 8 PM reminder)

> How each behaviour is proven. PHASE 6. NOTHING IS BUILT. Written on paper on 2026-10-10 against the Phase 5 CONTEXT, UI spec and plans (05-01 to 05-06) and the Phase 2 to 4 plans, not built code: package and file names for Phase 5 parts are those plans' names, and the build corrects them from the Phase 5 summaries. Go tests run only on a database whose name ends in `test` (`rndmroll_test`), with `-p 1`. There is no JS test runner (two throwaway node scripts, `scripts/check-global.ts` and `scripts/check-reminder.ts`, check the pure rules), so screen checks are device steps in `06-03-PLAN.md` Tasks 3 and 5 ('group done' or what looked wrong). There are no pictures for this phase: the strings approval in Task 5 is the design check.

## Commands

| What | Command |
|---|---|
| Pure rules, no database | `go test ./internal/clock ./internal/account ./internal/social ./internal/board -count=1` (package paths as Phase 5 built them) |
| Full suite | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v \| grep -c '^--- SKIP'` prints 0 |
| App types | `npx tsc --noEmit` |
| Global logic | `node --experimental-strip-types scripts/check-global.ts` prints no failure and the line 'report reasons match the server list' (before plan 06-01 exists it prints 'not compared yet' instead, which is fine then but not at the gate) |
| Reminder logic | `node --experimental-strip-types scripts/check-reminder.ts` prints no failure |
| Earlier scripts | `for f in scripts/check-*.ts; do node --experimental-strip-types "$f" \|\| echo "FAIL $f"; done` prints no FAIL |
| App bundle | `npx expo export --platform ios --output-dir $(mktemp -d)` (after 06-03 Task 1 the bundle holds `expo-notifications`) |
| Cold launch | `xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`, with the first frames recorded |
| Emoji allow-list | `LC_ALL=C grep -rlE --exclude-dir=lab $'\xf0\x9f\|\xe2[\x98-\x9e]' app components lib scripts` prints exactly `lib/entry/reactions.ts`; the same scan over `internal cmd migrations` prints nothing |
| Radius allow-lists | the `radius.lg` list is Phase 5's close list; the `radius.full` list is Phase 5's close list (Phase 4's five, then `Avatar`, `NameChip`, `ReactionRow`, `CommentBar`) plus exactly `components/board/ViewSwitch.tsx`: `grep -rln --exclude-dir=lab "radius\.lg" app components lib` and `"radius\.full"`; neither appears in `components/reminder` or `app/(app)/board` |
| Route listings | `ls "app/(app)"` is unchanged from Phase 5's close; `ls "app/(app)/board"` lists Phase 5's files plus exactly `person.tsx`; `ls migrations` lists exactly 0001 to 0007 (fourteen files) |
| No DELETE in the client | `grep -rn "'DELETE'" lib app components` prints nothing |
| Key, not link | `grep -rn "photo_url" internal/account internal/store \| grep -v _test` prints nothing |
| One visibility function | `grep -rniE "AreFriends\|OwnerPublic\|IsBlocked" internal/httpapi \| grep -v _test` prints nothing |
| Phone computes no age | `grep -rn "can_be_public" app components lib` prints only `lib/api/account.ts`, `lib/account/` and `components/profile/AccountSettings.tsx` |
| No stranger variant | `grep -rn "comments === null\|comments == null\|comments !== null\|comments != null" app components lib` prints nothing |
| One new dependency | `git diff pre-phase6 -- package.json` shows exactly one added dependency, `expo-notifications`; `git diff pre-phase6 -- app.json` shows exactly one added plugin string |
| Guarded import | `grep -rn "expo-notifications" app components lib scripts \| grep -v "lib/reminder/native.ts"` prints nothing |
| No push token | `grep -rn "getExpoPushTokenAsync\|getDevicePushTokenAsync" app components lib` prints nothing |

Never run `expo lint`. Never run Go tests on `rndmroll_dev`.

## Map

| Requirement | Behaviour | Proof | Where |
|---|---|---|---|
| ACCT-04 | An account has one setting, Public or Private. Private is the default and a missing row means Private. A repeat change gives the same answer | Go suite (both repos), HTTP tests | 06-01 Tasks 2, 3 |
| ACCT-04 | An under-18 account can never be Public: the age is a civil-date rule (including leap-day births and the case where adding 18 years to a leap day lands a day early), refused with 403 and nothing stored; the day before the 18th birthday is refused and the day itself accepted | Go table tests, suite with the fixed clock, HTTP test with a 17 year old made through the API | 06-01 Tasks 1, 2, 3 |
| ACCT-04 | Even a row that says public never lists or opens an under-18 owner (stale row, direct write) | Postgres test with a hand-written row, HTTP test | 06-01 Tasks 2, 3 |
| ACCT-04 | Turning Public on makes past posts visible, and going Private removes them at once | Go suite, HTTP test | 06-01 Tasks 2, 3 |
| ACCT-04 | Turning on asks first with the consent sheet; turning off does not; under 18 sees a disabled switch and the line; a failure snaps back | Check script (`publicRowState`, `nextVisibilityAction`, strings), walkthrough steps 1 to 4 and 16 | 06-02 Task 1; 06-03 Task 5 |
| ACCT-04 | The phone computes no age: it only reads `can_be_public` | The read-only grep | 06-02 Task 2; 06-03 Task 4 |
| ACCT-04 | The test reset returns an account to Private and clears its reports | Go suite, HTTP test | 06-01 Tasks 2, 3 |
| FEED-06 | One visibility function decides every read of a post, a card, a reaction, a comment and a photo link; its 32 input combinations match a hand-written table; the function, the post fetch and the list queries agree on a fixture matrix | Table test, equivalence test (Postgres), the one-function grep | 06-01 Tasks 1, 2, 3 |
| FEED-06 | Global lists Public adults' posts newest first, 20 a page, with keyset paging that repeats and skips nothing even when posts share one instant; limit 1 to 40; a bad cursor, limit or view is 400 | Go suite, HTTP tests | 06-01 Tasks 1, 2, 3 |
| FEED-06 | A Private account can open Global; Global is not behind the post-first rule; the viewer's own public posts appear as 'You' | HTTP test, walkthrough steps 5 to 7 | 06-01 Task 3; 06-03 Task 5 |
| FEED-06 | A Public friend is visible on Global without the gate; a private friend's post stays behind the gate | Equivalence matrix | 06-01 Task 2 |
| FEED-06 | A blocked person (either way) is never listed, opened or fetched; a post you reported leaves your Friends, Global and person lists and stays for others | Go suite, HTTP tests, walkthrough steps 10 and 11 | 06-01 Tasks 2, 3; 06-03 Task 5 |
| FEED-06 | Comments go through the same visibility function as the post: whoever can see a public post (a stranger, or a friend who has not posted that day) reads and writes its comments; a blocked pair can do neither; a private non-friend's post, its comment list and a comment create are 404; `comments`, `comment_count` and `can_comment` are never null; the owner can delete any comment on their post and a writer their own | HTTP tests, Go suite, the response scan, walkthrough steps 6 and 7 | 06-01 Tasks 2, 3; 06-03 Tasks 4, 5 |
| FEED-06 | A comment written by a blocked person is absent from the viewer's comment list and count | Go suite, HTTP test | 06-01 Tasks 2, 3 |
| FEED-06 | Phase 5's derived notification list does not need friendship: a stranger's reaction or comment on my public post notifies me (and lights the bell), and a blocked actor's items are absent; friend request and accepted items are unchanged | Plan 05-03's shared suite with the new case, one HTTP test, the start-rule check of the built queries | 06-01 Tasks 2, 3 |
| FEED-06 | A reaction is allowed on any visible post and refused on your own | HTTP tests | 06-01 Task 3 |
| FEED-06 | Photo links come only from the single link function and only for a post the viewer may see; no response carries `photo_key` | Response-scan test, the key-not-link grep | 06-01 Tasks 2, 3 |
| FEED-06 | The person page: 404 for a private stranger, a blocked pair, an unknown id, a non-UUID and yourself, with identical bodies; `posts` only when the person is Public; `relation` is each of its four values | Go suite, HTTP tests, walkthrough step 9 | 06-01 Tasks 2, 3; 06-03 Task 5 |
| FEED-06 | The switch, the Global grid and its states (loading, empty, offline, failed, paging failed), paging merge without repeats; the name chip and the author row open the person page; your own chip is a label | Check script (`mergePage`, `footerState`, `emptyLine`), walkthrough steps 5, 6 and 9 | 06-02 Tasks 1, 3; 06-03 Task 5 |
| MOD-01 | Report a post: four reasons, one per person per post, only on a post you can see and do not own; an unknown reason is refused; a repeat is a 200 and stores nothing new | Go suite, HTTP tests | 06-01 Tasks 2, 3 |
| MOD-01 | Report a comment: three reasons (`not_public` is refused), one per person per comment, only on a comment whose post you can see and that you did not write (409 `own_comment`); a repeat is a 200 and stores nothing new; the comment leaves the reporter's list and count only | Go suite, HTTP tests, the cutover smoke test | 06-01 Tasks 2, 3; 06-03 Task 4 |
| MOD-01 | A reported post or comment leaves the reporter's lists at once; reports are rate limited; nothing happens automatically and the developer can read both report tables with one query each | HTTP tests, the cutover smoke test, the STATE note | 06-01 Task 3; 06-03 Tasks 4, 6 |
| MOD-01 | One report sheet with four reasons for a post and one with three reasons and Block for a comment, no free text; the thanks line; block from the post page, the person page and the comment sheet (a second, confirming step) uses Phase 5's action and removes the person's cards and comments at once | Check script (`REPORT_REASONS`, `COMMENT_REPORT_REASONS`, `commentSheetRows`, `removeComment`), walkthrough steps 8, 10 and 11 | 06-02 Tasks 1, 3; 06-03 Task 5 |
| MOD-01 | A comment row shows 'Delete' when the server's `can_delete` is true and 'Report' otherwise, never both, and the phone tests no friendship; the only ownership result it uses is the one `CommentRow` already computes for 'You' | The no-stranger-variant and `can_delete` greps, walkthrough steps 7 and 8 | 06-02 Task 2; 06-03 Task 5 |
| MOD-01 | Delete and block: the delete sheet of a comment that is not yours has a second row 'Delete and block {first name}' and your own comment's sheet has two rows; the app deletes first and then blocks, sends no block after a failed delete, and after a failed block leaves the comment deleted and says so with a retry that sends only the block; the server has no new route for it and the two calls in order work | Check script (`commentSheetRows`, `deleteAndBlockOutcome`), HTTP test of the two calls in order and of a failed block after a delete, the cutover smoke test, walkthrough step 15 | 06-01 Task 3; 06-02 Tasks 1, 2; 06-03 Tasks 4, 5 |
| NOTIF-02 | The reminder is a LOCAL daily notification at 20:00 on the phone's clock, with fixed words and no category word, and no push token is ever requested | Check script (`reminderContent`, constants), the two greps, the minute-ahead device proof | 06-03 Tasks 1, 3 |
| NOTIF-02 | It is asked once, after the first posted day; 'Not now' is final; a granted or denied permission never asks | Check script (`shouldAsk`), walkthrough steps 12 to 14 | 06-03 Tasks 1, 5 |
| NOTIF-02 | The schedule never doubles: cancel all, then at most one, on every Today open and foreground return | Check script (`syncAction`), device proof: one banner after three foreground returns | 06-03 Tasks 1, 3 |
| NOTIF-02 | The row is hidden when the module is missing, off and dimmed with the Settings line when the permission is denied, and works when allowed | Check script (`rowState`), the old-client launch, walkthrough steps 13 and 14 | 06-03 Tasks 1, 2, 3, 5 |
| NOTIF-02 | The banner is not shown while the app is open; a tap on it opens Today | Device proof | 06-03 Task 3 |
| NOTIF-02 | Log out cancels the reminder; the values are kept per account | Check script (`storageKey`), code reading of the one Log out line | 06-03 Tasks 1, 2 |
| NOTIF-02 | The app starts and behaves as at the end of plan 06-02 on a dev client that lacks the native module | Old-client cold launch | 06-03 Tasks 2, 3 |
| Safety | No Phase 6 response carries `photo_key`, birthday, email, a password hash or a friend code; a `Person` has exactly `id`, `name`, `username`, `photo_url` | HTTP response-scan tests | 06-01 Task 3 |
| Safety | Ids cannot be probed: a post, person or private stranger that is not visible answers 404 with the same body as a missing one | HTTP tests | 06-01 Task 3 |
| Safety | Every route refuses no token (401), a deleted user (401), an unfinished account (403), a body over 16 KB (413), the 11th change or report in a minute (429) | HTTP tests | 06-01 Task 3 |
| Safety | Migration 0007 is reversible and the earlier migration tests start and restore at the latest version | `TestMigrations_0007IsReversible`, the other migration tests reading `latestMigration` | 06-01 Task 2 |
| Safety | The users table, earlier migrations and the four real accounts are untouched | `git diff --stat pre-phase6` on those paths, user count before and after | 06-03 Task 4 |
| Safety | The dev database moves from 6 to 7 with a dump, a binary backup, a rehearsal on a throwaway copy and a rollback | Cutover steps | 06-03 Task 4 |
| Safety | Test tools only for throwaway emails, refused outside `APP_ENV=test` | The 9091 refusal check | 06-03 Task 4 |
| Safety | One new dependency and one rebuild, with the installed app and `ios/` backed up and a written rollback | The package.json and app.json diffs, the backup listing, Task 3 | 06-03 Tasks 1, 3 |
| Launch | The first screen is still Today with no Unmatched Route or Welcome flash; the bar has five items; no launch file is edited; the person route is only in the Board stack | Cold launches (a) and (b) before and after the rebuild, `git diff` on the launch files, the `ls` listings | 06-02 Task 3; 06-03 Tasks 3, 4 |
| Copy | No em dash, no 'roll' on screen, tokens only, no new use of the 24dp or round radius tokens, the phone computes no date or age | Scans | 06-03 Task 4 |
| Copy | Developer approves the strings | Section 14 rows 1 to 11 of the UI spec and the listed choices | 06-03 Task 5 |
| Risk | Strangers' names, usernames and photos are visible to every signed-in person once an account is Public, and strangers can comment on public posts; the reports reader, the Terms and a contact address are not decided | Recorded as launch-list items in STATE; accepted and flagged | 06-03 Task 6 |
| Risk | Photo links are public until the real storage account exists; reports of a deleted post or comment vanish with it | Recorded; accepted and flagged | 06-01; 06-03 Task 6 |
| Risk | A real notification shows on the simulator once, at the right time | The minute-ahead proof; the result and the trigger shape go in the summary | 06-03 Task 3 |
| Visual | Each screen matches its picture: the phone screenshot is compared side by side with the picture, and every difference is in the UI spec's choices list or is fixed (the developer's rule of 2026-10-10: exactly the same visuals) | Side-by-side screenshot check per screen, differences listed in the task summary | 06-02 and 06-03 (each screen task and the walkthrough) |

## Sign-off

- [ ] Every row proven
- [ ] The Phase 6 criteria in ROADMAP are ticked in the close-out
- [ ] `nyquist_compliant` set to true by the close-out
