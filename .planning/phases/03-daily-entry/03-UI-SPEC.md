---
phase: 3
slug: daily-entry
status: draft
created: 2026-10-10
revision: 2
approval_word: go
---

# Phase 3 - UI Design Contract (Daily Entry, "log it")

> Design contract the Phase 3 app plans (03-02, 03-03) build from. PHASE 3. NOTHING IS BUILT. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later), and the Today, Edit wheel and sheet parts of `02-daily-roll/02-UI-SPEC.md` (revision 6 or later, 2026-10-10: the new Today pictures, `docs/reference/today-screen-1a-1b-2026-10-10.png`, `docs/reference/today-after-spin-02-2026-10-10.png` and `docs/reference/today-skipped-and-entry-05-06-2026-10-10.png`; Phase 2 is not built either). No new font, colour or radius is added (section 2). **Revision 2 (2026-10-10):** the developer sent the screens for after the spin, skipped and the entry page, saying "this is how it should look after the spin". Today after the spin has no wheel: the result word fills the screen with a 'Spun at' stamp and the buttons under it (3.1, 3.2). Skipped shows a sentence under the heading and the word struck through (3.4). Posted follows Skipped's layout until the developer's picture 04 arrives (3.3). The entry page has no bottom bar, an edge-to-edge photo and a pinned Edit and Delete footer (section 9). The six reactions are the six in the developer's pictures: love, fire, laughing, wow, sad, sleepy (section 6). Every override of an older rule is in the choices list at the end of section 16. The only non-token thing on screen is the set of six reaction emoji, which the developer chose as content (section 6). All imagery is the developer's: photos are the person's own and every placeholder is plain.

## 1. What this phase puts on screen

