---
sketch: 002
name: carousel-interaction
question: "Swipe-to-next-screen vs. Tinder-style draggable card deck for the 3-screen carousel?"
winner: null
tags: [onboarding, interaction, gesture]
---

# Sketch 002: Carousel Interaction

## Design Question
Should the 3-screen onboarding carousel (Ritual → Real Photos → Everyone Rolls) keep its
current swipe/chevron-to-advance-one-screen interaction, or become a Tinder-style draggable
card deck (drag left, card rotates and flies off, next one underneath)?

## How to View
open .planning/sketches/002-carousel-interaction/index.html

## Variants
- **A: Swipe to next screen (current)** — each step owns the full screen; swipe left or tap
  the corner chevron advances with a directional push. No card metaphor, no continuous
  drag-tracking (matches the real app's `AdvanceControl`, which only reads the gesture on
  release, not while dragging).
- **B: Tinder-style card deck** — the three steps become cards in a stack; drag the top card,
  it rotates and trails your finger in real time, a "KEEP" stamp fades in as you drag, release
  past the threshold to send it flying off-screen.

## What to Look For
- **This is the one carrying real risk, not just a style preference.** RNDMRoll is an app about
  a randomized daily "roll" — a drag-flip-rotate card mechanic is the same gesture vocabulary as
  a dating app or a loot-box/gacha reveal. Watch whether it reads that way once you're actually
  dragging it, not just looking at a screenshot.
- PROJECT.md already has a standing rule for the wheel-spin screen specifically banning
  "prize-wheel/gambling aesthetics" — this interaction sits right next to that line for a
  different screen. Worth asking whether it's consistent to ban it on one screen and add it on
  another.
- Functionally, B also drops something A has: the chevron is a visible, discoverable affordance
  regardless of whether you know to swipe. B's drag gesture has no visible affordance at all —
  you'd need a first-run hint to teach it.
