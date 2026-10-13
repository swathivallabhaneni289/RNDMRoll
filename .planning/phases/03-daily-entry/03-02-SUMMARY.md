---
phase: 03-daily-entry
plan: 02
status: built, waiting for the server cutover (plan 03-03)
built: 2026-10-13
commits: 92f3c27, 6dcd7f3, def647f
---

# 03-02 Summary: the entry app

The app side of logging an entry, built from `03-UI-SPEC.md` revision 3, the developer's pictures of
2026-10-10 (03 Log entry, 04 Today posted, 05 Today skipped) and the wire contract of `03-01-PLAN.md`
Task 3b. The server of plan 03-01 was not live on the dev API while this was built, so nothing here has
run against it: every wire name below is the paper's, and `03-01-SUMMARY.md` did not exist yet (see Open).

## What was built

**Task 1, `92f3c27` (pure rules, API calls, check script).**
- `lib/entry/rules.ts` (limits, `cleanTitle`, `missingParts`, `stillNeededLine`, `nextSpinLine`,
  `settledLabel`), `stars.ts`, `reactions.ts` (the only file with emoji), `photo.ts` (limits, checks, the
  upload memory). No file in `lib/entry` imports another.
- `lib/api/entries.ts`: `uploadEntryPhoto`, `createEntry`, `entryErrorMessage`, and the `Entry` types.
- `lib/api/types.ts`: `TodayEntry`, `Today.entry`, `WeekDay.skipped`, error codes `no_spin`, `day_closed`,
  `spin_changed`. `lib/api/spin.ts` re-exports `TodayEntry`. `lib/wheel/format.ts`: `dateDot`.
- `scripts/check-entry.ts` (2701 checks).

**Task 2, `6dcd7f3` (components only, no route).**
- `components/entry/`: `StarInput`, `StarDisplay`, `ReactionPicker`, `PhotoField`, `CategoryHeader`,
  `UnderlineField`, `useEntryDraft`, `EntryForm`. A comment-only edit in `lib/theme/tokens.ts` (the
  round-shape comment names the two entry files).

**Task 3, `def647f` (the route, Log it, Posted and Skipped).**
- `app/(app)/today/log.tsx`: the entry screen. Reads Today from the store once when it opens
  (`useState(() => ...)`), keeps that spin (date, number, category name, late or not) for the visit; a
  missing spin or a state that is not Result or Late shows 'There is nothing to log right now.' and a
  'Back to Today' button. `CategoryHeader` plus `EntryForm` inside `Screen scroll={false}` (all four
  edges). The leave guard is Edit wheel's.
- `components/entry/useCreateEntry.ts`: one save (see Departures 3 and 4 for where messages go).
- `components/today/DayActions.tsx` (the late line, 'Log it'), `SettledBlock.tsx` (the Posted row of
  picture 04, the struck word of picture 05), `TodayScreen.tsx` (headings 'Posted.' and 'Skipped.', the
  sentence under Skipped, DayActions under the stamp row, SettledBlock in place of the word block, the
  cross-fade and the one-time announcement).
- `app/(app)/today/_layout.tsx`: `<Stack.Screen name="log" />`. `app/(app)/_layout.tsx`: the hide list
  is now `['notifications', 'entry', 'log', 'edit-entry']` (one hunk, nothing else in that file).
- `components/entry/EntryForm.tsx`: a small edit (Departure 2).
- Not edited: `lib/wheel/todayStore.ts`, `lib/wheel/useToday.ts`, `components/nav/SpinTabButton.tsx`,
  `components/ui/PrimaryButton.tsx`, `components/ui/TextField.tsx`, `lib/api/client.ts`.

## The three things found out first (start rule)

