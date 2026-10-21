/**
 * Where a tap on a phone push goes (the server's internal/push sets the data). Pure, so the check
 * script can run it under Node. The four kinds are the bell's: a comment on your post, a reply to
 * your comment, a follow request to you, and your follow request accepted.
 */

export type PushTarget =
  | { pathname: '/(app)/today/entry'; params: { id: string } }
  | { pathname: '/(app)/today/post'; params: { id: string } }
  | { pathname: '/(app)/today/person'; params: { id: string } }
  | { pathname: '/(app)/today/follow-requests' };

export const PUSH_KINDS = ['comment', 'reply', 'follow_request', 'follow_accepted'] as const;

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

/**
 * The page a push opens, or null for anything that is not one of the four pushes (the 8 PM
 * reminder has its own tap handling and carries no `kind`).
 *  - comment: your own entry page.
 *  - reply: your entry page when the post is yours (`own` is "true"), else the post page.
 *  - follow_request: the follow requests page.
 *  - follow_accepted: the person's profile.
 */
export function pushTarget(data: unknown): PushTarget | null {
  if (data === null || typeof data !== 'object') return null;
  const d = data as Record<string, unknown>;
  const kind = text(d.kind);
  const entryId = text(d.entry_id);
  const actorId = text(d.actor_id);
  switch (kind) {
    case 'comment':
      return entryId === '' ? null : { pathname: '/(app)/today/entry', params: { id: entryId } };
    case 'reply':
      if (entryId === '') return null;
      return text(d.own) === 'true'
        ? { pathname: '/(app)/today/entry', params: { id: entryId } }
        : { pathname: '/(app)/today/post', params: { id: entryId } };
    case 'follow_request':
      return { pathname: '/(app)/today/follow-requests' };
    case 'follow_accepted':
      return actorId === '' ? null : { pathname: '/(app)/today/person', params: { id: actorId } };
    default:
      return null;
  }
}
