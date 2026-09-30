/**
 * Dev-only fixtures for the frosted-material lab. Nothing here is product copy and
 * nothing here ships: the whole lab folder is behind __DEV__. Dates are relative to
 * the current day so the feed always reads as "today".
 */

export type Category = 'movie' | 'book' | 'song' | 'meal' | 'place' | 'game';

export type SceneId = 'window' | 'horizon' | 'table' | 'doorway' | 'night' | 'snow' | 'stairs' | 'tiles';

/** One comment under an entry. Fixture data only: comments are not in the current requirements. */
export type LabComment = {
  id: string;
  /** "You" for the signed-in user, a first name for a friend. */
  author: string;
  text: string;
  /** Clock time, like "8:26 PM". */
  time: string;
};

export type LabEntry = {
  id: string;
  /** Unique position; picks a photo when the developer supplies their own library. */
  slot: number;
  /** "You" for the signed-in user, a first name for a friend. */
  author: string;
  category: Category;
  title: string;
  /** 1 to 5 in half-star steps. */
  rating: number;
  reaction: string;
  /** The daily window opens at 8:00 PM local, so entries carry that clock time. */
  date: Date;
  /** Subtle metadata line for the diary view. */
  rolledAt: string;
  scene: SceneId;
  /** Oldest first, so the most recent comments are at the end. */
  comments: LabComment[];
};

const SCENES: SceneId[] = ['window', 'horizon', 'table', 'doorway', 'night', 'snow', 'stairs', 'tiles'];

function daysAgo(days: number, minutes: number): Date {
  const date = new Date();
  date.setHours(20, minutes, 0, 0);
  date.setDate(date.getDate() - days);
  return date;
}

function rolledLabel(minutes: number): string {
  return `Rolled 8:${minutes < 10 ? '0' : ''}${minutes} PM`;
}

type CommentSeed = [author: string, text: string, time: string];

/**
 * Comments by entry id, oldest first. Friends comment on each other's entries, the
 * post's own author now and then replies (never first), and "You" joins in on a few.
 * Every time is after the entry was rolled.
 */
const COMMENT_SEED: Record<string, CommentSeed[]> = {
  f0: [
    ['Jonas', 'Saw it in IMAX. My ears rang until Tuesday.', '8:14 PM'],
    ['Priya', 'The organ score is doing half the work.', '8:19 PM'],
    ['Maya', 'Worth the ringing, Jonas.', '8:26 PM'],
    ['You', 'Putting this on for Saturday.', '9:02 PM'],
  ],
  f1: [
    ['Priya', 'Cumin is never too much. Send the recipe.', '8:11 PM'],
    ['Amara', 'Were the eggs set or runny? This matters.', '8:17 PM'],
    ['Jonas', "Runny. I'm not a monster.", '8:20 PM'],
  ],
  f2: [
    ['Theo', 'Still hear a hospital drama every time it starts.', '8:16 PM'],
    ['Noor', 'Twice is restrained. I did four on the tram.', '8:33 PM'],
  ],
  f3: [
    ['Amara', 'Wrong how? Asking for the ferns.', '8:22 PM'],
    ['Maya', 'Went in winter for the heat alone. No regrets.', '8:29 PM'],
    ['Theo', 'Amara, a maidenhair was tagged as a spleenwort.', '8:41 PM'],
  ],
  f4: [
    ['Jonas', 'My ceiling is about twenty lines.', '8:12 PM'],
    ['Noor', "You left the well open, didn't you.", '8:24 PM'],
  ],
  f5: [
    ['Priya', 'I bought a used cassette player the next day.', '8:13 PM'],
    ['Theo', 'Komorebi. I looked the word up right after.', '8:29 PM'],
    ['Amara', 'Cheap lunch is easy. The camera is where it goes wrong.', '8:44 PM'],
    ['Noor', 'Priya, cassettes are the cheap way in.', '9:05 PM'],
  ],
  d00: [
    ['Noor', 'He never says what he means. Keep going.', '8:21 PM'],
    ['Maya', 'Wait for the chapters with his father.', '8:34 PM'],
    ['You', 'Slowly, on purpose.', '8:52 PM'],
  ],
  d01: [
    ['Priya', "Apparently you're not meant to drive while it's on.", '8:18 PM'],
    ['Jonas', 'I was asleep by minute five.', '8:36 PM'],
  ],
  d03: [
    ['Maya', 'The last scene on the street. I had to sit down.', '8:40 PM'],
    ['Noor', 'Second watch for me. It gets quieter each time.', '9:12 PM'],
  ],
  d05: [['Theo', 'Four is respectable. Three is showing off.', '8:20 PM']],
  d06: [
    ['Noor', "Hugh Grant knows exactly what film he's in.", '8:37 PM'],
    ['Amara', 'Third person to tell me to watch it. Fine.', '8:52 PM'],
  ],
  d07: [
    ['Maya', "Hallway was the right call. Don't sit down for that part.", '8:31 PM'],
    ['Jonas', 'Can I borrow your copy or do I have to buy one?', '8:47 PM'],
    ['You', 'Buy one. Mine has notes in the margins.', '9:03 PM'],
  ],
  d09: [['Theo', 'Pink + White on a bike ride is a different song.', '8:44 PM']],
  d12: [
    ['Noor', 'The Under Pressure scene ruined me a week ago.', '8:48 PM'],
    ['Amara', 'Do not watch this on a plane. I found that out.', '9:03 PM'],
  ],
  d15: [['Jonas', "Tea, balcony, Neil Young. You're sixty now.", '8:27 PM']],
  d17: [
    ['Priya', 'Rain, bus stop, umbrella. Nothing else has to happen.', '8:19 PM'],
    ['Amara', 'I still say the catbus is the best character.', '8:41 PM'],
  ],
  d18: [
    ['Theo', 'Eleven is nothing. Wait until Path of Pain.', '8:33 PM'],
    ['You', 'Already been. It was not fun.', '8:51 PM'],
  ],
  d21: [['Maya', 'Agreed. She sounds like she means it now.', '8:29 PM']],
};

