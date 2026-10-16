# Phase 5 server: as built (2026-10-16)

Built against `05-FOLLOW-OVERRIDES.md` (binding), `05-CONTEXT.md` (`api_contract`) and the
structure of `05-01` to `05-03`, with friends replaced by one-way follows. Server only: nothing in
`app/`, `components/`, `lib/`, `scripts/`, `package.json`, the lockfile or `app.json` changed.

## Commits and tests

- Code: `5b8d366` feat(server): follows, blocks, Board, posts, reactions, comments and
  notifications (Phase 5). This summary is its own commit after it.
- Gates: `go build ./... && go vet ./... && gofmt -l .` clean.
- Full suite on `rndmroll_test` with `-p 1`: 412 top-level tests pass (baseline 391), 979 with
  subtests (baseline 958), 0 fail, 0 skip.
- New tests:
  - `internal/httpapi/feed_integration_test.go` (Postgres, real server, one Fixed clock):
    - a public account is followed at once;
    - a private account's request is accepted and declined;
    - cancel, unfollow and remove follower;
    - switching to public accepts pending requests silently;
    - a block removes follows both ways and hides lookup;
    - the gate (posted day opens, skip does not, archived hidden, blocked hidden), with paging and
      `last_night`;
    - reactions (`own_post`) and comments (`comments_off`);
    - notifications filtered by the settings, unread and seen;
    - the followers count.
  - `internal/store/postgres/follow_feed_test.go`: 0008 reversibility, database refusals, the same
    reaction list as `entries_reaction_check`, and cascade.
  - Pure tests in `internal/social`, `internal/board`, `internal/notifications` and
    `internal/spin/labels_test.go`.
- The existing 0002 to 0006 migration tests walk through 0008 down and up and still pass.

## Code map

- `migrations/0008_follow_feed.{up,down}.sql`. Tables:
  - `follows`: `id`, `follower_id`, `followee_id`, `status` (`pending` or `accepted`), `via`
    (`direct`, `request` or `silent`), `requested_at`, `accepted_at`. Unique pair; not self.
  - `blocks`;
  - `notification_settings`;
  - `post_reactions`;
  - `post_comments`;
  - `notification_seen`.
  - Every table cascades from `users`; reactions and comments also cascade from `entries`.
- `internal/social`: model, `CleanUsername`, the ONE rule `CanView`, `CanReact`, `CanComment`,
  `CanDeleteComment` (`visibility.go`), and the service.
- `internal/board`: the Board, the post page, reactions and comments.
- `internal/notifications`: the derived list.
- `internal/spin/board_facts.go`: `DayInPlay`, `BoardFacts` (under the person lock, using the Today
  rules for `action`) and `ViewerClock`.
- `internal/spin/labels.go`: `ClockOf`, `PostedLocalTime`, `ViewerLabel`, `DaysBefore`.
- Postgres repos: `social_repo.go`, `board_repo.go`, `notifications_repo.go`.
- HTTP: `follow.go`, `board.go`, `notifications.go`, `feed_errors.go`.
- Wiring: `Deps.Follow`, `Deps.Board` and `Deps.Notifications` in `server.go`, `cmd/api/main.go`
  and `buildIntegrationServerWith`.
- The visibility hook: `ProfileHandler.OnBecamePublic`, set by `NewServer` from the follow
  handler. It runs after a saved `PATCH /v1/me` with `"account_visibility":"public"`.

## Wire shapes (exact, as built)

Conventions for every route:
- Instants are strings like `"2026-10-08T20:30:00.000Z"` (RFC 3339 UTC, milliseconds). Dates are
  `"YYYY-MM-DD"`.
- Lists are always `[]`, never `null`, unless a field is marked `|null`.
- Every route is on the signed-in group:
  - no token or a deleted account: 401;
  - an unfinished account: 403 `profile_incomplete`;
  - a body over 16 KB: 413;
  - a body member of the wrong JSON type: 400 `validation_failed` with `field`.
- A path id that is not a UUID is 404 `not_found`. Every visibility failure is 404 `not_found`,
  with a body identical to a miss: `{"error":"not_found"}`.
- An empty body counts as `{}`.

