import { api, ApiError } from '@/lib/api/client';
import type { ReportReason } from '@/lib/api/person';
import { reportOutcome } from '@/lib/board/text';

/**
 * Report a post or a comment (developer, 2026-10-18). The same reasons as reporting a person
 * (REPORT_REASONS in person.ts). The person is never told; an admin reviews it on the admin page.
 * Once reported, the post or comment is gone for the reporter for good, and a comment takes its
 * replies with it (moderation, 2026-11-17). A repeat report answers 404, exactly like your own post
 * or comment and one you cannot see.
 */

/** POST /posts/{id}/report {reason}. */
export async function reportPost(entryId: string, reason: ReportReason): Promise<void> {
  await api.post<unknown>(`/posts/${encodeURIComponent(entryId)}/report`, { reason });
}

/** POST /comments/{id}/report {reason}. */
export async function reportComment(commentId: string, reason: ReportReason): Promise<void> {
  await api.post<unknown>(`/comments/${encodeURIComponent(commentId)}/report`, { reason });
}

const GENERIC = 'Something went wrong. Please try again.';

/**
 * The line after a report of a post, a comment or a person that failed, or null when it counts as
 * sent: a 404 does too (reportOutcome in lib/board/text.ts says why). No connection and too many
 * tries say what the client says; nothing leaves the screen then.
 */
export function reportFailureLine(err: unknown): string | null {
  if (!(err instanceof ApiError)) return GENERIC;
  const outcome = reportOutcome(err.status, err.code);
  if (outcome === 'done') return null;
  return outcome === 'client' ? err.userMessage : GENERIC;
}
