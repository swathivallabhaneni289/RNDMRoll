---
phase: 1
slug: foundation-accounts
status: approved
reviewed_at: 2026-09-17
shadcn_initialized: false
preset: none
created: 2026-09-15
revised: 2026-10-08
revision: 15
revision_reason: "Revision 10 (2026-10-04) is a scoped change requested by the developer after the dice mechanic gave way to the spin wheel: the five-pip dice Brand Mark is replaced by the developer's wheel logo, and the words roll, rolls and rolling become spin, spins and spinning in the onboarding copy and the bio placeholder (the screen formerly named Everyone Rolls is now Everyone Spins). Nothing else in the visual system changes. Revision 9 is a scoped ADDITION to revision 8, not a visual-system revision: per docs/design-brief-2026-09-17-onboarding.md (a third design brief, scoped explicitly to onboarding only, with a binding reconciliation-notes section already resolving conflicts) and CONTEXT.md's D-05 (revised 2026-09-17), this revision (1) adds 4 new pre-signup marketing screens (Welcome, The Ritual, Real > Perfect, and a renamed fourth screen, see below) ahead of the existing choose-method screen, and (2) consolidates the prior three separate name/username/photo onboarding steps into one 'Create your profile' screen that adds a bio field. Every other locked system from revision 8 — color palette, elevation mechanism, typography families, spacing scale, brand mark composition, background-texture composition, the `md` (8dp) no-pill radius cap, Registry Safety, and the accessibility floor — carries forward UNCHANGED in value; only new usage sites and, in a few explicitly flagged places, new reasoning for an unchanged value, are added. SCREEN COUNT, RECOMPUTED NOT ASSUMED: this revision's own task framing loosely suggested 'eleven' screens; that figure does not check out against arithmetic and is corrected here rather than carried forward uncritically. The actual count, enumerated: (1) Welcome, (2) The Ritual, (3) Real > Perfect, (4) Everyone Spins (renamed, see Copy-Accuracy below) — the 4 new pre-signup marketing screens — plus (5) choose-method, (6) verify-email, (7) Create your profile (replacing the prior name+username+photo three-screen run with one), (8) profile view, (9) profile edit. Nine screens total: 7 (revision 8's count) minus 3 (name/username/photo, removed) plus 1 (Create your profile, added) plus 4 (new marketing screens) = 9. Footnote, not a screen invented here: revision 8's own still-open, non-blocking checker recommendation (no dedicated email/password entry screen exists despite D-01 naming it) remains open; if a future planning pass adds one, the count becomes ten, but that is not decided or built in this revision. COPY-ACCURACY, BINDING PER THE BRIEF'S OWN RECONCILIATION NOTES: the brief's Screen 2 ('Every night at 8:00 PM, everyone gets the same category') and Screen 4 ('same prompt. different taste... Tonight everyone rolls the same category') both assert a single shared/global category, which contradicts the product's locked mechanic (ROLL-01: per-user local 8:00 PM window; ROLL-03: per-user customizable, independently-weighted wheel). Neither is adopted as written. The Ritual's body copy is rewritten to 'Every night at 8:00 PM, your wheel reveals tonight's category.' — keeping the nightly-ritual beat, dropping the shared-category claim. Screen 4 is renamed from 'Same Prompt' to 'Everyone Spins' at the contract level, since 'Same Prompt' as a screen label itself asserts the rejected mechanic; its headline is rewritten to 'everyone spins. everyone shares.' and its body to 'Every night at 8:00 PM, your circle each spins their own wheel. The fun is seeing what everyone got and how they showed up for it.' — same emotional beat (a nightly ritual everyone shares, comparing results), accurate that categories are per-user, not identical. The Ritual's own headline ('life's better when it's random') and Real > Perfect's headline/body ('real photos only' / 'No posters. No album covers. No stock images. Only moments you actually captured.') have no mechanic-accuracy conflict and are kept as the brief wrote them. TYPOGRAPHY AND PILL BUTTONS REJECTED A THIRD TIME: the brief specifies Playfair Display + Inter and pill CTAs again; both are rejected again for the same standing reasons (PROJECT.md moodboard note, 2026-09-15/16/17). Domine + Work Sans and the `md` (8dp) button/input radius cap carry forward with zero exceptions on anything tappable, including the new screens' CTAs. DISPLAY SIZE UNCHANGED, RATIONALE REWRITTEN: Display stays 40/46 (unchanged value), but revision 8's own stated reason for that ceiling — 'that range belongs on a full-bleed splash/hero surface, which this phase does not have in scope' — is now false, since this phase adds four such surfaces, and cannot be carried forward as-is without the spec contradicting itself. The new reason: Body/Label/Heading/Display (16/14/22/40) is a closed four-size type scale, already at this contract's declared 3-4-size ceiling; introducing a fifth, larger 'hero' tier for the four marketing headlines would break that ceiling for a stylistic want the existing Display role already serves. The same Display role now carries two registers instead of one — full-bleed marketing headlines (Welcome's wordmark, The Ritual, Real > Perfect, Everyone Spins) and the one remaining low-density form-screen moment (verify-email's 'Check your email') — distinguished by context and frequency, not by size. DISPLAY ROLE APPLICATION AND RESTRAINT ARGUMENT, RECOUNTED AND REFRAMED: six Display uses now exist (Welcome's wordmark-text-only, The Ritual, Real > Perfect, Everyone Spins, choose-method's wordmark, verify-email's heading), against three Heading uses (Create your profile, profile view, profile edit) — a clean one-headline-tier-per-screen mapping across all nine screens. Revision 8's restraint argument ('two uses out of seven screens keeps Display reading as emphasis') no longer holds arithmetically and is replaced with a register-based argument instead of a frequency-based one: Display marks marketing/low-density screens, Heading marks dense form-carrying screens, and that functional split — not rarity — is what keeps Display from reading as a uniform size bump. STEP-PROGRESS INDICATOR RETIRED AND REPLACED, NOT CARRIED FORWARD UNEXAMINED: revision 8's Accent/Ink reserved-for list included a dot-based step-progress indicator for the post-signup name/username/photo sequence; that sequence no longer exists after consolidation, so the dot indicator has no remaining use site and is retired rather than left as dead spec. In its place, a numeric 'NN / 03' indicator (matching the brief's own '01/03' notation) is added for the 3-screen numbered marketing run (The Ritual, Real > Perfect, Everyone Spins); Welcome sits outside the numbered sequence as a cover/splash screen, consistent with the brief's own progress indicator first appearing on Screen 2, not Screen 1. Real > Perfect and Everyone Spins each also get a progress reading (02/03, 03/03) by extrapolation from Ritual's stated pattern for a consistent step-sequence UI, since the brief's own text only wrote the indicator explicitly under Screen 2 — flagged here as a reasoned inference, not a brief quote. CTA SYSTEM EXTENDED WITH ONE NEW ON-PHOTO VARIANT, USED EXACTLY ONCE: revision 8's rule ('CTA label is always Dominant-colored text on Ink fill') is kept as the default, standard CTA for every screen whose CTA sits on a light Dominant/Secondary surface (Everyone Spins' 'Continue', Create your profile's 'Finish setup', profile edit's 'Save changes', all three choose-method method buttons). Welcome is the one screen whose CTA ('Get started') sits directly on a dark, scrimmed full-bleed photo, where the standard Ink-fill/Dominant-label pairing would blend into the scrim rather than separate from it — so a second, explicitly named 'on-photo CTA' variant is added: fill inverts to Dominant `#F6F5F2` solid, label inverts to Ink `#111111`, same `raised` shadow spec and same `md` radius cap (still no pill). This is the deliberate inverse-pairing logic revision 8 already used once (Ink-fill/Dominant-label as the mirror of revision 7's Dominant-fill/Ink-label), applied here for a legibility reason instead of a whole-theme reason. Both variants stay strictly within the existing Ink/Dominant pair — no third color is introduced for this. SCRIM, SPECIFIED AS A GUARANTEE MECHANISM NOT AN ASSUMPTION: because this UI-SPEC declares layout/color contracts, not actual photo assets (no photo pipeline exists yet in Phase 1), contrast cannot be computed against an unknown future photograph. Rather than assume a worst-case photo luminance, Welcome's scrim is specified so that every text/wordmark/CTA element sits inside a zone of fully OPAQUE (100%) Ink `#111111`, never a partial translucent blend: a top scrim flat-opaque from 0% to 8% of screen height (fading to 0% opacity by 20% height, pure transition, no content placed there), and a bottom scrim flat-opaque from the bottom edge to 28% of screen height measured up from that edge (fading to 0% opacity by 55% height from the bottom edge, again pure transition with no content). Because the flat zones are fully opaque Ink, not a blend, they inherit the exact already-verified 17.33:1 Dominant-on-Ink contrast ratio from revision 8's Color section regardless of whatever real photograph eventually sits beneath them — a guarantee, not an estimate. PHOTO PLACEHOLDER TOKEN, NEW: every photo panel in this revision (Welcome's full-bleed background, The Ritual's three polaroid-style cards, Real > Perfect's single panel, Everyone Spins' two split panels) renders, until a real photo pipeline exists, as a Secondary `#E8E7E3` solid fill with a centered `image-outline` Ionicons glyph at Ink 30% opacity (20% opacity and larger glyph size specifically for Welcome's full-bleed background, given its larger field) — an explicit layout stand-in, not a decision about final photo treatment; real photography replaces the fill directly and the scrim/contrast guarantees above are written to hold regardless of what eventually fills it. SHAPE, EXTENDED NOT ADDED TO: the existing `lg` (24dp) named-exception radius token (introduced in revision 8 for the profile-view card and log-out sheet) is extended to cover the new photo placeholder panels (Ritual's polaroid cards, Real > Perfect's panel, Everyone Spins' split panels) — still never applied to anything tappable, consistent with revision 8's own rule for this token. Real polaroids conventionally have sharp square corners; this revision deliberately does not add a third, single-use sharp-corner exception just to chase that literal detail, keeping the shape system closed at three named radii (sm/md/lg) plus the circular avatar/icon-badge token. BRAND MARK AND BACKGROUND TEXTURE, SCOPING DECIDED EXPLICITLY RATHER THAN LEFT AMBIGUOUS: revision 8 said the Brand Mark (five-pip glyph + wordmark lockup) and Background Texture (dot grid) each apply to 'exactly one screen: choose-method.' Welcome now also carries the word 'rndmroll' per the brief. Rather than leave the executor to guess whether that duplicates the lockup, this revision states the resolution directly: Welcome shows the wordmark TEXT ONLY, in Display type, Dominant-on-scrim per the on-photo CTA's color logic — no five-pip glyph — while the full Mark-plus-wordmark lockup stays exclusive to choose-method, unchanged from revision 6-8. The dot-grid Background Texture is NOT extended to any of the four new photo screens (it would be visually muddy and low-contrast layered over full-bleed or panel photography, and photography itself already supplies the screens' visual texture) — it stays scoped to choose-method only, unchanged. CREATE YOUR PROFILE, THE CONSOLIDATED SCREEN: replaces the prior name, username, and photo steps in the same sequence position (after verify-email, before landing in the app), and adds a bio field, previously profile-edit-only (per the brief's own explicit Screen 5 layout and CONTEXT.md D-05, revised). Layout, top to bottom: a 120dp circular hero avatar (larger than the shared avatar/icon-badge token's default usage elsewhere, still using that same component family) with the existing icon-badge edit affordance overlapping its bottom-right edge (reusing the already-declared icon-badge component, not inventing a new one) as the tap-to-add/change-photo control; a Name field; an @username field auto-suggested from the entered name, preserving revision 8's exact idle/checking/available/taken-with-alternates state machine verbatim in behavior, now rendering inline in a dense single-screen layout via a fixed-height (24dp) status row immediately beneath the field that stays reserved at all times so the bio field below it never jumps between idle/checking/available states — the one explicitly flagged exception is the 'taken' state, where up to 3 tappable alternate-suggestion chips render in a horizontal wrap row beneath the status row and are allowed to push the bio field and CTA down, since accommodating real alternates needs real estate that a fixed-height row cannot reserve in advance; a bio field (new to onboarding), multi-line, using the placeholder prop only, reusing the exact existing copy 'Tell people what you're spinning for.' already locked for the profile-view empty-bio state — never prefilled with the brief's own mockup copy ('collecting quiet moments.'), since a prefilled bio the user did not write would be fabricated content, which PROJECT.md's standing constraints treat as a form of AI-slop copy; and a single primary CTA, 'Finish setup' (reused verbatim from revision 8's photo-step terminal CTA string, since this screen is now that same terminal step), pinned outside the scrollable form content at the bottom of the screen with standard keyboard-avoidance so it stays reachable while the keyboard is open. D-06 (photo optional/skippable) still holds inside this one screen even though the brief's own mockup shows the filled avatar state: revision 8's dedicated 'Skip for now' secondary CTA is retired (there is no longer a separate step to skip past), replaced by a small 'Optional' caption (Label role, Muted color) positioned beside the avatar, making the skippability legible without a second competing CTA on an already-dense screen; the avatar's empty state reuses the existing plain person-silhouette-at-40%-Ink-opacity treatment from revision 8, just at the larger 120dp size. UNCHANGED, CARRIED FORWARD VERBATIM IN VALUE: the full color palette (Dominant `#F6F5F2`, Secondary `#E8E7E3`, card white `#FFFFFF`, Ink `#111111`, Muted `#625F5B`, Divider `#D3D0C9`, Destructive `#9A3B32`, Success `#416B4C`), the shadow-primary elevation mechanism and its Ink-`#111111`-shadowColor refinement, the spacing scale, the `md` radius cap on every tappable element, the Domine/Work Sans font families and weights, the Brand Mark's composition/geometry (only its placement scoping sentence changed, per above), the Background Texture's composition/opacity-ladder position (its scoping sentence likewise only clarified, not changed in value), the font-loading/session-restore launch gate, Registry Safety (still not applicable to this React Native/Expo target; the new swipe/paging behavior on the 3 numbered marketing screens is standard `ScrollView`/paging capability, not a third-party registry component, and its exact library choice is left to planning, not decided here), and the accessibility floor (extended with new entries for the chevron-forward advance controls, the numeric step indicator, and the avatar's larger tap target; not replaced). Checker Sign-Off reset to pending; status reset to draft pending re-verification against all 6 dimensions."
---

