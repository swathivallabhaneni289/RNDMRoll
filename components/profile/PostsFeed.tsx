import { useRef, useState, type ReactNode } from 'react';
import { Pressable, ScrollView, Text, View, type NativeScrollEvent, type NativeSyntheticEvent } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useDiary } from '@/components/diary/useDiary';
import { Gallery } from '@/components/entry/EntryView';
import { ActionRow, CaptionText, POST_PHOTO_ASPECT, captionBold, captionLine } from '@/components/entry/PostCaption';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { useEntryMenu } from '@/components/entry/useEntryMenu';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { MenuSheet } from '@/components/ui/MenuSheet';
import type { LatestComment, PostSocial } from '@/lib/api/diary';
import type { Entry } from '@/lib/api/entries';
import { firstName } from '@/lib/api/social';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/** Your small round photo at the left of each post's top row. */
const AVATAR = 32;

/** The small round photo beside a comment and beside 'Add a comment'. */
const COMMENT_AVATAR = 24;

export type PostMe = { name: string; photo: string | null };

/** A small round photo and one line beside it: a comment, or 'Add a comment'. A tap opens the post. */
function PhotoLine({
  photo,
  photoName,
  label,
  children,
  onPress,
}: {
  photo: string | null;
  photoName: string;
  label: string;
  children: ReactNode;
  onPress: () => void;
}) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={label} style={{ minHeight: minTouchTarget - space.sm, flexDirection: 'row', alignItems: 'center' }}>
      <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
        <SizedAvatar size={COMMENT_AVATAR} name={photoName} uri={photo} placeholder="initials" />
      </View>
      <Text numberOfLines={1} maxFontSizeMultiplier={1.3} style={[captionLine, { flex: 1, marginLeft: space.sm }]}>
        {children}
      </Text>
    </Pressable>
  );
}

function CommentLine({ comment, onPress }: { comment: LatestComment; onPress: () => void }) {
  const name = firstName(comment.author) || comment.author.username;
  return (
    <PhotoLine
      photo={comment.author.photo_url}
      photoName={comment.author.name || comment.author.username}
      label={`${name}: ${comment.text}`}
      onPress={onPress}
    >
      <Text style={captionBold}>{name}</Text>
      {` ${comment.text}`}
    </PhotoLine>
  );
}

/**
 * Everything under one post's photo in the list (developer, 2026-10-17 and 2026-10-18), in
 * Instagram's rhythm with this app's own parts: the ActionRow (stars left, comment mark right) and
 * the CaptionText, shared with the entry page; then, close under them, the comments: 'View all
 * comments' (no number) when there is more than one, the newest on one line with the writer's
 * photo, and 'Add a comment' with yours. Groups are set apart by space, not rules.
 */
export function PostDetails({
  entry,
  social,
  me,
  onOpenComments,
}: {
  entry: Entry;
  social: PostSocial;
  me: PostMe;
  onOpenComments: () => void;
}) {
  const latest = social.latest_comments;
  return (
    <View>
      <ActionRow entry={entry} onComment={onOpenComments} />
      <CaptionText entry={entry} />
      {/* Close under the caption; the link names no number, which can grow large (developer, 2026-10-18). */}
      <View style={{ paddingHorizontal: space.lg, marginTop: space.sm }}>
        {social.comment_count > latest.length ? (
          <Pressable onPress={onOpenComments} accessibilityRole="button" hitSlop={8} style={{ alignSelf: 'flex-start', marginBottom: space.xs }}>
            <Text maxFontSizeMultiplier={1.3} style={captionLine}>
              View all comments
            </Text>
          </Pressable>
        ) : null}
        {latest.map((comment, index) => (
          <CommentLine key={`${comment.author.id}-${index}`} comment={comment} onPress={onOpenComments} />
        ))}
        <PhotoLine photo={me.photo} photoName={me.name} label="Add a comment" onPress={onOpenComments}>
          Add a comment
        </PhotoLine>
      </View>
    </View>
  );
}

