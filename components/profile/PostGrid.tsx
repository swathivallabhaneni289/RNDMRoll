import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { AppText } from '@/components/ui/AppText';
import type { Entry } from '@/lib/api/entries';
import { color, space, type } from '@/lib/theme/tokens';

/** Three square tiles a row with a thin gap, like Instagram's profile grid. */
const COLUMNS = 3;
const GAP = 2;

/**
 * Every post the person has made, as a grid of cover photos, newest first (developer,
 * 2026-10-16: the Diary tab and "Recent activity" are gone; all the pictures show on Profile).
 * Skipped days are not shown. A tile opens the entry page, whose three dots keep Edit, Archive,
 * commenting and Delete. A post without a cover shows its title on a plain grey tile.
 */
export function PostGrid({
  posts,
  width,
  onOpen,
}: {
  posts: Entry[];
  /** The width the grid fills (the page width inside its side margins). */
  width: number;
  onOpen: (id: string) => void;
}) {
  if (posts.length === 0) {
    return (
      <AppText role="label" tone="muted">
        Your posts will show here.
      </AppText>
    );
  }

  const tile = Math.floor((width - GAP * (COLUMNS - 1)) / COLUMNS);

  return (
    <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: GAP }}>
      {posts.map((post) => (
        <Pressable
          key={post.id}
          onPress={() => onOpen(post.id)}
          accessibilityRole="button"
          accessibilityLabel={`${post.category.name}. ${post.title}.`}
          accessibilityHint="Opens the post."
          style={{ width: tile, height: tile, backgroundColor: color.secondary }}
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
        </Pressable>
      ))}
    </View>
  );
}
