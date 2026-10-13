import { Pressable, View, type AccessibilityActionEvent } from 'react-native';
import { StarGlyphIcon } from '@/components/entry/StarDisplay';
import {
  formatStars,
  starFromTap,
  starGlyphs,
  stepStars,
  STAR_COUNT,
  type StarSide,
} from '@/lib/entry/stars';
import { minTouchTarget } from '@/lib/theme/tokens';

const SLOT = 45;
const GLYPH = 34;
// The first glyph sits flush with the column edge, as in the picture: the row starts half of the
// slot's spare width to the left. The stars are 45dp apart centre to centre.
const FLUSH = -(SLOT - GLYPH) / 2;

/**
 * The rating row (UI spec 5): five 34dp stars, each slot split into a left and a right tap zone
 * (left gives a half star, right the whole star). No minus, plus or helper line, and no press
 * scale. The figure is drawn by the form on the label row. For VoiceOver the row is one adjustable
 * element: increment and decrement go through `stepStars`.
 */
export function StarInput({ value, onChange }: { value: number; onChange: (next: number) => void }) {
  const figure = formatStars(value);

  function tap(star: number, side: StarSide) {
    const next = starFromTap(star, side);
    if (next !== value) onChange(next);
  }

  function onAction(event: AccessibilityActionEvent) {
    const name = event.nativeEvent.actionName;
    if (name === 'increment') onChange(stepStars(value, 1));
    else if (name === 'decrement') onChange(stepStars(value, -1));
  }

  return (
    <View
      accessible
      accessibilityRole="adjustable"
      accessibilityLabel="Rating"
      accessibilityValue={{ text: figure === '' ? 'not rated' : `${figure} out of ${STAR_COUNT}` }}
      accessibilityActions={[{ name: 'increment' }, { name: 'decrement' }]}
      onAccessibilityAction={onAction}
      style={{ flexDirection: 'row', height: minTouchTarget, marginLeft: FLUSH }}
    >
      {starGlyphs(value).map((glyph, index) => {
        const star = index + 1;
        return (
          <View
            key={star}
            style={{ width: SLOT, height: minTouchTarget, alignItems: 'center', justifyContent: 'center' }}
          >
            <StarGlyphIcon glyph={glyph} size={GLYPH} />
            <Pressable
              accessible={false}
              onPress={() => tap(star, 'left')}
              hitSlop={{ top: 6, bottom: 6 }}
              style={{ position: 'absolute', left: 0, top: 0, width: SLOT / 2, height: minTouchTarget }}
            />
            <Pressable
              accessible={false}
              onPress={() => tap(star, 'right')}
              hitSlop={{ top: 6, bottom: 6 }}
              style={{ position: 'absolute', right: 0, top: 0, width: SLOT / 2, height: minTouchTarget }}
            />
          </View>
        );
      })}
    </View>
  );
}
