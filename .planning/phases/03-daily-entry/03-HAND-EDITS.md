# Phase 3 hand edits (PHASE 3, NOTHING IS BUILT)

**APPLIED 2026-10-10 by the lead:** sections A, B and C (including the optional C9 and C10), in one go. One change: the PROJECT.md row also records the developer's later pick for the entry photo size (50 MB, the same ceiling as profile photos; the old notes said 5 MB), so it does not repeat the 5 MB figure. Section D is a list of facts, not an edit. The text below stays as a record of what was applied.

Written 2026-10-10 for the lead. These are the exact edits to SHARED documents that follow from the Phase 3 decisions. Nothing here has been applied. The Phase 3 documents themselves are complete without them, but the shared documents will disagree with Phase 3 until they are applied.

How to use:
- Apply by hand with explicit paths. Re-read each file just before editing: other helpers have been editing these files and the text may have moved. Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words.
- Section A is required. Section B changes the Phase 2 UI contract. Section C is a set of small notes for the Phase 2 plans; apply them BEFORE Phase 2 is built if it has not started (they make Phase 3 cheaper and change nothing Phase 2 builds), and skip them otherwise (Phase 3's plans re-check the built code anyway). Section D is a list of facts for STATE.md and HANDOFF.json, which belong to the lead.
- Every replacement is written so it stays true while Phase 2 is being built: Phase 2 builds none of the Phase 3 parts.

## A. Shared project documents (required)

**A1. `.planning/REQUIREMENTS.md`, ENTRY-01** (answers 2 and 3).

Find:
```
an optional one-tap reaction, and an optional comment (a short free-text thought), logged against the day's rolled category
```
Replace with:
```
an optional one-tap reaction (one of six fixed reactions), and an optional comment (a short free-text thought), logged against the day's rolled category. The author can change or delete a post; a deleted post still counts as a posted day
```

**A2. `.planning/ROADMAP.md`, Phase 3: the plan list, the plan count and two criteria.**

Find (criterion 2):
```
  2. User can log an entry with a photo (camera or library), title, half-star rating, and optional reaction
```
Replace with:
```
  2. User can log an entry with a photo (camera or library), title, half-star rating, and an optional reaction (one of six)
```

Find (the end of criterion 4, the plans line and the line after it; the other phases also have a 'Plans: TBD' line, so keep the criterion in the find text):
```
  4. User can skip a day when the rolled category isn't satisfiable and sees how it changes the week count (for example, 2 of 4 this week)

**Plans**: TBD
**UI hint**: yes
```
Replace with:
```
  4. User can skip a day when the rolled category isn't satisfiable and sees how it changes the week count (for example, 2 of 4 this week)
  5. User can change or delete their post afterwards, and a deleted post still counts as a posted day

**Plans**: 0/3 plans executed

Plans:
**Wave 1**

- [ ] 03-01-PLAN.md: The entry server: entries table, photo upload ticket, post, edit, delete and skip, and the Today changes
- [ ] 03-02-PLAN.md: The entry app: Log it, the entry screen with half stars and six reactions, and the Posted state

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 03-03-PLAN.md: The entry page (view, edit, delete), skip, the skip mark, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes
```

**A3. `.planning/ROADMAP.md`, the progress table.**

Find:
```
| 3. Daily Entry | 0/TBD | Not started | - |
```
Replace with:
```
| 3. Daily Entry | 0/3 | Not started | - |
```

**A4. `.planning/PROJECT.md`, Key Decisions: one new row.** Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| Apple and Google sign-in are REMOVED'), keeping the blank line and the footer after the table as they are (the Outcome cell says just Pending; the older rows use a dash character there, which is left alone):
```
| 2026-10-09: Phase 3 entry rules. (1) A movie or book day may log anything watched or read lately; no date is asked and nothing says 'earlier'; the free skip is for people with nothing. (2) A reaction is one of six fixed emoji: heart, laughing, wow, fire, clap, yum. Tap the chosen one again to clear it. The author and friends use the same six (Phase 5 reuses them). The six are content the developer chose, not icons, so they appear only in the reaction picker and where a chosen reaction is shown. (3) The author can change title, stars, reaction, comment and photo at any time and can delete the post; a deleted post still leaves the day counted as posted and settled, so there is no second post that day | Developer decisions, 2026-10-09, answering the three Phase 3 questions with the suggested option A each (the questions were prepared 2026-10-08). The safe defaults told with them: stars stored as a whole number 1 to 10 and shown as '3.5', title 1 to 80 characters, comment up to 140, a photo required (JPG, PNG or WebP, up to 50 MB like profile photos), one entry per day belonging to the final spin, late decided by the server clock. Entry photos are stored by key and the server builds the link, so Phase 5 can swap in friend-only links. Details: `.planning/phases/03-daily-entry/03-CONTEXT.md` | Pending |
```

**A5. `.planning/PROJECT.md`, Constraints: a clarification of the emoji rule (recommended).**

Find:
```
no emoji-as-icons, no em dashes
```
Replace with:
```
no emoji-as-icons (the six reaction emoji chosen for Phase 3 are content, not icons), no em dashes
```

## B. `.planning/phases/02-daily-roll/02-UI-SPEC.md` (the Phase 3 changes to the Phase 2 contract)

**B1. Section 4.1, the Result and Late rows, plus two new rows.**

Find:
```
| Result | spun today | Today's spin. | Next spin opens tomorrow at 8:00 PM. | result block (4.3) | still, winner under pointer |
| Late | after midnight, before the next 8:00 PM, spin not posted | Last night's spin. | Next spin opens at 8:00 PM. | result block | still |
```
Replace with:
```
| Result | spun today | Today's spin. | Next spin opens tomorrow at 8:00 PM. | result block (4.3); Phase 3 adds 'Log it' and 'Skip this one' below it (03-UI-SPEC section 3.2) | still, winner under pointer |
| Late | after midnight, before the next 8:00 PM, spin not posted | Last night's spin. | Next spin opens at 8:00 PM. | result block; Phase 3 adds the late line, 'Log it' and 'Skip this one' (03-UI-SPEC section 3.2) | still |
| Posted | Phase 3 only: the shown spin's day has a post | Posted. | Next spin opens tomorrow at 8:00 PM. (last night's spin: Next spin opens at 8:00 PM.) | result block, then the posted block (03-UI-SPEC section 3.3) | still |
| Skipped | Phase 3 only: the shown spin's day was skipped | Skipped. | same as Posted | result block, then the skipped block (03-UI-SPEC section 3.4) | still |
```

**B2. Section 4.1, the first note.**

Find:
```
- There is no button on Result or Late until Phase 3 can post.
```
Replace with:
```
- Phase 2 builds no button on Result or Late. Phase 3 adds 'Log it', 'Skip this one' and the late line there, and the Posted and Skipped states (03-UI-SPEC sections 3.1 to 3.5).
```

**B3. Section 4.3 (Result block), the last sentence.**

Find:
```
No other button, no share, no "log it" until Phase 3.
```
Replace with:
```
No other button and no share in Phase 2. Phase 3 adds 'Log it' and 'Skip this one' below this block (03-UI-SPEC section 3.2).
```

**B4. Section 6 (Weekly card), the dot states.**

Find:
```
A skipped day gets its own mark in Phase 3.
```
Replace with:
```
A skipped day gets its own mark in Phase 3 (an ink dash, 03-UI-SPEC section 8); Phase 2 never shows one.
```

**B5. Section 11 (Accessibility), the weekly card line.**

Find:
```
"Monday posted, Tuesday not posted", and so on.
```
Replace with:
```
"Monday posted, Tuesday not posted" (Phase 3 adds "skipped"), and so on.
```

**B6. Section 13 (Strings for approval), row 3.**

Find:
```
The late line "Posting it now would count as late." is left out until Phase 3, when posting exists.
```
Replace with:
```
The late line "Posting it now would count as late." is left out of Phase 2; Phase 3 adds it (03-UI-SPEC row 1).
```

## C. Other Phase 2 documents (small notes; best applied before Phase 2 is built)

**C1. `02-CONTEXT.md`, D-01 (answer 1).**

Find:
```
A movie or book day may log the last one watched or read; Phase 3 settles the rule.
```
Replace with:
```
A movie or book day may log the last one watched or read; Phase 3 settled the rule on 2026-10-09: anything watched or read lately, no date asked.
```

**C2. `02-CONTEXT.md`, Claude's Discretion.**

Find:
```
- The result screen has no button until Phase 3.
```
Replace with:
```
- The result screen has no button in Phase 2. Phase 3 adds 'Log it' and 'Skip this one' (see 03-CONTEXT).
```

**C3. `02-01-PLAN.md`, Task 2c, the `MarkPosted` bullet** (so Phase 3 does not have to refactor built code).

Find:
```
if just made and none held, grant the bonus (`bonus_granted` true), else false. All under the lock.
```
Replace with:
```
if just made and none held, grant the bonus (`bonus_granted` true), else false. All under the lock. Write this body as an inner function that takes the already open transaction (for example `markPostedIn(tx, now, loc, userID, date, source)`), and let `MarkPosted` open the transaction and call it. Phase 3 calls the same inner function inside its own transaction, so a post and its entry commit together (03-01 Task 2c).
```

**C4. `02-01-PLAN.md`, binding decision 12 (`daySettled` reads the latest spin).**

Find:
```
or the spin's `result` is not `open` (Phase 3 sets that, and skipped).
```
Replace with:
```
or the date's LATEST spin's `result` is not `open` (a bonus spin leaves seq 0 at `open` for good; Phase 3 sets `posted` and `skipped` on the latest spin).
```

**C5. `02-01-PLAN.md`, Task 3b, the `state` bullet (the Today shape).**

Find:
```
- `state`: `waiting`, `open`, `result` or `late`. `can_use`: only in `result`, day not settled, bonus held.
```
Replace with:
```
- `state`: `waiting`, `open`, `result` or `late`. `can_use`: only in `result`, day not settled, bonus held. Phase 3 adds the states `posted` and `skipped`, a top-level `entry` and a `skipped` flag on each week day (03-01 Task 3b); keep the types open to them. Phase 2 never emits them.
```

**C6. `02-01-PLAN.md`, Task 3d, the reset tool.**

Find:
```
sets `bonus_held` 0. Wheel, zone and `seeded_*` stay.
```
Replace with:
```
sets `bonus_held` 0. Wheel, zone and `seeded_*` stay. (Phase 3 widens this: it also clears entries and real posted days, 03-01 Task 2c.)
```

**C7. `02-01-PLAN.md`, `<not_in_this_plan>`.**

Find:
```
(Phase 3 adds `entries.spin_id`, sets `spins.result`, calls `MarkPosted`)
```
Replace with:
```
(Phase 3 adds the `entries` table with its `spin_id`, adds `spins.settled_at`, sets `spins.result` and calls the inner function of `MarkPosted`)
```

**C8. `02-02-PLAN.md`, Task 2b, the `ResultBlock.tsx` bullet.**

Find:
```
No other button, and no posting line in Phase 2.
```
Replace with:
```
No other button, and no posting line in Phase 2 (Phase 3 adds its buttons in a separate component, `components/today/DayActions.tsx`, so this file keeps its shape).
```

**C9. `02-02-PLAN.md`, Task 1, the `todayStore.ts` description (optional).** After the sentence that ends 'So a second account never sees the first one's wheel, result or pending spin.', add:
```
The store also exports a way to put a Today that arrived from outside `useToday` into the store, obeying the same user-id rule (Phase 3 uses it after a post or a skip).
```

**C10. `02-03-PLAN.md`, Task 2, the Emoji scan (optional; it only matters if Phase 2's scans are re-run after Phase 3 exists).**

Find (the whole bullet):
```
- Emoji: `LC_ALL=C grep -rlE --exclude-dir=lab $'\xf0\x9f|\xe2[\x98-\x9e]' app components lib` prints nothing.
```
Replace with:
```
- Emoji: `LC_ALL=C grep -rlE --exclude-dir=lab $'\xf0\x9f|\xe2[\x98-\x9e]' app components lib` prints nothing. (This holds at Phase 2's close. From Phase 3 the one allowed hit is `lib/entry/reactions.ts`, the six reaction characters.)
```

## D. Facts for STATE.md and HANDOFF.json (the lead's files, not edited here)

- Phase 3 (Daily Entry) is written on paper and waits for the developer's separate 'go', which can only follow Phase 2 being built, walked and closed. Documents: `.planning/phases/03-daily-entry/` (03-CONTEXT, 03-UI-SPEC, 03-VALIDATION, 03-01-PLAN, 03-02-PLAN, 03-03-PLAN, 03-HAND-EDITS). Nothing is built.
- Plans: 03-01 server (wave 1), 03-02 app (wave 1, shares no file with 03-01), 03-03 entry page, skip, cutover, short walkthrough and close-out (wave 2).
- Migration 0005 (Phase 1's clean-up is 0003, Phase 2's is 0004). Tag `pre-phase3` is made by the build. The dev database is expected at version 4 before the Phase 3 cutover.
- The developer's three answers (2026-10-09), all option A: movie and book days log anything lately; six fixed reactions; edit and delete allowed, a deleted post still counts.
- Flags to carry: the photo limit is 50 MB, the same as profile photos (the developer's pick of 2026-10-09; the earlier notes said 5 MB, and the walkthrough still tries a full-size photo); entry photo links are public until Phase 5; unused, reset and test photos stay in storage; entry uploads have never run against a real bucket and the real key will need delete rights; four real dev accounts (`smoke-test@example.com`, `ava.stone.1004@example.com`, `test1@gmail.com`, `test4@gmail.com`) must stay untouched.
- Hand edits still to apply: sections A and B above (required), section C (optional).

## E. Already done, do not repeat

- REQUIREMENTS.md ENTRY-04 (the free skip), the ROADMAP.md Phase 3 'Resolved' note and Phase 3 criterion 4: already carry the free-skip wording (Phase 2 hand edits 3 and 5).
- The PROJECT.md Key Decisions row about the free skip and weekly goal, and the Apple and Google removal row: already in.
- No edit is needed for answer 1 (no requirement change), ENTRY-02 or ENTRY-03.
