import { Fragment, ReactNode } from 'react';
import { Pressable, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { followersSpoken, followingSpoken, spinsSpoken, streakSpoken } from '@/components/profile/summaryText';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

/**
 * The four figures of the Profile page (04-UI-SPEC 4.2): Spins, Streak, Followers and
 * Following (developer, 2026-10-17, like Instagram). A dark rule above, four equal columns with a
 * faint vertical rule between them, a faint rule below. Each figure is printed as the server sent
 * it. One spoken element per column. Followers and Following are buttons to that list on People
 * when their handler is given; they look the same as the others (a figure is not styled as a link
 * in the developer's picture).
 *
 * The figure is the light serif (`note`, as the developer's picture draws it) and keeps its size
 * with the phone's text size, shrinking only to stay on one line.
 *
 * The word under it is the Label role, muted, with no letter spacing, so even 'Followers' keeps 8
 * points clear of the rules on the narrowest phone. 'Week streak' did not fit its column and is
 * shown as 'Streak' (developer, 2026-10-27); it is still spoken 'Week streak, 4'. Each column keeps
 * 4 points of padding at both sides, so a word that Larger Text shrinks to fit never touches a rule.
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
    { label: 'Streak', figure: weekStreak, spoken: streakSpoken(weekStreak) },
    { label: 'Followers', figure: followers, spoken: followersSpoken(followers), hint: theirs ? 'Opens their followers.' : 'Opens your followers.', onPress: onOpenFollowers },
    { label: 'Following', figure: following, spoken: followingSpoken(following), hint: theirs ? 'Opens the people they follow.' : 'Opens the people you follow.', onPress: onOpenFollowing },
  ];

  function column(item: (typeof items)[number]): ReactNode {
    const content = (
      <>
        <AppText role="note" numberOfLines={1} adjustsFontSizeToFit minimumFontScale={0.5} allowFontScaling={false}>
          {String(item.figure)}
        </AppText>
        {/* Pulled 4 points up, so the figure and its word read as one, as in the developer's picture. */}
        <View style={{ marginTop: -space.xs }}>
          <AppText role="label" tone="muted" numberOfLines={1} adjustsFontSizeToFit minimumFontScale={0.8}>
            {item.label}
          </AppText>
        </View>
      </>
    );
    if (item.onPress) {
      return (
        <Pressable
          onPress={item.onPress}
          accessibilityRole="button"
          accessibilityLabel={item.spoken}
          accessibilityHint={item.hint}
          style={{ flex: 1, alignItems: 'center', minHeight: minTouchTarget, justifyContent: 'center', paddingHorizontal: space.xs }}
        >
          {content}
        </Pressable>
      );
    }
    return (
      <View accessible accessibilityLabel={item.spoken} style={{ flex: 1, alignItems: 'center', paddingHorizontal: space.xs }}>
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
