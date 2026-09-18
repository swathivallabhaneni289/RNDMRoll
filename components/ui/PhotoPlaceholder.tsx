import { View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { color } from '@/lib/theme/tokens';

/** No 30%/20%-Ink token exists in the locked token set; declared here rather than
 * editing plan 01-02's tokens module for a single-use figure. */
const COVER_GLYPH_OPACITY = 0.2;
const PANEL_GLYPH_OPACITY = 0.3;

/**
 * Secondary-fill photo stand-in with a centered image-outline glyph.
 * Phase 1 has no photo pipeline, so this takes no image source prop at
 * all: it is a declared layout stand-in real photography later replaces
 * by filling the same box. Hidden from screen readers — it carries no
 * information they need.
 */
export function PhotoPlaceholder({ variant }: { variant: 'cover' | 'panel' }) {
  const isCover = variant === 'cover';

  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={{
        flex: 1,
        backgroundColor: color.secondary,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Ionicons
        name="image-outline"
        size={isCover ? 64 : 32}
        color={color.ink}
        style={{ opacity: isCover ? COVER_GLYPH_OPACITY : PANEL_GLYPH_OPACITY }}
      />
    </View>
  );
}
