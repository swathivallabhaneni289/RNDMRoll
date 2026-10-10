import { Modal, Pressable, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { color, elevation, radius, space } from '@/lib/theme/tokens';

/** Each row is at least this tall (UI-SPEC 4.3). */
const ROW_MIN_HEIGHT = 56;

function SheetRow({ label, onPress, disabled }: { label: string; onPress: () => void; disabled: boolean }) {
  return (
    <Pressable
      onPress={disabled ? undefined : onPress}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      style={{
        minHeight: ROW_MIN_HEIGHT,
        justifyContent: 'center',
        alignItems: 'center',
        borderTopWidth: 1,
        borderTopColor: color.divider,
      }}
    >
      <AppText role="button" tone={disabled ? 'muted' : 'default'}>
        {label}
      </AppText>
    </Pressable>
  );
}

/**
 * A bottom sheet with a title, a line of text and two full-width rows separated by hairlines
 * (02-UI-SPEC 4.3 and 7). It repeats the Log out sheet's shape (Modal, `scrimOverlay`, `radius.lg`
 * top corners, `space.lg` padding) with rows in place of centred text buttons. Used by the bonus
 * spin sheet and the leave sheet of Edit wheel; the Log out sheet itself is not touched.
 *
 * The first row is the action the sheet asks about ("Spin again", "Leave"); the second dismisses
 * ("Keep this word", "Keep editing"). The scrim and the system back gesture also dismiss.
 */
export function ConfirmSheet({
  visible,
  title,
  body,
  confirmLabel,
  dismissLabel,
  onConfirm,
  onDismiss,
  busy = false,
}: {
  visible: boolean;
  title: string;
  body: string;
  confirmLabel: string;
  dismissLabel: string;
  onConfirm: () => void;
  onDismiss: () => void;
  busy?: boolean;
}) {
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onDismiss}>
      <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
        <Pressable
          onPress={onDismiss}
          accessibilityRole="button"
          accessibilityLabel={dismissLabel}
          style={{ flex: 1 }}
        />
        <View
          accessibilityViewIsModal
          style={{
            ...elevation.card,
            borderTopLeftRadius: radius.lg,
            borderTopRightRadius: radius.lg,
            padding: space.lg,
            paddingBottom: space.xl,
          }}
        >
          <AppText role="heading">{title}</AppText>
          <View style={{ marginTop: space.sm }}>
            <AppText role="body">{body}</AppText>
          </View>
          <View style={{ marginTop: space.lg }}>
            <SheetRow label={confirmLabel} onPress={onConfirm} disabled={busy} />
            <SheetRow label={dismissLabel} onPress={onDismiss} disabled={busy} />
          </View>
        </View>
      </View>
    </Modal>
  );
}
