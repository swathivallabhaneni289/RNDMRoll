# Phase 6 hand edits (PHASE 6, NOTHING IS BUILT)

**NOT APPLIED.** Written 2026-10-10 by a background writer for the lead, and amended the same evening for a change of decision (the developer, 2026-10-10 about 17:20, `06-CONTEXT.md` D-03): on Global, anyone who can see a public post can react AND comment, exactly as on a friend's post, so reporting covers comments too and "comments stay between friends" is gone from this file. Every Find string below was checked against the live file on 2026-10-10 between 16:12 and 16:18 IST, and checked again after the amendment at 21:16 IST (see the closing note of section G), and occurs exactly once, unless the item says otherwise. Finds that quote text which 04-HAND-EDITS or 05-HAND-EDITS puts in place were checked against that text as written in those files, NOT against the live documents (they are not applied yet): apply 04 and 05 first, and if a Find here is missing, the earlier file was not applied, or its replacement text was changed since.

These are the exact edits to SHARED documents that follow from the Phase 6 decisions (`06-CONTEXT.md`). The Phase 6 documents are complete without them, but the shared documents will disagree with Phase 6 until they are applied. Phase 6 has NO pictures yet: everything on screen is a default, and the developer's pictures of the Global view, the privacy switch, the person page and the report sheet would replace it.

