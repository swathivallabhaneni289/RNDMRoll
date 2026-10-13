---
phase: 4
slug: personal-diary
status: draft
created: 2026-10-10
revision: 1
approval_word: go
---

# Phase 4 - UI Design Contract (Personal Diary and the Profile page)

> Design contract the Phase 4 app plan (04-02) builds from. PHASE 4. NOTHING IS BUILT. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later), the bar, the Profile tab and the first-run page of `02-daily-roll/02-UI-SPEC.md` (revision 6 or later) and the entry page, the stars and the reaction file of `03-daily-entry/03-UI-SPEC.md` (revision 2 or later). The Profile page also mounts two components of `02.1-settings/02.1-UI-SPEC.md` (revision 1), the Settings gear and the bio in place, which Phase 2.1 builds and this phase does not edit. Phases 2, 2.1 and 3 are not built either. The developer's pictures of 2026-10-10 are the layout source: screen 07 (Diary), screen 12 (Profile) and, for the bar, screens 08 to 11 (`docs/reference/screens-07-diary-08-board-locked-2026-10-10.png`, `docs/reference/screens-11-notifications-12-profile-2026-10-10.png`, `docs/reference/screens-03a-home-03b-diary-2026-10-10.png`). Where a picture and an older paper rule disagree the picture wins and the override is listed at the end of section 10 (the developer's rule of 2026-10-10). All imagery is the developer's: every photo is the person's own and every placeholder is plain. One text role is added (section 2) and one file joins the round-shape allow-list (section 2).

## Visual reference (binding)

The developer's pictures named above are the visual reference for the build ("use exactly the same visuals ... as a reference to build this project", 2026-10-10). The plans build to match them, and at each screen's device check the phone screenshot is compared side by side with its picture. Every difference that is not in the "Choices made here that you may want to change" list at the end of this document is a defect. Where this document and a picture disagree, the picture wins and this document is corrected.

## 1. What this phase puts on screen

