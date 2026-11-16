// @ts-nocheck
/**
 * Throwaway checks for the pure entry logic (plan 03-02 Task 1). Not part of the app bundle.
 * Run with:  node --experimental-strip-types scripts/check-entry.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 * This file holds no emoji: the reactions are checked by code point.
 */
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { STALE_AFTER_MS, isStale, photoSource } from '../lib/api/photoSource.ts';
import { REACTIONS, isReactionName } from '../lib/entry/reactions.ts';
import {
  COMMENT_MAX,
  TITLE_MAX,
  changedFields,
  cleanTitle,
  isEntryId,
  missingParts,
  nextSpinLine,
  SETTLED_LABEL,
  LATE_LINE,
  missedLine,
  BOARD_OPEN_LINE,
  MISSED_SPIN_LINE,
  skipBody,
  stillNeededLine,
} from '../lib/entry/rules.ts';
import {
  STARS_MAX,
  STARS_MIN,
  formatStars,
  starFromTap,
  starGlyphs,
  starsSpoken,
  stepStars,
} from '../lib/entry/stars.ts';
import {
  PHOTOS_MAX,
  PHOTO_MAX_BYTES,
  PHOTO_TYPES,
  canAddPhoto,
  createUploadMemory,
  photoListChanged,
  photoMessage,
  photoProblem,
} from '../lib/entry/photo.ts';
import { dateDot, dateLine } from '../lib/wheel/format.ts';

let checks = 0;
let failures = 0;

function check(name: string, ok: boolean, detail?: string): void {
  checks += 1;
  if (!ok) {
    failures += 1;
    console.log(`FAIL ${name}${detail ? `: ${detail}` : ''}`);
  }
}

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

const DOT = '\u00B7';
const NBSP = '\u00A0';

// ---- stars: taps -----------------------------------------------------------------------------
{
  const values: number[] = [];
  for (let k = 1; k <= 5; k += 1) {
    check(`left tap on star ${k}`, starFromTap(k, 'left') === 2 * k - 1, `${starFromTap(k, 'left')}`);
    check(`right tap on star ${k}`, starFromTap(k, 'right') === 2 * k, `${starFromTap(k, 'right')}`);
    values.push(starFromTap(k, 'left'), starFromTap(k, 'right'));
  }
  check('ten taps give exactly 1 to 10', same([...values].sort((a, b) => a - b), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]), values.join(','));
  check('constants', STARS_MIN === 1 && STARS_MAX === 10);
  // A tap can never leave 1 to 10, even for a star number outside 1 to 5.
  for (const k of [-3, 0, 6, 99, 2.5, Number.NaN]) {
    for (const side of ['left', 'right']) {
      const v = starFromTap(k, side);
      check(`tap on star ${k} ${side} stays in 1..10`, Number.isInteger(v) && v >= 1 && v <= 10, `${v}`);
    }
  }
}

// ---- stars: steps ----------------------------------------------------------------------------
{
  check('decrement from 0 stays 0', stepStars(0, -1) === 0);
  check('increment from 0 gives 1', stepStars(0, 1) === 1);
  check('no change from 0 stays 0', stepStars(0, 0) === 0);
  check('decrement from 1 stays 1', stepStars(1, -1) === 1);
  check('increment from 10 stays 10', stepStars(10, 1) === 10);
  check('increment from 5', stepStars(5, 1) === 6);
  check('decrement from 5', stepStars(5, -1) === 4);
  // A long walk: once started it never returns to 0 and never leaves 1 to 10.
  let seed = 20261013;
  const rand = () => {
    seed = (seed * 1664525 + 1013904223) % 4294967296;
    return seed / 4294967296;
  };
  let v = 0;
  let started = false;
  let ok = true;
  for (let i = 0; i < 5000; i += 1) {
    v = stepStars(v, rand() < 0.5 ? -1 : 1);
    if (v >= 1) started = true;
    if (started && (v < 1 || v > 10 || !Number.isInteger(v))) ok = false;
  }
  check('walk stays within 1 to 10 once started', ok && started, `ended at ${v}`);
}

// ---- stars: figure and glyphs ------------------------------------------------------------------
{
  const want = ['0.5', '1', '1.5', '2', '2.5', '3', '3.5', '4', '4.5', '5'];
  for (let v = 1; v <= 10; v += 1) check(`formatStars ${v}`, formatStars(v) === want[v - 1], formatStars(v));
  check('formatStars 0 shows no figure', formatStars(0) === '');
  check('starGlyphs(7)', same(starGlyphs(7), ['full', 'full', 'full', 'half', 'empty']), starGlyphs(7).join(','));
  check('starGlyphs(1)', same(starGlyphs(1), ['half', 'empty', 'empty', 'empty', 'empty']), starGlyphs(1).join(','));
  check('starGlyphs(10)', same(starGlyphs(10), ['full', 'full', 'full', 'full', 'full']));
  check('starGlyphs(0)', same(starGlyphs(0), ['empty', 'empty', 'empty', 'empty', 'empty']));
  for (let v = 0; v <= 10; v += 1) {
    const g = starGlyphs(v);
    const total = g.filter((x) => x === 'full').length * 2 + g.filter((x) => x === 'half').length;
    check(`starGlyphs(${v}) has five and adds up`, g.length === 5 && total === v, g.join(','));
  }
  check('starsSpoken 7', starsSpoken(7) === '3.5 out of 5 stars', starsSpoken(7));
  check('starsSpoken 10', starsSpoken(10) === '5 out of 5 stars', starsSpoken(10));
  check('starsSpoken 0', starsSpoken(0) === 'not rated', starsSpoken(0));
}

