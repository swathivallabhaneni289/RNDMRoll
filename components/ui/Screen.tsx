import { ReactNode } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { color, space } from '@/lib/theme/tokens';
import { BackgroundDotGrid } from '@/components/brand/BackgroundDotGrid';

/**
 * The shared screen shell: Dominant-surface SafeAreaView with `space.lg`
 * side margins. `texture` opts in to the dot-grid background (scoped by
 * UI-SPEC to the choose-method screen only, hence the false default).
 * `scroll` wraps children in keyboard-avoiding scroll for form screens.
 */
export function Screen({
  children,
  texture = false,
  scroll = true,
}: {
  children?: ReactNode;
  texture?: boolean;
  scroll?: boolean;
}) {
  const content = scroll ? (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
      <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{ flexGrow: 1 }}>
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  ) : (
    children
  );

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: color.dominant, paddingHorizontal: space.lg }}>
      {texture ? <BackgroundDotGrid /> : null}
      <View style={{ flex: 1 }}>{content}</View>
    </SafeAreaView>
  );
}
