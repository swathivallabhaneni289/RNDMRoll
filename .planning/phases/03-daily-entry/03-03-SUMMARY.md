---
phase: 03-daily-entry
plan: 03
status: built, waiting for the developer's walkthrough (Task 1 and the static gates of Task 2 only)
built: 2026-10-14
commits: f7da618, ec5eee7, 87e1f94
---

# 03-03 Summary: the entry page, edit, delete, skip and the dash

Task 1 of the plan is built, and the static gates of Task 2 are green. The cutover was done earlier by
the lead (dev database at version 6, backups exist). Nothing here was run on the phone: the cold
launch and the whole walkthrough (Task 3) are the developer's. Phase 3 is NOT ticked in `ROADMAP.md`
or `REQUIREMENTS.md`, and `STATE.md` is untouched. Nothing was pushed.

## The wire check (done first)

`lib/api/entries.ts` and `lib/api/types.ts` (as 03-02 left them) were compared with the endpoint
table of `03-01-SUMMARY.md` and with the Go json tags in `internal/httpapi/entry.go` and
`internal/spin/model.go`. **No mismatch was found, nothing in 03-02's code had to change.**

- `Entry` has the 13 members of the server (`id`, `local_date`, `category`, `title`, `stars`,
  `reaction`, `comment`, `photos [{id,url,position}]`, `photo_url`, `photo_source`, `late`,
  `created_at`, `updated_at`).
- `Today.entry` is `{id, late, title, stars, reaction, photo_url, photo_count}` or null, and each
  week day has `skipped`. States `posted` and `skipped` exist.
- The create body, the ticket body and the error codes match. `size_bytes` is optional on the
  server and the phone does not send it.
- New calls use the routes as built: `GET /v1/entries/{id}`, `PATCH /v1/entries/{id}`,
  `POST /v1/entries/{id}/delete` (not DELETE), `POST /v1/spin/skip`.

## Commits

| Commit | What |
|---|---|
| `f7da618` | calls and pure rules: `getEntry`, `updateEntry`, `deleteEntry`, `skipDay`, `not_found` message, `isEntryId`, `skipBody`, `changedFields`, 20 new checks |
| `ec5eee7` | `EntryView`, `useEntry`, `edit-entry` with `useUpdateEntry`, the thin `entry` route, the two `Stack.Screen` lines |
| `87e1f94` | skip in `DayActions`, 'View entry' in `SettledBlock`, the dash in `WeeklyCard`, no cross-fade in `TodayScreen` |

The plan asked for one Task 1 commit. It is three, because of the lead's instruction to commit after
each logical chunk. This summary has its own commit after them.

## What was built

- **Entry page** (`components/entry/EntryView.tsx`, route `app/(app)/today/entry.tsx`, picture 06):
  Back row with the place name ('Today', or 'Diary' when `from` is 'diary'), then the photos square
  and edge to edge with the system's own paging, '1 / 4' counter at the bottom right (none for one
  photo), `image-outline` fallback with 'Couldn't load this photo.', one adjustable VoiceOver
  element ('Photo 1 of 4 for {title}'). Under it: category and long date on one line (category cut
  first), 'Posted late.' when late, the title (up to 3 lines), the read-only stars with the figure,
  a thin rule and the reaction at 24dp as one spoken element, and the comment in the `note` role
  with its line breaks. Pinned footer 'Edit | Delete', 56dp, hairlines, no icon, bottom safe area
  included, no bottom bar. The first photo reveals, then the details rise 90ms apart (none under
  Reduce Motion). Loading, offline ('Try again') and gone states as the spec says.
- **`useEntry`**: loads by id, reloads when the screen regains focus, keeps the entry on screen while
  a later read runs or fails. An id that is not a UUID is 'gone' and is never sent to the server.
