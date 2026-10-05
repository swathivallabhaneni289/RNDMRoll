import { useEffect, useRef, useState } from 'react';
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
import type { ApiUser, AvatarUploadTicket, UsernameAvailability, UsernameSuggestion } from '@/lib/api/types';
import { clearDraft, getDraft } from '@/lib/onboarding/draft';
import { useSession } from '@/lib/session/store';

/**
 * D-05 (revised 2026-09-17) step 3: the consolidated "Create your profile"
 * screen, replacing the prior separate name/username/photo steps. Built
 * across two plan tasks: Task 1 established the screen shell -- hero avatar
 * with immediate upload, name field, bio field, and the pinned CTA -- and
 * positioned the username field with its reserved status row. Task 2 (this
 * pass) wires the username suggestion/400ms-debounce/five-state machine
 * into that field and row, and owns the single authoritative PATCH /me
 * save. Every endpoint, debounce value, and state name here is carried over
 * unchanged from the screen this one consolidates -- see 01-11-SUMMARY.md
 * for the exact wire contract this depends on.
 */

const AVATAR_SIZE = 120;
const AVATAR_GLYPH_SIZE = 56;
const BIO_MAX_LENGTH = 160;
const NAME_MAX_LENGTH = 50;
const USERNAME_DEBOUNCE_MS = 400;
const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";
const CAMERA_UNAVAILABLE_MESSAGE = "Camera isn't available on this device.";
const LIBRARY_UNAVAILABLE_MESSAGE = "Couldn't open your photo library.";

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken' | 'error';

function validateName(value: string): string | undefined {
  if (value.length === 0) return 'Enter your name.';
  if (value.length > NAME_MAX_LENGTH) return `Name must be ${NAME_MAX_LENGTH} characters or fewer.`;
  return undefined;
}

function validateUsernameFormat(value: string): string | undefined {
  if (!USERNAME_PATTERN.test(value)) {
    return '3 to 20 letters, numbers, or underscores.';
  }
  return undefined;
}

