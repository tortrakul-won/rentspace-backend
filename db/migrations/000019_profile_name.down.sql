ALTER TABLE profiles RENAME COLUMN profile_name TO display_name;

ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name TEXT NOT NULL DEFAULT '';
