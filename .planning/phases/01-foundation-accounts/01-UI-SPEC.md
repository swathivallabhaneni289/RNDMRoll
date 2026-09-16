---
phase: 1
slug: foundation-accounts
status: draft
reviewed_at: 2026-09-15
shadcn_initialized: false
preset: none
created: 2026-09-15
revised: 2026-09-16
revision: 8
revision_reason: "Revision 8 reverses revision 7's dark-primary theme back to a light editorial theme, per the user's explicit decision on reviewing a full second design brief (docs/design-brief-2026-09-16.md) and the resulting PROJECT.md updates (Key Decisions row 'Theme system: light editorial throughout', and the Context note confirming this is now a whole-app decision, not a per-phase split). This is a lightness-axis flip applied with the same rigor revision 7 used when it flipped light-to-dark, not a naive re-invert and not a literal return to revisions 1-6's exact earlier light hex values. TRIGGER AND SCOPE: the brief asked for a warm off-white/charcoal editorial aesthetic (Playfair Display + Inter, pill CTAs, a splash + 3-screen marketing onboarding, several later-phase layout ideas). Per the brief's own reconciliation notes (binding, read in full) and the user's explicit follow-up decisions, only the theme direction is adopted here; typography, pill buttons, and new-screen scope are explicitly rejected or deferred, detailed below. RECOMPUTED, NOT REUSED: Dominant becomes `#F6F5F2` (warm off-white, luminance 0.9133), Secondary becomes `#E8E7E3` (soft warm grey, luminance 0.7986, one tonal step DARKER/greyer than Dominant, reverting to the pre-revision-7 light-theme convention where secondary/recessed surfaces read as more shadowed, the inverse of revision 7's dark-theme-only 'lighter equals more elevated' rule). A new elevation-only utility tone, `#FFFFFF` (pure white, luminance 1.0), is added for the Elevation system's `card` tier: in this light theme, elevated surfaces move toward the brighter/whiter ceiling rather than away from it, the direct inverse of revision 7's `#262626`-toward-black elevation tone, and itself a callback in spirit (an elevation-only utility tone bracketing the Dominant/Secondary pair) even though the concrete hex differs because black-crush avoidance doesn't apply at the white end. Ink/Accent becomes `#111111`, adopted directly at the brief's own cited charcoal value (the same "adopt the brief's own cited hex" approach used for Dominant and Secondary, rather than inventing a neighbor value with no clear justification) — verified at 17.33:1 against Dominant, 15.26:1 against Secondary, 18.89:1 against the white elevation tone, all far above the 4.5:1 floor. Revision 5's original "avoid pure `#000000`" anti-vibration reasoning does not need to be invoked here, since `#111111` is itself already comfortably short of pure black. Muted text is freshly computed (not reused from any prior revision) as `#625F5B`, a warm mid-grey chosen and iterated specifically to clear 4.5:1 against both structural surfaces: 5.83:1 on Dominant, 5.13:1 on Secondary (an earlier candidate, `#6B6864`, was rejected during this computation for measuring only 4.48:1 on Secondary, just under the floor). Divider/Border becomes `#D3D0C9` (luminance 0.6316), deliberately darker than both Dominant and Secondary, reverting to the pre-revision-7 convention that a border must read darker than the surfaces it separates in a light theme (the inverse of revision 7's lighter-than-both dark-theme rule) while landing on a genuinely warm-toned grey rather than a cold neutral, consistent with every other structural color in this revision. DELIBERATE DEPARTURE FROM 'TRUE NEUTRAL R=G=B', STATED EXPLICITLY: revisions 4 through 7 held every structural neutral to zero hue bias (equal R/G/B channels) as a hard sub-rule of 'grayscale discipline'. This revision relaxes that specific sub-rule in favor of a warm-tinted neutral system (Dominant, Secondary, the white elevation tone, Ink, Muted, and Divider all carry a slight warm bias, per the brief's specific cited hex family and PROJECT.md's own reconciliation framing, which explicitly calls the warm off-white palette compatible with 'grayscale discipline' meaning 'no chromatic/saturated brand hue', not literally 'zero hue bias in every neutral'). The core rule that sub-rule served is fully intact and unchanged: no saturated or chromatic brand color anywhere in this palette, only the two small muted functional colors carry any real hue. DESTRUCTIVE/SUCCESS RE-VERIFIED, NOT BLINDLY CARRIED FORWARD: revision 7's dark-tuned values (`#C97268`/`#6FA37E`) do not carry forward (they were computed for near-black backgrounds and are now the wrong direction entirely). Rather than assuming the original revision 5/6 light-theme values (`#9A3B32` destructive, `#416B4C` success) still work simply because they predate revision 7, this revision re-verified them by contrast math against the NEW warm Dominant/Secondary hexes specifically (which differ from revision 1-6's pure white/`#F0F0F0`): Destructive measures 6.33:1 on Dominant, 5.58:1 on Secondary; Success measures 5.61:1 on Dominant, 4.94:1 on Secondary — both clear 4.5:1 with Success's Secondary margin the tightest at 4.94:1, still comfortably passing. Because both values independently re-verify against the new surfaces, they are kept unchanged in hex rather than recolored, but this is a documented re-verification, not an assumption. ELEVATION MECHANISM REVERTED, NOT JUST RECOLORED: revision 7 replaced shadow-based elevation with tonal elevation specifically because a black shadow on a near-black background is mathematically invisible (~1.05:1). That constraint does not exist on a light background — a dark shadow under a light surface is the textbook case shadows are designed for — so this revision reverts to shadow-primary elevation as revisions 3-6 originally used, but recomputed: `subtle` stays a flat, un-shadowed Secondary-toned panel (the tonal step alone, plus the Divider/Border where a hard edge is needed, does the separating); `card` becomes a `#FFFFFF` fill with a soft shadow; `raised` (the primary CTA) gets the heaviest shadow of the three, and its fill inverts from revision 7's light-Ink-on-dark to dark-Ink-on-light — solid `#111111` fill with a `#F6F5F2` (Dominant-colored) label, the exact same 'CTA label is always Dominant-colored text on Ink fill' rule as revision 7, just with Dominant itself now light instead of dark, a clean symmetric continuity across the inversion. One refinement beyond a pure revert: every shadow in this revision uses `shadowColor:` Ink `#111111` rather than a literal `#000000` (revisions 3-6's original shared value) — the palette's own darkest neutral is already the natural shadow candidate and keeps every shadow warm-coherent with the rest of the system instead of introducing a cool black with no other role in the palette. TYPOGRAPHY: Domine (headings) + Work Sans (body/UI) are explicitly KEPT, not replaced. The brief specifies Playfair Display + Inter; both are rejected again for the same reason they were rejected on 2026-09-15 (recognizable AI-generated-UI default fonts, see PROJECT.md moodboard note) — re-adopting them now specifically because a new brief asked would contradict that standing decision for no new reason. Sizes are NOT left untouched, however: per this revision's explicit discretion to lean into 'oversized editorial typography' using the existing font pairing, Display moves from 28/34 to 40/46 and Heading moves from 20/26 to 22/28, producing meaningfully more dramatic scale contrast against the unchanged Body (16/24) and Label (14/20) rows, without pushing into the brief's literal 48-72px range, which belongs on a future full-bleed splash/hero surface this phase does not have (see Scope note below), not a dense onboarding form. The same two Display promotions from revision 6 (wordmark lockup, verify-email heading) carry forward unchanged in count and location; the increase is in scale drama, not in how many screens get a Display moment, preserving the restraint argument already established in revision 6. PILL BUTTONS: explicitly rejected again. The brief asks for pill CTAs twice; pill buttons are a standing hard constraint (PROJECT.md Constraints and user memory, 'no pill-shaped buttons', stated alongside 'most importantly, make no mistakes'). The `md` (8dp) radius cap on buttons and inputs is unchanged. Per this revision's explicit discretion, a NEW `lg` (24dp) radius token is added to Shape, reserved for large non-interactive containers only (the profile-view container card, and the log-out confirmation sheet) — this is how the brief's 'rounded corners 20-24px' request is honored without touching button/input shapes, and it does not weaken the no-pill rule since it is explicitly never applied to anything tappable. SCOPE: this revision does NOT add a splash screen or the brief's 3-screen pre-signup marketing onboarding. Per docs/design-brief-2026-09-16.md's own reconciliation notes and CONTEXT.md's D-05 sequence (already starting at 'choose signup method', no splash step), Phase 1's success criteria (create account/log in/stay logged in; view own profile, per ROADMAP.md) do not require a pre-signup marketing sequence to be satisfied, so none is assumed into scope here. This is flagged as an explicitly open, undecided question rather than silently resolved either way — see the Scope note under Design System below. WHOLE-APP IMPLICATION: PROJECT.md's Context/Key Decisions sections already record this theme reversal as whole-app, not Phase-1-only, superseding the earlier light-utility/dark-moment split revision 7 had reintroduced. This UI-SPEC's Color section updates its Phase 2 coordination note accordingly: future phases adopt this same light editorial direction in their own UI-SPECs, not a dark 'moment screen' treatment. UNCHANGED: Spacing scale, icon library (`@expo/vector-icons`), component approach (custom RN primitives, no UI kit), font-loading/session-restore gate mechanics, Copywriting Contract (no color dependency), Interaction Contracts' state machines and timing (only the hex values the color-role names resolve to changed), Accessibility requirements, Registry Safety (not applicable to this RN/Expo target). Brand Mark and Background Texture are recolored (Ink pips/dots now `#111111` on the light Dominant background) but their composition, geometry, opacity-ladder position (texture stays at 8%), and single-screen/two-use scoping are otherwise unchanged, reverting to the same 'dark dot on light background' physical scenario revision 6 originally specified (revision 7's caveat about light dots visually 'glowing' on a dark background no longer applies). Platform lock flips from `userInterfaceStyle: dark` back to `light`. Checker Sign-Off reset to pending; status reset to draft pending re-verification against all 6 dimensions."
---

# Phase 1 - UI Design Contract

> Visual and interaction contract for frontend phases. Generated by gsd-ui-researcher, verified by gsd-ui-checker.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | none. shadcn is a web/Tailwind registry tool and does not apply here: RESEARCH.md locks this phase to React Native / Expo SDK 57 with expo-router (not React/Next.js/Vite), so the shadcn init gate does not fire. |
| Preset | not applicable |
| Component library | none (custom). Plain React Native primitives (View, Text, Pressable, TextInput) styled from a single shared tokens module (e.g. `lib/theme/tokens.ts`), not a third-party RN UI kit. Unchanged by this revision. |
| Icon library | `@expo/vector-icons` (Ionicons as the default icon set). Unchanged by this revision. |
| Font | **Domine** (serif, display/heading only) at one weight, `Domine_600SemiBold`, plus **Work Sans** (body/UI) at two weights, `WorkSans_400Regular` and `WorkSans_600SemiBold`. Explicitly KEPT in this revision despite docs/design-brief-2026-09-16.md specifying Playfair Display + Inter — both of those are rejected again for the same reason they were rejected on 2026-09-15 (recognizable AI-generated-UI-default fonts, see PROJECT.md moodboard note), and re-adopting a rejected pairing simply because a new brief asked for it would undo that standing decision for no new reason. Font families and weights are unchanged by this revision; only the Display and Heading **sizes** change — see Typography below — as this revision's answer to the brief's "oversized editorial typography" request, using the existing locked font pairing rather than the brief's fonts. Same `checkpoint:human-verify` pattern as prior revisions applies before installing: confirm both `@expo-google-fonts/domine` and `@expo-google-fonts/work-sans` resolve to `github.com/expo/google-fonts` before adding them as dependencies (RESEARCH.md flags `expo-*` packages as heuristically SUS due to frequent monorepo republishing, not an actual legitimacy concern — this checkpoint is the cheap resolution). |

**Platform lock:** Phase 1 reverts to **light mode only** (no dark-mode variant specified for this phase), reversing revision 7's dark-primary lock per the user's explicit decision on reviewing docs/design-brief-2026-09-16.md (see PROJECT.md Key Decisions, "Theme system: light editorial throughout," and the matching Context note). Set `"userInterfaceStyle": "light"` in `app.json` to enforce this at the OS level, for the same single-theme-only reason as every prior revision: without it, system dark mode still partially applies (status bar, keyboard, native text-input chrome) even though the screens themselves are styled light-only, producing a visibly broken mixed state.

This reversal is now recorded at the whole-app level, not a per-phase split: PROJECT.md's Context and Key Decisions sections confirm the dark-primary direction (adopted 2026-09-15, revision 7) is reversed back to light editorial throughout the app, superseding revision 7's own reintroduction of the "light utility screens / dark moment screens" split (which itself had superseded an even earlier version of that same split). Phase 2 onward should adopt this same light editorial direction when their own UI-SPECs are created, not a dark "moment screen" treatment — see the Color section's Phase 2 coordination note below.

**Scope note (explicitly not resolved here):** docs/design-brief-2026-09-16.md also describes a full-screen splash and a 3-screen pre-signup marketing/education onboarding sequence ("life's better when it's random" / "real photos only" / "same prompt, different taste"), additive to and distinct from CONTEXT.md's existing D-05 post-signup onboarding sequence (choose method → verify email → name → username → photo). This UI-SPEC does **not** add that splash/marketing sequence to Phase 1's scope: ROADMAP.md's Phase 1 success criteria (create an account and log in, staying logged in across sessions; view one's own profile) do not require it, and CONTEXT.md's D-05 sequence already begins at "choose signup method" with no splash step. Per the brief's own reconciliation notes, whether the app gets a pre-signup marketing sequence at all remains an open, undecided product question — flagged here rather than silently assumed either way, and left for a follow-up discussion if the team wants to pursue it, not built into this phase's seven screens (choose-method, verify-email, name, username, photo, profile view, profile edit — unchanged count from revision 7).

---

## Spacing Scale

Declared values (must be multiples of 4, unitless React Native `dp`, not `px`):

| Token | Value | Usage |
|-------|-------|-------|
| xs | 4 | Icon gaps, inline padding |
| sm | 8 | Compact element spacing |
| md | 16 | Default element spacing |
| lg | 24 | Section padding |
| xl | 32 | Layout gaps |
| 2xl | 48 | Major section breaks |
| 3xl | 64 | Page-level spacing |

Exceptions: `44` as a minimum touch-target hit-area (via padding or `hitSlop`, not a new spacing token) for icon-only controls and tappable rows: back button, close button, the username-availability status icon, the "resend email" tap target. The 40dp icon-badge diameter (Visual Personality) and the 2dp background-texture dot diameter (Background Texture) remain exempt, for the same reasons as prior revisions.

Unchanged by this revision.

---

## Shape (Radius)

Not in the base template; declared here because PROJECT.md's standing "no pill-shaped buttons" constraint needs a concrete home or the executor will default to a large/full radius.

| Token | Value | Usage |
|-------|-------|-------|
| sm | 4 | Small chips/status pills (e.g. "Available" / "Taken" inline badges next to the username field) |
| md | 8 | Buttons, text inputs, and cards by default (e.g. the Secondary-surface field-row panels used by the Elevation `subtle` tier). For buttons and inputs specifically, this is also the **maximum radius** in this phase, enforcing PROJECT.md's "no pill-shaped buttons" rule (a pill is defined here as radius >= half the element's height) — that cap is unchanged by this revision and applies with no exceptions, regardless of the new `lg` token below. |
| lg | 24 | **New in this revision.** A named exception to the `md` card default, reserved for exactly two large, non-interactive content containers: the profile-view container card, and the log-out confirmation sheet (top corners, if rendered as a bottom sheet). This is how docs/design-brief-2026-09-16.md's "rounded corners (20-24px)" request is honored — landing at the brief's own upper bound, a multiple of 4 — without touching anything tappable. Explicitly never applied to buttons, inputs, chips, ordinary field-row cards, or the icon-badge/avatar circular token below; if a future task needs a large-radius treatment on something tappable, that is a `md`-cap violation, not a legitimate use of this token. |
| avatar / icon badge | circular (50% of width/height) | Profile photo, avatar-placeholder, and the two icon badges (empty-bio icon, verify-email icon — see Visual Personality). Unchanged by this revision. |

