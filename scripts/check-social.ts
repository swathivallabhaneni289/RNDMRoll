// @ts-nocheck
/**
 * Throwaway checks for the social screens' fixes: a comment on a post that went away, a follow
 * request that went away, an invite link that leads to no one, where your posts list opens, the
 * two Settings pages when the first read fails, a profile photo that would not load, the comment
 * box's emoji row and reply prefix, and the report reasons on the phone, the server, the admin page
 * and the database. Not part of the app bundle. Run with:  node --experimental-strip-types scripts/check-social.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 * The pure rules are run; the hooks and screens have no React here, so the lines that matter are
 * pinned as written.
 */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { POST_GONE_LINE, commentFailureLine } from '../lib/board/text.ts';
import { placementStep } from '../lib/board/placement.ts';
import { countAfterComment, placeComment, withEmoji, withReplyPrefix } from '../lib/board/comments.ts';
import { COMMENT_EMOJIS } from '../lib/entry/reactions.ts';
import { COMMENT_MAX } from '../lib/entry/rules.ts';

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

// ---- a comment sent again after a lost answer ----------------------------------------------------
{
  const a = { id: 'a', parent_id: null };
  const a1 = { id: 'a1', parent_id: 'a' };
  const b = { id: 'b', parent_id: null };
  const ids = (list: { id: string }[]) => list.map((item) => item.id).join(',');
  eq('a new comment adds one', countAfterComment([a], 1, b), 2);
  eq('a comment a reload already shows is counted once', countAfterComment([a, b], 2, b), 2);
  eq('with no count from the server, the list counts', countAfterComment([a], null, b), 2);
  eq('a comment already shown is placed once', ids(placeComment([a, b], b)), 'a,b');
  eq('a reply goes after the last reply of its thread', ids(placeComment([a, a1, b], { id: 'a2', parent_id: 'a' })), 'a,a1,a2,b');
  eq('a comment goes at the end', ids(placeComment([a, a1], b)), 'a,a1,b');
  const post = source('components/board/usePost.ts');
  check(
    'the post hook counts through countAfterComment',
    /comment_count: countAfterComment\(list, now\.card\.comment_count, saved\)/.test(post) && !/comment_count \?\? list\.length\) \+ 1/.test(post)
  );
}

// ---- a comment on a post that went away ----------------------------------------------------------
{
  eq('the gone line is the post page line', POST_GONE_LINE, "This post isn't here any more.");
  eq('a comment answered not found says the post is gone', commentFailureLine('not_found', 'comment'), POST_GONE_LINE);
  eq('a reply answered not found names the comment', commentFailureLine('not_found', 'reply'), "That comment isn't here any more.");
  eq('comments off, on a comment', commentFailureLine('comments_off', 'comment'), 'Commenting is off.');
  eq('comments off, on a reply', commentFailureLine('comments_off', 'reply'), 'Commenting is off.');
  eq('a comment that failed otherwise', commentFailureLine('server_error', 'comment'), "Couldn't post your comment. Please try again.");
  eq('a comment that failed with no code', commentFailureLine(null, 'comment'), "Couldn't post your comment. Please try again.");
  eq('a reply that failed otherwise', commentFailureLine('server_error', 'reply'), "Couldn't post your reply. Please try again.");
  eq('a delete answered not found never says the post is gone', commentFailureLine('not_found', 'other'), 'Something went wrong. Please try again.');
  for (const code of ['not_found', 'comments_off', 'server_error', null]) {
    for (const what of ['comment', 'reply', 'other']) {
      const line = commentFailureLine(code, what);
      check(`no em dash or en dash: ${code} ${what}`, !/[–—]/.test(line), line);
      check(`one plain sentence: ${code} ${what}`, /^[A-Z].*\.$/.test(line), line);
    }
  }

  const board = source('lib/api/board.ts');
  check(
    'socialErrorMessage keeps the client words for no connection and too many tries',
    /err\.code === 'network_unavailable' \|\| err\.code === 'rate_limited'\)\) return err\.userMessage;/.test(board)
  );
  check(
    'socialErrorMessage asks commentFailureLine for every other line',
    /return commentFailureLine\(err instanceof ApiError \? err\.code : null, what\);/.test(board)
  );

  const post = source('components/board/usePost.ts');
  check(
    'a comment answered not found reads the post again, a top-level comment too; a reply also ends reply mode',
    /if \(err instanceof ApiError && err\.code === 'not_found'\) \{(?:\s*\/\/[^\n]*)*\s*if \(parent !== null && mountedRef\.current && replyToRef\.current === parent\) chooseReply\(null\);\s*void load\(\);\s*\}\s*return socialErrorMessage\(err, parent !== null \? 'reply' : 'comment'\);/.test(
      post
    )
  );
  check('the post page prints the same gone line', /\{POST_GONE_LINE\}/.test(source('components/board/PostView.tsx')));
}

