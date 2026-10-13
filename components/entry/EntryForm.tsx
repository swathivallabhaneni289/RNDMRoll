import { ReactNode, useRef } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, Text, View } from 'react-native';
import { PhotoField } from '@/components/entry/PhotoField';
import { ReactionPicker } from '@/components/entry/ReactionPicker';
import { StarInput } from '@/components/entry/StarInput';
import { UnderlineField } from '@/components/entry/UnderlineField';
import type { EntryDraft } from '@/components/entry/useEntryDraft';
import { CapsLabel } from '@/components/today/CapsLabel';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import type { Entry } from '@/lib/api/entries';
import { COMMENT_MAX, stillNeededLine, TITLE_MAX } from '@/lib/entry/rules';
import { formatStars } from '@/lib/entry/stars';
import { color, space, type } from '@/lib/theme/tokens';

type EntryFormProps = {
  mode: 'new' | 'edit';
  /** The entry being edited (edit mode). The draft was started from it; the form reads the draft. */
  initial?: Entry | null;
  draft: EntryDraft;
  /** 'Save' for a new entry, 'Save changes' for edit. */
  submitLabel: string;
  submitting: boolean;
  /** A failure line shown above the button, in `destructive`. */
  message?: string | null;
  /** The attempt cannot go on (the day moved on): the button reads 'Back to Today' and calls `onTerminal`. */
  terminal?: boolean;
  onTerminal?: () => void;
  /** A failed photo upload, shown under the photo strip. */
  photoMessage?: string | null;
  onSubmit: () => void;
  /** The fixed header (a `CategoryHeader`), drawn above the scrolling middle. */
  header: ReactNode;
};

const labelLine = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  textAlign: 'center',
} as const;

/**
 * The entry form (UI spec 4): the fixed `header`, a scrolling middle (Photos, Title, Rating,
 * Reaction, Comment) and a pinned footer with the 'Still needed' line, the failure line and the
 * button. Built the way the Edit wheel screen is: one KeyboardAvoidingView holds the scroll area
 * and the footer, so the footer rides above the keyboard. The route owns the `Screen`, the leave
 * guard and the navigation; this component never navigates.
 */
export function EntryForm({
  mode,
  draft,
  submitLabel,
  submitting,
  message = null,
  terminal = false,
  onTerminal,
  photoMessage = null,
  onSubmit,
  header,
}: EntryFormProps) {
  const scrollRef = useRef<ScrollView>(null);
  const commentFocusedRef = useRef(false);

  const needed = stillNeededLine(draft.missing);
  const canSubmit = !submitting && draft.missing.length === 0 && (mode === 'new' || draft.dirty);
  const figure = formatStars(draft.stars);

  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
      {header}

      <ScrollView
        ref={scrollRef}
        keyboardShouldPersistTaps="handled"
        showsVerticalScrollIndicator={false}
        // The keyboard shrinks this area; keep the comment in view while it has focus.
        onLayout={() => {
          if (commentFocusedRef.current) scrollRef.current?.scrollToEnd({ animated: true });
        }}
        contentContainerStyle={{ paddingTop: space.lg, paddingBottom: space.xl }}
      >
        <View pointerEvents={submitting ? 'none' : 'auto'}>
          <PhotoField
            photos={draft.photos}
            mode={mode}
            onAdd={draft.addPhoto}
            onRemove={draft.removePhoto}
            message={photoMessage}
          />

          <View style={{ marginTop: space.lg }}>
            <UnderlineField
              label="Title"
              accessibilityLabel="Title"
              value={draft.title}
              onChangeText={draft.setTitle}
              maxLength={TITLE_MAX}
              count="focused"
              autoCapitalize="sentences"
              returnKeyType="done"
            />
          </View>

          <View style={{ marginTop: space.lg }}>
            <View
              style={{
                minHeight: type.note.lineHeight,
                flexDirection: 'row',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <CapsLabel>Rating</CapsLabel>
              <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
                <AppText role="note">{figure}</AppText>
              </View>
            </View>
            <View style={{ marginTop: space.sm }}>
              <StarInput value={draft.stars} onChange={draft.setStars} />
            </View>
          </View>

          <View style={{ marginTop: space.lg }}>
            <CapsLabel>Reaction · Optional</CapsLabel>
            <View style={{ marginTop: space.sm }}>
              <ReactionPicker value={draft.reaction} onChange={draft.setReaction} />
            </View>
          </View>

          <View style={{ marginTop: space.lg }}>
            <UnderlineField
              label="Comment · Optional"
              accessibilityLabel="Comment, optional"
              placeholder="A line about it"
              value={draft.comment}
              onChangeText={draft.setComment}
              maxLength={COMMENT_MAX}
              multiline
              maxLines={3}
              count="always"
              countJoin=" / "
              onFocus={() => {
                commentFocusedRef.current = true;
                scrollRef.current?.scrollToEnd({ animated: true });
              }}
              onBlur={() => {
                commentFocusedRef.current = false;
              }}
            />
          </View>
        </View>
      </ScrollView>

      <View style={{ paddingVertical: space.md, backgroundColor: color.dominant }}>
        {needed !== '' && !terminal ? (
          <View style={{ marginBottom: space.sm }} accessibilityLiveRegion="polite">
            <Text style={{ ...labelLine, color: color.muted }}>{needed}</Text>
          </View>
        ) : null}
        {message ? (
          <View style={{ marginBottom: space.sm }} accessibilityLiveRegion="polite">
            <Text style={{ ...labelLine, color: color.destructive }}>{message}</Text>
          </View>
        ) : null}
        <PrimaryButton
          label={terminal ? 'Back to Today' : submitLabel}
          onPress={terminal ? onTerminal : onSubmit}
          disabled={terminal ? false : !canSubmit}
          loading={terminal ? false : submitting}
        />
      </View>
    </KeyboardAvoidingView>
  );
}
