import { api, ApiError } from '@/lib/api/client';
import { readPerson, type Person } from '@/lib/api/social';
import { isReactionName, type ReactionName } from '@/lib/entry/reactions';
import { isEntryId } from '@/lib/entry/rules';

/**
 * The Board, a post, reactions and comments (Phase 5, 05-CONTEXT `api_contract` with the
 * follow changes of 05-FOLLOW-OVERRIDES section 6). Every field the app reads is read here, once,
 * so a `null` list or a missing key from the server never reaches a screen. The phone prints the
 * server's strings as sent (`posted_local_time`, `at_label`) and works out no date or time.
 *
 * The paths have no `/v1`: the client adds it.
 */

export type BoardCategory = { id: string; name: string };

/** One photo of a post, the shape of an entry's photo. */
export type PostPhoto = { id: string; url: string; position: number };

export type Card = {
  entry_id: string;
  author: Person;
  /** The caller's own post. */
  mine: boolean;
  local_date: string;
  category: BoardCategory;
  title: string;
  /** The stored whole number 1 to 10 (7 means 3.5 stars). */
  stars: number;
  /** The author's own reaction. */
  reaction: ReactionName | null;
  photo_url: string;
  late: boolean;
  posted_at: string;
  /** "8:41 PM" in the author's own zone, written by the server. */
  posted_local_time: string;
  /** Null when the caller may not see comments (Phase 6). */
  comment_count: number | null;
  /** The caller's reaction. */
  my_reaction: ReactionName | null;
};

export type Comment = {
  id: string;
  author: Person;
  text: string;
  at: string;
  /** "8:52 PM", "Mon" or "Oct 3" in the viewer's zone, written by the server. */
  at_label: string;
  can_delete: boolean;
  /**
   * Set on a reply: the top-level comment it sits under (one level; the server stores a reply to a
   * reply under that top-level comment). Null for a top-level comment.
   */
  parent_id: string | null;
};

export type Received = { person: Person; reaction: ReactionName; at: string };

export type Post = {
  card: Card;
  /** The author's own comment on the entry. */
  note: string | null;
  /**
   * Null when the caller may not see comments (Phase 6). One flat list: the top-level comments
   * oldest first, each followed directly by its replies oldest first.
   */
  comments: Comment[] | null;
  can_react: boolean;
  /** False when the owner switched commenting off (or the caller may not comment). */
  can_comment: boolean;
  comments_off: boolean;
  /** Who reacted, only on the caller's own post. */
  received: Received[] | null;
  /**
   * The post's photos in order. Not in the written contract: read when the server sends it,
   * otherwise the one cover photo of the card stands in.
   */
  photos: PostPhoto[];
};

export type BoardState = 'no_follows' | 'locked' | 'skipped' | 'unlocked';
export type DayLabel = 'today' | 'last_night';
export type LockedAction = 'log' | 'spin' | 'wait';

export type BoardLocked = {
  posted_count: number;
  names: string[];
  category: string | null;
  action: LockedAction;
};

export type BoardDay = {
  local_date: string;
  label: 'today' | 'yesterday' | 'earlier';
  cards: Card[];
};

export type Board = {
  state: BoardState;
  day_label: DayLabel;
  locked: BoardLocked | null;
  days: BoardDay[];
  next_before: string | null;
};

// ---- reading the wire ----------------------------------------------------------------------------

type Raw = Record<string, unknown>;

