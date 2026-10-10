---
phase: 3
slug: daily-entry
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-10
---

# Phase 3: Validation Map (Daily Entry, "log it")

> How each behaviour is proven. PHASE 3. NOTHING IS BUILT. Written on paper on 2026-10-10 against the Phase 2 plans. Go tests run only on a database whose name ends in `test` (`rndmroll_test`), with `-p 1`. There is no JS test runner (a throwaway node script, `scripts/check-entry.ts`, checks the pure star, reaction, rule and upload-memory logic), so screen checks are device steps in `03-03-PLAN.md` Task 3 ('group done' or what looked wrong).

## Commands

| What | Command |
|---|---|
| Pure rules, no database | `go test ./internal/clock ./internal/spin ./internal/storage ./internal/config -count=1` |
| Full suite | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v \| grep -c '^--- SKIP'` prints 0 |
| App types | `npx tsc --noEmit` |
| Entry logic | `node --experimental-strip-types scripts/check-entry.ts` prints no failure and the line 'reaction names match the server list' (before plan 03-01 exists it prints 'not compared yet' instead, which is fine then but not at the gate); Phase 2's `scripts/check-wheel.ts` still prints no failure |
| App bundle | `npx expo export --platform ios --output-dir $(mktemp -d)` |
| Cold launch | `xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`, with the first frames recorded |
| Emoji allow-list | `LC_ALL=C grep -rlE --exclude-dir=lab $'\xf0\x9f\|\xe2[\x98-\x9e]' app components lib scripts` prints exactly `lib/entry/reactions.ts`; the same scan over `internal cmd migrations` prints nothing |
| Radius allow-lists | Phase 2's lists are unchanged: `grep -rln --exclude-dir=lab "radius\.lg" app components lib` prints the same five files and `"radius\.full"` the same three; neither appears in `components/entry`, `components/today` or `app/(app)/today` |
| Route listings | `ls "app/(app)"` lists exactly `_layout.tsx`, `profile`, `spin.tsx`, `today`; `ls "app/(app)/today"` lists exactly `_layout.tsx`, `edit-entry.tsx`, `edit-wheel.tsx`, `entry.tsx`, `index.tsx`, `log.tsx`; `ls migrations` lists exactly 0001 to 0005 (ten files) |
| No DELETE in the client | `grep -rn "'DELETE'" lib app components` prints nothing |
| Key, not link | `grep -rn "photo_url" internal/spin internal/store \| grep -v _test` prints nothing |
| Photo quality | `grep -c "quality: 0.8" components/entry/PhotoField.tsx` prints 2 |

Never run `expo lint`. Never run Go tests on `rndmroll_dev`.

## Map

| Requirement | Behaviour | Proof | Where |
|---|---|---|---|
| ENTRY-01 | An entry holds a photo key, a title (1 to 80 characters, trimmed), stars 1 to 10, a reaction (one of six names, or none), a comment (up to 140, or none) and a photo source; anything else is refused with the field named | Go table tests of the cleaners, HTTP tests, and the database check constraints refusing raw SQL | 03-01 Tasks 1, 2, 3 |
| ENTRY-01 | The photo ticket: JPG, PNG, WebP only, a 50 MiB ceiling, type and length signed into the upload, a key in the caller's own folder, a new key every time | Go tests with a stub presigner, HTTP tests | 03-01 Tasks 1, 3 |
| ENTRY-01 | A key from another person's folder, with `..`, in capitals or of the wrong shape is refused | Go tests of `EntryPhotoKeyOwnedBy`, HTTP tests | 03-01 Tasks 1, 3 |
| ENTRY-01 | The server builds the link from the key. No response carries the key. The database layer never holds a link | HTTP response-scan test, the key-not-link grep | 03-01 Task 3, 03-03 Task 2 |
| ENTRY-01 | The entry, the posted day and the spin's result commit or fail together | Postgres fault-injection tests (a failure at the spin result, and one in the posted-day write, leave no entry, no posted day and the spin open) | 03-01 Task 2 |
| ENTRY-01 | Late is decided by the server clock only: a post at exactly the end of the late window is refused, one nanosecond earlier is accepted and marked late | Go suite with the fixed clock | 03-01 Task 2 |
| ENTRY-01 | A real post counts the week and earns the bonus on the 4th day (once, one held at most); a late post just after midnight on a Monday for Sunday's spin counts for Sunday's week | Go suite | 03-01 Task 2 |
| ENTRY-01 | A retry with the same photo key returns the same entry and writes nothing; a different key on a settled day is refused; 20 callers at once give one entry | Go suite, concurrency tests, HTTP test | 03-01 Tasks 2, 3 |
| ENTRY-01 | A wrong spin number (the bonus spin was used on another phone) is refused and writes nothing | Go suite, HTTP test | 03-01 Tasks 2, 3 |
| ENTRY-01 | Edit changes title, stars, reaction, comment and photo only (never date, category, late stamp or posted day), clears the reaction and comment, works any time after posting, and answers 404 for another person's entry | Go suite, HTTP tests | 03-01 Tasks 2, 3 |
| ENTRY-01 | Delete removes the entry; the posted day and the spin's result stay; the day cannot be posted again; the week count does not drop; the old photo file is removed after the change and a storage error does not fail the request | Go suite, HTTP tests with a recording fake store, walkthrough step 9 | 03-01 Tasks 2, 3; 03-03 Task 3 |
| ENTRY-01 | Stars: a tap adds a half (3 becomes 3.5, star 5 gives 4.5), steps by a half, every value 1 to 10 can be produced, none outside the range, figure text | Check script, walkthrough step 4 | 03-02 Task 1; 03-03 Task 3 |
| ENTRY-01 | Reactions: exactly six, in order, names equal to the server's list; tap again clears | Check script (it reads the Go list), walkthrough step 5 | 03-02 Task 1; 03-03 Task 3 |
| ENTRY-01 | The six characters live in one file and nowhere else | Emoji scan | 03-02 Task 1; 03-03 Task 2 |
| ENTRY-01 | Save needs a photo, a title and stars; the 'Still needed' line lists what is missing | Check script (`missingParts`, `stillNeededLine`), walkthrough steps 2, 3, 5 | 03-02 Task 1; 03-03 Task 3 |
| ENTRY-01 | A photo from the camera or the library at quality 0.8; a wrong type or one over 50 MB is refused with a plain message before any upload | Check script (`photoProblem`, `photoMessage`), the quality grep, walkthrough step 17 (a full-size library photo) | 03-02 Task 1; 03-03 Task 3 |
| ENTRY-01 | A failed save keeps the form filled; a retry uploads the picked photo once and posts with the same key | Check script (upload memory), walkthrough step 16 (storage holds exactly one more photo) | 03-02 Task 1; 03-03 Task 3 |
| ENTRY-01 | Leaving with unsaved work asks first | Walkthrough step 6 | 03-03 Task 3 |
| ENTRY-02 | A repeat title on another day makes a new dated entry and never replaces the old one | Go suite (one title on two dates gives two entries, each readable) | 03-01 Task 2 |
| ENTRY-03 | The spun category is on top of the entry screen before anything is typed, and stays while scrolling | Walkthrough steps 1, 2, 12 | 03-03 Task 3 |
| ENTRY-03 | The entry takes the final spin's category id and the name written that day (a bonus spin, then a post; a rename afterwards changes nothing) | Go suite | 03-01 Task 2 |
| ENTRY-04 | Skip is free, asks no reason, is final for the day, works in the late window, is refused after it, repeats as the same answer, is refused after a post, and makes a post and a bonus refused | Go suite, HTTP tests | 03-01 Tasks 2, 3 |
| ENTRY-04 | A skip does not count toward the 4, is not a miss, shows as a dash on the week dots, and the sheet shows the current count before it asks | Go test (week flag, count unchanged), walkthrough steps 10, 11 | 03-01 Task 2; 03-03 Task 3 |
| ENTRY-04 | Bonus spin then skip: the earlier spin stays open but the day is skipped, the dot is flagged once, and the next morning is not `late` | `BuildToday` test, Go suite | 03-01 Tasks 1, 2 |
| ENTRY-01, 04 | Posted and Skipped show the right heading, sub-line and lines. Made before midnight, the morning after is 'Your next spin is coming.'. Made after midnight in the late window, they stay until the next spin opens | `BuildToday` table test, walkthrough steps 9, 11, 13, 15 | 03-01 Task 1; 03-03 Task 3 |
| ENTRY-01, 04 | 'Log it', 'Skip this one' and the late line appear only in Result and Late | Walkthrough steps 1, 12 | 03-03 Task 3 |
| ENTRY-01 | 'View entry' opens the entry page; it shows photo, title, stars, reaction, comment and date; Edit and Delete work | Walkthrough steps 7, 8, 9 | 03-03 Task 3 |
| Safety | The photo ticket, post, read, edit, delete and skip routes refuse no token (401), a deleted user (401), an unfinished account (403), a body over 16 KB (413), the 11th post in a minute (429); no response carries `photo_key`, birthday, email or password hash | HTTP tests | 03-01 Task 3 |
| Safety | Another person's entry, photo key or day is never reachable | HTTP tests with two accounts | 03-01 Task 3 |
| Safety | The test reset clears entries and real posted days | Go suite, HTTP test | 03-01 Tasks 2, 3 |
| Safety | Migration 0005 is reversible and the migration tests start and restore at the latest version | `TestMigrations_0005IsReversible`, the other migration tests reading `latestMigration` | 03-01 Task 2 |
| Safety | The users table, earlier migrations and the four real accounts are untouched | `git diff --stat pre-phase3` on those paths, user count before and after | 03-03 Task 2 |
| Safety | The dev database moves from 4 to 5 with a dump, a binary backup and a rollback | Cutover steps and the rehearsal on a throwaway copy | 03-03 Task 2 |
| Safety | No new native module | `git diff --stat pre-phase3 -- package.json package-lock.json app.json ios android` prints nothing | 03-03 Task 2 |
| Launch | The first screen is still Today with no Unmatched Route or Welcome flash; no launch file is edited; the bar has three items; the entry routes sit in the Home stack | Cold launches (a) and (b), `git diff` on the launch files, the `ls` listings | 03-02 Task 3; 03-03 Task 2 |
| Copy | No em dash, no 'roll' on screen, tokens only, no pill, no new use of the 24dp or round radius tokens; the phone computes no date or winner | Scans | 03-03 Task 2 |
| Copy | Developer approves the strings | Section 16 rows 1 to 10 of the UI spec and the listed choices | 03-03 Task 3 |
| Risk | A full-size library photo saves under the 50 MB limit (the profile ceiling) | Walkthrough step 17; the result and the photo's size go in the summary | 03-03 Task 3 |

## Sign-off

- [ ] Every row proven
- [ ] The Phase 3 criteria in ROADMAP are ticked in the close-out
- [ ] `nyquist_compliant` set to true by the close-out
