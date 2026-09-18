---
phase: 01-foundation-accounts
plan: 03
subsystem: database
tags: [postgres, pgx, golang-migrate, user-domain, repository-pattern, uuid]

requires:
  - phase: 01-foundation-accounts (plan 01)
    provides: Go module with pinned pgx/v5, google/uuid dependencies; local rndmroll_dev and rndmroll_test Postgres databases
provides:
  - Phase 1 Postgres schema (users, email_verification_tokens, refresh_tokens) with case-insensitive unique indexes
  - internal/user domain package: User, VerificationSource, OnboardingComplete, typed sentinel errors
  - internal/user repository interfaces: Repository, RefreshTokenRepository, EmailVerificationRepository, ProfilePatch, RefreshToken
  - internal/store/postgres: pgx-backed implementations (NewPool, NewUserRepo, NewRefreshTokenRepo, NewEmailVerificationRepo) with a 6-case integration test suite
affects: [01-05, 01-06, 01-08, 01-09, 01-10, 01-11]

tech-stack:
  added: []
  patterns:
    - "pgx positional placeholders exclusively; no fmt.Sprintf/string concatenation builds SQL anywhere in internal/store/postgres"
    - "UpdateProfile-style partial updates use coalesce($n::text, column) so the query string never varies with which fields are set"
    - "23505 unique-violation mapped to typed domain errors by ConstraintName, not by pre-checking availability first (insert-time mapping is authoritative)"
    - "TestMain shells out to the migrate CLI (resolving repo root via runtime.Caller, not cwd) then every test calls a requireTestPool(t) helper that calls t.Skip when TEST_DATABASE_URL is unset"

key-files:
  created:
    - migrations/0001_init.up.sql
    - migrations/0001_init.down.sql
    - internal/user/model.go
    - internal/user/repository.go
    - internal/store/postgres/db.go
    - internal/store/postgres/user_repo.go
    - internal/store/postgres/refresh_token_repo.go
    - internal/store/postgres/email_verification_repo.go
    - internal/store/postgres/postgres_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "email_verified_via records how an email was verified (password_flow/apple/google) per RESEARCH.md Open Question 1's recommendation, so the onboarding router can skip the redundant verification email for Apple/Google signups without taking on Apple private-relay sender-domain registration"
  - "REQUIREMENTS.md left unmodified for ACCT-01/ACCT-03, matching 01-01's precedent: schema + repository interfaces are not user-facing functionality (no HTTP endpoints exist yet), so marking them complete here would be a false positive in project-wide tracking"
  - "Rotate reads the old row and inserts the new refresh-token row before updating the old row's revoked_at/rotated_to, because rotated_to is a foreign key into refresh_tokens and must reference an existing row"
  - "ConsumeByHash compares expiry against the now passed in by the caller, not SQL now(), so the flow is deterministically testable; it also checks consumed_at before expiry so a consumed-and-expired token still reports ErrTokenConsumed"
  - "Every SQL query is a plain string literal (no shared constant + '+' concatenation, even for the repeated column list) to stay unambiguously inside the plan's 'no query is assembled with fmt.Sprintf, string concatenation, or a template' constraint"

patterns-established:
  - "Repository interfaces live in internal/<domain>/repository.go; pgx implementations live in internal/store/postgres/<domain>_repo.go with a New<Domain>Repo(pool) constructor returning the domain interface type"
  - "Case-insensitive uniqueness is a unique index on lower(column), not a citext column or a plain UNIQUE constraint"

requirements-completed: [ACCT-01, ACCT-03]

