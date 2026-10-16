import { Text, View } from 'react-native';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { color, space, type } from '@/lib/theme/tokens';

export type StatusKind = 'network' | 'not_open' | 'closed' | 'generic';

/** The four messages of 02-UI-SPEC section 4.4, verbatim. */
export const STATUS_MESSAGES: Record<StatusKind, string> = {
  network: "Couldn't connect. Check your connection and try again.",
  not_open: "Your spin isn't open yet. It opens at 8:00 PM.",
  closed: "Today's spin has closed. The next one opens at 8:00 PM.",
  generic: 'Something went wrong. Please try again.',
};

/**
 * One plain Label line in `ink`, centred, with the app's primary button "Try again".
 * What "Try again" does is the caller's business (a retry after a lost connection sends the
 * same request again; the other messages ask the server for Today).
 */
export function StatusMessage({
  kind,
  onRetry,
  busy = false,
}: {
  kind: StatusKind;
  onRetry: () => void;
  busy?: boolean;
}) {
  return (
    <View style={{ alignSelf: 'stretch' }}>
      <Text
        accessibilityLiveRegion="polite"
        style={{
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          color: color.ink,
          textAlign: 'center',
        }}
      >
        {STATUS_MESSAGES[kind]}
      </Text>
      <View style={{ marginTop: space.md }}>
        <PrimaryButton label="Try again" onPress={onRetry} loading={busy} />
      </View>
    </View>
  );
}
