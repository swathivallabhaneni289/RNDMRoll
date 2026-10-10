# Phase 4 hand edits (PHASE 4, NOTHING IS BUILT)

**NOT APPLIED.** Written 2026-10-10 by a background writer for the lead, and RE-VERIFIED the same evening at 21:50 IST after the other window's commits `b7f799a`, `0557d19` and `a491fc9` (Phase 2.1 Settings inserted, SET-01 to SET-04, Phase 3 revision 3). Every Find string below was checked against the live file at 21:50 IST and occurs exactly once, unless the item says otherwise; none of the earlier Find strings moved. What did change: the Key Decisions table now ends with the Phase 2.1 row, the requirement coverage lines are stale (see A3), the ROADMAP Overview, progress table and execution order do not mention Phase 2.1 yet (see A0), and the migration numbers shifted by one.

These are the exact edits to SHARED documents that follow from the Phase 4 decisions (`04-CONTEXT.md`). The Phase 4 documents are complete without them, but the shared documents will disagree with Phase 4 until they are applied.

How to use:
- Apply by hand with explicit paths, AFTER the other window has committed its own edits to `.planning/` and after Phase 3 is closed (the edits in A are required before Phase 4 is built; B and C change nothing that Phase 2, 2.1 or 3 builds, so they can go in earlier). Never apply them inside the other window's working session: a GSD commit stages all of `.planning/`.
- Each edit gives the file, the text to find and the text to put in its place. Find strings are short and distinctive; if one no longer matches, search for its first words. Where a find string has to quote existing text that holds the words "roll" or "rolled", that is a quote of old text, not new wording.
- Order across the three new hand-edit files: 04 first, then 05, then 06. The requirement counts and the progress table assume that order, and each later file says what its Find is if an earlier one was applied.
- Section A0 repairs gaps that commit `0557d19` left; skip an item there if the other window has already fixed it. Section A is required. Section B is notes on the Phase 2 contract. Section C is small optional notes. Section D is a list of facts for STATE.md and HANDOFF.json, which belong to the other window. Section E lists what is already done.

## A0. Gaps left by commit 0557d19 (Phase 2.1 inserted)

