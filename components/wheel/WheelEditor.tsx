import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from 'react-native';
import { useNavigation, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { ConfirmSheet } from '@/components/ui/ConfirmSheet';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { Wheel } from '@/components/wheel/Wheel';
import { ApiError } from '@/lib/api/client';
import { getWheel, saveWheel } from '@/lib/api/spin';
import type { WheelCategory, WheelLimits } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';
import { clearPendingSpin, useTodaySnapshot } from '@/lib/wheel/todayStore';

/** The limits the server sends; used until the first answer arrives and as a floor if it omits them. */
const DEFAULT_LIMITS: WheelLimits = { min: 2, max: 12, name_max: 24, weight_min: 1, weight_max: 10 };

/**
 * Every category has the same chance (D-23), so there is no number to edit and every Save sends
 * 1 for every row, new and saved. A wheel saved earlier with other numbers is made even by its
 * next Save.
 */
const EQUAL_WEIGHT = 1;

const PREVIEW_SIZE = 240;
const NAME_FIELD_HEIGHT = 48;
const ROW_MIN_HEIGHT = 56;

const NETWORK_MESSAGE = "Couldn't connect. Check your connection and try again.";
const GENERIC_MESSAGE = 'Something went wrong. Please try again.';
const NAME_EMPTY_MESSAGE = 'Give this category a name.';
const NAME_TAKEN_MESSAGE = 'You already have a category with this name.';

type Row = {
  /** A stable key for the list; not sent. */
  key: string;
  /** The saved category's id, or null for a row added in this visit. */
  id: string | null;
  name: string;
  /** The field has been left once, so its error may show. */
  touched: boolean;
};

let rowKeyCounter = 0;
function nextRowKey(): string {
  rowKeyCounter += 1;
  return `row-${rowKeyCounter}`;
}

function rowsFrom(categories: WheelCategory[]): Row[] {
  return [...categories]
    .sort((a, b) => a.position - b.position)
    .map((category) => ({
      key: nextRowKey(),
      id: category.id,
      name: category.name,
      touched: false,
    }));
}

function signatureOf(rows: ReadonlyArray<Pick<Row, 'id' | 'name'>>): string {
  return rows.map((row) => `${row.id ?? 'new'}\u0001${row.name.trim()}`).join('\u0002');
}

const labelText = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
} as const;

/**
 * Edit wheel (02-UI-SPEC section 7): a live preview, one row per category (name field and
 * Remove), Add a category, and a pinned "Save wheel". There are no numbers on this screen:
 * every category has the same chance, and Save sends weight 1 for every row. Changes stay on
 * the phone until Save, which sends the whole list in one PATCH (all or nothing); a saved
 * category that is removed is simply left out of the list, which archives it on the server.
 * Every way out with unsaved changes (Back, the iOS swipe, a Home or Spin tab press that pops
 * the stack) opens the leave sheet through one `beforeRemove` guard; the swipe is switched off
 * while changes are unsaved.
 */
