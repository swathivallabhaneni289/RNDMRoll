---
phase: 6
slug: public-global
status: draft
created: 2026-10-10
revision: 2
approval_word: go
---

# Phase 6 - UI Design Contract (Public Accounts, Global and the 8 PM reminder)

> Design contract the Phase 6 app plans (06-02, 06-03) build from. PHASE 6. NOTHING IS BUILT. It inherits every token, type size, spacing step, radius and elevation tier from `01-foundation-accounts/01-UI-SPEC.md` (revision 18 or later), and the bar, bell, Notifications page, `ConfirmSheet` and `StatusMessage` of `02-daily-roll/02-UI-SPEC.md` (revision 6 or later), the entry page and photo parts of `03-daily-entry/03-UI-SPEC.md` (revision 2 or later), and the Board, card, post page, comments and Friends screen of `05-friend-feed/05-UI-SPEC.md` (revision 1, cited below as 05-UI-SPEC with its section number; its numbers win where this document gives none), and the Settings page of `02.1-settings/02.1-UI-SPEC.md` (revision 1, cited below as 02.1-UI-SPEC; the reminder row sits on it). Phases 2, 2.1 and 3 to 5 are not built either. **There are NO pictures for this phase.** The developer's pictures 08, 09 and 10 (`docs/reference/screens-07-diary-08-board-locked-2026-10-10.png`, `docs/reference/screens-09-board-unlocked-10-post-2026-10-10.png`) draw the "Friends | Global" switch, the card grid and the post page, and every screen below is built from those parts and the standing tokens. The Global view, the person page, the two report sheets, the reminder sheet and the reminder's row on Settings have never been drawn, so they are plain defaults: send pictures and this document is redrawn from them. The public or private setting is not drawn here at all: it is Phase 2.1's (the 'Private account' switch on the Settings page, 02.1-UI-SPEC section 8). Every override of an older rule is in the choices list at the end of section 14. All imagery is the developer's: photos are people's own and every placeholder is plain.

## Visual reference (binding)

The developer's pictures named above are the visual reference for the build ("use exactly the same visuals ... as a reference to build this project", 2026-10-10). The plans build to match them, and at each screen's device check the phone screenshot is compared side by side with its picture. Every difference that is not in the "Choices made here that you may want to change" list at the end of this document is a defect. Where this document and a picture disagree, the picture wins and this document is corrected.

## 1. What this phase puts on screen

