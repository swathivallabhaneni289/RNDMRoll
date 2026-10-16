import { ReactNode, useEffect, useRef, useState } from 'react';
import { Alert, KeyboardAvoidingView, Platform, Pressable, ScrollView, Text, useWindowDimensions, View } from 'react-native';
import { Image } from 'expo-image';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { CommentBar, CommentList, CommentsOffBar, CommentsRule, ReceivedReactions } from '@/components/board/Comments';
import { usePost, type PostLoad } from '@/components/board/usePost';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { editEntryPath, fromName, fromRoot, goToStart, type EntryFrom } from '@/components/entry/entryFrom';
import { useEntry } from '@/components/entry/useEntry';
import { StatusMessage } from '@/components/today/StatusMessage';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import { ApiError } from '@/lib/api/client';
import { archiveEntry, deleteEntry, entryErrorMessage, unarchiveEntry, updateEntry, type Entry } from '@/lib/api/entries';
import { REACTIONS } from '@/lib/entry/reactions';
import { formatStars, starsSpoken } from '@/lib/entry/stars';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { dateLineLong } from '@/lib/wheel/format';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

const COUNTER_INSET = 12;
const COUNTER_PADDING = 6;
const REACTION_SIZE = 24;
const RULE_HEIGHT = 20;

type EntryPhoto = Entry['photos'][number];

/** One line of capitals as the Label role draws it (letter spacing 1.5, `muted`). */
const capsStyle = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  letterSpacing: 1.5,
  textTransform: 'uppercase',
  color: color.muted,
} as const;

