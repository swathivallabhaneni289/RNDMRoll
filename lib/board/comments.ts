/**
 * Where a comment the server saved goes in a post's list, and the count after it (the post page
 * and your own entry page, components/board/usePost.ts). Pure: no React and no imports, so a node
 * script can check it (scripts/check-social.ts).
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