| Screen or state | Where | New or changed |
|---|---|---|
| Friends and Global switch | Board header (Phase 5's Board screen) | New (it is drawn in picture 08 and 09, Phase 5 leaves it out). |
| Global view | the Board, second view | New. |
| Post page, someone else's post | Phase 5's `PostView` (05-UI-SPEC 4), hidden route `app/(app)/post.tsx` | Changed: 'Report this post' beside Phase 5's 'Block {first name}' link (05-UI-SPEC 4 item 8 says Phase 6 adds it), a quiet 'Report' on each comment you cannot delete (05-UI-SPEC 4.1), the author row opens the person page. Reactions, comments and the comment bar are Phase 5's, for friends and strangers alike. |
| Report sheet (post) | over the post page | New. |
| Report sheet (comment) | over the post page | New. |
| Block sheet | Phase 5's shared `BlockSheet` (05-UI-SPEC 4.3) | New entry points only: the person page and the comment report sheet (the post page already has it). |
| Delete sheet (comment) | Phase 5's comment list (05-UI-SPEC 4.1), on a friend's post page and on your own entry page (05-UI-SPEC 5) | Changed: a comment that is not yours gets a second row, 'Delete and block {first name}'. |
| Person page | `app/(app)/board/person` (inside the Board stack) | New. |
| Name chip on a card | Phase 5's card | Changed: it opens the person page (it is a drawn label in Phase 5). |
| Reminder row | the Settings page (Phase 2.1's `SettingsScreen`), a new row directly under 'Private account' | New (built in plan 06-03). It is the only thing this phase puts on Settings or Profile: the setting, its switch and the gear are Phase 2.1's. |
| Welcome page | `app/(auth)/welcome.tsx` | Changed in code only: one hook call that cancels a stale reminder on a signed-out launch (section 9). Nothing on screen changes. |
| Reminder permission sheet | over Today, once | New (06-03). |
| The phone notification | the lock screen | New (06-03). |
| Bottom bar | `app/(app)/_layout.tsx` | Unchanged: Home, Diary, Spin, Board, Profile. |

The one new route, `person`, sits inside the Board stack (`app/(app)/board/`, which Phase 5 made a folder with its own Stack so later screens can push inside it). The post page stays Phase 5's hidden route `app/(app)/post.tsx`. Under Tabs a file placed directly in `app/(app)/` becomes a tab, so none is added.

## 2. Tokens added (the only new design values)

None. No new colour, space, radius, font or text role. Every value comes from `lib/theme/tokens.ts` as Phases 2 to 5 leave it.

Reused as they are:

| Need | Token |
|---|---|
| Text, the chosen half of the switch, filled controls | `ink` |
| Labels, helper lines, the unchosen half of the switch | `muted` (5.82:1 on the page) |
| Page, footers, text on `ink` | `dominant` |
| Hairlines, the switch track border, rules | `divider` and `inkRest` |
| Soft fills | `fieldSoft`, `secondary` |
| Errors | `destructive` |
| Disabled glyphs and the disabled switch thumb | `inkDisabled` |
| Radius on every tappable | `md` (8dp) |
| Primary button | `PrimaryButton` |
| Sheets | Phase 2's `ConfirmSheet` (already on the `radius.lg` list) |

**Radius.** Phase 6 adds no radius token. The Friends and Global track is a pill, drawn as picture 08 draws it (full width, rounded ends): the visual reference above wins over the old no-pill rule, and the choices list says so. Phase 5's spec (05-UI-SPEC section 2) already rewrote the token note and lists the files that may use `radius.full`: Phase 4's five, then `Avatar`, `NameChip`, `ReactionRow` and `CommentBar`. Phase 6 adds exactly one: `components/board/ViewSwitch.tsx`, the track. Nothing else here is round (the person page's photo reuses `Avatar`; the chips on cards are `NameChip`), and no file joins the `radius.lg` list.

**The phone's own switch.** The one switch this phase draws, on the reminder row, is the platform `Switch` exactly as Phase 2.1 draws 'Private account' (02.1-UI-SPEC section 2): track `ink` when on and `divider` when off, thumb `dominant`, all from tokens. It is rounded by nature (listed in the choices).

## 3. The Board header and the Friends | Global switch

Phase 5 draws the Board header as picture 08 and 09 (05-UI-SPEC 3.1: a fixed header with the overline, the title 'Board' and the bell, and the content under it) and leaves the switch for this phase. Phase 6 adds the switch under the title, as in the picture, as the last part of that fixed header:

1. **Overline.** In Friends: Phase 5's ("Today" or "Last night"). In Global: the Label 'Public posts' (capitals by style, `muted`, letter spacing 1.5, the same slot).
2. **Title** "Board" and the bell: unchanged.
3. **Switch**, `md` below the title: a full-width track inside the `lg` side margins, 44dp high, a 1dp `inkRest` border, holding two equal halves 'Friends' and 'Global'. The chosen half is an `ink` fill inset 3dp from the track, its text `dominant`; the other half has no fill and its text is `muted`. Text is the Button role. Each half is at least 44dp high and the whole half is the tap target. Friends is chosen on first open.
4. **What it does.** A tap on a half shows that view in place, below the switch, with no screen change and no animation. The choice is kept while the app is open (in the Board store) and is Friends again after a relaunch. Pull down refreshes the view that shows. The header and the switch are fixed (05-UI-SPEC 3.1); the content under them scrolls.
5. **No friends.** The switch still shows. Friends view is Phase 5's no-friends prompt with the person's diary; Global works the same for everyone.
6. **Skipped or locked** (Phase 5): they apply to the Friends half only. Global never shows a lock.

## 4. The Global view

A two-column grid of public posts, built from Phase 5's `PostCard` exactly (05-UI-SPEC 3.3 and picture 09: the photo at 7 to 8 with the name chip at its top left and the category word at its bottom left, the `cardTitle` title, the stars with the figure). Same card, same column width and gaps, same side margins, same press behaviour, rows of two. Differences from Friends:

- **No day sections and no gate.** One list, newest first by the time posted, with no "Today" or "Yesterday" headings. A person who has not posted today sees it at once.
- **Paging.** 20 cards a page, loaded as you scroll to the end. While the next page loads the Label 'Loading more.' (`muted`, centred) sits under the last row. At the end nothing is drawn. Pull down refreshes from the top. A card never appears twice (the list drops a repeated post).
- **Who is in it.** Posts of Public accounts, including the caller's own (shown as 'You', as in picture 09), never a person you blocked or who blocked you, never a post you reported.
- **Cards.** A card is one button that opens the post page (`from` = 'board'). The name chip on it (`NameChip`, 05-UI-SPEC 3.4: a drawn label in Phase 5) becomes a second button that opens the person page (a 44dp hit area; the chip itself looks the same); on the caller's own card (chip 'You') it stays a plain label. The card shows no comment count (Phase 5's card has none).
- **States.** All are Phase 2's `StatusMessage` pattern, centred under the switch:

| State | When | Shows |
|---|---|---|
| Loading | first fetch | the Label 'Loading public posts.' |
| Empty | the server returns no cards | the Label 'No public posts yet.' No picture, no button. |
| Offline | the fetch fails on a lost connection | 'Couldn't connect. Check your connection and try again.' in `destructive` and the standard 'Try again' button |
| Failed | any other failure | 'Something went wrong. Please try again.' and 'Try again' |
| Paging failed | the next page fails | the same line under the last row and a text link 'Try again' that asks for that page only; the cards already shown stay |

- **The Global list asks again** on tab focus and when it comes back to the foreground, as the Friends view does.

## 5. The post page for someone else's post

Phase 5's friend post page (picture 10). It serves a friend's post and a public stranger's post in exactly the same way: the six reactions, the comment count, the comment list and the 'Add a comment' bar are there for everyone who can see the post (developer, 2026-10-10: Global is "very similar to how it is for friends ... able to comment and everything"). Your own post does NOT open this page: it opens Phase 3's entry page with Phase 5's additions (05-CONTEXT, "Two pages, not one"), and this phase does not change it. What this page shows follows the fields the server sends, never a guess on the phone.

- **Author row.** The round initial or photo and the name, as in the picture. The left part (the circle and the name) is a button that opens the person page, with the whole row's height as its tap area. The right part ('MOVIE . 8:41 PM') stays a label. On the owner's own post the row is not a button.
- **Reactions and comments.** Exactly Phase 5's page, with no variant for strangers: the reaction row (six reactions, the chosen one ringed, '{n} comments' at its right), the comment list oldest first, and the 'Add a comment' bar pinned at the bottom (1 to 140 characters, the same limit and the same 'Post' text button). Not being a friend hides nothing: a stranger's comment appears with their name and time like any other, and a stranger writes one with the same bar. The post's owner can delete any comment on it and a writer can delete their own (Phase 5's 'Delete' and its sheet).
- **A comment row's action.** 05-UI-SPEC 4.1 draws the row: an `Avatar` (28dp), the first name, the comment's `at_label` and, when the server says `can_delete`, at the right end of that line the `TextButton` 'Delete' (Label size, tone `muted`, underlined, 44dp hit area). Phase 6 puts a `TextButton` 'Report' in that same place and the same style when `can_delete` is false (the comment is not yours and the post is not yours). It never shows on your own comment or on any comment on your own post, where 'Delete' shows: one or the other, never both.
- **Own post.** Not this page: it is Phase 3's entry page with Phase 5's reactions received, comments and comment bar, unchanged here. No Report and no Block there. If this page is ever given the caller's own post (`mine` true) it draws neither link.
- **The two quiet links.** 05-UI-SPEC 4 (item 8) already draws 'Block {first name}' at the end of the scrolling content, `xl` below the last comment (or below the reaction row when there are no comments), as a `TextButton` (tone `muted`, underlined, 44dp), and says Phase 6 adds 'Report this post' beside it. So the row holds 'Block {first name}' (Phase 5's, unchanged) and, to its right, 'Report this post' in the same style: `lg` apart when both fit inside the margins, else stacked left aligned with `md` between. With the comment bar pinned at the bottom (05-UI-SPEC 4.2) the links scroll above it. {first name} is the author's first name as Phase 5 writes it (`firstName`), cut with an ellipsis after 20 characters.
- **After a report** (section 6) the 'Report this post' link is replaced by the Label 'Thanks. We'll look at it.' (`muted`, a polite live region) and the page goes back by itself about 1.5 seconds later (immediately if the person taps Back first); the card is already gone from the lists.
- **After a comment report** (section 6) the comment leaves the list at once and the Label 'Thanks. We'll look at it.' (`muted`, a polite live region) shows on its own line directly above the comment bar for about 3 seconds. The page stays.

## 6. The report sheets and the block sheet

**Report sheet.** 'Report this post' opens a bottom sheet that reuses `ConfirmSheet`'s modal, scrim, top corners and padding. Title (Heading role): 'Report this post'. Five full-width rows, each at least 56dp high, separated by hairlines, Button role, `ink`: 'It's spam', 'It's unkind or harassing', 'It shouldn't be on a public feed', 'Something else', and last 'Cancel'. No free text and no second confirmation: a tap on a reason sends it. The rows map to the server's `spam`, `unkind`, `not_public`, `other`, in that order.

| Case | What the person sees |
|---|---|
| Sent | the sheet closes; the link becomes 'Thanks. We'll look at it.' (section 5) |
| No connection | the sheet closes; under the link the Label 'Couldn't connect. Check your connection and try again.' in `destructive`; the link stays so it can be tried again |
| Rate limited, or anything else | the Label 'Something went wrong. Please try again.' in `destructive` under the link |
| The post is gone (404) | the page shows Phase 5's gone state and goes back |

Reporting the same post twice is the same answer (the server stores one report per person and post), so a repeat tap is harmless.

**Comment report sheet.** A comment's 'Report' link (section 5) opens the same kind of bottom sheet. Title (Heading role): 'Report this comment'. Five full-width rows, each at least 56dp high, separated by hairlines, Button role, `ink`: 'It's spam', 'It's unkind or harassing', 'Something else', 'Block {first name}' (the comment writer's first name as the comment row shows it, cut with an ellipsis after 20 characters) and last 'Cancel'. No free text. A tap on one of the first three sends the report (`spam`, `unkind`, `other`, in that order) and closes the sheet; 'Block {first name}' closes it and opens Phase 5's block sheet (below) for the comment's writer (a second, confirming step: one tap never blocks).

| Case | What the person sees |
|---|---|
| Sent | the sheet closes; the comment leaves the list at once; the Label 'Thanks. We'll look at it.' shows directly above the comment bar for about 3 seconds |
| No connection | the sheet closes; in the same place the Label 'Couldn't connect. Check your connection and try again.' in `destructive` for about 3 seconds; the comment stays |
| Your own comment (409) or a comment that is gone (404) | the post reloads (the comment is gone, or it shows 'Delete'); the Label 'Something went wrong. Please try again.' in `destructive` in the same place |
| Rate limited, or anything else | the Label 'Something went wrong. Please try again.' in `destructive` in the same place; the comment stays |

A repeat report of the same comment is the same answer, so a repeat tap is harmless. A reported comment is left out only of the reporter's lists; the writer and everyone else still see it, and nothing happens automatically.

**Delete sheet of a comment that is not yours.** 05-UI-SPEC 4.1 draws the delete sheet: `ConfirmSheet`, title 'Delete this comment?', body 'It will be gone for good.', rows 'Delete' (`destructive`) and 'Keep it'. The comment list is shared by a friend's post page and your own entry page (05-UI-SPEC 5), so the sheet is too. When the comment's writer is not the caller (the post's owner deleting a friend's or a stranger's comment), Phase 6 adds a second row between the two: 'Delete and block {first name}' (text in `destructive`). It deletes the comment and then blocks its writer with Phase 5's two existing calls in that order (`POST /v1/comments/{id}/delete`, then `POST /v1/blocks`); the sheet and the row's words are the confirmation, there is no third step. On your own comment the sheet stays Phase 5's, two rows.

| Case | What the person sees |
|---|---|
| Both succeed | the sheet closes and the row is gone; the writer's other comments and posts are gone from your lists at the next reload |
| The delete answers 'not found' | counted as done; the block is still sent |
| The delete fails (no connection, or anything else) | nothing is deleted and no block is sent; Phase 5's offline sentence above the comment bar |
| The delete succeeds and the block fails | the comment is already gone (a delete has no undo); the Label 'Comment deleted. Couldn't block {first name}.' in `destructive` directly above the comment bar, with a `TextButton` 'Try again' that sends only the block; it stays until the block succeeds or the page is left |

**Block sheet** (Phase 5's shared `components/people/BlockSheet.tsx`, 05-UI-SPEC 4.3; Phase 6 builds none of its own). Title 'Block {first name}?', body 'You will stop seeing each other's posts. They won't be told.', rows 'Block' (`destructive`) and 'Don't block'; 'Block' sends `POST /v1/blocks`. Phase 6 adds two ways in: the comment report sheet's 'Block {first name}' row and the person page's link (the post page already has it). On success the screen goes back to where the person came from, which asks the server again, and the person's cards are gone from Friends and Global at once (and their comments from every post you open).

## 7. The person page

Route `app/(app)/board/person`, opened with the person's id. Pushed over the Board with a Back control (chevron and 'Back', as on Edit wheel). The bottom bar stays. Top to bottom, `lg` side margins, scrolling:

1. The Back row, 44dp.
2. `md` below: the header row: the person's round photo, 64dp, or their initial on a `secondary` circle when they have none (Phase 5's `Avatar`, `components/people/Avatar.tsx`, 05-UI-SPEC section 2, at the size its size prop gives; this phase adds no new round file); to its right the name (Heading role, up to two lines) and under it the @username (Label, `muted`).
3. `md` below: one full-width button (`PrimaryButton`, 8dp) by the server's `relation`:

| relation | Button |
|---|---|
| none | 'Add friend' (primary; sends a request, then reads 'Requested') |
| request_sent | 'Requested' (outlined, disabled) |
| request_received | 'Accept' (primary; accepts, then reads 'Friends') |
| friends | 'Friends' (outlined, disabled) |

   A request failure shows Phase 2's offline or generic line in `destructive` under the button.
4. `sm` below: one Label line, `muted`: 'Public account' or 'Private account' (from `visibility`, the person's `users.account_visibility` as Phase 2.1 stores it; the page only shows it and never changes it).
5. `lg` below, a 1dp `divider` rule, then:
   - **Public person** (`posts` is a list): the small-caps Label 'Posts' and the same two-column grid as Global, newest first, 20 a page, with the same paging and states. An empty list says 'No posts yet.' A card opens the post page with `from` = 'person' (its Back label reads 'Back').
   - **Private person who is a friend** (`posts` is null): one Label line, `muted`: 'Their posts show on your Board.'
6. `xl` below the content, the quiet `TextButton` 'Block {first name}' (same style as on the post page), which opens Phase 5's block sheet (section 6).

States: Loading shows the Label 'Loading this person.'; offline is `StatusMessage` with 'Try again'; a 404 shows 'This person isn't here.' and a 'Back' text button. The page reloads when it comes back into focus, so a request or a block shows at once. The person page is never opened for the caller's own account (the caller's own card is not a button).

## 8. The public or private setting (Phase 2.1's, only read here)

Phase 6 draws no switch, no sheet and no line for the setting, and the phone never writes it. The setting is Phase 2.1's: the column `users.account_visibility`, PUBLIC by default for everyone, switched by the row 'Private account' on the Settings page (ON means private; it saves the moment it is tapped and asks nothing; its sentence is 'Public accounts can appear on Global. Private accounts are seen by friends only.', 02.1-UI-SPEC section 8). An account whose birthday makes it under 18 STARTS private and can switch like anyone else (02.1-CONTEXT D-11).

What this phase does with it, always from the server's reading of the column at the moment a list or a page is read (never a copy per post, never a cache on the phone):
- **Global** lists the posts of authors whose account is public, and only those (section 4).
- A person who switches to private leaves Global at once, past posts included, and appears again, past posts included, when they switch back. A card already on a phone's screen stays until that phone asks again (tab focus, pull down, back to the foreground); opening it then answers 'not found' and the post page shows Phase 5's gone state.
- **The person page** shows 'Public account' or 'Private account' from the server's `visibility` (section 7).
- **Not here:** the switch, a consent sheet, a line about age, any refusal. Switching to public asks nothing in Phase 2.1, so past posts show on Global with no question; the sentence under the switch is the only warning (choices list).

**Safety note (a recommendation, not a rule).** An under-18 account that switches to public lets strangers see and comment on a young person's posts. Phase 2.1 deferred refusing it and Phase 6 adds no refusal: it is one server line, for the developer to decide before any outside user (launch list, and the choices list below).

## 9. The 8 PM reminder (plan 06-03)

A LOCAL notification the phone schedules itself, daily at 20:00 in the phone's own time zone. The server is not involved and no Apple push setup is needed. It needs one native module and one rebuild (plan 06-03). If the installed dev client does not have the module, none of this shows: no row, no sheet, nothing dead.

**The row.** On the Settings page (Phase 2.1's `SettingsScreen`), a new row directly under 'Private account' and above 'Log out', so the rows read Edit profile, Username, Password, Private account, Remind me at 8:00 PM, Log out. It is drawn exactly like the 'Private account' row (02.1-UI-SPEC section 8): at least 56dp, the label 'Remind me at 8:00 PM' (Body role, `ink`) at the left and the switch at the right, the whole row one switch element so a tap anywhere toggles, and a 1dp `divider` hairline under it as under every Settings row. Under the row, `xs` below inside the same hairline block and only when the system permission is off: the Label 'Turn on notifications for RNDMRoll in Settings.' (`muted`). The switch then shows off and dimmed and a tap on it does nothing but show that line (the row is disabled). The row is not drawn when the module is missing.

**The permission sheet, asked once.** When Today shows Posted for the first time on this phone after this phase is installed, and the person was never asked, a `ConfirmSheet` opens about 600ms after the Posted state has appeared (so it does not talk over the 'Posted.' announcement). Title (Heading): 'Get a reminder at 8:00 PM?'. Body (Body): 'We'll tell you when your spin opens.' Rows: 'Remind me' and 'Not now'.
- 'Remind me' closes the sheet, switches the Reminder on and asks the system for permission (the system's own prompt, whose wording the app cannot change). Allowed: the reminder is scheduled. Not allowed: the switch stays off and the row on Settings shows the Settings line.
- 'Not now' (or a swipe down) is remembered and the sheet is never shown again. The row on the Settings page is the way back.

**The notification.** Title 'Your spin is open.' Body 'Spin today's wheel.' Daily at 20:00 local time. No category word, no name, no counts: it says the same thing to everyone and reveals nothing on a lock screen. It plays the phone's default notification sound, which the person can turn off in iOS Settings. It is not shown while the app is open in the foreground (the Today screen already says it). Tapping it opens the app on Home.

**Behaviour.**
- The schedule is rebuilt every time Today opens for a signed-in person and every time the app comes back to the foreground: all earlier schedules are cancelled and, if the switch is on and the permission is granted, exactly one is added. It can never double.
- Turning the switch off cancels it. **Every way of signing out cancels it:** the Log out row on Settings, the Log out button on the Edit profile page and a session that expires while the app is open all end in the same session change, and the reminder hook (alive on Today for the whole signed-in session) cancels every scheduled reminder as soon as the session is no longer signed in, and again when the signed-in screens go away. A session that ended while the app was closed (a revoked sign-in, for example after a password change on another phone) cannot be cancelled until the app is next opened: every signed-out launch starts on Welcome, and a one-line hook there cancels every scheduled reminder at once. Until that launch the reminder can still show once a day, with the same neutral words. Nothing in the session store or in the Log out controls is edited for any of this. The next person to sign in on the phone gets their own schedule when Today opens.
- The switch value and the "already asked" mark live on the phone, per account (so a second person on the same phone is asked on their own).
- It still fires on a day the person has already spun or posted. That is accepted in this version.

## 10. Motion

Tab switches, the bar's active state and pushed pages follow `.planning/phases/02.2-navigation-motion/02.2-CONTEXT.md` (the screens cross-fade in 200ms, the bar's colours glide over 220ms, pages use the platform push); nothing in this section overrides them.

Calm, as everywhere. Nothing new is invented.
- The Friends and Global halves and the reminder switch change at once. Content swaps with no animation.
- Sheets use `ConfirmSheet`'s existing motion.
- Cards reveal with Phase 5's `ImageReveal` and nothing else.
- The 'Thanks. We'll look at it.' line appears at once, and a reported comment leaves the list at once with no fade.
- Reduce Motion on: nothing fades or rises in this phase; things appear at once.

## 11. Copy contract

Every string the person sees. 'Spin' wording, plain words, no em dashes, no legal text. N marks a new string; E marks an existing app, Phase 2 or Phase 5 string reused. {first name} is the person's first name as Phase 5 writes it (`firstName`, 05-UI-SPEC 3.4); where an earlier section says {name}, read {first name}.

| Where | String |
|---|---|
| Board switch | Friends (E, picture 08) / Global (E, picture 08) / Public posts (N, overline in Global) |
| Global states | Loading public posts. (N) / No public posts yet. (N) / Loading more. (N) / Couldn't connect. Check your connection and try again. (E) / Something went wrong. Please try again. (E) / Try again (E) |
| Post page links | Report this post (N) / Block {first name} (E, Phase 5) / Thanks. We'll look at it. (N) |
| Report sheet | Report this post (N) / It's spam (N) / It's unkind or harassing (N) / It shouldn't be on a public feed (N) / Something else (N) / Cancel (E) |
| Comment, report | Report (N) / Report this comment (N) / It's spam (E) / It's unkind or harassing (E) / Something else (E) / Block {first name} (E, Phase 5's words) / Cancel (E) / Thanks. We'll look at it. (E) |
| Comment, delete | Delete this comment? (E, Phase 5) / It will be gone for good. (E) / Delete (E) / Delete and block {first name} (N) / Keep it (E) / Comment deleted. Couldn't block {first name}. (N) / Try again (E) |
| Block sheet | Block {first name}? (E, Phase 5) / You will stop seeing each other's posts. They won't be told. (E) / Block (E) / Don't block (E) |
| Person page | Add friend (E, Phase 5) / Requested (E) / Accept (E) / Friends (E) / Public account (N) / Private account (E, Phase 2.1's Settings label) / Posts (N) / No posts yet. (N) / Their posts show on your Board. (N) / Loading this person. (N) / This person isn't here. (N) / Back (E) |
| Settings, reminder row | Remind me at 8:00 PM (N) / Turn on notifications for RNDMRoll in Settings. (N) |
| Reminder sheet | Get a reminder at 8:00 PM? (N) / We'll tell you when your spin opens. (N) / Remind me (N) / Not now (N) |
| The notification | Your spin is open. (E, Phase 2's heading) / Spin today's wheel. (N) |
| Errors | the existing offline and generic strings only. `own_comment` and `not_found` on a report have no string of their own: the post reloads and the generic line shows |

## 12. Accessibility

- **Board switch.** The container is labelled 'Board view'; each half is a button with the state selected or not ('Friends, selected'). Changing it announces 'Friends' or 'Global' politely.
- **Global grid.** Each card reads as Phase 5's card does (05-UI-SPEC 14: '{title}. {n} out of 5 stars. {category}. By {first name}.'); the name chip becomes a second button, 'Open {first name}'s page'. 'Loading public posts.' and 'No public posts yet.' are polite live regions.
- **Post page.** The author row is a button 'Open {name}'s page'. 'Report this post' and 'Block {first name}' are buttons; Report's hint is 'Asks why.' and Block's is 'Asks first.' A comment's 'Report' is a button labelled 'Report comment by {first name}' with the hint 'Asks why.' (05-UI-SPEC 14 already names Phase 5's 'Delete comment by {first name}' in the same place).
- **Delete sheet.** The row 'Delete and block {first name}' is a button labelled 'Delete this comment and block {first name}' with the hint 'Deletes it, then blocks them.' The failure line 'Comment deleted. Couldn't block {first name}.' is a polite live region and its 'Try again' is a button labelled 'Try blocking {first name} again'. 'Thanks. We'll look at it.' is a polite live region, and so is the message line above the comment bar.
- **Report sheets.** Each row is a button labelled by its words ('Block {first name}' in the comment sheet keeps the hint 'Asks first.'). Focus lands on the title.
- **Person page.** The header is one element: '{name}, @{username}, {Public or Private} account'. The relation button announces its state ('Friends, disabled'). 'Posts' is a header element.
- **Reminder switch.** 'Remind me at 8:00 PM' is one element with the switch role and the state on or off, as Phase 2.1's 'Private account' row is; when the permission is off it is announced as dimmed and its line is read with it ('Remind me at 8:00 PM, off, dimmed. Turn on notifications for RNDMRoll in Settings.').
- **Reminder sheet.** As `ConfirmSheet`: title first, then body, then the rows.
- **Targets.** Every tappable is at least 44dp (switch halves 44, rows 56, links 44, the chip's hit area 44).
- **Text size.** Everything scales with the phone's text size except the category word on cards and the six emoji (Phase 5).
- **Contrast.** `muted` on `dominant` 5.82:1, `ink` 17.32:1, `destructive` as in Phase 1.

## 13. Notes for the plan

- **Plan split.** 06-01 is the server. 06-02 builds the pure logic, the API calls, the Friends and Global switch, the Global view, the post page's 'Report this post' link (beside Phase 5's Block link), the quiet 'Report' on a comment and the two report sheets, the extra 'Delete and block' row on the comment delete sheet, and the person page. 06-03 builds the reminder (the module, the sheet, its row on the Settings page, the scheduler, the rebuild) and runs the cutover, gates, walkthrough and close-out. The public or private setting, its switch, the Settings page and the gear are Phase 2.1's, and none of it is built here.
- **Wiring.** The Board store gets a `view` value (`friends` or `global`) and a second list for Global. The phone never works out age, friendship or visibility, and never writes the account setting: it reads `relation`, `visibility`, `can_comment`, `can_react` and each comment's `can_delete`. The block and report calls also remove the person's or the post's cards, and a comment report removes the comment, from memory at once.
- **Pure logic in pure files** (`lib/board/view.ts`, `lib/board/report.ts`, `lib/reminder/plan.ts`), checked by throwaway node scripts (`scripts/check-global.ts`, `scripts/check-reminder.ts`) and by device steps.
- **Launch.** `app/(app)/_layout.tsx`, `app/index.tsx`, the root and auth layouts, `lib/session/store.ts` and `lib/api/client.ts` are not edited. The person route is added to the Board stack only. No Profile file is edited (`ProfileForm.tsx` stays as Phases 1 and 2.1 left it). The one edit to a Phase 2.1 file is `components/settings/SettingsScreen.tsx`: one import and one element, the reminder row under 'Private account'. The reminder is cancelled on every sign-out by the reminder hook that Today already mounts, and on a signed-out launch by a second one-line hook on the Welcome screen (section 9), so the session store and the Log out controls are not touched. `app/(auth)/welcome.tsx` is the one Phase 1 screen this phase edits (an import and one call; no layout and no launch file).
- **Rebuild.** Plans 06-01 and 06-02 need no rebuild. Plan 06-03 adds `expo-notifications` and rebuilds the dev client once; before the rebuild the app must start and behave exactly as at the end of 06-02 (the module import is guarded).
- **Never run `expo lint`.** Tokens only, no hex. The emoji scan still allows exactly one file, `lib/entry/reactions.ts`; the notification text has no emoji.

## 14. Strings for approval

Developer: say keep or change for each row. The word go is only for starting the build (see the plans). Strings are approved in the walkthrough, after you have seen each screen. Every string is in section 11.

| # | Where | Strings | Note |
|---|---|---|---|
| 1 | Board | Public posts | New overline in Global. The switch words are yours (picture 08). |
| 2 | Global | Loading public posts. / No public posts yet. / Loading more. | New. |
| 3 | Post page | Report this post / Thanks. We'll look at it. | New. 'Block {first name}' beside it is Phase 5's. |
| 4 | Report sheet | Report this post / It's spam / It's unkind or harassing / It shouldn't be on a public feed / Something else | New. Four reasons, no free text. |
| 5 | Comment | Report / Report this comment / It's spam / It's unkind or harassing / Something else / Block {first name} / Thanks. We'll look at it. | New. The first three reasons use the post sheet's words. Block is a second, confirming step (Phase 5's block sheet). |
| 6 | Delete and block | Delete and block {first name} / Comment deleted. Couldn't block {first name}. | New. The rest of the delete sheet (Delete this comment? / It will be gone for good. / Delete / Keep it) is Phase 5's. |
| 7 | Person page | Public account / Private account / Posts / No posts yet. / Their posts show on your Board. / Loading this person. / This person isn't here. | New. The block sheet is Phase 5's (Block {first name}? / You will stop seeing each other's posts. They won't be told. / Block / Don't block). |
| 8 | Reminder | Remind me at 8:00 PM / Turn on notifications for RNDMRoll in Settings. / Get a reminder at 8:00 PM? / We'll tell you when your spin opens. / Remind me / Not now | New. The row is on the Settings page, under 'Private account' (a Phase 2.1 word). |
| 9 | Notification | Your spin is open. / Spin today's wheel. | The first is your Today heading. |

**Choices made here that you may want to change:**
- The setting is **Phase 2.1's, not built here**: every account is **Public by default** (yours, the four test accounts and every older account included) and the 'Private account' switch on the Settings page turns it private. That is your own rule ("originally it's gonna be a public account and they can turn it into private, just like how Instagram works"); it replaces my first notes (Private by default, under 18 never Public, a Public switch on Edit profile with a sheet).
- **An account under 18 starts private and may switch it**, like anyone else (Phase 2.1). Nothing in Phase 6 refuses public for an under-18 account, so strangers could then see and comment on a young person's posts. **This is a recommendation for you to decide before any outside user:** refusing it is one server line and it is on the launch list. I have not added it.
- **Global is open at any time** and is not behind the post-first rule. A friend who is Public is therefore visible on Global without posting. The post-first rule protects private posts only.
- **Strangers can comment on public posts, as you said.** Global works like Friends: the post page has reactions, the comments and the comment bar for everyone who can see it. **The owner can delete any comment** on their post and a writer can delete their own. **Anyone can report a comment** (a quiet 'Report' where 'Delete' would be) **or block its writer** (a second, confirming step). There is no switch to turn comments off and no word filter; Phase 5's 20 a minute limit is the only brake.
- **No question is asked when an account becomes public**, and the posts it already has show on Global at once (turning 'Private account' off asks nothing in Phase 2.1). Because every existing account is public, **all their past posts are on Global the moment it exists**. Only the test group is here, so that is fine now; before any outside user it is on the launch list. Say if you want a question back.
- **No per-post switch.** The account setting decides.
- The **Friends and Global switch** is drawn as in your picture 08: a pill with rounded ends, in the pill style Phase 4 and 5 already use (your picture wins over the old no-pill rule). The **reminder switch** is the phone's own switch (rounded by nature), like 'Private account'.
- The overline in Global reads **Public posts** (the picture's 'Today' belongs to the Friends day).
- **Report** is one sheet with four reasons for a post, and one with three reasons plus Block for a comment, no free text. A reported post or comment disappears for the reporter at once and stays for everyone else. **Nothing happens automatically**: reports go into tables you read. Who reads them, and the Terms, are on the launch list.
- **Delete and block.** When you delete a comment that is not yours (it is on your post), the delete sheet has a second row, 'Delete and block {first name}': it deletes the comment and then blocks the writer, as two calls in that order. If the block fails after the delete, the comment is already gone and you see 'Comment deleted. Couldn't block {first name}.' with a 'Try again' that sends only the block. On your own comment the sheet stays Phase 5's two rows.
- **Report and Block also show on a friend's post**, not only on strangers'. The block sheet and its words are Phase 5's.
- The **person page is small**: photo, name, @username, one button, and posts only when the account is Public. A private friend's posts stay on the Board.
- The **reminder is a local notification**, daily at 8:00 PM, with the same words for everyone and no category word. It fires even on a day you already spun. It is asked once, after your first posted day, and its row on the Settings page (under 'Private account') is the way back. Every way of logging out cancels it. It needs one rebuild of the test app.
- The reminder row's line says "in Settings" (your words in the notes). On the app's own Settings page that could be read as this page, not the phone's Settings app. If you want, I change it to "Turn on notifications for RNDMRoll in your phone's Settings."
- iOS has no wording the app can set for the notification permission prompt; the sheet before it is the explanation.
