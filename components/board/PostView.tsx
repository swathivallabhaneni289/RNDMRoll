import { useState } from 'react';
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { CommentBar, CommentList, CommentsOffBar } from '@/components/board/Comments';
import { usePost } from '@/components/board/usePost';
import { Gallery } from '@/components/entry/EntryView';
import { ActionRow, CaptionText, POST_PHOTO_ASPECT, type CaptionPost } from '@/components/entry/PostCaption';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import type { Comment, Post } from '@/lib/api/board';
import { firstName } from '@/lib/api/social';
import { useOpenPerson } from '@/lib/people/openPerson';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

const AUTHOR_AVATAR = 32;

function Details({
  post,
  myId,
  onDelete,
  onReply,
  onComment,
}: {
  post: Post;
  myId: string | null;
  onDelete: (id: string) => Promise<string | null>;
  onReply: (comment: Comment) => void;
  onComment?: () => void;
}) {
  const card = post.card;
  const openPerson = useOpenPerson();
  const name = card.mine ? 'You' : firstName(card.author);
  const comments = post.comments ?? [];
  const caption: CaptionPost = {
    title: card.title,
    stars: card.stars,
    reaction: card.reaction,
    comment: post.note,
    local_date: card.local_date,
    late: card.late,
    category: card.category,
  };

  return (
    <View>
      {/* The author row, like the top of a post in the Profile list: the round photo and the first name. A tap opens their profile. */}
      <Pressable
        onPress={() => openPerson(card.author.id)}
        accessibilityRole="button"
        accessibilityLabel={name}
        accessibilityHint={card.mine ? 'Opens your profile.' : 'Opens their profile.'}
        style={{ height: minTouchTarget, paddingHorizontal: space.lg, flexDirection: 'row', alignItems: 'center' }}
      >
        <SizedAvatar size={AUTHOR_AVATAR} name={card.author.name || card.author.username} uri={card.author.photo_url} placeholder="initials" />
        <View style={{ marginLeft: space.sm, flexShrink: 1 }}>
          <AppText role="button" numberOfLines={1}>
            {name}
          </AppText>
        </View>
      </Pressable>

      <Gallery photos={post.photos} title={card.title} aspect={POST_PHOTO_ASPECT} />
      <ActionRow entry={caption} onComment={onComment} whose={card.mine ? 'your' : 'their'} />
      <CaptionText entry={caption} time={card.posted_local_time} />

      <View style={{ paddingHorizontal: space.lg, paddingBottom: space.xl }}>
        {comments.length > 0 ? <View style={{ marginTop: space.md, height: 1, backgroundColor: color.divider }} /> : null}
        <CommentList comments={comments} myId={myId} onDelete={onDelete} onReply={post.can_comment ? onReply : undefined} />
      </View>
    </View>
  );
}

/**
 * Someone else's post (05-UI-SPEC section 4, the developer's picture 10), pushed in the Board's
 * stack with no bottom bar: the Back row, the author row, the photos edge to edge (swipe for more),
 * then the same parts as your own posts (developer, 2026-10-18: every post reads the same): the
 * ActionRow (their stars and emoji, and the comment mark, which puts the cursor in the comment
 * box) and the CaptionText with the posted time, then a hairline and the comments oldest first
 * with their replies under each (no count line; no reactions on posts), and
 * the comment bar at the bottom ('Reply' on a comment turns it into a reply). When the owner
 * switched commenting off the bar is replaced by 'Commenting is off.' and 'Reply' is not shown.
 * The Spin stack mounts it too (`place` 'today', a reply notification on someone else's post).
 */
export function PostView({ id, backName, place = 'board' }: { id: string; backName: string; place?: 'board' | 'today' }) {
  const router = useRouter();
  const { user } = useSession();
  const load = usePost(id);
  // A new number puts the cursor in the comment box (the comment mark under the photo).
  const [focusKey, setFocusKey] = useState(0);
  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace(place === 'today' ? '/(app)/today' : '/(app)/board');
  }

  const post = load.post;
  const ready = load.status === 'ready' && post !== null;

  return (
    <SafeAreaView edges={['top']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, justifyContent: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel={`Back to ${backName}`}
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="body">{backName}</AppText>
          </View>
        </Pressable>
      </View>

      {ready ? (
        <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
          <ScrollView style={{ flex: 1 }} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>
            <Details
              post={post}
              myId={user?.id ?? null}
              onDelete={load.remove}
              onReply={load.startReply}
              onComment={post.can_comment ? () => setFocusKey((n) => n + 1) : undefined}
            />
          </ScrollView>
          {post.can_comment ? (
            <CommentBar onSend={load.comment} replyTo={load.replyTo} onCancelReply={load.cancelReply} focusKey={focusKey} />
          ) : (
            <CommentsOffBar />
          )}
        </KeyboardAvoidingView>
      ) : (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          {load.status === 'loading' ? (
            <AppText role="label" tone="muted">
              Loading this post.
            </AppText>
          ) : load.status === 'offline' ? (
            <StatusMessage kind={load.failure} onRetry={load.reload} />
          ) : (
            <View style={{ alignItems: 'flex-start' }}>
              <AppText role="body">This post isn't here any more.</AppText>
              <View style={{ marginTop: space.lg }}>
                <TextButton label={`Back to ${backName}`} onPress={goBack} />
              </View>
            </View>
          )}
        </View>
      )}
    </SafeAreaView>
  );
}
