import { useCallback, useRef } from 'react';
import { TextInput, type HostInstance, type LayoutChangeEvent, type ScrollView } from 'react-native';
import { space } from '@/lib/theme/tokens';

/**
 * Keeps the focused text box in view when the keyboard makes a scrolling form shorter. A tap on a
 * box scrolls it into the area that shows at that moment, but the keyboard view shrinks the
 * ScrollView only afterwards, so a box low on the page ended up under the keyboard until the person
 * scrolled. Each time the ScrollView gets shorter, this scrolls the least amount that shows the
 * box's field (the view right around the box: a profile field's border and count, or a category's
 * row) with `space.md` to spare, or the box alone when its field is too tall to fit. A box that is
 * already in view does not move, and a box outside this ScrollView is left alone. Spread the result
 * on the ScrollView.
 */
export function useKeepFocusedInputInView() {
  const ref = useRef<ScrollView>(null);
  const heightRef = useRef(0);

  const onLayout = useCallback((event: LayoutChangeEvent) => {
    const height = event.nativeEvent.layout.height;
    const shorter = height < heightRef.current;
    heightRef.current = height;
    const scroll = ref.current?.getNativeScrollRef();
    const box: HostInstance | null = TextInput.State.currentlyFocusedInput();
    if (!shorter || !scroll || !box || !scroll.contains(box)) return;
    // Places on the screen now, with the ScrollView already shorter.
    const area = scroll.getBoundingClientRect();
    const field = (box.parentElement ?? box).getBoundingClientRect();
    const shown = field.height + 2 * space.md <= area.height ? field : box.getBoundingClientRect();
    const below = shown.bottom + space.md - area.bottom;
    const above = area.top - (shown.top - space.md);
    if (below > 0) {
      // A box taller than the area keeps its top in view.
      ref.current?.scrollTo({ y: scroll.scrollTop + Math.min(below, -above), animated: true });
    } else if (above > 0) {
      ref.current?.scrollTo({ y: Math.max(0, scroll.scrollTop - above), animated: true });
    }
  }, []);

  return { ref, onLayout };
}