// ---- dateDot -----------------------------------------------------------------------------------
{
  check('dateDot example', dateDot('2026-10-08') === `THU ${DOT} OCT 8`, dateDot('2026-10-08'));
  check('dateDot month end', dateDot('2026-10-31') === `SAT ${DOT} OCT 31`, dateDot('2026-10-31'));
  check('dateDot year end', dateDot('2026-12-31') === `THU ${DOT} DEC 31`, dateDot('2026-12-31'));
  check('dateDot year start', dateDot('2027-01-01') === `FRI ${DOT} JAN 1`, dateDot('2027-01-01'));
  check('dateDot leap day', dateDot('2028-02-29') === `TUE ${DOT} FEB 29`, dateDot('2028-02-29'));
  check('dateDot not a leap day', dateDot('2027-02-29') === '');
  check('dateDot bad input', dateDot('') === '' && dateDot('nope') === '' && dateDot('2026-13-01') === '');
  const SHORT = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'];
  const MON = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];
  // Every day from 2024 to 2030 against the built-in calendar (the script may use dates; the app may not).
  let swept = true;
  for (let t = Date.UTC(2024, 0, 1); t < Date.UTC(2031, 0, 1); t += 86400000) {
    const d = new Date(t);
    const iso = d.toISOString().slice(0, 10);
    const want = `${SHORT[d.getUTCDay()]} ${DOT} ${MON[d.getUTCMonth()]} ${d.getUTCDate()}`;
    if (dateDot(iso) !== want || dateLine(iso).replace(', ', ` ${DOT} `) !== want) {
      check(`dateDot ${iso}`, false, `${dateDot(iso)} / ${dateLine(iso)}`);
      swept = false;
      break;
    }
    checks += 1;
  }
  check('dateDot sweep 2024 to 2030 ran clean', swept);
}

// ---- photos: counts, types, sizes -----------------------------------------------------------------
{
  check('PHOTOS_MAX is 10', PHOTOS_MAX === 10);
  check('canAddPhoto 0', canAddPhoto(0) === true);
  check('canAddPhoto 9', canAddPhoto(9) === true);
  check('canAddPhoto 10', canAddPhoto(10) === false);
  check('canAddPhoto 11', canAddPhoto(11) === false);
  // 50,000,000 bytes, not 50 MiB: storage's free plan may count 50 MB that way (private photos).
  check('50 MB ceiling', PHOTO_MAX_BYTES === 50_000_000, `${PHOTO_MAX_BYTES}`);
  check('photo types', same(PHOTO_TYPES, ['image/jpeg', 'image/png', 'image/webp']));
  check('50 MB passes', photoProblem('image/jpeg', PHOTO_MAX_BYTES) === null);
  check('50 MB plus 1 is size', photoProblem('image/jpeg', PHOTO_MAX_BYTES + 1) === 'size');
  check('50 MiB is size now', photoProblem('image/jpeg', 50 * 1024 * 1024) === 'size');
  check('gif is type', photoProblem('image/gif', 1000) === 'type');
  check('png passes', photoProblem('image/png', 1000) === null);
  check('webp passes', photoProblem('image/webp', 1000) === null);
  check('no type passes as JPG (null)', photoProblem(null, 1000) === null);
  check('no type passes as JPG (undefined)', photoProblem(undefined, 1000) === null);
  check('no type and too big is size', photoProblem(null, PHOTO_MAX_BYTES + 1) === 'size');
  check('type problem wins over size', photoProblem('image/gif', PHOTO_MAX_BYTES + 1) === 'type');
  check('type message', photoMessage('type') === 'Use a JPG, PNG or WebP photo.', photoMessage('type'));
  check(
    'size message',
    photoMessage('size') === 'That photo is over 50 MB. Choose a smaller one.',
    photoMessage('size')
  );
}