Error codes added (join `ApiErrorCode`): `own_post` (409) and `comments_off` (409). The others
are reused: `not_found`, `validation_failed` (body `{"error","field","message"}`),
`profile_incomplete`, `rate_limited`, `payload_too_large`.

```
Person   = {"id": uuid string, "name": string, "username": string, "photo_url": string|null}
Relation = "none" | "requested" | "following"
Req      = {"request_id": uuid string, "person": Person, "at": instant}
```

### Follows, people, blocks, settings

| Route | Limit/min | Request | Success | Failures |
|---|---|---|---|---|
| `GET /v1/people/lookup?username=` | 30 | exact, case-insensitive, one leading `@` allowed | 200 `{"person":Person,"relation":Relation,"follows_you":bool,"private":bool}` | 400 field `username` (`required`, `invalid`); 404 for a miss, yourself, or a block either way |
| `GET /v1/follows` | 60 | none | 200 `{"followers":[Person],"following":[Person],"incoming":[Req],"outgoing":[Req]}` | none |
| `POST /v1/follows` | 20 | `{"user_id": uuid}` | 200 `{"relation":"following"\|"requested"}` (a repeat answers the current relation) | 400 field `user_id` (`required`, `invalid`); 404 for yourself, an unknown or unfinished account, or a block either way |
| `POST /v1/follows/{user_id}/unfollow` | 20 | `{}` | 200 `{"relation":"none"}` (ends a follow or cancels a request; absent is fine) | 404 only for a non-UUID id |
| `POST /v1/followers/{user_id}/remove` | 20 | `{}` | 200 `{}` (also removes a pending request from that person; absent is fine) | none |
| `POST /v1/follows/requests/{id}/accept` | 20 | `{}` | 200 `{}` | 404 unless it is a pending request addressed to you |
| `POST /v1/follows/requests/{id}/decline` | 20 | `{}` | 200 `{}` (silent) | 404 as for accept |
| `GET /v1/me/blocks` | 60 | none | 200 `{"items":[{"user":Person,"blocked_at":instant}]}` | none |
| `POST /v1/me/blocks` | 20 | `{"user_id": uuid}` | 200 `{}` (a repeat is fine; deletes follows and requests both ways) | 400 field `user_id`; 404 for yourself or an unknown id |
| `DELETE /v1/me/blocks/{user_id}` | 20 | none | 204, no body (absent is fine; restores nothing) | 404 for a non-UUID id |
| `GET /v1/me/notification-settings` | 60 | none | 200 `{"reactions":bool,"comments":bool,"milestones":bool}`, all true by default | none |
| `PATCH /v1/me/notification-settings` | 30 | any of the three booleans (missing or null is unchanged) | 200, the same body as stored | 400 field `reactions`, `comments` or `milestones` (`invalid`) for a non-boolean |

Order inside the `GET /v1/follows` lists:
- `followers` and `following`: by lower-cased name, then username.
- `incoming` and `outgoing`: newest first.

The lists, lookup, Board and notifications never show a person with a block either way.

### Board, posts, reactions, comments

```
Card = {"entry_id": uuid, "author": Person, "mine": bool, "local_date": date,
        "category": {"id": string, "name": string}, "title": string, "stars": 1..10,
        "reaction": name|null, "photo_url": string, "late": bool, "posted_at": instant,
        "posted_local_time": "8:41 PM", "comment_count": int, "my_reaction": name|null}
Comment = {"id": uuid, "author": Person, "text": string, "at": instant, "at_label": string,
           "can_delete": bool}
Photo = {"id": string, "url": string, "position": int}
Post = {"card": Card, "note": string|null, "photos": [Photo], "comments": [Comment],
        "can_react": bool, "can_comment": bool, "comments_off": bool,
        "received": [{"person": Person, "reaction": name, "at": instant}] | null}
Board = {"state": "no_follows"|"locked"|"skipped"|"unlocked",
         "day_label": "today"|"last_night",
         "locked": {"posted_count": int, "names": [string], "category": string|null,
                    "action": "log"|"spin"|"wait"} | null,
         "days": [{"local_date": date, "label": "today"|"yesterday"|"earlier", "cards": [Card]}],
         "next_before": date|null}
```

