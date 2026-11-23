// @ts-nocheck
/**
 * Throwaway checks for the shared form pattern: what a grey button still needs, what a tap on it
 * does, what VoiceOver says about a problem, how far a page scrolls to show a field, the error look
 * of a field and the alert for camera or photo access that is off. Not part of the app bundle.
 * Run from the repo root with:  node --experimental-strip-types scripts/check-forms.ts
 * It prints one line per failure and a summary; a clean run prints no line that starts with FAIL.
 * (The nocheck line is for the same reason as in check-wheel.ts: Node needs the .ts extension on
 * imports and tsc, which scans every .ts file in the repo, would object to it.)
 */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { stillNeededLine } from '../lib/entry/rules.ts';
import { primaryButtonAnswer, scrollTopToShow, spokenProblem, stillNeeded } from '../lib/forms/rules.ts';
import {
  ACCESS_OFF_MESSAGE,
  ACCESS_OFF_NOT_NOW,
  ACCESS_OFF_OPEN_SETTINGS,
  accessOffTitle,
} from '../lib/media/access.ts';

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
  check(name, JSON.stringify(actual) === JSON.stringify(expected), `got ${JSON.stringify(actual)}, wanted ${JSON.stringify(expected)}`);
}

const source = (rel: string) => readFileSync(fileURLToPath(new URL(`../${rel}`, import.meta.url)), 'utf8');

/** How many times `text` appears in `body`. */
function count(body: string, text: string): number {
  return body.split(text).length - 1;
}

// ---- stillNeeded: the entry form's wording for any form -------------------------------------

eq('nothing missing says nothing', stillNeeded([]), '');
eq('only blank words say nothing', stillNeeded(['', '  ']), '');
eq('one', stillNeeded(['birthday']), 'Still needed: birthday.');
eq('two are joined by and', stillNeeded(['birthday', 'username']), 'Still needed: birthday and username.');
eq('three: commas, then and, no comma before and', stillNeeded(['email', 'birthday', 'username']), 'Still needed: email, birthday and username.');
eq(
  'the whole sign-up page, in page order',
  stillNeeded(['email', 'password', 'birthday', 'name', 'username']),
  'Still needed: email, password, birthday, name and username.'
);
eq('the order given is kept', stillNeeded(['username', 'birthday']), 'Still needed: username and birthday.');
eq('blank words are skipped', stillNeeded(['', 'name', '  ']), 'Still needed: name.');
eq('words are trimmed', stillNeeded([' name ']), 'Still needed: name.');
eq('a repeated word is said once', stillNeeded(['name', 'username', 'name']), 'Still needed: name and username.');
eq('words are said as given', stillNeeded(['a photo']), 'Still needed: a photo.');
{
  const parts = ['photo', 'title', 'stars'];
  for (let mask = 0; mask < 8; mask += 1) {
    const missing = parts.filter((_, i) => (mask & (1 << i)) !== 0);
    eq(`the entry form's words for [${missing.join(', ')}]`, stillNeeded(missing), stillNeededLine(missing));
  }
  eq('the approved entry line', stillNeeded(parts), 'Still needed: photo, title and stars.');
}

// ---- spokenProblem: what VoiceOver says ------------------------------------------------------

eq('with the field', spokenProblem("Usernames can't have dots.", 'Username'), "Username. Usernames can't have dots.");
eq('the message alone', spokenProblem('Pick your birthday.'), 'Pick your birthday.');
eq('an empty message says nothing', spokenProblem('', 'Username'), '');
eq('a blank message says nothing', spokenProblem('   ', 'Username'), '');
eq('a blank label is left out', spokenProblem('Enter your email.', '  '), 'Enter your email.');
eq('both are trimmed', spokenProblem('  Enter your email. ', ' Email '), 'Email. Enter your email.');

// ---- primaryButtonAnswer: every disabled, loading and handler mix ---------------------------

