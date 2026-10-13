-- Phase 3 schema: the daily entry (log it).
--
-- Two small additions to spins (when the day was settled, and the pair key
-- the entry's foreign key points at) and two new tables: entries (one per
-- person per local date) and entry_photos (one to ten photos per entry, in
-- order). users is not touched.
--
-- The database stores each photo's storage KEY (folder and file name), never
-- a link: the server builds the link when it reads. An entry's category id
-- and written name are not copied here: they come from the entry's spin, so
-- a later rename or wheel edit changes nothing.
--
-- created_at and updated_at come from the Go service clock (no defaults, no
-- trigger), so a moved test clock stays coherent with the spin rows.
--
-- "At least one photo" is not a database rule: the service enforces it in the
-- same transaction that writes the rows. "At most ten" is both: the service
-- and the position range together with the unique position per entry.

-- When a day became posted or skipped (set by the service from its clock). A
-- post or skip made at or after the spin's window_closes_at keeps Today on
-- Posted or Skipped until the next opening.
alter table spins add column settled_at timestamptz;

-- The foreign key from entries names (spin_id, user_id), so an entry can never
-- point at another person's spin.
alter table spins add constraint spins_id_user_unique unique (id, user_id);

create table entries (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    spin_id uuid not null,
    local_date date not null,
    title text not null,
    stars smallint not null,
    reaction text,
    comment text,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    constraint entries_spin_fk foreign key (spin_id, user_id)
        references spins (id, user_id),
    constraint entries_one_per_day unique (user_id, local_date),
    constraint entries_one_per_spin unique (spin_id),
    constraint entries_title_len check (char_length(btrim(title)) between 1 and 80),
    constraint entries_stars_range check (stars between 1 and 10),
    -- The photo rows point at (id, user_id), so a photo can never belong to
    -- another person's entry.
    constraint entries_id_user_unique unique (id, user_id),
    constraint entries_reaction_check check (
        reaction is null or reaction in ('love', 'fire', 'laughing', 'wow', 'sad', 'sleepy')
    ),
    constraint entries_comment_len check (comment is null or char_length(comment) <= 140)
);

-- The diary (a later phase) reads a person's entries newest first.
create index entries_user_date_idx on entries (user_id, local_date desc);

create table entry_photos (
    id uuid primary key default gen_random_uuid(),
    entry_id uuid not null,
    user_id uuid not null references users(id) on delete cascade,
    position smallint not null,
    storage_key text not null,
    content_type text not null,
    size_bytes integer,
    source text not null,
    created_at timestamptz not null,
    -- Deleting an entry removes its photo rows.
    constraint entry_photos_entry_fk foreign key (entry_id, user_id)
        references entries (id, user_id) on delete cascade,
    -- At most ten per entry; the unique position is deferred so one
    -- transaction may reorder (swap) the rows.
    constraint entry_photos_position_range check (position between 0 and 9),
    constraint entry_photos_position_unique unique (entry_id, position)
        deferrable initially deferred,
    -- A file belongs to one photo row only.
    constraint entry_photos_key_unique unique (storage_key),
    constraint entry_photos_key_len check (char_length(storage_key) between 1 and 200),
    constraint entry_photos_content_type_check check (
        content_type in ('image/jpeg', 'image/png', 'image/webp')
    ),
    -- The size the phone declared: informational, never used for a decision.
    constraint entry_photos_size_range check (size_bytes is null or size_bytes between 1 and 52428800),
    constraint entry_photos_source_check check (source in ('camera', 'library'))
);
