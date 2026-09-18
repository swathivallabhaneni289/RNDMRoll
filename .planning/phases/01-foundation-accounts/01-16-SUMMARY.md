---
phase: 01-foundation-accounts
plan: 16
subsystem: ui
tags: [react-native, expo-router, expo-status-bar, react-native-svg, design-tokens]

requires:
  - phase: 01-04
    provides: "AppText, Screen, PrimaryButton, StepProgress, PhotoPlaceholder, PhotoPanel/PolaroidCard/SplitPhotoPanels, PhotoScrim, AdvanceControl primitives"
  - phase: 01-05
    provides: "lib/onboarding/intro-seen.ts (markIntroSeen), app/(auth)/_layout.tsx's eight declared routes with welcome as the default initial route"
provides:
  - "app/(auth)/welcome.tsx - the full-bleed cover screen opening the pre-signup sequence"
  - "app/(auth)/ritual.tsx - numbered pitch screen 1 of 3 (polaroid scatter)"
  - "app/(auth)/real-photos.tsx - numbered pitch screen 2 of 3 (single-panel real-photos policy)"
  - "app/(auth)/everyone-rolls.tsx - numbered pitch screen 3 of 3, hands off to choose-method"
affects: [01-15 (copy-approval and contradiction-resolution checkpoint), any future plan touching the (auth) route group]

tech-stack:
  added: []
  patterns:
    - "AppText and PrimaryButton accept no style prop (Omit<TextProps, 'style'>), so all positioning is done via wrapping View elements, never by styling the primitive directly"
    - "Locked Copywriting Contract strings are rendered as a single-line JSX string expression ({'...'}), never wrapped across source lines, so each string stays intact for line-scoped grep verification and so apostrophes inside the copy (life's, it's, tonight's) never collide with JSX quoting"
    - "Route navigation uses flat paths with no (auth) group prefix (e.g. router.push('/ritual')), matching the convention already established in app/(auth)/_layout.tsx's own router.replace calls"

key-files:
  created:
    - app/(auth)/welcome.tsx
    - app/(auth)/ritual.tsx
    - app/(auth)/real-photos.tsx
    - app/(auth)/everyone-rolls.tsx
  modified: []

key-decisions:
  - "Resolved a UI-SPEC self-contradiction as flagged by the plan: the Design System section's flow enumeration (Welcome's 'Get started' leads to The Ritual) is followed over the Interaction Contracts section's looser summary sentence ('Welcome's CTA and Everyone Rolls' CTA both hand off to choose-method'), because the Interaction Contracts reading would leave The Ritual, Real > Perfect, and Everyone Rolls permanently unreachable. welcome.tsx's CTA routes to /ritual, not /choose-method."
  - "The scrim's top opaque fraction is computed, not hardcoded, as Math.max(SCRIM_TOP_SAFE_FRACTION, (insets.top + TAGLINE_BLOCK_HEIGHT + space.sm) / height), where TAGLINE_BLOCK_HEIGHT = type.label.lineHeight * 2 (the tagline's actual two-line rendered height). This raises the flat top zone past the contract's declared 8% whenever a device's status-bar inset plus the tagline block would otherwise exceed it, preserving the contract's own stated opaque-zone contrast guarantee. No simulator/device exists in this environment to visually confirm; see the worked example under 'Scrim Resolution' below."
  - "everyone-rolls.tsx does not wrap its body in AdvanceControl, per the plan's explicit 'executor's choice' framing: the Continue CTA already provides the visible forward affordance the chevron exists to guarantee, so a duplicate chevron next to it was judged unwanted rather than merely optional."
  - "The Ritual's three PolaroidCard elements are each wrapped in an individual flex:1 View before being placed in a row, because PolaroidCard itself takes no width prop and its inner aspectRatio:1 view has nothing to resolve against without a parent-supplied width. The middle card is rendered without extra top margin and the two outer cards get a small extra marginTop, producing the 'scattered' look the UI-SPEC's prose describes without introducing any animation or randomization (the three rotation values remain the fixed -6/0/6 props the contract requires)."

patterns-established:
  - "Locked/contract copy strings render as {'...'} single-line JSX expressions, one per AppText, so verify-block line-scoped greps never see a string broken across a wrap"

