import { useRef, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import { TextField } from '@/components/ui/TextField';
import { ApiError } from '@/lib/api/client';
import { updateProfile } from '@/lib/api/profile';
import { BIO_LONG_MESSAGE, BIO_MAX, bioProblem, GENERIC_MESSAGE, sameText } from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/** The server's fixed words for a bio that is too long (field error on `bio`). */
const SERVER_TOO_LONG = 'This value is too long.';

function messageFor(err: unknown): string {
  if (!(err instanceof ApiError)) return GENERIC_MESSAGE;
  if (err.code === 'validation_failed' && err.field === 'bio' && err.message === SERVER_TOO_LONG) {
    return BIO_LONG_MESSAGE;
  }
  // Not valid (a control character), no connection, too many tries, anything else.
  return err.userMessage;
}

/** The small-caps label of the profile form's boxes, so Reading and Editing show the same label. */
const labelStyle = {
  fontFamily: type.button.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  letterSpacing: 2,
  textTransform: 'uppercase' as const,
  color: color.muted,
};

function TextAction({ label, tone, onPress }: { label: string; tone: 'ink' | 'muted'; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={label}
      style={{ minHeight: minTouchTarget, alignSelf: 'flex-start', justifyContent: 'center' }}
    >
      <Text
        style={{
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          color: tone === 'ink' ? color.ink : color.muted,
          textDecorationLine: 'underline',
        }}
      >
        {label}
      </Text>
    </Pressable>
  );
}

/**
 * The bio shown as text and edited in place (02.1-UI-SPEC section 7). It reads `user.bio` from the
 * session and writes only `PATCH /me {bio}`, so it works anywhere inside the signed in app: on
 * Settings now (with `label`) and under the @username on Phase 4's Profile page (without it).
 * Reading shows the text and an Edit bio link; Editing is the soft Bio box with Save and Cancel.
 */
export function BioInline({ label = false }: { label?: boolean }) {
  const { user, reloadUser } = useSession();
  const stored = user?.bio ?? '';
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const busyRef = useRef(false);

  const canSave = !saving && !sameText(text, stored) && bioProblem(text) === '';

  function startEditing() {
    setText(stored);
    setError('');
    setEditing(true);
  }

  function cancel() {
    setEditing(false);
    setError('');
  }

  async function save() {
    if (!canSave || busyRef.current) return;
    busyRef.current = true;
    setSaving(true);
    setError('');
    try {
      // An empty text clears the bio.
      await updateProfile({ bio: text.trim() });
      await reloadUser();
      setEditing(false);
    } catch (err) {
      setError(messageFor(err));
    } finally {
      busyRef.current = false;
      setSaving(false);
    }
  }

  if (editing) {
    return (
      <View>
        <TextField
          label="Bio"
          variant="soft"
          hint="Optional"
          value={text}
          onChangeText={(value) => {
            setText(value);
            if (error !== '') setError('');
          }}
          placeholder="Tell people what you're spinning for."
          multiline
          numberOfLines={3}
          maxLength={BIO_MAX}
          showCount
          autoFocus
          editable={!saving}
          textAlignVertical="top"
        />
        <View style={{ marginTop: space.sm, flexDirection: 'row', alignItems: 'center' }}>
          <TextButton label="Save" disabled={!canSave} onPress={save} />
          <View style={{ marginLeft: space.lg }}>
            <TextButton label="Cancel" tone="muted" disabled={saving} onPress={cancel} />
          </View>
        </View>
        {error !== '' ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.xs }}>
            <AppText role="label" tone="destructive">
              {error}
            </AppText>
          </View>
        ) : null}
      </View>
    );
  }

  return (
    <View>
      {label ? <Text style={labelStyle}>Bio</Text> : null}
      {stored !== '' ? (
        <View style={{ marginTop: label ? space.sm : 0 }}>
          <Pressable
            onPress={startEditing}
            accessibilityRole="button"
            accessibilityLabel={`Bio: ${stored}. Edit your bio.`}
          >
            <AppText role="body">{stored}</AppText>
          </Pressable>
          <View style={{ marginTop: space.xs }}>
            <TextAction label="Edit bio" tone="ink" onPress={startEditing} />
          </View>
        </View>
      ) : (
        <View style={{ marginTop: label ? space.sm : 0 }}>
          <TextAction label="Add a bio" tone="muted" onPress={startEditing} />
        </View>
      )}
    </View>
  );
}