// ---- a follow request that went away -------------------------------------------------------------
{
  const social = source('lib/api/social.ts');
  check(
    'answerRequest gives gone for not found and passes every other failure on',
    /export async function answerRequest\([\s\S]*?return 'done';[\s\S]*?err\.code === 'not_found'\) return 'gone';\s*throw err;/.test(social)
  );
  check(
    'Accept and Decline both go through answerRequest (People needs no change: no line, and its reload drops the row)',
    /export async function acceptRequest\(requestId: string\): Promise<void> \{\s*await answerRequest\(requestId, 'accept'\);\s*\}/.test(social) &&
      /export async function declineRequest\(requestId: string\): Promise<void> \{\s*await answerRequest\(requestId, 'decline'\);\s*\}/.test(social)
  );

  const requests = source('components/notifications/FollowRequestsScreen.tsx');
  check(
    'Follow requests: an Accept that comes back gone drops the row unless they follow you now',
    /const answer = await answerRequest\(request\.request_id, 'accept'\);[\s\S]*?if \(answer === 'gone' && !lists\?\.followers\.some\(\(p\) => p\.id === request\.person\.id\)\) \{[\s\S]*?setRequests\([\s\S]*?\);\s*return;\s*\}\s*showAccepted\(request, lists\);/.test(
      requests
    )
  );
  check(
    'Follow requests: a Decline that comes back gone drops the row, unless an earlier Accept went through',
    /const answer = await answerRequest\(request\.request_id, 'decline'\);\s*if \(answer === 'gone'\) \{(?:\s*\/\/[^\n]*)*\s*const lists = await getFollows\(\)\.catch\(\(\) => null\);\s*if \(lists\?\.followers\.some\(\(p\) => p\.id === request\.person\.id\)\) \{\s*showAccepted\(request, lists\);\s*return;\s*\}\s*\}\s*setRequests\(\(now\) => \(now \?\? \[\]\)\.filter\(\(r\) => r\.request_id !== request\.request_id\)\);/.test(
      requests
    )
  );
}

// ---- an invite link that leads to no one ---------------------------------------------------------
{
  const route = source('app/(app)/board/person.tsx');
  check('the person route never hands the profile an empty id', !/setId\(/.test(route) && !/id=\{[^}]*\?\? ''/.test(route));
  check(
    'the profile is opened only with the given id or the id the lookup found',
    (route.match(/<PersonScreen id=\{([^}]+)\} \/>/g) ?? []).join(' ') === '<PersonScreen id={given} /> <PersonScreen id={lookup.id} />'
  );
  check(
    'a username the lookup does not find shows the gone line, with no Try again',
    /if \(!username \|\| lookup\.state === 'gone'\) \{\s*return \(\s*<LinkPage>\s*<AppText role="body">\{GONE_LINE\}<\/AppText>/.test(route) &&
      /const GONE_LINE = "This account isn't available\.";/.test(route)
  );
  check(
    'a failed lookup shows Try again, which looks the username up again',
    /<StatusMessage kind=\{lookup\.failure\} onRetry=\{retry\} busy=\{lookup\.retrying\} \/>/.test(route) &&
      /setAttempt\(\(n\) => n \+ 1\);/.test(route) &&
      /\}, \[given, username, own, router, attempt\]\);/.test(route)
  );
  check('the gone and failed pages have a way back', /router\.replace\('\/\(app\)\/board'\)/.test(route));
}