---

## Typography

React Native `lineHeight` is an absolute number, not a ratio, so both are declared explicitly. Font weight is expressed as a `fontFamily` string, not a numeric `fontWeight` prop — see platform note below.

| Role | Size | Font Family | Line Height |
|------|------|-------------|-------------|
| Body | 16 | `WorkSans_400Regular` | 24 |
| Label | 14 | `WorkSans_400Regular` | 20 |
| Button / CTA label | 16 | `WorkSans_600SemiBold` | 24 |
| Heading | 22 | `Domine_600SemiBold` | 28 |
| Display | 40 | `Domine_600SemiBold` | 46 |

Body, Label, and Button/CTA are unchanged by this revision. **Heading moves from 20/26 to 22/28 and Display moves from 28/34 to 40/46** — this revision's answer to docs/design-brief-2026-09-16.md's "oversized editorial typography (48-72px where appropriate)" request, using the existing locked Domine + Work Sans pairing rather than the brief's rejected fonts. 40dp does not reach the brief's literal 48-72px range on purpose: that range belongs on a full-bleed splash/hero surface, which this phase does not have in scope (see the Scope note under Design System) — 40dp is a deliberately dramatic but still form-screen-appropriate ceiling for the two Display uses this phase actually has. Exactly three font files are loaded application-wide, unchanged: `Domine_600SemiBold` (Heading, Display, wordmark only), `WorkSans_400Regular` (Body, Label), and `WorkSans_600SemiBold` (Button/CTA label, text links).

