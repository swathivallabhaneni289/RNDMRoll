# Phase 6: as built (2026-10-17 to 2026-10-25)

Written 2026-11-13 from PRD.md and the git log. **PRD.md is the current record**: every requirement with its status (section 8), the launch checklist (section 14) and the known issues (section 16). The other papers in this folder were written before the build and are history.

**Status:** built. The developer's walkthrough of Phase 6 (a practice run) is still to do, and ROADMAP.md ticks the phase after it.

**How it was built:** by directing Claude Code. The person page, reports and the 8 PM reminder followed `06-FOLLOW-OVERRIDES.md`; the rest was built from the developer's instructions without separate paper.

## What was built

| Part | PRD | Migration | Main commits |
|---|---|---|---|
| Person page: photo, name, @username, bio and the Posts, Week streak, Followers and Following figures; posts for accepted followers, on days they posted too; three dots with Remove follower, Mute, Report and Block; a blocked person's page shows Unblock | PROF-03 | | `5b5f724` `3b8e089` `fb2f1b5` `23e179d` `c37fe28` `67f8d33` |
| Mute a person, report a person | SOC-07, SAFE-01 | 0010 | `db1545d` `2a7e4a1` |
| Hide a comment on your own post | CMT-04 | 0011 | `3ad9541` `2e25fde` `b5bfb57` |
| Report a post or a comment; the admin page at `/admin`: reports grouped by person, remove a post or comment, disable or enable an account | SAFE-01, ADM-01 to ADM-04 | 0012 | `22692fe` `d2fb9a8` `8e15f72` |
| A photo in a comment or reply | CMT-02 | 0013 | `45411ad` `f7e47fc` |
| The 8 PM reminder, made on the phone, asked for once at the start, with its switch in Settings > Notifications | SPIN-11, NOT-03 | | `9a56c45` `05a109d` `e45bcb1` |
| A brand-new person's first spin opens at once | SPIN-02 | | `75120e8` `75088d1` `9f0b35d` |
| Find your friends after sign-up, invite links and suggestions | ACC-05, SOC-04, SOC-05 | | `150533b` `ffb0315` `4efb1de` `8ad5f37` |
| Forgot password with a 6-digit code; real email is off, so codes print to the server log | ACC-07 | 0014, 0016 | `ac5c292` `44ebc13` `5ab2cf8` |
| Phone pushes for comments, replies, follow requests and accepted requests; built and off (`docs/PUSH-SETUP.md`) | NOT-04 | 0015, 0017 | `c228224` `5ab2cf8` |
| Terms of Use and Privacy Policy in the app and in `docs/legal`; drafts, with a lawyer's review on the launch checklist | ACC-08, SET-02 | | `899aa2b` |
| Delete account: hidden at once, deleted for good after 30 days unless the person logs back in | ACC-10 | 0018 | `b2e2dc4` |

## After the phase

Later changes are in PRD.md section 17, the decision log. The largest: a post is late from noon the next day (2026-10-28, `1e00f6b`); `make admin` and `make password` (2026-10-29, `cb53c5a` `63da5c9`); the database on Supabase in Singapore (2026-11-09); private photos behind links that stop working within an hour (2026-11-13, migration 0019, `9c1568c`); and Forgot password checks the code before the new password (2026-11-13, `8588650`).

## Not built

- GIFs in comments (PRD CMT-06, Later: it needs a GIPHY key).
- What PRD.md section 14 lists for launch, and the known issues in section 16.
