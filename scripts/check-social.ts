// @ts-nocheck
/**
 * Throwaway checks for the social screens' fixes: a comment on a post that went away, a follow
 * request that went away, an invite link that leads to no one, where your posts list opens, the
 * two Settings pages when the first read fails, and a profile photo that would not load. Not part
 * of the app bundle. Run with:  node --experimental-strip-types scripts/check-social.ts
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
import { countAfterComment, placeComment } from '../lib/board/comments.ts';

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

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
