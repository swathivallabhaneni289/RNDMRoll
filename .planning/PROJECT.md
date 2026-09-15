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
- Structural risk, now resolved: a rolled category isn't always satisfiable (e.g., wheel says "book," nothing was read that day). Resolved via a capped streak-freeze (no entry required, streak preserved), distinct from the separate ROLL-02 reroll token (swap category, still post). Tracked as **ENTRY-04** (REQUIREMENTS.md, Phase 3: Daily Entry); exact freeze cap is a minor tuning parameter left for Phase 3 planning.
- Related detail, now resolved: daily roll timing is each user's own local 8:00 PM, not one globally-synced moment — day boundaries/streaks follow each user's local calendar day. Tracked within **ROLL-01** (REQUIREMENTS.md, Phase 2: Daily Roll); exact window/late-badge grace-period cutoff (feeding FEED-02's "late" badge, Phase 5) is a minor tuning parameter left for Phase 2 planning.
- Other open questions carried from intel, not yet decided: default wheel size/starter categories; exact reweighting UI (drag-to-resize wedges vs. numeric sliders); whether wheel categories can be customized per-friend-group as well as per-user.
- Ingest conflicts: this project's intel set contained three DOC-type sources (README.md, RESEARCH.md, docs/original-concept-notes.md) with no formal ADRs. Three competing-variant conflicts (roll mechanic, roll cadence, photo-source rule) were resolved by explicit user approval in favor of README.md's later, decision-bearing framing over docs/original-concept-notes.md's earlier framing. Full detail: `.planning/INGEST-CONFLICTS.md`.
- **Moodboard reference (2026-09-15):** user provided two reference mockup boards for "rndmroll" establishing overall visual direction. Extracted as project-wide reference, not literally copied (user's explicit framing: "similar vibe... not the exact same thing... make sure it's not really too much ai coded" — the board's own bottom design-system panel listed Playfair Display + Inter as its typefaces, which are two of the most recognizable AI-generated-UI default fonts, so exact fonts/colors were deliberately not copied verbatim):
  - **Theme system, whole-app — superseded 2026-09-15:** originally split dark/cinematic for "moment" screens vs. light for "utility" screens, mirroring the moodboard's own light/dark pattern. That framing is retired: Phase 1 (accounts/profile, a utility phase) was revised to dark-primary per explicit user feedback ("black background and uses a white as a text instead maybe"), after several earlier rounds already pushing toward black — the accumulated signal is a dark-primary app overall, not a light/dark split by screen type. Current direction: **dark-primary throughout** — near-black backgrounds, near-white text/accent, strict grayscale (no chromatic brand color), small muted functional colors only for destructive/success states. Phase 2/3's moment screens (roll, reveal, camera, entry) remain the most cinematic/photo-forward expression of this same dark identity, not a separate light-vs-dark track. Phase 5 (diary, friend board, wheel settings) should default to this same dark-primary treatment when designed, rather than assuming light-for-utility — revisit explicitly with the user if that still seems right at that point, don't just inherit the old split silently.
  - **Bottom navigation pattern:** 5 tabs (home, add/log, roll — center, circular, visually distinct — board, profile), for whichever phase first introduces app-wide navigation chrome (likely Phase 5, once the friend board exists).
  - **Category icon set:** movie (clapperboard), book, song (music note), meal (fork & knife), place (map pin), game (game controller) — reference for Phase 2's wheel-category iconography.
  - **Wheel visual pattern:** pie-slice wedges each with icon + label, solid circular center button for "SPIN" — reference for Phase 2's wheel design (still must honor PROJECT.md's wheel-specific constraint below: flat/muted, no neon, no glossy 3D pointer).
  - **Launch/splash pattern:** centered app-mark icon (a simple rounded-square glyph, e.g. dice dots) + wordmark + one-line tagline — reference for a future launch-screen treatment.
  - Not adopted: the moodboard's literal typefaces (Playfair Display + Inter) and its exact hex palette — Phase 1 instead uses Domine + Work Sans (same elegant-serif-headline-plus-clean-body mood) and a contrast-verified strict-grayscale palette, both chosen specifically to avoid the AI-generated-UI look the user repeatedly flagged as the top priority to avoid.

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
| Roll timing: each user's own local 8:00 PM, not one globally-synced moment | Avoids a multi-timezone friend group spinning at odd local hours just to stay in sync BeReal-style; day boundaries/streaks follow each user's local calendar day | — Pending |
| Theme system: dark-primary throughout (superseded the earlier light-utility/dark-moment split) | User pushed toward black across several UI-SPEC revisions, culminating in an explicit ask for Phase 1 (a utility screen) to go dark too — the real preference is dark-primary overall, not split by screen type | — Pending |
| Typography: Domine (serif headings) + Work Sans (body/UI), not the moodboard's literal Playfair Display + Inter | User's explicit framing ("similar vibe, not the exact same thing... not too ai coded") — Playfair Display + Inter are two of the most recognizable AI-generated-UI default fonts; kept the same elegant-serif-plus-clean-sans mood without that specific baggage | — Pending |
| Unsatisfiable-category resolution: capped streak freeze (no entry required, streak preserved) | Chosen over "skip, streak breaks" (punishes bad luck); coexists with the separate ROLL-02 reroll token; capped (not unlimited) to preserve daily scarcity | — Pending |

---
*Last updated: 2026-09-15 after ingest synthesis and roadmap creation; roll-timing and unsatisfiable-category decisions resolved same day*
