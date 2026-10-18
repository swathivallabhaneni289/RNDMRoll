-- Photos in comments (developer, 2026-10-17, 06-FOLLOW-OVERRIDES section 10):
-- one photo per comment or reply, uploaded like an entry photo (the same
-- ticket and key shape, entries/{writer}/...). The text may be empty when a
-- photo is there. Deleting a comment deletes its photo file (the server does
-- it, best effort). A file belongs to one row only.
alter table post_comments add column photo_key text;
create unique index post_comments_photo_key_idx on post_comments (photo_key) where photo_key is not null;

alter table post_comments drop constraint post_comments_body_len;
alter table post_comments add constraint post_comments_body_len check (
    char_length(body) <= 140
    and (char_length(btrim(body)) >= 1 or photo_key is not null)
);
