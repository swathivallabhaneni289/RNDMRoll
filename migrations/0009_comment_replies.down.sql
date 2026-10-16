-- Reverse of 0009. Replies stay as plain comments.
drop index if exists post_comments_parent_idx;
alter table post_comments drop column if exists parent_id;
