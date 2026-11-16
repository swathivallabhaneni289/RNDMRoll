import { ReactNode, useCallback, type RefObject } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';
import { SafeAreaView, useSafeAreaInsets, type Edge } from 'react-native-safe-area-context';
import { color, space } from '@/lib/theme/tokens';
import { BackgroundDotGrid } from '@/components/brand/BackgroundDotGrid';
import { WheelBackground } from '@/components/brand/WheelBackground';
import { useKeepFocusedInputInView } from '@/components/ui/useKeepFocusedInputInView';

const ALL_EDGES: readonly Edge[] = ['top', 'right', 'bottom', 'left'];

/**
 * The shared screen shell: Dominant-surface SafeAreaView with `space.lg`
 * side margins. `texture` opts in to the dot-grid background (scoped by
 * UI-SPEC to the choose-method screen only, hence the false default).
 * `wheels` opts in to the faint wheel illustration used on Welcome and the method
 * chooser (it replaced the dot grid there). `scroll` wraps children in
 * keyboard-avoiding scroll for form screens. Its keyboard offset is the top inset when `edges`
 * includes 'top', and a box tapped low on the form is scrolled back into view once the keyboard is
 * up. `edges` chooses which safe-area insets the shell
 * adds; the default is all four, so signed-out screens and the first-run page are unchanged.
 * Inside the bottom tab bar (which adds the bottom inset itself) a screen passes
 * ['top', 'left', 'right'] so the bottom inset is not counted twice.
 * `scrollRef` gives a form the scroll view, so it can bring a field with a
 * problem into view with bringIntoView (lib/forms/bringIntoView.ts). It stays
 * null when `scroll` is false.
 */
export function Screen({
  children,
  texture = false,
  wheels = false,
  wheelLabel = null,
  wheelKick = 0,
  scroll = true,
  edges = ALL_EDGES,
  scrollRef,
}: {
  children?: ReactNode;
  texture?: boolean;
  wheels?: boolean | 'corner';
  wheelLabel?: string | null;
  wheelKick?: number;
  scroll?: boolean;
  edges?: readonly Edge[];
  scrollRef?: RefObject<ScrollView | null>;
}) {
  const insets = useSafeAreaInsets();
  const focusedInView = useKeepFocusedInputInView();
  const keepInViewRef = focusedInView.ref;
  // The keyboard hook and the form both hold the scroll view.
  const setScrollView = useCallback(
    (view: ScrollView | null) => {
      keepInViewRef.current = view;
      if (scrollRef) scrollRef.current = view;
    },
    [keepInViewRef, scrollRef]
  );
  const content = scroll ? (
    <KeyboardAvoidingView
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      // The keyboard's top is measured from the top of the phone, but this view's place is measured
      // from just under the top safe area when the shell adds it. Passing that inset lines the two up.
      // Without it, the view pads too little and the keyboard covers the bottom of the form.
      keyboardVerticalOffset={edges.includes('top') ? insets.top : 0}
      style={{ flex: 1 }}
    >
      <ScrollView
        ref={setScrollView}
        onLayout={focusedInView.onLayout}
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{ flexGrow: 1 }}
      >
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  ) : (
    children
  );

  return (
    <SafeAreaView
      edges={edges}
      style={{ flex: 1, backgroundColor: color.dominant, paddingHorizontal: space.lg }}
    >
      {texture ? <BackgroundDotGrid /> : null}
      {wheels === 'corner' ? <WheelBackground corner /> : wheels ? <WheelBackground active={wheelLabel} kick={wheelKick} /> : null}
      <View style={{ flex: 1 }}>{content}</View>
    </SafeAreaView>
  );
}