- **Edit entry** (`app/(app)/today/edit-entry.tsx`, `useUpdateEntry`): the form is mounted only after
  the entry has loaded, so the draft starts from it on the first render. Header 'Cancel', 'Edit
  entry' centred, category word with the entry's own date. Only changed fields are sent; `''`
  clears the reaction or comment; when the photos changed the whole ordered list is sent (`{id}`
  for a kept photo, `{photo_key, photo_source}` for a new one, each new file uploaded once through
  the upload memory). The leave guard is the one of `log.tsx`. A gone entry shows the message and
  'Back to Today'.
- **Delete**: `POST /entries/{id}/delete`. The answer's Today goes into the store and the page goes
  back; 'not found' counts as done; a lost connection or any other failure shows one line on the page.
- **Skip** (`DayActions`): the quiet underlined 'Skip this one' under 'Log it'. It asks with the
  phone's own alert and sends the date and number of the spin on screen. The answer's Today goes
  into the store, so Today shows Skipped at once. A failure shows its line under the link; the four
  state answers also ask for Today again. Both controls are inert while it runs and in the
  non-live copy of the bonus overlay.
- **'View entry'**: underlined link at the bottom of the Posted row's text column, outside the
  spoken group, pushes `entry` with the id and `from` = 'today'.
- **The dash**: a skipped day draws the 10 by 2dp `ink` dash. Precedence posted, skipped, today's
  ring, small dot. Letters stay `muted`. The spoken line says 'skipped'.
- **Posted and Skipped on Today**: as 03-02 built them (pictures 04 and 05); only the swap changed
  (see differences).

## Differences from the paper (the lead's overrides first)

