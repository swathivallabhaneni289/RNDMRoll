# Roadmap: RNDMRoll

## Overview

RNDMRoll delivers a daily habit-loop mobile app in five phases: first the account/profile foundation, then the once-daily customizable spin-wheel roll, then the structured photo+title+rating+reaction entry, then the personal diary that entry feeds, and finally the friend circle and reciprocity-gated feed that closes the loop. By the end of Phase 5, the full loop — roll, log, diary, gated friend feed — is usable daily by the developer and a small friend group, matching this milestone's success metric.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation & Accounts** - Users can create accounts, log in, and view their profile (completed 2026-10-09)
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

**Plans**: 21/21 plans executed (01-18 was replaced by 01-19 and never built)

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Go module scaffold, config loader, and external service provisioning
- [x] 01-02-PLAN.md — Expo app scaffold, platform light lock, and UI-SPEC design tokens

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-03-PLAN.md — Postgres schema, user domain model, and pgx repositories
- [x] 01-04-PLAN.md — Brand mark, dot-grid texture, and the twelve UI primitives
- [x] 01-05-PLAN.md — SecureStore session store, API client, and auth-gated routing

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-06-PLAN.md — Auth primitives, rate limiting, and shared HTTP plumbing
- [x] 01-07-PLAN.md — Onboarding screens: choose method, email auth, verify email
- [x] 01-16-PLAN.md — Pre-signup marketing screens: welcome cover and the three numbered pitch screens

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-08-PLAN.md — Password auth endpoints: signup, login, refresh, logout
- [x] 01-09-PLAN.md — Email verification flow, mailer, and the verified-only gate
- [x] 01-10-PLAN.md — Apple and Google sign-in with server-side token verification (removed 2026-10-09)
- [x] 01-11-PLAN.md — Profile, username suggestion, and avatar upload endpoints

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-12-PLAN.md — Consolidated create-profile screen: avatar, name, username, bio
- [x] 01-13-PLAN.md — API server wiring and end-to-end integration tests
- [x] 01-14-PLAN.md — Profile view and edit screens

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01-15-PLAN.md — Phase verification: automated sweep and device UAT

**Added during the device walkthrough (2026-10-01 to 2026-10-09)**

- [x] 01-17-PLAN.md — Defects found in the static walkthrough review
- [ ] ~~01-18-PLAN.md~~ — Code-flow sign-up (superseded by 01-19, never built)
- [x] 01-19-PLAN.md — One-request sign-up with a birthday (server) and the shared profile form
- [x] 01-20-PLAN.md — Sign-up in two pages: account details, then photo and bio once
- [x] 01-21-PLAN.md — Birthday date scroller and green "Looks good." notes
- [x] 01-22-PLAN.md — Log in with an email or a username; text boxes that never cut off what you type

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

**Plans**: 0/3 plans executed

Plans:
**Wave 1**

- [ ] 02-01-PLAN.md: The spin server: wheel, time zone, one spin a day, weekly bonus reroll, and test-only clock tools
- [ ] 02-02-PLAN.md: The spin app: Today screen, wheel, countdown, weekly card, Edit wheel, and the bottom bar

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 02-03-PLAN.md: Cutover, gates, device walkthrough, and phase close-out

**UI hint**: yes

**Resolved**: Roll timing is each user's own local 8:00 PM, not one globally-synced moment — day boundaries and streaks follow each user's local calendar day. The window stays open until local midnight. A post after midnight and before the next 8:00 PM is late (the feed still unlocks, with a late badge). After that the day is missed and does not count toward the weekly goal.

### Phase 3: Daily Entry

**Goal**: Users log a structured daily entry against their rolled category
**Depends on**: Phase 2
**Requirements**: ENTRY-01, ENTRY-02, ENTRY-03, ENTRY-04
**Success Criteria** (what must be TRUE):

  1. User can see today's rolled category before logging an entry
  2. User can log an entry with a photo (camera or library), title, half-star rating, and an optional reaction (one of six)
  3. Logging a repeat title creates a new dated entry rather than overwriting the prior one
  4. User can skip a day when the rolled category isn't satisfiable and sees how it changes the week count (for example, 2 of 4 this week)
  5. User can change or delete their post afterwards, and a deleted post still counts as a posted day

**Plans**: 0/3 plans executed

Plans:
**Wave 1**

- [ ] 03-01-PLAN.md: The entry server: entries table, photo upload ticket, post, edit, delete and skip, and the Today changes
- [ ] 03-02-PLAN.md: The entry app: Log it, the entry screen with half stars and six reactions, and the Posted state

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 03-03-PLAN.md: The entry page (view, edit, delete), skip, the skip mark, cutover, gates, device walkthrough and phase close-out

**UI hint**: yes

**Resolved**: Unsatisfiable-category days use a free skip: no entry required, visible in the diary, and it shows its effect on the weekly goal count. Distinct from the ROLL-02 reroll.

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
| 1. Foundation & Accounts | 21/21 | Complete | 2026-10-09 |
| 2. Daily Roll | 0/TBD | Not started | - |
| 3. Daily Entry | 0/3 | Not started | - |
| 4. Personal Diary | 0/TBD | Not started | - |
| 5. Friend Feed | 0/TBD | Not started | - |