> **Revision 11 (2026-10-04) override, requested by the developer ("it's not useful"):** the pre-signup sequence is now ONE page. Welcome is a plain Dominant page: the Label tagline "one spin. / one real moment." top-left, the wheel Brand Mark rolling in from the left with no drawings on it, then, once it is in place, the pointer dropping in, the drawings fading in and the wheel spinning by itself (so a new visitor sees what to do; tap it to spin again; each stop lights one random category label in the background, and the background wheels (which carry dial tick marks) turn, spin up when the main wheel moves, while the dashed arc orbits and the specks drift; Reduce Motion shows everything still), the Display wordmark "RNDMRoll" under it, the wheel's eight sections each carrying a thin line drawing of a category (ink on the light wedges, light on the dark ones: movie, book, song, meal, place, camera, with two repeated as a sample) so that every roll-in or tap ends with one drawing under the pointer and its label lit in the background, and the standard Ink "Get started" button, which goes straight to choose-method. Welcome shows on EVERY signed-out launch (the old rule that skipped it after the first time is gone, which also removes the flash of Welcome before the sign-in page). **Removed:** The Ritual, Real > Perfect and Everyone Spins screens, the numeric step indicator, the swipe and chevron paging (AdvanceControl), the on-photo button variant's use on Welcome, and the full-bleed photo cover with scrim. Wherever this document still describes those, treat it as history. **Changed:** the Brand Mark plus wordmark lockup now appears on Welcome (spinning) as well as on choose-method (still); both Welcome and choose-method now carry a faint wheel illustration background (two oversized ghost wheels bleeding off opposite corners, a dashed arc, scattered specks, and the six categories (MOVIE, MEAL, BOOK, SONG, PLACE, CAMERA) as tiny tracked labels, all ink at very low opacity, kept clear of the content and buttons) instead of the dot-grid texture; the dot-grid component stays in the code but is unused. **Unchanged:** choose-method and everything after it.