1. **How the store holds Today and how a server answer goes in.** `lib/wheel/todayStore.ts` is a module
   store read with `useSyncExternalStore`, keyed by the signed-in user id (one `TodaySnapshot`: user id,
   `today`, the countdown `anchor`, `loadFailure`, `requesting`, `spinFailure`, `spinning`, `pendingSpin`,
   `lastReportedZone`, `turn`, `refetchOnLanding`). `readToday(userId)` reads it outside React.
   `applyToday(userId, today, mark, extra?)` puts a server Today in; `mark` is `performance.now()` taken
   when that request started. It returns false and writes nothing when the store belongs to another
   account, so a late answer from a signed-out session is dropped. It is last write wins (no comparison
   of marks): a `getToday` that began before the post can answer after it and overwrite Posted. So the
   save applies the post's answer first and then asks for Today again (`refetchToday(userId, 'manual')`,
   which queues behind anything in flight, so its answer is the newest). Every Phase 2 load goes through
   `refetchToday` in `useToday.ts` (one queue, calls made together become one request). The store needed
   no change.
2. **How `WheelEditor` builds its leave guard and its pinned Save.** One `KeyboardAvoidingView` (padding
   on iOS, no offset) holds a `ScrollView` (`keyboardShouldPersistTaps="handled"`) and, as a sibling after
   it, the pinned footer (failure line, then `PrimaryButton`). The guard: `hasChangesRef.current` is set on
   every render; `navigation.setOptions({ gestureEnabled: !hasChanges })` in an effect; a `beforeRemove`
   listener that returns when `allowLeaveRef.current` or nothing is unsaved, else `preventDefault()`s,
   keeps the action, calls `clearPendingSpin(userId)` and opens the `ConfirmSheet` ('Leave without
   saving?' / 'Your changes will be lost.' / 'Leave' / 'Keep editing'). The listener reads refs, so it
   sees the newest values. 'Leave' sets the allow ref and dispatches the kept action; a successful save
   sets the allow ref before going back. `log.tsx` copies this, reading `draft.isUnsaved()` (a ref
   reader) inside the listener and `draft.unsaved` for `gestureEnabled`.
3. **Which functions switch on `Today.state`.**
   - `TodayScreen.tsx`: `isResultState` (already listed all four states), the `comp` line (uses it),
     `headingFor` (posted and skipped had a Phase 2 stand-in "Today's spin."; it now has one case per
     state, so it is total), `renderStateBlock` (it fell through to the Open block for any other state;
     now `open` is explicit and Result, Late, Posted and Skipped get an empty block), `hubOpen` (open
     only), `editLinkShown` (waiting or open, wheel drawn only), and the `late` flag given to
     `ResultBlock`. No other component reads the state (`grep "'late'" components lib app` and a state
     grep found nothing else).
   - `lib/wheel/useToday.ts`: only `afterInstantLoad` (checks `waiting`) and `consumePendingSpin` (checks
     `open`). The triggers (focus, foreground, the timer armed from `next_open_at` and `window.closes_at`)
     do not look at the state, so Posted and Skipped already behave like Result. No edit.
   - `components/nav/SpinTabButton.tsx`: solid only when `state === 'open'` and not spinning; the hint is
     'Spins today's wheel.' for open, 'Shows the wheel. It opens at 8:00 PM.' for waiting, else 'Shows
     today's spin.'. Posted and Skipped already get the locked look and the right hint. No edit.

## Where it differs from the paper

Small, and the intent stands in each.

**From Task 1** (`92f3c27`):
1. The Today types live in `lib/api/types.ts` (spin.ts re-exports), not in `spin.ts` as the plan says.
   `WeekDay.skipped` was optional ("Phase 3 only") and is now required; read it as `day.skipped === true`
   and `!today.entry`, because a Phase 2 server omits both keys.
2. `formatStars` below 1 gives '' (no figure). `stepStars` from below 1: a positive step gives the step
   clamped to 1 to 10, any other step stays 0. `starFromTap` brings a star number outside 1 to 5 back
   into range. Extra export: `STAR_COUNT` (5).
3. `nextSpinLine` with equal dates but a null or blank weekday gives the no-weekday line.
4. `photoProblem` checks the type first, then the size; a missing type counts as `image/jpeg` (as on the
   profile page), an empty string is a wrong type. The caller must use the same default for the upload.
5. The upload memory keeps the in-flight promise (two calls at once share one upload), drops a failed
   attempt only if it is still the stored one, `forget(uri)` drops one file and `forget()` all. The draft
   never adds one uri twice (the server refuses a repeated key).
6. `uploadEntryPhoto`: a refused ticket (400 `validation_failed` or 413) becomes `photo_rejected`, as
   `uploadAvatar` does, whatever `err.field` says.

**From Task 2** (`6dcd7f3`):
1. `useEntryDraft` returns three things the paper did not name: `unsaved`, `isUnsaved()` (a ref reader,
   because `markSaved()` then `router.back()` runs in one tick) and `existingPhotoId(uri)` (edit mode: the
   PATCH needs `{ id }` for a kept photo). In edit mode an existing photo is a `PickedPhoto` whose `uri`
   is its url.
2. The draft starts once from `initial` and re-seeds only when a different entry id arrives; the same
   entry reloading never overwrites typed text. `dirty` ignores surrounding whitespace.
3. `PhotoField` clears the screen's photo message by itself when the photo list changes (the paper left
   the owner open); a new message shows again.
