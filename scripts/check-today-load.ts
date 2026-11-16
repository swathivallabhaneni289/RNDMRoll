// @ts-nocheck
/**
 * Throwaway checks for how the Spin page loads Today (2026-11-16): the time zone report and
 * GET /today in order, the whole wheel drawn while it loads, the sign-up path that starts the load
 * early, and the line a failed refresh shows. Not part of the app bundle.
 * Run from the repo root with:  node --experimental-strip-types scripts/check-today-load.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { isZoneRefusal, loadToday, mustReportZone } from '../lib/wheel/loadToday.ts';
import { drawnAngles, labelLine, LOADING_WHEEL, poseOf, toneIndexes, wedgeCentres } from '../lib/wheel/geometry.ts';

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
  const a = JSON.stringify(actual);
  const e = JSON.stringify(expected);
  check(name, a === e, `got ${a}, wanted ${e}`);
}

const source = (rel: string) => readFileSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), 'utf8');

// ---- when the zone is reported ------------------------------------------------------------------

const HERE = 'Europe/London';
const AWAY = 'Asia/Tokyo';
const took = (zone: string, session: number) => ({ zone, session });

check("a session's first load reports", mustReportZone(HERE, null, 1, false));
check("a session's first load reports, even on a tab focus", mustReportZone(HERE, null, 1, true));
check('a new sign in reports the same zone again', mustReportZone(HERE, took(HERE, 1), 2, false));
check('a new sign in reports on a tab focus too', mustReportZone(HERE, took(HERE, 1), 2, true));
check('a later load skips an unchanged zone', !mustReportZone(HERE, took(HERE, 1), 1, false));
check('a later tab focus skips an unchanged zone', !mustReportZone(HERE, took(HERE, 1), 1, true));
check('a later load reports a changed zone', mustReportZone(AWAY, took(HERE, 1), 1, false));
check('a tab focus alone never reports (the answer in another zone does)', !mustReportZone(AWAY, took(HERE, 1), 1, true));

// ---- which refusal means "no zone on the server" ------------------------------------------------

const NETWORK = { status: 0, code: 'network_unavailable' };
const BUSY = { status: 503, code: 'server_error' };
const TOO_MANY = { status: 429, code: 'rate_limited' };
const ZONE_REQUIRED = { status: 400, code: 'validation_failed', field: 'time_zone', reason: 'required' };
const ZONE_INVALID = { status: 400, code: 'validation_failed', field: 'time_zone', reason: 'invalid' };
const OTHER_FIELD = { status: 400, code: 'validation_failed', field: 'for_date', reason: 'invalid' };

check('400 on time_zone is the no-zone refusal', isZoneRefusal(ZONE_REQUIRED) && isZoneRefusal(ZONE_INVALID));
check('400 on another field is not', !isZoneRefusal(OTHER_FIELD));
check('no connection is not', !isZoneRefusal(NETWORK));
check('a server failure is not', !isZoneRefusal(BUSY));
check('nothing at all is not', !isZoneRefusal(null) && !isZoneRefusal(undefined) && !isZoneRefusal('time_zone'));

// ---- one load, against a fake server -------------------------------------------------------------

/**
 * A fake server. `report` and `get` list what each request answers, in order. A report answers
 * 'today' (the zone stored, Today in it) or 'zone-only' (an older server); a GET answers the zone
 * its Today is counted in. An object is thrown as the failure. Every Today carries `sentAt`, the
 * clock reading its request went out at, so the load's `mark` can be compared with it.
 */
function fakeServer(answers: { report?: unknown[]; get?: unknown[] }) {
  const report = [...(answers.report ?? [])];
  const get = [...(answers.get ?? [])];
  const sent: string[] = [];
  let clock = 0;
  return {
    sent,
    requests: {
      now: () => {
        clock += 1;
        return clock;
      },
      reportZone: async (zone: string) => {
        sent.push('POST');
        const next = report.shift();
        if (next === undefined) throw new Error('an unexpected POST /time-zone');
        if (typeof next === 'object') throw next;
        if (next === 'zone-only') return { time_zone: zone };
        return { time_zone: zone, today: { time_zone: zone, sentAt: clock } };
      },
      getToday: async () => {
        sent.push('GET');
        const next = get.shift();
        if (next === undefined) throw new Error('an unexpected GET /today');
        if (typeof next === 'object') throw next;
        return { time_zone: next, sentAt: clock };
      },
    },
  };
}

