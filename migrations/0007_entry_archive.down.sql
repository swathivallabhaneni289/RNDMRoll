-- Reverse of 0007.
drop index if exists entries_user_archived_idx;
alter table entries drop column if exists comments_off;
alter table entries drop column if exists archived_at;
