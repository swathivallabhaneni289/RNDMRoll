-- Reverse of 0018. Every pending deletion is forgotten with the column.
drop index if exists users_deletion_requested_idx;
alter table users drop column if exists deletion_requested_at;
