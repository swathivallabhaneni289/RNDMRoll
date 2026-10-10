# Phase 6 hand edits (PHASE 6, NOTHING IS BUILT)

**NOT APPLIED.** Written 2026-10-10 by a background writer for the lead; amended the same evening for the developer's change of about 17:20 (on Global, anyone who can see a public post can react AND comment, exactly as on a friend's post, so reporting covers comments too); and RE-VERIFIED at 21:50 IST after the other window's commits `b7f799a`, `0557d19` and `a491fc9`. That last round changed this file in six ways: (1) ACCT-04 is REMOVED, because Phase 2.1 owns the public or private setting (SET-03, `users.account_visibility`, default public, a "Private account" switch in Settings) and Phase 6 only reads it; (2) the reminder switch is a row on Phase 2.1's Settings page, not on the Edit profile page; (3) the migration is 0008 (0009 if Phase 4 adds its index), with no account table; (4) the requirement totals are recomputed from the live list; (5) the ROADMAP Overview now goes from six phases to seven, and the plan list follows the three plan files; (6) the Key Decisions table now ends with the Phase 2.1 row. Every Find string below was checked against the live file at 21:50 IST and occurs exactly once, unless the item says otherwise. Finds that quote text which 04-HAND-EDITS or 05-HAND-EDITS puts in place were checked against those files as they stand, NOT against the live documents (they are not applied yet): apply 04 and 05 first, and if a Find here is missing, the earlier file was not applied, or its replacement text was changed since.

These are the exact edits to SHARED documents that follow from the Phase 6 decisions (`06-CONTEXT.md`). The Phase 6 documents are complete without them, but the shared documents will disagree with Phase 6 until they are applied. Phase 6 has NO pictures of its own yet: everything on screen is a default built from pictures 08 to 10 and from Phase 2.1's Settings page, and the developer's pictures of the Global view, the person page, the report sheet and the reminder row would replace it.

