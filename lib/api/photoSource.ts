/**
 * Photo links on the phone (private photos, 2026-11-09). Every photo the server sends is a signed
 * link to private storage, signed for 60 minutes from a time rounded down to the half hour: the same
 * photo comes back with the same link for half an hour and with a new one after that, and no link
 * works for more than an hour. The field names stay `avatar_url` and `photo_url`.
 * Pure: no React, no imports (scripts/check-entry.ts loads it as it is).
 */

/**
 * How old a screen's data may get before a return from the background reads it again
 * (useForegroundReload, and your own profile in lib/session/store.ts): 20 minutes. A link has at
 * least 60 minus 30 = 30 minutes left when it arrives, so this must stay under 60 minus 30: data
 * younger than it still holds links with 10 minutes or more to run, and older data is read again.
 */
export const STALE_AFTER_MS = 20 * 60 * 1000;

/**
 * Whether data that answered at `loadedAt` (Date.now(), null before any answer) is old enough to
 * read again: over STALE_AFTER_MS. Younger data holds no link that has run out, so a photo in it
 * that will not load is not fixed by reading it again.
 */
export function isStale(loadedAt: number | null, now: number): boolean {
  return loadedAt !== null && now - loadedAt > STALE_AFTER_MS;
}

/**
 * The source an expo-image `Image` loads a photo from. A server link (http or https) gets a
 * `cacheKey`: the link without its query string, which is the photo's storage path and never
 * changes. A fresh link for a photo the phone already saved then shows the saved copy with no new
 * download, and a saved copy never needs its old link again. Only the query goes: the path stays
 * exactly as sent. A local file (a pick not yet uploaded) is loaded as it is, with no cache key.
 */
export function photoSource(uri: string): { uri: string; cacheKey?: string } {
  if (!/^https?:\/\//i.test(uri)) return { uri };
  const query = uri.indexOf('?');
  return { uri, cacheKey: query === -1 ? uri : uri.slice(0, query) };
}
