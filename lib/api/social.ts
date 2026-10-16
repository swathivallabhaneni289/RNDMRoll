import { api, ApiError } from '@/lib/api/client';

/**
 * Follows (Phase 5, 05-FOLLOW-OVERRIDES section 6). Following is one way, like Instagram, and every
 * follow is a request the other person accepts (no public accounts since 2026-10-16). People are found by exact username.
 * Thin calls over the typed client; every field the app reads is read here, so a server that
 * sends `null` for an empty list (a nil slice in Go) or leaves a key out never breaks a screen.
 *
 * The paths have no `/v1`: the client adds it.
 */

/** A person as the server describes them. */
export type Person = {
  id: string;
  name: string;
  username: string;
  photo_url: string | null;
};

/** The caller toward the other person. */
export type Relation = 'none' | 'requested' | 'following';

/** The answer of a username lookup. */
export type Lookup = {
  person: Person;
  relation: Relation;
  /** The other person follows the caller (the button then reads "Follow back"). */
  follows_you: boolean;
  /** The other account is private: a follow becomes a request. */
  private: boolean;
};

/** A pending follow request, incoming or outgoing. */
export type FollowRequest = {
  request_id: string;
  person: Person;
  at: string;
};

export type Follows = {
  followers: Person[];
  following: Person[];
  incoming: FollowRequest[];
  outgoing: FollowRequest[];
};

/** A server may send `null` for an empty list: read both as an empty list. */
function list<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

/** A person read defensively: any missing text is '', a missing photo is null. Null when there is no id. */
export function readPerson(value: unknown): Person | null {
  if (value === null || typeof value !== 'object') return null;
  const raw = value as Partial<Record<keyof Person, unknown>>;
  if (typeof raw.id !== 'string' || raw.id === '') return null;
  return {
    id: raw.id,
    name: text(raw.name),
    username: text(raw.username),
    photo_url: typeof raw.photo_url === 'string' && raw.photo_url !== '' ? raw.photo_url : null,
  };
}

function readPeople(value: unknown): Person[] {
  return list<unknown>(value)
    .map(readPerson)
    .filter((person): person is Person => person !== null);
}

function readRelation(value: unknown): Relation {
  return value === 'requested' || value === 'following' ? value : 'none';
}

function readRequests(value: unknown): FollowRequest[] {
  const out: FollowRequest[] = [];
  for (const item of list<Record<string, unknown>>(value)) {
    const person = readPerson(item?.person);
    if (person === null || typeof item.request_id !== 'string') continue;
    out.push({ request_id: item.request_id, person, at: text(item.at) });
  }
  return out;
}

/** The first word of the display name, else the username: "Maya" for "Maya Rao". */
export function firstName(person: { name: string | null; username: string | null }): string {
  const word = (person.name ?? '').trim().split(/\s+/)[0] ?? '';
  if (word !== '') return word;
  return person.username ?? '';
}

/**
 * GET /people/lookup?username= (exact, case-insensitive). Null for a miss, for yourself and for
 * anyone a block stands between (the server answers 404 `not_found` for all of them alike).
 */
export async function lookupPerson(username: string): Promise<Lookup | null> {
  try {
    const body = await api.get<Record<string, unknown>>(`/people/lookup?username=${encodeURIComponent(username)}`);
    const person = readPerson(body?.person);
    if (person === null) return null;
    return {
      person,
      relation: readRelation(body.relation),
      follows_you: body.follows_you === true,
      private: body.private === true,
    };
  } catch (err) {
    if (err instanceof ApiError && err.code === 'not_found') return null;
    throw err;
  }
}

/** POST /follows {user_id}: always `requested` now (every account acts as private, 2026-10-16). */
export async function follow(userId: string): Promise<Relation> {
  const body = await api.post<{ relation?: unknown }>('/follows', { user_id: userId });
  return readRelation(body?.relation);
}

/** POST /follows/{user_id}/unfollow: also cancels a pending request. Silent. */
export async function unfollow(userId: string): Promise<Relation> {
  const body = await api.post<{ relation?: unknown }>(`/follows/${encodeURIComponent(userId)}/unfollow`, {});
  return readRelation(body?.relation);
}

/** POST /followers/{user_id}/remove: that person stops following you. Silent. */
export async function removeFollower(userId: string): Promise<void> {
  await api.post<unknown>(`/followers/${encodeURIComponent(userId)}/remove`, {});
}

/** GET /follows: followers, following, and the pending requests both ways. */
export async function getFollows(): Promise<Follows> {
  const body = await api.get<Record<string, unknown>>('/follows');
  return {
    followers: readPeople(body?.followers),
    following: readPeople(body?.following),
    incoming: readRequests(body?.incoming),
    outgoing: readRequests(body?.outgoing),
  };
}

/** POST /follows/requests/{id}/accept. */
export async function acceptRequest(requestId: string): Promise<void> {
  await api.post<unknown>(`/follows/requests/${encodeURIComponent(requestId)}/accept`, {});
}

/** POST /follows/requests/{id}/decline. Silent: the other person sees "Follow" again. */
export async function declineRequest(requestId: string): Promise<void> {
  await api.post<unknown>(`/follows/requests/${encodeURIComponent(requestId)}/decline`, {});
}
