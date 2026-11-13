# Requirements: RNDMRoll

**Defined:** 2026-09-15
**Core Value:** A working daily habit loop — roll, log, diary, gated friend feed — used reliably every day by the developer and a small friend group.

> **Status (2026-11-13).** PRD.md is the current record of every requirement, under newer IDs and with today's status. This file keeps the first IDs from September, and their statuses below are brought up to date.

## v1 Requirements

### Accounts & Profile

- [x] **ACCT-01**: User can create an account and log in, and stays logged in across app sessions
- [x] **ACCT-02**: User can add friends (e.g., via invite/username) to form their friend circle
- [x] **ACCT-03**: User can view their own profile

### Settings (inserted 2026-10-10, Phase 2.1)

- [x] **SET-01**: User can change their username from Settings (built on the Edit page, PRD PROF-02)
- [x] **SET-02**: User can change their password from Settings (other phones are signed out)
- [ ] ~~**SET-03**: User can make their account public or private (default public); the choice is stored and shown, and read by the Friends view and Global in Phases 5 and 6~~ (replaced by PRD SOC-01)
- [x] **SET-04**: User can edit their bio in place on the profile page (built on the Edit page, PRD PROF-02)

### Roll & Wheel

- [x] **ROLL-01**: User can spin a wheel once per day within a fixed daily window opening at 8:00 PM in the user's own local timezone (not one globally-synced moment), and cannot re-spin until the next window opens unless a reroll token is available. Day boundaries and streaks follow each user's local calendar day. The window stays open until local midnight. A post is on time until noon the next day; from noon until the next 8:00 PM it is late (the feed still unlocks, with a late badge). After that the day is missed and does not count toward the weekly goal.
- [x] **ROLL-02**: User can earn one bonus reroll each week the weekly goal is met (an entry posted on at least 4 days, Monday to Sunday), keep one at a time, and spend it to re-spin within the same day
- [x] **ROLL-03**: User can customize their wheel by adding, removing, and reweighting categories (bigger wedge = higher probability), starting from a small default set (6-8 categories)
- [x] **ROLL-04**: Wheel spin screen shows every category wedge with a deceleration/landing moment, in a flat/muted, typography-led, restrained-motion style (no prize-wheel/gambling aesthetics: no neon colors, no glossy 3D pointer, no confetti/flash-on-land, no clipart wedge icons)

### Daily Entry

- [x] **ENTRY-01**: User can log today's entry with a photo (captured live or chosen from the photo library), a title, a 1-5 star rating in half-star increments, an optional one-tap reaction (one of six fixed reactions), and an optional comment (a short free-text thought), logged against the day's rolled category. The author can change or delete a post; a deleted post still counts as a posted day
- [x] **ENTRY-02**: User can log an entry for a title already logged before (e.g., a rewatch) without overwriting the prior entry; logging it again creates a new dated entry instead
- [x] **ENTRY-03**: User can see today's rolled category as the required category for today's entry before submitting
- [x] **ENTRY-04**: User can skip a day when the spun category isn't satisfiable (e.g., wheel says "book," nothing was read that day). No entry is required that day. The skip is visible in the diary (DIARY-01), not a silent gap, and shows its effect on the week (e.g., "2 of 4 this week"). A skip is distinct from a reroll (ROLL-02, which swaps the category and still requires posting). (Built as PRD SPIN-05: a skipped day shows as a dash on the week row; whether it also shows on the profile is an open question in PRD section 16.)

### Personal Diary

- [x] **DIARY-01**: User can view a running chronological log of every entry they've rated, forming a personal taste history (built as the Profile grid, PRD PROF-01)
- [ ] ~~**DIARY-02**: User can view their diary filtered by category~~ (removed 2026-10-16, PRD PROF-05)

### Friend Feed

