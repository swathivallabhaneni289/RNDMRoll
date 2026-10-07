/**
 * Client-side birthday rules for the profile form. Three plain number fields (Month, Day,
 * Year), no native date picker. The server decides again with its own clock; this only
 * gives the person a message before they press Continue.
 */

export const MIN_AGE = 13;
export const MIN_BIRTHDAY_YEAR = 1900;

export interface BirthdayParts {
  month: string;
  day: string;
  year: string;
}

export const EMPTY_BIRTHDAY: BirthdayParts = { month: '', day: '', year: '' };

export const BIRTHDAY_MESSAGES = {
  incomplete: 'Enter your birthday as month, day and year.',
  year: 'Enter a four-digit year.',
  notReal: "That date doesn't exist. Check the day and month.",
  future: 'That date is in the future.',
  underAge: 'You must be at least 13 to use RNDMRoll.',
} as const;

export type BirthdayCheck = { ok: true; iso: string } | { ok: false; error: string };

function pad(value: number, width: number): string {
  return String(value).padStart(width, '0');
}

export interface BirthdayOptions {
  /**
   * Skip the 13+ rule. Finish mode sets this so an under-13 birthday still reaches the
   * server, which deletes the unfinished account and answers under_minimum_age.
   */
  ignoreAge?: boolean;
}

/**
 * Checks the three fields against `now` (the device clock) and returns the YYYY-MM-DD string
 * the server expects, or the message to show. A year before 1900 gets the year message.
 * "In the future" is judged on the local date (the server allows up to UTC+14); the age is
 * judged on the UTC date, exactly as the server does.
 */
export function checkBirthday(
  parts: BirthdayParts,
  now: Date = new Date(),
  options: BirthdayOptions = {}
): BirthdayCheck {
  const month = parts.month.trim();
  const day = parts.day.trim();
  const year = parts.year.trim();

  if (month === '' || day === '' || year === '') {
    return { ok: false, error: BIRTHDAY_MESSAGES.incomplete };
  }
  if (!/^\d{1,2}$/.test(month) || !/^\d{1,2}$/.test(day)) {
    return { ok: false, error: BIRTHDAY_MESSAGES.notReal };
  }
  if (!/^\d{4}$/.test(year) || Number(year) < MIN_BIRTHDAY_YEAR) {
    return { ok: false, error: BIRTHDAY_MESSAGES.year };
  }

  const m = Number(month);
  const d = Number(day);
  const y = Number(year);

  // Date rolls an impossible day (31 / 02) into the next month, so a real date is one that
  // survives the round trip unchanged.
  const candidate = new Date(y, m - 1, d);
  if (candidate.getFullYear() !== y || candidate.getMonth() !== m - 1 || candidate.getDate() !== d) {
    return { ok: false, error: BIRTHDAY_MESSAGES.notReal };
  }

  const todayY = now.getFullYear();
  const todayM = now.getMonth() + 1;
  const todayD = now.getDate();

  const isFuture = y > todayY || (y === todayY && (m > todayM || (m === todayM && d > todayD)));
  if (isFuture) {
    return { ok: false, error: BIRTHDAY_MESSAGES.future };
  }

  // Year, month, day comparison on the UTC date: someone born on Feb 29 turns a year older
  // on Mar 1 in a non-leap year, the same rule and the same clock the server uses.
  const utcY = now.getUTCFullYear();
  const utcM = now.getUTCMonth() + 1;
  const utcD = now.getUTCDate();
  let age = utcY - y;
  if (utcM < m || (utcM === m && utcD < d)) age -= 1;
  if (!options.ignoreAge && age < MIN_AGE) {
    return { ok: false, error: BIRTHDAY_MESSAGES.underAge };
  }

  return { ok: true, iso: `${pad(y, 4)}-${pad(m, 2)}-${pad(d, 2)}` };
}
