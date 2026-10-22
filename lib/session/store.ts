import * as SecureStore from 'expo-secure-store';
import { AppState } from 'react-native';
import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { api, ApiError, isRetryable } from '@/lib/api/client';
import {
  beginSignIn,
  deleteLeftPushToken,
  logOutFailed,
  registerPushToken,
  unregisterPushToken,
} from '@/lib/notifications/pushToken';
import type { ApiUser, AuthResult } from '@/lib/api/types';

/**
 * SecureStore-backed session persistence and React context. This is the only module
 * in the repository permitted to import expo-secure-store: every credential (access
 * and refresh token) is written to and read from the OS keychain here, and nowhere
 * else, including React state that survives a reload or any logging call.
 */

const ACCESS_TOKEN_KEY = 'rndmroll.access_token';
const REFRESH_TOKEN_KEY = 'rndmroll.refresh_token';
/**
 * Log outs whose /auth/logout could not reach the server (no connection, a server error, too many
 * requests), oldest first: sent again later (drainPendingLogouts).
 */
const PENDING_LOGOUT_KEY = 'rndmroll.pending_logout';
/** The most log outs kept waiting; an older one beyond this is dropped. */
const PENDING_LOGOUT_LIMIT = 10;

const KEYCHAIN_OPTIONS: SecureStore.SecureStoreOptions = {
  keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
};

/**
 * 'offline' means tokens are stored but the server could not be reached to confirm them (no
 * connection, a timeout, a 5xx). The person is NOT signed out: the app shows an offline page
 * with Try again, and tries again by itself when the app returns to the foreground.
 */
export type SessionStatus = 'loading' | 'unauthenticated' | 'authenticated' | 'offline';

// ---------------------------------------------------------------------------
// Module-level accessors lib/api/client.ts imports. These exist so the fetch
// client can attach a bearer token and drive refresh/sign-out without ever
// importing React context or touching SecureStore itself.
// ---------------------------------------------------------------------------

let inMemoryAccessToken: string | null = null;

/**
 * Counts sign ins on this phone (log in, sign-up, a password change or reset). A request, a refresh
 * or a log out remembers the count it began under: once it has moved on, someone signed in since,
 * and whatever it was doing belongs to a session that is over, so it must not touch the new one.
 * It only ever goes up, also when a sign in fails.
 */
let signInEpoch = 0;

/**
 * The sign in still writing its tokens (signIn), until it has finished or failed. It never rejects.
 * The count of sign ins moves on when a sign in begins, but its access token goes in only after
 * both keychain writes, so a request begun meanwhile waits for it (lib/api/client.ts) and then
 * reads the two together.
 */
let signInWriting: Promise<void> | null = null;

/**
 * The session a log out is ending, marked before the log out awaits anything (doSignOut,
 * endSession). A refresh of that session that finishes from then on keeps nothing: no keychain, no
 * access token in memory, no state. refreshToken and accessToken are the tokens such a refresh got
 * from the server (null until then): the log out sends the refresh token to /auth/logout instead of
 * the keychain's, which the server has replaced, and a log out that fails goes on with both.
 */
type Ending = { epoch: number; refreshToken: string | null; accessToken: string | null };
let ending: Ending | null = null;

/** Keychain writes of a refresh still landing (refreshOnce, saveUnsaved). Each never rejects. */
const refreshWrites = new Set<Promise<void>>();

/**
 * The count of sign ins when the session in place now began. It moves on only once a sign in has
 * written its tokens: one whose keychain write fails leaves the session from before in place, so a
 * refresh of that session still on its way is still that session's (refreshOnce).
 */
let sessionEpoch = 0;

/**
 * Tokens of the session in place that the keychain would not keep (it can be locked, as when the
 * phone is). The server has already replaced the refresh token the keychain still holds, and would
 * read that one sent again as reuse and end the session, so every read of the session's refresh
 * token comes here first (sessionRefreshToken), and the keychain write is tried again on each
 * return to the front (saveUnsaved). Null once written, and once a sign in or a log out has put
 * the keychain in step.
 */
let unsaved: { accessToken: string; refreshToken: string } | null = null;

/**
 * The refresh token of the session in place: one the keychain would not keep, else the keychain's
 * (that read itself, so a read takes no longer than before).
 */
