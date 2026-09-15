# Phase 1: Foundation & Accounts - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-15
**Phase:** 1-Foundation & Accounts
**Areas discussed:** Login method, Signup friction, Profile content

---

## Login method

| Option | Description | Selected |
|--------|-------------|----------|
| Email/phone + password only | The core Instagram signup mechanic, no third-party social login | |
| Email/phone + password, plus Apple/Google sign-in | Matches Instagram's full option set | ✓ |
| You decide | Let Claude pick based on fastest MVP path | |

**User's choice:** Email/phone + password, plus Apple/Google sign-in.
**Notes:** Initial freeform answer was "Instagram let people authenticate their accounts" — clarified into the concrete option above once Instagram's actual signup mechanics (email/phone + password, or continue with Facebook, plus Apple sign-in per App Store rules) were spelled out. Confirmed: Apple sign-in is required alongside Google per App Store guideline 4.8. Phone-number/SMS signup was not explicitly requested — defaulted to email-based signup only (Claude's discretion) to avoid SMS provider infrastructure for MVP.

---

## Signup friction / Onboarding sequence

| Option | Description | Selected |
|--------|-------------|----------|
| No verification required | Lowest friction, jump straight in | |
| Email verification required | Must confirm before posting | ✓ |
| You decide | | |

**User's choice:** Email verification required.

**Follow-up — username setup:** User's freeform answer ("onboarding should be similar to what Instagram setup") was clarified into a concrete sequence: (1) choose signup method → (2) verify email → (3) enter name → (4) auto-suggested username generated from name, editable, with uniqueness check → (5) optional profile photo → (6) land in app. Confirmed via follow-up question (photo optional vs. required) — user selected **photo optional (Recommended)**, matching real Instagram behavior.

**Notes:** Two "Other/clarify" rounds were needed here — user's first two answers ("Instagram let people authenticate" and "should be similar to what Instagram setup") were high-level references to Instagram's real UX rather than direct option picks, so each was translated into concrete choices and confirmed before locking in.

---

## Profile content

| Option | Description | Selected |
|--------|-------------|----------|
| Identity fields, editable | Avatar, name, username, bio — editable from Phase 1 | ✓ |
| Identity fields, view-only | Same fields, editing deferred to later phase | |
| You decide | | |

**User's choice:** Identity fields, editable.
**Notes:** No stats/streak/diary-count shown in Phase 1 — those features don't exist until later phases (Phase 2 roll, Phase 4 diary).

---

## Claude's Discretion

- Session/token persistence mechanism (JWT access+refresh via Go backend, stored in Expo SecureStore) — technical implementation detail, not asked.
- Phone-number/SMS signup omitted from MVP scope — no explicit ask; email-only assumed sufficient for a small friend group.
- Password requirements/strength rules and password-reset flow mechanics — standard implementation, not discussed.

## Deferred Ideas

None — discussion stayed within Phase 1 scope. Adding friends (ACCT-02) remains correctly scoped to Phase 5 per ROADMAP.md.