> **Revision 12 (2026-10-05) override, requested by the developer ("it looks too AI made"):** the profile VIEW screen is redesigned; every string is unchanged. **Layout, top to bottom, all left-aligned on the `lg` side margins:** a small tracked kicker `Profile` (Work Sans SemiBold at the Label size, letter spacing 2.4, Muted, uppercase by style only); the dial-ring avatar (a 104dp disc inside a thin ring carrying sixty dial ticks, ink at low opacity, static; a photo fills the disc, and without a photo the disc shows the person's initials, the first letter of the first and last word, in the Display serif with font scaling off); the name in the Display role (up to three lines, shrinking to no less than 60 percent to fit); the username in Body Muted; then the bio between two hairlines (Divider color, hairline width), either the bio text in Body or, when empty, the `Add a bio` Heading row with the hint line and a trailing arrow, which still opens the editor; then, pinned to the bottom, two full-width action rows between hairlines (`Edit profile` with a trailing arrow, `Log out` in Destructive), each at least 56dp high. The log-out sheet keeps its text and behavior and now uses the same row style for `Log out` and `Stay logged in`. **Removed from this screen:** the white card with its shadow, the person-glyph avatar, the circular icon badge on the empty-bio row, and the centered text buttons. **Radius allow-lists (for the 01-15 Task 1 scan):** `radius.full` is now used in `components/brand/DialAvatar.tsx` instead of `app/(app)/profile/index.tsx`; `radius.lg` stays in `app/(app)/profile/index.tsx` for the log-out sheet only. The avatar placeholder on Create your profile and Edit profile is unchanged for now. **Unchanged:** the identity-only scope (D-07), the tokens, and every other screen.

> **Revision 13 (2026-10-07) override, plan 01-19 (the developer chose one-page sign-up on 2026-10-06):** sign-up, finishing a profile and editing a profile are now ONE page, "Make it yours.", shown in three modes. It **supersedes** revision 12's Profile view (the kicker, dial-ring avatar, name display and action rows screen), the **Create your profile** screen (revision 9) and the **Check your email** screen (verify-email). Those three screens no longer exist; where this document still describes them, treat it as history. Revisions 9 and 12 text about the numbered screens, the nine-screen count and the verified-email gate is likewise history.
>
> **The page.** Routes: `app/(auth)/make-it-yours.tsx` (signup and finish) and `app/(app)/profile/edit.tsx` (edit), both thin wrappers around one component, `components/profile/ProfileForm.tsx`, which takes `mode`. The mode is chosen once on mount and never recomputed, so the form does not change shape when the session changes while the screen swaps. The profile route is the landing page after launch for a complete account. Field order is the same in every mode: photo, name, username, bio, with the extra fields of each mode above or below as listed here. The username check, 400 ms debounce, fixed-height status row, taken-with-three-alternates state, photo picking and the Log out sheet are carried over from the old edit and Create your profile screens unchanged. The avatar uses `DialAvatar` (the only place `radius.full` is used for it); the Log out sheet is the one `radius.lg` use on this page; nothing tappable gets either token.
>
> **Mode 1, signup (signed out, from choose-method "Continue with Email"):** Email, Password (with Show / Hide, 8 or more characters, 72 at most), Birthday (three number fields: Month, Day, Year, placeholders MM, DD, YYYY, with the hint "Used only to check your age. Never shown to anyone."), photo (optional), Name, Username, Bio (optional), then one button, "Continue" with an arrow, and a "Log in instead" link. Continue stays greyed (the existing disabled CTA treatment) until every required field is valid, the username is neither checking nor taken, and no photo step is running. Birthday rules: a real date, year 1900 or later, not in the future, age 13 or more; the server decides again. Under 13 shows "You must be at least 13 to use RNDMRoll." inline from the form and, from the server, the refusal Alert ("RNDMRoll is for ages 13 and up." / "We can't make an account for you. We didn't keep your birthday."). No code, no email check, no link: Continue signs the person in and they land on the profile page. A chosen photo is uploaded after the account exists; if that step fails the person is still in, with an Alert ("Couldn't upload your photo. You can add one later from your profile.").
>
> **Mode 2, finish (signed in, `onboarding_complete` false, an Apple or Google account, open decision (a) chosen):** Birthday, photo, Name, Username, Bio, "Continue" with an arrow, plus Log out and its confirmation sheet (a wrong Google or Apple pick is not a trap). No email, no password. The birthday is sent alone first; under 13 shows the refusal Alert, signs out and returns to Welcome, and no photo call is made.
>
> **Mode 3, edit (signed in, complete):** photo, Name, Username, Bio, "Save changes", Log out. No birthday field, no streak, no counts (D-07 identity-only scope still holds). The Log out sheet keeps its text ("Log out of RNDMRoll? You'll need to sign back in." with "Log out" and "Stay logged in") and returns to Welcome.
>
> **Copy added:** Birthday, Month, Day, Year, MM, DD, YYYY, Show, Hide; the birthday hint; birthday errors ("Enter your birthday as month, day and year." / "Enter a four-digit year." / "That date doesn't exist. Check the day and month." / "That date is in the future."); "Enter your name." (replaces "Name is required."); "That password is too long. Use 72 characters or fewer."; "That email or password isn't right."; "Too many tries. Wait a minute and try again."; "Use a JPG or PNG under 5 MB."; "Already have an account? Log in" (choose-method link). All are pending the developer's keep-or-change answer in plan 01-19 Task 4. No legal text is written anywhere.
>
> **Login.** A separate small route, `app/(auth)/login.tsx`: a Back control at the top left (the chevron and "Back", the same as on the sign-up page), Email, Password, "Log in", "Sign up instead". Back and "Sign up instead" both return to choose-method, so a person who is not sure how they signed up can pick Email, Apple or Google (added 2026-10-07 at the developer's request; "Sign up instead" used to jump straight to the email form). An account that never verified its email can log in.
>
> **Unchanged:** the tokens, fonts, the `md` radius cap on every tappable element, the Welcome page and choose-method (which gains the "Log in" link).

> **Revision 14 (2026-10-08) override, plan 01-20 (the developer asked for the photo and bio on a different page after signing up):** sign-up is now TWO pages and the profile form has a fourth mode. It **supersedes** revision 13's Mode 1 and Mode 2 field lists (photo and bio leave them) and adds Mode 4. Everything else in revision 13 stands.
>
> **Page 1, "Create your account." (Mode 1 signup, and Mode 2 finish for an unfinished Apple or Google account):** the heading is in the Heading role (22/28) with no sub-line. Signup: Email, Password (Show / Hide, 8 or more characters, 72 at most), Birthday (Month, Day, Year, with the hint), Name, Username (live check, up to three alternates), "Continue" with an arrow, and the "Log in instead" link. Finish: Birthday, Name, Username, "Continue" with an arrow, Log out. No photo and no bio on either. Continue stays greyed until every field is valid. This page's Continue makes the account, in one request, as before.
>
> **Page 2, "Make it yours." (Mode 4, extras):** shown once, in the landing route's place, right after a sign-up or a finish; never reachable afterwards and never after a relaunch. It is the edit page without Name and Username: no Back (the account exists), the "RNDMRoll" label and rule, the Display heading "Make it yours." with its sub-line, the camera circle with "Add photo" (or "Change photo"), the Bio box (soft variant, count inside, hint "Optional"), "Continue" with an arrow, and under it the "Skip for now" text link (TextButton, Muted). Both fields are optional: Continue with nothing filled in simply moves on. A chosen photo uploads first; if that fails (too big, wrong type, no connection) the person stays on the page with the message under the circle and nothing is saved, then retries or taps "Skip for now". Otherwise the bio and photo are saved in one save and the page is replaced by the landing page (Mode 3, edit). There is no Log out on this page.
>
> **Heading roles:** the two account pages (Log in, Create your account.) use the Heading role; "Make it yours." keeps the Display role the developer's mockup gave it, on page 2 and on the landing page. "Create your account." at Display size would wrap to two lines.
>
> **Copy added:** "Create your account." and "Skip for now" (both pending the developer's keep-or-change answer in plan 01-19 Task 4). **Copy removed:** revision 13's photo-failed Alert ("Couldn't upload your photo. You can add one later from your profile."), replaced by the message under the circle.

> **Revision 15 (2026-10-08) override, plan 01-21 (the developer asked for less typing: a picked birthday, and a green "looks good" note):** two changes to page 1 (Modes 1 and 2, which share one form). Nothing else in revision 14 changes.
>
> **Birthday as a date scroller (iOS):** the three typed boxes are replaced by one soft box under the BIRTHDAY label (the look of the other soft boxes: 8dp radius, fieldSoft fill, 1dp inkRest border, 50dp high) that reads "Select your birthday" in Muted until a date is chosen, then the date as "April 15, 2000" in Ink, with a small down chevron on the right. Tapping it opens a bottom sheet in the shape of the log-out sheet (card elevation, 24dp top corners, not tappable itself) with the small caps label "Birthday", iOS's own scrolling date picker (month, day and year columns; the system draws it, so it uses the system font and Light colors) and a "Done" button (the standard Ink primary button) that stays greyed until the picker has been moved. Done stores the date and closes the sheet; the dark area above the sheet closes it without changing anything. The picker cannot go past today, starts at January 1 of 25 years ago (a guess, never accepted without a scroll, because the birthday cannot be changed after sign-up) and cannot produce a date that does not exist. The 13+ rule is unchanged: a date that is too young shows "You must be at least 13 to use RNDMRoll." under the box, and the finish page still leaves that rule to the server. Off iOS, or if the native view is missing, the three typed boxes of revision 13 remain with their messages; the typed-only messages ("Enter your birthday as month, day and year.", "Enter a four-digit year.", "That date doesn't exist. Check the day and month.") and "That date is in the future." are reachable only there.
>
> **Green notes ("Looks good."):** a green check and the words "Looks good." (the look of the username's "Available": Ionicons checkmark-circle in the success color, Label size) show under a field whose value is good, on the sign-up and finish pages only. Email: as you type, once it looks like an email address. Password: as you type, once it has 8 or more characters (it replaces the grey "At least 8 characters." line). Name: after you leave the box, or at once when it came filled in. Birthday: once a valid date is chosen, judged with the 13+ rule even on the finish page (so a too-young date there shows neither a note nor an error, and the server's refusal follows). The username keeps its own "Available". An error always replaces the note. This amends the form validation timing rule: errors still appear when you leave a box, but the green note is live for email and password. The edit page and the photo-and-bio page have no notes.
>
> **Copy added:** "Looks good.", "Done", "Select your birthday" (and the accessibility labels "Close", "Birthday, not set", "Opens the date picker"). **Radius allow-list:** the 24dp token (`radius.lg`) is now used in `components/profile/BirthdayPickerField.tsx` (the date sheet only) as well as in `components/ui/PhotoPanel.tsx` and `components/profile/ProfileForm.tsx` (the log-out sheet only). **Not added (the developer's choice, 2026-10-08):** the keyboard Next key, hints inside the empty boxes, tap-to-pick username choices, Apple and Google above Email, email-domain buttons, Face ID.

# Phase 1 - UI Design Contract

> Visual and interaction contract for frontend phases. Generated by gsd-ui-researcher, verified by gsd-ui-checker.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | none. shadcn is a web/Tailwind registry tool and does not apply here: RESEARCH.md locks this phase to React Native / Expo SDK 57 with expo-router (not React/Next.js/Vite), so the shadcn init gate does not fire. |
| Preset | not applicable |
| Component library | none (custom). Plain React Native primitives (View, Text, Pressable, TextInput) styled from a single shared tokens module (e.g. `lib/theme/tokens.ts`), not a third-party RN UI kit. Unchanged by this revision. |
| Icon library | `@expo/vector-icons` (Ionicons as the default icon set). Unchanged by this revision. New glyphs used this revision: `image-outline` (photo placeholders), `chevron-forward` (marketing-screen advance control) — both already part of the Ionicons set, no new icon dependency. |
| Font | **Domine** (serif, display/heading only) at one weight, `Domine_600SemiBold`, plus **Work Sans** (body/UI) at two weights, `WorkSans_400Regular` and `WorkSans_600SemiBold`. Explicitly KEPT in this revision despite docs/design-brief-2026-09-17-onboarding.md specifying Playfair Display + Inter — the same pairing rejected on 2026-09-15 and 2026-09-16, now rejected a third time for the same reason (recognizable AI-generated-UI-default fonts, see PROJECT.md moodboard note). Font families, weights, and sizes are unchanged from revision 8 — see Typography below for why the Display size ceiling's stated *reasoning* changes even though its value does not. |

**Platform lock:** Unchanged by this revision. Phase 1 stays **light mode only** (`"userInterfaceStyle": "light"` in `app.json`), per revision 8's whole-app theme reversal.

**Onboarding flow (revision 9):** the full Phase 1 onboarding sequence, in order, is: (1) **Welcome** (new) — a full-bleed hero photo cover screen — leads via its "Get started" CTA to (2) **The Ritual** (new), which leads via swipe or its chevron-forward control to (3) **Real > Perfect** (new), which leads the same way to (4) **Everyone Spins** (new, renamed from the source brief's "Same Prompt" — see revision_reason for why), whose "Continue" CTA hands off into the existing, UNCHANGED (5) **choose-method** screen → (6) **verify-email** screen if email was chosen (also UNCHANGED) → (7) **Create your profile** (new, consolidated — replaces revision 8's separate name/username/photo steps in this same sequence position) → lands in the app, where the existing, UNCHANGED (8) **profile view** and (9) **profile edit** screens live. Nine screens total — see revision_reason for the full arithmetic and why this corrects a loosely-stated "eleven" in this revision's own task framing. Screens 5, 6, 8, and 9 are not re-specified below beyond what revision 8 already declared; this document only adds new content for screens 1-4 and 7, and updates the handful of shared sections (Typography's Display role, Color's Accent/CTA rules, Shape's `lg` token, Brand Mark and Background Texture placement, Copywriting Contract, Interaction Contracts, Checker Sign-Off) that the additions touch.

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

Exceptions: `44` as a minimum touch-target hit-area (via padding or `hitSlop`, not a new spacing token) for icon-only controls and tappable rows: back button, close button, the username-availability status icon, the "resend email" tap target, and — new this revision — the chevron-forward advance control on The Ritual and Real > Perfect, and the 120dp avatar/photo-picker tap target on Create your profile (already well above 44dp, stated for completeness). The 40dp icon-badge diameter (Visual Personality), the 120dp hero-avatar diameter (Create your profile), and the 2dp background-texture dot diameter remain exempt, for the same reasons as prior revisions.

Otherwise unchanged by this revision.

---

## Shape (Radius)

Not in the base template; declared here because PROJECT.md's standing "no pill-shaped buttons" constraint needs a concrete home or the executor will default to a large/full radius.

| Token | Value | Usage |
|-------|-------|-------|
| sm | 4 | Small chips/status pills (e.g. "Available" / "Taken" inline badges, and the up-to-3 username alternate-suggestion chips on Create your profile) |
| md | 8 | Buttons, text inputs, and cards by default. For buttons and inputs specifically, this is also the **maximum radius** in this phase, enforcing PROJECT.md's "no pill-shaped buttons" rule (a pill is defined here as radius >= half the element's height) — unchanged, applies with no exceptions, including the new on-photo CTA variant (see Color) and the Welcome/Everyone Spins CTAs. |
| lg | 24 | A named exception to the `md` card default, reserved for large, non-interactive content containers. Revision 8 scope: the profile-view container card, and the log-out confirmation sheet. **Extended in this revision** to cover the new photo placeholder panels: The Ritual's three polaroid-style cards, Real > Perfect's single full-width panel, and Everyone Spins' two split panels (see Photo Treatment below). Still never applied to anything tappable — a violation of that rule is a `md`-cap violation, not a legitimate use of this token, unchanged reasoning from revision 8. |
| avatar / icon badge | circular (50% of width/height) | Profile photo, avatar-placeholder, and the icon badges (empty-bio icon, verify-email icon, and the new Create-your-profile edit-avatar badge, which reuses this exact component at its existing 40dp size — see Visual Personality). The 120dp hero avatar on Create your profile uses this same circular geometry, just at a larger diameter. |

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

All five rows are unchanged in value from revision 8. **What changes in this revision is the reasoning behind the Display ceiling, not the ceiling itself:** revision 8 justified stopping at 40 (short of the source brief's literal 48-72px request) because "that range belongs on a full-bleed splash/hero surface, which this phase does not have in scope." This phase now has four such surfaces, so that specific justification no longer holds and cannot be carried forward as-is. The reason Display still stays at 40/46 rather than growing: Body/Label/Heading/Display (16/14/22/40) is a closed four-size type scale, already at this design contract's own declared 3-4-size ceiling (see the design-contract questions this UI-SPEC answers) — adding a fifth, larger "hero" tier purely to satisfy the marketing screens' stylistic want would break that ceiling for something the existing Display role already serves. Display now carries two registers instead of one: full-bleed marketing headlines (see Display Role Application below) and the one remaining low-density form-screen moment (verify-email), distinguished by context and frequency, not size.

Platform note (unchanged by this revision): set `fontFamily` directly per role in the tokens module, and do NOT also set a numeric `fontWeight` alongside a custom-loaded `fontFamily` — on Android, React Native's font matcher does not reliably combine a custom family with a separate `fontWeight` override and can silently fall back to that family's default weight. If a font fails to load (see "Session-restore and font-load gate" in Interaction Contracts), fall back to the platform system font at the equivalent numeric weight (400/600), applied only in that fallback branch.

Default text color is Ink `#111111` rendered on `#F6F5F2`/`#E8E7E3`/`#FFFFFF` light surfaces, unchanged. On the one on-photo screen (Welcome), text sits on the Ink scrim instead and is Dominant-colored — see Color below.

### Display Role Application (recounted this revision)

Six Display uses now exist across the nine-screen sequence, against three Heading uses — a clean one-headline-tier-per-screen mapping:

**Display:**
- **Welcome:** the wordmark "rndmroll" text (see Copywriting Contract), Dominant-colored on the Ink scrim, TEXT ONLY — no accompanying Brand Mark glyph, see Brand Mark below for why.
- **The Ritual:** headline "life's better when it's random."
- **Real > Perfect:** headline "real photos only."
- **Everyone Spins:** headline "everyone spins. everyone shares."
- **Choose-method screen wordmark:** "RNDMRoll" (unchanged from revision 8), paired with the full Brand Mark glyph — this phase's one full mark-plus-wordmark logo lockup.
- **Verify-email waiting screen heading:** "Check your email" (unchanged from revision 8).

**Heading (unchanged tier, new/renamed use sites):**
- **Create your profile** screen heading (new; replaces the three separate name/username/photo step headings this screen consolidates).
- **Profile view** and **profile edit** screen headings (unchanged from revision 8).

**Restraint argument, reframed:** revision 8 argued that Display's rarity (two uses out of seven screens) kept it reading as emphasis rather than a uniform size bump. That argument no longer holds arithmetically now that Display outnumbers Heading. The argument this revision uses instead: Display marks the marketing/low-density register (a cover screen and three pitch screens, plus the one prior low-density form moment), Heading marks the dense form-carrying register (three screens with real interactive density: fields, avatars, validation states) — a functional split, not a frequency-based one, and the split itself is what keeps Display from reading as an undifferentiated size bump rather than the raw count.

---

## Elevation (Shadow / Tonal Elevation System)

Not in the base template; added in revision 2, reverted from tonal to shadow-primary in revision 8. Unchanged by this revision except for two new usage-site additions noted inline below (no new tokens, no changed values).

| Token | Fill | iOS shadow (primary depth cue) | Android `elevation` | Usage |
|-------|------|-------------------------------|----------------------|-------|
| subtle | Secondary surface `#E8E7E3`, no shadow — intentionally flat/recessed | none | `0` | Secondary-surface panels at rest: profile field rows, the alternate-username suggestion chips (now rendering inline on Create your profile, see Interaction Contracts). |
| card | `#FFFFFF` fill, brighter than Dominant/Secondary | `shadowColor:#111111` / `{width:0, height:2}` / opacity `0.08` / radius `6` | `3` | The profile-view container card, modal/sheet surfaces (log-out confirmation sheet). Note: the new photo placeholder panels (Ritual/Real > Perfect/Everyone Spins) reuse this tier's shadow spec and `lg` radius, but NOT its white fill — see Photo Treatment below for why they use Secondary fill instead. |
| raised | Ink fill `#111111` solid (standard CTA) — see Color for the new on-photo variant's inverted fill | `shadowColor:#111111` / `{width:0, height:4}` / opacity `0.18` / radius `10` — the heaviest shadow of the three tiers | `6` | The primary CTA button, default/enabled state, on every screen that has one: choose-method's three method buttons, Everyone Spins' "Continue," Create your profile's "Finish setup," profile edit's "Save changes," and Welcome's "Get started" (on-photo variant — same shadow spec; on the dark scrim this shadow is a secondary, barely-visible refinement, since the button's own light-fill-on-dark-scrim contrast already does the primary separation work — unlike on light-surface screens, where the shadow is the main depth cue). |

Platform note: unchanged from revision 8.

Disabled/loading CTA states use `subtle` instead of `raised`, unchanged.

Negative scope, unchanged: no elevation/tonal-shift on avatars, text inputs, or plain-text buttons/links. No elevation/tonal-shift on the Brand Mark or the Background Texture layer.

---

## Color

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `#F6F5F2` (warm off-white) | Screen backgrounds, default surface behind all form-carrying and standard-background screens |
| Secondary (30%) | `#E8E7E3` (soft warm grey) | Cards, input field fill, section backgrounds, and — new this revision — the photo placeholder panel fill (see Photo Treatment) |
| Elevation tier / "card" tone (utility) | `#FFFFFF` (pure white) | The Elevation system's `card` tier fill only (profile-view card, modal/sheet surfaces) — NOT used for photo placeholders, see Photo Treatment |
| Accent / Ink (10% interactive budget, plus default text) | `#111111` (charcoal) | See "Accent/Ink reserved for" below |
| Muted text (utility) | `#625F5B` (warm mid-grey) | Secondary/helper text, timestamps, and — new this revision — the "Optional" avatar caption and the numeric step-indicator's "/ 03" denominator |
| Divider / Border (utility) | `#D3D0C9` | Hairline dividers, default border on static secondary-surface cards/chips |
| Destructive | `#9A3B32` | Destructive actions and error states only |
| Success | `#416B4C` | Username-available status only |

All eight values are unchanged in hex and in contrast-verification from revision 8; nothing above is recomputed. See revision 8's own record for the full contrast math (17.33:1 Ink-on-Dominant, 15.26:1 Ink-on-Secondary, 18.89:1 Ink-on-white, 5.83:1/5.13:1 Muted, etc.) — that math is reused directly, including for the new scrim treatment below, rather than re-derived.

**Accent/Ink reserved for** (explicit list; nothing else may use this color at full strength):
- The standard CTA fill (solid Ink `#111111`, label Dominant `#F6F5F2`) on every screen whose CTA sits on a light surface: choose-method's method buttons, Everyone Spins' "Continue," Create your profile's "Finish setup," profile edit's "Save changes."
- **New this revision — the on-photo CTA variant**, used exactly once: Welcome's "Get started" button. Fill inverts to Dominant `#F6F5F2` solid; label inverts to Ink `#111111`. Same `md` radius cap, same `raised` shadow spec (see Elevation). This inversion exists because the standard Ink-fill/Dominant-label pairing would visually merge into Welcome's dark Ink scrim rather than separate from it — the same "invert which neutral is the fill vs. the label" logic revision 7/8 already used once at the whole-theme level, applied here for a single-screen legibility reason instead.
- Selected/active state (a chosen sign-in method's outline, a focused text input's border): Ink at 15% opacity / 1dp width at rest, Ink at 100% opacity / 1.5dp width when active or focused. Applies to Create your profile's Name/username/bio fields the same way it applied to the prior separate steps.
- Text links ("Forgot password?", "Log in instead" / "Sign up instead"): Ink at 100%, underlined, `WorkSans_600SemiBold`.
- **New this revision — the numeric step-progress indicator** (replaces revision 8's dot-based indicator, which has no remaining use site after consolidation — see revision_reason): "NN / 03" on The Ritual, Real > Perfect, and Everyone Spins. Current step number: Ink `#111111` at 100%, `WorkSans_600SemiBold`, Label size (14/20). "/ 03": Muted `#625F5B`, `WorkSans_400Regular`, same size. Welcome carries no progress indicator (cover screen, outside the numbered sequence).
- **New this revision — the chevron-forward advance control** on The Ritual and Real > Perfect (see Interaction Contracts): Ink `#111111` at 100%, 24dp glyph, no fill/badge, 44dp hit-slop.
- The Brand Mark's disc, hub dot and pointer: Ink at 100% opacity; its four wedges and hub ring are Secondary `#E8E7E3`; no badge/border. Used only in the choose-method lockup (see Brand Mark below).

**Default text color:** unchanged — Ink `#111111` on light surfaces. Welcome is the one screen where default text instead sits Dominant-colored on the Ink scrim (see Photo Treatment).

**Muted text**, **Divider/Border**, **CTA fill states** (default/enabled vs. disabled), and **Status colors** (Success/Destructive): all unchanged in value and reasoning from revision 8. Disabled/blocked CTA state (e.g. Create your profile's "Finish setup" while a username check is in flight) uses the same Ink-at-35%-opacity-fill / Dominant-at-70%-opacity-label / `subtle`-elevation treatment as revision 8 declared for the equivalent prior-step CTA.

**Coordination with Phase 2:** unchanged from revision 8 — see that revision's record for the full wheel-color coordination note.

---

## Why This Reads as Intentional, Not Generic

Not in the base template; carried forward from revision 8, extended here with the specific risk this revision's addition introduces.

Revision 8's points (specific mood-precedent matching, the Domine/Work Sans choice made against the brief's fonts a third time now, no pill buttons despite the brief asking twice more, the Brand Mark (a dice-pip mark until revision 10) and dot-grid texture as product-specific elements, the warm-neutral choice argued from photo-forward content, direct-citation-then-independently-verified hex values, typography carrying more identity than color) all still apply unchanged and are not repeated here.

**This revision's specific risk:** a four-screen swipeable photo-and-headline intro is itself a recognizable, common "app onboarding carousel" pattern by 2026 — generic dot-carousel marketing screens are as much a cliché in this direction as the premium-editorial-SaaS-template risk revision 8 named for the theme itself. What keeps this addition from reading as that generic pattern:
- **The copy is specific to this product's actual, real mechanic, not generic feature-bullet marketing copy.** The Ritual and Everyone Spins' rewritten copy (see Copywriting Contract) describes this app's actual per-user wheel and shared nightly ritual, not a templated "here's what our app does" pitch — and the rewrite exists specifically because the source brief's version was mechanically inaccurate, not just stylistically generic.
- **The numeric step indicator, not a dot carousel.** A row of dots is the single most common visual tell of a generic onboarding carousel; this revision's "01 / 03" numeric treatment (itself dictated by the brief, not invented to dodge the cliché, but worth noting) reads differently, and the dot-based indicator this app already had (from revision 8's now-removed post-signup sequence) is retired rather than reused here for that reason.
- **Photo panels use the placeholder/polaroid treatment consistent with this app's own real-photo-only policy** (Real > Perfect's entire content is that policy), not stock or generic vector illustration — PROJECT.md's standing "no AI-slop photos" and "no generic illustrations" constraints apply here the same as everywhere else.
- **No pill CTA, no gradient scrim glow, no new color** — the on-photo CTA variant and the scrim both stay strictly within the existing Ink/Dominant pair; nothing chromatic is introduced for these four screens despite them being the most "hero" surfaces this phase has.

---

## Photo Treatment (Placeholder & Scrim)

Not in the base template; new in this revision. This phase has no photo-asset pipeline (RESEARCH.md's Environment Availability notes S3-compatible storage as unprovisioned), so every photo surface below is declared as a layout/color contract for a placeholder, written so it holds once real photography is wired in.

### Placeholder fill (all photo surfaces)
Secondary `#E8E7E3` solid fill, with a centered `image-outline` (Ionicons) glyph:
- Welcome's full-bleed background: glyph at Ink 20% opacity, 64dp — a larger, more sparse treatment appropriate to the larger field.
- The Ritual's three polaroid cards, Real > Perfect's panel, Everyone Spins' two split panels: glyph at Ink 30% opacity, 32dp.

This is an explicit layout stand-in, not a decision about final photo treatment. Real photography replaces the fill directly; the scrim spec below is written to guarantee its contrast regardless of what image eventually fills the placeholder.

### Welcome's scrim (dual-edge, opaque-zone guarantee)
Because contrast cannot be computed against an unknown future photograph, the scrim is specified so every text/wordmark/CTA element sits inside a fully OPAQUE Ink `#111111` zone, never a partial blend:
- **Top scrim:** Ink `#111111` at 100% opacity from 0% to 8% of screen height, linearly fading to 0% opacity by 20% height. The top-left tagline (see Copywriting Contract) sits inside the flat 0-8% zone only.
- **Bottom scrim:** Ink `#111111` at 100% opacity from the bottom edge up to 28% of screen height (measured from the bottom edge), linearly fading to 0% opacity by 55% height from the bottom edge. The wordmark and "Get started" CTA both sit inside the flat 0-28% zone only.
- The fading portions of both scrims are a pure visual transition, carry no content, and carry no contrast guarantee — only the flat, fully-opaque zones do. Because those zones are 100% opaque Ink, text/icon content placed there inherits the exact 17.33:1 Dominant-on-Ink ratio already verified in Color, regardless of the underlying image.
- No scrim is used on The Ritual, Real > Perfect, or Everyone Spins: their photo panels are framed content elements within a standard Dominant-background layout (not full-bleed), so their headline/body/progress text sits on the ordinary Dominant surface using the standard default Ink text color — no contrast question to resolve. This is the deliberate reading of "full-bleed photography screens" in the objective: only Welcome is a true full-bleed cover (the brief's own "should feel like a magazine cover" framing); the other three use photos as elements in generous negative space, which is both truer to the brief's own per-screen descriptions and avoids three more unnecessary scrim calculations.

### Photo panel geometry
- **The Ritual:** three polaroid-style cards, each a square photo area (Secondary fill, `lg` radius) plus an additional 16dp bottom padding strip inside the same card, simulating a polaroid frame's characteristic wider bottom border. Scattered across the upper two-thirds of the screen with fixed rotation offsets of -6deg, 0deg, and +6deg — fixed values, not random or animated, per PROJECT.md's "no over-the-top scroll animations" constraint.
- **Real > Perfect:** one panel, full width minus `lg` (24dp) side margins, portrait-leaning aspect ratio (e.g. 4:5) to match the brief's "lots of empty space" direction, `lg` radius, reuses `card` elevation's shadow spec.
- **Everyone Spins:** two panels side by side, each roughly half-width minus an `sm` (8dp) gap and `lg` (24dp) side margins, same aspect ratio/radius/shadow treatment as Real > Perfect's panel.

All three non-Welcome screens' panels reuse `card` elevation's shadow spec and the `lg` radius token, but use Secondary fill instead of `card`'s white fill (see Elevation table) — these are photo stand-ins, not literal white "card" UI surfaces, so Secondary (already this system's "recessed neutral" tone) reads more correctly as an empty-image placeholder than pure white would.

---

## Visual Personality (Empty & Status States)

Not in the base template; added in revision 2, redesigned in revision 5, recolored across revisions 7-8. Unchanged by this revision except one new reuse noted below.

### Icon badge component contract

A small reusable pattern: a circular container (`avatar / icon badge` radius token) at 40dp diameter, holding one 20dp `@expo/vector-icons` (Ionicons) glyph, with `card` elevation, plus a 1dp solid border in Ink at 20% opacity. Used in these locations:

| Location | Icon | Glyph color | Badge fill | Badge border |
|----------|------|-------------|------------|---------------|
| Profile view screen, empty-bio state | `create-outline` | Ink `#111111` | Dominant `#F6F5F2` | 1dp solid Ink at 20% opacity |
| Verify-email waiting screen | `mail-outline` | Ink `#111111` | Secondary `#E8E7E3` | 1dp solid Ink at 20% opacity |
| **New this revision** — Create your profile, avatar edit affordance | `create-outline` | Ink `#111111` | Dominant `#F6F5F2` (stands out against the Secondary-tone avatar placeholder it overlaps) | 1dp solid Ink at 20% opacity |

Rule stated explicitly for the executor, unchanged: badge fill is always the *other* neutral surface relative to whatever it's placed on.

Explicitly NOT used for:
- The avatar placeholder (both the default 40dp usage and the new 120dp hero usage on Create your profile), which stays a plain person-silhouette icon directly on the Secondary surface with no badge/border — Ink at 40% opacity.
- The username-availability check icon, which stays inline in its existing Success-color treatment, no badge.
- The photo placeholder panels (see Photo Treatment above), which use `image-outline`, not this badge component.

---

## Brand Mark

Not in the base template; added in revision 6, recolored in revision 8, redrawn in revision 10 (the dice-pip mark was replaced by the wheel logo).

### Composition & Geometry
Revision 10: the developer's wheel logo replaces the five-pip dice mark. A solid Ink `#111111` disc carrying four light wedges (Secondary `#E8E7E3`), a ring-and-dot hub, and a fixed rounded pointer at 12 o'clock with a small gap cut round its tip. The shapes are measured from the developer-supplied PNG and expressed in disc-radius units on a square viewBox (`-1.25 -1.45 2.5 2.5`); no enclosing shape and no background (the gap is a real SVG mask, so the mark sits correctly on the dot-grid texture).

### Implementation
Unchanged: `components/brand/BrandMark.tsx`, `react-native-svg`, same `checkpoint:human-verify` pattern.

### Placement in Phase 1 (updated this revision)
The full Mark-plus-wordmark lockup — the wheel glyph at `size={64}` directly above the Display-styled "RNDMRoll" wordmark — remains exclusive to exactly one place: the **choose-method** screen, centered above the three sign-in method buttons, unchanged from revision 8.

**New this revision:** Welcome also shows the word "rndmroll" (see Copywriting Contract), but as **wordmark text only**, in Display type, Dominant-colored on the Ink scrim — the wheel glyph is NOT repeated there. This is a deliberate typographic-only variant, not a second instance of the logo lockup: the full Mark-plus-wordmark combination stays exclusive to choose-method, preserving the "used in exactly one place" specificity revision 6-8 established for the combined lockup, while still letting Welcome carry the product name per the brief.

---

## Background Texture

Not in the base template; added in revision 6, recolored in revision 8. Geometry and opacity-ladder position unchanged by this revision; only the scoping sentence is confirmed/extended.

### Token
`texture.dotGrid`: 2dp solid Ink `#111111` dots at 8% opacity (`rgba(17,17,17,0.08)`), 16dp grid spacing, full-bleed, lowest z-index, static. Unchanged.

### Implementation
Unchanged: `<BackgroundDotGrid>` in `components/brand/`, `react-native-svg` `<Pattern>`.

### Where it applies (confirmed this revision)
Exactly one screen, unchanged from revision 8: the **choose-method** screen background only. **Explicitly NOT extended** to Welcome, The Ritual, Real > Perfect, or Everyone Spins — a low-opacity dot grid layered over full-bleed or panel photography would read as visual noise/mud rather than texture, and these screens' photography already supplies their own visual texture, making the dot grid redundant even where legibility wouldn't be a concern. Also not applied to Create your profile, verify-email, or the profile view/edit screens, unchanged.

---

## Copywriting Contract

No em dashes used anywhere below, per PROJECT.md's standing "no em dashes" constraint.

| Element | Copy |
|---------|------|
| Tagline - Welcome screen (top-left, small, Label role, Dominant-on-scrim) | "one spin.\none real moment." |
| Wordmark - Welcome screen (bottom, Display role, text-only, Dominant-on-scrim) | "RNDMRoll" (same locked string as the choose-method wordmark, deliberately not the brief's lowercase "rndmroll" styling — one consistent brand string across the app rather than two differently-cased variants for one screen) |
| Primary CTA - Welcome screen (on-photo variant, see Color) | "Get started" |
| Headline - The Ritual (Display) | "life's better when it's random." |
| Body - The Ritual | "Every night at 8:00 PM, your wheel reveals tonight's category." (rewritten from the source brief's "everyone gets the same category" for mechanic accuracy — see revision_reason) |
| Step indicator - The Ritual | "01 / 03" |
| Headline - Real > Perfect (Display) | "real photos only." |
| Body - Real > Perfect | "No posters. No album covers. No stock images. Only moments you actually captured." |
| Step indicator - Real > Perfect | "02 / 03" |
| Headline - Everyone Spins (Display; screen renamed from the source brief's "Same Prompt" — see revision_reason) | "everyone spins. everyone shares." |
| Body - Everyone Spins | "Every night at 8:00 PM, your circle each spins their own wheel. The fun is seeing what everyone got and how they showed up for it." (rewritten from the source brief's "same prompt. different taste" for mechanic accuracy — see revision_reason) |
| Step indicator - Everyone Spins | "03 / 03" |
| Primary CTA - Everyone Spins | "Continue" |
| Wordmark - choose-method screen (Display role, paired with full Brand Mark lockup) | "RNDMRoll". Per PROJECT.md, this is a working title and the final consumer-facing name is still TBD. Unchanged from revision 8. |
| Heading - verify-email waiting screen (Display role) | "Check your email". Unchanged. |
| Primary CTA - choose-method screen | Three method buttons: "Continue with Email", "Continue with Apple" (iOS only), "Continue with Google". Unchanged. |
| Heading - Create your profile (Heading role) | "Create your profile" |
| Avatar caption - Create your profile (Label role, Muted, replaces revision 8's dedicated "Skip for now" secondary CTA — see revision_reason) | "Optional" |
| Bio placeholder - Create your profile (TextInput `placeholder`, never prefilled) | "Tell people what you're spinning for." (reused verbatim from the existing profile empty-bio copy, below, for continuity) |
| Primary CTA - Create your profile | "Finish setup" (reused verbatim from revision 8's retired photo-step terminal CTA string) |
| Primary CTA - profile edit | "Save changes". Unchanged. |
| Empty state (profile VIEW screen, bio absent) - heading | "Add a bio". Unchanged. |
| Empty state (profile VIEW screen, bio absent) - body | "Tell people what you're spinning for." Unchanged. |
| Empty state (profile EDIT screen, bio field) | Same copy, as `TextInput` `placeholder`. Unchanged. |
| Error state - duplicate email | "That email's already registered. Log in instead." Unchanged. |
| Error state - username taken | "That username's taken. Try one of these:" followed by up to 3 tappable alternates. Unchanged in copy; see Interaction Contracts for its new inline rendering on Create your profile. |
| Error state - network failure | "Couldn't connect. Check your connection and try again." Unchanged. |
| Destructive confirmation - log out | "Log out of RNDMRoll? You'll need to sign back in." Buttons: "Log out" / "Stay logged in". Unchanged. |

---

## Interaction Contracts

Not part of the base template.

### Pre-signup marketing sequence (Welcome / The Ritual / Real > Perfect / Everyone Spins) — new this revision
- **Advance mechanism:** The Ritual and Real > Perfect (screens with no explicit CTA per the source brief) advance via horizontal swipe paging (standard React Native paging capability; exact library choice is a planning/implementation decision, not fixed here) AND an explicit `chevron-forward` icon control, bottom-right, Ink `#111111` at 100%, 24dp glyph, 44dp hit-slop, `accessibilityLabel="Next"` — the icon control exists specifically so advancing is not swipe-only (an undiscoverable, non-accessible-by-default gesture).
- **Back navigation:** standard swipe-right or platform back gesture returns to the previous marketing screen; Welcome has no back destination (it is the sequence's earliest point).
- **Welcome's CTA** ("Get started") and **Everyone Spins' CTA** ("Continue") both hand off to the existing, unchanged choose-method screen — this is a straight `push`/`replace` navigation, not a state machine.
- **Step indicator semantics:** the numeric "NN / 03" indicator (see Color) carries an `accessibilityLabel` of the form "Step 1 of 3" (etc.) in addition to its visible text, for screen-reader users. Welcome carries no step indicator (outside the numbered sequence, cover screen).
- **One-time, non-skippable sequence:** because this sequence only ever renders for a user with no valid session (the session-restore gate below routes straight to `(app)` otherwise), no separate "skip the whole intro" affordance is specified — every viewer of this sequence is a genuinely new user seeing it for the first and only time.

### Create your profile screen state machine — new this revision, replaces revision 8's separate "Username step state machine" and "Photo step (D-06)" sections
- **Avatar:** 120dp circular tap target, `accessibilityLabel="Add profile photo"`. Empty state: person-silhouette icon, Ink 40% opacity, on Secondary fill (unchanged pattern, larger size) — labeled "Optional" via the adjacent caption (see Copywriting Contract), not a separate "Skip" button. Tapping opens the photo picker (camera or library, per RESEARCH.md's `expo-image-picker`); backing out of the picker, or never tapping at all, leaves the avatar in its empty state and does not block the primary CTA.
- **Name field:** standard `TextInput`, validated on blur (see Form validation timing below).
- **Username field — state machine, preserved verbatim from revision 8, now rendering inline:**
  - **Idle:** field prefilled with the suggestion returned by `/usernames/suggest`, fully editable, not a blank field. Status row (see below) empty.
  - **Checking:** on edit, debounce 400ms, then show a small inline spinner plus "Checking..." (Muted, Label size) in the status row. Block CTA until the check resolves.
  - **Available:** Success-color check icon plus "Available" label in the status row.
  - **Taken:** destructive-color "That username's taken." in the status row, plus up to 3 tappable alternate-suggestion chips (`sm` radius, Secondary fill, Ink text, `WorkSans_400Regular` Label size) in a horizontal wrap row directly beneath it.
  - **Insert-time conflict:** if the live check passed but the server-side insert still rejects, show the same "Taken" state and refreshed alternates after submit, not a generic error toast.
  - **Layout contract for the dense single-screen rendering:** the status row is a **fixed-height 24dp region** reserved directly beneath the username field at all times, across idle/checking/available, so the bio field beneath it never shifts vertically as state changes. The **taken state is the one explicit exception**: its alternate-chip row is allowed to push the bio field and the pinned CTA's scroll-content boundary downward, since real alternates need real estate a fixed-height row cannot pre-reserve; when the state resolves away from "taken" (user picks an alternate, or edits to something available), the alternate row collapses and layout returns to the fixed baseline.
- **Bio field:** multi-line `TextInput`, ~3 lines tall, `placeholder` prop only ("Tell people what you're spinning for.") — never prefilled with default text a user did not write.
- **Primary CTA:** "Finish setup", pinned outside the screen's `ScrollView` at the bottom (not part of the scrolling content), with standard keyboard-avoidance (`KeyboardAvoidingView` / equivalent) so it stays reachable while the keyboard is open and any field is focused. Disabled/blocked-fill state (Ink 35% opacity fill, Dominant 70% opacity label, `subtle` elevation) while a username check is in flight, same mechanism as every other CTA in this phase.

### Verify-email screen, Social sign-in buttons, Form validation timing, Session-restore and font-load gate on launch
Unchanged from revision 8 — see that revision's record. Form validation timing's existing rule ("validate on blur for email/password/name fields, live 400ms-debounce only for username availability") now also governs the Name/username fields on Create your profile, the same rule applied to a different screen layout, not a new rule.

### Accessibility: icon-only controls
Every icon-only control must carry an explicit `accessibilityLabel`, extended this revision. Full current list: back button, close button, "Resend email" (with live-countdown label updates), the username-availability status icon, the avatar/photo-picker control (now at 120dp on Create your profile, same label requirement), **new this revision:** the chevron-forward advance control ("Next") and the numeric step-progress indicator ("Step N of 3"). This list is a floor, not a ceiling.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|--------------|
| shadcn official | none | not applicable, React Native/Expo target uses no shadcn registry |
| third-party | none | not applicable |

Unchanged by this revision. The new horizontal swipe-paging behavior on the 3-screen numbered marketing run is a standard React Native/Expo capability (e.g. `ScrollView` with `pagingEnabled`, or a paging library selected during planning) and does not introduce a shadcn/third-party registry concern.

---

## Checker Sign-Off

- [x] Dimension 1 Copywriting: PASS
- [x] Dimension 2 Visuals: PASS
- [x] Dimension 3 Color: PASS
- [x] Dimension 4 Typography: PASS
- [x] Dimension 5 Spacing: PASS
- [x] Dimension 6 Registry Safety: PASS

**Approval:** APPROVED by gsd-ui-checker, 2026-09-17, for revision 9 (pre-signup marketing onboarding addition + name/username/photo consolidation). 6/6 dimensions passed.

**Recommendations carried forward from revision 8 (re-stated for the current screen set, not new debt):**
- No dedicated copy/screen exists for email/password entry despite D-01 naming it as a login method alongside Apple/Google. Carried forward unchanged across revisions 7, 8, and 9 — not new debt from this revision, still worth closing during planning. If closed, the phase's screen count becomes 10 (see revision_reason).
- Create your profile's heading copy is now declared directly in this revision ("Create your profile" — see Copywriting Contract), closing the equivalent gap revision 8 flagged for the three screens this one replaces.

### Focal Points (recounted this revision)

- **Welcome:** the full-bleed photo is the dominant visual field; the wordmark + "Get started" CTA (on-photo variant, `raised` elevation) are the screen's secondary but clearly legible focal group, anchored in the bottom opaque scrim zone.
- **The Ritual / Real > Perfect / Everyone Spins:** the photo placeholder panel(s) are the primary visual focal point on each screen; the Display headline is the primary textual focal point; the chevron-forward control and step indicator are deliberately minor, unelevated, small-scale elements that must not compete with either.
- **Choose-method:** unchanged from revision 8 — the CTA group is the focal point; the Brand Mark/wordmark lockup and background texture sit compositionally above it but carry no elevation/fill weight, so they don't outweigh it.
- **Create your profile:** the 120dp avatar is the focal point (per the source brief's own "use the profile photo as the hero" direction), with the form fields and pinned CTA clearly present but secondary in visual weight.
- **Profile view / profile edit:** unchanged from revision 8.