// ---- reactions -----------------------------------------------------------------------------------
{
  const NAMES = ['love', 'fire', 'laughing', 'wow', 'sad', 'sleepy'];
  const LABELS = ['Love', 'Fire', 'Laughing', 'Wow', 'Sad', 'Sleepy'];
  const CODE_POINTS = [0x1f60d, 0x1f525, 0x1f602, 0x1f62e, 0x1f622, 0x1f634];
  check('exactly six reactions', REACTIONS.length === 6, `${REACTIONS.length}`);
  check('names and order', same(REACTIONS.map((r) => r.name), NAMES), REACTIONS.map((r) => r.name).join(','));
  check('labels', same(REACTIONS.map((r) => r.label), LABELS), REACTIONS.map((r) => r.label).join(','));
  check('characters are non-empty', REACTIONS.every((r) => typeof r.char === 'string' && r.char.length > 0));
  check('characters are all different', new Set(REACTIONS.map((r) => r.char)).size === 6);
  check(
    'each character is one code point (no variation selector)',
    REACTIONS.every((r) => Array.from(r.char).length === 1)
  );
  check(
    'code points',
    same(REACTIONS.map((r) => r.char.codePointAt(0)), CODE_POINTS),
    REACTIONS.map((r) => r.char.codePointAt(0).toString(16)).join(',')
  );
  check('isReactionName accepts all six', NAMES.every((n) => isReactionName(n)));
  check(
    'isReactionName refuses others',
    !isReactionName('clap') && !isReactionName('Love') && !isReactionName('') && !isReactionName(null) && !isReactionName(7)
  );

  // The server's list, read as text from internal/spin/entry.go (written by plan 03-01).
  const serverFile = fileURLToPath(new URL('../internal/spin/entry.go', import.meta.url));
  if (!existsSync(serverFile)) {
    console.log('reaction names not compared with the server list yet');
  } else {
    const text = readFileSync(serverFile, 'utf8');
    const literal = /Reactions\s*=\s*\[\]string\{([^}]*)\}/.exec(text);
    if (!literal) {
      check('server reaction list found in internal/spin/entry.go', false, 'no Reactions slice literal');
    } else {
      const serverNames = [...literal[1].matchAll(/"([^"]*)"/g)].map((m) => m[1]);
      const match = same(serverNames, REACTIONS.map((r) => r.name));
      check('reaction names match the server list', match, `server ${serverNames.join(',')} app ${NAMES.join(',')}`);
      if (match) console.log('reaction names match the server list');
    }
  }
}

// ---- rules ---------------------------------------------------------------------------------------
{
  check('limits', TITLE_MAX === 80 && COMMENT_MAX === 140);
  check('cleanTitle trims', cleanTitle('  Dune  ') === 'Dune');
  check('cleanTitle of spaces', cleanTitle('   ') === '');
  check('cleanTitle keeps inner spaces', cleanTitle(' a  b ') === 'a  b');

  check('missing all', same(missingParts({ hasPhoto: false, title: '', stars: 0 }), ['photo', 'title', 'stars']));
  check('missing none', same(missingParts({ hasPhoto: true, title: 'Dune', stars: 7 }), []));
  check('missing title is empty after trim', same(missingParts({ hasPhoto: true, title: '   ', stars: 7 }), ['title']));
  check('missing photo and stars', same(missingParts({ hasPhoto: false, title: 'x', stars: 0 }), ['photo', 'stars']));
  check('missing stars only', same(missingParts({ hasPhoto: true, title: 'x', stars: 0 }), ['stars']));
  check('one star is enough', same(missingParts({ hasPhoto: true, title: 'x', stars: 1 }), []));
  check('missing photo only', same(missingParts({ hasPhoto: false, title: 'x', stars: 10 }), ['photo']));

  check('still needed, nothing', stillNeededLine([]) === '');
  check('still needed, one', stillNeededLine(['stars']) === 'Still needed: stars.', stillNeededLine(['stars']));
  check('still needed, photo', stillNeededLine(['photo']) === 'Still needed: photo.');
  check('still needed, two', stillNeededLine(['title', 'stars']) === 'Still needed: title and stars.');
  check('still needed, photo and title', stillNeededLine(['photo', 'title']) === 'Still needed: photo and title.');
  check(
    'still needed, three',
    stillNeededLine(['photo', 'title', 'stars']) === 'Still needed: photo, title and stars.',
    stillNeededLine(['photo', 'title', 'stars'])
  );
  check(
    'still needed keeps the order photo, title, stars',
    stillNeededLine(['stars', 'photo']) === 'Still needed: photo and stars.'
  );

  const withDay = nextSpinLine('2026-10-08', '2026-10-08', 'Friday');
  check('next spin line with the weekday', withDay === `Your next spin opens Friday at 8:00${NBSP}PM.`, withDay);
  check('next spin line has a no-break space before PM', withDay.includes(`8:00${NBSP}PM.`) && !withDay.includes('8:00 PM.'));
  check('the character before PM is U+00A0', withDay.charCodeAt(withDay.indexOf('PM.') - 1) === 0xa0);
  check('the dot in dateDot is U+00B7', dateDot('2026-10-08').charCodeAt(4) === 0xb7);
  const without = nextSpinLine('2026-10-07', '2026-10-08', 'Friday');
  check('next spin line after midnight', without === `Your next spin opens at 8:00${NBSP}PM.`, without);
  check('after-midnight line has the no-break space', without.includes(`8:00${NBSP}PM.`) && !without.includes('8:00 PM.'));
  check('no weekday sent (null)', nextSpinLine('2026-10-08', '2026-10-08', null) === without);
  check('no weekday sent (undefined)', nextSpinLine('2026-10-08', '2026-10-08', undefined) === without);
  check('no weekday sent (empty)', nextSpinLine('2026-10-08', '2026-10-08', '') === without);

  check('settled label is today all day', SETTLED_LABEL === "Today's spin was", SETTLED_LABEL);
  check('board open line', BOARD_OPEN_LINE === "You're in. The Board is open.", BOARD_OPEN_LINE);
  check('missed spin line', MISSED_SPIN_LINE === 'The wheel missed you.', MISSED_SPIN_LINE);
  check('missed line', missedLine('Monday', 'Movie') === "Monday's movie got away. New night, new spin.", missedLine('Monday', 'Movie'));
  check('late line names noon and 8 PM', LATE_LINE === `Fashionably late is fine until 8:00${NBSP}PM.`, LATE_LINE);
}

