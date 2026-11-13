import type { TodayState } from '@/lib/api/types';

/**
 * Where Today shows the Edit wheel link. No React and only a type import, so a plain node script
 * can check it (scripts/check-auth-today.ts).
 *
 * Under the wheel while the next spin waits or is open, and under the countdown once the day is
 * posted or skipped. Posted and Skipped last until the next 8 PM, so someone who posts or skips
 * every evening never sees Waiting: without the link there, the wheel could only be edited between
 * 8 PM and the spin. Never in Result or Late, where a bonus spin can still turn the wheel, nor while
 * a bonus spin brings the wheel back. A change shows from the next spin (SPIN-04): the shown spin
 * keeps its own copy of the wheel.
 */
export function editWheelLinkShown(a: {
  state: TodayState | null;
  composition: 'wheel' | 'result';
  bonusActive: boolean;
  /** Today's first answer has arrived. */
  loaded: boolean;
}): boolean {
  if (!a.loaded || a.bonusActive) return false;
  if (a.composition === 'wheel') return a.state === 'waiting' || a.state === 'open';
  return a.state === 'posted' || a.state === 'skipped';
}
