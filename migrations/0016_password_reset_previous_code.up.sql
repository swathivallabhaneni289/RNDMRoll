-- Forgot password (2026-10-22): the code a new one replaced is kept, so a
-- reset that types it is told a newer code was sent (and costs no try)
-- instead of being counted as a wrong guess. Only an HMAC is kept, as for the
-- live code. It is set only when an unexpired code is replaced, goes with the
-- row when the code is spent, and is never read once the code has expired.
alter table password_resets add column previous_code_hash bytea;
