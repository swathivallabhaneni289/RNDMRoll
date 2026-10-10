# Plan 02-01 summary: the spin server

**Built:** 2026-10-10, on branch `phase-01-foundation-accounts` (tag `pre-phase2` is HEAD before this plan). Local commits only, nothing pushed. The dev database `rndmroll_dev` was not migrated and no API was restarted: the cutover is plan 02-03.

## Commits

| Task | Commit | Message |
|---|---|---|
| 1 | `ad546ec` | feat(server): clock, config and spin rules (02-01 task 1) |
| 2 | `70d6ac7` | feat(server): daily spin tables, repository and service (02-01 task 2) |
| 3 | `9aba882` | feat(server): spin, wheel and test-tool routes (02-01 task 3) |
| summary | this file's own commit | docs: plan 02-01 summary |

## Gates

Run on `rndmroll_test` only, with `-p 1`: `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v`.

| | Baseline (HEAD at the tag) | Final |
|---|---|---|
| top-level `--- PASS` | 181 | 258 |
| `--- FAIL` | 0 | 0 |
| `--- SKIP` | 0 | 0 |
| `go build ./...` | 0 | 0 |
| `go vet ./...` | 0 | 0 |

Per package (top-level passes, baseline then final): auth 17/17, clock 0/7, config 6/11, httpapi 82/102, mail 6/6, middleware 27/27, spin 0/38, storage 7/7, store/postgres 16/23, user 20/20. The shared suite and the window table run as subtests, so they count once at top level. `go test -race` on clock, spin, httpapi and store/postgres (with the database) was clean.

Acceptance greps all print nothing (or only the allowed lines): no `time.Now()` in `internal/spin`, `internal/clock` (outside `clock.go`), `internal/store/postgres/spin_repo.go` or the spin routes; no `clock` in `internal/auth` or `internal/middleware`; exactly the two helper lines in `internal/spin/model.go` for the screen time and weekday; no `c.Param`; no whole word "roll"; no em dash in `internal`, `cmd` or `migrations`; the two display strings only in `internal/spin` and `internal/httpapi`.

`rndmroll_test` is at schema version 4, clean. `TestMain` of the Postgres and httpapi packages runs `migrate up`, and `TestMigrations_0002`, `_0003` and the new `_0004` all read the latest version from the files and restore it.

## What was built

- `internal/clock`: `Clock`, `Real`, `PerUser` (shift per account, time keeps running), `Fixed`, and a small `ClockControl` interface the test route uses.
- `internal/config`: `APP_ENV`, `SPIN_TEST_TOOLS`, `SPIN_TEST_USER_EMAILS`. Tools on needs `APP_ENV=test`, a non-empty list and a database named `*_dev` or `*_test` (URL form with `dbname` parameter, and key=value form, both read), else `Load()` fails with a message that names `SPIN_TEST_TOOLS`.
- `internal/spin` (pure): `Date`, `ParseZone`, the window (`DayStart`, `WindowFor`, `PrevDate`, `NextOpen`, `InstantOfLocal`), weeks, `Pick` and `PickOther`, wheel validation, the types and `BuildToday`.
- `internal/spin` (storage and rules): `Repository` and `Tx`, `Service` (`SetTimeZone`, `Today`, `Spin`, `Bonus`, `Wheel`, `SaveWheel`, `MarkPosted`, `MarkTestDays`, `ResetTestData`, `ZoneOf`).
- `internal/spin/spintest`: `MemRepo` (one lock per person, rolls back when the callback fails) and `RunServiceSuite`, run by `internal/spin/service_suite_test.go` on `MemRepo` and by `internal/store/postgres/spin_repo_test.go` on Postgres, including the five concurrency cases.
- `migrations/0004_daily_spin.{up,down}.sql`: six new tables, `users` untouched, `set_updated_at()` neither redefined nor dropped.
- `internal/store/postgres/spin_repo.go`.
- `internal/httpapi`: `spin.go`, `wheel.go`, `spin_testtools.go`, `spin_errors.go`; `server.go` (`Deps.Spin`, `Deps.Wheel`, `Deps.TestTools`), `cmd/api/main.go`, and both test builders (`newFullTestRig` through `newFullTestRigWith`, `buildIntegrationServer` through `buildIntegrationServerWith`).

## Differences from the plan (each is a choice the plan left open or a narrowing forced by another rule)

Ones that matter downstream:

1. **One bonus spin per date.** The plan's partial unique index allows one bonus row per person per date, so `bonus.can_use` is also false when the day's latest spin is already a bonus, and a second `POST /v1/spin/bonus` for that day (after_seq equal to the latest, day not settled) answers `no_bonus` and spends nothing. The plan's `can_use` sentence said "only in `result`, day not settled, bonus held"; this adds "and the shown spin is the daily one".
2. **`wheel_invalid` index is -1 for list-level reasons** (`too_few`, `too_many`, `too_many_total`). Row reasons carry the row's position in the sent list: `name_required`, `name_too_long`, `duplicate_name`, `weight_range`, `duplicate_id`, `unknown_id`. The body is exactly `{"error":"wheel_invalid","reason":"...","index":n}`.
3. **`seeded_*` for a person whose first call needs no zone.** `GET /v1/wheel`, `PATCH /v1/wheel` and `POST /v1/test/reset` do not need a stored zone. If one of them is the person's very first call, the state row is made with `seeded_tz` `UTC` and `seeded_local_date` the UTC date. `Today`, spin, bonus and `MarkPosted` refuse with 400 `time_zone` until a zone is stored and make nothing. The phone reports its zone before it loads Today, so this only happens for a direct API call. Phase 3 reads `seeded_local_date` for the first partial week and should know the date can be a UTC date in that case.

