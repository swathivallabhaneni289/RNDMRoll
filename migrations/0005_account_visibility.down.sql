-- Reverse of 0005: the check first, then the column. The column holds only the
-- public or private choice, which no other table copies, so going down loses
-- that choice and nothing else. Every user row stays.
alter table users drop constraint users_account_visibility_check;

alter table users drop column account_visibility;
