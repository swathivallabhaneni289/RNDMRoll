import { Fragment, ReactNode } from 'react';
import { Pressable, Text, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { followersSpoken, spinsSpoken, streakSpoken } from '@/components/profile/summaryText';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/**
 * The three figures of the Profile page (04-UI-SPEC 4.2): Spins, Week streak, Followers. A dark
 * rule above, three equal columns with a faint vertical rule between them, a faint rule below.
 * Each figure is printed as the server sent it. One spoken element per column. Followers (Phase 5,
 * one-way follows) is a button to People when `onOpenFollowers` is given; it looks the same as
 * the other two (a figure is not styled as a link in the developer's picture).
 *
 * The figure is the light serif (`note`, as the developer's picture draws it) and keeps its size
 * with the phone's text size, shrinking only to stay on one line.
 */
export function StatsRow({
  spins,
  weekStreak,
  followers,
  onOpenFollowers,
}: {
  spins: number;
  weekStreak: number;
  followers: number;
  onOpenFollowers?: () => void;
}) {
  const items: { label: string; figure: number; spoken: string; onPress?: () => void }[] = [
    { label: 'Spins', figure: spins, spoken: spinsSpoken(spins) },
    { label: 'Week streak', figure: weekStreak, spoken: streakSpoken(weekStreak) },
    { label: 'Followers', figure: followers, spoken: followersSpoken(followers), onPress: onOpenFollowers },
  ];

  function column(item: (typeof items)[number]): ReactNode {
    const content = (
      <>
        <AppText role="note" numberOfLines={1} adjustsFontSizeToFit minimumFontScale={0.5} allowFontScaling={false}>
          {String(item.figure)}
        </AppText>
        <Text
          numberOfLines={1}
          style={{
            fontFamily: type.label.fontFamily,
            fontSize: type.label.fontSize,
            lineHeight: type.label.lineHeight,
            letterSpacing: 1,
            color: color.muted,
          }}
        >
          {item.label}
        </Text>
      </>
    );
    if (item.onPress) {
      return (
        <Pressable
          onPress={item.onPress}
          accessibilityRole="button"
          accessibilityLabel={item.spoken}
          accessibilityHint="Opens People."
          style={{ flex: 1, alignItems: 'center', minHeight: minTouchTarget, justifyContent: 'center' }}
        >
          {content}
        </Pressable>
      );
    }
    return (
      <View accessible accessibilityLabel={item.spoken} style={{ flex: 1, alignItems: 'center' }}>
        {content}
      </View>
    );
  }

  return (
    <View>
      <View style={{ height: 1, backgroundColor: color.ink }} />
      <View style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.md }}>
        {items.map((item, index) => (
          <Fragment key={item.label}>
            {index > 0 ? <View style={{ width: 1, height: 40, backgroundColor: color.divider }} /> : null}
            {column(item)}
          </Fragment>
        ))}
      </View>
      <View style={{ height: 1, backgroundColor: color.divider }} />
    </View>
  );
}
