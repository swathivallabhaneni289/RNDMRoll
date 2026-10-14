// @ts-nocheck
/**
 * Throwaway checks for the pure entry logic (plan 03-02 Task 1). Not part of the app bundle.
 * Run with:  node --experimental-strip-types scripts/check-entry.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 * This file holds no emoji: the reactions are checked by code point.
 */
import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { REACTIONS, isReactionName } from '../lib/entry/reactions.ts';
import {
  COMMENT_MAX,
  TITLE_MAX,
  changedFields,
  cleanTitle,
  isEntryId,
  missingParts,
  nextSpinLine,
  settledLabel,
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
  check('50 MiB ceiling', PHOTO_MAX_BYTES === 52428800, `${PHOTO_MAX_BYTES}`);
  check('photo types', same(PHOTO_TYPES, ['image/jpeg', 'image/png', 'image/webp']));
  check('50 MiB passes', photoProblem('image/jpeg', PHOTO_MAX_BYTES) === null);
  check('50 MiB plus 1 is size', photoProblem('image/jpeg', PHOTO_MAX_BYTES + 1) === 'size');
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
  check('next spin line for last night', without === `Your next spin opens at 8:00${NBSP}PM.`, without);
  check('last night line has the no-break space', without.includes(`8:00${NBSP}PM.`) && !without.includes('8:00 PM.'));
  check('no weekday sent (null)', nextSpinLine('2026-10-08', '2026-10-08', null) === without);
  check('no weekday sent (undefined)', nextSpinLine('2026-10-08', '2026-10-08', undefined) === without);
  check('no weekday sent (empty)', nextSpinLine('2026-10-08', '2026-10-08', '') === without);

  check('settled label today', settledLabel('2026-10-08', '2026-10-08') === "Today's spin was", settledLabel('2026-10-08', '2026-10-08'));
  check('settled label last night', settledLabel('2026-10-07', '2026-10-08') === "Last night's spin was");
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

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
