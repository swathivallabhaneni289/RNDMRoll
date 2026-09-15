# Phase 1: Foundation & Accounts - Context

**Gathered:** 2026-09-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can create an account, log in (staying logged in across app sessions), and view/edit their own profile. This is the identity foundation the rest of the app (daily roll, diary, friend feed) builds on. Adding friends (ACCT-02) is explicitly out of scope here — that's Phase 5, once there's a friend feed to gate.

</domain>

<decisions>
## Implementation Decisions

### Login Method
- **D-01:** Authentication mirrors Instagram's real signup options: email/phone number + password, PLUS Apple and Google sign-in.
- **D-02:** Apple sign-in is required alongside Google/other third-party social login per Apple App Store guideline 4.8 — not optional once Google sign-in is offered.
- **D-03:** Phone-number signup/SMS verification is not required for MVP — default to email-based signup to avoid needing SMS provider infrastructure (Claude's discretion, not explicitly asked).

### Signup Friction / Onboarding Sequence
- **D-04:** Email verification IS required before a user can use the app (user explicitly chose this over the lower-friction "no verification" option).
- **D-05:** Onboarding sequence mirrors Instagram's real flow: (1) choose signup method → (2) verify email → (3) enter name → (4) auto-suggested username generated from name (editable, with a uniqueness check — NOT a blank field the user fills from scratch) → (5) optional profile photo (skippable) → (6) land in the app.
- **D-06:** Profile photo is optional/skippable at signup, matching real Instagram behavior — can be added later from the profile screen.

### Profile Content
- **D-07:** Phase 1 profile screen shows identity fields only: avatar, name, username, bio. No stats/streak/diary-count placeholders — those depend on features (roll, diary) that don't exist until later phases.
- **D-08:** All profile fields (name, username, bio, photo) are editable starting in Phase 1, not deferred to a later phase.

### Claude's Discretion
- Session/token persistence mechanism (e.g., JWT access+refresh token pair issued by the Go backend, stored via Expo SecureStore) — technical implementation, not asked.
- Whether phone-number signup is skipped entirely for MVP (see D-03) — no explicit ask, assumed email-first is sufficient for a small friend group.
- Password requirements/strength rules, password reset flow mechanics — standard implementation, not discussed.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project decisions & constraints
- `.planning/PROJECT.md` — tech stack (Go backend REST/JSON API, React Native/Expo client), standing design constraints (no purple gradients, no pill buttons, no emoji-as-icons, no AI-slop copy, etc. — applies to every UI/copy surface including login/signup/profile screens), launch gate
- `.planning/REQUIREMENTS.md` — ACCT-01 (create account, log in, stay logged in), ACCT-03 (view own profile)
- `README.md` — "Decisions made so far" table (backend/mobile stack rationale)

</canonical_refs>

<code_context>
## Existing Code Insights

No code exists yet — this is the first implementation phase of a brand-new repository (currently contains only planning docs: README.md, RESEARCH.md, .planning/). No reusable assets, established patterns, or integration points to note yet; this phase establishes them.

</code_context>

<specifics>
## Specific Ideas

- User explicitly wants the account/onboarding experience to closely mirror Instagram's real, current signup and profile-setup flow (not a generic auth form) — referenced multiple times during discussion as the concrete model to follow.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (Adding friends, ACCT-02, remains correctly scoped to Phase 5 per ROADMAP.md — not a new deferral, just confirming the existing boundary.)

</deferred>

---

*Phase: 1-Foundation & Accounts*
*Context gathered: 2026-09-15*
