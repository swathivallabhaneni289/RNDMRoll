import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, Text, TextInput, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { AppText } from '@/components/ui/AppText';
import type { Comment } from '@/lib/api/board';
import { ApiError } from '@/lib/api/client';
import { canHide, hideComment, isHidden, unhideComment } from '@/lib/api/comments';
import { firstName } from '@/lib/api/social';
import { COMMENT_EMOJIS } from '@/lib/entry/reactions';
import { COMMENT_MAX } from '@/lib/entry/rules';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';
import { useOpenPerson } from '@/lib/people/openPerson';

const AVATAR = 28;
/** A reply's round initial is smaller, and the reply sits one step in under its comment. */
const REPLY_AVATAR = 20;
const REPLY_INSET = space.xl;
/** The close mark on the 'Replying to' line. */
const REPLY_CLOSE = 18;
const FIELD_HEIGHT = 44;
/** The characters on the quick row above the comment box. */
const QUICK_EMOJI = 24;

/** 'Reply' and 'Hide' under a comment: the label size in ink, in the button weight, no underline. */
const actionText = { fontFamily: type.button.fontFamily, fontSize: type.label.fontSize, lineHeight: type.label.lineHeight, color: color.ink } as const;

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

/**
 * The comments, oldest first (05-UI-SPEC 4.1): the round initial, the first name ('You' for your
 * own), the server's time label and the text, then 'Reply' (while commenting is on) and 'Hide' or
 * 'Unhide' (someone's comment on your own post; a hidden one reads 'Hidden' after its time and its
 * text is muted) side by side under it. Delete, where the server says the caller may, is a long
 * press on the comment (developer, 2026-10-18, like Instagram), asked first with the phone's own
 * alert (no red button, no fade); deleting a comment takes its replies with it. Replies come in the
 * server's order, right after their comment, one step in with a smaller initial and no rule above.
 */
