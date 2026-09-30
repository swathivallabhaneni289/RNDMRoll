import { View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { color } from '@/lib/theme/tokens';

type Props = {
  /** 0.5 to 5 in half-star steps. */
  value: number;
  size?: number;
};

type Glyph = 'star' | 'star-half' | 'star-outline';

function glyphFor(position: number, value: number): Glyph {
  if (value >= position) return 'star';
  if (value >= position - 0.5) return 'star-half';
  return 'star-outline';
}

/**
 * Read-only half-star rating drawn with Ionicons (the project draws every glyph from
 * @expo/vector-icons; text stars would break the no-emoji-as-icons rule). Filled,
 * half and outline stars differ by shape, not by colour, so the rating still reads
 * at a glance in monochrome and on any surface.
 */
export function StarRating({ value, size = 14 }: Props) {
  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={`${value} out of 5 stars`}
      style={{ flexDirection: 'row', alignItems: 'center', gap: 2 }}
    >
      {[1, 2, 3, 4, 5].map((position) => (
        <Ionicons key={position} name={glyphFor(position, value)} size={size} color={color.ink} />
      ))}
    </View>
  );
}
