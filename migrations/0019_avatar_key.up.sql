-- Private photos (developer, 2026-11-09): the bucket is private, so a
-- profile photo is stored as its storage key, avatars/{id}/{16 hex}.{ext},
-- the way post photos already are, and the server signs a link that stops
-- working within an hour each time it sends one.
--
-- Each stored link becomes the key at its end. The pattern is tied to the end
-- of the link, so it works whatever the host or the bucket is called (cutting
-- at the first /avatars/ would break with a bucket named avatars). A row that
-- does not match, or that names someone else's folder, becomes no photo
-- instead of failing the migration. A bare key is kept as it is, so going down
-- and up again changes nothing.
alter table users rename column avatar_url to avatar_key;
update users set avatar_key = substring(avatar_key from '(avatars/[0-9a-f-]{36}/[0-9a-f]{16}\.[a-z0-9]{1,10})$') where avatar_key is not null;
update users set avatar_key = null where avatar_key !~ ('^avatars/' || id::text || '/[0-9a-f]{16}\.[a-z0-9]{1,10}$');
alter table users add constraint users_avatar_key_shape check (avatar_key is null or avatar_key ~ ('^avatars/' || id::text || '/[0-9a-f]{16}\.[a-z0-9]{1,10}$'));