{
  const table = [
    // disabled, loading, handler -> blocked, tap, dimmed
    [false, false, false, false, 'press', false],
    [false, false, true, false, 'press', false],
    [true, false, false, true, 'none', true],
    [true, false, true, true, 'blocked', false],
    // Loading alone stays busy and not dimmed, as before (Try again, the offline page).
    [false, true, false, false, 'none', false],
    [false, true, true, false, 'none', false],
    [true, true, false, false, 'none', true],
    [true, true, true, false, 'none', true],
  ];
  for (const [disabled, loading, hasBlockedPress, blocked, tap, dimmed] of table) {
    eq(
      `button disabled ${disabled}, loading ${loading}, blocked handler ${hasBlockedPress}`,
      primaryButtonAnswer({ disabled, loading, hasBlockedPress }),
      { blocked, tap, dimmed }
    );
  }
}

// ---- scrollTopToShow: the least scroll that shows a field ----------------------------------

{
  const area = { top: 100, bottom: 700 };
  const spare = 16;
  eq('a field in view does not move', scrollTopToShow(area, { top: 300, bottom: 400 }, 200, spare), null);
  eq('a field just inside the spare does not move', scrollTopToShow(area, { top: 116, bottom: 684 }, 200, spare), null);
  eq('one point too low moves one point', scrollTopToShow(area, { top: 200, bottom: 685 }, 200, spare), 201);
  eq('a field below shows its bottom with the spare', scrollTopToShow(area, { top: 800, bottom: 900 }, 200, spare), 416);
  eq('a field above shows its top with the spare', scrollTopToShow(area, { top: -50, bottom: 50 }, 200, spare), 34);
  eq('never above the top of the page', scrollTopToShow(area, { top: -200, bottom: -100 }, 100, spare), 0);
  eq('no move when the page is already at the top', scrollTopToShow(area, { top: 50, bottom: 150 }, 0, spare), null);
  eq('a field taller than the area keeps its top in view', scrollTopToShow(area, { top: 300, bottom: 1300 }, 200, spare), 384);
  eq('a field past both edges shows its top', scrollTopToShow(area, { top: 0, bottom: 900 }, 200, spare), 84);
}

// ---- The alert for camera or photo access that is off --------------------------------------

