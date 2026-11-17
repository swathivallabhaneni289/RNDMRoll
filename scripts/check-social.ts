// @ts-nocheck
/**
 * Throwaway checks for the social screens' fixes: a comment on a post that went away, a follow
 * request that went away, an invite link that leads to no one, where your posts list opens, the
 * two Settings pages when the first read fails, a profile photo that would not load, the comment
 * box's emoji row and reply prefix, the report reasons on the phone, the server, the admin page
 * and the database, and the moderation build's phone side (2026-11-17): "Report sent." and the three
 * report flows, a reported comment leaving the list, the under review line, the Spin page's removed
 * line, the bell's four decision kinds and the email that asks for a second look.
 * Not part of the app bundle. Run with:  node --experimental-strip-types scripts/check-social.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 * The pure rules are run; the hooks and screens have no React here, so the lines that matter are
 * pinned as written.
 */
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { POST_GONE_LINE, POST_REMOVED_LINE, REPORT_SENT, UNDER_REVIEW, commentFailureLine, reportOutcome } from '../lib/board/text.ts';
import { placementStep } from '../lib/board/placement.ts';
import { afterReplyEnds, countAfterComment, placeComment, withEmoji, withReplyPrefix, withoutComment } from '../lib/board/comments.ts';
import { COMMENT_EMOJIS } from '../lib/entry/reactions.ts';
import { COMMENT_MAX } from '../lib/entry/rules.ts';
import {
  DECISION_KINDS,
  SECOND_LOOK_HINT,
  decisionLine,
  isDecisionKind,
  secondLookEmail,
  secondLookFallback,
  secondLookMailto,
} from '../lib/notifications/moderation.ts';
import { REPORT_NOTICE_MS, getReportNotice, setReportNotice, subscribeReportNotice } from '../lib/report/notice.ts';

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

  // Reply mode ended (the close mark, or the comment being answered left the page): a lone mention
  // goes, so one tap on Post never sends just '@maya' as a new comment.
  eq('reply mode ended from outside takes away a lone mention', afterReplyEnds('@maya ', 'maya'), '');
  eq('a lone mention with no space goes too', afterReplyEnds('@maya', 'maya'), '');
  eq('the mention Reply wrote goes whole', afterReplyEnds(withReplyPrefix('', 'twenty_characters_xy', COMMENT_MAX), 'twenty_characters_xy'), '');
  eq('typed words stay', afterReplyEnds('@maya hi', 'maya'), '@maya hi');
  eq('a mention of someone else stays', afterReplyEnds('@sam ', 'maya'), '@sam ');
  eq('no username: nothing was added, so nothing is taken', afterReplyEnds('@', ''), '@');
  eq('an empty box stays empty', afterReplyEnds('', 'maya'), '');

  const comments = source('components/board/Comments.tsx');
  check('the emoji row adds through withEmoji', /setText\(\(before\) => withEmoji\(before, char, COMMENT_MAX\)\);/.test(comments));
  check('Reply fills the box through withReplyPrefix', /setText\(\(before\) => withReplyPrefix\(before, username, COMMENT_MAX\)\)/.test(comments));
  // Reply mode also ends from outside the bar: the comment being answered was reported or deleted,
  // or the reply came back not found. The lone mention goes then too, as at the close mark.
  check(
    'the box takes a lone mention away whenever reply mode ends, not only at the close mark',
    /if \(replyTo === null\) \{[\s\S]{0,300}?setText\(\(before\) => afterReplyEnds\(before, [a-z]+\)\);[\s\S]{0,60}?return;\s*\}/.test(comments)
  );
  check('the close mark keeps the same rule', /function cancelReply\(\) \{[\s\S]{0,200}?afterReplyEnds\(before, replyTo\.author\.username\)/.test(comments));
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

// ---- moderation: the words, word for word (2026-11-17) ---------------------------------------------
{
  eq('after any report', REPORT_SENT, 'Report sent.');
  eq('under your own post or comment while 3 or more reports wait', UNDER_REVIEW, 'Hidden while we take a look.');
  eq('the Spin page for a post an admin removed', POST_REMOVED_LINE, 'We removed this post. The day still counts.');
  eq('the bell: your post removed', decisionLine('post_removed'), 'We removed your post. It broke our rules.');
  eq('the bell: your comment removed', decisionLine('comment_removed'), 'We removed your comment. It broke our rules.');
  eq('the bell: your post back', decisionLine('post_back'), 'Sorry, we got that wrong. Your post is back.');
  eq('the bell: your comment back', decisionLine('comment_back'), 'Sorry, we got that wrong. Your comment is back.');
  eq('the hint on a removed item that opens the email', SECOND_LOOK_HINT, 'Opens an email to ask for a second look.');
  eq('no mail app: the line names the address', secondLookFallback(' help@example.com '), 'Email help@example.com to ask for a second look.');

  eq('the four decision kinds, as the server names them', DECISION_KINDS.join(','), 'post_removed,comment_removed,post_back,comment_back');
  check('each of them is a decision kind', DECISION_KINDS.every((kind) => isDecisionKind(kind)));
  check(
    'no other kind is one',
    !['comment', 'reply', 'follow', 'follow_request', 'follow_accepted', 'spin_open', 'bonus_spin', 'removed', ''].some((kind) => isDecisionKind(kind))
  );

  const look = { kind: 'post_removed', contact: 'help@example.com', username: 'maya', text: 'Aftersun', ref: '1a2b3c4d' };
  const post = secondLookEmail(look);
  eq('the subject for a post', post.subject, 'Second look at my post');
  eq('the body for a post', post.lines.join('\n'), 'Username: @maya\nPost: Aftersun\nRef: 1a2b3c4d\n\nWhy it should come back:');
  const said = secondLookEmail({ ...look, kind: 'comment_removed', text: 'so good' });
  eq('the subject for a comment', said.subject, 'Second look at my comment');
  eq('the body for a comment', said.lines.join('\n'), 'Username: @maya\nComment: so good\nRef: 1a2b3c4d\n\nWhy it should come back:');

  // Built from escapes so this file holds no dash of its own: the hyphen, U+2010 to U+2015, the minus.
  const DASH = /[-‐-―−]/;
  const sentences = [REPORT_SENT, UNDER_REVIEW, POST_REMOVED_LINE, SECOND_LOOK_HINT, secondLookFallback('help@example.com'), ...DECISION_KINDS.map(decisionLine)];
  for (const line of [...sentences, post.subject, ...post.lines, said.subject, ...said.lines]) {
    check('no dash of any kind in the new words', !DASH.test(line), line);
  }
  for (const line of sentences) {
    check('a short plain sentence: a capital, a full stop, no exclamation mark', /^[A-Z][^!]*\.$/.test(line) && line.length <= 50, line);
  }
}

// ---- moderation: the email link, how a report ends, and a comment leaving the list ----------------
{
  const look = { kind: 'post_removed', contact: 'help@example.com', username: 'maya', text: 'Aftersun', ref: '1a2b3c4d' };
  const fields = (link: string) =>
    Object.fromEntries(
      link
        .slice(link.indexOf('?') + 1)
        .split('&')
        .map((pair) => [pair.slice(0, pair.indexOf('=')), decodeURIComponent(pair.slice(pair.indexOf('=') + 1))])
    );
  const link = secondLookMailto(look) ?? '';
  check('the link writes to the address', link.startsWith('mailto:help@example.com?'), link);
  eq('the link carries the subject', fields(link).subject, 'Second look at my post');
  eq('the link carries the body, its lines ending in CRLF', fields(link).body, secondLookEmail(look).lines.join('\r\n'));
  const odd = secondLookMailto({ ...look, text: 'Fish & chips? #1 = best\nreally' }) ?? '';
  check(
    'a title with ? & # = and a line break stays inside the body',
    (odd.match(/\?/g) ?? []).length === 1 && (odd.match(/&/g) ?? []).length === 1 && !/[#\s]/.test(odd),
    odd
  );
  eq('and comes back word for word', fields(odd).body.split('\r\n')[1], 'Post: Fish & chips? #1 = best\nreally');
  check('an address with spaces round it is trimmed', (secondLookMailto({ ...look, contact: '  help@example.com ' }) ?? '').startsWith('mailto:help@example.com?'));
  for (const contact of ['', '   ', 'help', 'help@', '@example.com', 'a@b@c', 'help @example.com', 'help@example.com?cc=x', 'help@example.com#x']) {
    eq(`no usable address, no link (the tap does nothing): ${JSON.stringify(contact)}`, secondLookMailto({ ...look, contact }), null);
  }

  // How a report ended: a 404 counts as sent; no connection and too many tries keep the client's words.
  eq('a report answered 404 not_found is done', reportOutcome(404, 'not_found'), 'done');
  eq('a 404 whose body did not read is done too', reportOutcome(404, 'server_error'), 'done');
  eq('too many tries: the client words', reportOutcome(429, 'rate_limited'), 'client');
  eq('a 429 with any code: the client words', reportOutcome(429, 'server_error'), 'client');
  eq('no connection: the client words', reportOutcome(0, 'network_unavailable'), 'client');
  eq('a server failure: the plain line', reportOutcome(500, 'server_error'), 'generic');
  eq('a refused reason: the plain line', reportOutcome(400, 'validation_failed'), 'generic');

  const a = { id: 'a', parent_id: null };
  const a1 = { id: 'a1', parent_id: 'a' };
  const a2 = { id: 'a2', parent_id: 'a' };
  const b = { id: 'b', parent_id: null };
  const all = [a, a1, a2, b];
  const ids = (list: { id: string }[]) => list.map((item) => item.id).join(',');
  const top = withoutComment(all, 4, 'a', null);
  check('a top-level comment leaves with its replies, and the count with them', ids(top.list) === 'b' && top.count === 1, JSON.stringify(top));
  const reply = withoutComment(all, 4, 'a1', null);
  check('a reply leaves alone', ids(reply.list) === 'a,a2,b' && reply.count === 3, JSON.stringify(reply));
  const none = withoutComment(all, 4, 'x', 'a1');
  check('an id not in the list changes nothing', ids(none.list) === 'a,a1,a2,b' && none.count === 4 && !none.endReply, JSON.stringify(none));
  eq('the count never goes below 0', withoutComment(all, 1, 'a', null).count, 0);
  eq('with no count from the server, the list counts', withoutComment(all, null, 'b', null).count, 3);
  check('reply mode ends when the reply being answered left with its thread', withoutComment(all, 4, 'a', 'a2').endReply);
  check('reply mode ends when the comment being answered left', withoutComment(all, 4, 'b', 'b').endReply);
  check('reply mode stays when another thread left', !withoutComment(all, 4, 'b', 'a1').endReply);
  check('no reply mode: nothing to end', !withoutComment(all, 4, 'a', null).endReply);
}

// ---- moderation: the screens, pinned as written ---------------------------------------------------
{
  const tree = (dir: string): string[] =>
    (readdirSync(fileURLToPath(new URL(`../${dir}`, import.meta.url)), { recursive: true }) as string[])
      .filter((rel) => /\.tsx?$/.test(rel))
      .map((rel) => `${dir}/${rel}`);
  const stale = [...tree('app'), ...tree('components'), ...tree('lib')].filter((rel) => /REPORT_THANKS|Thanks for letting us know/.test(source(rel)));
  check('the old thanks line is gone from the app', stale.length === 0, stale.join(', '));

  const reports = source('lib/api/reports.ts');
  check(
    'every report flow reads its ending through reportOutcome',
    /const outcome = reportOutcome\(err\.status, err\.code\);\s*if \(outcome === 'done'\) return null;\s*return outcome === 'client' \? err\.userMessage : GENERIC;/.test(reports)
  );
  const postView = source('components/board/PostView.tsx');
  check(
    'the post page: one report at a time, then "Report sent." and back while it is still the page showing',
    /if \(reportingRef\.current\) return;\s*reportingRef\.current = true;/.test(postView) &&
      /failure = reportFailureLine\(err\);/.test(postView) &&
      /setReportNotice\(REPORT_SENT\);\s*if \(focusedRef\.current\) goBack\(\);/.test(postView)
  );
  check('the post page hands a reported comment to its list', /onReported=\{load\.drop\}/.test(postView) && /onReported=\{onReported\}/.test(postView));
  check('your own entry page does too', /onReported=\{social\.drop\}/.test(source('components/entry/EntryView.tsx')));
  const person = source('components/people/PersonScreen.tsx');
  check(
    'the person page: one action at a time, then "Report sent." and back, with no read first',
    /async function sendReport\(reason: ReportReason\) \{\s*if \(busyRef\.current\) return;/.test(person) &&
      /failure = reportFailureLine\(err\);/.test(person) &&
      /setReportNotice\(REPORT_SENT\);\s*if \(focusedRef\.current\) goBack\(\);/.test(person) &&
      /afterClose\(\(\) => void sendReport\(item\.reason\)\)/.test(person)
  );
  const list = source('components/board/Comments.tsx');
  check(
    'a comment report: one at a time, then it leaves the list and "Report sent." shows, and the page stays',
    /if \(reportingRef\.current\) return;\s*reportingRef\.current = true;/.test(list) &&
      /failure = reportFailureLine\(err\);/.test(list) &&
      /onReported\?\.\(comment\.id\);\s*setReportNotice\(REPORT_SENT\);/.test(list)
  );
  check(
    'the writer sees the review line under their comment, and a screen reader hears it',
    /\{comment\.under_review \? \(/.test(list) &&
      /<AppText role="label">\{UNDER_REVIEW\}<\/AppText>/.test(list) &&
      /\$\{comment\.under_review \? `\. \$\{UNDER_REVIEW\}` : ''\}`\}/.test(list)
  );
  const hook = source('components/board/usePost.ts');
  check(
    'a reported comment leaves the list with no request, the same way a deleted one does',
    /const after = withoutComment\(now\.comments \?\? \[\], now\.card\.comment_count, commentId, replyToRef\.current\?\.id \?\? null\);\s*if \(after\.endReply\) chooseReply\(null\);/.test(hook) &&
      /comment_count: after\.count/.test(hook) &&
      /await deleteComment\(commentId\);\s*drop\(commentId\);/.test(hook)
  );
  // A read already on its way (a photo reload, a return from the background) may have begun before
  // the report or the delete, and would answer with the comment: drop moves the read count on, so
  // that answer is dropped.
  check(
    'a read that began before a comment left cannot bring it back',
    /const drop = useCallback\(\s*\(commentId: string\) => \{[\s\S]{0,400}?\+\+seqRef\.current;[\s\S]{0,400}?put\(\{ \.\.\.now, comments: after\.list/.test(hook) &&
      /const next = await getPost\(id\);\s*if \(!mountedRef\.current \|\| seq !== seqRef\.current\) return;\s*put\(next\);/.test(hook)
  );

  const layout = source('app/(app)/_layout.tsx');
  const overlay = layout.slice(layout.indexOf('function ReportNotice('), layout.indexOf('export default function AppLayout('));
  check(
    'one overlay over every page: under the Back row, taps go through, on its own panel, told to a screen reader',
    /useReportNotice\(\)/.test(overlay) &&
      /top: insets\.top \+ minTouchTarget \+ space\.sm/.test(overlay) &&
      /pointerEvents="none"/.test(overlay) &&
      /backgroundColor: color\.dominant/.test(overlay) &&
      /announceForAccessibility/.test(overlay) &&
      /<ReportNotice \/>/.test(layout)
  );
  check('the overlay neither fades nor moves', overlay !== '' && !/Animated|FadeIn|FadeOut|withTiming|opacity/.test(overlay));

  for (const [rel, shown] of [
    ['components/entry/EntryView.tsx', /\{entry\.under_review === true \? \(/],
    ['components/profile/PostsFeed.tsx', /\{entry\.under_review === true \? \(/],
    ['components/today/SettledBlock.tsx', /\{entry\.under_review === true \? \(/],
    ['components/board/PostCard.tsx', /\{card\.under_review \? \(/],
  ] as const) {
    const text = source(rel);
    check(`${rel}: the review line, only on the server's word`, shown.test(text) && /<AppText role="label">\{UNDER_REVIEW\}<\/AppText>/.test(text));
  }
  check('your own card tells a screen reader too', /\$\{card\.under_review \? ` \$\{UNDER_REVIEW\}` : ''\}`\}/.test(source('components/board/PostCard.tsx')));
  eq('a card and a comment read under_review only as true', (source('lib/api/board.ts').match(/under_review: raw\.under_review === true,/g) ?? []).length, 2);
  const settled = source('components/today/SettledBlock.tsx');
  check(
    "the Spin page: a post an admin removed says so, not 'deleted'",
    /\{removed \? POST_REMOVED_LINE : 'This entry was deleted\. The day still counts\.'\}/.test(settled) &&
      /removed=\{today\.entry_removed === true\}/.test(settled)
  );

  const api = source('lib/api/notifications.ts');
  check('the bell keeps the four decision kinds', /'bonus_spin',\s*\.\.\.DECISION_KINDS,\s*\];/.test(api));
  check(
    'contact and ref are read as strings, empty when missing, and no key is named email',
    /contact: typeof raw\.contact === 'string' \? raw\.contact : '',/.test(api) &&
      /ref: typeof raw\.ref === 'string' \? raw\.ref : '',/.test(api) &&
      !/email/.test(api)
  );
  const bell = source('components/today/NotificationsScreen.tsx');
  check(
    'the four kinds say their sentence and draw the wheel mark, not an empty face',
    /case 'comment_back': \{[\s\S]{0,200}const line = decisionLine\(item\.kind\);/.test(bell) &&
      /if \(item\.kind === 'spin_open' \|\| isDecisionKind\(item\.kind\)\) \{/.test(bell)
  );
  check(
    'a post back opens your entry page; a comment back opens the post it is on',
    /if \(item\.kind === 'comment' \|\| item\.kind === 'post_back'\) \{\s*if \(item\.entry_id === null\) return;\s*openEntry\(item\.entry_id\);/.test(bell) &&
      /if \(item\.kind === 'reply' \|\| item\.kind === 'comment_back'\) \{\s*if \(item\.entry_id === null\) return;\s*void openReply\(item\.entry_id\);/.test(bell)
  );
  check(
    'a removed item opens the email only with an address, and names the address when no mail app opens',
    /const link = secondLookLink\(item\);\s*if \(link !== null\) askSecondLook\(item, link\);/.test(bell) &&
      /Linking\.openURL\(link\)\.catch\(\(\) => \{\s*setMailFailedId\(item\.id\);/.test(bell) &&
      /\{secondLookFallback\(item\.contact\)\}/.test(bell)
  );
  check('the hint only where the email opens', /accessibilityHint=\{secondLookLink\(item\) !== null \? SECOND_LOOK_HINT : undefined\}/.test(bell));
  check(
    'a post back shows its picture; a removed item never does',
    /\(item\.kind === 'comment' \|\| item\.kind === 'reply' \|\| item\.kind === 'post_back'\) && item\.photo_url !== ''/.test(bell)
  );
  check('no phone push for the decisions', !/post_removed|comment_removed|post_back|comment_back/.test(source('lib/notifications/pushTarget.ts')));

  // Nothing red: only Log out is.
  for (const rel of [
    'app/(app)/_layout.tsx',
    'components/board/Comments.tsx',
    'components/board/PostView.tsx',
    'components/board/PostCard.tsx',
    'components/people/PersonScreen.tsx',
    'components/entry/EntryView.tsx',
    'components/profile/PostsFeed.tsx',
    'components/today/SettledBlock.tsx',
    'components/today/NotificationsScreen.tsx',
  ]) {
    check(`${rel}: nothing red`, !/destructive/.test(source(rel)));
  }
}

// ---- moderation: the "Report sent." store (set, read, clear by its timer, restart, clear on '') ----
{
  const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
  let calls = 0;
  const unsubscribe = subscribeReportNotice(() => {
    calls += 1;
  });
  eq('the report notice starts empty', getReportNotice(), '');
  eq('it shows for about 2.5 seconds', REPORT_NOTICE_MS, 2500);
  setReportNotice(REPORT_SENT);
  eq('it holds the line once set', getReportNotice(), REPORT_SENT);
  await sleep(1500);
  setReportNotice(REPORT_SENT);
  await sleep(1500);
  eq('a second report restarted the timer (3 s after the first, 1.5 s after the second)', getReportNotice(), REPORT_SENT);
  await sleep(1200);
  eq('it clears itself after its time', getReportNotice(), '');
  eq('it told its listener on each set and on the clear', calls, 3);
  setReportNotice(REPORT_SENT);
  setReportNotice('');
  eq('an empty text clears it at once', getReportNotice(), '');
  unsubscribe();
  const before = calls;
  setReportNotice(REPORT_SENT);
  setReportNotice('');
  eq('an unsubscribed listener is not told', calls, before);
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
