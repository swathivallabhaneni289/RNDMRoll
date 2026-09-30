# Frosted Material + Editorial UI Experiment (2026-09-30)

Status: **experiment, undecided.** Nothing here is adopted. The code is isolated in `lab/`, development builds only, and does not touch the roll screen, onboarding, authentication or any product logic.

## Why this exists

`docs/motion-interaction-direction.md` and `docs/design-brief-2026-09-16.md` lock the design system to "no glassmorphism". `.planning/sketches/003-post-grid-glass-direction` asked whether a monochrome glass treatment reads as premium on the post-onboarding screens and left the winner open. The brief below deliberately reopens that line for a controlled test: a restrained frosted material, not generic glassmorphism, on three future screens (Phase 4 diary, Phase 5 friend feed, entry detail). Adopting or rejecting it is a separate decision that belongs in PROJECT.md Key Decisions; until then treat the standing constraints as still in force everywhere outside `lab/`.

## What was built

| Screen | Route | What to look at |
|---|---|---|
| A. Friend feed | `/lab/feed` | Posts laid out like a social post: a header (avatar, name, category and time), the photo with nothing on it, then title, half-star rating and a caption below the photo, then the two latest comments. Whitespace, not cards, separates posts. |
| B. Personal diary | `/lab/diary` | GRID and DIARY views, no glass. Typographic switcher with a sliding underline. Crossfade in 300ms with the scroll position carried across. |
| C. Entry detail | `/lab/entry/[id]` | The photo grows out of the list it was tapped in and stays fixed at the top with nothing on it. A scrolling column below holds the header, caption and the full comment thread, and a composer is pinned to the bottom (posting is local state, not saved). Pull down on the photo or tap close to return. |

`/lab` is a small menu. It also has "Use photos from my library", which swaps the designed placeholders for photos you pick.

## Revision 2026-09-30 (developer feedback)

The developer looked at the lab and found the frosted panel unclear: on the feed it sat on the photo, on the detail it sat over it. What they wanted was a post that reads like a social post: a header, the photo, the caption below the photo, then the comments, with nothing hovering over any photo. So the frosted overlay was retired from the feed and the detail.

