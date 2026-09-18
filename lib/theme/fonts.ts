import { useFonts } from 'expo-font';
import { Domine_600SemiBold } from '@expo-google-fonts/domine';
import { WorkSans_400Regular, WorkSans_600SemiBold } from '@expo-google-fonts/work-sans';
import { Platform } from 'react-native';

/**
 * Loads the three locked font files this app uses (UI-SPEC Typography/Font rows).
 * Screens must not render text until `fontsLoaded` is true (or `fontError` is set,
 * in which case they fall back to `systemFallback` below) — see the Interaction
 * Contracts "Session-restore and font-load gate" section.
 */
export function useAppFonts() {
  const [fontsLoaded, fontError] = useFonts({
    Domine_600SemiBold,
    WorkSans_400Regular,
    WorkSans_600SemiBold,
  });

  return { fontsLoaded, fontError };
}

/**
 * Platform system-font fallback for each of the five type roles, used only in the
 * font-load-error branch. Applied as the equivalent numeric weight (400 or 600)
 * alongside the platform system family, per UI-SPEC's Typography platform note.
 */
export const systemFallback = {
  body: { fontFamily: Platform.OS === 'ios' ? 'System' : 'sans-serif', weight: 400 as const },
  label: { fontFamily: Platform.OS === 'ios' ? 'System' : 'sans-serif', weight: 400 as const },
  button: { fontFamily: Platform.OS === 'ios' ? 'System' : 'sans-serif-medium', weight: 600 as const },
  heading: { fontFamily: Platform.OS === 'ios' ? 'System' : 'sans-serif-medium', weight: 600 as const },
  display: { fontFamily: Platform.OS === 'ios' ? 'System' : 'sans-serif-medium', weight: 600 as const },
} as const;
