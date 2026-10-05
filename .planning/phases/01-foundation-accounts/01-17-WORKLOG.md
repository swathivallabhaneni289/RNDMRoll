# 01-17 worklog (working state, written 2026-10-01 ~14:55 IST)

Plan: `01-17-PLAN.md`. Raw evidence: `01-15-SWEEP.md`. Decisions: `.continue-here.md` status block and STATE.md.

## Where things stand

**Updated 2026-10-05 ~14:40 IST: the 01-17 patch is now COMMITTED** (`479b170` mobile, `ba57e39` Go, `06d8a8a` Profile draft; local only, not pushed). Use `git revert` as the undo; the `patch -R` line below is obsolete. The walkthrough is at steps 15 (second half) to 18; the developer has since asked for an Instagram-style sign-up (see `.continue-here.md` and `01-18-RESEARCH-signup.md`), which would replace steps 8 to 15.

**Updated 2026-10-04 ~21:15 IST (resume): the Part 1 pass in the test plan below is STALE.** Today's commits replaced the four marketing screens with one Welcome page (see the 2026-10-04 bullet in `.continue-here.md`), so redo walkthrough Section A (rewritten steps 1-7 in `01-15-PLAN.md`) first, then Part 2. Everything else below still stands: patch applied and uncommitted, API in test mode, MinIO up. A pre-trace of steps 8 to 18 is in `01-15-PRETRACE.md`; read its section 2 before driving the sign-up steps.

**Updated 2026-10-03 ~12:50 IST: the fixes are APPLIED to the live tree (uncommitted) and checked. Next is walkthrough Part 2 with the developer.**

- **Applied:** `~/.local/share/rndmroll-work/hardening.patch` (22 files: 21 modified plus the new `internal/httpapi/oauth_claim_test.go`), clean apply, no rejects. Undo if ever needed: `patch -R -p1 < ~/.local/share/rndmroll-work/hardening.patch` (2026-10-04: this no longer applies cleanly, 1 of 4 hunks fails in `app/(auth)/_layout.tsx` because the Oct-4 commits touched it; commit the fixes first and use `git revert` instead). It went through two rounds of isolated fixing and adversarial review plus a hand round before this.
- **Gates re-run on the patched live tree, all green:** `tsc --noEmit` exit 0; `go build` and `go vet` ok; `go test ./... -p 1 -count=1 -v` on `rndmroll_test` exit 0 with 158 passes, 0 failures, 0 skips (subtests included; it was 133 before the patch); every design scan passes (em dash clean in `app components lib` and also in `internal cmd`; the raw-hex literal segment still lists only `lib/theme/tokens.test-assert.ts`, same deviation as in `01-15-SWEEP.md`; the two radius token sets match the permitted files exactly, measured with `lib/theme/` left out because the compile-time pins file mentions both tokens; no bare `borderRadius` above 8; no emoji; retired files absent; `app.json` light in all three places). Cold launch of the dev client after the patch lands on the sign-in choices with no error screen. **Not re-run:** `expo export` (do it at close-out, after a commit, because it archives HEAD).
- **API restarted** with `start-api-test-mode.sh`: the process on 8080 is `~/.local/share/rndmroll-api/api`, `/healthz` 200, `MAIL_DRIVER=log` (verification links print to `~/.local/share/rndmroll-api/api.log`), local MinIO storage. An end-to-end smoke test through the live API passed 19 of 19: signup, sign-in refused until verified, link page hands off to `rndmroll://verify-email`, verify, token not reusable, sign-in, upload ticket, upload with exact size and type, public read identical, wrong size and wrong type both refused with 403, unsupported type refused up front, profile save and read back.
- **Smoke account left in `rndmroll_dev`:** `smoke-test@example.com`, username `test_user`. It is the taken username for walkthrough step 14. Keep or delete at close-out.
- **Two test photos are in the simulator library:** TEST 1 (JPEG, portrait) and TEST 2 (HEIC, landscape). They are plain generated images, nothing from the developer's own photos. The app crops the picked photo to a square and re-saves it as JPEG, so an iOS crop screen with a Choose button appears. The simulator has no camera.
- Sign-up is limited to 3 per minute per computer address and the in-memory limiter resets on an API restart. Resend is 1 per 30 s per address.
- **Open question:** the yellow "Open debugger to view warnings" bar shows on every launch. Its text was not identified (React Native warnings do not reach the phone's system log). Tapping the bar in the simulator shows it.
- **Simulator note:** do not quit Simulator.app; it shuts the phone down (see memory `reference-simulator-window-gotchas`). The phone was restarted once on 2026-10-03 and the app relaunched.
- Local test storage (MinIO, loopback only) is running, bucket `rndmroll-avatars`.

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

1. Part 1: steps 1-7. Done 2026-10-03 for the old four-screen pitch, now STALE (rewritten 2026-10-04, redo first); the developer said then that they look fine. REDONE 2026-10-04 about 22:00 IST: the developer confirmed the new Welcome page, Get started to the method chooser and the relaunch back to Welcome; Reduce Motion was checked by Claude with simulator screenshots (still for 47 s, setting restored to off). Next: steps 9 to 11. Follow-up from them: the welcome pages need pictures to stay engaging (the photo panels show only a placeholder glyph today). They are taking screenshots themselves and will design the pictures, so do not generate any (standing rule: no AI-slop photos). For their screenshots the app was reset to first launch (key `rndmroll.intro_seen` removed from the app's AsyncStorage manifest, backup was in the session scratchpad; pressing Continue on the last welcome page sets it again). Do not apply the patch or reload the app until they say they are done with the screenshots.
2. Apply patch, `tsc`, Go tests, start test storage and swap the API, add two sample photos to the simulator (`xcrun simctl addmedia booted <file>`).
3. Part 2: steps 8-15 (signup, verify with the NEWEST link from `~/.local/share/rndmroll-api/api.log`, create profile with a library photo; the camera needs a physical phone).
4. Part 3: steps 16-18, then extras for the fixes: expire the stored refresh token in `rndmroll_dev` and relaunch (must land on sign-in, not hang); stop the API, relaunch, restart the API, return from background (session must restore).
5. Steps 19-20 (Apple, Google) deferred by the developer to the end of the project.
6. Then: `01-17-SUMMARY.md`, `01-15-SUMMARY.md`, VALIDATION approval, ROADMAP and STATE, only after the developer types "approved".

## Commit plan (branch `lab/frosted-editorial`, no checkout while Metro is live)

Explicit paths only. Never `app/lab/`, `docs/reference/`, `.claude/` or `.planning/config.json`. Suggested commits: mobile fixes; Go fixes; planning and design docs (`01-17-PLAN.md`, `01-17-WORKLOG.md`, `01-15-SWEEP.md`, `.continue-here.md`, STATE.md, PROJECT.md, `docs/design-brief-2026-10-01-feed.md`, the two doc amendments). Remove nothing else.

## Standing rules

GSD auto mode (`workflow.auto_advance`, `_auto_chain_active`) must stay OFF: it auto-approves human-verify checkpoints, and the walkthroughs must stay the developer's. Next after Phase 1: `/gsd-discuss-phase 2` run interactively (Daily Roll), then plan and execute, then Phases 3, 4 and 5 (glass feed, see `docs/design-brief-2026-10-01-feed.md`).
