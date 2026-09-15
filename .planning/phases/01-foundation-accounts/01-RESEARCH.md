# Phase 1: Foundation & Accounts - Research

**Researched:** 2026-09-15
**Domain:** Mobile app authentication (email/password + Apple/Google sign-in), email verification, session persistence, profile management — Go REST backend + React Native/Expo client
**Confidence:** MEDIUM (core libraries and official platform requirements are CITED/VERIFIED against primary sources; several architecture-pattern and product-behavior details are ASSUMED from training knowledge and flagged in the Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Authentication mirrors Instagram's real signup options: email/phone number + password, PLUS Apple and Google sign-in.
- **D-02:** Apple sign-in is required alongside Google/other third-party social login per Apple App Store guideline 4.8 — not optional once Google sign-in is offered.
- **D-03:** Phone-number signup/SMS verification is not required for MVP — default to email-based signup to avoid needing SMS provider infrastructure (Claude's discretion, not explicitly asked).
- **D-04:** Email verification IS required before a user can use the app (user explicitly chose this over the lower-friction "no verification" option).
- **D-05:** Onboarding sequence mirrors Instagram's real flow: (1) choose signup method → (2) verify email → (3) enter name → (4) auto-suggested username generated from name (editable, with a uniqueness check — NOT a blank field the user fills from scratch) → (5) optional profile photo (skippable) → (6) land in the app.
- **D-06:** Profile photo is optional/skippable at signup, matching real Instagram behavior — can be added later from the profile screen.
- **D-07:** Phase 1 profile screen shows identity fields only: avatar, name, username, bio. No stats/streak/diary-count placeholders — those depend on features (roll, diary) that don't exist until later phases.
- **D-08:** All profile fields (name, username, bio, photo) are editable starting in Phase 1, not deferred to a later phase.

### Claude's Discretion

- Session/token persistence mechanism (e.g., JWT access+refresh token pair issued by the Go backend, stored via Expo SecureStore) — technical implementation, not asked.
- Whether phone-number signup is skipped entirely for MVP (see D-03) — no explicit ask, assumed email-first is sufficient for a small friend group.
- Password requirements/strength rules, password reset flow mechanics — standard implementation, not discussed.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. (Adding friends, ACCT-02, remains correctly scoped to Phase 5 per ROADMAP.md — not a new deferral, just confirming the existing boundary.)

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ACCT-01 | User can create an account and log in, and stays logged in across app sessions | Standard Stack (bcrypt, golang-jwt, pgx), Architecture Patterns (access+refresh token issuance, SecureStore persistence, silent-refresh-on-launch), Common Pitfalls (token expiry/refresh races, bcrypt 72-byte limit), Code Examples |
| ACCT-03 | User can view their own profile | Architecture Patterns (profile GET/PATCH `/me` endpoint), Standard Stack (validator for PATCH payloads), Don't Hand-Roll (image handling via expo-image-picker/expo-image, not custom) |
</phase_requirements>

## Project Constraints (from PROJECT.md)

No `CLAUDE.md` exists in this repository. CONTEXT.md's `canonical_refs` explicitly directs downstream agents to PROJECT.md's standing design constraints, and states they apply to "every UI/copy surface including login/signup/profile screens" — this phase delivers ~8 such surfaces (six onboarding screens per D-05, plus profile view and profile edit, per D-07/D-08). These constraints apply with the same authority as a locked decision and must not be contradicted by any plan or task in this phase:

- No purple gradients
- No pill-shaped buttons
- No fake reviews/testimonials
- No fake customer counters/metrics
- No vague hero text
- No emoji-as-icons
- No em dashes
- No over-the-top scroll animations
- No AI-slop photos
- No AI-slop copy
- No cursor animations

These apply to every onboarding screen (choose-method, verify-email, name, username, photo), the profile view, and the profile edit screen — button styling, copy tone, and any transition/animation choices for this phase's UI must be checked against this list.

## Summary

This phase is a standard "identity foundation" build: email/password auth plus Apple and Google sign-in, mandatory email verification, an Instagram-style multi-step onboarding sequence, persistent login via token storage, and an editable profile screen. Nothing here is exotic — every piece has a well-established, boring-on-purpose solution in the Go and Expo ecosystems, and the main planning risk is skipping a step (e.g., forgetting Apple Sign In is an App Store *requirement*, not a nice-to-have, once Google is offered — confirmed directly against Apple's own guideline text) or hand-rolling something (password hashing, JWT verification, OAuth token verification) that has a trusted library already.

On the backend: Gin (HTTP framework) + `golang.org/x/crypto/bcrypt` (password hashing) + `golang-jwt/jwt/v5` (access tokens) + `jackc/pgx/v5` (Postgres driver) + `golang-migrate/migrate/v4` (schema migrations) is the standard, current (2026) Go stack for this kind of API. Social sign-in verification is done server-side by validating provider-issued tokens against each provider's public keys — Google via the official `google.golang.org/api/idtoken` package, Apple via JWKS fetched from `https://appleid.apple.com/auth/keys` (paired with `golang-jwt` and, optionally, a JWKS-caching helper like `MicahParks/keyfunc`). None of this requires hand-rolling cryptography.

On the client: Expo SDK 57 managed workflow with `expo-router` for file-based, auth-state-gated navigation (an `(auth)` route group vs. an `(app)` route group, switched by a root layout that reads persisted session state), `expo-secure-store` for token storage (Keychain/Keystore-backed), `expo-apple-authentication` for Sign in with Apple, and `@react-native-google-signin/google-signin` for Google sign-in. The Google library requires a custom dev client (EAS Build) — it does not work in Expo Go — which is already compatible with this project's stated EAS Build/Update stack.

**Primary recommendation:** Build the backend as Gin + bcrypt + golang-jwt/v5 (access token) + a DB-backed opaque refresh token (for revocation) + pgx/v5 + golang-migrate, and build the client onboarding as an `expo-router` auth flow with `expo-secure-store` for token persistence, `expo-apple-authentication` and `@react-native-google-signin/google-signin` for social sign-in, verifying both provider tokens server-side before minting the app's own session.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Email/password signup & login | API/Backend (Go) | Database (Postgres) | Password hashing, credential checks, and token issuance must happen server-side; DB stores the user record |
| Apple / Google sign-in | Mobile Client (Expo/RN) | API/Backend (Go) | Client collects the provider's identity token via native SDK; backend re-verifies that token against the provider's public keys before trusting it and minting a session — never trust the client's word alone |
| Email verification | API/Backend (Go) | Database (Postgres) | Token generation, expiry, and single-use consumption are server-side; DB stores the hashed token + expiry |
| Session issuance & persistence | API/Backend (Go) issues; Mobile Client stores | Database (Postgres) | Backend issues access+refresh tokens and stores refresh-token state for revocation; client persists tokens in SecureStore and attaches them to requests |
| Session restore on app relaunch | Mobile Client (Expo/RN) | API/Backend (Go) | Client reads SecureStore on launch and calls the refresh endpoint; backend validates/rotates the refresh token |
| Profile view/edit (avatar, name, username, bio) | API/Backend (Go) | Object Storage (S3-compatible) + Mobile Client | Backend validates and persists profile fields; avatar binary goes to S3-compatible storage; client renders the form and picks/crops the photo locally before upload |
| Username uniqueness check & suggestion | API/Backend (Go) | Database (Postgres) | Uniqueness must be enforced at the DB (unique constraint) and checked server-side; client only displays the suggestion and shows real-time availability by calling the backend |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/gin-gonic/gin` | v1.12.0 [CITED: gin-gonic.com] | Go HTTP router/middleware for the REST/JSON API | Radix-tree routing, built-in JSON binding+validation, panic recovery, route grouping for versioning/auth — the most widely adopted Go web framework, API-frozen for stability |
| `golang.org/x/crypto/bcrypt` | x/crypto v0.57.0 [VERIFIED: pkg.go.dev / proxy.golang.org] | Password hashing | Official Go extended-crypto package; purpose-built adaptive hashing with a tunable cost factor — the de facto standard for Go password storage |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 [VERIFIED: proxy.golang.org] | Signing/verifying the app's own access tokens, and verifying Apple's identityToken JWT | Most widely used Go JWT library; v5 tightened default validation (algorithm confusion protection) vs. v4 |
| `github.com/jackc/pgx/v5` | v5.11.0 [VERIFIED: proxy.golang.org] | Postgres driver | Actively maintained, faster and more feature-complete than `lib/pq`, which is in maintenance-only mode [ASSUMED — widely known Go ecosystem consensus, not re-verified this session] |
| `github.com/golang-migrate/migrate/v4` | v4.20.1 [CITED: github.com/golang-migrate/migrate] | Postgres schema migrations | Standard, actively maintained (18.9k stars) Go migration tool; simple up/down SQL file pairs, CLI + library modes |
| `expo` (Expo SDK) | 57.0.22 [VERIFIED: npm registry] | Managed workflow runtime | Already the project's locked mobile stack (PROJECT.md); SDK 57 is current as of research date |
| `expo-router` [WARNING: flagged as suspicious — verify before using.] | 57.0.21 [ASSUMED — recommended based on training knowledge; VERIFIED: npm registry for version/currency] | File-based navigation with auth-gated route groups | Current default navigation solution bundled with new Expo projects; a root layout can redirect between an `(auth)` group and an `(app)` group based on session state, which maps cleanly onto the D-05 multi-step onboarding sequence |
| `expo-secure-store` [WARNING: flagged as suspicious — verify before using.] | 57.0.4 [CITED: docs.expo.dev] | On-device encrypted storage for access/refresh tokens | iOS Keychain / Android Keystore-backed; explicitly the discretion item named in CONTEXT.md for session persistence |
| `expo-apple-authentication` [WARNING: flagged as suspicious — verify before using.] | 57.0.2 [CITED: docs.expo.dev] | Sign in with Apple on iOS | First-party Expo module; returns `identityToken` (JWT), `authorizationCode`, and a stable user id for server-side verification. iOS-only — gate the sign-in button on `Platform.OS === 'ios'` (and/or `AppleAuthentication.isAvailableAsync()`); Android users get email/password + Google only |
| `@react-native-google-signin/google-signin` [WARNING: flagged as suspicious — verify before using.] | 16.1.5 [CITED: docs.expo.dev (Expo's own Google-auth guide names this as its primary recommendation)] | Google sign-in | Native Google Sign-In SDK wrapper; Expo's official guide recommends this over `expo-auth-session`'s web-based OAuth flow. Requires a custom dev client (EAS Build) — incompatible with Expo Go, but this project already builds with EAS |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `google.golang.org/api` (idtoken subpackage) | v0.298.0 [VERIFIED: proxy.golang.org — parent module only] / `idtoken` package path and `idtoken.Validate(ctx, token, clientID)` signature [ASSUMED — training knowledge, not independently confirmed; a direct lookup of `proxy.golang.org/google.golang.org/api/idtoken/@latest` returned "not found" because subpackages aren't separately versioned, so confirm the exact import path and function signature against current godoc.org/pkg.go.dev at implementation time] | Server-side verification of Google's ID token | Official Google client library module is confirmed to exist and be current; avoids hand-rolling JWKS/signature checks for Google once the exact API is confirmed |
| `github.com/MicahParks/keyfunc/v3` | v3.8.2 [VERIFIED: proxy.golang.org] | JWKS fetching/caching for Apple's public keys | Pairs with `golang-jwt` to verify Apple's `identityToken` against `https://appleid.apple.com/auth/keys` without hand-rolling JWKS caching/rotation logic [ASSUMED common pairing — training knowledge, package existence verified this session] |
| `github.com/go-playground/validator/v10` | v10.30.4 [VERIFIED: proxy.golang.org] | Struct-tag based request validation | Integrates directly with Gin's JSON binding (`binding:"required,email"` etc.) for signup/login/profile-update payload validation |
| `github.com/sqlc-dev/sqlc` | v1.31.1 [VERIFIED: proxy.golang.org] | Optional typed-SQL codegen from `.sql` files | Optional — use if the team wants compile-time-checked queries instead of hand-written `pgx` calls; adds a codegen build step. Not required for Phase 1's small query surface (Claude's discretion) |
| `expo-image-picker` | 57.0.17 [VERIFIED: npm registry] | Profile photo selection for avatar upload | Already implied by PROJECT.md's mobile stack; used for the optional signup-time photo (D-06) and profile-edit photo (D-08) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Gin | Echo or Fiber | Echo is very similar in ergonomics/performance; Fiber (fasthttp-based) is fastest but breaks compatibility with some standard `net/http` middleware. Gin has the largest ecosystem and is a safe, boring default — no reason to deviate given no explicit constraint |
| `jackc/pgx/v5` | `lib/pq` + `database/sql` | `lib/pq` is simpler/more familiar but effectively in maintenance mode; pgx has better native Postgres type support and performance [ASSUMED] |
| DB-backed opaque refresh token | Long-lived JWT refresh token (self-contained, no DB lookup) | Opaque + DB-backed allows revoking a session (e.g., "log out everywhere," compromised device) — a self-contained JWT refresh token cannot be revoked before its expiry without a blocklist, which just reintroduces server-side state anyway |
| `@react-native-google-signin/google-signin` | `expo-auth-session` (web-based OAuth, works in Expo Go) | `expo-auth-session` avoids the custom-dev-client requirement, but Expo's own current guidance recommends the native library for a better sign-in UX; since this project already uses EAS Build (per PROJECT.md), the dev-client requirement is not a real cost |
| Third-party Apple-Sign-In wrapper (e.g. `Timothylock/go-signin-with-apple`) | `golang-jwt` + `keyfunc` (manual JWKS verification) | The wrapper bundles convenience helpers but is a much smaller, less-audited dependency for something security-sensitive; verifying manually with two well-established libraries (JWT parsing + JWKS caching) keeps the trust surface smaller [ASSUMED — judgment call, not a verified security audit] |

**Installation:**
```bash
# Backend (Go)
go get github.com/gin-gonic/gin@v1.12.0
go get golang.org/x/crypto@v0.57.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get github.com/jackc/pgx/v5@v5.11.0
go get github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1
go get google.golang.org/api@v0.298.0
go get github.com/MicahParks/keyfunc/v3@v3.8.2
go get github.com/go-playground/validator/v10@v10.30.4

# Mobile (Expo)
npx expo install expo-router expo-secure-store expo-apple-authentication expo-image-picker
npm install @react-native-google-signin/google-signin
```

**Version verification:** All versions above were checked against `proxy.golang.org/<module>/@latest` (Go) and `npm view <pkg> version` (Expo/npm) on the research date (2026-09-15). Re-verify before implementation if planning is delayed by more than a couple weeks — the Go module proxy and npm registry are authoritative for currency, training-data versions are not.

## Package Legitimacy Audit

> Note on methodology: the `package-legitimacy check` seam covers npm/PyPI/crates ecosystems only. Go modules were verified manually against `proxy.golang.org/<module>/@latest` (the authoritative Go module proxy) — existence and current version are VERIFIED, but the seam's heuristic downloads/age/SLOP-SUS scoring does not apply to Go modules, so no verdict column value is fabricated for them; they're marked "VERIFIED (manual)" instead.

| Package | Registry | Age/Signal | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----------|-----------|--------------|---------|-------------|
| `expo-apple-authentication` | npm | "too-new" heuristic (latest patch published 2026-09-11 — Expo monorepo republishes very frequently) | 1.2M/wk | github.com/expo/expo | SUS | Flagged — heuristic false positive (see note below); planner should add a lightweight `checkpoint:human-verify` before install per protocol |
| `expo-secure-store` | npm | "too-new" heuristic (latest patch 2026-09-11) | 4.4M/wk | github.com/expo/expo | SUS | Flagged — same false-positive pattern; planner adds checkpoint |
| `expo-router` | npm | "too-new" heuristic (latest patch 2026-09-11) | 4.7M/wk | github.com/expo/expo | SUS | Flagged — same false-positive pattern; planner adds checkpoint |
| `@react-native-google-signin/google-signin` | npm | "too-new" heuristic (latest patch 2026-09-03) | 729K/wk | github.com/react-native-google-signin/google-signin | SUS | Flagged — same false-positive pattern; planner adds checkpoint |
| `expo-auth-session` | npm | "too-new" heuristic | 1.4M/wk | github.com/expo/expo | SUS | Not used as primary recommendation (see Alternatives Considered); if planner substitutes it, same checkpoint applies |
| `expo-image-picker` | npm | (not run through seam — first-party Expo package already implied by PROJECT.md stack) | — | github.com/expo/expo | — | Already an accepted project dependency; no new audit needed |
| `github.com/gin-gonic/gin` | Go module proxy | Stable, API-frozen since v1.x | — (Go proxy has no download-count signal) | github.com/gin-gonic/gin | VERIFIED (manual) | Approved |
| `golang.org/x/crypto` | Go module proxy | Official Go sub-repo, maintained by the Go team | — | go.googlesource.com/crypto | VERIFIED (manual) | Approved |
| `github.com/golang-jwt/jwt/v5` | Go module proxy | Actively maintained, standard library-adjacent | — | github.com/golang-jwt/jwt | VERIFIED (manual) | Approved |
| `github.com/jackc/pgx/v5` | Go module proxy | Actively maintained | — | github.com/jackc/pgx | VERIFIED (manual) | Approved |
| `github.com/golang-migrate/migrate/v4` | Go module proxy | 18.9k GitHub stars, active CI | — | github.com/golang-migrate/migrate | VERIFIED (manual) | Approved |
| `google.golang.org/api` | Go module proxy | Official Google-maintained client library | — | github.com/googleapis/google-api-go-client | VERIFIED (manual) | Approved |
| `github.com/MicahParks/keyfunc/v3` | Go module proxy | Established, narrowly-scoped JWKS helper | — | github.com/MicahParks/keyfunc | VERIFIED (manual) | Approved — lower-traffic than the above, planner may still want a quick review before pinning |
| `github.com/go-playground/validator/v10` | Go module proxy | Extremely widely used in Go REST APIs | — | github.com/go-playground/validator | VERIFIED (manual) | Approved |
| `github.com/sqlc-dev/sqlc` | Go module proxy | Actively maintained codegen tool | — | github.com/sqlc-dev/sqlc | VERIFIED (manual) | Approved — optional, Claude's discretion whether to use |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** `expo-apple-authentication`, `expo-secure-store`, `expo-router`, `@react-native-google-signin/google-signin`, `expo-auth-session` (if used). **Important context for the planner:** all five are official Expo (`expo/expo` monorepo) or well-known community (`react-native-google-signin`) packages with 700K–4.7M weekly downloads. The "too-new" signal fires because Expo republishes every package in its monorepo on nearly every SDK patch release, making `publishedAt` (latest version) look recent — it does not reflect the package's actual age or trustworthiness. Per protocol these are still flagged; the `checkpoint:human-verify` the planner inserts should be resolvable in seconds by confirming the repo is `github.com/expo/expo` (or the named community org) and download counts are in the hundreds-of-thousands-to-millions range.

## Architecture Patterns

### System Architecture Diagram

```
┌───────────────────────────────┐
│   Mobile Client (Expo/RN)      │
│   expo-router: (auth) screens  │
│   choose-method → verify-email │
│   → name → username → photo    │
│   expo-secure-store: tokens    │
└───────────────┬─────────────────┘
                │ HTTPS/JSON
                ▼
┌────────────────────────────────────────────┐        ┌──────────────────────┐
│  Go REST API (Gin)                          │───────▶│ Apple ID servers      │
│  POST /auth/signup                          │        │ (JWKS: verify         │
│  POST /auth/login                           │◀───────│  identityToken)       │
│  POST /auth/oauth/apple                     │        └──────────────────────┘
│  POST /auth/oauth/google                    │        ┌──────────────────────┐
│  POST /auth/verify-email                    │───────▶│ Google idtoken.Validate│
│  POST /auth/refresh                         │◀───────│ (verify ID token)      │
│  GET/PATCH /me   (profile)                  │        └──────────────────────┘
│  GET /usernames/suggest?name=..             │───────▶┌──────────────────────┐
└──────┬───────────────┬──────────────┬────────┘        │ Email provider (TBD) │
       │               │              │                 │ sends verify link    │
       ▼               ▼              ▼                 └──────────────────────┘
┌─────────────┐  ┌───────────────┐  ┌─────────────────────┐
│  Postgres    │  │ S3-compatible │  │ (in Postgres) refresh │
│  users,      │  │ object storage│  │ _tokens table for     │
│  email_verif │  │ (avatar photos)│  │ revocation support    │
│  _tokens     │  └───────────────┘  └─────────────────────┘
└─────────────┘
```

Primary use-case trace (signup → logged-in profile view): client posts to `/auth/signup` → API hashes password with bcrypt, creates `users` row with `email_verified=false`, generates a single-use verification token, hands it to the email provider → user taps the emailed link → client calls `/auth/verify-email` → API flips `email_verified=true` and consumes the token → onboarding continues client-side (name → username-suggest calls `/usernames/suggest` → optional photo upload to S3-compatible storage) → API issues access+refresh tokens → client stores both in SecureStore → client calls `GET /me` to render the profile screen. On every future app launch, the root layout reads SecureStore, calls `/auth/refresh`, and routes to the `(app)` group on success or the `(auth)` group on failure/absence.

### Recommended Project Structure

**Backend (Go):**
```
cmd/
└── api/
    └── main.go           # wiring: config, DB pool, router, graceful shutdown
internal/
├── auth/                 # password hashing, JWT issuance/verification, OAuth token verification (Apple/Google)
├── httpapi/              # Gin handlers + route registration, grouped by resource (auth, profile, username)
├── middleware/            # auth middleware (parses+validates access token), request logging, recovery
├── user/                  # domain model + repository interface for User
├── store/postgres/        # pgx-backed repository implementations
└── mail/                  # email-sending abstraction (provider TBD — see Open Questions)
migrations/                 # golang-migrate .up.sql / .down.sql pairs
```

**Mobile (Expo Router):**
```
app/
├── _layout.tsx            # root layout: reads SecureStore, redirects to (auth) or (app)
├── (auth)/
│   ├── _layout.tsx         # auth-stack layout, redirects to (app) if already logged in
│   ├── choose-method.tsx   # email/phone+password vs Apple vs Google entry point
│   ├── verify-email.tsx
│   ├── name.tsx
│   ├── username.tsx        # auto-suggested + editable, live uniqueness check
│   └── photo.tsx            # optional/skippable
└── (app)/
    ├── _layout.tsx          # app-stack layout, requires session
    └── profile/
        ├── index.tsx        # view own profile
        └── edit.tsx          # edit avatar/name/username/bio
lib/
├── api/                     # typed fetch client, attaches access token, handles 401→refresh→retry
└── session/                  # SecureStore read/write, session context/store
```

### Pattern 1: Access token + DB-backed refresh token
**What:** Short-lived signed JWT (e.g. 15 min) as the access token for API auth; a longer-lived opaque random token (e.g. 30 days) stored hashed in Postgres as the refresh token, tied to a user + device/session row.
**When to use:** Any mobile app needing "stay logged in across app sessions" (ACCT-01) while retaining the ability to revoke a session server-side.
**Example:**
```go
// Source: pattern synthesized from golang-jwt/jwt v5 docs + standard refresh-token practice [ASSUMED]
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

### Pattern 2: Server-side verification of third-party identity tokens
**What:** Client obtains an identity/ID token from Apple or Google's native SDK; client sends that raw token to the backend; backend independently verifies it against the provider's public keys before creating/logging in the user — never trusts client-asserted identity.
**When to use:** Every OAuth/social sign-in flow (D-01, D-02).
**Example:**
```go
// Source: [ASSUMED] — google.golang.org/api parent module VERIFIED to exist (proxy.golang.org),
// but the idtoken subpackage import path and this exact function signature are from training
// knowledge only; confirm against current pkg.go.dev before implementing.
payload, err := idtoken.Validate(ctx, googleIDToken, googleClientID)
if err != nil { /* reject */ }
email := payload.Claims["email"].(string)
```
```go
// Apple: fetch/cache JWKS from https://appleid.apple.com/auth/keys via keyfunc,
// then parse+verify identityToken with golang-jwt, checking iss=="https://appleid.apple.com"
// and aud==<your Apple Service ID / bundle ID>.
```

### Pattern 3: Auth-gated routing with Expo Router
**What:** A root `_layout.tsx` reads session state (from SecureStore, hydrated into memory/context on launch) and conditionally renders/redirects between an `(auth)` route group and an `(app)` route group.
**When to use:** Any Expo Router app needing to gate whole sections of the app behind login — directly supports "staying logged in across app sessions" (ACCT-01) as a UX-level concern, not just a token-storage concern.
**Example:**
```tsx
// Source: docs.expo.dev/router/advanced/authentication/ [CITED] — Stack.Protected with a
// boolean guard prop is the officially documented API for this pattern. The official example
// wraps a SessionProvider + SplashScreenController around the root layout; abbreviated here.
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

### Anti-Patterns to Avoid
- **Trusting the client's claimed Apple/Google identity without server verification:** the client can be tampered with; always re-verify the identity token server-side before issuing your own session.
- **Storing the access token only, with no refresh mechanism:** forces users to re-enter credentials frequently, contradicting "stays logged in across app sessions" (ACCT-01) — and long-lived access tokens that never expire are a security liability if leaked.
- **Blank username field at signup:** explicitly rejected by D-05 — must be pre-filled with a suggestion derived from the entered name, with live uniqueness checking, not a scratch field.
- **Hand-rolled JWT signature verification (parsing the JWT manually, checking `alg` field yourself):** a well-known source of "algorithm confusion" vulnerabilities (e.g. accepting `alg: none`); always use a maintained library with algorithm allow-listing (golang-jwt/v5 does this by default).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom salted-SHA256 or similar | `golang.org/x/crypto/bcrypt` | bcrypt is adaptive (tunable cost), constant-time compare, and battle-tested; custom hashing schemes are a top source of real-world credential breaches |
| JWT signing/verification | Manual base64+HMAC construction | `golang-jwt/jwt/v5` | Handles algorithm confusion protection, claim validation, and edge cases (clock skew, `alg: none` attacks) that are easy to get subtly wrong by hand |
| Apple/Google identity token verification | Manually parsing the JWT payload and trusting it, or writing your own JWKS fetch/cache | `google.golang.org/api/idtoken` (Google) + `golang-jwt` + `MicahParks/keyfunc` (Apple JWKS) | Signature verification against rotating provider public keys is exactly the kind of crypto plumbing that's easy to get wrong (stale keys, missing `aud`/`iss` checks) and has maintained libraries already |
| Secure on-device token storage | Plain AsyncStorage or a hand-rolled encryption layer | `expo-secure-store` | AsyncStorage is unencrypted; SecureStore wraps the OS-native Keychain/Keystore, which is the platform-sanctioned place for credentials |
| Profile photo capture/selection/cropping | Custom camera roll integration | `expo-image-picker` (+ `expo-image` for display, already in PROJECT.md's stack) | Already the project's locked photo stack; no reason to duplicate for avatars specifically |
| Database schema migrations | Hand-run SQL scripts tracked manually | `golang-migrate/migrate` | Ordered, reversible, versioned migrations with a `schema_migrations` tracking table — avoids drift between environments |

**Key insight:** Every "Don't Hand-Roll" item above is security- or platform-integration-sensitive. In an auth phase, the cost of a subtle hand-rolled bug (broken token verification, weak hashing, unencrypted secrets) is far higher than the cost of adding a well-known dependency — this is the one phase where "just use the library" is not optional-nice-to-have but close to a correctness requirement.

## Common Pitfalls

### Pitfall 1: bcrypt's 72-byte password length limit
**What goes wrong:** `bcrypt.GenerateFromPassword` returns `ErrPasswordTooLong` for passwords over 72 bytes (not characters — multi-byte UTF-8 passwords hit the limit sooner).
**Why it happens:** It's a hard algorithmic constraint of bcrypt itself, not a library bug [VERIFIED: pkg.go.dev bcrypt docs].
**How to avoid:** Cap password length at signup (e.g. require 8-72 bytes, reject longer with a clear validation error) — the DTO example below already does this via `max=72`. Don't attempt a pre-hash workaround (e.g. SHA-256 before bcrypt); the cap alone is simpler and has no failure modes to get wrong.
**Warning signs:** Users with long or emoji-heavy passwords silently failing to log in after a successful-looking signup.

### Pitfall 1b: Apple only returns name/email on the user's *first* authorization
**What goes wrong:** `AppleAuthentication.signInAsync()` returns `fullName` and `email` only on the very first authorization for a given Apple ID + app (bundle ID) pair. Every subsequent call for the same user returns `null` for both fields — Apple does not re-send them.
**Why it happens:** This is Apple's documented privacy-preserving behavior for Sign in with Apple, not a bug or a token-scope misconfiguration.
**How to avoid:** Persist `fullName` and `email` to the `users` row on the very first callback, before the user could conceivably sign out and back in. Design the D-05 onboarding sequence so the "enter name" step (D-05 step 3) pre-fills from this first-authorization payload for Apple signups, since a second attempt to fetch it will silently come back empty. During development, be aware that reinstalling the app does *not* reset this — the user must manually revoke the app's access in iOS Settings → Apple ID → Sign in with Apple to get first-authorization behavior again for testing.
**Warning signs:** Apple sign-in users end up with a blank/null name and email in the database on any authorization after the first.

### Pitfall 2: Requiring Apple Sign In as an afterthought, or configuring it incorrectly
**What goes wrong:** Teams often add Google first, ship, and treat Apple Sign In as a "nice to have" added later — but Apple Review Guideline 4.8 makes it a hard *rejection* reason once any third-party/social login is offered for the primary account [CITED: developer.apple.com guideline 4.8].
**Why it happens:** Guideline 4.8 is easy to miss until an actual App Store submission gets rejected.
**How to avoid:** Build both sign-in options in Phase 1 itself (already the plan per D-02), and make sure the Apple option meets the guideline's three equivalence requirements (name+email only, private-relay-email option, no ad tracking without consent) rather than just "any Apple button."
**Warning signs:** Google sign-in ships without a working Apple sign-in button on iOS.

### Pitfall 3: Verifying only on the client, not the server, for social sign-in
**What goes wrong:** Trusting the mobile app's claim of "this Apple/Google response was valid" without the backend independently re-verifying the token's signature and claims — allows a modified client (or a replayed/forged token) to impersonate any user.
**Why it happens:** It's tempting to skip server verification because the native SDK "already validated" on-device — but on-device validation only proves the *client* trusts it, not that the *server* should.
**How to avoid:** Always send the raw identity/ID token to the backend and verify signature + `iss` + `aud` + `exp` there before minting a session (Pattern 2 above).
**Warning signs:** The backend's OAuth endpoints accept a `{provider, email, name}` payload from the client instead of the raw provider token.

### Pitfall 4: Username auto-suggestion collisions and race conditions
**What goes wrong:** Two users entering similar names concurrently can both pass a "is username available" check and then both try to claim it, or the naive `name1`, `name2`, `name3` incrementing suffix scheme becomes slow/predictable at scale.
**Why it happens:** A read-then-write availability check without a DB-level constraint has a race window.
**How to avoid:** Back the check with a case-insensitive uniqueness constraint in Postgres — either a `citext` column type or a `UNIQUE` index on `lower(username)` (same treatment for `email`, so `User@x.com` and `user@x.com` can't both register) — not a plain `UNIQUE` constraint on `username` as typed, which would let `Swathi` and `swathi` coexist as distinct accounts. Treat the live "is it available" endpoint as a UX convenience, not the source of truth — handle the unique-constraint-violation error at insert time as the authoritative rejection, and retry with the next suggestion. [ASSUMED — standard pattern, not tied to a specific citation]
**Warning signs:** Occasional duplicate-username insert failures in production despite a "green" availability check in the UI.

### Pitfall 5: Email verification token reuse or long-lived tokens
**What goes wrong:** A verification token that never expires or can be reused stays valid in old emails indefinitely, and a leaked email (e.g. forwarded, or a compromised mail account) lets someone verify/hijack the account later.
**Why it happens:** Easy to implement the "happy path" (generate token, send, verify) and skip expiry/single-use enforcement.
**How to avoid:** Store the token hashed (not plaintext) with an expiry (e.g. 24h) in a dedicated table, mark it consumed on successful verification, and reject already-consumed or expired tokens. [ASSUMED — standard pattern]
**Warning signs:** Verification links still work weeks after signup, or work more than once.

## Code Examples

### Signup handler skeleton (Go/Gin)
```go
// Source: pattern synthesized from Gin binding/validator docs + bcrypt docs [CITED/ASSUMED mix — see inline]
type SignupRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt's 72-byte ceiling, see Pitfall 1
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
    token := h.mail.SendVerificationEmail(c.Request.Context(), user) // generates + stores hashed token
    c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
}
```

### Username suggestion endpoint
```go
// Source: [ASSUMED] standard normalize-and-suffix pattern, not tied to a specific citation
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
        candidate = fmt.Sprintf("%s%d", base, randSuffix()) // e.g. random 1-4 digit suffix, not sequential
    }
    return "", ErrNoAvailableUsername // extremely unlikely; fall back to a longer random suffix
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `lib/pq` as the default Go Postgres driver | `jackc/pgx/v5` | pgx has been the community-recommended driver for several years now [ASSUMED] | Better performance, native Postgres type support; `lib/pq` still works but receives minimal new development |
| `expo-auth-session` web-based OAuth for Google sign-in | Native `@react-native-google-signin/google-signin` (or the newer `react-native-nitro-google-signin`) | Reflects Expo's current (2026) official Google-auth guide [CITED: docs.expo.dev] | Better native sign-in UX, but requires a custom dev client instead of Expo Go — a non-issue here since the project already uses EAS Build |
| golang-jwt v4 | golang-jwt v5 | v5.0.0 release | Tightened default claim validation, breaking API changes from v4 — make sure any copied v4-era code samples are updated to v5 syntax |

**Deprecated/outdated:**
- `lib/pq`: not deprecated outright, but effectively in maintenance mode — avoid for new projects [ASSUMED].
- Manually parsing JWTs without an `alg` allow-list: an outdated/unsafe pattern from early JWT-library eras; modern libraries (golang-jwt v5) default to safer behavior — don't copy older tutorial code that skips this.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | Gin recommended over Echo/Fiber as the Go web framework | Standard Stack / Alternatives | Low — all three are viable; swapping frameworks mid-build is mechanical but wasted effort if the planner has a different preference |
| A2 | `pgx/v5` preferred over `lib/pq` because `lib/pq` is "in maintenance mode" | Standard Stack | Low — both work; claim not re-verified against `lib/pq`'s repo activity this session |
| A3 | Access-token (short JWT) + DB-backed opaque refresh-token pattern for session persistence | Architecture Patterns (Pattern 1) | Medium — this is the specific mechanism CONTEXT.md left to Claude's discretion; if the planner wants a simpler single-long-lived-JWT approach instead, revocation ("log out everywhere") becomes harder later |
| A4 | Username auto-suggestion algorithm (normalize name → strip chars → numeric-suffix-on-collision) | Architecture Patterns (Pattern 3) / Code Examples | Low-Medium — functionally sound but not Instagram's actual (proprietary, unpublished) algorithm; if the planner wants exact Instagram parity, this is directionally right but not verified against Instagram's real behavior |
| A5 | Email verification via single-use, hashed, expiring opaque token (not JWT) emailed as a deep link | Architecture Patterns / Common Pitfalls | Low — standard, widely-used pattern |
| A6 | Transactional email provider not yet selected — no specific SES/Postmark/Resend recommendation made | Open Questions / Environment Availability | Medium — blocks the actual "send verification email" implementation until an account/API key exists; needs a decision before/during planning |
| A7 | `sqlc` offered as optional codegen layer, not required | Standard Stack (Supporting) | Low — purely a discretionary convenience choice |
| A8 | Manual JWKS verification (golang-jwt + keyfunc) recommended over a third-party Apple-Sign-In wrapper library | Standard Stack (Alternatives) / Don't Hand-Roll | Low-Medium — judgment call favoring fewer/more-audited dependencies over convenience; not a formal security audit of the alternative library |
| A9 | `expo-router` recommended as the navigation solution (not explicitly locked by CONTEXT.md or PROJECT.md) | Standard Stack / Architecture Patterns | Low — it's the current Expo default and fits the auth-gated-routing need well, but the planner could choose React Navigation directly instead with no fundamental difference |
| A10 | D-04's email-verification gate applies only to password signups; Apple/Google signups are treated as pre-verified via `email_verified_via` | Open Questions #1 | Medium-High — if wrong, the planner either builds a redundant verification-email send to (possibly) Apple private-relay addresses (triggering an unplanned Apple sender-domain-registration dependency), or ships a security gap by skipping verification where the user actually wanted it required. Confirm with the user before locking this into the plan |

## Open Questions

1. **Does the D-04 email-verification gate apply to Apple/Google signups, or only password signups?**
   - What we know: D-04 requires email verification before app use. D-01/D-02 add Apple and Google sign-in, and both providers independently verify the email address before ever handing it to your app — Apple guarantees the associated email is a real, reachable address (or a working `@privaterelay.appleid.com` forwarding address); Google's ID token includes a verified `email_verified` claim.
   - What's unclear: CONTEXT.md's D-05 sequence lists "verify email" as step 2 of the onboarding flow generically, without saying whether that step still runs (and blocks) for social signups, where it would be redundant — and if a verification email is actually sent to an Apple private-relay address, Apple requires the sending domain to be registered in the Apple Developer console first (a new provisioning dependency, not just an email-provider credential).
   - Recommendation: Treat provider-verified emails as already verified — set `email_verified=true` at account-creation time for Apple/Google signups (using the provider's own verification, not sending a redundant email), and reserve the "verify email" step (and the token-based flow in Architecture Patterns/Common Pitfalls) for password signups only. This has a schema consequence: the `users` table needs to record *how* an email was verified (e.g. an `email_verified_via` enum: `password_flow` / `apple` / `google`), not just a boolean, so the onboarding router can skip step 2 correctly for social signups. Confirm this reading with the user during planning if there's any doubt — it changes both the state machine and whether Apple private-relay domain registration becomes a blocking dependency.

2. **Which transactional email provider sends the verification email?**
   - What we know: Email verification is required (D-04); the backend needs to send an email with a verification link/code.
   - What's unclear: No provider (AWS SES, Postmark, Resend, SendGrid, etc.) is named in PROJECT.md or CONTEXT.md, and none was found configured in this environment.
   - Recommendation: Treat as a Phase 1 planning decision — pick one (Resend and AWS SES are both common low-friction choices for a small-scale MVP), get API credentials before the first execution wave that needs to send real email, and design the `mail` package behind an interface so the concrete provider is swappable.

3. **Phone-number signup: fully out of scope, or a stubbed field?**
   - What we know: D-03 defers phone-number signup/SMS verification entirely for MVP.
   - What's unclear: Whether the signup UI should even show a "phone number" tab/option that's disabled, or omit it entirely (cleaner, matches "email-first" framing).
   - Recommendation: Omit phone signup entirely from the UI for Phase 1 — D-03 frames it as skipped, not "shown but disabled," and a disabled option adds UI complexity with no user value yet.

4. **Exact password strength rules.**
   - What we know: CONTEXT.md explicitly leaves this to standard implementation, not discussed.
   - What's unclear: Minimum length, complexity requirements.
   - Recommendation: A simple minimum-length rule (e.g. 8 characters minimum, 72-byte bcrypt ceiling as the hard max) is sufficient for a small friend-group MVP — avoid complex composition rules (uppercase/symbol requirements), which current security guidance (NIST 800-63B) treats as counterproductive. [ASSUMED — general security best-practice knowledge, not re-verified against NIST's current text this session]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go | Backend build/run | ✓ | go1.26.4 (darwin/arm64) | — |
| PostgreSQL | User/session/verification-token storage | ✓ (running locally) | 16.14 (Homebrew) | — |
| Node.js | Mobile app tooling | ✓ | v26.4.0 | — |
| npm | Mobile package management | ✓ | 11.17.0 | — |
| Expo/EAS CLI | Builds, custom dev client for Google sign-in | ✗ (not globally installed) | — | Use `npx expo` / `npx eas-cli` — no install needed, works fine as a fallback |
| Docker | Not strictly required for this phase | ✗ (not found) | — | Postgres already runs natively via Homebrew; Docker not needed unless the team wants containerized parity later |
| S3-compatible object storage account | Avatar photo upload (D-08) | Unknown — no credentials found in this environment | — | Must be provisioned (bucket + access keys) before the profile-photo-upload task can be executed; blocks that specific task, not the rest of the phase |
| Transactional email provider | Sending verification emails (D-04) | Unknown — not configured | — | See Open Questions #1 — must be selected and credentials provisioned before the verification-email-sending task can be executed |

**Missing dependencies with no fallback:**
- S3-compatible object storage credentials (blocks avatar upload implementation/testing)
- Transactional email provider account (blocks verification-email sending implementation/testing)

**Missing dependencies with fallback:**
- Expo/EAS CLI — use `npx` invocations, no global install required

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go: standard `testing` package + `net/http/httptest` (no third-party test framework needed for Go). Mobile: none detected yet — Jest is the standard choice for Expo/RN projects if component-level tests are wanted. |
| Config file | none — see Wave 0 |
| Quick run command | `go test ./... -run TestAuth -v` (backend); no mobile test command yet |
| Full suite command | `go test ./...` (backend) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| ACCT-01 | Signup creates a user with `email_verified=false` and a hashed password | unit | `go test ./internal/httpapi -run TestSignup -v` | ❌ Wave 0 |
| ACCT-01 | Login with correct credentials returns access+refresh tokens; wrong password rejected | unit | `go test ./internal/httpapi -run TestLogin -v` | ❌ Wave 0 |
| ACCT-01 | Refresh endpoint issues a new access token for a valid refresh token, rejects expired/revoked ones | unit | `go test ./internal/httpapi -run TestRefresh -v` | ❌ Wave 0 |
| ACCT-01 | Unverified user cannot access `(app)`-gated endpoints | integration | `go test ./internal/httpapi -run TestEmailVerificationGate -v` | ❌ Wave 0 |
| ACCT-03 | `GET /me` returns the caller's own profile only | unit | `go test ./internal/httpapi -run TestGetProfile -v` | ❌ Wave 0 |
| ACCT-03 | `PATCH /me` updates name/username/bio, rejects a taken username | unit | `go test ./internal/httpapi -run TestUpdateProfile -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -run <relevant test> -v`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full backend suite green before `/gsd-verify-work`. Manual UAT (device or simulator walkthrough of the full onboarding sequence, D-05) substitutes for automated mobile UI tests in Phase 1 given no mobile test framework exists yet — acceptable given the small scope, but flagged as a Wave 0 gap if the team wants automated coverage sooner.

### Wave 0 Gaps
- [ ] `internal/httpapi/*_test.go` — covers ACCT-01, ACCT-03 (httptest-based handler tests against a test Postgres instance or a repository mock)
- [ ] Test Postgres setup (either a `TEST_DATABASE_URL` pointing at a local/dockerized instance, or a mock repository layer) — needed before any handler test can run
- [ ] Go test framework: no install needed (`testing` is stdlib); consider `github.com/stretchr/testify` for assertion ergonomics if the team wants it [ASSUMED optional convenience, not required]
- [ ] Mobile test framework (Jest + `@testing-library/react-native`) — not set up; Phase 1 can ship with manual UAT only for the mobile UI, but this is a gap if automated mobile coverage becomes a priority

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|--------------------|
| V2 Authentication | yes | bcrypt password hashing (DefaultCost), rate-limit login/signup attempts, generic "invalid credentials" error (don't reveal whether email exists) |
| V3 Session Management | yes | Short-lived signed JWT access token + DB-backed rotating opaque refresh token; refresh tokens stored hashed; support revocation ("log out") by deleting/invalidating the refresh-token row |
| V4 Access Control | yes (minimal at this phase) | `GET/PATCH /me` must derive the target user from the authenticated token's subject claim, never from a client-supplied user ID — prevents one user editing another's profile |
| V5 Input Validation | yes | `go-playground/validator` struct tags on all request DTOs (email format, password length, username charset/length) |
| V6 Cryptography | yes | Never hand-roll — bcrypt for passwords (library), golang-jwt for token signing (library), TLS for all client-backend traffic (infra-level, not app code) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Credential stuffing / brute-force login | Spoofing | Rate-limit `/auth/login` per IP/account; generic error message on failure |
| JWT algorithm confusion (`alg: none` or HS/RS confusion) | Tampering | golang-jwt v5's default algorithm allow-listing; explicitly specify expected signing method when parsing |
| Forged social-login identity (client sends fabricated `{provider, email}` instead of a real provider token) | Spoofing | Always verify the raw provider token server-side (Pattern 2) — never accept client-asserted identity claims directly |
| Username/email enumeration via signup or username-availability endpoints | Information Disclosure | Rate-limit the username-suggestion/availability endpoint; for login, return the same generic error for "no such user" and "wrong password" |
| SQL injection via profile fields (name/username/bio) | Tampering | `pgx` parameterized queries exclusively — never string-concatenate SQL |
| Refresh token theft/replay | Spoofing / Elevation of Privilege | Rotate refresh tokens on each use (issue a new one, invalidate the old), store hashed (not plaintext) in Postgres, set a reasonable expiry (e.g. 30 days) |

## Sources

### Primary (HIGH confidence)
- None this session that reached the seam's "VERIFIED via authoritative source + tool" bar for a specific package recommendation — see CITED/VERIFIED tags inline for the closest equivalents (official docs fetched directly, and Go module proxy / npm registry existence checks).

### Secondary (MEDIUM confidence — official docs fetched directly via WebFetch, or registry-verified)
- `docs.expo.dev/versions/latest/sdk/apple-authentication/` — expo-apple-authentication setup, API, App Store requirement note
- `docs.expo.dev/versions/latest/sdk/securestore/` — expo-secure-store storage backing, size limits, API
- `docs.expo.dev/guides/google-authentication/` — Expo's official Google sign-in library recommendation and dev-client requirement
- `docs.expo.dev/router/advanced/authentication/` — Expo Router's documented `Stack.Protected` auth-gating pattern (Pattern 3 code example)
- `developer.apple.com/app-store/review/guidelines/#sign-in-with-apple` — full text of Guideline 4.8
- `pkg.go.dev/golang.org/x/crypto/bcrypt` — bcrypt API, cost parameters, 72-byte limit
- `github.com/golang-jwt/jwt` (repo README) — v5 status, signing methods, installation
- `github.com/golang-migrate/migrate` (repo README) — tool overview, CLI usage, maintenance status
- `gin-gonic.com/en/docs/introduction/` — Gin feature overview
- `proxy.golang.org/<module>/@latest` — version/currency verification for all Go modules listed in Standard Stack and Supporting tables
- `npm view <pkg> version` — version/currency verification for all npm/Expo packages listed

### Tertiary (LOW confidence — training knowledge, not verified this session; see Assumptions Log for the full list)
- pgx vs. lib/pq ecosystem preference (A2)
- Access+refresh token session architecture as "the" pattern for this use case (A3)
- Username auto-suggestion algorithm specifics (A4)
- Email verification token mechanics (A5)
- Password strength recommendation (Open Question #3)

## Metadata

**Confidence breakdown:**
- Standard stack (library choice/versions): MEDIUM-HIGH — package existence/versions verified directly against proxy.golang.org and npm registry; "why this is the standard choice" framing is training knowledge for the well-established libraries (Gin, bcrypt, pgx) and CITED for Expo-specific guidance (Apple/Google sign-in libraries, SecureStore)
- Architecture: MEDIUM — the session/token pattern, onboarding routing pattern, and username-suggestion algorithm are sound, widely-used designs but not pulled from a single canonical source this session (flagged in Assumptions Log)
- Pitfalls: MEDIUM-HIGH — bcrypt's 72-byte limit and Apple's Guideline 4.8 requirement are directly CITED from official docs; the remaining pitfalls (token verification, username races, email token reuse) are well-established but ASSUMED

**Research date:** 2026-09-15
**Valid until:** 2026-10-15 (30 days) for library/version specifics — re-check `proxy.golang.org`/npm before implementation if significantly delayed; Apple's Guideline 4.8 text and Expo's official Google-auth recommendation are policy/guidance documents that change less frequently but should still be spot-checked if planning is delayed by more than a month
