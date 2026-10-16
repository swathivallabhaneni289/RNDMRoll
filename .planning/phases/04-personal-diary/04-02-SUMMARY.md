# 04-02 Summary: the Diary tab and opening entries from Diary and Profile

Built 2026-10-16 in a worktree. Local commits only, nothing pushed. The Profile page was already built (`04-02-PROFILE-SUMMARY.md`). This adds the Diary and wires the entry pages.

## What was built

- **Diary tab** (`components/diary/DiaryScreen.tsx`, route `app/(app)/diary/index.tsx`). From the top: the heading 'Diary', a row of category chips ('All' first, then the server's order, using `Chip`), a hairline, then the list, newest first.
  - A post row shows the cover (76dp), 'Oct 8 · Meal' (plus '· Late' on a late post), the title in the light serif, the stars with '4.5', and the note in quotation marks when there is one (picture 07).
  - A skipped day shows the date and the category word struck through. It only appears under All, because the server sends skipped rows only then.
  - Paging uses `next_cursor`: 20 rows a page, one request at a time, and no doubled rows. Pull to refresh works, and the page reloads every time the tab comes into focus. That reload asks for as many rows as are shown, up to 50, so coming Back from an entry keeps your place.
  - Lines on the page: 'Loading your diary.' while loading. 'Nothing here yet.' when the list is empty, or 'Nothing in {category} yet.' when a chip is chosen. 'Loading more.' under the list while the next page loads. An offline first load shows `StatusMessage` with 'Try again'. A failed next page shows the offline line and 'Try again' under the list.
- **Diary stack** (`app/(app)/diary/_layout.tsx`): index, entry, edit-entry. The tab name is still `diary`, and the old `app/(app)/diary.tsx` is gone.
- **Entry pages in three stacks.** `EntryView` takes `from` ('today', 'diary' or 'profile'). It uses that value to choose the Back name, the Edit route inside the same stack, the 'Back to ...' button when the entry is gone, and where to go when there is no page to go back to. The body of Phase 3's edit route moved, unchanged, into `components/entry/EditEntryScreen.tsx`, and three thin routes mount it. The helpers are in `components/entry/entryFrom.ts`.
- **Profile.** A recent row opens `/(app)/profile/entry`, where Back reads 'Profile'. 'See diary' still opens the Diary tab.
- **Bottom bar.** It is hidden on `entry` and `edit-entry` in the Diary and Profile stacks too, using the same focused-route check as Today. The five tabs and `components/nav` are unchanged.
- **API** (`lib/api/diary.ts`): `getDiary({ categoryId, cursor, limit })`. A key goes into the query string only when it is given. Lists that come back as `null` are read as empty.

## Differences from the paper (on purpose)

- I kept the five-tab bar and the `book-outline` icon. The plan's "bar of four" and `document-text-outline` are out of date.
- No tastes, `diaryStore` or `useHideTabBar` was built. The chosen chip lives in the Diary screen's own state: it lasts while the app is open and is All again after a relaunch. Nothing outside the Diary sets it now that tastes are gone.
- No `quote` role and no token changes. The note line uses the title serif at body size, set to italic.
- A skipped row strikes the category word (as Today does) instead of printing 'Skipped'. The empty line is 'Nothing here yet.' with no 'Go to Today' button.
- There are no fades: list photos appear plainly, and pages use the iOS push.

## Known small gap

The one place a wrong label can show: you open edit entry, and while it is open the entry is deleted on another phone. The save then fails and the button says 'Back to Today' (Phase 3's `EntryForm` and `useUpdateEntry`, not edited). Tapping it still goes to the first page of the current stack, so it lands on the Diary when opened from the Diary.

## Gates

- `npx tsc --noEmit`: exit 0.
- `check-entry.ts` (2721 checks), `check-wheel.ts` (47877), `check-settings.ts` (54) and `check-navigation-motion.ts` (37) all pass, with no failures.
- `npx expo export --platform ios` succeeded.
- Diff of `package.json`, `package-lock.json`, `app.json`, `internal` and `migrations` against the base commit: empty.
- No em or en dash anywhere in app, components, lib or scripts. The new files say 'spin', never the old word. No hex, `rgba`, `radius.full` or fade in the new files.

## Not done here

Nothing has been checked on the phone yet: the developer walks it. Things to check:

- The Diary list with real data.
- A chip filter, then All.
- Scrolling to the second page.
- Opening a post: no bar, Back reads 'Diary'.
- Edit and save, then back to the list.
- Delete: the row is gone.
- From Profile, a recent row: Back reads 'Profile'.
