# 04-01 Summary: diary and profile summary server

Two read routes, built. Nothing in the app was touched.

## Wire shapes

Both routes need a finished account (403 `profile_incomplete` otherwise), the token only, 60 a minute per person.

`GET /v1/me/summary` answers 200:

```
{"spins":int,"week_streak":int,"friends":0,
 "tastes":[{"category":{"id","name","removed":bool},"average":4.6,"count":9}],
 "recent":[Entry]}
```

- `spins`: days spun (daily spins; a bonus spin does not add one). Archived days still count.
- `week_streak`: consecutive goal weeks ending this week or last, by the stored zone and the service clock. No zone or a bad zone gives 0.
- `friends`: always 0 until Phase 5.
- `tastes`: one per category with at least one visible entry, wheel order then removed ones by name. `average` is half the mean stars, one decimal. `count` is the number of entries.
- `recent`: at most 3 visible entries, newest first. `Entry` is the same object `GET /v1/entries/{id}` sends.
- Empty lists are `[]`.

`GET /v1/diary?category_id=&cursor=&limit=` answers 200:

```
{"items":[Row],"next_cursor":string|null,"categories":[{"id","name","removed"}]}
Row = {"kind":"entry","entry":Entry}
    | {"kind":"skipped","local_date":"2026-10-06","category":{"id","name"}}
```

- Newest first, one row per date. Skipped rows only without `category_id`.
- `limit` 1 to 50, default 20. `cursor` is opaque, taken from `next_cursor`.
- `categories` (chips) are sent with every page: categories with at least one visible entry, wheel order, then removed ones.
- 400 `validation_failed` with `field` `category_id`, `cursor` or `limit`, reason `invalid`. A category id that is not the caller's gives an empty list.

## Archived and deleted

- Archived entries (`archived_at` set) are left out of recent, tastes (counts and averages), chips and the diary rows. `spins` and `week_streak` still count the day, and the date makes no skipped row either.
- Deleted entries are gone from all of it.

## Differences from the plan

- No migration. Existing indexes (`entries_user_date_idx`, the spins unique key) cover the reads; the 3,000 entry plan check was not run.
- No in-memory Reader, no `spintest` snapshot and no shared suite. Postgres is the only Reader. The rules are unit tested in `internal/diary` with a fake Reader; the whole behaviour is tested over Postgres through the real server in `internal/httpapi/diary_integration_test.go`.
- Ids are strings (as in the spin package), not `uuid.UUID`.
- The Entry JSON is the existing `entryView` (it has `photos`, `archived`, `comments_off` as well), unchanged.
- Files: `internal/diary/*`, `internal/store/postgres/diary_repo.go`, `internal/httpapi/diary.go`, wiring in `server.go` and `cmd/api/main.go`.