- **Feed.** Each post is now a header (avatar, name, category and time), the photo with nothing on it, a caption block on the plain ground (title, half-star rating, then the author's name and reaction as one wrapping paragraph), and the two latest comments. "View all N comments" (when there are more than two) and "Add a comment" (when there are none) open the entry, and so does the photo. No card, border or shadow: whitespace separates posts.
- **Detail.** The frosted panel and the ghost copy that rode on the flying photo are gone. The photo is fixed at the top, full width and at most 46% of the screen height. Under it a column scrolls: header block, caption, then the full comment thread. A composer is pinned to the bottom, and posting adds a "You" comment to local state only (nothing is saved). With the keyboard up the photo shortens so the composer stays in view. Pull-down to dismiss is attached to the photo and the close row only, so the thread scrolls normally.
- **Kept.** The photo flight from the feed, grid and diary, the backdrop, the staged reveal (now header, title, rating, caption, thread and composer, once the photo has landed), reduced motion and Android back. GRID and DIARY are unchanged.
- **Retired, kept for reference.** `lab/components/FrostedPanel.tsx` (no screen imports it), the blur branch of `lab/components/LabPhoto.tsx`, and `lab/theme/frost.ts`. The blur, cost and panel findings under "Decisions and findings", and the whole "Review and verification" section (its counts and fixes describe the frosted-overlay build, before this revision), are historical. The shared transition and backdrop findings still apply.
- **Design rules.** Colours still come from the tokens, and radius is still only `radius.sm` and `radius.md`, with one exception: the avatar is the single circle, and `radius.full` appears only in `lab/components/Avatar.tsx`. The constraints under "Decisions and findings" otherwise carry over.
- **Comments are a design exploration only.** They are fixture data (2 to 4 on each friend's post, 1 to 3 on some of the developer's own). The current requirements do not include comment threads: FEED-04 is a one-tap reaction on a friend's entry, and ENTRY-01's comment is the author's own short thought (the caption here). Adding threads would be a requirements decision first.
- **Open question 1** below has a first answer for the feed and detail: laid over the photo, the material did not read clearly.
- **Verified** on web only (Chromium at an iPhone-sized viewport, plus layout and fixture checks). Not run on a simulator or device. The keyboard behaviour was exercised on web by firing the screen's own keyboard handlers, because web has no soft keyboard.

## How to see it

The routes are not in `app/` until you enable them, so a running Metro session is never disturbed by a half-written file.

```
bash lab/enable.sh      # creates five one-line shims under app/lab/ (the app hot-reloads once)
xcrun simctl openurl booted 'rndmroll:///lab'
bash lab/disable.sh     # removes them
```

Run enable between UAT steps: the onboarding draft is held in memory, so a reload mid-signup loses it. On a simulator, the system asks "Open in RNDMRoll?" for the custom-scheme link; tap Open.

Dev-only URL parameters, for recording without touch automation: `/lab/feed?open=f0`, `/lab/feed?y=1080`, `/lab/diary?mode=diary`, `/lab/diary?switch=diary`, `/lab/diary?open=d01`.

## Decisions and findings

- **Blur technique.** The panel is a blurred copy of the photo it sits on (react-native-svg `FeGaussianBlur`), a warm-white tint from the tokens, and a hairline rim. No new dependency and no native rebuild. React Native's own `filter: blur` is switched off on iOS in this build (`enableSwiftUIBasedFilters` defaults to false), and `expo-blur` is not installed. Verified rendering on web and on the iOS 26.5 simulator.
- **Cost.** The blurred copy is rasterised at a quarter of its size and scaled up. At full size it froze the photo transition for over a second on the simulator, because the filter runs on the main thread that also drives the animation. The detail's panel also mounts only after the photo has landed. Simulator graphics are software rendered, so check on a device before trusting the numbers.
- **Shared transition.** Hand-rolled (measure the tapped photo, animate a container from that rect, and back to wherever the source is on close). Reanimated's built-in shared element transitions sit behind a static native flag that is off.
- **Backdrop.** The brief asks for the previous screen to blur behind the detail. That needs a native backdrop blur, which this build does not have, so the previous screen dissolves into a solid warm ground instead. A wash at 0.92 and 0.97 opacity left crisp ghosts of the list behind the metadata and close icon, which read as a bug. To get a true blur, add `expo-blur` (one native rebuild) and swap it into `lab/screens/EntryDetailScreen.tsx`.
- **Photos.** The standing rule is real, user-captured photographs only. The lab therefore bundles no photography: it ships abstract monochrome scenes drawn from the design tokens (very dark to very bright, so panel legibility can be judged across the range) and lets you pick your own photos.
- **Constraints kept.** Tokens for every colour (translucent values derived with `alpha()`), only `radius.sm` and `radius.md`, Ionicons for stars, no gradients, no em dashes, no emoji, no pill shapes, no shadows on the panel (a shadow shows through a translucent fill).

## Review and verification

- Rendered and driven on web (Chromium, iPhone-sized viewport) and on the iOS 26.5 simulator (screen recordings, frames inspected at up to 30fps). Typecheck, design-constraint scans and 639 layout and fixture checks pass.
- An independent review (three reviewers, then a skeptic per finding) confirmed six issues, all fixed: a double tap could stack two detail screens; the diary's fixed-height text block clipped at larger system text sizes (height now follows the OS font scale); the feed's frosted layer vanished in one frame on open (a copy now rides on the flying photo and dissolves); a photo partly scrolled under the top edge flew from its hidden strip (it now fades up in place); the inactive GRID/DIARY label was 2.6:1 (now 4.8:1); one blank frame when the source photo was hidden before the flying copy had painted.
- **Not verified:** a physical device (the simulator draws with software graphics, so timing there is pessimistic), Android, and the double-tap guard and clipped-photo fallback by actually tapping (verified by reading the code and by the reviewers' trace, not by touch automation). Pull-to-dismiss was exercised on web only.

## Open questions for the developer

1. Does the material read as quiet and physical, or as glassmorphism after all? Judge it on real photographs from your library, not the placeholders.
2. Is a solid warm ground acceptable behind the detail, or do you want the live blur enough to accept `expo-blur`?
3. If adopted: record the resolution in PROJECT.md, amend the "no glassmorphism" line in the motion doc, and carry the tokens in `lab/theme/frost.ts` into the Phase 4/5 UI-SPEC.

## Brief (verbatim, supplied 2026-09-30)

### RNDMRoll: Frosted Material + Editorial UI Direction

I want to explore a restrained frosted-material treatment for RNDMRoll without turning the app into a generic AI-generated glassmorphism design.

Do NOT redesign the entire application.

First inspect the existing RNDMRoll design system and preserve:

* warm off-white background
* charcoal/black typography
* Domine for major headings
* Work Sans for UI and body text
* monochrome visual language
* editorial typography and spacing
* existing navigation structure
* existing interaction patterns

### IMPORTANT: What "glass" means in this project

Glass should feel inspired by Apple's restrained frosted-material interfaces, not by generic web3/startup glassmorphism.

Use:

* translucent warm-white or neutral-grey surfaces
* subtle background blur
* very thin low-contrast borders
* extremely restrained shadows
* moderate corner radii
* layered depth
* transparency that reveals only a small amount of the content underneath

Do NOT use:

* gradients
* purple, blue, pink, or other chromatic accents
* glowing borders
* neon
* excessive blur
* huge rounded cards
* floating glass panels everywhere
* glass buttons
* glass navigation bars everywhere
* excessive transparency
* 3D effects
* decorative blobs
* generic SaaS dashboard patterns

The glass material should feel quiet, physical, and tactile.

### 1. FRIEND FEED / BOARD

This is the primary place where the frosted material should appear.

The friend feed should feel more layered and social than the personal diary.

Use:

* warm off-white background
* personal photos as the visual focus
* subtle translucent information panels
* very light background blur behind panels
* thin borders
* restrained shadows
* charcoal typography

The photo should remain more visually important than the glass.

A feed item could conceptually have:

PHOTO

[translucent information layer]

FRIEND NAME
MOVIE
Interstellar
★★★★½
"One-line reaction"

Do not turn every piece of information into a separate floating glass card.

The glass should feel like one material layer sitting naturally over the photograph.

### 2. PERSONAL DIARY / PHOTO GRID

Do NOT use the same glass treatment here.

The personal archive should feel cleaner and more timeless.

Create two viewing modes:

GRID
DIARY

GRID:

* photographs dominate
* minimal text
* consistent image sizing
* generous spacing
* no card containers around every image

DIARY:

* larger photograph
* title
* category
* half-star rating
* short reaction
* date
* subtle metadata

Create a restrained transition between GRID and DIARY.

The switch should feel like changing the way the user views their own collection, not like opening a new application.

### 3. ENTRY DETAIL

Use the strongest glass treatment here, but keep it restrained.

When a user taps a diary photo:

1. photo expands smoothly
2. surrounding content transitions naturally
3. background becomes subtly blurred
4. information layer appears over or beneath the photograph
5. title enters using a subtle fade + rise
6. rating follows
7. reaction follows
8. secondary metadata appears last

The transition should feel like opening a memory.

Avoid dramatic zooming or cinematic effects.

### 4. VIEW SWITCHER

Create a simple GRID / DIARY switch.

Do not use a large pill-shaped segmented control.

Instead explore:

* typography-based navigation
* thin underline
* subtle sliding indicator
* slight opacity change between active/inactive states

Example:

GRID        DIARY
────

When switching:

* grid items subtly rearrange/fade
* diary entries gently enter
* maintain the same scroll position where technically practical

The transition should take roughly 200–350ms.

### 5. FRIEND ENTRY INTERACTION

When opening a friend's entry:

Use a shared-element-style transition if technically practical.

The selected photograph should visually remain connected between the feed and detail screen.

Avoid:

* cards flying across the screen
* 3D rotations
* excessive scaling
* dramatic page flips

The interaction should feel like the same piece of content becoming more detailed.

### 6. MATERIAL DEPTH

Use depth through layering rather than decoration.

Think in three visual layers:

1. warm editorial background
2. photography
3. subtle frosted information surface

Do not introduce additional decorative layers unless they have a clear interaction purpose.

### 7. MOTION

Use React Native Reanimated.

Motion should be:

* subtle
* tactile
* short
* consistent

Use:

* fade
* slight vertical translation
* subtle scale
* shared transitions
* gentle blur changes
* opacity changes
* restrained spring physics where appropriate

Avoid:

* excessive bounce
* confetti
* particles
* neon flashes
* exaggerated spring animations
* animations on every element

### 8. DESIGN PRINCIPLE

The goal is NOT:

"Make RNDMRoll look futuristic."

The goal is:

"Make RNDMRoll feel like a real consumer product with a distinct visual language."

The static interface should remain editorial and restrained.

The glass material should add depth.

Motion should add personality.

Typography should provide hierarchy.

Photography should provide emotion.

Do not add visual effects simply because they are fashionable.

### 9. IMPLEMENTATION RULE

Before changing anything, inspect the current screens and components.

Do not rewrite working screens unnecessarily.

Implement this as a controlled visual experiment.

First create the following three screens/variations:

A. Friend Feed with restrained frosted material
B. Personal Diary Grid with no glass
C. Entry Detail with subtle frosted material

Show these as a coherent system.

Do not modify the roll screen, onboarding, authentication, or existing product logic during this experiment.

The result should look like one deliberately designed application rather than three unrelated design trends.
