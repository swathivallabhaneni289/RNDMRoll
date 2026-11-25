// @ts-nocheck
/**
 * Throwaway checks for the pure Settings rules and the notice store (plan 02.1-02 Task 1), the
 * username lines of sign-up and the Edit page, the birthday's line on a tap of the grey Continue,
 * and what sign-up, Edit, Password and Delete account do on a tap of their grey button (pinned as
 * written, as there is no React here to run them), and the order of the Notifications switches
 * (Likes first since 2026-11-25). Not part of the app bundle. Run from the repo
 * root with:
 *   node --experimental-strip-types <this file>
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * Files are imported and read by absolute path built from the current directory, so the script can
 * live in scripts/ or anywhere else. (The nocheck line is because Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.cwd();
const rules = await import(pathToFileURL(join(root, 'lib/settings/rules.ts')).href);
const notice = await import(pathToFileURL(join(root, 'lib/settings/notice.ts')).href);
const birthday = await import(pathToFileURL(join(root, 'lib/profile/birthday.ts')).href);
const source = (rel: string) => readFileSync(join(root, rel), 'utf8');

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

/** How many times `text` appears in `body`. */
function count(body: string, text: string): number {
  return body.split(text).length - 1;
}

const {
  usernameProblem,
  passwordProblem,
  newPasswordProblem,
  bioProblem,
  sameText,
  utf8Length,
  USERNAME_PATTERN,
  USERNAME_MAX_LENGTH,
  USERNAME_DOT_MESSAGE,
  USERNAME_SPACE_MESSAGE,
  USERNAME_DASH_MESSAGE,
  USERNAME_AT_MESSAGE,
  USERNAME_CHARACTER_MESSAGE,
  USERNAME_EMPTY_MESSAGE,
  USERNAME_SHORT_MESSAGE,
  USERNAME_LONG_MESSAGE,
  CURRENT_PASSWORD_EMPTY_MESSAGE,
  PASSWORD_SHORT_MESSAGE,
  PASSWORD_WEAK_MESSAGE,
  passwordChecks,
  PASSWORD_LONG_MESSAGE,
  PASSWORD_SAME_MESSAGE,
  BIO_LONG_MESSAGE,
  BIO_MAX,
} = rules;

// The username lines, word for word (2026-11-16). They replace the one rule line.
eq('username dot line', USERNAME_DOT_MESSAGE, 'No dots. Use _ instead.');
eq('username space line', USERNAME_SPACE_MESSAGE, 'No spaces. Use _ instead.');
eq('username dash line', USERNAME_DASH_MESSAGE, 'No dashes. Use _ instead.');
eq('username @ line', USERNAME_AT_MESSAGE, 'Leave out the @.');
eq('username characters line names the set the rule allows', USERNAME_CHARACTER_MESSAGE, 'Only a to z, 0 to 9 and _.');
eq('username empty line', USERNAME_EMPTY_MESSAGE, 'Pick a username.');
eq('username short line', USERNAME_SHORT_MESSAGE, 'At least 3 characters.');
eq('username long line, said at once', USERNAME_LONG_MESSAGE, 'At most 20 characters.');
eq('a username is at most 20', USERNAME_MAX_LENGTH, 20);
check('the old rule line is gone from the rules', rules.USERNAME_FORMAT_MESSAGE === undefined);

