/**
 * Pure countdown helpers. No React, no app imports. The phone never works out the date or the
 * time a window opens: the server sends `now` and the instants, and the phone only counts the
 * time since the answer arrived with a monotonic clock (`performance.now()`, passed in by the
 * caller so nothing here reads any clock).
 */

export type CountdownParts = { hours: string; minutes: string; seconds: string };

function two(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/**
 * Splits a number of milliseconds into two-digit hours, minutes and seconds. Part seconds
 * count as a whole second, so "00:00:01" shows until the instant itself and 00:00:00 means
 * it is time. Never negative; hours stop at 99 so the row never grows past three cells.
 */
export function split(msRemaining: number): CountdownParts {
  const total = Math.max(0, Math.ceil((Number.isFinite(msRemaining) ? msRemaining : 0) / 1000));
  const hours = Math.min(99, Math.floor(total / 3600));
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return { hours: two(hours), minutes: two(minutes), seconds: two(seconds) };
}

/**
 * Whole hours and minutes for the spoken line, "5 hours 4 minutes" (the screen reader reads
 * "Your next spin opens in 5 hours 4 minutes, at 8:00 PM.", updated once a minute).
 */
export function spokenMinutes(ms: number): string {
  const totalMinutes = Math.max(0, Math.floor((Number.isFinite(ms) ? ms : 0) / 60000));
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (totalMinutes < 1) return 'less than a minute';
  const hourPart = `${hours} ${hours === 1 ? 'hour' : 'hours'}`;
  const minutePart = `${minutes} ${minutes === 1 ? 'minute' : 'minutes'}`;
  if (hours === 0) return minutePart;
  if (minutes === 0) return hourPart;
  return `${hourPart} ${minutePart}`;
}

/** Days since 1970-01-01 of a civil date (proleptic Gregorian). Month is 1 to 12. */
function daysFromCivil(year: number, month: number, day: number): number {
  const y = month <= 2 ? year - 1 : year;
  const era = Math.floor(y / 400);
  const yearOfEra = y - era * 400;
  const dayOfYear = Math.floor((153 * (month + (month > 2 ? -3 : 9)) + 2) / 5) + day - 1;
  const dayOfEra = yearOfEra * 365 + Math.floor(yearOfEra / 4) - Math.floor(yearOfEra / 100) + dayOfYear;
  return era * 146097 + dayOfEra - 719468;
}

const RFC3339 = /^(\d{4})-(\d{2})-(\d{2})[Tt ](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?([Zz]|[+-]\d{2}:?\d{2})$/;

/**
 * An RFC 3339 instant as milliseconds since 1970-01-01 UTC, or null when it does not parse.
 * Written by hand so lib/wheel never touches the phone's date object; the check script compares
 * it with the built-in parser.
 */
export function parseInstant(text: string | null | undefined): number | null {
  if (typeof text !== 'string') return null;
  const m = RFC3339.exec(text.trim());
  if (!m) return null;
  const [, y, mo, d, h, mi, s, frac, zone] = m;
  const month = Number(mo);
  const day = Number(d);
  const hour = Number(h);
  const minute = Number(mi);
  const second = Number(s);
  if (month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || second > 60) return null;
  const millis = frac ? Number(frac.padEnd(3, '0').slice(0, 3)) : 0;
  let offsetMinutes = 0;
  if (zone !== 'Z' && zone !== 'z') {
    const sign = zone[0] === '-' ? -1 : 1;
    const digits = zone.slice(1).replace(':', '');
    offsetMinutes = sign * (Number(digits.slice(0, 2)) * 60 + Number(digits.slice(2, 4)));
  }
  const days = daysFromCivil(Number(y), month, day);
  return ((days * 24 + hour) * 60 + minute - offsetMinutes) * 60000 + second * 1000 + millis;
}

/** What the server's clock reads right now: its `now` plus the time counted since the answer. */
export function serverNowMs(anchor: { serverMs: number; mark: number }, perfNow: number): number {
  return anchor.serverMs + Math.max(0, perfNow - anchor.mark);
}

/** Milliseconds from the server's now to `instant` (negative once it has passed), or null. */
export function msUntil(
  instant: string | null | undefined,
  anchor: { serverMs: number; mark: number },
  perfNow: number
): number | null {
  const at = parseInstant(instant);
  if (at === null) return null;
  return at - serverNowMs(anchor, perfNow);
}

/** When a window opens, 8:00 PM, in minutes after midnight. */
const WINDOW_OPENS_MINUTE = 20 * 60;
const MINUTES_IN_DAY = 24 * 60;

/**
 * The time of day the next spin opens, written as the server writes its times ("8:00 PM",
 * "9:50 PM": 12-hour, no leading zero, part minutes dropped). It is 8:00 PM, except after a big
 * time zone change: the wheel then opens when the 12 hours since the last spin end, which can be
 * later that evening, and the line under the countdown names that moment, the one it counts to.
 * Worked out from the server's instants alone, with no date object and no locale. The only opening
 * the server gives that is not some date's 8:00 PM is the end of that gap tonight, inside the
 * window of the person's date (Today's `window`, 8:00 PM to midnight in their zone), so it is
 * 8:00 PM plus the time since the window opened. Any other opening, or an instant that does not
 * parse, is 8:00 PM.
 */
export function opensAtTime(
  nextOpenAt: string | null | undefined,
  window: { opens_at: string; closes_at: string } | null | undefined
): string {
  const at = parseInstant(nextOpenAt);
  const opens = parseInstant(window?.opens_at);
  const closes = parseInstant(window?.closes_at);
  let minutes = WINDOW_OPENS_MINUTE;
  if (at !== null && opens !== null && closes !== null && at >= opens && at < closes) {
    minutes += Math.floor((at - opens) / 60000);
    // A clock turned back an hour at midnight gives that evening a fifth hour: the second 11 PM.
    if (minutes >= MINUTES_IN_DAY) minutes -= 60;
  }
  const hour = Math.floor(minutes / 60);
  return `${hour % 12 === 0 ? 12 : hour % 12}:${two(minutes % 60)} ${hour < 12 ? 'AM' : 'PM'}`;
}

/**
 * `opensAtTime` for a page other than Spin that reads Today's last answer (the Board's locked
 * card): null when there is no answer, no anchor (the app has just come back to the front and asks
 * again), or that answer's opening has passed. A stale answer then never names an evening that is
 * over, and the page says 8:00 PM as before.
 */
export function opensAtFromAnswer(
  today: { next_open_at: string; window: { opens_at: string; closes_at: string } } | null,
  anchor: { serverMs: number; mark: number } | null,
  perfNow: number
): string | null {
  if (today === null || anchor === null) return null;
  const left = msUntil(today.next_open_at, anchor, perfNow);
  if (left === null || left <= 0) return null;
  return opensAtTime(today.next_open_at, today.window);
}

/**
 * The shortest wait, in milliseconds, to the next of `instants` that is still ahead of the
 * server's now (nearest first), or null when none is. Used to re-arm the refetch timer after
 * every answer.
 */
export function nextWaitMs(
  instants: ReadonlyArray<string | null | undefined>,
  anchor: { serverMs: number; mark: number },
  perfNow: number
): number | null {
  let best: number | null = null;
  for (const instant of instants) {
    const wait = msUntil(instant, anchor, perfNow);
    if (wait === null || wait <= 0) continue;
    if (best === null || wait < best) best = wait;
  }
  return best;
}
