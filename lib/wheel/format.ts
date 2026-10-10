/**
 * Date lines from the server's civil date ("2026-10-08"), by arithmetic and fixed arrays: no
 * date object and no locale formatting, so the phone shows the server's date in the same words
 * on every device. Phase 3's entry page and Phase 4 use `dateLine`; Today uses `dateLineLong`.
 *
 * Nothing in lib/wheel formats a time or works out a weekday NAME for the screen apart from
 * these date lines: the stamp time (`spin.spun_local_time`) and the next opening's weekday
 * (`next_open_weekday`) are printed exactly as the server sent them.
 */

const WEEKDAYS_SHORT = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'] as const;
const WEEKDAYS_LONG = ['SUNDAY', 'MONDAY', 'TUESDAY', 'WEDNESDAY', 'THURSDAY', 'FRIDAY', 'SATURDAY'] as const;
const MONTHS_SHORT = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'] as const;
const MONTHS_LONG = [
  'JANUARY',
  'FEBRUARY',
  'MARCH',
  'APRIL',
  'MAY',
  'JUNE',
  'JULY',
  'AUGUST',
  'SEPTEMBER',
  'OCTOBER',
  'NOVEMBER',
  'DECEMBER',
] as const;

const CIVIL_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

const DAYS_IN_MONTH = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];

function isLeap(year: number): boolean {
  return (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
}

function parseCivil(localDate: string): { year: number; month: number; day: number } | null {
  const m = CIVIL_DATE.exec(localDate);
  if (!m) return null;
  const year = Number(m[1]);
  const month = Number(m[2]);
  const day = Number(m[3]);
  if (month < 1 || month > 12 || day < 1) return null;
  const last = DAYS_IN_MONTH[month - 1] + (month === 2 && isLeap(year) ? 1 : 0);
  if (day > last) return null;
  return { year, month, day };
}

/** Day of the week, 0 = Sunday, by Sakamoto's method (pure arithmetic). */
function weekdayIndex(year: number, month: number, day: number): number {
  const offsets = [0, 3, 2, 5, 0, 3, 5, 1, 4, 6, 2, 4];
  const y = month < 3 ? year - 1 : year;
  return (y + Math.floor(y / 4) - Math.floor(y / 100) + Math.floor(y / 400) + offsets[month - 1] + day) % 7;
}

/** "WED, OCT 8" (short form). Empty string when the date does not parse. */
export function dateLine(localDate: string): string {
  const c = parseCivil(localDate);
  if (!c) return '';
  const weekday = WEEKDAYS_SHORT[weekdayIndex(c.year, c.month, c.day)];
  return `${weekday}, ${MONTHS_SHORT[c.month - 1]} ${c.day}`;
}

/** "THURSDAY, OCTOBER 8" (long form, the Today date line). Empty string when the date does not parse. */
export function dateLineLong(localDate: string): string {
  const c = parseCivil(localDate);
  if (!c) return '';
  const weekday = WEEKDAYS_LONG[weekdayIndex(c.year, c.month, c.day)];
  return `${weekday}, ${MONTHS_LONG[c.month - 1]} ${c.day}`;
}
