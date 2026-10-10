import { useEffect, useState } from 'react';
import { AccessibilityInfo } from 'react-native';

/**
 * Whether the phone's Reduce Motion setting is on, kept current with the
 * `reduceMotionChanged` listener (no new native module: AccessibilityInfo is React Native's).
 * The wheel and the Today reveal read it so no turning, rise, fade or press scale runs under it.
 */
export function useReduceMotion(): boolean {
  const [reduce, setReduce] = useState(false);

  useEffect(() => {
    let active = true;
    AccessibilityInfo.isReduceMotionEnabled()
      .then((enabled) => {
        if (active) setReduce(enabled);
      })
      .catch(() => {});
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', (enabled) => {
      setReduce(enabled);
    });
    return () => {
      active = false;
      subscription.remove();
    };
  }, []);

  return reduce;
}

/** The setting read right now, for code that acts at a tap or at the start of a turn. */
export async function readReduceMotion(): Promise<boolean> {
  try {
    return await AccessibilityInfo.isReduceMotionEnabled();
  } catch {
    return false;
  }
}