4. Rating row: the first star is flush with the column edge (picture 03), so the row starts 5.5dp left of
   the column. Its spoken value is 'N out of 5' or 'not rated' (UI spec 14), not `starsSpoken`.
5. Thumbnails have square corners; the cover's 2dp frame is an overlay (every image is 56dp); the remove
   mark's 44dp hit area sits inside the tile (a horizontal ScrollView drops touches outside its bounds).
6. 'Camera' and 'Library' show only on the empty panel. UI spec section 16 says they "stay as a row" after
   a photo is chosen; section 7 and plan task 2d (strip plus the plus tile with the Alert sheet) say
   otherwise, and those were followed.
7. The Rating label row is a fixed 28dp high so the figure appearing does not move the label. 'Still
   needed' and the failure line are centred above the button (Edit wheel centres its save message).
8. The comment scrolls itself into view (scrollToEnd on focus and on layout change while focused): not in
   the paper, unverified on a device. `EntryForm` takes `onTerminal` as its own prop and accepts but does
   not read `initial`.
9. `UnderlineField` has a `count` prop ('focused' or 'always') and `countJoin` ('/' or ' / ').

**From Task 3** (`def647f`):
1. **'Log it' has no spoken hint.** UI spec 14 gives it 'Opens the entry screen.'. `PrimaryButton` takes
   no hint prop and Task 2's gate keeps that file untouched, so the hint is missing. One optional prop on
   `PrimaryButton` would fix it (not done here).
