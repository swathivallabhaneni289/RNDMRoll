// @ts-nocheck
/**
 * Checks for accounts, sessions, settings and legal on the phone: who stays logged in and who is
 * logged out (SES-01 to SES-06), log outs kept for later, the launch without a connection, the
 * exact account lines, the sign-up's legal line, the Settings rows, the Forgot password page's
 * rules and the app's privacy promises. Not part of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-accounts-extra.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 *
 * Unlike the other check scripts, this one runs lib/session/store.ts and lib/api/client.ts for real,
 * SessionProvider included. Node has no React Native, so a loader hook below maps '@/...' to the
 * repo's files and swaps the native modules (expo-secure-store, expo-image, react-native, the push
 * token and notification modules) and react itself for small stand-ins: a keychain in memory, and
 * a React that runs one component's hooks. Each scenario loads its own copy of the app modules (a
 * "world"), so nothing one scenario leaves in module state reaches the next. No request leaves the
 * process: fetch is replaced, and the API address points nowhere.
 * (The nocheck line is because Node needs the .ts extension on imports; see check-settings.ts.)
 */
import { registerHooks } from 'node:module';
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.cwd();
// Read by lib/api/client.ts when it loads: even a fetch that got through would go nowhere.
process.env.EXPO_PUBLIC_API_BASE_URL = 'http://accounts-check.invalid';

let checks = 0;
let failures = 0;

function check(name: string, ok: boolean, detail?: string): void {
  checks += 1;
  if (!ok) {
    failures += 1;
    console.log(`FAIL ${name}${detail ? `: ${detail}` : ''}`);
  }
}

function eq(name: string, actual: unknown, expected: unknown): void {
  check(name, actual === expected, `got ${JSON.stringify(actual)}, wanted ${JSON.stringify(expected)}`);
}

const source = (rel: string) => readFileSync(join(root, rel), 'utf8');

/** Each step must start a line of the log, in this order (other lines may come between). */
function inOrder(name: string, log: string[], steps: string[]): void {
  let at = -1;
  for (const step of steps) {
    const next = log.findIndex((line, i) => i > at && line.startsWith(step));
    if (next < 0) {
      check(name, false, `"${step}" not found after line ${at} of ${JSON.stringify(log)}`);
      return;
    }
    at = next;
  }
  check(name, true);
}

// ---- Loading the app's modules in node --------------------------------------------------------

const STUBBED = [
  'expo-secure-store',
  'expo-image',
  'react-native',
  'react',
  '@/lib/notifications/module',
  '@/lib/notifications/pushToken',
];
const worlds = new Map();
globalThis.__accountsWorlds = worlds;

function worldOf(parentURL: string | undefined): string | null {
  if (!parentURL || !parentURL.startsWith('file:')) return null;
  return new URL(parentURL).searchParams.get('world');
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    const world = worldOf(context.parentURL);
    if (world !== null && STUBBED.includes(specifier)) {
      const stub = worlds.get(Number(world)).stubs[specifier];
      const names = Object.keys(stub).filter((name) => name !== 'default');
      const code =
        `const stub = globalThis.__accountsWorlds.get(${Number(world)}).stubs[${JSON.stringify(specifier)}];\n` +
        `export const { ${names.join(', ')} } = stub;\nexport default stub;\n`;
      return { url: `data:text/javascript,${encodeURIComponent(code)}`, shortCircuit: true };
    }
    if (specifier.startsWith('@/')) {
      const file = ['.ts', '.tsx'].map((ext) => join(root, specifier.slice(2) + ext)).find((path) => existsSync(path));
      if (file !== undefined) {
        const url = pathToFileURL(file);
        if (world !== null) url.searchParams.set('world', world);
        return { url: url.href, shortCircuit: true };
      }
    }
    return nextResolve(specifier, context);
  },
});

const ACCESS = 'rndmroll.access_token';
const REFRESH = 'rndmroll.refresh_token';
const PENDING = 'rndmroll.pending_logout';
/** What the keychain stand-in gives for WHEN_UNLOCKED_THIS_DEVICE_ONLY (the others are other values). */
const THIS_DEVICE_ONLY = 'accounts-check:WHEN_UNLOCKED_THIS_DEVICE_ONLY';
const STATUSES = ['loading', 'unauthenticated', 'authenticated', 'offline'];
const USER = {
  id: 'user-one',
  email: 'one@example.com',
  name: 'One',
  username: 'one',
  bio: null,
  avatar_url: null,
  email_verified: false,
  onboarding_complete: true,
  account_visibility: 'private',
};
const USER_TWO = { ...USER, id: 'user-two', email: 'two@example.com', name: 'Two', username: 'two' };

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const noContent = () => new Response(null, { status: 204 });
const html = (status: number) => new Response('<html>busy</html>', { status });
const offline = () => {
  throw new TypeError('Network request failed');
};

const realSetTimeout = globalThis.setTimeout;

/**
 * Lets everything that is ready run: promises, and timers of a few milliseconds (a scenario turns the
 * 10 second limit into one of 0). Real time passes in between, so a timer always gets its turn.
 */
async function settle(): Promise<void> {
  for (let round = 0; round < 3; round += 1) {
    for (let i = 0; i < 20; i += 1) await new Promise((resolve) => setImmediate(resolve));
    await new Promise((resolve) => realSetTimeout(resolve, 3));
  }
}

/** A React with just what SessionProvider calls: hooks by call order, effects after each render. */
function fakeReact(w) {
  const slots = [];
  let cursor = 0;
  let queued = false;
  let effects = [];
  const sameDeps = (a, b) => Array.isArray(a) && Array.isArray(b) && a.length === b.length && a.every((x, i) => Object.is(x, b[i]));
  w.render = () => {
    queued = false;
    if (w.component === null) return;
    cursor = 0;
    w.value = w.component().props.value;
    const run = effects;
    effects = [];
    for (const effect of run) effect();
  };
  return {
    createContext: (defaultValue) => ({ Provider: 'Provider', defaultValue }),
    createElement: (type, props, ...children) => ({ type, props: { ...props, children } }),
    useContext: () => w.value,
    useState(initial) {
      const i = cursor++;
      if (slots[i] === undefined) {
        const slot = { value: typeof initial === 'function' ? initial() : initial };
        slot.set = (next) => {
          const value = typeof next === 'function' ? next(slot.value) : next;
          if (STATUSES.includes(value)) w.log.push(`status ${value}`);
          if (Object.is(value, slot.value)) return;
          slot.value = value;
          if (!queued) {
            queued = true;
            queueMicrotask(w.render);
          }
        };
        slots[i] = slot;
      }
      return [slots[i].value, slots[i].set];
    },
    useRef(initial) {
      const i = cursor++;
      if (slots[i] === undefined) slots[i] = { current: initial };
      return slots[i];
    },
    useCallback(fn, deps) {
      const i = cursor++;
      if (slots[i] !== undefined && sameDeps(slots[i].deps, deps)) return slots[i].fn;
      slots[i] = { fn, deps };
      return fn;
    },
    useEffect(effect, deps) {
      const i = cursor++;
      const before = slots[i];
      if (before !== undefined && sameDeps(before.deps, deps)) return;
      const slot = { deps, cleanup: undefined };
      slots[i] = slot;
      effects.push(() => {
        if (typeof before?.cleanup === 'function') before.cleanup();
        slot.cleanup = effect();
      });
    },
  };
}

