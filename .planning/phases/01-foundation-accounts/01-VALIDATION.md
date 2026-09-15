---
phase: 1
slug: foundation-accounts
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-15
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go: standard `testing` package + `net/http/httptest` (no third-party test framework needed). Mobile: no test framework set up yet — manual UAT substitutes for Phase 1 (see Manual-Only Verifications). |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./internal/httpapi -run <TestName> -v` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/... -run <relevant test> -v`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full backend suite must be green; manual UAT walkthrough of the full Instagram-style onboarding sequence (D-05) on device/simulator
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

Task IDs are assigned during planning (step 8) — this table maps requirements to their verification approach ahead of that; the planner aligns actual task IDs to these rows.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | ACCT-01 | V2/V3 | Signup creates a user with `email_verified=false` and a hashed (bcrypt) password | unit | `go test ./internal/httpapi -run TestSignup -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ACCT-01 | V2/V3 | Login with correct credentials returns access+refresh tokens; wrong password rejected with a generic error | unit | `go test ./internal/httpapi -run TestLogin -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ACCT-01 | V3 | Refresh endpoint issues a new access token for a valid refresh token; rejects expired/revoked ones | unit | `go test ./internal/httpapi -run TestRefresh -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ACCT-01 | V2 | Unverified user cannot access `(app)`-gated endpoints (D-04 email verification gate) | integration | `go test ./internal/httpapi -run TestEmailVerificationGate -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ACCT-03 | V4 | `GET /me` returns the caller's own profile only, derived from the token subject claim (never a client-supplied ID) | unit | `go test ./internal/httpapi -run TestGetProfile -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ACCT-03 | V4/V5 | `PATCH /me` updates name/username/bio, rejects a taken username | unit | `go test ./internal/httpapi -run TestUpdateProfile -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/httpapi/*_test.go` — covers ACCT-01, ACCT-03 (httptest-based handler tests against a test Postgres instance or a repository mock)
- [ ] Test Postgres setup (`TEST_DATABASE_URL` pointing at a local/dockerized instance, or a mock repository layer) — needed before any handler test can run
- [ ] Go test framework: no install needed (`testing` is stdlib); `github.com/stretchr/testify` optional for assertion ergonomics

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full Instagram-style onboarding sequence (signup method → verify email → name → auto-suggested editable username → optional photo → land in app) | ACCT-01 | No mobile test framework set up in Phase 1 (RESEARCH.md flags this as an accepted gap given small scope) | Walk through signup end-to-end on iOS simulator and Android emulator; confirm each step matches CONTEXT.md D-05 sequence and PROJECT.md design constraints (no purple gradients/pill buttons/emoji icons/etc.) |
| Apple Sign In first-authorization name/email capture | ACCT-01 | Requires a real Apple ID and device-level Apple auth flow; name/email are only returned on the FIRST authorization and must be persisted immediately | Sign in with a fresh Apple test account, confirm name/email are captured and stored on first auth; sign out and back in, confirm subsequent logins still work without needing that data again |
| Google Sign In flow | ACCT-01 | Requires real Google OAuth consent flow on-device | Sign in with a Google test account on device/simulator, confirm profile is created/linked correctly |
| Profile edit (name/username/bio/photo) | ACCT-03 | UI-level interaction, no mobile test framework yet | Edit each field from the profile screen, confirm changes persist after app restart |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