function list(value: unknown): Raw[] {
  return Array.isArray(value) ? (value.filter((item) => item !== null && typeof item === 'object') as Raw[]) : [];
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function textOrNull(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null;
}

function int(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function reaction(value: unknown): ReactionName | null {
  return isReactionName(value) ? value : null;
}

const NOBODY: Person = { id: '', name: '', username: '', photo_url: null };

function readCard(raw: Raw): Card | null {
  if (typeof raw.entry_id !== 'string') return null;
  const category = (raw.category ?? {}) as Raw;
  return {
    entry_id: raw.entry_id,
    author: readPerson(raw.author) ?? NOBODY,
    mine: raw.mine === true,
    local_date: text(raw.local_date),
    category: { id: text(category.id), name: text(category.name) },
    title: text(raw.title),
    stars: int(raw.stars),
    reaction: reaction(raw.reaction),
    photo_url: text(raw.photo_url),
    late: raw.late === true,
    posted_at: text(raw.posted_at),
    posted_local_time: text(raw.posted_local_time),
    comment_count: typeof raw.comment_count === 'number' ? raw.comment_count : null,
    my_reaction: reaction(raw.my_reaction),
  };
}

function readComment(raw: Raw): Comment | null {
  if (typeof raw.id !== 'string') return null;
  return {
    id: raw.id,
    author: readPerson(raw.author) ?? NOBODY,
    text: text(raw.text),
    at: text(raw.at),
    at_label: text(raw.at_label),
    can_delete: raw.can_delete === true,
    parent_id: textOrNull(raw.parent_id),
  };
}

function readComments(value: unknown): Comment[] {
  return list(value)
    .map(readComment)
    .filter((item): item is Comment => item !== null);
}

function readPost(raw: Raw): Post {
  const card = readCard((raw.card ?? {}) as Raw);
  if (card === null) throw new ApiError(0, 'server_error');
  const received: Received[] = [];
  for (const item of list(raw.received)) {
    const person = readPerson(item.person);
    const name = reaction(item.reaction);
    if (person !== null && name !== null) received.push({ person, reaction: name, at: text(item.at) });
  }
  const photos: PostPhoto[] = list(raw.photos)
    .filter((photo) => typeof photo.url === 'string' && photo.url !== '')
    .map((photo, index) => ({
      id: typeof photo.id === 'string' && photo.id !== '' ? photo.id : `${card.entry_id}-${index}`,
      url: photo.url as string,
      position: typeof photo.position === 'number' ? photo.position : index,
    }));
  return {
    card,
    note: textOrNull(raw.note),
    comments: Array.isArray(raw.comments) ? readComments(raw.comments) : raw.comments === null ? null : [],
    can_react: raw.can_react === true,
    can_comment: raw.can_comment === true,
    comments_off: raw.comments_off === true,
    received: Array.isArray(raw.received) ? received : null,
    photos: photos.length > 0 ? photos : card.photo_url !== '' ? [{ id: card.entry_id, url: card.photo_url, position: 0 }] : [],
  };
}

function readState(value: unknown): BoardState {
  return value === 'no_follows' || value === 'locked' || value === 'skipped' ? value : 'unlocked';
}

function readLocked(value: unknown): BoardLocked | null {
  if (value === null || typeof value !== 'object') return null;
  const raw = value as Raw;
  const action = raw.action === 'log' || raw.action === 'spin' ? raw.action : 'wait';
  return {
    posted_count: int(raw.posted_count),
    names: Array.isArray(raw.names) ? raw.names.filter((name): name is string => typeof name === 'string') : [],
    category: textOrNull(raw.category),
    action,
  };
}

function readDays(value: unknown): BoardDay[] {
  const days: BoardDay[] = [];
  for (const raw of list(value)) {
    const label = raw.label === 'today' || raw.label === 'yesterday' ? raw.label : 'earlier';
    const cards = list(raw.cards)
      .map(readCard)
      .filter((card): card is Card => card !== null);
    if (cards.length > 0) days.push({ local_date: text(raw.local_date), label, cards });
  }
  return days;
}

// ---- the calls -----------------------------------------------------------------------------------

/** GET /board: the first page, or the days before `before` (a date the server sent as `next_before`). */
export async function getBoard({ before, limit }: { before?: string | null; limit?: number } = {}): Promise<Board> {
  const query: string[] = [];
  if (before) query.push(`before=${encodeURIComponent(before)}`);
  if (limit !== undefined) query.push(`limit=${limit}`);
  const raw = await api.get<Raw>(query.length > 0 ? `/board?${query.join('&')}` : '/board');
  const next = raw?.next_before;
  return {
    state: readState(raw?.state),
    day_label: raw?.day_label === 'last_night' ? 'last_night' : 'today',
    locked: readLocked(raw?.locked),
    days: readDays(raw?.days),
    next_before: typeof next === 'string' && next !== '' ? next : null,
  };
}

/** An id that is not a UUID is answered here as gone and never sent. */
function checkId(id: string): void {
  if (!isEntryId(id)) throw new ApiError(404, 'not_found');
}

/** GET /posts/{entry_id}: a post the caller may see (the owner always may). 404 when not visible. */
export async function getPost(entryId: string): Promise<Post> {
  checkId(entryId);
  const raw = await api.get<Raw>(`/posts/${entryId}`);
  return readPost(raw ?? {});
}

/** POST /posts/{entry_id}/reaction {reaction}: null clears. 409 `own_post` on your own post. */
export async function setReaction(entryId: string, next: ReactionName | null): Promise<ReactionName | null> {
  checkId(entryId);
  const raw = await api.post<Raw>(`/posts/${entryId}/reaction`, { reaction: next });
  return reaction(raw?.my_reaction);
}

/**
 * POST /posts/{entry_id}/comments {text, parent_id?}: 201 with the stored comment; 409
 * `comments_off`. With `parentId` it is a reply to that comment (404 when that comment is gone).
 */
export async function addComment(entryId: string, body: string, parentId: string | null = null): Promise<Comment> {
  checkId(entryId);
  const raw = await api.post<Raw>(
    `/posts/${entryId}/comments`,
    parentId !== null ? { text: body, parent_id: parentId } : { text: body }
  );
  const comment = readComment((raw?.comment ?? {}) as Raw);
  if (comment === null) throw new ApiError(0, 'server_error');
  return comment;
}

/** POST /comments/{id}/delete. A comment that is already gone counts as deleted. */
export async function deleteComment(commentId: string): Promise<void> {
  try {
    await api.post<unknown>(`/comments/${encodeURIComponent(commentId)}/delete`, {});
  } catch (err) {
    if (err instanceof ApiError && err.code === 'not_found') return;
    throw err;
  }
}

/** The line for a failed reaction or comment. No red: the screens print it in ink. */
export function socialErrorMessage(err: unknown, what: 'reaction' | 'comment' | 'reply' | 'other'): string {
  if (err instanceof ApiError) {
    if (err.code === 'network_unavailable') return err.userMessage;
    if (err.code === 'comments_off') return 'Commenting is off.';
    if (what === 'reply' && err.code === 'not_found') return "That comment isn't here any more.";
    if (err.code === 'rate_limited') return err.userMessage;
  }
  if (what === 'reaction') return "Couldn't save that. Please try again.";
  if (what === 'comment') return "Couldn't post your comment. Please try again.";
  if (what === 'reply') return "Couldn't post your reply. Please try again.";
  return 'Something went wrong. Please try again.';
}
