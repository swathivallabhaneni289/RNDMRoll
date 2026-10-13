import { Tabs } from 'expo-router';
import { getFocusedRouteNameFromRoute } from 'expo-router/react-navigation';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SpinTabButton } from '@/components/nav/SpinTabButton';
import { isSelectedTab, TabIcon } from '@/components/nav/TabIcon';
import { spokenTabLabel, TabLabel } from '@/components/nav/TabLabel';
import { useSession } from '@/lib/session/store';
import { color, type } from '@/lib/theme/tokens';

/**
 * The signed-in app: a bottom bar of Home, Diary, Spin, Board and Profile (02-UI-SPEC section 8).
 * Diary and Board are plain pages ("Nothing here yet.") until Phases 4 and 5 fill them (decision
 * D-24). The landing is Today, reached by the explicit child path `/(app)/today` (a bare
 * `/(app)` shows Unmatched Route here).
 *
 *  - The middle button is an action drawn by `SpinTabButton`; it never becomes the selected tab.
 *  - The bar is hidden on the one-time photo-and-bio page (the `profile` screen while
 *    `extrasPending` is true; the bar reads the focused route's options) and on the pages
 *    pushed over Today that have no bar (`notifications` now, `entry` in Phase 3).
 *  - Every file left in this folder becomes a tab by itself, so the folder holds exactly
 *    `today`, `diary`, `spin`, `board` and `profile` (Phase 1's placeholder page is gone).
 */
export const unstable_settings = {
  initialRouteName: 'today',
};

/** The bar is this tall above the bottom safe area. */
const BAR_CONTENT_HEIGHT = 56;
/** Today's pushed pages that show no bottom bar. */
const BARLESS_TODAY_PAGES = ['notifications', 'entry', 'log', 'edit-entry'];

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
        name="today"
        options={({ route, navigation }) => {
          const focusedPage = getFocusedRouteNameFromRoute(route);
          const barless = focusedPage !== undefined && BARLESS_TODAY_PAGES.includes(focusedPage);
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Home',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Home'),
            tabBarIcon: () => <TabIcon name="home-outline" focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
            tabBarStyle: barless ? hiddenBarStyle : barStyle,
          };
        }}
      />
      <Tabs.Screen
        name="diary"
        options={({ route, navigation }) => {
          const selected = isSelectedTab(navigation, route);
          return {
            title: 'Diary',
            tabBarAccessibilityLabel: spokenTabLabel(navigation, route, 'Diary'),
            tabBarIcon: () => <TabIcon name="book-outline" focused={selected} />,
            tabBarLabel: ({ focused, children }) => <TabLabel title={children} focused={focused} />,
          };
        }}
      />
      <Tabs.Screen
        name="spin"
        options={{
          title: 'Spin',
          tabBarAccessibilityLabel: 'Spin',
          tabBarItemStyle: { paddingTop: 0 },
          tabBarButton: () => <SpinTabButton />,
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
            tabBarStyle: extrasPending ? hiddenBarStyle : barStyle,
          };
        }}
      />
    </Tabs>
  );
}
