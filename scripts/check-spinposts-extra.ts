// @ts-nocheck
/**
 * Throwaway checks for the Board's words and its Late marker (BRD-02, BRD-03, BRD-04, SPIN-06), the
 * 8 PM reminder and the morning note (SPIN-11, SPIN-14), the phone's request limit and who may end a
 * session (section 10, Reliability), the Spin page's fixed words and numbers (SPIN-02, SPIN-03,
 * SPIN-04, SPIN-06, SPIN-08, SPIN-09, POST-05), the photo settings (section 10, Performance), how a
 * report ends, and the server's moderation answers read through the app's own readers (moderation,
 * 2026-11-17; the answers are internal/httpapi/testdata/moderation_contract.json), with a second
 * look sent through the real client (2026-11-19).
 * Not part of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-spinposts-extra.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts.)
 *
 * The reminder, the API client, the session store and the entry calls import React Native and Expo
 * modules that Node cannot load. They are loaded as they are, with only those modules swapped for
 * small stand-ins (registerHooks below), so their own code runs here. The screens have no React here,
 * so the lines that matter are pinned as written, as in the other check scripts.
 */
import { existsSync, readFileSync } from 'node:fs';
import * as nodeModule from 'node:module';
import { fileURLToPath } from 'node:url';
import {
  LOCKED_BODY,
  NO_FOLLOWS_LINE,
  NO_FOLLOWS_TITLE,
  OVERLINE,
  SKIPPED_LINE,
  cardSpoken,
  commentCount,
  lockedButton,
  lockedFootnote,
  postedLine,
  starsFigure,
} from '../lib/board/text.ts';

// The reminder works in the phone's own time. New York changes its clocks on November 1, 2026, so
// the 30 evenings below cross a clock change. Set before any Date is made.
process.env.TZ = 'America/New_York';

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

const root = new URL('../', import.meta.url);
const source = (rel: string) => readFileSync(fileURLToPath(new URL(rel, root)), 'utf8');
const NBSP = ' ';
const DASHES = /[–—]/;

/**
 * The text of the function declared at `start` (for example 'function headingFor('), from that word
 * to its closing brace, or null when it is not there. Strings and comments are skipped while the
 * braces are counted.
 */
function functionText(text: string, start: string): string | null {
  const at = text.indexOf(start);
  if (at < 0) return null;
  const open = text.indexOf('{', text.indexOf(')', at));
  if (open < 0) return null;
  let depth = 0;
  for (let i = open; i < text.length; i += 1) {
    const ch = text[i];
    if (ch === "'" || ch === '"' || ch === '`') {
      for (i += 1; i < text.length && text[i] !== ch; i += 1) if (text[i] === '\\') i += 1;
    } else if (ch === '/' && text[i + 1] === '/') {
      i = text.indexOf('\n', i);
      if (i < 0) return null;
    } else if (ch === '{') {
      depth += 1;
    } else if (ch === '}') {
      depth -= 1;
      if (depth === 0) return text.slice(at, i + 1);
    }
  }
  return null;
}

// ---- stand-ins for the phone's modules ------------------------------------------------------------
// One shared record the stand-ins write to, so a check can see what the real code asked for.
const fake = {
  secure: new Map<string, string>(),
  log: [] as unknown[][],
  scheduled: [] as any[],
  uploads: [] as any[],
  notifications: null as any,
};
globalThis.__spinpostsFake = fake;

const STAND_INS: Record<string, string> = {
  react: `
    export const useEffect = () => {};
    export const useCallback = (f) => f;
    export const useContext = () => undefined;
    export const useRef = (v) => ({ current: v });
    export const useState = (v) => [v, () => {}];
    export const useSyncExternalStore = () => null;
    export const createContext = () => ({ Provider: null });
    export const createElement = () => null;`,
  'react-native': `
    export const AppState = { currentState: 'active', addEventListener: () => ({ remove() {} }) };
    export const Platform = { OS: 'ios' };`,
  'expo-router': `export const useRouter = () => ({ navigate() {} });`,
  'expo-secure-store': `
    const f = globalThis.__spinpostsFake;
    export const WHEN_UNLOCKED_THIS_DEVICE_ONLY = 'when-unlocked-this-device-only';
    export async function getItemAsync(k) { return f.secure.has(k) ? f.secure.get(k) : null; }
    export async function setItemAsync(k, v) { f.log.push(['secure.set', k, v]); f.secure.set(k, v); }
    export async function deleteItemAsync(k) { f.secure.delete(k); }`,
  'expo-image': `export const Image = { clearMemoryCache: async () => {}, clearDiskCache: async () => {} };`,
  'expo-constants': `export default {};`,
  'expo-file-system/legacy': `
    const f = globalThis.__spinpostsFake;
    export const FileSystemUploadType = { BINARY_CONTENT: 0, MULTIPART: 1 };
    export async function getInfoAsync(uri) { return { exists: true, size: 4321, uri }; }
    export async function uploadAsync(url, uri, options) { f.uploads.push({ url, uri, options }); return { status: 200, body: '', headers: {} }; }`,
  '@/lib/notifications/module': `
    const f = globalThis.__spinpostsFake;
    let queue = Promise.resolve();
    export function loadNotifications() { return f.notifications; }
    export function queueScheduleChange(change) {
      const next = queue.then(async () => {
        if (f.notifications === null) return;
        try { await change(f.notifications); } catch (err) { f.log.push(['queue.error', String(err)]); }
      });
      queue = next;
      return next;
    }
    export function cancelScheduledNotifications() { return queueScheduleChange((api) => api.cancelAllScheduledNotificationsAsync()); }
    export function listenForTaps() { return () => {}; }`,
  '@/lib/notifications/pushToken': `
    export function beginSignIn() { return () => {}; }
    export async function deleteLeftPushToken() {}
    export function logOutFailed() {}
    export async function registerPushToken() {}
    export async function unregisterPushToken() { return null; }`,
  '@/lib/wheel/todayStore': `
    export function useTodaySnapshot() { return { today: null }; }
    export function applyToday() {}`,
};

nodeModule.registerHooks({
  resolve(specifier, context, nextResolve) {
    if (Object.hasOwn(STAND_INS, specifier)) {
      return { url: `data:text/javascript,${encodeURIComponent(STAND_INS[specifier])}`, shortCircuit: true };
    }
    if (specifier.startsWith('@/')) {
      for (const ext of ['.ts', '.tsx']) {
        const url = new URL(`${specifier.slice(2)}${ext}`, root);
        if (existsSync(fileURLToPath(url))) return { url: url.href, shortCircuit: true };
      }
    }
    return nextResolve(specifier, context);
  },
});

