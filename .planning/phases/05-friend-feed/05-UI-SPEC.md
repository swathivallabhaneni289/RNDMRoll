---
phase: 5
slug: friend-feed
status: draft
created: 2026-10-10
revision: 1
approval_word: go
---

# Phase 5 - UI Design Contract (Friend Feed: friends, the Board, reactions, comments, notifications)

> Design contract the Phase 5 app plans (05-04, 05-05, 05-06) build from. PHASE 5. NOTHING IS BUILT. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later), the bar, the bell and the Notifications stub of `02-daily-roll/02-UI-SPEC.md` (revision 6 or later), the entry page, the stars and the reaction file of `03-daily-entry/03-UI-SPEC.md` (revision 2 or later) and the Profile page, the Diary and the shared `Chip` of `04-personal-diary/04-UI-SPEC.md` (revision 1 or later). Phases 2, 3 and 4 are not built either. The developer's pictures of 2026-10-10 are the layout source: screen 08 (Board locked), screen 09 (Board unlocked), screen 10 (a post with reactions and comments), screen 11 (Notifications) and, for the Profile and the bar, screens 07 and 12 (`docs/reference/screens-07-diary-08-board-locked-2026-10-10.png`, `docs/reference/screens-09-board-unlocked-10-post-2026-10-10.png`, `docs/reference/screens-11-notifications-12-profile-2026-10-10.png`, `docs/reference/screens-03a-home-03b-diary-2026-10-10.png`). Their note about the bell is in `05-CONTEXT.md` decision D-06. Where a picture and an older paper rule disagree the picture wins and the override is listed at the end of section 16 (the developer's rule of 2026-10-10). The frosted-glass look of 2026-10-01 is dropped (05-CONTEXT D-03): nothing here blurs or tints. All imagery is the developer's: photos are the person's own, avatars are initials, and every placeholder is plain.

## Visual reference (binding)

The developer's pictures named above are the visual reference for the build ("use exactly the same visuals ... as a reference to build this project", 2026-10-10). The plans build to match them, and at each screen's device check the phone screenshot is compared side by side with its picture. Every difference that is not in the "Choices made here that you may want to change" list at the end of this document is a defect. Where this document and a picture disagree, the picture wins and this document is corrected.

## 1. What this phase puts on screen

