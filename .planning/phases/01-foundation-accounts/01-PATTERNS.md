# Phase 1: Foundation & Accounts - Pattern Map

**Mapped:** 2026-09-16
**Files analyzed:** 35 (backend + mobile, new-only; counting each onboarding/app screen individually)
**Analogs found:** 0 / 35 in-repo (first implementation phase) — 6 of the UI files have a concrete in-repo reference (the approved `01-UI-SPEC.md`, revision 9) instead of an external-only one

**Reconciled 2026-09-17 against UI-SPEC revision 9 / D-05 (revised):** the separate `name.tsx`, `username.tsx`, and `photo.tsx` onboarding screens were consolidated into a single `app/(auth)/profile-setup.tsx`, so the three rows for them below are now one row. This reconciliation was filename-and-citation level only: the four pre-signup marketing screens revision 9 adds (`welcome.tsx`, `ritual.tsx`, `real-photos.tsx`, `everyone-rolls.tsx`, planned in `01-16-PLAN.md`) are **not** mapped in this document, and the counts above were not re-derived for them. Treat `01-UI-SPEC.md` revision 9 and the plan set as authoritative where this map is silent.

## Repository State

Confirmed by direct inspection: the repository contains only planning docs (`.planning/`), `README.md`, `RESEARCH.md`, and `docs/`. No `cmd/`, `internal/`, `app/`, or any source code exists yet. There are **no in-repo analogs** for any file in this phase — this phase establishes the patterns that later phases will copy from.

Per instructions, this PATTERNS.md does not fabricate nonexistent analog files. Instead, for each file it cites the external/framework convention from RESEARCH.md (§ Architecture Patterns, § Code Examples, § Recommended Project Structure) as the closest available reference, and flags it as the pattern later phases should treat as canonical once this phase lands.

## File Classification