1. **Confirmations are the phone's own `Alert.alert`**, as for Log out. Skip: title 'Skip this one?',
   body as the spec, buttons 'Cancel' and a red 'Skip it'. Delete: title 'Delete this entry?', body
   as the spec, buttons 'Cancel' and a red 'Delete'. **The spec's 'Don't skip' and 'Keep it' are
   replaced by 'Cancel'** (the phone's standard cancel, same as Log out). It is one string in each
   file if the developer wants the spec words back. `ConfirmSheet.tsx` was not edited, so the plan's
   `destructive` row flag was NOT added (the file is as the lead left it, with `dismissLabel` and
   `body`). The leave sheet and the bonus sheet still use `ConfirmSheet`.
2. **No cross-fades between Today states.** 03-02 had a real 400ms cross-fade from Result or Late to
   Posted or Skipped (an overlay copy of the old block). It is removed: Posted and Skipped appear at
   once, and the heading text swaps at once to and from 'Posted.' and 'Skipped.'. **Still fading, on
   purpose:** the spin reveal (wheel to word) and the bonus spin's wheel fade, because they are the
   spin animation and not a state swap; and the entry page's `FadeInUp` details and photo reveal, which
   the lead allowed. All other heading changes keep Phase 2's fade. The developer should say if
   any of these should also go.
3. **Skip text for the previous week's day.** The spec leaves out the first sentence ('You are at {n}
   of {goal} this week.') when the spin is from before `week.start`, but the next sentence says
   "toward it", which then has nothing to point at. In that case the text reads "A skipped day
   doesn't count toward your week, and it isn't a miss. You can't post for this day afterwards."
   Otherwise the text is the spec's, word for word.
4. **Edit refreshes Today.** A PATCH answers with the entry only, so after a save Today is asked for
   again (`refetchToday`); otherwise the Posted row would keep the old title, stars and cover. The
   plan did not say this.
5. **'Back to Today' from a gone entry** (entry page and edit entry) uses `router.dismissAll()`
   (or `replace` to Today when nothing is above it), so it lands on Today and not on another gone page.
   The helper is `components/entry/goToToday.ts`.
6. **Edit shows a short 'Loading your entry.' screen** before the form, because it loads the entry
   itself (it does not trust the entry page's copy).
7. **`useEntry` also reports `failure`** ('network' or 'generic') so the offline block can show the
   right `StatusMessage`.
8. **Binding decision 12 needed no edit.** The Tabs layout already lists `entry`, `log` and
   `edit-entry` in `BARLESS_TODAY_PAGES` (03-02). The route file itself does nothing about the bar.
9. **Kept from 03-02, still open:** 'Log it' has no spoken hint ('Opens the entry screen.'), because
   `PrimaryButton` takes no hint prop.

## Gates (run 2026-10-14, static only)

| Gate | Result |
|---|---|
| `npx tsc --noEmit` | exit 0 |
| `scripts/check-entry.ts` | 'reaction names match the server list', 'ok: 2721 checks, no failure' (was 2701) |
| `scripts/check-wheel.ts` | 'ok: 47877 checks, no failure' |
| `scripts/check-settings.ts` | 54 checks, 0 failures |
| `scripts/check-navigation-motion.ts` | 37 checks, 0 failures |
| `npx expo export --platform ios --output-dir <scratch>` | exported, one iOS bundle, no error. No tracked file changed |
| `go build ./...`, `go vet ./...` | exit 0 (read only, no server file touched) |

Scans (app-wide ones with `--exclude-dir=lab`; the emoji and em dash scans were run with `bash -c`):

| Scan | Result |
|---|---|
| em dash over app, components, lib, scripts, internal, cmd, migrations | nothing. No en dash in any file this plan changed |
| emoji and symbols over app, components, lib, scripts | exactly `lib/entry/reactions.ts`; over internal, cmd, migrations: nothing |
| hex or `rgba(` in `components/entry`, `components/today`, `app/(app)/today` | nothing. App-wide 6-digit hex: only `lib/theme/tokens.ts` and `lib/theme/tokens.test-assert.ts` |
| the word 'roll' in the plan's folders and files | nothing |
| `Math.random`, `new Date`, `Date.now` in entry, today, wheel, `app/(app)/today`, `lib/entry` | nothing |
| `radius.lg` in entry, today, `app/(app)/today` | nothing. App-wide: Phase 2's five files, unchanged |
| `radius.full` in entry, today, `app/(app)/today` | exactly `PhotoField.tsx` and `ReactionPicker.tsx` |
| `chevron-forward` and `dateLine(` in `EntryView.tsx` | nothing (it uses `chevron-back` and `dateLineLong`) |
| `package.json`, lockfile, `app.json`, `ios`, `android` against `pre-phase3` | no diff |
| migrations against `pre-phase3` outside 0006; `internal/user`, `user_repo.go` | no diff. `ls migrations` is 12 files |
| `time.Now()` in the spin, storage and entry server files | nothing |
| `ls "app/(app)/today"` | `_layout.tsx`, `edit-entry.tsx`, `edit-wheel.tsx`, `entry.tsx`, `index.tsx`, `log.tsx`, `notifications.tsx` (exactly the plan's list) |

Scan results that differ from the plan's text, all from work outside this plan (checked: none of this
plan's three commits touch these files; `git diff --name-only 9423845 HEAD` lists only the 14 files
above):

- `grep "'DELETE'" lib app components` prints `lib/api/client.ts` (two lines). That is `ed34a92`
  (Blocked accounts, the Settings window). No entry code calls DELETE.
- `ls "app/(app)"` also lists `board.tsx` and `diary.tsx` (older than `pre-phase3`).
- App-wide `radius.full` is seven files, because `components/ui/Chip.tsx` (Settings window, `1fe35fe`)
  is on it.
- `git diff --stat pre-phase3` is not empty for `lib/api/client.ts`, `lib/session/store.ts`,
  `app/_layout.tsx` and `app/(app)/_layout.tsx`: other windows' commits since the tag. The
  `app/(app)/_layout.tsx` hide list line is 03-02's (`def647f`). This plan edits none of them.
- `app/lab/` is untracked and was left alone; the export did not trip on it.

## Not run (and why)

- `make test-all-integration` and the Go tests: they use the databases, and the lead said static
  gates only. No server code was written.
- The cutover recipe and the smoke test through the API: already done by the lead; not repeated.
- The cold launches (a) and (b) and the whole walkthrough: the developer's. Nothing was launched,
  terminated or restarted. Metro and the simulator were not touched.

## What the developer still has to walk on the phone

Everything below is unseen on a device. Plan 03-03 Task 3's text still applies, with these changes of
wording: the skip and delete confirmations are the phone's alert (buttons 'Cancel' and the red
action), and nothing fades between Today states.

1. **Cold launch** (Task 2): signed out gives Welcome; signed in gives Today with the bar and no
   Welcome flash or Unmatched Route; open 'Log it' (no bar), Cancel (bar back); force quit and open
   again gives Today. Also confirm the Phase 3 API is the one on 8080.
2. **Log it, then Posted** (steps 1 to 6): the Posted row, '+1' on the thumbnail with two photos.
3. **Entry page** (step 7): tap 'View entry'. The bottom bar is gone; the photo runs edge to edge;
   swipe sideways between photos and watch '1 / 2' follow; one photo has no counter; the date line,
   title, stars with figure, rule and reaction, serif comment; 'Edit | Delete' pinned at the bottom;
   Back row says 'Today'; a late post says 'Posted late.'. Check that scrolling the page moves the
   photo away but not the Back row or the footer, and that swiping the photos does not fight the
   vertical scroll.
4. **Edit** (step 8): form filled in, 'Save changes' grey until something changes; change title and
   stars; remove the second photo (no cross left on the last one); add a photo; clear the reaction
   and the comment; Save. The page shows the new data, and Today's Posted row shows it too (title,
   stars, cover, '+N'). 'Cancel' with changes asks to leave.