| Screen or state | Where | New or changed |
|---|---|---|
| Board tab | `app/(app)/board/index.tsx`, component `components/board/BoardScreen.tsx` | New. Header with overline, title and bell; the locked card, the skipped block, the no-friends block, the grid by day. No Friends and Global switch yet (Phase 6). |
| Post page (a friend's post) | `app/(app)/post.tsx` (a hidden route), component `components/board/PostView.tsx` | New. Photo, title, stars, their reaction, their note, the reaction row, comments, the comment bar. No bottom bar. |
| Own post (your entry page) | Phase 3's `EntryView`, picture 06 (opened from Today, the Diary, Profile, the Board or a notification) | Changed by ADDITION only: below the author's note and above the footer, the received-reactions line, the '{n} comments' list with its delete buttons and the add-a-comment bar. The square edge-to-edge photo, the Back label and the fixed Edit and Delete footer stay as Phase 3 built them. |
| Friends screen | `app/(app)/profile/friends.tsx` (plan 05-04) | New. Find by username, requests, sent, friends, blocked, share an invite. |
| Profile | Phase 4's `ProfileScreen` | Changed: only the Friends column of the three-numbers row becomes a button that opens the Friends screen. The header (photo, name, @username with Phase 2.1's `BioInline` under it, the Edit chip and Phase 2.1's `SettingsGear` at the top right) is Phase 4's and Phase 2.1's and is not touched. |
| Bell | Today header (Phase 2) and Board header | Changed: a black dot while anything is unread; it opens Notifications above the current tab. |
| Notifications page | `app/(app)/notifications.tsx` (a hidden route), component `components/today/NotificationsScreen.tsx` | Changed: no longer a stub. Phase 2's route `app/(app)/today/notifications.tsx` is removed. No bottom bar. |
| Bottom bar | `app/(app)/_layout.tsx` | Changed: Board added. Home, Diary, Spin, Board, Profile (five), Spin in the middle. Hidden on the post page and on Notifications. |

Under Tabs a file placed directly in `app/(app)/` becomes a tab. `post.tsx` and `notifications.tsx` are registered in the layout as hidden routes (`href: null`, bar not drawn) so the bar still has exactly five items. The Board is a folder with its own Stack so later screens can push inside it.

## 2. Tokens added (the only new design values)

**One text role**, added to the `type` group of `lib/theme/tokens.ts`, pinned in `tokens.test-assert.ts`, named in `AppText`, with a `systemFallback` entry in `lib/theme/fonts.ts` (the file `Domine_400Regular` is already loaded by Phase 2):

| Role | Font | Size and line | Used for |
|---|---|---|---|
| `cardTitle` | `Domine_400Regular` | 16 / 22 | the title under a Board card, the author's note on the post page |

The pictures draw card titles and the note in a small light serif. No existing role is that size (`note` is 20/28, `quote` is 15/20 with italic requested, `heading` is 22/28 semi-bold). If the built `tokens.ts` already has an upright serif at 16/22 or 15/20, use it and add nothing.

No new colour, space, radius or elevation. Reused as they are:

| Need | Token |
|---|---|
| Page, bar, comment bar fill | `dominant` |
| Photo placeholder, locked tiles | `secondary` |
| Text, stars, the dot, the ring around the chosen reaction | `ink` |
| Labels, times, counts, helper lines | `muted` (5.82:1) |
| Hairlines between rows and comments | `divider`; the one darker rule under the reaction row is `ink` (as the picture) |
| Name chip fill | `dominant` |
| Comment field border | `inkRest` |
| Locked card | `elevation.card` with a 1dp `ink` border and `radius.md` |
| Errors, Delete | `destructive` |
| Buttons | `PrimaryButton`; row actions on the Friends screen are Phase 4's `Chip` |

**Round shapes (a change to the allow-list, made once).** The pictures draw the name chip over each photo as a pill, the initials as circles, the chosen reaction as a ring and the comment field as a pill. The token comment on `radius.full` says it must never be applied to a chip or an input; the developer's pictures override that, so the comment is rewritten to name these uses. Exactly four new files may use `radius.full`, each a drawn shape with one job:

| File | Shape |
|---|---|
| `components/people/Avatar.tsx` (created by plan 05-04 with the person kind; plan 05-05 adds the other two kinds to the same file) | the initial circle (22, 28 and 32dp), a person's photo cropped to a circle, the wheel-mark circle and the solid dot circle used on Notifications |
| `components/board/NameChip.tsx` | the pill over a card photo (not a control: the card is the button) |
| `components/board/ReactionRow.tsx` | the 1.5dp ring around the chosen reaction |
| `components/board/CommentBar.tsx` | the pill text field |

The allow-list for `radius.full` becomes: Phase 4's list (`components/ui/IconBadge.tsx`, `components/brand/DialAvatar.tsx`, `components/nav/SpinTabButton.tsx`, `components/ui/Chip.tsx`, `lib/theme/tokens.test-assert.ts`) plus these four. The `radius.lg` list (five files) does not change. The locked card and every button keep `radius.md`. Card photos have square corners, as in the picture.

## 3. The Board

### 3.1 Layout and header

`Screen` with `scroll` off and `edges` top, left and right (the bar adds the bottom inset); side margins `lg`. A fixed header, then the content under it.

Header, top to bottom:
1. `lg` below the top safe area, one row. At the left a column: the overline, a Label in capitals by style, letter spacing 1.5, `muted`: 'Today' or 'Last night' (the server's `day_label`: `today` or `last_night`); under it, `xs` below, the heading 'Board' in the `title` role (32/38), `ink`, a header element for VoiceOver. At the right, vertically centred on the heading, the bell (9.1).
2. `lg` below: the content of the current state (3.2). There is no Friends and Global switch in this phase: the picture's switch belongs to Phase 6, and a control with nothing behind it breaks the no-dead-controls rule.

### 3.2 States

The phone shows what the server says (`Board.state`). It never works out the day in play, the gate, the count or the late window.

| State | Server `state` | What is shown under the header |
|---|---|---|
| Loading | first fetch | the Label 'Loading the Board.' (`muted`, centred, `xl` below the header). |
| Offline on load | first fetch fails | Phase 2's `StatusMessage`: 'Couldn't connect. Check your connection and try again.' and 'Try again'. |
| No friends | `no_friends` | the no-friends block (3.8). |
| Locked | `locked` | up to four grey tiles with name chips and the locked card (3.6), then the earlier days the person posted (3.5). |
| Skipped | `skipped` | the skipped block (3.7), then the earlier days the person posted (3.5). |
| Unlocked | `unlocked` | the grid by day (3.5). |

A failed refresh or a failed reload on focus keeps what is on screen and shows, above the content, one Label in `destructive` ('Couldn't connect. Check your connection and try again.') with a 'Try again' text button; it goes when a reload succeeds.

### 3.3 The card

One button, the whole card is the tap target, two to a row. Column width is `(screen width - 2 x lg - md) / 2`; the gap between columns is `md` (16) and between rows `lg` (24).

Top to bottom:
1. **Photo.** `aspectRatio` 7 to 8 (width to height), `secondary` fill, `expo-image` `contentFit="cover"`, square corners, no fade (lists do not use `ImageReveal`). A photo that cannot load shows the `image-outline` glyph centred (`muted`).
   - **Name chip** (3.4) at the top left, inset `sm`.
   - **Category word** at the bottom left, inset `sm`: the category as written on that day, Label role in capitals by style, letter spacing 1.5, `ink`, on a small plate (`dominant` fill, `radius.sm`, `xs` and `sm` padding) so it reads on any photo. (The picture draws it as bare grey text on a placeholder.)
2. **Title**, `sm` below the photo: the `cardTitle` role, `ink`, one line, cut with an ellipsis.
3. **Stars**, `xs` below: the read-only `StarDisplay` (Phase 3) at 12dp glyphs, `xs`, then the figure in Label role, `muted`, always one decimal ('5.0', '4.5').

A late post adds the word 'Late' after the figure (Label role, `muted`), as the feed badge of 05-CONTEXT D-10. The author's reaction is not on the card (the picture has none); it is on the post page. A pressed card shows no scale change beyond the existing `PressScale` (0.97 over 90ms).

The card opens the post page (section 4) for a friend's post, and Phase 3's entry page with comments for your own (section 5), both through the hidden `post` route with `from` = 'board'.

### 3.4 The name chip

A drawn label, not a control. A pill (`radius.full`), `dominant` fill, 28dp high, `xs` padding at the left and `sm` at the right: an `Avatar` (22dp) then `xs` then the first name in Label role, `ink`, one line, cut with an ellipsis. For your own post the name is 'You'. The first name is the first word of the person's display name (a pure function; if the name is empty the username is used). The same chip shape, without a photo behind it, is used on the locked tiles (3.6).

### 3.5 The grid by day (unlocked, and the earlier days under a lock)

The state `unlocked` carries `days`, and so do `locked` and `skipped`: under a lock `days` holds only the EARLIER days the person posted, newest first, and is empty when there are none (the gate is per day, answer 6 option B: a day you posted stays readable under today's lock or skip). Rows of two cards, grouped by day:
- In `unlocked` the first day is the day in play (`label` = `today`). It has no heading of its own: the overline in the page header says 'Today' or 'Last night'. Under a lock there is no `today` day, so every day has a heading.
- Days follow, newest first, each under a heading: `lg` above and `md` below, the Label in capitals by style, letter spacing 1.5, `muted`: 'Yesterday' (label `yesterday`) or the short date from Phase 2's `dateLine` ('TUE, OCT 6') for `earlier`.
- Inside a day, cards are in the order the server sends (newest first). Your own card is one of them, with the chip 'You'; it is not pinned. An odd card leaves the right cell empty.
- A day appears only if you posted it (05-CONTEXT D-02). Days you did not post are not listed and not hinted at.

Under a lock the days start `xl` below the locked block or the skipped block, and the whole content below the fixed page header scrolls: the locked or skipped block is the header of the list, the days are its rows. The same paging, refresh and focus rules apply in all three states.

Paging: five days at a time. When the last row comes into view and `next_before` is set, the next page is requested (once at a time) with `before`. A small `ActivityIndicator` (`ink`) sits as the list footer while it loads; if that page fails the footer shows the offline sentence and a 'Try again' text button and the days above stay. Pull down reloads from the first page. The tab reloads the first page quietly each time it comes into focus (so a post made on Today, or a reaction, shows at once); the days already shown are not cleared while it loads.

### 3.6 Locked

Shown for `state` = `locked`. The page behind the card shows up to four tiles in two columns (the number of friends who posted, at most four): each tile is the card's photo area (`secondary` fill, 7 to 8) with the name chip of a friend who posted, taken from `locked.names`, and under it two grey bars (60 percent and 35 percent of the card width, 8dp high, `divider`, `radius.sm`) where the title and stars would be. These are drawn shapes, not data. The bars and tiles never show a real title, photo, category or stars.

The locked card sits over the lower third of the first row of tiles and the top of the second row, full width of the margins, centred. `radius.md`, `elevation.card`, a 1dp `ink` border, `lg` padding. Top to bottom:
1. The Label 'Locked' (capitals by style, letter spacing 1.5, `muted`).
2. `sm` below, the heading in the `note` role (20/28), `ink`: '{n} friends posted today.' ('1 friend posted today.'; in the late window 'today' reads 'last night'; with none posted: 'No friends have posted yet.').
3. `sm` below, Body role, `muted`: 'Post your spin to see what they got and how they rated it.'
4. Late window only (`day_label` = `last_night`) and action `log`: `sm` below, the Label 'Posting it now would count as late.' (Phase 3's string, `muted`).
5. `md` below, by `locked.action`:
   - `log`: the `PrimaryButton` 'Log your {category}', full width, with the category as the person wrote it, in lower case ('Log your meal'). It makes sure Today is loaded (the same store and call Today uses) and then pushes Phase 3's entry screen (`/(app)/today/log`). If Today is not in Result or Late at that moment, it opens Home instead and nothing is posted.
   - `spin`: the `PrimaryButton` 'Spin first', which goes to Home (the person spins there, then posts).
   - `wait`: no button; the Label 'Your next spin opens at 8:00 PM.' (`muted`, with a no-break space before PM).

The card is not a modal: the bell and the bar stay usable. Under the card and the tiles, the earlier days the person posted follow as in 3.5, each under its heading; with none, nothing follows.

### 3.7 Skipped

Shown for `state` = `skipped` (05-CONTEXT D-01). No grid for today. Centred in the space under the header: the sentence in the `note` role, `ink`, 'You skipped today. Friends' posts open after your next post.' (late window: 'You skipped last night. Friends' posts open after your next post.'), and `md` below it the `TextButton` 'Go to Today' (tone `ink`, underlined, 44dp), which goes to Home. Under it the earlier days the person posted follow as in 3.5, each under its heading; with none, nothing follows.

### 3.8 No friends

Shown for `state` = `no_friends` (05-CONTEXT D-13, FEED-03). Top to bottom: Body role, `ink`, 'Add friends to see what they spun today.'; `md` below, the `PrimaryButton` 'Add friends', which opens the Friends screen (section 7); `xl` below, the Label 'YOUR DIARY' (capitals by style, letter spacing 1.5, `muted`) and under it the person's own latest ten entries as Phase 4's diary rows (the same row component, built by plan 04-02: use its built name), then the text link 'See diary' (Ink, underlined, 44dp) that opens the Diary tab. A row opens the entry page as it does in the Diary. With no entries yet the diary part shows Phase 4's line 'Nothing here yet. Your first post will show up here.' The bell stays in the header.

## 4. The post page (a friend's post)

The picture 'Board, post, reactions, comments' (screen 10). Route `app/(app)/post.tsx` (a hidden route), opened with the entry id and a `from` value. The page loads `GET /v1/posts/{entry_id}`. If the response says the post is yours (`card.mine`) the route renders Phase 3's `EntryView` (section 5); otherwise it renders `PostView`. These are two layouts, not one component with one ratio: a friend's post follows picture 10 (a photo about 3 to 2 wide, the author row on top, the reaction row, the comments, the comment bar); your own entry page stays Phase 3's page of picture 06 (a square photo, edge to edge, the fixed Edit and Delete footer). The two share small parts only: the comment list, the comment bar and, on a friend's post, the reaction row. The bottom bar is NOT drawn on this page: the comment bar is pinned in its place.

`Screen` with `scroll` off, all four edges (no bar adds the bottom inset); the header and the comment bar are pinned, the middle scrolls, inside a `KeyboardAvoidingView` so the comment bar sits above the keyboard.

Top to bottom:
1. **Back row**, 44dp, inside the `lg` margins: the chevron (Ionicons `chevron-back`, 24dp) and the name of where the person came from in Body role: 'Board', 'Notifications', 'Today', 'Diary' or 'Profile' (from `from`). It goes back to that place (history).
2. **Author row**, `sm` below: an `Avatar` (28dp, photo if the person has one, else the initial), `sm`, the first name in the `button` role, `ink`; at the right end a Label in capitals by style, letter spacing 1.5, `muted`: the category as written, a middle dot (U+00B7), and the card's `posted_local_time` as sent (the time of day in the author's own zone, for example '8:41 PM'): 'MOVIE', a middle dot, '8:41 PM'. A late post adds 'Late' after another middle dot.
3. **Photo**, `sm` below, the full width of the screen edge to edge, no margins, no radius, `aspectRatio` 3 to 2 (width to height), cover, `ImageReveal`. Label for VoiceOver: 'Photo for {title}'. If it cannot load: a `secondary` panel of the same size with the `image-outline` glyph and the Label 'Couldn't load this photo.' (`muted`).
4. From here on inside the `lg` margins. **Title row**, `md` below the photo: the title in the `heading` role (22/28), `ink`, up to two lines, flexible at the left; at the right, top aligned, the read-only `StarDisplay` at 18dp (no figure), `sm`, and the author's reaction character at 22/28 if there is one (no rule between). One spoken element: 'Five out of 5 stars. Their reaction: Sad.' (no reaction: just the stars).
5. **The note**, `xs` below, only when there is one: the `cardTitle` role, `ink`, line breaks kept.
6. **The reaction row**, `md` below (6, section 6).
7. **Comments** (4.1).
8. **Block link**, `xl` below the last comment (or the reaction row when there are none): the `TextButton` 'Block {first name}' (tone `muted`, underlined, 44dp), which opens the block sheet (4.3). Phase 6 adds 'Report this post' beside it.
9. `xl` of space at the end, clear of the comment bar.

**Pinned comment bar** (4.2) at the bottom, above the bottom safe area and the keyboard.

### 4.1 Comments

Under the reaction row's darker `ink` rule (picture). Rows, each with `md` padding above and below and a 1dp `divider` rule between rows (none after the last): an `Avatar` (28dp) at the left, `sm`, then a column: a line with the first name (`button` role, `ink`; 'You' for your own), `sm`, the comment's `at_label` as sent (Label, `muted`; the viewer's zone: a time of day today, a short weekday for the six days before, a month and day before that), and, when the row can be deleted, at the right end the `TextButton` 'Delete' (Label size, tone `muted`, underlined, 44dp hit area); then the comment text in the Body role, `ink`, line breaks shown as written. Order: oldest first. A comment you or the post's owner may delete shows 'Delete' (the server says so with `can_delete`).

**Delete a comment.** 'Delete' opens `ConfirmSheet` (Phase 2): title (Heading) 'Delete this comment?', body 'It will be gone for good.', rows 'Delete' (`destructive`) and 'Keep it'. 'Delete' sends the delete, closes the sheet and removes the row; a 'not found' answer counts as done. A lost connection closes the sheet and shows the offline sentence above the comment bar.

When there are no comments the list shows nothing (no empty line): the count label is also left out.

### 4.2 The comment bar

Pinned above the bottom safe area, `dominant` fill, a 1dp `divider` rule on top, `md` padding. A row: a text field at the left, `radius.full`, a 1dp `inkRest` border, 44dp high, `md` side padding, Body role, `ink`; the placeholder 'Add a comment' in `muted` (the picture draws it; this overrides the older rule against hint words inside boxes, listed in section 16); single line that grows to three, `maxLength` 140, `autoCapitalize` sentences; Return ends editing. At the right, `md` away, the text button 'Post' (`button` role): `muted` while the field is empty or while sending, `ink` when it can be sent. Tapping it sends once (a ref blocks a second tap), shows a small `ActivityIndicator` in place of the word while it runs, appends the new comment at the end of the list and clears the field. On a failure the text stays and one Label in `destructive` ('Couldn't connect. Check your connection and try again.' or 'Couldn't post your comment. Please try again.') shows above the bar.

If the server says the person cannot comment (`can_comment` false), the bar is not drawn and nothing takes its place. Phase 5 always sends true for a post the person may see, and so does Phase 6 (comments are open to everyone who can see a public post), so this is a safety branch, not a state the person normally meets.

### 4.3 Block

'Block {first name}' opens `ConfirmSheet`: title 'Block {first name}?', body 'You will stop seeing each other's posts. They won't be told.', rows 'Block' (`destructive`) and 'Don't block'. 'Block' sends `POST /v1/blocks`, closes the sheet and goes back to where the person came from, where the post is gone after the next reload. The sheet and its strings are shared with the Friends screen (section 7).

### 4.4 States

| State | When | What is shown |
|---|---|---|
| Loading | fetch | the Back row, then the Label 'Loading this post.' (`muted`, centred, `xl` below). No comment bar. |
| Offline | fetch fails | the Back row, then `StatusMessage` with 'Try again'. |
| Gone | the server answers `not_found` (deleted, a block, or never visible) | the Back row, the Label 'This post isn't here any more.' and a `TextButton` 'Back to Board' (the place's name) that goes back. |
| Ready | loaded | everything of section 4. |

The page reloads when it comes back into focus.

## 5. Your own post (Phase 3's entry page, with comments added)

Your own entry page stays Phase 3's page, which follows the developer's picture 06: no bottom bar, the photo square and edge to edge, the Back label naming where you came from ('Today', 'Diary', 'Profile', and now 'Board' and 'Notifications'), the category and long date line, the title in the `title` role, the stars with the figure and the author's reaction, the note in the `note` role, and the fixed `Edit | Delete` footer. It keeps its square photo: the 3 to 2 photo belongs to a friend's post (section 4). The developer's note says a reaction or a comment opens 'your entry with the comments (like screen 10, but for your post)', so Phase 5 ADDS three things to this page, in the scrolling part, below the author's note and above the footer:

1. **Reactions received**, `lg` below the note: the Label 'REACTIONS' (capitals by style, letter spacing 1.5, `muted`). Under it, `sm` below, one wrapped line of items, each the person's first name (Body role, `ink`), `xs`, and their reaction character at 22/28, with `md` between items ('Maya', 'Priya'...). With none: the Label 'No reactions yet.' (`muted`). Nothing is counted at zero. One spoken element: 'Reactions: Maya, Fire. Priya, Sad.'
2. **Comments**, `lg` below, with a 1dp `ink` rule above and the Label '{n} comments' ('1 comment') at the right end (no count and no rule when there are none): the same comment rows as 4.1. You can delete any comment on your own post, and your own comments.
3. **The add-a-comment bar**, at the end of the list and above the footer (not pinned: the pinned slot is Phase 3's Edit and Delete footer): the same pill field and 'Post' word as 4.2, in the scrolling content, so the owner may reply to their friends. The page scrolls the new comment into view.

If the extras cannot be loaded the entry page still shows everything of Phase 3 and the three blocks are simply not drawn (no error line); they load again on focus.

## 6. Reactions

A friend's post page shows one row of six. Your own post shows what you received (section 5); the author's own reaction is shown beside the stars as in Phase 3.

**The six**, stored and sent by name, in this order (the order of the picture's row). The characters live in `lib/entry/reactions.ts` and nowhere else; this table names them by code point and says in words what they are.

| Name stored and sent | Character | Code point | Spoken |
|---|---|---|---|
| love | face with heart eyes | U+1F60D | Love |
| fire | fire | U+1F525 | Fire |
| laughing | face with tears of joy | U+1F602 | Laughing |
| wow | face with open mouth | U+1F62E | Wow |
| sad | crying face | U+1F622 | Sad |
| sleepy | sleeping face | U+1F634 | Sleepy |

Phase 3's paper already uses these six, with the same names, the same order and the same characters (the other window switched it to the picture's six on 2026-10-10), so the two phases share one list and one file. The developer had tapped a different six for Phase 3 (heart, laughing, wow, fire, clap, yum); the picture won by the rule of 2026-10-10 and the developer is told once (section 16). The names are read from the shared list at build time, never retyped. One word ("six") from the developer puts the tapped list back; `05-HAND-EDITS.md` item 1b has the recipe (one file and one list in the app, one list on the server).

**The row.** Under the note (or the title row when there is no note), `md` below, between a 1dp `divider` rule above and a 1dp `ink` rule below (picture): six buttons in one row, each 44dp wide and 44dp high, character at 22/28 in a Text (`allowFontScaling` off), no fill and no border when not chosen. The chosen one has a 1.5dp `ink` ring (a circle) around it. At the right end the Label '{n} comments' ('1 comment'), `muted`; it shares the row when at least 72dp is left, else it sits `xs` below the row, right aligned; it is not drawn at 0.

**Behaviour.**
- Tap an unchosen reaction: it is chosen at once (the ring moves) and the request goes out; tap the chosen one: it clears. One reaction per person per post; choosing another replaces it.
- If the request fails the ring goes back and one Label in `destructive` shows under the row ('Couldn't connect. Check your connection and try again.' or 'Couldn't save that. Please try again.').
- A friend's row shows no counts and no names (picture). The owner sees who reacted on their own post (section 5) and in Notifications.
- The server refuses a reaction on your own post; the app never offers one there.
- Motion: `PressScale` on the buttons, nothing else; none under Reduce Motion.

**The one sanctioned colour.** The characters draw in the system's colour (yellow faces, a red flame). That is the developer's choice (Phase 3). They appear only in the reaction row, where an author's reaction is shown (the card's post page and the entry page), in the received-reactions line, and in a reaction notification (section 10). Never as an icon, bullet, button label or decoration.

## 7. The Friends screen

Built by plan 05-04. Route `app/(app)/profile/friends.tsx`, inside the Profile stack, opened from the Friends number on Profile, from 'Add friends' on the Board and from a friend-request or accepted notification. Back control: the chevron and 'Profile' in Body role. The bottom bar stays. `Screen` with `scroll` on and `edges` top, left and right, side margins `lg`.

Top to bottom:
1. The heading 'Friends' in the `title` role, `ink`, a header element.
2. **Find.** `lg` below: the small-caps label 'Find a friend by username' (the existing `soft` `TextField` label), the field (`autoCapitalize` none, `autoCorrect` off, `maxLength` 30, no text inside the empty box), and under it, `md` below, the `PrimaryButton` 'Find' (grey until the field has text; Return also finds). The result shows under it:
   - a hit: a row (below) with one action: 'Add friend' (relation none), 'Requested' (request sent, a disabled `Chip`), 'Accept' (request received), 'Friends' (plain text, already friends);
   - a miss: the Label 'No one has that username.' (`muted`). The same line for yourself and for someone who blocked you.
3. **Requests**, `xl` below, only when there are incoming requests: the Label 'REQUESTS' (capitals by style, letter spacing 1.5, `muted`) and rows with two `Chip` actions at the right, 'Accept' (tone `strong`) and 'Decline' (tone `outline`).
4. **Sent**, only when there are outgoing requests: the Label 'SENT' and rows with one `Chip` 'Cancel' (tone `outline`).
5. **Friends**, `xl` below: the Label 'FRIENDS' and one row per friend; with none: the Label 'No friends yet.' (`muted`). A row is one button: it opens an action sheet (`ConfirmSheet`) titled with the friend's name and the rows 'Remove friend', 'Block {first name}' (`destructive`) and 'Cancel'. 'Remove friend' asks again with title 'Remove {first name}?', body 'You will no longer see each other's posts.', rows 'Remove' and 'Keep'. 'Block' uses the sheet of 4.3.
6. **Blocked**, only when there are any: the Label 'BLOCKED' and rows with one `Chip` 'Unblock' (tone `outline`).
7. **Share**, `xl` below: the `TextButton` 'Share an invite' (Ink, underlined, 44dp), which opens the phone's share sheet with the text 'Add me on RNDMRoll: @{username}'. There is no link (the domain does not exist yet).

**A person row** (in every section): an `Avatar` (32dp), `md`, a column with the display name (`button` role, `ink`, one line, ellipsis) and, `xs` below, '@username' in Label role, `muted`; the action at the right end; a 1dp `divider` rule between rows, 56dp high at least.

**States.** Loading: the Label 'Loading your friends.' (`muted`). Offline: `StatusMessage` with 'Try again'. After any action the lists reload. The messages: 'You are already friends.' (`already_friends`), 'You already sent a request.' (`request_exists`), 'You can't send more requests right now. Cancel one first.' (`request_limit`, at 50 waiting requests), the offline sentence, and the generic 'Something went wrong. Please try again.', as one Label in `destructive` under the field. A decline is silent: the request just goes. The other person is never told about a decline, a removal or a block.

## 8. Profile: the Friends number

Phase 4 draws the three numbers on the Profile page with Friends as plain text. Here the Friends column becomes one button: the number and its Label together are the tap target (44dp high at least), no icon, no underline, no change of look (a number is not styled as a link in the picture), spoken 'N friends. Opens your friends.' It opens the Friends screen (section 7). The Friends number is the real count of accepted friends from `GET /v1/me/summary`. A pending request adds nothing to it (the bell carries requests). Nothing else on the Profile page changes: the header (the photo, the name, @username with Phase 2.1's `BioInline` under it, the Edit chip, and Phase 2.1's `SettingsGear` at the top right, which Phase 4 mounts) is not touched here, and the Friends number stays the third column of the three-numbers row.

## 9. The bell

### 9.1 Look

`components/notifications/BellButton.tsx`, used in Today's header (replacing Phase 2's inline bell) and in the Board header. Ionicons `notifications-outline`, 24dp, `ink`, in a 44dp hit area. When anything is unread a solid `ink` dot (8dp) sits at the top right of the glyph, offset 6dp right and 6dp up from the glyph's corner, with a 2dp `dominant` ring around it so it never touches the bell. No number, no badge, no colour, no animation. The dot is a drawn shape (a react-native-svg `Circle`, like the day dots of Phase 2), not a View with a radius, so it is not a use of `radius.full` and needs no allow-list entry. The unread dots on the Notifications rows are drawn the same way.

### 9.2 When it is on

The dot is on while the server's `unread` is true: something is newer than the person's last seen time. The app asks `GET /v1/notifications/summary` when Today or the Board comes into focus, when the app returns to the foreground, and when Today's own refetch at the server's `next_open_at` runs (so "Your spin is open." can light the bell at 8:00 PM). Opening the Notifications page marks everything seen, so the dot is off when the person comes back. The page is the only place that clears it.

### 9.3 What it opens

The Notifications page (section 10), above the tab the person was on, with its back arrow returning there.

## 10. The Notifications page

The picture 'Notifications (bell)' (screen 11). Route `app/(app)/notifications.tsx` (a hidden route), component `components/today/NotificationsScreen.tsx` (Phase 2's file, filled in). The bottom bar is NOT drawn (the picture has none); the bar still has exactly five items. `Screen` with `scroll` on, all four edges, side margins `lg`.

Top to bottom:
1. **Back row**, 44dp: the chevron only (Ionicons `chevron-back`, 24dp; spoken 'Back'), as the picture. Returns to the tab the person came from.
2. **Heading**, `sm` below: 'Notifications' in the `title` role, `ink`, a header element.
3. **Sections**, `lg` below. Each section: a Label in capitals by style, letter spacing 1.5, `muted` ('Today', 'This week', 'Earlier'; the server's `section` is `today`, `week` or `earlier`; an empty section is left out), `sm` below it its rows.

**A row** (one button, at least 72dp high, `md` padding above and below, a 1dp `divider` rule under it across the margins): an `Avatar` (32dp) at the left, `md`, a column, and an unread dot at the right end. The column: the sentence in the Body role, `ink`, up to three lines, with the person's first name in the `button` role; under it, `xs` below, the item's `at_label` as sent in Label role, `muted` (the same rule as a comment's: a time of day today, a short weekday for the six days before, a month and day before that). The dot is a solid `ink` circle, 8dp, vertically aligned with the first line; there is none on a read row.

| Kind | Avatar | Sentence | Tapping |
|---|---|---|---|
| reaction | the person's initial or photo | '{Name} reacted {character} to your {category}.' | your own post with its comments (section 5) |
| comment | the person | '{Name} commented on your {category}: "{text}"' (straight quotation marks, the text cut by the server at about 60 characters) | your own post with its comments |
| friend_request | the person | '{Name} sent you a friend request.' | the Friends screen |
| friend_accepted | the person | '{Name} accepted your request.' | the Friends screen |
| spin_open | an outlined circle with the app's wheel mark (`BrandMark`, `ink`) | 'Your spin is open.' | Home |
| bonus_spin | a solid `ink` circle with a `dominant` dot | 'You hit {goal} this week. One bonus spin added.' | Home |

`{category}` is the category as written that day, in lower case ('your meal'). `{character}` is the reaction's character, drawn inline at the Body size from `lib/entry/reactions.ts`. `{goal}` is the server's number; if the server sends none the sentence reads 'You hit your goal this week. One bonus spin added.' 'Your spin is open.' appears only for the date in play, from 8:00 PM until midnight, and never as a list of past days.

**States.** Loading: the Label 'Loading your notifications.' (`muted`, `lg` below the heading). Offline: `StatusMessage` with 'Try again'. Empty: Phase 2's line 'Nothing new yet.' (`muted`). Ready: the sections. Opening the page loads the list and then marks everything seen; the dots the page loaded with stay until the person leaves. If the seen call fails nothing is shown (the next open tries again).

At most 50 items; no paging. A deleted post or comment's notification is simply gone from the list.

## 11. The bottom bar and the hidden routes

The bar has five items in this order: Home, Diary, Spin, Board, Profile. All of Phase 2 section 8 holds (fill, border, `56 + bottom inset`, 24dp icons in a 44dp hit area, Label size labels, idle `muted`, active `ink`, colour only, no bounce). The Board item is Ionicons `people-outline` with the label 'Board'. The Spin button keeps its circle and its two looks and is now in the middle of five. The pictures draw 'Roll' and a light grey filled circle; neither is used (spin wording, Phase 2's looks).

The bar is hidden (not drawn) on: the first-run page and the entry page (as before), the post page, and the Notifications page. Hidden routes are registered in `app/(app)/_layout.tsx` with `href: null` and `tabBarStyle` display none, and the Tabs use history for Back so a page opened from Board, Today or Notifications returns to where the person was. The Board tab keeps its scroll position and its loaded days while a post is open.

## 12. Motion

Tab switches, the bar's active state and pushed pages follow `.planning/phases/02.2-navigation-motion/02.2-CONTEXT.md` (the screens cross-fade in 200ms, the bar's colours glide over 220ms, pages use the platform push); nothing in this section overrides them.

Calm, nothing invented. A card press and a reaction press use the existing `PressScale` (0.97 over 90ms). A photo on the post page reveals with `ImageReveal` (450ms); the title row and details rise with `FadeInUp` (14dp, 400ms) 90ms apart. A comment you just added rises with `FadeInUp` once; comments loaded from the server do not. The locked card fades in over 400ms with the state change. List photos do not fade. No confetti, flash, sound, vibration, counter animation or bounce. Reduce Motion on: no scale, fade or rise anywhere in this phase.

## 13. Copy contract

Every string the person sees. 'Spin' wording, plain words, no em dashes, no legal text. N marks a new string; E marks an existing app or earlier-phase string reused.

| Where | String |
|---|---|
| Bar, Board heading | Board (N) |
| Board overline | Today / Last night (N, capitals by style) |
| Board states | Loading the Board. (N) / Couldn't connect. Check your connection and try again. (E) / Try again (E) |
| Locked card | Locked (N) / {n} friends posted today. / 1 friend posted today. / {n} friends posted last night. / 1 friend posted last night. / No friends have posted yet. (all N) / Post your spin to see what they got and how they rated it. (N) / Posting it now would count as late. (E, Phase 3) / Log your {category} (N) / Spin first (N) / Your next spin opens at 8:00 PM. (E, Phase 3, with a no-break space before PM) |
| Skipped | You skipped today. Friends' posts open after your next post. / You skipped last night. Friends' posts open after your next post. (N) / Go to Today (E, Phase 4) |
| No friends | Add friends to see what they spun today. / Add friends (N) / YOUR DIARY (N, capitals by style) / See diary (E, Phase 4) / Nothing here yet. Your first post will show up here. (E, Phase 4) |
| Day headings | Yesterday (N) / the short date such as TUE, OCT 6 (E: Phase 2's `dateLine`) |
| Card | the title and category as written; Late (N) |
| Post page | Board / Notifications / Today / Diary / Profile (the Back labels, N for the first two) / Loading this post. (N) / This post isn't here any more. (N) / Back to Board (N) / Couldn't load this photo. (E, Phase 3) / Block {first name} (N) |
| Reactions (spoken) | Love / Fire / Laughing / Wow / Sad / Sleepy (N) |
| Reaction messages | Couldn't connect. Check your connection and try again. (E) / Couldn't save that. Please try again. (N) |
| Comments | {n} comments / 1 comment (N) / Add a comment (N, the field's placeholder) / Post (N) / Delete (N) / Delete this comment? / It will be gone for good. / Delete / Keep it (all N) / Couldn't post your comment. Please try again. (N) |
| Own post | REACTIONS (N, capitals by style) / No reactions yet. (N) |
| Block sheet | Block {first name}? / You will stop seeing each other's posts. They won't be told. / Block / Don't block (all N) |
| Friends screen | Friends / Find a friend by username / Find / No one has that username. / Add friend / Requested / Accept / Decline / Cancel / Remove friend / Block {first name} / Remove {first name}? / You will no longer see each other's posts. / Remove / Keep / REQUESTS / SENT / FRIENDS / BLOCKED / No friends yet. / Unblock / Share an invite / Add me on RNDMRoll: @{username} / Loading your friends. / You are already friends. / You already sent a request. / You can't send more requests right now. Cancel one first. (all N) |
| Notifications | Notifications (E, Phase 2) / Today / This week / Earlier (N, capitals by style) / Nothing new yet. (E, Phase 2) / Loading your notifications. (N) |
| Notification sentences | {Name} reacted {character} to your {category}. / {Name} commented on your {category}: "{text}" / {Name} sent you a friend request. / {Name} accepted your request. / Your spin is open. / You hit {goal} this week. One bonus spin added. / You hit your goal this week. One bonus spin added. (all N) |
| Errors | Couldn't connect. Check your connection and try again. (E) / Something went wrong. Please try again. (E) |

## 14. Accessibility

- **Board.** The heading 'Board' is a header element; the overline is read with it ('Today, Board'). The bell: a button labelled 'Notifications', or 'Notifications, new' while the dot is on, hint 'Opens your notifications.', 44dp.
- **A card** is one button: '{title}. {n} out of 5 stars. {category}. By {first name}.' (own post: 'By you.'), then 'Posted late.' when late; hint 'Opens the post.' The name chip and the category plate are not separate elements.
- **Locked.** The card is one group: 'Locked. {n} friends posted today.' then the line, then the button. The tiles are hidden from VoiceOver (they are decoration). A polite announcement is not needed; the state is on screen when the tab opens.
- **Day headings** are header elements ('Yesterday', 'Tuesday, October 6' spoken from the server's date by Phase 4's fixed month table and a weekday name written by the server).
- **Post page.** The Back control is labelled 'Back to Board' (or the place). Author row: one element, 'Maya. Movie. 8:41 PM.' The photo is 'Photo for {title}'. The title row is one element ('Aftersun. 5 out of 5 stars. Their reaction: Sad.'). The reaction row's container is labelled 'Your reaction'; each button is its spoken name with state selected or not; a selected one has the hint 'Tap again to clear.' The count label is read ('3 comments'). Each comment is one element ('Jonah, 8:52 PM: The hotel room scene wrecked me.') with a separate 'Delete comment by Jonah' button. The comment field is labelled 'Add a comment'; 'Post' is a button labelled 'Post comment', disabled until there is text.
- **Own post.** 'Reactions: Maya, Fire. Priya, Sad.' is one element. Comments as above; the add field and 'Post comment' as above.
- **Friends screen.** Person rows are one element each with the action as a separate button ('Accept Leo', 'Decline Leo', 'Cancel request to Leo', 'Unblock Leo'). The result of Find is announced politely.
- **Notifications.** A row is one button: the sentence in full with the reaction named ('Maya reacted Fire to your meal'), then the time label, then 'New' when unread. The heading is a header element. Section labels are headers.
- **Targets.** Every tappable is at least 44dp (reaction buttons 44, rows 56 or 72, bell 44, chips 32 visual with 44 hit area).
- **Text size.** Everything scales with the phone's text size except the six emoji, the star glyphs and the avatar initials, which keep their size.
- **Contrast.** `muted` on `dominant` 5.82:1, `ink` 17.32:1, `destructive` as in Phase 1. The unread dot is shape, not colour: it is present or absent.

## 15. Notes for the plan

- **Plan split.** 05-01 to 05-03 are the server (friends and blocks and the visibility rule; the Board, posts, reactions and comments and the one photo-link function; the derived notification list and the seen time). 05-04 is the Friends screen and the Friends number. 05-05 is the Board, a friend's post page (picture 10), the reaction row, the comments, the additions to your own entry page (picture 06) and the five-item bar with the hidden routes. 05-06 is the notification list, the bell dots, and then the cutover, the gates, a short walkthrough and the close-out.
- **Two layouts, shared parts.** A friend's post (`PostView`, picture 10: photo about 3 to 2, author row on top, reaction row, comments, pinned comment bar) and your own entry page (`EntryView`, picture 06: square photo, fixed Edit and Delete footer, plus the three additions of section 5) are separate layouts. They share `CommentList`, `CommentBar` and, on a friend's post only, `ReactionRow`. There is no single post component and no single photo ratio.
- **Shared parts.** Plan 05-04 creates `lib/people/names.ts` (`firstName`, `initialOf`), `components/people/Avatar.tsx` (the person kind) and `components/people/BlockSheet.tsx`; plan 05-05 extends the Avatar with the wheel-mark and solid-dot kinds and imports the other two. Neither plan declares them again.
- **Wiring.** The Board and the post page read the endpoints of the contract in `05-CONTEXT.md` exactly. The phone never works out the day in play, the gate, a count, a late flag or a section. Time labels are written by the server and printed as sent: `posted_local_time` on a card (the author's zone), `at_label` on a comment and on a notification item (the viewer's zone). The phone does no date or time maths and holds no formatter.
- **Today.** Phase 5 edits two things in Today: the bell becomes `BellButton` and goes to `/(app)/notifications`; Today's refetch also asks for the unread summary. Nothing else in Today changes.
- **Pure logic in pure files** (`lib/board/*.ts`, `lib/notifications/rules.ts`) with no React and no imports, checked by throwaway node scripts and by device steps.
- **Launch.** `app/(app)/_layout.tsx` (Tabs: Board, two hidden routes, history back) is a launch file and changes here, so one cold launch is recorded after it changes. `app/index.tsx`, `app/_layout.tsx`, `lib/session/store.ts` and `lib/api/client.ts` are not edited. An edit to `lib/api/types.ts` (new error codes) can reload the dev client to Welcome: make it when the developer is not mid-test.
- **Public or private.** The choice is Phase 2.1's `users.account_visibility` (default public, switched in Settings). Nothing in Phase 5 reads it: the Friends view shows a friend's posts to accepted friends whatever their setting, and the Board has no switch. Phase 6 reads it for Global.
- **Deferred.** Notification settings: Phase 2.1's Settings page, with Phase 6's reminder row.
- **Never run `expo lint`.** No new native module in this phase. Tokens only, no hex. The emoji scan still allows exactly one file, `lib/entry/reactions.ts`. Round shapes only in the nine files of section 2.

## 16. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plans). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 13.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Board, locked | Locked / {n} friends posted today. / Post your spin to see what they got and how they rated it. / Log your {category} / Spin first / Your next spin opens at 8:00 PM. | From your picture. 'Roll' became 'spin'. 'Log your meal' now follows today's category. |
| 2 | Board, skipped and no friends | You skipped today. Friends' posts open after your next post. / Add friends to see what they spun today. / Add friends / YOUR DIARY | New. The first is the sentence you chose. |
| 3 | Board, days | Yesterday / TUE, OCT 6 | New. |
| 4 | Post page | Board / Notifications (Back labels) / Loading this post. / This post isn't here any more. / Back to Board | New. |
| 5 | Comments | {n} comments / Add a comment / Post / Delete / Delete this comment? / It will be gone for good. / Keep it | From your picture; the delete words are new. |
| 6 | Reactions | Love, Fire, Laughing, Wow, Sad, Sleepy, with the six characters of section 6 | From your picture. Different from the six you tapped for Phase 3: say "six" to keep those. |
| 7 | Own post | REACTIONS / No reactions yet. | New. |
| 8 | Block | Block {first name}? / You will stop seeing each other's posts. They won't be told. / Block / Don't block | New. |
| 9 | Friends screen | Friends / Find a friend by username / Find / No one has that username. / Add friend / Requested / Accept / Decline / Cancel / Remove friend / Remove {first name}? / Unblock / Share an invite / Add me on RNDMRoll: @{username} / REQUESTS / SENT / FRIENDS / BLOCKED / No friends yet. / You can't send more requests right now. Cancel one first. | New. No link in the invite until the domain exists. The last line shows when 50 of your requests are waiting. |
| 10 | Notifications | Notifications / Today / This week / Earlier / Nothing new yet. / the six sentences of section 10 | From your picture. 'Leo joined your group.' became 'Leo accepted your request.' and 'last week' became 'this week'. |

**Choices made here that you may want to change:**
- **Pills and circles.** Your pictures draw the name chip over each photo as a pill, the initials as circles, the ring around the chosen reaction and the comment box as a pill. The older rule was no pill and nothing tappable round, and the token note said never on a chip or an input. I followed your pictures, in exactly four new files (`Avatar`, `NameChip`, `ReactionRow`, `CommentBar`), and rewrote the token note.
- **The six reactions are the ones in your picture** (love, fire, laughing, wow, sad, sleepy), not the six you tapped for Phase 3 (heart, laughing, wow, fire, clap, yum). Phase 3's paper already uses them too, so the two phases agree. Say "six" and I put your tapped list back (`05-HAND-EDITS.md` has the recipe).
- **Words.** The picture says 'Post your roll' (now 'spin'), 'Log your meal' (now follows today's category), 'Leo joined your group.' (now 'accepted your request', because groups are not built), 'You hit 4 last week.' (now 'this week', because the bonus is paid the moment the goal is hit) and 'Roll' on the middle button (the bar says 'Spin').
- **Two photo ratios, two layouts.** A friend's post page has a wide photo, about 3 to 2, edge to edge (your picture 10). Your own entry page keeps Phase 3's square photo, edge to edge (your picture 06), with the comments added below your note and above Edit and Delete. They are two pages that share the comment list, the comment bar and, on a friend's post, the reaction row; they are not one page. Say if you want both photos the same.
- **No glass.** The Board is plain light cards, as your pictures. The glass look of 2026-10-01 is dropped.
- **No counts on a friend's reaction row.** Your picture shows none. You see who reacted on your own post and in Notifications.
- **Notification sections** are Today, This week and, new, Earlier (up to 30 days). Your picture shows the first two.
- **Your own card is not pinned** to the top of the Board. It sits among the others, shown as 'You', as in your picture.
- **The bar is hidden on the post page and on Notifications**, because your pictures 10 and 11 draw no bar. Phase 2's paper said the bar stays on Notifications. The bar still has exactly five items.
- **No Friends and Global switch yet.** It is in your pictures 08 and 09. It arrives in Phase 6 with the Global view, so there is no dead control now.
- **'Add a comment' is written inside the comment box**, as your picture. The older rule was no hint words inside boxes.
- **The category word sits on a small light plate** at the bottom of each card photo so it reads on any photo (your picture has bare grey text on a placeholder).
- **Names are first names only** on chips, comments and notifications ('Maya', not 'Maya Rao'). The Friends screen shows the full name and the username.
- **Avatars show the person's photo when they have one**, the initial in a circle when they do not (your picture draws only initials).
- **Comment text and names are 16**, a little larger than the picture's 14, to stay on the app's type scale. The card title and the note on a friend's post use one new small serif role, `cardTitle`.
- **Today before you post.** The locked card shows how many friends posted and the first names of up to four of them, as your picture does, and nothing else about their posts.
- **Reacting is allowed on a friend's post only, never your own.** The author's own reaction is the one beside the stars.
- **Block is in this phase** (a friend action); reporting a post comes with Global in Phase 6.
- **A friend request is shown as 'Requested' to the sender**, who can cancel it from 'Sent'. A decline, a removal and a block are silent.