requirements-completed: [ACCT-01]

coverage:
  - id: D1
    description: "Welcome cover screen: full-bleed photo placeholder, dual-edge scrim with a computed (not hardcoded) top-safe fraction, top-left tagline and bottom-anchored wordmark+CTA inside the scrim's guaranteed-opaque zones, on-photo CTA variant, no step indicator, no brand glyph, no background texture, light status bar"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; all of Task 1's <verify> greps re-run individually (exact-string, variant, constant-reference, and zero-occurrence checks) - see Task Commits below"
        status: pass
      - kind: other
        ref: "npx expo export --platform ios --output-dir <scratchpad>/expo-export-01-16"
        status: pass
    human_judgment: true
    rationale: "The scrim's opaque-zone contrast guarantee and the exact on-device appearance of the bottom-anchored CTA group need a human to verify on a real device/simulator, which does not exist in this environment (same gap 01-04 and 01-05 flagged for their own visual/gestural deliverables). Structural and string-level checks pass statically."
  - id: D2
    description: "The Ritual and Real > Perfect pitch screens: numbered step indicator, photo panel composition (three fixed-rotation polaroids / one portrait panel), Display headline, mechanic-accurate body copy, swipe-plus-chevron advance via the shared AdvanceControl, no CTA, no intro-seen write"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; all of Task 2's <verify> greps re-run individually per file, including the negative checks (no PrimaryButton, no markIntroSeen, no rejected same-category/same-prompt phrasing, no raw hex) - see Task Commits below"
        status: pass
      - kind: other
        ref: "npx expo export --platform ios --output-dir <scratchpad>/expo-export-01-16"
        status: pass
    human_judgment: true
    rationale: "The swipe gesture's feel and the polaroid scatter's visual arrangement are interaction/visual concerns that need a human to exercise on-device; structural and string-level checks pass statically."
  - id: D3
    description: "Everyone Rolls screen: step indicator reading 3 of 3, split photo panels, mechanic-accurate renamed-screen headline and body, standard-variant Continue CTA that calls markIntroSeen() (fire-and-forget) then navigates to choose-method"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "npx tsc --noEmit; all of Task 3's <verify> greps re-run individually, including the negative checks (no onPhoto variant, no rejected same-category/same-prompt/different-taste phrasing, no raw hex) - see Task Commits below"
        status: pass
      - kind: other
        ref: "npx expo export --platform ios --output-dir <scratchpad>/expo-export-01-16"
        status: pass
    human_judgment: true
    rationale: "Confirming the flag write actually persists and that the handoff to choose-method resumes correctly needs a real device/simulator run against a live intro-seen flag, which this environment does not have (choose-method's own behavior is unchanged by this plan and was covered by an earlier plan)."
  - id: D4
    description: "The four screens form one reachable chain (cover to pitch to pitch to pitch, ending at choose-method) with no em dash and no raw hex color literal in any of the four files"
    requirement: "ACCT-01"
    verification:
      - kind: other
        ref: "grep -rn for U+2014 (em dash) across all four files: zero matches. grep -cE for raw hex literal across all four files: zero matches in each."
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-18
status: complete
---

# Phase 01-foundation-accounts, Plan 16: Pre-Signup Marketing Screens Summary

**Four React Native/Expo-Router screens (Welcome, The Ritual, Real > Perfect, Everyone Rolls) transcribing UI-SPEC revision 9's Copywriting Contract verbatim, composed entirely from plan 01-04's primitives with zero new copy, color, or dependency.**

## Performance

- **Duration:** ~35 min (resumed once after an unrelated infrastructure interruption; net working time)
- **Completed:** 2026-09-18T18:57:40+05:30
- **Tasks:** 3 (all `type="auto"`, no checkpoints - plan frontmatter confirmed `autonomous: true`)
- **Files modified:** 4 created (app/(auth)/welcome.tsx, ritual.tsx, real-photos.tsx, everyone-rolls.tsx)

## Accomplishments