function sessionRefreshToken(): Promise<string | null> {
  if (unsaved !== null) return Promise.resolve(unsaved.refreshToken);
  return SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
}

/**
 * True when a and b, two counts of sign ins, name the same session: no sign in between them has
 * written its tokens (one that failed left the session in place), and none is writing now.
 */
function sameSession(a: number, b: number): boolean {
  return a === b || (signInWriting === null && sessionEpoch <= Math.min(a, b));
}

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

export function setAccessToken(token: string | null): void {
  inMemoryAccessToken = token;
}

/** The count of sign ins so far (lib/api/client.ts reads it with the access token). */
export function getSignInEpoch(): number {
  return signInEpoch;
}

/** The sign in still writing its tokens, or null (lib/api/client.ts waits for it). */
export function pendingSignIn(): Promise<void> | null {
  return signInWriting;
}

function isEnding(epoch: number): boolean {
  return ending !== null && ending.epoch === epoch;
}

/** A log out of the session signed in at epoch begins (a second mark keeps what the first has). */
function markEnding(epoch: number): void {
  if (!isEnding(epoch)) ending = { epoch, refreshToken: null, accessToken: null };
}

/** Waits until no refresh is still writing the keychain. Never rejects. */
async function refreshWritesLanded(): Promise<void> {
  while (refreshWrites.size > 0) await Promise.all(refreshWrites);
}

/**
 * Writes again the tokens the keychain would not keep (unsaved). Not while a sign in writes its
 * own, a log out of the session in place runs, or a refresh writes: those put the keychain in step
 * themselves, and a refresh's newer tokens must land last. Its two writes go in one after the
 * other and count as a refresh's, so a sign in, a log out or a refresh begun meanwhile waits for
 * them or lands after them. Never throws.
 */
async function saveUnsaved(): Promise<void> {
  const tokens = unsaved;
  if (tokens === null || signInWriting !== null || refreshWrites.size > 0) return;
  if (ending !== null && ending.epoch >= sessionEpoch) return;
  let landed = () => {};
  const landing = new Promise<void>((resolve) => {
    landed = resolve;
  });
  refreshWrites.add(landing);
  try {
    await Promise.all([
      SecureStore.setItemAsync(REFRESH_TOKEN_KEY, tokens.refreshToken, KEYCHAIN_OPTIONS),
      SecureStore.setItemAsync(ACCESS_TOKEN_KEY, tokens.accessToken, KEYCHAIN_OPTIONS),
    ]);
    if (unsaved === tokens) unsaved = null;
  } catch {
    // Still not kept: the next return to the front tries again.
  } finally {
    refreshWrites.delete(landing);
    landed();
  }
}

// ---------------------------------------------------------------------------
// Log outs left over: /auth/logout could not reach the server, so it goes out
// again later, together with the push token's DELETE when its own could not.
// ---------------------------------------------------------------------------

/**
 * refresh_token: the session to end. access_token and push_token: for the DELETE of the phone's
 * push token, when that DELETE could not reach the server either (null otherwise).
 */
type PendingLogout = {
  refresh_token: string;
  access_token: string | null;
  push_token: string | null;
};

/** Changes to the kept list go one after another, so none is lost to another running at once. */
let pendingLogoutsQueue: Promise<void> = Promise.resolve();

/**
 * False once the kept list is known to be empty in this run, so a request that works reads nothing
 * for it (requestWorked). True at launch, and again whenever a log out is kept.
 */
let mayHavePendingLogouts = true;
/** Counts log outs kept for later, so a drain that read the list before one was kept never clears the flag above. */
let logoutsKept = 0;

function parsePendingLogouts(text: string | null): PendingLogout[] {
  if (text === null) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  const list: PendingLogout[] = [];
  for (const item of parsed) {
    if (typeof item !== 'object' || item === null) continue;
    const { refresh_token, access_token, push_token } = item as Record<string, unknown>;
    if (typeof refresh_token !== 'string' || refresh_token === '') continue;
    list.push({
      refresh_token,
      access_token: typeof access_token === 'string' ? access_token : null,
      push_token: typeof push_token === 'string' ? push_token : null,
    });
  }
  return list;
}

/**
 * Reads the kept list and, when change gives back a new one, writes that back, in turn with every
 * other change. Resolves to the list as it was read. Never throws: a list that cannot be read or
 * written is left as it is (read as empty).
 */
