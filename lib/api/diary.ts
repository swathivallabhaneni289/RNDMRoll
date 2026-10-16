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

export type DiaryItem =
  | { kind: 'entry'; entry: Entry }
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
    items: list<DiaryItem>(body.items).filter(knownRow),
    next_cursor: typeof next === 'string' && next !== '' ? next : null,
    categories: list<DiaryCategory>(body.categories),
  };
}
