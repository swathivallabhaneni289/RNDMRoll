// @ts-nocheck
/**
 * Throwaway checks for the navigation motion numbers and wiring (plan 02.2-01). Not part of the
 * app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-navigation-motion.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * The motion files are read as text and not imported: they pull in react-native and Reanimated,
 * which Node cannot load. (The nocheck line is for the same reason as in check-wheel.ts.)
 */
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.cwd();

let checks = 0;
let failures = 0;

function check(name: string, ok: boolean, detail?: string): void {
  checks += 1;
  if (!ok) {
    failures += 1;
    console.log(`FAIL ${name}${detail ? `: ${detail}` : ''}`);
  }
}

function read(rel: string): string {
  return readFileSync(join(root, rel), 'utf8');
}

function exportedNumber(src: string, name: string): number | null {
  const m = src.match(new RegExp(`export const ${name}\\s*=\\s*(\\d+)\\s*;`));
  return m ? Number(m[1]) : null;
}

// ---- the numbers ------------------------------------------------------------------------------
const nav = read('lib/motion/navigation.ts');
check('pressMs is 90', exportedNumber(nav, 'pressMs') === 90, String(exportedNumber(nav, 'pressMs')));
check('tabFadeMs is 200', exportedNumber(nav, 'tabFadeMs') === 200, String(exportedNumber(nav, 'tabFadeMs')));
check('barColorMs is 220', exportedNumber(nav, 'barColorMs') === 220, String(exportedNumber(nav, 'barColorMs')));

// ---- the shared curve -------------------------------------------------------------------------
const primitives = read('lib/motion/primitives.tsx');
check(
  'primitives exports the ease-out curve',
  /export const EASE_OUT\s*=\s*Easing\.out\(Easing\.cubic\)\s*;/.test(primitives),
);
check(
  'navigation re-exports the same curve as easeOut',
  /import \{ EASE_OUT \} from '@\/lib\/motion\/primitives';/.test(nav) &&
    /export const easeOut\s*=\s*EASE_OUT\s*;/.test(nav),
);
const pressLiteral = primitives.match(/onPressIn[\s\S]*?duration:\s*(\d+)/);
check(
  'PressScale holds the same press duration as pressMs',
  pressLiteral !== null && Number(pressLiteral[1]) === exportedNumber(nav, 'pressMs'),
  pressLiteral ? pressLiteral[1] : 'no duration found',
);

// ---- the glide lives in one place -------------------------------------------------------------
check(
  'the bar colour glide uses barColorMs and easeOut',
  /withTiming\(\s*target\s*,\s*\{\s*duration:\s*barColorMs\s*,\s*easing:\s*easeOut\s*\}\s*\)/.test(nav),
);

const barFiles = [
  'app/(app)/_layout.tsx',
  ...readdirSync(join(root, 'components/nav'))
    .filter((f) => /\.tsx?$/.test(f))
    .map((f) => `components/nav/${f}`),
];
const literalTiming = /\b(duration|delay|durationMs|delayMs)\s*[:=]\s*\d/;
const timingCall = /\bwith(Timing|Spring|Delay|Sequence|Repeat|Decay)\s*\(/;
for (const rel of barFiles) {
  const src = read(rel);
  check(`${rel} has no literal duration`, !literalTiming.test(src));
  check(`${rel} starts no timing of its own`, !timingCall.test(src));
}

// ---- the layout wiring ------------------------------------------------------------------------
const layout = read('app/(app)/_layout.tsx');
check('layout swaps tabs at once (the fade was dropped, 2026-10-13)', /animation:\s*'none'/.test(layout));
check('layout uses TabIcon twice (Board, Profile; Home and Diary were dropped 2026-10-16)', (layout.match(/<TabIcon\b/g) ?? []).length === 2);
check('layout uses TabLabel three times (Spin, Board, Profile)', (layout.match(/<TabLabel\b/g) ?? []).length === 3);
check('layout draws the Spin tab with the wheel mark, no raised button', /<SpinTabIcon\b/.test(layout) && !/tabBarButton/.test(layout));

// ---- tokens only, plain wording ---------------------------------------------------------------
const wording = [
  'lib/motion/navigation.ts',
  'components/nav/TabIcon.tsx',
  'components/nav/TabLabel.tsx',
  'app/(app)/_layout.tsx',
  'scripts/check-navigation-motion.ts',
];
// Built from char codes so this file holds no dash of its own.
const DASHES = new RegExp('[' + String.fromCharCode(0x2013, 0x2014) + ']');
const summary = '.planning/phases/02.2-navigation-motion/02.2-01-SUMMARY.md';
if (existsSync(join(root, summary))) wording.push(summary);
for (const rel of wording) {
  const src = read(rel);
  check(`${rel} has no hex or rgb colour`, !/#[0-9a-fA-F]{3,8}\b/.test(src) && !/\brgba?\(/.test(src));
  check(`${rel} has no em or en dash`, !DASHES.test(src));
  check(`${rel} keeps the spin wording`, !/\broll(s|ed|ing)?\b/i.test(src));
}

console.log(`${checks} checks, ${failures} failures`);
process.exitCode = failures === 0 ? 0 : 1;
