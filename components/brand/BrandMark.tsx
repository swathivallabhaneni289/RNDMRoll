import { Circle, Svg } from 'react-native-svg';
import { color as tokenColor } from '@/lib/theme/tokens';

/**
 * The five-pip quincunx brand mark: five solid circles on a fixed 40x40
 * viewBox, die-face-five layout. Carries no elevation, no fill container,
 * and no border, per UI-SPEC's Focal Points section: the mark must not
 * outweigh the CTA group on the choose-method screen.
 */
export function BrandMark({
  size = 64,
  color = tokenColor.ink,
}: {
  size?: number;
  color?: string;
}) {
  return (
    <Svg viewBox="0 0 40 40" width={size} height={size}>
      <Circle cx={8} cy={8} r={4} fill={color} opacity={1} />
      <Circle cx={32} cy={8} r={4} fill={color} opacity={1} />
      <Circle cx={20} cy={20} r={4} fill={color} opacity={1} />
      <Circle cx={8} cy={32} r={4} fill={color} opacity={1} />
      <Circle cx={32} cy={32} r={4} fill={color} opacity={1} />
    </Svg>
  );
}
