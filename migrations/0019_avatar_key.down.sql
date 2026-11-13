-- Reverse of 0019. The rows keep their bare keys: the old public base of the
-- links is not known here, so the links cannot be made again.
alter table users drop constraint if exists users_avatar_key_shape;
alter table users rename column avatar_key to avatar_url;