eq('camera title', accessOffTitle('camera'), 'Camera access is off');
eq('library title', accessOffTitle('library'), 'Photo access is off');
eq('message', ACCESS_OFF_MESSAGE, 'Turn it on in Settings.');
eq('first button', ACCESS_OFF_NOT_NOW, 'Not now');
eq('second button', ACCESS_OFF_OPEN_SETTINGS, 'Open Settings');
{
  const words = [
    accessOffTitle('camera'),
    accessOffTitle('library'),
    ACCESS_OFF_MESSAGE,
    ACCESS_OFF_NOT_NOW,
    ACCESS_OFF_OPEN_SETTINGS,
    stillNeeded(['email', 'birthday', 'username']),
    spokenProblem('Pick your birthday.', 'Birthday'),
  ];
  for (const text of words) {
    check(`no em dash and no exclamation mark: ${text}`, !/[\u2014!]/.test(text));
  }
}
{
  const alert = source('lib/media/showAccessOff.ts');
  check(
    'the alert: title, message, Not now as the cancel button, then Open Settings',
    /Alert\.alert\(accessOffTitle\(source\), ACCESS_OFF_MESSAGE, \[\s*\{ text: ACCESS_OFF_NOT_NOW, style: 'cancel' \},\s*\{\s*text: ACCESS_OFF_OPEN_SETTINGS,/.test(alert)
  );
  check('Open Settings opens the Settings app', /Linking\.openSettings\(\)\.catch\(\(\) => undefined\)/.test(alert));
}

// ---- What the components do with these (no React to run them, so pinned as written) ----------

{
  const field = source('components/ui/TextField.tsx');
  const fieldError = field.slice(field.indexOf('export function FieldError('), field.indexOf('export function TextField('));
  check('FieldError is exported', fieldError.length > 0);
  check(
    'FieldError: a 16dp alert mark in ink, hidden from VoiceOver',
    /accessibilityElementsHidden\s*importantForAccessibility="no-hide-descendants"/.test(fieldError) &&
      /<Ionicons name="alert-circle" size=\{16\} color=\{color\.ink\} \/>/.test(fieldError)
  );
  check('FieldError: the words in ink at Label size', /<AppText role="label">\{text\}<\/AppText>/.test(fieldError));
  check('FieldError: read once, after the label when there is one', /accessibilityLabel=\{label \? `\$\{label\}: \$\{text\}` : text\}/.test(fieldError));
  check('FieldError: no red', !/destructive/.test(fieldError));
  check(
    'an error replaces the green note',
    /success && !error && status === 'idle' \? <FieldSuccess/.test(field) && /\{error \? <FieldError text=\{error\} label=\{label\} \/> : null\}/.test(field)
  );
  check('the error keeps the 1.5dp ink border', /const marked = focused \|\| Boolean\(error\);/.test(field));
  check(
    'every box variant uses it',
    /borderWidth: marked \? 1\.5 : 1,/.test(field) &&
      /borderWidth: line \? 0 : marked \? 1\.5 : 1,/.test(field) &&
      /borderBottomWidth: marked \? 1\.5 : 1,/.test(field) &&
      count(field, 'borderColor: marked ? color.ink : color.inkRest,') === 2
  );
  check('both text boxes take the ref', /ref\?: Ref<TextInput>;/.test(field) && count(field, 'ref={ref}') === 2);
  check(
    'a password box: dots until its eye is tapped, and never capitals or autocorrect',
    /const inputProps: TextInputProps = revealable \? \{ autoCapitalize: 'none', autoCorrect: false, \.\.\.rest, secureTextEntry: !shown \} : rest;/.test(field) &&
      count(field, '{...inputProps}') === 2 &&
      count(field, '{eye}') === 2 &&
      /<Ionicons name=\{shown \? 'eye-off-outline' : 'eye-outline'\} size=\{22\} color=\{color\.muted\} \/>/.test(field) &&
      /accessibilityLabel=\{`\$\{shown \? 'Hide' : 'Show'\} \$\{label\.toLowerCase\(\)\}`\}/.test(field)
  );
  check('the error is the hint of both text boxes', count(field, 'accessibilityHint={error ? error : rest.accessibilityHint}') === 2);
  check('the comment no longer says a live region speaks on iPhone', !/announces the error to screen readers on appearance/.test(field));
}
{
  const button = source('components/ui/PrimaryButton.tsx');
  check('PrimaryButton follows primaryButtonAnswer', /primaryButtonAnswer\(\{ disabled, loading, hasBlockedPress: onBlockedPress !== undefined \}\)/.test(button));
  check(
    'a tap runs onPress, onBlockedPress or nothing',
    /onPress=\{answer\.tap === 'press' \? onPress : answer\.tap === 'blocked' \? onBlockedPress : undefined\}/.test(button)
  );
  check('VoiceOver: dimmed only when it says so', /accessibilityState=\{\{ disabled: answer\.dimmed, busy: loading \}\}/.test(button));
  check('VoiceOver: the blocked hint only while blocked', /accessibilityHint=\{answer\.blocked \? blockedHint : undefined\}/.test(button));
  check(
    'a grey button keeps its grey look and gets no press scale',
    /style=\{isInert \? undefined : pressStyle\}/.test(button) && /onPressIn=\{\s*isInert/.test(button) && /\) : disabled \? \(/.test(button)
  );
  check('the button never closes the keyboard, so a handler can focus a box', !/Keyboard/.test(button));
}
{
  const screen = source('components/ui/Screen.tsx');
  check('Screen takes a scrollRef', /scrollRef\?: RefObject<ScrollView \| null>;/.test(screen));
  check(
    'the keyboard hook and the form both get the scroll view',
    /keepInViewRef\.current = view;\s*if \(scrollRef\) scrollRef\.current = view;/.test(screen) &&
      /ref=\{setScrollView\}\s*onLayout=\{focusedInView\.onLayout\}/.test(screen) &&
      !/\{\.\.\.focusedInView\}/.test(screen)
  );
  check('a tap on a button still keeps the keyboard up', /keyboardShouldPersistTaps="handled"/.test(screen));
}
{
  // The keyboard hook keeps the whole field in view, not just the box's border: the line under a
  // soft box, the username's chips and the password checklist sit outside the TextInput's parent.
  const hook = source('components/ui/useKeepFocusedInputInView.ts');
  check('the field marker is exported', /export const FIELD_VIEW_ID = 'form-field';/.test(hook));
  check(
    'the hook walks up from the box to its scroll view, the outermost marked view first',
    /for \(let view = box\.parentElement; view !== null && view !== scroll; view = view\.parentElement\) \{\s*if \(view\.id === FIELD_VIEW_ID\) views\.unshift\(view\);\s*\}/.test(hook)
  );
  check(
    'then the view right around the box, once',
    /const parent = box\.parentElement;\s*if \(parent !== null && !views\.includes\(parent\)\) views\.push\(parent\);/.test(hook)
  );
  check(
    'the first that fits with space.md to spare is kept in view, else the box alone',
    /let shown = box\.getBoundingClientRect\(\);\s*for \(const view of viewsAround\(box, scroll\)\) \{\s*const rect = view\.getBoundingClientRect\(\);\s*if \(rect\.height \+ 2 \* space\.md <= area\.height\) \{\s*shown = rect;\s*break;\s*\}\s*\}/.test(hook)
  );
  check('the hook no longer keeps only the parent', !/box\.parentElement \?\? box/.test(hook));
  const field = source('components/ui/TextField.tsx');
  check("both of TextField's own views carry the marker", count(field, '<View nativeID={FIELD_VIEW_ID}>') === 2);
  const form = source('components/profile/ProfileForm.tsx');
  check(
    'every sign-up and Edit field view carries the marker',
    ['email', 'password', 'birthday', 'name', 'username'].every((key) =>
      form.includes(`<View ref={fieldRefs.${key}} nativeID={FIELD_VIEW_ID}`)
    )
  );
  const password = source('components/settings/PasswordScreen.tsx');
  check(
    'both Password field views carry the marker',
    password.includes('<View ref={currentFieldRef} nativeID={FIELD_VIEW_ID}') &&
      password.includes('<View ref={nextFieldRef} nativeID={FIELD_VIEW_ID}')
  );
}
{
  const birthday = source('components/profile/BirthdayPickerField.tsx');
  check('the birthday shows its error with the mark', /<FieldError text=\{error\} label="Birthday" \/>/.test(birthday));
  check(
    'the birthday box keeps the 1.5dp ink border with an error',
    /borderWidth: open \|\| error \? 1\.5 : 1,/.test(birthday) && /borderColor: open \|\| error \? color\.ink : color\.inkRest,/.test(birthday)
  );
  check('the birthday error is its hint', /accessibilityHint=\{error \? error : 'Opens the date picker'\}/.test(birthday));
  const handle = birthday.indexOf('useImperativeHandle(ref, () => ({ open: openSheet }));');
  check('a form can open the date sheet, set up before the early return', handle > 0 && handle < birthday.indexOf('if (!nativePicker) return null;'));
}
{
  const announce = source('lib/forms/announce.ts');
  check(
    'announceProblem speaks with VoiceOver, not a live region',
    /AccessibilityInfo\.announceForAccessibility\(spoken\)/.test(announce) && /spokenProblem\(text, label\)/.test(announce)
  );
  const bring = source('lib/forms/bringIntoView.ts');
  check(
    'bringIntoView measures on the next frame, only inside its scroll view, with space.md to spare',
    /requestAnimationFrame\(/.test(bring) &&
      /!scroll\.contains\(field\)/.test(bring) &&
      /scrollTopToShow\(scroll\.getBoundingClientRect\(\), field\.getBoundingClientRect\(\), scroll\.scrollTop, space\.md\)/.test(bring) &&
      /scrollView\.scrollTo\(\{ y, animated: true \}\)/.test(bring)
  );
}

console.log(failures === 0 ? `ok: ${checks} checks, no failure` : `${failures} of ${checks} checks failed`);
if (failures > 0) process.exitCode = 1;
