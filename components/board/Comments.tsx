import { useRef, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, Text, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { AppText } from '@/components/ui/AppText';
import type { Comment, Received } from '@/lib/api/board';
import { firstName } from '@/lib/api/social';
import { commentCount } from '@/lib/board/text';
import { REACTIONS } from '@/lib/entry/reactions';
import { COMMENT_MAX } from '@/lib/entry/rules';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

const AVATAR = 28;
const FIELD_HEIGHT = 44;
const REACTION_SIZE = 22;

const labelText = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
} as const;

/** One quiet line in ink (never red): a failed send or delete, or 'Commenting is off.'. */
export function QuietNote({ text, tone = 'default' }: { text: string; tone?: 'default' | 'muted' }) {
  return (
    <View accessibilityLiveRegion="polite">
      <AppText role="label" tone={tone}>
        {text}
      </AppText>
    </View>
  );
}

/** The ink rule with '{n} comments' at its right (none at 0). */
export function CommentsRule({ count }: { count: number | null }) {
  const words = commentCount(count);
  return (
    <View>
      {words !== '' ? (
        <View style={{ alignItems: 'flex-end', marginBottom: space.xs }}>
          <AppText role="label" tone="muted">
            {words}
          </AppText>
        </View>
      ) : null}
      <View style={{ height: 1, backgroundColor: color.ink }} />
    </View>
  );
}

/**
 * The comments, oldest first (05-UI-SPEC 4.1): the round initial, the first name ('You' for your
 * own), the server's time label, 'Delete' where the server says the caller may, and the text. Delete
 * asks first with the phone's own alert (no red button, no fade).
 */
export function CommentList({
  comments,
  myId,
  onDelete,
}: {
  comments: Comment[];
  myId: string | null;
  onDelete: (id: string) => Promise<string | null>;
}) {
  const [busyId, setBusyId] = useState<string | null>(null);
  const [message, setMessage] = useState('');

  function askDelete(comment: Comment) {
    if (busyId !== null) return;
    Alert.alert('Delete this comment?', 'It will be gone for good.', [
      { text: 'Keep it', style: 'cancel' },
      {
        text: 'Delete',
        onPress: () => {
          setBusyId(comment.id);
          setMessage('');
          void onDelete(comment.id).then((failure) => {
            setBusyId(null);
            if (failure !== null) setMessage(failure);
          });
        },
      },
    ]);
  }

  if (comments.length === 0 && message === '') return null;

  return (
    <View>
      {comments.map((comment, index) => {
        const mine = myId !== null && comment.author.id === myId;
        const name = mine ? 'You' : firstName(comment.author);
        return (
          <View
            key={comment.id}
            style={{
              flexDirection: 'row',
              paddingVertical: space.md,
              borderTopWidth: index === 0 ? 0 : 1,
              borderTopColor: color.divider,
            }}
          >
            <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
              <SizedAvatar
                size={AVATAR}
                name={comment.author.name || comment.author.username}
                uri={comment.author.photo_url}
                placeholder="initials"
              />
            </View>
            <View style={{ flex: 1, marginLeft: space.sm }}>
              <View style={{ flexDirection: 'row', alignItems: 'center' }}>
                <View
                  accessible
                  accessibilityLabel={`${name}, ${comment.at_label}: ${comment.text}`}
                  style={{ flex: 1, flexDirection: 'row', alignItems: 'baseline' }}
                >
                  <AppText role="button" numberOfLines={1}>
                    {name}
                  </AppText>
                  <View style={{ marginLeft: space.sm }}>
                    <AppText role="label" tone="muted">
                      {comment.at_label}
                    </AppText>
                  </View>
                </View>
                {comment.can_delete ? (
                  <Pressable
                    onPress={() => askDelete(comment)}
                    disabled={busyId !== null}
                    accessibilityRole="button"
                    accessibilityLabel={`Delete comment by ${name}`}
                    hitSlop={(minTouchTarget - type.label.lineHeight) / 2}
                  >
                    <Text style={{ ...labelText, color: color.muted, textDecorationLine: 'underline' }}>
                      {busyId === comment.id ? 'Deleting' : 'Delete'}
                    </Text>
                  </Pressable>
                ) : null}
              </View>
              <View importantForAccessibility="no-hide-descendants" accessibilityElementsHidden>
                <AppText role="body">{comment.text.replace(/\s*\n\s*/g, ' ')}</AppText>
              </View>
            </View>
          </View>
        );
      })}
      {message !== '' ? (
        <View style={{ paddingBottom: space.sm }}>
          <QuietNote text={message} />
        </View>
      ) : null}
    </View>
  );
}

