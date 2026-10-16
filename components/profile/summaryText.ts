/**
 * The words of the Profile page's figures and lists (04-UI-SPEC sections 4 and 8): pure functions
 * that turn what the server sent into what is printed and what is spoken. No React and no app
 * imports, so a plain node script can check them.
 *
 * The phone works nothing out. The only date work is a fixed table of month names that turns the
 * server's civil date ("2026-10-08") into "Oct 8": no `Date`, no `Intl`, no weekday.
 */

const MONTHS_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'] as const;
const MONTHS_LONG = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
] as const;

const CIVIL_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

function parse(localDate: string): { month: number; day: number } | null {
  const m = CIVIL_DATE.exec(localDate);
  if (!m) return null;
  const month = Number(m[2]);
  const day = Number(m[3]);
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  return { month, day };
}

/** "Oct 8" from "2026-10-08" (no leading zero on the day). Empty when the date does not parse. */
export function monthDay(localDate: string): string {
  const c = parse(localDate);
  return c ? `${MONTHS_SHORT[c.month - 1]} ${c.day}` : '';
}

/** "October 8", for what is spoken. Empty when the date does not parse. */
export function monthDayLong(localDate: string): string {
  const c = parse(localDate);
  return c ? `${MONTHS_LONG[c.month - 1]} ${c.day}` : '';
}

/** "Meal \u00B7 Oct 8" (the screen draws it in capitals by style). */
export function recentLabel(categoryName: string, localDate: string): string {
  const date = monthDay(localDate);
  return date === '' ? categoryName : `${categoryName} \u00B7 ${date}`;
}

/** "4.6", always one decimal ("4.0"), from the average the server sent. */
export function averageText(average: number): string {
  return average.toFixed(1);
}

/** "1 spin", "42 spins". */
export function spinsSpoken(n: number): string {
  return n === 1 ? '1 spin' : `${n} spins`;
}

/** "Week streak, 4". */
export function streakSpoken(n: number): string {
  return `Week streak, ${n}`;
}

/** "1 follower", "5 followers". */
export function followersSpoken(n: number): string {
  return n === 1 ? '1 follower' : `${n} followers`;
}

/** "Movie, average 4.6 out of 5, 9 posts." ("1 post"). */
export function tasteSpoken(name: string, average: number, count: number): string {
  const posts = count === 1 ? '1 post' : `${count} posts`;
  return `${name}, average ${averageText(average)} out of 5, ${posts}.`;
}

/** "Mushroom ramen at Kuro. Meal. October 8." */
export function recentSpoken(title: string, categoryName: string, localDate: string): string {
  const date = monthDayLong(localDate);
  return date === '' ? `${title}. ${categoryName}.` : `${title}. ${categoryName}. ${date}.`;
}
