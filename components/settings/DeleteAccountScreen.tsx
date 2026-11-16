import { useRef, useState } from 'react';
import { Alert, KeyboardAvoidingView, Platform, Pressable, ScrollView, TextInput, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { BackRow } from '@/components/settings/BackRow';
import { deleteAccount } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { announceProblem } from '@/lib/forms/announce';
import { bringIntoView } from '@/lib/forms/bringIntoView';
import { stillNeeded } from '@/lib/forms/rules';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';
import { color, space, type } from '@/lib/theme/tokens';

/** The icon beside each point is as tall as the point's bold line, so the two sit level. */
const ICON_SIZE = type.button.lineHeight;

type Point = { icon: React.ComponentProps<typeof Ionicons>['name']; title: string; detail: string };

/** Under the title, before the points. */
const LEAD = "Here's what happens if you delete\u00A0it.";

/**
 * What deleting does, as three points: right away, after 30 days, and how to keep it. Nothing is
 * removed during the 30 days (migration 0018), so the removal is told on the 30 days point, never
 * as happening now. A few spaces are no-break spaces (U+00A0), so no line ends on one stranded
 * word at any text size.
 */
const POINTS: readonly Point[] = [
  {
    icon: 'eye-off-outline',
    title: 'Hidden right away',
    detail: "No one can see your account, and you're logged\u00A0out.",
  },
  {
    icon: 'trash-outline',
    title: 'Deleted for good after 30\u00A0days',
    detail: 'Then your profile, posts, photos, comments and follows are\u00A0removed.',
  },
  {
    icon: 'log-in-outline',
    title: 'Log back in to keep it',
    detail: 'You have 30\u00A0days. Nothing\u00A0is\u00A0lost.',
  },
];

/** The heading over the password. */
const STEP_HEADING = "Confirm it's you";

/** The one ask before anything is sent, in the phone's own alert. */
const CONFIRM_TITLE = 'Delete your account?';
const CONFIRM_MESSAGE = 'It will be deleted for good after 30 days. Log back in before then to keep it.';

const DONE_TITLE = 'Your account will be deleted';

/** The day comes written by the server and is printed as sent; without it, the 30 days still hold. */
function doneMessage(label: string): string {
  if (label === '') return CONFIRM_MESSAGE;
  return `It will be deleted for good on ${label}. Log back in before then to keep it.`;
}

const WRONG_PASSWORD_MESSAGE = "That password isn't right.";
/** The Log in page's line: a tap on the grey Delete account (the box is empty), or the server's rare "required". */
const PASSWORD_REQUIRED_MESSAGE = 'Enter your password.';
const TOO_MANY_MESSAGE = 'Too many tries. Wait a minute and try again.';

/**
 * Settings > Delete account (developer, 2026-10-25: "add the delete the account thing like how
 * instagram has"): what deleting does, the person's password, and one ask in the phone's own alert
 * before anything is sent. What deleting does is a short line and three points between hairlines,
 * each an outline icon, a bold line and a muted line, so it reads at a glance (developer,
 * 2026-10-27: "display the warning more neatly"); the password sits under its own heading,
 * "Confirm it's you".
 *
 * Delete is `POST /me/delete`. The server hides the account from everyone at once, ends every
 * session (this phone's too) and deletes it for good after 30 days; logging back in before then
 * keeps it. On success the phone's alert gives the day the server wrote, and its OK signs out
 * through the session store, which returns to Welcome. The password is never put in a route, a log
 * or a notice, and a failed try never clears the box.
 */
export function DeleteAccountScreen() {
  const { signOut } = useSession();
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [passwordError, setPasswordError] = useState('');
  const [formError, setFormError] = useState('');
  const [deleting, setDeleting] = useState(false);
  // From the tap until the ask is answered, and from Delete until the answer: a second tap shows no
  // second alert and sends nothing.
  const busyRef = useRef(false);
  // The password box sits low on the page, where the keyboard comes up. While it has focus the page
  // scrolls to its end so the box is not hidden, the way the entry form keeps its comment in view.
  const scrollRef = useRef<ScrollView>(null);
  const passwordFocusedRef = useRef(false);
  const passwordFieldRef = useRef<View>(null);
  const passwordBoxRef = useRef<TextInput>(null);
  const insets = useSafeAreaInsets();

  const canDelete = password !== '' && !deleting;

  // A tap on the grey Delete account: the box asks for the password, VoiceOver says it, the page
  // scrolls the box into view and the cursor goes into it.
  function showProblem() {
    setPasswordError(PASSWORD_REQUIRED_MESSAGE);
    announceProblem(PASSWORD_REQUIRED_MESSAGE, 'Password');
    bringIntoView(scrollRef.current, passwordFieldRef.current);
    passwordBoxRef.current?.focus();
  }

  function handleChange(value: string) {
    setPassword(value);
    setPasswordError('');
    setFormError('');
  }

  function handleError(err: unknown) {
    if (!(err instanceof ApiError)) {
      setFormError(GENERIC_MESSAGE);
      return;
    }
    if (err.code === 'validation_failed' && err.field === 'password') {
      setPasswordError(err.reason === 'required' ? PASSWORD_REQUIRED_MESSAGE : WRONG_PASSWORD_MESSAGE);
      return;
    }
    // A 429 without the usual body has no code, so the status counts too.
    if (err.status === 429 || err.code === 'rate_limited') {
      setFormError(TOO_MANY_MESSAGE);
      return;
    }
    // The account is already on its way out (a try whose answer was lost went through, or an admin
    // disabled it): the client is signing out by itself, and "disabled" would be the wrong word.
    if (err.code === 'account_disabled') return;
    // No connection, anything else.
    setFormError(err.userMessage);
  }

  function signOutAfterDelete() {
    // The server has already ended every session, so nothing here shows an error. A sign out
    // already running (the client's own, for a request answered account_disabled meanwhile) is
    // shared. Only a keychain that would not clear fails it: still signed in on this phone, so the
    // page is usable again, and its next request signs out once more.
    signOut().catch(() => {
      busyRef.current = false;
      setDeleting(false);
    });
  }

  async function remove() {
    setDeleting(true);
    setPasswordError('');
    setFormError('');
    try {
      const result = await deleteAccount(password);
      setPassword('');
      // The button stays busy until the sign out takes the page away.
      Alert.alert(
        DONE_TITLE,
        doneMessage(result.delete_on_label),
        [{ text: 'OK', onPress: signOutAfterDelete }],
        { cancelable: false },
      );
    } catch (err) {
      handleError(err);
      busyRef.current = false;
      setDeleting(false);
    }
  }

  function confirmDelete() {
    if (!canDelete || busyRef.current) return;
    busyRef.current = true;
    const release = () => {
      busyRef.current = false;
    };
    Alert.alert(
      CONFIRM_TITLE,
      CONFIRM_MESSAGE,
      [
        { text: 'Cancel', style: 'cancel', onPress: release },
        { text: 'Delete', style: 'destructive', onPress: () => void remove() },
      ],
      { cancelable: true, onDismiss: release },
    );
  }

  return (
    <Screen edges={['top', 'left', 'right']} scroll={false}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        // The keyboard's top is measured from the top of the phone, but this view's place is measured
        // from just under the top safe area. Passing that inset lines the two up. Without it, the view
        // pads too little and the keyboard covers part of the Delete account button.
        keyboardVerticalOffset={insets.top}
        style={{ flex: 1 }}
      >
        <ScrollView
          ref={scrollRef}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={{ flexGrow: 1 }}
          // The keyboard shrinks this area; keep the password in view while it has focus.
          onLayout={() => {
            if (passwordFocusedRef.current) scrollRef.current?.scrollToEnd({ animated: true });
          }}
        >
          <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
            <BackRow />

            <View style={{ marginTop: space.md }}>
              <AppText role="title" accessibilityRole="header">
                Delete account
              </AppText>
            </View>

            <View style={{ marginTop: space.sm }}>
              <AppText role="body" tone="muted">
                {LEAD}
              </AppText>
            </View>

            {/* The hairline over the first point is the one Settings puts above its rows. No fill,
                radius or shadow: the points sit on the page, and none of them is tappable. */}
            <View style={{ marginTop: space.lg, borderTopWidth: 1, borderTopColor: color.divider }}>
              {POINTS.map((point) => (
                <View
                  key={point.title}
                  accessible
                  accessibilityLabel={`${point.title}. ${point.detail}`}
                  style={{
                    flexDirection: 'row',
                    alignItems: 'flex-start',
                    paddingVertical: space.md,
                    borderBottomWidth: 1,
                    borderBottomColor: color.divider,
                  }}
                >
                  {/* Drawn, not read: VoiceOver reads the row as one line. */}
                  <View
                    accessibilityElementsHidden
                    importantForAccessibility="no-hide-descendants"
                    style={{ width: ICON_SIZE, marginRight: space.md }}
                  >
                    <Ionicons name={point.icon} size={ICON_SIZE} color={color.ink} />
                  </View>
                  <View style={{ flex: 1 }}>
                    <AppText role="button">{point.title}</AppText>
                    <View style={{ marginTop: space.xs }}>
                      <AppText role="body" tone="muted">
                        {point.detail}
                      </AppText>
                    </View>
                  </View>
                </View>
              ))}
            </View>

            {/* Its own block, outside the password's: Show sits at the top of that block, on the label row. */}
            <View style={{ marginTop: space.xl }}>
              <AppText role="heading" accessibilityRole="header">
                {STEP_HEADING}
              </AppText>
            </View>

            <View ref={passwordFieldRef} style={{ marginTop: space.md }}>
              <TextField
                ref={passwordBoxRef}
                label="Password"
                variant="soft"
                value={password}
                onChangeText={handleChange}
                error={passwordError}
                editable={!deleting}
                secureTextEntry={!showPassword}
                autoCapitalize="none"
                autoCorrect={false}
                autoComplete="current-password"
                textContentType="password"
                onFocus={() => {
                  passwordFocusedRef.current = true;
                  scrollRef.current?.scrollToEnd({ animated: true });
                }}
                onBlur={() => {
                  passwordFocusedRef.current = false;
                }}
              />
              {/* Sits on the label row, where the other fields show their hint. */}
              <Pressable
                onPress={() => setShowPassword((value) => !value)}
                accessibilityRole="button"
                accessibilityLabel={showPassword ? 'Hide password' : 'Show password'}
                hitSlop={12}
                style={{ position: 'absolute', top: 0, right: 0 }}
              >
                <AppText role="label" tone="muted">
                  {showPassword ? 'Hide' : 'Show'}
                </AppText>
              </Pressable>
            </View>

            {formError !== '' ? (
              <View accessibilityLiveRegion="polite" style={{ marginTop: space.md }}>
                <AppText role="body" tone="default">
                  {formError}
                </AppText>
              </View>
            ) : null}

            <View style={{ marginTop: space.lg }}>
              <PrimaryButton
                label="Delete account"
                disabled={!canDelete}
                loading={deleting}
                onPress={confirmDelete}
                onBlockedPress={showProblem}
                blockedHint={stillNeeded(['password'])}
              />
            </View>
          </View>
        </ScrollView>
      </KeyboardAvoidingView>
    </Screen>
  );
}