const load = (rel: string) => import(new URL(rel, root).href);
const client = await load('lib/api/client.ts');
const store = await load('lib/session/store.ts');
const reminder = await load('lib/notifications/reminder.ts');
const entries = await load('lib/api/entries.ts');
const { ApiError, api, isRetryable } = client;

// ---- the Board's words (BRD-02, BRD-03, BRD-04) ----------------------------------------------------
{
  eq('the locked button logs the category in lower case', lockedButton('log', 'Meal'), 'Log your meal');
  eq('a category of two words is lower case too', lockedButton('log', 'Board games'), 'Log your board games');
  eq('with no category the button logs the spin', lockedButton('log', null), 'Log your spin');
  eq('before the spin the button says Spin first', lockedButton('spin', 'Meal'), 'Spin first');
  eq('Spin first with no category', lockedButton('spin', null), 'Spin first');
  eq('while waiting there is no button', lockedButton('wait', 'Meal'), null);

  eq('while waiting the footnote says when the next spin opens', lockedFootnote('wait', false), `Your next spin opens at 8:00${NBSP}PM.`);
  eq('while waiting, late or not', lockedFootnote('wait', true), `Your next spin opens at 8:00${NBSP}PM.`);
  eq('a late post has its footnote', lockedFootnote('log', true), `Fashionably late is fine until 8:00${NBSP}PM.`);
  eq('an on-time post has none', lockedFootnote('log', false), null);
  eq('Spin first has none, late', lockedFootnote('spin', true), null);
  eq('Spin first has none', lockedFootnote('spin', false), null);

  eq('nobody posted yet', postedLine(0), 'No one you follow has posted yet.');
  eq('one person posted', postedLine(1), '1 person you follow posted today.');
  eq('three people posted', postedLine(3), '3 people you follow posted today.');
  eq('a count below zero reads as nobody', postedLine(-1), 'No one you follow has posted yet.');

  eq('BRD-03, word for word', SKIPPED_LINE, 'You skipped today. Posts from people you follow open after your next post.');
  eq('BRD-04 heading, word for word', NO_FOLLOWS_TITLE, "See your friends' spins");
  eq('BRD-04 line, word for word', NO_FOLLOWS_LINE, 'Follow people you know. Their posts show here on the days you post too.');

  eq('7 stars read 3.5', starsFigure(7), '3.5');
  eq('10 stars read 5.0', starsFigure(10), '5.0');
  eq('1 star reads 0.5', starsFigure(1), '0.5');
  eq('below the range reads 0.0', starsFigure(-1), '0.0');
  eq('above the range reads 5.0', starsFigure(11), '5.0');

  eq('no comments say nothing', commentCount(0), '');
  eq('an unknown count says nothing', commentCount(null), '');
  eq('one comment', commentCount(1), '1 comment');
  eq('three comments', commentCount(3), '3 comments');

  const card = { title: 'Aftersun', stars: 10, category: 'Movie', by: 'Maya' };
  eq('VoiceOver reads an on-time card', cardSpoken({ ...card, late: false }), 'Aftersun. 5.0 out of 5 stars. Movie. By Maya.');
  eq('VoiceOver reads a late card', cardSpoken({ ...card, late: true }), 'Aftersun. 5.0 out of 5 stars. Movie. By Maya. Posted late.');

  const words = [
    OVERLINE,
    LOCKED_BODY,
    SKIPPED_LINE,
    NO_FOLLOWS_TITLE,
    NO_FOLLOWS_LINE,
    postedLine(0),
    postedLine(1),
    postedLine(3),
    cardSpoken({ ...card, late: true }),
  ];
  for (const action of ['log', 'spin', 'wait']) {
    words.push(lockedButton(action, 'Meal') ?? '', lockedFootnote(action, true) ?? '', lockedFootnote(action, false) ?? '');
  }
  for (const line of words) check('no em dash or en dash on the Board', !DASHES.test(line), line);

  const blocks = source('components/board/BoardBlocks.tsx');
  check('the locked card shows up to four names', /const names = locked\.names\.slice\(0, 4\);/.test(blocks));
  check(
    'the locked card takes its words from the rules above',
    /lockedButton\(locked\.action, locked\.category\)/.test(blocks) &&
      /lockedFootnote\(locked\.action, locked\.post_late\)/.test(blocks) &&
      /postedLine\(locked\.posted_count\)/.test(blocks)
  );
  const skipped = functionText(blocks, 'function SkippedBlock(') ?? '';
  check('the skipped day shows its line with Go to Spin', skipped.includes('{SKIPPED_LINE}') && /<TextButton label="Go to Spin" onPress=\{onGoToSpin\} \/>/.test(skipped));
  const noFollows = functionText(blocks, 'function NoFollowsBlock(') ?? '';
  check(
    'following nobody shows the heading, the line and Find people',
    noFollows.includes('{NO_FOLLOWS_TITLE}') && noFollows.includes('{NO_FOLLOWS_LINE}') && /label="Find people"/.test(noFollows)
  );
}

// ---- Late on the Board (SPIN-06, BRD-01) ----------------------------------------------------------
{
  const postCard = source('components/board/PostCard.tsx');
  check('a late card shows Late beside its stars', /card\.late \? `\$\{starsFigure\(card\.stars\)\}  Late` : starsFigure\(card\.stars\)/.test(postCard));
  check('VoiceOver is told the card is late', /cardSpoken\(\{[\s\S]{0,200}late: card\.late,?[\s\S]{0,40}\}\)/.test(postCard));
}

