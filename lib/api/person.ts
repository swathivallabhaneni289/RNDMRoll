import { api, ApiError } from '@/lib/api/client';
import { readPerson, type Person, type Relation } from '@/lib/api/social';

/**
 * Another person's page (GET /people/{user_id}): who they are, their four figures, the caller's
 * relation, and, only for an accepted follower, their posts grid under the Board rule (days the
 * caller posted too). Every field is read defensively, as in social.ts.
 */

/** One tile of another person's grid. */
export type PersonPost = {
  id: string;
  local_date: string;
  title: string;
  category: { name: string };
  photo_url: string;
  /** Only its length is read (the stacked-photos mark), so the grid takes it like an entry's photos. */
  photos: { length: number };
};

export type PersonPage = {
  person: Person;
  /** Their bio, '' when none. Shown to anyone who can open the page. */
  bio: string;
  /**
   * 'blocked': you blocked them. The page then has only who they are, zero figures and no posts,
   * like Instagram's (developer, 2026-10-18).
   */
  relation: Relation | 'blocked';
  /** They follow the caller (the button then reads "Follow back"). */
  follows_you: boolean;
  /** The caller muted them: their posts are off the caller's Board. They are never told. */
  muted: boolean;
  stats: { posts: number; spins: number; week_streak: number; followers: number; following: number };
  /** Null when the caller does not follow them (accepted): the page shows the header only. */
  posts: PersonPost[] | null;
  next_cursor: string | null;
};

function num(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function relation(value: unknown): Relation | 'blocked' {
  return value === 'requested' || value === 'following' || value === 'blocked' ? value : 'none';
}

function readPost(value: unknown): PersonPost | null {
  if (value === null || typeof value !== 'object') return null;
  const raw = value as Record<string, unknown>;
  if (typeof raw.entry_id !== 'string' || raw.entry_id === '') return null;
  return {
    id: raw.entry_id,
    local_date: text(raw.local_date),
    title: text(raw.title),
    category: { name: text(raw.category_name) },
    photo_url: text(raw.photo_url),
    photos: { length: num(raw.photo_count) },
  };
}

/**
 * GET /people/{user_id}, a page of the grid after `cursor` when given. Null for a miss, for
 * yourself and for someone who blocked you (the server answers 404 for all of them alike); someone
 * you blocked comes back with relation 'blocked'.
 */
export async function getPerson(userId: string, cursor?: string): Promise<PersonPage | null> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  try {
    const body = await api.get<Record<string, unknown>>(`/people/${encodeURIComponent(userId)}${query}`);
    const person = readPerson(body?.person);
    if (person === null) return null;
    const stats = (body.stats ?? {}) as Record<string, unknown>;
    const posts = Array.isArray(body.posts)
      ? body.posts.map(readPost).filter((post): post is PersonPost => post !== null)
      : null;
    return {
      person,
      bio: text(body.bio),
      relation: relation(body.relation),
      follows_you: body.follows_you === true,
      muted: body.muted === true,
      stats: {
        posts: num(stats.posts),
        spins: num(stats.spins),
        week_streak: num(stats.week_streak),
        followers: num(stats.followers),
        following: num(stats.following),
      },
      posts,
      next_cursor: typeof body.next_cursor === 'string' && body.next_cursor !== '' ? body.next_cursor : null,
    };
  } catch (err) {
    if (err instanceof ApiError && err.code === 'not_found') return null;
    throw err;
  }
}

/** POST /people/{user_id}/mute: their posts leave your Board. Silent. */
export async function mutePerson(userId: string): Promise<void> {
  await api.post<unknown>(`/people/${encodeURIComponent(userId)}/mute`, {});
}

/** POST /people/{user_id}/unmute: their posts come back to your Board. */
export async function unmutePerson(userId: string): Promise<void> {
  await api.post<unknown>(`/people/${encodeURIComponent(userId)}/unmute`, {});
}

/** The reasons a person can be reported for, as the server names them, with the words shown. */
export const REPORT_REASONS = [
  { reason: 'spam', label: 'Spam' },
  { reason: 'nudity', label: 'Nudity or sexual content' },
  { reason: 'harassment', label: 'Bullying or harassment' },
  { reason: 'hate', label: 'Hate speech' },
  { reason: 'violence', label: 'Violence or threats' },
  { reason: 'impersonation', label: 'Pretending to be someone else' },
  { reason: 'other', label: 'Something else' },
] as const;

export type ReportReason = (typeof REPORT_REASONS)[number]['reason'];

/** POST /people/{user_id}/report {reason}. The person is never told. */
export async function reportPerson(userId: string, reason: ReportReason): Promise<void> {
  await api.post<unknown>(`/people/${encodeURIComponent(userId)}/report`, { reason });
}

/**
 * GET /people/{user_id}/followers or /following: their list, for an accepted follower only. Null
 * when the server answers 404 (not following them, a block, or gone).
 */
export async function getPeopleOf(userId: string, list: 'followers' | 'following'): Promise<Person[] | null> {
  try {
    const body = await api.get<Record<string, unknown>>(`/people/${encodeURIComponent(userId)}/${list}`);
    const raw = Array.isArray(body?.people) ? body.people : [];
    return raw.map(readPerson).filter((person): person is Person => person !== null);
  } catch (err) {
    if (err instanceof ApiError && err.code === 'not_found') return null;
    throw err;
  }
}