| New File | Role | Data Flow | Reference Source | Match Quality |
|----------|------|-----------|-------------------|---------------|
| `cmd/api/main.go` | config/wiring | request-response | RESEARCH.md Recommended Project Structure (Backend) | no analog — establishes pattern |
| `internal/httpapi/auth.go` (signup/login/refresh handlers) | controller | request-response | RESEARCH.md Code Examples "Signup handler skeleton (Go/Gin)" | no analog — establishes pattern |
| `internal/httpapi/oauth.go` (Apple/Google endpoints) | controller | request-response | RESEARCH.md Architecture Patterns Pattern 2 | no analog — establishes pattern |
| `internal/httpapi/verify_email.go` | controller | request-response | RESEARCH.md Common Pitfalls #5 + Architecture Patterns | no analog — establishes pattern |
| `internal/httpapi/profile.go` (`GET/PATCH /me`) | controller | CRUD | RESEARCH.md Architectural Responsibility Map (Profile row) | no analog — establishes pattern |
| `internal/httpapi/username.go` (`GET /usernames/suggest`) | controller | request-response | RESEARCH.md Code Examples "Username suggestion endpoint" | no analog — establishes pattern |
| `internal/auth/password.go` (bcrypt hash/compare) | utility | transform | RESEARCH.md Standard Stack (bcrypt) + Pitfall 1 | no analog — establishes pattern |
| `internal/auth/jwt.go` (access token issue/verify) | utility | transform | RESEARCH.md Architecture Patterns Pattern 1 (`IssueAccessToken`) | no analog — establishes pattern |
| `internal/auth/refresh.go` (opaque refresh token issue/rotate) | service | CRUD | RESEARCH.md Architecture Patterns Pattern 1 (refresh_tokens table) | no analog — establishes pattern |
| `internal/auth/oauth_google.go` (idtoken.Validate wrapper) | service | request-response | RESEARCH.md Architecture Patterns Pattern 2 (Google example) | no analog — establishes pattern |
| `internal/auth/oauth_apple.go` (JWKS verify via keyfunc) | service | request-response | RESEARCH.md Architecture Patterns Pattern 2 (Apple example) + Pitfall 1b | no analog — establishes pattern |
| `internal/middleware/auth.go` (parse+validate access token) | middleware | request-response | RESEARCH.md Recommended Project Structure (`internal/middleware/`) | no analog — establishes pattern |
| `internal/middleware/recovery.go`, `logging.go` | middleware | request-response | RESEARCH.md Standard Stack (Gin built-in recovery) | no analog — establishes pattern |
| `internal/user/model.go` (domain User struct) | model | CRUD | RESEARCH.md Architectural Responsibility Map | no analog — establishes pattern |
| `internal/user/repository.go` (repo interface) | model | CRUD | RESEARCH.md Recommended Project Structure (`internal/user/`) | no analog — establishes pattern |
| `internal/store/postgres/user_repo.go` | model | CRUD | RESEARCH.md Standard Stack (pgx/v5) | no analog — establishes pattern |
| `internal/store/postgres/refresh_token_repo.go` | model | CRUD | RESEARCH.md Architecture Patterns Pattern 1 | no analog — establishes pattern |
| `internal/store/postgres/email_verification_repo.go` | model | CRUD | RESEARCH.md Common Pitfalls #5 | no analog — establishes pattern |
| `internal/mail/mailer.go` (interface) + provider impl | service | event-driven | RESEARCH.md Open Questions #2 (provider TBD — unresolved, no provider selected) | no analog — blocked on provisioning |
| `migrations/0001_users.up.sql` / `.down.sql` (+ email_verification_tokens, refresh_tokens) | migration | batch | RESEARCH.md Standard Stack (golang-migrate) | no analog — establishes pattern |
| `app/_layout.tsx` (root auth-gated layout) | provider | event-driven | RESEARCH.md Architecture Patterns Pattern 3 (`Stack.Protected`) | no analog — establishes pattern |
| `app/(auth)/_layout.tsx` | component | request-response | RESEARCH.md Recommended Project Structure (Mobile) | no analog — establishes pattern |
| `app/(auth)/choose-method.tsx` | component | request-response | `01-UI-SPEC.md` Brand Mark, Background Texture, Copywriting Contract (wordmark/CTA copy), Social sign-in buttons | in-repo UI-SPEC reference |
| `app/(auth)/verify-email.tsx` | component | request-response | `01-UI-SPEC.md` Display Role Application (promoted heading), Visual Personality (mail-outline icon badge), Interaction Contracts > Verify-email screen | in-repo UI-SPEC reference |
| `app/(auth)/profile-setup.tsx` | component | request-response | `01-UI-SPEC.md` Typography (Heading role), Interaction Contracts > Create your profile screen state machine, Form validation timing, Color (Success/Destructive status colors), Copywriting Contract (Create your profile / Optional / Finish setup) | in-repo UI-SPEC reference |
| `app/(app)/_layout.tsx` | component | CRUD | RESEARCH.md Recommended Project Structure (Mobile) | no analog — establishes pattern |
| `app/(app)/profile/index.tsx` | component | CRUD | `01-UI-SPEC.md` Elevation (`card`/`lg` radius container), Visual Personality (empty-bio icon badge), Copywriting Contract (empty state copy) | in-repo UI-SPEC reference |
| `app/(app)/profile/edit.tsx` | component | CRUD | `01-UI-SPEC.md` Shape (`md` radius inputs), Color (CTA fill states), Copywriting Contract ("Save changes") | in-repo UI-SPEC reference |
| `lib/api/client.ts` (typed fetch, attaches token, 401→refresh→retry) | service | request-response | RESEARCH.md Architecture Patterns (client refresh-on-401 described in Primary use-case trace) | no analog — establishes pattern |
| `lib/session/store.ts` (SecureStore read/write, session context) | store | event-driven | RESEARCH.md Standard Stack (expo-secure-store) + Pattern 3 | no analog — establishes pattern |

## Pattern Assignments

Since no in-repo analogs exist, each entry below cites the RESEARCH.md excerpt to copy from directly, verbatim where RESEARCH.md already gives code.

### `internal/httpapi/auth.go` (controller, request-response)

