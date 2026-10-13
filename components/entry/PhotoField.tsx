import { useEffect, useState } from 'react';
import { Alert, Pressable, ScrollView, View } from 'react-native';
import { Image } from 'expo-image';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import type { PickedPhoto } from '@/components/entry/useEntryDraft';
import { AppText } from '@/components/ui/AppText';
import { PhotoPlaceholder } from '@/components/ui/PhotoPlaceholder';
import type { PhotoSource } from '@/lib/api/entries';
import { fileSize } from '@/lib/api/profile';
import { canAddPhoto, photoMessage, photoProblem } from '@/lib/entry/photo';
import { ImageReveal, PressScale } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, minTouchTarget, radius, space } from '@/lib/theme/tokens';

const PANEL_RATIO = 1.7;
const THUMB = 56;
const CHIP = 20;
const PILL_HEIGHT = 40;
const PILL_GAP = 10;

const CAMERA_DENIED = 'Camera access is needed to take a photo.';
const LIBRARY_DENIED = 'Photo library access is needed to choose a photo.';
const CAMERA_UNAVAILABLE = "Camera isn't available on this device.";
const LIBRARY_UNAVAILABLE = "Couldn't open your photo library.";
const FILE_REFUSED = "That file can't be used as a photo. Try another one.";

type PhotoFieldProps = {
  /** The ordered photos of the draft; the first is the cover. */
  photos: readonly PickedPhoto[];
  /** In `edit` the last remaining photo has no remove mark: one must stay. */
  mode: 'new' | 'edit';
  onAdd: (photo: PickedPhoto) => void;
  onRemove: (uri: string) => void;
  /** A message from the screen (a photo upload that failed), shown under the strip. */
  message?: string | null;
};

