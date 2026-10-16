import { Tabs } from 'expo-router';
import { getFocusedRouteNameFromRoute } from 'expo-router/react-navigation';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SpinTabButton } from '@/components/nav/SpinTabButton';
import { isSelectedTab, TabIcon } from '@/components/nav/TabIcon';
import { spokenTabLabel, TabLabel } from '@/components/nav/TabLabel';
import { useSession } from '@/lib/session/store';
import { color, type } from '@/lib/theme/tokens';

/**
 * The signed-in app: a bottom bar of Diary, Spin, Board and Profile (developer, 2026-10-16: Home
 * was dropped because it repeated the Spin page). The `today` folder IS the Spin page and the
 * landing, reached by the explicit child path `/(app)/today` (a bare `/(app)` shows Unmatched
 * Route here). Its bar slot is the raised Spin circle drawn by `SpinTabButton`, an action that
 * goes to the page (and spins when the wheel is open); it never shows a selected look.
 *
 *  - The bar is hidden on the one-time photo-and-bio page (the `profile` screen while
 *    `extrasPending` is true; the bar reads the focused route's options), on the pages pushed
 *    over the Spin page that have no bar, and on the entry page and edit entry wherever they are
 *    pushed (the Diary and Profile stacks mount them too).
 *  - Every file left in this folder becomes a tab by itself, so the folder holds exactly
 *    `today`, `diary`, `board` and `profile`. Diary is a folder with its own stack.
 */
export const unstable_settings = {
  initialRouteName: 'today',
};

/** The bar is this tall above the bottom safe area. */
const BAR_CONTENT_HEIGHT = 56;
/** Today's pushed pages that show no bottom bar. */
const BARLESS_TODAY_PAGES = ['notifications', 'entry', 'log', 'edit-entry'];
/** The Diary's and Profile's pushed pages that show no bottom bar (the same entry pages). */
const BARLESS_ENTRY_PAGES = ['entry', 'edit-entry'];

/** True while the tab's own stack shows one of `pages`. */
function showsPage(route: Parameters<typeof getFocusedRouteNameFromRoute>[0], pages: string[]): boolean {
  const focusedPage = getFocusedRouteNameFromRoute(route);
  return focusedPage !== undefined && pages.includes(focusedPage);
}

export default function AppLayout() {
  const insets = useSafeAreaInsets();
  const { extrasPending } = useSession();

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
        name="diary"
        options={({ route, navigation }) => {
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Diary',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Diary'),
            tabBarIcon: () => <TabIcon name="book-outline" focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
            tabBarStyle: showsPage(route, BARLESS_ENTRY_PAGES) ? hiddenBarStyle : barStyle,
          };
        }}
      />
      <Tabs.Screen
        name="today"
        options={({ route }) => ({
          // The Spin page: the wheel, the word, Log it and Skip it, the posted or skipped day, the
          // countdown and the week, all on one page. Its bar slot is the raised circle.
          title: 'Spin',
          tabBarAccessibilityLabel: 'Spin',
          tabBarItemStyle: { paddingTop: 0 },
          tabBarButton: () => <SpinTabButton />,
          tabBarStyle: showsPage(route, BARLESS_TODAY_PAGES) ? hiddenBarStyle : barStyle,
        })}
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
