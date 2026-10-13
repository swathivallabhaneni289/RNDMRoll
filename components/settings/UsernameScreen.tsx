import { useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { BackRow } from '@/components/settings/BackRow';
import { ApiError } from '@/lib/api/client';
import { updateProfile } from '@/lib/api/profile';
import { LOOKS_GOOD_MESSAGE, GENERIC_MESSAGE, USERNAME_NOTICE } from '@/lib/settings/rules';
import { setSettingsNotice } from '@/lib/settings/notice';
import { useUsernameCheck } from '@/lib/settings/useUsernameCheck';
import { useSession } from '@/lib/session/store';
import { elevation, radius, space } from '@/lib/theme/tokens';

/**
 * The Username page (02.1-UI-SPEC section 5), pushed from the Username row. One soft box with the
 * "@" prefix holding the current name, the live check (the hook), the alternates as small chips,
 * one line saying what the name is for, and Save username. Save is `PATCH /me {username}`; on
 * success the session user is refreshed, Settings is told to show "Username changed." and the page
 * goes back. No password is asked and the old name is free for others at once.
 */
export function UsernameScreen() {
  const router = useRouter();
  const { user, reloadUser } = useSession();
  const stored = user?.username ?? '';
  const check = useUsernameCheck(stored);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState('');
  const busyRef = useRef(false);

  function handleChange(value: string) {
    if (formError !== '') setFormError('');
    check.setValue(value);
  }

  async function save() {
    if (!check.canSave || busyRef.current) return;
    busyRef.current = true;
    setSaving(true);
    setFormError('');
    try {
      await updateProfile({ username: check.value });
      await reloadUser();
      setSettingsNotice(USERNAME_NOTICE);
      if (router.canGoBack()) router.back();
      else router.replace('/(app)/profile');
    } catch (err) {
      if (err instanceof ApiError && err.code === 'username_taken') {
        // The check said free but the save raced into a taken name: the same taken state, with the
        // fresh alternates from that answer.
        check.markTaken(err.suggestions ?? []);
      } else {
        setFormError(err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE);
      }
    } finally {
      busyRef.current = false;
      setSaving(false);
    }
  }

  // The green note is the box's own success line; the box's "Available" look is not used here.
  const fieldStatus = check.status === 'available' ? 'idle' : check.status;

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Username
          </AppText>
        </View>

        <View style={{ marginTop: space.lg }}>
          <TextField
            label="Username"
            variant="soft"
            prefix="@"
            value={check.value}
            onChangeText={handleChange}
            status={fieldStatus}
            error={check.error}
            success={check.status === 'available' ? LOOKS_GOOD_MESSAGE : undefined}
            autoCapitalize="none"
            autoCorrect={false}
          />
          {check.status === 'taken' && check.alternates.length > 0 ? (
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', marginTop: space.xs, gap: space.xs }}>
              {check.alternates.map((alternate) => (
                <Pressable
                  key={alternate}
                  onPress={() => check.pick(alternate)}
                  accessibilityRole="button"
                  accessibilityLabel={`Use username ${alternate}`}
                  hitSlop={8}
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
          <AppText role="label" tone="muted">
            This is how people find you and how you log in.
          </AppText>
        </View>

        {formError !== '' ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Save username" disabled={!check.canSave} loading={saving} onPress={save} />
        </View>
      </View>
    </Screen>
  );
}
