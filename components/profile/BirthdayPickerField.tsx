import { useMemo, useState } from 'react';
import { Keyboard, Modal, Platform, Pressable, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { FieldSuccess } from '@/components/ui/TextField';
import { MIN_BIRTHDAY_YEAR, type BirthdayParts } from '@/lib/profile/birthday';
import { color, elevation, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

/**
 * The birthday as one tap instead of eight typed digits: a soft box that shows the date, and a
 * bottom sheet with iOS's own scrolling date picker (three columns: month, day, year) and a
 * Done button. The picker is the SwiftUI DatePicker from @expo/ui, which is already compiled
 * into the dev client, so there is no new native module. iOS only: anywhere else (or if the
 * JavaScript side cannot be loaded) `birthdayPickerAvailable` is false and the form keeps its
 * three typed boxes. A dev client built WITHOUT the ExpoUI pod would not fall back: the sheet
 * would fail when it opens, so keep ExpoUI in every dev client build. The birthday cannot be
 * changed after sign-up, so Done stays off until the picker has moved: the starting date is
 * only a guess and must never be accepted by accident.
 */

type SwiftUi = typeof import('@expo/ui/swift-ui');
type SwiftUiModifiers = typeof import('@expo/ui/swift-ui/modifiers');

// Loaded inside try/catch so a JavaScript load failure falls back to the typed boxes instead of
// crashing the sign-up page at import time. A missing native view does not throw here.
function loadNativePicker(): {
  Host: SwiftUi['Host'];
  DatePicker: SwiftUi['DatePicker'];
  datePickerStyle: SwiftUiModifiers['datePickerStyle'];
} | null {
  if (Platform.OS !== 'ios') return null;
  try {
    const ui = require('@expo/ui/swift-ui') as SwiftUi;
    const modifiers = require('@expo/ui/swift-ui/modifiers') as SwiftUiModifiers;
    return { Host: ui.Host, DatePicker: ui.DatePicker, datePickerStyle: modifiers.datePickerStyle };
  } catch {
    return null;
  }
}

const nativePicker = loadNativePicker();

export const birthdayPickerAvailable = nativePicker !== null;

const WHEEL_MODIFIERS = nativePicker ? [nativePicker.datePickerStyle('wheel')] : [];
// The height of iOS's own date wheel.
const WHEEL_HEIGHT = 216;
// Where the picker starts. Only a guess; the person has to move it before Done works.
const STARTING_AGE_YEARS = 25;

const MONTH_NAMES = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
];

// Noon, local time, so converting through a UTC string can never move the calendar day.
function atNoon(year: number, monthIndex: number, day: number): Date {
  return new Date(year, monthIndex, day, 12, 0, 0);
}

function startingDate(): Date {
  return atNoon(new Date().getFullYear() - STARTING_AGE_YEARS, 0, 1);
}

/** The date the three parts name, or null while they are empty or not a real date. */
function partsToDate(parts: BirthdayParts): Date | null {
  if (parts.month === '' || parts.day === '' || parts.year.length !== 4) return null;
  const month = Number(parts.month);
  const day = Number(parts.day);
  const year = Number(parts.year);
  const date = atNoon(year, month - 1, day);
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return date;
}

function formatDate(date: Date): string {
  return `${MONTH_NAMES[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()}`;
}

type BirthdayPickerFieldProps = {
  /** What the form holds now; the box shows it once it is a real date. */
  value: BirthdayParts;
  /** Called with the chosen date (local calendar day) when Done is tapped. */
  onPick: (date: Date) => void;
  /** The form's message for this value, shown in place of the green note. */
  error?: string;
  /** The green note shown when the value is good. */
  success?: string;
};

export function BirthdayPickerField({ value, onPick, error, success }: BirthdayPickerFieldProps) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<Date>(startingDate);
  const [moved, setMoved] = useState(false);
  const range = useMemo(() => ({ start: atNoon(MIN_BIRTHDAY_YEAR, 0, 1), end: new Date() }), []);

  if (!nativePicker) return null;
  const { Host, DatePicker } = nativePicker;

  const picked = partsToDate(value);

  function openSheet() {
    Keyboard.dismiss();
    setDraft(picked ?? startingDate());
    setMoved(false);
    setOpen(true);
  }

  function closeSheet() {
    setOpen(false);
  }

  function handleChange(date: Date) {
    setDraft(date);
    setMoved(true);
  }

  function handleDone() {
    onPick(draft);
    setOpen(false);
  }

  const labelStyle = {
    fontFamily: type.button.fontFamily,
    fontSize: type.label.fontSize,
    lineHeight: type.label.lineHeight,
    letterSpacing: 2,
    textTransform: 'uppercase' as const,
    color: color.muted,
  };

  return (
    <View>
      <Text style={labelStyle}>Birthday</Text>
      <Pressable
        onPress={openSheet}
        accessibilityRole="button"
        accessibilityLabel={picked ? `Birthday, ${formatDate(picked)}` : 'Birthday, not set'}
        accessibilityHint="Opens the date picker"
        style={{
          marginTop: space.xs,
          backgroundColor: color.fieldSoft,
          borderRadius: radius.md,
          borderWidth: open ? 1.5 : 1,
          borderColor: open ? color.ink : color.inkRest,
          paddingHorizontal: 14,
          minHeight: minTouchTarget + 6,
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <AppText role="body" tone={picked ? 'default' : 'muted'}>
          {picked ? formatDate(picked) : 'Select your birthday'}
        </AppText>
        <Ionicons name="chevron-down" size={18} color={color.muted} />
      </Pressable>
      {error ? (
        <View accessibilityLiveRegion="polite" style={{ marginTop: space.xs }}>
          <AppText role="label" tone="destructive">
            {error}
          </AppText>
        </View>
      ) : success ? (
        <FieldSuccess text={success} label="Birthday" />
      ) : null}

      <Modal visible={open} transparent animationType="fade" onRequestClose={closeSheet}>
        <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
          <Pressable
            onPress={closeSheet}
            accessibilityRole="button"
            accessibilityLabel="Close"
            style={{ flex: 1 }}
          />
          <View
            style={{
              ...elevation.card,
              borderTopLeftRadius: radius.lg,
              borderTopRightRadius: radius.lg,
              padding: space.lg,
              paddingBottom: space.xl,
            }}
          >
            <Text style={labelStyle}>Birthday</Text>
            <Host colorScheme="light" style={{ alignSelf: 'stretch', height: WHEEL_HEIGHT, marginTop: space.sm }}>
              <DatePicker
                selection={draft}
                range={range}
                displayedComponents={['date']}
                onDateChange={handleChange}
                modifiers={WHEEL_MODIFIERS}
              />
            </Host>
            <View style={{ marginTop: space.md }}>
              <PrimaryButton label="Done" onPress={handleDone} disabled={!moved} />
            </View>
          </View>
        </View>
      </Modal>
    </View>
  );
}
