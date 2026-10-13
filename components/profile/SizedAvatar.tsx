import { View } from 'react-native';
import { DialAvatar } from '@/components/brand/DialAvatar';

/** The width of `DialAvatar`'s photo disc when it is drawn without the dial ring. */
const DIAL_AVATAR_DISC = 104;

/**
 * The existing round avatar (`DialAvatar`, the Phase 1 named round shape) drawn at another size.
 * `DialAvatar` has one fixed size, and this page needs 72dp beside the name and 44dp in a list, so
 * the same component is scaled inside a box of the wanted size. Nothing here names a radius: the
 * circle is `DialAvatar`'s, which keeps the round-shape list as it is.
 *
 * The avatar is decoration beside the name, so the box is not an accessibility element of its own.
 */
export function SizedAvatar({
  size,
  name,
  uri,
  placeholder = 'camera',
}: {
  size: number;
  name: string;
  uri?: string | null;
  placeholder?: 'initials' | 'camera';
}) {
  return (
    <View style={{ width: size, height: size, alignItems: 'center', justifyContent: 'center' }}>
      <View style={{ transform: [{ scale: size / DIAL_AVATAR_DISC }] }}>
        <DialAvatar name={name} uri={uri} placeholder={placeholder} ring={false} />
      </View>
    </View>
  );
}