5. **Delete** (step 9): the phone's alert says 'Delete this entry?' with Cancel and a red Delete.
   After Delete, Today says 'Posted.' and 'This entry was deleted. The day still counts.', the week
   count is unchanged and there is no 'Log it'. (Claude then checks `entries`, `posted_days` and the
   photo files.)
6. **Skip** (steps 10, 11, 15): 'Skip this one' is a quiet underlined link under 'Log it'. The alert
   shows the count sentence with Cancel and a red 'Skip it'. Cancel changes nothing. 'Skip it' shows
   Skipped at once with no fade, with the struck word, and the dash on today's dot with the count
   unchanged. Force quit and reopen: the same. The late-window skip (12 AM) says 'Last night's spin
   was'.
7. **The dash** on other days: a day skipped earlier in the week shows the dash, not a small dot.
8. **Gone entry**: open the entry page for a deleted entry from elsewhere (or an id that does not
   exist) and check 'This entry isn't here any more.' and that 'Back to Today' lands on Today.
9. **Connection** (step 16 and delete): with the API stopped, the entry page shows 'Try again'; a
   Delete shows 'Couldn't connect...' on the page and the entry stays.
10. **Reduce Motion** on: details and photo just appear.
11. **Strings**: approve section 16 of `03-UI-SPEC.md`, including the replaced 'Don't skip' and 'Keep
    it' (now 'Cancel') and the one reworded skip sentence (difference 3).
12. **Full-size library photo** (step 17) and the other groups of Task 3 as written.

Things most likely to look wrong, from not having seen them: the paging gallery inside the vertical
scroll, the Edit and Delete footer's inner rule height, the 'View entry' spacing against a three-line
title, and how the Skipped struck word sits after the fade removal.

## Known stubs

None. No placeholder text or empty data flows to a screen in this plan's files.

## Self-Check: PASSED

Files created exist: `components/entry/EntryView.tsx`, `useEntry.ts`, `useUpdateEntry.ts`,
`goToToday.ts`, `app/(app)/today/entry.tsx`, `edit-entry.tsx`. Commits `f7da618`, `ec5eee7` and
`87e1f94` are on `phase-02-daily-roll`.