How to use:
- Apply by hand with explicit paths, AFTER the other window has committed its own edits to `.planning/`, after 04-HAND-EDITS and 05-HAND-EDITS, and after Phase 5 is closed (section A is required before Phase 6 is built; the rest change nothing that Phases 2 to 5 build). Never apply them inside the other window's working session: a GSD commit stages all of `.planning/`.
- Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words. Where a find string quotes old text that holds the words "roll" or "rolled", that is a quote of old text, not new wording.
- Section A is requirements, B the roadmap, C PROJECT.md, D the Phase 2 and 2.1 documents (D-14 is reversed), E a check on the Phase 5 documents, F facts for STATE.md and HANDOFF.json (the other window's files), G what is already done.

## A. `.planning/REQUIREMENTS.md` (required)

**A0. No ACCT-04.** Do NOT add a requirement for the public or private setting. REQUIREMENTS.md already holds SET-03 ("User can make their account public or private (default public); the choice is stored and shown, and read by the Friends view and Global in Phases 5 and 6"), traced to Phase 2.1. Phase 6 only reads that setting. (The first version of this file added ACCT-04, with "Private by default" and "never Public under 18"; both were superseded by the developer's later words and by `0557d19`.)

**A1. FEED-06 (Global).** Apply after 05-HAND-EDITS A4 (it quotes the end of the FEED-05 line written there). Checked against 05-HAND-EDITS at 21:50. Reactions AND comments are open to everyone on public posts; the public or private choice is SET-03's. FEED-05 (Phase 5) stays exactly as 05 wrote it; FEED-06 extends it from a friend's post to any public post.

Find:
```
the comment's writer and the post's owner can delete a comment
```
Replace with:
```
the comment's writer and the post's owner can delete a comment
- [ ] **FEED-06**: User can switch the Board between Friends and Global; Global shows, newest first, the posts of accounts that are public (SET-03), to every signed-in person whatever their own account setting, at any time (it is not gated behind posting); anyone who can see a public post can react to it and read and write comments on it, exactly as on a friend's post (a blocked pair excepted)
```

**A2. NOTIF-02 (the 8 PM phone reminder) and MOD-01 (report a post or a comment, block a person).** Apply after 05-HAND-EDITS A5 (it quotes the end of the NOTIF-01 line written there). Checked against 05-HAND-EDITS at 21:50. The reminder switch is on Phase 2.1's Settings page.

Find:
```
tapping an item opens the post (their own, with its comments), the Friends screen or Home
```
Replace with:
```
tapping an item opens the post (their own, with its comments), the Friends screen or Home
- [ ] **NOTIF-02**: User gets a phone notification at 8:00 PM in their own time zone that their spin is open, even when the app is closed, and can turn it off with a switch on the Settings page

### Safety

- [ ] **MOD-01**: User can report a post, or a comment that is not their own, with a reason from a short list; the reported post or comment disappears from their lists and the report is stored for the developer to read; user can block a person from a post, a comment or the person page (ACCT-02)
```

**A3. Traceability and coverage.** Apply after 05-HAND-EDITS A6. Checked against 05-HAND-EDITS at 21:50. No ACCT-04 row (A0). The live list holds 21 requirements, 04-HAND-EDITS takes the total to 22 (DIARY-03), 05 to 24 (FEED-05, NOTIF-01) and this file to 27 (FEED-06, MOD-01, NOTIF-02).

Find:
```
| NOTIF-01 | Phase 5 | Pending |
```
Replace with:
```
| NOTIF-01 | Phase 5 | Pending |
| FEED-06 | Phase 6 | Pending |
| MOD-01 | Phase 6 | Pending |
| NOTIF-02 | Phase 6 | Pending |
```

Find (reads 24 after 04 and 05 were applied):
```
- v1 requirements: 24 total
```
Replace with:
```
- v1 requirements: 27 total
```

Find (same rule):
```
- Mapped to phases: 24
```
Replace with:
```
- Mapped to phases: 27
```

**A4. The Out of Scope row 'Public launch / outside-user growth'.** Checked 21:50 (the row is untouched by 04, 05 and `0557d19`). Strangers can see, react to and comment on public posts, and accounts are public by default, so reporting and blocking are a precondition.

Find:
```
| Public launch / outside-user growth | Explicit user framing for this milestone; success is daily reliable use within a small friend group, not adoption |
```
Replace with:
```
| Public launch / outside-user growth | Explicit user framing for this milestone; success is daily reliable use within a small friend group, not adoption. Accounts are public by default (SET-03), and Phase 6 builds a Global view where strangers can see, react to and comment on each other's public posts, but only the test group uses it; opening the app to outside people stays behind the launch gate, and reporting a post or a comment and blocking a person must work first (PROJECT.md) |
```

## B. `.planning/ROADMAP.md` (required)

**B1. The Overview: six phases becomes seven.** Checked 21:50. The first Find reads "six" only after 04-HAND-EDITS A0a was applied; if it was not, the live text reads "in five phases:" and the Replace is the same.

Find:
```
in six phases:
```
Replace with:
```
in seven phases:
```

Find (the sentence also holds two long dash characters later on, which this item does not touch, so the find stops at its first full stop):
```
and finally the friend circle and reciprocity-gated feed that closes the loop.
```
Replace with:
```
then the friend circle and reciprocity-gated feed that closes the loop, and finally a Global view of public posts, reporting, and the 8 PM phone reminder (the friend group does not need them to use the app).
```
(The next sentence, "By the end of Phase 5, the full loop ... is usable daily by the developer and a small friend group", stays true and stays as it is.)

**B2. The phase list: a Phase 6 line.** Apply after 05-HAND-EDITS A7 (it quotes the end of the Phase 5 line written there). Checked against 05-HAND-EDITS at 21:50. If 05 was not applied, add the line directly after the line that begins '- [ ] **Phase 5: Friend Feed**'.

Find:
```
react and comment, and are told when something happens
```
Replace with:
```
react and comment, and are told when something happens
- [ ] **Phase 6: Public Accounts and Global** - Users see the posts of public accounts on a Global view of the Board, react and comment on them as on a friend's post, report a post or a comment, and get an 8 PM phone reminder
```

**B3. The Phase 6 detail section.** Checked 21:50. It goes after Phase 5's section and before the progress section. The public or private setting is Phase 2.1's, so this phase depends on it and lists no requirement for it.

Find:
```
## Progress
```
Replace with:
```
### Phase 6: Public Accounts and Global

**Goal**: Users see the posts of public accounts on a Global view of the Board that everyone can look at, react to and comment on as on a friend's post, can report a post or a comment, and get a phone reminder at 8:00 PM (whether an account is public is Phase 2.1's setting)
**Depends on**: Phase 2.1, Phase 5
**Requirements**: FEED-06, MOD-01, NOTIF-02
**Success Criteria** (what must be TRUE):

  1. User can switch the Board between Friends and Global, and Global shows the posts of public accounts to everyone, whatever their own setting, who can react and comment on them as on a friend's post
  2. User can open a person from a post, add them as a friend or block them, and see their public posts
  3. User can report a post or a comment, the report is stored, and the reported post or comment disappears from their lists
  4. User gets a phone notification at 8:00 PM local time that their spin is open, and can turn it off in Settings

**Plans**: 0/3 plans executed

Plans:
**Wave 1**

- [ ] 06-01-PLAN.md: The Global server: the extended visibility rule (it reads `users.account_visibility`), the Global list, the person page, reports of posts and comments, and the migration (0008)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 06-02-PLAN.md: The Global app: the Friends and Global switch, the post page for other people's posts with Report and Block, the quiet Report on a comment, and the person page

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 06-03-PLAN.md: The 8 PM reminder (one native rebuild) as a row on Settings, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes

**Note**: Phase 6 has no developer pictures of its own yet; its screens reuse the parts of pictures 08 to 10 and Phase 2.1's Settings page.

## Progress
```
(The plan titles, count and waves were checked against the three plan files' front matter and `<objective>` at 21:50 IST: 06-01 wave 1, 06-02 wave 2, 06-03 wave 3. The plan files' objectives still describe the first reading (an account setting of their own, "private by default", a consent sheet on the Edit profile page); the CONTEXT no longer does. Copy the titles from the plan files once they are corrected.)

**B4. The progress table.** Apply after 05-HAND-EDITS A9 (it reads the Phase 5 row written there; if 05 was not applied the row says 0/TBD). Checked against 05-HAND-EDITS at 21:50.

Find:
```
| 5. Friend Feed | 0/6 | Not started | - |
```
Replace with:
```
| 5. Friend Feed | 0/6 | Not started | - |
| 6. Public Accounts and Global | 0/3 | Not started | - |
```

**B5. The execution order line.** Checked 21:50. The line holds arrow characters, which this ASCII file does not retype: find the line that begins 'Phases execute in numeric order:' (under 'Execution Order'; after 04-HAND-EDITS A0c it lists 1, 2, 2.1, 3, 4, 5) and add ' ' followed by the arrow and the digit 6 at its end, so that it reads 1, 2, 2.1, 3, 4, 5, 6 with the same arrows.

## C. `.planning/PROJECT.md` (required)

**C1. Key Decisions: one new row.** Checked 21:50. Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| 2026-10-10: Settings, public or private, several photos (developer, voice).'; after 04 and 05 it is the Phase 5 row that begins '| 2026-10-10: Phase 5 Board, friends and notifications.'), keeping the blank line and the footer after the table as they are. The Outcome cell says just Pending.
```
| 2026-10-10: Phase 6 Global, report and the 8 PM reminder. (1) Whether an account is public or private is Phase 2.1's setting (SET-03: `users.account_visibility`, default public, a "Private account" switch in Settings that saves on toggle; an account under 18 starts private and can switch like anyone else); Phase 6 only reads it. A public account's posts show on Global, a second view on the Board next to Friends, to every signed-in person whatever their own setting; the setting is read at the time the list is opened, so switching to private hides past posts at once. Global is open at any time and is not gated by posting. Anyone who can see a public post can react to it and read and write comments on it, exactly as on a friend's post; the post's owner can delete any comment on it and the writer their own. (2) A person page (photo, name, @username, one relation button, Block, and their public posts) opens from a card or a post. (3) A post can be reported with one of four reasons and a comment that is not yours with one of three; the report is stored, the post or comment disappears for the reporter, and nothing acts automatically. Blocking a person is offered from a post, a comment and the person page. (4) The 8 PM reminder is a local daily phone notification (no Apple push key needed), asked once after the first post, with a switch as a row on Phase 2.1's Settings page; it needs the expo-notifications module and one dev-client rebuild | Developer, 2026-10-10 (voice, cleaned): the Board shows friends or global; public accounts show on global; a private account can still look at global; Global is very similar to how it is for friends, with reactions and comments and everything; the 8 PM reminder should also go out as a phone notification. Their later words on the setting ("totally up to the user", "originally it's gonna be a public account") are in the Phase 2.1 row above and win over the first Phase 6 notes (private by default, under 18 never public, a consent sheet). Everything else is a default with no picture yet (the report reasons, the reminder asked once and a switch to turn it off), each a paper edit to change. Open before any outside user: whether an under-18 account may be public (Phase 2.1 allows it). Details: `.planning/phases/06-public-global/06-CONTEXT.md` | Pending |
```

**C2. Launch gate: what must exist before outsiders see each other's posts.** Apply after 05-HAND-EDITS A13 (it quotes the end of the sentence written there). Checked against 05-HAND-EDITS at 21:50. Strangers can comment, so reporting a comment and blocking are a precondition, not an extra.

Find:
```
comments and requests need the developer's Apple push setup.
```
Replace with:
```
comments and requests need the developer's Apple push setup. Added 2026-10-10 (Phase 6): before anyone outside the test group can see another person's posts, or comment on them, reporting a post or a comment and blocking a person must work in the app, and there must be a named person who reads the reports and how fast, Terms that say what is allowed and what is done with reports (the developer supplies the text; Claude writes none), a contact address for complaints (App Store rule for apps with user posts), and the developer's decision whether an account under 18 may be public (Phase 2.1 lets it start private and switch; refusing public for under 18 is one server line). Reporting and blocking exist in the app from Phase 6 and Phase 5; nothing acts on a report automatically.
```
(If 05-HAND-EDITS A13 was not applied, find `(not part of plan 01-18).` instead and put the same text after it.)

**C3. Out of Scope bullet: a clause, not a reversal.** Checked 21:50.

Find:
```
Public launch / outside-user growth for this milestone
```
Replace with:
```
Public launch / outside-user growth for this milestone (accounts are public by default, and Phase 6 builds a Global view where strangers can see, react to and comment on public posts, used by the test group only; opening the app to outside people stays behind the launch gate, and reporting a post or a comment and blocking a person must work first)
```

**C4. Active list: one bullet, and the requirement count.** Apply after 05-HAND-EDITS A11. Checked against 05-HAND-EDITS at 21:50.

Find:
```
and a notifications list behind the bell
```
Replace with:
```
and a notifications list behind the bell
- [ ] Users see the posts of public accounts on a Global view of the Board that anyone can react and comment on as on a friend's post, can report a post or a comment, and get an 8 PM phone reminder
```

Find (reads 24 across 6 phases after 04 and 05 were applied):
```
(24 v1 requirements across 6 phases)
```
Replace with:
```
(27 v1 requirements across 7 phases)
```

**C5. Constraints, the mobile stack: the one new native module.** Checked 21:50.

Find:
```
(expo-camera, expo-image-picker, expo-image)
```
Replace with:
```
(expo-camera, expo-image-picker, expo-image, and from Phase 6 expo-notifications for the 8 PM reminder)
```

## D. `.planning/phases/02-daily-roll/` and `02.1-settings/` (Phase 6 reverses D-14 and adds a Settings row; nothing Phase 2 or 2.1 builds changes)

**D1. `02-CONTEXT.md`, D-14.** Checked 21:50.

Find:
```
- **D-14 8 PM phone reminder:** Not in the first version. It needs a new phone feature, so the test app would have to be rebuilt, and no requirement asks for it. The Today countdown instead.
```
Replace with:
```
- **D-14 8 PM phone reminder:** Not in Phase 2. It needs a new phone feature (a native module), so the test app would have to be rebuilt. REVERSED on 2026-10-10 by the developer ("this should also go out as a phone notification"): it is built in Phase 6 as a local daily notification (06-CONTEXT, NOTIF-02), with the one rebuild. Phase 2 keeps the Today countdown and builds none of it.
```

**D2. `02-CONTEXT.md`, the end of D-18.** Checked 21:50. (05-HAND-EDITS C3 edits an earlier sentence of the same line; the two finds do not overlap.)

Find:
```
D-14 (no 8 PM phone reminder in this version) and the native rebuild point stand.
```
Replace with:
```
D-14 (no 8 PM phone reminder in this version) and the native rebuild point stand for Phase 2; Phase 6 reverses D-14 (06-CONTEXT).
```

**D3. `02-CONTEXT.md`, the Deferred list.** Checked 21:50.

Find:
```
- Phone alerts and the 8 PM reminder: later, needs a native rebuild.
```
Replace with:
```
- Phone alerts and the 8 PM reminder: the reminder is Phase 6 (a local notification, one native rebuild); alerts sent by the server need the developer's Apple push setup (launch list).
```

**D4. `02.1-settings/02.1-CONTEXT.md`, D-05 (the Settings rows; optional pointer).** Checked 21:50. Phase 6 puts one row on the page Phase 2.1 builds; the other window owns that file, so apply only when it is quiet.

Find:
```
- **D-05 Rows, in this order:** Edit profile, Username, Password, Private account, Log out.
```
Replace with:
```
- **D-05 Rows, in this order:** Edit profile, Username, Password, Private account, Log out. (Phase 6 adds one row, "Remind me at 8:00 PM", directly under Private account: 06-CONTEXT.)
```

(`02-02-PLAN.md` and `02-03-PLAN.md` list "phone reminders" and "a native rebuild" as NOT in Phase 2. That stays true; no edit.)

## E. A check on the Phase 5 documents (no Find)

**E1. Done, checked 21:50.** `05-UI-SPEC.md` already says that the Board has no Friends and Global switch in Phase 5 and that it arrives in Phase 6 (its sections 3 and 8 and its choices list). Nothing to add.

**E2.** `05-HAND-EDITS.md` A13 (the launch gate), A8 (the plan list) and A10 (the Key Decisions row) are the text that C2, B2 and C1 above quote or follow: if any is reworded there, reword the Find here to match.

## F. Facts for STATE.md and HANDOFF.json (the other window's files, not edited here)

- Phase 6 (Public Accounts and Global, with the 8 PM reminder) is written on paper and waits for the developer's separate 'go', which can only follow Phase 5 being built, walked and closed. Documents: `.planning/phases/06-public-global/` (06-CONTEXT, 06-UI-SPEC, 06-VALIDATION, 06-01 to 06-03 PLAN, 06-HAND-EDITS). Nothing is built. It has NO developer pictures of its own: it reuses the parts of pictures 08 to 10.
- Origin: the developer's voice notes of 2026-10-10 (the Board shows friends or global; public and private accounts; a private account can still look at global; Global is "very similar to how it is for friends", with reactions and comments and everything) and their note that the 8 PM reminder should also go out as a phone notification.
- The public or private setting is NOT Phase 6's: Phase 2.1 stores it (`users.account_visibility`, default public, "Private account" switch in Settings, under-18 accounts start private and may switch; REQUIREMENTS.md SET-03). Phase 6 reads the author's column when a list is read. The older Phase 6 notes ("Private by default", "under 18 is always Private", a "Public account" switch on the Edit profile page, a consent sheet, `account_settings` and the `/v1/account/visibility` routes, ACCT-04) are gone from 06-CONTEXT and from this file; the 06-01, 06-02 and 06-03 plan files, `06-UI-SPEC.md` and `06-VALIDATION.md` may still carry them and need the same correction.
- Defaults to tell the developer once (each is a paper edit to change): Global is open at any time and not gated by posting (so a Public friend is visible on Global without posting); comments on a public post work exactly as on a friend's post (anyone who can see it can read and write them; the owner can delete any comment on it, the writer their own; a quiet Report sits where Delete would be); no per-post switch; a post can be reported with four reasons and a comment with three, and nothing acts on a report automatically; the reminder is a row on Settings, asked once after the first post. Open for the developer before any outside user: whether an under-18 account may be public.
- STATE.md: if its Phase 6 notes (or any Phase 5 note) say that comments on Global are friends-only, or that a stranger sees no comments, or "Private by default", correct them: those were first readings, replaced on 2026-10-10.
- The reminder needs `expo-notifications` and ONE dev-client rebuild (`npx expo prebuild --clean`, which also drops the two native modules Apple and Google sign-in left in `ios/`, as the `.continue-here.md` loose-ends note says). No other phase needs a rebuild. Phase 2's 'no new native module' scans are unaffected because Phase 6 builds later.
- Migration 0008 (`post_reports`, `comment_reports` and one index to read public entries newest first; NO account table): Phase 1's clean-up is 0003, Phase 2's 0004, Phase 2.1's 0005, Phase 3's 0006, Phase 5's 0007; Phase 4 adds one only if it needs an index, and then Phase 5 is 0008 and Phase 6 is 0009. The 06-01 plan file still says `0007_public_global` and needs that fix. The visibility rule becomes one function used by every route that returns a post, a card, a reaction, a comment or a photo link; whoever can see an entry can read and write its comments, so Phase 5's `comments`, `comment_count` and `can_comment` keep their Phase 5 shape in Phase 6 (never null).
- Launch list additions (written into PROJECT.md by C2): who reads the reports and how fast, Terms that say what is allowed and what is done with reports, a contact address for complaints, and the under-18 decision. These are the developer's, not Claude's (Claude writes no legal text).
- Hand edits still to apply: sections A to D (required), E (a check).

## G. Already done, do not repeat

- `05-CONTEXT.md` already says the Board has no switch until Phase 6, that Block is a Phase 5 action, and that Report, Global and the person page are Phase 6; its data note already says migration 0007 and that Phase 2.1 owns the setting.
- REQUIREMENTS.md SET-03, the ROADMAP Phase 2.1 entry and the PROJECT.md Settings row (default public, under 18 starts private, "Supersedes the Phase 6 notes' Private by default") are already in (`0557d19`).
- The bell and the Notifications page are Phase 2 and Phase 5; the reminder here is a separate, phone-level notification and adds nothing to that page.
- REQUIREMENTS.md already says the Phase 3 reaction list is "one of six"; this phase changes no reaction.
- The statement "Public launch / outside-user growth" stays out of scope: A4 and C3 only add that Phase 6 builds a Global view for the test group, and that reporting a post or a comment and blocking a person must work before outside use (strangers can comment).
- Phase 5's comment fields are not changed by Phase 6 (no edit to `05-HAND-EDITS.md`, `05-CONTEXT.md` or FEED-05): comments follow the post's visibility, so a stranger on a public post gets the same `comments`, `comment_count` and `can_comment` as a friend. FEED-06 only extends FEED-05 from a friend's post to any public post.

**Closing note.** Re-verified at 21:50 IST on 2026-10-10 (the amendment for the developer's change of about 17:20 was made earlier the same evening). Each live-file Find in this file occurs exactly once; each Find that quotes 04-HAND-EDITS or 05-HAND-EDITS text was checked against those files as they stand. The totals are 21 live requirements, 22 after 04, 24 after 05 and 27 after this file; the phases are 6 after 04 (Phase 2.1 counted) and 7 after this file.
