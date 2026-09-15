---
phase: 1
slug: foundation-accounts
status: draft
reviewed_at: pending
shadcn_initialized: false
preset: none
created: 2026-09-15
revised: 2026-09-15
revision: 7
revision_reason: "Revision 7 responds to user feedback on the revision 6 visual sample: 'black background and uses a white as a text instead maybe.' This inverts the theme from light-primary to dark-primary — near-black backgrounds, near-white text and accents — while keeping every locked grayscale-discipline decision from revisions 5 and 6 fully in force: no chromatic brand accent is introduced, only the neutral Ink color plus the two small muted functional colors (destructive, success) carry any visual weight, exactly as before. This also reverses PROJECT.md's earlier 'Phase 1 stays light because it is a utility screen' framing (see PROJECT.md's moodboard note distinguishing dark/cinematic moment screens from light utility screens) — the user's repeated push toward black across revisions 4 through 6, culminating in this explicit request, makes clear the preference is for a dark-primary app overall, not a light-utility/dark-moment split; this UI-SPEC does not itself edit PROJECT.md, but that standing note should be revisited separately. Every color role is recomputed, not naively inverted: Dominant becomes `#121212` (Material dark-theme baseline surface, chosen over pure `#000000` to avoid OLED black-crush/flash-glare, the same reasoning revision 5 used to avoid pure white-on-pure-black at the other end); Secondary becomes `#1E1E1E`, one tonal step lighter than Dominant rather than darker, because dark-theme surfaces read as more elevated by getting lighter, the inverse of the light theme's shadow-gets-darker convention; Ink/Accent becomes `#EDEDED`, a near-white rather than pure `#FFFFFF`, mirroring revision 5's own near-black-not-pure-black reasoning to avoid text vibration at the opposite extreme. Muted text becomes `#888888` (4.71:1 on Secondary `#1E1E1E`, 5.29:1 on Dominant `#121212`, both re-verified against the new dark backgrounds). Divider/Border becomes `#333333`, now lighter than both structural surfaces it separates (the direction flips because it must show up against dark backgrounds rather than light ones); a new elevation-only tonal step `#262626` is added to the Color table (utility, not part of the 60/30/10 budget) to give the Elevation system's `card` tier a genuine third lightness step beyond Dominant and Secondary — notably the exact hex value that was revisions 1 through 6's light-theme Ink color, an intentional continuity callback, not a coincidence. Destructive and Success are both recomputed because the revision 5/6 values were tuned for legibility against light backgrounds and fail badly against the new dark ones (recheck: the old `#9A3B32` destructive measures only 2.42:1-2.72:1 against the new dark surfaces, well under the 4.5:1 floor; the old `#416B4C` success measures only 2.73:1-3.06:1, also failing) — Destructive is now `#C97268` (a lightened muted terracotta, 4.84:1 on `#1E1E1E`, 5.44:1 on `#121212`) and Success is now `#6FA37E` (a lightened muted sage, 5.73:1 on `#1E1E1E`, 6.44:1 on `#121212`), both re-verified to clear 4.5:1 against both dark surfaces while staying as visibly desaturated/muted as the values they replace, so neither reads as a color pop. The Elevation section's mechanism is reworked, not just recolored: a black drop shadow on a near-black background is mathematically near-invisible (`#000000` against `#121212` measures roughly 1.05:1 contrast, imperceptible), so this revision replaces shadow-based elevation with tonal elevation as the primary depth cue for the `subtle` and `card` tiers (each tier is a lighter background tone than the one below it, the same fix Material Design's own dark-theme specification applies for the same reason), keeps a much-reduced shadow only as a supplementary edge-softening cue on iOS, and gives the `raised` CTA tier a soft light-colored glow (`shadowColor` set to Ink, not black) instead of a dark shadow, since a bright Ink-filled element is the one surface in this system that can cast a visible light halo against a dark background. The Brand Mark's five pips and the Background Texture's dot-grid both switch from the old dark Ink (`#262626`) to the new light Ink (`#EDEDED`), at the exact same geometry, opacity ladder (8% texture / 15% / 20% / 35% / 40% / 100%), and single-screen scoping already locked in revision 6 — no change to composition, only to which end of the lightness scale Ink sits at. The Platform lock flips from `userInterfaceStyle: light` to `userInterfaceStyle: dark`. The icon badge component's fill-swap rule (badge fill is always the surface tone other than its host) is preserved exactly, just recolored: a Dominant-fill badge on a Secondary-surface host, a Secondary-fill badge on the Dominant background. 'Why This Reads as Intentional, Not Generic' gains an update defending the dark-theme choice on its own terms: this app's core content is real user photos, and a dark chrome is a considered choice for photo-forward apps for the same reason many of them default dark, letting photos supply the visual richness rather than a colorful UI competing with them — while explicitly avoiding the near-black-plus-single-accent-pop AI cliché that same section has warned against before, because there is still no saturated accent color anywhere in this palette, only the neutral white/gray system plus the two small muted functional colors, unchanged in role from every revision back to revision 4. The Phase 2 wheel-color coordination note is updated to cite the new dark anchor palette, and now flags that Phase 1 and Phase 2/3 share a dark base for the first time (Phase 2/3 were already specified dark per PROJECT.md's moment-screen framing), which weakens the rationale for treating Phase 5's utility screens as light-by-default too — flagged for that phase's own future UI-SPEC to decide, not resolved here. Checker Sign-Off reset to pending."
---

# Phase 1 - UI Design Contract

