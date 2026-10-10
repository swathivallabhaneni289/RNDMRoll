---
phase: 3
slug: daily-entry
status: draft
created: 2026-10-10
revision: 1
approval_word: go
---

# Phase 3 - UI Design Contract (Daily Entry, "log it")

> Design contract the Phase 3 app plans (03-02, 03-03) build from. PHASE 3. NOTHING IS BUILT. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later), and the Today, Edit wheel and sheet parts of `02-daily-roll/02-UI-SPEC.md` (revision 3 or later; Phase 2 is not built either). No new font, colour or radius is added (section 2). The only non-token thing on screen is the set of six reaction emoji, which the developer chose as content (section 6). All imagery is the developer's: photos are the person's own and every placeholder is plain.

## 1. What this phase puts on screen

| Screen or state | Where | New or changed |
|---|---|---|
| Result and Late on Today | `components/today` | Changed: the 'Log it' button, the 'Skip this one' link and the late line. |
| Posted on Today | same | New state. |
| Skipped on Today | same | New state. |
| Skip sheet | over Today | New. It is Phase 2's `ConfirmSheet`. |
| Entry screen | `app/(app)/today/log` | New. 'Log it' opens it. |
| Entry page (view) | `app/(app)/today/entry` | New. The Phase 4 diary opens the same page. |
| Edit entry | `app/(app)/today/edit-entry` | New. |
| Delete sheet | over the entry page | New. `ConfirmSheet` again. |
| Week dots | `components/today/WeeklyCard.tsx` | Changed: a skipped day gets a dash. |
| Bottom bar | `app/(app)/_layout.tsx` | Unchanged: Home, Spin, Profile. |

All new routes sit inside the Home stack (`app/(app)/today/`), next to Today and Edit wheel. Under Tabs a file placed directly in `app/(app)/` becomes a tab, so none is.

## 2. Tokens added (the only new design values)

None. Every colour, space, radius, type size and elevation comes from `lib/theme/tokens.ts` as it stands (plus Phase 2's `wheel1` to `wheel6`, which this phase does not use).

Reused as they are:

| Need | Token |
|---|---|
| Text and filled stars | `ink` |
| Empty star outlines, helper lines, labels | `muted` (5.82:1 on the page, over the 3:1 floor for graphics) |
| Page, footer | `dominant` |
| Hairlines | `divider` |
| Unselected reaction fill, soft boxes | `fieldSoft` with an `inkRest` border |
| Selected reaction fill | `secondary` with a 1.5dp `ink` border |
| Errors | `destructive` |
| Radius on every tappable | `md` (8dp) |
| Primary button | `PrimaryButton` (`raised` elevation) |

Radius allow-lists are unchanged: Phase 3 adds no use of `radius.lg` or `radius.full`. The photo panel, the star buttons, the stepper buttons and the reaction buttons are all tappable, so all are 8dp or less. The delete and skip sheets are Phase 2's `ConfirmSheet` (already on the `radius.lg` list).

**The one sanctioned colour.** The six reaction characters are system emoji and draw in the system's colour (a red heart, a yellow face). That is the developer's choice (2026-10-09). It is allowed in exactly two places: the six buttons of the reaction picker, and where an author's chosen reaction is shown (the entry page). It is never used as an icon, a bullet, a button label or decoration anywhere else.

## 3. Today: Result, Late, Posted, Skipped

The phone shows what the server says. It never works out the date, the window, the state or whether a post is late.

### 3.1 States (rows that change or are new; every other row of Phase 2's table 4.1 stands)

| State | When | Heading | Sub-line | Under the wheel | Wheel |
|---|---|---|---|---|---|
| Result | spun today, not posted, not skipped | Today's spin. | Next spin opens tomorrow at 8:00 PM. | result block, then the day actions (3.2) | still, winner under the pointer |
| Late | after midnight, before the next 8:00 PM, last night's spin not posted and not skipped | Last night's spin. | Next spin opens at 8:00 PM. | result block, then the day actions with the late line (3.2) | still |
| Posted | the shown spin's day has a post | Posted. | Next spin opens tomorrow at 8:00 PM. | result block, then the posted block (3.3) | still |
| Skipped | the shown spin's day was skipped | Skipped. | Next spin opens tomorrow at 8:00 PM. | result block, then the skipped block (3.4) | still |

Notes:
- **Which sub-line.** The app compares two server date strings: when `spin.local_date` equals `local_date`, the 'tomorrow' sentence; when the shown spin is last night's, 'Next spin opens at 8:00 PM.'. No `Date` object is used.
- **How long Posted and Skipped show.** A post or skip made before midnight shows until midnight; after midnight Today is back to Phase 2's Waiting ('Your next spin is coming.', countdown). A post or skip made after midnight, in the late window, keeps Posted or Skipped up (with last night's spin) until the next spin opens at 8:00 PM, because the person has just acted and should see it land. The server decides this (`state`); the phone only draws it.
- The hub is not tappable in these states, as in Result. The 'Use bonus spin' link is gone (the server sends `can_use` false once the day is settled).
- After the person's own Log it or Skip it, the answer carries the new Today, so the screen changes at once with no loading step. The heading swap uses Phase 2's 400ms fade.
- The middle Spin button in these states goes to Home and shows the wheel, and nothing spins. Its spoken hint is Phase 2's 'Shows today's spin.'.

