---
phase: 02-daily-roll
plan: 03
status: closed 2026-10-13 with device checks not walked (listed at the end)
closed: 2026-10-13
commits: this file's own commit, then the close-out commit (see git log)
---

# 02-03 Summary: cutover, gates, walkthrough and close-out

Branch `phase-01-foundation-accounts`. Local commits only, nothing pushed. This plan changed no code.

## In one paragraph

Phase 2 is on the dev API and the dev database and it works. Every automatic gate passes. The server rules
were driven through the API with curl on 2026-10-13 and every check passed. The developer walked the
visible parts on the simulator and said "looks good". Six device checks were NOT walked with hands, and
more steps of the plan are not on record. Phase 2 is closed with those open. They are listed under
"NOT walked" and are marked "not walked" in `02-VALIDATION.md`.

## Cutover facts

- `rndmroll_dev` moved from version 3 to 4 (Phase 2, `0004_daily_spin`, on 2026-10-10) and from 4 to 5
  (Phase 2.1, `0005_account_visibility`, on 2026-10-13). Read on 2026-10-13: version 5, not dirty.
- Binary backups in `~/.local/share/rndmroll-api/`: `api.pre-19`, `api.pre-photo` (Phase 1),
  `api.pre-phase2` (2026-10-10 22:08), `api.pre-phase2.1` (2026-10-13 13:14).
- Data dumps in the same folder: `dev-pre-0004.sql` (2026-10-10 22:08), `dev-pre-0005.sql`
  (2026-10-13 13:14).
- The API on 8080 answers `/healthz` 200. Its process has `APP_ENV=test`, `SPIN_TEST_TOOLS=1` and
  `SPIN_TEST_USER_EMAILS=cutover-p2@example.com,walkthrough-p2@example.com` (read from the process on
  2026-10-13). **The test tools are ON.** It was not restarted without them (that was Group 9's
  "before" step, which was not reached).
- Users: five rows. The four real accounts (`smoke-test@example.com`, `ava.stone.1004@example.com`,
  `test1@gmail.com`, `test4@gmail.com`) and `walkthrough-p2@example.com` (id
  `73d396dd-6bb4-47ad-93a5-0326e5162090`). `cutover-p2@example.com` is gone. The four real accounts
  were last updated between 2026-09-23 and 2026-10-08, have no spins, no posted days and no stored time
  zone. `firstrun-p2@example.com` was never made (Group 8b was not walked).

## Gates

| Gate | Result | Tree |
|---|---|---|
| `go build ./...`, `go vet ./...` | exit 0 | `18cbcab` |
| Go suite on `rndmroll_test`, `-p 1 -count=1 -v` | 297 top-level PASS, 0 FAIL, 0 SKIP, all packages ok | `18cbcab` |
| `npx tsc --noEmit` | exit 0 | `b8729fd` |
| `node --experimental-strip-types scripts/check-wheel.ts` | "ok: 47877 checks, no failure", exit 0 | `b8729fd` |
| `npx expo export --platform ios` | exit 0, one iOS bundle (4.6 MB) | `b8729fd` |

