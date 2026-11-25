// @ts-nocheck
/**
 * Throwaway checks for where a tapped phone push goes (lib/notifications/pushTarget.ts), for the
 * morning note's text and time (lib/notifications/postNote.ts) and for the 8 PM reminder's dates,
 * ids and switch (lib/notifications/reminderRules.ts, run in New York). Not part
 * of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-push.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts.)
 */
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const { pushTarget, PUSH_KINDS } = await import(pathToFileURL(join(process.cwd(), 'lib/notifications/pushTarget.ts')).href);
const { POST_NOTE_TITLE, postNoteBody, postNoteDate } = await import(pathToFileURL(join(process.cwd(), 'lib/notifications/postNote.ts')).href);

let checks = 0;
let failures = 0;
function check(name, got, want) {
  checks += 1;
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    failures += 1;
    console.log(`FAIL ${name}: ${JSON.stringify(got)}`);
  }
}

check('comment opens your entry', pushTarget({ kind: 'comment', entry_id: 'e1', own: 'true' }), {
  pathname: '/(app)/today/entry',
  params: { id: 'e1' },
});
check('reply on your post opens your entry', pushTarget({ kind: 'reply', entry_id: 'e1', own: 'true' }), {
  pathname: '/(app)/today/entry',
  params: { id: 'e1' },
});
check('reply on their post opens the post', pushTarget({ kind: 'reply', entry_id: 'e2', own: 'false' }), {
  pathname: '/(app)/today/post',
  params: { id: 'e2' },
});
check('request opens follow requests', pushTarget({ kind: 'follow_request', actor_id: 'u1' }), {
  pathname: '/(app)/today/follow-requests',
});
check('accepted opens the person', pushTarget({ kind: 'follow_accepted', actor_id: 'u2' }), {
  pathname: '/(app)/today/person',
  params: { id: 'u2' },
});
check('the reminder is not a push', pushTarget({ url: '/(app)/today' }), null);
check('no data', pushTarget(undefined), null);
check('comment without entry', pushTarget({ kind: 'comment' }), null);
check('unknown kind', pushTarget({ kind: 'spin_open' }), null);
// A like is a phone push too (2026-11-25): a tap opens your own entry page, as the bell's like line does.
check('like opens your entry', pushTarget({ kind: 'like', entry_id: 'e1', own: 'true', actor_id: 'u1' }), {
  pathname: '/(app)/today/entry',
  params: { id: 'e1' },
});
check('like without entry', pushTarget({ kind: 'like', own: 'true', actor_id: 'u1' }), null);
check('the five push kinds', PUSH_KINDS, ['comment', 'reply', 'like', 'follow_request', 'follow_accepted']);

// The morning note: two hours before late_from, never in the past, none without a waiting spin.
const noon = Date.parse('2026-10-29T06:30:00.000Z'); // noon in Kolkata
check('note at 10 AM', postNoteDate(Date.parse('2026-10-28T15:00:00.000Z'), noon)?.toISOString(), '2026-10-29T04:30:00.000Z');
check('note just before 10 AM', postNoteDate(noon - 2 * 3600_000 - 1, noon)?.toISOString(), '2026-10-29T04:30:00.000Z');
check('no note from 10 AM', postNoteDate(noon - 2 * 3600_000, noon), null);
check('no note after noon', postNoteDate(noon + 1, noon), null);
check('no note without a spin', postNoteDate(Date.parse('2026-10-28T15:00:00.000Z'), null), null);
check('note title', POST_NOTE_TITLE, 'Beat the noon bell');
check('note line', postNoteBody('Movie'), 'Post your movie pick to stay on time.');

