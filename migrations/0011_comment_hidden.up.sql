-- Hide a comment (developer, 2026-10-18, like Instagram): the post's owner
-- hides a comment on their post. A hidden comment is seen only by its writer
-- (who is not told) and the post's owner; its replies go with it.
--
-- DROPPED the same day (developer: "let's remove it. We don't need it").
-- The column stays because dev was already at version 11 and is never
-- migrated down; nothing writes it, so it is always null.
alter table post_comments add column hidden_at timestamptz;
