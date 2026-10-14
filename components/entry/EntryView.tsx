import { ReactNode, useEffect, useRef, useState } from 'react';
import { Alert, Pressable, ScrollView, Text, useWindowDimensions, View } from 'react-native';
import { Image } from 'expo-image';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { goToToday } from '@/components/entry/goToToday';
import { useEntry } from '@/components/entry/useEntry';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import { ApiError } from '@/lib/api/client';
import { deleteEntry, entryErrorMessage, type Entry } from '@/lib/api/entries';
import { REACTIONS } from '@/lib/entry/reactions';
import { formatStars, starsSpoken } from '@/lib/entry/stars';
import { FadeInUp, ImageReveal, staggerDelay } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { dateLineLong } from '@/lib/wheel/format';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/** The footer's two halves are at least this tall (UI spec 9), plus the bottom safe area. */
const FOOTER_MIN_HEIGHT = 56;
const COUNTER_INSET = 12;
const COUNTER_PADDING = 6;
const REACTION_SIZE = 24;
const RULE_HEIGHT = 20;
/** The details rise one after another, starting a little after the photo (UI spec 12). */
const RISE_BASE_MS = 200;

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
 * The photos: square, the full width of the screen, no margins and no radius, one page per photo
 * with the system's own paging. With more than one a plain counter sits at the bottom right and
 * the gallery is one adjustable element for VoiceOver. The first photo is revealed (450ms) unless
 * Reduce Motion is on.
 */
function Gallery({ photos, title, reduceMotion }: { photos: EntryPhoto[]; title: string; reduceMotion: boolean }) {
  const { width } = useWindowDimensions();
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
      style={{ width, height: width, backgroundColor: color.secondary }}
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
            <View key={photo.id} style={{ width, height: width, backgroundColor: color.secondary }}>
              {index === 0 && !reduceMotion ? <ImageReveal style={{ flex: 1 }}>{image}</ImageReveal> : image}
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

/** One detail of the page: it rises after the photo, in order, and just appears under Reduce Motion. */
function Rise({ index, reduceMotion, children }: { index: number; reduceMotion: boolean; children: ReactNode }) {
  if (reduceMotion) return <View>{children}</View>;
  return <FadeInUp delay={staggerDelay(index, RISE_BASE_MS)}>{children}</FadeInUp>;
}

/** The text and rating below the photo (UI spec 9, steps 3 to 7). */
function Details({ entry, reduceMotion }: { entry: Entry; reduceMotion: boolean }) {
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
 * The entry page (03-UI-SPEC section 9, the developer's picture 06). A component with a thin route
 * (`app/(app)/today/entry.tsx`), so the Phase 4 diary mounts the same page with `from` = 'diary'.
 *
 * Top to bottom: the Back row (the place the person came from), the photos edge to edge, the
 * category and long date, the title, the rating and reaction, the comment, and a pinned
 * 'Edit | Delete' footer where the bottom bar would be (the Tabs layout hides the bar for the
 * route name `entry`). The page reloads when it comes back into focus.
 *
 * Delete asks with the phone's own alert (Cancel, and a red Delete). The delete is POST
 * /entries/{id}/delete; the answer's Today goes into the store; 'not found' counts as done.
 */
export function EntryView({ id, from }: { id: string; from: 'today' | 'diary' }) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const reduceMotion = useReduceMotion();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const load = useEntry(id);

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

  const backName = from === 'diary' ? 'Diary' : 'Today';

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
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
        { text: 'Delete', style: 'destructive', onPress: () => void runDelete() },
      ],
      { cancelable: true }
    );
  }

  const entry = load.entry;
  const ready = load.status === 'ready' && entry !== null;

  return (
    <SafeAreaView edges={['top']} style={{ flex: 1, backgroundColor: color.dominant }}>
      {/* The Back row: the chevron and the name of where the person came from. */}
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
        <>
          <ScrollView style={{ flex: 1 }} showsVerticalScrollIndicator={false}>
            <Gallery photos={entry.photos} title={entry.title} reduceMotion={reduceMotion} />
            <Details entry={entry} reduceMotion={reduceMotion} />
          </ScrollView>

          {deleteMessage !== null ? (
            <View style={{ paddingHorizontal: space.lg, paddingVertical: space.sm }} accessibilityLiveRegion="polite">
              <Text
                style={{
                  fontFamily: type.label.fontFamily,
                  fontSize: type.label.fontSize,
                  lineHeight: type.label.lineHeight,
                  color: color.destructive,
                  textAlign: 'center',
                }}
              >
                {deleteMessage}
              </Text>
            </View>
          ) : null}

          <View style={{ borderTopWidth: 1, borderTopColor: color.divider, backgroundColor: color.dominant }}>
            <View style={{ flexDirection: 'row', minHeight: FOOTER_MIN_HEIGHT }}>
              <Pressable
                onPress={() => router.push({ pathname: '/(app)/today/edit-entry', params: { id } })}
                disabled={deleting}
                accessibilityRole="button"
                accessibilityLabel="Edit"
                accessibilityState={{ disabled: deleting }}
                style={{ flex: 1, minHeight: FOOTER_MIN_HEIGHT, alignItems: 'center', justifyContent: 'center' }}
              >
                <AppText role="button" tone={deleting ? 'muted' : 'default'}>
                  Edit
                </AppText>
              </Pressable>
              <View style={{ width: 1, marginVertical: space.sm, backgroundColor: color.divider }} />
              <Pressable
                onPress={askDelete}
                disabled={deleting}
                accessibilityRole="button"
                accessibilityLabel="Delete"
                accessibilityHint="Asks first."
                accessibilityState={{ disabled: deleting }}
                style={{ flex: 1, minHeight: FOOTER_MIN_HEIGHT, alignItems: 'center', justifyContent: 'center' }}
              >
                <AppText role="button" tone={deleting ? 'muted' : 'destructive'}>
                  Delete
                </AppText>
              </Pressable>
            </View>
            <View style={{ height: insets.bottom }} />
          </View>
        </>
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
                <TextButton label="Back to Today" onPress={() => goToToday(router)} />
              </View>
            </View>
          )}
        </View>
      )}
    </SafeAreaView>
  );
}
