# Push notifications: what you need to do

The app can now send a phone notification while it is closed, for four things:

- someone comments on your post ("Alice commented: "nice"", or "Alice sent a photo.")
- someone replies to your comment ("Alice replied to your comment: ...")
- someone sends you a follow request ("Alice wants to follow you.")
- someone accepts your follow request ("Alice accepted your follow request.")

The code is all in place, but it is switched off. Until you do the steps below, nothing is sent and nothing breaks.

## What you need to provide

1. An Apple Developer Program membership (99 USD a year, developer.apple.com/programs).
2. An APNs key from Apple: the `.p8` file, its Key ID and your Team ID.
3. An Expo account (free, expo.dev).
4. The Expo project id in `app.json` (step 3 below writes it for you).
5. On the server: `PUSH_DRIVER=expo`. Optional: `EXPO_ACCESS_TOKEN`.

## Steps

1. **Join the Apple Developer Program** with your Apple ID, if you have not yet.

2. **Make the APNs key.**
   - Go to developer.apple.com, then Certificates, Identifiers & Profiles, then Keys.
   - Press +, give it a name, tick "Apple Push Notifications service (APNs)", then Continue and Register.
   - Download the `.p8` file. Apple lets you download it only once, so keep it safe.
   - Write down the Key ID (shown on the key) and your Team ID (top right of the page, or under Membership).

3. **Link the app to Expo.** In the project folder:
   - `npm install -g eas-cli` (once)
   - `eas login`
   - `eas init`
   - This adds `extra.eas.projectId` (and an `owner`) to `app.json`. Commit that change.

4. **Give Expo the APNs key.**
   - `eas credentials`, choose iOS, then the build profile you use, then "Push Notifications: Manage your Apple Push Notifications Key", then add a new key.
   - Point it at the `.p8` file and enter the Key ID and Team ID.
   - Or let EAS make and upload a key for you when it asks during `eas build`.

5. **Turn sending on at the server.** Set these where the server runs, then restart it:
   - `PUSH_DRIVER=expo` (the default is `off`, which sends nothing)
   - `EXPO_ACCESS_TOKEN=...` only if you turn on "Enhanced Security for Push Notifications" in the Expo project's settings. Make the token at expo.dev, Account settings, Access tokens.

6. **Run the new database steps.** Run them the same way as the others (`migrate ... up`), which runs all three in order:
   - `0015_push_tokens` adds the table `push_tokens`.
   - `0016_password_reset_previous_code` lets Forgot password say a code was replaced by a newer one.
   - `0017_push_token_sessions` ties each push token to the session (the refresh token) of the phone that sent it.

7. **Rebuild the app** on a real iPhone (`eas build` or a new dev client build). The project id has to be in the build. A push token cannot be made on the Simulator.

## How to test

1. Install the new build on a real iPhone, sign in, and say Allow when it asks about notifications. (If you said Don't Allow before, turn notifications on for RNDMRoll in the phone's Settings.)
2. Check that the phone's token arrived and is tied to its session: `select user_id, platform, last_seen_at, refresh_token_id is not null as has_session from push_tokens;` shows one row, with `has_session` true. A row with `has_session` false gets nothing until the app sends its token again (sign out and in once).
3. Close the app. From a second account, comment on a post of the first account, or send it a follow request.
4. The phone shows the notification within a few seconds. Tap it: it opens the post, the follow requests page, or the person's profile.
5. Quick check without the app: paste the token from the table into expo.dev/notifications and send a test.
6. If nothing comes, look in the server log for lines starting with `push:`.

## Good to know

- Comment and reply notifications follow the "Comments" switch in Settings > Notifications. Follow requests and accepted requests are always sent.
- Nobody is notified about their own comment, and nothing is sent between two people with a block.
- A phone's token follows the session it was sent from. Pushes go to it only while that session lasts, and stop on their own, even if the phone never reaches the server again, when:
  - the person logs out (the app also deletes the token),
  - the session runs out (30 days without opening the app),
  - the password is changed, with Change password or Forgot password (every other phone stops too),
  - an admin disables the account (enabling it again does not bring them back: each phone signs in again),
  - an old copy of the session is used again, as when a refresh answer never reached the phone or someone copied it (the session ends for both).
- Signing in as someone else on the same phone moves the token to that account. A late message from the old session never takes it back.
- The 8 PM reminder is separate. It is made on the phone itself and needs none of this.
