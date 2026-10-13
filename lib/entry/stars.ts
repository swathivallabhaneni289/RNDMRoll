/**
 * The rating rule (UI spec 5, plan 03-02 Task 1). Pure: no React, no imports.
 *
 * The rating is a whole number `v` from 1 to 10 where `v` is the stored value: 7 means 3.5 stars.
 * 0 means "not rated" on screen only and is never sent. Five stars, each with a left half and
 * a right half: the left half of star `k` is `2k - 1` (a half star), the right half is `2k`.
 * So every value from 1 to 10 is one tap and nothing outside 1 to 10 can be produced.
 */

export const STARS_MIN = 1;
export const STARS_MAX = 10;
/** Stars drawn in the row (and in `starGlyphs`). */
export const STAR_COUNT = 5;

export type StarSide = 'left' | 'right';
export type StarGlyph = 'full' | 'half' | 'empty';

/** A whole number from anything the caller hands in (a bad value counts as 0). */
function toWhole(n: number): number {
  return Number.isFinite(n) ? Math.trunc(n) : 0;
}

function clampStored(n: number): number {
  return Math.min(STARS_MAX, Math.max(STARS_MIN, n));
}

/**
 * The value a tap gives: star `k` (1 to 5, counted from the left), left half `2k - 1`, right
 * half `2k`. A star number outside 1 to 5 is brought back into it, so the answer is always 1 to 10.
 */
export function starFromTap(k: number, side: StarSide): number {
  const star = Math.min(STAR_COUNT, Math.max(1, toWhole(k)));
  return side === 'left' ? 2 * star - 1 : 2 * star;
}

/**
 * One adjust step (VoiceOver increment and decrement). A decrement from 0 stays 0 and an
 * increment from 0 gives 1; once a value is set the result is clamped to 1 to 10, so it never
 * goes below 1 or above 10.
 */
export function stepStars(v: number, delta: number): number {
  const current = toWhole(v);
  const step = toWhole(delta);
  if (current < STARS_MIN) return step > 0 ? clampStored(step) : 0;
  return clampStored(current + step);
}

/** The figure beside the label: 1 gives "0.5", 2 gives "1", 7 gives "3.5". Below 1 there is no figure (""). */
export function formatStars(v: number): string {
  const stored = toWhole(v);
  if (stored < STARS_MIN) return '';
  const value = clampStored(stored);
  const hasHalf = value % 2 === 1;
  const wholeStars = (value - (hasHalf ? 1 : 0)) / 2;
  return hasHalf ? `${wholeStars}.5` : `${wholeStars}`;
}

/**
 * The five glyphs left to right. Position `p` (1 to 5) is `full` when `v` is at least `2p`,
 * `half` when `v` is `2p - 1`, otherwise `empty`. 7 gives full, full, full, half, empty.
 */
export function starGlyphs(v: number): StarGlyph[] {
  const stored = toWhole(v);
  const glyphs: StarGlyph[] = [];
  for (let p = 1; p <= STAR_COUNT; p += 1) {
    if (stored >= 2 * p) glyphs.push('full');
    else if (stored === 2 * p - 1) glyphs.push('half');
    else glyphs.push('empty');
  }
  return glyphs;
}

/** What a screen reader says: "3.5 out of 5 stars", or "not rated" below 1. */
export function starsSpoken(v: number): string {
  const figure = formatStars(v);
  return figure === '' ? 'not rated' : `${figure} out of ${STAR_COUNT} stars`;
}
