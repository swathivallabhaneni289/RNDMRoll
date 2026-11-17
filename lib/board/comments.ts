/**
 * Where a comment the server saved goes in a post's list, what leaves it, and the count after it
 * (the post page and your own entry page, components/board/usePost.ts). Pure: no React and no
 * imports, so a node script can check it (scripts/check-social.ts).
 */

type Placed = { id: string; parent_id: string | null };

/** `list` with `saved` placed after the last comment of its thread (a reply), or at the end. */
export function placeComment<C extends Placed>(list: C[], saved: C): C[] {
  const rest = list.filter((item) => item.id !== saved.id);
  const top = saved.parent_id;
  if (top === null) return [...rest, saved];
  let at = -1;
  rest.forEach((item, index) => {
    if (item.id === top || item.parent_id === top) at = index;
  });
  if (at === -1) return [...rest, saved];
  return [...rest.slice(0, at + 1), saved, ...rest.slice(at + 1)];
}

/**
 * The comment count once `saved` is placed. A photo comment sent again after a lost answer comes
 * back with the id the first try saved; when a reload already shows it, it is placed once and
 * counted once.
 */
export function countAfterComment(list: Placed[], count: number | null | undefined, saved: Placed): number {
  const before = count ?? list.length;
  return list.some((item) => item.id === saved.id) ? before : before + 1;
}

/**
 * `list` once the comment `id` leaves it: a top-level comment takes its replies, a reply goes
 * alone, and an id not in the list changes nothing. `count` is the comment count after it (the
 * server's, or the list's length when it sent none, less what left, never below 0), and `endReply`
 * says reply mode ends because the comment being replied to (`replyToId`) is no longer in the list.
 * Used after a delete, and after a report: the comment is gone for the reporter from then on.
 */
export function withoutComment<C extends Placed>(
  list: C[],
  count: number | null | undefined,
  id: string,
  replyToId: string | null
): { list: C[]; count: number; endReply: boolean } {
  const left = list.filter((item) => item.id !== id && item.parent_id !== id);
  return {
    list: left,
    count: Math.max(0, (count ?? list.length) - (list.length - left.length)),
    endReply: replyToId !== null && !left.some((item) => item.id === replyToId),
  };
}

/**
 * The comment box's text after a tap on the quick emoji row (components/board/Comments.tsx): the
 * character at the end while it still fits in `max`, else the text as it was. It counts as the
 * field's maxLength does (UTF-16 units), never more than the server's count of characters, so a
 * text it keeps is never too long for the server.
 */
export function withEmoji(before: string, char: string, max: number): string {
  return before.length + char.length > max ? before : before + char;
}

/**
 * The comment box's text when Reply is tapped: "@username " in front, in place of a mention
 * already there, cut to `max` as the field counts. A cut never keeps half of an emoji (a lone
 * surrogate): the server would refuse the comment as invalid text.
 */
export function withReplyPrefix(before: string, username: string, max: number): string {
  const text = `@${username} ${before.replace(/^@\S+\s*/, '')}`.slice(0, max);
  return /[\uD800-\uDBFF]$/.test(text) ? text.slice(0, -1) : text;
}
