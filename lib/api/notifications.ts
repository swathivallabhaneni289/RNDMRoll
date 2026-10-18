import { api } from '@/lib/api/client';
import { readPerson, type Person } from '@/lib/api/social';

/**
 * The list behind the bell (Phase 5, 05-FOLLOW-OVERRIDES section 6). The server builds it, sorts
 * it into sections, writes each time label and says what is unread; the phone prints it. The
 * notification settings already filter it on the server. Every field the app reads is read here.
 * There are no reaction items (reactions on posts were removed, developer, 2026-10-18); a stray one
 * from an older server is dropped like any unknown kind.
 *
 * The paths have no `/v1`: the client adds it.
 */

export type NotificationKind =
  | 'comment'
  | 'reply'
  | 'follow'
  | 'follow_request'
  | 'follow_accepted'
  | 'spin_open'
  | 'bonus_spin';

export type NotificationSection = 'today' | 'week' | 'earlier';

export type NotificationItem = {
  id: string;
  kind: NotificationKind;
  at: string;
  /** "9:18 PM", "Mon" or "Oct 3", written by the server in the viewer's zone. */
  at_label: string;
  section: NotificationSection;
  unread: boolean;
  actor: Person | null;
  entry_id: string | null;
  category_name: string | null;
  /**
   * A comment's or reply's text, already cut by the server. A `reply` item is a reply to your
   * comment, on your own entry or on someone else's post (`entry_id` is that post).
   */
  text: string | null;
  /** Set on `follow_request` items (they exist only while the request is pending). */
  request_id: string | null;
  /** The post's first photo for a comment or reply; '' when there is none. */
  photo_url: string;
};

const KINDS: readonly NotificationKind[] = [
  'comment',
  'reply',
  'follow',
  'follow_request',
  'follow_accepted',
  'spin_open',
  'bonus_spin',
];

function textOrNull(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null;
}

function readItem(raw: Record<string, unknown>): NotificationItem | null {
  if (typeof raw.id !== 'string' || !KINDS.includes(raw.kind as NotificationKind)) return null;
  const section = raw.section === 'today' || raw.section === 'week' ? raw.section : 'earlier';
  return {
    id: raw.id,
    kind: raw.kind as NotificationKind,
    at: typeof raw.at === 'string' ? raw.at : '',
    at_label: typeof raw.at_label === 'string' ? raw.at_label : '',
    section,
    unread: raw.unread === true,
    actor: readPerson(raw.actor),
    entry_id: textOrNull(raw.entry_id),
    category_name: textOrNull(raw.category_name),
    text: textOrNull(raw.text),
    request_id: textOrNull(raw.request_id),
    photo_url: typeof raw.photo_url === 'string' ? raw.photo_url : '',
  };
}

/** GET /notifications: `{unread, items}`, newest first. */
export async function getNotifications(): Promise<{ unread: boolean; items: NotificationItem[] }> {
  const body = await api.get<Record<string, unknown>>('/notifications');
  const raw = Array.isArray(body?.items) ? (body.items as unknown[]) : [];
  const items: NotificationItem[] = [];
  for (const item of raw) {
    if (item === null || typeof item !== 'object') continue;
    const read = readItem(item as Record<string, unknown>);
    if (read !== null) items.push(read);
  }
  return { unread: body?.unread === true, items };
}

/** GET /notifications/summary: `{unread}`, for the dot on the bell. */
export async function getNotificationSummary(): Promise<boolean> {
  const body = await api.get<{ unread?: unknown }>('/notifications/summary');
  return body?.unread === true;
}

/** POST /notifications/seen: everything up to now is read. */
export async function markNotificationsSeen(): Promise<void> {
  await api.post<unknown>('/notifications/seen', {});
}
