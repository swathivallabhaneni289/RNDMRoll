# Phase 3: Daily Entry (log it) - Context

**Gathered:** 2026-10-09
**Written:** 2026-10-10, on paper, against the Phase 2 documents as revised on 2026-10-09. Phase 2 is NOT built. Every Phase 2 file, table and function named here is the Phase 2 PLAN's name. At build time the built code wins; each plan's start_rules say how to re-check.
**Status:** Ready for the developer to read. NOTHING IS BUILT. PHASE 3.
**Approval word for a Phase 3 plan:** "go" ("start" and "approved" belong to Phase 1). Phase 2 has its own "go". Phase 3 needs a second, separate "go", said after Phase 2 is built, walked on the phone and closed.

Wording: folder and requirement ids keep the old name (ENTRY-01 to ENTRY-04). Anything the user sees says "spin", never "roll".

<domain>
## Phase Boundary

After spinning, the user logs what the spin asked for: a photo of their own, a title, 1 to 5 stars in half steps, one optional reaction out of six, and an optional comment. Or they skip the day, for free. A post counts the day toward the weekly goal of 4. A skip does not count and is not a miss. A post can be changed or deleted afterwards, and a deleted post still counts as a posted day.

What the user gets: a 'Log it' button and a 'Skip this one' link on the result screen of Today, an entry screen, a Posted state and a Skipped state on Today, a skip mark on the week dots, and an entry page (view, edit, delete). The entry page is built once here and reused by the Phase 4 diary.

