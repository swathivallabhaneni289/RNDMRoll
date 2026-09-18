import { Circle, Defs, Pattern, Rect, Svg } from 'react-native-svg';
import { texture } from '@/lib/theme/tokens';

/**
 * The static, full-bleed dot-grid background texture, scoped to the
 * choose-method screen only (UI-SPEC "Background Texture"). Decorative:
 * no animation, no parallax, no transform, hidden from screen readers.
 */
export function BackgroundDotGrid() {
  const dotRadius = texture.dotDiameter / 2;

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
        <Pattern
          id="dotGrid"
          width={texture.spacing}
          height={texture.spacing}
          patternUnits="userSpaceOnUse"
        >
          <Circle cx={texture.spacing / 2} cy={texture.spacing / 2} r={dotRadius} fill={texture.dotColor} />
        </Pattern>
      </Defs>
      <Rect width="100%" height="100%" fill="url(#dotGrid)" />
    </Svg>
  );
}