- Built all four pre-signup marketing screens D-05 (revised 2026-09-17) adds, composed entirely from plan 01-04's fourteen existing UI primitives - no new component, no new npm package, no new color or copy.
- Welcome renders a full-bleed photo cover with a dual-edge scrim whose top opaque zone is computed from the actual safe-area inset and tagline block height, rather than the contract's hardcoded 8% default, so the opaque-zone contrast guarantee holds on notched devices too.
- The Ritual and Real > Perfect share one AdvanceControl-wrapped structure (swipe-left plus a visible chevron, one shared callback) with their contract-mandated, mechanic-accurate copy.
- Everyone Rolls is the sequence's single exit: its Continue CTA fires `markIntroSeen()` without awaiting it and navigates immediately to `/choose-method`.
- Resolved, per the plan's explicit instruction, a genuine self-contradiction inside UI-SPEC revision 9 (see Key Decisions and "Flagged Resolutions" below) in favor of the reading that leaves no screen unreachable.
- Restored `node_modules` (absent in this fresh worktree, as in plans 01-04 and 01-05 before it - worktrees don't share gitignored directories) via `npm ci`, confirmed zero lockfile drift, and recreated the untracked `expo-env.d.ts` needed for `tsc` to resolve `process.env.EXPO_PUBLIC_*` typings, matching both prior plans' precedent.

## Task Commits

Each task was committed atomically:

1. **Task 1: Welcome cover screen with the scrim-guaranteed content zones** - `57d2c73` (feat)
2. **Task 2: The Ritual and Real > Perfect, the two swipe-and-chevron pitch screens** - `1802211` (feat)
3. **Task 3: Everyone Rolls, the split-panel screen that hands off to choose-method** - `943d2f3` (feat)

_No separate plan-metadata commit: this is a worktree-mode dispatch - STATE.md and ROADMAP.md are excluded per orchestrator instructions; this SUMMARY.md is committed separately below._

## Files Created/Modified

- `app/(auth)/welcome.tsx` - Full-bleed cover screen. Plain `flex:1` View (not the shared `Screen` shell), `PhotoPlaceholder variant="cover"` under a computed-fraction `PhotoScrim`, top-left tagline and bottom-anchored wordmark+`PrimaryButton variant="onPhoto"`, `<StatusBar style="light" />` scoped to this screen only. Advances to `/ritual`; writes no intro-seen flag.
- `app/(auth)/ritual.tsx` - `Screen scroll={false}` wrapping `AdvanceControl`; step indicator `current={1}`; three `PolaroidCard` elements at rotations -6/0/6; Display headline `life's better when it's random.`; body `Every night at 8:00 PM, your wheel reveals tonight's category.`. Advances to `/real-photos`.
- `app/(auth)/real-photos.tsx` - Same structure; step indicator `current={2}`; one `PhotoPanel` at its default 4:5 aspect ratio; Display headline `real photos only.`; body `No posters. No album covers. No stock images. Only moments you actually captured.`. Advances to `/everyone-rolls`.
- `app/(auth)/everyone-rolls.tsx` - `Screen scroll={false}`, no `AdvanceControl` (CTA already provides the forward affordance); step indicator `current={3}`; `SplitPhotoPanels`; Display headline `everyone rolls. everyone shares.`; body `Every night at 8:00 PM, your circle each rolls their own category from their own wheel. The fun is seeing what everyone got and how they showed up for it.`; standard-variant `PrimaryButton label="Continue"` whose handler calls `markIntroSeen()` then navigates to `/choose-method`.

## Decisions Made

See `key-decisions` in frontmatter above. Summary:
1. Resolved the flagged UI-SPEC self-contradiction (see "Flagged Resolutions" below) in favor of the Design System section's flow enumeration.
2. Computed the scrim's top opaque fraction from `insets.top`, the tagline's actual rendered block height, and `space.sm`, rather than using the contract's hardcoded 8% default.
3. Chose not to wrap `everyone-rolls.tsx`'s body in `AdvanceControl`, since the plan explicitly left this to executor judgment and a duplicate chevron beside the CTA was called out as unwanted.
4. Wrapped each `PolaroidCard` in its own `flex:1` container to give it a resolvable width for its internal `aspectRatio:1`, since the component itself takes no width prop.

## Flagged Resolutions (per the plan's own request, for the plan 01-15 checkpoint)

**1. Scrim top-zone widening.** UI-SPEC's Photo Treatment section declares the scrim's top flat zone as a fixed 0-8% of screen height. `welcome.tsx` instead computes `topOpaqueFraction = Math.max(SCRIM_TOP_SAFE_FRACTION, (insets.top + TAGLINE_BLOCK_HEIGHT + space.sm) / height)`, where `TAGLINE_BLOCK_HEIGHT = type.label.lineHeight * 2` (40dp - the tagline's actual two-line rendered height) and `SCRIM_TOP_SAFE_FRACTION` is the contract's own 0.08 default, exported by `PhotoScrim`. This widens the flat zone past 8% whenever a device's own status-bar inset plus the tagline block would otherwise exceed it, which is true on every notched iOS device. No simulator exists in this environment to measure a live value, so a worked example instead: for a representative notched-device inset of `insets.top = 59`, `height = 844` (iPhone 14/15-class), `TAGLINE_BLOCK_HEIGHT = 40`, `space.sm = 8`: `(59 + 40 + 8) / 844 = 107 / 844 ≈ 0.1268`, which exceeds the 0.08 default and is what the formula would resolve to on that device - computed from the formula, not measured on a running simulator. `PhotoScrim`'s own fade band (`SCRIM_TOP_FADE_DEPTH`, 0.12) moves with whatever value is passed in, so the guarantee UI-SPEC states as the point of the scrim (every text element inside a fully opaque zone) is preserved rather than broken.

**2. Welcome's CTA route.** UI-SPEC revision 9 contains two readings of where Welcome's "Get started" CTA goes: the Design System section's flow enumeration (Welcome to The Ritual to Real > Perfect to Everyone Rolls to choose-method) and the Interaction Contracts section's summary sentence ("Welcome's CTA and Everyone Rolls' CTA both hand off to the existing, unchanged choose-method screen"). These cannot both be literally true. `welcome.tsx` follows the Design System enumeration - the CTA navigates to `/ritual`, not `/choose-method` - because the Interaction Contracts reading would leave The Ritual, Real > Perfect, and Everyone Rolls permanently unreachable from any screen, stranding three of the four screens this revision adds as dead routes with no path in. The Interaction Contracts sentence is read as making its own narrower point (that both CTAs are straight navigations, not state-machine transitions), which stays true under this resolution.

Neither resolution changes any declared copy, color, or token value from UI-SPEC revision 9.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule: Blocking] `node_modules` was absent despite worktree-base expectations**
- **Found during:** pre-Task-1 environment check
- **Issue:** This fresh worktree had no `node_modules/` (gitignored, not shared across git worktrees) - same gap plans 01-04 and 01-05 both hit and resolved in their own worktrees.
- **Fix:** `npm ci` to restore exactly from the existing, unmodified `package-lock.json`.
- **Files modified:** none tracked (node_modules is gitignored)
- **Verification:** `git status --porcelain package-lock.json` returned empty immediately after, confirming no lockfile drift.
- **Committed in:** not applicable (no tracked change)

**2. [Rule: Blocking] `expo-env.d.ts` was absent, breaking `tsc`'s resolution of `process.env.EXPO_PUBLIC_*`**
- **Found during:** baseline `npx tsc --noEmit` check before writing any screen
- **Issue:** This ambient-types file is generated by Expo tooling and deliberately left untracked; a fresh worktree never had one.
- **Fix:** Recreated it at the repo root with Expo's standard one-line reference (`/// <reference types="expo/types" />`), left untracked - identical to plan 01-05's precedent and stated reasoning.
- **Files modified:** `expo-env.d.ts` (created, untracked, not committed)
- **Verification:** `npx tsc --noEmit` passed cleanly both before and after writing all four screens.
- **Committed in:** not committed (intentionally untracked, consistent with plan 01-05's identical file)

---

**Total deviations:** 2 auto-fixed (both blocking-environment, both pure precedent-following restorations, neither touching any tracked source file). **Impact on plan:** Neither affected scope; both were necessary just to run `tsc`/`expo export` at all. No scope creep - only the four files named in the plan's `files_modified` were created or modified.

## Issues Encountered

- **Package legitimacy check:** the plan's own instructions require verifying any new npm package's source repository before installing. No new package was needed for this plan - all four screens compose exclusively from plan 01-04's existing primitives (`AppText`, `Screen`, `PrimaryButton`, `StepProgress`, `PhotoPlaceholder`, `PhotoPanel`/`PolaroidCard`/`SplitPhotoPanels`, `PhotoScrim`, `AdvanceControl`) plus `expo-status-bar`, which was already a declared dependency (`~57.0.1` in `package.json`) before this plan started. No `npm view` verification was required or performed.
- **`AppText` and `PrimaryButton` accept no `style` prop** (`AppTextProps` explicitly `Omit`s `style` from `TextProps`; `PrimaryButtonProps` never declares one). All positioning (the tagline's top-left placement, the bottom-anchored wordmark+CTA group, inter-element spacing) is done by wrapping these primitives in plain `View` elements with their own `style`, never by attempting to style the primitives directly. This is a structural constraint of the existing primitives, not a plan deviation.
- **Verify-block substring risk (precedented in `01-05-SUMMARY.md` deviation 4):** the plan's own automated `<verify>` blocks do blunt whole-file greps for banned words (`markIntroSeen`, `PrimaryButton`, `BrandMark`/`BackgroundDotGrid`/`StepProgress`, and case-insensitive `same category`/`same prompt`/`different taste`) that can't distinguish code from an explanatory comment mentioning the same word. All four files were written with zero explanatory comments referencing any of these banned strings, sidestepping the risk entirely rather than writing a comment and then having to reword it. Every one of Task 1, 2, and 3's `<verify>` grep lines was re-run individually (the combined one-liner form was expected to be rejected by the worktree path-safety checker, per 01-04's identical precedent, so this plan ran each check as a separate plain command from the start) and all passed.
- **Plan-level verification beyond `tsc`:** ran `grep -rn` for the literal em dash character (U+2014) across all four files (zero matches) and `npx expo export --platform ios` to an output directory under this session's scratchpad (completed successfully, confirming all four new routes resolve alongside the five pre-existing ones), since `tsc --noEmit` alone would not catch a Metro bundler resolution break.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All nine screens in D-05's revised sequence now have a file on disk: the four this plan adds, plus the five plan 01-05's `app/(auth)/_layout.tsx` already declared as routes (`choose-method`, `email`, `verify-email`, `profile-setup` - built by sibling/earlier plans - and this plan's own four).
- The two flagged resolutions above (scrim top-fraction widening; Welcome's CTA routing to The Ritual rather than choose-method) are carried forward explicitly for the plan 01-15 checkpoint to confirm, per the plan's own instruction. Neither is a silent judgment call - both are documented here with full reasoning.
- No new npm dependency, no new color, no new copy: every string on all four screens is verbatim from UI-SPEC revision 9's Copywriting Contract, and every token reference resolves through `lib/theme/tokens.ts`.
- No blockers for the plan 01-15 checkpoint or for any later plan touching the `(auth)` route group. A real device/simulator run remains the only way to visually confirm the scrim's guaranteed contrast, the polaroid scatter's on-screen arrangement, and the swipe gesture's feel - flagged as `human_judgment: true` in this summary's `coverage` block, consistent with how plans 01-04 and 01-05 flagged their own equivalent visual/gestural gaps.

## Self-Check

- All 4 `key-files.created` verified present on disk via successful `Write` tool calls (harness-tracked; no re-read needed).
- `git log --oneline` shows all three task commits (`57d2c73`, `1802211`, `943d2f3`) directly on top of the wave-3 base (`0818321`).
- `npx tsc --noEmit` clean (exit 0) both before writing any screen and after all three tasks.
- Every one of Task 1's, Task 2's, and Task 3's `<acceptance_criteria>` / `<verify>` grep checks re-run individually (not the combined one-liner) after each write: all PASS.
- Plan-level `<verification>`: `npx tsc --noEmit` clean; zero em dash (U+2014) across all four files; zero raw hex literal across all four files; `npx expo export --platform ios` completed successfully with all four new routes bundled.

## Self-Check: PASSED

---
*Phase: 01-foundation-accounts*
*Plan: 16*
*Completed: 2026-09-18*
