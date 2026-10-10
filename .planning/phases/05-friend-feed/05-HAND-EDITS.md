# Phase 5 hand edits (PHASE 5, NOTHING IS BUILT)

**NOT APPLIED.** Written 2026-10-10 by a background writer for the lead, and RE-VERIFIED the same evening at 21:50 IST after the other window's commits `b7f799a`, `0557d19` and `a491fc9` (Phase 2.1 Settings inserted, SET-01 to SET-04, Phase 3 revision 3, migrations shifted by one). Every Find string below was checked against the live file at 21:50 IST and occurs exactly once, unless the item says otherwise; none of the earlier Find strings moved. What did change: the Key Decisions table now ends with the Phase 2.1 row (A10), the coverage lines are stale and the totals are recomputed (A6, A11), and the plan list and the migration number follow the plan files (A8, D). The other window is still working in `.planning/`, so re-read each file just before editing.

These are the exact edits to SHARED documents that follow from the Phase 5 decisions (`05-CONTEXT.md`). The Phase 5 documents are complete without them, but the shared documents will disagree with Phase 5 until they are applied.

How to use:
- Apply by hand with explicit paths, AFTER the other window has committed its own edits to `.planning/` and after Phase 4 is closed (section A is required before Phase 5 is built; B, C and the note items change nothing that Phases 2 to 4 build, so they can go in earlier). Never apply them inside the other window's working session: a GSD commit stages all of `.planning/`.
- Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words. Where a find string quotes old text that holds the words "roll" or "rolled", that is a quote of old text, not new wording.
- Order across the three new hand-edit files: 04 first, then 05, then 06. Where a Find depends on 04 having been applied, the item says what it reads if 04 was not applied.
- Item 1 is about a decision the developer must be told. Section A is required. Section B is the glass decision. Section C is notes on Phase 2 and Phase 3 documents. Section D is a list of facts for STATE.md and HANDOFF.json, which belong to the other window. Section E lists what is already done.

## Item 1. The reaction list (Phase 3 paper and PROJECT.md)

