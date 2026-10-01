# 01-17 worklog (working state, written 2026-10-01 ~14:55 IST)

Plan: `01-17-PLAN.md`. Raw evidence: `01-15-SWEEP.md`. Decisions: `.continue-here.md` status block and STATE.md.

## Where things stand

- **The fixes exist as a patch and are NOT applied to the live tree.** Patch: `~/.local/share/rndmroll-work/hardening.patch` (22 files, mobile plus Go, made against HEAD `a2006b5`, dry-run clean on the live tree). Helper scripts and the apply recipe are beside it (`README.md`, `start-test-storage.sh`, `start-api-test-mode.sh`).
- It went through two rounds of isolated fixing and adversarial review, then a third small round by hand. Final checks in the isolated copy: `tsc` exit 0; `go build`, `go vet` and `make test-all-integration` all pass, 0 skipped; design scans pass.
- **Waiting on the developer:** walkthrough Part 1 (steps 1-7, welcome screens). Apply the patch only after they finish Part 1 and agree (it reloads the running app).
- Local test storage (MinIO, loopback only) is installed and running, bucket `rndmroll-avatars`, proven with the repo's own presign code (good upload 200, public read identical, wrong body size and wrong content type both 403).
- The live API (pid 94625, port 8080) still has the placeholder storage. The swap is `start-api-test-mode.sh`, after the patch is applied (the API must be rebuilt from the patched Go code). Stop the old API gracefully (SIGTERM) at a boundary.

## What the patch fixes

Session: expired refresh token no longer hangs the splash (deadlock in `lib/api/client.ts`); a network error no longer deletes tokens; a foreground retry after a real return from background; never delete or overwrite tokens a newer sign-in replaced.
Create your profile: Finish disabled while uploading, no overlapping uploads, failed replacement keeps the previous photo, Name prefilled from the stored name, camera failures handled, username lowercased and check failures shown, fixed 24dp status row (UI-SPEC), Bio cannot move.
Edit profile: upload size taken from the real blob (storage rejects any mismatch with 403), camera handled, name and bio trimmed, whitespace-only bio shows the prompt, log out sheet elevation.
Auth: back gesture and Android back disabled on Create your profile, superseded verification link has its own message, email trimmed, stale draft cleared.
Backend: social sign-in on an unverified account is a safe claim (revoke tokens, claim and clear password, then link; every step idempotent, heals on retry), Apple needs an email, a repeat Apple sign-in with no name provably keeps the stored name (threat T-01-UAT-01), Rotate guarded, em dashes out of Go comments.

## Deliberately not changed

- D4 routing flash on relaunch and logout (only if walkthrough step 7 or 18 shows it; routing is fragile, see commits 255ca31, 61855e2, a4fffa5).
- D13 and the expired-link dead end on a cold-start link (needs a device check): the Resend button is disabled when the app opens cold from the link.
- D15 OS permission prompt text (app.json plugin, needs the next native rebuild; batch with `expo-blur` and the Google Sign-In plugin in Phase 5).
- Status row at screens narrower than about 284dp or large text sizes (fits at the spec baseline).
- Subject-match healing needs a non-empty provider email; a refresh token issued just before a claim stays valid until its access token expires (short).

## New or changed on-screen text needing the developer's approval

Both screens: `3 to 20 letters, numbers, or underscores.` / `Take photo` / `Choose from library` / `Camera isn't available on this device.` / `Couldn't check that username. Try again.` / `That username's taken.` (no alternates) / `Couldn't open your photo library.`
Verify email: `This link has expired or was replaced. Send a new one below.`
Plus the 20 earlier strings listed in `01-15-SWEEP.md` and the six in 01-15 section F.

## Test plan with the developer (short messages, small groups)

1. Part 1: steps 1-7 (sent, waiting).
2. Apply patch, `tsc`, Go tests, start test storage and swap the API, add two sample photos to the simulator (`xcrun simctl addmedia booted <file>`).
3. Part 2: steps 8-15 (signup, verify with the NEWEST link from `~/.local/share/rndmroll-api/api.log`, create profile with a library photo; the camera needs a physical phone).
4. Part 3: steps 16-18, then extras for the fixes: expire the stored refresh token in `rndmroll_dev` and relaunch (must land on sign-in, not hang); stop the API, relaunch, restart the API, return from background (session must restore).
5. Steps 19-20 (Apple, Google) deferred by the developer to the end of the project.
6. Then: `01-17-SUMMARY.md`, `01-15-SUMMARY.md`, VALIDATION approval, ROADMAP and STATE, only after the developer types "approved".

## Commit plan (branch `lab/frosted-editorial`, no checkout while Metro is live)

Explicit paths only. Never `app/lab/`, `docs/reference/`, `.claude/` or `.planning/config.json`. Suggested commits: mobile fixes; Go fixes; planning and design docs (`01-17-PLAN.md`, `01-17-WORKLOG.md`, `01-15-SWEEP.md`, `.continue-here.md`, STATE.md, PROJECT.md, `docs/design-brief-2026-10-01-feed.md`, the two doc amendments). Remove nothing else.

## Standing rules

GSD auto mode (`workflow.auto_advance`, `_auto_chain_active`) must stay OFF: it auto-approves human-verify checkpoints, and the walkthroughs must stay the developer's. Next after Phase 1: `/gsd-discuss-phase 2` run interactively (Daily Roll), then plan and execute, then Phases 3, 4 and 5 (glass feed, see `docs/design-brief-2026-10-01-feed.md`).