### 3.2 The day actions (Result and Late)

Below the result block, centred column inside the `lg` side margins, top to bottom:
1. **The late line**, Late state only: Label role, `muted`, centred: 'Posting it now would count as late.' `lg` below the result block.
2. **'Log it'**: `PrimaryButton`, standard variant, no arrow, full width of the column, label 'Log it'. `lg` below the result block in Result, `sm` below the late line in Late. It pushes the entry screen (section 4).
3. **'Skip this one'**: `TextButton`, tone `muted`, centred, `md` below the button, 44dp hit area. It opens the skip sheet (3.5).
4. The weekly card, `xl` below, as before.

While a skip request is running the button and the link are disabled. The 'Use bonus spin' link keeps its place under the word, above these (Phase 2 section 4.3).

### 3.3 The posted block

Under the result block, centred:
- If the entry was posted late (`entry.late` from the server): the Label 'Posted late.' in `muted`.
- If the day has an entry: the text link 'View entry' (Ink, underlined, `WorkSans_600SemiBold`, 44dp high), `md` below. It opens the entry page.
- If the day is posted but the entry was deleted (`entry` is null): the Label 'This entry was deleted. The day still counts.' in `muted`. No link.

(A pretend posted day made with the test tools also reaches the third case. Real use does not.)

### 3.4 The skipped block

Under the result block, centred, one Label in `muted`: 'It doesn't count toward your week, and it isn't a miss.' The category word stays so the person sees which spin was skipped. There is no undo: a skip is final.

### 3.5 The skip sheet

`ConfirmSheet` (Phase 2: same modal, scrim, top corners and padding as the Log out sheet). Title (Heading role): 'Skip this one?'. Body (Body role): 'You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards.' where {n} is `week.posted` and {goal} is `week.goal` from the server. When the shown spin is from the week before the card's week (`spin.local_date` is earlier than `week.start`, a plain string compare: a skip just after midnight on Monday for Sunday's spin), the first sentence is left out, because the card is already showing the new week. Two full-width rows, each at least 56dp, separated by hairlines: 'Skip it' and 'Don't skip'. 'Skip it' closes the sheet and sends the skip. No reason is asked, ever.

### 3.6 Messages

One Label line in `destructive`, centred, under the 'Skip this one' link (skip) or above the Save button (entry screen). Network failure is Phase 2's string and a repeat is allowed. The four state errors end the attempt: the screen shows the message and, on the entry screen, the Save button turns into 'Back to Today'.

| Case | Message |
|---|---|
| No connection | Couldn't connect. Check your connection and try again. |
| The day already has a post or a skip | This day already has a post or a skip. |
| The late window is over | This day has closed. It can't be posted or skipped any more. |
| No spin for that day | There is no spin for this day. |
| Another phone used the bonus spin | Your spin changed on another phone. Go back to Today to see it. |
| Anything else | Something went wrong. Please try again. |

After any of the four state errors Today asks the server again, so it shows the truth.

## 4. The entry screen

