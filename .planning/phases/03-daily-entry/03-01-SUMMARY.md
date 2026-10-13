# Plan 03-01 summary: the entry server

**Built:** 2026-10-13, on branch `phase-01-foundation-accounts` (tag `pre-phase3` is HEAD before this plan, at `18cbcab`). Local commits only, nothing pushed. Other windows committed app and document files on the same branch in between; none of them are server files (the only commits that touch `internal`, `cmd` or `migrations` since the tag are the three below). All three tasks are built and proven on `rndmroll_test`. **The cutover is NOT done and is left for the lead on purpose**: `rndmroll_dev` is still at version 5 (read only check, not dirty) and no API on 8080 or 9090 was touched.

## Commits

| Task | Commit | Message |
|---|---|---|
| 1 | `81a0d10` | feat(server): entry rules, entry photo store and Today states (03-01 task 1) |
| 2 | `7287018` | feat(server): entries and photos tables, shared post transaction, skip and edit (03-01 task 2) |
| 3 | `be227e9` | feat(server): entry, photo ticket and skip routes (03-01 task 3) |
| summary only | this file's own commit | docs: plan 03-01 summary |

## Gates

Run on `rndmroll_test` only, with `-p 1`: `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./... -p 1 -count=1 -v`.

| | Baseline (HEAD at the tag) | After task 1 | After task 2 | Final |
|---|---|---|---|---|
| top-level `--- PASS` | 297 | 339 | 347 | 380 |
| `--- FAIL` | 0 | 0 | 0 | 0 |
| `--- SKIP` | 0 | 0 | 0 | 0 |

`go build ./...`, `go vet ./...` and `gofmt -l internal cmd` are clean at the end (exit 0, exit 0, no output). The routes alone, before their route tests, stood at 352.

Per package (top-level passes, baseline then final): auth 21/21, clock 7/7, config 11/11, httpapi 127/160, mail 6/6, middleware 27/27, spin 38/69, storage 7/19, store/postgres 30/37, user 23/23. `cmd/api` and `internal/spin/spintest` have no test files of their own; the shared suite in `spintest` is run by `spin` (in memory) and by `store/postgres`. `go test -race -p 1` was clean on spin, store/postgres and httpapi.

Plan acceptance checks:

- `ls migrations` lists 0001 to 0006 (twelve files); `git diff --stat pre-phase3 -- migrations` shows only the two 0006 files.
- `rndmroll_test` is at version 6, not dirty. `rndmroll_dev` is at version 5, not dirty (read only).
- `time.Now()` is in no new non-test code (checked over `internal/spin`, `internal/storage/entry_photos.go`, `internal/store/postgres/entry_repo.go` and `internal/httpapi/entry*.go`, leaving `clock.go` out).
- `git diff --stat pre-phase3` prints nothing for `internal/storage/s3.go`, `internal/httpapi/errors.go`, `internal/httpapi/spin_errors.go`, `internal/user` and `internal/store/postgres/user_repo.go`.
- `CoverKey` in `internal/httpapi`, outside tests, is only the one line inside `linkToday`. Below that layer, `EntryRefOf` is the one place that sets it; the model declares it and the shared suite reads it.
- `photo_url` appears in non-test files of `internal/spin` and `internal/store` only as the one json tag on `EntryRef`.
- No em dash and no emoji under `internal`, `cmd` or `migrations`. The word scan the plan names (`internal/spin`, `internal/storage`, `internal/httpapi/entry*.go`) prints nothing.

## What was built

