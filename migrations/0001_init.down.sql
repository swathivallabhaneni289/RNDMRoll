drop table if exists refresh_tokens;
drop table if exists email_verification_tokens;
drop trigger if exists users_set_updated_at on users;
drop function if exists set_updated_at();
drop table if exists users;