// ---- lib/entry files load alone --------------------------------------------------------------------
{
  for (const name of ['rules', 'stars', 'reactions', 'photo']) {
    const file = fileURLToPath(new URL(`../lib/entry/${name}.ts`, import.meta.url));
    const text = readFileSync(file, 'utf8');
    check(`lib/entry/${name}.ts has no import`, !/^\s*import\s/m.test(text) && !/require\(/.test(text));
  }
}

// ---- upload memory (counting fake upload) ----------------------------------------------------------
async function memoryChecks(): Promise<void> {
  const wait = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));
  const counter = () => {
    const state = { calls: 0 };
    const upload = (name: string, ms = 0) => async () => {
      state.calls += 1;
      const mine = state.calls;
      if (ms > 0) await wait(ms);
      return `key-${name}-${mine}`;
    };
    return { state, upload };
  };

  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const a = await memory.ensure('file:///a.jpg', upload('a'));
    const again = await memory.ensure('file:///a.jpg', upload('a'));
    check('same uri twice uploads once', state.calls === 1 && a === again, `calls ${state.calls}, ${a} / ${again}`);
    const b = await memory.ensure('file:///b.jpg', upload('b'));
    check('a second uri uploads again', state.calls === 2 && b !== a, `calls ${state.calls}`);
    check('the first key is still remembered', (await memory.ensure('file:///a.jpg', upload('a'))) === a && state.calls === 2);
  }
  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const [x, y] = await Promise.all([
      memory.ensure('file:///s.jpg', upload('s', 15)),
      memory.ensure('file:///s.jpg', upload('s', 15)),
    ]);
    check('two simultaneous calls upload once', state.calls === 1 && x === y, `calls ${state.calls}, ${x} / ${y}`);
  }
  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    let failed = false;
    try {
      await memory.ensure('file:///t.jpg', async () => {
        state.calls += 1;
        throw new Error('network');
      });
    } catch {
      failed = true;
    }
    check('a throwing upload rejects', failed && state.calls === 1);
    const key = await memory.ensure('file:///t.jpg', upload('t'));
    check('a throwing upload is called again on the next try', state.calls === 2 && key === 'key-t-2', `calls ${state.calls}, ${key}`);
    check('after the retry the key is remembered', (await memory.ensure('file:///t.jpg', upload('t'))) === key && state.calls === 2);
  }
  {
    // A function that throws before it returns a promise still becomes a rejection, and is not remembered.
    const memory = createUploadMemory();
    let rejected = false;
    try {
      await memory.ensure('file:///u.jpg', () => {
        throw new Error('sync');
      });
    } catch {
      rejected = true;
    }
    const { state, upload } = counter();
    const key = await memory.ensure('file:///u.jpg', upload('u'));
    check('a synchronous throw is a rejection and is not remembered', rejected && state.calls === 1 && key === 'key-u-1');
  }
  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const first = await memory.ensure('file:///f.jpg', upload('f'));
    memory.forget();
    const second = await memory.ensure('file:///f.jpg', upload('f'));
    check('forget() makes the same uri upload again', state.calls === 2 && first !== second, `calls ${state.calls}`);
  }
  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const a = await memory.ensure('file:///1.jpg', upload('1'));
    const b = await memory.ensure('file:///2.jpg', upload('2'));
    memory.forget('file:///1.jpg');
    const a2 = await memory.ensure('file:///1.jpg', upload('1'));
    const b2 = await memory.ensure('file:///2.jpg', upload('2'));
    check('forget(uri) drops only that file', state.calls === 3 && a2 !== a && b2 === b, `calls ${state.calls}`);
  }
  {
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const keys: string[] = [];
    for (const uri of ['file:///x.jpg', 'file:///y.jpg', 'file:///z.jpg']) {
      keys.push(await memory.ensure(uri, upload(uri.slice(-5, -4))));
    }
    check('three uris upload three times', state.calls === 3 && new Set(keys).size === 3, `calls ${state.calls}`);
    const again: string[] = [];
    for (const uri of ['file:///x.jpg', 'file:///y.jpg', 'file:///z.jpg']) {
      again.push(await memory.ensure(uri, upload('never')));
    }
    check('each uri keeps its own key and the order', state.calls === 3 && same(keys, again), `calls ${state.calls}`);
  }
  {
    // The retry rule end to end: a post fails after two uploads, the retry reuses both keys in order.
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const photos = ['file:///p1.jpg', 'file:///p2.jpg'];
    const attempt = async () => {
      const keys: string[] = [];
      for (const uri of photos) keys.push(await memory.ensure(uri, upload(uri.slice(-6, -4))));
      return keys;
    };
    const first = await attempt();
    const retry = await attempt();
    check('a retry sends the same keys in the same order', state.calls === 2 && same(first, retry), `calls ${state.calls}`);
  }
  {
    // forget(uri) while an upload is still running: the old attempt does not overwrite a newer one.
    const { state, upload } = counter();
    const memory = createUploadMemory();
    const slow = memory.ensure('file:///g.jpg', upload('g', 20));
    memory.forget('file:///g.jpg');
    const fresh = await memory.ensure('file:///g.jpg', upload('g'));
    await slow;
    const remembered = await memory.ensure('file:///g.jpg', upload('g'));
    check('forget during a running upload keeps the newer attempt', state.calls === 2 && remembered === fresh, `calls ${state.calls}`);
  }
}