function changePendingLogouts(
  change: (list: PendingLogout[]) => PendingLogout[] | null
): Promise<PendingLogout[]> {
  let read: PendingLogout[] = [];
  const run = pendingLogoutsQueue.then(async () => {
    try {
      read = parsePendingLogouts(await SecureStore.getItemAsync(PENDING_LOGOUT_KEY));
      const list = change(read);
      if (list === null) return;
      if (list.length === 0) await SecureStore.deleteItemAsync(PENDING_LOGOUT_KEY);
      else {
        const kept = JSON.stringify(list.slice(-PENDING_LOGOUT_LIMIT));
        await SecureStore.setItemAsync(PENDING_LOGOUT_KEY, kept, KEYCHAIN_OPTIONS);
      }
    } catch {
      // Best effort: the next change or launch tries again.
    }
  });
  pendingLogoutsQueue = run;
  return run.then(() => read);
}

const readPendingLogouts = () => changePendingLogouts(() => null);

const forgetPendingLogout = (refreshToken: string) =>
  changePendingLogouts((list) =>
    list.some((entry) => entry.refresh_token === refreshToken)
      ? list.filter((entry) => entry.refresh_token !== refreshToken)
      : null
  );

/**
 * Signed out on this phone, with no sign in on its way. Only then may a log out left over DELETE
 * the phone's push token: the server deletes it by person and token, so for a session signed in now
 * (the same person again, say) it would take that session's token away. A session signed in now
 * sends the token again anyway, and the server moves it to that session.
 */
async function signedOutNow(): Promise<boolean> {
  if (signInWriting !== null || inMemoryAccessToken !== null) return false;
  const stored = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
  return stored === null && signInWriting === null && inMemoryAccessToken === null;
}

let draining: Promise<void> | null = null;

/**
 * Sends the log outs left over: for each one, the push token's DELETE when it is known and nobody
 * is signed in, then /auth/logout with that session's refresh token, and it is forgotten once the
 * server has answered it. Still no connection (or a server error, or too many requests): it stays
 * for the next try, and so do the ones after it. Run at launch, on each return to the front, after
 * each sign in or restore that worked (a sign in shows the connection is back), and after a request
 * that worked, at most every 30 seconds (requestWorked). Never throws, never shows anything, and
 * never touches the session signed in now: it only names old sessions' tokens.
 */
function drainPendingLogouts(): Promise<void> {
  if (draining !== null) return draining;
  const run: Promise<void> = (async () => {
    for (const entry of await readPendingLogouts()) {
      if (entry.push_token !== null && entry.access_token !== null && (await signedOutNow())) {
        await deleteLeftPushToken(entry.push_token, entry.access_token);
      }
      try {
        await api.post('/auth/logout', { refresh_token: entry.refresh_token }, { auth: false });
      } catch (err) {
        if (isRetryable(err)) return;
      }
      await forgetPendingLogout(entry.refresh_token);
    }
    // All sent: a request that works sends nothing more, unless a log out was kept meanwhile (also
    // one kept after this read was queued). Not known when the list cannot be read (the flag stays).
    const keptBefore = logoutsKept;
    await changePendingLogouts((list) => {
      mayHavePendingLogouts = list.length > 0 || logoutsKept !== keptBefore;
      return null;
    });
  })()
    .catch(() => undefined)
    .finally(() => {
      if (draining === run) draining = null;
    });
  draining = run;
  return run;
}

/** A request that works sets off the log outs left over at most this often (requestWorked). */
const REQUEST_DRAIN_EVERY_MS = 30_000;
let lastRequestDrainAt = -Infinity;

/**
 * A request got an answer that worked (lib/api/client.ts calls it): the connection is back, so the
 * log outs left over go out now, as on a return to the front. Only when some may be waiting, and at
 * most every 30 seconds, so a server that keeps failing /auth/logout does not get one with every
 * request. Never throws, never waited on.
 */
export function requestWorked(): void {
  if (!mayHavePendingLogouts || draining !== null) return;
  const now = Date.now();
  if (now - lastRequestDrainAt < REQUEST_DRAIN_EVERY_MS) return;
  lastRequestDrainAt = now;
  void drainPendingLogouts();
}

interface SessionHandlers {
  refreshSession: (epoch: number) => Promise<string | null>;
  signOut: () => Promise<void>;
}

let registeredHandlers: SessionHandlers | null = null;

