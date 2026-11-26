/**
 * An invite link opened while signed out (PRD known issue 1): the inviter's profile opens once the
 * person is in. No React and no app imports (the storage and the clock are handed in), so a plain
 * node script can check it (scripts/check-social.ts); lib/people/invite.ts is the app's copy.
 *
 *  - The link is kept, in memory and on the phone for a day, so a sign-up survives the app being
 *    closed while the person reads the emailed code.
 *  - A Log in makes it ready at once. A sign-up makes it ready only when Find your friends goes on
 *    to the Spin page, after the code page and the photo-and-bio page as before.
 *  - The Spin page opens it once it is ready, then it is forgotten. So is your own link.
 *  - Log out forgets it, and so does any other way out of a session. So does a session restored at
 *    launch: a link that came while signed in opens as it always has.
 */

/** How long a kept invite lasts. */
export const INVITE_KEEP_MS = 24 * 60 * 60 * 1000;

/** Where the phone keeps it. A username and a time, nothing private. */
export const INVITE_KEY = 'rndmroll.invite';

/**
 * The username of an invite link's button (rndmroll://u/{username}, from the page at
 * /i/{username}), lowercased, or null for any other link.
 */
export function inviteUsername(path: string): string | null {
  const match = /^(?:[a-z]+:\/\/)?\/?u\/([A-Za-z0-9_]{3,20})\/?$/.exec(path);
  return match ? match[1].toLowerCase() : null;
}

/** A kept invite: whose link, and when it came (Date.now()). */
export interface Invite {
  username: string;
  at: number;
}

/** The phone's own store, as AsyncStorage gives it. */
export interface InviteStorage {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  removeItem(key: string): Promise<void>;
}

/** The session's status as lib/session/store.ts names it. */
export type InviteSessionStatus = 'loading' | 'unauthenticated' | 'authenticated' | 'offline';

/** A stored invite, or null when it is missing, unreadable or over a day old. */
export function readInvite(text: string | null, now: number): Invite | null {
  if (text === null) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null) return null;
  const { username, at } = parsed as Record<string, unknown>;
  if (typeof username !== 'string' || typeof at !== 'number') return null;
  const name = inviteUsername(`u/${username}`);
  if (name === null || !fresh(at, now)) return null;
  return { username: name, at };
}

function fresh(at: number, now: number): boolean {
  return at <= now && now - at <= INVITE_KEEP_MS;
}

/**
 * True when a change of the session's status ends whatever invite is kept: a session restored at
 * launch, or after the offline page (the link came while signed in), and every way out of a session
 * (Log out, Delete account, a disabled account, a refused refresh). A signed-out launch keeps it,
 * and so does a sign in from the signed-out pages.
 */
export function endsInvite(from: InviteSessionStatus, to: InviteSessionStatus): boolean {
  if (to === 'authenticated') return from === 'loading' || from === 'offline';
  if (to === 'unauthenticated') return from === 'authenticated';
  return false;
}

export interface InviteStore {
  /** Settles once the invite kept from an earlier start is read (or could not be). Never rejects. */
  ready(): Promise<void>;
  /** An invite link came while no one is signed in on this phone: kept, the newest in place of any other. */
  note(username: string): void;
  /** Its page may open on the Spin page now. */
  arm(): void;
  /** Not now after all (the sign in that made it ready failed). */
  disarm(): void;
  /** The invite is over. */
  forget(): void;
  /** The session's status changed: see endsInvite. */
  sessionChanged(from: InviteSessionStatus, to: InviteSessionStatus): void;
  /**
   * The username whose page the Spin page opens now, once, or null. Only once ready; the invite is
   * then forgotten, also when it is your own (`own`, your username) or over a day old.
   */
  take(own: string | null): string | null;
  /** The invite kept now (for the checks). */
  peek(): Invite | null;
}

export function createInviteStore(storage: InviteStorage, now: () => number = Date.now): InviteStore {
  let invite: Invite | null = null;
  let armed = false;
  // A note or a forget since the start: the read of the stored invite then keeps nothing, so a link
  // that came while it was on its way is never put back by an older one.
  let touched = false;
  // The phone's writes go one after another, after the read, so the last change is the one kept.
  let queue: Promise<void> = Promise.resolve();

  function write(change: () => Promise<void>): void {
    queue = queue.then(change).catch(() => undefined);
  }

  const loading: Promise<void> = (async () => {
    try {
      const text = await storage.getItem(INVITE_KEY);
      if (touched) return;
      invite = readInvite(text, now());
      // Unreadable or over a day old: gone from the phone too.
      if (invite === null && text !== null) await storage.removeItem(INVITE_KEY);
    } catch {
      // Not readable: as if none was kept.
    }
  })();
  queue = loading;

  // Off the phone too, also when none is in memory: one kept from an earlier start may still be
  // there, as the read of it keeps nothing once this has run.
  function forget(): void {
    touched = true;
    armed = false;
    invite = null;
    write(() => storage.removeItem(INVITE_KEY));
  }

  return {
    ready: () => loading,
    note(username) {
      touched = true;
      const kept: Invite = { username: username.toLowerCase(), at: now() };
      invite = kept;
      write(() => storage.setItem(INVITE_KEY, JSON.stringify(kept)));
    },
    arm() {
      armed = true;
    },
    disarm() {
      armed = false;
    },
    forget,
    sessionChanged(from, to) {
      if (endsInvite(from, to)) forget();
    },
    take(own) {
      if (!armed) return null;
      const kept = invite;
      forget();
      if (kept === null || !fresh(kept.at, now())) return null;
      if (own !== null && kept.username === own.toLowerCase()) return null;
      return kept.username;
    },
    peek: () => invite,
  };
}