await memoryChecks();

// ---- plan 03-03: entry id, skip sheet text, edit patch -----------------------------------------
{
  const good = '0b6f1c1e-52a4-4a0e-9d0c-1f7a3e5b9c2d';
  check('a UUID is an entry id', isEntryId(good));
  check('a UUID in capitals is an entry id', isEntryId(good.toUpperCase()));
  check('an empty string is not an entry id', !isEntryId(''));
  check('a short id is not an entry id', !isEntryId('123'));
  check('an id with a path in it is not an entry id', !isEntryId(`${good}/delete`));
  check('a number is not an entry id', !isEntryId(7));
  check('undefined is not an entry id', !isEntryId(undefined));
  check('a 36 character non-UUID is not an entry id', !isEntryId('zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz'));

  check(
    'skip text names the count',
    skipBody('2026-10-08', '2026-10-05', 2, 4) ===
      "You are at 2 of 4 this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards."
  );
  check(
    'skip text for the day before the shown week leaves the count out',
    !skipBody('2026-10-04', '2026-10-05', 0, 4).includes('this week.') &&
      skipBody('2026-10-04', '2026-10-05', 0, 4).startsWith("A skipped day doesn't count")
  );
  check('skip text on the first day of the week keeps the count', skipBody('2026-10-05', '2026-10-05', 0, 4).startsWith('You are at 0 of 4'));

  const was = { title: 'Ramen', stars: 9, reaction: 'love', comment: 'Warm.' };
  check('nothing changed gives an empty patch', same(changedFields(was, { ...was, comment: 'Warm.' }), {}));
  check('spaces around the title are not a change', same(changedFields(was, { ...was, title: '  Ramen ', comment: 'Warm.' }), {}));
  check('a new title is sent trimmed', same(changedFields(was, { ...was, title: ' Pho ', comment: 'Warm.' }), { title: 'Pho' }));
  check('new stars are sent', same(changedFields(was, { ...was, stars: 7, comment: 'Warm.' }), { stars: 7 }));
  check('a cleared reaction is the empty string', same(changedFields(was, { ...was, reaction: null, comment: 'Warm.' }), { reaction: '' }));
  check('a new reaction is sent by name', same(changedFields(was, { ...was, reaction: 'fire', comment: 'Warm.' }), { reaction: 'fire' }));
  check('a cleared comment is the empty string', same(changedFields(was, { ...was, comment: '  ' }), { comment: '' }));
  check(
    'a comment added to an entry with none is sent',
    same(changedFields({ ...was, comment: null }, { ...was, comment: 'Nice' }), { comment: 'Nice' })
  );
  check(
    'no comment on both sides is no change',
    same(changedFields({ ...was, comment: null, reaction: null }, { ...was, comment: '', reaction: null }), {})
  );
}

