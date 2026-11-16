/**
 * Small pure rules of the shared form pattern: what a grey button says it still needs, what a tap
 * on it does, what VoiceOver says about a problem, and how far a page scrolls to show a field. No
 * React, no imports: this file loads on its own, so scripts/check-forms.ts can run it.
 */

/**
 * "Still needed: birthday and username." The entry form's approved wording (stillNeededLine in
 * lib/entry/rules.ts) for any form. The words are said as given and in the order given (page
 * order), joined by commas and 'and', with a period. Blank words are skipped and a repeated word is
 * said once. An empty list gives '' (nothing to say). It is the VoiceOver hint of a grey button
 * (PrimaryButton's blockedHint), not a line on the page.
 */
export function stillNeeded(labels: readonly string[]): string {
  const items: string[] = [];
  for (const label of labels) {
    const word = label.trim();
    if (word !== '' && !items.includes(word)) items.push(word);
  }
  if (items.length === 0) return '';
  const last = items[items.length - 1];
  const list = items.length === 1 ? last : `${items.slice(0, -1).join(', ')} and ${last}`;
  return `Still needed: ${list}.`;
}

/**
 * What VoiceOver says for a field's problem: "Username. Usernames can't have dots." with the field's
 * label, or the message alone without one. An empty message gives '' (nothing is said).
 */
export function spokenProblem(text: string, label?: string): string {
  const message = text.trim();
  if (message === '') return '';
  const name = (label ?? '').trim();
  return name === '' ? message : `${name}. ${message}`;
}

/** How a PrimaryButton answers a tap, given its `disabled`, `loading` and `onBlockedPress`. */
export type PrimaryButtonAnswer = {
  /** Grey because of what is typed: disabled and not loading. */
  blocked: boolean;
  /** What a tap runs: `onPress`, `onBlockedPress`, or nothing. */
  tap: 'press' | 'blocked' | 'none';
  /** Marked disabled for VoiceOver, which reads it as dimmed. */
  dimmed: boolean;
};

/**
 * A blocked button with a blocked-press handler takes the tap and runs the handler, so the form can
 * show what is missing, and it is not dimmed for VoiceOver. Without a handler it ignores the tap and
 * is dimmed, as before. While loading it ignores taps and keeps the dimmed state its caller gave it.
 */
export function primaryButtonAnswer(a: {
  disabled: boolean;
  loading: boolean;
  hasBlockedPress: boolean;
}): PrimaryButtonAnswer {
  const blocked = a.disabled && !a.loading;
  const tap = a.loading ? 'none' : !a.disabled ? 'press' : a.hasBlockedPress ? 'blocked' : 'none';
  return { blocked, tap, dimmed: a.disabled && tap !== 'blocked' };
}

/** Top and bottom edges of a view on the screen, in points. */
export type Span = { top: number; bottom: number };

/**
 * The scroll position that shows `field` with `spare` points around it inside `area`, or null when
 * it already shows. Both are measured on the screen now, and `scrollTop` is how far the page is
 * scrolled. A field below moves up the least amount that shows its bottom, but a field taller than
 * the area keeps its top in view. A field above moves down until its top shows. Never below 0. The
 * same steps as the keyboard hook (components/ui/useKeepFocusedInputInView.ts).
 */
export function scrollTopToShow(area: Span, field: Span, scrollTop: number, spare: number): number | null {
  const below = field.bottom + spare - area.bottom;
  const above = area.top - (field.top - spare);
  let next: number | null = null;
  if (below > 0) next = scrollTop + Math.min(below, -above);
  else if (above > 0) next = scrollTop - above;
  if (next === null) return null;
  next = Math.max(0, next);
  return next === scrollTop ? null : next;
}
