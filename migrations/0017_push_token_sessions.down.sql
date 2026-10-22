-- Reverse of 0017. Tokens are no longer tied to a session.
drop index if exists push_tokens_refresh_token_idx;
alter table push_tokens drop column if exists refresh_token_id;
