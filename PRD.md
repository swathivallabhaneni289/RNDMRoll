# RNDMRoll: Product Requirements Document

| | |
|---|---|
| **Product** | RNDMRoll, an iPhone app for small groups of friends |
| **Version** | 1.0, built and not yet released |
| **Status** | Version 1 runs end to end on the iPhone Simulator against a server on my computer, with the database and photos on Supabase in Singapore. A public release waits on the launch checklist in [section 14](#14-release-plan). |
| **Last updated** | November 17, 2026 |
| **Related** | [README](./README.md) (the case study), [RESEARCH.md](./RESEARCH.md) (competitor research), [`.planning/`](./.planning) (phase specs and plans), [Terms of Use](./docs/legal/TERMS.md), [Privacy Policy](./docs/legal/PRIVACY.md) |
| **How it was built** | I built RNDMRoll by directing Claude Code. The product decisions, designs and testing are mine; Claude wrote the code to my prompts. The README's "How I built it with Claude" section has the detail. |

## Contents

1. [Summary](#1-summary)
2. [Problem and opportunity](#2-problem-and-opportunity)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Users](#4-users)
5. [Research](#5-research)
6. [Product principles](#6-product-principles)
7. [The experience](#7-the-experience)
8. [Functional requirements](#8-functional-requirements)
9. [Business rules](#9-business-rules)
10. [Non-functional requirements](#10-non-functional-requirements)
11. [Design and content guidelines](#11-design-and-content-guidelines)
12. [System overview and data](#12-system-overview-and-data)
13. [Success metrics](#13-success-metrics)
14. [Release plan](#14-release-plan)
15. [Risks and mitigations](#15-risks-and-mitigations)
16. [Known issues and open questions](#16-known-issues-and-open-questions)
17. [Decision log](#17-decision-log)
18. [Out of scope and future ideas](#18-out-of-scope-and-future-ideas)
- [Appendix A: Glossary](#appendix-a-glossary)
- [Appendix B: API overview](#appendix-b-api-overview)

**How to read the requirement tables.** Every requirement has an ID, a priority and a status.

- **Priority.** P0 is needed for the core loop to work. P1 makes the loop pleasant or safe. P2 is polish.
- **Status.** *Built* works today. *Built, off* is finished but switched off until outside accounts exist. *Launch* must be done before a public release. *Later* is a deliberate future idea. *Removed* was built or planned, then dropped on purpose.

---

## 1. Summary

RNDMRoll is a daily ritual for a small group of friends. At 8 PM in your own time zone, your spin opens: you tap the wheel and it lands on a category, such as a meal, a movie, a song, a book, a place or a game. You post one real thing for it: photos, a title, a half-star rating and an optional note. Your friends' posts for that day stay locked until you post your own. Over weeks, the posts become a record of what you and your friends actually ate, watched, listened to, read, visited and played.

**The core loop**

1. **Spin.** Once a day, from 8 PM, the wheel picks today's category.
2. **Post.** One real thing for that category, on one page, mostly with taps.
3. **See friends.** The Board opens and shows what friends posted for the same day.

**Version 1 at a glance**

| | |
|---|---|
| Audience | Friend groups; anyone 13 and over can join |
| Platform | iPhone app (Expo and React Native, TypeScript); Go API; PostgreSQL and private photo storage on Supabase (Singapore) |
| Scope | Accounts, the daily spin, posts, profiles, following, the Board, comments, notifications, reporting, an admin page, Terms and Privacy |
| Built in | Six phases plus two short inserted ones (2.1 and 2.2), September 15 to October 25, 2026 |
| Quality | More than 750 Go test functions (more than 210 of them against a real PostgreSQL database), 12 app check scripts, strict TypeScript |
| Not yet | Released. Real email, push credentials, a domain, database backups and a legal review are on the launch checklist. |

## 2. Problem and opportunity

People enjoy keeping a record of their taste. Letterboxd does it for films and Beli for food. People also enjoy a small daily prompt they share with friends, which is what made BeReal work. The two rarely meet:

- **Logging apps wait for you to remember.** Most logs go quiet after a few weeks, because nothing brings you back.
- **Daily photo apps bring you back but leave nothing behind.** Yesterday's photo is gone and nothing adds up.

**The opportunity.** A random daily pick gives people a reason to show up. A structured post (category, title, rating, photo) turns each day into a lasting record. Locking friends' posts until you post keeps everyone contributing instead of scrolling. The research (section 5) found that every piece exists somewhere, but found no app that joins a random pick to a lasting, rated, photographed entry, or puts a post-first lock on a taste diary.

## 3. Goals and non-goals

### Goals for version 1

| ID | Goal | How we know |
|---|---|---|
| G1 | A daily habit a small group keeps | People post on 4 or more days most weeks (section 13) |
| G2 | Each day becomes a lasting record | Every post is structured and stays on the profile, even after the day passes |
| G3 | Contributing is the way in | The Board opens only after you post; most posts are followed by a Board visit |
| G4 | Small, private and safe | Every account is private, every follow is a request, 13 and over only; people, posts and comments can be reported, and people blocked |
| G5 | Posting stays light | One page, mostly taps; library photos allowed; the emoji and the note are optional |

### Non-goals for version 1

- **Growth beyond friend groups.** The first thing to prove is a habit a handful of people keep.
- **Ads, selling data or tracking tools.** RNDMRoll has none.
- **Daily streak pressure.** The weekly goal replaces a strict daily streak on purpose.
- **Unlimited re-spins.** They would make the daily pick meaningless.
- **Music integration.** People post a screenshot of a song as a photo instead.
- **Live-camera-only posts.** You can log the book you finished two nights ago.
- **Android.** The app is React Native, but version 1 is designed and checked on iPhone only.

## 4. Users

**Target user.** A member of a small circle of friends who want a light evening habit and a shared record of what they are into. They open the app in the evening, when their spin opens, to post and to see what friends posted.

### Personas

These are drawn from the planning notes and research. No demographics were invented.

**A. The daily regular.** Part of a small group that uses the app most evenings. Spins at 8 PM, posts a photo, a title and a rating, then opens the Board to see and comment on friends' posts. Motivated by the weekly goal, the bonus spin and the week streak. Needs: posting that takes seconds. "Nobody wants to physically enter something."

**B. The friend whose evening doesn't match the wheel.** The wheel says "Book" on a day they read nothing. They have friends in other time zones, sometimes post after midnight, and travel. Needs: a free skip that doesn't break anything, their own 8 PM, a late window, and permission to log something from earlier in the week.

**C. The newcomer.** Just signed up and follows nobody. Must be 13 or over. Needs: a first spin right away, an easy way to find and invite friends, and a Board that explains itself instead of sitting empty.

**Admin (a role, not a persona).** Reviews reports on a web page and can remove content, bring it back within 30 days, or disable accounts.

### Jobs to be done

- When it's evening and I want a small thing to share with friends, I want something to pick for me, so I don't have to decide.
- When I've done something worth remembering, I want to log it in seconds, so my record keeps growing.
- When my friends post, I want to see what they got and how they rated it, so we have something to talk about.
- When the pick doesn't fit my day, I want to skip without losing my progress.

## 5. Research

On September 15, 2026, I asked for a competitor review: about 50 searches across five angles, covering roughly a dozen apps in depth. The full write-up is in [RESEARCH.md](./RESEARCH.md).

**Verdict.** The research found no exact match, but every piece has shipped somewhere. The opening is in how the pieces are joined, not in any one feature.

| App | What it does | What RNDMRoll takes or avoids |
|---|---|---|
| BeReal | A daily surprise prompt and a post-before-you-peek feed | Takes the lock, made softer with a late window. Avoids one shared moment for everyone: each person has their own 8 PM. |
| Letterboxd | Star ratings, a diary and short reviews, for films only | Takes half stars and repeat entries (a rewatch is a new post). Avoids poster art. |
| Beli | Your own food photos and a competitive friend feed | Proof that photos plus friends can be a daily habit. Avoids ranking items against each other: half stars work across a meal, a movie and a book. |
| Memorizer, Quick Memo | Photo, opinion, rating and category in one entry | Takes one entry shape for every category. Adds the daily pick and friends. |
| pov | A daily prompt, an unedited photo, streaks and a friend feed | Adds the rating and the lasting record |
| PI.FYI | Taste logging across categories, a friend feed and a daily streak | Adds the random pick and the lock |
| The Dice Challenge | A roll assigns a dare | Takes random assignment, but the result is a kept, rated post. The dice became a wheel. |
| Daylio | Logging in two taps and under 30 seconds | Takes the bar for low effort and an editable set of categories. |
| Wordle | One puzzle a day | Takes once-a-day scarcity. |
| Poparazzi, Artifact, Lapse | Apps that struggled or shut down | Warnings: authenticity alone isn't enough, empty feeds kill apps, and the social half is not optional. |

**Two open gaps**

1. A random pick that produces a lasting, rated, photographed entry. Randomizers pick something and forget it.
2. A friend feed locked until you post, applied to a taste diary instead of a selfie.

**Risks the research named, and the product's answer**

| Risk | Answer in version 1 |
|---|---|
| Cold start: a locked feed is empty without friends | First spin right away; Find your friends after sign-up with invite links and suggestions; a Board that explains itself |
| A heavy post form fights the lock that rewards it | One page, tap-to-rate stars, optional emoji and note, library photos allowed |
| A random category won't always fit the day | A free skip, a weekly goal of 4 instead of a daily streak, "watched or read lately" counts |
| "Your own photo" can't be enforced | Accepted. The app quietly records camera or library on each photo and shows nothing. |
| Scarcity against flexibility | Strictly once a day; the only extra spin is earned with the weekly goal |

## 6. Product principles

1. **A ritual, not a feed.** Once a day. The only extra spin is earned.
2. **Give before you get.** Friends' posts open after you post. A skip doesn't open them.
3. **Forgiving.** A free skip, a weekly goal instead of a daily streak, and late posts that still count.
4. **Your own real things.** Your photos, from the camera or the library. No fetched art.
5. **Small, trusted circles.** Every account is private and every follow is a request.
6. **Less typing.** Pickers, one-tap choices and a green "Looks good." when a field is right.
7. **The server owns the pick and the clock.** The server picks the category, sets every date, window and time label, and follows each person's own time zone through daylight saving changes. The app animates and prints. It formats date headings from the server's dates, and only the 8 PM reminder uses the phone's clock.
8. **Calm, never a casino.** Flat black and white, a restrained spin, no confetti, sounds or neon.
9. **The core loop first.** Spin, post, see friends. Everything else waits.

## 7. The experience

The bottom bar has three tabs: **Spin**, **Board** and **Profile**. The Spin page holds all of today: the wheel, the word, Log it and Skip it, the posted or skipped day, the countdown and the week.

### Journey 1: first evening

1. **Welcome.** "one spin. one real moment." with the spinning wheel logo and Get started.
2. **Sign up, page 1, "Create your account."** Email, password, birthday, name and username, with a line above Continue that links the Terms of Use and the Privacy Policy. Continue makes the account and signs the person in.
3. **Page 2, "Make it yours."** An optional photo and bio, then Continue or Skip for now. Shown once.
4. **Find your friends.** Invite friends (a link through the share sheet), Suggested for you and search. Done goes to the Spin page.
5. **The first spin.** A new person can spin at once, at any hour; before 8 PM the page says "Your first spin is ready." They tap the centre of the wheel; it turns for about 4 seconds and lands; the word rises.
6. **Log it.** Photos, a title, stars, an optional emoji and an optional note, then Save. The page shows "Posted." with "You're in. The Board is open." and a "See the Board" button, and stays that way until the next spin at 8 PM.
7. **The Board.** With no follows yet, it shows "See your friends' spins" and Find people.

### Journey 2: a normal evening

1. At 8 PM the phone reminds them: "Spin o'clock. Your wheel is ready when you are."
2. The Spin page says "Your spin is open." They tap the hub and get "Movie".
3. They post the film they watched this week, with 4.5 stars.
4. The Board unlocks. They see three friends' posts for tonight, open one and reply to a comment.

### Journey 3: the pick doesn't fit

1. The wheel lands on "Book" and they haven't read anything.
2. Skip it asks once: "Skip today's spin?", with the week count, and says the day can't be posted afterwards.
3. The week row shows a dash for the day. The goal count doesn't change and nothing is lost.
4. The Board stays locked for that day: "You skipped today. Posts from people you follow open after your next post."

### Journey 4: late, and the weekly goal

1. They spin at 11 PM and get Movie, but don't post that night. At 10 AM the phone says "Beat the noon bell. Post your movie pick to stay on time." They post at 2 PM: the page still says "Today's spin." and adds "Fashionably late is fine until 8:00 PM." Posting before noon would still have been on time.
2. The post counts for the day and opens the Board. Their card shows "Late".
3. It's their fourth posted day this week, so they earn a bonus spin. The week section says "You have a bonus spin.", and the notifications list says "You hit your goal this week. One bonus spin added."
4. A few days later, "Use bonus spin" turns an unwanted word into a different one.

### Journey 5: leaving

1. Settings, then Delete account. They read what will go, type their password and tap Delete account, then Delete.
2. The phone says the account will be deleted for good on a date 30 days away, and logs out.
3. If they log back in before then, the app says "Your account won't be deleted." and everything is as it was.

### Journey 6: something goes wrong

1. A comment breaks the rules. They press and hold it, tap Report and pick a reason.
2. The app says "Report sent." The comment and its replies are gone for them for good.
3. An admin sees the report on the admin page and removes the comment. Its writer gets "We removed your comment for bullying or harassment." in the bell. The comment is kept for 30 days in case the admin got it wrong, then deleted.
4. If the writer disagrees, tapping that message opens an email asking for a second look. Within the 30 days the admin can bring the comment back, and the writer gets "Sorry, we got that wrong. Your comment is back."

## 8. Functional requirements

### 8.1 Accounts and sign-up

| ID | Requirement | Priority | Status |
|---|---|---|---|
| ACC-01 | A Welcome page shows the spinning wheel logo and Get started, which leads to Sign up or Log in | P1 | Built |
| ACC-02 | A new person signs up on one page with email, password, birthday, name and username; one request creates the account and signs them in | P0 | Built |
| ACC-03 | People under 13 cannot make an account | P0 | Built |
| ACC-04 | A second page, shown once, offers an optional photo and bio | P1 | Built |
| ACC-05 | Find your friends is shown once after sign-up | P1 | Built |
| ACC-06 | A person logs in with an email or a username, and a password | P0 | Built |
| ACC-07 | Forgot password sends a 6-digit code by email, checks the code, and only then lets the person set a new password | P0 | Built; real email is Launch |
| ACC-08 | Sign-up shows the Terms of Use and Privacy Policy before Continue | P0 | Built |
| ACC-09 | The email address is verified | P1 | Launch |
| ACC-10 | A person can delete their account in the app | P0 | Built (October 25, 2026) |
| ACC-11 | A person can download a copy of their data | P2 | Later (by email request until then) |

**Acceptance criteria and rules**

- **ACC-02 email.** One plain address of at most 254 characters, in plain ASCII at sign-up. Unique, ignoring capitals. A taken email shows "That email is already in use." The server holds that answer for at least 300 ms. The one exception is a retry: when an earlier try got no answer or a server failure, and the email and password are the same, the app logs in to that account instead. The photo-and-bio page follows only when it is the account that try made.
- **ACC-02 password.** 8 to 72 characters, with a capital letter, a number and a symbol. A four-line checklist turns green as each rule is met. Typed once; there is no confirm box.
- **ACC-02 name and username.**
  - Name: 1 to 50 characters.
  - Username: 3 to 20 lowercase letters, numbers or underscores, unique ignoring capitals.
  - A username names its problem while it is typed: "No dots. Use _ instead.", the same for a space or a dash, "Leave out the @." for a leading @, "Only a to z, 0 to 9 and _." for any other character, and "At most 20 characters." The box has no length cap, so a long paste is named, not cut short. "Pick a username." and "At least 3 characters." wait until the box is left or Continue is tapped. The Edit page uses the same lines.
  - The app suggests a username from the name and checks it live 400 ms after typing stops, once it has no problem. A taken name shows up to 3 one-tap alternatives.
- **ACC-02 feedback.** Email, password, birthday and name show a green "Looks good." when they're right, and a free username shows a green "Available". Continue stays grey until all of it is right, and a tap on it then shows what is wrong (section 11, Forms). A birthday never picked says "Pick your birthday.", and the date scroller opens when it is the first problem. Limits: 10 sign-ups a minute per IP address, and 3 a minute per address and email.
- **ACC-03 birthday.** Picked on the iPhone's date scroller; a real date from 1900 on. Under 13, the field says "You must be at least 13 to use RNDMRoll." and Continue stays off. If the server still refuses, the app says "RNDMRoll is for ages 13 and up." Nothing is stored. The birthday is never shown to anyone and can't be changed in the app.
- **ACC-04.** The bio is at most 160 characters. Skip for now discards anything typed.
- **ACC-05.** The page offers Invite friends, Suggested for you and search, and Done goes to the Spin page.
- **ACC-06.**
  - A username can be typed as the app shows it, with one leading @ ("@name").
  - A wrong password and an unknown account get the same answer: "That email, username or password isn't right."
  - A disabled account hears "This account has been disabled." only after the right password.
  - Limits: 20 a minute per IP address, 5 per address and typed name, and 5 per address and account.
- **ACC-07 the code.**
  - The page always moves to the code step, whether or not the email has an account.
  - A code lasts 30 minutes and allows 5 wrong tries. Asking again within a minute sends nothing. A newer code replaces the old one, and wrong tries carry over.
  - Only a hash of the code is stored.
- **ACC-07 the result.** A reset signs the person in on this phone and ends every other session.
- **ACC-10, like Instagram.**
  - Settings, then Delete account. The page says what goes and when, asks for the password, and confirms once: "Delete your account?"
  - The account is hidden from everyone at once, every session ends, and the phone shows the date it will be deleted for good, 30 days later, then logs out.
  - Logging in, or resetting the password, within the 30 days cancels the deletion and says "Your account won't be deleted."
  - After 30 days a job on the server deletes the photo files first, then the account and every row tied to it. The account then behaves as if it never existed.
  - Limit: 5 tries a minute per person.

### 8.2 Sessions and passwords

| ID | Requirement | Priority | Status |
|---|---|---|---|
| SES-01 | A person stays logged in across app launches | P0 | Built |
| SES-02 | Log out ends the session on the server and on the phone | P0 | Built |
| SES-03 | A log out made offline is sent when the connection returns | P1 | Built |
| SES-04 | Launching without a connection keeps the person logged in and offers Try again | P1 | Built |
| SES-05 | A person changes their password in Settings, which logs out their other phones | P0 | Built |
| SES-06 | A disabled account is logged out at once everywhere | P0 | Built |
| SES-07 | A list of sessions and "log out everywhere" | P2 | Later |

**Acceptance criteria and rules**

- **SES-01 tokens.**
  - The access token lasts 15 minutes.
  - The refresh token lasts 30 days from its last use. It is replaced on every refresh and stored only as a hash.
  - Presenting an old, already-replaced token ends that whole session.
- **SES-01 on the phone.** Tokens are kept in the iPhone keychain, on that phone only.
- **SES-02.** Settings asks "Are you sure you want to log out?"; the Edit page asks with a sheet. Log out also removes this phone's push token.
- **SES-03.** Up to 10 unsent log outs are kept and retried at launch, on return to the app and after any request that works.
- **SES-04.** The page reads "You're offline." and "Check your connection and try again. You are still logged in."
- **SES-05.**
  - The current password is required, and the new one must differ and pass the sign-up rule.
  - Settings then shows "Password changed. Other phones are signed out."
  - Limit: 5 a minute per person.

### 8.3 The daily spin and the wheel

| ID | Requirement | Priority | Status |
|---|---|---|---|
| SPIN-01 | One daily spin per local date, open from 8:00 PM to midnight in the person's own time zone | P0 | Built |
| SPIN-02 | A brand-new person can spin at once, at any hour | P1 | Built |
| SPIN-03 | The server picks the category; the app animates the wheel to it | P0 | Built |
| SPIN-04 | People edit their wheel: rename, add and remove categories | P0 | Built |
| SPIN-05 | A person can skip a day for free | P0 | Built |
| SPIN-06 | A post is on time until noon the next day; from noon to the next 8 PM it still counts, marked late | P0 | Built |
| SPIN-07 | Four posted days in a week earn one bonus spin | P0 | Built |
| SPIN-08 | A bonus spin swaps today's word for a different one | P1 | Built |
| SPIN-09 | A countdown shows when the next spin opens | P1 | Built |
| SPIN-10 | The phone reports its time zone; days follow it through daylight saving changes | P0 | Built |
| SPIN-11 | A reminder on the phone at 8 PM: "Spin o'clock. Your wheel is ready when you are." | P1 | Built |
| SPIN-12 | People choose the reminder's time | P2 | Later |
| SPIN-13 | Weighted or resizable slices | P2 | Removed October 10, 2026; dragging slice edges is a Later idea |
| SPIN-14 | A note on the phone at 10 AM while a spin is not posted or skipped: "Beat the noon bell. Post your movie pick to stay on time." | P1 | Built |
| SPIN-15 | When a day closes with its spin not posted or skipped, the next open spin says so: "Monday's movie got away. New night, new spin." | P1 | Built |
| SPIN-16 | After an evening with no spin at all, the countdown page says "The wheel missed you." | P1 | Built |

**Acceptance criteria and rules**

- **SPIN-01 window.**
  - Daily spins are at least 12 hours apart, so changing time zones can't buy a second spin.
  - A missed day can't be spun later.
  - A retried spin request returns the same spin.
- **SPIN-02.** Before 8 PM the heading reads "Your first spin is ready."; from 8 PM it reads "Your spin is open." That spin is the day's spin, so there is no second one at 8 PM.
- **SPIN-02 after sign-up.** The Spin page starts loading on the photo-and-bio page, so it is ready when the person gets there.
- **SPIN-03 while loading.** Until the server's first answer, the Spin page shows a whole wheel of six blank wedges, hidden from VoiceOver, never an empty circle.
- **SPIN-03 the pick.** Every category has the same chance: the starting wheel and every wheel saved in the app weigh each category equally. The server stores the result, with a copy of the wheel, before answering.
- **SPIN-03 the turn.**
  - The wheel turns for 4 seconds, at least 5 full turns, and lands on the server's pick.
  - The word rises after a short pause, and the page shows "Spun at 8:02 PM".
  - With Reduce Motion on, the wheel doesn't turn and the result appears at once.
- **SPIN-04.**
  - The wheel starts as Meal, Movie, Song, Book, Place and Game.
  - It holds 2 to 12 categories, with names of 1 to 24 characters, unique ignoring capitals.
  - Changes apply from the next spin. Removed categories stay in past spins.
  - Edit wheel is on the Spin page before the day's spin and once the day is posted or skipped. It is not shown while a spin waits for its post.
  - Leaving with unsaved changes asks "Leave without saving?"
- **SPIN-05 the question.** Skip it asks once: "You are at {n} of 4 this week. A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards."
- **SPIN-05 the effect.** A skip has no limit and no cost. The week row shows a dash, and the Board stays locked for that day. "Skipped." stays on the page until the next spin at 8 PM.
- **SPIN-06.**
  - The server's clock decides lateness, and late posts show "Late" on the Board.
  - At the next 8 PM the day closes: "This day has closed. It can't be posted or skipped any more."
- **SPIN-07 the week.**
  - A week runs Monday to Sunday in the person's own time zone.
  - The goal is met at the fourth posted day. A person holds at most one bonus spin, and it never expires.
  - The week row shows seven days: posted, skipped, today and the rest.
  - The week row, its "today" and the date above the heading follow the spin's day, which runs from 8 PM to the next 8 PM. Until 8 PM on Monday they are still on Sunday, so a Sunday spin posted on Monday morning shows in the week it counts for.
- **SPIN-08.** Available only on today's result before posting or skipping, once per day. The new category is always different. It asks "Use your bonus spin?" first.
- **SPIN-09.** The countdown shows hours, minutes and seconds from the server's time on a steady clock, and never trusts the phone's clock.
- **SPIN-10.** The phone reports its zone on the first load after a launch or a sign-in, and again only when the zone changes. The answer to `POST /v1/time-zone` carries the Spin page's state, so no second request follows. If a report fails, the page still loads with the zone the server already has.
- **SPIN-11.**
  - The phone asks for permission once, at the first signed-in start, and the switch is in Settings.
  - Reminders are scheduled up to 30 evenings ahead, skipping an evening whose spin was already made.
- **SPIN-14.** The note comes two hours before a post would count as late, only while the spin is not posted or skipped, and follows the same switch as the 8 PM reminder. A post or a skip before then cancels it.

### 8.4 The daily post

| ID | Requirement | Priority | Status |
|---|---|---|---|
| POST-01 | Log it opens the post page for the current spin | P0 | Built |
| POST-02 | A post has 1 to 10 photos, a title and a star rating, plus an optional emoji and an optional note | P0 | Built |
| POST-03 | Photos come from the camera or the photo library | P0 | Built |
| POST-04 | The author can edit a post at any time | P1 | Built |
| POST-05 | The author can delete a post; the day still counts | P1 | Built |
| POST-06 | The author can archive a post and restore it from Settings, Archive | P2 | Built |
| POST-07 | The author can turn commenting off or on for each post | P2 | Built |
| POST-08 | The same title can be posted again as a new dated post | P2 | Built |
| POST-09 | Drafts that survive closing the app, and an "Edited" marker | P2 | Later |

**Acceptance criteria and rules**

- **POST-02 fields.**
  - Title: 1 to 80 characters.
  - Stars: half a star to 5 stars in half steps, stored as 1 to 10.
  - Emoji: one of six (love, fire, laughing, wow, sad, sleepy).
  - Note: at most 140 characters.
  - Save stays off until there is a photo, a title and stars, and the footer says what is still needed: "Still needed: photo, title and stars."
- **POST-03 photos.**
  - JPEG, PNG or WebP, each at most 50 MB.
  - A photo uploads straight to storage through a link that is valid for 5 minutes and signed for its type and size.
  - The app records camera or library on each photo and never shows it.
- **POST-04.** Editing never changes the date, the category or the late flag.
- **POST-05.** The post disappears for everyone. The posted day, the spin and the week's count stay. The Spin page says "This entry was deleted. The day still counts."
- **POST-06.** An archived post is hidden from everyone else, and from the author's profile and Board, though the author can still open it. The day still counts. Settings, Archive says "Only you can see these." and restores with one tap.
- **POST-07.** With commenting off, nobody can add a comment, the author included. Existing comments stay.

### 8.5 Profile and people pages

| ID | Requirement | Priority | Status |
|---|---|---|---|
| PROF-01 | A Profile page with photo, name, username, bio, figures and every post | P0 | Built |
| PROF-02 | An Edit page for photo, name, username and bio | P1 | Built |
| PROF-03 | A person page for anyone else | P0 | Built |
| PROF-04 | A People page with Followers (and requests) and Following | P1 | Built |
| PROF-05 | Filter the post history by category | P2 | Removed October 16, 2026 (the server still filters by category) |

**Acceptance criteria and rules**

- **PROF-01.**
  - Four figures: Spins, Week streak, Followers and Following. Followers and Following open the People page.
  - Every post that isn't archived appears in a three-column grid, newest first; a tile opens a scrolling list of the posts.
  - The page has an Edit button and a Settings gear.
- **PROF-02.** One save for all changes, then "Saved." for 3 seconds. A username change takes effect at once. A profile photo can be replaced but not removed.
- **PROF-03 what anyone signed in sees,** unless there is a block either way: name, username, photo, bio and four figures (Posts, Week streak, Followers and Following). Someone who doesn't follow the person sees "This account is private." in place of the posts.
- **PROF-03 what followers see.** An accepted follower also sees the posts, for days they posted themselves, and can open the Followers and Following lists.
- **PROF-03 the menu.** The three dots hold Remove follower (only when they follow you), Mute, Report and Block. After Report the app says "Report sent." and goes back. A report never hides a person: their page still opens (SAFE-01).
- **Week streak.** The number of weeks in a row that met the goal, ending this week or last week, so a week in progress never breaks it.

### 8.6 Following and finding people

| ID | Requirement | Priority | Status |
|---|---|---|---|
| SOC-01 | Every account is private and every follow is a request | P0 | Built |
| SOC-02 | Accept, decline, follow back, cancel a request, unfollow and remove a follower | P0 | Built |
| SOC-03 | Search people by username or name | P0 | Built |
| SOC-04 | Share an invite link | P1 | Built; links that open the app directly are Launch |
| SOC-05 | Suggestions of people to follow | P1 | Built |
| SOC-06 | Block a person | P0 | Built |
| SOC-07 | Mute a person | P1 | Built |

**Acceptance criteria and rules**

- **SOC-01.** Following is one way. Accepting doesn't follow back; the app offers "Follow back".
- **SOC-02.** Declines, cancels, unfollows and removals are silent. The other person just sees Follow again.
- **SOC-03.** Matches the start of a username, a name or any word of a name, up to 20 results. Email is never searchable. Opened from the Search field on the Board.
- **SOC-04.** The share sheet sends "Join me on RNDMRoll, one spin a day. Follow me here:" with a link. The link's web page has an "Open in RNDMRoll" button that opens the sender's profile in the app.
- **SOC-05.** Suggestions draw on people your follows follow, people who follow you, and the newest accounts, with people who follow you first.
- **SOC-06.**
  - A block removes follows both ways and hides each person from the other in search, the Board, posts, comments and notifications. The blocker still sees them in Blocked accounts and on their page, with Unblock, unless that account is disabled or being deleted.
  - The blocked person isn't told. Unblock restores nothing.
  - Blocked accounts are listed in Settings.
- **SOC-07.** Removes the person's posts from your Board only. Silent, no follow needed.

### 8.7 The Board

| ID | Requirement | Priority | Status |
|---|---|---|---|
| BRD-01 | The Board shows posts from people you follow, and your own, for days you posted | P0 | Built |
| BRD-02 | The day in play is locked until you post for it | P0 | Built |
| BRD-03 | A skipped day shows its own message | P0 | Built |
| BRD-04 | Following nobody shows a prompt to find people | P0 | Built |
| BRD-05 | A post page shows the full post and its comments | P0 | Built |
| BRD-06 | Reactions on friends' posts | P2 | Removed October 18, 2026 (comments cover it) |

**Acceptance criteria and rules**

- **BRD-01 layout.**
  - Two cards per row; days newest first, posts newest first within a day; 5 days per page.
  - Earlier days have headings such as "Yesterday" or "WED, OCT 8".
  - Pull to refresh.
- **BRD-01 a card.** The first photo, the name, the category, the title, the stars and "Late" when the post was late. Muted people are left out.
- **BRD-02 while locked.**
  - The page shows "Locked", how many people you follow posted, up to four first names, and one action: "Log your meal" (the day's category, in lower case), "Spin first", or the time the next spin opens.
  - Nothing else of those posts reaches the phone.
- **BRD-02 the day in play.** The most recent 8 PM that has passed. It lasts until the next 8 PM, so the Board calls it "Today" the whole time, also after midnight. A person who has never spun, or who made a first spin today (posted or not), is on today.
- **BRD-03.** "You skipped today. Posts from people you follow open after your next post." with Go to Spin.
- **BRD-04.** "See your friends' spins", "Follow people you know. Their posts show here on the days you post too." and Find people.
- **BRD-05.** The author, the photos edge to edge, the stars, the emoji, the note with the posted time, comments oldest first with replies under them, and Report in the three dots. After Report the post page closes, and the post is gone for the reporter everywhere (SAFE-01).

### 8.8 Comments

| ID | Requirement | Priority | Status |
|---|---|---|---|
| CMT-01 | Comment on a post you can see, and reply to a comment | P0 | Built |
| CMT-02 | Add one photo to a comment or reply | P1 | Built |
| CMT-03 | A quick row of eight emoji above the comment box | P2 | Built |
| CMT-04 | The post's author can hide a comment | P1 | Built |
| CMT-05 | The writer or the post's author can delete a comment | P1 | Built |
| CMT-06 | GIFs in comments | P2 | Later (needs a GIPHY key) |
| CMT-07 | Edit a comment, react to a comment | P2 | Later |

**Acceptance criteria and rules**

- **CMT-01.** 1 to 140 characters. Replies are one level deep, and a reply starts with "@username". Line breaks are allowed.
- **CMT-02.** JPEG, PNG or WebP, at most 50 MB. With a photo the text may be empty. In notifications, a photo-only comment reads "{first name} sent a photo."
- **CMT-03.** Tapping an emoji adds it to the text if it still fits in 140 characters.
- **CMT-04.** A hidden comment is seen only by its writer, who isn't told, and by the post's author, who sees "Hidden" and can Unhide. Its replies are hidden from everyone else too.
- **CMT-04 is not the review hide.** A comment with 3 or more open reports is hidden by a different rule (SAFE-05). Then only its writer sees it, with "Hidden while we take a look.", and the post's author doesn't see it either.
- **CMT-05.** Press and hold, then Delete. The app asks first: "It will be gone for good." Deleting a comment deletes its replies and photos.

### 8.9 Notifications

| ID | Requirement | Priority | Status |
|---|---|---|---|
| NOT-01 | An in-app list behind the bell on the Spin page | P0 | Built |
| NOT-02 | A Follow requests page with Accept and Decline | P0 | Built |
| NOT-03 | Switches for comment and milestone notifications and for the reminder | P1 | Built |
| NOT-04 | Phone notifications for comments, replies, follow requests and accepted requests | P1 | Built, off until Apple and Expo credentials exist |
| NOT-05 | Grouping ("Maya and 2 others") and marking one item read | P2 | Later |

**Acceptance criteria and rules**

- **NOT-01 what it lists.**
  - Comments on your posts, replies to your comments, new followers and accepted requests.
  - "Your spin is open." from 8 PM to midnight, on evenings the wheel opens. It is not shown after a first spin made earlier that day, and after a time zone change it comes when the 12-hour gap ends.
  - "You hit your goal this week. One bonus spin added."
  - Moderation of your own posts and comments:
    - "Your post is hidden while we take a look." ("Your comment is hidden while we take a look." for a comment). This comes from the report that hides it (SAFE-05). Tapping it opens the post (for a comment, the post it's on, while you can still see that post).
    - A removal names the reason the admin picked, for example "We removed your post for hate speech." ("your comment" for a comment). Something else keeps "We removed your post. It broke our rules." Tapping it opens an email to the contact address asking for a second look.
    - "Sorry, we got that wrong. Your post is back." ("Your comment is back." for a comment). This comes after Bring it back, or after a Dismiss that shows a hidden item again. Tapping it opens the post (for a comment, the post it's on, while you can still see that post).
- **NOT-01 how it shows.**
  - Sections: Today, This week and Earlier.
  - Items from the last 30 days, at most 50.
  - A dot on the bell while anything is unread. Opening the page marks all as seen.
- **NOT-02.** The first row of the list shows only a dot when a request waits, never a count or a name.
- **NOT-03.** "Comments" and "Milestones and bonus spins" are saved on the server. "Remind me to spin and post" (the 8 PM reminder and the 10 AM note) is saved on the phone. Follow notifications and the admin's decisions can't be turned off.
- **NOT-04.** A push token is tied to the login it came from and stops working when that login ends. Pushes never go to yourself or across a block. The admin's decisions are never pushed; they wait in the bell.

### 8.10 Safety and moderation

| ID | Requirement | Priority | Status |
|---|---|---|---|
| SAFE-01 | Report a person, a post or a comment | P0 | Built |
| SAFE-02 | Block, mute, hide a comment and remove a follower (8.6, 8.8) | P0 | Built |
| SAFE-03 | Minimum age of 13 (ACC-03) | P0 | Built |
| SAFE-04 | Admins are alerted about new reports | P1 | Launch |
| SAFE-05 | Content hidden automatically after several reports, and an appeal flow | P2 | Hiding at 3 reports Built (November 17, 2026); an appeal in the app is Later, and second looks are by email |

**Acceptance criteria and rules**

- **SAFE-01 reasons.** Spam; Nudity or sexual content; Bullying or harassment; Hate speech; Violence or threats; Pretending to be someone else; Something else.
- **SAFE-01 after a report.**
  - The app says "Report sent." A post report closes the post page, a person report closes the person's page, and a reported comment leaves the list with its replies.
  - A reported post or comment is gone for the reporter for good, on every read: the Board, the post page, the person page and its Posts figure, comments and their counts, the bell and phone notifications. A reported comment takes its replies with it, for the reporter only.
  - A report never hides a person. Their page still opens for the reporter.
  - One report per reporter and item. A repeat report of a post or comment answers 404, exactly like one that isn't there, and keeps the first reason and title. A repeat report of a person is as before: it replaces the reason and opens the report again.
  - You can't report your own post or comment.
  - Limit: 10 a minute for each kind (people, posts, comments).
- **SAFE-05 hidden while we take a look.**
  - A post or comment with 3 or more open reports, from people whose accounts aren't disabled or being deleted, is hidden from everyone but its owner and the admin page. It is worked out from the open reports on every read and never stored, so it shows again once fewer than 3 are open. The log only notes the report that hid it, for the owner's bell.
  - The owner sees it with the line "Hidden while we take a look." and keeps every action, delete included. For a comment, the owner is its writer.
  - Replies under a hidden comment go with it, except for its writer, who still sees the replies under it that they can see.
  - The report that hides it tells the owner in the bell, "Your post is hidden while we take a look." (NOT-01), and the admin page's History lists it as "Post hidden by reports" ("Comment" for a comment). Removed before doesn't count it.
  - The report that makes it hidden is emailed to the admins at once (ADM-06).
- **SAFE-05 second looks.** The removal message in the bell opens an email to the contact address, with the subject "Second look at my post" (or "my comment"), the username, the title or text, a short ref and a line for why it should come back. If the mail app can't open, the app says "Email {address} to ask for a second look." The address is a server setting (CONTACT_EMAIL). Without it, the message still shows and a tap does nothing.
- **Moderation, left out on purpose.**
  - An "ask for a second look" button in the app. Second looks are by email.
  - Phone pushes for the admin's decisions or a hide. They show in the bell only.
  - Anything sent on Disable, or when the 30-day job deletes removed content.
  - An undo for a reporter.
  - A message for people whose comments were under a removed post. Those comments just go with it.
  - A typed note with a removal. The reason is one of the 7 report reasons.
  - The reason on the Spin page's removed line and in the second-look email.
  - Remove on a post nobody reported. The posts on a person page are for looking only.

### 8.11 The admin page

| ID | Requirement | Priority | Status |
|---|---|---|---|
| ADM-01 | A web page at /admin, for admin accounts only | P0 | Built, off unless admin emails are set |
| ADM-02 | A Reports tab with one card per reported post, comment or account, a Disabled tab and a Removed tab | P0 | Built |
| ADM-03 | A person page laid out like a profile, with their details, their reports, open ones first (up to 200), their posts while a report about the account is open or it is disabled, and the history of decisions about them, only for people someone reported, disabled accounts and people with a past decision | P0 | Built |
| ADM-04 | Dismiss a card's reports, remove a post or comment, bring it back within 30 days, disable or enable an account | P0 | Built |
| ADM-05 | A record of which admin took each action | P2 | Later (the decision log says what was decided and when, not by whom) |
| ADM-06 | An email to the admins about new reports | P1 | Built, sent for real once email is on |

**Acceptance criteria and rules**

- **ADM-01 who is an admin.** Admins are the accounts listed in the server's settings when it starts. Their email can't be used to sign up.
- **ADM-01 the session.** An admin logs in with their app email and password and stays logged in for 8 hours, or until their password changes.
- **ADM-01 protection.** The page runs no scripts and carries strict security headers. Every action needs a form token and a same-site request.
- **ADM-02 the Reports tab.** Every report on the same post, comment or account is one card: why it was reported, who reported it (up to three usernames, then how many more), when, the thing itself and whose it is, with Remove, Dismiss and Open profile.
  - The cards sit side by side, as many as fit across the screen, and one under another on a phone. The Removed tab and the person page lay out their cards the same way.
  - A post or comment card that is hidden under review (SAFE-05) has a "Hidden" tag. This is not the author's Hide (CMT-04).
  - Order: hidden cards first, then violence and nudity, then bullying and hate, then the rest; within those, the most reported, then the longest waiting.
  - The owner row says "Removed before: N" when N is more than 0.
  - The tabs and the browser tab show how many cards are open, like "(3) Reports", and the Reports tab refreshes itself every minute.
  - The newest 500 open reports show; past that the page says so.
- **ADM-02 the Disabled tab.** Each disabled account with its photo, username, name and when, with Enable right there.
- **ADM-02 the Removed tab.** Everything removed in the last 30 days: what it is, whose it is, the title or text, its photos, when it was removed, the days left, the reason picked at Remove, a short ref and Bring it back. The ref is the one in the owner's second-look email. This tab doesn't refresh itself.
- **ADM-03 what a report shows.** The admin sees the reported thing in full before acting.
  - The person: name, username, profile photo, bio and join date.
  - A post report: the whole post as it is now, with its title, its date, its text and every photo in order. Each photo opens full size in a new tab. When the title changed after the report, the title it had then shows too, and both are labelled. A post that is gone (deleted by its author, or 30 days after a removal) shows only the title it had when it was reported, and says so.
  - A comment report: the comment's text from when it was reported, and its photo.
  - The person page reads like a profile. A back arrow on top goes to the tab it was opened from (Reports, Disabled or Removed). Then come their photo, name, username, bio and join date with Disable or Enable, the open cards, their posts, the History and the closed cards.
  - The person page puts open cards first. Closed ones fold away under "Closed reports", one card for each action that closed them.
  - Their posts show while a report about the account is open, or while it's disabled, since Disable closes those reports and Enable is decided on them. The newest 60 show, newest first, each with every photo (the first one large), its category and day, its title and text, and a tag when it's removed, hidden or reported. Otherwise the page says when they show.
  - The person page's History lists every decision about the person, newest first: removed, brought back, shown again, disabled and enabled, each with when. Their profile card says "Removed before: N" once, when N is more than 0.
- **ADM-03 privacy.**
  - An admin can open only people someone reported (them, a post or a comment), disabled accounts and people with a decision in the log. Any other person's page goes back to the list, and they can't be disabled. Each report names who made it, by username.
  - Their posts show only while a report about the account is open or it's disabled. A report about one post or comment shows only that one. Archived posts, and posts past the 30 days after a removal, never show.
  - The page never shows an email address or a birthday.
- **ADM-04 dismissing.** Dismiss closes every report on the card, and only those: a report that came in after the page was drawn stays open.
  - If the post or comment was hidden (SAFE-05) and no longer is, it shows again for everyone except the people who reported it. Its owner gets "Sorry, we got that wrong. Your post is back." ("Your comment is back." for a comment).
  - Nothing is sent when it was never hidden, when newer reports keep it hidden, or when a comment's post or the comment it answers is still hidden or removed.
- **ADM-04 removing.**
  - Remove hides the post or comment from everyone at once, its owner included, and closes its reports. Nothing is deleted yet: no files and no rows.
  - After 30 days an hourly job deletes it for good with its photos, and with the comments or replies under it and their photos.
  - Remove has a Reason list of the 7 report reasons, set to the reporters' most serious one. The admin can pick another.
  - The owner's message in the bell names that reason, for example "We removed your post for hate speech." ("your comment" for a comment). Something else keeps "We removed your post. It broke our rules." The History and the Removed tab show the reason too.
  - A removed post still counts for its day, like a post its author deleted: the posted day, the late flag, the week toward the bonus, the streak and the Board's unlock stay, and no new post is allowed that day. The Spin page says "We removed this post. The day still counts."
  - Edit, archive, delete, hide and delete comment answer 404 on a removed post or comment.
- **ADM-04 bringing back.** Bring it back, on the Removed tab, works within the 30 days. The post or comment shows again for everyone who could see it before, except the people who reported it, and the owner gets "Sorry, we got that wrong. Your post is back." After the 30 days it can't come back, and a comment can't come back while its post or the comment it answers is still removed.
- **ADM-04 disabling.**
  - Disabling closes the person's reports and ends every session at once.
  - It also deletes their reset code and push tokens.
  - It hides their profile, posts and comments from everyone else in the app.
  - Enabling brings it all back. An admin can't disable themselves.
  - Disable and Enable go in the decision log. Nothing is sent to the person.
- **The decision log.** Every Remove, Bring it back, Dismiss that shows something again, Disable and Enable, and each report that hides something, is saved with whose post, comment or account it was and when. One about a post or comment keeps its title or text at the time, and a removal also keeps the report reasons and the reason picked at Remove.
  - It feeds the person page's History, "Removed before" and the owner's bell messages, so a second-look email can be answered.
  - "Removed before: N" counts the person's removed posts and comments that were not brought back.
  - It doesn't record which admin acted (ADM-05).
- **Reporting again.** A second report of the same account by the same person replaces the reason and opens it again, even after an admin closed it. A second report of the same post or comment answers like a miss (SAFE-01).
- **ADM-06 the email.** When someone reports a person, post or comment, the admins get an email: at once for violence or nudity, and for the report that hides a post or comment (SAFE-05), at most one a minute; otherwise at most one an hour, counting everything that came in.
  - It says how many reports came in and why, how many of them hid a post or comment ("Hidden while you take a look: N", only when any did), and links to the admin page. It never names anyone or shows what was reported.
  - The link opens the page through a short hop, so an admin who is logged in stays logged in when they click it from their mail.
  - Waiting reports are kept in memory and sent when the server stops.
  - It uses the same sender as the Forgot password code: printed to the server log for now, sent with Resend once email is on.

### 8.12 Settings and legal

| ID | Requirement | Priority | Status |
|---|---|---|---|
| SET-01 | Settings, from the gear on Profile: Notifications, Blocked accounts, Archive, Password, Terms of Use, Privacy Policy, Delete account and Log out | P0 | Built |
| SET-02 | The Terms of Use and the Privacy Policy, readable in the app and on GitHub, from one source | P0 | Built (October 25, 2026); legal review is Launch |
| SET-03 | Record which version of the terms a person accepted, and tell people when the terms change | P1 | Launch |
| SET-04 | Change email, two-factor sign-in and Face ID | P2 | Later |

**Acceptance criteria and rules**

- **SET-02 where.** The sign-up page links both documents above Continue: "By tapping Continue, you agree to the Terms of Use and confirm you've read the Privacy Policy." Settings has a row for each.
- **SET-02 one source.** The text lives in one file. A check script writes the Markdown copies in `docs/legal` and fails if they drift.
- **SET-02 the links.** The two names are ink, the same size as the line and never underlined. VoiceOver reaches them as the line's two actions.

## 9. Business rules

These rules decide what counts. The server owns all of them.

| Rule | Definition |
|---|---|
| Today | The calendar date of the server's clock in the person's stored time zone |
| The spin window | 8:00 PM to local midnight. A person who has never spun can spin at once, at any hour, and gets no second spin at 8 PM that day. Two daily spins are at least 12 hours apart. When a clock change skips or repeats the hour, the first real instant counts. |
| Late window | Noon the next day to the next 8:00 PM. A post then is late and still counts. A post between midnight and noon is on time. |
| Day closed | At the next 8:00 PM a day can no longer be posted or skipped |
| Posted day | A day with a post. It stays posted if the post is later deleted, archived or removed by an admin. |
| Skip | Final, free and unlimited. It is not a posted day and not a miss. |
| Week | Monday to Sunday in the person's time zone. A late post counts toward the week of its spin's date. |
| Weekly goal | 4 posted days in a week |
| Bonus spin | Earned when a week reaches the goal and no bonus is held; a week that reaches it while one is held earns none. At most one held; never expires; only on today's result, once per day; always a different category. |
| Week streak | Weeks in a row that met the goal, ending this week or last |
| Spins figure | Days spun; a bonus spin doesn't add one |
| Day in play (Board) | The date of the most recent 8:00 PM that has passed, or today for a person who made a first spin today or has never spun |
| Who sees a post | Nobody in the app once an admin removed it, the author included. Otherwise the author always. Anyone else only if they follow the author (accepted), there is no block either way, the post isn't archived, they haven't reported it, it isn't hidden under review, and they posted for that date themselves. On the admin page, an admin also sees a reported post in full, with every photo, and a removed post until it is deleted. |
| Who sees a comment | Anyone who can see the post, except: a comment by someone they blocked or who blocked them, or whose account is disabled or being deleted; a comment they reported; a comment the post's author hid, seen only by its writer and the post's author; a comment hidden under review, seen only by its writer; and a removed comment, seen by nobody in the app. A reply shows only with the comment it answers. |
| Hidden under review | 3 or more open reports on a post or comment from people whose accounts aren't disabled or being deleted. Worked out live from the open reports, never stored. The log only notes the report that hid it, for the owner's bell. |
| Removed | An admin's Remove: hidden from everyone at once, kept 30 days so it can be brought back, then deleted with its photos. One 30-day rule, on the database clock. |
| Disabled account | Treated as a block by everyone in the app: hidden everywhere, and logged out. Admins still see the account on the admin page. |
| Deleted account | Hidden from everyone at once, like a disabled account, and logged out. Deleted for good 30 days later; logging in or resetting the password before then cancels it. One 30-day rule, on the database clock, decides both. |
| Time labels | Written by the server ("8:41 PM", "Mon", "Oct 3"). The app never works out a time from the phone's clock, except the 8 PM reminder and the birthday check, which the server checks again. |

## 10. Non-functional requirements

### Security

- Passwords are hashed with bcrypt. A wrong password and an unknown account take the same work and get the same answer.
- Access tokens are signed JSON Web Tokens with the algorithm pinned, and last 15 minutes.
- Refresh tokens are 256-bit random values, stored only as SHA-256 hashes, rotated on every refresh. Reuse ends the session.
- Reset codes come from a secure random source, are stored as HMAC hashes, expire in 30 minutes and allow 5 wrong tries. Every reset answer takes at least 300 ms.
- Rate limits on almost every API route (Appendix B). GET /me, the profile photo upload link, the health check, the invite page and the admin pages after log in have none. IPv6 addresses count by their /64, and also by their /48.
- Request bodies are capped at 16 KB. Path IDs that aren't UUIDs never reach the database. Error answers use fixed text only.
- Photo uploads use 5-minute signed links, limited to the uploader's own folder, type and size.
- The admin page uses a signed, HttpOnly, SameSite=Strict cookie, form tokens, an Origin check, a strict Content Security Policy and no framing.
- **Launch:** HTTPS through the host; trusting the real proxy for client IP addresses.

### Privacy

- No ads, analytics, crash reporting or tracking tools in the app or the server.
- Other people in the app never see your email or birthday, and search never matches email. Admins don't either, and the admin page opens only accounts someone reported, that are disabled, or that have a decision in the log (ADM-03).
- The decision log keeps, for each Remove, Bring it back, Dismiss that shows something again, Disable and Enable, and each report that hides something: whose post, comment or account it was, what was decided and when, the title or comment text at the time, and for a removal the report reasons and the reason picked at Remove. It names neither the reporters nor the admin. It is kept while the owner's account exists and is deleted with it.
- The app asks for the camera, the photo library and notifications only, and never for location or contacts.
- Request logs never record headers or bodies, and secret values in links are hidden. **Fix before launch:** search words are still logged. While email runs in log mode (today's setup), each email, with its address and any reset code, is printed to the server log.
- Photos are in a private bucket. The server gives a signed photo link only to someone allowed to see that photo, and each link stops working within an hour. A link already given out works until it runs out.
- **Launch:** removal of photo metadata for every file type.
- The Privacy Policy describes the real data practices, including today's limits.

### Reliability

- Every API request has a 10-second limit on the phone; photo uploads to storage have no limit set. Spin, post, post edit and skip requests are safe to retry, and so is a comment with a photo.
- Only a definite "no" from the server logs a person out. A lost connection never does.
- Each person's spin work waits at most 5 seconds for its lock, then answers "try again".
- Emails and pushes are sent in the background and never fail the request that caused them.
- **Launch:** database backups (the Supabase free plan has none, and database backups never cover photo files), log retention and a cleanup job for unused photo files and old login records.

### Performance

- Page sizes: Board 5 days (up to 10), person page 21 posts (up to 60), post history 20 (up to 50). Single lists with no more pages: notifications (at most 50, from the last 30 days), search 20, suggestions 20.
- Camera photos and most library photos are saved as 80% JPEGs on the phone before upload. Library PNG files are saved again as PNG, and WebP files go up unchanged. Photos are cached on the phone after the first view.
- A Spin page load is normally one request; when the zone is reported, that answer carries the page's state. On the server, eight of the state's reads go to the database together in one batch, the 5-second lock limit is sent with the start of the transaction, and two database connections stay open while idle.
- **Later:** keep the Spin page's last state on the phone across launches, so a cold start shows it at once; fewer database round trips behind the bell's dot; and a new person's state row and starting wheel written in one statement.
- **Launch:** set and measure targets for API response times and Board load time. None are measured yet.

### Accessibility

- VoiceOver labels and hints on the app's controls. The wheel announces "Your spin is {word}." Posted and Skipped are announced when they change.
- A box with a problem has it as its VoiceOver hint. A grey button that answers a tap is not read as dimmed, and its hint names what is missing, such as "Still needed: birthday and username." After the tap, VoiceOver says the first problem.
- Reduce Motion: no spin, no sliding sheets, no photo reveals, and a still background.
- Larger Text is honoured. The shared text style stops at 1.3 times, so page headers don't break. A few labels, such as the Board's day headings, have no cap yet.
- Most touch targets are at least 44 points. A few are smaller: View all comments, a comment's name and photo, and the Terms and Privacy words on sign-up.
- Text contrast: ink 17.3:1 and muted grey 5.8:1 on the page colour.
- **Launch:** walk Reduce Motion and VoiceOver on a device (not yet walked).

### Time and time zones

- Every day, window and week follows the person's own time zone, through daylight saving changes, skipped midnights and skipped dates.
- Tests cover 15 zones on every day of 2026, including 30-minute shifts, skipped midnights and Samoa's skipped day in 2011.

### Compatibility

- iPhone, portrait, light mode. Android is not checked.

### Quality

- More than 750 Go test functions; more than 210 of them run against a real PostgreSQL database.
- The Go tests run about 85% of the server's code (statement coverage).
- 12 check scripts: eleven for app logic and one for the legal text.
- Strict TypeScript.
- **Later:** continuous integration on every push (none yet).

## 11. Design and content guidelines

**Look.** Light, editorial and black and white:

- Paper `#F6F5F2`, ink `#111111`, muted `#625F5B`.
- Green only for "Looks good.", "Available" and the met rules of the password checklist; red only for Log out.
- A problem under a field is in ink with an alert mark, and its box keeps the dark border, so it never reads as a grey hint.
- Domine for headings and Work Sans for text.
- Buttons are rounded at most 8 points. My pictures add a few round shapes: the pill chips (Edit, Follow, Following, Follow back, Unblock), the Camera and Library buttons, the round emoji buttons and the comment field.

**The wheel.** Equal black and light slices, words only, a thin rim and a slim pointer. Flat, so it never reads as a prize wheel.

**The logo.** The Shutter mark is the app icon: camera blades on a wheel. I chose it from logo concepts I asked for. Inside the app it appears only beside the RNDMRoll title on the Spin page.

**Motion.** Calm and short:

- The spin is about 4 seconds of slowing down, then the word rises.
- Tabs swap at once. Pushed pages keep the iOS slide, except the Sign up or Log in page, which fades in from Welcome.
- No confetti, sound or neon. Two small bounces remain: the pop when an emoji is picked and the pointer settling on Welcome.

**Forms.**

- Most problems show once the box is left or the button is tapped. A username's characters and length show at once (ACC-02).
- A grey button still answers a tap. On sign-up, Edit profile, Password, Delete account and Edit wheel, every field shows its problem, touched or not, the page scrolls to the first one and the cursor goes into its box. The keyboard then leaves the whole field in view, with its line and anything under it, such as the username's alternatives or the password checklist. The post form's Save scrolls to the first missing part.
- A button that is grey only because nothing changed, or while a check or a save runs, ignores a tap.
- **Later:** a second pass over the rest of the November 16 review of forms and messages.

**Copy rules.**

- Say "spin", never "roll".
- No em dashes.
- Plain, short sentences.
- No underlined text used as a button; actions are buttons, icons or ink words. Exception today: the grey "Skip for now" on the second sign-up page.
- No counts that can grow large under posts.
- No fake reviews or counters.
- No vague hero lines.
- No emoji used as icons.

**Imagery.** The wheel mark and the screen designs are mine. The Welcome wheel's category drawings are first-pass line drawings for me to redraw. The app has no stock or generated photos.

## 12. System overview and data

**Architecture.**

- **App:** an Expo and React Native app (TypeScript, Expo Router, Reanimated).
- **API:** a Go server (Gin, pgx) with 20 SQL migrations. It runs on my computer until it is hosted, and reaches PostgreSQL on Supabase in Singapore through the session pooler over TLS. Supabase's Data API is off, and only the server holds the database password.
- **Photos:** kept in a private Supabase Storage bucket in Singapore. The phone uploads through 5-minute signed links and loads photos through signed links that stop working within an hour, made only for people allowed to see them.
- **Services:** email through Resend, and phone notifications through Expo's push service and Apple. Both are built but off until the accounts exist; today emails print to the server log.
- **Web pages:** the admin page and the invite page are plain server-rendered pages on the API.

**Main data.**

| Area | Tables |
|---|---|
| Accounts | users, refresh_tokens, password_resets, push_tokens |
| Spin | user_time_zones, user_spin_state, wheel_categories, spins, posted_days, goal_weeks |
| Posts | entries, entry_photos |
| Social | follows, blocks, mutes, post_comments, notification_settings, notification_seen |
| Safety | user_reports, post_reports, comment_reports, moderation_events (the decision log) |

Two unused tables remain: email_verification_tokens and post_reactions.

**What is kept, and for how long.**

- A deleted account is hidden at once and deleted for good 30 days later by an hourly job on the server: its photo files first, then its row, which removes every other row tied to it. The job only touches that account's own photo folders, plus the photos other people added to comments on its posts.
- A deleted post removes its photos and comments from the database, and the server tries once to remove the photo files of the post and of its comments. A file whose removal fails stays in storage, but the server makes no new links for it (section 16, issue 3). Its spin and posted day stay.
- A post or comment an admin removed is hidden at once and kept for 30 days, so it can be brought back. Then the same hourly job, after the account purge, deletes the photo files first, its own and those of the comments and replies under it, then its row, which takes those comments and replies with it. A failure keeps the row for the next run. Its spin and posted day stay.
- Saving a new profile photo deletes the old photo's file. If that fails, the file stays (section 16, issue 3).
- Reports outlive the reported content, with a copy of the title or comment text, until the reporter's or the owner's account is deleted. The decision log keeps its copy until the owner's account is deleted.
- There is no automatic cleanup yet for unused photo files or old login records (see the launch checklist in section 14, and issue 3 in section 16).

## 13. Success metrics

The app isn't released, so there is no usage data. The targets below are my starting guesses, to be tested with the first group and changed once real numbers exist.

**North star: posted days per active person per week.** It measures the habit and the record at once. Starting target: 4, the weekly goal.

| Metric | Definition | Why it matters | Starting target (a guess) |
|---|---|---|---|
| Spin to post | Share of spins that end in a post that day, late posts included | The core loop | 60% or more |
| Weekly goal | Share of active people with 4 or more posted days in a week | The habit | 40% or more |
| Retention | Share of a week's new people still spinning in week 2 and week 4 | Does it last | 60% in week 2, 40% in week 4 |
| Skips by category | Skips divided by spins, for each category | A category skipped a lot is a bad starting fit | Look at any category above 30% |
| Board pull | Share of posts followed by a Board visit within 10 minutes | Is the lock a reason to post or a wall | 70% or more |
| Request acceptance | Share of follow requests accepted within 7 days | Cold start | 80% or more |
| Circle size | Median number of people each person follows after week 1 | Cold start | 3 or more |
| Late share | Share of posts made late | Is the window right | Watch only |

**Guardrails, also starting guesses.** Reports, blocks and unfollows per 100 active people each week, and how long a report waits for an answer. The target for that wait is an open question (section 16).

**How to measure without trackers.** Most metrics above can be computed from tables that already exist, such as spins, posted days, goal weeks and follows. Three need new records on the server: Board visits for Board pull, declined requests for request acceptance, and unfollows. Today a decline, an unfollow and an unblock delete their rows. No outside analytics tool is needed.

## 14. Release plan

### Phases (all built by directing Claude Code, walked by me on the iPhone Simulator)

| Phase | Scope | Dates | Status |
|---|---|---|---|
| 1 | Accounts and sign-up | Sep 15 to Oct 9, 2026 | Closed |
| 2 | The daily spin | Oct 6 to Oct 13 | Closed |
| 2.1 | Settings | Oct 10 to Oct 13 | Closed |
| 2.2 | Smoother navigation | Oct 13 | Closed |
| 3 | The daily post | Oct 9 to Oct 16 | Closed |
| 4 | Profile and the post history | Oct 10 to Oct 16 | Closed |
| 5 | Following and the Board | Oct 10 to Oct 18 | Closed |
| 6 | People pages, reporting and the admin page; also Forgot password, phone notifications, the 8 PM reminder, Terms and Privacy | Oct 10 to Oct 25 | Built; Terms and Privacy drafted Oct 25; my final walkthrough is still to do |

Dates run from the written scope to the close. Phase 6 was built from Oct 17.

Phases 1, 2, 2.1, 3, 4 and 5 each had a written scope, a screen-by-screen design spec, a build plan and a test checklist in [`.planning/`](./.planning). Phase 2.2 had a scope and a plan. Phase 6's paper covers the person page, reports and the 8 PM reminder; the rest of Phase 6 was built from my instructions without separate paper.

### Launch checklist

**Legal and accounts**

- [ ] A lawyer reviews the Terms of Use and the Privacy Policy, including which law applies.
- [ ] A real contact email for the Terms and the Privacy Policy, and the same address in the server's CONTACT_EMAIL for the second-look email.
- [x] Account deletion in the app (Apple requires it for apps with sign-up). Built October 25, 2026.
- [ ] Record the accepted terms version, and tell people when the terms change.
- [ ] Verify email addresses at sign-up.

**Infrastructure**

- [ ] A domain and HTTPS, with links that open the app directly (invite links).
- [ ] Real email: a Resend account and a verified sender address.
- [x] Photo storage in a private Supabase bucket in Singapore, with photo links that stop working within an hour.
- [ ] Strip photo metadata for every file type.
- [ ] Trust the real proxy for client IP addresses.
- [ ] Backups (a paid Supabase plan, or my own encrypted copies), log retention, and cleanup of unused photos and old login records.
- [ ] Monitoring and alerts for the server.
- [ ] Test tools off, admin accounts set.

**Notifications**

- [ ] Apple Developer Program, an APNs key, an Expo project, push switched on, then tested on a real iPhone.

**Product fixes:** the known issues in section 16.

**Quality**

- [ ] Device walkthroughs: Reduce Motion, VoiceOver, offline while the app is open, the first-run sign-up page, the bonus-spin sheet, the late state, and wheels of 2 to 12 on screen.
- [ ] App Store privacy labels and the privacy manifest, and remove the unused microphone and Face ID permission text that plugins add.
- [ ] Continuous integration.

**Moderation**

- [ ] Decide who reads reports and how fast.
- [ ] Alerts for new reports.

**Name**

- [ ] Check the RNDMRoll name.

## 15. Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Cold start: a new person follows nobody and the Board is empty | High | High | First spin at once; Find your friends; invite links; suggestions; an empty Board that explains itself; track circle size |
| Posting feels like work, so people skip | Medium | High | One page, tap-to-rate, an optional emoji and note, library photos; track spin to post |
| The category doesn't fit the day | High | Medium | Free skip; weekly goal of 4; "lately" counts; track skips by category and adjust the starting wheel |
| The lock feels like a wall, not a reason | Medium | High | Show who posted (count and first names) to pull people in; late posts still open it; track Board pull |
| Harm to younger users | Medium | High | 13 and over only; every account private; report people, posts and comments; block from a person's page, search and the follow lists; mute from a person's page; admin page; a lawyer review before launch |
| Photo privacy: a link already given out keeps working for a while after access ends, and some files keep metadata | Low | Medium | Private bucket and links that stop working within an hour; disclosed in the Privacy Policy; metadata removal is on the launch checklist |
| Moderation load on one admin | Low at first | Medium | One card per reported thing; disable in one step; email alerts; a post or comment 3 people report is hidden until an admin looks |
| Three people hide a post that breaks no rule, until an admin looks | Low | Medium | The admin is emailed at once when something is hidden; Dismiss shows it again and tells the owner; the owner is told in the bell and sees the line on it meanwhile |
| One developer: data loss or downtime | Medium | High | Backups and monitoring on the launch checklist; strong test suite. The Supabase free plan has no backups, so lost data and deleted photos can't be restored |
| Name clash with an existing app called "Roll: Fresh film daily" | Low | Medium | Check the name before launch (an open question in section 16) |

## 16. Known issues and open questions

### Known issues

Some were found while writing this document; others were already in the planning notes. None blocks the demo, and all are on the launch list.

1. **Invite links don't survive signing in.** A link opened while signed out is lost; after sign-up or log in, the inviter's profile doesn't open.
2. **A photo link already given out keeps working until it runs out.** After a block, a removed follower, an archive, a report, a hide under review or an admin's Remove, a link a phone already has stops working within an hour, not at once.
3. **Some photo files are never removed from storage.** This covers uploads never posted and any file whose one removal try failed. The server makes no new links for them.
4. **Some photos keep their metadata.** Library WebP files, and some profile photo formats, are uploaded with their original details, which can include location.
5. **Search words appear in server logs.**
6. **Sign-up shows whether an email is registered.** One address can try 10 sign-ups a minute, and 3 a minute for the same email.
7. **An old access token works for up to 15 minutes** after logging out, changing the password or resetting it.
8. **The locked Board's count can be off.** It can include people whose post was deleted, archived, removed, hidden under review or reported by you, while the first names come only from posts you can see.
9. **Reactions on posts left unused pieces.** The removed feature's server route and table still exist.
10. **Some of your own comments can't be reached.** After an unfollow, being removed as a follower, a block, an archive, or your own report of the post or of the comment you replied to, your comment there stays, and the app has no list of your comments to delete it from.
11. **Reinstalling the app can bring back an old login.** iOS keeps keychain items after an app is deleted, and the app doesn't clear them on a fresh install.
12. **After a big time zone change, the countdown's line can say the wrong time.** When a move puts 8 PM less than 12 hours after the last spin, the wheel opens when the 12 hours end, such as 9:50 PM. The countdown counts to that moment and the wheel opens on time, but the line under it still reads "Opens at 8:00 PM."
13. **A Dismiss from an old admin page can close a newer person report.** When the same person reports the same account again after the page was drawn, Dismiss on the old card closes the new report too. A post or comment can't be reported twice by the same person, so only reports of people are affected. The Reports tab redraws every minute; a person's page doesn't.
14. **Any 3 people who can see a post can hide it until an admin looks.** The same goes for a comment. The admin is emailed at once, and Dismiss shows it again.
15. **A reporter who leaves can show a hidden item again with no review.** A report stops counting once its reporter's account is disabled or being deleted, and goes when that account is deleted. If fewer than 3 open reports are left, the post or comment shows again, and its owner isn't told.
16. **The words outlive a removal.** After the 30 days, a removed post's title or a comment's text stays in its reports until the reporter's or the owner's account is deleted, and in the decision log until the owner's account is deleted.
17. **The address in the second-look email is a server setting.** It comes from the server's CONTACT_EMAIL, apart from the address in the Terms and the Privacy Policy, so the two must be set to match. Until it is set, the removal message still shows, but tapping it does nothing.
18. **Enable can show hidden posts again with no review.** Disable closes every open report about the person and their posts and comments. After Enable, anything of theirs that was hidden under review shows again, with no review and no bell item.
19. **A comment can come back without a word to its writer.** If a comment is dismissed while its post is still hidden, it shows again once the post's own reports are dismissed. The post's owner is told, and the comment's writer isn't.
20. **For up to an hour, the Removed tab can offer a comment it can't bring back.** Once a removed post passes its 30 days, a removed comment under it shows "0 days left" until the next hourly purge. Bring it back on that comment says "Bring back its post first.", though the post can no longer come back.
21. **A hidden note can be left without a back note.** The owner is told when a report hides their post or comment, and again only when a Dismiss shows it again or Bring it back returns it. If it shows again another way, nothing is sent: a reporter's account is disabled or being deleted, Disable and Enable (issue 18), or a comment under a hidden post (issue 19). The same way, a reporter's account that comes back can hide it again with no note.

### Open questions

- Who moderates after launch, and how fast should reports be answered?
- Should people choose their reminder time?
- Should skipped days appear on the profile? The server already returns them.
- Should a person's first, partial week count differently for the week streak? The data to do it is already stored.
- Is the RNDMRoll name clear to use?
- Is Android in scope after the iPhone launch?

## 17. Decision log

These are the product decisions I made along the way, with the reason and the cost.

| Date (2026) | Decision | Why | Trade-off |
|---|---|---|---|
| Sep 15 | A spin wheel, not a dice roll | A wheel shows every option before it lands and can be edited; a die tops out around six faces | Can read like a prize wheel, so it's kept flat and black and white |
| Sep 15 | Once a day, no re-spins on demand | Scarcity keeps it a ritual, like Wordle | None recorded |
| Sep 15 | Each person's own 8 PM | Nobody should spin at 3 AM to keep up with friends abroad | Less of BeReal's "everyone at once" surprise |
| Sep 15 | Camera or library photos | Log the book you finished two nights ago | No guarantee the photo is your own |
| Sep 15 | Half-star ratings | Whole stars are too coarse | None recorded |
| Sep 15 | Success means daily use by a small group | Prove the habit before growth | Growth waits |
| Sep 15 | Domine and Work Sans | A considered look that doesn't read as templated | None recorded |
| Sep 16 | A light, monochrome editorial look | Chosen over a dark theme after seeing the brief | None recorded |
| Oct 4 | My own wheel logo, and "spin" instead of "roll" | None recorded | None recorded |
| Oct 4 | One Welcome page | The pitch pages weren't useful | None recorded |
| Oct 5 | Forgot password with a 6-digit code before launch (built Oct 21) | Needed before any launch | Real email needs a provider |
| Oct 5 | Terms of Use and a Privacy Policy before launch (drafted Oct 25) | Needed before any launch | Need a lawyer's review before launch |
| Oct 6 | No email code at sign-up | Only a password is needed to sign up | Emails aren't verified (a launch item) |
| Oct 7 | Daily, not weekly | Weekly would mean a quiet feed and no habit | None recorded |
| Oct 7 | A weekly goal of 4 instead of a daily streak, a free skip and one bonus spin | "Book" on a day you read nothing shouldn't break anything | Weaker pressure than a strict daily streak |
| Oct 7 | Open 8 PM to midnight; late until the next 8 PM | Life happens | A softer day boundary |
| Oct 8 | Two-page sign-up, photo and bio second | Like Instagram, the photo gets its own page after the account details | None recorded |
| Oct 8 | A date scroller and green "Looks good." notes | Less typing | None recorded |
| Oct 9 | One password, typed once; log in with email or username | Two passwords were confusing; Instagram asks once | None recorded |
| Oct 9 | Apple and Google sign-in removed | Email and password are enough for a friend group; two fewer native SDKs | No one-tap sign-in |
| Oct 9 | Movie and book days accept anything watched or read lately | The skip is for days with truly nothing | None recorded |
| Oct 9 | A deleted post still counts | None recorded | No second post that day |
| Oct 9 | Photos up to 50 MB | The same limit as profile photos, so a full-size library photo saves | None recorded |
| Oct 10 | Equal slices and equal chances | Uneven slices looked wrong, and people don't need the odds | The app offers no way to make a category more likely |
| Oct 10 | 1 to 10 photos per post, the first is the cover | None recorded | None recorded |
| Oct 10 | No friends' posts after a skip | One rule for everyone: post to see | None recorded |
| Oct 10 | Report a post or a comment | None recorded | None recorded |
| Oct 10 | Frosted glass cards dropped | Plain cards match the pictures | None recorded |
| Oct 10 and Oct 19 | The 8 PM reminder runs on the phone, and is asked for at the start | It needs no server and no Apple push key | None recorded |
| Oct 13 | One-way following | Works like Instagram | None recorded |
| Oct 13 | Tabs swap at once; the cross-fade was dropped | It looked wrong on the phone | None recorded |
| Oct 16 | Three tabs: Spin, Board and Profile | Home repeated Spin; Profile shows every post | None recorded |
| Oct 16 | No music on posts | A song screenshot needs no music service | None recorded |
| Oct 16 | Every account private, every follow a request | Small trusted circles, safer for younger users | Slower to find people |
| Oct 16 | The wheel stays on the page at rest | The wheel should always be visible | None recorded |
| Oct 18 | Reactions on friends' posts removed | They repeated what comments do | The unused route and table remain |
| Oct 18 | No underlined text used as a button | Underlined text reads as unfinished | None; the last old links were restyled Nov 13 |
| Oct 18 | An admin page to review reports and disable accounts | None recorded | None recorded |
| Oct 19 | A new person's first spin opens at once | A newcomer sees the loop on day one | No second spin at 8 PM that day |
| Oct 19 | Find your friends, invite links and suggestions | It should work like sending an invite link | None recorded |
| Oct 21 | The Shutter logo as the app icon | Chosen from options I asked for | None recorded |
| Oct 21 | A portfolio release first | Show the work now; real accounts and keys later | Not public yet |
| Oct 25 | Account deletion in the app, like Instagram: hidden at once, deleted after 30 days, logging back in keeps it | Apple requires it, and the 30 days protect against a mistake | The account's data stays 30 more days |
| Oct 28 | Late from noon the next day instead of midnight | Four hours to post on time were too few | A softer nudge to post the same night |
| Oct 28 | After midnight it is still "Today's spin", with a 10 AM note for a spin not yet posted and a line when a day closes without a post; the week row and the date follow the spin's day too; "Posted." and "Skipped." stay until the next 8 PM, the posted page points to the Board, and an evening with no spin is mentioned | A spin's day runs from 8 PM to the next 8 PM, so "Last night's spin" read like a new day | None recorded |
| Nov 8 | Database and photos on Supabase in Singapore; photos private, with links that stop working within an hour | Only people allowed to see a photo get a link, like Instagram's expiring links | No backups on the free plan; data kept outside the country of most users |
| Nov 16 | Forms say what is wrong in a few words, in ink with an alert mark, and a grey button shows the problems when tapped | A grey Continue that did nothing on a tap gave no reason | None recorded |
| Nov 16 | The Spin page shows a whole blank wheel while it loads, and starts loading during sign-up | A bare circle looked broken, and the first Spin page after sign-up was slow | None recorded |
| Nov 17 | A reported post or comment is gone for the reporter for good, and 3 open reports hide it from everyone but its owner until an admin looks | None recorded | Any 3 people who can see a post can hide it until an admin looks |
| Nov 17 | Remove keeps a post or comment for 30 days, with Bring it back; every decision goes in a log; second looks are by email | "We need to have this stored somewhere": the log is what lets me answer an email | Removed content stays 30 more days; no appeal button in the app |
| Nov 18 | The owner is told in the bell when 3 reports hide their post or comment | The owner should know it's under review, and "Sorry, we got that wrong" shouldn't come out of nowhere | One more bell item, in the bell only |
| Nov 18 | Remove asks for a reason, and the owner's message names it | "It would be reasonable if we actually answer them why we are removing it" | One more choice at Remove; the reason is one of the 7 report reasons, with no typed note |
| Nov 18 | The person page reads like a profile, with a back arrow and their posts while their account is reported or disabled; the admin page uses the width of the screen | "When there's like an open account thing, I should be able to see that person's profile page" | Admins see more of a reported account than the reported posts |

## 18. Out of scope and future ideas

**Left out on purpose.** Unlimited re-spins, live-camera-only posts, music on posts, Apple and Google sign-in, and reactions on friends' posts. The moderation pieces left out on purpose are listed in section 8.10.

**Future ideas (not ranked).**

1. **Year in review.** Top categories, average rating per category, longest streak and most-rated title.
2. **Taste insights**, such as "you rate meals higher on weekends".
3. **A shareable result card** without the photo or title, for outside the app.
4. **Small groups with their own wheel** and evening time.
5. **Four favourites** pinned on the profile.
6. **GIFs in comments**, and editing or reacting to comments.
7. **Notification grouping**, and choosing the reminder time.
8. **Starter category packs**, earned free with streaks and never sold.
9. **Restoring a broken streak** under clear limits.
10. **A short proof clip** on a post.
11. **Account features:** Face ID, two-factor sign-in, changing the email, and a list of sessions.

## Appendix A: Glossary

| Term | Meaning |
|---|---|
| Spin | The once-a-day turn of the wheel that picks today's category |
| Wheel | A person's own list of 2 to 12 categories |
| Post | One real thing for the day's category: photos, title, rating, optional emoji and note. Called an "entry" in the code. |
| Posted day | A date with a post; it stays posted if the post is deleted |
| Skip | Passing on a day for free; final for that day |
| Late | Posted from noon the day after the spin until the next 8 PM |
| Board | The friends' page, locked until you post for the day in play |
| Day in play | The date the Board's lock is about: the date of the most recent 8 PM that has passed, or today on a new person's first day |
| Weekly goal | 4 posted days, Monday to Sunday |
| Bonus spin | An extra spin earned by the weekly goal, used to swap today's word |
| Week streak | Weeks in a row that met the goal |
| Archive | Hiding your own post from everyone else, restorable |
| Mute | Hiding someone's posts from your Board, silently |
| Block | Cutting all contact both ways, silently |
| Admin | An account allowed into the admin page to review reports |
| Hidden while we take a look | A post or comment with 3 or more open reports: only its owner and the admin page see it, until an admin decides |
| Removed | Taken down by an admin: hidden from everyone, its owner included, and deleted for good after 30 days unless it is brought back |
| Decision log | The admin's record of each Remove, Bring it back, Dismiss that shows something again, Disable and Enable, with when |

## Appendix B: API overview

All routes below start with `/v1` unless they are web pages. Limits are per person per minute for signed-in routes and per IP address for public ones; a route with no limit says so.

| Area | Routes | Limits |
|---|---|---|
| Sign-up and log in | `POST /auth/signup`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/password/forgot`, `/auth/password/verify`, `/auth/password/reset`; `GET /usernames/suggest`, `/usernames/available` | Sign-up 10 per IP, then 3 per IP and email; log in 20 per IP, 5 per IP and name, 5 per IP and account; refresh and log out 60 per IP each; forgot, verify and reset 10 per IP and 5 per IP and email, shared; usernames 90 per IP |
| Me | `GET`, `PATCH /me`; `POST /me/avatar/upload-url`; `POST /me/password`; `POST /me/delete`; `GET`, `PATCH /me/notification-settings`; `GET`, `POST /me/blocks`; `DELETE /me/blocks/:user_id`; `GET /me/archive`; `GET /me/summary`; `POST`, `DELETE /me/push-tokens` | Profile edit 30; password 5 per person and 20 per IP; delete 5 per person; `GET /me` and the photo upload link have no limit |
| Spin | `POST /time-zone` (its answer carries the Spin page's state too); `GET /today`; `POST /spin`, `/spin/bonus`, `/spin/skip`; `GET`, `PATCH /wheel` | Spin, bonus and skip 10; today 60; wheel save 20 |
| Posts | `POST /entry-photos/upload-url`; `POST /entries`; `GET`, `PATCH /entries/:id`; `POST /entries/:id/archive`, `/unarchive`, `/delete`; `GET /diary` | Post 10; edits 20 |
| People | `GET /people/search`, `/people/lookup`, `/people/suggestions`, `/people/:user_id`, `/followers`, `/following`; `POST /people/:user_id/mute`, `/unmute`, `/report` | Search 120; person reports 10 |
| Follows | `GET`, `POST /follows`; `POST /follows/:user_id/unfollow`; `POST /follows/requests/:id/accept`, `/decline`; `POST /followers/:user_id/remove` | 60 to read; 20 for each change |
| Board and comments | `GET /board`; `GET /posts/:id`; `POST /posts/:id/comments`, `/report`; `POST /comments/:id/delete`, `/hide`, `/unhide`, `/report`; `POST /posts/:id/reaction` (unused) | Comment 20; post reports 10 and comment reports 10, counted apart; reaction 60 |
| Notifications | `GET /notifications`, `/notifications/summary`; `POST /notifications/seen` | 60, 120, 30 |
| Web pages | `GET /i/:username` (invite page); `/admin` (admin page; the Removed tab is `GET /admin/removed`, and Bring it back is `POST /admin/removed/restore`); `GET /healthz` | Admin log in 5 per IP, sharing the app's per-account limit; the others have none |

Test-only routes under `/test` exist only when the test tools are on.