> Visual and interaction contract for frontend phases. Generated by gsd-ui-researcher, verified by gsd-ui-checker.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | none. shadcn is a web/Tailwind registry tool and does not apply here: RESEARCH.md locks this phase to React Native / Expo SDK 57 with expo-router (not React/Next.js/Vite), so the shadcn init gate does not fire. |
| Preset | not applicable |
| Component library | none (custom). Plain React Native primitives (View, Text, Pressable, TextInput) styled from a single shared tokens module (e.g. `lib/theme/tokens.ts`), not a third-party RN UI kit (React Native Paper, NativeBase, Tamagui). Default, applied because no library was named in CONTEXT.md/RESEARCH.md: a generic kit's default button/input shapes would fight PROJECT.md's specific constraints (no pill buttons, no gradients, flat/muted) more than they'd help. |
| Icon library | `@expo/vector-icons` (Ionicons as the default icon set). Bundled with Expo SDK 57, no extra install. Required because PROJECT.md bans emoji-as-icons, so every icon-shaped element (back, close, checkmark, resend, avatar placeholder, and the two icon badges added in revision 2 — see "Visual Personality" below) must come from this library, never a raw emoji character. |
| Font | **Domine** (serif, display/heading only) at one weight, `Domine_600SemiBold`, plus **Work Sans** (body/UI) at two weights, `WorkSans_400Regular` and `WorkSans_600SemiBold` — three font files total, loaded via `@expo-google-fonts/domine` and `@expo-google-fonts/work-sans` (both v0.4.2, confirmed against the public npm registry API; both list `homepage: github.com/expo/google-fonts`, the same publishing pattern as every other Expo Google Fonts package, and their package contents were inspected directly via unpkg to confirm the exact exported weight names used below — `Domine_600SemiBold`, `WorkSans_400Regular`, `WorkSans_600SemiBold` all exist as real files) and `expo-font`, gated with `expo-splash-screen` (v57.0.9). Replaces revision 2/3's Plus Jakarta Sans, which the user explicitly rejected alongside the color palette ("use different fonts... make it look nice"). Domine is a serif designed specifically for on-screen headline legibility (sturdy strokes, contained proportions) rather than a digitized book/print face; it is used here only for Heading/Display roles and the wordmark, never for body or UI chrome, which keeps it a considered accent detail rather than a legibility risk in dense form text. Work Sans is a humanist grotesque with enough real character (open apertures, two-story lowercase 'a') to avoid reading as a generic default, while staying clean and legible enough for forms, labels, and buttons. Neither is Inter or Space Grotesk, the two faces explicitly called out as the overused "safe"/default look of AI-generated UI. See "Why This Reads as Intentional, Not Generic" below for the fuller rationale, and Typography below for the per-role mapping. Same `checkpoint:human-verify` pattern as prior revisions applies before installing: confirm both packages resolve to `github.com/expo/google-fonts` before adding them as dependencies. Unchanged by this revision — the theme inversion is a color/elevation change only, not a typography change. |

**Platform lock:** Phase 1 now designs for **dark mode only** (no light-mode variant specified for this phase), reversing revisions 1 through 6's light-mode-only lock per explicit user feedback on the revision 6 visual sample: "black background and uses a white as a text instead maybe." Set `"userInterfaceStyle": "dark"` in `app.json` to enforce this at the OS level — the same single-theme-only rationale as before, just inverted: without this, system light mode still partially applies (status bar, keyboard, native text-input chrome) even though the screens themselves are styled dark-only, producing a visibly broken mixed state.

This also reverses PROJECT.md's recorded "Phase 1 stays light because it's a utility screen" framing (see PROJECT.md's moodboard note distinguishing dark/cinematic "moment" screens — daily roll, reveal, camera capture, new-entry logging — from light "utility" screens — accounts, profile, diary, friend board, settings). The user's repeated push toward near-black tones across revisions 4 through 6, culminating in this explicit request, makes clear the preference is for a dark-primary app overall, not a light-utility/dark-moment split. This UI-SPEC does not itself edit PROJECT.md; that standing note should be revisited separately so it doesn't contradict this phase's now-locked dark theme.

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

Exceptions: `44` as a minimum touch-target hit-area (via padding or `hitSlop`, not a new spacing token) for icon-only controls and tappable rows: back button, close button, the username-availability status icon, the "resend email" tap target. This follows standard iOS HIG (44pt) / Material (48dp, rounded down to this project's nearest declared scale point) touch-target guidance and applies everywhere an icon-only control appears in this phase. The 40dp icon-badge diameter added in revision 2 (Visual Personality section) is a decorative element, not a tap target, so it is exempt from the 44dp minimum. The 2dp background-texture dot diameter added in revision 6 (Background Texture section) is likewise exempt: it is a hairline/sub-element value in the same register as the existing 1dp/1.5dp border widths declared in Color and Elevation, not an element dimension on the 4-multiple scale.

Unchanged by this revision.

---

## Shape (Radius)

Not in the base template; declared here because PROJECT.md's standing "no pill-shaped buttons" constraint needs a concrete home or the executor will default to a large/full radius.

| Token | Value | Usage |
|-------|-------|-------|
| sm | 4 | Small chips/status pills (e.g. "Available" / "Taken" inline badges next to the username field) |
| md | 8 | Buttons, text inputs, cards. This is the maximum radius for any button or input in this phase, enforcing PROJECT.md's "no pill-shaped buttons" rule (a pill is defined here as radius >= half the element's height). |
| avatar / icon badge | circular (50% of width/height) | Profile photo and avatar-placeholder, PLUS the two icon badges added in revision 2 (empty-bio icon, verify-email icon — see Visual Personality). Explicit, named exception to the radius cap, extended in revision 2 to cover small circular icon containers as well as the avatar itself: both are the Instagram-parity / "soft icon chip" convention this phase follows, not a step toward pill-shaped buttons or CTAs, which remain capped at `md` (8) with no exceptions. |

Unchanged by this revision.

---

## Typography

React Native `lineHeight` is an absolute number, not a ratio, so both are declared explicitly. Font weight is expressed as a `fontFamily` string, not a numeric `fontWeight` prop — see platform note below.

| Role | Size | Font Family | Line Height |
|------|------|-------------|-------------|
| Body | 16 | `WorkSans_400Regular` | 24 |
| Label | 14 | `WorkSans_400Regular` | 20 |
| Button / CTA label | 16 | `WorkSans_600SemiBold` | 24 |
| Heading | 20 | `Domine_600SemiBold` | 26 |
| Display | 28 | `Domine_600SemiBold` | 34 |

Exactly three font files are loaded application-wide: `Domine_600SemiBold` (Heading, Display, and the wordmark only), `WorkSans_400Regular` (Body, Label), and `WorkSans_600SemiBold` (Button/CTA label, and text links — see Color below). The Button/CTA role reuses Body's size and line-height but switches family and weight to Work Sans SemiBold; it is declared as its own row, distinct from revision 3's table, because this revision needs an explicit non-serif emphasis weight for buttons and links now that Domine is restricted to headings only (requirement: the display face is used with restraint, never for body/UI text).

