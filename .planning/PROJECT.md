# RNDMRoll

## What This Is

A daily habit-loop mobile app: each day a spin wheel assigns you a category (movie, book, song, meal, place, etc.), you log a photo + title + star rating + optional one-line reaction against it, building a personal chronological taste diary, and unlocking a gated friend feed showing what your circle rolled and rated that day. Combines BeReal's daily-ritual hook, Letterboxd's rating/log structure, and a randomizer for variety — not a clone of any single precedent. "RNDMRoll" is a working title; the final consumer-facing app name is still TBD.

## Core Value

A working daily habit loop — roll, log, diary, gated friend feed — used reliably every day by the developer and a small friend group. This milestone explicitly does NOT target public launch or outside-user growth numbers; that is deferred.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Users can create accounts, log in, and manage a friend circle
- [ ] Users receive one randomized category per day via a customizable spin wheel, on a strict once-daily cadence (with an optional weekly streak-earned reroll)
- [ ] Users log a fixed-structure daily entry (photo + title + half-star rating + optional reaction) against the rolled category
- [ ] Users build a personal chronological diary of everything they've rated
- [ ] Users see a friend feed gated behind posting their own entry for the day, with lightweight one-tap emoji reactions

See REQUIREMENTS.md for the full checkable list (17 v1 requirements across 5 phases).

### Out of Scope

- Public launch / outside-user growth for this milestone — explicit user framing; success is measured by daily reliability of use within a small friend group, not adoption numbers
- Strict technical enforcement of photo authenticity (e.g., detecting stock/official art) — accepted open tradeoff of allowing camera-roll uploads, not solved yet
- Dice-roll mechanic — superseded by spin-wheel (see Key Decisions)
- On-demand/unlimited re-rolling — superseded by strict once-daily cadence with optional streak-earned token (see Key Decisions)
- Camera-only/live-capture-only photo requirement — superseded by camera + photo library choice (see Key Decisions)
- Final consumer-facing app naming — still TBD, not a build task for this milestone
- v2 feature backlog (Year in Rolls recap, taste correlation insights, spoiler-free share card, RollGroups, friends-of-friends discovery, four-favorites profile pin, streak restore, behind-the-scenes proof clip, curated starter wheel packs) — see REQUIREMENTS.md v2 section

## Context

- Prior-art research (RESEARCH.md) found no exact match for the full combination (roll-assigned category → mandatory personal photo → universal 4-field entry → chronological diary → reciprocity-gated friend feed); treated as assemblable whitespace, not a defensible moat — individual pieces, and most pairs of pieces, already exist in shipped products (Memorizer/Mio, pov, PI.FYI, Rated, Quick Memo, Jotly, The Dice Challenge, Beli, Crumble, Tastral, BeReal, Letterboxd).
- Naming collision flagged by research: an app called "Roll — Fresh film daily" already ships in the daily-authentic-photo space. Different app, but relevant if the "RNDMRoll" working name or a close variant sticks as the final name.
- Known risks from research, carried forward as things to watch rather than blockers: cold-start/no-network-effect at low friend density; entry friction (photo+title+rating+reaction) is heavier than comparison points like Daylio and is stacked directly against the gate that rewards it; "authenticity-by-constraint" as the entire value proposition is a proven failure mode (Poparazzi: #1 in 2021, shut down 2023); solo mode is unproven for this product shape (this milestone sidesteps that specific risk by requiring a real friend group from day one, per the Core Value framing above).
- Structural risk, currently unresolved: a rolled category isn't always satisfiable (e.g., wheel says "book," nothing was read that day). Candidate resolutions — skip (breaks streak), fake it (breaks the authenticity premise), free reroll (collapses the once-daily differentiator) — are each real tradeoffs. This is tracked as an explicit open decision on **ENTRY-04** (REQUIREMENTS.md, Phase 3: Daily Entry), to be closed out during Phase 3 discussion, not pre-decided here.
- Related unresolved detail: exact daily roll-window and day-boundary semantics (opening/availability time around 8:00 PM, window duration, timezone handling, when a "day" turns over for streak/feed purposes). Tracked within **ROLL-01** (REQUIREMENTS.md, Phase 2: Daily Roll), to be closed out during Phase 2 discussion. FEED-02's "late" badge logic (Phase 5: Friend Feed) inherits this definition.
- Other open questions carried from intel, not yet decided: default wheel size/starter categories; exact reweighting UI (drag-to-resize wedges vs. numeric sliders); whether wheel categories can be customized per-friend-group as well as per-user.
- Ingest conflicts: this project's intel set contained three DOC-type sources (README.md, RESEARCH.md, docs/original-concept-notes.md) with no formal ADRs. Three competing-variant conflicts (roll mechanic, roll cadence, photo-source rule) were resolved by explicit user approval in favor of README.md's later, decision-bearing framing over docs/original-concept-notes.md's earlier framing. Full detail: `.planning/INGEST-CONFLICTS.md`.

## Constraints

- **Tech stack — backend**: Go (REST/JSON API, e.g. Gin/Echo/Fiber), Postgres, S3-compatible object storage for photos — decided in conversation, recorded in README.md "Decisions made so far" table
- **Tech stack — mobile**: React Native with Expo managed workflow (expo-camera, expo-image-picker, expo-image), EAS Build/Update for releases — decided in conversation
- **Design (standing, applies to all UI/copy work)**: no purple gradients, no pill-shaped buttons, no fake reviews/testimonials, no fake customer counters/metrics, no vague hero text, no emoji-as-icons, no em dashes, no over-the-top scroll animations, no AI-slop photos, no AI-slop copy, no cursor animations
- **Design (wheel-spin screen specifically)**: flat/muted, typography-led, restrained-motion treatment — no neon colors, no glossy 3D pointer, no confetti/flash-on-land, no clipart icons on wedges (avoid prize-wheel/gambling aesthetics)
- **Launch gate** (applies whenever a future milestone targets public shippability — not required for this milestone): custom domain connected, favicon added, any "Made with AI"/builder-attribution tag removed. Do not mark any milestone shippable/launchable until met.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Roll mechanic: spin wheel, not dice | Wedge sizing maps naturally to reweighting; deceleration/anticipation moment more shareable than a flat die number; a die caps out awkwardly around 6 faces | — Pending |
| Roll cadence: strictly once/day at a fixed time (8:00 PM), no on-demand re-roll; optional streak-earned extra reroll token per week | Preserves scarcity/non-addictiveness (Wordle precedent: one puzzle/day, no archive); evidence favors strict daily scarcity over "once a day or on demand" | — Pending |
| Photo source: camera capture AND photo library upload both allowed | Forcing live-only capture fights logging something consumed hours/days earlier; accepted tradeoff — reopens the authenticity-enforcement question, which stays unsolved | — Pending |
| Star rating: 1-5 in half-star increments, Letterboxd-style | Integer stars too coarse; Beli-style comparative ranking doesn't work across disparate categories (movie vs. meal vs. book) | — Pending |
| Backend stack: Go + Postgres + S3-compatible storage | Decided in conversation, recorded in README.md | — Pending |
| Mobile stack: React Native + Expo managed workflow + EAS Build/Update | Decided in conversation, recorded in README.md | — Pending |
| Milestone 1 success metric: daily reliable use by developer + small friend group, not public launch | Explicit user framing — defers growth/launch concerns to a later milestone | — Pending |

---
*Last updated: 2026-09-15 after ingest synthesis and roadmap creation*