let current = null;
let worldCount = 0;
// Every request of every world comes here, and goes to the world of the scenario that runs now.
globalThis.fetch = (url, init) =>
  current === null ? Promise.reject(new TypeError('no scenario is running')) : current.fetch(url, init);

/**
 * A fresh copy of the app's session modules over a keychain holding `keychain`. route answers each
 * request (a Response, a promise of one, or a throw for no connection).
 */
async function newWorld(name: string, keychain: Record<string, string> = {}) {
  const id = ++worldCount;
  const w = {
    id,
    name,
    log: [],
    calls: [],
    writes: [],
    alerts: [],
    keychain: new Map(Object.entries(keychain)),
    keychainLocked: false,
    unsentPushToken: null,
    route: () => offline(),
    component: null,
    value: null,
    appStateListener: null,
  };
  w.fetch = (url, init = {}) => {
    const u = new URL(url);
    const call = {
      method: init.method ?? 'GET',
      path: u.pathname.replace(/^\/v1/, '') + u.search,
      body: typeof init.body === 'string' ? JSON.parse(init.body) : undefined,
      auth: init.headers?.Authorization ?? null,
    };
    w.calls.push(call);
    w.log.push(`fetch ${call.method} ${call.path}`);
    return new Promise((resolve, reject) => {
      const aborted = () => reject(new DOMException('The operation was aborted.', 'AbortError'));
      if (init.signal?.aborted) return aborted();
      init.signal?.addEventListener('abort', aborted);
      Promise.resolve()
        .then(() => w.route(call))
        .then(resolve, reject);
    });
  };
  const locked = () => {
    if (w.keychainLocked) throw new Error('the keychain is locked');
  };
  w.stubs = {
    'expo-secure-store': {
      WHEN_UNLOCKED_THIS_DEVICE_ONLY: THIS_DEVICE_ONLY,
      WHEN_UNLOCKED: 'accounts-check:WHEN_UNLOCKED',
      AFTER_FIRST_UNLOCK: 'accounts-check:AFTER_FIRST_UNLOCK',
      AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY: 'accounts-check:AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY',
      ALWAYS: 'accounts-check:ALWAYS',
      async getItemAsync(key) {
        locked();
        return w.keychain.has(key) ? w.keychain.get(key) : null;
      },
      async setItemAsync(key, value, options) {
        locked();
        w.writes.push({ key, value, options });
        w.keychain.set(key, value);
        w.log.push(`keychain set ${key}`);
      },
      async deleteItemAsync(key) {
        locked();
        w.keychain.delete(key);
        w.log.push(`keychain delete ${key}`);
      },
    },
    'expo-image': {
      Image: {
        clearMemoryCache: async () => w.log.push('photos cleared from memory'),
        clearDiskCache: async () => w.log.push('photos cleared from disk'),
      },
    },
    'react-native': {
      Alert: { alert: (...args) => w.alerts.push(args) },
      AppState: {
        currentState: 'active',
        addEventListener: (_type, listener) => {
          w.appStateListener = listener;
          return { remove: () => (w.appStateListener = null) };
        },
      },
      Platform: { OS: 'ios' },
    },
    react: fakeReact(w),
    '@/lib/notifications/module': {
      cancelScheduledNotifications: async () => w.log.push('reminders cancelled'),
    },
    '@/lib/notifications/pushToken': {
      beginSignIn: () => () => undefined,
      deleteLeftPushToken: async (token, accessToken) => w.log.push(`deleteLeftPushToken ${token} ${accessToken}`),
      logOutFailed: () => w.log.push('logOutFailed'),
      registerPushToken: async (userId) => w.log.push(`registerPushToken ${userId}`),
      unregisterPushToken: async (mayRefresh) => {
        w.log.push(`unregisterPushToken ${mayRefresh}`);
        return w.unsentPushToken;
      },
    },
  };
  worlds.set(id, w);
  current = w;
  w.at = (rel: string) => {
    const url = pathToFileURL(join(root, rel));
    url.searchParams.set('world', String(id));
    return url.href;
  };
  w.store = await import(w.at('lib/session/store.ts'));
  w.client = await import(w.at('lib/api/client.ts'));
  w.mount = () => {
    w.component = () => w.store.SessionProvider({ children: null });
    w.render();
  };
  w.appState = (next: string) => w.appStateListener?.(next);
  w.status = () => w.value?.status;
  w.to = (path: string) => w.calls.filter((call) => call.path === path);
  w.tokens = () => [w.keychain.get(ACCESS) ?? null, w.keychain.get(REFRESH) ?? null].join('/');
  return w;
}

/** Answers a launch restore with stored tokens a1/r1: the refresh gives a2/r2, then /me works. */
const signedIn = (rest = (_call) => offline()) => (call) => {
  if (call.path === '/auth/refresh') return json(200, { access_token: 'a2', refresh_token: 'r2', expires_in: 900 });
  if (call.path === '/me' && call.auth === 'Bearer a2') return json(200, USER);
  return rest(call);
};

