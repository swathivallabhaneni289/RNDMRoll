---
phase: 2
slug: daily-roll
status: draft
created: 2026-10-07
revision: 7
approval_word: go
---

# Phase 2 - UI Design Contract (Daily Roll, "the spin")

> Design contract the Phase 2 app plan builds from. Nothing here is built. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 15 or later). The developer's pictures of 2026-10-10 are the layout source: layout 1a (`docs/reference/today-screen-1a-1b-2026-10-10.png`), the screen after the spin (`docs/reference/today-after-spin-02-2026-10-10.png`) and the skipped and entry-page screens (`docs/reference/today-skipped-and-entry-05-06-2026-10-10.png`); they replace the picture of 2026-10-07 (`today-screen-mockup-2026-10-07.png`). Revision 2: pointer sign, 22 degree floor, bar on Edit wheel, Spin button size, strings. Revision 3 (2026-10-09, rebased on the final Phase 1 code): the one-time photo-and-bio page after sign-up comes before Today with no bottom bar (sections 1, 8, 9), the Profile tab's Continue button goes to Today and Phase 1's placeholder page is removed (section 9), the "Saved." line is placed against the real Profile page (section 9), and the radius allow-list is recorded (section 12). No decision changed. Revision 4 (2026-10-09, later that day): Phase 1 removed Apple and Google sign-in completely, so the one-time photo-and-bio page follows a sign-up only (sections 1 and 9). No decision changed. Revision 5 (2026-10-10): the developer chose layout 1a of their new picture by tap and said they want the page "very similar" to it. Changes: the bell is in and opens a Notifications page (4.5); the round header photo is gone; nothing sits under the heading in any state (the line moved into the state block); the countdown reads "Opens at 8:00 PM" over the digits; "Edit wheel" sits under the state block; the weekly card becomes an open week section with new dot marks (section 6); the wheel is black and light with a light hub and a dot, and words sit across the wedge (section 3); the Spin button is a circle, outlined while locked and solid black when the spin opens (section 8); the heading and countdown use a lighter serif, which adds three text roles (section 2). Wedge widths still follow the weights. Nothing is built; the choices to check are at the end of section 13. Revision 6 (2026-10-10, later that day): the developer sent the screen after the spin and said "this is how it should look after the spin". Changes: after the spin the wheel is not drawn and the word fills its place in a large lighter serif (new role `hero`), with a rule and "Spun at 8:02 PM" under it (4.3); the "Your category" label and the "Next spin opens" lines are gone from Today; the "Edit wheel" link shows only while the wheel shows; a bonus spin brings the wheel back; the reveal and the server shape change (`spun_local_time`, `next_open_weekday`); three more text roles (`hero`, `heroSmall`, `note`). The Posted and Skipped looks and the entry page are in 03-UI-SPEC. Revision 7 (2026-10-10, evening, D-23): the developer saw Edit wheel on the phone and did not want the chances shown, and asked for one shape for everything and no grey. Every category has the same chance (weight 1), so Edit wheel has no stepper, no percentage and no intro line about higher numbers (section 7, copy in section 10); the wheel is only `ink` and `secondary` for every count, with a 1.5dp `dominant` seam between all wedges and no grey wedge (sections 2 and 3), so the `wheelMid` token of revision 5 is gone. Wedge widths are equal (section 3), not "still following the weights" as the revision 5 note says.

## 1. What this phase puts on screen

