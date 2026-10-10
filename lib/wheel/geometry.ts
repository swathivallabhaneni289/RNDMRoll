/**
 * Pure geometry for the wheel (02-UI-SPEC section 3). No React, no app imports, no phone
 * clock and no randomness: the winner always comes from the server, and everything here only
 * draws it. Angles are degrees clockwise from 12 o'clock, and a positive rotation turns the
 * disc clockwise. Colours are named here ('dark', 'light', 'mid', 'ink', 'dominant') and mapped
 * to tokens by the component, so this file holds no hex values.
 */

/** No wedge is drawn narrower than this, so a one-in-ten category stays readable. */
export const MIN_WEDGE_DEGREES = 22;
/** The turn is at least this many full clockwise turns. */
export const MIN_TURNS = 5;

/** Width of the ink rim, in dp. The wedges live inside it. */
export const RIM_WIDTH = 4;
/** The hub is this share of the wheel's diameter. */
export const HUB_FRACTION = 0.19;

/** An across label has its middle at this share of the wheel's radius. */
const ACROSS_RADIUS_FRACTION = 0.7;
/** Estimated width of one capital in the 11dp wheel label with its 1.5 letter spacing. */
const CHAR_WIDTH = 8.3;
/** Room kept clear of the sides of the wedge for an across label. */
const ACROSS_SLACK = 4;
/** An along label runs from this far outside the hub to this far inside the rim. */
const ALONG_HUB_GAP = 12;
const ALONG_RIM_GAP = 10;

export type Tone = 'dark' | 'light' | 'mid';
export type LabelColorName = 'dominant' | 'ink';
export type LabelDirection = 'across' | 'along';
export type LabelPlacement = { direction: LabelDirection; flip: boolean };

/**
 * The angle each wedge is drawn at. Every wedge is the same size: 360 divided by the number of
 * categories (the developer asked for equal wedges on 2026-10-10, after seeing the weighted
 * wheel). The weights still decide how OFTEN each category comes up (the server picks by them,
 * and Edit wheel shows each share), but not how wide it is drawn. Twelve wedges are 30 degrees
 * each, so every count from 2 to 12 stays above MIN_WEDGE_DEGREES. The angles add up to 360.
 */
export function drawnAngles(weights: readonly number[]): number[] {
  const n = weights.length;
  if (n === 0) return [];
  return weights.map(() => 360 / n);
}

/** Where each wedge starts, in the same order as `angles`. The first starts at 0. */
export function wedgeStarts(angles: readonly number[]): number[] {
  const starts: number[] = [];
  let at = 0;
  for (const a of angles) {
    starts.push(at);
    at += a;
  }
  return starts;
}

/** The middle of each wedge. */
export function wedgeCentres(angles: readonly number[]): number[] {
  return wedgeStarts(angles).map((start, i) => start + angles[i] / 2);
}

/** A point at `deg` clockwise from 12 o'clock and distance `r` from the centre, as "x y". (From BrandMark.) */
export function polar(deg: number, r: number): string {
  const a = (deg * Math.PI) / 180;
  return `${(r * Math.sin(a)).toFixed(4)} ${(-r * Math.cos(a)).toFixed(4)}`;
}

/** The same point as numbers, y growing downwards, for placing labels. */
export function polarPoint(deg: number, r: number): { x: number; y: number } {
  const a = (deg * Math.PI) / 180;
  return { x: r * Math.sin(a), y: -r * Math.cos(a) };
}

/**
 * One wedge as an SVG path from the centre. The large-arc flag is 1 when the wedge is wider
 * than half a turn (weights 10 and 1 draw a wedge near 327 degrees), else 0; the sweep flag is
 * 1 so the arc runs clockwise.
 */
export function wedgePath(a0: number, a1: number, r: number): string {
  const large = a1 - a0 > 180 ? 1 : 0;
  const rr = r.toFixed(3);
  return `M 0 0 L ${polar(a0, r)} A ${rr} ${rr} 0 ${large} 1 ${polar(a1, r)} Z`;
}

/**
 * The tone of each wedge, clockwise from the first (02-UI-SPEC section 2). Wedge i is `dark`
 * when i is even and `light` when i is odd, so the first wedge is dark. When the count is odd
 * the last wedge is `mid` instead, so it differs from the wedge before it and from the first
 * wedge it touches at the wrap. Touching wedges therefore never match.
 */
export function toneIndexes(n: number): Tone[] {
  const tones: Tone[] = [];
  for (let i = 0; i < n; i += 1) tones.push(i % 2 === 0 ? 'dark' : 'light');
  if (n >= 3 && n % 2 === 1) tones[n - 1] = 'mid';
  return tones;
}

/** The label colour on a wedge: `dominant` on dark, `ink` on light and on mid. */
export function labelColorFor(tone: Tone): LabelColorName {
  return tone === 'dark' ? 'dominant' : 'ink';
}

/** The rotation that brings the wedge centred at `centre` under the pointer at 12 o'clock. */
export function poseOf(centre: number): number {
  return -centre;
}

