-- Reverse of 0016. The previous code is just a wrong code again.
alter table password_resets drop column if exists previous_code_hash;