| Screen or state | Where | New or changed |
|---|---|---|
| Result and Late on Today | `components/today` | Changed: after the spin there is no wheel (Phase 2 builds the word and its 'Spun at' stamp); this phase adds the late line, the 'Log it' button and the 'Skip this one' link under them. |
| Posted on Today | same | New state. It follows Skipped's layout with the word not struck, until the developer's Posted picture arrives. |
| Skipped on Today | same | New state: a sentence under the heading, and the word struck through. |
| Skip sheet | over Today | New. It is Phase 2's `ConfirmSheet`. |
| Entry screen | `app/(app)/today/log` | New. 'Log it' opens it. |
| Entry page (view) | `app/(app)/today/entry` | New. No bottom bar: a pinned Edit and Delete footer replaces it. The Phase 4 diary opens the same page. |
| Edit entry | `app/(app)/today/edit-entry` | New. |
| Delete sheet | over the entry page | New. `ConfirmSheet` again. |
| Week dots | `components/today/WeeklyCard.tsx` (Phase 2's week section; the file keeps its name) | Changed: a skipped day gets a dash. |
| Bottom bar | `app/(app)/_layout.tsx` | Unchanged: Home, Spin, Profile (the pictures show the final five tabs; Diary comes in Phase 4 and Board in Phase 5). It is not drawn on the entry page (section 9). |

All new routes sit inside the Home stack (`app/(app)/today/`), next to Today and Edit wheel. Under Tabs a file placed directly in `app/(app)/` becomes a tab, so none is.

## 2. Tokens added (the only new design values)

No new colour, space, radius or font. Every value comes from `lib/theme/tokens.ts` as Phase 2 leaves it. Phase 2's token task defines all the text roles up front, so nothing edits the tokens twice. This phase uses four of them: `hero` (`Domine_400Regular` 100/104, the result word, built by Phase 2), `heroSmall` (80/84, the word in Posted and Skipped), `note` (20/28, the comment on the entry page) and `title` (32/38, the Today heading and the entry page title). `wheelMid`, `countdown` and `wheelLabel` are not used here. The entry page's date line uses `dateLineLong` (Phase 2, `lib/wheel/format.ts`), not the short `dateLine`.

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
| Today heading, entry page title | `title` role (Domine Regular 32/38) |
| The result word | `hero` role (100/104), `ink` |
| The word in Posted and Skipped | `heroSmall` role (80/84); `ink`, or `inkInactive` with a 2dp `ink` strike line when struck |
| The comment on the entry page | `note` role (20/28), `ink` |
| Footer on the entry page | `dominant` fill, `divider` hairlines, Button role text |

Radius allow-lists are unchanged: Phase 3 adds no use of `radius.lg` or `radius.full`. The photo panel, the star buttons, the stepper buttons and the reaction buttons are all tappable, so all are 8dp or less. The delete and skip sheets are Phase 2's `ConfirmSheet` (already on the `radius.lg` list).

**The one sanctioned colour.** The six reaction characters are system emoji and draw in the system's colour (yellow faces, a red flame). That is the developer's choice (2026-10-09; the six are the ones in their picture of 2026-10-10). It is allowed in exactly two places: the six buttons of the reaction picker, and where an author's chosen reaction is shown (the entry page). It is never used as an icon, a bullet, a button label or decoration anywhere else.

## 3. Today: Result, Late, Posted, Skipped

The phone shows what the server says. It never works out the date, the window, the state or whether a post is late.

### 3.1 States (rows that change or are new; every other row of Phase 2's table 4.1 stands)

| State | When | Heading | Under the heading | The word block | Wheel |
|---|---|---|---|---|---|
| Result | spun today, not posted, not skipped | Today's spin. | nothing | the word (`hero`) with the stamp row 'Spun at {time}', then the day actions (3.2) | not drawn |
| Late | after midnight, before the next 8:00 PM, last night's spin not posted and not skipped | Last night's spin. | nothing | the word with the stamp row, then the late line and the day actions (3.2) | not drawn |
| Posted | the shown spin's day has a post | Posted. | Your next spin opens {weekday} at 8:00 PM. | the label 'Today's spin was' and the word in `heroSmall`, not struck, then the posted block (3.3) | not drawn |
| Skipped | the shown spin's day was skipped | Skipped. | Your next spin opens {weekday} at 8:00 PM. | the label 'Today's spin was' and the word in `heroSmall`, struck through (3.4) | not drawn |

Notes:
- **No wheel (Phase 2 revision 6, 2026-10-10).** In all four states the wheel and its pointer are not drawn: after the spin the word takes the wheel's place (Phase 2 builds that look and the reveal). So there is no hub to tap and no 'Edit wheel' link in these states: the link shows only while the wheel is drawn.
- **Where the sub-line sits.** A sentence sits under the heading ONLY in Posted and Skipped (Body role, 16/24, `ink`). Result and Late have nothing under the heading: the stamp row under the word takes that job, and the old 'Next spin opens ...' lines are gone from them.
- **Which sentence.** The app compares two server date strings: when `spin.local_date` equals `local_date`, it writes 'Your next spin opens {weekday} at 8:00 PM.' with `next_open_weekday` from the server; when the shown spin is last night's, it writes 'Your next spin opens at 8:00 PM.' and the label reads 'Last night's spin was'. The space before PM is a no-break space so 'PM.' never stands alone on a line. No `Date` object is used and the phone never works out a weekday.
- **How long Posted and Skipped show.** A post or skip made before midnight shows until midnight; after midnight Today is back to Phase 2's Waiting ('Your next spin is coming.', countdown). A post or skip made after midnight, in the late window, keeps Posted or Skipped up (with last night's spin) until the next spin opens at 8:00 PM, because the person has just acted and should see it land. The server decides this (`state`); the phone only draws it.
- The 'Use bonus spin' link is gone (the server sends `can_use` false once the day is settled).
- After the person's own Log it or Skip it, the answer carries the new Today, so the screen changes at once with no loading step. The heading and everything under it cross-fade over Phase 2's 400ms; the word is not animated in size.
- The middle Spin button in these states goes to Home and shows today's spin, and nothing spins. Its spoken hint is Phase 2's 'Shows today's spin.'. It is drawn outlined (locked) in Posted and Skipped, as in every state except open (Phase 2 section 8, revision 5).

