# Phase 6: Follow overrides (BINDING, 2026-10-16)

This file WINS over `06-CONTEXT.md`, every `06-0*-PLAN.md`, `06-UI-SPEC.md`, `06-VALIDATION.md`
and `06-HAND-EDITS.md` wherever they disagree. Those papers were written on 2026-10-10 for mutual
friends. The developer decided on 2026-10-13 that following works like Instagram (one-way), and
`05-FOLLOW-OVERRIDES.md` turned Phase 5 into follows. Read the Phase 6 papers for Global, the
reports and the 8 PM reminder; read this file for what changed. Phase 5's override file is the
base: its wire contract, its `Relation`, its block rules and its house rules hold here too.

## 0. NO GLOBAL IN THIS VERSION (developer, 2026-10-16, wins over everything below)

Global is DEFERRED until more people use the app. Do NOT build `GET /v1/board?view=global`, the
public-entries index, or the Board switch: the Board shows the Friends view only (people you
follow), with no switch.

NO PUBLIC/PRIVATE SWITCH (developer, 2026-10-16, later the same day): every account behaves as
PRIVATE. Every follow is a request the other person Accepts or Declines; following stays one-way
(accepting does not follow back; "Follow back" is offered). Rule 3 of section 2 is OUT. The
"started following you" notification is OUT. The "Private account" row comes out of Settings;
`users.account_visibility` may stay in the database but nothing reads it. The person page: a
non-follower sees ONLY the header (photo, name, @username, figures) and the Follow button; an
accepted follower sees Tastes and the posts grid (each post through the visibility function).
The `private` field in wire shapes can be dropped or always true.

Phase 6 builds: the person page, report a post or a comment, block from those places, and the
8 PM reminder. Wherever sections 1 to 9 mention Global, Public accounts or the switch, skip it.

## 1. Words

- "friend", "friends", "Add friend", "Accept" (as a person-page button), "request_sent",
  "request_received" are OUT. Use "follow", "people you follow", "followers".
- EXCEPTION (developer, 2026-10-16: "keep it friends and global"): the Board switch still reads
  "Friends | Global", as in picture 08. The Friends side shows the people you follow.
  `GET /v1/board` without `view` is that view (Phase 5's), unchanged.
- "Home" is OUT everywhere: the reminder opens the Spin tab (`/(app)/today`).

## 2. Visibility (replaces the rule in `06-CONTEXT.md` api_contract)

A viewer sees an entry iff nobody blocked anybody, the entry is not archived (owner excepted),
AND one of:
1. the viewer is its owner; or
2. the viewer follows the owner (accepted) AND has a posted day for the entry's `local_date`
   (the Phase 5 gate, unchanged); or
3. the owner's account is Public (read at request time, never cached).

So a Public account's posts are seen by everyone signed in, follower or not, without the gate;
a Private account's posts only by accepted followers, behind the gate. This is the one function
everything goes through (posts, cards, reactions, comments, photo links), as before.

`comments_off` (migration 0007) is honoured on Global and person-page posts exactly as in Phase 5
section 4. `06-CONTEXT.md`'s "an owner switch to turn comments off" in Out of scope / Deferred is
OUT of date: the switch exists (Phase 3, 03-04).

## 3. The person page (replaces `06-CONTEXT.md` "Person page" and `06-UI-SPEC.md` section 7)

It looks like the Profile page (`docs/reference/profile-page-2026-10-13.png`), seen by a visitor:
no Edit button, no gear, no bell. Opened from a Global card's name, the post page author row, a
People page row (Phase 5) and a follow-type notification. A thin route in the Board and Profile
stacks; add it to the right `BARLESS_*` list.

- Header (always, for anyone not blocked): round photo, name, @username, the three figures
  Spins, Week streak, Followers (the same `StatsRow`, not tappable for a visitor), one relation
  button: "Follow" / "Requested" (tap cancels) / "Following" (tap opens Unfollow and Block) /
  "Follow back" when they follow you and you do not follow them. Existing `Chip`: strong for
  Follow and Follow back, outline for Requested and Following. A quiet "Block {name}" sits in a
  three-dots menu at the top right (`components/ui/MenuSheet.tsx`), not as a line on the page.
- PRIVATE account and the viewer is not an accepted follower: ONLY the header
  (`docs/reference/profile-header-what-a-visitor-sees-2026-10-13.png`), then one quiet line
  "This account is private." Follow sends a request (Phase 5 rules).
- PUBLIC account, or an accepted follower of a private one: the header, then Tastes and the
  posts as the same two-column grid, newest first, 20 a page. Each post in the grid passes the
  visibility function, so a follower of a private account sees only days they also posted.
- Reachable for any person who is not blocked either way. A block either way is 404 (the old
  "a Private non-friend is 404" rule is OUT).

## 4. Wire changes (replace the `GET /v1/people/{user_id}` row of `06-CONTEXT.md`)

| Route | Request | Success |
|---|---|---|
| `GET /v1/people/{user_id}` | `cursor`, `limit` 1 to 40 (posts, default 20) | 200 `{"person":Person,"relation":Relation,"follows_you":bool,"private":bool,"stats":{"spins":int,"week_streak":int,"followers":int},"tastes":[Taste]|null,"posts":[Card]|null,"next_cursor":string|null}`; `tastes` and `posts` are null when the header-only rule applies; 404 `not_found` when either blocked the other, or for an unknown id. Yourself: 200 with `relation` "none" is not used; the app opens your own Profile tab instead |

`Relation` is Phase 5's: `"none" | "requested" | "following"`. Follow, unfollow and block calls
are Phase 5's routes; Phase 6 adds none. `Taste` is the shape of `GET /v1/me/summary`. The stats
use the same queries as `/v1/me/summary` (Phase 5 renamed `friends` to `followers` there).

`GET /v1/board?view=global` and the two report routes stay as in `06-CONTEXT.md`. Global never
shows blocked people either way. Your own posts show on Global when your account is Public.

## 5. Report and block

Unchanged from the papers, except: "Block {name}" uses Phase 5's `POST /v1/me/blocks`, which also
removes follows both ways and pending requests. After a block from the person page the page
closes and the person is gone from every list.

## 6. Migration number

Phase 5 is `0008` (follows, blocks, settings, reactions, comments, seen time). Phase 6 is
`0009`: `post_reports`, `comment_reports`, and the index to read public entries newest first.
Check `ls migrations` before writing it.

## 7. The 8 PM reminder

Unchanged, except:
- The switch "Remind me at 8:00 PM" goes in Settings' **Notifications** page, under the three
  switches (reactions and likes, comments, milestones). The old placement "under Private account"
  is OUT. (Choice made here: listed in the UI spec's choices list.)
- Tapping the reminder opens the Spin tab.
- It is the ONLY Phase 6 change that may touch `package.json`, the lockfile and `app.json`
  (`npx expo install expo-notifications`), and only after the developer agrees to the rebuild.

## 8. House rules (same as Phase 5 section 5)

Four tabs (Diary, Spin, Board, Profile); no fades, no red text except Log out, no em dashes,
"spin" never "roll", plain light cards (no glass, no blur), no music, never `expo lint`.

## 9. Order

Phase 6 is built only after Phase 5 is built and closed (the other session is building it now).
Re-check every Phase 5 name used here against the built code and `05-0*-SUMMARY.md` first; the
built code wins.