/** Runs one load in HERE and checks what went out, what came back and the mark it carries. */
async function scenario(
  name: string,
  report: boolean,
  answers: { report?: unknown[]; get?: unknown[] },
  want: { sent: string[]; ok: boolean; zone?: string; reported: boolean; error?: unknown }
): Promise<void> {
  const server = fakeServer(answers);
  const load = await loadToday(HERE, report, server.requests);
  eq(`${name}: requests`, server.sent, want.sent);
  eq(`${name}: answered`, load.ok, want.ok);
  eq(`${name}: zone taken`, load.reported, want.reported);
  if (want.ok && load.ok) {
    eq(`${name}: Today shown`, load.today.time_zone, want.zone);
    eq(`${name}: counted from its own request`, load.mark, load.today.sentAt);
  }
  if (!want.ok && !load.ok) check(`${name}: the failure kept`, load.error === want.error);
}

async function loads(): Promise<void> {
  // A load that reports: the report's own Today is the load.
  await scenario('report answered with Today', true, { report: ['today'] }, {
    sent: ['POST'], ok: true, zone: HERE, reported: true,
  });
  await scenario('report from an older server', true, { report: ['zone-only'], get: [HERE] }, {
    sent: ['POST', 'GET'], ok: true, zone: HERE, reported: true,
  });
  await scenario('report with no connection still asks for Today', true, { report: [NETWORK], get: [HERE] }, {
    sent: ['POST', 'GET'], ok: true, zone: HERE, reported: false,
  });
  await scenario('report refused for too many tries still asks for Today', true, { report: [TOO_MANY], get: [HERE] }, {
    sent: ['POST', 'GET'], ok: true, zone: HERE, reported: false,
  });
  await scenario('a refused zone name is not sent twice', true, { report: [ZONE_INVALID], get: [ZONE_REQUIRED] }, {
    sent: ['POST', 'GET'], ok: false, reported: false, error: ZONE_REQUIRED,
  });
  await scenario('report and Today both fail', true, { report: [NETWORK], get: [NETWORK] }, {
    sent: ['POST', 'GET'], ok: false, reported: false, error: NETWORK,
  });
  await scenario('one report per load, even in another zone', true, { report: ['zone-only'], get: [AWAY] }, {
    sent: ['POST', 'GET'], ok: true, zone: AWAY, reported: true,
  });

  // A load that does not report.
  await scenario('an unchanged zone is one GET', false, { get: [HERE] }, {
    sent: ['GET'], ok: true, zone: HERE, reported: false,
  });
  await scenario('no connection is one GET', false, { get: [NETWORK] }, {
    sent: ['GET'], ok: false, reported: false, error: NETWORK,
  });
  await scenario('another 400 sends no report', false, { get: [OTHER_FIELD] }, {
    sent: ['GET'], ok: false, reported: false, error: OTHER_FIELD,
  });

  // The 400 time_zone fallback: no zone stored, or the stored one no longer loads.
  await scenario('no zone on the server: report, and its Today', false, { get: [ZONE_REQUIRED], report: ['today'] }, {
    sent: ['GET', 'POST'], ok: true, zone: HERE, reported: true,
  });
  await scenario('no zone on an older server: report, then Today', false, {
    get: [ZONE_REQUIRED, HERE], report: ['zone-only'],
  }, { sent: ['GET', 'POST', 'GET'], ok: true, zone: HERE, reported: true });
  await scenario('no zone and the report fails', false, { get: [ZONE_REQUIRED], report: [NETWORK] }, {
    sent: ['GET', 'POST'], ok: false, reported: false, error: NETWORK,
  });
  await scenario('no zone twice: still one report', false, {
    get: [ZONE_REQUIRED, ZONE_REQUIRED], report: ['zone-only'],
  }, { sent: ['GET', 'POST', 'GET'], ok: false, reported: true, error: ZONE_REQUIRED });

  // Another phone moved the stored zone: this phone's goes back.
  await scenario('Today in another zone: report, and its Today', false, { get: [AWAY], report: ['today'] }, {
    sent: ['GET', 'POST'], ok: true, zone: HERE, reported: true,
  });
  await scenario('Today in another zone, older server', false, { get: [AWAY, HERE], report: ['zone-only'] }, {
    sent: ['GET', 'POST', 'GET'], ok: true, zone: HERE, reported: true,
  });
  await scenario('Today in another zone and the report fails: the first Today stays', false, {
    get: [AWAY], report: [NETWORK],
  }, { sent: ['GET', 'POST'], ok: true, zone: AWAY, reported: false });
  await scenario('Today in another zone and the second GET fails: the first Today stays', false, {
    get: [AWAY, NETWORK], report: ['zone-only'],
  }, { sent: ['GET', 'POST', 'GET'], ok: true, zone: AWAY, reported: true });
}

