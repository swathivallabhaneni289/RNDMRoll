# Sketch Manifest

## Design Direction

A head-to-head comparison, requested explicitly to settle an open disagreement: the app's
locked design system (light editorial, strict monochrome, no gradients, no glass, no pill
buttons, restrained motion — reaffirmed three times across earlier design-brief rounds) versus
a gradient/glassmorphism/glow direction with a Tinder-style swipeable card deck, which the
developer wants to see built and compared before deciding whether to reverse those constraints.
Research (see the transition-motion workflow run earlier this session) flagged real risks in
the alternate direction: it undoes decisions made specifically to avoid a generic "AI-app"
look, and a drag/flip/rotate card mechanic reads gambling-adjacent for an app centered on a
randomized daily "roll." Both directions are built honestly here — not a strawman — so the
comparison is fair.

## Reference Points

- Current direction: RNDMRoll's own `lib/theme/tokens.ts` (color.dominant #F6F5F2, color.ink
  #111111, Domine + Work Sans, radius capped at 8dp for tappable elements)
- Alternate direction: the Instagram glassmorphism redesign concept shown earlier this session,
  plus Cash App / Arc Search style gradient-driven transitions (both flagged by research as
  conflicting with the locked constraints, included here anyway per explicit request)

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | welcome-cover-direction | Restrained editorial vs. gradient/glassmorphism for the Welcome cover screen | null | [onboarding, theme, gradient, glass] |
| 002 | carousel-interaction | Swipe-to-next-screen vs. Tinder-style draggable card deck for the 3-screen carousel | null | [onboarding, interaction, gesture] |
| 003 | post-grid-glass-direction | Does a monochrome (no gradient) glass effect read as premium on the post-onboarding grid/diary view? Speculative — that screen isn't planned yet (Phase 4/5). | null | [post-onboarding, glass, grid, speculative] |
