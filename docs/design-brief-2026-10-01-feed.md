# Design brief, 2026-10-01: the friend feed as frosted glass (feed only)

Status: **decided by the developer for the Phase 5 friend feed.** Every other screen keeps the light editorial system unless the developer says otherwise. For the feed this replaces the "restrained glass laid over the photo" experiment in `docs/frosted-editorial-experiment.md`, whose overlay was retired on 2026-09-30 because it read as unclear.

## What the developer said (verbatim, voice dictated, 2026-10-01)

> '/Users/swathivallabhaneni/Downloads/IMG_1142.jpg' and uh, do you see this image you see how play uh, the glass theme is looking right now that's how I want my feed to look like whenever a user gets uh, to spin the wheel and gets a topic and they post it uh, this is where uh, the other users will be able to see the whole thing it's basically Instagram how we can see uh, posts when a user uploads them that's exactly how I wanted it to be

## The reference

`IMG_1142.jpg`, a screenshot taken 2026-09-23, 1320 x 1027. A copy is kept at `docs/reference/feed-glass-reference-2026-10-01.jpg`. It is a third-party concept (Instagram drawn as a visionOS-style glass window). Reference only: never ship, trace or reuse any of its assets or branding.

What it shows:

- One large frosted translucent window floating over a softly blurred warm grey room.
- Inside it, separate frosted panels with a thin light edge and large rounded corners: a navigation column on the left, a horizontal row of circular avatars across the top (each with a ring and a name beneath), and a post card below.
- The post card: a header (circular avatar, name, small time, a three-dot menu), a large media area, an action row (heart with a count, comment with a count, share) and a one-line caption.
- Light text on dark translucent glass, thin outline icons.

## What is decided

1. **Friend feed (Phase 5):** an Instagram-style vertical feed. When a friend spins, gets a topic and posts, the other users see the whole entry here: who, when, the rolled topic (category), the photo, the title, the half-star rating and the author's reaction, with the one-tap reaction for viewers.
2. **The glass is the post card, not an overlay on the photo.** Each post sits on a frosted translucent card over a soft blurred background. The photo stays sharp and is the focus.
3. **A horizontal row of friends across the top**, as in the reference.

## Not decided (settle in the Phase 5 UI-SPEC discussion)

- Tint: warm light glass with ink text (matches the rest of the app) or dark smoked glass (matches the reference).
- What the top row means. Recommendation: friends who have posted today, plus your own slot.
- Navigation on a phone: the reference's side column does not fit. PROJECT.md's moodboard note already proposes a bottom bar with five tabs and a circular centre roll button. Whether that bar is glass is open.
- Whether the diary, entry detail or anything else adopts glass. Default: no.
- Live blur or the lab's faked blur (see implementation notes).

## Standing rules that still apply

These are the developer's own rules from PROJECT.md. Where the reference breaks one, the feed follows the rule, not the reference:

- **No chromatic or purple gradients.** The reference's rainbow story rings are out. Use a plain monochrome ring or none. The Welcome cover's monochrome scrim is the one sanctioned gradient.
- **No fake counters.** Figures like "234.6k" are mock data. Show real reaction counts only, and nothing when the count is zero.
- **No third-party branding.** No Instagram or Apple marks, wordmark, verified badge or brand avatars.
- **No emoji as icons.** Vector icons only (Ionicons). Emoji reactions as user content are fine, since FEED-04 is a one-tap emoji reaction.
- **No pill-shaped buttons.** Tappable corners stay capped at `radius.md` (8dp). Large corner radii are allowed only on non-interactive containers (the `radius.lg` 24dp token). Circles only for avatars.
- **Reciprocity stays.** The feed unlocks after you post your own entry, and a late post still unlocks it with a "late" badge.
- **Real user photos only.** No stock or generated imagery.

## Implementation notes for Phase 5

- Real iOS frosted glass is `expo-blur`'s `BlurView` (a native UIVisualEffectView). It is not installed, and adding it needs a dev-client rebuild. Batch it with the other pending native changes: the Google Sign-In config plugin and the image-picker permission strings.
- Android blur is weaker. Decide the fallback (a solid translucent tint) in the UI-SPEC.
- Blur on a scrolling list is expensive: at most one blurred layer per card, and measure on a real device.
- Starting point for layout: `lab/screens/FeedScreen.tsx` and `lab/components/FeedPost.tsx` (header, photo, caption). Comment threads in the lab are a design exploration only and are not in the requirements.
