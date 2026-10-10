---
phase: 02-daily-roll
plan: 02
status: built, waiting for the cutover and the walkthrough (plan 02-03)
built: 2026-10-10
commits: 13e9a3e, 90d325b, 5dd86df, 84af689
---

# 02-02 Summary: the spin app

The app side of the daily spin, built from `02-UI-SPEC.md` revision 6 and the developer's pictures of
2026-10-10. Nothing here needed the new server: the phone shows what the server says and was exercised
against an in-app stand-in for it (see Verification).

## What was built

**Task 1, `13e9a3e` (tokens, API, pure logic).**
- `wheelMid` and six text roles (`title`, `countdown`, `wheelLabel`, `hero`, `heroSmall`, `note`) with
  13 `satisfies` pins; `Domine_400Regular` loaded; `systemFallback` has the six roles; `AppText` takes them.
- `lib/api/types.ts` holds the Phase 2 wire types (`Today`, `Spin`, `Week`, `WheelResponse`, ...) and
  the seven new error codes; `ApiError` has `reason` and `index`; `client.ts` has the two new messages.
  `lib/api/spin.ts` has the six calls and re-exports the types.
- `lib/wheel/`: `geometry.ts` (22 degree floor, large-arc flag, tones, pose and target, pointer check, label
  placement, truncation, shares), `countdown.ts` (split, spoken minutes, a hand-written RFC 3339 parser so
  nothing touches the phone's date object), `format.ts` (`dateLine`, `dateLineLong`), `timeZone.ts`,
  `todayStore.ts` (module store keyed by user id), `useToday.ts` (the request queue and the triggers).
- `scripts/check-wheel.ts`: 146316 checks, no failure (Node 26.4 runs it directly).

**Task 2, `90d325b` (components only).**
- `components/wheel/Wheel.tsx` (memo, react-native-svg and reanimated, 4 s turn, `scheduleOnRN` on landing,
  Reduce Motion read at the start of a turn), `WheelEditor.tsx` (rows, steppers, preview, pinned Save,
  `beforeRemove` leave guard, `gestureEnabled` off while changes are unsaved).
- `components/today/`: `TodayScreen.tsx`, `CountdownBlock.tsx`, `ResultBlock.tsx`, `WeeklyCard.tsx` (the
  week section), `StatusMessage.tsx`, `NotificationsScreen.tsx`, plus `CapsLabel.tsx` (see below).
- `components/nav/SpinTabButton.tsx`, `components/ui/ConfirmSheet.tsx`.

**Task 3, `5dd86df` (launch files).**
- `app/(app)/_layout.tsx` is Tabs (Home, Spin, Profile); `app/(app)/today/` is a Stack of Today, Edit wheel
  and Notifications; `spin.tsx` redirects to Today; `home.tsx` is deleted.
- `app/index.tsx` sends a finished account to `/(app)/today`, or to `/(app)/profile` while `extrasPending`.
- `ProfileForm` has `inTab` (no Back, "Saved." for 3 seconds, no double bottom inset), Continue goes to
  Today, and the first-run exits call `router.navigate('/(app)/today')` before `finishExtras()`.
- Not edited: `app/_layout.tsx`, `app/(auth)/_layout.tsx`, `lib/session/store.ts`, `profile/edit.tsx`,
  `package.json`, the Welcome wheel and `components/brand`.

## Fixes found in review (`84af689`)

- The middle Spin button now asks for the spin and then navigates. The other way round, the leave sheet's
  guard (which clears the request, decision 4) ran before the flag existed, so "Leave" would have gone to
  Today and spun.
- The "isn't open yet" and "has closed" messages now stay under the countdown when the follow-up answer
  says waiting (decision 5, UI-SPEC 4.4). They used to be replaced by the countdown after a moment.
  A day that already has a spin still shows the result with no error text.

## Where it differs from the paper

1. **Countdown cells are 56dp, not 48dp.** Domine Regular at 40dp makes the pair "00" 53.9dp wide (the
   narrowest, "11", is 32dp). A 48dp cell wrapped the pair onto two lines on the phone. The row is still
   fixed width. (`CountdownBlock.tsx`)
2. **Label fit is also capped by the rim.** `acrossRoom` is `2 x r x tan(angle / 2)` as written, and also
   never more than the rim's chord at that radius, so a word on a wedge wider than about 90 degrees cannot
   reach outside the wheel. Along, the longest line at the largest wheel is 8 letters (the paper says about 9).
   The estimate of 8.3dp a letter is the paper's; measured Work Sans capitals average 8.8dp, so a very wide
   word (W, M) can be a few dp wider than estimated. (`lib/wheel/geometry.ts`)
3. **The stamp rule is `lg` below the word's line box**, literally as written. In Domine at 100/104 that
   puts it about 58dp under the baseline of "Meal", where the developer's picture has about 26dp (their
   font is taller). A pull-up that matched the picture put the rule through names that shrink to fit
   (`adjustsFontSizeToFit` moves the baseline), so it was dropped. The word's own position matches the
   picture's 100dp from the heading.
4. **`Screen` got its `edges` prop in Task 2's commit**, not Task 3's, because Today and Edit wheel (Task 2)
   pass it and Task 2 had to compile on its own.
5. **Two files the plan does not list:** `components/today/CapsLabel.tsx` (the small-caps Label, used by the
   date line, "This week", slot A and the stamp row) and `lib/motion/useReduceMotion.ts` (the Reduce Motion
   hook and a one-time read). No new package or native module.
6. **`Today.state` is typed `waiting | open | result | late | posted | skipped`** so Phase 3 adds states
   without editing `types.ts`; Posted and Skipped draw like Result until Phase 3 gives them their own look.
