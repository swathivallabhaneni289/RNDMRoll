-- Forgot password (developer, 2026-10-21): a 6-digit code is emailed and
-- typed into the app with the new password. Only an HMAC of the code is
-- stored. A code lives 30 minutes and allows 5 wrong tries; a new request
-- replaces any earlier code for the person.
create table password_resets (
    user_id uuid primary key references users(id) on delete cascade,
    code_hash bytea not null,
    expires_at timestamptz not null,
    attempts int not null default 0,
    created_at timestamptz not null
);
