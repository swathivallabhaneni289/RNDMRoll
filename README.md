# RNDMRoll — daily dice-roll taste log

*(Name still TBD — "RNDMRoll" is the working title used for this repo.)*

A daily habit-loop app: a spin of the wheel assigns you a category — movie, book, song, meal, place, whatever's stocked into the wheel — and you log one real entry for it: a photo, a title, a star rating, and an optional one-line reaction.

Combines three researched, real precedents rather than cloning any single one: **BeReal's** daily-ritual/prompt hook, **Letterboxd's** structured rating-and-log mechanic, and a **randomizer** for variety and replay.

> **Status:** concept + prior-art research phase. No code yet — see [Open questions](#open-questions-still-unresolved) before implementation starts.

## What it is

Once a day (or, per the current decision below, strictly once a day), a spin of the wheel assigns you the day's category. You post one entry for it: a photo, a title, a star rating (1–5, half-star granularity), and an optional one-line reaction.

## Core mechanics

- **Wheel spin** picks the day's category — a visual, segmented spin-wheel (not a dice roll — see [Decisions](#decisions-made-so-far)), landing with a deceleration animation.
- **Entry = four fields, no matter what got rolled**: photo + title + rating + optional one-line note.
- **Photo source:** camera roll / photo library is allowed, not camera-only. You can log something you experienced a day or two before the roll (read a book two nights ago, rolled "book" today) rather than being forced to stage a photo live in the moment. See [Decisions](#decisions-made-so-far) for the reasoning.
- **Personal diary:** a running, chronological log of everything you've rated — your own visual taste history. A re-rolled category that repeats a title (e.g. a rewatch) creates a new dated entry rather than overwriting the old rating.
- **Friend feed:** a shared board showing what everyone in your circle rolled and rated that day, with lightweight (one-tap/emoji, not photo-based) reactions.

## Why this concept

Grounded in research, not just a vibe: BeReal proved the daily-surprise-prompt mechanic drives return visits; Letterboxd proved people enjoy rating/logging as a habit; both single-mechanic apps that lasted (vs. died, like Poparazzi/Artifact) had either a durable habit loop or a monetization fit — daily prompt + personal log combines the habit loop from one with the content structure of the other.

## Decisions made so far

| Decision | Choice | Why |
|---|---|---|
| Roll mechanic | **Spin wheel**, not a dice roll | A wheel visually shows every category as a wedge before landing, pairs naturally with a user-editable/reweightable category set (bigger wedge = more likely, no separate weighting UI needed), and the deceleration/anticipation moment is more satisfying and shareable than a flat dice number. A plain die also caps out awkwardly around 6 faces. Design care needed so it doesn't read as a spammy prize-wheel/gambling UI — lean flat/minimal, not carnival. |
| Photo source | **Camera roll / library allowed**, not live-capture-only | Forcing live in-app camera capture (BeReal/Rawly-style) actively fights logging something consumed hours or days before the roll happened — exactly the "rolled 'book' today, finished the book two nights ago" case. Live-only would force users to fake it or skip. This does reopen part of the authenticity-enforcement question (see risks below) since there's no technical guarantee against posting stock/official art — that's an accepted tradeoff, not solved yet. |

## Feature backlog

### Changes to the original mechanics (from prior-art research)

- **Once a day, not "on demand."** No unlimited manual re-roll — optionally earn a single extra reroll token per week via streak milestones. Wordle's creator deliberately capped it at one puzzle/day with no archive to preserve scarcity/non-addictiveness; the later paywalled archive was seen by fans as diluting that value.
- **Reciprocity gate conditional on friend-circle size.** Don't show a locked/empty feed to users with no friends yet on the app — route them to their personal diary + insights until a real circle exists. (BeReal's own cold-start problem, solved partly by Friends-of-Friends.)
- **If a gate is used, make it soft like BeReal's actual behavior**, not a hard block: posting late still unlocks the feed, just with a visible "late" badge.
- **Small (6–8 category) base wheel, fully user-editable** — add/remove/reweight — rather than one fixed universal list. (Closest precedent: Daylio's user-customizable activity set.)
- **Half-star granularity** (1–5 in 0.5 increments, Letterboxd-style), not integer stars and not Beli-style comparative ranking — ranking doesn't work across disparate categories (a movie can't be ranked against a sandwich).
- **Reactions stay one-tap/emoji**, not photo-based (i.e. don't default to BeReal's RealMoji selfie-reaction pattern) — the core entry is already heavier than BeReal's single photo, don't stack more friction on top.

### New features to consider

- **Your Year in Rolls** — free annual Wrapped-style recap (top categories, avg rating/category, longest streak, most-rated title), kept category-agnostic since there's no uniform metadata like genre/runtime across categories.
- **Spoiler-free roll card** — a shareable result card (category icon + star pips, no photo/title) for posting outside the app, encoding your own pattern (streak, rating distribution) rather than a shared daily answer.
- **Taste correlation insights** — "you rate meals higher on weekends," "movies avg 4.2 vs. books 3.1." The concrete payoff that makes solo/no-friends mode viable from day one.
- **RollGroups** — invite-only sub-circles (a book club, a foodie group) running their own private wheel and trigger window.
- **Behind-the-Scenes proof clip** — optional few-second clip before the still, viewable via long-press. A partial answer to "how do you stop someone photographing a poster/screen and calling it real," though note the current decision to allow camera-roll uploads at all already loosens this goal.
- **Friends-of-Friends discovery** — opt-in, same-gate-applies expansion, for users who have real-life friends on the app they haven't found yet.
- **Four Favorites pin** — four pinned all-time favorite rolls (any category) at the top of your profile, separate from the chronological diary.
- **Streak restore** — limited, conditional recovery for a broken streak.
- **Curated starter category packs** — one-tap "add all" theme bundles (e.g. "Nightlife," "Cozy Night In") inside the wheel editor, unlocked free via streak milestones rather than sold as IAP.

Full competitive research (prior-art verdict, closest matches, whitespace, and risks) is in [`RESEARCH.md`](./RESEARCH.md).

## Open questions still unresolved

- Exact wheel size and default starter categories.
- How weighting/reweighting works in the UI (drag to resize wedges? numeric sliders?).
- The core structural risk flagged by research: a rolled category isn't always satisfiable (what happens if the dice/wheel says "book" and you didn't read anything that day) — skip-breaks-streak vs. reroll-kills-the-differentiator is still undecided.
- Tech stack — not yet chosen.
- Whether/how to signal "this photo is from camera roll, logged a day late" vs. same-day, now that library uploads are allowed.
- Whether this needs a friend group to feel complete from day one, or ships solo-first with the insights feature as the retention hook.

---

Original raw concept notes (pre-research, pre-decisions) are preserved in [`docs/original-concept-notes.md`](./docs/original-concept-notes.md).
