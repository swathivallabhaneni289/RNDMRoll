import { useEffect, useRef, useState } from 'react';
import { Alert, Modal, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { AppText, TextLink } from '@/components/ui/AppText';
import { DialAvatar } from '@/components/brand/DialAvatar';
import {
  BirthdayPickerField,
  birthdayPickerAvailable,
  type BirthdayPickerHandle,
} from '@/components/profile/BirthdayPickerField';
import { SettingsGear } from '@/components/settings/SettingsGear';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { FieldError, FieldSuccess, TextField } from '@/components/ui/TextField';
import { FIELD_VIEW_ID } from '@/components/ui/useKeepFocusedInputInView';
import { color, elevation, radius, space, type } from '@/lib/theme/tokens';
import { api, ApiError, isRetryable } from '@/lib/api/client';
import { fileSize, updateProfile, uploadAvatar, type ProfilePatch } from '@/lib/api/profile';
import type { AuthResult, SignupStart, UsernameAvailability, UsernameSuggestion } from '@/lib/api/types';
import { clearPendingSignup, lostSignup, setPendingSignup } from '@/lib/auth/pendingSignup';
import { sayIfRestored } from '@/lib/auth/restored';
import { opensExtras, shouldTryLogin, type SignupTry } from '@/lib/auth/signupRetry';
import { announceProblem } from '@/lib/forms/announce';
import { bringIntoView } from '@/lib/forms/bringIntoView';
import { stillNeeded } from '@/lib/forms/rules';
import { showAccessOff } from '@/lib/media/showAccessOff';
import {
  BIRTHDAY_MESSAGES,
  birthdayBoxToFix,
  birthdayProblem,
  checkBirthday,
  EMPTY_BIRTHDAY,
  type BirthdayParts,
} from '@/lib/profile/birthday';
import { PasswordChecklist } from '@/components/settings/PasswordChecklist';
import { setSettingsNotice } from '@/lib/settings/notice';
import {
  PASSWORD_LONG_MESSAGE,
  PASSWORD_WEAK_MESSAGE,
  passwordProblem as passwordRuleProblem,
  USERNAME_CHARACTER_MESSAGE,
  usernameProblem,
} from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';

/**
 * The one profile page, in three modes (plans 01-19 and 01-20):
 *   signup: signed out. Email, password, birthday, name, username. Continue emails a code, and
 *           the page after it (app/(auth)/signup-code.tsx) makes the account once it is typed in.
 *   extras: signed in, right after signup. Photo and bio, both optional. Continue
 *           or Skip for now. Shown once, in place of the landing page.
 *   edit:   signed in and complete. Photo, name, username, bio. Save changes, Log out.
 * The caller picks the mode once on mount and never changes it (the landing route remounts
 * the form with a new key when its page changes), so the signIn or signOut that ends a
 * submit cannot make this page show anything new while the screen swaps.
 * The look is the "Make it yours." edit page built in 6054474.
 */
export type ProfileFormMode = 'signup' | 'extras' | 'edit';

const BIO_MAX_LENGTH = 160;
const NAME_MAX_LENGTH = 50;
const EMAIL_MAX_LENGTH = 254;
const USERNAME_DEBOUNCE_MS = 400;
// Sign-up keeps only addresses the server keeps (internal/httpapi/auth.go, validEmail and
// asciiEmail): plain printable ASCII, no spaces, one @, dot-separated parts on both sides made of
// letters, digits and ! # $ % & ' * + - / = ? ^ _ ` { | } ~, and a dot after the @. So "Looks
// good." never shows for an address the server refuses. (The server also takes an IP address in
// brackets after the @. This page does not: nobody signs up with one.)
const EMAIL_PART = "[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+";
const EMAIL_PATTERN = new RegExp(`^${EMAIL_PART}(\\.${EMAIL_PART})*@${EMAIL_PART}(\\.${EMAIL_PART})+$`);

const LOOKS_GOOD_MESSAGE = 'Looks good.';
const EMAIL_MESSAGE = 'Enter a valid email address.';
const EMAIL_TAKEN_MESSAGE = 'That email is already in use.';
const NAME_EMPTY_MESSAGE = 'Enter your name.';
const NAME_LONG_MESSAGE = `Name must be ${NAME_MAX_LENGTH} characters or fewer.`;
const BIO_LONG_MESSAGE = `Bio must be ${BIO_MAX_LENGTH} characters or fewer.`;
const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";
const GENERIC_MESSAGE = 'Something went wrong. Please try again.';
const REFUSAL_TITLE = 'RNDMRoll is for ages 13 and up.';
const REFUSAL_MESSAGE = "We can't make an account for you. We didn't keep your birthday.";

/** The Terms of Use and Privacy Policy names in the sign-up line: ink and semibold, the line's own size. */
const LEGAL_LINK = { fontFamily: type.button.fontFamily, color: color.ink } as const;

// Without alternates the "Try one of these:" lead-in would point at nothing.
function takenMessage(alternates: string[]): string {
  return alternates.length > 0 ? USERNAME_TAKEN_MESSAGE : USERNAME_TAKEN_NO_ALTERNATES_MESSAGE;
}

function emailProblem(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 && trimmed.length <= EMAIL_MAX_LENGTH && EMAIL_PATTERN.test(trimmed)
    ? undefined
    : EMAIL_MESSAGE;
}

function passwordProblem(value: string): string | undefined {
  const problem = passwordRuleProblem(value);
  return problem === '' ? undefined : problem;
}

function nameProblem(value: string): string | undefined {
  const trimmed = value.trim();
  if (trimmed.length === 0) return NAME_EMPTY_MESSAGE;
  if (trimmed.length > NAME_MAX_LENGTH) return NAME_LONG_MESSAGE;
  return undefined;
}

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken';

type FieldKey = 'email' | 'password' | 'birthday' | 'name' | 'username';
/** A field that holds the bottom button back, with the line it shows on a tap of the grey button. */
type Problem = { key: FieldKey; label: string; message: string };

/** How long "Saved." stays under the bottom button after a save in the Profile tab. */
const SAVED_NOTE_MS = 3000;

/**
 * `inTab` (edit mode only) is set when this page is the Profile tab of the bottom bar: no Back
 * control (inside tabs "back" would jump to another tab), the person stays on the page after a
 * save and sees "Saved." for 3 seconds, and the bottom inset is left to the bar.
 *
 * `pushed` (edit mode only) is set when the Profile page's Edit button opened this page on top of
 * it (04-UI-SPEC 4.6): the Back control shows, the bottom bar shows (so the bottom inset is left to
 * it), the one bottom button always reads "Save changes" and is grey until something has changed
 * ("Continue" is not on this page), the photo link reads "Edit your picture", a free username is
 * confirmed with the green "Looks good.", and a successful save goes back to the Profile page with
 * "Saved." put into the notice store for it to show. Everything else is as it was.
 */
export function ProfileForm({
  mode,
  inTab = false,
  pushed = false,
}: {
  mode: ProfileFormMode;
  inTab?: boolean;
  pushed?: boolean;
}) {
  const router = useRouter();
  const { user, signIn, signOut, reloadUser, finishExtras } = useSession();
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const [savedNote, setSavedNote] = useState(false);
  const savedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const signup = mode === 'signup';
  const edit = mode === 'edit';
  const extras = mode === 'extras';
  const pushedEdit = edit && pushed;

  // What is stored right now. A signup has nothing stored; after its signIn (on the code page, or
  // here after a lost verify) the session user appears, but this page is already on its way out,
  // so it keeps reading blanks.
  const initialName = signup ? '' : (user?.name ?? '');
  const initialUsername = signup ? '' : (user?.username ?? '');
  const initialBio = signup ? '' : (user?.bio ?? '');
  const initialAvatarUrl = signup ? null : (user?.avatar_url ?? null);

  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>(undefined);
  // The address the server said is taken; the message shows only while the box still holds it.
  const [takenEmail, setTakenEmail] = useState<string | null>(null);

  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | undefined>(undefined);
  // The Password box shows only the too-long line, as the checklist under it says the rest. After a
  // tap on the grey Continue it shows any line, so the box names what is still missing.
  const [passwordAsked, setPasswordAsked] = useState(false);

  const [birthday, setBirthday] = useState<BirthdayParts>(EMPTY_BIRTHDAY);
  const [birthdayError, setBirthdayError] = useState<string | undefined>(undefined);

  const [name, setName] = useState(initialName);
  const [nameError, setNameError] = useState<string | undefined>(undefined);
  const [nameTouched, setNameTouched] = useState(false);

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

  // A message from the last try goes away as soon as anything on the page is changed.
  useEffect(() => {
    setFormError(undefined);
  }, [email, password, birthday, name, username, bio]);

  const submittingRef = useRef(false);
  const usernameRef = useRef(username);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The suggestion is fetched once (Name blur, or mount when a name is already known). It
  // never overwrites a username the person typed, and a failed fetch re-arms it for the
  // next Name blur.
  const suggestionFetchedRef = useRef(false);
  const usernameEditedRef = useRef(false);
  // A photo that already uploaded is remembered by its local file, with the key it got, so a
  // retry after a failed later step never uploads it twice.
  const uploadedRef = useRef<{ uri: string; key: string } | null>(null);
  // A tap on the grey button scrolls to the first field with a problem (the view around its label,
  // box and line) and puts the cursor in its box. Each view carries FIELD_VIEW_ID, so the keyboard
  // that comes up then leaves all of it in view, the username's chips and the password checklist too.
  const scrollRef = useRef<ScrollView>(null);
  const fieldRefs = {
    email: useRef<View>(null),
    password: useRef<View>(null),
    birthday: useRef<View>(null),
    name: useRef<View>(null),
    username: useRef<View>(null),
  };
  const boxRefs = {
    email: useRef<TextInput>(null),
    password: useRef<TextInput>(null),
    name: useRef<TextInput>(null),
    username: useRef<TextInput>(null),
  };
  const birthdayRef = useRef<BirthdayPickerHandle>(null);
  const birthdayBoxRefs = {
    month: useRef<TextInput>(null),
    day: useRef<TextInput>(null),
    year: useRef<TextInput>(null),
  };

  useEffect(() => {
    usernameRef.current = username;
  }, [username]);

  useEffect(() => {
    if (signup && name.trim().length > 0 && username === '') {
      maybeSuggestUsername(name);
    }
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      if (savedTimerRef.current) clearTimeout(savedTimerRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Live, 400ms-debounced availability check: the one field validated live rather than
  // on blur, per UI-SPEC's form validation timing rule.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (initialUsername !== '' && username === initialUsername) {
      // Back to the stored value: nothing to check, and it is never "taken" against itself.
      setUsernameStatus('idle');
      setUsernameError(undefined);
      setUsernameAlternates([]);
      return;
    }

    if (usernameProblem(username, initialUsername) !== '') {
      // Not a username yet, so nothing to check. A character it can't have is said at once. An
      // empty or short one is said only when the box is left or the grey button is tapped, so the
      // first letters typed show nothing under the box.
      setUsernameStatus('idle');
      setUsernameError(usernameProblem(username, initialUsername, 'typing') || undefined);
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
    if (!signup || suggestionFetchedRef.current || usernameEditedRef.current || trimmed.length === 0) return;
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

  // Leaving the box says what more typing could still fix (empty, too short). A taken name, or one
  // the check couldn't reach, keeps its own line.
  function handleUsernameBlur() {
    const problem = usernameProblem(username, initialUsername);
    if (problem !== '') setUsernameError(problem);
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
    setNameTouched(true);
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
      const result = checkBirthday(next, new Date());
      setBirthdayError(result.ok ? undefined : result.error);
    }
  }

  function handleBirthdayBlur(key: keyof BirthdayParts) {
    // Month and Day blur on the way to Year, so they speak only once all three are filled.
    const filled = birthday.month !== '' && birthday.day !== '' && birthday.year !== '';
    if (key !== 'year' && !filled) return;
    const result = checkBirthday(birthday, new Date());
    setBirthdayError(result.ok ? undefined : result.error);
  }

  // The date scroller gives a real date every time, so the three parts are filled from it and
  // checked by the same rules as typed boxes (a message for a date that is too young).
  function handleBirthdayPicked(date: Date) {
    const next: BirthdayParts = {
      month: String(date.getMonth() + 1),
      day: String(date.getDate()),
      year: String(date.getFullYear()),
    };
    setBirthday(next);
    const result = checkBirthday(next, new Date());
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
        showAccessOff('camera');
        return;
      }
      const result = await ImagePicker.launchCameraAsync({
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        quality: 0.8,
        // Hands back a standard JPEG even for a HEIC photo, so any picture works everywhere.
        preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
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
        showAccessOff('library');
        return;
      }
      const result = await ImagePicker.launchImageLibraryAsync({
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        quality: 0.8,
        // Hands back a standard JPEG even for a HEIC photo, so any picture works everywhere.
        preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
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
    return fileSize(uri);
  }

  /** Uploads the picked photo once. A repeat call for the same file returns the first key. */
  async function ensureAvatarUploaded(): Promise<string | undefined> {
    if (!pendingAvatarUri) return undefined;
    if (uploadedRef.current?.uri === pendingAvatarUri) return uploadedRef.current.key;
    const contentLength = await resolveContentLength(pendingAvatarUri);
    const key = await uploadAvatar(pendingAvatarUri, pendingAvatarMimeType, contentLength);
    uploadedRef.current = { uri: pendingAvatarUri, key };
    return key;
  }

  const trimmedName = name.trim();
  const trimmedBio = bio.trim();

  const birthdayResult = signup ? checkBirthday(birthday, new Date()) : null;
  const birthdayValid = birthdayResult?.ok === true;
  const nameValid = nameProblem(name) === undefined;
  // The green notes. Email and password speak as you type; the name after you leave the box (or
  // at once when it came filled in); the birthday once a date is chosen.
  const emailTakenNow = signup && takenEmail !== null && email.trim().toLowerCase() === takenEmail;
  const emailOk = signup && email.trim() !== '' && emailProblem(email) === undefined && !emailTakenNow;
  const passwordOk = signup && password !== '' && passwordProblem(password) === undefined;
  const nameOk = signup && nameValid && (nameTouched || initialName.trim() !== '');
  const birthdayOk = signup && checkBirthday(birthday, new Date()).ok;
  const usernameValid = usernameProblem(username, initialUsername) === '';
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
      !emailTakenNow &&
      passwordProblem(password) === undefined &&
      birthdayValid &&
      nameValid &&
      usernameValid &&
      !usernameBlocking &&
      !submitting;
  } else if (extras) {
    // Photo and bio are both optional, so Continue with neither filled in simply moves on.
    canSubmit = !submitting;
  } else {
    canSubmit = hasChanges && nameValid && usernameValid && !usernameBlocking && !submitting;
  }

  // What holds the bottom button back, in page order, with each field's line. A username still being
  // checked is not a problem (Checking... shows under it), and neither is an Edit page with nothing
  // changed, so a tap on a button grey only for those does nothing.
  const problems: Problem[] = [];
  function addProblem(key: FieldKey, label: string, message: string | undefined) {
    if (message) problems.push({ key, label, message });
  }
  if (signup) {
    addProblem('email', 'Email', emailTakenNow ? EMAIL_TAKEN_MESSAGE : emailProblem(email));
    addProblem('password', 'Password', passwordProblem(password));
    addProblem('birthday', 'Birthday', birthdayProblem(birthday, birthdayPickerAvailable));
  }
  if (signup || (edit && hasChanges)) {
    addProblem('name', 'Name', nameProblem(name));
    addProblem(
      'username',
      'Username',
      usernameStatus === 'taken' ? takenMessage(usernameAlternates) : usernameProblem(username, initialUsername)
    );
  }

  // A tap on the grey button: every field shows its line, touched or not. Then VoiceOver says the
  // first one, the page scrolls it into view and the cursor goes into its box. A birthday never
  // picked opens the date sheet instead; under 13 opens nothing.
  function showProblems() {
    if (signup) {
      setEmailError(emailProblem(email));
      setPasswordError(passwordProblem(password));
      setPasswordAsked(true);
      setBirthdayError(birthdayProblem(birthday, birthdayPickerAvailable) || undefined);
    }
    setNameTouched(true);
    setNameError(nameProblem(name));
    const usernameLine = usernameProblem(username, initialUsername);
    if (usernameLine !== '') setUsernameError(usernameLine);

    const first = problems[0];
    if (!first) return;
    announceProblem(first.message, first.label);
    bringIntoView(scrollRef.current, fieldRefs[first.key].current);
    const key = first.key;
    if (key !== 'birthday') {
      boxRefs[key].current?.focus();
    } else if (!birthdayPickerAvailable) {
      const box = birthdayBoxToFix(birthday, first.message);
      if (box) birthdayBoxRefs[box].current?.focus();
    } else if (first.message === BIRTHDAY_MESSAGES.notPicked) {
      birthdayRef.current?.open();
    }
  }

  // A server answer about the email: its box is at the top of the form, likely out of view from
  // Continue, so the page brings it back and VoiceOver says the line.
  function showEmailLine(message: string) {
    announceProblem(message, 'Email');
    bringIntoView(scrollRef.current, fieldRefs.email.current);
  }

  function showRefusal() {
    Alert.alert(REFUSAL_TITLE, REFUSAL_MESSAGE);
  }

  // The server named a field: show this page's own message under it.
  function applyFieldError(field: string | undefined): boolean {
    switch (field) {
      case 'email':
        setEmailError(EMAIL_MESSAGE);
        showEmailLine(EMAIL_MESSAGE);
        return true;
      case 'password':
        setPasswordError(passwordRuleProblem(password) || PASSWORD_WEAK_MESSAGE);
        return true;
      case 'birthday': {
        // The client check already passed, so the server disagreed (a clock edge): a
        // "month, day and year" message would be wrong, so show the generic one.
        const result = checkBirthday(birthday, new Date());
        if (result.ok) setFormError(GENERIC_MESSAGE);
        else setBirthdayError(result.error);
        return true;
      }
      case 'name':
        setNameError(trimmedName.length === 0 ? NAME_EMPTY_MESSAGE : NAME_LONG_MESSAGE);
        return true;
      case 'username':
        // The phone and the server share the rule, so this is rare: the phone's own line, or the
        // characters line when the phone sees nothing wrong.
        setUsernameError(usernameProblem(username, initialUsername) || USERNAME_CHARACTER_MESSAGE);
        return true;
      case 'bio':
        setBioError(BIO_LONG_MESSAGE);
        return true;
      case 'avatar_key':
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
    if (err.code === 'email_taken') {
      // An email problem: it shows under the Email box until the email is changed, and Continue
      // waits for a different one. It does not tell the person to log in.
      setTakenEmail(email.trim().toLowerCase());
      showEmailLine(EMAIL_TAKEN_MESSAGE);
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
    const check = checkBirthday(birthday, new Date());
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
    const thisTry: SignupTry = { email: body.email, password, username };

    let started: SignupStart;
    try {
      // No account yet (developer, 2026-11-24): the server keeps the sign-up and emails a 6-digit
      // code, and the code page makes the account once that code is typed in.
      started = await api.post<SignupStart>('/auth/signup', body, { auth: false });
    } catch (err) {
      // A start makes no account, so one whose answer was lost needs nothing: the next Continue
      // starts again. But a verify whose answer was lost may have made one, before the person came
      // back here, and then this email_taken is most likely their own new account
      // (lib/auth/signupRetry.ts).
      if (!(err instanceof ApiError)) throw err;
      const lost = lostSignup();
      if (lost === null || !shouldTryLogin(lost, thisTry.email, password, err)) throw err;
      // If the name or username changed since that sign-up, they land in the account with its
      // details, which they can edit on Profile.
      const result = await logInToLostSignup(thisTry, err);
      await signIn(result, { extras: opensExtras(lost, result) });
      clearPendingSignup();
      // Only a log in can bring an account back from deletion: said once, as on the Log in page.
      sayIfRestored(result);
      return;
    }

    // In memory only (lib/auth/pendingSignup.ts): the code page reads it, and its Back comes back
    // to this page as it was typed.
    setPendingSignup(thisTry, started.signup_token);
    router.push('/signup-code');
  }

  // Logs in with a try's email and password. A refusal (the email is someone else's, or the
  // password is not that account's) gives back the email_taken answer, which shows as it always
  // has. A failure that could pass (no answer, too many tries, a server failure) shows its own
  // message, and Continue stays on so the next tap tries again.
  async function logInToLostSignup(attempt: SignupTry, taken: ApiError): Promise<AuthResult> {
    try {
      return await api.post<AuthResult>(
        '/auth/login',
        { login: attempt.email, password: attempt.password },
        { auth: false }
      );
    } catch (err) {
      throw isRetryable(err) ? err : taken;
    }
  }

  async function submitExtras() {
    let avatarKey: string | undefined;
    try {
      avatarKey = await ensureAvatarUploaded();
    } catch (err) {
      // Stay on the page: nothing was saved, so the person can try the photo again or skip it.
      setAvatarError(err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE);
      return;
    }

    const patch: ProfilePatch = {};
    if (trimmedBio.length > 0) patch.bio = trimmedBio;
    if (avatarKey) patch.avatar_key = avatarKey;

    if (Object.keys(patch).length > 0) {
      await updateProfile(patch);
      await reloadUser();
    }
    // Today first, the flag second, in this order: clearing the flag first would remount this
    // form as the edit page for a frame before the tab switch (plan 02-02 decision 11).
    // Then Find your friends, once (developer, 2026-10-19), which goes on to the Spin page.
    router.navigate('/(app)/today/find-friends');
    finishExtras();
  }

  async function submitEdit() {
    const patch: ProfilePatch = {};
    if (trimmedName !== initialName.trim()) patch.name = trimmedName;
    if (username !== initialUsername) patch.username = username;
    if (trimmedBio !== initialBio.trim()) patch.bio = trimmedBio;

    const avatarKey = await ensureAvatarUploaded();
    if (avatarKey) patch.avatar_key = avatarKey;

    await updateProfile(patch);
    await reloadUser();
    // Saved: the stored avatar is the new one now, so Save changes goes quiet again. A retry
    // never uploads twice (uploadedRef keeps the key).
    setPendingAvatarUri(null);
    if (pushedEdit) {
      // Back to the Profile page, which shows "Saved." for 3 seconds from the notice store.
      setSettingsNotice('Saved.');
      if (router.canGoBack()) router.back();
      else router.replace('/(app)/profile');
    } else if (inTab) {
      // Inside the tabs "back" would jump to another tab, so the person stays here and sees
      // "Saved." under the button for a few seconds.
      setSavedNote(true);
      if (savedTimerRef.current) clearTimeout(savedTimerRef.current);
      savedTimerRef.current = setTimeout(() => setSavedNote(false), SAVED_NOTE_MS);
    } else if (router.canGoBack()) {
      router.back();
    }
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

  // Skip for now: leave the photo and bio for later and go to Today (then clear the flag, in
  // that order, so the edit page never flashes).
  function handleSkipExtras() {
    // Then Find your friends, once (developer, 2026-10-19), which goes on to the Spin page.
    router.navigate('/(app)/today/find-friends');
    finishExtras();
  }

  async function handleConfirmSignOut() {
    setSigningOut(true);
    try {
      await signOut();
      // Edit: the root guard routes back to Welcome once status flips to unauthenticated.
    } catch {
      // The keychain could not be cleared: still signed in, and Log out can be tapped again.
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
    <Screen wheels="corner" edges={edit && (inTab || pushed) ? ['top', 'left', 'right'] : undefined} scrollRef={scrollRef}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
          {(signup || (edit && !inTab)) && router.canGoBack() ? (
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
            {edit && inTab ? (
              // The 44dp gear overhangs the label row by the same amount above and below, so the
              // header row keeps its height and nothing on the page moves.
              <View style={{ marginLeft: space.sm, marginVertical: -15 }}>
                <SettingsGear />
              </View>
            ) : null}
          </View>
        </View>

        {signup ? (
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
            <View ref={fieldRefs.email} nativeID={FIELD_VIEW_ID}>
              <TextField
                ref={boxRefs.email}
                label="Email"
                variant="soft"
                value={email}
                onChangeText={handleEmailChange}
                onBlur={() => setEmailError(emailProblem(email))}
                error={emailError ?? (emailTakenNow ? EMAIL_TAKEN_MESSAGE : undefined)}
                success={emailOk ? LOOKS_GOOD_MESSAGE : undefined}
                keyboardType="email-address"
                autoCapitalize="none"
                autoCorrect={false}
                autoComplete="email"
                textContentType="emailAddress"
              />
            </View>

            <View ref={fieldRefs.password} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.md }}>
              <TextField
                ref={boxRefs.password}
                label="Password"
                variant="soft"
                value={password}
                onChangeText={handlePasswordChange}
                onBlur={() => setPasswordError(passwordProblem(password))}
                error={passwordAsked || passwordError === PASSWORD_LONG_MESSAGE ? passwordError : undefined}
                success={passwordOk ? LOOKS_GOOD_MESSAGE : undefined}
                revealable
                autoCapitalize="none"
                autoCorrect={false}
                autoComplete="new-password"
                textContentType="newPassword"
              />
              <PasswordChecklist value={password} />
            </View>
          </View>
        ) : null}

        {signup ? (
          <View ref={fieldRefs.birthday} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.md }}>
            {birthdayPickerAvailable ? (
              <BirthdayPickerField
                ref={birthdayRef}
                value={birthday}
                onPick={handleBirthdayPicked}
                error={birthdayError}
                success={birthdayOk ? LOOKS_GOOD_MESSAGE : undefined}
              />
            ) : (
              <>
                <Text style={labelStyle}>Birthday</Text>
                <View style={{ flexDirection: 'row', gap: space.sm }}>
                  <View style={{ flex: 1 }}>
                    <TextField
                      ref={birthdayBoxRefs.month}
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
                      ref={birthdayBoxRefs.day}
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
                      ref={birthdayBoxRefs.year}
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
                  <FieldError text={birthdayError} label="Birthday" />
                ) : birthdayOk ? (
                  <FieldSuccess text={LOOKS_GOOD_MESSAGE} label="Birthday" />
                ) : null}
              </>
            )}
            {/* The hint under the box; a problem takes its place. */}
            {birthdayError ? null : (
              <View style={{ marginTop: space.xs }}>
                <AppText role="label" tone="muted">
                  Used only to check your age. Never shown to anyone.
                </AppText>
              </View>
            )}
          </View>
        ) : null}

        {!signup ? (
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
              <TextButton
                label={pushedEdit ? 'Edit your picture' : avatarPreviewUri ? 'Change photo' : 'Add photo'}
                tone={pushedEdit ? 'default' : 'muted'}
                onPress={handleChangePhoto}
              />
            </View>
            {avatarError ? <FieldError text={avatarError} label="Photo" /> : null}
          </View>
        ) : null}

        {!extras ? (
          <>
            <View ref={fieldRefs.name} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.lg }}>
              <TextField
                ref={boxRefs.name}
                label="Name"
                variant="soft"
                value={name}
                onChangeText={handleNameChange}
                onBlur={handleNameBlur}
                error={nameError}
                success={nameOk ? LOOKS_GOOD_MESSAGE : undefined}
                autoCapitalize="words"
                autoComplete="name"
                textContentType="name"
              />
            </View>

            <View ref={fieldRefs.username} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.md }}>
              <TextField
                ref={boxRefs.username}
                label="Username"
                variant="soft"
                prefix="@"
                value={username}
                onChangeText={handleUsernameChange}
                onBlur={handleUsernameBlur}
                // No length cap: a paste over 20 shows "At most 20 characters." at once and is
                // never checked, where a cap would cut it short without a word.
                // The pushed Edit page confirms a free name with the green "Looks good." (the
                // field's own note, as the old Username page did); everywhere else it says "Available".
                status={pushedEdit && usernameStatus === 'available' ? 'idle' : usernameStatus}
                success={pushedEdit && usernameStatus === 'available' ? LOOKS_GOOD_MESSAGE : undefined}
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

        {!signup ? (
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
              numberOfLines={2}
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
            <AppText role="body" tone="default">
              {formError}
            </AppText>
          </View>
        ) : null}

        {signup ? (
          // Above Continue, so it is read before the tap that starts the sign-up. The two names open
          // their pages: ink in the line's own size, never underlined (developer, 2026-10-18).
          // VoiceOver cannot tap a word inside a line, so they are also the line's two actions.
          <View style={{ marginTop: space.lg }}>
            <AppText
              role="label"
              tone="muted"
              accessibilityActions={[
                { name: 'terms', label: 'Open the Terms of Use' },
                { name: 'privacy', label: 'Open the Privacy Policy' },
              ]}
              onAccessibilityAction={(event) => {
                if (event.nativeEvent.actionName === 'terms') router.push('/terms');
                if (event.nativeEvent.actionName === 'privacy') router.push('/privacy');
              }}
            >
              {'By tapping Continue, you agree to the '}
              <Text style={LEGAL_LINK} onPress={() => router.push('/terms')}>
                Terms of Use
              </Text>
              {" and confirm you've read the "}
              <Text style={LEGAL_LINK} onPress={() => router.push('/privacy')}>
                Privacy Policy
              </Text>
              {'.'}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: signup ? space.md : space.lg }}>
          {edit && !hasChanges && !pushedEdit ? (
            // Nothing to save, so the one button moves on to Today (no greyed button at rest). The
            // pushed Edit page has no such button: "Save changes" stays, grey, until something changes.
            <PrimaryButton label="Continue" arrow onPress={() => router.navigate('/(app)/today')} />
          ) : (
            <PrimaryButton
              label={edit ? 'Save changes' : 'Continue'}
              arrow
              onPress={handleSubmit}
              disabled={!canSubmit}
              loading={submitting}
              onBlockedPress={problems.length > 0 ? showProblems : undefined}
              blockedHint={problems.length > 0 ? stillNeeded(problems.map((problem) => problem.label.toLowerCase())) : undefined}
            />
          )}
        </View>
        {savedNote && !hasChanges ? (
          // Directly under the button, gone at once when anything is edited again.
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.xs, alignItems: 'center' }}>
            <AppText role="label" tone="muted">
              Saved.
            </AppText>
          </View>
        ) : null}

        {signup ? (
          <View style={{ marginTop: space.lg, alignItems: 'center' }}>
            <TextLink onPress={() => router.replace('/login')}>Log in instead</TextLink>
          </View>
        ) : extras ? (
          // Outlined under the black Continue, as Log in sits under Sign up on the first screen, and
          // as on Find your friends after it (developer, 2026-11-25).
          <View style={{ marginTop: space.md }}>
            <PrimaryButton variant="outline" label="Skip for now" disabled={submitting} onPress={handleSkipExtras} />
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