// ---- where your posts list opens -----------------------------------------------------------------
{
  const base = { startId: 'p25', ids: ['p1', 'p2'], ready: true, loadingMore: false, moreFailed: false, askedAtThese: false };
  eq('the post is on the first page: open there', placementStep({ ...base, ids: ['p1', 'p25'] }), 'place');
  eq('the post is in even while a page is on its way', placementStep({ ...base, ids: ['p25'], loadingMore: true }), 'place');
  eq('not in yet with pages to come: ask for the next', placementStep(base), 'load_more');
  eq('not in yet while a page is on its way: wait', placementStep({ ...base, loadingMore: true }), 'wait');
  eq('before the first page: wait', placementStep({ ...base, ids: [], ready: false }), 'wait');
  eq('a page that failed: open at the top', placementStep({ ...base, moreFailed: true }), 'give_up');
  eq('asked at these very rows and none came: open at the top', placementStep({ ...base, askedAtThese: true }), 'give_up');
  eq('no post named: open at the top at once', placementStep({ ...base, startId: '', ready: false }), 'give_up');

  // The protocol of PostsFeed run against a stand-in for useDiary: loadMore asks for nothing when
  // there is no next page, so the only sign of the end is the rows staying the very same array.
  function run(pages: string[][], startId: string, failAt = -1) {
    let next = 1;
    const diary = { rows: pages[0].slice(), ready: true, loadingMore: false, moreFailed: false };
    let askedAt = null;
    let asked = 0;
    for (let turn = 0; turn < 100; turn += 1) {
      const step = placementStep({
        startId,
        ids: diary.rows,
        ready: diary.ready,
        loadingMore: diary.loadingMore,
        moreFailed: diary.moreFailed,
        askedAtThese: askedAt === diary.rows,
      });
      if (step === 'place' || step === 'give_up') return { step, asked, rows: diary.rows.length };
      if (step === 'wait') {
        // The page on its way answers: a new array, as useDiary's put makes.
        if (next === failAt) diary.moreFailed = true;
        else diary.rows = [...diary.rows, ...pages[next]];
        next += 1;
        diary.loadingMore = false;
        continue;
      }
      askedAt = diary.rows;
      asked += 1;
      if (next < pages.length) diary.loadingMore = true;
    }
    return { step: 'stuck', asked, rows: diary.rows.length };
  }
  const page = (from: number) => Array.from({ length: 20 }, (_, i) => `p${from + i}`);
  const three = [page(1), page(21), page(41)];
  const found = run(three, 'p45');
  check('a post on the third page is found after two more pages', found.step === 'place' && found.asked === 2 && found.rows === 60, JSON.stringify(found));
  const first = run(three, 'p3');
  check('a post on the first page asks for nothing', first.step === 'place' && first.asked === 0, JSON.stringify(first));
  const missing = run(three, 'p99');
  check('a post on no page opens at the top after the last page', missing.step === 'give_up' && missing.asked === 3 && missing.rows === 60, JSON.stringify(missing));
  const failed = run(three, 'p45', 2);
  check('a page that fails on the way opens at the top', failed.step === 'give_up' && failed.asked === 2, JSON.stringify(failed));

  const feed = source('components/profile/PostsFeed.tsx');
  check('PostsFeed decides with placementStep', /placementStep\(\{[\s\S]*?askedAtThese: askedAt === diary\.rows,\s*\}\)/.test(feed));
  check(
    'PostsFeed asks for the next page with the rows it has, and opens at the top on give_up',
    /if \(step === 'give_up'\) setPlaced\(true\);\s*if \(step !== 'load_more'\) return;\s*setAskedAt\(diary\.rows\);\s*diary\.loadMore\(\);/.test(feed)
  );
  check(
    'the list waits under the loading line while the post is looked for',
    /diary\.status === 'loading' \|\| \(diary\.status === 'ready' && searching\) \?/.test(feed)
  );
  check('a drag counts as placed, so a late page never jumps the list', /onScrollBeginDrag=\{\(\) => setPlaced\(true\)\}/.test(feed));
}

