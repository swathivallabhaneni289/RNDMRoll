import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { FlatList, LayoutChangeEvent, NativeScrollEvent, NativeSyntheticEvent, useWindowDimensions, View } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import Animated, {
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withDelay,
  withTiming,
} from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { color, space } from '@/lib/theme/tokens';
import { DIARY, LabEntry } from '@/lab/data/entries';
import {
  buildDiaryLayout,
  buildGridLayout,
  DIARY_MARGIN,
  diaryEntryAtOffset,
  diaryOffsetForEntry,
  GridRow,
  gridEntryAtOffset,
  gridOffsetForEntry,
} from '@/lab/data/diaryLayout';
import { HeroClipContext } from '@/lab/motion/hero';
import { duration, easeOut } from '@/lab/motion/timing';
import { grid } from '@/lab/theme/frost';
import { DiaryEntryView } from '@/lab/components/DiaryEntryView';
import { GridRowView } from '@/lab/components/GridView';
import { LabBackLink } from '@/lab/components/LabBackLink';
import { LabText } from '@/lab/components/LabText';
import { ViewMode, ViewSwitcher } from '@/lab/components/ViewSwitcher';

/**
 * Screen B: the personal archive. No glass here on purpose: the diary should feel
 * cleaner and more timeless than the social board. Two ways to look at the same
 * collection, GRID and DIARY, switched by a typographic control. Both lists stay
 * mounted, so the switch is a crossfade with a small rise, and the scroll position
 * carries over: whichever entry sat nearest the top of one view is scrolled to the
 * top of the other before it fades in.
 */
