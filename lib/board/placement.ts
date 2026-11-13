/**
 * Where your posts list opens (components/profile/PostsFeed.tsx). A picture in the Profile grid
 * opens the list at that post, but the list reads its own pages, so a picture from the grid's
 * second page or later is not in the list's first one. The list asks for its next page until the
 * post is in it, and opens there; with no page left (the post was deleted or archived meanwhile) or
 * a page that failed, it opens at the top. It opens only once it knows, so a late page never jumps
 * a list the person is reading. Pure: no React and no imports, so a node script can read it.
 */

/**
 * 'place': the post is in the list, open there. 'load_more': ask for the next page. 'wait': a page
 * is on its way (or the first one is not in). 'give_up': open at the top.
 */
export type PlacementStep = 'place' | 'load_more' | 'wait' | 'give_up';

export function placementStep(a: {
  /** The tapped post's id, '' when the route gave none. */
  startId: string;
  /** The ids of the posts in the list so far. */
  ids: string[];
  /** The first page is in. */
  ready: boolean;
  loadingMore: boolean;
  moreFailed: boolean;
  /**
   * A next page was already asked for while the list held exactly these rows, and none came: the
   * list ends here (the paging hook asks for nothing when it has no next page).
   */
  askedAtThese: boolean;
}): PlacementStep {
  if (a.startId === '') return 'give_up';
  if (a.ids.includes(a.startId)) return 'place';
  if (!a.ready || a.loadingMore) return 'wait';
  if (a.moreFailed || a.askedAtThese) return 'give_up';
  return 'load_more';
}
