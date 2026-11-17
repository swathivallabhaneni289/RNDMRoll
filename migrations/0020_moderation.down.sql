-- Reverse of 0020. The decision log goes first, then the indexes, then the
-- columns: every removed post and comment shows again.
drop table if exists moderation_events;
drop index if exists comment_reports_comment_idx;
drop index if exists post_reports_entry_idx;
drop index if exists post_comments_removed_idx;
drop index if exists entries_removed_idx;
alter table post_comments drop column if exists removed_at;
alter table entries drop column if exists removed_at;
