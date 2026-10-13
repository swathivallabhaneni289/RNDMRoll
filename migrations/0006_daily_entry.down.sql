-- Reverse of 0006, last table first. Photo rows go with their entries; the
-- two additions to spins are dropped after the tables that depend on them.
drop table if exists entry_photos;
drop table if exists entries;
alter table spins drop constraint if exists spins_id_user_unique;
alter table spins drop column if exists settled_at;
