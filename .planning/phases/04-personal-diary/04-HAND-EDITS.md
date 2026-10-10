# Phase 4 hand edits (PHASE 4, NOTHING IS BUILT)

**NOT APPLIED.** Written 2026-10-10 by a background writer for the lead. Every Find string below was checked against the live file on 2026-10-10 between 16:12 and 16:15 IST and occurs exactly once, unless the item says otherwise. The other window was still editing the Phase 2 UI spec (revision 6) and the Phase 3 documents at that time, so re-read each file just before editing.

These are the exact edits to SHARED documents that follow from the Phase 4 decisions (`04-CONTEXT.md`). The Phase 4 documents are complete without them, but the shared documents will disagree with Phase 4 until they are applied.

How to use:
- Apply by hand with explicit paths, AFTER the other window has committed its own edits to `.planning/` and after Phase 3 is closed (the edits in A are required before Phase 4 is built; B and C change nothing that Phase 2 or Phase 3 builds, so they can go in earlier). Never apply them inside the other window's working session: a GSD commit stages all of `.planning/`.
- Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words. Where a find string has to quote existing text that holds the words "roll" or "rolled", that is a quote of old text, not new wording.
- Order across the three new hand-edit files: 04 first, then 05, then 06. The requirement counts and the progress table assume that order, and each later file says what its Find is if an earlier one was applied.
- Section A is required. Section B is notes on the Phase 2 contract. Section C is small optional notes. Section D is a list of facts for STATE.md and HANDOFF.json, which belong to the other window. Section E lists what is already done.

## A. Shared project documents (required)

**A1. `.planning/REQUIREMENTS.md`, DIARY-01** (answer 4, pictures 07 and 12). Checked 16:15.

Find:
```
forming a personal taste history
```
Replace with:
```
forming a personal taste history. Skipped days appear in it as quiet rows, not a silent gap, and any row opens the entry it shows
```

**A2. `.planning/REQUIREMENTS.md`, DIARY-02.** Checked 16:15.

Find:
```
- [ ] **DIARY-02**: User can view their diary filtered by category
```
Replace with:
```
- [ ] **DIARY-02**: User can view their diary filtered by category, one category at a time, with a choice that shows everything again (renaming or removing a category never splits the diary into two filters)
- [ ] **DIARY-03**: User can open a profile page that shows their photo, name and username with an Edit button; how many days they have spun, their week streak and how many friends they have; their average rating for each category; and their last three posts, with a link to the full diary
```
(ACCT-03, "User can view their own profile", stays as it is: it is complete in Phase 1 and the new page is DIARY-03.)

**A3. `.planning/REQUIREMENTS.md`, traceability and coverage.** Checked 16:15.

Find:
```
| DIARY-02 | Phase 4 | Pending |
```
Replace with:
```
| DIARY-02 | Phase 4 | Pending |
| DIARY-03 | Phase 4 | Pending |
```

Find:
```
- v1 requirements: 17 total
```
Replace with:
```
- v1 requirements: 18 total
```

Find:
```
- Mapped to phases: 17
```
Replace with:
```
- Mapped to phases: 18
```

**A4. `.planning/ROADMAP.md`, the phase list line for Phase 4.** Checked 16:15.

Find:
```
- [ ] **Phase 4: Personal Diary** - Users browse a chronological, filterable log of everything they've rated
```
Replace with:
```
- [ ] **Phase 4: Personal Diary** - Users browse a chronological, filterable log of everything they've rated and see a profile page that sums it up
```

**A5. `.planning/ROADMAP.md`, Phase 4: goal, requirements, criteria, plan list.** Checked 16:15. (The other phases also have 'Plans: TBD' lines, so the criterion stays in the find text.)

Find (goal):
```
**Goal**: Users can browse the chronological taste diary that their daily entries build up
```
Replace with:
```
**Goal**: Users can browse the chronological taste diary that their daily entries build up, and see a profile page that sums it up
```

Find (requirements; this line occurs once, in Phase 4):
```
**Requirements**: DIARY-01, DIARY-02
```
Replace with:
```
**Requirements**: DIARY-01, DIARY-02, DIARY-03
```