/**
 * Called once by SessionProvider on mount, registering the live, state-updating
 * implementations of refresh/sign-out so the module-level functions below (which
 * lib/api/client.ts calls) can drive them without importing React context.
 */
export function registerSessionHandlers(handlers: SessionHandlers): void {
  registeredHandlers = handlers;
}

/**
 * Delegate lib/api/client.ts calls on a 401, for the session signed in at epoch (the count its
 * request was sent under; the session signed in now when left out). Resolves to the new access
 * token to send the request again with, or null when this refresh got none for that session. Throws
 * if called before SessionProvider has mounted, which should never happen in practice since it
 * wraps the whole app.
 */
export async function refreshSession(epoch: number = signInEpoch): Promise<string | null> {
  if (!registeredHandlers) {
    throw new Error('refreshSession called before SessionProvider mounted');
  }
  return registeredHandlers.refreshSession(epoch);
}

/**
 * True only when the server answered and rejected the refresh token (401/403, or the
 * invalid/expired token codes /auth/refresh returns). Network failures, timeouts,
 * 5xx and 429 are not definitive: the token may still be good, so it must be kept.
 */
export function isDefinitiveAuthFailure(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false;
  return (
    err.status === 401 ||
    err.status === 403 ||
    err.code === 'token_invalid' ||
    err.code === 'token_expired'
  );
}

export async function signOut(): Promise<void> {
  if (!registeredHandlers) {
    throw new Error('signOut called before SessionProvider mounted');
  }
  return registeredHandlers.signOut();
}

// ---------------------------------------------------------------------------
// React context
// ---------------------------------------------------------------------------

interface SessionContextValue {
  status: SessionStatus;
  user: ApiUser | null;
  /**
   * True from a finished sign-up until the person finishes or skips the photo-and-bio page.
   * In memory only: a relaunch never shows it.
   */
  extrasPending: boolean;
  signIn: (result: AuthResult, options?: { extras?: boolean }) => Promise<void>;
  finishExtras: () => void;
  signOut: () => Promise<void>;
  refreshSession: () => Promise<void>;
  reloadUser: () => Promise<void>;
  /** Tries the launch restore again (the offline page's Try again). */
  retryRestore: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | undefined>(undefined);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>('loading');
  const [user, setUser] = useState<ApiUser | null>(null);
  const [extrasPending, setExtrasPending] = useState(false);

  // Refs, not state: the foreground retry reads these from an AppState callback that
  // must not re-subscribe on every change.
  const statusRef = useRef<SessionStatus>('loading');
  const mountedRef = useRef(false);
  const restoringRef = useRef(false);
  // Counts sign-ins and sign-outs in flight so a restore never races them.
  const authBusyRef = useRef(0);
  // The person's log out running now (doSignOut). A sign in forgets it, so the new session's log
  // out never joins one from before.
  const signOutInFlightRef = useRef<Promise<void> | null>(null);

