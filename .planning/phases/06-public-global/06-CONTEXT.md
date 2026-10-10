# Phase 6: Public Accounts, Global and the 8 PM reminder - Context

**Gathered:** 2026-10-10 (the developer's voice note about the Board, "friends or global", public and private accounts; their notification note, "the 8 PM reminder should also go out as a phone notification")
**Written:** 2026-10-10, on paper, against Phase 2 to Phase 5 documents. NONE of those phases is built. Every file, table and function named here from them is that PLAN's name. At build time the built code wins; each plan's start_rules say how to re-check.
**Status:** Ready for the developer to read. NOTHING IS BUILT. PHASE 6. **There are NO pictures for this phase yet** (the Global view, the privacy switch, the person page, the report sheet, the reminder sheet). Everything on screen here is a plain default built from the Phase 5 picture parts; send pictures and the UI spec is redrawn from them.
**Approval word for a Phase 6 plan:** "go", said separately, after Phase 5 is built, walked on the phone and closed.

Wording: anything the user sees says "spin", never "roll". New requirement ids (ACCT-04, FEED-06, MOD-01, NOTIF-02) are in `06-HAND-EDITS.md`.

Why this is its own phase: Global puts strangers' photos in front of each other. That is a different product from the friend circle: it needs reporting and blocking before it is used by anyone outside the test group, Terms and Privacy text, and a person who reads the reports. Keeping it out of Phase 5 lets the friend-group app ship and be used without it. The phone reminder is also here because it is the one change that needs a new native module and one rebuild of the test app.

**Visual reference (binding, developer 2026-10-10: "use exactly the same visuals ... as a reference to build this project"):** the developer's pictures are the visual reference for this phase. Build to match them: what is on each screen, its order, spacing, type, shapes (the pills and circles included) and states. At every screen's device check the phone is compared with its picture side by side (a simulator screenshot opened next to the picture); any difference that is not in the UI spec's "Choices made here that you may want to change" list is a defect to fix, not a choice. Where this paper and a picture disagree, the picture wins and the paper is corrected.

Phase 6 has no pictures of its own yet: it reuses the parts of pictures 08 to 10 (the switch, the card, the post page). When the developer sends pictures for the Global view, the privacy switch, the person page, the report sheet and the reminder sheet, they become the reference and the UI spec is redrawn from them.

<domain>
## Phase Boundary

Each account is Public or Private. A Public account's posts also show on Global, a second view on the Board next to Friends, to every signed-in person, whatever their own setting. A Private account's posts are seen only by accepted friends (Phase 5). A person can open another person (a public one, or someone they have a request with) on a small person page, add them as a friend, block them, and report a post. At 8:00 PM the phone itself reminds the person that their spin is open.

What the user gets:
- **Account setting:** "Public account" switch on the Edit profile page (default off). Turning it on asks first, because past posts become visible.
- **Board:** a "Friends | Global" switch under the title (the picture of screen 08 already draws it). Global is a two-column grid of public posts, newest first, open at any time.
- **Post page:** someone else's post, exactly like a friend's: the six reactions and the comments (anyone who can see a post can react and comment); a quiet "Report this post" and "Block {name}"; a quiet "Report" on a comment that is not yours.
- **Person page:** photo, name, @username, one relation button, their public posts.
- **8 PM reminder:** a phone notification "Your spin is open." at 8:00 PM local time, a switch for it on the Edit profile page, asked once after the first post.

Out of scope:
- Following (a one-way relation) a public person; ranking or recommendations; hashtags or search by name; direct messages.
- Reporting a person (a post and a comment can be reported; blocking covers a person); an owner switch to turn comments off; filtering comments by words.
- A moderation tool, automatic hiding after many reports, an appeals flow. Reports go into a table the developer reads (launch list).
- Phone alerts sent by the server (a friend reacted, commented, asked): they need the developer's Apple push setup (launch list). The reminder here is a LOCAL notification and needs none.
- A per-post audience switch ("post this one publicly"): the account setting decides. If a picture later shows a switch on the entry screen it changes the Phase 3 entry screen and its paper.

Phase 6 is BUILT only after Phase 5 is built and closed.

</domain>

<decisions>
## Implementation Decisions

### What the developer decided (their words, 2026-10-10, cleaned)
- **D-01** The Board has the posts of your friends, or Global. Any person's account is private or public. If a person has a public account their posts show on the global level, and a person with a PRIVATE account can still look at Global, because Global is made of public accounts.
- **D-02** The 8 PM reminder also goes out as a phone notification, so people get it without opening the app. (Their note; it reverses Phase 2 D-14, which left it out of the first version.)
- **D-03** (their words, 2026-10-10, voice): Global is "very similar to how it is for friends": you can react and comment and everything. The difference is the account, not the screen: a private account's posts reach its friends, a public account's posts reach everyone, including people whose own account is private ("like how Instagram will let you look at all these reels or posts").

### Claude's Discretion (safe defaults, told to the developer; every one is a paper edit to change)
Accounts and safety:
- **One setting per account, Private by default.** Every existing account is Private. There is no per-post switch.
- **Under 18 is always Private.** The birthday is already collected (13 or older). The switch is off and disabled for an account under 18, with the line "Public accounts are for people 18 and over." The server refuses a change (`under_age_for_public`). Reason: a global feed of photos from 13 to 17 year olds is a safety and store-rules risk that a small friend group does not have. Say if you want it different.
- **Turning on asks first** with a sheet: "Make your account public?" / "Anyone on RNDMRoll will see your posts on Global, including the ones you have already posted." / "Make public" and "Keep private". Turning off needs no sheet; the posts leave Global at once and friends stay friends.
- **Private posts stay gated.** The post-first rule of Phase 5 applies to Friends only. Global is open at any time, even to a person who has not posted. A friend who is Public is therefore visible on Global without the gate; the gate protects private posts only. This is stated plainly in the UI spec's choices list.
- **Photos on Global are viewable by any signed-in person.** The single link function of Phase 5 (visibility check, then link) is extended; real friend-only and signed links still wait for the real storage account.

Global view:
- The same two-column card grid as the Friends view (Phase 5 D-05), no day sections: newest first by the time posted, 20 cards a page, endless scroll, pull to refresh. Your own posts appear if your account is Public. People you blocked, and people who blocked you, never appear.
- Reactions: allowed on any visible post (one per person, the six).
- Comments: exactly as on a friend's post (developer, 2026-10-10: Global is "very similar to how it is for friends ... able to comment and everything"). Anyone who can see a public post reads its comments and may write one (1 to 140 characters, as Phase 5). The post's owner can delete any comment on it and the writer can delete their own. A comment that is not yours and that you cannot delete shows a quiet "Report" where "Delete" would be. Not being a friend never hides the comments.
- A card's tap opens the post page. The name chip on a card and the author row on the post page open the person page.

Person page (`Person`: name, @username, photo):
- A header, one relation button ("Add friend", "Requested", "Accept", "Friends"), a quiet "Block {name}", and, when the person's account is Public, their posts as the same grid, newest first, 20 a page. A Private person who is a friend shows no grid here (their posts are on the Board, behind the gate). A Private non-friend is not reachable at all: 404.
- Opened from Global cards and the post page. Phase 5's Friends screen keeps its plain rows.

Report:
- On the post page of someone else's post, a quiet text link "Report this post" under the comments (or under the reactions when there are none). It opens a sheet with four rows: "It's spam", "It's unkind or harassing", "It shouldn't be on a public feed", "Something else", and "Cancel". No free text. After sending the line "Thanks. We'll look at it." shows and the post disappears from the reporter's lists. Reports are stored (reporter, post, reason, time) and nothing happens automatically.
- On a comment that is not yours (and that you cannot delete) a quiet text link "Report" sits where "Delete" would be. It opens a sheet with "It's spam", "It's unkind or harassing", "Something else", "Block {name}" and "Cancel". A comment report is stored like a post report and the comment leaves the reporter's list at once.
- The owner of a post has no Report on a comment (they can delete it), so the delete sheet of a comment that is not theirs gets a second row, "Delete and block {name}": it deletes the comment and blocks its writer (Phase 5's block action) in one go, after the same one tap. Plain "Delete" and "Keep it" stay.
- Blocking is Phase 5's action, offered again here from the post page, the person page and the comment sheet.
- Launch list additions: who reads the reports and how fast; Terms that say what is allowed and what is done with reports; a contact address (App Store rule for apps with user posts).

Reminder:
- A LOCAL notification repeating every day at 20:00 in the phone's own time zone (`expo-notifications`, a calendar trigger). The server is not involved, so it needs no Apple push key. It needs the native module, so one dev-client rebuild (`npx expo prebuild --clean`, which also drops the two native modules Phase 1's Apple and Google sign-in left in `ios/`). No other phase needs a rebuild.
- Text: title "Your spin is open." body "Spin today's wheel." Tapping it opens the app on Home.
- Asked once, after the person's first posted day: a sheet "Get a reminder at 8:00 PM?" / "We'll tell you when your spin opens." / "Remind me" and "Not now". "Remind me" shows the system permission prompt, then schedules. "Not now" is remembered (never asked again; the switch is the way back).
- The switch "Remind me at 8:00 PM" lives on the Edit profile page under the account switch. The value is kept on the phone (`expo-secure-store`, already installed). On every app start the schedule is rebuilt (cancel all, schedule one) so it can never double. If the person turns the system permission off, the switch shows off with the line "Turn on notifications for RNDMRoll in Settings." Nothing else.
- A reminder still fires on a day the person already spun. That is accepted for the first version.

</decisions>

<api_contract>
## API contract (binding for plans 06-01, 06-02 and 06-03)

Same conventions as Phase 5 (snake_case, RFC 3339 UTC, signed-in group, `OnboardingComplete` first, hand validation, `KeyBySubject` limits, GET/POST/PATCH only). `Person`, `Card`, `Post`, `Comment` are Phase 5's, with NO change of shape: comments follow the post's visibility, so `comment_count`, `Post.comments` and `Post.can_comment` are sent exactly as Phase 5 sends them (never null in Phase 6) to everyone who can see the post, friend or stranger. (Phase 5 typed them nullable only as a precaution; Phase 6 does not use the null.)

| Route | Request | Success | Limit/min |
|---|---|---|---|
| `GET /v1/account/visibility` | none | 200 `{"visibility":"private"|"public","can_be_public":bool}` | 60 |
| `PATCH /v1/account/visibility` | `{"visibility":"public"|"private"}` | 200 same shape; 403 `under_age_for_public` | 10 |
| `GET /v1/board?view=global` | `cursor` (opaque), `limit` 1 to 40, default 20 | 200 `{"state":"open","cards":[Card],"next_cursor":string|null}` | 60 |
| `GET /v1/people/{user_id}` | `cursor`, `limit` (the posts) | 200 `{"person":Person,"relation":"none"|"friends"|"request_sent"|"request_received","visibility":"public"|"private","posts":[Card]|null,"next_cursor":string|null}`; 404 `not_found` unless the person is Public, a friend, or has a request with the caller, and nobody blocked anybody | 60 |
| `POST /v1/posts/{entry_id}/report` | `{"reason":"spam"|"unkind"|"not_public"|"other"}` | 200 `{}` (a repeat from the same person for the same post is 200 and stores nothing new) | 10 |
| `POST /v1/comments/{id}/report` | `{"reason":"spam"|"unkind"|"other"}` | 200 `{}` (a repeat is 200 and stores nothing new; 404 `not_found` if the comment is not visible to the caller; a person cannot report their own comment: 409 `own_comment`) | 10 |

`GET /v1/board` without `view` is Phase 5's Friends view, unchanged.

The visibility rule becomes: a viewer sees an entry iff the viewer is its owner, OR (the viewer and the owner are accepted friends, nobody blocked anybody, AND the viewer has a posted day for the entry's `local_date`), OR (the owner's account is Public AND nobody blocked anybody). Everything that returns a post, a card, a reaction, a comment, or a photo link goes through this one function: whoever can see an entry can read and write its comments (a blocked pair excepted), so a stranger may comment on a public post.

New error codes: `under_age_for_public` 403, `own_comment` 409. Everything else reuses Phase 5's.

Data (migration 0007, hand numbered, after `ls migrations`): `account_settings (user_id pk references users on delete cascade, visibility text not null default 'private' check in ('private','public'), updated_at)`; `post_reports (id, reporter_id references users on delete cascade, entry_id references entries on delete cascade, reason text check in the four, created_at, unique (reporter_id, entry_id))`; `comment_reports (id, reporter_id references users on delete cascade, comment_id references the comments table of Phase 5 on delete cascade, reason text check in the three, created_at, unique (reporter_id, comment_id))`; one index to read public entries newest first. Nothing is added to `users`. A missing `account_settings` row means Private.

</api_contract>

<canonical_refs>
## Canonical References

- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md` (outside the project, read only): the new scope and the defaults.
- `.planning/phases/05-friend-feed/` (all files): the Board, the cards, the post page, the visibility function, friends and blocks, notifications.
- `.planning/phases/04-personal-diary/`, `03-daily-entry/`, `02-daily-roll/`, `01-foundation-accounts/` (the Edit profile page, the birthday, the profile form).
- `.planning/PROJECT.md` (launch gate), `.planning/REQUIREMENTS.md` (Out of Scope: "Public launch / outside-user growth" needs a one-line revisit, see the hand edits).
- Expo notifications reference (read at build time): local scheduling with a calendar trigger, permission request, the response listener.

### Code to read before planning or building
- Server: the Phase 5 `internal/board` and `internal/social` packages, `internal/httpapi/*`, `internal/store/postgres`, `internal/user` (the birthday and age), the entry photo store.
- App: the Phase 5 Board and post page components, `components/profile/ProfileForm.tsx` (the Edit profile page), `app/(app)/_layout.tsx`, `lib/session/store.ts` (read only; the reminder value is NOT stored there), `package.json`, `app.json` (the notification config plugin; iOS has no usage-description string for notifications, the permission is the system prompt).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets (after Phases 2 to 5 are built)
- The Board grid, the card, the post page, the reaction row, `ConfirmSheet`, the Friends screen rows, `StatusMessage`, the visibility function and the single photo-link function.
- The birthday is on the account (Phase 1 migration 0002). Age is computed on the server from the birthday and the clock.
- `expo-secure-store` is installed. `expo-notifications` is NOT: this phase adds it.

### Gaps to close
- `account_settings` and `post_reports`; the four routes and the visibility change; the Global view; the person page; the report sheet; the Edit profile switches; the reminder permission sheet and scheduler; the native module and the one rebuild.

### Integration points and traps
- **Rebuild:** adding `expo-notifications` changes `package.json`, `package-lock.json` and `app.json`; the dev client must be rebuilt and reinstalled on the Simulator; never run `expo lint`; install with `npx expo install expo-notifications` only after the developer agrees to the rebuild. The app must still start if the module is missing (guard the import) so the Phase 5 build keeps working until the rebuild.
- **Launch files:** the Edit profile page and the Board header change; no launch file other than `app/(app)/_layout.tsx` if a hidden person route needs registering; `app/index.tsx` is not touched.
- **Safety:** under-18 accounts can never become Public: the check is on the server, not only in the app. A test fixture for an under-18 account is made through the API with a birthday that makes them 17.
- **Tests:** the shared Phase 5 suite gains public, private, blocked and under-age cases; never touch the four real dev accounts.

</code_context>

<specifics>
## Specific Ideas

- The "Friends | Global" switch is drawn as in picture 08: a full-width rounded track with the chosen side filled `ink` with `dominant` text.
- The Global empty state: one plain line, "No public posts yet.", no picture.
- The person page's relation button reuses the Friends screen's button states.

</specifics>

<deferred>
## Deferred Ideas

- Following, discovery, ranking, search by name.
- Reporting a person; an owner switch to turn comments off; filtering comments by words; a per-day cap on comments from one person (the 20 a minute limit of Phase 5 applies).
- Server-sent phone alerts (Apple push setup).
- Automatic hiding after reports, a moderation screen.
- A per-post public switch.
- A reminder that skips a day already spun; a choice of reminder time.

</deferred>

<hand_edits>
## Hand edits at merge-back

The exact text is in `06-HAND-EDITS.md`: new requirement lines ACCT-04, FEED-06, MOD-01, NOTIF-02; the REQUIREMENTS "Out of Scope" row revisit; the ROADMAP Phase 6 entry; one PROJECT.md Key Decisions row and the launch-gate additions (reports reader, Terms, contact address); the Phase 2 note that D-14 is reversed; the Phase 5 note that the Board gains the switch. Apply them by hand after Phase 5 is closed, before Phase 6 is built.

</hand_edits>

---

*Phase: 06-public-global*
*Context gathered: 2026-10-10*
