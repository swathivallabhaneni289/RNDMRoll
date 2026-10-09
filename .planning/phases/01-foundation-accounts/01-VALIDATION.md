---
phase: 1
slug: foundation-accounts
status: automated_gates_passed
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-15
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

> **Revised 2026-10-09:** Apple and Google sign-in were removed from the project completely. The rows below about a social account under 13 being deleted, the oauth body cap, the Apple and Google claim (`ClaimAndRevoke`) and the manual Apple and Google sign-in checks describe tests and checks that were deleted with the feature; they stay here as history. Still true: sign-up under 13 is refused and nothing is stored, the 16 KB body cap on signup, login and `PATCH /me`, and `PATCH /me` always refuses a birthday (it is set once, at sign-up). New: `TestMigrations_0003IsReversible` covers migration 0003, which drops the two provider columns.

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
- **Before `/gsd-verify-work`:** Full backend suite must be green; manual UAT walkthrough of the two-page sign-up (01-19 Task 4 as rewritten by 01-20) on device/simulator
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

Task IDs are assigned during planning (step 8) — this table maps requirements to their verification approach ahead of that; the planner aligns actual task IDs to these rows.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| Task 1 | 01-19 | 7 | ACCT-01 | V2/V3 | One request `POST /v1/auth/signup` (email, password, birthday, name, username, optional bio) creates a complete user with `email_verified=false` and a hashed (bcrypt) password in a single INSERT, and returns the login body (tokens plus user, no birthday) | unit + integration | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -p 1 -run Signup -v` (52 tests match, none skipped) | ✅ | ✅ green |
| Task 1 | 01-08 | 4 | ACCT-01 | V2/V3 | Login with correct credentials returns access+refresh tokens; wrong password rejected with a generic error | unit | `go test ./internal/httpapi -run 'TestLogin_CorrectCredentials_Returns200WithTokensAndUser\|TestLogin_WrongPassword_Returns401InvalidCredentials' -v` | ✅ | ✅ green |
| Task 2 | 01-08 | 4 | ACCT-01 | V3 | Refresh endpoint issues a new access token for a valid refresh token; rejects expired/revoked ones | unit | `go test ./internal/httpapi -run 'TestRefresh_ValidToken_Returns200WithNewTokenPairDifferentFromPresented\|TestRefresh_ExpiredToken_Returns401TokenExpired' -v` | ✅ | ✅ green |
| Task 3 | 01-19 | 7 | ACCT-01 | V2 | The signed-in group needs only a valid token for an account that still exists (`RequireAuth` plus `RequireUser`): a deleted user's token gets 401 `token_invalid` on `/me` and `/me/avatar/upload-url`, and an unverified account can log in, refresh and reach `/me` (the old verified-email gate, D-04, is retired until launch) | unit + integration | `go test ./internal/middleware -p 1 -run RequireUser -v && TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -p 1 -run 'Unverified\|Deleted' -v` | ✅ | ✅ green |
| Task 3 | 01-11 | 4 | ACCT-03 | V4 | `GET /me` returns the caller's own profile only, derived from the token subject claim (never a client-supplied ID) | unit | `go test ./internal/httpapi -run 'TestProfile_GetMe_ReturnsCallerProfileWithOnboardingComplete\|TestProfile_GetMe_NoBearerTokenReturns401' -v` | ✅ | ✅ green |
| Task 3 | 01-11 | 4 | ACCT-03 | V4/V5 | `PATCH /me` updates name/username/bio, rejects a taken username | unit | `go test ./internal/httpapi -run 'TestProfile_PatchMe_UpdatesFieldsIndependently\|TestProfile_PatchMe_UsernameTakenReturns409WithAlternates' -v` | ✅ | ✅ green |
| Task 1 | 01-19 | 7 | ACCT-01 | V5 | Age rule: under 13 is refused with 403 `under_minimum_age` and nothing is stored (sign-up); a social account under 13 is deleted, no other field applies and refresh fails; `ValidateBirthdate` and `AgeOn` cover Feb 29 in a non-leap year, exactly today minus 13 years, and tomorrow | unit + integration | `go test ./internal/user -run 'Birthdate\|Age' -v && TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -p 1 -run 'UnderThirteen\|ThirteenToday\|DayBeforeThirteenth\|AgeIsJudged' -v` | ✅ | ✅ green |
| Task 1 | 01-19 | 7 | ACCT-01 | V8 | Birthday privacy: no response body, error bodies and the 409 `username_taken` body included, has any key containing "birth"; `userColumns` selects `has_birthday` and never the date | integration | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -p 1 -run Birth -v` | ✅ | ✅ green |
| Task 1 | 01-19 | 7 | ACCT-01 | V13 | Body cap: a request over 16 KB (oversize `Content-Length` and chunked) gets 413 `payload_too_large` on signup, login, oauth google and `PATCH /me`; a spoofed `X-Forwarded-For` is still limited by its real address | unit + integration | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/middleware ./internal/httpapi -p 1 -run 'BodyLimit\|TooLarge\|Spoof' -v` | ✅ | ✅ green |
| Task 1 | 01-19 | 7 | ACCT-03 | V4/V5 | `GET /v1/usernames/suggest` and `/available` work with no token, are limited to 90 per minute per IP, and reject `name` over 100 or `username` over 20 characters before any query | integration | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi -p 1 -run Username -v` | ✅ | ✅ green |
| Task 1 | 01-19 | 7 | ACCT-01 | V2 | Apple and Google claim: a pre-registered unverified email account is claimed in one transaction (`ClaimAndRevoke`: verified true, password cleared, all refresh tokens revoked); name, username, avatar and birthday are kept, onboarding stays complete, and the pre-registrant's refresh then fails | integration | `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/httpapi ./internal/store/postgres -p 1 -run 'Claim' -v` | ✅ | ✅ green |

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
| Two-page sign-up (page 1 "Create your account.": email, password, birthday picked on a date scroller, name, username, green "Looks good." notes, one Continue, no code and no email check; page 2 "Make it yours.": photo and bio, Continue or Skip for now, shown once, then the landing page), the finish mode for a social account, relaunch, and edit | ACCT-01, ACCT-03 | No mobile test framework set up in Phase 1 (RESEARCH.md flags this as an accepted gap given small scope) | Walk plan 01-19 Task 4 as rewritten by 01-20 (groups 1 to 5) on the iOS simulator; confirm each step and the PROJECT.md design constraints (no purple gradients, pill buttons, emoji icons, em dashes, etc.) |
| Apple Sign In first-authorization name/email capture | ACCT-01 | Requires a real Apple ID and device-level Apple auth flow; name/email are only returned on the FIRST authorization and must be persisted immediately | Sign in with a fresh Apple test account, confirm name/email are captured and stored on first auth; sign out and back in, confirm subsequent logins still work without needing that data again |
| Google Sign In flow | ACCT-01 | Requires real Google OAuth consent flow on-device | Sign in with a Google test account on device/simulator, confirm profile is created/linked correctly |
| Profile edit (name/username/bio/photo) | ACCT-03 | UI-level interaction, no mobile test framework yet | Edit each field from the profile screen, confirm changes persist after app restart |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

The 01-19 rows were re-run on 2026-10-07 after Task 1: every `-run` pattern above matches real tests and passes (the full suite on `rndmroll_test` is 254 passes, 0 failures, 0 skips, run with `-p 1`). The other rows were backed by passing tests in the earlier 01-15 sweep and are covered again by that full run; the phase-closing sweep is re-run before closing (see 01-15-SUMMARY.md). The Manual-Only Verifications above remain outstanding pending the device walkthrough (01-19 Task 4).

**Approval:** pending device walkthrough (01-15 Task 2)
