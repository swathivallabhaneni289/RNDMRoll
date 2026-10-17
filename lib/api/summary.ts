import { api } from '@/lib/api/client';

/**
 * GET /me/summary (plan 04-01 Task 3, the contract of 04-CONTEXT.md): the numbers, the tastes and
 * the last posts of the Profile page. The client adds the `/v1` prefix, so the path here is
 * `/me/summary`. The phone prints what the server sent; it works nothing out.
 *
 * The server side may not be live yet: a 404 or a lost connection is an ordinary `ApiError`, and
 * the Profile page shows zeros and empty lines for it (never an error screen).
 *
 * These types live here, and not in `types.ts`, because `types.ts` holds another plan's
 * uncommitted lines; move them there when that file is clean.
 */

/** A category as the server names it: as written, with `removed` for one the person took off the wheel. */
export type SummaryCategory = { id: string; name: string; removed: boolean };

export type Taste = {
  category: SummaryCategory;
  /** Half of the mean of the stored stars, to one decimal, as the server wrote it (4.6). */
  average: number;
  /** The number of posts behind the average. */
  count: number;
};

/**
 * The part of an Entry the Profile page prints. The server sends the whole Entry (Phase 3's
 * `Entry`); this is a subset on purpose, so this file does not depend on the entry calls.
 */
export type RecentEntry = {
  id: string;
  /** The civil date the post belongs to, "2026-10-08". */
  local_date: string;
  /** The category as written on that day. */
  category: { id: string; name: string };
  title: string;
  /** The cover photo's link ('' or missing when there is none). */
  photo_url?: string | null;
};

export type MeSummary = {
  /** The number of days the person spun. */
  spins: number;
  /** Weeks in a row, ending with this week or last, in which the weekly goal was met. */
  week_streak: number;
  /** Accepted followers (Phase 5: the field was `friends` before follows replaced friends). */
  followers: number;
  /** Accepted follows the person has (the Following figure). */
  following: number;
  /** One per category with at least one post, in wheel order, then removed ones. */
  tastes: Taste[];
  /** At most 3, newest first. */
  recent: RecentEntry[];
};

/** A server may send `null` for an empty list (a nil slice in Go): read both as an empty list. */
function list<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function count(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

export async function getSummary(): Promise<MeSummary> {
  const body = await api.get<Partial<MeSummary>>('/me/summary');
  return {
    spins: count(body.spins),
    week_streak: count(body.week_streak),
    followers: count(body.followers),
    following: count(body.following),
    tastes: list<Taste>(body.tastes),
    recent: list<RecentEntry>(body.recent),
  };
}
