import { Tabs } from 'expo-router';
import { getFocusedRouteNameFromRoute } from 'expo-router/react-navigation';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SpinTabIcon } from '@/components/nav/SpinTabIcon';
import { isSelectedTab, TabIcon } from '@/components/nav/TabIcon';
import { spokenTabLabel, TabLabel } from '@/components/nav/TabLabel';
import { useReminderSetup } from '@/lib/notifications/reminder';
import { useSession } from '@/lib/session/store';
import { requestPendingSpin } from '@/lib/wheel/todayStore';
import { color, type } from '@/lib/theme/tokens';

/**
 * The signed-in app: a bottom bar of Spin, Board and Profile (developer, 2026-10-16: Home
 * was dropped because it repeated the Spin page, and Diary because Profile shows every post). The `today` folder IS the Spin page and the
 * landing, reached by the explicit child path `/(app)/today` (a bare `/(app)` shows Unmatched
 * Route here). Its bar slot is a plain tab like the others (developer, 2026-10-16: no raised
 * circle), with the wheel mark as its icon; a press also asks the page to spin once it has
 * loaded and is still open (`pendingSpin` in the Today store).
 *
 *  - The bar is hidden on the one-time photo-and-bio page (the `profile` screen while
 *    `extrasPending` is true; the bar reads the focused route's options), on the pages pushed
 *    over the Spin page that have no bar, and on the entry page and edit entry wherever they are
 *    pushed (the Profile stack mounts them too).
 *  - Every file left in this folder becomes a tab by itself, so the folder holds exactly
 *    `today`, `board` and `profile`, each a folder with its own stack. The Board's stack
 *    (Phase 5) hides the bar on a post, your entry page and edit entry.
 */
export const unstable_settings = {
  initialRouteName: 'today',
};

/** The bar is this tall above the bottom safe area. */
const BAR_CONTENT_HEIGHT = 56;
/** Today's pushed pages that show no bottom bar. */
const BARLESS_TODAY_PAGES = ['notifications', 'follow-requests', 'post', 'entry', 'log', 'edit-entry'];
/** Profile's pushed pages that show no bottom bar (the same entry pages). */
const BARLESS_ENTRY_PAGES = ['entry', 'edit-entry', 'post'];
/** The Board's pushed pages that show no bottom bar: a post, your entry page and edit entry. */
const BARLESS_BOARD_PAGES = ['post', 'entry', 'edit-entry'];

/** True while the tab's own stack shows one of `pages`. */
function showsPage(route: Parameters<typeof getFocusedRouteNameFromRoute>[0], pages: string[]): boolean {
  const focusedPage = getFocusedRouteNameFromRoute(route);
  return focusedPage !== undefined && pages.includes(focusedPage);
}

export default function AppLayout() {
  const insets = useSafeAreaInsets();
  const { extrasPending, user } = useSession();
  const userId = user?.id ?? null;
  // Signed in: the 8 PM reminder's schedule is rebuilt and its tap opens the Spin tab (Phase 6).
  useReminderSetup();

  // A numeric height replaces the bar's own safe-area arithmetic; the bar then pads its content
  // by the inset itself, so the height is the content plus the inset.
  const barStyle = {
    backgroundColor: color.dominant,
    borderTopWidth: 1,
    borderTopColor: color.divider,
    height: BAR_CONTENT_HEIGHT + insets.bottom,
    elevation: 0,
  };
  const hiddenBarStyle = { display: 'none' as const };

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        sceneStyle: { backgroundColor: color.dominant },
        // Tab screens swap at once. The cross-fade of Phase 2.2 D-02 looked wrong on the phone
        // (developer, 2026-10-13); the bar's colours still glide. Options kept: 'fade' or 'shift'.
        animation: 'none',
        tabBarStyle: barStyle,
        tabBarActiveTintColor: color.ink,
        tabBarInactiveTintColor: color.muted,
        tabBarAllowFontScaling: false,
        // Label size, regular weight: the active tab differs by colour only.
        tabBarLabelStyle: {
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          fontWeight: '400',
        },
        tabBarItemStyle: { paddingTop: 6 },
      }}
    >
      <Tabs.Screen
        name="today"
        listeners={{
          // The request goes before the navigation, as the old Spin button did: with unsaved
          // changes in Edit wheel the leave sheet's guard clears it.
          tabPress: () => {
            if (userId !== null) requestPendingSpin(userId);
          },
        }}
        options={({ route, navigation }) => {
          // The Spin page: the wheel, the word, Log it and Skip it, the posted or skipped day, the
          // countdown and the week, all on one page.
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Spin',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Spin'),
            tabBarIcon: () => <SpinTabIcon focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
            tabBarStyle: showsPage(route, BARLESS_TODAY_PAGES) ? hiddenBarStyle : barStyle,
          };
        }}
      />
      <Tabs.Screen
        name="board"
        options={({ route, navigation }) => {
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Board',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Board'),
            tabBarIcon: () => <TabIcon name="people-outline" focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
            tabBarStyle: showsPage(route, BARLESS_BOARD_PAGES) ? hiddenBarStyle : barStyle,
          };
        }}
      />
      <Tabs.Screen
        name="profile"
        options={({ route, navigation }) => {
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Profile',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Profile'),
            tabBarIcon: () => <TabIcon name="person-outline" focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
            tabBarStyle: extrasPending || showsPage(route, BARLESS_ENTRY_PAGES) ? hiddenBarStyle : barStyle,
          };
        }}
      />
    </Tabs>
  );
}
