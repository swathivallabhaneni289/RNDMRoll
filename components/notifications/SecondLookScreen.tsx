import { useEffect, useRef, useState } from 'react';
import { Alert, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { useNavigation, useRouter } from 'expo-router';
import { Image } from 'expo-image';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { BackRow } from '@/components/settings/BackRow';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { FieldError, TextField } from '@/components/ui/TextField';
import { ApiError } from '@/lib/api/client';
import { uploadEntryPhoto } from '@/lib/api/entries';
import { getNotifications } from '@/lib/api/notifications';
import { photoSource } from '@/lib/api/photoSource';
import { askSecondLook } from '@/lib/api/secondLooks';
import { createUploadMemory, photoMessage, photoProblem } from '@/lib/entry/photo';
import { announceProblem } from '@/lib/forms/announce';
import { bringIntoView } from '@/lib/forms/bringIntoView';
import { showAccessOff } from '@/lib/media/showAccessOff';
import {
  SECOND_LOOK_LABEL,
  SECOND_LOOK_MAX_PHOTOS,
  SECOND_LOOK_PHOTOS,
  SECOND_LOOK_SENT,
  SECOND_LOOK_TITLE,
  isRemovalKind,
  secondLookFailure,
  secondLookItemLine,
  secondLookTextProblem,
  type RemovalKind,
} from '@/lib/notifications/moderation';
import { chosenSecondLook, type SecondLookItem } from '@/lib/notifications/secondLookItem';
import { setReportNotice } from '@/lib/report/notice';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

/** A chosen photo, small and rounded, as the comment box shows its one. */
const PREVIEW = 64;
/** The round close mark on a chosen photo's corner. */
const CLOSE = 24;
/** What a screen reader says before a line under the photos. */
const PHOTOS_LABEL = 'Photos';
/** A page opened without the bell's item, for a removal the bell's list no longer has. */
const GONE_LINE = "This isn't here any more.";

type Photo = { uri: string; mimeType: string };

/**
 * The page to ask for a second look at a post or comment an admin removed (developer, 2026-11-19:
 * "maybe give a little option there, not like something really big"), pushed from its line in
 * Notifications. It names the item ("Your post: Dune"), then a box for why it should come back
 * (required, up to 500 characters) and Add photos: up to 3 from the camera or the library,
 * screenshots too, each with a close mark. Send is grey only while it sends. A tap with nothing
 * written, or too much, says so under the box; a refusal says what is wrong in a few words, under
 * the box, under the photos or over Send.
 *
 * The photos upload at Send, one after another, each once (the comment box's rule), so a send tried
 * again after a lost answer, with the same words, names the same files and the server answers it as
 * the same request. A refusal of the photos forgets them all, so the next Send uploads them again.
 * Once it is sent, "Sent. We'll take another look." shows over Notifications, which reads its list
 * again and says "Second look asked." under the line. What was sent is not shown again. No bottom
 * bar, so the page keeps all four safe-area edges.
 */
export function SecondLookScreen({ removal }: { removal: string }) {
  const router = useRouter();
  const navigation = useNavigation();
  // The item the bell kept for this removal. A page opened without it (a link) reads the bell's list.
  const [item, setItem] = useState<SecondLookItem | null>(() => chosenSecondLook(removal));
  const [status, setStatus] = useState<'ready' | 'loading' | 'offline' | 'gone'>(() =>
    item === null ? 'loading' : 'ready'
  );
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [text, setText] = useState('');
  const [textError, setTextError] = useState('');
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [photosError, setPhotosError] = useState('');
  const [formError, setFormError] = useState('');
  const [sending, setSending] = useState(false);
  // From the tap until the answer: a second tap sends nothing.
  const sendingRef = useRef(false);
  const mountedRef = useRef(true);
  // Each photo is uploaded once, at Send, and its key kept until it is removed.
  const uploads = useRef(createUploadMemory()).current;
  // A line under the box or the photos scrolls them into view; the box's also puts the cursor in it.
  const scrollRef = useRef<ScrollView>(null);
  const textFieldRef = useRef<View>(null);
  const textBoxRef = useRef<TextInput>(null);
  const photosRef = useRef<View>(null);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  /** The bell's list, read for this removal when the bell kept no item for it. */
  async function readItem() {
    setStatus('loading');
    try {
      const { items } = await getNotifications();
      if (!mountedRef.current) return;
      const found = items.find((one) => one.removal !== '' && one.removal === removal);
      if (found === undefined || !isRemovalKind(found.kind)) {
        setStatus('gone');
        return;
      }
      setItem({ removal, kind: found.kind, text: found.text ?? '' });
      setStatus('ready');
    } catch (err) {
      if (!mountedRef.current) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }

  useEffect(() => {
    if (item === null) void readItem();
    // Once, as the page opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today/notifications');
  }

  function handleTextChange(value: string) {
    setText(value);
    setTextError('');
    setFormError('');
  }

  /** A picked file is checked (type and size, as for a comment's photo) before it is kept, up to 3 in all. */
  function take(result: ImagePicker.ImagePickerResult) {
    if (result.canceled || result.assets.length === 0) return;
    const next = [...photos];
    let problem = '';
    for (const asset of result.assets) {
      const refused = photoProblem(asset.mimeType, asset.fileSize ?? 0);
      if (refused !== null) {
        if (problem === '') problem = photoMessage(refused);
        continue;
      }
      if (next.length < SECOND_LOOK_MAX_PHOTOS && !next.some((photo) => photo.uri === asset.uri)) {
        next.push({ uri: asset.uri, mimeType: asset.mimeType ?? 'image/jpeg' });
      }
    }
    setPhotos(next);
    setPhotosError(problem);
    setFormError('');
  }

  async function pick(source: 'camera' | 'library') {
    let result: ImagePicker.ImagePickerResult;
    try {
      const permission =
        source === 'camera'
          ? await ImagePicker.requestCameraPermissionsAsync()
          : await ImagePicker.requestMediaLibraryPermissionsAsync();
      if (!permission.granted) {
        // Access turned off (iOS asks only once): the phone's alert, with a way to Settings.
        showAccessOff(source);
        return;
      }
      const options: ImagePicker.ImagePickerOptions = {
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        quality: 0.8,
        // A standard JPEG even for a HEIC photo, so any picture works everywhere.
        preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
      };
      // The camera takes one photo; the library takes as many at once as there is room for.
      result =
        source === 'camera'
          ? await ImagePicker.launchCameraAsync(options)
          : await ImagePicker.launchImageLibraryAsync({
              ...options,
              allowsMultipleSelection: true,
              selectionLimit: SECOND_LOOK_MAX_PHOTOS - photos.length,
            });
    } catch {
      setPhotosError(source === 'camera' ? "Camera isn't available on this device." : "Photos aren't available right now.");
      return;
    }
    take(result);
  }

  function choosePhotos() {
    Alert.alert(SECOND_LOOK_PHOTOS, undefined, [
      { text: 'Camera', onPress: () => void pick('camera') },
      { text: 'Library', onPress: () => void pick('library') },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  function dropPhoto(uri: string) {
    uploads.forget(uri);
    setPhotos((now) => now.filter((photo) => photo.uri !== uri));
    setPhotosError('');
  }

  /** The box's line: VoiceOver says it, the box scrolls into view and the cursor goes into it. */
  function showTextProblem(line: string) {
    setTextError(line);
    announceProblem(line, SECOND_LOOK_LABEL);
    bringIntoView(scrollRef.current, textFieldRef.current);
    textBoxRef.current?.focus();
  }

  /** A failed send's line, where secondLookFailure puts it. */
  function showFailure(err: unknown, kind: RemovalKind) {
    const answer = err instanceof ApiError ? err : null;
    const failed = secondLookFailure(kind, answer?.status ?? 0, answer?.code ?? '', answer?.field, answer?.reason);
    const line = failed.line ?? answer?.userMessage ?? '';
    if (failed.at === 'text') {
      showTextProblem(line);
      return;
    }
    if (failed.at === 'photos') {
      // The server would not take a photo: all are uploaded again at the next Send.
      if (answer?.code === 'validation_failed') uploads.forget();
      setPhotosError(line);
      announceProblem(line, PHOTOS_LABEL);
      bringIntoView(scrollRef.current, photosRef.current);
      return;
    }
    setFormError(line);
    announceProblem(line);
  }

  async function send() {
    if (item === null || sendingRef.current) return;
    const problem = secondLookTextProblem(text);
    if (problem !== '') {
      showTextProblem(problem);
      return;
    }
    sendingRef.current = true;
    setSending(true);
    setTextError('');
    setPhotosError('');
    setFormError('');
    let sent = false;
    try {
      // One photo after another, as the entry form uploads its own.
      const keys: string[] = [];
      for (const photo of photos) {
        keys.push(await uploads.ensure(photo.uri, () => uploadEntryPhoto(photo.uri, photo.mimeType)));
      }
      await askSecondLook(item.removal, text, keys);
      sent = true;
      setReportNotice(SECOND_LOOK_SENT);
      // Not when the person already left while this ran: going back again would close Notifications.
      if (mountedRef.current && navigation.isFocused()) goBack();
    } catch (err) {
      if (mountedRef.current) showFailure(err, item.kind);
    } finally {
      sendingRef.current = false;
      // After a success the button stays busy until the page has gone.
      if (!sent && mountedRef.current) setSending(false);
    }
  }

  const line = item === null ? null : secondLookItemLine(item.kind, item.text);

  return (
    <Screen scrollRef={scrollRef}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow fallback="/(app)/today/notifications" />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            {SECOND_LOOK_TITLE}
          </AppText>
        </View>

        {status === 'loading' ? (
          <View style={{ marginTop: space.lg }}>
            <AppText role="label" tone="muted">
              Loading.
            </AppText>
          </View>
        ) : status === 'offline' ? (
          <View style={{ marginTop: space.lg }}>
            <StatusMessage kind={failure} onRetry={() => void readItem()} />
          </View>
        ) : status === 'gone' || line === null ? (
          <View style={{ marginTop: space.lg }}>
            <AppText role="body">{GONE_LINE}</AppText>
          </View>
        ) : (
          <>
            <View style={{ marginTop: space.sm }}>
              <AppText role="body">
                <Text style={{ fontFamily: type.button.fontFamily }}>{line.label}</Text>
                {` ${line.words}`}
              </AppText>
            </View>

            <View ref={textFieldRef} style={{ marginTop: space.lg }}>
              <TextField
                ref={textBoxRef}
                label={SECOND_LOOK_LABEL}
                variant="soft"
                value={text}
                onChangeText={handleTextChange}
                error={textError}
                editable={!sending}
                multiline
                numberOfLines={4}
              />
            </View>

            <View ref={photosRef} style={{ marginTop: space.lg }}>
              {photos.length > 0 ? (
                // The chosen photos, small and rounded, each with a close mark, until they are sent.
                <View style={{ flexDirection: 'row', gap: space.md, paddingTop: space.sm, marginBottom: space.sm }}>
                  {photos.map((photo, index) => (
                    <View key={photo.uri} style={{ width: PREVIEW, height: PREVIEW }}>
                      <Image
                        source={photoSource(photo.uri)}
                        style={{ width: PREVIEW, height: PREVIEW, borderRadius: radius.md }}
                        contentFit="cover"
                        transition={0}
                        accessibilityLabel={`Photo ${index + 1} of ${photos.length}`}
                      />
                      <Pressable
                        onPress={() => dropPhoto(photo.uri)}
                        disabled={sending}
                        accessibilityRole="button"
                        accessibilityLabel={`Remove photo ${index + 1}`}
                        hitSlop={12}
                        style={{
                          position: 'absolute',
                          top: -8,
                          right: -8,
                          width: CLOSE,
                          height: CLOSE,
                          borderRadius: CLOSE / 2,
                          backgroundColor: color.ink,
                          alignItems: 'center',
                          justifyContent: 'center',
                        }}
                      >
                        <Ionicons name="close" size={16} color={color.dominant} />
                      </Pressable>
                    </View>
                  ))}
                </View>
              ) : null}
              {photos.length < SECOND_LOOK_MAX_PHOTOS ? (
                // The photo mark and the words in ink: never a pill, never underlined.
                <Pressable
                  onPress={choosePhotos}
                  disabled={sending}
                  accessibilityRole="button"
                  accessibilityLabel={SECOND_LOOK_PHOTOS}
                  style={{ minHeight: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
                >
                  <Ionicons name="image-outline" size={24} color={color.ink} />
                  <View style={{ marginLeft: space.sm }}>
                    <AppText role="button">{SECOND_LOOK_PHOTOS}</AppText>
                  </View>
                </Pressable>
              ) : null}
              {photosError !== '' ? <FieldError text={photosError} label={PHOTOS_LABEL} /> : null}
            </View>

            {formError !== '' ? (
              <View style={{ marginTop: space.md }}>
                <FieldError text={formError} />
              </View>
            ) : null}

            <View style={{ marginTop: space.lg }}>
              <PrimaryButton label="Send" loading={sending} onPress={() => void send()} />
            </View>
          </>
        )}
      </View>
    </Screen>
  );
}
