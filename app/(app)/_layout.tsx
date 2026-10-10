import { Ionicons } from '@expo/vector-icons';
import { Tabs } from 'expo-router';
import { getFocusedRouteNameFromRoute } from 'expo-router/react-navigation';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SpinTabButton } from '@/components/nav/SpinTabButton';
import { useSession } from '@/lib/session/store';
import { color, type } from '@/lib/theme/tokens';

/**
 * The signed-in app: a bottom bar of Home, Spin and Profile (02-UI-SPEC section 8). Diary and
 * Board join in Phases 4 and 5 and get no placeholder tabs. The landing is Today, reached by
 * the explicit child path `/(app)/today` (a bare `/(app)` shows Unmatched Route here).
 *
 *  - The middle button is an action drawn by `SpinTabButton`; it never becomes the selected tab.
 *  - The bar is hidden on the one-time photo-and-bio page (the `profile` screen while
 *    `extrasPending` is true; the bar reads the focused route's options) and on the pages
 *    pushed over Today that have no bar (`notifications` now, `entry` in Phase 3).
 *  - Every file left in this folder becomes a tab by itself, so the folder holds exactly
 *    `today`, `spin` and `profile` (Phase 1's placeholder page is gone).
 */
export const unstable_settings = {
  initialRouteName: 'today',
};

/** The bar is this tall above the bottom safe area. */
const BAR_CONTENT_HEIGHT = 56;
/** Today's pushed pages that show no bottom bar. */
const BARLESS_TODAY_PAGES = ['notifications', 'entry'];

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
        options={({ route }) => {
          const focusedPage = getFocusedRouteNameFromRoute(route);
          const barless = focusedPage !== undefined && BARLESS_TODAY_PAGES.includes(focusedPage);
          return {
            title: 'Home',
            tabBarIcon: ({ color: tint }) => <Ionicons name="home-outline" size={24} color={tint} />,
            tabBarStyle: barless ? hiddenBarStyle : barStyle,
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
        name="profile"
        options={{
          title: 'Profile',
          tabBarIcon: ({ color: tint }) => <Ionicons name="person-outline" size={24} color={tint} />,
          tabBarStyle: extrasPending ? hiddenBarStyle : barStyle,
        }}
      />
    </Tabs>
  );
}