// ---- the 8 PM reminder and the morning note (SPIN-11, SPIN-14) ------------------------------------
{
  check(
    'the reminder runs in New York time here, with its clock change',
    new Date(2026, 9, 20).getTimezoneOffset() !== new Date(2026, 10, 5).getTimezoneOffset(),
    `offsets ${new Date(2026, 9, 20).getTimezoneOffset()} and ${new Date(2026, 10, 5).getTimezoneOffset()}`
  );
  const {
    POST_NOTE_ID,
    REMINDER_BODY,
    REMINDER_DAYS_AHEAD,
    REMINDER_HOUR,
    REMINDER_ID,
    REMINDER_MINUTE,
    REMINDER_TITLE,
    PERMISSION_OFF_MESSAGE,
    askAtStartOnce,
    isReminderId,
    noteNextOpen,
    notePostNote,
    permissionFrom,
    permissionLine,
    rebuildReminder,
    reminderContent,
    reminderDates,
    reminderTrigger,
    shouldSchedule,
    switchShowsOn,
  } = reminder;

  eq('the reminder title, word for word', REMINDER_TITLE, "Spin o'clock");
  eq('the reminder line, word for word', REMINDER_BODY, 'Your wheel is ready when you are.');
  check('the reminder opens the Spin tab', JSON.stringify(reminderContent()) === JSON.stringify({ title: REMINDER_TITLE, body: REMINDER_BODY, data: { url: '/(app)/today' } }));
  check('at 8:00 PM', REMINDER_HOUR === 20 && REMINDER_MINUTE === 0);
  check('the daily trigger is 8:00 PM', JSON.stringify(reminderTrigger('daily')) === JSON.stringify({ type: 'daily', hour: 20, minute: 0 }));
  eq('up to 30 evenings ahead', REMINDER_DAYS_AHEAD, 30);

  const at = (m: number, d: number, h: number, min = 0, s = 0) => new Date(2026, m - 1, d, h, min, s).getTime();
  const evenings = (dates: Date[]) => dates.map((x) => `${x.getMonth() + 1}/${x.getDate()} ${x.getHours()}:${String(x.getMinutes()).padStart(2, '0')}`);
  const oneAnEvening = (dates: Date[], first: number) =>
    dates.every((x, i) => {
      const want = new Date(first);
      want.setDate(want.getDate() + i);
      return x.getFullYear() === want.getFullYear() && x.getMonth() === want.getMonth() && x.getDate() === want.getDate() && x.getHours() === 20 && x.getMinutes() === 0 && x.getSeconds() === 0;
    });

  // 3 PM, the spin opens tonight: 30 evenings from tonight, each at 8:00 PM, across November 1.
  let dates = reminderDates(at(10, 20, 15), at(10, 20, 20));
  eq('30 evenings', dates.length, 30);
  check('one a night from tonight, every one at 8:00 PM phone time', oneAnEvening(dates, at(10, 20, 20)), evenings(dates).join(', '));
  // Oct 31 at 8 PM to Nov 1 at 8 PM: the clocks go back at 2 AM between them.
  check(
    'the clock change keeps 8:00 PM (the gap in real time is 25 hours across it)',
    dates[11].getDate() === 31 && dates[12].getTime() - dates[11].getTime() === 25 * 3600 * 1000,
    `${dates[11]} to ${dates[12]}`
  );
  check('none in the past', dates.every((x) => x.getTime() > at(10, 20, 15)));

  // 9 PM: tonight's 8 PM has passed, so the first is tomorrow.
  dates = reminderDates(at(10, 20, 21), at(10, 20, 20));
  check('from 9 PM the first evening is tomorrow', dates.length === 30 && oneAnEvening(dates, at(10, 21, 20)), evenings(dates).slice(0, 2).join(', '));
  check('none in the past, from 9 PM', dates.every((x) => x.getTime() > at(10, 20, 21)));

  // A first spin made at 10 AM: the next opening is tomorrow at 8 PM, so tonight is skipped.
  dates = reminderDates(at(10, 20, 10, 5), at(10, 21, 20));
  check('the evening of a spin already made is skipped', dates.length === 30 && oneAnEvening(dates, at(10, 21, 20)), evenings(dates).slice(0, 2).join(', '));

  // The server's 8 PM may be a little after the phone's: a minute of slack keeps tonight.
  dates = reminderDates(at(10, 20, 15), at(10, 20, 20, 0, 30));
  check('a next opening 30 seconds after 8 PM still keeps tonight', dates.length === 30 && oneAnEvening(dates, at(10, 20, 20)));

  for (const permission of ['granted', 'denied', 'undetermined', 'unavailable']) {
    for (const on of [true, false]) {
      const want = on && permission === 'granted';
      eq(`scheduled only with the switch on and permission granted: ${on} ${permission}`, shouldSchedule(on, permission), want);
      eq(`the switch shows on only then: ${on} ${permission}`, switchShowsOn(on, permission), want);
    }
    eq(`the line under the switch: ${permission}`, permissionLine(permission), permission === 'denied' ? PERMISSION_OFF_MESSAGE : '');
  }
  eq('the line under the switch, word for word', PERMISSION_OFF_MESSAGE, 'Turn on notifications for RNDMRoll in Settings.');
  eq('granted', permissionFrom({ granted: true, status: 'granted' }), 'granted');
  eq('never answered', permissionFrom({ granted: false, status: 'undetermined' }), 'undetermined');
  eq('turned off', permissionFrom({ granted: false, status: 'denied' }), 'denied');

  check('the reminder ids are the reminder', isReminderId(REMINDER_ID) && isReminderId(`${REMINDER_ID}-0`) && isReminderId(`${REMINDER_ID}-29`));
  check('the morning note is the reminder too', isReminderId(POST_NOTE_ID));
  check('any other id is not', !isReminderId('rndmroll-push-comment') && !isReminderId(`${REMINDER_ID}x`) && !isReminderId(''));

  // The real schedule, on a stand-in for the phone's notification module.
  const api = {
    SchedulableTriggerInputTypes: { DAILY: 'daily', DATE: 'date' },
    permission: { granted: false, status: 'undetermined' },
    askAnswer: { granted: true, status: 'granted' },
    async getPermissionsAsync() {
      return this.permission;
    },
    async requestPermissionsAsync() {
      fake.log.push(['ask']);
      this.permission = this.askAnswer;
      return this.askAnswer;
    },
    async cancelAllScheduledNotificationsAsync() {
      fake.log.push(['cancel']);
      fake.scheduled = [];
    },
    async scheduleNotificationAsync(request) {
      fake.log.push(['schedule', request.identifier]);
      fake.scheduled.push(request);
      return request.identifier;
    },
    setNotificationHandler() {},
  };
  fake.notifications = api;
  const NOW = at(10, 20, 15);
  const realNow = Date.now;
  Date.now = () => NOW;
  const ids = () => fake.scheduled.map((r) => r.identifier);
  const ASKED = 'rndmroll.reminder_asked';
  const ON = 'rndmroll.reminder_on';

  // The first signed-in start asks once, and marks it asked before asking.
  await askAtStartOnce();
  const markAt = fake.log.findIndex((e) => e[0] === 'secure.set' && e[1] === ASKED && e[2] === 'true');
  const askAt = fake.log.findIndex((e) => e[0] === 'ask');
  check('the first start asks for permission', askAt >= 0);
  check('it is marked asked before the ask', markAt >= 0 && markAt < askAt, JSON.stringify(fake.log));
  eq('allowing turns the reminder on', fake.secure.get(ON), 'true');
  const asks = fake.log.filter((e) => e[0] === 'ask').length;
  await askAtStartOnce();
  eq('a later start never asks again', fake.log.filter((e) => e[0] === 'ask').length, asks);

  // Nobody signed in: nothing is scheduled.
  store.setAccessToken(null);
  fake.log.length = 0;
  await rebuildReminder();
  check('signed out, the schedule is cleared and nothing is set', fake.log.some((e) => e[0] === 'cancel') && fake.scheduled.length === 0, JSON.stringify(fake.log));

  // Signed in, the next opening not known yet: one reminder every evening.
  store.setAccessToken('session-token');
  fake.log.length = 0;
  await rebuildReminder();
  check(
    'before the first Today: one daily reminder at 8 PM',
    ids().join() === REMINDER_ID && JSON.stringify(fake.scheduled[0].trigger) === JSON.stringify({ type: 'daily', hour: 20, minute: 0 }) &&
      fake.scheduled[0].content.title === REMINDER_TITLE && fake.scheduled[0].content.body === REMINDER_BODY,
    JSON.stringify(fake.scheduled)
  );
  check('every rebuild clears the schedule first', fake.log[0]?.[0] === 'cancel', JSON.stringify(fake.log));

  // Today gives the next opening: each of the 30 evenings on its own.
  noteNextOpen(new Date(at(10, 20, 20)).toISOString());
  await rebuildReminder();
  const want = reminderDates(NOW, at(10, 20, 20)).map((x) => x.getTime());
  check(
    'with the next opening known: 30 evenings, each at its date',
    fake.scheduled.length === 30 &&
      fake.scheduled.every((r, i) => r.identifier === `${REMINDER_ID}-${i}` && r.trigger.type === 'date' && r.trigger.date.getTime() === want[i] && r.content.title === REMINDER_TITLE),
    ids().join()
  );

  // A spin waiting for its post: the morning note at 10 AM, two hours before late.
  notePostNote(new Date(at(10, 21, 12)).toISOString(), 'Movie');
  await rebuildReminder();
  const note = fake.scheduled.find((r) => r.identifier === POST_NOTE_ID);
  check(
    'the morning note comes at 10 AM with its words',
    note !== undefined && note.trigger.type === 'date' && note.trigger.date.getTime() === at(10, 21, 10) &&
      note.content.title === 'Beat the noon bell' && note.content.body === 'Post your movie pick to stay on time.',
    JSON.stringify(note)
  );
  eq('the evenings stay with the note', fake.scheduled.length, 31);

  // A post or a skip clears late_from: the note goes at the next rebuild.
  notePostNote(null, null);
  await rebuildReminder();
  check('posted or skipped: no morning note', !ids().includes(POST_NOTE_ID) && fake.scheduled.length === 30, ids().join());

  // The switch off, or the permission turned off: nothing.
  fake.secure.set(ON, 'false');
  await rebuildReminder();
  eq('with the switch off nothing is scheduled', fake.scheduled.length, 0);
  fake.secure.set(ON, 'true');
  api.permission = { granted: false, status: 'denied' };
  await rebuildReminder();
  eq('with the permission off nothing is scheduled', fake.scheduled.length, 0);
  api.permission = { granted: true, status: 'granted' };
  await rebuildReminder();
  eq('both on again: the evenings are back', fake.scheduled.length, 30);
  check('nothing failed inside the schedule changes', !fake.log.some((e) => e[0] === 'queue.error'), JSON.stringify(fake.log.filter((e) => e[0] === 'queue.error')));

  Date.now = realNow;
  fake.notifications = null;
}