Platform note (unchanged by this revision): set `fontFamily` directly per role in the tokens module, and do NOT also set a numeric `fontWeight` alongside a custom-loaded `fontFamily` — on Android, React Native's font matcher does not reliably combine a custom family with a separate `fontWeight` override and can silently fall back to that family's default weight. If a font fails to load (see "Session-restore and font-load gate" in Interaction Contracts), fall back to the platform system font at the equivalent numeric weight (400/600), applied only in that fallback branch.

Default text color is now Ink `#111111` rendered on `#F6F5F2`/`#E8E7E3`/`#FFFFFF` light surfaces instead of Ink `#EDEDED` on dark surfaces (see Color below).

### Display Role Application (unchanged in count/location by this revision; sizes updated above)

Exactly two deliberate promotions from Heading to Display, unchanged in location from revisions 6-7, now at the larger 40/46 values:

- **Choose-method screen wordmark:** "RNDMRoll" (see Copywriting Contract), set in Display, positioned directly beneath the Brand Mark — together this phase's one logo-lockup moment.
- **Verify-email waiting screen heading:** "Check your email" (see Copywriting Contract), promoted from Heading to Display — this screen has the lowest content density in the phase, so the larger headline (now noticeably more dramatic at 40 vs. the prior 28) has room to breathe.

