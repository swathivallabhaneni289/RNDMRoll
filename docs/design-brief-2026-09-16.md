# Design Brief — 2026-09-16 (Editorial Direction)

Verbatim design brief supplied by the user on 2026-09-16, kept as source material for UI-SPEC work across phases. This is reference material, not a locked contract — individual phases still go through their own `/gsd-ui-phase` research + checker loop, and must reconcile this brief against PROJECT.md's standing constraints (see "Reconciliation notes" below) rather than adopt it verbatim.

## Reconciliation notes (resolved 2026-09-16)

- **Theme**: this brief's warm off-white/editorial palette supersedes UI-SPEC revision 7's dark-primary theme. Confirmed by explicit user decision — see PROJECT.md Key Decisions. Phase 1's UI-SPEC is being revised (rev 8) to this direction; later phases adopt it when their own UI-SPEC is created.
- **Typography**: brief specifies Playfair Display + Inter. **Not adopted** — these are the same two fonts already explicitly rejected earlier in this project (see PROJECT.md moodboard note, 2026-09-15) as recognizable AI-generated-UI defaults. Project keeps **Domine (headings) + Work Sans (body/UI)**, which carries the same elegant-serif-plus-clean-sans mood without that baggage.
- **Pill-shaped buttons**: brief asks for pill CTAs twice (splash "get started", category reveal "open camera"). **Not adopted** — pill buttons are a standing hard constraint (PROJECT.md Constraints, and user memory). Buttons stay capped at the project's existing max radius (8dp / md token); large rounded corners (20–24px equivalents) are fine for photo cards and containers, just not for button/pill shapes.
- **Roll mechanic stays per-user, not global-sync**: brief's "everyone reveals together" / synced Daily Sync framing is **not adopted** as literal product behavior. ROLL-01 stays each user's own local 8:00 PM window, not a single synced moment (explicit prior resolution — see PROJECT.md Context). Any "Daily Sync" countdown screen that gets built for Phase 2 must be reframed around the user's own personal window, not a shared global countdown; the friend-avatars-at-bottom idea and "calm, cinematic" mood are still usable.
- **Entry fields**: brief's 4-field entry (photo, title, rating, one thought) is **expanded to 5 conceptual fields** per user decision — photo, title, half-star rating, an optional one-tap reaction, AND a separate optional comment/thought field. See REQUIREMENTS.md ENTRY-01 (updated 2026-09-16).
- **Rating scale**: brief says "rates it 1–5 stars" (whole stars). Project requirement is 1–5 in **half-star** increments (Letterboxd-style, already resolved — see PROJECT.md Key Decisions). Half-star UI must still read cleanly in the new editorial visual language.
- **New/expanded scope not yet decided** — carried forward for the relevant phase's own discussion, not decided here:
  - Splash screen + a 3-screen pre-signup education/marketing onboarding ("life's better when it's random", "real photos only", "same prompt, different taste") — this is additive to, and distinct from, Phase 1 CONTEXT.md's existing D-05 post-signup onboarding sequence (choose method → verify → name → username → photo). Whether the app has both a pre-signup marketing onboarding AND the existing post-signup setup flow is open — resolve during Phase 1's UI-SPEC revision or a follow-up discussion, not assumed here.
  - "Today's Board" (Phase 5 friend feed) as a horizontal-card editorial layout instead of an Instagram-style feed — directionally consistent with FEED-01 through FEED-04, no requirement conflict, just a layout preference to carry into Phase 5's UI-SPEC.
  - "My Diary" stats header (rolls, streak, avg ★, top category) and a grid+timeline toggle — goes beyond DIARY-01/02's current scope (chronological log + category filter). Carry into Phase 4 discussion as a proposed enhancement, not yet a locked requirement.
  - Enriched profile (favorite categories, recent activity, yearly roll map) — explicitly out of scope for Phase 1 per existing CONTEXT.md decision D-07 ("Phase 1 profile shows identity fields only... no stats/streak/diary-count placeholders"). This brief's enriched profile is a later-phase concern, consistent with — not in conflict with — that existing decision.
  - Camera screen mockup ("warm real-life bedroom overlooking a city") is a reference photo shown in the brief, not an asset to source or generate — real user-captured photos remain the only photo source (no AI-generated/stock photography, per standing constraint).

---

## Original brief (verbatim)

The product
RNDMRoll is a social app where everyone gets the same random category during Daily Sync (8:00 PM), captures a real photo, rates it 1–5 stars, and logs one short thought. It's a visual diary mixed with a shared social board.
Tagline:
one roll. one real moment.
Overall aesthetic
Think editorial rather than tech.
Warm off-white backgrounds (#F6F5F2)
Charcoal black (#111111)
Soft greys (#E8E7E3)
Minimal monochrome UI
Large photography is the hero
Elegant serif headlines + clean sans body text
Lots of negative space
Rounded corners (20–24px)
Subtle paper texture and film grain only
No neon gradients, glassmorphism, glowing borders, or colorful UI
The interface should feel like a luxury magazine spread rather than a coded dashboard.
Typography
Headlines: Playfair Display
UI & body: Inter
Use oversized editorial typography (48–72px where appropriate)
Keep everything lowercase except category reveals.
Mood references
Imagine a blend of:
BeReal's authenticity
Letterboxd's logging habit
VSCO's photography
COS campaign layouts
A24 movie posters
Apple product pages

### 1. Splash
Full-screen photograph with minimal branding.
Large: rndmroll
Small: one roll. one real moment.
One black pill button: get started

### 2. Onboarding (3 screens)
Use photography and editorial layouts instead of illustrations.
Screen 1: life's better when it's random. Explain Daily Sync.
Screen 2: Show scattered real photos. real photos only. Explain no posters or stock images.
Screen 3: Show friends' photos pinned like a gallery. same prompt. different taste. CTA: Continue

### 3. Daily Sync
This is the signature page.
Large countdown: 07:58
Text: everyone reveals together
A subtle circular countdown sits behind the timer.
Show tiny friend avatars along the bottom.
The page should feel calm and cinematic.

### 4. Spin Wheel
The wheel is the centerpiece.
Six categories: movie, book, song, meal, place, game
Design the wheel with thin monochrome segments and elegant icons.
Center button: SPIN
Add subtle motion blur around the wheel.
Do not make it look like a casino wheel.

### 5. Category Reveal
Full-screen takeover.
Small: today's roll
Huge serif: MOVIE
Body: capture it. rate it. log it.
Large white pill button: open camera
This page should feel like an A24 title card.

### 6. Camera
Minimal camera UI.
No colorful controls.
Warm real-life bedroom overlooking a city.
Real photography only.

### 7. Log Entry
Four fields only: Photo, Title, Rating, One thought.
Nothing else.
Keep the form elegant and spacious.

### 8. Today's Board
This should not resemble Instagram.
Design it as an editorial social board with beautiful horizontal photo cards.
Each card contains: profile, photo, title, stars, tiny reactions.
Use lots of white space.

### 9. My Diary
A visual archive.
Stats across the top: rolls, streak, avg ★, top category
Then a perfectly aligned photo grid.
Below that, a timeline toggle.

### 10. Profile
Feels personal, not influencer-focused.
Feature: favorite categories, recent activity, yearly roll map, minimal bio
