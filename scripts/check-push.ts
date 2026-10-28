// @ts-nocheck
/**
 * Throwaway checks for where a tapped phone push goes (lib/notifications/pushTarget.ts) and for the
 * morning note's text and time (lib/notifications/postNote.ts). Not part
 * of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-push.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts.)
 */
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const { pushTarget } = await import(pathToFileURL(join(process.cwd(), 'lib/notifications/pushTarget.ts')).href);
const { postNoteBody, postNoteDate } = await import(pathToFileURL(join(process.cwd(), 'lib/notifications/postNote.ts')).href);

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

// The morning note: two hours before late_from, never in the past, none without a waiting spin.
const noon = Date.parse('2026-10-29T06:30:00.000Z'); // noon in Kolkata
check('note at 10 AM', postNoteDate(Date.parse('2026-10-28T15:00:00.000Z'), noon)?.toISOString(), '2026-10-29T04:30:00.000Z');
check('note just before 10 AM', postNoteDate(noon - 2 * 3600_000 - 1, noon)?.toISOString(), '2026-10-29T04:30:00.000Z');
check('no note from 10 AM', postNoteDate(noon - 2 * 3600_000, noon), null);
check('no note after noon', postNoteDate(noon + 1, noon), null);
check('no note without a spin', postNoteDate(Date.parse('2026-10-28T15:00:00.000Z'), null), null);
check('note text', postNoteBody('Movie'), 'Your movie is waiting. Post it by noon to be on time.');

console.log(`${checks - failures}/${checks} push checks passed`);
