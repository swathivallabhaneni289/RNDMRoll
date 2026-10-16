-- Phase 5 schema: one-way follows, blocks, notification settings, reactions
-- and comments on posts, and the notification seen time. users and entries
-- are not altered.
--
-- Every table references users(id) on delete cascade, so deleting an account
-- (or truncating users with cascade, as the test cleanups do) clears them.
-- Reactions and comments also reference entries(id) on delete cascade, so a
-- deleted post takes them with it. Every timestamp is written from the Go
-- service clock (no defaults that read now()), so a moved test clock stays
-- coherent.

-- One row per (follower, followee). status 'pending' is a request to a
-- private account; 'accepted' is a follow. id is the request id the routes
-- and notification items carry. via says how the row became accepted:
-- 'direct' (a public account was followed at once; the followee gets a
-- "started following you" item), 'request' (the followee accepted; the
-- follower gets an "accepted your request" item) or 'silent' (the followee
-- switched to public, which accepts every pending request with no item).
create table follows (
    id uuid primary key default gen_random_uuid(),
    follower_id uuid not null references users(id) on delete cascade,
    followee_id uuid not null references users(id) on delete cascade,
    status text not null,
    via text,
    requested_at timestamptz not null,
    accepted_at timestamptz,
    constraint follows_pair_unique unique (follower_id, followee_id),
    constraint follows_not_self check (follower_id <> followee_id),
    constraint follows_status_check check (status in ('pending', 'accepted')),
    constraint follows_via_check check (
        (status = 'pending' and via is null and accepted_at is null)
        or (status = 'accepted' and via in ('direct', 'request', 'silent') and accepted_at is not null)
    )
);

create index follows_followee_idx on follows (followee_id, status, requested_at desc);
create index follows_follower_idx on follows (follower_id, status);

-- A block hides each person from the other. Making one deletes the follows
-- and requests between the two in the same transaction.
create table blocks (
    blocker_id uuid not null references users(id) on delete cascade,
    blocked_id uuid not null references users(id) on delete cascade,
    created_at timestamptz not null,
    primary key (blocker_id, blocked_id),
    constraint blocks_not_self check (blocker_id <> blocked_id)
);

create index blocks_blocked_idx on blocks (blocked_id);

-- The three switches of Settings > Notifications. No row means all three on.
create table notification_settings (
    user_id uuid primary key references users(id) on delete cascade,
    reactions boolean not null default true,
    comments boolean not null default true,
    milestones boolean not null default true,
    updated_at timestamptz not null
);

-- One reaction per person per post. The names are the same six as
-- entries_reaction_check (migration 0006).
create table post_reactions (
    entry_id uuid not null references entries(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    reaction text not null,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    primary key (entry_id, user_id),
    constraint post_reactions_name check (
        reaction in ('love', 'fire', 'laughing', 'wow', 'sad', 'sleepy')
    )
);

create index post_reactions_user_idx on post_reactions (user_id, updated_at desc);
create index post_reactions_entry_idx on post_reactions (entry_id, updated_at desc);

-- Comments: one level, plain text, 1 to 140 characters after trimming.
create table post_comments (
    id uuid primary key default gen_random_uuid(),
    entry_id uuid not null references entries(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    body text not null,
    created_at timestamptz not null,
    constraint post_comments_body_len check (char_length(btrim(body)) between 1 and 140)
);

create index post_comments_entry_idx on post_comments (entry_id, created_at, id);
create index post_comments_user_idx on post_comments (user_id, created_at desc);

-- When the person last opened the notification list. It only moves forward.
create table notification_seen (
    user_id uuid primary key references users(id) on delete cascade,
    seen_at timestamptz not null
);