/**
 * Your posts as one scrolling list, opened from a picture in the Profile grid and placed at that
 * picture (developer, 2026-10-17: "how Instagram lets you see the pictures you posted"). Each post
 * has a top row with your photo, your username and its three dots (Edit, Archive, commenting,
 * Delete), the photos to swipe, and PostDetails under them. The diary sends each post's counts
 * and newest comment, so the list makes no read per post. The list pages as the end comes into view.
 */
export function PostsFeed({ startId }: { startId: string }) {
  const router = useRouter();
  const { user } = useSession();
  const name = user?.name || user?.username || '';
  const handle = user?.username ?? '';
  const diary = useDiary(null);
  const posts = diary.rows.flatMap((row) => (row.kind === 'entry' ? [{ entry: row.entry, social: row.social }] : []));
  const menu = useEntryMenu('profile', diary.reload);

  const scrollRef = useRef<ScrollView>(null);
  // Jump to the tapped post once, as soon as it has been laid out.
  const [placed, setPlaced] = useState(false);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/profile');
  }

  function handleScroll(event: NativeSyntheticEvent<NativeScrollEvent>) {
    const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
    if (contentOffset.y + layoutMeasurement.height * 2 >= contentSize.height) diary.loadMore();
  }

  function renderPost({ entry, social }: { entry: Entry; social: PostSocial }) {
    return (
      <View
        key={entry.id}
        onLayout={(event) => {
          if (placed || entry.id !== startId) return;
          scrollRef.current?.scrollTo({ y: event.nativeEvent.layout.y, animated: false });
          setPlaced(true);
        }}
        style={{ marginBottom: space.xl }}
      >
        <View
          style={{
            height: minTouchTarget,
            paddingHorizontal: space.lg,
            flexDirection: 'row',
            alignItems: 'center',
          }}
        >
          <View
            accessibilityElementsHidden
            importantForAccessibility="no-hide-descendants"
            style={{ flex: 1, flexDirection: 'row', alignItems: 'center' }}
          >
            <SizedAvatar size={AVATAR} name={name} uri={user?.avatar_url ?? null} placeholder="initials" />
            <View style={{ marginLeft: space.sm, flex: 1 }}>
              <AppText role="button" numberOfLines={1}>
                {handle}
              </AppText>
            </View>
          </View>
          <Pressable
            onPress={() => menu.open(entry)}
            disabled={menu.busy}
            accessibilityRole="button"
            accessibilityLabel={`More for ${entry.title}`}
            hitSlop={8}
            style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
          >
            <Ionicons name="ellipsis-horizontal" size={24} color={menu.busy ? color.muted : color.ink} />
          </Pressable>
        </View>
        <Gallery photos={entry.photos} title={entry.title} aspect={POST_PHOTO_ASPECT} />
        <PostDetails
          entry={entry}
          social={social}
          me={{ name, photo: user?.avatar_url ?? null }}
          onOpenComments={() => router.push({ pathname: '/(app)/profile/entry', params: { id: entry.id, from: 'profile' } })}
        />
      </View>
    );
  }

  return (
    <SafeAreaView edges={['top']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, justifyContent: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back to Profile"
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="body">Posts</AppText>
          </View>
        </Pressable>
      </View>

      {menu.message !== null ? (
        <View style={{ paddingHorizontal: space.lg, paddingVertical: space.sm }} accessibilityLiveRegion="polite">
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.ink,
              textAlign: 'center',
            }}
          >
            {menu.message}
          </Text>
        </View>
      ) : null}

      {diary.status === 'loading' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <AppText role="label" tone="muted">
            Loading your posts.
          </AppText>
        </View>
      ) : diary.status === 'offline' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <StatusMessage kind={diary.failure} onRetry={diary.retry} />
        </View>
      ) : (
        <ScrollView
          ref={scrollRef}
          onScroll={handleScroll}
          scrollEventThrottle={200}
          showsVerticalScrollIndicator={false}
        >
          {posts.map(renderPost)}
        </ScrollView>
      )}
      <MenuSheet visible={menu.visible} items={menu.items} onClose={menu.close} />
    </SafeAreaView>
  );
}
