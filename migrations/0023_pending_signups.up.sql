-- Sign-up emails a 6-digit code (developer, 2026-11-24: "lets add it", after
-- asking for sign-up to "work like any other website"): the account is made
-- only once the code is typed into the app, and it is made verified. Until
-- then the sign-up waits here, with everything the account will be made with.
--
-- pending_signups: one row per sign-up started. The app holds a token whose
-- SHA-256 is token_hash; the token itself is never kept. Only an HMAC of the
-- code is kept, bound to the row and its email, so a code finishes only its
-- own sign-up. A code lives 30 minutes; a sign-up allows 5 wrong tries across
-- all its codes, and at most 5 codes (the first and 4 more). The code a newer
-- one replaced is kept, so typing it is told apart and costs no try. When the
-- account is made, every row of its email goes; the purge sweeps the others
-- once their code has expired and they are an hour old, since the cap of 5
-- sign-ups an hour per email counts them until then. No row holds the email
-- or the username for anyone: the account's own unique indexes decide, when
-- it is made.
create table pending_signups (
    -- Made by the server before the insert: the code is bound to it.
    id uuid primary key,
    token_hash bytea not null unique,
    -- The account to make, as the sign-up page sent it and the server
    -- checked it. The password is a bcrypt hash, as in users.
    email text not null,
    password_hash text not null,
    name text not null,
    username text not null,
    bio text,
    birthday date not null,
    -- The live code and the one it replaced, if any.
    code_hash bytea not null,
    previous_code_hash bytea,
    -- When the live code expires, and when it was made (a new code is not
    -- made within a minute of the last).
    expires_at timestamptz not null,
    code_created_at timestamptz not null,
    codes_sent int not null default 1,
    attempts int not null default 0,
    -- When the sign-up started: the hourly cap and the sweep count it.
    created_at timestamptz not null
);

-- The sign-ups of an email: the hourly cap counts them, and making the
-- account deletes them.
create index pending_signups_email_idx on pending_signups (lower(email), created_at);
-- The sweep of codes that expired.
create index pending_signups_expires_idx on pending_signups (expires_at);