- `migrations/0006_daily_entry.{up,down}.sql` (hand numbered, `users` untouched): `spins.settled_at` (null until the result becomes posted or skipped) and `spins_id_user_unique`; table `entries` (one per person per date, one per spin, title 1 to 80, stars 1 to 10, six reaction names, comment at most 140, index `entries_user_date_idx`); table `entry_photos` (position 0 to 9 with a deferred unique position, a unique key, jpeg, png or webp, camera or library, size 1 to 52428800). Photo rows go with their entry by cascade. `entries_spin_fk` has no cascade. The down file drops `entry_photos`, `entries`, the constraint, then the column.
- `internal/spin/entry.go`, `entry_errors.go`: the limits, the six reactions, `Entry`, `EntryPhoto`, `CleanEntryInput`, `CleanEntryPatch`, `ContentTypeOfKey`, `EntryRefOf`, `FieldProblem`. `model.go`: `EntryRef`, `Today.entry`, `WeekDayView.skipped`, the states `posted` and `skipped`, `Spin.SettledAt` and the display rules built on it.
- `internal/storage/entry_photos.go`: `EntryPhotoStore` (one ticket per photo, `PublicURL`, `DeleteEntryPhoto`) and `EntryPhotoKeyOwnedBy`. `s3.go` is untouched.
- `internal/spin/entry_service.go`, `repository.go`, `service.go`: `PostEntry`, `GetEntry`, `EditEntry`, `DeleteEntry`, `SkipDay`. A post is one `InUserTx`: the entry, its photos, the posted day (`markPostedIn`, which `MarkPosted` already used) and the spin's result. Every Today is built with the entry summary and the skipped dates. `ClearTestData` also deletes entries and every posted day now.
- `internal/store/postgres/entry_repo.go`, `spin_repo.go`: the new `Tx` methods on Postgres; the spin reads carry `settled_at`.
- `internal/spin/spintest/memrepo_entry.go`, `entry_suite.go`: the same methods in memory and one shared suite, `RunEntrySuite`, run by both repositories.
- `internal/httpapi/entry.go`, `entry_errors.go`: `EntryHandler`, six routes, `linkToday`, three new error codes. `spin.go` (the `PhotoURL` field and a `linkToday` call on each of its three Todays), `server.go` and `cmd/api/main.go`: wiring.

## The wire contract (for plans 03-02 and 03-03)

Checked against `internal/httpapi/entry.go` and `entry_errors.go` as built, and against the wire names listed in `03-02-SUMMARY.md`: every name there matches. Instants are RFC 3339 UTC with milliseconds, names are snake_case, dates are `YYYY-MM-DD`. Every route needs a signed-in, finished account. The limits are per person.

| Route | Body | Success | Per minute |
|---|---|---|---|
| `POST /v1/entry-photos/upload-url` | `{"content_type","content_length"}` | 200 `{"upload_url","photo_key","expires_in":300}` | 30 |
| `POST /v1/entries` | `{"for_date","spin_seq","title","stars","reaction"?,"comment"?,"photos":[{"photo_key","photo_source","size_bytes"?}]}` | 201 new, 200 same again: `{"already","entry","today"}` | 10 |
| `GET /v1/entries/{id}` | none | 200 `{"entry"}` | 60 |
| `PATCH /v1/entries/{id}` | any of `title`, `stars`, `reaction`, `comment`, `photos` | 200 `{"entry"}` | 20 |
| `POST /v1/entries/{id}/delete` | `{}` | 200 `{"today"}` | 20 |
| `POST /v1/spin/skip` | `{"for_date","spin_seq"}` | 200 `{"already","today"}` (new or repeat) | 10 |

