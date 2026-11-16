// @ts-nocheck
/**
 * Throwaway checks for the pure wheel logic (plan 02-02 Task 1). Not part of the app bundle.
 * Run with:  node --experimental-strip-types scripts/check-wheel.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is because Node needs the .ts extension on imports and tsc, which scans
 * every .ts file in the repo, would object to it.)
 */
import {
  acrossRoom,
  alongRoom,
  drawnAngles,
  labelCapacity,
  labelColorFor,
  labelLine,
  labelPlacement,
  MIN_WEDGE_DEGREES,
  poseOf,
  targetRotation,
  toneIndexes,
  truncate,
  wedgeCentres,
  wedgePath,
  wedgeStarts,
  wedgeUnderPointer,
} from '../lib/wheel/geometry.ts';
import { msUntil, nextWaitMs, parseInstant, spokenMinutes, split } from '../lib/wheel/countdown.ts';
import { dateLine, dateLineLong } from '../lib/wheel/format.ts';
import { NAME_EMPTY_LINE, NAME_TAKEN_LINE, nameProblems } from '../lib/wheel/categoryNames.ts';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

let checks = 0;
let failures = 0;

function check(name: string, ok: boolean, detail?: string): void {
  checks += 1;
  if (!ok) {
    failures += 1;
    console.log(`FAIL ${name}${detail ? `: ${detail}` : ''}`);
  }
}

// A small deterministic generator so a failure can be reproduced.
let seed = 20261010;
function rand(): number {
  seed = (seed * 1664525 + 1013904223) % 4294967296;
  return seed / 4294967296;
}
function randInt(lo: number, hi: number): number {
  return lo + Math.floor(rand() * (hi - lo + 1));
}

// ---- drawn angles ---------------------------------------------------------------------------
for (let n = 2; n <= 12; n += 1) {
  for (let trial = 0; trial < 400; trial += 1) {
    const weights = Array.from({ length: n }, () => randInt(1, 10));
    // The heaviest case for the floor: one 10 and the rest 1.
    if (trial === 0) weights.fill(1), (weights[0] = 10);
    const angles = drawnAngles(weights);
    const sum = angles.reduce((a, b) => a + b, 0);
    check(`angles sum n=${n}`, Math.abs(sum - 360) < 1e-6, `sum ${sum} for ${weights}`);
    check(
      `angles floor n=${n}`,
      angles.every((a) => a >= MIN_WEDGE_DEGREES - 1e-6),
      `${angles.map((a) => a.toFixed(2))} for ${weights}`
    );
    // Every wedge is the same size, whatever the weights.
    check(`angles are equal n=${n}`, angles.every((a) => Math.abs(a - 360 / n) < 1e-9), `${angles} for ${weights}`);
  }
}
{
  // The starting wheel: Meal 3, Movie 1, Song 3, Book 1, Place 2, Game 1 -> six equal wedges of 60 degrees.
  const a = drawnAngles([3, 1, 3, 1, 2, 1]).map((x) => Math.round(x));
  check('starting wheel angles', a.join(',') === '60,60,60,60,60,60', a.join(','));
}

// ---- the pointer ends on the server's winner -------------------------------------------------
for (let n = 2; n <= 12; n += 1) {
  for (let trial = 0; trial < 60; trial += 1) {
    const weights = Array.from({ length: n }, () => randInt(1, 10));
    const angles = drawnAngles(weights);
    const centres = wedgeCentres(angles);
    let rotation = [0, -49.3, 1234.5, -2000, 359.9, 7200.25][trial % 6];
    for (let winner = 0; winner < n; winner += 1) {
      const target = targetRotation(rotation, poseOf(centres[winner]));
      check(`turn is at least five turns n=${n} w=${winner}`, target - rotation >= 1800 - 1e-9, `${target - rotation}`);
      check(`turn is clockwise n=${n}`, target > rotation);
      const under = wedgeUnderPointer(target, angles);
      check(`pointer on winner n=${n} w=${winner}`, under === winner, `got ${under} weights ${weights}`);
      rotation = target;
    }
  }
}