- `Card` fields:
  - `reaction` is the author's own reaction; `my_reaction` is the caller's.
  - `photo_url` is the cover photo's link (position order, first).
  - `posted_local_time` is the time of day in the AUTHOR's zone (the entry's spin zone; the viewer's
    zone if that name does not load).
  - `comment_count` is a number (comments by someone with a block with the viewer are not counted).
- `Post` fields:
  - `photos` lists every photo in position order. It has the same shape as Entry's `photos`
    (`entryView`), so the post page can swipe them.
  - `note` is the author's own comment field.
  - `comments` are oldest first; `at_label` is written in the VIEWER's zone.
  - `received` is a list (newest change first) only when `card.mine` is true, else `null`.
  - `can_react` is false on your own post.
  - `can_comment` is false when `comments_off` is true.
- Labels (`at_label`, also used by notifications):
  - the time of day (`"8:52 PM"`) for an instant on the viewer's current local day, or later;
  - the short weekday (`"Mon"`) for the six days before;
  - the month and day (`"Oct 3"`) before that.
- Board fields:
  - `day_label` is `last_night` before 8:00 PM local (the day in play is the previous evening), and
    `today` from 8:00 PM.
  - The states are decided in order:
    1. `no_follows`: the caller follows nobody (accepted). `days` is empty and `locked` is null.
    2. `unlocked`: a posted day exists for the day in play.
    3. `skipped`: the day in play's latest spin was skipped.
    4. `locked`: otherwise.
  - `locked` is set only in the `locked` state:
    - `posted_count` is the number of followed people with a posted day for the day in play;
    - `names` are up to 4 first names of followed people with a visible entry that day, in posting
      order;
    - `category` is the shown spin's category as written, when `action` is `log`;
    - `action` is `log` (a spin can still be posted), `spin` (the window is open and there is no
      spin yet) or `wait`.
  - `days` lists the caller's posted dates up to the day in play, newest first. They are paged by
    `before` (exclusive) and `limit` (days, 1 to 10, default 5). Under a lock or a skip that means
    the earlier posted days only.
  - Each day's cards are newest first. They are the caller's own entries plus those of followed
    people, with no block either way, not archived. A date with no cards is left out but counts
    toward the page.
  - `next_before` is the page's last date when more posted dates remain, else null.