/** 'Camera' and 'Library': pills, the named exception to the no-pill rule (UI spec 2 and 7). */
function Pill({ label, hint, onPress, reduce }: { label: string; hint: string; onPress: () => void; reduce: boolean }) {
  const face = (
    <View
      style={{
        height: PILL_HEIGHT,
        paddingHorizontal: space.lg,
        borderRadius: radius.full,
        borderWidth: 1,
        borderColor: color.ink,
        backgroundColor: color.dominant,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <AppText role="body">{label}</AppText>
    </View>
  );
  const shared = {
    onPress,
    hitSlop: { top: 2, bottom: 2 },
    accessibilityRole: 'button' as const,
    accessibilityLabel: label,
    accessibilityHint: hint,
  };
  return reduce ? <Pressable {...shared}>{face}</Pressable> : <PressScale {...shared}>{face}</PressScale>;
}

/**
 * The photo field of the entry screen (UI spec 7). The wide panel shows the cover; while there is
 * no photo it holds the stand-in glyph, 'Add a photo' and the Camera and Library pills; with photos
 * a strip of 56dp thumbnails sits under it (the cover framed, a remove mark on each) and, below ten
 * photos, a plus tile that opens the Add a photo sheet. It keeps no upload state: it checks a pick
 * (type, then size) and hands the accepted photo up.
 */
export function PhotoField({ photos, mode, onAdd, onRemove, message = null }: PhotoFieldProps) {
  const reduce = useReduceMotion();
  const [problem, setProblem] = useState<string | null>(null);
  const count = photos.length;
  const cover = photos[0];
  const removable = mode === 'new' || count > 1;

  // The screen's message goes away as soon as the photo list changes after it arrived (UI spec 7).
  const signature = photos.map((photo) => photo.uri).join('\n');
  const [arrivedWith, setArrivedWith] = useState<string | null>(null);
  // Declared first, so a new message that arrives in the same render still wins below. Once the
  // list has changed the message stays gone, even if the list later returns to what it was.
  useEffect(() => {
    setArrivedWith((current) => (current === signature ? current : null));
  }, [signature]);
  useEffect(() => {
    setArrivedWith(message ? signature : null);
    // Only a new message restarts this; the list changing afterwards is what hides it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [message]);
  const shown = problem ?? (message && arrivedWith === signature ? message : null);

  async function accept(result: ImagePicker.ImagePickerResult, source: PhotoSource) {
    if (result.canceled || !result.assets || result.assets.length === 0) return;
    const asset = result.assets[0];
    let size: number;
    try {
      size = await fileSize(asset.uri);
    } catch {
      setProblem(FILE_REFUSED);
      return;
    }
    const refused = photoProblem(asset.mimeType ?? 'image/jpeg', size);
    if (refused) {
      setProblem(photoMessage(refused));
      return;
    }
    setProblem(null);
    onAdd({ uri: asset.uri, mimeType: asset.mimeType ?? null, source });
  }

  // Both launchers reject on a device without a camera (simulators) or when the picker fails to
  // open; the launch is wrapped on its own so a later problem is never reported as that.
  async function pickFromCamera() {
    let result: ImagePicker.ImagePickerResult;
    try {
      const permission = await ImagePicker.requestCameraPermissionsAsync();
      if (!permission.granted) {
        setProblem(CAMERA_DENIED);
        return;
      }
      result = await ImagePicker.launchCameraAsync({
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        quality: 0.8,
        // A standard JPEG even for a HEIC photo, so any picture works everywhere.
        preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
      });
    } catch {
      setProblem(CAMERA_UNAVAILABLE);
      return;
    }
    await accept(result, 'camera');
  }

  async function pickFromLibrary() {
    let result: ImagePicker.ImagePickerResult;
    try {
      const permission = await ImagePicker.requestMediaLibraryPermissionsAsync();
      if (!permission.granted) {
        setProblem(LIBRARY_DENIED);
        return;
      }
      result = await ImagePicker.launchImageLibraryAsync({
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        quality: 0.8,
        // A standard JPEG even for a HEIC photo, so any picture works everywhere.
        preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
      });
    } catch {
      setProblem(LIBRARY_UNAVAILABLE);
      return;
    }
    await accept(result, 'library');
  }

  function openSheet() {
    Alert.alert('Add a photo', undefined, [
      { text: 'Camera', onPress: () => void pickFromCamera() },
      { text: 'Library', onPress: () => void pickFromLibrary() },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  const coverImage = cover ? (
    <Image
      source={{ uri: cover.uri }}
      style={{ width: '100%', height: '100%' }}
      contentFit="cover"
      accessibilityElementsHidden
    />
  ) : null;

  return (
    <View>
      <View style={{ width: '100%', aspectRatio: PANEL_RATIO, backgroundColor: color.secondary, overflow: 'hidden' }}>
        {cover ? (
          reduce ? (
            coverImage
          ) : (
            <ImageReveal key={cover.uri} style={{ flex: 1 }}>
              {coverImage}
            </ImageReveal>
          )
        ) : (
          <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
            <View style={{ width: '100%', height: 40 }}>
              <PhotoPlaceholder variant="panel" />
            </View>
            <AppText role="label" tone="muted">
              Add a photo
            </AppText>
            <View style={{ flexDirection: 'row', gap: PILL_GAP, marginTop: space.md }}>
              <Pill label="Camera" hint="Opens the camera." onPress={() => void pickFromCamera()} reduce={reduce} />
              <Pill
                label="Library"
                hint="Opens your photo library."
                onPress={() => void pickFromLibrary()}
                reduce={reduce}
              />
            </View>
          </View>
        )}
      </View>

      {count > 0 ? (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          keyboardShouldPersistTaps="handled"
          style={{ flexGrow: 0, marginTop: space.sm }}
          contentContainerStyle={{ gap: space.xs }}
        >
          {photos.map((photo, index) => (
            <View key={photo.uri} style={{ width: THUMB, height: THUMB, backgroundColor: color.secondary }}>
              <Image
                source={{ uri: photo.uri }}
                style={{ width: THUMB, height: THUMB }}
                contentFit="cover"
                accessible
                accessibilityLabel={`Photo ${index + 1} of ${count}`}
              />
              {index === 0 ? (
                <View
                  pointerEvents="none"
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    borderWidth: 2,
                    borderColor: color.ink,
                  }}
                />
              ) : null}
              {removable ? (
                <Pressable
                  onPress={() => onRemove(photo.uri)}
                  accessibilityRole="button"
                  accessibilityLabel={`Remove photo ${index + 1}`}
                  style={{
                    position: 'absolute',
                    top: 0,
                    right: 0,
                    width: minTouchTarget,
                    height: minTouchTarget,
                    alignItems: 'flex-end',
                  }}
                >
                  <View
                    style={{
                      width: CHIP,
                      height: CHIP,
                      backgroundColor: color.ink,
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Ionicons name="close" size={14} color={color.dominant} />
                  </View>
                </Pressable>
              ) : null}
            </View>
          ))}
          {canAddPhoto(count) ? (
            <Pressable
              onPress={openSheet}
              accessibilityRole="button"
              accessibilityLabel="Add another photo"
              style={{
                width: THUMB,
                height: THUMB,
                borderRadius: radius.md,
                borderWidth: 1,
                borderColor: color.inkRest,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Ionicons name="add" size={22} color={color.ink} />
            </Pressable>
          ) : null}
        </ScrollView>
      ) : null}

      {shown ? (
        <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
          <AppText role="label" tone="destructive">
            {shown}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}
