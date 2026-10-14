-- Entry archive and comments switch. archived_at is set when the owner puts
-- an entry away (null means not archived). An archived entry still counts for
-- its day: the posted day and the spin's result are not touched. comments_off
-- is the owner's switch for comments on the entry.
alter table entries add column archived_at timestamptz;
alter table entries add column comments_off boolean not null default false;

-- The archive list reads a person's archived entries newest first.
create index entries_user_archived_idx on entries (user_id, local_date desc)
    where archived_at is not null;
