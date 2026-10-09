alter table users drop constraint users_email_verified_via_check;
alter table users add constraint users_email_verified_via_check
    check (email_verified_via in ('password_flow', 'apple', 'google'));

alter table users add column apple_subject text;
alter table users add column google_subject text;

create unique index users_apple_subject_idx on users (apple_subject) where apple_subject is not null;
create unique index users_google_subject_idx on users (google_subject) where google_subject is not null;