// ---- wedge path large arc flag ---------------------------------------------------------------
{
  const big = wedgePath(0, 327, 100);
  const small = wedgePath(0, 33, 100);
  const half = wedgePath(0, 180, 100);
  check('large arc flag on a wedge over half a turn', / 0 1 1 /.test(big), big);
  check('small arc flag on a thin wedge', / 0 0 1 /.test(small), small);
  check('exactly half a turn uses the small flag', / 0 0 1 /.test(half), half);
  const angles = drawnAngles([10, 1]);
  check('weights 10 and 1 still draw two equal halves', Math.abs(angles[0] - 180) < 1e-9 && Math.abs(angles[1] - 180) < 1e-9, `${angles}`);
}

// ---- tones -----------------------------------------------------------------------------------
// Tones alternate by index for every count: even index dark, odd index light, and there is no
// third tone. With an odd count the last wedge is dark and meets the first (also dark); the
// component's seam keeps them apart, so what is checked here is that the last wedge is dark and
// as big as the first.
for (let n = 2; n <= 12; n += 1) {
  const tones = toneIndexes(n);
  check(`tones length n=${n}`, tones.length === n);
  check(`first wedge is dark n=${n}`, tones[0] === 'dark');
  check(`only dark and light n=${n}`, tones.every((t) => t === 'dark' || t === 'light'), tones.join(','));
  check(`no mid tone n=${n}`, !tones.includes('mid'));
  tones.forEach((tone, i) => {
    check(`tone alternates by index n=${n} i=${i}`, tone === (i % 2 === 0 ? 'dark' : 'light'), tones.join(','));
  });
  // Neighbours differ everywhere except across the wrap of an odd count.
  for (let i = 0; i < n - 1; i += 1) {
    check(`no equal neighbours n=${n} i=${i}`, tones[i] !== tones[i + 1], tones.join(','));
  }
  if (n % 2 === 1) {
    const angles = drawnAngles(Array.from({ length: n }, () => 1));
    check(`odd count: last wedge is dark n=${n}`, tones[n - 1] === 'dark');
    check(`odd count: last wedge is as big as the first n=${n}`, Math.abs(angles[n - 1] - angles[0]) < 1e-9, `${angles}`);
    check(`odd count: the wrap joins two dark wedges n=${n}`, tones[n - 1] === tones[0]);
  } else {
    check(`even count: last wedge is light n=${n}`, tones[n - 1] === 'light');
    check(`even count: the wrap differs n=${n}`, tones[n - 1] !== tones[0]);
  }
}
check('label colour on dark', labelColorFor('dark') === 'dominant');
check('label colour on light', labelColorFor('light') === 'ink');

// ---- label placement -------------------------------------------------------------------------
{
  const names = ['Meal', 'Movie', 'Song', 'A', 'Concert', 'Board games', 'A very long category', 'Twenty-four characters!!', 'Café', ''];
  const radii = [116, 100, 120];
  let across = 0;
  let along = 0;
  for (let trial = 0; trial < 6000; trial += 1) {
    const n = randInt(2, 12);
    const weights = Array.from({ length: n }, () => randInt(1, 10));
    const angles = drawnAngles(weights);
    const centres = wedgeCentres(angles);
    const i = randInt(0, n - 1);
    const name = names[randInt(0, names.length - 1)];
    const radius = radii[randInt(0, radii.length - 1)];
    const finalRotation = poseOf(centres[randInt(0, n - 1)]) + 360 * randInt(0, 12);
    const placement = labelPlacement(centres[i], angles[i], name, radius, finalRotation);
    // Independent check of "never upside down in the pose the wheel stops in".
    const baseline = (placement.direction === 'across' ? centres[i] + 90 : centres[i]) + (placement.flip ? 180 : 0);
    const horizontal = Math.sin(((baseline + finalRotation) * Math.PI) / 180);
    check('label never upside down', horizontal >= -1e-9, `${placement.direction} flip=${placement.flip} h=${horizontal}`);
    // Along whenever across would not fit.
    const need = Array.from(name.trim()).length * 8.3 + 4;
    const fits = need <= acrossRoom(angles[i], radius);
    check('along whenever across would not fit', fits ? placement.direction === 'across' : placement.direction === 'along');
    if (placement.direction === 'across') across += 1;
    else along += 1;
    // The line shown never exceeds the room of its direction.
    const line = labelLine(name, placement.direction, angles[i], radius);
    check('line within capacity', Array.from(line).length <= labelCapacity(placement.direction, angles[i], radius));
  }
  check('both directions occur', across > 0 && along > 0, `across ${across} along ${along}`);
  // The wide-wedge cap: the words never reach out of the rim.
  check('across room is capped by the rim', acrossRoom(200, 116) <= 2 * Math.sqrt(114.5 * 114.5 - 81.2 * 81.2) + 1e-6);
  check('along room at the largest wheel', Math.abs(alongRoom(116) - 76.26) < 0.2, `${alongRoom(116)}`);
  console.log(`info: along capacity at the largest wheel = ${labelCapacity('along', 60, 116)} letters`);
}

