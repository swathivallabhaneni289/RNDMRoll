import { api } from '@/lib/api/client';
import { readPerson, type Person } from '@/lib/api/social';
import { DECISION_KINDS, type DecisionKind } from '@/lib/notifications/moderation';

/**
 * The list behind the bell (Phase 5, 05-FOLLOW-OVERRIDES section 6). The server builds it, sorts
 * it into sections, writes each time label and says what is unread; the phone prints it. The
 * notification settings already filter it on the server. Every field the app reads is read here.
 * There are no reaction items (reactions on posts were removed, developer, 2026-10-18); a stray one
 * from an older server is dropped like any unknown kind. What moderation did to your own post or
 * comment (removed, back, kept removed after a second look, or hidden while we take a look) comes
 * as eight kinds with no person, whatever the settings say (moderation, 2026-11-17, 2026-11-18 and
 * 2026-11-19; lib/notifications/moderation.ts).
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
  | 'bonus_spin'
  | DecisionKind;

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
   * comment, on your own entry or on someone else's post (`entry_id` is that post). On an admin's
   * decision: the post's title or the comment's words.
   */
  text: string | null;
  /** Set on `follow_request` items (they exist only while the request is pending). */
  request_id: string | null;
  /**
   * The post's first photo for a comment or reply, and for `post_back` and `post_hidden` while you
   * can see the post; '' when there is none.
   */
  photo_url: string;
  /** On an admin's decision: the short ref the admin page shows for it, '' when none. */
  ref: string;
  /**
   * On `post_removed` and `comment_removed`: the reason the admin gave, as a report reason's key
   * ('hate'); '' when none.
   */
  reason: string;
  /**
   * On `post_removed` and `comment_removed`: the removal's full id, which a second look names
   * (lib/api/secondLooks.ts); '' on the other kinds. Never read out of `id`.
   */
  removal: string;
  /**
   * On `post_removed` and `comment_removed`: the second look on it, 'asked', 'kept' or 'restored'
   * ('restored' also when a Bring it back came with none asked), or 'closed' when none was asked
   * and none can be any more (the item was deleted, isn't removed any more, or is past its 30
   * days); '' while one can be asked (canAskSecondLook in lib/notifications/moderation.ts).
   */
  second_look: string;
};

const KINDS: readonly NotificationKind[] = [
  'comment',
  'reply',
  'follow',
  'follow_request',
  'follow_accepted',
  'spin_open',
  'bonus_spin',
  ...DECISION_KINDS,
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
    ref: typeof raw.ref === 'string' ? raw.ref : '',
    reason: typeof raw.reason === 'string' ? raw.reason : '',
    removal: typeof raw.removal === 'string' ? raw.removal : '',
    second_look: typeof raw.second_look === 'string' ? raw.second_look : '',
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
