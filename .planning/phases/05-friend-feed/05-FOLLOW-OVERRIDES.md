# Phase 5: Follow overrides (BINDING, 2026-10-16)

> **CHANGE AFTER THE BUILD (developer, 2026-10-16):** there are no public accounts. Every follow is
> a request the other person accepts or declines (still one-way). The server ignores
> `account_visibility`, the lookup's `private` is always true, the "started following you" item is
> never made, the Settings "Account privacy" page is gone, and requests are answered on their own
> "Follow requests" page, opened from one row at the top of the bell list. Sections below that
> talk about public accounts are history.

This file WINS over `05-CONTEXT.md`, every `05-0*-PLAN.md`, `05-UI-SPEC.md` and `05-HAND-EDITS.md`
wherever they disagree. Those papers were written for mutual friends; the developer decided on
2026-10-13 that following works like Instagram (one-way). Read the papers for the design of the
Board, the post page, reactions, comments and notifications; read this file for what changed.

## 1. Follows replace friends

- Following is one-way. A follows B does not mean B follows A.
- B is PUBLIC (`users.account_visibility = 'public'`, migration 0005, the default): A follows at
  once. B gets the notification "{Name} started following you."
- B is PRIVATE: A's follow becomes a REQUEST. B sees "{Name} wants to follow you." in the bell
  list with Accept and Decline on the row. Accept: A follows B, A gets "{Name} accepted your
  request." Decline is silent (the request is gone, A sees "Follow" again).
- A can cancel a pending request. Unfollow is silent. B can remove a follower (silent).
- Switching an account from public to private keeps existing followers. Switching to public
  accepts every pending request at once (no notification for those).
- People are found by exact username (case-insensitive), never by email.
- Paper D-14 ("every post is friends-only, nothing reads the public or private choice") is OUT:
  the choice now decides follow-at-once versus request.
- Phase 5 still shows posts only to the owner and to accepted followers (no Global; that is
  Phase 6). A public account's posts are NOT shown to non-followers in Phase 5.

## 2. Blocks

- A block removes the follow in BOTH directions and any request either way, hides each person
  from the other everywhere (lookup, lists, Board, posts, comments, notifications), and stops new
  follows or requests. Unblock restores nothing. The blocked person is never told.
- Block is offered from a person row's action sheet (Find people result, Followers, Following).

## 3. The gate (unchanged in spirit)