// ---- truncate ----------------------------------------------------------------------------------
check('truncate keeps a short name', truncate('Meal', 9) === 'Meal');
check('truncate cuts with an ellipsis', truncate('Concert tickets', 8) === 'Concert…', truncate('Concert tickets', 8));
check('truncate to one', truncate('Meal', 1) === '…');
check('truncate counts letters not bytes', truncate('Café au lait', 4) === 'Caf…', truncate('Café au lait', 4));

// ---- date lines ------------------------------------------------------------------------------
{
  const SHORT = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'];
  const LONG = ['SUNDAY', 'MONDAY', 'TUESDAY', 'WEDNESDAY', 'THURSDAY', 'FRIDAY', 'SATURDAY'];
  const MON = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];
  const MONLONG = ['JANUARY', 'FEBRUARY', 'MARCH', 'APRIL', 'MAY', 'JUNE', 'JULY', 'AUGUST', 'SEPTEMBER', 'OCTOBER', 'NOVEMBER', 'DECEMBER'];
  check('dateLine example', dateLine('2026-10-08') === 'THU, OCT 8', dateLine('2026-10-08'));
  check('dateLineLong example', dateLineLong('2026-10-08') === 'THURSDAY, OCTOBER 8', dateLineLong('2026-10-08'));
  check('month end', dateLineLong('2026-10-31') === 'SATURDAY, OCTOBER 31', dateLineLong('2026-10-31'));
  check('year end', dateLineLong('2026-12-31') === 'THURSDAY, DECEMBER 31', dateLineLong('2026-12-31'));
  check('year start', dateLine('2027-01-01') === 'FRI, JAN 1', dateLine('2027-01-01'));
  check('leap day', dateLine('2028-02-29') === 'TUE, FEB 29', dateLine('2028-02-29'));
  check('not a leap day', dateLine('2027-02-29') === '');
  check('bad input', dateLine('2026-13-01') === '' && dateLineLong('nope') === '');
  // Every day from 2024 to 2030 against the built-in calendar (the script may use dates; the app may not).
  for (let t = Date.UTC(2024, 0, 1); t < Date.UTC(2031, 0, 1); t += 86400000) {
    const d = new Date(t);
    const iso = d.toISOString().slice(0, 10);
    const want = `${SHORT[d.getUTCDay()]}, ${MON[d.getUTCMonth()]} ${d.getUTCDate()}`;
    const wantLong = `${LONG[d.getUTCDay()]}, ${MONLONG[d.getUTCMonth()]} ${d.getUTCDate()}`;
    if (dateLine(iso) !== want || dateLineLong(iso) !== wantLong) {
      check(`date line ${iso}`, false, `${dateLine(iso)} / ${dateLineLong(iso)}`);
      break;
    }
    checks += 1;
  }
}

