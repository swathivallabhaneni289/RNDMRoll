/**
 * The words of the Terms of Use and the Privacy Policy: the one copy the app shows
 * (components/legal/LegalScreen.tsx) and the one scripts/check-legal.ts writes to docs/legal as
 * Markdown for GitHub, so the two never drift. Written in plain words for RNDMRoll's first release
 * (developer, 2026-10-25) and to be read by a lawyer before a public launch.
 *
 * This file imports nothing, so the check script can load it with Node alone.
 */

export type LegalBlock = { kind: 'paragraph'; text: string } | { kind: 'list'; items: string[] };

export type LegalSection = { heading: string; blocks: LegalBlock[] };

export type LegalDocument = {
  title: string;
  /** The date this text took effect, as the page shows it. */
  effective: string;
  /** The opening paragraphs, before the first heading. */
  intro: string[];
  sections: LegalSection[];
};

/** Where people write to RNDMRoll: one place, so it is changed once. */
export const CONTACT_EMAIL = '';

const EFFECTIVE = 'November 17, 2026';

/**
 * The Privacy Policy's own date, since it can change without the Terms (it did for what admins see,
 * 2026-10-29 and 2026-10-30). Both changed for private photos on 2026-11-13, and for reports,
 * hiding and removals on 2026-11-17.
 */
const PRIVACY_EFFECTIVE = 'November 17, 2026';

function paragraph(text: string): LegalBlock {
  return { kind: 'paragraph', text };
}

function list(...items: string[]): LegalBlock {
  return { kind: 'list', items };
}

