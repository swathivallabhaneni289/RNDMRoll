-- Push notifications when the app is closed (comments, replies, follow
-- requests, accepted requests). One row per phone: the Expo push token the
-- app got after sign-in. A token belongs to one account at a time; signing in
-- on the same phone as someone else moves it. Sending is off until the
-- server runs with PUSH_DRIVER=expo (docs/PUSH-SETUP.md).
create table push_tokens (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users (id) on delete cascade,
    token text not null unique,
    platform text not null,
    created_at timestamptz not null default now(),
    last_seen_at timestamptz not null default now(),
    constraint push_tokens_platform check (platform in ('ios', 'android')),
    constraint push_tokens_token_len check (char_length(token) between 1 and 200)
);
create index push_tokens_user_idx on push_tokens (user_id);
