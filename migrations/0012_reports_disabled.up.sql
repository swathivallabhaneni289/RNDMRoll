-- Reports on posts and comments, an admin's review of every report, and a
-- disabled account (developer, 2026-10-18: an admin web page reviews reports
-- and can disable an account, like Instagram's review).
--
-- users.disabled_at: set by an admin. A disabled person cannot log in, every
-- request with an old token is refused, and they are gone for everyone else
-- (treated as a block either way). Clearing it brings everything back.
alter table users add column disabled_at timestamptz;

-- resolved_at and resolution: set when an admin acts on a report (dismissed,
-- removed, disabled). An open report has both null.
alter table user_reports add column resolved_at timestamptz;
alter table user_reports add column resolution text
    check (resolution in ('dismissed', 'removed', 'disabled'));

create table post_reports (
    id uuid primary key default gen_random_uuid(),
    reporter_id uuid not null references users(id) on delete cascade,
    -- Null once the post is gone (removed by an admin or deleted): the report stays as history.
    entry_id uuid references entries(id) on delete set null,
    reason text not null check (reason in
        ('spam', 'nudity', 'harassment', 'hate', 'violence', 'impersonation', 'other')),
    created_at timestamptz not null,
    resolved_at timestamptz,
    resolution text check (resolution in ('dismissed', 'removed', 'disabled')),
    -- Who owned the post, kept so the report still points at a person.
    owner_id uuid not null references users(id) on delete cascade,
    -- The post's title when reported, kept for the review.
    title text not null,
    unique (reporter_id, entry_id)
);

create table comment_reports (
    id uuid primary key default gen_random_uuid(),
    reporter_id uuid not null references users(id) on delete cascade,
    comment_id uuid references post_comments(id) on delete set null,
    reason text not null check (reason in
        ('spam', 'nudity', 'harassment', 'hate', 'violence', 'impersonation', 'other')),
    created_at timestamptz not null,
    resolved_at timestamptz,
    resolution text check (resolution in ('dismissed', 'removed', 'disabled')),
    owner_id uuid not null references users(id) on delete cascade,
    -- The comment text when reported, kept for the review.
    body text not null,
    unique (reporter_id, comment_id)
);

create index post_reports_open_idx on post_reports (created_at) where resolved_at is null;
create index comment_reports_open_idx on comment_reports (created_at) where resolved_at is null;
