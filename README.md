# RNDMRoll

**A daily ritual for a small group of friends.** Once a day you spin a wheel. It lands on a category, like a meal, a movie or a book, and you post one real thing you had, watched or read for it. Your friends' posts open once you've posted yours.

I took this from an idea to a working iPhone app by directing Claude, Anthropic's AI coding assistant. The idea, the product decisions, the designs and the testing are mine; Claude wrote the code to my prompts. The app is React Native with Expo, backed by a Go API and PostgreSQL.

<!-- Screenshots and a short screen recording go here. -->

## The problem

People enjoy keeping a record of their taste. Letterboxd does it for films and Beli for food. People also enjoy a small daily prompt they share with friends, which is what made BeReal work. But the two don't meet:

- Logging apps wait for you to remember to log, so most logs go quiet after a few weeks.
- Daily photo apps give you a reason to open them every day, but leave nothing lasting behind. Yesterday's photo is gone.

**The bet:** a random daily pick gives you a reason to show up, and a structured post (title, rating, photo) turns each day into a lasting record of what you like. Locking friends' posts until you've posted keeps everyone contributing instead of scrolling.

## Who it's for

A small circle of friends who want a light daily habit and a shared record of what they're into. Version 1 is built for a friend group using it every day, not for growth. A habit that a handful of people keep is the thing to prove first.

## Research

Before building, I compared about a dozen apps: BeReal, Letterboxd, Beli, Daylio, PI.FYI, Rated, Memorizer, pov, The Dice Challenge and others. Every single piece of this idea already exists somewhere. Two gaps were open:

1. A random pick that produces a **lasting, rated, photographed entry**. Existing randomizers pick something and forget it.
2. A friend feed **locked until you post**, applied to a taste diary instead of a selfie.

The research also named the risks I designed around: a locked feed is empty without friends (cold start); a heavy post form fights the lock that rewards it; and a random category won't always fit your day. The full write-up is in [`RESEARCH.md`](./RESEARCH.md).

## Key decisions

