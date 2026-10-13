import { api } from '@/lib/api/client';
import type { SpinResponse, Today, WheelResponse, WheelSaveEntry } from '@/lib/api/types';

/**
 * The daily spin and the wheel (plan 02-01 Task 3b and 3c). Modelled on `lib/api/profile.ts`:
 * thin calls over the typed client, no reshaping. The wire types live in `types.ts` with the
 * rest of the wire contract and are re-exported here so a screen can take everything it needs
 * from one place.
 *
 * Every call is an ordinary signed-in call. The server owns the date, the window, the winner,
 * the time a spin was made and the next opening's weekday; nothing here works any of them out.
 * The client only sends GET, POST and PATCH, so saving a wheel is one PATCH of the whole list.
 */
export type {
  Bonus,
  Spin,
  SpinResponse,
  Today,
  TodayEntry,
  TodayState,
  Week,
  WeekDay,
  WheelCategory,
  WheelEntry,
  WheelLimits,
  WheelResponse,
  WheelSaveEntry,
} from '@/lib/api/types';

/** Tells the server which time zone the phone is in. Sent on mount, on foreground and on a change. */
export async function reportTimeZone(timeZone: string): Promise<void> {
  await api.post<{ time_zone: string }>('/time-zone', { time_zone: timeZone });
}

/** Today's state for the signed-in person (may make their starting wheel on first use). */
export async function getToday(): Promise<Today> {
  return api.get<Today>('/today');
}

/**
 * The day's daily spin. `forDate` is the `local_date` of the Today that is on screen, so a
 * stale screen is refused instead of spinning another day. A retry returns the same spin.
 */
export async function spin(forDate: string): Promise<SpinResponse> {
  return api.post<SpinResponse>('/spin', { for_date: forDate });
}

/**
 * A bonus spin. `afterSeq` is the `seq` of the spin that was on screen when the person said
 * yes, so a retry across a lost answer never spends the bonus twice.
 */
export async function spinBonus(afterSeq: number, forDate: string): Promise<SpinResponse> {
  return api.post<SpinResponse>('/spin/bonus', { after_seq: afterSeq, for_date: forDate });
}

export async function getWheel(): Promise<WheelResponse> {
  return api.get<WheelResponse>('/wheel');
}

/** Saves the whole list in one request (all or nothing). A stale `version` is a 409 wheel_stale. */
export async function saveWheel(version: number, list: WheelSaveEntry[]): Promise<WheelResponse> {
  return api.patch<WheelResponse>('/wheel', { version, categories: list });
}