**Reference:** RESEARCH.md "Code Examples > Signup handler skeleton (Go/Gin)" (lines ~360-380 of 01-RESEARCH.md)

```go
type SignupRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt's 72-byte ceiling
}

func (h *AuthHandler) Signup(c *gin.Context) {
    var req SignupRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil { /* 500 */ return }
    user, err := h.users.Create(c.Request.Context(), req.Email, hash) // email_verified=false
    if err != nil { /* handle unique-email conflict -> 409 */ return }
    token := h.mail.SendVerificationEmail(c.Request.Context(), user)
    c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
}
```

Apply this shape (Gin binding struct + validator tags + bcrypt + repo call + typed error → HTTP status mapping) to `Login`, `Refresh`, `VerifyEmail` handlers in the same file/package. Login/Refresh should return the generic "invalid credentials" error on any auth failure (RESEARCH.md Security Domain — Credential stuffing mitigation), never distinguishing "no such user" from "wrong password."

---

### `internal/auth/jwt.go` (utility, transform) and `internal/auth/refresh.go` (service, CRUD)

**Reference:** RESEARCH.md "Architecture Patterns > Pattern 1: Access token + DB-backed refresh token"

```go
func IssueAccessToken(userID string, secret []byte) (string, error) {
    claims := jwt.RegisteredClaims{
        Subject:   userID,
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(secret)
}
// Refresh token: crypto/rand 32 bytes, store sha256(token) + expiry + userID in a
// refresh_tokens table; on /auth/refresh, look up by hash, check expiry/revocation,
// rotate (issue new refresh token, invalidate old) to limit replay window.
```

15-minute access token expiry, 30-day refresh token expiry (per RESEARCH.md Security Domain — Refresh token theft/replay mitigation: rotate on each use, store hashed not plaintext).

---

### `internal/auth/oauth_google.go` / `internal/auth/oauth_apple.go` (service, request-response)

**Reference:** RESEARCH.md "Architecture Patterns > Pattern 2: Server-side verification of third-party identity tokens"

```go
payload, err := idtoken.Validate(ctx, googleIDToken, googleClientID)
if err != nil { /* reject */ }
email := payload.Claims["email"].(string)
```

Apple: fetch/cache JWKS from `https://appleid.apple.com/auth/keys` via `keyfunc`, then parse+verify `identityToken` with `golang-jwt`, checking `iss=="https://appleid.apple.com"` and `aud==<Apple Service ID/bundle ID>`.

**Critical constraint (Pitfall 1b):** Apple returns `fullName`/`email` only on the user's *first* authorization for a given Apple ID + bundle ID. Persist both to the `users` row immediately on that first callback — the name field on the consolidated create-profile screen (D-05, revised 2026-09-17) must pre-fill from this payload for Apple signups since a retry will return `null`.

**Never trust client-asserted identity** (Pitfall 3 / Security Domain — Forged social-login identity): the OAuth endpoints must accept only the raw provider token, never a `{provider, email, name}` payload from the client.

---

### `internal/httpapi/username.go` + username-suggestion logic (controller + utility, request-response)

**Reference:** RESEARCH.md "Code Examples > Username suggestion endpoint"

```go
func SuggestUsername(ctx context.Context, repo UserRepo, displayName string) (string, error) {
    base := normalize(displayName) // lowercase, strip to [a-z0-9_], truncate to e.g. 20 chars
    if base == "" {
        base = "user"
    }
    candidate := base
    for i := 0; i < maxAttempts; i++ {
        taken, err := repo.UsernameTaken(ctx, candidate)
        if err != nil { return "", err }
        if !taken {
            return candidate, nil
        }
        candidate = fmt.Sprintf("%s%d", base, randSuffix()) // random suffix, not sequential
    }
    return "", ErrNoAvailableUsername
}
```

**Race-condition constraint (Pitfall 4):** the live suggest/availability endpoint is UX convenience only — back it with a case-insensitive uniqueness constraint in Postgres (`citext` column or `UNIQUE` index on `lower(username)`, same treatment for `email`). The authoritative rejection happens at insert time via the unique-constraint violation, not the pre-check.

