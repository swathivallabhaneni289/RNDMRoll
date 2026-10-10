import { Redirect } from 'expo-router';

/**
 * The middle button of the bottom bar is an action, not a place (components/nav/SpinTabButton.tsx
 * draws it and sends the person to Today). This route exists only so the Tabs layout has a
 * screen for that button; if it is ever focused it goes straight to Today.
 */
export default function SpinRoute() {
  return <Redirect href="/(app)/today" />;
}
