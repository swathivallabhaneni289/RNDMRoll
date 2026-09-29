---
sketch: 001
name: welcome-cover-direction
question: "Restrained editorial vs. gradient/glassmorphism for the Welcome cover screen?"
winner: null
tags: [onboarding, theme, gradient, glass]
---

# Sketch 001: Welcome Cover Direction

## Design Question
Should the Welcome cover screen stay strictly monochrome/editorial (the current locked
direction), or move to a gradient background with glassmorphic panels and glow accents (the
direction requested this session, which reverses PROJECT.md's standing no-gradient/no-glass
constraint)?

## How to View
open .planning/sketches/001-welcome-cover-direction/index.html

## Variants
- **A: Restrained editorial (current)** — faithful recreation of the real app screen: flat
  secondary-color photo placeholder, black-to-transparent scrim (functional, for text
  legibility — not a decorative brand gradient), rectangular button capped at the 8dp radius
  token, strict monochrome, Domine wordmark.
- **B: Gradient / glassmorphism** — purple-to-pink gradient background, blurred glow blobs,
  a frosted-glass card (`backdrop-filter: blur`) holding the wordmark and CTA, pill-shaped
  gradient button, gradient text-fill on the wordmark.

## What to Look For
- Does B actually read as more "interesting," or does it read as a generic app-demo-reel
  template — the same "vibe coded" look the current direction was built specifically to avoid?
- A's placeholder is honestly plain right now because there's no real photo pipeline yet
  (Phase 3) — judge A's ceiling with a real photo behind it, not the placeholder gray box.
- B's gradient/glow only works because it's decorative; it has no functional relationship to
  the content, unlike A's scrim (which exists purely so the text stays legible over a photo).
