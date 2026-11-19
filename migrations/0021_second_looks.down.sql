-- Reverse of 0021. The requests go, with their words (their photo files stay
-- in the owners' folders until the accounts go), then every Keep it removed
-- in the log, so the old check holds again.
drop table if exists second_looks;
delete from moderation_events where action = 'kept';
alter table moderation_events drop constraint if exists moderation_events_action_check;
alter table moderation_events add constraint moderation_events_action_check
    check (action in ('removed', 'restored', 'shown_again', 'disabled', 'enabled', 'hidden'));
