# 03-04 Summary: archive and comments switch (server only)

## Migration

`0007_entry_archive` (up and down): `entries.archived_at timestamptz null`,
`entries.comments_off boolean not null default false`, plus a partial index for
the archive list.

Notification settings and Blocks migrations now move to **0008**.

## Wire shapes added

- Entry (every answer that carries an entry) gains `archived` (bool) and
  `comments_off` (bool).
- Today's `entry` object gains the same two members. When the day's entry is
  archived Today still has `state: "posted"`, `entry` is present and
  `entry.archived` is true.
- `POST /v1/entries/:id/archive` and `POST /v1/entries/:id/unarchive`:
  empty body, 200 `{"entry": Entry, "today": Today}`. Both are idempotent.
  Someone else's entry, a missing one or a bad id is 404 `not_found`. Limit 20
  a minute per person. Needs a time zone, like delete.
- `GET /v1/me/archive`: 200 `{"entries": [Entry, ...]}`, the caller's archived
  entries, newest local date first. Limit 60 a minute per person.
- `PATCH /v1/entries/:id` accepts optional `comments_off` (bool). Missing or
  null leaves it unchanged; a non-bool is 400 `comments_off` invalid.
  Changing it bumps `updated_at` like any edit.

## Rules

- Archiving writes only `archived_at`. The posted day, the spin result, the
  week count and `updated_at` do not change, and the day is not free for
  another entry.
- Unarchive clears `archived_at` and creates nothing.
- Archived entries are not deleted and their photos stay.

## Tests

`internal/httpapi/entry_archive_test.go` (round trip, day still counts, owner
only, comments_off persists, list order) and one case in the shared entry suite
(runs on the in-memory repo and Postgres). Existing shape tests were updated for
the two new members.
