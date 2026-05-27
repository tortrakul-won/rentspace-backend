ALTER TABLE profiles RENAME COLUMN legal_name TO legal_name_th;

ALTER TABLE profiles ADD COLUMN IF NOT EXISTS legal_name_en TEXT NOT NULL DEFAULT '';
