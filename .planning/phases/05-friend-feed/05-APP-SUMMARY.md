# Phase 5 app summary (follows, Board, posts, People, Notifications)

Built 2026-10-16 against `05-FOLLOW-OVERRIDES.md` (binding) and the `api_contract` of
`05-CONTEXT.md`. App files only: nothing in `internal/`, `migrations/` or any Go file. No new
package, no package.json, lockfile or app.json change.

## Wire fields the app reads, per route

The client adds `/v1`. Every field below is read in one file under `lib/api/`; a list that comes
as `null` or is left out is read as empty, a missing string as `''`, a missing boolean as false.

| Route | Sent | Read from the answer | File |
|---|---|---|---|
| `GET /people/lookup?username=` | the username as typed, trimmed, a leading `@` removed | `person{id,name,username,photo_url}`, `relation` (`none`/`requested`/`following`), `follows_you`, `private`. 404 `not_found` is "No one has that username." | `lib/api/social.ts` |
| `POST /follows` | `{user_id}` | `relation` | `social.ts` |
| `POST /follows/{user_id}/unfollow` | `{}` | `relation` (also used to cancel a request) | `social.ts` |
| `POST /followers/{user_id}/remove` | `{}` | nothing | `social.ts` |
| `GET /follows` | none | `followers[Person]`, `following[Person]`, `incoming[{request_id,person,at}]`, `outgoing[{request_id,person,at}]` | `social.ts` |
| `POST /follows/requests/{id}/accept` | `{}` | nothing | `social.ts` |
| `POST /follows/requests/{id}/decline` | `{}` | nothing | `social.ts` |
| `POST /me/blocks` | `{user_id}` | nothing | `lib/api/account.ts` (`blockUser`) |
| `GET /me/blocks`, `DELETE /me/blocks/{user_id}` | unchanged | unchanged (`items[{user{id,name,username,photo_url},blocked_at}]`, 204) | `account.ts` |
| `GET /board?before=&limit=5` | `limit=5`; `before` = the last `next_before` | `state` (`no_follows`/`locked`/`skipped`/`unlocked`; anything else is drawn as unlocked), `day_label` (`today`/`last_night`), `locked{posted_count,names[],category,action}` (`action` other than `log`/`spin` is `wait`), `days[{local_date,label,cards[Card]}]` (a day with no cards is dropped), `next_before` | `lib/api/board.ts` |
| `GET /posts/{entry_id}` | none (a non-UUID id is answered locally as gone) | `card` (Card), `note`, `comments[Comment]` (or `null`), `can_react`, `can_comment`, `comments_off`, `received[{person,reaction,at}]` (or `null`), and **`photos[{id,url,position}]` (NOT in the written contract; read when present, else the card's `photo_url` stands in)** | `board.ts` |
| `POST /posts/{entry_id}/reaction` | `{reaction: name or null}` | `my_reaction` | `board.ts` |
| `POST /posts/{entry_id}/comments` | `{text}` (trimmed, 1 to 140) | `comment` (Comment). 409 `comments_off` prints "Commenting is off." and reloads the post | `board.ts` |
| `POST /comments/{id}/delete` | `{}` | nothing; 404 counts as deleted | `board.ts` |
| `GET /notifications` | none | `unread`, `items[Item]` | `lib/api/notifications.ts` |
| `GET /notifications/summary` | none | `unread` | `notifications.ts` |
| `POST /notifications/seen` | `{}` | nothing (the dot goes out on a 2xx) | `notifications.ts` |
| `GET /me/summary` | none | `followers` (renamed from `friends`), with `spins`, `week_streak`, `tastes`, `recent` as before | `lib/api/summary.ts` |

Shapes as read:
- `Person`: `id` (a person with no `id` string is dropped), `name`, `username`, `photo_url` (empty is null).
- `Card`: `entry_id`, `author` (Person), `mine`, `local_date`, `category{id,name}`, `title`, `stars` (1 to 10), `reaction` (one of the six names, else null), `photo_url`, `late`, `posted_at`, `posted_local_time` (printed as sent), `comment_count` (number, or null when not a number), `my_reaction`.
- `Comment`: `id`, `author` (Person), `text`, `at`, `at_label` (printed as sent), `can_delete`.
- `Item`: `id`, `kind` (`reaction`/`comment`/`follow`/`follow_request`/`follow_accepted`/`spin_open`/`bonus_spin`; any other kind is dropped), `at`, `at_label` (printed as sent), `section` (`today`/`week`, anything else `earlier`), `unread`, `actor` (Person or null), `entry_id`, `category_name`, `reaction`, `text`, `request_id`.
- Reaction names are checked against `lib/entry/reactions.ts` (love, fire, laughing, wow, sad, sleepy); an unknown name is read as null.
- Error codes added to `ApiErrorCode`: `own_post`, `comments_off`.

## Files

Added:
- `lib/api/social.ts`, `lib/api/board.ts`, `lib/api/notifications.ts`
- `lib/board/text.ts` (the Board's words, pure), `lib/notifications/unreadStore.ts` (the bell's dot)
- `components/board/BoardScreen.tsx`, `BoardBlocks.tsx` (locked, skipped, no follows), `PostCard.tsx` (card and name chip), `PostView.tsx` (someone else's post), `Comments.tsx` (list, bar, "Commenting is off.", received reactions), `useBoard.ts`, `usePost.ts`
- `components/notifications/BellButton.tsx` (bell and unread dot, drawn with svg)
- `components/people/PeopleScreen.tsx`
- Routes: `app/(app)/board/_layout.tsx`, `index.tsx`, `post.tsx`, `entry.tsx`, `edit-entry.tsx`, `people.tsx`, `notifications.tsx`; `app/(app)/profile/people.tsx`

Changed:
- `app/(app)/board.tsx` removed (the folder replaces it). `app/(app)/_layout.tsx`: `BARLESS_BOARD_PAGES` (post, entry, edit-entry, notifications); still four tabs, three `TabIcon`, three `TabLabel`. `profile/_layout.tsx` registers `people`. `today/entry.tsx` reads `via=notifications` for the Back name.
- `components/entry/EntryView.tsx`: `Gallery` exported (with an `aspect` prop); under the details the received reactions and the comments; the comment bar pinned at the bottom (or "Commenting is off."); the menu's commenting switch reloads them. The three-dots menu is unchanged.
- `components/entry/entryFrom.ts`: `board` (Back name 'Board', edit route in the Board stack).
- `components/today/NotificationsScreen.tsx`: the real list. `TodayScreen.tsx`: the bell is `BellButton` with the dot.
- `components/profile/StatsRow.tsx`, `summaryText.ts`, `ProfileScreen.tsx`, `lib/api/summary.ts`: Followers, a button to People.
- `lib/api/account.ts` (`blockUser`), `lib/api/types.ts` (two codes), `lib/theme/tokens.ts` (a comment naming the two new round shapes).

## Choices to know about

- Someone else's post shows its photos 3 wide to 2 high (picture 10); your own entry page stays square. One prop (`aspect`) to change.
- Comment delete and Block ask with the phone's own alert, without a red button (the shared sheet fades, so it is not used).
- Block is offered from People only (the result row's dots, Followers and Following sheets), as the overrides say; the post page has no Block link.
- No follows: a plain prompt and "Find people"; the person's own diary rows are not repeated there.
- Pending requests you sent are listed under Following with "Requested" (tap cancels).
- A notification tap from the Spin page's bell opens your entry in the Spin stack and People in the Profile stack; from the Board's bell both stay in the Board stack, so Back returns to the Board.
- The bonus line has no number on the wire, so it reads "You hit your goal this week. One bonus spin added."

## Gates

`npx tsc --noEmit` exit 0; `check-entry`, `check-wheel`, `check-settings`, `check-navigation-motion` pass; `npx expo export --platform ios` succeeds; no em dash, the spin wording kept, no hex, no emoji or reaction name outside `lib/entry/reactions.ts` in the new and changed files.

## Check first on the phone

1. Cold launch: four tabs, Spin in the middle; tap Board with the server live (not "Couldn't connect").
2. Board states with two test accounts: no follows (Find people), locked (tiles and Log your ...), after posting the grid; your own card opens your entry page with Back 'Board'.
3. Someone else's post: swipe photos (if the server sends `photos`), tap a reaction then tap it again to clear, post a comment, delete it; with commenting off on their side the bar reads "Commenting is off.".
4. Your entry page: who reacted, the comments, the bar with the keyboard up (it should sit above the keyboard).
5. People: find by username, Follow a public account (Following at once) and a private one (Requested, then Accept from the other phone's bell).
6. The dot on both bells, gone after opening Notifications; Back from Board's Notifications lands on the Board.
