# 01-18 research: an Instagram-style sign-up (2026-10-05)

Status: research only. The developer asked on 2026-10-05 for account creation to work "exactly like Instagram" and has NOT yet answered the open decisions (section 4). Two read-only web researchers produced this (workflow `wf_7deebd1f-e50`); their tool calls were audited afterwards: web search, web fetch and downloads into the scratchpad only, no repo file touched.

## 1. Instagram's sign-up, what could be verified

- The order differs by version. iOS, May 2022: country and phone, code, name, password, birthday, username, Facebook friends, contacts, photo, follow people, notifications. Android, July 2024 (the newest first-hand recording found): email, code, password, birthday, name, username, terms (I agree), then straight into the app, with no photo or contacts step before it. A December 2025 third-party article gives yet another order and is unreliable. No 2025 or 2026 first-hand recording was found, so expect A/B tests and regional differences.
- Verified from Instagram's Help Center and Meta: the sign-up code is 6 digits and is sent by email or SMS to what you entered; the minimum age is 13 in most countries (higher in some), under 13 is removed with an appeal route of up to 30 days, under 18 gets a private Teen Account, the birthday is not shown publicly; the password minimum is 6 (8 or more is advised); the account can be created in the app or on the website.
- Not verified from Instagram: the code's lifetime, the resend and change-email controls, the wrong-code messages, attempt limits, the username rules (third parties say 30 characters, letters, numbers, periods and underscores, case-insensitive), whether a taken username gets suggested alternatives, and whether the photo step still exists.
- Shape of every screen: one question, one primary Next button, a log-in link on the early screens; a save-login-info prompt (Save or Not now) at a version-dependent point.

## 2. One-time-code practice (recommended parameters for the Go API)

- Code: 6 decimal digits from a CSPRNG (`crypto/rand`), kept as a string so leading zeros survive. NIST SP 800-63B-4 3.1.3.2 and OWASP ASVS 5.0 6.5.x allow this.
- Lifetime 10 minutes. Wrong guesses: 5 per code, then the code dies and a resend is needed; plus an aggregate cap per address (about 15 failures per hour, then block for an hour) that a resend does not reset.
- Resend: 60 second cooldown per address, at most 5 sends per hour and 10 per day; the server enforces it (429 with Retry-After). Every resend invalidates the earlier codes in the same transaction. The UI must say the earlier email's code no longer works.
- Storage: never the plain code. Store HMAC-SHA256 with a server secret over (pending id, normalized email, code); compare with `hmac.Equal`. Bind it to the address and the pending sign-up. Increment attempts atomically in one SQL statement.
- Rate limits per address (the control that matters) and per IP (weak on mobile networks, never lock out on IP alone), plus a global ceiling on emails per minute.
- No account enumeration: the send-code answer is the same whether or not the address is registered; a registered owner gets a different email instead of a code.
- Do not create the `users` row at the email step. Keep a `pending_signups` row and an opaque pending token; create the user only after the code is verified and the final step is submitted, with the create call checking server-side proof, and a unique index on lowercased email.
- Email: subject "Your RNDMRoll code is 123456", a plain line with the code, "It expires in 10 minutes", "If you did not ask for this, ignore it". No link in the same email. SPF, DKIM and DMARC on the sending domain.
- Client: one input, not six boxes; `textContentType="oneTimeCode"` and `autoComplete="one-time-code"`. Apple documents autofill for SMS only; codes from Mail are filled by iOS 17 and later in some cases with a lag, Android has no documented email autofill, so typing and pasting must always work.
- Pitfalls to avoid: per-code limit with unlimited resends, read-then-write attempt counters, plain SHA-256 of a 6-digit code, trusting the client's "verified" flag, reusing this code as a login factor.

## 3. What it would mean for RNDMRoll (a draft, not a decision)

- The link flow goes away: no callback page, no Safari hop, no deep-link handling, and no need for universal links, the domain or the Apple account for sign-up (they stay relevant later for password reset and invites). The PROJECT.md universal-links row and its memory note must be corrected once the developer confirms.
- Replaced: walkthrough steps 8 to 15, the email form and the link-based verify screen, and the one-page Create your profile (D-05, revised 2026-09-17). Photo and bio would move to the Profile and Edit screens, which already have an Add a bio prompt.
- Kept: Welcome and the method chooser, username suggestion and availability, session handling, avatar upload, Apple and Google sign-in (whether they also ask birthday and username is open).
- Proposed screens after the method chooser: email, 6-digit code, password, birthday, name, username, agree, then into the app.
- Risks: routing guards are fragile (commits 255ca31, 61855e2, a4fffa5, cold-launch constraint), the existing 158 Go tests around signup and verify would be rewritten, and the work sits on top of an uncommitted pile (01-17 patch, TextField, DialAvatar, the rejected Profile draft) where `patch -R` already fails.

## 4. Open decisions, with Claude's picks (asked of the developer, unanswered)

1. Phone numbers as well as email? Pick: email only now; texting needs a paid service and another account.
2. A birthday step with the 13 or older check, as Instagram has? Pick: yes; it is a privacy choice.
3. Photo and bio later from the Profile instead of at sign-up? Pick: yes; this drops the one-page Create your profile.
4. OK to commit the tested fixes (steps 1 to 14) before any rewrite, leaving the rejected Profile draft out? Pick: yes.

## 5. Main sources

- https://www.facebook.com/help/instagram/155940534568753 (create an account), https://www.facebook.com/help/instagram/366075557613433 and /966909308115586 (age), https://about.fb.com/news/2024/09/instagram-teen-accounts/
- https://pageflows.com/post/android/onboarding/instagram/ (July 2024) and https://pageflows.com/post/ios/onboarding/instagram/ (May 2022)
- https://pages.nist.gov/800-63-4/sp800-63b.html, https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x15-V6-Authentication.md, the OWASP Authentication, Multifactor and Forgot Password cheat sheets
- https://developer.apple.com/videos/play/wwdc2018/204/ and Apple Developer Forums threads 738690, 810439, 755996