export default function DiaryScreen() {
  const insets = useSafeAreaInsets();
  const { width, fontScale } = useWindowDimensions();
  const reduced = useReducedMotion();
  // Bottom edge of the header: photos scrolled up past it cannot be flown honestly.
  const [headerHeight, setHeaderHeight] = useState(0);

  // Dev aids for recording on a simulator without touch automation:
  //   /lab/diary?mode=diary   start in the diary view
  //   /lab/diary?switch=diary switch views by itself after a moment
  //   /lab/diary?open=d01     open that entry by itself after a moment
  const params = useLocalSearchParams<{ mode?: string; switch?: string; open?: string }>();
  const [initialMode] = useState<ViewMode>(() => (params.mode === 'diary' ? 'diary' : 'grid'));

  const [mode, setMode] = useState<ViewMode>(initialMode);
  const [viewport, setViewport] = useState(0);
  const gridVisible = useSharedValue(initialMode === 'grid' ? 1 : 0);
  const diaryVisible = useSharedValue(initialMode === 'diary' ? 1 : 0);

  const gridList = useRef<FlatList<GridRow>>(null);
  const diaryList = useRef<FlatList<LabEntry>>(null);
  const gridOffset = useRef(0);
  const diaryOffset = useRef(0);

  const gridLayout = useMemo(() => buildGridLayout(DIARY, width), [width]);
  const diaryLayout = useMemo(() => buildDiaryLayout(DIARY.length, width, fontScale), [width, fontScale]);
  const bottomPad = insets.bottom + space.xxl;

  const switchTo = useCallback(
    (next: ViewMode) => {
      if (next === mode) return;

      const clamp = (offset: number, contentHeight: number) =>
        Math.max(0, Math.min(offset, contentHeight + bottomPad - viewport));

      if (next === 'diary') {
        const anchor = gridEntryAtOffset(gridLayout, gridOffset.current);
        diaryList.current?.scrollToOffset({
          offset: clamp(diaryOffsetForEntry(diaryLayout, anchor), diaryLayout.totalHeight),
          animated: false,
        });
      } else {
        const anchor = diaryEntryAtOffset(diaryLayout, diaryOffset.current, DIARY.length);
        gridList.current?.scrollToOffset({
          offset: clamp(gridOffsetForEntry(gridLayout, anchor), gridLayout.totalHeight),
          animated: false,
        });
      }

      // The outgoing view leaves quickly; the incoming one arrives a beat later.
      const scale = reduced ? 0 : 1;
      const outgoing = next === 'diary' ? gridVisible : diaryVisible;
      const incoming = next === 'diary' ? diaryVisible : gridVisible;
      outgoing.value = withTiming(0, { duration: duration.quick * scale, easing: easeOut });
      incoming.value = withDelay(
        70 * scale,
        withTiming(1, { duration: (duration.switch - 70) * scale, easing: easeOut }),
      );
      setMode(next);
    },
    [mode, reduced, viewport, bottomPad, gridLayout, diaryLayout, gridVisible, diaryVisible],
  );

  useEffect(() => {
    const target = params.switch === 'diary' || params.switch === 'grid' ? params.switch : null;
    if (!target) return;
    const timer = setTimeout(() => switchTo(target), 1600);
    return () => clearTimeout(timer);
    // Mount-only dev aid.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const gridLayerStyle = useAnimatedStyle(() => ({
    opacity: gridVisible.value,
    transform: [{ scale: 0.985 + gridVisible.value * 0.015 }],
  }));
  const diaryLayerStyle = useAnimatedStyle(() => ({
    opacity: diaryVisible.value,
    transform: [{ translateY: (1 - diaryVisible.value) * 10 }],
  }));

  function trackGrid(event: NativeSyntheticEvent<NativeScrollEvent>) {
    gridOffset.current = event.nativeEvent.contentOffset.y;
  }
  function trackDiary(event: NativeSyntheticEvent<NativeScrollEvent>) {
    diaryOffset.current = event.nativeEvent.contentOffset.y;
  }
  function measureViewport(event: LayoutChangeEvent) {
    setViewport(event.nativeEvent.layout.height);
  }

  return (
    <HeroClipContext.Provider value={headerHeight}>
    <View style={{ flex: 1, backgroundColor: color.dominant }}>
      <View
        onLayout={(event) => setHeaderHeight(event.nativeEvent.layout.height)}
        style={{ paddingTop: insets.top + space.sm, paddingHorizontal: space.lg, paddingBottom: space.sm }}
      >
        <LabBackLink />
        <LabText role="display" style={{ marginTop: space.lg }}>
          Diary
        </LabText>
        <View style={{ marginTop: space.xs }}>
          <ViewSwitcher value={mode} onChange={switchTo} />
        </View>
      </View>

      <View style={{ flex: 1 }} onLayout={measureViewport}>
        <Animated.View
          accessibilityElementsHidden={mode !== 'grid'}
          importantForAccessibility={mode === 'grid' ? 'auto' : 'no-hide-descendants'}
          style={[
            { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, pointerEvents: mode === 'grid' ? 'auto' : 'none' },
            gridLayerStyle,
          ]}
        >
          <FlatList
            ref={gridList}
            data={gridLayout.rows}
            keyExtractor={(row) => row.key}
            renderItem={({ item }) => (
              <GridRowView
                row={item}
                tileWidth={gridLayout.tileWidth}
                tileHeight={gridLayout.tileHeight}
                autoOpenId={mode === 'grid' ? params.open : undefined}
              />
            )}
            getItemLayout={(_, index) => ({
              length: gridLayout.rows[index].height,
              offset: gridLayout.rows[index].offset,
              index,
            })}
            onScroll={trackGrid}
            scrollEventThrottle={16}
            initialNumToRender={6}
            maxToRenderPerBatch={4}
            windowSize={5}
            showsVerticalScrollIndicator={false}
            contentContainerStyle={{ paddingHorizontal: grid.margin, paddingBottom: bottomPad }}
          />
        </Animated.View>

        <Animated.View
          accessibilityElementsHidden={mode !== 'diary'}
          importantForAccessibility={mode === 'diary' ? 'auto' : 'no-hide-descendants'}
          style={[
            { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, pointerEvents: mode === 'diary' ? 'auto' : 'none' },
            diaryLayerStyle,
          ]}
        >
          <FlatList
            ref={diaryList}
            data={DIARY}
            keyExtractor={(entry) => entry.id}
            renderItem={({ item }) => (
              <DiaryEntryView entry={item} layout={diaryLayout} autoOpen={mode === 'diary' && params.open === item.id} />
            )}
            getItemLayout={(_, index) => ({
              length: diaryLayout.itemHeight,
              offset: diaryLayout.itemHeight * index,
              index,
            })}
            onScroll={trackDiary}
            scrollEventThrottle={16}
            initialNumToRender={3}
            windowSize={5}
            showsVerticalScrollIndicator={false}
            contentContainerStyle={{ paddingHorizontal: DIARY_MARGIN, paddingBottom: bottomPad }}
          />
        </Animated.View>
      </View>
    </View>
    </HeroClipContext.Provider>
  );
}
