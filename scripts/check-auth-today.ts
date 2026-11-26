// @ts-nocheck
/**
 * Throwaway checks for Log in's "@name", sign-up's code page and its way back to an account a lost
 * answer made, where Today shows Edit wheel, and the end of underlined text links. Not part of the
 * app bundle.
 * Run from the repo root with:  node --experimental-strip-types scripts/check-auth-today.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { loginName } from '../lib/auth/loginName.ts';
import { clearPendingSignup, lostSignup, pendingSignup, setLostSignup, setPendingSignup } from '../lib/auth/pendingSignup.ts';
import {
  answerLine,
  codeLine,
  EMAIL_TAKEN,
  EXPIRED,
  GENERIC,
  NO_MORE_CODES,
  REPLACED,
  sentLine,
  SHORT_CODE,
  TOO_MANY,
  USERNAME_TAKEN,
  WRONG_CODE,
} from '../lib/auth/signupCode.ts';
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

// ---- Sign-up: the account a lost verify answer made -----------------------------------------

const lost = { email: 'Sam@Example.com', password: 'Spin!2026', username: 'sam' };
const taken = { status: 409, code: 'email_taken' };
// The code page's verify once the sign-up is gone: its 30 minutes are up, or a verify finished it.
const expired = { status: 400, code: 'validation_failed', reason: 'expired' };

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
check('an expired sign-up after a lost verify logs in', shouldTryLogin(lost, 'Sam@Example.com', 'Spin!2026', expired));
check('an expired sign-up with another password does not log in', !shouldTryLogin(lost, 'Sam@Example.com', 'Spin!2027', expired));
check('an expired sign-up that was not lost does not log in', !shouldTryLogin(null, 'Sam@Example.com', 'Spin!2026', expired));
for (const reason of ['wrong_code', 'replaced', 'invalid']) {
  check(
    `a code answered ${reason} after a lost verify does not log in`,
    !shouldTryLogin(lost, 'Sam@Example.com', 'Spin!2026', { status: 400, code: 'validation_failed', reason })
  );
}

check('the lost try\'s own account opens the photo-and-bio page', opensExtras(lost, { user: { username: 'sam' } }));
check(
  'an account brought back from deletion signs in like Log in',
  !opensExtras(lost, { restored: true, user: { username: 'sam' } })
);
check('an older account with that email signs in like Log in', !opensExtras(lost, { user: { username: 'sam_old' } }));
check('an account with no username signs in like Log in', !opensExtras(lost, { user: { username: null } }));

// The sign-up in progress, in memory only (lib/auth/pendingSignup.ts).
eq('no sign-up is in progress when the app starts', pendingSignup(), null);
eq('and none is lost', lostSignup(), null);
setPendingSignup(lost, 'token-1');
eq('Continue keeps the sign-up with its token', JSON.stringify(pendingSignup()), JSON.stringify({ ...lost, token: 'token-1' }));
eq('a started sign-up is not lost', lostSignup(), null);
setLostSignup(pendingSignup());
eq('a verify that may have made the account is remembered', lostSignup()?.email, lost.email);
setPendingSignup({ ...lost, username: 'sam_2' }, 'token-2');
eq('a new Continue takes the place of the sign-up in progress', pendingSignup()?.token, 'token-2');
eq('and keeps the lost one, whose account the server may still finish', lostSignup()?.username, 'sam');
clearPendingSignup();
check('signing in forgets both', pendingSignup() === null && lostSignup() === null);
const pendingModule = source('lib/auth/pendingSignup.ts');
check('the sign-up in progress loads nothing (a type import only), so it never reaches storage', !/^import (?!type )/m.test(pendingModule));

const form = source('components/profile/ProfileForm.tsx');
check(
  'Continue only starts the sign-up, keeps it in memory and opens the code page',
  /started = await api\.post<SignupStart>\('\/auth\/signup', body, \{ auth: false \}\);/.test(form) &&
    /setPendingSignup\(thisTry, started\.signup_token\);\s*router\.push\('\/signup-code'\);/.test(form)
);
check('a start whose answer was lost needs nothing, as it made no account', !/signupAnswerLost|setLostSignup/.test(form));
check(
  'email_taken after a lost verify logs in, then opens the page opensExtras picks',
  /const lost = lostSignup\(\);\s*if \(lost === null \|\| !shouldTryLogin\(lost, thisTry\.email, password, err\)\) throw err;/.test(form) &&
    /await signIn\(result, \{ extras: opensExtras\(lost, result\) \}\);\s*clearPendingSignup\(\);/.test(form)
);

const codePage = source('app/(auth)/signup-code.tsx');
check(
  'a verify that made the account is remembered until signed in',
  /\{ signup_token: signup\.token, code: digits \},\s*\{ auth: false \}\s*\);\s*(\/\/[^\n]*\n\s*)*setLostSignup\(signup\);/.test(codePage) &&
    /await signIn\(result, \{ extras \}\);\s*clearPendingSignup\(\);/.test(codePage)
);
check(
  'a lost or unreadable verify answer is remembered',
  /if \(!\(err instanceof ApiError\) \|\| signupAnswerLost\(err\)\) \{\s*setLostSignup\(signup\);\s*throw err;/.test(codePage)
);
check(
  '"expired" or email_taken after a lost verify logs in, then opens the page opensExtras picks',
  /const lost = lostSignup\(\);\s*if \(lost === null \|\| !shouldTryLogin\(lost, signup\.email, signup\.password, err\)\) throw err;/.test(codePage) &&
    /extras = opensExtras\(lost, result\);/.test(codePage)
);
check('the code signs in as Continue once did, with the photo-and-bio page next', /let extras = true;/.test(codePage));
check(
  'a refused log in gives back the answer that led to it, one that could pass shows itself',
  [form, codePage].every((page) => /throw isRetryable\(err\) \? err : taken;/.test(page))
);
check(
  'a line about the code goes under the Code box, any other under the page',
  /if \(err instanceof ApiError && err\.field === 'code'\) setCodeError\(codeLine\(err\.reason\)\);\s*else setFormError\(failure\(err\)\);/.test(codePage) &&
    /return err\.code === 'network_unavailable' \? err\.userMessage : answerLine\(err\);/.test(codePage)
);
check(
  'a second tap, on either button, sends nothing',
  (codePage.match(/if \(sendingRef\.current\) return;/g) ?? []).length === 2 &&
    (codePage.match(/sendingRef\.current = true;/g) ?? []).length === 2 &&
    (codePage.match(/finally \{\s*sendingRef\.current = false;/g) ?? []).length === 2
);
check(
  'the sign-up is read once, on mount, and without one the page hands over to page 1',
  /const \[signup\] = useState\(pendingSignup\);\s*if \(signup === null\) return <Redirect href="\/make-it-yours" \/>;/.test(codePage) &&
    !/useLocalSearchParams|useGlobalSearchParams/.test(codePage)
);
check('Back goes to page 1', /if \(router\.canGoBack\(\)\) router\.back\(\);\s*else router\.replace\('\/make-it-yours'\);/.test(codePage));
check(
  'the code page is its own screen, with the swipe back while signed out',
  /<Stack\.Screen name="signup-code" options=\{\{ gestureEnabled: !signedIn \}\} \/>/.test(source('app/(auth)/_layout.tsx'))
);
{
  const start = codePage.indexOf('label="Code"');
  const box = start < 0 ? '' : codePage.slice(start, codePage.indexOf('/>', start));
  check(
    'the Code box: digits only, the number pad, the code from Messages or Mail, 6 at most, keyboard up',
    /setCode\(next\.replace\(\/\\D\/g, ''\)\.slice\(0, 6\)\)/.test(box) &&
      ['keyboardType="number-pad"', 'autoComplete="one-time-code"', 'textContentType="oneTimeCode"', 'maxLength={6}', 'autoFocus'].every((prop) =>
        box.includes(prop)
      )
  );
}
check(
  'the words: heading, line, Verify code, and Send a new code as a button like the Forgot password one',
  /<AppText role="heading" accessibilityRole="header">\s*Check your email\s*<\/AppText>/.test(codePage) &&
    /<AppText role="body" tone="muted">\s*\{sentLine\(signup\.email, resent\)\}\s*<\/AppText>/.test(codePage) &&
    /<PrimaryButton label="Verify code" onPress=\{verifyCode\} loading=\{busy\} disabled=\{busy\} \/>/.test(codePage) &&
    [codePage, source('app/(auth)/forgot-password.tsx')].every((page) => page.includes('<AppText role="button">Send a new code</AppText>'))
);

// ---- Sign-up: the code page's lines -----------------------------------------------------------

eq('the line under the heading names the address', sentLine('sam@example.com', false), 'We sent a code to sam@example.com.');
eq('and says when a new code went', sentLine('sam@example.com', true), 'We sent a new code to sam@example.com.');
eq('a code that is not 6 digits', codeLine('invalid'), 'Enter the 6-digit code from the email.');
eq(
  'a wrong code, or its 5 tries used up',
  codeLine('wrong_code'),
  "That code isn't right. After 5 wrong tries, go back and tap Continue for a new code."
);
eq('a code a newer one replaced', codeLine('replaced'), 'That code was replaced by a newer one. Use the newest code in your email.');
eq('a sign-up that is gone', codeLine('expired'), 'That code has expired. Go back and tap Continue for a new one.');
eq('no reason reads as a wrong code', codeLine(undefined), WRONG_CODE);
eq('a reason the app does not know reads as a wrong code', codeLine('something_new'), WRONG_CODE);
eq(
  'Send a new code once the sign-up is over',
  answerLine({ status: 400, code: 'validation_failed', field: 'signup_token', reason: 'expired' }),
  'No more codes for this sign-up. Go back and tap Continue.'
);
eq('an email an account got meanwhile', answerLine(taken), 'That email is already in use.');
eq(
  'a username someone took meanwhile',
  answerLine({ status: 409, code: 'username_taken' }),
  'Someone just took that username. Go back and pick another.'
);
eq('too many requests', answerLine({ status: 429, code: 'rate_limited' }), 'Too many requests. Wait a minute and try again.');
eq('a 429 without the usual body', answerLine({ status: 429, code: 'server_error' }), TOO_MANY);
eq('a server failure', answerLine({ status: 500, code: 'server_error' }), 'Something went wrong. Please try again.');
eq(
  'a token the server did not take',
  answerLine({ status: 400, code: 'validation_failed', field: 'signup_token', reason: 'invalid' }),
  GENERIC
);
for (const line of [
  SHORT_CODE,
  WRONG_CODE,
  REPLACED,
  EXPIRED,
  NO_MORE_CODES,
  EMAIL_TAKEN,
  USERNAME_TAKEN,
  TOO_MANY,
  GENERIC,
  sentLine('sam@example.com', false),
  sentLine('sam@example.com', true),
]) {
  check(`copy rules (no dash, no !, ends with a period): ${line}`, !/[—–!]/.test(line) && /\.$/.test(line) && !line.includes('  '));
}

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
  'the link is drawn under both settled countdowns (a failed refresh says so between them)',
  (
    today.match(
      /<CountdownBlock anchor=\{anchor\} nextOpenAt=\{view\.next_open_at\} todayWindow=\{view\.window\} onZero=\{instantReached\} \/>\s*\{refreshLine\}\s*\{editLink\}/g
    ) ?? []
  ).length === 2
);
check('the fading copy of the wheel draws no link', /\{frozen \? null : editLink\}/.test(today));

// After a big time zone change the next spin can open later than 8:00 PM (PRD known issue 12): every
// line that says when names the moment the countdown counts to (opensAtTime), never a fixed time.
const countdown = source('components/today/CountdownBlock.tsx');
const code = (text: string) => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');
check('the countdown works its time out from the opening and the window', /const opensAt = opensAtTime\(nextOpenAt, todayWindow\);/.test(countdown));
check('the line under the countdown names it', countdown.includes('{`Opens at ${opensAt}.`}'));
check(
  'so do both spoken lines',
  countdown.includes('? `Your next spin opens at ${opensAt}.`') && countdown.includes(', at ${opensAt}.`')
);
check('the countdown names no fixed time', !/\d:\d\d\s?[AP]M/.test(code(countdown)), code(countdown).match(/\d:\d\d\s?[AP]M.*/)?.[0]);
check('Spin works the same time out from its answer', /const opensAt = view !== null \? opensAtTime\(view\.next_open_at, view\.window\) : null;/.test(today));
check("the hub's hint while waiting names it", /hubHint=\{viewState === 'waiting' && opensAt !== null \? `Opens at \$\{opensAt\}\.` : undefined\}/.test(today));
check("so does Skipped's line", /nextSpinLine\(view\.spin\?\.local_date \?\? view\.local_date, view\.local_date, view\.next_open_weekday, opensAt\)/.test(today));
check('Spin names no fixed opening time', !/Opens at \d|opens at \d/.test(code(today)));

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