export function WheelEditor() {
  const router = useRouter();
  const navigation = useNavigation();
  const { width } = useWindowDimensions();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const snapshot = useTodaySnapshot(userId);
  const spunToday = snapshot.today?.spin != null;

  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [loadMessage, setLoadMessage] = useState(GENERIC_MESSAGE);
  const [version, setVersion] = useState(0);
  const [limits, setLimits] = useState<WheelLimits>(DEFAULT_LIMITS);
  const [rows, setRows] = useState<Row[]>([]);
  const [baseline, setBaseline] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [leaveOpen, setLeaveOpen] = useState(false);
  const [focusKey, setFocusKey] = useState<string | null>(null);

  const allowLeaveRef = useRef(false);
  const pendingActionRef = useRef<Parameters<typeof navigation.dispatch>[0] | null>(null);
  const savingRef = useRef(false);

  const applyServerWheel = useCallback((next: { version: number; categories: WheelCategory[]; limits: WheelLimits }) => {
    const nextRows = rowsFrom(next.categories);
    setVersion(next.version);
    setLimits({ ...DEFAULT_LIMITS, ...next.limits });
    setRows(nextRows);
    setBaseline(signatureOf(nextRows));
  }, []);

  const load = useCallback(async () => {
    setStatus('loading');
    try {
      const wheel = await getWheel();
      applyServerWheel(wheel);
      setStatus('ready');
    } catch (err) {
      setLoadMessage(err instanceof ApiError && err.code === 'network_unavailable' ? NETWORK_MESSAGE : GENERIC_MESSAGE);
      setStatus('error');
    }
  }, [applyServerWheel]);

  useEffect(() => {
    void load();
  }, [load]);

  const hasChanges = status === 'ready' && signatureOf(rows) !== baseline;
  const hasChangesRef = useRef(false);
  hasChangesRef.current = hasChanges;

  // The iOS swipe is off while changes are unsaved; the guard below catches every other way out.
  useEffect(() => {
    navigation.setOptions({ gestureEnabled: !hasChanges });
  }, [navigation, hasChanges]);

  useEffect(() => {
    return navigation.addListener('beforeRemove', (event) => {
      if (allowLeaveRef.current || !hasChangesRef.current) return;
      event.preventDefault();
      pendingActionRef.current = event.data.action;
      // A pressed Spin button must not start a spin after this sheet (UI-SPEC and plan 02-02).
      clearPendingSpin(userId);
      setLeaveOpen(true);
    });
  }, [navigation, userId]);

  function confirmLeave() {
    allowLeaveRef.current = true;
    setLeaveOpen(false);
    const action = pendingActionRef.current;
    pendingActionRef.current = null;
    if (action) navigation.dispatch(action);
    else router.back();
  }

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }

  // ---- row validation -----------------------------------------------------------------------

  const trimmedNames = rows.map((row) => row.name.trim());
  const lowerNames = trimmedNames.map((name) => name.toLowerCase());
  function problemFor(index: number): string | null {
    const name = trimmedNames[index];
    if (name.length === 0) return NAME_EMPTY_MESSAGE;
    if (lowerNames.indexOf(lowerNames[index]) !== index || lowerNames.lastIndexOf(lowerNames[index]) !== index) {
      return NAME_TAKEN_MESSAGE;
    }
    return null;
  }
  const rowProblems = rows.map((_, index) => problemFor(index));
  const countOk = rows.length >= limits.min && rows.length <= limits.max;
  const allValid = rowProblems.every((problem) => problem === null) && countOk;
  const canSave = hasChanges && allValid && !saving;

  // ---- edits ---------------------------------------------------------------------------------

  function updateRow(key: string, patch: Partial<Row>) {
    setSaveMessage(null);
    setRows((current) => current.map((row) => (row.key === key ? { ...row, ...patch } : row)));
  }

  function removeRow(key: string) {
    setSaveMessage(null);
    setRows((current) => (current.length <= limits.min ? current : current.filter((row) => row.key !== key)));
  }

  function addRow() {
    if (rows.length >= limits.max) return;
    setSaveMessage(null);
    const key = nextRowKey();
    setRows((current) => [...current, { key, id: null, name: '', touched: false }]);
    setFocusKey(key);
  }

  async function save() {
    if (!canSave || savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    setSaveMessage(null);
    try {
      const list = rows.map((row) => ({ id: row.id, name: row.name.trim(), weight: EQUAL_WEIGHT }));
      await saveWheel(version, list);
      // Saved: nothing is left to lose, then back to Today, which reloads its wheel.
      allowLeaveRef.current = true;
      goBack();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'network_unavailable') {
        setSaveMessage(NETWORK_MESSAGE);
      } else if (err instanceof ApiError && err.code === 'wheel_stale') {
        // The wheel changed somewhere else: show what the server has now, with the generic message.
        setSaveMessage(GENERIC_MESSAGE);
        try {
          applyServerWheel(await getWheel());
        } catch {
          // The generic message already says to try again.
        }
      } else {
        setSaveMessage(GENERIC_MESSAGE);
      }
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }

  // ---- drawing -------------------------------------------------------------------------------

  const previewEntries = useMemo(
    () => rows.map((row) => ({ name: row.name.trim(), weight: EQUAL_WEIGHT })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [signatureOf(rows)]
  );
  const previewSize = Math.min(PREVIEW_SIZE, width - 2 * space.lg);
  const atMin = rows.length <= limits.min;
  const atMax = rows.length >= limits.max;

  return (
    <Screen scroll={false} edges={['top', 'left', 'right']}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
        <ScrollView
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}
          contentContainerStyle={{ paddingTop: space.sm, paddingBottom: space.lg }}
        >
          <Pressable
            onPress={goBack}
            accessibilityRole="button"
            accessibilityLabel="Back"
            hitSlop={12}
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              alignSelf: 'flex-start',
              minHeight: minTouchTarget,
            }}
          >
            <Ionicons name="chevron-back" size={22} color={color.ink} />
            <AppText role="body">Back</AppText>
          </Pressable>

          <View style={{ marginTop: space.sm }}>
            <AppText role="heading" accessibilityRole="header">
              Edit wheel
            </AppText>
          </View>
          {spunToday ? (
            <View style={{ marginTop: space.xs }}>
              <AppText role="label" tone="muted">
                Changes show on your next spin.
              </AppText>
            </View>
          ) : null}

          {status === 'loading' ? (
            <View style={{ marginTop: space.xl, alignItems: 'center' }}>
              <AppText role="label" tone="muted">
                Loading your wheel.
              </AppText>
            </View>
          ) : status === 'error' ? (
            <View style={{ marginTop: space.xl }}>
              <Text style={{ ...labelText, color: color.ink, textAlign: 'center' }}>{loadMessage}</Text>
              <View style={{ marginTop: space.md }}>
                <PrimaryButton label="Try again" onPress={() => void load()} />
              </View>
            </View>
          ) : (
            <>
              <View style={{ marginTop: space.lg, alignItems: 'center' }}>
                <Wheel
                  entries={previewEntries}
                  spinId={0}
                  winnerIndex={null}
                  size={previewSize}
                  interactive={false}
                  showPointer={false}
                  decorative
                />
              </View>

              <View style={{ marginTop: space.lg }}>
                {rows.map((row, index) => {
                  const problem = row.touched ? rowProblems[index] : null;
                  const shownName = row.name.trim().length > 0 ? row.name.trim() : 'new category';
                  return (
                    <View
                      key={row.key}
                      style={{
                        minHeight: ROW_MIN_HEIGHT,
                        paddingVertical: space.sm,
                        borderTopWidth: 1,
                        borderTopColor: color.divider,
                      }}
                    >
                      <View style={{ flexDirection: 'row', alignItems: 'center' }}>
                        <NameField
                          value={row.name}
                          maxLength={limits.name_max}
                          autoFocus={focusKey === row.key}
                          onChangeText={(text) => updateRow(row.key, { name: text })}
                          onBlur={() => updateRow(row.key, { touched: true })}
                          hasError={problem !== null}
                        />
                        <Pressable
                          onPress={() => removeRow(row.key)}
                          disabled={atMin}
                          accessibilityRole="button"
                          accessibilityLabel={`Remove ${shownName}`}
                          accessibilityState={{ disabled: atMin }}
                          hitSlop={8}
                          style={{ minHeight: minTouchTarget, justifyContent: 'center', paddingLeft: space.md }}
                        >
                          <Text style={{ ...labelText, color: atMin ? color.muted : color.ink }}>Remove</Text>
                        </Pressable>
                      </View>
                      {problem ? (
                        <View style={{ marginTop: space.xs }} accessibilityLiveRegion="polite">
                          <Text style={{ ...labelText, color: color.ink }}>{problem}</Text>
                        </View>
                      ) : null}
                    </View>
                  );
                })}
                <View style={{ borderTopWidth: 1, borderTopColor: color.divider }} />
              </View>

              {atMin ? (
                <View style={{ marginTop: space.sm }}>
                  <AppText role="label" tone="muted">
                    A wheel needs at least 2 categories.
                  </AppText>
                </View>
              ) : null}

              <View style={{ marginTop: space.sm }}>
                {atMax ? (
                  <AppText role="label" tone="muted">
                    You can have up to 12 categories.
                  </AppText>
                ) : (
                  <Pressable
                    onPress={addRow}
                    accessibilityRole="button"
                    hitSlop={8}
                    style={{
                      flexDirection: 'row',
                      alignItems: 'center',
                      alignSelf: 'flex-start',
                      minHeight: minTouchTarget,
                    }}
                  >
                    <Ionicons name="add" size={20} color={color.ink} />
                    <View style={{ width: space.xs }} />
                    <Text
                      style={{
                        fontFamily: type.button.fontFamily,
                        fontSize: type.button.fontSize,
                        lineHeight: type.button.lineHeight,
                        color: color.ink,
                      }}
                    >
                      Add a category
                    </Text>
                  </Pressable>
                )}
              </View>

              <View style={{ marginTop: space.md }}>
                <AppText role="label" tone="muted">
                  Removed categories stay in your past spins.
                </AppText>
              </View>
            </>
          )}
        </ScrollView>

        {status === 'ready' ? (
          <View style={{ paddingTop: space.sm, paddingBottom: space.md, backgroundColor: color.dominant }}>
            {saveMessage ? (
              <View style={{ marginBottom: space.sm }} accessibilityLiveRegion="polite">
                <Text style={{ ...labelText, color: color.ink, textAlign: 'center' }}>{saveMessage}</Text>
              </View>
            ) : null}
            <PrimaryButton label="Save wheel" onPress={() => void save()} disabled={!canSave} loading={saving} />
          </View>
        ) : null}
      </KeyboardAvoidingView>

      <ConfirmSheet
        visible={leaveOpen}
        title="Leave without saving?"
        body="Your changes will be lost."
        confirmLabel="Leave"
        dismissLabel="Keep editing"
        onConfirm={confirmLeave}
        onDismiss={() => {
          pendingActionRef.current = null;
          setLeaveOpen(false);
        }}
      />
    </Screen>
  );
}

function NameField({
  value,
  maxLength,
  autoFocus,
  onChangeText,
  onBlur,
  hasError,
}: {
  value: string;
  maxLength: number;
  autoFocus: boolean;
  onChangeText: (text: string) => void;
  onBlur: () => void;
  hasError: boolean;
}) {
  const [focused, setFocused] = useState(false);
  return (
    <TextInput
      value={value}
      onChangeText={onChangeText}
      onFocus={() => setFocused(true)}
      onBlur={() => {
        setFocused(false);
        onBlur();
      }}
      maxLength={maxLength}
      autoFocus={autoFocus}
      autoCapitalize="sentences"
      autoCorrect={false}
      returnKeyType="done"
      accessibilityLabel="Category name"
      style={{
        flex: 1,
        height: NAME_FIELD_HEIGHT,
        borderRadius: radius.md,
        borderWidth: focused || hasError ? 1.5 : 1,
        borderColor: hasError ? color.ink : focused ? color.ink : color.inkRest,
        paddingHorizontal: 12,
        fontFamily: type.body.fontFamily,
        fontSize: type.body.fontSize,
        color: color.ink,
      }}
    />
  );
}
