# Phase 5: Friend Feed (friends, the Board, reactions, comments, notifications) - Context

**Gathered:** 2026-10-10 (answers 5 to 8 of the developer, their pictures 08 to 11, and their notification text, the same day)
**Written:** 2026-10-10, on paper, against the Phase 2 documents (revision 5), the Phase 3 documents and the Phase 4 documents. Phases 2, 3 and 4 are NOT built. Every file, table and function named here from those phases is that PLAN's name. At build time the built code wins; each plan's start_rules say how to re-check.
**Status:** Ready for the developer to read. NOTHING IS BUILT. PHASE 5.
**Approval word for a Phase 5 plan:** "go", said separately, after Phase 4 is built, walked on the phone and closed. Phases 2, 3 and 4 each have their own "go".

Wording: folder and requirement ids keep the old names (ACCT-02, FEED-01 to FEED-04, plus the new FEED-05 and NOTIF-01 in `05-HAND-EDITS.md`). Anything the user sees says "spin", never "roll", even where the developer's pictures say "Roll" or "your roll" (see the differences list at the end of the UI spec).

Pictures (the layout source; saved in `docs/reference/`):
- `screens-07-diary-08-board-locked-2026-10-10.png`: screen 08, Board locked until you post.
- `screens-09-board-unlocked-10-post-2026-10-10.png`: screen 09, Board unlocked; screen 10, a post with reactions and comments.
- `screens-11-notifications-12-profile-2026-10-10.png`: screen 11, Notifications (the bell); screen 12, Profile (Phase 4).
- `screens-03a-home-03b-diary-2026-10-10.png`: 3a Home with the bell.
Their note: "Unread: a small black dot on the bell, as shown on screen 04" (screen 04, Home with the dot, is not yet saved; the dot is drawn on the bell of 08 and 09).
Read them before the UI spec.

**Visual reference (binding, developer 2026-10-10: "use exactly the same visuals ... as a reference to build this project"):** the developer's pictures are the visual reference for this phase. Build to match them: what is on each screen, its order, spacing, type, shapes (the pills and circles included) and states. At every screen's device check the phone is compared with its picture side by side (a simulator screenshot opened next to the picture); any difference that is not in the UI spec's "Choices made here that you may want to change" list is a defect to fix, not a choice. Where this paper and a picture disagree, the picture wins and the paper is corrected.

<domain>
## Phase Boundary

People become friends by exact username, and after posting their own spin they see what their friends spun and rated that day on the Board. They can react with one tap and comment on a friend's post, and they are told in a list behind the bell when something happens.

What the user gets:
- **Friends screen:** find a person by exact username, send a request, accept or decline, cancel, remove, block; share an invite text.
- **Board tab (Friends view):** the locked state until you have posted, the unlocked two-column grid, earlier days you posted, the no-friends state.
- **Post page:** a friend's photo, title, stars, their own reaction, their note, a row of six reactions, comments, an add-a-comment bar. The same page shows your own post with the reactions it received and its comments (and Edit and Delete from Phase 3).
- **Notifications:** the bell shows a black dot when something is unread, on Home and on Board. The Notifications page (a stub in Phase 2) lists reactions, comments, friend requests, accepted requests, "Your spin is open." and "Bonus spin added.".
- **Bottom bar:** Home, Diary, Spin, Board, Profile (five).
- **Profile:** the Friends number is real and opens the Friends screen.

Out of scope:
- Public and private accounts, the Global view, reporting a post, the person page for someone you are not friends with (Phase 6).
- The 8 PM phone reminder and any phone alert (Phase 6 and the launch list): they need a new native module, a rebuild, and for real alerts the developer's Apple push setup.
- Groups ("Leo joined your group." in the picture): the Phase 5 wording is "accepted your request". Groups are SOCIAL-01 (v2).
- Search by name or email, friends-of-friends, invite links (they need the domain), comment replies, editing a comment, reacting to a comment.
- Friend-only photo links on real storage (needs the real storage account, the launch list). Phase 5 builds the check and the single link function so that swap is one change.