/** The photo that could not load: a `secondary` panel of the same size, the glyph and a line. */
function PhotoFallback() {
  return (
    <View
      style={{
        flex: 1,
        backgroundColor: color.secondary,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Ionicons name="image-outline" size={32} color={color.muted} />
      <View style={{ marginTop: space.sm }}>
        <AppText role="label" tone="muted">
          Couldn't load this photo.
        </AppText>
      </View>
    </View>
  );
}

/**
 * The photos: square (or `aspect` wide to 1 high), the full width of the screen, no margins and no
 * radius, one page per photo with the system's own paging. With more than one a plain counter sits
 * at the bottom right and the gallery is one adjustable element for VoiceOver. Nothing fades.
 * Exported for the post page of someone else's post (Phase 5), which uses the same gallery.
 */
export function Gallery({
  photos,
  title,
  aspect = 1,
}: {
  photos: EntryPhoto[];
  title: string;
  reduceMotion?: boolean;
  aspect?: number;
}) {
  const { width } = useWindowDimensions();
  const height = Math.round(width / aspect);
  const scrollRef = useRef<ScrollView>(null);
  const [page, setPage] = useState(0);
  const [failed, setFailed] = useState<Record<string, boolean>>({});
  const count = photos.length;
  const current = Math.min(page, Math.max(count - 1, 0));
  const label = count > 1 ? `Photo ${current + 1} of ${count} for ${title}` : `Photo for ${title}`;

  function goTo(next: number) {
    const clamped = Math.min(Math.max(next, 0), count - 1);
    scrollRef.current?.scrollTo({ x: clamped * width, animated: false });
    setPage(clamped);
  }

  return (
    <View
      accessible
      accessibilityRole="adjustable"
      accessibilityLabel={label}
      accessibilityActions={count > 1 ? [{ name: 'increment' }, { name: 'decrement' }] : undefined}
      onAccessibilityAction={(event) => {
        if (event.nativeEvent.actionName === 'increment') goTo(current + 1);
        else if (event.nativeEvent.actionName === 'decrement') goTo(current - 1);
      }}
      style={{ width, height, backgroundColor: color.secondary }}
    >
      <ScrollView
        ref={scrollRef}
        horizontal
        pagingEnabled
        showsHorizontalScrollIndicator={false}
        scrollEnabled={count > 1}
        onMomentumScrollEnd={(event) => {
          setPage(Math.round(event.nativeEvent.contentOffset.x / width));
        }}
      >
        {photos.map((photo, index) => {
          const image = failed[photo.id] ? (
            <PhotoFallback />
          ) : (
            <Image
              source={{ uri: photo.url }}
              style={{ width: '100%', height: '100%' }}
              contentFit="cover"
              onError={() => setFailed((now) => ({ ...now, [photo.id]: true }))}
            />
          );
          return (
            <View key={photo.id} style={{ width, height, backgroundColor: color.secondary }}>
              {image}
            </View>
          );
        })}
      </ScrollView>
      {count > 1 ? (
        <View
          pointerEvents="none"
          style={{
            position: 'absolute',
            right: COUNTER_INSET,
            bottom: COUNTER_INSET,
            padding: COUNTER_PADDING,
            backgroundColor: color.dominant,
          }}
        >
          <AppText role="label">{`${current + 1} / ${count}`}</AppText>
        </View>
      ) : null}
    </View>
  );
}

/** One detail of the page. It appears at once: the developer does not want fades on the entry page. */
function Rise({ children }: { index: number; reduceMotion: boolean; children: ReactNode }) {
  return <View>{children}</View>;
}

/** The text and rating below the photo (UI spec 9, steps 3 to 7). Also used by the Profile posts list. */
export function Details({ entry, reduceMotion }: { entry: Entry; reduceMotion: boolean }) {
  const reaction = entry.reaction === null ? null : (REACTIONS.find((item) => item.name === entry.reaction) ?? null);
  const category = entry.category.name;
  const date = dateLineLong(entry.local_date);
  const spokenRating = reaction
    ? `${starsSpoken(entry.stars)}. Reaction: ${reaction.label}.`
    : `${starsSpoken(entry.stars)}.`;
  const comment = entry.comment !== null && entry.comment.trim() !== '' ? entry.comment : null;
  let index = 0;

  return (
    <View style={{ paddingHorizontal: space.lg, paddingTop: space.lg, paddingBottom: space.xl }}>
      <Rise index={index++} reduceMotion={reduceMotion}>
        <View accessible accessibilityLabel={`${category}. ${date}`} style={{ flexDirection: 'row' }}>
          <Text numberOfLines={1} style={{ ...capsStyle, flexShrink: 1 }}>
            {category}
          </Text>
          <Text style={capsStyle}>{` · ${date}`}</Text>
        </View>
      </Rise>

      {entry.late ? (
        <View style={{ marginTop: space.xs }}>
          <Rise index={index++} reduceMotion={reduceMotion}>
            <AppText role="label" tone="muted">
              Posted late.
            </AppText>
          </Rise>
        </View>
      ) : null}

      <View style={{ marginTop: space.sm }}>
        <Rise index={index++} reduceMotion={reduceMotion}>
          <AppText role="title" numberOfLines={3}>
            {entry.title}
          </AppText>
        </Rise>
      </View>

      <View style={{ marginTop: space.md }}>
        <Rise index={index++} reduceMotion={reduceMotion}>
          <View accessible accessibilityLabel={spokenRating} style={{ flexDirection: 'row', alignItems: 'center' }}>
            <StarDisplay value={entry.stars} size={20} decorative />
            <View style={{ marginLeft: space.sm }}>
              <AppText role="body">{formatStars(entry.stars)}</AppText>
            </View>
            {reaction ? (
              <>
                <View
                  style={{
                    marginHorizontal: space.md,
                    width: 1,
                    height: RULE_HEIGHT,
                    backgroundColor: color.divider,
                  }}
                />
                <Text
                  allowFontScaling={false}
                  style={{ fontSize: REACTION_SIZE, lineHeight: type.heading.lineHeight }}
                >
                  {reaction.char}
                </Text>
              </>
            ) : null}
          </View>
        </Rise>
      </View>

      {comment !== null ? (
        <View style={{ marginTop: space.md }}>
          <Rise index={index++} reduceMotion={reduceMotion}>
            <AppText role="note">{comment}</AppText>
          </Rise>
        </View>
      ) : null}
    </View>
  );
}

/**
 * What others did with your post (Phase 5, 05-FOLLOW-OVERRIDES section 5): under the details, who
 * reacted (a name and the character, or 'No reactions yet.') and the comments, oldest first, each
 * with 'Delete' (the owner may delete any), its replies under it, and 'Reply' while commenting is
 * on (the bar at the bottom then sends a reply). Read from GET /posts/{entry_id}; until that answers, or
 * if it cannot, nothing is drawn here and the rest of the page is unchanged.
 */
function OwnPostExtras({ social, myId }: { social: PostLoad; myId: string | null }) {
  const post = social.post;
  if (social.status !== 'ready' || post === null) return null;
  const comments = post.comments ?? [];
  return (
    <View style={{ paddingHorizontal: space.lg, paddingBottom: space.xl }}>
      <ReceivedReactions received={post.received ?? []} />
      {comments.length > 0 ? (
        <View style={{ marginTop: space.lg }}>
          <CommentsRule count={post.card.comment_count ?? comments.length} />
        </View>
      ) : null}
      <CommentList
        comments={comments}
        myId={myId}
        onDelete={social.remove}
        onReply={post.can_comment ? social.startReply : undefined}
      />
    </View>
  );
}

/**
 * The entry page (03-UI-SPEC section 9, the developer's picture 06). A component with a thin route
 * (`app/(app)/today/entry.tsx`), mounted again by the Diary and Profile stacks with `from` =
 * or 'profile' (Phase 4). `from` names the Back row and the gone-state button and keeps
 * Edit inside the same stack.
 *
 * Top to bottom: the Back row (the place the person came from), the photos edge to edge, the
 * category and long date, the title, the rating and reaction, the comment, and a pinned
 * three-dots menu in the Back row (Edit, Delete; the Tabs layout hides the bar for the
 * route name `entry`). The page reloads when it comes back into focus. Phase 5 adds, under the
 * details, the reactions it received and its comments, and the comment bar pinned at the bottom
 * ('Commenting is off.' in its place when commenting is switched off), all from GET /posts/{id};
 * the Board stack mounts it too (`from` = 'board').
 *
 * Delete asks with the phone's own alert (Cancel, and a red Delete). The delete is POST
 * /entries/{id}/delete; the answer's Today goes into the store; 'not found' counts as done.
 */
export function EntryView({ id, from, backLabel }: { id: string; from: EntryFrom; backLabel?: string }) {
  const router = useRouter();
  const reduceMotion = useReduceMotion();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const load = useEntry(id);
  const social = usePost(id);

  const [deleting, setDeleting] = useState(false);
  const [deleteMessage, setDeleteMessage] = useState<string | null>(null);
  const deletingRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const backName = fromName(from);
  /** The Back row may name the page it returns to (Notifications); the gone state keeps the stack's name. */
  const backRowName = backLabel ?? backName;

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace(fromRoot(from));
  }

  async function runDelete() {
    if (userId === null || deletingRef.current) return;
    deletingRef.current = true;
    setDeleting(true);
    setDeleteMessage(null);
    const mark = performance.now();
    try {
      const answer = await deleteEntry(id);
      applyToday(userId, answer.today, mark);
      void refetchToday(userId, 'manual');
      goBack();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'not_found') {
        // Already gone (another phone): the same end. Today asks again so it shows the truth.
        void refetchToday(userId, 'manual');
        goBack();
        return;
      }
      deletingRef.current = false;
      if (mountedRef.current) {
        setDeleting(false);
        setDeleteMessage(entryErrorMessage(err).text);
      }
    }
  }

  function askDelete() {
    if (deletingRef.current) return;
    Alert.alert(
      'Delete this entry?',
      "It will be gone for good. The day still counts as posted, so you can't post again for this day.",
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Delete', onPress: () => void runDelete() },
      ],
      { cancelable: true }
    );
  }

  const [menuOpen, setMenuOpen] = useState(false);

  /** Run after the menu has closed, so an alert or a page never opens under its dim cover. */
  function fromMenu(action: () => void) {
    return () => {
      setMenuOpen(false);
      setTimeout(action, 0);
    };
  }

  /** Archive or put back, then leave: the day stays posted either way. */
  async function runArchive(archive: boolean) {
    if (userId === null || deletingRef.current) return;
    deletingRef.current = true;
    setDeleting(true);
    setDeleteMessage(null);
    const mark = performance.now();
    try {
      const answer = archive ? await archiveEntry(id) : await unarchiveEntry(id);
      applyToday(userId, answer.today, mark);
      void refetchToday(userId, 'manual');
      if (archive) goBack();
      else {
        load.reload();
        social.reload();
      }
    } catch (err) {
      if (mountedRef.current) setDeleteMessage(entryErrorMessage(err).text);
    } finally {
      deletingRef.current = false;
      if (mountedRef.current) setDeleting(false);
    }
  }

  /** Switch commenting on or off. It is saved now and shows once comments exist. */
  async function runComments(off: boolean) {
    if (deletingRef.current) return;
    deletingRef.current = true;
    setDeleting(true);
    setDeleteMessage(null);
    try {
      await updateEntry(id, { comments_off: off });
      load.reload();
      social.reload();
    } catch (err) {
      if (mountedRef.current) setDeleteMessage(entryErrorMessage(err).text);
    } finally {
      deletingRef.current = false;
      if (mountedRef.current) setDeleting(false);
    }
  }

  const archived = load.entry?.archived === true;
  const commentsOff = load.entry?.comments_off === true;
  const menuItems: MenuItem[] = [
    { label: 'Edit', onPress: fromMenu(() => router.push({ pathname: editEntryPath(from), params: { id } })) },
    { label: archived ? 'Restore' : 'Archive', onPress: fromMenu(() => void runArchive(!archived)) },
    {
      label: commentsOff ? 'Turn on commenting' : 'Turn off commenting',
      onPress: fromMenu(() => void runComments(!commentsOff)),
    },
    { label: 'Delete', onPress: fromMenu(askDelete) },
  ];

  const entry = load.entry;
  const ready = load.status === 'ready' && entry !== null;

  return (
    <SafeAreaView edges={['top']} style={{ flex: 1, backgroundColor: color.dominant }}>
      {/* The Back row: the chevron and the name of where the person came from. */}
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, justifyContent: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel={`Back to ${backRowName}`}
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="body">{backRowName}</AppText>
          </View>
        </Pressable>
        {ready ? (
          <Pressable
            onPress={() => !deletingRef.current && setMenuOpen(true)}
            disabled={deleting}
            accessibilityRole="button"
            accessibilityLabel="More"
            hitSlop={8}
            style={{
              position: 'absolute',
              right: space.lg,
              width: minTouchTarget,
              height: minTouchTarget,
              alignItems: 'flex-end',
              justifyContent: 'center',
            }}
          >
            <Ionicons name="ellipsis-horizontal" size={24} color={deleting ? color.muted : color.ink} />
          </Pressable>
        ) : null}
      </View>

      {ready ? (
        <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
          <ScrollView style={{ flex: 1 }} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>
            <Gallery photos={entry.photos} title={entry.title} reduceMotion={reduceMotion} />
            <Details entry={entry} reduceMotion={reduceMotion} />
            <OwnPostExtras social={social} myId={userId} />
          </ScrollView>

          {deleteMessage !== null ? (
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
                {deleteMessage}
              </Text>
            </View>
          ) : null}
          {social.status === 'ready' && social.post !== null ? (
            social.post.can_comment ? (
              <CommentBar onSend={social.comment} replyTo={social.replyTo} onCancelReply={social.cancelReply} />
            ) : (
              <CommentsOffBar />
            )
          ) : null}
        </KeyboardAvoidingView>
      ) : (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          {load.status === 'loading' ? (
            <AppText role="label" tone="muted">
              Loading your entry.
            </AppText>
          ) : load.status === 'offline' ? (
            <StatusMessage kind={load.failure} onRetry={load.reload} />
          ) : (
            <View style={{ alignItems: 'flex-start' }}>
              <AppText role="body">This entry isn't here any more.</AppText>
              <View style={{ marginTop: space.lg }}>
                <TextButton label={`Back to ${backName}`} onPress={() => goToStart(router, from)} />
              </View>
            </View>
          )}
        </View>
      )}
      <MenuSheet visible={menuOpen} items={menuItems} onClose={() => setMenuOpen(false)} />
    </SafeAreaView>
  );
}
