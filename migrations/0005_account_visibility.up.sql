-- Phase 2.1 (Settings): an account is public or private.
--
-- Developer decision of 2026-10-10: every account starts public. An account
-- whose birthday makes it under 18 on the day it is created is set to private
-- by the sign-up code (user.DefaultVisibility), not by SQL. Nothing is
-- rewritten here: the column default fills every existing row, the four real
-- dev accounts included, and no account is edited by hand.
--
-- The value is stored and shown only. The Friends view and Global (Phases 5
-- and 6) will read the AUTHOR's value of this column.

alter table users add column account_visibility text not null default 'public';

alter table users add constraint users_account_visibility_check
    check (account_visibility in ('public', 'private'));