// usernameProblem: a character a username can't have is said at once, while typing too; empty and
// short only once done (the box left, or the grey button tapped).
const stored = 'p21_walker';
const typing = (value: string) => usernameProblem(value, stored, 'typing');
const done = (value: string) => usernameProblem(value, stored);
function both(name: string, value: string, expected: string): void {
  eq(`username ${name}, while typing`, typing(value), expected);
  eq(`username ${name}, once done`, done(value), expected);
}
both('a dot between words', 'first.last', USERNAME_DOT_MESSAGE);
both('a dot alone', '.', USERNAME_DOT_MESSAGE);
both('a space', 'has space', USERNAME_SPACE_MESSAGE);
both('a tab counts as a space', 'has\tspace', USERNAME_SPACE_MESSAGE);
both('a no-break space counts as a space', 'has space', USERNAME_SPACE_MESSAGE);
both('a dash', 'has-dash', USERNAME_DASH_MESSAGE);
both('a dash the keyboard made longer', 'has–dash', USERNAME_DASH_MESSAGE);
both('a leading @', '@sam', USERNAME_AT_MESSAGE);
both('an @ alone', '@', USERNAME_AT_MESSAGE);
both('a leading @ wins over a later dot', '@a.b', USERNAME_AT_MESSAGE);
both('an @ inside is another character', 'sam@home', USERNAME_CHARACTER_MESSAGE);
both('an accented letter', 'josé', USERNAME_CHARACTER_MESSAGE);
both('an emoji', 'sam\u{1F600}', USERNAME_CHARACTER_MESSAGE);
both('a symbol', 'sam!', USERNAME_CHARACTER_MESSAGE);
both('a capital as typed (the box lowers it first)', 'Has_Caps', USERNAME_CHARACTER_MESSAGE);
both('the first problem wins, space before dot', 'a b.c', USERNAME_SPACE_MESSAGE);
both('the first problem wins, dot before space', 'a.b c', USERNAME_DOT_MESSAGE);
both('the first problem wins, dash before dot', 'a-b.c', USERNAME_DASH_MESSAGE);
both('a dot in a short name names the dot, never the length', 'a.', USERNAME_DOT_MESSAGE);
both('21 is long, said at once', 'a'.repeat(21), USERNAME_LONG_MESSAGE);
eq('username empty says nothing while typing', typing(''), '');
eq('username empty asks once done', done(''), USERNAME_EMPTY_MESSAGE);
eq('sign-up has no stored name: empty asks once done', usernameProblem('', ''), USERNAME_EMPTY_MESSAGE);
eq('sign-up has no stored name: empty says nothing while typing', usernameProblem('', '', 'typing'), '');
eq('username 1 says nothing while typing', typing('a'), '');
eq('username 2 says nothing while typing', typing('ab'), '');
eq('username 1 is short once done', done('a'), USERNAME_SHORT_MESSAGE);
eq('username 2 is short once done', done('ab'), USERNAME_SHORT_MESSAGE);
eq('username the default is once done', usernameProblem('ab', stored), usernameProblem('ab', stored, 'done'));
both('3 is ok', 'abc', '');
both('20 is ok', 'a'.repeat(20), '');
both('lowered capitals are ok', 'Has_Caps'.toLowerCase(), '');
both('ok_name9 is ok', 'ok_name9', '');
both('underscores alone are ok', '___', '');
both('digits alone are ok', '123', '');
both('the stored name is never judged against itself', stored, '');
eq('a stored name that breaks the rule is never judged, while typing', usernameProblem('Odd Name', 'Odd Name', 'typing'), '');
eq('a stored name that breaks the rule is never judged, once done', usernameProblem('Odd Name', 'Odd Name'), '');

// Every mix of a few characters up to 4 long, and long ones. Once done, '' exactly for a username,
// so a grey button always has a line to show. While typing, a line only for what more typing can't
// fix, and then the same line as once done.
{
  const alphabet = ['a', 'z', '0', '9', '_', '.', ' ', '-', '@', 'A', 'é'];
  let level = [''];
  const values = [''];
  for (let length = 1; length <= 4; length += 1) {
    level = level.flatMap((prefix) => alphabet.map((ch) => prefix + ch));
    values.push(...level);
  }
  for (const n of [19, 20, 21, 25]) values.push('a'.repeat(n), 'a'.repeat(n - 1) + '.', '@' + 'a'.repeat(n - 1));
  const notJudgedLikeTheServer = values.filter((value) => (done(value) === '') !== USERNAME_PATTERN.test(value));
  const typingTooEarly = values.filter((value) => [USERNAME_EMPTY_MESSAGE, USERNAME_SHORT_MESSAGE].includes(typing(value)));
  const typingDiffers = values.filter((value) => typing(value) !== '' && typing(value) !== done(value));
  check(`once done, '' exactly for a username (${values.length} values)`, notJudgedLikeTheServer.length === 0, JSON.stringify(notJudgedLikeTheServer.slice(0, 5)));
  check('while typing, never the empty or short line', typingTooEarly.length === 0, JSON.stringify(typingTooEarly.slice(0, 5)));
  check('while typing, a line is the same as once done', typingDiffers.length === 0, JSON.stringify(typingDiffers.slice(0, 5)));
}