export const TERMS: LegalDocument = {
  title: 'Terms of Use',
  effective: EFFECTIVE,
  intro: [
    'These terms are the rules for using RNDMRoll, the app where you spin a wheel once a day and post one real thing for the category it lands on. They are an agreement between you and the developer of RNDMRoll ("we" or "us").',
    'By making an account, you agree to these terms. Please read them together with the Privacy Policy, which explains what we collect and why.',
  ],
  sections: [
    {
      heading: 'Who can use RNDMRoll',
      blocks: [
        paragraph(
          "You must be at least 13 years old. RNDMRoll asks for your birthday when you sign up and doesn't make an account if the birthday you give shows you're under 13.",
        ),
        paragraph(
          "If you are under 18, or under the age of adulthood where you live, you need a parent or guardian's permission to use RNDMRoll, and they should read these terms with you.",
        ),
        paragraph(
          "You can't use RNDMRoll if we have disabled an account of yours before, or if the law where you live doesn't allow you to.",
        ),
      ],
    },
    {
      heading: 'Your account',
      blocks: [
        list(
          "Use an email address that's yours and that you can read. It's how you reset your password.",
          "Keep your password to yourself. You're responsible for what happens on your account.",
          "Don't pretend to be someone else, and don't make an account for someone without their permission.",
          'If you think someone else got into your account, change your password in Settings. That logs your other phones out within 15 minutes.',
        ),
      ],
    },
    {
      heading: 'What you post',
      blocks: [
        paragraph('Your posts, photos, comments, profile and wheel are yours. You keep every right you have to them.'),
        paragraph(
          "To run RNDMRoll, we need your permission to store what you post, copy it, and show it to the people allowed to see it. By posting, you give us that permission. It's free, it applies worldwide, and we use it only to run RNDMRoll. It ends when the content or your account is deleted, except for the few copies the Privacy Policy says we keep, such as the words of a reported post.",
        ),
        paragraph(
          "Only post things you have the right to share. A photo of your own dinner is fine. Someone else's photo, or a picture of a person who didn't want to be posted, isn't.",
        ),
      ],
    },
    {
      heading: 'Who sees your posts',
      blocks: [
        paragraph(
          'Every account on RNDMRoll is private, and every follow is a request you can accept or decline. In the app, your posts are shown only to you and to the people whose request you accepted, and they see a day of your posts only after posting their own for that day. Admins can see a post or comment that someone reports, and the posts of an account while a report about it is open. Someone who reports a post of yours stops seeing it. Photo links stop working within an hour, as the Privacy Policy explains.',
        ),
        paragraph(
          'People who are signed in can find your account by your name or username, so friends can find you and send a request. What they see before you accept is described in the Privacy Policy.',
        ),
        paragraph(
          'You can remove a follower, mute someone or block someone at any time. Anyone who can see a post can still take a screenshot of it, so share with people you trust.',
        ),
      ],
    },
    {
      heading: 'Rules for everyone',
      blocks: [
        paragraph("RNDMRoll is for small groups of friends. Don't use it to:"),
        list(
          'Bully, harass, threaten or shame anyone.',
          'Post hate speech, or attack people for their race, ethnicity, religion, disability, sex, gender identity or sexual orientation.',
          'Post nudity or sexual content, or anything that sexualizes a minor.',
          'Post violence, threats or gore, or encourage anyone to hurt themselves or others.',
          'Sell or promote anything illegal, including drugs and weapons.',
          "Share someone's private information, such as their address or phone number, without their permission.",
          'Pretend to be someone else, or mislead people about who you are.',
          'Send spam, or make accounts in bulk or with automated tools.',
          "Get around RNDMRoll's limits, blocks or security, overload it, or collect other people's information from it.",
          "Break the law or someone else's rights, such as copyright.",
        ),
      ],
    },
    {
      heading: 'Reports and what we do about them',
      blocks: [
        paragraph(
          'You can report a person, a post or a comment. A post or comment you report disappears for you. The person you report is never told it was you.',
        ),
        paragraph('A post or comment that several people report may be hidden from everyone but its owner while we review it.'),
        paragraph(
          "We review reports. We may remove posts or comments, or disable accounts, that break these terms, put someone at risk or could cause legal trouble, and we may act without warning when that is needed to keep people safe. A post or comment we remove is hidden from everyone, its owner included. We keep it for 30 days in case we got it wrong, then delete it with its photos. A disabled account can't log in, and its profile, posts and comments are hidden from everyone else on RNDMRoll.",
        ),
        paragraph(
          `If you think we got a decision wrong, email us at ${CONTACT_EMAIL} and ask for a second look. If we removed a post or comment of yours, the message about it in the app opens that email for you, and we can bring it back within the 30 days.`,
        ),
        paragraph("Don't make a new account to get around a block or a disabled account."),
      ],
    },
    {
      heading: 'Leaving RNDMRoll',
      blocks: [
        paragraph(
          `You can stop using RNDMRoll at any time. You can delete your posts, and your comments on posts you can still see. A post or comment we removed is deleted for good 30 days later. To remove any other comment of yours, email us at ${CONTACT_EMAIL}. You can remove a follower whenever you like.`,
        ),
        paragraph(
          "To delete your whole account, go to Settings, then Delete account, and enter your password. Your account is hidden from everyone right away and deleted for good after 30 days, with everything in it. If you log back in before then, it won't be deleted. The Privacy Policy explains the few copies that take longer to clear.",
        ),
        paragraph('We may disable or delete an account that breaks these terms.'),
      ],
    },
    {
      heading: 'RNDMRoll is offered as it is',
      blocks: [
        paragraph(
          "RNDMRoll is a small app made by one developer. We work to keep it running and your posts safe, but we can't promise that it will always be available, that it will be free of mistakes, or that nothing will ever be lost. Features may change or end.",
        ),
        paragraph(
          "As far as the law allows, RNDMRoll is provided as it is, without warranties of any kind, and we aren't responsible for indirect losses, for what other people post, or for problems caused by things outside our control. Nothing in these terms takes away rights you have by law that can't be taken away.",
        ),
      ],
    },
    {
      heading: 'Changes to these terms',
      blocks: [
        paragraph(
          "We may update these terms as RNDMRoll changes. When we do, we'll change the date at the top, and for an important change we'll tell you before it applies, by email or in the app. If you keep using RNDMRoll after a change applies, you accept the new terms.",
        ),
      ],
    },
    {
      heading: 'Contact',
      blocks: [paragraph(`Questions about these terms? Email us at ${CONTACT_EMAIL}.`)],
    },
  ],
};

