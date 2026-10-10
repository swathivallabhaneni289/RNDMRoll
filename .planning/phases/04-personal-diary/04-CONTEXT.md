# Phase 4: Personal Diary (and the Profile page) - Context

**Gathered:** 2026-10-10 (the developer's typed answers and their pictures 07 and 12, the same day)
**Written:** 2026-10-10, on paper, against the Phase 2 documents (revision 5) and the Phase 3 documents. Phase 2 and Phase 3 are NOT built. Every Phase 2 and Phase 3 file, table and function named here is that PLAN's name. At build time the built code wins; each plan's start_rules say how to re-check.
**Status:** Ready for the developer to read. NOTHING IS BUILT. PHASE 4.
**Approval word for a Phase 4 plan:** "go" ("start" and "approved" belong to Phase 1). Phase 2 and Phase 3 each have their own "go". Phase 4 needs a third, separate "go", said after Phase 3 is built, walked on the phone and closed.

Wording: folder and requirement ids keep the old name (DIARY-01, DIARY-02). Anything the user sees says "spin", never "roll", even where the developer's pictures say "Roll" or "Rolls" (see the differences list at the end of the UI spec).

Pictures (the layout source; saved in `docs/reference/`):
- `screens-07-diary-08-board-locked-2026-10-10.png`: screen 07 Diary (list, category chips, bar of five).
- `screens-11-notifications-12-profile-2026-10-10.png`: screen 12 Profile (header, three numbers, tastes, recent activity).
- `screens-03a-home-03b-diary-2026-10-10.png`: 3a Home and 3b Diary (3b is the same screen as 07).
Read them before the UI spec.

**Visual reference (binding, developer 2026-10-10: "use exactly the same visuals ... as a reference to build this project"):** the developer's pictures are the visual reference for this phase. Build to match them: what is on each screen, its order, spacing, type, shapes (the pills and circles included) and states. At every screen's device check the phone is compared with its picture side by side (a simulator screenshot opened next to the picture); any difference that is not in the UI spec's "Choices made here that you may want to change" list is a defect to fix, not a choice. Where this paper and a picture disagree, the picture wins and the paper is corrected.

<domain>
## Phase Boundary

After posting, the person can look back. The Diary tab lists everything they posted, newest first, with skipped days as quiet rows, and filters it by category. The Profile page becomes the page in the developer's picture 12: who they are, three numbers, their average rating per category, and their last three posts with a link to the Diary. Both open the entry page that Phase 3 builds.

What the user gets: a Diary tab (list, category chips, paging, empty states), a Profile page (view) with an Edit button that opens the existing edit form, and the entry page reachable from both. The bottom bar gains one item: Home, Diary, Spin, Profile.

Out of scope:
- Friends, the Board, reactions and comments from other people, the Friends number being anything but 0 (Phase 5).
- Public and private accounts, the person page for someone else, Global (Phase 6).
- A photo grid, a stats header on the Diary, search, month headings, sharing a diary (the picture has none of them).
- Anything that creates or changes an entry. Phase 3 owns post, edit and delete. Phase 4 only reads, and opens Phase 3's pages.
- Real storage accounts, the domain, Terms and Privacy (the developer's launch list).

Phase 4 is BUILT only after Phase 3 is built and closed, and after these documents are in the main tree with the edits in `04-HAND-EDITS.md`. Phase 4 needs ALL of Phase 3: the `entries` table, the entry JSON, the entry page component and its Edit and Delete flows, the skip result on spins.

</domain>

<decisions>
## Implementation Decisions (answered 2026-10-10)

### What the developer decided
- **D-01 Diary layout (question 4, their own picture 07):** a list, one row per entry, with a row of category chips on top. No photo-grid switch, because the picture has none (this is option B, "list only"). Row, left to right: a small square photo; then a small label "Oct 8 . Meal" (date, a dot, the category as written that day), the title in the serif, the stars with the number ("4.5"), and the author's own note as one italic line when there is one. Thin rules between rows.
- **D-02 The Diary stays its own tab.** Pictures 07, 08, 09 and 12 all draw five slots with Diary in them, and the developer's own note says "the bottom bar stays at five". Claude's suggestion of 2026-10-10 to fold the Diary into Profile was not taken (the pictures win). One word undoes it on paper: "drop".
- **D-03 The Profile page follows picture 12.** Photo, name, @username and an Edit button; three numbers (Spins, Week streak, Friends); "Your tastes, avg rating"; "Recent activity" with a "See diary" link.
- **D-04 Recent activity lives on the Profile page, not on Today** (answer 8 of 2026-10-10: "it should be placed in the profile page", with picture 12). It shows the person's OWN last three posts. There is no friends card on Today (Phase 5 changes nothing on Today).
- **D-05 Reactions, edit and delete (answers 2 and 3 of 2026-10-10, restated):** Phase 3 decided them; the diary only shows the result.

### Carried over (restated so nothing is asked twice)
- **D-06 Skipped days** are visible in the diary as quiet rows, not a silent gap (Phase 2 decision 2). A missed day shows nothing.
- **D-07 A rename or removal of a category never rewrites old entries.** They keep the name they had when written. The filter works by category id, so a rename never splits the diary into two filters.
- **D-08 A repeat title** is a new dated entry (ENTRY-02). The photos are always the person's own.
- **D-09 The diary is light editorial**: Domine and Work Sans, `ink` on `dominant`, hairlines, no glass (the glass look is dropped everywhere, Phase 5 D-03).

### Claude's Discretion (safe defaults, told to the developer; change any by saying so)
Diary:
- Newest first by the spin's local date, then by spin number. A day has one row, even when it was posted late after midnight (it sits on the day of the spin).
- 20 rows a page, loaded as you scroll. Pull down to refresh. Needs a connection; with none, a plain message and "Try again".
- The chip row: "All" first, then every category that has at least one entry, in the order of the person's wheel, then removed categories (by the last name they had). One chosen at a time; the choice is kept while the app is open and starts at All again after a relaunch. A category with no entries has no chip.
- Skipped rows show under All only, as one quiet line: the date and the word "Skipped", no photo (as Phase 2 decided). They are not shown under a category chip.
- A late entry shows "Late" at the end of its label ("Oct 8 . Meal . Late"). On-time entries show nothing.
- Month headings: none (the picture has none).
- The row's note line is the author's own note (the "comment" field of the entry), one line, cut with an ellipsis. The author's reaction is NOT on the row; it is on the entry page (the picture's row has none).
- Tapping a row opens the entry page inside the Diary stack (a thin route file around Phase 3's `EntryView`), so Back returns to the list where it was. After Edit or Delete the list reloads when it comes back into focus.
- Empty diary: one plain line, "Nothing here yet. Your first post will show up here.", and a text link "Go to Today". An empty chip: "Nothing in {category} yet." No picture.
- After spinning again, the diary shows only the final category of the day (the entry belongs to it).

Profile:
- **Spins** is the number of days the person spun (one per local date, a bonus spin does not add a second). The picture says "Rolls"; it reads "Spins".
- **Week streak** is the number of weeks in a row, ending with this week or last week, in which the weekly goal was met (Phase 2's `goal_weeks` rows with `goal_met_at`). It does not drop to 0 in the middle of a week that has not met the goal yet.
- **Friends** shows the real number. It is 0 until Phase 5. Tapping it does nothing in Phase 4 (no dead control: it is drawn as plain text); Phase 5 makes it open the Friends screen.
- **Your tastes** lists every category with at least one entry, with the average of its stars to one decimal ("4.6"), in the order of the person's wheel, then removed ones. Each is a button that opens the Diary filtered to that category (added here; the picture draws them as chips that look tappable). No chip for a category with no entry. With no entries at all the whole section is left out.
- **Recent activity**: the last three posted entries (small photo, title, "MEAL . OCT 8"). Each opens the entry page. "See diary" opens the Diary tab at All. With no entries the section is left out.
- **Edit** opens Phase 1's form page in edit mode as a pushed page with Back (the file `app/(app)/profile/edit.tsx` already exists and nothing opens it). Its bottom button is "Save changes" (grey until something changed). After a save the page goes back to the Profile page. Log out stays at the bottom of that edit page, exactly as Phase 1 built it, so the Profile page itself has no Log out.
- **First run is untouched.** Right after a sign-up the Profile route still shows Phase 1's photo-and-bio page once (flag `extrasPending`, bar hidden, Continue or Skip for now end on Today). Only after that does the Profile route show the new page.
- Other people's profiles do not exist yet (Phase 6).

Bar:
- Home, Diary, Spin, Profile. Spin sits third of four, a little right of centre, until Phase 5 adds Board and puts it in the middle of five. Diary uses Ionicons `document-text-outline`, the same size and colours as Home and Profile (Phase 2 section 8). If a centred Spin matters now, say so.
- The Today week section stays display only (Phase 2 section 6 made it a plain section). It is NOT made tappable here: the pictures draw no arrow on it.

</decisions>

<api_contract>
## API contract (binding for plans 04-01 and 04-02)

Instants are RFC 3339 UTC, names snake_case, dates `YYYY-MM-DD`. `Entry` is exactly Phase 3's `Entry`:
`{"id","local_date","category":{"id","name"},"title","stars":1..10,"reaction":name|null,"comment":text|null,"photo_url","photo_source","late","created_at","updated_at"}` (never `photo_key`).

All routes are on the signed-in group (`RequireAuth`, `RequireUser`), call `u.OnboardingComplete()` first, take the user only from the token, and are rate limited with `KeyBySubject`. The app's client sends only GET, POST and PATCH.

| Route | Query | Success |
|---|---|---|
| `GET /v1/diary` | `category_id` (uuid, optional), `cursor` (opaque, optional), `limit` (1 to 50, default 20) | 200 `{"items":[Row],"next_cursor":string or null,"categories":[Chip]}` |
| `GET /v1/me/summary` | none | 200 `{"spins":int,"week_streak":int,"friends":int,"tastes":[Taste],"recent":[Entry]}` |

- `Row` is one of `{"kind":"entry","entry":Entry}` or `{"kind":"skipped","local_date":"2026-10-06","category":{"id","name"}}`. Skipped rows appear only when no `category_id` is given. Order: `local_date` descending, then spin number descending. A skipped row's category is the skipped spin's category as written that day.
- `Chip` is `{"id","name","removed":bool}`: categories that have at least one entry, in wheel order, then removed ones (the last name they had). Sent on every page (it is small); the app uses the first page's.
- `Taste` is `{"category":{"id","name","removed":bool},"average":4.6,"count":9}`: the mean of `stars` divided by 2, rounded to one decimal; `count` is the number of entries.
- `recent` holds at most 3 `Entry`, newest first.
- `friends` is the count of accepted friends: the constant 0 in Phase 4, replaced in Phase 5.
- Errors: 400 `validation_failed` with `field` `category_id` (not a uuid: `invalid`), `cursor` (`invalid`), `limit` (`invalid`); 404 is never used (a `category_id` that is not the caller's simply gives an empty list); the existing 401, 403 `profile_incomplete`, 429.
- The photo links are built the way Phase 3 builds them (from the stored key, through the entry photo store). The key never leaves the server.

</api_contract>

<canonical_refs>
## Canonical References

Read before planning or building. Paths are inside the project unless stated.

### Decisions and pictures
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/PHASE345-DECISIONS.md`: the answers of 2026-10-10 and the new Board scope (outside the project, read only).
- `docs/reference/screens-*.png` (listed at the top).
- `.planning/phases/04-personal-diary/04-HAND-EDITS.md`: the edits to shared documents.

### Project rules
- `.planning/PROJECT.md`: stack, standing design rules (no purple gradients, no emoji as icons, no em dashes, no fake counters, no AI-slop copy or photos, no over-the-top animation), launch gate. Light editorial, strict monochrome. Where a picture and a rule disagree the picture wins and the override is listed (feedback of 2026-10-10).
- `.planning/REQUIREMENTS.md` (DIARY-01, DIARY-02) and `.planning/ROADMAP.md` (Phase 4 criteria).
- `.planning/phases/02-daily-roll/` (all six files): tokens added, the bar (section 8), the Profile tab and first-run (section 9), `goal_weeks`, `spins`, `wheel_categories`, the Today response, `ConfirmSheet`.
- `.planning/phases/03-daily-entry/` (all files): the `entries` table, `Entry` JSON, `EntryView`, the Edit and Delete flows, the skip result, the reaction names (stored as names, never emoji).
- `.planning/phases/01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later): tokens, type, spacing, radius, the profile form.
- `docs/motion-interaction-direction.md` and `docs/design-brief-2026-09-16.md` (section Diary): background only.

### Code to read before planning or building
- Server: `internal/spin/` (service, week, `goal_weeks` reads), `internal/store/postgres/spin_repo.go`, `entry_repo.go` (built in Phase 3), `internal/httpapi/entry.go`, `server.go`, `cmd/api/main.go`.
- App: `app/(app)/_layout.tsx` (the Tabs layout), `app/(app)/profile/index.tsx` and `edit.tsx`, `components/profile/ProfileForm.tsx`, `components/today/*`, `components/entry/*` (Phase 3), `lib/api/entries.ts`, `lib/theme/tokens.ts`, `components/ui/*`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets (after Phases 2 and 3 are built)
- **Entry page and flows:** Phase 3's `EntryView` (a component with a thin route file), Edit entry, the Delete sheet (`ConfirmSheet`), `StarDisplay` (read-only stars), the reaction character file `lib/entry/reactions.ts` (the only file that holds emoji), `lib/api/entries.ts`.
- **Text, buttons, screen:** `AppText` (including Phase 2's `title` role), `TextButton`, `PrimaryButton`, `Screen` (with `edges`), `StatusMessage` (message and "Try again"), `PhotoPlaceholder`, `ImageReveal`, `PressScale`, `FadeInUp`.
- **Dates:** `dateLine()` in `lib/wheel/format.ts` gives 'WED, OCT 8' from a server date. Phase 4 needs the form 'Oct 8' (month name and day, no weekday) and 'OCT 8' in capitals: one new pure function in the same file, no `Date` maths on a device clock.
- **Weekly goal data:** `goal_weeks` (`user_id`, `week_start`, `goal`, `goal_met_at`, `bonus_granted`).
- **Stack nesting:** Phase 3 put the entry routes inside the Home stack. Phase 4 adds the same two as thin route files inside a Diary stack (`app/(app)/diary/`), because a push to the Home stack's route from the Diary tab would switch tabs.

### Gaps to close (nothing of these exists)
- The two read endpoints and their queries; an index if the plan shows one is needed.
- The Diary tab: chips, row, paging, empty and offline states, the entry routes inside its stack.
- The Profile page: header, three numbers, tastes, recent activity, Edit route, the first-run split.
- The bar item and its icon.

### Integration points
- **Routes:** `internal/httpapi/server.go` is the only engine builder; the new handler is a field on `Deps`, registered only when not nil; `cmd/api/main.go` and both test builders change together in one task.
- **No migration is expected.** The diary reads `entries` (indexed by its `unique (user_id, local_date)`) and `spins`; a plan may add one index in a numbered migration only if its query plan shows the need (the next free number is 0006, or whatever the built code shows).
- **Launch files:** `app/(app)/_layout.tsx` (Tabs) is a launch file and changes here, so one cold launch is checked after it changes (Phase 2 section 12). `app/index.tsx` is not touched.
- **Test traps:** never delete or edit the four real dev accounts in `rndmroll_dev` (`smoke-test@example.com`, `ava.stone.1004@example.com`, `test1@gmail.com`, `test4@gmail.com`); Go tests run only on a database whose name ends in `test`, with `-p 1`, through the guarded helpers. The test reset clears entries and posted days (Phase 3), so a throwaway account can be walked again.
- **Limits:** no new native module; never run `expo lint`; no JS test runner (a throwaway node script checks pure logic); local commits, one per task, explicit paths, never pushed.

</code_context>

<specifics>
## Specific Ideas

- Row photo: 76dp square, `radius.md` (8dp), centre-cropped from the original; a plain `secondary` panel with the `image-outline` glyph if it cannot load. The row is one button (the whole row is the tap target, at least 88dp high).
- The chips are drawn as in the picture (rounded, `fieldSoft` fill, the chosen one `ink` fill with `dominant` text). That is a pill: it is an override of the old no-pill rule and is listed in the UI spec.
- The "Week streak" figure is a count of weeks, written "4" under the words "Week streak", as in the picture.
- Stale, ignore: the "Rolls", "Roll" and "Mashed" sample strings in the pictures' mock data, and the lab's diary fixtures under `app/lab/`.

</specifics>

<deferred>
## Deferred Ideas

- A photo grid view and a grid or list switch (the picture has none).
- A stats header on the Diary (spins, streak, average, top category). The numbers live on the Profile page instead.
- Search, sharing a diary, a yearly recap (INSIGHT-01, v2).
- Month headings.
- The Friends number opening the Friends screen (Phase 5).
- Tapping the week section on Today to open the Diary (the picture has no arrow).

</deferred>

<hand_edits>
## Hand edits at merge-back

The exact find-this and replace-with text is in `04-HAND-EDITS.md`. In short: REQUIREMENTS DIARY-01 gains "Skipped days appear as quiet rows" and "opens an entry from the list"; a new requirement DIARY-03 for the Profile page (ACCT-03 stays complete); the ROADMAP Phase 4 plan list and criteria (list, filter, open an entry, the Profile page); one PROJECT.md Key Decisions row ("2026-10-10: Phase 4 diary and profile"); the Phase 2 UI spec section 8 and 9 notes (the bar gains Diary; the Profile tab shows the view page after first run). Apply them in the main tree by hand, after Phase 3 is closed, before Phase 4 is built.

</hand_edits>

---

*Phase: 04-personal-diary*
*Context gathered: 2026-10-10*