### 3.2 The day actions (Result and Late)

The picture ('02 Today, after the spin'). Under the word and its stamp row (Phase 2 section 4.3, the Result block), inside the `lg` side margins, top to bottom:
1. The 'Use bonus spin' link keeps its place, centred, `md` below the stamp row, when it shows (Phase 2 section 4.3).
2. **The late line**, Late state only: Label role, `muted`, centred: 'Posting it now would count as late.' `md` below the stamp row (below the bonus link when that shows).
3. **'Log it'**: `PrimaryButton`, standard variant, no arrow, full width of the content column, label 'Log it'. `xxl` (48) below the stamp row; when the late line or the bonus link shows, `md` below the last of them. It pushes the entry screen (section 4).
4. **'Skip this one'**: `TextButton`, tone `muted`, underlined, centred, `md` below the button, 44dp hit area. It opens the skip sheet (3.5).
5. The week section (Phase 2 section 6, `WeeklyCard.tsx`, file name unchanged) sits at the bottom of the page after a flexible space of at least `xl`: on a tall phone its top rule is about 100dp under 'Skip this one', as in the picture.

There is no 'Edit wheel' link here (3.1). While a skip request is running the button and the link are disabled.

### 3.3 The posted block

There is no picture for Posted yet (the developer's screens 03 and 04 have not arrived), so it follows Skipped's composition (3.4) with the word in `ink` and not struck: the heading 'Posted.', the sentence under it, then, centred vertically in the space between the sentence and the week section, the label 'Today's spin was' ('Last night's spin was' in the late window) and the word in `heroSmall`. Under the word, left aligned with it:
- If the entry was posted late (`entry.late` from the server): the Label 'Posted late.' in `muted`.
- If the day has an entry: the text link 'View entry' (Ink, underlined, `WorkSans_600SemiBold`, 44dp high), `md` below. It opens the entry page.
- If the day is posted but the entry was deleted (`entry` is null): the Label 'This entry was deleted. The day still counts.' in `muted`. No link.

(A pretend posted day made with the test tools also reaches the third case. Real use does not.) When the developer's Posted picture arrives this section changes to match it.

### 3.4 The skipped block

The picture ('05 Today, skipped'). The page is: the heading 'Skipped.'; under it the sentence of 3.1 (Body role, `ink`); then, centred vertically in the space between that sentence and the week section, left aligned on the `lg` margin: the Label 'Today's spin was' (capitals by style, `muted`, letter spacing 1.5; 'Last night's spin was' in the late window) and, `sm` below it, the word in the `heroSmall` role (`Domine_400Regular` 80/84), colour `inkInactive` (very light, about 1.6:1 by design), one line that shrinks to fit as Phase 2's `hero` does, with a 2dp `ink` strike line through the middle of the word, 4dp longer than the word at both ends. Then the week section, which shows the dash for the skipped day, today's included.

No button, no 'Edit wheel' link and no reassurance line: the old line 'It doesn't count toward your week, and it isn't a miss.' is no longer on the page (the skip sheet, 3.5, keeps its own text). There is no undo: a skip is final.

### 3.5 The skip sheet

`ConfirmSheet` (Phase 2: same modal, scrim, top corners and padding as the Log out sheet). Title (Heading role): 'Skip this one?'. Body (Body role): 'You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards.' where {n} is `week.posted` and {goal} is `week.goal` from the server. When the shown spin is from the week before the week section's week (`spin.local_date` is earlier than `week.start`, a plain string compare: a skip just after midnight on Monday for Sunday's spin), the first sentence is left out, because the week section is already showing the new week. Two full-width rows, each at least 56dp, separated by hairlines: 'Skip it' and 'Don't skip'. 'Skip it' closes the sheet and sends the skip. No reason is asked, ever.

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