| Screen | Where | New or changed |
|---|---|---|
| Today (Home tab) | `app/(app)/today` | New. It is the signed-in landing screen. |
| Edit wheel | pushed from Today | New. |
| Notifications page | pushed from the bell on Today, `app/(app)/today/notifications` | New. Says "Nothing new yet." until Phase 5 (4.5). |
| Profile tab | the "Make it yours." page in edit mode | Moved into a tab, four small changes (section 9). |
| First-run page | the "Make it yours." photo-and-bio page (Phase 1's page 2, extras mode), on the Profile route | Phase 1's page, unchanged. Phase 2 only hides the bottom bar on it and ends it on Today (sections 8 and 9). |
| Bottom bar | Home, Spin, Profile | New. Diary and Board do not exist yet and get no placeholder tabs. |
| Phase 1 placeholder page | `app/(app)/home.tsx`: "You're in." / "Your daily spin will appear here." | Removed. Today replaces it. |

**Landing (no decision named it):** a signed-in, finished account lands on Today, with one exception: right after a sign-up it lands once on the photo-and-bio page (section 9), and Continue or Skip for now then end on Today. Signed-out flow unchanged.

## 2. Tokens added (the only new design values)

Colour. No new colour token. The wheel is drawn with existing tokens only. (Revision 5 added a medium grey, `wheelMid`; revision 7 removed it with its pin in `tokens.test-assert.ts`, D-23: the wheel has no grey wedge.)

Wedges alternate `ink` and `secondary`; the seams between wedges are `dominant`, the page colour; the rim, the hub dot, the pointer, the day dots and the top rule are `ink`; the hub is `dominant`; the small day dots and the closing rule are `divider`. Screens never write a hex value. (Revision 4's six tones `wheel1` to `wheel6` are gone: the picture shows two.)

**Label colour:** `dominant` on `ink` wedges; `ink` on `secondary` wedges.

| Wedge | Label | Contrast | AA (4.5:1) |
|---|---|---|---|
| `ink` | `dominant` | 17.32 | pass |
| `secondary` | `ink` | 15.26 | pass |

**Tones by count (a fixed rule, so a plan can pin it).** Wedge i (from 0, clockwise) is `ink` when i is even and `secondary` when i is odd, for every count from 2 to 12. There is no third tone and no grey. With an odd count the last wedge is `ink` and touches the first wedge, which is also `ink`. To keep every wedge a separate shape of equal size, a 1.5dp seam in `dominant` is drawn between ALL wedges, the same way for every count (a `dominant` stroke of 1.5dp on each wedge path), so the wheel looks the same at every count and the wrap needs no special case.

| Wedges | Tones, clockwise from the first wedge |
|---|---|
| any count from 2 to 12 | ink, light, ink, light, and so on (even place ink, odd place light) |
| 3, 5, 7, 9, 11 | the same, so the last wedge is ink and meets the first (ink); the seam parts them |

**Text roles added** (in the `type` group of `lib/theme/tokens.ts`, pinned in `tokens.test-assert.ts`, named in `AppText`; `lib/theme/fonts.ts` loads one more file):

| Role | Font | Size and line | Used for |
|---|---|---|---|
| `title` | `Domine_400Regular` | 32 / 38 | the Today heading |
| `countdown` | `Domine_400Regular` | 40 / 46 | the countdown digits |
| `wheelLabel` | `WorkSans_400Regular` | 11 / 14 | words on the wheel (letter spacing 1.5, capitals by style, never scaled with the phone's text size) |
| `hero` | `Domine_400Regular` | 100 / 104 | the result word after the spin (Result, Late) |
| `heroSmall` | `Domine_400Regular` | 80 / 84 | the word in Posted and Skipped (Phase 3) |
| `note` | `Domine_400Regular` | 20 / 28 | the comment on the entry page (Phase 3) |

`Domine_400Regular` ships in the installed `@expo-google-fonts/domine`: JavaScript only, no native module. The picture's big serif is lighter than Domine SemiBold, which is why a Regular is added, for the roles marked Regular above; the wordmark, the Heading role and the Display role stay `Domine_600SemiBold`. The type scale is therefore no longer "closed at 16, 14, 22 and 40" (D-12) for these six roles. `heroSmall` and `note` are first used in Phase 3 and are defined here so the tokens are edited once. 11dp is below the Label size and is allowed only for words on the wheel, which are also spoken in full and shown in full on the result.

## 3. The wheel

**Words only.** Every wedge carries its category name and nothing else. Names show in capitals by text style only.

**Size.** Diameter is the smaller of 232dp and the screen width minus 48dp (revision 4: 296dp). The rim is a 4dp `ink` ring. Between all wedges there is a 1.5dp seam in `dominant` (section 2), drawn as a `dominant` stroke on every wedge path: the part of that stroke along the outer arc lies under the rim, and the hub covers the centre. The hub is a `dominant` circle, 19 percent of the diameter (44dp at 232), with an `ink` dot 7 percent of the diameter (16dp at 232) in its middle: no ring and no star. The hub and the pointer never rotate; the wedges and labels do.

**Pointer.** An `ink` triangle, apex down, 16dp wide and 12dp tall, centred at 12 o'clock. Its tip touches the rim's outer edge. It sits outside the disc, so it is always `ink` on `dominant` (17.32:1).

**All wedges are the same size** (the developer's request on 2026-10-10, after seeing the weighted wheel: "why some are too big and some are too small, make them all equal").
- Every wedge is `360 / n` degrees (60 each for six, 30 each for twelve).
- Every category has the same chance (D-23): the app gives each category weight 1 and shows no number or chance figure anywhere (section 7).
- Wedges run clockwise in list order; the first starts at angle 0 (12 o'clock).
- Starting wheel, in this order: Meal, Movie, Song, Book, Place, Game: six equal wedges of 60 degrees, each with the same chance (one in six). This replaces the weighted start of D-01 and D-05 (Meal 3, Song 3, Place 2 and one each for the rest); the server's starting weights are set to 1 in the server's own change. An account made before that change keeps its old numbers on the server until its next Save in Edit wheel, which sends 1 for every category.

**Labels.** One rule for 2 to 12 wedges, in the `wheelLabel` role (11/14, letter spacing 1.5, capitals by style, `allowFontScaling={false}`). Labels turn with the wheel.
- **Across** (as in the picture): the word runs across the wedge, perpendicular to its centre line, with its middle at 70 percent of the radius.
- **Along:** the word runs on the wedge's centre line, from 12dp outside the hub to 10dp inside the rim.
- **Which:** across when the word fits, else along. The test is plain arithmetic with no text measuring: an estimated width of `n x 8.3` dp plus 4dp, against the room `2 x r x tan(angle / 2)` at `r` = 70 percent of the radius. A word that fits neither way is cut with an ellipsis (about 9 letters along, at the largest wheel).
- **Never upside down.** After the direction is chosen, a word whose baseline would point to the left in the pose the wheel stops in is turned 180 degrees. This is decided before the spin starts and again at every rest placement. (The picture's MEAL is upside down; that is not copied.)
- Label colours are the lookup in section 2. The full name is in Edit wheel, the result and the spoken label.

**Rest position.** With no result, the first wedge's centre is under the pointer, as in the picture. The wheel is drawn only before the spin, while it turns and during a bonus spin (4.3); when it lands, the winning wedge's centre is under the pointer until the reveal takes the wheel away.

**Spin geometry.** Angles run clockwise from 12 o'clock and the disc turns clockwise, so the pose that brings a wedge centre `c` to the pointer is `-c`; the target is `-c` of the winning wedge. Final rotation = `ceil((current + 1800 - pose) / 360) x 360 + pose`, so the wheel turns at least five full times, clockwise, and stops with the centre of the server's wedge under the pointer. The word under the pointer is always the word shown big. The server picks and stores the category first; the app only animates to it.

**Which wheel is drawn.** Before the first spin of the day, the live wheel. While a spin turns, the wheel the server used for that spin, so pointer and result agree even after an edit. A bonus spin uses that same wheel. Spin responses carry it. After the spin the wheel is not drawn at all (4.3).

## 4. Today screen

**Layout, top to bottom.** Side margins `lg` (24). The page scrolls inside `Screen`; only the bottom bar is fixed. The layout source is picture 1a (`docs/reference/today-screen-1a-1b-2026-10-10.png`, the centered wheel). It has no doodle, photograph or friends card, so nothing is cut from it.

1. **Header row**, 44dp high. Left: the wordmark "RNDMRoll" as text, Heading role (22/28, `Domine_600SemiBold`), `ink`. Right: the bell, Ionicons `notifications-outline`, 24dp, `ink`, hit area 44dp, spoken label "Notifications". It opens the Notifications page (4.5). There is no unread dot in Phase 2 and no profile photo (Profile is on the bar).
2. **Date line**, `md` below the header row. The local date written in full, for example "THURSDAY, OCTOBER 8", Label size, `muted`, letter spacing 1.5, capitals by style. The date is the server's local date.
3. **Heading**, `sm` below. The `title` role (32/38), `ink`, left, sentence case. Nothing sits under it in Waiting, Open, Result and Late: the line that revision 4 put there now sits in the state block (item 5). Posted and Skipped (Phase 3) have one line under it, as in picture 05 (4.1).
4. **The wheel**, `lg` below, centred, with the pointer on top, in Waiting, Open and Spinning. After the spin (Result, Late, Posted, Skipped) it is not drawn and the word takes its place (4.3).
5. **State block**, `lg` below the wheel, centred: slot A (a small-caps Label), slot B (the big line) and slot C (a Label line), see 4.1; then the "Use bonus spin" link when it applies, then Phase 3's actions when it adds them. In Result and Late there is no small label and no big line: the word, the stamp row and the buttons of 4.3 replace the block.
6. **Edit wheel link**, last in the block, `sm` below it: centred, Label size, `ink`, `WorkSans_400Regular`, underlined, 44dp hit area. It shows only while the wheel is drawn (Waiting, Open and Offline on spin) and is disabled while the wheel turns. It is not in Result, Late, Posted or Skipped (picture 02), so a wheel is edited before the day's spin or after midnight.
7. **Week section**, `lg` below the link, full width (section 6).
8. `xl` of space after the closing rule, clear of the bar.

The heading, the wheel and the wheel's position are the same in every state, so nothing jumps when the state changes among Waiting, Open and Spinning; only the block under the wheel changes height. After the spin the word block takes the wheel's place (4.3). The picture's heading is about 32dp in a lighter serif than Domine SemiBold, so the heading and the countdown use `Domine_400Regular` (section 2); the result word uses the `hero` role.

### 4.1 States

The phone shows what the server says and never works out the date, the window or the winner. Slots: A is a small-caps Label (`muted`, letter spacing 1.5, capitals by style), B is the big line, C is a Label line (`muted`).

| State | When | Heading | Slot A | Slot B | Slot C | Wheel | Spin button |
|---|---|---|---|---|---|---|---|
| Loading | first fetch | none | none | none | Loading your wheel. | rim only, no wedges | locked |
| Waiting | before 8:00 PM local, or the day was missed | Your next spin is coming. | Opens at 8:00 PM | the countdown (4.2) | none | still | locked |
| Open | 8:00 PM to local midnight, not spun | Your spin is open. | Tap the centre to spin | none | Open until midnight. | still, hub tappable | open |
| Spinning | from the answer arriving until 4.0 s later | Your spin is open. | Spinning | none | none | turning | locked |
| Result | spun today | Today's spin. | none | the word, `hero` (4.3) | the stamp row "Spun at 8:02 PM" | not drawn | locked |
| Late | after midnight, before the next 8:00 PM, spin not posted | Last night's spin. | none | the word, `hero` | the stamp row | not drawn | locked |
| Posted | Phase 3 only: the shown spin's day has a post | Posted., with the sub-line "Your next spin opens Friday at 8:00 PM." under it | Today's spin was | the word, `heroSmall` | the posted block (03-UI-SPEC section 3.3) | not drawn | locked |
| Skipped | Phase 3 only: the shown spin's day was skipped | Skipped., with the same sub-line under it | Today's spin was | the word, `heroSmall`, struck through | none | not drawn | locked |
| Offline on load | first fetch fails | none | none | none | message and "Try again" (4.4) | not drawn | locked |
| Offline on spin | the spin request fails | Your spin is open. | Tap the centre to spin | none | message and "Try again" | still | open |

Notes:
- Phase 2 builds no button on Result or Late. Phase 3 adds 'Log it', 'Skip this one' and the late line there, and the Posted and Skipped states (03-UI-SPEC sections 3.1 to 3.5). Posted and Skipped are the only states with a line under the heading (picture 05); its words are in 03-UI-SPEC.
- The wheel is drawn in Waiting, Open, Spinning and Offline on spin only. In Result and Late it is not drawn, and a bonus spin brings it back for the spin (4.3).
- When the day closes while the screen is open, the screen asks the server again and shows Late if today's spin is not posted, else Waiting. A spin tapped after close gets the "closed" message in 4.4.
- The hub is tappable only in Open (disabled, no press effect in Waiting). The Spin button's two looks follow the last column (section 8).
- Today asks the server again on foreground, on tab focus, when a countdown reaches zero, and at the server's `next_open_at` and close time (also in Late, which has no countdown). Plan 02-02 decision 5 has the retry rules.

### 4.2 Countdown (Waiting)

Two lines, centred: slot A, the Label "Opens at 8:00 PM" (capitals by style, letter spacing 1.5, `muted`); slot B, the digits "05:04:01" in the `countdown` role (`Domine_400Regular` 40/46), `ink`. Hours, minutes and seconds each sit in their own fixed-width cell (48dp, centred) with a colon between, so the row never changes width as digits change. The time is always written 8:00 PM (a product constant). The countdown runs to the server's open time, measured from the server's "now", so a wrong phone clock changes nothing. VoiceOver reads "Opens at 8:00 PM, in 5 hours 4 minutes", updated once a minute, never per second.

### 4.3 Result block (after the spin)

The wheel is not drawn after the spin: the word takes its place (picture 02, `docs/reference/today-after-spin-02-2026-10-10.png`). Under the heading, top to bottom:
1. **The word**, on the `lg` margin, left aligned, `hero` role (`Domine_400Regular` 100/104), `ink`, as the person wrote it (not forced to capitals): one line that shrinks to fit the column (`adjustsFontSizeToFit`, minimum scale 0.36, about 36dp) and is cut with an ellipsis if it still does not fit. 96dp (`xxxl` plus `xl`) below the heading. There is no label above it and no sub-line.
2. **Stamp row**, `lg` below the word: a 1dp `ink` rule that fills the width at the left, a `md` gap, then the Label "Spun at 8:02 PM" (capitals by style, letter spacing 1.5, `muted`) at the right. The time is `spin.spun_local_time`, written by the server in the person's zone; the phone does no time maths. After a bonus spin it is the bonus spin's time.
3. Only if a bonus spin is held and the day is not settled, `md` below: the text link "Use bonus spin" (Ink, underlined, `WorkSans_600SemiBold`, 44dp high), centred.

Nothing else in Phase 2: no button, no share, no "Edit wheel" link, no countdown. Phase 3 adds, under the stamp row, the late line (Late only), 'Log it' and 'Skip this one' (03-UI-SPEC section 3.2). The week section follows at the bottom after a flexible space of at least `xl`; on a tall phone its top rule sits low, about 100dp under the last control. How the word arrives is in section 5, step 4. Reopening the app after a spin shows this layout at once, with no wheel and no animation.

**Bonus spin sheet.** The link opens a bottom sheet. It reuses the log-out sheet's modal, scrim, top corners and padding. Title (Heading role): "Use your bonus spin?". Body (Body role): "You will spin again and get a different word. This uses up your bonus spin." Two full-width rows, each at least 56dp high, separated by hairlines: "Spin again" and "Keep this word". "Spin again" closes the sheet, brings the wheel back (the word block fades out and the wheel, resting on the old winner, fades in over 400ms) and runs the Spinning state; the reveal of section 5 follows. The server never returns the same category. After it, the link is gone.

### 4.4 Messages

One plain Label line in `destructive`, centred, with a standard `ink` "Try again" button (the existing primary button, 8dp radius). In Offline on load the wheel area holds the message instead of a wheel.

| Case | Message |
|---|---|
| No connection (load or spin) | Couldn't connect. Check your connection and try again. |
| Spin before the window opens (clock mismatch) | Your spin isn't open yet. It opens at 8:00 PM. |
| Spin after local midnight | Today's spin has closed. The next one opens at 8:00 PM. |
| Anything else | Something went wrong. Please try again. |

A retry after a lost connection returns the same spin, so a spin cannot be lost or doubled. After the "isn't open yet" and "closed" messages, "Try again" asks the server for Today again instead of sending a new spin. If the day already has a spin, the screen shows it (Result) with no error text.

### 4.5 Notifications page

Opened by the bell. A page in the Today stack (`app/(app)/today/notifications.tsx`, component `components/today/NotificationsScreen.tsx`), pushed over Today with a Back control (the chevron alone, no label, as in the developer's Notifications picture, `docs/reference/screens-11-notifications-12-profile-2026-10-10.png`). There is no bottom bar on this page, as in that picture: the Today tab screen hides it while the focused route is `notifications`, and it is back when the person returns to Today. Top to bottom, `lg` side margins: the heading "Notifications" (`title` role, 32/38), then the Label line "Nothing new yet." (`muted`). Nothing else: no list, no settings, no sample row. Nothing in Phase 2 can create a notification, so the page is always empty. The real list (friend activity and comments) and phone alerts are Phase 5 (STATE.md, Phase 5 notes). Without this page the bell would have nowhere to go, which breaks the no-dead-controls rule.

## 5. Spin motion

Wait, spin, anticipation, reveal: calm before, life only during the spin, quiet after.

1. **Tap.** The hub (or the middle Spin button) presses in to 0.97 scale over 90ms. The request goes out. The wheel does not move until the answer arrives, normally well under a second.
2. **Turn.** From the answer, 4000ms, `withTiming` with `Easing.bezier(0.3, 0, 0.1, 1)`: smooth start, fastest in the first second, then a steady slow-down that creeps into place. No bounce, overshoot, flash, confetti, sound or vibration. The numbers are a starting point for the device check.
3. **Settle.** The wheel stops on the winning wedge. Nothing else moves for 600ms.
4. **Reveal.** The wheel and its pointer fade out over 400ms while the heading swaps with the existing 400ms fade. The result word (`hero`) rises 14dp into place while fading in (existing `FadeInUp`: 400ms, ease-out cubic). The stamp row follows 120ms later, then the bonus spin link or Phase 3's buttons 120ms after that.
5. **Done.** Nothing else animates. No screen change.

**Reduce Motion on:** no turning, rise, fade or press scale. The wheel is not shown at all: the result layout (heading, word, stamp row) appears at once. The countdown text still updates.

## 6. Week section

**Not tappable, and not a card.** Phase 2 has nowhere for it to go (the diary is Phase 4), and a control with no destination breaks the rule of no dead controls. So the picture's arrow is gone and the section has no press state and no role of "button". It has no fill, border, radius or shadow: it is an open section between two rules, as in the picture. (Revision 4 drew a card with a `fieldSoft` fill.) The component is still `components/today/WeeklyCard.tsx`, because Phase 3 names it.

**Content, top to bottom, full width inside the `lg` margins:**
1. A 1dp `ink` rule.
2. `md` below: one row. Left, the Label "This week" (`muted`, letter spacing 1.5, capitals by style). Right, "{n} of {goal} this week", for example "2 of 4 this week": the number `{n}` in the Heading role (22/28), the rest in the Label role, `ink`, on one baseline, right aligned. The numbers come from the server; with no posted days it reads "0 of 4 this week".
3. `sm` below: seven day columns, Monday to Sunday, evenly spaced: a Label letter above a 12dp mark slot. The letters come from the server (M T W T F S S now) and are always `muted`, today's too. The marks are drawn shapes (react-native-svg), not controls. In order of precedence:

| Day | Mark |
|---|---|
| Posted | filled `ink` dot, 10dp |
| Skipped (Phase 3 only; Phase 2 never shows one) | `ink` dash, 10dp wide and 2dp tall, square ends (03-UI-SPEC section 8) |
| Today, not posted, not skipped (the server's `week.today_index`) | ring in `ink`, 10dp, 1.5dp stroke |
| Any other day: still to come, or past with nothing | small dot in `divider`, 5dp |

4. Only while a bonus spin is held, `sm` below: the Label "You have a bonus spin." (`muted`).
5. `md` below: a 1dp `divider` rule closes the section.

The phone tells which day is today from `week.today_index`, which the server already sends (plan 02-01 Task 3b); it does no date maths.

**Removed from picture 1 and from revision 4:** the flame, "day streak", "Keep it going.", the arrow, the card, "Goal: {goal} posted days a week." and "Goal met this week." Nothing can be posted in Phase 2, so the weekly count is the only truthful figure; posted dots come only from the test setting's pretend days.

## 7. Edit wheel screen

Opened by the "Edit wheel" link under the state block on Today (section 4, item 6: Label size, `ink`, underlined, 44dp hit area). It is disabled while the wheel is spinning. The screen is pushed over Today with a Back control (chevron and "Back", the same as on the Log in page; the Profile tab no longer has one). The bottom bar stays, and every way out with unsaved changes (Back, swipe, a Home or Spin tab press) opens the leave sheet.

**Layout, top to bottom, scrolling.**
1. Heading "Edit wheel" (Heading role).
2. Only if today's spin exists, a line: "Changes show on your next spin." (Label, `muted`). Otherwise nothing sits under the heading.
3. **Live preview**, 240dp, centred: the same wheel as Today (section 3), no pointer, not tappable, redrawing as you change anything. Labels are cut to fit.
4. **Category rows**, one per category, each at least 56dp high, separated by hairlines (`divider`). A row is one line:
   - the name in a text field (existing input style: `md` radius, `inkRest` border, 48dp high, up to 24 characters; editing it is the rename),
   - a "Remove" text link at its right, in `destructive` (44dp hit area).

   A row has no stepper, no number and no chance figure: every category has the same chance (D-23).
5. **Add row**: a text button "Add a category" with an Ionicons `add` glyph (44dp high). It adds a last row with an empty focused name and weight 1. At 12 categories it is replaced by the Label "You can have up to 12 categories."
6. A Label, `muted`: "Removed categories stay in your past spins."
7. **Save**, the existing primary button "Save wheel", pinned above the bar and the keyboard, grey (existing disabled treatment) until something has changed and every row is valid.

**Limits (the screen enforces them; the server checks again).**
- 2 to 12 categories. At 2, every "Remove" is disabled and the Label "A wheel needs at least 2 categories." shows under the rows.
- Name: 1 to 24 characters after trimming spaces. No two names the same, ignoring capitals. The field stops at 24 characters.
- Weight: not shown and not editable. It is always 1 (D-23): a category the person adds has weight 1, and Save sends weight 1 for every category, new and saved, so a wheel saved earlier with other numbers is made even by its next Save. (The server still accepts 1 to 10.)
- Row errors (Label, `destructive`, under the field, shown on leaving the field or on Save): "Give this category a name." and "You already have a category with this name."

**Behaviour.**
- Changes stay on the phone until Save, which sends the whole list in one request (all or nothing). Success returns to Today with the wheel redrawn. Opening the screen changes nothing: Save stays grey until a name is added, renamed or removed, so an older wheel keeps its numbers on the server until then.
- "Remove" on a saved category archives it (it keeps its name in old spins). On a row added in this visit it simply deletes the row.
- Leaving with unsaved changes opens a sheet (same reused parts as 4.3). Title: "Leave without saving?" Body: "Your changes will be lost." Rows: "Leave" and "Keep editing".
- If Save cannot connect: "Couldn't connect. Check your connection and try again." in `destructive` above the Save button. Other server refusals use "Something went wrong. Please try again."
- The preview redraws at once on every change, with no animation, whatever the Reduce Motion setting.

## 8. Bottom bar

Three items: Home, Spin, Profile. `dominant` fill, 1dp `divider` top border, 56dp plus the bottom safe area. Nothing for Diary or Board.

**Hidden on the first-run page.** While the photo-and-bio page (section 9) is on screen the bar is not drawn at all. That page is still part of signing up, it has no Back and no Log out, and a bar would let the person step sideways past it. The bar appears with Today. (Decided as suggested on 2026-10-09; the developer can flip it in the walkthrough.)

| Item (Ionicons, Label size) | Idle | Active |
|---|---|---|
| Home | `home-outline`, `muted` | `home-outline`, `ink` |
| Profile | `person-outline`, `muted` | `person-outline`, `ink` |

Icons are 24dp inside a 44dp hit area; labels are Label size (14). The picture uses outline glyphs only, so the active tab is shown by colour alone and the label font stays the same. No bouncing icons.

The pictures of 2026-10-10 (screens 02 and 05) show five tabs: Home, Diary, Roll, Board and Profile, the final bar. Phase 2 and 3 build the three above; Diary arrives in Phase 4 and Board in Phase 5, and the middle stays the Spin button. The pictured word "Roll" and its light grey filled circle with an asterisk are not used (spin wording; the looks below are the developer's own words of 2026-10-10).

**Spin button, middle.** A circle, 56dp (`radius.full`, a named exception, see Round shapes), its top edge 8dp above the bar's top edge. The label "Spin" sits under it, `muted` in both looks. It is an action and never shows as the selected tab. It has two looks, chosen by the server's state:
- **Locked** (every state except `open`: Loading, Waiting, Spinning, Result, Late, Posted, Skipped, Offline on load): `dominant` fill, a 1.5dp `ink` outline, and inside it the app's own wheel mark (`BrandMark`, the developer's logo) at 28dp in `ink` with `dominant` wedges. Locked is not dimmed and not disabled; it only says that no spin can start right now.
- **Open** (state `open`, which includes Offline on spin): solid `ink` fill, no outline, the same mark inverted (`dominant` disc, `ink` wedges).

There is no shadow or elevation in either look, and no animation between them (with or without Reduce Motion): the fill simply changes when the state does. The top 8dp above the bar must still be tappable (device check).

What a press does:
- Open and unspun: goes to Home and starts the spin once Today has loaded and is still open.
- Waiting, Result or Late (and Phase 3's Posted and Skipped): goes to Home and shows the wheel. Nothing spins.
- Spoken hint: Open "Spins today's wheel." Waiting "Shows the wheel. It opens at 8:00 PM." Otherwise "Shows today's spin."

**Round shapes.** The Spin button is a circle: the one round tappable of this phase, a named exception (the developer's picture of 2026-10-10 replaces their 2026-10-08 pick of a dark rounded square). The header photo is gone, so the photo circle on Make it yours and Profile (a Phase 1 named exception) is the only other one. The visible hub is a circle in the wheel drawing with no button styling; the tappable part is a transparent 64dp square (8dp radius) over it. Day dots and the hub dot are drawn shapes. Nothing else tappable is round, and nothing is a pill.

## 9. Profile tab and the first-run page

The Profile tab shows Phase 1's "Make it yours." page in edit mode (Name, Username, Bio, the photo, one bottom button, Log out), strings unchanged, with four changes because it is now a tab, not the end of a stack:
1. No Back control, whatever the navigation history says (inside tabs, "back" would jump to another tab). The empty left side keeps the "RNDMRoll" label and rule where they are.
2. After "Save changes" the person stays on Profile (Phase 1 went back a page when there was one). Nothing is left to save, so the bottom button swaps back to "Continue", and the Label "Saved." shows under it in `muted`, centred, for 3 seconds. It goes away at once if anything is edited again.
3. No bottom safe-area inset on the page (the bar adds it).
4. The bottom button keeps both Phase 1 states: "Save changes" once a field or the photo has changed, "Continue" while nothing has. "Continue" now opens Today. Phase 1 opened a temporary page ("You're in." / "Your daily spin will appear here."), which Phase 2 removes with its route; the developer asked on 2026-10-08 for a Continue that goes "to the next phase", and this is it.

Log out, its sheet, the username check and the photo picker are unchanged.

**First run (the same page in extras mode).** Right after a sign-up, the Profile route shows Phase 1's page 2 once: "Make it yours." with its sub-line, the camera circle, the Bio box, Continue and "Skip for now". Phase 2 changes only what is around it:
- The bottom bar is hidden on it (section 8). The page keeps the default safe-area insets, because no bar adds the bottom one.
- Continue (after its save) and Skip for now both take the person to Today, and the bar appears. If the photo upload fails the person stays on the page with Phase 1's message under the circle, and can retry or skip.
- It is shown once. The flag behind it lives in memory only, so a relaunch goes straight to Today and the page never comes back. A returning person who logs in, and an expired session that refreshes, also go straight to Today.
- All of its strings are Phase 1's. Phase 2 adds none.

## 10. Copy contract

Every string the person sees. "Spin" wording, plain words, no em dashes, no legal text. New strings are marked N.

| Where | String |
|---|---|
| Wordmark (brand name) | RNDMRoll |
| Date line | THURSDAY, OCTOBER 8 (example) |
| Link on Today | Edit wheel (N) |
| Bell (spoken label) | Notifications (N) |
| Notifications page | Notifications / Nothing new yet. / Back (N) |
| Heading, waiting | Your next spin is coming. |
| Label above the countdown | Opens at 8:00 PM (N) |
| Label, open | Tap the centre to spin (N, no full stop: it is a small-caps label) |
| Heading, open and spinning | Your spin is open. |
| Heading, result | Today's spin. |
| Heading, late | Last night's spin. |
| Stamp row, result and late | Spun at {time}, for example Spun at 8:02 PM (N, capitals by style; the time comes from the server) |
| Countdown | 05:04:01 (example), under the label above |
| Line, open | Open until midnight. |
| Label, spinning | Spinning |
| Loading | Loading your wheel. |
| Bonus spin link | Use bonus spin |
| Bonus spin sheet | Use your bonus spin? / You will spin again and get a different word. This uses up your bonus spin. / Spin again / Keep this word |
| Week section | This week (N) / {n} of {goal} this week / You have a bonus spin. / M T W T F S S (letters from the server) |
| Errors | Couldn't connect. Check your connection and try again. / Your spin isn't open yet. It opens at 8:00 PM. / Today's spin has closed. The next one opens at 8:00 PM. / Something went wrong. Please try again. |
| Retry button | Try again |
| Edit wheel | Edit wheel / Changes show on your next spin. / Remove / Add a category / You can have up to 12 categories. / A wheel needs at least 2 categories. / Removed categories stay in your past spins. / Give this category a name. / You already have a category with this name. / Save wheel |
| Leave sheet | Leave without saving? / Your changes will be lost. / Leave / Keep editing |
| Bottom bar | Home / Spin / Profile |
| Profile tab | Saved. (the only new string; Continue, Save changes and everything on the first-run page are Phase 1's) |
| Removed with the placeholder page | You're in. / Your daily spin will appear here. (Phase 1 strings, still on Phase 1's keep-or-change list; Phase 2 deletes the page they sit on) |
| Starting category names | Meal, Movie, Song, Book, Place, Game |

The existing offline string and generic fallback are reused. New server error codes join the API error list; only "spin not open" and "spin closed" get their own strings above, the rest use the generic one.

## 11. Accessibility

- **Wheel.** One non-interactive element, label "Wheel with {n} categories: {names in order}". Hidden from VoiceOver during a spin; "Spinning." is read instead.
- **Hub.** A button, label "Spin the wheel", hint "Opens at 8:00 PM." while waiting (state disabled). Hit area 64dp. Wheel labels, tab labels and countdown digits do not scale with the phone's text size.
- **Result.** When the spin lands, announce "Your spin is {name}." with `announceForAccessibility`. The result word is a header element with the label "Today's spin: {name}." (late: "Last night's spin: {name}."); the stamp row is one element, "Spun at 8:02 PM"; the wheel is not in the tree once it is gone. Under Reduce Motion the announcement is made when the result appears.
- **Week section.** One element: "{n} of {goal} this week." (then "You have a bonus spin." when held), then each day by the server's weekday name, "Monday posted, Tuesday not posted" (Phase 3 adds "skipped"), and so on. Filled against ring against small dot, never colour alone.
- **Bell.** A button, label "Notifications", hint "Opens your notifications.", hit area 44dp.
- **Spin button.** A button, label "Spin", with the hint of section 8. Its two looks are not told by colour alone: the hint says when it opens.
- **Edit wheel.** Name field "Category name". "Remove {name}". There is no stepper and no number to read. The preview is hidden from VoiceOver.
- **Profile tab.** The "Saved." line is a polite live region, so VoiceOver reads it once when it appears.
- **Targets.** Every tappable is at least 44dp (hub 64, Spin button 56, bell 44, rows 56).
- **Contrast.** `muted` on `dominant` 5.82:1, wedge labels 15.26:1 or better, `ink` text 17.32:1.

## 12. Notes for the plan (build only after Phase 1 is closed)

This contract is built only after Phase 1 is closed and these documents are copied into the main tree by hand, with the REQUIREMENTS, ROADMAP and PROJECT edits listed in the context file (hand edit 7 covers the Phase 1 `01-UI-SPEC.md` "Coordination with Phase 2" line).

- **Cold launch.** Replacing the `(app)` Stack with Tabs and changing the landing in `app/index.tsx` (it now also reads `extrasPending`: `/(app)/profile` during first run, `/(app)/today` otherwise) are launch-file changes. After them, cold-launch the dev client (`xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`) and confirm the first screen in every case of plan 02-03 Task 2, including "just signed up" (the photo-and-bio page first, no bar, no flash of Today). Never relaunch while a new person is on that page: the flag is in memory and a relaunch skips the page. Redirect to an explicit child such as `/(app)/today`, never a bare `/(app)`.
- **Placeholder page.** Delete `app/(app)/home.tsx`: under Tabs every file left in `app/(app)/` becomes a tab by itself, and the bar must show exactly three items.
- **No new native module.** Tabs come from expo-router; drawing uses react-native-svg and reanimated; icons are Ionicons. Never run `expo lint`.
- **Screen insets.** `Screen` gets an `edges` choice so the bottom inset is not counted twice inside Tabs. The default stays for signed-out screens and for the first-run page (its bar is hidden, so nothing else adds the bottom inset); Today and the edit-mode Profile page pass the top, left and right edges.
- **Radius allow-list** (from the code on 2026-10-09). The 24dp token `radius.lg` is used only by `components/ui/PhotoPanel.tsx`, `components/profile/BirthdayPickerField.tsx` (the date sheet, Phase 1 plan 01-21), `components/profile/ProfileForm.tsx` (the log-out sheet) and, new in Phase 2, `components/ui/ConfirmSheet.tsx`. `radius.full` is used only by `components/ui/IconBadge.tsx`, `components/brand/DialAvatar.tsx` and, new in revision 5, `components/nav/SpinTabButton.tsx` (the round Spin button). Plan 02-03 Task 2 scans for exactly these files.
- **Wheel code.** The Welcome wheel picks its own winner among 8 fixed sectors, so Today's wheel is a new component whose target comes from the server. Reuse its angle idea and `polar`.
- **Pure logic in pure files** (drawn angles, all equal, tones, label placement, the pointer check, countdown split, truncation), checked by a throwaway script and by device steps for counts 2 to 12, the tone table and the spin.
- **API.** `lib/api/spin.ts` (not "roll", so the word scans stay meaningful), using GET, POST and PATCH only, so "Save wheel" is one PATCH of the whole list. Spin responses carry the wheel used. The server sends the open time, the close time and its own "now", plus two ready-made strings so the phone does no date or time maths: `spin.spun_local_time` ("8:02 PM") and `next_open_weekday` ("Friday").
- **Text roles and fonts (revision 5).** `title`, `countdown`, `wheelLabel`, `hero`, `heroSmall` and `note` join the `type` group and are pinned in `tokens.test-assert.ts` in the same task (read that file first: its shape pin changes). `lib/theme/fonts.ts` loads `Domine_400Regular` and its `systemFallback` gets the new roles; `components/ui/AppText.tsx` gets the role names. Colour: no wheel token is added (the six `wheel1` to `wheel6` tokens of revision 4 are not added, and revision 5's `wheelMid` was dropped in revision 7); the wheel uses `ink`, `secondary` and `dominant`.
- **The mark.** The Spin button reuses `components/brand/BrandMark.tsx` (props `color` and `wedgeColor`), not a new drawing. That file is not edited.
- **Notifications route.** `app/(app)/today/notifications.tsx` joins the Today Stack beside `edit-wheel`; the list of `app/(app)` stays `_layout.tsx`, `profile`, `spin.tsx` and `today`. The Tabs layout hides the bar by reading the focused route name inside the `today` tab screen (`getFocusedRouteNameFromRoute`) for `notifications` now and for `entry` in Phase 3, so Phase 3 edits no launch file.

## 13. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plan). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 10.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Today | Your next spin is coming. / Opens at 8:00 PM | From your picture, with "spin". |
| 2 | Today | Your spin is open. / Tap the centre to spin / Open until midnight. | New. Only the centre is tappable, so it says centre. |
| 3 | Today | Today's spin. / Last night's spin. / Spun at 8:02 PM (the time is written by the server) | New. The late line "Posting it now would count as late." is left out of Phase 2; Phase 3 adds it (03-UI-SPEC row 1). |
| 4 | Bonus spin | Use your bonus spin? / Use bonus spin / Spin again / Keep this word | You wrote "Use your bonus reroll?". I used "spin" because "roll" is not allowed on screen. Say if you want your original. |
| 5 | Week section | This week / {n} of {goal} this week / You have a bonus spin. | From your picture. "4 day streak", "Goal met this week." and "Goal: {goal} posted days a week." are gone. |
| 6 | Messages | Your spin isn't open yet. It opens at 8:00 PM. / Today's spin has closed. The next one opens at 8:00 PM. | New. |
| 7 | Edit wheel | Edit wheel / Save wheel / Add a category / Remove / Changes show on your next spin. | New. The older lines about numbers and chances are gone: every category has the same chance (D-23). |
| 8 | Edit wheel | You can have up to 12 categories. / A wheel needs at least 2 categories. / Give this category a name. / You already have a category with this name. | New. |
| 9 | Edit wheel | Leave without saving? / Your changes will be lost. / Leave / Keep editing | New. |
| 10 | Profile | Saved. | New. |
| 11 | Starting wheel | Meal, Movie, Song, Book, Place, Game, in that order | Your picture draws six equal wedges with Movie on top. Ours are equal too, and every category has the same chance. |
| 12 | Notifications | Notifications / Nothing new yet. | New. Behind the bell. |

**Choices made here that you may want to change:**
- The Today screen follows your pictures of 2026-10-10: layout 1a (the centered wheel) and the screen after the spin.
- After the spin the wheel is gone and the word fills the screen, as in your "after the spin" picture. So "Edit wheel" is there only while the wheel shows (before the spin), and a bonus spin brings the wheel back for its turn.
- The word is one line in a large lighter serif (100). A long name shrinks to fit (down to about 36) and is cut with an ellipsis after that.
- The label "Your category" and the "Next spin opens ..." lines are gone from Today after the spin, because your picture has neither. "Spun at 8:02 PM" is written by the server.
- Your pictures show five tabs and the word "Roll" and a light grey middle circle; the app keeps three tabs until Diary and Board arrive, the word "Spin", and the outlined or solid circle you described.
- The Spin button is a circle: outlined while no spin can start, solid black when the spin opens. This replaces the dark rounded square you confirmed on 2026-10-08.
- The bell opens a Notifications page that only says "Nothing new yet." until Phase 5. There is no unread dot. The page has a back arrow and no tab bar, as in your Notifications picture.
- The round photo in the header is gone, because your picture has none. Profile is on the bar.
- The heading and the countdown use a lighter serif (Domine Regular) at 32 and 40, like your picture. That adds three text roles, so the type scale is no longer closed.
- Words on the wheel are 11dp: across the wedge where they fit and along it where they do not, and never upside down when the wheel stops (your picture's MEAL is upside down).
- Every category has the same chance, and Edit wheel shows no numbers: no plus and minus, no percentage, no line about higher numbers. Save sends 1 for every category (D-23).
- The wheel is black and light only, with no grey wedge. A thin seam in the page colour sits between all wedges, so with an odd count the last wedge and the first (both black) stay two separate wedges of the same size.
- A day still to come and a past day with nothing look the same: a small grey dot.
- Nothing sits under the heading in Waiting, Open, Result and Late. Posted and Skipped (Phase 3) have one line under it, as in your skipped picture. In Open, "Open until midnight." sits under the small label.
- "Something random, something good." is gone from Today, because your new picture does not have it.
- The week section is display only (no arrow, flame or "day streak").
- The wordmark is plain text (serif, Heading size), since the slanted-R lettering is your artwork.
- The bottom bar stays on Edit wheel, and leaving it with changes asks first.
- Edit wheel saves with one Save button.
- After signing up, the photo-and-bio page still comes first, with no bottom bar, and Continue or Skip for now then land on Today (added 2026-10-09).
- On the Profile tab, the bottom button "Continue" (when nothing is changed) goes to Today, and "Saved." shows under it after a save. Phase 1's temporary "You're in." page is gone (added 2026-10-09).