Everywhere else stays at Heading (now 22/28): the name/username/photo onboarding step headings, and the profile view/edit screen headings. This exclusion is unchanged and deliberate — these are dense, form-carrying screens where a Display-sized headline would compete with inputs and validation copy. Two uses out of seven screens is what keeps Display reading as emphasis rather than a uniform size bump, now more true than ever given the larger jump between Heading and Display.

---

## Elevation (Shadow / Tonal Elevation System)

Not in the base template; added in revision 2. **Reverted in this revision from revision 7's tonal-elevation mechanism back to a shadow-primary mechanism**, because the specific problem that forced revision 7's tonal system — a black shadow measuring ~1.05:1 against a near-black `#121212` background, effectively invisible — does not exist on a light background. A dark shadow under a light surface is the textbook case drop shadows are designed for, so this revision restores shadow-based depth as the primary cue, the same approach revisions 3-6 originally used, recomputed for the new warm surfaces.

One refinement carried forward from revision 7's own diligence rather than reverted: every shadow below uses `shadowColor:` Ink `#111111`, not a literal `#000000`. The palette's own darkest neutral is already the natural shadow candidate, and using it keeps every shadow warm-coherent with the rest of this system instead of introducing a cool black with no other role in the palette (revisions 3-6 originally shared a plain `#000000` shadow, back when the palette had no distinct warm identity of its own).

| Token | Fill | iOS shadow (primary depth cue) | Android `elevation` | Usage |
|-------|------|-------------------------------|----------------------|-------|
| subtle | Secondary surface `#E8E7E3`, no shadow — intentionally flat/recessed | none | `0` (deliberately flat; this tier does not rise above the background, so it gets no shadow at all) | Secondary-surface panels at rest: profile field rows, the alternate-username suggestion chips. Separation from the background comes from the Secondary/Dominant tonal step itself, plus a Divider/Border edge where one is declared (see Color) — not from elevation. |
| card | `#FFFFFF` (new elevation-only white, see Color) — brighter than both Dominant and Secondary, since elevated surfaces move toward the white ceiling in a light theme rather than away from it | `shadowColor:#111111` / `{width:0, height:2}` / opacity `0.08` / radius `6` | `3` | The profile-view container card (also uses the new `lg` 24 radius, see Shape) and modal/sheet surfaces (the log-out confirmation sheet, also `lg` radius on its top corners). The two icon badges (Visual Personality) pick up `card`'s edge-definition treatment (border, faint shadow) but keep their own fill rule from Visual Personality rather than adopting `#FFFFFF`. |
| raised | Ink fill `#111111` solid (unchanged role: the primary CTA button's fill) | `shadowColor:#111111` / `{width:0, height:4}` / opacity `0.18` / radius `10` — the heaviest shadow of the three tiers, giving the primary CTA the most visual weight on screen, unchanged in intent from every prior revision | `6` | The primary CTA button only, default/enabled state |

Platform note: Android's `elevation` prop only renders a visible shadow on a `View` with an explicit opaque `backgroundColor`, unchanged from prior revisions. On a light background this is the standard, well-understood Material shadow case (unlike revision 7's dark-background caveat, which required an explicit on-device spot-check) — a normal QA pass is sufficient here, no elevated verification concern.

Disabled/loading CTA states use `subtle` instead of `raised`, unchanged from prior revisions — a shadowed button under an inert control still reads as a bug, not polish.

Negative scope, unchanged: no elevation/tonal-shift on avatars, text inputs, or plain-text buttons ("Skip for now", "Resend email", "Forgot password?", the method-toggle links). No elevation/tonal-shift on the Brand Mark or the Background Texture layer, for the same reasons as revision 6/7 (a flat vector glyph directly on its host surface; a background-layer fill one z-index below everything else).

---