  const signIn = useCallback((result: AuthResult, options?: { extras?: boolean }) => {
    authBusyRef.current += 1;
    // The new session begins here, before anything is awaited: the count of sign ins and the push
    // token's own count (beginSignIn) move on together, so a request, a refresh or a log out from
    // before sees at once that someone signed in since it began. The access token requests carry
    // goes in only once both keychain writes have landed; a request begun meanwhile waits for this
    // sign in (signInWriting) and then carries the new token.
    signInEpoch += 1;
    const epoch = signInEpoch;
    signOutInFlightRef.current = null;
    const undoBegin = beginSignIn(result.user.id);
    let finished = () => {};
    const writing = new Promise<void>((resolve) => {
      finished = resolve;
    });
    signInWriting = writing;
    return (async () => {
      let wroteAccessToken = false;
      try {
        // A refresh's keychain write still landing goes first, so it never lands over these.
        await refreshWritesLanded();
        // A sign in after this one (a second tap) has the last word on the keychain and the state.
        if (signInEpoch !== epoch) return;
        await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
        wroteAccessToken = true;
        if (signInEpoch !== epoch) return;
        await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
        if (signInEpoch !== epoch) return;
        // The new session is in place, and the keychain holds its tokens.
        sessionEpoch = epoch;
        unsaved = null;
        setAccessToken(result.access_token);
        // The push token goes to the server again now, with the new access token, even for the
        // same person: a password change or reset makes the server forget every token of the
        // account, this phone's too.
        void registerPushToken(result.user.id);
        // Set before the status flips, so the landing page never draws once as the edit page.
        // A plain log in clears it.
        setExtrasPending(options?.extras === true);
        setUser(result.user);
        setStatus('authenticated');
        // The connection works: a log out left over from before goes out now.
        void drainPendingLogouts();
      } catch (err) {
        // A keychain write failed: nothing of the new session stays. The push token's state is
        // as it was, the keychain gets the access token back that memory still holds (the
        // session from before, or none), and the state was never set. The count of sign ins
        // stays moved on: whatever was running for the session from before has stopped.
        if (signInEpoch === epoch) {
          undoBegin();
          if (wroteAccessToken) {
            const previous = inMemoryAccessToken;
            try {
              if (previous !== null) {
                await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, previous, KEYCHAIN_OPTIONS);
              } else {
                await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
              }
            } catch {
              // Best effort: the keychain's access token is only ever written, never read back.
            }
          }
        }
        throw err;
      } finally {
        if (signInWriting === writing) signInWriting = null;
        finished();
        authBusyRef.current -= 1;
      }
    })();
  }, []);

  const finishExtras = useCallback(() => setExtrasPending(false), []);

  // One refresh at a time for each session. The server rotates the refresh token on every call, so
  // a second refresh with the same stored token is answered 401 and would be read as "signed out".
  // The launch restore and the client's own 401 path can both ask at once: they share one call.
  const refreshInFlightRef = useRef<{ epoch: number; done: Promise<string | null> } | null>(null);

  // Ends the session signed in at epoch (the count of sign ins when the log out or the failed
  // refresh began, never a later one). fromRefresh is true only when a failed refresh is what signs
  // out. Its refresh token is read first, before the push token is let go, so it is this session's
  // own even if someone signs in later on: /auth/logout then still ends it on the server, which
  // never touches the new session. A sign in since then (after a failed refresh signed out, say)
  // started a session this one must not touch otherwise: it stops at once if one happened already,
  // and after each wait before the keychain or the state. When /auth/logout cannot reach the
  // server, its refresh token is kept to be sent later (drainPendingLogouts). When the keychain
  // cannot be cleared, the person stays signed in, as if Log out had never been tapped.
  const endSession = useCallback(async (fromRefresh: boolean, epoch: number) => {
    const signedInSince = () => signInEpoch !== epoch;
    if (signedInSince()) return;
    markEnding(epoch);
    authBusyRef.current += 1;
    let kept: PendingLogout | null = null;
    try {
      // For the push token's DELETE, if it must be sent later: read before the push token goes.
      const accessToken = inMemoryAccessToken;
      let storedRefreshToken: string | null = null;
      try {
        // A refresh still writing its tokens lands first, so this read has them and nothing lands
        // after the keychain is cleared.
        await refreshWritesLanded();
        storedRefreshToken = await sessionRefreshToken();
      } catch {
        // Not readable: nothing to send to /auth/logout.
      }
      // A sign in while the keychain was read: the token read may be the new session's.
      if (signedInSince()) return;
      // This phone stops getting pushes for this account. Never throws; it comes first, while the
      // access token still works. A log out by the person may refresh an expired access token on
      // the way (also while a routine refresh runs, which never waits on it). No refresh when a
      // failed refresh is what signs out: it would wait on itself. It gives back the token when
      // its DELETE could not reach the server.
      const unsent = await unregisterPushToken(!fromRefresh);
      // A refresh that finished during this log out kept the token it got out of the keychain: the
      // server has replaced the one read above with it.
      const rotated = ending !== null && ending.epoch === epoch ? ending.refreshToken : null;
      const refreshToken = rotated ?? storedRefreshToken;
      if (refreshToken) {
        try {
          await api.post('/auth/logout', { refresh_token: refreshToken }, { auth: false });
        } catch (err) {
          // No connection (or the server could not end it now): sent again later. Not after a
          // failed refresh: the server has already refused that token.
          if (!fromRefresh && isRetryable(err)) {
            kept = { refresh_token: refreshToken, access_token: accessToken, push_token: unsent };
            const entry = kept;
            mayHavePendingLogouts = true;
            logoutsKept += 1;
            await changePendingLogouts((list) => [...list, entry]);
          }
        }
      }
      if (signedInSince()) return;
      try {
        await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
        if (signedInSince()) return;
        await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
      } catch (err) {
        // The keychain could not be cleared: still signed in. This session goes on as before, so
        // its refreshes keep their tokens again, the push token is sent again, and its log out is
        // not sent later. A refresh that finished during this log out kept its tokens out of the
        // keychain and memory, and the server has replaced the token the keychain holds with them:
        // the session goes on with those (unsaved until the keychain takes them).
        if (!signedInSince()) {
          const got = isEnding(epoch) ? ending : null;
          if (isEnding(epoch)) ending = null;
          if (got !== null && got.refreshToken !== null && got.accessToken !== null) {
            unsaved = { accessToken: got.accessToken, refreshToken: got.refreshToken };
            setAccessToken(got.accessToken);
            void saveUnsaved();
          }
          logOutFailed();
          if (kept !== null) await forgetPendingLogout(kept.refresh_token);
        }
        throw err;
      }
      if (signedInSince()) return;
      unsaved = null;
      setAccessToken(null);
      setExtrasPending(false);
      setUser(null);
      setStatus('unauthenticated');
    } finally {
      authBusyRef.current -= 1;
    }
  }, []);

  // A log out by the person, and the client's when an admin disabled the account. One at a time:
  // a second one while the first runs (Settings' Log out confirmed twice) shares it. Run again, it
  // would find the push token already taken by the first, finish at once and clear the access
  // token that the first one's DELETE of the push token still needs. A failed refresh that signs
  // out (endSession(true)) never joins it: it must never wait. Neither does a log out after a new
  // sign in (signIn forgets the old one), and the old one's end then leaves the new one in place.
  // Begun while a sign in still writes its tokens, it ends that new session once it is in place (or
  // the one from before, when that sign in failed). The session is marked as ending at once, before
  // anything is awaited, so a refresh of it that finishes from now on keeps nothing.
  const doSignOut = useCallback((): Promise<void> => {
    if (signOutInFlightRef.current) return signOutInFlightRef.current;
    const epoch = signInEpoch;
    markEnding(epoch);
    const pendingSignIn = signInWriting;
    const attempt: Promise<void> = (async () => {
      if (pendingSignIn) await pendingSignIn;
      await endSession(false, epoch);
    })().finally(() => {
      if (signOutInFlightRef.current === attempt) signOutInFlightRef.current = null;
    });
    signOutInFlightRef.current = attempt;
    return attempt;
  }, [endSession]);

  // Refreshes the session signed in at epoch, the count its caller read (a request's own, or the
  // restore's), never a later one. A sign in since then has tokens of its own: this refresh then
  // never signs out (endSession checks this count) and never writes over them. A log out of this
  // session begun meanwhile (ending) gets the new tokens kept out of the keychain and memory: only
  // the requests that waited for this refresh use the access token (the log out's DELETE among
  // them), and the log out sends the refresh token to /auth/logout. Resolves to the access token to
  // send again with, or null when there is none for this session.
  const refreshOnce = useCallback(async (begun: number): Promise<string | null> => {
    // The count this refresh is for. It moves on to a later sign in's count only when that sign in
    // failed, as the session it is for is then still in place (below).
    let epoch = begun;
    const signedInSince = () => signInEpoch !== epoch;
    const endingNow = () => isEnding(epoch);
    // During a log out, a refresh before this one may have kept its token out of the keychain.
    const rotated = ending !== null && ending.epoch === epoch ? ending.refreshToken : null;
    const storedRefreshToken = rotated ?? (await sessionRefreshToken());
    // A sign in while the keychain was read: the token read may already be the new session's, and
    // refreshing it here would rotate it on the server while the keychain keeps the old value.
    if (signedInSince()) return null;
    if (!storedRefreshToken) {
      // Nothing to refresh with: the device is already signed out (or a log out of this session is
      // clearing it, and sets the state itself), so clear state.
      if (!endingNow()) await endSession(true, epoch);
      throw new ApiError(401, 'token_invalid', 'no refresh token stored');
    }
    let result: AuthResult;
    try {
      // The server rotates the refresh token on every call, so both returned tokens
      // must be persisted, not only the access token, or the next launch is stranded.
      result = await api.post<AuthResult>(
        '/auth/refresh',
        { refresh_token: storedRefreshToken },
        { auth: false }
      );
    } catch (err) {
      // Only a definitive rejection ends the session, and only while the token we tried is
      // still the stored one: a sign-in that finished while this request was in flight
      // replaced it, and its fresh tokens must survive. Any other failure leaves the tokens
      // in the keychain so a later attempt or launch can still succeed. A log out of this
      // session that is running ends it itself.
      if (isDefinitiveAuthFailure(err) && !signedInSince() && !endingNow()) {
        let stillStored = true;
        try {
          stillStored = (await sessionRefreshToken()) === storedRefreshToken;
        } catch {
          // Not readable: the count of sign ins says it is still this session's.
        }
        if (stillStored && !signedInSince() && !endingNow()) await endSession(true, epoch);
      }
      throw err;
    }
    const keepForLogOut = () => {
      if (ending !== null && ending.epoch === epoch) {
        ending.refreshToken = result.refresh_token;
        ending.accessToken = result.access_token;
      }
      return result.access_token;
    };
    // A sign in began since this refresh did. One still writing its tokens decides: it is waited
    // for. When it failed (a keychain write), the session this refresh is for is still in place,
    // and the server has replaced its refresh token with the one this refresh got, so that is kept
    // as the session's, under the count now. A sign in that worked has its own tokens.
    if (signedInSince()) {
      while (signInWriting !== null) await signInWriting;
      if (!sameSession(epoch, signInEpoch)) return null;
      if (endingNow()) return keepForLogOut();
      epoch = signInEpoch;
    }
    // Same guards on success, after each wait: never write rotated tokens over a newer sign-in,
    // nor anything once a log out of this session has begun.
    if (endingNow()) return keepForLogOut();
    let current: string | null = storedRefreshToken;
    try {
      current = await sessionRefreshToken();
    } catch {
      // Not readable (a locked keychain): the count of sign ins says it is still this session's.
    }
    if (current !== storedRefreshToken) {
      return !signedInSince() && endingNow() ? keepForLogOut() : null;
    }
    if (signedInSince()) return null;
    if (endingNow()) return keepForLogOut();
    let landed = () => {};
    const landing = new Promise<void>((resolve) => {
      landed = resolve;
    });
    refreshWrites.add(landing);
    let written = true;
    try {
      await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
      if (!signedInSince() && !endingNow()) {
        await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
      }
    } catch {
      written = false;
    } finally {
      refreshWrites.delete(landing);
      landed();
    }
    // The keychain would not keep them (it can be locked, as when the phone is): the server has
    // already replaced the refresh token it holds, so these stay in memory for every later refresh
    // and log out, and the write is tried again on the next return to the front. Set before
    // anything else runs: a sign in or a log out waiting for these writes reads it.
    if (!written) {
      unsaved = { accessToken: result.access_token, refreshToken: result.refresh_token };
    } else if (!signedInSince() && !endingNow()) {
      unsaved = null;
    }
    if (signedInSince()) return null;
    if (endingNow()) return keepForLogOut();
    setAccessToken(result.access_token);
    return result.access_token;
  }, [endSession]);

  const doRefreshSession = useCallback((epoch: number): Promise<string | null> => {
    const running = refreshInFlightRef.current;
    // One refresh at a time for each session, also across a sign in that failed: a second one would
    // send the refresh token the first has on its way, which the server reads as reuse.
    if (running !== null && sameSession(running.epoch, epoch)) return running.done;
    const run = { epoch, done: refreshOnce(epoch) };
    refreshInFlightRef.current = run;
    const clear = () => {
      if (refreshInFlightRef.current === run) refreshInFlightRef.current = null;
    };
    run.done.then(clear, clear);
    return run.done;
  }, [refreshOnce]);

  const refreshCurrentSession = useCallback(
    () => doRefreshSession(signInEpoch).then(() => undefined),
    [doRefreshSession]
  );

  // An answer for a session that is over (someone signed in since, or a log out of it began) is not
  // the person's signed in now, so it is dropped.
  const reloadUser = useCallback(async () => {
    const epoch = signInEpoch;
    const freshUser = await api.get<ApiUser>('/me');
    if (signInEpoch !== epoch || isEnding(epoch) || !mountedRef.current) return;
    setUser(freshUser);
  }, []);

  useEffect(() => {
    registerSessionHandlers({ refreshSession: doRefreshSession, signOut: doSignOut });
  }, [doRefreshSession, doSignOut]);

  useEffect(() => {
    statusRef.current = status;
  }, [status]);

  // The relaunch half of ACCT-01's stay-logged-in chain, also re-run when the app returns
  // to the foreground. Absence of a refresh token means a genuinely signed-out device;
  // presence means attempt to restore. Any failure leaves this session unauthenticated,
  // but only a definitive rejection (expired/revoked refresh token) deletes the tokens, so
  // after a network error the foreground retry can still succeed, and after a definitive
  // one the missing token stops further retries. The count of sign ins is read before the
  // keychain is: a sign in meanwhile has its own tokens and status, and the restore then stops
  // after each wait, refreshes only for its own count, and sets nothing.
  const restore = useCallback(async () => {
    if (restoringRef.current || authBusyRef.current > 0) return;
    restoringRef.current = true;
    const epoch = signInEpoch;
    const signedInSince = () => signInEpoch !== epoch;
    try {
      let storedRefreshToken: string | null;
      try {
        storedRefreshToken = await sessionRefreshToken();
      } catch {
        // The keychain could not be read (it is locked while the phone is, say): not known to be
        // signed out, so the offline page shows rather than the splash for good, and a return to
        // the front tries again. Someone already shown as signed out stays so.
        if (!mountedRef.current || signedInSince()) return;
        if (statusRef.current === 'loading' || statusRef.current === 'offline') {
          setStatus('offline');
        }
        return;
      }
      if (signedInSince()) return;
      if (!storedRefreshToken) {
        if (mountedRef.current) setStatus('unauthenticated');
        return;
      }
      try {
        await doRefreshSession(epoch);
        // A sign in meanwhile has set its own person and status, and a log out begun meanwhile
        // sets its own: an answer from before either is not the person's signed in now.
        if (signedInSince() || isEnding(epoch)) return;
        const freshUser = await api.get<ApiUser>('/me');
        if (!mountedRef.current || signedInSince() || isEnding(epoch)) return;
        setUser(freshUser);
        setStatus('authenticated');
        // The connection works: a log out left over from before goes out now.
        void drainPendingLogouts();
      } catch (err) {
        if (!mountedRef.current || signedInSince()) return;
        // Only a definitive rejection (the tokens are gone by now) means signed out. Any other
        // failure keeps the tokens, so the person sees the offline page, not Welcome. A keychain
        // that cannot be read counts as still holding them.
        let stillHasTokens = true;
        try {
          stillHasTokens = (await sessionRefreshToken()) !== null;
        } catch {
          // Not readable: as above.
        }
        // A sign in while the keychain was read has set its own status.
        if (!mountedRef.current || signedInSince()) return;
        setStatus(!isDefinitiveAuthFailure(err) && stillHasTokens ? 'offline' : 'unauthenticated');
      }
    } finally {
      restoringRef.current = false;
    }
  }, [doRefreshSession]);

  useEffect(() => {
    mountedRef.current = true;
    restore();
    // A log out from before that could not reach the server goes out now if it can.
    void drainPendingLogouts();

    // System sheets (permission prompts, the photo picker) only move the app to inactive,
    // so a retry waits for a real return from the background. A start in the background (iOS can
    // start the app before it is opened, while the phone is locked) counts as one, so the first
    // time it comes to the front, a restore that found the keychain locked runs again.
    let wasBackgrounded = AppState.currentState === 'background';
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') wasBackgrounded = true;
      if (next === 'active' && wasBackgrounded) {
        wasBackgrounded = false;
        // Tokens a refresh got that the keychain would not keep: written now, while it is unlocked.
        void saveUnsaved();
        if (statusRef.current === 'unauthenticated' || statusRef.current === 'offline') restore();
        void drainPendingLogouts();
      }
    });

    return () => {
      mountedRef.current = false;
      subscription.remove();
    };
    // restore is stable (its deps are stable callbacks), so this runs once per mount.
  }, [restore]);

  const value: SessionContextValue = {
    status,
    user,
    extrasPending,
    signIn,
    finishExtras,
    signOut: doSignOut,
    refreshSession: refreshCurrentSession,
    reloadUser,
    retryRestore: restore,
  };

  return createElement(SessionContext.Provider, { value }, children);
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error('useSession must be used within a SessionProvider');
  }
  return ctx;
}