| Screen or state | Where | New or changed |
|---|---|---|
| Diary tab | `app/(app)/diary/index.tsx`, component `components/diary/DiaryScreen.tsx` | New. Heading, category chips, the list, paging, empty and offline states. |
| Entry page (from the Diary) | `app/(app)/diary/entry.tsx` | New thin route around Phase 3's `EntryView`, Back label 'Diary'. |
| Edit entry (from the Diary) | `app/(app)/diary/edit-entry.tsx` | New thin route around Phase 3's edit screen. |
| Profile page (view) | `app/(app)/profile/index.tsx`, component `components/profile/ProfileScreen.tsx` | New. Replaces the form as the Profile tab's root after first run. Mounts Phase 2.1's gear and bio in place (4.1). |
| Settings gear | `components/settings/SettingsGear.tsx` (Phase 2.1), at the far right of the Profile page's photo row | Not built here. Opens Phase 2.1's Settings page (`app/(app)/profile/settings`). Not in picture 12. |
| Bio in place | `components/settings/BioInline.tsx` (Phase 2.1), under the photo row, label off | Not built here. Not in picture 12. |
| Entry page and Edit entry (from the Profile) | `app/(app)/profile/entry.tsx`, `edit-entry.tsx` | New thin routes, Back label 'Profile'. |
| Edit profile page | `app/(app)/profile/edit.tsx` | The existing Phase 1 form page in edit mode, pushed from the Profile page's Edit chip (Settings has no 'Edit profile' row since decision D-16 of Phase 2.1). One change: its bottom button is 'Save changes' only, and its photo link reads 'Edit your picture' (section 4.6). |
| First-run page | `app/(app)/profile/index.tsx` while `extrasPending` | Unchanged (Phase 1's page 2, no bar, ends on Today). |
| Bottom bar | `app/(app)/_layout.tsx` | Changed: Home, Diary, Spin, Profile (four). Diary is new. |
| Today | `components/today/*` | Unchanged. The week section stays display only. |

New routes sit inside their own tab's stack, so a push never switches tabs. Under Tabs a file placed directly in `app/(app)/` becomes a tab: the only one added is the `diary` folder (a Stack), which is the new tab on purpose. Phase 2.1's pages (Settings, Notifications, Blocked accounts, Password; the Username page was removed by its decision D-16) sit in the Profile stack beside these and are not part of this phase.

## 2. Tokens added (the only new design values)

**One text role.** In the `type` group of `lib/theme/tokens.ts`, pinned in `tokens.test-assert.ts`, named in `AppText`, with a `systemFallback` entry in `lib/theme/fonts.ts` (the file `Domine_400Regular` is already loaded by Phase 2):

| Role | Font | Size and line | Used for |
|---|---|---|---|
| `quote` | `Domine_400Regular`, italic requested | 15 / 20 | the author's note on a diary row |

The picture's note line is a small italic serif. No existing role is that size. Domine ships no italic file, so a phone may draw the line upright or slanted; the curly quotation marks (U+201C and U+201D) and the `muted` colour carry the difference either way. No other role, colour or space step is added.

**Reused as they are:**

| Need | Token |
|---|---|
| Page, bar, photo panel fallback | `dominant`, `secondary` |
| Text, selected chip fill | `ink` |
| Labels, dates, stars figure, the note line, helper lines | `muted` (5.82:1 on the page) |
| Hairlines, vertical rules between numbers | `divider` |
| Unselected chip fill | `fieldSoft` (no border) |
| Outlined chip border | `inkRest` (taste chips), `ink` (the Edit button) |
| Errors | `destructive` |
| Row photo, thumbnail radius | `radius.md` (8dp) |

Heading `title` (Domine Regular 32/38); row title and recent-activity title `note` (Domine Regular 20/28, chosen because the picture's titles are light, not semi-bold); name on the Profile page Heading (22/28); the three big numbers `title`; everything else Label or Body.

**Round shapes (a change to the allow-list).** The pictures draw the category chips, the taste chips and the Edit button as pills. They are tappable, so this is an exception to the old rule "nothing tappable is round, nothing is a pill" (Phase 2 section 8). It is made ONCE: one shared component, `components/ui/Chip.tsx`, is the only new file that may use `radius.full` (a named exception, like the Spin button). The allow-list for `radius.full` becomes exactly: `components/ui/IconBadge.tsx`, `components/brand/DialAvatar.tsx`, `components/nav/SpinTabButton.tsx`, `components/ui/Chip.tsx`, `lib/theme/tokens.test-assert.ts`. The `radius.lg` list (five files) does not change. The photo circle on the Profile page is the existing Phase 1 circle component, not a new use. Phase 2.1's gear and bio components add no round shape and no token: they are mounted as they are.

## 3. The Diary tab

### 3.1 Layout

`Screen` with `scroll` off and `edges` top, left and right (the bar adds the bottom inset), side margins `lg`. A fixed header, then a list that scrolls under it (as the picture: the heading and the chips stay).

Fixed header, top to bottom:
1. `lg` below the top safe area: the heading 'Diary' in the `title` role, `ink`, a header element for VoiceOver.
2. `md` below: the chip row (3.4), one line that scrolls sideways and runs to the screen edge on the right (the last chip is allowed to be cut off, as in the picture).
3. `md` below: a 1dp `divider` rule across the full screen width.

The list fills the rest, full width, rows inset by the `lg` margin (3.2).

### 3.2 The row

One button, the whole row is the tap target, at least 88dp high (the picture's pitch is about 110dp: a 76dp photo plus `md` above and below). Left to right:

1. **Photo**, 76dp square, `radius.md`, centre-cropped from `photo_url` (`expo-image`, `contentFit="cover"`, no fade: lists do not use `ImageReveal`). A photo that cannot load shows a `secondary` panel with the `image-outline` glyph.
2. `md` gap, then the text column, top to bottom:
   - **Label line**, Label role, `muted`, mixed case (not capitals): the date as month and day, a middle dot (U+00B7), the category name as written that day: 'Oct 8 \u00B7 Meal'. A late entry adds a second dot and the word: 'Oct 8 \u00B7 Meal \u00B7 Late'. One line, the category name cut with an ellipsis first.
   - **Title**, `note` role, `ink`, up to two lines then an ellipsis.
   - **Stars**, `xs` below: the read-only `StarDisplay` (Phase 3) at 16dp glyphs, then `xs` and the figure ('4.5', and '5.0' written as in the picture: always one decimal here) in Label role, `muted`.
   - **The author's note**, `xs` below, only when the entry has one: `quote` role, `muted`, wrapped in curly quotation marks, one line, cut with an ellipsis.

The author's reaction is NOT on the row (the picture has none); it is on the entry page. A pressed row shows a `secondary` fill behind it, no scale. A 1dp `divider` rule sits under every row, inset by the margin on the left and right.

The row is one spoken element (section 8).

### 3.3 The skipped row

Shown only under All. Not a button, no photo, no press state. One line in Label role, `muted`, at least 56dp high with `md` padding, then the same 1dp rule: the date, a middle dot, the word 'Skipped' ('Oct 6 \u00B7 Skipped'). The category of the skipped spin is in the spoken label only. A missed day shows nothing at all.

### 3.4 The chips

A horizontally scrolling row (no scroll indicator), `lg` padding at the left, 8dp between chips, built from `Chip` (section 2): visual height 32dp, a 44dp hit area (`hitSlop`), Body role text, 14dp side padding.
- First chip 'All', then one chip per category that has at least one entry, in the order the server sends (the person's wheel order, then removed categories), each showing the category name as the person wrote it (the picture says 'Movies', 'Books'; the app shows 'Movie', 'Book', because names are the person's and are never made plural).
- One chosen at a time. Chosen: `ink` fill, `dominant` text. Not chosen: `fieldSoft` fill, `ink` text. Nothing animates.
- With no entries at all the chip row is not drawn (only 'All' would be left).
- The choice is kept while the app is open (a small store keyed by the user id, as Today's store is) and is All again after a relaunch. Tapping a taste on the Profile page sets it (4.3).

### 3.5 States

| State | When | What is shown (under the heading and chips) |
|---|---|---|
| Loading | first fetch | the Label 'Loading your diary.' (`muted`, centred, `xl` below the rule). The chip row is not drawn yet. |
| Offline on load | first fetch fails | Phase 2's `StatusMessage`: 'Couldn't connect. Check your connection and try again.' and 'Try again'. |
| Empty | loaded, no rows, All | the Label 'Nothing here yet. Your first post will show up here.' (`muted`, centred) and under it the `TextButton` 'Go to Today' (tone `ink`, underlined, 44dp), which goes to Today. No chips. No picture. |
| Empty for a category | loaded, no rows, a category chosen | the Label 'Nothing in {category} yet.' (centred). The chips stay so All can be tapped. |
| List | rows | the list. |
| Loading more | scrolling to the end, a next page exists | a Label 'Loading more.' (`muted`, centred) as the list footer. |
| Offline on more | a page after the first fails | the same footer shows the offline sentence and a 'Try again' text button; the rows above stay. |
| Refreshing | pulled down | the platform's pull-to-refresh control; the rows stay. |

A failed refresh or a failed reload on focus keeps what is on screen and shows nothing new (the next focus or pull tries again). A failed first load with nothing on screen is Offline on load.

### 3.6 Paging, refresh and focus

20 rows a page, newest first, loaded as the person scrolls (the next page is requested when the last two rows come into view, once at a time, with the server's `next_cursor`). Pull down reloads the first page. The tab reloads the first page quietly each time it comes into focus (so a post made on Today shows at once) and when the person changes the chip; rows already shown are not cleared while it loads. A chosen category that no longer has an entry (its last entry was deleted) shows the Empty for a category state. All dates and names come from the server: the phone does no date maths.

### 3.7 Opening an entry

A tap on a row pushes `app/(app)/diary/entry` with the entry id and `from` = 'diary' (Phase 3's route parameter). The page is Phase 3's `EntryView`: no bottom bar (its pinned Edit and Delete footer replaces it), the Back row names where the person came from, here 'Diary'. Edit opens `app/(app)/diary/edit-entry` and Delete opens Phase 3's sheet; both come back to the list, which reloads when it is focused again, so an edit shows at once and a deleted entry's row is gone. Delete keeps the day counted as posted (Phase 3 D-03), so no row replaces it. An entry that is gone (another phone deleted it) shows Phase 3's 'This entry isn't here any more.' with a button that reads 'Back to Diary' when the page was opened from the Diary (the Phase 3 text is 'Back to Today').

## 4. The Profile page

Route `app/(app)/profile/index.tsx` renders `ProfileScreen` for a finished account that is not in first run. The header comes from the signed-in user already in the session store (no request), and Phase 2.1's bio in place reads the same session user; the numbers come from `GET /v1/me/summary`.

`Screen` with `scroll` on and `edges` top, left and right; side margins `lg`; the page scrolls, the bar does not.

### 4.1 Header

One row, `lg` below the top safe area: the person's photo in the existing Phase 1 circle (the same component the Make it yours page uses; 72dp; with no photo, the stand-in camera circle as there), then `md`, then a column with the name (Heading role, `ink`, up to two lines, then an ellipsis) and, `xs` below, the username as '@name' in Body role, `muted` (one line, ellipsis). At the right end, a group of two controls, `xs` apart, both vertically centred on the photo: the Edit chip (a `Chip`, tone `strong`: a 1dp `ink` border, `dominant` fill, `ink` text, Body role, the word 'Edit', visual height 36dp, hit area 44dp) and, at the far right, Phase 2.1's `SettingsGear` (Ionicons `settings-outline`, 24dp, `ink`, in its own 44dp square hit area; it takes no props and opens Settings). Directly under that row, `sm` below, Phase 2.1's `BioInline` with its label off: the bio as Body text, `ink`, left aligned with the photo and the full width of the margins (so a 160 character bio has room), or the text link 'Add a bio' in `muted` when there is none; a tap turns it into a box in place with 'Save' and 'Cancel' (its look and strings are Phase 2.1's UI spec section 7). The photo, the name, the @username and the Edit chip are picture 12. The gear and the bio are not drawn in it: the developer asked for a settings section and for the bio to be editable on this page in words, so both are listed as differences in section 10. A plain row: there is no wordmark, bell or back control on this page.

### 4.2 The three numbers

`lg` below the header: a 1dp `ink` rule across the margins; then three equal columns, each a number in the `title` role (`ink`, centred) with `xs` below it a Label in `muted`, sentence case, letter spacing 1: 'Spins', 'Week streak', 'Friends'. 1dp `divider` vertical rules, 40dp tall, sit between the columns. A 1dp `divider` rule closes the block, `md` below.
- **Spins** is the number of days the person has spun. **Week streak** is the number of weeks in a row, ending with this week or last, in which the weekly goal was met. **Friends** is the number of accepted friends: always 0 in Phase 4 (Phase 5 fills it and makes it open the Friends screen). All three are read from the server and printed as sent. None is tappable in Phase 4 (no dead controls).
- With nothing loaded yet the three numbers are not drawn (4.5).

### 4.3 Your tastes

Shown only when the person has at least one entry. `lg` below the block: the Label 'YOUR TASTES' a middle dot (U+00B7) 'AVG RATING' (capitals by style, letter spacing 1.5, `muted`). `md` below: the taste chips in a grid of three columns, 8dp gaps, each a `Chip` tone `outline` (1dp `inkRest` border, `dominant` fill), visual height 40dp (the picture's chips are taller than the Diary's), the category name at the left (Body role, cut with an ellipsis first) and the average at the right (Body role, always one decimal, '4.6', '4.0'). Order: the person's wheel order, then removed categories. Each chip is a button: it sets the Diary's chosen category to that one and opens the Diary tab (3.4). Spoken: 'Movie, average 4.6 out of 5, 9 posts. Opens your diary on Movie.'

### 4.4 Recent activity

Shown only when the person has at least one entry. `lg` below: a row with the Label 'RECENT ACTIVITY' (capitals by style, letter spacing 1.5, `muted`) at the left and the text link 'See diary' at the right (Label size, `ink`, underlined, 44dp hit area; it sets the Diary's choice to All and opens the Diary tab). Under it, up to three rows (the newest three posted entries), each one button, at least 72dp high with `md` padding and a 1dp `divider` rule under it: a 48dp photo square (`radius.md`, same fallback as 3.2), `md`, then a column with the title (`note` role, `ink`, one line, ellipsis) and, `xs` below, a Label line in capitals by style, letter spacing 1.2, `muted`: the category as written, a middle dot, the date as month and day ('MEAL \u00B7 OCT 8'). A tap pushes `app/(app)/profile/entry` with `from` = 'profile' (Back label 'Profile'). `xl` of space at the end.

### 4.5 States

| State | When | What is shown |
|---|---|---|
| Loading | first summary fetch | the header, then under it the Label 'Loading your profile.' (`muted`, `lg` below). No numbers yet. |
| Offline | first fetch fails | the header, then `StatusMessage` with 'Try again'. |
| No entries | loaded, no entries | the header, the three numbers, then the Label 'Nothing here yet. Your first post will show up here.' (`muted`, `lg` below). No tastes, no recent activity, no 'See diary'. |
| Full | entries exist | everything of 4.1 to 4.4. |
| Saved | the person came back from the Edit page after a save | the notice line, from Phase 2.1's notice store (`useSettingsNotice()`): the Label 'Saved.' (`muted`, `sm` below the bio, polite live region) for 3 seconds, then gone. |

The page asks the server again when it comes into focus (so a post, a delete or a rename shows). A failed reload with data on screen keeps the data. The header, the gear and the bio are drawn in every state (they read the session, not the summary).

### 4.6 The Edit page

The Edit chip pushes `app/(app)/profile/edit` inside the Profile stack: Phase 1's form page in edit mode, with its Back control, its fields (photo, name, username, bio), its username check, its Log out and the Log out sheet, all unchanged, and the bottom bar showing. It is reached from the Profile page's Edit chip only (Settings has no 'Edit profile' and no 'Username' row: the username and the bio live on the Profile page and on this page, Phase 2.1 decision D-16). Under the photo the link reads 'Edit your picture' (it opens the same camera or library choices), and a free username is confirmed with the green 'Looks good.'. One change: the bottom button always reads 'Save changes' and is grey (the existing disabled look) until a field or the photo has changed. Phase 1's 'Continue' (shown while nothing had changed) is not on this page. No gear is drawn on it. After a successful save the page goes back and puts 'Saved.' into Phase 2.1's notice store, which the Profile page shows for 3 seconds (4.5). The bottom bar stays on this page, as on Edit wheel. The Bio box on this page and the bio in place on the Profile page edit the same field. **Log out:** this page keeps Phase 1's Log out button and sheet at its bottom, and Phase 2.1's Settings has its own Log out row and sheet with the same words; the Profile page itself has none.

### 4.7 First run

Unchanged. Right after a sign-up the Profile route still shows Phase 1's photo-and-bio page once (no bar, Continue or Skip for now end on Today, the flag in memory only). Only after that does the Profile route show 4.1 to 4.5.

## 5. The bottom bar

Four items, in this order: Home, Diary, Spin, Profile. All of Phase 2 section 8 holds (fill, border, height `56 + bottom inset`, 24dp icons in a 44dp hit area, Label size labels, idle `muted`, active `ink`, colour only, no bounce). The Diary item is Ionicons `document-text-outline` with the label 'Diary'. The Spin button keeps its circle and its two looks; it now sits third of four, a little right of centre, until Phase 5 adds Board and puts it in the middle of five. The pictures draw five slots with 'Roll' on the middle button and a light grey filled circle; neither is used (spin wording; the looks are Phase 2's). The bar is hidden on the first-run page and on the entry page (Phase 3), and shown on the Edit profile page and on Edit entry.

## 6. Motion

Tab switches, the bar's active state and pushed pages follow `.planning/phases/02.2-navigation-motion/02.2-CONTEXT.md` (the screens cross-fade in 200ms, the bar's colours glide over 220ms, pages use the platform push); nothing in this section overrides them.

Nothing new. Chips change at once; rows have no entrance animation; list photos have no fade; pages push with the stack's default. The entry page's own reveal is Phase 3's. Reduce Motion needs no special case in this phase (nothing here animates).

## 7. Copy contract

Every string the person sees. 'Spin' wording, plain words, no em dashes, no legal text. N marks a new string; E marks an existing app, Phase 2 or Phase 3 string reused.

| Where | String |
|---|---|
| Tab label, Diary heading | Diary (N) |
| Chips | All (N) / the person's category names |
| Row label | {Mon D} \u00B7 {category} (N: for example Oct 8 \u00B7 Meal) / \u00B7 Late (N) |
| Skipped row | {Mon D} \u00B7 Skipped (N) |
| Diary states | Loading your diary. (N) / Nothing here yet. Your first post will show up here. (N) / Go to Today (N) / Nothing in {category} yet. (N) / Loading more. (N) |
| Offline | Couldn't connect. Check your connection and try again. (E) / Try again (E) |
| Profile numbers | Spins (N) / Week streak (N) / Friends (N) |
| Profile sections | YOUR TASTES \u00B7 AVG RATING (N, capitals by style) / RECENT ACTIVITY (N, capitals by style) / See diary (N) |
| Profile states | Loading your profile. (N) / Nothing here yet. Your first post will show up here. (E, as the Diary) / Saved. (E, Phase 2) |
| Profile button | Edit (E, Phase 3 entry page word) |
| Edit page | Save changes (E, Phase 1) / Edit your picture (N, the photo link; Phase 1's 'Change photo' and 'Add photo' stay on the first-run page) / Looks good. (E, the green note under a free username) |
| Entry page, Back label and gone state | Diary / Profile (N, the name of where the person came from; Today is Phase 3's) / Back to Diary / Back to Profile (N) |
| Gear and bio (printed by Phase 2.1's components; this phase adds none) | Settings (spoken label) / Opens your settings. (spoken hint) / Add a bio / Edit bio / Save / Cancel / Bio / Optional / Tell people what you're spinning for. / Bio must be 160 characters or fewer. (all E, Phase 2.1) |
| Generic error | Something went wrong. Please try again. (E) |

Server-written strings are printed as sent: the category names, the titles, the notes, the dates as 'YYYY-MM-DD' that the app turns into 'Oct 8' with Phase 2's fixed month table (`monthDay` in `lib/wheel/format.ts`, no `Date` object).

## 8. Accessibility

- **Diary heading** is a header element. The tab label is 'Diary'.
- **Chips.** One button each, label '{name}', state selected or not; the first is 'All'. The row's container is labelled 'Filter by category'.
- **A row** is one button: '{title}. {n} out of 5 stars. {category}. {date}.' then 'Posted late.' when late, then 'Note: {text}.' when there is one; hint 'Opens the entry.' Dates are spoken from the server's date as 'October 8' by the same fixed table (the weekday is not needed).
- **A skipped row** is one static element: 'Skipped. {date}. {category}.'
- **Empty and loading lines** are plain text; 'Nothing here yet...' is a polite live region when it replaces a list.
- **Profile numbers.** One element per column: '{n} spins' ('1 spin'), 'Week streak, {n}', '{n} friends' ('1 friend'). The whole block is not a button.
- **Tastes.** A button: '{name}, average {x.x} out of 5, {count} posts. Opens your diary on {name}.' ('1 post').
- **Recent activity.** A row is a button: '{title}. {category}. {date}.' hint 'Opens the entry.'; 'See diary' is a button with hint 'Opens your diary.'
- **Edit.** A button, label 'Edit profile'.
- **Settings gear.** Phase 2.1's component: a button labelled 'Settings', hint 'Opens your settings.', 44dp hit area. It is read after 'Edit profile'.
- **Bio in place.** Phase 2.1's component: reading is one button ('Bio: {text}. Edit your bio.', or 'Add a bio'); editing has the labelled field with its count ('12 of 160 characters') and 'Save' and 'Cancel' buttons. It is read after the gear: the header row first, then the bio, then the numbers.
- **'Saved.'** is a polite live region.
- **Targets.** Every tappable is at least 44dp (chips 32dp visual with a 44dp hit area, rows 88dp, recent rows 72dp, Edit and the gear 44dp hit areas).
- **Text size.** Everything scales with the phone's text size except the three big numbers' size (they keep 32 and only shrink to fit one line) and the star glyphs.
- **Contrast.** `muted` on `dominant` 5.82:1, `ink` 17.32:1, `dominant` on `ink` (the chosen chip) 17.32:1, `destructive` as in Phase 1.

## 9. Notes for the plan

- **Plan split.** 04-01 is the server (the two read endpoints). 04-02 is the app: pure logic and API calls, components, routes and the bar, then the gates and a short walkthrough.
- **Wiring.** The Diary reads `GET /v1/diary` (04-01). The Profile page reads the session user for the header and `GET /v1/me/summary` for the rest. The entry pages are Phase 3's `EntryView` and edit screen, mounted by thin routes in the Diary and Profile stacks; Phase 3's `from` value ('today' now) gains 'diary' and 'profile' for the Back label, the gone-state button and the Edit route.
- **A small refactor of Phase 3 files** (no behaviour change, proved by Phase 3's own steps): the body of the edit-entry route becomes a component, `components/entry/EditEntryScreen.tsx`, so three thin routes can mount it; the bar-hiding `useFocusEffect` of the entry route becomes a hook, `components/nav/useHideTabBar.ts`; `EntryView` and the edit screen choose their Edit route and the gone-state label from `from`.
- **One change to a Phase 1 file:** `components/profile/ProfileForm.tsx` gets one optional prop (`pushed`) for 4.6 and one call that puts 'Saved.' into the notice store. Phase 2.1 edits the same file once (its gear line); the two edits do not overlap.
- **Phase 2.1's pieces.** `SettingsGear`, `BioInline`, `lib/settings/notice.ts` and the Settings, Username and Password pages are built by plans 02.1-01 and 02.1-02 before this phase. This phase mounts the gear and the bio and reads the notice store; it edits none of them, and plan 04-02 depends on both Phase 2.1 plans being built.
- **Launch files.** `app/(app)/_layout.tsx` (Tabs) and `app/(app)/profile/index.tsx` change here, so one cold launch is recorded after they change, including the 'just signed up' case. `app/index.tsx`, `app/_layout.tsx`, `lib/session/store.ts` and `lib/api/client.ts` are not edited. `app/(app)/profile/_layout.tsx` also registers `entry` and `edit-entry` and keeps Phase 2.1's three lines.
- **Pure logic in pure files** (`lib/diary/rows.ts`, `filter.ts`, `stats.ts` and `monthDay` in `lib/wheel/format.ts`), checked by a throwaway node script and by device steps.
- **Never run `expo lint`.** No new native module. Tokens only, no hex. The emoji scan still allows exactly one file, `lib/entry/reactions.ts` (the entry page shows the author's reaction; the diary row does not).

## 10. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plans). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 7.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Diary | Diary / All / Loading your diary. | New. The chips show your own category names. |
| 2 | Diary row | Oct 8 \u00B7 Meal / \u00B7 Late / Oct 6 \u00B7 Skipped | New. 'Late' appears only on late posts. |
| 3 | Diary empty | Nothing here yet. Your first post will show up here. / Go to Today / Nothing in {category} yet. | New. No picture. |
| 4 | Profile | Spins / Week streak / Friends | From your picture, 'Rolls' became 'Spins'. |
| 5 | Profile sections | YOUR TASTES \u00B7 AVG RATING / RECENT ACTIVITY / See diary / Edit | From your picture. |
| 6 | Profile states | Loading your profile. / Saved. | New / Phase 2's. |
| 7 | Entry page | Diary / Profile (Back labels) / Back to Diary / Back to Profile | New. |
| 8 | Gear and bio | Settings / Add a bio / Edit bio / Save / Cancel | Phase 2.1's words, approved with Settings. You asked for them in words; picture 12 draws neither. |

**Choices made here that you may want to change:**
- **The Diary stays its own tab**, as your pictures draw it (the bar of five in the end). My earlier suggestion to fold it into Profile was not taken. Say "drop" and the Diary page becomes a page opened from 'See diary'.
- **Your chips are pills, and so is the Edit button.** The older rule was no pills and nothing tappable round. I followed your pictures with one shared chip component, so exactly one new file uses the round radius (`components/ui/Chip.tsx`).
- **'Roll' and 'Rolls' became 'Spin' and 'Spins'**, and the bar says 'Spin' (your Today picture, 1a, says Spin too).
- **Chips show your category names as you wrote them** ('Movie', not 'Movies').
- **The author's note is on the row** (your picture has it; my older default did not). The reaction is not on the row; it is on the entry page.
- **No month headings** in the list (your picture has none).
- **One new text role, `quote`** (a small serif for the note line). The row title uses the existing light serif (`note`), because your titles are light. The note may draw upright on some phones because the serif has no italic file.
- **A skipped day is a quiet text row** (date and the word 'Skipped'), only under All.
- **The Friends number is plain text** and shows 0 until Phase 5, so it is not a dead button.
- **The Edit page has no Continue**: only 'Save changes', grey until you change something; after saving you are back where you came from (the Profile page, or Settings) with 'Saved.' for three seconds.
- **The Profile page has no Log out.** Log out is at the bottom of the Edit page, where Phase 1 put it, and Settings (Phase 2.1) has its own Log out row with the same words.
- **The Spin button sits third of four** (a little right of centre) until Board arrives in Phase 5. Say if a centred Spin matters now.
- **Tapping a taste opens the Diary on that category** (added: your picture draws them as chips that look tappable). 'See diary' opens it on All.
- **A recent-activity row opens the entry page inside Profile**, so Back says 'Profile'.
- **The Today week section stays display only**: your picture has no arrow on it.
- **No photo grid and no stats header on the Diary** (your picture has neither; the numbers are on the Profile page).
- **A brand-new account sees a plain line** ('Nothing here yet. Your first post will show up here.') under the three numbers instead of an empty page.
- **The Edit button stays exactly as in your picture** and says only 'Edit'. It is the one way to the Edit page: Settings (Phase 2.1) has no 'Edit profile' row any more (your words of 2026-10-13: the bio and username are on the profile page, not in the settings).
- **A Settings gear sits at the far right of the photo row, right of the Edit button** (both centred on the photo). Your picture 12 draws no gear: you asked for a settings section in words (Phase 2.1). It is Phase 2.1's component, mounted as it is, and it opens Settings (Notifications, Blocked accounts, Private account, Password, Log out).
- **Your bio shows under your name and is edited in place** with Save and Cancel. Your picture 12 draws no bio: you asked in words that the bio can be edited on the profile page (Phase 2.1). It runs the full width under the photo row, because beside the photo and two controls it would wrap into many short lines. The Bio box on the Edit page and this bio are the same field.
- **'Saved.' comes through Phase 2.1's small notice store**, so it shows on whichever page the Edit page returns to.
