-- Reverse of 0004, last table first. Triggers and indexes go with their
-- tables. set_updated_at() belongs to 0001 and stays.
drop table if exists goal_weeks;
drop table if exists posted_days;
drop table if exists spins;
drop table if exists wheel_categories;
drop table if exists user_spin_state;
drop table if exists user_time_zones;