coverage:
  - id: D1
    description: "Phase 1 Postgres schema — users, email_verification_tokens, refresh_tokens tables with 4 case-insensitive unique indexes, bio/username check constraints, and an updated_at trigger — migrates up/down/up cleanly"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "make-equivalent migrate CLI: migrate -path migrations -database $DATABASE_URL up/down 1/up against rndmroll_dev, all exit 0"
        status: pass
      - kind: other
        ref: "psql: insert 'A@X.com' then 'a@x.com' into users fails with duplicate key on users_email_lower_idx; insert username 'Swathi' fails users_username_charset check"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/user domain package — User struct with pointer fields for nullable columns, VerificationSource enum, OnboardingComplete authority method, 9 sentinel errors including a singular ErrInvalidCredentials, and the Repository/RefreshTokenRepository/EmailVerificationRepository/ProfilePatch/RefreshToken contracts wave-4 plans code against"
    requirement: ACCT-01
    verification:
      - kind: other
        ref: "go build ./internal/user && go vet ./internal/user; grep checks for OnboardingComplete, ErrInvalidCredentials, and all 5 declared types"
        status: pass
    human_judgment: false
  - id: D3
    description: "pgx-backed UserRepo, RefreshTokenRepo, EmailVerificationRepo — every query is a bound-parameter literal, 23505 mapped to typed errors by constraint name, Rotate revokes-old/inserts-new in one transaction, ConsumeByHash is single-use"
    requirement: ACCT-01
    verification:
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestUserRepo_Create_CaseInsensitiveEmailConflict"
        status: pass
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestUserRepo_UpdateProfile_UsernameConflictReturnsErrUsernameTaken"
        status: pass
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestUserRepo_UpdateProfile_NilFieldLeavesColumnUnchanged"
        status: pass
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestRefreshTokenRepo_GetActiveByHash_ExpiredAndRevoked"
        status: pass
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestRefreshTokenRepo_Rotate_OldHashNoLongerResolves"
        status: pass
      - kind: integration
        ref: "internal/store/postgres/postgres_test.go#TestEmailVerificationRepo_ConsumeByHash_SecondCallReturnsErrTokenConsumed"
        status: pass
      - kind: other
        ref: "grep -c 23505 internal/store/postgres/user_repo.go (>=1); grep -rnE Sprintf/concatenation-building-SQL over internal/store/postgres/ (0 matches); go vet ./internal/store/..."
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 03: Schema, User Domain Model, and pgx Repositories Summary

**Postgres schema (users/email_verification_tokens/refresh_tokens with case-insensitive unique indexes), internal/user domain contracts, and pgx-backed repository implementations verified against a real database via a 6-case integration suite.**

## Performance

- **Duration:** 45 min
- **Started:** 2026-09-18T09:30:00Z (approx.)
- **Completed:** 2026-09-18T10:15:25Z
- **Tasks:** 3 (Task 3 executed as RED/GREEN TDD — no separate REFACTOR commit needed)
- **Files modified:** 11 (9 created, 2 modified)

## Accomplishments