export function CommentList({
  comments,
  myId,
  onDelete,
  onReply,
}: {
  comments: Comment[];
  myId: string | null;
  onDelete: (id: string) => Promise<string | null>;
  onReply?: (comment: Comment) => void;
}) {
  const openPerson = useOpenPerson();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  // Hide and Unhide answer at once on the row; the next read of the post agrees.
  const [hiddenNow, setHiddenNow] = useState<Record<string, boolean>>({});

  /** Hide or show someone's comment on your own post. No question first, like Instagram. */
  function toggleHidden(comment: Comment, hide: boolean) {
    if (busyId !== null) return;
    setBusyId(comment.id);
    setMessage('');
    void (hide ? hideComment(comment.id) : unhideComment(comment.id))
      .then(() => setHiddenNow((now) => ({ ...now, [comment.id]: hide })))
      .catch((err: unknown) =>
        setMessage(
          err instanceof ApiError && err.code === 'network_unavailable'
            ? err.userMessage
            : 'Something went wrong. Please try again.',
        ),
      )
      .finally(() => setBusyId(null));
  }

  function askDelete(comment: Comment) {
    if (busyId !== null) return;
    const hasReplies = comment.parent_id === null && comments.some((item) => item.parent_id === comment.id);
    const title = comment.parent_id !== null ? 'Delete this reply?' : 'Delete this comment?';
    const detail = hasReplies ? 'It will be gone for good, with its replies.' : 'It will be gone for good.';
    Alert.alert(title, detail, [
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
        const isReply = comment.parent_id !== null;
        const kind = isReply ? 'reply' : 'comment';
        const hidden = hiddenNow[comment.id] ?? isHidden(comment);
        return (
          <View
            key={comment.id}
            style={{
              flexDirection: 'row',
              marginLeft: isReply ? REPLY_INSET : 0,
              paddingTop: isReply ? space.xs : space.md,
              paddingBottom: space.md,
              borderTopWidth: index === 0 || isReply ? 0 : 1,
              borderTopColor: color.divider,
            }}
          >
            <Pressable
              onPress={() => openPerson(comment.author.id)}
              accessibilityRole="button"
              accessibilityLabel={mine ? 'Your profile' : `${name}'s profile`}
              hitSlop={6}
              style={{ alignSelf: 'flex-start' }}
            >
              <SizedAvatar
                size={isReply ? REPLY_AVATAR : AVATAR}
                name={comment.author.name || comment.author.username}
                uri={comment.author.photo_url}
                placeholder="initials"
              />
            </Pressable>
            <Pressable
              onLongPress={comment.can_delete ? () => askDelete(comment) : undefined}
              delayLongPress={400}
              disabled={busyId !== null}
              accessible={false}
              style={{ flex: 1, marginLeft: space.sm, opacity: busyId === comment.id ? 0.5 : 1 }}
            >
              {/* One element for a screen reader: who, when and the words, with Delete as an action. */}
              <View
                accessible
                accessibilityLabel={`${isReply ? 'Reply. ' : ''}${name}, ${hidden ? `${comment.at_label}, hidden` : comment.at_label}: ${comment.text}`}
                accessibilityHint={comment.can_delete ? 'Press and hold to delete.' : undefined}
                accessibilityActions={comment.can_delete ? [{ name: 'delete', label: 'Delete' }] : undefined}
                onAccessibilityAction={(event) => {
                  if (event.nativeEvent.actionName === 'delete') askDelete(comment);
                }}
              >
                <View style={{ flexDirection: 'row', alignItems: 'baseline' }}>
                  <Pressable onPress={() => openPerson(comment.author.id)} hitSlop={6} style={{ flexShrink: 1 }}>
                    <AppText role="button" numberOfLines={1}>
                      {name}
                    </AppText>
                  </Pressable>
                  <View style={{ marginLeft: space.sm }}>
                    <AppText role="label" tone="muted">
                      {hidden ? `${comment.at_label} · Hidden` : comment.at_label}
                    </AppText>
                  </View>
                </View>
                <View importantForAccessibility="no-hide-descendants" accessibilityElementsHidden>
                  <AppText role="body" tone={hidden ? 'muted' : undefined}>
                    {comment.text.replace(/\s*\n\s*/g, ' ')}
                  </AppText>
                </View>
              </View>
              {onReply || canHide(comment) ? (
                // Reply and Hide side by side in one plain style, never underlined (developer,
                // 2026-10-18); Delete is a long press on the comment, with the phone's alert first.
                <View style={{ flexDirection: 'row', marginTop: space.xs, columnGap: space.lg }}>
                  {onReply ? (
                    <Pressable
                      onPress={() => onReply(comment)}
                      accessibilityRole="button"
                      accessibilityLabel={`Reply to ${mine ? 'your' : `${name}'s`} ${kind}`}
                      hitSlop={(minTouchTarget - type.label.lineHeight) / 2}
                    >
                      <Text style={actionText}>Reply</Text>
                    </Pressable>
                  ) : null}
                  {canHide(comment) ? (
                    <Pressable
                      onPress={() => toggleHidden(comment, !hidden)}
                      disabled={busyId !== null}
                      accessibilityRole="button"
                      accessibilityLabel={`${hidden ? 'Unhide' : 'Hide'} ${kind} by ${name}`}
                      accessibilityHint={hidden ? undefined : 'Only they and you will see it. They are not told.'}
                      hitSlop={(minTouchTarget - type.label.lineHeight) / 2}
                    >
                      <Text style={actionText}>{hidden ? 'Unhide' : 'Hide'}</Text>
                    </Pressable>
                  ) : null}
                </View>
              ) : null}
            </Pressable>
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
 *
 * Reply mode (`replyTo` set, from a comment's 'Reply'): a small line 'Replying to {name}' with a
 * close mark sits above the field, the field gets '@username ' in front and the keyboard comes up;
 * the send is a reply. The close mark ends reply mode (and takes away the '@username ' if nothing
 * else was typed).
 */
export function CommentBar({
  onSend,
  replyTo = null,
  onCancelReply,
  focusKey = 0,
}: {
  onSend: (text: string) => Promise<string | null>;
  replyTo?: Comment | null;
  onCancelReply?: () => void;
  /** A new number puts the cursor in the field (the comment mark under a post was tapped). */
  focusKey?: number;
}) {
  const insets = useSafeAreaInsets();
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState('');
  const sendingRef = useRef(false);
  const inputRef = useRef<TextInput>(null);
  const canSend = text.trim().length > 0 && text.trim().length <= COMMENT_MAX && !sending;
  const mention = replyTo !== null && replyTo.author.username !== '' ? `@${replyTo.author.username}` : '';

  // Each Reply tap is a new object, so tapping Reply again refills and focuses the field.
  useEffect(() => {
    if (focusKey > 0) inputRef.current?.focus();
  }, [focusKey]);

  useEffect(() => {
    if (replyTo === null) return;
    const username = replyTo.author.username;
    if (username !== '') setText((before) => `@${username} ${before.replace(/^@\S+\s*/, '')}`.slice(0, COMMENT_MAX));
    inputRef.current?.focus();
  }, [replyTo]);

  // A tap on the quick row adds the character at the end, never past the limit.
  function addEmoji(char: string) {
    setText((before) => (before.length + char.length > COMMENT_MAX ? before : before + char));
    inputRef.current?.focus();
  }

  function cancelReply() {
    if (mention !== '' && text.trim() === mention) setText('');
    onCancelReply?.();
  }

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

  const replyName = replyTo !== null ? firstName(replyTo.author) || replyTo.author.username : '';

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
      {replyTo !== null ? (
        <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: space.sm }}>
          <View style={{ flex: 1 }} accessibilityLiveRegion="polite">
            <AppText role="label" tone="muted" numberOfLines={1}>
              {`Replying to ${replyName}`}
            </AppText>
          </View>
          <Pressable
            onPress={cancelReply}
            accessibilityRole="button"
            accessibilityLabel={`Stop replying to ${replyName}`}
            hitSlop={(minTouchTarget - REPLY_CLOSE) / 2}
            style={{ marginLeft: space.sm }}
          >
            <Ionicons name="close" size={REPLY_CLOSE} color={color.muted} />
          </Pressable>
        </View>
      ) : null}
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: space.sm }}>
        {COMMENT_EMOJIS.map((emoji) => (
          <Pressable
            key={emoji.char}
            onPress={() => addEmoji(emoji.char)}
            accessibilityRole="button"
            accessibilityLabel={`Add ${emoji.label}`}
            style={{ minHeight: minTouchTarget, minWidth: 36, alignItems: 'center', justifyContent: 'center' }}
          >
            <Text allowFontScaling={false} style={{ fontSize: QUICK_EMOJI, lineHeight: QUICK_EMOJI * 1.25 }}>
              {emoji.char}
            </Text>
          </Pressable>
        ))}
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center' }}>
        <TextInput
          ref={inputRef}
          value={text}
          onChangeText={setText}
          placeholder={replyTo !== null ? 'Add a reply' : 'Add a comment'}
          placeholderTextColor={color.muted}
          accessibilityLabel={replyTo !== null ? `Reply to ${replyName}` : 'Add a comment'}
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
          accessibilityLabel={replyTo !== null ? 'Post reply' : 'Post comment'}
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