// utf8Length and passwordProblem (short under 8 characters, long over 72 BYTES).
eq('utf8 ascii', utf8Length('abc'), 3);
eq('utf8 two byte', utf8Length('é'), 2);
eq('utf8 three byte', utf8Length('€'), 3);
eq('utf8 four byte', utf8Length('\u{1F600}'), 4);
eq('password 7 is short', passwordProblem('Aa1!aaa'), PASSWORD_SHORT_MESSAGE);
eq('password empty is short', passwordProblem(''), PASSWORD_SHORT_MESSAGE);
eq('password 8 strong is ok', passwordProblem('Aa1!aaaa'), '');
eq('password 72 strong ascii is ok', passwordProblem('Aa1!' + 'a'.repeat(68)), '');
eq('password 73 ascii is long', passwordProblem('Aa1!' + 'a'.repeat(69)), PASSWORD_LONG_MESSAGE);
eq('password 40 two-byte (80 bytes) is long', passwordProblem('\u00c9'.repeat(38) + '1!'), PASSWORD_LONG_MESSAGE);
eq('password 8 plain lowercase is weak', passwordProblem('a'.repeat(8)), PASSWORD_WEAK_MESSAGE);
eq('password without a capital is weak', passwordProblem('aaaaaa1!'), PASSWORD_WEAK_MESSAGE);
eq('password without a number is weak', passwordProblem('Aaaaaaa!'), PASSWORD_WEAK_MESSAGE);
eq('password without a symbol is weak', passwordProblem('Aaaaaaa1'), PASSWORD_WEAK_MESSAGE);
eq('password with an emoji as the symbol is ok', passwordProblem('Aaaaaa1\u{1F600}'), '');
eq('password checks all met', passwordChecks('Aa1!aaaa').every((c) => c.met), true);
eq('password checks none met', passwordChecks('').some((c) => c.met), false);

// newPasswordProblem: the same rules plus "different from the current one".
eq('new password same as current', newPasswordProblem('Wheel1234!', 'Wheel1234!'), PASSWORD_SAME_MESSAGE);
eq('new password different', newPasswordProblem('Wheel12345!', 'Wheel1234!'), '');
eq('new password short wins over same', newPasswordProblem('abc', 'abc'), PASSWORD_SHORT_MESSAGE);
eq('new password long', newPasswordProblem('Aa1!' + 'a'.repeat(69), 'Wheel1234!'), PASSWORD_LONG_MESSAGE);
eq('new password differs only by case', newPasswordProblem('Wheel1234!', 'wheel1234!'), '');

// bioProblem at 160 and 161, and sameText.
eq('BIO_MAX', BIO_MAX, 160);
eq('bio empty', bioProblem(''), '');
eq('bio 160 is ok', bioProblem('a'.repeat(160)), '');
eq('bio 161 is long', bioProblem('a'.repeat(161)), BIO_LONG_MESSAGE);
eq('bio 160 plus trailing space is ok (trimmed as sent)', bioProblem('a'.repeat(160) + ' '), '');
eq('sameText trims', sameText('  hello ', 'hello'), true);
eq('sameText differs', sameText('hello', 'hello!'), false);
eq('sameText empty and spaces', sameText('', '   '), true);
eq('sameText keeps inner line breaks', sameText('a\nb', 'a b'), false);

eq('the empty current password line', CURRENT_PASSWORD_EMPTY_MESSAGE, 'Enter your current password.');