Phase 5 is BUILT only after Phase 4 is built and closed, and after these documents are in the main tree with the edits in `05-HAND-EDITS.md`. It needs Phase 3 (entries, posted days, the entry page) and Phase 4 (the Diary list for the no-friends state, the Profile page, the bar).

</domain>

<decisions>
## Implementation Decisions (answered 2026-10-10)

### What the developer decided
- **D-01 After a skip (question 5, option A):** no friends' posts that day. Only a posted day opens the feed. A person who skipped sees "You skipped today. Friends' posts open after your next post." One rule for everyone.
- **D-02 Which days (question 6, option B):** earlier days stay readable too, newest first, for any day the person posted. A day with no post of yours is not listed. (More to build than "today only": the feed is paged by day.)
- **D-03 No glass (question 7, "forget it"):** the Board uses the same plain light cards as the rest of the app (pictures 08 to 10 are plain). The frosted-glass decision of 2026-10-01 is dropped. No blur, no `expo-glass-effect`, no native rebuild for the look. (Claude's reading of "forget it"; the pictures confirm it.)
- **D-04 Recent activity (question 8):** not a card on Today. The person's own recent posts are on the Profile page (Phase 4 D-04). Phase 5 changes nothing on Today except the dot on the bell.
- **D-05 The Board follows pictures 08, 09 and 10.** Header "TODAY / Board" with the bell; a two-column grid of photo cards; the locked card; the post page.
- **D-06 Notifications (their note of 2026-10-10):** a small black dot on the bell when unread (Home and Board). The bell opens Notifications (screen 11) as a page on top of the screen you were on, with a back arrow. Notifications are not a tab, so the bar stays at five. Tapping a notification: a reaction or comment opens your own post with its comments (screen 10, but for your post); "Your spin is open." goes to Home; "Bonus spin added." goes to Home.
- **D-07 Friend comments (decided earlier the same day, tap "Yes, add friend comments"):** friends can write short comments under a post. The comment's writer AND the post's owner can delete a comment.

### Carried over (restated so nothing is asked twice)
- **D-08 Friends** are found by exact username only, never by email. A request is sent, the other person accepts or declines, and nothing is shared until they accept. A decline is silent. A request shows as pending to the sender, who can cancel it. Remove friend and Block are silent. Block hides each of you from the other and stops new requests. One accepted friend is enough.
- **D-09 The gate** reads the posted days, never the entries: a posted day for the date opens that date's friends' posts, on time or late, even if the post was deleted later (Phase 3 D-03). A spin alone does not open it. A skip does not (D-01).
- **D-10 The late badge:** a post made in the late window shows the word "Late" on its card and page.
- **D-11 One reaction per person per post.** Tap another to change, tap the same to clear. A person cannot react to their own post (their own reaction is the author's reaction of Phase 3). The same list of names serves the author and the friends (Phase 3 D-02).
- **D-12 A post belongs to the day of the person who posted it**, with no time zone conversion between friends: the day is the stored `local_date` string.
- **D-13 No friends at all:** the Board shows an "Add friends" prompt above the person's own diary (FEED-03).
- **D-14 Every post is friends-only in Phase 5** and photos are shown only to accepted friends and the owner. (Phase 6 adds public accounts.) There is no per-post audience switch.
- **D-15 The bar:** Home, Diary, Spin, Board, Profile, with Spin in the middle. Board uses Ionicons `people-outline`.

### Claude's Discretion (safe defaults, told to the developer; change any by saying so)
Reactions:
- **The six, from the picture's reaction row (screen 10):** love, fire, laughing, wow, sad, sleepy, stored by name. This DIFFERS from the six the developer tapped for Phase 3 on 2026-10-09 and 2026-10-10 (heart, laughing, wow, fire, clap, yum): the picture has a crying face and a sleeping face where the tapped list had clap and yum, and a heart-eyes face for heart. The picture wins by the rule of 2026-10-10, and this is the first item the developer is told. It touches Phase 3's paper (its names list, its reaction file and its database check): `05-HAND-EDITS.md` item 1 lists the exact change, and one word ("six") puts the tapped list back. The characters are in one app file, the same one Phase 3 names.
- On your own post the row of six is replaced by what you received: the reactions as small characters with the names (a name, then the reaction character, for example Maya and the fire character, Priya and the crying-face character), or the plain line "No reactions yet." Nothing is counted at zero. On a friend's post the row shows only your own choice ringed, with no counts (as the picture).

Comments:
- 1 to 140 characters, trimmed, no control characters except line breaks (kept as spaces in the list). One level, no replies. Chronological, oldest first, as the picture. The label "{n} comments" ("1 comment") sits at the right of the reaction row.
- Allowed for the owner and for the owner's accepted friends who have opened the day (the gate applies). Delete: the writer or the owner, with a plain sheet "Delete this comment?". No edit.
- A comment on a post that is later deleted disappears with it.

Board:
- **Day in play ("today" on the Board):** the date of the most recent 8:00 PM local that has passed. So from 8 PM until the next 8 PM the Board's "today" is one date, and a post made after midnight in the late window belongs to the evening before (Phase 3 D-10). The server works it out; the phone only draws it. The overline reads "Today" in that window.
- **Order:** inside a day, newest first. Your own post is one of the cards (shown as "You"), not pinned. Earlier days follow, each under a small day label ("Yesterday", "Tue, Oct 6").
- **A card** is as in picture 09: the photo with the author's name chip at its top left and the category word in small capitals at its bottom left; under it the title and the stars with the number. The card is one button that opens the post page.
- **Locked card** (picture 08): the small label "Locked", the line "{n} friends posted today." ("1 friend posted today."), the line "Post your spin to see what they got and how they rated it.", and a black button. The picture's words "your roll" and "Log your meal" become "spin" and "Log your {category}" with the category as the person wrote it, lower case. Behind the card: up to four grey tiles with the name chips of friends who posted, no title, no stars.
- **The button depends on where the person is:** a spin exists and can be posted: "Log your {category}" opens Phase 3's entry screen; the window is open and no spin yet: "Spin first" goes to Home; nothing can be posted now: no button, the line "Your next spin opens at 8:00 PM." The server says which (`locked.action` is `log`, `spin` or `wait`).
- **No friend has posted yet:** the same card with the heading "No friends have posted yet." and the same line and button.
- **Skipped:** a plain centred block with the D-01 sentence and a "Go to Today" text link. No grid.
- **Loading, offline, empty** use Phase 2's `StatusMessage`. Pull down refreshes. The Board asks again on focus.
- **Paging:** five days at a time, as you scroll.
- The Board has no switch yet (Phase 6 adds "Friends | Global" under the title). The grid is the Friends view.

Friends screen (not in the pictures; plain, built from existing parts):
- Opened from the Friends number on Profile, from the "Add friends" button, and from a friend-request notification. Route inside the Profile stack.
- Top to bottom: a username box with a "Find" button ("Find a friend by username"); the result row (name, @username, one button: "Add friend", "Requested" (disabled), "Accept", or "Friends"); "Requests" (incoming: Accept and Decline); "Sent" (outgoing: Cancel); "Friends" (rows: tap opens an action sheet with "Remove friend" and "Block {name}"); a "Share an invite" text link that opens the share sheet with "Add me on RNDMRoll: @{username}" (no link until the domain exists).
- Lookup answers "No one has that username." for a miss and for someone who blocked you (the two look the same). Repeated lookups are rate limited.

Notifications (derived list, no stored events, no jobs):
- The list is built when asked from existing data; a small per-person "seen" time makes older items read. There is no per-item read state.
- Kinds: a friend reacted to your post; a friend commented on your post; a friend sent you a request; a friend accepted your request; "Your spin is open." (only for the date in play, from 8:00 PM until midnight); "Bonus spin added." (when the weekly goal paid a bonus spin, at the time it was earned).
- Sections as in the picture: Today, This week (the last seven days), and a third "Earlier" (up to 30 days; the picture shows only two). At most 50 items.
- Copy: "{Name} reacted {character} to your {category}." / "{Name} commented on your {category}: "{text}"" (text cut at about 60 characters) / "{Name} sent you a friend request." / "{Name} accepted your request." / "Your spin is open." / "You hit {goal} this week. One bonus spin added." (the picture says "last week"; the bonus is paid the moment the goal is hit, Phase 2 D-02, so "this week").
- Unread: an item newer than the person's seen time has a black dot at the right, as in the picture. Opening the page marks everything seen (the dots the page loaded with stay until you leave). The bell's dot is on while anything is newer than the seen time.
- Tapping: reaction or comment opens the person's own post (the post page, with its comments); a friend request opens the Friends screen; "accepted" opens the Friends screen; the two spin items go to Home.
- The page opens above whichever tab the person was on and its back arrow returns there. The route is hidden from the bar (not a tab). The Phase 2 route and component names are reused and the page is no longer a stub.

Blocking:
- Block is in Phase 5 (a friend action). It removes the friendship and any request between the two, hides each from the other on every list, and stops lookups from finding the blocker. Unblock is in the Friends screen's "Blocked" list at its end ("Unblock"). The blocked person is never told.

Profile:
- The Friends number is the real count and a button to the Friends screen. A small dot is not added to it (the bell covers requests).

</decisions>

<api_contract>
## API contract (binding for plans 05-01 to 05-06)

Instants are RFC 3339 UTC, names snake_case, dates `YYYY-MM-DD`. All routes are on the signed-in group (`RequireAuth`, `RequireUser`), call `u.OnboardingComplete()` first, take the user only from the token, validate by hand, and are rate limited with `KeyBySubject` (per-minute limits in the table). The app's client sends only GET, POST and PATCH. Ids in paths are UUIDs parsed in Go.

`Person`: `{"id","name","username","photo_url":string|null}`. `Entry` is Phase 3's. `reaction` is a name from the list shared with Phase 3 (the picture's six: `love`, `fire`, `laughing`, `wow`, `sad`, `sleepy`; the server and the database hold no emoji).

`Card`: `{"entry_id","author":Person,"mine":bool,"local_date","category":{"id","name"},"title","stars":1..10,"reaction":name|null,"photo_url","late":bool,"posted_at","posted_local_time":"8:41 PM","comment_count":int,"my_reaction":name|null}`. `reaction` is the author's own; `my_reaction` is the caller's. `posted_local_time` is the time of day in the AUTHOR's own time zone, written by the server (the phone cannot know another person's zone and never does date or time maths). Phase 5 always sends a number for `comment_count` and a list for `Post.comments`, but TYPE both as nullable from the start (`int | null`, `Comment[] | null`): Phase 6 sends null to a stranger who may not see comments, and this keeps that a data change, not a type change.
`Comment`: `{"id","author":Person,"text","at","at_label":"8:52 PM","can_delete":bool}`. `at_label` is written by the server in the VIEWER's time zone with one rule shared with notification items: the time of day ("8:52 PM") for an instant on the viewer's current local day, the short weekday ("Mon") for the six days before, the month and day ("Oct 3") before that.
`Post`: `{"card":Card,"note":text|null,"comments":[Comment],"can_react":bool,"can_comment":bool,"received":[{"person":Person,"reaction":name,"at"}]|null}`. `received` is set only when `mine` is true. `note` is the author's own comment field.

