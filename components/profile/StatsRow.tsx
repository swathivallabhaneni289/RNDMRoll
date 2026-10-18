import { Fragment, ReactNode } from 'react';
import { Pressable, Text, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { followersSpoken, followingSpoken, spinsSpoken, streakSpoken } from '@/components/profile/summaryText';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/**
 * The four figures of the Profile page (04-UI-SPEC 4.2): Spins, Week streak, Followers and
 * Following (developer, 2026-10-17, like Instagram). A dark rule above, four equal columns with a
 * faint vertical rule between them, a faint rule below. Each figure is printed as the server sent
 * it. One spoken element per column. Followers and Following are buttons to that list on People
 * when their handler is given; they look the same as the others (a figure is not styled as a link
 * in the developer's picture).
 *
 * The figure is the light serif (`note`, as the developer's picture draws it) and keeps its size
 * with the phone's text size, shrinking only to stay on one line.
 */
export function StatsRow({
  spins,
  weekStreak,
  followers,
  following,
  onOpenFollowers,
  onOpenFollowing,
  posts,
  theirs = false,
}: {
  spins: number;
  weekStreak: number;
  followers: number;
  following: number;
  onOpenFollowers?: () => void;
  onOpenFollowing?: () => void;
  /** Another person's page: the first figure is their Posts in place of Spins. */
  posts?: number;
  /** Another person's page: the hints say "their" in place of "your". */
  theirs?: boolean;
}) {
  const first =
    posts !== undefined
      ? { label: 'Posts', figure: posts, spoken: posts === 1 ? '1 post' : `${posts} posts` }
      : { label: 'Spins', figure: spins, spoken: spinsSpoken(spins) };
  const items: { label: string; figure: number; spoken: string; hint?: string; onPress?: () => void }[] = [
    first,
    { label: 'Week streak', figure: weekStreak, spoken: streakSpoken(weekStreak) },
    { label: 'Followers', figure: followers, spoken: followersSpoken(followers), hint: theirs ? 'Opens their followers.' : 'Opens your followers.', onPress: onOpenFollowers },
    { label: 'Following', figure: following, spoken: followingSpoken(following), hint: theirs ? 'Opens the people they follow.' : 'Opens the people you follow.', onPress: onOpenFollowing },
  ];

  function column(item: (typeof items)[number]): ReactNode {
    const content = (
      <>
        <AppText role="note" numberOfLines={1} adjustsFontSizeToFit minimumFontScale={0.5} allowFontScaling={false}>
          {String(item.figure)}
        </AppText>
        <Text
          numberOfLines={1}
          adjustsFontSizeToFit
          minimumFontScale={0.8}
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
          accessibilityHint={item.hint}
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
