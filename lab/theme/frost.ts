import { color, radius, space } from '@/lib/theme/tokens';

/**
 * Converts one of the locked tokens.ts hex colours into an rgba() string.
 * The lab never writes a raw colour: every translucent value below is derived
 * from a token, so lib/theme/tokens.ts stays the single source of truth.
 */
export function alpha(hex: string, opacity: number): string {
  const value = hex.startsWith('#') ? hex.slice(1) : hex;
  const r = parseInt(value.slice(0, 2), 16);
  const g = parseInt(value.slice(2, 4), 16);
  const b = parseInt(value.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${opacity})`;
}

export type FrostVariant = 'feed' | 'detail';

/**
 * The frosted material: a blurred copy of the photo it sits on, a warm-white
 * tint, and a hairline rim. Nothing else. No gradient, no glow, no drop shadow
 * (a shadow would show through a translucent fill and muddy the panel).
 *
 * `blur` is a Gaussian sigma in dp. react-native-svg's FeGaussianBlur has no
 * edge mode on native, so the panel must sit at least ~2.5 sigma inside the
 * photo edge or the blurred copy fades out at the rim. `inset` respects that:
 * feed 16dp / sigma 6 (2.7x), detail 24dp / sigma 10 (2.4x).
 */
export const frost = {
  feed: {
    blur: 6,
    tint: alpha(color.dominant, 0.72),
    rim: alpha(color.card, 0.6),
    inset: space.md,
    padding: space.md,
    radius: radius.md,
  },
  detail: {
    blur: 10,
    tint: alpha(color.dominant, 0.78),
    rim: alpha(color.card, 0.7),
    inset: space.lg,
    padding: space.md,
    radius: radius.md,
  },
} as const;

/**
 * Ink at reduced opacity for secondary lines on the panel. Muted (#625F5B)
 * drops under 4.5:1 when the photo behind is near black, so hierarchy on the
 * material comes from role and size, with ink at 0.82 as the quietest tone.
 */
export const onFrostSecondary = 0.82;

/**
 * Warm ground that takes over from the previous screen while an entry is open. It
 * fades in to fully opaque: at 0.92 and even 0.97 the list underneath still showed
 * through as a crisp ghost (tile edges behind the metadata, the back link behind the
 * close icon), which reads as a bug. A live blur of the previous screen needs a
 * native blur view, which this build does not have, so the previous screen recedes
 * into a calm warm ground instead.
 */
export const backdropWash = color.dominant;

export const photoMetrics = {
  /** width / height. Same portrait ratio PhotoPanel uses by default. */
  aspect: 4 / 5,
  feedRadius: radius.md,
  gridRadius: radius.sm,
  diaryRadius: radius.md,
  detailRadius: 0,
} as const;

export const grid = {
  columns: 3,
  gutter: space.sm,
  margin: space.lg,
} as const;
