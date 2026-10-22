import {
  getAccessToken,
  getSignInEpoch,
  isDefinitiveAuthFailure,
  pendingSignIn,
  refreshSession,
  requestWorked,
  signOut,
} from '@/lib/session/store';
import type { ApiErrorBody, ApiErrorCode } from '@/lib/api/types';

/**
 * Typed fetch client every mobile screen calls through. Attaches the bearer token,
 * converts non-2xx responses into a typed ApiError, and single-flights a refresh+retry
 * on a 401 token_expired response so a burst of concurrent expired-token requests
 * produces exactly one refresh (see the module-level `refreshing` below).
 *
 * Each request remembers the count of sign ins it started under (getSignInEpoch), read in the
 * same moment as the access token it carries. A request begun while a sign in still writes its
 * tokens waits for that sign in, then reads both (so it never pairs the new count with the old
 * token). A 401 that comes back after a newer sign in belongs to a session that is over: it never
 * refreshes with the new session's tokens, never sends the request again with its access token and
 * never signs it out. It fails quietly as a 401 to its caller instead.
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
 * Your invite link (GET /i/{username} on the server, a small page that opens the app at you). It
 * uses the API's own address until the real web address is set up (launch list).
 */
export function inviteLink(username: string): string {
  return `${BASE_URL.replace(/\/+$/, '')}/i/${encodeURIComponent(username)}`;
}

/**
 * UI-SPEC's Copywriting Contract error strings, verbatim, for the codes it declares.
 * Any other code falls through to a generic message the calling screen is expected
 * to override with screen-specific copy.
 */