// ---- requests and sessions (section 10, Reliability) ----------------------------------------------
{
  const { isDefinitiveAuthFailure, registerSessionHandlers, setAccessToken } = store;
  for (const [status, code] of [
    [401, 'token_invalid'],
    [401, 'token_expired'],
    [403, 'profile_incomplete'],
    [400, 'token_invalid'],
    [400, 'token_expired'],
  ]) {
    eq(`a definite no: ${status} ${code}`, isDefinitiveAuthFailure(new ApiError(status, code)), true);
  }
  for (const [status, code] of [
    [0, 'network_unavailable'],
    [429, 'rate_limited'],
    [500, 'server_error'],
    [503, 'server_error'],
    [404, 'not_found'],
  ]) {
    eq(`not a definite no: ${status} ${code}`, isDefinitiveAuthFailure(new ApiError(status, code)), false);
  }
  eq('an error that is not from the server is not a definite no', isDefinitiveAuthFailure(new TypeError('Network request failed')), false);
  for (const status of [0, 429, 500, 503]) eq(`try again after ${status}`, isRetryable(new ApiError(status, 'server_error')), true);
  for (const status of [400, 401, 404, 409]) eq(`no use trying again after ${status}`, isRetryable(new ApiError(status, 'validation_failed')), false);
  eq('try again after an error that is not from the server', isRetryable(new TypeError('Network request failed')), true);

  // The real client, with a session signed in and its refresh and sign out handlers counted.
  const calls = { refresh: 0, signOut: 0 };
  let refreshWith = async () => 'fresh-token';
  registerSessionHandlers({
    refreshSession: async () => {
      calls.refresh += 1;
      return refreshWith();
    },
    signOut: async () => {
      calls.signOut += 1;
    },
  });
  setAccessToken('session-token');
  const realFetch = globalThis.fetch;
  const sent = [];
  const answer = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  const outcome = async (run) => {
    try {
      return { value: await run() };
    } catch (err) {
      return { err };
    }
  };
  const isError = (got, status, code) => got.err instanceof ApiError && got.err.status === status && got.err.code === code;

  // A server that never answers: aborted after 10 seconds (the timer fires at once here).
  {
    const realSetTimeout = globalThis.setTimeout;
    const realClearTimeout = globalThis.clearTimeout;
    const delays = [];
    globalThis.setTimeout = (fn, ms) => {
      delays.push(ms);
      queueMicrotask(fn);
      return 0;
    };
    globalThis.clearTimeout = () => {};
    globalThis.fetch = (url, init) =>
      new Promise((_, reject) => {
        sent.push({ url, init });
        init.signal.addEventListener('abort', () => reject(new DOMException('The operation was aborted.', 'AbortError')));
      });
    const got = await outcome(() => api.get('/today'));
    globalThis.setTimeout = realSetTimeout;
    globalThis.clearTimeout = realClearTimeout;
    check('every request has a 10 second limit', delays.includes(10_000), JSON.stringify(delays));
    check('the limit aborts the request itself', sent[0]?.init?.signal?.aborted === true);
    check('a request over the limit is no connection', isError(got, 0, 'network_unavailable'), String(got.err));
  }

  globalThis.fetch = async () => {
    throw new TypeError('Network request failed');
  };
  check('a lost connection is no connection', isError(await outcome(() => api.get('/today')), 0, 'network_unavailable'));

  for (const [status, code] of [
    [500, 'server_error'],
    [503, 'server_error'],
    [429, 'rate_limited'],
  ]) {
    globalThis.fetch = async () => answer(status, { error: code });
    check(`${status} comes back as itself`, isError(await outcome(() => api.get('/today')), status, code));
  }
  globalThis.fetch = async () => answer(403, { error: 'profile_incomplete' });
  check('403 comes back as itself', isError(await outcome(() => api.get('/today')), 403, 'profile_incomplete'));
  check('no connection, a timeout, 403, 429 or a 5xx never logs out or refreshes', calls.signOut === 0 && calls.refresh === 0, JSON.stringify(calls));

  // An expired token: one refresh. A refresh that could not reach the server is not a log out.
  const expiredThen = (after) => {
    let n = 0;
    return async (url, init) => {
      sent.push({ url, init });
      n += 1;
      return n === 1 ? answer(401, { error: 'token_expired' }) : after;
    };
  };
  refreshWith = async () => {
    throw new ApiError(0, 'network_unavailable');
  };
  globalThis.fetch = expiredThen(answer(200, { ok: true }));
  let got = await outcome(() => api.get('/today'));
  check('an expired token is refreshed once', calls.refresh === 1, JSON.stringify(calls));
  check('a refresh with no connection comes back as no connection', isError(got, 0, 'network_unavailable'), String(got.err));
  eq('and never logs out', calls.signOut, 0);

  // A refresh the server refused: the request's own 401 comes back (the store has signed out).
  refreshWith = async () => {
    throw new ApiError(401, 'token_invalid');
  };
  globalThis.fetch = expiredThen(answer(200, { ok: true }));
  got = await outcome(() => api.get('/today'));
  check('a refused refresh gives the request its own 401', isError(got, 401, 'token_expired'), String(got.err));

  // The control: a refresh that works sends the request again with the new token.
  refreshWith = async () => 'fresh-token';
  sent.length = 0;
  globalThis.fetch = expiredThen(answer(200, { ok: true }));
  got = await outcome(() => api.get('/today'));
  check(
    'a refresh that works sends the request again with the new token',
    got.value?.ok === true && sent.length === 2 && sent[1].init.headers.Authorization === 'Bearer fresh-token',
    JSON.stringify(sent.map((s) => s.init.headers))
  );

  // The server says the account is disabled: that is a definite no.
  globalThis.fetch = async () => answer(401, { error: 'account_disabled' });
  got = await outcome(() => api.get('/today'));
  check('a disabled account logs out', isError(got, 401, 'account_disabled') && calls.signOut === 1, JSON.stringify(calls));

  const storeText = source('lib/session/store.ts');
  check(
    'the launch restore shows offline unless the answer was a definite no',
    /setStatus\(!isDefinitiveAuthFailure\(err\) && stillHasTokens \? 'offline' : 'unauthenticated'\);/.test(storeText)
  );

  // ---- photo uploads (POST-03, section 10 Performance and Reliability) ----
  for (const [type, ext] of [
    ['image/png', 'png'],
    ['image/webp', 'webp'],
    ['image/jpeg', 'jpg'],
  ]) {
    sent.length = 0;
    fake.uploads.length = 0;
    const key = `entries/u1/abc.${ext}`;
    globalThis.fetch = async (url, init) => {
      sent.push({ url, init });
      return answer(200, { upload_url: `https://storage.example.test/put/${ext}?sig=1`, photo_key: key, expires_in: 300 });
    };
    const out = await outcome(() => entries.uploadEntryPhoto(`file:///picked/photo.${ext}`, type));
    eq(`${type}: the upload gives back its key`, out.value, key);
    const ticket = sent[0] ? JSON.parse(sent[0].init.body) : null;
    check(
      `${type}: the ticket asks for the picked file's own type and size`,
      sent.length === 1 && String(sent[0].url).endsWith('/v1/entry-photos/upload-url') && ticket?.content_type === type && ticket?.content_length === 4321,
      JSON.stringify(ticket)
    );
    const put = fake.uploads[0];
    check(
      `${type}: the bytes go straight to storage with that type, no limit and no sign-in header`,
      fake.uploads.length === 1 &&
        put.url === `https://storage.example.test/put/${ext}?sig=1` &&
        put.uri === `file:///picked/photo.${ext}` &&
        JSON.stringify(put.options) === JSON.stringify({ httpMethod: 'PUT', headers: { 'Content-Type': type }, uploadType: 0 }),
      JSON.stringify(put)
    );
  }
  globalThis.fetch = realFetch;

  eq('day_closed, word for word (SPIN-06)', entries.entryErrorMessage(new ApiError(409, 'day_closed')).text, "This day has closed. It can't be posted or skipped any more.");
  eq('day_closed ends the attempt', entries.entryErrorMessage(new ApiError(409, 'day_closed')).terminal, true);
  eq('day_settled, word for word', entries.entryErrorMessage(new ApiError(409, 'day_settled')).text, 'This day already has a post or a skip.');
}