---

### `migrations/0001_*.up.sql` (migration, batch)

**Reference:** RESEARCH.md Standard Stack — `golang-migrate/migrate/v4`, paired with Pitfall 4 (case-insensitive uniqueness).

Tables needed: `users` (id, email UNIQUE via lower(email) index, password_hash nullable for social-only accounts, username UNIQUE via lower(username) index, name, bio, avatar_url, email_verified boolean, timestamps), `email_verification_tokens` (hashed token, user_id, expires_at, consumed_at), `refresh_tokens` (hashed token, user_id, expires_at, revoked_at, device/session metadata).

**Open question, not a settled pattern (RESEARCH.md Open Questions #1 / Assumption A10, risk Medium-High):** whether `users` needs an `email_verified_via` enum (`password_flow`/`apple`/`google`) to record *how* an email was verified, so social signups can skip the redundant verification-email step. RESEARCH.md's own text says this must be confirmed with the user before locking into a plan — this pattern map flags it as open rather than prescribing the column, since resolving it is a planning/product decision, not a pattern-copy decision.

---

### `app/_layout.tsx` (provider, event-driven)

**Reference:** RESEARCH.md "Architecture Patterns > Pattern 3: Auth-gated routing with Expo Router" — cited directly from `docs.expo.dev/router/advanced/authentication/`

```tsx
export default function RootLayout() {
  const { status } = useSession(); // 'loading' | 'authenticated' | 'unauthenticated'
  if (status === 'loading') return <SplashScreen />;
  return (
    <Stack>
      <Stack.Protected guard={status === 'authenticated'}>
        <Stack.Screen name="(app)" />
      </Stack.Protected>
      <Stack.Protected guard={status !== 'authenticated'}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
    </Stack>
  );
}
```

---

### `lib/session/store.ts` (store, event-driven)

**Reference:** RESEARCH.md Standard Stack — `expo-secure-store` (Keychain/Keystore-backed). On app relaunch: read SecureStore, call `/auth/refresh`, route to `(app)` on success, `(auth)` on failure/absence (RESEARCH.md Primary use-case trace, final paragraph).

---

### Onboarding + profile screens `app/(auth)/*.tsx`, `app/(app)/profile/*.tsx` (component, request-response / CRUD)

**Primary reference for these files: the approved `01-UI-SPEC.md` (revision 9, light editorial theme, status: approved 2026-09-17)** — this is an in-repo, concrete design contract, not an external framework convention, and takes precedence over RESEARCH.md's generic structure notes for anything visual/interaction-level. RESEARCH.md's Recommended Project Structure (Mobile) still governs the file layout (`app/(auth)/`, `app/(app)/profile/`). The post-signup D-05 (revised 2026-09-17) sequence order is now: `choose-method.tsx` → `verify-email.tsx` → `profile-setup.tsx`, the single consolidated screen carrying the optional avatar (per D-06), the name, the username (auto-suggested, editable, live uniqueness check — **never a blank field**, per RESEARCH.md Anti-Patterns), and the bio.

**Concrete tokens to copy from `01-UI-SPEC.md`:**
- **Color** (§ Color, lines ~120-163): Dominant `#F6F5F2` backgrounds, Secondary `#E8E7E3` cards/fields, `#FFFFFF` elevation-only white, Ink `#111111` (default text + sole accent), Muted `#625F5B`, Divider `#D3D0C9`, Destructive `#9A3B32`, Success `#416B4C`.
- **Shape** (§ Shape (Radius), lines ~56-65): `md` (8dp) is the hard cap on buttons/inputs — no pill buttons; `lg` (24dp) reserved for large non-interactive containers only — the profile-view container card, the log-out sheet, and (extended in revision 9) the marketing photo panels — never on tappable elements.
- **Typography** (§ Typography, lines ~71-97): Body 16/24, Label 14/20, Button/CTA 16/24 (`WorkSans_600SemiBold`), Heading 22/28 (`Domine_600SemiBold`), Display 40/46 — per revision 9's Display Role Application, Display is used on the four pre-signup marketing screens plus the choose-method wordmark and the verify-email heading, and explicitly **not** on the dense create-profile screen, which takes the Heading tier.
- **Elevation** (§ Elevation, lines ~100-116): `subtle` (flat, no shadow) for field rows; `card` (`#FFFFFF` fill + soft shadow) for the profile container; `raised` (Ink fill + heaviest shadow) for the primary CTA only.
- **Interaction Contracts** (§ Interaction Contracts): the "Create your profile screen state machine" section, which revision 9 substituted for revision 8's separate "Username step state machine" and "Photo step (D-06)" sections and which carries the username states (idle/checking/available/taken/insert-conflict) and the optional-photo affordance together; plus verify-email waiting/resend-cooldown/deep-link states, social sign-in button loading/error/cancel states, form validation timing (blur vs. 400ms-debounced live), session-restore/font-load splash gate, the pre-signup marketing sequence's advance/back rules, and the icon-only `accessibilityLabel` requirement.
- **Copywriting Contract** (§ Copywriting Contract, lines ~247-267): exact button/error/empty-state strings — use verbatim, do not paraphrase (e.g. "That email's already registered. Log in instead.", "Couldn't connect. Check your connection and try again.").

**Standing design constraints still apply on top of UI-SPEC tokens** (PROJECT.md Constraints, restated in RESEARCH.md's Project Constraints section and the user's own memory): no purple gradients, no pill-shaped buttons, no fake reviews/counters, no vague hero text, no emoji-as-icons, no em dashes, no over-the-top scroll animations, no AI-slop photos/copy, no cursor animations. UI-SPEC revision 9 was checker-approved against these constraints already (see its Checker Sign-Off), so implementers should treat UI-SPEC as the pre-verified concretization of these rules, not a separate check to redo from scratch.

---

## Shared Patterns

### Server-side token verification (never trust client identity claims)
**Source:** RESEARCH.md Architecture Patterns Pattern 2 + Common Pitfalls #3 + Security Domain
**Apply to:** `internal/auth/oauth_google.go`, `internal/auth/oauth_apple.go`, `internal/httpapi/oauth.go`
Always verify the raw provider token server-side (signature + `iss` + `aud` + `exp`) before minting a session. Reject `{provider, email, name}`-shaped client payloads outright — accept only the raw identity/ID token.

### Password hashing — never hand-roll
**Source:** RESEARCH.md Don't Hand-Roll table, Pitfall 1
**Apply to:** `internal/auth/password.go`, `internal/httpapi/auth.go`
`bcrypt.GenerateFromPassword` with `bcrypt.DefaultCost`; cap password length at 8-72 bytes at the validator layer (`binding:"required,min=8,max=72"`), don't pre-hash to work around the 72-byte ceiling.

### JWT signing/verification — algorithm allow-listing
**Source:** RESEARCH.md Don't Hand-Roll, Anti-Patterns, Security Domain
**Apply to:** `internal/auth/jwt.go`, `internal/middleware/auth.go`
Use `golang-jwt/jwt/v5` exclusively; never hand-parse JWTs or accept `alg: none`. v5 defaults to safe algorithm allow-listing — explicitly specify the expected signing method when parsing.

### Access control on `/me` — subject-from-token, never client-supplied ID
**Source:** RESEARCH.md Security Domain (V4 Access Control)
**Apply to:** `internal/httpapi/profile.go`, `internal/middleware/auth.go`
`GET/PATCH /me` must derive the target user exclusively from the authenticated access token's `sub` claim — never from a client-supplied user ID in the URL or body.

### Input validation via struct tags
**Source:** RESEARCH.md Standard Stack (`go-playground/validator/v10`), Security Domain (V5)
**Apply to:** All `internal/httpapi/*.go` request DTOs
`binding:"required,email"`, `binding:"required,min=8,max=72"`, username charset/length constraints — integrated directly with Gin's `ShouldBindJSON`.

### Email verification token — hashed, single-use, expiring
**Source:** RESEARCH.md Common Pitfalls #5
**Apply to:** `internal/httpapi/verify_email.go`, `internal/store/postgres/email_verification_repo.go`
Store the token hashed (not plaintext) with a 24h expiry; mark consumed on successful verification; reject already-consumed or expired tokens.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/api/main.go` | config/wiring | request-response | No backend code exists yet; no in-repo wiring/main pattern to copy |
| `internal/httpapi/auth.go`, `oauth.go`, `verify_email.go`, `profile.go`, `username.go` | controller | request-response / CRUD | No `internal/httpapi/` directory exists yet |
| `internal/auth/password.go`, `jwt.go`, `refresh.go`, `oauth_google.go`, `oauth_apple.go` | utility/service | transform / request-response / CRUD | No `internal/auth/` directory exists yet |
| `internal/middleware/auth.go`, `recovery.go`, `logging.go` | middleware | request-response | No `internal/middleware/` directory exists yet |
| `internal/user/model.go`, `repository.go` | model | CRUD | No `internal/user/` directory exists yet |
| `internal/store/postgres/user_repo.go`, `refresh_token_repo.go`, `email_verification_repo.go` | model | CRUD | No `internal/store/` directory exists yet |
| `internal/mail/mailer.go` + provider impl | service | event-driven | No `internal/mail/` directory exists yet; concrete provider also blocked on Open Question #2 (provider not yet selected) |
| `migrations/0001_*.up.sql`/`.down.sql` (users, email_verification_tokens, refresh_tokens) | migration | batch | No `migrations/` directory exists yet |
| `app/_layout.tsx` | provider | event-driven | No `app/` directory exists yet |
| `app/(auth)/_layout.tsx`, `app/(app)/_layout.tsx` | component | request-response / CRUD | No `app/(auth)/` or `app/(app)/` directories exist yet |
| `lib/api/client.ts` | service | request-response | No `lib/api/` directory exists yet |
| `lib/session/store.ts` | store | event-driven | No `lib/session/` directory exists yet |

All entries above reference RESEARCH.md's "Recommended Project Structure," "Architecture Patterns," and "Code Examples" sections as the closest available (external, not in-repo) pattern source. The 6 onboarding/profile UI files (`choose-method.tsx`, `verify-email.tsx`, `profile-setup.tsx`, `profile/index.tsx`, `profile/edit.tsx`, and file layout for `_layout.tsx`s) are the exception — those have the approved in-repo `01-UI-SPEC.md` as a concrete reference and are listed separately in File Classification above, not in this table.

**Phase 2+ PATTERNS.md documents should treat this phase's actual implementation as the first real in-repo analog set** once it lands (e.g., future controller files should copy from `internal/httpapi/auth.go`'s handler shape, not re-derive from RESEARCH.md; future onboarding-adjacent screens should copy from this phase's implemented screens, which will themselves already conform to `01-UI-SPEC.md`'s tokens).

Two items are explicitly blocked on external provisioning, not on code patterns (RESEARCH.md Environment Availability):
- S3-compatible object storage credentials — blocks avatar upload implementation (`profile.go` photo field, the avatar control on `profile-setup.tsx`)
- Transactional email provider account — blocks `internal/mail/mailer.go` concrete implementation (interface/abstraction can still be built and unit-tested against a mock)

## Metadata

**Analog search scope:** Full repository (`.planning/`, `README.md`, `RESEARCH.md`, `docs/`) — confirmed no `cmd/`, `internal/`, `app/`, or other source directories exist. Read directly: `01-CONTEXT.md`, `01-RESEARCH.md`, `01-UI-SPEC.md`, `.planning/PROJECT.md`. Checked and confirmed absent: `.claude/skills/`, `.agents/skills/`.
**Files scanned:** All top-level and depth-2 repository entries (8 items in `.planning/phases/01-foundation-accounts/` plus top-level repo listing; none outside `01-UI-SPEC.md` are source code).
**Pattern extraction date:** 2026-09-16
