# RNDMRoll Motion & Interaction Direction

> Produced 2026-09-29 via an external AI agent, briefed with the project context document
> generated in this session. Reviewed and partially implemented same day (see status notes per
> section). Kept verbatim below as the reference; do not silently edit it: if a later phase's
> planning contradicts a section, resolve the conflict explicitly and record the resolution
> here, the same way PROJECT.md tracks its own decisions.

**Status of each section:**
- Section 1 (Onboarding) and Section 10 (Implementation primitives): **implemented** 2026-09-29 (see `lib/motion/`, `app/(auth)/*`).
- Sections 2–9: **not implemented.** Every screen they describe (Roll, Log Entry, Diary, Friend Board) is Phase 2–5 territory, none of which is planned or built yet. Keep this document and hand it to whoever plans/builds those phases as design input; it is not yet validated against real screens or real data shapes.

---

Do not redesign RNDMRoll into a generic modern SaaS/mobile UI. The goal is to make it feel like a deliberately designed consumer product with a distinctive editorial personality.

Keep the existing RNDMRoll visual system:

* warm off-white background
* charcoal/black typography
* monochrome photography
* Domine for major headings
* Work Sans for UI/body
* no gradients
* no purple/blue accent colors
* no glassmorphism
* no pill-heavy UI
* no excessive rounded cards
* no generic AI-dashboard patterns

### Overall motion philosophy

Motion should feel editorial, tactile, and intentional rather than flashy.

Use:

* subtle fade + vertical movement
* directional transitions based on navigation
* scale changes of approximately 0.97 → 1.0 for emphasis
* gentle opacity changes
* spring animations only where they communicate physical interaction
* staggered entrances for groups of content
* shared-element-style transitions where appropriate
* subtle press feedback on interactive elements

Avoid:

* excessive bouncing
* spinning everything
* dramatic zooms
* confetti
* particle effects
* neon effects
* parallax everywhere
* animation longer than necessary
* animation on every UI element simultaneously

The interface should feel calm when browsing and become more energetic specifically during the daily roll.

### 1. Onboarding

Do not make onboarding feel like four static marketing slides.

Each screen should have a subtle entrance sequence:

1. Heading fades in while moving upward approximately 12–16px.
2. Supporting text follows 60–100ms later.
3. Image/visual follows another 80–120ms later with a very subtle scale from 0.97 → 1.
4. Primary action enters last.

When moving forward:

* use a directional transition that suggests moving deeper into the product.
* content should leave in the same direction it entered.
* do not use a generic horizontal carousel animation.

When returning:

* reverse the direction.

The onboarding should feel like one continuous story rather than four unrelated screens.

### 2. Roll screen

The roll screen is the most important interaction in the app.

Before spinning:

* keep the interface almost completely still.
* create anticipation through subtle typography and the wheel itself.
* when the user presses SPIN, compress the button slightly.
* begin the wheel movement smoothly rather than instantly.
* use natural deceleration.
* slightly increase the visual prominence of the selected wedge as the wheel approaches its destination.

Do NOT make this resemble a casino or gambling wheel.

No:

* flashing lights
* confetti
* neon
* 3D wheel
* glossy effects
* loud sound-effect-style visuals

The feeling should be:

WAIT → SPIN → ANTICIPATION → REVEAL

rather than:

CLICK → RANDOM ANIMATION → RESULT.

### 3. Roll result

The category reveal should feel like a deliberate moment.

After the wheel stops:

1. briefly settle the wheel.
2. transition the background/foreground subtly.
3. reveal the category name using a large typographic entrance.
4. let the category appear slightly below its final position and rise into place.
5. introduce the category illustration/photo treatment afterward.
6. reveal the "Let's log it" action last.

The category name should be the visual focus.

For example:

Your roll is...

MOVIE

[small visual treatment]

Let's log it →

Do not immediately push the user into another screen.

Give the result approximately 500–800ms of visual breathing room.

### 4. Log Entry screen

Make the logging process feel progressive instead of showing a giant form all at once.

Use a vertical sequence:

PHOTO
↓
TITLE
↓
RATING
↓
REACTION
↓
SAVE

Fields can reveal subtly as the user interacts with the previous step.

For example:

* photo selected → title field becomes active
* title entered → rating becomes visually emphasized
* rating selected → reaction field becomes available
* save → transition into the completed diary entry

Do not hide required information behind unnecessary steps. The interaction should remain fast.

### 5. Rating interaction

Make the half-star rating feel tactile.

When a star is selected:

* use a very small scale animation
* briefly increase opacity/weight
* settle naturally

For example:

☆ ☆ ☆ ☆ ☆
↓
★ ★ ★ ★ ★

Do not use a giant bounce or colorful celebration.

The interaction should feel satisfying because it is precise, not because it is loud.

### 6. Diary

The diary should feel like a physical collection of memories.

When entries appear:

* use a subtle staggered entrance
* photo first
* title second
* metadata third

When opening an entry:

* use a smooth expansion/shared transition from the diary item into the detail view if technically practical.

Avoid cards flying around the screen.

The user's photos should remain visually dominant.

### 7. Friend Board

The friend board should communicate the idea:

"You rolled. They rolled. Now you can see."

When the board becomes available:

* use a subtle reveal rather than a standard page load.
* entries should enter sequentially.
* the user's own entry can appear naturally alongside friends rather than being visually promoted with a giant badge.

When a friend entry is opened:

* use a subtle expansion transition.
* preserve the photo's position where possible.

### 8. Navigation

Do not animate the entire interface every time the user changes tabs.

Use:

* subtle active-state transitions
* small icon/label movement
* gentle opacity changes
* no bouncing navigation icons

The center Roll action can have slightly more visual emphasis because it is the core action.

### 9. Empty states

Do not use generic illustrations or stock graphics.

Create editorial empty states using:

* typography
* monochrome photography
* subtle hand-drawn marks
* whitespace
* short, personality-driven copy

An empty diary should feel intentional, not unfinished.

### 10. Implementation

Use React Native Reanimated for motion.

Create reusable animation primitives rather than implementing unrelated animations screen by screen.

Examples:

* FadeInUp
* StaggeredList
* PressScale
* PageEnter
* PageExit
* RevealText
* WheelSettle
* RatingSelect
* ImageExpand

Keep animation durations and easing consistent throughout the app.

Use motion to establish hierarchy and communicate interaction, not to decorate the interface.

Before implementing anything, inspect the existing screens and preserve their layout, typography, spacing, and visual language. Make the product feel more designed through interaction and transitions rather than adding visual effects.
