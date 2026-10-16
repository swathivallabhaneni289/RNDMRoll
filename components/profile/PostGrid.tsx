import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import type { Entry } from '@/lib/api/entries';
import { color, space, type } from '@/lib/theme/tokens';

/** Three tiles a row, close together (developer, 2026-10-17: bigger pictures, the thin gap is fine). */
const COLUMNS = 3;
const GAP = 2;
/** Tiles are 3 wide to 4 high, like Instagram's profile grid. */
const TILE_RATIO = 4 / 3;
/** The stacked-photos mark on a post with more than one picture. */
const MULTI_ICON = 18;
const MULTI_INSET = 6;

/**
 * Every post the person has made, as a grid of cover photos, newest first (developer,
 * 2026-10-16: the Diary tab and "Recent activity" are gone; all the pictures show on Profile).
 * Skipped days are not shown. A post with more than one picture carries a small stacked-photos
 * mark at its top right. A tile opens the posts list at that post. A post without a cover shows its title on a plain grey tile.
 */
export function PostGrid({
  posts,
  width,
  onOpen,
}: {
  posts: Entry[];
  /** The width the grid fills (the full page width: it runs edge to edge). */
  width: number;
  onOpen: (id: string) => void;
}) {
  if (posts.length === 0) {
    return (
      <View style={{ paddingHorizontal: space.lg }}>
        <AppText role="label" tone="muted">
          Your posts will show here.
        </AppText>
      </View>
    );
  }

  const tile = Math.floor((width - GAP * (COLUMNS - 1)) / COLUMNS);
  const tileHeight = Math.round(tile * TILE_RATIO);

  return (
    <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: GAP }}>
      {posts.map((post) => (
        <Pressable
          key={post.id}
          onPress={() => onOpen(post.id)}
          accessibilityRole="button"
          accessibilityLabel={`${post.category.name}. ${post.title}.`}
          accessibilityHint="Opens the post."
          style={{ width: tile, height: tileHeight, backgroundColor: color.secondary }}
        >
          {post.photo_url !== '' ? (
            <Image source={{ uri: post.photo_url }} style={{ width: '100%', height: '100%' }} contentFit="cover" />
          ) : (
            <View style={{ flex: 1, padding: space.sm, justifyContent: 'flex-end' }}>
              <Text
                numberOfLines={3}
                maxFontSizeMultiplier={1.3}
                style={{
                  fontFamily: type.label.fontFamily,
                  fontSize: type.label.fontSize,
                  lineHeight: type.label.lineHeight,
                  color: color.ink,
                }}
              >
                {post.title}
              </Text>
            </View>
          )}
          {post.photos.length > 1 ? (
            <View pointerEvents="none" style={{ position: 'absolute', top: MULTI_INSET, right: MULTI_INSET }}>
              <Ionicons name="copy" size={MULTI_ICON} color={color.dominant} />
            </View>
          ) : null}
        </Pressable>
      ))}
    </View>
  );
}
