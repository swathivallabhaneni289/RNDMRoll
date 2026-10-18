import { api } from '@/lib/api/client';
import type { Entry } from '@/lib/api/entries';

/**
 * GET /diary (plan 04-01, the wire shapes of 04-01-SUMMARY): the person's diary, newest first,
 * one row per date, with the category chips sent on every page. The client adds the `/v1`
 * prefix. Archived entries are already left out by the server; skipped rows come only without a
 * category. The phone prints what the server sent and pages with its opaque cursor.
 */

/** A chip: a category with at least one visible entry, wheel order, then removed ones. */
export type DiaryCategory = { id: string; name: string; removed: boolean };

export type LatestComment = { author: { id: string; name: string; username: string; photo_url: string | null }; text: string };

/** The comments on one of your posts, sent on every entry row (2026-10-17): the count and the newest. */
export type PostSocial = {
  comment_count: number;
  latest_comments: LatestComment[];
};

export const NO_SOCIAL: PostSocial = { comment_count: 0, latest_comments: [] };

export type DiaryItem =
  | { kind: 'entry'; entry: Entry; social: PostSocial }
  | { kind: 'skipped'; local_date: string; category: { id: string; name: string } };

export type DiaryPage = {
  items: DiaryItem[];
  /** The cursor of the next page, or null when this page is the last. */
  next_cursor: string | null;
  categories: DiaryCategory[];
};

/** A server may send `null` for an empty list (a nil slice in Go): read both as an empty list. */
function list<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

/** Only the two row kinds this app knows are kept; anything else is dropped, not drawn wrong. */
function knownRow(item: DiaryItem): boolean {
  if (item === null || typeof item !== 'object') return false;
  if (item.kind === 'entry') return typeof item.entry?.id === 'string';
  if (item.kind === 'skipped') return typeof item.local_date === 'string';
  return false;
}

function count(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}

function readLatest(raw: unknown): LatestComment | null {
  if (raw === null || typeof raw !== 'object') return null;
  const c = raw as Record<string, unknown>;
  const author = typeof c.author === 'object' && c.author !== null ? (c.author as Record<string, unknown>) : null;
  if (typeof c.text !== 'string' || author === null || typeof author.id !== 'string') return null;
  return {
    author: {
      id: author.id,
      name: typeof author.name === 'string' ? author.name : '',
      username: typeof author.username === 'string' ? author.username : '',
      photo_url: typeof author.photo_url === 'string' && author.photo_url !== '' ? author.photo_url : null,
    },
    text: c.text,
  };
}

/** The row's `social` as sent, or none from an older server. */
function readSocial(raw: unknown): PostSocial {
  if (raw === null || typeof raw !== 'object') return NO_SOCIAL;
  const s = raw as Record<string, unknown>;
  return {
    comment_count: count(s.comment_count),
    latest_comments: list<unknown>(s.latest_comments)
      .map(readLatest)
      .filter((c): c is LatestComment => c !== null),
  };
}

/**
 * One page. Only the keys given go into the query string: no `category_id` means All, no
 * `cursor` means the first page, and the server's default `limit` is 20 (1 to 50).
 */
export async function getDiary({
  categoryId,
  cursor,
  limit,
}: {
  categoryId?: string | null;
  cursor?: string | null;
  limit?: number;
} = {}): Promise<DiaryPage> {
  const query: string[] = [];
  if (categoryId) query.push(`category_id=${encodeURIComponent(categoryId)}`);
  if (cursor) query.push(`cursor=${encodeURIComponent(cursor)}`);
  if (limit !== undefined) query.push(`limit=${limit}`);
  const path = query.length > 0 ? `/diary?${query.join('&')}` : '/diary';

  const body = await api.get<Partial<DiaryPage>>(path);
  const next = body.next_cursor;
  return {
    items: list<DiaryItem>(body.items)
      .filter(knownRow)
      .map((item) => (item.kind === 'entry' ? { ...item, social: readSocial((item as { social?: unknown }).social) } : item)),
    next_cursor: typeof next === 'string' && next !== '' ? next : null,
    categories: list<DiaryCategory>(body.categories),
  };
}
