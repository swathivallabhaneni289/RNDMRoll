# Requirements: RNDMRoll

**Defined:** 2026-09-15
**Core Value:** A working daily habit loop — roll, log, diary, gated friend feed — used reliably every day by the developer and a small friend group.

## v1 Requirements

### Accounts & Profile

- [ ] **ACCT-01**: User can create an account and log in, and stays logged in across app sessions
- [ ] **ACCT-02**: User can add friends (e.g., via invite/username) to form their friend circle
- [ ] **ACCT-03**: User can view their own profile

### Roll & Wheel

- [ ] **ROLL-01**: User can spin a wheel once per day within a fixed daily window opening around 8:00 PM, and cannot re-spin until the next window opens unless a reroll token is available. Window duration, timezone handling, and day-boundary rules (which also govern FEED-02's "late" badge) are an open decision to close out during Phase 2 discussion, not yet locked.
- [ ] **ROLL-02**: User can earn one extra reroll token per week via streak milestones, and can spend a token to re-spin within the same day
- [ ] **ROLL-03**: User can customize their wheel by adding, removing, and reweighting categories (bigger wedge = higher probability), starting from a small default set (6-8 categories)
- [ ] **ROLL-04**: Wheel spin screen shows every category wedge with a deceleration/landing moment, in a flat/muted, typography-led, restrained-motion style (no prize-wheel/gambling aesthetics: no neon colors, no glossy 3D pointer, no confetti/flash-on-land, no clipart wedge icons)

### Daily Entry

- [ ] **ENTRY-01**: User can log today's entry with a photo (captured live or chosen from the photo library), a title, a 1-5 star rating in half-star increments, and an optional one-line reaction, logged against the day's rolled category
- [ ] **ENTRY-02**: User can log an entry for a title already logged before (e.g., a rewatch) without overwriting the prior entry; logging it again creates a new dated entry instead
- [ ] **ENTRY-03**: User can see today's rolled category as the required category for today's entry before submitting
- [ ] **ENTRY-04**: User can resolve a day when the rolled category isn't satisfiable (e.g., wheel says "book," nothing was read that day), and sees the streak consequence of that resolution when confirming it. The exact resolution mechanic (skip and break the streak, allow a reroll, or another approach) is an open decision to close out during Phase 3 discussion, not yet locked.

### Personal Diary

- [ ] **DIARY-01**: User can view a running chronological log of every entry they've rated, forming a personal taste history
- [ ] **DIARY-02**: User can view their diary filtered by category

### Friend Feed

- [ ] **FEED-01**: User can view a shared feed of what friends in their circle rolled and rated that day, gated behind having posted their own entry that day
- [ ] **FEED-02**: User who posts a late entry (per the daily window's day-boundary rules from ROLL-01) still unlocks the feed, marked with a visible "late" badge (soft gate, not a hard block)
- [ ] **FEED-03**: User with no friends yet (or below the gate threshold) is routed to their personal diary instead of a locked/empty feed
- [ ] **FEED-04**: User can react to a friend's feed entry with a one-tap emoji reaction (not photo-based)

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
| ACCT-01 | Phase 1 | Pending |
| ACCT-03 | Phase 1 | Pending |
| ROLL-01 | Phase 2 | Pending |
| ROLL-02 | Phase 2 | Pending |
| ROLL-03 | Phase 2 | Pending |
| ROLL-04 | Phase 2 | Pending |
| ENTRY-01 | Phase 3 | Pending |
| ENTRY-02 | Phase 3 | Pending |
| ENTRY-03 | Phase 3 | Pending |
| ENTRY-04 | Phase 3 | Pending |
| DIARY-01 | Phase 4 | Pending |
| DIARY-02 | Phase 4 | Pending |
| ACCT-02 | Phase 5 | Pending |
| FEED-01 | Phase 5 | Pending |
| FEED-02 | Phase 5 | Pending |
| FEED-03 | Phase 5 | Pending |
| FEED-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-15*
*Last updated: 2026-09-15 after roadmap creation*
