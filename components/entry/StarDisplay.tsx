import { View } from 'react-native';
import { Path, Svg } from 'react-native-svg';
import { starGlyphs, starsSpoken, type StarGlyph } from '@/lib/entry/stars';
import { color, space } from '@/lib/theme/tokens';

// The star is drawn once in a 34 by 34 box and scaled to any size. Ten vertices: even ones are
// the points, odd ones the notches, vertex 0 is the top point.
const BOX = 34;
const OUTER = 17;
const INNER = OUTER * 0.45;
const CX = BOX / 2;
const CY = (BOX - OUTER * (1 + Math.sin((54 * Math.PI) / 180))) / 2 + OUTER;

function vertex(k: number): string {
  const angle = ((-90 + 36 * k) * Math.PI) / 180;
  const r = k % 2 === 0 ? OUTER : INNER;
  return `${(CX + r * Math.cos(angle)).toFixed(2)} ${(CY + r * Math.sin(angle)).toFixed(2)}`;
}

function outline(order: number[]): string {
  return `M ${order.map((k) => vertex(k)).join(' L ')} Z`;
}

const FULL_PATH = outline([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
/** The left half of the star: the top point, the left side and the bottom notch. */
const HALF_PATH = outline([0, 9, 8, 7, 6, 5]);

/** One star: `ink` when full, `divider` when empty, a half is the left half in `ink` over a light star. */
export function StarGlyphIcon({ glyph, size }: { glyph: StarGlyph; size: number }) {
  return (
    <Svg width={size} height={size} viewBox={`0 0 ${BOX} ${BOX}`}>
      <Path d={FULL_PATH} fill={glyph === 'full' ? color.ink : color.divider} />
      {glyph === 'half' ? <Path d={HALF_PATH} fill={color.ink} /> : null}
    </Svg>
  );
}

/**
 * The read-only row of five stars (UI spec 3.3 and 9). One spoken element ('3.5 out of 5 stars');
 * `decorative` hides it from screen readers when a parent row already speaks the same words.
 */
export function StarDisplay({
  value,
  size = 20,
  decorative = false,
}: {
  value: number;
  size?: number;
  decorative?: boolean;
}) {
  return (
    <View
      accessible={!decorative}
      accessibilityRole="image"
      accessibilityLabel={starsSpoken(value)}
      accessibilityElementsHidden={decorative}
      importantForAccessibility={decorative ? 'no-hide-descendants' : 'yes'}
      style={{ flexDirection: 'row', alignItems: 'center' }}
    >
      {starGlyphs(value).map((glyph, index) => (
        <View key={index} style={{ marginLeft: index === 0 ? 0 : space.xs }}>
          <StarGlyphIcon glyph={glyph} size={size} />
        </View>
      ))}
    </View>
  );
}
