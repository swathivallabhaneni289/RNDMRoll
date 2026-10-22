-- Push tokens belong to a session (2026-10-22): each token is tied to the
-- refresh token of the session that sent it, and is sent to only while that
-- session lasts (not revoked, not expired). Log out, a password change, an
-- admin's disable and a session that simply runs out then stop the phone's
-- pushes even when its DELETE never reaches the server. A refresh moves the
-- token to the new refresh token. Rows kept before this have no session and
-- are not sent to until the app sends its token again.
alter table push_tokens add column refresh_token_id uuid references refresh_tokens (id) on delete set null;
create index push_tokens_refresh_token_idx on push_tokens (refresh_token_id);