| Route | Request | Success | Limit/min |
|---|---|---|---|
| `GET /v1/people/lookup?username=` | exact, case-insensitive | 200 `{"person":Person,"relation":"none"|"friends"|"request_sent"|"request_received"}`; 404 `not_found` for a miss, for yourself, and for someone who blocked you | 30 |
| `GET /v1/friends` | none | 200 `{"friends":[Person],"incoming":[Req],"outgoing":[Req],"blocked":[Person]}`, `Req` = `{"request_id","person":Person,"at"}` | 60 |
| `POST /v1/friends/requests` | `{"username"}` | 200 `{"relation":"request_sent"|"friends"}` (if they already asked you, it accepts) | 10 |
| `POST /v1/friends/requests/{id}/accept` | `{}` | 200 `{"relation":"friends"}` | 20 |
| `POST /v1/friends/requests/{id}/decline` | `{}` | 200 `{}` | 20 |
| `POST /v1/friends/requests/{id}/cancel` | `{}` | 200 `{}` | 20 |
| `POST /v1/friends/{user_id}/remove` | `{}` | 200 `{}` | 20 |
| `POST /v1/blocks` | `{"user_id"}` | 200 `{}` (also removes the friendship and requests) | 20 |
| `POST /v1/blocks/{user_id}/delete` | `{}` | 200 `{}` | 20 |
| `GET /v1/board` | `before` (date, optional), `limit` days (1 to 10, default 5) | 200 `Board` (below) | 60 |
| `GET /v1/posts/{entry_id}` | none | 200 `Post`; 404 `not_found` if not visible to the caller (never 403, so ids cannot be probed) | 60 |
| `POST /v1/posts/{entry_id}/reaction` | `{"reaction":name|null}` | 200 `{"my_reaction":name|null}` | 60 |
| `POST /v1/posts/{entry_id}/comments` | `{"text"}` | 201 `{"comment":Comment}` | 20 |
| `POST /v1/comments/{id}/delete` | `{}` | 200 `{}` | 20 |
| `GET /v1/notifications` | none | 200 `{"unread":bool,"items":[Item]}` | 60 |
| `GET /v1/notifications/summary` | none | 200 `{"unread":bool}` | 120 |
| `POST /v1/notifications/seen` | `{}` | 200 `{"unread":false}` | 30 |

