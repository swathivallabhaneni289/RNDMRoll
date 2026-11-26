import { invites } from '@/lib/people/invite';
import { inviteUsername } from '@/lib/people/inviteLink';
import { getAccessToken } from '@/lib/session/store';

/**
 * Links from outside the app. An invite link's button (rndmroll://u/{username}, the page at
 * /i/{username}) becomes that person's profile on the Board tab, so Back goes to the Board as from
 * any profile. Every other link is left as it came.
 *
 * With no one signed in on this phone (also at a cold start, before the session is known), the
 * link is kept as well, and the inviter's page opens on the Spin page after Log in or sign-up
 * (lib/people/invite.ts). The path given back stays the same: a signed-in link opens as it always
 * has, and a signed-out one still goes nowhere by itself.
 */
export function redirectSystemPath({ path }: { path: string; initial: boolean }): string {
  const username = inviteUsername(path);
  if (username === null) return path;
  if (getAccessToken() === null) invites.note(username);
  return `/board/person?username=${username}`;
}
