# Phase 2: Daily Roll (the spin) - Context

**Gathered:** 2026-10-07
**Rebased:** 2026-10-09 on the final Phase 1 code (main tree commit ead0c70, branch phase-01-foundation-accounts). No developer decision changed. At merge time run `git log --oneline ead0c70..HEAD` in the main tree and re-read anything that touches a file named in these documents.
**Status:** Ready for the developer to read. NOTHING IS BUILT.
**Approval word for a Phase 2 plan:** "go" ("start" and "approved" belong to Phase 1).

Wording: folder and requirement ids keep the old name (ROLL-01 to ROLL-04). Anything the user sees says "spin", never "roll".

<domain>
## Phase Boundary

Once a day, from 8:00 PM in the user's own time zone, the user spins a wheel and gets one category. The server picks it, the app animates to it. A user can earn one bonus reroll each week the weekly goal is met and spend it to spin again the same day, and can change their own wheel (add, rename, remove, reweight). The signed-in app lands on a new Today screen with a bottom bar of Home, a middle Spin button and Profile. A brand-new account still sees Phase 1's photo-and-bio page once, before Today (see "Launch files" and "First run" below).

Out of scope:
- Entries. Phase 2 never creates one. Posting, the skip action, the late stamp and photo upload are Phase 3.
- Diary (Phase 4), friend feed, Board and friends (Phase 5).

Phase 2 is BUILT only after Phase 1 is closed and these documents are copied into the main tree by hand, with the hand edits at the end of this file.

</domain>

<decisions>
## Implementation Decisions (all answered 2026-10-07)

### Cadence, goal and window
- **D-01 Cadence:** The spin happens every day, once a day, from 8:00 PM local. Not weekly. The wheel is mostly everyday things; movie and book are small wedges (about 1 day in 5). A movie or book day may log the last one watched or read; Phase 3 settles the rule. The streak is forgiving (D-02).
- **D-02 Weekly goal (replaces the capped freeze):** A day counts when the entry is posted. Spinning alone does not count. Post on at least 4 days each week (Monday to Sunday; 4 is a starting number) to keep the streak. Skipping a day is fine. ENTRY-04 becomes a free "skip this one", still visible in the diary (not a silent gap), showing its effect on the week ("2 of 4 this week"). Each week the goal is hit, one bonus reroll is earned. Keep one at a time. An unused one carries over. This makes ROLL-02's "one extra reroll per week" literal.
- **D-03 Spin window and late:** The wheel opens at 8:00 PM local and stays open until local midnight. The spin belongs to that local date. Posting after midnight but before the next 8:00 PM is late: the feed still unlocks, with a late tag. After that the day is missed. A missed day simply does not count toward the weekly goal.
- **D-04 Secret test setting:** It moves the clock and marks pretend posted days while testing, for listed throwaway accounts only. It is off for normal use and refused outside test mode. With it, earning a reroll can be tested in Phase 2 with pretend days. The real path connects in Phase 3. No ROADMAP criterion 3 amendment is needed.

### The wheel
- **D-05 Starting wheel:** Six categories: movie, book, song, meal, place, game. Weights: meal 3, song 3, place 2, game 1, movie 1, book 1. A wheel holds 2 to 12 categories with whole-number weights. The server keeps the list. Any user can resize wedges.
- **D-06 Wedge colours:** Only ink and light, like the logo. No real colours. Six tones: charcoal, dark grey, medium grey, warm grey, off-white, slightly darker off-white. This supersedes the Phase 1 UI-SPEC rev 8 "Coordination with Phase 2" note and agrees with PROJECT.md (strict monochrome). Tones come from `lib/theme/tokens.ts` (a new grey needs a token; no raw hex in screens). Neighbours must always contrast, also at the wrap from the last wedge to the first. The label colour flips to stay readable (WCAG AA, 4.5:1).
- **D-07 Wedge content:** Category words only. No drawings on the wheel (the Welcome page keeps its drawings). ROLL-04 stays as written, with no wording change.