// The birthday on a tap of the grey Continue (lib/profile/birthday.ts).
{
  const { BIRTHDAY_MESSAGES, EMPTY_BIRTHDAY, birthdayProblem, birthdayBoxToFix } = birthday;
  const now = new Date(2026, 10, 16, 12);
  const good = { month: '1', day: '2', year: '2000' };
  const young = { month: '1', day: '2', year: '2020' };
  eq('birthday never picked line', BIRTHDAY_MESSAGES.notPicked, 'Pick your birthday.');
  eq('the date sheet, never picked, asks for one', birthdayProblem(EMPTY_BIRTHDAY, true, now), BIRTHDAY_MESSAGES.notPicked);
  eq('the typed boxes, empty, keep their own line', birthdayProblem(EMPTY_BIRTHDAY, false, now), BIRTHDAY_MESSAGES.incomplete);
  eq('the typed boxes, half filled', birthdayProblem({ month: '1', day: '', year: '' }, false, now), BIRTHDAY_MESSAGES.incomplete);
  eq('a good date says nothing', birthdayProblem(good, true, now), '');
  eq('under 13 says so, with the sheet too', birthdayProblem(young, true, now), BIRTHDAY_MESSAGES.underAge);
  eq('the cursor goes to Month first', birthdayBoxToFix(EMPTY_BIRTHDAY, BIRTHDAY_MESSAGES.incomplete), 'month');
  eq('then Day', birthdayBoxToFix({ month: '1', day: '', year: '' }, BIRTHDAY_MESSAGES.incomplete), 'day');
  eq('then Year', birthdayBoxToFix({ month: '1', day: '2', year: '' }, BIRTHDAY_MESSAGES.incomplete), 'year');
  eq('Year for a short year', birthdayBoxToFix({ month: '1', day: '2', year: '19' }, BIRTHDAY_MESSAGES.year), 'year');
  eq('Year for a date in the future', birthdayBoxToFix({ month: '1', day: '2', year: '2030' }, BIRTHDAY_MESSAGES.future), 'year');
  eq('Month for a date that does not exist', birthdayBoxToFix({ month: '2', day: '31', year: '2000' }, BIRTHDAY_MESSAGES.notReal), 'month');
  eq('no cursor for under 13', birthdayBoxToFix(young, BIRTHDAY_MESSAGES.underAge), null);
  eq('no cursor without a problem', birthdayBoxToFix(good, ''), null);
}

// Every new line: one short sentence, no em dash, no exclamation mark.
for (const text of [
  USERNAME_DOT_MESSAGE,
  USERNAME_SPACE_MESSAGE,
  USERNAME_DASH_MESSAGE,
  USERNAME_AT_MESSAGE,
  USERNAME_CHARACTER_MESSAGE,
  USERNAME_EMPTY_MESSAGE,
  USERNAME_SHORT_MESSAGE,
  USERNAME_LONG_MESSAGE,
  CURRENT_PASSWORD_EMPTY_MESSAGE,
  birthday.BIRTHDAY_MESSAGES.notPicked,
]) {
  check(`short, ends with a period, no em dash or exclamation mark: ${text}`, text.length <= 30 && /\.$/.test(text) && !/[—!]/.test(text));
}

// ---- What the pages do with these (no React here to run them, so pinned as written) ----------