function commentsFor(entryId: string): LabComment[] {
  return (COMMENT_SEED[entryId] ?? []).map(([author, text, time], index) => ({
    id: `${entryId}-c${index + 1}`,
    author,
    text,
    time,
  }));
}

// [days ago, category, title, rating, reaction]
const DIARY_SEED: Array<[number, Category, string, number, string]> = [
  [0, 'book', 'The Remains of the Day', 4, 'Forty pages tonight. Stevens is exhausting and I like him.'],
  [1, 'song', 'Weightless', 4.5, 'Eight minutes of nothing happening, exactly what I needed.'],
  [2, 'meal', 'Dal and rice', 4, 'Same as always. Better with the good pickle.'],
  [4, 'movie', 'Past Lives', 5, 'Sat through the credits, then washed up without the radio on.'],
  [5, 'place', 'The canal path', 3.5, 'Cold hands, good light, nobody about.'],
  [6, 'game', 'Wordle in four', 3, 'Should have got it in three.'],
  [8, 'movie', 'Paddington 2', 4.5, 'Unfairly good. The pop-up book scene is the whole film.'],
  [9, 'book', 'Piranesi', 4.5, 'Read the last chapter standing in the hallway.'],
  [11, 'meal', 'Street momos', 4, 'Burnt my tongue, no regrets.'],
  [12, 'song', 'Pink + White', 4, 'That bassline on the second verse.'],
  [13, 'place', 'The public library', 3.5, 'Found the one quiet chair on the third floor.'],
  [15, 'game', 'Chess, one blitz', 2.5, 'Hung my queen on move nine.'],
  [16, 'movie', 'Aftersun', 4.5, 'Did not expect to be this quiet afterwards.'],
  [18, 'book', 'Braiding Sweetgrass', 4, 'One chapter a night. The strawberries one twice.'],
  [19, 'meal', 'Lentil soup', 3.5, 'Needed more lemon. Noted.'],
  [20, 'song', 'Harvest Moon', 4.5, 'Neil Young on the balcony with tea.'],
  [22, 'place', 'Rooftop at dusk', 4, 'Pigeons, a water tank, orange light.'],
  [23, 'movie', 'My Neighbor Totoro', 5, 'Third watch. The bus stop scene, again.'],
  [25, 'game', 'Hollow Knight', 4, 'Died to the same boss eleven times and enjoyed ten of them.'],
  [27, 'book', 'Stoner', 4.5, 'Slow and plain, and it got me in the last thirty pages.'],
  [28, 'meal', 'Fried egg sandwich', 3, 'Fine. The egg was the only good part.'],
  [30, 'song', 'Both Sides Now', 5, "Joni's 2000 version, not the young one."],
  [32, 'place', 'Old bookshop', 4, 'Bought nothing, stayed an hour.'],
  [34, 'movie', 'Roman Holiday', 4, 'Better than I remembered.'],
];

