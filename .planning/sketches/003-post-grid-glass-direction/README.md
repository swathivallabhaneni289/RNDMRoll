---
sketch: 003
name: post-grid-glass-direction
question: "Does a monochrome (no gradient, no color) glass effect read as premium on the post-onboarding grid/diary view?"
winner: null
tags: [post-onboarding, glass, grid, speculative]
---

# Sketch 003: Post-Onboarding Grid — Monochrome Glass

## Design Question
After the purple/pink gradient direction (Sketch 001B) was explicitly rejected, this narrows
the ask: does a *monochrome* glass effect (frosted, blurred, no color anywhere) look premium
applied to a grid of entries, on a screen that doesn't exist yet — the Diary/feed grid a user
would see after onboarding?

**This is speculative.** The grid/diary screen this sketches is Phase 4 (Personal Diary) or
Phase 5 (Friend Feed) territory — neither has been planned yet. Nothing here is close to being
built for real; it's purely to test the idea before it's anywhere near a roadmap phase.

## How to View
open .planning/sketches/003-post-grid-glass-direction/index.html

## Variants
- **A: Flat grid (current, no glass)** — plain white cards on the off-white background, the
  restrained direction with zero depth effects, as a baseline.
- **B: Monochrome glass grid** — same off-white background plus the app's own existing
  dot-grid texture, frosted white glass cards (`backdrop-filter: blur`) that let the texture
  show through, and a sticky header that blurs the grid as it scrolls underneath. No color
  anywhere — this is the constraint the previous gradient sketch violated, kept intact here.

## What to Look For
- Glass effects only read as "glass" when there's something worth blurring behind them — B
  works (if it works) because the dot-grid texture gives the blur something to do. A flat
  color background wouldn't show this off at all.
- This still reverses the no-glassmorphism rule in PROJECT.md, just without also reversing
  the no-gradient/monochrome rule. Worth deciding whether "glass, but monochrome" is an
  acceptable middle ground or still a line you'd rather not cross.
- Compare information density and scannability, not just mood — a diary grid needs to read
  fast at a glance; check whether the glass card's lower contrast (translucent white on
  off-white) makes the category/title text harder to scan than A's solid white card.