{
  const form = source('components/profile/ProfileForm.tsx');
  check('ProfileForm has no username rule of its own', !/USERNAME_PATTERN|USERNAME_FORMAT_MESSAGE|a-z0-9_/.test(form));
  check(
    'the old rule line is gone from the page and the rules',
    !form.includes('3 to 20 letters') && !source('lib/settings/rules.ts').includes('3 to 20 letters')
  );
  check(
    'a name that is not a username yet is never checked for being free, and typing says only what typing cannot fix',
    /if \(usernameProblem\(username, initialUsername\) !== ''\) \{[^}]*setUsernameError\(usernameProblem\(username, initialUsername, 'typing'\) \|\| undefined\);[^}]*return;\s*\}\s*setUsernameError\(undefined\);\s*setUsernameStatus\('checking'\);/.test(form)
  );
  check(
    'leaving the box says empty and short, and keeps a taken line',
    /function handleUsernameBlur\(\) \{\s*const problem = usernameProblem\(username, initialUsername\);\s*if \(problem !== ''\) setUsernameError\(problem\);\s*\}/.test(form)
  );
  check('Continue and Save follow the same rule', /const usernameValid = usernameProblem\(username, initialUsername\) === '';/.test(form));
  check('a server refusal always has a line', /setUsernameError\(usernameProblem\(username, initialUsername\) \|\| USERNAME_CHARACTER_MESSAGE\);/.test(form));
  {
    // A cap would cut a pasted 21+ name to 20 without a word, and the cut name could pass the check.
    const start = form.indexOf('label="Username"');
    const box = start < 0 ? '' : form.slice(start, form.indexOf('/>', start));
    check('the username box has no length cap, so a long paste is named, not cut short', box.length > 0 && !/maxLength/.test(box));
  }
  check('capitals are still lowered as typed', /setUsername\(value\.toLowerCase\(\)\);/.test(form));

  check('the page gives Screen its scroll view', /scrollRef=\{scrollRef\}>/.test(form));
  check(
    'Continue and Save changes: a tap while grey shows the problems, and VoiceOver hears the Still needed line',
    /onBlockedPress=\{problems\.length > 0 \? showProblems : undefined\}/.test(form) &&
      /blockedHint=\{problems\.length > 0 \? stillNeeded\(problems\.map\(\(problem\) => problem\.label\.toLowerCase\(\)\)\) : undefined\}/.test(form)
  );
  check('an Edit page with nothing changed has no problem, so the tap does nothing', /if \(signup \|\| \(edit && hasChanges\)\) \{/.test(form));
  check(
    'a username being checked is no problem, a taken one is',
    /usernameStatus === 'taken' \? takenMessage\(usernameAlternates\) : usernameProblem\(username, initialUsername\)/.test(form)
  );
  const tap = form.slice(form.indexOf('function showProblems()'), form.indexOf('function showEmailLine('));
  check(
    'the tap shows every field, touched or not',
    [
      'setEmailError(emailProblem(email));',
      'setPasswordError(passwordProblem(password));',
      'setPasswordAsked(true);',
      'setBirthdayError(birthdayProblem(birthday, birthdayPickerAvailable) || undefined);',
      'setNameError(nameProblem(name));',
      "if (usernameLine !== '') setUsernameError(usernameLine);",
    ].every((line) => tap.includes(line))
  );
  check(
    'the first problem is said, scrolled to, then gets the cursor',
    /announceProblem\(first\.message, first\.label\);\s*bringIntoView\(scrollRef\.current, fieldRefs\[first\.key\]\.current\);[\s\S]*boxRefs\[key\]\.current\?\.focus\(\);/.test(tap)
  );
  check(
    'a birthday never picked opens the sheet, and under 13 opens nothing',
    /\} else if \(first\.message === BIRTHDAY_MESSAGES\.notPicked\) \{\s*birthdayRef\.current\?\.open\(\);/.test(tap) &&
      count(form, 'birthdayRef.current?.open()') === 1
  );
  check(
    'the Password box names what is missing once the grey button was tapped',
    /error=\{passwordAsked \|\| passwordError === PASSWORD_LONG_MESSAGE \? passwordError : undefined\}/.test(form)
  );
  check(
    'a taken email brings the Email box into view',
    /setTakenEmail\(email\.trim\(\)\.toLowerCase\(\)\);\s*showEmailLine\(EMAIL_TAKEN_MESSAGE\);/.test(form) &&
      /bringIntoView\(scrollRef\.current, fieldRefs\.email\.current\);/.test(form)
  );
  check(
    'photo access that is off uses the shared alert',
    /showAccessOff\('camera'\);/.test(form) && /showAccessOff\('library'\);/.test(form) && !/access is needed/.test(form)
  );
  check(
    'the photo and typed birthday lines have the error mark',
    /<FieldError text=\{avatarError\} label="Photo" \/>/.test(form) && /<FieldError text=\{birthdayError\} label="Birthday" \/>/.test(form)
  );
  check(
    "a birthday problem takes the place of the birthday's hint",
    /\{birthdayError \? null : \(\s*<View style=\{\{ marginTop: space\.xs \}\}>\s*<AppText role="label" tone="muted">\s*Used only to check your age/.test(form)
  );
}
{
  const page = source('components/settings/PasswordScreen.tsx');
  check('Save password: the page gives Screen its scroll view', /<Screen edges=\{\['top', 'left', 'right'\]\} scrollRef=\{scrollRef\}>/.test(page));
  check(
    'Save password: a tap while grey shows the problems, with its Still needed hint',
    /onBlockedPress=\{showProblems\}/.test(page) &&
      /blockedHint=\{stillNeeded\(\[currentMissing \? 'current password' : '', nextProblem !== '' \? 'new password' : ''\]\)\}/.test(page)
  );
  check('Save password: an empty current box asks for it', /if \(currentMissing\) setCurrentError\(CURRENT_PASSWORD_EMPTY_MESSAGE\);/.test(page));
  check(
    "Save password: the new box names the checklist's problems after the tap",
    /nextProblem !== '' && \(asked \|\| \(nextTouched && !isChecklistProblem\(nextProblem\)\)\)/.test(page) && /setAsked\(true\);/.test(page)
  );
  check(
    'Save password: the current box first, then the new one',
    /if \(currentMissing\) \{\s*announceProblem\(CURRENT_PASSWORD_EMPTY_MESSAGE, 'Current password'\);\s*bringIntoView\(scrollRef\.current, currentFieldRef\.current\);\s*currentBoxRef\.current\?\.focus\(\);\s*\} else if \(nextProblem !== ''\) \{\s*announceProblem\(nextProblem, 'New password'\);\s*bringIntoView\(scrollRef\.current, nextFieldRef\.current\);\s*nextBoxRef\.current\?\.focus\(\);/.test(page)
  );
}
{
  const page = source('components/settings/DeleteAccountScreen.tsx');
  check("Delete account: the line is the Log in page's", /const PASSWORD_REQUIRED_MESSAGE = 'Enter your password\.';/.test(page));
  check(
    'Delete account: a tap while grey asks for the password, says it, scrolls to it and puts the cursor in it',
    /onBlockedPress=\{showProblem\}/.test(page) &&
      /function showProblem\(\) \{\s*setPasswordError\(PASSWORD_REQUIRED_MESSAGE\);\s*announceProblem\(PASSWORD_REQUIRED_MESSAGE, 'Password'\);\s*bringIntoView\(scrollRef\.current, passwordFieldRef\.current\);\s*passwordBoxRef\.current\?\.focus\(\);/.test(page)
  );
  check('Delete account: its Still needed hint', /blockedHint=\{stillNeeded\(\['password'\]\)\}/.test(page));
}
{
  // Settings > Notifications (D-16 order, 2026-11-25): Likes first, kept in the saved reactions value,
  // then Comments, then Milestones and bonus spins.
  const page = source('components/settings/NotificationSettingsScreen.tsx');
  const rows = page.match(/const ROWS: \{ key: NotificationSettingKey; label: string \}\[\] = \[([\s\S]*?)\];/)?.[1] ?? '';
  const order = [...rows.matchAll(/\{ key: '(\w+)', label: '([^']+)' \}/g)].map((m) => `${m[1]}:${m[2]}`);
  eq(
    'Notifications: the switches are Likes, Comments, then Milestones',
    order.join(' | '),
    'reactions:Likes | comments:Comments | milestones:Milestones and bonus spins'
  );
  check('Notifications: no comment says likes have no switch', !/with no switch|likes have no switch/i.test(page));
}

// The notice store: set, read, clear by timer, restart on a new text, clear at once on ''.
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
let calls = 0;
const unsubscribe = notice.subscribeSettingsNotice(() => {
  calls += 1;
});
eq('notice starts empty', notice.getSettingsNotice(), '');
notice.setSettingsNotice('Username changed.');
eq('notice set', notice.getSettingsNotice(), 'Username changed.');
check('notice emitted on set', calls === 1, `calls ${calls}`);
await sleep(2000);
eq('notice still there at 2s', notice.getSettingsNotice(), 'Username changed.');
notice.setSettingsNotice('Password changed. Other phones are signed out.');
eq('notice replaced', notice.getSettingsNotice(), 'Password changed. Other phones are signed out.');
await sleep(2000);
eq('a new text restarted the timer (still there 2s after the replace)', notice.getSettingsNotice(), 'Password changed. Other phones are signed out.');
await sleep(1300);
eq('notice cleared after 3s', notice.getSettingsNotice(), '');
check('notice emitted on clear (set, replace, clear)', calls === 3, `calls ${calls}`);
notice.setSettingsNotice('Username changed.');
notice.setSettingsNotice('');
eq('empty text clears at once', notice.getSettingsNotice(), '');
await sleep(3300);
eq('no timer left behind', notice.getSettingsNotice(), '');
unsubscribe();
const before = calls;
notice.setSettingsNotice('x');
notice.setSettingsNotice('');
check('an unsubscribed listener is not called', calls === before, `calls ${calls}, before ${before}`);

console.log(`${checks} checks, ${failures} failures`);
process.exit(failures === 0 ? 0 : 1);
