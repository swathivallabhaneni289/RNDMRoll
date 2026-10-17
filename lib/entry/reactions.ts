/**
 * The six reactions (UI spec 6, plan 03-02 Task 1).
 *
 * This is the ONLY file in the app that holds an emoji. The server and the database keep the
 * names only (`internal/spin/entry.go`, `Reactions`), never the characters, so swapping a
 * character later is a change in this one file. The order below is the order of the picker and
 * of the server's list. The scan in the plan's acceptance list expects exactly this file to
 * match, so the characters are written as literal characters here and nowhere else.
 *
 * Pure: no imports, no React.
 */

export type ReactionName = 'love' | 'fire' | 'laughing' | 'wow' | 'sad' | 'sleepy';

export const REACTIONS: readonly { name: ReactionName; char: string; label: string }[] = [
  { name: 'love', char: '😍', label: 'Love' },
  { name: 'fire', char: '🔥', label: 'Fire' },
  { name: 'laughing', char: '😂', label: 'Laughing' },
  { name: 'wow', char: '😮', label: 'Wow' },
  { name: 'sad', char: '😢', label: 'Sad' },
  { name: 'sleepy', char: '😴', label: 'Sleepy' },
];

/**
 * The quick row above the comment box, as Instagram draws it (developer, 2026-10-17): one tap adds
 * the character to the text. Kept here so this stays the one file that holds an emoji.
 */
export const COMMENT_EMOJIS: readonly { char: string; label: string }[] = [
  { char: '❤️', label: 'Red heart' },
  { char: '🙌', label: 'Raised hands' },
  { char: '🔥', label: 'Fire' },
  { char: '👏', label: 'Clapping' },
  { char: '😢', label: 'Crying' },
  { char: '😍', label: 'Heart eyes' },
  { char: '😮', label: 'Wow' },
  { char: '😂', label: 'Laughing' },
];

/** True for exactly the six names, spelled as the server spells them. */
export function isReactionName(v: unknown): v is ReactionName {
  return typeof v === 'string' && REACTIONS.some((reaction) => reaction.name === v);
}
