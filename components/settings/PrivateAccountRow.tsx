import { useRef, useState } from 'react';
import { Pressable, Switch, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { SETTINGS_ROW_MIN_HEIGHT } from '@/components/settings/SettingsRow';
import { ApiError } from '@/lib/api/client';
import { updateProfile } from '@/lib/api/profile';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';
import { color, space } from '@/lib/theme/tokens';

const SENTENCE = 'Public accounts can appear on Global. Private accounts are seen by friends only.';

/**
 * The Private account row (02.1-UI-SPEC section 8). The whole row is one switch element: a tap
 * anywhere on it toggles, and the system switch drawn inside it ignores touches itself. The change
 * shows at once and the row is disabled while it is saved; on success the session user is
 * refreshed, on failure the switch goes back and one red line says why. No confirmation: the
 * choice is reversible. ON means 'private', OFF means 'public' (the default for everyone).
 */
export function PrivateAccountRow() {
  const { user, reloadUser } = useSession();
  const stored = user?.account_visibility === 'private';
  // The value being saved, shown at once; null when nothing is in flight.
  const [pending, setPending] = useState<boolean | null>(null);
  const [error, setError] = useState('');
  const busyRef = useRef(false);

  const checked = pending ?? stored;
  const saving = pending !== null;

  async function toggle() {
    if (busyRef.current || user === null) return;
    busyRef.current = true;
    const next = !checked;
    setError('');
    setPending(next);
    try {
      await updateProfile({ account_visibility: next ? 'private' : 'public' });
      await reloadUser();
    } catch (err) {
      // The switch returns to what the stored value says (the finally below drops the pending one).
      setError(err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE);
    } finally {
      busyRef.current = false;
      setPending(null);
    }
  }

  return (
    <View style={{ borderBottomWidth: 1, borderBottomColor: color.divider, paddingBottom: space.md }}>
      <Pressable
        onPress={toggle}
        disabled={saving}
        accessibilityRole="switch"
        accessibilityLabel="Private account"
        accessibilityHint={SENTENCE}
        accessibilityState={{ checked, disabled: saving }}
        style={{
          minHeight: SETTINGS_ROW_MIN_HEIGHT,
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <AppText role="body">Private account</AppText>
        <View pointerEvents="none">
          <Switch
            value={checked}
            trackColor={{ false: color.divider, true: color.ink }}
            thumbColor={color.dominant}
            ios_backgroundColor={color.divider}
          />
        </View>
      </Pressable>
      <View style={{ marginTop: space.xs }}>
        <AppText role="label" tone="muted">
          {SENTENCE}
        </AppText>
      </View>
      {error !== '' ? (
        <View accessibilityLiveRegion="polite" style={{ marginTop: space.xs }}>
          <AppText role="label" tone="default">
            {error}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}