| Decision | What I chose | Why | What it costs |
|---|---|---|---|
| How the category is picked | A spin wheel you can edit, not a dice roll | A wheel shows every option before it lands, and people can rename, add and resize slices. A die tops out around six faces. | A wheel can read like a prize wheel, so it's kept flat and black and white. |
| How often | Once a day, from 8 PM in your own time zone | Scarcity keeps it a ritual (Wordle's one puzzle a day is the model). Your own time zone means nobody spins at 3 AM to keep up with friends abroad. | Less of BeReal's "everyone at once" surprise. |
| When the category doesn't fit | A free skip, and a weekly goal of 4 posted days instead of a daily streak | "Book" on a day you read nothing shouldn't break a streak. Four days out of seven earns a bonus spin. | Weaker pressure than a strict daily streak. |
| Photo source | Camera or photo library | You can log the book you finished two nights ago without staging a photo. | No guarantee the photo is your own. Accepted for now. |
| Late posts | A post after midnight still counts until the next spin opens, marked late | Life happens. A late post still opens the Board. | The day boundary is softer. |
| Who sees posts | Every account is private, and following is by request | Small trusted circles, safer for younger users (13 and over can join), and less to moderate. | Slower to find people. Find your friends, invite links and suggestions help. |
| Signing in | Email and password, then log in with email or username | Enough for a friend group, and it dropped two native SDKs and their setup. | No one-tap sign-in. |

## What I left out on purpose

- **Unlimited re-spins.** They would make the daily pick meaningless. The only extra spin is earned with the weekly goal.
- **Live-camera-only posts.** These would force people to fake or skip anything that happened earlier in the day.
- **Music attached to posts.** People post a screenshot of the song as a photo instead, which needs no music service.
- **A wider backlog, kept for later.** A year-end recap, taste insights ("you rate meals higher on weekends"), a shareable result card and small groups with their own wheel. Version 1 stays on the core loop: spin, post, see friends.

## How I'd measure it

The app isn't released, so there's no usage data yet. These are the numbers I'd watch with a first group:

- **Spin to post:** of the people who spin, how many post that day. This is the core loop.
- **Weekly goal:** the share of people who post on 4 or more days in a week.
- **Retention:** who is still spinning in week 2 and week 4.
- **Skips by category:** a category that gets skipped a lot is a bad fit for the starting wheel.
- **Board pull:** how often someone opens the Board right after posting. This shows whether the lock is a reason to post or a wall.
- **Follow requests:** how many get accepted, and how many people each person follows. This is the cold-start check.

## What the app does

- **Spin.** The wheel opens at 8 PM. A new person gets their first spin right away. The starting wheel is Meal, Movie, Song, Book, Place and Game, all with the same chance, and you can change it (2 to 12 categories).
- **Post.** Title, a half-star rating from 0.5 to 5, photos, an optional one-tap reaction and a short line. Edit, delete or archive it later.
- **Board.** Posts from the people you follow, locked until you've posted that day. Comments, replies and photos in comments.
- **Profile.** Every post you've made, your spins, your week streak and your followers.
- **People.** Search, follow requests with Accept and Decline, invite links and suggestions.
- **Safety.** Block, mute, and report a person, post or comment. Reports go to an admin web page where an admin can remove content or disable an account.
- **Notifications.** A list in the app, an 8 PM reminder on the phone, and phone notifications for comments, replies and follow requests.
- **Accounts.** Two short sign-up pages, and Forgot password with a 6-digit emailed code.

## How I built it with Claude

I built RNDMRoll by prompting Claude Code. I decided what the app should do and how it should look, and Claude wrote the code to those instructions.

**My part**

- **The idea and the research.** I came up with the concept, asked for the competitor research in [`RESEARCH.md`](./RESEARCH.md), and chose what to keep and what to cut.
- **The product calls.** The decisions in the table above are mine, and so is the list of what I left out.
- **The look.** I shared my own mockups and reference pictures, picked the fonts and the black and white style, and chose the logo from options I asked for.
- **The plan.** The work ran in phases, and I approved each phase's scope and design before anything was built.
- **The testing.** I walked through each phase on the iPhone simulator and sent back what to change, often as screenshots and voice notes, until it worked the way I wanted.
- **The quality bar.** I asked for code reviews and had every problem fixed and checked again before pushing.

**Claude's part**

- Writing the Go server, the database migrations, the app screens and the tests, following those plans.
- Running the reviews and checks I asked for.

Each phase had a written scope, a screen-by-screen design spec, a build plan and a test checklist. They're in [`.planning/`](./.planning), the record of what I asked for and approved.

1. Accounts and sign-up
2. The daily spin, then Settings and smoother navigation
3. The daily post
4. Profile and the post history
5. Following and the Board
6. People pages, reporting and the admin page

| Part | Tools |
|---|---|
| App | TypeScript, React Native 0.86, Expo 57, Expo Router, Reanimated |
| Server | Go 1.26, Gin, PostgreSQL through pgx, 15 SQL migrations |
| Photos | S3-compatible storage with presigned uploads (MinIO when running locally) |
| Sign-in | bcrypt password hashes, short-lived JWT access tokens and refresh tokens, rate limits |
| Email and push | Resend for email, the Expo push service for phone notifications |

A few things that took real care:

- **Time zones.** Every person's day, 8 PM opening and late window follow their own zone, including the days clocks jump for daylight saving (`internal/spin/window.go`).
- **One source of time.** The server writes every time label ("8:41 PM", "Mon", "Oct 3") so the app never guesses.
- **Tests.** About 450 Go test functions, most of them running against a real PostgreSQL database.

<details>
<summary><b>Run it locally</b></summary>

You need macOS with Xcode and the iOS Simulator, Node, Go 1.26, PostgreSQL, [golang-migrate](https://github.com/golang-migrate/migrate) and [MinIO](https://min.io) for photos.

1. **Settings.** Copy `.env.example` to `.env`. Set `DATABASE_URL`, `TEST_DATABASE_URL` and a long random `JWT_SECRET`. Keep `MAIL_DRIVER=log`, which prints emails (such as Forgot password codes) to the server log instead of sending them.
2. **Photo storage.** Start MinIO, create a bucket with public read access, and point the `S3_*` lines in `.env` at it (for example `S3_ENDPOINT=http://127.0.0.1:9000`, `S3_REGION=us-east-1`, `S3_PUBLIC_BASE_URL=http://127.0.0.1:9000/<bucket>`).
3. **Database.**
   ```sh
   make db-create
   export DATABASE_URL=postgres://localhost:5432/rndmroll_dev?sslmode=disable
   make migrate-up
   ```
4. **Server.** `make dev` starts the API on port 8080.
5. **App.** `npm install`, then `npx expo run:ios`. This builds the app into the simulator. It needs a native build, so Expo Go won't work. The app looks for the API at `http://localhost:8080`, or at `EXPO_PUBLIC_API_BASE_URL` if that is set.

Optional:

- **Admin page.** Set `ADMIN_EMAILS` to your account's email before starting the server, then open `http://localhost:8080/admin`.
  The admin account must exist before the server starts: sign up with that email while it is not in `ADMIN_EMAILS` (sign-up refuses a listed address), then add it and restart the server.
- **Phone notifications.** These are off until Apple and Expo credentials are added. The steps are in [`docs/PUSH-SETUP.md`](./docs/PUSH-SETUP.md).

Tests:

```sh
make test                  # quick tests, no database needed
make test-all-integration  # everything, against TEST_DATABASE_URL, one package at a time
npm run typecheck          # the app's TypeScript check
```

Where things are:

```
app/          screens (Expo Router)
components/   UI pieces used by the screens
lib/          app logic: API client, session, theme, motion
cmd/api/      the server's entry point
internal/     server packages: spin, board, diary, social, auth, admin, push, storage
migrations/   database migrations
.planning/    the phase-by-phase scope, specs and plans
```

</details>
