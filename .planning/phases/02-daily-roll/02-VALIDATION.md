---
phase: 2
slug: daily-roll
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-08
---

# Phase 2: Validation Map (Daily Roll, "the spin")

> How each behaviour is proven. Nothing is built yet. Rebased on the final Phase 1 code (main tree commit ead0c70) on 2026-10-09. Revised later that day: Phase 1 removed Apple and Google sign-in completely, so the database is at version 3 before Phase 2 starts and Phase 2's migration is 0004. Revised on 2026-10-10 (UI-SPEC revision 5): the Today screen follows the developer's new picture, `docs/reference/today-screen-1a-1b-2026-10-10.png`, layout 1a, so the wheel tones, the bell and its page, the Spin button looks, the week section and the type rows below changed. Revised again on 2026-10-10 (revision 6): after the spin the wheel is gone and the word fills the screen with the time it was spun (`docs/reference/today-after-spin-02-2026-10-10.png`), the server writes that time and the next open's weekday, and three more text roles were added. Go tests run only on a database whose name ends in `test` (`rndmroll_test`), with `-p 1`. There is no JS test runner (a throwaway node script checks the pure wheel logic), so screen checks are device steps in `02-03-PLAN.md` Task 3 ("group done" or what looked wrong).

## Commands

| What | Command |
|---|---|
| Pure rules, no database | `go test ./internal/clock ./internal/spin ./internal/config -count=1` |
| Full suite | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v \| grep -c '^--- SKIP'` prints 0 |
| App types | `npx tsc --noEmit` |
| Wheel logic | `node --experimental-strip-types scripts/check-wheel.ts` prints no failure |
| App bundle | `npx expo export --platform ios --output-dir $(mktemp -d)` |
| Cold launch | `xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`, with the first frames recorded |
| Radius allow-lists | `grep -rln --exclude-dir=lab "radius\.lg" app components lib` prints exactly five files and `grep -rln --exclude-dir=lab "radius\.full" app components lib` exactly four: `components/ui/IconBadge.tsx`, `components/brand/DialAvatar.tsx`, `components/nav/SpinTabButton.tsx` (the Spin button's circle, new in revision 5) and `lib/theme/tokens.test-assert.ts` (the lists are in `02-03-PLAN.md` Task 2) |
| Token pins | `grep -nE "wheelMid\|type\.(title\|countdown\|wheelLabel\|hero\|heroSmall\|note)" lib/theme/tokens.test-assert.ts` prints thirteen pin lines (one colour, a family pin and a size pin for each of six type roles) |
| Placeholder page gone | `ls "app/(app)"` lists exactly `_layout.tsx`, `profile`, `spin.tsx`, `today`, and `grep -rn --exclude-dir=lab -E "'/home'\|\"/home\"" app components lib` prints nothing |
| Today routes | `ls "app/(app)/today"` lists exactly `_layout.tsx`, `edit-wheel.tsx`, `index.tsx`, `notifications.tsx` |
| No phone-side time | `grep -rnE "toLocaleTimeString\|toLocaleDateString\|getHours\|getDay\(" components lib/wheel app --exclude-dir=lab` prints nothing |

Never run `expo lint`. Never run Go tests on `rndmroll_dev`.

## Map

| Requirement | Behaviour | Proof | Where |
|---|---|---|---|
| ROLL-01 | Window opens 8:00 PM local, closes at local midnight, in each zone, across DST | Go tests: New York, London, Kolkata, Kathmandu, Lord Howe, Kiritimati, Apia | 02-01 Task 1 |
| ROLL-01 | Zones whose midnight is skipped or repeated (Havana, Cairo, Santiago, Apia's skipped day) and every day of 2026 | Go table test | 02-01 Task 1 |
| ROLL-01 | A zone change cannot buy a second spin inside 12 hours | Go suite (Kiritimati to Pago Pago, Tokyo to Los Angeles) | 02-01 Task 2 |
| ROLL-01 | One spin per user per local date, also with 20 callers at once | Go suite on Postgres, concurrency test | 02-01 Task 2 |
| ROLL-01 | A retry or a second phone gets the same spin | Go suite and HTTP test | 02-01 Tasks 2, 3 |
| ROLL-01 | Before 8 PM, after midnight: refused with the right message | HTTP tests (`spin_not_open`, `spin_closed`) and curl in the walkthrough | 02-01 Task 3, 02-03 Task 3 |
| ROLL-01 | Server picks by weights | Fixed-seed statistics, 120000 draws within 1 point | 02-01 Task 1 |
| ROLL-01 | Late state after midnight | `BuildToday` test and walkthrough group 5 | 02-01 Task 1, 02-03 Task 3 |
| ROLL-01 | The stamp time and the next-open weekday are written by the server (the stamp in the zone the spin was made in) and printed as sent; the phone formats no time or weekday | Go tests (`spun_local_time` "8:02 PM" and "12:05 AM", a half-hour and a 45-minute zone, the spin's own zone after a zone change; `next_open_weekday` for New York and Los Angeles, where it differs from UTC), the HTTP full-day test, and the no-phone-side-time grep | 02-01 Tasks 1, 3; 02-02 Task 2; 02-03 Task 2 |
| ROLL-02 | Bonus earned the moment 4 days are posted, one held at most, carried over | Go suite with `MarkPosted`, concurrency test | 02-01 Task 2 |
| ROLL-02 | Bonus spin changes the category and is spent once | Go suite (200 repeats), retry test, HTTP test | 02-01 Tasks 2, 3 |
| ROLL-02 | Refused once the day is posted (skipped comes with Phase 3) | Go suite (`day_settled`, one `daySettled` rule) and walkthrough Group 4 | 02-01 Task 2, 02-03 Task 3 |
| ROLL-02 | A stale screen or a retry across midnight never spends or loses a bonus | Go suite (`for_date`, `after_seq`) | 02-01 Task 2 |
| ROLL-02 | Earning can be shown without real posts | Test tools, walkthrough group 4 | 02-01 Task 3, 02-03 Task 3 |
| ROLL-03 | Six starting categories, made once on first use | Go suite, 10 first callers | 02-01 Task 2 |
| ROLL-03 | Add, rename, reweight, archive, limits 2 to 12, weight 1 to 10, name rules | Go suite and HTTP tests; walkthrough groups 6 and 7 | 02-01, 02-03 |
| ROLL-03 | Old spins keep the name they had | Go suite | 02-01 Task 2 |
| ROLL-03 | Another person's wheel is never reachable | HTTP tests (`unknown_id`, two accounts) | 02-01 Task 3 |
| ROLL-03 | Stale version (409 `wheel_stale`), duplicate id and non-UUID id are refused and change nothing | Go suite and HTTP tests | 02-01 Tasks 2, 3 |
| ROLL-04 | Wedges are words only, two tones from tokens (`ink` and `secondary`) with `wheelMid` only on the last wedge of an odd count, neighbours differ including the wrap | Check script (`toneIndexes` for counts 2 to 12), scans (no hex, no emoji) and walkthrough Group 1 and Group 7 | 02-02 Tasks 1, 2; 02-03 Task 3 |
| ROLL-04 | Words on the wheel read across the wedge when they fit and along it when they do not, and never upside down in the pose the wheel stops in | Check script (`labelPlacement`) and walkthrough Group 1, Group 2 and Group 7 | 02-02 Task 1, 02-03 Task 3 |
| ROLL-04 | The word under the pointer is the word the server picked, for every count 2 to 12 and heavy weights | Check script and walkthrough Group 2 and Group 7 | 02-02 Task 1, 02-03 Task 3 |
| ROLL-04 | Turn of about 4 seconds, smooth slow-down, stops on the server's word | Walkthrough Group 2 | 02-03 Task 3 |
| ROLL-04 | Reduce Motion: no turning, word at once, right wedge under the pointer | Walkthrough Group 3 | 02-03 Task 3 |
| Today | The bell opens a Notifications page that says "Nothing new yet." and nothing else; the page has a back arrow and no tab bar, and the arrow returns to Today with the bar back (the real list is Phase 5) | Cold-launch case (b) of 02-02 Task 3, the `ls` check of `app/(app)/today` and walkthrough Group 1 | 02-02 Task 3, 02-03 Task 3 |
| Today | The Spin button is a round outline while the spin is locked (waiting, spinning, result, late) and solid `ink` only when the spin is open | Walkthrough Group 1 and Group 2 (outlined before 8:00 PM, solid when it opens, outlined again after the spin) | 02-02 Task 2, 02-03 Task 3 |
| Today | No sub-line under the heading, so the heading and the wheel do not move when the state changes; the Edit wheel link sits last in the state block | Walkthrough Group 1 and Group 2 (watch the wheel when the spin opens) | 02-02 Task 2, 02-03 Task 3 |
| Today | After the spin the wheel is not drawn: the word fills the screen (`hero`, left aligned, one line shrinking to fit) with a rule and "Spun at {time}" under it; no Edit wheel link, no countdown, no kicker; the wheel fades out after the landing; reopening the app shows this at once with no animation | Walkthrough Group 2, Group 3 (Reduce Motion) and Group 5, and the `ResultBlock` and `TodayScreen` greps of 02-02 Task 2 | 02-02 Task 2, 02-03 Task 3 |
| Today | A bonus spin brings the wheel back (on the previous spin's own wheel), turns to the new word and gives way to it again; the stamp shows the second spin's time | Walkthrough Group 4 | 02-02 Task 2, 02-03 Task 3 |
| Safety | Test tools exist only in test mode | Config test, 404 test, start refusal on port 9091 (tools message in the output) | 02-01 Task 1, 3; 02-03 Task 1 |
| Safety | Test tools answer 404 for a caller not on the allow-list; one account's clock move changes no other | HTTP tests | 02-01 Task 3 |
| Safety | Every Phase 2 route refuses no token (401), deleted user (401), unfinished account with no name or username (403), body over 16 KB (413), 11th spin in a minute (429); no response carries birthday, email or password hash | HTTP tests | 02-01 Task 3 |
| Safety | Migration 0004 is reversible, and the older migration tests (0002 and 0003) still pass because they start and restore at the latest version read from the files | `TestMigrations_0004IsReversible`, the 0002 and 0003 tests | 02-01 Task 2 |
| Safety | Users table and four real accounts untouched | `git diff --stat` on user files, user count before and after | 02-03 Tasks 1, 3 |
| Launch | First screen is Today, no Unmatched Route, no Welcome flash, in these cases: signed out, signed in, expired token, log out then in | Cold launches with recorded first frames, right after 02-02 Task 3 and after the cutover | 02-02 Task 3, 02-03 Task 2 |
| Launch | Just signed up: the photo-and-bio page comes first with no bottom bar and no flash of Today; Continue or Skip for now ends on Today with the bar; a relaunch there shows Today | Recorded first frames (02-02 Task 3 case (c), 02-03 case (e)) and walkthrough Group 8b | 02-02 Task 3, 02-03 Tasks 2, 3 |
| Launch | Phase 1's placeholder page is gone: the bar shows exactly three items and nothing links to `/home` | `ls "app/(app)"` and a grep | 02-02 Task 3, 02-03 Task 2 |
| Launch | A second account never sees the first one's wheel | Cold launch case (d), walkthrough Group 9 | 02-03 Tasks 2, 3 |
| Launch | Time zone reaches the server from the phone | `select tz_name from user_time_zones` equals the Mac zone | 02-03 Task 2 |
| Profile tab | No Back, stays after Save with "Saved." for 3 seconds under Continue, no double bottom inset, Continue opens Today | Walkthrough Group 8 (bar and insets looked at on screen) | 02-02 Task 3, 02-03 Task 3 |
| Safety | The root and auth layouts and the session store are not edited | `git diff --stat pre-phase2` on them prints nothing | 02-03 Task 2 |
| Copy | No em dash, no "roll" on screen, tokens only, no pill, no emoji; the 24dp and round radius tokens are used only in the files listed in 02-03 Task 2 | Scans | 02-03 Task 2 |
| Copy | Six new type roles (`title`, `countdown`, `wheelLabel`, `hero`, `heroSmall`, `note`; the last two are used from Phase 3) and one font file (`Domine_400Regular`) are the only typographic additions; no new native module | Token pins (`tsc`), the `fonts.ts` and `AppText.tsx` greps of 02-02 Task 1, and `git diff pre-phase2 --stat` of `package.json` | 02-02 Task 1, 02-03 Task 2 |
| Copy | Developer approves the strings | Section 13 rows 1 to 11 and the listed choices | 02-03 Task 3 |

## Sign-off

- [ ] Every row proven
- [ ] The five Phase 2 criteria in ROADMAP are ticked in the close-out
- [ ] `nyquist_compliant` set to true by the close-out