Opened by 'Log it'. Pushed over Today with a Back control (chevron and 'Back', as on Edit wheel). The bottom bar stays. Every way out with unsaved work (Back, the iOS swipe, a Home or Spin tab press) opens the leave sheet: 'Leave without saving?' / 'Your changes will be lost.' / 'Leave' / 'Keep editing' (Phase 2's strings), and the swipe is disabled while there is unsaved work. 'Unsaved work' is any of: a photo chosen, a title, stars, a reaction, a comment.

The screen reads Today once, when it opens, and keeps that spin for the whole visit, so the form never changes under the person. If Today has no spin, or its state is not Result or Late at that moment, the screen shows the Label 'There is nothing to log right now.' and a 'Back to Today' button, nothing else. If Today moves on while the form is open (another phone posted, skipped or used the bonus spin), the server refuses the save and the message of section 3.6 says so.

**Three parts: a fixed header, a scrolling middle, a pinned footer.** Build it as the Edit wheel screen is built (read `components/wheel/WheelEditor.tsx`): `Screen` with `scroll` off and `edges` top, left and right, a `KeyboardAvoidingView`, the footer above the keyboard.

**Fixed header** (does not scroll):
1. The Back row, 44dp.
2. `sm` below: the Label 'Your category' (capitals by style, letter spacing 1.5, `muted`).
3. `xs` below: the category name as the person wrote it, Heading role, one line, cut with an ellipsis if long. This is the required category for the entry (ENTRY-03). It comes from the shown spin (`spin.category.name`).
4. Late state only, `xs` below: the Label 'Posting it now would count as late.' in `muted`.
5. `md` below: a 1dp `divider` rule across the screen width.

**Scrolling middle**, `lg` of space above the first section and between sections:
1. **Photo** (section 7).
2. **Title**: `TextField` variant `soft`, label 'Title', single line, `maxLength` 80, `autoCapitalize` sentences, Return ends editing. The label row's hint shows the live count, '12/80'. No text inside the empty box.
3. **Stars** (section 5).
4. **Reaction** (section 6), label row hint 'Optional'.
5. **Comment**: `TextField` variant `soft`, label 'Comment', hint 'Optional', multiline, 2 lines tall, `maxLength` 140, count inside the box (as the Bio box). No text inside the empty box.
6. `xl` of space at the end, clear of the footer.

**Pinned footer** (above the keyboard and the bar, `dominant` fill, `md` padding):
- While Save is disabled for a missing part, the Label 'Still needed: photo, title and stars.' in `muted`, listing only what is missing, in the order photo, title, stars, joined with commas and 'and' ('Still needed: stars.', 'Still needed: title and stars.'). It is a polite live region.
- A failure message (3.6), if any, in `destructive`.
- The `PrimaryButton` 'Save'. Grey (the existing disabled treatment) until a photo, a title and stars are all set, and while saving; it shows its spinner while saving.

**Saving.** One tap runs: upload the photo if this picked file has not been uploaded yet (ticket, then the file), then the post. A repeat tap while saving does nothing. On success the screen goes back to Today, which already shows Posted. On a network failure or any non-terminal failure the form stays exactly as it was and Save is live again; a second try never uploads the same picked photo twice. If the photo upload itself fails, the message goes under the photo panel (section 7) and nothing is posted.

## 5. The stars input

A row of seven controls, left to right, each at least 44dp square and spread to the full width of the column: a minus button, five stars, a plus button. In a 375dp-wide phone (327dp column) the controls take 308dp and the gaps share the rest.

**Look.** Stars are Ionicons `star` (filled), `star-half` (half) and `star-outline` (empty), 30dp. Filled and half are `ink`; empty are `muted`. They differ by shape, never by colour alone. The minus and plus buttons are Edit wheel's steppers: 44dp square, 8dp radius, `inkRest` border, Ionicons `remove` and `add` 22dp in `ink`. A disabled one shows its glyph in `inkDisabled` and ignores taps.

**Label row.** The small-caps label 'Stars' on the left. On the right the figure (section below), Button role (16, semi-bold), `ink`. Empty shows no figure. Under the row, `xs` below: the Label 'Tap a star again for a half.' in `muted`.

**The value.** A whole number `v` from 0 to 10, where 0 means empty on screen only and is never sent. `v` is the stored value: 7 means 3.5 stars.

**The rule (pure, in `lib/entry/stars.ts`).**
- Tap star `k` (1 to 5):
  - if `v` is `2k` (a whole `k`): `k` below 5 gives `2k + 1` (3 becomes 3.5); star 5 gives 9 (5 becomes 4.5, because 5.5 does not exist);
  - if `v` is `2k + 1` (`k` and a half) and `k` is below 5: gives `2k` (3.5 back to 3);
  - otherwise: gives `2k` (a whole `k`).
- Minus gives `v - 1`, disabled at 0 and at 1. Plus gives `v + 1`, from 0 gives 1, disabled at 10. So 0.5 is reached with the minus button, and every value from 1 to 10 can be produced. No tap or step can leave the range 1 to 10.
- Figure: `v` even shows `v / 2` ('3'); `v` odd shows `(v - 1) / 2` and '.5' ('3.5', '0.5').
- Glyph at position `p` (1 to 5): `star` when `v` is at least `2p`; `star-half` when `v` is `2p - 1`; otherwise `star-outline`.

**Motion.** A tap uses `PressScale` (0.97 over 90ms, then a spring back with no overshoot), the docs' 'very small scale'. The glyph changes at once. No bounce, no colour, no sound. With Reduce Motion on the controls are plain pressables.

## 6. The reaction picker

Label row: 'Reaction' on the left, 'Optional' (hint style) on the right. Under it, six buttons in one row, spread across the column, each 44dp square with an 8dp radius. The character is drawn in a Text at the Heading size and line height (22/28), system emoji font, `allowFontScaling` off, so the type scale stays closed at 16, 14, 22 and 40.

| Name stored and sent | Character | Code point | Spoken |
|---|---|---|---|
| heart | ❤️ | U+2764 U+FE0F | Heart |
| laughing | 😂 | U+1F602 | Laughing |
| wow | 😮 | U+1F62E | Wow |
| fire | 🔥 | U+1F525 | Fire |
| clap | 👏 | U+1F44F | Clap |
| yum | 😋 | U+1F60B | Yum |

- **States.** Unselected: `fieldSoft` fill, 1dp `inkRest` border. Selected: `secondary` fill, 1.5dp `ink` border. One is selected at most. Tapping an unselected one selects it (and drops the old one). Tapping the selected one clears it.
- **Where the characters live.** In `lib/entry/reactions.ts` and nowhere else. The server and the database hold only the names. A scan proves no other new file holds an emoji (03-VALIDATION).
- **Phase 5** shows the same six to friends. Nothing in Phase 3 changes if the characters are swapped later: only that one file.
- **Motion.** `PressScale`, as the stars. Nothing else.

## 7. The photo field

Label row: the small-caps label 'Photo'.

**The panel.** Full width of the column, square, 8dp radius (it is tappable), `secondary` fill, 1dp `inkRest` border, photo `contentFit` cover (a centre crop; the stored file is the original). The whole panel is the tap target.
- **Empty:** the Phase 1 stand-in (`PhotoPlaceholder`, a centred `image-outline` glyph) with the Label 'Add a photo' in `muted` under the glyph. Plain. Nothing drawn, nothing generated.
- **Chosen:** the person's photo fills the panel (`ImageReveal`, 450ms, with Reduce Motion off). Under it, `sm` below, a `TextButton` 'Change photo' (tone `muted`), centred.

**Picking.** Tap the panel or 'Change photo': the action sheet from the profile page (an Alert) with the title 'Add a photo' (or 'Change photo' when one is chosen) and the rows 'Take photo', 'Choose from library', 'Cancel'. Both launchers use `quality: 0.8` and `preferredAssetRepresentationMode: Compatible`, as the profile page does. The camera sets the source 'camera'; the library sets 'library'. Nothing about the source is shown anywhere.

**Checks, before any upload.** The picked file's type must be JPG, PNG or WebP (a missing type counts as JPG, as on the profile page) and its size, read with `fileSize(uri)`, must be 50 MB or less (50 x 1024 x 1024 bytes). On a refusal the message appears under the panel and the previous choice, if any, stays.

| Case | Message under the panel (Label, `destructive`) |
|---|---|
| Wrong type | Use a JPG, PNG or WebP photo. |
| Over 50 MB | That photo is over 50 MB. Choose a smaller one. |
| No camera permission | Camera access is needed to take a photo. |
| No library permission | Photo library access is needed to choose a photo. |
| No camera on this device | Camera isn't available on this device. |
| Library would not open | Couldn't open your photo library. |
| The upload failed | Photo upload failed. Please try again. |
| Storage or the server refused the file | That file can't be used as a photo. Try another one. |

The last two are the existing app strings. A message clears as soon as a new photo is chosen.

## 8. The week dots (a change to Phase 2 section 6)

Four states now. The dot slot stays 12dp, drawn with react-native-svg, not a control.

| State | Mark | Letter above |
|---|---|---|
| Posted | filled `ink` dot | `muted` |
| Skipped | a short dash: `ink`, 10dp wide, 2dp tall, centred in the slot, square ends | `muted` (`ink` if it is today) |
| Today, not posted, not skipped | ring in `ink`, 1.5dp | `ink` |
| Any other day with nothing | ring in `muted`, 1dp | `muted` |

Order of precedence: posted, then skipped, then today's ring, then the plain ring. A day the server marks `skipped` shows the dash whatever else is true. The card's title and sub-lines are unchanged: a skip does not change '{n} of {goal} this week', which is the point.

Spoken (the card's one description): each day by its weekday name, 'Monday posted, Tuesday skipped, Wednesday not posted', and so on. Dash against dot against ring, never colour alone.

## 9. The entry page (view)

Route `app/(app)/today/entry`, opened with the entry's id. Pushed with a Back control. The bottom bar stays. The Phase 4 diary opens the same page from its own tab, so the page is a component (`EntryView`) with a thin route file.

Top to bottom, `lg` side margins, scrolling:
1. The Back row, 44dp.
2. `md` below: the photo, square, full width, 8dp radius, cover (`ImageReveal`). Label for VoiceOver: 'Photo for {title}'. If the photo cannot load, a `secondary` panel with the `image-outline` glyph and the Label 'Couldn't load this photo.' in `muted`.
3. `md` below: one row. Left: the category name as written, Label role, capitals by style, letter spacing 1.5, `muted`. Right: the date from `dateLine(local_date)` ('WED, OCT 8'), Label, `muted`, letter spacing 1.2.
4. If late, `xs` below: the Label 'Posted late.' in `muted`.
5. `sm` below: the title, Heading role, up to three lines, `ink`.
6. `sm` below: one row. The stars (read-only `StarDisplay`, 22dp glyphs) and the figure ('3.5') in Button role, then, at the right end, the chosen reaction character at 22/28 if there is one. One spoken element: '3.5 out of 5 stars. Reaction: Fire.' (no reaction: just the stars).
7. If there is a comment, `md` below: the comment, Body role, `ink`, line breaks kept.
8. `xl` below: two full-width action rows between 1dp `divider` hairlines (above the first, between, below the last), each at least 56dp: 'Edit' with a trailing Ionicons `chevron-forward` 20dp, and 'Delete' in `destructive` with no icon. Row text is Button role.

States: Loading shows the Label 'Loading your entry.'; a lost connection shows Phase 2's message and 'Try again' (`StatusMessage`); an entry that is gone (another phone deleted it) shows 'This entry isn't here any more.' with a 'Back to Today' text button. The page reloads when it comes back into focus, so an edit shows at once.

## 10. Edit entry

Route `app/(app)/today/edit-entry`, opened from the 'Edit' row with the entry's id. The same three-part layout and the same fields, rules, steps and messages as the entry screen, with these differences:
- The fixed header holds the Back row, the Heading 'Edit entry', and `xs` below it the category name as written in Label role, `muted`. The category is shown, not editable.
- Every field starts filled from the entry. The photo panel shows the current photo (from `photo_url`) with 'Change photo' under it. Choosing a new photo follows section 7; the new file is uploaded at Save.
- The button reads 'Save changes'. It is grey until something has changed and the form is valid (a photo, a title and stars are always present here, so 'Still needed' only appears if the title is cleared).
- Only changed fields are sent. Clearing the reaction or the comment is allowed.
- Leaving with changes opens the leave sheet. On success the screen goes back to the entry page, which reloads.
- If the server says the entry is gone, the message is 'This entry isn't here any more.' and the button becomes 'Back to Today'.

## 11. Delete

The 'Delete' row opens `ConfirmSheet`. Title (Heading): 'Delete this entry?'. Body: 'It will be gone for good. The day still counts as posted, so you can't post again for this day.' Rows: 'Delete' (text in `destructive`; `ConfirmSheet` gets one optional per-row flag for this) and 'Keep it'. 'Delete' sends the delete, closes the sheet and, on success, goes back (to Today when the page was opened from Today). Today then shows Posted with the deleted-entry line (3.3), the week card unchanged, and no way to post again that day. If the connection fails the sheet closes and the Label 'Couldn't connect. Check your connection and try again.' shows on the page. A 'not found' answer counts as done.

## 12. Motion

Calm before and after, as everywhere. Nothing new is invented.
- Taps on stars, reactions, 'Log it', 'Save' and the sheet rows use the existing press scale (0.97, 90ms).
- A chosen photo fades in with the existing 450ms `ImageReveal`. On the entry page the photo reveals first, then the title and details rise with `FadeInUp` (14dp, 400ms) 90ms apart.
- Today's heading swap to Posted or Skipped is Phase 2's 400ms fade. No confetti, flash, sound, vibration, counter animation or bounce.
- Reduce Motion on: no scale, fade or rise anywhere in this phase; things appear at once.

## 13. Copy contract

Every string the person sees. 'Spin' wording, plain words, no em dashes, no legal text. N marks a new string; E marks an existing app or Phase 2 string reused.

| Where | String |
|---|---|
| Today, Result and Late | Log it (N) / Skip this one (N) / Posting it now would count as late. (N) |
| Today, Posted | Posted. (N) / Next spin opens tomorrow at 8:00 PM. (E) / Next spin opens at 8:00 PM. (E) / Posted late. (N) / View entry (N) / This entry was deleted. The day still counts. (N) |
| Today, Skipped | Skipped. (N) / It doesn't count toward your week, and it isn't a miss. (N) |
| Skip sheet | Skip this one? / You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards. / Skip it / Don't skip (all N) |
| Entry screen, header | Back (E) / Your category (E) / Posting it now would count as late. (N) |
| Entry screen, labels | Photo / Title / Stars / Reaction / Comment (capitals by style) / Optional (E) |
| Entry screen, photo | Add a photo / Change photo / Take photo / Choose from library / Cancel (the last three E) |
| Entry screen, stars | Tap a star again for a half. (N) / the figure, for example 3.5 |
| Entry screen, footer | Still needed: photo, title and stars. (N, the list shrinks) / Save (E) / Back to Today (N) |
| Entry screen, empty | There is nothing to log right now. (N) |
| Reactions (spoken) | Heart / Laughing / Wow / Fire / Clap / Yum (N) |
| Photo messages | Use a JPG, PNG or WebP photo. (N) / That photo is over 50 MB. Choose a smaller one. (N) / Camera access is needed to take a photo. (E) / Photo library access is needed to choose a photo. (E) / Camera isn't available on this device. (E) / Couldn't open your photo library. (E) / Photo upload failed. Please try again. (E) / That file can't be used as a photo. Try another one. (E) |
| Leave sheet | Leave without saving? / Your changes will be lost. / Leave / Keep editing (all E, Phase 2) |
| Entry page | Posted late. / Edit / Delete / Loading your entry. / This entry isn't here any more. / Couldn't load this photo. / Back to Today (all N except the last, which repeats) |
| Edit entry | Edit entry (N) / Save changes (E) |
| Delete sheet | Delete this entry? / It will be gone for good. The day still counts as posted, so you can't post again for this day. / Delete / Keep it (all N) |
| Errors | Couldn't connect. Check your connection and try again. (E) / This day already has a post or a skip. (N) / This day has closed. It can't be posted or skipped any more. (N) / There is no spin for this day. (N) / Your spin changed on another phone. Go back to Today to see it. (N) / Something went wrong. Please try again. (E) |
| Retry button | Try again (E) |
| Date line | the server's date, for example WED, OCT 8 (E) |

## 14. Accessibility

- **Result and Late actions.** 'Log it' is a button with the hint 'Opens the entry screen.' 'Skip this one' is a button with the hint 'Asks first.'
- **Posted and Skipped.** The heading is a header element labelled 'Posted.' or 'Skipped.'. When the state first appears because of the person's own action, announce it once with `announceForAccessibility` ('Posted.' or 'Skipped.'). 'View entry' is a button.
- **Entry screen.** The category name is a header element labelled 'Your category: {name}.' Title and Comment are labelled by their fields; the count is read ('12 of 80 characters'). The photo panel is a button labelled 'Add a photo' or 'Change photo', hint 'Opens the camera or your library.' The 'Still needed' line is a polite live region.
- **Stars.** The row's container reads 'Stars, 3.5 out of 5' (or 'Stars, not rated'). Each star is a button labelled 'One star', 'Two stars', up to 'Five stars', hint 'Tap again for a half.' The steppers are labelled 'Lower rating by a half' and 'Raise rating by a half' and are disabled at their ends.
- **Reactions.** The row's container is labelled 'Reaction, optional'. Each button is labelled by its spoken name (Heart ... Yum) with state selected or not; a selected one has the hint 'Tap again to clear.'
- **Entry page.** Photo labelled 'Photo for {title}'. The stars and reaction are one element ('3.5 out of 5 stars. Reaction: Fire.'). 'Edit' and 'Delete' are buttons; Delete's hint is 'Asks first.'
- **Week dots.** The card's one description gains 'skipped' (section 8).
- **Targets.** Every tappable is at least 44dp (stars and steppers 44, reaction buttons 44, rows 56, panel full width).
- **Text size.** Wheel labels and digits stay fixed as in Phase 2. Everything in this phase scales with the phone's text size except the star glyphs and the six emoji, which are fixed.
- **Contrast.** `muted` on `dominant` 5.82:1, `ink` 17.32:1, `destructive` as in Phase 1.

## 15. Notes for the plan

- **Plan split.** 03-02 builds the pure logic, the API calls, the entry screen with its stars, reactions and photo field, 'Log it', the late line and the Posted state (its rows for Skipped are written too, so Today's state table is complete). 03-03 builds the entry page, edit, delete, the skip link and sheet, the 'View entry' link, the dash on the dots, and proves the Skipped state.
- **Wiring.** The entry screen reads Today from the same store Today uses, sends `for_date` (the shown spin's `local_date`) and `spin_seq` (its `seq`), and puts the answer's `today` back into the store. The phone never computes the date, the window, the late flag or the state.
- **Pure logic in pure files** (`lib/entry/rules.ts`, `stars.ts`, `reactions.ts`, `photo.ts`), checked by a throwaway node script and by device steps.
- **Launch.** No launch file is edited. The three new routes are added to `app/(app)/today/_layout.tsx` only. One cold launch is checked after they exist.
- **Never run `expo lint`.** No new native module. Tokens only, no hex. The emoji scan allows exactly one file, `lib/entry/reactions.ts`.

## 16. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plans). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 13.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Today, Result and Late | Log it / Skip this one / Posting it now would count as late. | The first two are from your notes. |
| 2 | Today, Posted | Posted. / Posted late. / View entry / This entry was deleted. The day still counts. | New. |
| 3 | Today, Skipped | Skipped. / It doesn't count toward your week, and it isn't a miss. | New. |
| 4 | Skip sheet | Skip this one? / You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards. / Skip it / Don't skip | New. The sheet shows your count before you decide. |
| 5 | Entry screen | Your category / Photo / Title / Stars / Reaction / Comment / Optional / Add a photo / Change photo / Tap a star again for a half. / Still needed: photo, title and stars. / Save | New. |
| 6 | Photo | Use a JPG, PNG or WebP photo. / That photo is over 50 MB. Choose a smaller one. | New. |
| 7 | Errors | This day already has a post or a skip. / This day has closed. It can't be posted or skipped any more. / There is no spin for this day. / Your spin changed on another phone. Go back to Today to see it. | New. |
| 8 | Entry page | Edit / Delete / Loading your entry. / This entry isn't here any more. / Couldn't load this photo. / Edit entry / Save changes | New. |
| 9 | Delete sheet | Delete this entry? / It will be gone for good. The day still counts as posted, so you can't post again for this day. / Delete / Keep it | New. |
| 10 | Reactions | Heart, Laughing, Wow, Fire, Clap, Yum, with the six characters above | Your choice of six. These are the characters I picked for each name; say if you want others. |

**Choices made here that you may want to change:**
- The photo shows as a square, centre-cropped, on the entry screen and the entry page.
- 'Save' on the entry screen and 'Save changes' on edit, as on the profile page. 'Log it' only starts the entry.
- A second tap on a star adds a half (3 becomes 3.5), as you were told. Star 5 goes to 4.5. There are minus and plus buttons beside the stars, and 0.5 is reached with the minus button.
- The six reaction characters draw in the system's own colours. They appear only in the picker and where a reaction is shown.
- The skip mark on the dots is a short dark dash.
- No hint words inside the Title and Comment boxes.
- After midnight, a post or skip you make in the late window stays on Today ('Posted.' or 'Skipped.') until the next spin opens. One made before midnight goes back to 'Your next spin is coming.' after midnight.
- The photo limit is 50 MB, the same as profile photos (your pick of 2026-10-09). A normal full-size library photo is accepted.