// ---- countdown -------------------------------------------------------------------------------
{
  const s = (h: number, m: number, sec: number) => ((h * 60 + m) * 60 + sec) * 1000;
  const p = (ms: number) => {
    const x = split(ms);
    return `${x.hours}:${x.minutes}:${x.seconds}`;
  };
  check('split 5:04:01', p(s(5, 4, 1)) === '05:04:01', p(s(5, 4, 1)));
  check('split a part second counts up', p(s(0, 0, 1) - 400) === '00:00:01', p(s(0, 0, 1) - 400));
  check('split zero', p(0) === '00:00:00');
  check('split negative', p(-5000) === '00:00:00');
  check('split hours stop at 99', p(s(130, 0, 0)) === '99:00:00');
  check('split rolls the minute', p(s(0, 59, 59) + 500) === '01:00:00', p(s(0, 59, 59) + 500));
  check('spoken 5 hours 4 minutes', spokenMinutes(s(5, 4, 30)) === '5 hours 4 minutes', spokenMinutes(s(5, 4, 30)));
  check('spoken one hour', spokenMinutes(s(1, 0, 10)) === '1 hour', spokenMinutes(s(1, 0, 10)));
  check('spoken one minute', spokenMinutes(s(0, 1, 5)) === '1 minute');
  check('spoken under a minute', spokenMinutes(30000) === 'less than a minute');

  const samples = [
    '2026-10-10T20:00:00Z',
    '2026-10-10T20:00:00.5Z',
    '2026-10-10T19:12:33.123456789Z',
    '2026-10-10T20:00:00+05:30',
    '2026-10-10T20:00:00-07:00',
    '2026-02-28T23:59:59Z',
    '2028-02-29T00:00:00Z',
    '1999-12-31T23:59:59.999Z',
    '2026-12-31T23:59:59Z',
    '2027-01-01T00:00:00Z',
  ];
  for (const text of samples) {
    const mine = parseInstant(text);
    const built = Date.parse(text.replace(/\.(\d{3})\d+/, '.$1'));
    check(`parseInstant ${text}`, mine === built, `${mine} vs ${built}`);
  }
  check('parseInstant rejects junk', parseInstant('tomorrow') === null && parseInstant(null) === null);

  const anchor = { serverMs: parseInstant('2026-10-10T14:55:59Z'), mark: 1000 };
  const left = msUntil('2026-10-10T20:00:00Z', anchor, 1000 + 2000);
  check('msUntil counts from the anchor', left === parseInstant('2026-10-10T20:00:00Z') - (anchor.serverMs + 2000), `${left}`);
  const wait = nextWaitMs(['2026-10-10T20:00:00Z', '2026-10-11T00:00:00Z', '2026-10-10T10:00:00Z'], anchor, 1000);
  check('nextWaitMs picks the nearest ahead', wait === parseInstant('2026-10-10T20:00:00Z') - anchor.serverMs, `${wait}`);
  check('nextWaitMs with nothing ahead', nextWaitMs(['2026-10-10T10:00:00Z'], anchor, 1000) === null);
}