7. **Under Reduce Motion nothing is held:** the answer shows the result composition at once (the wheel
   is not drawn, no 600ms calm, no rise). `Wheel` itself still jumps to the winner's pose and reports
   landing at once if it happens to be mounted.
8. **A failsafe:** if the wheel never reports landing within 6 seconds of the answer, Today lands it, so
   a stuck `spinning` flag can never block refetches.
9. **Tab bar details the paper left open:** `sceneStyle` is `dominant`, the Home and Profile items are
   pushed down 6dp (`tabBarItemStyle`) to sit where the picture has them, and the label font is set to the
   regular weight explicitly.
10. **The leave sheet rows are both Ink** (the paper names no tone for "Leave").

## Verification

Static gates (all green at the last commit): `npx tsc --noEmit` exit 0; every acceptance grep of Tasks 1,
2 and 3; radius allow-lists (five `radius.lg` files, four `radius.full`); 13 token pins; no em dash, no
emoji, no hex or `rgba` in the new screens, no "roll" word, no `Math.random` or date object in
`lib/wheel`; `package.json` and `package-lock.json` unchanged.

On the simulator (iPhone 17 Pro, dev client, Metro on 8081 serving this tree). The simulator was signed
out (Welcome) when the work started and no touch tool exists, so the app was driven with a temporary
in-app stand-in: a `fetch` replacement answering `/auth/refresh`, `/me`, `/time-zone`, `/today`, `/spin`,
`/spin/bonus` and `/wheel`, an automatic sign-in with a made-up account, and (once) a hook that opened the
bonus sheet. It was never committed and has been removed; the simulator was left signed out and its Reduce
Motion setting back off. (Two things that were tried and dropped: attaching to Metro's inspector, which
Metro refused, and a small local listener on 127.0.0.1:9099 meant to feed code to the app. The listener was
killed and its files deleted before any app code used it; the permission system refused the step that
would have wired the app to it.) What it showed:
- Waiting (layout 1a): wordmark and bell, long date line, two-line `title` heading, two-tone wheel with the
  words across the wedges, label and live countdown, Edit wheel, week section with filled, ring and small
  dots, bar with the outlined round Spin button 8dp above the rule. Close to the picture; the countdown row
  is wider than the picture's (point 1).
- Open: solid Spin button, "Tap the centre to spin", "Open until midnight.". A countdown that reached zero
  became Open by itself within a second, with nothing jumping.
- The spin: 4 s turn, the word under the pointer, 600ms calm, then the wheel fades out while the heading
  swaps and the word, the stamp row and the bonus link rise in. Result and Late look like the after-spin
  picture (huge word, rule, "Spun at 8:02 PM"). A relaunch shows the result at once.
- Bonus spin: the word fades out and the wheel fades in on the old winner, turns to a different word, and
  the reveal runs again with the bonus spin's own time.
- Wheels of 2, 3, 5, 7 and 12 categories and a 10-1-1 weighting: tones alternate, the mid grey is only on
  the last wedge of an odd count, words are across or along and none is upside down at rest.
- Offline on load and offline on spin show the plain messages with "Try again".
- Reduce Motion (set through the simulator's preferences): the spin goes straight to the result.
- Bell page: back chevron, "Notifications", "Nothing new yet.", no bar. Edit wheel: preview, rows, grey
  "Save wheel" pinned above the bar. Profile tab: edit mode, no Back, bar present, no double inset.
- First run (a made-up sign-up flag): the photo-and-bio page first, no bar, no flash of Today; doing what
  Skip for now does (Today first, flag second) went straight to Today with no flash of the edit page.
- Cold launch with keychain tokens (the restore path, against the stand-in): Loading, then Today, no
  Welcome flash, no Unmatched Route. Cold launch signed out: Welcome.

## Open

- **Case (c) of Task 3 step k is OPEN** (a brand-new sign-up through the form, `firstrun-k-p2@example.com`):
  needs the developer's hands. Group 8b of plan 02-03 covers it and that plan is not closed until it passes.
  No account was created or deleted by this plan.
- Not exercised for lack of a way to tap: the bell, back arrow and tab presses themselves; the hub tap
  (the same `spin()` ran through the Spin button's `pendingSpin` path); Save wheel, the steppers, the leave
  sheet and the Profile tab's "Saved."; the top 8dp of the Spin button above the bar (UI-SPEC section 8
  device check). The two fixes above were checked by reading and `tsc`, not on the phone. All are in the
  02-03 walkthrough.
- A signed-in cold launch against the real new server is plan 02-03's (cutover) job.
- On a phone with no bottom safe-area inset (iPhone SE) the "Spin" label under the circle would sit at
  the screen edge, because the bar is 56dp plus the inset as written.
- Android was not looked at.

## Notes for Phase 3

- `applyToday(userId, today, mark, extra?)` in `lib/wheel/todayStore.ts` is the way to put a Today from a
  post or a skip into the store (`mark` is `performance.now()` taken when that request started). Every
  Phase 2 request goes through `refetchToday`, `spinToday` and `spinBonusToday` in `lib/wheel/useToday.ts`.
- `TodayScreen.tsx` switches on state in `headingFor`, `isResultState`, `renderStateBlock` and the
  `comp` line; `SpinTabButton` is locked for every state except `open`.
- The tab layout already hides the bar for a pushed route named `entry` (`BARLESS_TODAY_PAGES`).
- `Wheel` and `WheelEditor` use `RIM_WIDTH`, `HUB_FRACTION` and the label rules from `geometry.ts`; the
  check script is the place to extend if the wheel rules change.
