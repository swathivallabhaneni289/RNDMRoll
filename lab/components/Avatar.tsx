import { View } from 'react-native';
import { color, radius, space } from '@/lib/theme/tokens';
import { LabText } from './LabText';

/** The avatar in a post header, and the smaller one beside a comment. */
export const AVATAR_SIZE = 36;
export const AVATAR_SIZE_SMALL = 28;
/** Space between an avatar and the text beside it. */
export const AVATAR_GAP = space.sm + space.xs;

/**
 * A person as one initial on a plain circle. The only circle in the lab: everything
 * else stays square-ish (radius sm or md), so this is the one file that reads the
 * circular radius token. The circle is the secondary surface with ink on it, no ring
 * and no colour per person, in keeping with the monochrome ground.
 *
 * Hidden from screen readers: the name always sits next to it as text.
 */
export function Avatar({ name, size = AVATAR_SIZE }: { name: string; size?: number }) {
  const initial = name.trim().charAt(0).toUpperCase();
  const large = size >= AVATAR_SIZE;
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={{
        width: size,
        height: size,
        borderRadius: radius.full,
        backgroundColor: color.secondary,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <LabText role={large ? 'button' : 'label'} strong={!large}>
        {initial}
      </LabText>
    </View>
  );
}