- [x] **FEED-01**: User can view a shared feed of what friends in their circle rolled and rated that day, gated behind having posted their own entry that day
- [x] **FEED-02**: User who posts a late entry (per the daily window's day-boundary rules from ROLL-01) still unlocks the feed, marked with a visible "late" badge (soft gate, not a hard block)
- [x] **FEED-03**: User with no friends yet (or below the gate threshold) is routed to their personal diary instead of a locked/empty feed (changed: an empty Board offers Find people, PRD BRD-04)
- [ ] ~~**FEED-04**: User can react to a friend's feed entry with a one-tap emoji reaction (not photo-based)~~ (removed 2026-10-18, PRD BRD-06)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap; sourced from README.md's prior-art-research feature backlog.

### Insights & Recap

- **INSIGHT-01**: User can view an annual "Year in Rolls" recap (top categories, avg rating per category, longest streak, most-rated title)
- **INSIGHT-02**: User can view taste correlation insights (e.g., "you rate meals higher on weekends")

### Sharing

- **SHARE-01**: User can generate a spoiler-free roll card (category icon + star pips, no photo/title) to share outside the app

### Social Expansion

- **SOCIAL-01**: User can create/join invite-only RollGroups: sub-circles with their own private wheel and trigger window
- **SOCIAL-02**: User can discover friends-of-friends (opt-in, same gate rules apply)

### Profile Enhancements

- **PROFILE-01**: User can pin up to four all-time favorite rolls to their profile, separate from the chronological diary

### Retention & Trust

- **RETAIN-01**: User can restore a broken streak under limited, conditional conditions
- **TRUST-01**: User can attach an optional few-second "behind-the-scenes" proof clip to an entry, viewable via long-press
- **WHEEL-01**: User can add a curated starter category pack (themed bundle) to their wheel in one tap, unlocked free via streak milestones (not sold as IAP)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Public launch / outside-user growth | Explicit user framing for this milestone; success is daily reliable use within a small friend group, not adoption |
| Strict technical enforcement of photo authenticity (e.g. stock/official-art detection) | Accepted open tradeoff of allowing camera-roll uploads (see PROJECT.md Context); unsolved, not a v1 requirement |
| Dice-roll mechanic | Superseded by spin-wheel per resolved ingest conflict (see INGEST-CONFLICTS.md) |
| On-demand/unlimited re-rolling | Superseded by strict once-daily cadence per resolved ingest conflict |
| Camera-only/live-capture-only photo requirement | Superseded by camera + photo-library choice per resolved ingest conflict |
| Final consumer-facing app naming | Still TBD per user; not a build task for this milestone |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| ACCT-01 | Phase 1 | Complete |
| ACCT-03 | Phase 1 | Complete |
| ROLL-01 | Phase 2 | Complete |
| ROLL-02 | Phase 2 | Complete |
| ROLL-03 | Phase 2 | Complete (equal slices since 2026-10-10, PRD SPIN-13) |
| ROLL-04 | Phase 2 | Complete |
| SET-01 | Phase 2.1 | Complete, on the Edit page (PRD PROF-02) |
| SET-02 | Phase 2.1 | Complete (PRD SES-05) |
| SET-03 | Phase 2.1 | Replaced by PRD SOC-01 |
| SET-04 | Phase 2.1 | Complete, on the Edit page (PRD PROF-02) |
| ENTRY-01 | Phase 3 | Complete (PRD POST-02 to POST-05) |
| ENTRY-02 | Phase 3 | Complete (PRD POST-08) |
| ENTRY-03 | Phase 3 | Complete (PRD POST-01) |
| ENTRY-04 | Phase 3 | Complete (PRD SPIN-05); skipped days on the profile are an open question (PRD section 16) |
| DIARY-01 | Phase 4 | Complete, as the Profile grid (PRD PROF-01) |
| DIARY-02 | Phase 4 | Removed 2026-10-16 (PRD PROF-05) |
| ACCT-02 | Phase 5 | Complete: one-way follows by request, invite links and suggestions (PRD SOC-01 to SOC-05) |
| FEED-01 | Phase 5 | Complete, the Board (PRD BRD-01, BRD-02) |
| FEED-02 | Phase 5 | Complete (PRD SPIN-06) |
| FEED-03 | Phase 5 | Changed: an empty Board offers Find people (PRD BRD-04) |
| FEED-04 | Phase 5 | Removed 2026-10-18 (PRD BRD-06) |

**Coverage:**

- v1 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-15*
*Last updated: 2026-09-16 — ENTRY-01 split into a distinct optional reaction field and optional comment field, per user decision (see PROJECT.md Key Decisions). ROLL-01 and ENTRY-04 resolved 2026-09-15 via discussion (see PROJECT.md Key Decisions).*