Smaller ones:

4. **HTTP statuses** (the plan named codes only): `spin_not_open`, `spin_closed`, `no_bonus`, `day_settled` and `wheel_stale` are 409; `wheel_invalid` and field errors are 400; `profile_incomplete` is 403; a lock timeout (5 seconds) is 503 `{"error":"server_error"}`.
5. **Bonus draw** is filtered to snapshot categories that still exist as rows (a category removed for good at the 500 row cap would break the foreign key). If nothing else is left, `ErrNoOther` answers 409 `no_bonus` and the spend is rolled back.
6. **A control character in a category name** reports `name_required` (the plan's reason list has no `name_invalid`). So does a name with no visible character (for example only U+200B).
7. **Instants** are written with millisecond precision in UTC, for example `2026-10-08T00:02:00.000Z`. Dates are `YYYY-MM-DD`.
8. **`BuildToday`** takes one `TodayInput` struct, not a positional list. The inputs are the same (now, zone, today's latest spin, yesterday's latest spin, settled, last daily spin time, live wheel, bonus held, goal) plus the posted dates of the week.
9. **`daySettled`** is a pure function over the posted-day row and the date's latest spin (`daySettled(posted *PostedDay, latest *Spin)`); the service reads both with `dayFacts(tx, date)`, which also puts `posted` on the shown spin's result when a posted-day row exists.
10. **Extra error values and one type** beyond the plan's list: `ErrZoneInvalid` (a name the phone sent is not accepted; a stored name that no longer loads is `ErrZoneMissing`, as planned), `ErrBusy`, and `InputError{Field, Reason}` for after-parse field problems (`after_seq`, `dates`). `ErrWheelInvalid` is the target of `errors.Is` for a `*WheelError`.
11. **`ParseZone`** returns `ErrZoneInvalid` for every refusal. A name in capitals (`AMERICA/NEW_YORK`) passes the shape rule and loads on a case-blind host, as the plan's rule implies.
12. **Pretend days** go through `Service.MarkTestDays`, which validates all dates and then calls `markPostedIn` (source `test`) for each, in one transaction. `MarkPosted` and `markPostedIn(tx, now, loc, userID, date, source)` have the planned shape; `userID` is unused inside because the transaction is already scoped.
13. **Test routes:** `POST /v1/test/reset` answers `{"ok":true}`; an "exactly one of now, local, reset" violation names the field `now`; reset reads its body (empty is fine) so the 16 KB cap applies to it like every other route; `local` takes `YYYY-MM-DDTHH:MM:SS`.
14. **`late` is guarded** by "now is before yesterday's `late_until`" (Phase 3's start rule 2): at or after that instant, with a daily spin under 12 hours old keeping the wheel shut, the state is `waiting`.
15. **`Spin` with a `for_date` that has a daily spin** returns it with `already` true at any server date, and `today` is the current date's Today (in the `late` state its `spin` is that spin).
16. **A slow test:** `TestSpinRepo_LockTimeoutIsErrBusyAndFreesTheCaller` holds a row lock for the 5 second timeout, so the Postgres package takes about 5 seconds longer.

Departure from the paper that is only cosmetic: the Task 1 test file names are `date_zone_week_test.go`, `pick_wheel_test.go`, `window_test.go` and `model_test.go`.

## Facts for plan 02-02, plan 02-03 and Phase 3

- **Wire shape** is as in plan Task 3b, plus: `Today.spin` in `late` is last night's spin; `spin.result` is `posted` when a posted-day row exists for that date; `week.today_index` is 0 to 6.
- **Phase 3 can use these names unchanged:** `Service.MarkPosted` and `markPostedIn`, `Tx` (it only needs more methods on the same type: `SettleSpin`, entry reads and writes; the Postgres type is `pgTx` in `spin_repo.go`), `InUserTx`, `dayFacts`, `daySettled`, `BuildToday`/`TodayInput`, `Spin`/`SpinView`/`WeekDayView`. No type named `Entry` exists in package `spin` (the wheel type is `WheelEntry`).
- **`MemRepo.InUserTx` rolls back** when the callback returns an error (it works on a copy), so a Phase 3 rollback test can run on both repositories. The test hooks in `spintest.Env` (`SetSpinResult`, `ForceZone`, `FillArchived`) reach storage directly.
- **No migration after 0004.** Phase 3's 0005 adds `spins.settled_at` and `unique (id, user_id)`; neither is in 0004.
- **Test mode variables** for the dev API in plan 02-03: `APP_ENV=test SPIN_TEST_TOOLS=1 SPIN_TEST_USER_EMAILS=a@example.com,b@example.com`. With the tools on and `APP_ENV` unset, `config.MustLoad()` panics with the text "SPIN_TEST_TOOLS=1 needs APP_ENV=test", which is the refusal check of 02-03 step 6.
- **Starting wheel:** since 2026-10-10 all six (Meal, Movie, Song, Book, Place, Game) start at weight 1. This plan built it as Meal 3, Movie 1, Song 3, Book 1, Place 2, Game 1. The API still accepts weights 1 to 10.
- **Not done here, on purpose:** `.env.example` does not list the three new variables (that file was off limits); `docs/` and the phone app were not touched.