- `Entry` has exactly 13 members: `id`, `local_date`, `category` (`{id, name}`, the name as written that day), `title`, `stars` (a whole number 1 to 10, so 7 means 3.5), `reaction` (`love`, `fire`, `laughing`, `wow`, `sad`, `sleepy` or null), `comment` (or null), `photos` (`[{id, url, position}]`, one to ten, in position order), `photo_url` (the first photo's url), `photo_source` (the first photo's, `camera` or `library`), `late`, `created_at`, `updated_at`. A storage key is never sent, except as `photo_key` in the ticket's answer and in request bodies.
- `Today` gains `entry` (`{id, late, title, stars, reaction, photo_url, photo_count}` or null) and `week.days[].skipped`. `state` can now be `posted` or `skipped`. `entry` is not null only in those two states. There `spin` is set (today's, or last night's in the late window) and `bonus.can_use` is false. A day settled at or after its window closed stays `posted` or `skipped` until the next opening. One settled earlier goes back to `waiting` after midnight.
- All six routes that send a `today` fill `photo_url` with a link, never a key: `GET /v1/today`, `POST /v1/spin`, `POST /v1/spin/bonus`, create, delete and skip.
- PATCH: `""` clears `reaction` or `comment`. `photos` is the whole ordered list the entry should end with: `{"id"}` keeps a photo, `{"photo_key","photo_source"}` adds one, a photo left out is removed. Absent or null `photos` leaves them alone. An empty body changes nothing.
- A create again with the same ordered `photo_key` list is 200 with `already: true` and the same entry, even when the day is settled. Any other list for a taken day is 409 `day_settled`. A skip again is 200 with `already: true`.
- `size_bytes` is an optional member of a new photo item (on create, and for an added photo in PATCH). It is a whole number from 1 to 52428800 and is for information only. The plan's request shape did not list it. A value out of range, or of the wrong type, makes `photos` `invalid`.
- 400 `validation_failed` with `field` and a fixed `message`. Fields and reasons: `for_date` (`required`, `invalid`), `spin_seq` (`required`, `invalid` when negative), `title` (`required`, `too_long` over 80 characters, `invalid`), `stars` (`required`, `invalid`), `reaction` (`invalid`), `comment` (`too_long` over 140 characters, `invalid`), `photos`, `content_type` (`invalid`), `content_length` (`required`, `too_long` over 50 MiB) and `time_zone` (`required`).
- The fixed texts: `required` "This field is required.", `invalid` "This value is not valid.", `too_long` "This value is too long." The `photos` field has its own three: "Add at least one photo." (none, or an empty list), "You can add up to 10 photos." (over ten) and "One of the photos can't be used." (anything else: an item that is not an object, both or neither of `id` and `photo_key`, a repeated key or id, a key that is not the caller's own server-made key or is already on a photo, a bad `photo_source` or size, an `id` on a create, or an `id` that is not one of the entry's photos).
- 400 `validation_failed` with no field and "Request body is not valid.": the body is not an object, or is not readable JSON. A member of the wrong JSON type is answered by its name instead (that field, `invalid`).
- 400 `time_zone` `required`: the person has no readable stored zone. Only post, skip and delete need one, because they build Today. The check comes after the body and field checks and before any day rule. For delete it comes before the 404 of a well formed id.
- 401 `token_invalid` or `token_expired` (no token, a deleted account); 403 `profile_incomplete`; 413 `payload_too_large` (body over 16 KB); 429 `rate_limited` (a refused request counts toward the limit too).
- 404 `not_found`: the path id is not a 36 character UUID, or there is no such entry, or it is someone else's, or it was deleted.
- 409: `no_spin` (no spin on that date, or a date that has not come), `day_settled` (already posted or skipped), `spin_changed` (`spin_seq` is not the latest spin's number), `day_closed` (the late window has ended; the spin routes keep `spin_closed`).
- Post checks, in this order: the same list again (200), `no_spin`, `day_settled`, `spin_changed`, `day_closed`, then a key already used (`photos` `invalid`). Skip: `no_spin`, a repeat (200), `day_settled`, `spin_changed`, `day_closed`.
- 503 `server_error`: another request held the person's lock too long. 500 `server_error`: anything unknown, logged, with fixed text.

## Differences from the plan (the ones the phone and later phases must know come first)

1. **`respondEntryError` sends every error it does not own to `respondSpinError`, not to `RespondError`** (the lead's decision). Busy stays 503, a missing zone stays the `time_zone` field error, an `InputError` names its field, and only an unknown error is the logged fixed 500. Entry errors are mapped first, so a closed day is `day_closed` here and `spin_closed` stays on the spin routes.
2. **Today shows `entry` only in `posted` and `skipped`.** `result`, `late`, `open` and `waiting` give null. A day settled by a pretend (test) posted day, which has no `settled_at`, now shows `posted`, not `result` (the plan's table, row 1). No existing test asserted the old state, so none was changed.
3. **Post, skip and delete need a stored zone** (the service runs them with the zone required, because they build Today). Get and patch do not. A test that makes "another person" delete must give that person a zone.
4. **Field answers are finer than the plan lists.** A missing `title` or `stars` on create is `required`. A title with a control character, or bytes that are not text, is `title` `invalid`. A comment with a lone CR is `invalid`, and CR LF becomes LF before the characters are counted. A member of the wrong JSON type (a number for `title`, a string for `content_length`) is answered by name.
5. **An empty body counts as `{}` on every body route.** Create and skip answer `for_date` `required`, the ticket answers `content_type` `invalid`, patch changes nothing (200 with the entry) and delete works.
6. **PATCH `photos: null` means unchanged**, like `title: null`. Only a list that is present and empty is `required`. On create, absent, null and `[]` are all `required`.
7. **A photo `id` with a bad shape inside a PATCH list is 400 `photos` `invalid`, never 404.** Only the path `{id}` is 404. The handler lower cases photo ids, so one id written in two cases is a repeat (`invalid`). A path id in capitals is accepted and lower cased.
8. **The ticket route checks in this order:** `content_type` missing or empty is `invalid`; `content_length` missing, zero or negative is `required`; only then the store, where an unsupported type is `content_type` `invalid` and over 50 MiB is `content_length` `too_long`. So a request wrong in both answers `content_length` when its type is present but unsupported.
9. **An edit writes only when something differs.** An empty patch, or one that repeats the stored values, writes nothing and keeps `updated_at`. Any real change, a photo-only edit included, bumps it. The plan said both "an empty patch writes nothing" and "any change bumps `updated_at`"; comparing values keeps both true.
10. **Order inside every route:** rate limit, `finishedUser`, the path `{id}` (404), the body, the fields, the service. Create checks `for_date`, `spin_seq`, `title`, `stars`, then the shape and owner of `photos`. Then the cleaner: title, stars range, reaction, comment, photo count, repeats, sources, sizes. Then the extension of each new key (`photos` `invalid`). Then the zone and the day rules.
11. **Files dropped by an edit or a delete are removed inside the request**, after the commit, on `context.WithoutCancel(request)` with ONE 3 second timeout for the whole batch (the plan said 3 seconds per call). A failure is logged with `log.Printf` (the line names the key, which is an internal path) and the rest still run.
12. **The cleaners return `(EntryInput, error)` and `(EntryPatch, error)`.** The error is a `*FieldProblem` read with `errors.As` (a typed nil pointer in an error is not nil). The cleaners do not check a key's extension, a key's owner or a patch id's shape. The service checks the extension with `ContentTypeOfKey` and checks every patch id against the entry's own photos in Go, BEFORE any id reaches SQL.
13. **Two small additions.** `EntryRefOf(*Entry) *EntryRef` builds the Today summary (the cover is the lowest position). `week.go` is not edited: `WeekDayView` lives in `model.go`.
14. **A zero `Date` is answered first.** `PostEntry` and `SkipDay` return `InputError{for_date, invalid}` for it, because it would print as 0000-00-00 and reach Postgres as a 500. The HTTP layer parses the date first, so it cannot happen there.
15. **`Tx` details the plan left open.** `PhotosOfEntry` of an unknown entry is an empty list. `InsertPhotos` and `ReplacePhotos` of an entry that is not the person's are `ErrEntryNotFound`. `ReplacePhotos` returns the removed keys in their old position order, deletes dropped rows first, moves kept rows with ONE `update ... from unnest`, and leaves the entry row alone (`UpdateEntry` owns `updated_at`). `SettleSpin` takes posted or skipped only, and zero rows is `ErrDaySettled`. `InsertEntry` conflicts over both unique keys. On Postgres an id that is not a uuid is a plain error (a bug, not a 404). `PhotoKeysInUse` is the one statement with no `user_id`, on purpose: a key is unique across people. Every other statement in `entry_repo.go` names the person.
16. **Every entry `PostEntry` returns** (new or `already`) is read back inside the transaction, so `late`, the category and the photo order are exactly what a later read gives.
17. **Storage:** a length of zero or less is `ErrEntryPhotoTooLarge` (as the avatar store does), so the handler answers `required` for it itself. A bad key given to `DeleteEntryPhoto` makes no storage call and returns an unexported error. A storage error comes back unwrapped.
18. **Wiring:** `NewServer` sets `Spin.PhotoURL` only when the entry handler's store is set, and before the spin routes are registered. `cmd/api/main.go` builds one `storageConfig` for both the avatar store and the entry photo store.
19. **MemRepo** mimics the check constraints and returns plain errors, so a service bug shows in the in-memory run. Its `PhotoKeysInUse` sees only the caller's own rows (a lock on another person's data could deadlock), so a key collision between two people is covered by the Postgres and HTTP tests only. Its `ClearTestData` now removes entries, posted days of every source, spins and goal weeks, and sets `bonus_held` to 0 (before, it kept real posted days).
20. **Parts and gates (the lead's decision).** Task 2 was built as two parts (the spin side, then the Postgres side) and Task 3 as two parts (the routes, then the route tests). Separate agents built the parts and the lead gated each one.
21. **Phase 2 assumptions that had to be fixed:** none is recorded in the build notes. `markPostedIn` already existed, so `MarkPosted` kept its body, and `MemRepo` already undid a failed call.
22. **Test notes.** Beyond the plan's list: fault injection runs each of the three methods twice (failing instead of the write, and failing right after it); the raw SQL test covers key length, the accepted limits, a swap inside one transaction and a key held by another person; the rate limits are checked on all six routes and per person; a second account runs end to end on Postgres; a test proves the privacy scanner can fail. The whole-day tests are in a third file, `entry_flow_test.go`, because `entry_test.go` was at 1322 lines and the cap is 1500. No test file holds non-ASCII text: the wide character for the 80/81 and 140/141 cases is built from its code point (`eAcute`), because the file writing tool turns a backslash-u escape into the literal character.
23. **Mutation run on a scratch copy** (the working tree was never changed): every mutation tried was caught, except two that change no answer. Removing the late-window check or the settled-day check from `postEntryIn` is harmless because `markPostedIn` and `SettleSpin` refuse the same cases again. That is defence in depth, not a gap.

## Facts for the cutover and for later phases

- **Order matters one way only:** the new binary's spin reads name `settled_at`, so 0006 must be applied BEFORE it starts. The old binary on a version 6 database keeps working, except for the test reset (next point). That is the migrate-first recipe and the rollback.
- **With entries present, the OLD binary's `POST /v1/test/reset` fails**, because it deletes spins and `entries_spin_fk` has no cascade. Test resets stop working until the new binary is back. Never `migrate down` on dev once real entries exist: down drops `entries` and `entry_photos`.
- Migration 0006 was rehearsed on a scratch database, dropped afterwards: `up`, `goto 5`, `goto 6`. A swap of positions in one transaction passes, two rows at one position are refused at commit, and deleting the user clears both tables. The same cases are held on `rndmroll_test` by `TestMigrations_0006IsReversible`, `TestEntryTables_RefuseWhatTheServiceNeverSends` and `TestEntryTables_DeletingAnAccountClearsThemToo`.
- The four real dev accounts: `users` is unchanged and nothing here deletes accounts.
- **Photo links are public until Phase 5.** Unused uploads and the photos of reset days stay in storage (there is no cleanup job). Storage upload has never run against a real bucket, and the real key needs delete rights, because an edit or a delete asks for a file removal.
- `entries_user_date_idx` is there for the Phase 4 diary. Phase 4's migration, if it needs one, is 0007.
- Time: no `time.Now()` in new code. `now` comes from the per-account clock, once per call, after the lock. `created_at` and `updated_at` come from Go (no defaults), so a moved test clock stays coherent.

## Left for the cutover (the lead)

1. Back up the binary and `pg_dump rndmroll_dev`. Read only: dev is at version 5; write down `select count(*) from users`.
2. Rehearse on a throwaway copy: `goto 6`, `goto 5`, `goto 6`, same user count, drop it. Echo the URL before every `migrate`.
3. `migrate -path migrations -database "postgres://localhost:5432/rndmroll_dev?sslmode=disable" up 1`; check version 6, not dirty, and the user count. The new binary MUST NOT start before this.
4. Build from HEAD and restart on 8080 as the Phase 2.1 close-out left it; `/healthz` 200.
5. Live check with throwaway accounts from the allow-list: ticket, create, GET, skip, delete, reset. Then delete the throwaway accounts by id and re-count users.
6. Rollback if needed: stop, copy the old binary back, start; leave 0006 applied (the old binary cannot reset test accounts that hold entries; see Facts). Never `migrate down` on dev once real entries exist.
