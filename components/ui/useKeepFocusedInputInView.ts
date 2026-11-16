import { useCallback, useRef } from 'react';
import { TextInput, type HostInstance, type LayoutChangeEvent, type ScrollView } from 'react-native';
import { space } from '@/lib/theme/tokens';

/**
 * The nativeID of a view the hook keeps in view whole while a box inside it has focus: TextField's
 * own view (its label, box and the line under it), and a form's view around a field together with
 * what sits under it (sign-up's username chips, the password checklist).
 */
export const FIELD_VIEW_ID = 'form-field';

/** A view on the way up from the focused box. */
type Element = NonNullable<HostInstance['parentElement']>;

/**
 * The views around the box, in the order they are tried: the marked ones, outermost first, then
 * the view right around the box (a category's row keeps its problem in it).
 */
function viewsAround(box: HostInstance, scroll: HostInstance): Element[] {
  const views: Element[] = [];
  for (let view = box.parentElement; view !== null && view !== scroll; view = view.parentElement) {
    if (view.id === FIELD_VIEW_ID) views.unshift(view);
  }
  const parent = box.parentElement;
  if (parent !== null && !views.includes(parent)) views.push(parent);
  return views;
}

/**
 * Keeps the focused text box in view when the keyboard makes a scrolling form shorter. A tap on a
 * box scrolls it into the area that shows at that moment, but the keyboard view shrinks the
 * ScrollView only afterwards, so a box low on the page ended up under the keyboard until the person
 * scrolled. Each time the ScrollView gets shorter, this scrolls the least amount that shows the
 * box's field with `space.md` to spare: the first of viewsAround that fits, or the box alone. So a
 * problem under the box stays above the keyboard, also when a tap on a grey button put the cursor
 * there (bringIntoView showed the field before the keyboard came up). A box that is already in
 * view does not move, and a box outside this ScrollView is left alone. Spread the result on the
 * ScrollView.
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
    let shown = box.getBoundingClientRect();
    for (const view of viewsAround(box, scroll)) {
      const rect = view.getBoundingClientRect();
      if (rect.height + 2 * space.md <= area.height) {
        shown = rect;
        break;
      }
    }
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
