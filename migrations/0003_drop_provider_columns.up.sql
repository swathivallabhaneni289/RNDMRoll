-- Apple and Google sign-in were removed from the project (developer
-- decision, 2026-10-09). No account used them, so nothing is lost.
-- The two unique indexes go with their columns, and the verification check
-- is narrowed to the one source that still exists. The check fails on
-- purpose if a row still carries an Apple or Google source.

drop index users_apple_subject_idx;
drop index users_google_subject_idx;

alter table users drop column apple_subject;
alter table users drop column google_subject;

alter table users drop constraint users_email_verified_via_check;
alter table users add constraint users_email_verified_via_check
    check (email_verified_via in ('password_flow'));