**What happened.** The developer typed "2-A" on 2026-10-09 and again on 2026-10-10 at about 15:35 (heart, laughing, wow, fire, clap, yum). The same afternoon their picture (screen 10, the reaction row) showed love, fire, laughing, wow, sad, sleepy, in that order, and the entry page picture (06) shows the love face in the author's slot. By the rule of 2026-10-10 the picture wins over an older paper rule, so Phase 5 uses the picture's six (`05-CONTEXT.md`, Claude's Discretion). THE DEVELOPER MUST BE TOLD, once, in plain words; one word, "six", puts their tapped list back.

**Status checked 2026-10-10 16:16 IST and again 21:50 IST (after Phase 3 revision 3).** The other window has ALREADY switched the Phase 3 paper to the picture's six (between 16:12 and 16:13), with the stored names `love`, `fire`, `laughing`, `wow`, `sad`, `sleepy` in that order, and it keeps a sentence in each place saying the tapped six were replaced. So only the shared document below is left.

| Place | State | What to do |
|---|---|---|
| `03-CONTEXT.md` D-02 and the "stored by short name" bullet | already the picture's six | nothing |
| `03-UI-SPEC.md` section 6 table, the note under it, section 13 spoken names, section 14, section 16 row 10 and the choices list | already the picture's six | nothing |
| `03-01-PLAN.md` must-haves, binding decision 4, the `Reactions` constant, the `entries_reaction_check` constraint and the validation tests | already the picture's six (the older names are refused as `invalid`) | nothing |
| `03-02-PLAN.md` the reaction file and its check script | already the picture's six | nothing |
| `03-03-PLAN.md` strings step | already the picture's six | nothing |
| `03-VALIDATION.md`, `REQUIREMENTS.md` ENTRY-01 | name no reactions (they say "one of six") | nothing |
| `.planning/PROJECT.md` Key Decisions row '2026-10-09: Phase 3 entry rules.' (no longer the last row: the Phase 2.1 Settings row follows it) | STILL the tapped six | edit 1a below |
| `.planning/STATE.md` (the '[Phase 03]: Entry rules' line) and `.planning/phases/01-foundation-accounts/.continue-here.md` (the Phase 3 paragraph) | STILL the tapped six | the other window's files: see section D |
| `03-HAND-EDITS.md` A4 | the record of the text applied on 2026-10-10 | leave as it is |

**1a. `.planning/PROJECT.md`, the Phase 3 Key Decisions row.** Checked 21:50.

Find:
```
A reaction is one of six fixed emoji: heart, laughing, wow, fire, clap, yum.
```
Replace with:
```
A reaction is one of six fixed emoji: love, fire, laughing, wow, sad, sleepy (the six in the developer's picture of 2026-10-10; they had tapped heart, laughing, wow, fire, clap, yum, and the picture won).
```

**1b. If the developer says "six" (keeps the tapped list).** Reverse the table above: in every Phase 3 place named there put back the names heart, laughing, wow, fire, clap, yum in that order, with the characters U+1F60D, U+1F602, U+1F62E, U+1F525, U+1F44F, U+1F60B (the other window's Phase 3 paper uses the heart-eyes face for heart), the database check list, the Go list and the reaction file; in `05-CONTEXT.md` and the Phase 5 plans and UI spec replace the six names and characters the same way; and drop the replacement sentence added by 1a. It is one file and one list in the app and one list on the server.

## A. Shared project documents (required)

**A1. `.planning/REQUIREMENTS.md`, ACCT-02** (D-08). Checked 21:50.

Find:
```
- [ ] **ACCT-02**: User can add friends (e.g., via invite/username) to form their friend circle
```
Replace with:
```
- [ ] **ACCT-02**: User can find a person by exact username, send a friend request, and become friends when it is accepted; user can decline a request, cancel one they sent, remove a friend, and block a person
```

**A2. `.planning/REQUIREMENTS.md`, FEED-01** (answers 5 and 6). Checked 21:50.

Find:
```
- [ ] **FEED-01**: User can view a shared feed of what friends in their circle rolled and rated that day, gated behind having posted their own entry that day
```
Replace with:
```
- [ ] **FEED-01**: User can view a shared Board of what friends in their circle spun and rated in the current day (8:00 PM to the next 8:00 PM), and on earlier days they posted, newest first, gated behind having a posted day for that day (a later delete does not close it, and a skip does not open it)
```

**A3. `.planning/REQUIREMENTS.md`, FEED-03.** Checked 21:50.

Find:
```
- [ ] **FEED-03**: User with no friends yet (or below the gate threshold) is routed to their personal diary instead of a locked/empty feed
```
Replace with:
```
- [ ] **FEED-03**: User with no friends yet (or below the gate threshold) is routed to their personal diary instead of a locked/empty feed, with an Add friends prompt above it. With friends but no post yet, a plain locked card says how many friends posted and points to Today; after a skip, a plain line says friends' posts open after their next post
```

**A4. `.planning/REQUIREMENTS.md`, FEED-04 and the new FEED-05.** Checked 21:50. (FEED-02 is unchanged: the late badge is as it was.)

Find:
```
- [ ] **FEED-04**: User can react to a friend's feed entry with a one-tap emoji reaction (not photo-based)
```
Replace with:
```
- [ ] **FEED-04**: User can react to a friend's feed entry with a one-tap emoji reaction (one of six fixed reactions, one per person per post, tap again to clear; not photo-based)
- [ ] **FEED-05**: User can write short comments (up to 140 characters) under a friend's post and under their own post, and read the comments on them; the comment's writer and the post's owner can delete a comment
```

**A5. `.planning/REQUIREMENTS.md`, the new Notifications section.** Checked 21:50. The section goes after Friend Feed and before the v2 list.

Find:
```
## v2 Requirements
```
Replace with:
```
### Notifications

- [ ] **NOTIF-01**: User sees a small black dot on the bell when something is unread, and a Notifications page behind the bell that lists reactions and comments on their posts, friend requests and accepted requests, "Your spin is open." and "Bonus spin added."; tapping an item opens the post (their own, with its comments), the Friends screen or Home

## v2 Requirements
```

**A6. `.planning/REQUIREMENTS.md`, traceability and coverage.** Checked 21:50.

Find:
```
| FEED-04 | Phase 5 | Pending |
```
Replace with:
```
| FEED-04 | Phase 5 | Pending |
| FEED-05 | Phase 5 | Pending |
| NOTIF-01 | Phase 5 | Pending |
```

Find (this reads 22 if 04-HAND-EDITS A3 was applied: 21 live requirements plus DIARY-03. If 04 was not applied it reads 17, which is stale because the list holds 21 since the Settings requirements were added, and the new value is then 23):
```
- v1 requirements: 22 total
```
Replace with:
```
- v1 requirements: 24 total
```

Find (same rule: 17 becomes 23 if 04 was not applied):
```
- Mapped to phases: 22
```
Replace with:
```
- Mapped to phases: 24
```

**A7. `.planning/ROADMAP.md`, the phase list line for Phase 5.** Checked 21:50.

Find:
```
- [ ] **Phase 5: Friend Feed** - Users build a friend circle and see a reciprocity-gated daily feed of what friends rolled and rated
```
Replace with:
```
- [ ] **Phase 5: Friend Feed** - Users build a friend circle, see a reciprocity-gated Board of what friends spun and rated, react and comment, and are told when something happens
```

**A8. `.planning/ROADMAP.md`, Phase 5: goal, dependencies, requirements, criteria, plan list.** Checked 21:50. (The Phase 4 section also has a 'Plans' line, so each criterion stays in its find text.)

Find (goal):
```
**Goal**: Users build a friend circle and see what their friends rolled and rated each day, gated behind posting their own entry
```
Replace with:
```
**Goal**: Users build a friend circle, see what their friends spun and rated each day on the Board, gated behind posting their own entry, react and comment on friends' posts, and are told in a list behind the bell when something happens
```

Find:
```
**Depends on**: Phase 1, Phase 3
```
Replace with:
```
**Depends on**: Phase 1, Phase 3, Phase 4
```

Find:
```
**Requirements**: ACCT-02, FEED-01, FEED-02, FEED-03, FEED-04
```
Replace with:
```
**Requirements**: ACCT-02, FEED-01, FEED-02, FEED-03, FEED-04, FEED-05, NOTIF-01
```

Find (criterion 1):
```
  1. User can add friends to form their friend circle
```
Replace with:
```
  1. User can find a person by exact username, send a friend request, and become friends when it is accepted (and can decline, cancel, remove or block)
```

Find (criterion 2):
```
  2. User who has posted today's entry can view friends' entries for the day
```
Replace with:
```
  2. User who has posted today's entry can view friends' entries for the day on the Board, and for earlier days they posted
```

Find (the end of criterion 5, the plans line and the line after it):
```
  5. User can react to a friend's entry with a one-tap emoji

**Plans**: TBD
**UI hint**: yes
```
Replace with:
```
  5. User can react to a friend's entry with a one-tap emoji (one of six)
  6. User can comment on a friend's entry, and the writer or the owner of the post can delete a comment
  7. User sees a dot on the bell when something is unread and a list of what happened, and each item opens the right place

**Plans**: 0/6 plans executed

Plans:
**Wave 1**

- [ ] 05-01-PLAN.md: The friends server: friend requests, friendships, blocks, the one visibility rule and the migration of the whole phase (0007)
- [ ] 05-04-PLAN.md: The friends app: the Friends screen, the Friends number on Profile as a button, and blocking

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 05-02-PLAN.md: The Board server: the Board, posts, reactions, comments and the single photo-link function
- [ ] 05-03-PLAN.md: The notifications server: the derived list, the seen time and the bell summary

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 05-05-PLAN.md: The Board app: the Board with its locked, skipped, no-friends and unlocked states, the post page with reactions and comments, the additions to your own entry page, and the Board tab (the bar of five)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 05-06-PLAN.md: The notifications app: the list, the unread dots on the bell, the tap routing, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes
```
(The plan titles and waves were checked against the six plan files' front matter and `<objective>` at 21:50 IST: 05-01 and 05-04 are wave 1, 05-02 and 05-03 wave 2, 05-05 wave 3, 05-06 wave 4. The plan files still name the migration `0006_friend_feed` in places; it is 0007 now (Phase 2.1 took 0005 and Phase 3 took 0006), or 0008 if Phase 4 adds an index. Titles can be reworded: copy them from each plan's `<objective>` when applying.)

**A9. `.planning/ROADMAP.md`, the progress table.** Checked 21:50.

Find:
```
| 5. Friend Feed | 0/TBD | Not started | - |
```
Replace with:
```
| 5. Friend Feed | 0/6 | Not started | - |
```

**A10. `.planning/PROJECT.md`, Key Decisions: one new row.** Checked 21:50. Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| 2026-10-10: Settings, public or private, several photos (developer, voice).', which `0557d19` added after the Phase 3 row; or the Phase 4 row from 04-HAND-EDITS A7 if that was applied), keeping the blank line and the footer after the table as they are. The Outcome cell says just Pending.
```
| 2026-10-10: Phase 5 Board, friends and notifications. (1) Friends are found by exact username and must accept a request; Block is a friend action. A friend's posts are shown to accepted friends whatever the public or private setting, which is Phase 2.1's (default public) and which Phase 5 does not read. (2) The Board is a two-column grid of plain light photo cards (no frosted glass: the 2026-10-01 glass decision is dropped). It is locked until you have posted, a skip does not open it, and earlier days you posted stay readable, newest first. With no friends it shows an Add friends prompt above your own diary. (3) A post page shows the photo, title, stars, the author's reaction and note, a row of six reactions (love, fire, laughing, wow, sad, sleepy; one per person; none on your own post) and comments (friends only, up to 140 characters; the writer and the post's owner can delete). (4) The bell shows a small black dot when something is unread and opens a Notifications page above the current tab: reactions, comments, friend requests, accepted requests, "Your spin is open." and "Bonus spin added.", built from existing data with a per-person seen time and no stored events. (5) The bar is Home, Diary, Spin, Board, Profile | Developer, 2026-10-10: answers 5 (A), 6 (B), 7 ("forget it", read as dropping the glass look) and 8 (Recent activity belongs on the Profile page), their pictures 08 to 11, and their notification note; friend comments were tapped earlier the same day. Safe defaults told with them: the day in play runs 8 PM to 8 PM, newest first with your own post as one of the cards, a locked card shows the count and up to four first names, a friend's post shows only your own reaction, a blocked person is never told, photo links stay public until real storage exists (the visibility check and one link function are built). Phone alerts for reactions and comments need the developer's Apple push setup and are not in this phase. Details: `.planning/phases/05-friend-feed/05-CONTEXT.md` | Pending |
```

**A11. `.planning/PROJECT.md`, the Active list and the requirement count.** Checked 21:50.

Find:
```
- [ ] Users see a friend feed gated behind posting their own entry for the day, with lightweight one-tap emoji reactions
```
Replace with:
```
- [ ] Users see a friend feed (the Board) gated behind posting their own entry for the day, with lightweight one-tap emoji reactions and short comments, and a notifications list behind the bell
```

Find (this reads 22 across 6 phases if 04-HAND-EDITS A9 was applied; if not, it reads "(17 v1 requirements across 5 phases)", which is stale, and the new value is "(23 v1 requirements across 6 phases)"):
```
(22 v1 requirements across 6 phases)
```
Replace with:
```
(24 v1 requirements across 6 phases)
```

**A12. `.planning/PROJECT.md`, the moodboard bottom-navigation line.** Already done by 04-HAND-EDITS A8 (Home, Diary, Spin, Board, Profile). If 04 was not applied, apply A8 from there. Nothing more to do here.

**A13. `.planning/PROJECT.md`, Launch gate.** Checked 21:50. (06-HAND-EDITS appends to the same sentence; apply this first.)

Find:
```
(not part of plan 01-18).
```
Replace with:
```
(not part of plan 01-18). Added 2026-10-10 (Phase 5): entry photo links are public until the real storage account exists and the single photo-link function is switched to short-lived signed links, and phone alerts for reactions, comments and requests need the developer's Apple push setup.
```

## B. The glass decision is dropped (answer 7)

**B1. `.planning/PROJECT.md`, Key Decisions row 'Friend feed look'.** Checked 21:50. Mark it superseded; do not delete it (it is the record, and the older rows are kept the same way).

Find:
```
| Friend feed look: an Instagram-style vertical feed of posts, each a frosted glass card over a soft blurred background (feed only; every other screen stays light editorial) |
```
Replace with:
```
| SUPERSEDED 2026-10-10 (the glass look is dropped and the Board uses plain light cards, see the Phase 5 row below): Friend feed look: an Instagram-style vertical feed of posts, each a frosted glass card over a soft blurred background (feed only; every other screen stays light editorial) |
```

**B2. `.planning/PROJECT.md`, Context bullet 'Friend feed look'.** Checked 21:50.

Find:
```
**Friend feed look (2026-10-01, feed only):**
```
Replace with:
```
**Friend feed look (2026-10-01, feed only; SUPERSEDED 2026-10-10: the glass look was dropped and the Board uses plain light cards, see Key Decisions):**
```

**B3. `docs/design-brief-2026-10-01-feed.md`, the status line.** Checked 21:50.

Find:
```
Status: **decided by the developer for the Phase 5 friend feed.**
```
Replace with:
```
Status: **SUPERSEDED 2026-10-10: the developer dropped the glass look ("forget it", asked light or dark) and the Board uses plain light cards as in their pictures 08 to 10 (`.planning/phases/05-friend-feed/05-CONTEXT.md` D-03). Kept as a record.** Originally: decided by the developer for the Phase 5 friend feed.
```

**B4. `docs/motion-interaction-direction.md`, the dated amendment.** Checked 21:50.

Find:
```
(amended 2026-10-01: the Phase 5 friend feed is the one exception, see `docs/design-brief-2026-10-01-feed.md`; the rule stands everywhere else)
```
Replace with:
```
(amended 2026-10-01 for the Phase 5 friend feed, then reversed 2026-10-10 when the glass look was dropped: the rule stands everywhere, with no exception)
```

## C. Notes on Phase 2 and Phase 3 documents (optional; nothing they build changes)

**C1. `.planning/phases/02-daily-roll/02-UI-SPEC.md`, section 4 item 1 (the bell's dot).** Checked 21:50 (revision 6, committed in `b7f799a`).

Find:
```
There is no unread dot in Phase 2 and no profile photo (Profile is on the bar).
```
Replace with:
```
There is no unread dot in Phase 2 and no profile photo (Profile is on the bar). Phase 5 adds the dot: a small `ink` dot at the top right of the bell while anything is unread (05-UI-SPEC).
```

**C2. `.planning/phases/02-daily-roll/02-UI-SPEC.md`, section 4.5 (the Notifications page is no longer a stub in Phase 5).** Checked 21:50 (revision 6, committed in `b7f799a`).

Find:
```
The real list (friend activity and comments) and phone alerts are Phase 5 (STATE.md, Phase 5 notes).
```
Replace with:
```
The real list (reactions and comments on your posts, friend requests and accepted requests, "Your spin is open." and "Bonus spin added.") is Phase 5 (05-UI-SPEC), built into this same page and route; the 8 PM phone reminder is Phase 6, and phone alerts for friends' actions need the developer's Apple push setup (launch list).
```

**C3. `.planning/phases/02-daily-roll/02-CONTEXT.md`, D-18.** Checked 21:50.

Find:
```
The real list, friend activity and comments, and phone alerts are Phase 5 (STATE.md Phase 5 notes).
```
Replace with:
```
The real list (reactions, comments, friend requests, spin open, bonus spin) is Phase 5 (05-CONTEXT D-06); the 8 PM reminder is Phase 6; phone alerts for friends' actions need the developer's Apple push setup (launch list).
```

**C4. `.planning/phases/03-daily-entry/03-CONTEXT.md`, the out-of-scope list** (the STATE.md todo "add friends' comments to the Phase 3 out-of-scope list"). Checked 21:50.

Find:
```
- Friends, the feed, friends' reactions, friend-only photo links, reporting a post (Phase 5).
```
Replace with:
```
- Friends, the feed, friends' reactions and comments, notifications, friend-only photo links, reporting a post (Phases 5 and 6).
```

**C5. Photo links are public until real storage exists (two places in the Phase 3 paper).** Checked 21:50. The Phase 3 text says Phase 5 swaps in friend-only links; Phase 5 builds the check and one link function, and the swap itself needs the real storage account.

In `.planning/phases/03-daily-entry/03-CONTEXT.md`, find:
```
until Phase 5 swaps in friend-only links.
```
Replace with:
```
until real storage exists (Phase 5 builds the visibility check and one photo-link function; switching it to short-lived signed links is a launch-list step).
```

In `.planning/phases/03-daily-entry/03-01-PLAN.md` (the risk table row), find:
```
Links are public until Phase 5 swaps in friend-only links.
```
Replace with:
```
Links are public until real storage exists; Phase 5 builds the visibility check and one photo-link function, and the switch to signed links is a launch-list step.
```

**C6. Phase 4 documents, the Friends number. No edit.** Checked 21:50. `04-CONTEXT.md` already says: "Tapping it does nothing in Phase 4 (no dead control: it is drawn as plain text); Phase 5 makes it open the Friends screen." Plan 05-04 turns the number into a button. After Phase 4 is built, re-check that the number is drawn as plain text, not as a pressable.

## D. Facts for STATE.md and HANDOFF.json (the other window's files, not edited here)

- Phase 5 (Friend Feed: friends, the Board, reactions, comments, notifications) is written on paper and waits for the developer's separate 'go', which can only follow Phase 4 being built, walked and closed. Documents: `.planning/phases/05-friend-feed/` (05-CONTEXT, 05-UI-SPEC, 05-VALIDATION, 05-01 to 05-06 PLAN, 05-HAND-EDITS). Nothing is built.
- The glass look is dropped: STATE.md's '[Phase 05]: Friend feed look' line (frosted glass cards) is superseded. `package.json` still lists `expo-glass-effect`; remove it when Phase 5 is built if nothing uses it (a code change, not a hand edit). No native rebuild is needed for the Board. The memory note about the lab's glass look should be rewritten once the developer confirms "forget it" meant "drop the glass".
- The reaction list is the picture's six (love, fire, laughing, wow, sad, sleepy): STATE.md's '[Phase 03]: Entry rules' line and `.continue-here.md`'s Phase 3 paragraph still name heart, laughing, wow, fire, clap, yum. The Phase 3 paper itself is already updated. The developer must be told once.
- STATE.md '[Phase 05]: Friend comments' line: the two open questions. (a) A phone alert when a friend comments: not in Phase 5 (needs the Apple push setup on the launch list; suggested none for now). (b) Renaming the author's own note from 'Comment' to 'Thought': still unanswered. If yes, it is a copy change in the Phase 3 UI spec (the field label and strings); the Phase 5 paper calls it "the author's note" and is unaffected.
- STATE.md Pending Todos: "Ask the Phase 4 and Phase 5 design questions ..." is stale (answered 2026-10-10, `~/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md`). "Friend comments and notifications: once the rename question is settled, add FEED-05 ..." is now written as A4 and A5 above, and the Phase 3 out-of-scope note as C4; `PHASE5-QUESTIONS.md` is superseded by `05-CONTEXT.md`.
- Plans (waves as in the plan files): 05-01 friends server and 05-04 friends app (wave 1), 05-02 Board server and 05-03 notifications server (wave 2), 05-05 Board app including the Board tab and the bar of five (wave 3), 05-06 notifications app, cutover and close-out (wave 4). Migration 0007 (Phase 1 clean-up 0003, Phase 2 0004, Phase 2.1 0005 `account_visibility`, Phase 3 0006; it becomes 0008 if Phase 4 adds its optional index; Phase 6 then uses 0008, or 0009 in that case). The 05-01 to 05-03 plan files still say `0006_friend_feed` and need that one-number fix. The bar becomes Home, Diary, Spin, Board, Profile; `app/(app)/_layout.tsx` is a launch file, so one cold launch is checked after it changes.
- Phase 2.1 (Settings) stores the public or private choice (`users.account_visibility`, default public, switch in Settings). Phase 5 does not read it: a friend's posts show to accepted friends whatever it says. Phase 6 reads it. REQUIREMENTS.md SET-03 says so.
- REQUIREMENTS.md and PROJECT.md coverage lines are stale at 17 (the list holds 21 since `0557d19`); 04-HAND-EDITS A3 and A9 fix them to 22 and this file takes them to 24.
- Flags to carry: the gate reads posted days, never entries (a deleted post keeps the day open); a skip does not open the Board; photo links stay public until real storage exists; phone alerts for friends' actions need the developer's Apple push setup; a friend walk needs two or three throwaway test accounts made through the API and deleted by exact email afterwards; the four real dev accounts must stay untouched.
- Hand edits still to apply: item 1a and sections A and B (required), C (optional).

## E. Already done, do not repeat

- The Phase 3 paper's reaction list (item 1): already the picture's six, applied by the other window on 2026-10-10 (16:12 to 16:13).
- REQUIREMENTS.md FEED-02 (the late badge) is unchanged on purpose.
- The bell and its empty Notifications page are already in the Phase 2 paper (02-UI-SPEC 4.5, 02-CONTEXT D-18); this phase fills them.
- The moodboard bottom-navigation line: 04-HAND-EDITS A8.
- REQUIREMENTS.md SET-01 to SET-04 (including SET-03, the public or private setting that Phase 6 reads), the ROADMAP Phase 2.1 entry and the PROJECT.md Settings row: already in (`0557d19`). The Overview and progress-table gaps that commit left are 04-HAND-EDITS A0.
- PHASE5-QUESTIONS.md's "Hand edits that follow" are superseded by this list (its glass tint line, its Today card, "No comment threads" and "batch it with Google Sign-In" are all out of date).