// ---- private photos: a signed link's cache key, and the edit's photo list (2026-11-09) -------------
{
  const own = readFileSync(fileURLToPath(new URL('../lib/api/photoSource.ts', import.meta.url)), 'utf8');
  check('lib/api/photoSource.ts has no import', !/^\s*import\s/m.test(own) && !/require\(/.test(own));

  // A link as the server signs it (an S3 presigned GET): the storage path, then the signature's query.
  const path = 'https://abcd.supabase.co/storage/v1/s3/photos/entries/0b6f1c1e-52a4-4a0e-9d0c-1f7a3e5b9c2d/7f3a9c2e4b1d6a8f.jpg';
  const path2 = path.replace('7f3a9c2e4b1d6a8f', '0011223344556677');
  const signed = (at: string, date: string, signature: string) =>
    `${at}?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=key%2F20261109%2Fap-southeast-1%2Fs3%2Faws4_request` +
    `&X-Amz-Date=${date}&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=${signature}`;
  const first = signed(path, '20261109T080000Z', 'a1b2');
  const next = signed(path, '20261109T083000Z', 'c3d4');
  check('a signed link loads from the whole link', photoSource(first).uri === first);
  check('a signed link is saved under its path, the query gone', photoSource(first).cacheKey === path, photoSource(first).cacheKey);
  check(
    "the next half hour's link keeps the cache key",
    next !== first && photoSource(next).cacheKey === photoSource(first).cacheKey
  );
  check('another photo has another cache key', photoSource(signed(path2, '20261109T080000Z', 'a1b2')).cacheKey === path2);
  check('a link with no query is its own cache key', same(photoSource(path), { uri: path, cacheKey: path }));
  const minio = 'http://127.0.0.1:9000/rndmroll-avatars/avatars/0b6f1c1e-52a4-4a0e-9d0c-1f7a3e5b9c2d/7f3a9c2e4b1d6a8f.jpg';
  check('a local storage link (http) gets the same key', photoSource(signed(minio, '20261109T080000Z', 'e5f6')).cacheKey === minio);
  check('the scheme is read in any case', photoSource('HTTPS://x.test/a.jpg?s=1').cacheKey === 'HTTPS://x.test/a.jpg');
  check(
    'only the query goes: the path stays as sent',
    photoSource('https://x.test/a%2Fb/c%20d.jpg?s=1&t=2').cacheKey === 'https://x.test/a%2Fb/c%20d.jpg'
  );
  for (const uri of [
    'file:///var/mobile/Containers/Data/Application/A1/Library/Caches/ImagePicker/B2.jpg',
    'file:///tmp/photo.jpg?v=1',
    'ph://CC95F08C-88C3-4012-9D6D-64A413D254B3/L0/001',
  ]) {
    const source = photoSource(uri);
    check(`a local file loads as it is: ${uri}`, same(source, { uri }) && !('cacheKey' in source), JSON.stringify(source));
  }
  check(
    'data is read again before its links can run out (20 minutes, under 60 minus 30)',
    STALE_AFTER_MS === 20 * 60 * 1000 && STALE_AFTER_MS < (60 - 30) * 60 * 1000
  );
  // isStale: the one rule for the return from the background, a photo that will not load and
  // Search coming back into focus. Nothing before the first answer, nothing up to 20 minutes.
  const loadedAt = Date.UTC(2026, 10, 13, 8, 5);
  check('nothing is stale before the first answer', !isStale(null, loadedAt + 60 * 60 * 1000));
  check('an answer just in is not stale', !isStale(loadedAt, loadedAt));
  check('an answer 19 minutes old is not stale', !isStale(loadedAt, loadedAt + 19 * 60 * 1000));
  check('an answer exactly 20 minutes old is not stale yet', !isStale(loadedAt, loadedAt + STALE_AFTER_MS));
  check('an answer over 20 minutes old is stale', isStale(loadedAt, loadedAt + STALE_AFTER_MS + 1));
  check('an answer an hour old is stale', isStale(loadedAt, loadedAt + 60 * 60 * 1000));

  // An entry as read when its edit opened, and read again half an hour later (the page came back
  // into focus): the same photos, new links. The server sends them in any order with a position.
  const read = (date: string) => [
    { id: 'p2', url: signed(path2, date, 'b2'), position: 1 },
    { id: 'p1', url: signed(path, date, 'a1'), position: 0 },
  ];
  const inOrder = (photos) => [...photos].sort((a, b) => a.position - b.position);
  const opened = inOrder(read('20261109T080000Z'));
  const later = inOrder(read('20261109T083000Z'));
  // The draft as useEntryDraft seeds it: each photo's id, and the link it came with.
  const draft = opened.map((photo) => ({ uri: photo.url, mimeType: null, source: 'library', id: photo.id }));
  const pick = { uri: 'file:///tmp/new.jpg', mimeType: 'image/jpeg', source: 'camera' };
  check(
    'an edit that left the photos alone sends no photo list when only the links changed',
    !photoListChanged(draft, later) && draft.every((photo, i) => photo.uri !== later[i].url)
  );
  check('the draft matches the read it was seeded from', !photoListChanged(draft, opened));
  check('a removed photo sends the list', photoListChanged(draft.slice(1), later));
  check('a new order sends the list', photoListChanged([draft[1], draft[0]], later));
  check('a new pick sends the list', photoListChanged([...draft, pick], later));
  check('a pick in place of a photo sends the list', photoListChanged([draft[0], pick], later));
  check('a new entry with no photo is no change', !photoListChanged([], []));
  check('a pick on a new entry is a change', photoListChanged([pick], []));

  // The edit page uses it: the save compares ids (it reads no link), and the draft keeps each id.
  const source = (rel: string) => readFileSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), 'utf8');
  const save = source('components/entry/useUpdateEntry.ts');
  check('the edit save compares the photos by id', /photoListChanged\(\s*draft\.photos/.test(save) && !/\.url\b/.test(save));
  check('the edit draft keeps each photo id', /\bid:\s*photo\.id\b/.test(source('components/entry/useEntryDraft.ts')));

  // Every expo-image source goes through photoSource, so none is missed: no bare `source={{` left.
  const bare: string[] = [];
  const walk = (rel: string) => {
    for (const item of readdirSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), { withFileTypes: true })) {
      const child = `${rel}/${item.name}`;
      if (item.isDirectory()) walk(child);
      else if (/\.tsx?$/.test(item.name) && /source=\{\{/.test(source(child))) bare.push(child);
    }
  };
  walk('app');
  walk('components');
  check('every photo source goes through photoSource', bare.length === 0, bare.join(', '));

  // The wiring around photos that will not load. These are hooks and screens, and this script has
  // no React to run them, so the lines that matter are pinned as written.
  const photoReload = source('lib/api/usePhotoReload.ts');
  check(
    'a photo that will not load reads the screen again only once its data is stale, then at most once a minute',
    /if \(!staleRef\.current\(\)\) return;\s*const now = Date\.now\(\);\s*if \(now - lastRef\.current < RELOAD_GAP_MS\) return;/.test(photoReload)
  );
  const foreground = source('lib/api/useForegroundReload.ts');
  check(
    'the return from the background and stale use the same rule',
    /focusedRef\.current && isStale\(loadedAtRef\.current, Date\.now\(\)\)/.test(foreground) &&
      /stale = useCallback\(\(\) => isStale\(loadedAtRef\.current, Date\.now\(\)\)/.test(foreground)
  );
  check(
    'Search reads again when it comes back into focus with stale data',
    /useFocusEffect\(\s*useCallback\(\(\) => \{\s*if \(stale\(\)\) setRound\(\(n\) => n \+ 1\);/.test(source('components/people/SearchScreen.tsx'))
  );
  check(
    "a person's next page is added only to the list it was asked for",
    /const cursor = page\.next_cursor;[\s\S]*getPerson\(id, cursor\)[\s\S]*now === null \|\| now\.next_cursor !== cursor/.test(
      source('components/people/PersonScreen.tsx')
    )
  );
  const blocked = source('components/settings/BlockedAccountsScreen.tsx');
  check(
    'Blocked accounts: a read begun before an Unblock that went through is dropped',
    /await unblock\(userId\);\s*seqRef\.current \+= 1;/.test(blocked) &&
      (blocked.match(/if \(seq !== seqRef\.current\) return;/g) ?? []).length === 2
  );
  // A failed photo remembers the answer it came in, not its link: the next answer tries it again
  // even when the link is the same (it stays the same for up to half an hour).
  check('a Board card tries its photo again on the next answer', /card !== failedCard/.test(source('components/board/PostCard.tsx')));
  check('the gallery tries a photo again on the next answer', /failed\[photo\.id\] === photo \?/.test(source('components/entry/EntryView.tsx')));
  check(
    "a notification's picture tries again on the next answer",
    /item !== failedItem/.test(source('components/today/NotificationsScreen.tsx'))
  );
}

// ---- the grey Save, photo access and the server's refusals (2026-11-16) ----------------------------
{
  const source = (rel: string) => readFileSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), 'utf8');

  // A tap on the grey Save scrolls to missing[0] and says the Still needed line: that part must be
  // the line's first word, for every mix of missing parts.
  const parts = ['photo', 'title', 'stars'];
  for (let mask = 1; mask < 8; mask += 1) {
    const has = { hasPhoto: !(mask & 1), title: mask & 2 ? '' : 'Dune', stars: mask & 4 ? 0 : 7 };
    const missing = missingParts(has);
    const line = stillNeededLine(missing);
    check(
      `the first missing part leads the line: ${missing.join(',')}`,
      missing.length > 0 && line.startsWith(`Still needed: ${missing[0]}`) && parts.includes(missing[0]),
      line
    );
  }

  const form = source('components/entry/EntryForm.tsx');
  check(
    'the Save button is grey as before',
    /const canSubmit = !submitting && draft\.missing\.length === 0 && \(mode === 'new' \|\| draft\.dirty\);/.test(form) &&
      /disabled=\{terminal \? false : !canSubmit\}/.test(form)
  );
  check(
    'a tap on the grey Save is taken only while a part is missing (not for an edit with no change, not while saving)',
    /const blocked = !terminal && !submitting && draft\.missing\.length > 0;/.test(form) &&
      /onBlockedPress=\{blocked \? showMissing : undefined\}/.test(form)
  );
  check('its VoiceOver hint is the Still needed line, only while blocked', /blockedHint=\{blocked \? needed : undefined\}/.test(form));
  check(
    'the tap brings the first missing part into view and says the Still needed line',
    /const first = draft\.missing\[0\];\s*const part = first === 'photo' \? photoRef\.current : first === 'title' \? titleRef\.current : starsRef\.current;\s*announceProblem\(needed\);\s*bringIntoView\(scrollRef\.current, part\);/.test(
      form
    )
  );
  check(
    'the three parts carry their refs',
    /<View ref=\{photoRef\}>\s*<PhotoField/.test(form) &&
      /<View ref=\{titleRef\} style=\{\{ marginTop: space\.lg \}\}>\s*<UnderlineField\s+label="Title"/.test(form) &&
      /<View ref=\{starsRef\} style=\{\{ marginTop: space\.lg \}\}>[\s\S]{0,400}<CapsLabel>Rating<\/CapsLabel>/.test(form)
  );
  check(
    'the Still needed line stays, muted, above the button',
    /\{needed !== '' && !terminal \? \(/.test(form) && /<Text style=\{\{ \.\.\.labelLine, color: color\.muted \}\}>\{needed\}<\/Text>/.test(form)
  );

  const photos = source('components/entry/PhotoField.tsx');
  check(
    'photo access turned off opens the alert with Open Settings, camera and library',
    /requestCameraPermissionsAsync\(\);\s*if \(!permission\.granted\) \{\s*showAccessOff\('camera'\);\s*return;\s*\}/.test(photos) &&
      /requestMediaLibraryPermissionsAsync\(\);\s*if \(!permission\.granted\) \{\s*showAccessOff\('library'\);\s*return;\s*\}/.test(photos) &&
      /import \{ showAccessOff \} from '@\/lib\/media\/showAccessOff';/.test(photos)
  );
  check('the old access lines are gone from the photo field', !/access is needed/.test(photos));

  const entries = source('lib/api/entries.ts');
  check('the 429 line', /const RATE_LIMITED_LINE = 'Too many tries\. Wait a minute\.';/.test(entries));
  check('the character line', /const CHARACTER_LINE = "That has a character we can't use\.";/.test(entries));
  check(
    'a 429 says to wait, and the form stays (not terminal)',
    /if \(err\.code === 'rate_limited'\) return \{ text: RATE_LIMITED_LINE, terminal: false \};/.test(entries) &&
      !/clientText\('rate_limited'\)/.test(entries)
  );
  check(
    'only a title or a comment refused as invalid gets the character line (photos and the rest stay generic)',
    /if \(err\.code === 'validation_failed' && \(err\.field === 'title' \|\| err\.field === 'comment'\) && err\.reason === 'invalid'\) \{\s*return \{ text: CHARACTER_LINE, terminal: false \};\s*\}/.test(
      entries
    ) && (entries.match(/validation_failed' &&/g) ?? []).length === 1
  );
  check(
    'the terminal answers still come first, the generic line last',
    /const terminal = TERMINAL_MESSAGES\[err\.code\];\s*if \(terminal !== undefined\) return \{ text: terminal, terminal: true \};/.test(entries) &&
      /\n  \}\n  return \{ text: clientText\('server_error'\), terminal: false \};\n\}/.test(entries)
  );
  // The server's words, read as text: the reason a control character gets, and the two text fields.
  const serverFile = fileURLToPath(new URL('../internal/spin/entry.go', import.meta.url));
  if (existsSync(serverFile)) {
    const go = readFileSync(serverFile, 'utf8');
    check(
      'the server refuses a control character in a title or comment as invalid',
      /if hasControl\(title, false\) \{\s*return "", ProblemInvalid/.test(go) && /if hasControl\(text, true\) \{\s*return "", ProblemInvalid/.test(go)
    );
    check('the server names the fields title and comment', /fieldTitle\s*=\s*"title"/.test(go) && /fieldComment\s*=\s*"comment"/.test(go));
    const reasons = readFileSync(fileURLToPath(new URL('../internal/spin/entry_errors.go', import.meta.url)), 'utf8');
    check('the server spells that reason invalid', /ProblemInvalid\s*=\s*"invalid"/.test(reasons));
    const respond = readFileSync(fileURLToPath(new URL('../internal/httpapi/errors.go', import.meta.url)), 'utf8');
    check('the server sends the field and the reason', /"field":\s*field,[\s\S]{0,80}"reason":\s*reason,/.test(respond));
  }
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