export default function ProfileSetupScreen() {
  const { reloadUser, user } = useSession();

  // The draft only holds a name from a first-time Apple sign-in; fall back to
  // the name the server already has so Google users and a relaunched app do
  // not see a blank field.
  const [name, setName] = useState(() => getDraft().suggestedName || user?.name?.trim() || '');
  const [nameError, setNameError] = useState<string | undefined>();

  const [username, setUsername] = useState('');
  const [usernameStatus, setUsernameStatus] = useState<UsernameStatus>('idle');
  const [usernameFormatError, setUsernameFormatError] = useState<string | undefined>();
  const [alternates, setAlternates] = useState<string[]>([]);

  const [bio, setBio] = useState('');

  const [avatarUri, setAvatarUri] = useState<string | undefined>();
  const [avatarPublicUrl, setAvatarPublicUrl] = useState<string | undefined>();
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [avatarError, setAvatarError] = useState<string | undefined>();

  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>();

  // A suggestion is fetched exactly once (mount, or the Name field's first
  // blur -- whichever comes first) per D-05: the username field must never
  // render blank. usernameEditedRef guards against that suggestion landing
  // late and clobbering a value the user has since typed themselves.
  const suggestionFetchedRef = useRef(false);
  const nameEditedRef = useRef(false);
  const usernameEditedRef = useRef(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Bumped on every check dispatch so a slow, stale response (superseded by
  // a newer keystroke's check) can be discarded on arrival instead of
  // clobbering the state a faster, more recent check already set.
  const checkTokenRef = useRef(0);
  // Same idea for avatar uploads: only the newest upload may write state.
  const uploadTokenRef = useRef(0);

  useEffect(() => {
    if (name.trim().length > 0) {
      suggestionFetchedRef.current = true;
      fetchSuggestion(name.trim());
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The stored user can land after first render; fill Name then, but never
  // over text the user has typed.
  const storedName = user?.name?.trim() ?? '';
  useEffect(() => {
    if (nameEditedRef.current || !storedName) return;
    setName((current) => (current.length === 0 ? storedName : current));
    if (!suggestionFetchedRef.current) {
      suggestionFetchedRef.current = true;
      fetchSuggestion(storedName);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storedName]);

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  async function fetchSuggestion(displayName: string) {
    // Show checking only when the suggestion can land: if the user already typed a username
    // the result is ignored below and nothing would ever reset this status.
    if (!usernameEditedRef.current) setUsernameStatus('checking');
    try {
      const result = await api.get<UsernameSuggestion>(`/usernames/suggest?name=${encodeURIComponent(displayName)}`);
      if (!usernameEditedRef.current) {
        setUsername(result.username);
        setUsernameStatus('idle');
        setUsernameFormatError(undefined);
        setAlternates([]);
      }
    } catch {
      // A failed suggestion is not a failed check of anything the user typed,
      // so drop back to idle and leave the field blank.
      if (!usernameEditedRef.current) {
        setUsernameStatus('idle');
      }
    }
  }

  async function runAvailabilityCheck(value: string) {
    setUsernameStatus('checking');
    checkTokenRef.current += 1;
    const token = checkTokenRef.current;
    try {
      const result = await api.get<UsernameAvailability>(`/usernames/available?username=${encodeURIComponent(value)}`);
      if (checkTokenRef.current !== token) return;
      if (result.available) {
        setUsernameStatus('available');
        setAlternates([]);
      } else {
        setUsernameStatus('taken');
        setAlternates(result.alternates);
      }
    } catch {
      if (checkTokenRef.current !== token) return;
      setUsernameStatus('error');
    }
  }

  function handleUsernameChange(rawText: string) {
    const text = rawText.toLowerCase();
    usernameEditedRef.current = true;
    setUsername(text);
    setAlternates([]);
    // Idle straight away so no stale taken/available text lingers through the
    // debounce, and bump the token so an in-flight check for the old text is
    // discarded when it lands.
    setUsernameStatus('idle');
    checkTokenRef.current += 1;
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const formatError = validateUsernameFormat(text);
    if (formatError) {
      setUsernameFormatError(formatError);
      return;
    }
    setUsernameFormatError(undefined);
    debounceRef.current = setTimeout(() => runAvailabilityCheck(text), USERNAME_DEBOUNCE_MS);
  }

  function handleAlternateTap(alternate: string) {
    usernameEditedRef.current = true;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    setUsername(alternate);
    setUsernameFormatError(undefined);
    runAvailabilityCheck(alternate);
  }

  function handleNameBlur() {
    setNameError(validateName(name.trim()));
    if (!suggestionFetchedRef.current && name.trim().length > 0) {
      suggestionFetchedRef.current = true;
      fetchSuggestion(name.trim());
    }
  }

  async function handleSubmit() {
    if (ctaDisabled || submitting) return;

    setSubmitting(true);
    setSubmitError(undefined);

    const body: { name: string; username: string; bio?: string; avatar_url?: string } = {
      name: name.trim(),
      username,
    };
    if (bio.trim().length > 0) body.bio = bio.trim();
    if (avatarPublicUrl) body.avatar_url = avatarPublicUrl;

    try {
      await api.patch<ApiUser>('/me', body);
      await reloadUser();
      clearDraft();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'username_taken') {
        setUsernameStatus('taken');
        setAlternates(err.suggestions ?? []);
      } else if (err instanceof ApiError) {
        setSubmitError(err.userMessage);
      } else {
        setSubmitError('Something went wrong. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  function handleAvatarPress() {
    if (avatarUploading) return;
    Alert.alert('Add profile photo', undefined, [
      { text: 'Take photo', onPress: () => pickImage('camera') },
      { text: 'Choose from library', onPress: () => pickImage('library') },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  async function pickImage(source: 'camera' | 'library') {
    setAvatarError(undefined);

    let result: ImagePicker.ImagePickerResult;
    // The launch throws where there is no camera (the iOS simulator), and the
    // permission call can reject too; neither may escape as an unhandled
    // rejection from the action sheet's onPress.
    try {
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

      result =
        source === 'camera'
          ? await ImagePicker.launchCameraAsync(pickerOptions)
          : await ImagePicker.launchImageLibraryAsync(pickerOptions);
    } catch {
      setAvatarError(source === 'camera' ? CAMERA_UNAVAILABLE_MESSAGE : LIBRARY_UNAVAILABLE_MESSAGE);
      return;
    }

    if (result.canceled || !result.assets?.[0]) return;

    await uploadAvatar(result.assets[0]);
  }

  async function uploadAvatar(asset: ImagePicker.ImagePickerAsset) {
    uploadTokenRef.current += 1;
    const token = uploadTokenRef.current;
    // A failed replacement must not throw away a photo that already uploaded fine.
    const previousUri = avatarUri;
    const previousUrl = avatarPublicUrl;
    setAvatarUri(asset.uri);
    setAvatarUploading(true);
    setAvatarError(undefined);
    try {
      const fileResponse = await fetch(asset.uri);
      const blob = await fileResponse.blob();
      const contentType = asset.mimeType ?? 'image/jpeg';

      const ticket = await api.post<AvatarUploadTicket>('/me/avatar/upload-url', {
        content_type: contentType,
        // The presigned PUT's bound content-length must match the bytes
        // actually sent in the request body below (the blob), not the
        // picker's own asset metadata -- which can disagree once
        // `quality: 0.8` re-compresses the original file.
        content_length: blob.size,
      });

      const putResponse = await fetch(ticket.upload_url, {
        method: 'PUT',
        headers: { 'Content-Type': contentType },
        body: blob,
      });

      if (!putResponse.ok) {
        throw new Error('avatar upload PUT failed');
      }

      if (uploadTokenRef.current !== token) return;
      setAvatarPublicUrl(ticket.public_url);
    } catch (err) {
      if (uploadTokenRef.current !== token) return;
      // Go back to the last photo that uploaded fine (or none), so the preview never shows
      // a photo that will not be saved.
      setAvatarUri(previousUri);
      setAvatarPublicUrl(previousUrl);
      setAvatarError(err instanceof ApiError ? err.userMessage : "Couldn't upload your photo. Try again.");
    } finally {
      if (uploadTokenRef.current === token) setAvatarUploading(false);
    }
  }

  const nameInvalid = name.trim().length === 0 || Boolean(nameError);
  const usernameInvalid = username.trim().length === 0 || Boolean(usernameFormatError);
  const ctaDisabled = nameInvalid || usernameInvalid || usernameStatus === 'checking' || avatarUploading || submitting;

  function renderUsernameStatus() {
    if (usernameFormatError) {
      return (
        <AppText role="label" tone="destructive">
          {usernameFormatError}
        </AppText>
      );
    }
    if (usernameStatus === 'checking') {
      return (
        <View accessible accessibilityLabel="Checking availability" style={{ flexDirection: 'row', alignItems: 'center' }}>
          <ActivityIndicator size="small" color={color.ink} />
          <AppText role="label" tone="muted">
            {' Checking...'}
          </AppText>
        </View>
      );
    }
    if (usernameStatus === 'available') {
      return (
        <View accessible accessibilityLabel="Username available" style={{ flexDirection: 'row', alignItems: 'center' }}>
          <Ionicons name="checkmark-circle" size={16} color={color.success} />
          <AppText role="label" tone="success">
            {' Available'}
          </AppText>
        </View>
      );
    }
    if (usernameStatus === 'taken') {
      return (
        <AppText role="label" tone="destructive">
          {alternates.length > 0 ? USERNAME_TAKEN_MESSAGE : USERNAME_TAKEN_NO_ALTERNATES_MESSAGE}
        </AppText>
      );
    }
    if (usernameStatus === 'error') {
      return (
        <AppText role="label" tone="destructive">
          {USERNAME_CHECK_FAILED_MESSAGE}
        </AppText>
      );
    }
    return null;
  }

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
              disabled={avatarUploading}
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
              onChangeText={(text) => {
                nameEditedRef.current = true;
                setName(text);
              }}
              onBlur={handleNameBlur}
              error={nameError}
            />
          </View>

          <View style={{ marginTop: space.md }}>
            <TextField
              label="Username"
              autoCapitalize="none"
              autoCorrect={false}
              autoComplete="username"
              textContentType="username"
              value={username}
              onChangeText={handleUsernameChange}
            />
            {/* Fixed 24dp status row; every status string fits one label line,
                so none of them moves the bio field below it. The taken state is the
                one explicit exception: its alternate-chip row is allowed to
                push content below it downward. */}
            <View style={{ height: 24, justifyContent: 'center', marginTop: 4 }}>
              {renderUsernameStatus()}
            </View>
            {usernameStatus === 'taken' && alternates.length > 0 ? (
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: space.xs, marginTop: space.xs }}>
                {alternates.map((alternate) => (
                  <Pressable
                    key={alternate}
                    onPress={() => handleAlternateTap(alternate)}
                    accessibilityRole="button"
                    accessibilityLabel={`Use username ${alternate}`}
                    style={{
                      backgroundColor: color.secondary,
                      borderRadius: radius.sm,
                      paddingHorizontal: space.sm,
                      paddingVertical: 4,
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
              placeholder="Tell people what you're spinning for."
              value={bio}
              onChangeText={(text) => setBio(text.slice(0, BIO_MAX_LENGTH))}
              multiline
              numberOfLines={3}
              maxLength={BIO_MAX_LENGTH}
              showCount
            />
          </View>
        </ScrollView>

        <View style={{ paddingTop: space.md, paddingBottom: space.md }}>
          {submitError ? (
            <View style={{ marginBottom: space.sm, alignItems: 'center' }}>
              <AppText role="label" tone="destructive">
                {submitError}
              </AppText>
            </View>
          ) : null}
          <PrimaryButton label="Finish setup" onPress={handleSubmit} disabled={ctaDisabled} loading={submitting} />
        </View>
      </KeyboardAvoidingView>
    </Screen>
  );
}