## Color

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `#F6F5F2` (warm off-white) | Screen backgrounds, default surface behind all onboarding and profile screens |
| Secondary (30%) | `#E8E7E3` (soft warm grey) — one tonal step **darker/greyer** than Dominant | Cards, input field fill, section backgrounds, bottom tab bar (once it exists in later phases) |
| Elevation tier / "card" tone (utility, not part of 60/30/10) | `#FFFFFF` (pure white) — brighter than both Dominant and Secondary | The Elevation system's `card` tier fill (profile-view container card, modal/sheet surfaces). See Elevation above. |
| Accent / Ink (10% interactive budget, plus default text — see note below) | `#111111` (charcoal, near-black but not pure `#000000`) | See "Accent/Ink reserved for" below |
| Muted text (utility, not part of 60/30/10) | `#625F5B` (warm mid-grey) | Secondary/helper text, timestamps, and other non-primary copy — see note below |
| Divider / Border (utility, not part of 60/30/10) | `#D3D0C9` (warm light-mid grey, darker than both Dominant and Secondary) | Hairline dividers, and the default border on static secondary-surface cards/chips — see note below |
| Destructive | `#9A3B32` (muted terracotta/brick) | Destructive actions and error states only |
| Success | `#416B4C` (muted sage green) | Username-available status only, see Status colors below |

This reverses revision 7's strictly-neutral dark palette (`#121212` / `#1E1E1E` / `#EDEDED`) to a warm-toned **light** palette, per the user's explicit decision after reviewing docs/design-brief-2026-09-16.md. This is a genuine, considered recomputation, not a naive re-invert of revision 7's hex values and not a literal copy of revisions 1-6's earlier true-neutral light palette:

- **Every structural color here carries a deliberate slight warm bias**, unlike revisions 4-7's "true neutral, R=G=B, zero hue bias" rule. This is a stated, intentional relaxation of that specific sub-rule, adopted directly from the brief's cited hex family (`#F6F5F2`-family background, `#111111`-family ink, `#E8E7E3`-family soft greys) and consistent with PROJECT.md's own reconciliation framing, which calls this warm off-white direction compatible with "grayscale discipline." That discipline's real substance — no chromatic/saturated brand hue anywhere in the structural system — is fully intact; what changed is only that "grayscale" here means "no saturated hue," not "literally zero hue bias in every neutral."
- **Secondary `#E8E7E3` is darker/greyer than Dominant, not lighter.** This reverts to the pre-revision-7 light-theme convention (recessed/secondary surfaces read as more shadowed), the exact inverse of revision 7's dark-theme-only "lighter equals more elevated" rule (see Elevation above for the mechanism this drives).
- **The new elevation-only white `#FFFFFF`** sits brighter than both Dominant and Secondary, giving the Elevation system's `card` tier somewhere to go that isn't available by darkening (which would just look muddy against a light backdrop) — the inverse of revision 7's `#262626`-toward-black third tone, achieving the same structural role (a third tonal step reserved for elevation, not part of the 60/30/10 budget) via the opposite direction.
- **Ink `#111111` is adopted directly at the brief's own cited charcoal value, then independently contrast-verified rather than assumed safe by citation alone.** 17.33:1 against Dominant, 15.26:1 against Secondary, and 18.89:1 against the white elevation tone — all far above the 4.5:1 floor, in the same high-crisp-contrast register revision 7 established at the opposite end of the lightness scale.

**Accent/Ink reserved for** (explicit list; nothing else may use this color at full strength):
- The single primary-path CTA button fill per screen (Continue on name/username/photo steps, Save changes on profile edit). Fill is solid Ink `#111111`; label text is Dominant `#F6F5F2`, chosen for maximum legibility against the near-black fill — the same "CTA label is always Dominant-colored text on Ink fill" rule as revision 7, just with Dominant itself now light instead of dark.
- Selected/active state, declared with both a color step and a width step: a chosen sign-in method's outline, and a focused text input's border, go from Ink at 15% opacity / 1dp width at rest to Ink at 100% opacity / 1.5dp width when active or focused.
- Text links ("Forgot password?", "Log in instead" / "Sign up instead"): Ink at 100% and underlined, set in `WorkSans_600SemiBold`. Unchanged mechanism from prior revisions.
- The onboarding step-progress indicator: active step dot Ink at 100%; inactive step dots Ink at 20% opacity.
- The Brand Mark's five pips (see "Brand Mark" below): Ink at 100%, no fill/badge/border treatment.

**Default text color:** all body copy, headings, and labels default to Accent/Ink `#111111` on the light surfaces this phase uses — the conventional dark-text-on-light-background direction, unlike revision 7's inverted white-on-black default. This dual duty (near-universal text color, plus the sole interactive accent) remains intentional and unchanged in reasoning from prior revisions: CTA/active-state prominence comes from solid-fill inversion, the `raised`/`card` elevation tokens, and the underline/weight/opacity/width steps declared above, not from hue exclusivity.

**Muted text**: `#625F5B`, freshly computed for this revision (not reused from any earlier light or dark revision), reserved for secondary/helper copy deliberately de-emphasized relative to default body text — timestamps, de-emphasized captions, and any helper text that is not an active validation error (validation errors stay Destructive). Verified at 5.83:1 on Dominant `#F6F5F2` and 5.13:1 on Secondary `#E8E7E3`, both clearing the 4.5:1 body-text floor with a comfortable margin. An initial candidate, `#6B6864`, was rejected during this revision's computation for measuring only 4.48:1 on Secondary, just under the floor — `#625F5B` is the darkened value that clears it.

**Divider / Border gray**: `#D3D0C9`, a decorative-register value not held to text-contrast ratios. Two uses, unchanged from prior revisions: (1) plain hairline dividers, and (2) an explicit 1dp solid border on static secondary-surface cards and chips that have no other border contract — profile field rows, alternate-username suggestion chips, and the icon-badge component (see Visual Personality). This reverts to the pre-revision-7 convention: the border must read **darker** than both structural surfaces it separates (verified: luminance 0.6316, darker than Dominant's 0.9133 and Secondary's 0.7986), the exact inverse of revision 7's lighter-than-both rule. (Interactive surfaces keep their existing Ink-opacity border contract instead of this token.)

