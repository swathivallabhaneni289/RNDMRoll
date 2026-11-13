// @ts-nocheck
/**
 * Throwaway checks for Log in's "@name", sign-up's way back to an account its lost answer made,
 * where Today shows Edit wheel, and the end of underlined text links. Not part of the app bundle.
 * Run from the repo root with:  node --experimental-strip-types scripts/check-auth-today.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { loginName } from '../lib/auth/loginName.ts';
import { opensExtras, shouldTryLogin, signupAnswerLost } from '../lib/auth/signupRetry.ts';
import { editWheelLinkShown } from '../components/today/editWheelLink.ts';

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

const source = (rel: string) => readFileSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), 'utf8');

// ---- Log in: a username typed as the app shows it -------------------------------------------

eq('"@name" is looked up as a username', loginName('@name'), 'name');
eq('spaces around "@Name" go, and the capitals stay for the server', loginName('  @Name '), 'Name');
eq('a plain username is only trimmed', loginName(' sam_1 '), 'sam_1');
eq('an email address is only trimmed', loginName(' a@b.co '), 'a@b.co');
eq('"@" alone stays, so it gets the address check', loginName('@'), '@');
eq('"@@x" keeps both, so it gets the address check', loginName('@@x'), '@@x');
eq('"@a@b.com" keeps its @, so it gets the address check', loginName('@a@b.com'), '@a@b.com');
eq('"@na me" is not one word, so it gets the address check', loginName('@na me'), '@na me');
eq('"@nope.com" is not a username, so it gets the address check', loginName('@nope.com'), '@nope.com');
eq('"@sam-1" is not a username, so it is sent as typed', loginName('@sam-1'), '@sam-1');
eq('an empty box stays empty', loginName('   '), '');
check('"@name" skips the address check (no @ left)', !loginName('@name').includes('@'));

const login = source('app/(auth)/login.tsx');
check('Log in checks the box through loginName', /const typed = loginName\(value\);/.test(login));
check('Log in sends the box through loginName', /\{ login: loginName\(login\), password \}/.test(login));
check(
  "the server's address message follows loginName too",
  /setLoginError\(loginName\(login\)\.includes\('@'\)/.test(login) && !/login\.includes\('@'\)/.test(login)
);
check('Log in no longer sends the raw box', !/login: login\.trim\(\)/.test(login));

// ---- Sign-up: the account a lost answer made ------------------------------------------------

const lost = { email: 'Sam@Example.com', password: 'Spin!2026', username: 'sam' };
const taken = { status: 409, code: 'email_taken' };

check('no answer is a lost answer', signupAnswerLost({ status: 0, code: 'network_unavailable' }));
check('a server failure is a lost answer', signupAnswerLost({ status: 500, code: 'server_error' }));
check('a proxy failure is a lost answer', signupAnswerLost({ status: 503, code: 'server_error' }));
check('email_taken is an answer', !signupAnswerLost(taken));
check('too many tries is an answer', !signupAnswerLost({ status: 429, code: 'rate_limited' }));
check('a field error is an answer', !signupAnswerLost({ status: 400, code: 'validation_failed' }));

check('the same email in other capitals logs in', shouldTryLogin(lost, 'sam@example.COM', 'Spin!2026', taken));
check('the same email with spaces around it logs in', shouldTryLogin(lost, ' Sam@Example.com ', 'Spin!2026', taken));
check('another password does not log in', !shouldTryLogin(lost, 'sam@example.com', 'Spin!2027', taken));
check('another email does not log in', !shouldTryLogin(lost, 'sam2@example.com', 'Spin!2026', taken));
check('a sign-up that was not lost does not log in', !shouldTryLogin(null, 'sam@example.com', 'Spin!2026', taken));
check(
  'username_taken does not log in',
  !shouldTryLogin(lost, 'sam@example.com', 'Spin!2026', { status: 409, code: 'username_taken' })
);
check(
  'a second lost answer does not log in',
  !shouldTryLogin(lost, 'sam@example.com', 'Spin!2026', { status: 0, code: 'network_unavailable' })
);

check('the lost try\'s own account opens the photo-and-bio page', opensExtras(lost, { user: { username: 'sam' } }));
check(
  'an account brought back from deletion signs in like Log in',
  !opensExtras(lost, { restored: true, user: { username: 'sam' } })
);
check('an older account with that email signs in like Log in', !opensExtras(lost, { user: { username: 'sam_old' } }));
check('an account with no username signs in like Log in', !opensExtras(lost, { user: { username: null } }));

const form = source('components/profile/ProfileForm.tsx');
check(
  'a lost or unreadable sign-up answer is remembered',
  /if \(!\(err instanceof ApiError\) \|\| signupAnswerLost\(err\)\) \{\s*lostSignupRef\.current = thisTry;\s*throw err;/.test(form)
);
check(
  'email_taken after a lost try logs in, then opens the page opensExtras picks',
  /shouldTryLogin\(lost, thisTry\.email, password, err\)/.test(form) &&
    /extrasPage = opensExtras\(lost, result\);/.test(form) &&
    /await signIn\(result, \{ extras: extrasPage \}\);\s*lostSignupRef\.current = null;/.test(form)
);
check(
  'a refused log in gives back the email_taken answer, one that could pass shows itself',
  /throw isRetryable\(err\) \? err : taken;/.test(form)
);

// ---- Today: where Edit wheel shows ------------------------------------------------------------

function shown(state, composition, extra = {}) {
  return editWheelLinkShown({ state, composition, bonusActive: false, loaded: true, ...extra });
}
check('Edit wheel shows under the wheel while waiting', shown('waiting', 'wheel'));
check('Edit wheel shows under the wheel while open', shown('open', 'wheel'));
check('Edit wheel shows under the countdown once posted', shown('posted', 'result'));
check('Edit wheel shows under the countdown once skipped', shown('skipped', 'result'));
check('no Edit wheel on Result, where a bonus spin can still turn the wheel', !shown('result', 'result'));
check('no Edit wheel on Late', !shown('late', 'result'));
check('no Edit wheel while the turning wheel lands on a result', !shown('result', 'wheel'));
check('no Edit wheel while a bonus spin brings the wheel back', !shown('open', 'wheel', { bonusActive: true }));
check('no Edit wheel before the first answer', !shown(null, 'wheel', { loaded: false }));
check('no Edit wheel while offline with no answer', !shown(null, 'wheel'));

const today = source('components/today/TodayScreen.tsx');
check(
  'the link is drawn under both settled countdowns',
  (today.match(/<CountdownBlock anchor=\{anchor\} nextOpenAt=\{view\.next_open_at\} onZero=\{instantReached\} \/>\s*\{editLink\}/g) ?? [])
    .length === 2
);
check('the fading copy of the wheel draws no link', /\{frozen \? null : editLink\}/.test(today));

// ---- No underlined text used as a button (PRD section 11, copy rules) ---------------------------

const underlined: string[] = [];
const walk = (rel: string) => {
  for (const item of readdirSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), { withFileTypes: true })) {
    const child = `${rel}/${item.name}`;
    if (item.isDirectory()) walk(child);
    else if (/\.tsx?$/.test(item.name) && /textDecorationLine:\s*'[^']*underline/.test(source(child))) underlined.push(child);
  }
};
walk('app');
walk('components');
walk('lib');
check('no underlined text in the app', underlined.length === 0, underlined.join(', '));
check(
  'Log in instead and Sign up instead reach 44 points',
  /const LINK_SLOP_Y = \(minTouchTarget - type\.button\.lineHeight\) \/ 2;/.test(source('components/ui/AppText.tsx'))
);

check('the bell page says what it holds', !/Empty until Phase 5/.test(source('app/(app)/today/notifications.tsx')));

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
