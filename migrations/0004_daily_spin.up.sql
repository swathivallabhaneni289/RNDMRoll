-- Phase 2 schema: the daily spin. New tables only; users is not touched.
--
-- Every table references users(id) on delete cascade, so deleting an account
-- (or truncating users with cascade, as the test cleanups do) clears them.
-- Every updated_at trigger calls set_updated_at() from 0001; this migration
-- never redefines it and its down file never drops it.
--
-- Dates in these tables (local_date, week_start, seeded_local_date) are the
-- person's own calendar dates, written by the server from its own clock and
-- the person's stored zone, never from SQL now().

-- The person's time zone, reported by the phone. Its own table so nothing is
-- added to users.
create table user_time_zones (
    user_id uuid primary key references users(id) on delete cascade,
    tz_name text not null check (char_length(tz_name) between 1 and 64),
    updated_at timestamptz not null default now()
);

-- One row per person: the lock for everything a person does with the wheel,
-- the spin and the week. Made on first use, with the six starting categories,
-- in one transaction. seeded_* say when and where that happened, so the next
-- phase can treat the first partial week differently for a streak.
create table user_spin_state (
    user_id uuid primary key references users(id) on delete cascade,
    seeded_at timestamptz not null,
    seeded_local_date date not null,
    seeded_tz text not null,
    weekly_goal smallint not null check (weekly_goal between 1 and 7),
    bonus_held smallint not null default 0 check (bonus_held in (0, 1)),
    wheel_version integer not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create trigger user_spin_state_set_updated_at
    before update on user_spin_state
    for each row
    execute function set_updated_at();

-- A person's wheel. Removing a category archives it, so old spins keep
-- pointing at it. Name uniqueness among live categories is checked in Go,
-- under the state row lock, not here.
create table wheel_categories (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    name text not null check (char_length(btrim(name)) between 1 and 24),
    weight smallint not null check (weight between 1 and 10),
    position integer not null,
    archived_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint wheel_categories_id_user_unique unique (id, user_id)
);

create index wheel_categories_user_position_idx on wheel_categories (user_id, position);

create trigger wheel_categories_set_updated_at
    before update on wheel_categories
    for each row
    execute function set_updated_at();

-- One row per spin. seq 0 is the day's daily spin; a bonus spin is seq 1.
-- category_name and wheel are copies as they were when the spin was made, so
-- a later rename or edit changes nothing here. result is set by the entry
-- phase; this phase only ever stores 'open'.
create table spins (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    local_date date not null,
    tz_name text not null,
    kind text not null check (kind in ('daily', 'bonus')),
    seq smallint not null,
    category_id uuid not null,
    category_name text not null,
    wheel jsonb not null,
    winner_index smallint not null,
    spun_at timestamptz not null,
    window_opens_at timestamptz not null,
    window_closes_at timestamptz not null,
    result text not null default 'open' check (result in ('open', 'posted', 'skipped')),
    created_at timestamptz not null default now(),
    constraint spins_category_fk foreign key (category_id, user_id)
        references wheel_categories (id, user_id),
    constraint spins_user_date_seq_unique unique (user_id, local_date, seq),
    constraint spins_kind_seq_check check (
        (kind = 'daily' and seq = 0) or (kind = 'bonus' and seq >= 1)
    )
);

-- At most one bonus spin per person per date.
create unique index spins_one_bonus_per_day on spins (user_id, local_date) where kind = 'bonus';

-- Dates counted as posted. This phase fills it only through the test tools
-- (source 'test'); the entry phase writes source 'post'. A 'test' row hides
-- a real post for that date.
create table posted_days (
    user_id uuid not null references users(id) on delete cascade,
    local_date date not null,
    source text not null default 'post' check (source in ('post', 'test')),
    late boolean not null default false,
    created_at timestamptz not null default now(),
    primary key (user_id, local_date)
);

-- A week that reached its goal. Having a row means the week is paid: no
-- second bonus spin comes from it, even after the held one is spent.
create table goal_weeks (
    user_id uuid not null references users(id) on delete cascade,
    week_start date not null,
    goal smallint not null,
    goal_met_at timestamptz not null,
    bonus_granted boolean not null,
    primary key (user_id, week_start)
);
