import { useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { DialAvatar } from '@/components/brand/DialAvatar';
import { Screen } from '@/components/ui/Screen';
import { color, elevation, radius, space, type } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';

const ROW_HEIGHT = 56;
const hairline = { borderColor: color.divider, borderTopWidth: StyleSheet.hairlineWidth } as const;

/** A full-width row between hairlines: the profile's actions, and the log-out sheet's. */
function ActionRow({
  label,
  onPress,
  tone = 'default',
  arrow = false,
  last = false,
  disabled = false,
}: {
  label: string;
  onPress: () => void;
  tone?: 'default' | 'destructive';
  arrow?: boolean;
  last?: boolean;
  disabled?: boolean;
}) {
  return (
    <Pressable
      onPress={disabled ? undefined : onPress}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      style={({ pressed }) => ({
        ...hairline,
        borderBottomWidth: last ? StyleSheet.hairlineWidth : 0,
        minHeight: ROW_HEIGHT,
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'space-between',
        opacity: pressed ? 0.55 : 1,
      })}
    >
      <AppText role="button" tone={disabled ? 'muted' : tone}>
        {label}
      </AppText>
      {arrow ? <Ionicons name="arrow-forward" size={18} color={color.ink} /> : null}
    </Pressable>
  );
}

/**
 * ACCT-03's profile view. D-07 is exact about scope: identity fields only
 * (avatar, name, username, bio), nothing summarizing a feature that
 * doesn't exist yet this phase, since a placeholder for it would be
 * inventing product surface with no data behind it. Renders the session
 * user directly (lib/session/store.ts); edit.tsx's reloadUser() call after
 * a save is what keeps this screen fresh, so it performs no fetch of its
 * own.
 *
 * Layout (UI-SPEC revision 12): a left-aligned masthead (small caps label, the
 * dial-ring avatar, the name in the display serif, the username), the bio between
 * two hairlines, and the actions as rows pinned to the bottom. No card, no shadow.
 */
export default function ProfileScreen() {
  const router = useRouter();
  const { user, signOut } = useSession();
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

  const trimmedBio = user?.bio?.trim() ?? '';
  const hasBio = trimmedBio.length > 0;

  async function handleConfirmSignOut() {
    setSigningOut(true);
    try {
      await signOut();
      // The root guard (plan 01-05) routes back to the method chooser once
      // status flips to unauthenticated; this screen performs no explicit
      // navigation of its own.
    } finally {
      setSigningOut(false);
      setConfirmingSignOut(false);
    }
  }

  return (
    <Screen>
      <View style={{ flex: 1, paddingTop: space.lg, paddingBottom: space.lg }}>
        <Text
          accessibilityRole="header"
          style={{
            fontFamily: type.button.fontFamily,
            fontSize: type.label.fontSize,
            lineHeight: type.label.lineHeight,
            letterSpacing: 2.4,
            textTransform: 'uppercase',
            color: color.muted,
          }}
        >
          Profile
        </Text>

        <View style={{ marginTop: space.xl }}>
          <DialAvatar name={user?.name ?? ''} uri={user?.avatar_url} />
        </View>

        <View style={{ marginTop: space.lg }}>
          <AppText role="display" numberOfLines={3} adjustsFontSizeToFit minimumFontScale={0.6}>
            {user?.name ?? ''}
          </AppText>
          <View style={{ marginTop: space.xs }}>
            <AppText role="body" tone="muted">{`@${user?.username ?? ''}`}</AppText>
          </View>
        </View>

        <View style={{ ...hairline, borderBottomWidth: StyleSheet.hairlineWidth, marginTop: space.xl }}>
          {hasBio ? (
            <View style={{ paddingVertical: space.lg }}>
              <AppText role="body">{trimmedBio}</AppText>
            </View>
          ) : (
            <Pressable
              onPress={() => router.push('/(app)/profile/edit')}
              accessibilityRole="button"
              accessibilityLabel="Add a bio"
              style={({ pressed }) => ({
                flexDirection: 'row',
                alignItems: 'center',
                paddingVertical: space.lg,
                opacity: pressed ? 0.55 : 1,
              })}
            >
              <View style={{ flex: 1 }}>
                <AppText role="heading">Add a bio</AppText>
                <AppText role="body" tone="muted">
                  Tell people what you're spinning for.
                </AppText>
              </View>
              <Ionicons name="arrow-forward" size={18} color={color.ink} />
            </Pressable>
          )}
        </View>

        <View style={{ flex: 1, minHeight: space.xl }} />

        <ActionRow label="Edit profile" arrow onPress={() => router.push('/(app)/profile/edit')} />
        <ActionRow label="Log out" tone="destructive" last onPress={() => setConfirmingSignOut(true)} />
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
            <View style={{ marginTop: space.lg }}>
              <ActionRow
                label="Log out"
                tone="destructive"
                disabled={signingOut}
                onPress={handleConfirmSignOut}
              />
              <ActionRow
                label="Stay logged in"
                last
                disabled={signingOut}
                onPress={() => setConfirmingSignOut(false)}
              />
            </View>
          </View>
        </View>
      </Modal>
    </Screen>
  );
}
