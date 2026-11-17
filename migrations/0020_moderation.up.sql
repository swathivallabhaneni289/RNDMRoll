-- Moderation (developer, 2026-11-17): a post or comment that several people
-- report is hidden while the admin takes a look, and the admin's Remove keeps
-- it for 30 days before it is deleted for good.
--
-- entries.removed_at and post_comments.removed_at: set by the admin's Remove,
-- on the database's clock. A removed post or comment is gone for everyone,
-- its owner included, but nothing is deleted yet: Bring it back clears it
-- inside the 30 days, and after them the purge deletes the row and its photos.
-- Null for everything else.
--
-- Hidden while under review is never stored. Every read works it out from the
-- open reports (social.ReviewThreshold of them, from accounts that are not
-- gone), so a dismissed report or a reporter who goes stops counting at once.
alter table entries add column removed_at timestamptz;
alter table post_comments add column removed_at timestamptz;

-- The Removed tab and the purge read the removed rows.
create index entries_removed_idx on entries (removed_at) where removed_at is not null;
create index post_comments_removed_idx on post_comments (removed_at) where removed_at is not null;

-- The open reports on one post or comment, counted on every read, and the
-- reports a deleted post or comment sets to null.
create index post_reports_entry_idx on post_reports (entry_id, resolved_at);
create index comment_reports_comment_idx on comment_reports (comment_id, resolved_at);

-- moderation_events: the decision log. One row for each thing the admin
-- decided about a post, a comment or an account, kept while the owner's
-- account exists, so the admin can answer an email about it later.
create table moderation_events (
    id uuid primary key default gen_random_uuid(),
    -- Whose post, comment or account it is.
    owner_id uuid not null references users(id) on delete cascade,
    item text not null check (item in ('post', 'comment', 'account')),
    action text not null check (action in ('removed', 'restored', 'shown_again', 'disabled', 'enabled')),
    -- The post, or for a comment its post. Null once it is deleted.
    entry_id uuid references entries(id) on delete set null,
    comment_id uuid references post_comments(id) on delete set null,
    -- The post, comment or account id. It has no foreign key, so it outlives
    -- the purge, and a later restored row still matches its removed row.
    item_id uuid not null,
    -- The title or the comment text at the time, for the history.
    snapshot text not null default '',
    -- The report reasons at a removal.
    reasons text[] not null default '{}',
    -- When it happened on the owner's own clock: the bell and the history
    -- read it.
    at timestamptz not null,
    created_at timestamptz not null default now()
);

create index moderation_events_owner_idx on moderation_events (owner_id, at desc);
