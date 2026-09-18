import { Defs, LinearGradient, Rect, Stop, Svg } from 'react-native-svg';
import { color } from '@/lib/theme/tokens';

export const SCRIM_TOP_SAFE_FRACTION = 0.08;
export const SCRIM_BOTTOM_SAFE_FRACTION = 0.28;
export const SCRIM_TOP_FADE_DEPTH = 0.12;

/**
 * The dual-edge opaque-zone Ink scrim for the full-bleed Welcome cover
 * screen. Both flat zones are 100% opaque Ink, never a translucent
 * blend, so content placed inside them inherits the already-verified
 * Dominant-on-Ink contrast ratio regardless of whatever photograph
 * eventually sits beneath. Only the top zone's height is parameterized:
 * the caller may raise the flat top zone to whatever its content needs
 * (e.g. to clear a large status-bar inset) while the fade band keeps its
 * declared depth by moving with it. The bottom zone takes no such prop.
 */
export function PhotoScrim({
  topOpaqueFraction = SCRIM_TOP_SAFE_FRACTION,
}: {
  topOpaqueFraction?: number;
}) {
  return (
    <Svg
      style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
      pointerEvents="none"
      width="100%"
      height="100%"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    >
      <Defs>
        <LinearGradient id="scrim" x1="0" y1="0" x2="0" y2="1">
          <Stop offset={0} stopColor={color.ink} stopOpacity={1} />
          <Stop offset={topOpaqueFraction} stopColor={color.ink} stopOpacity={1} />
          <Stop offset={topOpaqueFraction + SCRIM_TOP_FADE_DEPTH} stopColor={color.ink} stopOpacity={0} />
          <Stop offset={0.45} stopColor={color.ink} stopOpacity={0} />
          <Stop offset={0.72} stopColor={color.ink} stopOpacity={1} />
          <Stop offset={1} stopColor={color.ink} stopOpacity={1} />
        </LinearGradient>
      </Defs>
      <Rect width="100%" height="100%" fill="url(#scrim)" />
    </Svg>
  );
}