- Phase 1 schema migration (`migrations/0001_init.up.sql`/`.down.sql`): `users`, `email_verification_tokens`, `refresh_tokens` tables, 4 case-insensitive unique indexes (email, username, apple_subject, google_subject), bio-length and username-charset check constraints, and an `updated_at` trigger — round-trips cleanly through up/down/up against `rndmroll_dev`
- `internal/user` domain package: `User` (pointer fields for nullable columns), `VerificationSource` enum, `(*User).OnboardingComplete()` as the sole authority for the onboarding-complete condition, 9 sentinel errors (singular `ErrInvalidCredentials` so login can't be used to enumerate registered emails), and the `Repository` / `RefreshTokenRepository` / `EmailVerificationRepository` interfaces wave-4 handler plans code against
- `internal/store/postgres`: pgx-backed implementations of all three interfaces — every query is a fixed bound-parameter literal (no `fmt.Sprintf`/concatenation), `23505` unique violations mapped to typed domain errors by constraint name, `UpdateProfile` uses `coalesce($n::text, column)` for partial updates, `Rotate` revokes-old/inserts-new in one transaction, `ConsumeByHash` is single-use and deterministically testable
- 6-case integration test suite in `postgres_test.go`, all passing against `TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable`, and cleanly skipping (not failing) when that variable is unset

## Task Commits

Each task was committed atomically:

1. **Task 1: Phase 1 schema migration** - `e16fc6a` (feat)
2. **Task 2: User domain model, typed errors, repository interfaces** - `9dc7ef0` (feat)
3. **Task 3 (RED): Add failing tests for pgx-backed repository implementations** - `64d59bf` (test)
3. **Task 3 (GREEN): Implement pgx-backed repositories** - `9113ae1` (feat)

**Plan metadata:** this commit (`docs(01-03)`)

## Files Created/Modified

- `migrations/0001_init.up.sql` / `.down.sql` - Phase 1 schema: users, email_verification_tokens, refresh_tokens, indexes, check constraints, updated_at trigger
- `internal/user/model.go` - `User`, `VerificationSource`, `OnboardingComplete`, sentinel errors
- `internal/user/repository.go` - `Repository`, `RefreshTokenRepository`, `EmailVerificationRepository`, `ProfilePatch`, `RefreshToken`
- `internal/store/postgres/db.go` - `NewPool`: pgxpool with MaxConns=10, 5s connect timeout, startup Ping
- `internal/store/postgres/user_repo.go` - `UserRepo` implementing `user.Repository`
- `internal/store/postgres/refresh_token_repo.go` - `RefreshTokenRepo` implementing `user.RefreshTokenRepository`
- `internal/store/postgres/email_verification_repo.go` - `EmailVerificationRepo` implementing `user.EmailVerificationRepository`
- `internal/store/postgres/postgres_test.go` - integration suite: `TestMain` runs `migrate` against `TEST_DATABASE_URL`, each test skips via `requireTestPool(t)` when unset
- `go.mod` / `go.sum` - added transitive entries (`github.com/jackc/puddle/v2`, `golang.org/x/sync`) required by `pgxpool`, already pinned via `pgx/v5` — see Deviations

## Final `internal/user/repository.go` Signatures (verbatim, for wave 4)

```go
type ProfilePatch struct {
	Name      *string
	Username  *string
	Bio       *string
	AvatarURL *string
}

type Repository interface {
	Create(ctx context.Context, email string, passwordHash *string, verified bool, via *VerificationSource) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmailCI(ctx context.Context, email string) (*User, error)
	GetByProviderSubject(ctx context.Context, provider VerificationSource, subject string) (*User, error)
	LinkProviderSubject(ctx context.Context, id uuid.UUID, provider VerificationSource, subject string) error
	UsernameTaken(ctx context.Context, username string) (bool, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID, via VerificationSource) error
	UpdateProfile(ctx context.Context, id uuid.UUID, p ProfilePatch) (*User, error)
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshTokenRepository interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error)
	// GetActiveByHash returns ErrTokenExpired for a past expires_at and
	// ErrTokenInvalid for a revoked or missing row.
	GetActiveByHash(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
	// Rotate revokes oldID and inserts a new row in one transaction, so a
	// crash mid-rotation cannot leave two simultaneously valid tokens.
	Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error)
	RevokeByHash(ctx context.Context, tokenHash []byte) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type EmailVerificationRepository interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	// ConsumeByHash atomically sets consumed_at and returns the owning
	// user ID, or ErrTokenConsumed / ErrTokenExpired / ErrTokenInvalid.
	ConsumeByHash(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error)
	// DeleteForUser is used when a resend supersedes outstanding tokens.
	DeleteForUser(ctx context.Context, userID uuid.UUID) error
}
```

Concrete constructors available from `internal/store/postgres`: `NewPool(ctx, databaseURL) (*pgxpool.Pool, error)`, `NewUserRepo(pool) user.Repository`, `NewRefreshTokenRepo(pool) user.RefreshTokenRepository`, `NewEmailVerificationRepo(pool) user.EmailVerificationRepository`.

## Decisions Made

See `key-decisions` in frontmatter. Summary: `email_verified_via` resolves RESEARCH.md's Open Question 1 by recording the verification mechanism per-provider; `Rotate` orders its transaction insert-then-update because `rotated_to` is a self-referencing foreign key; `ConsumeByHash` takes the current time as a parameter instead of using SQL `now()` so its expiry/consumed-order behavior is deterministic under test; every SQL query stayed a plain literal (no shared constant plus concatenation) to keep unambiguous compliance with the plan's ban on string-concatenated SQL; REQUIREMENTS.md checkboxes were left unmodified for the same reason 01-01 gave (no HTTP surface exists yet).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `migrate` CLI binary was missing the Postgres driver**
- **Found during:** Task 1 verify (`migrate -path migrations -database $DATABASE_URL up`)
- **Issue:** `error: failed to open database: database driver: unknown driver postgres (forgotten import?)` — the `migrate` binary at `$HOME/go/bin/migrate` (installed in plan 01-01) was built without the `postgres` build tag, so no database driver was compiled in.
- **Fix:** `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` — reinstalled the same tool (not a different package) with the correct build tag.
- **Verification:** `migrate ... up` / `down 1` / `up` round-trip against `rndmroll_dev` all succeed.
- **Commit:** not separately committed (no repo file changed; the fix is to a local `$HOME/go/bin` tool binary, outside the worktree).

**2. [Rule 3 - Blocking] `go.sum` missing transitive entries for `pgxpool`**
- **Found during:** Task 3 RED phase (`go build ./internal/store/postgres`)
- **Issue:** `missing go.sum entry for module providing package github.com/jackc/puddle/v2 (imported by github.com/jackc/pgx/v5/pgxpool)`. `pgx/v5` was already pinned in `go.mod` (added in plan 01-01) but `pgxpool` specifically hadn't been imported before, so its own transitive dependency (`puddle/v2`, pulling in `golang.org/x/sync`) had no `go.sum` hash yet.
- **Fix:** `go get github.com/jackc/pgx/v5/pgxpool@v5.11.0` — same version already in `go.mod`; this only added the two missing indirect `require` lines and their `go.sum` hashes, it did not change any existing dependency version. Deliberately did not run `go mod tidy` to avoid a broad, unplanned diff to `go.mod`.
- **Verification:** `go build ./...` and `go vet ./...` both clean afterward.
- **Files modified:** `go.mod`, `go.sum` (bundled into the Task 3 RED commit `64d59bf`).

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking build/tooling issues, neither touching application logic)
**Impact on plan:** Both necessary to make the plan's own verify commands runnable at all. No scope creep — no new libraries were introduced beyond what the plan already specified (`pgx/v5/pgxpool`) or plan 01-01 already installed (`migrate`).