Platform note (replaces v1's system-font Roboto/San Francisco note, which no longer applies): set `fontFamily` directly per role in the tokens module, and do NOT also set a numeric `fontWeight` alongside a custom-loaded `fontFamily` — on Android, React Native's font matcher does not reliably combine a custom family with a separate `fontWeight` override and can silently fall back to that family's default weight, producing a Body-weight Heading with no visible error. Because this revision loads three distinct named font files (`Domine_600SemiBold`, `WorkSans_400Regular`, `WorkSans_600SemiBold`), there is no cross-platform weight-mapping branch to maintain: each role points at one exact font file on both iOS and Android. If a font fails to load (see "Session-restore and font-load gate" in Interaction Contracts), the fallback is the platform system font at the equivalent numeric weight (400/600) via `fontWeight`, applied only in that fallback branch, never mixed with a custom family — in that fallback branch, both the Domine and Work Sans roles collapse onto the same single system font, so the Heading/Display vs. Body/Label distinction is carried by size alone for the remainder of that app session rather than by two different type families. This is an accepted, expected degradation, not a bug to chase at runtime; restoring the two-family look simply requires a successful font load on the next app launch.

Typography sizes, weights, roles, and the platform-note mechanics are all unchanged by this revision — only the color each role renders in changes (see Color below). Default text color is now Ink `#EDEDED` rendered on `#121212`/`#1E1E1E` dark surfaces instead of Ink `#262626` on light surfaces.

### Display Role Application (revision 6, unchanged by revision 7)

Revision 5 declared the Display role (28/600 Domine) but every screen in this phase defaulted to Heading (20/600 Domine) in practice, leaving Display effectively unused — a second symptom of the same "really plain" feedback that prompted revision 6: a declared-but-unused type step contributes no visual presence. Revision 6 made exactly two deliberate promotions from Heading to Display, using the sizes already in the table above (no new size or weight is added), and revision 7 does not change this:

- **Choose-method screen wordmark:** "RNDMRoll" (see Copywriting Contract for the exact string and its working-title status) is set in Display, positioned directly beneath the Brand Mark (see "Brand Mark" below) — together they are this phase's one logo-lockup moment, and the wordmark's role there is identity, not information density.
- **Verify-email waiting screen heading:** "Check your email" (see Copywriting Contract) promotes from Heading to Display. This screen carries the lowest content density in the phase (one icon badge, one paragraph, one text button, no form fields), so a larger headline has room to breathe without crowding anything else on screen.

Both promoted headings share the same 28/600 Domine values from the table; they are distinguished from each other by role (brand identity vs. screen headline), not by size. Everywhere else stays at Heading (20/600 Domine): the name/username/photo onboarding step headings, and the profile view/edit screen headings. This exclusion is deliberate, not an oversight — these are dense, form-carrying screens where a larger headline would compete with inputs and validation copy for attention, and promoting every screen's heading to Display would flatten the effect entirely: if every screen has a Display-sized moment, none of them reads as one. Two chosen uses out of roughly seven screens in this phase is what keeps Display feeling like emphasis rather than a uniform size bump.

---

## Elevation (Shadow / Tonal Elevation System)

Not in the base template; added in revision 2 to give the UI quiet depth instead of the flat, shadow-less baseline that read as unfinished. Reworked in this revision from a shadow-based mechanism to a **tonal elevation mechanism** as the primary depth cue, for a concrete, measurable reason: a black drop shadow on a near-black background does not read. `#000000` measured against the new Dominant background `#121212` is roughly **1.05:1 contrast** — imperceptible, not merely subtle. Revisions 2 through 6's shadow-only system depended entirely on a light background making a dark shadow visible; that assumption no longer holds once the background itself is dark.

The fix: each elevation tier is now a **lighter** background tone than the one beneath it, the same fix Material Design's own dark-theme specification applies for the same reason (dark-theme surfaces are conventionally lit by getting lighter as they rise in the stack, not by casting a visible shadow). This is the inverse of the light-theme mechanism, where each tier got a *darker* shadow: in dark mode, lightness itself carries the depth signal.

| Token | Fill / tonal shift | iOS shadow (supplementary edge cue only, not the primary signal) | Android `elevation` | Usage |
|-------|---|---|---|---|
| subtle | Secondary surface `#1E1E1E` — already the first tonal step lighter than Dominant `#121212`; the Dominant/Secondary surface pairing itself *is* this tier's elevation cue, no separate treatment needed | none | `1` (native-idiom nod only) | Secondary-surface cards at rest: profile field rows, the alternate-username suggestion chips |
| card | New elevation-only tone `#262626` (see Color) — a second tonal step lighter again than Secondary, applied only to surfaces without a more specific fill rule elsewhere in this document | `shadowColor:#000000` / `{width:0, height:2}` / opacity `0.3` / radius `6` — kept only as a faint edge-softening cue; do not rely on this for the tier's legibility, the tonal shift is what actually reads | `3` | The profile-view container card, modal/sheet surfaces (the log-out confirmation sheet). The two icon badges (Visual Personality section) pick up `card`'s edge-definition treatment (border, faint shadow) but keep their own fill rule from Visual Personality (Dominant/Secondary swap for host-contrast) rather than adopting `#262626` — their fill is chosen for contrast against their specific host surface, not for generic elevation. |
| raised | Ink fill `#EDEDED` (unchanged role: the primary CTA button's fill, see Color) — already the single brightest value in the whole system, so nothing can tonally elevate "above" it; this tier's depth cue is the fill-inversion itself, the same as every prior revision | `shadowColor:` Ink (`#EDEDED`) / `{width:0, height:2}` / opacity `0.2` / radius `10` — a soft **light glow**, not a dark shadow. A bright Ink-filled element is the one surface in this system where a light-colored halo will actually read against a dark background; a black shadow under it would suffer the exact same near-invisibility problem described above. | `6` | The primary CTA button only, default/enabled state |

This replaces revision 3-6's shared-`#000000`-shadow-color history: those revisions kept all three tokens on one neutral black shadow color because the background was light and any neutral shadow read cleanly. That reasoning no longer transfers, which is why `raised` now uses a fundamentally different shadow color (light, not black) from `subtle`/`card` (which keep a much-reduced black shadow purely as a faint edge cue, not a legibility mechanism).

Platform note: Android's `elevation` prop only renders a visible shadow on a `View` with an explicit opaque `backgroundColor` — unchanged from prior revisions. Its values are retained here mostly as a native-idiom nod: Android's ambient+key-light shadow renderer draws a soft occlusion gradient rather than a flat offset rectangle, so it is expected to remain faintly visible even on dark surfaces, unlike iOS's literal `shadowColor` value — but this expectation should be spot-checked on a real Android device before shipping, the same "verify on-device" discipline already required for the Background Texture's opacity bounds (see below), rather than trusted on the strength of this description alone.

Disabled/loading CTA states use `subtle` instead of `raised`, since a glowing button under an inert control reads as a bug, not polish.

Negative scope, stated explicitly so the executor doesn't over-apply: no elevation/tonal-shift on avatars, text inputs, or plain-text buttons ("Skip for now", "Resend email", "Forgot password?", the method-toggle links). Three tokens used on a small, named set of surfaces is what keeps this reading as "quiet depth" rather than decoration sprinkled everywhere.

Also no elevation/tonal-shift on the Brand Mark or on the Background Texture layer (both added in revision 6, see those sections below) — stated explicitly because the icon badge's `card`-elevation precedent could otherwise lead the executor to shadow or tonally lift the mark by analogy. The mark is a flat vector glyph directly on its host surface, the same way default text has no shadow; the texture is a background-layer fill, one z-index below everything else on screen, and giving a background layer elevation treatment is not a meaningful operation regardless.

---

## Color

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `#121212` (near-black, true neutral, R=G=B, zero hue bias) | Screen backgrounds, default surface behind all onboarding and profile screens |
| Secondary (30%) | `#1E1E1E` (true neutral dark gray, R=G=B, zero hue bias) — the first tonal step **lighter** than Dominant | Cards, input field fill, section backgrounds, bottom tab bar (once it exists in later phases) |
| Elevation tier / "card" tone (utility, not part of 60/30/10) | `#262626` (true neutral dark gray, R=G=B) — a second tonal step lighter again than Secondary | The Elevation system's `card` tier fill, for surfaces without a more specific fill rule elsewhere in this document (profile-view container card, modal/sheet surfaces). See Elevation above. |
| Accent / Ink (10% interactive budget, plus default text — see note below) | `#EDEDED` (near-white, true neutral, R=G=B, no cool/warm undertone) | See "Accent/Ink reserved for" below |
| Muted text (utility, not part of 60/30/10) | `#888888` (true neutral mid-gray) | Secondary/helper text, timestamps, and other non-primary copy — see note below |
| Divider / Border (utility, not part of 60/30/10) | `#333333` (true neutral dark-mid gray, lighter than both Secondary `#1E1E1E` and the card tone `#262626`) | Hairline dividers, and the default border on static secondary-surface cards/chips — see note below |
| Destructive | `#C97268` (muted terracotta/brick) | Destructive actions and error states only |
| Success | `#6FA37E` (muted sage green) | Username-available status only, see Status colors below |

This inverts revision 5/6's strictly grayscale light palette (white `#FFFFFF` / light gray `#F0F0F0` / near-black ink `#262626`) to a strictly grayscale **dark** palette, per explicit user direction after reviewing the revision 6 sample: "black background and uses a white as a text instead maybe." Every structural color in this table remains a true neutral (equal R/G/B channels, zero hue bias) — no warm or cool tint anywhere in the background/card/text/border system, exactly as before. Destructive and Success remain the only non-neutral colors in the phase, recomputed for the new dark backgrounds (see below) but unchanged in role: small, muted functional signals, not a "color pop."

Values are chosen deliberately, not by naively inverting the light-theme hex codes:
- **Dominant `#121212`**, not pure `#000000` — this is the Material Design dark-theme baseline surface color, chosen specifically to avoid the flash-glare/black-crush a true `#000000` background produces (especially on OLED displays, where pure black can look like a "hole" rather than a surface, and where UI elements can appear to flicker against it during transitions). This mirrors the exact reasoning revision 5 used for `#262626` over pure black text on the light palette, now applied at the opposite end.
- **Secondary `#1E1E1E` is lighter than Dominant, not darker.** Dark-theme surfaces conventionally read as "elevated" by getting lighter, the inverse of the light theme's shadow-gets-darker convention (see Elevation above for the full mechanism this drives).
- **Ink `#EDEDED` is a near-white, not pure `#FFFFFF`** — the same anti-vibration reasoning revision 5 applied to near-black text on a light background, mirrored here: at the font weights and sizes used throughout this phase's dense form UI, pure white on near-black can read as a harsher vibration than a very-bright neutral gray, while `#EDEDED` keeps the same crisp, high-contrast register (16.0:1 against Dominant, 14.24:1 against Secondary, 12.92:1 against the card tone — all far above the 4.5:1 floor) and stays fully legible.

**Accent/Ink reserved for** (explicit list; nothing else may use this color at full strength):
- The single primary-path CTA button fill per screen (Continue on name/username/photo steps, Save changes on profile edit). Label text on this fill is `#121212` (Dominant), chosen explicitly for maximum legibility against a near-white fill — this is the one place the dark Dominant color is used as text rather than as a surface color, the inverse of revision 5/6's white-on-ink CTA label.
- Selected/active state, declared with both a color step and a width step so the state doesn't rely on hue alone: a chosen sign-in method's outline, and a focused text input's border, go from Ink at 15% opacity / 1dp width at rest to Ink at 100% opacity / 1.5dp width when active or focused.
- Text links ("Forgot password?", "Log in instead" / "Sign up instead"): Ink at 100% **and underlined**, set in `WorkSans_600SemiBold`. The underline and weight are the affordance signal here, not color alone — link color and default body-text color are the same hue, so without the underline and weight a link would be indistinguishable from a sentence around it.
- The onboarding step-progress indicator: the active step dot is Ink at 100%; inactive step dots are Ink at 20% opacity, same size. Both states need a declared value since they're the same hue family.
- The Brand Mark's five pips (see "Brand Mark" below): Ink at 100%, no fill/badge/border treatment around them. This is a full-strength decorative-register use, justified because it is the app's one deliberate logo element, used exactly once in this phase, not a recurring decorative motif.

**Default text color** (exempt from the reserved-for list above, the same way body-copy color was implicit and unbudgeted in prior revisions): all body copy, headings, and labels default to Accent/Ink `#EDEDED`. This dual duty — near-universal text color, and the sole interactive accent — is intentional and does not dilute the accent's signal value, because in this revision CTA/active-state prominence comes from solid-fill inversion (Dominant-colored text on an Ink fill, versus Ink text on a dark surface), the `raised`/`card` elevation tokens, and the underline/weight/opacity/width steps declared above, not from hue exclusivity. Nothing in this phase uses Ink at full strength purely as decoration except the Brand Mark noted above; every other full-strength use is either running text or one of the four bullets above.

**Muted text**: `#888888`, reserved for secondary/helper copy that is deliberately de-emphasized relative to default body text — e.g. timestamps, de-emphasized captions, and any helper text that is not an active validation error (validation errors stay Destructive, per Interaction Contracts). Contrast-checked at 4.5:1+ against both dark surfaces it can appear on: 5.29:1 on `#121212`, 4.71:1 on the `#1E1E1E` secondary surface — both clear the standard body-text minimum, re-verified for the new dark backgrounds and, as before, deliberately not copied from Instagram's own dark-mode secondary gray verbatim (chosen to hold the 4.5:1 floor with intent, not incidentally).

**Divider / Border gray**: `#333333`, a decorative-register value not held to text-contrast ratios (it separates surfaces, it does not carry text). Two uses: (1) plain hairline dividers, and (2) an explicit 1dp solid border on static secondary-surface cards and chips that have no other border contract — profile field rows, alternate-username suggestion chips, and the icon-badge component (see Visual Personality). This second use remains load-bearing, not decorative, for the same reason it was in the light palette: the Dominant/Secondary lightness step (`#121212` vs `#1E1E1E`) is a real but small step — roughly the same magnitude of perceptual difference as the light palette's own `#FFFFFF`/`#F0F0F0` step — so the border is what keeps a card legible as a distinct surface rather than a slightly-lighter patch of the background, especially outdoors/at low screen brightness. The direction flips from the light palette (there the border had to read *darker* than both surfaces it separated; here it must read *lighter* than both) but the underlying design reason is identical. (Interactive surfaces — text inputs, the chosen sign-in method — keep their existing Ink-opacity border contract from the bullet above instead of this token; this token is only for surfaces that have no other declared border.)

**CTA fill states:** default/enabled = Ink `#EDEDED` solid, label `#121212`. Disabled, or awaiting a blocked action (e.g. Continue while a username check is in flight) = Ink at 35% opacity as the fill (`rgba(237,237,237,0.35)`), label `#121212` at 70% opacity, paired with `subtle` elevation (see Elevation) instead of `raised`. A visibly faded near-white button, not a new hue, is what signals disabled here; this reads correctly against both `#121212` and `#1E1E1E`, the two backgrounds a CTA can sit on in this phase.

**Status colors** (exempt from the 60/30/10 budget as small inline utility signals, not surfaces, the same exemption pattern used in prior revisions):
- Success `#6FA37E` for the username-available check icon and label only. Contrast-verified against both dark backgrounds it can appear on in this phase: 6.44:1 on `#121212`, 5.73:1 on the `#1E1E1E` secondary surface — both comfortably clear the 4.5:1 minimum for 14dp label text. The revision 5/6 value (`#416B4C`) was re-measured against these same two surfaces and fails badly (3.06:1 / 2.73:1), which is why this revision lightens the hue rather than reusing it.
- Destructive `#C97268` for destructive actions and error states only, contrast-verified the same way: 5.44:1 on `#121212`, 4.84:1 on `#1E1E1E`. The revision 5/6 value (`#9A3B32`) also fails against the new dark surfaces (2.72:1 / 2.42:1), for the same reason: both legacy functional colors were tuned to sit dark-on-light and are simply too dark themselves to register against a dark background.

Both status colors stay clearly muted/desaturated by design, matching the visual register of the values they replace: Destructive reads as a dusty terracotta rather than an alert-red, Success reads as a dusty sage rather than a saturated green. They exist purely as functional usability signals, not as the "color pop" the user has explicitly asked this palette to avoid, and red vs. green sit far enough apart in hue that they can't be confused with each other or with the neutral Ink accent, which has no meaningful hue at all. Keeping the same pattern (two small, muted, non-neutral functional colors, nothing else) echoes the precedent named in revision 5: Instagram's own chrome — including its dark mode — is otherwise black-and-white but still shows a small colored checkmark/error state during its own username validation, proof that "keep it black and white" was never meant as "zero color anywhere."

**Coordination with Phase 2 (daily-roll category wheel):** this phase's anchor palette is now near-black (`#121212`), true neutral dark grays (`#1E1E1E`, `#262626`), and near-white ink (`#EDEDED`) — still a strictly grayscale system, inverted in lightness but not in discipline. Phase 2's wheel still needs its own broader categorical palette (N visually distinct wedge colors) to be legible as a wheel at all, and a grayscale anchor palette still cannot itself supply that variety — Phase 2 should not attempt to force wedges into black/white/gray. What carries forward is the register, not the hues: PROJECT.md's wheel-specific constraint remains explicit and binding here too ("flat/muted, typography-led, restrained-motion treatment — no neon colors, no glossy 3D pointer, no confetti/flash-on-land, no clipart icons on wedges"), so Phase 2's wedge colors should land in the same quiet, non-neon, non-gradient saturation-and-lightness range this phase's palette establishes, even though the wedges themselves will need real chromatic variety this phase's palette does not have. Non-wedge wheel chrome (frame, pointer, typography, spin button) should draw directly from this phase's dark palette for visual continuity with the rest of the app.

New in this revision: PROJECT.md's moodboard note already specified Phase 2/3 ("moment" screens — roll, reveal, camera, entry) as dark/cinematic, in explicit contrast to Phase 1's former light "utility" treatment. This revision means Phase 1 and Phase 2/3 now share the same dark tonal base for the first time, which weakens (without resolving) the rationale for keeping Phase 5's utility screens (diary, friend board, settings) light by default under that same moodboard framing. This is flagged here as a forward-looking consideration for Phase 5's own future UI-SPEC to decide, not resolved in this document. Treat this phase's palette as the tonal calibration reference for Phase 2's wheel-color research, not a hard color lock.

---

## Why This Reads as Intentional, Not Generic

Not in the base template; added because revision 5's grayscale palette faced a distinct risk from revision 4's (read as the *absence* of a decision rather than a deliberate one). Revision 6 then had to answer a flatness complaint. Revision 7 faces a third, related risk: a dark near-black background with a single bright accent is one of the most recognizable AI-generated-UI clichés there is (the "dark mode SaaS landing page" look), and simply inverting the light palette's lightness values without a considered justification would land squarely in that trap.

- **This is still a named precedent, not an absence of color.** The user's original palette direction pointed at a specific, real, widely-recognized reference: "you know how Instagram does?" Instagram itself ships a dark mode built on this exact grammar — near-black ground, near-white default text/icons, dark gray secondary elements, red/green reserved narrowly for a handful of out-of-band status signals — so this revision is still matching a named, deliberate precedent, now at its dark variant, not inventing a new one from scratch.
- **The dark chrome is chosen for this specific app's content, not as decoration.** This app's core content is real user-submitted photos (the daily entry photo, the profile photo, eventually the friend feed). A dark chrome is a considered, well-precedented choice for photo-forward apps specifically because it lets photos supply the visual richness and color variety, rather than competing with a colorful UI around them — the same reasoning that leads photography-portfolio apps and photo-editing tools to default dark. That is the argument for *this app* going dark-primary, not a generic "dark mode looks modern" justification.
- **`#121212` is chosen on its own terms, not as a stand-in for pure `#000000`.** Equal R/G/B channels, chosen to avoid OLED black-crush and the flash-glare a true-black screen produces during transitions, for the same reason revision 5 avoided pure white behind pure black text. This differs from a naive "invert every hex" pass, which would have landed on `#000000`.
- **The near-black-plus-single-accent-pop cliché is explicitly avoided, on purpose.** The trap this revision has to dodge is not "dark mode" itself, it's dark mode paired with one saturated brand-color accent (electric blue, purple, neon green) used everywhere for emphasis. This palette has no such color: the sole "accent" is Ink, a true neutral near-white with zero hue, used for text, CTA fill, and structural emphasis — never a chromatic color. The only non-neutral colors in the entire system are the two small, muted, narrowly-scoped functional signals (Destructive, Success), unchanged in role from every revision back to revision 4. A palette with genuinely no saturated accent color anywhere is the thing that separates this from the cliché, not the darkness of the background.
- **Surfaces are true neutral steps, not disguised off-blacks.** `#121212`, `#1E1E1E`, and `#262626` all have equal R/G/B channels, unlike the common "off-black" defaults that carry a faint blue or warm tint while still claiming to be neutral. The explicit 1dp border (`#333333`, see Color) on static secondary-surface cards and chips is carried forward unchanged in purpose: surface separation is carried by a real, visible edge plus the tonal elevation system (see Elevation), not by an unsupported lightness step alone.
- **Typography still carries more of the identity than color does.** With zero brand hue anywhere in the palette (unchanged from revision 5/6), Domine's serif presence at Heading/Display sizes remains this app's single strongest visual signature. Neither the type choice, sizes, nor weights changed in this revision — only the lightness axis of the color system inverted.
- **Destructive and Success are retained, unchanged in role, recomputed in value.** Keeping exactly two small, muted, non-neutral colors reserved for functional status signals mirrors the exact thing Instagram's own black-and-white chrome (light or dark) still does. Recomputing their hex values for the new dark backgrounds (see Color) is a contrast-legibility correction, not an expansion of scope — they remain reserved for the same narrow purposes as before.
- **The Brand Mark, Background Texture, and Display promotion are unchanged in composition, only recolored.** Revision 6's three additions (see their sections below) already established that this app's "add some something" answer comes from the product's own concept (dice pips), a calibrated texture, and selective type emphasis — not from a color accent. Recoloring them to the new light Ink keeps that same discipline: nothing about this revision introduces a new decorative element, it only changes which end of the neutral lightness scale the existing elements sit at.

---

## Visual Personality (Empty & Status States)

Not in the base template; added in revision 2 to close the "plain-text-only empty state" gap flagged after user review, redesigned in revision 5 when the secondary accent color was retired, and recolored (not restructured) in this revision for the dark palette. The goal remains a small, restrained amount of custom personality, not decoration for its own sake — and never emoji or stock/AI-slop imagery, per PROJECT.md.

### Icon badge component contract

A small reusable pattern: a circular container (`avatar / icon badge` radius token, see Shape) at 40dp diameter, holding one 20dp `@expo/vector-icons` (Ionicons) glyph, with `card` elevation (see Elevation) so it reads as a small raised chip rather than a flat sticker, plus a 1dp solid border in Ink at 20% opacity. The border remains load-bearing, not decorative, for the same reason as the light palette: without a second accent hue to tint the badge, and with the Dominant/Secondary lightness step being a real but small one, the border is the primary visible signal separating the badge from its host surface, not a reinforcing extra on top of a color step that was already doing most of the work. Do not drop it as redundant. Do not use `StyleSheet.hairlineWidth` for this border — on some Android pixel densities it can round down to a sub-pixel, effectively invisible line; use an explicit `1` (dp) instead. Used exactly twice in this phase:

| Location | Icon | Glyph color | Badge fill | Badge border |
|----------|------|-------------|------------|---------------|
| Profile view screen, empty-bio state | Ionicons `create-outline` (pencil line) | Ink `#EDEDED` | Dominant `#121212`, so the badge stands out from the Secondary-surface (`#1E1E1E`) card it sits on | 1dp solid Ink at 20% opacity |
| Verify-email waiting screen | Ionicons `mail-outline` | Ink `#EDEDED` | Secondary surface `#1E1E1E`, so the badge stands out from the Dominant (`#121212`) background it sits on | 1dp solid Ink at 20% opacity |

Rule stated explicitly for the executor: badge fill is always the *other* neutral surface relative to whatever it's placed on (a Dominant-fill badge on a Secondary-surface card; a Secondary-fill badge on a Dominant background) — never the same surface color as its immediate background, and never a low-opacity tint of Ink (a faint near-white wash over a dark background reads as a pale gray smudge, not a chip, which is why this revision uses solid neutral fills plus a real border instead of a tinted-accent approach, the same rule the light palette followed).

This is deliberately small in scope: two icons across the whole phase, not an icon on every empty field or every screen. Explicitly NOT used for:
- The avatar placeholder, which stays a plain person-silhouette icon directly on the Secondary surface with no badge/border — rendered in Ink at 40% opacity (a soft neutral silhouette, not full-strength ink), since it represents "no photo yet," a neutral absence, not an empty-state prompt inviting action
- The username-availability check icon, which stays inline in its existing Success-color treatment (see Interaction Contracts), no badge
- Any decorative/hero illustration — out of scope; this phase uses only these two functional line icons, nothing purely ornamental

---

## Brand Mark

Not in the base template; added in revision 6 in direct response to user feedback on the revision 5 visual sample ("really plain I think we should add some something"). The app's own concept — RNDMRoll, a randomizer — has a literal, ownable visual vocabulary (dice pips) that a wordmark-only treatment was leaving unused. This is a small, custom, monochrome mark built entirely from the locked Ink color; it does not touch the grayscale palette and is not a generic app-icon-generator glyph or a dice emoji. This revision recolors it for the dark theme; the composition, geometry, and implementation are unchanged.

### Composition
A single geometric mark built from five solid circular pips (dots) arranged in the classic die-face-five quincunx layout (one pip in each corner, one centered) — no enclosing square or die outline. Dropping the outline is deliberate: a full die-face-in-a-square reads as a literal dice emoji or a stock icon-generator glyph, while five bare pips in this specific arrangement is recognizable as "dice" through the dot pattern alone, which is what keeps it reading as a considered mark rather than a stock icon.

### Geometry
Specified on a 40 x 40 viewBox (all values multiples of 4, per the locked Spacing Scale):
- Each pip: 8dp diameter solid circle.
- Corner pip centers: (8,8), (32,8), (8,32), (32,32) — 4dp margin from each pip's outer edge to the viewBox edge.
- Center pip: centered at (20,20).
- Color: Ink `#EDEDED` pips at 100% opacity (see the "Accent/Ink reserved for" bullet list in Color, above), fully transparent background — no fill, no border, no badge treatment. It sits directly on whatever surface hosts it, the same "no decoration for its own sake" discipline that governs the icon badges. This is the one value that changed from revision 6: the pips render in the new light Ink (`#EDEDED`) rather than the old dark Ink (`#262626`), since the mark now sits on a dark background.

This is a fixed, hand-specified layout, not an auto-generated grid, so it stays crisp at the sizes it's used at in this phase.

### Implementation
A custom vector component, not a raster asset and not an emoji character: `components/brand/BrandMark.tsx`, rendering five `<Circle>` elements inside a single `<Svg viewBox="0 0 40 40">` via `react-native-svg`. Add as a new dependency with `npx expo install react-native-svg` (resolves the SDK-57-compatible version automatically; do not hand-pin a version string). Same `checkpoint:human-verify` pattern as the font packages applies before installing: confirm the resolved package is the official `react-native-svg` (maintained under the `software-mansion` org) before it lands in `package.json`. The component takes a `size` prop (default 40, scales the whole viewBox proportionally) and a `color` prop (default Ink `#EDEDED`), so the same component can theme-shift later without a new asset — this parameterization was originally added in revision 6 anticipating a future dark/cinematic-theme phase; that theme is now realized in this phase itself, and the same `color` prop remains useful for Phase 2/3's already-planned dark/cinematic moment screens.

### Placement in Phase 1
Used in exactly one place this phase: the choose-method (entry) screen, centered above the three sign-in method buttons, rendered at `size={64}` (larger than the 40 default, so it reads with real presence rather than icon-badge scale) directly above the wordmark "RNDMRoll" set in Display type (see "Display Role Application" in Typography, above, and the Copywriting Contract entry for the exact string). Together this is the one explicit logo-lockup moment in the app.

Not used anywhere else in Phase 1 — not on the verify-email, name, username, photo, or profile screens. A mark that appeared on every screen would stop reading as a considered brand moment and start reading as a persistent decorative watermark, the same restraint already established for the icon-badge pattern (used exactly twice, not everywhere).

This is consistent with, and sets up, the launch/splash pattern already flagged as a future reference in PROJECT.md's moodboard note ("centered app-mark icon... e.g. dice dots + wordmark") — that splash screen is out of scope for Phase 1, but this component is built so it can be reused there without a redesign.

---

## Background Texture

Not in the base template; added in revision 6 alongside the Brand Mark, for the same user-flagged reason ("really plain... add some something") — a large flat background with nothing above the fold (the choose-method screen, before any card or input breaks it up) was the specific surface that read as sterile. This stays a restrained structural texture, not a decorative pattern, and is deliberately used on exactly one screen. This revision recolors it for the dark theme; the geometry, opacity ladder position, and single-screen scoping are unchanged.

### Token
`texture.dotGrid`:
- Dot diameter: 2dp, solid circle.
- Dot color: Ink `#EDEDED` at 8% opacity (`rgba(237,237,237,0.08)`). This is the new value that changed from revision 6 (`rgba(38,38,38,0.08)`) — the dots switch from the old dark Ink to the new light Ink, since they now sit on the dark Dominant background (`#121212`) rather than the old white one. This remains the lowest rung of Ink's existing opacity ladder used elsewhere in this phase (8% here, versus 15% rest-state borders on the chosen-method outline/focused input, 20% badge borders and inactive step dots, 35% disabled-CTA fill, 40% avatar silhouette, 100% text/links/active states/brand mark) — the ladder's steps are unchanged, only the underlying color they're applied to inverted.
- Grid spacing: 16dp between dot centers, both axes (reuses the `md` spacing token for its rhythm rather than an arbitrary new number).
- Coverage: full-bleed behind all content, lowest z-index layer, static — no parallax, no animation, no scroll-linked movement (an animated background texture would trip PROJECT.md's "no over-the-top scroll animations" constraint).

At 2dp dots, 8% opacity, and a 16dp grid, this is specified to read as a faint but genuine dot-grid field on a real device at normal screen brightness — denser and more opaque than a first-draft pass at 1dp/4%/32dp, which would read as isolated, barely-perceptible specks rather than a field, which is why revision 6 specified these denser, more opaque values instead. It should still disappear in a quick screenshot thumbnail; that dual bar (visible up close on-device, invisible at a glance) is what keeps it reading as "texture," not "pattern." Executor should spot-check both bounds on a real device before shipping, not rely on the specified values alone — this check matters more in this revision, not less, since light dots on a dark background can visually "glow" more than dark dots on a light background at the same nominal opacity, and the on-device check is what catches that if it happens.

### Implementation
Same vector approach as the Brand Mark, not a raster/PNG tile: a `<BackgroundDotGrid>` component in `components/brand/`, rendering a `react-native-svg` `<Pattern>` of 2dp `<Circle>` elements tiled via a `<Rect fill="url(#dotGrid)">` sized to the screen's full dimensions. Renders once per screen mount, not recalculated on every re-render.

### Where it applies
Exactly one screen in Phase 1: the choose-method (entry) screen background only.

Explicitly NOT applied to:
- Any of the onboarding form steps (name, username, photo) — these are dense with text inputs and validation states; a textured background behind form fields competes with legibility rather than adding ambience.
- The verify-email waiting screen — low-density but text-critical (the user must read and act on the instructions here); kept flat for the same legibility reasoning.
- The profile view/edit screens — these already carry visual weight from the avatar, elevation-carrying cards, and (on the view screen) the icon badge; layering a background texture under an already-textured composition would start compounding into decoration for its own sake, exactly what PROJECT.md's standing "no over-the-top" constraint warns against.
- Any card, input, or button surface — the texture is a background-layer-only treatment, never applied on top of or behind interactive/text-bearing components, the same rule the icon-badge border follows for staying structural rather than ornamental.

This keeps the texture rare (one screen out of roughly seven in this phase) by the same discipline already established for the icon-badge pattern (exactly two uses, not applied broadly) — restraint under the "no over-the-top" constraint is the load-bearing design decision here, not the texture itself.

---

## Copywriting Contract

No em dashes used anywhere below, per PROJECT.md's standing "no em dashes" constraint. Unchanged by this revision — the theme inversion does not affect copy.

| Element | Copy |
|---------|------|
| Wordmark - choose-method screen (Display role, revision 6, paired with Brand Mark) | "RNDMRoll". Per PROJECT.md, this is a working title and the final consumer-facing name is still TBD. Bind it to a single string prop on the wordmark component rather than hard-coding it in multiple places, so a future name change is a one-line swap, not a design change. |
| Heading - verify-email waiting screen (Display role, revision 6) | "Check your email" |
| Primary CTA - choose-method screen | Three method buttons, no generic "Continue": "Continue with Email", "Continue with Apple" (iOS only), "Continue with Google" |
| Primary CTA - name step | "Continue to username" |
| Primary CTA - username step | "Continue to photo" |
| Primary CTA - photo step (required weight) | "Finish setup" |
| Secondary CTA - photo step (skippable, D-06) | "Skip for now", rendered as a plain text button (no fill, no border) so it never competes visually with "Finish setup" |
| Primary CTA - profile edit | "Save changes" |
| Empty state (profile VIEW screen, bio absent) - heading | "Add a bio" |
| Empty state (profile VIEW screen, bio absent) - body | "Tell people what you're rolling for." Shown next to the icon-badge treatment described in Visual Personality above; tapping the whole empty-state row navigates to profile edit. |
| Empty state (profile EDIT screen, bio field) | Same copy, "Tell people what you're rolling for.", rendered as the standard `TextInput` `placeholder` prop, matching every other field's placeholder treatment. No icon badge here; the badge belongs to the view screen's empty state only, never inside a live editable input. |
| Error state - duplicate email | "That email's already registered. Log in instead." |
| Error state - username taken (live check or insert-time conflict, see RESEARCH.md Pitfall 4) | "That username's taken. Try one of these:" followed by up to 3 tappable alternates |
| Error state - network failure | "Couldn't connect. Check your connection and try again." |
| Destructive confirmation - log out | "Log out of RNDMRoll? You'll need to sign back in to keep your streak going." Buttons: "Log out" / "Stay logged in" |

---

## Interaction Contracts

Not part of the base template; added because this phase's screens (onboarding sequence + async username/verification checks) carry more interaction state than static tokens capture. Unchanged by this revision — state machines, timing, and gating logic are theme-independent; only the colors referenced within them (destructive/success/Ink) changed, per Color above.

### Username step state machine
- **Idle:** field prefilled with the suggestion returned by `/usernames/suggest`, fully editable, not a blank field (D-05).
- **Checking:** on edit, debounce 400ms, then show a small inline spinner plus "Checking..." next to the field. Block submit (Continue stays visible but tapping it while a check is in flight does nothing) until the check resolves.
- **Available:** success-color check icon plus "Available" label next to the field.
- **Taken:** destructive-color "That username's taken." plus up to 3 tappable alternate suggestions rendered below the field.
- **Insert-time conflict:** if the live check passed but the server-side insert still rejects (race condition, RESEARCH.md Pitfall 4), show the same "Taken" state copy and refreshed alternates after submit, not a generic error toast. The insert response is authoritative over the earlier live check.

### Verify-email screen
- **Waiting state:** shows the entered email address, the `mail-outline` icon badge (Visual Personality above), and "We sent a verification link to {email}. Tap it to continue." plus a "Resend email" text button (not primary-CTA styling).
- **Resend cooldown:** after tapping Resend, disable the button and show a 30-second countdown in its place, then re-enable.
- **Deep-link return:** when verification completes (link tap or poll), show a brief confirmation state (success-color checkmark plus "Verified") for a moment, then auto-advance to the name step. No extra manual "Continue" tap and no silent jump with zero feedback.

### Social sign-in buttons
- **Default:** full-width, secondary-surface-colored buttons (not accent-filled; accent stays reserved for the single per-screen primary path), each with a provider icon plus label, e.g. "Continue with Apple".
- **Loading:** on tap, swap the tapped button's label for an inline spinner and disable all three method buttons until the native SDK round-trip resolves. Success routes onward; failure shows inline error copy below the button group; user-initiated cancellation returns silently to idle with no error shown.
- **Platform gating:** Apple Sign In renders on iOS only. On Android, the choose-method screen shows Email + Google only, omitting the Apple option entirely rather than showing it disabled/greyed-out (RESEARCH.md; matches Open Question #3's "omit, don't disable" precedent for out-of-scope options).

### Form validation timing
- Validate on blur for email/password/name fields. Validate live (400ms debounce) only for username availability, since that needs a network round-trip and D-05 requires real-time feedback.
- Error text sits directly below its field, left-aligned, in the destructive color, replacing any helper text rather than stacking both.
- On submit with unresolved errors, focus jumps to the first invalid field.

### Session-restore and font-load gate on launch
- The root layout calls `SplashScreen.preventAutoHideAsync()` immediately, then in parallel: (a) loads `Domine_600SemiBold`, `WorkSans_400Regular`, and `WorkSans_600SemiBold` via `useFonts` from `expo-font`, and (b) reads SecureStore and calls `/auth/refresh`.
- Hold the native splash screen (render nothing else, no spinner) until BOTH conditions have settled: the font hook has returned `true` OR its error branch has fired, AND the session-refresh call has resolved (success or failure). One combined gate, not two sequential holds, so there is exactly one splash-to-app transition rather than a double flash.
- **Font-load failure branch (must be handled, not left to hang):** if `useFonts` returns an error, treat it the same as "loaded" for gating purposes, do not wait indefinitely for a font that will never arrive, fall back to the platform system font (San Francisco / Roboto) for that app session for all three roles (see Typography platform note for the collapsed-family behavior this produces), and log the error for later investigation.
- Once both conditions are settled, call `SplashScreen.hideAsync()` and route directly to `(app)` or `(auth)`. Do not render either route's UI, and do not render text in a system-font fallback state, before this combined gate clears — this is what prevents both the "wrong screen" flash and an "unstyled/system-font" flash of text in one gate rather than requiring two.

### Photo step (D-06)
- "Finish setup" (accent-filled, primary weight, `raised` elevation) is visually dominant; "Skip for now" (plain text, secondary weight, no elevation) sits below it. Skipping must never read as an error path or a discouraged choice, matching real Instagram behavior.

### Accessibility: icon-only controls
Every icon-only control (no visible text label) in this phase must carry an explicit `accessibilityLabel` so screen-reader users get a text equivalent the sighted layout doesn't otherwise provide. This applies at minimum to:
- The back button (wherever `expo-router`'s back navigation is rendered as a bare chevron/arrow icon): `accessibilityLabel="Go back"`
- The close button (dismissing the log-out confirmation sheet, or any modal/sheet in this phase): `accessibilityLabel="Close"`
- The "Resend email" tap target on the verify-email screen, since it carries a live countdown state: `accessibilityLabel="Resend verification email"` while enabled, and `accessibilityLabel="Resend available in {n} seconds"` (updated each second) while the cooldown is active, so the disabled/counting state is announced, not just visually shown
- The username-availability status icon (the success-color check icon in the Username step state machine above), since its meaning is carried entirely by color and shape: `accessibilityLabel="Username available"` / `accessibilityLabel="Username taken"` matching its current state
- The avatar placeholder / photo-picker control on the photo step, since it is icon-only until a photo is chosen: `accessibilityLabel="Add profile photo"`, updated to `accessibilityLabel="Change profile photo"` once a photo is set

This list is a floor, not a ceiling: any other icon-only element introduced during implementation (with no adjacent visible text) needs the same treatment before it ships.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|--------------|
| shadcn official | none | not applicable, React Native/Expo target uses no shadcn registry |
| third-party | none | not applicable |

Unchanged by this revision.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PENDING
- [ ] Dimension 2 Visuals: PENDING
- [ ] Dimension 3 Color: PENDING
- [ ] Dimension 4 Typography: PENDING
- [ ] Dimension 5 Spacing: PENDING
- [ ] Dimension 6 Registry Safety: PENDING

**Approval:** pending re-review for revision 7 (light-to-dark theme inversion: Dominant/Secondary/Ink recomputed for a dark palette, Destructive/Success recontrasted against the new dark backgrounds, Elevation reworked from shadow-based to tonal elevation, Brand Mark and Background Texture recolored to the new light Ink, Platform lock flipped to `userInterfaceStyle: dark`; strict grayscale discipline and all other sections unchanged from revision 6).

### Focal Points (carried forward from revision 4 checker recommendation, re-confirm on this revision)

Stated explicitly so the executor doesn't have to infer hierarchy from the Elevation/Color sections alone: the primary CTA (`raised` elevation, Ink fill, Dominant-colored label) is the focal point on every onboarding step (choose-method, name, username, photo). On the profile view screen, the avatar + name/username pairing is the focal point, with the empty-bio icon badge as a clearly secondary element beneath it.

**Revision 6 addition, unchanged in substance by revision 7:** on the choose-method screen specifically, the Brand Mark + wordmark lockup and the background texture (see their sections above) sit compositionally above the three method buttons but must not outweigh them as the screen's focal point. This is enforced by the same token choices used throughout the rest of this spec to signal primary vs. secondary weight: the mark and wordmark carry no elevation and no fill/badge container (flat Ink pips and Display text directly on the background), and the texture is a static, lowest-z-index background layer at 8% opacity — versus the CTA group's `raised` elevation (now a light glow, not a shadow, see Elevation) and solid 100%-opacity Ink fill. The identity/atmosphere elements are present and give the screen presence; the CTA group remains the heaviest single element on screen and the thing the user is meant to act on.