// ---- the wiring --------------------------------------------------------------------------------

const useToday = source('lib/wheel/useToday.ts');
check(
  'every load goes through loadToday with the report rule',
  /loadToday\(zone, mustReportZone\(zone, before\.lastReportedZone, session, trigger === 'focus'\)/.test(useToday)
);
check('mount and foreground no longer force a report', !/trigger === 'mount' \|\| trigger === 'foreground'/.test(useToday));
check('the session is the count of sign ins', /const session = getSignInEpoch\(\);/.test(useToday));
check(
  'the zone and Today go into the store in one write',
  /applyToday\(userId, load\.today, load\.mark, \{ \.\.\.keep, \.\.\.reported \}\)/.test(useToday)
);
check(
  'a failed refresh with Today on screen is kept for the page to say',
  /if \(current\.today\) \{[^}]*?patchIfOwner\(userId, \{ \.\.\.reported, refetchFailure: failureKindOf\(load\.error\) \}\)/.test(useToday)
);
check(
  'a failed load with nothing on screen leaves the page to a load already waiting',
  /\} else if \(!\(queuedLoad !== null && queuedLoadUser === userId\)\) \{[^}]*?patchIfOwner\(userId, \{ \.\.\.reported, loadFailure: failureKindOf\(load\.error\) \}\);/.test(
    useToday
  )
);
check(
  'an earlier failure does not greet the Spin page: its own first load decides',
  /useLayoutEffect\(\(\) => \{[\s\S]*?if \(snap\.today === null && snap\.loadFailure !== null\) patchIfOwner\(userId, \{ loadFailure: null \}\);/.test(
    useToday
  )
);

const store = source('lib/wheel/todayStore.ts');
check('an answer clears a failed refresh', /loadFailure: null,\s*refetchFailure: null,\s*\.\.\.extra,/.test(store));
check('a new snapshot has no failed refresh', /loadFailure: null,\s*refetchFailure: null,\s*requesting: null,/.test(store));

const spinApi = source('lib/api/spin.ts');
check(
  "the report hands back the server's answer",
  /reportTimeZone\(timeZone: string\): Promise<TimeZoneResponse> \{\s*return api\.post<TimeZoneResponse>\('\/time-zone'/.test(spinApi)
);
check('the answer may carry Today', /interface TimeZoneResponse \{\s*time_zone: string;\s*today\?: Today;\s*\}/.test(source('lib/api/types.ts')));

const layout = source('app/(app)/_layout.tsx');
const claimAt = layout.indexOf('useReminderSetup();');
const prefetch = /useEffect\(\(\) => \{\s*if \(extrasPending && userId !== null\) void refetchToday\(userId, 'mount'\);\s*\}, \[extrasPending, userId\]\);/.exec(
  layout
);
check('a new account starts loading Today on the photo-and-bio page', prefetch !== null);
check('the load starts after the store is claimed for that account', prefetch !== null && claimAt !== -1 && prefetch.index > claimAt);

// ---- the wheel while loading -------------------------------------------------------------------

const goWheel = source('internal/spin/wheel.go');
const defaultWheel = /func DefaultWheel\(\) \[\]WheelInput \{([\s\S]*?)\n\}/.exec(goWheel);
const startingCount = defaultWheel ? (defaultWheel[1].match(/Name: "/g) ?? []).length : -1;
check('the starting wheel was found in internal/spin/wheel.go', startingCount > 0, 'no DefaultWheel');
eq('the loading wheel has as many wedges as the starting wheel', LOADING_WHEEL.length, startingCount);
eq('six wedges', LOADING_WHEEL.length, 6);
check('no words on the loading wheel', LOADING_WHEEL.every((entry) => entry.name === ''));
check('every wedge the same', LOADING_WHEEL.every((entry) => entry.weight === 1));
eq(
  'drawn as six 60 degree wedges',
  drawnAngles(LOADING_WHEEL.map((entry) => entry.weight)).map((a) => Math.round(a)),
  [60, 60, 60, 60, 60, 60]
);
eq('dark and light in turn, like the real wheel', toneIndexes(LOADING_WHEEL.length), [
  'dark', 'light', 'dark', 'light', 'dark', 'light',
]);
check('an empty name prints nothing', labelLine('', 'across', 60, 116) === '' && labelLine('', 'along', 60, 116) === '');
check('one list for every render (the memoised wheel keeps it)', Object.isFrozen(LOADING_WHEEL));
eq(
  "a new account's wheel rests where the loading wheel did",
  poseOf(wedgeCentres(drawnAngles(Array(startingCount).fill(1)))[0]),
  poseOf(wedgeCentres(drawnAngles(LOADING_WHEEL.map((entry) => entry.weight)))[0])
);

const screen = source('components/today/TodayScreen.tsx');
check('Today draws the loading wheel until its first answer', /\(view\?\.wheel \?\? LOADING_WHEEL\)/.test(screen));
check('never the bare rim', !/view\?\.wheel \?\? \[\]/.test(screen));
check('the loading wheel is hidden from VoiceOver and its centre takes no tap', /decorative=\{view === null\}/.test(screen));
check("it still says it's loading", /if \(view === null\) return labelLine\('Loading your wheel\.'\);/.test(screen));
check('no Edit wheel until the first answer', /loaded: today !== null/.test(screen));

const wheel = source('components/wheel/Wheel.tsx');
check('a decorative wheel is hidden from VoiceOver', /const hiddenFromReader = decorative \|\|/.test(wheel));
check('a decorative wheel has a plain centre', /\{decorative \|\| count === 0 \? \(\s*<View pointerEvents="none" style=\{hubStyle\}>/.test(wheel));

// ---- a failed refresh says so ------------------------------------------------------------------

check(
  'Waiting: under the countdown, after a spin message',
  /onRetry=\{\(\) => tryAgain\(bonusAfterSeqRef\.current\)\}\s*\/>\s*<\/View>\s*\) : \(\s*refreshLine\s*\)\}/.test(screen)
);
check("Open: in place of 'Open until midnight.'", /\(refreshMessage \?\? labelLine\('Open until midnight\.'\)\)/.test(screen));
check(
  'Result and Late: under the stamp row, after a bonus message',
  /\) : refreshShown \? \(\s*<View style=\{\{ marginTop: space\.lg \}\}>\{refreshMessage\}<\/View>\s*\) : null\}/.test(screen) &&
    /followsLink=\{bonusLinkShown \|\| bonusFailed \|\| refreshShown\}/.test(screen)
);
check(
  'Posted and Skipped: under the countdown',
  (screen.match(/\/>\s*\{refreshLine\}\s*\{editLink\}/g) ?? []).length === 2
);
check(
  "the page's own lines and Try again, busy while it asks",
  /<StatusMessage kind=\{refetchFailure\} busy=\{retrying\} onRetry=\{retryRefresh\} \/>/.test(screen) &&
    /void refetch\('manual'\)\.finally\(\(\) => setRetrying\(false\)\)/.test(screen)
);
check(
  'the lines are the ones already on the page',
  /network: "Couldn't connect\. Check your connection and try again\.",/.test(source('components/today/StatusMessage.tsx')) &&
    /generic: 'Something went wrong\. Please try again\.',/.test(source('components/today/StatusMessage.tsx'))
);

// ---- copy rules in the files this touches ------------------------------------------------------

// Built from char codes so this file holds no dash of its own.
const DASHES = new RegExp('[' + String.fromCharCode(0x2013, 0x2014) + ']');
for (const rel of [
  'lib/wheel/loadToday.ts',
  'lib/wheel/useToday.ts',
  'lib/wheel/todayStore.ts',
  'lib/wheel/geometry.ts',
  'lib/api/spin.ts',
  'components/today/TodayScreen.tsx',
  'app/(app)/_layout.tsx',
  'scripts/check-today-load.ts',
]) {
  check(`${rel} has no em or en dash`, !DASHES.test(source(rel)));
}

loads()
  .catch((err) => check('the loads ran', false, String(err)))
  .then(() => {
    console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
    if (failures > 0) process.exitCode = 1;
  });
