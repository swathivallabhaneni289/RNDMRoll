import { Pressable, View } from 'react-native';
import { CapsLabel } from '@/components/today/CapsLabel';
import { AppText } from '@/components/ui/AppText';
import { dateDot } from '@/lib/wheel/format';
import { minTouchTarget, space } from '@/lib/theme/tokens';

type CategoryHeaderProps = {
  /** `new` is the entry screen. `edit` is plan 03-03's edit header ('Edit entry' centred, the entry's own date at the right). */
  variant?: 'new' | 'edit';
  /** The server's civil date: the spin's `local_date` (new) or the entry's (edit). Written as THU · OCT 8. */
  localDate: string;
  /** The category as written on that day. */
  categoryName: string;
  /** New only: the spin is last night's, so the label and the late line change. */
  late?: boolean;
  onCancel: () => void;
};

/**
 * The fixed header of the entry screen (UI spec 4): 'Cancel' and the date on a 44dp row, the
 * category word large with its small label at the right on one baseline, and the late line.
 * The word shrinks to fit down to 60 percent and is then cut with an ellipsis.
 */
export function CategoryHeader({
  variant = 'new',
  localDate,
  categoryName,
  late = false,
  onCancel,
}: CategoryHeaderProps) {
  const edit = variant === 'edit';
  const spinLabel = late ? "Last night's spin" : "Today's spin";
  const date = dateDot(localDate);

  return (
    <View>
      <View style={{ height: minTouchTarget, alignItems: 'center', justifyContent: 'center' }}>
        <Pressable
          onPress={onCancel}
          accessibilityRole="button"
          accessibilityLabel="Cancel"
          hitSlop={{ right: 12 }}
          style={{ position: 'absolute', left: 0, top: 0, height: minTouchTarget, justifyContent: 'center' }}
        >
          <AppText role="body">Cancel</AppText>
        </Pressable>
        <CapsLabel>{edit ? 'Edit entry' : date}</CapsLabel>
      </View>

      <View style={{ marginTop: space.sm, flexDirection: 'row', alignItems: 'baseline' }}>
        <View style={{ flex: 1 }}>
          <AppText
            role="countdown"
            numberOfLines={1}
            adjustsFontSizeToFit
            minimumFontScale={0.6}
            accessibilityRole="header"
            accessibilityLabel={edit ? categoryName : `${spinLabel}: ${categoryName}.`}
          >
            {categoryName}
          </AppText>
        </View>
        <View
          style={{ marginLeft: space.md }}
          accessibilityElementsHidden={!edit}
          importantForAccessibility={edit ? 'yes' : 'no-hide-descendants'}
        >
          <CapsLabel>{edit ? date : spinLabel}</CapsLabel>
        </View>
      </View>

      {late && !edit ? (
        <View style={{ marginTop: space.xs }}>
          <AppText role="label" tone="muted">
            Posting it now would count as late.
          </AppText>
        </View>
      ) : null}
    </View>
  );
}