| Route | Limit/min | Request | Success | Failures (in check order) |
|---|---|---|---|---|
| `GET /v1/board` | 60 | `?before=YYYY-MM-DD&limit=1..10` | 200 `Board` | 400 field `before` or `limit` (`invalid`); 400 field `time_zone` (`required`) with no stored zone |
| `GET /v1/posts/{entry_id}` | 60 | none | 200 `Post` | 400 field `time_zone`; 404 |
| `POST /v1/posts/{entry_id}/reaction` | 60 | `{"reaction": name\|null}`, key required | 200 `{"my_reaction": name\|null}` | 400 field `reaction` (`required` when the key is missing, `invalid` for a non-string); 404; 409 `own_post`; 400 field `reaction` (`invalid` for an unknown name) |
| `POST /v1/posts/{entry_id}/comments` | 20 | `{"text": string}` | 201 `{"comment": Comment}` | 400 field `text` (`invalid`) for a non-string; 404; 409 `comments_off`; 400 field `text` (`required`, `too_long`, `invalid`) |
| `POST /v1/comments/{id}/delete` | 20 | `{}` | 200 `{}` | 404 (missing, or neither the writer nor the post's owner) |

Reaction and comment rules:
- Reaction names are the six of `spin.Reactions`: love, fire, laughing, wow, sad, sleepy.
- One reaction per person per post. Setting the same name again changes nothing (its time is not
  moved); null clears it, and a repeat clear is fine.
- Comments are trimmed, `\r\n` becomes `\n`, and they are 1 to 140 characters. A control character
  other than a line break is `invalid`.

### Notifications

```
Item = {"id": string, "kind": "reaction"|"comment"|"follow"|"follow_request"|"follow_accepted"|"spin_open"|"bonus_spin",
        "at": instant, "at_label": string, "section": "today"|"week"|"earlier", "unread": bool,
        "actor": Person|null, "entry_id": uuid|null, "category_name": string|null,
        "reaction": name|null, "text": string|null, "request_id": uuid|null}
```

| Route | Limit/min | Request | Success | Failures |
|---|---|---|---|---|
| `GET /v1/notifications` | 60 | none | 200 `{"unread":bool,"items":[Item]}` (at most 50, newest first, ties by id descending) | 400 field `time_zone` |
| `GET /v1/notifications/summary` | 120 | none | 200 `{"unread":bool}` (always equal to the list's `unread`) | 400 field `time_zone` |
| `POST /v1/notifications/seen` | 30 | `{}` | 200 `{"unread":false}` (the seen time only moves forward) | 400 field `time_zone` |

Each kind of item:

| Kind | `id` | `actor` | Other fields | When it shows |
|---|---|---|---|---|
| `reaction` | `reaction:<entry_id>:<actor_id>` | the reactor | `entry_id`, `category_name` (as written), `reaction`; `at` is the reaction's last change | someone else reacted to your entry |
| `comment` | `comment:<comment_id>` | the writer | `entry_id`, `category_name`, `text` (cut at 60 characters plus `...`, line breaks as spaces) | someone else commented on your entry |
| `follow` | `follow:<follower_id>` | the follower | none | someone followed your PUBLIC account at once |
| `follow_request` | `follow_request:<request_id>` | the requester | `request_id`, which works with accept and decline | only while the request is pending, whatever its age |
| `follow_accepted` | `follow_accepted:<followee_id>` | the person who accepted | none | your request was accepted |
| `spin_open` | `spin_open:<date>` | none | `at` is 8:00 PM local | only from 8:00 PM until local midnight |
| `bonus_spin` | `bonus_spin:<week_start>` | none | `text` is the weekly goal as a decimal string; `at` is when the goal was met | a granted bonus |

Window, sections and unread:
- Every item kind except pending requests is limited to the last 30 days.
- `section` and `at_label` come from the same day count in the viewer's zone: `today` reads as a
  time of day, `week` (the six days before) as a weekday, and `earlier` as a month and day.
- An item is `unread` when its `at` is after the seen time, or when there is no seen row.

Exclusions:
- your own actions;
- anyone with a block either way;
- the items of a deleted post (cascade).

Settings filter the list, the unread flag and the summary:
- `reactions` off hides `reaction` items;
- `comments` off hides `comment` items;
- `milestones` off hides `spin_open` and `bonus_spin` items;
- follow items always show.

### Profile summary

`GET /v1/me/summary`: the field `friends` is now `followers`, the count of accepted followers with
no block. The rest is unchanged:
`{"followers":int,"recent":[...],"spins":int,"tastes":[...],"week_streak":int}`.

## Where this differs from the overrides or the papers

### Additions to the wire contract

1. `Post.photos` is added (the coordinator's addition): `[{"id","url","position"}]` in position
   order, the same shape as Entry's photos. It is tested in the gate test.
2. Lookup refuses a value that cannot be a username with 400 field `username` (`required`,
   `invalid`). A well-formed miss, yourself and a block are all the same 404.
3. A bad `user_id` body on `POST /v1/follows` or `POST /v1/me/blocks` is 400 field `user_id`. An
   unknown or unfinished account is 404.

### Behaviour choices

4. Archived entries:
   - they are left out of every Board list, the owner's own included (as in the Diary);
   - the owner still reads them at `GET /v1/posts/{id}`;
   - nobody else sees them.
5. `no_follows` lists no days, even when the caller has posted. The app shows its own Diary there,
   as the paper says.
6. Remove follower also deletes a pending request from that person.
7. Unfollow, remove follower and unblock are idempotent: an unknown id answers success, not 404.
8. `comments_off` refuses the owner's own new comments too (409 `comments_off`). Existing comments
   stay and can still be deleted by their writer or the owner.
9. Switching to public accepts pending requests with `via = silent`. Neither side gets an item.
   Switching to private keeps every follower.
10. Pending `follow_request` items are not cut at 30 days, so a request is listed for as long as
    it waits.

### Implementation choices

11. Check order:
    - reaction: not visible (404), own post (409), then name (400);
    - comment: not visible (404), `comments_off` (409), then text (400).
    - A wrong JSON type is refused first, at the bind.
12. The summary is computed from the same filtered list, not from separate `exists` queries, so it
    can never disagree with the list.
13. Time zones:
    - the post page and all three notification routes need a stored zone (400 field `time_zone`), as
      Today does;
    - adding a comment without a stored zone works, and its returned `at_label` uses UTC.

### Left out

14. Not built:
    - the test fixtures (`/v1/test/friends`, `/v1/test/reset-social`) and their resetters;
    - in-memory repositories and shared suites;
    - the paper's 50 pending request cap and its `request_limit` code;
    - the paper's `already_friends` and `request_exists` codes.
    - Repeats are idempotent instead of using those codes.
    - Tests run on Postgres through the real server, plus pure unit tests.

### Shared test database

15. The shared test database is restored to migration version 7 after the suite run, so the main
    tree, which has no 0008 file yet, can still run its tests. The suite migrates it to 8 again on
    its next run from this branch.

## Replies (added 2026-10-16)

Instagram-style replies to comments, one level. Server commit `44c8f20`, app commit `80e3662`.

- Migration `0009_comment_replies` (up and down): `post_comments.parent_id uuid null references
  post_comments(id) on delete cascade`, and the index `post_comments_parent_idx (parent_id,
  created_at, id) where parent_id is not null`. Down drops both; replies stay as plain comments.
- `POST /v1/posts/{entry_id}/comments` body: `{"text": string, "parent_id": uuid|null}`.
  `parent_id` is optional; missing or null is a top-level comment.
  - A `parent_id` that is not a string, or a string that is not a UUID (`""` too), is 400 field
    `parent_id` (`invalid`). It is checked at the bind, before anything else.
  - Check order after the bind: post not visible (404), `comments_off` (409, for replies too, and
    before the parent is looked at), parent not found (404 `not_found`), then the text (400, the
    same rules as a comment).
  - "Not found" for the parent means: not on this post, missing, or not shown to the caller (its
    writer, or for a reply its top-level comment's writer, has a block with the caller).
  - A reply to a reply is stored under that reply's top-level comment. The answer's `parent_id`
    is always a top-level comment id.
- `Comment` = `{"id","author":Person,"text","at","at_label","can_delete","parent_id": uuid|null}`.
  The key is always present: null on a top-level comment, the top-level comment's id on a reply.
- `Post.comments` stays ONE flat list: top-level comments oldest first, each followed directly
  by its replies oldest first.
- Blocks hide a reply like a comment. A reply under a top-level comment hidden from the viewer is
  left out too, and is not counted. `Card.comment_count` and the post's count include replies and
  use the same rule.
- Delete is unchanged (`POST /v1/comments/{id}/delete`, the writer or the post's owner). Deleting
  a top-level comment deletes its replies (cascade). The top-level comment's writer cannot delete
  replies under it.
- New notification kind `"reply"`: `Item.kind` adds `"reply"`.
  - `id` is `reply:<reply comment id>` and `actor` is the reply's writer.
  - It also carries `entry_id`, `category_name` (as written) and `text` (cut at 60 plus `...`, as
    for `comment`); `reaction` and `request_id` are null.
  - It goes to the writer of the top-level comment the reply is stored under, unless they wrote
    the reply.
  - It shows only while that post is visible to them by the `CanView` rule, and with no block
    with the reply's writer.
  - The post's owner still gets a `comment` item (`comment:<reply id>`) for a reply by someone
    else. The exception: when the owner wrote the top-level comment, the owner gets only the
    `reply` item. A reply under a top-level comment whose writer has a block with the owner makes
    no item.
  - The `comments` setting off hides `reply` items too, and they no longer count for the unread
    flag.
- The item does not say whose post it is. The app reads `GET /v1/posts/{entry_id}` on tap:
  `card.mine` opens your entry page, otherwise the post page. The Spin stack gains
  `app/(app)/today/post.tsx`.
- Tests:
  - `internal/httpapi/replies_integration_test.go`: the round trip, the order, a reply to a reply
    flattened, a parent on another post (404), a bad `parent_id` (400), `comments_off`, deleting
    replies, blocks, notifications with no duplicate for the owner, and the settings filter;
  - `internal/store/postgres/comment_replies_test.go`: 0009 down and up, plus cascade;
  - `TestThread` in `internal/board`.
- Full suite on `rndmroll_test` with `-p 1`: 417 top-level tests pass, 984 with subtests, 0 fail,
  0 skip.
- The shared test database is put back to migration version **8** after the run. The suite
  migrates it to 9 again on its next run from this branch.
