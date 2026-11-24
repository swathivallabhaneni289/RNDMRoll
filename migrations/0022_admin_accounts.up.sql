-- The admin's own account is hidden from everyone else in the app
-- (developer, 2026-11-24: "the admin account should not show to anyone"):
-- it is not in search, suggestions or Find your friends, its page and posts
-- open as a miss and no one can follow it, as if it were disabled. It still
-- logs in, to the admin page and to the app.
--
-- admin_accounts: the accounts the ADMIN_EMAILS addresses have. The server
-- rewrites it at every start that finds one (cmd/api), and a start that
-- finds none leaves it as it was, so a forgotten setting never shows the
-- admin to everyone. It only hides: who may open the admin page still comes
-- from ADMIN_EMAILS alone. Its own table, so nothing is added to users.
create table admin_accounts (
    user_id uuid primary key references users(id) on delete cascade
);
