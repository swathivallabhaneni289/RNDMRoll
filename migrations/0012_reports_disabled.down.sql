-- Reverse of 0012.
drop table if exists comment_reports;
drop table if exists post_reports;
alter table user_reports drop column if exists resolution;
alter table user_reports drop column if exists resolved_at;
alter table users drop column if exists disabled_at;