`Board`:
`{"state":"no_friends"|"locked"|"skipped"|"unlocked","day_label":"today"|"last_night","locked":{"friends_posted":int,"names":[string],"category":string|null,"action":"log"|"spin"|"wait"}|null,"days":[{"local_date","label":"today"|"yesterday"|"earlier","cards":[Card]}],"next_before":date|null}`.
- `no_friends`: `days` is empty; the app shows the prompt and the person's diary (Phase 4's `GET /v1/diary`).
- `locked`: `locked` is set (`names` is up to four first names of friends who posted the day in play, `category` the shown spin's name as written, or null). `days` lists the EARLIER days (before the day in play) that the caller posted, newest first, and is empty when there are none: the gate is per day, so a day you posted stays readable under today's lock (answer 6, option B).
- `skipped`: the same: `days` lists earlier posted days, newest first.
- `unlocked`: `days` lists the day in play first (if the caller posted it), then earlier days the caller posted, newest first. A day with no visible cards is left out. Each day's cards include the caller's own entry.
`Item`: `{"id":string,"kind":"reaction"|"comment"|"friend_request"|"friend_accepted"|"spin_open"|"bonus_spin","at","at_label":"9:18 PM","section":"today"|"week"|"earlier","unread":bool,"actor":Person|null,"entry_id":uuid|null,"category_name":string|null,"reaction":name|null,"text":string|null,"request_id":uuid|null}`. The server fills `section`, `unread` and `at_label` (the same rule as a comment's); the phone never works out a date.

Errors (new codes join `ApiErrorCode` and the app's message list): `already_friends` 409, `request_exists` 409, `own_post` 409 (a reaction on your own post), `request_limit` 409 (50 requests of yours are already waiting; copy: "You can't send more requests right now. Cancel one first."), `not_friends` 403 is NOT used (visibility failures are 404), `comment_too_long` is a `validation_failed` field error (`text`: `required`, `too_long`, `invalid`), `blocked` is never exposed (a blocked request answers `not_found`).

The gate, in one rule: a viewer sees an entry iff the viewer is its owner, OR (the viewer and the owner are accepted friends, nobody blocked anybody, AND the viewer has a posted day for the entry's `local_date`).

</api_contract>

<canonical_refs>
## Canonical References

Read before planning or building. Paths are inside the project unless stated.

### Decisions and pictures
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md` (outside the project, read only): the answers and the new scope.
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/questions/PHASE5-QUESTIONS.md`: the older defaults. This document supersedes it where they differ (no glass, comments exist, the Today card is gone, the bar has five).
- `docs/reference/screens-*.png` (listed at the top); `docs/design-brief-2026-10-01-feed.md` is superseded by D-03.
- `.planning/STATE.md` (friend comments and notifications entries) and `.planning/phases/05-friend-feed/05-HAND-EDITS.md`.

### Project rules
- `.planning/PROJECT.md` (standing design rules, launch gate), `.planning/REQUIREMENTS.md` (ACCT-02, FEED-01 to FEED-04), `.planning/ROADMAP.md` (Phase 5).
- Phase 2 documents: the clock, `spins`, `posted_days`, `goal_weeks`, the Today response, the bell and the Notifications stub (section 4.5), `ConfirmSheet`, `StatusMessage`.
- Phase 3 documents: `entries`, the photo store (`EntryPhotoStore.PublicURL`), `EntryView`, the reaction file, the delete rule.
- Phase 4 documents: the diary list component and endpoint, the Profile page, the bar change.

### Code to read before planning or building
- Server: `internal/spin/` (clock, posted days, week), `internal/httpapi/entry.go`, `server.go`, `errors.go`, `internal/storage/entry_photos.go`, `internal/store/postgres/*_repo.go`, `internal/middleware/ratelimit.go`, `cmd/api/main.go`.
- App: `app/(app)/_layout.tsx`, `app/(app)/today/notifications.tsx`, `components/today/NotificationsScreen.tsx`, `components/today/*` (the bell), `components/entry/*`, `lib/entry/reactions.ts`, `lib/api/*`, `lib/theme/tokens.ts`, `components/ui/*`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets (after Phases 2 to 4 are built)
- **The bell and the page:** Phase 2 draws the bell in Today's header and a Notifications page that says "Nothing new yet." (route `app/(app)/today/notifications.tsx`, component `NotificationsScreen`). Phase 5 fills the page and adds the dot and a second bell on Board.
- **Entry page and flows:** `EntryView`, `StarDisplay`, `lib/entry/reactions.ts`, `ConfirmSheet`, `ImageReveal`, `PressScale`, `FadeInUp`.
- **Lists:** Phase 4's diary row and list component for the no-friends state.
- **Clock and gate data:** the per-account clock, `posted_days`, the spin result, `goal_weeks`.
- **Test tools:** Phase 2's secret test setting (clock, pretend posted days, reset). Phase 5 adds friend fixtures to the test reset so two or three test accounts can be walked.
- **Share:** the React Native `Share` API is built in (no native module).

### Gaps to close (nothing of these exists)
- Tables for friend requests, friendships, blocks, reactions, comments and the notification seen time; the visibility function and the single photo-link function; the Board, post, reaction, comment, friends and notification endpoints.
- The Friends screen, the Board (locked, unlocked, no friends, skipped, paging), the post page, the reaction row, the comment list and bar, the notification list, the bell dot, the Board tab and its bell, the Friends number as a button, the hidden notifications route above the tabs.

### Integration points
- **Routes:** `internal/httpapi/server.go` is the only engine builder; handlers are fields on `Deps` registered only when not nil; `cmd/api/main.go` and both test builders change together in one task.
- **Data:** migration 0006 (Phase 1's clean-up is 0003, Phase 2's 0004, Phase 3's 0005; re-check with `ls migrations`), hand numbered. Every table references `users(id) on delete cascade` so the `truncate users cascade` cleanups work; nothing is added to `users`. Reactions and comments reference `entries(id) on delete cascade` so Phase 3's delete removes them.
- **Package layout (suggested):** `internal/social` (requests, friendships, blocks, the visibility rule), `internal/board` (board, posts, reactions, comments, photo link function), `internal/notifications` (derived list and seen time). One repository interface each, a MemRepo and a Postgres run of one shared suite, as Phase 2 did.
- **Launch files:** `app/(app)/_layout.tsx` (Tabs: Board added, a hidden notifications route) is a launch file, so one cold launch is checked after it changes. `app/index.tsx`, `lib/session/store.ts`, `lib/api/client.ts` are not edited (an edit to `lib/api/types.ts` can reload the dev client to Welcome: make it when the developer is not mid-test).
- **Test traps:** never delete or edit the four real dev accounts in `rndmroll_dev`; two or three test accounts are needed for a friend walk, made through the API with throwaway emails and deleted by exact email afterwards; Go tests run only on a database whose name ends in `test`, with `-p 1`.
- **Limits:** no new native module in this phase (the Phase 6 reminder is the one rebuild); never run `expo lint`; local commits, one per task, explicit paths, never pushed.

</code_context>

<specifics>
## Specific Ideas

- Name chip: a small round initial and the first name, on a light fill, at the top left of the photo (as the picture). It is a drawn label, not a control.
- **Two pages, not one (corrected 2026-10-10 after the developer's picture 06 was read).** A FRIEND's post page follows picture 10: a photo about 3:2 wide, the author row on top, the reaction row, the comments, the add-a-comment bar. YOUR OWN entry page stays Phase 3's page, which follows the developer's picture 06 (the photo square and edge to edge, the Back label naming where you came from, the fixed Edit and Delete footer). The developer's note says a reaction or comment opens "your entry with the comments (like screen 10, but for your post)": so Phase 5 ADDS to Phase 3's own entry page, below the author's note and above the footer, the "received" line and the "{n} comments" list with its delete buttons and the add-a-comment bar (the owner may reply). Both pages share the small parts (reaction row, comment list, comment bar), not the layout. The own page keeps its square photo.
- A friend's name on the post page's top row opens nothing in Phase 5 (no person page until Phase 6).
- Stale, ignore: "Leo joined your group.", "Post your roll", "Log your meal" as fixed strings, and "last week" in the bonus line, in the pictures.

</specifics>

<deferred>
## Deferred Ideas

- Public accounts, Global, report, person page, the 8 PM phone reminder (Phase 6).
- Phone alerts for reactions, comments and requests (needs the Apple push setup on the launch list).
- Friend-only photo links on real storage (launch list); the link function is ready for it.
- Groups, discovery, replies, editing comments, reacting to comments, muting a friend.
- A count of reactions on a friend's post, grouping of notifications ("Maya and 2 others").
- Marking single notifications read; deleting a notification.

</deferred>

<hand_edits>
## Hand edits at merge-back

The exact find-this and replace-with text is in `05-HAND-EDITS.md`. In short: the Phase 3 paper's reaction list (item 1, only if the developer keeps the picture's six); REQUIREMENTS ACCT-02 and FEED-01 to FEED-03 wording, new FEED-05 (comments) and NOTIF-01 (notifications); the ROADMAP Phase 5 goal, criteria and plan list; one PROJECT.md Key Decisions row and the moodboard bar line; the Phase 2 UI spec notes (the bell's dot, the bar of five, the Notifications page no longer a stub); the Phase 4 Friends number note. Apply them in the main tree by hand, after Phase 4 is closed, before Phase 5 is built.

</hand_edits>

---

*Phase: 05-friend-feed*
*Context gathered: 2026-10-10*