**A0a. `.planning/ROADMAP.md`, the Overview: five phases becomes six and Settings is named.** Checked 21:50. (Phase 6's hand edits take it from six to seven.)

Find:
```
in five phases:
```
Replace with:
```
in six phases:
```

Find:
```
then the once-daily customizable spin-wheel roll, then the structured photo+title+rating+reaction entry
```
Replace with:
```
then the once-daily customizable spin-wheel roll, then settings (username, password, a public or private account and the bio on the profile page), then the structured photo+title+rating+reaction entry
```

**A0b. `.planning/ROADMAP.md`, the progress table: a Phase 2.1 row (and the stale Phase 2 count).** Checked 21:50.

Find:
```
| 2. Daily Roll | 0/TBD | Not started | - |
```
Replace with:
```
| 2. Daily Roll | 0/3 | Not started | - |
| 2.1 Settings (INSERTED) | 0/2 | Not started | - |
```

**A0c. `.planning/ROADMAP.md`, the execution order line.** Checked 21:50. The line holds arrow characters, which this ASCII file does not retype: find the line that begins 'Phases execute in numeric order:' (under 'Execution Order') and put '2.1' after the 2 in its list, so that it reads 1, 2, 2.1, 3, 4, 5 with the same arrows.

## A. Shared project documents (required)

**A1. `.planning/REQUIREMENTS.md`, DIARY-01** (answer 4, pictures 07 and 12). Checked 21:50.

Find:
```
forming a personal taste history
```
Replace with:
```
forming a personal taste history. Skipped days appear in it as quiet rows, not a silent gap, and any row opens the entry it shows
```

**A2. `.planning/REQUIREMENTS.md`, DIARY-02 and the new DIARY-03.** Checked 21:50. (The gear and the bio come from Phase 2.1: SET-04 is the bio in place and Settings holds the rest, so DIARY-03 only names them.)

Find:
```
- [ ] **DIARY-02**: User can view their diary filtered by category
```
Replace with:
```
- [ ] **DIARY-02**: User can view their diary filtered by category, one category at a time, with a choice that shows everything again (renaming or removing a category never splits the diary into two filters)
- [ ] **DIARY-03**: User can open a profile page that shows their photo, name and username with an Edit button and the Settings gear (Phase 2.1); their bio, edited in place (SET-04); how many days they have spun, their week streak and how many friends they have; their average rating for each category; and their last three posts, with a link to the full diary
```
(ACCT-03, "User can view their own profile", stays as it is: it is complete in Phase 1 and the new page is DIARY-03.)

**A3. `.planning/REQUIREMENTS.md`, traceability and coverage.** Checked 21:50. The live coverage lines say 17, but the list has held 21 requirements since commit `0557d19` added SET-01 to SET-04 (the lines were not updated). These edits therefore go from the stale 17 straight to 22 (21 plus DIARY-03). If the other window has corrected the lines to 21 meanwhile, the Find reads 21 and the replacement is still 22.

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
- v1 requirements: 22 total
```

Find:
```
- Mapped to phases: 17
```
Replace with:
```
- Mapped to phases: 22
```

**A4. `.planning/ROADMAP.md`, the phase list line for Phase 4.** Checked 21:50.

Find:
```
- [ ] **Phase 4: Personal Diary** - Users browse a chronological, filterable log of everything they've rated
```
Replace with:
```
- [ ] **Phase 4: Personal Diary** - Users browse a chronological, filterable log of everything they've rated and see a profile page that sums it up
```

**A5. `.planning/ROADMAP.md`, Phase 4: goal, dependencies, requirements, criteria, plan list.** Checked 21:50. (The other phases also have 'Plans: TBD' lines, so the criterion stays in the find text.)

Find (goal):
```
**Goal**: Users can browse the chronological taste diary that their daily entries build up
```
Replace with:
```
**Goal**: Users can browse the chronological taste diary that their daily entries build up, and see a profile page that sums it up
```

Find (dependencies; it mounts Phase 2.1's gear and bio, so it needs that phase too):
```
**Depends on**: Phase 3
```
Replace with:
```
**Depends on**: Phase 2.1, Phase 3
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
  4. User can see a profile page with their spins, week streak, friends, average rating per category and last three posts, edit their bio in place, reach Settings and the diary from it, and open the edit form with Edit

**Plans**: 0/2 plans executed

Plans:
**Wave 1**

- [ ] 04-01-PLAN.md: The diary server: the list endpoint with paging and the category filter, and the profile summary endpoint (no migration expected; 0007 only if an index is needed)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 04-02-PLAN.md: The diary app: the Diary tab, the Profile page with the Settings gear, the bio in place and Edit, the bar of four, the entry pages in three stacks, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes
```
(The two plan files exist; this wave split and these titles were checked against their front matter and `<objective>` at 21:50. Titles can still be reworded: copy them from each plan's `<objective>` when applying.)

**A6. `.planning/ROADMAP.md`, the progress table.** Checked 21:50.

Find:
```
| 4. Personal Diary | 0/TBD | Not started | - |
```
Replace with:
```
| 4. Personal Diary | 0/2 | Not started | - |
```

**A7. `.planning/PROJECT.md`, Key Decisions: one new row.** Checked 21:50. Add this line directly after the LAST row of the Key Decisions table (today the row that begins '| 2026-10-10: Settings, public or private, several photos (developer, voice).', which `0557d19` added after the Phase 3 row; if a later row exists, after that one), keeping the blank line and the footer after the table as they are. The Outcome cell says just Pending.
```
| 2026-10-10: Phase 4 diary and profile. (1) The Diary is a list, newest first, with a row of category chips on top; a row shows a small photo (the cover), the date and category, the title, the stars with the number and the author's own note as one italic line; skipped days are quiet rows under All only. No photo grid, no stats header, no month headings. (2) The Diary stays its own tab: the bar is Home, Diary, Spin, Profile until Board arrives in Phase 5, then Home, Diary, Spin, Board, Profile. (3) The Profile page is a view page: photo, name, @username and Edit; Spins, Week streak and Friends; Your tastes with the average rating per category; Recent activity, the last three posts, with See diary. Edit opens the existing form. It also mounts Phase 2.1's Settings gear at its top right and the bio edited in place under the @username, which the picture does not draw (the developer asked for both in words). Recent activity lives there, not on Today | Developer, 2026-10-10: their own pictures 07 (Diary) and 12 (Profile) and their typed answers 4 and 8. Claude had suggested folding the Diary into Profile; the pictures kept it as a tab. Safe defaults told with them: 20 rows a page, "Late" shown after the category on a late post, Spins counts days spun, Week streak counts weeks in a row that met the weekly goal, each taste opens the Diary on that category, Log out is in Settings and stays at the bottom of the edit form, the Friends number is 0 until Phase 5. Details: `.planning/phases/04-personal-diary/04-CONTEXT.md` | Pending |
```

**A8. `.planning/PROJECT.md`, the moodboard bottom-navigation line (two small edits).** Checked 21:50. This also fixes the old word for the middle button. 05-HAND-EDITS refers back to this item.

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

**A9. `.planning/PROJECT.md`, the Active list and the requirement count.** Checked 21:50. (The count goes from the stale 17 across 5 phases to 22 across 6 phases: 21 requirements since Settings, plus DIARY-03, across Phases 1, 2, 2.1, 3, 4 and 5.)

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
(22 v1 requirements across 6 phases)
```

**A10. `.planning/PROJECT.md`, Context: one new bullet (optional but recommended).** Checked 21:50. The numbered screen pictures are now the layout source for Phases 2 to 6 and no shared document says so.

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

**B1. Section 8, the paragraph about the five pictured tabs.** Checked 21:50 (revision 6, committed in `b7f799a`).

Find:
```
Phase 2 and 3 build the three above; Diary arrives in Phase 4 and Board in Phase 5, and the middle stays the Spin button.
```
Replace with:
```
Phase 2 and 3 build the three above; Diary arrives in Phase 4 (Ionicons `document-text-outline`, giving Home, Diary, Spin, Profile with Spin third of four) and Board in Phase 5 (`people-outline`, giving the five of the pictures with Spin in the middle), and the middle stays the Spin button.
```

**B2. Section 9, a closing note after the first-run bullets.** Checked 21:50 (revision 6). It also records what Phase 2.1 does to the same page.

Find:
```
- All of its strings are Phase 1's. Phase 2 adds none.
```
Replace with:
```
- All of its strings are Phase 1's. Phase 2 adds none.

**Phase 2.1 and Phase 4 change the edit-mode page (note added 2026-10-10; nothing changes in Phase 2).** Phase 2.1 adds a Settings gear at the right end of this page's header row and a Settings page behind it (Edit profile, Username, Password, Private account, Log out). Phase 4 then replaces the edit-mode page in the tab with a view page (04-UI-SPEC: photo, name, @username and Edit, the same gear at its top right, the bio edited in place under the @username; Spins, Week streak, Friends; tastes; recent activity). The view page's Edit button opens the form described above as a pushed page with a Back control. That page has one bottom button, "Save changes" (grey until something changed), and no "Continue"; after a save it goes back to the view page. So changes 1, 2 and 4 above (no Back, the "Saved." line and Continue to Today) apply to Phase 2 only. Log out stays at the bottom of the edit page (and is a row in Settings). The first-run page is untouched.
```

## C. Other documents (small optional notes)

**C1. `.planning/phases/03-daily-entry/03-UI-SPEC.md`, section 9: where the Diary's thin routes live.** Checked 21:50 (revision 3). Phase 3's paper already says the Diary passes `from` = 'Diary'; this adds only where the routes are. Apply when Phase 3 is closed.

Find:
```
The Phase 4 diary opens the same page from its own tab (passing `from` = 'Diary'), so the page is a component (`EntryView`) with a thin route file.
```
Replace with:
```
The Phase 4 diary opens the same page from its own tab (passing `from` = 'Diary'), so the page is a component (`EntryView`) with a thin route file. Phase 4 adds more thin route files inside the Diary stack (`app/(app)/diary/entry.tsx` and `edit-entry.tsx`, 04-CONTEXT), so a push from the Diary tab never switches tabs; the page itself does not change.
```

**C2. `.planning/phases/01-foundation-accounts/01-CONTEXT.md`, D-07 (a pointer, optional).** Checked 21:50. Phase 1 is closed; D-07 stays true for Phase 1 and will mislead a later reader.

Find:
```
Phase 1 profile screen shows identity fields only: avatar, name, username, bio.
```
Replace with:
```
Phase 1 profile screen shows identity fields only: avatar, name, username, bio. (Still true for Phase 1. On 2026-10-10 Phase 2.1 added Settings and the bio edited in place, and the developer's picture 12 added a view page with spins, week streak, friends, tastes and recent activity, built in Phase 4: 04-CONTEXT D-03.)
```

## D. Facts for STATE.md and HANDOFF.json (the other window's files, not edited here)

- Phase 4 (Personal Diary and the Profile page) is written on paper and waits for the developer's separate 'go', which can only follow Phase 3 being built, walked and closed, and Phase 2.1 being built (it mounts that phase's `SettingsGear` and `BioInline`). Documents: `.planning/phases/04-personal-diary/` (04-CONTEXT, 04-UI-SPEC, 04-VALIDATION, 04-01-PLAN, 04-02-PLAN, 04-HAND-EDITS). Nothing is built.
- The developer's answers (2026-10-10): question 4 is their own picture 07 (list, chips, no grid switch); question 8 puts Recent activity on the Profile page (picture 12), not on Today. They also asked whether to keep the Diary; their pictures keep it as a tab. The record is `~/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md`.
- STATE.md Pending Todos may still say "Ask the Phase 4 and Phase 5 design questions as ONE message ..." and `.continue-here.md` may still say to ask them when the developer says "phase 4". Both are stale: the questions were asked and answered on 2026-10-10.
- STATE.md and REQUIREMENTS.md: the coverage lines say 17 requirements although the list holds 21 since `0557d19`; A3 above fixes it to 22.
- Plans: 04-01 server (wave 1: the `GET /v1/diary` list and `GET /v1/me/summary`), 04-02 app (wave 2: Diary tab, Profile view page with the gear and the bio in place, the bar of four, the entry pages in three stacks, close-out). No migration is expected. If one is needed (an index only) it is 0007: Phase 1's clean-up is 0003, Phase 2's 0004, Phase 2.1's 0005 (`account_visibility`), Phase 3's 0006. Phases 5 and 6 then shift by one more (05-HAND-EDITS and 06-HAND-EDITS say how). The 04-01 and 04-02 plan files still say 0006 in places and need that one-number fix.
- The bar in Phase 4 is Home, Diary, Spin, Profile (Spin third of four) until Phase 5 adds Board. `app/(app)/_layout.tsx` is a launch file: one cold launch is checked after it changes.
- Flags to carry: the Friends number is plain text and 0 until Phase 5; the Profile page replaces the form that the Profile tab showed directly in Phase 2, with Phase 2.1's gear added (Log out is a Settings row and also stays at the bottom of the edit page); the diary reads `entries` (Phase 3, which now carries one to ten photos with the first as the cover) and the spins' skipped results.
- Hand edits still to apply: sections A0 and A (required), B and C (optional).

## E. Already done, do not repeat

- REQUIREMENTS.md ENTRY-01 (one of six reactions) and ENTRY-04 (the skip is visible in the diary, DIARY-01), the ROADMAP Phase 3 plan list and criteria, and the PROJECT.md Phase 3 row: already in (Phase 3 hand edits, applied 2026-10-10). SET-01 to SET-04, the ROADMAP Phase 2.1 entry and the PROJECT.md Settings row: already in (`0557d19`).
- 02-UI-SPEC section 8 already says the pictures show five tabs and that Diary arrives in Phase 4 (B1 only adds the icon and the order).
- Phase 3's paper already gives the entry page a `from` value for its Back label ('Today' now, 'Diary' from Phase 4) and says the Diary opens the same page.
- Phase 2.1's CONTEXT already tells the lead that the Phase 4 Profile page mounts `SettingsGear` and `BioInline`; 04-CONTEXT says so too.
- PHASE4-QUESTIONS.md's "Hand edits that follow" are superseded by this list. In particular its ROADMAP criterion "as a list or a photo grid" is NOT used: the picture has no grid.
