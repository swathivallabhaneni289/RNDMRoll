// @ts-nocheck
/**
 * Checks for the Terms of Use and the Privacy Policy (lib/legal/documents.ts), and the writer of
 * their GitHub copies in docs/legal. Not part of the app bundle. Run from the repo root with:
 *   node --experimental-strip-types scripts/check-legal.ts            checks only
 *   node --experimental-strip-types scripts/check-legal.ts --write    writes docs/legal, then checks
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is because Node needs the .ts extension on imports; see check-settings.ts.)
 */
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.cwd();
const legal = await import(pathToFileURL(join(root, 'lib/legal/documents.ts')).href);
const { legalMarkdown } = await import(pathToFileURL(join(root, 'lib/legal/markdown.ts')).href);

const COPIES = [
  { document: legal.TERMS, path: 'docs/legal/TERMS.md' },
  { document: legal.PRIVACY, path: 'docs/legal/PRIVACY.md' },
];

let checks = 0;
let failures = 0;

function check(name: string, ok: boolean, detail?: string): void {
  checks += 1;
  if (!ok) {
    failures += 1;
    console.log(`FAIL ${name}${detail ? `: ${detail}` : ''}`);
  }
}

if (process.argv.includes('--write')) {
  mkdirSync(join(root, 'docs/legal'), { recursive: true });
  for (const { document, path } of COPIES) writeFileSync(join(root, path), legalMarkdown(document));
}

check(
  'the contact is an email address',
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(legal.CONTACT_EMAIL),
  `got ${JSON.stringify(legal.CONTACT_EMAIL)}`,
);

for (const { document, path } of COPIES) {
  const name = document.title;
  check(`${name}: has a title and a date`, document.title !== '' && document.effective !== '');
  check(`${name}: has opening lines and sections`, document.intro.length > 0 && document.sections.length > 0);

  // Every piece of text, in page order, with where it sits (for the messages below).
  const texts = document.intro.map((text, i) => [`intro ${i + 1}`, text]);
  const headings = new Set();
  for (const section of document.sections) {
    check(`${name}: "${section.heading}" is the only section with its heading`, !headings.has(section.heading));
    headings.add(section.heading);
    check(`${name}: "${section.heading}" has something under it`, section.blocks.length > 0);
    texts.push([`heading "${section.heading}"`, section.heading]);
    for (const block of section.blocks) {
      if (block.kind === 'paragraph') {
        texts.push([`a paragraph of "${section.heading}"`, block.text]);
      } else {
        check(`${name}: a list in "${section.heading}" has items`, block.items.length > 0);
        // The items are the screen's keys, so none repeats inside its list.
        check(`${name}: no item repeats in a list of "${section.heading}"`, new Set(block.items).size === block.items.length);
        for (const item of block.items) texts.push([`a list item of "${section.heading}"`, item]);
      }
    }
  }
  check(`${name}: no opening line repeats`, new Set(document.intro).size === document.intro.length);

  for (const [where, text] of texts) {
    check(`${name}: ${where} is not empty`, text.trim() !== '');
    // No em or en dashes in anything the developer ships (their rule), and nothing Markdown would
    // turn into formatting, so the app and GitHub show the same words. (An underscore inside a word,
    // as in an email address, is left alone by GitHub, so it is allowed.)
    check(`${name}: ${where} has no dash`, !/[–—]/.test(text), text);
    check(`${name}: ${where} has no Markdown characters`, !/[*`<>[\]#|\\]|(^|\s)_|_(\s|$)/.test(text), text);
    check(`${name}: ${where} has no placeholder`, !/\b(TODO|TBD|XXX|FIXME)\b/.test(text), text);
  }
  check(
    `${name}: the contact email is in it`,
    texts.some(([, text]) => legal.CONTACT_EMAIL !== '' && text.includes(legal.CONTACT_EMAIL)),
  );

  let onDisk = '';
  try {
    onDisk = readFileSync(join(root, path), 'utf8');
  } catch {
    // Missing: the check below says so.
  }
  check(`${path} matches lib/legal/documents.ts (run the script with --write)`, onDisk === legalMarkdown(document));
}

console.log(`${checks} checks, ${failures} failures`);
process.exit(failures === 0 ? 0 : 1);