/** A world that launched with stored tokens and is signed in (the restore refreshed to a2/r2). */
async function signedInWorld(name: string, rest?: (call) => unknown) {
  const w = await newWorld(name, { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.route = signedIn(rest);
  w.mount();
  await settle();
  eq(`${name}: the launch restore signs in`, w.status(), 'authenticated');
  return w;
}

const failed = (promise: Promise<unknown>) => promise.then(() => null, (err) => err);

// ---- SES-06 and SES-04: only a definite no from the server signs out ---------------------------
{
  const w = await newWorld('the client on its own');
  const { api, ApiError, isRetryable } = w.client;
  const { isDefinitiveAuthFailure, registerSessionHandlers, setAccessToken } = w.store;
  let refreshes = 0;
  let signOuts = 0;
  let refreshAnswer = () => {
    setAccessToken('new');
    return 'new';
  };
  registerSessionHandlers({
    refreshSession: async () => {
      refreshes += 1;
      await new Promise((resolve) => setImmediate(resolve));
      return refreshAnswer();
    },
    signOut: async () => {
      signOuts += 1;
    },
  });
  setAccessToken('old');
  w.route = (call) => {
    switch (call.path) {
      case '/me':
        return call.auth === 'Bearer new' ? json(200, { id: 'x' }) : json(401, { error: 'token_expired' });
      case '/disabled':
        return json(401, { error: 'account_disabled' });
      case '/s429':
        return json(429, { error: 'rate_limited' });
      case '/s500':
        return json(500, { error: 'server_error' });
      case '/s503':
        return html(503);
      case '/silent':
        return new Promise(() => undefined);
      case '/auth/refresh':
        return json(401, { error: 'token_expired' });
      default:
        return offline();
    }
  };

  const answers = await Promise.all([api.get('/me'), api.get('/me'), api.get('/me')]);
  eq('three requests whose token expired at once share one refresh', refreshes, 1);
  check('each of the three is sent again with the new token and works', answers.every((answer) => answer?.id === 'x'));
  eq('the three went out with the old token', w.to('/me').filter((call) => call.auth === 'Bearer old').length, 3);
  eq('and once more each with the new one', w.to('/me').filter((call) => call.auth === 'Bearer new').length, 3);

  const disabled = await failed(api.get('/disabled'));
  check('a 401 account_disabled reaches the caller as itself', disabled instanceof ApiError && disabled.status === 401 && disabled.code === 'account_disabled');
  eq('a 401 account_disabled signs out at once, once', signOuts, 1);
  eq('and refreshes nothing', refreshes, 1);

  for (const [path, status, code] of [
    ['/offline', 0, 'network_unavailable'],
    ['/s429', 429, 'rate_limited'],
    ['/s500', 500, 'server_error'],
    ['/s503', 503, 'server_error'],
  ]) {
    const err = await failed(api.get(path));
    check(`${path}: the error says ${status} ${code}`, err?.status === status && err?.code === code, `got ${err?.status} ${err?.code}`);
    check(`${path}: may work later (isRetryable)`, isRetryable(err));
    check(`${path}: is not a definite no (isDefinitiveAuthFailure)`, !isDefinitiveAuthFailure(err));
  }
  eq('no connection, 429, 500 and 503 sign nobody out', signOuts, 1);
  eq('and refresh nothing', refreshes, 1);

  // The client's own 10 second limit: a silent server ends as no connection.
  const delays = [];
  globalThis.setTimeout = (fn, ms, ...args) => {
    delays.push(ms);
    return realSetTimeout(fn, ms === 10_000 ? 0 : ms, ...args);
  };
  let silent;
  try {
    silent = await failed(api.get('/silent'));
  } finally {
    globalThis.setTimeout = realSetTimeout;
  }
  check('every request has a 10 second limit', delays.includes(10_000), `timers ${JSON.stringify(delays)}`);
  check('a request the server never answers ends as no connection', silent?.status === 0 && silent?.code === 'network_unavailable');
  eq('and signs nobody out', signOuts, 1);

  // /auth/refresh answers an expired refresh token with 401 token_expired: never a second refresh.
  const asSent = await failed(api.post('/auth/refresh', { refresh_token: 'r' }, { auth: false }));
  eq("the store's own /auth/refresh 401 comes back as itself", asSent?.code, 'token_expired');
  const withToken = await failed(api.post('/auth/refresh', { refresh_token: 'r' }));
  eq('/auth/refresh sent with the session token: its 401 comes back as itself', withToken?.code, 'token_expired');
  eq('a 401 from /auth/refresh starts no second refresh', refreshes, 1);
  eq('nor signs out', signOuts, 1);

  // A refresh that could not reach the server gives the request that error, not its 401.
  setAccessToken('old');
  refreshAnswer = () => {
    throw new ApiError(0, 'network_unavailable');
  };
  const lost = await failed(api.get('/me'));
  check('an expired token whose refresh found no connection fails as no connection', lost?.status === 0 && lost?.code === 'network_unavailable');
  refreshAnswer = () => {
    throw new ApiError(401, 'token_invalid');
  };
  const refused = await failed(api.get('/me'));
  check("an expired token whose refresh was refused fails with the request's 401", refused?.status === 401 && refused?.code === 'token_expired');
  eq('neither signs out from the client (the store decides on a refused refresh)', signOuts, 1);

  for (const err of [
    new ApiError(401, 'token_invalid'),
    new ApiError(401, 'invalid_credentials'),
    new ApiError(403, 'account_disabled'),
    new ApiError(400, 'token_expired'),
    new ApiError(400, 'token_invalid'),
  ]) {
    check(`a definite no: ${err.status} ${err.code}`, isDefinitiveAuthFailure(err));
  }
  for (const err of [
    new ApiError(0, 'network_unavailable'),
    new ApiError(429, 'rate_limited'),
    new ApiError(500, 'server_error'),
    new ApiError(502, 'server_error'),
    new ApiError(503, 'server_error'),
    new TypeError('Network request failed'),
    null,
  ]) {
    check(`not a definite no: ${err?.status ?? String(err)}`, !isDefinitiveAuthFailure(err));
  }
  for (const status of [0, 429, 500, 502, 503]) check(`${status} may work later`, isRetryable(new ApiError(status, 'server_error')));
  check('an error that is not an ApiError may work later', isRetryable(new TypeError('x')));
  for (const status of [400, 401, 403, 404, 409]) check(`${status} will not work later as it is`, !isRetryable(new ApiError(status, 'validation_failed')));
  await settle();
}

// ---- SES-01: a 401 for a session that is over touches nothing of the new one ------------------
{
  const w = await signedInWorld('a newer sign in');
  const held = new Map();
  w.route = signedIn((call) => {
    if (call.path.startsWith('/held')) return new Promise((resolve) => held.set(call.path, resolve));
    if (call.path === '/during') return json(200, {});
    return offline();
  });
  const expired = failed(w.client.api.get('/held-expired'));
  const disabled = failed(w.client.api.get('/held-disabled'));
  await settle();
  check('both requests went out with the first session token', w.to('/held-expired')[0]?.auth === 'Bearer a2' && w.to('/held-disabled')[0]?.auth === 'Bearer a2');

  const signing = w.value.signIn({ access_token: 'b1', refresh_token: 's1', expires_in: 900, user: USER_TWO });
  const during = w.client.api.get('/during');
  await signing;
  await during;
  await settle();
  eq('a request begun while a sign in writes its tokens waits and carries the new token', w.to('/during')[0]?.auth, 'Bearer b1');
  eq('the new sign in is in place', w.tokens(), 'b1/s1');

  held.get('/held-expired')(json(401, { error: 'token_expired' }));
  held.get('/held-disabled')(json(401, { error: 'account_disabled' }));
  const [e1, e2] = await Promise.all([expired, disabled]);
  await settle();
  check("the old request's token_expired comes back as a plain 401", e1?.status === 401 && e1?.code === 'token_invalid');
  check("the old request's account_disabled comes back as a plain 401", e2?.status === 401 && e2?.code === 'token_invalid');
  eq('neither refreshes (only the launch restore did)', w.to('/auth/refresh').length, 1);
  eq('neither logs out', w.to('/auth/logout').length, 0);
  check('nothing let go of the push token', !w.log.some((line) => line.startsWith('unregisterPushToken')));
  eq('the new session stays signed in', w.status(), 'authenticated');
  eq('with its own person', w.value.user?.id, 'user-two');
  eq('and its tokens in the keychain', w.tokens(), 'b1/s1');
}
{
  // The same, with the store's handlers swapped for counters once the new session is in: the client
  // itself never asks for a refresh or a log out for a session that is over.
  const w = await signedInWorld('a newer sign in, the client alone');
  const held = new Map();
  w.route = signedIn((call) => (call.path.startsWith('/held') ? new Promise((resolve) => held.set(call.path, resolve)) : offline()));
  const expired = failed(w.client.api.get('/held-expired'));
  const disabled = failed(w.client.api.get('/held-disabled'));
  await settle();
  await w.value.signIn({ access_token: 'b1', refresh_token: 's1', expires_in: 900, user: USER_TWO });
  await settle();
  let asked = 0;
  w.store.registerSessionHandlers({
    refreshSession: async () => {
      asked += 1;
      return null;
    },
    signOut: async () => {
      asked += 1;
    },
  });
  held.get('/held-expired')(json(401, { error: 'token_expired' }));
  held.get('/held-disabled')(json(401, { error: 'account_disabled' }));
  await Promise.all([expired, disabled]);
  eq('the client asks for no refresh and no log out for a request of a session that is over', asked, 0);
}

// ---- SES-06 and SES-02: account_disabled logs out at once, in order --------------------------
{
  const w = await signedInWorld('disabled while signed in');
  w.route = signedIn((call) => {
    if (call.path === '/board') return json(401, { error: 'account_disabled' });
    if (call.path === '/auth/logout') return noContent();
    return offline();
  });
  const mark = w.log.length;
  const err = await failed(w.client.api.get('/board'));
  await settle();
  const log = w.log.slice(mark);
  eq('the caller gets account_disabled', err?.code, 'account_disabled');
  eq('the phone is logged out', w.status(), 'unauthenticated');
  inOrder('log out: push token first, then /auth/logout, then the keychain, then the state', log, [
    'unregisterPushToken true',
    'fetch POST /auth/logout',
    `keychain delete ${ACCESS}`,
    `keychain delete ${REFRESH}`,
    'status unauthenticated',
  ]);
  const firstStatus = log.indexOf('status unauthenticated');
  check(
    'the state turns unauthenticated only once both keychain items are gone',
    firstStatus > log.indexOf(`keychain delete ${ACCESS}`) && firstStatus > log.indexOf(`keychain delete ${REFRESH}`) && log.indexOf(`keychain delete ${REFRESH}`) >= 0
  );
  check('the push token goes before /auth/logout is sent', log.indexOf('unregisterPushToken true') >= 0 && log.indexOf('unregisterPushToken true') < log.indexOf('fetch POST /auth/logout'));
  inOrder('then the reminders and saved photos go', log, ['status unauthenticated', 'reminders cancelled']);
  check('the saved photos are cleared', log.includes('photos cleared from memory') && log.includes('photos cleared from disk'));
  eq("/auth/logout ends this session (the launch refresh's token)", w.to('/auth/logout')[0]?.body?.refresh_token, 'r2');
  eq('/auth/logout carries no access token', w.to('/auth/logout')[0]?.auth, null);
  eq('no token is left in the keychain', w.tokens(), '/');
  eq('nor in memory', w.store.getAccessToken(), null);
  check('nothing is kept for later (the server answered)', !w.keychain.has(PENDING));
}

// ---- SES-02 and SES-03: Log out with no connection is kept, then sent ------------------------
{
  const w = await signedInWorld('log out offline');
  w.unsentPushToken = 'push-1';
  let online = false;
  w.route = signedIn((call) => (call.path === '/auth/logout' && online ? noContent() : offline()));
  await w.value.signOut();
  await settle();
  eq('Log out with no connection still logs out on the phone', w.status(), 'unauthenticated');
  eq('and empties the keychain', w.tokens(), '/');
  eq(
    'the log out is kept: its session, its access token and the push token whose DELETE failed',
    w.keychain.get(PENDING),
    JSON.stringify([{ refresh_token: 'r2', access_token: 'a2', push_token: 'push-1' }])
  );

  online = true;
  const mark = w.log.length;
  w.appState('background');
  w.appState('active');
  await settle();
  inOrder('back in the app: the push token DELETE, then /auth/logout, then it is forgotten', w.log.slice(mark), [
    'deleteLeftPushToken push-1 a2',
    'fetch POST /auth/logout',
    `keychain delete ${PENDING}`,
  ]);
  eq('the kept log out ends the old session', w.to('/auth/logout').at(-1)?.body?.refresh_token, 'r2');
  check('nothing is kept any more', !w.keychain.has(PENDING));
  eq('still logged out', w.status(), 'unauthenticated');
}

// ---- A refused refresh logs out, and its log out is never kept --------------------------------
{
  const w = await newWorld('a refused refresh', { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.route = (call) => (call.path === '/auth/refresh' ? json(401, { error: 'token_invalid' }) : offline());
  w.mount();
  await settle();
  eq('a refresh the server refused shows Welcome', w.status(), 'unauthenticated');
  check('the push token goes without a refresh on the way', w.log.includes('unregisterPushToken false'));
  eq('/auth/logout is tried with the refused token', w.to('/auth/logout')[0]?.body?.refresh_token, 'r1');
  check('and is not kept when it finds no connection (the server already refused it)', !w.keychain.has(PENDING));
  eq('the keychain is emptied', w.tokens(), '/');
}

// ---- SES-04: a launch the server could not answer keeps the person logged in -----------------
for (const [label, fail, timed] of [
  ['no connection', () => offline(), false],
  ['a 500', () => json(500, { error: 'server_error' }), false],
  ['a 502 without the usual body', () => html(502), false],
  ['a 503 without the usual body', () => html(503), false],
  ['a 429', () => json(429, { error: 'rate_limited' }), false],
  ['a server that never answers (the 10 second limit)', () => new Promise(() => undefined), true],
]) {
  const w = await newWorld(`launch with ${label}`, { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.route = (call) => (call.path === '/auth/refresh' ? fail() : offline());
  if (timed) globalThis.setTimeout = (fn, ms, ...args) => realSetTimeout(fn, ms === 10_000 ? 0 : ms, ...args);
  try {
    w.mount();
    await settle();
  } finally {
    globalThis.setTimeout = realSetTimeout;
  }
  eq(`launch with ${label}: the offline page, not Welcome`, w.status(), 'offline');
  eq(`launch with ${label}: both tokens stay`, w.tokens(), 'a1/r1');
  eq(`launch with ${label}: nothing logs out`, w.to('/auth/logout').length, 0);
  check(`launch with ${label}: the push token stays`, !w.log.some((line) => line.startsWith('unregisterPushToken')));
  w.route = signedIn();
  await w.value.retryRestore();
  await settle();
  eq(`launch with ${label}: Try again with the connection back signs in`, w.status(), 'authenticated');
}
{
  const w = await newWorld('launch: /me finds no connection', { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.route = (call) => (call.path === '/auth/refresh' ? json(200, { access_token: 'a2', refresh_token: 'r2', expires_in: 900 }) : offline());
  w.mount();
  await settle();
  eq('a refresh that worked, then /me with no connection: the offline page', w.status(), 'offline');
  eq('with the refreshed tokens kept', w.tokens(), 'a2/r2');
}
{
  const w = await newWorld('launch: keychain locked', { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.keychainLocked = true;
  w.route = signedIn();
  w.mount();
  await settle();
  eq('a keychain that cannot be read at launch: the offline page', w.status(), 'offline');
  eq('and no refresh is tried', w.to('/auth/refresh').length, 0);
  w.keychainLocked = false;
  w.appState('background');
  w.appState('active');
  await settle();
  eq('unlocked and back in the app: signed in', w.status(), 'authenticated');
}
for (const [label, answer] of [
  ['401 token_invalid', () => json(401, { error: 'token_invalid' })],
  ['401 token_expired', () => json(401, { error: 'token_expired' })],
  ['403 account_disabled', () => json(403, { error: 'account_disabled' })],
]) {
  const w = await newWorld(`launch refused: ${label}`, { [ACCESS]: 'a1', [REFRESH]: 'r1' });
  w.route = (call) => (call.path === '/auth/refresh' ? answer() : call.path === '/auth/logout' ? noContent() : offline());
  w.mount();
  await settle();
  eq(`launch refused with ${label}: Welcome`, w.status(), 'unauthenticated');
  eq(`launch refused with ${label}: the keychain is emptied`, w.tokens(), '/');
}
{
  const w = await newWorld('launch: no stored token');
  w.mount();
  await settle();
  eq('no stored token: Welcome', w.status(), 'unauthenticated');
  eq('and no request at all', w.calls.length, 0);
}

// ---- SES-03: kept log outs, at most 10, sent in order -----------------------------------------
{
  const twelve = Array.from({ length: 12 }, (_, i) => ({ refresh_token: `rt${i}`, access_token: null, push_token: null }));
  const w = await newWorld('twelve kept log outs at launch', { [PENDING]: JSON.stringify(twelve) });
  let sent = 0;
  w.route = (call) => {
    if (call.path !== '/auth/logout') return offline();
    sent += 1;
    return sent === 3 ? offline() : noContent();
  };
  w.mount();
  await settle();
  eq('at launch the kept log outs go out oldest first, up to one that finds no connection', w.to('/auth/logout').map((call) => call.body.refresh_token).join(','), 'rt0,rt1,rt2');
  const kept = JSON.parse(w.keychain.get(PENDING) ?? '[]').map((entry) => entry.refresh_token);
  eq('what is left is at most 10, the newest, from the one that failed on', kept.join(','), 'rt2,rt3,rt4,rt5,rt6,rt7,rt8,rt9,rt10,rt11');
  eq('they go to /auth/logout with no access token', w.to('/auth/logout').every((call) => call.auth === null), true);
}
{
  const ten = Array.from({ length: 10 }, (_, i) => ({ refresh_token: `old${i}`, access_token: null, push_token: null }));
  const w = await signedInWorld('a log out kept when 10 wait', (call) => offline());
  w.keychain.set(PENDING, JSON.stringify(ten));
  await w.value.signOut();
  await settle();
  const kept = JSON.parse(w.keychain.get(PENDING) ?? '[]').map((entry) => entry.refresh_token);
  eq('an eleventh kept log out drops the oldest', kept.join(','), 'old1,old2,old3,old4,old5,old6,old7,old8,old9,r2');
}
{
  const entry = [{ refresh_token: 'rt-a', access_token: 'old-access', push_token: 'push-a' }];
  const signedInHere = await newWorld('a kept push DELETE while someone is signed in', { [PENDING]: JSON.stringify(entry), [REFRESH]: 'r-now' });
  signedInHere.route = (call) => (call.path === '/auth/logout' ? noContent() : offline());
  signedInHere.store.requestWorked();
  await settle();
  check('a kept push token DELETE is not sent while someone is signed in', !signedInHere.log.some((line) => line.startsWith('deleteLeftPushToken')));
  eq('the kept log out itself still goes', signedInHere.to('/auth/logout')[0]?.body?.refresh_token, 'rt-a');

  const inMemory = await newWorld('a kept push DELETE with a session in memory', { [PENDING]: JSON.stringify(entry) });
  inMemory.route = (call) => (call.path === '/auth/logout' ? noContent() : offline());
  inMemory.store.setAccessToken('live');
  inMemory.store.requestWorked();
  await settle();
  check('nor while a session holds an access token', !inMemory.log.some((line) => line.startsWith('deleteLeftPushToken')));

  const signedOut = await newWorld('a kept push DELETE while signed out', { [PENDING]: JSON.stringify(entry) });
  signedOut.route = (call) => (call.path === '/auth/logout' ? noContent() : offline());
  signedOut.store.requestWorked();
  await settle();
  inOrder('signed out, the kept push token DELETE goes first, with its own access token', signedOut.log, [
    'deleteLeftPushToken push-a old-access',
    'fetch POST /auth/logout',
  ]);
  check('then the kept list is gone', !signedOut.keychain.has(PENDING));
}
{
  const two = [
    { refresh_token: 'rt0', access_token: null, push_token: null },
    { refresh_token: 'rt1', access_token: null, push_token: null },
  ];
  const w = await newWorld('requests that work send kept log outs at most every 30 seconds', { [PENDING]: JSON.stringify(two) });
  w.route = () => offline();
  const realNow = Date.now;
  let shift = 0;
  Date.now = () => realNow() + shift;
  try {
    w.store.requestWorked();
    await settle();
    eq('a request that works sends the first kept log out', w.to('/auth/logout').length, 1);
    w.store.requestWorked();
    await settle();
    eq('another within 30 seconds sends nothing', w.to('/auth/logout').length, 1);
    shift = 29_000;
    w.store.requestWorked();
    await settle();
    eq('nor at 29 seconds', w.to('/auth/logout').length, 1);
    shift = 31_000;
    w.store.requestWorked();
    await settle();
    eq('after 30 seconds it tries again, oldest first', w.to('/auth/logout').map((call) => call.body.refresh_token).join(','), 'rt0,rt0');
  } finally {
    Date.now = realNow;
  }
  eq('one that found no connection stays, with the ones after it', JSON.parse(w.keychain.get(PENDING)).length, 2);
}
{
  const kept = JSON.stringify([{ refresh_token: 'rt-x', access_token: null, push_token: null }]);
  const works = await newWorld('a request that works', { [PENDING]: kept });
  works.route = (call) => (call.path === '/usernames/suggest' ? json(200, { username: 'abc' }) : call.path === '/auth/logout' ? noContent() : offline());
  await works.client.api.get('/usernames/suggest', { auth: false });
  await settle();
  eq('any request that works sends the kept log outs', works.to('/auth/logout').map((call) => call.body.refresh_token).join(','), 'rt-x');
  const fails = await newWorld('a request that fails', { [PENDING]: kept });
  fails.route = (call) => (call.path === '/usernames/suggest' ? json(500, { error: 'server_error' }) : call.path === '/auth/logout' ? noContent() : offline());
  await failed(fails.client.api.get('/usernames/suggest', { auth: false }));
  await failed(fails.client.api.get('/elsewhere', { auth: false }));
  await settle();
  eq('a request that fails sends none', fails.to('/auth/logout').length, 0);
}
for (const [label, stored, expected] of [
  ['text that is not JSON', '{not json', ''],
  ['JSON that is not a list', '"just text"', ''],
  ['a list with broken entries', JSON.stringify([{ refresh_token: 5 }, null, 'x', { refresh_token: '' }, { refresh_token: 'ok', access_token: 7 }]), 'ok'],
]) {
  const w = await newWorld(`a garbled kept list: ${label}`, { [PENDING]: stored });
  w.route = (call) => (call.path === '/auth/logout' ? noContent() : offline());
  let threw = false;
  try {
    w.store.requestWorked();
    await settle();
  } catch {
    threw = true;
  }
  check(`a garbled kept list (${label}) throws nothing`, !threw);
  eq(`a garbled kept list (${label}) sends only the good entries`, w.to('/auth/logout').map((call) => call.body.refresh_token).join(','), expected);
  check(`a garbled kept list (${label}) sends no push token DELETE`, !w.log.some((line) => line.startsWith('deleteLeftPushToken')));
}

// ---- SES-01: tokens in the keychain of this phone only, by one module ------------------------
{
  const all = [...worlds.values()].flatMap((w) => w.writes);
  check('the scenarios above wrote the keychain', all.length > 10, `${all.length} writes`);
  const loose = all.filter((write) => write.options?.keychainAccessible !== THIS_DEVICE_ONLY);
  check('every keychain write is WHEN_UNLOCKED_THIS_DEVICE_ONLY', loose.length === 0, JSON.stringify(loose.slice(0, 3).map((write) => write.key)));
  for (const key of [ACCESS, REFRESH, PENDING]) check(`${key} was written in these scenarios`, all.some((write) => write.key === key));

  const files = [];
  const walk = (dir) => {
    for (const name of readdirSync(join(root, dir))) {
      const rel = join(dir, name);
      if (statSync(join(root, rel)).isDirectory()) walk(rel);
      else if (/\.(ts|tsx|js|jsx)$/.test(name)) files.push(rel);
    }
  };
  for (const dir of ['app', 'components', 'lib']) walk(dir);
  // The session's keychain items are named by the session store alone.
  for (const key of [ACCESS, REFRESH, PENDING]) {
    eq(`only lib/session/store.ts names ${key}`, files.filter((rel) => source(rel).includes(key)).join(','), 'lib/session/store.ts');
  }
  // The keychain itself: the session store, and the 8 PM reminder's switch (plain flags, not
  // credentials, lib/notifications/reminder.ts says why). Anything new here needs a look.
  const importers = files.filter((rel) => /from ['"]expo-secure-store['"]|require\(['"]expo-secure-store['"]\)/.test(source(rel)));
  eq('the keychain is used by the session store and the reminder switch only', importers.join(','), 'lib/notifications/reminder.ts,lib/session/store.ts');
}

// ---- SES-04: the offline page -----------------------------------------------------------------
{
  const page = source('components/session/OfflineScreen.tsx');
  check("the offline page says You're offline.", /accessibilityRole="header">\s*You're offline\.\s*</.test(page));
  check('and that the person is still logged in', page.includes('Check your connection and try again. You are still logged in.'));
  check('Try again runs the launch restore again', /const \{ retryRestore \} = useSession\(\);/.test(page) && /await retryRestore\(\);/.test(page) && /label="Try again"[\s\S]*?onPress=\{tryAgain\}/.test(page));
  const layout = source('app/_layout.tsx');
  check('the app shows the offline page for the offline status, over the app', /gateSettled && status === 'offline' \? \(\s*<View style=\{StyleSheet\.absoluteFill\}>\s*<OfflineScreen \/>/.test(layout));
  check('Welcome (the auth group) is not where offline lands', /<Stack\.Protected guard=\{gateSettled && \(status !== 'authenticated'/.test(layout));
}

// ---- The account lines, word for word (PRD 8.1, 8.2) ------------------------------------------
const lines = [];
{
  const w = await newWorld('the account lines');
  const { ApiError } = w.client;
  for (const [code, status, line] of [
    ['email_taken', 409, 'That email is already in use.'],
    ['invalid_credentials', 401, "That email, username or password isn't right."],
    ['account_disabled', 403, 'This account has been disabled.'],
  ]) {
    eq(`the ${code} line`, new ApiError(status, code).userMessage, line);
    lines.push(line);
  }
  const restored = await import(w.at('lib/auth/restored.ts'));
  restored.sayIfRestored({ access_token: 'x', refresh_token: 'y', expires_in: 900, user: USER, restored: true });
  eq('a log in that kept the account says so', JSON.stringify(w.alerts), JSON.stringify([['Welcome back', "Your account won't be deleted."]]));
  w.alerts.length = 0;
  restored.sayIfRestored({ access_token: 'x', refresh_token: 'y', expires_in: 900, user: USER });
  restored.sayIfRestored({ access_token: 'x', refresh_token: 'y', expires_in: 900, user: USER, restored: false });
  eq('any other log in says nothing', w.alerts.length, 0);
  lines.push(restored.RESTORED_MESSAGE);
}
{
  const rules = await import(pathToFileURL(join(root, 'lib/settings/rules.ts')).href);
  const birthday = await import(pathToFileURL(join(root, 'lib/profile/birthday.ts')).href);
  eq('the password change notice', rules.PASSWORD_NOTICE, 'Password changed. Other phones are signed out.');
  eq('the under 13 line', birthday.BIRTHDAY_MESSAGES.underAge, 'You must be at least 13 to use RNDMRoll.');
  lines.push(rules.PASSWORD_NOTICE, birthday.BIRTHDAY_MESSAGES.underAge);
  check('Password shows that notice on Settings', /setSettingsNotice\(PASSWORD_NOTICE\);/.test(source('components/settings/PasswordScreen.tsx')));

  const form = source('components/profile/ProfileForm.tsx');
  check("the server's under 13 refusal", /const REFUSAL_TITLE = 'RNDMRoll is for ages 13 and up\.';/.test(form));
  check(
    'is shown, with the under 13 line under Birthday',
    /if \(err\.code === 'under_minimum_age'\) \{\s*setBirthdayError\(BIRTHDAY_MESSAGES\.underAge\);\s*showRefusal\(\);/.test(form) &&
      /Alert\.alert\(REFUSAL_TITLE, REFUSAL_MESSAGE\);/.test(form)
  );
  check('sign-up says a taken email in the same words', /const EMAIL_TAKEN_MESSAGE = 'That email is already in use\.';/.test(form));
  lines.push('RNDMRoll is for ages 13 and up.');
  const restoredAfter = (page: string, call: string) =>
    new RegExp(`${call.replace(/[()[\]{}.*+?^$|\\]/g, '\\$&')}\\s*(lostSignupRef\\.current = null;\\s*)?(//[^\\n]*\\n\\s*)*sayIfRestored\\(result\\);`).test(page);
  check('Log in says a kept account right after it signs in', restoredAfter(source('app/(auth)/login.tsx'), 'await signIn(result);'));
  check('a Forgot password reset does too', restoredAfter(source('app/(auth)/forgot-password.tsx'), 'await signIn(result);'));
  check("sign-up's way back to its lost account does too", restoredAfter(form, 'await signIn(result, { extras: extrasPage });'));

  const del = source('components/settings/DeleteAccountScreen.tsx');
  check('Delete account asks once', /const CONFIRM_TITLE = 'Delete your account\?';/.test(del) && /Alert\.alert\(\s*CONFIRM_TITLE,/.test(del));
  check(
    'then shows the day the server sent',
    /doneMessage\(result\.delete_on_label\)/.test(del) && /return `It will be deleted for good on \$\{label\}\. Log back in before then to keep it\.`;/.test(del)
  );
  check('Settings asks before it logs out', /Alert\.alert\(\s*'Log out',\s*'Are you sure you want to log out\?',/.test(source('components/settings/LogOutSheet.tsx')));
  lines.push('Delete your account?', 'Are you sure you want to log out?', "You're offline.", 'Check your connection and try again. You are still logged in.');
}
for (const line of lines) {
  check(`copy rules (no dash, ends with . or ?): ${line}`, !/[—–]/.test(line) && /[.?]$/.test(line) && !line.includes('  '));
}

// ---- ACC-08, SET-02: the sign-up legal line ---------------------------------------------------
{
  const form = source('components/profile/ProfileForm.tsx');
  const start = form.indexOf("{'By tapping Continue, you agree to the '}");
  const open = form.lastIndexOf('<AppText', start);
  const close = form.indexOf('</AppText>', start);
  const block = open >= 0 && close > start ? form.slice(open, close) : '';
  check('sign-up has the legal line', block !== '');
  // The words as JSX shows them: a string in braces keeps every space, the Text tags go, and plain
  // text loses its line breaks and the spaces around them.
  const TAG = /<\/?Text(?:\s+\w+=\{(?:[^{}]|\{[^{}]*\})*\})*\s*>/g;
  const shown = (start >= 0 && close > start ? form.slice(start, close) : '')
    .split(/(\{'[^']*'\}|\{"[^"]*"\})/)
    .map((part) =>
      /^\{['"]/.test(part)
        ? part.slice(2, -2)
        : part
            .replace(TAG, '\n')
            .split('\n')
            .map((line) => line.trim())
            .filter(Boolean)
            .join(' ')
    )
    .join('');
  eq(
    'the line, word for word',
    shown,
    "By tapping Continue, you agree to the Terms of Use and confirm you've read the Privacy Policy."
  );
  check('Terms of Use opens the Terms page', /<Text style=\{LEGAL_LINK\} onPress=\{\(\) => router\.push\('\/terms'\)\}>\s*Terms of Use\s*<\/Text>/.test(block));
  check('Privacy Policy opens the Privacy page', /<Text style=\{LEGAL_LINK\} onPress=\{\(\) => router\.push\('\/privacy'\)\}>\s*Privacy Policy\s*<\/Text>/.test(block));
  check(
    "VoiceOver reaches them as the line's two actions",
    /accessibilityActions=\{\[\s*\{ name: 'terms', label: 'Open the Terms of Use' \},\s*\{ name: 'privacy', label: 'Open the Privacy Policy' \},\s*\]\}/.test(block) &&
      /if \(event\.nativeEvent\.actionName === 'terms'\) router\.push\('\/terms'\);/.test(block) &&
      /if \(event\.nativeEvent\.actionName === 'privacy'\) router\.push\('\/privacy'\);/.test(block)
  );
  const style = /const LEGAL_LINK = (\{[^}]*\}) as const;/.exec(form)?.[1] ?? '';
  check('the two names are ink', /color: color\.ink/.test(style), style);
  check('at the line size and never underlined', style !== '' && !/fontSize|textDecoration|lineHeight/.test(style), style);
  const signupBranch = form.lastIndexOf('{signup ? (', start);
  const branchEnd = form.indexOf(') : null}', start);
  check(
    'the line is on sign-up only',
    signupBranch >= 0 && !/\) : (null\}|\()/.test(form.slice(signupBranch, start)) && branchEnd > close && !/\) : \(/.test(form.slice(close, branchEnd))
  );
  const button = form.indexOf("label={edit ? 'Save changes' : 'Continue'}");
  check('the line comes before Continue', start >= 0 && button > start);
  for (const [route, name] of [
    ['app/(auth)/terms.tsx', 'TERMS'],
    ['app/(auth)/privacy.tsx', 'PRIVACY'],
    ['app/(app)/profile/terms.tsx', 'TERMS'],
    ['app/(app)/profile/privacy.tsx', 'PRIVACY'],
  ]) {
    const page = existsSync(join(root, route)) ? source(route) : '';
    check(
      `${route} shows ${name} from lib/legal/documents`,
      new RegExp(`import \\{ ${name} \\} from '@/lib/legal/documents';`).test(page) && new RegExp(`<LegalScreen document=\\{${name}\\}`).test(page)
    );
  }
  const legal = source('components/legal/LegalScreen.tsx');
  check('the legal page shows the title, the date, the opening lines and the sections', ['{document.title}', 'document.effective', 'document.intro', 'document.sections'].every((piece) => legal.includes(piece)));
}

// ---- SET-01: the Settings rows ------------------------------------------------------------------
{
  const page = source('components/settings/SettingsScreen.tsx');
  const rows = [...page.matchAll(/<SettingsRow\s+([\s\S]*?)\/>/g)].map((match) => match[1]);
  const labels = rows.map((row) => /label="([^"]+)"/.exec(row)?.[1]);
  eq(
    'Settings lists its rows in this order',
    labels.join(', '),
    'Notifications, Blocked accounts, Archive, Password, Terms of Use, Privacy Policy, Delete account, Log out'
  );
  for (const row of rows) {
    const label = /label="([^"]+)"/.exec(row)?.[1];
    const target = /router\.push\('\/\(app\)\/profile\/([a-z-]+)'\)/.exec(row)?.[1];
    if (label === 'Log out') {
      check('Log out asks first, with the alert', /onPress=\{\(\) => setLoggingOut\(true\)\}/.test(row) && /<LogOutSheet visible=\{loggingOut\}/.test(page));
      continue;
    }
    check(`${label} opens a page that exists`, target !== undefined && existsSync(join(root, `app/(app)/profile/${target}.tsx`)), target);
  }
  eq('one red row', (page.match(/tone="destructive"/g) ?? []).length, 1);
  check('and it is Log out', rows.some((row) => /label="Log out"/.test(row) && /tone="destructive"/.test(row)));
}

// ---- ACC-07: the Forgot password page ---------------------------------------------------------
{
  const w = await newWorld('forgot password');
  const { ApiError } = w.client;
  const reset = await import(w.at('lib/auth/resetPage.ts'));
  const { codeLine, maybeSpentByLostReset, answerLost, tooMany } = reset;
  eq('a replaced code', codeLine('replaced'), reset.REPLACED);
  eq('a used code', codeLine('used'), reset.USED);
  eq('a code that is not 6 digits', codeLine('invalid'), reset.SHORT_CODE);
  eq('a wrong code', codeLine('wrong_code'), reset.WRONG_CODE);
  eq('no reason reads as a wrong code', codeLine(undefined), reset.WRONG_CODE);
  eq('a reason the app does not know reads as a wrong code', codeLine('something_new'), reset.WRONG_CODE);
  eq('the replaced line', reset.REPLACED, 'That code was replaced by a newer one. Use the newest code in your email.');
  eq('the used line', reset.USED, 'That code was already used. If your password changed, log in with the new one.');
  eq('the short code line', reset.SHORT_CODE, 'Enter the 6-digit code from the email.');
  check('the wrong code line says wrong or expired', reset.WRONG_CODE.startsWith("That code isn't right or has expired."));
  for (const line of [reset.REPLACED, reset.USED, reset.SHORT_CODE, reset.WRONG_CODE]) {
    check(`a code line never says whether the email has an account: ${line}`, !/account/i.test(line));
  }
  for (const reason of ['wrong_code', 'used', undefined, 'something_new']) {
    check(`after a lost reset answer, ${reason} may mean the password changed`, maybeSpentByLostReset(reason, true));
  }
  for (const reason of ['replaced', 'invalid']) check(`after a lost reset answer, ${reason} is still its own line`, !maybeSpentByLostReset(reason, true));
  for (const reason of ['wrong_code', 'used', 'replaced', 'invalid', undefined]) {
    check(`with no lost answer, ${reason} is its own line`, !maybeSpentByLostReset(reason, false));
  }
  for (const err of [new ApiError(0, 'network_unavailable'), new ApiError(500, 'server_error'), new ApiError(503, 'server_error'), new TypeError('x')]) {
    check(`an answer was lost: ${err.status ?? err.name}`, answerLost(err));
  }
  for (const status of [400, 401, 403, 404, 409, 429]) check(`an answer came: ${status}`, !answerLost(new ApiError(status, 'validation_failed')));
  check('too many: a 429 with no code', tooMany(new ApiError(429, 'server_error')));
  check('too many: rate_limited', tooMany(new ApiError(400, 'rate_limited')));
  check('not too many: a 500', !tooMany(new ApiError(500, 'server_error')));
  check('not too many: no answer', !tooMany(new TypeError('x')));

  const page = source('app/(auth)/forgot-password.tsx');
  check(
    'the page uses these rules and has none of its own',
    /import \{ answerLost, codeLine, maybeSpentByLostReset, SHORT_CODE, tooMany \} from '@\/lib\/auth\/resetPage';/.test(page) &&
      !/function (codeLine|maybeSpentByLostReset|tooMany|answerLost)\(/.test(page)
  );
  const send = page.slice(page.indexOf("await api.post('/auth/password/forgot'"), page.indexOf('} catch (err) {', page.indexOf("await api.post('/auth/password/forgot'")));
  check('Send code moves to the code step after any answer, with no branch on it', /setStep\('code'\);/.test(send) && !/\bif \(|\?|return/.test(send.replace(/\/\/[^\n]*/g, '')));
  check('the notice never says whether the email has an account', /setNotice\(`We sent a code to \$\{typed\}\.`\);/.test(page));
  check('a spent code is never sent again', (page.match(/if \(spentRef\.current \|\| sendingRef\.current\) return;/g) ?? []).length === 2);
  check('the code is marked spent before the new session is kept', /spentRef\.current = true;\s*try \{\s*(\/\/[^\n]*\n\s*)*await signIn\(result\);/.test(page));
  check(
    'a code that changed meanwhile goes back to the code step',
    /setCodeError\(codeLine\(err\.reason\)\);\s*\}\s*(\/\/[^\n]*\n\s*)*setStep\('code'\);/.test(page)
  );
  check(
    'a lost reset answer says the password may have changed',
    /if \(answerLost\(err\)\) maybeChangedRef\.current = true;\s*setFormError\(MAYBE_CHANGED\);/.test(page) &&
      /const MAYBE_CHANGED = 'Something went wrong\. If your password did change, log in with the new one\.';/.test(page)
  );
  check('a new code forgets the lost answer', /maybeChangedRef\.current = false;\s*setStep\('code'\);/.test(page));
  const login = source('app/(auth)/login.tsx');
  check(
    'Forgot password takes an email typed on Log in along, and never a username',
    /const typed = login\.trim\(\);\s*router\.push\(EMAIL_PATTERN\.test\(typed\) \? \{ pathname: '\/forgot-password', params: \{ email: typed \} \} : '\/forgot-password'\);/.test(login)
  );
  check(
    'the page fills in only an email it was given',
    /const \[carried\] = useState\(\(\) => carriedEmail\(params\.email\)\);\s*const \[email, setEmail\] = useState\(carried\);/.test(page) &&
      /const typed = typeof param === 'string' \? param\.trim\(\) : '';\s*return EMAIL_PATTERN\.test\(typed\) \? typed : '';/.test(page)
  );
  check('the keyboard opens by itself only for an empty email box', /autoFocus=\{carried === ''\}/.test(page));
}

// ---- Section 10 Privacy: no location, contacts or tracking; no tracking tools -----------------
{
  const BANNED = /(^|[@/-])(sentry|firebase|crashlytics|amplitude|segment|mixpanel|bugsnag|datadog|posthog|appsflyer|admob|google-mobile-ads|fbsdk|newrelic|rollbar)([/-]|$)/i;
  const BANNED_EXPO = ['expo-location', 'expo-contacts', 'expo-tracking-transparency', 'expo-ads-admob', 'expo-facebook', 'expo-firebase-analytics', 'expo-insights'];
  const pkg = JSON.parse(source('package.json'));
  const deps = Object.keys({ ...pkg.dependencies, ...pkg.devDependencies });
  check('package.json lists dependencies', deps.length > 10);
  const badDeps = deps.filter((name) => BANNED.test(name) || BANNED_EXPO.includes(name));
  eq('the app ships no location, contacts, ads, analytics, crash reporting or tracking package', badDeps.join(','), '');
  const app = JSON.parse(source('app.json')).expo;
  const plugins = (app.plugins ?? []).map((plugin) => (Array.isArray(plugin) ? plugin[0] : plugin));
  const badPlugins = plugins.filter((name) => BANNED.test(name) || BANNED_EXPO.includes(name));
  eq('app.json adds no such plugin', badPlugins.join(','), '');
  const askedIn = (keys: string[]) => keys.filter((key) => /^NSLocation|^NSContacts|^NSUserTrackingUsageDescription$/.test(key));
  eq('app.json asks for no location, contacts or tracking permission', askedIn(Object.keys(app.ios?.infoPlist ?? {})).join(','), '');
  const goMod = source('go.mod');
  const modules = [...goMod.matchAll(/^\s*(?:require\s+)?([a-z0-9.\-]+\.[a-z]+\/[^\s]+)\s+v[^\s]+/gim)].map((match) => match[1]);
  check('go.mod lists its modules', modules.length > 5);
  eq('the server ships no analytics, crash reporting or tracking module', modules.filter((path) => BANNED.test(path) || /segmentio|getsentry|DataDog/i.test(path)).join(','), '');

  const plist = 'ios/RNDMRoll/Info.plist';
  if (existsSync(join(root, plist))) {
    const keys = [...source(plist).matchAll(/<key>([^<]+)<\/key>/g)].map((match) => match[1]);
    eq('the built app asks for no location, contacts or tracking permission', askedIn(keys).join(','), '');
    const usage = keys.filter((key) => /UsageDescription$/.test(key));
    // The camera and the photo library; the microphone and Face ID strings come from Expo plugins
    // and are on the launch list to take out (PRD section 14).
    const allowed = ['NSCameraUsageDescription', 'NSPhotoLibraryUsageDescription', 'NSPhotoLibraryAddUsageDescription', 'NSMicrophoneUsageDescription', 'NSFaceIDUsageDescription'];
    eq('the built app asks only for the camera and the photo library (and the two launch-list strings)', usage.filter((key) => !allowed.includes(key)).join(','), '');
  } else {
    console.log(`note: ${plist} is not here (expo prebuild makes it), so only app.json was checked for permissions`);
  }
}

await settle();
console.log(`${checks} checks, ${failures} failures`);
process.exit(failures === 0 ? 0 : 1);
