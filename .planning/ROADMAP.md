# Roadmap: RNDMRoll

## Overview

RNDMRoll delivers a daily habit-loop mobile app in five phases: first the account/profile foundation, then the once-daily customizable spin-wheel roll, then the structured photo+title+rating+reaction entry, then the personal diary that entry feeds, and finally the friend circle and reciprocity-gated feed that closes the loop. By the end of Phase 5, the full loop — roll, log, diary, gated friend feed — is usable daily by the developer and a small friend group, matching this milestone's success metric.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation & Accounts** - Users can create accounts, log in, and view their profile
- [ ] **Phase 2: Daily Roll** - Users get a once-daily category from a customizable, restrained-motion spin wheel
- [ ] **Phase 3: Daily Entry** - Users log a structured daily entry against their rolled category
- [ ] **Phase 4: Personal Diary** - Users browse a chronological, filterable log of everything they've rated
- [ ] **Phase 5: Friend Feed** - Users build a friend circle and see a reciprocity-gated daily feed of what friends rolled and rated

## Phase Details

### Phase 1: Foundation & Accounts
**Goal**: Users can create an account, log in, and see their own profile: the foundation the rest of the app builds on
**Depends on**: Nothing (first phase)
**Requirements**: ACCT-01, ACCT-03
**Success Criteria** (what must be TRUE):
  1. User can create an account and log in, staying logged in across app sessions
  2. User can view their own profile
**Plans**: TBD
**UI hint**: yes

### Phase 2: Daily Roll
**Goal**: Users receive a randomized, once-daily category via a customizable spin-wheel mechanic, with a restrained, non-gambling visual treatment
**Depends on**: Phase 1
**Requirements**: ROLL-01, ROLL-02, ROLL-03, ROLL-04
**Success Criteria** (what must be TRUE):
  1. User can spin a wheel once per day within a fixed daily window (opening around 8:00 PM) and receive a category
  2. User cannot spin again until the next day's window opens, unless they have a reroll token
  3. User can earn and spend one weekly streak-based reroll token
  4. User can customize their wheel's categories and relative weights
  5. Wheel spin screen uses a flat, muted, restrained-motion visual treatment (no prize-wheel/gambling aesthetic)
**Plans**: TBD
**UI hint**: yes

**Note**: ROLL-01's exact window duration, timezone handling, and day-boundary rules are an open decision, not yet locked; resolve during Phase 2 discussion. Phase 5's FEED-02 "late" badge logic depends on this definition.

### Phase 3: Daily Entry
**Goal**: Users log a structured daily entry against their rolled category
**Depends on**: Phase 2
**Requirements**: ENTRY-01, ENTRY-02, ENTRY-03, ENTRY-04
**Success Criteria** (what must be TRUE):
  1. User can see today's rolled category before logging an entry
  2. User can log an entry with a photo (camera or library), title, half-star rating, and optional reaction
  3. Logging a repeat title creates a new dated entry rather than overwriting the prior one
  4. User can resolve a day when the rolled category isn't satisfiable and sees the streak consequence of that resolution
**Plans**: TBD
**UI hint**: yes

**Note**: ENTRY-04's exact resolution mechanic (skip-and-break-streak, reroll, or another approach) is an open decision, not yet locked; resolve during Phase 3 discussion. It trades off streak fairness against the once-daily differentiator and the authenticity premise.

### Phase 4: Personal Diary
**Goal**: Users can browse the chronological taste diary that their daily entries build up
**Depends on**: Phase 3
**Requirements**: DIARY-01, DIARY-02
**Success Criteria** (what must be TRUE):
  1. User can view a chronological diary of all past entries
  2. User can filter their diary by category
**Plans**: TBD
**UI hint**: yes

### Phase 5: Friend Feed
**Goal**: Users build a friend circle and see what their friends rolled and rated each day, gated behind posting their own entry
**Depends on**: Phase 1, Phase 3
**Requirements**: ACCT-02, FEED-01, FEED-02, FEED-03, FEED-04
**Success Criteria** (what must be TRUE):
  1. User can add friends to form their friend circle
  2. User who has posted today's entry can view friends' entries for the day
  3. User who posts late still unlocks the feed with a "late" badge instead of being blocked
  4. User with no friends yet (or below the gate threshold) sees their diary instead of an empty/locked feed
  5. User can react to a friend's entry with a one-tap emoji
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & Accounts | 0/TBD | Not started | - |
| 2. Daily Roll | 0/TBD | Not started | - |
| 3. Daily Entry | 0/TBD | Not started | - |
| 4. Personal Diary | 0/TBD | Not started | - |
| 5. Friend Feed | 0/TBD | Not started | - |
