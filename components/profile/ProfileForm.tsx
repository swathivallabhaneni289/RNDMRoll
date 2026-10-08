import { useEffect, useRef, useState } from 'react';
import { Alert, Modal, Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { AppText, TextLink } from '@/components/ui/AppText';
import { DialAvatar } from '@/components/brand/DialAvatar';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { TextField } from '@/components/ui/TextField';
import { color, elevation, radius, space, type } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import { fetchProfile, updateProfile, uploadAvatar, type ProfilePatch } from '@/lib/api/profile';
import type { AuthResult, UsernameAvailability, UsernameSuggestion } from '@/lib/api/types';
import { BIRTHDAY_MESSAGES, checkBirthday, EMPTY_BIRTHDAY, type BirthdayParts } from '@/lib/profile/birthday';
import { useSession } from '@/lib/session/store';

/**
 * The one profile page, in four modes (plans 01-19 and 01-20):
 *   signup: signed out. Email, password, birthday, name, username. Continue.
 *   finish: signed in with an unfinished account (Apple or Google). Birthday, name, username.
 *           Continue, plus Log out.
 *   extras: signed in, right after signup or finish. Photo and bio, both optional. Continue
 *           or Skip for now. Shown once, in place of the landing page.
 *   edit:   signed in and complete. Photo, name, username, bio. Save changes, Log out.
 * The caller picks the mode once on mount and never changes it (the landing route remounts
 * the form with a new key when its page changes), so the signIn or signOut that ends a
 * submit cannot make this page show anything new while the screen swaps.
 * The look is the "Make it yours." edit page built in 6054474.
 */
export type ProfileFormMode = 'signup' | 'finish' | 'extras' | 'edit';

const BIO_MAX_LENGTH = 160;
const NAME_MAX_LENGTH = 50;
const EMAIL_MAX_LENGTH = 254;
const PASSWORD_MIN_LENGTH = 8;
const PASSWORD_MAX_BYTES = 72;
const USERNAME_DEBOUNCE_MS = 400;
const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const EMAIL_MESSAGE = 'Enter a valid email address.';
const PASSWORD_SHORT_MESSAGE = 'Password must be at least 8 characters.';
const PASSWORD_LONG_MESSAGE = 'That password is too long. Use 72 characters or fewer.';
const NAME_EMPTY_MESSAGE = 'Enter your name.';
const NAME_LONG_MESSAGE = `Name must be ${NAME_MAX_LENGTH} characters or fewer.`;
const BIO_LONG_MESSAGE = `Bio must be ${BIO_MAX_LENGTH} characters or fewer.`;
const USERNAME_FORMAT_MESSAGE = '3 to 20 letters, numbers, or underscores.';
const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";
const GENERIC_MESSAGE = 'Something went wrong. Please try again.';
const REFUSAL_TITLE = 'RNDMRoll is for ages 13 and up.';
const REFUSAL_MESSAGE = "We can't make an account for you. We didn't keep your birthday.";

// Without alternates the "Try one of these:" lead-in would point at nothing.
function takenMessage(alternates: string[]): string {
  return alternates.length > 0 ? USERNAME_TAKEN_MESSAGE : USERNAME_TAKEN_NO_ALTERNATES_MESSAGE;
}

// bcrypt (the server's hash) refuses more than 72 BYTES, and a field's maxLength counts
// UTF-16 units, so the byte count is taken by hand.
function utf8Length(value: string): number {
  let bytes = 0;
  for (const ch of value) {
    const cp = ch.codePointAt(0) ?? 0;
    bytes += cp < 0x80 ? 1 : cp < 0x800 ? 2 : cp < 0x10000 ? 3 : 4;
  }
  return bytes;
}

function emailProblem(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 && trimmed.length <= EMAIL_MAX_LENGTH && EMAIL_PATTERN.test(trimmed)
    ? undefined
    : EMAIL_MESSAGE;
}

function passwordProblem(value: string): string | undefined {
  if (value.length < PASSWORD_MIN_LENGTH) return PASSWORD_SHORT_MESSAGE;
  if (utf8Length(value) > PASSWORD_MAX_BYTES) return PASSWORD_LONG_MESSAGE;
  return undefined;
}

function nameProblem(value: string): string | undefined {
  const trimmed = value.trim();
  if (trimmed.length === 0) return NAME_EMPTY_MESSAGE;
  if (trimmed.length > NAME_MAX_LENGTH) return NAME_LONG_MESSAGE;
  return undefined;
}

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken';

export function ProfileForm({ mode }: { mode: ProfileFormMode }) {
  const router = useRouter();
  const { user, signIn, signOut, reloadUser, startExtras, finishExtras } = useSession();
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

  const signup = mode === 'signup';
  const edit = mode === 'edit';
  const extras = mode === 'extras';
  // Signup and finish ask who the person is; extras and edit are the profile pages.
  const joining = signup || mode === 'finish';
  // Finish mode leaves the 13+ rule to the server: an under-13 birthday must be sent so the
  // server can delete the unfinished account and the refusal path runs (plan step 18).
  const ageOptions = { ignoreAge: mode === 'finish' };

  // What is stored right now. A signup has nothing stored; after its signIn the session
  // user appears, but this page is already on its way out, so it keeps reading blanks.
  const initialName = signup ? '' : (user?.name ?? '');
  const initialUsername = signup ? '' : (user?.username ?? '');
  const initialBio = signup ? '' : (user?.bio ?? '');
  const initialAvatarUrl = signup ? null : (user?.avatar_url ?? null);

  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>(undefined);

  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | undefined>(undefined);
  const [showPassword, setShowPassword] = useState(false);

  const [birthday, setBirthday] = useState<BirthdayParts>(EMPTY_BIRTHDAY);
  const [birthdayError, setBirthdayError] = useState<string | undefined>(undefined);

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

  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | undefined>(undefined);

  const submittingRef = useRef(false);
  const usernameRef = useRef(username);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The suggestion is fetched once (Name blur, or mount when a name is already known). It
  // never overwrites a username the person typed, and a failed fetch re-arms it for the
  // next Name blur.
  const suggestionFetchedRef = useRef(false);
  const usernameEditedRef = useRef(false);
  // A photo that already uploaded is remembered by its local file, so a retry after a
  // failed later step never uploads it twice.
  const uploadedRef = useRef<{ uri: string; url: string } | null>(null);
  // Finish mode: the birthday is accepted once, so a retry skips straight to the rest.
  const birthdaySavedRef = useRef(false);

  useEffect(() => {
    usernameRef.current = username;
  }, [username]);

  useEffect(() => {
    if (joining && name.trim().length > 0 && username === '') {
      maybeSuggestUsername(name);
    }
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Live, 400ms-debounced availability check: the one field validated live rather than
  // on blur, per UI-SPEC's form validation timing rule.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (username === '' || (initialUsername !== '' && username === initialUsername)) {
      // Nothing typed, or back to the stored value: nothing to check, and it is never
      // "taken" against itself.
      setUsernameStatus('idle');
      setUsernameError(undefined);
      setUsernameAlternates([]);
      return;
    }

    if (!USERNAME_PATTERN.test(username)) {
      setUsernameStatus('idle');
      setUsernameError(USERNAME_FORMAT_MESSAGE);
      setUsernameAlternates([]);
      return;
    }

    setUsernameError(undefined);
    setUsernameStatus('checking');
    debounceRef.current = setTimeout(() => {
      void checkUsernameAvailability(username);
    }, USERNAME_DEBOUNCE_MS);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [username]);

  async function checkUsernameAvailability(candidate: string) {
    try {
      const result = await api.get<UsernameAvailability>(
        `/usernames/available?username=${encodeURIComponent(candidate)}`,
        { auth: false }
      );
      if (usernameRef.current !== candidate) return; // stale response, field moved on
      if (result.available) {
        setUsernameStatus('available');
        setUsernameAlternates([]);
        setUsernameError(undefined);
      } else {
        const alternates = result.alternates ?? [];
        setUsernameStatus('taken');
        setUsernameAlternates(alternates);
        setUsernameError(takenMessage(alternates));
      }
    } catch {
      if (usernameRef.current !== candidate) return;
      setUsernameStatus('idle');
      setUsernameAlternates([]);
      setUsernameError(USERNAME_CHECK_FAILED_MESSAGE);
    }
  }

  function maybeSuggestUsername(fromName: string) {
    const trimmed = fromName.trim();
    if (!joining || suggestionFetchedRef.current || usernameEditedRef.current || trimmed.length === 0) return;
    suggestionFetchedRef.current = true;
    void fetchSuggestion(trimmed);
  }

  async function fetchSuggestion(displayName: string) {
    try {
      const result = await api.get<UsernameSuggestion>(
        `/usernames/suggest?name=${encodeURIComponent(displayName)}`,
        { auth: false }
      );
      // Setting the username runs the availability check above.
      if (!usernameEditedRef.current) setUsername(result.username);
    } catch {
      // A failed suggestion must not leave the field blank for good: the next Name blur retries.
      suggestionFetchedRef.current = false;
    }
  }

  function handleUsernameChange(value: string) {
    usernameEditedRef.current = true;
    setUsername(value.toLowerCase());
  }

  function handleAlternatePress(alternate: string) {
    usernameEditedRef.current = true;
    setUsername(alternate);
  }

  function handleUsernameBlur() {
    if (username === '') setUsernameError(USERNAME_FORMAT_MESSAGE);
  }

  function handleEmailChange(value: string) {
    setEmail(value);
    if (emailError) setEmailError(emailProblem(value));
  }

  function handlePasswordChange(value: string) {
    setPassword(value);
    if (passwordError) setPasswordError(passwordProblem(value));
  }

  function handleNameChange(value: string) {
    setName(value);
    // A shown name error goes away as soon as the name is fixed, not at the next blur.
    if (nameError) setNameError(nameProblem(value));
  }

  function handleNameBlur() {
    setNameError(nameProblem(name));
    maybeSuggestUsername(name);
  }

  function setBirthdayPart(key: keyof BirthdayParts, value: string) {
    const next = { ...birthday, [key]: value.replace(/\D/g, '') };
    setBirthday(next);
    // Speak at once when the date is complete (the number pad has no Done key, so Year never
    // blurs by itself); until then only clear or refresh a message that is already showing.
    const complete = next.month !== '' && next.day !== '' && next.year.length === 4;
    if (complete || birthdayError) {
      const result = checkBirthday(next, new Date(), ageOptions);
      setBirthdayError(result.ok ? undefined : result.error);
    }
  }

  function handleBirthdayBlur(key: keyof BirthdayParts) {
    // Month and Day blur on the way to Year, so they speak only once all three are filled.
    const filled = birthday.month !== '' && birthday.day !== '' && birthday.year !== '';
    if (key !== 'year' && !filled) return;
    const result = checkBirthday(birthday, new Date(), ageOptions);
    setBirthdayError(result.ok ? undefined : result.error);
  }

  function handleChangePhoto() {
    Alert.alert(avatarPreviewUri ? 'Change photo' : 'Add profile photo', undefined, [
      { text: 'Take photo', onPress: () => void pickFromCamera() },
      { text: 'Choose from library', onPress: () => void pickFromLibrary() },
      { text: 'Cancel', style: 'cancel' },
    ]);
  }

  // Both launchers reject on devices without a camera (simulators) or when the picker
  // fails to open; surface that as the avatar error instead of an unhandled rejection
  // from the Alert button handler.
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

  // The presigned PUT signs ContentLength, so it must be the size of the bytes actually
  // uploaded, not the picker's asset.fileSize (which can differ after the picker
  // re-encodes at quality 0.8).
  async function resolveContentLength(uri: string): Promise<number> {
    const response = await fetch(uri);
    const blob = await response.blob();
    return blob.size;
  }

  /** Uploads the picked photo once. A repeat call for the same file returns the first URL. */
  async function ensureAvatarUploaded(): Promise<string | undefined> {
    if (!pendingAvatarUri) return undefined;
    if (uploadedRef.current?.uri === pendingAvatarUri) return uploadedRef.current.url;
    const contentLength = await resolveContentLength(pendingAvatarUri);
    const url = await uploadAvatar(pendingAvatarUri, pendingAvatarMimeType, contentLength);
    uploadedRef.current = { uri: pendingAvatarUri, url };
    return url;
  }

  const trimmedName = name.trim();
  const trimmedBio = bio.trim();

  const birthdayResult = joining ? checkBirthday(birthday, new Date(), ageOptions) : null;
  const birthdayValid = birthdayResult?.ok === true;
  const nameValid = nameProblem(name) === undefined;
  const usernameValid =
    (initialUsername !== '' && username === initialUsername) || USERNAME_PATTERN.test(username);
  const usernameBlocking = usernameStatus === 'checking' || usernameStatus === 'taken';

  const hasChanges =
    trimmedName !== initialName.trim() ||
    username !== initialUsername ||
    trimmedBio !== initialBio.trim() ||
    pendingAvatarUri !== null;

  let canSubmit: boolean;
  if (signup) {
    canSubmit =
      emailProblem(email) === undefined &&
      passwordProblem(password) === undefined &&
      birthdayValid &&
      nameValid &&
      usernameValid &&
      !usernameBlocking &&
      !submitting;
  } else if (mode === 'finish') {
    canSubmit = birthdayValid && nameValid && usernameValid && !usernameBlocking && !submitting;
  } else if (extras) {
    // Photo and bio are both optional, so Continue with neither filled in simply moves on.
    canSubmit = !submitting;
  } else {
    canSubmit = hasChanges && nameValid && usernameValid && !usernameBlocking && !submitting;
  }

  function showRefusal() {
    Alert.alert(REFUSAL_TITLE, REFUSAL_MESSAGE);
  }

  // The server named a field: show this page's own message under it.
  function applyFieldError(field: string | undefined): boolean {
    switch (field) {
      case 'email':
        setEmailError(EMAIL_MESSAGE);
        return true;
      case 'password':
        setPasswordError(utf8Length(password) > PASSWORD_MAX_BYTES ? PASSWORD_LONG_MESSAGE : PASSWORD_SHORT_MESSAGE);
        return true;
      case 'birthday': {
        // The client check already passed, so the server disagreed (a clock edge): a
        // "month, day and year" message would be wrong, so show the generic one.
        const result = checkBirthday(birthday, new Date(), ageOptions);
        if (result.ok) setFormError(GENERIC_MESSAGE);
        else setBirthdayError(result.error);
        return true;
      }
      case 'name':
        setNameError(trimmedName.length === 0 ? NAME_EMPTY_MESSAGE : NAME_LONG_MESSAGE);
        return true;
      case 'username':
        setUsernameError(USERNAME_FORMAT_MESSAGE);
        return true;
      case 'bio':
        setBioError(BIO_LONG_MESSAGE);
        return true;
      case 'avatar_url':
        setAvatarError(new ApiError(400, 'photo_upload_failed').userMessage);
        return true;
      default:
        return false;
    }
  }

  function handleSubmitError(err: unknown) {
    if (!(err instanceof ApiError)) {
      setFormError(GENERIC_MESSAGE);
      return;
    }
    if (err.code === 'under_minimum_age') {
      setBirthdayError(BIRTHDAY_MESSAGES.underAge);
      showRefusal();
      return;
    }
    if (err.code === 'username_taken') {
      // Insert-time conflict: the live check passed but the save still raced into a taken
      // username. Re-render the same taken state with the fresh alternates from the 409
      // body, not a generic error.
      const suggestions = err.suggestions ?? [];
      setUsernameStatus('taken');
      setUsernameAlternates(suggestions);
      setUsernameError(takenMessage(suggestions));
      return;
    }
    if (err.code === 'validation_failed' && applyFieldError(err.field)) return;
    setFormError(err.userMessage);
  }

  async function submitSignup() {
    const check = checkBirthday(birthday, new Date(), ageOptions);
    if (!check.ok) {
      setBirthdayError(check.error);
      return;
    }
    const body: Record<string, string> = {
      // A keyboard's trailing space passes client validation but the server rejects it.
      email: email.trim(),
      password,
      birthday: check.iso,
      name: trimmedName,
      username,
    };

    const result = await api.post<AuthResult>('/auth/signup', body, { auth: false });

    // The landing route opens on the photo-and-bio page for this one sign-in.
    await signIn(result, { extras: true });
  }

  async function submitFinish() {
    const check = checkBirthday(birthday, new Date(), ageOptions);
    if (!check.ok) {
      setBirthdayError(check.error);
      return;
    }

    // The birthday goes ALONE first. An age under 13 deletes the account, so nothing else
    // (no name, no username) may be sent before the server has accepted it.
    if (!birthdaySavedRef.current) {
      try {
        await updateProfile({ birthday: check.iso });
        birthdaySavedRef.current = true;
      } catch (err) {
        if (err instanceof ApiError && err.code === 'under_minimum_age') {
          showRefusal();
          await signOut();
          router.replace('/welcome');
          return;
        }
        // A retry after a lost reply: the server already finished this account and now
        // refuses a birthday from it. If the account now reads complete, reload and the root
        // guard moves on; otherwise fall through to the usual error.
        if (err instanceof ApiError && err.code === 'validation_failed' && err.field === 'birthday') {
          try {
            const fresh = await fetchProfile();
            if (fresh.onboarding_complete) {
              startExtras();
              await reloadUser();
              return;
            }
          } catch {
            // fall through to the error below
          }
        }
        throw err;
      }
    }

    await updateProfile({ name: trimmedName, username });
    // Up before the user reads as complete, so the landing route opens on the photo-and-bio page.
    startExtras();
    // The root guard swaps this screen for the app once the user reads as complete.
    await reloadUser();
  }

  async function submitExtras() {
    let avatarUrl: string | undefined;
    try {
      avatarUrl = await ensureAvatarUploaded();
    } catch (err) {
      // Stay on the page: nothing was saved, so the person can try the photo again or skip it.
      setAvatarError(err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE);
      return;
    }

    const patch: ProfilePatch = {};
    if (trimmedBio.length > 0) patch.bio = trimmedBio;
    if (avatarUrl) patch.avatar_url = avatarUrl;

    if (Object.keys(patch).length > 0) {
      await updateProfile(patch);
      await reloadUser();
    }
    // The landing route swaps this page for the edit page once the flag clears.
    finishExtras();
  }

  async function submitEdit() {
    const patch: ProfilePatch = {};
    if (trimmedName !== initialName.trim()) patch.name = trimmedName;
    if (username !== initialUsername) patch.username = username;
    if (trimmedBio !== initialBio.trim()) patch.bio = trimmedBio;

    const avatarUrl = await ensureAvatarUploaded();
    if (avatarUrl) patch.avatar_url = avatarUrl;

    await updateProfile(patch);
    await reloadUser();
    // Saved: the stored avatar is the new one now, so Save changes goes quiet again. A retry
    // never uploads twice (uploadedRef keeps the URL).
    setPendingAvatarUri(null);
    // This page is also the app's landing page, so there may be nothing to go back to.
    if (router.canGoBack()) router.back();
  }

  async function handleSubmit() {
    if (!canSubmit || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setFormError(undefined);
    // A photo message from the last try would otherwise sit on the page while this one runs.
    setAvatarError(undefined);

    try {
      if (signup) await submitSignup();
      else if (mode === 'finish') await submitFinish();
      else if (extras) await submitExtras();
      else await submitEdit();
    } catch (err) {
      handleSubmitError(err);
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  const avatarPreviewUri = pendingAvatarUri ?? initialAvatarUrl;

  // Skip for now: leave the photo and bio for later; the landing route shows the edit page.
  function handleSkipExtras() {
    finishExtras();
  }

  async function handleConfirmSignOut() {
    setSigningOut(true);
    try {
      await signOut();
      // Edit: the root guard routes back to Welcome once status flips to unauthenticated.
      // Finish: this page lives in the signed-out group too, so it has to leave itself.
      if (mode === 'finish') router.replace('/welcome');
    } finally {
      setSigningOut(false);
      setConfirmingSignOut(false);
    }
  }

  const labelStyle = {
    fontFamily: type.button.fontFamily,
    fontSize: type.label.fontSize,
    lineHeight: type.label.lineHeight,
    letterSpacing: 2,
    textTransform: 'uppercase' as const,
    color: color.muted,
  };

  return (
    <Screen wheels="corner">
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
          {(signup || edit) && router.canGoBack() ? (
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

        {joining ? (
          <View style={{ marginTop: space.xl }}>
            <AppText role="heading">Create your account.</AppText>
          </View>
        ) : (
          <View style={{ marginTop: space.xl }}>
            <AppText role="display">Make it yours.</AppText>
            <View style={{ marginTop: space.sm }}>
              <AppText role="body" tone="muted">
                This is where your daily spins will live.
              </AppText>
            </View>
          </View>
        )}

        {signup ? (
          <View style={{ marginTop: space.lg }}>
            <TextField
              label="Email"
              variant="soft"
              value={email}
              onChangeText={handleEmailChange}
              onBlur={() => setEmailError(emailProblem(email))}
              error={emailError}
              keyboardType="email-address"
              autoCapitalize="none"
              autoCorrect={false}
              autoComplete="email"
              textContentType="emailAddress"
            />

            <View style={{ marginTop: space.md }}>
              <TextField
                label="Password"
                variant="soft"
                value={password}
                onChangeText={handlePasswordChange}
                onBlur={() => setPasswordError(passwordProblem(password))}
                error={passwordError}
                secureTextEntry={!showPassword}
                autoCapitalize="none"
                autoCorrect={false}
                autoComplete="new-password"
                textContentType="newPassword"
              />
              {/* Sits on the label row, where the other fields show their hint. */}
              <Pressable
                onPress={() => setShowPassword((current) => !current)}
                accessibilityRole="button"
                accessibilityLabel={showPassword ? 'Hide password' : 'Show password'}
                hitSlop={12}
                style={{ position: 'absolute', top: 0, right: 0 }}
              >
                <AppText role="label" tone="muted">
                  {showPassword ? 'Hide' : 'Show'}
                </AppText>
              </Pressable>
              {!passwordError ? (
                <View style={{ marginTop: space.xs }}>
                  <AppText role="label" tone="muted">
                    At least 8 characters.
                  </AppText>
                </View>
              ) : null}
            </View>
          </View>
        ) : null}

        {joining ? (
          <View style={{ marginTop: signup ? space.md : space.lg }}>
            <Text style={labelStyle}>Birthday</Text>
            <View style={{ flexDirection: 'row', gap: space.sm }}>
              <View style={{ flex: 1 }}>
                <TextField
                  label="Month"
                  variant="soft"
                  value={birthday.month}
                  onChangeText={(value) => setBirthdayPart('month', value)}
                  onBlur={() => handleBirthdayBlur('month')}
                  placeholder="MM"
                  keyboardType="number-pad"
                  maxLength={2}
                  autoComplete="off"
                />
              </View>
              <View style={{ flex: 1 }}>
                <TextField
                  label="Day"
                  variant="soft"
                  value={birthday.day}
                  onChangeText={(value) => setBirthdayPart('day', value)}
                  onBlur={() => handleBirthdayBlur('day')}
                  placeholder="DD"
                  keyboardType="number-pad"
                  maxLength={2}
                  autoComplete="off"
                />
              </View>
              <View style={{ flex: 1.6 }}>
                <TextField
                  label="Year"
                  variant="soft"
                  value={birthday.year}
                  onChangeText={(value) => setBirthdayPart('year', value)}
                  onBlur={() => handleBirthdayBlur('year')}
                  placeholder="YYYY"
                  keyboardType="number-pad"
                  maxLength={4}
                  autoComplete="off"
                />
              </View>
            </View>
            {birthdayError ? (
              <View accessibilityLiveRegion="polite" style={{ marginTop: space.xs }}>
                <AppText role="label" tone="destructive">
                  {birthdayError}
                </AppText>
              </View>
            ) : null}
            <View style={{ marginTop: space.xs }}>
              <AppText role="label" tone="muted">
                Used only to check your age. Never shown to anyone.
              </AppText>
            </View>
          </View>
        ) : null}

        {!joining ? (
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
        ) : null}

        {!extras ? (
          <>
            <View style={{ marginTop: space.lg }}>
              <TextField
                label="Name"
                variant="soft"
                value={name}
                onChangeText={handleNameChange}
                onBlur={handleNameBlur}
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
                onChangeText={handleUsernameChange}
                onBlur={handleUsernameBlur}
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
          </>
        ) : null}

        {!joining ? (
          <View style={{ marginTop: space.md }}>
            <TextField
              label="Bio"
              variant="soft"
              value={bio}
              onChangeText={(value) => {
                setBio(value);
                if (bioError) setBioError(undefined);
              }}
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
        ) : null}

        {formError ? (
          <View style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton
            label={edit ? 'Save changes' : 'Continue'}
            arrow
            onPress={handleSubmit}
            disabled={!canSubmit}
            loading={submitting}
          />
        </View>

        {signup ? (
          <View style={{ marginTop: space.lg, alignItems: 'center' }}>
            <TextLink onPress={() => router.replace('/login')}>Log in instead</TextLink>
          </View>
        ) : extras ? (
          <View style={{ marginTop: space.lg, alignItems: 'center' }}>
            <TextButton label="Skip for now" tone="muted" disabled={submitting} onPress={handleSkipExtras} />
          </View>
        ) : (
          <View style={{ marginTop: space.lg, alignItems: 'center' }}>
            <TextButton label="Log out" tone="destructive" onPress={() => setConfirmingSignOut(true)} />
          </View>
        )}
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
