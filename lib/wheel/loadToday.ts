import type { TimeZoneResponse, Today } from '@/lib/api/types';

/**
 * One load of Today: the time zone report and GET /today, in order. No React and only type
 * imports, so a plain node script can check it (scripts/check-today-load.ts). `useToday` runs it
 * through its one-at-a-time queue and puts the result in the store.
 *
 * Plan 02-02 decision 7 reported the zone on every mount and foreground and waited for that answer
 * before asking for Today. Since 2026-11-16 (the Spin page took 2.5 s after sign-up):
 *  - A session's first load reports the zone. A later one reports it only when the phone's zone
 *    changed, and a tab focus never does by itself (`mustReportZone`).
 *  - The report's answer carries Today, and then that is the whole load. An older server answers
 *    with the zone alone, and GET /today follows as before.
 *  - A failed report never blocks Today: the GET still goes, and the server uses the stored zone.
 *  - A load that did not report and is answered 400 on `time_zone` (nothing stored, or the stored
 *    name no longer loads) reports, then takes that answer's Today or asks once more.
 *  - A Today counted in another zone than the phone's (another phone reported its own) is followed
 *    by a report, so the server counts in this phone's zone again. If that fails, the Today in hand
 *    is used.
 *  - At most one report per load, so none of this can repeat itself.
 */

/** The zone the server took from this phone, and the count of sign ins it was taken under. */
export type ReportedZone = { zone: string; session: number };

/**
 * Whether a load reports the phone's zone before asking for Today. A session's first load always
 * does (nothing was reported under this sign in yet). A later one does when the zone is not the one
 * reported, except on a tab focus: that only asks for Today, and `loadToday` reports if the answer
 * is in another zone.
 */
export function mustReportZone(
  zone: string,
  last: ReportedZone | null,
  session: number,
  tabFocus: boolean
): boolean {
  if (last === null || last.session !== session) return true;
  return !tabFocus && last.zone !== zone;
}

/**
 * The server has no usable zone for this person: 400 validation_failed on `time_zone`. Read from
 * the error's fields (an ApiError's status, code and field), so this file needs no app import.
 */
export function isZoneRefusal(err: unknown): boolean {
  if (typeof err !== 'object' || err === null) return false;
  const e = err as { status?: unknown; code?: unknown; field?: unknown };
  return e.status === 400 && e.code === 'validation_failed' && e.field === 'time_zone';
}

/** The two requests and the clock, passed in so a check can run a load with no server. */
export type TodayRequests = {
  reportZone: (zone: string) => Promise<TimeZoneResponse>;
  getToday: () => Promise<Today>;
  /** `performance.now()` in the app. */
  now: () => number;
};

/**
 * What one load got. `mark` is `now()` taken just before the request that brought `today` (the
 * countdown counts from it). `reported` is true once a report was answered 2xx, also when the load
 * failed after it.
 */
export type TodayLoad =
  | { ok: true; today: Today; mark: number; reported: boolean }
  | { ok: false; error: unknown; reported: boolean };

type Got = { today: Today; mark: number };

export async function loadToday(zone: string, report: boolean, requests: TodayRequests): Promise<TodayLoad> {
  let reported = false;
  let reportSent = false;

  /** The one report a load may send: its Today, or null from an older server. Throws when it fails. */
  async function sendReport(): Promise<Got | null> {
    reportSent = true;
    const mark = requests.now();
    const answer = await requests.reportZone(zone);
    reported = true;
    return answer.today ? { today: answer.today, mark } : null;
  }

  async function get(): Promise<Got> {
    const mark = requests.now();
    return { today: await requests.getToday(), mark };
  }

  if (report) {
    try {
      const got = await sendReport();
      if (got) return { ok: true, ...got, reported };
    } catch {
      // A failed report never blocks Today: the GET below still goes.
    }
  }

  let got: Got;
  try {
    got = await get();
  } catch (error) {
    if (reportSent || !isZoneRefusal(error)) return { ok: false, error, reported };
    // No usable zone on the server: report it, then Today.
    try {
      return { ok: true, ...((await sendReport()) ?? (await get())), reported };
    } catch (retryError) {
      return { ok: false, error: retryError, reported };
    }
  }

  if (!reportSent && got.today.time_zone !== zone) {
    // Counted in another phone's zone: this phone's goes back, and its Today is the one shown.
    try {
      return { ok: true, ...((await sendReport()) ?? (await get())), reported };
    } catch {
      // The Today in hand is still Today.
    }
  }
  return { ok: true, ...got, reported };
}
