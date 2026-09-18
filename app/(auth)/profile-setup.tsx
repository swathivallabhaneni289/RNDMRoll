import { useRef, useState } from 'react';
import { ActivityIndicator, Alert, Platform, Pressable, ScrollView, View } from 'react-native';
import { KeyboardAvoidingView } from 'react-native';
import * as ImagePicker from 'expo-image-picker';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { IconBadge } from '@/components/ui/IconBadge';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { color, radius, space } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import type { AvatarUploadTicket } from '@/lib/api/types';
import { getDraft } from '@/lib/onboarding/draft';

/**
 * D-05 (revised 2026-09-17) step 3: the consolidated "Create your profile"
 * screen, replacing the prior separate name/username/photo steps. This is
 * one route built across two plan tasks: Task 1 (this pass) establishes the
 * screen shell -- hero avatar with immediate upload, name field, bio field,
 * and the pinned CTA -- and positions the username field with its reserved
 * status row. Task 2 wires the username suggestion/debounce/state machine
 * into that field and row, and owns the single authoritative PATCH /me save.
 */

const AVATAR_SIZE = 120;
const AVATAR_GLYPH_SIZE = 56;
const BIO_MAX_LENGTH = 160;
const NAME_MAX_LENGTH = 50;

function validateName(value: string): string | undefined {
  if (value.length === 0) return 'Enter your name.';
  if (value.length > NAME_MAX_LENGTH) return `Name must be ${NAME_MAX_LENGTH} characters or fewer.`;
  return undefined;
}