**CTA fill states:** default/enabled = Ink `#111111` solid, label `#F6F5F2`. Disabled, or awaiting a blocked action (e.g. Continue while a username check is in flight) = Ink at 35% opacity as the fill (`rgba(17,17,17,0.35)`), label Dominant at 70% opacity (`rgba(246,245,242,0.7)`), paired with `subtle` elevation instead of `raised`. Same mechanism as prior revisions, recolored.

**Status colors** (exempt from the 60/30/10 budget, same exemption pattern as prior revisions):
- Success `#416B4C` for the username-available check icon and label only. **Re-verified for this revision's new warm surfaces** (not assumed unchanged just because this is the same hex revision 5/6 originally used against a plain white background): 5.61:1 on Dominant `#F6F5F2`, 4.94:1 on Secondary `#E8E7E3` — both clear 4.5:1, with Secondary the tighter of the two margins.
- Destructive `#9A3B32` for destructive actions and error states only, re-verified the same way: 6.33:1 on Dominant, 5.58:1 on Secondary.

Neither value carries forward from revision 7's dark-tuned recomputation (`#C97268`/`#6FA37E`) — those were computed specifically for near-black backgrounds and are the wrong direction entirely for a light theme. Both status colors stay clearly muted/desaturated by design: Destructive reads as a dusty terracotta rather than an alert-red, Success reads as a dusty sage rather than a saturated green, unchanged in role and visual register from every revision back to revision 4.

**Coordination with Phase 2 (daily-roll category wheel):** this phase's anchor palette is now warm off-white (`#F6F5F2`), soft warm grey (`#E8E7E3`), a white elevation tone, and charcoal Ink (`#111111`) — light, warm-toned, and strictly non-chromatic except for the two small muted functional colors. Phase 2's wheel still needs its own broader categorical palette (N visually distinct wedge colors) to be legible as a wheel at all; this anchor palette still cannot itself supply that variety and Phase 2 should not attempt to force wedges into black/white/gray. What carries forward is the register, not the hues: PROJECT.md's wheel-specific constraint remains explicit and binding ("flat/muted, typography-led, restrained-motion treatment — no neon colors, no glossy 3D pointer, no confetti/flash-on-land, no clipart icons on wedges"), so Phase 2's wedge colors should land in a quiet, non-neon, non-gradient saturation-and-lightness range consistent with this phase's palette. Non-wedge wheel chrome (frame, pointer, typography, spin button) should draw directly from this phase's now-light palette for visual continuity.

This revision also resolves the forward-looking flag revision 7 raised: PROJECT.md's Context and Key Decisions sections now record the theme reversal as a whole-app decision, not a Phase-1-only choice — the earlier "light utility screens / dark moment screens" split is superseded. Phase 2 onward should adopt this same light editorial direction in their own future UI-SPECs, not a dark "moment screen" treatment, when those phases reach their own `/gsd-ui-phase` research.

---

## Why This Reads as Intentional, Not Generic

Not in the base template; added because revision 5's grayscale palette faced the risk of reading as an absence of decision, revision 6 had to answer a flatness complaint, and revision 7 had to actively dodge the "dark mode SaaS landing page" cliché. This revision faces its own distinct risk: a warm off-white background, an elegant serif headline, and generous negative space is itself close to a different, equally recognizable 2026-era cliché — the "premium editorial SaaS template" look, the light-mode counterpart to revision 7's dark-mode trap. Simply inverting revision 7's lightness values back without a considered justification would land squarely in that trap.

