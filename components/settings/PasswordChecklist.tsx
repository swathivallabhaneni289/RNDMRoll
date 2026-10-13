import { View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { passwordChecks } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

/**
 * The four things a new password needs, each turning green with a tick as the typed password
 * meets it. Used under the new password box of Settings and under the password box of sign-up.
 */
export function PasswordChecklist({ value }: { value: string }) {
  const checks = passwordChecks(value);
  return (
    <View style={{ marginTop: space.xs }}>
      {checks.map((check) => (
        <View
          key={check.label}
          accessible
          accessibilityLabel={`${check.label}, ${check.met ? 'done' : 'not yet'}`}
          style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: 2 }}
        >
          <Ionicons
            name={check.met ? 'checkmark-circle' : 'ellipse-outline'}
            size={16}
            color={check.met ? color.success : color.muted}
          />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="label" tone={check.met ? 'success' : 'muted'}>
              {check.label}
            </AppText>
          </View>
        </View>
      ))}
    </View>
  );
}