A viewer sees an entry iff the viewer is its owner, OR (the viewer follows the owner with an
accepted follow, nobody blocked anybody, the entry is not archived, AND the viewer has a posted
day for the entry's `local_date`). Archived entries are visible to the owner only. Deleted
entries are gone. Paper D-01 (a skip does not open the day) and D-09 (posted days, not entries)
stand.

## 4. Comments obey `comments_off`

`entries.comments_off` (migration 0007) true: the server refuses new comments (409
`comments_off`), `Post.can_comment` is false, and the app hides the comment bar and shows the
quiet line "Commenting is off." Existing comments stay visible and can still be deleted by their
writer or the owner.

## 5. Navigation and look (the developer's current rules)

- The bar is FOUR tabs: Diary, Spin (the raised circle, the `today` folder), Board, Profile.
  Paper D-15 and every "bar of five" or "Home" mention are OUT. "Go to Home" / "Go to Today"
  become "Go to Spin" and route to `/(app)/today`.
- `app/(app)/today/notifications.tsx` STAYS (05-06 wanted to remove it; do not). The bell on the
  Spin page and on the Board opens it; the unread dot sits on both bells.
- Every file directly in `app/(app)/` becomes a tab: new pages go inside a stack folder. Board
  becomes `app/(app)/board/` with its own Stack (index, post, people, entry, edit-entry). Add each
  no-bar page to the right `BARLESS_*` list in `app/(app)/_layout.tsx`. Board keeps its `TabIcon`
  and `TabLabel` (the navigation check asserts exactly three of each).
- The entry page is the CURRENT one (`components/entry/EntryView.tsx`): edge-to-edge gallery and
  the three-dots menu, no footer. The papers' "square photo and Edit and Delete footer" is out of
  date. Received reactions and the comments go under the details; the comment bar at the bottom.
- House rules: no fades or cross-fades, no red text except Log out, no em dashes, the word "spin"
  never "roll", plain light cards (no glass, no blur), no music, no new packages, no package.json,
  lockfile or app.json changes, never `expo lint`.
- Reactions are the six names already shared with Phase 3 (`lib/entry/reactions.ts`): love,
  fire, laughing, wow, sad, sleepy. One per person per post; tap another to change, the same to
  clear; never on your own post.

## 6. Wire contract (binding; replaces the friends rows of the paper's table)

Instants RFC 3339 UTC, snake_case, dates `YYYY-MM-DD`. Signed-in group, `OnboardingComplete`
first, user from the token only, per-subject rate limits as the paper. Ids in paths are UUIDs.
Visibility failures are 404 `not_found`, never 403. The app client adds `/v1`.

`Person`: `{"id","name","username","photo_url":string|null}`.
`Relation` (the caller toward the other person): `"none" | "requested" | "following"`.

| Route | Request | Success |
|---|---|---|
| `GET /v1/people/lookup?username=` | exact, case-insensitive | 200 `{"person":Person,"relation":Relation,"follows_you":bool,"private":bool}`; 404 for a miss, yourself, or someone who blocked you or whom you blocked |
| `POST /v1/follows` | `{"user_id"}` | 200 `{"relation":"following"|"requested"}` (repeat is idempotent) |
| `POST /v1/follows/{user_id}/unfollow` | `{}` | 200 `{"relation":"none"}` (also cancels a pending request) |
| `POST /v1/followers/{user_id}/remove` | `{}` | 200 `{}` |
| `GET /v1/follows` | none | 200 `{"followers":[Person],"following":[Person],"incoming":[Req],"outgoing":[Req]}`, `Req` = `{"request_id","person":Person,"at"}` |
| `POST /v1/follows/requests/{id}/accept` | `{}` | 200 `{}` |
| `POST /v1/follows/requests/{id}/decline` | `{}` | 200 `{}` |
| `GET /v1/me/blocks` | none | 200 `{"items":[{"user":{"id","name","username","photo_url"},"blocked_at"}]}` (EXACTLY what `lib/api/account.ts` reads today) |
| `POST /v1/me/blocks` | `{"user_id"}` | 200 `{}` (removes follows both ways and requests) |
| `DELETE /v1/me/blocks/{user_id}` | none | 204 (EXACTLY what `lib/api/account.ts` sends today) |
| `GET /v1/me/notification-settings` | none | 200 `{"reactions":bool,"comments":bool,"milestones":bool}`, all true by default |
| `PATCH /v1/me/notification-settings` | any of the three booleans | 200 the same body as stored |
| `GET /v1/board` | `before` date optional, `limit` days 1 to 10 (default 5) | 200 `Board` |
| `GET /v1/posts/{entry_id}` | none | 200 `Post` |
| `POST /v1/posts/{entry_id}/reaction` | `{"reaction":name|null}` | 200 `{"my_reaction":name|null}`; 409 `own_post` on your own |
| `POST /v1/posts/{entry_id}/comments` | `{"text"}` 1 to 140 chars | 201 `{"comment":Comment}`; 409 `comments_off` |
| `POST /v1/comments/{id}/delete` | `{}` | 200 `{}` |
| `GET /v1/notifications` | none | 200 `{"unread":bool,"items":[Item]}` |
| `GET /v1/notifications/summary` | none | 200 `{"unread":bool}` |
| `POST /v1/notifications/seen` | `{}` | 200 `{"unread":false}` |

`Card`, `Comment` and `Post` are as in `05-CONTEXT.md` (the `api_contract` section), with
`Post.can_comment` false when `comments_off` is on, and `Post.comments_off: bool` added.

`Board`: `{"state":"no_follows"|"locked"|"skipped"|"unlocked","day_label":"today"|"last_night","locked":{"posted_count":int,"names":[string],"category":string|null,"action":"log"|"spin"|"wait"}|null,"days":[{"local_date","label":"today"|"yesterday"|"earlier","cards":[Card]}],"next_before":date|null}`.
`no_follows` means the caller follows nobody (accepted). Copy uses "people you follow", e.g.
"3 people you follow posted today." / "1 person you follow posted today." /
"No one you follow has posted yet." / "Follow people to see their spins here." with a
"Find people" button.

`Item`: `{"id","kind":"reaction"|"comment"|"follow"|"follow_request"|"follow_accepted"|"spin_open"|"bonus_spin","at","at_label","section":"today"|"week"|"earlier","unread":bool,"actor":Person|null,"entry_id":uuid|null,"category_name":string|null,"reaction":name|null,"text":string|null,"request_id":uuid|null}`.
`follow_request` items exist only while the request is pending (accepting or declining removes
the item). Copy: "{Name} started following you." / "{Name} wants to follow you." /
"{Name} accepted your request." / the reaction, comment, spin and bonus lines of the paper.

Notification settings FILTER the list (and the unread flag): `reactions` off hides `reaction`
items, `comments` off hides `comment` items, `milestones` off hides `spin_open` and `bonus_spin`.
Follow items always show.

`GET /v1/me/summary`: the field `friends` is RENAMED `followers` (accepted followers count). The
app changes `lib/api/summary.ts`, `components/profile/StatsRow.tsx` and its spoken text in the
same phase; the figure reads "Followers" and is a button to the People page.

## 7. The People page (replaces the paper's Friends screen)

Opened from the Followers figure on Profile, from the Board's "Find people" button and from a
follow-type notification. Thin routes in the Profile and Board stacks. Top to bottom: a username
box with Find; the result row (photo, name, @username, one button: "Follow", "Requested"
(tap cancels), "Following" (tap opens Unfollow / Block), "Follow back" when they follow you);
"Requests" (incoming, Accept and Decline, only for a private account); "Followers" (row action
sheet: Remove follower, Block); "Following" (row action sheet: Unfollow, Block). Buttons use the
existing `Chip` (strong for Follow, outline for Requested and Following). No red.

## 8. Migration

ONE migration `0008` holds follows (with status pending or accepted), blocks, notification
settings, reactions, comments and the notification seen time. Only the server agent touches
`internal/` and `migrations/`; only the app agent touches app files.

## 9. Changes after the build (developer, 2026-10-16 and 2026-10-17)

- Search: `GET /v1/people/search?q=` (start of a username, a name or a word of a name; self and
  blocks left out; at most 20, closest first) answers `{people:[{person, relation, follows_you}]}`.
  It lives behind the search icon on the Board; the People page no longer has a username box.
- `/me/summary` also sends `following` (accepted follows the person has). Profile shows four
  figures: Spins, Week streak, Followers, Following. Followers and Following open People on that
  tab (`show=followers|following`); People has the two lists as tabs, with Requests on top of
  Followers and "Follow back" on a follower you do not follow.
- A `follow` notification ("started following you") is now also made when the owner accepts a
  request. It keeps the request's time, so the owner's own Accept lights no dot. Its row has
  Follow back, Requested or Following at the right.
- Notification items send `photo_url` (the post's first photo, "" when none) for reactions,
  comments and replies; the row shows it small at the right, like Instagram.
- The Follow requests row shows only a dot when one waits: no name, no count, no photo.
- **Reactions on posts are REMOVED** (developer, 2026-10-18: "we're gonna have that in the
  comments anyway, so I feel like it's a redundancy. Let's not keep it anywhere"). No reaction row
  on someone's post, no who-reacted line or sheet on your own post, no reaction items in the
  bell (the server stops making them), and no "Reactions and likes" switch in Settings >
  Notifications (its stored value is kept and no longer changes anything). Left in place on the
  server and unused: `POST /v1/posts/{id}/reaction`, the `post_reactions` table, and `received`,
  `can_react` and `my_reaction` on the wire; dropping them needs a migration and is a later
  clean-up. The author's own emoji on an entry (picked when posting, shown next to the stars)
  is a different thing and stays.
- The comment box has Instagram's quick row of eight emojis above it; a tap adds one at the end.
  Return still makes a new line (Return-to-send was tried and taken back).
- `GET /v1/diary` sends `social` on every entry row: `{comment_count, latest_comments: [{author:
  {id, name, username, photo_url}, text}]}`, the newest comment only, counted as the post page
  shows the owner (no blocks either way). The Profile posts list follows the rhythm of
  Instagram's list of a profile's posts with this app's own parts (developer, 2026-10-18): under
  the photo an action row (a comment mark at the left that opens the post, your stars and own
  emoji at the right; no likes, saves or shares, none exist here), then the category in bold and
  the day ("Song · Sunday, October 18") one step smaller, the title in bold over the full width,
  your note; then, close under it, "View all comments" (no number: it can grow large) when there is more than one, the newest comment on one
  line (the writer's small photo, the first name in bold, the words, cut with dots), and "Add a
  comment" with your own photo. One font, all ink, groups set apart by space, not rules. Every
  comment line opens the entry page. The list makes no read per post.
- Under each comment, "Reply" and "Hide" (or "Unhide", someone's comment on your own post) sit
  side by side in ink at the label size in the button weight, never underlined (developer,
  2026-10-18: "I don't want anything to be just plain text with things underlined ever"). Delete
  moved off the row to a long press on the comment, asked first with the phone's alert; a
  screen reader gets it as the comment's "Delete" action. Hide itself is the other session's
  (0011, `can_hide` and `hidden` on comments); a hidden comment is also left out of the posts
  list's count and preview. Less space between the post's details and its comments.
- Your entry page matches the posts list (developer, 2026-10-18): the photos 4 wide to 5 high,
  then the same ActionRow and CaptionText (`components/entry/PostCaption.tsx`, shared by both;
  the comment mark here puts the cursor in the comment box), then a hairline and the comments.
  The "2 comments" count line above them is gone from this page (the number can grow large).
  Someone else's post page reads the same way: the author's photo and first name on top, the
  photos 4 wide to 5 high, the same ActionRow (their stars and emoji) and CaptionText (with the
  posted time, "Song · Sunday, October 18 · 8:45 PM"), a hairline and the comments, no count line.
  `CommentsRule` is removed.
- Photos in comments and GIFs are not Phase 5: see `06-FOLLOW-OVERRIDES.md` section 10.