Label row: 'Reaction' on the left, 'Optional' (hint style) on the right. Under it, six buttons in one row, spread across the column, each 44dp square with an 8dp radius. The character is drawn in a Text at the Heading size and line height (22/28), system emoji font, `allowFontScaling` off. On the entry page the chosen character is drawn at 24dp (section 9).

| Name stored and sent | Character | Code point | Spoken |
|---|---|---|---|
| love | 😍 | U+1F60D | Love |
| fire | 🔥 | U+1F525 | Fire |
| laughing | 😂 | U+1F602 | Laughing |
| wow | 😮 | U+1F62E | Wow |
| sad | 😢 | U+1F622 | Sad |
| sleepy | 😴 | U+1F634 | Sleepy |

These six are the reaction row in the developer's picture of 2026-10-10 (screen 10: love, fire, laughing, wow, sad, sleepy, in that order), and the entry page picture shows the love face in the reaction slot. Phase 5 uses the same six and the same names. They replace the six the developer tapped on 2026-10-09 and again on 2026-10-10 (heart, laughing, wow, fire, clap, yum), because the pictures are newer. The list is one app file and one server list, so changing it back is a paper edit.

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

## 8. The week dots (a change to Phase 2 section 6, revision 5)

Four states now. The dot slot stays 12dp, drawn with react-native-svg, not a control. The new Today picture (2026-10-10) draws these four marks; Phase 2 draws three of them and this phase adds the dash.

| State | Mark | Letter above |
|---|---|---|
| Posted | filled `ink` dot, 10dp | `muted` |
| Skipped | a short dash: `ink`, 10dp wide, 2dp tall, centred in the slot, square ends | `muted` |
| Today, not posted, not skipped | ring in `ink`, 10dp, 1.5dp stroke | `muted` |
| Any other day with nothing (a day still to come, or a past day with no post and no skip) | small `divider` dot, 5dp | `muted` |

Order of precedence: posted, then skipped, then today's ring, then the small dot. A day the server marks `skipped` shows the dash whatever else is true. Every letter stays `muted`: today is shown by its ring, not by its letter. The week row's count and the bonus line are unchanged: a skip does not change '{n} of {goal} this week', which is the point.

