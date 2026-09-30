import { space } from '@/lib/theme/tokens';
import { formatMonth, LabEntry } from '@/lab/data/entries';
import { grid, photoMetrics } from '@/lab/theme/frost';

/**
 * Pure geometry for the two diary views. Every row has a fixed, known height so the
 * lists can use getItemLayout, and so switching between GRID and DIARY can map the
 * scroll position of one list onto the other without measuring anything: the entry
 * nearest the top of the viewport in the outgoing view becomes the entry at the top
 * of the incoming view.
 */

export const MONTH_ROW_HEIGHT = 64;
/** Padding, gaps and slack in the diary info block; they do not grow with text size. */
const DIARY_INFO_FIXED = 50;
/** The text rows (label, heading, stars, two-line reaction, metadata) at font scale 1. */
const DIARY_INFO_TEXT = 134;
/** Info block height at the default text size (184dp). It grows with the OS font scale. */
export const DIARY_INFO_HEIGHT = DIARY_INFO_FIXED + DIARY_INFO_TEXT;
export const DIARY_GAP = space.xxl;
export const DIARY_MARGIN = space.lg;

export type GridRow =
  | { key: string; kind: 'month'; label: string; height: number; offset: number; firstIndex: number }
  | { key: string; kind: 'tiles'; entries: LabEntry[]; height: number; offset: number; firstIndex: number };

export type GridLayout = {
  rows: GridRow[];
  tileWidth: number;
  tileHeight: number;
  totalHeight: number;
};

/** Entries must already be sorted newest first. */
export function buildGridLayout(entries: readonly LabEntry[], screenWidth: number): GridLayout {
  const usable = screenWidth - grid.margin * 2;
  const tileWidth = Math.floor((usable - grid.gutter * (grid.columns - 1)) / grid.columns);
  const tileHeight = Math.round(tileWidth / photoMetrics.aspect);
  const rowHeight = tileHeight + grid.gutter;

  const rows: GridRow[] = [];
  let offset = 0;
  let index = 0;

  while (index < entries.length) {
    const month = formatMonth(entries[index].date);
    let end = index;
    while (end < entries.length && formatMonth(entries[end].date) === month) end += 1;

    rows.push({ key: `month-${month}`, kind: 'month', label: month, height: MONTH_ROW_HEIGHT, offset, firstIndex: index });
    offset += MONTH_ROW_HEIGHT;

    for (let start = index; start < end; start += grid.columns) {
      const slice = entries.slice(start, Math.min(start + grid.columns, end));
      rows.push({ key: `tiles-${slice[0].id}`, kind: 'tiles', entries: slice, height: rowHeight, offset, firstIndex: start });
      offset += rowHeight;
    }
    index = end;
  }

  return { rows, tileWidth, tileHeight, totalHeight: offset };
}

/** Index of the entry nearest the top of the viewport for a given scroll offset. */
export function gridEntryAtOffset(layout: GridLayout, offset: number): number {
  const { rows } = layout;
  let found = rows.length - 1;
  for (let i = 0; i < rows.length; i += 1) {
    if (rows[i].offset + rows[i].height > offset) {
      found = i;
      break;
    }
  }
  const row = rows[found];
  const next = rows[found + 1];
  return (next && offset - row.offset > row.height / 2 ? next : row).firstIndex;
}

/** Scroll offset that puts an entry's row, or its month heading when it opens a month, at the top. */
export function gridOffsetForEntry(layout: GridLayout, index: number): number {
  const heading = layout.rows.find((row) => row.kind === 'month' && row.firstIndex === index);
  if (heading) return heading.offset;
  const row = layout.rows.find(
    (candidate) =>
      candidate.kind === 'tiles' && index >= candidate.firstIndex && index < candidate.firstIndex + candidate.entries.length,
  );
  return row ? row.offset : 0;
}

export type DiaryLayout = {
  photoWidth: number;
  photoHeight: number;
  /** Height of the fixed info block under the photo; scales with the OS font scale. */
  infoHeight: number;
  itemHeight: number;
  totalHeight: number;
};

/**
 * `fontScale` is the OS text size multiplier. Rows keep a fixed height so the lists
 * can use exact offsets, which means the height itself must follow the text size or
 * larger text would be clipped (the last lines are the metadata and the reaction).
 */
export function buildDiaryLayout(count: number, screenWidth: number, fontScale = 1): DiaryLayout {
  const photoWidth = screenWidth - DIARY_MARGIN * 2;
  const photoHeight = Math.round(photoWidth / photoMetrics.aspect);
  const infoHeight = Math.ceil(DIARY_INFO_FIXED + DIARY_INFO_TEXT * Math.max(1, fontScale));
  const itemHeight = photoHeight + infoHeight + DIARY_GAP;
  return { photoWidth, photoHeight, infoHeight, itemHeight, totalHeight: itemHeight * count };
}

export function diaryEntryAtOffset(layout: DiaryLayout, offset: number, count: number): number {
  return Math.max(0, Math.min(count - 1, Math.round(offset / layout.itemHeight)));
}

export function diaryOffsetForEntry(layout: DiaryLayout, index: number): number {
  return index * layout.itemHeight;
}
