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
import { api, ApiError } from '@/lib/api/client';
import { resendPushToken, unregisterPushToken } from '@/lib/notifications/pushToken';
import type { ApiUser, AuthResult } from '@/lib/api/types';

/**
 * SecureStore-backed session persistence and React context. This is the only module
 * in the repository permitted to import expo-secure-store: every credential (access
 * and refresh token) is written to and read from the OS keychain here, and nowhere
 * else, including React state that survives a reload or any logging call.
 */

const ACCESS_TOKEN_KEY = 'rndmroll.access_token';
const REFRESH_TOKEN_KEY = 'rndmroll.refresh_token';

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

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

export function setAccessToken(token: string | null): void {
  inMemoryAccessToken = token;
}

interface SessionHandlers {
  refreshSession: () => Promise<void>;
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
 * Delegate lib/api/client.ts calls on a 401. Throws if called before SessionProvider
 * has mounted, which should never happen in practice since it wraps the whole app.
 */
export async function refreshSession(): Promise<void> {
  if (!registeredHandlers) {
    throw new Error('refreshSession called before SessionProvider mounted');
  }
  return registeredHandlers.refreshSession();
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
  // Bumped by signIn so a restore that started earlier cannot overwrite a newer sign-in, and a log
  // out that started earlier stops before it touches the newer session (endSession).
  const signInEpochRef = useRef(0);
  // The person's log out running now (doSignOut). A sign in forgets it, so the new session's log
  // out never joins one from before.
  const signOutInFlightRef = useRef<Promise<void> | null>(null);

  const signIn = useCallback(async (result: AuthResult, options?: { extras?: boolean }) => {
    authBusyRef.current += 1;
    signInEpochRef.current += 1;
    signOutInFlightRef.current = null;
    try {
      await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
      await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
      setAccessToken(result.access_token);
      // The push token goes to the server again now, even for the same person: a password change
      // or reset makes the server forget every token of the account, this phone's too.
      resendPushToken(result.user.id);
      // Set before the status flips, so the landing page never draws once as the edit page.
      // A plain log in clears it.
      setExtrasPending(options?.extras === true);
      setUser(result.user);
      setStatus('authenticated');
    } finally {
      authBusyRef.current -= 1;
    }
  }, []);

  const finishExtras = useCallback(() => setExtrasPending(false), []);

  // One refresh at a time. The server rotates the refresh token on every call, so a second
  // refresh with the same stored token is answered 401 and would be read as "signed out".
  // The launch restore and the client's own 401 path can both ask at once: they share one call.
  const refreshInFlightRef = useRef<Promise<void> | null>(null);

  // Ends the session. fromRefresh is true only when a failed refresh is what signs out.
  // A sign in while it runs (after a failed refresh signed out, say) starts a session this one
  // must not touch: after each wait it stops if one happened, before /auth/logout (which would
  // end the new session on the server), the keychain or the state.
  const endSession = useCallback(async (fromRefresh: boolean) => {
    authBusyRef.current += 1;
    const epoch = signInEpochRef.current;
    const signedInSince = () => signInEpochRef.current !== epoch;
    try {
      // This phone stops getting pushes for this account. Never throws; it comes first, while the
      // access token still works. A log out by the person may refresh an expired access token on
      // the way (also while a routine refresh runs, which never waits on it). No refresh when a
      // failed refresh is what signs out: it would wait on itself.
      await unregisterPushToken(!fromRefresh);
      if (signedInSince()) return;
      try {
        const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
        if (signedInSince()) return;
        if (storedRefreshToken) {
          await api.post('/auth/logout', { refresh_token: storedRefreshToken }, { auth: false });
        }
      } catch {
        // Best-effort: a network failure here must not trap the user in a signed-in shell.
      }
      if (signedInSince()) return;
      await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
      if (signedInSince()) return;
      await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
      if (signedInSince()) return;
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
  const doSignOut = useCallback((): Promise<void> => {
    if (signOutInFlightRef.current) return signOutInFlightRef.current;
    const attempt: Promise<void> = endSession(false).finally(() => {
      if (signOutInFlightRef.current === attempt) signOutInFlightRef.current = null;
    });
    signOutInFlightRef.current = attempt;
    return attempt;
  }, [endSession]);

  const refreshOnce = useCallback(async () => {
    const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
    if (!storedRefreshToken) {
      // Nothing to refresh with: the device is already signed out, so clear state.
      await endSession(true);
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
      // in the keychain so a later attempt or launch can still succeed.
      const stillStored = (await SecureStore.getItemAsync(REFRESH_TOKEN_KEY)) === storedRefreshToken;
      if (isDefinitiveAuthFailure(err) && stillStored) {
        await endSession(true);
      }
      throw err;
    }
    // Same guard on success: never write rotated tokens over a newer sign-in.
    if ((await SecureStore.getItemAsync(REFRESH_TOKEN_KEY)) !== storedRefreshToken) return;
    await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
    await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
    setAccessToken(result.access_token);
  }, [endSession]);

  const doRefreshSession = useCallback((): Promise<void> => {
    if (refreshInFlightRef.current) return refreshInFlightRef.current;
    const attempt = refreshOnce().finally(() => {
      refreshInFlightRef.current = null;
    });
    refreshInFlightRef.current = attempt;
    return attempt;
  }, [refreshOnce]);

  const reloadUser = useCallback(async () => {
    const freshUser = await api.get<ApiUser>('/me');
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
  // one the missing token stops further retries.
  const restore = useCallback(async () => {
    if (restoringRef.current || authBusyRef.current > 0) return;
    restoringRef.current = true;
    const epoch = signInEpochRef.current;
    try {
      const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
      if (!storedRefreshToken) {
        if (mountedRef.current && epoch === signInEpochRef.current) setStatus('unauthenticated');
        return;
      }
      try {
        await doRefreshSession();
        await reloadUser();
        if (mountedRef.current) setStatus('authenticated');
      } catch (err) {
        if (mountedRef.current && epoch === signInEpochRef.current) {
          // Only a definitive rejection (the tokens are gone by now) means signed out. Any other
          // failure keeps the tokens, so the person sees the offline page, not Welcome.
          const stillHasTokens = (await SecureStore.getItemAsync(REFRESH_TOKEN_KEY)) !== null;
          setStatus(!isDefinitiveAuthFailure(err) && stillHasTokens ? 'offline' : 'unauthenticated');
        }
      }
    } finally {
      restoringRef.current = false;
    }
  }, [doRefreshSession, reloadUser]);

  useEffect(() => {
    mountedRef.current = true;
    restore();

    // System sheets (permission prompts, the photo picker) only move the app to inactive,
    // so a retry waits for a real return from the background.
    let wasBackgrounded = false;
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') wasBackgrounded = true;
      if (next === 'active' && wasBackgrounded) {
        wasBackgrounded = false;
        if (statusRef.current === 'unauthenticated' || statusRef.current === 'offline') restore();
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
    refreshSession: doRefreshSession,
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