export default function ProfileSetupScreen() {
  const [name, setName] = useState(() => getDraft().suggestedName ?? '');
  const [nameError, setNameError] = useState<string | undefined>();

  const [username, setUsername] = useState('');

  const [bio, setBio] = useState('');

  const [avatarUri, setAvatarUri] = useState<string | undefined>();
  const [avatarPublicUrl, setAvatarPublicUrl] = useState<string | undefined>();
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [avatarError, setAvatarError] = useState<string | undefined>();

  // Referenced by Task 2's submit handler once it lands; avatarPublicUrl
  // itself is the field this screen's single PATCH /me reads from.
  const avatarPublicUrlRef = useRef(avatarPublicUrl);
  avatarPublicUrlRef.current = avatarPublicUrl;

  function handleNameBlur() {
    setNameError(validateName(name.trim()));
  }

  function handleAvatarPress() {
    Alert.alert('Add profile photo', undefined, [
      { text: 'Take Photo', onPress: () => pickImage('camera') },
      { text: 'Choose from Library', onPress: () => pickImage('library') },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  async function pickImage(source: 'camera' | 'library') {
    setAvatarError(undefined);

    const permission =
      source === 'camera'
        ? await ImagePicker.requestCameraPermissionsAsync()
        : await ImagePicker.requestMediaLibraryPermissionsAsync();

    if (!permission.granted) {
      setAvatarError(
        source === 'camera'
          ? "We need camera access to take a profile photo. You can add one later from your profile."
          : "We need photo library access to add a profile photo. You can add one later from your profile."
      );
      return;
    }

    const pickerOptions: ImagePicker.ImagePickerOptions = {
      mediaTypes: ['images'],
      allowsEditing: true,
      aspect: [1, 1],
      quality: 0.8,
    };

    const result =
      source === 'camera'
        ? await ImagePicker.launchCameraAsync(pickerOptions)
        : await ImagePicker.launchImageLibraryAsync(pickerOptions);

    if (result.canceled || !result.assets?.[0]) return;

    const asset = result.assets[0];
    setAvatarUri(asset.uri);
    await uploadAvatar(asset);
  }

  async function uploadAvatar(asset: ImagePicker.ImagePickerAsset) {
    setAvatarUploading(true);
    setAvatarError(undefined);
    try {
      const fileResponse = await fetch(asset.uri);
      const blob = await fileResponse.blob();
      const contentType = asset.mimeType ?? 'image/jpeg';

      const ticket = await api.post<AvatarUploadTicket>('/me/avatar/upload-url', {
        content_type: contentType,
        content_length: asset.fileSize ?? blob.size,
      });

      const putResponse = await fetch(ticket.upload_url, {
        method: 'PUT',
        headers: { 'Content-Type': contentType },
        body: blob,
      });

      if (!putResponse.ok) {
        throw new Error('avatar upload PUT failed');
      }

      setAvatarPublicUrl(ticket.public_url);
    } catch (err) {
      setAvatarError(err instanceof ApiError ? err.userMessage : "Couldn't upload your photo. Try again.");
    } finally {
      setAvatarUploading(false);
    }
  }

  const nameInvalid = name.trim().length === 0 || Boolean(nameError);
  const ctaDisabled = nameInvalid;

  return (
    <Screen scroll={false}>
      <KeyboardAvoidingView style={{ flex: 1 }} behavior={Platform.OS === 'ios' ? 'padding' : 'height'}>
        <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{ paddingBottom: space.lg }}>
          <View style={{ marginTop: space.lg }}>
            <AppText role="heading">Create your profile</AppText>
          </View>

          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              marginTop: space.xl,
              gap: space.sm,
            }}
          >
            <Pressable
              onPress={handleAvatarPress}
              accessibilityRole="button"
              accessibilityLabel="Add profile photo"
              style={{
                width: AVATAR_SIZE,
                height: AVATAR_SIZE,
                borderRadius: radius.full,
                backgroundColor: color.secondary,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {avatarUri ? (
                <Image
                  source={{ uri: avatarUri }}
                  style={{ width: AVATAR_SIZE, height: AVATAR_SIZE, borderRadius: radius.full }}
                  contentFit="cover"
                />
              ) : (
                <Ionicons name="person" size={AVATAR_GLYPH_SIZE} color={color.inkAvatarPlaceholder} />
              )}

              {avatarUploading ? (
                <View
                  style={{
                    position: 'absolute',
                    width: AVATAR_SIZE,
                    height: AVATAR_SIZE,
                    borderRadius: radius.full,
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <ActivityIndicator color={color.ink} />
                </View>
              ) : null}

              <View
                style={{ position: 'absolute', bottom: 0, right: 0 }}
                accessibilityElementsHidden
                importantForAccessibility="no-hide-descendants"
              >
                <IconBadge name="create-outline" surface="secondary" accessibilityLabel="Add profile photo" />
              </View>
            </Pressable>

            <AppText role="label" tone="muted">
              Optional
            </AppText>
          </View>

          {avatarError ? (
            <View style={{ marginTop: space.sm, alignItems: 'center' }}>
              <AppText role="label" tone="destructive">
                {avatarError}
              </AppText>
            </View>
          ) : null}

          <View style={{ marginTop: space.xl }}>
            <TextField
              label="Name"
              autoCapitalize="words"
              autoComplete="name"
              textContentType="name"
              value={name}
              onChangeText={setName}
              onBlur={handleNameBlur}
              error={nameError}
            />
          </View>

          <View style={{ marginTop: space.md }}>
            <TextField label="Username" autoCapitalize="none" autoCorrect={false} value={username} onChangeText={setUsername} />
            {/* Reserved 24dp status row: Task 2 fills this with the
                checking/available/taken state machine. Kept empty and
                fixed-height here so the bio field below never jumps once
                that state machine starts driving content into it. */}
            <View style={{ minHeight: 24, justifyContent: 'center', marginTop: 4 }} />
          </View>

          <View style={{ marginTop: space.md }}>
            <TextField
              label="Bio"
              placeholder="Tell people what you're rolling for."
              value={bio}
              onChangeText={(text) => setBio(text.slice(0, BIO_MAX_LENGTH))}
              multiline
              numberOfLines={3}
            />
          </View>
        </ScrollView>

        <View style={{ paddingTop: space.md, paddingBottom: space.md }}>
          <PrimaryButton label="Finish setup" disabled={ctaDisabled} />
        </View>
      </KeyboardAvoidingView>
    </Screen>
  );
}
