import { useEffect, useRef, useState } from 'react';
import { Pressable, Switch, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { SETTINGS_ROW_MIN_HEIGHT } from '@/components/settings/SettingsRow';
import {
  getNotificationSettings,
  NOTIFICATION_DEFAULTS,
  updateNotificationSettings,
  type NotificationSettingKey,
  type NotificationSettings,
} from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

/** The three switches, in the order the developer gave them (decision D-16). */
const ROWS: { key: NotificationSettingKey; label: string }[] = [
  { key: 'reactions', label: 'Reactions and likes' },
  { key: 'comments', label: 'Comments' },
  { key: 'milestones', label: 'Milestones and bonus spins' },
];

type Flags = Partial<Record<NotificationSettingKey, boolean>>;
type Messages = Partial<Record<NotificationSettingKey, string>>;

/**
 * One switch row: the label at the left and the system switch at the right, a hairline under it,
 * and, under that, one red line when the last save failed. The whole row is one switch element: a
 * tap anywhere on it toggles, and the system switch inside it ignores touches itself (the same
 * build as the Private account row).
 */
function SwitchRow({
  label,
  checked,
  saving,
  error,
  onToggle,
}: {
  label: string;
  checked: boolean;
  saving: boolean;
  error: string;
  onToggle: () => void;
}) {
  return (
    <View style={{ borderBottomWidth: 1, borderBottomColor: color.divider }}>
      <Pressable
        onPress={onToggle}
        disabled={saving}
        accessibilityRole="switch"
        accessibilityLabel={label}
        accessibilityState={{ checked, disabled: saving }}
        style={{
          minHeight: SETTINGS_ROW_MIN_HEIGHT,
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <View style={{ flex: 1, marginRight: space.md }}>
          <AppText role="body">{label}</AppText>
        </View>
        <View pointerEvents="none">
          <Switch
            value={checked}
            trackColor={{ false: color.divider, true: color.ink }}
            thumbColor={color.dominant}
            ios_backgroundColor={color.divider}
          />
        </View>
      </Pressable>
      {error !== '' ? (
        <View accessibilityLiveRegion="polite" style={{ paddingBottom: space.sm }}>
          <AppText role="label" tone="default">
            {error}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

/**
 * Settings > Notifications (decision D-16): three switches, all ON until the person switches one
 * off. Each one saves on its own toggle (`PATCH /me/notification-settings`); the change shows at
 * once, and on a failure the switch goes back and one red line says why.
 *
 * The server side is built after Phase 3's server, so `GET` may 404 for now. Then the switches
 * simply show ON (the defaults); there is never an error screen. A toggle then fails with the red
 * line and goes back, which is the specified failure behaviour, not a bug.
 */
export function NotificationSettingsScreen() {
  const [values, setValues] = useState<NotificationSettings>(NOTIFICATION_DEFAULTS);
  const [saving, setSaving] = useState<Flags>({});
  const [errors, setErrors] = useState<Messages>({});
  // Keys the person has already touched: a late answer to the first load never overwrites them.
  const touched = useRef<Flags>({});
  const busy = useRef<Flags>({});

  useEffect(() => {
    let alive = true;
    getNotificationSettings()
      .then((loaded) => {
        if (!alive) return;
        setValues((current) => ({
          reactions: touched.current.reactions ? current.reactions : loaded.reactions,
          comments: touched.current.comments ? current.comments : loaded.comments,
          milestones: touched.current.milestones ? current.milestones : loaded.milestones,
        }));
      })
      .catch(() => {
        // Not live yet, or no connection: the switches keep showing ON.
      });
    return () => {
      alive = false;
    };
  }, []);

  async function toggle(key: NotificationSettingKey) {
    if (busy.current[key]) return;
    busy.current[key] = true;
    touched.current[key] = true;
    const before = values[key];
    const next = !before;
    setErrors((current) => ({ ...current, [key]: '' }));
    setSaving((current) => ({ ...current, [key]: true }));
    setValues((current) => ({ ...current, [key]: next }));
    try {
      const answer = await updateNotificationSettings({ [key]: next });
      // The server's body is what is stored now; this key follows it.
      setValues((current) => ({ ...current, [key]: answer[key] }));
    } catch (err) {
      setValues((current) => ({ ...current, [key]: before }));
      setErrors((current) => ({ ...current, [key]: err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE }));
    } finally {
      busy.current[key] = false;
      setSaving((current) => ({ ...current, [key]: false }));
    }
  }

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Notifications
          </AppText>
        </View>

        <View style={{ marginTop: space.lg, borderTopWidth: 1, borderTopColor: color.divider }}>
          {ROWS.map((row) => (
            <SwitchRow
              key={row.key}
              label={row.label}
              checked={values[row.key]}
              saving={saving[row.key] === true}
              error={errors[row.key] ?? ''}
              onToggle={() => void toggle(row.key)}
            />
          ))}
        </View>
      </View>
    </Screen>
  );
}
