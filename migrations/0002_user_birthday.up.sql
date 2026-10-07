-- Sign-up asks for a birthday so the server can enforce the minimum age.
-- The column is private: no query selects the date itself (see userColumns
-- in internal/store/postgres/user_repo.go), only whether one is on file.
-- Nullable with no default, so existing rows stay NULL and nothing is
-- rewritten. The clock-dependent rules (not in the future, 13 or older)
-- live in Go, deliberately not as constraints.

alter table users add column birthday date;

alter table users add constraint users_birthday_min
    check (birthday is null or birthday >= date '1900-01-01');
