-- Second looks in the app (developer, 2026-11-19): the owner of a post or
-- comment an admin removed can ask, once per removal and only inside the 30
-- days, for the admin to look again. They write why (1 to 500 characters)
-- and may add up to 3 photos as proof, screenshots too. The admin answers
-- with Bring it back or Keep it removed, and the owner's bell says which.
-- This replaces the email asking for a second look, so the bell no longer
-- carries an address.
--
-- second_looks: one row per request, kept while the owner's account exists,
-- so the person page's History still shows the words after the purge. Only
-- the photos go, 30 days after the removal whatever the answer (the purge's
-- second-look sweep, on the removal's own line).
create table second_looks (
    id uuid primary key default gen_random_uuid(),
    owner_id uuid not null references users(id) on delete cascade,
    -- The removal asked about: one request per removal. The log row lives as
    -- long as the account, as this does.
    removal_id uuid not null unique references moderation_events(id) on delete cascade,
    item text not null check (item in ('post', 'comment')),
    -- The post or comment id. It has no foreign key, so it outlives the
    -- purge, as moderation_events.item_id does.
    item_id uuid not null,
    -- The post, or for a comment its post, and the comment. Null once it is
    -- deleted: the row stays for the sweep and the History.
    entry_id uuid references entries(id) on delete set null,
    comment_id uuid references post_comments(id) on delete set null,
    -- Their words, trimmed.
    body text not null check (char_length(btrim(body)) between 1 and 500),
    -- Up to 3 photo files in the owner's own entries/ folder, each used by
    -- nothing else (checked when it is asked, as a comment's photo is). The
    -- sweep empties it once the files are deleted.
    photo_keys text[] not null default '{}' check (cardinality(photo_keys) <= 3),
    -- When it was asked on the owner's own clock, as moderation_events.at.
    at timestamptz not null,
    created_at timestamptz not null default now(),
    -- The admin's answer: null while it is open, restored (Bring it back)
    -- or kept (Keep it removed). A Bring it back after a Keep makes it
    -- restored.
    decision text check (decision in ('restored', 'kept')),
    decided_at timestamptz,
    check ((decision is null) = (decided_at is null))
);

-- A person's requests, for the History.
create index second_looks_owner_idx on second_looks (owner_id, created_at desc);
-- Is this file held by a request? (one file, one row)
create index second_looks_photo_keys_idx on second_looks using gin (photo_keys);

-- Keep it removed is a decision in the log too (kept), so the owner's bell
-- and the History read it as they read the others.
alter table moderation_events drop constraint moderation_events_action_check;
alter table moderation_events add constraint moderation_events_action_check
    check (action in ('removed', 'restored', 'shown_again', 'disabled', 'enabled', 'hidden', 'kept'));
