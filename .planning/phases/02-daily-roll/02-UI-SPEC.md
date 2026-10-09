---
phase: 2
slug: daily-roll
status: draft
created: 2026-10-07
revision: 3
approval_word: go
---

# Phase 2 - UI Design Contract (Daily Roll, "the spin")

> Design contract the Phase 2 app plan builds from. Nothing here is built. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 15 or later). The developer's picture (`today-screen-mockup-2026-10-07.png`) is the layout source, minus every doodle, photograph, the bell and the friends card. Revision 2: pointer sign, 22 degree floor, bar on Edit wheel, Spin button size, strings. Revision 3 (2026-10-09, rebased on the final Phase 1 code): the one-time photo-and-bio page after sign-up comes before Today with no bottom bar (sections 1, 8, 9), the Profile tab's Continue button goes to Today and Phase 1's placeholder page is removed (section 9), the "Saved." line is placed against the real Profile page (section 9), and the radius allow-list is recorded (section 12). No decision changed.

## 1. What this phase puts on screen

| Screen | Where | New or changed |
|---|---|---|
| Today (Home tab) | `app/(app)/today` | New. It is the signed-in landing screen. |
| Edit wheel | pushed from Today | New. |
| Profile tab | the "Make it yours." page in edit mode | Moved into a tab, four small changes (section 9). |
| First-run page | the "Make it yours." photo-and-bio page (Phase 1's page 2, extras mode), on the Profile route | Phase 1's page, unchanged. Phase 2 only hides the bottom bar on it and ends it on Today (sections 8 and 9). |
| Bottom bar | Home, Spin, Profile | New. Diary and Board do not exist yet and get no placeholder tabs. |
| Phase 1 placeholder page | `app/(app)/home.tsx`: "You're in." / "Your daily spin will appear here." | Removed. Today replaces it. |

**Landing (no decision named it):** a signed-in, finished account lands on Today, with one exception: right after a sign-up (or after an Apple or Google account is finished) it lands once on the photo-and-bio page (section 9), and Continue or Skip for now then end on Today. Signed-out flow unchanged.

## 2. Tokens added (the only new design values)

Added inside the `color` group of `lib/theme/tokens.ts` (so the shape pin in `tokens.test-assert.ts` holds) and pinned there with `satisfies`.

| Token | Value | Plain name | Source |
|---|---|---|---|
| `wheel1` | `#111111` | charcoal | same value as `ink` |
| `wheel2` | `#4D4A46` | dark grey | new |
| `wheel3` | `#8A8680` | medium grey | new |
| `wheel4` | `#D3D0C9` | warm grey | same value as `divider` |
| `wheel5` | `#F6F5F2` | off-white | same value as `dominant` |
| `wheel6` | `#E8E7E3` | slightly darker off-white | same value as `secondary` |

No other new colour. The rim and hub ring use `divider`, the seams `dominant`, the pointer, hub fill and day dots `ink`. Screens never write a hex value.

**Label colour (fixed lookup, kept next to the tones):** tones 1 and 2 take `dominant` text; tones 3 to 6 take `ink`.

| Tone | Label | Contrast | AA (4.5:1) |
|---|---|---|---|
| 1 charcoal | dominant | 17.32 | pass |
| 2 dark grey | dominant | 8.08 | pass |
| 3 medium grey | ink | 5.22 | pass (dominant would be 3.32, so it flips) |
| 4 warm grey | ink | 12.26 | pass |
| 5 off-white | ink | 17.32 | pass |
| 6 off-white, darker | ink | 15.26 | pass |

Tone 4 equals the rim colour and tone 5 the seam colour, so those wedges meet them without an edge; a device check confirms the edges still read.

**Neighbour rule.** Two wedges that touch must differ by at least 2:1. Pairs below 2:1 are only 4 with 5 (1.41), 4 with 6 (1.24) and 5 with 6 (1.13), so the three light tones never touch. Every other pair is 2.14 or better. The last wedge touches the first, and the rule applies there too.

**Assignment (fixed, so a plan can pin it).** Cycle order S = 1, 5, 3, 6, 2, 4. Wedge i (from 0, clockwise) takes the first tone in S, starting at position `i mod 6`, that is not the previous wedge's tone and is at least 2:1 from it. The last wedge must also be at least 2:1 from wedge 0. Result:

| Wedges | Tones, clockwise from the first wedge | Worst touching pair |
|---|---|---|
| 2 | 1 5 | 17.32 |
| 3 | 1 5 3 | 3.32 |
| 4 | 1 5 3 6 | 2.92 |
| 5 | 1 5 3 6 2 | 2.14 (last with first) |
| 6 | 1 5 3 6 2 4 | 2.92 |
| 7 | 1 5 3 6 2 4 3 | 2.35 |
| 8 | 1 5 3 6 2 4 1 5 | 2.92 |
| 9 | 1 5 3 6 2 4 1 5 3 | 2.92 |
| 10 | 1 5 3 6 2 4 1 5 3 6 | 2.92 |
| 11 | 1 5 3 6 2 4 1 5 3 6 2 | 2.14 (last with first) |
| 12 | 1 5 3 6 2 4 1 5 3 6 2 4 | 2.92 |

## 3. The wheel

**Words only.** Every wedge carries its category name and nothing else. Names show in capitals by text style only.

**Size.** Diameter is the smaller of 296dp and the screen width minus 48dp. Rim is a 4dp ring in `divider`. The hub is 19 percent of the diameter (56dp at 296), `ink` fill with a 2dp `divider` ring, and a four-point star drawn with react-native-svg in `dominant` at 18dp. The hub and the pointer never rotate; the wedges, seams and labels do.

**Pointer.** An `ink` triangle, apex down, 16dp wide and 12dp tall, centred at 12 o'clock. Its tip touches the rim's outer edge. It sits outside the disc, so it is always `ink` on `dominant` (17.32:1).

**Wedge width follows weight** (bigger wedge, higher chance).
- True angle of a wedge: `360 x weight / total`.
- No wedge is drawn narrower than 22 degrees (a 15dp chord at the label start, 40dp out, enough for a 14dp line). A narrower wedge is drawn at 22 and the others share the rest in proportion. Repeat until nothing is under 22. Twelve wedges need 264 degrees, so it fits. A wedge over 180 degrees (weights 10 and 1) is drawn with the large-arc flag.
- Edit wheel's chance figures use the true shares, not the drawn angles.
- Wedges run clockwise in list order; the first starts at angle 0 (12 o'clock).
- Starting wheel, in this order: Meal 3, Movie 1, Song 3, Book 1, Place 2, Game 1: 98, 33, 98, 33, 65 and 33 degrees.

**Labels.** One rule for 2 to 12 wedges: the label runs along the wedge's centre line, from 12dp outside the hub to 10dp inside the rim, in the Label size (14/20) at 14dp, never smaller. A label is turned 180 degrees when its wedge, in the pose the wheel stops in, is on the left half, so every word reads upright at rest. The turn is set before the spin starts and at every rest placement. Labels turn with the wheel. A name too long for the room is cut with an ellipsis (about 9 letters at the largest wheel). The full name is in Edit wheel, the result and the spoken label.

**Rest position.** With no result, the first wedge's centre is under the pointer, as in the picture. With a result, the winning wedge's centre is.

**Spin geometry.** Angles run clockwise from 12 o'clock and the disc turns clockwise, so the pose that brings a wedge centre `c` to the pointer is `-c`; the target is `-c` of the winning wedge. Final rotation = `ceil((current + 1800 - pose) / 360) x 360 + pose`, so the wheel turns at least five full times, clockwise, and stops with the centre of the server's wedge under the pointer. The word under the pointer is always the word shown big. The server picks and stores the category first; the app only animates to it.

**Which wheel is drawn.** Once today's spin exists, Today shows the wheel the server used for it, so pointer and result agree even after an edit. A bonus spin uses that same wheel. Spin responses carry it.

## 4. Today screen

**Layout, top to bottom.** Side margins `lg` (24). The page scrolls inside `Screen`; only the bottom bar is fixed.

1. **Header row**, 44dp high. Left: the wordmark "RNDMRoll" as text, Label size, `WorkSans_600SemiBold`, letter spacing 2.4, `ink`. Right: the profile photo, 40dp (hit area 44), in `DialAvatar` (no dial ring, `size` 40), with the camera placeholder when there is no photo. It opens the Profile tab. No bell.
2. **Date line**, `md` below the header. Left: the local date, for example "WED, OCT 8", Label size, `muted`, letter spacing 1.2, capitals by style. The date is the server's local date. Right: the "Edit wheel" link (section 7).
3. **Heading**, `sm` below. Heading role (22/28), `ink`, left, sentence case.
4. **Sub-line**, `sm` below. Label role, `muted`.
5. **The wheel**, `lg` below, centred, with the pointer on top.
6. **State block**, `lg` below the wheel, centred (countdown, open note, result, or message).
7. **Weekly card**, `xl` below, full width.
8. `xl` of space after the card, clear of the bar.

The picture's heading is about 27dp; Heading (22) keeps the result word (Display, 40) the largest thing on screen.

### 4.1 States

The phone shows what the server says and never works out the date, the window or the winner.

| State | When | Heading | Sub-line | Under the wheel | Wheel |
|---|---|---|---|---|---|
| Loading | first fetch | none | none | "Loading your wheel." | rim only, no wedges |
| Waiting | before 8:00 PM local, or the day was missed | Your next spin is coming. | Something random, something good. | countdown (4.2) | still |
| Open | 8:00 PM to local midnight, not spun | Your spin is open. | Tap the centre to spin. | "Open until midnight." | still, hub tappable |
| Spinning | from the answer arriving until 4.0 s later | Your spin is open. | Tap the centre to spin. | "Spinning." | turning |
| Result | spun today | Today's spin. | Next spin opens tomorrow at 8:00 PM. | result block (4.3) | still, winner under pointer |
| Late | after midnight, before the next 8:00 PM, spin not posted | Last night's spin. | Next spin opens at 8:00 PM. | result block | still |
| Offline on load | first fetch fails | none | none | message and "Try again" (4.4) | not drawn |
| Offline on spin | the spin request fails | Your spin is open. | Tap the centre to spin. | message and "Try again" | still |

Notes:
- There is no button on Result or Late until Phase 3 can post.
- When the day closes while the screen is open, the screen asks the server again and shows Late if today's spin is not posted, else Waiting. A spin tapped after close gets the "closed" message in 4.4.
- The hub is tappable only in Open (disabled, no press effect in Waiting).
- Today asks the server again on foreground, on tab focus, when a countdown reaches zero, and at the server's `next_open_at` and close time (also in Late, which has no countdown). Plan 02-02 decision 5 has the retry rules.

### 4.2 Countdown (Waiting)

Three lines, centred: the Label "Opens in" (capitals by style, letter spacing 1.5, `muted`); the digits "02 : 14 : 36" in Heading role (22/28), `ink`; the Label "at 8:00 PM" (`muted`). Hours, minutes and seconds each sit in their own fixed-width cell (32dp, centred) so the row never changes width as digits change. The time is always written 8:00 PM (a product constant). The countdown runs to the server's open time, measured from the server's "now", so a wrong phone clock changes nothing. VoiceOver reads "Opens in 2 hours 14 minutes", updated once a minute, never per second.

### 4.3 Result block

Centred under the wheel. First the Label "Your category" (capitals by style, letter spacing 1.5, `muted`). Then the category name, `ink`, as the person wrote it (not forced to capitals): in the Display role (40/46) when it has 12 characters or fewer, else in the Heading role (22/28), up to two lines. No other size, so the type scale stays closed. Under it, only if a bonus spin is held and the day is not settled, the text link "Use bonus spin" (Ink, underlined, `WorkSans_600SemiBold`, 44dp high). No other button, no share, no "log it" until Phase 3.

**Bonus spin sheet.** The link opens a bottom sheet. It reuses the log-out sheet's modal, scrim, top corners and padding. Title (Heading role): "Use your bonus spin?". Body (Body role): "You will spin again and get a different word. This uses up your bonus spin." Two full-width rows, each at least 56dp high, separated by hairlines: "Spin again" and "Keep this word". "Spin again" closes the sheet and runs the Spinning state. The server never returns the same category. After it, the link is gone.

### 4.4 Messages

One plain Label line in `destructive`, centred, with a standard `ink` "Try again" button (the existing primary button, 8dp radius). In Offline on load the wheel area holds the message instead of a wheel.

| Case | Message |
|---|---|
| No connection (load or spin) | Couldn't connect. Check your connection and try again. |
| Spin before the window opens (clock mismatch) | Your spin isn't open yet. It opens at 8:00 PM. |
| Spin after local midnight | Today's spin has closed. The next one opens at 8:00 PM. |
| Anything else | Something went wrong. Please try again. |

A retry after a lost connection returns the same spin, so a spin cannot be lost or doubled. After the "isn't open yet" and "closed" messages, "Try again" asks the server for Today again instead of sending a new spin. If the day already has a spin, the screen shows it (Result) with no error text.

## 5. Spin motion

Wait, spin, anticipation, reveal: calm before, life only during the spin, quiet after.

1. **Tap.** The hub (or the middle Spin button) presses in to 0.97 scale over 90ms. The request goes out. The wheel does not move until the answer arrives, normally well under a second.
2. **Turn.** From the answer, 4000ms, `withTiming` with `Easing.bezier(0.3, 0, 0.1, 1)`: smooth start, fastest in the first second, then a steady slow-down that creeps into place. No bounce, overshoot, flash, confetti, sound or vibration. The numbers are a starting point for the device check.
3. **Settle.** The wheel stops on the winning wedge. Nothing else moves for 600ms.
4. **Reveal.** The heading and sub-line swap with the existing 400ms fade. The result word rises 14dp into place while fading in (existing `FadeInUp`: 400ms, ease-out cubic). The bonus spin link follows 120ms later.
5. **Done.** Nothing else animates. No screen change.

**Reduce Motion on:** no turning, rise, fade or press scale. The wheel is placed at once with the winner under the pointer, and the result block, heading and sub-line appear at once. The countdown text still updates.

## 6. Weekly card

**Not tappable.** Phase 2 has nowhere for it to go (the diary is Phase 4), and a control with no destination breaks the rule of no dead controls. So the picture's arrow is removed, and the card has no press state and no role of "button". It uses `md` (8dp) radius, a 1dp `divider` border, a `fieldSoft` fill and no shadow, and stays 8dp when Phase 4 gives it a destination.

**Content, left aligned, `md` padding:**
- Title, Heading role (22/28): "{n} of {goal} this week", for example "2 of 4 this week". The numbers come from the server; with no posted days it reads "0 of 4 this week".
- Sub-line, Label role, `muted`, one of: "You have a bonus spin." (a bonus spin is held), else "Goal met this week." (goal met, none held), else "Goal: {goal} posted days a week."
- Seven day columns, Monday to Sunday: a Label letter above a 12dp dot. The letters come from the server (M T W T F S S now) in `muted`. Dot states: posted is a filled `ink` dot; today, not posted, is a ring in `ink` 1.5dp with an `ink` letter; any other day not posted is a 1dp ring in `muted`. A skipped day gets its own mark in Phase 3. Dots are drawn shapes (react-native-svg), not controls, evenly spaced across the card's inner width.

**Removed from the picture:** the flame, "day streak", "Keep it going." and the arrow. Nothing can be posted in Phase 2, so the weekly count is the only truthful figure; posted dots come only from the test setting's pretend days.

## 7. Edit wheel screen

Opened by the small "Edit wheel" link on Today's date row (Label size, `ink`, underlined, `WorkSans_600SemiBold`, 44dp hit area). It is disabled while the wheel is spinning. The screen is pushed over Today with a Back control (chevron and "Back", the same as on the Log in page; the Profile tab no longer has one). The bottom bar stays, and every way out with unsaved changes (Back, swipe, a Home or Spin tab press) opens the leave sheet.

**Layout, top to bottom, scrolling.**
1. Heading "Edit wheel" (Heading role).
2. A line: "A higher number comes up more often." (Label, `muted`). If today's spin exists, add under it: "Changes show on your next spin."
3. **Live preview**, 240dp, centred: the same wheel as Today (section 3), no pointer, not tappable, redrawing as you change anything. Labels are cut to fit.
4. **Category rows**, one per category, each at least 56dp high, separated by hairlines (`divider`). A row holds:
   - the name in a text field (existing input style: `md` radius, `inkRest` border, 48dp high, up to 24 characters; editing it is the rename),
   - the weight stepper: a minus button, the number (Body role, centred, 32dp wide) and a plus button. Each button is a 44dp square with 8dp radius, `inkRest` border, Ionicons `remove` and `add` in `ink`. Minus is disabled at 1 and plus at 10,
   - the Label line "About {pct}% of spins" (`muted`), whole number, from the true shares,
   - a "Remove" text link in `destructive` (44dp hit area).
5. **Add row**: a text button "Add a category" with an Ionicons `add` glyph (44dp high). It adds a last row with an empty focused name and weight 1. At 12 categories it is replaced by the Label "You can have up to 12 categories."
6. A Label, `muted`: "Removed categories stay in your past spins."
7. **Save**, the existing primary button "Save wheel", pinned above the bar and the keyboard, grey (existing disabled treatment) until something has changed and every row is valid.

**Limits (the screen enforces them; the server checks again).**
- 2 to 12 categories. At 2, every "Remove" is disabled and the Label "A wheel needs at least 2 categories." shows under the rows.
- Name: 1 to 24 characters after trimming spaces. No two names the same, ignoring capitals. The field stops at 24 characters.
- Weight: whole numbers 1 to 10, changed only with the buttons (no typing).
- Row errors (Label, `destructive`, under the field, shown on leaving the field or on Save): "Give this category a name." and "You already have a category with this name."

**Behaviour.**
- Changes stay on the phone until Save, which sends the whole list in one request (all or nothing). Success returns to Today with the wheel redrawn.
- "Remove" on a saved category archives it (it keeps its name in old spins). On a row added in this visit it simply deletes the row.
- Leaving with unsaved changes opens a sheet (same reused parts as 4.3). Title: "Leave without saving?" Body: "Your changes will be lost." Rows: "Leave" and "Keep editing".
- If Save cannot connect: "Couldn't connect. Check your connection and try again." in `destructive` above the Save button. Other server refusals use "Something went wrong. Please try again."
- The preview redraws at once on every change, with no animation, whatever the Reduce Motion setting.

## 8. Bottom bar

Three items: Home, Spin, Profile. `dominant` fill, 1dp `divider` top border, 56dp plus the bottom safe area. Nothing for Diary or Board.

**Hidden on the first-run page.** While the photo-and-bio page (section 9) is on screen the bar is not drawn at all. That page is still part of signing up, it has no Back and no Log out, and a bar would let the person step sideways past it. The bar appears with Today. (Decided as suggested on 2026-10-09; the developer can flip it in the walkthrough.)

| Item (Ionicons, Label size) | Idle | Active |
|---|---|---|
| Home | `home-outline`, `muted` | `home`, `ink`, label `WorkSans_600SemiBold` |
| Profile | `person-outline`, `muted` | `person`, `ink`, label `WorkSans_600SemiBold` |

Icons are 24dp inside a 44dp hit area; labels are Label size (14). No bouncing icons, only colour and fill change.

**Spin button, middle.** A square with 8dp radius (not a circle: it is tappable, so the 8dp cap holds), 48dp by 48dp, `ink` fill, `raised` elevation, rising 16dp above the bar's top edge, so its bottom edge is 32dp below that edge and clear of the label line. Inside, a wheel mark drawn with react-native-svg in `dominant`: a ring and six spokes, 28dp, after the picture's middle button. The label "Spin" sits under it on the same line as the other labels. The top 16dp above the bar must still be tappable (device check). It is an action and never shows as the selected tab.
- Open and unspun: goes to Home and starts the spin once Today has loaded and is still open.
- Waiting, Result or Late: goes to Home and shows the wheel. Nothing spins. It does not look disabled.
- Spoken hint: Open "Spins today's wheel." Waiting "Shows the wheel. It opens at 8:00 PM." Otherwise "Shows today's spin."

**Round shapes.** The header photo is a circle, a named exception (confirmed by the developer on 2026-10-08): the avatar token allows it and the Make it yours page already has a tappable circular photo. The visible hub is a circle in the wheel drawing with no button styling; the tappable part is a transparent 64dp square (8dp radius) over it. Day dots and the star are drawn shapes. No other tappable element is round, and nothing is a pill.

## 9. Profile tab and the first-run page

The Profile tab shows Phase 1's "Make it yours." page in edit mode (Name, Username, Bio, the photo, one bottom button, Log out), strings unchanged, with four changes because it is now a tab, not the end of a stack:
1. No Back control, whatever the navigation history says (inside tabs, "back" would jump to another tab). The empty left side keeps the "RNDMRoll" label and rule where they are.
2. After "Save changes" the person stays on Profile (Phase 1 went back a page when there was one). Nothing is left to save, so the bottom button swaps back to "Continue", and the Label "Saved." shows under it in `muted`, centred, for 3 seconds. It goes away at once if anything is edited again.
3. No bottom safe-area inset on the page (the bar adds it).
4. The bottom button keeps both Phase 1 states: "Save changes" once a field or the photo has changed, "Continue" while nothing has. "Continue" now opens Today. Phase 1 opened a temporary page ("You're in." / "Your daily spin will appear here."), which Phase 2 removes with its route; the developer asked on 2026-10-08 for a Continue that goes "to the next phase", and this is it.

Log out, its sheet, the username check and the photo picker are unchanged.

**First run (the same page in extras mode).** Right after a sign-up, and after an Apple or Google account is finished, the Profile route shows Phase 1's page 2 once: "Make it yours." with its sub-line, the camera circle, the Bio box, Continue and "Skip for now". Phase 2 changes only what is around it:
- The bottom bar is hidden on it (section 8). The page keeps the default safe-area insets, because no bar adds the bottom one.
- Continue (after its save) and Skip for now both take the person to Today, and the bar appears. If the photo upload fails the person stays on the page with Phase 1's message under the circle, and can retry or skip.
- It is shown once. The flag behind it lives in memory only, so a relaunch goes straight to Today and the page never comes back. A returning person who logs in, and an expired session that refreshes, also go straight to Today.
- All of its strings are Phase 1's. Phase 2 adds none.

## 10. Copy contract

Every string the person sees. "Spin" wording, plain words, no em dashes, no legal text. New strings are marked N.

| Where | String |
|---|---|
| Wordmark (brand name) | RNDMRoll |
| Date line | WED, OCT 8 (example) |
| Link on Today | Edit wheel (N) |
| Heading, waiting | Your next spin is coming. |
| Sub-line, waiting and open | Something random, something good. / Tap the centre to spin. |
| Heading, open and spinning | Your spin is open. |
| Heading, result | Today's spin. |
| Heading, late | Last night's spin. |
| Sub-line, result | Next spin opens tomorrow at 8:00 PM. |
| Sub-line, late | Next spin opens at 8:00 PM. |
| Countdown | Opens in / 02 : 14 : 36 (example) / at 8:00 PM |
| Open note | Open until midnight. |
| Spinning note | Spinning. |
| Loading | Loading your wheel. |
| Result kicker | Your category |
| Bonus spin link | Use bonus spin |
| Bonus spin sheet | Use your bonus spin? / You will spin again and get a different word. This uses up your bonus spin. / Spin again / Keep this word |
| Weekly card | {n} of {goal} this week / You have a bonus spin. / Goal met this week. / Goal: {goal} posted days a week. / M T W T F S S (letters from the server) |
| Errors | Couldn't connect. Check your connection and try again. / Your spin isn't open yet. It opens at 8:00 PM. / Today's spin has closed. The next one opens at 8:00 PM. / Something went wrong. Please try again. |
| Retry button | Try again |
| Edit wheel | Edit wheel / A higher number comes up more often. / Changes show on your next spin. / About {pct}% of spins / Remove / Add a category / You can have up to 12 categories. / A wheel needs at least 2 categories. / Removed categories stay in your past spins. / Give this category a name. / You already have a category with this name. / Save wheel |
| Leave sheet | Leave without saving? / Your changes will be lost. / Leave / Keep editing |
| Bottom bar | Home / Spin / Profile |
| Profile tab | Saved. (the only new string; Continue, Save changes and everything on the first-run page are Phase 1's) |
| Removed with the placeholder page | You're in. / Your daily spin will appear here. (Phase 1 strings, still on Phase 1's keep-or-change list; Phase 2 deletes the page they sit on) |
| Starting category names | Meal, Movie, Song, Book, Place, Game |

The existing offline string and generic fallback are reused. New server error codes join the API error list; only "spin not open" and "spin closed" get their own strings above, the rest use the generic one.

## 11. Accessibility

- **Wheel.** One non-interactive element, label "Wheel with {n} categories: {names in order}". Hidden from VoiceOver during a spin; "Spinning." is read instead.
- **Hub.** A button, label "Spin the wheel", hint "Opens at 8:00 PM." while waiting (state disabled). Hit area 64dp. Wheel labels, tab labels and countdown digits do not scale with the phone's text size.
- **Result.** When the spin lands, announce "Your spin is {name}." with `announceForAccessibility`. The result word is a header element with the label "Today's spin: {name}." (late: "Last night's spin: {name}."). Under Reduce Motion the announcement is made when the result appears.
- **Weekly card.** One element: "{n} of {goal} this week. {sub-line}." then each day by the server's weekday name, "Monday posted, Tuesday not posted", and so on. Filled against ring, never colour alone.
- **Edit wheel.** Name field "Category name". Steppers "Lower weight for {name}" and "Raise weight for {name}"; value read as "{name}, weight {n}". "Remove {name}". The preview is hidden from VoiceOver.
- **Profile tab.** The "Saved." line is a polite live region, so VoiceOver reads it once when it appears.
- **Targets.** Every tappable is at least 44dp (hub 64, Spin button 48, rows 56).
- **Contrast.** `muted` on `dominant` 5.82:1, wedge labels 5.22:1 or better, `ink` text 17.32:1.

## 12. Notes for the plan (build only after Phase 1 is closed)

This contract is built only after Phase 1 is closed and these documents are copied into the main tree by hand, with the REQUIREMENTS, ROADMAP and PROJECT edits listed in the context file (hand edit 7 covers the Phase 1 `01-UI-SPEC.md` "Coordination with Phase 2" line).

- **Cold launch.** Replacing the `(app)` Stack with Tabs and changing the landing in `app/index.tsx` (it now also reads `extrasPending`: `/(app)/profile` during first run, `/(app)/today` otherwise) are launch-file changes. After them, cold-launch the dev client (`xcrun simctl terminate booted com.rndmroll.app; xcrun simctl launch booted com.rndmroll.app`) and confirm the first screen in every case of plan 02-03 Task 2, including "just signed up" (the photo-and-bio page first, no bar, no flash of Today). Never relaunch while a new person is on that page: the flag is in memory and a relaunch skips the page. Redirect to an explicit child such as `/(app)/today`, never a bare `/(app)`.
- **Placeholder page.** Delete `app/(app)/home.tsx`: under Tabs every file left in `app/(app)/` becomes a tab by itself, and the bar must show exactly three items.
- **No new native module.** Tabs come from expo-router; drawing uses react-native-svg and reanimated; icons are Ionicons. Never run `expo lint`.
- **Screen insets.** `Screen` gets an `edges` choice so the bottom inset is not counted twice inside Tabs. The default stays for signed-out screens and for the first-run page (its bar is hidden, so nothing else adds the bottom inset); Today and the edit-mode Profile page pass the top, left and right edges.
- **Radius allow-list** (from the code on 2026-10-09). The 24dp token `radius.lg` is used only by `components/ui/PhotoPanel.tsx`, `components/profile/BirthdayPickerField.tsx` (the date sheet, Phase 1 plan 01-21), `components/profile/ProfileForm.tsx` (the log-out sheet) and, new in Phase 2, `components/ui/ConfirmSheet.tsx`. `radius.full` is used only by `components/ui/IconBadge.tsx` and `components/brand/DialAvatar.tsx`. Plan 02-03 Task 2 scans for exactly these files.
- **Wheel code.** The Welcome wheel picks its own winner among 8 fixed sectors, so Today's wheel is a new component whose target comes from the server. Reuse its angle idea and `polar`.
- **Pure logic in pure files** (drawn angles with the 22 degree floor, tones, the pointer check, countdown split, truncation), checked by a throwaway script and by device steps for counts 2 to 12, the tone table and the spin.
- **API.** `lib/api/spin.ts` (not "roll", so the word scans stay meaningful), using GET, POST and PATCH only, so "Save wheel" is one PATCH of the whole list. Spin responses carry the wheel used. The server sends the open time, the close time and its own "now".

## 13. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plan). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 10.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Today | Your next spin is coming. / Something random, something good. / Opens in / at 8:00 PM | From your picture, with "spin". |
| 2 | Today | Your spin is open. / Tap the centre to spin. / Open until midnight. | New. Only the centre is tappable, so it says centre. |
| 3 | Today | Today's spin. / Last night's spin. / Your category | New. The late line "Posting it now would count as late." is left out until Phase 3, when posting exists. |
| 4 | Bonus spin | Use your bonus spin? / Use bonus spin / Spin again / Keep this word | You wrote "Use your bonus reroll?". I used "spin" because "roll" is not allowed on screen. Say if you want your original. |
| 5 | Weekly card | {n} of {goal} this week / Goal: {goal} posted days a week. / Goal met this week. / You have a bonus spin. | Replaces "4 day streak". No streak word, no instruction Phase 2 cannot follow. |
| 6 | Messages | Your spin isn't open yet. It opens at 8:00 PM. / Today's spin has closed. The next one opens at 8:00 PM. | New. |
| 7 | Edit wheel | Edit wheel / Save wheel / Add a category / Remove / A higher number comes up more often. / Changes show on your next spin. | New. |
| 8 | Edit wheel | You can have up to 12 categories. / A wheel needs at least 2 categories. / Give this category a name. / You already have a category with this name. | New. |
| 9 | Edit wheel | Leave without saving? / Your changes will be lost. / Leave / Keep editing | New. |
| 10 | Profile | Saved. | New. |
| 11 | Starting wheel | Meal, Movie, Song, Book, Place, Game, in that order | Your picture used another order with equal wedges. |

**Choices made here that you may want to change:**
- The weekly card is display only (no arrow, flame or "day streak").
- The Spin button is a dark rounded square with a light icon (yours is light with a dark icon). Confirmed by the developer on 2026-10-08: keep it dark, like the other buttons.
- The header photo stays a circle. This is the one tappable circle, as on Make it yours. Confirmed by the developer on 2026-10-08: keep it round.
- Wedges are unequal (bigger weight, bigger wedge) with words along the wedge.
- The wordmark is plain text, since the slanted-R lettering is your artwork.
- Countdown digits are 22, not about 32, and a long result name drops to 22, because the type scale is closed.
- The bottom bar stays on Edit wheel, and leaving it with changes asks first.
- Edit wheel saves with one Save button.
- After signing up, the photo-and-bio page still comes first, with no bottom bar, and Continue or Skip for now then land on Today (added 2026-10-09).
- On the Profile tab, the bottom button "Continue" (when nothing is changed) goes to Today, and "Saved." shows under it after a save. Phase 1's temporary "You're in." page is gone (added 2026-10-09).
