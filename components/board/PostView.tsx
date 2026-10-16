import { useState } from 'react';
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { CommentBar, CommentList, CommentsOffBar, CommentsRule, QuietNote } from '@/components/board/Comments';
import { capsText } from '@/components/board/PostCard';
import { usePost } from '@/components/board/usePost';
import { Gallery } from '@/components/entry/EntryView';
import { ReactionPicker } from '@/components/entry/ReactionPicker';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import type { Post } from '@/lib/api/board';
import { firstName } from '@/lib/api/social';
import { REACTIONS } from '@/lib/entry/reactions';
import { starsSpoken } from '@/lib/entry/stars';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/** The photos of someone else's post are wider than high, as the developer's picture 10 draws them. */
const PHOTO_ASPECT = 3 / 2;
const AUTHOR_AVATAR = 28;
const STAR_SIZE = 18;
const REACTION_SIZE = 22;

function Details({
  post,
  myId,
  onReact,
  onDelete,
}: {
  post: Post;
  myId: string | null;
  onReact: (next: Post['card']['my_reaction']) => void;
  onDelete: (id: string) => Promise<string | null>;
}) {
  const card = post.card;
  const own = REACTIONS.find((item) => item.name === card.reaction) ?? null;
  const name = card.mine ? 'You' : firstName(card.author);
  const meta = [card.category.name, card.posted_local_time, card.late ? 'Late' : ''].filter((part) => part !== '').join(' · ');
  const spokenTitle = own ? `${card.title}. ${starsSpoken(card.stars)}. Their reaction: ${own.label}.` : `${card.title}. ${starsSpoken(card.stars)}.`;
  const comments = post.comments ?? [];

  return (
    <View>
      {/* The author row: the round initial, the first name, and the category with the time. */}
      <View
        accessible
        accessibilityLabel={`${name}. ${meta.replace(/ · /g, '. ')}.`}
        style={{ paddingHorizontal: space.lg, paddingVertical: space.sm, flexDirection: 'row', alignItems: 'center' }}
      >
        <SizedAvatar size={AUTHOR_AVATAR} name={card.author.name || card.author.username} uri={card.author.photo_url} placeholder="initials" />
        <View style={{ marginLeft: space.sm, flexShrink: 1 }}>
          <AppText role="button" numberOfLines={1}>
            {name}
          </AppText>
        </View>
        <View style={{ flex: 1, marginLeft: space.md, alignItems: 'flex-end' }}>
          <Text numberOfLines={1} style={{ ...capsText, color: color.muted }}>
            {meta}
          </Text>
        </View>
      </View>

      <Gallery photos={post.photos} title={card.title} aspect={PHOTO_ASPECT} />

      <View style={{ paddingHorizontal: space.lg, paddingTop: space.md, paddingBottom: space.xl }}>
        <View accessible accessibilityLabel={spokenTitle} style={{ flexDirection: 'row', alignItems: 'flex-start' }}>
          <View style={{ flex: 1 }}>
            <AppText role="heading" numberOfLines={2}>
              {card.title}
            </AppText>
          </View>
          <View style={{ marginLeft: space.md, flexDirection: 'row', alignItems: 'center', minHeight: type.heading.lineHeight }}>
            <StarDisplay value={card.stars} size={STAR_SIZE} decorative />
            {own ? (
              <Text allowFontScaling={false} style={{ marginLeft: space.sm, fontSize: REACTION_SIZE, lineHeight: type.heading.lineHeight }}>
                {own.char}
              </Text>
            ) : null}
          </View>
        </View>

        {post.note !== null ? (
          <View style={{ marginTop: space.xs }}>
            <Text
              maxFontSizeMultiplier={1.3}
              style={{ fontFamily: type.note.fontFamily, fontSize: type.body.fontSize, lineHeight: type.body.lineHeight, color: color.ink }}
            >
              {post.note}
            </Text>
          </View>
        ) : null}

        {post.can_react && !card.mine ? (
          <View style={{ marginTop: space.md, paddingTop: space.md, borderTopWidth: 1, borderTopColor: color.divider }}>
            <View accessibilityLabel="Your reaction">
              <ReactionPicker value={card.my_reaction} onChange={onReact} />
            </View>
          </View>
        ) : null}

        <View style={{ marginTop: space.md }}>
          <CommentsRule count={card.comment_count ?? comments.length} />
        </View>
        <CommentList comments={comments} myId={myId} onDelete={onDelete} />
      </View>
    </View>
  );
}

/**
 * Someone else's post (05-UI-SPEC section 4, the developer's picture 10), pushed in the Board's
 * stack with no bottom bar: the Back row, the author row, the photos edge to edge (swipe for more),
 * the title with the stars and their reaction, their note, the row of six reactions (yours ringed;
 * tap to set, tap again to clear), the comments oldest first, and the comment bar at the bottom.
 * When the owner switched commenting off the bar is replaced by 'Commenting is off.'.
 */
export function PostView({ id, backName }: { id: string; backName: string }) {
  const router = useRouter();
  const { user } = useSession();
  const load = usePost(id);
  const [message, setMessage] = useState('');

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/board');
  }

  async function onReact(next: Post['card']['my_reaction']) {
    setMessage('');
    const failure = await load.react(next);
    if (failure !== null) setMessage(failure);
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
            <Details post={post} myId={user?.id ?? null} onReact={(next) => void onReact(next)} onDelete={load.remove} />
          </ScrollView>
          {message !== '' ? (
            <View style={{ paddingHorizontal: space.lg, paddingVertical: space.sm }}>
              <QuietNote text={message} />
            </View>
          ) : null}
          {post.can_comment ? <CommentBar onSend={load.comment} /> : <CommentsOffBar />}
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