### Today screen and bar
- **D-08 Today screen:** It follows the developer's picture with every doodle and photo removed: no handwritten note, no sparkles, no leaf sketch, no polaroids, no random decoration. The developer wants to see the wheel spinning.
  - Kept: the small wordmark top left, the date line, the serif heading ("Your next spin is coming."), the sub-line, the six-wedge ink-and-light wheel with words only, a dark centre with a star, a pointer on top, the "opens in" countdown with "at 8:00 PM", the weekly card (seven day dots M to S, weekly goal count), the bottom bar.
  - Told, no question yet (change if the developer objects): the friends' "Recent activity" card waits for Phase 5; the bell stays out; the round picture top right is the user's own profile photo (the camera placeholder if none) and opens Profile.
  - All imagery is the developer's. Nothing is generated. Exact copy and layout go to the UI spec for approval.
- **D-09 Bottom bar:** Home, the middle Spin button, Profile. Diary and Board join in Phases 4 and 5. The middle button brings you to the wheel. No placeholder tabs for later phases.
- **D-10 Starting a spin:** Tap the wheel's round centre or the middle Spin button. No extra button under the wheel (the countdown sits there).
- **D-11 Before 8 PM:** The wheel sits still with the countdown and spins only when you spin (about 4 seconds, smooth slow-down). With Reduce Motion on there is no spinning and the result appears at once.
- **D-12 Result word size:** Big, in the largest existing text size (Display, 40/46). The type scale stays closed at 16, 14, 22 and 40.
- **D-13 Changing your wheel:** A small "Edit wheel" link on the Today screen opens a list of the user's categories. Each has minus and plus buttons for its weight, and the wheel redraws live. Add, rename and remove happen there. No dragging of wedge edges.
- **D-14 8 PM phone reminder:** Not in the first version. It needs a new phone feature, so the test app would have to be rebuilt, and no requirement asks for it. The Today countdown instead.
- **D-15 Middle Spin button style (2026-10-08, B):** a dark square with a light wheel icon, like the other buttons. Not the light button of the picture.
- **D-16 Header photo on Today (2026-10-08, A):** round, as on the Make it yours page (the one named circle exception for a tappable element).

### Claude's Discretion (safe defaults, told to the developer, no question asked)
- A reroll must change the category and is not allowed after the day is posted or skipped. Using one asks first. The notes say "Use your bonus reroll?", but "reroll" holds "roll", so the UI spec decides the final words.
- The reroll is earned the moment the goal is hit. A week hit while one is already held counts as paid and adds nothing. The first week after sign-up counts like any other. (Told to the developer on 2026-10-08 as decided unless they object; no objection.)
- The goal number (4) and the week start (Monday) are named server constants, each in one place; that is the "setting" for now, and changing either needs a server edit. The server sends the day letters.
- Travel rule: one daily spin per local date and at least 12 hours between daily spins. A person flying west can lose a day. (Told to the developer on 2026-10-08 as decided unless they object; no objection.)
- Rename changes the name on the wheel and in the Edit wheel list only. Delete archives. Past spins and entries keep the name they had when written. At least 2 live categories stay.
- The phone reports its time zone (own small table, never on users). The server computes the local date from its own clock.
- The server picks and stores the category first; the app only animates to it. Retrying returns the same spin. With no connection: a plain message and a retry.
- The result screen has no button until Phase 3.
- Under Reduce Motion the wheel still shows the winning wedge under the pointer. No confetti, flash or sound.
- Category names: up to 24 characters, no duplicates ignoring capitals. Weights are whole numbers from 1 to 10.
- A result name over 12 characters drops to the Heading size, because the type scale stays closed (D-12).
- The signed-in landing screen is Today (Home), from D-08 and D-09; the UI spec names it. Exception, first run (rebase 2026-10-09, not yet told to the developer; REBASE-DONE asks for a yes): right after a sign-up, or after an Apple or Google account is finished, the person sees Phase 1's photo-and-bio page ("Make it yours.", `ProfileForm` mode `extras`) once, and only then Today. The bottom bar is hidden on that page, because it is still part of signing up. Continue and Skip for now both end on Today.
- The Profile tab's bottom button keeps both Phase 1 states: "Continue" while nothing has changed and "Save changes" once something has. Continue now opens Today (Phase 1's temporary page "You're in." is removed). (Rebase 2026-10-09, same status as the line above.)

