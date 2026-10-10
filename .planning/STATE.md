---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 02
current_phase_name: daily-roll
status: planning
stopped_at: Apple and Google sign-in removed completely 2026-10-09 (pushed to GitHub on 2026-10-10 at the developer's ask); Phase 2 documents renumbered and waiting for the developer's go; Phase 3 documents written on paper (nothing built)
last_updated: "2026-10-09T18:21:00.000Z"
last_activity: 2026-10-09
last_activity_desc: Apple and Google sign-in removed; Phase 3 planned on paper
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 21
  completed_plans: 21
  percent: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-15)

**Core value:** A working daily habit loop — roll, log, diary, gated friend feed — used reliably every day by the developer and a small friend group.
**Current focus:** Phase 02 — daily-roll (planned on paper, waiting for go)

## Current Position

Phase: 02 (daily-roll) — PLANNED ON PAPER (Phase 01 closed 2026-10-09); 03 (daily-entry) — PLANNED ON PAPER (2026-10-09)
Plan: 0 of 3
Status: Waiting for the developer's go on Phase 2
Last activity: 2026-10-09 — Apple and Google sign-in removed; Phase 3 planned on paper

Progress: [██░░░░░░░░] 20%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: none yet
- Trend: -

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Ingest: Roll mechanic resolved to spin-wheel (not dice), per user approval over docs/original-concept-notes.md
- Ingest: Roll cadence resolved to strictly once/day at 8:00 PM + optional weekly streak-earned reroll token (not on-demand)
- Ingest: Photo source resolved to camera capture AND photo library upload both allowed (not camera-only)
- [Phase 01]: Real Google and Apple sign-in and production photo storage setup are deferred to the end of the whole project; test-mode stand-ins meanwhile — Developer decision 2026-10-01: wants speed, account creation is their manual task. GitHub issue 2 holds the checklist.
- [Phase 05]: Friend feed look: Instagram-style posts as frosted glass cards over a soft blurred background, feed only — Developer decision 2026-10-01 from a reference screenshot. See docs/design-brief-2026-10-01-feed.md. Standing rules still apply.
- [Phase 01]: Defects from the static walkthrough review are fixed in plan 01-17 before the device walkthrough; branch stays lab/frosted-editorial (renamed phase-01-foundation-accounts on 2026-10-09) — Developer: do not skip anything. Isolated copy, review, apply at a boundary.
- [Phase 01]: Plan 01-18 (code-flow sign-up) is superseded by 01-19 (2026-10-07): one-page sign-up with a birthday (13+), no code, no email check; Apple and Google also enter a birthday (open decision a). Developer said start.
- [Phase 01]: Sign-up is two pages (2026-10-08): page 1 account details (email, password, birthday, name, username), page 2 photo and bio once with Skip for now; replaces the one-page form; plan 01-20, built the same day. Developer picked option a after asking how Instagram does it.
- [Phase 01]: Less typing on page 1 (2026-10-08): the birthday is picked on iOS's scrolling date picker (typed boxes off iOS) and a green "Looks good." note shows under checkable fields on the sign-up and Apple/Google finish pages; plan 01-21. Face ID, Next key, box hints, username choices, Apple/Google first: not chosen.
- [Phase 01]: Phase 1 closed 2026-10-09 (the developer typed approve). ONE password per account, typed once on sign-up and used with the email and the username (a confirmation box and then two separate passwords were built and removed the same evening); Log in takes an email or a username in one box (plan 01-22); the sign-up page order follows the Instagram photo (email, password, birthday, name, username); text boxes never clip what is typed. Launch list unchanged (domain, favicon, remove the AI tag, real Terms and Privacy, Forgot password, real Google, Apple, S3 and Resend accounts).
- [Phase 01]: Apple and Google sign-in removed completely (2026-10-09, developer: 'We're not going to have that'). Email and password only; Log in takes an email or a username. Server endpoints, verifiers, settings, repository methods, app buttons, finish mode, the two native packages and the database columns users.apple_subject and users.google_subject are gone (migration 0003). Phase 2's migration is 0004 and Phase 3's is 0005. The launch list no longer has real Google or Apple sign-in accounts.
- [Phase 03]: Entry rules (2026-10-09, all three suggestions taken): movie and book days may log anything watched or read lately (no date asked); an entry carries one tap-reaction from six emoji (heart, laughing, wow, fire, clap, yum), the same set friends use later; the author can edit and delete any time and a deleted post still counts the day as posted. Also (developer tap, same night): entry photos may be up to 50 MB, the same ceiling as profile photos (the old notes said 5 MB). Documents in .planning/phases/03-daily-entry, nothing built; their edits to shared documents (ENTRY-01 wording, ROADMAP plan list, PROJECT.md row, notes in the Phase 2 documents) are applied.
- [Phase 05]: Friend comments (2026-10-10, developer tap 'Yes, add friend comments'): friends can write short comments under a post, so Phase 5 gets a new item (to be written as FEED-05; the Phase 5 notes had said 'no comment threads'). Decided: the comment's writer AND the post's owner can delete a comment. Also decided (developer, in their words: 'we'll have a notification alert thing also designed'): notifications ARE wanted and the developer designs their look (their Today picture has a bell, which Phase 2 D-08 kept out; it may be the same thing). Scope to pin down in Phase 5: an in-app list behind the bell, phone alerts, or both. Phone alerts need a new phone feature (a rebuild of the test app) and, for real alerts, Apple push setup on the developer's launch list; the Phase 2 D-14 8 PM reminder could share that feature. NOT settled yet: renaming the author's own note from 'Comment' to 'Thought' so the two never get mixed up (suggested: yes, a copy change in the Phase 3 plans, nothing built). Not yet written into REQUIREMENTS.md, ROADMAP.md or the Phase 5 questions file.
- [Whole project]: the developer wants everything finished by 2026-10-10; parallelize, keep device checks short.

### Pending Todos

- Phase 2 build waits for the developer's word 'go': build 02-01 (server) and 02-02 (app) in parallel with agents, then 02-03 (cutover, scans, a SHORT device walkthrough). UPDATE 2026-10-10: the developer will start it in a NEW session (their start message there is the go) and wants Phase 2 completed while they step out; the phone walkthrough (02-03 Task 3) needs their taps, so leave it for when they are back.
- Ask the Phase 4 and Phase 5 design questions as ONE message while Phase 2 builds (files in ~/.local/share/rndmroll-phase2/questions/: PHASE4-QUESTIONS.md and PHASE5-QUESTIONS.md), labelled PHASE 4 and PHASE 5, nothing built.
- Friend comments and notifications: once the rename question above is settled, add FEED-05 (friend comments) and a notifications item (scope: bell list, phone alerts or both; the developer designs it) to REQUIREMENTS.md, the Phase 5 requirement list and a criterion in ROADMAP.md, a PROJECT.md decision row, update ~/.local/share/rndmroll-phase2/questions/PHASE5-QUESTIONS.md (its line 'No comment threads' is now wrong), add 'friends' comments' to the Phase 3 out-of-scope list, and apply the rename in the Phase 3 UI spec if chosen.
- Developer: delete the unused GOOGLE_CLIENT_ID_* and APPLE_* lines in .env and .env.example (Claude's permissions deny editing env files); harmless if they stay.
- Developer: the launch list (custom domain, favicon, remove the AI tag, real Terms and Privacy text, Forgot password, the S3 and Resend accounts).

### Blockers/Concerns

- ROLL-01 and ENTRY-04 are resolved on paper (Phase 2 CONTEXT D-01 to D-03: local 8:00 PM to midnight window, late until the next 8:00 PM; ENTRY-04 is a free skip plus a weekly goal of 4 posted days). Nothing of either is built yet.
- Photo authenticity enforcement beyond allowing camera+library choice remains an explicitly open, unsolved question (see PROJECT.md Context)

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none — this is the first milestone)* | | | |

## Session Continuity

Last session: 2026-10-09T18:21:00.000Z
Stopped at: Apple and Google sign-in removed completely (pushed to GitHub on 2026-10-10 at the developer's ask); Phase 2 documents renumbered, waiting for the developer's go; Phase 3 documents written on paper
Resume file: .planning/phases/01-foundation-accounts/.continue-here.md
