import { Easing } from 'react-native-reanimated';

/**
 * One set of durations and curves for every lab animation, so the three screens
 * move like one product (docs/motion-interaction-direction.md: keep durations and
 * easing consistent). Curves are ease-out or ease-in-out cubic. There is no spring
 * with overshoot anywhere in the lab.
 */
export const duration = {
  /** Press feedback compress. */
  press: 90,
  /** Fades that only confirm a state change. */
  quick: 180,
  /** Text reveals, view-switcher indicator. */
  base: 260,
  /** Grid to diary crossfade, total. */
  switch: 300,
  /** Photo travelling between a list and the detail view. */
  hero: 340,
} as const;

/** Gap between sibling reveals; the motion doc allows 60 to 120ms. */
export const STAGGER = 80;

export const easeOut = Easing.out(Easing.cubic);
export const easeInOut = Easing.inOut(Easing.cubic);
