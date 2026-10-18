import { api } from '@/lib/api/client';
import type { ReportReason } from '@/lib/api/person';

/**
 * Report a post or a comment (developer, 2026-10-18). The same reasons as reporting a person
 * (REPORT_REASONS in person.ts). The person is never told; an admin reviews it on the admin page.
 * Your own post or comment, and one you cannot see, answer 404 like a miss.
 */

/** POST /posts/{id}/report {reason}. */
export async function reportPost(entryId: string, reason: ReportReason): Promise<void> {
  await api.post<unknown>(`/posts/${encodeURIComponent(entryId)}/report`, { reason });
}

/** POST /comments/{id}/report {reason}. */
export async function reportComment(commentId: string, reason: ReportReason): Promise<void> {
  await api.post<unknown>(`/comments/${encodeURIComponent(commentId)}/report`, { reason });
}

/** The one line shown after a report is sent. */
export const REPORT_THANKS = "Thanks for letting us know. They won't be told.";