const USER_MESSAGES: Partial<Record<ApiErrorCode, string>> = {
  network_unavailable: "Couldn't connect. Check your connection and try again.",
  email_taken: 'That email is already in use.',
  rate_limited: 'Too many tries. Wait a minute and try again.',
  invalid_credentials: "That email, username or password isn't right.",
  // An admin disabled the account (the admin page): said on the log-in screen.
  account_disabled: 'This account has been disabled.',
  photo_rejected: "That file can't be used as a photo. Try another one.",
  photo_upload_failed: 'Photo upload failed. Please try again.',
  // Phase 2 (02-UI-SPEC 4.4). Every other new code uses the generic message below.
  spin_not_open: "Your spin isn't open yet. It opens at 8:00 PM.",
  spin_closed: "Today's spin has closed. The next one opens at 8:00 PM.",
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
  /**
   * The fixed word a 400 wheel_invalid gives for what is wrong with the list. On a Forgot password
   * reset's code (field "code"): "wrong_code", "replaced", "used" or "invalid" (lib/api/types.ts).
   */
  reason?: string;
  /** The row of the sent list a 400 wheel_invalid points at, counted from 0. */
  index?: number;

  constructor(
    status: number,
    code: ApiErrorCode,
    message?: string,
    suggestions?: string[],
    field?: string,
    reason?: string,
    index?: number
  ) {
    super(message ?? code);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.suggestions = suggestions;
    this.field = field;
    this.reason = reason;
    this.index = index;
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
  /**
   * Sends this access token instead of the session's, once: no refresh, no second try and no sign
   * out on its answer, whatever it is (a log out left over from before sends its push token's
   * DELETE so, for a session that is over).
   */
  bearer?: string;
}

/**
 * True when a request may work later as it is: it never reached the server (no connection, or the
 * client's own 10 second limit), or the server answered too many requests or failed itself.
 */
export function isRetryable(err: unknown): boolean {
  if (!(err instanceof ApiError)) return true;
  return err.status === 0 || err.status === 429 || err.status >= 500;
}

/**
 * Single-flight guard: several requests failing with an expired token concurrently
 * must await one shared refresh rather than each firing its own, which would rotate
 * the server-side refresh token repeatedly and strand the other callers. One for each
 * session (the count of sign ins its requests were sent under): a request of a newer
 * session never waits on an older session's refresh.
 */
let refreshing: { epoch: number; done: Promise<string | null> } | null = null;

function sharedRefresh(epoch: number): Promise<string | null> {
  if (refreshing !== null && refreshing.epoch === epoch) return refreshing.done;
  const run = { epoch, done: refreshSession(epoch) };
  refreshing = run;
  const clear = () => {
    if (refreshing === run) refreshing = null;
  };
  run.done.then(clear, clear);
  return run.done;
}

/**
 * What a request started before a newer sign in gets instead of its 401: a plain 401 that
 * says nothing about the account (an old session's "disabled" is not the new session's).
 */
function signedInSinceError(): ApiError {
  return new ApiError(401, 'token_invalid', 'signed in again since this request began');
}

function request<T>(
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
  path: string,
  body: unknown,
  opts: RequestOptions
): Promise<T> {
  if (opts.auth === false) return send<T>(method, path, body, opts, null, null);
  if (opts.bearer !== undefined) return send<T>(method, path, body, opts, opts.bearer, null);
  // A sign in still writing its tokens: this request is the new session's. It waits for that sign
  // in to finish (or fail), then reads the count and the token together, in the same moment.
  const writing = pendingSignIn();
  if (writing !== null) return writing.then(() => request<T>(method, path, body, opts));
  return send<T>(method, path, body, opts, getAccessToken(), getSignInEpoch());
}

/**
 * token: the access token to send (none when null). epoch: the count of sign ins the session's
 * token was read under (its retry keeps it), or null when the request is not the session's (no
 * token, or opts.bearer's own): such a request never refreshes and never signs out.
 */
async function send<T>(
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
  path: string,
  body: unknown,
  opts: RequestOptions,
  token: string | null,
  epoch: number | null
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
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
    // The connection works: a log out left over from before can go out now (never waited on).
    requestWorked();
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
  const apiError = new ApiError(
    response.status,
    code,
    errorBody.message,
    errorBody.suggestions,
    errorBody.field,
    errorBody.reason,
    errorBody.index
  );

  // Only the session's own requests sign out or refresh on a 401.
  if (epoch === null || response.status !== 401) throw apiError;
  const signedInSince = () => getSignInEpoch() !== epoch;

  // An admin disabled the account while it was signed in: sign out at once. Not when someone
  // signed in since this request started: the account it was sent for is not the one signed in.
  if (code === 'account_disabled') {
    if (signedInSince()) throw signedInSinceError();
    signOut().catch(() => undefined);
    throw apiError;
  }

  // /auth/refresh answers an expired REFRESH token with the same 401 token_expired. Taking
  // this branch for it (or for any unauthenticated call) would start a second refresh whose
  // own 401 awaits the first, so the two wait on each other forever and restore() never
  // settles. Those 401s must surface as the ApiError for the store to sign out on.
  if (code === 'token_expired' && !opts.retry && path !== '/auth/refresh') {
    // Refreshed and sent again only for the session it was sent for. After a newer sign in the
    // keychain holds the new session's tokens: refreshing with them and sending this again would
    // act for the new session (an old log out's DELETE of the push token would take the new
    // session's away).
    if (signedInSince()) throw signedInSinceError();
    let fresh: string | null;
    try {
      fresh = await sharedRefresh(epoch);
    } catch (refreshErr) {
      if (signedInSince()) throw signedInSinceError();
      // The store has already signed out on a definitive rejection. Any other refresh
      // failure (offline, 5xx) must surface as itself and must not end the session.
      throw isDefinitiveAuthFailure(refreshErr) ? apiError : refreshErr;
    }
    if (signedInSince()) throw signedInSinceError();
    // Sent again with the token the refresh got, also when a log out of this session has begun
    // meanwhile and that token was kept out of memory (the log out's own DELETE needs it).
    return send<T>(method, path, body, { ...opts, retry: true }, fresh ?? getAccessToken(), epoch);
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
  /**
   * Added for DELETE /me/blocks/{user_id} (Settings > Blocked accounts); answers 204. `body` is
   * optional (DELETE /me/push-tokens sends the token in it).
   */
  delete<T>(path: string, opts: RequestOptions = {}, body?: unknown): Promise<T> {
    return request<T>('DELETE', path, body, opts);
  },
};
