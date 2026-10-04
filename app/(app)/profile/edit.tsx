import { useEffect, useRef, useState } from 'react';
import { Alert, Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Image } from 'expo-image';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { color, elevation, radius, space } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import { updateProfile, uploadAvatar, type ProfilePatch } from '@/lib/api/profile';
import type { UsernameAvailability } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';

const AVATAR_DIAMETER = 96;
const AVATAR_GLYPH_SIZE = 40;
const BIO_MAX_LENGTH = 160;
const USERNAME_DEBOUNCE_MS = 400;
const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken';

/**
 * D-08's profile edit screen: all four fields (name, username, bio, photo)
 * editable from one screen, saving only whatever actually changed. The
 * username field reuses the create-profile screen's state machine
 * (01-UI-SPEC.md "Create your profile screen state machine") verbatim in
 * behavior: a 400ms-debounced live availability check, checking/available/
 * taken states, tappable alternates, and the same save-time conflict
 * handling if the check passed but the insert still races into a taken
 * username. Every input stays on the standard input radius cap; this
 * screen never renders the profile-view/log-out sheet's larger container
 * token.
 */
export default function EditProfileScreen() {
  const router = useRouter();
  const { user, reloadUser } = useSession();

  const initialName = user?.name ?? '';
  const initialUsername = user?.username ?? '';
  const initialBio = user?.bio ?? '';
  const initialAvatarUrl = user?.avatar_url ?? null;

  const [name, setName] = useState(initialName);
  const [nameError, setNameError] = useState<string | undefined>(undefined);

  const [username, setUsername] = useState(initialUsername);
  const [usernameStatus, setUsernameStatus] = useState<UsernameStatus>('idle');
  const [usernameError, setUsernameError] = useState<string | undefined>(undefined);
  const [usernameAlternates, setUsernameAlternates] = useState<string[]>([]);

  const [bio, setBio] = useState(initialBio);
  const [bioError, setBioError] = useState<string | undefined>(undefined);

  const [pendingAvatarUri, setPendingAvatarUri] = useState<string | null>(null);
  const [pendingAvatarMimeType, setPendingAvatarMimeType] = useState<string>('image/jpeg');
  const [pendingAvatarFileSize, setPendingAvatarFileSize] = useState<number | undefined>(undefined);
  const [avatarError, setAvatarError] = useState<string | undefined>(undefined);

  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | undefined>(undefined);

  const usernameRef = useRef(username);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    usernameRef.current = username;
  }, [username]);

  // Live, 400ms-debounced availability check -- the one field validated
  // live rather than on blur, per UI-SPEC's form validation timing rule.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (username === initialUsername) {
      // Back to the stored value: nothing to check, and it is never "taken"
      // against itself.
      setUsernameStatus('idle');
      setUsernameError(undefined);
      setUsernameAlternates([]);
      return;
    }

    if (!USERNAME_PATTERN.test(username)) {
      setUsernameStatus('idle');
      setUsernameError('3-20 lowercase letters, numbers, or underscores.');
      setUsernameAlternates([]);
      return;
    }

    setUsernameError(undefined);
    setUsernameStatus('checking');
    debounceRef.current = setTimeout(() => {
      checkUsernameAvailability(username);
    }, USERNAME_DEBOUNCE_MS);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [username]);

  async function checkUsernameAvailability(candidate: string) {
    try {
      const result = await api.get<UsernameAvailability>(
        `/usernames/available?username=${encodeURIComponent(candidate)}`
      );
      if (usernameRef.current !== candidate) return; // stale response, field moved on
      if (result.available) {
        setUsernameStatus('available');
        setUsernameAlternates([]);
        setUsernameError(undefined);
      } else {
        setUsernameStatus('taken');
        setUsernameAlternates(result.alternates);
        setUsernameError(USERNAME_TAKEN_MESSAGE);
      }
    } catch {
      if (usernameRef.current !== candidate) return;
      setUsernameStatus('idle');
    }
  }

  function handleAlternatePress(alternate: string) {
    setUsername(alternate);
  }

  function validateNameOnBlur() {
    if (name.trim().length === 0) {
      setNameError('Name is required.');
    } else if (name.length > 50) {
      setNameError('Name must be 50 characters or fewer.');
    } else {
      setNameError(undefined);
    }
  }

  function validateBioOnBlur() {
    if (bio.length > BIO_MAX_LENGTH) {
      setBioError(`Bio must be ${BIO_MAX_LENGTH} characters or fewer.`);
    } else {
      setBioError(undefined);
    }
  }

  function handleChangePhoto() {
    Alert.alert('Change photo', undefined, [
      { text: 'Take photo', onPress: () => void pickFromCamera() },
      { text: 'Choose from library', onPress: () => void pickFromLibrary() },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  async function pickFromCamera() {
    const permission = await ImagePicker.requestCameraPermissionsAsync();
    if (!permission.granted) {
      setAvatarError('Camera access is needed to take a photo.');
      return;
    }
    const result = await ImagePicker.launchCameraAsync({
      mediaTypes: ImagePicker.MediaTypeOptions.Images,
      quality: 0.8,
    });
    applyPickedAsset(result);
  }

  async function pickFromLibrary() {
    const permission = await ImagePicker.requestMediaLibraryPermissionsAsync();
    if (!permission.granted) {
      setAvatarError('Photo library access is needed to choose a photo.');
      return;
    }
    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ImagePicker.MediaTypeOptions.Images,
      quality: 0.8,
    });
    applyPickedAsset(result);
  }

  function applyPickedAsset(result: ImagePicker.ImagePickerResult) {
    if (result.canceled || !result.assets || result.assets.length === 0) return;
    const asset = result.assets[0];
    setPendingAvatarUri(asset.uri);
    setPendingAvatarMimeType(asset.mimeType ?? 'image/jpeg');
    setPendingAvatarFileSize(asset.fileSize);
    setAvatarError(undefined);
  }

  async function resolveContentLength(uri: string, fallback?: number): Promise<number> {
    if (fallback && fallback > 0) return fallback;
    const response = await fetch(uri);
    const blob = await response.blob();
    return blob.size;
  }

  const hasChanges =
    name !== initialName || username !== initialUsername || bio !== initialBio || pendingAvatarUri !== null;

  const nameValid = name.trim().length > 0 && name.length <= 50;
  const usernameValid = username === initialUsername || USERNAME_PATTERN.test(username);
  const usernameBlocking = usernameStatus === 'checking' || usernameStatus === 'taken';

  const canSubmit = hasChanges && nameValid && usernameValid && !usernameBlocking && !saving;

  async function handleSave() {
    if (!canSubmit) return;

    setSaving(true);
    setSaveError(undefined);

    try {
      const patch: ProfilePatch = {};
      if (name !== initialName) patch.name = name;
      if (username !== initialUsername) patch.username = username;
      if (bio !== initialBio) patch.bio = bio;

      if (pendingAvatarUri) {
        const contentLength = await resolveContentLength(pendingAvatarUri, pendingAvatarFileSize);
        const publicUrl = await uploadAvatar(pendingAvatarUri, pendingAvatarMimeType, contentLength);
        patch.avatar_url = publicUrl;
      }

      await updateProfile(patch);
      await reloadUser();
      router.back();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'username_taken') {
        // Insert-time conflict: the live check passed but the save still
        // raced into a taken username. Re-render the same taken state with
        // the fresh alternates from the 409 body, not a generic error.
        setUsernameStatus('taken');
        setUsernameAlternates(err.suggestions ?? []);
        setUsernameError(USERNAME_TAKEN_MESSAGE);
      } else {
        setSaveError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
      }
    } finally {
      setSaving(false);
    }
  }

  const avatarPreviewUri = pendingAvatarUri ?? initialAvatarUrl;

  return (
    <Screen>
      <View style={{ paddingTop: space.xl, paddingBottom: space.xl }}>
        <AppText role="heading">Edit profile</AppText>

        <View style={{ marginTop: space.lg, alignItems: 'center' }}>
          <Pressable
            onPress={handleChangePhoto}
            accessibilityRole="button"
            accessibilityLabel="Change profile photo"
            style={{
              width: AVATAR_DIAMETER,
              height: AVATAR_DIAMETER,
              borderRadius: radius.full,
              backgroundColor: color.secondary,
              alignItems: 'center',
              justifyContent: 'center',
              overflow: 'hidden',
            }}
          >
            {avatarPreviewUri ? (
              <Image
                source={{ uri: avatarPreviewUri }}
                style={{ width: AVATAR_DIAMETER, height: AVATAR_DIAMETER }}
                contentFit="cover"
                accessibilityLabel="Profile photo"
              />
            ) : (
              <Ionicons name="person" size={AVATAR_GLYPH_SIZE} color={color.inkAvatarPlaceholder} />
            )}
          </Pressable>
          {avatarError ? (
            <View style={{ marginTop: space.xs }}>
              <AppText role="label" tone="destructive">
                {avatarError}
              </AppText>
            </View>
          ) : null}
        </View>

        <View style={{ marginTop: space.lg }}>
          <TextField
            label="Name"
            value={name}
            onChangeText={setName}
            onBlur={validateNameOnBlur}
            error={nameError}
            autoCapitalize="words"
            autoComplete="name"
            textContentType="name"
          />
        </View>

        <View style={{ marginTop: space.md }}>
          <TextField
            label="Username"
            value={username}
            onChangeText={(value) => setUsername(value.toLowerCase())}
            status={usernameStatus}
            error={usernameError}
            autoCapitalize="none"
            autoCorrect={false}
          />
          {usernameStatus === 'taken' && usernameAlternates.length > 0 ? (
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', marginTop: space.xs, gap: space.xs }}>
              {usernameAlternates.map((alternate) => (
                <Pressable
                  key={alternate}
                  onPress={() => handleAlternatePress(alternate)}
                  accessibilityRole="button"
                  accessibilityLabel={`Use username ${alternate}`}
                  style={{
                    ...elevation.subtle,
                    borderRadius: radius.sm,
                    paddingHorizontal: space.sm,
                    paddingVertical: space.xs,
                  }}
                >
                  <AppText role="label">{alternate}</AppText>
                </Pressable>
              ))}
            </View>
          ) : null}
        </View>

        <View style={{ marginTop: space.md }}>
          <TextField
            label="Bio"
            value={bio}
            onChangeText={setBio}
            onBlur={validateBioOnBlur}
            error={bioError}
            placeholder="Tell people what you're spinning for."
            multiline
            numberOfLines={3}
            maxLength={BIO_MAX_LENGTH}
            textAlignVertical="top"
          />
          <View style={{ marginTop: space.xs, alignItems: 'flex-end' }}>
            <AppText role="label" tone="muted">
              {`${BIO_MAX_LENGTH - bio.length} characters left`}
            </AppText>
          </View>
        </View>

        {saveError ? (
          <View style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {saveError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Save changes" onPress={handleSave} disabled={!canSubmit} loading={saving} />
        </View>
      </View>
    </Screen>
  );
}
