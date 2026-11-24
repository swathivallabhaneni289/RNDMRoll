// @ts-nocheck
/**
 * Throwaway checks for the social screens' fixes: a comment on a post that went away, a follow
 * request that went away, an invite link that leads to no one, where your posts list opens, the
 * two Settings pages when the first read fails, a profile photo that would not load, the comment
 * box's emoji row and reply prefix, the report reasons on the phone, the server, the admin page
 * and the database, and the moderation build's phone side (2026-11-17): "Report sent." and the three
 * report flows, a reported comment leaving the list, the under review line, the Spin page's removed
 * line, the bell's decision kinds (with the two hidden ones of 2026-11-18, and the reason a removal
 * names, also 2026-11-18), and the second look a removal opens in the app (2026-11-19): who can ask,
 * its words, its page, its refusals, and a send that ends after the person left the page. Also
 * likes (2026-11-24, BRD-06): the heart's words, how a like is read and sent, the heart under each
 * post, the Likes sheet, and the bell's like line.
 * Not part of the app bundle. Run with:  node --experimental-strip-types scripts/check-social.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 * The pure rules are run; the hooks and screens have no React here, so the lines that matter are
 * pinned as written.
 */
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  LIKED_YOUR_POST,
  LIKES_CLOSE,
  LIKES_HINT,
  LIKES_LOADING,
  LIKES_TITLE,
  LIKE_FAILED,
  LIKE_LABEL,
  NO_LIKES,
  POST_GONE_LINE,
  POST_REMOVED_LINE,
  REPORT_SENT,
  UNDER_REVIEW,
  commentFailureLine,
  likeCount,
  likesSpoken,
  reportOutcome,
} from '../lib/board/text.ts';
import { placementStep } from '../lib/board/placement.ts';
import { afterReplyEnds, countAfterComment, placeComment, withEmoji, withReplyPrefix, withoutComment } from '../lib/board/comments.ts';
import { COMMENT_EMOJIS } from '../lib/entry/reactions.ts';
import { COMMENT_MAX } from '../lib/entry/rules.ts';
import {
  DECISION_KINDS,
  REASON_WORDS,
  SECOND_LOOK_ASKED,
  SECOND_LOOK_HINT,
  SECOND_LOOK_LABEL,
  SECOND_LOOK_MAX_CHARS,
  SECOND_LOOK_MAX_PHOTOS,
  SECOND_LOOK_NOT_SENT,
  SECOND_LOOK_PHOTOS,
  SECOND_LOOK_SENT,
  SECOND_LOOK_TITLE,
  canAskSecondLook,
  decisionLine,
  isDecisionKind,
  isRemovalKind,
  isSecondLookAsked,
  secondLookFailure,
  secondLookItemLine,
  secondLookStateLine,
  secondLookTextProblem,
} from '../lib/notifications/moderation.ts';
import { chooseSecondLook, chosenSecondLook, onSecondLookSent, secondLookSent } from '../lib/notifications/secondLookItem.ts';
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
    for (const what of ['comment', 'reply', 'like', 'other']) {
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
  eq('the bell: 3 reports hid your post', decisionLine('post_hidden'), 'Your post is hidden while we take a look.');
  eq('the bell: 3 reports hid your comment', decisionLine('comment_hidden'), 'Your comment is hidden while we take a look.');
  eq('the bell: your post kept removed after a second look', decisionLine('post_kept'), 'We looked again. Your post stays removed.');
  eq('the bell: your comment kept removed after a second look', decisionLine('comment_kept'), 'We looked again. Your comment stays removed.');
  eq('under a removal once its second look is asked', SECOND_LOOK_ASKED, 'Second look asked.');
  eq('the hint on a removal that opens the second look', SECOND_LOOK_HINT, 'Opens a page to ask for a second look.');
  eq("the second look's title", SECOND_LOOK_TITLE, 'Ask for a second look');
  eq("the second look's box", SECOND_LOOK_LABEL, 'Tell us why it should come back');
  eq("the second look's photos", SECOND_LOOK_PHOTOS, 'Add photos');
  eq('after a second look is sent', SECOND_LOOK_SENT, "Sent. We'll take another look.");
  eq('a second look that failed after the person left its page', SECOND_LOOK_NOT_SENT, 'Second look not sent. Please try again.');
  eq('a second look takes up to 500 characters', SECOND_LOOK_MAX_CHARS, 500);
  eq('and up to 3 photos', SECOND_LOOK_MAX_PHOTOS, 3);

  // A removal names the reason the admin gave, sent as its key (2026-11-18).
  for (const [reason, postLine, commentLine] of [
    ['spam', 'We removed your post for spam.', 'We removed your comment for spam.'],
    ['nudity', 'We removed your post for nudity or sexual content.', 'We removed your comment for nudity or sexual content.'],
    ['harassment', 'We removed your post for bullying or harassment.', 'We removed your comment for bullying or harassment.'],
    ['hate', 'We removed your post for hate speech.', 'We removed your comment for hate speech.'],
    ['violence', 'We removed your post for violence or threats.', 'We removed your comment for violence or threats.'],
    ['impersonation', 'We removed your post for pretending to be someone else.', 'We removed your comment for pretending to be someone else.'],
  ]) {
    eq(`the bell: your post removed for ${reason}`, decisionLine('post_removed', reason), postLine);
    eq(`the bell: your comment removed for ${reason}`, decisionLine('comment_removed', reason), commentLine);
  }
  // Something else, none, or a key the app doesn't know (an object's own names among them): the plain line.
  for (const reason of ['other', '', 'rude', 'Hate speech', 'HATE', 'constructor', 'toString', '__proto__']) {
    eq(`the bell: your post removed, reason ${JSON.stringify(reason)}`, decisionLine('post_removed', reason), 'We removed your post. It broke our rules.');
    eq(`the bell: your comment removed, reason ${JSON.stringify(reason)}`, decisionLine('comment_removed', reason), 'We removed your comment. It broke our rules.');
  }
  for (const kind of ['post_back', 'comment_back', 'post_kept', 'comment_kept', 'post_hidden', 'comment_hidden']) {
    eq(`the bell: ${kind} never reads a reason`, decisionLine(kind, 'hate'), decisionLine(kind));
  }
  // The words are the report list's own (lib/api/person.ts imports the API client, so it is read as text), lowercased.
  const reportList = source('lib/api/person.ts').match(/export const REPORT_REASONS = \[([\s\S]*?)\] as const;/)?.[1] ?? '';
  const labels = Object.fromEntries([...reportList.matchAll(/\{ reason: '([a-z]+)', label: '([^']+)' \}/g)].map((m) => [m[1], m[2]]));
  eq('the report list has its seven reasons', Object.keys(labels).length, 7);
  eq(
    'a removal names every report reason but Something else',
    Object.keys(REASON_WORDS).sort().join(','),
    Object.keys(labels).filter((reason) => reason !== 'other').sort().join(',')
  );
  for (const [reason, words] of Object.entries(REASON_WORDS)) {
    eq(`a removal's words for ${reason} are the report list's, lowercased`, words, labels[reason]?.toLowerCase());
  }

  eq(
    'the eight decision kinds, as the server names them',
    DECISION_KINDS.join(','),
    'post_removed,comment_removed,post_back,comment_back,post_kept,comment_kept,post_hidden,comment_hidden'
  );
  check('each of them is a decision kind', DECISION_KINDS.every((kind) => isDecisionKind(kind)));
  check(
    'no other kind is one',
    !['comment', 'reply', 'like', 'follow', 'follow_request', 'follow_accepted', 'spin_open', 'bonus_spin', 'removed', 'hidden', 'kept', ''].some(
      (kind) => isDecisionKind(kind)
    )
  );

  // The second look's page names the item at its top.
  const post = secondLookItemLine('post_removed', 'Aftersun');
  check('the page names a post by its title', post.label === 'Your post:' && post.words === 'Aftersun', JSON.stringify(post));
  const said = secondLookItemLine('comment_removed', 'so good');
  check('a comment by its words', said.label === 'Your comment:' && said.words === 'so good', JSON.stringify(said));
  eq('a comment that was only a photo says so', secondLookItemLine('comment_removed', '').words, 'a photo');
  eq('a comment of only spaces reads as a photo too', secondLookItemLine('comment_removed', '   ').words, 'a photo');

  // Built from escapes so this file holds no dash of its own: the hyphen, U+2010 to U+2015, the minus.
  const DASH = /[-‐-―−]/;
  // Every line a second look can show: its own, the box's two at Send, and each refusal's.
  const refusals = [
    [400, 'validation_failed', 'text', 'required'],
    [400, 'validation_failed', 'text', 'too_long'],
    [400, 'validation_failed', 'text', 'invalid'],
    [400, 'validation_failed', 'photo_keys', 'invalid'],
    [404, 'not_found'],
    [409, 'second_look_asked'],
    [409, 'second_look_too_late'],
    [409, 'second_look_back'],
    [429, 'rate_limited'],
    [500, 'server_error'],
  ];
  const lookLines = [
    ...new Set(
      ['post_removed', 'comment_removed'].flatMap((kind) =>
        refusals.map(([status, code, field, reason]) => secondLookFailure(kind, status, code, field, reason).line)
      )
    ),
  ];
  eq('a refused second look has eleven lines (the gone line differs for a post and a comment)', lookLines.length, 11);
  const sentences = [
    REPORT_SENT,
    UNDER_REVIEW,
    POST_REMOVED_LINE,
    SECOND_LOOK_ASKED,
    SECOND_LOOK_HINT,
    SECOND_LOOK_SENT,
    SECOND_LOOK_NOT_SENT,
    secondLookTextProblem(''),
    secondLookTextProblem('x'.repeat(501)),
    secondLookFailure('post_removed', 500, 'photo_upload_failed', '', '', 3).line,
    ...['asked', 'kept', 'closed'].map((state) => secondLookStateLine(state)),
    ...lookLines,
    ...DECISION_KINDS.map((kind) => decisionLine(kind)),
  ];
  // Names on the page, not sentences: a capital, no full stop.
  const names = [SECOND_LOOK_TITLE, SECOND_LOOK_LABEL, SECOND_LOOK_PHOTOS];
  // A removal with its reason, for a post and a comment: their own bound, as the longest, "We removed
  // your comment for pretending to be someone else.", is 58 characters.
  const reasonSentences = Object.keys(REASON_WORDS).flatMap((reason) => [decisionLine('post_removed', reason), decisionLine('comment_removed', reason)]);
  eq('a removal names each of the six reasons, for a post and for a comment', reasonSentences.length, 12);
  for (const line of [...sentences, ...reasonSentences, ...names, post.label, said.label]) {
    check('no dash of any kind in the new words', !DASH.test(line), line);
  }
  for (const line of names) {
    check('a short plain name: a capital, no full stop', /^[A-Z][^!.]*$/.test(line) && line.length <= 40, line);
  }
  for (const line of sentences) {
    check('a short plain sentence: a capital, a full stop, no exclamation mark', /^[A-Z][^!]*\.$/.test(line) && line.length <= 50, line);
  }
  for (const line of reasonSentences) {
    check('a removal with its reason: a plain sentence of at most 60 characters', /^[A-Z][^!]*\.$/.test(line) && line.length <= 60, line);
  }
}

// ---- likes (BRD-06, 2026-11-24): the words, the reader, the heart, the Likes sheet and the bell ----
{
  eq("the heart on someone else's post, for VoiceOver", LIKE_LABEL, 'Like');
  eq("the Likes sheet's heading", LIKES_TITLE, 'Likes');
  eq("the Likes sheet's close mark and dim area", LIKES_CLOSE, 'Close likes');
  eq('the Likes sheet with nobody in it', NO_LIKES, 'No likes yet.');
  eq('the Likes sheet while your posts list reads it', LIKES_LOADING, 'Loading likes.');
  eq('the hint on the heart under your own post', LIKES_HINT, 'Opens who liked it.');
  eq('a like or an undo that would not save', LIKE_FAILED, "Couldn't save that. Please try again.");
  eq("the bell's line after the first name", LIKED_YOUR_POST, ' liked your post.');
  eq('the heart under your own post says when nobody liked it', likesSpoken(0), 'No likes yet');
  eq('one like', likesSpoken(1), '1 like');
  eq('three likes', likesSpoken(3), '3 likes');
  eq('no number beside the heart at none', likeCount(0), '');
  eq('nor below none', likeCount(-1), '');
  eq('nor when unknown', likeCount(Number.NaN), '');
  eq('and nothing spoken but none then', likesSpoken(Number.NaN), 'No likes yet');
  eq('the number beside the heart', likeCount(12), '12');
  for (const code of ['not_found', 'own_post', 'server_error', null]) {
    eq(`a like that failed with ${code} gets its one line`, commentFailureLine(code, 'like'), LIKE_FAILED);
  }

  // Built from escapes so this file holds no dash of its own: the hyphen, U+2010 to U+2015, the minus.
  const DASH = /[-‐-―−]/;
  const words = [LIKE_LABEL, LIKES_TITLE, LIKES_CLOSE, NO_LIKES, LIKES_LOADING, LIKES_HINT, LIKE_FAILED, LIKED_YOUR_POST, likesSpoken(0), likesSpoken(1), likesSpoken(3)];
  for (const line of words) check('no dash of any kind in the like words', !DASH.test(line), line);
  for (const line of [NO_LIKES, LIKES_LOADING, LIKES_HINT, LIKE_FAILED]) {
    check('a like line is a short plain sentence: a capital, a full stop, no exclamation mark', /^[A-Z][^!]*\.$/.test(line) && line.length <= 50, line);
  }
  for (const line of [LIKE_LABEL, LIKES_TITLE, LIKES_CLOSE]) {
    check('a like name is short and plain: a capital, no full stop', /^[A-Z][^!.]*$/.test(line) && line.length <= 40, line);
  }

  // The reader: any stored name counts as a like, and the list keeps the server's order.
  const board = source('lib/api/board.ts');
  check(
    'liked is any name the server sends, never read through the reaction names',
    /liked: typeof raw\.my_reaction === 'string' && raw\.my_reaction !== '',/.test(board) && !/isReactionName\(raw\.my_reaction\)|reaction\(raw\.my_reaction\)/.test(board)
  );
  check('can_react is read only as true', /can_react: raw\.can_react === true,/.test(board));
  check(
    "the likes are the server's people in its order, null when it sends no list",
    /likes: Array\.isArray\(raw\.received\)\s*\? list\(raw\.received\)\s*\.map\(\(row\) => readPerson\(row\.person\)\)\s*\.filter\(\(person\): person is Person => person !== null\)\s*: null,/.test(board)
  );
  check(
    'a like sends love and an undo sends null, to the reaction route, after the id check',
    /export async function setLike\(entryId: string, liked: boolean\): Promise<boolean> \{\s*checkId\(entryId\);\s*const raw = await api\.post<Raw>\(`\/posts\/\$\{entryId\}\/reaction`, \{ reaction: liked \? 'love' : null \}\);\s*return typeof raw\?\.my_reaction === 'string' && raw\.my_reaction !== '';\s*\}/.test(
      board
    )
  );

  // The hook: the heart changes at once, one request at a time, and the server ends at the last tap.
  const hook = source('components/board/usePost.ts');
  const toggle = hook.slice(hook.indexOf('const toggleLike = useCallback('), hook.indexOf('return { status,'));
  check('nothing on your own post or one you may not like', /if \(now === null \|\| now\.card\.mine \|\| !now\.can_react\) return null;/.test(toggle));
  check(
    'a tap drops a read already on its way, then shows the new heart at once',
    /return null;(?:\s*\/\/[^\n]*)*\s*\+\+seqRef\.current;\s*wantRef\.current = !now\.card\.liked;\s*put\(\{ \.\.\.now, card: \{ \.\.\.now\.card, liked: wantRef\.current \} \}\);/.test(toggle)
  );
  check(
    'one like request at a time: a tap while one is on its way only changes the choice',
    /if \(likingRef\.current\) return null;\s*likingRef\.current = true;/.test(toggle) && /finally \{\s*likingRef\.current = false;\s*\}/.test(toggle)
  );
  check('it sends the newest choice until the server has it', /while \(wantRef\.current !== saved\) saved = await setLike\(id, wantRef\.current\);\s*settle\(saved\);/.test(toggle));
  check(
    "the server's answer is kept, and a read that began while the like was out can't flip it back",
    /const settle = useCallback\(\s*\(saved: boolean\) => \{\s*\+\+seqRef\.current;\s*const now = postRef\.current;\s*if \(mountedRef\.current && now !== null && now\.card\.liked !== saved\) put\(/.test(hook)
  );
  // A like whose answer never came may still have been saved: the read after the rollback shows
  // the heart the server holds, so the owner's Likes list and the liker's heart agree.
  check(
    'a failure puts the heart back; then a post gone for you is read again with no line, and no answer or a server failure reads it again with the line',
    /catch \(err\) \{\s*wantRef\.current = saved;\s*settle\(saved\);\s*if \(err instanceof ApiError && err\.code === 'not_found'\) \{(?:\s*\/\/[^\n]*)*\s*void load\(\);\s*return null;\s*\}(?:\s*\/\/[^\n]*)*\s*if \(isRetryable\(err\)\) void load\(\);\s*return socialErrorMessage\(err, 'like'\);/.test(
      toggle
    ) && /import \{ ApiError, isRetryable \} from '@\/lib\/api\/client';/.test(hook)
  );
  check('the page gets the toggle', /cancelReply, toggleLike \};/.test(hook) && /toggleLike: \(\) => Promise<string \| null>;/.test(hook));

  // The heart under the photo: ink, an icon and not an emoji, beside the comment mark and never inside it.
  const caption = source('components/entry/PostCaption.tsx');
  const commentMark = caption.slice(caption.indexOf('{onComment ? ('), caption.indexOf(') : null}', caption.indexOf('{onComment ? (')));
  check(
    'the heart sits before the comment mark, outside it, so comments off hides only the mark',
    /\{like \|\| onComment \? \(/.test(caption) &&
      /\{like \? <LikeButton like=\{like\} \/> : null\}\s*\{onComment \? \(/.test(caption) &&
      commentMark !== '' &&
      !commentMark.includes('LikeButton')
  );
  check('the gap keeps the two hit areas apart', /marginLeft: like \? space\.md : 0,/.test(caption) && (caption.match(/hitSlop=\{8\}/g) ?? []).length === 3);
  check(
    "someone else's post: the outline heart fills, in ink both ways, a button that says Like and whether it is liked",
    /<Ionicons name=\{liked \? 'heart' : 'heart-outline'\} size=\{COMMENT_ICON\} color=\{color\.ink\} \/>/.test(caption) &&
      /accessibilityLabel=\{LIKE_LABEL\}\s*accessibilityState=\{\{ selected: liked \}\}/.test(caption)
  );
  check(
    'your own post: the outline heart in ink, the number beside it only when there is one, said and hinted',
    /<Ionicons name="heart-outline" size=\{COMMENT_ICON\} color=\{color\.ink\} \/>\s*\{number !== '' \? \(/.test(caption) &&
      /const number = likeCount\(like\.count\);/.test(caption) &&
      /accessibilityLabel=\{likesSpoken\(like\.count\)\}\s*accessibilityHint=\{LIKES_HINT\}/.test(caption)
  );
  check(
    'only a tap pops the heart, and never under Reduce Motion or on the first draw',
    /const reduceMotion = useReduceMotion\(\);/.test(caption) &&
      /if \(taps === 0 \|\| reduceMotion\) return;\s*scale\.value = withSequence\(/.test(caption) &&
      /\}, \[taps\]\);/.test(caption) &&
      /setTaps\(\(n\) => n \+ 1\);\s*onToggle\(\);/.test(caption)
  );

  // The Likes sheet.
  const sheet = source('components/board/LikesSheet.tsx');
  check('the sheet is headed Likes, a header for VoiceOver', /<AppText role="heading" accessibilityRole="header">\s*\{LIKES_TITLE\}\s*<\/AppText>/.test(sheet));
  eq('the close mark and the dim area both say Close likes', (sheet.match(/accessibilityLabel=\{LIKES_CLOSE\}/g) ?? []).length, 2);
  check(
    "each person's row opens their profile",
    /<PersonRow person=\{person\} first=\{index === 0\} onPress=\{\(\) => open\(person\)\} pressHint="Opens their profile\." \/>/.test(sheet)
  );
  check(
    'a tap on a person closes the sheet first, so their page never opens under the dim cover',
    /function open\(person: Person\) \{(?:\s*\/\/[^\n]*)*\s*onClose\(\);\s*setTimeout\(\(\) => openPerson\(person\.id\), 0\);\s*\}/.test(sheet)
  );
  check(
    'nobody yet, the loading line, the gone line and Try again',
    /\{NO_LIKES\}/.test(sheet) && /\{LIKES_LOADING\}/.test(sheet) && /\{POST_GONE_LINE\}/.test(sheet) && /<StatusMessage\s*kind=\{read\.failure\}/.test(sheet)
  );
  check(
    'with only the post id, the sheet reads it each time it opens, and only the newest answer counts',
    /if \(!given && entryId !== undefined\) void readLikes\(entryId\);/.test(sheet) &&
      /const seq = \+\+seqRef\.current;/.test(sheet) &&
      (sheet.match(/if \(seq !== seqRef\.current\) return;/g) ?? []).length === 2 &&
      /setRead\(\{ state: 'ready', likes: post\.likes \?\? \[\] \}\);/.test(sheet)
  );
  check('under Reduce Motion the sheet appears at once', /if \(reduceMotion\) \{\s*offset\.value = 0;\s*return;\s*\}/.test(sheet));

  // The three pages with the heart.
  const postView = source('components/board/PostView.tsx');
  check(
    "a toggle only on someone else's post you may like, the count only on your own",
    /if \(post\.card\.mine\) return post\.likes !== null \? \{ mode: 'count', count: post\.likes\.length, onOpen \} : undefined;\s*return post\.can_react \? \{ mode: 'toggle', liked: post\.card\.liked, onToggle \} : undefined;/.test(
      postView
    ) && /like=\{likeControl\(post, toggleLike, \(\) => setLikesOpen\(true\)\)\}/.test(postView)
  );
  check(
    'a like that would not save shows its line for a moment under the Back row, not in the page',
    /void load\.toggleLike\(\)\.then\(\(line\) => \{\s*if \(line !== null\) setReportNotice\(line\);\s*\}\);/.test(postView) && !/setNotice\(line\)/.test(postView)
  );
  check('the post page opens Likes with the list it read', /<LikesSheet visible=\{likesOpen\} likes=\{post\?\.likes \?\? \[\]\} onClose=\{\(\) => setLikesOpen\(false\)\} \/>/.test(postView));

  const entryView = source('components/entry/EntryView.tsx');
  const entryLike = entryView.slice(entryView.indexOf('like={'), entryView.indexOf('onComment={social.status'));
  check(
    'your entry page: the count waits for the post read, as the comment mark does, and never asks whether commenting is on',
    /social\.status === 'ready' && social\.post !== null && social\.post\.likes !== null/.test(entryLike) &&
      /\{ mode: 'count', count: social\.post\.likes\.length, onOpen: \(\) => setLikesOpen\(true\) \}/.test(entryLike) &&
      !/can_comment/.test(entryLike)
  );
  check('your entry page opens Likes with the list it read', /<LikesSheet visible=\{likesOpen\} likes=\{social\.post\?\.likes \?\? \[\]\} onClose=\{\(\) => setLikesOpen\(false\)\} \/>/.test(entryView));

  const feed = source('components/profile/PostsFeed.tsx');
  check("your posts list: the diary's like count beside each heart", /like=\{\{ mode: 'count', count: social\.like_count, onOpen: onOpenLikes \}\}/.test(feed));
  check(
    'and Likes reads who liked that post when it opens',
    /onOpenLikes=\{\(\) => setLikesFor\(entry\.id\)\}/.test(feed) &&
      /<LikesSheet visible=\{likesFor !== null\} entryId=\{likesFor \?\? undefined\} onClose=\{\(\) => setLikesFor\(null\)\} \/>/.test(feed)
  );

  // The bell: "Maya liked your post.", named by first name like every other row.
  const bell = source('components/today/NotificationsScreen.tsx');
  const likeCase = bell.slice(bell.indexOf("case 'like':"), bell.indexOf("case 'follow':"));
  check(
    'a like names the person by first name, then "liked your post.", spoken the same',
    /const name = item\.actor \? firstName\(item\.actor\) : '';/.test(bell) &&
      /case 'like':(?:\s*\/\/[^\n]*)*\s*return \{ name, rest: LIKED_YOUR_POST, spoken: `\$\{name\}\$\{LIKED_YOUR_POST\}` \};/.test(likeCase) &&
      !/username/.test(likeCase)
  );
}

// ---- moderation: the second look's rules, how a report ends, and a comment leaving the list ------
{
  // Who can ask: a removal with its id, while the server sends no second look on it.
  const removal = { kind: 'post_removed', removal: '00000008-0000-4000-8000-000000000008', second_look: '' };
  check('a removed post can ask', canAskSecondLook(removal));
  check('a removed comment can ask', canAskSecondLook({ ...removal, kind: 'comment_removed' }));
  for (const state of ['asked', 'kept', 'restored', 'closed', 'expired', 'ASKED', ' ']) {
    check(`not once its second look is ${JSON.stringify(state)}`, !canAskSecondLook({ ...removal, second_look: state }));
  }
  check('not without the removal id', !canAskSecondLook({ ...removal, removal: '' }));
  for (const kind of DECISION_KINDS.filter((k) => !isRemovalKind(k))) {
    check(`${kind} never asks`, !canAskSecondLook({ ...removal, kind }));
  }
  check('nor does any other kind', !['comment', 'reply', 'follow', 'spin_open', 'removed', ''].some((kind) => canAskSecondLook({ ...removal, kind })));
  eq('the removals are the two kinds that ask', DECISION_KINDS.filter((kind) => isRemovalKind(kind)).join(','), 'post_removed,comment_removed');
  // "Second look asked." only while the admin has not answered.
  check('asked: the line shows', isSecondLookAsked({ ...removal, second_look: 'asked' }));
  check('on a removed comment too', isSecondLookAsked({ ...removal, kind: 'comment_removed', second_look: 'asked' }));
  for (const state of ['', 'kept', 'restored', 'closed', 'expired']) {
    check(`no line while it is ${JSON.stringify(state)}`, !isSecondLookAsked({ ...removal, second_look: state }));
  }
  check('never on a kept item', !isSecondLookAsked({ ...removal, kind: 'post_kept', second_look: 'asked' }));

  // A page opened without the bell's item, for a removal that can't ask: a plain line instead of the form.
  eq('asked: the page says so', secondLookStateLine('asked'), 'Second look asked.');
  eq('kept: the page says it was answered', secondLookStateLine('kept'), 'We already took another look.');
  eq('restored: answered too', secondLookStateLine('restored'), 'We already took another look.');
  eq('the answered line is the 409 second_look_back one', secondLookStateLine('kept'), secondLookFailure('post_removed', 409, 'second_look_back').line);
  for (const state of ['closed', '', 'expired', 'ASKED', 'Kept', ' ']) {
    eq(`${JSON.stringify(state)}: the page says it isn't here`, secondLookStateLine(state), "This isn't here any more.");
  }

  // The box at Send, counted as the server counts: trimmed, a line break as \n, in characters.
  const emoji = '\u{1F600}';
  eq('nothing written', secondLookTextProblem(''), 'Tell us why first.');
  eq('only spaces and line breaks', secondLookTextProblem('  \n\t \r\n '), 'Tell us why first.');
  eq('one character', secondLookTextProblem('x'), '');
  eq('500 characters', secondLookTextProblem('x'.repeat(500)), '');
  eq('501 characters', secondLookTextProblem('x'.repeat(501)), 'At most 500 characters.');
  eq('the spaces round 500 characters are not counted', secondLookTextProblem(`  ${'x'.repeat(500)}\n `), '');
  eq('an emoji counts once: 500 of them', secondLookTextProblem(emoji.repeat(500)), '');
  eq('501 emoji', secondLookTextProblem(emoji.repeat(501)), 'At most 500 characters.');
  eq('a CRLF line break counts once', secondLookTextProblem(`${'x'.repeat(249)}\r\n${'x'.repeat(250)}`), '');

  // A refusal, where its line shows and the line (null: the client's own words).
  const fail = (kind: string, status: number, code: string, field?: string, reason?: string, photo?: number) =>
    JSON.stringify(secondLookFailure(kind, status, code, field, reason, photo));
  const at = (where: string, line: string | null) => JSON.stringify({ at: where, line });
  for (const kind of ['post_removed', 'comment_removed']) {
    eq(`${kind}: nothing written, said by the server`, fail(kind, 400, 'validation_failed', 'text', 'required'), at('text', 'Tell us why first.'));
    eq(`${kind}: too long`, fail(kind, 400, 'validation_failed', 'text', 'too_long'), at('text', 'At most 500 characters.'));
    eq(`${kind}: a character the server refuses`, fail(kind, 400, 'validation_failed', 'text', 'invalid'), at('text', "That has a character we can't use."));
    eq(`${kind}: a photo the server refuses`, fail(kind, 400, 'validation_failed', 'photo_keys', 'invalid'), at('photos', "Couldn't use those photos. Please try again."));
    eq(`${kind}: a file storage refused`, fail(kind, 400, 'photo_rejected'), at('photos', null));
    eq(`${kind}: a photo that would not upload`, fail(kind, 500, 'photo_upload_failed'), at('photos', null));
    // At Send the page knows which photo was uploading, and names it by its place.
    const named = (n: number) => `Photo ${n} couldn't upload. Remove it or try again.`;
    eq(`${kind}: the second photo would not upload`, fail(kind, 500, 'photo_upload_failed', '', '', 2), at('photos', named(2)));
    eq(`${kind}: storage refused the first photo`, fail(kind, 400, 'photo_rejected', undefined, undefined, 1), at('photos', named(1)));
    eq(`${kind}: the third photo`, fail(kind, 500, 'photo_upload_failed', '', '', 3), at('photos', named(3)));
    eq(`${kind}: no connection while a photo uploads keeps the connection line`, fail(kind, 0, 'network_unavailable', '', '', 2), at('form', null));
    eq(`${kind}: a place names only a photo that would not upload`, fail(kind, 409, 'second_look_asked', '', '', 2), at('form', 'You already asked for a second look.'));
    eq(`${kind}: asked already`, fail(kind, 409, 'second_look_asked'), at('form', 'You already asked for a second look.'));
    eq(`${kind}: past the 30 days`, fail(kind, 409, 'second_look_too_late'), at('form', "It's too late to ask. It's been 30 days."));
    eq(`${kind}: answered by a Bring it back, never saying it is back`, fail(kind, 409, 'second_look_back'), at('form', 'We already took another look.'));
    eq(`${kind}: too many tries`, fail(kind, 429, 'rate_limited'), at('form', 'Too many tries. Wait a minute and try again.'));
    eq(`${kind}: a 429 whose body did not read`, fail(kind, 429, 'server_error'), at('form', 'Too many tries. Wait a minute and try again.'));
    eq(`${kind}: no connection`, fail(kind, 0, 'network_unavailable'), at('form', null));
    eq(`${kind}: a server failure`, fail(kind, 500, 'server_error'), at('form', 'Something went wrong. Please try again.'));
    eq(`${kind}: anything but an answer`, fail(kind, 0, ''), at('form', 'Something went wrong. Please try again.'));
    eq(`${kind}: another field's 400`, fail(kind, 400, 'validation_failed', 'removal', 'invalid'), at('form', 'Something went wrong. Please try again.'));
  }
  eq('a post that went away', fail('post_removed', 404, 'not_found'), at('form', "This post isn't here any more."));
  eq('a comment that went away', fail('comment_removed', 404, 'not_found'), at('form', "This comment isn't here any more."));
  eq('a 404 whose body did not read', fail('post_removed', 404, 'server_error'), at('form', "This post isn't here any more."));

  // The bell keeps the item for the page, which finds it by the removal's id alone.
  const first = '00000008-0000-4000-8000-000000000008';
  const second = '00000009-0000-4000-8000-000000000009';
  eq('nothing is kept before a tap', chosenSecondLook(first), null);
  chooseSecondLook({ removal: first, kind: 'post_removed', text: 'Dune' });
  eq('the page finds the item the bell kept', chosenSecondLook(first)?.text, 'Dune');
  eq('another removal finds nothing', chosenSecondLook(second), null);
  eq('nor does an empty id', chosenSecondLook(''), null);
  chooseSecondLook({ removal: second, kind: 'comment_removed', text: '' });
  eq('a newer tap takes its place', chosenSecondLook(first), null);
  eq('with its own item', chosenSecondLook(second)?.kind, 'comment_removed');

  // A send that went through after the person left the page: the bell, while open, reads its list again.
  let told = 0;
  const stop = onSecondLookSent(() => {
    told += 1;
  });
  secondLookSent();
  eq('the open bell is told once', told, 1);
  stop();
  secondLookSent();
  eq('a bell that has closed is not told', told, 1);

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
  check('the bell keeps the eight decision kinds', /'bonus_spin',\s*\.\.\.DECISION_KINDS,\s*\];/.test(api));
  check(
    'the bell keeps a like, right after a reply, in its kinds and in their type',
    /'reply',\s*'like',\s*'follow',/.test(api) && /\| 'reply'\s*\| 'like'\s*\| 'follow'/.test(api)
  );
  check(
    'the ref, the removal id and its second look are read as strings, empty when missing; no address, and no key named email',
    /ref: typeof raw\.ref === 'string' \? raw\.ref : '',/.test(api) &&
      /removal: typeof raw\.removal === 'string' \? raw\.removal : '',/.test(api) &&
      /second_look: typeof raw\.second_look === 'string' \? raw\.second_look : '',/.test(api) &&
      !/contact/.test(api) &&
      !/email/.test(api)
  );
  check("a removal's reason is read as a string, empty when missing", /reason: typeof raw\.reason === 'string' \? raw\.reason : '',/.test(api));
  const bell = source('components/today/NotificationsScreen.tsx');
  check(
    'the eight kinds say their sentence (a removal with its reason), spoken the same with "Second look asked." after it while asked, and draw the wheel mark, not an empty face',
    /case 'post_hidden':\s*case 'comment_hidden':\s*case 'post_removed':\s*case 'comment_removed':\s*case 'post_back':\s*case 'comment_back':\s*case 'post_kept':\s*case 'comment_kept': \{[\s\S]{0,300}const line = decisionLine\(item\.kind, item\.reason\);\s*return \{ name: '', rest: line, spoken: isSecondLookAsked\(item\) \? `\$\{line\} \$\{SECOND_LOOK_ASKED\}` : line \};/.test(
      bell
    ) && /if \(item\.kind === 'spin_open' \|\| isDecisionKind\(item\.kind\)\) \{/.test(bell)
  );
  check(
    'a comment, a like, or a post back or hidden opens your entry page; a comment back or hidden opens the post it is on',
    /if \(item\.kind === 'comment' \|\| item\.kind === 'like' \|\| item\.kind === 'post_back' \|\| item\.kind === 'post_hidden'\) \{\s*if \(item\.entry_id === null\) return;\s*openEntry\(item\.entry_id\);/.test(
      bell
    ) &&
      /if \(item\.kind === 'reply' \|\| item\.kind === 'comment_back' \|\| item\.kind === 'comment_hidden'\) \{\s*if \(item\.entry_id === null\) return;\s*void openReply\(item\.entry_id\);/.test(bell)
  );
  check(
    'a removal opens the second look only while it can ask; once asked, and on a kept item, a tap does nothing',
    /if \(item\.kind === 'post_removed' \|\| item\.kind === 'comment_removed'\) \{[^}]*if \(canAskSecondLook\(item\)\) openSecondLook\(item\);\s*return;\s*\}\s*if \(item\.kind === 'post_kept' \|\| item\.kind === 'comment_kept'\) return;/.test(
      bell
    )
  );
  check(
    'the page opens in this stack with only the removal id, the item kept for it first',
    /chooseSecondLook\(\{ removal: item\.removal, kind: item\.kind, text: item\.text \?\? '' \}\);\s*router\.push\(\{\s*pathname: '\/\(app\)\/today\/second-look',\s*params: \{ removal: item\.removal \},\s*\}\);/.test(
      bell
    )
  );
  check(
    'one tap opens one page: another tap before it shows does nothing, until Notifications is back in view',
    /function openSecondLook\(item: NotificationItem & \{ kind: RemovalKind \}\) \{\s*if \(openingRef\.current\) return;\s*openingRef\.current = true;\s*chooseSecondLook\(/.test(bell) &&
      /useFocusEffect\(\s*useCallback\(\(\) => \{\s*\/\/[^\n]*\n\s*openingRef\.current = false;\s*void load\(\);\s*\}, \[load\]\)\s*\);/.test(bell)
  );
  check(
    'a second look that went through after the person left its page: the bell reads its list again, and only the newest read is used',
    /useEffect\(\(\) => onSecondLookSent\(\(\) => void load\(\)\), \[load\]\);/.test(bell) &&
      /const seq = \+\+seqRef\.current;/.test(bell) &&
      /const answer = await getNotifications\(\);\s*if \(seq !== seqRef\.current\) return;/.test(bell) &&
      /if \(seq !== seqRef\.current \|\| itemsRef\.current !== null\) return;/.test(bell)
  );
  check('the email is gone from the bell: no mail app, no address', !/Linking|mailto|secondLookFallback|contact/.test(bell));
  check('the hint only where the page opens', /accessibilityHint=\{canAskSecondLook\(item\) \? SECOND_LOOK_HINT : undefined\}/.test(bell));
  check(
    '"Second look asked." under the sentence while it waits: grey, a state and not an action',
    /\{isSecondLookAsked\(item\) \? \(\s*\/\/[^\n]*\n\s*<Text maxFontSizeMultiplier=\{1\.3\} style=\{\{ \.\.\.rowText, color: color\.muted \}\}>\s*\{SECOND_LOOK_ASKED\}\s*<\/Text>\s*\) : null\}/.test(
      bell
    )
  );
  check(
    'a comment, a like, a reply, or a post back or hidden shows its picture; a removed item never does',
    /\(item\.kind === 'comment' \|\| item\.kind === 'like' \|\| item\.kind === 'reply' \|\| item\.kind === 'post_back' \|\| item\.kind === 'post_hidden'\) &&\s*item\.photo_url !== ''/.test(
      bell
    )
  );
  const pushTarget = source('lib/notifications/pushTarget.ts');
  check('no phone push for the decisions', !DECISION_KINDS.some((kind) => pushTarget.includes(kind)));
  check('no phone push for a like', !pushTarget.includes("'like'"));

  // Nothing red: only Log out is.
  for (const rel of [
    'app/(app)/_layout.tsx',
    'components/board/Comments.tsx',
    'components/board/PostView.tsx',
    'components/board/PostCard.tsx',
    'components/board/LikesSheet.tsx',
    'components/people/PersonScreen.tsx',
    'components/entry/EntryView.tsx',
    'components/entry/PostCaption.tsx',
    'components/profile/PostsFeed.tsx',
    'components/today/SettledBlock.tsx',
    'components/today/NotificationsScreen.tsx',
    'components/notifications/SecondLookScreen.tsx',
    'app/(app)/today/second-look.tsx',
  ]) {
    check(`${rel}: nothing red`, !/destructive/.test(source(rel)));
  }

  // The second look's page (2026-11-19).
  const look = source('components/notifications/SecondLookScreen.tsx');
  check('the page is in the Spin page stack', /<Stack\.Screen name="second-look" \/>/.test(source('app/(app)/today/_layout.tsx')));
  check('with no bottom bar', /const BARLESS_TODAY_PAGES = \[[^\]]*'second-look'/.test(layout));
  check('the route hands the page the removal id alone', /<SecondLookScreen removal=\{removal \?\? ''\} \/>/.test(source('app/(app)/today/second-look.tsx')));
  check(
    'the page shows the item the bell kept, or reads the bell list for it',
    /useState<SecondLookItem \| null>\(\(\) => chosenSecondLook\(removal\)\)/.test(look) && /if \(item === null\) void readItem\(\);/.test(look)
  );
  check(
    'read from the bell list, a removal that cannot ask shows a plain line by its state instead of the form',
    /if \(found === undefined \|\| !canAskSecondLook\(found\)\) \{\s*\/\/[^\n]*\n\s*setClosedLook\(found\?\.second_look \?\? ''\);\s*setStatus\('closed'\);\s*return;\s*\}/.test(look) &&
      /status === 'closed' \|\| line === null \? \(\s*<View style=\{\{ marginTop: space\.lg \}\}>\s*<AppText role="body">\{secondLookStateLine\(closedLook\)\}<\/AppText>/.test(look) &&
      !/isRemovalKind|GONE_LINE/.test(look)
  );
  check(
    'Send is grey only while it sends, one send at a time',
    /<PrimaryButton label="Send" loading=\{sending\} onPress=\{\(\) => void send\(\)\} \/>/.test(look) &&
      !/disabled=\{!|onBlockedPress/.test(look) &&
      /if \(item === null \|\| sendingRef\.current\) return;/.test(look)
  );
  check('the box has no length cap to cut a paste short; Send says it is too long', !/maxLength/.test(look));
  check(
    'nothing written or too much: the line under the box, said, scrolled to, and the cursor in it',
    /const problem = secondLookTextProblem\(text\);\s*if \(problem !== ''\) \{\s*showTextProblem\(problem\);\s*return;\s*\}/.test(look) &&
      /setTextError\(line\);\s*announceProblem\(line, SECOND_LOOK_LABEL\);\s*bringIntoView\(scrollRef\.current, textFieldRef\.current\);\s*textBoxRef\.current\?\.focus\(\);/.test(look)
  );
  check(
    'the photos and the form show their lines with the error look',
    /<FieldError text=\{photosError\} label=\{PHOTOS_LABEL\} \/>/.test(look) && /<FieldError text=\{formError\} \/>/.test(look)
  );
  check(
    'the photos upload at Send, one after another and each once, then the request names them',
    /for \(const \[index, photo\] of photos\.entries\(\)\) \{\s*uploading = index \+ 1;\s*keys\.push\(await uploads\.ensure\(photo\.uri, \(\) => uploadEntryPhoto\(photo\.uri, photo\.mimeType\)\)\);\s*\}\s*uploading = 0;\s*await askSecondLook\(item\.removal, text, keys\);/.test(
      look
    )
  );
  check(
    'a photo that would not upload is named by its place',
    /secondLookFailure\(kind, answer\?\.status \?\? 0, answer\?\.code \?\? '', answer\?\.field, answer\?\.reason, photo\)/.test(look)
  );
  check('photos the server refused upload again at the next Send', /if \(answer\?\.code === 'validation_failed'\) uploads\.forget\(\);/.test(look));
  check('a removed photo is forgotten', /function dropPhoto\(uri: string\) \{\s*uploads\.forget\(uri\);/.test(look));
  check(
    'sent: the line over Notifications, and back only while the page is still the one showing; otherwise Notifications reads its list again',
    /setReportNotice\(SECOND_LOOK_SENT\);\s*\/\/[^\n]*\n\s*if \(mountedRef\.current && navigation\.isFocused\(\)\) goBack\(\);\s*else secondLookSent\(\);/.test(look)
  );
  check(
    'a failure after the person left the page: "Second look not sent." over the page then showing; on the page, its own line as before',
    /return navigation\.addListener\('beforeRemove', \(\) => \{\s*leftRef\.current = true;\s*\}\);/.test(look) &&
      /if \(leftRef\.current \|\| !mountedRef\.current\) setReportNotice\(SECOND_LOOK_NOT_SENT\);\s*else showFailure\(err, item\.kind, uploading\);/.test(look)
  );
  check('up to 3 photos: Add photos goes at 3', /\{photos\.length < SECOND_LOOK_MAX_PHOTOS \? \(/.test(look));
  check("photo access off: the phone's alert, with a way to Settings", /showAccessOff\(source\);/.test(look));
  check('Add photos is the photo mark and ink words, never a pill or underlined text', !/textDecorationLine|TextButton|radius\.full|<Chip/.test(look));
  const dashes = new RegExp('[' + String.fromCharCode(0x2013, 0x2014) + ']');
  check('the page has no em or en dash', !dashes.test(look));
  check(
    'the request names the removal and sends the words and the photo keys',
    /api\.post<unknown>\(`\/removals\/\$\{encodeURIComponent\(removal\)\}\/second-look`, \{ text, photo_keys: photoKeys \}\)/.test(source('lib/api/secondLooks.ts'))
  );
  const types = source('lib/api/types.ts');
  check('the three refusals are error codes the app knows', ['second_look_asked', 'second_look_too_late', 'second_look_back'].every((code) => types.includes(`| '${code}'`)));
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
