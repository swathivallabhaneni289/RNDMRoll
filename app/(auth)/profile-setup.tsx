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

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken';

function validateName(value: string): string | undefined {
  if (value.length === 0) return 'Enter your name.';
  if (value.length > NAME_MAX_LENGTH) return `Name must be ${NAME_MAX_LENGTH} characters or fewer.`;
  return undefined;
}

function validateUsernameFormat(value: string): string | undefined {
  if (!USERNAME_PATTERN.test(value)) {
    return 'Use lowercase letters, numbers, and underscores, 3 to 20 characters.';
  }
  return undefined;
}

export default function ProfileSetupScreen() {
  const { reloadUser } = useSession();

  const [name, setName] = useState(() => getDraft().suggestedName ?? '');
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
  const usernameEditedRef = useRef(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Bumped on every check dispatch so a slow, stale response (superseded by
  // a newer keystroke's check) can be discarded on arrival instead of
  // clobbering the state a faster, more recent check already set.
  const checkTokenRef = useRef(0);

  useEffect(() => {
    const draft = getDraft();
    if (draft.suggestedName) {
      suggestionFetchedRef.current = true;
      fetchSuggestion(draft.suggestedName);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  async function fetchSuggestion(displayName: string) {
    setUsernameStatus('checking');
    try {
      const result = await api.get<UsernameSuggestion>(`/usernames/suggest?name=${encodeURIComponent(displayName)}`);
      if (!usernameEditedRef.current) {
        setUsername(result.username);
        setUsernameStatus('idle');
        setUsernameFormatError(undefined);
        setAlternates([]);
      }
    } catch {
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
      setUsernameStatus('idle');
    }
  }

  function handleUsernameChange(text: string) {
    usernameEditedRef.current = true;
    setUsername(text);
    setAlternates([]);
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const formatError = validateUsernameFormat(text);
    if (formatError) {
      setUsernameFormatError(formatError);
      setUsernameStatus('idle');
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
    if (bio.trim().length > 0) body.bio = bio;
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
  const usernameInvalid = username.trim().length === 0 || Boolean(usernameFormatError);
  const ctaDisabled = nameInvalid || usernameInvalid || usernameStatus === 'checking' || submitting;

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
          {USERNAME_TAKEN_MESSAGE}
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
            <TextField
              label="Username"
              autoCapitalize="none"
              autoCorrect={false}
              autoComplete="username"
              textContentType="username"
              value={username}
              onChangeText={handleUsernameChange}
            />
            {/* Fixed-height 24dp status row, reserved across idle/checking/
                available so the bio field below never jumps. The taken
                state is the one explicit exception: its alternate-chip row
                is allowed to push content below it downward. */}
            <View style={{ minHeight: 24, justifyContent: 'center', marginTop: 4 }}>{renderUsernameStatus()}</View>
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
              placeholder="Tell people what you're rolling for."
              value={bio}
              onChangeText={(text) => setBio(text.slice(0, BIO_MAX_LENGTH))}
              multiline
              numberOfLines={3}
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