- Go PASS counts: 181 at tag `pre-phase2`, 258 at the end of 02-01, 297 now (the 39 above 258 are
  Phase 2.1's server tests). The two commits after `18cbcab` (`a31013d`, `b8729fd`) touch no `.go` file,
  `go.mod`, `go.sum` or migration (checked with `git diff --stat 18cbcab HEAD`), so the Go result stands
  for `b8729fd`.
- The check script's count (47877) is lower than the 146316 that 02-02 reported, because the script and
  the rules changed after that (equal wedges D-23, finer wheel D-25).

## Scans (run at `b8729fd`)

All scans of 02-03 Task 2 print what the plan says, with the two deviations at the end.

- Em dash in `app components lib scripts internal cmd migrations`: nothing.
- Raw hex in `app components lib`: only `lib/theme/tokens.ts` and `lib/theme/tokens.test-assert.ts`.
  `wheelMid`: nothing. `rgba(` in wheel, today, nav, `(app)`: nothing. `wheelCharcoal` anywhere in code:
  nothing (D-26, `b8729fd`, removed the token and its pin).
- Emoji: nothing. Word "roll" in the wheel, today, nav, `(app)`, `lib/wheel`, `lib/api/spin.ts` and
  `internal/spin`: nothing.
- Radius: `radius.full` in wheel, today and `(app)`: nothing; in nav only `SpinTabButton.tsx`;
  `radius.lg` in wheel, today, nav: nothing. App-wide `radius.lg` is the five files of the plan and
  `radius.full` the four.
- No phone-side winner, date or clock (`Math.random`, `new Date`, `Date.now`): nothing. No time or weekday
  formatted on the phone: nothing; `spun_local_time` is in `TodayScreen.tsx` and `lib/api/types.ts`.
- `DialAvatar` in today: nothing. `clock.` in `internal/auth` and `internal/middleware`: nothing.
- `git diff pre-phase2 --stat` of `package.json`, `package-lock.json`, `app.json`, `ios`, `android`:
  nothing (no new native module). Launch files (`app/_layout.tsx`, `app/(auth)/_layout.tsx`,
  `lib/session/store.ts`, `app/(app)/profile/edit.tsx`): nothing.
- `ls "app/(app)"`: `_layout.tsx board.tsx diary.tsx profile spin.tsx today`. No `/home` link.
  `ls "app/(app)/today"`: `_layout.tsx edit-wheel.tsx index.tsx notifications.tsx`.
  "Nothing here yet." once in `diary.tsx` and once in `board.tsx`; four tab icon lines in the layout.
- 12 type token pins in `tokens.test-assert.ts`. "About" and "A higher number" in `components lib app`:
  nothing. `Domine_400Regular` is loaded in `lib/theme/fonts.ts`.

Two scans do not print what the plan's literal text says, both because of Phase 2.1 and neither is a
Phase 2 fault:

1. "Users untouched" against tag `pre-phase2` prints five files (`internal/user/model.go`,
   `repository.go`, `visibility.go`, `visibility_test.go`, `internal/store/postgres/user_repo.go`). They
   are Phase 2.1's account visibility. The same command from `pre-phase2` to `pre-phase2.1` prints
   nothing, so Phase 2's own commits did not touch the user files.
2. `ls migrations` lists ten files (0001 to 0005), not the eight of the plan. `0005_account_visibility`
   is Phase 2.1's.

## Cold launches and the time zone

- (a) signed out gives Welcome: seen (reported to me by the developer's session; I did not run the launch).
- (b) signed in, (c) expired token, (d) log out then log in and a second account, (e) just signed up: no
  recorded first frames on the real server are on record. 02-02 ran its signed-in cold launch against an
  in-app stand-in, not the real API.
- Time zone: the walkthrough account's stored zone is `Asia/Kolkata`, which is the Mac's zone
  (`/var/db/timezone/zoneinfo/Asia/Kolkata`). The check "change the Mac's zone and see the stored zone
  follow" was not done.

## What the developer walked

On the simulator, signed in as the throwaway account, the developer walked and said "looks good":

- Today waiting and open.
- The spin (smooth).
- Edit wheel: no numbers, and a new category that is not grey. That session left a category "Hobby" on
  the wheel at weight 1, version 2, so the app's Save did send weight 1.
- The bell page.
- The bottom bar.

Two things changed in the app after that and may not have been looked at since: the wheel's dark parts
went from soft charcoal back to black (`ink`, D-26, `b8729fd`, 13:53 IST on 2026-10-13), and the bar and
tabs got motion (Phase 2.2, `18cbcab` and `a31013d`, which are not part of this plan).

## What the API checks proved

Done by curl and jq on 2026-10-13 against the dev API, with `walkthrough-p2@example.com` only. Time zone
`Asia/Kolkata`. The account was reset between groups.

**Group A, the spin is idempotent: pass.** Clock set to 2026-10-13 8:30 PM: state `open`. First
`POST /v1/spin`: 201, `already` false, id `25be0012-a9ae-4e5a-ae8a-7f3ec4ceb1f4`, category Book,
`winner_index` 3 of 7, kind `daily`, seq 0, `spun_local_time` "8:30 PM", `next_open_weekday` "Wednesday",
state `result`. Second call: 200, `already` true, same id. A third with `for_date` 2026-10-13: 200, same
id. One row in `spins`. `GET /v1/today` shows the same id.

**Group B, weekly goal and bonus: pass.** Clock set to Sunday 2026-10-11 8:30 PM (week start 2026-10-05).
Monday to Thursday (10-05 to 10-08) marked posted: `week.posted` 4, `week.goal_met` true, `bonus_held`
true; `bonus.can_use` false before the spin. Daily spin: 201, Place, seq 0; `can_use` true. Bonus spin
(`after_seq` 0): 201, kind `bonus`, seq 1, Hobby (different from Place), stamp "8:30 PM", `bonus.held`
false, `can_use` false, `week.posted` still 4; database `bonus_held` 0 and two rows in `spins`. A second
bonus (`after_seq` 1): 409 `no_bonus`. A retry of the first bonus (`after_seq` 0): 200, `already` true,
same spin id, still two rows, `bonus_held` still 0. Settled day: after an account reset, a daily spin and
marking today (Sunday) posted: `week.posted` 5, `spin.result` `posted`, `can_use` false, and the bonus
call answered 409 `day_settled` with nothing spent (the earned bonus is still held, `bonus_held` 1).

**Group C, the late state: pass.** Spin at 2026-10-13 8:30 PM (Hobby). Clock set to 2026-10-14 12:30 AM:
`state` `late`, `local_date` 2026-10-14, the spin is last night's (same id, `local_date` 2026-10-13,
`spun_local_time` "8:30 PM"), `next_open_weekday` "Wednesday". `POST /v1/spin` with `for_date`
2026-10-13: 200, `already` true, same id. With `for_date` 2026-10-12 (no spin): 409 `spin_closed`. With
no date: 409 `spin_not_open`. Still one row. At 7:59 PM that day: `late`. At 8:00 PM: `open`, `spin` null.

**Group D, wheel saves of 2 and 12: pass.** `PATCH /v1/wheel` with 2 categories (Meal, Movie, weight 1):
200; `GET` matches (2 categories, weights 1,1, positions 0,1, version 2 to 3). Four spins, each after an
account reset: `winner_index` 1, 0, 1, 1, each inside the list and `wheel[winner_index].name` equal to the
category. `PATCH` with 12 (Meal, Movie and ten new names, all weight 1): 200; `GET` shows 12 categories,
all weight 1, positions 0 to 11, version 4. Four spins: `winner_index` 10, 1, 9, 6, each inside the list
with the matching name. Every spin had `spun_local_time` and `next_open_weekday` set. Also: 13
categories is refused 400 `wheel_invalid` reason `too_many`, 1 category 400 `too_few`, and the wheel did
not change (12, version 4).

**End state: pass.** Clock reset and account reset done. `today.now` was within 0 seconds of real time
(2026-10-13T08:25:16Z), state `waiting`, no spins, no posted days, `bonus_held` 0, five users, four real
accounts present. The wheel was put back to the seven names it had (Meal, Movie, Song, Book, Place, Game,
Hobby, all weight 1, now version 5; five of them are new rows with new ids).

## Differences from the plan, and things the developer may notice

- The walkthrough did not run as written (Claude moves the clock, the developer walks one group). The
  developer walked the visible parts alone, and the server-side groups were done afterwards by curl.
- The account resets above removed the developer's own 8:56 PM "Hobby" spin of this session. The
  simulator now shows the real time of day (waiting), not that result.
- The walkthrough account still exists and the API is still in test mode (see "State left behind").
- ROADMAP criterion 4 and ROLL-03 say "relative weights" and "reweighting". Since D-23 (2026-10-10) every
  category has the same chance and the app sends weight 1. The server still accepts weights 1 to 10.
- `02-02-SUMMARY.md` has a status line written before the cutover ("waiting for the cutover"). This
  file supersedes it. `02-01-SUMMARY.md` says Phase 3's migration is 0005; it is 0006 (0005 is Phase 2.1's).

