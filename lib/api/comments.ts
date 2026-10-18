import { api } from '@/lib/api/client';

/**
 * Hide a comment (developer, 2026-10-18, like Instagram): the post's owner hides someone's comment
 * on their own post. Everyone else stops seeing it and its replies; its writer still sees it and is
 * never told. A comment carries `can_hide` (you own the post and did not write it) and `hidden`
 * (you hid it). The two fields are read here until lib/api/board.ts's Comment has them.
 */
export type HideFields = { can_hide?: boolean; hidden?: boolean };

export function canHide(comment: object): boolean {
  return (comment as HideFields).can_hide === true;
}

export function isHidden(comment: object): boolean {
  return (comment as HideFields).hidden === true;
}

/** POST /comments/{id}/hide. */
export async function hideComment(commentId: string): Promise<void> {
  await api.post<unknown>(`/comments/${encodeURIComponent(commentId)}/hide`, {});
}

/** POST /comments/{id}/unhide. */
export async function unhideComment(commentId: string): Promise<void> {
  await api.post<unknown>(`/comments/${encodeURIComponent(commentId)}/unhide`, {});
}
