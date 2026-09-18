-- Phase 1 schema: users, email_verification_tokens, refresh_tokens.

create table users (
    id uuid primary key default gen_random_uuid(),
    email text not null,
    password_hash text,
    name text,
    username text,
    bio text check (length(bio) <= 160),
    avatar_url text,
    email_verified boolean not null default false,
    email_verified_via text check (email_verified_via in ('password_flow', 'apple', 'google')),
    apple_subject text,
    google_subject text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint users_username_charset check (username ~ '^[a-z0-9_]{3,20}$')
);

-- Case-insensitive uniqueness: a plain UNIQUE on the raw column would let
-- "Swathi" and "swathi" (or "A@X.com" and "a@x.com") coexist as distinct rows.
create unique index users_email_lower_idx on users (lower(email));
create unique index users_username_lower_idx on users (lower(username)) where username is not null;
create unique index users_apple_subject_idx on users (apple_subject) where apple_subject is not null;
create unique index users_google_subject_idx on users (google_subject) where google_subject is not null;

create or replace function set_updated_at() returns trigger as $$
begin
    new.updated_at = now();
    return new;
end;
$$ language plpgsql;

create trigger users_set_updated_at
    before update on users
    for each row
    execute function set_updated_at();

-- Verification tokens are stored hashed with an expiry and a consumed
-- marker so a link cannot be replayed indefinitely.
create table email_verification_tokens (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    token_hash bytea not null unique,
    expires_at timestamptz not null,
    consumed_at timestamptz,
    created_at timestamptz not null default now()
);

create index email_verification_tokens_user_idx on email_verification_tokens (user_id);

create table refresh_tokens (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    token_hash bytea not null unique,
    expires_at timestamptz not null,
    revoked_at timestamptz,
    rotated_to uuid references refresh_tokens(id),
    user_agent text,
    created_at timestamptz not null default now()
);

create index refresh_tokens_user_idx on refresh_tokens (user_id);
