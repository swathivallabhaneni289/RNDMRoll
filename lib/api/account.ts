import { api } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';

/**
 * Account calls of the Settings pages (plan 02.1-02, and the notification and block calls of
 * decision D-16). Thin, like `lib/api/profile.ts`: no reshaping beyond filling a missing field, and
 * nothing here logs or keeps either password.
 *
 * The paths have no `/v1`: the client adds it.
 */

/**
 * POST /me/password (plan 02.1-01). The server checks the current password, stores the new one,
 * signs every other phone out and answers with a log-in shaped body: both tokens and the user. The
 * caller hands that body to the session's sign in, so this phone stays signed in on the new tokens.
 *
 * Errors (all surface as `ApiError`): a 400 `validation_failed` whose `field` is `current_password`
 * (wrong current password, never a 401) or `new_password`, a 429 `rate_limited`, and
 * `network_unavailable`.
 */
export async function changePassword(currentPassword: string, newPassword: string): Promise<AuthResult> {
  return api.post<AuthResult>('/me/password', {
    current_password: currentPassword,
    new_password: newPassword,
  });
}

// ---------------------------------------------------------------------------
// Notification settings (Settings > Notifications).
// ---------------------------------------------------------------------------

/**
 * What the person is told about. All three are ON for everyone until they switch one off.
 * These shapes are the wire contract of the server plan that follows Phase 3's server; they live
 * here and not in `types.ts` so that plan can land its server without this app file moving.
 */
export type NotificationSettings = {
  /** Reactions and likes on their posts. */
  reactions: boolean;
  /** Comments on their posts. */
  comments: boolean;
  /** Milestones and bonus spins. */
  milestones: boolean;
};

export type NotificationSettingKey = keyof NotificationSettings;

/** What the switches show when the server cannot answer (not live yet, or no connection). */
export const NOTIFICATION_DEFAULTS: NotificationSettings = {
  reactions: true,
  comments: true,
  milestones: true,
};

function settingsFrom(body: Partial<NotificationSettings> | null | undefined, fallback: NotificationSettings) {
  return {
    reactions: typeof body?.reactions === 'boolean' ? body.reactions : fallback.reactions,
    comments: typeof body?.comments === 'boolean' ? body.comments : fallback.comments,
    milestones: typeof body?.milestones === 'boolean' ? body.milestones : fallback.milestones,
  };
}

/** GET /me/notification-settings: `{reactions, comments, milestones}`. */
export async function getNotificationSettings(): Promise<NotificationSettings> {
  const body = await api.get<Partial<NotificationSettings>>('/me/notification-settings');
  return settingsFrom(body, NOTIFICATION_DEFAULTS);
}

/**
 * PATCH /me/notification-settings with any of the three booleans; the answer is the same body as
 * the GET (all three, as stored now). A key the answer leaves out keeps the value that was sent.
 */
export async function updateNotificationSettings(
  patch: Partial<NotificationSettings>
): Promise<NotificationSettings> {
  const body = await api.patch<Partial<NotificationSettings>>('/me/notification-settings', patch);
  return settingsFrom(body, { ...NOTIFICATION_DEFAULTS, ...patch });
}

// ---------------------------------------------------------------------------
// Blocked accounts (Settings > Blocked accounts).
// ---------------------------------------------------------------------------

/** The person who was blocked, as the server describes them. Any text field may be null. */
export type BlockedUser = {
  id: string;
  name: string | null;
  username: string | null;
  photo_url: string | null;
};

export type BlockedAccount = {
  user: BlockedUser;
  /** When the block was made, an RFC 3339 instant. */
  blocked_at: string;
};

/** GET /me/blocks: `{items: [{user, blocked_at}]}`. A missing or null list is an empty one. */
export async function getBlocks(): Promise<BlockedAccount[]> {
  const body = await api.get<{ items?: BlockedAccount[] | null }>('/me/blocks');
  return Array.isArray(body?.items) ? body.items : [];
}

/** DELETE /me/blocks/{user_id}: 204 with no body. */
export async function unblock(userId: string): Promise<void> {
  await api.delete<void>(`/me/blocks/${encodeURIComponent(userId)}`);
}