export const PRIVACY: LegalDocument = {
  title: 'Privacy Policy',
  effective: PRIVACY_EFFECTIVE,
  intro: [
    'This policy explains what RNDMRoll collects, why, who can see it and the choices you have. RNDMRoll is run by its developer ("we" or "us").',
    "In short: we collect what the app needs to work, the app shows your posts only to you and the followers you accept, and we don't sell your information. RNDMRoll has no ads and no analytics or tracking tools. Two limits are explained below: admins can see what is reported, including a reported account's posts, and when someone loses access to a photo, a link they already have stops working within an hour, not at once.",
  ],
  sections: [
    {
      heading: 'What you give us',
      blocks: [
        list(
          "Account details: your email address, password, birthday, name and username. Your password is stored only as a hash, a scrambled form we can't read.",
          'Your profile: a photo and a bio, if you add them.',
          'Your posts: the category your spin landed on, a title, a star rating, photos, and an emoji and a short note if you add them, with the time you posted.',
          'Comments and replies you write, and photos you add to them.',
          'Your wheel: the categories you add, rename or remove.',
          'Your spins: when you spun, what it landed on, and whether you posted or skipped that day.',
          'Your connections: who you follow, who follows you, follow requests, and the people you block or mute.',
          'Reports you make: who or what you reported and the reason you picked.',
          'Your notification settings.',
        ),
      ],
    },
    {
      heading: 'What comes from your phone',
      blocks: [
        list(
          'Your time zone, so your spin opens at 8 PM where you are. The app sends it each time you open it.',
          'Basic details your phone sends when you log in, such as the app and system version, saved with that login.',
          'If phone notifications are on, a push token: an address for sending notifications to your phone. It works only while the login it came from lasts.',
          "Whether each post photo came from the camera or your library. It isn't shown to anyone else.",
          'Your IP address, which reaches our server with every request.',
        ),
        paragraph(
          "RNDMRoll doesn't ask for your location or your contacts. It asks for your camera or photo library only when you add a photo, and for notifications so it can remind you at 8 PM and tell you about comments and follow requests.",
        ),
      ],
    },
    {
      heading: 'Photos and the details inside them',
      blocks: [
        paragraph(
          "Photo files can hold details your phone saved in them, such as when and where a photo was taken. RNDMRoll doesn't read these details. A photo you take in the app is saved again without them. A photo you choose from your library is usually saved again the same way, but some kinds of file, such as WebP images, are uploaded as they are, with any details they hold.",
        ),
      ],
    },
    {
      heading: 'How we use it',
      blocks: [
        list(
          'To run RNDMRoll: make your account, open your spin at 8 PM your time, save your posts and show them to the people allowed to see them.',
          'To count your week: posted days, the weekly goal, bonus spins and your week streak.',
          'To send what you asked for: the 8 PM reminder, notifications in the app, phone notifications when they are on, and password reset codes by email.',
          'To keep people safe: limit repeated tries, review reports, hide reported posts and comments, remove content or disable accounts that break the Terms of Use, and keep a record of those decisions.',
          'To find and fix problems.',
        ),
        paragraph(
          "We use your birthday to check that you're at least 13 when you sign up. It isn't shown to anyone, including you, and you can't change it in the app.",
        ),
        paragraph("We don't sell your information or share it for advertising."),
      ],
    },
    {
      heading: 'Who can see what',
      blocks: [
        list(
          'People who follow you: in the app, only people whose follow request you accepted can see your posts, their photos, the comments on them and your followers and following lists. They see a day of your posts only after posting their own for that day. Before they post, they can see that you posted that day and your first name, but not your post.',
          'Anyone signed in: anyone with an account can find you by name or username, and you may appear in their suggestions. They can see your name, username, profile photo and bio, and how many posts, spins, followers and following you have, and your week streak. They cannot see your posts.',
          'When you comment: everyone who can see the post sees your comment, with your name, username and profile photo.',
          'Reports: a post or comment you report is hidden from you from then on. A post or comment of yours that several people report is hidden from everyone but you while we take a look. We tell you in your notifications, and you see a line on it saying so. The people you report are never told it was you.',
          'Your posts show the time you posted, in your own time zone, and say Late when you posted late.',
          'Archived posts and their photos are hidden from everyone else in the app. A photo link someone already had stops working within an hour.',
          "If you block someone, they can't find you or see your profile, posts or comments, and you don't see their posts or comments. Photo links they already had stop working within an hour. You still see their name, username and profile photo in your Blocked accounts list and on their page, so you can unblock them.",
          "Admins, who review reports, can see each report and who made it, the reported post or comment, and the reported account's name, username, profile photo, bio and join date, and whether it is disabled or its owner asked to delete it. While a report about an account is open, or while it is disabled, they can also see its posts and their photos, but not the ones its owner archived. They can see a post or comment they removed, with its photos, for 30 days, and the record of decisions about an account, including how many of its posts and comments were removed before. They can open only accounts that someone reported, that are disabled, or that have such a record.",
          "We run RNDMRoll's server, so we can reach everything it stores, in our database and in our photo storage at Supabase. We look only when we need to run or fix RNDMRoll, keep people safe, or do something you asked, such as sending you a copy of your information.",
        ),
        paragraph('Other users and admins never see your email address or your birthday.'),
      ],
    },
    {
      heading: 'Photo links',
      blocks: [
        paragraph(
          'Photos are kept in private storage at Supabase. A photo opens only through a link from our server, and the server gives links only to people allowed to see that photo. Each link stops working within an hour. When someone loses access, for example after a block, a removed follower, an archived post, a report or a removal, the server stops giving them links at once, but a link they already have works until it runs out. Your profile photo is shown the same way to anyone signed in, except people you blocked. Someone who blocked you still sees it in their Blocked accounts list and on your page. People who viewed a photo may still have a copy on their phone, or a screenshot.',
        ),
      ],
    },
    {
      heading: 'Companies that help us',
      blocks: [
        paragraph('A few companies help run RNDMRoll. We give them only what they need to provide their service to us:'),
        list(
          'Supabase keeps our database and photos in Singapore, so it stores the information in this policy. Your phone sends and loads photos directly with Supabase, so it also sees your IP address. Our server runs on a computer we control, and its logs stay there.',
          'Resend sends password reset codes by email. It receives your email address and the code.',
          "Expo and Apple deliver phone notifications when those are on. They receive your push token, your phone's device token, the words of the notification, such as a friend's first name and the start of their comment, and the ID numbers the app uses to open the right page.",
        ),
      ],
    },
    {
      heading: 'On your phone',
      blocks: [
        list(
          "Your login is kept in your iPhone's keychain, on that phone only. Logging out removes it; deleting the app may not, so log out first. If you're offline when you log out, the app keeps it until it can end that login on our server.",
          'A few settings, such as whether the 8 PM reminder is on, are kept on your phone.',
          "The 8 PM reminder is scheduled on your phone. RNDMRoll's server isn't involved.",
          'Photos you view are saved on your phone so they load faster, and logging out clears them. Copies of photos you add can stay on your phone until you delete the app.',
        ),
      ],
    },
    {
      heading: 'Cookies',
      blocks: [
        paragraph(
          "The app doesn't use cookies. RNDMRoll's admin page, which only admins use, sets one cookie that keeps an admin logged in for 8 hours.",
        ),
      ],
    },
    {
      heading: 'How long we keep it',
      blocks: [
        list(
          'Your account and everything in it, for as long as your account exists. When you delete your account, it is hidden from everyone at once and deleted for good 30 days later, unless you log back in before then.',
          'When you delete a post, it is deleted with its photos and its comments, including photos in those comments. Your spin for that day, and the fact that you posted, stay because they count toward your week.',
          'When we remove a post or comment, it is hidden from everyone at once, you included, and kept for 30 days in case we got it wrong. Then it is deleted with its photos and with any comments or replies under it. Your spin for that day, and the fact that you posted, stay.',
          'Some photo files can stay in storage after what they belonged to is gone: photos you uploaded but never posted, and, rarely, a photo whose removal failed. The server makes no new links for these files, so any link to one stops working within an hour. We delete all of your photo files when your account is deleted.',
          'Archived posts stay, hidden from everyone else in the app, until you restore or delete them.',
          "Reports are kept after the reported content is removed, with a copy of the post's title or the comment's words, so we can look back at them. They're deleted when the account that made the report, or the account it's about, is deleted.",
          "A record of each decision we make about your posts, comments or account: what we decided and when, the post's title or the comment's words at the time, and the reasons it was reported. It is kept while your account exists, so we can answer you about a decision, and deleted with your account.",
          'A login ends 30 days after you last used the app. Records of past logins, with the details your phone sent, are kept while your account exists, to help protect it.',
          "A password reset code stops working after 30 minutes or 5 wrong tries. It's deleted once it's used, or replaced when you ask for a new one.",
          "Limits on repeated tries keep your IP address, and the email or username you typed, only in the server's memory, for a short time. Server logs record each request with its IP address and the part of the app it asked for, including any search words, and are kept only as long as we need them to fix problems and keep RNDMRoll safe. Supabase also keeps its own logs of requests to our database and photo storage for a short time.",
          'A disabled account is kept, hidden from everyone else on RNDMRoll, in case the decision is reviewed, until its owner asks us to delete it.',
        ),
      ],
    },
    {
      heading: 'Your choices',
      blocks: [
        list(
          'Change your photo, name, username and bio on your profile, and your password in Settings. You can replace your profile photo but not remove it yourself; email us to remove it.',
          'Delete your posts, and your comments on posts you can still see, or archive a post so no one else sees it in the app. A post or comment we removed is deleted for good 30 days later.',
          'Remove a follower, unfollow, mute or block anyone.',
          "In Settings, under Notifications, turn comment and milestone notifications and the 8 PM reminder on or off. Follow request notifications can't be turned off there. To stop all phone notifications, turn them off for RNDMRoll in your iPhone's Settings.",
          `Ask for a copy of your information, or ask us to correct it, by emailing ${CONTACT_EMAIL} from the address on your account. We'll answer within 30 days.`,
          `Ask for a second look at a post or comment we removed by emailing ${CONTACT_EMAIL}. The message about it in the app opens that email for you, and we can bring it back within 30 days of removing it.`,
          'Delete your account in Settings, under Delete account. It is hidden from everyone at once and deleted for good after 30 days, with everything in it, photos included. Logging back in before then keeps it. Copies in server logs are cleared on their normal schedule.',
        ),
        paragraph(
          'Depending on where you live, the law may give you more rights over your information, such as the right to complain to a data protection authority. Email us and we will help.',
        ),
      ],
    },
    {
      heading: 'Children',
      blocks: [
        paragraph(
          "RNDMRoll is not for children under 13. The app won't make an account if the birthday given shows the person is under 13. If you believe a child under 13 has an account, email us and we'll delete it.",
        ),
      ],
    },
    {
      heading: 'Security',
      blocks: [
        paragraph(
          'On our server, passwords, login tokens and reset codes are stored only as hashes. On your phone, your login is kept in the iPhone keychain. The token that keeps you logged in changes every time it is renewed, reset codes stop working after 30 minutes, and repeated tries are limited. Your information travels to our database and photo storage over encrypted connections, and Supabase encrypts it where it is stored. No system is perfectly secure, so if you think someone got into your account, change your password and email us.',
        ),
      ],
    },
    {
      heading: 'Where your information is kept',
      blocks: [
        paragraph(
          'Your account, posts and photos are stored with Supabase in Singapore. Resend, Expo and Apple, listed above, may handle what they receive in other countries. So your information may be kept in a country other than the one you live in.',
        ),
      ],
    },
    {
      heading: 'Changes to this policy',
      blocks: [
        paragraph(
          "We may update this policy as RNDMRoll changes. When we do, we'll change the date at the top, and for an important change we'll tell you before it applies, by email or in the app.",
        ),
      ],
    },
    {
      heading: 'Contact',
      blocks: [paragraph(`Questions about your privacy? Email us at ${CONTACT_EMAIL}.`)],
    },
  ],
};
