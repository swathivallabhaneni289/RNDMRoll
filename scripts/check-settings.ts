// @ts-nocheck
/**
 * Throwaway checks for the pure Settings rules and the notice store (plan 02.1-02 Task 1). Not part
 * of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types <this file>
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * Files are imported by absolute path built from the current directory, so the script can live in
 * scripts/ or anywhere else. (The nocheck line is because Node needs the .ts extension on imports
 * and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.cwd();
const rules = await import(pathToFileURL(join(root, 'lib/settings/rules.ts')).href);
const notice = await import(pathToFileURL(join(root, 'lib/settings/notice.ts')).href);

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

const {
  usernameProblem,
  passwordProblem,
  newPasswordProblem,
  bioProblem,
  sameText,
  utf8Length,
  USERNAME_FORMAT_MESSAGE,
  PASSWORD_SHORT_MESSAGE,
  PASSWORD_LONG_MESSAGE,
  PASSWORD_SAME_MESSAGE,
  BIO_LONG_MESSAGE,
  BIO_MAX,
} = rules;

// usernameProblem: '' when there is nothing to say, else the format message.
eq('username empty', usernameProblem('', 'p21_walker'), '');
eq('username equals stored', usernameProblem('p21_walker', 'p21_walker'), '');
eq('username stored but odd (never judged against itself)', usernameProblem('Odd Name', 'Odd Name'), '');
eq('username too short', usernameProblem('ab', 'p21_walker'), USERNAME_FORMAT_MESSAGE);
eq('username 3 is ok', usernameProblem('abc', 'p21_walker'), '');
eq('username 20 is ok', usernameProblem('a'.repeat(20), 'p21_walker'), '');
eq('username 21 is long', usernameProblem('a'.repeat(21), 'p21_walker'), USERNAME_FORMAT_MESSAGE);
eq('username with capitals is refused as typed', usernameProblem('Has_Caps', 'p21_walker'), USERNAME_FORMAT_MESSAGE);
eq('username lowered is ok', usernameProblem('Has_Caps'.toLowerCase(), 'p21_walker'), '');
eq('username ok_name9', usernameProblem('ok_name9', 'p21_walker'), '');
eq('username with a space', usernameProblem('has space', 'p21_walker'), USERNAME_FORMAT_MESSAGE);
eq('username with a dash', usernameProblem('has-dash', 'p21_walker'), USERNAME_FORMAT_MESSAGE);

// utf8Length and passwordProblem (short under 8 characters, long over 72 BYTES).
eq('utf8 ascii', utf8Length('abc'), 3);
eq('utf8 two byte', utf8Length('é'), 2);
eq('utf8 three byte', utf8Length('€'), 3);
eq('utf8 four byte', utf8Length('\u{1F600}'), 4);
eq('password 7 is short', passwordProblem('a'.repeat(7)), PASSWORD_SHORT_MESSAGE);
eq('password empty is short', passwordProblem(''), PASSWORD_SHORT_MESSAGE);
eq('password 8 is ok', passwordProblem('a'.repeat(8)), '');
eq('password 72 ascii is ok', passwordProblem('a'.repeat(72)), '');
eq('password 73 ascii is long', passwordProblem('a'.repeat(73)), PASSWORD_LONG_MESSAGE);
eq('password 40 two-byte (80 bytes) is long', passwordProblem('é'.repeat(40)), PASSWORD_LONG_MESSAGE);
eq('password 36 two-byte (72 bytes) is ok', passwordProblem('é'.repeat(36)), '');
eq('password 4 emoji (4 characters) is short', passwordProblem('\u{1F600}'.repeat(4)), PASSWORD_SHORT_MESSAGE);
eq('password 8 emoji (32 bytes) is ok', passwordProblem('\u{1F600}'.repeat(8)), '');

// newPasswordProblem: the same rules plus "different from the current one".
eq('new password same as current', newPasswordProblem('wheel1234', 'wheel1234'), PASSWORD_SAME_MESSAGE);
eq('new password different', newPasswordProblem('wheel12345', 'wheel1234'), '');
eq('new password short wins over same', newPasswordProblem('abc', 'abc'), PASSWORD_SHORT_MESSAGE);
eq('new password long', newPasswordProblem('a'.repeat(73), 'wheel1234'), PASSWORD_LONG_MESSAGE);
eq('new password differs only by case', newPasswordProblem('Wheel1234', 'wheel1234'), '');

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