Spoken (the week section's one description): each day by its weekday name, 'Monday posted, Tuesday skipped, Wednesday not posted', and so on. Dash against dot against ring, never colour alone.

## 9. The entry page (view)

The picture ('06 Entry page'). Route `app/(app)/today/entry`, opened with the entry's id and a `from` value. Pushed over Today with a Back control. The bottom bar is NOT drawn on this page: a pinned footer replaces it. The Phase 4 diary opens the same page from its own tab (passing `from` = 'Diary'), so the page is a component (`EntryView`) with a thin route file. The Phase 3 build only ever passes `from` = 'Today'.

Top to bottom:
1. The Back row, 44dp, inside the `lg` side margins: the chevron and the name of where the person came from, in the Body role: 'Today' when opened from Today, 'Diary' when opened from the diary.
2. Directly under it, with no gap: the photo, square, the full width of the screen edge to edge (no side margins, no radius), cover (`ImageReveal`). Label for VoiceOver: 'Photo for {title}'. If the photo cannot load, a `secondary` panel of the same size with the `image-outline` glyph and the Label 'Couldn't load this photo.' in `muted`.
3. From here on inside the `lg` side margins. `lg` below the photo: one Label line, capitals by style, letter spacing 1.5, `muted`: the category name as written, then a middle dot, then the long date from `dateLineLong(local_date)`, for example 'MEAL · THURSDAY, OCTOBER 8'. The category name is cut with an ellipsis first if the line is too long. (Revision 1 kept the short date here; the picture shows the long one fits.)
4. If late, `xs` below: the Label 'Posted late.' in `muted`.
5. `sm` below: the title in the `title` role (32/38), `ink`, up to three lines.
6. `md` below: one row. The stars (read-only `StarDisplay`, 20dp glyphs: filled and half in `ink`, empty outlines in `muted`), the figure ('4.5') in the Body role, a 1dp by 20dp `divider` rule, then the chosen reaction character at 24dp. No reaction: no rule and no character. One spoken element: '3.5 out of 5 stars. Reaction: Fire.' (no reaction: just the stars).
7. If there is a comment, `md` below: the comment in the `note` role (20/28), `ink`, line breaks kept.
8. `xl` of space at the end, clear of the footer. The page scrolls; the Back row and the footer do not.

**Pinned footer** (on `dominant`, above the bottom safe area, which it includes): a 1dp `divider` hairline on top, then two equal halves with a 1dp `divider` vertical rule between them, each at least 56dp high: 'Edit' (Button role, `ink`) on the left and 'Delete' (Button role, `destructive`) on the right. No chevron and no icon. 'Edit' opens edit entry (section 10); 'Delete' opens the delete sheet (section 11).

States: Loading shows the Label 'Loading your entry.'; a lost connection shows Phase 2's message and 'Try again' (`StatusMessage`); an entry that is gone (another phone deleted it) shows 'This entry isn't here any more.' with a 'Back to Today' text button. In these three states the footer is not drawn. The page reloads when it comes back into focus, so an edit shows at once.

## 10. Edit entry

Route `app/(app)/today/edit-entry`, opened from the 'Edit' button in the entry page footer with the entry's id. The same three-part layout and the same fields, rules, steps and messages as the entry screen, with these differences:
- The fixed header holds the Back row, the Heading 'Edit entry', and `xs` below it the category name as written in Label role, `muted`. The category is shown, not editable.
- Every field starts filled from the entry. The photo panel shows the current photo (from `photo_url`) with 'Change photo' under it. Choosing a new photo follows section 7; the new file is uploaded at Save.
- The button reads 'Save changes'. It is grey until something has changed and the form is valid (a photo, a title and stars are always present here, so 'Still needed' only appears if the title is cleared).
- Only changed fields are sent. Clearing the reaction or the comment is allowed.
- Leaving with changes opens the leave sheet. On success the screen goes back to the entry page, which reloads.
- If the server says the entry is gone, the message is 'This entry isn't here any more.' and the button becomes 'Back to Today'.

## 11. Delete

The 'Delete' button in the entry page footer opens `ConfirmSheet`. Title (Heading): 'Delete this entry?'. Body: 'It will be gone for good. The day still counts as posted, so you can't post again for this day.' Rows: 'Delete' (text in `destructive`; `ConfirmSheet` gets one optional per-row flag for this) and 'Keep it'. 'Delete' sends the delete, closes the sheet and, on success, goes back (to Today when the page was opened from Today). Today then shows Posted with the deleted-entry line (3.3), the week section unchanged, and no way to post again that day. If the connection fails the sheet closes and the Label 'Couldn't connect. Check your connection and try again.' shows on the page. A 'not found' answer counts as done.

## 12. Motion

Calm before and after, as everywhere. Nothing new is invented.
- Taps on stars, reactions, 'Log it', 'Save' and the sheet rows use the existing press scale (0.97, 90ms).
- A chosen photo fades in with the existing 450ms `ImageReveal`. On the entry page the photo reveals first, then the title and details rise with `FadeInUp` (14dp, 400ms) 90ms apart; the pinned footer is there at once.
- After a spin, Phase 2's reveal runs: the wheel fades out, the word rises 14dp while fading in, and the stamp row, then the buttons, follow 120ms apart. Result to Posted or Skipped: the heading and everything under it cross-fade over Phase 2's 400ms; there is no resize or slide animation. No confetti, flash, sound, vibration, counter animation or bounce.
- Reduce Motion on: no scale, fade or rise anywhere in this phase; things appear at once.

## 13. Copy contract

Every string the person sees. 'Spin' wording, plain words, no em dashes, no legal text. N marks a new string; E marks an existing app or Phase 2 string reused.

| Where | String |
|---|---|
| Today, Result and Late | Log it (N) / Skip this one (N) / Posting it now would count as late. (N) / Spun at {time} (E, Phase 2: capitals by style, the time written by the server, for example 8:02 PM) |
| Today, Posted | Posted. (N) / Your next spin opens {weekday} at 8:00 PM. (N, with a no-break space before PM) / Your next spin opens at 8:00 PM. (N, late window) / Today's spin was (N) / Last night's spin was (N) / Posted late. (N) / View entry (N) / This entry was deleted. The day still counts. (N) |
| Today, Skipped | Skipped. (N) / Your next spin opens {weekday} at 8:00 PM. (N, as Posted) / Your next spin opens at 8:00 PM. (N, late window) / Today's spin was (N) / Last night's spin was (N) |
| Skip sheet | Skip this one? / You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards. / Skip it / Don't skip (all N) |
| Entry screen, header | Back (E) / Your category (N: Phase 2's Today no longer shows this label) / Posting it now would count as late. (N) |
| Entry screen, labels | Photo / Title / Stars / Reaction / Comment (capitals by style) / Optional (E) |
| Entry screen, photo | Add a photo / Change photo / Take photo / Choose from library / Cancel (the last three E) |
| Entry screen, stars | Tap a star again for a half. (N) / the figure, for example 3.5 |
| Entry screen, footer | Still needed: photo, title and stars. (N, the list shrinks) / Save (E) / Back to Today (N) |
| Entry screen, empty | There is nothing to log right now. (N) |
| Reactions (spoken) | Love / Fire / Laughing / Wow / Sad / Sleepy (N) |
| Photo messages | Use a JPG, PNG or WebP photo. (N) / That photo is over 50 MB. Choose a smaller one. (N) / Camera access is needed to take a photo. (E) / Photo library access is needed to choose a photo. (E) / Camera isn't available on this device. (E) / Couldn't open your photo library. (E) / Photo upload failed. Please try again. (E) / That file can't be used as a photo. Try another one. (E) |
| Leave sheet | Leave without saving? / Your changes will be lost. / Leave / Keep editing (all E, Phase 2) |
| Entry page | Posted late. / Edit / Delete / Loading your entry. / This entry isn't here any more. / Couldn't load this photo. / Back to Today (all N except the last, which repeats) / the Back label: Today or Diary (N, it names where the person came from) |
| Edit entry | Edit entry (N) / Save changes (E) |
| Delete sheet | Delete this entry? / It will be gone for good. The day still counts as posted, so you can't post again for this day. / Delete / Keep it (all N) |
| Errors | Couldn't connect. Check your connection and try again. (E) / This day already has a post or a skip. (N) / This day has closed. It can't be posted or skipped any more. (N) / There is no spin for this day. (N) / Your spin changed on another phone. Go back to Today to see it. (N) / Something went wrong. Please try again. (E) |
| Retry button | Try again (E) |
| Date line, Today | the server's date in full, for example THURSDAY, OCTOBER 8 (E, Phase 2 revision 5) |
| Date, entry page | the long form after the category, for example MEAL · THURSDAY, OCTOBER 8 (E: Phase 2's `dateLineLong`; the short `dateLine` ('WED, OCT 8') is still Phase 2's helper but this page no longer uses it) |

## 14. Accessibility

- **Result and Late.** The word is a header element labelled 'Today's spin: {name}.' (Late: 'Last night's spin: {name}.'). The stamp row is one element, 'Spun at 8:02 PM' (Phase 2). The wheel is not in the accessibility tree. 'Log it' is a button with the hint 'Opens the entry screen.' 'Skip this one' is a button with the hint 'Asks first.'
- **Posted and Skipped.** The heading is a header element labelled 'Posted.' or 'Skipped.'. When the state first appears because of the person's own action, announce it once with `announceForAccessibility` ('Posted.' or 'Skipped.'). 'View entry' is a button. The sentence under the heading is read after it. The label and the word are one element: in Skipped 'Today's spin was Meal. Skipped.' (late window: 'Last night's spin was Meal. Skipped.'); in Posted 'Today's spin was Meal.'
- **Entry screen.** The category name is a header element labelled 'Your category: {name}.' Title and Comment are labelled by their fields; the count is read ('12 of 80 characters'). The photo panel is a button labelled 'Add a photo' or 'Change photo', hint 'Opens the camera or your library.' The 'Still needed' line is a polite live region.
- **Stars.** The row's container reads 'Stars, 3.5 out of 5' (or 'Stars, not rated'). Each star is a button labelled 'One star', 'Two stars', up to 'Five stars', hint 'Tap again for a half.' The steppers are labelled 'Lower rating by a half' and 'Raise rating by a half' and are disabled at their ends.
- **Reactions.** The row's container is labelled 'Reaction, optional'. Each button is labelled by its spoken name (Love ... Sleepy) with state selected or not; a selected one has the hint 'Tap again to clear.'
- **Entry page.** Photo labelled 'Photo for {title}'. The stars and reaction are one element ('3.5 out of 5 stars. Reaction: Fire.'). 'Edit' and 'Delete' are buttons in the footer, each at least 56dp high; Delete's hint is 'Asks first.' The Back control is labelled 'Back to Today' or 'Back to Diary' (the visible word is the place). The category and date line is one element.
- **Week dots.** The week section's one description gains 'skipped' (section 8).
- **Targets.** Every tappable is at least 44dp (stars and steppers 44, reaction buttons 44, sheet rows and footer buttons 56, panel full width).
- **Text size.** Wheel labels and digits stay fixed as in Phase 2. Everything in this phase scales with the phone's text size except the star glyphs, the six emoji and the two big words (`hero` and `heroSmall`), which keep their size and only shrink to fit.
- **Contrast.** `muted` on `dominant` 5.82:1, `ink` 17.32:1, `destructive` as in Phase 1. The struck word in `inkInactive` is about 1.6:1 by design: it is a recap, it is read aloud, and the heading and the sentence carry the same facts.

## 15. Notes for the plan

- **Plan split.** 03-02 builds the pure logic, the API calls, the entry screen with its stars, reactions and photo field, 'Log it', the late line and the Posted state (its rows for Skipped are written too, so Today's state table is complete). 03-03 builds the entry page, edit, delete, the skip link and sheet, the 'View entry' link, the dash on the dots, and proves the Skipped state.
- **Wiring.** The entry screen reads Today from the same store Today uses, sends `for_date` (the shown spin's `local_date`) and `spin_seq` (its `seq`), and puts the answer's `today` back into the store. The phone never computes the date, the window, the late flag or the state. The sentence under the heading in Posted and Skipped uses the server's `next_open_weekday` (Phase 2's Today shape); the entry page route takes a `from` value ('Today' now, 'Diary' from Phase 4) for its Back label.
- **Pure logic in pure files** (`lib/entry/rules.ts`, `stars.ts`, `reactions.ts`, `photo.ts`), checked by a throwaway node script and by device steps.
- **Launch.** No launch file is edited. The three new routes are added to `app/(app)/today/_layout.tsx` only. One cold launch is checked after they exist.
- **Never run `expo lint`.** No new native module. Tokens only, no hex. The emoji scan allows exactly one file, `lib/entry/reactions.ts`.

## 16. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plans). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 13.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Today, Result and Late | Log it / Skip this one / Posting it now would count as late. | The first two are from your notes and your after-the-spin picture. 'Spun at {time}' is in Phase 2's strings. |
| 2 | Today, Posted | Posted. / Your next spin opens {weekday} at 8:00 PM. / Today's spin was / Posted late. / View entry / This entry was deleted. The day still counts. | New. The sentence is from your skipped picture. |
| 3 | Today, Skipped | Skipped. / Your next spin opens {weekday} at 8:00 PM. / Today's spin was / Last night's spin was | From your skipped picture. The old line 'It doesn't count toward your week, and it isn't a miss.' is gone from the page (the skip sheet keeps it). |
| 4 | Skip sheet | Skip this one? / You are at {n} of {goal} this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards. / Skip it / Don't skip | New. The sheet shows your count before you decide. |
| 5 | Entry screen | Your category / Photo / Title / Stars / Reaction / Comment / Optional / Add a photo / Change photo / Tap a star again for a half. / Still needed: photo, title and stars. / Save | New. |
| 6 | Photo | Use a JPG, PNG or WebP photo. / That photo is over 50 MB. Choose a smaller one. | New. |
| 7 | Errors | This day already has a post or a skip. / This day has closed. It can't be posted or skipped any more. / There is no spin for this day. / Your spin changed on another phone. Go back to Today to see it. | New. |
| 8 | Entry page | Edit / Delete / Loading your entry. / This entry isn't here any more. / Couldn't load this photo. / Today or Diary (the Back label) / Edit entry / Save changes | New. |
| 9 | Delete sheet | Delete this entry? / It will be gone for good. The day still counts as posted, so you can't post again for this day. / Delete / Keep it | New. |
| 10 | Reactions | Love, Fire, Laughing, Wow, Sad, Sleepy, with the six characters above | The six reactions are the six in your picture (love, fire, laughing, wow, sad, sleepy), not the six you tapped on 2026-10-09 and 2026-10-10 (heart, laughing, wow, fire, clap, yum). Phase 5's paper uses the same six. Say if you want your tapped six back: it is one file and one list. |

**Choices made here that you may want to change:**
- The photo shows as a square, centre-cropped, on the entry screen and the entry page.
- 'Save' on the entry screen and 'Save changes' on edit, as on the profile page. 'Log it' only starts the entry.
- A second tap on a star adds a half (3 becomes 3.5), as you were told. Star 5 goes to 4.5. There are minus and plus buttons beside the stars, and 0.5 is reached with the minus button.
- The six reaction characters draw in the system's own colours. They appear only in the picker and where a reaction is shown.
- The skip mark on the dots is a short dark dash.
- No hint words inside the Title and Comment boxes.
- After midnight, a post or skip you make in the late window stays on Today ('Posted.' or 'Skipped.') until the next spin opens. One made before midnight goes back to 'Your next spin is coming.' after midnight.
- The photo limit is 50 MB, the same as profile photos (your pick of 2026-10-09). A normal full-size library photo is accepted.
- **From your pictures of 2026-10-10 (each overrides an older rule):**
- After the spin there is no wheel. The word fills the screen as in your picture (about 100dp tall) with a 'Spun at' stamp, and 'Log it' and 'Skip this one' sit under it. Before, the wheel stayed with the word under it. 'Edit wheel' shows only while the wheel is on screen.
- A skipped day shows the heading, the sentence 'Your next spin opens Friday at 8:00 PM.' under it, and the word struck through in very light grey. The reassurance line is gone from the page; the skip sheet keeps it.
- Posted has no picture yet (your 03 and 04 are not here). It uses Skipped's layout with the word not struck and 'View entry' under it. Send picture 04 and it changes.
- The entry page has no bottom bar. A footer with Edit and Delete side by side replaces it, the photo runs edge to edge with square corners, and the date is written in full beside the category. Before: two full-width rows, an inset rounded photo and the short date.
- The Back label names where you came from: Today now, Diary from Phase 4.
- The six reactions are the six in your picture (love, fire, laughing, wow, sad, sleepy), not the six you tapped on 2026-10-09 and 2026-10-10 (heart, laughing, wow, fire, clap, yum). Phase 5's paper uses the same six. Say if you want your tapped six back: it is one file and one list.
- The bar in your pictures has five tabs, says 'Roll' and has a grey filled round middle button. That is not copied: the bar keeps Home, Spin and Profile until Diary (Phase 4) and Board (Phase 5), the word is Spin, and the Spin button keeps your outlined and solid black looks.
