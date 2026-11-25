import { api } from '@/lib/api/client';
import type { AccountDeletion, AuthResult } from '@/lib/api/types';

/**
 * Account calls of the Settings pages (plan 02.1-02, the notification and block calls of decision
 * D-16, and Delete account of 2026-10-25). Thin, like `lib/api/profile.ts`: no reshaping beyond
 * filling a missing field, and nothing here logs or keeps a password.
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

/**
 * POST /me/delete (developer, 2026-10-25: "like how instagram has"). The server checks the
 * password, hides the account from everyone at once and ends every session, this phone's too; the
 * account is deleted for good after 30 days unless the person logs back in before then. The answer
 * says when, in words the server wrote (`delete_on_label`, printed as sent). A label the answer
 * leaves out comes back as '' (the page then says "after 30 days" instead).
 *
 * Errors (all surface as `ApiError`): a 400 `validation_failed` whose `field` is `password` and
 * whose `reason` is `incorrect` (wrong password) or `required`, a 429 `rate_limited`, and
 * `network_unavailable`.
 */
export async function deleteAccount(password: string): Promise<AccountDeletion> {
  const body = await api.post<Partial<AccountDeletion> | null>('/me/delete', { password });
  return {
    delete_after: typeof body?.delete_after === 'string' ? body.delete_after : '',
    delete_on_label: typeof body?.delete_on_label === 'string' ? body.delete_on_label : '',
  };
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
  /** Likes on their posts: the Likes switch (the bell's like lines and the like pushes). */
  reactions: boolean;
  /** Comments on their posts. */
  comments: boolean;
  /** Milestones and bonus spins. */
  milestones: boolean;
};

export type NotificationSettingKey = keyof NotificationSettings;

/** The value of a switch the server's answer leaves out. No switch shows until the server answers. */
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

/**
 * POST /me/blocks {user_id}: 200 `{}`. The server removes the follows both ways and any request
 * either way; the blocked person is never told.
 */
export async function blockUser(userId: string): Promise<void> {
  await api.post<unknown>('/me/blocks', { user_id: userId });
}

/** DELETE /me/blocks/{user_id}: 204 with no body. */
export async function unblock(userId: string): Promise<void> {
  await api.delete<void>(`/me/blocks/${encodeURIComponent(userId)}`);
}
