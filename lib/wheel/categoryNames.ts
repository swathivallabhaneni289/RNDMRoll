/**
 * Edit wheel's rules for category names (02-UI-SPEC section 7): which rows have a problem and the
 * line under each. Pure: no React, no imports, so scripts/check-wheel.ts can run it.
 */

/** Under a category with no name, or only spaces. */
export const NAME_EMPTY_LINE = 'Name it or remove it.';

/** Under a category whose name another row has too, capitals and outer spaces ignored. */
export const NAME_TAKEN_LINE = 'Already on the wheel.';

/** A row as Edit wheel holds it: the saved category's id (null for a row added on this visit) and the typed name. */
export type NameRow = { id: string | null; name: string };

/**
 * The line for each row, or null when its name is fine. Names are compared trimmed and in lower
 * case. Of two rows with the same name, the line goes under the one the person typed in (a new
 * row, or a saved one renamed), so it sits where they are and the cursor goes there; a saved name
 * left as it was keeps its place. When every row with that name was typed, or every one is saved
 * as it was, each gets the line. `saved` is each saved category's name by id, as last read.
 */
export function nameProblems(rows: readonly NameRow[], saved: Readonly<Record<string, string>>): (string | null)[] {
  const keys = rows.map((row) => row.name.trim().toLowerCase());
  const kept = rows.map((row) => row.id !== null && saved[row.id]?.trim() === row.name.trim());
  return rows.map((_, index) => {
    if (keys[index] === '') return NAME_EMPTY_LINE;
    const twins = keys.flatMap((key, other) => (other !== index && key === keys[index] ? [other] : []));
    if (twins.length === 0) return null;
    if (kept[index] && twins.some((other) => !kept[other])) return null;
    return NAME_TAKEN_LINE;
  });
}
