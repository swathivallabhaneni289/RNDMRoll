-- Delete account in the app, like Instagram (developer, 2026-10-25).
--
-- users.deletion_requested_at: set when the person asks, in the app and with
-- their password, for the account to be deleted. From then on the account is
-- gone for everyone else, exactly like a disabled one, and every session of
-- it has ended. Logging in, or a Forgot password reset, inside the grace
-- period (user.DeletionGracePeriod, 30 days, on the database's clock) clears
-- it again. Once the grace period is over the purge (internal/purge) deletes
-- the account's files and then the row, and the cascades take everything
-- else. Null for every other account.
alter table users add column deletion_requested_at timestamptz;

-- The purge reads the accounts that are due, the oldest ask first.
create index users_deletion_requested_idx on users (deletion_requested_at)
    where deletion_requested_at is not null;