## Issues Encountered

None beyond the two deviations above, both resolved within the task they were found in.

## User Setup Required

None - no external service configuration required. Local Postgres (`rndmroll_dev`, `rndmroll_test`) was already provisioned by plan 01-01.

## Next Phase Readiness

- Wave 4 handler plans (01-06, 01-08 through 01-11) can build against `internal/user.Repository`, `internal/user.RefreshTokenRepository`, and `internal/user.EmailVerificationRepository` — either the real `internal/store/postgres` implementations or fake in-memory test doubles — without writing any SQL or reading storage code. Final signatures are reproduced verbatim above.
- `rndmroll_dev` is left migrated up (schema present); `rndmroll_test` is migrated up as a side effect of the test suite's `TestMain`.
- No blockers carried forward.

## Self-Check: PASSED
- FOUND: migrations/0001_init.up.sql
- FOUND: migrations/0001_init.down.sql
- FOUND: internal/user/model.go
- FOUND: internal/user/repository.go
- FOUND: internal/store/postgres/db.go
- FOUND: internal/store/postgres/user_repo.go
- FOUND: internal/store/postgres/refresh_token_repo.go
- FOUND: internal/store/postgres/email_verification_repo.go
- FOUND: internal/store/postgres/postgres_test.go
- FOUND: commit e16fc6a
- FOUND: commit 9dc7ef0
- FOUND: commit 64d59bf
- FOUND: commit 9113ae1
- go build ./... — PASS
- go vet ./... — PASS
- TEST_DATABASE_URL=postgres://localhost:5432/rndmroll_test?sslmode=disable go test ./internal/store/postgres -v -count=1 — 6/6 PASS
- migrate up / down 1 / up against rndmroll_dev — PASS (left migrated up)

---
*Phase: 01-foundation-accounts*
*Completed: 2026-09-18*
