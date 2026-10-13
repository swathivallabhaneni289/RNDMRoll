import { Fragment } from 'react';
import { Text, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { friendsSpoken, spinsSpoken, streakSpoken } from '@/components/profile/summaryText';
import { color, space, type } from '@/lib/theme/tokens';

/**
 * The three figures of the Profile page (04-UI-SPEC 4.2): Spins, Week streak, Friends. A dark
 * rule above, three equal columns with a faint vertical rule between them, a faint rule below.
 * Each figure is printed as the server sent it. Nothing here is a button (Friends stays plain text
 * until Phase 5 gives it a screen to open). One spoken element per column.
 *
 * The figure is the light serif (`note`, as the developer's picture draws it) and keeps its size
 * with the phone's text size, shrinking only to stay on one line.
 */
export function StatsRow({ spins, weekStreak, friends }: { spins: number; weekStreak: number; friends: number }) {
  const items = [
    { label: 'Spins', figure: spins, spoken: spinsSpoken(spins) },
    { label: 'Week streak', figure: weekStreak, spoken: streakSpoken(weekStreak) },
    { label: 'Friends', figure: friends, spoken: friendsSpoken(friends) },
  ];

  return (
    <View>
      <View style={{ height: 1, backgroundColor: color.ink }} />
      <View style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.md }}>
        {items.map((item, index) => (
          <Fragment key={item.label}>
            {index > 0 ? <View style={{ width: 1, height: 40, backgroundColor: color.divider }} /> : null}
            <View accessible accessibilityLabel={item.spoken} style={{ flex: 1, alignItems: 'center' }}>
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
            </View>
          </Fragment>
        ))}
      </View>
      <View style={{ height: 1, backgroundColor: color.divider }} />
    </View>
  );
}
