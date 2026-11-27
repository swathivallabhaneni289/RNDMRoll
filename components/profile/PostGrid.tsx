import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import type { Entry } from '@/lib/api/entries';
import { photoSource } from '@/lib/api/photoSource';
import { color, space, type } from '@/lib/theme/tokens';

/**
 * Three tiles a row, close together, edge to edge (developer, 2026-10-17: bigger pictures, the thin
 * gap is fine; 2026-11-27, after trying the page's side margins: "it should look like how instagram
 * handles the profile page", whose grid runs edge to edge with 3:4 tiles).
 */
/**
 * A small even margin at both sides, so a tile never touches the screen's edge and looks cut off
 * (developer, 2026-11-27, picked over the full page margins, which made the pictures look
 * "compressed"). The tiles stay nearly as big as edge to edge.
 */
const SIDE = space.sm;
const COLUMNS = 3;
const GAP = 2;
/** Tiles are 3 wide to 4 high, like Instagram's profile grid. */
const TILE_RATIO = 4 / 3;
/** The stacked-photos mark on a post with more than one picture. */
const MULTI_ICON = 18;
const MULTI_INSET = 6;

/** What a tile reads: your own entries, and the tiles of another person's page, both fit. */
export type GridPost = Pick<Entry, 'id' | 'photo_url' | 'title'> & {
  category: { name: string };
  photos: { length: number };
};

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
  emptyText = 'Your posts will show here.',
  onPhotoError,
}: {
  posts: GridPost[];
  /** The page's full width; the grid sits inside it with a small margin at each side. */
  width: number;
  onOpen: (id: string) => void;
  /** The line shown when there is nothing to show. */
  emptyText?: string;
  /** A tile's photo would not load (its link may have run out): the page reads again, quietly. */
  onPhotoError?: () => void;
}) {
  if (posts.length === 0) {
    return (
      <View style={{ paddingHorizontal: space.lg }}>
        <AppText role="label" tone="muted">
          {emptyText}
        </AppText>
      </View>
    );
  }

  const tile = Math.floor((width - 2 * SIDE - GAP * (COLUMNS - 1)) / COLUMNS);
  const tileHeight = Math.round(tile * TILE_RATIO);

  return (
    <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: GAP, paddingHorizontal: SIDE }}>
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
            <Image
              source={photoSource(post.photo_url)}
              style={{ width: '100%', height: '100%' }}
              contentFit="cover"
              onError={onPhotoError}
            />
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