Find (the end of criterion 2, the plans line and the line after it):
```
  2. User can filter their diary by category

**Plans**: TBD
**UI hint**: yes
```
Replace with:
```
  2. User can filter their diary by category
  3. User can open any entry from the diary, and skipped days show as quiet rows
  4. User can see a profile page with their spins, week streak, friends, average rating per category and last three posts, reach the diary from it, and open the edit form with Edit

**Plans**: 0/2 plans executed

Plans:
**Wave 1**

- [ ] 04-01-PLAN.md: The diary server: the list endpoint with paging and the category filter, and the profile summary endpoint
- [ ] 04-02-PLAN.md: The diary app: the Diary tab, the Profile page with Edit, the bar change, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes
```
(The plan titles and the wave split are PROVISIONAL: the plan files did not exist when this was written. When applying, copy each title from the plan file's `<objective>` and each wave from its front matter.)

**A6. `.planning/ROADMAP.md`, the progress table.** Checked 16:15.

Find:
```
| 4. Personal Diary | 0/TBD | Not started | - |
```
Replace with:
```
| 4. Personal Diary | 0/2 | Not started | - |
```

**A7. `.planning/PROJECT.md`, Key Decisions: one new row.** Checked 16:15. Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| 2026-10-09: Phase 3 entry rules.'; if a later row exists, after that one), keeping the blank line and the footer after the table as they are. The Outcome cell says just Pending.
```
| 2026-10-10: Phase 4 diary and profile. (1) The Diary is a list, newest first, with a row of category chips on top; a row shows a small photo, the date and category, the title, the stars with the number and the author's own note as one italic line; skipped days are quiet rows under All only. No photo grid, no stats header, no month headings. (2) The Diary stays its own tab: the bar is Home, Diary, Spin, Profile until Board arrives in Phase 5, then Home, Diary, Spin, Board, Profile. (3) The Profile page is a view page: photo, name, @username and Edit; Spins, Week streak and Friends; Your tastes with the average rating per category; Recent activity, the last three posts, with See diary. Edit opens the existing form. Recent activity lives there, not on Today | Developer, 2026-10-10: their own pictures 07 (Diary) and 12 (Profile) and their typed answers 4 and 8. Claude had suggested folding the Diary into Profile; the pictures kept it as a tab. Safe defaults told with them: 20 rows a page, "Late" shown after the category on a late post, Spins counts days spun, Week streak counts weeks in a row that met the weekly goal, each taste opens the Diary on that category, Log out stays on the Edit page, the Friends number is 0 until Phase 5. Details: `.planning/phases/04-personal-diary/04-CONTEXT.md` | Pending |
```

**A8. `.planning/PROJECT.md`, the moodboard bottom-navigation line (two small edits).** Checked 16:15. This also fixes the old word for the middle button. 05-HAND-EDITS refers back to this item.

Find:
```
5 tabs (home, add/log, roll
```
Replace with:
```
5 tabs (home, diary, spin
```

Find:
```
(likely Phase 5, once the friend board exists).
```
Replace with:
```
(Phase 2 introduces it with Home, Spin and Profile; Diary arrives in Phase 4 and Board in Phase 5; decided 2026-10-10 from the developer's pictures: Home, Diary, Spin in the middle, Board, Profile).
```

**A9. `.planning/PROJECT.md`, the Active list and the requirement count.** Checked 16:15.

Find:
```
- [ ] Users build a personal chronological diary of everything they've rated
```
Replace with:
```
- [ ] Users build a personal chronological diary of everything they've rated, and see a profile page that sums it up
```

Find:
```
(17 v1 requirements across 5 phases)
```
Replace with:
```
(18 v1 requirements across 5 phases)
```

**A10. `.planning/PROJECT.md`, Context: one new bullet (optional but recommended).** Checked 16:15. The numbered screen pictures are now the layout source for Phases 2 to 6 and no shared document says so.

Find:
```
## Constraints
```
Replace with:
```
- **Screen pictures (2026-10-10):** the developer sent numbered screen pictures: 1a Today, 02 after the spin, 05 Today skipped, 06 Entry page, 07 Diary, 08 to 10 Board (locked, unlocked, a post with reactions and comments), 11 Notifications (the bell), 12 Profile, and 3a Home. They are saved in `docs/reference/` and are the layout source for Phases 2 to 5, and they win over older paper rules where they differ (feedback of 2026-10-10: "very similar" to the picture). Each UI spec lists its overrides in "Choices made here that you may want to change". Their word on the middle button is "Roll"; the app says "Spin" everywhere.

## Constraints
```

## B. `.planning/phases/02-daily-roll/02-UI-SPEC.md` (notes on the Phase 2 contract; nothing Phase 2 builds changes)

**B1. Section 8, the paragraph about the five pictured tabs.** Checked 16:15 (revision 6).

Find:
```
Phase 2 and 3 build the three above; Diary arrives in Phase 4 and Board in Phase 5, and the middle stays the Spin button.
```
Replace with:
```
Phase 2 and 3 build the three above; Diary arrives in Phase 4 (Ionicons `document-text-outline`, giving Home, Diary, Spin, Profile with Spin third of four) and Board in Phase 5 (`people-outline`, giving the five of the pictures with Spin in the middle), and the middle stays the Spin button.
```

**B2. Section 9, a closing note after the first-run bullets.** Checked 16:15 (revision 6).

Find:
```
- All of its strings are Phase 1's. Phase 2 adds none.
```
Replace with:
```
- All of its strings are Phase 1's. Phase 2 adds none.

**Phase 4 replaces the edit-mode page (note added 2026-10-10; nothing changes in Phase 2).** After the first run the Profile tab shows a view page (04-UI-SPEC: photo, name, @username and Edit; Spins, Week streak, Friends; tastes; recent activity). Its Edit button opens the form described above as a pushed page with a Back control. That page has one bottom button, "Save changes" (grey until something changed), and no "Continue"; after a save it goes back to the view page. So changes 1, 2 and 4 above (no Back, the "Saved." line and Continue to Today) apply to Phase 2 only. Log out stays at the bottom of the edit page. The first-run page is untouched.
```

## C. Other documents (small optional notes)

**C1. `.planning/phases/03-daily-entry/03-UI-SPEC.md`, section 9: where the Diary's thin routes live.** Checked 16:15. Phase 3's paper already says the Diary passes `from` = 'Diary'; this adds only where the routes are. Apply when Phase 3 is closed.

Find:
```
The Phase 4 diary opens the same page from its own tab (passing `from` = 'Diary'), so the page is a component (`EntryView`) with a thin route file.
```
Replace with:
```
The Phase 4 diary opens the same page from its own tab (passing `from` = 'Diary'), so the page is a component (`EntryView`) with a thin route file. Phase 4 adds two more thin route files inside the Diary stack (`app/(app)/diary/entry.tsx` and `edit-entry.tsx`, 04-CONTEXT), so a push from the Diary tab never switches tabs; the page itself does not change.
```

**C2. `.planning/phases/01-foundation-accounts/01-CONTEXT.md`, D-07 (a pointer, optional).** Checked 16:15. Phase 1 is closed; D-07 stays true for Phase 1 and will mislead a later reader.

Find:
```
Phase 1 profile screen shows identity fields only: avatar, name, username, bio.
```
Replace with:
```
Phase 1 profile screen shows identity fields only: avatar, name, username, bio. (Still true for Phase 1. On 2026-10-10 the developer's picture 12 added a view page with spins, week streak, friends, tastes and recent activity, built in Phase 4: 04-CONTEXT D-03.)
```

## D. Facts for STATE.md and HANDOFF.json (the other window's files, not edited here)

- Phase 4 (Personal Diary and the Profile page) is written on paper and waits for the developer's separate 'go', which can only follow Phase 3 being built, walked and closed. Documents: `.planning/phases/04-personal-diary/` (04-CONTEXT, 04-UI-SPEC, 04-VALIDATION, 04-01-PLAN, 04-02-PLAN, 04-HAND-EDITS; the plan and spec files were still being written when this list was made). Nothing is built.
- The developer's answers (2026-10-10): question 4 is their own picture 07 (list, chips, no grid switch); question 8 puts Recent activity on the Profile page (picture 12), not on Today. They also asked whether to keep the Diary; their pictures keep it as a tab. The record is `~/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md`.
- STATE.md Pending Todos still says "Ask the Phase 4 and Phase 5 design questions as ONE message ..." and `.continue-here.md` says to ask them when the developer says "phase 4". Both are stale: the questions were asked and answered on 2026-10-10.
- Plans: 04-01 server (the `GET /v1/diary` list and `GET /v1/me/summary`), 04-02 app (Diary tab, Profile view page, the bar). No migration is expected; if one is needed it is 0006 (Phase 1 clean-up 0003, Phase 2 0004, Phase 3 0005).
- The bar in Phase 4 is Home, Diary, Spin, Profile (Spin third of four) until Phase 5 adds Board. `app/(app)/_layout.tsx` is a launch file: one cold launch is checked after it changes.
- Flags to carry: the Friends number is plain text and 0 until Phase 5; the Profile page replaces the form that the Profile tab showed directly in Phase 2 (Log out moves one tap deeper, to the edit page); the diary reads `entries` (Phase 3) and the spins' skipped results.
- Hand edits still to apply: section A (required), B and C (optional).

## E. Already done, do not repeat

- REQUIREMENTS.md ENTRY-01 (one of six reactions) and ENTRY-04 (the skip is visible in the diary, DIARY-01), the ROADMAP Phase 3 plan list and criteria, and the PROJECT.md Phase 3 row: already in (Phase 3 hand edits, applied 2026-10-10).
- 02-UI-SPEC section 8 already says the pictures show five tabs and that Diary arrives in Phase 4 (B1 only adds the icon and the order).
- Phase 3's paper already gives the entry page a `from` value for its Back label ('Today' now, 'Diary' from Phase 4) and says the Diary opens the same page.
- PHASE4-QUESTIONS.md's "Hand edits that follow" are superseded by this list. In particular its ROADMAP criterion "as a list or a photo grid" is NOT used: the picture has no grid.
