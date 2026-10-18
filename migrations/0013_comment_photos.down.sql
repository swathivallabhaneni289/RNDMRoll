-- Reverse of 0013. Photo-only comments go (they would break the old rule).
delete from post_comments where char_length(btrim(body)) = 0;
alter table post_comments drop constraint post_comments_body_len;
alter table post_comments add constraint post_comments_body_len check (char_length(btrim(body)) between 1 and 140);
drop index if exists post_comments_photo_key_idx;
alter table post_comments drop column if exists photo_key;