// ---- Edit wheel: category names and the grey Save wheel (2026-11-16) -------------------------------
{
  check('empty name line', NAME_EMPTY_LINE === 'Name it or remove it.', NAME_EMPTY_LINE);
  check('repeated name line', NAME_TAKEN_LINE === 'Already on the wheel.', NAME_TAKEN_LINE);
  for (const line of [NAME_EMPTY_LINE, NAME_TAKEN_LINE]) {
    check(`one short sentence, no dash or exclamation: ${line}`, /^[A-Z][^–—!]*\.$/.test(line) && line.split(' ').length <= 5, line);
  }

  const saved = { a: 'Meal', b: 'Movie', c: 'Song' };
  const rows = (...list: [string | null, string][]) => list.map(([id, name]) => ({ id, name }));
  const lines = (list, names = saved) =>
    nameProblems(list, names)
      .map((p) => (p === null ? '-' : p === NAME_EMPTY_LINE ? 'empty' : p === NAME_TAKEN_LINE ? 'taken' : `?${p}`))
      .join(',');
  const want = (name: string, list, expected: string, names = saved) => {
    const got = lines(list, names);
    check(name, got === expected, `got ${got}, wanted ${expected}`);
  };
  want('the saved wheel as it was has no problem', rows(['a', 'Meal'], ['b', 'Movie'], ['c', 'Song']), '-,-,-');
  want('a new row with no name', rows(['a', 'Meal'], ['b', 'Movie'], [null, '']), '-,-,empty');
  want('a name of spaces is empty', rows(['a', 'Meal'], [null, '   ']), '-,empty');
  want('a saved row cleared is empty', rows(['a', ''], ['b', 'Movie']), 'empty,-');
  want('a new row with a saved name: only the new row', rows(['a', 'Meal'], ['b', 'Movie'], [null, 'movie']), '-,-,taken');
  want('capitals and outer spaces are ignored', rows(['b', 'Movie'], [null, '  MOVIE ']), '-,taken');
  want('inner spaces count', rows(['b', 'Movie'], [null, 'Mo vie']), '-,-');
  want('an earlier row renamed to a later name: only the renamed row', rows(['a', 'Song'], ['b', 'Movie'], ['c', 'Song']), 'taken,-,-');
  want('a later row renamed to an earlier name: only the renamed row', rows(['a', 'Meal'], ['b', 'Movie'], ['c', 'meal']), '-,-,taken');
  want('two new rows with one name: both', rows(['a', 'Meal'], [null, 'Tea'], [null, 'tea']), '-,taken,taken');
  want('two renamed rows with one name: both', rows(['a', 'Tea'], ['b', 'Tea'], ['c', 'Song']), 'taken,taken,-');
  want('a new and a renamed row on a saved name: both, not the saved row', rows(['a', 'Song'], ['c', 'Song'], [null, 'song']), 'taken,-,taken');
  want('a saved row with only its capitals changed counts as typed', rows(['b', 'MOVIE'], [null, 'movie']), 'taken,taken');
  want('a saved row with spaces added around it is as it was', rows(['b', ' Movie '], [null, 'movie']), '-,taken');
  want('saved rows that match only on the phone both get the line', rows(['x', 'Tea'], ['y', 'TEA']), 'taken,taken', { x: 'Tea', y: 'TEA' });
  want('an id the last read did not have counts as typed', rows(['a', 'Meal'], ['z', 'meal']), '-,taken');
  want('an empty name is never also a repeat', rows([null, ''], [null, ' ']), 'empty,empty');
  check('one answer per row', nameProblems(rows(['a', 'Meal'], [null, 'x'], [null, '']), saved).length === 3);
  check('nothing for no rows', nameProblems([], saved).length === 0);

  // The rule decides whether Save wheel can go, so a sweep of random lists: rows new, saved as they
  // were or renamed, names with any capitals and outer spaces. Every repeated name keeps at least one
  // line (or Save would go and the server refuse the list), a line goes only to a row with a twin, a
  // saved row as it was gets one only when every row with its name is too, and empty is always empty.
  {
    const pool = ['Meal', 'Movie', 'Song', 'Tea', ''];
    const vary = (name: string) => {
      const r = rand();
      const cased = r < 0.33 ? name.toUpperCase() : r < 0.66 ? name.toLowerCase() : name;
      return rand() < 0.2 ? `  ${cased} ` : cased;
    };
    let bad: string | null = null;
    for (let trial = 0; trial < 4000 && bad === null; trial += 1) {
      const n = randInt(2, 12);
      const names: Record<string, string> = {};
      const list: { id: string | null; name: string }[] = [];
      const kept: boolean[] = [];
      for (let i = 0; i < n; i += 1) {
        const name = vary(pool[randInt(0, pool.length - 1)]);
        const kind = randInt(0, 2);
        if (kind === 0) list.push({ id: null, name });
        else {
          names[`id${i}`] = kind === 1 ? name.trim() : `${name.trim()} before`;
          list.push({ id: `id${i}`, name });
        }
        kept.push(kind === 1);
      }
      const got = nameProblems(list, names);
      const keys = list.map((row) => row.name.trim().toLowerCase());
      for (let i = 0; i < n && bad === null; i += 1) {
        const group = keys.flatMap((key, j) => (key === keys[i] ? [j] : []));
        const allKept = group.every((j) => kept[j]);
        const expected =
          keys[i] === '' ? NAME_EMPTY_LINE : group.length === 1 ? null : kept[i] && !allKept ? null : NAME_TAKEN_LINE;
        if (got[i] !== expected) bad = `row ${i} got ${got[i]}, wanted ${expected}: ${JSON.stringify({ list, names })}`;
        if (keys[i] !== '' && group.length > 1 && !group.some((j) => got[j] === NAME_TAKEN_LINE)) {
          bad = `a repeated name with no line: ${JSON.stringify({ list, names })}`;
        }
      }
      checks += 1;
    }
    check('name sweep: every repeated name keeps a line, only rows with a twin get one, empty is always empty', bad === null, bad ?? '');
  }

  const editorRules = readFileSync(fileURLToPath(new URL('../lib/wheel/categoryNames.ts', import.meta.url)), 'utf8');
  check('lib/wheel/categoryNames.ts has no import', !/^\s*import\s/m.test(editorRules) && !/require\(/.test(editorRules));

  // The screen has no React here, so the lines that matter are pinned as written.
  const editor = readFileSync(fileURLToPath(new URL('../components/wheel/WheelEditor.tsx', import.meta.url)), 'utf8');
  check('Edit wheel takes its row problems from nameProblems', /const rowProblems = nameProblems\(rows, savedNames\);/.test(editor));
  check(
    'the saved names come from every server answer',
    /setSavedNames\(Object\.fromEntries\(next\.categories\.map\(\(category\) => \[category\.id, category\.name\]\)\)\);/.test(editor)
  );
  check('the old row lines are gone', !/Give this category a name|You already have a category with this name/.test(editor));
  check(
    'a problem still waits for its box to be left (or the tap on Save wheel)',
    /const problem = row\.touched \? rowProblems\[index\] : null;/.test(editor) &&
      /onBlur=\{\(\) => updateRow\(row\.key, \{ touched: true \}\)\}/.test(editor)
  );
  check(
    'the grey Save wheel takes a tap only while something changed and a name has a problem',
    /const blocked = hasChanges && !saving && firstProblem >= 0;/.test(editor) &&
      /onBlockedPress=\{blocked \? showProblems : undefined\}/.test(editor) &&
      /disabled=\{!canSave\}/.test(editor) &&
      /const canSave = hasChanges && allValid && !saving;/.test(editor)
  );
  check(
    "its VoiceOver hint is the first row's problem, only while blocked",
    /blockedHint=\{blocked \? \(rowProblems\[firstProblem\] \?\? undefined\) : undefined\}/.test(editor)
  );
  check(
    "the tap shows every row's problem, says the first, scrolls to its row and puts the cursor in it, in that order",
    /setRows\(\(current\) => current\.map\(\(item\) => \(item\.touched \? item : \{ \.\.\.item, touched: true \}\)\)\);\s*announceProblem\(problem, NAME_LABEL\);\s*bringIntoView\(focusedInView\.ref\.current, rowRefs\.current\.get\(row\.key\)\);\s*boxRefs\.current\.get\(row\.key\)\?\.focus\(\);/.test(
      editor
    )
  );
  check(
    'each row hands its view and its name box to the tap',
    /if \(view\) rowRefs\.current\.set\(row\.key, view\);\s*else rowRefs\.current\.delete\(row\.key\);/.test(editor) &&
      /if \(box\) boxRefs\.current\.set\(row\.key, box\);\s*else boxRefs\.current\.delete\(row\.key\);/.test(editor)
  );
  check(
    'a problem shows with FieldError, inside the view that holds the box (the keyboard hook keeps that view in view)',
    /<View style=\{\{ flex: 1 \}\}>\s*<TextInput\s+ref=\{ref\}[\s\S]*?\/>\s*\{problem !== null \? <FieldError text=\{problem\} label=\{NAME_LABEL\} \/> : null\}\s*<\/View>/.test(editor)
  );
  check(
    'the box keeps the 1.5 ink border while a problem shows, and the problem is its hint',
    /const marked = focused \|\| problem !== null;/.test(editor) &&
      /borderWidth: marked \? 1\.5 : 1,\s*borderColor: marked \? color\.ink : color\.inkRest,/.test(editor) &&
      /accessibilityHint=\{problem \?\? undefined\}/.test(editor)
  );
  check(
    'Remove stays level with the box when a problem shows under it',
    /flexDirection: 'row', alignItems: 'flex-start'/.test(editor) && /style=\{\{ height: NAME_FIELD_HEIGHT, justifyContent: 'center', paddingLeft: space\.md \}\}/.test(editor)
  );
  check('no plain ink row line is left', !/<Text style=\{\{ \.\.\.labelText, color: color\.ink \}\}>\{problem\}<\/Text>/.test(editor));
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