</decisions>

<canonical_refs>
## Canonical References

Read before planning or building. Paths are inside the copy unless stated.

### The picture and the notes (outside the copy, read only)
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/today-screen-mockup-2026-10-07.png`: the developer's Today picture. Remove every doodle, photo, the bell and the Recent activity card. "roll" becomes "spin". Wedges from the top, clockwise: book, song, game, place, meal, movie.
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/PHASE2-DECISIONS.md`: the source of D-01 to D-16.
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/parallel-phases-report-2026-10-05.md` (UPDATE block first) and `.../research-run-2026-10-05/`: unblock map, hazards, time zone and spin research.

### Project rules
- `.planning/PROJECT.md`: stack, standing design rules, launch gate. Light editorial, strict monochrome.
- `.planning/REQUIREMENTS.md` (ROLL-01 to ROLL-04) and `.planning/ROADMAP.md` (Phase 2 criteria).
- `.planning/phases/01-foundation-accounts/01-UI-SPEC.md` (revision 15 or later; the Phase 1 close-out may raise it): tokens, type, spacing, radius, elevation, and revisions 13 to 15 for the sign-up pages, the photo-and-bio page and the Profile page that Phase 2 reuses.
- `docs/motion-interaction-direction.md`: wait, spin, anticipation, reveal. No flashing, confetti, particles, glossy or 3D wheel, bounce.

### Code to read before planning
- App: `lib/theme/tokens.ts`, `lib/motion/primitives.tsx`, `components/brand/`, `app/index.tsx`, `app/_layout.tsx`, `app/(app)/` (it holds Phase 1's placeholder `home.tsx`, which Phase 2 deletes), `app/(app)/profile/index.tsx`, `components/profile/ProfileForm.tsx` (read all of it: modes signup, finish, extras, edit), `components/ui/Screen.tsx`, `lib/api/` (`client.ts`, `types.ts`, `profile.ts`), `lib/session/store.ts` (the `extrasPending` flag; read only).
- Server: `internal/httpapi/`, `internal/middleware/`, `internal/store/postgres/`, `cmd/api/main.go`, `migrations/`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets
- **Wheel drawing:** wedge paths in `BrandMark.tsx` (`M 0 0 L polar(a0) A r r 0 0 1 polar(a1) Z`) fit equal wedges only; wedges over 180 degrees (weights 10 and 1) need the large-arc flag. Angles run clockwise from the top, so the pose that puts a wedge centre `c` at the pointer is `-c`. `SpinningMark.tsx` has the layer split (rotating disc, fixed pointer), a Reanimated rotation, an ease and a target-angle formula. Reuse the shape, with about 4 seconds.
- **Gaps to close:** the Welcome wheel picks its winner on the phone with `Math.random`, hard-codes 8 sectors, has no text labels, and under Reduce Motion does not move the pointer. Today needs a wheel that takes N words and the server's winning index, and jumps to it under Reduce Motion.
- **Tokens:** ink, muted, divider, secondary and dominant give five greys; a medium grey is a new token, added inside the `color` group so the shape pin in `tokens.test-assert.ts` holds. Ink against muted is only 3.0 (use dominant text on muted). Divider, secondary and dominant sit 1.1 to 1.4 apart, so alternation carries the separation.
- **Type, radius, icons:** Display 40/46 is the largest size. Radius md (8) caps anything tappable; circles only for avatars and icon badges, so the UI spec must say how the round hub, the avatar and the middle Spin button respect this. The 24dp token (`radius.lg`) is allowed only in `components/ui/PhotoPanel.tsx`, `components/profile/BirthdayPickerField.tsx`, `components/profile/ProfileForm.tsx` and, new here, `components/ui/ConfirmSheet.tsx`; the round token (`radius.full`) only in `components/ui/IconBadge.tsx` and `components/brand/DialAvatar.tsx` (the exact scans are in plan 02-03 Task 2). Icons are Ionicons or react-native-svg, never emoji.
- **Patterns:** a handler with `Register(rg)`, a domain interface, a Postgres repo, a fake. A new error code goes in `ApiErrorCode` in `lib/api/types.ts` and, on the server, in the Phase 2 file `internal/httpapi/spin_errors.go` (`errors.go` is not edited, plan 02-01 Task 3a). `lib/api/profile.ts` models a new `lib/api/spin.ts` (not "roll": an import path would trip the word scans). The client allows only GET, POST and PATCH.

### Integration points
- **Routes:** `internal/httpapi/server.go` is the only engine builder. Routes go on the signed-in group (RequireAuth then RequireUser); one needing a finished profile also checks `OnboardingComplete()`. Never take a user id from the request.
- **Assembly points:** `Deps` and `NewServer`, `cmd/api/main.go` and both test builders (`buildIntegrationServer`, `newFullTestRig`) change together, in one task.
- **Clock:** none exists. Add an injectable per-account clock for the spin slice only (never tokens, rate limiter or refresh expiry), an `APP_ENV` setting, and a secret test setting that makes `Load()` fail outside test mode.
- **Time maths:** the server owns the 8 PM constant, local date, window instants and the open time the countdown shows. Use civil dates, never +24 hours, and handle zones whose midnight is skipped or repeated. Each spin stores local date, zone name and UTC instants. Embed tz data. Check that Hermes reports the device zone.
- **Data:** migrations start at 0003, numbered by hand after `ls migrations`. New tables only; do not touch `users`, `ApiUser` or `/me`. One daily spin per user per local date. A reroll spends the bonus in the same transaction and never repeats the category.
- **Test traps:** `TestMigrations_0002IsReversible` fails at its start check (in `postgres_test.go`, the check that says `expected version 2 (clean) before the test`, which wants exactly version 2) and restores to version 2 in its deferred `goto 2`, so both change to the latest. Find them by that text, not by line number: line numbers move whenever a test is added above. Cleanups truncate `users` with cascade, so per-user tables must reference `users(id)`. Go tests run only on a database whose name ends in `test`. The three real dev accounts in `rndmroll_dev` are never deleted or edited. Apply no Phase 2 migration before the tree is ready.
- **Launch files:** `app/index.tsx` redirects a finished account to `/(app)/profile` (Phase 2 sends it to `/(app)/today` instead, except during first run, next bullet), and `app/(app)/_layout.tsx` is a Stack with `profile` and Phase 1's placeholder `home`, no tab bar. Replace the Stack with Tabs (Home, Spin, Profile) and delete `app/(app)/home.tsx`: under Tabs every file in `app/(app)/` becomes a tab by itself, and its only link (the Continue button on the Profile page) now goes to Today. Redirect to explicit children only, `/(app)/today` or `/(app)/profile` (a bare `/(app)` shows Unmatched Route). After ANY change to those files, a Stack.Protected guard, a Tabs layout or a redirect, cold-launch the dev client (`xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`) and confirm the first screen (record the first frames). An edit to `lib/api/client.ts` or to a launch file can reload the dev client back to Welcome, so make it when the developer is not mid-test on the phone, and say so.
- **First run (rebase 2026-10-09):** Phase 1 sign-up is two pages. Page 2 (the "Make it yours." photo-and-bio page, `ProfileForm` mode `extras`, with Continue and Skip for now) shows once, on the `/(app)/profile` route, while the in-memory flag `extrasPending` is true. The flag lives in `lib/session/store.ts`: `signIn(result, { extras: true })` sets it after a sign-up, `startExtras()` sets it after a finished Apple or Google account, `finishExtras()` and sign-out clear it, and a relaunch never has it. Phase 2 only READS it (`useSession()`, no edit to the store): `app/index.tsx` sends to `/(app)/profile` while it is true and to `/(app)/today` otherwise. That one index route also runs after every sign-in, because the root Stack puts `index` back when `(auth)` leaves (checked in the installed router, `StackRouter.getStateForRouteNamesChange`), so a sign-up, a finished social account and a plain log in all pass through it. The bottom bar is hidden on that page (`tabBarStyle` display none on the `profile` screen while the flag is true). Continue (after its save) and Skip for now call `router.navigate('/(app)/today')` first and `finishExtras()` second. If a person ever reaches Today with the flag still set (Android hardware Back is the only road found), nothing breaks: the bar shows on Today, and the Profile tab shows page 2 once more until Continue or Skip for now clears it. On iOS there is no other road off the page: no bar, no Back, no Log out on it. `app/_layout.tsx`, `app/(auth)/_layout.tsx` and `lib/session/store.ts` are not edited.
- **Profile as a tab:** the tab shows `app/(app)/profile/index.tsx`, which renders `ProfileForm` in mode `extras` while the flag is set and in mode `edit` otherwise (`profile/edit.tsx` is a second copy that nothing opens; leave it alone). Edit mode in the tab: no Back control (unconditional, because `router.canGoBack()` is true through tab history and the old condition would show it), stay after Save with a "Saved." line (skip the `router.back()` at the end of `submitEdit`), the bottom button keeps its two states but "Continue" now opens Today instead of Phase 1's `/home`, and `Screen` gets an `edges` prop (no double bottom inset). One new `ProfileForm` prop, `inTab` (default off, edit mode only), carries these.
- **Limits:** no new native module; never run `expo lint`; no JS test runner (a throwaway node script checks the pure wheel logic), so wheel, countdown and stepper checks are also device steps ("reply ok or what looked wrong"). Local commits, one per task, explicit paths, never pushed.

</code_context>

<specifics>
## Specific Ideas

- The countdown shows the server's open time, in tabular figures.
- Result: settle the wheel, then the word rises into place, with 500 to 800 ms of calm and no button to another screen.
- Settled in the UI spec (revisions 2 and 3): no card arrow, flame or "streak"; the header photo is a named circle exception (confirmed by the developer on 2026-10-08, D-16); the Spin button is an 8dp dark square with a light icon (confirmed on 2026-10-08, D-15); the wordmark is "RNDMRoll"; the star is a drawn SVG shape.
- Stale, ignore: "camera" in `CategoryRow.tsx`, the two Blockers/Concerns lines in STATE.md about ROLL-01 and ENTRY-04 (lines 82 and 83 on 2026-10-09; search for the words, the numbers move), "Dark-primary".

</specifics>

<deferred>
## Deferred Ideas

- Friends "Recent activity" card on Today: Phase 5.
- Diary tab (Phase 4) and Board tab (Phase 5).
- The bell and phone reminders, including the 8 PM reminder: later, needs a native rebuild.
- Dragging wedge edges to resize.
- Earning a reroll from real posts: Phase 3.
- Feed date across time zones (author's or viewer's): Phase 5 decides.
- Phone checks for an unfinished account and for a just finished Apple or Google account: with Phase 1's check 5, at the end of the project (both buttons are hidden until the real accounts exist). Static checks cover them meanwhile (plan 02-03 Task 2).
- A cleanup that clears the first-run flag when a person leaves the photo-and-bio page by a road other than Continue or Skip for now (only Android hardware Back is known): not built, because nothing on iOS leads off that page.

</deferred>

<hand_edits>
## Hand edits at merge-back

Apply by hand in the main tree after Phase 1 is closed, with explicit paths, before Phase 2 is built. Quotes show the start of the old text.

**1. REQUIREMENTS.md, ROLL-01 (last sentence).** Replace the sentence that begins "Exact window/late-badge grace-period cutoff" with: "The window stays open until local midnight. A post after midnight and before the next 8:00 PM is late (the feed still unlocks, with a late badge). After that the day is missed and does not count toward the weekly goal."

**2. REQUIREMENTS.md, ROLL-02.** Replace the line with: "User can earn one bonus reroll each week the weekly goal is met (an entry posted on at least 4 days, Monday to Sunday), keep one at a time, and spend it to re-spin within the same day".

**3. REQUIREMENTS.md, ENTRY-04.** Replace the line with: "User can skip a day when the spun category isn't satisfiable (e.g., wheel says "book," nothing was read that day). No entry is required that day. The skip is visible in the diary (DIARY-01), not a silent gap, and shows its effect on the week (e.g., "2 of 4 this week"). A skip is distinct from a reroll (ROLL-02, which swaps the category and still requires posting)." Delete the old "Freezes are capped" and "A frozen day" sentences.

**4. ROADMAP.md, Phase 2.** Replace the "Resolved" note's last sentence (begins "Exact window/late-badge") with the D-03 window. Replace "Plans: TBD" with the list 02-01 to 02-03. Criterion 3 keeps its "streak-based reroll token" wording on purpose (D-04). REQUIREMENTS.md ROLL-01 to ROLL-04 are ticked at the close-out, not now.

**5. ROADMAP.md, Phase 3.** Replace the "Resolved" note (begins "Unsatisfiable-category days use a capped streak-freeze") with: "Unsatisfiable-category days use a free skip: no entry required, visible in the diary, and it shows its effect on the weekly goal count. Distinct from the ROLL-02 reroll." Replace criterion 4 with: "User can skip a day when the rolled category isn't satisfiable and sees how it changes the week count (for example, 2 of 4 this week)".

**6. PROJECT.md.**
- Key Decisions row "Unsatisfiable-category resolution: capped streak freeze": replace with "Unsatisfiable-category resolution: a free skip plus a weekly goal". Rationale: "A forgiving weekly goal (4 posted days per Monday to Sunday week, 2026-10-07) replaces the capped freeze."
- Context bullets "Structural risk, now resolved" and "Related detail, now resolved": use the free skip, weekly goal and D-03 window in place of the freeze cap and tuning clauses.
- Active bullet "optional weekly streak-earned reroll", the out-of-scope line 32 ("optional streak-earned token") and Key Decisions row "Roll cadence": reword to a bonus reroll earned each week the weekly goal is met.
- Moodboard bullets "Category icon set ..." and "Wheel visual pattern: pie-slice wedges each with icon + label": add "(superseded: words only on wedges, Phase 2 decision 7; the Welcome wheel keeps its drawings)".

**7. 01-UI-SPEC.md, the "Coordination with Phase 2" line (line 191 of revision 15, in the Color section; search for the words, the number moves).** Replace its text with: "SUPERSEDED 2026-10-07 (Phase 2 decision 6): wedges use ink-and-light grey tones only, from tokens." Nothing else needs the line: the fuller revision 8 note is no longer in the file (the front matter was rewritten and only git history holds it), so there is no second place to edit. The Phase 1 rows added to PROJECT.md since (the two-page sign-up and the less-typing rows) sit below the rows that hand edit 6 changes and do not collide with it (checked 2026-10-09).

Do not copy STATE.md or config.json. Commit with a pathspec, only after ROADMAP.md and STATE.md show clean.

</hand_edits>

---

*Phase: 02-daily-roll*
*Context gathered: 2026-10-07*