/**
 * The rotation the wheel finishes on: at least MIN_TURNS full clockwise turns past `current`,
 * ending exactly on `pose` (always the winner's pose).
 */
export function targetRotation(current: number, pose: number): number {
  return Math.ceil((current + 360 * MIN_TURNS - pose) / 360) * 360 + pose;
}

/** The wedge that is under the pointer when the disc is turned by `rotation`. */
export function wedgeUnderPointer(rotation: number, angles: readonly number[]): number {
  const turned = ((rotation % 360) + 360) % 360;
  const onDisc = (360 - turned) % 360;
  let start = 0;
  for (let i = 0; i < angles.length; i += 1) {
    const end = start + angles[i];
    if (onDisc >= start && onDisc < end) return i;
    start = end;
  }
  return angles.length - 1;
}

function charCount(name: string): number {
  return Array.from(name.trim()).length;
}

/**
 * The room for an across label at 70 percent of the radius: the width of the wedge there,
 * `2 x r x tan(angle / 2)`. A wedge of half a turn or more has no sides that close in, so the
 * room is then set by the chord of the rim alone; that chord also caps a wide wedge, because
 * a word may never reach out of the wheel.
 */
export function acrossRoom(angle: number, radius: number): number {
  const r = ACROSS_RADIUS_FRACTION * radius;
  const rimInner = radius - RIM_WIDTH;
  const half = angle >= 180 ? Infinity : r * Math.tan((angle * Math.PI) / 360);
  const chordHalf = Math.sqrt(Math.max(0, rimInner * rimInner - r * r));
  return 2 * Math.min(half, chordHalf);
}

/** The length of the along line: from 12dp outside the hub to 10dp inside the rim. */
export function alongRoom(radius: number): number {
  const hubRadius = HUB_FRACTION * radius;
  return Math.max(0, radius - RIM_WIDTH - ALONG_RIM_GAP - (hubRadius + ALONG_HUB_GAP));
}

/** Where along the radius an along label is centred. */
export function alongMiddle(radius: number): number {
  const hubRadius = HUB_FRACTION * radius;
  return (hubRadius + ALONG_HUB_GAP + (radius - RIM_WIDTH - ALONG_RIM_GAP)) / 2;
}

/** Where along the radius an across label is centred. */
export function acrossMiddle(radius: number): number {
  return ACROSS_RADIUS_FRACTION * radius;
}

/**
 * Across when the word fits across the wedge, else along its centre line (UI-SPEC 3), then
 * turned a half turn when its baseline would point left of vertical in `finalRotation`, the
 * pose the wheel stops in. The test is plain arithmetic: an estimated width of `n x 8.3` plus
 * 4dp against the room; no text is measured on the phone. One rule for both directions.
 *
 * `centre` and `angle` are the wedge's centre and drawn angle, `radius` is the wheel's outer
 * radius in dp, `finalRotation` the disc rotation it will rest at.
 */
export function labelPlacement(
  centre: number,
  angle: number,
  name: string,
  radius: number,
  finalRotation: number
): LabelPlacement {
  const need = charCount(name) * CHAR_WIDTH + ACROSS_SLACK;
  const direction: LabelDirection = need <= acrossRoom(angle, radius) ? 'across' : 'along';
  // The direction the baseline points in on the disc: across runs clockwise round the centre
  // line (centre + 90), along runs out from the hub (centre).
  const baseline = direction === 'across' ? centre + 90 : centre;
  const onScreen = (((baseline + finalRotation) % 360) + 360) % 360;
  // The horizontal part of a baseline at `onScreen` degrees is sin(onScreen): negative means
  // it points left, so the word would read upside down. Straight up or down is left alone.
  const flip = onScreen > 180 + 1e-9;
  return { direction, flip };
}

/** How many characters fit on the label's line. */
export function labelCapacity(direction: LabelDirection, angle: number, radius: number): number {
  const room = direction === 'across' ? acrossRoom(angle, radius) - ACROSS_SLACK : alongRoom(radius);
  return Math.max(1, Math.floor(room / CHAR_WIDTH));
}

/** The name cut to `n` characters with an ellipsis. */
export function truncate(name: string, n: number): string {
  const chars = Array.from(name.trim());
  if (n <= 0) return '';
  if (chars.length <= n) return chars.join('');
  if (n === 1) return '…';
  return `${chars
    .slice(0, n - 1)
    .join('')
    .trimEnd()}…`;
}

/** The text a wedge shows: the name, cut to the room its direction has. */
export function labelLine(name: string, direction: LabelDirection, angle: number, radius: number): string {
  return truncate(name, labelCapacity(direction, angle, radius));
}

/** The true share of each weight as a whole number of percent, for "About 33% of spins". */
export function sharePercent(weights: readonly number[]): number[] {
  const total = weights.reduce((sum, w) => sum + w, 0);
  if (total <= 0) return weights.map(() => 0);
  return weights.map((w) => Math.round((100 * w) / total));
}