2. **`EntryForm.tsx` got a small edit** (it is Task 2's file): when a photo message arrives the form
   scrolls to the top. The strip is at the top of the scrolling middle, Save is in the footer, and the
   person may be down at the comment with the keyboard up, so the message would otherwise be out of view.
3. **Where a failure shows** (UI spec 3.6 and 7): a refused or failed photo (`photo_rejected`,
   `photo_upload_failed`, thrown while uploading) goes under the strip and posts nothing. A lost
   connection and every other failure, whether it came from an upload ticket or from the post, shows on
   the line above Save, so the same offline condition always lands in the same place. The four state
   answers (`day_settled`, `day_closed`, `no_spin`, `spin_changed`) show above Save, turn the button into
   'Back to Today' and ask Today again. Both messages are cleared at the start of each attempt.
4. `already: true` from the post is treated as success. After a success the button stays inert until the
   screen has gone (no second save in the pop animation), and the screen skips `router.back()` when it
   is no longer focused or mounted (Cancel then Leave while a save ran would otherwise pop the wrong
   screen). 'Cancel' does nothing while a save runs.
5. The screen's logic is in the route file `log.tsx` (no separate component file), as the plan's file list
   says. `ShownSpin` and `CreateEntryState` are exported from `useCreateEntry.ts` for the edit route.
6. **The Result to Posted/Skipped change is a real cross-fade** (UI spec 12): the old block (word, stamp
   row, DayActions) is kept as a non-interactive copy fading out over 400ms while the new block fades in.
   It runs only when the change happens while the screen is open and not under Reduce Motion; on first
   load there is no animation. This needed `FadeBox` to take a `grow` prop. The bonus-spin 'entering'
   overlay now also contains DayActions (static), so 'Log it' does not vanish before the word fades.
7. `resultComposition` became `dayComposition(day, live)`. `DayActions` takes `late`, `followsLink` (a
   bonus link or a message sits between the stamp row and it: `md` instead of `xxl`), `disabled` and
   `animate` (its rise comes 240ms after the word, after the stamp row). It pushes the route itself.
8. A failed bonus spin's message stays directly under the stamp row (above 'Log it').
9. The sentence under Skipped sits `sm` below the heading (the paper gives no number). The word block is
   centred by two flexible spaces (each at least `xl`) and Skipped adds no trailing spacer.
10. The strike line is a bar inside a wrapper that hugs the word, 4dp past it on both sides. Under
    shrink-to-fit it is within a few dp of the drawn word (not measured from the text layout).
11. The Posted row is one spoken element, 'Meal. Mushroom ramen at Kuro. 4.5 out of 5 stars. Reaction:
    Love.' (stars decorative; the thumbnail and the '+N' chip hidden). The text column holds the spoken
    group; plan 03-03's 'View entry' link goes after it as a sibling, not inside it, so it stays a
    separate button. The emoji is 18dp with `allowFontScaling` off (a named constant, there is no
    token for 18).
12. `todayStore.ts` and `useToday.ts` needed no edit (see the three things).

## Wire names assumed (not yet compared with the live server)

From `03-01-PLAN.md` Task 3b. `03-01-SUMMARY.md` and `internal/httpapi/entry.go` did not exist when
this was built; the Go tags of `internal/spin/model.go` (committed in the 03-01 work) match the Today
names, and the reaction names match `internal/spin/entry.go`.
- `POST /v1/entry-photos/upload-url` `{ content_type, content_length }` gives `{ upload_url, photo_key,
  expires_in }`; the file is PUT to `upload_url` with the same Content-Type.
- `POST /v1/entries` `{ for_date, spin_seq, title, stars, reaction?, comment?, photos: [{ photo_key,
  photo_source }] }` gives 201 or 200 `{ already, entry, today }`. `reaction` and `comment` are left out
  when empty; `stars` is the stored whole number 1 to 10.
- `Entry` carries `photos: [{ id, url, position }]`, `photo_url`, `photo_source`, `late`, `category`.
- `Today.entry` is `{ id, late, title, stars, reaction, photo_url, photo_count }` or null; each week day
  has `skipped`; `Today.state` may be `posted` or `skipped`; `next_open_weekday` for the sentence.
- Error codes `day_settled`, `day_closed`, `no_spin`, `spin_changed` (409), `not_found`, and the app's
  own `network_unavailable`, `photo_rejected`, `photo_upload_failed`.

## Types file edited

`lib/api/types.ts` was edited in Task 1 (`92f3c27`): `TodayEntry`, `Today.entry`, `WeekDay.skipped`
(now required) and three error codes. An edit to that file can reload the dev client back to Welcome, so
it is named here. No later task of this plan touched it.

## Verification (gates, last run after the Task 3 commit)

- `npx tsc --noEmit`: exit 0 (whole app).
- `node --experimental-strip-types scripts/check-entry.ts`: 'reaction names match the server list' and
  'ok: 2701 checks, no failure'. `scripts/check-wheel.ts`: 'ok: 47877 checks, no failure'. (Both print a
  harmless Node warning about the package type.)
- `components/today`, `components/entry`, `app/(app)/today`: no hex or `rgba(`, none of the banned whole word
  (the app says 'spin'; also checked in `lib/entry`, `lib/api/entries.ts`), no `Math.random`, `new Date`, `Date.now`, `Date.UTC`, no
  `radius.lg`; `radius.full` in those folders is exactly `components/entry/PhotoField.tsx` and
  `components/entry/ReactionPicker.tsx`; no 'Edit wheel' in `DayActions.tsx` or `SettledBlock.tsx`; no
  `@/lab` or `lab/` in `components/entry`.
- Emoji and symbol scan (run with `bash -c`, `--exclude-dir=lab`): exactly `lib/entry/reactions.ts`.
  Em dash scan over app, components, lib, scripts: nothing. No en dash in the files of this task.
- `ls "app/(app)/today"`: `_layout.tsx`, `edit-wheel.tsx`, `index.tsx`, `log.tsx`, `notifications.tsx`.
  Nothing was added directly under `app/(app)/` since `pre-phase3` (`board.tsx` and `diary.tsx` are older).
- `git diff pre-phase3 -- "app/(app)/_layout.tsx"`: the hide list line, one hunk.
- `git diff --stat pre-phase3 -- app/index.tsx app/_layout.tsx "app/(auth)/_layout.tsx"
  lib/session/store.ts`: nothing. `components/ui/TextField.tsx` and `PrimaryButton.tsx`: no diff.
  `package.json`, `package-lock.json`, `app.json`: no diff.
- **`lib/api/client.ts` is NOT empty against `pre-phase3`**: commit `ed34a92` (the Settings window,
  Blocked accounts) added `api.delete` and the `'DELETE'` method type. None of this plan's commits touch
  that file. Plan 03-03's scan `grep -rn "'DELETE'" lib app components` will print `client.ts` for that
  reason; the paper's intent (the entry delete is `POST /entries/{id}/delete`, no DELETE call in the
  entry code) still holds.