## NOT walked, so still open

Named by the developer's session as not walked with hands:

1. Reduce Motion (Group 3, step 12).
2. The bonus-spin sheet on screen (Group 4, steps 14 to 17: the "Use bonus spin" link, "Use your bonus
   spin?", "Keep this word", "Spin again", the wheel coming back with the second word). The server side is
   proven by the API checks above.
3. The Profile tab "Saved." line (Group 8, step 29).
4. Offline: "Couldn't connect." and "Try again" (Group 8, step 30).
5. The first-run page after a sign-up (Group 8b, steps 31b to 31f; cold-launch case (e)).
6. Larger Text (Group 8, step 31).

Not on record (no report reached me, so they are not claimed):

7. The late screen on the phone (Group 5, steps 19 to 20b).
8. Group 2 steps 10 and 11 (reopen shows the word at once; the Spin button from the Profile tab; the word
   under the pointer equals the word that fills the screen, on three days).
9. Edit wheel beyond "no numbers" and "new category not grey": the name and duplicate messages, the leave
   sheet, Save wheel returning to Today, and step 25b (Group 6, steps 22 to 25b).
10. Wheel counts 2 to 12 on the screen (Group 7, steps 26 to 28).
11. Group 9: logging in as a real account (`test1@gmail.com`), the first paint being Loading and never
    the throwaway's words, and a second account never seeing the first one's wheel (cold-launch case (d)).
12. Cold-launch cases (b) to (d) with recorded first frames on the real server, and the Mac zone change.
13. String approval: `02-UI-SPEC.md` section 13, keep or change for each row and the listed choices.
    No answers are on record.
14. Whether the developer has looked at the black wheel (D-26) and the Phase 2.2 bar motion on the phone.

## State left behind

- The dev API on 8080 runs WITH test tools. Before real use, restart `start-api-test-mode.sh` without
  `APP_ENV`, `SPIN_TEST_TOOLS` and `SPIN_TEST_USER_EMAILS` (they were set in front of the script at the
  cutover or sit in the untracked `.env`; check both).
- `walkthrough-p2@example.com` still exists. Delete it by id inside
  `begin; delete from users where id = '73d396dd-6bb4-47ad-93a5-0326e5162090' returning email; ... commit;`
  and commit only if the returned email is exactly that address. Then the user count is 4.
- The walkthrough account's clock is reset and its spins and pretend days are cleared. Its wheel is the
  seven categories above.
- The four real accounts were never touched by this plan.