How to use:
- Apply by hand with explicit paths, AFTER the other window has committed its own edits to `.planning/`, after 04-HAND-EDITS and 05-HAND-EDITS, and after Phase 5 is closed (section A is required before Phase 6 is built; the rest change nothing that Phases 2 to 5 build). Never apply them inside the other window's working session: a GSD commit stages all of `.planning/`.
- Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words. Where a find string quotes old text that holds the words "roll" or "rolled", that is a quote of old text, not new wording.
- Section A is requirements, B the roadmap, C PROJECT.md, D the Phase 2 documents (D-14 is reversed), E a check on the Phase 5 documents, F facts for STATE.md and HANDOFF.json (the other window's files), G what is already done.

## A. `.planning/REQUIREMENTS.md` (required)

**A1. ACCT-04 (public or private account).** Checked 16:18.

Find:
```
- [x] **ACCT-03**: User can view their own profile
```
Replace with:
```
- [x] **ACCT-03**: User can view their own profile
- [ ] **ACCT-04**: User can set their account to Public or Private (new accounts start Private, and an account under 18 can never be Public); a Public account's posts also appear on Global
```

**A2. FEED-06 (Global).** Apply after 05-HAND-EDITS A4 (it quotes the end of the FEED-05 line written there). Amended 21:16: reactions AND comments are open to everyone on public posts. FEED-05 (Phase 5) stays exactly as 05 wrote it; FEED-06 extends it from a friend's post to any public post.

Find:
```
the comment's writer and the post's owner can delete a comment
```
Replace with:
```
the comment's writer and the post's owner can delete a comment
- [ ] **FEED-06**: User can switch the Board between Friends and Global; Global shows the posts of Public accounts, newest first, to every signed-in person whatever their own account setting, at any time (it is not gated behind posting); anyone who can see a public post can react to it and read and write comments on it, exactly as on a friend's post (a blocked pair excepted)
```

**A3. NOTIF-02 (the 8 PM phone reminder) and MOD-01 (report a post or a comment, block a person).** Apply after 05-HAND-EDITS A5 (it quotes the end of the NOTIF-01 line written there). Amended 21:16: MOD-01 covers comments.

Find:
```
tapping an item opens the post (their own, with its comments), the Friends screen or Home
```
Replace with:
```
tapping an item opens the post (their own, with its comments), the Friends screen or Home
- [ ] **NOTIF-02**: User gets a phone notification at 8:00 PM in their own time zone that their spin is open, even when the app is closed, and can turn it off on the Edit profile page

### Safety

- [ ] **MOD-01**: User can report a post, or a comment that is not their own, with a reason from a short list; the reported post or comment disappears from their lists and the report is stored for the developer to read; user can block a person from a post, a comment or the person page (ACCT-02)
```

**A4. Traceability and coverage.** Apply after 05-HAND-EDITS A6.

Find:
```
| NOTIF-01 | Phase 5 | Pending |
```
Replace with:
```
| NOTIF-01 | Phase 5 | Pending |
| ACCT-04 | Phase 6 | Pending |
| FEED-06 | Phase 6 | Pending |
| MOD-01 | Phase 6 | Pending |
| NOTIF-02 | Phase 6 | Pending |
```

Find (reads 20 after 04 and 05 were applied; 17 plus 1 plus 2 is 20):
```
- v1 requirements: 20 total
```
Replace with:
```
- v1 requirements: 24 total
```

Find (same rule):
```
- Mapped to phases: 20
```
Replace with:
```
- Mapped to phases: 24
```

**A5. The Out of Scope row 'Public launch / outside-user growth'.** Checked 16:15 and again 21:16 (the row is untouched by 04 and 05). Amended 21:16: strangers can now comment, so reporting and blocking are a precondition.

Find:
```
| Public launch / outside-user growth | Explicit user framing for this milestone; success is daily reliable use within a small friend group, not adoption |
```
Replace with:
```
| Public launch / outside-user growth | Explicit user framing for this milestone; success is daily reliable use within a small friend group, not adoption. Phase 6 builds public accounts and a Global view where strangers can see, react to and comment on each other's public posts, but only the test group uses them; opening the app to outside people stays behind the launch gate, and reporting a post or a comment and blocking a person must work first (PROJECT.md) |
```

## B. `.planning/ROADMAP.md` (required)

**B1. The Overview: five phases becomes six.** Checked 16:18.

Find:
```
RNDMRoll delivers a daily habit-loop mobile app in five phases:
```
Replace with:
```
RNDMRoll delivers a daily habit-loop mobile app in six phases:
```

Find (the sentence also holds two long dash characters later on, which this item does not touch, so the find stops at its first full stop):
```
and finally the friend circle and reciprocity-gated feed that closes the loop.
```
Replace with:
```
then the friend circle and reciprocity-gated feed that closes the loop, and finally public accounts with a Global view and the 8 PM phone reminder (the friend group does not need them to use the app).
```
(The next sentence, "By the end of Phase 5, the full loop ... is usable daily by the developer and a small friend group", stays true and stays as it is.)

**B2. The phase list: a Phase 6 line.** Apply after 05-HAND-EDITS A7 (it quotes the end of the Phase 5 line written there). If 05 was not applied, add the line directly after the line that begins '- [ ] **Phase 5: Friend Feed**'. Amended 21:16: comments and reporting a comment.

Find:
```
react and comment, and are told when something happens
```
Replace with:
```
react and comment, and are told when something happens
- [ ] **Phase 6: Public Accounts and Global** - Users can make their account public so their posts show on a Global view of the Board where anyone can react and comment as on a friend's post, report a post or a comment, and get an 8 PM phone reminder
```

**B3. The Phase 6 detail section.** Checked 16:18 and again 21:16. It goes after Phase 5's section and before the progress section. Amended 21:16: goal, criteria 2 and 4, and the wave split (the plan files now exist: 06-01 is wave 1, 06-02 wave 2 and 06-03 wave 3).

Find:
```
## Progress
```
Replace with:
```
### Phase 6: Public Accounts and Global

**Goal**: Users can make their account public so their posts show on a Global view of the Board that everyone can look at, react to and comment on as on a friend's post, can report a post or a comment, and get a phone reminder at 8:00 PM
**Depends on**: Phase 5
**Requirements**: ACCT-04, FEED-06, MOD-01, NOTIF-02
**Success Criteria** (what must be TRUE):

  1. User can set their account to Public or Private (Private by default; an account under 18 cannot be Public)
  2. User can switch the Board between Friends and Global, and Global shows the posts of Public accounts to everyone, whatever their own setting, who can react and comment on them as on a friend's post
  3. User can open a person from a post, add them as a friend or block them, and see their public posts
  4. User can report a post or a comment, the report is stored, and the reported post or comment disappears from their lists
  5. User gets a phone notification at 8:00 PM local time that their spin is open, and can turn it off

**Plans**: 0/3 plans executed

Plans:
**Wave 1**

- [ ] 06-01-PLAN.md: The public accounts server: the account setting, the Global feed, the person page and reports of posts and comments

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 06-02-PLAN.md: The public accounts app: the privacy switch, the Friends and Global switch, the post page for other people's posts with Report and Block, and the person page

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 06-03-PLAN.md: The 8 PM reminder (one native rebuild), cutover, gates, device walkthrough and phase close-out

**UI hint**: yes

**Note**: Phase 6 has no developer pictures of its own yet; its screens reuse the parts of pictures 08 to 10.

## Progress
```
(The plan titles, count and waves were checked against the three plan files' front matter and `<objective>` at 21:16 IST. Titles can still be reworded; copy them from each plan's `<objective>` when applying, and the count from `ls`.)

**B4. The progress table.** Apply after 05-HAND-EDITS A9 (it reads the Phase 5 row written there; if 05 was not applied the row says 0/TBD).

Find:
```
| 5. Friend Feed | 0/6 | Not started | - |
```
Replace with:
```
| 5. Friend Feed | 0/6 | Not started | - |
| 6. Public Accounts and Global | 0/3 | Not started | - |
```

**B5. The execution order line.** Checked 16:18. The line holds arrow characters, which this ASCII file does not retype: find the line that begins 'Phases execute in numeric order:' (under 'Execution Order') and add ' ' followed by the arrow and the digit 6 at its end, so that it ends '4 -> 5 -> 6' in arrow form.

## C. `.planning/PROJECT.md` (required)

**C1. Key Decisions: one new row.** Checked 21:16. Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| 2026-10-09: Phase 3 entry rules.'; after 04 and 05 it is the Phase 5 row that begins '| 2026-10-10: Phase 5 Board, friends and notifications.'), keeping the blank line and the footer after the table as they are. The Outcome cell says just Pending. Amended 21:16: comments are open to everyone on public posts, and a comment can be reported.
```
| 2026-10-10: Phase 6 public accounts, Global, report and the 8 PM reminder. (1) An account is Public or Private. New accounts and all existing accounts are Private, and an account under 18 can never be Public (checked on the server from the birthday). A Public account's posts also show on Global, a second view on the Board next to Friends, to every signed-in person whatever their own setting. Global is open at any time and is not gated by posting. Anyone who can see a public post can react to it and read and write comments on it, exactly as on a friend's post; the post's owner can delete any comment on it and the writer their own. Turning Public on asks first, because past posts become visible. (2) A person page (photo, name, @username, one relation button, Block, and their public posts) opens from a card or a post. (3) A post can be reported with one of four reasons and a comment that is not yours with one of three; the report is stored, the post or comment disappears for the reporter, and nothing acts automatically. Blocking a person is offered from a post, a comment and the person page. (4) The 8 PM reminder is a local daily phone notification (no Apple push key needed), asked once after the first post, with a switch on the Edit profile page; it needs the expo-notifications module and one dev-client rebuild | Developer, 2026-10-10 (voice, cleaned): the Board shows friends or global; public accounts show on global; a private account can still look at global; Global is very similar to how it is for friends, with reactions and comments and everything; the 8 PM reminder should also go out as a phone notification. Everything else is a default with no picture yet (Private by default, under 18 never Public, Global ungated, the report reasons, the reminder asked once and a switch to turn it off), each a paper edit to change. Details: `.planning/phases/06-public-global/06-CONTEXT.md` | Pending |
```

**C2. Launch gate: what must exist before outsiders see each other's posts.** Apply after 05-HAND-EDITS A13 (it quotes the end of the sentence written there). Checked against the text as written in 05 (21:16). Amended 21:16: strangers can comment, so reporting a comment and blocking are a precondition, not an extra.

Find:
```
comments and requests need the developer's Apple push setup.
```
Replace with:
```
comments and requests need the developer's Apple push setup. Added 2026-10-10 (Phase 6): before anyone outside the test group can see another person's posts, or comment on them, reporting a post or a comment and blocking a person must work in the app, and there must be a named person who reads the reports and how fast, Terms that say what is allowed and what is done with reports (the developer supplies the text; Claude writes none), and a contact address for complaints (App Store rule for apps with user posts). Reporting and blocking exist in the app from Phase 6 and Phase 5; nothing acts on a report automatically.
```
(If 05-HAND-EDITS A13 was not applied, find `(not part of plan 01-18).` instead and put the same text after it.)

**C3. Out of Scope bullet: a clause, not a reversal.** Checked 16:18 and again 21:16. Amended 21:16.

Find:
```
Public launch / outside-user growth for this milestone
```
Replace with:
```
Public launch / outside-user growth for this milestone (Phase 6 builds public accounts and a Global view where strangers can see, react to and comment on public posts, used by the test group only; opening the app to outside people stays behind the launch gate, and reporting a post or a comment and blocking a person must work first)
```

**C4. Active list: one bullet, and the requirement count.** Apply after 05-HAND-EDITS A11. Amended 21:16: the bullet names comments and reporting a comment.

Find:
```
and a notifications list behind the bell
```
Replace with:
```
and a notifications list behind the bell
- [ ] Users can make their account public so their posts show on a Global view of the Board that anyone can react and comment on as on a friend's post, report a post or a comment, and get an 8 PM phone reminder
```

Find (reads 20 after 04 and 05 were applied):
```
(20 v1 requirements across 5 phases)
```
Replace with:
```
(24 v1 requirements across 6 phases)
```

**C5. Constraints, the mobile stack: the one new native module.** Checked 16:18.

Find:
```
(expo-camera, expo-image-picker, expo-image)
```
Replace with:
```
(expo-camera, expo-image-picker, expo-image, and from Phase 6 expo-notifications for the 8 PM reminder)
```

## D. `.planning/phases/02-daily-roll/` (Phase 6 reverses D-14; nothing Phase 2 builds changes)

**D1. `02-CONTEXT.md`, D-14.** Checked 16:18.

Find:
```
- **D-14 8 PM phone reminder:** Not in the first version. It needs a new phone feature, so the test app would have to be rebuilt, and no requirement asks for it. The Today countdown instead.
```
Replace with:
```
- **D-14 8 PM phone reminder:** Not in Phase 2. It needs a new phone feature (a native module), so the test app would have to be rebuilt. REVERSED on 2026-10-10 by the developer ("this should also go out as a phone notification"): it is built in Phase 6 as a local daily notification (06-CONTEXT, NOTIF-02), with the one rebuild. Phase 2 keeps the Today countdown and builds none of it.
```

**D2. `02-CONTEXT.md`, the end of D-18.** Checked 16:18.

Find:
```
D-14 (no 8 PM phone reminder in this version) and the native rebuild point stand.
```
Replace with:
```
D-14 (no 8 PM phone reminder in this version) and the native rebuild point stand for Phase 2; Phase 6 reverses D-14 (06-CONTEXT).
```

**D3. `02-CONTEXT.md`, the Deferred list.** Checked 16:18.

Find:
```
- Phone alerts and the 8 PM reminder: later, needs a native rebuild.
```
Replace with:
```
- Phone alerts and the 8 PM reminder: the reminder is Phase 6 (a local notification, one native rebuild); alerts sent by the server need the developer's Apple push setup (launch list).
```

(`02-02-PLAN.md` and `02-03-PLAN.md` list "phone reminders" and "a native rebuild" as NOT in Phase 2. That stays true; no edit.)

## E. A check on the Phase 5 documents (no Find: the files were still being written)

**E1.** When `05-UI-SPEC.md` exists, search it for 'Global'. It must say that the Board has no Friends and Global switch in Phase 5 and that Phase 6 adds it under the title (`05-CONTEXT.md` already says so). If it does not, add this sentence next to the Board header description: 'The Board has no Friends | Global switch yet; Phase 6 adds it under the title (06-CONTEXT).'

**E2.** `05-HAND-EDITS.md` A13 (the launch gate) and A8 (the plan list) are the text that C2 and B2 above quote: if either is reworded there, reword the Find here to match.

## F. Facts for STATE.md and HANDOFF.json (the other window's files, not edited here)

- Phase 6 (Public Accounts and Global, with the 8 PM reminder) is written on paper and waits for the developer's separate 'go', which can only follow Phase 5 being built, walked and closed. Documents: `.planning/phases/06-public-global/` (06-CONTEXT, 06-UI-SPEC, 06-VALIDATION, 06-01 to 06-03 PLAN, 06-HAND-EDITS). Nothing is built. It has NO developer pictures of its own: it reuses the parts of pictures 08 to 10.
- Origin: the developer's voice notes of 2026-10-10 (the Board shows friends or global; public and private accounts; a private account can still look at global; Global is "very similar to how it is for friends", with reactions and comments and everything, about 17:20) and their note that the 8 PM reminder should also go out as a phone notification.
- Defaults to tell the developer once (each is a paper edit to change): Private by default; an account under 18 can never be Public; Global is open at any time and not gated by posting (so a Public friend is visible on Global without posting); comments on a public post work exactly as on a friend's post (anyone who can see it can read and write them; the owner can delete any comment on it, the writer their own; a quiet Report sits where Delete would be); one account setting, no per-post switch; a post can be reported with four reasons and a comment with three, and nothing acts on a report automatically.
- STATE.md: if its Phase 6 notes (or any Phase 5 note) say that comments on Global are friends-only, or that a stranger sees no comments, correct them: that was the first reading, replaced on 2026-10-10 about 17:20.
- STATE.md '[Phase 05]: Friend comments' line: the question about a phone alert when a friend comments stays out (Apple push setup, launch list); the 8 PM reminder is Phase 6.
- The reminder needs `expo-notifications` and ONE dev-client rebuild (`npx expo prebuild --clean`, which also drops the two native modules Apple and Google sign-in left in `ios/`, as the `.continue-here.md` loose-ends note says). No other phase needs a rebuild. Phase 2's 'no new native module' scans are unaffected because Phase 6 builds later.
- Migration 0007 (`account_settings`, `post_reports`, `comment_reports`; Phase 5 uses 0006). The visibility rule becomes one function used by every route that returns a post, a card, a reaction, a comment or a photo link; whoever can see an entry can read and write its comments, so Phase 5's `comments`, `comment_count` and `can_comment` keep their Phase 5 shape in Phase 6 (never null).
- Launch list additions (written into PROJECT.md by C2): who reads the reports and how fast, Terms that say what is allowed and what is done with reports, a contact address for complaints. These are the developer's, not Claude's (Claude writes no legal text).
- Hand edits still to apply: sections A to D (required), E (a check).

## G. Already done, do not repeat

- `05-CONTEXT.md` already says the Board has no switch until Phase 6, that Block is a Phase 5 action, and that Report, public accounts and the person page are Phase 6.
- The bell and the Notifications page are Phase 2 and Phase 5; the reminder here is a separate, phone-level notification and adds nothing to that page.
- REQUIREMENTS.md already says the Phase 3 reaction list is "one of six"; this phase changes no reaction.
- The statement "Public launch / outside-user growth" stays out of scope: A5 and C3 only add that Phase 6 builds public accounts for the test group, and that reporting a post or a comment and blocking a person must work before outside use (strangers can comment).
- Phase 5's comment fields are not changed by Phase 6 (no edit to `05-HAND-EDITS.md`, `05-CONTEXT.md` or FEED-05): comments follow the post's visibility, so a stranger on a public post gets the same `comments`, `comment_count` and `can_comment` as a friend. FEED-06 only extends FEED-05 from a friend's post to any public post.

**Closing note, amended 2026-10-10 about 21:16 IST (the developer's change of about 17:20).** Every Find string in this file was checked again against the live files at 21:16 IST; each live-file Find occurs exactly once, and each Find that quotes 04-HAND-EDITS or 05-HAND-EDITS text was checked against that file as it stands (05-HAND-EDITS.md, 24,428 bytes). The edits made in this amendment: A2 (FEED-06 wording), A3 (MOD-01 covers comments and blocking), A5, B2, B3 (goal, criteria 2 and 4, waves 1 to 3), C1, C2, C3, C4, F and this closing note. No Find string was changed; only the Replace texts and the notes around them.
