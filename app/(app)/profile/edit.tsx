import { useEffect, useRef, useState } from 'react';
import { Alert, Modal, Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { DialAvatar } from '@/components/brand/DialAvatar';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { TextField } from '@/components/ui/TextField';
import { color, elevation, radius, space, type } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import { updateProfile, uploadAvatar, type ProfilePatch } from '@/lib/api/profile';
import type { UsernameAvailability } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';

const BIO_MAX_LENGTH = 160;
const USERNAME_DEBOUNCE_MS = 400;
const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";

// Without alternates the "Try one of these:" lead-in would point at nothing.
function takenMessage(alternates: string[]): string {
  return alternates.length > 0 ? USERNAME_TAKEN_MESSAGE : USERNAME_TAKEN_NO_ALTERNATES_MESSAGE;
}

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
  const { user, reloadUser, signOut } = useSession();
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

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
      setUsernameError('3 to 20 letters, numbers, or underscores.');
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
        setUsernameError(takenMessage(result.alternates));
      }
    } catch {
      if (usernameRef.current !== candidate) return;
      setUsernameStatus('idle');
      setUsernameAlternates([]);
      setUsernameError(USERNAME_CHECK_FAILED_MESSAGE);
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

  // Both launchers reject on devices without a camera (simulators) or when
  // the picker fails to open; surface that as the avatar error instead of an
  // unhandled rejection from the Alert button handler.
  async function pickFromCamera() {
    try {
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
    } catch {
      setAvatarError("Camera isn't available on this device.");
    }
  }

  async function pickFromLibrary() {
    try {
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
    } catch {
      setAvatarError("Couldn't open your photo library.");
    }
  }

  function applyPickedAsset(result: ImagePicker.ImagePickerResult) {
    if (result.canceled || !result.assets || result.assets.length === 0) return;
    const asset = result.assets[0];
    setPendingAvatarUri(asset.uri);
    setPendingAvatarMimeType(asset.mimeType ?? 'image/jpeg');
    setAvatarError(undefined);
  }

  // The presigned PUT signs ContentLength, so it must be the size of the
  // bytes actually uploaded, not the picker's asset.fileSize (which can
  // differ after the picker re-encodes at quality 0.8).
  async function resolveContentLength(uri: string): Promise<number> {
    const response = await fetch(uri);
    const blob = await response.blob();
    return blob.size;
  }

  const trimmedName = name.trim();
  const trimmedBio = bio.trim();

  const hasChanges =
    trimmedName !== initialName.trim() ||
    username !== initialUsername ||
    trimmedBio !== initialBio.trim() ||
    pendingAvatarUri !== null;

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
      if (trimmedName !== initialName.trim()) patch.name = trimmedName;
      if (username !== initialUsername) patch.username = username;
      if (trimmedBio !== initialBio.trim()) patch.bio = trimmedBio;

      if (pendingAvatarUri) {
        const contentLength = await resolveContentLength(pendingAvatarUri);
        const publicUrl = await uploadAvatar(pendingAvatarUri, pendingAvatarMimeType, contentLength);
        patch.avatar_url = publicUrl;
      }

      await updateProfile(patch);
      await reloadUser();
      // This page is also the app's landing page, so there may be nothing to go back to.
      if (router.canGoBack()) router.back();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'username_taken') {
        // Insert-time conflict: the live check passed but the save still
        // raced into a taken username. Re-render the same taken state with
        // the fresh alternates from the 409 body, not a generic error.
        setUsernameStatus('taken');
        const suggestions = err.suggestions ?? [];
        setUsernameAlternates(suggestions);
        setUsernameError(takenMessage(suggestions));
      } else {
        setSaveError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
      }
    } finally {
      setSaving(false);
    }
  }

  const avatarPreviewUri = pendingAvatarUri ?? initialAvatarUrl;

  async function handleConfirmSignOut() {
    setSigningOut(true);
    try {
      await signOut();
      // The root guard routes back to Welcome once status flips to unauthenticated.
    } finally {
      setSigningOut(false);
      setConfirmingSignOut(false);
    }
  }

  return (
    <Screen wheels="corner">
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
          {router.canGoBack() ? (
          <Pressable
            onPress={() => router.back()}
            accessibilityRole="button"
            accessibilityLabel="Back"
            hitSlop={12}
            style={{ flexDirection: 'row', alignItems: 'center' }}
          >
            <Ionicons name="chevron-back" size={22} color={color.ink} />
            <AppText role="body">Back</AppText>
          </Pressable>
          ) : (
            <View />
          )}
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <Text
              style={{
                fontFamily: type.label.fontFamily,
                fontSize: type.label.fontSize - 3,
                letterSpacing: 3,
                textTransform: 'uppercase',
                color: color.muted,
              }}
            >
              RNDMRoll
            </Text>
            <View style={{ width: 28, height: 1, marginLeft: space.sm, backgroundColor: color.muted }} />
          </View>
        </View>

        <View style={{ marginTop: space.xl }}>
          <AppText role="display">Make it yours.</AppText>
          <View style={{ marginTop: space.sm }}>
            <AppText role="body" tone="muted">
              This is where your daily spins will live.
            </AppText>
          </View>
        </View>

        <View style={{ marginTop: space.lg, alignItems: 'center' }}>
          <DialAvatar
            name={name}
            uri={avatarPreviewUri}
            placeholder="camera"
            ring={false}
            badge
            onPress={handleChangePhoto}
            accessibilityLabel="Change profile photo"
          />
          <View style={{ marginTop: space.sm }}>
            <TextButton label={avatarPreviewUri ? 'Change photo' : 'Add photo'} tone="muted" onPress={handleChangePhoto} />
          </View>
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
            variant="soft"
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
            variant="soft"
            prefix="@"
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
            variant="soft"
            value={bio}
            onChangeText={setBio}
            onBlur={validateBioOnBlur}
            error={bioError}
            placeholder="Tell people what you're spinning for."
            multiline
            numberOfLines={3}
            maxLength={BIO_MAX_LENGTH}
            showCount
            clearable
            hint="Optional"
            textAlignVertical="top"
          />
        </View>

        {saveError ? (
          <View style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {saveError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Save changes" arrow onPress={handleSave} disabled={!canSubmit} loading={saving} />
        </View>

        <View style={{ marginTop: space.lg, alignItems: 'center' }}>
          <TextButton label="Log out" tone="destructive" onPress={() => setConfirmingSignOut(true)} />
        </View>
      </View>

      <Modal
        visible={confirmingSignOut}
        transparent
        animationType="fade"
        onRequestClose={() => setConfirmingSignOut(false)}
      >
        <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
          <View
            style={{
              ...elevation.card,
              borderTopLeftRadius: radius.lg,
              borderTopRightRadius: radius.lg,
              padding: space.lg,
              paddingBottom: space.xl,
            }}
          >
            <AppText role="body">Log out of RNDMRoll? You'll need to sign back in.</AppText>
            <View style={{ marginTop: space.lg, alignItems: 'center' }}>
              <TextButton label="Log out" tone="destructive" disabled={signingOut} onPress={handleConfirmSignOut} />
            </View>
            <View style={{ marginTop: space.md, alignItems: 'center' }}>
              <TextButton label="Stay logged in" disabled={signingOut} onPress={() => setConfirmingSignOut(false)} />
            </View>
          </View>
        </View>
      </Modal>
    </Screen>
  );
}
