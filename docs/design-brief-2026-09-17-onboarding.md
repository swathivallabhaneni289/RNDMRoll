# Design Brief — 2026-09-17 (Onboarding Redesign)

Verbatim design brief supplied by the user on 2026-09-17, scoped explicitly to the onboarding flow only ("Redesign ONLY the onboarding flow for RNDMRoll"). Kept as source material for UI-SPEC revision 9. Reference material, not a locked contract as-is — reconciled against standing project decisions below.

## Reconciliation notes (resolved 2026-09-17)

- **Theme**: no conflict. Warm ivory / charcoal / negative-space / photography-led direction matches UI-SPEC revision 8 exactly (revision 8 already adopted this palette project-wide on 2026-09-16). No change needed here.
- **Typography**: brief specifies Playfair Display + Inter, again. **Not adopted**, for the same reason as 2026-09-15 and 2026-09-16 — both are the recognizable AI-generated-UI default pairing, already rejected twice. Keep **Domine (headings) + Work Sans (body/UI)**.
- **Pill-shaped buttons**: brief's "single black pill button: Get started" and other CTA framing. **Not adopted** — standing hard constraint. Buttons stay at the existing capped radius (`md` token / 8dp on tappable elements — see UI-SPEC revision 8's Shape section); the brief's "20-24px" softness is expressed on photo/card containers instead, consistent with how revision 8 already resolved this same tension.
- **Roll-mechanic copy accuracy (new issue, this brief)**: Screen 2 ("Every night at 8:00 PM, everyone gets the same category") and Screen 4 ("Tonight everyone rolls the same category. The fun is seeing how differently people interpret it.") describe a single shared/global category for all users. **Not adopted as written** — the product's locked mechanic (ROLL-01, ROLL-03, resolved 2026-09-15/16) is a per-user local 8:00 PM window with a per-user customizable, independently-weighted wheel. Category is not shared globally. The onboarding copy must be rewritten to keep the emotional beat (a nightly ritual everyone does together, comparing what people got) without asserting an identical category for everyone. Direction for the UI researcher: something like "every night at 8:00 PM, your wheel reveals tonight's category" (Screen 2) and reframe Screen 4's thesis from "same prompt, different taste" toward "everyone rolls, everyone shares" (different random categories, still one shared nightly ritual and shared board) — exact copy is the UI researcher's call, not locked here.
- **Flow integration (new screens vs. existing screens)**: the brief's 5 screens don't depict the existing choose-method or verify-email screens. Resolved reading, per the user's explicit framing that this redesign is scoped to "ONLY the onboarding flow" (additive/restructuring, not a replacement of already-decided account-creation mechanics which this brief doesn't address): the 4 new screens (Welcome, The Ritual, Real > Perfect, Same Prompt) run first, ending in a "Continue" CTA that leads into the existing choose-method screen (unchanged) → existing verify-email screen if email chosen (unchanged) → the new consolidated "Create your profile" screen (Screen 5, replacing the prior separate name/username/photo steps) → land in the app. See CONTEXT.md D-05 (revised 2026-09-17).
- **Screen 5 consolidation**: collapses what was three separate onboarding steps (name, username, photo) plus adds a bio field (previously profile-edit-only) into one screen. This is a real flow simplification, not just a visual restyle — treated as the user's explicit direction since it was given as concrete, deliberate layout instruction, but flagged clearly rather than silently applied. D-06 (photo optional/skippable) still holds inside this consolidated screen even though the brief's mockup shows the filled state. The existing username uniqueness-check / "Taken + alternates" state machine (UI-SPEC revision 8, Interaction Contracts) must still work inline within this single screen, not as a separate step — an interaction-design adaptation left to the UI researcher.

## Original brief (verbatim)

Redesign ONLY the onboarding flow for RNDMRoll. Do not make it look like a startup onboarding or a generic iOS form. It should feel like an editorial luxury mobile app with cinematic photography, warm monochrome colors, and lots of negative space.

### Art direction
Warm ivory background (#F7F5F2), never pure white.
Charcoal typography (#111111).
Playfair Display for headlines, Inter for body.
Large full-screen photography.
Asymmetrical composition.
Thin divider lines and subtle paper texture.
No gradients, glassmorphism, colorful icons, or generic illustrations.

### Screen 1 — Welcome
Use a full-bleed photograph of a bedroom overlooking a city at sunset.
Top left (small): one roll. one real moment.
Bottom: # rndmroll
A single black pill button: Get started
This should feel like a magazine cover.

### Screen 2 — The Ritual
Large serif headline: life's better when it's random.
Below it: Every night at 8:00 PM, everyone gets the same category.
Use three scattered authentic polaroid-style photos: TV playing a movie, coffee on a café table, book in someone's hand. Don't use illustrations.
Bottom progress: 01 / 03

### Screen 3 — Real > Perfect
Full-width photo with lots of empty space.
Headline: real photos only.
Body: No posters. No album covers. No stock images. Only moments you actually captured.
Use one authentic phone photo with natural grain.

### Screen 4 — Same Prompt
Split layout with two friends' photos.
Headline: same prompt. different taste.
Body: Tonight everyone rolls the same category. The fun is seeing how differently people interpret it.
Bottom CTA: Continue

### Screen 5 — Create your profile
This should not feel like a boring form. Use the profile photo as the hero.
Layout: Large circular profile image, # Swathi Vallabhaneni, @username field, small editable bio card: collecting quiet moments.
Black Continue button pinned to the bottom.
No giant gray rectangles. Use elegant spacing and minimal input styling.