Out of scope:
- The diary list, grid and category filter (Phase 4). Phase 3 builds the page a diary row opens, not the diary.
- Friends, the feed, friends' reactions, friend-only photo links, reporting a post (Phase 5).
- Real storage accounts, the domain, Terms and Privacy (the developer's launch list).
- Resizing or cropping a photo before upload (it needs a new native module, and none is allowed).
- Drafts that survive closing the app, posting without a connection, reminders.

Phase 3 is BUILT only after Phase 2 is built and closed, and after these documents are copied into the main tree by hand, with the edits in `03-HAND-EDITS.md`. Phase 3 needs ALL of Phase 2: the spin records, `posted_days`, the single `MarkPosted` call, the clock, the Today response, the result screen and the week dots.

</domain>

<decisions>
## Implementation Decisions (all answered 2026-10-09)

### The three answers (developer, 2026-10-09)
- **D-01 Movie and book days (question 1, option A):** A person may log anything they watched or read lately. No date is asked and nothing says 'earlier'. The skip is there for people who truly have nothing. No requirement edit.
- **D-02 The reaction (question 2, option A):** One emoji out of exactly six friend-safe ones: heart, laughing, wow, fire, clap, yum. Tap the chosen one again to clear it. The author and friends use the same six (Phase 5 reuses them). ENTRY-01's reaction becomes 'one of six fixed reactions'. The standing rule bans emoji as icons for screen furniture. Here the emoji ARE the reaction content the developer chose, so they appear only as the six characters in the picker and where a chosen reaction is shown, nowhere else. The UI spec says so.
- **D-03 Editing and deleting (question 3, option A):** The author can change title, stars, reaction, comment and photo at any time, and can delete the post. A deleted post still leaves the day counted as posted and settled, so there is no second post that day. ENTRY-01 gains: 'the author can change or delete a post; a deleted post still counts as a posted day'.

### Decided earlier (restated, so nothing is asked twice)
- **D-04 What an entry holds:** a photo, a title, 1 to 5 stars in half steps, an optional reaction, an optional comment.
- **D-05 The photo:** the person's own, from the camera or the library, chosen each time. Both are fine. No stock or fetched art, ever.
- **D-06 Repeat titles:** a repeat title makes a new dated entry and never replaces the old one (ENTRY-02).
- **D-07 The category:** the spun category is shown at the top of the entry before posting (ENTRY-03).
- **D-08 What counts:** a day counts only when posted. The goal is 4 posted days, Monday to Sunday.
- **D-09 Skipping:** free, not a miss, shown in the diary (Phase 4), and it shows its effect on the week ('2 of 4 this week'). It is not the same as spinning again. The capped freeze is gone.
- **D-10 Window and late:** the wheel is open 8 PM to midnight. A post after midnight and before the next 8 PM is late (the feed still unlocks, with a late tag; Phase 5). After that the day is missed.

### Claude's Discretion (safe defaults, told to the developer, no question asked)
From the question notes (told 2026-10-08, no objection):
- Library photos carry no tag. The app quietly records 'camera' or 'library' on each entry and shows nothing.
- Stars: required, start empty, half steps, zero not allowed. A whole star is one 44dp tap. A second tap on the same star adds the half, so 3 becomes 3.5. The value is shown as a number. The minus and plus buttons from Edit wheel are included, for exact control and for VoiceOver.
- Title required, 1 to 80 characters, spaces trimmed. Photo required (the feed is photo cards), with a plain placeholder until chosen. Comment optional, up to 140 characters, shown on the entry page (whether the feed card shows it is Phase 5's call).
- The entry screen is one scrolling page: category word on top (fixed), photo, title, stars, reaction, comment, one dark square Save button.
- The result screen gets its first button, 'Log it' (a normal 8dp square button), and the line 'Posting it now would count as late.' when it applies.
- After Log it, Today shows 'Posted.', the updated week card and a plain 'View entry' link to the entry page, which carries Edit and Delete.
- Skip is a quiet text link, 'Skip this one'. It asks once, then it is final for that day. No reason is asked. It works any time the day is still open, including the late window. After it, Today shows 'Skipped.'. A skip gets its own mark on the week dots and does not count toward the 4.
- Late is decided by the server clock, never the app. A post sent at 11:59 PM that arrives after midnight counts as late.
- One entry per day. After spinning again, the entry belongs to the final category, saved with the name it had that day.
- Photos: pick quality 0.8, 50 MB limit (changed from 5 MB on 2026-10-09, see below), plain message if too big or the wrong type (JPG, PNG, WebP). Saving needs a connection; if it fails the form stays filled and a retry sends the same photo once. iOS photo permission wording stays the default until the Phase 5 rebuild.

Added while writing the plans (2026-10-10, new, told here; change any of them by saying so):
- **50 MB means 50 x 1024 x 1024 bytes** (52,428,800), one named constant on each side, the same ceiling as profile photos (`maxAvatarBytes` on the server). The developer chose this on 2026-10-09 (the earlier notes said 5 MB, but Phase 1 had dropped its own 5 MB avatar limit after big photos were refused). It is a safety ceiling far above any real photo, so a full-size library photo saves. No resize is possible without a new native module, and none is needed. The walkthrough still tries a full-size library photo and reports the result. Changing the limit later is one constant per side.
- **Photos show as a square**, centre-cropped, on the entry screen and the entry page. The stored file is the original; Phase 4 and 5 may crop differently.
- **The six reactions are stored by short name** (`heart`, `laughing`, `wow`, `fire`, `clap`, `yum`). The server and the database hold no emoji. One app file, `lib/entry/reactions.ts`, holds the six characters (exact code points in the UI spec). They draw in the system's own colour, the one sanctioned colour on an otherwise monochrome screen.
- **After midnight (late window):** a post or a skip made after midnight keeps Today on 'Posted.' or 'Skipped.' for that spin until the next spin opens. One made before midnight returns to Phase 2's 'Your next spin is coming.' after midnight. This needs one new column, `spins.settled_at`.
- **The phone sends the spin number it saw** (`spin_seq`) with a post or a skip. If another phone used the bonus spin meanwhile, the server refuses (`spin_changed`) instead of attaching the entry to a category the person never saw.
- **A retry of Save returns the same entry.** The photo key, which is new for every upload, is the key: same key, same entry, no second post and no error.
- **Delete is a POST** (`/v1/entries/{id}/delete`). The app's client sends only GET, POST and PATCH, and Phase 2 decided it gets no DELETE. A 'not found' on delete is treated as already done.
- **Photo files:** a photo that is replaced or deleted is removed from storage right after the change, best effort (a failure is logged and ignored). A photo uploaded and never used stays in storage; noted, no cleanup job. The real storage key will need delete rights (the test storage has them).
- **Photo links are public** to anyone who has the link until Phase 5 swaps in friend-only links. The database stores the key (folder and file name), never the link; the server builds the link when it reads. Fine for a small test group; flag it before any outside user. Storage upload has never run against a real bucket.
- **The test reset also clears entries and real posted days** (Phase 2's reset cleared only test ones), so a throwaway account can be walked again. Their photo files stay in storage.
- **Leaving the entry screen or the edit screen with unsaved work asks first**, with the same sheet as Edit wheel.
- **No hint words inside the Title and Comment boxes** (the developer removed such hints on sign-up, Phase 1 revision 17). 'Optional' sits on the label row, as on the Bio box.
- **Edit has no time limit.** It changes title, stars, reaction, comment and photo only. The date, the category and the late stamp never change.
- **The stars figure:** halves show one decimal ('3.5'), whole numbers show none ('3').
- **Week edge:** a post or skip made just after midnight on a Monday, for Sunday's spin, belongs to the week that just ended (the server counts it there), but Today's card already shows the new week ('0 of 4') and last week's dots are not shown, so the post can look as if it did not count. The server is right; only the display is off. The skip sheet leaves out its '{n} of {goal}' sentence in that case. Not worth a new screen for a once-a-week window of a few hours; say so if you want it handled.
- **A pretend posted day for today** (test tools only) makes Today say 'Posted.' with the deleted-entry line. Real use never meets this.
- **Entry screens live inside the Home stack** (`app/(app)/today/`: `log`, `entry`, `edit-entry`), so the bottom bar stays Home, Spin, Profile and Phase 2's `ls "app/(app)"` check still holds.

</decisions>

<canonical_refs>
## Canonical References

Read before planning or building. Paths are inside the project unless stated.

### The questions and answers (outside the project, read only)
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/questions/PHASE3-QUESTIONS.md`: what was decided, the safe defaults and the 'Notes for the plan' (binding). The three answers of 2026-10-09 are D-01 to D-03 above.
- `/Users/swathivallabhaneni/.local/share/rndmroll-phase2/questions/PHASE4-QUESTIONS.md` and `PHASE5-QUESTIONS.md`: what the diary and the feed expect from Phase 3 (the entry page, the posted-day gate, the key-not-link rule, the six reactions).

### Project rules
- `.planning/PROJECT.md`: stack, standing design rules (no purple gradients, no pill buttons, no emoji-as-icons, no em dashes, no fake counters, no AI-slop copy or photos, no over-the-top animation), launch gate. Light editorial, strict monochrome.
- `.planning/REQUIREMENTS.md` (ENTRY-01 to ENTRY-04) and `.planning/ROADMAP.md` (Phase 3 criteria).
- `.planning/phases/02-daily-roll/` (all six files): the spin table, `posted_days`, `MarkPosted`, the Today response, the clock, the result screen, the week dots, `ConfirmSheet`, the Edit wheel leave guard. Phase 3 hooks into them exactly as they define them.
- `.planning/phases/01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later): tokens, type, spacing, radius, elevation, the soft text box, the profile form's photo picking.
- `docs/motion-interaction-direction.md`: section 5 (rating interaction: very small scale, no bounce, no colour), section 4 (log entry). The progressive reveal of section 4 is NOT adopted (one scrolling page, decided).

### Code to read before planning or building
- Server: `internal/storage/s3.go` (the presign seam), `internal/httpapi/profile.go` (the avatar ticket and `avatarURLAllowed`), `internal/httpapi/server.go`, `internal/httpapi/errors.go`, `internal/store/postgres/postgres_test.go` (the `latestMigration(t)` helper and the migration tests), `cmd/api/main.go`, and the Phase 2 files once built: `internal/spin/`, `internal/spin/spintest/`, `internal/store/postgres/spin_repo.go`, `internal/httpapi/spin.go`, `spin_errors.go`, `spin_testtools.go`.
- App: `lib/theme/tokens.ts`, `components/ui/*` (`AppText`, `PrimaryButton`, `TextButton`, `TextField`, `Screen`, `PhotoPlaceholder`), `components/profile/ProfileForm.tsx` (photo picking, messages, upload-once), `lib/api/profile.ts` (`fileSize`, `uploadAvatar`), `lib/api/client.ts` and `types.ts` (read only; see Launch files), `lib/motion/primitives.tsx`, and the Phase 2 files once built: `components/today/*`, `components/ui/ConfirmSheet.tsx`, `components/wheel/WheelEditor.tsx`, `lib/wheel/todayStore.ts`, `useToday.ts`, `format.ts`, `app/(app)/today/*`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets
- **Photo picking:** `ProfileForm.tsx` already has the action sheet (Take photo, Choose from library, Cancel), both launchers with `quality: 0.8` and `preferredAssetRepresentationMode: Compatible`, the permission messages, and an upload-once memory (`uploadedRef`). `lib/api/profile.ts` has `fileSize(uri)` (the exact byte length the presigned upload signs) and `uploadAvatar` (ticket, then `FileSystem.uploadAsync` PUT, then the result). Phase 3 copies the pattern into `lib/api/entries.ts` and `components/entry/PhotoField.tsx`. It edits neither Phase 1 file.
- **Photo storage:** `internal/storage/s3.go` binds the content type and length into the presigned request and keeps a stub seam for tests. `AvatarStore` is an interface that `fakeAvatarStore` implements, so it is NOT extended: entry photos get their own interface in a new file.
- **Key checks:** `avatarURLAllowed` in `profile.go` shows the rule (own folder prefix, plain file name, no `..`). The entry key rule is the same idea, stricter because the server makes the name.
- **Text boxes:** `TextField` variant `soft` has the small-caps label, a `hint` on the label row (used for 'Optional' and for the title's '12/80'), the count inside a multiline box, and a `success` note. Single-line boxes draw their text in a 40dp frame (Phase 1 revision 18).
- **Stars:** Ionicons `star`, `star-half`, `star-outline` all exist in the installed glyph map. The dev-only lab (`lab/components/StarRating.tsx`) shows the read-only idea. The lab is never imported by the app.
- **Tokens, motion:** `color`, `space`, `radius.md`, `type`, `elevation` from `tokens.ts`; `PressScale`, `FadeInUp`, `ImageReveal` from `lib/motion/primitives.tsx` (its own comment names a planned `RatingSelect`; Phase 3 uses `PressScale` for it, no new primitive). `dateLine()` from Phase 2's `lib/wheel/format.ts` gives 'WED, OCT 8' from a server date.
- **Today:** Phase 2's store (`lib/wheel/todayStore.ts`), `useToday`, `TodayScreen`, `ResultBlock`, `WeeklyCard`, `StatusMessage`, `ConfirmSheet`, the leave sheet and its `beforeRemove` guard.

### Gaps to close (nothing of these exists)
- The `entries` table and `spins.settled_at`; the entry endpoints, the photo ticket and the skip call; the shared transaction (entry insert, posted day, spin result); the `MarkPosted` split; the Today changes (states `posted` and `skipped`, `entry`, `skipped` per week day); the test reset widening.
- The half-star input, the reaction picker, the entry screen, the entry page, edit, delete; Today's Log it, Skip this one, Posted and Skipped; the dash on the week dots.

### Integration points
- **Routes:** `internal/httpapi/server.go` is the only engine builder. Entry routes go on the signed-in group (`RequireAuth` then `RequireUser`), each calling `u.OnboardingComplete()` first. The user id comes only from the token. An entry id in a path is a UUID parsed in Go and always combined with `user_id = caller`.
- **Assembly points:** `Deps` and `NewServer`, `cmd/api/main.go` and both test builders (`buildIntegrationServer`, `newFullTestRig`) change together, in one task.
- **Clock and transaction:** every entry call reads `now` once from Phase 2's per-account clock and runs inside the same `InUserTx` as the spin calls (the state-row lock). No `time.Now()` in new code. `MarkPosted` opens its own transaction in Phase 2; Phase 3 moves its body into a helper that takes an open transaction, so the entry insert, the posted day and the spin result commit or fail together.
- **Data:** migration 0005 (the Phase 1 clean-up is 0003, Phase 2's is 0004). New table `entries`; two small additions to `spins`. `truncate users cascade` cleanups still work because `entries` references `users(id) on delete cascade`.
- **Test traps:** the migration tests in `postgres_test.go` read the latest version from the highest migration file name (`latestMigration(t)`) and restore it in a cleanup (`requireMigrationDatabase`), so adding 0005 needs no change to them; a new migration test uses the same helpers (`requireMigrationDatabase`, `migrateGoTo`, `withFreshPool`, `schemaVersion`). A test pretend posted day (source `test`) hides a real post for that date: reset the test account first. The reset deletes `entries` before `spins` (a foreign key points from one to the other). Cleanups truncate `users` with cascade, so per-user tables must reference `users(id)`. `fakeAvatarStore` implements `AvatarStore`, which is why entry photos get their own interface and their own fake. The four real dev accounts in `rndmroll_dev` (`smoke-test@example.com`, `ava.stone.1004@example.com`, `test1@gmail.com`, `test4@gmail.com`) are never deleted or edited.
- **Launch files:** Phase 3 edits NO launch file. It does not touch `app/index.tsx`, `app/(app)/_layout.tsx`, the root or auth layouts, `lib/session/store.ts` or `lib/api/client.ts`. It adds three routes inside the nested Home stack (`app/(app)/today/_layout.tsx`). An edit to `lib/api/types.ts` (new error codes) can reload the dev client back to Welcome like an edit to `client.ts`, so make it when the developer is not mid-test on the phone. One cold launch is still checked after the new routes exist.
- **Limits:** no new native module; never run `expo lint` (it rewrites `package.json`); no JS test runner (a throwaway node script checks the pure star, reaction and rule logic); Go tests run only on a database whose name ends in `test`, with `-p 1`, through the guarded helpers. Local commits, one per task, explicit paths, never pushed.

</code_context>

<specifics>
## Specific Ideas

- The stars input is five 44dp stars with a minus button at the left end and a plus button at the right end, so it fits a 375dp-wide phone (308dp of controls in a 327dp column).
- The skip mark on the week dots is a short ink dash (10dp wide, 2dp tall) in place of the dot. Posted is a filled dot, a past day with nothing is a thin grey ring, a skipped day is the dash. Never colour alone.
- The skip sheet says the effect before the person decides: 'You are at 2 of 4 this week. A skipped day doesn't count toward it, and it isn't a miss.'
- Posted late shows as a plain 'Posted late.' line on Today and on the entry page. The feed's 'Late' badge is Phase 5.
- Stale, ignore: the 'Rolled 8:26 PM' strings and fake comments in the lab fixtures, and the lab's glass look (feed only, Phase 5).

</specifics>

<deferred>
## Deferred Ideas

- The diary list, grid and filter: Phase 4. Friends, the feed, friend reactions and friend-only photo links: Phase 5.
- A job that removes photo files nobody uses (half-finished posts, reset test entries).
- Resizing a big photo before upload (needs a native module and a rebuild).
- An 'Edited' marker or history (the table keeps `updated_at` for it).
- Reporting a post (the Phase 5 notes put it on the launch gate).
- Answer 1 options B and C ('when?' on movie and book days; strict same-day) and answer 2 options B and C (a separate author set; no reaction): not chosen.
- Drafts that survive closing the app.

</deferred>

<hand_edits>
## Hand edits at merge-back

The exact find-this and replace-with text is in `03-HAND-EDITS.md`. In short: ENTRY-01 gains 'one of six fixed reactions' and the change-or-delete sentence; the ROADMAP Phase 3 plan list and plan count; one PROJECT.md Key Decisions row ('2026-10-09: Phase 3 entry rules'); the 02-UI-SPEC section 4.1, 4.3, 6 and 13 changes (Posted and Skipped rows, the result screen button, the skip mark); the 02-CONTEXT lines that say 'no button until Phase 3'; and a few small notes for the Phase 2 plans (the Today shape, the `MarkPosted` split, the reset). ENTRY-04, the ROADMAP Phase 3 'Resolved' note and criterion 4 are already done (Phase 2 hand edits 3 and 5). Apply them in the main tree by hand, after Phase 2 is closed, before Phase 3 is built.

</hand_edits>

---

*Phase: 03-daily-entry*
*Context gathered: 2026-10-09*
