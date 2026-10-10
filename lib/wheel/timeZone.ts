/**
 * The time zone name the phone is set to, for example "Europe/London", or null when it cannot
 * be read. Reported to the server (plan 02-02 binding decision 7); the server decides the date.
 */
export function deviceTimeZone(): string | null {
  try {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    return typeof zone === 'string' && zone.length > 0 ? zone : null;
  } catch {
    return null;
  }
}
