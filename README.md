# RNDMRoll

**A daily ritual for a small group of friends.** Once a day you spin a wheel. It lands on a category, like a meal, a movie or a book, and you post one real thing you had, watched or read for it. Your friends' posts open once you've posted yours.

I took this from an idea to a working iPhone app by directing Claude, Anthropic's AI coding assistant. The idea, the product decisions, the designs and the testing are mine; Claude wrote the code to my prompts. The app is React Native with Expo, backed by a Go API and PostgreSQL.

**Documents:** the [product requirements document](./PRD.md) (every requirement, rule, metric and decision in detail), the [research](./RESEARCH.md), the [Terms of Use](./docs/legal/TERMS.md) and the [Privacy Policy](./docs/legal/PRIVACY.md).

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
| How the category is picked | A spin wheel you can edit, not a dice roll | A wheel shows every option before it lands, and people can rename, add and remove slices. A die tops out around six faces. | A wheel can read like a prize wheel, so it's kept flat and black and white. |
| How often | Once a day, from 8 PM in your own time zone | Scarcity keeps it a ritual (Wordle's one puzzle a day is the model). Your own time zone means nobody spins at 3 AM to keep up with friends abroad. | Less of BeReal's "everyone at once" surprise. |
| When the category doesn't fit | A free skip, and a weekly goal of 4 posted days instead of a daily streak | "Book" on a day you read nothing shouldn't break a streak. Four days out of seven earns a bonus spin. | Weaker pressure than a strict daily streak. |
| Photo source | Camera or photo library | You can log the book you finished two nights ago without staging a photo. | No guarantee the photo is your own. Accepted for now. |
| Late posts | A post is on time until noon the next day, then still counts until the next spin opens, marked late | Life happens, and four hours to post on time were too few. A late post still opens the Board. | The day boundary is softer, and so is the nudge to post the same night. |
| Who sees posts | Every account is private, and following is by request | Small trusted circles, safer for younger users (13 and over can join), and less to moderate. | Slower to find people. Find your friends, invite links and suggestions help. |
| Signing in | Email and password, then log in with email or username | Enough for a friend group, and it dropped two native SDKs and their setup. | No one-tap sign-in. |

## What I left out on purpose

- **Unlimited re-spins.** They would make the daily pick meaningless. The only extra spin is earned with the weekly goal.
- **Live-camera-only posts.** These would force people to fake or skip anything that happened earlier in the day.
- **Music attached to posts.** People post a screenshot of the song as a photo instead, which needs no music service.
- **Apple and Google sign-in, and reactions on friends' posts.** Email and password are enough for a friend group, and reactions repeated what comments do.
- **A wider backlog, kept for later.** A year-end recap, taste insights ("you rate meals higher on weekends"), a shareable result card and small groups with their own wheel. Version 1 stays on the core loop: spin, post, see friends.

These wait for a real launch:

- **An email check at sign-up.** Sign-up asks for no code, so the app doesn't know that an email address really belongs to the person.
- **Real email.** It's built and off, so a Forgot password code prints to the server log instead of arriving by email.
- **Phone pushes.** Built and off until there are Apple and Expo accounts. The 8 PM reminder and the 10 AM note are made on the phone itself, so they work today.
- **Hosting.** The server runs on my Mac for now. The database and photos are on Supabase's free plan in Singapore.
- **The App Store.** The app runs in the iOS Simulator. Invite links open a web page with an "Open in RNDMRoll" button, because opening the app straight from a link needs a domain.
- **Legal pages.** The Terms of Use and the Privacy Policy are drafts for now.

The full list, down to photo metadata and server alerts, is in the PRD's [launch checklist](./PRD.md#launch-checklist) and [known issues](./PRD.md#known-issues).

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
- **Safety.** Block, mute, and report a person, post or comment. A post or comment you report is gone for you, and one that 3 people report is hidden until an admin looks. Reports go to an admin web page, one card per reported thing, and the admin gets an email. The admin can remove content, kept 30 days in case it should come back, or disable an account. The owner is told in the app and can email for a second look.
- **Notifications.** A list in the app, an 8 PM reminder on the phone and a 10 AM note when a spin isn't posted yet, and phone notifications for comments, replies and follow requests (built, off for now).
- **Accounts.** Two short sign-up pages, Forgot password with a 6-digit emailed code (checked before you pick a new password; email is built but off for now, so the code prints to the server log), the Terms of Use and Privacy Policy, and Delete account in Settings: like Instagram, the account is hidden at once and deleted for good after 30 days unless you log back in.

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

Phases 1, 2, 2.1, 3, 4 and 5 each had a written scope, a screen-by-screen design spec, a build plan and a test checklist, and Phase 2.2 had a scope and a plan. Phase 6's paper covers the person page, reports and the 8 PM reminder; the rest of Phase 6 was built from my instructions without separate paper. These papers are in [`.planning/`](./.planning), the record of what I asked for and approved along the way. The [PRD](./PRD.md) is the record of what the app does today.

1. Accounts and sign-up
2. The daily spin, then Settings and smoother navigation
3. The daily post
4. Profile and the post history
5. Following and the Board
6. People pages, reporting and the admin page

| Part | Tools |
|---|---|
| App | TypeScript, React Native 0.86, Expo 57, Expo Router, Reanimated |
| Server | Go 1.26, Gin, PostgreSQL on Supabase (Singapore) through pgx and the session pooler, 20 SQL migrations |
| Photos | A private Supabase Storage bucket through its S3 API: signed uploads, and signed photo links that stop working within an hour, made only for people allowed to see the photo (a private MinIO bucket when running locally) |
| Sign-in | bcrypt password hashes, short-lived JWT access tokens and refresh tokens, rate limits |
| Email and push | Built for Resend (email) and the Expo push service (phone notifications), and switched off for now: Forgot password codes print to the server log, and no phone pushes are sent |

A few things that took real care:

- **Time zones.** Every person's day, 8 PM opening and late window follow their own zone, including the days clocks jump for daylight saving (`internal/spin/window.go`).
- **One source of time.** The server writes every time label ("8:41 PM", "Mon", "Oct 3") so the app never guesses.
- **Tests.** More than 700 Go test functions, and more than 180 of them run against a real PostgreSQL database. Together they run about 85% of the server's code. Twelve check scripts cover the app's logic and the legal text.

<details>
<summary><b>Run it locally</b></summary>

You need macOS with Xcode and the iOS Simulator, Node, Go 1.26, PostgreSQL, [golang-migrate](https://github.com/golang-migrate/migrate) and [MinIO](https://min.io) for photos.

1. **Settings.** Copy `.env.example` to `.env`. Set `DATABASE_URL`, `TEST_DATABASE_URL` and a long random `JWT_SECRET`. Keep `MAIL_DRIVER=log`, which prints emails (such as Forgot password codes) to the server log instead of sending them, and `APP_ENV=test`, which lets the admin page's login cookie work on plain `http://localhost`.
2. **Photo storage.** Start MinIO, create a private bucket (no public read access), and point the `S3_*` lines in `.env` at it (for example `S3_ENDPOINT=http://127.0.0.1:9000`, `S3_REGION=us-east-1`). For Supabase, turn on the S3 connection in Storage settings, make a private bucket, and use the endpoint, region and access keys from that page. Keep the keys on the server only.
3. **Database.**
   ```sh
   make db-create
   export DATABASE_URL=postgres://localhost:5432/rndmroll_dev?sslmode=disable
   make migrate-up
   ```
   For Supabase, set `DATABASE_URL` to the session pooler address from the Connect page, with `sslmode=require`.
4. **Server.** `make dev` starts the API on port 8080.
5. **App.** `npm install`, then `npx expo run:ios`. This builds the app into the simulator. It needs a native build, so Expo Go won't work. The app looks for the API at `http://localhost:8080`, or at `EXPO_PUBLIC_API_BASE_URL` if that is set.

Optional:

- **Admin page.** Set `ADMIN_EMAILS` to your account's email before starting the server, then open `http://localhost:8080/admin`.
  The admin account must exist before the server starts: sign up with that email while it is not in `ADMIN_EMAILS` (sign-up refuses a listed address), then add it and restart the server.
  Log in with that account's app email and password, or run `make admin` to open the page already logged in. It makes a one-time link that works for two minutes, signed with `JWT_SECRET`, the same secret that signs the page's cookie.
- **Forgot a password.** Run `make password` in a terminal. It asks for the account's email or username, then the new password twice without showing it, checks it with the app's password rule and ends every app session of that account.
- **Phone notifications.** These are off until Apple and Expo credentials are added. The steps are in [`docs/PUSH-SETUP.md`](./docs/PUSH-SETUP.md).

Tests:

```sh
make test                  # quick tests, no database needed
make test-all-integration  # everything, against TEST_DATABASE_URL, one package at a time
npm run typecheck          # the app's TypeScript check
for f in scripts/check-*.ts; do node --experimental-strip-types "$f"; done  # all 12 check scripts
node --experimental-strip-types scripts/check-legal.ts  # the Terms and Privacy text; add --write to refresh docs/legal
```

If the integration tests fail with `column "..." does not exist`, the test database has hit PostgreSQL's limit of 1,600 columns in a table: the tests that undo and redo migrations use up a few more on every run. With nothing connected to it, recreate it with `dropdb rndmroll_test && createdb rndmroll_test`, and the suite migrates the new database itself. Don't use `make db-create` for this, since it also makes `rndmroll_dev`.

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
