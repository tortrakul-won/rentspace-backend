ALTER TABLE users DROP COLUMN IF EXISTS full_name;

ALTER TABLE profiles RENAME COLUMN display_name TO profile_name;
