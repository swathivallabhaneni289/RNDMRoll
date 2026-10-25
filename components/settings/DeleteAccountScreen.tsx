import { useRef, useState } from 'react';
import { Alert, Pressable, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { BackRow } from '@/components/settings/BackRow';
import { deleteAccount } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';
import { space } from '@/lib/theme/tokens';

/** What deleting does, in plain words, one line each. */
const LINES = [
  'Deleting your account removes your profile, posts, photos, comments and follows.',
  'Your account is hidden from everyone right away and deleted for good after 30 days.',
  "If you log back in before then, your account won't be deleted.",
];

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
/** The Log in page's line. The button is off while the box is empty, so the server rarely says it. */
const PASSWORD_REQUIRED_MESSAGE = 'Enter your password.';
const TOO_MANY_MESSAGE = 'Too many tries. Wait a minute and try again.';

/**
 * Settings > Delete account (developer, 2026-10-25: "add the delete the account thing like how
 * instagram has"): what deleting does, the person's password, and one ask in the phone's own alert
 * before anything is sent.
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

  const canDelete = password !== '' && !deleting;

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
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Delete account
          </AppText>
        </View>

        {LINES.map((line) => (
          <View key={line} style={{ marginTop: space.sm }}>
            <AppText role="body">{line}</AppText>
          </View>
        ))}

        <View style={{ marginTop: space.lg }}>
          <TextField
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
          <PrimaryButton label="Delete account" disabled={!canDelete} loading={deleting} onPress={confirmDelete} />
        </View>
      </View>
    </Screen>
  );
}