/**
 * The comment bar (05-UI-SPEC 4.2): a round field 'Add a comment' and the word 'Post'. It sends
 * once (a ref blocks a second tap) and clears only when the server took it; a failure keeps the
 * text and shows one quiet line above the bar. Pinned at the bottom of the page above the safe area.
 */
export function CommentBar({ onSend }: { onSend: (text: string) => Promise<string | null> }) {
  const insets = useSafeAreaInsets();
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState('');
  const sendingRef = useRef(false);
  const canSend = text.trim().length > 0 && text.trim().length <= COMMENT_MAX && !sending;

  async function send() {
    if (!canSend || sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    setMessage('');
    const failure = await onSend(text);
    sendingRef.current = false;
    setSending(false);
    if (failure === null) setText('');
    else setMessage(failure);
  }

  return (
    <View
      style={{
        backgroundColor: color.dominant,
        borderTopWidth: 1,
        borderTopColor: color.divider,
        paddingHorizontal: space.lg,
        paddingTop: space.md,
        paddingBottom: space.md + insets.bottom,
      }}
    >
      {message !== '' ? (
        <View style={{ marginBottom: space.sm }}>
          <QuietNote text={message} />
        </View>
      ) : null}
      <View style={{ flexDirection: 'row', alignItems: 'center' }}>
        <TextInput
          value={text}
          onChangeText={setText}
          placeholder="Add a comment"
          placeholderTextColor={color.muted}
          accessibilityLabel="Add a comment"
          maxLength={COMMENT_MAX}
          autoCapitalize="sentences"
          multiline
          returnKeyType="default"
          maxFontSizeMultiplier={1.3}
          style={{
            flex: 1,
            minHeight: FIELD_HEIGHT,
            maxHeight: FIELD_HEIGHT + 2 * type.body.lineHeight,
            borderWidth: 1,
            borderColor: color.inkRest,
            borderRadius: radius.full,
            paddingHorizontal: space.md,
            paddingTop: space.sm + 2,
            paddingBottom: space.sm + 2,
            fontFamily: type.body.fontFamily,
            fontSize: type.body.fontSize,
            color: color.ink,
          }}
        />
        <Pressable
          onPress={() => void send()}
          disabled={!canSend}
          accessibilityRole="button"
          accessibilityLabel="Post comment"
          accessibilityState={{ disabled: !canSend, busy: sending }}
          hitSlop={8}
          style={{ marginLeft: space.md, minHeight: minTouchTarget, minWidth: minTouchTarget, alignItems: 'center', justifyContent: 'center' }}
        >
          {sending ? (
            <ActivityIndicator color={color.ink} />
          ) : (
            <AppText role="button" tone={canSend ? 'default' : 'muted'}>
              Post
            </AppText>
          )}
        </Pressable>
      </View>
    </View>
  );
}

/** 'Commenting is off.' in the comment bar's place. */
export function CommentsOffBar() {
  const insets = useSafeAreaInsets();
  return (
    <View
      style={{
        borderTopWidth: 1,
        borderTopColor: color.divider,
        paddingHorizontal: space.lg,
        paddingTop: space.md,
        paddingBottom: space.md + insets.bottom,
        alignItems: 'center',
      }}
    >
      <QuietNote text="Commenting is off." tone="muted" />
    </View>
  );
}

/** Who reacted to your own post: a name and the character, or 'No reactions yet.' (05-UI-SPEC 5). */
export function ReceivedReactions({ received }: { received: Received[] }) {
  const items = received
    .map((item) => {
      const reaction = REACTIONS.find((r) => r.name === item.reaction);
      return reaction ? { key: item.person.id, name: firstName(item.person), char: reaction.char, label: reaction.label } : null;
    })
    .filter((item): item is { key: string; name: string; char: string; label: string } => item !== null);

  return (
    <View>
      <Text
        accessibilityRole="header"
        style={{ ...labelText, letterSpacing: 1.5, textTransform: 'uppercase', color: color.muted }}
      >
        Reactions
      </Text>
      <View style={{ marginTop: space.sm }}>
        {items.length === 0 ? (
          <AppText role="label" tone="muted">
            No reactions yet.
          </AppText>
        ) : (
          <View
            accessible
            accessibilityLabel={`Reactions: ${items.map((item) => `${item.name}, ${item.label}`).join('. ')}.`}
            style={{ flexDirection: 'row', flexWrap: 'wrap' }}
          >
            {items.map((item) => (
              <View key={item.key} style={{ flexDirection: 'row', alignItems: 'center', marginRight: space.md }}>
                <AppText role="body">{item.name}</AppText>
                <Text
                  allowFontScaling={false}
                  style={{ marginLeft: space.xs, fontSize: REACTION_SIZE, lineHeight: type.heading.lineHeight }}
                >
                  {item.char}
                </Text>
              </View>
            ))}
          </View>
        )}
      </View>
    </View>
  );
}
