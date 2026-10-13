import { api } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';

/**
 * Account calls of the Settings pages (plan 02.1-02). Thin, like `lib/api/profile.ts`: no
 * reshaping, and nothing here logs or keeps either password.
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