// ---- photo settings on the phone (section 10, Performance) ----------------------------------------
{
  const field = source('components/entry/PhotoField.tsx');
  const comments = source('components/board/Comments.tsx');
  const count = (text: string, re: RegExp) => (text.match(re) ?? []).length;
  const quality = /quality: 0\.8,/g;
  const compatible = /preferredAssetRepresentationMode: ImagePicker\.UIImagePickerPreferredAssetRepresentationMode\.Compatible,/g;
  check('the post camera and library both save at 80%', count(field, quality) === 2 && count(field, /quality:/g) === 2);
  check('the post camera and library both save HEIC as JPEG (Compatible)', count(field, compatible) === 2);
  check('both pickers of the post are those two', /launchCameraAsync\(\{[\s\S]{0,200}quality: 0\.8/.test(field) && /launchImageLibraryAsync\(\{[\s\S]{0,200}quality: 0\.8/.test(field));
  check('the comment photo saves at 80%, Compatible', count(comments, quality) === 1 && count(comments, /quality:/g) === 1 && count(comments, compatible) === 1);
  check(
    "the comment photo's camera and library share those settings",
    /source === 'camera' \? await ImagePicker\.launchCameraAsync\(options\) : await ImagePicker\.launchImageLibraryAsync\(options\)/.test(comments)
  );
  check('a picked photo keeps its own type', /onAdd\(\{ uri: asset\.uri, mimeType: asset\.mimeType \?\? null, source \}\)/.test(field));
  check('a comment photo keeps its own type', /setPhoto\(\{ uri: asset\.uri, mimeType: asset\.mimeType \?\? 'image\/jpeg' \}\)/.test(comments));
  for (const rel of ['components/entry/useCreateEntry.ts', 'components/entry/useUpdateEntry.ts']) {
    check(`${rel} uploads each photo with its own type`, /uploadEntryPhoto\(photo\.uri, photo\.mimeType \?\? 'image\/jpeg'\)/.test(source(rel)));
  }
  check('the comment photo uploads with its own type', /uploadEntryPhoto\(photo\.uri, photo\.mimeType\)/.test(comments));
  // The second look's photos (2026-11-19): the comment photo's settings, up to 3.
  const look = source('components/notifications/SecondLookScreen.tsx');
  check('the second look photo saves at 80%, Compatible', count(look, quality) === 1 && count(look, /quality:/g) === 1 && count(look, compatible) === 1);
  check(
    "the second look's camera takes one photo and its library as many as there is room for, with those settings",
    /source === 'camera'\s*\? await ImagePicker\.launchCameraAsync\(options\)\s*: await ImagePicker\.launchImageLibraryAsync\(\{\s*\.\.\.options,\s*allowsMultipleSelection: true,\s*selectionLimit: SECOND_LOOK_MAX_PHOTOS - photos\.length,\s*\}\)/.test(look)
  );
  check('a second look photo keeps its own type', /next\.push\(\{ uri: asset\.uri, mimeType: asset\.mimeType \?\? 'image\/jpeg' \}\)/.test(look));
  check('a second look photo uploads with its own type', /uploadEntryPhoto\(photo\.uri, photo\.mimeType\)/.test(look));
  check(
    "a second look photo's size is read from its file as it is picked, as a post photo's is, never the picker's number",
    /size = await fileSize\(asset\.uri\);/.test(look) && !/asset\.fileSize/.test(look)
  );
}

// ---- the Spin page's fixed words and numbers ------------------------------------------------------
{
  const today = source('components/today/TodayScreen.tsx');
  // The heading rule is a pure switch inside the screen: its own text is run here, types taken out.
  const text = functionText(today, 'function headingFor(');
  const strip = nodeModule.stripTypeScriptTypes;
  let headingFor = null;
  try {
    headingFor = text === null || typeof strip !== 'function' ? null : new Function(`${strip(text)}\nreturn headingFor;`)();
  } catch (err) {
    check('the heading rule can be read', false, String(err));
  }
  check('the heading rule is there and this Node can read it', typeof headingFor === 'function', `text ${text !== null}, strip ${typeof strip}`);
  if (typeof headingFor === 'function') {
    eq('a first spin before 8 PM (SPIN-02)', headingFor('open', true), 'Your first spin is ready.');
    eq('the spin is open', headingFor('open', false), 'Your spin is open.');
    eq('posted', headingFor('posted', false), 'Posted.');
    eq('posted, first spin or not', headingFor('posted', true), 'Posted.');
    eq('skipped', headingFor('skipped', true), 'Skipped.');
    eq('nothing while loading', headingFor(null, false), '');
  }
  check('the heading follows the server first_spin only', /headingFor\(viewState, view\?\.first_spin === true\)/.test(today));
  check('a bonus spin asks first (SPIN-08)', /<ConfirmSheet[\s\S]{0,300}title="Use your bonus spin\?"/.test(today));

  const wheel = source('components/wheel/Wheel.tsx');
  check('the wheel turns for 4 seconds (SPIN-03)', /const TURN_MS = 4000;/.test(wheel) && /withTiming\(target, \{ duration: TURN_MS,/.test(wheel));
  check(
    'with Reduce Motion the wheel lands at once, with no turn',
    /void readReduceMotion\(\)\.then\(\(reduce\) => \{\s*if \(cancelled\) return;\s*if \(reduce\) \{\s*rotation\.value = target;\s*onLandedRef\.current\?\.\(\);\s*return;\s*\}\s*setTurning\(true\);\s*rotation\.value = withTiming\(/.test(wheel)
  );

  const editor = source('components/wheel/WheelEditor.tsx');
  check('leaving with changes asks (SPIN-04)', /title="Leave without saving\?"/.test(editor));
  check('every category weighs the same (SPIN-03)', /const EQUAL_WEIGHT = 1;/.test(editor));
  check(
    'Save sends weight 1 for every row',
    /rows\.map\(\(row\) => \(\{ id: row\.id, name: row\.name\.trim\(\), weight: EQUAL_WEIGHT \}\)\)/.test(editor) &&
      (editor.match(/\bweight:/g) ?? []).length === (editor.match(/\bweight: EQUAL_WEIGHT\b/g) ?? []).length
  );

  check('a deleted entry leaves its line (POST-05)', source('components/today/SettledBlock.tsx').includes('This entry was deleted. The day still counts.'));

  for (const rel of ['components/today/CountdownBlock.tsx', 'lib/wheel/useToday.ts', 'lib/wheel/todayStore.ts', 'lib/wheel/countdown.ts']) {
    check(`${rel} never reads the phone's clock (SPIN-09)`, !/Date\.now\(/.test(source(rel)));
  }
  check('the countdown counts on a steady clock', /msUntil\(nextOpenAt, anchor, performance\.now\(\)\)/.test(source('components/today/CountdownBlock.tsx')));
}

// ---- how a report ends, with the client's own errors (moderation, 2026-11-17) ---------------------
{
  // The post page, a comment's menu and the person page all end a report through this line.
  const reports = await load('lib/api/reports.ts');
  const line = reports.reportFailureLine;
  eq('a report answered 404 counts as sent', line(new ApiError(404, 'not_found')), null);
  eq('so does a 404 whose body did not read', line(new ApiError(404, 'server_error')), null);
  eq('too many tries says to wait', line(new ApiError(429, 'rate_limited')), 'Too many tries. Wait a minute and try again.');
  eq('no connection says so', line(new ApiError(0, 'network_unavailable')), "Couldn't connect. Check your connection and try again.");
  eq('a server failure is the plain line', line(new ApiError(500, 'server_error')), 'Something went wrong. Please try again.');
  eq('a refused reason is the plain line', line(new ApiError(400, 'validation_failed')), 'Something went wrong. Please try again.');
  eq('anything but an answer is the plain line', line(new Error('lost')), 'Something went wrong. Please try again.');
  check('the old thanks line is gone', !('REPORT_THANKS' in reports));
}

// ---- the server's moderation answers, through the app's own readers (moderation, 2026-11-17) -----
{
  // The app's side was built from the spec, not from the server. This file is what the real server
  // sends for each moderation key and kind, with ids, instants and photo links swapped for fixed
  // placeholders; its Go test (TestModerationContractIntegration_TheAnswersTheAppReads) writes and
  // checks it. Each answer is served here to the app's own call, through the real client, so a key
  // or kind the readers would drop fails here.
  let contract = null;
  try {
    contract = JSON.parse(source('internal/httpapi/testdata/moderation_contract.json'));
  } catch (err) {
    check("the server's moderation answers can be read", false, String(err));
  }
  if (contract !== null) {
    const ids = contract.ids;
    const realFetch = globalThis.fetch;
    const asked: string[] = [];
    // Answers the requests `run` makes with `body`, and keeps the paths they asked for.
    const serve = async (body: unknown, run: () => Promise<any>) => {
      globalThis.fetch = async (url) => {
        asked.push(String(url));
        return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });
      };
      try {
        return await run();
      } finally {
        globalThis.fetch = realFetch;
      }
    };
    const askedFor = (path: string) => asked.at(-1)?.endsWith(`/v1${path}`) === true;
    const notifications = await load('lib/api/notifications.ts');
    const boardApi = await load('lib/api/board.ts');
    const spinApi = await load('lib/api/spin.ts');
    const { loadToday } = await load('lib/wheel/loadToday.ts');
    const { DECISION_KINDS, REASON_WORDS, canAskSecondLook, decisionLine, isDecisionKind, isSecondLookAsked } = await load('lib/notifications/moderation.ts');

    // The bell: one item of each decision kind, none dropped, every key read as the server sent it.
    const bell = await serve(contract.bell, () => notifications.getNotifications());
    check('the bell reads GET /notifications', askedFor('/notifications'), asked.at(-1));
    eq('the bell keeps every decision item the server sends', bell.items.length, contract.bell.items.length);
    eq(
      'the server sends one item of each decision kind the app knows',
      JSON.stringify(contract.bell.items.map((item) => item.kind).sort()),
      JSON.stringify([...DECISION_KINDS].sort())
    );
    eq('the decisions light the dot', bell.unread, true);
    const firstPhoto = contract.post_page_owner.photos[0].url;
    for (const sent of contract.bell.items) {
      const read = bell.items.find((item) => item.id === sent.id);
      check(`${sent.kind}: read`, read !== undefined);
      if (read === undefined) continue;
      for (const key of Object.keys(sent)) {
        eq(`${sent.kind}: ${key} reads as sent`, JSON.stringify(read[key]), JSON.stringify(sent[key]));
      }
      const removed = sent.kind === 'post_removed' || sent.kind === 'comment_removed';
      const kept = sent.kind === 'post_kept' || sent.kind === 'comment_kept';
      const decision = sent.id.slice(sent.kind.length + 1);
      check(`${sent.kind}: a decision with its sentence`, isDecisionKind(read.kind) && decisionLine(read.kind, read.reason) !== '');
      // A removal names the reason the admin gave, sent as its key (2026-11-18); nothing else has one.
      if (removed) {
        const words = Object.hasOwn(REASON_WORDS, read.reason) ? REASON_WORDS[read.reason] : '';
        check(`${sent.kind}: the server sends a reason the app has words for`, words !== '', read.reason);
        eq(
          `${sent.kind}: the sentence names the reason's words`,
          decisionLine(read.kind, read.reason),
          `We removed your ${sent.kind === 'post_removed' ? 'post' : 'comment'} for ${words}.`
        );
      } else {
        eq(`${sent.kind}: no reason`, read.reason, '');
      }
      eq(`${sent.kind}: no person`, read.actor, null);
      eq(`${sent.kind}: the words are the title or the comment`, read.text, sent.kind.startsWith('post_') ? 'Dune' : 'Thanks for watching');
      check(`${sent.kind}: no address on any item`, !('contact' in sent) && !('contact' in read), JSON.stringify(read.contact));
      eq(`${sent.kind}: the ref, on a removal only, is the start of its own decision`, read.ref, removed ? decision.slice(0, 8) : '');
      // The id a second look names, on a removal only, sent as its own key (never read out of the item id).
      eq(`${sent.kind}: the removal id, on a removal only, is its own decision`, read.removal, removed ? decision : '');
      eq(`${sent.kind}: the second look, on a removal only`, read.second_look, removed ? sent.second_look : '');
      eq(`${sent.kind}: a removal or a kept one opens no post, the others open the post`, read.entry_id, removed || kept ? null : ids.post);
      eq(
        `${sent.kind}: only a post that is back or hidden shows its first photo`,
        read.photo_url,
        sent.kind === 'post_back' || sent.kind === 'post_hidden' ? firstPhoto : ''
      );
      // A tap opens the second look only while none was asked. Both removals here were brought
      // back after a Keep it removed ("restored"), so neither asks; read as sent with none asked,
      // a removal would.
      check(`${sent.kind}: the tap asks for no second look`, !canAskSecondLook(read));
      eq(`${sent.kind}: with none asked, only a removal would`, canAskSecondLook({ ...read, second_look: '' }), removed);
      // "closed": none was asked and none can be any more. No tap, no hint, no "Second look asked." line.
      const closed = { ...read, second_look: 'closed' };
      check(`${sent.kind}: closed, nothing opens and no line shows`, !canAskSecondLook(closed) && !isSecondLookAsked(closed));
    }
    eq(
      'both removals were answered by a Bring it back',
      contract.bell.items.filter((item) => item.kind.endsWith('_removed')).map((item) => item.second_look).join(','),
      'restored,restored'
    );

    // The bell's removals before any answer, as the server sent them: the comment's second look
    // asked, the post's not yet (the key left out). The post opens the form; the comment shows
    // "Second look asked." and opens nothing.
    const before = await serve(contract.bell_one_asked, () => notifications.getNotifications());
    eq('the removals before an answer are both read', before.items.length, 2);
    for (const [kind, asks, marked] of [
      ['post_removed', true, false],
      ['comment_removed', false, true],
    ] as const) {
      const read = before.items.find((item) => item.kind === kind);
      check(`${kind} before an answer: read`, read !== undefined);
      if (read === undefined) continue;
      eq(`${kind} before an answer: the second look reads as sent`, read.second_look, asks ? '' : 'asked');
      eq(`${kind} before an answer: the removal id is its own key`, read.removal, read.id.slice(kind.length + 1));
      eq(`${kind} before an answer: the tap opens the form`, canAskSecondLook(read), asks);
      eq(`${kind} before an answer: "Second look asked." shows`, isSecondLookAsked(read), marked);
    }

    // The owner's own Board card under review.
    const board = await serve(contract.board_owner, () => boardApi.getBoard());
    check('the Board reads GET /board', askedFor('/board'), asked.at(-1));
    const cards = board.days.flatMap((day) => day.cards);
    eq('the Board keeps every card the server sends', cards.length, contract.board_owner.days.reduce((n, day) => n + day.cards.length, 0));
    const own = cards.find((card) => card.entry_id === ids.post);
    check("the owner's card reads as theirs and under review", own?.mine === true && own.under_review === true && own.author.id === ids.owner, JSON.stringify(own));

    // The post page: under review for its owner, and for a writer on their own comment only.
    const pages = [
      ['the owner', contract.post_page_owner, true],
      ['the writer', contract.post_page_writer, false],
    ];
    for (const [who, sent, owner] of pages) {
      const page = await serve(sent, () => boardApi.getPost(ids.post));
      check(`${who}'s post page reads GET /posts/{id}`, askedFor(`/posts/${ids.post}`), asked.at(-1));
      eq(`${who}'s post page: the card under review only for the owner`, page.card.under_review, owner);
      eq(`${who}'s post page keeps every comment`, page.comments?.length, sent.comments.length);
      for (const comment of sent.comments) {
        const read = page.comments?.find((item) => item.id === comment.id);
        eq(`${who}'s post page: under_review on ${comment.text} reads as sent`, read?.under_review, comment.under_review === true);
      }
    }
    const writerPage = contract.post_page_writer.comments;
    eq("the writer's own comment is the one under review", writerPage.find((c) => c.under_review === true)?.id, ids.writer_comment);
    check("someone else's comment beside it is not", writerPage.some((c) => c.id === ids.viewer_comment && c.under_review === undefined));

    // The owner's entry page.
    const entry = await serve(contract.entry_owner, () => entries.getEntry(ids.post));
    check('the entry reads GET /entries/{id}', askedFor(`/entries/${ids.post}`), asked.at(-1));
    eq("the owner's entry is under review", entry.under_review, true);

    // Today, through the load the Spin page runs: under review, then removed.
    const zone = contract.today_entry_removed.time_zone;
    const requests = {
      reportZone: async () => {
        throw new Error('this check sends no report');
      },
      getToday: spinApi.getToday,
      now: () => 0,
    };
    const review = await serve(contract.today_under_review, () => loadToday(zone, false, requests));
    check('Today reads GET /today', askedFor('/today'), asked.at(-1));
    check("under review: Today's entry says so", review.ok && review.today.entry?.under_review === true && review.today.entry_removed !== true, JSON.stringify(review.today?.entry));
    const gone = await serve(contract.today_entry_removed, () => loadToday(zone, false, requests));
    check(
      'removed: Today says so, with no entry, and the day still posted',
      gone.ok && gone.today.entry_removed === true && gone.today.entry === null && gone.today.state === 'posted',
      JSON.stringify({ entry: gone.today?.entry, entry_removed: gone.today?.entry_removed, state: gone.today?.state })
    );
  }
}

// ---- a second look, sent through the real client (2026-11-19) -------------------------------------
{
  const looks = await load('lib/api/secondLooks.ts');
  const { secondLookFailure } = await load('lib/notifications/moderation.ts');
  const realFetch = globalThis.fetch;
  const removal = '00000008-0000-4000-8000-000000000008';
  const words = '  It was a joke between friends.\n';
  // Answers the one request with `status` and `body`, and keeps what was sent.
  const ask = async (status: number, body: unknown) => {
    const sent = [];
    globalThis.fetch = async (url, init) => {
      sent.push({ url: String(url), init });
      return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
    };
    try {
      await looks.askSecondLook(removal, words, ['entries/u1/abc.jpg']);
      return { sent, err: null };
    } catch (err) {
      return { sent, err };
    } finally {
      globalThis.fetch = realFetch;
    }
  };
  for (const status of [201, 200]) {
    const { sent, err } = await ask(status, { second_look: 'asked' });
    eq(`a ${status} is sent`, err, null);
    check(
      `${status}: POST /v1/removals/{id}/second-look with the removal id`,
      sent.length === 1 && sent[0].init?.method === 'POST' && sent[0].url.endsWith(`/v1/removals/${removal}/second-look`),
      sent[0]?.url
    );
    eq(`${status}: the words as written, and the photo keys`, sent[0]?.init?.body, JSON.stringify({ text: words, photo_keys: ['entries/u1/abc.jpg'] }));
  }
  for (const [code, line] of [
    ['second_look_asked', 'You already asked for a second look.'],
    ['second_look_too_late', "It's too late to ask. It's been 30 days."],
    ['second_look_back', 'We already took another look.'],
  ]) {
    const { err } = await ask(409, { error: code });
    check(`a 409 ${code} comes back with its code`, err instanceof ApiError && err.status === 409 && err.code === code, String(err?.code));
    eq(`a 409 ${code} reads over Send`, JSON.stringify(secondLookFailure('post_removed', err?.status, err?.code, err?.field, err?.reason)), JSON.stringify({ at: 'form', line }));
  }
  const long = await ask(400, { error: 'validation_failed', field: 'text', reason: 'too_long' });
  eq(
    'a 400 on the words reads under the box',
    JSON.stringify(secondLookFailure('comment_removed', long.err?.status, long.err?.code, long.err?.field, long.err?.reason)),
    JSON.stringify({ at: 'text', line: 'At most 500 characters.' })
  );
  const keys = await ask(400, { error: 'validation_failed', field: 'photo_keys', reason: 'invalid' });
  eq('a 400 on the photos reads under them', secondLookFailure('post_removed', keys.err?.status, keys.err?.code, keys.err?.field, keys.err?.reason).at, 'photos');
  const gone = await ask(404, { error: 'not_found' });
  eq('a 404 reads over Send', secondLookFailure('comment_removed', gone.err?.status, gone.err?.code).line, "This comment isn't here any more.");
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
