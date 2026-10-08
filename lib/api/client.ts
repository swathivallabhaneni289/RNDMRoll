import { getAccessToken, isDefinitiveAuthFailure, refreshSession } from '@/lib/session/store';
import type { ApiErrorBody, ApiErrorCode } from '@/lib/api/types';

/**
 * Typed fetch client every mobile screen calls through. Attaches the bearer token,
 * converts non-2xx responses into a typed ApiError, and single-flights a refresh+retry
 * on a 401 token_expired response so a burst of concurrent expired-token requests
 * produces exactly one refresh (see the module-level `refreshPromise` below).
 *
 * This module never reads or writes the OS keychain directly. It reaches session
 * state only through `getAccessToken` (an in-memory accessor) and the
 * `refreshSession`/`signOut` delegates that lib/session/store.ts exports;
 * lib/session/store.ts is the only module in the repo permitted to import the
 * secure-storage package.
 */

const BASE_URL = process.env.EXPO_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
const API_PREFIX = '/v1';

/**
 * UI-SPEC's Copywriting Contract error strings, verbatim, for the codes it declares.
 * Any other code falls through to a generic message the calling screen is expected
 * to override with screen-specific copy.
 */
const USER_MESSAGES: Partial<Record<ApiErrorCode, string>> = {
  network_unavailable: "Couldn't connect. Check your connection and try again.",
  email_taken: "That email's already registered. Log in instead.",
  rate_limited: 'Too many tries. Wait a minute and try again.',
  invalid_credentials: "That email or password isn't right.",
  photo_rejected: 'Use a JPG or PNG under 5 MB.',
  photo_upload_failed: 'Photo upload failed. Please try again.',
};

/** A silent API host must not hold the splash or a sheet for a minute: abort and report offline. */
const REQUEST_TIMEOUT_MS = 10_000;

const GENERIC_FALLBACK_MESSAGE = 'Something went wrong. Please try again.';

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  suggestions?: string[];
  /** The form field a 400 validation_failed names, when the server says which. */
  field?: string;

  constructor(status: number, code: ApiErrorCode, message?: string, suggestions?: string[], field?: string) {
    super(message ?? code);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.suggestions = suggestions;
    this.field = field;
  }

  get userMessage(): string {
    return USER_MESSAGES[this.code] ?? GENERIC_FALLBACK_MESSAGE;
  }
}

interface RequestOptions {
  /** Set false to omit the Authorization header (login, signup, refresh itself, etc). */
  auth?: boolean;
  /** Internal: set true on the single retry attempt after a refresh, to prevent looping. */
  retry?: boolean;
}

/**
 * Single-flight guard: several requests failing with an expired token concurrently
 * must await one shared refresh rather than each firing its own, which would rotate
 * the server-side refresh token repeatedly and strand the other callers.
 */
let refreshPromise: Promise<void> | null = null;

async function request<T>(
  method: 'GET' | 'POST' | 'PATCH',
  path: string,
  body: unknown,
  opts: RequestOptions
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (opts.auth !== false) {
    const token = getAccessToken();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  let response: Response;
  try {
    response = await fetch(`${BASE_URL}${API_PREFIX}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    });
  } catch {
    throw new ApiError(0, 'network_unavailable');
  } finally {
    clearTimeout(timer);
  }

  if (response.ok) {
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  let errorBody: ApiErrorBody;
  try {
    errorBody = (await response.json()) as ApiErrorBody;
  } catch {
    throw new ApiError(response.status, 'server_error');
  }

  const code = errorBody.error as ApiErrorCode;
  const apiError = new ApiError(response.status, code, errorBody.message, errorBody.suggestions, errorBody.field);

  // /auth/refresh answers an expired REFRESH token with the same 401 token_expired. Taking
  // this branch for it (or for any unauthenticated call) would start a second refresh whose
  // own 401 awaits the first, so the two wait on each other forever and restore() never
  // settles. Those 401s must surface as the ApiError for the store to sign out on.
  const isRefreshable = opts.auth !== false && path !== '/auth/refresh';
  if (response.status === 401 && code === 'token_expired' && !opts.retry && isRefreshable) {
    if (!refreshPromise) {
      refreshPromise = refreshSession().finally(() => {
        refreshPromise = null;
      });
    }
    try {
      await refreshPromise;
    } catch (refreshErr) {
      // The store has already signed out on a definitive rejection. Any other refresh
      // failure (offline, 5xx) must surface as itself and must not end the session.
      throw isDefinitiveAuthFailure(refreshErr) ? apiError : refreshErr;
    }
    return request<T>(method, path, body, { ...opts, retry: true });
  }

  throw apiError;
}

export const api = {
  get<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    return request<T>('GET', path, undefined, opts);
  },
  post<T>(path: string, body: unknown, opts: RequestOptions = {}): Promise<T> {
    return request<T>('POST', path, body, opts);
  },
  patch<T>(path: string, body: unknown, opts: RequestOptions = {}): Promise<T> {
    return request<T>('PATCH', path, body, opts);
  },
};