export const DIARY: LabEntry[] = DIARY_SEED.map(([days, category, title, rating, reaction], index) => {
  const minutes = 2 + ((index * 7) % 23);
  const id = `d${index < 10 ? '0' : ''}${index}`;
  return {
    id,
    slot: index,
    author: 'You',
    category,
    title,
    rating,
    reaction,
    date: daysAgo(days, minutes),
    rolledAt: rolledLabel(minutes),
    scene: SCENES[(index * 3 + 1) % SCENES.length],
    comments: commentsFor(id),
  };
});

// [author, category, title, rating, reaction]
const FRIEND_SEED: Array<[string, Category, string, number, string]> = [
  ['Maya', 'movie', 'Interstellar', 4.5, 'Watched it with the sound way up. The docking scene still gets me.'],
  ['Jonas', 'meal', 'Shakshuka at home', 4, 'Too much cumin. Would still eat it again tomorrow.'],
  ['Priya', 'song', 'Teardrop', 5, 'Played it twice on the walk home.'],
  ['Theo', 'place', 'Botanical greenhouse', 3.5, 'Warm, damp, and half the ferns are labelled wrong.'],
  ['Amara', 'game', 'Tetris, 40 lines', 3, 'Lost to a single I-piece drought.'],
  ['Noor', 'movie', 'Perfect Days', 5, 'Made me want a small camera and a cheap lunch.'],
];

const FRIENDS: LabEntry[] = FRIEND_SEED.map(([author, category, title, rating, reaction], index) => {
  const minutes = 1 + ((index * 5) % 17);
  const id = `f${index}`;
  return {
    id,
    slot: 100 + index,
    author,
    category,
    title,
    rating,
    reaction,
    date: daysAgo(0, minutes),
    rolledAt: rolledLabel(minutes),
    scene: SCENES[(index * 5 + 2) % SCENES.length],
    comments: commentsFor(id),
  };
});

/** Today's board: friends first, the developer's own entry sits among them, unbadged. */
export const FEED: LabEntry[] = [FRIENDS[0], FRIENDS[1], FRIENDS[2], DIARY[0], FRIENDS[3], FRIENDS[4], FRIENDS[5]];

const BY_ID = new Map<string, LabEntry>([...FRIENDS, ...DIARY].map((entry) => [entry.id, entry]));

export function getEntry(id: string | undefined): LabEntry | undefined {
  return id ? BY_ID.get(id) : undefined;
}

const MONTHS = [
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
const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

export function formatMonth(date: Date): string {
  return `${MONTHS[date.getMonth()]} ${date.getFullYear()}`;
}

/** "Tuesday 29 September" */
export function formatLong(date: Date): string {
  return `${DAYS[date.getDay()]} ${date.getDate()} ${MONTHS[date.getMonth()]}`;
}

/** "8:01 PM" */
export function formatTime(date: Date): string {
  const hours = date.getHours();
  const minutes = date.getMinutes();
  return `${hours % 12 === 0 ? 12 : hours % 12}:${minutes < 10 ? '0' : ''}${minutes} ${hours < 12 ? 'AM' : 'PM'}`;
}

/** "29 Sep" */
export function formatShort(date: Date): string {
  return `${date.getDate()} ${MONTHS[date.getMonth()].slice(0, 3)}`;
}

export function categoryLabel(category: Category): string {
  return category.charAt(0).toUpperCase() + category.slice(1);
}
