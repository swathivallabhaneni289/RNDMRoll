-- Replies to comments, one level. parent_id is the top-level comment a reply
-- belongs to (null for a top-level comment). The service always stores the
-- top-level comment, never a reply, and only a comment on the same entry.
-- Deleting a top-level comment deletes its replies.
alter table post_comments add column parent_id uuid references post_comments(id) on delete cascade;

-- The post page reads a comment's replies oldest first.
create index post_comments_parent_idx on post_comments (parent_id, created_at, id)
    where parent_id is not null;
