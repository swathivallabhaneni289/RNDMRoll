import { useEffect } from 'react';
import { useReducedMotion, useSharedValue, withTiming } from 'react-native-reanimated';
import { EASE_OUT } from '@/lib/motion/primitives';

/**
 * The one set of numbers for moving around the app (Phase 2.2, decisions D-03, D-07, D-08).
 * The bar, its icons and labels and the tab fade take their timings from here; none of them
 * holds a duration of its own.
 */

/** A pressed item compresses over this long. `PressScale` in primitives.tsx holds the same 90. */
export const pressMs = 90;
/** One tab screen fades into the next over this long. */
export const tabFadeMs = 200;
/** The bar's icon and label change between `muted` and `ink` over this long. */
export const barColorMs = 220;
/** The ease-out curve every entrance and transition shares (it lives in primitives.tsx). */
export const easeOut = EASE_OUT;

/**
 * Whether the phone's Reduce Motion setting was on when the app started, as Reanimated reads it
 * (D-08). Reanimated does not update this while the app runs, so after changing the setting the
 * app has to be closed and opened again. This is not the live `useReduceMotion` hook that the
 * wheel and the Spin button use; the name differs on purpose.
 */
export function useNavReduceMotion(): boolean {
  return useReducedMotion();
}

/**
 * A value that is 0 while `focused` is false and 1 while it is true, and that glides between the
 * two over `barColorMs` when `focused` changes. It starts at the current state, so nothing
 * animates when the bar first appears. With Reduce Motion on it is set at once.
 */
export function useFocusProgress(focused: boolean) {
  const reduceMotion = useNavReduceMotion();
  const progress = useSharedValue(focused ? 1 : 0);

  useEffect(() => {
    const target = focused ? 1 : 0;
    progress.value = reduceMotion
      ? target
      : withTiming(target, { duration: barColorMs, easing: easeOut });
  }, [focused, reduceMotion, progress]);

  return progress;
}