// ---- the two Settings pages when the first read fails --------------------------------------------
{
  const blocked = source('components/settings/BlockedAccountsScreen.tsx');
  const blockedLoad = blocked.match(/const load = useCallback\(async \(\) => \{[\s\S]*?\}, \[markLoaded\]\);/)?.[0] ?? '';
  check(
    'Blocked accounts: a failed read is no longer an empty list',
    blockedLoad !== '' && !/\?\? \[\]/.test(blockedLoad) && /setFailure\(err instanceof ApiError && err\.code === 'network_unavailable' \? 'network' : 'generic'\);/.test(blockedLoad)
  );
  check(
    'Blocked accounts: a failed first read shows the plain line with Try again',
    /items === null \? \(\s*failure === null \? null : \(\s*<View[^>]*>\s*<StatusMessage kind=\{failure\} onRetry=\{\(\) => void load\(\)\} \/>/.test(blocked)
  );
  check('Blocked accounts: an answer clears the failure', /setItems\(next\);\s*setFailure\(null\);/.test(blocked));
  check('Blocked accounts: no comment calls the page unbuilt', !/may 404|comes with the\s+\*?\s*person pages|red line|text link/.test(blocked));

  const settings = source('components/settings/NotificationSettingsScreen.tsx');
  check('Notifications: the server switches show only once read', /\{status === 'ready' \? \(\s*ROWS\.map\(/.test(settings));
  check(
    'Notifications: a failed read shows the plain line with Try again, the loading line before that',
    /status === 'loading' \? \([\s\S]*?\{LOADING_LINE\}[\s\S]*?\) : \(\s*<StatusMessage kind=\{failure\} onRetry=\{\(\) => void load\(\)\} \/>/.test(settings) &&
      /const LOADING_LINE = 'Loading your settings\.';/.test(settings)
  );
  check('Notifications: the reminder switch is always there', /\)\}\s*<ReminderRow \/>/.test(settings));
  check('Notifications: no comment calls the page unbuilt', !/may 404|keep showing ON|red line/.test(settings));
}

// ---- a profile photo that would not load ---------------------------------------------------------
{
  const avatar = source('components/brand/DialAvatar.tsx');
  check('the second try comes a short while later', /const RETRY_AFTER_MS = 10 \* 1000;/.test(avatar));
  check(
    'one quiet second try per link, then none',
    /retriedRef\.current === failedUri\s*\? null\s*: setTimeout\(\(\) => \{\s*retriedRef\.current = failedUri;\s*setFailedUri\(null\);\s*\}, RETRY_AFTER_MS\);/.test(avatar)
  );
  check(
    'a return from the background tries again (not a system sheet)',
    /if \(next === 'background'\) wasBackgrounded = true;\s*if \(next === 'active' && wasBackgrounded\) setFailedUri\(null\);/.test(avatar)
  );
  check(
    'the timer and the listener run only while a link has failed, and stop with it',
    /if \(failedUri === null\) return;/.test(avatar) &&
      /if \(timer !== null\) clearTimeout\(timer\);\s*subscription\.remove\(\);/.test(avatar) &&
      /\}, \[failedUri\]\);/.test(avatar)
  );
  check('the photo still hides only for the link that failed', /const photo = uri && uri !== failedUri \? uri : null;/.test(avatar));
}

// ---- your own entry deleted from another phone while you comment ----------------------------------
{
  const view = source('components/entry/EntryView.tsx');
  check(
    'the entry page reads the entry again once its post turns gone after it was shown',
    /if \(socialStatus === 'ready'\) socialShownRef\.current = true;/.test(view) &&
      /else if \(socialStatus === 'gone' && socialShownRef\.current\) \{\s*socialShownRef\.current = false;\s*reloadEntry\(\);/.test(view)
  );
  check('the entry page still has its own gone line', view.includes("This entry isn't here any more."));
}

// ---- your own entry page when its comments can't be read (2026-11-16) ---------------------------
{
  const view = source('components/entry/EntryView.tsx');
  check('the comments failure line', /const COMMENTS_FAILED_LINE = "Couldn't load comments\.";/.test(view));
  check(
    'a failed read shows the line and Try again, and keeps them while Try again reads again',
    /if \(social\.status === 'offline' \|\| \(failed && social\.status === 'loading'\)\) \{/.test(view) &&
      /\{COMMENTS_FAILED_LINE\}/.test(view)
  );
  check(
    'Try again reads the post again and is busy meanwhile (the app button, not underlined text)',
    /<PrimaryButton label="Try again" onPress=\{social\.reload\} loading=\{social\.status === 'loading'\} \/>/.test(view) &&
      !/textDecorationLine/.test(view)
  );
  check(
    'the failed flag is set by a failed read and cleared by an answer or a gone post',
    /if \(socialStatus === 'offline'\) setCommentsFailed\(true\);\s*else if \(socialStatus === 'ready' \|\| socialStatus === 'gone'\) setCommentsFailed\(false\);/.test(view) &&
      /failed=\{commentsFailed\}/.test(view)
  );
  check(
    'the comment bar and the comment mark still wait for an answer',
    /\{social\.status === 'ready' && social\.post !== null \? \(\s*social\.post\.can_comment \? \(\s*<CommentBar/.test(view) &&
      /onComment=\{social\.status === 'ready' && social\.post\?\.can_comment \?/.test(view)
  );
  check('the line is plain ink, centred', /color: color\.ink,\s*textAlign: 'center',\s*\}\}\s*>\s*\{COMMENTS_FAILED_LINE\}/.test(view));
}

// ---- photo access turned off, at the comment bar's photo button (2026-11-16) ---------------------
{
  const comments = source('components/board/Comments.tsx');
  check(
    'camera or photo access turned off opens the alert with Open Settings',
    /if \(!permission\.granted\) \{(?:\s*\/\/[^\n]*)*\s*showAccessOff\(source\);\s*return;\s*\}/.test(comments) &&
      /import \{ showAccessOff \} from '@\/lib\/media\/showAccessOff';/.test(comments)
  );
  check('the old access lines are gone from the comment bar', !/access is needed/.test(comments));
}

// ---- the comment box: the emoji row and the reply's @username (CMT-01, CMT-03) -------------------
{
  // The server counts characters (code points); the box counts as its maxLength does (UTF-16 units).
  const characters = (s: string) => [...s].length;
  const fits = (s: string) => s.length <= COMMENT_MAX && characters(s) <= 140 && s.isWellFormed();
  const fire = '🔥';
  eq('the box and the server have the same limit', COMMENT_MAX, 140);
  check('the server limit is 140 characters', /MaxCommentRunes = 140\b/.test(source('internal/board/model.go')));
  eq('an emoji that fits is added at the end', withEmoji('Nice', fire, COMMENT_MAX), `Nice${fire}`);
  eq('an emoji that ends at 140 exactly is added', withEmoji('a'.repeat(138), fire, COMMENT_MAX), `${'a'.repeat(138)}${fire}`);
  eq('139 units and a 2-unit emoji: the text is unchanged', withEmoji('a'.repeat(139), fire, COMMENT_MAX), 'a'.repeat(139));
  eq('a full box is unchanged', withEmoji('a'.repeat(140), fire, COMMENT_MAX), 'a'.repeat(140));
  for (const emoji of COMMENT_EMOJIS) {
    let text = '';
    for (let i = 0; i < 200; i += 1) text = withEmoji(text, emoji.char, COMMENT_MAX);
    check(`tapping ${emoji.label} again and again stops inside the limit`, fits(text) && text.length > COMMENT_MAX - emoji.char.length, `${text.length} units`);
  }

  eq('a reply starts with the @username and a space', withReplyPrefix('', 'maya', COMMENT_MAX), '@maya ');
  eq('what was typed stays after it', withReplyPrefix('so good', 'maya', COMMENT_MAX), '@maya so good');
  eq('a mention already there is replaced', withReplyPrefix('@sam  so good', 'maya', COMMENT_MAX), '@maya so good');
  eq('a long text is cut to the limit', withReplyPrefix('a'.repeat(140), 'maya', COMMENT_MAX), `@maya ${'a'.repeat(134)}`);
  // 139 units ending in emoji: the old cut kept half of the 67th emoji, which reaches the server as
  // U+FFFD and is refused as invalid text.
  const typed = `a${fire.repeat(69)}`;
  check('the case is real: a plain cut keeps half an emoji', /[\uD800-\uDBFF]$/.test(`@maya ${typed}`.slice(0, COMMENT_MAX)));
  eq('the reply drops the half emoji', withReplyPrefix(typed, 'maya', COMMENT_MAX), `@maya a${fire.repeat(66)}`);
  for (const username of ['abc', 'maya', 'maya_b', 'twenty_characters_xy']) {
    for (const lead of ['', 'a', 'ab']) {
      for (const emoji of COMMENT_EMOJIS) {
        // A box filled to the brim with whole emoji.
        const before = lead + emoji.char.repeat(Math.floor((COMMENT_MAX - lead.length) / emoji.char.length));
        const got = withReplyPrefix(before, username, COMMENT_MAX);
        check(`a reply by @${username} after ${lead.length} letters and ${emoji.label}`, fits(got) && got.startsWith(`@${username} `), `${got.length} units`);
      }
    }
  }

  const comments = source('components/board/Comments.tsx');
  check('the emoji row adds through withEmoji', /setText\(\(before\) => withEmoji\(before, char, COMMENT_MAX\)\);/.test(comments));
  check('Reply fills the box through withReplyPrefix', /setText\(\(before\) => withReplyPrefix\(before, username, COMMENT_MAX\)\)/.test(comments));
  check('the box allows COMMENT_MAX', /maxLength=\{COMMENT_MAX\}/.test(comments));
}

// ---- the seven report reasons: the same keys, order and words everywhere (SAFE-01) ---------------
{
  const prd = (source('PRD.md').match(/\*\*SAFE-01 reasons\.\*\* ([^\n]*)\./)?.[1] ?? '').split('; ');
  const keys = ['spam', 'nudity', 'harassment', 'hate', 'violence', 'impersonation', 'other'];
  const appList = source('lib/api/person.ts').match(/export const REPORT_REASONS = \[([\s\S]*?)\] as const;/)?.[1] ?? '';
  const app = [...appList.matchAll(/\{ reason: '([a-z]+)', label: '([^']+)' \}/g)].map((m) => `${m[1]}=${m[2]}`);
  const serverList = source('internal/social/person.go').match(/var ReportReasons = \[\]string\{([^}]*)\}/)?.[1] ?? '';
  const server = [...serverList.matchAll(/"([a-z]+)"/g)].map((m) => m[1]);
  const model = source('internal/admin/model.go');
  const wordsList = model.match(/var ReasonWords = map\[string\]string\{([\s\S]*?)\n\}/)?.[1] ?? '';
  const words = Object.fromEntries([...wordsList.matchAll(/"([a-z]+)":\s*"([^"]+)"/g)].map((m) => [m[1], m[2]]));
  const orderList = model.match(/var reasonOrder = map\[string\]int\{([\s\S]*?)\n\}/)?.[1] ?? '';
  const ranks = Object.fromEntries([...orderList.matchAll(/"([a-z]+)": (\d+)/g)].map((m) => [m[1], Number(m[2])]));

  eq('the PRD lists seven reasons', prd.length, 7);
  eq('the app: the PRD reasons, in its order, with its words', app.join(' | '), keys.map((k, i) => `${k}=${prd[i]}`).join(' | '));
  eq('the server takes the same keys in the same order', server.join(','), keys.join(','));
  eq('the admin page and the email: the same words', keys.map((k) => words[k]).join(' | '), prd.join(' | '));
  eq('the admin page and the email: no other reason', Object.keys(words).length, keys.length);
  eq('the admin order ranks each reason once', keys.map((k) => ranks[k]).sort().join(','), '0,1,2,3,4,5,6');
  eq('the admin order: violence and nudity, then bullying and hate', ['violence', 'nudity', 'harassment', 'hate'].map((k) => ranks[k]).join(','), '0,1,2,3');
  // The database refuses any other reason: the three tables check the same seven.
  const tables = [...['0010_mutes_user_reports', '0012_reports_disabled'].map((m) => source(`migrations/${m}.up.sql`)).join('\n').matchAll(/reason in\s*\(([^)]*)\)/g)];
  eq('three report tables check the reason', tables.length, 3);
  for (const [i, m] of tables.entries()) {
    eq(`report table ${i + 1} takes the same seven`, [...m[1].matchAll(/'([a-z]+)'/g)].map((k) => k[1]).join(','), keys.join(','));
  }
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
