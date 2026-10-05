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

export type SessionStatus = 'loading' | 'unauthenticated' | 'authenticated';

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
  signIn: (result: AuthResult) => Promise<void>;
  signOut: () => Promise<void>;
  refreshSession: () => Promise<void>;
  reloadUser: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | undefined>(undefined);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>('loading');
  const [user, setUser] = useState<ApiUser | null>(null);

  // Refs, not state: the foreground retry reads these from an AppState callback that
  // must not re-subscribe on every change.
  const statusRef = useRef<SessionStatus>('loading');
  const mountedRef = useRef(false);
  const restoringRef = useRef(false);
  // Counts sign-ins and sign-outs in flight so a restore never races them.
  const authBusyRef = useRef(0);
  // Bumped by signIn so a restore that started earlier cannot overwrite a newer sign-in.
  const signInEpochRef = useRef(0);

  const signIn = useCallback(async (result: AuthResult) => {
    authBusyRef.current += 1;
    signInEpochRef.current += 1;
    try {
      await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
      await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
      setAccessToken(result.access_token);
      setUser(result.user);
      setStatus('authenticated');
    } finally {
      authBusyRef.current -= 1;
    }
  }, []);

  const doSignOut = useCallback(async () => {
    authBusyRef.current += 1;
    try {
      try {
        const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
        if (storedRefreshToken) {
          await api.post('/auth/logout', { refresh_token: storedRefreshToken }, { auth: false });
        }
      } catch {
        // Best-effort: a network failure here must not trap the user in a signed-in shell.
      }
      await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
      await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
      setAccessToken(null);
      setUser(null);
      setStatus('unauthenticated');
    } finally {
      authBusyRef.current -= 1;
    }
  }, []);

  const doRefreshSession = useCallback(async () => {
    const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
    if (!storedRefreshToken) {
      // Nothing to refresh with: the device is already signed out, so clear state.
      await doSignOut();
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
        await doSignOut();
      }
      throw err;
    }
    // Same guard on success: never write rotated tokens over a newer sign-in.
    if ((await SecureStore.getItemAsync(REFRESH_TOKEN_KEY)) !== storedRefreshToken) return;
    await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
    await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
    setAccessToken(result.access_token);
  }, [doSignOut]);

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
      } catch {
        if (mountedRef.current && epoch === signInEpochRef.current) setStatus('unauthenticated');
      }
    } finally {
      restoringRef.current = false;
    }
  }, [doRefreshSession, reloadUser]);

  useEffect(() => {
    mountedRef.current = true;
    restore();

    // System sheets (Apple sign-in, permission prompts, the photo picker) only move the app
    // to inactive, so a retry waits for a real return from the background.
    let wasBackgrounded = false;
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') wasBackgrounded = true;
      if (next === 'active' && wasBackgrounded) {
        wasBackgrounded = false;
        if (statusRef.current === 'unauthenticated') restore();
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
    signIn,
    signOut: doSignOut,
    refreshSession: doRefreshSession,
    reloadUser,
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