- App-wide `radius.full` prints seven files: `IconBadge`, `Chip` (the Settings window's file, commit
  `1fe35fe`, not this plan's), `ReactionPicker`, `PhotoField`, `DialAvatar`, `SpinTabButton`,
  `tokens.test-assert`. `radius.lg` prints Phase 2's five.
- Hot reload (one screenshot, no tap, no relaunch): Metro served the new code and the simulator, signed
  in on a Result day, showed Today with the word, its 'Spun at 8:34 PM' stamp row, the new 'Log it'
  button 48dp under it, the week section and the bar; no red screen.

## Open / left for the lead

- **Integration against the live server.** `03-01-SUMMARY.md` did not exist when this was committed (it
  was checked at the start and again just before the commit). When it appears: re-read its wire shapes
  and compare them with the list above. Only `lib/api/entries.ts` and the `body` built in
  `components/entry/useCreateEntry.ts` carry names.
- **Cold launch (Task 3 step f) is OPEN for plan 03-03 Task 2.** It was not done now: nothing was
  terminated or relaunched. Still to confirm there: Today with the bar, no Welcome flash, no Unmatched
  Route; the bar hidden on the entry screen and back on Today after Cancel.
- **Not walked on a device** (no way to tap here): the keyboard and the pinned footer; the comment
  scrolling into view and the scroll to the strip when a photo message arrives; camera and library picks
  and the Alert sheet; half-star taps; the leave sheet and the disabled swipe; saving, a retry that
  reuses photo keys, and the four terminal messages; Posted and Skipped as they look (the dev API cannot
  produce them until the cutover, so neither has been seen); the cross-fade; the strike line's length
  under shrink-to-fit; the 'Posted.' and 'Skipped.' announcements with VoiceOver; the late window.
- 'Log it' hint missing (Departure 1 of Task 3).
- `components/ui/Chip.tsx` shows in the app-wide `radius.full` scan (the Settings window's, not this
  plan's); the plan's "six files" list is now seven for that reason.
- `client.ts` differs from the tag through `ed34a92`; plan 03-03's `'DELETE'` scan needs a note.
- For plan 03-03: 'Skip this one' goes in `DayActions` (it already has `late`, `followsLink`,
  `disabled`, `animate`; it will need the shown spin and the week for the sheet text), 'View entry' goes in
  `SettledBlock`'s text column after the spoken group, and `edit-entry.tsx` can copy `log.tsx`'s guard and
  `useCreateEntry.ts`'s shape (`ShownSpin`, `CreateEntryState`). The hide list already has `edit-entry`.
