# Context (from DOC sources)

Synthesized from three DOC-type sources. README.md and RESEARCH.md carry a mutual cross-reference (see `INGEST-CONFLICTS.md` INFO entry); docs/original-concept-notes.md is explicitly framed by README.md as the "pre-research, pre-decisions" predecessor document.

Sources:
- README.md — source: /Users/swathivallabhaneni/code/RNDMRoll/README.md
- RESEARCH.md — source: /Users/swathivallabhaneni/code/RNDMRoll/RESEARCH.md
- docs/original-concept-notes.md — source: /Users/swathivallabhaneni/code/RNDMRoll/docs/original-concept-notes.md

---

## Topic: Product Concept & Naming

- Working title "RNDMRoll" used for the repo; final name explicitly still TBD. (source: README.md)
- Earlier notes titled the concept "[Name TBD]" and deferred naming as "his call, not listed here." (source: docs/original-concept-notes.md)
- Concept: a daily habit-loop app combining BeReal's daily-ritual/prompt hook, Letterboxd's structured rating-and-log mechanic, and a randomizer for variety/replay — not cloning any single precedent. (source: README.md; source: docs/original-concept-notes.md)
- Status as of README.md: concept + prior-art research phase, no code yet.

## Topic: Core Mechanics

- Roll/spin assigns the day's category (movie, book, song, meal, place, etc., from a stocked wheel). (source: README.md; source: docs/original-concept-notes.md)
- Entry is a fixed 4-field structure regardless of rolled category: photo + title + star rating + optional one-line reaction. (source: README.md; source: docs/original-concept-notes.md)
- Star rating: 1-5 with half-star granularity, Letterboxd-style (not integer, not Beli-style comparative ranking — ranking doesn't work across disparate categories). (source: README.md)
- Personal diary: running chronological log of everything rated, forming a visual taste history. A repeated title (e.g. a rewatch) creates a new dated entry rather than overwriting the old rating. (source: README.md; source: docs/original-concept-notes.md)
- Friend feed: shared board showing what everyone in the friend circle rolled/rated that day, with lightweight one-tap/emoji reactions (explicitly not photo-based, i.e. not BeReal's RealMoji pattern). (source: README.md; source: docs/original-concept-notes.md)

## Topic: Decisions Made So Far (informal, recorded in DOC not ADR)

- **Roll mechanic: spin wheel, not a dice roll.** Rationale: a wheel visually shows every category as a wedge before landing, pairs naturally with a user-editable/reweightable category set (bigger wedge = more likely), the deceleration/anticipation moment is more satisfying/shareable than a flat dice number, and a die caps out awkwardly around 6 faces. Design care needed so it doesn't read as a spammy prize-wheel/gambling UI. (source: README.md)
- **Photo source: camera roll/library allowed, not camera-only (live-capture-only rejected).** Rationale: forcing live in-app capture fights logging something consumed hours/days earlier (e.g. rolled "book" today, finished the book two nights ago); live-only would force users to fake it or skip. Acknowledged tradeoff: reopens part of the authenticity-enforcement question since there's no technical guarantee against posting stock/official art — noted as an accepted, unsolved tradeoff. (source: README.md)
- Both of the above decisions diverge from docs/original-concept-notes.md's earlier framing (dice-roll randomizer; strict self-taken-photo-only, "no auto-fetched/stock images, ever"). See `INGEST-CONFLICTS.md` for the competing-variant writeups — these are preserved as separate variants, not merged.

## Topic: Feature Backlog — Changes to Original Mechanics (from prior-art research)

- Once a day, not "on demand" — no unlimited manual re-roll; optionally earn a single extra reroll token per week via streak milestones. Rationale: Wordle's creator deliberately capped it at one puzzle/day with no archive to preserve scarcity/non-addictiveness; the later paywalled archive was seen by fans as diluting that value. (source: README.md)
- Reciprocity gate conditional on friend-circle size — don't show a locked/empty feed to users with no friends yet; route to personal diary + insights until a real circle exists (BeReal's own cold-start problem). (source: README.md)
- If a gate is used, make it soft like BeReal's actual behavior, not a hard block — posting late still unlocks the feed with a visible "late" badge. (source: README.md)
- Small (6-8 category) base wheel, fully user-editable (add/remove/reweight) rather than one fixed universal list — closest precedent: Daylio's customizable activity set. (source: README.md)
- Half-star granularity (1-5 in 0.5 increments, Letterboxd-style), not integer stars, not Beli-style comparative ranking. (source: README.md)
- Reactions stay one-tap/emoji, not photo-based — the core entry is already heavier than BeReal's single photo. (source: README.md)

## Topic: Feature Backlog — New Features to Consider

- Your Year in Rolls — free annual Wrapped-style recap (top categories, avg rating/category, longest streak, most-rated title), category-agnostic. (source: README.md)
- Spoiler-free roll card — shareable result card (category icon + star pips, no photo/title) for posting outside the app. (source: README.md)
- Taste correlation insights — e.g. "you rate meals higher on weekends" — the concrete payoff intended to make solo/no-friends mode viable from day one. (source: README.md)
- RollGroups — invite-only sub-circles (book club, foodie group) running their own private wheel and trigger window. (source: README.md)
- Behind-the-Scenes proof clip — optional few-second clip before the still, viewable via long-press; partial answer to staged-photo detection, though the camera-roll decision already loosens this goal. (source: README.md)
- Friends-of-Friends discovery — opt-in, same-gate-applies expansion. (source: README.md)
- Four Favorites pin — four pinned all-time favorite rolls at the top of the profile, separate from the chronological diary. (source: README.md)
- Streak restore — limited, conditional recovery for a broken streak. (source: README.md)
- Curated starter category packs — one-tap "add all" theme bundles inside the wheel editor, unlocked free via streak milestones (not sold as IAP). (source: README.md)

## Topic: Prior-Art Research — Verdict & Whitespace

- No exact match found for the full combination (roll-assigned category -> mandatory personal photo -> universal 4-field entry -> chronological diary -> BeReal-style reciprocity gate). Treated as weak news, not a green light: individual pieces, and most pairs of pieces, already exist in shipped products. Real but assemblable whitespace, not a defensible moat. (source: RESEARCH.md)
- Naming collision flagged: an app called "Roll — Fresh film daily" already ships in the daily-authentic-photo space. Different app, but relevant if the working name sticks. (source: RESEARCH.md)
- Whitespace, precisely stated: a roll that produces a durable, rated, photographed diary artifact — the intersection where the roll feeds the log — is unoccupied. Randomizer/wheel apps pick-and-forget; rate-and-log apps always have a user-chosen category. Nothing sits at the intersection. (source: RESEARCH.md)
- Secondary whitespace: reciprocity gating applied to a rated taste diary rather than a moment-photo — no found precedent gates a rated, categorized diary entry behind having posted your own that day. (source: RESEARCH.md)

## Topic: Prior-Art Research — Closest Matches

- Memorizer/Mio — closest entry-schema match (photo + written opinion + rating + category tag across 8 categories, 70k MAU). Differs: photos optional/auto-pulled via image recognition, no roll (user picks), no reciprocity gate. (source: RESEARCH.md)
- pov (Daily Photo Challenge) — matches the BeReal half almost exactly. Differs: no rating, no titled items/categories, no diary. (source: RESEARCH.md)
- PI.FYI — cross-category taste logging, friends feed, streak-based habit loop. Differs: no roll, no self-taken-photo rule, no fixed 4-field schema, no reciprocity gate. (source: RESEARCH.md)
- Rated (Share & Review Content) — broadest category coverage, friend-similarity scores. Differs: no daily roll, standard posters/covers not personal photos, no reciprocity gate. (source: RESEARCH.md)
- Quick Memo — entry format nearly matches the 4 fields. Differs: no daily roll, no social/friend feed, photo optional/any-source. (source: RESEARCH.md)
- Jotly — closest precedent for the core authenticity idea (photograph literally anything, rate it). Differs: no randomizer, letter grade not stars, no friend feed/gate, niche/dormant. (source: RESEARCH.md)
- The Dice Challenge — closest direct analog to roll-assigns-a-category, including purchasable category packs. Differs: rolls produce activities/dares not photographed/rated/diary-logged entities. (source: RESEARCH.md)
- Beli — strongest real-world proof mandatory personal photo + competitive friend feed works daily, for food. Differs: single category, no roll, comparative bracket-ranking not stars, feed is open (no gate). (source: RESEARCH.md)
- Crumble — reciprocity-adjacent gate (mutual-accept-only friending, no public feed). Differs: single category (food), no roll, gate is on friending not on viewing-after-posting. (source: RESEARCH.md)
- Tastral — the "default" non-authentic version of the diary half; all cover art auto-pulled from TMDB/IGDB/Google Books/Spotify. (source: RESEARCH.md)
- BeReal — source mechanic for the daily-surprise-prompt and post-before-you-peek reciprocity gate; still active in 2026 under Voodoo. No categories/ratings/diary; always satisfiable (unlike a rolled category). (source: RESEARCH.md)
- Letterboxd — source mechanic for the 1-5 star rating, diary, one-line-reaction pattern. Film-only, official poster art, no daily prompt/roll, friend feed explicitly ungated. (source: RESEARCH.md)

## Topic: Prior-Art Research — Risks

- Cold-start / no network effect — a reciprocity-gated friend feed is worthless at low friend density (BeReal's own Friends-of-Friends fix; the Artifact failure mode). (source: RESEARCH.md)
- Entry friction is stacked directly against the gate that rewards it — Daylio's ~2-tap, under-30-second, photo-optional loop is the 4.8-star comparison point; this concept's mandatory photo+title+rating is heavier, and friction directly suppresses the action that unlocks the feed. (source: RESEARCH.md)
- Authenticity-by-constraint as the whole value proposition is a proven failure mode, not just a validation signal — Poparazzi hit #1 in 2021 and shut down in 2023. (source: RESEARCH.md)
- The self-photo rule has no clear enforcement mechanism as currently scoped, and this is a bigger open question given the decision to allow camera-roll uploads (cross-referenced to README.md). (source: RESEARCH.md)
- Solo mode is unproven for this shape of product — Lapse's 2025 removal of its social layer caused documented churn; Daylio's solo success rests on a personal-insight retention driver this concept doesn't yet have designed (cross-referenced to README.md's "Taste correlation insights"). (source: RESEARCH.md)
- Scarcity-vs-flexibility tradeoff — Wordle's one-puzzle/day-no-archive design vs. the risk of diluting value via a paywalled archive; evidence leans toward strict daily scarcity over "once a day or on demand." (source: RESEARCH.md)
- Structural risk at the center of the novelty claim: a rolled category isn't always satisfiable (e.g. wheel says "book," nothing was read that day). Resolutions considered: skip (streak breaks for bad luck), fake it (kills authenticity rule), free re-roll (collapses the differentiator). Still unresolved. (source: RESEARCH.md; also listed as an open question in README.md and docs/original-concept-notes.md)

## Topic: Open Questions

- Exact wheel size and default starter categories. (source: README.md; source: docs/original-concept-notes.md — "how big the category wheel is")
- How weighting/reweighting works in the UI (drag-to-resize wedges vs. numeric sliders). (source: README.md)
- Whether categories can be customized per user or per friend group. (source: docs/original-concept-notes.md)
- The core structural risk: rolled category not always satisfiable — skip-breaks-streak vs. reroll-kills-the-differentiator still undecided. (source: README.md; source: RESEARCH.md)
- Tech stack — not yet chosen. (source: README.md; source: docs/original-concept-notes.md)
- Whether/how to signal "logged a day late from camera roll" vs. same-day, now that library uploads are allowed. (source: README.md)
- Whether a friend group is needed to feel complete from day one, or the product ships solo-first with insights as the retention hook. (source: README.md; source: docs/original-concept-notes.md)
- Whether friends see today's board only after posting their own entry (reciprocity gate) or can peek anytime. (source: docs/original-concept-notes.md — refined in README.md's feature backlog into a proposed "soft gate conditional on friend-circle size," not yet a closed decision)
- Naming — final name not yet chosen; "his call, not listed here" per original notes, "RNDMRoll" used only as working title in README. (source: docs/original-concept-notes.md; source: README.md)