- **This is still matched against specific, real precedents, not a generic template.** docs/design-brief-2026-09-16.md names BeReal, Letterboxd, VSCO, COS campaign layouts, A24 movie posters, and Apple product pages as its mood references — a specific, considered blend, not "modern editorial app" as an undifferentiated genre.
- **The single biggest differentiator from the cliché is the typography choice, and it is a choice actively made against the brief.** Playfair Display + Inter is the single most common font pairing behind exactly this warm-off-white-editorial look in 2026 — it is what the brief itself asked for, and rejecting it a second time (see PROJECT.md moodboard note, reaffirmed 2026-09-16) in favor of Domine + Work Sans is precisely what keeps this from reading as a template pulled from a design-brief generator.
- **No pill buttons anywhere, despite the brief asking for them twice.** Full-radius CTAs are one of the more reliable visual tells of a generic "modern app" template; this revision holds the `md` (8dp) cap on every button and input with no exception, per PROJECT.md's standing constraint.
- **The dice-pip Brand Mark and dot-grid Background Texture are product-specific, hand-specified elements, not stock decoration** — carried forward from revision 6 unchanged in composition, only recolored. A generic editorial template reaches for stock photography or an abstract logomark; this app's own randomizer concept supplies its mark instead.
- **The warm-neutral choice is argued from this app's content, not decoration for its own sake.** This app's core content is real user-submitted photos. A warm, quiet, print-like backdrop is a considered choice for photo-forward apps specifically because it lets photos supply color and visual richness rather than competing with a busy or stark UI around them — the same reasoning that leads photography-editorial layouts (the brief's own VSCO/COS/A24 references) to default to warm neutrals rather than sterile pure white.
- **`#F6F5F2`, `#E8E7E3`, and `#111111` are adopted directly at the brief's own cited hex values, not naive inversions of revision 7's dark palette.** Direct citation is not the same as an unverified assumption: each was independently contrast-checked against every surface pairing it appears in before being locked (see Color above).
- **Typography still carries more of the identity than color does.** With zero saturated brand hue anywhere in the palette, Domine's serif presence at the now-larger Heading/Display sizes remains this app's single strongest visual signature — more pronounced in this revision than any prior one, given the increase from 28/20 to 40/22.
- **Destructive and Success are retained, unchanged in role, re-verified in value.** Keeping exactly two small, muted, non-neutral colors reserved for functional status signals, contrast-checked against the new warm surfaces rather than assumed, mirrors the same discipline every revision back to revision 4 has held.

---

## Visual Personality (Empty & Status States)

Not in the base template; added in revision 2, redesigned in revision 5, recolored (not restructured) across revisions 7 and 8. The goal remains a small, restrained amount of custom personality, never emoji or stock/AI-slop imagery, per PROJECT.md.

### Icon badge component contract

A small reusable pattern: a circular container (`avatar / icon badge` radius token, see Shape) at 40dp diameter, holding one 20dp `@expo/vector-icons` (Ionicons) glyph, with `card` elevation (see Elevation) so it reads as a small raised chip, plus a 1dp solid border in Ink at 20% opacity. Do not use `StyleSheet.hairlineWidth` for this border; use an explicit `1` (dp). Used exactly twice in this phase:

| Location | Icon | Glyph color | Badge fill | Badge border |
|----------|------|-------------|------------|---------------|
| Profile view screen, empty-bio state | Ionicons `create-outline` (pencil line) | Ink `#111111` | Dominant `#F6F5F2`, so the badge stands out from the Secondary-surface (`#E8E7E3`) card it sits on | 1dp solid Ink at 20% opacity |
| Verify-email waiting screen | Ionicons `mail-outline` | Ink `#111111` | Secondary surface `#E8E7E3`, so the badge stands out from the Dominant (`#F6F5F2`) background it sits on | 1dp solid Ink at 20% opacity |

Rule stated explicitly for the executor, unchanged from prior revisions: badge fill is always the *other* neutral surface relative to whatever it's placed on — never the same surface color as its immediate background, and never a low-opacity tint of Ink.

Explicitly NOT used for:
- The avatar placeholder, which stays a plain person-silhouette icon directly on the Secondary surface with no badge/border — rendered in Ink at 40% opacity (`rgba(17,17,17,0.4)`)
- The username-availability check icon, which stays inline in its existing Success-color treatment (see Interaction Contracts), no badge
- Any decorative/hero illustration — out of scope

---

## Brand Mark

Not in the base template; added in revision 6. This revision recolors it for the light theme; the composition, geometry, and implementation are unchanged.

### Composition
A single geometric mark built from five solid circular pips (dots) arranged in the classic die-face-five quincunx layout — no enclosing square or die outline. Unchanged from revision 6.

### Geometry
Specified on a 40 x 40 viewBox (all values multiples of 4):
- Each pip: 8dp diameter solid circle.
- Corner pip centers: (8,8), (32,8), (8,32), (32,32).
- Center pip: centered at (20,20).
- Color: Ink `#111111` pips at 100% opacity, fully transparent background — no fill, no border, no badge treatment. This is the value that changed from revision 7: the pips render in the new charcoal Ink (`#111111`) rather than the near-white Ink (`#EDEDED`), since the mark now sits on the light Dominant background again.

### Implementation
A custom vector component, unchanged: `components/brand/BrandMark.tsx`, rendering five `<Circle>` elements inside a single `<Svg viewBox="0 0 40 40">` via `react-native-svg`. Add as a dependency with `npx expo install react-native-svg` (resolves the SDK-57-compatible version automatically; do not hand-pin a version string). Same `checkpoint:human-verify` pattern as the font packages applies before installing: confirm the resolved package is the official `react-native-svg` (maintained under the `software-mansion` org) before it lands in `package.json`. The component's `color` prop default updates to Ink `#111111`.

### Placement in Phase 1
Used in exactly one place: the choose-method (entry) screen, centered above the three sign-in method buttons, rendered at `size={64}`, directly above the wordmark "RNDMRoll" set in Display type (now 40/46, see Typography). Not used anywhere else in Phase 1, unchanged from prior revisions.

---

## Background Texture

Not in the base template; added in revision 6. This revision recolors it for the light theme; the geometry, opacity ladder position, and single-screen scoping are unchanged.

### Token
`texture.dotGrid`:
- Dot diameter: 2dp, solid circle.
- Dot color: Ink `#111111` at 8% opacity (`rgba(17,17,17,0.08)`). This is the value that changed from revision 7 (`rgba(237,237,237,0.08)`) — the dots switch from light Ink back to charcoal Ink, since they now sit on the light Dominant background (`#F6F5F2`) again. This reverts to the same "dark dot on light background" physical scenario revision 6 originally specified (before revision 7's flip), so the 8% opacity value carries forward directly rather than being independently re-derived — the underlying visual situation is unchanged in kind, only the exact hex values are recomputed for the new warm palette. Revision 7's specific caveat about light dots visually "glowing" more than expected on a dark background no longer applies to this direction.
- Grid spacing: 16dp between dot centers, both axes.
- Coverage: full-bleed behind all content, lowest z-index layer, static — no parallax, no animation.

Executor should still spot-check on a real device before shipping, as a normal QA step — this direction (dark dot on light background) is the more predictable of the two cases perceptually, so it does not carry the elevated on-device-verification urgency revision 7 flagged for its own direction.

### Implementation
Same vector approach as the Brand Mark: a `<BackgroundDotGrid>` component in `components/brand/`, rendering a `react-native-svg` `<Pattern>` of 2dp `<Circle>` elements tiled via a `<Rect fill="url(#dotGrid)">`.

### Where it applies
Exactly one screen in Phase 1: the choose-method (entry) screen background only. Explicitly NOT applied to the onboarding form steps, the verify-email waiting screen, the profile view/edit screens, or any card/input/button surface — unchanged from prior revisions, for the same legibility and "no over-the-top" reasoning.

---

## Copywriting Contract

No em dashes used anywhere below, per PROJECT.md's standing "no em dashes" constraint. Unchanged by this revision — the theme reversal does not affect copy.

| Element | Copy |
|---------|------|
| Wordmark - choose-method screen (Display role, paired with Brand Mark) | "RNDMRoll". Per PROJECT.md, this is a working title and the final consumer-facing name is still TBD. Bind it to a single string prop on the wordmark component. |
| Heading - verify-email waiting screen (Display role) | "Check your email" |
| Primary CTA - choose-method screen | Three method buttons, no generic "Continue": "Continue with Email", "Continue with Apple" (iOS only), "Continue with Google" |
| Primary CTA - name step | "Continue to username" |
| Primary CTA - username step | "Continue to photo" |
| Primary CTA - photo step (required weight) | "Finish setup" |
| Secondary CTA - photo step (skippable, D-06) | "Skip for now", rendered as a plain text button (no fill, no border) |
| Primary CTA - profile edit | "Save changes" |
| Empty state (profile VIEW screen, bio absent) - heading | "Add a bio" |
| Empty state (profile VIEW screen, bio absent) - body | "Tell people what you're rolling for." Shown next to the icon-badge treatment described in Visual Personality above; tapping the whole empty-state row navigates to profile edit. |
| Empty state (profile EDIT screen, bio field) | Same copy, "Tell people what you're rolling for.", rendered as the standard `TextInput` `placeholder` prop. |
| Error state - duplicate email | "That email's already registered. Log in instead." |
| Error state - username taken | "That username's taken. Try one of these:" followed by up to 3 tappable alternates |
| Error state - network failure | "Couldn't connect. Check your connection and try again." |
| Destructive confirmation - log out | "Log out of RNDMRoll? You'll need to sign back in." Buttons: "Log out" / "Stay logged in" |

---

## Interaction Contracts

Not part of the base template. Unchanged by this revision — state machines, timing, and gating logic are theme-independent; only the hex values the color-role names (destructive/success/Ink) resolve to changed, per Color above.

### Username step state machine
- **Idle:** field prefilled with the suggestion returned by `/usernames/suggest`, fully editable, not a blank field (D-05).
- **Checking:** on edit, debounce 400ms, then show a small inline spinner plus "Checking..." next to the field. Block submit until the check resolves.
- **Available:** success-color check icon plus "Available" label next to the field.
- **Taken:** destructive-color "That username's taken." plus up to 3 tappable alternate suggestions.
- **Insert-time conflict:** if the live check passed but the server-side insert still rejects, show the same "Taken" state copy and refreshed alternates after submit, not a generic error toast.

### Verify-email screen
- **Waiting state:** shows the entered email address, the `mail-outline` icon badge, and "We sent a verification link to {email}. Tap it to continue." plus a "Resend email" text button.
- **Resend cooldown:** after tapping Resend, disable the button and show a 30-second countdown, then re-enable.
- **Deep-link return:** when verification completes, show a brief confirmation state (success-color checkmark plus "Verified") then auto-advance to the name step. No extra manual "Continue" tap and no silent jump with zero feedback.

### Social sign-in buttons
- **Default:** full-width, secondary-surface-colored buttons, each with a provider icon plus label.
- **Loading:** on tap, swap the tapped button's label for an inline spinner and disable all three method buttons until the native SDK round-trip resolves. Success routes onward; failure shows inline error copy below the button group; user-initiated cancellation returns silently to idle with no error shown.
- **Platform gating:** Apple Sign In renders on iOS only. On Android, the choose-method screen shows Email + Google only.

### Form validation timing
- Validate on blur for email/password/name fields. Validate live (400ms debounce) only for username availability.
- Error text sits directly below its field, left-aligned, in the destructive color.
- On submit with unresolved errors, focus jumps to the first invalid field.

### Session-restore and font-load gate on launch
- The root layout calls `SplashScreen.preventAutoHideAsync()` immediately, then in parallel: (a) loads the three font files via `useFonts`, and (b) reads SecureStore and calls `/auth/refresh`.
- Hold the native splash screen until both conditions settle, then call `SplashScreen.hideAsync()` and route to `(app)` or `(auth)`. Font-load failure falls back to the platform system font for that session, unchanged from prior revisions.

### Photo step (D-06)
- "Finish setup" (accent-filled, primary weight, `raised` elevation) is visually dominant; "Skip for now" (plain text, no elevation) sits below it.

### Accessibility: icon-only controls
Every icon-only control must carry an explicit `accessibilityLabel`, unchanged from prior revisions: back button, close button, "Resend email" (with live-countdown label updates), the username-availability status icon, and the avatar placeholder/photo-picker control. This list is a floor, not a ceiling.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|--------------|
| shadcn official | none | not applicable, React Native/Expo target uses no shadcn registry |
| third-party | none | not applicable |

Unchanged by this revision.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: pending
- [ ] Dimension 2 Visuals: pending
- [ ] Dimension 3 Color: pending
- [ ] Dimension 4 Typography: pending
- [ ] Dimension 5 Spacing: pending
- [ ] Dimension 6 Registry Safety: pending

**Approval:** pending re-verification for revision 8 (dark-to-light theme reversal, editorial direction). All 6 dimensions reset to pending per the write contract for this revision.

### Focal Points (carried forward from revision 4 checker recommendation, re-confirm on this revision)

The primary CTA (`raised` elevation, Ink fill, Dominant-colored label) is the focal point on every onboarding step (choose-method, name, username, photo). On the profile view screen, the avatar + name/username pairing is the focal point, with the empty-bio icon badge as a clearly secondary element beneath it.

On the choose-method screen specifically, the Brand Mark + wordmark lockup and the background texture sit compositionally above the three method buttons but must not outweigh them as the screen's focal point. This is enforced the same way as prior revisions: the mark and wordmark carry no elevation and no fill/badge container, and the texture is a static, lowest-z-index background layer at 8% opacity — versus the CTA group's `raised` elevation (now a soft shadow, not a glow, matching this revision's shadow-based elevation mechanism, see Elevation) and solid 100%-opacity Ink fill.