// The 8 PM reminder (lib/notifications/reminderRules.ts, NOT-03), on a phone in New York: Node
// reads TZ again when it is set, so the dates below are New York wall times on any machine. The
// checks above use absolute times only.
process.env.TZ = 'America/New_York';
const R = await import(pathToFileURL(join(process.cwd(), 'lib/notifications/reminderRules.ts')).href);
const ny = (y, mo, d, h, mi = 0, s = 0) => new Date(y, mo - 1, d, h, mi, s).getTime();
const wall = (date) =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ` +
  `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}:${String(date.getSeconds()).padStart(2, '0')}`;
const firstOf = (nowMs, nextOpenMs) => {
  const dates = R.reminderDates(nowMs, nextOpenMs);
  return dates.length === 0 ? null : wall(dates[0]);
};
check('the zone is New York', new Date(Date.UTC(2026, 9, 21, 0, 0)).getHours(), 20);

// 3 PM, the wheel opens tonight: tonight first, then every evening, 30 in all, each at 8:00 PM.
let dates = R.reminderDates(ny(2026, 10, 20, 15), ny(2026, 10, 20, 20));
check('30 evenings ahead', [dates.length, R.REMINDER_DAYS_AHEAD], [30, 30]);
check('tonight first, then each evening', dates.slice(0, 3).map(wall), ['2026-10-20 20:00:00', '2026-10-21 20:00:00', '2026-10-22 20:00:00']);
check('the last one 29 evenings on', wall(dates[29]), '2026-11-18 20:00:00');
check('one a day, every one at 8:00 PM', dates.every((d, i) => wall(d) === wall(new Date(2026, 9, 20 + i, 20, 0, 0))), true);
// A first spin this morning: the next opening is tomorrow, so no reminder tonight.
check('no reminder on the evening of a first spin', firstOf(ny(2026, 10, 20, 15), ny(2026, 10, 21, 20)), '2026-10-21 20:00:00');
check('still 30 from tomorrow', R.reminderDates(ny(2026, 10, 20, 15), ny(2026, 10, 21, 20)).length, 30);
// Past 8 PM, or at it, tonight is gone.
check('at 8:30 PM tomorrow comes first', firstOf(ny(2026, 10, 20, 20, 30), ny(2026, 10, 20, 20)), '2026-10-21 20:00:00');
check('at 8:00 PM sharp tomorrow comes first', firstOf(ny(2026, 10, 20, 20), ny(2026, 10, 20, 20)), '2026-10-21 20:00:00');
check('a minute before 8 PM tonight still comes', firstOf(ny(2026, 10, 20, 19, 59), ny(2026, 10, 20, 20)), '2026-10-20 20:00:00');
// A minute of slack: an opening up to a minute after 8 PM still counts tonight, later does not.
check('an opening at 7:59:30 PM', firstOf(ny(2026, 10, 20, 15), ny(2026, 10, 20, 19, 59, 30)), '2026-10-20 20:00:00');
check('an opening at 8:00:59 PM', firstOf(ny(2026, 10, 20, 15), ny(2026, 10, 20, 20, 0, 59)), '2026-10-20 20:00:00');
check('an opening at 8:01:00 PM', firstOf(ny(2026, 10, 20, 15), ny(2026, 10, 20, 20, 1, 0)), '2026-10-20 20:00:00');
check('an opening at 8:01:01 PM waits a day', firstOf(ny(2026, 10, 20, 15), ny(2026, 10, 20, 20, 1, 1)), '2026-10-21 20:00:00');
// Across the clock change on Nov 1 every evening is still 8:00 PM on the phone.
dates = R.reminderDates(ny(2026, 10, 28, 9), ny(2026, 10, 28, 20));
check('across Nov 1: every evening at 8:00 PM', dates.every((d, i) => wall(d) === wall(new Date(2026, 9, 28 + i, 20, 0, 0))), true);
check('across Nov 1: the instants move with the clock', [dates[3].toISOString(), dates[4].toISOString()], ['2026-11-01T00:00:00.000Z', '2026-11-02T01:00:00.000Z']);
check('fewer evenings when asked', R.reminderDates(ny(2026, 10, 20, 15), ny(2026, 10, 20, 20), 3).map(wall), [
  '2026-10-20 20:00:00',
  '2026-10-21 20:00:00',
  '2026-10-22 20:00:00',
]);

// The reminder's own ids, which a tap and the push handler recognise.
check('the reminder ids', ['rndmroll-daily-spin-reminder', 'rndmroll-daily-spin-reminder-0', 'rndmroll-daily-spin-reminder-post'].map(R.isReminderId), [true, true, true]);
check('other ids', ['rndmroll-daily-spin-reminderx', 'push-1', ''].map(R.isReminderId), [false, false, false]);

// The switch shows on, and a reminder is scheduled, only while it is on and the phone allows it.
for (const permission of ['granted', 'denied', 'undetermined', 'unavailable']) {
  for (const on of [true, false]) {
    const want = on && permission === 'granted';
    check(`schedule ${permission} ${on}`, R.shouldSchedule(on, permission), want);
    check(`switch shows ${permission} ${on}`, R.switchShowsOn(on, permission), want);
  }
  // The line under the switch only when the person turned the permission off.
  check(`line ${permission}`, R.permissionLine(permission), permission === 'denied' ? 'Turn on notifications for RNDMRoll in Settings.' : '');
}
check('the permission answer', [
  R.permissionFrom({ granted: true, status: 'granted' }),
  R.permissionFrom({ granted: false, status: 'undetermined' }),
  R.permissionFrom({ granted: false, status: 'denied' }),
  R.permissionFrom({ granted: true, status: 'undetermined' }),
], ['granted', 'undetermined', 'denied', 'granted']);
check('the line has no em dash and ends with a full stop', /[–—]/.test(R.PERMISSION_OFF_MESSAGE) || !R.PERMISSION_OFF_MESSAGE.endsWith('.'), false);

// The app reads these from reminder.ts: every one is still exported there, and none is written
// there twice.
const reminderSource = readFileSync(join(process.cwd(), 'lib/notifications/reminder.ts'), 'utf8');
const exported = reminderSource.match(/export \{([^}]*)\} from '@\/lib\/notifications\/reminderRules';/)?.[1] ?? '';
for (const name of ['isReminderId', 'PERMISSION_OFF_MESSAGE', 'permissionFrom', 'permissionLine', 'REMINDER_DAYS_AHEAD', 'REMINDER_HOUR', 'REMINDER_ID', 'REMINDER_MINUTE', 'reminderDates', 'shouldSchedule', 'switchShowsOn']) {
  check(`reminder.ts exports ${name}`, new RegExp(`\\b${name}\\b`).test(exported), true);
  check(`reminder.ts does not define ${name} again`, new RegExp(`export (const|function) ${name}\\b`).test(reminderSource), false);
}
check('reminder.ts exports the permission type', /export type \{ ReminderPermission \} from '@\/lib\/notifications\/reminderRules';/.test(reminderSource), true);
check('reminderRules.ts has no import', /^import /m.test(readFileSync(join(process.cwd(), 'lib/notifications/reminderRules.ts'), 'utf8')), false);

console.log(`${checks - failures}/${checks} push checks passed`);
if (failures > 0) process.exitCode = 1;
