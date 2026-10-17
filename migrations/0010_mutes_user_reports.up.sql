-- Mute and report from a person's page (developer, 2026-10-17). A mute hides
-- the muted person's posts from the muter's Board; nothing else changes and
-- the muted person is never told. A report is kept for review; one per
-- reporter and person (a second report updates the reason).
create table mutes (
    muter_id uuid not null references users(id) on delete cascade,
    muted_id uuid not null references users(id) on delete cascade,
    created_at timestamptz not null,
    primary key (muter_id, muted_id),
    constraint mutes_not_self check (muter_id <> muted_id)
);

create table user_reports (
    reporter_id uuid not null references users(id) on delete cascade,
    reported_id uuid not null references users(id) on delete cascade,
    reason text not null,
    created_at timestamptz not null,
    primary key (reporter_id, reported_id),
    constraint user_reports_not_self check (reporter_id <> reported_id),
    constraint user_reports_reason check (reason in
        ('spam', 'nudity', 'harassment', 'hate', 'violence', 'impersonation', 'other'))
);

create index user_reports_reported_idx on user_reports (reported_id, created_at);
