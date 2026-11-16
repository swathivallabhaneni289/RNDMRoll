import type { HostInstance, ScrollView } from 'react-native';
import { scrollTopToShow } from '@/lib/forms/rules';
import { space } from '@/lib/theme/tokens';

/**
 * Scrolls a form the least amount that shows `field` with `space.md` to spare, the way the keyboard
 * hook keeps a focused box in view. Pass the view around the whole field (its label, box and
 * message), not the text box alone, so the message shows too. `scrollView` is the form's
 * ScrollView: Screen's through its `scrollRef`, or a form's own. It measures on the next frame, so a
 * form can call it right after setting its messages and the page is measured with them showing. A
 * field that already shows does not move, and a field outside that ScrollView is left alone.
 */
export function bringIntoView(
  scrollView: ScrollView | null | undefined,
  field: HostInstance | null | undefined
): void {
  requestAnimationFrame(() => {
    const scroll = scrollView?.getNativeScrollRef();
    if (!scrollView || !scroll || !field || !scroll.contains(field)) return;
    const y = scrollTopToShow(scroll.getBoundingClientRect(), field.getBoundingClientRect(), scroll.scrollTop, space.md);
    if (y !== null) scrollView.scrollTo({ y, animated: true });
  });
}
